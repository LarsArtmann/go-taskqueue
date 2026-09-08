//go:build unix

// Chaos tests SIGKILL worker processes and run #!/bin/sh stub executables —
// POSIX only. The unix build tag keeps the GOOS=windows compile gate from
// implying runtime coverage these tests do not have.
package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestChaosKillWorkerMidRun: a worker SIGKILLed mid-task must leave the
// task recoverable — lease expiry lets another worker reclaim it, and the
// journal must never show two completions for one task (at-least-once,
// never double-done).
func TestChaosKillWorkerMidRun(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "slow-stub")
	// Task runs 30s — the kill lands mid-run.
	writeFile(t, stub, "#!/bin/sh\nsleep 30\nexit 0\n", 0o755)

	idLine := runTQ(t, dir, filepath.Join(dir, "q.db"),
		"enqueue", "--project", "chaos", "--type", "sh",
		"--payload", `"sleep 30"`,
	)
	// enqueue prints the task ID.
	taskID := strings.TrimSpace(idLine)

	// Start worker as a real process on the same DB.
	worker := exec.Command(tqBin, "worker", "--db", filepath.Join(dir, "q.db"),
		"--owner", "victim", "--poll", "20ms", "--lease", "1500ms", "--concurrency", "1")
	if err := worker.Start(); err != nil {
		t.Fatalf("start worker: %v", err)
	}

	// Wait until the victim actually claimed it.
	ctx := context.Background()

	s := openStore(t, filepath.Join(dir, "q.db"))
	defer func() { _ = s.Close() }()

	deadline := time.Now().Add(10 * time.Second)

	for {
		got, err := s.Get(ctx, task.ID(taskID))
		if err == nil && got.Status == "running" {
			break
		}

		if time.Now().After(deadline) {
			t.Fatalf("task never claimed by victim worker (last: %+v)", got)
		}

		time.Sleep(10 * time.Millisecond)
	}

	// SIGKILL the whole worker process mid-run.
	if err := worker.Process.Kill(); err != nil {
		t.Fatal(err)
	}

	_, _ = worker.Process.Wait()

	deadline = time.Now().Add(15 * time.Second)

	for {
		got, err := s.Get(ctx, task.ID(taskID))
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		// Wait for the lease to expire (2m default is too slow for a test;
		// poll ClaimDue with a normal lease instead — reclaim is allowed
		// the moment lease_expires passes).
		if got.Status == "pending" || got.Status == "completed" {
			t.Fatalf("unexpected status while waiting for expiry: %s", got.Status)
		}

		if got.LeaseExpires != nil && time.Now().After(*got.LeaseExpires) {
			break
		}

		if time.Now().After(deadline) {
			t.Fatalf("lease never expired (expires=%v)", got.LeaseExpires)
		}

		time.Sleep(50 * time.Millisecond)
	}

	// The reclaiming worker finishes the task.
	if _, err := s.ClaimDue(ctx, "successor", time.Minute); err != nil {
		t.Fatalf("successor claim: %v", err)
	}

	if err := s.Complete(ctx, task.ID(taskID), "successor", nil); err != nil {
		t.Fatalf("successor complete: %v", err)
	}

	// Invariant: exactly one completion in the journal.
	facts, _ := s.Facts(ctx, 0, 0)
	completions := 0

	for _, f := range facts {
		if f.TaskID == taskID && f.Type == "task.completed" {
			completions++
		}
	}

	if completions != 1 {
		t.Fatalf("task %s completed %d times, want exactly 1", taskID, completions)
	}
}

// TestChaosKillAgentPoolOnceMidDrain: SIGKILL an `agent-pool --once` while
// it is mid-task; a restarted pool must reclaim the expired lease, finish
// the work, and still exit cleanly — and the journal must show exactly one
// completion (round-5 M15/F77).
func TestChaosKillAgentPoolOnceMidDrain(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "q.db")

	idLine := runTQ(t, dir, dbPath, "enqueue", "--project", "chaos", "--type", "sh",
		"--payload", `"sleep 3"`)
	taskID := strings.TrimSpace(idLine)

	// An empty repos dir: the harvest tick finds nothing, the drain runs the
	// queued sh task.
	repos := filepath.Join(dir, "repos")
	if err := os.MkdirAll(repos, 0o755); err != nil {
		t.Fatal(err)
	}

	pool := exec.Command(tqBin, "agent-pool", "--db", dbPath,
		"--repos", repos, "--once",
		"--poll", "20ms", "--lease", "1s", "--concurrency", "1",
		"--task-timeout", "2m")
	if err := pool.Start(); err != nil {
		t.Fatalf("start pool: %v", err)
	}

	ctx := context.Background()
	s := openStore(t, dbPath)
	defer func() { _ = s.Close() }()

	deadline := time.Now().Add(10 * time.Second)

	for {
		got, err := s.Get(ctx, task.ID(taskID))
		if err == nil && got.Status == "running" {
			break
		}

		if time.Now().After(deadline) {
			t.Fatalf("task never claimed by pool (last: %+v)", got)
		}

		time.Sleep(10 * time.Millisecond)
	}

	if err := pool.Process.Kill(); err != nil {
		t.Fatal(err)
	}

	_, _ = pool.Process.Wait()

	// Wait out the lease before restarting: --once drains CLAIMABLE work,
	// and a Running task with a still-live lease is not claimable yet (the
	// pool would exit immediately and leave the reclaim to the next run).
	time.Sleep(1200 * time.Millisecond)

	// Restart: the successor pool reclaims the expired lease, drains the
	// task to completion, then --once exits 0.
	runTQ(t, dir, dbPath, "agent-pool",
		"--repos", repos, "--once",
		"--poll", "20ms", "--lease", "1s", "--concurrency", "1",
		"--task-timeout", "2m")

	got, err := s.Get(ctx, task.ID(taskID))
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if got.Status != task.Completed {
		t.Fatalf("status after restart = %s, want completed", got.Status)
	}

	facts, _ := s.Facts(ctx, 0, 0)
	completions := 0

	for _, f := range facts {
		if f.TaskID == taskID && f.Type == "task.completed" {
			completions++
		}
	}

	if completions != 1 {
		t.Fatalf("task %s completed %d times, want exactly 1", taskID, completions)
	}
}
