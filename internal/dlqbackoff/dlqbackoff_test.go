package dlqbackoff

import (
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestDeadlinesNilWindow pins the off switch: window 0 (the surface is
// not configured) yields nil without consulting any fact.
func TestDeadlinesNilWindow(t *testing.T) {
	t.Parallel()

	facts := []journal.Fact{{TaskID: "t1", Type: journal.DeadLettered, Time: time.Now()}}

	if got := Deadlines(facts, map[task.ID]string{"t1": "repo"}, time.Now(), 0); got != nil {
		t.Fatalf("Deadlines = %v, want nil with the window off", got)
	}

	if got := Deadlines(facts, map[task.ID]string{"t1": "repo"}, time.Now(), -time.Minute); got != nil {
		t.Fatalf("Deadlines = %v, want nil with a negative window", got)
	}
}

// TestDeadlinesFreshDeadPoisons pins the core derivation: a fresh
// still-standing dead letter with no completion on record puts the repo
// in backoff until exactly lastDead+window, and only that repo.
func TestDeadlinesFreshDeadPoisons(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 10, 28, 4, 0, 0, 0, time.UTC)
	now := base.Add(10 * time.Minute)
	window := time.Hour

	facts := []journal.Fact{
		{TaskID: "t-dead", Type: journal.DeadLettered, Time: base},
		{TaskID: "t-other", Type: journal.Enqueued, Time: base},
	}
	projects := map[task.ID]string{"t-dead": "poisoned", "t-other": "healthy"}

	deadlines := Deadlines(facts, projects, now, window)

	until, ok := deadlines["poisoned"]
	if !ok {
		t.Fatal("poisoned repo missing from the deadlines despite a fresh standing dead letter")
	}

	if !until.Equal(base.Add(window)) {
		t.Fatalf("deadline = %v, want exactly lastDead+window %v", until, base.Add(window))
	}

	if _, ok := deadlines["healthy"]; ok {
		t.Fatal("healthy repo in the deadlines with no dead letter")
	}
}

// TestDeadlinesBoundaryInstant pins the expiry boundary exactly, over a
// fixed clock: at until == now the backoff has EXPIRED (absent) while
// 1ns before the boundary it is still ACTIVE (exactly lastDead+window)
// — a future >= boundary refactor now fails loudly instead of silently
// shifting the poisoned verdict by one instant (closeout 2026-10-28
// 04-18 §f4; moved with the fold to its shared home 2026-10-28).
func TestDeadlinesBoundaryInstant(t *testing.T) {
	t.Parallel()

	const deadTask = "t-boundary"

	base := time.Date(2026, 10, 28, 4, 0, 0, 0, time.UTC)
	window := time.Hour
	until := base.Add(window)

	facts := []journal.Fact{{TaskID: deadTask, Type: journal.DeadLettered, Time: base}}
	projects := map[task.ID]string{deadTask: "poisoned"}

	for _, tc := range []struct {
		name    string
		now     time.Time
		wantSet bool
	}{
		{"at the boundary instant the backoff has expired", until, false},
		{"1ns before the boundary the backoff is active", until.Add(-time.Nanosecond), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deadlines := Deadlines(facts, projects, tc.now, window)

			got, ok := deadlines["poisoned"]
			if !tc.wantSet {
				if ok {
					t.Fatalf("deadline = %v, want absent at the boundary instant (expired)", got)
				}

				return
			}

			if !ok || !got.Equal(until) {
				t.Fatalf("deadline = %v (present: %v), want exactly %v (still active)", got, ok, until)
			}
		})
	}
}

// TestDeadlinesDismissalHeals pins the dismissal off-ramp: a Cancelled
// fact on the dead-lettered task (the DLQ-autopsy path) takes the dead
// letter out of the derivation and the repo drops out.
func TestDeadlinesDismissalHeals(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 10, 28, 4, 0, 0, 0, time.UTC)

	facts := []journal.Fact{
		{TaskID: "t-dead", Type: journal.DeadLettered, Time: base},
		{TaskID: "t-dead", Type: journal.Cancelled, Time: base.Add(5 * time.Minute)},
	}
	projects := map[task.ID]string{"t-dead": "healed"}

	if deadlines := Deadlines(facts, projects, base.Add(10*time.Minute), time.Hour); len(deadlines) != 0 {
		t.Fatalf("deadlines = %v, want empty after dismissal", deadlines)
	}
}

// TestDeadlinesCompletionHeals pins the completion off-ramp: any
// completion ever heals the repo, exactly like the harvester's
// ever-completed check (the shipped repo-wide semantics — not
// per-dead-letter ordering).
func TestDeadlinesCompletionHeals(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 10, 28, 4, 0, 0, 0, time.UTC)

	facts := []journal.Fact{
		{TaskID: "t-dead", Type: journal.DeadLettered, Time: base},
		{TaskID: "t-done", Type: journal.Completed, Time: base.Add(time.Minute)},
	}
	projects := map[task.ID]string{"t-dead": "rescued", "t-done": "rescued"}

	if deadlines := Deadlines(facts, projects, base.Add(10*time.Minute), time.Hour); len(deadlines) != 0 {
		t.Fatalf("deadlines = %v, want empty after any completion", deadlines)
	}
}

// TestDeadlinesLatestStandingDeadWins pins the latest-letter rule:
// between two still-standing dead letters the later one sets the
// deadline, and dismissing only the later one falls back to the
// earlier letter (the repo stays poisoned — dismissal heals its own
// task, not the repo's history).
func TestDeadlinesLatestStandingDeadWins(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 10, 28, 4, 0, 0, 0, time.UTC)
	now := base.Add(10 * time.Minute)
	window := time.Hour

	facts := []journal.Fact{
		{TaskID: "t-early", Type: journal.DeadLettered, Time: base},
		{TaskID: "t-late", Type: journal.DeadLettered, Time: base.Add(5 * time.Minute)},
	}
	projects := map[task.ID]string{"t-early": "repo", "t-late": "repo"}

	deadlines := Deadlines(facts, projects, now, window)

	if got := deadlines["repo"]; !got.Equal(base.Add(5 * time.Minute).Add(window)) {
		t.Fatalf("deadline = %v, want the LATER dead letter's %v", got, base.Add(5*time.Minute).Add(window))
	}

	laterDismissed := append(facts, journal.Fact{TaskID: "t-late", Type: journal.Cancelled, Time: base.Add(6 * time.Minute)})

	deadlines = Deadlines(laterDismissed, projects, now, window)

	if got := deadlines["repo"]; !got.Equal(base.Add(window)) {
		t.Fatalf("deadline = %v, want the earlier letter's %v once the later is dismissed", got, base.Add(window))
	}
}

// TestDeadlinesSkipsRepolessTasks pins the join contract: facts for
// tasks without a repo in projectByID never poison anything (the thin
// enqueue snapshots carry no project — the tasks table is the source).
func TestDeadlinesSkipsRepolessTasks(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 10, 28, 4, 0, 0, 0, time.UTC)

	facts := []journal.Fact{
		{TaskID: "t-known", Type: journal.DeadLettered, Time: base},
		{TaskID: "t-unknown", Type: journal.DeadLettered, Time: base},
	}
	projects := map[task.ID]string{"t-known": "repo"}

	deadlines := Deadlines(facts, projects, base.Add(time.Minute), time.Hour)

	if len(deadlines) != 1 {
		t.Fatalf("deadlines = %v, want only the repo with a known project", deadlines)
	}
}
