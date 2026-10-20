package readmodel

import (
	"context"
	"errors"
	"fmt"
	"strings"

	metaengine "github.com/larsartmann/go-cqrs-lite/metaengine/v4"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// Stats reads the status counts and the per-project-per-status counts
// from the Delta counter collections (upstream metaengine COOKBOOK §
// Status Count Dashboard) — O(1) counter reads, zero rows loaded. The
// counters fold the same fact stream the tasks map folds, with the
// from-side resolved by the apply-time prev read, so they stay exact
// across the from-status-ambiguous transitions (lease reclaims, dead
// rescues, dismissals).
//
// filter narrows both matrices exactly like a Tasks read would: a Status
// filter leaves only that status's counts, a Project filter only that
// project's. A project-scoped byStatus derives from the project|status
// counters — the global status counter cannot narrow by project.
func (m *Model) Stats(
	ctx context.Context,
	filter TaskFilter,
) (map[string]int, map[string]map[string]int, error) {
	statusRaw, err := metaengine.ExecuteTyped[statusCountsInput, map[string]int64](
		ctx, m.store, statusCountsInput{},
	)
	if err != nil && !errors.Is(err, metaengine.ErrNotFound) {
		return nil, nil, fmt.Errorf("readmodel: status counters: %w", err)
	}

	projectRaw, err := metaengine.ExecuteTyped[projectStatusCountsInput, map[string]int64](
		ctx, m.store, projectStatusCountsInput{},
	)
	if err != nil && !errors.Is(err, metaengine.ErrNotFound) {
		return nil, nil, fmt.Errorf("readmodel: project counters: %w", err)
	}

	byProject := projectMatrix(projectRaw, filter)
	byStatus := statusCounts(statusRaw, projectRaw, filter)

	return byStatus, byProject, nil
}

// projectMatrix splits the project|status counter keys into the
// per-project matrix under the filter.
func projectMatrix(projectRaw map[string]int64, filter TaskFilter) map[string]map[string]int {
	byProject := map[string]map[string]int{}

	for key, count := range projectRaw {
		project, status, ok := strings.Cut(key, projectStatusKeySep)
		if !ok || count == 0 || !matchesFilter(filter, project, status) {
			continue
		}

		if byProject[project] == nil {
			byProject[project] = map[string]int{}
		}

		byProject[project][status] = int(count)
	}

	return byProject
}

// statusCounts narrows the global status counter; a project filter
// derives it from the project|status counters instead — the global
// counter cannot narrow by project.
func statusCounts(statusRaw, projectRaw map[string]int64, filter TaskFilter) map[string]int {
	byStatus := map[string]int{}

	if filter.Project != nil {
		for key, count := range projectRaw {
			project, status, ok := strings.Cut(key, projectStatusKeySep)
			if !ok || count == 0 || project != *filter.Project {
				continue
			}

			if filter.Status != nil && status != *filter.Status {
				continue
			}

			byStatus[status] = int(count)
		}

		return byStatus
	}

	for status, count := range statusRaw {
		if count == 0 || filter.Status != nil && status != *filter.Status {
			continue
		}

		byStatus[status] = int(count)
	}

	return byStatus
}

// matchesFilter reports whether one project|status count survives the
// filter.
func matchesFilter(filter TaskFilter, project, status string) bool {
	if filter.Project != nil && project != *filter.Project {
		return false
	}

	if filter.Status != nil && status != *filter.Status {
		return false
	}

	return true
}

// StatusCounts counts the ledger rows per status — the Delta counter
// read behind the stats surfaces (dashboard chips, /api/stats, tq stats).
func (m *Model) StatusCounts(ctx context.Context) (map[string]int, error) {
	counts, _, err := m.Stats(ctx, TaskFilter{})

	return counts, err
}

// StatusCounts is the one seam both servers read status counts through:
// from the model when one is supplied, from the queue store otherwise.
// The dashboard server (internal/webui) and the machine API
// (internal/httpapi) shared this exact branch pair before it landed here.
func StatusCounts(ctx context.Context, model *Model, store queue.Store) (map[task.Status]int, error) {
	if model != nil {
		counts, err := model.StatusCounts(ctx)
		if err != nil {
			return nil, err
		}

		return task.StatusCountsMap(counts), nil
	}

	return store.StatusCounts(ctx)
}

// ProjectCounts counts the ledger rows per project per status — the same
// matrix the store-side ProjectCounts GROUP BY serves (queue.Store
// vocabulary parity for the projection).
func (m *Model) ProjectCounts(ctx context.Context) (map[string]map[string]int, error) {
	_, counts, err := m.Stats(ctx, TaskFilter{})

	return counts, err
}
