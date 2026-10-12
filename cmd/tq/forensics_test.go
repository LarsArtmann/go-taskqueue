package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestTasksDedupFilterPushdown pins the --dedup forensics flag: an exact
// dedup-key match returns exactly the task(s) enqueued under that key.
func TestTasksDedupFilterPushdown(t *testing.T) {
	path := t.TempDir() + "/q.db"

	seed, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}

	ctx := context.Background()

	keyed, err := seed.Enqueue(ctx, task.New{
		Project:  "p",
		Type:     "sh",
		Payload:  []byte(`"true"`),
		DedupKey: "batch:abc123",
	})
	if err != nil {
		t.Fatalf("enqueue keyed: %v", err)
	}

	if _, err := seed.Enqueue(ctx, task.New{
		Project: "p",
		Type:    "sh",
		Payload: []byte(`"true"`),
	}); err != nil {
		t.Fatalf("enqueue plain: %v", err)
	}

	_ = seed.Close()

	var rows []task.Task

	out := captureStdout(t, func() {
		if err := cmdTasks([]string{"--dedup", "batch:abc123", "--json", "--db", path}); err != nil {
			t.Errorf("cmdTasks --dedup: %v", err)
		}
	})

	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}

	if len(rows) != 1 || rows[0].ID != keyed.ID {
		t.Fatalf("--dedup rows = %+v, want only %s", rows, keyed.ID)
	}
}

// TestJournalAuditTaskScopedReplay pins --journal --task-id: the replay
// covers one task (exact id or unique prefix) and an unknown id refuses.
func TestJournalAuditTaskScopedReplay(t *testing.T) {
	store := journalAuditStore(t)
	ctx := context.Background()

	a, err := store.Enqueue(ctx, task.New{Project: "p", Type: "sh", Payload: []byte(`"true"`)})
	if err != nil {
		t.Fatalf("enqueue a: %v", err)
	}

	if _, err := store.Enqueue(ctx, task.New{Project: "p", Type: "sh", Payload: []byte(`"false"`)}); err != nil {
		t.Fatalf("enqueue b: %v", err)
	}

	tk, claim, err := store.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if tk.ID != a.ID {
		t.Fatalf("claim picked %s, want %s (first enqueued)", tk.ID, a.ID)
	}

	if err := store.Complete(ctx, tk.ID, claim, nil); err != nil {
		t.Fatalf("complete: %v", err)
	}

	report, err := journalDriftScoped(ctx, store, tk.ID.String())
	if err != nil {
		t.Fatalf("scoped drift: %v", err)
	}

	if report.TaskID != tk.ID.String() || report.TasksCompared != 1 {
		t.Fatalf("scoped report = task %s compared %d, want %s / 1", report.TaskID, report.TasksCompared, tk.ID)
	}

	// enqueued + claimed + completed = 3 facts for this task, and the
	// replay agrees with the store (no drift).
	if report.FactsReplayed != 3 || report.HasDrift() {
		t.Fatalf("scoped replay = %d facts, drift %v", report.FactsReplayed, report.Drift)
	}

	_, err = journalDriftScoped(ctx, store, "nonexistent00")
	if err == nil || !strings.Contains(err.Error(), "no task") {
		t.Fatalf("unknown id error = %v, want 'no task'", err)
	}
}
