package main

import (
	"context"

	"github.com/larsartmann/go-taskqueue/internal/executor"
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

// failureEvidenceFor delegates to the shared executor reader (ONE
// backward-scan decode, 2026-10-26 01-40 §b3) — best-effort, an absent
// or unparsable detail leaves the evidence zero.
func failureEvidenceFor(ctx context.Context, store *sqlite.Store, id task.ID) executor.FailureEvidence {
	return executor.LastFailureEvidence(ctx, store, id)
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

// evidenceStageMatches delegates to the shared executor matcher — ONE copy
// of the death-class alias knowledge, shared with the webui evidence
// filter (2026-10-26 01-40 §b1).
func evidenceStageMatches(evidence executor.FailureEvidence, lastError, want string) bool {
	return executor.MatchesEvidenceStage(evidence, lastError, want)
}
