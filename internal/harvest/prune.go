package harvest

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// pruneReasonHead heads every prune-stale cancellation reason so the
// journal has ONE voice for stale-item withdrawals — greppable across the
// ticked and absent rules (01:43 report f2).
const pruneReasonHead = "harvest --prune-stale: TODO_LIST item "

// pruneReasonPrefix heads ticked-item cancellations: the item is still in
// the file and now `[x]`.
const pruneReasonPrefix = pruneReasonHead + "is now [x]: "

// pruneAbsentReasonPrefix heads cancellations of tasks whose item text is
// gone from the file entirely (completed-and-deleted per the docs
// convention, or reworded — which armed a new key and a new task).
const pruneAbsentReasonPrefix = pruneReasonHead + "no longer present (key %s): item withdrawn or reworded"

// PruneWhy records which stale rule withdrew (or reported) a task.
type PruneWhy string

const (
	// PruneTicked: the item is still in the file and now `[x]`.
	PruneTicked PruneWhy = "ticked"
	// PruneAbsent: the item text is gone from the file entirely.
	PruneAbsent PruneWhy = "absent"
)

// PruneResult summarizes one stale-task prune pass over all repos.
type PruneResult struct {
	Repos int
	// Cancelled lists pending tasks withdrawn because their TODO_LIST item
	// is now ticked — the zombies a pool relaunch would otherwise execute —
	// or because the item text is gone from the file entirely.
	Cancelled []PrunedTask
	// Running lists stale-item tasks currently executing: a cooperative
	// stop is an operator decision (tq cancel), not a sweep default.
	Running []PrunedTask
	// Dead lists stale-item tasks already dead-lettered: terminal, listed
	// because the dedup key still suppresses re-enqueue if the item is
	// ever un-ticked with unchanged text.
	Dead []PrunedTask
	// ScanFailures lists repos that could not be read; the prune keeps
	// going past them (same contract as Audit).
	ScanFailures []ScanFailure
}

// PrunedTask pairs a withdrawn/declined task with the item text that doomed
// it and the rule that applied. For absent items the text is gone by
// definition; Item carries the repo and the dedup key instead.
type PrunedTask struct {
	Item   Item
	TaskID task.ID
	Why    PruneWhy
}

// PruneStale cancels pending queue tasks whose TODO_LIST item is now `[x]`
// (dedup-key match), or whose item text is gone from the file entirely, so
// a pool relaunch never inherits zombies: harvesting while no pool runs
// creates pending tasks, and ticking — or deleting — an item later never
// withdraws them (21:40 report §d1/§e1 — six stale tasks sat 5–24 h). Only
// PENDING tasks are cancelled; running ones are reported (a cooperative stop
// mid-execution is an operator decision), dead ones are terminal. Only tasks
// carrying a harvest dedup key in their payload are ever touched — external
// work is invisible to the sweep. DryRun reports without cancelling.
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
	items, byDedup, err := h.projectTaskIndex(ctx, repo)
	if err != nil {
		return err
	}

	// Present keys: every item still in the file, open or done. A task
	// whose (catchup-stripped) key is absent from this set points at text
	// that no longer exists — completed-and-deleted per the docs
	// convention, or reworded (which armed a new key and a new task).
	presentKeys := make(map[string]struct{}, len(items))
	for _, it := range items {
		presentKeys[it.Key] = struct{}{}
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
				res.Cancelled = append(res.Cancelled, PrunedTask{Item: it, TaskID: t.ID, Why: PruneTicked})

				continue
			}

			reason := pruneReasonPrefix + truncateItem(it.Text)
			if err := h.q.Cancel(ctx, t.ID, reason); err != nil {
				res.ScanFailures = append(res.ScanFailures, ScanFailure{
					Repo: repo, Reason: fmt.Sprintf("cancel %s failed: %s", t.ID, err),
				})

				continue
			}

			res.Cancelled = append(res.Cancelled, PrunedTask{Item: it, TaskID: t.ID, Why: PruneTicked})
		case task.Running:
			res.Running = append(res.Running, PrunedTask{Item: it, TaskID: t.ID, Why: PruneTicked})
		case task.Dead:
			res.Dead = append(res.Dead, PrunedTask{Item: it, TaskID: t.ID, Why: PruneTicked})
		}
	}

	repoName := filepath.Base(repo)

	// Absent-item pass, sorted for deterministic output.
	keys := make([]string, 0, len(byDedup))
	for key := range byDedup {
		keys = append(keys, key)
	}

	sort.Strings(keys)

	for _, key := range keys {
		h.pruneAbsentTask(ctx, repo, repoName, key, byDedup[key], presentKeys, res)
	}

	return nil
}

// pruneAbsentTask applies the absent rule to one harvested task: when its
// (catchup-stripped) item key no longer matches ANY present item, the text
// it was minted from is gone — completed-and-deleted per the docs
// convention, or reworded (which armed a new key and a new task) — so the
// pending task is a zombie and is withdrawn. The Item synthesized for
// reporting carries the key (the text is gone by definition); only
// harvest-provenance tasks reach here — byDedup is built from payload dedup
// keys, so external work is invisible to the sweep.
func (h *Harvester) pruneAbsentTask(
	ctx context.Context, repo, repoName, key string,
	t task.Task, presentKeys map[string]struct{}, res *PruneResult,
) {
	itemKey := strings.TrimPrefix(key, CatchupKeyPrefix)
	if _, present := presentKeys[itemKey]; present {
		return
	}

	it := Item{Repo: repo, RepoName: repoName, Key: itemKey}

	switch t.Status {
	case task.Pending:
		if h.cfg.DryRun {
			res.Cancelled = append(res.Cancelled, PrunedTask{Item: it, TaskID: t.ID, Why: PruneAbsent})

			return
		}

		reason := fmt.Sprintf(pruneAbsentReasonPrefix, itemKey)
		if err := h.q.Cancel(ctx, t.ID, reason); err != nil {
			res.ScanFailures = append(res.ScanFailures, ScanFailure{
				Repo: repo, Reason: fmt.Sprintf("cancel %s failed: %s", t.ID, err),
			})

			return
		}

		res.Cancelled = append(res.Cancelled, PrunedTask{Item: it, TaskID: t.ID, Why: PruneAbsent})
	case task.Running:
		res.Running = append(res.Running, PrunedTask{Item: it, TaskID: t.ID, Why: PruneAbsent})
	case task.Dead:
		res.Dead = append(res.Dead, PrunedTask{Item: it, TaskID: t.ID, Why: PruneAbsent})
	}
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
