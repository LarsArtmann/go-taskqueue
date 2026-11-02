package executor

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// CalibrationRow is ONE row of the verdict→outcome calibration dataset
// (turnstone's `intent_verdicts.user_decision` pattern): the structured
// verdict an agent turn recorded on the file channel, persisted against
// the run's derived outcome and the verify-gate join keys, so
// auto-dismiss/review rules can be tuned from data instead of by feel.
//
// Join keys: Verify (the receipt of the gate that judged the run) and
// Attestation (the write-time verdict on that receipt) — the same types
// the result facts carry, so a calibration row cross-references its
// completion fact by value comparison, not by guesswork.
type CalibrationRow struct {
	// TaskID names the queue task the run served.
	TaskID string `json:"task_id"`
	// Kind is the executor that ran: "agent", "review", "status",
	// "dlqfix" or "prioritize" — the auto-dismiss/review rules are
	// per-kind, so the row must say which family it tunes.
	Kind string `json:"kind"`
	// At is the write moment, unix MILLI (the repo-wide timestamp
	// convention — sub-millisecond ordering is never assumed).
	At int64 `json:"at"`
	// Verdict is the RAW verdict-channel body (the file's last write,
	// verbatim) — the gold label before any parser's opinion. Nil when
	// the run recorded no verdict (plain work turns ride the footer
	// derivation; their calibration row is still written, with the
	// outcome half populated).
	Verdict jsontext.Value `json:"verdict,omitempty"`
	// Verify is the gate receipt (executor.VerifyReceipt semantics:
	// absent means no gate ran — UNVERIFIED, never invalid).
	Verify *VerifyReceipt `json:"verify,omitempty"`
	// Attestation is the write-time verdict on Verify.
	Attestation *VerifyVerdict `json:"attestation,omitempty"`
	// Derived outcome: what the queue learned about the run WITHOUT the
	// agent reporting back — footer-attributed commits and the files
	// they touched, plus the crush session the run spent.
	CommitCount         int     `json:"commit_count"`
	CommitSHA           string  `json:"commit_sha,omitempty"`
	FileCount           int     `json:"file_count"`
	SessionID           string  `json:"session_id,omitempty"`
	CostUSD             float64 `json:"cost_usd,omitempty"`
	SessionPromptTokens int64   `json:"session_prompt_tokens,omitempty"`
	SessionCompletion   int64   `json:"session_completion_tokens,omitempty"`
	// PreflightDone carries the done-preflight reason when the task
	// completed WITHOUT an agent run (the no-verdict, no-cost class the
	// tuning rules must be able to exclude).
	PreflightDone string `json:"preflight_done,omitempty"`
}

// calibrationDetail mirrors the shared subset of the registered result
// types (AgentResult, ReviewResult, StatusResult, DLQFixResult,
// PrioritizeResult) as they marshal — the ONE decode shape
// recordCalibration needs, so no per-kind extraction branch exists and a
// new result type that keeps the shared field spellings lands in the
// dataset for free.
type calibrationDetail struct {
	Verify                  *VerifyReceipt `json:"verify"`
	Attestation             *VerifyVerdict `json:"attestation"`
	Commits                 []Commit       `json:"commits"`
	CommitSHA               string         `json:"commit_sha"`
	FilesChanged            []string       `json:"files_changed"`
	SessionID               string         `json:"session_id"`
	SessionCostUSD          float64        `json:"session_cost_usd"`
	SessionPromptTokens     int64          `json:"session_prompt_tokens"`
	SessionCompletionTokens int64          `json:"session_completion_tokens"`
	PreflightDone           string         `json:"preflight_done"`
}

// CalibrationDataset is the JSONL file (ONE row per line) the dataset
// accumulates in, inside $TQ_LOG_DIR — the same directory the sidecar
// logs already own, so a pool configured for forensics gets calibration
// for free and an unconfigured one keeps nothing (degrades, never
// errors a run).
const CalibrationDataset = "calibration.jsonl"

// recordCalibration appends ONE calibration row: the raw verdict body
// joined against the run's registered result detail (decoded through
// calibrationDetail) and the receipt/attestation join keys. Best-effort
// by contract: an unusable log dir, a decode miss or a marshal failure
// drops the row and never fails the run it observed.
func recordCalibration(kind string, id task.ID, verdict, detail jsontext.Value) {
	dir := os.Getenv("TQ_LOG_DIR")
	if dir == "" {
		return
	}

	var d calibrationDetail
	if len(detail) > 0 {
		// A decode miss leaves the outcome half zero — the verdict half
		// (the point of the dataset) is independent of it.
		_ = json.Unmarshal(detail, &d)
	}

	row := CalibrationRow{
		TaskID:              id.String(),
		Kind:                kind,
		At:                  time.Now().UnixMilli(),
		Verdict:             verdict,
		Verify:              d.Verify,
		Attestation:         d.Attestation,
		CommitCount:         len(d.Commits),
		CommitSHA:           d.CommitSHA,
		FileCount:           len(d.FilesChanged),
		SessionID:           d.SessionID,
		CostUSD:             d.SessionCostUSD,
		SessionPromptTokens: d.SessionPromptTokens,
		SessionCompletion:   d.SessionCompletionTokens,
		PreflightDone:       d.PreflightDone,
	}

	b, err := json.Marshal(row)
	if err != nil {
		return
	}

	f, err := os.OpenFile(filepath.Join(dir, CalibrationDataset),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	_, _ = f.Write(append(b, '\n'))
}
