package queue

import (
	"context"
	"errors"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// stubStore satisfies Store by embedding the interface and overriding only
// Enqueue — the done-guard lives entirely in the Queue wrapper, so the stub
// pins the layering: the wrapper refuses, the store never has to.
type stubStore struct {
	Store

	returned task.Task
	err      error
}

func (s *stubStore) Enqueue(ctx context.Context, n task.New) (task.Task, error) {
	return s.returned, s.err
}

func TestEnqueueDoneGuardRefusesCompleted(t *testing.T) {
	q := New(&stubStore{returned: task.Task{ID: "t1", Status: task.Completed}})

	got, err := q.Enqueue(context.Background(), task.New{Type: "agent", DedupKey: "k"})
	if !errors.Is(err, ErrTaskDone) {
		t.Fatalf("Enqueue completed key: err = %v, want ErrTaskDone", err)
	}

	if got.ID != "t1" {
		t.Fatalf("Enqueue completed key: returned task %q, want the stored row alongside the refusal", got.ID)
	}
}

func TestEnqueueDoneGuardOnlyFiresOnCompleted(t *testing.T) {
	for _, status := range []task.Status{task.Pending, task.Running, task.Dead, task.Cancelled} {
		q := New(&stubStore{returned: task.Task{ID: "t1", Status: status}})

		if _, err := q.Enqueue(context.Background(), task.New{Type: "agent", DedupKey: "k"}); err != nil {
			t.Fatalf("Enqueue %s key: err = %v, want nil (suppress-and-return stays)", status, err)
		}
	}
}

func TestEnqueueDoneGuardPassesStoreErrorsThrough(t *testing.T) {
	storeErr := errors.New("disk on fire")
	q := New(&stubStore{err: storeErr})

	if _, err := q.Enqueue(context.Background(), task.New{Type: "agent"}); !errors.Is(err, storeErr) {
		t.Fatalf("Enqueue store failure: err = %v, want the store error", err)
	}
}
