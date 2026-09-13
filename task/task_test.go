package task_test

import (
	"errors"
	"testing"

	tq "github.com/larsartmann/go-taskqueue/task"
)

func TestFacadeSurface(t *testing.T) {
	if got := tq.AllStatuses(); len(got) != 5 {
		t.Fatalf("AllStatuses = %d statuses, want 5", len(got))
	}
	if !tq.CanTransitionTo(tq.Pending, tq.Running) {
		t.Fatal("pending → running must be legal")
	}
	if tq.Terminal(tq.Dead) != true || tq.Terminal(tq.Running) != false {
		t.Fatal("Terminal disagrees with the lifecycle")
	}
	if tq.DefaultMaxAttempts != 3 {
		t.Fatalf("DefaultMaxAttempts = %d, want 3", tq.DefaultMaxAttempts)
	}
	for _, err := range []error{tq.ErrNotFound, tq.ErrLeaseNotHeld, tq.ErrDuplicateID, tq.ErrInvalidTransition} {
		if err == nil {
			t.Fatal("sentinel error must be re-exported")
		}
	}
	if !errors.Is(tq.ErrNotFound, tq.ErrNotFound) {
		t.Fatal("sentinel identity broken")
	}
	if tq.NewID() == "" {
		t.Fatal("NewID returned an empty id")
	}
}
