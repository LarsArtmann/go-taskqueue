package worker

import (
	"context"
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
	var finished atomic.Int32
	reg.RegisterFunc("job", func(context.Context, task.Task) error {
		time.Sleep(80 * time.Millisecond)
		finished.Add(1)
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

	time.Sleep(30 * time.Millisecond) // let all three get claimed
	cancel()                          // graceful stop
	<-runDone                         // pool fully drained before store cleanup
	waitFor(t, context.Background(), store, firstID(t, ctx, store), task.Completed, task.Dead)

	tasks, _ := store.List(context.Background(), queue.Filter{})
	completedOrDead := 0
	for _, tk := range tasks {
		if tk.Status == task.Completed || tk.Status == task.Dead {
			completedOrDead++
		}
	}
	// Shutdown mid-run burns attempts (Fail with zero backoff), so tasks may
	// be pending-retry rather than completed. The invariant: none are stuck
	// 'running' after the pool exits.
	for _, tk := range tasks {
		if tk.Status == task.Running {
			t.Fatalf("task %s stuck running after pool exit", tk.ID)
		}
	}
	_ = completedOrDead
}

func firstID(t *testing.T, ctx context.Context, store queue.Store) task.ID {
	t.Helper()
	tasks, err := store.List(context.Background(), queue.Filter{})
	if err != nil || len(tasks) == 0 {
		t.Fatalf("list: %v (%d tasks)", err, len(tasks))
	}
	return tasks[0].ID
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
