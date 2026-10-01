package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// cmdTasks is the list view `tq stats` deliberately is not: one row per
// task, filterable by project/status/type and a creation-time window, so
// reconstructing "what completed in the last 6h for project P" stops
// requiring raw sqlite reads against tasks.db (21:40 report §e7).
func cmdTasks(args []string) error {
	fs := flag.NewFlagSet("tasks", flag.ExitOnError)
	project := fs.String("project", "", "filter by project")
	status := fs.String("status", "", "filter by status (pending|running|completed|dead|cancelled)")
	band := fs.String("band", "", "filter by priority band (hot|machine|backlog)")
	taskType := fs.String("type", "", "filter by task type (e.g. agent, sh)")
	verifyContains := fs.String(
		"verify-contains",
		"",
		"filter by payload substring (e.g. a verify command; stale-verify audits without a tq show per task)",
	)
	since := fs.Duration("since", 0, "only tasks created within this window (e.g. 6h, 30m; 0 = all time)")
	parked := fs.Bool("parked", false, "only rate-limit-parked tasks (pending with a future not_before)")
	count := fs.Bool(
		"count",
		false,
		"print only the total number of matching tasks (ignores --limit; --json emits {\"count\": N})",
	)
	limit := fs.Int("limit", 50, "max tasks to list (0 = all)")
	asJSON := fs.Bool("json", false, "JSON output of the matching task list")
	jsonEnvelope := fs.Bool(
		"json-envelope",
		false,
		"wrap --json output as {tasks, total, truncated} so paging consumers can see the uncapped total",
	)

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	store := mustOpenDB(resolveDB(*db))
	defer store.Close()

	ctx := context.Background()

	filter := queue.Filter{Sort: "age-desc"}
	if *project != "" {
		filter.Project = project
	}

	if *status != "" {
		st := task.Status(*status)
		filter.Status = &st
	}

	if *band != "" {
		b := queue.Band(*band)
		switch b {
		case queue.BandHot:
			hotMin, hotMax := queue.HotMin, queue.HotMax
			filter.PriorityMin, filter.PriorityMax = &hotMin, &hotMax
		case queue.BandMachine:
			machineMin := queue.MachineMin
			filter.PriorityMin = &machineMin
		case queue.BandBacklog:
			backlogMax := queue.BacklogMax
			filter.PriorityMax = &backlogMax
		default:
			return fmt.Errorf("unknown band %q (want hot|machine|backlog)", *band)
		}
	}

	if *taskType != "" {
		filter.Type = taskType
	}

	if *verifyContains != "" {
		filter.PayloadContains = *verifyContains
	}

	// Creation window rides in the store (SQL pushdown, inclusive bound).
	if *since > 0 {
		cutoff := time.Now().Add(-*since)
		filter.Since = &cutoff
	}

	if *parked {
		filter.Parked = parked
	}

	if *limit > 0 {
		filter.Limit = *limit
	}

	if *count {
		n, err := store.CountTasks(ctx, filter)
		if err != nil {
			return err
		}

		if *asJSON {
			enc := json.NewEncoder(os.Stdout)

			return enc.Encode(struct {
				Count int `json:"count"`
			}{n})
		}

		fmt.Printf("%d matching task(s)\n", n)

		return nil
	}

	tasks, err := store.List(ctx, filter)
	if err != nil {
		return err
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")

		if *jsonEnvelope {
			total, err := store.CountTasks(ctx, func() queue.Filter {
				f := filter
				f.Limit = 0

				return f
			}())
			if err != nil {
				return err
			}

			return enc.Encode(struct {
				Tasks     []task.Task `json:"tasks"`
				Total     int         `json:"total"`
				Truncated bool        `json:"truncated"`
			}{tasks, total, total > len(tasks)})
		}

		return enc.Encode(tasks)
	}

	printTaskList(tasks, *limit, func() (int, error) {
		filter.Limit = 0

		return store.CountTasks(ctx, filter)
	})

	return nil
}

// printTaskList renders the list view: full IDs (they are the handle into
// `tq show`/`tq cancel`), status, project, age, and a last-error excerpt.
// total resolves the uncapped match count ONLY when the list hit the limit,
// so a truncated footer never reads as the whole set (dead-letter census
// missed a 09-20 death behind a silent 50-row cap).
func printTaskList(tasks []task.Task, limit int, total func() (int, error)) {
	printTaskListTo(os.Stdout, tasks, limit, total)
}

func printTaskListTo(w io.Writer, tasks []task.Task, limit int, total func() (int, error)) {
	if len(tasks) == 0 {
		fmt.Fprintln(w, "no matching tasks")

		return
	}

	fmt.Fprintf(
		w,
		"%-36s %-10s %-16s %-7s %5s %-9s  %s\n",
		"ID",
		"STATUS",
		"PROJECT",
		"TYPE",
		"ATT",
		"NOTBEFORE",
		"LAST ERROR",
	)

	now := time.Now()
	for _, t := range tasks {
		fmt.Fprintf(w, "%-36s %-10s %-16s %-7s %5d %-9s  %s\n",
			t.ID.String(), string(t.Status), t.Project, t.Type, t.Attempts,
			notBeforeCell(t, now),
			truncate(oneLine(t.LastError), 60),
		)
	}

	if limit > 0 && len(tasks) == limit {
		if n, err := total(); err == nil {
			fmt.Fprintf(
				w,
				"showing %d of %d matching task(s) (capped by --limit %d; --limit 0 lists all, --count prints just the total)\n",
				len(tasks),
				n,
				limit,
			)

			return
		}
	}

	fmt.Fprintf(w, "%d task(s)\n", len(tasks))
}

// notBeforeCell renders the park/backoff countdown for a future not_before
// (rate-limit requeues, deps) so a stalled-looking list explains itself;
// past or zero not_before renders "-".
func notBeforeCell(t task.Task, now time.Time) string {
	if t.NotBefore.IsZero() || !t.NotBefore.After(now) {
		return "-"
	}

	return truncate(t.NotBefore.Sub(now).Round(time.Second).String(), 9)
}

// oneLine flattens a multi-line error to its first line.
func oneLine(s string) string {
	if before, _, ok := strings.Cut(s, "\n"); ok {
		return before
	}

	return s
}
