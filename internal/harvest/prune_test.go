package harvest

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func pendingFilter(repoName string) queue.Filter {
	st := task.Pending

	return queue.Filter{Project: &repoName, Status: &st}
}

// TestPruneStaleCancelsPendingZombies pins the pool-relaunch contract: a
// pending task whose TODO_LIST item is now [x] is cancelled with a reason,
// while an OPEN item's pending task survives (21:40 report §d1/§e1). Two
// repos because the per-repo pacing allows at most one pending task per repo.
func TestPruneStaleCancelsPendingZombies(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "zombie", "# H\n- [ ] done by hand\n")
	writeRepo(t, dir, "alive", "# H\n- [ ] still real work\n")
	q := openQueue(t)
	h := New(q, Config{ProjectsDir: dir})
	ctx := context.Background()

	for range 2 {
		if _, err := h.Run(ctx); err != nil {
			t.Fatal(err)
		}
	}

	// The human does the zombie repo's item by hand and ticks it.
	mustWrite(t, dir+"/zombie/"+DefaultTodoFile, "# H\n- [x] done by hand\n")

	res, err := h.PruneStale(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Cancelled) != 1 || res.Cancelled[0].Item.Text != "done by hand" {
		t.Fatalf("Cancelled = %+v, want exactly the ticked item's task", res.Cancelled)
	}

	if len(res.Running) != 0 || len(res.Dead) != 0 {
		t.Errorf("Running/Dead = %+v/%+v, want none", res.Running, res.Dead)
	}

	cancelled, err := q.Get(ctx, res.Cancelled[0].TaskID)
	if err != nil {
		t.Fatal(err)
	}

	if cancelled.Status != task.Cancelled {
		t.Fatalf("pruned task status = %s, want cancelled", cancelled.Status)
	}

	trail, err := q.FactsForTask(ctx, cancelled.ID.String(), 0)
	if err != nil {
		t.Fatal(err)
	}

	var cancelFact *journal.Fact

	for i := range trail {
		if trail[i].Type == journal.Cancelled {
			cancelFact = &trail[i]
		}
	}

	if cancelFact == nil {
		t.Fatal("no task.cancelled fact on the pruned task")
	}

	var detail struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(cancelFact.Detail, &detail); err != nil {
		t.Fatalf("cancel fact detail not JSON with a reason: %v (%s)", err, cancelFact.Detail)
	}

	if !strings.Contains(detail.Reason, "[x]") || !strings.Contains(detail.Reason, "done by hand") {
		t.Errorf("cancel reason = %q, want the prune prefix + item text", detail.Reason)
	}

	// The open item's task must survive untouched.
	pending, err := q.List(ctx, pendingFilter("alive"))
	if err != nil {
		t.Fatal(err)
	}

	if len(pending) != 1 {
		t.Fatalf("pending tasks after prune = %d, want 1 (the open item)", len(pending))
	}
}

// TestPruneStaleReportsRunningAndDead: a cooperative stop mid-execution and a
// rescue out of the DLQ are operator decisions — the sweep only reports.
func TestPruneStaleReportsRunningAndDead(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "mixed", "# H\n- [ ] runs now\n- [ ] died trying\n")
	q := openQueue(t)
	h := New(q, Config{ProjectsDir: dir, MaxPerTick: 2})
	ctx := context.Background()

	if _, err := h.Run(ctx); err != nil {
		t.Fatal(err)
	}

	mustWrite(t, dir+"/mixed/"+DefaultTodoFile, "# H\n- [x] runs now\n- [x] died trying\n")

	claimed, err := q.ClaimDue(ctx, "worker-1", 0)
	if err != nil {
		t.Fatal(err)
	}

	res, err := h.PruneStale(ctx)
	if err != nil {
		t.Fatal(err)
	}

	running, err := q.Get(ctx, claimed.ID)
	if err != nil {
		t.Fatal(err)
	}

	if running.Status != task.Running {
		t.Fatalf("running task status = %s, want running (prune must not touch it)", running.Status)
	}

	// The second task is dead (budget 1 attempt burned) — reported, not rescued.
	if err := q.Fail(ctx, claimed.ID, "worker-1", "boom", 0, nil); err != nil {
		t.Fatal(err)
	}

	dead, err := q.ClaimDue(ctx, "worker-1", 0)
	if err != nil {
		t.Fatal(err)
	}

	if err := q.FailPermanent(ctx, dead.ID, "worker-1", "broken", nil); err != nil {
		t.Fatal(err)
	}

	res, err = h.PruneStale(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Running) != 0 || len(res.Dead) != 1 || res.Dead[0].TaskID != dead.ID {
		t.Fatalf("second pass Running/Dead = %+v/%+v, want 0/1 (%s)", res.Running, res.Dead, dead.ID)
	}

	after, err := q.Get(ctx, dead.ID)
	if err != nil {
		t.Fatal(err)
	}

	if after.Status != task.Dead {
		t.Fatalf("dead task status = %s, want dead (prune must not touch it)", after.Status)
	}
}

// TestPruneStaleDryRunChangesNothing pins the report-only mode.
func TestPruneStaleDryRunChangesNothing(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "peek", "# H\n- [ ] done by hand\n")
	q := openQueue(t)
	ctx := context.Background()

	// A REAL pending task first (a dry-run harvest enqueues nothing).
	if _, err := New(q, Config{ProjectsDir: dir}).Run(ctx); err != nil {
		t.Fatal(err)
	}

	mustWrite(t, dir+"/peek/"+DefaultTodoFile, "# H\n- [x] done by hand\n")

	res, err := New(q, Config{ProjectsDir: dir, DryRun: true}).PruneStale(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Cancelled) != 1 {
		t.Fatalf("dry-run Cancelled = %+v, want the would-cancel report", res.Cancelled)
	}

	still, err := q.Get(ctx, res.Cancelled[0].TaskID)
	if err != nil {
		t.Fatal(err)
	}

	if still.Status != task.Pending {
		t.Fatalf("dry-run task status = %s, want pending", still.Status)
	}
}
