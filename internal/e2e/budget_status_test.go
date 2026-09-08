//go:build unix

package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue"
)

// TestBudgetCapsStatusMintedEnqueues pins the loop's documented blast-radius
// cap end-to-end (SECURITY.md: "--daily-budget caps EVERY enqueue incl.
// status-minted"): once the daily budget is spent, a completed agent task
// must NOT mint a status report task — the budget refusal returns before
// the sweepers run. The uncapped twin run proves the mint WOULD have
// happened, so the refusal (not a broken sweeper) is what suppressed it.
func TestBudgetCapsStatusMintedEnqueues(t *testing.T) {
	run := func(t *testing.T, budget string) (string, string) {
		t.Helper()

		dir := t.TempDir()
		writeRepo(t, dir, "demorepo", "- [ ] budgeted item\n")

		stub := filepath.Join(dir, "stub-agent")
		writeFile(t, stub, "#!/bin/sh\nexit 0\n", 0o755)

		db := filepath.Join(dir, "q.db")
		cmd := exec.Command(tqBin,
			"agent-pool", "--repos", filepath.Join(dir, "demorepo"),
			"--db", db, "--poll", "50ms", "--once",
			"--status-every", "1", "--daily-budget", budget)
		cmd.Env = append(os.Environ(), "TQ_AGENT_BIN="+stub)

		out, err := runWithTimeout(cmd, 60*time.Second)
		if err != nil {
			t.Fatalf("agent-pool (budget %s): %v\n%s", budget, err, out)
		}

		return db, out
	}

	countStatus := func(t *testing.T, db string) int {
		t.Helper()

		s := openStore(t, db)
		defer s.Close() //nolint:errcheck

		tq := "status"
		tasks, err := s.List(context.Background(), queue.Filter{Type: &tq})
		if err != nil {
			t.Fatalf("list status tasks: %v", err)
		}

		return len(tasks)
	}

	// Positive control: budget high enough for item + status mint.
	db, out := run(t, "10")
	if n := countStatus(t, db); n != 1 {
		t.Fatalf("uncapped run minted %d status tasks, want 1 (sweeper works)\noutput:\n%s", n, out)
	}

	// The cap: budget covers only the item's own enqueue.
	db, out = run(t, "1")
	if n := countStatus(t, db); n != 0 {
		t.Fatalf("capped run minted %d status tasks, want 0 (budget refusal must gate the sweeper)\noutput:\n%s", n, out)
	}

	if !strings.Contains(out, "daily budget exhausted") {
		t.Fatalf("capped run must log a budget refusal, output:\n%s", out)
	}
}
