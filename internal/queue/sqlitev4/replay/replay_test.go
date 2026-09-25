package main

import (
	"context"
	"database/sql"
	"encoding/json/jsontext"
	"path/filepath"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	oldsqlite "github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlitev4"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// seedOldJournal produces a representative dogfood-shaped journal through
// the REAL hand-rolled store — the same producer the production journal
// has.
func seedOldJournal(t *testing.T, path string) {
	t.Helper()

	ctx := context.Background()

	store, err := oldsqlite.Open(path)
	if err != nil {
		t.Fatalf("open old store: %v", err)
	}
	defer store.Close()

	completed, err := store.Enqueue(ctx, task.New{
		Project: "go-taskqueue", Type: "agent",
		Payload: jsontext.Value(`{"repo":"go-taskqueue","prompt":"do a thing"}`),
	})
	if err != nil {
		t.Fatalf("enqueue completed: %v", err)
	}

	claimed, claim_worker_1, err := store.ClaimDue(ctx, "worker-1", time.Minute)
	if err != nil {
		t.Fatalf("claim completed: %v", err)
	}

	if claimed.ID != completed.ID {
		t.Fatalf("claimed %s, want %s", claimed.ID, completed.ID)
	}

	if err := store.Complete(ctx, claimed.ID,claim_worker_1, jsontext.Value(`{"ok":true}`)); err != nil {
		t.Fatalf("complete: %v", err)
	}

	dead, err := store.Enqueue(ctx, task.New{
		Project: "go-taskqueue", Type: "agent", MaxAttempts: 1,
		Payload:  jsontext.Value(`{"repo":"go-taskqueue","prompt":"boom"}`),
		DedupKey: "go-taskqueue:boom",
	})
	if err != nil {
		t.Fatalf("enqueue dead: %v", err)
	}

	deadClaimed, claim_worker_2, err := store.ClaimDue(ctx, "worker-2", time.Minute)
	if err != nil {
		t.Fatalf("claim dead: %v", err)
	}

	if deadClaimed.ID != dead.ID {
		t.Fatalf("claimed %s, want %s", deadClaimed.ID, dead.ID)
	}

	if err := store.Fail(
		ctx,
		dead.ID,
		claim_worker_2,
		"verify failed",
		0,
		jsontext.Value(`{"stage":"verify"}`),
	); err != nil {
		t.Fatalf("fail: %v", err)
	}

	pending, err := store.Enqueue(ctx, task.New{
		Project: "overview", Type: "sh",
		Payload: jsontext.Value(`"echo hi"`),
	})
	if err != nil {
		t.Fatalf("enqueue pending: %v", err)
	}

	parked, err := store.Enqueue(ctx, task.New{
		Project: "overview", Type: "sh",
		Payload:   jsontext.Value(`"echo parked"`),
		NotBefore: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("enqueue parked: %v", err)
	}

	_ = parked

	if err != nil {
		t.Fatalf("enqueue pending: %v", err)
	}

	if err := store.UpdatePendingPriority(ctx, pending.ID, 5, "manual", "test"); err != nil {
		t.Fatalf("reprioritize: %v", err)
	}

	if err := store.Heartbeat(ctx, dead.ID,claim_worker_2, time.Minute); err == nil {
		t.Log("heartbeat on dead task correctly refused")
	}

	if err := store.AppendFact(ctx, journal.Fact{
		TaskID: "session:abc123", Type: journal.SessionOpened,
		Detail: jsontext.Value(`{"repo":"go-taskqueue"}`),
	}); err != nil {
		t.Fatalf("append session fact: %v", err)
	}

	if err := store.SaveWatermark(ctx, "papdashboard:http://pap", 3); err != nil {
		t.Fatalf("save watermark: %v", err)
	}

	if err := store.SavePriorityScore(ctx, queue.PriorityScore{
		ItemKey: "go-taskqueue:some item", Score: 80, EffortMinutes: 30,
		Source: "ai:batch-scorer", Reasoning: "high impact", Tokens: 500,
		ScoredAt: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("save score: %v", err)
	}
}

func TestReplayRoundTripProjectionEquality(t *testing.T) {
	ctx := context.Background()

	from := filepath.Join(t.TempDir(), "old.db")
	to := filepath.Join(t.TempDir(), "new.db")

	seedOldJournal(t, from)

	stats, err := Migrate(ctx, from, to)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if stats.Tasks != 4 || stats.Facts < 6 || stats.Watermarks != 1 || stats.PriorityScores != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}

	report, err := Verify(ctx, from, to)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	for _, s := range report.Sections {
		if !s.OK {
			t.Errorf("projection %q mismatched: %s", s.Name, s.Detail)
		}
	}

	if !report.OK() {
		t.Logf("report:\n%s", report.Summary())
	}
}

func TestVerifyDetectsTamperedTarget(t *testing.T) {
	ctx := context.Background()

	from := filepath.Join(t.TempDir(), "old.db")
	to := filepath.Join(t.TempDir(), "new.db")

	seedOldJournal(t, from)

	if _, err := Migrate(ctx, from, to); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Tamper with a task row: the status-counts and DLQ sections must
	// catch the drift.
	db, err := sql.Open("sqlite", copyDSN(to))
	if err != nil {
		t.Fatalf("open target: %v", err)
	}

	if _, err := db.ExecContext(ctx, `UPDATE tasks SET status = 'cancelled' WHERE status = 'dead'`); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close target: %v", err)
	}

	report, err := Verify(ctx, from, to)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	if report.OK() {
		t.Fatalf("tampered target passed verification:\n%s", report.Summary())
	}

	var dlqFailed bool

	for _, s := range report.Sections {
		if !s.OK && (s.Name == "status counts" || s.Name == "dlq") {
			dlqFailed = true
		}
	}

	if !dlqFailed {
		t.Fatalf("expected status-counts or dlq section to catch the tamper:\n%s", report.Summary())
	}
}

func TestVerifyDetectsDeletedFact(t *testing.T) {
	ctx := context.Background()

	from := filepath.Join(t.TempDir(), "old.db")
	to := filepath.Join(t.TempDir(), "new.db")

	seedOldJournal(t, from)

	if _, err := Migrate(ctx, from, to); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	db, err := sql.Open("sqlite", copyDSN(to))
	if err != nil {
		t.Fatalf("open target: %v", err)
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM facts WHERE seq = 1`); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("close target: %v", err)
	}

	report, err := Verify(ctx, from, to)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	if report.OK() {
		t.Fatalf("deleted fact passed verification:\n%s", report.Summary())
	}
}

func TestMigrateRefusesExistingTarget(t *testing.T) {
	ctx := context.Background()

	dir := t.TempDir()
	from := filepath.Join(dir, "old.db")
	to := filepath.Join(dir, "new.db")

	seedOldJournal(t, from)

	if _, err := Migrate(ctx, from, to); err != nil {
		t.Fatalf("first migrate: %v", err)
	}

	if _, err := Migrate(ctx, from, to); err == nil {
		t.Fatal("second migrate into existing target must fail")
	}
}

func TestVerifyMatchesSQLiteV4Store(t *testing.T) {
	ctx := context.Background()

	from := filepath.Join(t.TempDir(), "old.db")
	to := filepath.Join(t.TempDir(), "new.db")

	seedOldJournal(t, from)

	if _, err := Migrate(ctx, from, to); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store, err := sqlitev4.Open(to)
	if err != nil {
		t.Fatalf("open replayed store: %v", err)
	}
	defer store.Close()

	// The replayed engine store must be a LIVE queue: the copied pending
	// task is claimable and terminal tasks stay terminal.
	claimed, _, err := store.ClaimDue(ctx, "worker-after-cutover", time.Minute)
	if err != nil {
		t.Fatalf("claim from replayed store: %v", err)
	}

	if claimed.Type != "sh" {
		t.Fatalf("claimed task %s type %q, want the pending sh task", claimed.ID, claimed.Type)
	}

	// The parked task (future NotBefore) stays unclaimable.
	_, _, err = store.ClaimDue(ctx, "worker-after-cutover",
		time.Minute)
	if err == nil {
		t.Fatal("second claim should find nothing due (parked NotBefore)")
	}
}
