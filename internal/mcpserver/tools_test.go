package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

var errStub = errors.New("stub failure")

// stubTask configures one stubStore row.
type stubTask struct {
	id       string
	project  string
	typ      string
	status   string
	attempts int
	lastErr  string
	parkedTo time.Time
}

// stubStore is the hermetic Store seam: no database, recorded filters.
type stubStore struct {
	tasks        []stubTask
	facts        []journal.Fact
	statusCounts map[string]int
	listErr      error
	gotFilters   []queue.Filter
}

func (s *stubStore) Get(_ context.Context, id task.ID) (task.Task, error) {
	for _, row := range s.tasks {
		if row.id == string(id) {
			return row.toTask(), nil
		}
	}

	return task.Task{}, errors.New("task not found: " + string(id))
}

func (s *stubStore) List(_ context.Context, f queue.Filter) ([]task.Task, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}

	s.gotFilters = append(s.gotFilters, f)

	var out []task.Task

	for _, row := range s.tasks {
		if f.Status != nil && *f.Status != task.Status(row.status) {
			continue
		}

		if f.Project != nil && *f.Project != row.project {
			continue
		}

		if f.Limit > 0 && len(out) >= f.Limit {
			break
		}

		out = append(out, row.toTask())
	}

	return out, nil
}

func (s *stubStore) FactsForTask(_ context.Context, id string, limit int) ([]journal.Fact, error) {
	var out []journal.Fact

	for _, fact := range s.facts {
		if fact.TaskID == id && (limit <= 0 || len(out) < limit) {
			out = append(out, fact)
		}
	}

	return out, nil
}

func (s *stubStore) StatusCounts(_ context.Context) (map[task.Status]int, error) {
	if s.statusCounts != nil {
		return task.StatusCountsMap(s.statusCounts), nil
	}

	counts := map[task.Status]int{}
	for _, row := range s.tasks {
		counts[task.Status(row.status)]++
	}

	return counts, nil
}

func (s *stubStore) ProjectCounts(_ context.Context) (map[string]int, error) {
	counts := map[string]int{}
	for _, row := range s.tasks {
		if row.project != "" {
			counts[row.project]++
		}
	}

	return counts, nil
}

func (row stubTask) toTask() task.Task {
	return task.Task{
		ID:        task.ID(row.id),
		Project:   row.project,
		Type:      row.typ,
		Status:    task.Status(row.status),
		Attempts:  row.attempts,
		LastError: row.lastErr,
		NotBefore: row.parkedTo,
		CreatedAt: time.Date(2026, 11, 23, 1, 2, 3, 0, time.UTC),
	}
}

// callTool drives one tool handler directly (the shape tests ride the
// handlers, not the transport — conversation coverage lives in
// server_test.go).
func callTool(t *testing.T, store Store, name, args string) (string, error) {
	t.Helper()

	srv := New(store, "test")

	tool, ok := srv.toolByName(name)
	if !ok {
		t.Fatalf("tool %s not registered", name)
	}

	return tool.Handler(context.Background(), json.RawMessage(args))
}

// TestTaskListGoldenShape pins the task_list payload shape: compact rows,
// omit-empty fields, RFC3339 UTC times, and the clipped last error.
func TestTaskListGoldenShape(t *testing.T) {
	store := &stubStore{tasks: []stubTask{
		{id: "t1", project: "demo", typ: "agent", status: "pending"},
		{id: "t2", typ: "sh", status: "dead", attempts: 3, lastErr: strings.Repeat("x", 300)},
	}}

	got, err := callTool(t, store, "task_list", `{}`)
	if err != nil {
		t.Fatal(err)
	}

	want := `[{` +
		`"id":"t1","project":"demo","type":"agent","status":"pending",` +
		`"maxAttempts":0,"createdAt":"2026-11-23T01:02:03Z"},` +
		`{"id":"t2","type":"sh","status":"dead","attempts":3,` +
		`"lastError":"` + strings.Repeat("x", 240) + `…",` +
		`"createdAt":"2026-11-23T01:02:03Z"}]`

	if got != want {
		t.Fatalf("task_list golden drift:\n got: %s\nwant: %s", got, want)
	}
}

// TestTaskListPushesFiltersDown pins the argument mapping: status string
// to typed enum (validated), project to pointer filter, limit default.
func TestTaskListPushesFiltersDown(t *testing.T) {
	store := &stubStore{tasks: []stubTask{
		{id: "t1", project: "demo", typ: "agent", status: "pending"},
	}}

	if _, err := callTool(t, store, "task_list", `{"status":"pending","project":"demo"}`); err != nil {
		t.Fatal(err)
	}

	if len(store.gotFilters) != 1 {
		t.Fatalf("want 1 filter recorded, got %d", len(store.gotFilters))
	}

	filter := store.gotFilters[0]
	if filter.Status == nil || *filter.Status != task.Pending {
		t.Fatalf("status filter not pushed down: %+v", filter)
	}

	if filter.Project == nil || *filter.Project != "demo" {
		t.Fatalf("project filter not pushed down: %+v", filter)
	}

	if filter.Limit != 25 {
		t.Fatalf("limit default = %d, want 25", filter.Limit)
	}

	if _, err := callTool(t, store, "task_list", `{"status":"weird"}`); err == nil {
		t.Fatal("invalid status must be a tool error")
	}
}

// TestTaskShowIncludesTrail pins task_show: full record plus the fact
// trail with clipped errors, newest facts bounded by trail_limit.
func TestTaskShowIncludesTrail(t *testing.T) {
	store := &stubStore{
		tasks: []stubTask{{id: "t1", project: "demo", typ: "agent", status: "dead", attempts: 3}},
		facts: []journal.Fact{
			{Seq: 1, TaskID: "t1", Type: "task.enqueued"},
			{Seq: 2, TaskID: "t1", Type: "task.dead-lettered", Error: strings.Repeat("e", 300)},
		},
	}

	got, err := callTool(t, store, "task_show", `{"id":"t1"}`)
	if err != nil {
		t.Fatal(err)
	}

	var payload taskShowResult
	if err := json.Unmarshal([]byte(got), &payload); err != nil {
		t.Fatal(err)
	}

	if payload.Task.ID != "t1" {
		t.Fatalf("task record missing: %s", got)
	}

	if len(payload.Trail) != 2 {
		t.Fatalf("trail entries = %d, want 2", len(payload.Trail))
	}

	if payload.Trail[1].Type != "task.dead-lettered" {
		t.Fatalf("trail[1] type = %s", payload.Trail[1].Type)
	}

	if n := len([]rune(payload.Trail[1].Error)); n != 241 { // 240 + ellipsis rune
		t.Fatalf("trail error not clipped: %d runes", n)
	}

	if _, err := callTool(t, store, "task_show", `{"id":"nope"}`); err == nil {
		t.Fatal("unknown id must be a tool error")
	}

	if _, err := callTool(t, store, "task_show", `{}`); err == nil {
		t.Fatal("missing id must be a tool error")
	}
}

// TestQueueStatsRidesSharedContract pins queue_stats to
// task.StatusCountsView — the same wire contract httpapi and webui pin
// (TestStatsSurfacesAgree's domain): every known status with zeros, plus
// the total.
func TestQueueStatsRidesSharedContract(t *testing.T) {
	store := &stubStore{tasks: []stubTask{
		{id: "t1", project: "demo", typ: "agent", status: "pending"},
		{id: "t2", project: "demo", typ: "sh", status: "dead"},
	}}

	got, err := callTool(t, store, "queue_stats", `{}`)
	if err != nil {
		t.Fatal(err)
	}

	var payload queueStatsResult
	if err := json.Unmarshal([]byte(got), &payload); err != nil {
		t.Fatal(err)
	}

	if payload.Total != 2 {
		t.Fatalf("total = %d, want 2", payload.Total)
	}

	for _, status := range task.AllStatuses() {
		if _, ok := payload.ByStatus[string(status)]; !ok {
			t.Fatalf("byStatus missing %q (zeros included is the contract): %v", status, payload.ByStatus)
		}
	}

	if payload.ByStatus["pending"] != 1 || payload.ByStatus["running"] != 0 {
		t.Fatalf("byStatus wrong: %v", payload.ByStatus)
	}

	if payload.ByProject["demo"] != 2 {
		t.Fatalf("byProject wrong: %v", payload.ByProject)
	}
}

// TestDLQListPinsDeadFilter pins dlq_list: the status filter is dead by
// construction and the project filter rides through.
func TestDLQListPinsDeadFilter(t *testing.T) {
	store := &stubStore{tasks: []stubTask{
		{id: "t1", project: "a", typ: "agent", status: "dead", attempts: 3},
		{id: "t2", project: "b", typ: "agent", status: "pending"},
	}}

	got, err := callTool(t, store, "dlq_list", `{"project":"a"}`)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(got, "t2") {
		t.Fatalf("dlq_list leaked a non-dead task: %s", got)
	}

	if !strings.Contains(got, `"status":"dead"`) {
		t.Fatalf("dlq_list missing dead row: %s", got)
	}

	filter := store.gotFilters[len(store.gotFilters)-1]
	if filter.Status == nil || *filter.Status != task.Dead {
		t.Fatalf("dlq_list status filter = %+v, want dead", filter.Status)
	}
}
