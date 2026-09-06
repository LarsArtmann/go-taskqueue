package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// CommandExecutor runs a command per task.
//
// If Template is empty, the task payload is used as the shell line verbatim —
// enqueue '{"cmd":"..."}' JSON or just the raw command text; JSON {"cmd":…}
// is unwrapped automatically.
//
// If Template is set, it may contain {{ID}}, {{PROJECT}}, {{TYPE}}, {{PAYLOAD}}
// placeholders substituted per task.
type CommandExecutor struct {
	Template string
}

// NewCommandExecutor builds a CommandExecutor from an optional template.
func NewCommandExecutor(template string) *CommandExecutor {
	return &CommandExecutor{Template: template}
}

// Execute renders the template (or unwraps the payload) and runs it.
func (e *CommandExecutor) Execute(ctx context.Context, t task.Task) error {
	line, err := e.render(t)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", line)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		tail := tailBytes(buf.Bytes(), 4096)
		if ctx.Err() != nil {
			return fmt.Errorf("command cancelled (%v): %s", ctx.Err(), tail)
		}
		// Non-zero exit with no diagnostic is a permanent error — retrying
		// "exit 2" never helps.
		return fmt.Errorf("command failed: %v: %s", err, tail)
	}
	return nil
}

func (e *CommandExecutor) render(t task.Task) (string, error) {
	if e.Template == "" {
		return unwrapCommand(t.Payload), nil
	}
	line := e.Template
	line = strings.ReplaceAll(line, "{{ID}}", t.ID.String())
	line = strings.ReplaceAll(line, "{{PROJECT}}", t.Project)
	line = strings.ReplaceAll(line, "{{TYPE}}", t.Type)
	line = strings.ReplaceAll(line, "{{PAYLOAD}}", string(t.Payload))
	return line, nil
}

// unwrapCommand accepts either a raw shell line or {"cmd": "..."} JSON and
// returns the command to run.
func unwrapCommand(payload []byte) string {
	s := strings.TrimSpace(string(payload))
	if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
		var m struct {
			Cmd string `json:"cmd"`
		}
		if err := json.Unmarshal(payload, &m); err == nil && m.Cmd != "" {
			return m.Cmd
		}
	}
	var str string
	if err := json.Unmarshal(payload, &str); err == nil {
		return str
	}
	if s == "" {
		return "true"
	}
	return s
}

func tailBytes(b []byte, n int) string {
	b = bytes.TrimSpace(b)
	if len(b) > n {
		b = b[len(b)-n:]
	}
	if len(b) == 0 {
		return "(no output)"
	}
	return string(b)
}
