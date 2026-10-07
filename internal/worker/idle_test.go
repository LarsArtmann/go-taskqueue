package worker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestIdleGapLadder pins the pure ladder: gaps double per consecutive
// empty claim, saturate at IdlePollMax, and stay at PollInterval when
// disabled, at streak zero, or when the cap sits below the interval.
func TestIdleGapLadder(t *testing.T) {
	base := 250 * time.Millisecond

	ladder := Config{PollInterval: base, IdlePollMax: 2 * time.Second}
	ladder.setDefaults()

	cases := []struct {
		idle int
		want time.Duration
	}{
		{0, base},
		{1, 500 * time.Millisecond},
		{2, time.Second},
		{3, 2 * time.Second},
		{9, 2 * time.Second}, // saturated
	}

	for _, tc := range cases {
		if got := ladder.idleGap(tc.idle); got != tc.want {
			t.Fatalf("idleGap(%d) = %s, want %s", tc.idle, got, tc.want)
		}
	}

	disabled := Config{PollInterval: base, IdlePollMax: -1}
	disabled.setDefaults()

	if got := disabled.idleGap(5); got != base {
		t.Fatalf("disabled idleGap(5) = %s, want %s", got, base)
	}

	lowCap := Config{PollInterval: base, IdlePollMax: 100 * time.Millisecond}
	lowCap.setDefaults()

	if got := lowCap.idleGap(4); got != base {
		t.Fatalf("low-cap idleGap(4) = %s, want the PollInterval floor %s", got, base)
	}
}

// emptyStore counts ClaimDue calls and always reports an empty queue;
// every other Store method is unreachable in this test.
type emptyStore struct {
	queue.Store

	mu    sync.Mutex
	calls []time.Time
}

func (s *emptyStore) ClaimDue(
	_ context.Context, _ string, _ time.Duration,
) (task.Task, queue.Claim, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.calls = append(s.calls, time.Now())

	return task.Task{}, "", queue.ErrNoTaskDue
}

func (s *emptyStore) claimCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.calls)
}

// TestPoolIdleBackoff runs one worker loop against a permanently empty
// store and proves the claim cadence slows down: far fewer polls than the
// base interval alone would produce, and no gap exceeds the cap by more
// than scheduling slack.
func TestPoolIdleBackoff(t *testing.T) {
	store := &emptyStore{}

	cfg := Config{
		Owner:        "idle-test",
		Concurrency:  1,
		PollInterval: 10 * time.Millisecond,
		IdlePollMax:  80 * time.Millisecond,
	}
	cfg.setDefaults()

	pool := New(store, cfg, quietLog())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		_ = pool.Start(ctx)
		close(done)
	}()

	time.Sleep(500 * time.Millisecond)
	cancel()
	<-done

	store.mu.Lock()
	calls := append([]time.Time(nil), store.calls...)
	store.mu.Unlock()

	if len(calls) < 3 {
		t.Fatalf("only %d claims recorded; the loop never ran", len(calls))
	}

	// Fixed 10ms cadence would produce ~50 calls in 500ms; the ladder
	// (10→20→40→80ms cap) must keep it well under that.
	if n := len(calls); n > 20 {
		t.Fatalf("%d claim polls in 500ms; backoff ladder did not slow the loop", n)
	}

	var maxGap time.Duration
	for i := 1; i < len(calls); i++ {
		if gap := calls[i].Sub(calls[i-1]); gap > maxGap {
			maxGap = gap
		}
	}

	if maxGap > 160*time.Millisecond {
		t.Fatalf("max poll gap %s exceeded the 80ms cap beyond scheduling slack", maxGap)
	}
}
