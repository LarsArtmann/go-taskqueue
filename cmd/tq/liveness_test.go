package main

import (
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestBuildLivenessView pins the tq show lease-staleness surface: an
// expired lease on a running task reads stale (crash-reclaim candidate),
// a future not_before reads parked, and a healthy task reads neither.
func TestBuildLivenessView(t *testing.T) {
	now := time.Now()

	future := now.Add(10 * time.Minute)
	past := now.Add(-10 * time.Minute)

	cases := []struct {
		name       string
		task       task.Task
		wantStale  bool
		wantParked bool
	}{
		{
			"healthy-running-lease",
			task.Task{Status: task.Running, LeaseOwner: "w1", LeaseExpires: &future},
			false,
			false,
		},
		{"stale-lease", task.Task{Status: task.Running, LeaseOwner: "w1", LeaseExpires: &past}, true, false},
		{"parked", task.Task{Status: task.Pending, NotBefore: future}, false, true},
		{"past-notbefore", task.Task{Status: task.Pending, NotBefore: past}, false, false},
		{"no-lease-no-park", task.Task{Status: task.Completed}, false, false},
	}

	for _, tc := range cases {
		v := buildLivenessView(tc.task)
		if v.LeaseStale != tc.wantStale {
			t.Errorf("%s: LeaseStale = %v, want %v", tc.name, v.LeaseStale, tc.wantStale)
		}

		if v.Parked != tc.wantParked {
			t.Errorf("%s: Parked = %v, want %v", tc.name, v.Parked, tc.wantParked)
		}
	}

	parked := buildLivenessView(task.Task{NotBefore: future})
	if parked.NotBefore == "" {
		t.Error("parked: NotBefore must be rendered")
	}

	stale := buildLivenessView(task.Task{LeaseExpires: &past})
	if stale.LeaseExpires == "" {
		t.Error("stale: LeaseExpires must be rendered")
	}
}

// TestNotBeforeCell pins the tq tasks NOTBEFORE column: future not_before
// renders a countdown, past or zero renders "-".
func TestNotBeforeCell(t *testing.T) {
	now := time.Now()

	if got := notBeforeCell(task.Task{}, now); got != "-" {
		t.Errorf("zero notBefore = %q, want \"-\"", got)
	}

	if got := notBeforeCell(task.Task{NotBefore: now.Add(-time.Minute)}, now); got != "-" {
		t.Errorf("past notBefore = %q, want \"-\"", got)
	}

	if got := notBeforeCell(task.Task{NotBefore: now.Add(90 * time.Second)}, now); got == "-" || got == "" {
		t.Errorf("future notBefore = %q, want a countdown", got)
	}
}
