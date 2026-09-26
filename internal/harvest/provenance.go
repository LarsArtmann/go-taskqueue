package harvest

import (
	"context"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// PriorityScorer is the score-cache read the priority story needs: both
// the sqlite store behind `tq show` and the webui server's queue.Store
// satisfy it.
type PriorityScorer interface {
	PriorityScore(ctx context.Context, itemKey string) (queue.PriorityScore, bool, error)
}

// ProvenanceEvent is one distilled task.reprioritized fact.
type ProvenanceEvent struct {
	At     time.Time
	Old    int
	New    int
	Source string
	Reason string
}

// Provenance is a task's priority story (ADR-0015): what the task's
// priority is, which band it sits in, the harvested item identity with its
// cached AI verdict (when the item was ever scored), and the
// reprioritization history — the answer to "why is this task ranked
// here?". Renderers project it into their own view shapes (`tq show`'s
// JSON section, the webui detail rows).
type Provenance struct {
	Current     int
	Band        string
	ItemKey     string
	MarkerLevel int
	Score       *queue.PriorityScore
	History     []ProvenanceEvent
}

// BuildProvenance reads a task's priority story off the score cache and
// its fact trail. Best-effort by design: a score-cache or evidence-parse
// miss simply omits the field rather than failing the caller.
func BuildProvenance(ctx context.Context, scorer PriorityScorer, t task.Task, trail []journal.Fact) Provenance {
	p := Provenance{Current: t.Priority, Band: string(queue.BandOf(t.Priority))}

	if item, ok := PayloadItemOf(t); ok {
		p.ItemKey = item.Key
		p.MarkerLevel = item.MarkerLevel

		if score, cached, err := scorer.PriorityScore(ctx, item.Key); err == nil && cached {
			p.Score = &score
		}
	}

	for _, fact := range trail {
		if fact.Type != journal.Reprioritized {
			continue
		}

		evidence, ok := queue.ParseReprioritizeEvidence(fact.Detail)
		if !ok {
			continue
		}

		p.History = append(p.History, ProvenanceEvent{
			At:     fact.Time,
			Old:    evidence.OldPriority,
			New:    evidence.NewPriority,
			Source: evidence.Source,
			Reason: evidence.Reason,
		})
	}

	return p
}
