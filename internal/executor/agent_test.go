//go:build unix

package executor

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// makeStubAgent writes an executable stand-in for the crush binary that
// ignores its arguments and runs body with cwd = repo dir.
func makeStubAgent(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stub-agent")

	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub agent: %v", err)
	}

	return path
}

// setupGitRepo creates a committed git repo at dir so the clean-tree guard
// has something real to inspect.
func setupGitRepo(t *testing.T, dir string) {
	t.Helper()

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir

		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "-q")

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# demo\n"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}

	run("add", "-A")
	run("commit", "-qm", "init")
}

func agentTaskT(t *testing.T, p AgentPayload) task.Task {
	t.Helper()

	payload, err := RenderAgentPayload(p)
	if err != nil {
		t.Fatalf("render payload: %v", err)
	}

	return task.Task{Type: TaskTypeAgent, Payload: payload}
}

func TestAgentExecutorRefusesDirtyTree(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	setupGitRepo(t, repo)

	if err := os.WriteFile(filepath.Join(repo, "wip.txt"), []byte("human work"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := &AgentExecutor{Bin: makeStubAgent(t, "echo should-not-run > ran.txt")}

	err := e.Execute(ctx, agentTaskT(t, AgentPayload{Repo: repo, Prompt: "hi"}))
	if err == nil || !strings.Contains(err.Error(), "uncommitted changes") {
		t.Fatalf("want dirty-tree refusal, got %v", err)
	}

	if _, err := os.Stat(filepath.Join(repo, "ran.txt")); err == nil {
		t.Fatal("agent ran despite dirty tree")
	}
}

func TestAgentExecutorDirtyTreeOverride(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	setupGitRepo(t, repo)

	if err := os.WriteFile(filepath.Join(repo, "wip.txt"), []byte("human work"), 0o644); err != nil {
		t.Fatal(err)
	}

	no := false

	e := &AgentExecutor{Bin: makeStubAgent(t, "true")}
	if err := e.Execute(ctx, agentTaskT(t, AgentPayload{Repo: repo, Prompt: "hi", RequireClean: &no})); err != nil {
		t.Fatalf("Execute with require_clean=false: %v", err)
	}
}

func TestAgentExecutorNonGitRepoSkipsCleanCheck(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()

	e := &AgentExecutor{Bin: makeStubAgent(t, "true")}
	if err := e.Execute(ctx, agentTaskT(t, AgentPayload{Repo: repo, Prompt: "hi"})); err != nil {
		t.Fatalf("non-git repo must skip clean check: %v", err)
	}
}

func TestAgentExecutorVerifyFailure(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	e := &AgentExecutor{Bin: makeStubAgent(t, "true")}

	err := e.Execute(ctx, agentTaskT(t, AgentPayload{Repo: repo, Prompt: "hi", Verify: "false"}))
	if err == nil || !strings.Contains(err.Error(), "verify failed") {
		t.Fatalf("want verify failure, got %v", err)
	}
}

func TestAgentExecutorDefaultVerifyGoRepo(t *testing.T) {
	ctx := context.Background()

	repo := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(repo, "go.mod"),
		[]byte("module demo.example.com/x\n\ngo 1.26\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(repo, "main.go"),
		[]byte("package main\n\nfunc main() {}\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	e := &AgentExecutor{Bin: makeStubAgent(t, "true")}
	if err := e.Execute(ctx, agentTaskT(t, AgentPayload{Repo: repo, Prompt: "hi"})); err != nil {
		t.Fatalf("default go verify should pass on clean module: %v", err)
	}
}

func TestAgentExecutorDefaultVerifyDetectsBreakage(t *testing.T) {
	ctx := context.Background()

	repo := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(repo, "go.mod"),
		[]byte("module demo.example.com/x\n\ngo 1.26\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(
		filepath.Join(repo, "main.go"),
		[]byte("package main\n\nfunc main() {}\n"),
		0o644,
	); err != nil {
		t.Fatal(err)
	}
	// Stub agent "breaks" the repo: valid module gains an invalid file.
	e := &AgentExecutor{Bin: makeStubAgent(t, "echo 'package main func broken {' > broken.go")}

	err := e.Execute(ctx, agentTaskT(t, AgentPayload{Repo: repo, Prompt: "hi"}))
	if err == nil || !strings.Contains(err.Error(), "verify failed") {
		t.Fatalf("want default verify to catch broken go code, got %v", err)
	}
}

func TestAgentExecutorContextCancelKillsAgent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	repo := t.TempDir()

	start := time.Now()
	e := &AgentExecutor{Bin: makeStubAgent(t, "echo evidence-marker-7f3a; sleep 30")}

	err := e.Execute(ctx, agentTaskT(t, AgentPayload{Repo: repo, Prompt: "hi"}))
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("want cancel error, got %v", err)
	}
	// The captured output must survive the cancel: it IS the failure
	// evidence the worker pins into task.failed (regression guard for the
	// go-retry migration, where DoWithValue dropped the value on error).
	if !strings.Contains(err.Error(), "evidence-marker-7f3a") {
		t.Fatalf("err = %v: cancel path lost the captured output tail", err)
	}

	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("cancellation took %s; agent not killed promptly", elapsed)
	}
}

func TestAgentPayloadSafetyFieldsRoundTrip(t *testing.T) {
	raw := `{"repo":"/tmp/r","prompt":"p","verify":"go test ./...","require_clean":false,"timeout_minutes":10}`

	var p AgentPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if p.Verify != "go test ./..." || p.RequireClean == nil || *p.RequireClean || p.TimeoutMinutes != 10 {
		t.Fatalf("round trip lost safety fields: %+v", p)
	}
}

// TestAgentPayloadVersionGate pins the forward-compatibility rule: a
// versioned payload above what this binary understands fails as a
// PERMANENT error (dead-letter, no retry burn), while unversioned payloads
// keep decoding as v1.
func TestAgentPayloadVersionGate(t *testing.T) {
	t.Parallel()

	e := &AgentExecutor{}

	future := task.Task{Type: TaskTypeAgent, Payload: []byte(`{"v":2,"repo":"/tmp/r","prompt":"p"}`)}

	err := e.Execute(context.Background(), future)
	if err == nil || !strings.Contains(err.Error(), "payload version 2") {
		t.Fatalf("want unknown-version permanent error, got %v", err)
	}

	if _, ok := errors.AsType[*PermanentError](err); !ok {
		t.Fatalf("unknown payload version must be permanent, got %v", err)
	}

	v1 := task.Task{Type: TaskTypeAgent, Payload: []byte(`{"repo":"/tmp/nonexistent-v1","prompt":"p"}`)}

	err = e.Execute(context.Background(), v1)
	if err != nil && strings.Contains(err.Error(), "payload version") {
		t.Fatalf("unversioned payload must decode as v1, got %v", err)
	}
}

// TestAgentExecutorArgvContract pins the exact command line handed to the
// agent binary. crush (v0.92) accepts --cwd/--quiet/--model/--session after
// the run subcommand but has NO --yolo flag there — an arg-order regression
// here means every autonomous pool run dies with "Unknown flag" (this exact
// bug shipped once; the stub ignores argv, so only this test catches it).
func TestAgentExecutorArgvContract(t *testing.T) {
	dir := t.TempDir()
	argsLog := filepath.Join(dir, "argv.log")
	bin := filepath.Join(dir, "argv-agent")

	script := "#!/bin/sh\nprintf '%s\n' \"$@\" > \"" + argsLog + "\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, ".crushrc"), []byte("permissions allow view\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := &AgentExecutor{Bin: bin, Yolo: true}
	if err := e.Execute(context.Background(), agentTaskT(t, AgentPayload{
		Repo: repo, Prompt: "do it", Model: "prov/m1",
	})); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	raw, err := os.ReadFile(argsLog)
	if err != nil {
		t.Fatal(err)
	}

	got := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")

	want := []string{"run", "--quiet", "--cwd", repo, "--model", "prov/m1", "--", "do it"}
	if len(got) != len(want) {
		t.Fatalf("argv = %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("argv[%d] = %q, want %q (full: %v)", i, got[i], want[i], got)
		}
	}
}

// TestAgentPromptTaskIDSubstitution pins the {{TASK_ID}} placeholder: the
// queue task ID only exists at execution time, so prompt contracts carry the
// placeholder and the executor must resolve it to the task's real ID before
// the agent runs (agents then put Task-Queue-ID footers in their commits,
// making git log ↔ tq facts cross-reference; 21:40 report §e3).
func TestAgentPromptTaskIDSubstitution(t *testing.T) {
	dir := t.TempDir()
	argsLog := filepath.Join(dir, "argv.log")
	bin := filepath.Join(dir, "argv-agent")

	script := "#!/bin/sh\nprintf '%s\n' \"$@\" > \"" + argsLog + "\"\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, ".crushrc"), []byte("permissions allow view\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := &AgentExecutor{Bin: bin}
	tk := agentTaskT(t, AgentPayload{Repo: repo, Prompt: "work item\n\nTask-Queue-ID: {{TASK_ID}}"})

	tk.ID = task.ID("000001a0fixedidforthesubstitutiontest")
	if err := e.Execute(context.Background(), tk); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	raw, err := os.ReadFile(argsLog)
	if err != nil {
		t.Fatal(err)
	}

	// The stub logs one line per argv element; the substituted prompt is the
	// final argument, so its full text must appear verbatim at the tail.
	want := "work item\n\nTask-Queue-ID: " + tk.ID.String()
	if !strings.HasSuffix(strings.TrimRight(string(raw), "\n"), want) {
		t.Fatalf("argv log = %q, want prompt tail %q ({{TASK_ID}} resolved)", string(raw), want)
	}
}

// TestAgentExecutorYoloWithoutRepoAutonomyFailsFast verifies the guard that
// keeps unattended pools from burning their attempt budget on runs that can
// never act: yolo requested, but the repo has no project-local crush config
// to grant permissions.
func TestAgentExecutorYoloWithoutRepoAutonomyFailsFast(t *testing.T) {
	repo := t.TempDir() // no .crushrc, no .crush.json
	e := &AgentExecutor{Bin: makeStubAgent(t, "echo should-not-run > ran.txt"), Yolo: true}

	err := e.Execute(context.Background(), agentTaskT(t, AgentPayload{Repo: repo, Prompt: "hi"}))
	if err == nil || !strings.Contains(err.Error(), "autonomy") || !strings.Contains(err.Error(), ".crushrc") {
		t.Fatalf("want autonomy guidance error, got %v", err)
	}

	if _, err := os.Stat(filepath.Join(repo, "ran.txt")); err == nil {
		t.Fatal("agent ran despite missing autonomy config")
	}
}

// TestExecWithTransientRetry pins the ETXTBSY absorption: transient
// "text file busy" exec failures retry (kernel 7.2 reproduced them with no
// writer holding the file), everything else passes through on attempt one.
func TestExecWithTransientRetry(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		errs       []error
		wantCalls  int
		wantErr    bool
		wantOutput string
	}{
		{name: "success first try", errs: nil, wantCalls: 1, wantOutput: "ok"},
		{
			name:       "etxtbsy then success",
			errs:       []error{syscall.ETXTBSY},
			wantCalls:  2,
			wantOutput: "ok",
		},
		{
			name:       "three etxtbsy give up",
			errs:       []error{syscall.ETXTBSY, syscall.ETXTBSY, syscall.ETXTBSY},
			wantCalls:  3,
			wantErr:    true,
			wantOutput: "partial",
		},
		{
			name:      "other errno passes through",
			errs:      []error{syscall.ENOENT},
			wantCalls: 1,
			wantErr:   true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			calls := 0
			out, err := execWithTransientRetry(func() (string, error) {
				calls++
				if calls-1 < len(tt.errs) {
					return "partial", tt.errs[calls-1]
				}

				return tt.wantOutput, nil
			})

			if calls != tt.wantCalls {
				t.Errorf("calls = %d, want %d", calls, tt.wantCalls)
			}

			if tt.wantErr && tt.name == "three etxtbsy give up" && !errors.Is(err, syscall.ETXTBSY) {
				// Wrap-chain guard: retry exhaustion must keep the original
				// errno reachable via errors.Is so callers can classify it.
				t.Errorf("err = %v: ETXTBSY no longer reachable through the wrap chain", err)
			}

			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil && out != tt.wantOutput {
				t.Errorf("out = %q, want %q", out, tt.wantOutput)
			}
		})
	}
}

// TestAgentInputContractMissesArePermanent pins the money-saving rule: any
// failure that a retry would reproduce verbatim must come back as a
// *PermanentError so the worker dead-letters after ONE attempt.
func TestAgentInputContractMissesArePermanent(t *testing.T) {
	ctx := context.Background()
	e := &AgentExecutor{}

	cases := []struct {
		name string
		tk   task.Task
	}{
		{"empty payload", task.Task{Type: TaskTypeAgent}},
		{"bad json", task.Task{Type: TaskTypeAgent, Payload: []byte("{oops")}},
		{"missing repo field", task.Task{Type: TaskTypeAgent, Payload: []byte(`{"prompt":"p"}`)}},
	}
	for _, tc := range cases {
		err := e.Execute(ctx, tc.tk)
		if _, ok := errors.AsType[*PermanentError](err); !ok {
			t.Errorf("%s: want permanent, got %v", tc.name, err)
		}
	}

	missingDir := filepath.Join(t.TempDir(), "missing")

	payload, _ := RenderAgentPayload(AgentPayload{Repo: missingDir, Prompt: "hi"})
	if _, ok := errors.AsType[*PermanentError](e.Execute(ctx, task.Task{Type: TaskTypeAgent, Payload: payload})); !ok {
		t.Error("missing repo dir: want permanent (retrying cannot create it)")
	}

	// Agent-run and verify failures stay transient: a fresh attempt is the fix.
	repo := t.TempDir()

	flaky := &AgentExecutor{Bin: makeStubAgent(t, "false")}
	if _, ok := errors.AsType[*PermanentError](
		flaky.Execute(ctx, agentTaskT(t, AgentPayload{Repo: repo, Prompt: "hi"})),
	); ok {
		t.Error("agent run failure must stay transient")
	}

	verifyFail := &AgentExecutor{Bin: makeStubAgent(t, "true")}
	if _, ok := errors.AsType[*PermanentError](
		verifyFail.Execute(ctx, agentTaskT(t, AgentPayload{Repo: repo, Prompt: "hi", Verify: "false"})),
	); ok {
		t.Error("verify failure must stay transient")
	}
}

// TestAgentDirtyTreeAndAutonomyArePreflight pins the preflight classes: a
// dirty tree and a missing autonomy config are ENVIRONMENT problems, not
// task problems — the executor must refuse to start with a *PreflightError
// so the worker requeues without burning an attempt (the human may commit
// or add a config any minute). A global crush config satisfies the probe.
func TestAgentDirtyTreeAndAutonomyArePreflight(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	setupGitRepo(t, repo)

	if err := os.WriteFile(filepath.Join(repo, "wip.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := &AgentExecutor{Bin: makeStubAgent(t, "true")}

	err := e.Execute(ctx, agentTaskT(t, AgentPayload{Repo: repo, Prompt: "hi"}))
	if _, ok := errors.AsType[*PreflightError](err); !ok {
		t.Errorf("dirty tree must be preflight, got %v", err)
	}

	autonomyRepo := t.TempDir()

	err = e.Execute(ctx, agentTaskT(t, AgentPayload{Repo: autonomyRepo, Prompt: "hi", Yolo: true}))
	if _, ok := errors.AsType[*PreflightError](err); !ok {
		t.Errorf("missing autonomy config must be preflight, got %v", err)
	}

	// D32: the user-global crush config satisfies the autonomy probe.
	restore := userGlobalCrushConfig
	userGlobalCrushConfig = func() bool { return true }

	t.Cleanup(func() { userGlobalCrushConfig = restore })

	if err := e.Execute(ctx, agentTaskT(t, AgentPayload{Repo: autonomyRepo, Prompt: "hi", Yolo: true})); err != nil {
		t.Errorf("global crush config must satisfy the autonomy probe, got %v", err)
	}
}

// TestVerifyStrategy pins the verify precedence: the repo's .tq-verify file
// wins over the payload and over auto-detection; auto-detection maps stack
// markers to real gate commands (a repo with a Makefile but no test target
// fails the gate, by design — no vacuous passes).
func TestVerifyStrategy(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string]string
		wantCmd string
	}{
		{
			"go module",
			map[string]string{"go.mod": "module x\n"},
			"go build ./... && go test ./... -count=1" +
				" && for f in $(find . -mindepth 2 -name go.mod -not -path '*/vendor/*');" +
				" do (cd \"${f%/*}\" && go build ./... && go test ./... -count=1) || exit 1; done",
		},
		// Multi-module detection is root-marker based: the command is the
		// same regardless of nested go.mod files, and the table writer
		// cannot create nested dirs. The nested-module behavior is
		// exercised for real by TestDefaultVerifyCoversNestedModules.
		{"package.json", map[string]string{"package.json": "{}"}, "npm test --silent"},
		{"makefile", map[string]string{"Makefile": "all:\n\ttrue\n"}, "make test"},
		{"flake", map[string]string{"flake.nix": "{}"}, "nix build && nix flake check"},
		{"cargo", map[string]string{"Cargo.toml": "[package]\n"}, "cargo test --quiet"},
		{"unknown stack detects nothing", nil, ""},
	}
	for _, tc := range cases {
		dir := t.TempDir()
		for f, c := range tc.files {
			if err := os.WriteFile(filepath.Join(dir, f), []byte(c), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		if got := autoDetectVerify(dir); got != tc.wantCmd {
			t.Errorf("%s: autoDetectVerify = %q, want %q", tc.name, got, tc.wantCmd)
		}
	}

	// Precedence: file wins over payload, payload wins over detection.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".tq-verify"), []byte("  false  \n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := verifyFor(dir, &AgentPayload{Verify: "true"})
	if got != "false" {
		t.Fatalf("verifyFor with file = %q, want the file command (payload and detection lose)", got)
	}

	// End-to-end: the file's command actually gates the run (verify fails).
	e := &AgentExecutor{Bin: makeStubAgent(t, "true")}

	err := e.Execute(context.Background(), agentTaskT(t, AgentPayload{Repo: dir, Prompt: "hi", Verify: "true"}))
	if err == nil || !strings.Contains(err.Error(), `verify failed ("false")`) {
		t.Fatalf(".tq-verify file must win and gate the run, got %v", err)
	}

	// No file, no payload, no markers: verify is a no-op, not an error.
	empty := t.TempDir()
	if err := e.Execute(context.Background(), agentTaskT(t, AgentPayload{Repo: empty, Prompt: "hi"})); err != nil {
		t.Fatalf("verify-less repo must pass when the agent succeeds, got %v", err)
	}
}

func TestAgentVersion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	stub := filepath.Join(dir, "versioned-agent")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\necho 'crush v0.92.1'\necho 'extra line'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	version, err := AgentVersion(context.Background(), stub)
	if err != nil {
		t.Fatalf("AgentVersion: %v", err)
	}

	if version != "crush v0.92.1" {
		t.Errorf("version = %q, want first stdout line only", version)
	}

	if _, err := AgentVersion(context.Background(), filepath.Join(dir, "missing")); err == nil {
		t.Error("missing binary must error, not return empty success")
	}
}

// TestMachineWideAgentCapSerializes: with MaxConcurrent=1 two concurrent
// agent executions must not overlap (flock slots), and both still succeed.
func TestMachineWideAgentCapSerializes(t *testing.T) {
	// Not parallel: t.Setenv pins TQ_AGENT_SLOT_DIR.
	t.Setenv("TQ_AGENT_SLOT_DIR", t.TempDir())

	dir := t.TempDir()
	log := filepath.Join(dir, "run.log")

	// Each run logs start, waits, logs end: overlapping runs interleave
	// start/start before end/end.
	stub := filepath.Join(dir, "slow-agent")

	script := "#!/bin/sh\necho start >> " + log + "\nsleep 0.4\necho end >> " + log + "\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	exec := &AgentExecutor{Bin: stub, MaxConcurrent: 1}

	run := func() error {
		payload, err := RenderAgentPayload(AgentPayload{Repo: dir, Prompt: "go"})
		if err != nil {
			return err
		}

		return exec.Execute(context.Background(), task.Task{ID: "t-cap", Type: TaskTypeAgent, Payload: payload})
	}

	var wg sync.WaitGroup

	errs := make(chan error, 2)

	for range 2 {
		wg.Go(func() {
			errs <- run()
		})
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("execute under cap: %v", err)
		}
	}

	body, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(body)), "\n")

	// Serialized: strictly start,end,start,end. Overlap would show two
	// adjacent "start" lines.
	want := []string{"start", "end", "start", "end"}
	if strings.Join(lines, ",") != strings.Join(want, ",") {
		t.Fatalf("run log = %v, want %v (cap must serialize runs)", lines, want)
	}
}

func TestDefaultVerifyCoversNestedModules(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		t.Helper()

		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module x\n\ngo 1.26\n")
	write("x.go", "package main\n\nfunc main() {}\n")
	write("sub/go.mod", "module x/sub\n\ngo 1.26\n")
	write("sub/sub.go", "package sub\n\nconst OK = true\n")
	write("vendor/keep.txt", "")

	cmdStr := defaultVerify(dir)
	if cmdStr == "" {
		t.Fatal("defaultVerify returned empty for a Go repo")
	}

	// The default command must reject a failing nested module test: the
	// root ./... gate cannot even see it.
	write(
		"sub/sub_fail_test.go",
		"package sub\n\nimport \"testing\"\n\nfunc TestBroken(t *testing.T) { t.Fatal(\"broken\") }\n",
	)

	if err := runIn(dir, cmdStr); err == nil {
		t.Fatal("verify passed despite a failing nested-module test")
	}

	write("sub/sub_fail_test.go", "package sub\n\nimport \"testing\"\n\nfunc TestOK(t *testing.T) {}\n")

	out, err := runInOutput(dir, cmdStr)
	if err != nil {
		t.Fatalf("verify failed on a healthy multi-module tree: %v\n%s", err, out)
	}

	if !strings.Contains(out, "x/sub") {
		t.Fatalf("verify output lacks evidence the nested module was tested:\n%s", out)
	}
}

// TestAgentExecutorCloseoutTurn pins the two-turn contract: with
// CloseoutPrompt set, the work turn runs verbose (for the session id) and a
// second run resumes the exact session with the resolved close-out prompt;
// without it, exactly one quiet run happens (the argv contract above).
func TestAgentExecutorCloseoutTurn(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	argsLog := filepath.Join(dir, "argv.log")
	bin := filepath.Join(dir, "closeout-agent")

	script := `#!/bin/sh
printf '%s\n' "$*" >> "` + argsLog + `"
if [ "$(wc -l < "` + argsLog + `")" = "1" ]; then
	printf 'INFO Created session for non-interactive run session_id=sess-1234\n'
fi
printf 'TQ_RESULT: {"files_changed":["x.go"]}\n'
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, ".crushrc"), []byte("permissions allow view\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := &AgentExecutor{Bin: bin, Yolo: true, CloseoutPrompt: "closeout review {{TASK_ID}}"}
	if err := e.Execute(context.Background(), agentTaskT(t, AgentPayload{Repo: repo, Prompt: "do it"})); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	raw, err := os.ReadFile(argsLog)
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 agent invocations (work + closeout), got %d: %q", len(lines), lines)
	}

	if !strings.Contains(lines[0], "--verbose") || strings.Contains(lines[0], "--session") {
		t.Fatalf("work turn must run verbose without --session: %q", lines[0])
	}

	if !strings.Contains(lines[1], "--session sess-1234") || !strings.Contains(lines[1], "closeout review ") {
		t.Fatalf("closeout turn must resume the extracted session with the resolved prompt: %q", lines[1])
	}
}

// runIn runs a shell line inside dir. The caller's cwd must never leak in:
// from this package's dir the verify line would re-run this very suite,
// recursing until the test timeout.
func runIn(dir, cmdLine string) error {
	cmd := exec.Command("sh", "-c", cmdLine)
	cmd.Dir = dir

	return cmd.Run()
}

// TestAgentExecutorRateLimitClassifiedAndGated pins the provider-exhaustion
// contract end-to-end: a run whose output reports a 429 usage limit (the
// Z.ai shape from the 2026-09-11 dead-lettered task) returns a
// *RateLimitError carrying the parsed wait — NOT a plain failure — and the
// executor's gate then refuses the NEXT run without invoking the agent
// binary again, until the gate window elapses.
func TestAgentExecutorRateLimitClassifiedAndGated(t *testing.T) {
	repo := t.TempDir()
	setupGitRepo(t, repo)

	reset := time.Now().Add(2 * time.Hour).Format("2006-01-02 15:04:05")
	stub := makeStubAgent(t, fmt.Sprintf(
		`echo 'WARN Provider request failed, retrying retry_delay=5s status_code=429 title="too many requests" message="Usage limit reached for 5 hour. Your limit will reset at %s"'; echo ran >> ran.log; exit 1`,
		reset,
	))

	e := &AgentExecutor{Bin: stub}

	err := e.Execute(context.Background(), agentTaskT(t, AgentPayload{Repo: repo, Prompt: "hi"}))
	if err == nil {
		t.Fatal("Execute must fail on a rate-limited run")
	}

	rl, ok := errors.AsType[*RateLimitError](err)
	if !ok {
		t.Fatalf("err = %v (%T), want *RateLimitError", err, err)
	}

	if rl.RetryAfter <= 0 || rl.RetryAfter > 2*time.Hour+rateLimitGrace+time.Minute {
		t.Fatalf("RetryAfter = %s, want ~2h + grace", rl.RetryAfter)
	}

	if !strings.Contains(rl.Error(), "429") {
		t.Fatalf("error text must keep the 429 evidence: %q", rl.Error())
	}

	// While the gate holds, the next Execute must refuse WITHOUT spawning
	// the stub (the ran.log line count stays at 1).
	err = e.Execute(context.Background(), agentTaskT(t, AgentPayload{Repo: repo, Prompt: "hi"}))

	gated, ok := errors.AsType[*RateLimitError](err)
	if !ok {
		t.Fatalf("gated err = %v (%T), want *RateLimitError", err, err)
	}

	if gated.RetryAfter <= 0 || gated.RetryAfter > 2*time.Hour+rateLimitGrace {
		t.Fatalf("gated RetryAfter = %s, want the remaining window", gated.RetryAfter)
	}

	ran, err := os.ReadFile(filepath.Join(repo, "ran.log"))
	if err != nil {
		t.Fatal(err)
	}

	if lines := strings.Count(strings.TrimRight(string(ran), "\n"), "\n") + 1; lines != 1 {
		t.Fatalf("agent binary ran %d times, want 1 (gate must refuse without spawning)", lines)
	}

	// Gate elapsed: the executor probes again (the stub runs, fails, and
	// re-arms the gate from fresh evidence).
	e.rateLimitUntil.Store(time.Now().Add(-time.Second).UnixNano())

	if err := e.Execute(context.Background(), agentTaskT(t, AgentPayload{Repo: repo, Prompt: "hi"})); err == nil {
		t.Fatal("post-gate run must re-classify from fresh evidence")
	} else if _, ok := errors.AsType[*RateLimitError](err); !ok {
		t.Fatalf("post-gate err = %v, want *RateLimitError from a fresh probe", err)
	}
}

func runInOutput(dir, cmdLine string) (string, error) {
	cmd := exec.Command("sh", "-c", cmdLine)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()

	return string(out), err
}
