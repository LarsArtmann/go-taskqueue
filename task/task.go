// Package task is the public facade over the task lifecycle module
// (ADR-0016): the Task record, the Status state machine, and the sentinel
// errors. The implementation stays internal; this package pins the names.
package task

import internaltask "github.com/larsartmann/go-taskqueue/internal/task"

// Task lifecycle.
type (
	Task   = internaltask.Task
	New    = internaltask.New
	ID     = internaltask.ID
	Status = internaltask.Status
)

// Status values.
const (
	Pending   = internaltask.Pending
	Running   = internaltask.Running
	Completed = internaltask.Completed
	Dead      = internaltask.Dead
	Cancelled = internaltask.Cancelled
)

// Retry default.
const DefaultMaxAttempts = internaltask.DefaultMaxAttempts

// Sentinel errors.
var (
	ErrNotFound          = internaltask.ErrNotFound
	ErrLeaseNotHeld      = internaltask.ErrLeaseNotHeld
	ErrDuplicateID       = internaltask.ErrDuplicateID
	ErrInvalidTransition = internaltask.ErrInvalidTransition
)

// Lifecycle helpers.
var (
	AllStatuses     = internaltask.AllStatuses
	CanTransitionTo = internaltask.CanTransitionTo
	Terminal        = internaltask.Terminal
	NewID           = internaltask.NewID
)
