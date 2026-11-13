package worker

import (
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// judgedFailureEvidence must append the failure-path calibration row
// (verdict-less, class stamped) beside stamping the evidence document, so
// the dataset covers every paid attempt outcome — not only completions.
func TestJudgedFailureEvidenceRecordsCalibrationRow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TQ_LOG_DIR", dir)

	tk := task.Task{ID: task.NewID(), Type: "agent"}

	_, sink := executor.NewSink(t.Context())
	execErr := executor.Permanent(errors.New("bad payload"))

	evidence := judgedFailureEvidence(tk, sink, execErr)
	if !strings.Contains(string(evidence), string(executor.FailureClassPermanent)) {
		t.Fatalf("stamped evidence %s lacks the permanent class", evidence)
	}

	raw, err := os.ReadFile(filepath.Join(dir, executor.CalibrationDataset))
	if err != nil {
		t.Fatalf("read dataset: %v", err)
	}

	var row executor.CalibrationRow
	if err := json.Unmarshal(raw[:len(raw)-1], &row); err != nil {
		t.Fatalf("decode calibration row: %v", err)
	}

	if row.Failure == nil || row.Failure.Class != string(executor.FailureClassPermanent) {
		t.Fatalf("calibration row failure half wrong: %+v", row.Failure)
	}

	if len(row.Verdict) != 0 {
		t.Fatalf("failure rows are verdict-less, got %s", row.Verdict)
	}
}
