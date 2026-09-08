package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/harvest"
)

// tqManagedStart/End bracket the block cmdBootstrap owns inside a repo's
// .crushrc. Everything outside the markers is user content and is never
// touched; re-running bootstrap replaces the block in place.
const (
	tqManagedStart = "# >>> tq bootstrap (managed) >>>"
	tqManagedEnd   = "# <<< tq bootstrap (managed) <<<"
	agentTools     = "view ls grep glob edit write bash"
)

// systemdUnitTemplate is deploy/systemd/tq-agent-pool.service embedded so
// `tq bootstrap --install` works from an installed binary (no repo checkout
// needed). TestUnitTemplateMatchesDeployFile pins it against the tracked
// file (modulo ExecStart, which --install renders with the real binary
// path) — if the tracked unit changes, that test forces this copy to sync.
const systemdUnitTemplate = `[Unit]
Description=tq agent-pool (self-managing TODO_LIST harvest + headless agents)
Documentation=https://github.com/LarsArtmann/go-taskqueue
After=network-online.target
Wants=network-online.target

[Service]
# ExecStart is rendered by ` + "`tq bootstrap --install`" + ` with the installed
# binary path; runtime settings live in the pool config file.
ExecStart={{BIN}} agent-pool --config %h/.config/tq/pool.conf
Restart=on-failure
RestartSec=30s

# Graceful drain: agents may run for a long time; give them a wide stop
# window. The pool survives the first SIGTERM and finishes in-flight tasks.
TimeoutStopSec=45min
KillSignal=SIGINT
KillMode=process

# Hardening. Deliberately conservative: the pool execs headless agents that
# write/commit inside ~/projects and talk to the network, so no sandboxing
# below may break either. NoNewPrivileges and the kernel/cgroup namespaces
# cost nothing; ProtectSystem=full keeps /usr,/boot,/etc read-only while
# leaving $HOME writable for the repos and the binary.
NoNewPrivileges=true
ProtectSystem=full
ProtectControlGroups=true
ProtectKernelModules=true
ProtectKernelTunables=true
ProtectKernelLogs=true
RestrictSUIDSGID=true
LockPersonality=true

[Install]
WantedBy=default.target
`

// bootstrapOptions is the resolved state of one `tq bootstrap` invocation.
type bootstrapOptions struct {
	repos         []string // repo specs: bare names (against projectsDir) or paths
	projectsDir   string
	agents        int    // pool concurrency AND machine-wide agent cap
	model         string // "" = inherit each repo's crush config default
	reasoning     string // low|medium|high|xhigh; applied when model is set
	verify        map[string]string
	interval      time.Duration
	dailyBudget   int
	maxPerTick    int
	once          bool
	dryRun        bool
	install       bool
	noRun         bool // ensure + report, then exit (pool runs later via systemd/cron)
	yolo          bool
	review        bool
	reviewAutofix bool
	exclusive     bool
	allowDirty    bool
	repoTimeout   string
	db            string
	binPath       string // resolved executable, for the systemd unit
}

// reorderBootstrapArgs lets repos and flags appear in any order
// (`tq bootstrap CV --agents 2` reads naturally). Go's flag package stops at
// the first positional, so flags after a repo name would be swallowed.
// The walk tracks value-taking flags (bool flags consume nothing) so a
// flag's VALUE is never mistaken for a positional; `--` ends flag parsing.
func reorderBootstrapArgs(fs *flag.FlagSet, args []string) []string {
	consumes := func(name string) bool {
		f := fs.Lookup(strings.TrimLeft(name, "-"))
		if f == nil {
			return true // unknown: assume it takes a value (parse will error)
		}

		bv, isBool := f.Value.(interface{ IsBoolFlag() bool })

		return !isBool || !bv.IsBoolFlag()
	}

	var flags, positionals []string

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			positionals = append(positionals, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(a, "-"):
			flags = append(flags, a)
			if !strings.Contains(a, "=") && consumes(a) && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
		default:
			positionals = append(positionals, a)
		}
	}

	return append(flags, positionals...)
}

func cmdBootstrap(args []string) error {
	o, err := parseBootstrapArgs(args)
	if err != nil {
		return err
	}

	paths, err := o.resolveRepos()
	if err != nil {
		return err
	}

	// The delegated pool gets absolute paths: with --repos set, harvest
	// treats repo specs as directories relative to its CWD, so bare names
	// would silently harvest nothing.
	o.repos = paths

	report, err := o.ensureRepos(paths)
	if err != nil {
		return err
	}

	fmt.Print(report)

	if o.install {
		return o.installService()
	}

	if o.noRun {
		fmt.Fprintf(os.Stderr, "tq: bootstrap: repo state ensured — start the pool later with:\n  tq %s\n", strings.Join(composePoolArgs(o), " "))
		return nil
	}

	if o.dryRun {
		fmt.Fprintf(os.Stderr, "tq: bootstrap: dry-run — nothing written, pool not started\nwould run: tq %s\n", strings.Join(composePoolArgs(o), " "))
		return nil
	}

	return cmdAgentPool(composePoolArgs(o))
}

func parseBootstrapArgs(args []string) (bootstrapOptions, error) {
	fs := flag.NewFlagSet("bootstrap", flag.ExitOnError)

	o := bootstrapOptions{}
	reposFlag := fs.String("repos", "", "comma-separated additional repos (positional args also work)")
	fs.StringVar(&o.projectsDir, "projects-dir", defaultProjectsDir(), "dir containing repos (default $TQ_PROJECTS_DIR or ~/projects)")
	fs.IntVar(&o.agents, "agents", 1, "parallel agents (also the machine-wide agent cap)")
	fs.StringVar(&o.model, "model", "", "crush model override pinned into payloads and repo configs, 'provider/model' (empty = each repo's crush config default)")
	fs.StringVar(&o.reasoning, "reasoning", "xhigh", "reasoning effort for the pinned model: low|medium|high|xhigh (xhigh = max possible)")
	verifyFlag := fs.String("verify", "", "per-repo verify override written into .tq-verify: name=cmd,name=cmd")
	fs.DurationVar(&o.interval, "interval", 5*time.Minute, "harvest cadence")
	fs.IntVar(&o.dailyBudget, "daily-budget", 20, "max agent tasks enqueued per calendar day (cost ceiling; 0 = unlimited)")
	fs.IntVar(&o.maxPerTick, "max-per-tick", harvest.DefaultMaxPerTick, "max new agent tasks per harvest tick")
	fs.BoolVar(&o.once, "once", false, "one harvest tick, drain, exit (cron/timer-friendly)")
	fs.BoolVar(&o.dryRun, "dry-run", false, "show the plan without writing anything or starting the pool")
	fs.BoolVar(&o.install, "install", false, "install the systemd user unit + pool config, then exit (daemon mode)")
	fs.BoolVar(&o.noRun, "no-run", false, "ensure repo state, print the pool command, and exit without starting the pool")
	fs.BoolVar(&o.allowDirty, "allow-dirty", false, "let agents run in repos with uncommitted changes (default: refuse)")
	fs.StringVar(&o.repoTimeout, "repo-timeout", "", "per-repo agent-task timeout ladder: name=duration,...")
	fs.StringVar(&o.db, "db", "", "task DB (default $TQ_DB or ./tasks.db)")
	noYolo := fs.Bool("no-yolo", false, "disable autonomy (agents will stall on permission prompts)")
	noReview := fs.Bool("no-review", false, "disable the second-agent review pass")
	noReviewAutofix := fs.Bool("no-review-autofix", false, "with reviews: do not mint fix tasks from request_changes verdicts")
	noExclusive := fs.Bool("no-exclusive", false, "allow two tasks of one project in flight across pools")

	fs.Usage = func() {
		fmt.Fprint(fs.Output(), usageBootstrap)
		fs.PrintDefaults()
	}

	if err := fs.Parse(reorderBootstrapArgs(fs, args)); err != nil {
		return o, err
	}

	o.repos = append(splitRepos(*reposFlag), fs.Args()...)
	o.verify = parseVerifySpec(*verifyFlag)
	o.yolo, o.review, o.reviewAutofix, o.exclusive = !*noYolo, !*noReview, !*noReviewAutofix, !*noExclusive

	if o.agents < 1 {
		o.agents = 1
	}

	if o.model != "" {
		o.reasoning = strings.ToLower(strings.TrimSpace(o.reasoning))
	}

	if err := o.validate(); err != nil {
		return o, err
	}

	return o, nil
}

const usageBootstrap = `tq bootstrap: one command from zero to a running agent pool.

Prepares each repo for unattended agent work (idempotent — safe to re-run):

  1. validates the repo (git checkout, TODO_LIST.md with open items)
  2. ensures .tq-verify   — the verify command agents are gated by
     (--verify name=cmd overrides; otherwise auto-detected and pinned)
  3. ensures .crushrc     — a managed block granting agent autonomy
     (permissions allow ` + agentTools + `) and, when --model is set, pinning
     the model + reasoning effort (applies to interactive crush in that
     repo too — project-local config wins over your global default)
  4. commits exactly those files (never pushes), so the pool's clean-tree
     check passes
  5. previews the harvest (how many tasks each repo will feed)

then starts the agent pool (or, with --install, installs the systemd user
unit + pool config for daemon mode and exits).

Usage:
  tq bootstrap [repos...] [flags]

Examples:
  tq bootstrap CV,SystemNix --agents 2
  tq bootstrap CV SystemNix go-taskqueue --model zai/glm-5.3-flash --once
  tq bootstrap CV --verify 'CV=templ generate && bash scripts/go-change-gate.sh' --dry-run
  tq bootstrap --install   # renders the unit + ~/.config/tq/pool.conf, enables + lingers
`

func (o bootstrapOptions) validate() error {
	if len(o.repos) == 0 {
		return errors.New("bootstrap: no repos: pass repo names, paths, or --repos a,b")
	}

	if o.model != "" && !strings.Contains(o.model, "/") {
		return fmt.Errorf("bootstrap: --model %q: want provider/model (run `crush models`; the provider must be declared with credentials in your crush config)", o.model)
	}

	switch o.reasoning {
	case "low", "medium", "high", "xhigh":
	default:
		return fmt.Errorf("bootstrap: --reasoning %q: want low|medium|high|xhigh", o.reasoning)
	}

	if o.install && o.once {
		return errors.New("bootstrap: --install is daemon mode; --once is cron mode — pick one")
	}

	if o.install && o.dryRun {
		return errors.New("bootstrap: --install and --dry-run are contradictory")
	}

	if o.noRun && (o.once || o.install) {
		return errors.New("bootstrap: --no-run already exits after ensuring; drop --once/--install")
	}

	return nil
}

// resolveRepos expands bare repo names against the projects dir and makes
// every spec absolute, so both the ensure-step and the delegated pool see
// identical paths.
func (o bootstrapOptions) resolveRepos() ([]string, error) {
	if _, err := os.Stat(o.projectsDir); err != nil {
		return nil, fmt.Errorf("bootstrap: --projects-dir %s: %w", o.projectsDir, err)
	}

	seen := map[string]bool{}

	var paths []string

	for _, spec := range o.repos {
		p := spec
		if !filepath.IsAbs(p) {
			if _, err := os.Stat(p); err != nil { // not a path relative to CWD — try the projects dir
				p = filepath.Join(o.projectsDir, spec)
			}
		}

		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, fmt.Errorf("bootstrap: repo %q: %w", spec, err)
		}

		info, err := os.Stat(abs)
		if err != nil || !info.IsDir() {
			return nil, fmt.Errorf("bootstrap: repo %q does not exist (looked at %s)", spec, abs)
		}

		if seen[abs] {
			continue
		}

		seen[abs] = true
		paths = append(paths, abs)
	}

	return paths, nil
}

// ensureRepos runs the per-repo ensure steps and returns the human report.
func (o bootstrapOptions) ensureRepos(paths []string) (string, error) {
	var b strings.Builder

	for _, repo := range paths {
		name := filepath.Base(repo)
		fmt.Fprintf(&b, "\n== %s (%s) ==\n", name, repo)

		if _, err := os.Stat(filepath.Join(repo, ".git")); err != nil {
			fmt.Fprintf(&b, "  WARN  not a git checkout — agents cannot commit or close the loop\n")
		}

		open, err := openTodoItems(repo)
		if err != nil {
			fmt.Fprintf(&b, "  WARN  todo file: %v\n", err)
		} else {
			fmt.Fprintf(&b, "  todo  %d open item(s) → tasks after harvest\n", open)
		}

		action, cmd, err := o.ensureTQVerify(repo)
		if err != nil {
			return "", fmt.Errorf("bootstrap %s: %w", name, err)
		}

		switch action {
		case verifyWrote:
			fmt.Fprintf(&b, "  verify wrote .tq-verify: %s\n", cmd)
		case verifyOverride:
			fmt.Fprintf(&b, "  verify would pin (--verify override): %s\n", cmd)
		case verifyKept:
			fmt.Fprintf(&b, "  verify kept existing .tq-verify: %s\n", cmd)
		default:
			fmt.Fprintf(&b, "  WARN  no verify command (tasks complete without proof) — pass --verify %s=<cmd>\n", name)
		}

		changedCrushrc, err := o.ensureCrushConfig(repo)
		if err != nil {
			return "", fmt.Errorf("bootstrap %s: %w", name, err)
		}

		if o.model != "" {
			fmt.Fprintf(&b, "  crush pinned model %s (reasoning %s) + autonomy in .crushrc (changed: %v)\n", o.model, o.reasoning, changedCrushrc)
		} else if changedCrushrc {
			fmt.Fprintf(&b, "  crush autonomy granted in .crushrc (changed: true)\n")
		} else {
			fmt.Fprintf(&b, "  crush autonomy already granted in .crushrc\n")
		}

		files := []string{".tq-verify", ".crushrc"}
		if o.dryRun {
			fmt.Fprintf(&b, "  git   dry-run — would commit %s if changed\n", strings.Join(files, ", "))
			continue
		}

		committed, err := ensureCommitted(repo, files)
		if err != nil {
			fmt.Fprintf(&b, "  WARN  commit failed (%v) — the pool refuses dirty repos until this is committed\n", err)
		} else if committed {
			fmt.Fprintf(&b, "  git   committed ensured files (never pushes)\n")
		} else {
			fmt.Fprintf(&b, "  git   nothing to commit\n")
		}

		if dirty, err := repoDirty(repo); err == nil && dirty {
			fmt.Fprintf(&b, "  WARN  tree has uncommitted changes — the pool refuses agent tasks here until committed (agents require a clean tree)\n")
		}
	}

	return b.String(), nil
}

// repoDirty reports uncommitted (staged, unstaged, or untracked) changes.
func repoDirty(repo string) (bool, error) {
	out, err := exec.Command("git", "-C", repo, "status", "--porcelain").Output()
	if err != nil {
		return false, err
	}

	return len(strings.TrimSpace(string(out))) > 0, nil
}

// openTodoItems counts the harvestable (open, unblocked) checkbox items.
func openTodoItems(repo string) (int, error) {
	items, err := harvest.ParseRepo(repo, harvest.DefaultTodoFile)
	if err != nil {
		return 0, err
	}

	return len(items), nil
}

// Verify actions reported by ensureTQVerify.
const (
	verifyWrote    = "wrote"    // pinned (detected or overridden) and written
	verifyOverride = "override" // --verify override intent (dry-run: not yet written)
	verifyKept     = "kept"     // existing .tq-verify left untouched
	verifyNone     = "none"     // nothing to enforce
)

// ensureTQVerify makes sure the repo declares its verify contract: an
// explicit --verify entry for this repo is always (re)written; an existing
// .tq-verify is kept; otherwise the detected command is pinned. Returns
// (action, effectiveCommand).
func (o bootstrapOptions) ensureTQVerify(repo string) (string, string, error) {
	name := filepath.Base(repo)

	if cmd, ok := o.verify[name]; ok && cmd != "" {
		if o.dryRun {
			return verifyOverride, cmd, nil
		}

		if err := os.WriteFile(filepath.Join(repo, ".tq-verify"), []byte(cmd+"\n"), 0o644); err != nil {
			return verifyNone, "", err
		}

		return verifyWrote, cmd, nil
	}

	if existing := executor.ReadTQVerify(repo); existing != "" {
		return verifyKept, existing, nil
	}

	detected := executor.DetectVerify(repo)
	if detected == "" || o.dryRun {
		return verifyNone, detected, nil
	}

	if err := os.WriteFile(filepath.Join(repo, ".tq-verify"), []byte(detected+"\n"), 0o644); err != nil {
		return verifyNone, "", err
	}

	return verifyWrote, detected, nil
}

// ensureCrushConfig writes the managed autonomy (+model) block into the
// repo's .crushrc, replacing any previous managed block and leaving all
// other content untouched. Idempotent: unchanged content reports changed
// only when the file actually differs.
func (o bootstrapOptions) ensureCrushConfig(repo string) (bool, error) {
	if o.dryRun {
		return true, nil
	}

	path := filepath.Join(repo, ".crushrc")

	var lines []string

	if b, err := os.ReadFile(path); err == nil {
		lines = stripManagedBlock(strings.Split(string(b), "\n"))
	}

	block := []string{
		tqManagedStart,
		"permissions allow " + agentTools,
	}
	if o.model != "" {
		block = append(block, "model large "+o.model+" --reasoning-effort "+o.reasoning)
	}
	block = append(block, tqManagedEnd)

	out := strings.Join(append(lines, block...), "\n")
	out = strings.TrimRight(out, "\n") + "\n"

	if b, err := os.ReadFile(path); err == nil && string(b) == out {
		return false, nil // byte-identical: nothing to do
	}

	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return false, err
	}

	return true, nil
}

// stripManagedBlock drops everything between (and including) the managed
// markers from a previous run.
func stripManagedBlock(lines []string) []string {
	var out []string

	inBlock := false
	for _, l := range lines {
		switch {
		case strings.TrimSpace(l) == tqManagedStart:
			inBlock = true
		case strings.TrimSpace(l) == tqManagedEnd:
			inBlock = false
		case !inBlock:
			out = append(out, l)
		}
	}

	// Unterminated block from a corrupted earlier run: keep the content
	// outside markers and drop the stray opener.
	// Trim edge blank lines so re-runs stay byte-identical (they carry no
	// meaning at file boundaries and would otherwise accumulate).
	for len(out) > 0 && strings.TrimSpace(out[0]) == "" {
		out = out[1:]
	}

	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}

	return out
}

// ensureCommitted stages exactly the given files and commits them if the
// staged diff is non-empty. Never pushes. Identity comes from the repo
// config or -c overrides, so it works in fresh clones and CI.
func ensureCommitted(repo string, files []string) (bool, error) {
	abs := make([]string, 0, len(files))
	for _, f := range files {
		abs = append(abs, filepath.Join(repo, f))
	}

	add := exec.Command("git", "-C", repo, "add", "--")
	add.Args = append(add.Args, abs...)
	if out, err := add.CombinedOutput(); err != nil {
		return false, fmt.Errorf("git add: %w: %s", err, tailStr(string(out), 512))
	}

	staged := exec.Command("git", "-C", repo, "diff", "--cached", "--name-only")

	var out strings.Builder

	staged.Stdout = &out
	if err := staged.Run(); err != nil {
		return false, fmt.Errorf("git diff --cached: %w", err)
	}

	if strings.TrimSpace(out.String()) == "" {
		return false, nil
	}

	commit := exec.Command(
		"git", "-C", repo,
		"-c", "user.name=tq bootstrap", "-c", "user.email=tq-bootstrap@localhost",
		"commit", "-m", "chore(tq): bootstrap agent autonomy + verify pin",
	)
	if out, err := commit.CombinedOutput(); err != nil {
		return false, fmt.Errorf("git commit: %w: %s", err, tailStr(string(out), 512))
	}

	return true, nil
}

// composePoolArgs maps the resolved bootstrap options onto agent-pool flags
// (explicitly, so pool defaults never silently diverge from the bootstrap
// contract).
func composePoolArgs(o bootstrapOptions) []string {
	args := []string{
		"agent-pool",
		"--projects-dir", o.projectsDir,
		"--repos", strings.Join(o.repos, ","),
		"--concurrency", fmt.Sprint(o.agents),
		"--max-concurrent-agents", fmt.Sprint(o.agents),
		"--interval", o.interval.String(),
		"--daily-budget", fmt.Sprint(o.dailyBudget),
		"--max-per-tick", fmt.Sprint(o.maxPerTick),
		"--yolo=" + fmt.Sprint(o.yolo),
		"--review=" + fmt.Sprint(o.review),
		"--review-autofix=" + fmt.Sprint(o.reviewAutofix),
		"--project-exclusive=" + fmt.Sprint(o.exclusive),
		"--allow-dirty=" + fmt.Sprint(o.allowDirty),
	}

	if o.model != "" {
		args = append(args, "--model", o.model)
	}

	if o.repoTimeout != "" {
		args = append(args, "--repo-timeout", o.repoTimeout)
	}

	if o.db != "" {
		args = append(args, "--db", o.db)
	}

	if o.once {
		args = append(args, "--once")
	}

	return args
}

// installService writes the pool config + systemd unit, enables the unit,
// and enables linger so the pool starts at boot without a login session.
func (o bootstrapOptions) installService() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("bootstrap: install: %w", err)
	}

	confDir := filepath.Join(home, ".config", "tq")
	if err := os.MkdirAll(confDir, 0o700); err != nil {
		return fmt.Errorf("bootstrap: install: %w", err)
	}

	conf := filepath.Join(confDir, "pool.conf")
	if err := os.WriteFile(conf, []byte(renderPoolConfig(o)), 0o600); err != nil {
		return fmt.Errorf("bootstrap: install: %w", err)
	}

	unitDir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(unitDir, 0o755); err != nil {
		return fmt.Errorf("bootstrap: install: %w", err)
	}

	unit := filepath.Join(unitDir, "tq-agent-pool.service")
	if err := os.WriteFile(unit, []byte(renderUnit(o.binPath)), 0o644); err != nil {
		return fmt.Errorf("bootstrap: install: %w", err)
	}

	for _, cmd := range [][]string{
		{"systemctl", "--user", "daemon-reload"},
		{"systemctl", "--user", "enable", "--now", "tq-agent-pool.service"},
		{"loginctl", "enable-linger"},
	} {
		c := exec.Command(cmd[0], cmd[1:]...)
		if out, err := c.CombinedOutput(); err != nil {
			return fmt.Errorf("bootstrap: install: %s: %w: %s", strings.Join(cmd, " "), err, tailStr(string(out), 512))
		}
	}

	fmt.Fprintf(os.Stderr, `tq: bootstrap: installed
  unit   %s
  config %s
  follow: journalctl --user -u tq-agent-pool -f
  status: tq serve   # read-only dashboard (add --auth-token for LAN)
`, unit, conf)

	return nil
}

// renderPoolConfig renders the resolved options as the flat key=value file
// agent-pool reads via --config (same key names as its flags).
func renderPoolConfig(o bootstrapOptions) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# generated by `tq bootstrap --install` %s — edit freely, flags on the CLI win\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(&b, "projects-dir = %s\n", o.projectsDir)
	fmt.Fprintf(&b, "repos = %s\n", strings.Join(o.repos, ","))
	fmt.Fprintf(&b, "concurrency = %d\n", o.agents)
	fmt.Fprintf(&b, "max-concurrent-agents = %d\n", o.agents)
	fmt.Fprintf(&b, "interval = %s\n", o.interval)
	fmt.Fprintf(&b, "daily-budget = %d\n", o.dailyBudget)
	fmt.Fprintf(&b, "max-per-tick = %d\n", o.maxPerTick)
	fmt.Fprintf(&b, "yolo = %v\n", o.yolo)
	fmt.Fprintf(&b, "review = %v\n", o.review)
	fmt.Fprintf(&b, "review-autofix = %v\n", o.reviewAutofix)
	fmt.Fprintf(&b, "project-exclusive = %v\n", o.exclusive)
	fmt.Fprintf(&b, "allow-dirty = %v\n", o.allowDirty)

	if o.model != "" {
		fmt.Fprintf(&b, "model = %s\n", o.model)
	}

	if o.repoTimeout != "" {
		fmt.Fprintf(&b, "repo-timeout = %s\n", o.repoTimeout)
	}

	return b.String()
}

// renderUnit renders the systemd unit with the real binary path.
func renderUnit(binPath string) string {
	return strings.ReplaceAll(systemdUnitTemplate, "{{BIN}}", binPath)
}

func parseVerifySpec(spec string) map[string]string {
	out := map[string]string{}
	for pair := range strings.SplitSeq(spec, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		if name, cmd, ok := strings.Cut(pair, "="); ok {
			out[strings.TrimSpace(name)] = strings.TrimSpace(cmd)
		}
	}

	return out
}

func tailStr(s string, n int) string {
	if len(s) <= n {
		return s
	}

	return s[len(s)-n:]
}
