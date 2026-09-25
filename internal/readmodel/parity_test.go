package readmodel_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/readmodel"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// fixture is one store + model pair over temp paths, seeded by the caller.
type fixture struct {
	t     *testing.T
	store *sqlite.Store
	model *readmodel.Model
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	ctx := context.Background()

	store, err := sqlite.Open(t.TempDir() + "/queue.db")
	if err != nil {
		t.Fatalf("open queue store: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	model, err := readmodel.Open(t.TempDir()+"/projection.db", store)
	if err != nil {
		t.Fatalf("open read model: %v", err)
	}

	t.Cleanup(func() { _ = model.Close() })

	if err := model.CatchUp(ctx); err != nil {
		t.Fatalf("catch up: %v", err)
	}

	return &fixture{t: t, store: store, model: model}
}

func (f *fixture) enqueue(project, typ string, priority int, dedup string) task.Task {
	f.t.Helper()

	tk, err := f.store.Enqueue(context.Background(), task.New{
		Project:   project,
		Type:      typ,
		Payload:   `true`,
		Priority:  priority,
		DedupKey:  dedup,
		MaxAttempts: 2,
	})
	if err != nil {
		f.t.Fatalf("enqueue %s/%s: %v", project, typ, err)
	}

	time.Sleep(2 * time.Millisecond) // distinct created_at for the DESC sort

	return tk
}

func (f *fixture) claim(owner string) (task.Task, queue.Claim) {
	f.t.Helper()

	tk, claim, err := f.store.ClaimDue(context.Background(), owner, time.Minute)
	if err != nil {
		f.t.Fatalf("claim as %s: %v", owner, err)
	}

	return tk, claim
}

func (f *fixture) resync() {
	f.t.Helper()

	if err := f.model.CatchUp(context.Background()); err != nil {
		f.t.Fatalf("catch up: %v", err)
	}
}

// rowsByID indexes a model read by task id.
func rowsByID(t *testing.T, rows []readmodel.TaskRow) map[string]readmodel.TaskRow {
	t.Helper()

	byID := make(map[string]readmodel.TaskRow, len(rows))
	for _, r := range rows {
		if _, dup := byID[r.ID]; dup {
			t.Fatalf("duplicate row for %s", r.ID)
		}

		byID[r.ID] = r
	}

	return byID
}

// TestParityWithStoreStore runs a lifecycle battery through the real
// (v4-backed) sqlite store, folds the journal into the read model, and
// pins the collection projection against the store's own reads: status
// counts, per-row state, filters, and the newest-first order.
func TestParityWithStoreProjection(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	// t1 completes; t2 fails once (retry → pending); t3 dies; t4 is
	// cancelled; t5 is requeued; t6 is reprioritized.
	t1 := f.enqueue("web", "sh", 2, "todo:a")
	t2 := f.enqueue("web", "agent", 5, "todo:b")
	t3 := f.enqueue("api", "sh", 0, "todo:c")
	t4 := f.enqueue("api", "review", 4, "todo:d")
	t5 := f.enqueue("web", "sh", 1, "todo:e")
	t6 := f.enqueue("api", "sh", 7, "todo:f")

	tk, claim := f.claim("w1")
	if tk.ID != t1.ID && tk.ID != t2.ID {
		t.Fatalf("claim returned unexpected task %s (priority order)", tk.ID)
	}

	// Drive each task to its target state explicitly.
	if err := f.store.Complete(ctx, t1.ID, claimFor(t, f, t1.ID), jsonText(`{"ok":true}`)); err != nil {
		t.Fatalf("complete: %v", err)
	}

	claim2 := claimFor(t, f, t2.ID)
	if err := f.store.Fail(ctx, t2.ID, claim2, "boom", 0, nil); err != nil {
		t.Fatalf("fail t2: %v", err)
	}

	claim3 := claimFor(t, f, t3.ID)
	if err := f.store.Fail(ctx, t3.ID, claim3, "fatal", 0, nil); err != nil {
		t.Fatalf("fail t3: %v", err)
	}

	if err := f.store.Cancel(ctx, t4.ID, "no longer needed"); err != nil {
		t.Fatalf("cancel t4: %v", err)
	}

	claim5 := claimFor(t, f, t5.ID)
	if err := f.store.Requeue(ctx, t5.ID, claim5, "env not ready", 0, false); err != nil {
		t.Fatalf("requeue t5: %v", err)
	}

	if err := f.store.UpdatePendingPriority(ctx, t6.ID, 3, "ai", "rescored"); err != nil {
		t.Fatalf("reprioritize t6: %v", err)
	}

	f.resync()

	// Status counts: model vs store, exact.
	want := map[string]int{"completed": 1, "pending": 3, "dead": 1, "cancelled": 1}
	got, err := f.model.StatusCounts(ctx)
	if err != nil {
		t.Fatalf("status counts: %v", err)
	}

	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("status counts = %v, want %v", got, want)
	}

	storeCounts, err := f.store.StatusCounts(ctx)
	if err != nil {
		t.Fatalf("store status counts: %v", err)
	}

	for st, n := range storeCounts {
		if got[string(st)] != n {
			t.Errorf("status %s: model %d, store %d", st, got[string(st)], n)
		}
	}

	// Row state vs the store rows, field by field.
	rows, err := f.model.Tasks(ctx, readmodel.TaskFilter{})
	if err != nil {
		t.Fatalf("tasks: %v", err)
	}

	if len(rows) != 6 {
		t.Fatalf("rows = %d, want 6", len(rows))
	}

	byID := rowsByID(t, rows)

	for _, tk := range []task.Task{t1, t2, t3, t4, t5, t6} {
		st, err := f.store.Get(ctx, tk.ID)
		if err != nil {
			t.Fatalf("store get %s: %v", tk.ID, err)
		}

		row, ok := byID[tk.ID.String()]
		if !ok {
			t.Fatalf("row missing for %s", tk.ID)
		}

		if row.Status != string(st.Status) {
			t.Errorf("%s status = %s, store %s", tk.ID, row.Status, st.Status)
		}

		if row.Priority != st.Priority {
			t.Errorf("%s priority = %d, store %d", tk.ID, row.Priority, st.Priority)
		}

		if row.Attempts != st.Attempts {
			t.Errorf("%s attempts = %d, store %d", tk.ID, row.Attempts, st.Attempts)
		}

		if row.Project != st.Project || row.Type != st.Type || row.DedupKey != st.DedupKey {
			t.Errorf("%s identity = %s/%s/%s, store %s/%s/%s",
				tk.ID, row.Project, row.Type, row.DedupKey, st.Project, st.Type, st.DedupKey)
		}

		if row.CreatedAt != st.CreatedAt.UnixMilli() {
			t.Errorf("%s created_at = %d, store %d", tk.ID, row.CreatedAt, st.CreatedAt.UnixMilli())
		}
	}

	if got := byID[t2.ID.String()].LastError; got != "boom" {
		t.Errorf("t2 last_error = %q, want boom", got)
	}

	if got := byID[t6.ID.String()].Priority; got != 3 {
		t.Errorf("t6 priority = %d, want 3 (reprioritized)", got)
	}

	// Newest-first ordering: t6 was enqueued last.
	if rows[0].ID != t6.ID.String() {
		t.Errorf("first row = %s, want newest %s", rows[0].ID, t6.ID)
	}

	// Filters: status.
	pending := string(task.Pending)
	rows, err = f.model.Tasks(ctx, readmodel.TaskFilter{Status: readmodel.StringPtr(pending)})
	if err != nil {
		t.Fatalf("tasks(status=pending): %v", err)
	}

	storePending, err := f.store.List(ctx, queue.Filter{Status: &task.Pending})
	if err != nil {
		t.Fatalf("store list pending: %v", err)
	}

	if len(rows) != len(storePending) {
		t.Errorf("pending rows = %d, store %d", len(rows), len(storePending))
	}

	// Filters: project.
	rows, err = f.model.Tasks(ctx, readmodel.TaskFilter{Project: readmodel.StringPtr("api")})
	if err != nil {
		t.Fatalf("tasks(project=api): %v", err)
	}

	if len(rows) != 2 {
		t.Errorf("api rows = %d, want 2 (t3, t6)", len(rows))
	}

	// Filters: combined status + project.
	rows, err = f.model.Tasks(ctx, readmodel.TaskFilter{
		Status:  readmodel.StringPtr(string(task.Dead)),
		Project: readmodel.StringPtr("api"),
	})
	if err != nil {
		t.Fatalf("tasks(dead+api): %v", err)
	}

	if len(rows) != 1 || rows[0].ID != t3.ID.String() {
		t.Errorf("dead+api rows = %+v, want only t3", rows)
	}
}

// TestTailAppliesNewFacts pins the live pump: facts appended after the
// initial catch-up land in the collection on the next pass, and the
// watcher notifies with the folded row.
func TestTailAppliesNewFacts(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	tk := f.enqueue("web", "sh", 1, "todo:later")

	watchCh, watcher := readmodel.WatchTaskRows(f.model)
	defer watcher.Close()

	f.resync()

	rows, err := f.model.Tasks(ctx, readmodel.TaskFilter{})
	if err != nil {
		t.Fatalf("tasks: %v", err)
	}

	if len(rows) != 1 || rows[0].ID != tk.ID.String() {
		t.Fatalf("rows = %+v, want only %s", rows, tk.ID)
	}

	select {
	case row := <-watchCh:
		if row.ID != tk.ID.String() {
			t.Errorf("watcher row = %s, want %s", row.ID, tk.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for watcher notification")
	}
}

// jsonText is a tiny helper keeping jsontext imports out of the test faces.
func jsonText(s string) jsontextValue { return jsontextValue(s) }
