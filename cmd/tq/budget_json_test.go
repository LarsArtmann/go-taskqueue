package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestBudgetUsageViewGoldenJSON pins budgetUsageView's wire keys exactly:
// runs/cost_usd/prompt_tokens/completion_tokens/messages. This struct is the
// cross-module contract budget.SessionUsage feeds onto `tq stats --json` —
// cmd/tq is replace-free (ADR-0016/ADR-0017) and cannot import
// internal/budget, so this pin is the drift guard: renaming a key here is a
// wire break every consumer of the stats payload feels, and this test makes
// it loud.
func TestBudgetUsageViewGoldenJSON(t *testing.T) {
	blob, err := json.Marshal(&budgetUsageView{
		Runs:             7,
		CostUSD:          1.25,
		PromptTokens:     1200,
		CompletionTokens: 340,
		Messages:         21,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	got := string(blob)

	want := `{"runs":7,"cost_usd":1.25,"prompt_tokens":1200,"completion_tokens":340,"messages":21}`
	if got != want {
		t.Fatalf("budgetUsageView wire shape drifted:\n got  %s\n want %s", got, want)
	}

	for _, key := range []string{"runs", "cost_usd", "prompt_tokens", "completion_tokens", "messages"} {
		if !strings.Contains(got, `"`+key+`":`) {
			t.Fatalf("wire key %q missing from %s", key, got)
		}
	}
}

// TestBudgetViewSessionUsageEnvelope pins the parent envelope: session_usage
// is omitempty (bare stores stay keyless) and the sibling keys spent_today
// /cap keep their own contract.
func TestBudgetViewSessionUsageEnvelope(t *testing.T) {
	blob, err := json.Marshal(budgetView{SpentToday: 4})
	if err != nil {
		t.Fatalf("marshal bare: %v", err)
	}

	if got, want := string(blob), `{"spent_today":4}`; got != want {
		t.Fatalf("bare budgetView = %s, want %s (session_usage must be omitempty)", got, want)
	}

	blob, err = json.Marshal(budgetView{
		SpentToday:   4,
		Cap:          100,
		SessionUsage: &budgetUsageView{Runs: 2, CostUSD: 0.5, PromptTokens: 10, CompletionTokens: 20, Messages: 3},
	})
	if err != nil {
		t.Fatalf("marshal full: %v", err)
	}

	want := `{"spent_today":4,"cap":100,"session_usage":{"runs":2,"cost_usd":0.5,"prompt_tokens":10,"completion_tokens":20,"messages":3}}`
	if got := string(blob); got != want {
		t.Fatalf("full budgetView wire shape drifted:\n got  %s\n want %s", got, want)
	}
}
