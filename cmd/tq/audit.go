package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/larsartmann/go-taskqueue/internal/harvest"
	"github.com/larsartmann/go-taskqueue/internal/queue"
)

// cmdAudit compares every repo's TODO_LIST.md checkboxes with the queue's
// terminal task states and repairs the drift the harvest loop cannot see:
// work a completed agent never ticked off gets a catch-up task (dedup-keyed,
// so auditing again never re-arms it).
func cmdAudit(args []string) error {
	fs := flag.NewFlagSet("audit", flag.ExitOnError)
	journalFlag := fs.Bool(
		"journal",
		false,
		"journal-drift audit: rebuild task state from the fact journal and diff against the tasks table (advisory)",
	)
	projectsDir := fs.String("projects-dir", "", "directory of repos to audit (each with a TODO_LIST.md)")
	repos := fs.String("repos", "", "comma-separated explicit repo paths (overrides --projects-dir)")
	todoFile := fs.String("todo-file", harvest.DefaultTodoFile, "backlog file name inside each repo")
	taskType := fs.String(
		"type",
		harvest.DefaultType,
		"task type whose tasks are audited (catch-ups are enqueued as it)",
	)
	maxAttempts := fs.Int("max-attempts", 0, "attempt budget for catch-up tasks (0 = audit default 2)")
	dryRun := fs.Bool("dry-run", false, "report drift without enqueueing catch-ups")
	asJSON := fs.Bool("json", false, "JSON output of the audit result")

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *projectsDir == "" && *repos == "" {
		*projectsDir = defaultProjectsDir()
	}

	if *journalFlag {
		s := mustOpenDB(resolveDB(*db))
		defer s.Close()
		return cmdJournalAudit(context.Background(), s, *asJSON)
	}

	if err := checkProjectsDir(*projectsDir); err != nil {
		return err
	}

	cfg := harvest.Config{
		ProjectsDir: *projectsDir,
		TodoFile:    *todoFile,
		Type:        *taskType,
		MaxAttempts: *maxAttempts,
		DryRun:      *dryRun,
	}

	if *repos != "" {
		cfg.Repos = expandRepoSpecs(*projectsDir, splitRepos(*repos))
	}

	s := mustOpenDB(resolveDB(*db))
	defer s.Close()

	res, err := harvest.New(queue.New(s), cfg).Audit(context.Background())
	if err != nil {
		return err
	}

	if *asJSON {
		if err := printDriftJSON(res); err != nil {
			return fmt.Errorf("encode audit result: %w", err)
		}

		return nil
	}

	printDriftReport(res, *dryRun)

	return nil
}

// printDriftJSON writes the audit result as indented JSON, mirroring the
// tq harvest --json output shape.
func printDriftJSON(res harvest.DriftResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	return enc.Encode(res)
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

	for _, f := range res.ScanFailures {
		fmt.Printf("ERROR  %-24s scan failed: %s\n", filepath.Base(f.Repo), f.Reason)
	}

	for _, d := range res.StaleDone {
		fmt.Printf("DRIFT  %-24s stale-done  task %s is %s, checkbox ticked: %s\n",
			d.Item.RepoName, d.TaskID, d.TaskStatus, truncate(d.Item.Text, 80))
	}

	if len(res.StaleOpen) == 0 && len(res.StaleDone) == 0 && len(res.ScanFailures) == 0 {
		fmt.Println("(no drift)")
	}

	fmt.Printf("audit: %d repos, %d stale-open (%d catch-ups enqueued), %d stale-done, %d scan failures\n",
		res.Repos, len(res.StaleOpen), len(res.Enqueued), len(res.StaleDone), len(res.ScanFailures))
}
