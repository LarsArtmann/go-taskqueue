package executor

import (
	"os"
	"path/filepath"
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
		"anthropic":  fakeAnthropic,
		"openai-proj": fakeOpenAI,
		"openai-classic": fakeOpenAICls,
		"github":     fakeGitHub,
		"github-pat": fakeGitHubPat,
		"aws":        fakeAWS,
		"google":     fakeGoogle,
		"slack":      fakeSlack,
		"bearer":     fakeJWT,
		"assignment": fakeAssign,
	}

	for name, secret := range samples {
		got := RedactSecrets("failed with: "+secret+" — retrying")

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
		"go test ./... -race",                            // plain command output
		"task-skills-configuration-handler failed",       // sk- prose false positive
		"password: (none)",                               // short assignment value
		"3 tasks succeeded",                              // ordinary log line
		"token count: 128",                               // free-form 'token' word survives
		"asking for bearer token required for endpoint",  // bearer prose, no key
		"sk- short prefix",                               // sk- without a key body
		"AKIA too short",                                 // AWS prefix without key body
		"exit status 1: build failed in pkg/api/key.go",  // path containing api/key
		"",                                               // empty stays empty
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

func errFailed() error { return errFailedSentinel{} }

type errFailedSentinel struct{}

func (errFailedSentinel) Error() string { return "agent run failed" }

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
