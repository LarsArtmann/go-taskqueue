package readmodel_test

import (
	"context"
	"encoding/json/jsontext"
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
		Project:  project,
		Type:     typ,
		Payload:  jsontext.Value(`"x"`),
		Priority: priority,
		DedupKey: dedup,
	})
	if err != nil {
		f.t.Fatalf("enqueue %s/%s: %v", project, typ, err)
	}

	time.Sleep(2 * time.Millisecond) // distinct created_at for the DESC sort

	return tk
}

func (f *fixture) claim() (task.Task, queue.Claim) {
	f.t.Helper()

	tk, claim, err := f.store.ClaimDue(context.Background(), "w1", time.Minute)
	if err != nil {
		f.t.Fatalf("claim: %v", err)
	}

	return tk, claim
}

func (f *fixture) resync() {
	f.t.Helper()

	if err := f.model.CatchUp(context.Background()); err != nil {
		f.t.Fatalf("catch up: %v", err)
	}
}

func (f *fixture) must(op string, err error) {
	f.t.Helper()

	if err != nil {
		f.t.Fatalf("%s: %v", op, err)
	}
}

// rowsByID indexes a model read by task id, failing on duplicates.
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

// TestParityWithStoreProjection runs a lifecycle battery through the real
// (v4-backed) sqlite store, folds the journal into the read model, and
// pins the collection projection against the store's own reads: status
// counts, per-row state, filters, and the newest-first order.
func TestParityWithStoreProjection(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	t1 := f.enqueue("web", "sh", 2, "todo:a")
	t2 := f.enqueue("web", "agent", 5, "todo:b")
	t3 := f.enqueue("api", "sh", 0, "todo:c")
	t4 := f.enqueue("api", "review", 4, "todo:d")
	t5 := f.enqueue("web", "sh", 1, "todo:e")
	t6 := f.enqueue("api", "sh", 7, "todo:f")

	// t6 is reprioritized while pending; t4 is cancelled while pending —
	// both before any claim so the claim order stays deterministic
	// (t2=5, t6=3, t1=2, t5=1, t3=0).
	f.must("reprioritize t6", f.store.UpdatePendingPriority(ctx, t6.ID, 3, "ai", "rescored"))
	f.must("cancel t4", f.store.Cancel(ctx, t4.ID, "no longer needed"))

	tk, claim := f.claim()
	if tk.ID != t2.ID {
		t.Fatalf("claim 1 = %s, want t2 (priority %d)", tk.ID, t2.Priority)
	}

	f.must("fail t2", f.store.Fail(ctx, t2.ID, claim, "boom", time.Hour, nil))

	tk, claim = f.claim()
	if tk.ID != t6.ID {
		t.Fatalf("claim 2 = %s, want t6", tk.ID)
	}

	f.must("requeue t6", f.store.Requeue(ctx, t6.ID, claim, "env not ready", time.Hour, false))

	tk, claim = f.claim()
	if tk.ID != t1.ID {
		t.Fatalf("claim 3 = %s, want t1", tk.ID)
	}

	f.must("complete t1", f.store.Complete(ctx, t1.ID, claim, jsontext.Value(`{"ok":true}`)))

	tk, claim = f.claim()
	if tk.ID != t5.ID {
		t.Fatalf("claim 4 = %s, want t5", tk.ID)
	}

	f.must("requeue t5", f.store.Requeue(ctx, t5.ID, claim, "env not ready", time.Hour, false))

	tk, claim = f.claim()
	if tk.ID != t3.ID {
		t.Fatalf("claim 5 = %s, want t3", tk.ID)
	}

	f.must("fail t3", f.store.FailPermanent(ctx, t3.ID, claim, "fatal", nil))

	f.resync()

	// Status counts: model vs store, exact.
	want := map[string]int{
		"pending":   3, // t2 (failed retry), t5 + t6 (requeued)
		"completed": 1, // t1
		"dead":      1, // t3
		"cancelled": 1, // t4
	}

	got, err := f.model.StatusCounts(ctx)
	f.must("status counts", err)

	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("status counts = %v, want %v", got, want)
	}

	storeCounts, err := f.store.StatusCounts(ctx)
	f.must("store status counts", err)

	for st, n := range storeCounts {
		if got[string(st)] != n {
			t.Errorf("status %s: model %d, store %d", st, got[string(st)], n)
		}
	}

	// Row state vs the store rows, field by field.
	rows, err := f.model.Tasks(ctx, readmodel.TaskFilter{})
	f.must("tasks", err)

	if len(rows) != 6 {
		t.Fatalf("rows = %d, want 6", len(rows))
	}

	byID := rowsByID(t, rows)

	for _, wantTask := range []task.Task{t1, t2, t3, t4, t5, t6} {
		st, err := f.store.Get(ctx, wantTask.ID)
		f.must("store get "+wantTask.ID.String(), err)

		row, ok := byID[wantTask.ID.String()]
		if !ok {
			t.Fatalf("row missing for %s", wantTask.ID)
		}

		if row.Status != string(st.Status) {
			t.Errorf("%s status = %s, store %s", wantTask.ID, row.Status, st.Status)
		}

		if row.Priority != st.Priority {
			t.Errorf("%s priority = %d, store %d", wantTask.ID, row.Priority, st.Priority)
		}

		if row.Attempts != st.Attempts {
			t.Errorf("%s attempts = %d, store %d", wantTask.ID, row.Attempts, st.Attempts)
		}

		if row.Project != st.Project || row.Type != st.Type || row.DedupKey != st.DedupKey {
			t.Errorf("%s identity = %s/%s/%s, store %s/%s/%s",
				wantTask.ID, row.Project, row.Type, row.DedupKey, st.Project, st.Type, st.DedupKey)
		}

		if row.CreatedAt != st.CreatedAt.UnixMilli() {
			t.Errorf("%s created_at = %d, store %d", wantTask.ID, row.CreatedAt, st.CreatedAt.UnixMilli())
		}

		if row.UpdatedAt < row.CreatedAt {
			t.Errorf("%s updated_at %d before created_at %d", wantTask.ID, row.UpdatedAt, row.CreatedAt)
		}
	}

	if got := byID[t2.ID.String()].LastError; got != "boom" {
		t.Errorf("t2 last_error = %q, want boom", got)
	}

	if got := byID[t3.ID.String()].LastError; got != "fatal" {
		t.Errorf("t3 last_error = %q, want fatal", got)
	}

	if got := byID[t6.ID.String()].Priority; got != 3 {
		t.Errorf("t6 priority = %d, want 3 (reprioritized)", got)
	}

	// Newest-first ordering: t6 was enqueued last.
	if rows[0].ID != t6.ID.String() {
		t.Errorf("first row = %s, want newest %s", rows[0].ID, t6.ID)
	}

	// Filters: status — ids must match the store's own list.
	pending := string(task.Pending)
	rows, err = f.model.Tasks(ctx, readmodel.TaskFilter{Status: readmodel.StringPtr(pending)})
	f.must("tasks(status=pending)", err)

	pendingStatus := task.Pending
	storePending, err := f.store.List(ctx, queue.Filter{Status: &pendingStatus})
	f.must("store list pending", err)

	if len(rows) != len(storePending) {
		t.Errorf("pending rows = %d, store %d", len(rows), len(storePending))
	}

	storeIDs := make(map[string]bool, len(storePending))
	for _, st := range storePending {
		storeIDs[st.ID.String()] = true
	}

	for _, r := range rows {
		if !storeIDs[r.ID] {
			t.Errorf("pending row %s not in store list", r.ID)
		}
	}

	// Filters: project.
	rows, err = f.model.Tasks(ctx, readmodel.TaskFilter{Project: readmodel.StringPtr("api")})
	f.must("tasks(project=api)", err)

	if len(rows) != 2 {
		t.Errorf("api rows = %d, want 2 (t3, t6)", len(rows))
	}

	// Filters: combined status + project.
	rows, err = f.model.Tasks(ctx, readmodel.TaskFilter{
		Status:  readmodel.StringPtr(string(task.Dead)),
		Project: readmodel.StringPtr("api"),
	})
	f.must("tasks(dead+api)", err)

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

	watchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	watchCh := f.model.Watch(watchCtx)

	f.resync()

	rows, err := f.model.Tasks(ctx, readmodel.TaskFilter{})
	f.must("tasks", err)

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

// TestReplayConverges pins the restart story: a fresh model over the same
// projection file replays the journal from zero and converges on the same
// projection (every fold is an upsert keyed by task id).
func TestReplayConverges(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	t1 := f.enqueue("web", "sh", 2, "todo:a")

	tk, claim := f.claim()
	if tk.ID != t1.ID {
		t.Fatalf("claim = %s, want t1", tk.ID)
	}

	f.must("complete t1", f.store.Complete(ctx, t1.ID, claim, jsontext.Value(`{}`)))

	f.resync()

	before, err := f.model.Tasks(ctx, readmodel.TaskFilter{})
	f.must("tasks before", err)

	replay, err := readmodel.Open(t.TempDir()+"/replay.db", f.store)
	f.must("open replay model", err)

	defer func() { _ = replay.Close() }()

	f.must("replay catch up", replay.CatchUp(ctx))

	after, err := replay.Tasks(ctx, readmodel.TaskFilter{})
	f.must("tasks after", err)

	if fmt.Sprint(after) != fmt.Sprint(before) {
		t.Errorf("replay rows diverged:\nbefore %v\nafter  %v", before, after)
	}
}
