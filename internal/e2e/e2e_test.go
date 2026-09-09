//go:build unix

// The e2e suite drives stub executables with #!/bin/sh shebangs — POSIX
// only. The unix build tag keeps the GOOS=windows compile gate from
// implying runtime coverage these tests do not have.
package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/harvest"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// The E2E suite drives the REAL CLI as a subprocess — the same binary an
// operator runs — against real repos and a real database. It exists because
// in-process tests cannot catch flag wiring, exit codes, signal handling,
// or stdout/stderr regressions. No agent API cost: TQ_AGENT_BIN points at a
// stub, mirroring scripts/smoke/multi-repo.sh.
//
// Build note: cmd/tq must be built before these tests (TestMain does it).

var tqBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tq-e2e-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	tqBin = filepath.Join(dir, "tq")
	build := exec.Command("go", "build", "-o", tqBin, "github.com/larsartmann/go-taskqueue/cmd/tq")
	build.Dir = repoRoot()

	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build tq: %v\n%s", err, out)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func repoRoot() string {
	wd, _ := os.Getwd()

	return filepath.Dir(filepath.Dir(wd))
}

// TestAgentPoolOnceSubprocess runs `tq agent-pool --once` as a subprocess:
// it must harvest the fixture repo, run the stub agent, complete the task,
// and exit 0 on its own (the --once hang regression).
func TestAgentPoolOnceSubprocess(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeRepo(t, dir, "demorepo", "- [ ] subprocess item\n")

	stub := filepath.Join(dir, "stub-agent")
	writeFile(t, stub, "#!/bin/sh\nexit 0\n", 0o755)

	db := filepath.Join(dir, "q.db")
	cmd := exec.Command(tqBin, "agent-pool",
		"--repos", filepath.Join(dir, "demorepo"),
		"--db", db, "--poll", "50ms", "--once",
	)

	cmd.Env = append(os.Environ(), "TQ_AGENT_BIN="+stub)

	out, err := runWithTimeout(cmd, 30*time.Second)
	if err != nil {
		t.Fatalf("agent-pool --once: %v\n%s", err, out)
	}

	s := openStore(t, db)
	defer func() { _ = s.Close() }()

	assertFactCounts(t, ctx, s, map[string]int{
		"task.enqueued":      1,
		"task.claimed":       1,
		"task.completed":     1,
		"task.dead-lettered": 0,
	})
}

// TestBudgetRefusalSubprocess: with --daily-budget already exhausted, the
// pool must skip the tick with the budget reason and still exit 0.
func TestBudgetRefusalSubprocess(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "demorepo", "- [ ] budgeted item\n")
	stub := filepath.Join(dir, "stub-agent")
	writeFile(t, stub, "#!/bin/sh\nexit 0\n", 0o755)

	db := filepath.Join(dir, "q.db")

	env := append(os.Environ(), "TQ_AGENT_BIN="+stub)

	run := func(extra ...string) string {
		cmd := exec.Command(tqBin, append([]string{
			"agent-pool", "--repos", filepath.Join(dir, "demorepo"),
			"--db", db, "--poll", "50ms", "--once",
		}, extra...)...)
		cmd.Env = env

		out, err := runWithTimeout(cmd, 30*time.Second)
		if err != nil {
			t.Fatalf("run %v: %v\n%s", extra, err, out)
		}

		return out
	}
	run("--daily-budget", "1")

	second := run("--daily-budget", "1")
	if !strings.Contains(second, "budget: skipping harvest tick") {
		t.Fatalf("second run must refuse the tick, output:\n%s", second)
	}
}

// --- helpers ---------------------------------------------------------------

func runWithTimeout(cmd *exec.Cmd, d time.Duration) (string, error) {
	var buf strings.Builder

	if cmd.Stdout != nil {
		panic("stdout already set")
	}

	timer := time.AfterFunc(d, func() { _ = cmd.Process.Kill() })
	defer timer.Stop()

	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()

	return buf.String(), err
}

func runTQ(t *testing.T, dir, db string, args ...string) string {
	t.Helper()

	out, err := runWithTimeout(exec.Command(tqBin, append(args, "--db", db)...), 15*time.Second)
	if err != nil {
		t.Fatalf("tq %v: %v\n%s", args, err, out)
	}

	return strings.TrimSpace(out)
}

func writeRepo(t *testing.T, dir, name, items string) string {
	t.Helper()

	repo := filepath.Join(dir, name)
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(repo, harvest.DefaultTodoFile), "## Work\n\n"+items, 0o644)

	return repo
}

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func openStore(t *testing.T, path string) *queue.SQLiteStore {
	t.Helper()

	s, err := queue.OpenSQLite(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}

	return s
}

func assertFactCounts(t *testing.T, ctx context.Context, s *queue.SQLiteStore, want map[string]int) {
	t.Helper()

	facts, err := s.Facts(ctx, 0, 0)
	if err != nil {
		t.Fatalf("facts: %v", err)
	}

	got := map[string]int{}
	for _, f := range facts {
		got[string(f.Type)]++
	}

	for typ, n := range want {
		if got[typ] != n {
			t.Fatalf("%s = %d, want %d (all: %v)", typ, got[typ], n, got)
		}
	}
}

// TestForceCancelSubprocess: `tq cancel --force` mid-run stops the sleep
// process tree at the worker's next heartbeat and finalizes the task as
// cancelled — far below the 30s the payload would otherwise run.
func TestForceCancelSubprocess(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db := filepath.Join(dir, "q.db")

	id := runTQ(t, dir, db, "enqueue", "--type", "sh", "--project", "e2e", "--payload", "sleep 30")

	worker := exec.Command(tqBin, "worker", "--db", db, "--poll", "50ms", "--lease", "400ms")

	var workerLog strings.Builder

	worker.Stdout = &workerLog

	worker.Stderr = &workerLog
	if err := worker.Start(); err != nil {
		t.Fatalf("start worker: %v", err)
	}
	defer func() { _ = worker.Process.Kill(); _ = worker.Wait() }()

	s := openStore(t, db)
	defer func() { _ = s.Close() }()

	waitStatus := func(want task.Status) task.Task {
		t.Helper()

		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			tk, err := s.Get(ctx, task.ID(id))
			if err != nil {
				t.Fatalf("get: %v", err)
			}

			if tk.Status == want {
				return tk
			}

			time.Sleep(20 * time.Millisecond)
		}

		t.Fatalf("task never reached %s; worker log:\n%s", want, workerLog.String())

		return task.Task{}
	}

	waitStatus(task.Running)

	// Without --force the CLI refuses a running task and says why.
	refuse := exec.Command(tqBin, "cancel", "--db", db, id)

	refuseOut, err := refuse.CombinedOutput()
	if err == nil || !strings.Contains(string(refuseOut), "--force") {
		t.Fatalf("cancel without --force must refuse with guidance, err=%v out=%s", err, refuseOut)
	}

	forced := exec.Command(tqBin, "cancel", "--db", db, "--force", id)
	if out, err := forced.CombinedOutput(); err != nil {
		t.Fatalf("cancel --force: %v\n%s", err, out)
	}

	got := waitStatus(task.Cancelled)
	if got.Attempts != 0 {
		t.Fatalf("cancelled task attempts = %d, want 0", got.Attempts)
	}
}

// TestAuditAndTopJSONSubprocess (round-5 M26/F138): the two reporting
// commands answer machine-readable on a seeded database.
func TestAuditAndTopJSONSubprocess(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "q.db")

	for range 3 {
		runTQ(t, dir, dbPath, "enqueue", "--project", "demo", "--type", "sh",
			"--payload", `"true"`)
	}

	// agent-pool --once drains them with the stub agent? No worker here:
	// top --json on pending work is the contract being pinned.
	out := runTQ(t, dir, dbPath, "top", "--once", "--json")
	if !strings.Contains(out, `"go-taskqueue"`) && !strings.Contains(out, "demo") {
		t.Fatalf("top --json missing project rows: %s", out)
	}

	if !strings.HasPrefix(strings.TrimSpace(out), "{") && !strings.HasPrefix(strings.TrimSpace(out), "[") {
		t.Fatalf("top --json must be JSON, got: %s", out[:min(40, len(out))])
	}

	auditOut := runTQ(t, dir, dbPath, "audit", "--projects-dir", dir, "--json")
	trimmed := strings.TrimSpace(auditOut)

	if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
		t.Fatalf("audit --json must be JSON, got: %s", trimmed[:min(40, len(trimmed))])
	}
}

// TestAgentPoolStartupPruneSweep pins the self-cleaning relaunch contract:
// a pending task left behind by a harvest-while-no-pool-runs window whose
// item was later deleted from TODO_LIST.md must be cancelled by the pool's
// startup zombie sweep — BEFORE the first harvest tick — so the relaunch
// executes nothing stale.
func TestAgentPoolStartupPruneSweep(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	writeRepo(t, dir, "demorepo", "- [ ] stale item done elsewhere\n")

	stub := filepath.Join(dir, "stub-agent")
	writeFile(t, stub, "#!/bin/sh\nexit 0\n", 0o755)

	db := filepath.Join(dir, "q.db")
	env := append(os.Environ(), "TQ_AGENT_BIN="+stub)

	// Harvest with no pool running: one pending task, nothing executes.
	seed := exec.Command(tqBin, "harvest",
		"--repos", filepath.Join(dir, "demorepo"), "--db", db)
	seed.Env = env

	if out, err := runWithTimeout(seed, 30*time.Second); err != nil {
		t.Fatalf("seed harvest: %v\n%s", err, out)
	}

	// The item completes out-of-band and the docs convention deletes it.
	writeRepo(t, dir, "demorepo", "- [ ] a fresh item for the next round\n")

	pool := exec.Command(tqBin, "agent-pool",
		"--repos", filepath.Join(dir, "demorepo"),
		"--db", db, "--poll", "50ms", "--once",
	)
	pool.Env = env

	out, err := runWithTimeout(pool, 30*time.Second)
	if err != nil {
		t.Fatalf("agent-pool --once: %v\n%s", err, out)
	}

	s := openStore(t, db)
	defer func() { _ = s.Close() }()

	// The stale task was cancelled by the startup sweep (never claimed);
	// the fresh item was harvested and completed by the stub.
	assertFactCounts(t, ctx, s, map[string]int{
		"task.enqueued":      2,
		"task.cancelled":     1,
		"task.claimed":       1,
		"task.completed":     1,
		"task.dead-lettered": 0,
	})

	if !strings.Contains(out, "startup prune: cancelled stale task") {
		t.Errorf("pool output missing the startup-prune log line:\n%s", out)
	}
}
