package runactor

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestFirstActorExitCancelsTheRest(t *testing.T) {
	g := New(context.Background())

	ended := make(chan struct{})

	g.Go("decider", func(ctx context.Context) error {
		return nil // first actor to return decides: graceful stop
	})

	g.Go("follower", func(ctx context.Context) error {
		<-ctx.Done()
		close(ended)

		return nil
	})

	if err := g.Run(); err != nil {
		t.Fatalf("graceful actor exit: Run = %v, want nil", err)
	}

	select {
	case <-ended:
	default:
		t.Fatal("follower actor was not cancelled by the decider's exit")
	}
}

func TestActorErrorIsTheCause(t *testing.T) {
	g := New(context.Background())

	boom := errors.New("bind failed")

	g.Go("http", func(ctx context.Context) error { return boom })
	g.Go("tail", func(ctx context.Context) error {
		<-ctx.Done()

		return nil
	})

	err := g.Run()
	if err == nil || !errors.Is(err, boom) {
		t.Fatalf("Run = %v, want the http actor's error", err)
	}

	cause, ok := errors.AsType[ExitCause](err)
	if !ok || cause.Actor != "http" {
		t.Fatalf("cause = %+v, want ExitCause{http}", cause)
	}
}

func TestTeardownRunsLIFO(t *testing.T) {
	g := New(context.Background())

	var order []string

	g.OnShutdown(func() error {
		order = append(order, "store")

		return nil
	})
	g.OnShutdown(func() error {
		order = append(order, "http")

		return nil
	})

	g.Go("act", func(ctx context.Context) error { return nil })

	if err := g.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(order) != 2 || order[0] != "http" || order[1] != "store" {
		t.Fatalf("teardown order = %v, want [http store] (LIFO)", order)
	}
}

func TestTeardownErrorsSurface(t *testing.T) {
	g := New(context.Background())

	g.OnShutdown(func() error { return errors.New("close failed") })
	g.Go("act", func(ctx context.Context) error { return nil })

	if err := g.Run(); err == nil || err.Error() != "close failed" {
		t.Fatalf("Run = %v, want the teardown error", err)
	}
}

// TestInterruptCancelsGracefully lives in runactor_unix_test.go (it sends a
// real SIGTERM via syscall.Kill — POSIX only).

// TestExecutionScopeSurvivesShutdown pins THE invariant: the pool's
// shutdown context must never cancel an in-flight task's execution scope.
func TestExecutionScopeSurvivesShutdown(t *testing.T) {
	shutdown, cancelShutdown := context.WithCancel(context.Background())

	scope, done := ExecutionScope(shutdown, 500*time.Millisecond)
	defer done()

	cancelShutdown() // Ctrl-C fires while the task is mid-flight

	select {
	case <-scope.Done():
		t.Fatal("execution scope cancelled by shutdown — the stranded-in-running bug")
	case <-time.After(50 * time.Millisecond):
	}

	deadline, ok := scope.Deadline()
	if !ok || time.Until(deadline) > 500*time.Millisecond {
		t.Fatal("execution scope is not bounded by its own timeout")
	}

	done()
	<-scope.Done() // bounded only by the timeout/cancel of the scope itself
}
