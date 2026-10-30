package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
)

// TestSecretHitGoldenJSON pins the wire shape of one secret_evidence row:
// seq/taskId/type/field/count — the contract `tq audit --journal --json`
// consumers parse. cmd/tq is replace-free (ADR-0016/ADR-0017), so this Go
// level pin is the drift guard journal-drift.sh phase 6 alone cannot be.
func TestSecretHitGoldenJSON(t *testing.T) {
	blob, err := json.Marshal(SecretHit{
		Seq:    42,
		TaskID: "task-1",
		Type:   "task.failed",
		Field:  "error",
		Count:  1,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	want := `{"seq":42,"taskId":"task-1","type":"task.failed","field":"error","count":1}`
	if got := string(blob); got != want {
		t.Fatalf("SecretHit wire shape drifted:\n got  %s\n want %s", got, want)
	}
}

// TestDriftReportSecretEvidenceJSON pins the parent contract: the key is
// snake_case `secret_evidence`, omitempty (a clean audit stays keyless),
// and rows keep the golden shape when present.
func TestDriftReportSecretEvidenceJSON(t *testing.T) {
	blob, err := json.Marshal(DriftReport{TasksCompared: 1})
	if err != nil {
		t.Fatalf("marshal bare: %v", err)
	}

	if strings.Contains(string(blob), "secret_evidence") {
		t.Fatalf("bare DriftReport leaked secret_evidence (must be omitempty): %s", blob)
	}

	report := DriftReport{
		TasksCompared:  1,
		FactsReplayed:  3,
		SecretEvidence: []SecretHit{{Seq: 7, TaskID: "task-9", Type: "task.dead", Field: "detail", Count: 2}},
	}

	blob, err = json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal populated: %v", err)
	}

	got := string(blob)
	if !strings.Contains(got, `"secret_evidence":[{"seq":7,"taskId":"task-9","type":"task.dead","field":"detail","count":2}]`) {
		t.Fatalf("secret_evidence wire shape drifted: %s", got)
	}
}

// TestScanFactSecretsWiring pins the detector wiring behind the rows: only
// evidence-bearing fact types are scanned, the error and detail fields are
// labeled, and the count rides executor.SecretHits (locations, not
// overlapping pattern matches).
func TestScanFactSecretsWiring(t *testing.T) {
	token := "ghp_" + strings.Repeat("a", 30)

	facts := []journal.Fact{
		{Seq: 1, TaskID: "t1", Type: journal.Failed, Error: "boom with " + token},
		{Seq: 2, TaskID: "t2", Type: journal.DeadLettered, Detail: []byte(`{"tail":"bearer ` + token + `"}`)},
		{Seq: 3, TaskID: "t3", Type: journal.Claimed, Error: "bearer " + token},
		{Seq: 4, TaskID: "t4", Type: journal.Completed, Error: "clean failure"},
	}

	hits := scanFactSecrets(facts)

	if len(hits) != 2 {
		t.Fatalf("got %d hits (%+v), want 2", len(hits), hits)
	}

	if hits[0].Seq != 1 || hits[0].Field != "error" || hits[0].TaskID != "t1" || hits[0].Type != "task.failed" {
		t.Fatalf("error-field hit wrong: %+v", hits[0])
	}

	if hits[1].Seq != 2 || hits[1].Field != "detail" || hits[1].TaskID != "t2" {
		t.Fatalf("detail-field hit wrong: %+v", hits[1])
	}

	for i, hit := range hits {
		content := "boom with " + token
		if i == 1 {
			content = `{"tail":"bearer ` + token + `"}`
		}

		if want := executor.SecretHits(content); hit.Count != want {
			t.Fatalf("hit %+v count %d, want %d", hit, hit.Count, want)
		}
	}
}
