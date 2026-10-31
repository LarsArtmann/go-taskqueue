package readmodel

import (
	"slices"
	"testing"
)

// TestTransitionDeltaTable pins the counter transition helpers at the
// helper level (TODO row 684): they were covered only INDIRECTLY via the
// parity lifecycle test, and the empty-from guard is exactly the shape
// the closeout §d2 bug class broke once — a seed enqueue (no prior
// status) must carry +1 only, never a spurious -1 under "".
func TestTransitionDeltaTable(t *testing.T) {
	t.Parallel()

	t.Run("empty from-status is a seed enqueue", func(t *testing.T) {
		t.Parallel()

		got := transitionDelta("pending", "")
		if got["pending"] != 1 || len(got) != 1 {
			t.Fatalf("transitionDelta(pending, \"\") = %v, want {pending:1} only", got)
		}
	})

	t.Run("transition moves one out, one in", func(t *testing.T) {
		t.Parallel()

		got := transitionDelta("running", "pending")
		if got["running"] != 1 || got["pending"] != -1 || len(got) != 2 {
			t.Fatalf("transitionDelta(running, pending) = %v, want {running:1 pending:-1}", got)
		}
	})

	t.Run("same-status transition nets zero per key", func(t *testing.T) {
		t.Parallel()

		got := transitionDelta("running", "running")
		if got["running"] != 0 || len(got) != 1 {
			t.Fatalf("transitionDelta(running, running) = %v, want {running:0}", got)
		}
	})
}

// TestProjectTransitionDeltaTable pins the project-scoped key shape: the
// lifecycle rows the fold feeds it — seed enqueue, a rescue re-enqueue
// (task.enqueued carrying From=dead, the dead count must drop), and a
// dismiss (task.cancelled from dead).
func TestProjectTransitionDeltaTable(t *testing.T) {
	t.Parallel()

	keyFor := func(project, status string) string { return project + projectStatusKeySep + status }

	t.Run("seed enqueue has no from key", func(t *testing.T) {
		t.Parallel()

		got := projectTransitionDelta("api", "pending", "")
		if len(got) != 1 || got[keyFor("api", "pending")] != 1 {
			t.Fatalf("seed delta = %v, want only +1 on api|pending", got)
		}
	})

	t.Run("rescue re-enqueue walks dead back to pending", func(t *testing.T) {
		t.Parallel()

		got := projectTransitionDelta("api", "pending", "dead")
		if got[keyFor("api", "pending")] != 1 || got[keyFor("api", "dead")] != -1 || len(got) != 2 {
			t.Fatalf("rescue delta = %v, want +1 api|pending and -1 api|dead", got)
		}
	})

	t.Run("dismiss walks dead to cancelled", func(t *testing.T) {
		t.Parallel()

		got := projectTransitionDelta("api", "cancelled", "dead")
		if got[keyFor("api", "cancelled")] != 1 || got[keyFor("api", "dead")] != -1 {
			t.Fatalf("dismiss delta = %v, want +1 api|cancelled and -1 api|dead", got)
		}
	})

	t.Run("keys stay project-scoped (no bare status leakage)", func(t *testing.T) {
		t.Parallel()

		got := projectTransitionDelta("api", "pending", "dead")

		for k := range got {
			if !slices.Contains([]string{keyFor("api", "pending"), keyFor("api", "dead")}, k) {
				t.Fatalf("delta carried a foreign key %q: %v", k, got)
			}
		}
	})
}
