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

// ErrEmptyType is returned by Enqueue when New.Type is empty (Normalize
// cannot invent a type; callers must choose an executor).
var ErrEmptyType = errors.New("queue: task type must not be empty")

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
	// Dead-lettered. Facts: task.failed (+ task.dead-lettered). evidence,
	// when non-empty (executor.FailureEvidence JSON), lands on the
	// task.failed fact's detail — the forensics (exit code, output tail)
	// that make a failed attempt debuggable from the journal alone.
	Fail(ctx context.Context, id task.ID, owner string, errText string, backoff time.Duration, evidence json.RawMessage) error
	// FailPermanent dead-letters a Running task immediately, regardless of
	// the attempt budget: the error class makes retrying pointless. The
	// attempt is still counted. Facts: task.failed (carrying evidence)
	// + task.dead-lettered (class "permanent").
	FailPermanent(ctx context.Context, id task.ID, owner string, errText string, evidence json.RawMessage) error
	// Requeue returns a claimed task to Pending WITHOUT counting an
	// attempt; it becomes claimable again after delay. For preflight
	// refusals: the environment was not ready, not the task. Facts:
	// task.requeued.
	Requeue(ctx context.Context, id task.ID, owner string, errText string, delay time.Duration) error
	// Heartbeat extends the lease of a Running task held by owner.
	Heartbeat(ctx context.Context, id task.ID, owner string, extend time.Duration) error
	// Cancel withdraws a Pending task. A non-empty reason is stored in the
	// task.cancelled fact's detail ("reason" key) so the journal records WHY
	// the task was withdrawn.
	Cancel(ctx context.Context, id task.ID, reason string) error
	// CancelRunning records a cooperative cancel request for a Running
	// task: the task.cancel-requested fact is the flag. The executing
	// worker observes it at its next heartbeat, stops the execution, and
	// finalizes with CancelOwned; an expired lease finalizes it at
	// reclaim. A non-empty reason is stored on the request fact's detail
	// and carried onto the final task.cancelled fact. Idempotent — a
	// second request appends nothing.
	CancelRunning(ctx context.Context, id task.ID, reason string) error
	// CancelRequested reports whether a cooperative cancel request is
	// pending for the task — the worker's heartbeat observation query.
	CancelRequested(ctx context.Context, id task.ID) (bool, error)
	// CancelOwned finalizes a cooperative cancel: Running -> Cancelled,
	// recorded by the lease-holding worker after it stopped the execution.
	CancelOwned(ctx context.Context, id task.ID, owner string) error
	// MarkOrphaned appends a task.orphaned fact for every Running task
	// whose lease expired before the cutoff and that has no orphaned fact
	// yet (idempotent). It changes no state — orphans stay Running until a
	// reclaim — it records WHY the task is stranded so the journal can
	// explain it. Returns how many facts were appended.
	MarkOrphaned(ctx context.Context, cutoff time.Time) (int, error)
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
	// Watermark returns the persisted read cursor for a journal consumer
	// and whether the consumer ever checkpointed (seq 0 is a valid cursor:
	// "consumed nothing yet"): the resume point for bridges and sweepers
	// after a restart.
	Watermark(ctx context.Context, consumer string) (seq int64, exists bool, err error)
	// SaveWatermark checkpoints a consumer cursor as a monotonic upsert
	// (never regresses). It records consumer progress, not task state, so
	// no fact is appended.
	SaveWatermark(ctx context.Context, consumer string, seq int64) error
	// FactsForTask returns one task's facts in Seq order, bounded to the
	// most recent limit when > 0 (0 = unbounded).
	FactsForTask(ctx context.Context, id string, limit int) ([]journal.Fact, error)
	// CountFacts counts facts of one type recorded at or after since —
	// the SQL pushdown behind spend projections and stats.
	CountFacts(ctx context.Context, ftype journal.FactType, since time.Time) (int64, error)
	// StatusCounts counts tasks per status — the GROUP BY behind dashboard
	// counters: O(statuses) work instead of a full task scan.
	StatusCounts(ctx context.Context) (map[task.Status]int, error)
	// ProjectCounts counts tasks per project per status — the GROUP BY
	// behind the overview chips and per-project views.
	ProjectCounts(ctx context.Context) (map[string]map[task.Status]int, error)
	// CountTasks counts the tasks matching the filter — the COUNT(*)
	// pushdown behind pagination ("page 2 of 14").
	CountTasks(ctx context.Context, f Filter) (int, error)
	// Close releases resources.
	Close() error
}

// Filter selects tasks for List.
type Filter struct {
	Project *string
	Status  *task.Status
	Type    *string
	// Query is a case-insensitive substring search over id, type, project,
	// payload, lease owner and last error — pushed into SQL LIKE, not a
	// post-filter.
	Query string
	Limit int
	// Offset skips the first Offset matches (pagination); applied after
	// ordering. Meaningful together with Limit.
	Offset int
	// SeverityOrder orders by display severity (dead, running, pending,
	// cancelled, completed), newest first within a status — the dashboard
	// table order, stable across pages. Default order stays priority then
	// age (the queue's fairness order).
	SeverityOrder bool
	// Sort overrides the ordering with an allowlisted column sort for the
	// dashboard's sortable headers: "age-asc", "age-desc",
	// "priority-asc", "priority-desc", "attempts-asc", "attempts-desc".
	// Unknown values fall back to the default order (never interpolated
	// into SQL).
	Sort string
	// Since restricts the listing to tasks created at or after this time
	// (inclusive) — pushed into SQL as a created_at comparison, not a
	// post-filter.
	Since *time.Time
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
