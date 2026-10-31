package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestParkedRowsCarryEvidenceSuffix pins TODO row 1414: `tq tasks --parked`
// rows carry the same evidence= suffix as `tq dlq` rows — the
// verify-stage/class of the death that parked the task, alias-only deaths
// included (row 1419: the alias rides the recorded error text, so a
// gate-env death without a structured Class still renders
// "alias=gate-env").
func TestParkedRowsCarryEvidenceSuffix(t *testing.T) {
	id := task.ID("000001a1parked0000000000000000001")

	detail, err := json.Marshal(executor.FailureEvidence{Stage: "verify", ExitCode: 1})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	facts := []journal.Fact{{
		Type:   journal.Failed,
		TaskID: id.String(),
		Error:  "agent verify gate environmental: " + executor.VerifyGateEnvCode,
		Detail: detail,
	}}

	summaries := latestFailureEvidenceSummary(facts)
	const want = "verify exit=1 alias=gate-env"
	if got := summaries[id.String()]; got != want {
		t.Fatalf("latestFailureEvidenceSummary = %q, want %q", got, want)
	}

	parked := task.Task{ID: id, Status: task.Pending, Type: "agent", Project: "demo"}

	var buf bytes.Buffer
	printTaskListTo(&buf, []task.Task{parked}, 0, func() (int, error) { return 1, nil }, nil, summaries, nil)

	if out := buf.String(); !strings.Contains(out, "evidence="+want) {
		t.Fatalf("parked row = %q, want evidence=%q suffix", out, want)
	}
}

// TestParkedRowsWithoutEvidenceRenderPlain pins the negative: a parked row
// whose task never recorded a task.failed fact renders with NO evidence
// suffix (nothing fabricated).
func TestParkedRowsWithoutEvidenceRenderPlain(t *testing.T) {
	id := task.ID("000001a1parked0000000000000000002")

	summaries := latestFailureEvidenceSummary(nil)
	if len(summaries) != 0 {
		t.Fatalf("empty fact tail fabricated %d summaries", len(summaries))
	}

	parked := task.Task{ID: id, Status: task.Pending, Type: "agent", Project: "demo"}

	var buf bytes.Buffer
	printTaskListTo(&buf, []task.Task{parked}, 0, func() (int, error) { return 1, nil }, nil, summaries, nil)

	if strings.Contains(buf.String(), "evidence=") {
		t.Fatalf("parked row = %q, must not carry an evidence suffix", buf.String())
	}
}
