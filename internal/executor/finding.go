package executor

import "strings"

// Risk ratings, ordered: a higher rank absorbs a lower one. The vocabulary
// matches the review agent's normalized severity ratings (see
// normalizeSeverity) so a learned finding and a deterministic one speak the
// same scale.
const (
	RiskLow    = "low"
	RiskMedium = "medium"
	RiskHigh   = "high"
)

// Finding is one detector's output about a piece of model output: a severity
// rating plus opaque flag strings.
type Finding struct {
	Severity string
	Flags    []string
}

// MergeFindings combines a DETERMINISTIC finding (a rule table, a secret
// scan, a gate exit code) with a LEARNED finding (an LLM judge), under
// turnstone's merge rule: severity = max(deterministic, learned), flags =
// union. A learned detector may RAISE the severity or ADD flags; it can
// never LOWER what the deterministic gate found. This is the code form of
// SECURITY.md "Learned checks may only narrow": a judge is veto-only, and
// defeating the judge can never erase a tripwire.
//
// It is defined and tested now, ahead of any LLM risk tier, so the promise
// is machine-checked rather than prose: when a learned tier lands it must
// route through MergeFindings.
func MergeFindings(deterministic, learned Finding) Finding {
	severity := deterministic.Severity
	if riskRank(learned.Severity) > riskRank(deterministic.Severity) {
		severity = learned.Severity
	}

	return Finding{
		Severity: severity,
		Flags:    unionFlags(deterministic.Flags, learned.Flags),
	}
}

// riskRank orders severity ratings; unknown or empty ranks lowest, so an
// unrated learned finding can never outrank a real rating, and a real
// rating can never be lowered by an unrated one.
func riskRank(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case RiskHigh:
		return 3
	case RiskMedium:
		return 2
	case RiskLow:
		return 1
	default:
		return 0
	}
}

// unionFlags merges two flag lists preserving first-seen order and dropping
// empty/duplicate entries. Union, not intersection: a flag either detector
// raised survives.
func unionFlags(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))

	out := make([]string, 0, len(a)+len(b))

	for _, group := range [][]string{a, b} {
		for _, flag := range group {
			if flag == "" || seen[flag] {
				continue
			}

			seen[flag] = true

			out = append(out, flag)
		}
	}

	return out
}
