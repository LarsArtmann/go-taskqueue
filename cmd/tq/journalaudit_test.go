package main

import (
	"context"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// journalAuditStore builds a scratch sqlite store (the doctor-test pattern).
func journalAuditStore(t *testing.T) *sqlite.Store {
	t.Helper()

	store, err := sqlite.Open(t.TempDir() + "/q.db")
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	return store
}

func TestJournalDriftNoDriftOverFullLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := journalAuditStore(t)

	if _, err := store.Enqueue(ctx, task.New{Project: "j", Type: "sh", Payload: []byte(`"true"`)}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	if _, err := store.Enqueue(ctx, task.New{Project: "j", Type: "sh", Payload: []byte(`"false"`)}); err != nil {
		t.Fatalf("Enqueue #2: %v", err)
	}

	// First claim: complete whichever task came up (happy path).
	first, err := store.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue #1: %v", err)
	}

	if err := store.Complete(ctx, first.ID, "w1", nil); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// t2: dead-letter immediately (permanent failure: failed + dead-lettered).
	claimed, err := store.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue t2: %v", err)
	}

	if err := store.FailPermanent(ctx, claimed.ID, "w1", "boom", nil); err != nil {
		t.Fatalf("FailPermanent: %v", err)
	}

	// The remaining pending task: cancel before it is ever claimed.
	if _, err := store.Enqueue(ctx, task.New{Project: "j", Type: "sh", Payload: []byte(`"true"`)}); err != nil {
		t.Fatalf("Enqueue #3: %v", err)
	}

	pending, err := store.List(ctx, queue.Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	for i := range pending {
		if pending[i].Status == task.Pending {
			if err := store.Cancel(ctx, pending[i].ID, "no longer needed"); err != nil {
				t.Fatalf("Cancel: %v", err)
			}

			break
		}
	}

	report, err := journalDrift(ctx, store)
	if err != nil {
		t.Fatalf("journalDrift: %v", err)
	}

	if report.HasDrift() {
		t.Fatalf("unexpected drift: %+v", report.Drift)
	}

	if report.TasksCompared != 3 {
		t.Errorf("TasksCompared = %d, want 3", report.TasksCompared)
	}

	if report.FactsReplayed == 0 {
		t.Errorf("FactsReplayed = 0, want > 0")
	}
}

func TestReplayStatusesTransitions(t *testing.T) {
	t.Parallel()

	steps := []struct {
		id   string
		typ  journal.FactType
		want task.Status
	}{
		{"a", journal.Enqueued, task.Pending},
		{"a", journal.Claimed, task.Running},
		{"a", journal.Completed, task.Completed},
		{"b", journal.Enqueued, task.Pending},
		{"b", journal.Failed, task.Pending},
		{"b", journal.DeadLettered, task.Dead},
		{"c", journal.Enqueued, task.Pending},
		{"c", journal.Cancelled, task.Cancelled},
		{"d", journal.Enqueued, task.Pending},
		{"d", journal.Requeued, task.Pending},
		{"e", journal.Enqueued, task.Pending},
		{"e", journal.Released, task.Pending},
	}

	facts := make([]journal.Fact, 0, len(steps))

	for _, step := range steps {
		facts = append(facts, journal.Fact{TaskID: step.id, Type: step.typ})
	}

	got := replayStatuses(facts)

	want := map[string]task.Status{
		"a": task.Completed, "b": task.Dead, "c": task.Cancelled, "d": task.Pending, "e": task.Pending,
	}

	for id, expected := range want {
		if got[task.ID(id)] != expected {
			t.Errorf("task %s: replayed %v, want %v", id, got[task.ID(id)], expected)
		}
	}
}
