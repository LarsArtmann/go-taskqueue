package main

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"encoding/json/jsontext"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// seedDrift corrupts the tasks table of a CLOSED store's database file and
// returns the store reopened for the audit (hermetic: facts stay truthful,
// the projection lies).
func seedDrift(t *testing.T, path string) *sqlite.Store {
	t.Helper()

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("raw open: %v", err)
	}

	if _, err := db.Exec(`UPDATE tasks SET status = 'completed', attempts = 99, priority = 42, dedup_key = 'seeded'`); err != nil {
		t.Fatalf("seed drift: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("raw close: %v", err)
	}

	store, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	return store
}

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

func TestJournalDriftSeededDriftAllFields(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	path := t.TempDir() + "/drift.db"

	store, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}

	if _, err := store.Enqueue(ctx, task.New{
		Project: "j", Type: "sh", Payload: []byte(`"true"`), Priority: 3, DedupKey: "todo:real",
	}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	store = seedDrift(t, path)

	report, err := journalDrift(ctx, store)
	if err != nil {
		t.Fatalf("journalDrift: %v", err)
	}

	if !report.HasDrift() {
		t.Fatalf("expected seeded drift, got %+v", report)
	}

	fields := map[string]DriftRow{}
	for _, row := range report.Drift {
		fields[row.Field] = row
	}

	for _, field := range []string{"status", "attempts", "priority", "dedup_key"} {
		row, ok := fields[field]
		if !ok {
			t.Errorf("no drift row for field %s (report: %+v)", field, report)

			continue
		}

		if row.Replayed == row.Stored {
			t.Errorf("field %s: stored == replayed == %q, expected divergence", field, row.Stored)
		}
	}

	if fields["status"].Stored != "completed" || fields["status"].Replayed != "pending" {
		t.Errorf("status row = %+v, want stored=completed replayed=pending", fields["status"])
	}

	if fields["priority"].Replayed != "3" {
		t.Errorf("priority row = %+v, want replayed=3", fields["priority"])
	}

	if fields["dedup_key"].Replayed != "todo:real" {
		t.Errorf("dedup row = %+v, want replayed=todo:real", fields["dedup_key"])
	}
}

func TestReplayProjectionTransitions(t *testing.T) {
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

	for i, step := range steps {
		attempt := 0
		if step.typ == journal.Failed || step.typ == journal.DeadLettered {
			attempt = i // any post-increment number; max() takes the last
		}

		facts = append(facts, journal.Fact{TaskID: step.id, Type: step.typ, Attempt: attempt})
	}

	got := replayProjection(facts)

	want := map[string]task.Status{
		"a": task.Completed, "b": task.Dead, "c": task.Cancelled, "d": task.Pending, "e": task.Pending,
	}

	for id, expected := range want {
		if got[task.ID(id)] == nil {
			t.Errorf("task %s: missing from replay", id)

			continue
		}

		if got[task.ID(id)].status != expected {
			t.Errorf("task %s: replayed %v, want %v", id, got[task.ID(id)].status, expected)
		}
	}

	if got[task.ID("b")].attempts != 5 {
		t.Errorf("task b: attempts = %d, want 5 (max failed/dead-lettered attempt)",
			got[task.ID("b")].attempts)
	}
}

func TestReplayProjectionPriorityAndDedup(t *testing.T) {
	t.Parallel()

	p := 7
	facts := []journal.Fact{
		{TaskID: "p", Type: journal.Enqueued, Detail: jsontext.Value(`{"priority":3,"dedup_key":"todo:x"}`)},
		{TaskID: "p", Type: journal.Reprioritized, Detail: jsontext.Value(`{"old_priority":3,"new_priority":7,"source":"manual"}`)},
		{TaskID: "legacy", Type: journal.Enqueued},
	}

	got := replayProjection(facts)

	state := got[task.ID("p")]
	if state == nil || state.priority == nil || *state.priority != p {
		t.Fatalf("task p: replayed priority = %v, want %d", state, p)
	}

	if state.dedupKey != "todo:x" || !state.dedupKnown {
		t.Errorf("task p: dedup = %q known=%v, want todo:x known=true", state.dedupKey, state.dedupKnown)
	}

	legacy := got[task.ID("legacy")]
	if legacy.priorityKnown || legacy.dedupKnown {
		t.Errorf("legacy thin fact must not claim knowledge: %+v", legacy)
	}
}
