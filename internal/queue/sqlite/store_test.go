package sqlite

import (
	"context"
	"database/sql"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
	_ "modernc.org/sqlite"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()

	s, err := Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	return s
}

func TestEnqueueAndClaim(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	got, err := s.Enqueue(ctx, task.New{Project: "go-cqrs-lite", Type: "lint", Payload: jsontext.Value(`{"x":1}`)})
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

	if _, err := s.ClaimDue(ctx, "w2", time.Minute); !errors.Is(err, queue.ErrNoTaskDue) {
		t.Fatalf("second claim err = %v, want queue.ErrNoTaskDue", err)
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

	if err := s.Complete(ctx, tk.ID, "w1", jsontext.Value(`{"ok":true}`)); err != nil {
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

	if err := s.Fail(ctx, tk.ID, "w1", "boom-1", 250*time.Millisecond, nil); err != nil {
		t.Fatalf("fail1: %v", err)
	}

	got, _ := s.Get(ctx, tk.ID)
	if got.Status != task.Pending || got.Attempts != 1 || got.LastError != "boom-1" {
		t.Fatalf("after fail1: %+v", got)
	}

	// Backoff gates the retry until not_before passes. 250ms comfortably
	// exceeds claim-check latency on a loaded machine (1ms did not).
	if _, err := s.ClaimDue(ctx, "w1", time.Minute); !errors.Is(err, queue.ErrNoTaskDue) {
		t.Fatalf("claim during backoff err = %v, want queue.ErrNoTaskDue", err)
	}

	time.Sleep(300 * time.Millisecond)

	// Attempt 2: fail -> dead (maxAttempts=2).
	if _, err := s.ClaimDue(ctx, "w1", time.Minute); err != nil {
		t.Fatalf("claim2: %v", err)
	}

	if err := s.Fail(ctx, tk.ID, "w1", "boom-2", 0, nil); err != nil {
		t.Fatalf("fail2: %v", err)
	}

	got, _ = s.Get(ctx, tk.ID)
	if got.Status != task.Dead || got.Attempts != 2 {
		t.Fatalf("after fail2: %+v", got)
	}

	// Facts: enqueued, claimed, failed, claimed, failed, dead-lettered.
	facts, _ := s.Facts(ctx, 0, 0)

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

	if _, err := s.ClaimDue(ctx, "w1", time.Minute); !errors.Is(err, queue.ErrNoTaskDue) {
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

	if _, err := s.ClaimDue(ctx, "w1", time.Minute); !errors.Is(err, queue.ErrNoTaskDue) {
		t.Fatalf("future task claimable: err = %v", err)
	}
}

func TestHeartbeatExtendsLease(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	tk, _ := s.Enqueue(ctx, task.New{Type: "a"})
	// The original lease must survive the claim→heartbeat gap on slow
	// CI runners (Windows once took >40ms and the lease expired before
	// the first heartbeat), so keep it generous; only the sleep after the
	// heartbeat has to outlast it.
	if _, err := s.ClaimDue(ctx, "w1", 500*time.Millisecond); err != nil {
		t.Fatalf("claim: %v", err)
	}

	if err := s.Heartbeat(ctx, tk.ID, "w1", time.Minute); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}

	time.Sleep(600 * time.Millisecond) // original lease would be gone

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
	if err := s.Cancel(ctx, tk.ID, ""); err != nil {
		t.Fatalf("cancel pending: %v", err)
	}

	got, _ := s.Get(ctx, tk.ID)
	if got.Status != task.Cancelled {
		t.Fatalf("after cancel: %+v", got)
	}

	if err := s.Cancel(ctx, tk.ID, ""); !errors.Is(err, task.ErrInvalidTransition) {
		t.Fatalf("double cancel err = %v, want ErrInvalidTransition", err)
	}
}

func TestListFilters(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	_, _ = s.Enqueue(ctx, task.New{Project: "p1", Type: "x"})
	_, _ = s.Enqueue(ctx, task.New{Project: "p2", Type: "x"})

	proj := "p1"

	got, err := s.List(ctx, queue.Filter{Project: &proj})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(got) != 1 || got[0].Project != "p1" {
		t.Fatalf("project filter: %+v", got)
	}

	st := task.Pending

	got, err = s.List(ctx, queue.Filter{Status: &st})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("status filter len = %d, want 2", len(got))
	}

	all, _ := s.List(ctx, queue.Filter{})
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

	tasks, err := s.List(ctx, queue.Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("stored %d tasks, want 1", len(tasks))
	}

	facts, err := s.Facts(ctx, 0, 0)
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
	old := `CREATE TABLE tasks (
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
	);`
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

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open with legacy schema: %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	if _, err := s.Enqueue(ctx, task.New{Type: "sh", DedupKey: "k1"}); err != nil {
		t.Fatalf("enqueue after migration: %v", err)
	}

	if _, err := s.Enqueue(ctx, task.New{Type: "sh", DedupKey: "k1"}); err != nil {
		t.Fatalf("idempotent enqueue after migration: %v", err)
	}

	tasks, err := s.List(ctx, queue.Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("stored %d tasks, want 1", len(tasks))
	}
}

func TestWatermarkAbsentReturnsZero(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	seq, exists, err := s.Watermark(ctx, "papdashboard:http://stub:1")
	if err != nil {
		t.Fatalf("Watermark absent: %v", err)
	}

	if seq != 0 || exists {
		t.Fatalf("absent consumer = %d/%v, want 0/false", seq, exists)
	}
}

func TestWatermarkSaveAndReadRoundtrip(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	if err := s.SaveWatermark(ctx, "consumer-a", 42); err != nil {
		t.Fatalf("SaveWatermark: %v", err)
	}

	seq, exists, err := s.Watermark(ctx, "consumer-a")
	if err != nil || !exists {
		t.Fatalf("Watermark: seq=%d exists=%v err=%v", seq, exists, err)
	}

	if seq != 42 {
		t.Fatalf("roundtrip seq = %d, want 42", seq)
	}

	// A checkpointed 0 is a real cursor, distinct from "no row": saving 0
	// marks the consumer as existing.
	if err := s.SaveWatermark(ctx, "consumer-zero", 0); err != nil {
		t.Fatalf("SaveWatermark zero: %v", err)
	}

	zeroSeq, zeroExists, err := s.Watermark(ctx, "consumer-zero")
	if err != nil || zeroSeq != 0 || !zeroExists {
		t.Fatalf("zero cursor = %d/%v (%v), want 0/true", zeroSeq, zeroExists, err)
	}

	// Distinct consumers hold independent cursors.
	if err := s.SaveWatermark(ctx, "consumer-b", 7); err != nil {
		t.Fatalf("SaveWatermark consumer-b: %v", err)
	}

	if seq, _, _ := s.Watermark(ctx, "consumer-b"); seq != 7 {
		t.Fatalf("consumer-b seq = %d, want 7", seq)
	}

	if seq, _, _ := s.Watermark(ctx, "consumer-a"); seq != 42 {
		t.Fatalf("consumer-a seq after consumer-b write = %d, want 42", seq)
	}
}

func TestWatermarkMonotonicGuard(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	if err := s.SaveWatermark(ctx, "consumer-a", 100); err != nil {
		t.Fatalf("SaveWatermark 100: %v", err)
	}

	// A lagging or rewound writer must not drag the cursor backwards.
	if err := s.SaveWatermark(ctx, "consumer-a", 30); err != nil {
		t.Fatalf("SaveWatermark regression: %v", err)
	}

	seq, exists, err := s.Watermark(ctx, "consumer-a")
	if err != nil || !exists {
		t.Fatalf("Watermark: seq=%d exists=%v err=%v", seq, exists, err)
	}

	if seq != 100 {
		t.Fatalf("seq after regression attempt = %d, want 100", seq)
	}

	// An equal seq is a no-op, not an error.
	if err := s.SaveWatermark(ctx, "consumer-a", 100); err != nil {
		t.Fatalf("SaveWatermark equal seq: %v", err)
	}

	if seq, _, _ := s.Watermark(ctx, "consumer-a"); seq != 100 {
		t.Fatalf("seq after equal write = %d, want 100", seq)
	}
}

func TestMigrateAddsWatermarksTable(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "old.db")

	// A pre-watermarks database: full legacy tasks table, nothing else.
	old := `CREATE TABLE tasks (
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
		completed_at INTEGER,
		dedup_key TEXT NOT NULL DEFAULT ''
	);`
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

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open with legacy schema: %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	if err := s.SaveWatermark(ctx, "consumer-a", 5); err != nil {
		t.Fatalf("SaveWatermark after migration: %v", err)
	}

	if seq, _, _ := s.Watermark(ctx, "consumer-a"); seq != 5 {
		t.Fatalf("Watermark after migration = %d, want 5", seq)
	}
}

func TestFailPermanentDeadLettersImmediately(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	tk, _ := s.Enqueue(ctx, task.New{Type: "broken", MaxAttempts: 5})
	if _, err := s.ClaimDue(ctx, "w1", time.Minute); err != nil {
		t.Fatalf("claim: %v", err)
	}

	// The lease guard holds for permanent failures too: only the owner
	// that claimed the task may dead-letter it.
	if err := s.FailPermanent(ctx, tk.ID, "w2", "nope", nil); !errors.Is(err, task.ErrLeaseNotHeld) {
		t.Fatalf("wrong-owner FailPermanent err = %v, want ErrLeaseNotHeld", err)
	}

	if err := s.FailPermanent(ctx, tk.ID, "w1", "agent: payload needs repo", nil); err != nil {
		t.Fatalf("FailPermanent: %v", err)
	}

	got, _ := s.Get(ctx, tk.ID)
	if got.Status != task.Dead || got.Attempts != 1 || got.MaxAttempts != 1 {
		t.Fatalf("after FailPermanent: %+v", got)
	}

	// The dead-letter fact carries the error text and its class, so `tq
	// facts` can tell "the task is broken" from "the budget ran out".
	facts, _ := s.Facts(ctx, 0, 0)

	var dl *journal.Fact

	for i := range facts {
		if facts[i].Type == journal.DeadLettered {
			dl = &facts[i]
		}
	}

	if dl == nil {
		t.Fatal("no dead-lettered fact recorded")
	}

	if dl.Error == "" {
		t.Error("dead-lettered fact lost the error text")
	}

	var detail struct {
		Class string `json:"class"`
	}
	if err := json.Unmarshal(dl.Detail, &detail); err != nil || detail.Class != "permanent" {
		t.Errorf("dead-letter detail = %s (%v), want class=permanent", dl.Detail, err)
	}

	// Dead means dead: nothing claimable afterwards.
	if _, err := s.ClaimDue(ctx, "w1", time.Minute); !errors.Is(err, queue.ErrNoTaskDue) {
		t.Fatalf("dead task claimable: err = %v", err)
	}
}

func openTestStoreExclusive(t *testing.T) *Store {
	t.Helper()

	s, err := Open(filepath.Join(t.TempDir(), "q.db"), WithProjectExclusivity())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	return s
}

// TestProjectExclusivitySerializesPerProject pins the store-level per-repo
// guarantee: with the option on, a project with a running task yields no
// further claims (other projects and empty projects unaffected), and the
// blocked sibling becomes claimable the moment the runner completes. Claim
// order is priority/created_at/id — not FIFO — so the test never assumes
// which same-project task goes first.
func TestProjectExclusivitySerializesPerProject(t *testing.T) {
	ctx := context.Background()

	// Default (off): two same-project tasks can both run — opt-in only.
	off := openTestStore(t)
	offA, _ := off.Enqueue(ctx, task.New{Project: "x", Type: "a"})
	offB, _ := off.Enqueue(ctx, task.New{Project: "x", Type: "b"})

	c1, err := off.ClaimDue(ctx, "w1", time.Minute)
	if err != nil || (c1.ID != offA.ID && c1.ID != offB.ID) {
		t.Fatalf("default claim1 = %v, %v", c1.ID, err)
	}

	c2, err := off.ClaimDue(ctx, "w1", time.Minute)
	if err != nil || c2.ID == c1.ID {
		t.Fatalf("default store must allow parallel same-project claims: c1=%v c2=%v, %v", c1.ID, c2.ID, err)
	}

	s := openTestStoreExclusive(t)
	x1, _ := s.Enqueue(ctx, task.New{Project: "repo-x", Type: "a"})
	x2, _ := s.Enqueue(ctx, task.New{Project: "repo-x", Type: "b"})
	other, _ := s.Enqueue(ctx, task.New{Project: "repo-y", Type: "c"})
	empty, _ := s.Enqueue(ctx, task.New{Type: "no-project"})
	xIDs := map[task.ID]bool{x1.ID: true, x2.ID: true}

	var claimed []task.ID

	for {
		got, err := s.ClaimDue(ctx, "w1", time.Minute)
		if errors.Is(err, queue.ErrNoTaskDue) {
			break
		}

		if err != nil {
			t.Fatalf("claim: %v", err)
		}

		claimed = append(claimed, got.ID)
	}

	if len(claimed) != 3 {
		t.Fatalf("claimed %d tasks, want 3 (one repo-x sibling must stay blocked)", len(claimed))
	}

	var xClaimed, blocked task.ID

	for _, id := range claimed {
		if xIDs[id] {
			if xClaimed != "" {
				t.Fatal("both repo-x tasks claimed — exclusivity broken")
			}

			xClaimed = id

			continue
		}

		if id != other.ID && id != empty.ID {
			t.Fatalf("claimed unexpected task %s", id)
		}
	}

	for _, id := range []task.ID{x1.ID, x2.ID} {
		if id != xClaimed {
			blocked = id
		}
	}
	// Nothing due while the repo-x runner holds the project.
	if _, err := s.ClaimDue(ctx, "w1", time.Minute); !errors.Is(err, queue.ErrNoTaskDue) {
		t.Fatalf("blocked sibling claimable: err = %v", err)
	}

	// Completing the runner releases the project.
	if err := s.Complete(ctx, xClaimed, "w1", nil); err != nil {
		t.Fatalf("complete: %v", err)
	}

	got, err := s.ClaimDue(ctx, "w1", time.Minute)
	if err != nil || got.ID != blocked {
		t.Fatalf("after complete claim = %v, %v; want %s", got.ID, err, blocked)
	}
}

// TestProjectExclusivityAcrossStoreHandles proves the guard is store-level,
// not pool-level: two independent Store handles on the same file (the
// multi-process shape) can never both run one project's tasks.
func TestProjectExclusivityAcrossStoreHandles(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "shared.db")

	s1, err := Open(path, WithProjectExclusivity())
	if err != nil {
		t.Fatalf("open s1: %v", err)
	}

	t.Cleanup(func() { _ = s1.Close() })

	s2, err := Open(path, WithProjectExclusivity())
	if err != nil {
		t.Fatalf("open s2: %v", err)
	}

	t.Cleanup(func() { _ = s2.Close() })

	x1, _ := s1.Enqueue(ctx, task.New{Project: "repo-x", Type: "a"})
	x2, _ := s1.Enqueue(ctx, task.New{Project: "repo-x", Type: "b"})

	first, err := s1.ClaimDue(ctx, "pool-1", time.Minute)
	if err != nil {
		t.Fatalf("pool-1 claim: %v", err)
	}

	got, err := s2.ClaimDue(ctx, "pool-2", time.Minute)
	if err == nil && (got.ID == x1.ID || got.ID == x2.ID) {
		t.Fatal("pool-2 claimed a repo-x task while pool-1 runs one — cross-handle exclusivity broken")
	}

	if err == nil {
		if err := s2.Complete(ctx, got.ID, "pool-2", nil); err != nil {
			t.Fatalf("pool-2 complete: %v", err)
		}
	}
	// Releasing pool-1's task lets pool-2 have the sibling.
	if err := s1.Complete(ctx, first.ID, "pool-1", nil); err != nil {
		t.Fatalf("pool-1 complete: %v", err)
	}

	got, err = s2.ClaimDue(ctx, "pool-2", time.Minute)
	if err != nil || (got.ID != x1.ID && got.ID != x2.ID) {
		t.Fatalf("sibling claim after release = %v, %v; want the repo-x sibling", got.ID, err)
	}
}

// TestParkedRequeueNotResurrectableByStaleLease pins the rate-limit park
// contract (13:29 report f16): a Requeue with a future not_before parks the
// task as pending with the lease fully cleared, so NOTHING can bring it
// back early — not the expired-lease reclaim branch (it only matches
// status='running'), and not the stale owner's lease-taking calls.
func TestParkedRequeueNotResurrectableByStaleLease(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	tk, _ := s.Enqueue(ctx, task.New{Type: "agent", MaxAttempts: 3})
	if _, err := s.ClaimDue(ctx, "w1", time.Minute); err != nil {
		t.Fatalf("claim: %v", err)
	}

	// Rate-limit park: requeue with a delay longer than the original lease.
	if err := s.Requeue(ctx, tk.ID, "w1", "rate limited (retry after 1h)", time.Hour); err != nil {
		t.Fatalf("Requeue: %v", err)
	}

	parked, _ := s.Get(ctx, tk.ID)
	if parked.Status != task.Pending || parked.LeaseOwner != "" || parked.LeaseExpires != nil {
		t.Fatalf("parked task must be pending with a cleared lease, got %+v", parked)
	}

	if parked.NotBefore.Before(time.Now().Add(50 * time.Minute)) {
		t.Fatalf("parked not_before = %v, want ~1h out", parked.NotBefore)
	}

	// The expired-lease reclaim branch must not see it: a claim while the
	// park is live returns ErrNoTaskDue (the stale lease is GONE, and the
	// pending branch is gated on not_before).
	if _, err := s.ClaimDue(ctx, "w2", time.Minute); !errors.Is(err, queue.ErrNoTaskDue) {
		t.Fatalf("claim during park err = %v, want ErrNoTaskDue", err)
	}

	// The stale owner cannot resurrect the parked task through any
	// lease-taking call — the parked task holds no lease to match.
	if err := s.Heartbeat(ctx, tk.ID, "w1", time.Minute); !errors.Is(err, task.ErrLeaseNotHeld) {
		t.Errorf("stale Heartbeat err = %v, want ErrLeaseNotHeld", err)
	}

	if err := s.Complete(ctx, tk.ID, "w1", jsontext.Value(`"x"`)); !errors.Is(err, task.ErrLeaseNotHeld) {
		t.Errorf("stale Complete err = %v, want ErrLeaseNotHeld", err)
	}

	if err := s.Requeue(ctx, tk.ID, "w1", "stale", time.Minute); !errors.Is(err, task.ErrLeaseNotHeld) {
		t.Errorf("stale Requeue err = %v, want ErrLeaseNotHeld", err)
	}

	if err := s.Fail(ctx, tk.ID, "w1", "stale", time.Minute, jsontext.Value(`"x"`)); !errors.Is(err, task.ErrLeaseNotHeld) {
		t.Errorf("stale Fail err = %v, want ErrLeaseNotHeld", err)
	}

	if got, _ := s.Get(ctx, tk.ID); got.Status != task.Pending {
		t.Fatalf("parked task mutated by stale calls: %+v", got)
	}
}

func TestRequeueDoesNotBurnAttempts(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	tk, _ := s.Enqueue(ctx, task.New{Type: "env-not-ready", MaxAttempts: 3})
	if _, err := s.ClaimDue(ctx, "w1", time.Minute); err != nil {
		t.Fatalf("claim: %v", err)
	}

	// Wrong owner cannot requeue.
	if err := s.Requeue(ctx, tk.ID, "w2", "nope", time.Second); !errors.Is(err, task.ErrLeaseNotHeld) {
		t.Fatalf("wrong-owner Requeue err = %v, want ErrLeaseNotHeld", err)
	}

	if err := s.Requeue(ctx, tk.ID, "w1", "preflight: repo dirty", 150*time.Millisecond); err != nil {
		t.Fatalf("Requeue: %v", err)
	}

	got, _ := s.Get(ctx, tk.ID)
	if got.Status != task.Pending || got.Attempts != 0 || got.LeaseOwner != "" {
		t.Fatalf("after requeue: %+v (attempt must NOT be burned)", got)
	}

	// Delay gates the next claim (not_before semantics, like Fail backoff).
	if _, err := s.ClaimDue(ctx, "w1", time.Minute); !errors.Is(err, queue.ErrNoTaskDue) {
		t.Fatalf("claim during requeue delay err = %v, want queue.ErrNoTaskDue", err)
	}

	time.Sleep(200 * time.Millisecond)

	if _, err := s.ClaimDue(ctx, "w1", time.Minute); err != nil {
		t.Fatalf("claim after delay: %v", err)
	}

	// The fact log records why the task went back, with no failure.
	facts, _ := s.Facts(ctx, 0, 0)

	var rq bool

	for _, f := range facts {
		if f.Type == journal.Requeued {
			rq = true

			if f.Error == "" {
				t.Error("requeued fact lost the reason")
			}

			var ev queue.RequeueEvidence
			if err := json.Unmarshal(f.Detail, &ev); err != nil {
				t.Fatalf("requeued fact detail not queue.RequeueEvidence: %v (%s)", err, f.Detail)
			}

			if ev.Reason == "" || ev.RetryIn <= 0 {
				t.Errorf("queue.RequeueEvidence = %+v, want reason + retry_in_ms", ev)
			}
		}
	}

	if !rq {
		t.Error("no task.requeued fact recorded")
	}
}

// seedFacts appends n enqueued facts (unique task ids) to the journal.
func seedFacts(ctx context.Context, t *testing.T, s *Store, n int) {
	t.Helper()

	for i := range n {
		if _, err := s.Enqueue(
			ctx,
			task.New{Project: "p", Type: "sh", Payload: jsontext.Value(`"true"`)},
		); err != nil {
			t.Fatalf("seed enqueue %d: %v", i, err)
		}
	}
}

func TestFactsCursorBounded(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := openTestStore(t)
	seedFacts(ctx, t, s, 7)

	first, err := s.Facts(ctx, 0, 3)
	if err != nil {
		t.Fatalf("Facts: %v", err)
	}

	if len(first) != 3 || first[0].Seq != 1 || first[2].Seq != 3 {
		t.Fatalf("limit 3 from 0 = %d facts, seqs %d..%d, want 1..3", len(first), first[0].Seq, first[len(first)-1].Seq)
	}

	second, err := s.Facts(ctx, first[len(first)-1].Seq, 3)
	if err != nil {
		t.Fatalf("Facts(cursor): %v", err)
	}

	if len(second) != 3 || second[0].Seq != 4 {
		t.Fatalf("page 2 = %d facts from seq %d, want seqs 4..6", len(second), second[0].Seq)
	}

	all, err := s.Facts(ctx, 0, 0)
	if err != nil {
		t.Fatalf("Facts(unbounded): %v", err)
	}

	if len(all) != 7 {
		t.Fatalf("unbounded = %d facts, want 7", len(all))
	}
}

func TestLastFactsReturnsTailInAscendingOrder(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := openTestStore(t)
	seedFacts(ctx, t, s, 10)

	tail, err := s.LastFacts(ctx, 3)
	if err != nil {
		t.Fatalf("LastFacts: %v", err)
	}

	if len(tail) != 3 || tail[0].Seq != 8 || tail[2].Seq != 10 {
		t.Fatalf("LastFacts(3) = %d facts seqs %d..%d, want 8..10 ascending", len(tail), tail[0].Seq, tail[2].Seq)
	}

	whole, err := s.LastFacts(ctx, 0)
	if err != nil {
		t.Fatalf("LastFacts(unbounded): %v", err)
	}

	if len(whole) != 10 || whole[0].Seq != 1 {
		t.Fatalf("LastFacts(0) = %d facts from seq %d, want all 10 from 1", len(whole), whole[0].Seq)
	}
}

func TestHeadSeq(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := openTestStore(t)

	head, err := s.HeadSeq(ctx)
	if err != nil {
		t.Fatalf("HeadSeq(empty): %v", err)
	}

	if head != 0 {
		t.Fatalf("empty journal head = %d, want 0", head)
	}

	seedFacts(ctx, t, s, 4)

	head, err = s.HeadSeq(ctx)
	if err != nil {
		t.Fatalf("HeadSeq: %v", err)
	}

	if head != 4 {
		t.Fatalf("head = %d, want 4", head)
	}
}

func TestFactsForTaskFiltersAndBounds(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := openTestStore(t)

	a, err := s.Enqueue(ctx, task.New{Project: "p", Type: "sh", Payload: jsontext.Value(`"true"`), DedupKey: "a"})
	if err != nil {
		t.Fatalf("enqueue a: %v", err)
	}

	b, err := s.Enqueue(ctx, task.New{Project: "p", Type: "sh", Payload: jsontext.Value(`"true"`), DedupKey: "b"})
	if err != nil {
		t.Fatalf("enqueue b: %v", err)
	}

	if _, err := s.ClaimDue(ctx, "w1", time.Minute); err != nil {
		t.Fatalf("claim: %v", err)
	}

	// ClaimDue may pick either task; follow whichever one was claimed.
	claimed, err := s.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	claimedFacts, err := s.FactsForTask(ctx, claimed.ID.String(), 0)
	if err != nil {
		t.Fatalf("FactsForTask(claimed): %v", err)
	}

	if len(claimedFacts) != 2 { // enqueued + claimed
		t.Fatalf("claimed task has %d facts, want 2 (enqueued+claimed)", len(claimedFacts))
	}

	for _, f := range claimedFacts {
		if f.TaskID != claimed.ID.String() {
			t.Fatalf("fact for wrong task %s in claimed trail", f.TaskID)
		}
	}

	other := b.ID
	if claimed.ID == b.ID {
		other = a.ID
	}

	otherFacts, err := s.FactsForTask(ctx, other.String(), 1)
	if err != nil {
		t.Fatalf("FactsForTask(other): %v", err)
	}

	// other is the FIRST-claimed task ([enqueued, claimed]); the bound
	// returns the MOST RECENT fact, still in ascending order.
	if len(otherFacts) != 1 || otherFacts[0].Type != journal.Claimed {
		t.Fatalf("bounded trail = %+v, want exactly the most recent fact (claimed)", otherFacts)
	}
}

func TestCountFactsByTypeSince(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := openTestStore(t)
	seedFacts(ctx, t, s, 3)

	n, err := s.CountFacts(ctx, journal.Enqueued, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("CountFacts: %v", err)
	}

	if n != 3 {
		t.Fatalf("enqueued since -1h = %d, want 3", n)
	}

	n, err = s.CountFacts(ctx, journal.Enqueued, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("CountFacts(future): %v", err)
	}

	if n != 0 {
		t.Fatalf("enqueued since +1h = %d, want 0", n)
	}

	n, err = s.CountFacts(ctx, journal.Completed, time.Time{})
	if err != nil {
		t.Fatalf("CountFacts(completed): %v", err)
	}

	if n != 0 {
		t.Fatalf("completed = %d, want 0", n)
	}
}

func TestListQueryPushdown(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := openTestStore(t)

	seed := []task.New{
		{Project: "alpha", Type: "sh", Payload: jsontext.Value(`"echo hello"`)},
		{Project: "beta", Type: "agent", Payload: jsontext.Value(`{"repo":"go-taskqueue","prompt":"fix the bug"}`)},
		{Project: "gamma", Type: "http", Payload: jsontext.Value(`{"url":"https://example.com/ping"}`)},
	}

	for i := range seed {
		if _, err := s.Enqueue(ctx, seed[i]); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	cases := []struct {
		name  string
		query string
		want  int
	}{
		{"payload substring", "hello", 1},
		{"payload json field", "go-taskqueue", 1},
		{"type match", "agent", 1},
		{"project match", "gamma", 1},
		{"case-insensitive", "ECHO HELLO", 1},
		{"no match", "zebra", 0},
		{"matches several", "e", 3},
	}

	for _, tc := range cases {
		got, err := s.List(ctx, queue.Filter{Query: tc.query})
		if err != nil {
			t.Fatalf("List(q=%q): %v", tc.query, err)
		}

		if len(got) != tc.want {
			t.Fatalf("query %q matched %d tasks, want %d", tc.query, len(got), tc.want)
		}
	}
}

func TestListQueryLikeEscaping(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := openTestStore(t)

	seed := []task.New{
		{Project: "pct", Type: "sh", Payload: jsontext.Value(`"progress 100% done"`)},
		{Project: "under", Type: "sh", Payload: jsontext.Value(`"snake_case_name"`)},
		{Project: "plain", Type: "sh", Payload: jsontext.Value(`"nothing special"`)},
	}

	for i := range seed {
		if _, err := s.Enqueue(ctx, seed[i]); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}

	cases := []struct {
		name  string
		query string
		want  int
	}{
		{"literal percent", "100%", 1},
		{"literal underscore", "snake_case", 1},
		{"percent is not a wildcard", "1% done", 0},
		{"underscore is not a wildcard", "snakeXcase", 0},
		{"backslash literal", "100%\\", 0},
	}

	for _, tc := range cases {
		got, err := s.List(ctx, queue.Filter{Query: tc.query})
		if err != nil {
			t.Fatalf("List(q=%q): %v", tc.query, err)
		}

		if len(got) != tc.want {
			t.Fatalf("query %q matched %d tasks, want %d", tc.query, len(got), tc.want)
		}
	}
}

func TestListOffsetPagination(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := openTestStore(t)
	seedFacts(ctx, t, s, 5)

	page1, err := s.List(ctx, queue.Filter{Limit: 2})
	if err != nil {
		t.Fatalf("page1: %v", err)
	}

	if len(page1) != 2 {
		t.Fatalf("page1 = %d rows, want 2", len(page1))
	}

	page2, err := s.List(ctx, queue.Filter{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("page2: %v", err)
	}

	if len(page2) != 2 || page2[0].ID == page1[0].ID {
		t.Fatalf("page2 must not overlap page1, got %d rows", len(page2))
	}

	page3, err := s.List(ctx, queue.Filter{Limit: 2, Offset: 4})
	if err != nil {
		t.Fatalf("page3: %v", err)
	}

	if len(page3) != 1 {
		t.Fatalf("page3 = %d rows, want the 1 remaining", len(page3))
	}
}

func TestStatusCountsAndProjectCounts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := openTestStore(t)

	if _, err := s.Enqueue(ctx, task.New{Project: "a", Type: "sh", Payload: jsontext.Value(`"true"`)}); err != nil {
		t.Fatalf("seed a: %v", err)
	}

	if _, err := s.Enqueue(ctx, task.New{Project: "a", Type: "sh", Payload: jsontext.Value(`"true"`)}); err != nil {
		t.Fatalf("seed a2: %v", err)
	}

	if _, err := s.Enqueue(ctx, task.New{Project: "b", Type: "sh", Payload: jsontext.Value(`"true"`)}); err != nil {
		t.Fatalf("seed b: %v", err)
	}

	// ClaimDue may pick either task; derive expectations from the winner.
	claimed, err := s.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	counts, err := s.StatusCounts(ctx)
	if err != nil {
		t.Fatalf("StatusCounts: %v", err)
	}

	total := 0

	for _, n := range counts {
		total += n
	}

	if total != 3 || counts[task.Pending] != 2 || counts[task.Running] != 1 {
		t.Fatalf("counts = %v, want 2 pending + 1 running", counts)
	}

	projects, err := s.ProjectCounts(ctx)
	if err != nil {
		t.Fatalf("ProjectCounts: %v", err)
	}

	if len(projects) != 2 {
		t.Fatalf("projects = %d, want 2", len(projects))
	}

	// Seeds: project a holds 2 tasks, project b holds 1. Whichever task
	// ClaimDue won defines each project's expected split.
	if projects["b"][task.Pending]+projects["a"][task.Pending] != 2 {
		t.Fatalf("two pendings expected across projects, got %v", projects)
	}

	claimedProject := claimed.Project

	if projects[claimedProject][task.Running] != 1 {
		t.Fatalf("claimed project %s must hold the run, got %v", claimedProject, projects[claimedProject])
	}

	if claimedProject == "a" && projects["a"][task.Pending] != 1 {
		t.Fatalf("project a claimed 1 of its 2, want 1 pending left, got %v", projects["a"])
	}
}

func TestListSeverityOrder(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := openTestStore(t)

	for range 4 {
		if _, err := s.Enqueue(
			ctx,
			task.New{Project: "a", Type: "sh", Payload: jsontext.Value(`"true"`)},
		); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	if _, err := s.ClaimDue(ctx, "w1", time.Minute); err != nil {
		t.Fatalf("claim1: %v", err)
	}

	if _, err := s.ClaimDue(ctx, "w2", time.Minute); err != nil {
		t.Fatalf("claim2: %v", err)
	}

	running, err := s.List(ctx, queue.Filter{Status: new(task.Running)})
	if err != nil {
		t.Fatalf("list running: %v", err)
	}

	if len(running) != 2 {
		t.Fatalf("want 2 running, got %d", len(running))
	}

	if err := s.FailPermanent(ctx, running[0].ID, running[0].LeaseOwner, "boom", nil); err != nil {
		t.Fatalf("fail permanent: %v", err)
	}

	all, err := s.List(ctx, queue.Filter{SeverityOrder: true})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	sawDead, sawRunning, sawPending := 0, 0, 0

	lastRank := -1

	rank := map[task.Status]int{
		task.Dead:      0,
		task.Running:   1,
		task.Pending:   2,
		task.Cancelled: 3,
		task.Completed: 4,
	}

	for _, got := range all {
		if rank[got.Status] < lastRank {
			t.Fatalf("severity order violated at %s: %v", got.ID, got.Status)
		}

		lastRank = rank[got.Status]

		switch got.Status {
		case task.Dead:
			sawDead++
		case task.Running:
			sawRunning++
		case task.Pending:
			sawPending++
		}
	}

	if sawDead != 1 || sawRunning != 1 || sawPending != 2 {
		t.Fatalf(
			"expected 1 dead + 1 running + 2 pending, got dead=%d running=%d pending=%d",
			sawDead,
			sawRunning,
			sawPending,
		)
	}
}

//go:fix inline
func ptrStatus(st task.Status) *task.Status { return new(st) }

func TestCountTasksMatchesList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := openTestStore(t)
	seedFacts(ctx, t, s, 5)

	full, err := s.CountTasks(ctx, queue.Filter{})
	if err != nil {
		t.Fatalf("CountTasks: %v", err)
	}

	if full != 5 {
		t.Fatalf("count = %d, want 5", full)
	}

	listed, err := s.List(ctx, queue.Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(listed) != full {
		t.Fatalf("List(%d) and CountTasks(%d) disagree", len(listed), full)
	}

	qcount, err := s.CountTasks(ctx, queue.Filter{Query: "true"})
	if err != nil {
		t.Fatalf("CountTasks(query): %v", err)
	}

	if qcount != 5 {
		t.Fatalf("query count = %d, want 5 (all payloads contain true)", qcount)
	}

	zero, err := s.CountTasks(ctx, queue.Filter{Query: "nope"})
	if err != nil {
		t.Fatalf("CountTasks(no match): %v", err)
	}

	if zero != 0 {
		t.Fatalf("no-match count = %d, want 0", zero)
	}
}

// TestLoadSnapshotScaleAt100k pins the bounded-read architecture against
// the scale that motivated it: 100k tasks and 100k+ facts must render a
// dashboard snapshot in bounded time and memory — no full-journal or
// full-table scans. Skipped under -short; run explicitly with
// `go test ./internal/queue/ -run TestLoadSnapshotScaleAt100k`.
func TestLoadSnapshotScaleAt100k(t *testing.T) {
	if testing.Short() {
		t.Skip("scale test; run explicitly or without -short")
	}

	if raceDetector {
		t.Skip("latency ceilings are meaningless under the race detector's ~10x overhead; run without -race")
	}

	ctx := context.Background()
	s := openTestStore(t)

	const (
		tasks    = 100_000
		pageSize = 200
	)

	// Bulk-seed through a single transaction: direct inserts of the same
	// rows Enqueue would write (one task + one enqueued fact each).
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}

	now := time.Now().UnixMilli()

	taskStmt, err := tx.PrepareContext(ctx, `INSERT INTO tasks
		(id, project, type, payload, deps, priority, attempts, max_attempts, not_before, status,
		 lease_owner, lease_expires, last_error, created_at, updated_at, completed_at)
		VALUES (?, 'scale', 'sh', '"true"', '[]', 0, 0, 3, 0, 'pending', '', NULL, '', ?, ?, NULL)`)
	if err != nil {
		t.Fatalf("prepare task: %v", err)
	}

	factStmt, err := tx.PrepareContext(ctx,
		`INSERT INTO facts (time, task_id, type, owner, attempt, error, detail)
		 VALUES (?, ?, 'task.enqueued', '', 0, '', '')`)
	if err != nil {
		t.Fatalf("prepare fact: %v", err)
	}

	for i := range tasks {
		id := fmt.Sprintf("scale-%06d", i)

		if _, err := taskStmt.ExecContext(ctx, id, now, now); err != nil {
			t.Fatalf("insert task %d: %v", i, err)
		}

		if _, err := factStmt.ExecContext(ctx, now, id); err != nil {
			t.Fatalf("insert fact %d: %v", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// The dashboard's exact query path, page 1 plus a filtered page.
	start := time.Now()

	snap, err := s.List(ctx, queue.Filter{SeverityOrder: true, Limit: pageSize})
	if err != nil {
		t.Fatalf("page1: %v", err)
	}

	page1 := time.Since(start)

	start = time.Now()

	matches, err := s.CountTasks(ctx, queue.Filter{})
	if err != nil {
		t.Fatalf("count: %v", err)
	}

	count := time.Since(start)

	start = time.Now()

	if _, err := s.CountTasks(ctx, queue.Filter{Query: "scale-09999"}); err != nil {
		t.Fatalf("count query: %v", err)
	}

	queryCount := time.Since(start)

	start = time.Now()

	if _, err := s.LastFacts(ctx, 50); err != nil {
		t.Fatalf("last facts: %v", err)
	}

	facts := time.Since(start)

	start = time.Now()

	if _, err := s.FactsForTask(ctx, "scale-099999", 0); err != nil {
		t.Fatalf("facts for task: %v", err)
	}

	taskFacts := time.Since(start)

	if len(snap) != pageSize {
		t.Fatalf("page1 = %d rows, want %d", len(snap), pageSize)
	}

	if matches != tasks {
		t.Fatalf("count = %d, want %d", matches, tasks)
	}

	t.Logf("SCALE 100k: page1=%v count=%v query-count=%v last-50-facts=%v task-trail=%v",
		page1, count, queryCount, facts, taskFacts)

	// Bounded reads: every path is O(page) or O(log N + page). Generous
	// ceilings catch O(N) regressions (a full 100k scan costs >100ms on
	// this machine, usually far more) without being flaky on busy CI.
	for name, d := range map[string]time.Duration{
		"page1": page1, "count": count, "queryCount": queryCount,
		"facts": facts, "taskFacts": taskFacts,
	} {
		if d > 250*time.Millisecond {
			t.Errorf("%s took %v; bounded reads must stay far below the O(N) wall", name, d)
		}
	}
}

// TestCancelRunningRequestAndHonour pins the cooperative-cancel store
// contract: request (idempotent fact), observation, owner-guarded finalize.
func TestCancelRunningRequestAndHonour(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	tk, err := s.Enqueue(ctx, task.New{Type: "a"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if _, err := s.ClaimDue(ctx, "w1", time.Minute); err != nil {
		t.Fatalf("claim: %v", err)
	}

	// Pending-only Cancel refuses running tasks (that is --force territory).
	if err := s.Cancel(ctx, tk.ID, ""); !errors.Is(err, task.ErrInvalidTransition) {
		t.Fatalf("Cancel on running err = %v, want ErrInvalidTransition", err)
	}

	if err := s.CancelRunning(ctx, tk.ID, ""); err != nil {
		t.Fatalf("CancelRunning: %v", err)
	}

	if requested, err := s.CancelRequested(ctx, tk.ID); err != nil || !requested {
		t.Fatalf("CancelRequested = (%v, %v), want (true, nil)", requested, err)
	}

	// Idempotent: a second request appends nothing.
	if err := s.CancelRunning(ctx, tk.ID, ""); err != nil {
		t.Fatalf("second CancelRunning: %v", err)
	}

	requestedFacts, err := s.CountFacts(ctx, journal.CancelRequested, time.Time{})
	if err != nil {
		t.Fatalf("CountFacts: %v", err)
	}

	if requestedFacts != 1 {
		t.Fatalf("cancel-requested facts = %d, want 1", requestedFacts)
	}

	// Only the lease holder finalizes.
	if err := s.CancelOwned(ctx, tk.ID, "not-the-owner"); !errors.Is(err, task.ErrLeaseNotHeld) {
		t.Fatalf("CancelOwned by wrong owner err = %v, want ErrLeaseNotHeld", err)
	}

	if err := s.CancelOwned(ctx, tk.ID, "w1"); err != nil {
		t.Fatalf("CancelOwned: %v", err)
	}

	got, err := s.Get(ctx, tk.ID)
	if err != nil || got.Status != task.Cancelled {
		t.Fatalf("after CancelOwned: (%v, %v), want status cancelled", got.Status, err)
	}
}

// TestReclaimFinalizesCancelRequest proves the crashed-worker path: an
// expired lease on a cancel-requested task finalizes the cancel instead of
// re-executing the task.
func TestReclaimFinalizesCancelRequest(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	tk, err := s.Enqueue(ctx, task.New{Type: "a"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if _, err := s.ClaimDue(ctx, "crashed-worker", 30*time.Millisecond); err != nil {
		t.Fatalf("claim: %v", err)
	}

	if err := s.CancelRunning(ctx, tk.ID, ""); err != nil {
		t.Fatalf("CancelRunning: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if _, err := s.ClaimDue(ctx, "w2", time.Minute); !errors.Is(err, queue.ErrNoTaskDue) {
		t.Fatalf("ClaimDue after cancel-requested reclaim err = %v, want queue.ErrNoTaskDue", err)
	}

	got, err := s.Get(ctx, tk.ID)
	if err != nil || got.Status != task.Cancelled {
		t.Fatalf("reclaimed task status = %s (err %v), want cancelled", got.Status, err)
	}

	facts, err := s.FactsForTask(ctx, tk.ID.String(), 0)
	if err != nil {
		t.Fatalf("FactsForTask: %v", err)
	}

	sawReleased, sawCancelled := false, false
	for _, f := range facts {
		sawReleased = sawReleased || f.Type == journal.Released
		sawCancelled = sawCancelled || f.Type == journal.Cancelled
	}

	if !sawReleased || !sawCancelled {
		t.Fatalf("finalize facts missing (released=%v cancelled=%v): %+v", sawReleased, sawCancelled, facts)
	}
}

// TestCancelReasonStoredInFactDetail pins the forensics contract: a
// --reason-style Cancel lands in the task.cancelled fact's detail ("reason"
// key), an empty reason stores none, and a cooperative cancel carries the
// operator's reason from the request fact onto the final cancelled fact.
func TestCancelReasonStoredInFactDetail(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	reasonFor := func(t *testing.T, id task.ID, ftype journal.FactType) string {
		t.Helper()

		facts, err := s.FactsForTask(ctx, id.String(), 0)
		if err != nil {
			t.Fatalf("FactsForTask: %v", err)
		}

		for _, f := range facts {
			if f.Type != ftype {
				continue
			}

			var d struct {
				Reason string `json:"reason"`
			}

			_ = json.Unmarshal(f.Detail, &d)

			return d.Reason
		}

		return ""
	}

	// Pending cancel with a reason (the tq cancel --reason path).
	withReason, err := s.Enqueue(ctx, task.New{Type: "a"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if err := s.Cancel(ctx, withReason.ID, "stale: superseded by task-42"); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	if got := reasonFor(t, withReason.ID, journal.Cancelled); got != "stale: superseded by task-42" {
		t.Fatalf("task.cancelled reason = %q, want the stored reason", got)
	}

	// Pending cancel WITHOUT a reason keeps the fact detail empty.
	noReason, err := s.Enqueue(ctx, task.New{Type: "a"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if err := s.Cancel(ctx, noReason.ID, ""); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	facts, err := s.FactsForTask(ctx, noReason.ID.String(), 0)
	if err != nil {
		t.Fatalf("FactsForTask: %v", err)
	}

	for _, f := range facts {
		if f.Type == journal.Cancelled && len(f.Detail) != 0 {
			t.Fatalf("cancel without reason stored detail %q, want empty", f.Detail)
		}
	}

	// Cooperative cancel: the reason rides the request fact and is carried
	// onto the final task.cancelled fact by the owner-side finalize.
	coop, err := s.Enqueue(ctx, task.New{Type: "a"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if _, err := s.ClaimDue(ctx, "w1", time.Minute); err != nil {
		t.Fatalf("claim: %v", err)
	}

	if err := s.CancelRunning(ctx, coop.ID, "duplicate of task-7"); err != nil {
		t.Fatalf("CancelRunning: %v", err)
	}

	if got := reasonFor(t, coop.ID, journal.CancelRequested); got != "duplicate of task-7" {
		t.Fatalf("task.cancel-requested reason = %q, want the stored reason", got)
	}

	if err := s.CancelOwned(ctx, coop.ID, "w1"); err != nil {
		t.Fatalf("CancelOwned: %v", err)
	}

	facts, err = s.FactsForTask(ctx, coop.ID.String(), 0)
	if err != nil {
		t.Fatalf("FactsForTask: %v", err)
	}

	var cancelled journal.Fact

	for _, f := range facts {
		if f.Type == journal.Cancelled {
			cancelled = f
		}
	}

	var d struct {
		Cooperative string `json:"cooperative"`
		Reason      string `json:"reason"`
	}
	if err := json.Unmarshal(cancelled.Detail, &d); err != nil {
		t.Fatalf("cancelled detail %q: %v", cancelled.Detail, err)
	}

	if d.Cooperative != "true" || d.Reason != "duplicate of task-7" {
		t.Fatalf("cooperative cancelled detail = %+v, want cooperative + carried reason", d)
	}
}

// TestMarkOrphanedRecordsStrandedTasks: expired-lease Running tasks get a
// task.orphaned fact (once); live-lease tasks do not; task state is
// untouched — orphaning is an observation, the reclaim stays authoritative.
func TestMarkOrphanedRecordsStrandedTasks(t *testing.T) {
	ctx := context.Background()

	s := openTestStore(t)
	defer func() { _ = s.Close() }()

	stranded, err := s.Enqueue(ctx, task.New{Type: "sh"})
	if err != nil {
		t.Fatal(err)
	}

	// Deterministic despite ClaimDue returning an ARBITRARY due task:
	// claim A as the only task, then claim B immediately — A must not be
	// reclaimable yet (B is the only due task), and A's lease must be dead
	// by the MarkOrphaned call. The wait uses an ABSOLUTE deadline from the
	// victim's claim; the 1s lease makes the two hazards (B reclaiming A in
	// the claim gap; A still alive at the mark) impossible on any runner
	// (a fixed 150ms sleep after the second claim lost both on Windows CI
	// once: marked 0).
	claimAt := time.Now()

	if _, err := s.ClaimDue(ctx, "victim", time.Second); err != nil {
		t.Fatal(err)
	}

	live, err := s.Enqueue(ctx, task.New{Type: "sh"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.ClaimDue(ctx, "alive", time.Minute); err != nil {
		t.Fatal(err)
	}

	// Victim's lease (1s) dies, alive's stays — deadline = claim + 1.3s.
	if wait := time.Until(claimAt.Add(1300 * time.Millisecond)); wait > 0 {
		time.Sleep(wait)
	}

	n, err := s.MarkOrphaned(ctx, time.Now())
	if err != nil {
		t.Fatalf("MarkOrphaned: %v", err)
	}

	if n != 1 {
		t.Fatalf("marked %d tasks, want 1 (only the expired lease)", n)
	}

	facts, err := s.Facts(ctx, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	orphaned := map[string]bool{}

	for _, f := range facts {
		if f.Type == journal.Orphaned {
			orphaned[f.TaskID] = true
		}
	}

	if !orphaned[stranded.ID.String()] {
		t.Error("stranded task has no task.orphaned fact")
	}

	if orphaned[live.ID.String()] {
		t.Error("live-lease task must not be marked orphaned")
	}

	// Observation only: the stranded task is still Running (reclaim will act).
	got, err := s.Get(ctx, stranded.ID)
	if err != nil {
		t.Fatal(err)
	}

	if got.Status != task.Running {
		t.Fatalf("stranded task status = %s, want still running (mark is not a state change)", got.Status)
	}

	// Idempotent: a second pass appends nothing.
	n, err = s.MarkOrphaned(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	if n != 0 {
		t.Fatalf("second MarkOrphaned marked %d, want 0 (idempotent)", n)
	}
}

// TestEnqueueClaimBaseline10k measures queue-op throughput at the round-5
// baseline scale (10k tasks): enqueue rate, claim+complete rate, and a
// page-query. It asserts only CORRECTNESS (counts) — timings are printed
// for the FEATURES baseline and re-measured by hand. Skipped under -short.
func TestEnqueueClaimBaseline10k(t *testing.T) {
	if testing.Short() {
		t.Skip("baseline measurement, skipped under -short")
	}

	if os.Getenv("TQ_BASELINE") == "" {
		t.Skip(
			"on-demand baseline: run with TQ_BASELINE=1 (10k writes are too slow for the default suite, especially under -race)",
		)
	}

	ctx := context.Background()

	s := openTestStore(t)
	defer func() { _ = s.Close() }()

	const n = 10_000

	start := time.Now()

	for i := range n {
		if _, err := s.Enqueue(ctx, task.New{Type: "sh", Project: "baseline"}); err != nil {
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

	counts, err := s.StatusCounts(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if counts[task.Pending] != n-work || counts[task.Completed] != work {
		t.Fatalf("counts = %+v, want pending=%d completed=%d", counts, n-work, work)
	}

	t.Logf("baseline 10k: enqueue %d tasks in %v (%.0f/s), claim+complete %d in %v (%.0f/s)",
		n, enqueueDur, float64(n)/enqueueDur.Seconds(),
		work, claimDur, float64(work)/claimDur.Seconds())
}

// TestArchiveFactsBeforeKeepsProjections (ADR-0006 prototype): terminal
// tasks' facts move to facts_archive; active tasks' facts stay hot; the
// tasks projection and Facts() keep working; the watermark is recorded.
func TestArchiveFactsBeforeKeepsProjections(t *testing.T) {
	ctx := context.Background()

	s := openTestStore(t)
	defer func() { _ = s.Close() }()

	var done [2]task.Task

	for i := range done {
		enq, err := s.Enqueue(ctx, task.New{Type: "sh"})
		if err != nil {
			t.Fatal(err)
		}

		got, err := s.ClaimDue(ctx, "w", time.Minute)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}

		if err := s.Complete(ctx, got.ID, "w", nil); err != nil {
			t.Fatal(err)
		}

		done[i] = enq
	}

	active, err := s.Enqueue(ctx, task.New{Type: "sh"})
	if err != nil {
		t.Fatal(err)
	}

	head, err := s.HeadSeq(ctx)
	if err != nil {
		t.Fatal(err)
	}

	moved, err := s.ArchiveFactsBefore(ctx, head+1)
	if err != nil {
		t.Fatalf("ArchiveFactsBefore: %v", err)
	}

	if moved != 6 { // 2 terminal tasks x (enqueued, claimed, completed)
		t.Fatalf("moved %d facts, want 6", moved)
	}

	// Projections survive: all three tasks still resolvable with state.
	for i, want := range []task.Task{done[0], done[1], active} {
		got, err := s.Get(ctx, want.ID)
		if err != nil {
			t.Fatalf("get %d: %v", i, err)
		}

		wantStatus := task.Completed
		if i == 2 {
			wantStatus = task.Pending
		}

		if got.Status != wantStatus {
			t.Fatalf("task %d status = %s, want %s", i, got.Status, wantStatus)
		}
	}

	// Hot facts now contain ONLY the active task's enqueued fact.
	facts, err := s.Facts(ctx, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(facts) != 1 || facts[0].TaskID != active.ID.String() {
		t.Fatalf("hot facts = %+v, want only the active task's", facts)
	}

	stats, err := s.ArchiveSummary(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if stats.Archived != 6 || stats.Hot != 1 {
		t.Fatalf("summary = %+v, want archived=6 hot=1", stats)
	}

	if stats.Watermark < 0 {
		t.Fatalf("watermark = %d, want >= 0 after archiving", stats.Watermark)
	}

	// Re-run is a no-op (facts already moved; terminal tasks have no hot
	// facts left below any cutoff).
	moved, err = s.ArchiveFactsBefore(ctx, head+100)
	if err != nil {
		t.Fatal(err)
	}

	if moved != 0 {
		t.Fatalf("second pass moved %d, want 0", moved)
	}
}

// TestListSortAllowlist (M18/F94): the allowlisted column sorts order the
// rows as named, and unknown values fall back to the default order instead
// of reaching SQL.
func TestListSortAllowlist(t *testing.T) {
	ctx := context.Background()

	s := openTestStore(t)
	defer func() { _ = s.Close() }()

	for _, prio := range []int{1, 5, 3} {
		if _, err := s.Enqueue(ctx, task.New{Type: "sh", Priority: prio}); err != nil {
			t.Fatal(err)
		}
	}

	tasks, err := s.List(ctx, queue.Filter{Sort: "priority-desc", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}

	if len(tasks) != 3 || tasks[0].Priority < tasks[1].Priority || tasks[1].Priority < tasks[2].Priority {
		t.Fatalf("priority-desc order = %d,%d,%d", tasks[0].Priority, tasks[1].Priority, tasks[2].Priority)
	}

	tasks, err = s.List(ctx, queue.Filter{Sort: "priority-asc", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}

	if tasks[0].Priority > tasks[1].Priority || tasks[1].Priority > tasks[2].Priority {
		t.Fatalf("priority-asc order = %d,%d,%d", tasks[0].Priority, tasks[1].Priority, tasks[2].Priority)
	}

	// Unknown sort: falls back to the default order (priority DESC) — never
	// an error, never interpolated into SQL.
	tasks, err = s.List(ctx, queue.Filter{Sort: "created_at; DROP TABLE tasks", Limit: 10})
	if err != nil {
		t.Fatalf("hostile sort value: %v", err)
	}

	if tasks[0].Priority != 5 {
		t.Fatalf("unknown sort fell through to non-default order: %d first", tasks[0].Priority)
	}
}

// TestListSinceFilter pins the queue.Filter.Since SQL pushdown: inclusive lower
// bound on created_at (the exact boundary task is INCLUDED), and the
// boundary + 1ms excludes it. Both stores share the contract; the Postgres
// twin runs via the conformance battery.
func TestListSinceFilter(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	first, _ := s.Enqueue(ctx, task.New{Type: "a"})

	// Two rapid enqueues can land in the same millisecond (created_at is
	// unix-milli) — separate them so the boundary assertions are exact.
	time.Sleep(2 * time.Millisecond)

	second, _ := s.Enqueue(ctx, task.New{Type: "b"})
	if !second.CreatedAt.After(first.CreatedAt) {
		t.Fatalf("test premise broken: second not newer than first")
	}

	since := first.CreatedAt

	got, err := s.List(ctx, queue.Filter{Since: &since})
	if err != nil {
		t.Fatalf("List inclusive: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("inclusive boundary: %d tasks, want 2 (boundary task included)", len(got))
	}

	after := first.CreatedAt.Add(time.Millisecond)

	got, err = s.List(ctx, queue.Filter{Since: &after})
	if err != nil {
		t.Fatalf("List exclusive: %v", err)
	}

	if len(got) != 1 || got[0].ID != second.ID {
		t.Fatalf("boundary+1ms: %+v, want only the newer task", got)
	}

	// CountTasks shares the WHERE builder — same window, same count.
	n, err := s.CountTasks(ctx, queue.Filter{Since: &after})
	if err != nil {
		t.Fatalf("CountTasks: %v", err)
	}

	if n != 1 {
		t.Fatalf("CountTasks window = %d, want 1", n)
	}
}
