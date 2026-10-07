package sqlitev4

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestNotifyFiresAfterEnqueue pins the Waker contract's happy path: a
// successful enqueue commit leaves exactly one signal on the buffered-1
// channel (M7 — claims wake in microseconds instead of waiting out the
// idle poll gap).
func TestNotifyFiresAfterEnqueue(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "wake.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	waker, ok := any(s).(queue.Waker)
	if !ok {
		t.Fatal("Store must implement queue.Waker")
	}

	select {
	case <-waker.Notify():
		t.Fatal("a fresh store must not carry a stale wake signal")
	default:
	}

	if _, err := s.Enqueue(context.Background(), task.New{Type: "sh", Payload: []byte(`"echo hi"`)}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	select {
	case <-waker.Notify():
	default:
		t.Fatal("enqueue commit must fire the wake signal")
	}
}

// TestNotifyCoalesces: the channel is buffered-1 with non-blocking sends —
// a burst of enqueues leaves ONE pending signal, and the sender never
// blocks on an unread channel (a slow consumer cannot stall a writer).
func TestNotifyCoalesces(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "wake.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)

		for range 5 {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if _, err := s.Enqueue(ctx, task.New{Type: "sh", Payload: []byte(`"x"`)}); err != nil {
				t.Errorf("enqueue: %v", err)
			}

			cancel()
		}
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("burst enqueues must never block on the unread wake channel")
	}

	signals := 0
	for {
		select {
		case <-s.Notify():
			signals++
		default:
			if signals != 1 {
				t.Fatalf("coalesced signals = %d, want exactly 1", signals)
			}

			return
		}
	}
}

// TestNotifyFiresAfterRequeue: a requeue (preflight/gate/rate-limit class)
// returns a task to PENDING — claimable again after its delay — so it
// fires the wake like any other pending-landing commit.
func TestNotifyFiresAfterRequeue(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "wake.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	ctx := context.Background()

	enq, err := s.Enqueue(ctx, task.New{Type: "sh", Payload: []byte(`"x"`)})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	claimed, claim, err := s.ClaimDue(ctx, "w", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	drain := func() {
		for {
			select {
			case <-s.Notify():
			default:
				return
			}
		}
	}

	drain() // the enqueue's own signal

	if err := s.Requeue(ctx, claimed.ID, claim, "gate slow", 0, false, queue.RequeueClassGate); err != nil {
		t.Fatalf("requeue: %v", err)
	}

	if enq.ID != claimed.ID {
		t.Fatal("claim raced an unexpected task")
	}

	select {
	case <-s.Notify():
	default:
		t.Fatal("requeue commit must fire the wake signal")
	}
}
