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

// TaskTypeAgent runs a headless AI coding agent (crush) in a repository.
// Register it under this type to make the queue agent-capable.
const TaskTypeAgent = "agent"

// TaskTypeCrush is the pre-convergence name of TaskTypeAgent. Deprecated: use
// TaskTypeAgent.
const TaskTypeCrush = TaskTypeAgent

// AgentPayload is the payload contract for "agent" tasks: run a headless AI
// coding agent in a repository, then prove the result.
type AgentPayload struct {
	// Repo is the repository the agent works in: a name resolved against the
	// executor's ProjectsDir, or an absolute path. Required.
	Repo string `json:"repo"`
	// Prompt is the full instruction for the agent run. Required.
	Prompt string `json:"prompt"`
	// Model optionally overrides the crush model ("provider/model").
	Model string `json:"model,omitempty"`
	// Session optionally continues a previous crush session by ID.
	Session string `json:"session,omitempty"`
	// Dedup is the harvester's item key; purely informational, used to keep
	// TODO items and tasks 1:1 across harvest runs.
	Dedup string `json:"dedup,omitempty"`
	// Verify is a shell command that must exit 0 after the agent run for the
	// task to complete — the enforced quality gate. Empty means auto-detect:
	// Go repositories (go.mod present) run "go build ./... && go test ./...
	// -count=1", everything else runs nothing.
	Verify string `json:"verify,omitempty"`
	// RequireClean refuses to start unless the repo's git tree is clean, so
	// the pool never tramples human work-in-progress. Default true. Repos
	// without a .git directory skip the check (nothing to protect).
	RequireClean *bool `json:"require_clean,omitempty"`
	// TimeoutMinutes caps the whole task (agent run + verify). Default 30.
	// The worker's task timeout still applies as a hard ceiling above this.
	TimeoutMinutes int `json:"timeout_minutes,omitempty"`
	// Yolo marks this task's agent as autonomous. crush run has NO yolo
	// flag (verified against crush v0.92: "Unknown flag: --yolo"); autonomy
	// comes from the repo's own project-local crush config granting
	// permissions (`.crushrc`: `permissions allow view ls grep edit write bash`).
	// This field makes the executor fail fast with remediation guidance when
	// autonomy is requested but the repo has no such config, instead of
	// burning agent attempts on runs that stall on permission prompts.
	Yolo bool `json:"yolo,omitempty"`
}

// CrushPayload is the pre-convergence name of AgentPayload. Deprecated: use
// AgentPayload.
type CrushPayload = AgentPayload

// DefaultAgentBinary is used when Bin, $TQ_AGENT_BIN and $TQ_CRUSH_BIN are
// all empty.
const DefaultAgentBinary = "crush"

// defaultAgentTaskTimeout bounds one agent task unless the payload overrides.
const defaultAgentTaskTimeout = 30 * time.Minute

// AgentExecutor runs one headless AI coding agent per task, then enforces the
// payload's verify contract. It must be safe for concurrent use; the worker
// pool executes several tasks in parallel.
type AgentExecutor struct {
	// Bin is the agent executable. Default: $TQ_AGENT_BIN, $TQ_CRUSH_BIN or
	// "crush". Point it at a stub in tests.
	Bin string
	// ProjectsDir resolves relative Repo names in payloads ("demo" →
	// <ProjectsDir>/demo). Absolute Repo paths bypass it.
	ProjectsDir string
	// Yolo is the operator-level autonomy request (pool start). crush run
	// has no yolo flag; see AgentPayload.Yolo for how autonomy is actually
	// granted (repo-local crush config). Fail closed, never silently.
	Yolo bool
}

// NewAgentExecutor builds an AgentExecutor for a projects directory.
func NewAgentExecutor(projectsDir string) *AgentExecutor {
	return &AgentExecutor{ProjectsDir: projectsDir}
}

func (e *AgentExecutor) binary() string {
	if e.Bin != "" {
		return e.Bin
	}
	if b := os.Getenv("TQ_AGENT_BIN"); b != "" {
		return b
	}
	if b := os.Getenv("TQ_CRUSH_BIN"); b != "" {
		return b
	}
	return DefaultAgentBinary
}

// Execute guards the repo, runs the agent, then runs the verify command.
// Any miss is a failed attempt (the queue retries with backoff, then
// dead-letters). Input-contract misses (payload, repo, dirty tree, autonomy)
// are permanent: the identical retry would fail identically, and for agent
// tasks every retry is real money.
func (e *AgentExecutor) Execute(ctx context.Context, t task.Task) error {
	var p AgentPayload
	if len(t.Payload) == 0 {
		return Permanent(errors.New("agent: empty payload, want {repo, prompt}"))
	}
	if err := json.Unmarshal(t.Payload, &p); err != nil {
		return Permanent(fmt.Errorf("agent: decode payload: %w", err))
	}
	if p.Repo == "" || p.Prompt == "" {
		return Permanent(errors.New("agent: payload needs non-empty repo and prompt"))
	}
	repoDir, err := e.repoDir(p.Repo)
	if err != nil {
		return Permanent(err)
	}
	if requireClean(p) {
		if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
			if err := assertCleanTree(ctx, repoDir); err != nil {
				return Permanent(err)
			}
		}
	}

	timeout := defaultAgentTaskTimeout
	if p.TimeoutMinutes > 0 {
		timeout = time.Duration(p.TimeoutMinutes) * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := e.runAgent(runCtx, repoDir, &p); err != nil {
		return err
	}
	return runVerify(runCtx, repoDir, &p)
}

// repoDir resolves a payload repo name: absolute paths pass through,
// relative names resolve against ProjectsDir.
func (e *AgentExecutor) repoDir(repo string) (string, error) {
	if filepath.IsAbs(repo) {
		if info, err := os.Stat(repo); err != nil || !info.IsDir() {
			return "", fmt.Errorf("agent: repo directory does not exist: %s", repo)
		}
		return repo, nil
	}
	if e.ProjectsDir == "" {
		return "", fmt.Errorf("agent: relative repo %q needs a projects dir on the executor", repo)
	}
	dir := filepath.Join(e.ProjectsDir, repo)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("agent: repo %q does not exist under the projects dir", repo)
	}
	return dir, nil
}

func requireClean(p AgentPayload) bool {
	if p.RequireClean == nil {
		return true
	}
	return *p.RequireClean
}

// assertCleanTree fails unless the repo has no uncommitted changes.
func assertCleanTree(ctx context.Context, repo string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repo, "status", "--porcelain")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("agent: git status failed in %s: %v: %s", repo, err, tailBytes(out.Bytes(), 512))
	}
	if s := strings.TrimSpace(out.String()); s != "" {
		return fmt.Errorf("agent: repo %s has uncommitted changes; refusing to run agent (commit/stash first, or set require_clean=false): %s",
			repo, tailBytes(out.Bytes(), 512))
	}
	return nil
}

// runAgent spawns the headless agent in the repo and waits for it.
func (e *AgentExecutor) runAgent(ctx context.Context, repoDir string, p *AgentPayload) error {
	if e.Yolo || p.Yolo {
		if err := requireRepoAutonomy(repoDir); err != nil {
			return err
		}
	}
	args := []string{"run", "--quiet", "--cwd", repoDir}
	if p.Model != "" {
		args = append(args, "--model", p.Model)
	}
	if p.Session != "" {
		args = append(args, "--session", p.Session)
	}
	args = append(args, "--", p.Prompt)

	cmd := exec.CommandContext(ctx, e.binary(), args...)
	cmd.Dir = repoDir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	// Kill the whole process tree on cancel (agents spawn children) and do
	// not hang the worker if grandchildren hold the pipes open.
	prepareProcessGroup(cmd)
	cmd.WaitDelay = 10 * time.Second
	if err := cmd.Run(); err != nil {
		tail := tailBytes(buf.Bytes(), 8192)
		if ctx.Err() != nil {
			return fmt.Errorf("agent run cancelled (%v): %s", ctx.Err(), tail)
		}
		return fmt.Errorf("agent run failed: %w: %s", err, tail)
	}
	return nil
}

// runVerify enforces the quality gate after the agent exited cleanly.
func runVerify(ctx context.Context, repoDir string, p *AgentPayload) error {
	verify := p.Verify
	if verify == "" {
		verify = defaultVerify(repoDir)
	}
	if verify == "" {
		return nil // nothing to verify (non-Go repo, no explicit command)
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", verify)
	cmd.Dir = repoDir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	prepareProcessGroup(cmd)
	cmd.WaitDelay = 10 * time.Second
	if err := cmd.Run(); err != nil {
		tail := tailBytes(buf.Bytes(), 4096)
		if ctx.Err() != nil {
			return fmt.Errorf("agent verify cancelled (%v): %s", ctx.Err(), tail)
		}
		return fmt.Errorf("agent verify failed (%q): %w: %s", verify, err, tail)
	}
	return nil
}

// requireRepoAutonomy fails fast when an autonomous run is requested but
// the repo has no project-local crush config that could grant permissions.
// Without this check an unattended pool burns its attempt budget on runs
// that stall or die on permission prompts (crush run has no --yolo flag).
func requireRepoAutonomy(repoDir string) error {
	for _, name := range []string{".crushrc", "crushrc", ".crush.json", "crush.json"} {
		if _, err := os.Stat(filepath.Join(repoDir, name)); err == nil {
			return nil
		}
	}
	return Permanent(fmt.Errorf("agent: autonomy requested but %s has no project-local crush config; add a .crushrc with 'permissions allow view ls grep edit write bash' (or unset yolo)", repoDir))
}

// defaultVerify picks a sensible verification command for a repo.
func defaultVerify(repo string) string {
	if _, err := os.Stat(filepath.Join(repo, "go.mod")); err == nil {
		return "go build ./... && go test ./... -count=1"
	}
	if _, err := os.Stat(filepath.Join(repo, "package.json")); err == nil {
		return "npm test --silent"
	}
	return ""
}

// RenderAgentPayload marshals a payload for tasks of type agent.
func RenderAgentPayload(p AgentPayload) (json.RawMessage, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("agent: encode payload: %w", err)
	}
	return b, nil
}

// RenderCrushPayload is the pre-convergence name of RenderAgentPayload.
// Deprecated: use RenderAgentPayload.
func RenderCrushPayload(p AgentPayload) (json.RawMessage, error) {
	return RenderAgentPayload(p)
}
