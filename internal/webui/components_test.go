package webui

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/task"
	"github.com/larsartmann/templ-components/display"
)

// The mapping helpers are the single source of the dashboard's color and
// label language (components.go doc comment); these tables pin every
// domain value onto its visual counterpart so tone drift is a test failure.

func TestStatusBadgeType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		status task.Status
		want   display.BadgeType
	}{
		{task.Pending, display.BadgeWarning},
		{task.Running, display.BadgeInfo},
		{task.Completed, display.BadgeSuccess},
		{task.Dead, display.BadgeError},
		{task.Cancelled, display.BadgeNeutral},
		{task.Status("bogus"), display.BadgeNeutral},
	}

	for _, tc := range cases {
		if got := statusBadgeType(tc.status); got != tc.want {
			t.Errorf("statusBadgeType(%s) = %v, want %v", tc.status, got, tc.want)
		}
	}
}

func TestFactTone(t *testing.T) {
	t.Parallel()

	cases := []struct {
		factType journal.FactType
		want     display.ScrollbackTone
	}{
		{journal.Enqueued, display.ScrollbackToneNeutral},
		{journal.Claimed, display.ScrollbackToneInfo},
		{journal.Heartbeat, display.ScrollbackToneNeutral},
		{journal.Completed, display.ScrollbackToneSuccess},
		{journal.Failed, display.ScrollbackToneWarning},
		{journal.DeadLettered, display.ScrollbackToneDanger},
		{journal.Cancelled, display.ScrollbackToneNeutral},
		{journal.CancelRequested, display.ScrollbackToneWarning},
		{journal.Released, display.ScrollbackToneWarning},
		{journal.Requeued, display.ScrollbackToneWarning},
		{journal.FactType("task.unknown"), display.ScrollbackToneNeutral},
	}

	for _, tc := range cases {
		if got := factTone(tc.factType); got != tc.want {
			t.Errorf("factTone(%s) = %v, want %v", tc.factType, got, tc.want)
		}
	}
}

func TestShortIDTail(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in, want string
	}{
		{"short", "short"},                          // under the cut: unchanged
		{"12345678", "12345678"},                    // exactly the cut: unchanged
		{"0001020304050607aabbccddee", "…bbccddee"}, // over the cut: ellipsis + last 8
	}

	for _, tc := range cases {
		if got := shortIDTail(tc.in); got != tc.want {
			t.Errorf("shortIDTail(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFactTimestamp(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)

	if got := factTimestamp(now, now.Add(-time.Minute)); got != "11:59:00" {
		t.Errorf("same-day fact timestamp = %q, want %q", got, "11:59:00")
	}

	old := now.Add(-25 * time.Hour) // 2026-09-07 11:00 — crosses the day line
	if got := factTimestamp(now, old); got != old.Format("01-02 15:04:05") {
		t.Errorf("old fact timestamp = %q, want %q", got, old.Format("01-02 15:04:05"))
	}
}

func TestStatusHref(t *testing.T) {
	t.Parallel()

	cases := []struct {
		status task.Status
		want   string
	}{
		{task.Pending, "/?status=pending"},
		{task.Running, "/?status=running"},
		{task.Completed, "/?status=completed"},
		{task.Dead, "/?status=dead"},
		{task.Cancelled, "/?status=cancelled"},
	}

	for _, tc := range cases {
		if got := statusHref(tc.status); got != tc.want {
			t.Errorf("statusHref(%s) = %q, want %q", tc.status, got, tc.want)
		}
	}
}

func TestTaskRowClass(t *testing.T) {
	t.Parallel()

	base := func(st task.Status) string {
		return "row-" + string(st) + " hover:bg-gray-100 dark:hover:bg-gray-800"
	}

	if got := taskRowClass(task.Task{Status: task.Running}); got != base(task.Running) {
		t.Errorf("running row class = %q, want %q", got, base(task.Running))
	}

	dead := taskRowClass(task.Task{Status: task.Dead})
	if !strings.HasPrefix(dead, base(task.Dead)) || !strings.Contains(dead, "bg-red-50/50") {
		t.Errorf("dead row class = %q, want base %q + alarm tint", dead, base(task.Dead))
	}
}

func TestDetailItems(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	done := now.Add(-time.Minute)

	t.Run("full record", func(t *testing.T) {
		items := detailItems(task.Task{
			Project:     "demo",
			Type:        "sh",
			Status:      task.Completed,
			Priority:    7,
			Attempts:    2,
			MaxAttempts: 3,
			LeaseOwner:  "worker-1",
			Payload:     json.RawMessage(`"echo hi"`),
			CreatedAt:   now.Add(-2 * time.Hour),
			UpdatedAt:   now.Add(-2 * time.Minute),
			CompletedAt: &done,
		}, now)

		var terms []string

		for _, it := range items {
			terms = append(terms, it.Term)
		}

		want := []string{
			labelProject, labelType, labelStatus, labelAttempts, "priority", "created", "updated",
			"lease owner", labelCompleted, "payload",
		}

		if strings.Join(terms, ",") != strings.Join(want, ",") {
			t.Fatalf("terms = %v, want %v", terms, want)
		}

		if last := items[len(items)-1]; last.Term != "payload" || last.DetailComponent == nil {
			t.Errorf("payload must be the final item and render as a component, got %+v", last)
		}
	})

	t.Run("leased-out task omits conditional terms", func(t *testing.T) {
		items := detailItems(task.Task{
			Project:   "demo",
			Type:      "sh",
			Status:    task.Pending,
			Payload:   json.RawMessage(`{}`),
			CreatedAt: now,
			UpdatedAt: now,
		}, now)

		for _, it := range items {
			switch it.Term {
			case "lease owner", labelCompleted:
				t.Errorf("term %q must be absent without the data", it.Term)
			}
		}
	})
}
