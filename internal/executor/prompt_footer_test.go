package executor

import (
	"strings"
	"testing"
)

// TestPromptsFooterLastContract pins the footer-placement contract
// (TODO row 938; 2026-10-14 05-16 report §d1/§f5): every queue-authored
// prompt that teaches the commit footer must teach footer-AFTER-attribution
// — the harness template appends the Crush/Assisted-by block and a footer
// stranded above it is invisible to git trailers (the re-minted 05-47
// defect class). Phrase matching is whitespace-collapsed so line wrapping
// inside the templates cannot false-fail the pin.
func TestPromptsFooterLastContract(t *testing.T) {
	t.Parallel()

	prompts := map[string]string{
		"closeout": DefaultCloseoutPrompt,
		"status":   statusPrompt(statusPayloadT()),
		"dlqfix": dlqFixPrompt(DLQFixPayload{
			DeadTask: "000001a0deadtaskid00000000",
			DeadType: "agent",
			Work:     "fix the frobnicator",
			Failure:  FailureEvidence{Stage: "verify", ExitCode: 2},
			Attempts: 1,
		}),
	}

	for name, prompt := range prompts {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			collapsed := strings.Join(strings.Fields(prompt), " ")
			for _, want := range []string{
				"the FINAL line of the message",
				"put the footer AFTER that block",
				"invisible to git trailers",
			} {
				if !strings.Contains(collapsed, want) {
					t.Errorf("%s prompt lost the footer-placement phrase %q — template-copied commits will re-mint the footer-above-attribution defect", name, want)
				}
			}
		})
	}
}
