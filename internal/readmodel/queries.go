package readmodel

import (
	metaengine "github.com/larsartmann/go-cqrs-lite/metaengine/v4"
	"github.com/larsartmann/go-cqrs-lite/record/v4"

	"github.com/larsartmann/go-taskqueue/internal/journal"
)

// The task lifecycle statuses as folded into the row. Plain strings: the
// row is a wire/SQL surface and the queue vocabulary is a string enum.
const (
	statusPending   = "pending"
	statusRunning   = "running"
	statusCompleted = "completed"
	statusDead      = "dead"
	statusCancelled = "cancelled"
)

// tasksCollection is the (single) collection name; the planned table is
// meta_planned_tasks.
const tasksCollection = "tasks"

// taskRows is the collection-shaped read result — a struct with a slice
// field is the shape metaengine.ExecuteTyped reconstructs from scans.
type taskRows struct {
	Items []TaskRow
}

// TaskList is the query input: the declarative filter fields. Nil means
// "no constraint" — the engine binds only the fields the caller set.
type TaskList struct {
	Status  *string
	Project *string
}

// tasksQuery folds the task ledger straight off the journal fact types
// (the fold bindings ARE the fact vocabulary). The declarative filter/sort
// fields promote reads onto the planned table (pushdown scan);
// WithColumnarLayout has metaengine.Plan apply the reflection-derived
// LayoutPlan through the engine's LayoutPlanApplier, so every TaskRow
// field is a typed SQL column instead of a JSON blob.
var tasksQuery = metaengine.Query[TaskList, TaskRow](
	tasksCollection,
	metaengine.OnRecordTyped(string(journal.Enqueued), evtEnqueued{},
		func(_ record.Record, e evtEnqueued) (string, TaskRow) {
			return e.ID, TaskRow{
				ID:        e.ID,
				Project:   e.Project,
				Type:      e.Type,
				Status:    statusPending,
				Priority:  e.Priority,
				CreatedAt: e.CreatedAt,
				UpdatedAt: e.CreatedAt,
				DedupKey:  e.DedupKey,
			}
		}),
	metaengine.OnRecordTyped(string(journal.Claimed), evtClaimed{},
		func(_ record.Record, e evtClaimed, prev TaskRow) TaskRow {
			prev.Status = statusRunning
			prev.Attempts++
			prev.UpdatedAt = e.At

			return prev
		}),
	metaengine.OnRecordTyped(string(journal.Completed), evtCompleted{},
		func(_ record.Record, e evtCompleted, prev TaskRow) TaskRow {
			prev.Status = statusCompleted
			prev.UpdatedAt = e.At

			return prev
		}),
	metaengine.OnRecordTyped(string(journal.Failed), evtFailed{},
		func(_ record.Record, e evtFailed, prev TaskRow) TaskRow {
			prev.Status = statusPending
			prev.LastError = string(e.Error)
			prev.UpdatedAt = e.At

			return prev
		}),
	metaengine.OnRecordTyped(string(journal.DeadLettered), evtDeadLettered{},
		func(_ record.Record, e evtDeadLettered, prev TaskRow) TaskRow {
			prev.Status = statusDead
			prev.UpdatedAt = e.At

			return prev
		}),
	metaengine.OnRecordTyped(string(journal.Cancelled), evtCancelled{},
		func(_ record.Record, e evtCancelled, prev TaskRow) TaskRow {
			prev.Status = statusCancelled
			prev.UpdatedAt = e.At

			return prev
		}),
	metaengine.OnRecordTyped(string(journal.Requeued), evtRequeued{},
		func(_ record.Record, e evtRequeued, prev TaskRow) TaskRow {
			prev.Status = statusPending
			prev.UpdatedAt = e.At

			return prev
		}),
	metaengine.OnRecordTyped(string(journal.Reprioritized), evtReprioritized{},
		func(_ record.Record, e evtReprioritized, prev TaskRow) TaskRow {
			prev.Priority = e.Priority
			prev.UpdatedAt = e.At

			return prev
		}),
	metaengine.FilterOnField[TaskRow]("status", metaengine.FilterEq),
	metaengine.FilterOnField[TaskRow]("project", metaengine.FilterEq),
	metaengine.SortOnField[TaskRow]("created_at", true),
	metaengine.WithColumnarLayout(),
)
