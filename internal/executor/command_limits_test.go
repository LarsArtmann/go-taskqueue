//go:build unix

package executor

import (
	"context"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestCommandResourceLimits (round-5 M24/F129): the memory limit wraps the
// payload so the child shell sees the ulimit, and quoting survives.
func TestCommandResourceLimits(t *testing.T) {
	t.Parallel()

	e := &CommandExecutor{MemoryLimitMB: 64}

	// The payload prints its own ulimit then fails, so the value surfaces
	// in the error tail.
	err := e.Execute(context.Background(), task.Task{
		ID:      "t-limit",
		Type:    "sh",
		Payload: []byte(`{"cmd":"ulimit -v; exit 3"}`),
	})
	if err == nil {
		t.Fatal("payload exits 3: want a permanent error carrying the output")
	}

	msg := err.Error()
	if !strings.Contains(msg, "65536") {
		t.Fatalf("error tail must show the applied ulimit (65536 KB), got: %s", msg)
	}
}

// TestCommandNiceOnly wraps with nice but no memory cap.
func TestCommandNiceOnly(t *testing.T) {
	t.Parallel()

	e := &CommandExecutor{Nice: 10}

	// `nice` applies; observing it via the shell's own niceness print.
	err := e.Execute(context.Background(), task.Task{
		ID:      "t-nice",
		Type:    "sh",
		Payload: []byte(`{"cmd":"nice; exit 3"}`),
	})
	if err == nil {
		t.Fatal("payload exits 3: want an error carrying the output")
	}

	// `nice` prints the process's niceness: the inherited base plus our
	// bump. The base varies by environment, so assert the bump landed
	// (>= 10) rather than an exact value.
	msg := err.Error()

	found := false

	for _, field := range strings.Fields(msg) {
		if n := len(field); n > 0 && field[0] >= '0' && field[0] <= '9' {
			v := 0

			for _, c := range field {
				if c < '0' || c > '9' {
					v = -1
					break
				}

				v = v*10 + int(c-'0')
			}

			if v >= 10 {
				found = true

				break
			}
		}
	}

	if !found {
		t.Fatalf("niceness >= 10 must surface in output, got: %s", msg)
	}
}

// TestCommandUnchangedWithoutLimits: no config means no wrapper bytes.
func TestCommandUnchangedWithoutLimits(t *testing.T) {
	t.Parallel()

	e := &CommandExecutor{}
	if got := e.limited("echo hi"); got != "echo hi" {
		t.Fatalf("limited() = %q, want passthrough", got)
	}
}
