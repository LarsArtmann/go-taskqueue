// Package dlqbackoff derives poisoned-repo DLQ-backoff deadlines from
// the journal: the one fold behind `tq pool-health`'s poisoned WARN +
// dlqBackoffUntil JSON field, the `tq dlq` row cross-links, and the
// webui dashboard's backoff panel — three surfaces, one journal truth.
// A repo whose latest still-standing dead letter is fresher than the
// backoff window and that has no completion on record is poisoned:
// work on it waits until lastDead+window (the harvest pool's Pacing
// verdict made queryable).
package dlqbackoff

import (
	"context"
	"fmt"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// PoisonedAdvice is the operator-advice tail shared by every poisoned
// line (`tq pool-health` WARN, `tq dlq` rows, the webui panel): one
// prose contract, so cross-surface greps and pins never drift.
const PoisonedAdvice = "fix the repo or rescue dead tasks"

// drainPageSize bounds each Facts read when draining the journal
// (mirrors the page size the cmd/tq journal drain rides).
const drainPageSize = 1000

// Deadlines folds the journal into each repo's poisoned-repo backoff
// deadline: the latest still-standing dead letter extended by window,
// kept only while still in the future at now. Dismissal (a Cancelled
// fact on the dead-lettered task) takes a dead letter out of the
// derivation and any completion ever heals the repo — the same mirrors
// the harvester's table view applies. projectByID maps a task id to its
// repo (project); tasks without a repo are skipped. Nil when window is
// 0. The expiry boundary is the strict After here — pinned by
// TestDeadlinesBoundaryInstant.
func Deadlines(facts []journal.Fact, projectByID map[task.ID]string, now time.Time, window time.Duration) map[string]time.Time {
	if window <= 0 {
		return nil
	}

	deadAt, dismissed, everCompleted := scanOutcomes(facts, projectByID)
	lastLiveDead := latestStandingDead(deadAt, dismissed, projectByID)

	return stillActive(lastLiveDead, everCompleted, now, window)
}

// scanOutcomes splits the journal into the three inputs the fold reads:
// each task's latest dead-letter time, the dismissed tasks, and the
// repos with any completion ever.
func scanOutcomes(facts []journal.Fact, projectByID map[task.ID]string) (deadAt map[task.ID]time.Time, dismissed map[task.ID]bool, everCompleted map[string]bool) {
	deadAt = make(map[task.ID]time.Time)
	dismissed = make(map[task.ID]bool)
	everCompleted = make(map[string]bool)

	for _, fact := range facts {
		id := task.ID(fact.TaskID)

		switch fact.Type {
		case journal.Completed:
			if project := projectByID[id]; project != "" {
				everCompleted[project] = true
			}
		case journal.DeadLettered:
			deadAt[id] = fact.Time
		case journal.Cancelled:
			dismissed[id] = true
		}
	}

	return deadAt, dismissed, everCompleted
}

// latestStandingDead reduces the dead letters to the latest
// still-standing one per repo; dismissed dead letters are out of the
// derivation and repoless tasks are skipped.
func latestStandingDead(deadAt map[task.ID]time.Time, dismissed map[task.ID]bool, projectByID map[task.ID]string) map[string]time.Time {
	lastLiveDead := make(map[string]time.Time)

	for id, when := range deadAt {
		if dismissed[id] {
			continue
		}

		project := projectByID[id]
		if project == "" {
			continue
		}

		if best, ok := lastLiveDead[project]; !ok || when.After(best) {
			lastLiveDead[project] = when
		}
	}

	return lastLiveDead
}

// stillActive extends each repo's latest dead letter by the window and
// keeps only the deadlines still in the future at now; any completion
// ever heals the repo first.
func stillActive(lastLiveDead map[string]time.Time, everCompleted map[string]bool, now time.Time, window time.Duration) map[string]time.Time {
	deadlines := make(map[string]time.Time, len(lastLiveDead))

	for repo, lastDead := range lastLiveDead {
		if everCompleted[repo] {
			continue
		}

		if until := lastDead.Add(window); until.After(now) {
			deadlines[repo] = until
		}
	}

	return deadlines
}

// DeadlinesForStore is the store-backed seam under Deadlines for
// callers that hold a store and need only the deadline map (the webui
// snapshot loader, `tq dlq` rows): drains the journal, joins the task
// identities, folds. Nil without any journal I/O when window is 0.
func DeadlinesForStore( //nolint:nilnil // the off switch is a valid (nil, nil) state, not an invalid value
	ctx context.Context,
	store queue.Store,
	now time.Time,
	window time.Duration,
) (map[string]time.Time, error) {
	if window <= 0 {
		return nil, nil
	}

	var facts []journal.Fact

	after := int64(0)

	for {
		batch, err := store.Facts(ctx, after, drainPageSize)
		if err != nil {
			return nil, fmt.Errorf("read facts after %d: %w", after, err)
		}

		facts = append(facts, batch...)
		if len(batch) < drainPageSize {
			break
		}

		after = batch[len(batch)-1].Seq
	}

	tasks, err := store.List(ctx, queue.Filter{})
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	projectByID := make(map[task.ID]string, len(tasks))
	for _, t := range tasks {
		if t.Project != "" {
			projectByID[t.ID] = t.Project
		}
	}

	return Deadlines(facts, projectByID, now, window), nil
}
