package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
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
	taskType := fs.String("type", "", "filter by task type (e.g. agent, sh)")
	since := fs.Duration("since", 0, "only tasks created within this window (e.g. 6h, 30m; 0 = all time)")
	parked := fs.Bool("parked", false, "only rate-limit-parked tasks (pending with a future not_before)")
	limit := fs.Int("limit", 50, "max tasks to list (0 = all)")
	asJSON := fs.Bool("json", false, "JSON output of the matching task list")

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

	if *taskType != "" {
		filter.Type = taskType
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

	tasks, err := store.List(ctx, filter)
	if err != nil {
		return err
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")

		return enc.Encode(tasks)
	}

	printTaskList(tasks)

	return nil
}

// printTaskList renders the list view: full IDs (they are the handle into
// `tq show`/`tq cancel`), status, project, age, and a last-error excerpt.
func printTaskList(tasks []task.Task) {
	if len(tasks) == 0 {
		fmt.Println("no matching tasks")

		return
	}

	fmt.Printf("%-36s %-10s %-16s %-7s %5s  %s\n", "ID", "STATUS", "PROJECT", "TYPE", "ATT", "LAST ERROR")

	for _, t := range tasks {
		fmt.Printf("%-36s %-10s %-16s %-7s %5d  %s\n",
			t.ID.String(), string(t.Status), t.Project, t.Type, t.Attempts,
			truncate(oneLine(t.LastError), 60),
		)
	}

	fmt.Printf("%d task(s)\n", len(tasks))
}

// oneLine flattens a multi-line error to its first line.
func oneLine(s string) string {
	if before, _, ok := strings.Cut(s, "\n"); ok {
		return before
	}

	return s
}
