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

	if _, ok := errors.AsType[*VerifyGateError](err); ok {
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

	_, _, err := runVerify(ctx, task.ID("verify-gate-slow-test"), dir, &AgentPayload{Verify: "sleep 30"}, false, "")
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

	_, _, err := runVerify(ctx, task.ID("verify-gate-cancel-test"), dir, &AgentPayload{Verify: "sleep 30"}, false, "")
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("want cancelled error, got %v", err)
	}

	if _, ok := errors.AsType[*VerifyGateError](err); ok {
		t.Fatalf("cooperative cancel must not classify as gate failure, got %v", err)
	}
}

// gofmtStageVerify is the mint's gofmt stage shape: the command substitution
// is what swallows gofmt's own output from the log (the signature's blind
// tail).
const gofmtStageVerify = `go test ./... && test -z "$(gofmt -l .)"`

// skipWithoutGoToolchain guards the gofmt-signature fixtures: they run a
// real go test + gofmt probe, so they skip where the toolchain is absent
// (same hermeticity practice as the git-dependent tests).
func skipWithoutGoToolchain(t *testing.T) {
	t.Helper()

	for _, tool := range []string{"go", "gofmt"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}
}

// writeVendorRepo builds a committed Go module repo with one passing test,
// a gitignored vendor/ tree holding a non-gofmt file, and (with
// trackedBad) a non-gofmt file at the root: the two halves of the row-135
// class question (vendor-only death vs the agent's own formatting).
func writeVendorRepo(t *testing.T, trackedBad bool) string {
	t.Helper()
	skipWithoutGoToolchain(t)

	repo := t.TempDir()
	setupGitRepo(t, repo)

	write := func(name, content string) {
		t.Helper()

		if err := os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("go.mod", "module demo\n\ngo 1.25\n")
	write("main.go", "package main\n\nfunc main() {}\n")
	write("main_test.go", "package main\n\nimport \"testing\"\n\nfunc TestPass(t *testing.T) {}\n")
	write(".gitignore", "vendor/\n")

	if err := os.MkdirAll(filepath.Join(repo, "vendor"), 0o755); err != nil {
		t.Fatal(err)
	}

	write(filepath.Join("vendor", "bad.go"), "package vendor\n\nfunc  Bad() int {\nreturn 1\n}\n")

	if trackedBad {
		write("rootbad.go", "package main\n\nfunc  RootBad() int {\nreturn 2\n}\n")
	}

	commitAll(t, repo, "vendor repo fixture")

	return repo
}

// TestVendorGofmtSignatureClassifiesEnvironmental pins the row-135
// signature end to end: the minted gofmt-stage gate dies on gitignored
// vendor/ files only (every package stage passed), so the executor must
// classify VerifyGateEnvironmental carrying the vendor-gofmt reason code,
// instead of the plain verify-failed error that burns the attempt.
func TestVendorGofmtSignatureClassifiesEnvironmental(t *testing.T) {
	repo := writeVendorRepo(t, false)

	_, _, err := runVerify(
		context.Background(),
		task.ID("vendor-gofmt-test"),
		repo,
		&AgentPayload{Verify: gofmtStageVerify},
		false,
		"",
	)
	if err == nil {
		t.Fatal("want a verify failure, got nil")
	}

	var gateErr *VerifyGateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("want *VerifyGateError, got %v", err)
	}

	if gateErr.Class != VerifyGateEnvironmental {
		t.Fatalf("class = %s, want %s", gateErr.Class, VerifyGateEnvironmental)
	}

	if !strings.Contains(err.Error(), VerifyGateEnvCode) {
		t.Fatalf("error should carry the %s reason code, got %v", VerifyGateEnvCode, err)
	}
}

// TestVendorGofmtSignatureNeverMasksIntroducedWork pins the guard: when the
// gofmt re-measure flags a NON-vendor file (the agent's own formatting),
// the signature must not match and the death must stay unclassified (the
// plain verify-failure path), so real work defects still count.
func TestVendorGofmtSignatureNeverMasksIntroducedWork(t *testing.T) {
	repo := writeVendorRepo(t, true)

	_, _, err := runVerify(
		context.Background(),
		task.ID("vendor-gofmt-introduced-test"),
		repo,
		&AgentPayload{Verify: gofmtStageVerify},
		false,
		"",
	)
	if err == nil {
		t.Fatal("want a verify failure, got nil")
	}

	var gateErr *VerifyGateError
	if errors.As(err, &gateErr) && gateErr.Class == VerifyGateEnvironmental {
		t.Fatalf("introduced formatting must not classify environmental, got %v", err)
	}
}

// TestVendorGofmtSignatureClauses pins the cheap clauses of the signature
// matcher individually: no gofmt stage, no vendor dir, a failing or empty
// tail must each refuse the match.
func TestVendorGofmtSignatureClauses(t *testing.T) {
	repo := writeVendorRepo(t, false)
	ctx := context.Background()
	okTail := "ok  \tdemo\t0.01s\n"

	if !vendorGofmtSignature(ctx, gofmtStageVerify, repo, okTail) {
		t.Fatal("vendor-only gofmt death must match the signature")
	}

	if vendorGofmtSignature(ctx, gofmtStageVerify, repo, "FAIL\tdemo\t0.01s\n") {
		t.Fatal("a FAIL tail must not match the signature")
	}

	if vendorGofmtSignature(ctx, "go test ./...", repo, okTail) {
		t.Fatal("a gate without a gofmt stage must not match the signature")
	}

	if vendorGofmtSignature(ctx, gofmtStageVerify, repo, "") {
		t.Fatal("an empty tail (no ok lines) must not match the signature")
	}

	if vendorGofmtSignature(ctx, gofmtStageVerify, t.TempDir(), okTail) {
		t.Fatal("a repo without vendor/ must not match the signature")
	}
}

// TestAllPackagesPassed pins the tail reading: an ok package line passes,
// any FAIL anywhere in the window fails, and a window without ok lines
// (zero tests, foreign output) carries no pass evidence.
func TestAllPackagesPassed(t *testing.T) {
	cases := []struct {
		name string
		tail string
		want bool
	}{
		{"ok line", "ok  \tdemo\t0.01s\n", true},
		{"fail anywhere", "ok  \tdemo\t0.01s\n--- FAIL: TestX (0.00s)\nFAIL\n", false},
		{"no ok lines", "?\t\tdemo\t[no test files]\n", false},
		{"empty", "", false},
	}

	for _, tc := range cases {
		if got := allPackagesPassed(tc.tail); got != tc.want {
			t.Errorf("%s: allPackagesPassed = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestVerifyDeathStage pins the row-144 stage table end to end: each death
// shape maps onto its VerifyStage* class from the FULL output, judged at the
// tail cut — timeout kills outrank FAIL lines, FAIL means the test stage,
// the all-ok tail (or bare .go paths) in a gofmt gate is the gofmt witness,
// and everything else stays run.
func TestVerifyDeathStage(t *testing.T) {
	gofmtGate := gofmtStageVerify

	cases := []struct {
		name string
		err  error
		ctx  func() context.Context
		out  string
		want string
	}{
		{"task deadline", context.DeadlineExceeded, context.Background, "some output", VerifyStageE2ETimeout},
		{
			"go-test watchdog panic", nil, context.Background,
			"panic: test timed out after 3m0s\nrunning tests:", VerifyStageE2ETimeout,
		},
		{
			"test failure beats gofmt stage", nil, context.Background,
			"ok  \tdemo\t0.01s\n--- FAIL: TestX (0.00s)\nFAIL\n", VerifyStageTest,
		},
		{
			"package FAIL", nil, context.Background,
			"FAIL\tdemo\t0.01s\n", VerifyStageTest,
		},
		{
			"all-ok tail is the gofmt witness", nil, context.Background,
			"ok  \tdemo\t0.01s\n", VerifyStageGofmt,
		},
		{
			"bare gofmt path lines", nil, context.Background,
			"main.go\nutil.go\n", VerifyStageGofmt,
		},
		{"no gate, no marker", nil, context.Background, "sh: 1: nix: not found\n", VerifyStageRun},
		{
			"gofmt path only in gofmt gates", nil, context.Background,
			"main.go\n", VerifyStageGofmt,
		},
	}

	for _, tc := range cases {
		ctx := tc.ctx()
		if got := verifyDeathStage(ctx, tc.err, gofmtGate, []byte(tc.out)); got != tc.want {
			t.Errorf("%s: verifyDeathStage = %s, want %s", tc.name, got, tc.want)
		}
	}

	// A gate WITHOUT a gofmt stage must never stamp gofmt, even on an
	// all-ok-shaped tail (the stage name would be a lie).
	if got := verifyDeathStage(
		context.Background(),
		nil,
		"go test ./...",
		[]byte("ok  \tdemo\t0.01s\n"),
	); got != VerifyStageRun {
		t.Errorf("gofmt stage in a gofmt-free gate = %s, want %s", got, VerifyStageRun)
	}
}

func TestIsGateArtifactDeath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		evidence  FailureEvidence
		lastError string
		want      bool
	}{
		{
			name:      "stamped environmental code in last error",
			evidence:  FailureEvidence{Stage: "verify", VerifyStage: VerifyStageGofmt, Tail: "ok  \tdemo\t0.01s\n"},
			lastError: `agent verify gate environmental signature [vendor-gofmt] ("..."): boom`,
			want:      true,
		},
		{
			name: "legacy all-ok tail with gofmt stage in error",
			evidence: FailureEvidence{
				Stage:    "verify",
				ExitCode: 1,
				Tail:     "ok  \tdemo\t0.01s\nok  \tinternal/queue\t0.4s\n",
			},
			lastError: `agent verify failed ("go test ./... && test -z "$(gofmt -l .)""): exit 1: ok  demo`,
			want:      true,
		},
		{
			name:      "stamped gofmt without the environmental code is a real defect",
			evidence:  FailureEvidence{Stage: "verify", VerifyStage: VerifyStageGofmt, Tail: "main.go\n"},
			lastError: `agent verify failed ("gofmt -l ."): exit 1: main.go`,
			want:      false,
		},
		{
			name:      "test failure marker disqualifies",
			evidence:  FailureEvidence{Stage: "verify", Tail: "ok  \tdemo\t0.01s\n--- FAIL: TestX\nFAIL\n"},
			lastError: `agent verify failed ("go test ./... && gofmt -l ."): exit 1`,
			want:      false,
		},
		{
			name:      "empty evidence is never the artifact",
			evidence:  FailureEvidence{},
			lastError: `agent verify failed ("gofmt -l ."): exit 1`,
			want:      false,
		},
		{
			name:      "non-verify stage",
			evidence:  FailureEvidence{Stage: "agent", Tail: "ok  \tdemo\t0.01s\n"},
			lastError: "gofmt -l .",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := IsGateArtifactDeath(tt.evidence, tt.lastError); got != tt.want {
				t.Errorf("IsGateArtifactDeath(%+v, %q) = %v, want %v", tt.evidence, tt.lastError, got, tt.want)
			}
		})
	}
}
