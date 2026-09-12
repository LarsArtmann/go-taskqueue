package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func newTestStore(t *testing.T) *sqlite.Store {
	t.Helper()

	s, err := sqlite.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	return s
}

func fact(seq int64) journal.Fact {
	return journal.Fact{Seq: seq, TaskID: "t", Type: journal.Enqueued}
}

// TestDeliversEveryFactInOrder pins the exact-consumer contract: facts
// arrive in Seq order, none skipped, cursor advanced only past delivered
// facts — including a burst far larger than one page.
func TestDeliversEveryFactInOrder(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	var (
		mu   sync.Mutex
		got  []int64
		seen = map[int64]bool{}
	)

	d := New(s, Config{PollInterval: 2 * time.Millisecond, PageSize: 10})

	unsub := d.Subscribe("order", 0, func(_ context.Context, f journal.Fact) error {
		mu.Lock()
		defer mu.Unlock()

		if seen[f.Seq] {
			t.Errorf("fact %d delivered twice", f.Seq)
		}

		seen[f.Seq] = true
		got = append(got, f.Seq)

		return nil
	})
	defer unsub()

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)

	go func() { done <- d.Run(runCtx) }()

	for i := range 35 { // 3.5 pages at PageSize 10
		if _, err := s.Enqueue(ctx, mkTask(i)); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}

	deadline := time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(got)
		mu.Unlock()

		if n == 35 {
			break
		}

		time.Sleep(2 * time.Millisecond)
	}

	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()

	if len(got) != 35 {
		t.Fatalf("delivered %d facts, want 35", len(got))
	}

	for i, seq := range got {
		if seq != int64(i+1) {
			t.Fatalf("delivery out of order: got[%d] = %d", i, seq)
		}
	}

	if cursor, ok := d.Cursor("order"); !ok || cursor != 35 {
		t.Fatalf("cursor = %d/%v, want 35/true", cursor, ok)
	}
}

// TestHandlerErrorPausesOnlyThatSubscriber pins ADR-0009 D2: a failing
// handler applies backpressure to its own drain — the failing fact
// redelivers until accepted and never advances the cursor past itself —
// while other subscribers keep flowing.
func TestHandlerErrorPausesOnlyThatSubscriber(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	var healthyMu sync.Mutex

	healthyDelivered := 0

	flakyFail := true

	var flakyMu sync.Mutex

	flakySeen := []int64{}

	d := New(s, Config{PollInterval: 2 * time.Millisecond, PageSize: 10})

	d.Subscribe("healthy", 0, func(_ context.Context, _ journal.Fact) error {
		healthyMu.Lock()
		defer healthyMu.Unlock()

		healthyDelivered++

		return nil
	})

	d.Subscribe("flaky", 0, func(_ context.Context, f journal.Fact) error {
		flakyMu.Lock()
		defer flakyMu.Unlock()

		flakySeen = append(flakySeen, f.Seq)

		if flakyFail && f.Seq == 2 {
			return errors.New("subscriber down")
		}

		return nil
	})

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)

	go func() { done <- d.Run(runCtx) }()

	for i := range 5 {
		if _, err := s.Enqueue(ctx, mkTask(i)); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}

	// The healthy subscriber finishes everything; the flaky one is stuck
	// on fact 2 (redelivered each tick) while its cursor stays at 1.
	deadline := time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) {
		healthyMu.Lock()
		n := healthyDelivered
		healthyMu.Unlock()

		if n >= 5 {
			break
		}

		time.Sleep(2 * time.Millisecond)
	}

	flakySnapshot := func() []int64 {
		flakyMu.Lock()
		defer flakyMu.Unlock()

		return append([]int64(nil), flakySeen...)
	}

	sawFact2 := func() bool {
		for _, seq := range flakySnapshot() {
			if seq == 2 {
				return true
			}
		}

		return false
	}

	// The cursor pin is only meaningful once the pause engaged: tick
	// drains subscribers sequentially, so the healthy count alone can be
	// observed before the flaky drain first reaches fact 2 (cursor still
	// 0 — the 2026-09-13 load transient). Gate on the attempt, then pin.
	deadline = time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) && !sawFact2() {
		time.Sleep(2 * time.Millisecond)
	}

	if !sawFact2() {
		t.Fatalf("flaky handler never attempted fact 2, seen %v", flakySnapshot())
	}

	if cursor, _ := d.Cursor("flaky"); cursor != 1 {
		t.Fatalf("flaky cursor = %d while handler fails, want 1 (never past an unaccepted fact)", cursor)
	}

	// Heal the handler: the drain resumes from the cursor and fact 2 (and
	// the rest) deliver.
	flakyMu.Lock()
	flakyFail = false
	flakyMu.Unlock()

	deadline = time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) {
		if cursor, _ := d.Cursor("flaky"); cursor == 5 {
			break
		}

		time.Sleep(2 * time.Millisecond)
	}

	cancel()
	<-done

	if cursor, _ := d.Cursor("flaky"); cursor != 5 {
		t.Fatalf("flaky cursor after healing = %d, want 5", cursor)
	}

	flakyMu.Lock()
	defer flakyMu.Unlock()

	retries := 0

	for _, seq := range flakySeen {
		if seq == 2 {
			retries++
		}
	}

	if retries < 2 {
		t.Fatalf("fact 2 seen %d times, want >= 2 (at-least-once redelivery)", retries)
	}
}

// TestUnsubscribeStopsDelivery pins the unsubscribe contract.
func TestUnsubscribeStopsDelivery(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if _, err := s.Enqueue(ctx, mkTask(0)); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex

	delivered := 0

	d := New(s, Config{PollInterval: 2 * time.Millisecond})

	unsub := d.Subscribe("gone", 0, func(_ context.Context, _ journal.Fact) error {
		mu.Lock()
		defer mu.Unlock()

		delivered++

		return nil
	})

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)

	go func() { done <- d.Run(runCtx) }()

	deadline := time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) {
		mu.Lock()
		n := delivered
		mu.Unlock()

		if n > 0 {
			break
		}

		time.Sleep(2 * time.Millisecond)
	}

	unsub()

	if _, err := s.Enqueue(ctx, mkTask(99)); err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()

	if delivered != 1 {
		t.Fatalf("delivered %d facts after unsubscribe, want exactly 1", delivered)
	}
}

// TestLagReportsPerSubscriber pins the observability surface: lag is
// HeadSeq − cursor per subscriber.
func TestLagReportsPerSubscriber(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for i := range 5 {
		if _, err := s.Enqueue(ctx, mkTask(i)); err != nil {
			t.Fatal(err)
		}
	}

	d := New(s, Config{})

	d.Subscribe("caught-up", 5, func(_ context.Context, _ journal.Fact) error { return nil })
	d.Subscribe("behind", 2, func(_ context.Context, _ journal.Fact) error { return nil })

	lag, err := d.Lag(ctx)
	if err != nil {
		t.Fatalf("Lag: %v", err)
	}

	if lag["caught-up"] != 0 {
		t.Errorf("caught-up lag = %d, want 0", lag["caught-up"])
	}

	if lag["behind"] != 3 {
		t.Errorf("behind lag = %d, want 3", lag["behind"])
	}
}

func mkTask(i int) task.New {
	return task.New{
		Type:    "sh",
		Project: "consumer-test",
		Payload: json.RawMessage(`"echo hi"`),
	}
}
