package readmodel

import (
	"context"
	"fmt"

	metaengine "github.com/larsartmann/go-cqrs-lite/metaengine/v4"
)

// statuses is the closed status vocabulary the per-project rollup walks:
// one pushed-down GROUP BY per status assembles the project×status matrix
// without loading a single row.
var statuses = []string{ //nolint:gochecknoglobals // closed vocabulary mirror of the task.Status enum
	statusPending,
	statusRunning,
	statusCompleted,
	statusDead,
	statusCancelled,
}

// Stats reads the status counts and the per-project-per-status counts as
// SQL GROUP BY pushdowns over the planned tasks table — O(statuses +
// statuses×groups) engine work, zero rows loaded. The counts derive from
// the same folded rows every Tasks read serves, so they are exact by
// construction (no separate counter state to drift on reclaims, rescues
// or dismissals, which have no statically knowable from-status).
//
// filter narrows both matrices exactly like a Tasks read would: a Status
// filter leaves only that status's counts, a Project filter only that
// project's.
func (m *Model) Stats(
	ctx context.Context,
	filter TaskFilter,
) (map[string]int, map[string]map[string]int, error) {
	reader := metaengine.NewReader[TaskRow](m.store, tasksCollection)

	opts := []metaengine.ScanOption{}
	if filter.Project != nil {
		opts = append(opts, metaengine.WithFilter("project", metaengine.FilterEq, *filter.Project))
	}

	if filter.Status != nil {
		opts = append(opts, metaengine.WithFilter("status", metaengine.FilterEq, *filter.Status))
	}

	byStatus, err := groupedInts(ctx, reader, "status", opts)
	if err != nil {
		return nil, nil, fmt.Errorf("readmodel: status counts: %w", err)
	}

	byProject := map[string]map[string]int{}

	for _, st := range statuses {
		stOpts := append(opts, metaengine.WithFilter("status", metaengine.FilterEq, st))

		groups, err := groupedInts(ctx, reader, "project", stOpts)
		if err != nil {
			return nil, nil, fmt.Errorf("readmodel: project counts %s: %w", st, err)
		}

		for p, n := range groups {
			if byProject[p] == nil {
				byProject[p] = map[string]int{}
			}

			byProject[p][st] = n
		}
	}

	return byStatus, byProject, nil
}

// StatusCounts counts the ledger rows per status — the GROUP BY pushdown
// behind the stats surfaces (dashboard chips, /api/stats, tq stats).
func (m *Model) StatusCounts(ctx context.Context) (map[string]int, error) {
	counts, _, err := m.Stats(ctx, TaskFilter{})

	return counts, err
}

// ProjectCounts counts the ledger rows per project per status — the same
// matrix the store-side ProjectCounts GROUP BY serves (queue.Store
// vocabulary parity for the projection).
func (m *Model) ProjectCounts(ctx context.Context) (map[string]map[string]int, error) {
	_, counts, err := m.Stats(ctx, TaskFilter{})

	return counts, err
}

// groupedInts runs one GROUP BY COUNT pushdown and converts the engine's
// float64 values to the int counters the stats surfaces render.
func groupedInts(
	ctx context.Context,
	reader *metaengine.TypedReader[TaskRow],
	groupBy string,
	opts []metaengine.ScanOption,
) (map[string]int, error) {
	groups, err := reader.GroupedCount(ctx, groupBy, opts...)
	if err != nil {
		return nil, err //nolint:wrapcheck // caller adds the count name
	}

	counts := make(map[string]int, len(groups))
	for k, n := range groups {
		counts[k] = int(n)
	}

	return counts, nil
}
