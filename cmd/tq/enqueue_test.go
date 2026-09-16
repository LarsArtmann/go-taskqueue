package main

import (
	"context"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestCmdEnqueueDedupKeyIdempotent pins the fan-out contract: a re-enqueue
// with the same --dedup-key returns the stored task unchanged (one row),
// a different key mints a new task.
func TestCmdEnqueueDedupKeyIdempotent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "dedup.db")

	payload := filepath.Join(dir, "payload.json")
	if err := os.WriteFile(payload, []byte(`{"repo":"demo","prompt":"p"}`), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	base := []string{
		"--project", "demo",
		"--type", "agent",
		"--payload", "@" + payload,
		"--db", dbPath,
	}

	for range 2 {
		if err := cmdEnqueue(append(base, "--dedup-key", "libdive:demo@v1.17.0")); err != nil {
			t.Fatalf("enqueue with dedup key: %v", err)
		}
	}

	ctx := context.Background()

	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	assertCount := func(want int, phase string) {
		t.Helper()

		tasks, err := store.List(ctx, queue.Filter{})
		if err != nil {
			t.Fatalf("%s: list: %v", phase, err)
		}

		if len(tasks) != want {
			t.Fatalf("%s: got %d task(s), want %d", phase, len(tasks), want)
		}
	}

	assertCount(1, "same-key re-enqueue must not mint a second task")

	if err := cmdEnqueue(append(base, "--dedup-key", "libdive:demo@v1.18.0")); err != nil {
		t.Fatalf("enqueue with new dedup key: %v", err)
	}

	assertCount(2, "new dedup key must mint a new task")
}

// TestCmdEnqueueWaitStreamsFacts pins the --wait contract: the enqueue blocks
// until the task turns terminal, streams its journal facts, succeeds on
// completed, and fails the run on a dead task or a --timeout expiry.
func TestCmdEnqueueWaitStreamsFacts(t *testing.T) {
	t.Parallel()

	run := func(t *testing.T, failTask bool) (string, error) {
		t.Helper()

		dir := t.TempDir()
		dbPath := filepath.Join(dir, "wait.db")

		ctx := context.Background()

		store, err := sqlite.Open(dbPath)
		if err != nil {
			t.Fatalf("open store: %v", err)
		}
		defer store.Close()

		done := make(chan error, 1)

		go func() {
			done <- cmdEnqueue([]string{
				"--type", "sh",
				"--payload", `echo hi`,
				"--db", dbPath,
				"--wait",
				"--timeout", "15s",
			})
		}()

		// The command prints the task ID then blocks; find the task through
		// the store instead of parsing stdout.
		var tsk task.Task

		for range 100 {
			tasks, err := store.List(ctx, queue.Filter{})
			if err != nil {
				t.Fatalf("list: %v", err)
			}

			if len(tasks) == 1 {
				tsk = tasks[0]

				break
			}

			time.Sleep(10 * time.Millisecond)
		}

		if tsk.ID == "" {
			t.Fatal("task never appeared")
		}

		claimed, err := store.ClaimDue(ctx, "test-worker", time.Minute)
		if err != nil {
			t.Fatalf("claim: %v", err)
		}

		if claimed.ID != tsk.ID {
			t.Fatalf("claimed %s, want %s", claimed.ID, tsk.ID)
		}

		if failTask {
			err = store.FailPermanent(ctx, tsk.ID, "test-worker", "boom", nil)
		} else {
			err = store.Complete(ctx, tsk.ID, "test-worker", nil)
		}

		if err != nil {
			t.Fatalf("finish task: %v", err)
		}

		select {
		case err := <-done:
			return string(tsk.ID), err
		case <-time.After(15 * time.Second):
			t.Fatal("enqueue --wait never returned")
			return "", nil
		}
	}

	t.Run("completed succeeds", func(t *testing.T) {
		t.Parallel()

		id, err := run(t, false)
		if err != nil {
			t.Fatalf("wait on completed task: %v", err)
		}

		if id == "" {
			t.Fatal("empty task id")
		}
	})

	t.Run("dead fails the run", func(t *testing.T) {
		t.Parallel()

		_, err := run(t, true)
		if err == nil {
			t.Fatal("wait on dead task must fail the run")
		}

		if !strings.Contains(err.Error(), "finished dead") {
			t.Fatalf("error %q does not name the terminal status", err)
		}
	})

	t.Run("timeout fails the run", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		dbPath := filepath.Join(dir, "timeout.db")

		err := cmdEnqueue([]string{
			"--type", "sh",
			"--payload", `echo hi`,
			"--db", dbPath,
			"--wait",
			"--timeout", "150ms",
		})
		if err == nil {
			t.Fatal("expired wait must fail the run")
		}

		if !strings.Contains(err.Error(), "timed out") {
			t.Fatalf("error %q does not name the timeout", err)
		}
	})
}

// TestCmdEnqueueAgentConveniences pins the sugar path: convenience flags
// assemble a valid AgentPayload and imply --type agent.
func TestCmdEnqueueAgentConveniences(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "conv.db")

	prompt := filepath.Join(dir, "prompt.txt")
	if err := os.WriteFile(prompt, []byte("do the thing\n"), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}

	if err := cmdEnqueue([]string{
		"--repo", "demo",
		"--prompt-file", prompt,
		"--verify", "go build ./...",
		"--timeout-minutes", "60",
		"--yolo-task",
		"--db", dbPath,
	}); err != nil {
		t.Fatalf("enqueue with conveniences: %v", err)
	}

	ctx := context.Background()

	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	tasks, err := store.List(ctx, queue.Filter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("got %d task(s), want 1", len(tasks))
	}

	got := tasks[0]

	if got.Type != executor.TaskTypeAgent {
		t.Fatalf("type = %q, want %q (conveniences must imply agent)", got.Type, executor.TaskTypeAgent)
	}

	var payload executor.AgentPayload
	if err := json.Unmarshal(got.Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	if payload.Repo != "demo" {
		t.Errorf("repo = %q, want demo", payload.Repo)
	}

	if payload.Prompt != "do the thing\n" {
		t.Errorf("prompt = %q, want file content", payload.Prompt)
	}

	if payload.Verify != "go build ./..." {
		t.Errorf("verify = %q, want passed command", payload.Verify)
	}

	if payload.TimeoutMinutes != 60 {
		t.Errorf("timeout_minutes = %d, want 60", payload.TimeoutMinutes)
	}

	if !payload.Yolo {
		t.Error("yolo = false, want true")
	}
}

// TestCmdEnqueueAgentConvenienceErrors pins the fail-fast validation.
func TestCmdEnqueueAgentConvenienceErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "err.db")

	prompt := filepath.Join(dir, "prompt.txt")
	if err := os.WriteFile(prompt, []byte("p"), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}

	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "payload conflicts with conveniences",
			args: []string{"--type", "agent", "--payload", `{"repo":"d","prompt":"p"}`, "--repo", "d", "--prompt", "x", "--db", dbPath},
			want: "--payload cannot be combined",
		},
		{
			name: "repo required",
			args: []string{"--prompt", "x", "--db", dbPath},
			want: "--repo is required",
		},
		{
			name: "prompt source required",
			args: []string{"--repo", "d", "--db", dbPath},
			want: "needs a prompt",
		},
		{
			name: "inline and file prompt are exclusive",
			args: []string{"--repo", "d", "--prompt", "x", "--prompt-file", prompt, "--db", dbPath},
			want: "mutually exclusive",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := cmdEnqueue(tc.args)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}

			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want containing %q", err.Error(), tc.want)
			}
		})
	}
}
