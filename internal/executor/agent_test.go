package executor

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func agentTaskT(t *testing.T, p CrushPayload) task.Task {
	t.Helper()
	payload, err := RenderCrushPayload(p)
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
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module demo.example.com/x\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
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
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module demo.example.com/x\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
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
	e := &AgentExecutor{Bin: makeStubAgent(t, "sleep 30")}
	err := e.Execute(ctx, agentTaskT(t, AgentPayload{Repo: repo, Prompt: "hi"}))
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("want cancel error, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("cancellation took %s; agent not killed promptly", elapsed)
	}
}

func TestAgentPayloadSafetyFieldsRoundTrip(t *testing.T) {
	raw := `{"repo":"/tmp/r","prompt":"p","verify":"go test ./...","require_clean":false,"timeout_minutes":10}`
	var p CrushPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Verify != "go test ./..." || p.RequireClean == nil || *p.RequireClean || p.TimeoutMinutes != 10 {
		t.Fatalf("round trip lost safety fields: %+v", p)
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
	var got []string
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		got = append(got, line)
	}
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
