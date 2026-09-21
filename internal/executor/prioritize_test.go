package executor

import (
	"encoding/json/v2"
	"slices"
	"strings"
	"testing"
)

func TestParsePrioritizeResultContract(t *testing.T) {
	t.Parallel()

	items := []PrioritizeItem{
		{Key: "todo:aaa", Text: "Fix the flaky gate"},
		{Key: "todo:bbb", Text: "Water the plants"},
	}

	valid := "thoughts...\n\nTQ_RESULT: {\"verdicts\":[" +
		"{\"item_key\":\"todo:aaa\",\"score\":85,\"effort_minutes\":30,\"confidence\":80,\"reasoning\":\"unblocks CI\"}," +
		"{\"item_key\":\"todo:bbb\",\"score\":15,\"effort_minutes\":15,\"confidence\":90}" +
		"]}\n"

	res, err := ParsePrioritizeResult(valid, items)
	if err != nil {
		t.Fatalf("valid batch: %v", err)
	}

	if len(res.Verdicts) != 2 || res.Verdicts[0].ItemKey != "todo:aaa" || res.Verdicts[0].Score != 85 {
		t.Fatalf("verdicts = %+v", res.Verdicts)
	}

	cases := []struct {
		name string
		out  string
	}{
		{"missing item", "TQ_RESULT: {\"verdicts\":[{\"item_key\":\"todo:aaa\",\"score\":50}]}"},
		{
			"duplicate item",
			"TQ_RESULT: {\"verdicts\":[{\"item_key\":\"todo:aaa\",\"score\":50},{\"item_key\":\"todo:aaa\",\"score\":60},{\"item_key\":\"todo:bbb\",\"score\":1}]}",
		},
		{
			"unknown item",
			"TQ_RESULT: {\"verdicts\":[{\"item_key\":\"todo:zzz\",\"score\":50},{\"item_key\":\"todo:aaa\",\"score\":1},{\"item_key\":\"todo:bbb\",\"score\":1}]}",
		},
		{
			"score out of range",
			"TQ_RESULT: {\"verdicts\":[{\"item_key\":\"todo:aaa\",\"score\":101},{\"item_key\":\"todo:bbb\",\"score\":1}]}",
		},
		{"no result line", "I forgot the contract line"},
		{"unparsable json", "TQ_RESULT: {verdicts: not json}"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if _, err := ParsePrioritizeResult(tc.out, items); err == nil {
				t.Fatal("expected contract failure, got nil")
			}
		})
	}
}

// TestPrioritizePromptContract pins the prompt's mechanical contract:
// every item key appears, the TQ_RESULT shape is spelled out, and the
// read-only rule is stated.
func TestPrioritizePromptContract(t *testing.T) {
	t.Parallel()

	p := PrioritizePayload{
		Repo:     "demo",
		RepoName: "demo",
		Items: []PrioritizeItem{
			{Key: "todo:aaa", Text: "Fix the gate", Heading: "CI"},
			{Key: "todo:bbb", Text: "Docs pass"},
		},
	}

	prompt := prioritizePrompt(p)

	for _, want := range []string{
		"todo:aaa",
		"todo:bbb",
		"Fix the gate",
		"tq verdict '{",
		"TQ_RESULT_FILE",
		"READ-ONLY",
		`"verdicts"`,
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q", want)
		}
	}
}

// TestPrioritizeResultUsageKeysMatchAgentResult pins the shared
// session-usage json keys (09-52 f2): the budget projection and `tq show`
// parse the SAME keys off every completion-fact result type, so a rename
// in either struct — AgentResult's or PrioritizeResult's, either
// direction of drift — must fail here.
func TestPrioritizeResultUsageKeysMatchAgentResult(t *testing.T) {
	t.Parallel()

	usageKeys := []string{
		"session_cost_usd",
		"session_prompt_tokens",
		"session_completion_tokens",
		"session_message_count",
	}

	cases := []struct {
		name  string
		res   any
		extra []string
	}{
		{
			name: "AgentResult",
			res: AgentResult{
				SessionCostUSD:          1,
				SessionPromptTokens:     2,
				SessionCompletionTokens: 3,
				SessionMessageCount:     4,
			},
		},
		{
			name: "PrioritizeResult",
			res: PrioritizeResult{
				Verdicts:                []PrioritizeVerdict{{ItemKey: "todo:aaa", Score: 50}},
				SessionCostUSD:          1,
				SessionPromptTokens:     2,
				SessionCompletionTokens: 3,
				SessionMessageCount:     4,
			},
			extra: []string{"verdicts"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			raw, err := json.Marshal(tc.res)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			keys := make([]string, 0, len(got))
			for k := range got {
				keys = append(keys, k)
			}
			slices.Sort(keys)

			want := append(slices.Clone(usageKeys), tc.extra...)
			slices.Sort(want)
			if !slices.Equal(keys, want) {
				t.Fatalf("%s marshals %v, want exactly the shared usage keys %v (plus %v) — a rename here breaks the budget projection and tq show", tc.name, keys, usageKeys, tc.extra)
			}
		})
	}
}
