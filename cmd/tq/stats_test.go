package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
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

		if _, err := s.ClaimDue(ctx, "w1", time.Minute); err != nil {
			t.Fatalf("claim: %v", err)
		}

		if err := s.Requeue(ctx, tk.ID, "w1", "rate limited", delay, false); err != nil {
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
