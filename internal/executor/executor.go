// Package executor defines the pluggable execution layer: how a claimed
// task actually runs — sh commands, HTTP calls, and agent turns behind a
// registry keyed by task type (ErrUnknownType for unregistered types).
//
// Viewpoint — Consumer: the worker loop, which looks up an executor per
// claimed task and records whatever outcome it reports; Concern: taking
// one task's payload to a recorded outcome — payload decoding, timeouts,
// rate-limit detection (429s requeue without burn), output-tail secret
// redaction, and the shared seams (decodePayload, recordRunOutcome,
// prepareRepo, payloadTimeout); Non-goals: claiming/scheduling (the
// worker owns the loop) and storage (outcomes become facts via the
// store).
//
// Cross-package map (lifecycle × concern):
// docs/architecture-understanding/2026-10-23_lifecycle-concern-grid.md
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

	if _, ok := errors.AsType[*PermanentError](cause); ok {
		return cause
	}

	return &PermanentError{Cause: cause}
}

// PreflightError signals the executor refused to START a task: the
// environment is not ready (dirty repo waiting on a human commit, autonomy
// config missing, verify tool absent) but neither the task nor the payload
// is wrong. Workers must requeue the task WITHOUT burning an attempt — it
// becomes claimable again after a delay, so the pool picks it up as soon as
// the environment is fixed.
type PreflightError struct{ Cause error }

// ErrDirtyTree rides a dirty-tree refusal's cause chain (assertCleanTree
// wraps it) so the worker can classify the refusal WITHOUT string
// matching: under concurrent-agent co-tenancy the dirt usually belongs to
// ANOTHER session's in-flight files, so the breaker must not spend the
// task's attempts on it — dirty-tree preflights park on the escalating
// ladder instead of burning via the env-streak breaker.
var ErrDirtyTree = errors.New("dirty tree")

// DirtyTree reports whether the refusal was the dirty-working-tree guard
// (as opposed to missing autonomy or a dead toolchain, which a commit
// cannot heal and which stay inside the env-streak breaker's reach).
func (e *PreflightError) DirtyTree() bool { return errors.Is(e.Cause, ErrDirtyTree) }

func (e *PreflightError) Error() string { return "preflight: " + e.Cause.Error() }
func (e *PreflightError) Unwrap() error { return e.Cause }

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
