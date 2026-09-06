// Package executor defines the pluggable execution layer: how a claimed task
// actually runs.
package executor

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// ErrUnknownType is returned by Registry.Lookup for unregistered task types.
var ErrUnknownType = errors.New("executor: unknown task type")

// LeaseLostError signals the worker lost its lease while executing and must
// not record a failure for the task.
type LeaseLostError struct{ Cause error }

func (e *LeaseLostError) Error() string { return "executor: lease lost: " + e.Cause.Error() }
func (e *LeaseLostError) Unwrap() error { return e.Cause }

// PermanentError marks a failure that retrying can never fix: the identical
// input will fail identically (bad payload, missing repo, dirty tree, missing
// autonomy). Workers must dead-letter the task after the first attempt
// instead of burning the retry budget — for agent tasks every retry is real
// money.
//
// Detect it with errors.AsType / errors.Is; executors should return it via
// Permanent so wrapping is uniform.
type PermanentError struct{ Cause error }

func (e *PermanentError) Error() string { return "permanent: " + e.Cause.Error() }
func (e *PermanentError) Unwrap() error { return e.Cause }

// Permanent wraps cause in a *PermanentError; nil passes through unchanged.
// A cause that is already permanent is returned as-is, so defensive double
// wrapping stays a single class.
func Permanent(cause error) error {
	if cause == nil {
		return nil
	}
	if _, ok := cause.(*PermanentError); ok {
		return cause
	}
	return &PermanentError{Cause: cause}
}

// Executor runs one claimed task. It must be safe for concurrent use.
// Returning nil marks the task completed; any error marks a failed attempt.
// Return a *PermanentError (see Permanent) when retrying can never help.
type Executor interface {
	Execute(ctx context.Context, t task.Task) error
}

// Func adapts a plain function to Executor.
type Func func(ctx context.Context, t task.Task) error

// Execute calls f.
func (f Func) Execute(ctx context.Context, t task.Task) error { return f(ctx, t) }

// Registry maps task types to executors.
type Registry struct {
	mu    sync.RWMutex
	execs map[string]Executor
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{execs: make(map[string]Executor)} }

// Register binds a task type to an executor.
func (r *Registry) Register(taskType string, e Executor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.execs[taskType] = e
}

// RegisterFunc binds a task type to a function.
func (r *Registry) RegisterFunc(taskType string, f Func) { r.Register(taskType, f) }

// Lookup returns the executor for a task type.
func (r *Registry) Lookup(taskType string) (Executor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.execs[taskType]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownType, taskType)
	}
	return e, nil
}

// Types returns the registered task types.
func (r *Registry) Types() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.execs))
	for k := range r.execs {
		out = append(out, k)
	}
	return out
}
