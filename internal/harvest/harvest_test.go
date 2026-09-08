package harvest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
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

// TestRunSkipsBlockedItems pins the TODO_LIST "BLOCKED: <reason>" convention
// from the agent contract: when an agent cannot finish an item it appends the
// marker, and the next harvest tick must not spend another agent run on it.
func TestRunSkipsBlockedItems(t *testing.T) {
	q := openQueue(t)
	dir := t.TempDir()
	writeRepo(
		t,
		dir,
		"alpha",
		"## Work\n\n- [ ] do the thing\n- [ ] waiting on credentials — BLOCKED: needs owner API token\n",
	)

	h := New(q, Config{ProjectsDir: dir})

	res, err := h.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(res.Enqueued) != 1 || res.Enqueued[0].Item.Text != "do the thing" {
		t.Fatalf("enqueued = %+v, want only 'do the thing'", res.Enqueued)
	}

	if !hasSkip(res, "blocked: needs owner API token") {
		t.Fatalf("skips = %+v, want the blocked item skipped with its reason", res.Skipped)
	}
}

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

// TestRunModelLandsInAgentPayload pins the --model plumbing: Config.Model
// must reach the AgentPayload inside the stored payload, so a pool operator
// can pin a cheaper/better model without editing repos.
func TestRunModelLandsInAgentPayload(t *testing.T) {
	q := openQueue(t)
	dir := t.TempDir()
	writeRepo(t, dir, "gamma", "## Work\n\n- [ ] model me\n")

	h := New(q, Config{ProjectsDir: dir, Model: "prov/cheap-1"})

	res, err := h.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(res.Enqueued) != 1 {
		t.Fatalf("enqueued = %+v, want 1", res.Enqueued)
	}

	got, err := q.Get(context.Background(), res.Enqueued[0].TaskID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	var p struct {
		executor.AgentPayload

		Dedup string `json:"dedup"`
	}
	if err := json.Unmarshal(got.Payload, &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if p.Model != "prov/cheap-1" {
		t.Fatalf("payload model = %q, want prov/cheap-1", p.Model)
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

	tasks, err := q.List(ctx, queue.Filter{Project: new("beta")})
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
		tasks, err := q.List(ctx, queue.Filter{Project: new("gamma")})
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

		if err := q.Fail(ctx, id, "w", "boom", 0, nil); err != nil {
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

//go:fix inline

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

// TestRunPinsRepoVerifyIntoPayload: a repo that declares .tq-verify gets its
// command written into every harvested payload, so tasks record their gate.
func TestRunPinsRepoVerifyIntoPayload(t *testing.T) {
	q := openQueue(t)
	dir := t.TempDir()
	writeRepo(t, dir, "delta", "## Work\n\n- [ ] gated item\n")

	if err := os.WriteFile(
		filepath.Join(dir, "delta", ".tq-verify"),
		[]byte("go vet ./... && go test ./...\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	h := New(q, Config{ProjectsDir: dir})

	res, err := h.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(res.Enqueued) != 1 {
		t.Fatalf("enqueued = %+v, want 1", res.Enqueued)
	}

	got, err := q.Get(context.Background(), res.Enqueued[0].TaskID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	var p struct {
		executor.AgentPayload

		Dedup string `json:"dedup"`
	}
	if err := json.Unmarshal(got.Payload, &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if p.Verify != "go vet ./... && go test ./..." {
		t.Fatalf("payload verify = %q, want the repo's .tq-verify command", p.Verify)
	}
}

// TestRunDLQBackoffPausesPoisonedRepos: a repo whose recent work all went
// dead gets a cooldown instead of fresh failures every tick; a completed
// task anywhere in its history (or a zero backoff) lifts the guard.
func TestRunDLQBackoffPausesPoisonedRepos(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	// Poison repo "eps": enqueue one item, drive it to dead.
	q := openQueue(t)
	writeRepo(t, dir, "eps", "## Work\n\n- [ ] doomed\n")
	h := New(q, Config{ProjectsDir: dir})

	res, err := h.Run(ctx)
	if err != nil || len(res.Enqueued) != 1 {
		t.Fatalf("seed run: %+v, %v", res.Enqueued, err)
	}

	tk, _ := q.Get(ctx, res.Enqueued[0].TaskID)
	if _, err := q.Store.ClaimDue(ctx, "w", time.Minute); err != nil {
		t.Fatalf("claim: %v", err)
	}

	if err := q.Store.FailPermanent(ctx, tk.ID, "w", "repo is broken", nil); err != nil {
		t.Fatalf("fail: %v", err)
	}

	// New item appears; with backoff on, the repo is paused.
	writeRepo(t, dir, "eps", "## Work\n\n- [ ] doomed\n- [ ] fresh item\n")
	hp := New(q, Config{ProjectsDir: dir, DLQBackoff: 30 * time.Minute})

	res, err = hp.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(res.Enqueued) != 0 || !hasSkip(res, "poisoned: recent dead-letter") {
		t.Fatalf("poisoned repo must be paused: enqueued=%+v skips=%+v", res.Enqueued, res.Skipped)
	}

	// With the guard off (default), the fresh item is enqueued as before.
	hn := New(q, Config{ProjectsDir: dir})

	res, err = hn.Run(ctx)
	if err != nil || len(res.Enqueued) != 1 {
		t.Fatalf("default must keep harvesting fresh items: %+v, %v", res.Enqueued, err)
	}

	// A completed task lifts the guard even with backoff on.
	if _, err := q.Store.ClaimDue(ctx, "w", time.Minute); err != nil {
		t.Fatalf("claim2: %v", err)
	}

	if err := q.Store.Complete(ctx, res.Enqueued[0].TaskID, "w", nil); err != nil {
		t.Fatalf("complete: %v", err)
	}

	writeRepo(t, dir, "eps", "## Work\n\n- [ ] doomed\n- [ ] third item\n")

	res, err = hp.Run(ctx)
	if err != nil || len(res.Enqueued) != 1 {
		t.Fatalf("completion must lift the poisoned guard: %+v, %v", res.Enqueued, err)
	}
}

// TestRunPerRepoInterval: Config.RepoIntervals throttles new enqueues per
// repo independently of the tick pacing.
func TestRunPerRepoInterval(t *testing.T) {
	ctx := context.Background()
	q := openQueue(t)
	dir := t.TempDir()
	writeRepo(t, dir, "zeta", "## Work\n\n- [ ] one\n")

	h := New(q, Config{ProjectsDir: dir, RepoIntervals: map[string]time.Duration{"zeta": time.Hour}})

	res, err := h.Run(ctx)
	if err != nil || len(res.Enqueued) != 1 {
		t.Fatalf("first run: %+v, %v", res.Enqueued, err)
	}

	// A brand-new item still hits the interval (last enqueue was moments ago).
	writeRepo(t, dir, "zeta", "## Work\n\n- [ ] one\n- [ ] two\n")

	res, err = h.Run(ctx)
	if err != nil || len(res.Enqueued) != 0 || !hasSkip(res, "paced: per-repo interval") {
		t.Fatalf("interval must gate new items: enqueued=%+v skips=%+v", res.Enqueued, res.Skipped)
	}

	// Other repos are unaffected by zeta's interval.
	writeRepo(t, dir, "eta", "## Work\n\n- [ ] eta item\n")

	res, err = h.Run(ctx)
	if err != nil || len(res.Enqueued) != 1 {
		t.Fatalf("interval must be per-repo: %+v, %v", res.Enqueued, err)
	}
}

// taughtResultLine extracts the TQ_RESULT example a prompt teaches agents to
// emit, so the contract tests can parse it with the executor's real parser.
func taughtResultLine(t *testing.T, prompt string) string {
	t.Helper()

	for line := range strings.SplitSeq(prompt, "\n") {
		if strings.HasPrefix(line, "TQ_RESULT: ") {
			return line
		}
	}

	t.Fatal("prompt does not teach the TQ_RESULT self-report line")

	return ""
}

// TestAgentPromptsTeachParsableResult pins the TQ_RESULT contract end to end:
// the prompts are the only place pool agents learn the queue's conventions,
// so the taught example must parse with the executor's real parser. If prompt
// and parser drift, agents finish fine but `tq show` silently loses
// files_changed/commit_sha for every pool task.
func TestAgentPromptsTeachParsableResult(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		prompt string
	}{
		{"work item", DefaultPromptTemplate},
		{"catch-up", DefaultCatchupPrompt},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			line := taughtResultLine(t, tc.prompt)

			files, sha, ok := executor.ExtractResultPayload(line)
			if !ok {
				t.Fatalf("taught line %q does not parse", line)
			}

			if len(files) == 0 || sha == "" {
				t.Fatalf("taught line %q parses to empty files/sha", line)
			}
		})
	}
}

// TestAgentPromptsGuardrails pins the self-modifying-autonomy guardrail from
// the dogfood plan's accepted-risk list: prompts must forbid touching
// .crushrc/.tq-verify (they define the agent's own autonomy and verify gate),
// must restate the BLOCKED marker the harvester itself skips on, and must
// keep the commit-never-push rule.
func TestAgentPromptsGuardrails(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name       string
		prompt     string
		wantSubstr []string
	}{
		{
			name:       "work item",
			prompt:     DefaultPromptTemplate,
			wantSubstr: []string{".crushrc", ".tq-verify", "Never push", "— BLOCKED: <one-line reason>"},
		},
		{
			name:       "catch-up",
			prompt:     DefaultCatchupPrompt,
			wantSubstr: []string{".crushrc", ".tq-verify", "Never push"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			for _, want := range tc.wantSubstr {
				if !strings.Contains(tc.prompt, want) {
					t.Fatalf("prompt lost %q:\n%s", want, tc.prompt)
				}
			}
		})
	}
}

// TestRunRepoTimeoutLadder: Config.RepoTimeouts pins the ladder value into
// each harvested payload's TimeoutMinutes; repos without an entry keep the
// executor default (field omitted).
func TestRunRepoTimeoutLadder(t *testing.T) {
	ctx := context.Background()
	q := openQueue(t)
	dir := t.TempDir()
	writeRepo(t, dir, "big", "## Work\n\n- [ ] heavy item\n")
	writeRepo(t, dir, "small", "## Work\n\n- [ ] tiny item\n")

	h := New(q, Config{ProjectsDir: dir, RepoTimeouts: map[string]time.Duration{"big": 45 * time.Minute}})

	res, err := h.Run(ctx)
	if err != nil || len(res.Enqueued) != 2 {
		t.Fatalf("run: %+v, %v", res.Enqueued, err)
	}

	store := q.Store
	for _, enq := range res.Enqueued {
		tk, err := store.Get(ctx, enq.TaskID)
		if err != nil {
			t.Fatalf("get %s: %v", enq.TaskID, err)
		}

		var p struct {
			Repo           string `json:"repo"`
			TimeoutMinutes int    `json:"timeout_minutes"`
		}
		if err := json.Unmarshal(tk.Payload, &p); err != nil {
			t.Fatalf("payload %s: %v", tk.ID, err)
		}

		switch p.Repo {
		case "big":
			if p.TimeoutMinutes != 45 {
				t.Errorf("big repo timeout_minutes = %d, want 45", p.TimeoutMinutes)
			}
		case "small":
			if p.TimeoutMinutes != 0 {
				t.Errorf("small repo timeout_minutes = %d, want 0 (default, omitted)", p.TimeoutMinutes)
			}
		}
	}
}
