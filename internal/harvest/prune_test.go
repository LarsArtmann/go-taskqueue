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

// TestPruneStaleCancelsAbsentItems pins the absent-item policy: the docs
// convention DELETES completed items, so a pending harvested task whose item
// text is gone from the file entirely is a zombie (found by the 2026-09-09
// docs-health audit; policy decision: absent = withdrawn — the item was
// done-and-deleted or reworded, and a reword arms a new key and a new task).
// External work (no harvest dedup key in the payload) is never touched.
func TestPruneStaleCancelsAbsentItems(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "gone", "# H\n- [ ] done and deleted\n")
	writeRepo(t, dir, "kept", "# H\n- [ ] still real work\n")
	q := openQueue(t)
	h := New(q, Config{ProjectsDir: dir})
	ctx := context.Background()

	for range 2 {
		if _, err := h.Run(ctx); err != nil {
			t.Fatal(err)
		}
	}

	// External work in the same repo: an agent task with NO harvest dedup
	// key in its payload must be invisible to the absent rule.
	external, err := q.Enqueue(ctx, task.New{
		Project: "gone", Type: DefaultType,
		Payload: []byte(`{"prompt":"hand-enqueued, not harvested"}`),
	})
	if err != nil {
		t.Fatal(err)
	}

	// The item completes out-of-band and the docs convention deletes it.
	mustWrite(t, dir+"/gone/"+DefaultTodoFile, "# H\n")

	res, err := h.PruneStale(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Cancelled) != 1 || res.Cancelled[0].Why != PruneAbsent {
		t.Fatalf("Cancelled = %+v, want exactly the absent item's task", res.Cancelled)
	}

	cancelled, err := q.Get(ctx, res.Cancelled[0].TaskID)
	if err != nil {
		t.Fatal(err)
	}

	if cancelled.Status != task.Cancelled {
		t.Fatalf("absent-item task status = %s, want cancelled", cancelled.Status)
	}

	if reason := cancelReason(t, q, cancelled.ID); !strings.Contains(reason, "no longer present") {
		t.Errorf("cancel reason = %q, want the absent prefix", reason)
	}

	// The open item's task and the external task survive untouched.
	if still, err := q.Get(ctx, external.ID); err != nil || still.Status != task.Pending {
		t.Fatalf("external task = %+v err=%v, want pending (no dedup key, not sweepable)", still, err)
	}

	pending, err := q.List(ctx, pendingFilter("kept"))
	if err != nil {
		t.Fatal(err)
	}

	if len(pending) != 1 {
		t.Fatalf("kept repo pending tasks = %d, want 1", len(pending))
	}
}

// TestPruneStaleAbsentCatchupKey: status-sweeper catchup tasks carry
// "catchup:"-prefixed dedup keys over the same item key; the absent rule
// strips the prefix before matching, so a catchup task for a live item is
// untouched and one for a deleted item is withdrawn.
func TestPruneStaleAbsentCatchupKey(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "mixed", "# H\n- [ ] stays\n")
	q := openQueue(t)
	ctx := context.Background()

	live := "catchup:" + ItemKey("mixed", "stays")
	gone := "catchup:" + ItemKey("mixed", "deleted meanwhile")

	for _, key := range []string{live, gone} {
		payload, err := json.Marshal(map[string]string{"dedup": key})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := q.Enqueue(ctx, task.New{
			Project: "mixed", Type: DefaultType, Payload: payload, DedupKey: key,
		}); err != nil {
			t.Fatal(err)
		}
	}

	// The item the "gone" key was minted from never existed in this file —
	// same as having been deleted.
	res, err := New(q, Config{ProjectsDir: dir}).PruneStale(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Cancelled) != 1 || res.Cancelled[0].Why != PruneAbsent {
		t.Fatalf("Cancelled = %+v, want only the gone catchup task", res.Cancelled)
	}

	if got, err := q.Get(ctx, res.Cancelled[0].TaskID); err != nil || got.Status != task.Cancelled {
		t.Fatalf("gone catchup task = %+v err=%v, want cancelled", got, err)
	}

	pending, err := q.List(ctx, pendingFilter("mixed"))
	if err != nil {
		t.Fatal(err)
	}

	if len(pending) != 1 {
		t.Fatalf("pending catchup tasks = %d, want 1 (the live item's)", len(pending))
	}
}

// TestPruneStaleRewordedToBlockedIsWithdrawn pins the blocked-edit
// interaction: appending "— BLOCKED:" changes the item text (new key, skipped
// by the harvester), so the task minted from the OLD text is absent and gets
// withdrawn — the pool never executes work the owner deliberately blocked.
func TestPruneStaleRewordedToBlockedIsWithdrawn(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "blocked", "# H\n- [ ] fix the flaky test\n")
	q := openQueue(t)
	h := New(q, Config{ProjectsDir: dir})
	ctx := context.Background()

	if _, err := h.Run(ctx); err != nil {
		t.Fatal(err)
	}

	mustWrite(t, dir+"/blocked/"+DefaultTodoFile, "# H\n- [ ] fix the flaky test — BLOCKED: upstream flake, waiting on v1.2\n")

	res, err := h.PruneStale(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Cancelled) != 1 || res.Cancelled[0].Why != PruneAbsent {
		t.Fatalf("Cancelled = %+v, want the stale-text task withdrawn", res.Cancelled)
	}

	// The blocked item itself arms nothing: a subsequent harvest skips it.
	run, err := h.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(run.Enqueued) != 0 {
		t.Fatalf("blocked item enqueued %+v, want none", run.Enqueued)
	}
}

// cancelReason reads the last task.cancelled fact's reason from the journal.
func cancelReason(t *testing.T, q *queue.Queue, id task.ID) string {
	t.Helper()

	facts, err := q.FactsForTask(context.Background(), id.String(), 0)
	if err != nil {
		t.Fatal(err)
	}

	for i := range facts {
		if facts[i].Type == journal.Cancelled {
			var detail struct {
				Reason string `json:"reason"`
			}
			if err := json.Unmarshal(facts[i].Detail, &detail); err != nil {
				t.Fatalf("cancel fact detail not JSON with a reason: %v (%s)", err, facts[i].Detail)
			}
			return detail.Reason
		}
	}

	t.Fatal("no task.cancelled fact on " + id.String())
	return ""
}
