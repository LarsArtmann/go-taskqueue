package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// AgentPayload is the payload contract for the "agent" task type: run a
// headless AI coding agent in a repository, then prove the result.
type AgentPayload struct {
	// Repo names a directory under the executor's ProjectsDir (or an absolute
	// path). Required.
	Repo string `json:"repo"`
	// Prompt is the instruction handed to the agent. Required.
	Prompt string `json:"prompt"`
	// Verify is a shell command that must exit 0 for the task to complete.
	// Empty means auto-detect: Go repositories run
	// "go build ./... && go test ./... -count=1", others run nothing.
	Verify string `json:"verify,omitempty"`
	// TimeoutMinutes caps the agent run (agent and verify combined). Default
	// 30; the worker's task timeout still applies as a hard ceiling.
	TimeoutMinutes int `json:"timeout_minutes,omitempty"`
	// Yolo runs the agent with auto-accepted permissions (crush --yolo).
	// Default false: the agent runs with default permissions.
	Yolo bool `json:"yolo,omitempty"`
	// RequireClean refuses to start unless the repo's git tree is clean.
	// Default true — the pool must never trample human work-in-progress.
	RequireClean *bool `json:"require_clean,omitempty"`
}

const defaultAgentTimeout = 30 * time.Minute

// AgentExecutor runs one headless AI coding agent per task.
//
// The agent binary defaults to "crush" (overridable via the TQ_AGENT_BIN
// environment variable or the Bin field). The working directory is the
// resolved repo; the prompt is passed as the non-interactive run argument.
// After the agent exits, the verify command runs in the same directory; the
// task only completes when it exits 0.
type AgentExecutor struct {
	// ProjectsDir is the root under which relative Repo names resolve.
	ProjectsDir string
	// Bin is the agent binary. Default "crush".
	Bin string
	// BinArgs are extra arguments inserted before the run subcommand.
	BinArgs []string
}

// NewAgentExecutor builds an AgentExecutor for a projects directory. The
// binary is "crush" unless TQ_AGENT_BIN is set.
func NewAgentExecutor(projectsDir string) *AgentExecutor {
	bin := os.Getenv("TQ_AGENT_BIN")
	if bin == "" {
		bin = "crush"
	}
	return &AgentExecutor{ProjectsDir: projectsDir, Bin: bin}
}

// Execute resolves the payload, guards the repo, runs the agent, then runs
// the verify command. Any miss returns an error (a failed attempt).
func (e *AgentExecutor) Execute(ctx context.Context, t task.Task) error {
	var p AgentPayload
	if len(t.Payload) > 0 {
		if err := json.Unmarshal(t.Payload, &p); err != nil {
			return fmt.Errorf("agent: parse payload: %w", err)
		}
	}
	if p.Repo == "" || p.Prompt == "" {
		return errors.New("agent: payload requires \"repo\" and \"prompt\"")
	}

	repoDir, err := e.repoDir(p.Repo)
	if err != nil {
		return err
	}

	if requireClean(p) {
		if err := assertCleanTree(ctx, repoDir); err != nil {
			return err
		}
	}

	timeout := defaultAgentTimeout
	if p.TimeoutMinutes > 0 {
		timeout = time.Duration(p.TimeoutMinutes) * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	agentErr := e.runAgent(runCtx, repoDir, p)
	if agentErr != nil {
		return agentErr
	}
	return e.runVerify(runCtx, repoDir, p)
}

// repoDir resolves a payload repo name to an existing directory.
func (e *AgentExecutor) repoDir(repo string) (string, error) {
	dir := repo
	if !filepath.IsAbs(dir) {
		if e.ProjectsDir == "" {
			return "", fmt.Errorf("agent: relative repo %q needs a projects dir", repo)
		}
		dir = filepath.Join(e.ProjectsDir, repo)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("agent: repo %q: %w", repo, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("agent: repo %q is not a directory", repo)
	}
	return dir, nil
}

func requireClean(p AgentPayload) bool {
	if p.RequireClean == nil {
		return true
	}
	return *p.RequireClean
}

// assertCleanTree fails unless the repo has no uncommitted changes. An
// unusable git is also a failure: without git there is no undo for whatever
// the agent does.
func assertCleanTree(ctx context.Context, repoDir string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "status", "--porcelain")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("agent: git status failed in %s: %v: %s", repoDir, err, tailBytes(out.Bytes(), 512))
	}
	if s := strings.TrimSpace(out.String()); s != "" {
		return fmt.Errorf("agent: repo %s has uncommitted changes; refusing to run (commit/stash first, or set require_clean=false): %s",
			repoDir, tailBytes(out.Bytes(), 512))
	}
	return nil
}

// runAgent spawns the headless agent in repoDir and waits for it.
func (e *AgentExecutor) runAgent(ctx context.Context, repoDir string, p AgentPayload) error {
	bin := e.Bin
	if bin == "" {
		bin = "crush"
	}
	args := append([]string{}, e.BinArgs...)
	args = append(args, "run")
	if p.Yolo {
		args = append(args, "--yolo")
	}
	args = append(args, "--quiet", p.Prompt)

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = repoDir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	prepareProcessGroup(cmd)
	// If the process dies but a grandchild holds the pipes, give the wait a
	// deadline instead of hanging the worker forever.
	cmd.WaitDelay = 10 * time.Second
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("agent: cancelled after agent work (%v): %s", ctx.Err(), tailBytes(out.Bytes(), 4096))
		}
		return fmt.Errorf("agent: run failed: %v: %s", err, tailBytes(out.Bytes(), 4096))
	}
	return nil
}

// runVerify proves the repo still works after the agent touched it.
func (e *AgentExecutor) runVerify(ctx context.Context, repoDir string, p AgentPayload) error {
	verify := p.Verify
	if verify == "" {
		verify = defaultVerify(repoDir)
	}
	if verify == "" {
		return nil // nothing to verify (non-Go repo, no explicit command)
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", verify)
	cmd.Dir = repoDir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	prepareProcessGroup(cmd)
	cmd.WaitDelay = 10 * time.Second
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("agent: cancelled during verify (%v): %s", ctx.Err(), tailBytes(out.Bytes(), 4096))
		}
		return fmt.Errorf("agent: verify failed (%q): %v: %s", verify, err, tailBytes(out.Bytes(), 4096))
	}
	return nil
}

// defaultVerify picks a sensible verification command for a repo.
func defaultVerify(repoDir string) string {
	if _, err := os.Stat(filepath.Join(repoDir, "go.mod")); err == nil {
		return "go build ./... && go test ./... -count=1"
	}
	if _, err := os.Stat(filepath.Join(repoDir, "package.json")); err == nil {
		return "npm test --silent"
	}
	return ""
}
