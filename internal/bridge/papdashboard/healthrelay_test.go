package papdashboard

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

// postSnapshot delivers one webhook snapshot to the relay, the way the
// go-health-dashboard pusher would.
func postSnapshot(t *testing.T, url, status string, checks map[string]healthRelayCheck) {
	t.Helper()

	body, err := json.Marshal(healthSnapshot{Status: status, Version: "test", Checks: checks})
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	resp, err := http.Post(url, "application/json", bytes.NewReader(body)) //nolint:noctx // test-side direct POST
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("relay answered %d, want 204", resp.StatusCode)
	}
}

// waitForCalls polls the fake until it holds n ingests (the relay forwards
// on the request's goroutine).
func waitForCalls(t *testing.T, f *fakePap, n int) []recordedIngest {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for {
		calls := f.calls()
		if len(calls) >= n {
			return calls
		}

		if time.Now().After(deadline) {
			t.Fatalf("ingest calls = %d, want %d", len(calls), n)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

// TestHealthRelayTransitionsToAlerts pins the relay's translation contract:
// a healthy first observation raises nothing, the fall into warn/fail
// raises alert.triggered naming the degraded checks, recovery resolves the
// alert, and a repeated status is silent (fleet review f13: store down,
// wedged fold, follower demotion must reach the operator's alert surface).
func TestHealthRelayTransitionsToAlerts(t *testing.T) {
	t.Parallel()

	f := newFakePap(t)

	relay, err := NewHealthRelay(Config{Endpoint: f.server.URL, APIKey: "k"})
	if err != nil {
		t.Fatalf("relay: %v", err)
	}
	t.Cleanup(func() { _ = relay.Close() })

	postSnapshot(t, relay.URL(), "pass", map[string]healthRelayCheck{
		"database": {Status: "pass"},
	})

	if calls := f.calls(); len(calls) != 0 {
		t.Fatalf("healthy first observation raised %d alert(s), want 0", len(calls))
	}

	postSnapshot(t, relay.URL(), "fail", map[string]healthRelayCheck{
		"database": {Status: "fail", Error: "store down"},
		"workers":  {Status: "pass"},
	})

	calls := waitForCalls(t, f, 1)
	if calls[0].Event != "alert.triggered" {
		t.Errorf("event = %q, want alert.triggered", calls[0].Event)
	}

	if calls[0].AggregateID != healthAggregate {
		t.Errorf("aggregateId = %q, want %q", calls[0].AggregateID, healthAggregate)
	}

	if calls[0].Authorization != "Bearer k" {
		t.Errorf("authorization = %q, want Bearer k", calls[0].Authorization)
	}

	var payload struct {
		Severity string `json:"severity"`
		Body     string `json:"body"`
	}
	if err := json.Unmarshal(calls[0].Payload, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}

	if payload.Severity != "critical" {
		t.Errorf("fail severity = %q, want critical", payload.Severity)
	}

	if !bytes.Contains([]byte(payload.Body), []byte("database: fail — store down")) {
		t.Errorf("body lacks the degraded check: %q", payload.Body)
	}

	if bytes.Contains([]byte(payload.Body), []byte("workers")) {
		t.Errorf("body lists a passing check: %q", payload.Body)
	}

	postSnapshot(t, relay.URL(), "pass", map[string]healthRelayCheck{
		"database": {Status: "pass"},
	})

	calls = waitForCalls(t, f, 2)
	if calls[1].Event != "alert.resolved" {
		t.Errorf("recovery event = %q, want alert.resolved", calls[1].Event)
	}

	if calls[1].AggregateID != healthAggregate {
		t.Errorf("resolve aggregateId = %q, want %q (correlation)", calls[1].AggregateID, healthAggregate)
	}

	// A repeated status (push-always dashboards) must stay silent.
	postSnapshot(t, relay.URL(), "pass", map[string]healthRelayCheck{
		"database": {Status: "pass"},
	})

	if calls := f.calls(); len(calls) != 2 {
		t.Errorf("repeated pass raised %d extra alert(s), want 0", len(calls)-2)
	}
}

// TestHealthRelayWarnIsWarning pins the severity ladder: warn maps to the
// warning tier (a degraded-but-alive queue is not a pager event).
func TestHealthRelayWarnIsWarning(t *testing.T) {
	t.Parallel()

	f := newFakePap(t)

	relay, err := NewHealthRelay(Config{Endpoint: f.server.URL, APIKey: "k"})
	if err != nil {
		t.Fatalf("relay: %v", err)
	}
	t.Cleanup(func() { _ = relay.Close() })

	postSnapshot(t, relay.URL(), "warn", map[string]healthRelayCheck{
		"workers": {Status: "warn", Error: "no worker heartbeat in 10m0s"},
	})

	calls := waitForCalls(t, f, 1)

	var payload struct {
		Severity string `json:"severity"`
	}
	if err := json.Unmarshal(calls[0].Payload, &payload); err != nil {
		t.Fatalf("payload: %v", err)
	}

	if payload.Severity != "warning" {
		t.Errorf("warn severity = %q, want warning", payload.Severity)
	}
}
