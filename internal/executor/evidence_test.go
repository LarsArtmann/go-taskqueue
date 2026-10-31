package executor

import (
	"context"
	"encoding/json/v2"
	"strings"
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

// TestKnownEvidenceTermsMatchesStampedVocabulary pins that every term
// KnownEvidenceTerms lists is a value the executor actually stamps (or an
// alias the matcher derives from error text), and that the list carries
// no duplicates: the unknown-term warn (TODO row 1418) is only honest if
// the "known" side is exactly the stamped vocabulary.
func TestKnownEvidenceTermsMatchesStampedVocabulary(t *testing.T) {
	t.Parallel()

	terms := KnownEvidenceTerms()

	stamped := []string{
		"agent", // SetFailureEvidence(ctx, "agent", …) — agent.go
		"command",
		"verify",        // SetVerifyFailureEvidence stamps Stage "verify"
		TaskTypeDepBump, // depbump.go's fail() closure (row 241)
		VerifyStageGofmt, VerifyStageTest, VerifyStageE2ETimeout, VerifyStageRun,
		string(VerifyGateEnvironmental), string(VerifyGateDead), string(VerifyGateSlow),
	}

	if len(terms) != len(stamped) {
		t.Fatalf("KnownEvidenceTerms = %v, want exactly the stamped vocabulary %v", terms, stamped)
	}

	for _, want := range stamped {
		found := false

		for _, term := range terms {
			if strings.EqualFold(term, want) {
				found = true

				break
			}
		}

		if !found {
			t.Fatalf("KnownEvidenceTerms %v missing stamped term %q", terms, want)
		}
	}
}

// TestUnknownEvidenceTerms pins the typo-guard decode: positive terms
// outside the stamped vocabulary surface verbatim (lowercased), negated
// and blank terms are exempt, whitespace and case are tolerated.
func TestUnknownEvidenceTerms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec string
		want []string
	}{
		{name: "all known", spec: "gate-env, gofmt,e2e-timeout,VERIFY,command,agent,run,test,gate-dead,gate-slow"},
		{name: "typoed alias", spec: "gate-en,gate-env", want: []string{"gate-en"}},
		{name: "unknown negation exempt", spec: "!gate-en"},
		{name: "mixed unknown", spec: "gofmt,veryfy", want: []string{"veryfy"}},
		{name: "blank terms skipped", spec: " , ! ,gofmt"},
		{name: "empty spec", spec: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := UnknownEvidenceTerms(tt.spec)

			if len(got) != len(tt.want) {
				t.Fatalf("UnknownEvidenceTerms(%q) = %v, want %v", tt.spec, got, tt.want)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("UnknownEvidenceTerms(%q) = %v, want %v", tt.spec, got, tt.want)
				}
			}
		})
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
