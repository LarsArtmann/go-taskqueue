package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
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
	// MemoryLimitMB caps the shell process's virtual memory via
	// `ulimit -v` (POSIX sh; 0 = uncapped). Bounds runaway payloads on
	// shared machines — the queue keeps serving while the task dies fast.
	MemoryLimitMB int
	// Nice lowers scheduling priority via `nice -n` (0 = unchanged).
	// Interactive work wins CPU; queue work yields.
	Nice int
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

	cmd := exec.CommandContext(ctx, "sh", "-c", e.limited(line))
	prepareProcessGroup(cmd) // cooperative cancel must kill the whole tree

	var buf bytes.Buffer

	cmd.Stdout = &buf

	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		tail := tailBytes(buf.Bytes(), EvidenceTailBytes)
		SetFailureEvidence(ctx, "command", err, tail)

		if ctx.Err() != nil {
			return fmt.Errorf("command cancelled (%w): %s", ctx.Err(), tail)
		}
		// Non-zero exit is a permanent error — retrying "exit 2" never
		// helps; the payload decides the outcome, not the environment.
		return Permanent(fmt.Errorf("command failed: %w: %s", err, tail))
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

// limited wraps the command with resource guards when configured. The
// wrapper is plain POSIX sh applied BEFORE exec, so limits bind the
// payload process itself (and its whole tree, which inherits them).
func (e *CommandExecutor) limited(line string) string {
	if e.MemoryLimitMB <= 0 && e.Nice == 0 {
		return line
	}

	var b strings.Builder

	b.WriteString("exec")

	if e.Nice != 0 {
		b.WriteString(" nice -n ")
		b.WriteString(strconv.Itoa(e.Nice))
	}

	if e.MemoryLimitMB > 0 {
		b.WriteString(" sh -c 'ulimit -v ")
		b.WriteString(strconv.Itoa(e.MemoryLimitMB * 1024))
		// No exec before the user line: ulimit is a shell builtin, and the
		// user's line may use builtins too. The inner shell just runs it
		// with the limit already applied (children inherit it).
		b.WriteString("; " + line + "'")

		return b.String()
	}

	b.WriteString(" sh -c ")
	b.WriteString(quoteSh(line))

	return b.String()
}

// quoteSh single-quotes a string for sh (POSIX escape: ' → '\”).
func quoteSh(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'+"'"+'`) + "'"
}

// CommandFromPayload is the read-only view of unwrapCommand for surfaces
// that DISPLAY a command without executing it (tq show, the web dashboard):
// same accepted shapes — raw shell line, JSON string, {"cmd": "..."} — and
// the same "true" default for an empty payload. It lives beside the executor
// so the display can never drift from what actually runs.
func CommandFromPayload(payload []byte) string {
	return unwrapCommand(payload)
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
