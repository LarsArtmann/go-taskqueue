// pool-health pins: the fold's liveness semantics over a real scratch
// journal — skip streaks (the claim→requeue loop), terminal resets,
// harvest detection via dedup prefixes, starvation, and the JSON shape.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
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

	report, err := poolHealth(ctx, store, 24*time.Hour, time.Now(), 0)
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

	report, err := poolHealth(ctx, store, 24*time.Hour, time.Now(), 0)
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

	report, err := poolHealth(ctx, store, 24*time.Hour, time.Now(), 0)
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

	report, err := poolHealth(ctx, store, 24*time.Hour, time.Now(), 0)
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

	report, err := poolHealth(ctx, store, 24*time.Hour, time.Now(), 0)
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

// TestPoolHealthDLQBackoffWarns pins the poisoned-repo surface: a repo
// whose latest dead-letter fact is fresher than the backoff window, with
// no completion on record, reports "in DLQ backoff until" and warns —
// the harvest pool's Pacing verdict made queryable.
func TestPoolHealthDLQBackoffWarns(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := journalAuditStore(t)

	if _, err := store.Enqueue(ctx, task.New{Project: "poisoned", Type: "sh", Payload: []byte(`"true"`)}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	claimed, claim, err := store.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}

	if err := store.FailPermanent(ctx, claimed.ID, claim, "boom", nil); err != nil {
		t.Fatalf("FailPermanent: %v", err)
	}

	report, err := poolHealth(ctx, store, 24*time.Hour, time.Now(), time.Hour)
	if err != nil {
		t.Fatalf("poolHealth: %v", err)
	}

	r := poolRepoByName(t, report, "poisoned")
	if r.DLQBackoffUntil == nil {
		t.Fatal("DLQBackoffUntil not set despite a fresh dead letter and no completion")
	}

	if !r.DLQBackoffUntil.After(time.Now()) || r.DLQBackoffUntil.After(time.Now().Add(2*time.Hour)) {
		t.Fatalf("DLQBackoffUntil = %v, want ~1h past the dead letter", r.DLQBackoffUntil)
	}

	if r.Verdict != poolVerdictWarn {
		t.Fatalf("Verdict = %q, want WARN", r.Verdict)
	}

	if warnings := strings.Join(r.Warnings, "; "); !strings.Contains(warnings, "poisoned") {
		t.Fatalf("warnings %q lack the poisoned cause", warnings)
	}

	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if !strings.Contains(string(raw), `"dlqBackoffUntil"`) {
		t.Fatalf("JSON %s lacks dlqBackoffUntil", raw)
	}
}

// TestPoolHealthDLQBackoffExpiryAndOff pins the backoff's off-ramps: an
// expired window (now past lastDead+backoff) and the flag at 0 both
// leave DLQBackoffUntil unset.
func TestPoolHealthDLQBackoffExpiryAndOff(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := journalAuditStore(t)

	if _, err := store.Enqueue(ctx, task.New{Project: "expired", Type: "sh", Payload: []byte(`"true"`)}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	claimed, claim, err := store.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}

	if err := store.FailPermanent(ctx, claimed.ID, claim, "boom", nil); err != nil {
		t.Fatalf("FailPermanent: %v", err)
	}

	expired, err := poolHealth(ctx, store, 24*time.Hour, time.Now().Add(2*time.Hour), time.Hour)
	if err != nil {
		t.Fatalf("poolHealth expired: %v", err)
	}

	if r := poolRepoByName(t, expired, "expired"); r.DLQBackoffUntil != nil {
		t.Fatalf("DLQBackoffUntil = %v, want nil after the backoff window", r.DLQBackoffUntil)
	}

	off, err := poolHealth(ctx, store, 24*time.Hour, time.Now(), 0)
	if err != nil {
		t.Fatalf("poolHealth off: %v", err)
	}

	if r := poolRepoByName(t, off, "expired"); r.DLQBackoffUntil != nil {
		t.Fatalf("DLQBackoffUntil = %v, want nil with the flag off", r.DLQBackoffUntil)
	}
}

// TestPoolHealthDLQBackoffBoundaryInstant pins the fold's expiry
// boundary exactly, over the pure fold with a fixed clock: at
// until == now the backoff has EXPIRED (nil, verdict OK) while 1ns
// before the boundary it is still ACTIVE (set to exactly
// lastDead+backoff, verdict WARN) — a future >= boundary refactor now
// fails loudly instead of silently shifting the poisoned verdict by one
// instant (closeout 2026-10-28 04-18 §f4).
func TestPoolHealthDLQBackoffBoundaryInstant(t *testing.T) {
	t.Parallel()

	const deadTask = "t-boundary"

	base := time.Date(2026, 10, 28, 4, 0, 0, 0, time.UTC)
	backoff := time.Hour
	until := base.Add(backoff)

	facts := []journal.Fact{{TaskID: deadTask, Type: journal.DeadLettered, Time: base}}
	meta := map[task.ID]poolTaskMeta{deadTask: {project: "poisoned"}}

	for _, tc := range []struct {
		name    string
		now     time.Time
		wantSet bool
	}{
		{"at the boundary instant the backoff has expired", until, false},
		{"1ns before the boundary the backoff is active", until.Add(-time.Nanosecond), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report := poolHealthFrom(facts, meta, base.Add(-time.Minute), tc.now, backoff)

			r := poolRepoByName(t, report, "poisoned")
			if !tc.wantSet {
				if r.DLQBackoffUntil != nil {
					t.Fatalf("DLQBackoffUntil = %v, want nil at the boundary instant (expired)", r.DLQBackoffUntil)
				}

				if r.Verdict != poolVerdictOK {
					t.Fatalf("Verdict = %q, want OK once expired", r.Verdict)
				}

				return
			}

			if r.DLQBackoffUntil == nil || !r.DLQBackoffUntil.Equal(until) {
				t.Fatalf("DLQBackoffUntil = %v, want exactly %v (still active)", r.DLQBackoffUntil, until)
			}

			if r.Verdict != poolVerdictWarn {
				t.Fatalf("Verdict = %q, want WARN while in backoff", r.Verdict)
			}
		})
	}
}

// TestPoolHealthDLQBackoffDismissalHeals pins the dismissal off-ramp:
// dismissing the dead-lettered task (the DLQ-autopsy path) lifts the
// backoff — the verdict returns to OK.
func TestPoolHealthDLQBackoffDismissalHeals(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := journalAuditStore(t)

	if _, err := store.Enqueue(ctx, task.New{Project: "healed", Type: "sh", Payload: []byte(`"true"`)}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	claimed, claim, err := store.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}

	if err := store.FailPermanent(ctx, claimed.ID, claim, "boom", nil); err != nil {
		t.Fatalf("FailPermanent: %v", err)
	}

	if err := store.DismissDead(ctx, claimed.ID, "autopsy: gate artifact", "tester"); err != nil {
		t.Fatalf("DismissDead: %v", err)
	}

	report, err := poolHealth(ctx, store, 24*time.Hour, time.Now(), time.Hour)
	if err != nil {
		t.Fatalf("poolHealth: %v", err)
	}

	r := poolRepoByName(t, report, "healed")
	if r.DLQBackoffUntil != nil {
		t.Fatalf("DLQBackoffUntil = %v, want nil after dismissal", r.DLQBackoffUntil)
	}

	if r.Verdict != poolVerdictOK {
		t.Fatalf("Verdict = %q, want OK after dismissal", r.Verdict)
	}
}

// TestPoolHealthDLQBackoffCompletionHeals pins the completion off-ramp:
// rescuing the dead task and completing it clears the backoff, exactly
// like the harvester's ever-completed check.
func TestPoolHealthDLQBackoffCompletionHeals(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := journalAuditStore(t)

	if _, err := store.Enqueue(ctx, task.New{Project: "rescued", Type: "sh", Payload: []byte(`"true"`)}); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	claimed, claim, err := store.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}

	if err := store.FailPermanent(ctx, claimed.ID, claim, "boom", nil); err != nil {
		t.Fatalf("FailPermanent: %v", err)
	}

	if err := store.RescueDead(ctx, claimed.ID, 2); err != nil {
		t.Fatalf("RescueDead: %v", err)
	}

	rescued, claim2, err := store.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("ClaimDue rescued: %v", err)
	}

	if err := store.Complete(ctx, rescued.ID, claim2, nil); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	report, err := poolHealth(ctx, store, 24*time.Hour, time.Now(), time.Hour)
	if err != nil {
		t.Fatalf("poolHealth: %v", err)
	}

	r := poolRepoByName(t, report, "rescued")
	if r.DLQBackoffUntil != nil {
		t.Fatalf("DLQBackoffUntil = %v, want nil after a completion", r.DLQBackoffUntil)
	}

	if r.Verdict != poolVerdictOK {
		t.Fatalf("Verdict = %q, want OK after a completion", r.Verdict)
	}
}

// TestPoolHealthDLQBackoffFromPoolConfig pins the config-file reader:
// same file, spelling, and duration syntax the harvest pool runs with,
// quoting tolerated like every pool.conf value, malformed values loud
// (a silently-zero default would under-report the exact state the
// surface exists to surface), other keys ignored.
func TestPoolHealthDLQBackoffFromPoolConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "pool.conf")

	write := func(t *testing.T, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}
	}

	t.Run("key seeds the default", func(t *testing.T) {
		write(t, "projects-dir = /srv/repos\ndlq-backoff = 45m\n")

		got, found, err := dlqBackoffFromPoolConfig(path)
		if err != nil || !found || got != 45*time.Minute {
			t.Fatalf("got (%v, %v, %v), want (45m, true, nil)", got, found, err)
		}
	})

	t.Run("quoted value tolerated", func(t *testing.T) {
		write(t, "dlq-backoff = \"1h30m\"\n")

		got, found, err := dlqBackoffFromPoolConfig(path)
		if err != nil || !found || got != 90*time.Minute {
			t.Fatalf("got (%v, %v, %v), want (1h30m, true, nil)", got, found, err)
		}
	})

	t.Run("absent key means off", func(t *testing.T) {
		write(t, "projects-dir = /srv/repos\n")

		got, found, err := dlqBackoffFromPoolConfig(path)
		if err != nil || found || got != 0 {
			t.Fatalf("got (%v, %v, %v), want (0, false, nil)", got, found, err)
		}
	})

	t.Run("malformed value is loud", func(t *testing.T) {
		write(t, "dlq-backoff = soon\n")

		if _, _, err := dlqBackoffFromPoolConfig(path); err == nil {
			t.Fatalf("malformed dlq-backoff accepted, want error")
		}
	})

	t.Run("unreadable file is loud", func(t *testing.T) {
		if _, _, err := dlqBackoffFromPoolConfig(filepath.Join(dir, "absent.conf")); err == nil {
			t.Fatalf("unreadable config accepted, want error")
		}
	})
}

// TestApplyPoolHealthConfigDefault pins the precedence and resolution:
// --dlq-backoff flag > pool config file > 0; an empty --config falls back
// to the bootstrap default location ONLY when that file exists; an
// explicitly given but unreadable config is an error. Not parallel: the
// HOME-resolution subtests use t.Setenv.
func TestApplyPoolHealthConfigDefault(t *testing.T) {
	newFlagSet := func(t *testing.T) (*flag.FlagSet, *time.Duration) {
		t.Helper()

		fs := flag.NewFlagSet("pool-health", flag.ContinueOnError)
		dlqBackoff := fs.Duration("dlq-backoff", 0, "")

		return fs, dlqBackoff
	}

	writeConfig := func(t *testing.T, path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}
	}

	t.Run("config seeds, CLI wins", func(t *testing.T) {
		config := filepath.Join(t.TempDir(), "pool.conf")
		writeConfig(t, config, "dlq-backoff = 45m\n")

		fs, dlqBackoff := newFlagSet(t)
		if err := applyPoolHealthConfigDefault(config, fs, dlqBackoff); err != nil {
			t.Fatalf("apply: %v", err)
		}
		if *dlqBackoff != 45*time.Minute {
			t.Fatalf("backoff = %v, want 45m from config", *dlqBackoff)
		}

		fs, dlqBackoff = newFlagSet(t)
		if err := fs.Parse([]string{"-dlq-backoff=5m"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if err := applyPoolHealthConfigDefault(config, fs, dlqBackoff); err != nil {
			t.Fatalf("apply: %v", err)
		}
		if *dlqBackoff != 5*time.Minute {
			t.Fatalf("backoff = %v, want the explicit 5m to win over config", *dlqBackoff)
		}
	})

	t.Run("no config anywhere stays off", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())

		fs, dlqBackoff := newFlagSet(t)
		if err := applyPoolHealthConfigDefault("", fs, dlqBackoff); err != nil {
			t.Fatalf("apply: %v", err)
		}
		if *dlqBackoff != 0 {
			t.Fatalf("backoff = %v, want 0 without any config file", *dlqBackoff)
		}
	})

	t.Run("bootstrap default used when it exists", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		writeConfig(t, filepath.Join(home, ".config", "tq", "pool.conf"), "dlq-backoff = 2h\n")

		fs, dlqBackoff := newFlagSet(t)
		if err := applyPoolHealthConfigDefault("", fs, dlqBackoff); err != nil {
			t.Fatalf("apply: %v", err)
		}
		if got := defaultPoolConfigPath(); got == "" {
			t.Fatalf("defaultPoolConfigPath unresolved (no home dir) — cannot exercise the fallback here")
		} else if *dlqBackoff != 2*time.Hour {
			t.Fatalf("backoff = %v, want 2h from the bootstrap default file", *dlqBackoff)
		}
	})

	t.Run("given-but-unreadable config is loud", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "absent.conf")

		fs, dlqBackoff := newFlagSet(t)
		if err := applyPoolHealthConfigDefault(missing, fs, dlqBackoff); err == nil {
			t.Fatalf("unreadable config accepted, want error")
		}
	})
}

// TestPoolHealthSummaryCountsBackoff pins the summary aggregate: the
// trailing line counts repos currently in DLQ backoff so scripts get
// the aggregate without folding the JSON — including the explicit
// zero when the backoff surface is off, so the line's shape is stable
// for parsers (closeout 2026-10-28 04-18 §f5).
func TestPoolHealthSummaryCountsBackoff(t *testing.T) {
	t.Parallel()

	now := time.Now()
	active := now.Add(time.Hour)

	report := PoolHealthReport{
		Since:       now.Add(-24 * time.Hour).Format(time.RFC3339),
		FactsFolded: 7,
		Repos: []RepoHealth{
			{Repo: "poisoned", Verdict: poolVerdictWarn, DLQBackoffUntil: &active},
			{Repo: "stucky", Verdict: poolVerdictWarn},
			{Repo: "healthy", Verdict: poolVerdictOK},
		},
	}

	out := captureStdout(t, func() { printPoolHealth(report) })

	for _, want := range []string{
		"3 repos, 1 in DLQ backoff, 2 warn",
		"7 facts folded",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("summary %q lacks %q", out, want)
		}
	}

	report.Repos[0].DLQBackoffUntil = nil

	out = captureStdout(t, func() { printPoolHealth(report) })

	if !strings.Contains(out, "3 repos, 0 in DLQ backoff, 2 warn") {
		t.Fatalf("summary %q lacks the explicit zero count", out)
	}
}
