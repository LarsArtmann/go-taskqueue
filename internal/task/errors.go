package task

import "errors"

var (
	// ErrNotFound is returned when a task ID does not exist.
	ErrNotFound = errors.New("task: not found")
	// ErrLeaseNotHeld is returned when completing/failing/heartbeating a task
	// whose lease is owned by someone else or already expired.
	ErrLeaseNotHeld = errors.New("task: lease not held")
	// ErrDuplicateID is returned when enqueueing a task whose ID already exists.
	ErrDuplicateID = errors.New("task: duplicate id")
	// ErrInvalidTransition is returned by stores that validate transitions.
	ErrInvalidTransition = errors.New("task: invalid status transition")
)
