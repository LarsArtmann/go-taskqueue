package task

import "testing"

func TestCanTransitionTo(t *testing.T) {
	legal := []struct{ from, to Status }{
		{Pending, Running},
		{Pending, Cancelled},
		{Running, Pending},   // retry / lease expiry release
		{Running, Completed}, // success
		{Running, Dead},      // exhausted retries
		{Dead, Pending},      // DLQ rescue
	}
	for _, tr := range legal {
		if !CanTransitionTo(tr.from, tr.to) {
			t.Errorf("CanTransitionTo(%s, %s) = false, want true", tr.from, tr.to)
		}
	}
	illegal := []struct{ from, to Status }{
		{Pending, Completed},       // never ran
		{Pending, Dead},            // never ran
		{Completed, Pending},       // terminal
		{Completed, Running},       // terminal
		{Dead, Running},            // dead must go through pending
		{Cancelled, Pending},       // terminal
		{Cancelled, Running},       // terminal
		{Running, Running},         // no self-loop
		{Status("bogus"), Pending}, // unknown
		{Pending, Status("bogus")}, // unknown
	}
	for _, tr := range illegal {
		if CanTransitionTo(tr.from, tr.to) {
			t.Errorf("CanTransitionTo(%s, %s) = true, want false", tr.from, tr.to)
		}
	}
}

func TestTerminal(t *testing.T) {
	for _, s := range []Status{Completed, Dead, Cancelled} {
		if !Terminal(s) {
			t.Errorf("Terminal(%s) = false, want true", s)
		}
	}
	for _, s := range []Status{Pending, Running} {
		if Terminal(s) {
			t.Errorf("Terminal(%s) = true, want false", s)
		}
	}
}

func TestNewID(t *testing.T) {
	seen := make(map[ID]struct{})
	for range 1000 {
		id := NewID()
		if len(id) != 16+20 {
			t.Fatalf("NewID length = %d, want 36", len(id))
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("NewID duplicated: %s", id)
		}
		seen[id] = struct{}{}
	}
}
