package main

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// verifyReceiptView is the cmd/tq TWIN of executor.VerifyReceipt plus the
// stored executor.VerifyVerdict: this module is replace-free (ADR-0017) and
// resolves internal/* from the last published tag, so it cannot name the
// receipt types until a release re-pins them. The same boundary the evidence=
// suffix lives behind (parked.go's failureEvidenceSummary twin).
//
// Keep the JSON spelling in lockstep with internal/executor/receipt.go;
// TestVerifyReceiptViewDecodesExecutorSpelling pins it against a literal
// produced by the real struct. Delete this twin the day cmd/tq re-pins past
// the release that shipped the receipt.
type verifyReceiptView struct {
	Command string `json:"command,omitempty"`
	RC      int    `json:"rc"`
	Head    string `json:"head,omitempty"`
	OK      int    `json:"ok"`
	Fail    int    `json:"fail"`
	// Verdict is the rendered attestation: "ok", one of the refusal classes
	// (no-receipt / head-mismatch / command-mismatch / count-mismatch), or
	// "unattested" for a receipt stored before verdicts were stamped.
	Verdict string `json:"verdict"`
	// Detail is the human-readable mismatch when Verdict is a refusal.
	Detail string `json:"detail,omitempty"`
	// Checked lists the comparisons the verdict actually performed, so a
	// reader can tell "passed" from "not verified".
	Checked []string `json:"checked,omitempty"`
}

// terminalVerifyDetail is the slice of a completion/failure detail the receipt
// view needs. Everything is optional: a legacy detail has neither key.
type terminalVerifyDetail struct {
	Verify *verifyReceiptView `json:"verify"`
	// Attestation is the stored executor.VerifyVerdict.
	Attestation *struct {
		OK      bool     `json:"ok"`
		Refusal string   `json:"refusal,omitempty"`
		Detail  string   `json:"detail,omitempty"`
		Checked []string `json:"checked,omitempty"`
	} `json:"attestation"`
}

// buildVerifyReceiptView decodes the verify receipt off the task's terminal
// fact — the completion detail, or the failure detail for a dead task. Returns
// nil when no receipt was recorded (a legacy task, a command/HTTP task, or a
// run with nothing to verify): absent is absent, never a fabricated "ok".
func buildVerifyReceiptView(trail []journal.Fact) *verifyReceiptView {
	for _, fact := range slices.Backward(trail) {
		if fact.Type != journal.Completed && fact.Type != journal.Failed {
			continue
		}

		if len(fact.Detail) == 0 {
			continue
		}

		var detail terminalVerifyDetail

		if json.Unmarshal(fact.Detail, &detail) != nil || detail.Verify == nil {
			continue
		}

		view := detail.Verify
		view.Verdict = "unattested"

		if att := detail.Attestation; att != nil {
			view.Checked = att.Checked
			view.Detail = att.Detail
			view.Verdict = att.Refusal

			if att.OK {
				view.Verdict = "ok"
			}
		}

		return view
	}

	return nil
}

// verifyReceiptFor is buildVerifyReceiptView for callers that only have a
// store: one trail read, the receipt decoded off it. Best-effort — a read
// failure yields nil.
func verifyReceiptFor(ctx context.Context, store *sqlite.Store, id task.ID) *verifyReceiptView {
	trail, err := store.FactsForTask(ctx, id.String(), 0)
	if err != nil {
		return nil
	}

	return buildVerifyReceiptView(trail)
}

// dlqForensicsFor reads ONE trail per dead task and returns both surfaces the
// `tq dlq` row renders: the typed failure evidence (pinned-tag shape) and the
// verify receipt twin. One IO for two views, because the row carries both and
// the DLQ read cap must stay a cap on READS, not on rows.
func dlqForensicsFor(
	ctx context.Context,
	store *sqlite.Store,
	id task.ID,
) (executor.FailureEvidence, *verifyReceiptView) {
	trail, err := store.FactsForTask(ctx, id.String(), 0)
	if err != nil {
		return executor.FailureEvidence{}, nil
	}

	return executor.LastFailureEvidenceFromTrail(trail), buildVerifyReceiptView(trail)
}

// verifySuffix renders the compact `verify=` term `tq dlq` rows carry, or ""
// when the task recorded no receipt at all (a row without the term reads as
// "no gate evidence", which is the honest statement for a legacy dead task).
func verifySuffix(receipt *verifyReceiptView) string {
	if receipt == nil {
		return ""
	}

	return "verify=" + receipt.Verdict
}
