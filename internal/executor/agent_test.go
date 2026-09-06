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

// stubAgent writes an executable stand-in for the agent binary that ignores
// its arguments and runs the given shell body with cwd = repo dir.
func stubAgent(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "stub-agent")
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub agent: %v", err)
	}
	return path
}

// initGitRepo creates a committed git repo at dir so the clean-tree guard
// has something real to inspect.
func initGitRepo(t *testing.T, dir string) {
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

func agentTask(t *testing.T, repo, prompt, verify string) task.Task {
	t.Helper()
	payload, err := json.Marshal(AgentPayload{Repo: repo, Prompt: prompt, Verify: verify})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return task.Task{Type: "agent", Payload: payload}
}

func TestAgentPayloadRoundTrip(t *testing.T) {
	raw := `{"repo":"demo","prompt":"fix the bug","verify":"go test ./...","timeout_minutes":10,"yolo":true}`
	var p AgentPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Repo != "demo" || p.Prompt != "fix the bug" || p.Verify != "go test ./..." ||
		p.TimeoutMinutes != 10 || !p.Yolo || p.RequireClean != nil {
		t.Fatalf("round trip lost data: %+v", p)
	}
}

func TestAgentExecutorHappyPath(t *testing.T) {
	ctx := context.Background()
	projects := t.TempDir()
	repo := filepath.Join(projects, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repo)

	ex := &AgentExecutor{ProjectsDir: projects, Bin: stubAgent(t, "echo done > done.txt")}
	if err := ex.Execute(ctx, agentTask(t, "demo", "do the thing", "test -f done.txt")); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "done.txt")); err != nil {
		t.Fatalf("agent work missing: %v", err)
	}
}

func TestAgentExecutorAbsoluteRepoPath(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	initGitRepo(t, repo)

	ex := &AgentExecutor{Bin: stubAgent(t, "true")}
	if err := ex.Execute(ctx, agentTask(t, repo, "hi", "")); err != nil {
		t.Fatalf("Execute with absolute repo: %v", err)
	}
}

func TestAgentExecutorMissingRepo(t *testing.T) {
	ctx := context.Background()
	ex := &AgentExecutor{ProjectsDir: t.TempDir(), Bin: stubAgent(t, "true")}
	err := ex.Execute(ctx, agentTask(t, "nope", "hi", ""))
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("want missing-repo error, got %v", err)
	}
}

func TestAgentExecutorRefusesDirtyTree(t *testing.T) {
	ctx := context.Background()
	projects := t.TempDir()
	repo := filepath.Join(projects, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repo)
	if err := os.WriteFile(filepath.Join(repo, "wip.txt"), []byte("human work"), 0o644); err != nil {
		t.Fatal(err)
	}

	ex := &AgentExecutor{ProjectsDir: projects, Bin: stubAgent(t, "echo should-not-run > ran.txt")}
	err := ex.Execute(ctx, agentTask(t, "demo", "hi", ""))
	if err == nil || !strings.Contains(err.Error(), "uncommitted changes") {
		t.Fatalf("want dirty-tree refusal, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "ran.txt")); err == nil {
		t.Fatal("agent ran despite dirty tree")
	}
}

func TestAgentExecutorDirtyTreeOverride(t *testing.T) {
	ctx := context.Background()
	projects := t.TempDir()
	repo := filepath.Join(projects, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repo)
	if err := os.WriteFile(filepath.Join(repo, "wip.txt"), []byte("human work"), 0o644); err != nil {
		t.Fatal(err)
	}

	payload, _ := json.Marshal(AgentPayload{Repo: "demo", Prompt: "hi", RequireClean: boolPtr(false)})
	ex := &AgentExecutor{ProjectsDir: projects, Bin: stubAgent(t, "true")}
	if err := ex.Execute(ctx, task.Task{Type: "agent", Payload: payload}); err != nil {
		t.Fatalf("Execute with require_clean=false: %v", err)
	}
}

func TestAgentExecutorAgentFailure(t *testing.T) {
	ctx := context.Background()
	projects := t.TempDir()
	repo := filepath.Join(projects, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repo)

	ex := &AgentExecutor{ProjectsDir: projects, Bin: stubAgent(t, "echo boom >&2; exit 3")}
	err := ex.Execute(ctx, agentTask(t, "demo", "hi", ""))
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("want agent failure with output tail, got %v", err)
	}
}

func TestAgentExecutorVerifyFailure(t *testing.T) {
	ctx := context.Background()
	projects := t.TempDir()
	repo := filepath.Join(projects, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repo)

	ex := &AgentExecutor{ProjectsDir: projects, Bin: stubAgent(t, "true")}
	err := ex.Execute(ctx, agentTask(t, "demo", "hi", "false"))
	if err == nil || !strings.Contains(err.Error(), "verify failed") {
		t.Fatalf("want verify failure, got %v", err)
	}
}

func TestAgentExecutorContextCancelKillsAgent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	projects := t.TempDir()
	repo := filepath.Join(projects, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repo)

	start := time.Now()
	ex := &AgentExecutor{ProjectsDir: projects, Bin: stubAgent(t, "sleep 30")}
	err := ex.Execute(ctx, agentTask(t, "demo", "hi", ""))
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("want cancel error, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("cancellation took %s; agent not killed promptly", elapsed)
	}
}

func TestAgentExecutorDefaultVerifyGoRepo(t *testing.T) {
	ctx := context.Background()
	projects := t.TempDir()
	repo := filepath.Join(projects, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module demo.example.com/x\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repo)

	ex := &AgentExecutor{ProjectsDir: projects, Bin: stubAgent(t, "true")}
	if err := ex.Execute(ctx, agentTask(t, "demo", "hi", "")); err != nil {
		t.Fatalf("default go verify should pass on clean module: %v", err)
	}
}

func TestAgentExecutorDefaultVerifyDetectsBreakage(t *testing.T) {
	ctx := context.Background()
	projects := t.TempDir()
	repo := filepath.Join(projects, "demo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module demo.example.com/x\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	initGitRepo(t, repo)
	// Stub agent "breaks" the repo: valid module gains an invalid file.
	ex := &AgentExecutor{ProjectsDir: projects,
		Bin: stubAgent(t, "echo 'package main func broken {' > broken.go")}

	err := ex.Execute(ctx, agentTask(t, "demo", "hi", ""))
	if err == nil || !strings.Contains(err.Error(), "verify failed") {
		t.Fatalf("want default verify to catch broken go code, got %v", err)
	}
}

func TestAgentExecutorRequiresRepoAndPrompt(t *testing.T) {
	ctx := context.Background()
	ex := &AgentExecutor{ProjectsDir: t.TempDir(), Bin: stubAgent(t, "true")}
	err := ex.Execute(ctx, task.Task{Type: "agent", Payload: json.RawMessage(`{"repo":"demo"}`)})
	if err == nil || !strings.Contains(err.Error(), "prompt") {
		t.Fatalf("want missing-prompt error, got %v", err)
	}
}

func boolPtr(b bool) *bool { return &b }
