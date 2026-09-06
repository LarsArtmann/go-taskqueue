package harvest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func openQueue(t *testing.T) *queue.Queue {
	t.Helper()
	s, err := queue.OpenSQLite(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return queue.New(s)
}

func writeRepo(t *testing.T, projectsDir, name, todo string) string {
	t.Helper()
	repo := filepath.Join(projectsDir, name)
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, DefaultTodoFile), []byte(todo), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestParseRepo(t *testing.T) {
	repo := t.TempDir()
	todo := `# Project X

## Bugs

- [ ] Fix the flaky worker test
- [x] Done thing, must be ignored
- [ ]  Trim   and  collapse   whitespace 

### Docs

* [ ] Write ADR for the pool

Docs contain examples that must never be harvested:

` + "```" + `
- [ ] fake item in a code fence
` + "```" + `

- [ ] Real item after the fence
`
	if err := os.WriteFile(filepath.Join(repo, DefaultTodoFile), []byte(todo), 0o644); err != nil {
		t.Fatal(err)
	}

	items, err := ParseRepo(repo, DefaultTodoFile)
	if err != nil {
		t.Fatalf("ParseRepo: %v", err)
	}
	if len(items) != 4 {
		t.Fatalf("got %d items, want 4: %+v", len(items), items)
	}

	want := []struct{ heading, text string }{
		{"Bugs", "Fix the flaky worker test"},
		{"Bugs", "Trim   and  collapse   whitespace"}, // verbatim text; only the key collapses
		{"Docs", "Write ADR for the pool"},
		{"Docs", "Real item after the fence"},
	}
	for i, w := range want {
		if items[i].Heading != w.heading || items[i].Text != w.text {
			t.Fatalf("item[%d] = (%q, %q), want (%q, %q)", i, items[i].Heading, items[i].Text, w.heading, w.text)
		}
		if items[i].RepoName == "" || items[i].Key == "" {
			t.Fatalf("item[%d] missing RepoName/Key: %+v", i, items[i])
		}
	}
	if items[0].Key == items[1].Key {
		t.Fatal("distinct items must have distinct keys")
	}
	if items[1].Key != ItemKey(items[1].RepoName, "Trim   and\tcollapse   whitespace") {
		t.Fatal("key must collapse whitespace")
	}
}

func TestItemKeyStableAcrossRepoMoves(t *testing.T) {
	a := ItemKey("myrepo", "Do the thing")
	b := ItemKey("myrepo", "Do the thing")
	if a != b {
		t.Fatal("same repo+text must give same key")
	}
	if a == ItemKey("otherrepo", "Do the thing") {
		t.Fatal("different repos must give different keys")
	}
	if a == ItemKey("myrepo", "Do the thing, edited") {
		t.Fatal("edited text must change the key")
	}
}

func TestDiscoverRepos(t *testing.T) {
	dir := t.TempDir()
	withTodo := writeRepo(t, dir, "has-todo", "- [ ] x\n")
	if err := os.MkdirAll(filepath.Join(dir, "no-todo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, plainFileName), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	repos, err := DiscoverRepos(dir, DefaultTodoFile)
	if err != nil {
		t.Fatalf("DiscoverRepos: %v", err)
	}
	if len(repos) != 1 || repos[0] != withTodo {
		t.Fatalf("repos = %v, want [%s]", repos, withTodo)
	}
}

const plainFileName = "plain-file-ignored"

func TestRunEnqueuesOneItemPerRepoPerTick(t *testing.T) {
	q := openQueue(t)
	dir := t.TempDir()
	writeRepo(t, dir, "alpha", "## Work\n\n- [ ] first\n- [ ] second\n")

	h := New(q, Config{ProjectsDir: dir})
	res, err := h.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Enqueued) != 1 || res.Enqueued[0].Item.Text != "first" || !res.Enqueued[0].Fresh {
		t.Fatalf("first run enqueued = %+v, want exactly 'first' fresh", res.Enqueued)
	}
	if !hasSkip(res, "paced") {
		t.Fatalf("first run must pace the second item, skips = %+v", res.Skipped)
	}

	// Second tick: repo busy with the pending task, nothing new.
	res, err = h.Run(context.Background())
	if err != nil {
		t.Fatalf("Run 2: %v", err)
	}
	if len(res.Enqueued) != 0 {
		t.Fatalf("second run enqueued %+v, want none (repo busy)", res.Enqueued)
	}
	if !hasSkip(res, "tracked: pending") || !hasSkip(res, "repo busy") {
		t.Fatalf("second run skips = %+v", res.Skipped)
	}
}

func TestRunDedupAcrossTicksAndStatuses(t *testing.T) {
	q := openQueue(t)
	ctx := context.Background()
	dir := t.TempDir()
	repo := writeRepo(t, dir, "beta", "## Work\n\n- [ ] only item\n")

	h := New(q, Config{ProjectsDir: dir})

	// Tick 1: enqueue, then simulate the task completing.
	if res, _ := h.Run(ctx); len(res.Enqueued) != 1 {
		t.Fatalf("tick 1: %+v", res.Enqueued)
	}
	tasks, err := q.List(ctx, queue.Filter{Project: strPtr("beta")})
	if err != nil || len(tasks) != 1 {
		t.Fatalf("list: %v %d", err, len(tasks))
	}
	id := tasks[0].ID
	if err := fakeRunToCompletion(ctx, q, id); err != nil {
		t.Fatal(err)
	}

	// Tick 2: item still unchecked but the task is completed — known key,
	// must NOT re-enqueue (loop prevention; docs catch-up is the agent's
	// contract, and editing the item re-arms it).
	res, err := h.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Enqueued) != 0 {
		t.Fatalf("completed item re-enqueued: %+v", res.Enqueued)
	}
	if !hasSkip(res, "tracked: completed") {
		t.Fatalf("want 'tracked: completed' skip, got %+v", res.Skipped)
	}

	// Tick 3: human edits the item text — new key — new task.
	newTodo := "## Work\n\n- [ ] only item, now with more detail\n"
	if err := os.WriteFile(filepath.Join(repo, DefaultTodoFile), []byte(newTodo), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err = h.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Enqueued) != 1 || res.Enqueued[0].Item.Text != "only item, now with more detail" {
		t.Fatalf("edited item not re-armed: %+v", res.Enqueued)
	}
}

func TestRunRespectsTickCap(t *testing.T) {
	q := openQueue(t)
	dir := t.TempDir()
	writeRepo(t, dir, "r1", "- [ ] a\n")
	writeRepo(t, dir, "r2", "- [ ] b\n")
	writeRepo(t, dir, "r3", "- [ ] c\n")

	h := New(q, Config{ProjectsDir: dir, MaxPerTick: 2})
	res, err := h.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Enqueued) != 2 {
		t.Fatalf("enqueued %d, want cap 2", len(res.Enqueued))
	}
	if !hasSkip(res, "tick cap") {
		t.Fatalf("want 'tick cap' skip, got %+v", res.Skipped)
	}
}

func TestRunDLQAndCancelledSkipReasons(t *testing.T) {
	q := openQueue(t)
	ctx := context.Background()
	dir := t.TempDir()
	writeRepo(t, dir, "gamma", "- [ ] poisoned\n")

	h := New(q, Config{ProjectsDir: dir})
	if res, _ := h.Run(ctx); len(res.Enqueued) != 1 {
		t.Fatalf("tick 1: %+v", res.Enqueued)
	}

	// Exhaust attempts: 3 failures → dead.
	var id task.ID
	for range 3 {
		tasks, err := q.List(ctx, queue.Filter{Project: strPtr("gamma")})
		if err != nil || len(tasks) != 1 {
			t.Fatalf("list: %v %d", err, len(tasks))
		}
		id = tasks[0].ID
		claimed, err := q.ClaimDue(ctx, "w", time.Minute)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		if claimed.ID != id {
			t.Fatalf("claimed %s want %s", claimed.ID, id)
		}
		if err := q.Fail(ctx, id, "w", "boom", 0); err != nil {
			t.Fatalf("fail: %v", err)
		}
	}
	got, err := q.Get(ctx, id)
	if err != nil || got.Status != task.Dead {
		t.Fatalf("task should be dead: %v %s", err, got.Status)
	}

	res, err := h.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Enqueued) != 0 {
		t.Fatalf("dead task's item re-enqueued: %+v", res.Enqueued)
	}
	if !hasSkip(res, "in DLQ") {
		t.Fatalf("want DLQ skip reason, got %+v", res.Skipped)
	}
}

func strPtr(s string) *string { return &s }

func hasSkip(res Result, substr string) bool {
	for _, s := range res.Skipped {
		if strings.Contains(s.Reason, substr) {
			return true
		}
	}
	return false
}

// fakeRunToCompletion drives a claimed task to Completed so later harvest
// ticks observe the terminal state.
func fakeRunToCompletion(ctx context.Context, q *queue.Queue, id task.ID) error {
	claimed, err := q.ClaimDue(ctx, "w", time.Minute)
	if err != nil {
		return err
	}
	if claimed.ID != id {
		return fmt.Errorf("claimed %s, want %s", claimed.ID, id)
	}
	return q.Complete(ctx, id, "w", nil)
}
