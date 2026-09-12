package postgres

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestPostgresConformance closes the battery gap (21:40 §e-item): the full
// queue-conformance semantics the SQLite suite pins — dependency gating,
// delay/priority scheduling, the retry/backoff ladder with failure
// evidence, permanent dead-lettering, requeue without attempt burn,
// reason-carrying cancels, heartbeat, the filter/list/count read surface —
// run against the Postgres store too. CI's test-postgres job executes this
// via `-run TestPostgres` against the service container.
func TestPostgresConformance(t *testing.T) {
	s := testPostgresStore(t)
	ctx := context.Background()

	project := "battery-" + time.Now().Format("150405.000000000")

	t.Run("dependency gating", func(t *testing.T) {
		blocker, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project})
		if err != nil {
			t.Fatal(err)
		}

		waiter, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project, Deps: []task.ID{blocker.ID}})
		if err != nil {
			t.Fatal(err)
		}

		got, err := s.ClaimDue(ctx, "dep-w", time.Minute)
		if err != nil {
			t.Fatal(err)
		}

		if got.ID != blocker.ID {
			t.Fatalf("claimed dependent %s before its blocker — DAG gating broken", got.ID)
		}

		if err := s.Complete(ctx, blocker.ID, "dep-w", nil); err != nil {
			t.Fatal(err)
		}

		got, err = s.ClaimDue(ctx, "dep-w", time.Minute)
		if err != nil {
			t.Fatal(err)
		}

		if got.ID != waiter.ID {
			t.Fatalf("after blocker completed, claimed %s, want the waiter %s", got.ID, waiter.ID)
		}
	})

	t.Run("delay and priority order", func(t *testing.T) {
		future, err := s.Enqueue(ctx, task.New{
			Type:      "sh",
			Project:   project,
			NotBefore: time.Now().Add(time.Minute),
		})
		if err != nil {
			t.Fatal(err)
		}

		_ = future // stays pending-and-gated; per-task subtests below claim by ID

		low, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project, Priority: -5})
		if err != nil {
			t.Fatal(err)
		}

		high, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project, Priority: 5})
		if err != nil {
			t.Fatal(err)
		}

		got, err := s.ClaimDue(ctx, "sched-w", time.Minute)
		if err != nil {
			t.Fatal(err)
		}

		if got.ID != high.ID {
			t.Fatalf("claimed %s, want the highest-priority ready task %s", got.ID, high.ID)
		}

		if err := s.Complete(ctx, high.ID, "sched-w", nil); err != nil {
			t.Fatal(err)
		}

		// The delayed task is still not claimable a minute out.
		got, err = s.ClaimDue(ctx, "sched-w", time.Minute)
		if err != nil {
			t.Fatal(err)
		}

		if got.ID != low.ID {
			t.Fatalf("claimed %s, want the low-priority task (future one must stay gated)", got.ID)
		}

		if err := s.Complete(ctx, low.ID, "sched-w", nil); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("aging flips claim order within the cap", func(t *testing.T) {
		older, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project, Priority: 55})
		if err != nil {
			t.Fatal(err)
		}

		newer, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project, Priority: 60})
		if err != nil {
			t.Fatal(err)
		}

		// Backdate past the aging saturation point: the older task's
		// effective 55+queue.PriorityAgingMaxBonus must beat the newer's
		// 60 — the mirror of the sqlite white-box suite (ADR-0015 §4).
		backdated := time.Now().Add(-45 * 24 * time.Hour).UnixMilli()
		if _, err := s.pool.Exec(ctx, `UPDATE tasks SET created_at = $1 WHERE id = $2`, backdated, older.ID); err != nil {
			t.Fatal(err)
		}

		got, err := s.ClaimDue(ctx, "aging-w", time.Minute)
		if err != nil {
			t.Fatal(err)
		}

		if got.ID != older.ID {
			t.Fatalf("aging did not flip claim order: claimed %s, want older %s over newer %s", got.ID, older.ID, newer.ID)
		}

		if err := s.Complete(ctx, older.ID, "aging-w", nil); err != nil {
			t.Fatal(err)
		}

		// The cap: 300 days of age would be +100 uncapped; the bonus holds
		// at 10 (60 < 65) and the stronger fresh task still wins.
		ancient, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project, Priority: 50})
		if err != nil {
			t.Fatal(err)
		}

		stronger, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project, Priority: 65})
		if err != nil {
			t.Fatal(err)
		}

		ancientDate := time.Now().Add(-300 * 24 * time.Hour).UnixMilli()
		if _, err := s.pool.Exec(ctx, `UPDATE tasks SET created_at = $1 WHERE id = $2`, ancientDate, ancient.ID); err != nil {
			t.Fatal(err)
		}

		got, err = s.ClaimDue(ctx, "aging-w", time.Minute)
		if err != nil {
			t.Fatal(err)
		}

		if got.ID != stronger.ID {
			t.Fatalf("aging bonus not capped: claimed %s, want %s", got.ID, stronger.ID)
		}

		// Leave nothing behind: later subtests claim against an empty ready
		// set. stronger is running (lease held); the other two never claimed —
		// cancel those.
		if err := s.Complete(ctx, stronger.ID, "aging-w", nil); err != nil {
			t.Fatal(err)
		}
		for _, id := range []task.ID{newer.ID, ancient.ID} {
			if err := s.Cancel(ctx, id, "conformance cleanup"); err != nil {
				t.Fatal(err)
			}
		}
	})

	t.Run("retry backoff ladder with evidence", func(t *testing.T) {
		retry, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project, MaxAttempts: 5})
		if err != nil {
			t.Fatal(err)
		}

		got, err := s.ClaimDue(ctx, "fail-w", time.Minute)
		if err != nil || got.ID != retry.ID {
			t.Fatalf("claim: %v (%v)", got.ID, err)
		}

		evidence := jsontext.Value(`{"stage":"verify","exit_code":2,"tail":"boom"}`)
		if err := s.Fail(ctx, retry.ID, "fail-w", "attempt failed", 90*time.Second, evidence); err != nil {
			t.Fatal(err)
		}

		after, err := s.Get(ctx, retry.ID)
		if err != nil || after.Status != task.Pending {
			t.Fatalf("after first fail: %s (%v), want pending (retry ladder)", after.Status, err)
		}

		if after.Attempts != 1 {
			t.Fatalf("attempts = %d, want 1", after.Attempts)
		}

		// NotBefore backoff: nothing claimable for this task until it passes.
		if got, err := s.ClaimDue(ctx, "fail-w", time.Minute); err == nil && got.ID == retry.ID {
			t.Fatal("claimed a task inside its backoff window")
		} else if err != nil && !errors.Is(err, queue.ErrNoTaskDue) && got.ID == retry.ID {
			t.Fatalf("claim inside backoff: %v", err)
		}

		// The failure evidence rides the task.failed fact.
		trail, err := s.FactsForTask(ctx, retry.ID.String(), 0)
		if err != nil {
			t.Fatal(err)
		}

		for _, f := range trail {
			if f.Type == journal.Failed {
				if string(f.Detail) != string(evidence) {
					t.Fatalf("task.failed detail = %s, want the executor evidence verbatim", f.Detail)
				}

				return
			}
		}

		t.Fatal("no task.failed fact on the trail")
	})

	t.Run("permanent dead-letter", func(t *testing.T) {
		perm, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project, MaxAttempts: 9})
		if err != nil {
			t.Fatal(err)
		}

		got, err := s.ClaimDue(ctx, "perm-w", time.Minute)
		if err != nil || got.ID != perm.ID {
			t.Fatalf("claim: %v (%v)", got.ID, err)
		}

		if err := s.FailPermanent(ctx, perm.ID, "perm-w", "bad payload", nil); err != nil {
			t.Fatal(err)
		}

		after, err := s.Get(ctx, perm.ID)
		if err != nil || after.Status != task.Dead {
			t.Fatalf("after FailPermanent: %s (%v), want dead", after.Status, err)
		}

		if after.Attempts != 1 {
			t.Fatalf("attempts = %d, want 1 (budget must not be burned through)", after.Attempts)
		}
	})

	t.Run("requeue burns no attempt", func(t *testing.T) {
		rq, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project})
		if err != nil {
			t.Fatal(err)
		}

		got, err := s.ClaimDue(ctx, "rq-w", time.Minute)
		if err != nil || got.ID != rq.ID {
			t.Fatalf("claim: %v (%v)", got.ID, err)
		}

		if err := s.Requeue(ctx, rq.ID, "rq-w", "dirty tree", time.Minute); err != nil {
			t.Fatal(err)
		}

		after, err := s.Get(ctx, rq.ID)
		if err != nil || after.Status != task.Pending || after.Attempts != 0 {
			t.Fatalf("after requeue: %s attempts=%d (%v), want pending/0", after.Status, after.Attempts, err)
		}
	})

	t.Run("parked requeue is not resurrectable by a stale lease", func(t *testing.T) {
		pk, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := s.ClaimDue(ctx, "park-w", time.Minute); err != nil {
			t.Fatal(err)
		}

		// Rate-limit park: delay longer than the original lease.
		if err := s.Requeue(ctx, pk.ID, "park-w", "rate limited (retry after 1h)", time.Hour); err != nil {
			t.Fatal(err)
		}

		parked, err := s.Get(ctx, pk.ID)
		if err != nil {
			t.Fatal(err)
		}

		if parked.Status != task.Pending || parked.LeaseOwner != "" || parked.LeaseExpires != nil {
			t.Fatalf("parked task must be pending with a cleared lease, got %+v", parked)
		}

		if _, err := s.ClaimDue(ctx, "park-w2", time.Minute); !errors.Is(err, queue.ErrNoTaskDue) {
			t.Fatalf("claim during park err = %v, want ErrNoTaskDue", err)
		}

		if err := s.Heartbeat(ctx, pk.ID, "park-w", time.Minute); !errors.Is(err, task.ErrLeaseNotHeld) {
			t.Errorf("stale Heartbeat err = %v, want ErrLeaseNotHeld", err)
		}

		if err := s.Complete(ctx, pk.ID, "park-w", nil); !errors.Is(err, task.ErrLeaseNotHeld) {
			t.Errorf("stale Complete err = %v, want ErrLeaseNotHeld", err)
		}

		if err := s.Requeue(ctx, pk.ID, "park-w", "stale", time.Minute); !errors.Is(err, task.ErrLeaseNotHeld) {
			t.Errorf("stale Requeue err = %v, want ErrLeaseNotHeld", err)
		}

		if err := s.Fail(
			ctx,
			pk.ID,
			"park-w",
			"stale",
			time.Minute,
			failureDetail(jsontext.Value(`"x"`), ""),
		); !errors.Is(
			err,
			task.ErrLeaseNotHeld,
		) {
			t.Errorf("stale Fail err = %v, want ErrLeaseNotHeld", err)
		}

		if got, _ := s.Get(ctx, pk.ID); got.Status != task.Pending {
			t.Fatalf("parked task mutated by stale calls: %+v", got)
		}
	})

	t.Run("cancel reason lands on the fact", func(t *testing.T) {
		cancelled, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project})
		if err != nil {
			t.Fatal(err)
		}

		if err := s.Cancel(ctx, cancelled.ID, "done by hand"); err != nil {
			t.Fatal(err)
		}

		trail, err := s.FactsForTask(ctx, cancelled.ID.String(), 0)
		if err != nil {
			t.Fatal(err)
		}

		var reason string

		var sawFact bool

		for _, f := range trail {
			if f.Type != journal.Cancelled {
				continue
			}

			sawFact = true

			var detail struct {
				Reason string `json:"reason"`
			}
			if json.Unmarshal(f.Detail, &detail) == nil {
				reason = detail.Reason
			}
		}

		if !sawFact || reason != "done by hand" {
			t.Fatalf("cancel fact reason = %q (fact seen: %v), want the stored reason", reason, sawFact)
		}
	})

	t.Run("dismiss dead cancels with reason", func(t *testing.T) {
		doomed, err := s.Enqueue(ctx, task.New{Type: "flaky", Project: project, MaxAttempts: 1})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := s.ClaimDue(ctx, "dismiss-w", time.Minute); err != nil {
			t.Fatal(err)
		}

		if err := s.Fail(ctx, doomed.ID, "dismiss-w", "boom", 0, nil); err != nil {
			t.Fatal(err)
		}

		if got, err := s.Get(ctx, doomed.ID); err != nil || got.Status != task.Dead {
			t.Fatalf("pre-dismiss status = %v (%v), want dead", got.Status, err)
		}

		if err := s.DismissDead(ctx, doomed.ID, "root cause is external", "dlqfix-sweeper"); err != nil {
			t.Fatal(err)
		}

		if got, err := s.Get(ctx, doomed.ID); err != nil || got.Status != task.Cancelled {
			t.Fatalf("post-dismiss status = %v (%v), want cancelled", got.Status, err)
		}

		trail, err := s.FactsForTask(ctx, doomed.ID.String(), 0)
		if err != nil {
			t.Fatal(err)
		}

		var detail struct {
			Reason      string `json:"reason"`
			DismissedBy string `json:"dismissed_by"`
		}

		sawFact := false

		for _, f := range trail {
			if f.Type != journal.Cancelled {
				continue
			}

			sawFact = true

			if json.Unmarshal(f.Detail, &detail) != nil {
				t.Fatalf("dismiss fact detail not JSON: %s", f.Detail)
			}
		}

		if !sawFact || detail.Reason != "root cause is external" || detail.DismissedBy != "dlqfix-sweeper" {
			t.Fatalf("dismiss fact = %+v (fact seen: %v)", detail, sawFact)
		}

		if err := s.DismissDead(ctx, doomed.ID, "again", "operator"); !errors.Is(err, task.ErrInvalidTransition) {
			t.Fatalf("double dismiss err = %v, want ErrInvalidTransition", err)
		}

		pending, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project})
		if err != nil {
			t.Fatal(err)
		}

		if err := s.DismissDead(ctx, pending.ID, "nope", "operator"); !errors.Is(err, task.ErrInvalidTransition) {
			t.Fatalf("dismiss pending err = %v, want ErrInvalidTransition", err)
		}
	})

	t.Run("cooperative cancel carries the reason", func(t *testing.T) {
		running, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project})
		if err != nil {
			t.Fatal(err)
		}

		got, err := s.ClaimDue(ctx, "coop-w", time.Minute)
		if err != nil || got.ID != running.ID {
			t.Fatalf("claim: %v (%v)", got.ID, err)
		}

		if err := s.CancelRunning(ctx, running.ID, "superseded"); err != nil {
			t.Fatal(err)
		}

		// A second request appends nothing (idempotent).
		if err := s.CancelRunning(ctx, running.ID, "superseded"); err != nil {
			t.Fatal(err)
		}

		if err := s.CancelOwned(ctx, running.ID, "coop-w"); err != nil {
			t.Fatal(err)
		}

		trail, err := s.FactsForTask(ctx, running.ID.String(), 0)
		if err != nil {
			t.Fatal(err)
		}

		requests, cancelled := 0, false

		for _, f := range trail {
			switch f.Type {
			case journal.CancelRequested:
				requests++
			case journal.Cancelled:
				cancelled = true

				var detail struct {
					Reason string `json:"reason"`
				}
				if err := json.Unmarshal(f.Detail, &detail); err != nil || detail.Reason != "superseded" {
					t.Fatalf("final cancel detail = %s, want the carried reason", f.Detail)
				}
			}
		}

		if requests != 1 || !cancelled {
			t.Fatalf("cancel trail: %d request facts (want 1, idempotent), cancelled=%v", requests, cancelled)
		}
	})

	t.Run("heartbeat guards the lease", func(t *testing.T) {
		hb, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project})
		if err != nil {
			t.Fatal(err)
		}

		got, err := s.ClaimDue(ctx, "hb-w", time.Minute)
		if err != nil || got.ID != hb.ID {
			t.Fatalf("claim: %v (%v)", got.ID, err)
		}

		if err := s.Heartbeat(ctx, hb.ID, "hb-w", time.Minute); err != nil {
			t.Fatalf("heartbeat by owner: %v", err)
		}

		if err := s.Heartbeat(ctx, hb.ID, "stranger", time.Minute); !errors.Is(err, task.ErrLeaseNotHeld) {
			t.Fatalf("foreign heartbeat err = %v, want ErrLeaseNotHeld", err)
		}
	})

	t.Run("filter list and count read surface", func(t *testing.T) {
		st := task.Pending
		typ := "battery-type"

		for range 3 {
			if _, err := s.Enqueue(ctx, task.New{Type: typ, Project: project}); err != nil {
				t.Fatal(err)
			}
		}

		proj := project

		filtered, err := s.List(ctx, queue.Filter{Project: &proj, Type: &typ, Status: &st})
		if err != nil {
			t.Fatal(err)
		}

		if len(filtered) < 3 {
			t.Fatalf("filtered list = %d, want >= 3 (project+type+status)", len(filtered))
		}

		for _, tk := range filtered {
			if tk.Project != project || tk.Type != typ || tk.Status != task.Pending {
				t.Fatalf("filter leaked: %+v", tk)
			}
		}

		page, err := s.List(ctx, queue.Filter{Project: &proj, Type: &typ, Limit: 2})
		if err != nil {
			t.Fatal(err)
		}

		if len(page) != 2 {
			t.Fatalf("page size = %d, want 2 (limit)", len(page))
		}

		count, err := s.CountTasks(ctx, queue.Filter{Project: &proj, Type: &typ})
		if err != nil || count < 3 {
			t.Fatalf("count = %d (%v), want >= 3", count, err)
		}

		byQuery, err := s.List(ctx, queue.Filter{Query: strings.ToLower(typ)})
		if err != nil || len(byQuery) < 3 {
			t.Fatalf("query list = %d (%v), want >= 3 (type substring match)", len(byQuery), err)
		}
	})

	t.Run("bounded per-task fact reads", func(t *testing.T) {
		// Priority tops the filter subtest's leftover pending tasks so the
		// claim deterministically lands here.
		bounded, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project, Priority: 100})
		if err != nil {
			t.Fatal(err)
		}

		got, err := s.ClaimDue(ctx, "bnd-w", time.Minute)
		if err != nil || got.ID != bounded.ID {
			t.Fatalf("claim: %v (%v)", got.ID, err)
		}

		if err := s.Complete(ctx, bounded.ID, "bnd-w", nil); err != nil {
			t.Fatal(err)
		}

		trail, err := s.FactsForTask(ctx, bounded.ID.String(), 2)
		if err != nil {
			t.Fatal(err)
		}

		if len(trail) != 2 {
			t.Fatalf("bounded trail = %d facts, want exactly 2 (most recent)", len(trail))
		}

		if trail[0].Type != journal.Claimed || trail[1].Type != journal.Completed {
			t.Fatalf(
				"bounded trail = [%s %s], want [claimed completed] (most recent two)",
				trail[0].Type,
				trail[1].Type,
			)
		}
	})

	// The subtests below close the suite-parity gaps found in the 2026-09-10
	// name-level diff against the sqlite white-box suite: dedup, watermarks,
	// head seq, lease-expiry reclaim, exactly-once concurrent claims (the
	// SKIP LOCKED differentiator), and LIKE-metacharacter escaping.

	t.Run("dedup key enqueues once", func(t *testing.T) {
		dedup := project + "-dedup"

		first, err := s.Enqueue(ctx, task.New{Project: dedup, Type: "sh", DedupKey: "conformance:dedup"})
		if err != nil {
			t.Fatal(err)
		}

		second, err := s.Enqueue(ctx, task.New{Project: dedup, Type: "sh", DedupKey: "conformance:dedup"})
		if err != nil {
			t.Fatal(err)
		}

		if first.ID != second.ID {
			t.Fatalf("dedup enqueue returned a new task: %s vs %s", first.ID, second.ID)
		}

		p := dedup

		tasks, err := s.List(ctx, queue.Filter{Project: &p})
		if err != nil {
			t.Fatal(err)
		}

		if len(tasks) != 1 {
			t.Fatalf("project holds %d tasks, want 1 (dedup suppressed the twin)", len(tasks))
		}
	})

	t.Run("watermark roundtrip is monotonic", func(t *testing.T) {
		consumer := "conformance-wm-" + project

		if seq, exists, err := s.Watermark(ctx, consumer); err != nil || exists || seq != 0 {
			t.Fatalf("absent watermark = %d/%v (%v), want 0/false", seq, exists, err)
		}

		if err := s.SaveWatermark(ctx, consumer, 100); err != nil {
			t.Fatal(err)
		}

		if err := s.SaveWatermark(ctx, consumer, 30); err != nil {
			t.Fatalf("rewind write must not error: %v", err)
		}

		seq, exists, err := s.Watermark(ctx, consumer)
		if err != nil || !exists || seq != 100 {
			t.Fatalf("watermark after rewind = %d/%v (%v), want 100/true", seq, exists, err)
		}
	})

	t.Run("head seq advances with facts", func(t *testing.T) {
		before, err := s.HeadSeq(ctx)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project, Priority: 250}); err != nil {
			t.Fatal(err)
		}

		after, err := s.HeadSeq(ctx)
		if err != nil {
			t.Fatal(err)
		}

		if after <= before {
			t.Fatalf("head seq %d did not advance past %d after an enqueue fact", after, before)
		}
	})

	t.Run("lease expiry allows reclaim", func(t *testing.T) {
		tk, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project, Priority: 260})
		if err != nil {
			t.Fatal(err)
		}

		got, err := s.ClaimDue(ctx, "crashed-worker", 30*time.Millisecond)
		if err != nil || got.ID != tk.ID {
			t.Fatalf("claim: %v (%v)", got.ID, err)
		}

		time.Sleep(60 * time.Millisecond)

		got, err = s.ClaimDue(ctx, "reclaimer", time.Minute)
		if err != nil {
			t.Fatalf("reclaim: %v", err)
		}

		if got.ID != tk.ID || got.LeaseOwner != "reclaimer" {
			t.Fatalf("reclaimed by wrong task/owner: %s/%s", got.ID, got.LeaseOwner)
		}

		if err := s.Complete(ctx, tk.ID, "crashed-worker", nil); !errors.Is(err, task.ErrLeaseNotHeld) {
			t.Fatalf("stale owner complete err = %v, want ErrLeaseNotHeld", err)
		}
	})

	t.Run("concurrent claims are exactly-once", func(t *testing.T) {
		// THE Postgres differentiator: SKIP LOCKED must hand each pending
		// task to exactly one of the racing workers.
		const workers = 6

		for range workers {
			if _, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project, Priority: 230}); err != nil {
				t.Fatal(err)
			}
		}

		var wg sync.WaitGroup

		claimed := make(chan task.ID, workers)

		prefix := "race-" + project + "-"

		for i := range workers {
			owner := fmt.Sprintf("%sw%d", prefix, i)

			wg.Go(func() {
				tk, err := s.ClaimDue(ctx, owner, time.Minute)
				if err == nil {
					claimed <- tk.ID
				}
			})
		}

		wg.Wait()
		close(claimed)

		seen := make(map[task.ID]bool)

		for id := range claimed {
			if seen[id] {
				t.Fatalf("task %s claimed by more than one worker — SKIP LOCKED broken", id)
			}

			seen[id] = true
		}

		if len(seen) != workers {
			t.Fatalf("distinct tasks claimed = %d, want %d (one per worker)", len(seen), workers)
		}
	})

	t.Run("LIKE metacharacters stay literal", func(t *testing.T) {
		esc := "esc-" + project

		seed := []task.New{
			{Project: esc, Type: "sh", Payload: jsontext.Value(`"progress 100% done"`)},
			{Project: esc, Type: "sh", Payload: jsontext.Value(`"snake_case_name"`)},
		}

		for i := range seed {
			if _, err := s.Enqueue(ctx, seed[i]); err != nil {
				t.Fatalf("seed %d: %v", i, err)
			}
		}

		p := esc
		cases := []struct {
			query string
			want  int
		}{
			{"100%", 1},
			{"snake_case", 1},
			{"1% done", 0},
			{"snakeXcase", 0},
		}

		for _, tt := range cases {
			tasks, err := s.List(ctx, queue.Filter{Project: &p, Query: tt.query})
			if err != nil {
				t.Fatalf("query %q: %v", tt.query, err)
			}

			if len(tasks) != tt.want {
				t.Fatalf(
					"query %q matched %d tasks, want %d (LIKE metacharacters must stay literal)",
					tt.query,
					len(tasks),
					tt.want,
				)
			}
		}
	})
}
