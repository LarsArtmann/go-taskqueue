package main

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func writeRepo(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

func readRepo(t *testing.T, dir, name string) string {
	t.Helper()

	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}

	return string(b)
}

func TestEnsureCrushConfigCreatesIdempotentBlocks(t *testing.T) {
	repo := writeRepo(t, map[string]string{})

	o := bootstrapOptions{model: "zai/glm-5.3-flash", reasoning: "xhigh"}

	changed, err := o.ensureCrushConfig(repo)
	if err != nil || !changed {
		t.Fatalf("first ensure: changed=%v err=%v", changed, err)
	}

	got := readRepo(t, repo, ".crushrc")
	for _, want := range []string{
		"permissions allow view ls grep glob edit write bash",
		"model large zai/glm-5.3-flash --reasoning-effort xhigh",
		tqManagedStart, tqManagedEnd,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}

	// Second run is a byte-identical no-op.
	changed, err = o.ensureCrushConfig(repo)
	if err != nil || changed {
		t.Fatalf("second ensure: changed=%v err=%v (want false)", changed, err)
	}
}

func TestEnsureCrushConfigPreservesUserContentAndReplacesBlock(t *testing.T) {
	old := "# my human notes\n" + tqManagedStart + "\npermissions allow view\n" + tqManagedEnd + "\n"
	repo := writeRepo(t, map[string]string{".crushrc": old})

	o := bootstrapOptions{}

	if _, err := o.ensureCrushConfig(repo); err != nil {
		t.Fatal(err)
	}

	got := readRepo(t, repo, ".crushrc")
	if !strings.Contains(got, "# my human notes") {
		t.Fatalf("user content lost:\n%s", got)
	}

	if strings.Contains(got, "permissions allow view\n") || strings.Count(got, tqManagedStart) != 1 {
		t.Fatalf("old managed block not replaced:\n%s", got)
	}

	if !strings.Contains(got, "permissions allow view ls grep glob edit write bash") {
		t.Fatalf("new block missing:\n%s", got)
	}
}

func TestEnsureTQVerifyOverrideAlwaysRewrites(t *testing.T) {
	repo := writeRepo(t, map[string]string{".tq-verify": "go test ./..."})

	o := bootstrapOptions{verify: map[string]string{filepath.Base(repo): "bash scripts/gate.sh"}}

	action, cmd, err := o.ensureTQVerify(repo)
	if err != nil || action != verifyWrote || cmd != "bash scripts/gate.sh" {
		t.Fatalf("override: action=%q cmd=%q err=%v", action, cmd, err)
	}

	if got := readRepo(t, repo, ".tq-verify"); got != "bash scripts/gate.sh\n" {
		t.Fatalf("override not written: %q", got)
	}

	// Dry-run reports the override intent without writing.
	o.dryRun = true

	if err := os.WriteFile(filepath.Join(repo, ".tq-verify"), []byte("go test ./...\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	action, cmd, err = o.ensureTQVerify(repo)
	if err != nil || action != verifyOverride || cmd != "bash scripts/gate.sh" {
		t.Fatalf("dry-run override: action=%q cmd=%q err=%v", action, cmd, err)
	}

	if got := readRepo(t, repo, ".tq-verify"); got != "go test ./...\n" {
		t.Fatalf("dry-run wrote anyway: %q", got)
	}
}

func TestEnsureTQVerifyKeepsExistingDetectsMissing(t *testing.T) {
	repo := writeRepo(t, map[string]string{"go.mod": "module example.com/x\n"})

	o := bootstrapOptions{}

	action, cmd, err := o.ensureTQVerify(repo)
	if err != nil || action != verifyWrote || cmd == "" {
		t.Fatalf("detect+write: action=%q cmd=%q err=%v", action, cmd, err)
	}

	if got := readRepo(t, repo, ".tq-verify"); got != cmd+"\n" {
		t.Fatalf("detected command not pinned: %q vs %q", got, cmd)
	}

	// Existing file (no override) is kept untouched.
	if action, cmd, err = o.ensureTQVerify(repo); action != verifyKept || cmd == "" || err != nil {
		t.Fatalf("existing kept: action=%q cmd=%q err=%v", action, cmd, err)
	}

	// No markers at all: honest "none".
	empty := writeRepo(t, map[string]string{"TODO_LIST.md": "# TODO\n"})

	action, cmd, err = o.ensureTQVerify(empty)
	if err != nil || action != verifyNone || cmd != "" {
		t.Fatalf("no verify: action=%q cmd=%q err=%v", action, cmd, err)
	}
}

func TestEnsureCommittedOnlyStagedFiles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	repo := writeRepo(t, map[string]string{".crushrc": "permissions allow view\n", "unrelated.txt": "keep me dirty\n"})

	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "t"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}

	committed, err := ensureCommitted(repo, []string{".crushrc"})
	if err != nil || !committed {
		t.Fatalf("commit: committed=%v err=%v", committed, err)
	}

	out, err := exec.Command("git", "-C", repo, "status", "--porcelain").CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(out), "unrelated.txt") || strings.Contains(string(out), ".crushrc") {
		t.Fatalf("wrong staging: %s", out)
	}

	if committed, err = ensureCommitted(repo, []string{".crushrc"}); committed || err != nil {
		t.Fatalf("second commit not a no-op: committed=%v err=%v", committed, err)
	}
}

func TestComposePoolArgs(t *testing.T) {
	o := bootstrapOptions{
		repos:       []string{"/home/me/projects/CV", "/home/me/projects/SystemNix"},
		projectsDir: "/home/me/projects",
		agents:      2,
		model:       "zai/glm-5.3-flash",
		reasoning:   "xhigh",
		interval:    5 * time.Minute,
		dailyBudget: 20,
		maxPerTick:  3,
		yolo:        true, review: true, reviewAutofix: true, exclusive: true,
		logDir: "/state/tq/logs",
		db:     "/tmp/tq.db",
	}

	got := strings.Join(composePoolArgs(o), " ")

	for _, want := range []string{
		"--repos /home/me/projects/CV,/home/me/projects/SystemNix",
		"--concurrency 2", "--max-concurrent-agents 2",
		"--daily-budget 20", "--max-per-tick 3",
		"--yolo=true", "--review=true", "--review-autofix=true",
		"--project-exclusive=true", "--allow-dirty=false",
		"--db /tmp/tq.db",
		"--log-dir /state/tq/logs",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in: %s", want, got)
		}
	}

	// The pool must NOT carry --model: the repo .crushrc slot (model +
	// reasoning effort) is the only effort-carrying mechanism, and a payload
	// model makes the executor pass `crush run -m`, which resets effort.
	if strings.Contains(got, "--model") {
		t.Fatalf("--model must not be composed into pool args (drops reasoning effort): %s", got)
	}

	o.once = true

	if !strings.Contains(strings.Join(composePoolArgs(o), " "), "--once") {
		t.Fatal("missing --once")
	}
}

func TestRenderPoolConfig(t *testing.T) {
	o := bootstrapOptions{
		repos: []string{"/p/CV"}, projectsDir: "/p", agents: 2,
		interval: 5 * time.Minute, dailyBudget: 20, maxPerTick: 3,
		yolo: true, review: true, reviewAutofix: true, exclusive: true,
		model: "zai/glm-5.3-flash", logDir: "/state/tq/logs",
	}

	got := renderPoolConfig(o)

	for _, want := range []string{
		"projects-dir = /p", "repos = /p/CV", "concurrency = 2",
		"daily-budget = 20", "yolo = true",
		"log-dir = /state/tq/logs",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}

	// No model key: the .crushrc managed block carries model + reasoning
	// effort; a pool-config model would drop the effort (crush run -m).
	if strings.Contains(got, "model") {
		t.Fatalf("model must not render into pool.conf (drops reasoning effort):\n%s", got)
	}

	o.logDir = ""

	if strings.Contains(renderPoolConfig(o), "log-dir") {
		t.Fatal("log-dir rendered while empty (sidecars must stay off)")
	}
}

func TestRenderUnitMatchesDeployFile(t *testing.T) {
	deploy, err := os.ReadFile(filepath.Join("..", "..", "deploy", "systemd", "tq-agent-pool.service"))
	if err != nil {
		t.Skipf("deploy unit not readable from test cwd: %v", err)
	}

	normalize := func(s string) string {
		var keep []string

		for line := range strings.SplitSeq(s, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}

			if strings.HasPrefix(trimmed, "ExecStart=") {
				line = "ExecStart=<BIN>"
			}

			keep = append(keep, line)
		}

		return strings.Join(keep, "\n")
	}

	rendered := normalize(renderUnit("<BIN>"))

	tracked := normalize(string(deploy))
	if rendered != tracked {
		t.Fatalf(
			"embedded unit drifted from deploy/systemd file\n--- rendered ---\n%s\n--- tracked ---\n%s",
			rendered,
			tracked,
		)
	}
}

func TestInstallServiceRendersUnitAndConfigWithoutTouchingSystem(t *testing.T) {
	// The 21:40 window flagged that `tq bootstrap --install` had no test
	// pinning WHAT it renders nor THAT it stops at the user-session boundary.
	// Stubbed systemctl/loginctl record every call so the test proves the
	// unit + pool.conf land in $HOME and nothing else runs.
	// The stubs are #!/bin/sh scripts and the installed unit is a systemd
	// user unit — POSIX-only, so skip under the windows CI job.
	if runtime.GOOS == "windows" {
		t.Skip("installs a systemd user unit via #!/bin/sh stubs (POSIX-only)")
	}

	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	stubBin := t.TempDir()
	callLog := filepath.Join(stubBin, "calls.log")
	for _, name := range []string{"systemctl", "loginctl"} {
		script := "#!/bin/sh\necho \"$0 $@\" >> " + callLog + "\nexit 0\n"
		path := filepath.Join(stubBin, name)
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", stubBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	o := bootstrapOptions{
		repos:       []string{"CV", "SystemNix"},
		projectsDir: fakeHome + "/projects",
		agents:      2,
		interval:    5 * time.Minute,
		dailyBudget: 40,
		maxPerTick:  6,
		yolo:        true,
		review:      true,
		exclusive:   true,
		allowDirty:  true,
		logDir:      fakeHome + "/.local/state/tq/logs",
		binPath:     "/nix/store/xxx-go-taskqueue-0.1.0/bin/tq",
	}

	if err := o.installService(); err != nil {
		t.Fatalf("installService: %v", err)
	}

	conf := readRepo(t, filepath.Join(fakeHome, ".config", "tq"), "pool.conf")
	for _, want := range []string{
		"repos = CV,SystemNix",
		"concurrency = 2",
		"max-concurrent-agents = 2",
		"interval = 5m0s",
		"daily-budget = 40",
		"max-per-tick = 6",
		"yolo = true",
		"review = true",
		"project-exclusive = true",
		"allow-dirty = true",
		"log-dir = " + fakeHome + "/.local/state/tq/logs",
	} {
		if !strings.Contains(conf, want) {
			t.Errorf("pool.conf missing %q in:\n%s", want, conf)
		}
	}
	// The deliberate no-model rule: repo .crushrc owns model + reasoning.
	if strings.Contains(conf, "model") {
		t.Errorf("pool.conf must not pin a model (repo .crushrc owns it):\n%s", conf)
	}

	unit := readRepo(t, filepath.Join(fakeHome, ".config", "systemd", "user"), "tq-agent-pool.service")
	for _, want := range []string{
		// The template keeps systemd's %h home specifier (not the absolute
		// path) so the unit survives home-dir moves.
		"ExecStart=/nix/store/xxx-go-taskqueue-0.1.0/bin/tq agent-pool --config %h/.config/tq/pool.conf",
		"KillSignal=SIGINT",
		"TimeoutStopSec=45min",
		"KillMode=process",
		"ProtectSystem=full",
		"Restart=on-failure",
	} {
		if !strings.Contains(unit, want) {
			t.Errorf("unit missing %q in:\n%s", want, unit)
		}
	}

	calls, err := os.ReadFile(callLog)
	if err != nil {
		t.Fatalf("stubbed commands never ran: %v", err)
	}
	got := string(calls)
	for _, want := range []string{
		"systemctl --user daemon-reload",
		"systemctl --user enable --now tq-agent-pool.service",
		"loginctl enable-linger",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected call %q, got:\n%s", want, got)
		}
	}

	info, err := os.Stat(filepath.Join(fakeHome, ".config", "tq", "pool.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("pool.conf mode = %o, want 600 (contains repo layout)", mode)
	}
}

func TestParseBootstrapArgsValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"no repos", []string{}, "no repos"},
		{"model needs provider", []string{"--model", "glm-5.3-flash", "CV"}, "provider/model"},
		{"bad reasoning", []string{"--model", "zai/glm", "--reasoning", "max", "CV"}, "low|medium|high|xhigh"},
		{"install vs once", []string{"--install", "--once", "CV"}, "daemon mode"},
		{"install vs dry-run", []string{"--install", "--dry-run", "CV"}, "contradictory"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseBootstrapArgs(tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("want error containing %q, got %v", tt.wantErr, err)
			}
		})
	}

	o, err := parseBootstrapArgs([]string{"--repos", "a,b", "--agents", "3", "--no-review", "c"})
	if err != nil {
		t.Fatal(err)
	}

	if len(o.repos) != 3 || o.repos[2] != "c" {
		t.Fatalf("repos not merged: %v", o.repos)
	}

	if o.agents != 3 || o.reasoning != "xhigh" || !o.yolo || o.review || !o.reviewAutofix || !o.exclusive {
		t.Fatalf("unexpected defaults: %+v", o)
	}
}

func TestReorderBootstrapArgsMixedOrder(t *testing.T) {
	fs := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
	agents := fs.Int("agents", 1, "")
	model := fs.String("model", "", "")
	once := fs.Bool("once", false, "")

	args := reorderBootstrapArgs(
		fs,
		[]string{"CV", "--agents", "2", "SystemNix", "--model", "zai/glm", "--once", "go-taskqueue"},
	)

	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}

	if got := fs.Args(); strings.Join(got, ",") != "CV,SystemNix,go-taskqueue" {
		t.Fatalf("positionals wrong: %v", got)
	}

	if *agents != 2 || *model != "zai/glm" || !*once {
		t.Fatalf("flags not parsed: agents=%d model=%q once=%v", *agents, *model, *once)
	}
}

func TestBootstrapDryRunWritesNothing(t *testing.T) {
	projects := t.TempDir()
	repo := writeRepo(t, map[string]string{
		"TODO_LIST.md": "# TODO\n\n- [ ] fix the thing\n- [x] done thing\n",
	})

	rename := filepath.Join(projects, "demo")
	if err := os.Rename(repo, rename); err != nil {
		t.Fatal(err)
	}

	err := cmdBootstrap([]string{
		"--projects-dir", projects,
		"--projects-dir", projects, // idempotent flag handling sanity
		"--repos", "demo",
		"--dry-run",
		"--db", filepath.Join(t.TempDir(), "tasks.db"),
	})
	if err != nil {
		t.Fatalf("dry-run bootstrap: %v", err)
	}

	if _, err := os.Stat(filepath.Join(rename, ".crushrc")); !os.IsNotExist(err) {
		t.Fatal("dry-run wrote .crushrc")
	}

	if _, err := os.Stat(filepath.Join(rename, ".tq-verify")); !os.IsNotExist(err) {
		t.Fatal("dry-run wrote .tq-verify")
	}
}
