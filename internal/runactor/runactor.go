// Package runactor composes long-running components under one signal
// story: named actors run concurrently, the first exit (or failure) ends
// the whole group, teardown runs LIFO, and an interrupt actor turns
// SIGINT/SIGTERM into graceful cancellation — a second signal exits hard.
//
// Stdlib + golang.org/x/sync/errgroup only (ADR-0004's framework-free
// stance). This is the pattern steal from cordis's run.Group, shaped so
// the drain invariant stays expressible: an actor's context is its
// shutdown signal, NEVER its task-execution context (see ExecutionScope).
package runactor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// ErrInterrupted is the group's cancellation cause when a signal stopped
// it. Run reports nil for a plain interrupt (graceful shutdown), so this
// exists for callers that need to distinguish causes via errors.Is on
// context.Cause.
var ErrInterrupted = errors.New("interrupted")

// ExitCause explains why a group ended; it names the actor whose exit
// decided it.
type ExitCause struct {
	Actor string
	Err   error
}

func (e ExitCause) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("%s exited", e.Actor)
	}

	return fmt.Sprintf("%s: %v", e.Actor, e.Err)
}

func (e ExitCause) Unwrap() error { return e.Err }

// Group runs named actors until one of them returns, then cancels the
// rest and tears down in reverse registration order.
type Group struct {
	ctx    context.Context
	cancel context.CancelCauseFunc
	eg     *errgroup.Group

	mu       sync.Mutex
	teardown []func() error
	doneOnce sync.Once
	done     chan struct{}
}

// New builds a Group over parent. Call Go/InterruptOn/OnShutdown, then Run.
func New(parent context.Context) *Group {
	ctx, cancel := context.WithCancelCause(parent)

	return &Group{
		ctx:    ctx,
		cancel: cancel,
		eg:     &errgroup.Group{},
		done:   make(chan struct{}),
	}
}

// Ctx is the actors' shutdown context: cancelled when the group ends
// (signal, actor exit, or parent death).
func (g *Group) Ctx() context.Context { return g.ctx }

// Go runs fn as a named actor. The FIRST actor to return decides the
// program: an error is the group's cause (surfaced by Run); a clean
// return while the group is still healthy is a graceful stop.
func (g *Group) Go(name string, fn func(ctx context.Context) error) {
	g.eg.Go(func() error {
		err := fn(g.ctx)

		if g.ctx.Err() != nil {
			// The group is already shutting down; a Canceled error here
			// is noise, not the cause.
			return nil
		}

		if err == nil {
			g.cancel(context.Canceled)

			return nil
		}

		wrapped := fmt.Errorf("%s: %w", name, err)
		g.cancel(ExitCause{Actor: name, Err: err})

		return wrapped
	})
}

// OnShutdown registers a teardown step; steps run LIFO after every actor
// has stopped. Use for Close-ordered resources (store last to close).
func (g *Group) OnShutdown(fn func() error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.teardown = append(g.teardown, fn)
}

// InterruptOn cancels the group on any of the signals; a second signal
// exits the process hard (exit code 130) — the operator's escape hatch
// when graceful drain hangs.
func (g *Group) InterruptOn(sig ...os.Signal) {
	ch := make(chan os.Signal, 2)
	signal.Notify(ch, sig...)

	go func() {
		defer signal.Stop(ch)

		select {
		case <-g.ctx.Done():
			return
		case <-ch:
		}

		g.cancel(ErrInterrupted)

		select {
		case <-ch:
			os.Exit(130)
		case <-g.done:
		}
	}()
}

// Run blocks until every actor has returned, then tears down LIFO. It
// returns the decisive error (nil for a signal or clean exit), joined
// with any teardown errors.
func (g *Group) Run() error {
	waitErr := g.eg.Wait()

	// Release ctx holders (including the interrupt watcher via done).
	g.cancel(context.Canceled)

	g.doneOnce.Do(func() { close(g.done) })

	g.mu.Lock()
	teardown := append([]func() error(nil), g.teardown...)
	g.mu.Unlock()

	var teardownErr error

	for i := len(teardown) - 1; i >= 0; i-- {
		if err := teardown[i](); err != nil {
			teardownErr = errors.Join(teardownErr, err)
		}
	}

	if cause := context.Cause(g.ctx); cause != nil {
		var exitCause ExitCause

		switch {
		case errors.As(cause, &exitCause):
			return errors.Join(exitCause, teardownErr)
		case errors.Is(cause, context.Canceled), errors.Is(cause, ErrInterrupted):
			return teardownErr
		default:
			return errors.Join(cause, teardownErr)
		}
	}

	return errors.Join(waitErr, teardownErr)
}

// ExecutionScope derives a task-execution context that shutdown NEVER
// cancels — bounded only by the per-task deadline. This makes the drain
// invariant structural (the stranded-in-running lesson): a Ctrl-C lets
// in-flight tasks finish and record their outcome; the pool's shutdown
// context stops the LOOP, not the work.
func ExecutionScope(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), timeout)
}
