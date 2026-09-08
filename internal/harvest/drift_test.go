package harvest

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func taskFilter(repoName string) queue.Filter {
	t := DefaultType

	return queue.Filter{Project: &repoName, Type: &t}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()

	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

// completeHarvestedTask claims and completes the task a harvest run created
// for itemText, leaving the checkbox open — the stale-open setup.
func completeHarvestedTask(t *testing.T, h *Harvester, repoName, itemText string) task.Task {
	t.Helper()

	ctx := context.Background()

	tasks, err := h.q.List(ctx, taskFilter(repoName))
	if err != nil {
		t.Fatal(err)
	}

	key := ItemKey(repoName, itemText)

	for _, task := range tasks {
		var p struct {
			Dedup string `json:"dedup"`
		}
		if json.Unmarshal(task.Payload, &p) == nil && p.Dedup == key {
			if _, err := h.q.ClaimDue(ctx, "tester", time.Minute); err != nil {
				t.Fatal(err)
			}

			if err := h.q.Complete(ctx, task.ID, "tester", nil); err != nil {
				t.Fatal(err)
			}

			return task
		}
	}

	t.Fatalf("no task harvested for item %q", itemText)

	return task.Task{}
}

func TestAuditDetectsDrift(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "drifty", "# H\n- [ ] finished but unticked\n- [ ] still open work\n- [x] hand-done early\n")
	q := openQueue(t)
	h := New(q, Config{ProjectsDir: dir})
	ctx := context.Background()

	if _, err := h.Run(ctx); err != nil {
		t.Fatal(err)
	}
	// The agent finished the first item; the checkbox stayed open.
	done := completeHarvestedTask(t, h, "drifty", "finished but unticked")

	res, err := h.Audit(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(res.StaleOpen) != 1 {
		t.Fatalf("StaleOpen = %d entries, want 1: %+v", len(res.StaleOpen), res.StaleOpen)
	}

	d := res.StaleOpen[0]
	if d.TaskID != done.ID || d.Item.Text != "finished but unticked" || d.Kind != DriftStaleOpen {
		t.Errorf("wrong drift: %+v", d)
	}

	if len(res.StaleDone) != 0 {
		t.Errorf(
			"StaleDone = %d, want 0 (ticked item has no task at all — not drift): %+v",
			len(res.StaleDone),
			res.StaleDone,
		)
	}

	if len(res.Enqueued) != 1 {
		t.Fatalf("catch-ups enqueued = %d, want 1", len(res.Enqueued))
	}
}

func TestAuditStaleDoneReportedNotRepaired(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "reversee", "# H\n- [ ] will be ticked by hand\n")
	q := openQueue(t)
	h := New(q, Config{ProjectsDir: dir})
	ctx := context.Background()

	if _, err := h.Run(ctx); err != nil {
		t.Fatal(err)
	}
	// Human ticks the box while the task is still pending.
	todo := filepath.Join(dir, "reversee", DefaultTodoFile)
	mustWrite(t, todo, "# H\n- [x] will be ticked by hand\n")

	res, err := h.Audit(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(res.StaleDone) != 1 || res.StaleDone[0].TaskStatus != task.Pending {
		t.Fatalf("StaleDone = %+v, want one pending drift", res.StaleDone)
	}

	if len(res.Enqueued) != 0 {
		t.Errorf("auditor enqueued %d tasks for stale-done, want 0 (report-only)", len(res.Enqueued))
	}
}

func TestAuditEnqueuesCatchupOnce(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "loop", "# H\n- [ ] done but unticked forever\n")
	q := openQueue(t)
	h := New(q, Config{ProjectsDir: dir})
	ctx := context.Background()

	if _, err := h.Run(ctx); err != nil {
		t.Fatal(err)
	}

	completeHarvestedTask(t, h, "loop", "done but unticked forever")

	first, err := h.Audit(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(first.Enqueued) != 1 {
		t.Fatalf("first audit enqueued %d catch-ups, want 1", len(first.Enqueued))
	}

	// Even if the catch-up agent ALSO fails to tick the box, re-auditing
	// must not pile up duplicate catch-ups (dedup key catchup:<key>).
	second, err := h.Audit(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(second.Enqueued) != 0 {
		t.Fatalf("second audit enqueued %d catch-ups, want 0 (enqueue-once)", len(second.Enqueued))
	}
	// The drift itself is still visible until a human or agent closes it.
	if len(second.StaleOpen) != 1 {
		t.Fatalf("second audit lost track of the drift: %+v", second.StaleOpen)
	}
}

func TestAuditDryRunEnqueuesNothing(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "dryr", "# H\n- [ ] finished, dry run\n")
	q := openQueue(t)
	h := New(q, Config{ProjectsDir: dir, DryRun: true})
	ctx := context.Background()

	// Seed the queue manually: dry-run harvest creates nothing.
	if _, err := q.Enqueue(ctx, task.New{
		Project: "dryr",
		Type:    DefaultType,
		Payload: mustJSON(
			t,
			map[string]string{"repo": "dryr", "prompt": "x", "dedup": ItemKey("dryr", "finished, dry run")},
		),
		DedupKey: ItemKey("dryr", "finished, dry run"),
	}); err != nil {
		t.Fatal(err)
	}

	tasks, err := q.List(ctx, taskFilter("dryr"))
	if err != nil || len(tasks) != 1 {
		t.Fatalf("seed failed: %v %d", err, len(tasks))
	}

	if _, err := q.ClaimDue(ctx, "tester", time.Minute); err != nil {
		t.Fatal(err)
	}

	if err := q.Complete(ctx, tasks[0].ID, "tester", nil); err != nil {
		t.Fatal(err)
	}

	res, err := h.Audit(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(res.StaleOpen) != 1 {
		t.Fatalf("dry-run audit missed the drift: %+v", res.StaleOpen)
	}

	if len(res.Enqueued) != 0 {
		t.Errorf("dry-run enqueued %d catch-ups, want 0", len(res.Enqueued))
	}
}

func TestParseRepoAllReturnsDoneItems(t *testing.T) {
	dir := t.TempDir()
	repo := writeRepo(t, dir, "mixed", "# H\n- [ ] open one\n- [x] done one\n- [X] DONE upper\n")

	all, err := ParseRepoAll(repo, DefaultTodoFile)
	if err != nil {
		t.Fatal(err)
	}

	if len(all) != 3 {
		t.Fatalf("got %d items, want 3: %+v", len(all), all)
	}

	if all[0].Done || !all[1].Done || !all[2].Done {
		t.Errorf("Done flags wrong: %+v", all)
	}
	// ParseRepo still returns only the open ones (harvest semantics pinned).
	open, err := ParseRepo(repo, DefaultTodoFile)
	if err != nil {
		t.Fatal(err)
	}

	if len(open) != 1 || open[0].Text != "open one" {
		t.Errorf("ParseRepo open items = %+v, want just 'open one'", open)
	}
}

func TestAuditReportsUnscannableRepo(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "good", "# H\n- [ ] open work\n")
	// A repo whose TODO file is a directory: ParseRepoAll fails on it.
	badRepo := filepath.Join(dir, "badrepo")

	if err := os.MkdirAll(filepath.Join(badRepo, DefaultTodoFile), 0o755); err != nil {
		t.Fatal(err)
	}

	h := New(openQueue(t), Config{Repos: []string{filepath.Join(dir, "good"), badRepo}})

	res, err := h.Audit(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(res.ScanFailures) != 1 {
		t.Fatalf("ScanFailures = %d entries, want 1: %+v", len(res.ScanFailures), res.ScanFailures)
	}

	f := res.ScanFailures[0]
	if f.Repo != badRepo || f.Reason == "" {
		t.Errorf("wrong scan failure: %+v", f)
	}

	if res.Repos != 2 {
		t.Errorf("Repos = %d, want 2 (bad repo still counted)", res.Repos)
	}
}
