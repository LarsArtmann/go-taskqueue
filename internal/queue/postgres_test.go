package queue

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// testPostgresStore opens a PostgresStore against $TQ_TEST_POSTGRES and
// registers cleanup; tests skip when the variable is unset (CI provides a
// service container; local runs provide a cluster — see ADR-0007).
func testPostgresStore(t *testing.T) *PostgresStore {
	t.Helper()

	dsn := os.Getenv("TQ_TEST_POSTGRES")
	if dsn == "" {
		t.Skip("TQ_TEST_POSTGRES unset — Postgres conformance needs a database")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	s, err := OpenPostgres(ctx, dsn, 0)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	// Deterministic isolation: the database persists between runs, so each
	// test starts from an empty queue and journal.
	if _, err := s.pool.Exec(ctx, `TRUNCATE tasks, deps, facts, watermarks RESTART IDENTITY`); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	return s
}

// claimUntil claims tasks until the wanted one is held; anything claimed
// on the way is completed (ClaimDue returns an arbitrary due task).
func claimUntil(t *testing.T, s *PostgresStore, ctx context.Context, want task.ID, owner string, lease time.Duration) {
	t.Helper()

	for range 50 {
		got, err := s.ClaimDue(ctx, owner, lease)
		if err != nil {
			t.Fatalf("claim-until %s: %v", want, err)
		}

		if got.ID == want {
			return
		}

		if err := s.Complete(ctx, got.ID, owner, nil); err != nil {
			t.Fatalf("complete bystander %s: %v", got.ID, err)
		}
	}

	t.Fatalf("never claimed %s", want)
}

// TestPostgresLifecycle pins the core semantics against the Postgres
// backend: enqueue + facts, exclusive claim, lease expiry reclaim,
// complete, retry-then-dead-letter, dedup idempotency, cooperative cancel,
// orphan marking, and the read surface (counts, watermark).
func TestPostgresLifecycle(t *testing.T) {
	s := testPostgresStore(t)
	ctx := context.Background()

	// Unique project per run: CI databases persist between runs.
	project := "conf-" + time.Now().Format("150405.000000000")

	enq, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if enq.Status != task.Pending {
		t.Fatalf("fresh task status = %s", enq.Status)
	}

	// Exclusive claim.
	got, err := s.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if got.ID != enq.ID {
		t.Fatalf("claimed %s, want %s", got.ID, enq.ID)
	}

	if _, err := s.ClaimDue(ctx, "w2", time.Minute); !errors.Is(err, ErrNoTaskDue) {
		t.Fatalf("second claim of one task: err = %v, want ErrNoTaskDue", err)
	}

	// Lease guard: a foreign owner cannot complete.
	if err := s.Complete(ctx, enq.ID, "w2", nil); !errors.Is(err, task.ErrLeaseNotHeld) {
		t.Fatalf("foreign complete: err = %v, want ErrLeaseNotHeld", err)
	}

	// Cooperative cancel mid-run: request observed, owner finalizes.
	if err := s.CancelRunning(ctx, enq.ID); err != nil {
		t.Fatalf("cancel-running: %v", err)
	}

	requested, err := s.CancelRequested(ctx, enq.ID)
	if err != nil || !requested {
		t.Fatalf("cancel-requested = %v (%v)", requested, err)
	}

	if err := s.CancelOwned(ctx, enq.ID, "w1"); err != nil {
		t.Fatalf("cancel-owned: %v", err)
	}

	final, err := s.Get(ctx, enq.ID)
	if err != nil || final.Status != task.Cancelled {
		t.Fatalf("final status = %s (%v), want cancelled", final.Status, err)
	}

	// Retry ladder: attempts exhausted => dead.
	retry, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project, MaxAttempts: 1})
	if err != nil {
		t.Fatal(err)
	}

	claimUntil(t, s, ctx, retry.ID, "w1", time.Minute)

	if err := s.Fail(ctx, retry.ID, "w1", "boom", time.Millisecond); err != nil {
		t.Fatalf("fail: %v", err)
	}

	dead, err := s.Get(ctx, retry.ID)
	if err != nil || dead.Status != task.Dead {
		t.Fatalf("after last fail: %s (%v), want dead", dead.Status, err)
	}

	// Rescue restores a fresh budget.
	if err := s.RescueDead(ctx, retry.ID, 2); err != nil {
		t.Fatalf("rescue: %v", err)
	}

	// Dedup: same key returns the stored task unchanged.
	again, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project, DedupKey: "conf-dedup"})
	if err != nil {
		t.Fatal(err)
	}

	stored, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project, DedupKey: "conf-dedup"})
	if err != nil || again.ID != stored.ID {
		t.Fatalf("dedup key not idempotent: %v vs %v (%v)", again.ID, stored.ID, err)
	}

	// Read surface: counts, watermark, facts cursor, per-task trail.
	counts, err := s.StatusCounts(ctx)
	if err != nil || counts[task.Cancelled] < 1 || counts[task.Dead] < 0 {
		t.Fatalf("status counts = %+v (%v)", counts, err)
	}

	head, err := s.HeadSeq(ctx)
	if err != nil || head <= 0 {
		t.Fatalf("head seq = %d (%v)", head, err)
	}

	facts, err := s.Facts(ctx, 0, 0)
	if err != nil || len(facts) == 0 {
		t.Fatalf("facts = %d (%v)", len(facts), err)
	}

	trail, err := s.FactsForTask(ctx, enq.ID.String(), 0)
	if err != nil || len(trail) < 3 { // enqueued + claimed + cancelled (+detail)
		t.Fatalf("task trail = %d facts (%v), want >= 3", len(trail), err)
	}

	tail, err := s.LastFacts(ctx, 3)
	if err != nil || len(tail) != 3 {
		t.Fatalf("last facts = %d (%v)", len(tail), err)
	}
}

func TestPostgresWatermark(t *testing.T) {
	s := testPostgresStore(t)
	ctx := context.Background()

	if seq, exists, err := s.Watermark(ctx, "consumer-a"); err != nil || seq != 0 || exists {
		t.Fatalf("absent watermark = %d/%v (%v), want 0/false", seq, exists, err)
	}

	if err := s.SaveWatermark(ctx, "consumer-a", 9); err != nil {
		t.Fatalf("SaveWatermark: %v", err)
	}

	// The monotonic guard holds under the GREATEST upsert.
	if err := s.SaveWatermark(ctx, "consumer-a", 4); err != nil {
		t.Fatalf("SaveWatermark regression: %v", err)
	}

	if seq, exists, err := s.Watermark(ctx, "consumer-a"); err != nil || seq != 9 || !exists {
		t.Fatalf("watermark after regression = %d/%v (%v), want 9/true", seq, exists, err)
	}
}

// TestPostgresClaimExclusivityUnderConcurrency hammers ClaimDue from
// parallel goroutines (separate pool connections — SKIP LOCKED's job) and
// asserts no task is claimed twice.
func TestPostgresClaimExclusivityUnderConcurrency(t *testing.T) {
	s := testPostgresStore(t)
	ctx := context.Background()

	const n = 20

	ids := make(map[string]bool, n)
	for i := range n {
		enq, err := s.Enqueue(ctx, task.New{Type: "sh", Project: "conf-race", Priority: i % 3})
		if err != nil {
			t.Fatal(err)
		}

		ids[enq.ID.String()] = true
	}

	const workers = 4

	claimed := make(chan string, n)

	for w := range workers {
		go func() {
			for range n {
				got, err := s.ClaimDue(ctx, "race-w"+string(rune('0'+w)), time.Minute)
				if errors.Is(err, ErrNoTaskDue) {
					return
				}

				if err != nil {
					t.Errorf("claim: %v", err)

					return
				}

				claimed <- got.ID.String()
			}
		}()
	}

	seen := make(map[string]bool)

	for len(seen) < n {
		select {
		case id := <-claimed:
			if seen[id] {
				t.Fatalf("task %s claimed twice — SKIP LOCKED exclusivity broken", id)
			}

			seen[id] = true
		case <-time.After(10 * time.Second):
			t.Fatalf("claim stall: %d/%d claimed", len(seen), n)
		}
	}
}

// TestPostgresOrphanMarking: an expired-lease Running task is recorded as
// orphaned exactly once (fresh database, so the short lease never races
// bystander claims).
func TestPostgresOrphanMarking(t *testing.T) {
	s := testPostgresStore(t)
	ctx := context.Background()

	orphan, err := s.Enqueue(ctx, task.New{Type: "sh", Project: "conf-orphan"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.ClaimDue(ctx, "victim", time.Nanosecond); err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Millisecond)

	if n, err := s.MarkOrphaned(ctx, time.Now()); err != nil || n != 1 {
		t.Fatalf("mark-orphaned = %d (%v), want 1", n, err)
	}

	if n, err := s.MarkOrphaned(ctx, time.Now()); err != nil || n != 0 {
		t.Fatalf("second mark-orphaned = %d (%v), want 0", n, err)
	}

	got, err := s.Get(ctx, orphan.ID)
	if err != nil || got.Status != task.Running {
		t.Fatalf("orphan status = %s (%v), want still running (observation only)", got.Status, err)
	}
}

// TestPostgresBaseline1k measures Postgres throughput for the ADR-0007
// record (env-gated like the SQLite baseline). Skipped unless both
// TQ_TEST_POSTGRES and TQ_BASELINE are set.
func TestPostgresBaseline1k(t *testing.T) {
	if os.Getenv("TQ_BASELINE") == "" {
		t.Skip("on-demand baseline: run with TQ_BASELINE=1")
	}

	s := testPostgresStore(t)
	ctx := context.Background()

	const n = 1_000

	start := time.Now()
	for i := range n {
		if _, err := s.Enqueue(ctx, task.New{Type: "sh", Project: "bench"}); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}

	enqueueDur := time.Since(start)

	start = time.Now()
	const work = 1_000
	for range work {
		got, err := s.ClaimDue(ctx, "bench", time.Minute)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}

		if err := s.Complete(ctx, got.ID, "bench", nil); err != nil {
			t.Fatalf("complete: %v", err)
		}
	}

	claimDur := time.Since(start)

	t.Logf("postgres baseline 1k: enqueue %d in %v (%.0f/s), claim+complete %d in %v (%.0f/s)",
		n, enqueueDur, float64(n)/enqueueDur.Seconds(),
		work, claimDur, float64(work)/claimDur.Seconds())
}
