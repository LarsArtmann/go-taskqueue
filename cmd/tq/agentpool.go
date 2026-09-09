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
	allowDirty     bool
	model          string
	once           bool
	pruneStale     bool
	exclusive      bool
	dailyBudget    int
	budgetCmd      string
	repoInterval   string
	dlqBackoff     time.Duration
	cqaURL         string
	cqaOwner       string
	cqaToken       string
	doReview       bool
	alertURL       string
	alertKey       string
	alertPoll      time.Duration
	repoTimeout    string
	maxAgents      int
	reviewAutofix  bool
	statusEvery    int
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

		os.Setenv("TQ_LOG_DIR", *logDir)
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

	if err := checkProjectsDir(*projectsDir); err != nil {
		return agentPoolOptions{}, err
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
		allowDirty:     *allowDirty,
		model:          *model,
		once:           *once,
		pruneStale:     *pruneStale,
		exclusive:      *exclusive,
		dailyBudget:    *dailyBudget,
		budgetCmd:      *budgetCmd,
		repoInterval:   *repoInterval,
		dlqBackoff:     *dlqBackoff,
		cqaURL:         *cqaURL,
		cqaOwner:       *cqaOwner,
		cqaToken:       *cqaToken,
		doReview:       *doReview,
		alertURL:       *alertURL,
		alertKey:       *alertKey,
		alertPoll:      *alertPoll,
		repoTimeout:    *repoTimeout,
		maxAgents:      *maxAgents,
		reviewAutofix:  *reviewAutofix,
		statusEvery:    *statusEvery,
		logDir:         *logDir,
		logDirMaxAge:   *logDirMaxAge,
		logDirMaxBytes: *logDirMaxBytes,
	}, nil
}

// harvestConfigFromOptions assembles the harvester configuration, parsing
// the name=duration ladder flags (--repo-timeout, --repo-interval).
func harvestConfigFromOptions(o agentPoolOptions) (harvest.Config, error) {
	cfg := harvest.Config{
		ProjectsDir:   o.projectsDir,
		DiscoveryAddr: o.discoveryAddr,
		MaxPerTick:    o.maxPerTick,
		Model:         o.model,
		DLQBackoff:    o.dlqBackoff,
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

// registerAgentExecutors wires the agent-family executors around one
// AgentExecutor: reviews and status reports execute wherever agent tasks
// do, so even a pool without --review / --status-every drains the tasks
// another pool minted instead of failing them at executor lookup.
func registerAgentExecutors(reg *executor.Registry, agentExec *executor.AgentExecutor) {
	reg.Register(executor.TaskTypeAgent, agentExec)
	reg.Register(executor.TaskTypeReview, &executor.ReviewExecutor{Agent: agentExec})
	reg.Register(executor.TaskTypeStatus, &executor.StatusExecutor{Agent: agentExec})
}

// printAgentPoolBanner prints the startup summary: pool shape, the yolo
// trust warning, and the agent-binary probe (a broken binary is a startup
// warning, not a mid-task surprise).
func printAgentPoolBanner(o agentPoolOptions) {
	fmt.Fprintf(
		os.Stderr,
		"tq: agent-pool: %d agent(s) over %s (yolo=%v, dirty=%v, exclusive=%v, harvest every %s, verify enforced)\n",
		o.conc,
		repoRootDesc(o.projectsDir, o.repos),
		o.yolo,
		o.allowDirty,
		o.exclusive,
		o.interval,
	)

	if o.yolo {
		fmt.Fprintf(
			os.Stderr,
			"tq: WARNING: autonomy requested — agents may run shell commands unsandboxed in every repo whose .crushrc (or your user-global crush config) grants bash; the trust root is the filesystem. Cap the blast radius with --daily-budget / --budget-cmd and --max-per-tick (see SECURITY.md)\n",
		)
	}

	if o.cqaURL != "" {
		fmt.Fprintf(os.Stderr, "tq: agent-pool: ingesting CQA findings from %s each tick\n", o.cqaURL)
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

	if o.doReview {
		desc := "verdicts recorded"
		if o.reviewAutofix {
			desc = "request_changes mints fix tasks"
		}

		fmt.Fprintf(os.Stderr, "tq: agent-pool: agent reviews enabled (%s)\n", desc)
	}

	if o.statusEvery > 0 {
		fmt.Fprintf(
			os.Stderr,
			"tq: agent-pool: automated status reports every %d agent completion(s) per project\n",
			o.statusEvery,
		)
	}
}
