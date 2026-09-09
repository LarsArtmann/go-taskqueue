// Command tq is the CLI for go-taskqueue: enqueue work, run workers, inspect
// the queue, and replay the journal.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
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
	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/harvest"
	"github.com/larsartmann/go-taskqueue/internal/httpapi"
	"github.com/larsartmann/go-taskqueue/internal/journal"
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
  tq dlq [--db PATH] [--rescue TASK_ID [--max-attempts N]]
tq cancel TASK_ID [--force] [--reason WHY] [--db PATH]   (--force: cooperative cancel of a running task)
  tq facts [--db PATH] [--after SEQ]
  tq tail [-f] [--db PATH] [--after SEQ]
  tq watermarks show [--db PATH]   (journal consumer cursors)
  tq watermarks set CONSUMER SEQ [--db PATH]   (rewind = safe replay)
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
		"bootstrap":  cmdBootstrap,
		"enqueue":    cmdEnqueue,
		"worker":     cmdWorker,
		"harvest":    cmdHarvest,
		"agent-pool": cmdAgentPool,
		"stats":      cmdStats,
		"tasks":      cmdTasks,
		"audit":      cmdAudit,
		"doctor":     cmdDoctor,
		"top":        cmdTop,
		"show":       cmdShow,
		"dlq":        cmdDLQ,
		"cancel":     cmdCancel,
		"facts":      cmdFacts,
		"tail":       cmdTail,
		"watermarks": cmdWatermarks,
		"serve":      cmdServe,
		"version":    cmdVersion,
		"api":        cmdAPI,
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
	s, err := sqlite.Open(path, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tq: open db: %v\n", err)
		os.Exit(1)
	}

	return s
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

	n := task.New{
		Project:     *project,
		Type:        *taskType,
		Payload:     payloadJSON,
		Priority:    *priority,
		MaxAttempts: *maxAttempts,
		NotBefore:   time.Now().Add(*delay),
	}

	for d := range strings.SplitSeq(*deps, ",") {
		if d = strings.TrimSpace(d); d != "" {
			n.Deps = append(n.Deps, task.ID(d))
		}
	}

	s := mustOpenDB(resolveDB(*db))
	defer s.Close()

	t, err := queue.New(s).Enqueue(context.Background(), n)
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

	s := mustOpenDBOpts(resolveDB(*db), opts...)

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

	pool := worker.New(s, worker.Config{
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
	g.OnShutdown(func() error { return s.Close() })

	if *alertURL != "" {
		bridge := papdashboard.New(s, s, papdashboard.Config{
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
			q := queue.New(s)

			for {
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(*poll):
					if pool.InFlight() == 0 && !hasClaimableWork(ctx, q, pool.Owner(), *poll) {
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

	if *projectsDir == "" && *repos == "" {
		return errors.New("no repos: pass --repos or --projects-dir (or set $TQ_PROJECTS_DIR)")
	}

	if err := checkProjectsDir(*projectsDir); err != nil {
		return err
	}

	cfg := harvest.Config{
		ProjectsDir:   *projectsDir,
		DiscoveryAddr: *discoveryAddr,
		Log:           slog.Default(),
		TodoFile:      *todoFile,
		Type:          *taskType,
		MaxPerTick:    *maxPerTick,
		Priority:      *priority,
		MaxAttempts:   *maxAttempts,
		Model:         *model,
		DryRun:        *dryRun,
	}

	if *allowDirty {
		no := false
		cfg.RequireClean = &no
	}

	// --repos overrides --projects-dir; --repo-subset filters the discovered set.
	if err := resolveHarvestRepos(&cfg, *projectsDir, *repos, *repoSubset); err != nil {
		return err
	}

	s := mustOpenDB(resolveDB(*db))
	defer s.Close()

	if *pruneStale {
		pruned, err := harvest.New(queue.New(s), cfg).PruneStale(context.Background())
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

	res, err := harvest.New(queue.New(s), cfg).Run(context.Background())
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
		cfg.Repos = splitRepos(repos)

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
	for _, en := range res.Enqueued {
		id := en.TaskID.String()
		if dryRun {
			id = "(dry-run)"
		}

		fmt.Printf("ENQUEUED  %-24s %s  %s\n", en.Item.RepoName, en.Item.Text, id)
	}

	for _, sk := range res.Skipped {
		fmt.Printf("SKIP      %-24s %s  — %s\n", sk.Item.RepoName, sk.Item.Text, sk.Reason)
	}
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
	o, err := parseAgentPoolOptions(args)
	if err != nil {
		return err
	}

	cfg, err := harvestConfigFromOptions(o)
	if err != nil {
		return err
	}

	var opts []sqlite.StoreOption
	if o.exclusive {
		opts = append(opts, sqlite.WithProjectExclusivity())
	}

	s := mustOpenDBOpts(resolveDB(o.db), opts...)
	defer s.Close()

	q := queue.New(s)

	agentExec := &executor.AgentExecutor{ProjectsDir: o.projectsDir, Yolo: o.yolo, MaxConcurrent: o.maxAgents}

	reg := executor.NewRegistry()
	reg.Register("sh", executor.NewCommandExecutor(""))
	registerAgentExecutors(reg, agentExec)

	printAgentPoolBanner(o)

	// One signal story (runactor): interrupt cancels the pool loop, the
	// tick actor and the bridge; in-flight agent tasks finish under their
	// own execution scope (bounded only by --task-timeout); teardown closes
	// the store AFTER everything stopped, so bridge checkpoints always land.
	g := runactor.New(context.Background())
	g.InterruptOn(os.Interrupt, syscall.SIGTERM)
	g.OnShutdown(func() error { return s.Close() })

	ctx := g.Ctx()

	if o.alertURL != "" {
		bridge := papdashboard.New(s, s, papdashboard.Config{
			Endpoint:     o.alertURL,
			APIKey:       o.alertKey,
			PollInterval: o.alertPoll,
			// Mirror the pool's cap so the day it bites, an alert fires (and
			// resolves itself when the window rolls over).
			DailyBudget: o.dailyBudget,
		})

		g.Go("alert-bridge", func(ctx context.Context) error { return bridge.Run(ctx) })

		fmt.Fprintf(os.Stderr, "tq: agent-pool: forwarding dead letters + budget exhaustion to %s\n", o.alertURL)
	}

	log := slog.Default()
	cfg.Log = log
	guard := budget.Guard{DailyCap: o.dailyBudget, BudgetCmd: o.budgetCmd}
	h := harvest.New(q, cfg)

	// Startup zombie sweep, SYNCHRONOUSLY before any actor starts: the
	// worker's first claim would otherwise race the sweep and turn
	// cancellable zombies into running tasks (observed in the e2e). One
	// pass, then never again; --prune-stale=false disables it for operators
	// who want relaunches to inherit everything.
	if o.pruneStale {
		res, err := h.PruneStale(ctx)
		if err != nil {
			log.Warn("startup prune-stale failed", "err", err)
		} else {
			for _, c := range res.Cancelled {
				log.Warn(
					"startup prune: cancelled stale task",
					"repo",
					c.Item.RepoName,
					"why",
					c.Why,
					"item",
					pruneItemText(c.Item, c.Why),
					"task",
					c.TaskID.String(),
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

	var cqaBridge *cqa.Bridge
	if o.cqaURL != "" {
		cqaBridge = cqa.New(cqa.Config{
			BaseURL:     o.cqaURL,
			Token:       o.cqaToken,
			OwnerID:     o.cqaOwner,
			ProjectsDir: o.projectsDir,
		})
	}

	var sweeper *review.Sweeper

	if o.doReview {
		var err error

		sweeper, err = review.NewSweeper(ctx, s, review.SweeperConfig{
			Model:   o.model,
			Autofix: o.reviewAutofix,
			Log:     log,
		})
		if err != nil {
			return fmt.Errorf("review sweeper: %w", err)
		}
	}

	var statusSweeper *status.Sweeper

	if o.statusEvery > 0 {
		var err error

		statusSweeper, err = status.NewSweeper(ctx, s, status.SweeperConfig{
			Every:       o.statusEvery,
			Model:       o.model,
			Log:         log,
			AllowDirty:  o.allowDirty,
			TaskTimeout: o.timeout,
		})
		if err != nil {
			return fmt.Errorf("status sweeper: %w", err)
		}
	}

	// mintPass gates one budget-consuming pass: EVERY enqueue (harvested,
	// review, status, CQA) must clear the guard immediately before it — a
	// completion inside the same tick can spend the last slot after an
	// earlier check passed, and the documented cap is "EVERY enqueue"
	// (SECURITY.md). Returns false when the guard refused.
	mintPass := func(name string, mint func()) bool {
		if ok, reason := guard.Check(ctx, s); !ok {
			log.Warn("budget: skipping "+name, "reason", reason)

			return false
		}

		mint()

		return true
	}

	runTick := func() {
		// Sidecar retention: sweep aged logs before new work so a
		// long-running pool's output directory cannot grow forever.
		if o.logDir != "" && o.logDirMaxAge > 0 {
			if removed, err := executor.SweepSidecars(o.logDir, o.logDirMaxAge); err != nil {
				log.Warn("sidecar sweep failed", "err", err)
			} else if removed > 0 {
				log.Info("sidecar sweep", "removed", removed, "dir", o.logDir)
			}
		}

		// Byte-budget retention: age alone cannot bound a high-traffic dir.
		if o.logDir != "" && o.logDirMaxBytes > 0 {
			if removed, err := executor.SweepSidecarsByBytes(o.logDir, o.logDirMaxBytes); err != nil {
				log.Warn("sidecar byte sweep failed", "err", err)
			} else if removed > 0 {
				log.Info("sidecar byte sweep", "removed", removed, "dir", o.logDir)
			}
		}

		if !mintPass("harvest tick", func() {
			res, err := h.Run(ctx)
			if err != nil {
				log.Error("harvest failed", "err", err)
			} else {
				for _, en := range res.Enqueued {
					log.Info(
						"harvest: enqueued",
						"repo",
						en.Item.RepoName,
						"item",
						en.Item.Text,
						"task",
						en.TaskID.String(),
					)
				}

				for class, n := range groupedSkips(res.Skipped) {
					log.Info("harvest: skipped", "reason", class, "count", n)
				}

				log.Info("harvest tick done", "repos", res.Repos, "items", res.Items,
					"enqueued", len(res.Enqueued), "skipped", len(res.Skipped))
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

			for _, ft := range fixTasks {
				got, err := q.Enqueue(ctx, ft.Template)
				if err != nil {
					log.Error("cqa enqueue failed", "key", ft.Template.DedupKey, "err", err)

					continue
				}

				if got.Attempts == 0 && got.Status == task.Pending {
					fresh++

					log.Info(
						"cqa: enqueued fix task",
						"repo",
						ft.Project,
						"file",
						ft.File,
						"issues",
						len(ft.Issues),
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
	if o.discoveryAddr != "" && !o.once {
		watcher := harvest.NewWatcher(harvest.WatchConfig{
			Addr:          o.discoveryAddr,
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
			o.discoveryAddr,
			o.interval,
		)
	}

	g.Go("tick", func(ctx context.Context) error {
		runTick()

		if o.once {
			// The once-drain actor decides when the group ends; this actor
			// must not return before that (a clean return would end it).
			<-ctx.Done()

			return nil
		}

		ticker := time.NewTicker(o.interval)
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

	pool := worker.New(s, worker.Config{
		Owner:        o.owner,
		Concurrency:  o.conc,
		PollInterval: o.poll,
		Lease:        o.lease,
		TaskTimeout:  o.timeout,
		Executors:    reg,
	}, log)

	if o.once {
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
				case <-time.After(o.poll):
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

					if pool.InFlight() == 0 && !hasClaimableWork(ctx, q, pool.Owner(), o.poll) {
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
func groupedSkips(skips []harvest.Skipped) map[string]int {
	groups := make(map[string]int)

	for _, sk := range skips {
		class := sk.Reason
		if before, _, ok := strings.Cut(sk.Reason, ":"); ok {
			class = before
		}

		groups[class]++
	}

	return groups
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
		"agent pool daily enqueue cap to compare today's spend against (0 = spend shown without a cap)",
	)
	asJSON := fs.Bool("json", false, "JSON output of the stats aggregate (counts, budget, consumer lag)")

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	s := mustOpenDB(resolveDB(*db))
	defer s.Close()

	ctx := context.Background()

	f := queue.Filter{}
	if *project != "" {
		f.Project = project
	}

	if *status != "" {
		st := task.Status(*status)
		f.Status = &st
	}

	tasks, err := s.List(ctx, f)
	if err != nil {
		return err
	}

	head, err := s.HeadSeq(ctx)
	if err != nil {
		return err
	}

	byStatus, byProject := tallyStats(tasks)
	spent := budget.Guard{DailyCap: *dailyBudget}.SpentToday(ctx, s)

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")

		return enc.Encode(statsPayload{
			ByStatus:    byStatus,
			ByProject:   byProject,
			Budget:      budgetView{SpentToday: spent, Cap: *dailyBudget},
			Lag:         consumerLag(ctx, s),
			JournalHead: head,
		})
	}

	printStats(byStatus, byProject, *project == "")
	printBudgetSpend(spent, *dailyBudget, *project != "")
	printConsumerLag(s)

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
// behind the head (ADR-0009's observability surface) for the JSON payload.
func consumerLag(ctx context.Context, s *sqlite.Store) []consumerLagEntry {
	entries, err := s.ListWatermarks(ctx)
	if err != nil || len(entries) == 0 {
		return nil
	}

	head, err := s.HeadSeq(ctx)
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
// UI has a budget card; the text output had nothing). Spent counts today's
// task.enqueued facts — the same projection the pool's budget guard uses.
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
func printConsumerLag(s *sqlite.Store) {
	ctx := context.Background()

	entries, err := s.ListWatermarks(ctx)
	if err != nil || len(entries) == 0 {
		return
	}

	head, err := s.HeadSeq(ctx)
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

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return errors.New("usage: tq show TASK_ID (a unique prefix works)")
	}

	s := mustOpenDB(resolveDB(*db))
	defer s.Close()

	ctx := context.Background()

	t, err := resolveTask(ctx, s, fs.Arg(0))
	if err != nil {
		return err
	}
	// Include the task's fact trail: for completed agent tasks this is
	// where the structured result detail lives (session id, verify tail).
	id := t.ID.String()

	trail, err := s.FactsForTask(ctx, id, 0)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	return enc.Encode(struct {
		Task   task.Task      `json:"task"`
		Facts  []journal.Fact `json:"facts,omitempty"`
		Result any            `json:"result,omitempty"`
	}{t, trail, resultDetail(t, trail)})
}

// resolveTask looks a task up by its full ID, falling back to a UNIQUE
// prefix: 34-char IDs are hostile to hand-typing, and every tq ID is a
// ULID (time-ordered, so prefixes stay unambiguous in practice). An
// ambiguous prefix names its candidates instead of guessing.
func resolveTask(ctx context.Context, s *sqlite.Store, arg string) (task.Task, error) {
	t, err := s.Get(ctx, task.ID(arg))
	if err == nil {
		return t, nil
	}

	if !errors.Is(err, task.ErrNotFound) {
		return task.Task{}, err
	}

	tasks, err := s.List(ctx, queue.Filter{})
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

// resultDetail decodes a task's completion-fact detail into its typed result
// (agent self-report, review verdict, or status outcome) so `tq show` answers
// "what did the agent actually do" without eyeballing raw JSON. nil for task
// types without a structured result — the raw facts stay in the output.
func resultDetail(t task.Task, trail []journal.Fact) any {
	for _, t0 := range slices.Backward(trail) {
		if t0.Type != journal.Completed || len(t0.Detail) == 0 {
			continue
		}

		switch t.Type {
		case executor.TaskTypeAgent:
			var res executor.AgentResult
			if json.Unmarshal(t0.Detail, &res) == nil {
				return res
			}
		case executor.TaskTypeReview:
			var res executor.ReviewResult
			if json.Unmarshal(t0.Detail, &res) == nil {
				return res
			}
		case executor.TaskTypeStatus:
			var res executor.StatusResult
			if json.Unmarshal(t0.Detail, &res) == nil {
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

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	s := mustOpenDB(resolveDB(*db))
	defer s.Close()

	if *rescueAll {
		_, err := rescueAllDead(context.Background(), s, *olderThan, *maxAttempts)

		return err
	}

	if *rescue != "" {
		if err := s.RescueDead(context.Background(), task.ID(*rescue), *maxAttempts); err != nil {
			return err
		}

		fmt.Printf("rescued %s\n", *rescue)

		return nil
	}

	tasks, err := listDead(context.Background(), s)
	if err != nil {
		return err
	}

	printDLQ(tasks)

	return nil
}

// rescueAllDead re-queues every dead task older than olderThan (all if 0).
func rescueAllDead(ctx context.Context, s *sqlite.Store, olderThan time.Duration, maxAttempts int) (int, error) {
	dead, err := listDead(ctx, s)
	if err != nil {
		return 0, err
	}

	rescued := 0

	for _, t := range dead {
		if olderThan > 0 && time.Since(t.UpdatedAt) < olderThan {
			continue
		}

		if err := s.RescueDead(ctx, t.ID, maxAttempts); err != nil {
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

	s := mustOpenDB(resolveDB(*db))
	defer s.Close()

	ctx := context.Background()

	t, err := resolveTask(ctx, s, fs.Arg(0))
	if err != nil {
		return err
	}

	id := t.ID

	switch t.Status {
	case task.Pending:
		return s.Cancel(ctx, id, *reason)
	case task.Running:
		if !*force {
			return fmt.Errorf(
				"task %s is running; pass --force to request a cooperative cancel (the worker stops it at its next heartbeat)",
				id,
			)
		}

		if err := s.CancelRunning(ctx, id, *reason); err != nil {
			return err
		}

		fmt.Println("cancel requested; the executing worker will stop the task at its next heartbeat")

		return nil
	default:
		return fmt.Errorf("task %s is %s (terminal); nothing to cancel", id, t.Status)
	}
}

func cmdFacts(args []string) error {
	fs := flag.NewFlagSet("facts", flag.ExitOnError)
	after := fs.Int64("after", 0, "only facts with seq > this")
	asJSON := fs.Bool("json", false, "JSON output of the fact list (full detail, non-truncating)")
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
func formatFact(f journal.Fact) string {
	line := fmt.Sprintf("%5d %s %s %-20s %s %s",
		f.Seq, f.Time.Format(time.RFC3339), f.TaskID, f.Type, f.Owner, f.Error)

	var d struct {
		Class string `json:"class"`
	}
	if len(f.Detail) > 0 && json.Unmarshal(f.Detail, &d) == nil && d.Class != "" {
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

		s := mustOpenDB(resolveDB(*db))
		defer s.Close()

		entries, err := s.ListWatermarks(context.Background())
		if err != nil {
			return err
		}

		if len(entries) == 0 {
			fmt.Println("(no watermarks)")

			return nil
		}

		head, err := s.HeadSeq(context.Background())
		if err != nil {
			return err
		}

		for _, e := range entries {
			lag := max(head-e.Seq, 0)

			state := fmt.Sprintf("lag %-6d", lag)
			if lag == 0 {
				state = "current "
			}

			fmt.Printf("%-52s %8d  %s  updated %s\n",
				e.Consumer, e.Seq, state, time.UnixMilli(e.UpdatedAt).Format(time.RFC3339))
		}

		// A lagging cursor is ambiguous by design: cursors only advance
		// while their consumer's process runs, so "lagging" may just mean
		// "off" (e.g. a status loop disabled via --status-every 0).
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

		s := mustOpenDB(resolveDB(*db))
		defer s.Close()

		if err := s.SetWatermark(context.Background(), rest[0], seq); err != nil {
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

	s := mustOpenDB(resolveDB(*db))
	defer s.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	for {
		facts, err := s.Facts(ctx, *after, 0)
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

	s := mustOpenDB(resolveDB(*db))

	// One signal story (runactor): the interrupt actor cancels the http
	// actor, teardown closes the store after the server has fully stopped.
	g := runactor.New(context.Background())
	g.InterruptOn(os.Interrupt, syscall.SIGTERM)
	g.OnShutdown(func() error { return s.Close() })

	server := webui.New(s, cfg)

	g.Go("http", func(ctx context.Context) error { return server.Run(ctx) })

	if *allowWrites {
		fmt.Fprintf(os.Stderr, "%s%s%s\n", webui.BannerPrefix, *addr, webui.BannerWritesSuffix)
	} else {
		fmt.Fprintf(os.Stderr, "%s%s%s\n", webui.BannerPrefix, *addr, webui.BannerReadOnlySuffix)
	}

	return g.Run()
}
