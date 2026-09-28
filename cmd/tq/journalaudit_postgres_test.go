package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue/postgres"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// Postgres parity for the journal-drift audit (09-13/09-14 reports §f):
// journalDrift consumes only queue.Store (Facts + List), so the sqlite
// lifecycle pins must hold over the postgres backend too. Env-gated like
// the postgres conformance suites (TQ_TEST_POSTGRES; per-test schema so
// the battery cannot see each other's rows) — skips cleanly when unset.

func postgresDriftStore(t *testing.T) *postgres.Store {
	t.Helper()

	base := os.Getenv("TQ_TEST_POSTGRES")
	if base == "" {
		t.Skip("TQ_TEST_POSTGRES unset — journal-drift postgres parity needs a database")
	}

	var seed [8]byte

	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatalf("schema seed: %v", err)
	}

	schema := "tqdrift_" + hex.EncodeToString(seed[:])

	admin, err := sql.Open("pgx", base)
	if err != nil {
		t.Fatalf("admin open: %v", err)
	}

	if _, err := admin.ExecContext(context.Background(), "CREATE SCHEMA "+schema); err != nil {
		_ = admin.Close()
		t.Fatalf("create schema %s: %v", schema, err)
	}

	t.Cleanup(func() {
		_, _ = admin.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		_ = admin.Close()
	})

	dsn := base
	if strings.Contains(base, "?") {
		dsn = base + "&search_path=" + schema
	} else {
		dsn = base + "?search_path=" + schema
	}

	store, err := postgres.Open(context.Background(), dsn, 0)
	if err != nil {
		t.Fatalf("postgres.Open: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	return store
}

func TestJournalDriftNoDriftOverFullLifecyclePostgres(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := postgresDriftStore(t)

	if _, err := store.Enqueue(ctx, task.New{Project: "j", Type: "sh", Payload: []byte(`"true"`)}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	if _, err := store.Enqueue(ctx, task.New{Project: "j", Type: "sh", Payload: []byte(`"false"`)}); err != nil {
		t.Fatalf("Enqueue #2: %v", err)
	}

	first, claim1, err := store.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue #1: %v", err)
	}

	if err := store.Complete(ctx, first.ID, claim1, nil); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	claimed, claim2, err := store.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue t2: %v", err)
	}

	if err := store.FailPermanent(ctx, claimed.ID, claim2, "boom", nil); err != nil {
		t.Fatalf("FailPermanent: %v", err)
	}

	report, err := journalDrift(ctx, store)
	if err != nil {
		t.Fatalf("journalDrift: %v", err)
	}

	if report.HasDrift() {
		t.Fatalf("unexpected drift: %+v", report.Drift)
	}

	if report.TasksCompared != 2 {
		t.Errorf("TasksCompared = %d, want 2", report.TasksCompared)
	}

	if report.FactsReplayed == 0 {
		t.Errorf("FactsReplayed = 0, want > 0")
	}
}
