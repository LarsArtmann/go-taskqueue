package cqrs

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	"github.com/larsartmann/go-cqrs-lite/id/v4"
	"github.com/larsartmann/go-taskqueue/internal/journal"
)

type fakeSource struct {
	facts []journal.Fact
}

func (f *fakeSource) Facts(_ context.Context, after int64, limit int) ([]journal.Fact, error) {
	var out []journal.Fact

	for _, fact := range f.facts {
		if fact.Seq > after {
			out = append(out, fact)
		}
	}

	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}

	return out, nil
}

func factAt(seq int64, taskID string, factType journal.FactType, at time.Time) journal.Fact {
	return journal.Fact{Seq: seq, Time: at, TaskID: taskID, Type: factType}
}

func testFacts() []journal.Fact {
	at := time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)

	return []journal.Fact{
		{
			Seq: 1, Time: at, TaskID: "000001a0deadbeef", Type: journal.Enqueued,
			Owner: "harvest", Attempt: 0,
		},
		{
			Seq: 2, Time: at.Add(time.Second), TaskID: "000001a0deadbeef", Type: journal.Claimed,
			Owner: "pool-1",
		},
		{
			Seq: 3, Time: at.Add(2 * time.Second), TaskID: "000001a0deadbeef", Type: journal.Failed,
			Attempt: 1, Error: "exit status 1",
			Detail: jsontext.Value(`{"stage":"verify","exit_code":1,"tail":"boom"}`),
		},
		{
			Seq: 4, Time: at.Add(3 * time.Second), TaskID: "session:abc123", Type: journal.SessionOpened,
		},
	}
}

func TestReadAllMapsFactsInSeqOrder(t *testing.T) {
	j := NewFactJournal(&fakeSource{facts: testFacts()})

	events, err := j.ReadAll(context.Background())
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	if len(events) != 4 {
		t.Fatalf("ReadAll returned %d events, want 4", len(events))
	}

	prev := ""

	for i, evt := range events {
		fact := journal.FactType(evt.Type())
		if fact == "" {
			t.Fatalf("event %d has empty type", i)
		}

		id := evt.ID().String()
		if prev != "" && id <= prev {
			t.Fatalf("event ids not strictly increasing: %s after %s", id, prev)
		}

		prev = id

		if uint64(evt.Version()) != uint64(i+1) {
			t.Fatalf("event %d version = %d, want %d", i, evt.Version(), i+1)
		}
	}

	first := events[0]
	if first.StreamID().String() != "000001a0deadbeef" {
		t.Fatalf("first event stream id = %s", first.StreamID())
	}

	if string(first.StreamType()) != StreamTypeTask {
		t.Fatalf("first event stream type = %s, want %s", first.StreamType(), StreamTypeTask)
	}

	if !first.OccurredAt().Equal(time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("first event occurred at %v", first.OccurredAt())
	}

	if string(first.Type()) != string(journal.Enqueued) {
		t.Fatalf("first event type = %s, want %s", first.Type(), journal.Enqueued)
	}
}

func TestPayloadRoundTrip(t *testing.T) {
	j := NewFactJournal(&fakeSource{facts: testFacts()})

	events, err := j.ReadAll(context.Background())
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	var got factPayload
	if err := json.Unmarshal(events[2].Payload(), &got); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	want := factPayload{
		TaskID:  "000001a0deadbeef",
		Type:    journal.Failed,
		Attempt: 1,
		Error:   "exit status 1",
		Detail:  jsontext.Value(`{"stage":"verify","exit_code":1,"tail":"boom"}`),
	}
	if got.TaskID != want.TaskID || got.Type != want.Type || got.Attempt != want.Attempt ||
		got.Error != want.Error || string(got.Detail) != string(want.Detail) {
		t.Fatalf("payload = %+v, want %+v", got, want)
	}

	if len(events[0].Payload()) == 0 {
		t.Fatal("enqueued event payload is empty")
	}
}

func TestReadFromDrainsFromZeroCursorInBatches(t *testing.T) {
	j := NewFactJournal(&fakeSource{facts: testFacts()})
	ctx := context.Background()

	var (
		cursor  id.EventID
		visited []int64
	)

	for {
		events, err := j.ReadFrom(ctx, cursor, 2)
		if err != nil {
			t.Fatalf("ReadFrom: %v", err)
		}

		if len(events) > 2 {
			t.Fatalf("ReadFrom returned %d events, limit 2", len(events))
		}

		if len(events) == 0 {
			break
		}

		for _, evt := range events {
			seq, ok := seqFromEventID(evt.ID())
			if !ok {
				t.Fatalf("event id %s is not sequence-derived", evt.ID())
			}

			visited = append(visited, seq)
			cursor = evt.ID()
		}
	}

	if len(visited) != 4 {
		t.Fatalf("drained %d events, want 4", len(visited))
	}

	for i, seq := range visited {
		if seq != int64(i+1) {
			t.Fatalf("visited[%d] = %d, want %d", i, seq, i+1)
		}
	}
}

func TestReadFromResumesAfterCursor(t *testing.T) {
	j := NewFactJournal(&fakeSource{facts: testFacts()})
	ctx := context.Background()

	all, err := j.ReadAll(ctx)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	events, err := j.ReadFrom(ctx, all[1].ID(), 0)
	if err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("ReadFrom returned %d events, want 2 (seq 3 and 4)", len(events))
	}

	if uint64(events[0].Version()) != 3 || uint64(events[1].Version()) != 4 {
		t.Fatalf("resume returned versions %d, %d; want 3, 4", events[0].Version(), events[1].Version())
	}
}

func TestReadFromForeignCursorDrainsEmpty(t *testing.T) {
	j := NewFactJournal(&fakeSource{facts: testFacts()})

	events, err := j.ReadFrom(context.Background(), id.NewEventID(), 10)
	if err != nil {
		t.Fatalf("ReadFrom with foreign cursor: %v", err)
	}

	if len(events) != 0 {
		t.Fatalf("foreign cursor drained %d events, want 0", len(events))
	}
}

func TestSessionFactsUseSessionStreamType(t *testing.T) {
	j := NewFactJournal(&fakeSource{facts: testFacts()})

	events, err := j.ReadAll(context.Background())
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	last := events[3]
	if string(last.StreamType()) != StreamTypeSession {
		t.Fatalf("session fact stream type = %s, want %s", last.StreamType(), StreamTypeSession)
	}

	if last.StreamID().String() != "session:abc123" {
		t.Fatalf("session fact stream id = %s", last.StreamID())
	}

	if string(last.Type()) != string(journal.SessionOpened) {
		t.Fatalf("session fact type = %s", last.Type())
	}
}

func TestNonPositiveSeqRejected(t *testing.T) {
	if _, err := factEvent(factAt(0, "task1", journal.Enqueued, time.Now())); err == nil {
		t.Fatal("factEvent accepted a fact with seq 0")
	}

	if _, err := seqEventID(-1); err == nil {
		t.Fatal("seqEventID accepted a negative sequence")
	}

	if _, err := seqEventID(0); err == nil {
		t.Fatal("seqEventID accepted sequence 0")
	}
}

func TestSeqEventIDRoundTripAndOrdering(t *testing.T) {
	for _, seq := range []int64{1, 2, 42, 1 << 20, 1 << 40, (int64(1) << 62) + 12345} {
		eventID, err := seqEventID(seq)
		if err != nil {
			t.Fatalf("seqEventID(%d): %v", seq, err)
		}

		got, ok := seqFromEventID(eventID)
		if !ok {
			t.Fatalf("seqFromEventID(%s) rejected its own id", eventID)
		}

		if got != seq {
			t.Fatalf("round trip: got %d, want %d", got, seq)
		}
	}

	prev := ""

	for seq := int64(1); seq <= 500; seq++ {
		eventID, err := seqEventID(seq)
		if err != nil {
			t.Fatalf("seqEventID(%d): %v", seq, err)
		}

		id := eventID.String()
		if !strings.HasPrefix(id, "0") {
			t.Fatalf("synthetic id %s lacks the zero-epoch first char", id)
		}

		if prev != "" && id <= prev {
			t.Fatalf("ids not strictly increasing: %s after %s", id, prev)
		}

		prev = id
	}
}

func TestLimitZeroMeansUnlimited(t *testing.T) {
	j := NewFactJournal(&fakeSource{facts: testFacts()})

	events, err := j.ReadFrom(context.Background(), id.EventID{}, 0)
	if err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	if len(events) != 4 {
		t.Fatalf("ReadFrom returned %d events, want 4", len(events))
	}
}

func TestSliceSource(t *testing.T) {
	src := NewSliceSource(testFacts())
	ctx := context.Background()

	all, err := src.Facts(ctx, 0, 0)
	if err != nil || len(all) != 4 {
		t.Fatalf("Facts(all) = %d facts, err %v; want 4, nil", len(all), err)
	}

	window, err := src.Facts(ctx, 1, 2)
	if err != nil {
		t.Fatalf("Facts(after 1): %v", err)
	}

	if len(window) != 2 || window[0].Seq != 2 || window[1].Seq != 3 {
		t.Fatalf("Facts(after 1, limit 2) = seqs %d,%d; want 2,3", window[0].Seq, window[1].Seq)
	}

	empty, err := src.Facts(ctx, 4, 0)
	if err != nil || len(empty) != 0 {
		t.Fatalf("Facts(after head) = %d facts, err %v; want 0, nil", len(empty), err)
	}
}

func TestSourceErrorPropagates(t *testing.T) {
	boom := errors.New("disk on fire")

	j := NewFactJournal(failingSource{err: boom})

	if _, err := j.ReadAll(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("ReadAll error = %v, want disk on fire", err)
	}

	if _, err := j.ReadFrom(context.Background(), id.EventID{}, 0); !errors.Is(err, boom) {
		t.Fatalf("ReadFrom error = %v, want disk on fire", err)
	}
}

type failingSource struct{ err error }

func (f failingSource) Facts(context.Context, int64, int) ([]journal.Fact, error) {
	return nil, fmt.Errorf("failing: %w", f.err)
}

func TestInterfaceCompliance(t *testing.T) {
	var (
		journalAdapter event.Journal         = NewFactJournal(&fakeSource{})
		seekable       event.SeekableJournal = NewFactJournal(&fakeSource{})
	)

	_ = journalAdapter
	_ = seekable
}
