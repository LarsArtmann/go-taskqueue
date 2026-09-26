package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/session"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestStatsJSONParkedContract pins the 16-00 f33 wire shape: `tq stats
// --json` carries the parked count under the `parked` key — present with
// the rate-limit-parked total when tasks are parked, omitted entirely when
// none are (omitempty), so dashboards can distinguish "0 parked" from a
// pre-parked-era payload.
func TestStatsJSONParkedContract(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats.db")

	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx := context.Background()

	park := func(project string, delay time.Duration) {
		t.Helper()

		tk, err := s.Enqueue(ctx, task.New{Type: "agent", Project: project})
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}

		if _, claimW1, err := s.ClaimDue(ctx, "w1", time.Minute); err != nil {
			t.Fatalf("claim: %v", err)
		} else if err := s.Requeue(ctx, tk.ID, claimW1, "rate limited", delay, false); err != nil {
			t.Fatalf("requeue: %v", err)
		}
	}

	// Bare store: no parked key at all.
	out := captureStdout(t, func() {
		if err := cmdStats([]string{"--db", path, "--json"}); err != nil {
			t.Errorf("cmdStats: %v", err)
		}
	})

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("stats payload not JSON: %v (%s)", err, out)
	}

	if _, ok := payload["parked"]; ok {
		t.Errorf("bare store carried a parked key: %s", out)
	}

	// Two parked tasks: the key appears with the exact count.
	park("demo", time.Hour)
	park("demo", time.Hour)

	out = captureStdout(t, func() {
		if err := cmdStats([]string{"--db", path, "--json"}); err != nil {
			t.Errorf("cmdStats: %v", err)
		}
	})

	payload = map[string]any{}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("stats payload not JSON: %v (%s)", err, out)
	}

	if got := payload["parked"]; got != float64(2) {
		t.Errorf("parked = %v, want 2 (%s)", got, out)
	}

	// The human output carries the parked line only while tasks are parked.
	human := captureStdout(t, func() {
		if err := cmdStats([]string{"--db", path}); err != nil {
			t.Errorf("cmdStats: %v", err)
		}
	})

	if !strings.Contains(human, "parked") {
		t.Errorf("human stats output lost the parked line:\n%s", human)
	}
}

// TestStatsJSONSessionUsageContract pins the budget token/cost projection
// surface (09-52 f1): `tq stats --json` carries the derived session usage
// under `budget.session_usage` only while some completion fact carried
// usage (omitempty via the nil pointer), and the human output names the
// line alongside the enqueued count.
func TestStatsJSONSessionUsageContract(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats-usage.db")

	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx := context.Background()

	out := captureStdout(t, func() {
		if err := cmdStats([]string{"--db", path, "--json"}); err != nil {
			t.Errorf("cmdStats: %v", err)
		}
	})

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("stats payload not JSON: %v (%s)", err, out)
	}

	budget, _ := payload["budget"].(map[string]any)
	if budget == nil {
		t.Fatalf("bare store lost the budget object: %s", out)
	}

	if _, ok := budget["session_usage"]; ok {
		t.Errorf("bare store carried a budget.session_usage key: %s", out)
	}

	// One completed agent task with derived usage: the projection appears
	// with the exact sums.
	tk, err := s.Enqueue(ctx, task.New{Type: "agent", Project: "demo"})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	_, claimW1, err := s.ClaimDue(ctx, "w1", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	result := []byte(`{"session_id":"s1","session_cost_usd":0.42,"session_prompt_tokens":1200,"session_completion_tokens":3400,"session_message_count":9}`)
	if err := s.Complete(ctx, tk.ID, claimW1, result); err != nil {
		t.Fatalf("complete: %v", err)
	}

	out = captureStdout(t, func() {
		if err := cmdStats([]string{"--db", path, "--json"}); err != nil {
			t.Errorf("cmdStats: %v", err)
		}
	})

	payload = map[string]any{}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("stats payload not JSON: %v (%s)", err, out)
	}

	budget, _ = payload["budget"].(map[string]any)
	usage, _ := budget["session_usage"].(map[string]any)
	if usage == nil {
		t.Fatalf("completed usage lost the budget.session_usage key: %s", out)
	}

	if got := usage["runs"]; got != float64(1) {
		t.Errorf("session_usage.runs = %v, want 1 (%s)", got, out)
	}

	if got := usage["prompt_tokens"]; got != float64(1200) {
		t.Errorf("session_usage.prompt_tokens = %v, want 1200 (%s)", got, out)
	}

	if got := usage["cost_usd"]; got != float64(0.42) {
		t.Errorf("session_usage.cost_usd = %v, want 0.42 (%s)", got, out)
	}

	human := captureStdout(t, func() {
		if err := cmdStats([]string{"--db", path}); err != nil {
			t.Errorf("cmdStats: %v", err)
		}
	})

	if !strings.Contains(human, "derived runs") {
		t.Errorf("human stats output lost the session-usage line:\n%s", human)
	}
}

// TestStatsOpenSessions pins the stale-open-session visibility surface
// (03-28 §f9): `tq stats --json` carries the open-session count under
// `open_sessions` (omitted when none), and the human output names the
// line only while sessions are open.
func TestStatsOpenSessions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats.db")

	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx := context.Background()

	// Bare store: no open_sessions key at all.
	out := captureStdout(t, func() {
		if err := cmdStats([]string{"--db", path, "--json"}); err != nil {
			t.Errorf("cmdStats: %v", err)
		}
	})

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("stats payload not JSON: %v (%s)", err, out)
	}

	if _, ok := payload["open_sessions"]; ok {
		t.Errorf("bare store carried an open_sessions key: %s", out)
	}

	if err := session.Begin(ctx, s, "sess-stats", "/tmp/repo", "repo"); err != nil {
		t.Fatalf("session begin: %v", err)
	}

	out = captureStdout(t, func() {
		if err := cmdStats([]string{"--db", path, "--json"}); err != nil {
			t.Errorf("cmdStats: %v", err)
		}
	})

	payload = map[string]any{}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("stats payload not JSON: %v (%s)", err, out)
	}

	if got := payload["open_sessions"]; got != float64(1) {
		t.Errorf("open_sessions = %v, want 1 (%s)", got, out)
	}

	human := captureStdout(t, func() {
		if err := cmdStats([]string{"--db", path}); err != nil {
			t.Errorf("cmdStats: %v", err)
		}
	})

	if !strings.Contains(human, "open sessions") {
		t.Errorf("human stats output lost the open-sessions line:\n%s", human)
	}
}

// TestStatsSessionVolume pins the 03-28 §f20 surface: `tq stats` carries
// the session lifecycle fact counts (`sessions_opened` / `sessions_closed`,
// omitempty in JSON) and a human "sessions" line once any session fact
// exists — the volume read alongside the open-sessions lamp.
func TestStatsSessionVolume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats.db")

	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = s.Close() }()

	ctx := context.Background()

	out := captureStdout(t, func() {
		if err := cmdStats([]string{"--db", path, "--json"}); err != nil {
			t.Errorf("cmdStats: %v", err)
		}
	})

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("stats payload not JSON: %v (%s)", err, out)
	}

	if _, ok := payload["sessions_opened"]; ok {
		t.Errorf("bare store carried a sessions_opened key: %s", out)
	}

	if err := session.Begin(ctx, s, "sess-vol", "/tmp/repo", "repo"); err != nil {
		t.Fatalf("session begin: %v", err)
	}

	if err := s.AppendFact(ctx, journal.Fact{
		TaskID: string(session.SyntheticTaskID("sess-vol")),
		Type:   journal.SessionClosed,
	}); err != nil {
		t.Fatalf("append session.closed: %v", err)
	}

	out = captureStdout(t, func() {
		if err := cmdStats([]string{"--db", path, "--json"}); err != nil {
			t.Errorf("cmdStats: %v", err)
		}
	})

	payload = map[string]any{}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("stats payload not JSON: %v (%s)", err, out)
	}

	if got := payload["sessions_opened"]; got != float64(1) {
		t.Errorf("sessions_opened = %v, want 1 (%s)", got, out)
	}

	if got := payload["sessions_closed"]; got != float64(1) {
		t.Errorf("sessions_closed = %v, want 1 (%s)", got, out)
	}

	human := captureStdout(t, func() {
		if err := cmdStats([]string{"--db", path}); err != nil {
			t.Errorf("cmdStats: %v", err)
		}
	})

	if !strings.Contains(human, "opened / 1 closed") {
		t.Errorf("human stats output lost the sessions volume line:\n%s", human)
	}
}
