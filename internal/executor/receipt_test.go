//go:build unix

package executor

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// The verify receipt closes one class of paid-session waste: a closeout report
// used to hand-derive its "N ok / M fail" line with `grep -c '^ok:'` over a
// captured gate log, in context, inside a session that is billed per token.
// Two windows (02-11, 04-55) miscounted that way and had to re-issue a
// corrected receipt. The tests below pin the receipt against the grep it
// replaces, so "the emitted number" and "the number a reader would count"
// cannot diverge.
//
// Source: docs/research/2026-11-01_okf-lessons.md lesson 10; OKF v0.2 SPEC
// §10.2 (declared receipt fields) and §10.5 (deterministic attester).

// grepOKCount is the closeout convention verbatim: `grep -c '^ok:'` over a
// captured gate-output file. Used as the oracle the receipt must agree with.
func grepOKCount(t *testing.T, output string) int {
	t.Helper()

	path := filepath.Join(t.TempDir(), "gate.log")

	if err := os.WriteFile(path, []byte(output), 0o600); err != nil {
		t.Fatalf("write gate log: %v", err)
	}

	cmd := exec.Command("grep", "-c", "^ok:", path)

	raw, _ := cmd.Output() // grep exits 1 on zero matches but still prints "0"

	n, convErr := strconv.Atoi(strings.TrimSpace(string(raw)))
	if convErr != nil {
		t.Fatalf("grep -c returned %q: %v", raw, convErr)
	}

	return n
}

func TestParseVerifyCountsMatchesCloseoutGrep(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		output   string
		wantOK   int
		wantFail int
	}{
		{
			name:   "clean battery",
			output: "ok: build\nok: vet\nok: test\n",
			wantOK: 3,
		},
		{
			name:     "one failing gate",
			output:   "ok: build\nFAIL: vet reports 2 issues\nok: docs\n",
			wantOK:   2,
			wantFail: 1,
		},
		{
			name:   "indented ok does not count",
			output: "  ok: nested sub-check\nok: real\n",
			wantOK: 1,
		},
		{
			name:     "failures from a go test run are not FAIL-prefixed",
			output:   "ok: build\n--- FAIL: TestThing (0.01s)\n",
			wantOK:   1,
			wantFail: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			okCount, failCount := ParseVerifyCounts(tt.output)
			if okCount != tt.wantOK || failCount != tt.wantFail {
				t.Fatalf("counts = %d ok / %d fail, want %d / %d", okCount, failCount, tt.wantOK, tt.wantFail)
			}

			if want := grepOKCount(t, tt.output); okCount != want {
				t.Fatalf("receipt ok count %d != grep -c '^ok:' %d", okCount, want)
			}
		})
	}
}

// TestRunVerifyReceiptOnSuccess pins the success-path receipt: the command
// that actually ran, rc 0, the HEAD at gate time, and ok/fail tallies that
// agree with the closeout grep.
func TestRunVerifyReceiptOnSuccess(t *testing.T) {
	repo := t.TempDir()
	setupGitRepo(t, repo)

	pinned := `echo "ok: build" && echo "ok: vet"`
	if err := os.WriteFile(filepath.Join(repo, ".tq-verify"), []byte(pinned), 0o644); err != nil {
		t.Fatal(err)
	}

	commitAll(t, repo, "pin gate")

	wantHead := gitHeadRev(repo)
	if wantHead == "" {
		t.Fatal("fixture repo has no HEAD")
	}

	out, err := runVerify(context.Background(), task.ID("receipt-success-test"), repo, &AgentPayload{}, false, "")
	if err != nil {
		t.Fatalf("runVerify: %v", err)
	}

	if out.Receipt == nil {
		t.Fatal("a passing gate must carry a receipt")
	}

	if out.Receipt.Command != pinned {
		t.Fatalf("command = %q, want the executed pin %q", out.Receipt.Command, pinned)
	}

	if out.Receipt.RC != 0 {
		t.Fatalf("rc = %d, want 0", out.Receipt.RC)
	}

	if out.Receipt.Head != wantHead {
		t.Fatalf("head = %q, want %q", out.Receipt.Head, wantHead)
	}

	if out.Receipt.OK != 2 || out.Receipt.Fail != 0 {
		t.Fatalf("counts = %d ok / %d fail, want 2 / 0", out.Receipt.OK, out.Receipt.Fail)
	}

	if want := grepOKCount(t, out.Tail); want != 2 {
		t.Fatalf("fixture is not self-consistent: grep sees %d ok lines", want)
	}
}

// TestRunVerifyReceiptOnFailure pins the failure-path receipt: rc, the death
// class and the counts all come from the same run, so both paths answer the
// same two questions with one shape.
func TestRunVerifyReceiptOnFailure(t *testing.T) {
	repo := t.TempDir()
	setupGitRepo(t, repo)

	pinned := `echo "ok: build" && echo "FAIL: vet died" && exit 3`
	if err := os.WriteFile(filepath.Join(repo, ".tq-verify"), []byte(pinned), 0o644); err != nil {
		t.Fatal(err)
	}

	commitAll(t, repo, "pin failing gate")

	// Sidecar reuse pin (R2.7): the receipt must NOT mint a second artifact.
	// A failing gate writes exactly the one verify-sidecar file it always did.
	logDir := t.TempDir()
	t.Setenv("TQ_LOG_DIR", logDir)

	id := task.ID("receipt-failure-test")

	out, err := runVerify(context.Background(), id, repo, &AgentPayload{}, false, "")
	if err == nil {
		t.Fatal("want a verify failure, got nil")
	}

	if out.Receipt == nil {
		t.Fatal("a failing gate must carry a receipt too")
	}

	if out.Receipt.RC != 3 {
		t.Fatalf("rc = %d, want 3", out.Receipt.RC)
	}

	if out.Receipt.OK != 1 || out.Receipt.Fail != 1 {
		t.Fatalf("counts = %d ok / %d fail, want 1 / 1", out.Receipt.OK, out.Receipt.Fail)
	}

	if out.Stage == "" {
		t.Fatal("failure outcome must carry the death class")
	}

	entries, readErr := os.ReadDir(logDir)
	if readErr != nil {
		t.Fatalf("read sidecar dir: %v", readErr)
	}

	if len(entries) != 1 || entries[0].Name() != id.String()+".verify-failure.log" {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}

		t.Fatalf("sidecar dir = %v, want exactly %s.verify-failure.log", names, id)
	}
}

// TestVerifyReceiptAbsentIsUnverified pins the additive contract: a detail
// written by a binary that predates the receipt still decodes, and the absent
// receipt attests as UNVERIFIED rather than invalid.
func TestVerifyReceiptAbsentIsUnverified(t *testing.T) {
	t.Parallel()

	legacy := `{"verify_tail":"ok: build","commit_sha":"abc1234"}`

	var result AgentResult
	if err := json.Unmarshal([]byte(legacy), &result); err != nil {
		t.Fatalf("legacy detail must still decode: %v", err)
	}

	if result.Verify != nil {
		t.Fatalf("legacy detail invented a receipt: %+v", result.Verify)
	}

	verdict := AttestVerify(AttestationInput{Receipt: result.Verify, RepoVerify: "go build ./..."})
	if verdict.OK || verdict.Refusal != AttestationNoReceipt {
		t.Fatalf("verdict = %+v, want refusal %s", verdict, AttestationNoReceipt)
	}
}

// TestAgentResultReceiptRoundTrip pins the wire shape and keeps the additive
// half of the payload contract: a receipt survives a marshal/unmarshal, and an
// unknown sibling key is ignored (a NEWER writer must not break this reader).
func TestAgentResultReceiptRoundTrip(t *testing.T) {
	t.Parallel()

	want := AgentResult{Verify: &VerifyReceipt{
		Command: "go build ./...",
		RC:      0,
		Head:    "0123456789abcdef",
		OK:      7,
		Fail:    1,
	}}

	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got AgentResult
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Verify == nil || *got.Verify != *want.Verify {
		t.Fatalf("receipt round trip = %+v, want %+v", got.Verify, want.Verify)
	}

	mutated := jsontext.Value(strings.TrimSuffix(string(raw), "}") + `,"future_receipt_field":42}`)

	var tolerant AgentResult
	if err := json.Unmarshal(mutated, &tolerant); err != nil {
		t.Fatalf("unknown receipt sibling key must be ignored: %v", err)
	}

	if tolerant.Verify == nil || tolerant.Verify.OK != 7 {
		t.Fatalf("tolerant decode lost the receipt: %+v", tolerant.Verify)
	}
}

// TestAttestVerify pins the deterministic attester: three comparisons, one
// verdict, no model in the loop. Green cases first, then one red case per
// comparison — plus the counterfactual that keeps the comparison non-vacuous.
func TestAttestVerify(t *testing.T) {
	t.Parallel()

	const (
		head     = "0123456789abcdef0123456789abcdef01234567"
		shortHed = "0123456"
		command  = "go build ./... && go vet ./..."
		output   = "ok: build\nok: vet\n"
	)

	goodReceipt := func() *VerifyReceipt {
		return &VerifyReceipt{Command: command, RC: 0, Head: head, OK: 2}
	}

	tests := []struct {
		name        string
		in          AttestationInput
		wantOK      bool
		wantRefusal string
	}{
		{
			name: "all three comparisons pass",
			in: AttestationInput{
				Receipt: goodReceipt(), RepoVerify: command, ClaimedHead: head, GateOutput: output,
			},
			wantOK: true,
		},
		{
			name: "abbreviated claimed head matches",
			in: AttestationInput{
				Receipt: goodReceipt(), PayloadVerify: command, ClaimedHead: shortHed,
			},
			wantOK: true,
		},
		{
			name: "no pinned declaration skips the command comparison",
			in: AttestationInput{
				Receipt: goodReceipt(), ClaimedHead: head,
			},
			wantOK: true,
		},
		{
			name: "head mismatch",
			in: AttestationInput{
				Receipt: goodReceipt(), RepoVerify: command,
				ClaimedHead: "fedcba9876543210fedcba9876543210fedcba98",
			},
			wantRefusal: AttestationHeadMismatch,
		},
		{
			name: "executed command is not the pinned one",
			in: AttestationInput{
				Receipt: goodReceipt(), RepoVerify: "go test ./...",
			},
			wantRefusal: AttestationCommandMismatch,
		},
		{
			name: "recorded counts disagree with the captured output",
			in: AttestationInput{
				Receipt: goodReceipt(), RepoVerify: command, GateOutput: "ok: build\n",
			},
			wantRefusal: AttestationCountMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			verdict := AttestVerify(tt.in)
			if verdict.OK != tt.wantOK {
				t.Fatalf("OK = %v (%s), want %v", verdict.OK, verdict, tt.wantOK)
			}

			if verdict.Refusal != tt.wantRefusal {
				t.Fatalf("refusal = %q, want %q (%s)", verdict.Refusal, tt.wantRefusal, verdict.Detail)
			}
		})
	}
}

// TestAttestVerifyWhitespaceToleranceIsNotVacuous pins both halves of the
// normalisation rule: re-rendered whitespace agrees (a .tq-verify file may wrap
// its `&&` chain across lines), a REWRITTEN command does not. Without the
// second half the comparison could be satisfied by anything, which is the
// judgement creep the attester exists to avoid.
func TestAttestVerifyWhitespaceToleranceIsNotVacuous(t *testing.T) {
	t.Parallel()

	const command = "go build ./... && go vet ./..."

	wrapped := "go build ./...\n\t&&  go vet ./..."

	receipt := &VerifyReceipt{Command: command, RC: 0, OK: 1}

	if verdict := AttestVerify(AttestationInput{Receipt: receipt, RepoVerify: wrapped}); !verdict.OK {
		t.Fatalf("wrapped declaration must still match: %s", verdict)
	}

	rewritten := &VerifyReceipt{Command: "true && " + command, RC: 0, OK: 1}

	verdict := AttestVerify(AttestationInput{Receipt: rewritten, RepoVerify: command})
	if verdict.OK || verdict.Refusal != AttestationCommandMismatch {
		t.Fatalf("a rewritten command must refuse, got %s", verdict)
	}
}

// TestAttestVerifyReportsSkippedComparisons pins the honest-reporting half: a
// comparison that could not run is absent from Checked instead of reading as a
// silent pass.
func TestAttestVerifyReportsSkippedComparisons(t *testing.T) {
	t.Parallel()

	verdict := AttestVerify(AttestationInput{
		Receipt: &VerifyReceipt{Command: "go build ./...", RC: 0, OK: 1},
	})
	if !verdict.OK {
		t.Fatalf("verdict = %s, want ok", verdict)
	}

	if len(verdict.Checked) != 1 || verdict.Checked[0] != AttestationCheckReceipt {
		t.Fatalf("checked = %v, want only %q", verdict.Checked, AttestationCheckReceipt)
	}

	if got := verdict.String(); !strings.Contains(got, "ok") {
		t.Fatalf("String() = %q", got)
	}
}
