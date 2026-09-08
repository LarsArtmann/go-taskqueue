package queue

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
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

	t.Run("retry backoff ladder with evidence", func(t *testing.T) {
		retry, err := s.Enqueue(ctx, task.New{Type: "sh", Project: project, MaxAttempts: 5})
		if err != nil {
			t.Fatal(err)
		}

		got, err := s.ClaimDue(ctx, "fail-w", time.Minute)
		if err != nil || got.ID != retry.ID {
			t.Fatalf("claim: %v (%v)", got.ID, err)
		}

		evidence := json.RawMessage(`{"stage":"verify","exit_code":2,"tail":"boom"}`)
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
		} else if err != nil && !errors.Is(err, ErrNoTaskDue) && got.ID == retry.ID {
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
		filtered, err := s.List(ctx, Filter{Project: &proj, Type: &typ, Status: &st})
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

		page, err := s.List(ctx, Filter{Project: &proj, Type: &typ, Limit: 2})
		if err != nil {
			t.Fatal(err)
		}

		if len(page) != 2 {
			t.Fatalf("page size = %d, want 2 (limit)", len(page))
		}

		count, err := s.CountTasks(ctx, Filter{Project: &proj, Type: &typ})
		if err != nil || count < 3 {
			t.Fatalf("count = %d (%v), want >= 3", count, err)
		}

		byQuery, err := s.List(ctx, Filter{Query: strings.ToLower(typ)})
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
			t.Fatalf("bounded trail = [%s %s], want [claimed completed] (most recent two)", trail[0].Type, trail[1].Type)
		}
	})
}
