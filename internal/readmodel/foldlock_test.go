package readmodel_test

import (
	"context"
	"encoding/json/jsontext"
	"errors"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/readmodel"
)

// assertStats pins a model's status counters against the wanted matrix
// and against internal exactness: every row holds exactly one status, so
// the counter sum must equal the row count — a double-folded transition
// (the corruption the fold lock exists to prevent) breaks the sum before
// it breaks a spot value.
func assertStats(t *testing.T, m *readmodel.Model, want map[string]int) {
	t.Helper()

	byStatus, _, err := m.Stats(context.Background(), readmodel.TaskFilter{})
	if err != nil {
		t.Fatalf("stats: %v", err)
	}

	if !statusesEqual(byStatus, want) {
		t.Fatalf("status counts = %v, want %v", byStatus, want)
	}

	rows, err := m.Tasks(context.Background(), readmodel.TaskFilter{})
	if err != nil {
		t.Fatalf("export rows: %v", err)
	}

	total := 0
	for _, n := range byStatus {
		total += n
	}

	if total != len(rows) {
		t.Fatalf("counter sum = %d over %d rows — counters diverged from the ledger", total, len(rows))
	}
}

func statusesEqual(got, want map[string]int) bool {
	if len(got) != len(want) {
		return false
	}

	for status, n := range want {
		if got[status] != n {
			return false
		}
	}

	return true
}

// TestFoldLockElectsSingleFolderOverOneHome pins the projection home's
// single-writer protocol: the first folding Open wins the fold lock, the
// second mounts read-only (no fold, no checkpoint — store-backed reads
// of the holder's convergence), and after the holder releases, the next
// opener re-wins and converges WITHOUT double-counting — the Delta
// counters are not idempotent, so a second folder would inflate every
// transition the holder already folded.
func TestFoldLockElectsSingleFolderOverOneHome(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store, err := sqlite.Open(t.TempDir() + "/queue.db")
	if err != nil {
		t.Fatalf("open queue store: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	f := &fixture{t: t, store: store}
	t1 := f.enqueue("web", "sh", 1, "fold-a")

	tk, claim := f.claim()
	if tk.ID != t1.ID {
		t.Fatalf("claim = %s, want %s", tk.ID, t1.ID)
	}

	f.must("complete t1", f.store.Complete(ctx, t1.ID, claim, jsontext.Value(`{}`)))

	f.enqueue("api", "sh", 1, "fold-b")

	proj := t.TempDir() + "/projection.db"

	folder, err := readmodel.Open(ctx, proj, store, readmodel.WithDurableCursor())
	if err != nil {
		t.Fatalf("open folder: %v", err)
	}

	folder.CatchUp(ctx)

	if folder.ReadOnly() {
		t.Fatal("first folder mounted read-only — the election must be won by the first opener")
	}

	assertStats(t, folder, map[string]int{"completed": 1, "pending": 1})

	// The second folder loses the election and mounts read-only: its
	// CatchUp pulls ZERO facts (no fold, no checkpoint) and its reads
	// show exactly the holder's convergence.
	counted := &countingStore{Store: store}

	reader, err := readmodel.Open(ctx, proj, counted, readmodel.WithDurableCursor())
	if err != nil {
		t.Fatalf("open second model: %v", err)
	}

	t.Cleanup(func() { _ = reader.Close() })

	if !reader.ReadOnly() {
		t.Fatal("second model won the fold lock — two folders over one home")
	}

	if err := reader.CatchUp(ctx); err != nil {
		t.Fatalf("read-only catch up: %v", err)
	}

	if counted.factsSeen != 0 {
		t.Fatalf("read-only CatchUp pulled %d fact(s) — a reader must never fold", counted.factsSeen)
	}

	if got := reader.JournalCursor(); got != folder.JournalCursor() {
		t.Fatalf("read-only cursor = %d, want the folder's checkpoint %d", got, folder.JournalCursor())
	}

	assertStats(t, reader, map[string]int{"completed": 1, "pending": 1})

	// New facts fold ONLY through the holder. The reader's cursor sits
	// behind the checkpoint (it never folded), so an unguarded CatchUp
	// would re-fold the new transition and inflate pending to 3 —
	// exactly the corruption class the lock prevents.
	f.enqueue("api", "sh", 1, "fold-c")

	folder.CatchUp(ctx)

	assertStats(t, folder, map[string]int{"completed": 1, "pending": 2})
	assertStats(t, reader, map[string]int{"completed": 1, "pending": 2})

	if counted.factsSeen != 0 {
		t.Fatalf("read-only CatchUp folded after new facts: %d fact(s) pulled", counted.factsSeen)
	}

	// The holder releases; the next opener re-wins and converges from
	// the checkpoint — counters stay EXACT (sum == rows, no inflation).
	if err := folder.Close(); err != nil {
		t.Fatalf("close folder: %v", err)
	}

	resumed, err := readmodel.Open(ctx, proj, store, readmodel.WithDurableCursor())
	if err != nil {
		t.Fatalf("reopen after release: %v", err)
	}

	t.Cleanup(func() { _ = resumed.Close() })

	if resumed.ReadOnly() {
		t.Fatal("opener after a released lock must win the election")
	}

	if err := resumed.CatchUp(ctx); err != nil {
		t.Fatalf("resumed catch up: %v", err)
	}

	assertStats(t, resumed, map[string]int{"completed": 1, "pending": 2})
}

// TestReadOnlyMountIsTheExplicitReader pins WithReadOnly: an explicit
// reader never contends for the fold lock (a folder may hold it or not),
// never folds, and Run parks until the caller is done instead of exiting
// into a pump's restart loop.
func TestReadOnlyMountIsTheExplicitReader(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store, err := sqlite.Open(t.TempDir() + "/queue.db")
	if err != nil {
		t.Fatalf("open queue store: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	f := &fixture{t: t, store: store}
	f.enqueue("web", "sh", 1, "ro-a")

	proj := t.TempDir() + "/projection.db"

	folder, err := readmodel.Open(ctx, proj, store, readmodel.WithDurableCursor())
	if err != nil {
		t.Fatalf("open folder: %v", err)
	}

	t.Cleanup(func() { _ = folder.Close() })

	folder.CatchUp(ctx)

	reader, err := readmodel.Open(ctx, proj, store, readmodel.WithReadOnly())
	if err != nil {
		t.Fatalf("open read-only reader: %v", err)
	}

	t.Cleanup(func() { _ = reader.Close() })

	if !reader.ReadOnly() {
		t.Fatal("WithReadOnly must mount read-only")
	}

	assertStats(t, reader, map[string]int{"pending": 1})

	// The explicit reader never held the lock: a folder elected AFTER it
	// still wins the election.
	folder.Close()

	next, err := readmodel.Open(ctx, proj, store, readmodel.WithDurableCursor())
	if err != nil {
		t.Fatalf("open next folder: %v", err)
	}

	t.Cleanup(func() { _ = next.Close() })

	if next.ReadOnly() {
		t.Fatal("WithReadOnly reader must not contend for the fold lock")
	}

	// Run on a read-only mount parks until ctx is done.
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)

	go func() { done <- reader.Run(runCtx) }()

	select {
	case err := <-done:
		t.Fatalf("read-only Run returned early: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("read-only Run = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("read-only Run did not unpark on cancel")
	}
}

// TestAcquireFoldLockBoundedWait pins the lease mechanics: a held lock
// fails the bounded wait with ErrFoldLocked, release lets the next
// acquirer win, and Release is idempotent.
func TestAcquireFoldLockBoundedWait(t *testing.T) {
	t.Parallel()

	home := t.TempDir() + "/projection.db"

	lock, err := readmodel.AcquireFoldLock(home, 0)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}

	if _, err := readmodel.AcquireFoldLock(home, 150*time.Millisecond); !errors.Is(err, readmodel.ErrFoldLocked) {
		t.Fatalf("contended acquire = %v, want ErrFoldLocked", err)
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("double release = %v, want nil (idempotent)", err)
	}

	again, err := readmodel.AcquireFoldLock(home, 0)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}

	if err := again.Release(); err != nil {
		t.Fatalf("release again: %v", err)
	}
}
