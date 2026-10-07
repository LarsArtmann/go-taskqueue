package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/projectionhost/v4"
	"github.com/larsartmann/go-taskqueue/internal/readmodel"
)

// TestDoctorProjectionDLQ pins the --dlq poison-sidecar check: absent
// sidecar and empty sidecar are ok, stored poison facts are a WARN whose
// Items carry the most recent failures.
func TestDoctorProjectionDLQ(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "q.db")

	results, err := runDoctor(ctx, doctorOptions{DBPath: path, DLQ: true})
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	if r := resultByName(results, "projection-dlq"); r.Status != checkOK {
		t.Errorf("absent sidecar = %s (%s), want ok", r.Status, r.Detail)
	}

	// An empty sidecar (created by any serve run) is still ok.
	dlq, err := readmodel.OpenDeadLetters(ctx, readmodel.PathFor(path))
	if err != nil {
		t.Fatalf("open sidecar: %v", err)
	}

	if n, err := dlq.Count(ctx); err != nil || n != 0 {
		t.Fatalf("fresh sidecar count = %d, %v; want 0, nil", n, err)
	}

	if err := dlq.Close(); err != nil {
		t.Fatalf("close sidecar: %v", err)
	}

	results, err = runDoctor(ctx, doctorOptions{DBPath: path, DLQ: true})
	if err != nil {
		t.Fatalf("runDoctor 2: %v", err)
	}

	if r := resultByName(results, "projection-dlq"); r.Status != checkOK {
		t.Errorf("empty sidecar = %s (%s), want ok", r.Status, r.Detail)
	}

	// A poison fact flips the check to WARN with the failure in Items.
	dlq, err = readmodel.OpenDeadLetters(ctx, readmodel.PathFor(path))
	if err != nil {
		t.Fatalf("reopen sidecar: %v", err)
	}

	defer func() { _ = dlq.Close() }()

	poison := projectionhost.DeadLetterEntry{
		ProjectionName: "tasks",
		EventType:      "task.enqueued",
		Error:          "boom: unprocessable fact",
		FailedAt:       time.Now(),
	}

	if err := dlq.Store().Store(ctx, poison); err != nil {
		t.Fatalf("store poison: %v", err)
	}

	results, err = runDoctor(ctx, doctorOptions{DBPath: path, DLQ: true})
	if err != nil {
		t.Fatalf("runDoctor 3: %v", err)
	}

	r := resultByName(results, "projection-dlq")
	if r.Status != checkWarn {
		t.Errorf("poison sidecar = %s (%s), want warn", r.Status, r.Detail)
	}

	if len(r.Items) != 1 || r.Items[0] == "" {
		t.Errorf("Items = %v, want the poison entry row", r.Items)
	}
}
