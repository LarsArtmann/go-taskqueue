package main

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// failureEvidenceView is the tq show render of a dead task's failure
// forensics: the structured evidence of the LAST task.failed fact, so a
// death's stage class (gofmt / test / e2e-timeout / run, gate-env /
// gate-dead) is a queryable JSON surface instead of log forensics
// (2026-09-29 01-30 §f8/§f13 + 05-23 §f1).
type failureEvidenceView struct {
	Stage       string `json:"stage,omitempty"`
	VerifyStage string `json:"verify_stage,omitempty"`
	Class       string `json:"class,omitempty"`
	ExitCode    int    `json:"exit_code,omitempty"`
	Tail        string `json:"tail,omitempty"`
}

// failureEvidenceFor pulls the FailureEvidence off the task's LAST
// task.failed fact (the why lives there, not on the terminal fact) —
// the ONE decode in cmd/tq; best-effort, an absent or unparsable detail
// leaves the evidence zero.
func failureEvidenceFor(ctx context.Context, store *sqlite.Store, id task.ID) executor.FailureEvidence {
	trail, err := store.FactsForTask(ctx, id.String(), 0)
	if err != nil {
		return executor.FailureEvidence{}
	}

	for _, fact := range slices.Backward(trail) {
		if fact.Type != journal.Failed {
			continue
		}

		var evidence executor.FailureEvidence

		if json.Unmarshal(fact.Detail, &evidence) == nil {
			return evidence
		}

		break
	}

	return executor.FailureEvidence{}
}

// buildFailureEvidenceView decodes the task's last task.failed fact into
// the show view; a task with no failing fact (or a legacy empty detail)
// shows no failure section.
func buildFailureEvidenceView(ctx context.Context, store *sqlite.Store, id task.ID) *failureEvidenceView {
	evidence := failureEvidenceFor(ctx, store, id)

	if evidence == (executor.FailureEvidence{}) {
		return nil
	}

	return &failureEvidenceView{
		Stage:       evidence.Stage,
		VerifyStage: evidence.VerifyStage,
		Class:       evidence.Class,
		ExitCode:    evidence.ExitCode,
		Tail:        evidence.Tail,
	}
}

// evidenceStageMatches reports whether a dead task's failure evidence
// names the requested stage class — matched case-insensitively against
// the evidence Stage, VerifyStage and retry-taxonomy Class fields, plus
// the verify-gate aliases (gate-env / gate-dead / gate-slow) that live
// only in the recorded error text, never in a structured field.
func evidenceStageMatches(evidence executor.FailureEvidence, lastError, want string) bool {
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
	case string(executor.VerifyGateEnvironmental):
		return strings.Contains(lastError, executor.VerifyGateEnvCode) ||
			strings.Contains(lastError, "agent verify gate environmental")
	case string(executor.VerifyGateDead):
		return strings.Contains(lastError, "agent verify gate dead")
	case string(executor.VerifyGateSlow):
		return strings.Contains(lastError, "agent verify gate slow")
	}

	return false
}
