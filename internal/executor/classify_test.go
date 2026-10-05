package executor

import (
	"errors"
	"strings"
	"testing"
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
