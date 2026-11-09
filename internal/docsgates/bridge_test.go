//go:build unix

package docsgates

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func newTestStore(t *testing.T) *sqlite.Store {
	t.Helper()

	s, err := sqlite.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	return s
}

func stubRunner(result RunResult, err error) Runner {
	return func(context.Context, Repo) (RunResult, error) { return result, err }
}

func matrix(verdicts ...string) RunResult {
	res := RunResult{Pass: true}

	for _, v := range verdicts {
		name := "freshness"
		switch v {
		case "FAIL":
			name = strings.ToUpper(v + "-gate")
		}

		g := Gate{Name: name, Verdict: v}
		if v == "SKIP" {
			g.Name = "audit-all"
			g.Skipped = true
			g.Verdict = "SKIP"
		}

		res.Gates = append(res.Gates, g)
	}

	return res
}

func listTasks(t *testing.T, s *sqlite.Store) []task.Task {
	t.Helper()

	tasks, err := s.List(context.Background(), queue.Filter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	return tasks
}

func TestSweepMintsOneTaskPerFailGate(t *testing.T) {
	s := newTestStore(t)

	bridge := NewBridge(BridgeConfig{
		Store:  s,
		Runner: stubRunner(matrix("PASS", "FAIL", "SKIP"), nil),
		Repos:  []Repo{{Name: "go-pbx"}},
	})

	stats, err := bridge.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.Repos != 1 || stats.Failed != 1 || stats.Minted != 1 || stats.Skipped != 1 {
		t.Fatalf("first pass stats: %+v", stats)
	}

	tasks := listTasks(t, s)
	if len(tasks) != 1 {
		t.Fatalf("one fix task minted, got %d", len(tasks))
	}

	tk := tasks[0]
	if tk.DedupKey != "docs-gates:go-pbx:FAIL-GATE" {
		t.Fatalf("dedup key: %s", tk.DedupKey)
	}

	if tk.Type != executor.TaskTypeAgent || tk.Project != "go-pbx" {
		t.Fatalf("task shape: type=%s project=%s", tk.Type, tk.Project)
	}

	// Second identical pass: fully deduped.
	stats, err = bridge.Sweep(context.Background())
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}

	if stats.Minted != 0 || stats.Known != 1 {
		t.Fatalf("re-pass must dedup, got %+v", stats)
	}
}

func TestSweepMultipleReposAndGates(t *testing.T) {
	s := newTestStore(t)

	bridge := NewBridge(BridgeConfig{
		Store: s,
		Runner: func(_ context.Context, repo Repo) (RunResult, error) {
			if repo.Name == "a" {
				return matrix("FAIL", "FAIL"), nil
			}

			return matrix("PASS"), nil
		},
		Repos: []Repo{{Name: "a"}, {Name: "b"}},
	})

	stats, err := bridge.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.Failed != 2 || stats.Minted != 2 {
		t.Fatalf("stats: %+v", stats)
	}
}

func TestSweepRunnerFailureAbortsWithoutCheckpoint(t *testing.T) {
	s := newTestStore(t)

	bridge := NewBridge(BridgeConfig{
		Store:  s,
		Runner: stubRunner(RunResult{}, errors.New("boom")),
		Repos:  []Repo{{Name: "a"}},
	})

	if _, err := bridge.Sweep(context.Background()); err == nil {
		t.Fatal("runner failure must surface")
	}

	_, exists, err := s.Watermark(context.Background(), WatermarkKey)
	if err != nil {
		t.Fatalf("watermark read: %v", err)
	}

	if exists {
		t.Fatal("failed pass must NOT checkpoint the cursor")
	}
}

func TestCursorCheckpointsAtHeadAfterPass(t *testing.T) {
	s := newTestStore(t)

	bridge := NewBridge(BridgeConfig{
		Store:  s,
		Runner: stubRunner(matrix("PASS"), nil),
		Repos:  []Repo{{Name: "a"}},
	})

	// The journal gains facts from the sweep itself only via mints; a clean
	// pass still records coverage at the current head.
	if _, err := bridge.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	seq, exists, err := bridge.Cursor(context.Background())
	if err != nil || !exists {
		t.Fatalf("cursor after pass: seq=%d exists=%t err=%v", seq, exists, err)
	}

	if seq < 0 {
		t.Fatalf("cursor must be a journal seq, got %d", seq)
	}
}

func TestSweepCompletedDedupKeyCountsKnown(t *testing.T) {
	s := newTestStore(t)

	bridge := NewBridge(BridgeConfig{
		Store:  s,
		Runner: stubRunner(matrix("FAIL"), nil),
		Repos:  []Repo{{Name: "a"}},
	})

	if _, err := bridge.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	tk := listTasks(t, s)[0]
	tk2, claim, err := s.ClaimDue(context.Background(), "test-owner", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if tk2.ID != tk.ID {
		t.Fatalf("claimed unexpected task %s", tk2.ID)
	}

	if err := s.Complete(context.Background(), tk.ID, claim, nil); err != nil {
		t.Fatalf("complete: %v", err)
	}

	stats, err := bridge.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep after completion: %v", err)
	}

	if stats.Known != 1 || stats.Minted != 0 {
		t.Fatalf("completed dedup key is Known, never a failure: %+v", stats)
	}
}

func TestBinaryRunnerParsesMatrix(t *testing.T) {
	script := filepath.Join(t.TempDir(), "docs-gates")
	stub := "#!/bin/sh\ncat <<'JSON'\n{\n  \"pass\": false,\n  \"gates\": [\n    {\"name\": \"freshness\", \"verdict\": \"FAIL\", \"exit_code\": 1, \"skipped\": false},\n    {\"name\": \"cadence\", \"verdict\": \"PASS\", \"exit_code\": 0, \"skipped\": false}\n  ]\n}\nJSON\n"
	if err := os.WriteFile(script, []byte(stub), 0o700); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	run := binaryRunner(script, time.Minute)

	res, err := run(context.Background(), Repo{Name: "r", Dir: "."})
	if err != nil {
		t.Fatalf("run stub: %v", err)
	}

	if res.Pass || len(res.Gates) != 2 || res.Gates[0].Verdict != "FAIL" {
		t.Fatalf("parsed matrix: %+v", res)
	}
}

// TestBinaryRunnerExitOneOnFAIL pins the real binary's contract: `docs-gates
// run --json` exits 1 (ExitFindings) when any gate FAILs, and the runner
// must treat that as a verdict-carrying run, not an error — a red-gate repo
// must still mint fix tasks instead of aborting the sweep.
func TestBinaryRunnerExitOneOnFAIL(t *testing.T) {
	script := filepath.Join(t.TempDir(), "docs-gates")
	stub := `#!/bin/sh
cat <<'JSON'
{"pass": false, "gates": [{"name": "freshness", "verdict": "FAIL", "exit_code": 1, "skipped": false}]}
JSON
exit 1
`
	if err := os.WriteFile(script, []byte(stub), 0o700); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	run := binaryRunner(script, time.Minute)

	res, err := run(context.Background(), Repo{Name: "r", Dir: "."})
	if err != nil {
		t.Fatalf("run stub (exit 1 with FAIL verdict must not error): %v", err)
	}

	if res.Pass || len(res.Gates) != 1 || res.Gates[0].Verdict != "FAIL" {
		t.Fatalf("parsed matrix: %+v", res)
	}
}
