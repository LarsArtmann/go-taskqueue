package harvest

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// stubScorer pins BuildProvenance's score-cache read without a store.
type stubScorer struct {
	score  queue.PriorityScore
	cached bool
	err    error
}

func (s stubScorer) PriorityScore(context.Context, string) (queue.PriorityScore, bool, error) {
	return s.score, s.cached, s.err
}

func TestBuildProvenance(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	at := time.Now().UTC()

	detail, err := json.Marshal(queue.ReprioritizeEvidence{
		OldPriority: 10, NewPriority: 70, Source: "marker", Reason: "P1 marker added",
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		task   task.Task
		facts  []journal.Fact
		scorer stubScorer
		want   func(Provenance) bool
	}{
		{
			name: "harvest item carries identity and cached score",
			task: task.Task{
				Priority: 70,
				Payload:  []byte(`{"repo":"cv","item":"fix the flaky gate","dedup":"todo:cv:abc","markerLevel":2}`),
			},
			scorer: stubScorer{score: queue.PriorityScore{ItemKey: "todo:cv:abc", Score: 72}, cached: true},
			want: func(p Provenance) bool {
				return p.ItemKey == "todo:cv:abc" && p.MarkerLevel == 2 &&
					p.Score != nil && p.Score.Score == 72
			},
		},
		{
			name: "foreign payload stays itemless, score read skipped",
			task: task.Task{Priority: 5, Payload: []byte(`{"repo":"cv","prompt":"ad hoc"}`)},
			want: func(p Provenance) bool { return p.ItemKey == "" && p.Score == nil },
		},
		{
			name:   "score cache miss omits the verdict",
			task:   task.Task{Payload: []byte(`{"repo":"cv","item":"x","dedup":"todo:cv:x"}`)},
			scorer: stubScorer{err: errors.New("cache down")},
			want:   func(p Provenance) bool { return p.Score == nil },
		},
		{
			name: "reprioritized facts distill into history, junk skipped",
			task: task.Task{Priority: 70},
			facts: []journal.Fact{
				{Type: journal.Enqueued},
				{Type: journal.Reprioritized, Time: at, Detail: jsontext.Value(detail)},
				{Type: journal.Reprioritized, Detail: jsontext.Value(`{not json`)},
			},
			want: func(p Provenance) bool {
				return len(p.History) == 1 && p.History[0].Source == "marker" &&
					p.History[0].Old == 10 && p.History[0].New == 70 &&
					p.History[0].At.Equal(at)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := BuildProvenance(ctx, tc.scorer, tc.task, tc.facts)
			if !tc.want(got) {
				t.Fatalf("BuildProvenance = %+v", got)
			}
		})
	}
}
