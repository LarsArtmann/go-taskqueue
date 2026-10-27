package main

import (
	"encoding/json"
	"testing"
)

// TestStatsPayloadGoldenJSON pins statsPayload's wire keys exactly — the
// surfaces the budget goldens don't cover (by_status, by_project,
// consumer_lag, claim_anomalies envelope). `tq stats --json` is the
// script/dashboard contract; cmd/tq is replace-free (ADR-0016/ADR-0017),
// so this exact-shape pin is the drift guard: renaming a key or flipping
// an omitempty here is a silent wire break for every JSON consumer, and
// this test makes it loud.
func TestStatsPayloadGoldenJSON(t *testing.T) {
	blob, err := json.Marshal(statsPayload{
		ByStatus: map[string]int{"done": 12, "running": 2},
		ByProject: map[string]map[string]int{
			"alpha": {"done": 7, "running": 2},
			"beta":  {"done": 5},
		},
		Budget:                budgetView{SpentToday: 4},
		Lag:                   []consumerLagEntry{{Consumer: "sweeper", Seq: 41, Lag: 3}},
		JournalHead:           44,
		Parked:                1,
		OpenSessions:          2,
		SessionsOpened:        5,
		SessionsClosed:        3,
		ClaimAnomalies:        []claimSuspect{{ID: "task-1", Claims: 25}},
		ClaimAnomalyThreshold: 20,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	want := `{"by_status":{"done":12,"running":2},` +
		`"by_project":{"alpha":{"done":7,"running":2},"beta":{"done":5}},` +
		`"budget":{"spent_today":4},` +
		`"consumer_lag":[{"consumer":"sweeper","seq":41,"lag":3}],` +
		`"journal_head":44,` +
		`"parked":1,` +
		`"open_sessions":2,` +
		`"sessions_opened":5,` +
		`"sessions_closed":3,` +
		`"claim_anomalies":[{"id":"task-1","claims":25}],` +
		`"claim_anomaly_threshold":20}`
	if got := string(blob); got != want {
		t.Fatalf("statsPayload wire shape drifted:\n got  %s\n want %s", got, want)
	}
}

// TestStatsPayloadBareShape pins the omitempty half of the same contract: a
// bare store emits only the three always-present keys, so consumers can
// branch on absence instead of zero values.
func TestStatsPayloadBareShape(t *testing.T) {
	blob, err := json.Marshal(statsPayload{})
	if err != nil {
		t.Fatalf("marshal bare: %v", err)
	}

	want := `{"by_status":{},"budget":{"spent_today":0},"journal_head":0}`
	if got := string(blob); got != want {
		t.Fatalf("bare statsPayload = %s, want %s (every optional surface must stay keyless)", got, want)
	}
}
