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

// latestRequeueClass walks facts in seq order and returns, per task id,
// the class of its most recent task.requeued fact (last-wins). Legacy or
// malformed details normalize to RequeueClassUnknown, matching every
// other reader of the evidence (journalaudit, webui, readmodel).
func latestRequeueClass(facts []journal.Fact) map[string]string {
	latest := make(map[string]string)

	for _, fact := range facts {
		if fact.Type != journal.Requeued {
			continue
		}

		var evidence queue.RequeueEvidence
		if err := json.Unmarshal(fact.Detail, &evidence); err != nil || evidence.Class == "" {
			latest[fact.TaskID] = queue.RequeueClassUnknown

			continue
		}

		latest[fact.TaskID] = evidence.Class
	}

	return latest
}

// latestRequeueProvider walks facts in seq order and returns, per task
// id, the provider tag of its most recent task.requeued fact (last-wins):
// WHICH provider exhausted, so the parked surface answers "which provider
// is limited?" in one glance (04-03 §f8). Legacy or malformed details
// normalize to "" (no tag), matching the skip-don't-fail decode policy.
func latestRequeueProvider(facts []journal.Fact) map[string]string {
	latest := make(map[string]string)

	for _, fact := range facts {
		if fact.Type != journal.Requeued {
			continue
		}

		var evidence queue.RequeueEvidence
		if err := json.Unmarshal(fact.Detail, &evidence); err != nil {
			latest[fact.TaskID] = ""

			continue
		}

		latest[fact.TaskID] = evidence.Provider
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
func parkedByClass(tasks []task.Task, latest map[string]string, class string, now time.Time) []task.Task {
	out := make([]task.Task, 0, len(tasks))

	for _, t := range tasks {
		if t.Status != task.Pending || latest[t.ID.String()] != class {
			continue
		}

		if t.NotBefore.After(now) {
			out = append(out, t)
		}
	}

	return out
}
