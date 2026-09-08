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
	// FailPermanent dead-letters a Running task immediately, regardless of
	// the attempt budget: the error class makes retrying pointless. The
	// attempt is still counted. Facts: task.failed + task.dead-lettered
	// (class "permanent").
	FailPermanent(ctx context.Context, id task.ID, owner string, errText string) error
	// Requeue returns a claimed task to Pending WITHOUT counting an
	// attempt; it becomes claimable again after delay. For preflight
	// refusals: the environment was not ready, not the task. Facts:
	// task.requeued.
	Requeue(ctx context.Context, id task.ID, owner string, errText string, delay time.Duration) error
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
	// Facts exposes the journal (same store, same transaction domain):
	// facts with Seq strictly greater than after, in Seq order. limit
	// bounds the result when > 0; 0 means unbounded (bulk exports).
	Facts(ctx context.Context, after int64, limit int) ([]journal.Fact, error)
	// LastFacts returns the most recent limit facts in ascending Seq
	// order — the bounded read behind feed-style renders. limit <= 0
	// returns the whole journal.
	LastFacts(ctx context.Context, limit int) ([]journal.Fact, error)
	// HeadSeq returns the current highest fact Seq (0 when the journal is
	// empty): the O(1) watermark for tailers, bridges and resume points.
	HeadSeq(ctx context.Context) (int64, error)
	// FactsForTask returns one task's facts in Seq order, bounded to the
	// most recent limit when > 0 (0 = unbounded).
	FactsForTask(ctx context.Context, id string, limit int) ([]journal.Fact, error)
	// CountFacts counts facts of one type recorded at or after since —
	// the SQL pushdown behind spend projections and stats.
	CountFacts(ctx context.Context, ftype journal.FactType, since time.Time) (int64, error)
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
