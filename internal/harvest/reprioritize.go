package harvest

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// RepriChange records one would-be or applied priority change from a
// Reprioritize pass.
type RepriChange struct {
	TaskID      task.ID
	ItemText    string
	OldPriority int
	NewPriority int
	Source      PrioritySource
}
// Reprioritize re-resolves the priorities of PENDING tasks from the
// CURRENT truth in the todo files (markers) and, with UseImportance, each
// repo's metadata importance — so an owner's marker edit or importance
// change reaches tasks already sitting in the queue (ADR-0015 §5).
//
// Rules: only PENDING tasks are touched; hot/machine tasks are protected
// (RepriMutable); same-value writes are no-ops (the store appends no
// fact); dryRun reports without writing. Per-repo scan failures are
// returned in failures, never as errors — a broken repo must not block
// the others.
func (h *Harvester) Reprioritize(ctx context.Context, dryRun bool) (changes []RepriChange, failures []string) {
	repos := h.cfg.Repos
	if len(repos) == 0 && h.cfg.ProjectsDir != "" {
		discovered, err := DiscoverReposFor(ctx, h.cfg.DiscoveryAddr, h.cfg.ProjectsDir, h.cfg.TodoFile, h.cfg.Log)
		if err != nil {
			return nil, []string{fmt.Sprintf("discover repos: %v", err)}
		}

		repos = discovered
	}

	sort.Strings(repos)

	for _, repo := range repos {
		changes, failures = h.repriRepo(ctx, repo, dryRun, changes, failures)
	}

	return changes, failures
}

// repriRepo re-resolves one repo's pending tasks; the append-and-return
// shape keeps the caller's slices flowing without a shared pointer.
func (h *Harvester) repriRepo(
	ctx context.Context,
	repo string,
	dryRun bool,
	changes []RepriChange,
	failures []string,
) ([]RepriChange, []string) {
	items, err := ParseRepo(repo, h.cfg.TodoFile)
	if err != nil {
		return changes, append(failures, fmt.Sprintf("%s: %v", filepath.Base(repo), err))
	}

	repoName := filepath.Base(repo)

	importance := DefaultImportance

	if h.cfg.UseImportance {
		importance, err = ReadImportance(repo)
		if err != nil {
			return changes, append(failures, fmt.Sprintf("%s: metadata: %v", repoName, err))
		}
	}

	pending := h.pendingByKey(ctx, repoName)

	for _, item := range items {
		tk, ok := pending[item.Key]
		if !ok || !RepriMutable(tk.Priority) {
			continue
		}

		// Cached AI verdicts join the ladder here too (ADR-0015 §3: marker
		// > AI > keyword/importance) — the same feed the enqueue path uses,
		// so a scored item re-resolves consistently everywhere.
		var aiScore *int

		if score, ok, err := h.q.PriorityScore(ctx, item.Key); err == nil && ok {
			clamped := queue.ClampBacklog(score.Score)
			aiScore = &clamped
		}

		priority, source := ResolvePriority(ResolveInput{
			Text:              item.Text,
			MarkerLevel:       item.MarkerLevel,
			HotPriority:       h.cfg.SameSessionPriority,
			FlatPriority:      h.cfg.Priority,
			Importance:        importance,
			ImportanceEnabled: h.cfg.UseImportance,
			AIScore:           aiScore,
		})

		if priority == tk.Priority {
			continue // value-idempotent
		}

		if !dryRun {
			if err := h.q.UpdatePendingPriority(ctx, tk.ID, priority, string(source), repriReason(item, source)); err != nil {
				failures = append(failures, fmt.Sprintf("%s: update %s: %v", repoName, tk.ID, err))

				continue
			}
		}

		changes = append(changes, RepriChange{
			TaskID:      tk.ID,
			ItemText:    item.Text,
			OldPriority: tk.Priority,
			NewPriority: priority,
			Source:      source,
		})
	}

	return changes, failures
}

// pendingByKey lists the repo's tasks of the configured type and indexes
// the PENDING ones by payload dedup key.
func (h *Harvester) pendingByKey(ctx context.Context, repoName string) map[string]task.Task {
	project := repoName
	taskType := h.cfg.Type

	tasks, err := h.q.List(ctx, queue.Filter{Project: &project, Type: &taskType})
	if err != nil {
		return nil
	}

	pending := make(map[string]task.Task, len(tasks))

	for _, t := range tasks {
		if t.Status != task.Pending {
			continue
		}

		if key := payloadDedup(t); key != "" {
			pending[key] = t
		}
	}

	return pending
}

// repriReason builds the fact reason for one applied change.
func repriReason(item Item, source PrioritySource) string {
	if item.MarkerLevel > 0 {
		return fmt.Sprintf("marker P%d", item.MarkerLevel)
	}

	return "re-resolved from " + string(source)
}
