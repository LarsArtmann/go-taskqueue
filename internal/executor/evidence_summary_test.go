package executor

import "testing"

// TestSummaryWithErrorAlias pins the alias pass (TODO row 1419): an
// alias-only death (no structured Class, the gate class living only in
// the recorded error text) renders "alias=gate-env" instead of a bare
// stage, while a structured Class suppresses the pass entirely.
func TestSummaryWithErrorAlias(t *testing.T) {
	tests := []struct {
		name      string
		evidence  FailureEvidence
		lastError string
		want      string
	}{
		{
			name:      "alias-only gate-env death renders the alias",
			evidence:  FailureEvidence{Stage: "verify", ExitCode: 1},
			lastError: "agent verify gate environmental: " + VerifyGateEnvCode,
			want:      "verify exit=1 alias=gate-env",
		},
		{
			name:      "gate-dead alias matched via error text",
			evidence:  FailureEvidence{},
			lastError: "run died: agent verify gate dead (gate artifact)",
			want:      "alias=gate-dead",
		},
		{
			name:      "structured class suppresses the alias pass",
			evidence:  FailureEvidence{Stage: "verify", Class: "environmental", ExitCode: 2},
			lastError: "agent verify gate slow: e2e timeout",
			want:      "verify class=environmental exit=2",
		},
		{
			name:      "no alias and no fields render empty",
			evidence:  FailureEvidence{},
			lastError: "unrelated error",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.evidence.SummaryWithError(tt.lastError); got != tt.want {
				t.Fatalf("SummaryWithError(%q) = %q, want %q", tt.lastError, got, tt.want)
			}
		})
	}
}

// TestFailureEvidenceSummaryComposes pins the cmd/tq-side composition
// against the same corpus: the helper must agree with SummaryWithError
// byte-for-byte so the two surfaces (tq dlq rows, parked rows) render one
// row shape. The composition itself lives in cmd/tq (replace-free module,
// ADR-0017); this pins the executor half it mirrors.
func TestFailureEvidenceSummaryComposes(t *testing.T) {
	cases := []struct {
		evidence  FailureEvidence
		lastError string
	}{
		{FailureEvidence{Stage: "verify", ExitCode: 1}, "agent verify gate environmental: " + VerifyGateEnvCode},
		{FailureEvidence{Stage: "verify", Class: "command", ExitCode: 2}, "plain failure"},
		{FailureEvidence{}, ""},
	}

	for _, c := range cases {
		if got, want := c.evidence.SummaryWithError(c.lastError), c.evidence.Summary(); c.evidence.Class != "" && got != want {
			t.Fatalf("structured class must render exactly Summary: got %q want %q", got, want)
		}
	}
}
