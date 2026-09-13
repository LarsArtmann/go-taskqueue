package main

import (
	"context"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// journalAuditStore builds a scratch sqlite store (the doctor-test pattern).
func journalAuditStore(t *testing.T) *sqlite.Store {
	t.Helper()

	s, err := sqlite.Open(t.TempDir() + "/q.db")
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	return s
}

func TestJournalDriftNoDriftOverFullLifecycle(t *testing.T) {
	ctx := context.Background()
	s := journalAuditStore(t)

	t1, err := s.Enqueue(ctx, task.New{Project: "j", Type: "sh", Payload: []byte(`"true"`)})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	t2, err := s.Enqueue(ctx, task.New{Project: "j", Type: "sh", Payload: []byte(`"false"`)})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	t3, err := s.Enqueue(ctx, task.New{Project: "j", Type: "sh", Payload: []byte(`"true"`)})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	// t1: full happy path.
	if _, err := s.ClaimDue(ctx, "w1", time.Minute); err != nil {
		t.Fatalf("ClaimDue t1: %v", err)
	}
	if err := s.Complete(ctx, t1.ID, "w1", nil); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// t2: dead-letter immediately (permanent failure: failed + dead-lettered).
	if _, err := s.ClaimDue(ctx, "w1", time.Minute); err != nil {
		t.Fatalf("ClaimDue t2: %v", err)
	}
	if err := s.FailPermanent(ctx, t2.ID, "w1", "boom", nil); err != nil {
		t.Fatalf("FailPermanent: %v", err)
	}

	// t3: cancel while pending.
	if err := s.Cancel(ctx, t3.ID, "no longer needed"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	report, err := journalDrift(ctx, s)
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
	var fs []journal.Fact

	for _, s := range steps {
		fs = append(fs, journal.Fact{TaskID: s.id, Type: s.typ})
	}
	got := replayStatuses(fs)
	want := map[string]task.Status{
		"a": task.Completed, "b": task.Dead, "c": task.Cancelled, "d": task.Pending, "e": task.Pending,
	}
	for id, w := range want {
		if got[task.ID(id)] != w {
			t.Errorf("task %s: replayed %v, want %v", id, got[task.ID(id)], w)
		}
	}
}
