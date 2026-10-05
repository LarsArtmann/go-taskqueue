package readmodel_test

import (
	"bufio"
	"context"
	"encoding/json/jsontext"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
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

	f.must(
		"requeue t6",
		f.store.Requeue(ctx, t6.ID, claim, "env not ready", time.Hour, false, queue.RequeueClassPreflight),
	)

	tk, claim = f.claim()
	if tk.ID != t1.ID {
		t.Fatalf("claim 3 = %s, want t1", tk.ID)
	}

	f.must("complete t1", f.store.Complete(ctx, t1.ID, claim, jsontext.Value(`{"ok":true}`)))

	tk, claim = f.claim()
	if tk.ID != t5.ID {
		t.Fatalf("claim 4 = %s, want t5", tk.ID)
	}

	f.must(
		"requeue t5",
		f.store.Requeue(ctx, t5.ID, claim, "env not ready", time.Hour, false, queue.RequeueClassPreflight),
	)

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

		// Class parks mirror the store's not_before within a few ms: the
		// store timestamps the task update and the fact with two clock
		// readings of the same transaction.
		if row.ParkedBy != "" {
			if delta := row.NotBefore - st.NotBefore.UnixMilli(); delta < -5 || delta > 5 {
				t.Errorf("%s not_before = %d, store %d", wantTask.ID, row.NotBefore, st.NotBefore.UnixMilli())
			}
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

	// Parked-by: the two preflight requeues carry their class; the failed
	// t2 is a retry, not a park.
	for _, tc := range []struct {
		id   task.ID
		want string
	}{{t5.ID, queue.RequeueClassPreflight}, {t6.ID, queue.RequeueClassPreflight}, {t2.ID, ""}} {
		if got := byID[tc.id.String()].ParkedBy; got != tc.want {
			t.Errorf("%s parked_by = %q, want %q", tc.id, got, tc.want)
		}
	}

	// Newest-first ordering: t6 was enqueued last.
	if rows[0].ID != t6.ID.String() {
		t.Errorf("first row = %s, want newest %s", rows[0].ID, t6.ID)
	}

	// Filters: status — ids must match the store's own list.
	pending := string(task.Pending)
	rows, err = f.model.Tasks(ctx, readmodel.TaskFilter{Status: new(pending)})
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

	// Filters: project — cancelled tasks stay in the ledger, so api holds
	// t3, t4 (cancelled) and t6.
	rows, err = f.model.Tasks(ctx, readmodel.TaskFilter{Project: new("api")})
	f.must("tasks(project=api)", err)

	if len(rows) != 3 {
		t.Errorf("api rows = %d, want 3 (t3, t4, t6)", len(rows))
	}

	// Filters: combined status + project.
	rows, err = f.model.Tasks(ctx, readmodel.TaskFilter{
		Status:  new(string(task.Dead)),
		Project: new("api"),
	})
	f.must("tasks(dead+api)", err)

	if len(rows) != 1 || rows[0].ID != t3.ID.String() {
		t.Errorf("dead+api rows = %+v, want only t3", rows)
	}
}

// TestStatsParityLifecycle pins the GROUP-BY-pushdown stats surface
// (Model.Stats/StatusCounts/ProjectCounts) against the store's own count
// reads over the full lifecycle — including the transitions with no
// statically knowable from-status (rescue re-enqueues from dead, dismiss
// cancels from dead), which are exactly why the counts derive from the
// folded rows instead of a stateless event-counter projection.
func TestStatsParityLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	f := newFixture(t)

	t1 := f.enqueue("web", "sh", 6, "todo:s1")    // → completed
	t2 := f.enqueue("web", "agent", 5, "todo:s2") // → pending (failed retry)
	t3 := f.enqueue("api", "sh", 4, "todo:s3")    // → dead
	t4 := f.enqueue("api", "sh", 3, "todo:s4")    // → cancelled while pending
	t5 := f.enqueue("web", "sh", 2, "todo:s5")    // → pending (requeued, parked)
	t6 := f.enqueue("api", "sh", 1, "todo:s6")    // → dead → rescued → pending
	t7 := f.enqueue("api", "sh", 0, "todo:s7")    // → dead → dismissed → cancelled

	f.must("cancel t4", f.store.Cancel(ctx, t4.ID, "not needed"))

	tk, claim := f.claim()
	if tk.ID != t1.ID {
		t.Fatalf("claim 1 = %s, want t1", tk.ID)
	}

	f.must("complete t1", f.store.Complete(ctx, t1.ID, claim, jsontext.Value(`{}`)))

	tk, claim = f.claim()
	if tk.ID != t2.ID {
		t.Fatalf("claim 2 = %s, want t2", tk.ID)
	}

	f.must("fail t2", f.store.Fail(ctx, t2.ID, claim, "boom", time.Hour, nil))

	tk, claim = f.claim()
	if tk.ID != t3.ID {
		t.Fatalf("claim 3 = %s, want t3", tk.ID)
	}

	f.must("dead t3", f.store.FailPermanent(ctx, t3.ID, claim, "fatal", nil))

	tk, claim = f.claim()
	if tk.ID != t5.ID {
		t.Fatalf("claim 4 = %s, want t5", tk.ID)
	}

	f.must(
		"requeue t5",
		f.store.Requeue(ctx, t5.ID, claim, "env not ready", time.Hour, false, queue.RequeueClassPreflight),
	)

	tk, claim = f.claim()
	if tk.ID != t6.ID {
		t.Fatalf("claim 5 = %s, want t6", tk.ID)
	}

	f.must("dead t6", f.store.FailPermanent(ctx, t6.ID, claim, "fatal", nil))

	tk, claim = f.claim()
	if tk.ID != t7.ID {
		t.Fatalf("claim 6 = %s, want t7", tk.ID)
	}

	f.must("dead t7", f.store.FailPermanent(ctx, t7.ID, claim, "fatal", nil))

	// The from-status-ambiguous transitions: rescue re-enqueues the dead
	// t6 (fresh attempt budget), dismiss cancels the dead t7.
	f.must("rescue t6", f.store.RescueDead(ctx, t6.ID, 3))
	f.must("dismiss t7", f.store.DismissDead(ctx, t7.ID, "superseded", "test"))

	f.resync()

	wantStatus := map[string]int{
		"pending":   3, // t2 (failed retry), t5 (requeued), t6 (rescued)
		"completed": 1, // t1
		"dead":      1, // t3
		"cancelled": 2, // t4, t7 (dismissed)
	}

	wantProject := map[string]map[string]int{
		"web": {"pending": 2, "completed": 1},            // t2, t5 / t1
		"api": {"pending": 1, "dead": 1, "cancelled": 2}, // t6 / t3 / t4, t7
	}

	byStatus, byProject, err := f.model.Stats(ctx, readmodel.TaskFilter{})
	f.must("stats", err)

	if fmt.Sprint(byStatus) != fmt.Sprint(wantStatus) {
		t.Errorf("stats byStatus = %v, want %v", byStatus, wantStatus)
	}

	if fmt.Sprint(byProject) != fmt.Sprint(wantProject) {
		t.Errorf("stats byProject = %v, want %v", byProject, wantProject)
	}

	// Store parity: the projection counters equal the store's own GROUP
	// BY surfaces exactly.
	storeStatus, err := f.store.StatusCounts(ctx)
	f.must("store status counts", err)

	for st, n := range storeStatus {
		if byStatus[string(st)] != n {
			t.Errorf("status %s: model %d, store %d", st, byStatus[string(st)], n)
		}
	}

	storeProject, err := f.store.ProjectCounts(ctx)
	f.must("store project counts", err)

	for p, m := range storeProject {
		for st, n := range m {
			if byProject[p][string(st)] != n {
				t.Errorf("project %s status %s: model %d, store %d",
					p, st, byProject[p][string(st)], n)
			}
		}
	}

	// Filtered stats: a project filter narrows both matrices.
	byStatus, byProject, err = f.model.Stats(ctx, readmodel.TaskFilter{Project: new("api")})
	f.must("stats(api)", err)

	wantAPI := map[string]int{"pending": 1, "dead": 1, "cancelled": 2}
	if fmt.Sprint(byStatus) != fmt.Sprint(wantAPI) {
		t.Errorf("stats(api) byStatus = %v, want %v", byStatus, wantAPI)
	}

	if len(byProject) != 1 || fmt.Sprint(byProject["api"]) != fmt.Sprint(wantAPI) {
		t.Errorf("stats(api) byProject = %v, want only api %v", byProject, wantAPI)
	}

	// A status filter leaves only that status's counts.
	byStatus, byProject, err = f.model.Stats(
		ctx,
		readmodel.TaskFilter{Status: new(string(task.Dead))},
	)
	f.must("stats(dead)", err)

	if fmt.Sprint(byStatus) != fmt.Sprint(map[string]int{"dead": 1}) {
		t.Errorf("stats(dead) byStatus = %v, want dead:1", byStatus)
	}

	if len(byProject) != 1 || byProject["api"]["dead"] != 1 {
		t.Errorf("stats(dead) byProject = %v, want api dead:1", byProject)
	}

	// The single-matrix APIs stay consistent with Stats.
	sc, err := f.model.StatusCounts(ctx)
	f.must("status counts", err)

	if fmt.Sprint(sc) != fmt.Sprint(wantStatus) {
		t.Errorf("status counts = %v, want %v", sc, wantStatus)
	}

	pc, err := f.model.ProjectCounts(ctx)
	f.must("project counts", err)

	if fmt.Sprint(pc) != fmt.Sprint(wantProject) {
		t.Errorf("project counts = %v, want %v", pc, wantProject)
	}
}

// countingStore counts the facts the model pulls out of Facts — the
// restart-skip probe: a cursor-resumed model must fold nothing on an
// unchanged journal.
type countingStore struct {
	queue.Store

	factsSeen int64
}

func (c *countingStore) Facts(
	ctx context.Context,
	after int64,
	limit int,
) ([]journal.Fact, error) {
	facts, err := c.Store.Facts(ctx, after, limit)
	c.factsSeen += int64(len(facts))

	return facts, err
}

// TestDurableCursorSkipsReplay pins the WithDurableCursor contract: the
// first model checkpoints its cursor into the watermarks table after
// every applied batch; a reopened model resumes from the checkpoint
// (zero facts folded on an unchanged journal), tail folds only the new
// facts, and an empty projection under a stale checkpoint replays from
// zero — the documented delete-the-file escape hatch.
func TestDurableCursorSkipsReplay(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store, err := sqlite.Open(t.TempDir() + "/queue.db")
	if err != nil {
		t.Fatalf("open queue store: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	f := &fixture{t: t, store: store}

	t1 := f.enqueue("web", "sh", 2, "todo:c1")
	f.enqueue("web", "sh", 1, "todo:c2")
	f.enqueue("api", "sh", 0, "todo:c3")

	tk, claim := f.claim()
	if tk.ID != t1.ID {
		t.Fatalf("claim = %s, want t1", tk.ID)
	}

	f.must("complete t1", f.store.Complete(ctx, t1.ID, claim, jsontext.Value(`{}`)))

	proj := t.TempDir() + "/projection.db"

	first, err := readmodel.Open(proj, store, readmodel.WithDurableCursor())
	if err != nil {
		t.Fatalf("open first model: %v", err)
	}

	if err := first.CatchUp(ctx); err != nil {
		t.Fatalf("first catch up: %v", err)
	}

	head, err := store.HeadSeq(ctx)
	if err != nil {
		t.Fatalf("head seq: %v", err)
	}

	if got := first.JournalCursor(); got != head {
		t.Fatalf("first cursor = %d, want head %d", got, head)
	}

	wm, exists, err := store.Watermark(ctx, readmodel.CursorConsumer)
	if err != nil || !exists || wm != head {
		t.Fatalf("watermark = %d exists=%v err=%v, want %d", wm, exists, err, head)
	}

	_ = first.Close()

	// Reopen over the SAME projection: nothing refolds on the unchanged
	// journal, and the cursor resumes at the checkpoint.
	counted := &countingStore{Store: store}

	second, err := readmodel.Open(proj, counted, readmodel.WithDurableCursor())
	if err != nil {
		t.Fatalf("open second model: %v", err)
	}

	defer func() { _ = second.Close() }()

	if got := second.JournalCursor(); got != head {
		t.Fatalf("resumed cursor = %d, want checkpoint %d", got, head)
	}

	if err := second.CatchUp(ctx); err != nil {
		t.Fatalf("second catch up: %v", err)
	}

	if counted.factsSeen != 0 {
		t.Fatalf("resume folded %d facts, want 0", counted.factsSeen)
	}

	rows, err := second.Tasks(ctx, readmodel.TaskFilter{})
	f.must("tasks after resume", err)

	if len(rows) != 3 {
		t.Fatalf("rows after resume = %d, want 3", len(rows))
	}

	// Tail: one new fact folds exactly one fact's worth of work.
	f.enqueue("api", "sh", 0, "todo:c4")

	if err := second.CatchUp(ctx); err != nil {
		t.Fatalf("tail catch up: %v", err)
	}

	if counted.factsSeen != 1 {
		t.Fatalf("tail folded %d facts, want 1", counted.factsSeen)
	}

	rows, err = second.Tasks(ctx, readmodel.TaskFilter{})
	f.must("tasks after tail", err)

	if len(rows) != 4 {
		t.Fatalf("rows after tail = %d, want 4", len(rows))
	}

	_ = second.Close()

	// Empty projection under a stale checkpoint (the deleted-file escape
	// hatch): a fresh file replays from zero and converges.
	fresh, err := readmodel.Open(t.TempDir()+"/fresh.db", counted, readmodel.WithDurableCursor())
	if err != nil {
		t.Fatalf("open fresh model: %v", err)
	}

	defer func() { _ = fresh.Close() }()

	if err := fresh.CatchUp(ctx); err != nil {
		t.Fatalf("fresh catch up: %v", err)
	}

	if counted.factsSeen == 1 {
		t.Fatal("fresh model folded nothing — stale checkpoint wedged the empty projection")
	}

	rows, err = fresh.Tasks(ctx, readmodel.TaskFilter{})
	f.must("tasks after replay", err)

	if len(rows) != 4 {
		t.Fatalf("rows after replay = %d, want 4", len(rows))
	}

	wm, _, err = store.Watermark(ctx, readmodel.CursorConsumer)
	if err != nil || wm != head+1 {
		t.Fatalf("watermark after replay = %d err=%v, want %d", wm, err, head+1)
	}
}

// TestParkedByProjection pins the class-park surface: the requeue
// evidence's class + window land on the row, the next claim clears them,
// and a later park replaces the class — the exactness `tq top`,
// `tq tasks --parked-class` and the webui budget lamp are specified to
// read instead of re-deriving it from the fact tail.
func TestParkedByProjection(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	f := newFixture(t)

	tk := f.enqueue("web", "agent", 5, "todo:park")
	_, claim := f.claim()

	// Parked with a tiny window: the claim gate honors not_before, so the
	// test sleeps past it before re-claiming.
	f.must(
		"budget requeue",
		f.store.Requeue(ctx, tk.ID, claim, "daily cap spent", time.Millisecond, false, queue.RequeueClassBudget),
	)
	f.resync()

	pending := string(task.Pending)
	rows, err := f.model.Tasks(ctx, readmodel.TaskFilter{Status: new(pending)})
	f.must("tasks", err)

	if len(rows) != 1 {
		t.Fatalf("pending rows = %d, want 1", len(rows))
	}

	if rows[0].ParkedBy != queue.RequeueClassBudget {
		t.Errorf("parked_by = %q, want %q", rows[0].ParkedBy, queue.RequeueClassBudget)
	}

	if rows[0].NotBefore == 0 {
		t.Error("not_before = 0, want the requeue window")
	}

	time.Sleep(5 * time.Millisecond)

	_, claim = f.claim()
	f.must(
		"rate-limit requeue",
		f.store.Requeue(ctx, tk.ID, claim, "provider 429", time.Millisecond, false, queue.RequeueClassRateLimit),
	)
	f.resync()

	rows, err = f.model.Tasks(ctx, readmodel.TaskFilter{Status: new(pending)})
	f.must("tasks 2", err)

	if len(rows) != 1 || rows[0].ParkedBy != queue.RequeueClassRateLimit {
		t.Fatalf("rows = %+v, parked_by want %q", rows, queue.RequeueClassRateLimit)
	}

	time.Sleep(5 * time.Millisecond)

	tk2, claim := f.claim()
	if tk2.ID != tk.ID {
		t.Fatalf("claim = %s, want %s", tk2.ID, tk.ID)
	}

	f.must("complete", f.store.Complete(ctx, tk.ID, claim, jsontext.Value(`"x"`)))
	f.resync()

	completed := string(task.Completed)
	rows, err = f.model.Tasks(ctx, readmodel.TaskFilter{Status: new(completed)})
	f.must("tasks 3", err)

	if len(rows) != 1 || rows[0].ParkedBy != "" || rows[0].NotBefore != 0 {
		t.Fatalf("completed rows = %+v, want the park cleared", rows)
	}
}

// TestTailAppliesNewFacts pins the live pump: facts appended after the
// initial catch-up land in the collection on the next pass, and the
// watcher notifies with the folded row.
func TestTailAppliesNewFacts(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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

// TestEventsHandlerStreamsRows pins the ServeSSE surface: the handler
// streams each folded row as one JSON event carrying the projection write
// sequence as the SSE id (the Last-Event-ID reconnection watermark).
func TestEventsHandlerStreamsRows(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	f := newFixture(t)

	f.enqueue("web", "sh", 1, "todo:sse-1")
	f.resync()

	server := httptest.NewServer(f.model.EventsHandler())
	t.Cleanup(server.Close)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	f.must("build request", err)

	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Do(req)
	f.must("connect events stream", err)

	defer func() { _ = resp.Body.Close() }()

	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("content type = %q, want text/event-stream", ct)
	}

	tk := f.enqueue("web", "sh", 2, "todo:sse-2")
	f.resync()

	var (
		sawID      bool
		sawRowData bool
	)

	scanner := bufio.NewScanner(resp.Body)

	deadline := time.Now().Add(4 * time.Second)

	for scanner.Scan() {
		if time.Now().After(deadline) {
			t.Fatal("timeout waiting for the streamed row event")
		}

		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "id:"):
			sawID = true
		case strings.HasPrefix(line, "data:") && strings.Contains(line, tk.ID.String()):
			sawRowData = true
		}

		if sawID && sawRowData {
			return
		}
	}

	f.must("scan stream", scanner.Err())
	t.Fatalf("stream ended without the row event (id seen: %v, data seen: %v)", sawID, sawRowData)
}
