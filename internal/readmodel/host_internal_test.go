package readmodel

import (
	"context"
	"encoding/json/jsontext"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	cqrs "github.com/larsartmann/go-taskqueue/internal/journal/cqrs"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestTailSubscriberDeliversEveryFactAcrossPollBoundaries pins the tail
// cursor contract behind the webui smoke's red class: ReadFrom is
// exclusive of the given event ID (the journal's AfterSeq), so the poll
// cursor must be the LAST delivered seq (tailAnchor). An after+1 cursor
// skips the fact at seq after+1 at every poll boundary; a skipped
// terminal fact (Completed, DeadLettered) wedges the fold's ledger and
// the dashboard's stats assertion never converges.
func TestTailSubscriberDeliversEveryFactAcrossPollBoundaries(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store, err := sqlite.Open(t.TempDir() + "/queue.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	enqueue := func(payload string) {
		t.Helper()

		if _, err := store.Enqueue(ctx, task.New{
			Project: "tail",
			Type:    "sh",
			Payload: jsontext.Value(`"` + payload + `"`),
		}); err != nil {
			t.Fatalf("enqueue %s: %v", payload, err)
		}
	}

	enqueue("one")
	enqueue("two")

	if err := store.SaveWatermark(ctx, CursorConsumer, 2); err != nil {
		t.Fatalf("save anchor watermark: %v", err)
	}

	s := tailSubscriber{jr: cqrs.NewFactJournal(store), src: store, poll: 10 * time.Millisecond}

	var (
		mu    sync.Mutex
		delim []int64
	)

	done := make(chan struct{})

	handler := event.Handler(func(_ context.Context, evt event.Event) error {
		seq, ok := EventIDToSeq(evt.ID())
		if !ok {
			t.Errorf("delivered non-sequence event id %s", evt.ID())

			return nil
		}

		mu.Lock()
		delim = append(delim, seq)
		complete := len(delim) == 3
		mu.Unlock()

		if complete {
			close(done)
		}

		return nil
	})

	if err := s.SubscribeAll(handler); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	for _, payload := range []string{"three", "four", "five"} {
		time.Sleep(3 * s.poll)
		enqueue(payload)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("tail never delivered all facts; delivered %v", deliveredSeqs(&mu, &delim))
	}

	if got := deliveredSeqs(&mu, &delim); !slices.Equal(got, []int64{3, 4, 5}) {
		t.Fatalf("delivered %v, want [3 4 5] (no skips, no duplicates, in order)", got)
	}
}

func deliveredSeqs(mu *sync.Mutex, delim *[]int64) []int64 {
	mu.Lock()
	defer mu.Unlock()

	return slices.Clone(*delim)
}
