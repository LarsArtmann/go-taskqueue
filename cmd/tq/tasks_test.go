package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func tasksTestStore(t *testing.T) *sqlite.Store {
	t.Helper()

	s, err := sqlite.Open(filepath.Join(t.TempDir(), "tasks-cli.db"))
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

// TestCmdTasksSincePushdown: --since selects the creation window through
// the store (age-desc order, limit applied in SQL), not a CLI-side filter.
func TestCmdTasksSincePushdown(t *testing.T) {
	s := tasksTestStore(t)
	ctx := context.Background()

	old, err := s.Enqueue(ctx, task.New{Type: "sh", Project: "p"})
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(2 * time.Millisecond) // created_at is unix-milli; separate the two

	fresh, err := s.Enqueue(ctx, task.New{Type: "sh", Project: "p"})
	if err != nil {
		t.Fatal(err)
	}

	since := old.CreatedAt.Add(time.Millisecond)

	got, err := s.List(ctx, queue.Filter{Since: &since, Sort: "age-desc", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 1 || got[0].ID != fresh.ID {
		t.Fatalf("pushdown window = %+v, want only the fresh task", got)
	}
}

// TestCmdTasksBandFilter: --band maps to the store-level PriorityMin/Max
// pushdown (ADR-0015 band ranges), not a CLI-side filter; unknown bands
// are rejected.
func TestCmdTasksBandFilter(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "tasks-cli-band.db")

	seed, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	hot := task.New{Type: "sh", Project: "p"}
	hot.Priority = queue.HotMin
	machine := task.New{Type: "sh", Project: "p"}
	machine.Priority = queue.MachineMin
	backlog := task.New{Type: "sh", Project: "p"}

	want := make(map[queue.Band]task.ID)

	for band, n := range map[queue.Band]task.New{
		queue.BandHot:     hot,
		queue.BandMachine: machine,
		queue.BandBacklog: backlog,
	} {
		enqueued, err := seed.Enqueue(ctx, n)
		if err != nil {
			t.Fatal(err)
		}

		want[band] = enqueued.ID
	}

	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}

	for band, id := range want {
		out := captureStdout(t, func() {
			if err := cmdTasks([]string{"--db", dbPath, "--band", string(band), "--json"}); err != nil {
				t.Errorf("band %q: %v", band, err)
			}
		})

		var got []task.Task
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("band %q: decode output %q: %v", band, out, err)
		}

		if len(got) != 1 || got[0].ID != id {
			t.Fatalf("band %q = %+v, want only %s", band, got, id)
		}
	}

	err = cmdTasks([]string{"--db", dbPath, "--band", "bogus"})
	if err == nil || !strings.Contains(err.Error(), "unknown band") {
		t.Fatalf("band bogus err = %v, want unknown-band error", err)
	}
}
