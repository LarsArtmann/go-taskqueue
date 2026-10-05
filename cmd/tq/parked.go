package main

import (
	"encoding/json"
	"time"

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
