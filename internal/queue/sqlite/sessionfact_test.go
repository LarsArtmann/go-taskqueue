package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestAppendFactRecordsNonTaskFact pins the session bridge's write path: a
// fact keyed by a synthetic "session:<id>" identity persists, gets a Seq and
// a Time, and never touches the tasks table.
func TestAppendFactRecordsNonTaskFact(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	err := s.AppendFact(ctx, journal.Fact{
		TaskID: "session:sess-1",
		Type:   journal.SessionOpened,
		Detail: []byte(`{"session_id":"sess-1","repo":"/repos/demo"}`),
	})
	if err != nil {
		t.Fatalf("AppendFact: %v", err)
	}

	facts, err := s.Facts(ctx, 0, 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(facts) != 1 {
		t.Fatalf("facts = %d, want 1", len(facts))
	}

	f := facts[0]
	if f.Type != journal.SessionOpened || f.TaskID != "session:sess-1" {
		t.Fatalf("fact = %s/%s", f.Type, f.TaskID)
	}

	if f.Seq <= 0 || f.Time.IsZero() {
		t.Fatalf("store must assign Seq and Time, got seq=%d time=%v", f.Seq, f.Time)
	}

	var detail struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(f.Detail, &detail); err != nil || detail.SessionID != "sess-1" {
		t.Fatalf("detail roundtrip = %q / %v", string(f.Detail), err)
	}

	tasks, err := s.List(ctx, queue.Filter{})
	if err != nil {
		t.Fatal(err)
	}

	if len(tasks) != 0 {
		t.Fatalf("session fact leaked into tasks: %d rows", len(tasks))
	}
}

// TestAppendFactRejectsNothingTaskShaped — AppendFact is the sanctioned
// out-of-band writer; this pins its doc'd contract on the happy path only:
// no task row is created even when a task-like id is passed.
func TestAppendFactDoesNotMaterializeTaskRows(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	if err := s.AppendFact(ctx, journal.Fact{TaskID: "session:x", Type: journal.SessionClosed}); err != nil {
		t.Fatalf("AppendFact: %v", err)
	}

	if _, err := s.Get(ctx, "session:x"); !errors.Is(err, task.ErrNotFound) {
		t.Fatalf("Get(session:x) err = %v, want task.ErrNotFound", err)
	}
}
