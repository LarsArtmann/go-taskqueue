package executor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

func TestClassifyFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want FailureClass
	}{
		{"nil", nil, FailureClassTransient},
		{"plain error", errors.New("exit status 1"), FailureClassTransient},
		{"rate limit", &RateLimitError{Cause: errors.New("429"), RetryAfter: 1}, FailureClassProviderWindow},
		{"wrapped rate limit", &VerifyGateError{Cause: &RateLimitError{Cause: errors.New("429"), RetryAfter: 1}}, FailureClassProviderWindow},
		{"permanent", Permanent(errors.New("bad payload")), FailureClassPermanent},
		{"preflight", &PreflightError{Cause: errors.New("dirty tree")}, FailureClassEnvironment},
		{"verify gate", &VerifyGateError{Cause: errors.New("gate dead")}, FailureClassEnvironment},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := ClassifyFailure(tt.err); got != tt.want {
				t.Fatalf("ClassifyFailure(%v) = %q, want %q", tt.err, got, tt.want)
			}
		})
	}
}

func TestDLQFixPromptCarriesFailureClass(t *testing.T) {
	t.Parallel()

	prompt := dlqFixPrompt(DLQFixPayload{
		DeadTask: "t1",
		Work:     "do the thing",
		Failure: FailureEvidence{
			Stage: "agent",
			Tail:  "boom",
			Class: string(FailureClassProviderWindow),
		},
	})

	for _, want := range []string{"Retry classification: provider-window", "quota/rate window"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("autopsy prompt missing %q", want)
		}
	}

	// A classless (legacy) evidence must not render a classification line.
	plain := dlqFixPrompt(DLQFixPayload{DeadTask: "t1", Work: "w", Failure: FailureEvidence{Stage: "agent", Tail: "boom"}})
	if strings.Contains(plain, "Retry classification") {
		t.Error("classless evidence rendered a retry-classification line")
	}
}

func TestRetrySessionLadder(t *testing.T) {
	t.Setenv("TQ_LOG_DIR", t.TempDir())
	dir := os.Getenv("TQ_LOG_DIR")

	tk := task.Task{ID: task.ID("t-ladder"), Attempts: 1}

	// No sidecar yet: the first retry degrades to fresh.
	if got := retrySession(tk); got != "" {
		t.Errorf("no sidecar: retrySession = %q, want empty (fresh)", got)
	}

	// A sidecar with a session id: the first retry resumes it.
	line := "INFO Created session for non-interactive run session_id=s_ladder1\nsome output\n"
	if err := os.WriteFile(filepath.Join(dir, tk.ID.String()+".log"), []byte(line), 0o644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}

	if got := retrySession(tk); got != "s_ladder1" {
		t.Errorf("retrySession = %q, want s_ladder1 (same_session rung)", got)
	}

	// From the second retry on, the ladder goes back to fresh.
	tk.Attempts = 2
	if got := retrySession(tk); got != "" {
		t.Errorf("second retry: retrySession = %q, want empty (fresh rung)", got)
	}

	// The first run never resumes.
	tk.Attempts = 0
	if got := retrySession(tk); got != "" {
		t.Errorf("first run: retrySession = %q, want empty (fresh)", got)
	}
}
