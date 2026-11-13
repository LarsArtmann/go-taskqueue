package executor

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
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
	// Failure carries the stamped forensics of a FAILED attempt (the
	// worker's stampedFailureEvidence document: stage, class, exit code,
	// tail, verify receipt + attestation). Nil on completion rows —
	// ABSENCE means the row observed a completion, not a death, so the
	// tuning rules can exclude failure outcomes from verdict rates
	// without a second decoder.
	Failure *FailureEvidence `json:"failure,omitempty"`
}

// RecordCalibrationFailure appends ONE verdict-less calibration row for a
// FAILED attempt (parse miss, verify-gate death, rate-limit requeue, …):
// the same dataset file, with the failure half populated (class + the
// FailureEvidence join keys) and the verdict half empty. Best effort like
// recordCalibration; a nil evidence still writes the row (the class is
// the join the DLQ autopsy needs, and an evidence-less death is exactly
// the empty-detail class worth counting).
func RecordCalibrationFailure(kind string, id task.ID, evidence *FailureEvidence) {
	dir := os.Getenv("TQ_LOG_DIR")
	if dir == "" {
		return
	}

	b, err := json.Marshal(CalibrationRow{
		TaskID:  id.String(),
		Kind:    kind,
		At:      time.Now().UnixMilli(),
		Failure: evidence,
	})
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

// CalibrationMaxBytes is the retention cap for calibration.jsonl: the
// dataset is one append-only file that no sidecar sweep touches (the
// sweeps consider only *.log, by design — the dataset is not a log), so
// a long-lived pool would grow it forever. Keep-the-tail at 64 MiB ≈
// tens of thousands of rows — far past the ≥50-per-kind tuning floor —
// while bounding the file. Owner-adjustable later; a trim drops the
// OLDEST rows (the freshest evidence is the tuning surface's input).
const CalibrationMaxBytes = int64(64) << 20

// TrimCalibrationDataset enforces CalibrationMaxBytes on dir's
// calibration.jsonl: when the file exceeds the cap, the oldest rows are
// dropped until the newest complete lines fit (a leading partial line
// from the cut point is discarded too). The rewrite is atomic
// (tmp + rename, mode 0600). Absent file or sub-cap size is a no-op;
// best effort by contract — an error is returned for the caller to log,
// never to fail the run that wrote rows.
func TrimCalibrationDataset(dir string, maxBytes int64) (bool, error) {
	if dir == "" || maxBytes <= 0 {
		return false, nil
	}

	path := filepath.Join(dir, CalibrationDataset)

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, fmt.Errorf("executor: stat calibration dataset: %w", err)
	}

	if info.Size() <= maxBytes {
		return false, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("executor: read calibration dataset: %w", err)
	}

	// Cut to the newest maxBytes bytes, then drop the torn leading line.
	cut := len(data) - int(maxBytes)
	if cut > 0 {
		if nl := bytes.IndexByte(data[cut:], '\n'); nl >= 0 {
			cut += nl + 1
		} else {
			cut = len(data) // one monster row: nothing complete survives
		}
	}
	trimmed := data[cut:]

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, trimmed, 0o600); err != nil {
		return false, fmt.Errorf("executor: write calibration dataset trim: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)

		return false, fmt.Errorf("executor: swap calibration dataset trim: %w", err)
	}

	return true, nil
}
