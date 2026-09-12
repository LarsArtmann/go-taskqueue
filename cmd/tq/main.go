// Command tq is the CLI for go-taskqueue: enqueue work, run workers, inspect
// the queue, and replay the journal.
package main

import (
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"runtime/debug"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/bridge/cqa"
	"github.com/larsartmann/go-taskqueue/internal/bridge/papdashboard"
	"github.com/larsartmann/go-taskqueue/internal/budget"
	"github.com/larsartmann/go-taskqueue/internal/dlqfix"
	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/harvest"
	"github.com/larsartmann/go-taskqueue/internal/httpapi"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/journal/cqrs"
	"github.com/larsartmann/go-taskqueue/internal/prioritize"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/review"
	"github.com/larsartmann/go-taskqueue/internal/runactor"
	"github.com/larsartmann/go-taskqueue/internal/status"
	"github.com/larsartmann/go-taskqueue/internal/task"
	"github.com/larsartmann/go-taskqueue/internal/webui"
	"github.com/larsartmann/go-taskqueue/internal/worker"
)

const usage = `tq — projects-aware task work queue

Usage:
  tq enqueue --type TYPE [--project P] [--payload JSON] [--deps id,...] [--priority N]
            [--max-attempts N] [--delay DUR] [--db PATH]
  tq worker [--concurrency N] [--agents [--yolo]] [--db PATH] [--poll DUR] [--lease DUR]
           [--task-timeout DUR] [--alert-url URL [--alert-api-key K]]
  tq harvest --projects-dir DIR [--repos a,b] [--max-per-tick N] [--allow-dirty]
            [--prune-stale] [--dry-run] [--db PATH]
  tq bootstrap [repos...] [--agents N] [--model M] [--reasoning R] [--verify n=cmd]
              [--install | --once | --dry-run] [--daily-budget N] [--db PATH]
             (one command from zero to a running agent pool: ensures .crushrc
              autonomy + .tq-verify, commits them, previews the harvest,
              then runs agent-pool — or installs the systemd unit)
  tq agent-pool --projects-dir DIR [--repos a,b] [--interval DUR] [--concurrency N]
               [--yolo] [--max-per-tick N] [--task-timeout DUR]
               [--cqa-url URL [--cqa-owner ID] [--cqa-token T]] [--db PATH]
  tq stats [--project P] [--status S] [--daily-budget N] [--db PATH] [--json]
  tq tasks [--project P] [--status S] [--type T] [--since DUR] [--limit N] [--json] [--db PATH]
  tq audit --projects-dir DIR [--repos a,b] [--todo-file F] [--type T]
          [--max-attempts N] [--dry-run] [--json] [--db PATH]
  tq doctor [--json] [--daily-budget N] [--repos a,b] [--db PATH]
  tq top [--interval DUR] [--once] [--json] [--db PATH]
  tq show TASK_ID [--db PATH]   (a unique ID prefix works)
  tq dlq [--db PATH] [--rescue TASK_ID [--max-attempts N]] [--dismiss TASK_ID [--reason WHY]]
tq cancel TASK_ID [--force] [--reason WHY] [--db PATH]   (--force: cooperative cancel of a running task)
  tq facts [--db PATH] [--after SEQ] [--cqrs]
  tq tail [-f] [--db PATH] [--after SEQ]
  tq watermarks show [--db PATH]   (journal consumer cursors)
  tq watermarks set CONSUMER SEQ [--db PATH]   (rewind = safe replay)
  tq session begin --id ID [--repo DIR] [--project P] [--db PATH]
                  (record an interactive session's opening; default id $CRUSH_SESSION_ID)
  tq session close --id ID [--repo DIR] [--project P] [--summary TEXT]
                  [--allow-dirty] [--db PATH]   (attribute the session's
                  Crush-Session-footer commits; enqueues one review + one
                  status task over them — the pool does the rest)
  tq serve [--addr ADDR] [--auth-token TOKEN] [--db PATH] [--poll DUR] [--verbose]
  tq api [--addr ADDR] --auth-token TOKEN [--db PATH]   (write API: POST /api/v1/tasks)
  tq version

Default database: $TQ_DB or ./tasks.db
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	commands := map[string]func([]string) error{
		"bootstrap":    cmdBootstrap,
		"enqueue":      cmdEnqueue,
		"worker":       cmdWorker,
		"harvest":      cmdHarvest,
		"reprioritize": cmdReprioritize,
		"agent-pool":   cmdAgentPool,
		"stats":        cmdStats,
		"tasks":        cmdTasks,
		"audit":        cmdAudit,
		"doctor":       cmdDoctor,
		"top":          cmdTop,
		"show":         cmdShow,
		"dlq":          cmdDLQ,
		"cancel":       cmdCancel,
		"facts":        cmdFacts,
		"tail":         cmdTail,
		"watermarks":   cmdWatermarks,
		"session":      cmdSession,
		"serve":        cmdServe,
		"version":      cmdVersion,
		"api":          cmdAPI,
	}

	switch name := os.Args[1]; name {
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		cmd, ok := commands[name]
		if !ok {
			fmt.Fprintf(os.Stderr, "tq: unknown command %q\n\n%s", name, usage)
			os.Exit(2)
		}

		if err := cmd(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "tq: %v\n", err)
			os.Exit(1)
		}
	}
}

func defaultDB() string {
	if p := os.Getenv("TQ_DB"); p != "" {
		return p
	}

	return "tasks.db"
}

func mustOpenDB(path string) *sqlite.Store {
	return mustOpenDBOpts(path)
}

func mustOpenDBOpts(path string, opts ...sqlite.StoreOption) *sqlite.Store {
	store, err := sqlite.Open(path, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tq: open db: %v\n", err)
		os.Exit(1)
	}

	return store
}

func dbFlag(fs *flag.FlagSet) *string {
	return fs.String("db", "", "database path (default $TQ_DB or ./tasks.db)")
}

func resolveDB(v string) string {
	if v != "" {
		return v
	}

	return defaultDB()
}

func splitRepos(spec string) []string {
	var repos []string

	for r := range strings.SplitSeq(spec, ",") {
		if r = strings.TrimSpace(r); r != "" {
			repos = append(repos, r)
		}
	}

	return repos
}

// expandRepoSpecs makes --repos entries cwd-independent for the harvest,
// audit, and doctor commands: absolute paths and relative paths that exist
// against the working directory pass through, while anything else joins the
// projects dir — so a bare repo name ("alpha") resolves there instead of
// becoming <cwd>/alpha when the sweep Abs()es it (doctor stats the joined
// path directly). bootstrap.resolveRepos and the agent-pool option parse
// apply the same policy. Specs that resolve nowhere are left as-is: the
// sweeps report them per-repo as scan failures.
func expandRepoSpecs(projectsDir string, specs []string) []string {
	expanded := make([]string, len(specs))
	for i, spec := range specs {
		if !filepath.IsAbs(spec) {
			if _, err := os.Stat(spec); err != nil && projectsDir != "" {
				spec = filepath.Join(projectsDir, spec)
			}
		}

		expanded[i] = spec
	}

	return expanded
}

func cmdEnqueue(args []string) error {
	fs := flag.NewFlagSet("enqueue", flag.ExitOnError)
	project := fs.String("project", "", "project the task belongs to")
	taskType := fs.String("type", "", "task type (required)")
	payload := fs.String("payload", "", "JSON payload (or @file)")
	deps := fs.String("deps", "", "comma-separated dependency task IDs")
	priority := fs.Int("priority", 0, "higher claims first")
	maxAttempts := fs.Int("max-attempts", 0, "default 3")
	delay := fs.Duration("delay", 0, "delay before claimable (e.g. 30s, 5m)")

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *taskType == "" {
		return errors.New("--type is required")
	}

	var payloadJSON json.RawMessage

	if *payload != "" {
		raw := []byte(*payload)
		if after, ok := strings.CutPrefix(*payload, "@"); ok {
			b, err := os.ReadFile(after)
			if err != nil {
				return fmt.Errorf("read payload file: %w", err)
			}

			raw = b
		}

		if !json.Valid(raw) {
			// The shell path takes the payload as the command line itself
			// (tq enqueue --type sh --payload 'echo hi'), so wrap a non-JSON
			// payload as a JSON string instead of rejecting it. The stored
			// payload is always valid JSON.
			if *taskType != "sh" {
				return fmt.Errorf("payload is not valid JSON: %s", raw)
			}

			wrapped, err := json.Marshal(string(raw))
			if err != nil {
				return fmt.Errorf("wrap payload: %w", err)
			}

			raw = wrapped
		}

		payloadJSON = raw
	}

	newTask := task.New{
		Project:     *project,
		Type:        *taskType,
		Payload:     payloadJSON,
		Priority:    *priority,
		MaxAttempts: *maxAttempts,
		NotBefore:   time.Now().Add(*delay),
	}

	for d := range strings.SplitSeq(*deps, ",") {
		if d = strings.TrimSpace(d); d != "" {
			newTask.Deps = append(newTask.Deps, task.ID(d))
		}
	}

	s := mustOpenDB(resolveDB(*db))
	defer s.Close()

	t, err := queue.New(s).Enqueue(context.Background(), newTask)
	if err != nil {
		return err
	}

	fmt.Println(t.ID)

	return nil
}

func cmdWorker(args []string) error {
	fs := flag.NewFlagSet("worker", flag.ExitOnError)
	conc := fs.Int("concurrency", 2, "parallel executions")
	poll := fs.Duration("poll", 250*time.Millisecond, "idle poll interval")
	lease := fs.Duration("lease", 2*time.Minute, "claim lease length")
	timeout := fs.Duration("task-timeout", 10*time.Minute, "per-task timeout (use e.g. 45m with --agents)")
	owner := fs.String("owner", "", "lease owner identity")
	agents := fs.Bool(
		"agents",
		false,
		"enable the 'agent' executor: runs a headless AI agent (crush) per task — OPT-IN",
	)
	yolo := fs.Bool("yolo", false, "with --agents: agents auto-accept all permissions (operator decision)")
	exclusive := fs.Bool(
		"project-exclusive",
		false,
		"never run two tasks of the same project at once across ALL pools sharing this DB (enable it on every pool)",
	)
	projectsDir := fs.String("projects-dir", defaultProjectsDir(), "root dir for relative repo names in agent payloads")
	alertURL := fs.String(
		"alert-url",
		os.Getenv("TQ_PAP_URL"),
		"PapDashboard base URL: dead-lettered tasks raise alerts there (e.g. http://localhost:8080)",
	)
	alertKey := fs.String("alert-api-key", os.Getenv("TQ_PAP_API_KEY"), "PapDashboard API key (Bearer)")
	alertPoll := fs.Duration("alert-poll", 5*time.Second, "journal tail interval for alert forwarding")
	once := fs.Bool(
		"once",
		false,
		"run until the claimable queue is drained, then exit (scripts/tests; parity with agent-pool --once)",
	)

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	var opts []sqlite.StoreOption
	if *exclusive {
		opts = append(opts, sqlite.WithProjectExclusivity())
	}

	store := mustOpenDBOpts(resolveDB(*db), opts...)

	// The "sh" executor with empty template runs the payload itself as the
	// shell line ({"cmd":...} JSON is unwrapped). This keeps the CLI path
	// trivially usable while Go users register their own executors.
	reg := executor.NewRegistry()
	reg.Register("sh", executor.NewCommandExecutor(""))

	if *agents {
		fmt.Fprintln(
			os.Stderr,
			"tq: --agents: autonomous agent execution enabled (headless crush; dirty repos are skipped; verify is enforced)",
		)

		registerAgentExecutors(reg, &executor.AgentExecutor{ProjectsDir: *projectsDir, Yolo: *yolo})
	}

	pool := worker.New(store, worker.Config{
		Owner:        *owner,
		Concurrency:  *conc,
		PollInterval: *poll,
		Lease:        *lease,
		TaskTimeout:  *timeout,
		Executors:    reg,
	}, nil)

	// One signal story (runactor): interrupt cancels the pool loop and the
	// bridge; in-flight tasks finish under their own execution scope
	// (bounded only by --task-timeout); teardown closes the store AFTER
	// everything has stopped, so bridge checkpoints always land.
	g := runactor.New(context.Background())
	g.InterruptOn(os.Interrupt, syscall.SIGTERM)
	g.OnShutdown(func() error { return store.Close() })

	if *alertURL != "" {
		bridge := papdashboard.New(store, store, papdashboard.Config{
			Endpoint:     *alertURL,
			APIKey:       *alertKey,
			PollInterval: *alertPoll,
		})

		g.Go("alert-bridge", func(ctx context.Context) error { return bridge.Run(ctx) })

		fmt.Fprintf(os.Stderr, "tq: forwarding dead letters to %s\n", *alertURL)
	}

	if *once {
		// Timer-friendly mode (parity with agent-pool --once): as soon as
		// this pool has nothing in flight and no claimable work left, end
		// the group — Start returns once ctx is done, so Stop alone would
		// leave the process hanging until the next signal. Work claimed by
		// OTHER pools, or gated by a future NotBefore, is left for them /
		// for the next --once run.
		g.Go("once-drain", func(ctx context.Context) error {
			queue := queue.New(store)

			for {
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(*poll):
					if pool.InFlight() == 0 && !hasClaimableWork(ctx, queue, pool.Owner(), *poll) {
						pool.Stop()

						return nil
					}
				}
			}
		})
	}

	g.Go("pool", func(ctx context.Context) error { return pool.Start(ctx) })

	return g.Run()
}

// defaultProjectsDir resolves the agent projects root: $TQ_PROJECTS_DIR or
// ~/projects.
func defaultProjectsDir() string {
	if d := os.Getenv("TQ_PROJECTS_DIR"); d != "" {
		return d
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, "projects")
}

// defaultLogDir is where agent output sidecars land unless overridden:
// full stdout + verify output per task, in the XDG state dir (logs are
// state, not config — they may be deleted without breaking anything).
func defaultLogDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".local", "state", "tq", "logs")
}

func cmdHarvest(args []string) error {
	fs := flag.NewFlagSet("harvest", flag.ExitOnError)
	projectsDir := fs.String(
		"projects-dir",
		defaultProjectsDir(),
		"dir containing repos with TODO_LIST.md (default $TQ_PROJECTS_DIR or ~/projects)",
	)
	repos := fs.String("repos", "", "comma-separated repo dirs (overrides --projects-dir)")
	todoFile := fs.String("todo-file", harvest.DefaultTodoFile, "backlog file name inside each repo")
	taskType := fs.String("type", harvest.DefaultType, "task type to enqueue")
	maxPerTick := fs.Int("max-per-tick", harvest.DefaultMaxPerTick, "max new agent tasks per run (cost throttle)")
	priority := fs.Int("priority", 0, "priority for enqueued tasks")
	maxAttempts := fs.Int("max-attempts", 0, "attempt budget (0 = store default)")
	allowDirty := fs.Bool("allow-dirty", false, "let agents run in repos with uncommitted changes (default: refuse)")
	model := fs.String(
		"model",
		"",
		"crush model override (e.g. anthropic/claude-sonnet-4-5) written into every harvested agent payload",
	)
	repoSubset := fs.String(
		"repo-subset",
		"",
		"glob filter on repo names discovered under --projects-dir (e.g. 'go-*'); ignored with --repos",
	)
	discoveryAddr := fs.String(
		"discovery-addr",
		os.Getenv("TQ_DISCOVERY_ADDR"),
		"project-discovery-daemon endpoint for repo discovery INSTEAD of the local scan: unix socket (/run/project-discovery/daemon.sock, unix:// ok) or host:port; unreachable daemon = warning + local scan fallback ($TQ_DISCOVERY_ADDR)",
	)
	dryRun := fs.Bool("dry-run", false, "report what would be enqueued, change nothing")
	sameSessionPriority := fs.Int(
		"same-session-priority",
		0,
		"enqueue harvested items whose text references /tmp paths at this priority (hot: work them this session, the files will not survive); 0 disables",
	)
	priorityFrom := fs.String(
		"priority-from",
		"",
		`resolve harvested priorities from each repo's .config/metadata.yaml importance (0-100, default 50) plus keyword bumps, clamped to the backlog band; "importance" enables, empty keeps the flat --priority (markers and hot promotion apply either way; ADR-0015)`,
	)
	maxPendingPerRepo := fs.Int(
		"max-pending-per-repo",
		0,
		"cap how many PENDING tasks one repo may hold in the queue (0 = legacy: any pending/running task holds the repo). The queue becomes the working set; TODO_LIST.md is the warehouse; admission resumes as claims drain",
	)
	asJSON := fs.Bool("json", false, "JSON output of the harvest result")
	pruneStale := fs.Bool(
		"prune-stale",
		false,
		"cancel PENDING queue tasks whose TODO_LIST item is now [x] (dedup-key match) instead of harvesting, so a pool relaunch never inherits zombies; running tasks are only reported (use tq cancel for a cooperative stop)",
	)

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *priorityFrom != "" && *priorityFrom != "importance" {
		return fmt.Errorf(`--priority-from: want "importance" or empty, got %q`, *priorityFrom)
	}

	if *projectsDir == "" && *repos == "" {
		return errors.New("no repos: pass --repos or --projects-dir (or set $TQ_PROJECTS_DIR)")
	}

	if err := checkProjectsDir(*projectsDir); err != nil {
		return err
	}

	cfg := harvest.Config{
		ProjectsDir:         *projectsDir,
		DiscoveryAddr:       *discoveryAddr,
		Log:                 slog.Default(),
		TodoFile:            *todoFile,
		Type:                *taskType,
		MaxPerTick:          *maxPerTick,
		Priority:            *priority,
		MaxAttempts:         *maxAttempts,
		Model:               *model,
		SameSessionPriority: *sameSessionPriority,
		UseImportance:       *priorityFrom == "importance",
		MaxPendingPerRepo:   *maxPendingPerRepo,
		DryRun:              *dryRun,
	}

	if *allowDirty {
		no := false
		cfg.RequireClean = &no
	}

	// --repos overrides --projects-dir; --repo-subset filters the discovered set.
	if err := resolveHarvestRepos(&cfg, *projectsDir, *repos, *repoSubset); err != nil {
		return err
	}

	store := mustOpenDB(resolveDB(*db))
	defer store.Close()

	if *pruneStale {
		pruned, err := harvest.New(queue.New(store), cfg).PruneStale(context.Background())
		if err != nil {
			return err
		}

		if *asJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")

			return enc.Encode(pruned)
		}

		printPruneResult(pruned, *dryRun)

		return nil
	}

	res, err := harvest.New(queue.New(store), cfg).Run(context.Background())
	if err != nil {
		return err
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")

		return enc.Encode(res)
	}

	printHarvestResult(res)
	printHarvestLines(res, *dryRun)

	return nil
}

// resolveHarvestRepos applies the --repos / --repo-subset flag pair to the
// harvest config. Both empty leaves ProjectsDir as-is (discover every repo);
// --repos wins over --projects-dir, --repo-subset filters discovered repos
// (daemon-backed when --discovery-addr is set, same fallback as the ticks).
func resolveHarvestRepos(cfg *harvest.Config, projectsDir, repos, subset string) error {
	if repos != "" {
		cfg.ProjectsDir = ""
		cfg.Repos = expandRepoSpecs(projectsDir, splitRepos(repos))

		return nil
	}

	if subset == "" {
		return nil
	}

	found, err := harvest.DiscoverReposFor(context.Background(), cfg.DiscoveryAddr, projectsDir, cfg.TodoFile, cfg.Log)
	if err != nil {
		return fmt.Errorf("discover repos: %w", err)
	}

	for _, r := range found {
		if ok, _ := path.Match(subset, filepath.Base(r)); ok {
			cfg.Repos = append(cfg.Repos, r)
		}
	}

	if len(cfg.Repos) == 0 {
		return fmt.Errorf("--repo-subset %q matched no repos under %s", subset, projectsDir)
	}

	return nil
}

// printHarvestLines prints one line per enqueued and skipped backlog item.
func printHarvestLines(res harvest.Result, dryRun bool) {
	for _, enqueued := range res.Enqueued {
		id := enqueued.TaskID.String()
		if dryRun {
			id = "(dry-run)"
		}

		fmt.Printf("ENQUEUED  %-24s %s  %s%s\n", enqueued.Item.RepoName, enqueued.Item.Text, id, hotMark(enqueued.Hot))
	}

	for _, sk := range res.Skipped {
		fmt.Printf("SKIP      %-24s %s  — %s\n", sk.Item.RepoName, sk.Item.Text, sk.Reason)
	}
}

// hotMark renders the /tmp hot marker for harvest output.
func hotMark(hot bool) string {
	if hot {
		return "  [hot:/tmp]"
	}

	return ""
}

// printPruneResult renders a --prune-stale pass: cancelled zombies first
// (ticked item, or item text gone from the file), then running/dead
// stale-item tasks the sweep deliberately does not touch.
func printPruneResult(res harvest.PruneResult, dryRun bool) {
	verb := "CANCELLED "
	if dryRun {
		verb = "WOULD-CANCEL"
	}

	for _, c := range res.Cancelled {
		fmt.Printf("%s  %-24s %s  %s\n", verb, c.Item.RepoName, pruneItemText(c.Item, c.Why), c.TaskID)
	}

	for _, r := range res.Running {
		fmt.Printf(
			"RUNNING   %-24s %s  %s  — cooperative stop is an operator decision (tq cancel)\n",
			r.Item.RepoName,
			pruneItemText(r.Item, r.Why),
			r.TaskID,
		)
	}

	for _, d := range res.Dead {
		fmt.Printf(
			"DEAD      %-24s %s  %s  — already terminal (tq dlq --rescue to retry)\n",
			d.Item.RepoName,
			pruneItemText(d.Item, d.Why),
			d.TaskID,
		)
	}

	for _, f := range res.ScanFailures {
		fmt.Printf("SKIP      %s  — %s\n", f.Repo, f.Reason)
	}

	fmt.Printf("%d repo(s): %d cancelled, %d running, %d dead\n",
		res.Repos, len(res.Cancelled), len(res.Running), len(res.Dead))
}

// pruneItemText renders a pruned task's item for the report lines: the text
// when it still exists, the dedup key when the item is gone from the file.
func pruneItemText(it harvest.Item, why harvest.PruneWhy) string {
	if why == harvest.PruneAbsent {
		return "(" + it.Key + " — item text no longer in file)"
	}

	return it.Text
}

// cmdAgentPool is the self-managing loop in one process: it repeatedly
// harvests TODO_LIST.md backlogs into the queue (paced: one in-flight item
// per repo), optionally ingests Code-Quality-Agent scan findings as fix
// tasks, and runs a worker pool whose "agent" executor drives headless crush
// agents that do the work, verify it, and close the loop in the todo file.
// Ctrl-C drains gracefully, like tq worker.
func cmdAgentPool(args []string) error {
	poolOpts, err := parseAgentPoolOptions(args)
	if err != nil {
		return err
	}

	cfg, err := harvestConfigFromOptions(poolOpts)
	if err != nil {
		return err
	}

	var storeOpts []sqlite.StoreOption
	if poolOpts.exclusive {
		storeOpts = append(storeOpts, sqlite.WithProjectExclusivity())
	}

	store := mustOpenDBOpts(resolveDB(poolOpts.db), storeOpts...)
	defer store.Close()

	taskQueue := queue.New(store)

	agentExec := &executor.AgentExecutor{
		ProjectsDir:   poolOpts.projectsDir,
		Yolo:          poolOpts.yolo,
		MaxConcurrent: poolOpts.maxAgents,
	}
	if poolOpts.closeout {
		agentExec.CloseoutPrompt = executor.DefaultCloseoutPrompt
	}

	reg := executor.NewRegistry()
	reg.Register("sh", executor.NewCommandExecutor(""))
	registerAgentExecutors(reg, agentExec)

	printAgentPoolBanner(poolOpts)

	// One signal story (runactor): interrupt cancels the pool loop, the
	// tick actor and the bridge; in-flight agent tasks finish under their
	// own execution scope (bounded only by --task-timeout); teardown closes
	// the store AFTER everything stopped, so bridge checkpoints always land.
	g := runactor.New(context.Background())
	g.InterruptOn(os.Interrupt, syscall.SIGTERM)
	g.OnShutdown(func() error { return store.Close() })

	ctx := g.Ctx()

	var alertBridge *papdashboard.Bridge

	if poolOpts.alertURL != "" {
		alertBridge = papdashboard.New(store, store, papdashboard.Config{
			Endpoint:     poolOpts.alertURL,
			APIKey:       poolOpts.alertKey,
			PollInterval: poolOpts.alertPoll,
			// Mirror the pool'store cap so the day it bites, an alert fires (and
			// resolves itself when the window rolls over).
			DailyBudget: poolOpts.dailyBudget,
		})

		g.Go("alert-bridge", func(ctx context.Context) error { return alertBridge.Run(ctx) })

		fmt.Fprintf(os.Stderr, "tq: agent-pool: forwarding dead letters + budget exhaustion to %s\n", poolOpts.alertURL)
	}

	log := slog.Default()
	cfg.Log = log
	guard := budget.Guard{DailyCap: poolOpts.dailyBudget, BudgetCmd: poolOpts.budgetCmd}
	harvester := harvest.New(taskQueue, cfg)

	// Dead-pool detection (02:00 f6): the skip log makes a blind pool loud
	// on the FIRST tick; the detector decides when it has STAYED blind — N
	// consecutive all-repos scan-failed ticks — and raises (then resolves)
	// a PapDashboard alert, so the incident surfaces outside journald too.
	deadPool := &deadPoolDetector{ticks: poolOpts.deadPoolTicks}
	deadPool.notify = func(triggered bool, repos int, example string, streak int) {
		if triggered {
			log.Warn("dead pool: every repo scan-failed", "ticks", streak, "repos", repos, "example", example)
		} else {
			log.Info("dead pool resolved", "repos", repos)
		}

		if alertBridge == nil {
			return
		}

		if err := alertBridge.NotifyDeadPool(ctx, triggered, repos, example, streak); err != nil {
			log.Error("dead-pool alert failed", "triggered", triggered, "err", err)
		}
	}

	// Startup zombie sweep, SYNCHRONOUSLY before any actor starts: the
	// worker'store first claim would otherwise race the sweep and turn
	// cancellable zombies into running tasks (observed in the e2e). One
	// pass, then never again; --prune-stale=false disables it for operators
	// who want relaunches to inherit everything.
	if poolOpts.pruneStale {
		res, err := harvester.PruneStale(ctx)
		if err != nil {
			log.Warn("startup prune-stale failed", "err", err)
		} else {
			for _, cancelled := range res.Cancelled {
				log.Warn(
					"startup prune: cancelled stale task",
					"repo",
					cancelled.Item.RepoName,
					"why",
					cancelled.Why,
					"item",
					pruneItemText(cancelled.Item, cancelled.Why),
					"task",
					cancelled.TaskID.String(),
				)
			}

			for _, r := range res.Running {
				log.Warn("startup prune: stale item task already running (left alone)",
					"repo", r.Item.RepoName, "why", r.Why, "task", r.TaskID.String())
			}

			for _, f := range res.ScanFailures {
				log.Warn("startup prune: repo skipped", "repo", f.Repo, "reason", f.Reason)
			}
		}
	}

	// Startup reprioritize sweep, same pre-actor slot as prune-stale
	// (ADR-0015 §5): marker edits and importance changes reach PENDING
	// tasks on every pool relaunch. Value-idempotent — same-value
	// resolutions write nothing — and free (local SQL, no agent spend).
	if poolOpts.reprioritize {
		changes, failures := harvester.Reprioritize(ctx, false)
		for _, c := range changes {
			log.Info("startup reprioritize",
				"task", c.TaskID.String(),
				"old", c.OldPriority,
				"new", c.NewPriority,
				"source", string(c.Source),
				"item", c.ItemText,
			)
		}

		for _, f := range failures {
			log.Warn("startup reprioritize: repo skipped", "reason", f)
		}
	}

	var cqaBridge *cqa.Bridge
	if poolOpts.cqaURL != "" {
		cqaBridge = cqa.New(cqa.Config{
			BaseURL:     poolOpts.cqaURL,
			Token:       poolOpts.cqaToken,
			OwnerID:     poolOpts.cqaOwner,
			ProjectsDir: poolOpts.projectsDir,
		})
	}

	var sweeper *review.Sweeper

	if poolOpts.doReview {
		var err error

		sweeper, err = review.NewSweeper(ctx, store, review.SweeperConfig{
			Model:   poolOpts.model,
			Autofix: poolOpts.reviewAutofix,
			Log:     log,
		})
		if err != nil {
			return fmt.Errorf("review sweeper: %w", err)
		}
	}

	var statusSweeper *status.Sweeper

	if poolOpts.statusEvery > 0 {
		var err error

		statusSweeper, err = status.NewSweeper(ctx, store, status.SweeperConfig{
			Every:       poolOpts.statusEvery,
			Model:       poolOpts.model,
			Log:         log,
			AllowDirty:  poolOpts.allowDirty,
			TaskTimeout: poolOpts.timeout,
		})
		if err != nil {
			return fmt.Errorf("status sweeper: %w", err)
		}
	}

	var dlqfixSweeper *dlqfix.Sweeper

	if poolOpts.dlqFix {
		var err error

		dlqfixSweeper, err = dlqfix.NewSweeper(ctx, store, dlqfix.SweeperConfig{
			Model: poolOpts.model,
			Log:   log,
		})
		if err != nil {
			return fmt.Errorf("dlqfix sweeper: %w", err)
		}
	}

	var prioritizeSweeper *prioritize.Sweeper

	if poolOpts.prioritize {
		var err error

		prioritizeSweeper, err = prioritize.NewSweeper(ctx, store, prioritize.SweeperConfig{
			Model:      poolOpts.model,
			Yolo:       poolOpts.yolo,
			AllowDirty: poolOpts.allowDirty,
			BootMint:   true,
			Log:        log,
		})
		if err != nil {
			return fmt.Errorf("prioritize sweeper: %w", err)
		}
	}

	// mintPass gates one budget-consuming pass: EVERY enqueue (harvested,
	// review, status, CQA) must clear the guard immediately before it — a
	// completion inside the same tick can spend the last slot after an
	// earlier check passed, and the documented cap is "EVERY enqueue"
	// (SECURITY.md). Returns false when the guard refused.
	mintPass := func(name string, mint func()) bool {
		if ok, reason := guard.Check(ctx, store); !ok {
			log.Warn("budget: skipping "+name, "reason", reason)

			return false
		}

		mint()

		return true
	}

	// skipLogExamples remembers the last logged example per skip class:
	// a class is logged only on first sight or when its example changes,
	// so a steady state does not flood journald every tick.
	skipLogExamples := map[string]string{}

	runTick := func() {
		// Sidecar retention: sweep aged logs before new work so a
		// long-running pool'store output directory cannot grow forever.
		if poolOpts.logDir != "" && poolOpts.logDirMaxAge > 0 {
			if removed, err := executor.SweepSidecars(poolOpts.logDir, poolOpts.logDirMaxAge); err != nil {
				log.Warn("sidecar sweep failed", "err", err)
			} else if removed > 0 {
				log.Info("sidecar sweep", "removed", removed, "dir", poolOpts.logDir)
			}
		}

		// Byte-budget retention: age alone cannot bound a high-traffic dir.
		if poolOpts.logDir != "" && poolOpts.logDirMaxBytes > 0 {
			if removed, err := executor.SweepSidecarsByBytes(poolOpts.logDir, poolOpts.logDirMaxBytes); err != nil {
				log.Warn("sidecar byte sweep failed", "err", err)
			} else if removed > 0 {
				log.Info("sidecar byte sweep", "removed", removed, "dir", poolOpts.logDir)
			}
		}

		if !mintPass("harvest tick", func() {
			res, err := harvester.Run(ctx)
			if err != nil {
				log.Error("harvest failed", "err", err)
			} else {
				for _, enqueued := range res.Enqueued {
					log.Info(
						"harvest: enqueued",
						"repo",
						enqueued.Item.RepoName,
						"item",
						enqueued.Item.Text,
						"task",
						enqueued.TaskID.String(),
					)
				}

				for class, g := range groupedSkips(res.Skipped) {
					if prev, seen := skipLogExamples[class]; seen && prev == g.example {
						continue
					}

					skipLogExamples[class] = g.example
					if class == harvest.ReasonScanFailed {
						// A scan failure means the pool cannot see a repo at
						// all — surface the full reason, not just the class,
						// or a dead deployment reads as a quiet one.
						log.Warn("harvest: skipped", "reason", class, "count", g.count, "example", g.example)

						continue
					}

					log.Info("harvest: skipped", "reason", class, "count", g.count, "example", g.example)
				}

				log.Info("harvest tick done", "repos", res.Repos, "items", res.Items,
					"enqueued", len(res.Enqueued), "skipped", len(res.Skipped))

				deadPool.observe(res)
			}
		}) {
			return
		}

		if sweeper != nil {
			mintPass("review sweep", func() {
				stats, err := sweeper.Sweep(ctx)
				if err != nil {
					log.Error("review sweep failed", "err", err)
				} else if stats.ReviewsEnqueued > 0 || stats.FixesEnqueued > 0 || stats.Skipped > 0 {
					log.Info("review sweep done", "facts", stats.Facts,
						"reviews", stats.ReviewsEnqueued, "known", stats.ReviewsKnown,
						"fixes", stats.FixesEnqueued, "skipped", stats.Skipped)
				}
			})
		}

		if statusSweeper != nil {
			mintPass("status sweep", func() {
				stats, err := statusSweeper.Sweep(ctx)
				if err != nil {
					log.Error("status sweep failed", "err", err)
				} else if stats.ReportsEnqueued > 0 || stats.Skipped > 0 {
					log.Info("status sweep done", "facts", stats.Facts,
						"reports", stats.ReportsEnqueued, "known", stats.ReportsKnown, "skipped", stats.Skipped)
				}
			})
		}

		if dlqfixSweeper != nil {
			mintPass("dlq-fix sweep", func() {
				stats, err := dlqfixSweeper.Sweep(ctx)
				if err != nil {
					log.Error("dlq-fix sweep failed", "err", err)
				} else if stats.FixesEnqueued > 0 || stats.Rescued > 0 || stats.Dismissed > 0 || stats.Skipped > 0 {
					log.Info("dlq-fix sweep done", "facts", stats.Facts,
						"autopsies", stats.FixesEnqueued, "known", stats.FixesKnown,
						"rescued", stats.Rescued, "dismissed", stats.Dismissed, "skipped", stats.Skipped)
				}
			})
		}

		if prioritizeSweeper != nil {
			mintPass("prioritize sweep", func() {
				stats, err := prioritizeSweeper.Sweep(ctx)
				if err != nil {
					log.Error("prioritize sweep failed", "err", err)
				} else if stats.BatchesEnqueued > 0 || stats.VerdictsCached > 0 || stats.Skipped > 0 {
					log.Info("prioritize sweep done", "facts", stats.Facts,
						"batches", stats.BatchesEnqueued, "known", stats.BatchesKnown,
						"verdicts", stats.VerdictsCached, "reprioritized", stats.TasksReprioritized,
						"skipped", stats.Skipped)
				}
			})
		}

		if cqaBridge == nil {
			return
		}

		mintPass("cqa ingest", func() {
			fixTasks, err := cqaBridge.Collect(ctx)
			if err != nil {
				log.Error("cqa ingest failed", "err", err)

				return
			}

			fresh := 0

			for _, fixTask := range fixTasks {
				got, err := taskQueue.Enqueue(ctx, fixTask.Template)
				if err != nil {
					log.Error("cqa enqueue failed", "key", fixTask.Template.DedupKey, "err", err)

					continue
				}

				if got.Attempts == 0 && got.Status == task.Pending {
					fresh++

					log.Info(
						"cqa: enqueued fix task",
						"repo",
						fixTask.Project,
						"file",
						fixTask.File,
						"issues",
						len(fixTask.Issues),
						"task",
						got.ID.String(),
					)
				}
			}

			if len(fixTasks) > 0 {
				log.Info("cqa tick done", "files", len(fixTasks), "new", fresh)
			}
		})
	}

	var watchTriggers <-chan struct{}

	// Watch-driven harvest trigger: with a daemon addr, subscribe its
	// GET /v1/watch SSE stream so a repo change (an agent commit carrying
	// its TODO_LIST edit) harvests within seconds instead of waiting for
	// --interval. Additive by contract: the ticker below stays the fallback
	// heartbeat and a nil trigger channel blocks its select case forever,
	// so a dead watch stream degrades to interval-only harvesting.
	if poolOpts.discoveryAddr != "" && !poolOpts.once {
		watcher := harvest.NewWatcher(harvest.WatchConfig{
			Addr:          poolOpts.discoveryAddr,
			ProjectsDir:   cfg.ProjectsDir,
			Repos:         cfg.Repos,
			RepoIntervals: cfg.RepoIntervals,
			Log:           log,
		})
		watchTriggers = watcher.Triggers()

		g.Go("discovery-watch", func(ctx context.Context) error { return watcher.Run(ctx) })

		fmt.Fprintf(
			os.Stderr,
			"tq: agent-pool: watch-driven harvest triggers from %s (interval %s stays the fallback)\n",
			poolOpts.discoveryAddr,
			poolOpts.interval,
		)
	}

	g.Go("tick", func(ctx context.Context) error {
		runTick()

		if poolOpts.once {
			// The once-drain actor decides when the group ends; this actor
			// must not return before that (a clean return would end it).
			<-ctx.Done()

			return nil
		}

		ticker := time.NewTicker(poolOpts.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				runTick()
			case <-watchTriggers:
				runTick()
			}
		}
	})

	pool := worker.New(store, worker.Config{
		Owner:        poolOpts.owner,
		Concurrency:  poolOpts.conc,
		PollInterval: poolOpts.poll,
		Lease:        poolOpts.lease,
		TaskTimeout:  poolOpts.timeout,
		Executors:    reg,
	}, log)

	if poolOpts.once {
		// Timer-friendly mode: as soon as this pool has nothing in flight
		// and no claimable work left, end the group — Start only returns
		// once ctx is done, so Stop alone would leave the process hanging
		// until the next signal. Work claimed by OTHER pools, or gated by a
		// future NotBefore, is left for them / for the next --once run.
		g.Go("once-drain", func(ctx context.Context) error {
			for {
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(poolOpts.poll):
					// Sweep before the drain check so reviews and status reports
					// of work this drain just completed run in the SAME --once
					// process (idempotent; dedup keeps repeat sweeps free).
					mintPass("drain sweep", func() {
						if sweeper != nil {
							if _, err := sweeper.Sweep(ctx); err != nil {
								log.Error("review sweep failed", "err", err)
							}
						}

						if statusSweeper != nil {
							if _, err := statusSweeper.Sweep(ctx); err != nil {
								log.Error("status sweep failed", "err", err)
							}
						}
					})

					if pool.InFlight() == 0 && !hasClaimableWork(ctx, taskQueue, pool.Owner(), poolOpts.poll) {
						pool.Stop()

						return nil
					}
				}
			}
		})
	}

	g.Go("pool", func(ctx context.Context) error { return pool.Start(ctx) })

	return g.Run()
}

// hasClaimableWork reports whether any task is running under this owner or
// pending and due soon — the drain condition for `tq agent-pool --once`.
// Tasks owned by other pools or scheduled for later do not block the exit.
func hasClaimableWork(ctx context.Context, q *queue.Queue, owner string, poll time.Duration) bool {
	tasks, err := q.List(ctx, queue.Filter{})
	if err != nil {
		return true // fail safe: keep draining rather than exit early
	}

	dueSoon := time.Now().Add(2 * poll)

	for _, t := range tasks {
		if t.Status == task.Running && t.LeaseOwner == owner {
			return true
		}

		if t.Status == task.Pending && !t.NotBefore.After(dueSoon) {
			return true
		}
	}

	return false
}

func repoRootDesc(projectsDir, repos string) string {
	if repos != "" {
		return repos
	}

	return projectsDir
}

// groupedSkips counts skip reasons by their stable class (text before ':').
type skipClass struct {
	count   int
	example string
}

func groupedSkips(skips []harvest.Skipped) map[string]skipClass {
	groups := make(map[string]skipClass)

	for _, skip := range skips {
		class := skip.Reason
		if before, _, ok := strings.Cut(skip.Reason, ":"); ok {
			class = before
		}

		g := groups[class]

		g.count++
		if g.example == "" {
			g.example = truncateSkipReason(skip.Reason)
		}

		groups[class] = g
	}

	return groups
}

func truncateSkipReason(reason string) string {
	const maxLen = 200
	if len(reason) <= maxLen {
		return reason
	}

	return reason[:maxLen] + "…"
}

func printHarvestResult(res harvest.Result) {
	fmt.Printf("tq: harvest: %d repos, %d open items, %d newly enqueued, %d skipped\n",
		res.Repos, res.Items, len(res.Enqueued), len(res.Skipped))
}

func cmdStats(args []string) error {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	project := fs.String("project", "", "filter by project")
	status := fs.String("status", "", "filter by status")
	dailyBudget := fs.Int(
		"daily-budget",
		0,
		"agent pool daily enqueue cap to compare today'store spend against (0 = spend shown without a cap)",
	)
	asJSON := fs.Bool("json", false, "JSON output of the stats aggregate (counts, budget, consumer lag)")

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	store := mustOpenDB(resolveDB(*db))
	defer store.Close()

	ctx := context.Background()

	filter := queue.Filter{}
	if *project != "" {
		filter.Project = project
	}

	if *status != "" {
		st := task.Status(*status)
		filter.Status = &st
	}

	tasks, err := store.List(ctx, filter)
	if err != nil {
		return err
	}

	// Parked = rate-limit parked (pending with a future not_before) — the
	// "11 tasks parked until 19:40" one-glance count (13:29 report f11/f49).
	parked := true

	parkedCount, err := store.CountTasks(ctx, queue.Filter{Project: filter.Project, Parked: &parked})
	if err != nil {
		return err
	}

	head, err := store.HeadSeq(ctx)
	if err != nil {
		return err
	}

	byStatus, byProject := tallyStats(tasks)
	spent := budget.Guard{DailyCap: *dailyBudget}.SpentToday(ctx, store)

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")

		return enc.Encode(statsPayload{
			ByStatus:    byStatus,
			ByProject:   byProject,
			Budget:      budgetView{SpentToday: spent, Cap: *dailyBudget},
			Lag:         consumerLag(ctx, store),
			JournalHead: head,
			Parked:      parkedCount,
		})
	}

	printStats(byStatus, byProject, *project == "")

	if parkedCount > 0 {
		fmt.Printf("parked       %6d (rate-limit requeues waiting out their window)\n", parkedCount)
	}

	printBudgetSpend(spent, *dailyBudget, *project != "")
	printConsumerLag(store)

	return nil
}

// statsPayload is the --json shape of `tq stats`: the aggregates a script or
// dashboard consumes, never the raw task list (that is `tq tasks --json`).
type statsPayload struct {
	ByStatus    map[string]int            `json:"by_status"`
	ByProject   map[string]map[string]int `json:"by_project,omitempty"`
	Budget      budgetView                `json:"budget"`
	Lag         []consumerLagEntry        `json:"consumer_lag,omitempty"`
	JournalHead int64                     `json:"journal_head"`
	Parked      int                       `json:"parked,omitempty"`
}

type budgetView struct {
	SpentToday int `json:"spent_today"`
	Cap        int `json:"cap,omitempty"`
}

type consumerLagEntry struct {
	Consumer string `json:"consumer"`
	Seq      int64  `json:"seq"`
	Lag      int64  `json:"lag"`
}

// consumerLag collects the persisted journal-consumer cursors with their lag
// behind the head (ADR-0009'store observability surface) for the JSON payload.
func consumerLag(ctx context.Context, store *sqlite.Store) []consumerLagEntry {
	entries, err := store.ListWatermarks(ctx)
	if err != nil || len(entries) == 0 {
		return nil
	}

	head, err := store.HeadSeq(ctx)
	if err != nil {
		return nil
	}

	out := make([]consumerLagEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, consumerLagEntry{Consumer: e.Consumer, Seq: e.Seq, Lag: max(head-e.Seq, 0)})
	}

	return out
}

// printBudgetSpend surfaces the daily-budget projection in the CLI (the web
// UI has a budget card; the text output had nothing). Spent counts today'store
// task.enqueued facts — the same projection the pool'store budget guard uses.
// The count is always journal-wide, so a scoped table labels the line to
// keep the numbers honest.
func printBudgetSpend(spent, cap int, scoped bool) {
	label := "budget today"
	if scoped {
		label = "budget today (all projects)"
	}

	if cap > 0 {
		fmt.Printf("\n%s  %d/%d enqueued\n", label, spent, cap)

		return
	}

	fmt.Printf("\n%s  %d enqueued (pass --daily-budget N to compare against a cap)\n", label, spent)
}

// printConsumerLag renders the persisted journal-consumer cursors with
// their lag behind the head (ADR-0009's observability surface) — the first
// place to look when a bridge or sweeper looks quiet.
func printConsumerLag(store *sqlite.Store) {
	ctx := context.Background()

	entries, err := store.ListWatermarks(ctx)
	if err != nil || len(entries) == 0 {
		return
	}

	head, err := store.HeadSeq(ctx)
	if err != nil {
		return
	}

	fmt.Println("\nconsumer lag (journal head:", head, ")")

	for _, e := range entries {
		fmt.Printf("  %-52s seq %-8d lag %d\n", e.Consumer, e.Seq, max(head-e.Seq, 0))
	}
}

// tallyStats aggregates the task list into status counts and
// per-project-per-status counts.
func tallyStats(tasks []task.Task) (map[string]int, map[string]map[string]int) {
	byStatus := map[string]int{}
	byProject := map[string]map[string]int{}

	for _, t := range tasks {
		byStatus[string(t.Status)]++
		if byProject[t.Project] == nil {
			byProject[t.Project] = map[string]int{}
		}

		byProject[t.Project][string(t.Status)]++
	}

	return byStatus, byProject
}

// printStats renders the status table and, when scoped (project filter
// empty), the per-project breakdown.
func printStats(byStatus map[string]int, byProject map[string]map[string]int, scoped bool) {
	fmt.Printf("%-12s %6s\n", "STATUS", "COUNT")

	for _, st := range []string{"pending", "running", "completed", "dead", "cancelled"} {
		if c, ok := byStatus[st]; ok {
			fmt.Printf("%-12s %6d\n", st, c)
		}
	}

	if !scoped || len(byProject) == 0 {
		return
	}

	fmt.Println()
	fmt.Printf("%-28s %8s %8s %8s %8s %8s\n", "PROJECT", "pending", "running", "done", "dead", "cancld")

	projects := make([]string, 0, len(byProject))
	for p := range byProject {
		projects = append(projects, p)
	}

	sort.Strings(projects)

	for _, p := range projects {
		m := byProject[p]
		fmt.Printf("%-28s %8d %8d %8d %8d %8d\n", p,
			m["pending"], m["running"], m["completed"], m["dead"], m["cancelled"])
	}
}

func cmdShow(args []string) error {
	fs := flag.NewFlagSet("show", flag.ExitOnError)
	commits := fs.Bool(
		"commits",
		false,
		"also scan the task's repo git log for Task-Queue-ID footer commits (0 = missing footer, >1 = ambiguous cross-reference)",
	)

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return errors.New("usage: tq show TASK_ID (a unique prefix works)")
	}

	store := mustOpenDB(resolveDB(*db))
	defer store.Close()

	ctx := context.Background()

	t, err := resolveTask(ctx, store, fs.Arg(0))
	if err != nil {
		return err
	}
	// Include the task'store fact trail: for completed agent tasks this is
	// where the structured result detail lives (session id, verify tail).
	id := t.ID.String()

	trail, err := store.FactsForTask(ctx, id, 0)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	var commitView any

	if *commits {
		cv, err := commitsForTask(t)
		if err != nil {
			return err
		}

		commitView = cv
	}

	return enc.Encode(struct {
		Task    task.Task      `json:"task"`
		Facts   []journal.Fact `json:"facts,omitempty"`
		Result  any            `json:"result,omitempty"`
		Commits any            `json:"commits,omitempty"`
	}{t, trail, resultDetail(t, trail), commitView})
}

// commitHit is one git commit carrying the task's Task-Queue-ID footer.
type commitHit struct {
	SHA     string `json:"sha"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Subject string `json:"subject"`
}

// commitsForTask scans the task's repo git log for footer commits (the
// queue↔git cross-reference): count 0 means the footer contract was
// breached (work landed unreferenced), count >1 means an ambiguous
// cross-reference (the f26 three-ID cluster class).
func commitsForTask(t task.Task) (map[string]any, error) {
	repo := struct {
		Repo string `json:"repo"`
	}{}

	if err := json.Unmarshal(t.Payload, &repo); err != nil || repo.Repo == "" {
		return map[string]any{"note": "no repo in payload — footer scan unavailable"}, nil
	}

	if _, err := os.Stat(filepath.Join(repo.Repo, ".git")); err != nil {
		return map[string]any{"note": "repo not accessible: " + repo.Repo}, nil
	}

	cmd := exec.Command("git", "-C", repo.Repo, "log",
		"--pretty=format:%H%x09%an%x09%aI%x09%s", "--grep", "Task-Queue-ID: "+t.ID.String())

	out, err := cmd.Output()
	if err != nil {
		return map[string]any{"note": "git log failed: " + err.Error()}, nil
	}

	var hits []commitHit

	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "\t", 4)
		for len(parts) < 4 {
			parts = append(parts, "")
		}

		hits = append(hits, commitHit{SHA: parts[0], Author: parts[1], Date: parts[2], Subject: parts[3]})
	}

	verdict := "ok: exactly one footer commit"

	switch {
	case len(hits) == 0:
		verdict = "MISSING FOOTER: no commit references this task ID"
	case len(hits) > 1:
		verdict = "AMBIGUOUS: multiple commits reference this task ID"
	}

	return map[string]any{"task_id": t.ID.String(), "count": len(hits), "verdict": verdict, "commits": hits}, nil
}

// resolveTask looks a task up by its full ID, falling back to a UNIQUE
// prefix: 34-char IDs are hostile to hand-typing, and every tq ID is a
// ULID (time-ordered, so prefixes stay unambiguous in practice). An
// ambiguous prefix names its candidates instead of guessing.
func resolveTask(ctx context.Context, store *sqlite.Store, arg string) (task.Task, error) {
	t, err := store.Get(ctx, task.ID(arg))
	if err == nil {
		return t, nil
	}

	if !errors.Is(err, task.ErrNotFound) {
		return task.Task{}, err
	}

	tasks, err := store.List(ctx, queue.Filter{})
	if err != nil {
		return task.Task{}, err
	}

	var candidates []task.Task

	for _, cand := range tasks {
		if strings.HasPrefix(cand.ID.String(), arg) {
			candidates = append(candidates, cand)
		}
	}

	switch len(candidates) {
	case 1:
		return candidates[0], nil
	case 0:
		return task.Task{}, fmt.Errorf("no task with id or prefix %q", arg)
	default:
		ids := make([]string, 0, len(candidates))
		for _, cand := range candidates {
			ids = append(ids, cand.ID.String())
		}

		return task.Task{}, fmt.Errorf("prefix %q matches %d tasks — be more specific: %s",
			arg, len(candidates), strings.Join(ids, ", "))
	}
}

// resultDetail decodes a task'store completion-fact detail into its typed result
// (agent self-report, review verdict, or status outcome) so `tq show` answers
// "what did the agent actually do" without eyeballing raw JSON. nil for task
// types without a structured result — the raw facts stay in the output.
func resultDetail(t task.Task, trail []journal.Fact) any {
	for _, first := range slices.Backward(trail) {
		if first.Type != journal.Completed || len(first.Detail) == 0 {
			continue
		}

		switch t.Type {
		case executor.TaskTypeAgent:
			var res executor.AgentResult
			if json.Unmarshal(first.Detail, &res) == nil {
				return res
			}
		case executor.TaskTypeReview:
			var res executor.ReviewResult
			if json.Unmarshal(first.Detail, &res) == nil {
				return res
			}
		case executor.TaskTypeStatus:
			var res executor.StatusResult
			if json.Unmarshal(first.Detail, &res) == nil {
				return res
			}
		}

		return nil
	}

	return nil
}

func cmdDLQ(args []string) error {
	fs := flag.NewFlagSet("dlq", flag.ExitOnError)
	rescue := fs.String("rescue", "", "re-queue this dead task ID")
	rescueAll := fs.Bool("rescue-all", false, "re-queue EVERY dead task (only after a human decided they can succeed)")
	olderThan := fs.Duration("older-than", 0, "with --rescue-all: only tasks dead for at least this long (e.g. 24h)")
	maxAttempts := fs.Int("max-attempts", 3, "attempt budget for rescued task(s)")
	dismiss := fs.String(
		"dismiss",
		"",
		"cancel this dead task ID with a recorded reason (the same disposition the DLQ-autopsy sweeper makes on a wontfix verdict)",
	)
	reason := fs.String("reason", "", "why the task is dismissed; stored in the cancelled fact detail")

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	store := mustOpenDB(resolveDB(*db))
	defer store.Close()

	if *rescueAll {
		_, err := rescueAllDead(context.Background(), store, *olderThan, *maxAttempts)

		return err
	}

	if *rescue != "" {
		if err := store.RescueDead(context.Background(), task.ID(*rescue), *maxAttempts); err != nil {
			return err
		}

		fmt.Printf("rescued %s\n", *rescue)

		return nil
	}

	if *dismiss != "" {
		if err := store.DismissDead(context.Background(), task.ID(*dismiss), strings.TrimSpace(*reason), "operator"); err != nil {
			return err
		}

		fmt.Printf("dismissed %s  (reason recorded on the cancelled fact)\n", *dismiss)

		return nil
	}

	tasks, err := listDead(context.Background(), store)
	if err != nil {
		return err
	}

	printDLQ(tasks)

	return nil
}

// rescueAllDead re-queues every dead task older than olderThan (all if 0).
func rescueAllDead(ctx context.Context, store *sqlite.Store, olderThan time.Duration, maxAttempts int) (int, error) {
	dead, err := listDead(ctx, store)
	if err != nil {
		return 0, err
	}

	rescued := 0

	for _, t := range dead {
		if olderThan > 0 && time.Since(t.UpdatedAt) < olderThan {
			continue
		}

		if err := store.RescueDead(ctx, t.ID, maxAttempts); err != nil {
			return rescued, fmt.Errorf("rescue %s: %w", t.ID, err)
		}

		fmt.Printf("rescued %s  %s\n", t.ID, t.Project+"/"+t.Type)

		rescued++
	}

	fmt.Printf("rescued %d of %d dead task(s)\n", rescued, len(dead))

	return rescued, nil
}

func listDead(ctx context.Context, s *sqlite.Store) ([]task.Task, error) {
	st := task.Dead

	return s.List(ctx, queue.Filter{Status: &st})
}

func printDLQ(tasks []task.Task) {
	if len(tasks) == 0 {
		fmt.Println("(empty)")

		return
	}

	for _, t := range tasks {
		fmt.Printf("%s  %-24s attempts=%d/%d  %s\n",
			t.ID, t.Project+"/"+t.Type, t.Attempts, t.MaxAttempts, truncate(t.LastError, 80))
	}
}

// partitionFlags moves flag tokens ahead of positional arguments:
// flag.Parse stops at the first positional, so the documented
// `tq cancel <task-id> --reason why` order needs its flags hoisted to
// parse. valued names the flags that consume the following token;
// `--flag=value` forms need no lookahead.
func partitionFlags(args []string, valued map[string]bool) []string {
	var flags, positional []string

	for i := 0; i < len(args); i++ {
		a := args[i]
		if len(a) > 1 && a[0] == '-' {
			flags = append(flags, a)

			name := strings.TrimLeft(a, "-")
			if !strings.Contains(name, "=") && valued[name] && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}

			continue
		}

		positional = append(positional, a)
	}

	return append(flags, positional...)
}

func cmdCancel(args []string) error {
	fs := flag.NewFlagSet("cancel", flag.ExitOnError)

	force := fs.Bool(
		"force",
		false,
		"running tasks: request a cooperative cancel (observed at the worker's next heartbeat)",
	)

	reason := fs.String(
		"reason",
		"",
		"why the task is cancelled; stored in the cancel fact detail for forensics",
	)

	db := dbFlag(fs)
	if err := fs.Parse(partitionFlags(args, map[string]bool{"reason": true, "db": true})); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return errors.New("usage: tq cancel TASK_ID [--force] [--reason WHY]")
	}

	store := mustOpenDB(resolveDB(*db))
	defer store.Close()

	ctx := context.Background()

	t, err := resolveTask(ctx, store, fs.Arg(0))
	if err != nil {
		return err
	}

	taskID := t.ID

	switch t.Status {
	case task.Pending:
		return store.Cancel(ctx, taskID, *reason)
	case task.Running:
		if !*force {
			return fmt.Errorf(
				"task %s is running; pass --force to request a cooperative cancel (the worker stops it at its next heartbeat)",
				taskID,
			)
		}

		if err := store.CancelRunning(ctx, taskID, *reason); err != nil {
			return err
		}

		fmt.Println("cancel requested; the executing worker will stop the task at its next heartbeat")

		return nil
	default:
		return fmt.Errorf("task %s is %s (terminal); nothing to cancel", taskID, t.Status)
	}
}

func cmdFacts(args []string) error {
	fs := flag.NewFlagSet("facts", flag.ExitOnError)
	after := fs.Int64("after", 0, "only facts with seq > this")
	asJSON := fs.Bool("json", false, "JSON output of the fact list (full detail, non-truncating)")
	asCQRS := fs.Bool(
		"cqrs",
		false,
		"render the facts as go-cqrs-lite journal events (JSON array; see internal/journal/cqrs)",
	)
	withDetail := fs.Bool(
		"detail",
		false,
		"print each fact's full detail JSON verbatim below its line (multi-line tails stay intact)",
	)

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	s := mustOpenDB(resolveDB(*db))
	defer s.Close()

	facts, err := s.Facts(context.Background(), *after, 0)
	if err != nil {
		return err
	}

	if *asCQRS {
		return printCQRSEvents(facts)
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")

		return enc.Encode(facts)
	}

	for _, f := range facts {
		fmt.Println(formatFact(f))

		if *withDetail {
			fmt.Println(formatFactDetail(f))
		}
	}

	fmt.Printf("(%d facts)\n", len(facts))

	return nil
}

// cqrsEventJSON is the wire shape of one go-cqrs-lite journal event as
// rendered by `tq facts --cqrs` (the payload stays raw fact JSON).
type cqrsEventJSON struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	StreamID   string         `json:"streamId"`
	StreamType string         `json:"streamType"`
	Version    uint64         `json:"version"`
	OccurredAt time.Time      `json:"occurredAt"`
	Payload    jsontext.Value `json:"payload"`
}

func printCQRSEvents(facts []journal.Fact) error {
	events, err := cqrs.NewFactJournal(cqrs.NewSliceSource(facts)).ReadAll(context.Background())
	if err != nil {
		return err
	}

	out := make([]cqrsEventJSON, 0, len(events))

	for _, evt := range events {
		out = append(out, cqrsEventJSON{
			ID:         evt.ID().String(),
			Type:       string(evt.Type()),
			StreamID:   evt.StreamID().String(),
			StreamType: string(evt.StreamType()),
			Version:    uint64(evt.Version()),
			OccurredAt: evt.OccurredAt(),
			Payload:    jsontext.Value(evt.Payload()),
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	return enc.Encode(out)
}

// formatFactDetail renders a fact's detail JSON verbatim and non-truncated
// (multi-line verify tails stay intact), aligned under its fact line.
func formatFactDetail(f journal.Fact) string {
	if len(f.Detail) == 0 {
		return "       detail: (none)"
	}

	return "       detail: " + string(f.Detail)
}

// formatFact renders one journal fact for humans. Dead-letter facts carry
// their error class ("permanent" vs "exhausted") in Detail; show it so an
// operator can tell "the task is broken" from "the budget ran out".
func formatFact(fact journal.Fact) string {
	line := fmt.Sprintf("%5d %s %s %-20s %s %s",
		fact.Seq, fact.Time.Format(time.RFC3339), fact.TaskID, fact.Type, fact.Owner, fact.Error)

	var d struct {
		Class string `json:"class"`
	}
	if len(fact.Detail) > 0 && json.Unmarshal(fact.Detail, &d) == nil && d.Class != "" {
		line += " [class=" + d.Class + "]"
	}

	return line
}

// cmdWatermarks administers persisted journal-consumer cursors. `show`
// lists every consumer's checkpoint with its lag behind the journal head;
// `set` rewrites one cursor — a rewind forces replay, and the seq-derived
// idempotency keys downstream make replay safe (the bridge re-sends
// bit-identical requests).
func cmdWatermarks(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: tq watermarks show | set CONSUMER SEQ")
	}

	switch args[0] {
	case "show":
		fs := flag.NewFlagSet("watermarks show", flag.ExitOnError)

		db := dbFlag(fs)
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		store := mustOpenDB(resolveDB(*db))
		defer store.Close()

		entries, err := store.ListWatermarks(context.Background())
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			fmt.Println("(no watermarks)")

			return nil
		}

		head, err := store.HeadSeq(context.Background())
		if err != nil {
			return err
		}

		for _, entry := range entries {
			lag := max(head-entry.Seq, 0)

			state := fmt.Sprintf("lag %-6d", lag)
			if lag == 0 {
				state = "current "
			}

			fmt.Printf("%-52s %8d  %s  updated %s\n",
				entry.Consumer, entry.Seq, state, time.UnixMilli(entry.UpdatedAt).Format(time.RFC3339))
		}

		// A lagging cursor is ambiguous by design: cursors only advance
		// while their consumer'store process runs, so "lagging" may just mean
		// "off" (entry.g. a status loop disabled via --status-every 0).
		fmt.Println("(a lagging consumer may simply be off — cursors only advance while their process runs)")

		return nil

	case "set":
		fs := flag.NewFlagSet("watermarks set", flag.ExitOnError)

		db := dbFlag(fs)
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}

		rest := fs.Args()
		if len(rest) != 2 {
			return errors.New("usage: tq watermarks set CONSUMER SEQ [--db PATH]")
		}

		seq, err := strconv.ParseInt(rest[1], 10, 64)
		if err != nil || seq < 0 {
			return fmt.Errorf("invalid seq %q: must be a non-negative integer", rest[1])
		}

		store := mustOpenDB(resolveDB(*db))
		defer store.Close()

		if err := store.SetWatermark(context.Background(), rest[0], seq); err != nil {
			return err
		}

		fmt.Printf(
			"watermark %s -> %d (replays facts after this seq on the next consumer start; re-sends are idempotent)\n",
			rest[0],
			seq,
		)

		return nil

	default:
		return fmt.Errorf("unknown watermarks subcommand %q (show | set)", args[0])
	}
}

func cmdTail(args []string) error {
	fs := flag.NewFlagSet("tail", flag.ExitOnError)
	after := fs.Int64("after", 0, "only facts with seq > this")
	follow := fs.Bool("f", false, "follow (live)")

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	store := mustOpenDB(resolveDB(*db))
	defer store.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	for {
		facts, err := store.Facts(ctx, *after, 0)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}

			return err
		}

		for _, f := range facts {
			fmt.Println(formatFact(f))
			*after = f.Seq
		}

		if !*follow {
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}

	return s[:n] + "…"
}

// cmdAPI runs the production write API (ADR-0008): POST /api/v1/tasks,
// GET /api/v1/stats, token-gated. Unlike `serve` the token is REQUIRED
// (this surface exists to be exposed to other machines).
func cmdAPI(args []string) error {
	fs := flag.NewFlagSet("api", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8091", "listen address")
	authToken := fs.String("auth-token", os.Getenv("TQ_API_TOKEN"),
		"REQUIRED bearer token for every request (env $TQ_API_TOKEN)")

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	s := mustOpenDB(resolveDB(*db))
	defer s.Close()

	server, err := httpapi.New(s, *authToken, nil)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Fprintf(os.Stderr, "tq: write API on http://%s (token required)\n", *addr)

	return server.ListenAndServe(ctx, *addr)
}

// version is overridden at build time (-ldflags "-X main.version=...");
// "dev" marks an untagged go-build checkout, where debug.ReadBuildInfo
// still reports the VCS revision.
var version = "dev"

func cmdVersion(args []string) error {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Printf("tq %s (no build info)\n", version)

		return nil
	}

	rev := "(unknown)"

	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && s.Value != "" {
			rev = s.Value
			if len(rev) > 12 {
				rev = rev[:12]
			}
		}
	}

	vcs := ""
	if rev != "(unknown)" {
		vcs = " (" + rev + ")"
	}

	fmt.Printf("tq %s%s, %s\n", version, vcs, info.GoVersion)

	return nil
}

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", webui.DefaultAddr, "listen address (default: localhost only)")
	poll := fs.Duration("poll", webui.DefaultPoll, "journal tail interval")
	verbose := fs.Bool("verbose", false, "log every HTTP request (method, path, status, duration) to stderr")
	authToken := fs.String(
		"auth-token",
		os.Getenv("TQ_SERVE_TOKEN"),
		"require this token on every request (Authorization: Bearer or ?token=); required for non-loopback --addr (env $TQ_SERVE_TOKEN)",
	)
	allowWrites := fs.Bool(
		"allow-writes",
		os.Getenv("TQ_SERVE_WRITES") == "1",
		"enable admin actions in the dashboard (cancel pending/running, rescue dead; env $TQ_SERVE_WRITES=1); CSRF-guarded, and non-loopback binds still require --auth-token",
	)

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg := webui.Config{
		Addr:        *addr,
		Poll:        *poll,
		RequestLog:  *verbose,
		AuthToken:   *authToken,
		AllowWrites: *allowWrites,
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	store := mustOpenDB(resolveDB(*db))

	// One signal story (runactor): the interrupt actor cancels the http
	// actor, teardown closes the store after the server has fully stopped.
	g := runactor.New(context.Background())
	g.InterruptOn(os.Interrupt, syscall.SIGTERM)
	g.OnShutdown(func() error { return store.Close() })

	server := webui.New(store, cfg)

	g.Go("http", func(ctx context.Context) error { return server.Run(ctx) })

	if *allowWrites {
		fmt.Fprintf(os.Stderr, "%s%s%s\n", webui.BannerPrefix, *addr, webui.BannerWritesSuffix)
	} else {
		fmt.Fprintf(os.Stderr, "%s%s%s\n", webui.BannerPrefix, *addr, webui.BannerReadOnlySuffix)
	}

	return g.Run()
}
