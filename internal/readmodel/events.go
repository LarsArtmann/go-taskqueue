package readmodel

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// The events below are the fold inputs: one struct per journal fact type
// the collections consume. metaengine dispatches Store.ApplyRecord payloads
// by Go type, so the mapping from the open fact vocabulary is a 1:1 struct
// per consumed fact. Heartbeats, cancel requests, orphan observations,
// released markers and the tq-only fact families (session.*, question-*,
// budget) carry no ledger state and are skipped — the projector advances
// its cursor past them without an Apply.

type (
	// evtEnqueued seeds a row; the thin-enqueue side channel fills what
	// the current fact detail omits.
	evtEnqueued struct {
		ID        string
		Project   string
		Type      string
		Priority  int
		DedupKey  string
		CreatedAt int64
	}
	// evtClaimed marks an attempt start (status → running).
	evtClaimed struct {
		ID string
		At int64
	}
	// evtCompleted finishes the task.
	evtCompleted struct {
		ID string
		At int64
	}
	// evtFailed records a failed attempt (retry stays pending). Error is
	// a named string so the engine's type-based key inference keeps ID
	// unambiguous (two bare string fields would read as two keys).
	evtFailed struct {
		ID    string
		Error FailureText
		At    int64
	}
	// evtDeadLettered moves the task to the DLQ.
	evtDeadLettered struct {
		ID string
		At int64
	}
	// evtCancelled withdraws the task.
	evtCancelled struct {
		ID string
		At int64
	}
	// evtRequeued returns a claim to pending (no attempt burned).
	evtRequeued struct {
		ID string
		At int64
	}
	// evtReprioritized rewrites the stored priority.
	evtReprioritized struct {
		ID       string
		Priority int
		At       int64
	}
)

// FailureText is a failed attempt's error text — a named string so event
// structs carry at most one bare string (the fold key).
type FailureText string

// RowSource supplies the enqueue-time fields the current engine's thin
// task.enqueued fact omits (project/type fall back to the fact detail;
// priority, dedup key and creation time exist only on the row). It dies
// with the upstream detail growth (M4 memo) — the fold then reads
// everything from the fact.
type RowSource interface {
	// EnqueueRow reports the enqueue-time projection of one task; a
	// miss (task already compacted away) returns ok=false and the fold
	// keeps the fact-detail fallbacks.
	EnqueueRow(ctx context.Context, id string) (row RowSnapshot, ok bool, err error)
}

// RowSnapshot is the enqueue-time slice of a task row.
type RowSnapshot struct {
	Project   string
	Type      string
	Priority  int
	DedupKey  string
	CreatedAt int64
}

// StoreRows adapts a queue.Store into a RowSource via Get.
type StoreRows struct {
	Store queue.Store
}

// EnqueueRow implements RowSource.
func (s StoreRows) EnqueueRow(ctx context.Context, id string) (RowSnapshot, bool, error) {
	t, err := s.Store.Get(ctx, task.ID(id))
	if err != nil {
		return RowSnapshot{}, false, err
	}

	return RowSnapshot{
		Project:   t.Project,
		Type:      t.Type,
		Priority:  t.Priority,
		DedupKey:  t.DedupKey,
		CreatedAt: t.CreatedAt.UnixMilli(),
	}, true, nil
}

// enqueueDetail mirrors the queue.EnqueueDetail keys the projector needs.
// It is deliberately a local struct: the fact detail is wire surface, and
// the read model must not grow a compile-time coupling to projection
// internals beyond the store read it already owns.
type enqueueDetail struct {
	Project string `json:"project"`
	Type    string `json:"type"`
}

// repriDetail mirrors queue.ReprioritizeEvidence's wire keys.
type repriDetail struct {
	OldPriority int    `json:"old_priority"`
	NewPriority int    `json:"new_priority"`
	Source      string `json:"source"`
	Reason      string `json:"reason"`
}

// eventFor maps one journal fact to its fold input. ok=false skips the
// fact (no fold consumes it). The returned error covers only malformed
// DETAIL bytes on fact types whose fold needs them — state transitions
// never fail the pump on detail noise.
func eventFor(ctx context.Context, fact journal.Fact, src RowSource) (any, bool, error) {
	atMs := fact.Time.UnixMilli()

	switch fact.Type {
	case journal.Enqueued:
		return enqueuedEvent(ctx, fact, src)
	case journal.Claimed:
		return evtClaimed{ID: fact.TaskID, At: atMs}, true, nil
	case journal.Completed:
		return evtCompleted{ID: fact.TaskID, At: atMs}, true, nil
	case journal.Failed:
		return evtFailed{ID: fact.TaskID, Error: FailureText(fact.Error), At: atMs}, true, nil
	case journal.DeadLettered:
		return evtDeadLettered{ID: fact.TaskID, At: atMs}, true, nil
	case journal.Cancelled:
		return evtCancelled{ID: fact.TaskID, At: atMs}, true, nil
	case journal.Requeued:
		return evtRequeued{ID: fact.TaskID, At: atMs}, true, nil
	case journal.Reprioritized:
		return repriEvent(fact, atMs)
	case journal.Heartbeat,
		journal.CancelRequested,
		journal.Released,
		journal.Orphaned,
		journal.SessionOpened,
		journal.SessionClosed,
		journal.QuestionAsked,
		journal.QuestionAnswered:
		// No ledger state: the doc comment above lists these families as
		// deliberate skips.
		return nil, false, nil
	default:
		return nil, false, nil
	}
}

// enqueuedEvent builds the seed event, preferring the store row (side
// channel) and falling back to the fact detail for project/type.
func enqueuedEvent(ctx context.Context, fact journal.Fact, src RowSource) (any, bool, error) {
	evt := evtEnqueued{
		ID:        fact.TaskID,
		CreatedAt: fact.Time.UnixMilli(),
	}

	if src != nil {
		snap, ok, err := src.EnqueueRow(ctx, fact.TaskID)
		if err != nil {
			return nil, false, fmt.Errorf("readmodel: enqueue side channel %s: %w", fact.TaskID, err)
		}

		if ok {
			evt.Project, evt.Type = snap.Project, snap.Type
			evt.Priority, evt.DedupKey = snap.Priority, snap.DedupKey
			evt.CreatedAt = snap.CreatedAt
		}
	}

	if evt.Project == "" || evt.Type == "" {
		var detail enqueueDetail
		if err := json.Unmarshal(fact.Detail, &detail); err != nil {
			return nil, false, fmt.Errorf("readmodel: enqueue detail %s: %w", fact.TaskID, err)
		}

		evt.Project = fallbackString(evt.Project, detail.Project)
		evt.Type = fallbackString(evt.Type, detail.Type)
	}

	return evt, true, nil
}

// repriEvent decodes the reprioritize evidence; an unparsable detail is a
// malformed fact, reported as an error so the pump surfaces journal drift.
func repriEvent(fact journal.Fact, atMs int64) (any, bool, error) {
	var detail repriDetail
	if err := json.Unmarshal(fact.Detail, &detail); err != nil {
		return nil, false, fmt.Errorf("readmodel: reprioritized detail %s: %w", fact.TaskID, err)
	}

	return evtReprioritized{ID: fact.TaskID, Priority: detail.NewPriority, At: atMs}, true, nil
}

func fallbackString(prefer, fallback string) string {
	if prefer != "" {
		return prefer
	}

	return fallback
}
