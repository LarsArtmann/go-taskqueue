package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func TestParseAgentPoolOptionsDLQFixFlag(t *testing.T) {
	t.Parallel()

	opts, err := parseAgentPoolOptions([]string{"--repos", "/tmp/somewhere", "--dlq-fix"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if !opts.dlqFix {
		t.Fatal("--dlq-fix did not reach the options struct")
	}

	opts, err = parseAgentPoolOptions([]string{"--repos", "/tmp/somewhere"})
	if err != nil {
		t.Fatalf("parse default: %v", err)
	}

	if opts.dlqFix {
		t.Fatal("dlq-fix must default to off")
	}
}

// TestCmdDLQDismissDeadTask runs the operator lever end-to-end against a
// scratch store: a dead task becomes cancelled with the reason recorded on
// the cancelled fact.
func TestCmdDLQDismissDeadTask(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "dlq.db")

	seed, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open seed store: %v", err)
	}

	ctx := context.Background()

	enq, err := seed.Enqueue(ctx, task.New{
		Type:        executor.TaskTypeAgent,
		Project:     "demo",
		Payload:     []byte(`{"repo":"demo","prompt":"p"}`),
		MaxAttempts: 1,
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if _, err := seed.ClaimDue(ctx, "w", time.Minute); err != nil {
		t.Fatalf("claim: %v", err)
	}

	if err := seed.Fail(ctx, enq.ID, "w", "boom", 0, nil); err != nil {
		t.Fatalf("fail: %v", err)
	}

	if got, _ := seed.Get(ctx, enq.ID); got.Status != task.Dead {
		t.Fatalf("seed status = %s, want dead", got.Status)
	}

	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}

	cmdOut := captureStdout(t, func() {
		if err := cmdDLQ(
			[]string{"--db", dbPath, "--dismiss", enq.ID.String(), "--reason", "autopsy ruled it unfixable"},
		); err != nil {
			t.Errorf("cmdDLQ: %v", err)
		}
	})

	if !strings.Contains(cmdOut, "dismissed "+enq.ID.String()) {
		t.Fatalf("output %q missing dismissal line", cmdOut)
	}

	verify, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	defer verify.Close()

	got, err := verify.Get(ctx, enq.ID)
	if err != nil {
		t.Fatal(err)
	}

	if got.Status != task.Cancelled {
		t.Fatalf("status after dismiss = %s, want cancelled", got.Status)
	}
}
