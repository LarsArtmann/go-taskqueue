package main

import (
	"encoding/json"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// factTailLimit bounds the journal read per CLI frame/command to the most
// recent facts: requeue-class derivations (`tq top`'s budget chip,
// `tq tasks --parked-class`) and durations come from recent facts, and a
// bounded tail keeps every refresh O(1) as the journal grows.
const factTailLimit = 5000

// latestRequeueEvidence walks facts in seq order and returns, per task
// id, the RequeueEvidence of its most recent task.requeued fact
// (last-wins): ONE single-pass decider shared by the parked-class filter,
// the provider tag, and the budget chip, so no reader walks the fact tail
// twice (2026-10-27 07-28 closeout §f24). Legacy or malformed details
// normalize the same way every other reader of the evidence does
// (journalaudit, webui, readmodel): unparsable details to the zero
// evidence, a missing class to RequeueClassUnknown.
func latestRequeueEvidence(facts []journal.Fact) map[string]queue.RequeueEvidence {
	latest := make(map[string]queue.RequeueEvidence)

	for _, fact := range facts {
		if fact.Type != journal.Requeued {
			continue
		}

		var evidence queue.RequeueEvidence
		if err := json.Unmarshal(fact.Detail, &evidence); err != nil {
			latest[fact.TaskID] = queue.RequeueEvidence{}

			continue
		}

		if evidence.Class == "" {
			evidence.Class = queue.RequeueClassUnknown
		}

		latest[fact.TaskID] = evidence
	}

	return latest
}

// failureEvidenceSummary composes the evidence= suffix from PUBLISHED
// executor primitives — this module is replace-free (ADR-0017) and
// executor.SummaryWithError is newer than the pinned tag: Summary plus,
// when no structured Class exists, the verify-gate alias the recorded
// error text matches. The alias strings and matcher stay owned by
// executor (constants + MatchesEvidenceStage); nothing is re-typed here
// (the cmd/tq-view-vs-executor-type boundary class, dupe ledger row 11).
func failureEvidenceSummary(e executor.FailureEvidence, lastError string) string {
	stage := e.Summary()
	if e.Class != "" {
		return stage
	}

	for _, alias := range []string{
		string(executor.VerifyGateEnvironmental), string(executor.VerifyGateDead), string(executor.VerifyGateSlow),
	} {
		if executor.MatchesEvidenceStage(e, lastError, alias) {
			if stage != "" {
				stage += " "
			}

			return stage + "alias=" + alias
		}
	}

	return stage
}

// latestFailureEvidenceSummary walks facts in seq order and returns, per
// task id, the SummaryWithError of its most recent task.failed fact
// (last-wins; the fact's error text rides along so alias-only deaths still
// render). One self-describing row shape across `tq dlq` and
// `tq tasks --parked` (TODO row 1414); unparsable details skip.
func latestFailureEvidenceSummary(facts []journal.Fact) map[string]string {
	latest := make(map[string]journal.Fact)

	for _, fact := range facts {
		if fact.Type == journal.Failed {
			latest[fact.TaskID] = fact
		}
	}

	out := make(map[string]string, len(latest))

	for id, fact := range latest {
		var evidence executor.FailureEvidence
		if json.Unmarshal(fact.Detail, &evidence) != nil {
			continue
		}

		if s := failureEvidenceSummary(evidence, fact.Error); s != "" {
			out[id] = s
		}
	}

	return out
}

// parkedByClass filters tasks down to the ones currently inside a park of
// the given requeue class: pending, latest requeue evidence of the class,
// and the not_before window still open (a due park is claimable work, not
// parked). The class join runs CLI-side because it is fact evidence, not
// a stored column the queue filter could push down.
func parkedByClass(tasks []task.Task, latest map[string]queue.RequeueEvidence, class string, now time.Time) []task.Task {
	out := make([]task.Task, 0, len(tasks))

	for _, t := range tasks {
		if t.Status != task.Pending || latest[t.ID.String()].Class != class {
			continue
		}

		if t.NotBefore.After(now) {
			out = append(out, t)
		}
	}

	return out
}
