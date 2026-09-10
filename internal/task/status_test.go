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

// TestStatusTableExhaustive pins that every declared Status has a row in
// the transitions table and agrees with Valid. A new Status added without
// wiring fails here instead of silently accepting/rejecting at runtime.
func TestStatusTableExhaustive(t *testing.T) {
	declared := []Status{Pending, Running, Completed, Dead, Cancelled}
	if len(transitions) != len(declared) {
		t.Fatalf(
			"transitions table has %d source states, want %d (new Status without a transitions row?)",
			len(transitions),
			len(declared),
		)
	}

	for _, s := range declared {
		if _, ok := transitions[s]; !ok {
			t.Errorf("status %q missing from transitions table", s)
		}

		if !s.Valid() {
			t.Errorf("declared status %q reports Valid() = false", s)
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
