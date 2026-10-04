package budget

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"

	_ "time/tzdata" // embedded tzdb keeps the DST table hermetic (nix checkPhase has no zoneinfo)
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

func (m factSource) FactsSince(
	ctx context.Context,
	ftype journal.FactType,
	since time.Time,
	limit int,
) ([]journal.Fact, error) {
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
// rename in any of them fails here (both directions of drift).
func TestUsageTodaySumsDerivedSessionUsage(t *testing.T) {
	ctx := context.Background()
	j := journal.NewMemoryJournal()

	// One agent run, one prioritize batch, one review turn, one status
	// report and one autopsy, all derived, all counted.
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
	appendCompleted(t, j, "review-1", time.Now(), executor.ReviewResult{
		Verdict:                 executor.VerdictApprove,
		SessionID:               "s3",
		SessionCostUSD:          0.15,
		SessionPromptTokens:     800,
		SessionCompletionTokens: 200,
		SessionMessageCount:     5,
	})
	appendCompleted(t, j, "status-1", time.Now(), executor.StatusResult{
		Report:                  "docs/status/2026-09-21_00-00_demo.md",
		SessionID:               "s4",
		SessionCostUSD:          0.10,
		SessionPromptTokens:     400,
		SessionCompletionTokens: 900,
		SessionMessageCount:     6,
	})
	appendCompleted(t, j, "dlqfix-1", time.Now(), executor.DLQFixResult{
		Verdict:                 executor.VerdictFixed,
		Summary:                 "root cause was a stale pin; fixed and proven",
		SessionID:               "s5",
		SessionCostUSD:          0.05,
		SessionPromptTokens:     250,
		SessionCompletionTokens: 550,
		SessionMessageCount:     3,
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
	want := SessionUsage{Runs: 5, CostUSD: 0.80, PromptTokens: 2950, CompletionTokens: 5750, Messages: 27}

	if got != want {
		t.Fatalf("usage today = %+v, want %+v (non-usage and yesterday's completions excluded)", got, want)
	}
}

func TestUsageTodayIgnoresBrokenDetail(t *testing.T) {
	ctx := context.Background()
	j := journal.NewMemoryJournal()

	_, _ = j.Append(ctx, journal.Fact{TaskID: "junk", Type: journal.Completed, Detail: []byte(`not json`)})
	_, _ = j.Append(
		ctx,
		journal.Fact{TaskID: "zero", Type: journal.Completed, Detail: []byte(`{"session_prompt_tokens":0}`)},
	)

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

// cmdCacheStub builds a BudgetCmd script that counts its own executions in
// counterPath and refuses with "spent" until allowPath exists — the
// observable behavior changes mid-test without rewriting the script.
func cmdCacheStub(t *testing.T, counterPath, allowPath string) string {
	t.Helper()

	return fmt.Sprintf(
		`echo x >> %q; if [ -f %q ]; then exit 0; fi; echo spent; exit 1`,
		counterPath,
		allowPath,
	)
}

func countStubRuns(t *testing.T, counterPath string) int {
	t.Helper()

	b, err := os.ReadFile(counterPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0
		}

		t.Fatalf("read counter: %v", err)
	}

	return strings.Count(string(b), "\n")
}

func TestCheckCachesBudgetCmdVerdict(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	counter, allow := filepath.Join(dir, "runs"), filepath.Join(dir, "allow")

	base := time.Unix(1_700_000_000, 0)

	var offSec atomic.Int64

	g := Guard{
		BudgetCmd: cmdCacheStub(t, counter, allow),
		Now:       func() time.Time { return base.Add(time.Duration(offSec.Load()) * time.Second) },
	}.WithCmdCache(time.Minute)

	ctx := context.Background()

	offSec.Store(1)

	for i := range 3 {
		allowed, reason := g.Check(ctx, nil)
		if allowed || !strings.Contains(reason, "spent") {
			t.Fatalf("check %d = (%v, %q), want the cached refusal", i, allowed, reason)
		}
	}

	if runs := countStubRuns(t, counter); runs != 1 {
		t.Fatalf("budget command ran %d times inside the TTL window, want 1", runs)
	}

	// The budget clears mid-window: the cached refusal must hold until the
	// TTL expires (that staleness bound is the documented cost of not
	// exec'ing per claim).
	if err := os.WriteFile(allow, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write allow: %v", err)
	}

	offSec.Store(2)

	if allowed, _ := g.Check(ctx, nil); allowed {
		t.Fatal("check inside the TTL window must return the cached refusal")
	}

	// Past the TTL the guard re-runs the command and sees the clearance.
	offSec.Store(120)

	if allowed, reason := g.Check(ctx, nil); !allowed || reason != "" {
		t.Fatalf("check past the TTL = (%v, %q), want a fresh allow", allowed, reason)
	}

	if runs := countStubRuns(t, counter); runs != 2 {
		t.Fatalf("budget command ran %d times after expiry, want 2", runs)
	}

	// The fresh allow is itself cached: no third exec while it holds.
	offSec.Store(121)

	if allowed, _ := g.Check(ctx, nil); !allowed {
		t.Fatal("check inside the cached allow window must be allowed")
	}

	if runs := countStubRuns(t, counter); runs != 2 {
		t.Fatalf("budget command ran %d times after caching the allow, want 2", runs)
	}
}

func TestCheckCmdCacheSharedAcrossCopies(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	counter, allow := filepath.Join(dir, "runs"), filepath.Join(dir, "allow")

	base := time.Unix(1_700_000_000, 0)

	var tick atomic.Int64

	armed := Guard{
		BudgetCmd: cmdCacheStub(t, counter, allow),
		Now:       func() time.Time { return base.Add(time.Duration(tick.Add(1)) * time.Second) },
	}.WithCmdCache(time.Minute)

	ctx := context.Background()

	// The harvest gate and the claim gate hold DIFFERENT copies of the
	// armed guard: the verdict (and the exec count) must still be shared —
	// two caches would double the exec rate the cache exists to bound.
	harvestGate := armed

	if allowed, reason := harvestGate.Check(ctx, nil); allowed || !strings.Contains(reason, "spent") {
		t.Fatalf("harvest-side check = (%v, %q), want refusal", allowed, reason)
	}

	if allowed, reason := armed.Check(ctx, nil); allowed || !strings.Contains(reason, "spent") {
		t.Fatalf("claim-side check = (%v, %q), want the SAME cached refusal", allowed, reason)
	}

	if runs := countStubRuns(t, counter); runs != 1 {
		t.Fatalf("budget command ran %d times across two guard copies, want 1", runs)
	}
}

func TestWithCmdCacheZeroTTLIsInert(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	counter, allow := filepath.Join(dir, "runs"), filepath.Join(dir, "allow")

	g := Guard{BudgetCmd: cmdCacheStub(t, counter, allow)}.WithCmdCache(0)

	if g.cmdCache != nil {
		t.Fatal("zero ttl must not arm the cache")
	}

	for range 2 {
		if allowed, _ := g.Check(context.Background(), nil); allowed {
			t.Fatal("refusing cmd must refuse without the cache")
		}
	}

	if runs := countStubRuns(t, counter); runs != 2 {
		t.Fatalf("uncached guard ran the command %d times, want 2", runs)
	}
}

// TestNextMidnightAcrossDSTDays pins the DST-correct midnight the
// budget park uses (M2.3 of the paperclip-aftermath plan): a 23-hour
// spring-forward day and a 25-hour fall-back day both end at TRUE local
// midnight, which midnight+24h wall arithmetic misses by an hour. The
// contrast assertions keep that regression direction pinned, not just the
// correct values.
func TestNextMidnightAcrossDSTDays(t *testing.T) {
	t.Parallel()

	nyc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load New_York: %v", err)
	}

	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("load Berlin: %v", err)
	}

	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "plain UTC day",
			now:  time.Date(2026, 10, 4, 15, 0, 0, 0, time.UTC),
			want: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "US spring-forward day ends at true local midnight",
			now:  time.Date(2026, 3, 8, 23, 0, 0, 0, nyc),
			want: time.Date(2026, 3, 9, 0, 0, 0, 0, nyc),
		},
		{
			name: "US fall-back day ends at true local midnight",
			now:  time.Date(2026, 11, 1, 23, 0, 0, 0, nyc),
			want: time.Date(2026, 11, 2, 0, 0, 0, 0, nyc),
		},
		{
			name: "fall-back transition instant still rolls to next true midnight",
			now:  time.Date(2026, 11, 1, 0, 30, 0, 0, nyc),
			want: time.Date(2026, 11, 2, 0, 0, 0, 0, nyc),
		},
		{
			name: "EU spring-forward day ends at true local midnight",
			now:  time.Date(2026, 3, 29, 23, 0, 0, 0, berlin),
			want: time.Date(2026, 3, 30, 0, 0, 0, 0, berlin),
		},
		{
			name: "EU fall-back day ends at true local midnight",
			now:  time.Date(2026, 10, 25, 23, 0, 0, 0, berlin),
			want: time.Date(2026, 10, 26, 0, 0, 0, 0, berlin),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := NextMidnight(tc.now)
			if !got.Equal(tc.want) {
				t.Errorf("NextMidnight(%s) = %s, want %s", tc.now, got, tc.want)
			}

			if !got.After(tc.now) {
				t.Errorf("NextMidnight(%s) = %s must be strictly after now", tc.now, got)
			}

			// The +24h wall arithmetic this replaces is wrong by an hour on
			// every DST-transition day; keep that failure mode loud.
			if plus24 := tc.now.Add(24 * time.Hour); plus24.Equal(got) && plus24.Location() != nil {
				t.Errorf("midnight+24h wall arithmetic coincides with the DST-correct midnight at %s", tc.now)
			}
		})
	}
}
