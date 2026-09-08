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
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/bridge/cqa"
	"github.com/larsartmann/go-taskqueue/internal/bridge/papdashboard"
	"github.com/larsartmann/go-taskqueue/internal/budget"
	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/harvest"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/review"
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
            [--dry-run] [--db PATH]
  tq agent-pool --projects-dir DIR [--repos a,b] [--interval DUR] [--concurrency N]
               [--yolo] [--max-per-tick N] [--task-timeout DUR]
               [--cqa-url URL [--cqa-owner ID] [--cqa-token T]] [--db PATH]
  tq stats [--project P] [--status S] [--db PATH] [--json]
  tq audit --projects-dir DIR [--repos a,b] [--todo-file F] [--type T]
          [--max-attempts N] [--dry-run] [--json] [--db PATH]
  tq doctor [--json] [--daily-budget N] [--repos a,b] [--db PATH]
  tq top [--interval DUR] [--once] [--json] [--db PATH]
  tq show TASK_ID [--db PATH]
  tq dlq [--db PATH] [--rescue TASK_ID [--max-attempts N]]
  tq cancel TASK_ID [--force] [--db PATH]   (--force: cooperative cancel of a running task)
  tq facts [--db PATH] [--after SEQ]
  tq tail [-f] [--db PATH] [--after SEQ]
  tq serve [--addr ADDR] [--auth-token TOKEN] [--db PATH] [--poll DUR] [--verbose]

Default database: $TQ_DB or ./tasks.db
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	commands := map[string]func([]string) error{
		"enqueue":    cmdEnqueue,
		"worker":     cmdWorker,
		"harvest":    cmdHarvest,
		"agent-pool": cmdAgentPool,
		"stats":      cmdStats,
		"audit":      cmdAudit,
		"doctor":     cmdDoctor,
		"top":        cmdTop,
		"show":       cmdShow,
		"dlq":        cmdDLQ,
		"cancel":     cmdCancel,
		"facts":      cmdFacts,
		"tail":       cmdTail,
		"serve":      cmdServe,
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

func mustOpenDB(path string) *queue.SQLiteStore {
	return mustOpenDBOpts(path)
}

func mustOpenDBOpts(path string, opts ...queue.StoreOption) *queue.SQLiteStore {
	s, err := queue.OpenSQLite(path, opts...)
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
	once := fs.Bool("once", false, "run until the claimable queue is drained, then exit (scripts/tests; parity with agent-pool --once)")

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	var opts []queue.StoreOption
	if *exclusive {
		opts = append(opts, queue.WithProjectExclusivity())
	}

	s := mustOpenDBOpts(resolveDB(*db), opts...)
	defer s.Close()

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

		agentExec := &executor.AgentExecutor{ProjectsDir: *projectsDir, Yolo: *yolo}
		reg.Register(executor.TaskTypeAgent, agentExec)
		// Carry review tasks minted by a --review agent-pool sharing this DB.
		reg.Register(executor.TaskTypeReview, &executor.ReviewExecutor{Agent: agentExec})
	}

	pool := worker.New(s, worker.Config{
		Owner:        *owner,
		Concurrency:  *conc,
		PollInterval: *poll,
		Lease:        *lease,
		TaskTimeout:  *timeout,
		Executors:    reg,
	}, nil)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *alertURL != "" {
		bridge := papdashboard.New(s, papdashboard.Config{
			Endpoint:     *alertURL,
			APIKey:       *alertKey,
			PollInterval: *alertPoll,
		})
		go func() {
			if err := bridge.Run(ctx); err != nil {
				fmt.Fprintln(os.Stderr, "tq: alert bridge failed:", err)
				stop()
			}
		}()

		fmt.Fprintf(os.Stderr, "tq: forwarding dead letters to %s\n", *alertURL)
	}

	if *once {
		// Timer-friendly mode (parity with agent-pool --once): as soon as
		// this pool has nothing in flight and no claimable work left, stop
		// the pool AND cancel the signal context — Start only returns once
		// ctx is done, so Stop alone would leave the process hanging until
		// the next signal. Work claimed by OTHER pools, or gated by a future
		// NotBefore, is left for them / for the next --once run.
		go func() {
			q := queue.New(s)
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(*poll):
					if pool.InFlight() == 0 && !hasClaimableWork(ctx, q, pool.Owner(), *poll) {
						pool.Stop()
						stop()

						return
					}
				}
			}
		}()
	}

	return pool.Start(ctx)
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
	dryRun := fs.Bool("dry-run", false, "report what would be enqueued, change nothing")
	asJSON := fs.Bool("json", false, "JSON output of the harvest result")

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
		ProjectsDir: *projectsDir,
		TodoFile:    *todoFile,
		Type:        *taskType,
		MaxPerTick:  *maxPerTick,
		Priority:    *priority,
		MaxAttempts: *maxAttempts,
		Model:       *model,
		DryRun:      *dryRun,
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
// --repos wins over --projects-dir, --repo-subset filters discovered repos.
func resolveHarvestRepos(cfg *harvest.Config, projectsDir, repos, subset string) error {
	if repos != "" {
		cfg.ProjectsDir = ""
		cfg.Repos = splitRepos(repos)

		return nil
	}

	if subset == "" {
		return nil
	}

	found, err := harvest.DiscoverRepos(projectsDir, cfg.TodoFile)
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

// cmdAgentPool is the self-managing loop in one process: it repeatedly
// harvests TODO_LIST.md backlogs into the queue (paced: one in-flight item
// per repo), optionally ingests Code-Quality-Agent scan findings as fix
// tasks, and runs a worker pool whose "agent" executor drives headless crush
// agents that do the work, verify it, and close the loop in the todo file.
// Ctrl-C drains gracefully, like tq worker.
func cmdAgentPool(args []string) error {
	fs := flag.NewFlagSet("agent-pool", flag.ExitOnError)
	projectsDir := fs.String(
		"projects-dir",
		defaultProjectsDir(),
		"dir containing repos (default $TQ_PROJECTS_DIR or ~/projects)",
	)
	repos := fs.String("repos", "", "comma-separated repo dirs (overrides --projects-dir)")
	interval := fs.Duration("interval", 5*time.Minute, "harvest cadence")
	conc := fs.Int("concurrency", 1, "parallel agents (repos are paced: one in-flight backlog item per repo)")
	poll := fs.Duration("poll", 500*time.Millisecond, "idle poll interval")
	lease := fs.Duration("lease", 5*time.Minute, "claim lease length (agents are slow; heartbeats keep it alive)")
	timeout := fs.Duration("task-timeout", 45*time.Minute, "per-agent timeout (agent run + verify)")
	owner := fs.String("owner", "", "lease owner identity")
	yolo := fs.Bool(
		"yolo",
		false,
		"agents auto-accept all permissions — required for unattended pools whose items need writes/commits",
	)
	maxPerTick := fs.Int(
		"max-per-tick",
		harvest.DefaultMaxPerTick,
		"max new agent tasks per harvest tick (cost throttle)",
	)
	allowDirty := fs.Bool("allow-dirty", false, "let agents run in repos with uncommitted changes (default: refuse)")
	model := fs.String(
		"model",
		"",
		"crush model override (e.g. anthropic/claude-sonnet-4-5) written into every harvested agent payload",
	)
	once := fs.Bool("once", false, "run one harvest tick, drain the queue, then exit (cron/timer-friendly)")
	exclusive := fs.Bool(
		"project-exclusive",
		false,
		"never run two tasks of the same project at once across ALL pools sharing this DB (enable it on every pool)",
	)
	dailyBudget := fs.Int(
		"daily-budget",
		0,
		"max agent tasks enqueued per calendar day across all repos (0 = unlimited)",
	)
	budgetCmd := fs.String(
		"budget-cmd",
		"",
		"checked before each harvest tick: exit 0 = within budget, non-zero skips the tick (output is the reason)",
	)
	repoInterval := fs.String(
		"repo-interval",
		"",
		"per-repo minimum gap between new enqueues: name=duration,comma-separated (e.g. big-repo=1h,tiny=5m)",
	)
	dlqBackoff := fs.Duration(
		"dlq-backoff",
		0,
		"pause harvesting a repo whose recent work is all dead-lettered for this long (0 = off, e.g. 30m)",
	)
	cqaURL := fs.String(
		"cqa-url",
		os.Getenv("CQA_URL"),
		"Code-Quality-Agent API base URL: latest scans' fixable findings become fix tasks each tick",
	)
	cqaOwner := fs.String("cqa-owner", os.Getenv("CQA_OWNER_ID"), "CQA owner ID for the projects listing")
	cqaToken := fs.String("cqa-token", os.Getenv("CQA_TOKEN"), "CQA bearer token")
	doReview := fs.Bool(
		"review",
		false,
		"agent reviews: each completed agent task gets ONE review by a second agent (reviews are never reviewed)",
	)
	alertURL := fs.String(
		"alert-url",
		os.Getenv("TQ_PAP_URL"),
		"PapDashboard base URL: dead-lettered tasks and budget exhaustion raise alerts there (e.g. http://localhost:8080)",
	)
	alertKey := fs.String("alert-api-key", os.Getenv("TQ_PAP_API_KEY"), "PapDashboard API key (Bearer)")
	alertPoll := fs.Duration("alert-poll", 5*time.Second, "journal tail interval for alert forwarding")
	reviewAutofix := fs.Bool(
		"review-autofix",
		false,
		"with --review: a request_changes verdict mints an agent fix task per finding (loop bounded by the budget guard)",
	)
	configPath := fs.String(
		"config",
		os.Getenv("TQ_POOL_CONFIG"),
		"key=value settings file applied to flags not given on the command line (precedence: flag > env > file; $TQ_POOL_CONFIG)",
	)

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *configPath != "" {
		if err := applyPoolConfigFile(fs, *configPath); err != nil {
			return err
		}
	}

	if *projectsDir == "" && *repos == "" {
		return errors.New("no repos: pass --repos or --projects-dir (or set $TQ_PROJECTS_DIR)")
	}

	if err := checkProjectsDir(*projectsDir); err != nil {
		return err
	}

	cfg := harvest.Config{ProjectsDir: *projectsDir, MaxPerTick: *maxPerTick, Model: *model, DLQBackoff: *dlqBackoff}
	if *repoInterval != "" {
		cfg.RepoIntervals = make(map[string]time.Duration)

		for spec := range strings.SplitSeq(*repoInterval, ",") {
			spec = strings.TrimSpace(spec)
			if spec == "" {
				continue
			}

			name, dur, ok := strings.Cut(spec, "=")
			if !ok {
				return fmt.Errorf("--repo-interval: want name=duration, got %q", spec)
			}

			d, err := time.ParseDuration(strings.TrimSpace(dur))
			if err != nil {
				return fmt.Errorf("--repo-interval: %q: %w", spec, err)
			}

			cfg.RepoIntervals[strings.TrimSpace(name)] = d
		}
	}

	if *allowDirty {
		no := false
		cfg.RequireClean = &no
	}

	if *repos != "" {
		cfg.ProjectsDir = ""
		cfg.Repos = splitRepos(*repos)
	}

	var opts []queue.StoreOption
	if *exclusive {
		opts = append(opts, queue.WithProjectExclusivity())
	}

	s := mustOpenDBOpts(resolveDB(*db), opts...)
	defer s.Close()

	q := queue.New(s)

	agentExec := &executor.AgentExecutor{ProjectsDir: *projectsDir, Yolo: *yolo}

	reg := executor.NewRegistry()
	reg.Register("sh", executor.NewCommandExecutor(""))
	reg.Register(executor.TaskTypeAgent, agentExec)
	// Reviews execute wherever agent tasks do: even a pool without --review
	// carries review tasks another pool minted (failing them at executor
	// lookup would burn attempts for nothing).
	reg.Register(executor.TaskTypeReview, &executor.ReviewExecutor{Agent: agentExec})
	fmt.Fprintf(
		os.Stderr,
		"tq: agent-pool: %d agent(s) over %s (yolo=%v, dirty=%v, exclusive=%v, harvest every %s, verify enforced)\n",
		*conc,
		repoRootDesc(*projectsDir, *repos),
		*yolo,
		*allowDirty,
		*exclusive,
		*interval,
	)

	if *yolo {
		fmt.Fprintf(
			os.Stderr,
			"tq: WARNING: autonomy requested — agents may run shell commands unsandboxed in every repo whose .crushrc (or your user-global crush config) grants bash; the trust root is the filesystem. Cap the blast radius with --daily-budget / --budget-cmd and --max-per-tick (see SECURITY.md)\n",
		)
	}

	if *cqaURL != "" {
		fmt.Fprintf(os.Stderr, "tq: agent-pool: ingesting CQA findings from %s each tick\n", *cqaURL)
	}

	if *doReview {
		desc := "verdicts recorded"
		if *reviewAutofix {
			desc = "request_changes mints fix tasks"
		}

		fmt.Fprintf(os.Stderr, "tq: agent-pool: agent reviews enabled (%s)\n", desc)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if *alertURL != "" {
		bridge := papdashboard.New(s, papdashboard.Config{
			Endpoint:     *alertURL,
			APIKey:       *alertKey,
			PollInterval: *alertPoll,
			// Mirror the pool's cap so the day it bites, an alert fires (and
			// resolves itself when the window rolls over).
			DailyBudget: *dailyBudget,
		})
		go func() {
			if err := bridge.Run(ctx); err != nil {
				fmt.Fprintln(os.Stderr, "tq: alert bridge failed:", err)
				stop()
			}
		}()

		fmt.Fprintf(os.Stderr, "tq: agent-pool: forwarding dead letters + budget exhaustion to %s\n", *alertURL)
	}

	log := slog.Default()
	guard := budget.Guard{DailyCap: *dailyBudget, BudgetCmd: *budgetCmd}
	h := harvest.New(q, cfg)

	var cqaBridge *cqa.Bridge
	if *cqaURL != "" {
		cqaBridge = cqa.New(cqa.Config{
			BaseURL:     *cqaURL,
			Token:       *cqaToken,
			OwnerID:     *cqaOwner,
			ProjectsDir: *projectsDir,
		})
	}

	var sweeper *review.Sweeper

	if *doReview {
		var err error

		sweeper, err = review.NewSweeper(ctx, s, review.SweeperConfig{
			Model:   *model,
			Autofix: *reviewAutofix,
			Log:     log,
		})
		if err != nil {
			return fmt.Errorf("review sweeper: %w", err)
		}
	}

	runTick := func() {
		if ok, reason := guard.Check(ctx, s); !ok {
			log.Warn("budget: skipping harvest tick", "reason", reason)

			return
		}

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

		if sweeper != nil {
			stats, err := sweeper.Sweep(ctx)
			if err != nil {
				log.Error("review sweep failed", "err", err)
			} else if stats.ReviewsEnqueued > 0 || stats.FixesEnqueued > 0 || stats.Skipped > 0 {
				log.Info("review sweep done", "facts", stats.Facts,
					"reviews", stats.ReviewsEnqueued, "known", stats.ReviewsKnown,
					"fixes", stats.FixesEnqueued, "skipped", stats.Skipped)
			}
		}

		if cqaBridge == nil {
			return
		}

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
	}
	go func() {
		runTick()

		if *once {
			return
		}

		ticker := time.NewTicker(*interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runTick()
			}
		}
	}()

	pool := worker.New(s, worker.Config{
		Owner:        *owner,
		Concurrency:  *conc,
		PollInterval: *poll,
		Lease:        *lease,
		TaskTimeout:  *timeout,
		Executors:    reg,
	}, log)

	if *once {
		// Timer-friendly mode: as soon as this pool has nothing in flight
		// and no claimable work left, stop the pool AND cancel the signal
		// context — Start only returns once ctx is done, so Stop alone
		// would leave the process hanging until the next signal. Work
		// claimed by OTHER pools, or gated by a future NotBefore, is left
		// for them / for the next --once run.
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(*poll):
					// Sweep before the drain check so reviews of work this
					// drain just completed run in the SAME --once process
					// (idempotent; dedup keeps repeat sweeps free).
					if sweeper != nil {
						if _, err := sweeper.Sweep(ctx); err != nil {
							log.Error("review sweep failed", "err", err)
						}
					}

					if pool.InFlight() == 0 && !hasClaimableWork(ctx, q, pool.Owner(), *poll) {
						pool.Stop()
						stop()

						return
					}
				}
			}
		}()
	}

	return pool.Start(ctx)
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
	asJSON := fs.Bool("json", false, "JSON output")

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	s := mustOpenDB(resolveDB(*db))
	defer s.Close()

	f := queue.Filter{}
	if *project != "" {
		f.Project = project
	}

	if *status != "" {
		st := task.Status(*status)
		f.Status = &st
	}

	tasks, err := s.List(context.Background(), f)
	if err != nil {
		return err
	}

	byStatus, byProject := tallyStats(tasks)

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")

		return enc.Encode(tasks)
	}

	printStats(byStatus, byProject, *project == "")

	return nil
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
		return errors.New("usage: tq show TASK_ID")
	}

	s := mustOpenDB(resolveDB(*db))
	defer s.Close()

	ctx := context.Background()

	t, err := s.Get(ctx, task.ID(fs.Arg(0)))
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
		Task  task.Task      `json:"task"`
		Facts []journal.Fact `json:"facts,omitempty"`
	}{t, trail})
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
func rescueAllDead(ctx context.Context, s *queue.SQLiteStore, olderThan time.Duration, maxAttempts int) (int, error) {
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

func listDead(ctx context.Context, s *queue.SQLiteStore) ([]task.Task, error) {
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

func cmdCancel(args []string) error {
	fs := flag.NewFlagSet("cancel", flag.ExitOnError)

	force := fs.Bool("force", false, "running tasks: request a cooperative cancel (observed at the worker's next heartbeat)")

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		return errors.New("usage: tq cancel TASK_ID [--force]")
	}

	s := mustOpenDB(resolveDB(*db))
	defer s.Close()

	ctx := context.Background()
	id := task.ID(fs.Arg(0))

	t, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	switch t.Status {
	case task.Pending:
		return s.Cancel(ctx, id)
	case task.Running:
		if !*force {
			return fmt.Errorf(
				"task %s is running; pass --force to request a cooperative cancel (the worker stops it at its next heartbeat)",
				id,
			)
		}

		if err := s.CancelRunning(ctx, id); err != nil {
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

	for _, f := range facts {
		fmt.Println(formatFact(f))
	}

	fmt.Printf("(%d facts)\n", len(facts))

	return nil
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

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", webui.DefaultAddr, "listen address (default: localhost only)")
	poll := fs.Duration("poll", webui.DefaultPoll, "journal tail interval")
	verbose := fs.Bool("verbose", false, "log every HTTP request (method, path, status, duration) to stderr")
	authToken := fs.String("auth-token", os.Getenv("TQ_SERVE_TOKEN"),
		"require this token on every request (Authorization: Bearer or ?token=); required for non-loopback --addr (env $TQ_SERVE_TOKEN)")

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg := webui.Config{Addr: *addr, Poll: *poll, RequestLog: *verbose, AuthToken: *authToken}
	if err := cfg.Validate(); err != nil {
		return err
	}

	s := mustOpenDB(resolveDB(*db))
	defer s.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := webui.New(s, cfg)

	fmt.Fprintf(os.Stderr, "tq: dashboard on http://%s (read-only)\n", *addr)

	return server.Run(ctx)
}
