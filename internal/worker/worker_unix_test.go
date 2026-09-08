//go:build unix

package worker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// testStubScript writes an executable stub binary and returns its path.
// Unix-only: the stub is a /bin/sh script, so the Windows test job must
// skip this suite (platform honesty, round-5 M11/F55).
func testStubScript(t *testing.T, script string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "stub-bin")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	return path
}

// TestAgentResultDetailStored: on success the executor's structured result
// (crush session id, verify tail) lands in the task.completed fact detail,
// so `tq show` can answer "what did the agent do" without log-diving.
func TestAgentResultDetailStored(t *testing.T) {
	store := testStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reg := executor.NewRegistry()
	reg.Register(executor.TaskTypeAgent, &executor.AgentExecutor{
		// The stub prints a session id the extractor must find.
		Bin: testStubScript(t, "#!/bin/sh\necho 'session: crush-abc-123'\nexit 0\n"),
	})

	repo := t.TempDir() // non-git repo: skips the clean-tree guard

	payload, err := executor.RenderAgentPayload(executor.AgentPayload{Repo: repo, Prompt: "do it"})
	if err != nil {
		t.Fatal(err)
	}

	enq, _ := store.Enqueue(ctx, task.New{Project: "demo", Type: executor.TaskTypeAgent, Payload: payload})

	pool := New(store, Config{
		Concurrency: 1, PollInterval: 5 * time.Millisecond, TaskTimeout: 5 * time.Second,
		Executors: reg,
	}, quietLog())
	go func() { _ = pool.Start(ctx) }()

	waitFor(t, ctx, store, enq.ID, task.Completed)
	cancel()

	facts, _ := store.Facts(context.Background(), 0, 0)
	for _, f := range facts {
		if f.Type == "task.completed" && len(f.Detail) > 0 {
			if !strings.Contains(string(f.Detail), "crush-abc-123") {
				t.Fatalf("completion detail missing session id: %s", f.Detail)
			}

			return
		}
	}

	t.Fatal("completed fact carries no result detail")
}
