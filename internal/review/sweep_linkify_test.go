package review

import (
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/executor"
)

// TestFixPromptLinkifiesFindingCites pins that fix prompts minted from
// review findings carry /task/<reviewed>#finding-N anchor links for any
// `#finding-N` citation the reviewer wrote, so the fixer lands on the
// exact finding row of the reviewed task's detail page (design-pass §f27).
func TestFixPromptLinkifiesFindingCites(t *testing.T) {
	t.Parallel()

	prompt := fixPrompt(
		executor.ReviewPayload{
			Repo:         "demo",
			ReviewedTask: "0001ab",
			Item:         "write the thing",
		},
		executor.ReviewFinding{
			Severity: "high",
			Title:    "scroll offset wrong (#finding-2)",
			Detail:   "The header overlaps the row cited in #finding-2; see also #finding-12.",
		},
	)

	for _, want := range []string{
		"[#finding-2](/task/0001ab#finding-2)",
		"[#finding-12](/task/0001ab#finding-12)",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("fix prompt missing linkified cite %s:\n%s", want, prompt)
		}
	}

	if got := strings.Count(prompt, "[#finding-2](/task/0001ab#finding-2)"); got != 2 {
		t.Fatalf("cite #finding-2 must linkify once in title and once in detail, got %d:\n%s", got, prompt)
	}
}
