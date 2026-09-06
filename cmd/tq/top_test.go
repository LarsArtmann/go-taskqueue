package main

import (
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func topTask(id, project string, status task.Status) task.Task {
	return task.Task{ID: task.ID(id), Project: project, Status: status}
}

func TestAggregateTopCountsMatchTasks(t *testing.T) {
	tasks := []task.Task{
		topTask("t1", "alpha", task.Pending),
		topTask("t2", "alpha", task.Running),
		topTask("t3", "alpha", task.Completed),
		topTask("t4", "beta", task.Dead),
		topTask("t5", "beta", task.Cancelled),
		topTask("t6", "beta", task.Pending),
	}
	now := time.Now()

	got := aggregateTop(tasks, nil, now)
	if len(got) != 2 {
		t.Fatalf("got %d projects, want 2: %+v", len(got), got)
	}
	// Projects are sorted by name.
	if got[0].Project != "alpha" || got[1].Project != "beta" {
		t.Fatalf("projects not sorted: %q, %q", got[0].Project, got[1].Project)
	}

	a, b := got[0], got[1]
	if a.Pending != 1 || a.Running != 1 || a.Completed != 1 || a.Dead != 0 || a.Cancelled != 0 {
		t.Errorf("alpha counts wrong: %+v", a)
	}

	if b.Pending != 1 || b.Running != 0 || b.Completed != 0 || b.Dead != 1 || b.Cancelled != 1 {
		t.Errorf("beta counts wrong: %+v", b)
	}
}

func TestAggregateTopLastDurFromFacts(t *testing.T) {
	base := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	facts := []journal.Fact{
		{Seq: 1, TaskID: "t1", Type: journal.Claimed, Time: base},
		{Seq: 2, TaskID: "t1", Type: journal.Completed, Time: base.Add(2500 * time.Millisecond)},
		// A reclaimed task: first claim, lease released, second claim, done.
		{Seq: 3, TaskID: "t2", Type: journal.Claimed, Time: base.Add(10 * time.Second)},
		{Seq: 4, TaskID: "t2", Type: journal.Claimed, Time: base.Add(20 * time.Second)},
		{Seq: 5, TaskID: "t2", Type: journal.Completed, Time: base.Add(25 * time.Second)},
	}
	tasks := []task.Task{
		topTask("t1", "alpha", task.Completed),
		topTask("t2", "alpha", task.Completed),
		topTask("t3", "beta", task.Pending),
	}

	got := aggregateTop(tasks, facts, base.Add(time.Minute))
	if len(got) != 2 {
		t.Fatalf("got %d projects, want 2", len(got))
	}

	a := got[0]
	if !a.HasLast {
		t.Fatalf("alpha HasLast = false, want true")
	}
	// The most recent completion is t2's: 25s - 20s (second claim) = 5s.
	if a.LastDur != 5*time.Second {
		t.Errorf("alpha LastDur = %s, want 5s (winning attempt, not first claim)", a.LastDur)
	}

	if b := got[1]; b.HasLast || b.HasActive {
		t.Errorf("beta has no runs, got last=%v active=%v", b.LastDur, b.ActiveDur)
	}
}

func TestAggregateTopActiveDurOfRunningTask(t *testing.T) {
	base := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	facts := []journal.Fact{
		{Seq: 1, TaskID: "t1", Type: journal.Claimed, Time: base},
	}
	tasks := []task.Task{topTask("t1", "alpha", task.Running)}
	now := base.Add(90 * time.Second)
	got := aggregateTop(tasks, facts, now)

	v := got[0]
	if !v.HasActive {
		t.Fatal("HasActive = false, want true")
	}

	if v.ActiveDur != 90*time.Second {
		t.Errorf("ActiveDur = %s, want 1m30s", v.ActiveDur)
	}

	if v.Running != 1 {
		t.Errorf("Running = %d, want 1", v.Running)
	}
}

func TestAggregateTopEmpty(t *testing.T) {
	got := aggregateTop(nil, nil, time.Now())
	if len(got) != 0 {
		t.Errorf("expected no rows, got %+v", got)
	}
}
