package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func tasksTestStore(t *testing.T) *queue.SQLiteStore {
	t.Helper()

	s, err := queue.OpenSQLite(filepath.Join(t.TempDir(), "tasks-cli.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	return s
}

// TestResolveTaskPrefixPins the unique-prefix lookup: full ID fast path,
// unique prefix, ambiguous prefix names its candidates, unknown reports
// "no task" instead of a raw store error.
func TestResolveTaskPrefix(t *testing.T) {
	s := tasksTestStore(t)
	ctx := context.Background()

	a, err := s.Enqueue(ctx, task.New{Type: "sh", Project: "p"})
	if err != nil {
		t.Fatal(err)
	}

	b, err := s.Enqueue(ctx, task.New{Type: "sh", Project: "p"})
	if err != nil {
		t.Fatal(err)
	}

	if got, err := resolveTask(ctx, s, a.ID.String()); err != nil || got.ID != a.ID {
		t.Fatalf("full ID: got %v err %v, want %s", got.ID, err, a.ID)
	}

	// Unique prefix: ULIDs share their time prefix, so the shortest unique
	// prefix is one character past the IDs' longest common prefix.
	unique := a.ID.String()
	for i := range unique {
		if a.ID.String()[i] != b.ID.String()[i] {
			unique = a.ID.String()[:i+1]
			break
		}
	}

	got, err := resolveTask(ctx, s, unique)
	if err != nil || got.ID != a.ID {
		t.Fatalf("prefix %q: got %v err %v, want %s", unique, got.ID, err, a.ID)
	}

	// Ambiguous: both IDs share the ULID time prefix.
	_, err = resolveTask(ctx, s, a.ID.String()[:10])
	if err == nil || !strings.Contains(err.Error(), "matches 2 tasks") {
		t.Fatalf("shared prefix err = %v, want ambiguity error naming 2 tasks", err)
	}

	if _, err = resolveTask(ctx, s, "deadbeef"); err == nil || !strings.Contains(err.Error(), "no task") {
		t.Fatalf("unknown prefix err = %v, want no-task error", err)
	}
}
