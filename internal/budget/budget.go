// Package budget answers one question for unattended pools: may we spend
// more agent money right now? Every enqueued agent task is a real cost, so
// the guard projects spend from the journal's task.enqueued facts and can
// defer to an operator-supplied command for custom ceilings.
package budget

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
)

// FactSource is the journal view the guard projects spend from. *queue.Queue
// satisfies it.
type FactSource interface {
	Facts(ctx context.Context, after int64) ([]journal.Fact, error)
}

// Guard gates how much agent work a pool may start. Zero-value Guard
// allows everything.
type Guard struct {
	// DailyCap is the maximum number of enqueued agent tasks per calendar
	// day (local midnight). 0 = unlimited.
	DailyCap int
	// BudgetCmd, when set, is the final authority: it runs before every
	// harvest tick; exit 0 means within budget, any non-zero exit skips
	// the tick (its output becomes the log reason). Allows operators to
	// plug real cost accounting in.
	BudgetCmd string
	// Now is overridable in tests.
	Now func() time.Time
}

func (g Guard) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now()
}

// Check reports whether the pool may enqueue more work, and if not, a
// human-readable reason for the log.
func (g Guard) Check(ctx context.Context, src FactSource) (bool, string) {
	if g.BudgetCmd != "" {
		cmd := exec.CommandContext(ctx, "sh", "-c", g.BudgetCmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return false, "budget command refused: " + firstLine(string(out))
		}
		return true, ""
	}
	if g.DailyCap > 0 {
		if spent := g.SpentToday(ctx, src); spent >= g.DailyCap {
			return false, fmt.Sprintf("daily budget exhausted: %d/%d tasks enqueued today", spent, g.DailyCap)
		}
	}
	return true, ""
}

// SpentToday counts today's enqueued tasks from the journal. Every enqueue
// is presumed to become one agent run: on an agent-pool database this is
// exact; manually enqueued sh/http tasks count too (conservative).
func (g Guard) SpentToday(ctx context.Context, src FactSource) int {
	return g.spentSince(ctx, src, startOfDay(g.now()))
}

func (g Guard) spentSince(ctx context.Context, src FactSource, since time.Time) int {
	facts, err := src.Facts(ctx, 0)
	if err != nil {
		return 0 // fail open: the queue keeps working if the journal errors
	}
	n := 0
	for _, f := range facts {
		if f.Type == journal.Enqueued && !f.Time.Before(since) {
			n++
		}
	}
	return n
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return "exit status non-zero"
	}
	return s
}
