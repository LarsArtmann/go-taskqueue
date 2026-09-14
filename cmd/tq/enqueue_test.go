package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
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
