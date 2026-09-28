package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// VerifyGateClass says WHY a verify gate failed without judging the task:
//
//   - VerifyGateDead: the gate fails at the PRE-attempt rev too, so the
//     failure is pre-existing or environmental (boot-fragile host
//     precondition, broken baseline) — retrying the task changes nothing.
//   - VerifyGateSlow: the deadline killed the gate before a verdict
//     (context deadline exceeded, no gate output judgement) — a retry on a
//     warm cache may simply pass.
//
// Both classes requeue WITHOUT burning an attempt (the worker treats them
// like a PreflightError); only an introduced failure — the gate passing at
// the pre-attempt rev and failing after the agent's work — counts against
// the task.
type VerifyGateClass string

const (
	VerifyGateDead VerifyGateClass = "gate-dead"
	VerifyGateSlow VerifyGateClass = "gate-slow"
)

// VerifyGateError carries a gate-level (not task-level) verify failure.
type VerifyGateError struct {
	Class  VerifyGateClass
	Verify string
	Cause  error
	Tail   string
}

func (e *VerifyGateError) Error() string {
	switch e.Class {
	case VerifyGateDead:
		return fmt.Sprintf("agent verify gate dead (%q): failure is pre-existing/environmental (gate also fails at the pre-attempt rev): %s", e.Verify, e.Cause)
	case VerifyGateSlow:
		return fmt.Sprintf("agent verify gate slow (%q): deadline before a verdict, retry on a warm cache: %s", e.Verify, e.Cause)
	default:
		return fmt.Sprintf("agent verify gate %s (%q): %s", e.Class, e.Verify, e.Cause)
	}
}

func (e *VerifyGateError) Unwrap() error { return e.Cause }

// classifyVerifyFailure maps a failed verify run onto a VerifyGateError when
// the failure belongs to the gate or the environment rather than the task.
// It returns nil when the failure is introduced (counts against the task)
// or when it cannot be classified (missing baseline, probe infrastructure
// error) — the caller then keeps the plain verify-failure error.
func classifyVerifyFailure(ctx context.Context, verify, repoDir, baseRev string, gateErr error, tail string) error {
	// A cooperative cancel (worker shutdown, operator cancel) is neither
	// gate-dead nor gate-slow — it must keep the existing cancelled path
	// (and must NOT trigger a 30-min baseline probe).
	if ctx.Err() != nil && errors.Is(ctx.Err(), context.Canceled) {
		return nil
	}

	if isDeadline(gateErr) || isDeadline(ctx.Err()) {
		return &VerifyGateError{Class: VerifyGateSlow, Verify: verify, Cause: gateErr, Tail: tail}
	}

	if baseRev == "" {
		return nil
	}

	class, classified, probeErr := baselineVerify(verify, repoDir, baseRev)
	if probeErr != nil || !classified {
		// Probe infrastructure failed or the baseline passed: the failure
		// is introduced — never mask the real verify error with a probe
		// problem.
		return nil
	}

	return &VerifyGateError{Class: class, Verify: verify, Cause: gateErr, Tail: tail}
}

// isDeadline reports whether err is (or wraps) context.DeadlineExceeded.
func isDeadline(err error) bool {
	return err != nil && errors.Is(err, context.DeadlineExceeded)
}

// baselineVerify re-runs the same verify command in a throwaway worktree at
// baseRev (the rev BEFORE the attempt's work). Verdict:
//
//   - (VerifyGateDead, true, nil): the baseline fails the gate too —
//     pre-existing/environmental.
//   - (VerifyGateSlow, true, nil): the baseline probe itself hit the probe
//     deadline — the gate cannot produce a verdict anywhere; treat as
//     environmental rather than burning the attempt.
//   - ("", false, nil): the baseline passes — the failure is introduced.
//   - ("", false, err): the probe could not run (worktree creation failed);
//     the caller falls back to the plain verify failure.
func baselineVerify(verify, repoDir, baseRev string) (VerifyGateClass, bool, error) {
	worktree := filepath.Join(os.TempDir(), fmt.Sprintf("tq-verify-baseline-%d", time.Now().UnixNano()))

	if out, err := gitRun(repoDir, "worktree", "add", "--detach", worktree, baseRev); err != nil {
		return "", false, fmt.Errorf("baseline worktree: %w: %s", err, tailOutput(out))
	}

	defer func() {
		_, _ = gitRun(repoDir, "worktree", "remove", "--force", worktree)
		_ = os.RemoveAll(worktree)
	}()

	probeCtx, cancel := context.WithTimeout(context.Background(), defaultAgentTaskTimeout)
	defer cancel()

	cmd := exec.CommandContext(probeCtx, "sh", "-c", verify)
	cmd.Dir = worktree

	var buf bytes.Buffer

	cmd.Stdout = &buf
	cmd.Stderr = &buf
	prepareProcessGroup(cmd)
	cmd.WaitDelay = 10 * time.Second

	if err := cmd.Run(); err != nil {
		if isDeadline(err) || isDeadline(probeCtx.Err()) {
			return VerifyGateSlow, true, nil
		}

		return VerifyGateDead, true, nil
	}

	return "", false, nil
}

// gitRun runs git in dir and returns trimmed stdout+stderr.
func gitRun(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	var buf bytes.Buffer

	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(buf.String()), err
	}

	return strings.TrimSpace(buf.String()), nil
}

// gitHeadRev returns the current HEAD rev of the git repo at dir, or ""
// when dir is not a git repo or HEAD is unborn — the pre-attempt baseline
// probe is skipped in that case.
func gitHeadRev(dir string) string {
	rev, err := gitRun(dir, "rev-parse", "HEAD")
	if err != nil || rev == "" {
		return ""
	}

	return rev
}
