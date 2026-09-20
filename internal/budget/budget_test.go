package budget

import (
	"context"
	"encoding/json/v2"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
)

// seeded returns a journal with n enqueued facts today and m yesterday.
func seeded(nToday, nYesterday int) factSource {
	j := journal.NewMemoryJournal()
	ctx := context.Background()

	for i := range nToday + nYesterday {
		f := journal.Fact{TaskID: "t", Type: journal.Enqueued}
		if i >= nToday {
			f.Time = time.Now().Add(-24 * time.Hour)
		}

		_, _ = j.Append(ctx, f)
	}

	return factSource{j}
}

// factSource adapts MemoryJournal (Since/All) to the guard's FactSource
// view, mirroring the queue.Store cursor/pushdown mapping.
type factSource struct{ j *journal.MemoryJournal }

func (m factSource) Facts(ctx context.Context, after int64, limit int) ([]journal.Fact, error) {
	facts, err := m.j.Since(ctx, after)
	if err != nil {
		return nil, err
	}

	if limit > 0 && len(facts) > limit {
		facts = facts[:limit]
	}

	return facts, err
}

func (m factSource) FactsSince(ctx context.Context, ftype journal.FactType, since time.Time, limit int) ([]journal.Fact, error) {
	all, err := m.j.All(ctx)
	if err != nil {
		return nil, err
	}

	var out []journal.Fact

	for _, f := range all {
		if f.Type == ftype && !f.Time.Before(since) {
			out = append(out, f)
		}
	}

	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}

	return out, nil
}

func (m factSource) CountFacts(ctx context.Context, ftype journal.FactType, since time.Time) (int64, error) {
	facts, err := m.j.All(ctx)
	if err != nil {
		return 0, err
	}

	var n int64

	for _, f := range facts {
		if f.Type == ftype && !f.Time.Before(since) {
			n++
		}
	}

	return n, nil
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

// appendCompleted marshals result into a task.completed fact at time at.
func appendCompleted(t *testing.T, j *journal.MemoryJournal, id string, at time.Time, result any) {
	t.Helper()

	detail, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}

	_, _ = j.Append(context.Background(), journal.Fact{TaskID: id, Type: journal.Completed, Time: at, Detail: detail})
}

// TestUsageTodaySumsDerivedSessionUsage pins the token/cost projection:
// completion facts carrying derived session usage sum into the day's
// spend, marshalled through the REAL executor result types so a json key
// rename in either result type fails here (both directions of drift).
func TestUsageTodaySumsDerivedSessionUsage(t *testing.T) {
	ctx := context.Background()
	j := journal.NewMemoryJournal()

	// One agent run and one prioritize batch, both derived, both counted.
	appendCompleted(t, j, "agent-1", time.Now(), executor.AgentResult{
		SessionID:               "s1",
		SessionCostUSD:          0.42,
		SessionPromptTokens:     1200,
		SessionCompletionTokens: 3400,
		SessionMessageCount:     9,
	})
	appendCompleted(t, j, "prio-1", time.Now(), executor.PrioritizeResult{
		SessionID:               "s2",
		SessionCostUSD:          0.08,
		SessionPromptTokens:     300,
		SessionCompletionTokens: 700,
		SessionMessageCount:     4,
	})

	// sh completion without usage and a detailless one: never counted.
	appendCompleted(t, j, "sh-1", time.Now(), map[string]int{"exit_code": 0})
	_, _ = j.Append(ctx, journal.Fact{TaskID: "bare-1", Type: journal.Completed})

	// Yesterday's usage stays out of today's window.
	appendCompleted(t, j, "old-1", time.Now().Add(-24*time.Hour), executor.AgentResult{
		SessionCostUSD:      9.99,
		SessionPromptTokens: 99,
	})

	got := (Guard{}).UsageToday(ctx, factSource{j})
	want := SessionUsage{Runs: 2, CostUSD: 0.5, PromptTokens: 1500, CompletionTokens: 4100, Messages: 13}
	if got != want {
		t.Fatalf("usage today = %+v, want %+v (non-usage and yesterday's completions excluded)", got, want)
	}
}

func TestUsageTodayIgnoresBrokenDetail(t *testing.T) {
	ctx := context.Background()
	j := journal.NewMemoryJournal()

	_, _ = j.Append(ctx, journal.Fact{TaskID: "junk", Type: journal.Completed, Detail: []byte(`not json`)})
	_, _ = j.Append(ctx, journal.Fact{TaskID: "zero", Type: journal.Completed, Detail: []byte(`{"session_prompt_tokens":0}`)})

	if got := (Guard{}).UsageToday(ctx, factSource{j}); got.Runs != 0 {
		t.Fatalf("broken/zero details must not count as runs, got %+v", got)
	}
}

func TestCheckExhaustedReasonCarriesUsage(t *testing.T) {
	ctx := context.Background()
	j := journal.NewMemoryJournal()

	_, _ = j.Append(ctx, journal.Fact{TaskID: "t", Type: journal.Enqueued})
	appendCompleted(t, j, "agent-1", time.Now(), executor.AgentResult{SessionCostUSD: 0.25, SessionPromptTokens: 100})

	ok, reason := (Guard{DailyCap: 1}).Check(ctx, factSource{j})
	if ok {
		t.Fatal("at cap must refuse")
	}

	for _, want := range []string{"1/1", "100 prompt", "$0.2500"} {
		if !strings.Contains(reason, want) {
			t.Fatalf("refusal reason %q lost the usage projection (want %q)", reason, want)
		}
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
