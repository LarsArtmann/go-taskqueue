package queue_test

import (
	"errors"
	"testing"

	tq "github.com/larsartmann/go-taskqueue/queue"
)

func TestFacadeSurface(t *testing.T) {
	if tq.BandOf(50) != tq.BandBacklog || tq.BandOf(120) != tq.BandHot || tq.BandOf(200) != tq.BandMachine {
		t.Fatal("band boundaries drifted from ADR-0015")
	}

	if got := tq.ClampBacklog(150); got != tq.BacklogMax {
		t.Fatalf("ClampBacklog(150) = %d, want %d", got, tq.BacklogMax)
	}

	if tq.PriorityAgingDaysPerPoint != 3 || tq.PriorityAgingMaxBonus != 10 {
		t.Fatal("aging constants drifted from ADR-0015")
	}

	if tq.UnblockBumpPriority != 15 {
		t.Fatal("unblock bump drifted")
	}

	for _, err := range []error{tq.ErrNoTaskDue, tq.ErrEmptyType} {
		if err == nil || !errors.Is(err, err) {
			t.Fatal("sentinel error must be re-exported")
		}
	}
}
