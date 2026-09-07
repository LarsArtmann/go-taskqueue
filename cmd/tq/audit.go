package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/larsartmann/go-taskqueue/internal/harvest"
	"github.com/larsartmann/go-taskqueue/internal/queue"
)

// cmdAudit compares every repo's TODO_LIST.md checkboxes with the queue's
// terminal task states and repairs the drift the harvest loop cannot see:
// work a completed agent never ticked off gets a catch-up task (dedup-keyed,
// so auditing again never re-arms it).
func cmdAudit(args []string) error {
	fs := flag.NewFlagSet("audit", flag.ExitOnError)
	projectsDir := fs.String("projects-dir", "", "directory of repos to audit (each with a TODO_LIST.md)")
	repos := fs.String("repos", "", "comma-separated explicit repo paths (overrides --projects-dir)")
	dryRun := fs.Bool("dry-run", false, "report drift without enqueueing catch-ups")

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *projectsDir == "" && *repos == "" {
		*projectsDir = defaultProjectsDir()
	}

	if err := checkProjectsDir(*projectsDir); err != nil {
		return err
	}

	cfg := harvest.Config{ProjectsDir: *projectsDir, DryRun: *dryRun}

	if *repos != "" {
		cfg.Repos = splitRepos(*repos)
	}

	s := mustOpenDB(resolveDB(*db))
	defer s.Close()

	res, err := harvest.New(queue.New(s), cfg).Audit(context.Background())
	if err != nil {
		return err
	}

	printDriftReport(res, *dryRun)

	return nil
}

// printDriftReport writes the drift report produced by harvest.Audit: stale
// open checkboxes (with catch-up state), stale done ones, and the summary line.
func printDriftReport(res harvest.DriftResult, dryRun bool) {
	for _, d := range res.StaleOpen {
		line := fmt.Sprintf("DRIFT  %-24s stale-open  task %s completed, checkbox open: %s",
			d.Item.RepoName, d.TaskID, truncate(d.Item.Text, 80))

		switch {
		case dryRun:
			line += "  [catch-up: dry-run, not enqueued]"
		default:
			line += "  [catch-up armed]"
		}

		fmt.Println(line)
	}

	for _, d := range res.StaleDone {
		fmt.Printf("DRIFT  %-24s stale-done  task %s is %s, checkbox ticked: %s\n",
			d.Item.RepoName, d.TaskID, d.TaskStatus, truncate(d.Item.Text, 80))
	}

	if len(res.StaleOpen) == 0 && len(res.StaleDone) == 0 {
		fmt.Println("(no drift)")
	}

	fmt.Printf("audit: %d repos, %d stale-open (%d catch-ups enqueued), %d stale-done\n",
		res.Repos, len(res.StaleOpen), len(res.Enqueued), len(res.StaleDone))
}
