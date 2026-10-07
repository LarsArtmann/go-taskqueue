package main

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
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

// TestCmdTasksVerifyContains: --verify-contains rides the store-level
// PayloadContains pushdown (payload ALONE, limit applied post-filter) and
// --json carries the payload, so stale-verify hygiene audits run without
// a `tq show` per task (09-39 §e1).
func TestCmdTasksVerifyContains(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "tasks-cli-verify.db")

	seed, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	stale := task.New{Type: "agent", Project: "p", Payload: jsontext.Value(`{"repo":"r","verify":"go vet ./..."}`)}
	fresh := task.New{Type: "agent", Project: "p", Payload: jsontext.Value(`{"repo":"r","verify":"gofmt -l ."}`)}

	staleT, err := seed.Enqueue(ctx, stale)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := seed.Enqueue(ctx, fresh); err != nil {
		t.Fatal(err)
	}

	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := cmdTasks(
			[]string{"--db", dbPath, "--type", "agent", "--verify-contains", "go vet ./...", "--json"},
		); err != nil {
			t.Errorf("verify-contains: %v", err)
		}
	})

	var got []task.Task
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode output %q: %v", out, err)
	}

	if len(got) != 1 || got[0].ID != staleT.ID {
		t.Fatalf("verify-contains = %+v, want only %s", got, staleT.ID)
	}

	if !strings.Contains(string(got[0].Payload), "go vet") {
		t.Fatalf("--json payload field missing/unpopulated: %q", got[0].Payload)
	}

	// Human listing works too (no tq show needed to read the payload).
	out = captureStdout(t, func() {
		if err := cmdTasks([]string{"--db", dbPath, "--verify-contains", "gofmt"}); err != nil {
			t.Errorf("verify-contains: %v", err)
		}
	})

	if !strings.Contains(out, "1 task(s)") {
		t.Fatalf("human listing = %q, want exactly 1 task", out)
	}
}

// TestPrintTaskListFooterShapes pins the three footer shapes of the list
// view: uncapped prints the bare count, a hit limit resolves the uncapped
// total and prints the capped footer, and a total-resolution failure falls
// back to the bare count instead of lying (the silent-cap census class).
func TestPrintTaskListFooterShapes(t *testing.T) {
	tasks := make([]task.Task, 3)
	for i := range tasks {
		tasks[i] = task.Task{Type: "sh", Project: "p"}
	}

	cases := []struct {
		name  string
		limit int
		total func() (int, error)
		want  string
	}{
		{"uncapped", 0, nil, "3 task(s)\n"},
		{"under-limit", 10, nil, "3 task(s)\n"},
		{
			"capped",
			2,
			func() (int, error) { return 5, nil },
			"showing 2 of 5 matching task(s) (capped by --limit 2; --limit 0 lists all, --count prints just the total)\n",
		},
		{
			"total-error-fallback",
			2,
			func() (int, error) { return 0, errors.New("boom") },
			"2 task(s)\n",
		},
	}

	for _, tc := range cases {
		var buf bytes.Buffer

		list := tasks
		if tc.limit > 0 && tc.limit < len(tasks) {
			list = tasks[:tc.limit]
		}

		printTaskListTo(&buf, list, tc.limit, tc.total)

		if !strings.HasSuffix(buf.String(), tc.want) {
			t.Fatalf("%s: footer = %q, want suffix %q", tc.name, buf.String(), tc.want)
		}
	}
}

// TestCmdTasksTruncationFooter: the DEFAULT `tq tasks` invocation reports
// truncation end-to-end — 60 seeded tasks against the default 50-row cap
// must print the capped footer naming the true total (the dead-letter
// census that motivated the fix lost a whole death behind a silent cap).
func TestCmdTasksTruncationFooter(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "tasks-cli-truncate.db")

	seed, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	for range 60 {
		if _, err := seed.Enqueue(ctx, task.New{Type: "sh", Project: "p"}); err != nil {
			t.Fatal(err)
		}
	}

	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := cmdTasks([]string{"--db", dbPath}); err != nil {
			t.Errorf("default listing: %v", err)
		}
	})

	if !strings.Contains(out, "showing 50 of 60 matching task(s) (capped by --limit 50") {
		t.Fatalf("default listing = %q, want capped footer naming 50 of 60", out)
	}

	out = captureStdout(t, func() {
		if err := cmdTasks([]string{"--db", dbPath, "--limit", "0"}); err != nil {
			t.Errorf("limit 0: %v", err)
		}
	})

	if !strings.Contains(out, "60 task(s)\n") || strings.Contains(out, "showing") {
		t.Fatalf("limit 0 listing = %q, want bare 60-task footer", out)
	}
}

// TestCmdTasksCountJSON pins the --count --json shape: {"count": N} —
// machine consumers get a parseable total instead of prose.
func TestCmdTasksCountJSON(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "tasks-cli-count.db")

	seed, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	for range 7 {
		if _, err := seed.Enqueue(ctx, task.New{Type: "sh", Project: "p"}); err != nil {
			t.Fatal(err)
		}
	}

	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := cmdTasks([]string{"--db", dbPath, "--count", "--json"}); err != nil {
			t.Errorf("count --json: %v", err)
		}
	})

	var got struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode %q: %v", out, err)
	}

	if got.Count != 7 {
		t.Fatalf("count json = %d, want 7", got.Count)
	}
}

// TestCmdTasksJSONEnvelope pins the --json-envelope shape: {tasks, total,
// truncated} so a paging consumer can learn the uncapped total and that a
// cap bit, which the bare array cannot express.
func TestCmdTasksJSONEnvelope(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "tasks-cli-envelope.db")

	seed, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	for range 9 {
		if _, err := seed.Enqueue(ctx, task.New{Type: "sh", Project: "p"}); err != nil {
			t.Fatal(err)
		}
	}

	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := cmdTasks([]string{"--db", dbPath, "--json", "--json-envelope", "--limit", "4"}); err != nil {
			t.Errorf("json-envelope: %v", err)
		}
	})

	var got struct {
		Tasks     []task.Task `json:"tasks"`
		Total     int         `json:"total"`
		Truncated bool        `json:"truncated"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode %q: %v", out, err)
	}

	if len(got.Tasks) != 4 || got.Total != 9 || !got.Truncated {
		t.Fatalf("envelope = len %d total %d truncated %v, want 4/9/true", len(got.Tasks), got.Total, got.Truncated)
	}
}
