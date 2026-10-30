package executor

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"slices"
	"strings"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// FactTrailReader reads one task's own fact trail — the minimal surface
// the shared failure-evidence reader needs off a queue store.
type FactTrailReader interface {
	FactsForTask(ctx context.Context, id string, limit int) ([]journal.Fact, error)
}

// LastFailureEvidence pulls the FailureEvidence off the task's LAST
// task.failed fact (the why lives there, not on the terminal fact).
// Best-effort: an absent trail or unparsable detail leaves the evidence
// zero. ONE backward-scan decode shared by dlqfix autopsies and the
// cmd/tq failure surface.
func LastFailureEvidence(ctx context.Context, store FactTrailReader, id task.ID) FailureEvidence {
	trail, err := store.FactsForTask(ctx, id.String(), 0)
	if err != nil {
		return FailureEvidence{}
	}

	return LastFailureEvidenceFromTrail(trail)
}

// LastFailureEvidenceFromTrail decodes the FailureEvidence off the last
// task.failed fact in an already-fetched trail.
func LastFailureEvidenceFromTrail(trail []journal.Fact) FailureEvidence {
	for _, fact := range slices.Backward(trail) {
		if fact.Type != journal.Failed {
			continue
		}

		var evidence FailureEvidence

		if json.Unmarshal(fact.Detail, &evidence) == nil {
			return evidence
		}

		break
	}

	return FailureEvidence{}
}

// Summary renders the evidence as the compact inline suffix shared by the
// webui fact timeline and the tq dlq listing rows: "verify/gofmt class=x
// exit=1". Empty for zero evidence.
func (e FailureEvidence) Summary() string {
	stage := e.Stage

	if e.VerifyStage != "" {
		stage += "/" + e.VerifyStage
	}

	if e.Class != "" {
		stage += " class=" + e.Class
	}

	if e.ExitCode != 0 {
		stage += fmt.Sprintf(" exit=%d", e.ExitCode)
	}

	return stage
}

// MatchesEvidenceStage reports whether failure evidence names the
// requested stage class — matched case-insensitively against the evidence
// Stage, VerifyStage and retry-taxonomy Class fields, plus the verify-gate
// aliases (gate-env / gate-dead / gate-slow) that live only in the
// recorded error text, never in a structured field. ONE copy of the
// death-class alias knowledge: tq dlq --evidence-stage and the webui
// evidence filter share it.
func MatchesEvidenceStage(evidence FailureEvidence, lastError, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))

	if want == "" {
		return false
	}

	for _, have := range []string{evidence.Stage, evidence.VerifyStage, evidence.Class} {
		if strings.EqualFold(have, want) {
			return true
		}
	}

	switch want {
	case string(VerifyGateEnvironmental):
		return strings.Contains(lastError, VerifyGateEnvCode) ||
			strings.Contains(lastError, "agent verify gate environmental")
	case string(VerifyGateDead):
		return strings.Contains(lastError, "agent verify gate dead")
	case string(VerifyGateSlow):
		return strings.Contains(lastError, "agent verify gate slow")
	}

	return false
}
