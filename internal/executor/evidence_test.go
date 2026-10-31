package executor

import (
	"context"
	"encoding/json/v2"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

type trailStore map[string][]journal.Fact

func (s trailStore) FactsForTask(_ context.Context, id string, _ int) ([]journal.Fact, error) {
	return s[id], nil
}

func TestLastFailureEvidenceReadsLastFailedFact(t *testing.T) {
	id := task.ID("000001a1evidence00000000000000000001")

	detail, err := json.Marshal(FailureEvidence{Stage: "verify", ExitCode: 1})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	failed := journal.Fact{Type: journal.Failed, Detail: detail}

	store := trailStore{id.String(): []journal.Fact{
		{Type: journal.Completed},
		failed,
	}}

	got := LastFailureEvidence(context.Background(), store, id)

	if got != (FailureEvidence{Stage: "verify", ExitCode: 1}) {
		t.Fatalf("got %+v, want stage=verify exit=1", got)
	}
}

func TestLastFailureEvidenceZeroWhenNoFailedFact(t *testing.T) {
	id := task.ID("000001a1evidence00000000000000000002")

	store := trailStore{id.String(): []journal.Fact{{Type: journal.Completed}}}

	if got := LastFailureEvidence(context.Background(), store, id); got != (FailureEvidence{}) {
		t.Fatalf("got %+v, want zero evidence", got)
	}
}
