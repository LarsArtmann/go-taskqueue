package queue

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func openTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := OpenSQLite(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestEnqueueAndClaim(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	got, err := s.Enqueue(ctx, task.New{Project: "go-cqrs-lite", Type: "lint", Payload: json.RawMessage(`{"x":1}`)})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if got.Status != task.Pending || got.MaxAttempts != task.DefaultMaxAttempts {
		t.Fatalf("defaults not applied: %+v", got)
	}

	claimed, err := s.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}
	if claimed.ID != got.ID {
		t.Fatalf("claimed %s, want %s", claimed.ID, got.ID)
	}
	if claimed.Status != task.Running || claimed.LeaseOwner != "w1" || claimed.LeaseExpires == nil {
		t.Fatalf("claim state wrong: %+v", claimed)
	}

	if _, err := s.ClaimDue(ctx, "w2", time.Minute); !errors.Is(err, ErrNoTaskDue) {
		t.Fatalf("second claim err = %v, want ErrNoTaskDue", err)
	}
}

func TestCompleteVerifiesLease(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	tk, _ := s.Enqueue(ctx, task.New{Type: "a"})
	if _, err := s.ClaimDue(ctx, "w1", time.Minute); err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}

	if err := s.Complete(ctx, tk.ID, "w2", nil); !errors.Is(err, task.ErrLeaseNotHeld) {
		t.Fatalf("Complete by wrong owner err = %v, want ErrLeaseNotHeld", err)
	}
	if err := s.Complete(ctx, tk.ID, "w1", json.RawMessage(`{"ok":true}`)); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	got, _ := s.Get(ctx, tk.ID)
	if got.Status != task.Completed || got.CompletedAt == nil {
		t.Fatalf("post-complete state wrong: %+v", got)
	}
	// Idempotent-ish: second complete is a lease error, not corruption.
	if err := s.Complete(ctx, tk.ID, "w1", nil); !errors.Is(err, task.ErrLeaseNotHeld) {
		t.Fatalf("double complete err = %v, want ErrLeaseNotHeld", err)
	}
}

func TestFailRetriesThenDeadLetters(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	tk, _ := s.Enqueue(ctx, task.New{Type: "flaky", MaxAttempts: 2})

	// Attempt 1: fail -> back to pending.
	if _, err := s.ClaimDue(ctx, "w1", time.Minute); err != nil {
		t.Fatalf("claim1: %v", err)
	}
	if err := s.Fail(ctx, tk.ID, "w1", "boom-1", 250*time.Millisecond); err != nil {
		t.Fatalf("fail1: %v", err)
	}
	got, _ := s.Get(ctx, tk.ID)
	if got.Status != task.Pending || got.Attempts != 1 || got.LastError != "boom-1" {
		t.Fatalf("after fail1: %+v", got)
	}

	// Backoff gates the retry until not_before passes. 250ms comfortably
	// exceeds claim-check latency on a loaded machine (1ms did not).
	if _, err := s.ClaimDue(ctx, "w1", time.Minute); !errors.Is(err, ErrNoTaskDue) {
		t.Fatalf("claim during backoff err = %v, want ErrNoTaskDue", err)
	}
	time.Sleep(300 * time.Millisecond)

	// Attempt 2: fail -> dead (maxAttempts=2).
	if _, err := s.ClaimDue(ctx, "w1", time.Minute); err != nil {
		t.Fatalf("claim2: %v", err)
	}
	if err := s.Fail(ctx, tk.ID, "w1", "boom-2", 0); err != nil {
		t.Fatalf("fail2: %v", err)
	}
	got, _ = s.Get(ctx, tk.ID)
	if got.Status != task.Dead || got.Attempts != 2 {
		t.Fatalf("after fail2: %+v", got)
	}

	// Facts: enqueued, claimed, failed, claimed, failed, dead-lettered.
	facts, _ := s.Facts(ctx, 0)
	wantTypes := []journal.FactType{
		journal.Enqueued, journal.Claimed, journal.Failed,
		journal.Claimed, journal.Failed, journal.DeadLettered,
	}
	if len(facts) != len(wantTypes) {
		t.Fatalf("got %d facts, want %d", len(facts), len(wantTypes))
	}
	for i, ft := range wantTypes {
		if facts[i].Type != ft {
			t.Errorf("facts[%d].Type = %s, want %s", i, facts[i].Type, ft)
		}
	}

	// Rescue: dead -> pending with fresh budget.
	if err := s.RescueDead(ctx, tk.ID, 3); err != nil {
		t.Fatalf("RescueDead: %v", err)
	}
	got, _ = s.Get(ctx, tk.ID)
	if got.Status != task.Pending || got.Attempts != 0 || got.MaxAttempts != 3 {
		t.Fatalf("after rescue: %+v", got)
	}
}

func TestLeaseExpiryAllowsReclaim(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	tk, _ := s.Enqueue(ctx, task.New{Type: "a"})
	if _, err := s.ClaimDue(ctx, "crashed-worker", 30*time.Millisecond); err != nil {
		t.Fatalf("claim: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	// Another worker can claim once the lease expired.
	got, err := s.ClaimDue(ctx, "w2", time.Minute)
	if err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	if got.ID != tk.ID || got.LeaseOwner != "w2" {
		t.Fatalf("reclaimed by wrong task/owner: %+v", got)
	}
	// Old owner cannot complete anymore.
	if err := s.Complete(ctx, tk.ID, "crashed-worker", nil); !errors.Is(err, task.ErrLeaseNotHeld) {
		t.Fatalf("stale owner complete err = %v, want ErrLeaseNotHeld", err)
	}
}

func TestDepsBlockUntilCompleted(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	parent, _ := s.Enqueue(ctx, task.New{Type: "build"})
	child, _ := s.Enqueue(ctx, task.New{Type: "test", Deps: []task.ID{parent.ID}})

	// First claim is the parent (claimable); the child must NOT be claimable
	// while the parent is running.
	got, err := s.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("claim parent: %v", err)
	}
	if got.ID != parent.ID {
		t.Fatalf("first claim %s, want parent %s", got.ID, parent.ID)
	}
	if _, err := s.ClaimDue(ctx, "w1", time.Minute); !errors.Is(err, ErrNoTaskDue) {
		t.Fatalf("child claimable while parent running: err = %v", err)
	}
	if err := s.Complete(ctx, parent.ID, "w1", nil); err != nil {
		t.Fatalf("complete parent: %v", err)
	}
	// Now the child is claimable.
	got, err = s.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("claim child: %v", err)
	}
	if got.ID != child.ID {
		t.Fatalf("claimed %s, want child %s", got.ID, child.ID)
	}
}

func TestPriorityOrdersClaims(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	low, _ := s.Enqueue(ctx, task.New{Type: "low", Priority: 1})
	high, _ := s.Enqueue(ctx, task.New{Type: "high", Priority: 10})
	got, err := s.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if got.ID != high.ID {
		t.Fatalf("claimed %s (%s), want high-priority %s", got.ID, got.Type, high.ID)
	}
	_ = low
}

func TestNotBeforeDelays(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	if _, err := s.Enqueue(ctx, task.New{Type: "later", NotBefore: time.Now().Add(time.Hour)}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if _, err := s.ClaimDue(ctx, "w1", time.Minute); !errors.Is(err, ErrNoTaskDue) {
		t.Fatalf("future task claimable: err = %v", err)
	}
}

func TestHeartbeatExtendsLease(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	tk, _ := s.Enqueue(ctx, task.New{Type: "a"})
	if _, err := s.ClaimDue(ctx, "w1", 40*time.Millisecond); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if err := s.Heartbeat(ctx, tk.ID, "w1", time.Minute); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	time.Sleep(60 * time.Millisecond) // original lease would be gone
	if err := s.Heartbeat(ctx, tk.ID, "w1", time.Minute); err != nil {
		t.Fatalf("heartbeat after original expiry (should be extended): %v", err)
	}
	if err := s.Heartbeat(ctx, tk.ID, "w2", time.Minute); !errors.Is(err, task.ErrLeaseNotHeld) {
		t.Fatalf("wrong-owner heartbeat err = %v", wantLeaseErr())
	}
}

func TestCancelPendingOnly(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	tk, _ := s.Enqueue(ctx, task.New{Type: "a"})
	if err := s.Cancel(ctx, tk.ID); err != nil {
		t.Fatalf("cancel pending: %v", err)
	}
	got, _ := s.Get(ctx, tk.ID)
	if got.Status != task.Cancelled {
		t.Fatalf("after cancel: %+v", got)
	}
	if err := s.Cancel(ctx, tk.ID); !errors.Is(err, task.ErrInvalidTransition) {
		t.Fatalf("double cancel err = %v, want ErrInvalidTransition", err)
	}
}

func TestListFilters(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	_, _ = s.Enqueue(ctx, task.New{Project: "p1", Type: "x"})
	_, _ = s.Enqueue(ctx, task.New{Project: "p2", Type: "x"})

	proj := "p1"
	got, err := s.List(ctx, Filter{Project: &proj})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].Project != "p1" {
		t.Fatalf("project filter: %+v", got)
	}

	st := task.Pending
	got, err = s.List(ctx, Filter{Status: &st})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("status filter len = %d, want 2", len(got))
	}

	all, _ := s.List(ctx, Filter{})
	if len(all) != 2 {
		t.Fatalf("no filter len = %d, want 2", len(all))
	}
}

func TestGetNotFound(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	if _, err := s.Get(ctx, task.ID("nope")); !errors.Is(err, task.ErrNotFound) {
		t.Fatalf("Get err = %v, want ErrNotFound", err)
	}
}

func TestEmptyTypeRejected(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	if _, err := s.Enqueue(ctx, task.New{Type: ""}); err == nil {
		t.Fatal("empty type accepted")
	}
}

func wantLeaseErr() error { return task.ErrLeaseNotHeld }

func TestEnqueueDedupKey(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	first, err := s.Enqueue(ctx, task.New{Project: "demo", Type: "agent", DedupKey: "todo:demo:abc"})
	if err != nil {
		t.Fatalf("first Enqueue: %v", err)
	}
	second, err := s.Enqueue(ctx, task.New{Project: "demo", Type: "agent", DedupKey: "todo:demo:abc"})
	if err != nil {
		t.Fatalf("second Enqueue: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("dedup enqueue returned new task: %s vs %s", first.ID, second.ID)
	}

	tasks, err := s.List(ctx, Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("stored %d tasks, want 1", len(tasks))
	}

	facts, err := s.Facts(ctx, 0)
	if err != nil {
		t.Fatalf("Facts: %v", err)
	}
	enqueued := 0
	for _, f := range facts {
		if f.Type == journal.Enqueued {
			enqueued++
		}
	}
	if enqueued != 1 {
		t.Fatalf("journal has %d task.enqueued facts, want 1 (no duplicate on suppressed enqueue)", enqueued)
	}
}

func TestEnqueueWithoutDedupKeyIndependent(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	a, err := s.Enqueue(ctx, task.New{Type: "sh"})
	if err != nil {
		t.Fatalf("enqueue a: %v", err)
	}
	b, err := s.Enqueue(ctx, task.New{Type: "sh"})
	if err != nil {
		t.Fatalf("enqueue b: %v", err)
	}
	if a.ID == b.ID {
		t.Fatal("tasks without dedup key must be independent")
	}
}

func TestMigrateAddsDedupKeyToOldDatabase(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "old.db")

	// Simulate a pre-dedup_key database: create the table without the column.
	old := fmt.Sprintf(`CREATE TABLE tasks (
		id TEXT PRIMARY KEY,
		project TEXT NOT NULL DEFAULT '',
		type TEXT NOT NULL,
		payload TEXT NOT NULL DEFAULT '',
		deps TEXT NOT NULL DEFAULT '[]',
		priority INTEGER NOT NULL DEFAULT 0,
		attempts INTEGER NOT NULL DEFAULT 0,
		max_attempts INTEGER NOT NULL DEFAULT 3,
		not_before INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'pending',
		lease_owner TEXT NOT NULL DEFAULT '',
		lease_expires INTEGER,
		last_error TEXT NOT NULL DEFAULT '',
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		completed_at INTEGER
	);`)
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", dbPath)
	legacy, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open legacy: %v", err)
	}
	if _, err := legacy.Exec(old); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy: %v", err)
	}

	s, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite with legacy schema: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	if _, err := s.Enqueue(ctx, task.New{Type: "sh", DedupKey: "k1"}); err != nil {
		t.Fatalf("enqueue after migration: %v", err)
	}
	if _, err := s.Enqueue(ctx, task.New{Type: "sh", DedupKey: "k1"}); err != nil {
		t.Fatalf("idempotent enqueue after migration: %v", err)
	}
	tasks, err := s.List(ctx, Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("stored %d tasks, want 1", len(tasks))
	}
}
