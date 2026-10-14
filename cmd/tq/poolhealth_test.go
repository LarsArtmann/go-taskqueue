// pool-health pins: the fold's liveness semantics over a real scratch
// journal — skip streaks (the claim→requeue loop), terminal resets,
// harvest detection via dedup prefixes, starvation, and the JSON shape.
package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func poolRepoByName(t *testing.T, report PoolHealthReport, name string) RepoHealth {
	t.Helper()

	for _, r := range report.Repos {
		if r.Repo == name {
			return r
		}
	}

	t.Fatalf("repo %q missing from report (have %v)", name, report.Repos)

	return RepoHealth{}
}

// TestPoolHealthSkipStreakWarns drives the pathological loop pool-health
// exists to surface: three claim→requeue-without-burn cycles with no
// outcome must leave streak 3 and a WARN verdict.
func TestPoolHealthSkipStreakWarns(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := journalAuditStore(t)

	enq, err := store.Enqueue(ctx, task.New{Project: "stucky", Type: "sh", Payload: []byte(`"true"`)})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	for range 3 {
		claimed, claim, err := store.ClaimDue(ctx, "w1", time.Minute)
		if err != nil {
			t.Fatalf("ClaimDue: %v", err)
		}

		if claimed.ID != enq.ID {
			t.Fatalf("claimed %s, want the seeded task %s", claimed.ID, enq.ID)
		}

		if err := store.Requeue(ctx, claimed.ID, claim, "429", 0, false, queue.RequeueClassRateLimit, ""); err != nil {
			t.Fatalf("Requeue: %v", err)
		}
	}

	report, err := poolHealth(ctx, store, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("poolHealth: %v", err)
	}

	r := poolRepoByName(t, report, "stucky")
	if r.SkipStreak != 3 {
		t.Fatalf("SkipStreak = %d, want 3", r.SkipStreak)
	}

	if r.Verdict != poolVerdictWarn {
		t.Fatalf("Verdict = %q, want WARN", r.Verdict)
	}

	if len(r.Warnings) == 0 {
		t.Fatal("expected at least one warning for the streak")
	}

	if r.RequeuesByClass[queue.RequeueClassRateLimit] != 3 {
		t.Fatalf("RequeuesByClass[rate-limit] = %d, want 3", r.RequeuesByClass[queue.RequeueClassRateLimit])
	}
}

// TestPoolHealthTerminalResetsStreak pins that a turn which RAN clears
// the streak: requeues followed by a completion leave streak 0, verdict
// OK (the repo's work moved).
func TestPoolHealthTerminalResetsStreak(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := journalAuditStore(t)

	if _, err := store.Enqueue(ctx, task.New{Project: "healthy", Type: "sh", Payload: []byte(`"true"`)}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	for range 2 {
		claimed, claim, err := store.ClaimDue(ctx, "w1", time.Minute)
		if err != nil {
			t.Fatalf("ClaimDue: %v", err)
		}

		if err := store.Requeue(ctx, claimed.ID, claim, "gate dead", 0, false, queue.RequeueClassGate, ""); err != nil {
			t.Fatalf("Requeue: %v", err)
		}
	}

	claimed, claim, err := store.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue final: %v", err)
	}

	if err := store.Complete(ctx, claimed.ID, claim, nil); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	report, err := poolHealth(ctx, store, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("poolHealth: %v", err)
	}

	r := poolRepoByName(t, report, "healthy")
	if r.SkipStreak != 0 {
		t.Fatalf("SkipStreak = %d, want 0 after a completed turn", r.SkipStreak)
	}

	if r.Verdict != poolVerdictOK {
		t.Fatalf("Verdict = %q, want OK", r.Verdict)
	}

	if r.Completed != 1 || r.Enqueued != 1 {
		t.Fatalf("counts enq=%d done=%d, want 1/1", r.Enqueued, r.Completed)
	}
}

// TestPoolHealthHarvestDetection pins last-harvest recency: only
// harvest-minted tasks (todo:/batch: dedup keys) mark a repo harvested;
// direct enqueues without dedup keys do not.
func TestPoolHealthHarvestDetection(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := journalAuditStore(t)

	if _, err := store.Enqueue(ctx, task.New{
		Project: "farm", Type: "agent", Payload: []byte(`{}`), DedupKey: harvestItemKeyPrefix + "deadbeef",
	}); err != nil {
		t.Fatalf("Enqueue harvest: %v", err)
	}

	if _, err := store.Enqueue(ctx, task.New{
		Project: "farm", Type: "agent", Payload: []byte(`{}`), DedupKey: "batch:cafe",
	}); err != nil {
		t.Fatalf("Enqueue batch: %v", err)
	}

	if _, err := store.Enqueue(ctx, task.New{
		Project: "plain", Type: "agent", Payload: []byte(`{}`),
	}); err != nil {
		t.Fatalf("Enqueue direct: %v", err)
	}

	report, err := poolHealth(ctx, store, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("poolHealth: %v", err)
	}

	farm := poolRepoByName(t, report, "farm")
	if farm.LastHarvest == nil {
		t.Fatal("farm LastHarvest not set despite todo:/batch: dedup keys")
	}

	direct := poolRepoByName(t, report, "plain")
	if direct.LastHarvest != nil {
		t.Fatal("plain LastHarvest set without a harvest dedup key")
	}
}

// TestPoolHealthStarvedWarns pins the silent-outage verdict: a repo with
// fresh enqueues but zero outcomes in the window warns as starved.
func TestPoolHealthStarvedWarns(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := journalAuditStore(t)

	if _, err := store.Enqueue(ctx, task.New{Project: "quiet", Type: "sh", Payload: []byte(`"true"`)}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	report, err := poolHealth(ctx, store, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("poolHealth: %v", err)
	}

	r := poolRepoByName(t, report, "quiet")
	if r.Verdict != poolVerdictWarn {
		t.Fatalf("Verdict = %q, want WARN", r.Verdict)
	}

	if !strings.Contains(strings.Join(r.Warnings, "; "), "starved") {
		t.Fatalf("warnings %v lack the starvation cause", r.Warnings)
	}
}

// TestPoolHealthJSONShape pins the machine surface: verdict, streak,
// facts count, and requeue classes survive the JSON round trip.
func TestPoolHealthJSONShape(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := journalAuditStore(t)

	if _, err := store.Enqueue(ctx, task.New{Project: "stucky", Type: "sh", Payload: []byte(`"true"`)}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	claimed, claim, err := store.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}

	if err := store.Requeue(ctx, claimed.ID, claim, "429", 0, false, queue.RequeueClassRateLimit, ""); err != nil {
		t.Fatalf("Requeue: %v", err)
	}

	report, err := poolHealth(ctx, store, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("poolHealth: %v", err)
	}

	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	for _, want := range []string{
		`"factsFolded"`,
		`"verdict":"WARN"`,
		`"skipStreak":1`,
		`"requeuesByClass":{"rate-limit":1}`,
		`"repo":"stucky"`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("JSON %s lacks %q", raw, want)
		}
	}
}
