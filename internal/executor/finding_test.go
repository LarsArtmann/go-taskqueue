package executor

import (
	"strings"
	"testing"
)

// TestMergeFindingsLearnedNeverLowersDeterministic pins turnstone's merge
// rule (SECURITY.md "Learned checks may only narrow"): a learned detector may
// raise severity or add flags, but a crafted "benign" verdict can never
// lower a deterministic finding or clear its flags.
func TestMergeFindingsLearnedNeverLowersDeterministic(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		deterministic Finding
		learned       Finding
		wantSeverity  string
		wantFlags     []string
	}{
		{
			name:          "learned low cannot lower deterministic high",
			deterministic: Finding{Severity: RiskHigh, Flags: []string{"secret"}},
			learned:       Finding{Severity: RiskLow},
			wantSeverity:  RiskHigh,
			wantFlags:     []string{"secret"},
		},
		{
			name:          "learned high raises deterministic low",
			deterministic: Finding{Severity: RiskLow, Flags: []string{"a"}},
			learned:       Finding{Severity: RiskHigh, Flags: []string{"b"}},
			wantSeverity:  RiskHigh,
			wantFlags:     []string{"a", "b"},
		},
		{
			name:          "unrated learned cannot lower",
			deterministic: Finding{Severity: RiskMedium, Flags: []string{"gate"}},
			learned:       Finding{},
			wantSeverity:  RiskMedium,
			wantFlags:     []string{"gate"},
		},
		{
			name:          "flags union dedups and drops empties",
			deterministic: Finding{Severity: RiskMedium, Flags: []string{"x", "y"}},
			learned:       Finding{Severity: RiskMedium, Flags: []string{"y", "z", ""}},
			wantSeverity:  RiskMedium,
			wantFlags:     []string{"x", "y", "z"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := MergeFindings(tc.deterministic, tc.learned)

			if got.Severity != tc.wantSeverity {
				t.Fatalf("severity = %q, want %q", got.Severity, tc.wantSeverity)
			}

			if strings.Join(got.Flags, ",") != strings.Join(tc.wantFlags, ",") {
				t.Fatalf("flags = %v, want %v", got.Flags, tc.wantFlags)
			}
		})
	}
}

// TestMergeFindingsCannotClearDeterministicDetectors pins the merge against
// the real deterministic detectors it will gate: a secret hit must not be
// erasable by a learned "benign" finding (the judge-as-veto-only promise).
func TestMergeFindingsCannotClearDeterministicDetectors(t *testing.T) {
	t.Parallel()

	if SecretHits("AKIAIOSFODNN7EXAMPLE") == 0 {
		t.Fatal("fixture: expected the sample AWS key to trip the secret scanner")
	}

	secret := Finding{Severity: RiskHigh, Flags: []string{"secret-evidence"}}

	cleared := MergeFindings(secret, Finding{Severity: RiskLow})

	if cleared.Severity != RiskHigh || len(cleared.Flags) != 1 {
		t.Fatalf("a learned benign finding cleared the secret tripwire: %+v", cleared)
	}
}
