package budget

import (
	"context"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
)

// seeded returns a journal with n enqueued facts today and m yesterday.
func seeded(nToday, nYesterday int) factSource {
	j := journal.NewMemoryJournal()
	ctx := context.Background()
	for i := 0; i < nToday+nYesterday; i++ {
		f := journal.Fact{TaskID: "t", Type: journal.Enqueued}
		if i >= nToday {
			f.Time = time.Now().Add(-24 * time.Hour)
		}
		_, _ = j.Append(ctx, f)
	}
	return factSource{j}
}

// factSource adapts MemoryJournal (Since) to the guard's Facts view, the
// same mapping queue.Store.Facts uses.
type factSource struct{ j *journal.MemoryJournal }

func (m factSource) Facts(ctx context.Context, after int64) ([]journal.Fact, error) {
	return m.j.Since(ctx, after)
}

// TestSpentTodayMatchesFacts pins the spend projection: one enqueued task
// = one unit of spend, counted from journal facts, scoped to the local day.
func TestSpentTodayMatchesFacts(t *testing.T) {
	ctx := context.Background()
	j := seeded(3, 4)
	if got := (Guard{}).SpentToday(ctx, j); got != 3 {
		t.Fatalf("spent today = %d, want 3 (yesterday's 4 excluded)", got)
	}

	// Non-enqueue facts never count as spend.
	j2 := journal.NewMemoryJournal()
	_, _ = j2.Append(ctx, journal.Fact{TaskID: "t", Type: journal.Completed})
	_, _ = j2.Append(ctx, journal.Fact{TaskID: "t", Type: journal.DeadLettered})
	if got := (Guard{}).SpentToday(ctx, factSource{j2}); got != 0 {
		t.Fatalf("completed/dead facts must not count as spend, got %d", got)
	}
}

func TestCheckRefusesAtDailyCap(t *testing.T) {
	ctx := context.Background()
	g := Guard{DailyCap: 2}

	if ok, reason := g.Check(ctx, seeded(0, 9)); !ok || reason != "" {
		t.Fatalf("yesterday's spend must not gate today, got ok=%v reason=%q", ok, reason)
	}
	if ok, reason := g.Check(ctx, seeded(1, 0)); !ok || reason != "" {
		t.Fatalf("under cap must allow, got ok=%v reason=%q", ok, reason)
	}
	ok, reason := g.Check(ctx, seeded(2, 0))
	if ok {
		t.Fatal("at cap must refuse")
	}
	if reason == "" {
		t.Fatal("refusal must carry a human-readable reason")
	}
}

func TestCheckBudgetCmdAuthority(t *testing.T) {
	ctx := context.Background()

	if ok, _ := (Guard{BudgetCmd: "exit 0"}).Check(ctx, seeded(0, 0)); !ok {
		t.Fatal("exit 0 must allow")
	}
	ok, reason := (Guard{BudgetCmd: `echo "org budget spent" >&2; exit 3`}).Check(ctx, seeded(0, 0))
	if ok {
		t.Fatal("non-zero exit must refuse")
	}
	if reason != "budget command refused: org budget spent" {
		t.Fatalf("reason = %q, want the command's output line", reason)
	}

	// The command is the final authority: even with cap 0 (unlimited),
	// its refusal holds.
	if ok, _ := (Guard{BudgetCmd: "exit 1"}).Check(ctx, seeded(0, 0)); ok {
		t.Fatal("budget command must be the final authority")
	}
}
