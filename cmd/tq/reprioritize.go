package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/larsartmann/go-taskqueue/internal/harvest"
	"github.com/larsartmann/go-taskqueue/internal/queue"
)

// cmdReprioritize implements `tq reprioritize`: re-resolve PENDING task
// priorities from current TODO_LIST markers and (with --priority-from
// importance) each repo's metadata importance (ADR-0015 §5). Marker edits
// and importance changes reach tasks already sitting in the queue; hot
// and machine bands are protected; same-value resolutions change nothing.
func cmdReprioritize(args []string) error {
	fs := flag.NewFlagSet("reprioritize", flag.ContinueOnError)

	projectsDir := fs.String(
		"projects-dir",
		defaultProjectsDir(),
		"dir containing repos with TODO_LIST.md (default $TQ_PROJECTS_DIR or ~/projects)",
	)
	repos := fs.String("repos", "", "comma-separated repo dirs (overrides --projects-dir)")
	repoSubset := fs.String("repo-subset", "", "comma-separated repo names to narrow --projects-dir discovery")
	todoFile := fs.String("todo-file", harvest.DefaultTodoFile, "backlog file name inside each repo")
	taskType := fs.String("type", harvest.DefaultType, "task type the harvested items carry")
	priority := fs.Int("priority", 0, "flat fallback priority for unmarked items")
	priorityFrom := fs.String(
		"priority-from",
		"",
		`also re-resolve from each repo's .config/metadata.yaml importance; "importance" enables (markers always apply)`,
	)
	sameSessionPriority := fs.Int(
		"same-session-priority",
		0,
		"hot priority for items whose text references /tmp paths; 0 disables",
	)
	dryRun := fs.Bool("dry-run", false, "report would-be changes, change nothing")
	asJSON := fs.Bool("json", false, "JSON output of the reprioritize result")
	db := dbFlag(fs)

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *priorityFrom != "" && *priorityFrom != priorityFromImportance {
		return fmt.Errorf(`--priority-from: want %q or empty, got %q`, priorityFromImportance, *priorityFrom)
	}

	cfg := harvest.Config{
		ProjectsDir:         *projectsDir,
		TodoFile:            *todoFile,
		Type:                *taskType,
		Priority:            *priority,
		SameSessionPriority: *sameSessionPriority,
		UseImportance:       *priorityFrom == priorityFromImportance,
		PromptTemplate:      harvest.DefaultPromptTemplate,
	}

	if err := resolveHarvestRepos(&cfg, *projectsDir, *repos, *repoSubset); err != nil {
		return err
	}

	if *dryRun {
		cfg.DryRun = true
	}

	store := mustOpenDB(resolveDB(*db))
	defer store.Close()

	changes, failures := harvest.New(queue.New(store), cfg).Reprioritize(context.Background(), *dryRun)

	// Unblock pass (ADR-0015): PENDING tasks whose deps ALL completed gain
	// the queue-level bump - same sweep slot, same protections.
	unblocked, err := queue.New(store).BumpUnblocked(context.Background(), *dryRun)
	if err != nil {
		failures = append(failures, "unblock bump: "+err.Error())
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")

		return enc.Encode(struct {
			Changes  []harvest.RepriChange `json:"changes"`
			Unblocks []queue.UnblockChange `json:"unblocks"`
			Failures []string              `json:"failures,omitempty"`
		}{Changes: changes, Unblocks: unblocked, Failures: failures})
	}

	printRepriChanges(changes, *dryRun)
	printUnblockBumps(unblocked, *dryRun)

	for _, f := range failures {
		fmt.Fprintf(os.Stderr, "tq reprioritize: %s\n", f)
	}

	summary := fmt.Sprintf("%d change(s), %d unblock bump(s)", len(changes), len(unblocked))
	if *dryRun {
		summary = "dry-run: " + summary
	}

	fmt.Println(summary)

	if len(failures) > 0 {
		return fmt.Errorf("%d repo(s) failed", len(failures))
	}

	return nil
}

// printRepriChanges reports applied (or, dry-run, would-be) marker and
// importance re-resolutions.
func printRepriChanges(changes []harvest.RepriChange, dryRun bool) {
	for _, change := range changes {
		verb := "reprioritized"
		if dryRun {
			verb = "would reprioritize"
		}

		fmt.Printf("%s %s %d -> %d (%s): %s\n",
			verb, change.TaskID, change.OldPriority, change.NewPriority, change.Source, change.ItemText)
	}
}

// printUnblockBumps reports the unblock pass's priority bumps.
func printUnblockBumps(bumps []queue.UnblockChange, dryRun bool) {
	for _, bump := range bumps {
		verb := "unblocked"
		if dryRun {
			verb = "would bump"
		}

		fmt.Printf("%s %s %d -> %d (unblock, %d dep(s) completed)\n",
			verb, bump.TaskID, bump.OldPriority, bump.NewPriority, bump.CompletedDeps)
	}
}
