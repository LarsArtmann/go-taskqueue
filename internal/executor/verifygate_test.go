//go:build unix

package executor

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// commitAll stages and commits everything in dir with a fixed identity
// (setupGitRepo does not leave a repo-local identity, so later commits need
// their own env).
func commitAll(t *testing.T, dir, msg string) {
	t.Helper()

	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}

	cmd = exec.Command("git", "commit", "-qm", msg)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}
}

// TestVerifyGateDeadIsNotATaskFailure pins classification shape (1): the
// gate fails at the PRE-attempt rev too (here: it fails everywhere), so the
// executor must return a *VerifyGateError (gate-dead) — the worker requeues
// WITHOUT burning an attempt — instead of the plain verify-failed error.
func TestVerifyGateDeadIsNotATaskFailure(t *testing.T) {
	repo := t.TempDir()
	setupGitRepo(t, repo)

	if err := os.WriteFile(filepath.Join(repo, ".tq-verify"), []byte("false"), 0o644); err != nil {
		t.Fatal(err)
	}

	commitAll(t, repo, "pin dead gate")

	// The agent does real work AND commits it — the gate still fails at the
	// baseline rev, so the failure is the gate's, not the work's.
	e := &AgentExecutor{Bin: makeStubAgent(t, "echo work > work.txt && git add -A && git commit -qm work")}

	err := e.Execute(context.Background(), agentTaskT(t, AgentPayload{Repo: repo, Prompt: "hi"}))
	if err == nil {
		t.Fatal("want a gate classification, got nil")
	}

	var gateErr *VerifyGateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("want *VerifyGateError, got %v", err)
	}

	if gateErr.Class != VerifyGateDead {
		t.Fatalf("class = %s, want %s", gateErr.Class, VerifyGateDead)
	}

	if !strings.Contains(err.Error(), "gate dead") {
		t.Fatalf("error should name the classification, got %v", err)
	}
}

// TestVerifyGateIntroducedStillCounts pins the complement: the baseline
// passes and only the attempt's own work breaks the gate — the plain
// verify-failed error must survive (this shape is the ONLY one that counts
// against the task).
func TestVerifyGateIntroducedStillCounts(t *testing.T) {
	repo := t.TempDir()
	setupGitRepo(t, repo)

	if err := os.WriteFile(filepath.Join(repo, ".tq-verify"), []byte("test ! -f broken.marker"), 0o644); err != nil {
		t.Fatal(err)
	}

	commitAll(t, repo, "pin healthy gate")

	e := &AgentExecutor{Bin: makeStubAgent(t, "echo broken > broken.marker")}

	err := e.Execute(context.Background(), agentTaskT(t, AgentPayload{Repo: repo, Prompt: "hi"}))
	if err == nil || !strings.Contains(err.Error(), "verify failed") {
		t.Fatalf("want plain verify failure, got %v", err)
	}

	var gateErr *VerifyGateError
	if errors.As(err, &gateErr) {
		t.Fatalf("introduced failure must NOT classify as a gate error, got %v", err)
	}
}

// TestVerifyGateSlowOnDeadline pins classification shape (2): the deadline
// kills the gate before a verdict — no baseline probe, straight to
// gate-slow (retry on a warm cache, no attempt burn).
func TestVerifyGateSlowOnDeadline(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, ".tq-verify"), []byte("sleep 30"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	_, err := runVerify(ctx, task.ID("verify-gate-slow-test"), dir, &AgentPayload{Verify: "sleep 30"}, false, "")
	if err == nil {
		t.Fatal("want gate-slow, got nil")
	}

	var gateErr *VerifyGateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("want *VerifyGateError, got %v", err)
	}

	if gateErr.Class != VerifyGateSlow {
		t.Fatalf("class = %s, want %s", gateErr.Class, VerifyGateSlow)
	}
}

// TestVerifyGateCooperativeCancelStaysCancelled pins the guard: a worker
// shutdown (context.Canceled) must keep the existing cancelled error — it
// is neither gate-dead nor gate-slow and must not trigger a baseline probe.
func TestVerifyGateCooperativeCancelStaysCancelled(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, ".tq-verify"), []byte("sleep 30"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := runVerify(ctx, task.ID("verify-gate-cancel-test"), dir, &AgentPayload{Verify: "sleep 30"}, false, "")
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("want cancelled error, got %v", err)
	}

	var gateErr *VerifyGateError
	if errors.As(err, &gateErr) {
		t.Fatalf("cooperative cancel must not classify as gate failure, got %v", err)
	}
}
