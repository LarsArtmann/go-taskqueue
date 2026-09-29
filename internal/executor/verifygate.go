package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
//   - VerifyGateEnvironmental: the failure matches a BASELINED environmental
//     signature (the vendor-gofmt class: every package stage passed and the
//     gofmt stage flags only gitignored vendor/ files). The identical retry
//     dies identically and each re-dispatch re-does finished work, so the
//     worker DEAD-LETTERS immediately instead of riding the requeue ladder.
//
// The first two classes requeue WITHOUT burning an attempt (the worker
// treats them like a PreflightError); the environmental class dead-letters
// on the first death with the VerifyGateEnvCode reason. Only an introduced
// failure — the gate passing at the pre-attempt rev and failing after the
// agent's work — counts against the task.
type VerifyGateClass string

const (
	VerifyGateDead VerifyGateClass = "gate-dead"
	VerifyGateSlow VerifyGateClass = "gate-slow"

	// VerifyGateEnvironmental marks a proven-environmental verify death
	// (the row-135 vendor-gofmt class). See vendorGofmtSignature.
	VerifyGateEnvironmental VerifyGateClass = "gate-env"
)

// VerifyGateEnvCode is the machine-greppable reason code carried by every
// VerifyGateEnvironmental error text, so DLQ triage and rescue sweeps can
// classify the death as environmental without re-deriving it from evidence
// tails.
const VerifyGateEnvCode = "vendor-gofmt"

// Verify-gate death classes stamped into FailureEvidence.VerifyStage, so a
// verify death's failing stage is queryable data instead of log forensics
// (row 144: 62 of the 93 gofmt-vendor deaths had their sidecar logs
// rotated, leaving the all-ok tail as the only witness, and one e2e kill
// was misread as gofmt until a docs cross-check).
const (
	VerifyStageGofmt      = "gofmt"       // the gate's gofmt stage died
	VerifyStageTest       = "test"        // a go-test stage failed
	VerifyStageE2ETimeout = "e2e-timeout" // a timeout killed the run (go-test watchdog/panic or the task deadline)
	VerifyStageRun        = "run"         // the run as a whole: spawn failure, pre-test stage, foreign tooling
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
		return fmt.Sprintf(
			"agent verify gate dead (%q): failure is pre-existing/environmental (gate also fails at the pre-attempt rev): %s",
			e.Verify,
			e.Cause,
		)
	case VerifyGateSlow:
		return fmt.Sprintf(
			"agent verify gate slow (%q): deadline before a verdict, retry on a warm cache: %s",
			e.Verify,
			e.Cause,
		)
	case VerifyGateEnvironmental:
		return fmt.Sprintf(
			"agent verify gate environmental signature [%s] (%q): all package stages passed and gofmt flags only gitignored vendor/ files; the identical retry dies identically so the task is dead-lettered without re-dispatch (rescue: classify environmental): %s",
			VerifyGateEnvCode,
			e.Verify,
			e.Cause,
		)
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

	// Baselined environmental signature (the vendor-gofmt class) BEFORE the
	// baseline probe: the probe is structurally blind to this failure
	// (gitignored vendor/ never enters a fresh worktree, so the gate passes
	// there and the death misclassifies as introduced). The signature is
	// definitive, so classify first and skip the probe entirely.
	if vendorGofmtSignature(ctx, verify, repoDir, tail) {
		return &VerifyGateError{Class: VerifyGateEnvironmental, Verify: verify, Cause: gateErr, Tail: tail}
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

// vendorGofmtSignature reports whether a failed verify run matches the
// row-135 environmental signature (the gofmt-vendor dead-letter class): the
// gate carries a gofmt stage of the minted `test -z "$(gofmt -l .)"` shape,
// the output tail shows the Go package stages all passed (go-test ok lines,
// no FAIL anywhere in the window), and a direct gofmt re-measure flags
// files ONLY under the repo's vendor/ tree. The baseline worktree probe is
// structurally blind to this failure (vendor/ is gitignored, so it never
// enters a fresh worktree and the gate passes there), which is exactly why
// the signature must be measured against the LIVE tree. Any clause that
// cannot hold (no gofmt stage, no vendor dir, a flagged non-vendor file,
// probe failure) returns false so the existing classification still runs.
func vendorGofmtSignature(ctx context.Context, verify, repoDir, tail string) bool {
	if !strings.Contains(verify, "gofmt -l") {
		return false
	}

	if !allPackagesPassed(tail) {
		return false
	}

	if info, err := os.Stat(filepath.Join(repoDir, "vendor")); err != nil || !info.IsDir() {
		return false
	}

	flagged, err := gofmtFlagged(ctx, repoDir)
	if err != nil || len(flagged) == 0 {
		return false
	}

	for _, f := range flagged {
		if !strings.HasPrefix(filepath.ToSlash(f), "vendor/") {
			return false
		}
	}

	return true
}

// allPackagesPassed reports whether the verify output tail shows go-test
// progress and no failure: at least one `ok` package line and no FAIL
// anywhere in the window (the mint's command substitution swallows the
// gofmt stage's own output, so the tail ends on the last passing package).
func allPackagesPassed(tail string) bool {
	if strings.Contains(tail, "FAIL") {
		return false
	}

	for line := range strings.SplitSeq(tail, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "ok" {
			return true
		}
	}

	return false
}

// verifyTestFailRe matches go-test failure markers at line start: the
// per-test "--- FAIL: X" line and the per-package/suite "FAIL" lines.
var verifyTestFailRe = regexp.MustCompile(`(?m)^--- FAIL: |^FAIL\b`)

// gofmtFlaggedPathRe matches a bare .go path line — the exact shape
// `gofmt -l` prints. Every other stage annotates its paths (build/vet use
// file:line:col, panic frames are tab-indented), so a bare path is gofmt
// output that survived into the buffer (gates not using $(...) capture).
var gofmtFlaggedPathRe = regexp.MustCompile(`(?m)^\S+\.go$`)

// verifyDeathStage names the failing verify-gate stage from the run's FULL
// output and error — judged at the tail cut, where the whole buffer is
// still in hand, because the 512-byte evidence tail and later sidecar-log
// rotation are exactly what kept the death class unrecoverable (row 144).
// Priority mirrors a sequential &&-gate:
//
//  1. timeout-kill evidence (the go-test panic/watchdog lines, or the task
//     deadline) — a kill also prints FAIL lines, so it outranks them;
//  2. go-test failure markers — a &&-gate stops at its first failing
//     stage, so FAIL means the test stage died;
//  3. a gofmt-stage death, recognized either positively (bare .go path
//     lines in the buffer) or negatively (every package ok and no failure
//     marker anywhere: the minted `test -z "$(gofmt -l .)"` stage SWALLOWS
//     its own output, so a death that got past test and left no marker died
//     in gofmt — the all-ok tail IS the witness);
//  4. anything else — the run as a whole.
//
// Best effort by design: gates with stages after gofmt, or a swallowed
// gofmt stage in a gate without any passing package line, stamp as run
// rather than guess.
func verifyDeathStage(ctx context.Context, err error, verify string, output []byte) string {
	out := string(output)

	if isDeadline(err) || isDeadline(ctx.Err()) ||
		strings.Contains(out, "panic: test timed out after") ||
		strings.Contains(out, "Test killed with quit: ran too long") {
		return VerifyStageE2ETimeout
	}

	if verifyTestFailRe.MatchString(out) {
		return VerifyStageTest
	}

	if strings.Contains(verify, "gofmt") &&
		(allPackagesPassed(out) || gofmtFlaggedPathRe.MatchString(out)) {
		return VerifyStageGofmt
	}

	return VerifyStageRun
}

// verifyProbeTimeout bounds the gofmt re-measure: a parse-only walk, seconds
// even on large trees.
const verifyProbeTimeout = 2 * time.Minute

// gofmtFlagged runs `gofmt -l .` in repoDir (the same walk the minted gofmt
// stage performs, vendor/ included, which is the defect this signature
// classifies) and returns the flagged paths.
func gofmtFlagged(ctx context.Context, repoDir string) ([]string, error) {
	probeCtx, cancel := context.WithTimeout(ctx, verifyProbeTimeout)
	defer cancel()

	cmd := exec.CommandContext(probeCtx, "gofmt", "-l", ".")
	cmd.Dir = repoDir

	var buf bytes.Buffer

	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("gofmt probe: %w", err)
	}

	var paths []string

	for line := range strings.SplitSeq(buf.String(), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}

	return paths, nil
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
