// Package worker implements the claim→heartbeat→execute→complete loop.
package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
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
// drains in-flight tasks and returns. Store writes during drain use a fresh
// background context so a cancelled parent cannot orphan a running task's
// terminal state.
func (p *Pool) Start(ctx context.Context) error {
	drainCtx, drainCancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer drainCancel()
	for range p.cfg.Concurrency {
		p.wg.Add(1)
		go p.loop(ctx, drainCtx)
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

func (p *Pool) loop(ctx, drainCtx context.Context) {
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
			if !errors.Is(err, queue.ErrNoTaskDue) {
				p.log.Error("claim failed", "err", err)
			}
			if !sleepCtx(ctx, p.cfg.PollInterval) {
				return
			}
			continue
		}
		p.execute(drainCtx, t)
	}
}

func (p *Pool) execute(ctx context.Context, t task.Task) {
	p.mu.Lock()
	p.inFlight[t.ID] = struct{}{}
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.inFlight, t.ID)
		p.mu.Unlock()
	}()

	// Heartbeat ticker: extends the lease while execution runs.
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
			}
		}
	}()

	execErr := p.runExecutor(hbCtx, t)
	hbCancel()
	<-hbDone

	if execErr == nil {
		if err := p.store.Complete(ctx, t.ID, p.cfg.Owner, nil); err != nil {
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
	if errors.Is(execErr, context.Canceled) && ctx.Err() != nil {
		// Pool shutting down mid-task: release without burning an attempt is
		// not supported by Fail's contract; burn the attempt (crash-safe
		// equivalent) with a zero backoff so it is immediately reclaimable.
		if err := p.store.Fail(ctx, t.ID, p.cfg.Owner, "worker shutdown: "+execErr.Error(), 0); err != nil {
			p.log.Error("fail-on-shutdown failed", "task", t.ID, "err", err)
		}
		return
	}
	if err := p.store.Fail(ctx, t.ID, p.cfg.Owner, execErr.Error(), p.cfg.Backoff(t.Attempts+1)); err != nil {
		p.log.Error("fail failed", "task", t.ID, "err", err)
	}
}

func (p *Pool) runExecutor(ctx context.Context, t task.Task) error {
	exec, err := p.cfg.Executors.Lookup(t.Type)
	if err != nil {
		// Unknown type is a permanent error: dead-letter fast via huge
		// attempts marker is not in v0.1.0; retries will exhaust quickly.
		return fmt.Errorf("no executor for type %q: %w", t.Type, err)
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
