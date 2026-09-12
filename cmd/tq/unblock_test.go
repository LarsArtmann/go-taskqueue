package main

import (
	"context"
	"encoding/json/v2"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// seedWithDeps enqueues a pending task depending on depIDs and returns it.
func seedWithDeps(t *testing.T, q *queue.Queue, priority int, depIDs ...task.ID) task.Task {
	t.Helper()

	enq, err := q.Enqueue(context.Background(), task.New{
		Type:     "agent",
		Project:  "demo",
		Priority: priority,
		Deps:     depIDs,
	})
	if err != nil {
		t.Fatalf("enqueue with deps: %v", err)
	}

	return enq
}

func completeDep(t *testing.T, q *queue.Queue, id task.ID) {
	t.Helper()

	ctx := context.Background()

	// Equal-priority deps created in the same millisecond make ClaimDue's
	// pick arbitrary: claim until THIS task is held, then complete it.
	for {
		claimed, err := q.ClaimDue(ctx, "unblock-test", 5*time.Minute)
		if errors.Is(err, queue.ErrNoTaskDue) {
			break // nothing claimable left: we may already hold it from an earlier call
		}

		if err != nil {
			t.Fatalf("claim: %v", err)
		}

		if claimed.ID == id {
			break
		}
	}

	if err := q.Complete(ctx, id, "unblock-test", nil); err != nil {
		t.Fatalf("complete: %v", err)
	}
}

// TestBumpUnblocked pins the ADR-0015 unblock bump: a PENDING task whose
// deps ALL completed gains +UnblockBumpPriority once (source "unblock",
// band-protected, fact-guarded idempotency); blocked and hot tasks are
// untouched; dry-run reports without writing.
func TestBumpUnblocked(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store, err := sqlite.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	q := queue.New(store)

	depA, err := q.Enqueue(ctx, task.New{Type: "sh", Project: "demo", Payload: []byte(`{"cmd":"a"}`)})
	if err != nil {
		t.Fatalf("enqueue depA: %v", err)
	}

	depB, err := q.Enqueue(ctx, task.New{Type: "sh", Project: "demo", Payload: []byte(`{"cmd":"b"}`)})
	if err != nil {
		t.Fatalf("enqueue depB: %v", err)
	}

	blocked := seedWithDeps(t, q, 50, depA.ID, depB.ID)
	hot := seedWithDeps(t, q, 120, depA.ID)

	// Nothing completed yet: no bumps.
	changes, err := q.BumpUnblocked(ctx, false)
	if err != nil {
		t.Fatalf("blocked sweep: %v", err)
	}

	if len(changes) != 0 {
		t.Fatalf("bumped while still blocked: %+v", changes)
	}

	completeDep(t, q, depA.ID)

	// Partially unblocked: still nothing.
	if changes, err = q.BumpUnblocked(ctx, false); err != nil || len(changes) != 0 {
		t.Fatalf("bumped after one of two deps: %+v (%v)", changes, err)
	}

	completeDep(t, q, depB.ID)

	// Dry-run reports the blocked task only (hot is band-protected),
	// without writing.
	changes, err = q.BumpUnblocked(ctx, true)
	if err != nil {
		t.Fatalf("dry-run sweep: %v", err)
	}

	if len(changes) != 1 || changes[0].TaskID != blocked.ID {
		t.Fatalf("dry-run changes = %+v, want only the blocked task", changes)
	}

	mustPriority(t, ctx, store, blocked.ID, 50)

	// Real sweep: the bump lands with exactly one unblock fact.
	changes, err = q.BumpUnblocked(ctx, false)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if len(changes) != 1 || changes[0].OldPriority != 50 || changes[0].NewPriority != 50+queue.UnblockBumpPriority {
		t.Fatalf("changes = %+v", changes)
	}

	mustPriority(t, ctx, store, blocked.ID, 50+queue.UnblockBumpPriority)

	facts, err := store.FactsForTask(ctx, blocked.ID.String(), 0)
	if err != nil {
		t.Fatalf("facts: %v", err)
	}

	unblockFacts := 0

	for _, f := range facts {
		if f.Type != journal.Reprioritized {
			continue
		}

		unblockFacts++

		var evidence queue.ReprioritizeEvidence
		if err := json.Unmarshal(f.Detail, &evidence); err != nil || evidence.Source != queue.PrioritySourceUnblock {
			t.Fatalf("unblock fact evidence = %+v (%v)", evidence, err)
		}
	}

	if unblockFacts != 1 {
		t.Fatalf("unblock facts = %d, want exactly 1", unblockFacts)
	}

	mustPriority(t, ctx, store, hot.ID, 120)

	// Second sweep: the fact guard holds — no double bump.
	if changes, err = q.BumpUnblocked(ctx, false); err != nil || len(changes) != 0 {
		t.Fatalf("second sweep re-bumped: %+v (%v)", changes, err)
	}
}

// mustPriority fetches one task and asserts its stored priority.
func mustPriority(t *testing.T, ctx context.Context, store *sqlite.Store, id task.ID, want int) {
	t.Helper()

	got, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("get %s: %v", id, err)
	}

	if got.Priority != want {
		t.Fatalf("task %s priority = %d, want %d", id, got.Priority, want)
	}
}
