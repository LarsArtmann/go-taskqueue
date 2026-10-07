package consumer

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
)

// TestWakeDrainsBeforePoll pins the drain-wake seam: a dispatcher parked on
// a ten-second poll interval delivers a freshly enqueued fact within the
// wake bound because the store's Notify channel (queue.Waker) fires the
// drain. The ticker stays the degraded fallback — the poll-only delivery
// tests pin that path with a nil Wake.
func TestWakeDrainsBeforePoll(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	var mu sync.Mutex

	var got int

	d := New(s, Config{
		PollInterval: 10 * time.Second, // without wake: no delivery for 10s
		PageSize:     10,
		Wake:         s.Notify(),
	})

	unsub := d.Subscribe("wake", 0, func(_ context.Context, _ journal.Fact) error {
		mu.Lock()
		defer mu.Unlock()

		got++

		return nil
	})
	defer unsub()

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)

	go func() { done <- d.Run(runCtx) }()

	// Let the dispatcher settle (any spurious wake from store-open writes
	// drains harmlessly over empty facts), then enqueue and bound delivery.
	time.Sleep(50 * time.Millisecond)

	if _, err := s.Enqueue(ctx, mkTask(0)); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	delivered := func() bool {
		mu.Lock()
		defer mu.Unlock()

		return got >= 1
	}

	deadline := time.Now().Add(2 * time.Second)

	for !delivered() && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}

	if !delivered() {
		t.Fatal("wake-driven drain never delivered the enqueued fact within 2s (poll interval is 10s)")
	}

	cancel()
	<-done
}
