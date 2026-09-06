package harvest

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// CatchupKeyPrefix marks a catch-up task's dedup key. A catch-up task asks an
// agent to tick a checkbox whose work is already done; the prefix keeps it
// distinct from the original item's key, so it can never mask the drift it
// exists to close.
const CatchupKeyPrefix = "catchup:"

// DefaultCatchupPrompt is the contract for catch-up tasks: close the loop in
// the file, change nothing else.
const DefaultCatchupPrompt = `Repository: {{REPO_ABS}}

An earlier agent already finished this TODO_LIST.md item and its work was
verified (tests pass, tree committed), but the checkbox was never ticked:

Section "{{HEADING}}": "{{ITEM}}"

Contract:
1. Read AGENTS.md first and follow it.
2. Verify the item is genuinely done (do NOT redo the work). If it is NOT
   done after all, stop and leave the file unchanged.
3. Tick exactly this one checkbox in TODO_LIST.md (or remove the line if the
   file's conventions prefer that). Change nothing else.
4. Commit with a clear message. Never push.`

// DriftKind names which side of the file-vs-queue contract broke.
type DriftKind string

const (
	// DriftStaleOpen: the queue says the work completed, the checkbox is
	// still open — the loop was never closed in the file (or was reverted).
	DriftStaleOpen DriftKind = "stale-open"
	// DriftStaleDone: the checkbox is ticked but the task never completed
	// (pending, running or dead) — someone did the work by hand or ticked
	// prematurely; the queue task is now pointless. Report-only: cancelling
	// human-visible work is an operator decision (`tq cancel`).
	DriftStaleDone DriftKind = "stale-done"
)

// Drift is one file-vs-queue disagreement found by Audit.
type Drift struct {
	Kind       DriftKind
	Item       Item
	TaskID     task.ID
	TaskStatus task.Status
}

// DriftResult summarizes one audit pass over all repos.
type DriftResult struct {
	Repos int
	// StaleOpen lists completed-work-open-checkbox drift; with DryRun unset,
	// each entry here also got a catch-up task enqueued (see Enqueued).
	StaleOpen []Drift
	// StaleDone lists ticked-checkbox-unfinished-task drift (report-only).
	StaleDone []Drift
	// Enqueued lists the catch-up tasks actually created this pass.
	Enqueued []Enqueued
}

// Audit compares every repo's checkbox states against the queue's terminal
// task states and repairs one direction of drift: when a task COMPLETED but
// its checkbox is still open, a catch-up task (dedup-keyed CatchupKeyPrefix
// + item key) is enqueued to close the loop in the file. The store's unique
// dedup index makes that enqueue-once, so a repeating audit never piles up
// duplicate catch-ups even if the first one also fails to tick the box.
// The opposite direction (ticked checkbox, unfinished task) is reported
// only. DryRun reports everything without enqueuing.
func (h *Harvester) Audit(ctx context.Context) (DriftResult, error) {
	var res DriftResult

	repos := h.cfg.Repos
	if len(repos) == 0 {
		if h.cfg.ProjectsDir == "" {
			return res, fmt.Errorf("harvest: no repos and no projects dir configured")
		}
		var err error
		repos, err = DiscoverRepos(h.cfg.ProjectsDir, h.cfg.TodoFile)
		if err != nil {
			return res, fmt.Errorf("harvest: discover repos: %w", err)
		}
	}
	sort.Strings(repos)

	for _, repo := range repos {
		res.Repos++
		if err := h.auditRepo(ctx, repo, &res); err != nil {
			// A repo that cannot be read is a harvest-scan problem; the
			// audit keeps going and Run reports it the same way.
			continue
		}
	}
	return res, nil
}

func (h *Harvester) auditRepo(ctx context.Context, repo string, res *DriftResult) error {
	items, err := ParseRepoAll(repo, h.cfg.TodoFile)
	if err != nil {
		return err
	}
	repoName := filepath.Base(repo)
	tasks, err := h.q.List(ctx, queue.Filter{Project: &repoName, Type: &h.cfg.Type})
	if err != nil {
		return err
	}
	byDedup := make(map[string]task.Task, len(tasks))
	for _, t := range tasks {
		if key := payloadDedup(t); key != "" {
			if _, seen := byDedup[key]; !seen {
				byDedup[key] = t
			}
		}
	}

	for _, it := range items {
		t, tracked := byDedup[it.Key]
		switch {
		case !it.Done && tracked && t.Status == task.Completed:
			d := Drift{Kind: DriftStaleOpen, Item: it, TaskID: t.ID, TaskStatus: t.Status}
			res.StaleOpen = append(res.StaleOpen, d)
			catchupKey := CatchupKeyPrefix + it.Key
			if h.cfg.DryRun {
				continue
			}
			if _, armed := byDedup[catchupKey]; armed {
				continue // a previous audit already armed this repair
			}
			id, err := h.enqueueCatchup(ctx, it, catchupKey)
			if err != nil {
				continue
			}
			res.Enqueued = append(res.Enqueued, Enqueued{Item: it, TaskID: id, Fresh: true})
		case it.Done && tracked && t.Status != task.Completed:
			res.StaleDone = append(res.StaleDone, Drift{Kind: DriftStaleDone, Item: it, TaskID: t.ID, TaskStatus: t.Status})
		}
	}
	return nil
}

// enqueueCatchup arms the loop-closing agent task for one drifted item.
func (h *Harvester) enqueueCatchup(ctx context.Context, it Item, catchupKey string) (task.ID, error) {
	payload, err := h.buildPayload(it, DefaultCatchupPrompt, catchupKey)
	if err != nil {
		return "", err
	}
	maxAttempts := h.cfg.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 2
	}
	t, err := h.q.Enqueue(ctx, task.New{
		Project:     it.RepoName,
		Type:        h.cfg.Type,
		Payload:     payload,
		Priority:    h.cfg.Priority,
		MaxAttempts: maxAttempts,
		DedupKey:    catchupKey,
	})
	if err != nil {
		return "", err
	}
	return t.ID, nil
}
