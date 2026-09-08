package harvest

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// pruneReasonPrefix heads every cancellation reason a prune pass writes, so
// a task trail always says where its withdrawal came from.
const pruneReasonPrefix = "harvest --prune-stale: TODO_LIST item is now [x]: "

// PruneResult summarizes one stale-task prune pass over all repos.
type PruneResult struct {
	Repos int
	// Cancelled lists pending tasks withdrawn because their TODO_LIST item
	// is now ticked — the zombies a pool relaunch would otherwise execute.
	Cancelled []PrunedTask
	// Running lists ticked-item tasks currently executing: a cooperative
	// stop is an operator decision (tq cancel), not a sweep default.
	Running []PrunedTask
	// Dead lists ticked-item tasks already dead-lettered: terminal, listed
	// because the dedup key still suppresses re-enqueue if the item is
	// ever un-ticked with unchanged text.
	Dead []PrunedTask
	// ScanFailures lists repos that could not be read; the prune keeps
	// going past them (same contract as Audit).
	ScanFailures []ScanFailure
}

// PrunedTask pairs a withdrawn/declined task with the item text that doomed it.
type PrunedTask struct {
	Item   Item
	TaskID task.ID
}

// PruneStale cancels pending queue tasks whose TODO_LIST item is now `[x]`
// (dedup-key match), so a pool relaunch never inherits zombies: harvesting
// while no pool runs creates pending tasks, and ticking an item later never
// withdraws them (21:40 report §d1/§e1 — six stale tasks sat 5–24 h). Only
// PENDING tasks are cancelled; running ones are reported (a cooperative stop
// mid-execution is an operator decision), dead ones are terminal. DryRun
// reports without cancelling.
func (h *Harvester) PruneStale(ctx context.Context) (PruneResult, error) {
	var res PruneResult

	repos := h.cfg.Repos
	if len(repos) == 0 {
		if h.cfg.ProjectsDir == "" {
			return res, ErrNoRepos
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
		if err := h.pruneRepo(ctx, repo, &res); err != nil {
			res.ScanFailures = append(res.ScanFailures, ScanFailure{Repo: repo, Reason: err.Error()})
		}
	}

	return res, nil
}

func (h *Harvester) pruneRepo(ctx context.Context, repo string, res *PruneResult) error {
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
		if !it.Done {
			continue
		}

		t, tracked := byDedup[it.Key]
		if !tracked {
			continue
		}

		switch t.Status {
		case task.Pending:
			if h.cfg.DryRun {
				res.Cancelled = append(res.Cancelled, PrunedTask{Item: it, TaskID: t.ID})

				continue
			}

			reason := pruneReasonPrefix + truncateItem(it.Text)
			if err := h.q.Cancel(ctx, t.ID, reason); err != nil {
				res.ScanFailures = append(res.ScanFailures, ScanFailure{
					Repo: repo, Reason: fmt.Sprintf("cancel %s failed: %s", t.ID, err),
				})

				continue
			}

			res.Cancelled = append(res.Cancelled, PrunedTask{Item: it, TaskID: t.ID})
		case task.Running:
			res.Running = append(res.Running, PrunedTask{Item: it, TaskID: t.ID})
		case task.Dead:
			res.Dead = append(res.Dead, PrunedTask{Item: it, TaskID: t.ID})
		}
	}

	return nil
}

// truncateItem keeps a cancellation reason readable: the item text, cut to
// its first line and 120 chars.
func truncateItem(text string) string {
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = text[:i]
	}

	if len(text) > 120 {
		text = text[:120] + "…"
	}

	return text
}
