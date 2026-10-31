package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
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

	if err := store.Requeue(
		ctx,
		tk.ID,
		claim,
		"rate limited",
		time.Hour,
		false,
		queue.RequeueClassRateLimit,
		"zai",
	); err != nil {
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
	printTaskListTo(&buf, []task.Task{parked}, 0, func() (int, error) { return 1, nil }, providers, nil, nil)

	if out := buf.String(); !strings.Contains(out, "[zai] rate limited") {
		t.Fatalf("parked row = %q, want a [zai] tag on the last-error cell", out)
	}
}

// TestParkedJSONSurfacesProvider pins the parked JSON surface (2026-10-15
// 01-15 §b2): each parked row carries the requeue-evidence provider so
// JSON consumers get the one-glance "which provider" answer without
// journal reads; non-parked JSON stays the plain task array, and the
// envelope wraps the same provider-bearing rows.
func TestParkedJSONSurfacesProvider(t *testing.T) {
	ctx := context.Background()

	path := filepath.Join(t.TempDir(), "q.db")

	seed, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}

	if _, err := seed.Enqueue(ctx, task.New{Type: "agent"}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	tk, claim, err := seed.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if err := seed.Requeue(
		ctx,
		tk.ID,
		claim,
		"rate limited",
		time.Hour,
		false,
		queue.RequeueClassRateLimit,
		"zai",
	); err != nil {
		t.Fatalf("requeue: %v", err)
	}

	if err := seed.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	var rows []parkedTaskRow

	out := captureStdout(t, func() {
		if err := cmdTasks([]string{"--parked", "--json", "--db", path}); err != nil {
			t.Errorf("cmdTasks --parked --json: %v", err)
		}
	})

	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}

	if len(rows) != 1 || rows[0].Provider != "zai" || rows[0].ID != tk.ID {
		t.Fatalf("parked rows = %+v, want %s with provider zai", rows, tk.ID)
	}

	var env struct {
		Tasks     []parkedTaskRow `json:"tasks"`
		Total     int             `json:"total"`
		Truncated bool            `json:"truncated"`
	}

	out = captureStdout(t, func() {
		if err := cmdTasks([]string{"--parked", "--json", "--json-envelope", "--db", path}); err != nil {
			t.Errorf("cmdTasks --parked --json --json-envelope: %v", err)
		}
	})

	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("unmarshal envelope %q: %v", out, err)
	}

	if env.Total != 1 || len(env.Tasks) != 1 || env.Tasks[0].Provider != "zai" {
		t.Fatalf("envelope = %+v, want one zai row", env)
	}

	var plain []task.Task

	out = captureStdout(t, func() {
		if err := cmdTasks([]string{"--json", "--db", path}); err != nil {
			t.Errorf("cmdTasks --json: %v", err)
		}
	})

	if err := json.Unmarshal([]byte(out), &plain); err != nil {
		t.Fatalf("unmarshal plain %q: %v", out, err)
	}

	if len(plain) != 1 {
		t.Fatalf("plain rows = %+v, want the unmodified task array", plain)
	}
}
