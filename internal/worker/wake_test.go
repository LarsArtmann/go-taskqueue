package worker

import (
	"context"
	"testing"
	"time"
)

// callAt returns the timestamp of the i-th ClaimDue call (0-based), or the
// zero time when that call has not happened yet.
func (s *emptyStore) callAt(i int) time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()

	if i >= len(s.calls) {
		return time.Time{}
	}

	return s.calls[i]
}

// TestPoolWakePreemptsIdleLadder pins the claim-wake seam: a loop parked
// on a ten-second idle gap re-claims within the CI-stable wake bound
// after the store fires the wake channel (design target <50ms; the
// assertion leaves scheduling headroom for CI). The poll ladder stays
// the degraded fallback and is pinned separately by TestPoolIdleBackoff,
// which runs the same harness with a nil Wake.
func TestPoolWakePreemptsIdleLadder(t *testing.T) {
	store := &emptyStore{}

	wake := make(chan struct{}, 1)

	cfg := Config{
		Owner:        "wake-test",
		Concurrency:  1,
		PollInterval: 10 * time.Second, // without wake: one claim per 10s
		IdlePollMax:  10 * time.Second,
		Wake:         wake,
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

	deadline := time.Now().Add(2 * time.Second)
	for store.claimCount() < 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	if store.claimCount() < 1 {
		t.Fatalf("first claim never happened (%d calls)", store.claimCount())
	}

	// Fire the wake; the parked loop must come back for a second claim.
	sent := time.Now()

	wake <- struct{}{}

	for store.claimCount() < 2 {
		if time.Since(sent) > 2*time.Second {
			t.Fatalf("wake did not preempt the 10s idle gap within 2s (%d claims)", store.claimCount())
		}

		time.Sleep(2 * time.Millisecond)
	}

	if latency := store.callAt(1).Sub(sent); latency > 250*time.Millisecond {
		t.Fatalf("wake-to-claim latency %s exceeds the 250ms CI bound (design target 50ms)", latency)
	}

	cancel()
	<-done
}

// TestPoolWakeResetsLadder pins the ladder reset: while wakes arrive at a
// cadence faster than the doubling, every claim gap stays near the base
// interval — the streak never accumulates because each wake returns the
// ladder to zero. Without the reset the ladder would produce 320ms+ gaps
// within this window.
func TestPoolWakeResetsLadder(t *testing.T) {
	store := &emptyStore{}

	wake := make(chan struct{}, 1)

	cfg := Config{
		Owner:        "wake-reset-test",
		Concurrency:  1,
		PollInterval: 40 * time.Millisecond,
		IdlePollMax:  time.Second,
		Wake:         wake,
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

	trainStart := time.Now()

	stopTrain := time.After(600 * time.Millisecond)

train:
	for {
		select {
		case <-stopTrain:
			break train
		case wake <- struct{}{}:
		}

		time.Sleep(80 * time.Millisecond)
	}

	cancel()
	<-done

	store.mu.Lock()
	calls := append([]time.Time(nil), store.calls...)
	store.mu.Unlock()

	var maxGap time.Duration

	for i := 1; i < len(calls); i++ {
		if calls[i].Before(trainStart) || calls[i-1].Before(trainStart) {
			continue // only gaps fully inside the wake-train window count
		}

		if gap := calls[i].Sub(calls[i-1]); gap > maxGap {
			maxGap = gap
		}
	}

	if len(calls) < 5 {
		t.Fatalf("only %d claims recorded; the loop never ran", len(calls))
	}

	// With resets the gaps ride the 80ms wake cadence; the first
	// ladder-only gaps would already reach 320ms mid-window. 400ms leaves
	// scheduling headroom while still separating the two behaviors.
	if maxGap > 400*time.Millisecond {
		t.Fatalf("max in-train claim gap %s: the wake did not reset the idle ladder", maxGap)
	}
}
