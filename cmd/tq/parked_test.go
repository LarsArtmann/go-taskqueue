package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestParkedByClassJoinsFacts pins the --parked-class join against a real
// store: only the task whose latest requeue evidence carries the class AND
// whose not_before window is still open matches.
func TestParkedByClassJoinsFacts(t *testing.T) {
	store := journalAuditStore(t)
	ctx := context.Background()

	budgetParked, err := store.Enqueue(ctx, task.New{Type: "sh"})
	if err != nil {
		t.Fatalf("enqueue parked: %v", err)
	}

	if _, err := store.Enqueue(ctx, task.New{Type: "sh"}); err != nil {
		t.Fatalf("enqueue plain: %v", err)
	}

	tk, claim, err := store.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if tk.ID != budgetParked.ID {
		t.Fatalf("claim = %s, want %s", tk.ID, budgetParked.ID)
	}

	if err := store.Requeue(
		ctx,
		tk.ID,
		claim,
		"daily cap spent",
		time.Hour,
		false,
		queue.RequeueClassBudget,
		""); err != nil {
		t.Fatalf("requeue: %v", err)
	}

	tasks, err := store.List(ctx, queue.Filter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	facts, err := store.LastFacts(ctx, factTailLimit)
	if err != nil {
		t.Fatalf("facts: %v", err)
	}

	latest := latestRequeueClass(facts)
	now := time.Now()

	budget := parkedByClass(tasks, latest, queue.RequeueClassBudget, now)
	if len(budget) != 1 || budget[0].ID != budgetParked.ID {
		t.Fatalf("budget-parked = %+v, want only %s", budget, budgetParked.ID)
	}

	if rl := parkedByClass(tasks, latest, queue.RequeueClassRateLimit, now); len(rl) != 0 {
		t.Fatalf("rate-limit-parked = %+v, want none", rl)
	}
}

// TestParkedProviderJoinsOntoRows pins the parked surface's provider tag
// (04-03 §f8): the latest requeue evidence's provider renders as a
// "[zai] …" prefix on the row, so "which provider is exhausted?" is one
// glance; a task whose evidence carries no provider renders untagged.
func TestParkedProviderJoinsOntoRows(t *testing.T) {
	ctx := context.Background()
	store := journalAuditStore(t)
	defer store.Close()

	if _, err := store.Enqueue(ctx, task.New{Type: "agent"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	tk, claim, err := store.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if err := store.Requeue(ctx, tk.ID, claim, "rate limited", time.Hour, false, queue.RequeueClassRateLimit, "zai"); err != nil {
		t.Fatalf("requeue: %v", err)
	}

	facts, err := store.LastFacts(ctx, factTailLimit)
	if err != nil {
		t.Fatalf("facts: %v", err)
	}

	providers := latestRequeueProvider(facts)
	if providers[tk.ID.String()] != "zai" {
		t.Fatalf("provider = %q, want zai", providers[tk.ID.String()])
	}

	parked, err := store.Get(ctx, tk.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	var buf bytes.Buffer
	printTaskListTo(&buf, []task.Task{parked}, 0, func() (int, error) { return 1, nil }, providers)

	if out := buf.String(); !strings.Contains(out, "[zai] rate limited") {
		t.Fatalf("parked row = %q, want a [zai] tag on the last-error cell", out)
	}
}
