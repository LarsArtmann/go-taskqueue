package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func testStore(t *testing.T) queue.Store {
	t.Helper()

	s, err := queue.OpenSQLite(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	return s
}

func quietLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// waitFor polls until the task reaches a terminal status or the deadline hits.
func waitFor(t *testing.T, ctx context.Context, store queue.Store, id task.ID, want ...task.Status) task.Task {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		got, err := store.Get(ctx, id)
		if err != nil {
			t.Fatalf("get: %v", err)
		}

		if slices.Contains(want, got.Status) {
			return got
		}

		time.Sleep(5 * time.Millisecond)
	}

	got, _ := store.Get(ctx, id)
	t.Fatalf("task %s never reached %v (status=%s)", id, want, got.Status)

	return got
}

func TestEndToEnd(t *testing.T) {
	store := testStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := executor.NewRegistry()

	var ran atomic.Int32

	reg.RegisterFunc("greet", func(context.Context, task.Task) error {
		ran.Add(1)

		return nil
	})

	enq, err := store.Enqueue(ctx, task.New{Project: "go-taskqueue", Type: "greet"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	pool := New(store, Config{
		Concurrency: 1, PollInterval: 5 * time.Millisecond, TaskTimeout: 2 * time.Second, Executors: reg,
	}, quietLog())
	go func() { _ = pool.Start(ctx) }()

	waitFor(t, ctx, store, enq.ID, task.Completed)
	cancel()

	if ran.Load() != 1 {
		t.Fatalf("executor ran %d times, want 1", ran.Load())
	}
}

func TestRetryThenComplete(t *testing.T) {
	store := testStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := executor.NewRegistry()

	var attempts atomic.Int32

	reg.RegisterFunc("flaky", func(context.Context, task.Task) error {
		if attempts.Add(1) < 3 {
			return errors.New("transient")
		}

		return nil
	})

	enq, _ := store.Enqueue(ctx, task.New{Type: "flaky", MaxAttempts: 5})

	pool := New(store, Config{
		Concurrency: 1, PollInterval: 5 * time.Millisecond, TaskTimeout: 2 * time.Second,
		Executors: reg,
		Backoff:   func(int) time.Duration { return 10 * time.Millisecond },
	}, quietLog())
	go func() { _ = pool.Start(ctx) }()

	waitFor(t, ctx, store, enq.ID, task.Completed, task.Dead)
	cancel()

	got, _ := store.Get(context.Background(), enq.ID)
	if got.Status != task.Completed {
		t.Fatalf("status = %s (attempts %d), want completed", got.Status, got.Attempts)
	}

	if attempts.Load() != 3 {
		t.Fatalf("attempts = %d, want 3", attempts.Load())
	}
}

func TestPanicRecovery(t *testing.T) {
	store := testStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := executor.NewRegistry()
	reg.RegisterFunc("panic", func(context.Context, task.Task) error {
		panic("boom")
	})

	enq, _ := store.Enqueue(ctx, task.New{Type: "panic", MaxAttempts: 1})

	pool := New(store, Config{
		Concurrency: 1, PollInterval: 5 * time.Millisecond, TaskTimeout: 2 * time.Second,
		Executors: reg, Backoff: func(int) time.Duration { return 0 },
	}, quietLog())
	go func() { _ = pool.Start(ctx) }()

	waitFor(t, ctx, store, enq.ID, task.Dead)
	cancel()

	got, _ := store.Get(context.Background(), enq.ID)
	if !strings.Contains(got.LastError, "panicked") {
		t.Fatalf("lastError = %q, want panic marker", got.LastError)
	}
}

func TestExactlyOnceUnderConcurrency(t *testing.T) {
	store := testStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const n = 20

	reg := executor.NewRegistry()

	var mu sync.Mutex

	runs := make(map[string]int)

	reg.RegisterFunc("work", func(_ context.Context, tk task.Task) error {
		mu.Lock()
		runs[tk.ID.String()]++
		mu.Unlock()
		time.Sleep(2 * time.Millisecond)

		return nil
	})

	for i := range n {
		if _, err := store.Enqueue(ctx, task.New{Project: fmt.Sprintf("p%d", i%3), Type: "work"}); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	}

	pool := New(store, Config{
		Concurrency: 4, PollInterval: 5 * time.Millisecond, TaskTimeout: 5 * time.Second, Executors: reg,
	}, quietLog())
	go func() { _ = pool.Start(ctx) }()

	// Wait until all n are terminal.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		tasks, _ := store.List(context.Background(), queue.Filter{})
		done := 0

		for _, tk := range tasks {
			if tk.Status == task.Completed {
				done++
			}
		}

		if done == n {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	cancel()

	got, _ := store.List(context.Background(), queue.Filter{})
	done := 0

	for _, tk := range got {
		if tk.Status == task.Completed {
			done++
		}
	}

	if done != n {
		t.Fatalf("%d/%d completed", done, n)
	}

	if len(runs) != n {
		t.Fatalf("%d distinct tasks ran, want %d", len(runs), n)
	}

	for id, c := range runs {
		if c != 1 {
			t.Fatalf("task %s ran %d times, want 1", id, c)
		}
	}
}

func TestLeaseLostMidExecution(t *testing.T) {
	store := testStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := executor.NewRegistry()
	started := make(chan struct{})

	reg.RegisterFunc("slow", func(c context.Context, _ task.Task) error {
		close(started)
		<-c.Done() // run until our context dies

		return c.Err()
	})

	enq, _ := store.Enqueue(ctx, task.New{Type: "slow", MaxAttempts: 3})
	pool := New(store, Config{
		Concurrency: 1, PollInterval: 5 * time.Millisecond, TaskTimeout: 10 * time.Second,
		// Heartbeat slower than lease: the lease expires mid-run on purpose.
		Lease: 80 * time.Millisecond, Heartbeat: 500 * time.Millisecond,
		Executors: reg,
	}, quietLog())

	go func() { _ = pool.Start(ctx) }()

	<-started

	time.Sleep(150 * time.Millisecond) // lease now expired

	stolen, err := store.ClaimDue(ctx, "thief", time.Minute)
	if err != nil {
		t.Fatalf("steal claim: %v", err)
	}

	if stolen.ID != enq.ID {
		t.Fatalf("stole wrong task %s, want %s", stolen.ID, enq.ID)
	}

	if err := store.Complete(ctx, stolen.ID, "thief", nil); err != nil {
		t.Fatalf("thief complete: %v", err)
	}

	got, _ := store.Get(context.Background(), enq.ID)
	if got.Status != task.Completed {
		t.Fatalf("status = %s, want completed", got.Status)
	}

	cancel()

	// The pool must not have recorded a spurious failure for the stolen task.
	// (The worker detects the dead heartbeat context on next tick; give it a
	// moment, then assert attempts stayed at the thief's view.)
	time.Sleep(50 * time.Millisecond)

	got, _ = store.Get(context.Background(), enq.ID)
	if got.Status != task.Completed {
		t.Fatalf("status after settle = %s, want completed", got.Status)
	}
}

func TestShutdownDrains(t *testing.T) {
	store := testStore(t)
	ctx, cancel := context.WithCancel(context.Background())

	reg := executor.NewRegistry()

	var (
		mu         sync.Mutex
		claimedIDs []task.ID
	)

	firstClaim := make(chan struct{})

	reg.RegisterFunc("job", func(_ context.Context, tk task.Task) error {
		mu.Lock()
		if len(claimedIDs) == 0 {
			close(firstClaim)
		}

		claimedIDs = append(claimedIDs, tk.ID)
		mu.Unlock()
		time.Sleep(80 * time.Millisecond)

		return nil
	})

	for range 3 {
		if _, err := store.Enqueue(ctx, task.New{Type: "job"}); err != nil {
			t.Fatalf("enqueue: %v", err)
		}
	}

	pool := New(store, Config{
		Concurrency: 3, PollInterval: 5 * time.Millisecond, TaskTimeout: 5 * time.Second, Executors: reg,
	}, quietLog())
	runDone := make(chan struct{})

	go func() { _ = pool.Start(ctx); close(runDone) }()

	<-firstClaim // a task is claimed and executing; shutdown now races the drain
	cancel()     // graceful stop
	<-runDone    // pool fully drained before store cleanup

	// Whatever was claimed before shutdown must reach a terminal state; the
	// pool may not exit with its in-flight work unresolved.
	mu.Lock()
	claimed := slices.Clone(claimedIDs)
	mu.Unlock()

	if len(claimed) == 0 {
		t.Fatal("no task was ever claimed")
	}

	for _, id := range claimed {
		waitFor(t, context.Background(), store, id, task.Completed, task.Dead)
	}

	// Global invariant: none are stuck 'running' after the pool exits.
	tasks, _ := store.List(context.Background(), queue.Filter{})
	for _, tk := range tasks {
		if tk.Status == task.Running {
			t.Fatalf("task %s stuck running after pool exit", tk.ID)
		}
	}
}

// TestLongTaskCompletesAcrossShutdown is the regression test for the drain
// bug that shipped: pool shutdown used to cancel the task context, so any
// task running longer than a few seconds was orphaned (no outcome, lease
// left to expiry). The contract now: shutdown must NOT cancel an executing
// task; its terminal outcome is still recorded after the pool is gone.
func TestLongTaskCompletesAcrossShutdown(t *testing.T) {
	store := testStore(t)
	ctx, cancel := context.WithCancel(context.Background())

	reg := executor.NewRegistry()
	execStarted := make(chan struct{})
	release := make(chan struct{})

	var sawCtxCancelled atomic.Bool

	reg.RegisterFunc("long", func(c context.Context, _ task.Task) error {
		close(execStarted)

		select {
		case <-release:
		case <-c.Done():
			sawCtxCancelled.Store(true)
		}

		return nil
	})

	enq, _ := store.Enqueue(ctx, task.New{Type: "long", MaxAttempts: 1})
	pool := New(store, Config{
		Concurrency: 1, PollInterval: 5 * time.Millisecond, TaskTimeout: 10 * time.Second,
		Executors: reg,
	}, quietLog())
	runDone := make(chan struct{})

	go func() { _ = pool.Start(ctx); close(runDone) }()

	<-execStarted // task claimed and executing
	cancel()      // graceful shutdown requested while the task is mid-run
	close(release)
	<-runDone // pool exited; the outcome must have been recorded, not dropped

	if sawCtxCancelled.Load() {
		t.Fatal("shutdown cancelled the task context; long tasks would be orphaned")
	}

	got := waitFor(t, context.Background(), store, enq.ID, task.Completed)
	if got.Status != task.Completed {
		t.Fatalf("status = %s, want completed after shutdown", got.Status)
	}
}

// TestPermanentErrorDeadLettersAfterOneAttempt pins the money rule: a
// permanent error must dead-letter after ONE attempt even with budget left,
// because retrying an identical input burns identical agent money for
// nothing.
func TestPermanentErrorDeadLettersAfterOneAttempt(t *testing.T) {
	store := testStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := executor.NewRegistry()

	var ran atomic.Int32

	reg.RegisterFunc("broken", func(context.Context, task.Task) error {
		ran.Add(1)

		return executor.Permanent(errors.New("bad payload shape"))
	})

	enq, _ := store.Enqueue(ctx, task.New{Type: "broken", MaxAttempts: 5})

	pool := New(store, Config{
		Concurrency: 1, PollInterval: 5 * time.Millisecond, TaskTimeout: 2 * time.Second,
		Executors: reg,
	}, quietLog())
	go func() { _ = pool.Start(ctx) }()

	got := waitFor(t, ctx, store, enq.ID, task.Dead)

	cancel()

	if ran.Load() != 1 {
		t.Fatalf("executor ran %d times, want exactly 1", ran.Load())
	}

	if got.Attempts != 1 {
		t.Fatalf("attempts = %d, want 1", got.Attempts)
	}

	if !strings.Contains(got.LastError, "permanent: bad payload shape") {
		t.Fatalf("lastError = %q, want permanent class prefix", got.LastError)
	}
}

// TestFailureEvidenceRidesFailedFact pins the 21:40 §d4 contract end-to-end:
// a failing execution's forensics (stage, exit code, output tail) must land
// on the task.failed fact's detail — not an empty {} — so a failed attempt
// is debuggable from the journal alone.
func TestFailureEvidenceRidesFailedFact(t *testing.T) {
	store := testStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := executor.NewRegistry()
	reg.Register("sh", executor.NewCommandExecutor(""))

	enq, _ := store.Enqueue(ctx, task.New{
		Type:    "sh",
		Payload: []byte(`echo "boom evidence" >&2; exit 7`),
	})

	pool := New(store, Config{
		Concurrency: 1, PollInterval: 5 * time.Millisecond, TaskTimeout: 2 * time.Second,
		Executors: reg,
	}, quietLog())
	go func() { _ = pool.Start(ctx) }()

	waitFor(t, ctx, store, enq.ID, task.Dead) // exit 7 is a permanent failure
	cancel()

	trail, err := store.FactsForTask(context.Background(), enq.ID.String(), 0)
	if err != nil {
		t.Fatalf("facts: %v", err)
	}

	var failed *journal.Fact

	for i := range trail {
		if trail[i].Type == journal.Failed {
			failed = &trail[i]
		}
	}

	if failed == nil {
		t.Fatalf("no task.failed fact in trail of %d facts", len(trail))
	}

	var evidence executor.FailureEvidence
	if err := json.Unmarshal(failed.Detail, &evidence); err != nil {
		t.Fatalf("failed-fact detail is not FailureEvidence JSON: %v (detail=%s)", err, failed.Detail)
	}

	if evidence.Stage != "command" {
		t.Errorf("stage = %q, want command", evidence.Stage)
	}

	if evidence.ExitCode != 7 {
		t.Errorf("exit_code = %d, want 7", evidence.ExitCode)
	}

	if !strings.Contains(evidence.Tail, "boom evidence") {
		t.Errorf("tail = %q, want the failing output excerpt", evidence.Tail)
	}
}

// TestUnknownTaskTypeDeadLettersImmediately: no registered executor can ever
// appear mid-retry, so an unknown type must not exhaust the attempt budget.
func TestUnknownTaskTypeDeadLettersImmediately(t *testing.T) {
	store := testStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool := New(store, Config{
		Concurrency: 1, PollInterval: 5 * time.Millisecond, TaskTimeout: 2 * time.Second,
		Executors: executor.NewRegistry(),
	}, quietLog())
	go func() { _ = pool.Start(ctx) }()

	enq, _ := store.Enqueue(ctx, task.New{Type: "mystery", MaxAttempts: 9})
	got := waitFor(t, ctx, store, enq.ID, task.Dead)

	cancel()

	if got.Attempts != 1 {
		t.Fatalf("attempts = %d, want 1 (unknown type is permanent)", got.Attempts)
	}
}

// TestPreflightRequeuesWithoutAttemptBurn pins the preflight contract:
// a *PreflightError must NOT dead-letter, NOT burn an attempt, and NOT
// retry immediately — the task goes back to pending and is claimable again
// after the preflight backoff, so the pool picks it up once the human
// commits or adds the missing config.
func TestPreflightRequeuesWithoutAttemptBurn(t *testing.T) {
	store := testStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := executor.NewRegistry()

	var (
		ready atomic.Bool
		ran   atomic.Int32
	)

	reg.RegisterFunc("env", func(context.Context, task.Task) error {
		if !ready.Load() {
			return &executor.PreflightError{Cause: errors.New("repo dirty; human still working")}
		}

		ran.Add(1)

		return nil
	})

	enq, _ := store.Enqueue(ctx, task.New{Type: "env", MaxAttempts: 1})

	pool := New(store, Config{
		Concurrency: 1, PollInterval: 5 * time.Millisecond, TaskTimeout: 2 * time.Second,
		PreflightBackoff: 120 * time.Millisecond,
		Executors:        reg,
	}, quietLog())
	go func() { _ = pool.Start(ctx) }()

	// The first pass refuses (preflight); the task must stay pending with
	// zero attempts, not dead despite MaxAttempts=1.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := store.Get(context.Background(), enq.ID)
		if got.LastError != "" && got.Status == task.Pending && got.Attempts == 0 {
			break
		}

		time.Sleep(5 * time.Millisecond)
	}

	first, _ := store.Get(context.Background(), enq.ID)
	if first.Status != task.Pending || first.Attempts != 0 {
		t.Fatalf("after preflight refusal: status=%s attempts=%d, want pending/0", first.Status, first.Attempts)
	}

	// Environment fixed: the very same task completes without any rescue.
	ready.Store(true)

	got := waitFor(t, ctx, store, enq.ID, task.Completed)
	if ran.Load() != 1 {
		t.Fatalf("executor ran %d times after fix, want 1", ran.Load())
	}

	if got.Status != task.Completed {
		t.Fatalf("status = %s, want completed", got.Status)
	}

	cancel()
}

// TestCooperativeCancelMidRun drives the full cooperative-cancel loop: an
// operator requests the cancel while the executor is mid-run, the heartbeat
// observes the fact, the execution context is cancelled, and the task
// finalizes as Cancelled (no attempt burned, no retry).
func TestCooperativeCancelMidRun(t *testing.T) {
	store := testStore(t)

	ctx := t.Context()

	reg := executor.NewRegistry()

	started := make(chan struct{})

	var ran atomic.Int32

	reg.RegisterFunc("loop", func(c context.Context, _ task.Task) error {
		ran.Add(1)
		close(started)
		<-c.Done() // a long-running task that stops when cancelled

		return c.Err()
	})

	enq, err := store.Enqueue(ctx, task.New{Project: "p", Type: "loop"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	pool := New(store, Config{
		Concurrency: 1, PollInterval: 5 * time.Millisecond, Heartbeat: 10 * time.Millisecond,
		Lease: 2 * time.Second, TaskTimeout: 30 * time.Second, Executors: reg,
	}, quietLog())

	go func() { _ = pool.Start(ctx) }()

	<-started
	waitFor(t, ctx, store, enq.ID, task.Running)

	if err := store.CancelRunning(ctx, enq.ID, ""); err != nil {
		t.Fatalf("CancelRunning: %v", err)
	}

	got := waitFor(t, ctx, store, enq.ID, task.Cancelled)

	if ran.Load() != 1 {
		t.Fatalf("executor ran %d times, want exactly 1 (cancel must not retry)", ran.Load())
	}

	if got.Attempts != 0 {
		t.Fatalf("attempts = %d, want 0 (a cancelled task burns nothing)", got.Attempts)
	}

	facts, err := store.FactsForTask(ctx, enq.ID.String(), 0)
	if err != nil {
		t.Fatalf("FactsForTask: %v", err)
	}

	sawRequested, sawCancelled := false, false
	for _, f := range facts {
		sawRequested = sawRequested || f.Type == journal.CancelRequested
		sawCancelled = sawCancelled || f.Type == journal.Cancelled
	}

	if !sawRequested || !sawCancelled {
		t.Fatalf("journal missing cancel facts (requested=%v cancelled=%v)", sawRequested, sawCancelled)
	}
}

// TestHeartbeatDefaultTighterThanHalfLease pins the cadence contract: the
// default heartbeat must renew well before the lease dies (currently
// lease/4 — tighter than the lease/3 the round-5 plan asked for) and stays
// configurable via Config.Heartbeat.
func TestHeartbeatDefaultTighterThanHalfLease(t *testing.T) {
	lease := 4 * time.Second

	pool := New(testStore(t), Config{Lease: lease}, quietLog())
	if pool.cfg.Heartbeat <= 0 || pool.cfg.Heartbeat > lease/3 {
		t.Fatalf("default heartbeat = %v, want > 0 and <= lease/3 (%v)", pool.cfg.Heartbeat, lease/3)
	}

	// The knob overrides: slow agents with long leases can pace renewals.
	pool = New(testStore(t), Config{Lease: time.Hour, Heartbeat: 5 * time.Minute}, quietLog())
	if pool.cfg.Heartbeat != 5*time.Minute {
		t.Fatalf("explicit heartbeat = %v, want preserved", pool.cfg.Heartbeat)
	}
}
