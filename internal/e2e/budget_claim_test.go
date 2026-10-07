//go:build unix

package e2e

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/budget"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestBudgetClaimGateParksOverCapSubprocess pins the claim-time money gate
// end-to-end: work enqueued BEFORE the cap bit must never run AFTER it. Two
// single-item repos are harvested while uncapped (spent=2 enqueued-today; two
// repos because harvest coalesces one live task per repo); a --once pool with
// --daily-budget 1 then claims both — and must park each one until the next
// local midnight (requeue class "budget", no attempt burned) without the stub
// agent ever running, and still exit 0. The NotBefore-vs-midnight assertion
// rides budget.NextMidnight, so the DST-correct computation is proven through
// the real CLI, not just unit tables.
func TestBudgetClaimGateParksOverCapSubprocess(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()

	repoOne := writeRepo(t, dir, "repo-one", "- [ ] budget claim item one\n")
	repoTwo := writeRepo(t, dir, "repo-two", "- [ ] budget claim item two\n")
	repos := repoOne + "," + repoTwo

	marker := filepath.Join(dir, "agent-ran")
	stub := filepath.Join(dir, "stub-agent")
	writeFile(t, stub, "#!/bin/sh\ntouch "+marker+"\nexit 0\n", 0o755)

	db := filepath.Join(dir, "q.db")

	env := append(os.Environ(), "TQ_AGENT_BIN="+stub)

	// Seed uncapped: both items enqueue (spent=2), nothing executes.
	seed := exec.Command(tqBin, "harvest", "--repos", repos, "--db", db)

	seed.Env = env
	if out, err := runWithTimeout(seed, 30*time.Second); err != nil {
		t.Fatalf("seed harvest: %v\n%s", err, out)
	}

	// Capped pool: cap 1 < spent 2 → every claim parks, none runs.
	pool := exec.Command(tqBin, "agent-pool",
		"--repos", repos, "--db", db, "--poll", "50ms", "--once",
		"--daily-budget", "1")

	pool.Env = env
	if out, err := runWithTimeout(pool, 60*time.Second); err != nil {
		t.Fatalf("agent-pool (capped): %v\n%s", err, out)
	}

	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("stub agent ran on a spent budget (marker stat err=%v)", err)
	}

	s := openStore(t, db)
	defer func() { _ = s.Close() }()

	assertFactCounts(t, ctx, s, map[string]int{
		"task.enqueued":      2,
		"task.claimed":       2,
		"task.requeued":      2,
		"task.completed":     0,
		"task.failed":        0,
		"task.dead-lettered": 0,
	})

	// Every requeue must carry the budget class — the parked tasks must be
	// distinguishable from rate-limit/env requeues on every surface.
	facts, err := s.Facts(ctx, 0, 0)
	if err != nil {
		t.Fatalf("facts: %v", err)
	}

	requeues := 0

	for _, f := range facts {
		if f.Type != "task.requeued" {
			continue
		}

		requeues++

		var evidence queue.RequeueEvidence
		if err := json.Unmarshal(f.Detail, &evidence); err != nil {
			t.Fatalf("decode requeue detail %s: %v", f.Detail, err)
		}

		if evidence.Class != queue.RequeueClassBudget {
			t.Fatalf("requeue class = %q, want %q", evidence.Class, queue.RequeueClassBudget)
		}
	}

	if requeues != 2 {
		t.Fatalf("requeue facts = %d, want 2", requeues)
	}

	// Both tasks stay pending with the attempt intact, parked until the
	// next local midnight (±2min for cross-process skew).
	wantPark := budget.NextMidnight(time.Now())

	tasks, err := s.List(ctx, queue.Filter{})
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}

	if len(tasks) != 2 {
		t.Fatalf("tasks = %d, want 2", len(tasks))
	}

	for _, tk := range tasks {
		if tk.Status != task.Pending {
			t.Fatalf("%s status = %q, want pending", tk.ID, tk.Status)
		}

		if tk.Attempts != 0 {
			t.Fatalf("%s attempts = %d, want 0 (budget park must not burn)", tk.ID, tk.Attempts)
		}

		if d := tk.NotBefore.Sub(wantPark); d < -2*time.Minute || d > 2*time.Minute {
			t.Fatalf("%s not_before = %v, want ~%v (delta %s)", tk.ID, tk.NotBefore, wantPark, d)
		}
	}
}
