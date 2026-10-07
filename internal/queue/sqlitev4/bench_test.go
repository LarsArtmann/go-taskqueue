package sqlitev4

import (
	"context"
	"encoding/json/jsontext"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// benchOpen opens a store on a fresh temp file with project exclusivity on
// (the agent-pool configuration — the claim path these benchmarks measure).
func benchOpen(b *testing.B) *Store {
	b.Helper()

	s, err := Open(filepath.Join(b.TempDir(), "bench.db"), WithProjectExclusivity())
	if err != nil {
		b.Fatalf("Open: %v", err)
	}

	b.Cleanup(func() { _ = s.Close() })

	return s
}

// BenchmarkClaimCycle measures the full enqueue→claim→complete cycle: one
// durable commit per stage plus the candidate scan. The dominant cost is
// commit durability (fsync policy) — the IO-efficiency canary.
func BenchmarkClaimCycle(b *testing.B) {
	s := benchOpen(b)
	ctx := context.Background()

	b.ResetTimer()

	for range b.N {
		t, err := s.Enqueue(ctx, task.New{Project: "p", Type: "bench", Payload: jsontext.Value("{}")})
		if err != nil {
			b.Fatalf("Enqueue: %v", err)
		}

		claimed, claim, err := s.ClaimDue(ctx, "bench-owner", 2*time.Minute)
		if err != nil {
			b.Fatalf("ClaimDue: %v", err)
		}

		if claimed.ID != t.ID {
			b.Fatalf("ClaimDue claimed %s, want %s", claimed.ID, t.ID)
		}

		if err := s.Complete(ctx, claimed.ID, claim, jsontext.Value(`{}`)); err != nil {
			b.Fatalf("Complete: %v", err)
		}
	}
}

// BenchmarkClaimIdle measures the empty-queue claim poll — the fixed cost
// every idle worker loop pays at its poll cadence, forever.
func BenchmarkClaimIdle(b *testing.B) {
	s := benchOpen(b)
	ctx := context.Background()

	b.ResetTimer()

	for range b.N {
		if _, _, err := s.ClaimDue(ctx, "bench-owner", 2*time.Minute); err == nil {
			b.Fatal("ClaimDue on empty store: claimed a task")
		}
	}
}

// BenchmarkClaimDeepHistory measures the candidate scan when the projects
// carry a deep terminal history (the harvest-heavy shape: tens of
// thousands of completed rows per project backlog) — the project
// exclusivity probe's worst case.
func BenchmarkClaimDeepHistory(b *testing.B) {
	s := benchOpen(b)
	ctx := context.Background()

	b.StopTimer()

	seedHistory(b, s, 50, 400) // 50 projects × 400 completed = 20k rows

	for i := range b.N {
		if _, err := s.Enqueue(ctx, task.New{
			Project: fmt.Sprintf("proj-%d", i%50), Type: "bench", Payload: jsontext.Value("{}"),
		}); err != nil {
			b.Fatalf("Enqueue: %v", err)
		}
	}

	b.StartTimer()

	for range b.N {
		claimed, claim, err := s.ClaimDue(ctx, "bench-owner", 2*time.Minute)
		if err != nil {
			b.Fatalf("ClaimDue: %v", err)
		}

		if err := s.Complete(ctx, claimed.ID, claim, jsontext.Value(`{}`)); err != nil {
			b.Fatalf("Complete: %v", err)
		}
	}
}

// seedHistory bulk-inserts completed terminal rows directly (bypassing the
// engine's per-row commits) so history depth is cheap to build.
func seedHistory(b *testing.B, s *Store, projects, perProject int) {
	b.Helper()

	ctx := context.Background()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		b.Fatalf("BeginTx: %v", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO tasks (id, project, type, payload, priority, status, created_at, updated_at, completed_at)
		VALUES (?, ?, 'history', '{}', 0, 'completed', ?, ?, ?)`)
	if err != nil {
		b.Fatalf("Prepare: %v", err)
	}

	defer func() { _ = stmt.Close() }()

	now := time.Now().UnixMilli()

	for p := range projects {
		for i := range perProject {
			id := fmt.Sprintf("hist-%d-%d", p, i)
			if _, err := stmt.ExecContext(ctx, id, fmt.Sprintf("proj-%d", p), now, now, now); err != nil {
				b.Fatalf("seed %s: %v", id, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		b.Fatalf("Commit: %v", err)
	}
}
