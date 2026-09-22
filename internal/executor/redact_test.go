package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// Fake but shape-valid token samples: every class the pattern table covers.
// None is a real credential.
const (
	fakeAnthropic = "sk-ant-api03-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
	fakeOpenAI    = "sk-proj-abcdefghijklmnopqrstuvwx"
	fakeOpenAICls = "sk-abcdefghijklmnopqrstuvwxyz012345"
	fakeGitHub    = "ghp_abcdefghijklmnopqrstuvwxyz0123456789"
	fakeGitHubPat = "github_pat_abcdefghijklmnopqrstuv"
	fakeAWS       = "AKIAIOSFODNN7EXAMPLE"
	fakeGoogle    = "AIzaSyA0000000000000000000000000000000000"
	fakeSlack     = "xox" + "b-123456789012-abcdefghijklmnop"
	fakeJWT       = "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
	fakeAssign    = "api_key = a1b2c3d4e5f6g7h8i9j0"
)

func TestRedactSecretsMasksProviderTokens(t *testing.T) {
	t.Parallel()

	samples := map[string]string{
		"anthropic":      fakeAnthropic,
		"openai-proj":    fakeOpenAI,
		"openai-classic": fakeOpenAICls,
		"github":         fakeGitHub,
		"github-pat":     fakeGitHubPat,
		"aws":            fakeAWS,
		"google":         fakeGoogle,
		"slack":          fakeSlack,
		"bearer":         fakeJWT,
		"assignment":     fakeAssign,
	}

	for name, secret := range samples {
		got := RedactSecrets("failed with: " + secret + " — retrying")

		if strings.Contains(got, secret) {
			t.Errorf("%s: secret survived redaction: %s", name, got)
		}

		if !strings.Contains(got, RedactMarker) {
			t.Errorf("%s: no %s in output: %s", name, RedactMarker, got)
		}
	}
}

func TestRedactSecretsKeepsBenignOutput(t *testing.T) {
	t.Parallel()

	benign := []string{
		"go test ./... -race",                           // plain command output
		"task-skills-configuration-handler failed",      // sk- prose false positive
		"password: (none)",                              // short assignment value
		"3 tasks succeeded",                             // ordinary log line
		"token count: 128",                              // free-form 'token' word survives
		"asking for bearer token required for endpoint", // bearer prose, no key
		"sk- short prefix",                              // sk- without a key body
		"AKIA too short",                                // AWS prefix without key body
		"exit status 1: build failed in pkg/api/key.go", // path containing api/key
		"", // empty stays empty
	}

	for _, line := range benign {
		if got := RedactSecrets(line); got != line {
			t.Errorf("benign line mangled:\n in: %q\nout: %q", line, got)
		}
	}
}

func TestSecretHitsCountsMatches(t *testing.T) {
	t.Parallel()

	output := "line one\n" + fakeGitHub + "\nline two\n" + fakeAnthropic + " and " + fakeAnthropic

	if got := SecretHits(output); got != 3 {
		t.Errorf("SecretHits = %d, want 3", got)
	}

	if got := SecretHits("clean output"); got != 0 {
		t.Errorf("SecretHits = %d, want 0", got)
	}
}

// TestSecretPatternsOverlapCensus pins the span-merge contract from the
// other side (02-37 §b4/§f1): on each pattern's canonical sample, exactly
// ONE unordered pair of patterns overlaps — bearer × authorization-header
// (one "Authorization: Bearer …" line). A future pattern that overlaps an
// existing sample shape lands here as a second pair and fails loudly,
// forcing a span-merge/redaction-order review instead of a silent
// double-redact. Extend the sample table in the same change as
// secretPatterns; shapes absent from the table stay unpinned.
func TestSecretPatternsOverlapCensus(t *testing.T) {
	t.Parallel()

	samples := map[int]string{
		0:  fakeAnthropic,
		1:  fakeOpenAI,
		2:  fakeOpenAICls,
		3:  fakeGitHub,
		4:  fakeGitHubPat,
		5:  fakeAWS,
		6:  fakeGoogle,
		7:  fakeSlack,
		8:  fakeJWT,
		9:  "Authorization: Bearer " + fakeJWT[len("Bearer "):],
		10: fakeAssign,
	}

	if len(samples) != len(secretPatterns) {
		t.Fatalf("overlap census covers %d of %d secretPatterns — extend samples together with the table",
			len(samples), len(secretPatterns))
	}

	var overlapping []string
	for i, sample := range samples {
		own := secretPatterns[i].FindAllStringIndex(sample, -1)
		if len(own) != 1 {
			t.Fatalf("sample %d matches its own pattern %d times, want 1: %q", i, len(own), sample)
		}

		for j, re := range secretPatterns {
			if j == i {
				continue
			}

			for _, loc := range re.FindAllStringIndex(sample, -1) {
				if loc[0] < own[0][1] && own[0][0] < loc[1] {
					overlapping = append(overlapping, fmt.Sprintf("%d×%d", min(i, j), max(i, j)))
				}
			}
		}
	}

	slices.Sort(overlapping)
	overlapping = slices.Compact(overlapping)

	want := []string{"8×9"}
	if !slices.Equal(want, overlapping) {
		t.Errorf("overlapping pattern pairs = %v, want %v — review span merging (SecretHits) and redaction order (RedactSecrets) before extending",
			overlapping, want)
	}
}

// TestSecretHitsCountsInjectedTokensIndependentOfContext is the property
// half of the count contract (02-37 §f2): N non-overlapping injected fake
// tokens yield exactly N locations no matter what benign text surrounds
// them — the audit's hit-count must track secrets, not prose volume.
func TestSecretHitsCountsInjectedTokensIndependentOfContext(t *testing.T) {
	t.Parallel()

	tokens := []string{
		fakeAnthropic, fakeOpenAI, fakeOpenAICls, fakeGitHub, fakeGitHubPat,
		fakeAWS, fakeGoogle, fakeSlack, fakeAssign,
	}
	fillers := []string{
		"",
		"build output\n",
		"2026-09-22T00:00:00Z WARN retrying in 1s\n",
		"exit status 1\n",
		strings.Repeat("padding line\n", 37),
	}

	for n := 1; n <= len(tokens); n++ {
		for _, filler := range fillers {
			var b strings.Builder
			for i := range n {
				b.WriteString(filler)
				b.WriteString(tokens[i%len(tokens)])
				b.WriteString("\n")
			}
			b.WriteString(filler)

			if got := SecretHits(b.String()); got != n {
				t.Errorf("SecretHits = %d, want %d (filler %q)", got, n, filler[:min(len(filler), 20)])
			}
		}
	}
}

// TestRedactSecretsMasksAuthHeaderToExactlyOneMarker is the output-side
// mirror of the span-merge count fix (02-37 §f11): the overlapping
// bearer × authorization-header patterns must mask one
// "Authorization: Bearer …" line to a SINGLE marker, never two.
func TestRedactSecretsMasksAuthHeaderToExactlyOneMarker(t *testing.T) {
	t.Parallel()

	header := "Authorization: Bearer " + fakeJWT[len("Bearer "):]
	got := RedactSecrets("request rejected: " + header)

	if count := strings.Count(got, RedactMarker); count != 1 {
		t.Errorf("RedactSecrets emitted %d markers for one header, want 1: %s", count, got)
	}

	if strings.Contains(got, fakeJWT) {
		t.Error("header token survived redaction")
	}
}

func TestSecretHitsMergesOverlappingPatternSpans(t *testing.T) {
	t.Parallel()

	header := "Authorization: Bearer " + fakeJWT[len("Bearer "):]

	if got := SecretHits(header); got != 1 {
		t.Errorf("SecretHits = %d, want 1 (one secret, two overlapping patterns): %s", got, header)
	}

	mixed := header + "\n" + fakeAssign
	if got := SecretHits(mixed); got != 2 {
		t.Errorf("SecretHits = %d, want 2 (distinct secrets stay distinct): %s", got, mixed)
	}
}

func TestTailBytesRedactsSecrets(t *testing.T) {
	t.Parallel()

	filler := strings.Repeat("build output line\n", 300)
	tail := tailBytes([]byte(filler+fakeOpenAI+"\nverify failed"), EvidenceTailBytes)

	if strings.Contains(tail, fakeOpenAI) {
		t.Error("secret survived the evidence tail redaction")
	}

	if !strings.Contains(tail, RedactMarker) || !strings.Contains(tail, "verify failed") {
		t.Errorf("tail lost content: %q", tail)
	}
}

func TestTailBytesHonorsTQRedactFalse(t *testing.T) {
	t.Setenv("TQ_REDACT", "false")

	tail := tailBytes([]byte("output "+fakeGitHub), 1024)

	if !strings.Contains(tail, fakeGitHub) {
		t.Error("--redact=false did not keep the raw tail")
	}
}

func TestSetFailureEvidenceRedactsTail(t *testing.T) {
	t.Parallel()

	ctx, sink := NewSink(t.Context())
	SetFailureEvidence(ctx, "agent", errFailed(), tailBytes([]byte("boom\n"+fakeAWS), EvidenceTailBytes))

	failure := string(sink.Failure())
	if strings.Contains(failure, fakeAWS) {
		t.Errorf("failure evidence carries the raw secret: %s", failure)
	}

	if !strings.Contains(failure, RedactMarker) {
		t.Errorf("failure evidence missing %s: %s", RedactMarker, failure)
	}
}

func errFailed() error { return fakeRunError{} }

type fakeRunError struct{}

func (fakeRunError) Error() string { return "agent run failed" }

func TestWriteOutputSidecarRedacts(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TQ_LOG_DIR", dir)

	id := task.ID("000001a0redact00000001")
	if path := writeOutputSidecar(id, "agent printed "+fakeSlack, "verify printed "+fakeGoogle); path == "" {
		t.Fatal("sidecar write failed")
	}

	raw, err := os.ReadFile(filepath.Join(dir, "000001a0redact00000001.log"))
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}

	body := string(raw)
	for _, secret := range []string{fakeSlack, fakeGoogle} {
		if strings.Contains(body, secret) {
			t.Errorf("sidecar carries raw secret: %s", body)
		}
	}

	if strings.Count(body, RedactMarker) != 2 {
		t.Errorf("sidecar redaction count = %d, want 2: %s", strings.Count(body, RedactMarker), body)
	}
}

func TestWriteVerifyEvidenceRedacts(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TQ_LOG_DIR", dir)

	id := task.ID("000001a0redact00000002")
	if path := writeVerifyEvidence(id, []byte("gate output "+fakeAssign)); path == "" {
		t.Fatal("verify evidence write failed")
	}

	raw, err := os.ReadFile(filepath.Join(dir, "000001a0redact00000002.verify-failure.log"))
	if err != nil {
		t.Fatalf("read verify evidence: %v", err)
	}

	if body := string(raw); strings.Contains(body, fakeAssign) || !strings.Contains(body, RedactMarker) {
		t.Errorf("verify evidence not redacted: %s", body)
	}
}
