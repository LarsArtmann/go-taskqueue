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

// The Delta counter collections behind Stats (upstream metaengine
// COOKBOOK §Status Count Dashboard): one counter keyed by status, one
// keyed project|status. Both fold the SAME fact stream the tasks map
// folds — Plan routes one ApplyRecord to every query whose folds bind
// the event's wire type — so they stay exact without a second read.
const (
	statusCountsCollection        = "status_counts"
	projectStatusCountsCollection = "project_status_counts"
)

// projectStatusKeySep joins the project|status counter key. Projects are
// store-validated slugs (no pipe), statuses are the closed enum.
const projectStatusKeySep = "_"

// statusCountsInput and projectStatusCountsInput are the counter queries'
// declared input types — ExecuteTyped dispatch carriers, one distinct
// type per counter so type dispatch never shadows.
type (
	statusCountsInput        struct{}
	projectStatusCountsInput struct{}
)

// TaskList is the query's declared input type — a declaration carrier
// only. Reads MUST NOT dispatch through it (Store.ExecuteCtx binds nil
// pointer fields as typed-nil interfaces → `= NULL`, and empty strings
// as real filters): Model.Tasks reads via the TypedReader instead.
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
var tasksQuery = metaengine.Query[TaskList, TaskRow]( //nolint:gochecknoglobals // declarative fold definition, reused by Plan and every read
	tasksCollection,
	metaengine.OnRecordTyped(string(journal.Enqueued), evtEnqueued{},
		func(_ record.Record, evt evtEnqueued) (string, TaskRow) {
			return evt.ID, TaskRow{
				ID:        evt.ID,
				Project:   evt.Project,
				Type:      evt.Type,
				Status:    statusPending,
				Priority:  evt.Priority,
				CreatedAt: evt.CreatedAt,
				UpdatedAt: evt.CreatedAt,
				DedupKey:  evt.DedupKey,
			}
		}),
	metaengine.OnRecordTyped(string(journal.Claimed), evtClaimed{},
		func(_ record.Record, evt evtClaimed, prev TaskRow) TaskRow {
			prev.Status = statusRunning
			prev.NotBefore, prev.ParkedBy = 0, "" // a claim ends the park
			prev.UpdatedAt = evt.At

			return prev
		}),
	metaengine.OnRecordTyped(string(journal.Completed), evtCompleted{},
		func(_ record.Record, evt evtCompleted, prev TaskRow) TaskRow {
			prev.Status = statusCompleted
			prev.UpdatedAt = evt.At

			return prev
		}),
	metaengine.OnRecordTyped(string(journal.Failed), evtFailed{},
		func(_ record.Record, evt evtFailed, prev TaskRow) TaskRow {
			prev.Status = statusPending
			prev.Attempts++ // the store burns one attempt per failure
			prev.LastError = string(evt.Error)
			prev.NotBefore, prev.ParkedBy = 0, "" // a burned attempt is a retry, not a park
			prev.UpdatedAt = evt.At

			return prev
		}),
	metaengine.OnRecordTyped(string(journal.DeadLettered), evtDeadLettered{},
		func(_ record.Record, evt evtDeadLettered, prev TaskRow) TaskRow {
			prev.Status = statusDead
			prev.UpdatedAt = evt.At

			return prev
		}),
	metaengine.OnRecordTyped(string(journal.Cancelled), evtCancelled{},
		func(_ record.Record, evt evtCancelled, prev TaskRow) TaskRow {
			prev.Status = statusCancelled
			prev.UpdatedAt = evt.At

			return prev
		}),
	metaengine.OnRecordTyped(string(journal.Requeued), evtRequeued{},
		func(_ record.Record, evt evtRequeued, prev TaskRow) TaskRow {
			prev.Status = statusPending
			prev.NotBefore = evt.At + evt.RetryInMs
			prev.ParkedBy = string(evt.Class)
			prev.UpdatedAt = evt.At

			return prev
		}),
	metaengine.OnRecordTyped(string(journal.Reprioritized), evtReprioritized{},
		func(_ record.Record, evt evtReprioritized, prev TaskRow) TaskRow {
			prev.Priority = evt.Priority
			prev.UpdatedAt = evt.At

			return prev
		}),
	metaengine.FilterOnField[TaskRow]("status", metaengine.FilterEq),
	metaengine.FilterOnField[TaskRow]("project", metaengine.FilterEq),
	metaengine.SortOnField[TaskRow]("created_at", true),
	metaengine.WithColumnarLayout(),
)

// statusCountsQuery folds the global per-status counters. Every
// transition handler decrements the folded from-status and increments
// the new one; the from-status rides the event (filled by the model's
// apply-time prev read), so stateless handlers stay exact across the
// from-status-ambiguous transitions (lease reclaims, dead rescues,
// dismissals).
var statusCountsQuery = metaengine.Query[statusCountsInput, map[string]int64]( //nolint:gochecknoglobals // declarative fold definition, reused by Plan and every read
	statusCountsCollection,
	counterFold(string(journal.Enqueued), evtEnqueued{}, func(e evtEnqueued) metaengine.Delta {
		return transitionDelta(statusPending, string(e.From))
	}),
	counterFold(string(journal.Claimed), evtClaimed{}, func(e evtClaimed) metaengine.Delta {
		return transitionDelta(statusRunning, string(e.From))
	}),
	counterFold(string(journal.Completed), evtCompleted{}, func(e evtCompleted) metaengine.Delta {
		return transitionDelta(statusCompleted, string(e.From))
	}),
	counterFold(string(journal.Failed), evtFailed{}, func(e evtFailed) metaengine.Delta {
		return transitionDelta(statusPending, string(e.From))
	}),
	counterFold(string(journal.DeadLettered), evtDeadLettered{}, func(e evtDeadLettered) metaengine.Delta {
		return transitionDelta(statusDead, string(e.From))
	}),
	counterFold(string(journal.Cancelled), evtCancelled{}, func(e evtCancelled) metaengine.Delta {
		return transitionDelta(statusCancelled, string(e.From))
	}),
	counterFold(string(journal.Requeued), evtRequeued{}, func(e evtRequeued) metaengine.Delta {
		return transitionDelta(statusPending, string(e.From))
	}),
)

// projectStatusCountsQuery folds the project|status counters — the same
// transitions keyed by the folded project (constant per task; enqueued
// facts and the apply-time prev read both carry it).
var projectStatusCountsQuery = metaengine.Query[projectStatusCountsInput, map[string]int64]( //nolint:gochecknoglobals // declarative fold definition, reused by Plan and every read
	projectStatusCountsCollection,
	counterFold(string(journal.Enqueued), evtEnqueued{}, func(e evtEnqueued) metaengine.Delta {
		return projectTransitionDelta(e.Project, statusPending, string(e.From))
	}),
	counterFold(string(journal.Claimed), evtClaimed{}, func(e evtClaimed) metaengine.Delta {
		return projectTransitionDelta(string(e.Project), statusRunning, string(e.From))
	}),
	counterFold(string(journal.Completed), evtCompleted{}, func(e evtCompleted) metaengine.Delta {
		return projectTransitionDelta(string(e.Project), statusCompleted, string(e.From))
	}),
	counterFold(string(journal.Failed), evtFailed{}, func(e evtFailed) metaengine.Delta {
		return projectTransitionDelta(string(e.Project), statusPending, string(e.From))
	}),
	counterFold(string(journal.DeadLettered), evtDeadLettered{}, func(e evtDeadLettered) metaengine.Delta {
		return projectTransitionDelta(string(e.Project), statusDead, string(e.From))
	}),
	counterFold(string(journal.Cancelled), evtCancelled{}, func(e evtCancelled) metaengine.Delta {
		return projectTransitionDelta(string(e.Project), statusCancelled, string(e.From))
	}),
	counterFold(string(journal.Requeued), evtRequeued{}, func(e evtRequeued) metaengine.Delta {
		return projectTransitionDelta(string(e.Project), statusPending, string(e.From))
	}),
)

// counterFold is OnRecordTyped narrowed to the Delta handler shape — the
// record context is unused by the counters but the wire-typed binding
// matches the tasks map folds.
func counterFold[E any](eventType string, sample E, handler func(E) metaengine.Delta) metaengine.Fold {
	return metaengine.OnRecordTyped(eventType, sample, func(_ record.Record, e E) metaengine.Delta {
		return handler(e)
	})
}

// transitionDelta emits the +1 on the new key and the -1 on the
// from-key; an empty from (seed fact, or a prev row the projection has
// not folded) emits only the increment — the counter then converges on
// the next full replay, same degradation story as the map folds.
func transitionDelta(next, from string) metaengine.Delta {
	delta := metaengine.Delta{next: 1}
	if from != "" {
		delta[from] = -1
	}

	return delta
}

// projectTransitionDelta emits the +1 on the new project|status key and
// the -1 on the from-status key under the SAME project; an empty
// from-status (seed fact, or a prev row the projection has not folded)
// emits only the increment — the counter then converges on the next full
// replay, same degradation story as the map folds.
func projectTransitionDelta(project, nextStatus, fromStatus string) metaengine.Delta {
	delta := metaengine.Delta{projectStatusKey(project, nextStatus): 1}
	if fromStatus != "" {
		delta[projectStatusKey(project, fromStatus)] = -1
	}

	return delta
}

// projectStatusKey builds the project|status counter key.
func projectStatusKey(project, status string) string {
	return project + projectStatusKeySep + status
}
