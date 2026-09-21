// Package budget answers one question for unattended pools: may we spend
// more agent money right now? Every enqueued agent task is a real cost, so
// the guard projects spend from the journal's task.enqueued facts and can
// defer to an operator-supplied command for custom ceilings.
package budget

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
)

// FactSource is the journal view the guard projects spend from. *queue.Queue
// satisfies it: the count is a SQL pushdown, not a journal scan.
type FactSource interface {
	Facts(ctx context.Context, after int64, limit int) ([]journal.Fact, error)
	CountFacts(ctx context.Context, ftype journal.FactType, since time.Time) (int64, error)
	FactsSince(ctx context.Context, ftype journal.FactType, since time.Time, limit int) ([]journal.Fact, error)
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
			reason := fmt.Sprintf("daily budget exhausted: %d/%d tasks enqueued today", spent, g.DailyCap)
			if usage := g.UsageToday(ctx, src); usage.Runs > 0 {
				reason += "; " + usage.String()
			}

			return false, reason
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
	n, err := src.CountFacts(ctx, journal.Enqueued, since)
	if err != nil {
		return 0 // fail open: the queue keeps working if the journal errors
	}

	return int(n)
}

// SessionUsage is the derived agent-spend projection: tokens and cost
// summed from the day's task.completed facts whose result detail carries
// derived session usage. AgentResult (agent runs), PrioritizeResult (batch
// scorer runs), ReviewResult (review turns) and StatusResult (status/report
// runs) share the same json keys, so one parse covers all; completion
// facts without usage (sh tasks, records from before the derivation
// shipped) contribute nothing.
type SessionUsage struct {
	// Runs counts completion facts that carried derived session usage —
	// the derivable subset of today's completions, not every task.
	Runs             int
	CostUSD          float64
	PromptTokens     int64
	CompletionTokens int64
	Messages         int
}

func (u SessionUsage) String() string {
	return fmt.Sprintf("%d derived runs: %d prompt + %d completion tokens, $%.4f session cost",
		u.Runs, u.PromptTokens, u.CompletionTokens, u.CostUSD)
}

// sessionUsageDetail is the usage-carrying projection of a completion
// fact's result detail. The json keys are owned by the executor result
// types; budget_test marshals the REAL AgentResult, PrioritizeResult,
// ReviewResult and StatusResult so a key rename in any of them fails this
// projection's suite.
type sessionUsageDetail struct {
	CostUSD          float64 `json:"session_cost_usd"`
	PromptTokens     int64   `json:"session_prompt_tokens"`
	CompletionTokens int64   `json:"session_completion_tokens"`
	Messages         int     `json:"session_message_count"`
}

// UsageToday sums today's derived session usage from completion facts —
// spend measured in tokens and cost, the axis the enqueue count cannot
// see. Fail-open like the count projection: a journal error reads as zero
// usage, never blocks the pool.
func (g Guard) UsageToday(ctx context.Context, src FactSource) SessionUsage {
	return g.usageSince(ctx, src, startOfDay(g.now()))
}

func (g Guard) usageSince(ctx context.Context, src FactSource, since time.Time) SessionUsage {
	facts, err := src.FactsSince(ctx, journal.Completed, since, 0)
	if err != nil {
		return SessionUsage{} // fail open: the queue keeps working if the journal errors
	}

	var usage SessionUsage

	for _, f := range facts {
		var d sessionUsageDetail
		if err := json.Unmarshal(f.Detail, &d); err != nil {
			continue // non-JSON or empty detail: not a usage-carrying result
		}

		if d.PromptTokens == 0 && d.CompletionTokens == 0 && d.CostUSD == 0 {
			continue
		}

		usage.Runs++
		usage.CostUSD += d.CostUSD
		usage.PromptTokens += d.PromptTokens
		usage.CompletionTokens += d.CompletionTokens
		usage.Messages += d.Messages
	}

	return usage
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()

	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func firstLine(line string) string {
	line = strings.TrimSpace(line)
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}

	if line == "" {
		return "exit status non-zero"
	}

	return line
}
