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

// TaskTypeCrush runs a Crush agent headlessly in a repository. Register it
// under this type (or your own key) to make the queue agent-capable.
const TaskTypeCrush = "crush"

// CrushPayload is the payload schema for tasks executed by CrushExecutor.
type CrushPayload struct {
	// Repo is the repository directory the agent works in (its cwd).
	Repo string `json:"repo"`
	// Prompt is the full instruction for the agent run.
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
}

// DefaultCrushBinary is used when Binary and $TQ_CRUSH_BIN are empty.
const DefaultCrushBinary = "crush"

// defaultCrushTaskTimeout bounds one agent task unless the payload overrides.
const defaultCrushTaskTimeout = 30 * time.Minute

// CrushExecutor runs one Crush agent per task via `crush run` (non-interactive),
// then enforces the payload's verify contract.
//
// Safety model: --yolo (auto-accept all permissions) is an operator decision
// made when the pool starts, never a payload decision — a task payload can
// never escalate its own privileges.
type CrushExecutor struct {
	// Binary is the crush executable. Default: $TQ_CRUSH_BIN or "crush".
	// Point it at a stub in tests.
	Binary string
	// Yolo passes --yolo so the agent may act without permission prompts.
	// Without it, headless runs that need permissions will fail visibly and
	// the task retries/dead-letters — fail closed, not silently.
	Yolo bool
}

// NewCrushExecutor builds a CrushExecutor. See the struct docs for semantics.
func NewCrushExecutor(yolo bool) *CrushExecutor {
	return &CrushExecutor{Yolo: yolo}
}

func (e *CrushExecutor) binary() string {
	if e.Binary != "" {
		return e.Binary
	}
	if b := os.Getenv("TQ_CRUSH_BIN"); b != "" {
		return b
	}
	return DefaultCrushBinary
}

// Execute guards the repo, runs the agent, then runs the verify command. Any
// miss is a failed attempt (the queue retries with backoff, then dead-letters).
func (e *CrushExecutor) Execute(ctx context.Context, t task.Task) error {
	var p CrushPayload
	if len(t.Payload) == 0 {
		return errors.New("crush: empty payload, want {repo, prompt}")
	}
	if err := json.Unmarshal(t.Payload, &p); err != nil {
		return fmt.Errorf("crush: decode payload: %w", err)
	}
	if p.Repo == "" || p.Prompt == "" {
		return fmt.Errorf("crush: payload needs non-empty repo and prompt")
	}
	if info, err := os.Stat(p.Repo); err != nil || !info.IsDir() {
		return fmt.Errorf("crush: repo directory does not exist: %s", p.Repo)
	}
	if requireClean(p) {
		if _, err := os.Stat(filepath.Join(p.Repo, ".git")); err == nil {
			if err := assertCleanTree(ctx, p.Repo); err != nil {
				return err
			}
		}
	}

	timeout := defaultCrushTaskTimeout
	if p.TimeoutMinutes > 0 {
		timeout = time.Duration(p.TimeoutMinutes) * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := e.runAgent(runCtx, &p); err != nil {
		return err
	}
	return runVerify(runCtx, &p)
}

func requireClean(p CrushPayload) bool {
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
		return fmt.Errorf("crush: git status failed in %s: %v: %s", repo, err, tailBytes(out.Bytes(), 512))
	}
	if s := strings.TrimSpace(out.String()); s != "" {
		return fmt.Errorf("crush: repo %s has uncommitted changes; refusing to run agent (commit/stash first, or set require_clean=false): %s",
			repo, tailBytes(out.Bytes(), 512))
	}
	return nil
}

// runAgent spawns the headless agent in the repo and waits for it.
func (e *CrushExecutor) runAgent(ctx context.Context, p *CrushPayload) error {
	args := []string{"run", "--quiet", "--cwd", p.Repo}
	if e.Yolo {
		args = append(args, "--yolo")
	}
	if p.Model != "" {
		args = append(args, "--model", p.Model)
	}
	if p.Session != "" {
		args = append(args, "--session", p.Session)
	}
	args = append(args, "--", p.Prompt)

	cmd := exec.CommandContext(ctx, e.binary(), args...)
	cmd.Dir = p.Repo
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
			return fmt.Errorf("crush run cancelled (%v): %s", ctx.Err(), tail)
		}
		return fmt.Errorf("crush run failed: %w: %s", err, tail)
	}
	return nil
}

// runVerify enforces the quality gate after the agent exited cleanly.
func runVerify(ctx context.Context, p *CrushPayload) error {
	verify := p.Verify
	if verify == "" {
		verify = defaultVerify(p.Repo)
	}
	if verify == "" {
		return nil // nothing to verify (non-Go repo, no explicit command)
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", verify)
	cmd.Dir = p.Repo
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	prepareProcessGroup(cmd)
	cmd.WaitDelay = 10 * time.Second
	if err := cmd.Run(); err != nil {
		tail := tailBytes(buf.Bytes(), 4096)
		if ctx.Err() != nil {
			return fmt.Errorf("crush verify cancelled (%v): %s", ctx.Err(), tail)
		}
		return fmt.Errorf("crush verify failed (%q): %w: %s", verify, err, tail)
	}
	return nil
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

// RenderCrushPayload marshals a payload for tasks of type crush.
func RenderCrushPayload(p CrushPayload) (json.RawMessage, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("crush: encode payload: %w", err)
	}
	return b, nil
}
