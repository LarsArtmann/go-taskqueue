package executor

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// readCalibrationRows decodes every line of the dataset in dir.
func readCalibrationRows(t *testing.T, dir string) []CalibrationRow {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(dir, CalibrationDataset))
	if err != nil {
		t.Fatalf("read dataset: %v", err)
	}

	var rows []CalibrationRow

	for line := range strings.SplitSeq(strings.TrimSpace(string(raw)), "\n") {
		var row CalibrationRow
		if err := json.Unmarshal(jsontext.Value(line), &row); err != nil {
			t.Fatalf("decode row %q: %v", line, err)
		}

		rows = append(rows, row)
	}

	return rows
}

// TestRecordCalibration pins the verdict→outcome calibration dataset: one
// JSONL row per paid run under $TQ_LOG_DIR, the RAW verdict-channel body
// joined against the derived outcome, and the Verify/Attestation join keys
// carried so tuning rules cross-reference the completion fact.
func TestRecordCalibration(t *testing.T) {
	t.Run("row joins verdict, receipt and outcome", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("TQ_LOG_DIR", dir)

		ctx, _ := NewSink(context.Background())

		result := StatusResult{
			Verify:      &VerifyReceipt{Command: "./ci", RC: 0, Head: "abc123", OK: 5, Fail: 0},
			Attestation: &VerifyVerdict{OK: true, Checked: []string{AttestationCheckReceipt}},
			Report:      "docs/status/x.md",

			SessionID:      "sess-1",
			SessionCostUSD: 0.42,
		}

		recordRunOutcome(ctx, &result, "status", jsontext.Value(`{"report":"docs/status/x.md"}`),
			"out", "", task.ID("000001a0calibrationjoinkeys0000001"))

		rows := readCalibrationRows(t, dir)
		if len(rows) != 1 {
			t.Fatalf("rows = %d, want 1", len(rows))
		}

		row := rows[0]
		if row.TaskID != "000001a0calibrationjoinkeys0000001" || row.Kind != "status" {
			t.Fatalf("row identity = %s/%s, want task/kind", row.TaskID, row.Kind)
		}

		if row.At <= 0 {
			t.Fatalf("row.At = %d, want unix milli", row.At)
		}

		if string(row.Verdict) != `{"report":"docs/status/x.md"}` {
			t.Fatalf("row.Verdict = %s, want the raw channel body", row.Verdict)
		}

		if row.Verify == nil || row.Verify.Command != "./ci" || row.Verify.OK != 5 {
			t.Fatalf("row.Verify = %+v, want the receipt join key", row.Verify)
		}

		if row.Attestation == nil || !row.Attestation.OK {
			t.Fatalf("row.Attestation = %+v, want the attestation join key", row.Attestation)
		}

		if row.SessionID != "sess-1" || row.CostUSD != 0.42 {
			t.Fatalf("session block = %s/%v, want sess-1/0.42", row.SessionID, row.CostUSD)
		}
	})

	t.Run("derived outcome counts", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("TQ_LOG_DIR", dir)

		ctx, _ := NewSink(context.Background())

		result := AgentResult{}
		result.Commits = []Commit{{SHA: "deadbee"}, {SHA: "feedface"}}
		result.FilesChanged = []string{"a.go", "b.go", "c.go"}
		result.CommitSHA = "feedface"
		result.PreflightDone = ""

		recordRunOutcome(ctx, &result, "agent", nil, "out", "", task.ID("000001a0calibrationderived000000002"))

		row := readCalibrationRows(t, dir)[0]
		if row.CommitCount != 2 || row.CommitSHA != "feedface" || row.FileCount != 3 {
			t.Fatalf("derived outcome = %d/%s/%d, want 2/feedface/3", row.CommitCount, row.CommitSHA, row.FileCount)
		}
	})

	t.Run("no log dir writes nothing", func(t *testing.T) {
		t.Setenv("TQ_LOG_DIR", "")

		recordCalibration("agent", task.ID("000001a0calibrationnologdir00000003"), nil, nil)
	})

	t.Run("garbage detail keeps the verdict half", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("TQ_LOG_DIR", dir)

		recordCalibration("review", task.ID("000001a0calibrationgarbagede0000004"),
			jsontext.Value(`{"verdict":"request_changes"}`), jsontext.Value(`{"unexpected":"shape"}`))

		row := readCalibrationRows(t, dir)[0]
		if string(row.Verdict) != `{"verdict":"request_changes"}` {
			t.Fatalf("row.Verdict = %s, want the raw body", row.Verdict)
		}

		if row.Verify != nil || row.CommitCount != 0 {
			t.Fatalf("outcome half = %+v, want zero", row)
		}
	})
}

// TestFinishParsedRunCalibrates pins the closeout-free wiring: a parsed
// turn appends its calibration row (kind + raw verdict body), a parse miss
// fails the attempt and writes NO row.
func TestFinishParsedRunCalibrates(t *testing.T) {
	t.Run("success appends a row", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("TQ_LOG_DIR", dir)

		ctx, _ := NewSink(context.Background())
		repo := t.TempDir()

		raw := jsontext.Value(`{"verdict":"fixed","summary":"root cause"}`)

		var result DLQFixResult
		if err := json.Unmarshal(raw, &result); err != nil {
			t.Fatalf("seed result: %v", err)
		}

		if err := finishParsedRun(ctx, &result, nil, "dlqfix", raw, "output", repo,
			task.ID("000001a0calibrationfinishrun0000005")); err != nil {
			t.Fatalf("finishParsedRun: %v", err)
		}

		row := readCalibrationRows(t, dir)[0]
		if row.Kind != "dlqfix" || string(row.Verdict) != string(raw) {
			t.Fatalf("row = %s/%s, want dlqfix + raw body", row.Kind, row.Verdict)
		}
	})

	t.Run("parse miss fails and writes no row", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("TQ_LOG_DIR", dir)

		ctx, _ := NewSink(context.Background())

		var result DLQFixResult

		err := finishParsedRun(ctx, &result, errors.New("parse miss"), "dlqfix", nil, "output", t.TempDir(),
			task.ID("000001a0calibrationparsemiss00000006"))
		if err == nil {
			t.Fatal("want the parse miss to fail the attempt")
		}

		if _, statErr := os.Stat(filepath.Join(dir, CalibrationDataset)); !os.IsNotExist(statErr) {
			t.Fatalf("dataset exists after a parse miss: %v", statErr)
		}
	})
}

func TestRecordCalibrationFailure(t *testing.T) {
	t.Setenv("TQ_LOG_DIR", t.TempDir())
	dir := os.Getenv("TQ_LOG_DIR")

	RecordCalibrationFailure("agent", task.NewID(), &FailureEvidence{
		Stage:    "verify",
		Class:    string(FailureClassEnvironment),
		ExitCode: 1,
		Verify:   &VerifyReceipt{Command: "make", RC: 1},
	})

	raw, err := os.ReadFile(filepath.Join(dir, CalibrationDataset))
	if err != nil {
		t.Fatalf("read dataset: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 1 {
		t.Fatalf("want one row, got %d", len(lines))
	}

	var row CalibrationRow
	if err := json.Unmarshal(jsontext.Value(lines[0]), &row); err != nil {
		t.Fatalf("decode row: %v", err)
	}

	if row.Failure == nil || row.Failure.Stage != "verify" || row.Failure.Class != string(FailureClassEnvironment) {
		t.Fatalf("failure half missing or wrong: %+v", row.Failure)
	}

	if len(row.Verdict) != 0 {
		t.Fatalf("failure rows are verdict-less, got %s", row.Verdict)
	}

	// No log dir: the row is dropped, never an error.
	t.Setenv("TQ_LOG_DIR", "")
	RecordCalibrationFailure("agent", task.NewID(), nil)
}
