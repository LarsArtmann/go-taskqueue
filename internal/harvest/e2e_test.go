package harvest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
	"github.com/larsartmann/go-taskqueue/internal/worker"
)

// fakeAgentBin writes a stub agent that honors the agent contract: it runs
// with its cwd set to the repo (the executor's cmd.Dir) and checks off the
// first open item in TODO_LIST.md, then exits 0.
func fakeAgentBin(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "fake-agent")

	script := `#!/bin/sh
f="$PWD/TODO_LIST.md"
[ -f "$f" ] || { echo "no todo file in $PWD" >&2; exit 1; }
sed -i '0,/- \[ \]/s//- [x]/' "$f" || exit 1
echo "agent: closed one item in $PWD"
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	return bin
}

// TestSelfManagingLoop is the end-to-end proof of the agent-pool loop:
// harvest feeds TODO items to the queue, the pool's (fake) agent does an
// item and marks it done in TODO_LIST.md, the next harvest tick moves on to
// the next item, and nothing is ever enqueued twice.
func TestSelfManagingLoop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	projects := t.TempDir()
	writeRepo(t, projects, "loop", "## Backlog\n\n- [ ] first item\n- [ ] second item\n")

	s, err := queue.OpenSQLite(filepath.Join(t.TempDir(), "e2e.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	q := queue.New(s)

	reg := executor.NewRegistry()
	reg.Register(DefaultType, &executor.AgentExecutor{Bin: fakeAgentBin(t), ProjectsDir: projects})

	noClean := false
	h := New(q, Config{ProjectsDir: projects, RequireClean: &noClean})

	pool := worker.New(s, worker.Config{
		Owner:        "e2e-pool",
		Concurrency:  1,
		PollInterval: 20 * time.Millisecond,
		Lease:        time.Minute,
		Executors:    reg,
	}, nil)
	go func() { _ = pool.Start(ctx) }()

	defer pool.Stop()

	// Tick 1: first item enqueued and worked to completion by the pool.
	res, err := h.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Enqueued) != 1 || res.Enqueued[0].Item.Text != "first item" {
		t.Fatalf("tick 1 enqueued = %+v", res.Enqueued)
	}

	waitFor(t, ctx, func() bool { return taskStatus(t, ctx, q, res.Enqueued[0].TaskID) == task.Completed })

	// Tick 2: first item is done in the file (not re-enqueued), repo is idle,
	// second item becomes the next task.
	res, err = h.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Enqueued) != 1 || res.Enqueued[0].Item.Text != "second item" {
		t.Fatalf("tick 2 enqueued = %+v, want 'second item'", res.Enqueued)
	}

	waitFor(t, ctx, func() bool { return taskStatus(t, ctx, q, res.Enqueued[0].TaskID) == task.Completed })

	// Tick 3: backlog empty, loop is idle — no third task, no duplicates.
	res, err = h.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Enqueued) != 0 {
		t.Fatalf("tick 3 enqueued = %+v, want none (backlog drained)", res.Enqueued)
	}

	// Final state: both items checked off by the "agent", both tasks completed.
	todo, err := os.ReadFile(filepath.Join(projects, "loop", DefaultTodoFile))
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(todo), "- [ ]") {
		t.Fatalf("TODO_LIST.md still has open items:\n%s", todo)
	}

	tasks, err := q.List(ctx, queue.Filter{Project: new("loop")})
	if err != nil || len(tasks) != 2 {
		t.Fatalf("tasks: %v %d", err, len(tasks))
	}

	for _, tk := range tasks {
		if tk.Status != task.Completed {
			t.Fatalf("task %s status = %s, want completed", tk.ID, tk.Status)
		}
	}
}

func taskStatus(t *testing.T, ctx context.Context, q *queue.Queue, id task.ID) task.Status {
	t.Helper()

	got, err := q.Get(ctx, id)
	if err != nil {
		t.Fatalf("get %s: %v", id, err)
	}

	return got.Status
}

func waitFor(t *testing.T, ctx context.Context, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		select {
		case <-ctx.Done():
			t.Fatalf("condition not met before ctx done: %v", ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}

	t.Fatal("condition not met before deadline")
}
