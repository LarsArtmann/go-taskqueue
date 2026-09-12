package cqrs

import (
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"
	"strings"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	"github.com/larsartmann/go-cqrs-lite/id/v4"
	"github.com/larsartmann/go-taskqueue/internal/journal"
)

// FactSource is the read side of the fact journal the adapter consumes.
// It matches the semantics of the queue stores' Facts method: facts with
// Seq strictly greater than after, in ascending Seq order, bounded to the
// most recent limit when limit > 0.
type FactSource interface {
	Facts(ctx context.Context, after int64, limit int) ([]journal.Fact, error)
}

// Stream types assigned to fact events. Session facts carry the synthetic
// "session:<id>" task identity and get their own stream type so consumers
// can separate interactive-session observations from task history.
const (
	StreamTypeTask    = "Task"
	StreamTypeSession = "Session"
)

var (
	_ event.Journal         = (*FactJournal)(nil)
	_ event.SeekableJournal = (*FactJournal)(nil)
)

// FactJournal adapts a FactSource to the go-cqrs-lite journal interfaces.
// The zero EventID is the journal start: it decodes to position 0.
type FactJournal struct {
	source FactSource
}

// NewFactJournal adapts source (typically a queue store) as a
// go-cqrs-lite seekable journal.
func NewFactJournal(source FactSource) *FactJournal {
	return &FactJournal{source: source}
}

// SliceSource is an in-memory FactSource over an already-loaded fact
// slice (CLI rendering, tests, ephemeral tooling).
type SliceSource struct {
	facts []journal.Fact
}

// NewSliceSource returns a FactSource serving facts in memory with store
// semantics: Seq strictly greater than after, ascending, bounded to the
// most recent limit when limit > 0.
func NewSliceSource(facts []journal.Fact) *SliceSource {
	return &SliceSource{facts: facts}
}

// Facts implements FactSource.
func (s *SliceSource) Facts(_ context.Context, after int64, limit int) ([]journal.Fact, error) {
	var out []journal.Fact

	for _, f := range s.facts {
		if f.Seq > after {
			out = append(out, f)
		}
	}

	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}

	return out, nil
}

// ReadAll implements event.Journal: every fact, in Seq order.
func (j *FactJournal) ReadAll(ctx context.Context) ([]event.Event, error) {
	facts, err := j.source.Facts(ctx, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("cqrs: read all facts: %w", err)
	}

	return mapFacts(facts)
}

// ReadFrom implements event.SeekableJournal. Position is the synthetic
// sequence-encoded event ID; facts are returned in Seq order, up to limit
// when limit > 0. A cursor outside this journal's ID space (random or
// foreign ULID) drains to nothing with a nil error — the go-cqrs-lite
// dangling-cursor contract: an unknown position must never replay the
// journal, because replay would duplicate facts into projections.
func (j *FactJournal) ReadFrom(ctx context.Context, afterEventID id.EventID, limit int) ([]event.Event, error) {
	after, ok := seqFromEventID(afterEventID)
	if !ok {
		return nil, nil
	}

	facts, err := j.source.Facts(ctx, after, limit)
	if err != nil {
		return nil, fmt.Errorf("cqrs: read facts after %d: %w", after, err)
	}

	return mapFacts(facts)
}

func mapFacts(facts []journal.Fact) ([]event.Event, error) {
	events := make([]event.Event, 0, len(facts))

	for _, f := range facts {
		evt, err := factEvent(f)
		if err != nil {
			return nil, err
		}

		events = append(events, evt)
	}

	return events, nil
}

func factEvent(fact journal.Fact) (event.Event, error) {
	if fact.Seq <= 0 {
		return nil, fmt.Errorf("cqrs: fact for task %s has non-positive seq %d", fact.TaskID, fact.Seq)
	}

	payload, err := json.Marshal(factPayload{
		TaskID:  fact.TaskID,
		Type:    fact.Type,
		Owner:   fact.Owner,
		Attempt: fact.Attempt,
		Error:   fact.Error,
		Detail:  fact.Detail,
	})
	if err != nil {
		return nil, fmt.Errorf("cqrs: marshal fact seq %d: %w", fact.Seq, err)
	}

	eventID, err := seqEventID(fact.Seq)
	if err != nil {
		return nil, err
	}

	streamID, err := id.ParseStreamID(fact.TaskID)
	if err != nil {
		return nil, fmt.Errorf("cqrs: fact seq %d stream id: %w", fact.Seq, err)
	}

	streamType := id.StreamType(StreamTypeTask)
	if strings.HasPrefix(fact.TaskID, "session:") {
		streamType = id.StreamType(StreamTypeSession)
	}

	evt, err := event.NewEvent(
		event.Type(fact.Type),
		streamID,
		streamType,
		event.Version(uint64(fact.Seq)),
		payload,
		event.WithEventID(eventID),
		event.WithOccurredAt(fact.Time),
	)
	if err != nil {
		return nil, fmt.Errorf("cqrs: build event for fact seq %d: %w", fact.Seq, err)
	}

	return evt, nil
}

// factPayload carries the fact body; Seq and Time are deliberately absent
// because they are the event's Version and OccurredAt.
type factPayload struct {
	TaskID  string           `json:"taskId"`
	Type    journal.FactType `json:"type"`
	Owner   string           `json:"owner,omitempty"`
	Attempt int              `json:"attempt,omitempty"`
	Error   string           `json:"error,omitempty"`
	Detail  jsontext.Value   `json:"detail,omitempty"`
}
