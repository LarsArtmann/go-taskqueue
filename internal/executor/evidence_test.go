package executor

import (
	"context"
	"encoding/json/v2"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

type trailStore map[string][]journal.Fact

func (s trailStore) FactsForTask(_ context.Context, id string, _ int) ([]journal.Fact, error) {
	return s[id], nil
}

func TestLastFailureEvidenceReadsLastFailedFact(t *testing.T) {
	id := task.ID("000001a1evidence00000000000000000001")

	detail, err := json.Marshal(FailureEvidence{Stage: "verify", ExitCode: 1})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	failed := journal.Fact{Type: journal.Failed, Detail: detail}

	store := trailStore{id.String(): []journal.Fact{
		{Type: journal.Completed},
		failed,
	}}

	got := LastFailureEvidence(context.Background(), store, id)

	if got != (FailureEvidence{Stage: "verify", ExitCode: 1}) {
		t.Fatalf("got %+v, want stage=verify exit=1", got)
	}
}

func TestLastFailureEvidenceZeroWhenNoFailedFact(t *testing.T) {
	id := task.ID("000001a1evidence00000000000000000002")

	store := trailStore{id.String(): []journal.Fact{{Type: journal.Completed}}}

	if got := LastFailureEvidence(context.Background(), store, id); got != (FailureEvidence{}) {
		t.Fatalf("got %+v, want zero evidence", got)
	}
}

// TestMatchesEvidenceStageTable pins the single-term matcher IN the
// executor module: structured fields (Stage, VerifyStage, Class) match
// case-insensitively; the verify-gate aliases (gate-env / gate-dead /
// gate-slow) match through the recorded error text, the only place those
// classes live (TODO row 1409 — previously pinned only transitively via
// cmd/tq's wrapper test).
func TestMatchesEvidenceStageTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		evidence  FailureEvidence
		lastError string
		want      string
		match     bool
	}{
		{
			name:     "verify_stage exact",
			evidence: FailureEvidence{Stage: "verify", VerifyStage: VerifyStageE2ETimeout},
			want:     "e2e-timeout",
			match:    true,
		},
		{
			name:     "stage case-insensitive",
			evidence: FailureEvidence{Stage: "verify"},
			want:     "Verify",
			match:    true,
		},
		{
			name:     "class",
			evidence: FailureEvidence{Class: "environment"},
			want:     "environment",
			match:    true,
		},
		{
			name:      "gate-env via env code",
			lastError: `agent verify gate environmental signature [vendor-gofmt] ("gofmt"): boom`,
			want:      string(VerifyGateEnvironmental),
			match:     true,
		},
		{
			name:      "gate-dead via error text",
			lastError: `agent verify gate dead ("gofmt"): failure is pre-existing`,
			want:      string(VerifyGateDead),
			match:     true,
		},
		{
			name:      "gate-slow via error text",
			lastError: `agent verify gate slow ("test"): deadline before a verdict`,
			want:      string(VerifyGateSlow),
			match:     true,
		},
		{
			name:     "no match",
			evidence: FailureEvidence{Stage: "agent"},
			want:     "e2e-timeout",
			match:    false,
		},
		{
			name: "empty want never matches",
			want: "  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := MatchesEvidenceStage(tt.evidence, tt.lastError, tt.want); got != tt.match {
				t.Fatalf("MatchesEvidenceStage(%+v, %q, %q) = %v, want %v", tt.evidence, tt.lastError, tt.want, got, tt.match)
			}
		})
	}
}
