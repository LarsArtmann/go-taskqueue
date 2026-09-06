package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
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
}

// DefaultCrushBinary is used when Binary and $TQ_CRUSH_BIN are empty.
const DefaultCrushBinary = "crush"

// CrushExecutor runs one Crush agent per task via `crush run` (non-interactive).
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

// Execute runs the agent and returns nil on exit code 0.
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
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	// If crush leaves grandchildren holding the pipes open, do not hang the
	// worker past the kill: give up on output collection shortly after kill.
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

// RenderCrushPayload marshals a payload for tasks of type crush.
func RenderCrushPayload(p CrushPayload) (json.RawMessage, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("crush: encode payload: %w", err)
	}
	return b, nil
}
