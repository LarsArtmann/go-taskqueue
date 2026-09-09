// Package worker implements the claim→heartbeat→execute→complete loop.
package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// Config controls one worker pool.
type Config struct {
	Owner        string        // lease owner identity (default: auto)
	Concurrency  int           // parallel task executions (default 2)
	PollInterval time.Duration // idle poll gap (default 250ms)
	Lease        time.Duration // claim lease length (default 2m)
	Heartbeat    time.Duration // heartbeat cadence, < Lease (default 30s)
	TaskTimeout  time.Duration // per-task execution cap (default 10m)
	Executors    *executor.Registry
	Backoff      func(attempt int) time.Duration // retry backoff (default exp)
	// PreflightBackoff delays re-claiming after a preflight refusal
	// (dirty repo, missing autonomy config). Default 2m. No attempt is
	// burned by preflight refusals.
	PreflightBackoff time.Duration
}

func (c *Config) setDefaults() {
	if c.Owner == "" {
		c.Owner = fmt.Sprintf("worker-%d", time.Now().UnixMilli()%100000)
	}

	if c.Concurrency <= 0 {
		c.Concurrency = 2
	}

	if c.PollInterval <= 0 {
		c.PollInterval = 250 * time.Millisecond
	}

	if c.Lease <= 0 {
		c.Lease = 2 * time.Minute
	}

	if c.Heartbeat <= 0 {
		c.Heartbeat = c.Lease / 4
	}

	if c.TaskTimeout <= 0 {
		c.TaskTimeout = 10 * time.Minute
	}

	if c.Backoff == nil {
		c.Backoff = ExpBackoff
	}

	if c.PreflightBackoff <= 0 {
		c.PreflightBackoff = 2 * time.Minute
	}
}

// ExpBackoff returns an exponential retry delay for the given attempt number.
func ExpBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}

	d := min(time.Duration(math.Pow(2, float64(attempt)))*time.Second, 5*time.Minute)

	return d
}

// Pool runs N concurrent claim-execute loops against a store.
type Pool struct {
	cfg   Config
	store queue.Store
	log   *slog.Logger

	wg       sync.WaitGroup
	stopCh   chan struct{}
	stopOnce sync.Once
	mu       sync.Mutex
	inFlight map[task.ID]struct{}

	// preflight tracks consecutive requeues per task so a sustained-dirty
	// repo escalates instead of bouncing at the base backoff, and the
	// refusal log stays quiet once the situation is known.
	preflightMu   sync.Mutex
	preflightSeen map[task.ID]*preflightState
}

// preflightState is one task's consecutive-refusal tracker.
type preflightState struct {
	count   int
	lastLog time.Time
}

// preflightMaxBackoff caps the dirty-tree requeue ladder.
const preflightMaxBackoff = 15 * time.Minute

// preflightLogInterval rate-limits the per-task refusal log: the first
// refusal logs immediately, repeats stay quiet for this long.
const preflightLogInterval = time.Minute

// preflightDelay advances the task's refusal ladder and returns the next
// requeue delay: base * 2^(n-1), capped, with ±20% jitter so many refused
// tasks do not reclaim in lockstep.
func (p *Pool) preflightDelay(id task.ID) time.Duration {
	p.preflightMu.Lock()
	defer p.preflightMu.Unlock()

	st := p.preflightSeen[id]
	if st == nil {
		st = &preflightState{}

		if p.preflightSeen == nil {
			p.preflightSeen = make(map[task.ID]*preflightState)
		}

		p.preflightSeen[id] = st
	}

	st.count++

	d := p.cfg.PreflightBackoff << min(st.count-1, 8) //nolint:gosec // shift bounded by min
	if d <= 0 || d > preflightMaxBackoff {
		d = preflightMaxBackoff
	}

	jitter := 0.8 + 0.4*rand.Float64()

	return time.Duration(float64(d) * jitter)
}

// preflightShouldLog reports whether the refusal for this task should hit
// the log now (first refusal, or the interval elapsed since the last one).
// Assumes preflightDelay already ran for this refusal.
func (p *Pool) preflightShouldLog(id task.ID) bool {
	p.preflightMu.Lock()
	defer p.preflightMu.Unlock()

	st := p.preflightSeen[id]
	if st == nil {
		return true
	}

	if time.Since(st.lastLog) >= preflightLogInterval {
		st.lastLog = time.Now()

		return true
	}

	return false
}

// preflightCount reports the task's consecutive-refusal count (0 when
// untracked) — used for the log line after preflightShouldLog.
func (p *Pool) preflightCount(id task.ID) int {
	p.preflightMu.Lock()
	defer p.preflightMu.Unlock()

	if st := p.preflightSeen[id]; st != nil {
		return st.count
	}

	return 0
}

// preflightReset forgets the task's ladder after any non-preflight
// outcome (it left the refusal loop: completed, failed, was cancelled).
func (p *Pool) preflightReset(id task.ID) {
	p.preflightMu.Lock()
	defer p.preflightMu.Unlock()

	delete(p.preflightSeen, id)
}

// New creates a worker pool. Call Start to run it.
func New(store queue.Store, cfg Config, log *slog.Logger) *Pool {
	if log == nil {
		log = slog.Default()
	}

	cfg.setDefaults()

	return &Pool{
		cfg:      cfg,
		store:    store,
		log:      log,
		stopCh:   make(chan struct{}),
		inFlight: make(map[task.ID]struct{}),
	}
}

// Start launches the pool; it blocks the caller until ctx is cancelled, then
// drains in-flight tasks and returns. Tasks execute under a context that is
// NOT cancelled by pool shutdown (bounded only by TaskTimeout), so a
// graceful stop lets agents finish; terminal store writes use that same
// uncancellable context so a cancelled parent cannot orphan a running task's
// outcome. Stop waiting with a second Ctrl-C (SIGKILL) if truly urgent.
func (p *Pool) Start(ctx context.Context) error {
	taskCtx := context.WithoutCancel(ctx)

	for range p.cfg.Concurrency {
		p.wg.Add(1)
		go p.loop(ctx, taskCtx)
	}

	<-ctx.Done()
	p.Stop()
	p.wg.Wait()

	return nil
}

// Stop signals loops to exit after their current task.
func (p *Pool) Stop() {
	p.stopOnce.Do(func() { close(p.stopCh) })
}

// InFlight returns how many tasks are currently executing.
func (p *Pool) InFlight() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return len(p.inFlight)
}

// Owner returns the lease owner identity in use (after defaulting).
func (p *Pool) Owner() string { return p.cfg.Owner }

func (p *Pool) loop(ctx, taskCtx context.Context) {
	defer p.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		default:
		}

		t, err := p.store.ClaimDue(ctx, p.cfg.Owner, p.cfg.Lease)
		if err != nil {
			if ctx.Err() != nil {
				return // shutdown raced the claim; not an error
			}

			if !errors.Is(err, queue.ErrNoTaskDue) {
				p.log.Error("claim failed", "err", err)
			}

			if !sleepCtx(ctx, p.cfg.PollInterval) {
				return
			}

			continue
		}

		p.execute(taskCtx, t)
	}
}

// execute runs one claimed task under taskCtx. taskCtx survives pool shutdown
// (see Start): execution, heartbeats, and the terminal Complete/Fail write all
// use it, so a draining task keeps its lease and records its outcome. A worker
// that dies anyway loses its lease to expiry reclaim — at-least-once.
func (p *Pool) execute(ctx context.Context, t task.Task) {
	p.mu.Lock()
	p.inFlight[t.ID] = struct{}{}

	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.inFlight, t.ID)
		p.mu.Unlock()
	}()

	// Heartbeat ticker: extends the lease while execution runs. Parented on the
	// shutdown-surviving task context so draining tasks keep their lease.
	hbCtx, hbCancel := context.WithCancel(ctx)
	defer hbCancel()

	hbDone := make(chan struct{})
	go func() {
		defer close(hbDone)

		ticker := time.NewTicker(p.cfg.Heartbeat)
		defer ticker.Stop()

		for {
			select {
			case <-hbCtx.Done():
				return
			case <-ticker.C:
				if err := p.store.Heartbeat(hbCtx, t.ID, p.cfg.Owner, p.cfg.Lease); err != nil {
					// Lease lost (expiry/reclaim). Stop heartbeating; the
					// executor context is cancelled below so we do not
					// complete a task we no longer own.
					p.log.Warn("heartbeat failed; lease lost", "task", t.ID, "err", err)
					hbCancel()

					return
				}

				// Cooperative cancel: the operator's task.cancel-requested
				// fact is observed here, at most one heartbeat late.
				// Cancelling hbCtx stops the execution (executors kill
				// their process tree); the outcome is finalized as
				// Cancelled in execute.
				requested, err := p.store.CancelRequested(hbCtx, t.ID)
				if err != nil {
					p.log.Warn("cancel check failed", "task", t.ID, "err", err)

					continue
				}

				if requested {
					p.log.Info("cancel requested; stopping execution", "task", t.ID)
					hbCancel()

					return
				}
			}
		}
	}()

	// Executors can attach structured outcome detail (agent session id,
	// verify tail) to the sink; successful completions store it.
	runCtx, sink := executor.NewSink(hbCtx)
	execErr := p.runExecutor(runCtx, t)

	hbCancel()
	<-hbDone

	// Any non-preflight outcome leaves the refusal loop: forget the
	// task's dirty-tree ladder so a later refusal starts fresh.
	if _, isPreflight := errors.AsType[*executor.PreflightError](execErr); !isPreflight {
		p.preflightReset(t.ID)
	}

	// Terminal writes (Complete/Fail) use the shutdown-surviving task context,
	// so a draining task's outcome is never orphaned by the cancelled pool.
	terminalCtx := ctx
	if execErr == nil {
		if err := p.store.Complete(terminalCtx, t.ID, p.cfg.Owner, sink.Detail()); err != nil {
			p.log.Error("complete failed", "task", t.ID, "err", err)
		}

		return
	}
	// Lease lost during execution: do NOT fail — the reclaiming worker owns
	// the task now. Our attempt result is discarded (at-least-once).
	if _, ok := errors.AsType[*executor.LeaseLostError](execErr); ok {
		p.log.Warn("skipping fail: lease lost", "task", t.ID)

		return
	}

	// Cooperative cancel: the execution was stopped because the operator
	// requested it (observed at the heartbeat). Finalize as Cancelled — the
	// attempt does not burn, the task is withdrawn, not failed.
	if errors.Is(execErr, context.Canceled) {
		if requested, err := p.store.CancelRequested(ctx, t.ID); err == nil && requested {
			if cerr := p.store.CancelOwned(ctx, t.ID, p.cfg.Owner); cerr != nil {
				p.log.Warn("cancel-owned failed; lease lost mid-cancel", "task", t.ID, "err", cerr)
			} else {
				p.log.Info("task cancelled by operator request", "task", t.ID)
			}

			return
		}
	}

	if pre, ok := errors.AsType[*executor.PreflightError](execErr); ok {
		// The executor refused to START: environment not ready (dirty repo,
		// missing autonomy). Requeue WITHOUT burning an attempt — the task
		// becomes claimable again once the delay passes, so the pool picks
		// it up when the human has committed their work. Consecutive
		// refusals escalate (base * 2^n capped, ±20% jitter) so a
		// sustained-dirty repo does not bounce at the base backoff.
		delay := p.preflightDelay(t.ID)
		if err := p.store.Requeue(terminalCtx, t.ID, p.cfg.Owner, pre.Error(), delay); err != nil {
			p.log.Error("requeue failed", "task", t.ID, "err", err)
		} else if p.preflightShouldLog(t.ID) {
			p.log.Warn("preflight refused; requeued without attempt burn",
				"task", t.ID, "retry after", delay, "consecutive", p.preflightCount(t.ID), "reason", pre.Cause.Error())
		}

		return
	}

	if perm, ok := errors.AsType[*executor.PermanentError](execErr); ok {
		// The identical retry would fail identically (bad payload, missing
		// repo). Dead-letter now instead of burning the retry budget — for
		// agent tasks every retry is real money.
		if err := p.store.FailPermanent(terminalCtx, t.ID, p.cfg.Owner, perm.Error(), sink.Failure()); err != nil {
			p.log.Error("permanent fail failed", "task", t.ID, "err", err)
		}

		return
	}

	if errors.Is(execErr, context.Canceled) && ctx.Err() != nil {
		// Task context cancelled mid-run (defensive: the task context ignores
		// pool shutdown; only internal cancellation lands here). Burn the
		// attempt (crash-safe equivalent) with zero backoff so it is immediately
		// reclaimable.
		if err := p.store.Fail(terminalCtx, t.ID, p.cfg.Owner, "worker shutdown: "+execErr.Error(), 0, sink.Failure()); err != nil {
			p.log.Error("fail-on-shutdown failed", "task", t.ID, "err", err)
		}

		return
	}

	// Failure evidence (exit code, output tail — executor.FailureEvidence)
	// rides the task.failed fact's detail so a failed attempt is debuggable
	// from the journal alone (21:40 report §d4: both retry-path failures
	// left empty {} detail).
	if err := p.store.Fail(terminalCtx, t.ID, p.cfg.Owner, execErr.Error(), p.cfg.Backoff(t.Attempts+1), sink.Failure()); err != nil {
		p.log.Error("fail failed", "task", t.ID, "err", err)
	}
}

func (p *Pool) runExecutor(ctx context.Context, t task.Task) error {
	exec, err := p.cfg.Executors.Lookup(t.Type)
	if err != nil {
		// Unknown type is a permanent error: the payload can never match a
		// registered executor on a retry either. Dead-letter via the
		// permanent class instead of exhausting attempts.
		return executor.Permanent(fmt.Errorf("no executor for type %q: %w", t.Type, err))
	}

	runCtx, cancel := context.WithTimeout(ctx, p.cfg.TaskTimeout)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("executor panicked: %v", r)
			}
		}()

		done <- exec.Execute(runCtx, t)
	}()

	select {
	case err := <-done:
		return err
	case <-runCtx.Done():
		// The heartbeat goroutine shares ctx; if OUR ctx was cancelled the
		// lease is being abandoned anyway.
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Parent (heartbeat) context cancelled us without a pool shutdown:
		// cooperative cancel or lease loss — report the cancellation itself,
		// not a misleading timeout.
		if errors.Is(runCtx.Err(), context.Canceled) {
			return runCtx.Err()
		}

		return fmt.Errorf("task timeout after %s", p.cfg.TaskTimeout)
	}
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}
