// Package worker implements the claim→heartbeat→execute→complete loop.
package worker

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/larsartmann/go-retry"
	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// Config controls one worker pool.
type Config struct {
	Owner        string        // lease owner identity (default: auto)
	Concurrency  int           // parallel task executions (default 2)
	PollInterval time.Duration // idle poll gap (default 250ms)
	// IdlePollMax caps the adaptive idle poll gap: consecutive empty
	// claims (ErrNoTaskDue) double the gap up to this cap, and any claim
	// resets it to PollInterval — an idle pool stops paying the full
	// candidate-scan cost at every tick while a fresh task is still
	// claimed within the cap. Default 2s; negative disables the ladder
	// (fixed PollInterval, the historical behavior).
	IdlePollMax time.Duration

	// Wake is the optional claim-wake channel (queue.Waker.Notify from a
	// Waker store): a commit that may have landed a task in PENDING
	// fires it, so an idle loop re-claims in milliseconds instead of
	// waiting out the idle ladder, and the wake resets the ladder to
	// full cadence. The ladder stays the degraded fallback — a nil Wake
	// (or a store without the seam) keeps the pure poll behavior, and
	// the signal is process-local, so same-DB pools in other processes
	// still poll.
	Wake <-chan struct{}

	Lease       time.Duration // claim lease length (default 2m)
	Heartbeat   time.Duration // heartbeat cadence, < Lease (default 30s)
	TaskTimeout time.Duration // per-task execution cap (default 10m)
	Executors   *executor.Registry
	Backoff     func(attempt int) time.Duration // retry backoff (default exp)
	// PreflightBackoff delays re-claiming after a preflight refusal
	// (dirty repo, missing autonomy config). Default 2m. No attempt is
	// burned by preflight refusals.
	PreflightBackoff time.Duration

	// EnvRequeueBurn bounds the environmental-requeue ladder (R3 option
	// (a), docs/planning/2026-10-01_11-19_env-requeue-circuit-breaker.md):
	// after this many consecutive preflight/gate refusals the attempt
	// burns (store.Fail, exponential NotBefore escalation) instead of
	// requeueing again, so a sustained-sick gate cannot churn claims
	// forever. Default 3; negative disables (pure requeue behavior).
	EnvRequeueBurn int

	// Budget is the claim-time spend gate: before a claimed task's
	// executor runs, the hook decides whether the pool may start another
	// paid turn at all. A blocked task is requeued WITHOUT burning an
	// attempt, parked until the returned delay passes (a daily cap
	// resolves at local midnight), so work enqueued before the cap bit
	// cannot burn money after it. The reason rides the task.requeued
	// fact with requeue class "budget". Nil = ungated (historical
	// behavior); budget refusals never ride the environmental
	// streak/burn ladder — an exhausted budget is policy, not a sick
	// environment.
	Budget func(ctx context.Context) (blocked bool, reason string, retryIn time.Duration)

	// DonePreflight is the claim-time done gate: before a claimed task's
	// executor runs, the hook decides whether the task's work is provably
	// already done (footer commits exist, todo item ticked, fix cured…). A
	// done task completes immediately — no agent session, no attempt burn,
	// no review minted — with an AgentResult carrying PreflightDone=reason
	// as the completion detail. The hook must fail open (return false) on
	// its own errors: it augments dispatch, never blocks it. Nil =
	// ungated (historical behavior).
	DonePreflight func(ctx context.Context, t task.Task) (done bool, reason string)
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

	if c.IdlePollMax == 0 {
		c.IdlePollMax = 2 * time.Second
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

	if c.EnvRequeueBurn == 0 {
		c.EnvRequeueBurn = 3
	}
}

// idleGap returns the poll gap after idle consecutive empty claims:
// PollInterval shifted left by the streak (bounded), capped at
// IdlePollMax. The ladder is disabled (IdlePollMax < 0) or exhausted
// (idle <= 0) at the base interval. A cap below PollInterval never
// shortens a configured interval — the operator's floor wins.
func (c Config) idleGap(idle int) time.Duration {
	if c.IdlePollMax < 0 || idle <= 0 {
		return c.PollInterval
	}

	gap := c.PollInterval << min(idle, 8)
	if gap <= 0 { // shift overflow: saturate at the cap
		gap = c.IdlePollMax
	}

	return min(max(gap, c.PollInterval), max(c.IdlePollMax, c.PollInterval))
}

// ExpBackoff returns an exponential retry delay for the given attempt number.
func ExpBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}

	d := min(time.Duration(math.Pow(2, float64(attempt)))*time.Second, 5*time.Minute)

	return d
}

// rateLimitDelay spreads a provider reset delay by ±5% (capped at ±1min):
// enough that many parked tasks do not reclaim in the same instant and
// stampede the freshly reset quota, without warping a multi-hour Z.ai
// window the way percentage jitter would.
func rateLimitDelay(d time.Duration) time.Duration {
	spread := min(d/20, time.Minute)

	return d - spread + time.Duration(rand.Int64N(2*int64(spread)+1))
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

// EnvStreakCode marks breaker-burned attempts in the journal: a
// task.failed fact whose reason carries this code was killed by the
// environmental-requeue circuit breaker (sustained preflight/gate
// refusals), not by a judged execution failure.
const EnvStreakCode = "env-streak"

// preflightLogInterval rate-limits the per-task refusal log: the first
// refusal logs immediately, repeats stay quiet for this long.
const preflightLogInterval = time.Minute

// preflightDelay advances the task's refusal ladder and returns the
// next requeue delay together with the streak count AFTER this refusal
// (the circuit-breaker input): base * 2^(n-1), capped, with ±20% jitter
// so many refused tasks do not reclaim in lockstep.
func (p *Pool) preflightDelay(id task.ID) (time.Duration, int) {
	p.preflightMu.Lock()
	defer p.preflightMu.Unlock()

	st, _ := p.preflightStateFor(id, time.Time{})

	st.count++

	d := p.cfg.PreflightBackoff << min(st.count-1, 8)
	if d <= 0 || d > preflightMaxBackoff {
		d = preflightMaxBackoff
	}

	jitter := 0.8 + 0.4*rand.Float64()

	return time.Duration(float64(d) * jitter), st.count
}

// preflightStateFor returns the task's preflight entry, creating it (and
// the map) on first contact; lastLog seeds a fresh entry's log clock.
// Callers hold preflightMu.
func (p *Pool) preflightStateFor(id task.ID, lastLog time.Time) (*preflightState, bool) {
	if st := p.preflightSeen[id]; st != nil {
		return st, false
	}

	st := &preflightState{lastLog: lastLog}

	if p.preflightSeen == nil {
		p.preflightSeen = make(map[task.ID]*preflightState)
	}

	p.preflightSeen[id] = st

	return st, true
}

// preflightShouldLog reports whether the refusal for this task should hit
// the log now (first refusal, or the interval elapsed since the last one).
// Assumes preflightDelay already ran for this refusal.
func (p *Pool) preflightShouldLog(id task.ID) bool {
	p.preflightMu.Lock()
	defer p.preflightMu.Unlock()

	st, created := p.preflightStateFor(id, time.Now())
	if created {
		return true
	}

	if time.Since(st.lastLog) >= preflightLogInterval {
		st.lastLog = time.Now()

		return true
	}

	return false
}

// preflightReset forgets the task's ladder after any judged outcome
// (it left the refusal loop: completed, failed, was cancelled).
func (p *Pool) preflightReset(id task.ID) {
	p.preflightMu.Lock()
	defer p.preflightMu.Unlock()

	delete(p.preflightSeen, id)
}

// staysOnLadder reports whether an execution outcome keeps the task on
// the refusal ladder: the classes that requeue WITHOUT a verdict —
// preflight refusal and undetermined gate failure (gate-dead/gate-slow;
// the PROVEN-environmental gate signature dead-letters instead) — keep
// climbing it, while the parked classes (429, owner question) neither
// climb nor reset it. Success and every judged failure leave the ladder.
func staysOnLadder(execErr error) bool {
	if _, ok := errors.AsType[*executor.PreflightError](execErr); ok {
		return true
	}

	if gate, ok := errors.AsType[*executor.VerifyGateError](execErr); ok {
		return gate.Class != executor.VerifyGateEnvironmental
	}

	if _, ok := errors.AsType[*executor.RateLimitError](execErr); ok {
		return true
	}

	_, ok := errors.AsType[*executor.QuestionPendingError](execErr)

	return ok
}

// burnEnvStreak applies the environmental-requeue circuit breaker for a
// refusal that matured the ladder: instead of requeueing again, the
// attempt burns via Fail (exponential NotBefore escalation) with the
// EnvStreakCode reason riding the task.failed fact as the durable alert,
// and the ladder resets — the streak was consumed into the attempt.
// Reports whether the burn happened; the caller returns immediately
// when it did. With the breaker disabled (EnvRequeueBurn < 0) or the
// streak below N, the requeue stands. A FAILED burn (lease lost) also
// falls through so the task is never orphaned mid-transition.
func (p *Pool) burnEnvStreak(
	ctx context.Context,
	t task.Task,
	claim queue.Claim,
	streak int,
	cause error,
	sink *executor.Sink,
) bool {
	if p.cfg.EnvRequeueBurn <= 0 || streak < p.cfg.EnvRequeueBurn {
		return false
	}

	backoff := p.cfg.Backoff(t.Attempts + 1)
	reason := fmt.Sprintf("environmental requeue streak burned the attempt [%s]: streak %d: %s",
		EnvStreakCode, streak, cause.Error())

	if err := p.persistOutcome(ctx, t.ID, func(c context.Context) error {
		return p.store.Fail(c, t.ID, claim, reason, backoff, stampedFailureEvidence(sink.Failure(), cause))
	}); err != nil {
		p.log.Error("env-streak burn failed; requeueing instead", "task", t.ID, "streak", streak, "err", err)

		return false
	}

	p.preflightReset(t.ID)
	p.log.Warn("environmental requeue streak burned the attempt",
		"task", t.ID, "streak", streak, "code", EnvStreakCode, "retry after", backoff)

	return true
}

// stampedFailureEvidence rewrites the executor's failure evidence to
// carry the retry-taxonomy class of execErr, so DLQ autopsies and
// forensics read the class instead of re-deriving it from the output
// tail. Decode-remarshal keeps every field the executor set (stage, exit
// code, tail, verify stage); an unparsable or empty evidence degrades to
// a class-only document — the 21:40 §d4 empty-detail deaths at least
// gain the class. Only the worker calls this: the wrapper chain that
// decides the class is complete only here.
func stampedFailureEvidence(evidence jsontext.Value, execErr error) jsontext.Value {
	var fields map[string]any
	if len(evidence) > 0 {
		_ = json.Unmarshal(evidence, &fields) // unparsable → nil map, rebuilt below
	}

	if fields == nil {
		fields = map[string]any{}
	}

	fields["class"] = string(executor.ClassifyFailure(execErr))

	out, err := json.Marshal(fields)
	if err != nil {
		return evidence // keep the executor's evidence over losing it
	}

	return jsontext.Value(out)
}

// persistOutcome retries a store transition write (complete, fail,
// requeue, cancel) that failed with a transient store-busy error. A
// lost transition write orphans the task in Running until lease-expiry
// reclaim — for a completed agent task that is a paid turn executed
// twice — so the write is retried in-process before the reclaim
// backstop absorbs it. SQLITE_BUSY means the statement never ran (the
// lock was refused before execution), so a retry cannot double-apply;
// every write stays guarded by the store's RowsAffected re-checks.
// Non-busy errors pass through untouched: a lost claim or a moved task
// belongs to the reclaim/re-own path, not to this loop.
func (p *Pool) persistOutcome(ctx context.Context, id task.ID, write func(context.Context) error) error {
	return retry.Do(ctx, retry.Config{ //nolint:exhaustruct // optional hooks unset
		MaxAttempts:  3,
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     400 * time.Millisecond,
		Multiplier:   2.0,
		IsRetryable:  isTransientStoreBusy,
		OnRetry: func(attempt int, delay time.Duration, err error) {
			p.log.Warn("outcome write hit a transient store busy; retrying",
				"task", id, "attempt", attempt, "retry after", delay, "err", err)
		},
	}, func(ctx context.Context, _ int) error {
		return write(ctx)
	})
}

// completeAsPreflightDone is the shared done-gate completion write: the
// task completes with a preflight_done detail naming the signal, so the
// review sweeper skips minting a review of a session that never ran and
// audits see WHY the gate fired. Shared by the claim-time gate and the
// gate-slow guard (verify leg died after the work landed).
func (p *Pool) completeAsPreflightDone(ctx context.Context, t task.Task, claim queue.Claim, reason string) {
	// AgentResult marshals structurally; the fallback keeps the gate
	// completing even if a future field stops round-tripping.
	detail, derr := json.Marshal(executor.AgentResult{PreflightDone: reason})
	if derr != nil {
		detail = jsontext.Value(`{"preflight_done":"done preflight"}`)
	}

	if err := p.persistOutcome(ctx, t.ID, func(c context.Context) error {
		return p.store.Complete(c, t.ID, claim, detail)
	}); err != nil {
		p.log.Error("done-gate complete failed", "task", t.ID, "err", err)
	}
}

// isTransientStoreBusy reports whether err is a store lock-contention
// failure worth retrying in-process. The sqlite driver surfaces the
// SQLITE_BUSY family (base 5 and extended 517 BUSY_SNAPSHOT et al) with
// the token in the message, and its English form is "database is
// locked"; matching strings keeps the worker driver-agnostic. A false
// positive is harmless — the retried write is guarded by the store's
// RowsAffected re-checks; a false negative merely falls back to the
// lease-reclaim backstop.
func isTransientStoreBusy(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()

	return strings.Contains(msg, "SQLITE_BUSY") || strings.Contains(msg, "database is locked")
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

	// idle counts consecutive empty claims; any claim (or a hard claim
	// error) resets it so a recovering pool returns to full cadence.
	idle := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			return
		default:
		}

		t, claim, err := p.store.ClaimDue(ctx, p.cfg.Owner, p.cfg.Lease)
		if err != nil {
			if ctx.Err() != nil {
				return // shutdown raced the claim; not an error
			}

			if errors.Is(err, queue.ErrNoTaskDue) {
				idle++
			} else {
				p.log.Error("claim failed", "err", err)
				idle = 0
			}

			next, awake := p.idleWait(ctx, idle)
			if !awake {
				return
			}

			idle = next

			continue
		}

		idle = 0

		p.execute(taskCtx, t, claim)
	}
}

// idleWait parks an idle loop until the earliest of: the idle-gap tick
// (the poll fallback), a claim-wake signal (Config.Wake — the store just
// committed a PENDING landing), or shutdown. A wake resets the idle
// ladder so the pool returns to full cadence instead of grinding against
// the cap; the tick keeps the streak so the ladder keeps backing off. A
// nil Wake never fires (Go's nil-channel rule), which is exactly the
// poll-only fallback. Returns the next idle streak and false when the
// loop must stop (ctx cancelled or pool stopped).
func (p *Pool) idleWait(ctx context.Context, idle int) (int, bool) {
	timer := time.NewTimer(p.cfg.idleGap(idle))
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return idle, false
	case <-p.stopCh:
		return idle, false
	case <-p.cfg.Wake:
		return 0, true
	case <-timer.C:
		return idle, true
	}
}

// execute runs one claimed task under taskCtx. taskCtx survives pool shutdown
// (see Start): execution, heartbeats, and the terminal Complete/Fail write all
// use it, so a draining task keeps its lease and records its outcome. A worker
// that dies anyway loses its lease to expiry reclaim — at-least-once.
func (p *Pool) execute(ctx context.Context, t task.Task, claim queue.Claim) {
	p.mu.Lock()
	p.inFlight[t.ID] = struct{}{}

	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.inFlight, t.ID)
		p.mu.Unlock()
	}()

	// Claim-time spend gate: the budget hook decides BEFORE the executor
	// runs (and before any heartbeat machinery starts) whether this pool
	// may spend another paid turn. A blocked claim is requeued without an
	// attempt burn, parked until the hook's delay (midnight for a daily
	// cap), so work enqueued before the cap bit stays queued instead of
	// costing money after it. No ladder interaction: the block judges
	// neither the task nor the environment, so preflight streak state is
	// left untouched.
	if p.cfg.Budget != nil {
		if blocked, reason, retryIn := p.cfg.Budget(ctx); blocked {
			delay := retryIn
			if delay > 0 {
				delay = rateLimitDelay(delay)
			}

			if err := p.persistOutcome(ctx, t.ID, func(c context.Context) error {
				return p.store.Requeue(c, t.ID, claim, "budget gate: "+reason, delay, false, queue.RequeueClassBudget)
			}); err != nil {
				p.log.Error("budget requeue failed", "task", t.ID, "err", err)
			} else {
				p.log.Warn("budget gate blocked paid turn; requeued without attempt burn",
					"task", t.ID, "retry after", delay.Round(time.Second), "reason", reason)
			}

			return
		}
	}

	// Claim-time done gate: the hook judges whether this dispatch is a
	// re-fire of already-landed work (the 2026-10-02 class: one task,
	// enqueued once, claimed 15× — every claim a paid no-op over its own
	// footer commits). A done verdict completes the task WITHOUT running
	// the executor: zero spend, zero attempt burn, and the completion
	// detail's preflight_done reason keeps the review sweeper from minting
	// a review of a session that never ran.
	if p.cfg.DonePreflight != nil {
		if done, reason := p.cfg.DonePreflight(ctx, t); done {
			p.completeAsPreflightDone(ctx, t, claim, reason)
			p.log.Warn("done preflight: completed without agent run",
				"task", t.ID, "reason", reason)

			return
		}
	}

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
				if err := p.store.Heartbeat(hbCtx, t.ID, claim, p.cfg.Lease); err != nil {
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

	// The refusal ladder survives outcomes that neither judge the task
	// nor leave the loop: the climbing refusals (preflight, undetermined
	// gate failure) keep escalating it, the parked ones (429, owner
	// question) hold it. Every judged outcome — completion, burned or
	// permanent failure, cancellation — forgets it so a later refusal
	// starts fresh.
	if !staysOnLadder(execErr) {
		p.preflightReset(t.ID)
	}

	// Terminal writes (Complete/Fail) use the shutdown-surviving task context,
	// so a draining task's outcome is never orphaned by the cancelled pool.
	terminalCtx := ctx
	if execErr == nil {
		if err := p.persistOutcome(terminalCtx, t.ID, func(c context.Context) error {
			return p.store.Complete(c, t.ID, claim, sink.Detail())
		}); err != nil {
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
			if cerr := p.persistOutcome(ctx, t.ID, func(c context.Context) error {
				return p.store.CancelOwned(c, t.ID, claim)
			}); cerr != nil {
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
		delay, streak := p.preflightDelay(t.ID)
		if p.burnEnvStreak(terminalCtx, t, claim, streak, pre, sink) {
			return
		}

		if err := p.persistOutcome(terminalCtx, t.ID, func(c context.Context) error {
			return p.store.Requeue(c, t.ID, claim, pre.Error(), delay, false, queue.RequeueClassPreflight)
		}); err != nil {
			p.log.Error("requeue failed", "task", t.ID, "err", err)
		} else if p.preflightShouldLog(
			t.ID,
		) {
			p.log.Warn("preflight refused; requeued without attempt burn",
				"task", t.ID, "retry after", delay, "consecutive", streak, "reason", pre.Cause.Error())
		}

		return
	}

	if gate, ok := errors.AsType[*executor.VerifyGateError](execErr); ok {
		// Proven-environmental signature (executor.VerifyGateEnvironmental,
		// the vendor-gofmt verify-gate class): every re-dispatch re-does
		// finished work and dies identically, so dead-letter NOW instead of
		// riding the requeue ladder. The reason text carries the distinct
		// signature code so the rescue sweep can classify the death as
		// environmental without re-deriving it from evidence tails.
		if gate.Class == executor.VerifyGateEnvironmental {
			if err := p.persistOutcome(terminalCtx, t.ID, func(c context.Context) error {
				return p.store.FailPermanent(
					c,
					t.ID,
					claim,
					gate.Error(),
					stampedFailureEvidence(sink.Failure(), execErr),
				)
			}); err != nil {
				p.log.Error("permanent fail failed", "task", t.ID, "err", err)
			} else {
				p.log.Warn("verify gate environmental signature; dead-lettered without re-dispatch",
					"task", t.ID, "signature", executor.VerifyGateEnvCode, "reason", gate.Cause.Error())
			}

			return
		}

		// Gate-slow done guard (row 340): the verify leg died without judging
		// the task, but the run may have LANDED its work first (footer commits,
		// closeout report). A requeue would re-spawn a full agent window over
		// done work — the second-burn receipt class. The done hook decides;
		// done completes mechanically, not-done requeues on the ladder below.
		if p.cfg.DonePreflight != nil {
			if done, reason := p.cfg.DonePreflight(terminalCtx, t); done {
				p.completeAsPreflightDone(terminalCtx, t, claim, reason)
				p.log.Warn("gate-slow guard: verify leg died after the work landed; completed without re-dispatch",
					"task", t.ID, "class", gate.Class, "reason", reason)

				return
			}
		}

		// The verify gate failed WITHOUT judging the task: the gate also
		// fails at the pre-attempt rev (gate-dead: pre-existing or
		// environmental — e.g. a boot-fragile host precondition), or the
		// deadline killed it before a verdict (gate-slow: a retry on a
		// warm cache may simply pass). Requeue WITHOUT burning an attempt
		// on the same escalating ladder as preflight — a sustained gate
		// outage holds its tasks at growing delays instead of DLQ-ing
		// finished work, and the reason rides the requeue fact for
		// alerting.
		delay, streak := p.preflightDelay(t.ID)
		if p.burnEnvStreak(terminalCtx, t, claim, streak, gate, sink) {
			return
		}

		if err := p.persistOutcome(terminalCtx, t.ID, func(c context.Context) error {
			return p.store.Requeue(c, t.ID, claim, gate.Error(), delay, false, queue.RequeueClassGate)
		}); err != nil {
			p.log.Error("requeue failed", "task", t.ID, "err", err)
		} else if p.preflightShouldLog(
			t.ID,
		) {
			p.log.Warn("verify gate failed without judging the task; requeued without attempt burn",
				"task", t.ID, "class", gate.Class, "retry after", delay,
				"consecutive", streak, "reason", gate.Cause.Error())
		}

		return
	}

	if rl, ok := errors.AsType[*executor.RateLimitError](execErr); ok {
		// Provider exhaustion (429 / usage limit): not the task's fault,
		// and the identical retry fails identically until the provider's
		// window resets. Requeue WITHOUT burning an attempt, parked until
		// the parsed reset time (± small jitter so many parked tasks do
		// not reclaim in lockstep and stampede the freshly reset quota).
		delay := rateLimitDelay(rl.RetryAfter)

		if err := p.persistOutcome(terminalCtx, t.ID, func(c context.Context) error {
			return p.store.Requeue(c, t.ID, claim, rl.Error(), delay, rl.ResumeCloseout, queue.RequeueClassRateLimit)
		}); err != nil {
			p.log.Error("rate-limit requeue failed", "task", t.ID, "err", err)
		} else {
			// resume_closeout on the requeue fact says the re-claim resumes
			// the owed close-out turn; the provider tag (16-00 f35) names
			// WHICH provider armed the gate when the output carried one.
			attrs := []any{"task", t.ID, "retry after", delay.Round(time.Second), "reason", rl.Cause.Error()}
			if rl.Provider != "" {
				attrs = append(attrs, "provider", rl.Provider)
			}

			if rl.ResumeCloseout {
				attrs = append(attrs, "resume", "closeout")
			}

			p.log.Warn("provider rate limited; requeued without attempt burn", attrs...)
		}

		return
	}

	if questionErr, ok := errors.AsType[*executor.QuestionPendingError](execErr); ok {
		// Owner question (PapDashboard questions): the agent parked the run
		// awaiting a ruling. The identical retry is pointless until the
		// answer arrives — requeue WITHOUT burning an attempt, parked until
		// the question expires (the safety valve re-enters the task when
		// the answer never comes). No jitter: the expiry is the answer
		// deadline, not a quota window.
		if err := p.persistOutcome(terminalCtx, t.ID, func(c context.Context) error {
			return p.store.Requeue(
				c,
				t.ID,
				claim,
				questionErr.Error(),
				questionErr.RetryAfter,
				questionErr.ResumeCloseout,
				queue.RequeueClassQuestion,
			)
		}); err != nil {
			p.log.Error("question requeue failed", "task", t.ID, "err", err)
		} else {
			attrs := []any{
				"task", t.ID,
				"retry after", questionErr.RetryAfter.Round(time.Second),
				"question", questionErr.Cause.Error(),
			}
			if questionErr.ResumeCloseout {
				attrs = append(attrs, "resume", "closeout")
			}

			p.log.Warn("parked on owner question; requeued without attempt burn", attrs...)
		}

		return
	}

	if perm, ok := errors.AsType[*executor.PermanentError](execErr); ok {
		// The identical retry would fail identically (bad payload, missing
		// repo). Dead-letter now instead of burning the retry budget — for
		// agent tasks every retry is real money.
		if err := p.persistOutcome(terminalCtx, t.ID, func(c context.Context) error {
			return p.store.FailPermanent(c, t.ID, claim, perm.Error(), stampedFailureEvidence(sink.Failure(), execErr))
		}); err != nil {
			p.log.Error("permanent fail failed", "task", t.ID, "err", err)
		}

		return
	}

	if errors.Is(execErr, context.Canceled) && ctx.Err() != nil {
		// Task context cancelled mid-run (defensive: the task context ignores
		// pool shutdown; only internal cancellation lands here). Burn the
		// attempt (crash-safe equivalent) with zero backoff so it is immediately
		// reclaimable.
		if err := p.persistOutcome(terminalCtx, t.ID, func(c context.Context) error {
			return p.store.Fail(
				c,
				t.ID,
				claim,
				"worker shutdown: "+execErr.Error(),
				0,
				stampedFailureEvidence(sink.Failure(), execErr),
			)
		}); err != nil {
			p.log.Error("fail-on-shutdown failed", "task", t.ID, "err", err)
		}

		return
	}

	// Failure evidence (exit code, output tail — executor.FailureEvidence)
	// rides the task.failed fact's detail so a failed attempt is debuggable
	// from the journal alone (21:40 report §d4: both retry-path failures
	// left empty {} detail).
	if err := p.persistOutcome(terminalCtx, t.ID, func(c context.Context) error {
		return p.store.Fail(
			c,
			t.ID,
			claim,
			execErr.Error(),
			p.cfg.Backoff(t.Attempts+1),
			stampedFailureEvidence(sink.Failure(), execErr),
		)
	}); err != nil {
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
