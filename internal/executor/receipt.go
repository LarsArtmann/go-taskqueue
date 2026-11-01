package executor

import (
	"fmt"
	"slices"
	"strings"
)

// VerifyReceipt is the structured SUMMARY of one verify-gate run: what ran,
// at which HEAD, how it exited, and the two line tallies a closeout report
// would otherwise hand-derive with `grep -c '^ok:'`.
//
// Sources: OKF v0.2 SPEC §10.2 (a run yields a receipt with a declared field
// set) and §10.5 (a deterministic, LLM-free attester grades it, and a false
// verdict means the consumer REFUSES the value rather than decorating it) —
// see docs/research/2026-11-01_okf-lessons.md lesson 10.
//
// It is an EXTENSION of evidence shapes tq already had, never a parallel
// format: AgentResult.Verify and FailureEvidence.Verify both carry it, and
// the full gate output keeps living in the writeVerifyEvidence sidecar. This
// is the bounded summary — the part that must survive log rotation, because
// the counts are exactly what a report quotes.
//
// Measured when this landed (2026-11-01): 34 living status reports carry 59
// hand-derived "N ok / M fail" receipts, and 26 files say the words
// `grep -c '^ok:'` outright. Each of those was a number a paid session counted
// in context; ParseVerifyCounts makes the same number an observation.
type VerifyReceipt struct {
	// Command is the command that ACTUALLY ran, after verifyFor's
	// resolution (.tq-verify > payload > auto-detect). AgentResult used to
	// keep only the output tail, so "did the gate that ran equal the gate
	// we sanctioned" was unanswerable after the fact.
	Command string `json:"command,omitempty"`
	// RC is the gate's process exit code (0 on success, the verify error's
	// exit on failure).
	RC int `json:"rc"`
	// Head is the git HEAD rev the gate ran at — captured at gate time,
	// deliberately BEFORE any post-gate derivation, so a completion that
	// claims a different rev is a visible mismatch instead of a trust
	// statement. Absent when the repo has no HEAD (plain directory runs).
	Head string `json:"head,omitempty"`
	// OK and Fail are the `^ok:` and `^FAIL` line counts in the gate's
	// combined output — the same two anchors the closeout convention
	// hand-greps (AGENTS.md §Conventions, "Gate receipts"). ParseVerifyCounts
	// is the one implementation of that counting rule.
	OK   int `json:"ok"`
	Fail int `json:"fail"`
}

// IsZero reports whether the receipt carries no observation at all, which is
// the "no gate ran" case (unknown stack, no explicit command).
func (r *VerifyReceipt) IsZero() bool {
	return r == nil || (r.Command == "" && r.RC == 0 && r.Head == "" && r.OK == 0 && r.Fail == 0)
}

// ParseVerifyCounts tallies a captured gate output the way the closeout
// convention does: a line beginning with `ok:` counts as one passing check, a
// line beginning with `FAIL` counts as one failing check. The anchors are
// UNINDENTED on purpose — `grep -c '^ok:'` is the rule the reports already
// follow, so the emitted number and a reader's hand-grep must agree; a laxer
// anchor here would make the receipt disagree with the evidence it summarises.
func ParseVerifyCounts(output string) (okCount, failCount int) {
	for line := range strings.SplitSeq(output, "\n") {
		switch {
		case strings.HasPrefix(line, "ok:"):
			okCount++
		case strings.HasPrefix(line, "FAIL"):
			failCount++
		}
	}

	return okCount, failCount
}

// Attestation refusal classes (VerifyVerdict.Refusal). Closed vocabulary: a
// verdict either passes or names exactly one failed comparison.
const (
	// AttestationNoReceipt means no gate receipt was recorded at all — a
	// legacy task or a run with nothing to verify. The value is UNVERIFIED,
	// never invalid (a required receipt would break every legacy task).
	AttestationNoReceipt = "no-receipt"
	// AttestationHeadMismatch means the gate ran at one HEAD and the
	// completion claims another: something moved the tree between the gate
	// and the derivation, so the passing gate proves a different tree.
	AttestationHeadMismatch = "head-mismatch"
	// AttestationCommandMismatch means the executed command is not the
	// command the repo or the payload pinned. This is the `--reresolve-verify`
	// hole made observable: re-resolving drops the enqueue-time pin for a
	// value read from agent-editable repo content.
	AttestationCommandMismatch = "command-mismatch"
	// AttestationCountMismatch means the recorded ok/fail tallies disagree
	// with a re-count of the captured gate output — the miscount class that
	// burned the 02-11 and 04-55 closeouts.
	AttestationCountMismatch = "count-mismatch"
)

// Attestation comparisons a verdict may have actually performed. Reported so a
// skipped comparison is visible instead of reading as a pass (an unrun check
// that looks green is the silent-green failure this whole file exists to
// prevent).
const (
	AttestationCheckReceipt = "receipt"
	AttestationCheckHead    = "head"
	AttestationCheckCommand = "command"
	AttestationCheckCounts  = "counts"
)

// VerifyVerdict is the deterministic attester's ruling on a receipt.
type VerifyVerdict struct {
	// OK is true only when every comparison that COULD run passed.
	OK bool `json:"ok"`
	// Refusal names the failed comparison (one of the Attestation*
	// classes); empty when OK.
	Refusal string `json:"refusal,omitempty"`
	// Detail is the human-readable mismatch, e.g. "gate ran at 1a2b3c4,
	// completion claims 9f8e7d6".
	Detail string `json:"detail,omitempty"`
	// Checked lists the comparisons actually performed (AttestationCheck*
	// values), so a caller can tell "passed" from "not verified".
	Checked []string `json:"checked,omitempty"`
}

// String renders the verdict as the compact suffix the evidence surfaces
// print: "ok" / "ok (head,command,counts)" / "command-mismatch: …".
func (v VerifyVerdict) String() string {
	if !v.OK {
		if v.Refusal == "" {
			return "unattested"
		}

		if v.Detail == "" {
			return v.Refusal
		}

		return v.Refusal + ": " + v.Detail
	}

	if len(v.Checked) == 0 {
		return "ok"
	}

	return "ok (" + strings.Join(v.Checked, ",") + ")"
}

// Attest grades this run's receipt with the declarations the run itself saw.
// Returns nil when no gate ran (no receipt): absence is absence, and a stored
// "no-receipt" verdict would dress it up as a judgement instead of a fact.
//
// The verdict is stamped at WRITE time, by the process that held the gate
// output and the declarations, and stored beside the receipt. That is
// deliberate: a reader months later cannot reconstruct the repo's .tq-verify or
// the enqueue-time pin, so re-grading from the stored receipt alone would
// silently skip comparisons and read green. Re-grading IS still possible for a
// stored sidecar log (AttestVerify is pure) — that is the audit path, not the
// rendering path.
func (o VerifyOutcome) Attest(claimedHead string) *VerifyVerdict {
	if o.Receipt == nil {
		return nil
	}

	verdict := AttestVerify(AttestationInput{
		Receipt:       o.Receipt,
		PayloadVerify: o.PayloadVerify,
		RepoVerify:    o.RepoVerify,
		ClaimedHead:   claimedHead,
		GateOutput:    string(o.Output),
	})

	return &verdict
}

// AttestationInput is everything the attester grades: the receipt the run
// produced plus the declarations it must agree with. Every input is a plain
// value, so the attester stays a pure function with no git access, no store
// access and no model in the loop.
type AttestationInput struct {
	// Receipt is the gate receipt under judgement.
	Receipt *VerifyReceipt
	// PayloadVerify is the enqueue-time pin (AgentPayload.Verify).
	PayloadVerify string
	// RepoVerify is the repo's .tq-verify content at gate time.
	RepoVerify string
	// ClaimedHead is the rev the completion claims the work landed at.
	ClaimedHead string
	// GateOutput is the captured gate output, when the caller still has it
	// (the sidecar). Empty SKIPS the count comparison rather than passing
	// it — the skipped check is reported in VerifyVerdict.Checked.
	GateOutput string
}

// AttestVerify grades a verify receipt against what the run declared and what
// the completion claims. Three comparisons:
//
//  1. HEAD — the gate ran at the claimed rev (abbreviated shas accepted).
//  2. COMMAND — the executed command is one of the pinned declarations. When
//     neither the repo nor the payload pinned anything, auto-detection is the
//     sanctioned resolution and the comparison is skipped.
//  3. COUNTS — the recorded ok/fail tallies re-derive from the captured
//     output. Skipped when the caller has no output.
//
// A refusal does not merely annotate: consumers refuse to render the receipt
// as verified (OKF §10.5). It never fails a task by itself — a verdict is a
// judgement about a VALUE, and the failure path has its own evidence.
func AttestVerify(in AttestationInput) VerifyVerdict {
	if in.Receipt == nil {
		return VerifyVerdict{
			Refusal: AttestationNoReceipt,
			Detail:  "no verify receipt recorded (unverified, not invalid)",
		}
	}

	checked := []string{AttestationCheckReceipt}

	if in.ClaimedHead != "" && in.Receipt.Head != "" {
		checked = append(checked, AttestationCheckHead)

		if !sameRev(in.Receipt.Head, in.ClaimedHead) {
			return refusal(AttestationHeadMismatch, checked, fmt.Sprintf(
				"gate ran at %s, completion claims %s",
				shortRev(in.Receipt.Head), shortRev(in.ClaimedHead),
			))
		}
	}

	if declared := declaredVerifyCommands(in.RepoVerify, in.PayloadVerify); len(declared) > 0 {
		checked = append(checked, AttestationCheckCommand)

		if !slices.Contains(declared, normalizeSpace(in.Receipt.Command)) {
			return refusal(AttestationCommandMismatch, checked, fmt.Sprintf(
				"executed %q, pinned %q",
				normalizeSpace(in.Receipt.Command), strings.Join(declared, `" or "`),
			))
		}
	}

	if in.GateOutput != "" {
		checked = append(checked, AttestationCheckCounts)

		okCount, failCount := ParseVerifyCounts(in.GateOutput)
		if okCount != in.Receipt.OK || failCount != in.Receipt.Fail {
			return refusal(AttestationCountMismatch, checked, fmt.Sprintf(
				"receipt says %d ok / %d fail, output says %d ok / %d fail",
				in.Receipt.OK, in.Receipt.Fail, okCount, failCount,
			))
		}
	}

	return VerifyVerdict{OK: true, Checked: checked}
}

func refusal(class string, checked []string, detail string) VerifyVerdict {
	return VerifyVerdict{Refusal: class, Detail: detail, Checked: checked}
}

// declaredVerifyCommands returns the whitespace-normalised, de-duplicated
// declarations a receipt's command may match. The repo's .tq-verify outranks
// the payload in verifyFor, but BOTH are sanctioned here: which one won is a
// precedence question, not an integrity one.
func declaredVerifyCommands(repoVerify, payloadVerify string) []string {
	var declared []string

	for _, candidate := range []string{repoVerify, payloadVerify} {
		normalized := normalizeSpace(candidate)
		if normalized == "" || slices.Contains(declared, normalized) {
			continue
		}

		declared = append(declared, normalized)
	}

	return declared
}

// normalizeSpace collapses runs of whitespace so a command that was
// re-rendered with different spacing still compares equal. Nothing else is
// normalised: rewriting a command's tokens would be the attester inventing
// agreement, which is the judgement creep this comparison exists to avoid.
func normalizeSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

// shortRev trims a rev to its 7-character abbreviation for messages.
func shortRev(rev string) string {
	if len(rev) <= 7 {
		return rev
	}

	return rev[:7]
}

// sameRev reports whether two revs name the same commit, accepting the
// abbreviated form on either side (git's own minimum abbreviation is 7
// characters; anything shorter cannot be compared safely).
func sameRev(a, b string) bool {
	if a == "" || b == "" {
		return false
	}

	if a == b {
		return true
	}

	longest, shortest := a, b
	if len(shortest) > len(longest) {
		longest, shortest = shortest, longest
	}

	return len(shortest) >= 7 && strings.HasPrefix(longest, shortest)
}
