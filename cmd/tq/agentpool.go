package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/harvest"
)

// agentPoolOptions is the resolved agent-pool configuration: flag values
// after parsing, the --config file merge, and environment fallbacks.
type agentPoolOptions struct {
	db             string
	projectsDir    string
	repos          string
	interval       time.Duration
	discoveryAddr  string
	conc           int
	poll           time.Duration
	lease          time.Duration
	timeout        time.Duration
	owner          string
	yolo           bool
	maxPerTick     int
	priorityFrom   string
	maxPending     int
	allowDirty     bool
	model          string
	once           bool
	pruneStale     bool
	reprioritize   bool
	exclusive      bool
	dailyBudget    int
	budgetCmd      string
	repoInterval   string
	dlqBackoff     time.Duration
	cqaURL         string
	cqaOwner       string
	cqaToken       string
	doReview       bool
	dlqFix         bool
	prioritize     bool
	alertURL       string
	alertKey       string
	alertPoll      time.Duration
	deadPoolTicks  int
	repoTimeout    string
	maxAgents      int
	reviewAutofix  bool
	statusEvery    int
	closeout       bool
	logDir         string
	logDirMaxAge   time.Duration
	logDirMaxBytes int64
}

// parseAgentPoolOptions owns the agent-pool flag block: define, parse,
// apply the --config file, resolve env fallbacks, and validate. Precedence
// everywhere: flag > env > file.
func parseAgentPoolOptions(args []string) (agentPoolOptions, error) {
	fs := flag.NewFlagSet("agent-pool", flag.ExitOnError)
	projectsDir := fs.String(
		"projects-dir",
		defaultProjectsDir(),
		"dir containing repos (default $TQ_PROJECTS_DIR or ~/projects)",
	)
	repos := fs.String("repos", "", "comma-separated repo dirs (overrides --projects-dir)")
	interval := fs.Duration("interval", 5*time.Minute, "harvest cadence")
	discoveryAddr := fs.String(
		"discovery-addr",
		os.Getenv("TQ_DISCOVERY_ADDR"),
		"project-discovery-daemon endpoint for repo discovery INSTEAD of the local scan each tick: unix socket (/run/project-discovery/daemon.sock, unix:// ok) or host:port; unreachable daemon = warning + local scan fallback; also subscribes the daemon's /v1/watch SSE stream so repo changes harvest within seconds instead of waiting --interval ($TQ_DISCOVERY_ADDR)",
	)
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
	priorityFrom := fs.String(
		"priority-from",
		"",
		`resolve harvested priorities from each repo's .config/metadata.yaml importance (0-100, default 50) plus keyword bumps, clamped to the backlog band; "importance" enables, empty keeps flat priority 0 (markers and hot promotion apply either way; ADR-0015)`,
	)
	maxPending := fs.Int(
		"max-pending-per-repo",
		0,
		"cap how many PENDING tasks one repo may hold in the queue (0 = legacy: any pending/running task holds the repo); the queue becomes the working set, TODO_LIST.md the warehouse",
	)
	allowDirty := fs.Bool("allow-dirty", false, "let agents run in repos with uncommitted changes (default: refuse)")
	model := fs.String(
		"model",
		"",
		"crush model override (e.g. anthropic/claude-sonnet-4-5) written into every harvested agent payload",
	)
	once := fs.Bool("once", false, "run one harvest tick, drain the queue, then exit (cron/timer-friendly)")
	pruneStale := fs.Bool(
		"prune-stale",
		true,
		"one zombie sweep before the first harvest tick: cancel PENDING tasks whose TODO_LIST item is now [x] or gone from the file, so a relaunch never inherits stale work (--prune-stale=false to skip)",
	)
	reprioritize := fs.Bool(
		"reprioritize",
		true,
		"one priority sweep before the first harvest tick: re-resolve PENDING task priorities from current TODO_LIST markers and (with --priority-from importance) repo metadata; value-idempotent, hot/machine bands protected (--reprioritize=false to skip)",
	)
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
	dlqFix := fs.Bool(
		"dlq-fix",
		false,
		"DLQ autopsies: each dead-lettered AGENT task gets ONE autopsy task by a second agent; a fixed verdict rescues the original, a wontfix verdict dismisses it with the recorded reason (autopsies are never autopsied)",
	)
	prioritize := fs.Bool(
		"prioritize",
		false,
		"AI batch scorer: when a repo holds unscored backlog items in the queue, mint ONE prioritize task per repo whose verdicts cache scores and re-rank the pending tasks (marker > AI > keyword precedence, hot/machine bands protected; budget-guarded like every mint)",
	)
	alertURL := fs.String(
		"alert-url",
		os.Getenv("TQ_PAP_URL"),
		"PapDashboard base URL: dead-lettered tasks and budget exhaustion raise alerts there (e.g. http://localhost:8080)",
	)
	alertKey := fs.String("alert-api-key", os.Getenv("TQ_PAP_API_KEY"), "PapDashboard API key (Bearer)")
	alertPoll := fs.Duration("alert-poll", 5*time.Second, "journal tail interval for alert forwarding")
	deadPoolTicks := fs.Int(
		"dead-pool-ticks",
		3,
		"dead-pool detection: raise a PapDashboard alert (and WARN log) when EVERY watched repo scan-fails this many consecutive harvest ticks; the first tick with a readable repo resolves it (0 = off)",
	)
	repoTimeout := fs.String(
		"repo-timeout",
		"",
		"per-repo agent-task timeout ladder: name=duration,comma-separated (e.g. big-repo=60m,tiny=10m; pinned into harvested payloads, repos without an entry keep the 30m default)",
	)
	maxAgents := fs.Int(
		"max-concurrent-agents",
		0,
		"cap agent processes MACHINE-WIDE across every tq pool on this host via slot files (0 = uncapped)",
	)
	reviewAutofix := fs.Bool(
		"review-autofix",
		false,
		"with --review: a request_changes verdict mints an agent fix task per finding (loop bounded by the budget guard)",
	)
	statusEvery := fs.Int(
		"status-every",
		0,
		"automated done-prompt: every N completed agent tasks per project mint one status task that writes a docs/status report and appends next items to TODO_LIST.md (0 = off)",
	)
	closeout := fs.Bool(
		"task-closeout",
		false,
		"every agent task gets a second conversation turn: the brutal self-review + status report prompt (executor.DefaultCloseoutPrompt), answered by the same agent session before verify runs",
	)
	logDir := fs.String(
		"log-dir",
		os.Getenv("TQ_LOG_DIR"),
		"write full agent + verify output sidecars to DIR/<task-id>.log ($TQ_LOG_DIR; empty = off — result detail keeps only a tail). WARNING: sidecars are PLAINTEXT and may contain repo paths and prompt content",
	)
	logDirMaxAge := fs.Duration(
		"log-dir-max-age",
		0,
		"sweep sidecar logs older than this age from --log-dir each tick (e.g. 168h = 7d; 0 = keep forever; $TQ_LOG_DIR_MAX_AGE)",
	)
	logDirMaxBytes := fs.Int64(
		"log-dir-max-bytes",
		0,
		"cap the total size of sidecar logs in --log-dir: oldest *.log files are deleted each tick until the total fits (e.g. 5368709120 = 5GiB; 0 = uncapped; $TQ_LOG_DIR_MAX_BYTES)",
	)
	configPath := fs.String(
		"config",
		os.Getenv("TQ_POOL_CONFIG"),
		"key=value settings file applied to flags not given on the command line (precedence: flag > env > file; $TQ_POOL_CONFIG)",
	)

	db := dbFlag(fs)
	if err := fs.Parse(args); err != nil {
		return agentPoolOptions{}, err
	}

	if *configPath != "" {
		if err := applyPoolConfigFile(fs, *configPath); err != nil {
			return agentPoolOptions{}, err
		}
	}

	// The sidecar writer reads the env at execution time; a --log-dir (or
	// config-file log-dir) must reach it regardless of how it was set.
	if *logDir != "" {
		// Create it up front: the first sidecar sweep would otherwise
		// warn (and a sidecar write could fail) when the dir is new.
		if err := os.MkdirAll(*logDir, 0o755); err != nil {
			return agentPoolOptions{}, fmt.Errorf("log dir: %w", err)
		}

		_ = os.Setenv("TQ_LOG_DIR", *logDir)
	}

	if envAge := os.Getenv("TQ_LOG_DIR_MAX_AGE"); envAge != "" && *logDirMaxAge == 0 {
		if parsed, err := time.ParseDuration(envAge); err == nil {
			*logDirMaxAge = parsed
		}
	}

	if envBytes := os.Getenv("TQ_LOG_DIR_MAX_BYTES"); envBytes != "" && *logDirMaxBytes == 0 {
		if parsed, err := strconv.ParseInt(envBytes, 10, 64); err == nil {
			*logDirMaxBytes = parsed
		}
	}

	if *projectsDir == "" && *repos == "" {
		return agentPoolOptions{}, errors.New("no repos: pass --repos or --projects-dir (or set $TQ_PROJECTS_DIR)")
	}

	// Fully-absolute --repos entries never touch the projects dir, so a
	// risky (or default) projects root must not block the run.
	if *repos == "" || !allReposAbsolute(*repos) {
		if err := checkProjectsDir(*projectsDir); err != nil {
			return agentPoolOptions{}, err
		}
	}

	return agentPoolOptions{
		db:             *db,
		projectsDir:    *projectsDir,
		repos:          *repos,
		interval:       *interval,
		discoveryAddr:  *discoveryAddr,
		conc:           *conc,
		poll:           *poll,
		lease:          *lease,
		timeout:        *timeout,
		owner:          *owner,
		yolo:           *yolo,
		maxPerTick:     *maxPerTick,
		priorityFrom:   *priorityFrom,
		maxPending:     *maxPending,
		allowDirty:     *allowDirty,
		model:          *model,
		once:           *once,
		pruneStale:     *pruneStale,
		reprioritize:   *reprioritize,
		exclusive:      *exclusive,
		dailyBudget:    *dailyBudget,
		budgetCmd:      *budgetCmd,
		repoInterval:   *repoInterval,
		dlqBackoff:     *dlqBackoff,
		cqaURL:         *cqaURL,
		cqaOwner:       *cqaOwner,
		cqaToken:       *cqaToken,
		doReview:       *doReview,
		dlqFix:         *dlqFix,
		prioritize:     *prioritize,
		alertURL:       *alertURL,
		alertKey:       *alertKey,
		alertPoll:      *alertPoll,
		deadPoolTicks:  *deadPoolTicks,
		repoTimeout:    *repoTimeout,
		maxAgents:      *maxAgents,
		reviewAutofix:  *reviewAutofix,
		statusEvery:    *statusEvery,
		closeout:       *closeout,
		logDir:         *logDir,
		logDirMaxAge:   *logDirMaxAge,
		logDirMaxBytes: *logDirMaxBytes,
	}, nil
}

// harvestConfigFromOptions assembles the harvester configuration, parsing
// the name=duration ladder flags (--repo-timeout, --repo-interval).
func harvestConfigFromOptions(o agentPoolOptions) (harvest.Config, error) {
	if o.priorityFrom != "" && o.priorityFrom != "importance" {
		return harvest.Config{}, fmt.Errorf(`--priority-from: want "importance" or empty, got %q`, o.priorityFrom)
	}

	cfg := harvest.Config{
		ProjectsDir:       o.projectsDir,
		DiscoveryAddr:     o.discoveryAddr,
		MaxPerTick:        o.maxPerTick,
		Model:             o.model,
		DLQBackoff:        o.dlqBackoff,
		UseImportance:     o.priorityFrom == "importance",
		MaxPendingPerRepo: o.maxPending,
	}

	if o.repoTimeout != "" {
		cfg.RepoTimeouts = make(map[string]time.Duration)

		for spec := range strings.SplitSeq(o.repoTimeout, ",") {
			spec = strings.TrimSpace(spec)
			if spec == "" {
				continue
			}

			name, dur, ok := strings.Cut(spec, "=")
			if !ok {
				return harvest.Config{}, fmt.Errorf("--repo-timeout: want name=duration, got %q", spec)
			}

			d, err := time.ParseDuration(strings.TrimSpace(dur))
			if err != nil {
				return harvest.Config{}, fmt.Errorf("--repo-timeout: %q: %w", spec, err)
			}

			cfg.RepoTimeouts[strings.TrimSpace(name)] = d
		}
	}

	if o.repoInterval != "" {
		cfg.RepoIntervals = make(map[string]time.Duration)

		for spec := range strings.SplitSeq(o.repoInterval, ",") {
			spec = strings.TrimSpace(spec)
			if spec == "" {
				continue
			}

			name, dur, ok := strings.Cut(spec, "=")
			if !ok {
				return harvest.Config{}, fmt.Errorf("--repo-interval: want name=duration, got %q", spec)
			}

			d, err := time.ParseDuration(strings.TrimSpace(dur))
			if err != nil {
				return harvest.Config{}, fmt.Errorf("--repo-interval: %q: %w", spec, err)
			}

			cfg.RepoIntervals[strings.TrimSpace(name)] = d
		}
	}

	if o.allowDirty {
		no := false
		cfg.RequireClean = &no
	}

	if o.repos != "" {
		// Bare repo names resolve against the projects dir, never the
		// working directory: harvest Abs()es each entry, so un-expanded
		// names made every scan cwd-dependent (the systemd pool scans
		// from dirOf(dbPath) and skipped all repos as "scan failed").
		cfg.Repos = splitRepos(o.repos)
		for i, repo := range cfg.Repos {
			if !filepath.IsAbs(repo) {
				cfg.Repos[i] = filepath.Join(o.projectsDir, repo)
			}
		}

		cfg.ProjectsDir = ""
	}

	return cfg, nil
}

// deadPoolDetector watches consecutive harvest results and fires its
// notify hook once per all-repos-blind streak: a tick whose scan-failed
// skips cover every watched repo means the pool cannot read a single
// TODO_LIST (the 2026-09-10 pool-deploy incident class), and ticks of it
// in a row is a dead pool, not a hiccup. The first tick with a readable
// repo resolves the standing alert and resets the streak.
type deadPoolDetector struct {
	ticks   int // streak length that fires (<= 0 disables)
	streak  int
	alerted bool
	// notify receives triggered=true once per streak and triggered=false on
	// recovery; nil = track state only.
	notify func(triggered bool, repos int, example string, streak int)
}

func (d *deadPoolDetector) observe(res harvest.Result) {
	if d.ticks <= 0 || res.Repos == 0 {
		return
	}

	scanFailed := 0

	example := ""

	for _, skip := range res.Skipped {
		if class, _, _ := strings.Cut(skip.Reason, ":"); class == harvest.ReasonScanFailed {
			scanFailed++

			if example == "" {
				example = skip.Reason
			}
		}
	}

	if scanFailed < res.Repos {
		if d.alerted && d.notify != nil {
			d.notify(false, res.Repos, example, d.streak)
		}

		d.streak, d.alerted = 0, false

		return
	}

	d.streak++
	if d.streak < d.ticks || d.alerted {
		return
	}

	d.alerted = true
	if d.notify != nil {
		d.notify(true, res.Repos, example, d.streak)
	}
}

// registerAgentExecutors wires the agent-family executors around one
// AgentExecutor: reviews and status reports execute wherever agent tasks
// do, so even a pool without --review / --status-every drains the tasks
// another pool minted instead of failing them at executor lookup.
func registerAgentExecutors(reg *executor.Registry, agentExec *executor.AgentExecutor) {
	reg.Register(executor.TaskTypeAgent, agentExec)

	// The close-out turn belongs to WORK tasks only: reviews already are
	// the second opinion and status tasks already are the report — giving
	// them their own self-review doubles agent cost for no new signal.
	// DLQ autopsies run the same closeout-free clone (they are a diagnosis
	// instrument, not work).
	secondOpinion := agentExec.WithoutCloseout()

	reg.Register(executor.TaskTypeReview, &executor.ReviewExecutor{Agent: secondOpinion})
	reg.Register(executor.TaskTypeStatus, &executor.StatusExecutor{Agent: secondOpinion})
	reg.Register(executor.TaskTypeDLQFix, &executor.DLQFixExecutor{Agent: secondOpinion})
	reg.Register(executor.TaskTypePrioritize, &executor.PrioritizeExecutor{Agent: secondOpinion})
}

// printAgentPoolBanner prints the startup summary: pool shape, the yolo
// trust warning, and the agent-binary probe (a broken binary is a startup
// warning, not a mid-task surprise).
func printAgentPoolBanner(poolOpts agentPoolOptions) {
	fmt.Fprintf(
		os.Stderr,
		"tq: agent-pool: %d agent(s) over %s (yolo=%v, dirty=%v, exclusive=%v, harvest every %s, verify enforced)\n",
		poolOpts.conc,
		repoRootDesc(poolOpts.projectsDir, poolOpts.repos),
		poolOpts.yolo,
		poolOpts.allowDirty,
		poolOpts.exclusive,
		poolOpts.interval,
	)

	if poolOpts.yolo {
		fmt.Fprintf(
			os.Stderr,
			"tq: WARNING: autonomy requested — agents may run shell commands unsandboxed in every repo whose .crushrc (or your user-global crush config) grants bash; the trust root is the filesystem. Cap the blast radius with --daily-budget / --budget-cmd and --max-per-tick (see SECURITY.md)\n",
		)
	}

	if poolOpts.cqaURL != "" {
		fmt.Fprintf(os.Stderr, "tq: agent-pool: ingesting CQA findings from %s each tick\n", poolOpts.cqaURL)
	}

	if version, err := executor.AgentVersion(context.Background(), ""); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"tq: WARNING: agent binary probe failed: %v (agent tasks cannot run; TQ_AGENT_BIN overrides)\n",
			err,
		)
	} else {
		fmt.Fprintf(os.Stderr, "tq: agent-pool: agent binary: %s\n", version)
	}

	if poolOpts.doReview {
		desc := "verdicts recorded"
		if poolOpts.reviewAutofix {
			desc = "request_changes mints fix tasks"
		}

		fmt.Fprintf(os.Stderr, "tq: agent-pool: agent reviews enabled (%s)\n", desc)
	}

	if poolOpts.statusEvery > 0 {
		fmt.Fprintf(
			os.Stderr,
			"tq: agent-pool: automated status reports every %d agent completion(s) per project\n",
			poolOpts.statusEvery,
		)
	}

	if poolOpts.dlqFix {
		fmt.Fprintf(
			os.Stderr,
			"tq: agent-pool: DLQ autopsies enabled (dead agent tasks get one diagnosis run; fixed rescues, wontfix dismisses)\n",
		)
	}

	if poolOpts.deadPoolTicks > 0 {
		fmt.Fprintf(
			os.Stderr,
			"tq: agent-pool: dead-pool detection: alert after %d all-repo scan-failed tick(s)\n",
			poolOpts.deadPoolTicks,
		)
	}
}
