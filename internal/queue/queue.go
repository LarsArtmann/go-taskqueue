// Package queue provides the durable task store and the Queue facade.
//
// The Store interface is the persistence boundary; the sqlite Store is the
// embedded default. Every mutating operation also appends a fact to the
// journal in the same transaction, so the journal is always a complete,
// consistent history of the queue.
package queue

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// ErrNoTaskDue is returned by ClaimDue when nothing is claimable right now.
var ErrNoTaskDue = errors.New("queue: no due task")

// Store is the persistence boundary for tasks and facts.
type Store interface {
	// Enqueue persists a new task (ID and defaults assigned here) and records
	// the task.enqueued fact.
	Enqueue(ctx context.Context, n task.New) (task.Task, error)
	// ClaimDue atomically claims at most one due task for owner: pending,
	// NotBefore passed, all deps completed. Sets Running + lease. Returns
	// ErrNoTaskDue when nothing is claimable.
	ClaimDue(ctx context.Context, owner string, lease time.Duration) (task.Task, error)
	// Complete marks a Running task Completed (lease must be held) and records
	// the task.completed fact.
	Complete(ctx context.Context, id task.ID, owner string, result json.RawMessage) error
	// Fail records a failed attempt. When attempts remain the task returns to
	// Pending with NotBefore = now + backoff(attempt); otherwise it is
	// Dead-lettered. Facts: task.failed (+ task.dead-lettered).
	Fail(ctx context.Context, id task.ID, owner string, errText string, backoff time.Duration) error
	// Heartbeat extends the lease of a Running task held by owner.
	Heartbeat(ctx context.Context, id task.ID, owner string, extend time.Duration) error
	// Cancel withdraws a Pending task.
	Cancel(ctx context.Context, id task.ID) error
	// RescueDead re-queues a Dead task with a fresh attempt budget.
	RescueDead(ctx context.Context, id task.ID, maxAttempts int) error
	// Get returns the current task record.
	Get(ctx context.Context, id task.ID) (task.Task, error)
	// List returns tasks matching the filter.
	List(ctx context.Context, f Filter) ([]task.Task, error)
	// Facts exposes the journal (same store, same transaction domain).
	Facts(ctx context.Context, after int64) ([]journal.Fact, error)
	// Close releases resources.
	Close() error
}

// Filter selects tasks for List.
type Filter struct {
	Project *string
	Status  *task.Status
	Type    *string
	Limit   int
}

// Queue is the facade most consumers use: a Store plus convenience methods.
type Queue struct {
	Store
}

// New wraps a Store.
func New(s Store) *Queue { return &Queue{Store: s} }

// Enqueue normalized-and-enqueues a task.
func (q *Queue) Enqueue(ctx context.Context, n task.New) (task.Task, error) {
	return q.Store.Enqueue(ctx, n.Normalize())
}
