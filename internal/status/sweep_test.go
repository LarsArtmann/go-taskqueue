package status

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func newTestStore(t *testing.T) *queue.SQLiteStore {
	t.Helper()

	s, err := queue.OpenSQLite(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	return s
}

const (
	testOwner = "test-worker"
	testLease = 5 * time.Minute
)

// finishTask claims and completes one pending task with result detail. The
// claim loop skips past other pending tasks (the sweeper's minted reports can
// outrank the target in claim order); a task already Running under this
// owner's lease is completed directly.
func finishTask(t *testing.T, s *queue.SQLiteStore, id task.ID, detail json.RawMessage) {
	t.Helper()

	ctx := context.Background()

	for range 100 {
		claimed, err := s.ClaimDue(ctx, testOwner, testLease)
		if err != nil {
			break // nothing due: id already runs under our lease
		}

		if claimed.ID == id {
			break
		}
	}

	if err := s.Complete(ctx, id, testOwner, detail); err != nil {
		t.Fatalf("complete %s: %v", id, err)
	}
}

// runAgentTask enqueues and completes one agent task so the journal holds a
// real task.completed fact with result detail.
func runAgentTask(t *testing.T, s *queue.SQLiteStore, n int, result executor.AgentResult) task.Task {
	t.Helper()

	payload, err := json.Marshal(executor.AgentPayload{
		Repo:   "demo",
		Prompt: "do thing " + strconv.Itoa(n),
		Item:   "todo item " + strconv.Itoa(n),
	})
	if err != nil {
		t.Fatalf("marshal agent payload: %v", err)
	}

	enq, err := s.Enqueue(context.Background(), task.New{
		Type:     executor.TaskTypeAgent,
		Project:  "demo",
		Payload:  payload,
		DedupKey: "seed:" + strconv.Itoa(n),
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	detail, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}

	finishTask(t, s, enq.ID, detail)

	return enq
}

func payloadPayload(t *testing.T, tk task.Task) executor.StatusPayload {
	t.Helper()

	var p executor.StatusPayload
	if err := json.Unmarshal(tk.Payload, &p); err != nil {
		t.Fatalf("decode status payload: %v", err)
	}

	return p
}

func listByType(t *testing.T, s *queue.SQLiteStore, taskType string) []task.Task {
	t.Helper()

	tasks, err := s.List(context.Background(), queue.Filter{Type: &taskType})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	return tasks
}

func newSweeperOrDie(t *testing.T, s queue.Store, every int) *Sweeper {
	t.Helper()

	sw, err := NewSweeper(context.Background(), s, SweeperConfig{Every: every})
	if err != nil {
		t.Fatalf("NewSweeper: %v", err)
	}

	return sw
}

func TestNewSweeperRequiresEvery(t *testing.T) {
	t.Parallel()

	if _, err := NewSweeper(context.Background(), newTestStore(t), SweeperConfig{}); err == nil {
		t.Fatal("Every=0 must be refused")
	}
}

func TestSweepBelowEveryDoesNotMint(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	sw := newSweeperOrDie(t, s, 3)

	runAgentTask(t, s, 0, executor.AgentResult{CommitSHA: "aaaa"})

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	if got := listByType(t, s, executor.TaskTypeStatus); len(got) != 0 {
		t.Fatalf("status tasks = %d, want 0 (window not full)", len(got))
	}
}

func TestSweepNthCompletionMintsOneReport(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	sw := newSweeperOrDie(t, s, 2)

	first := runAgentTask(t, s, 0, executor.AgentResult{CommitSHA: "aaaa"})

	// Pool-realistic cadence: sweep between completions, so the window
	// fills gradually and the SECOND completion is the minting trigger.
	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep (window not full): %v", err)
	}

	second := runAgentTask(t, s, 1, executor.AgentResult{CommitSHA: "bbbb", FilesChanged: []string{"x.go"}})

	stats, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	if stats.ReportsEnqueued != 1 {
		t.Fatalf("ReportsEnqueued = %d, want 1", stats.ReportsEnqueued)
	}

	reports := listByType(t, s, executor.TaskTypeStatus)
	if len(reports) != 1 {
		t.Fatalf("status tasks = %d, want 1", len(reports))
	}

	rep := reports[0]

	payload := payloadPayload(t, rep)
	if payload.Repo != "demo" || payload.Project != "demo" {
		t.Fatalf("payload repo/project = %q/%q, want demo/demo", payload.Repo, payload.Project)
	}

	if len(payload.Completed) != 2 {
		t.Fatalf("window = %d entries, want 2", len(payload.Completed))
	}

	if payload.Completed[0].TaskID != first.ID.String() || payload.Completed[1].TaskID != second.ID.String() {
		t.Fatalf("window order = [%s, %s], want oldest first", payload.Completed[0].TaskID, payload.Completed[1].TaskID)
	}

	if payload.Completed[0].Commit != "aaaa" {
		t.Fatalf("per-completion detail missing for first entry: %+v", payload.Completed[0])
	}

	if payload.Completed[1].Commit != "bbbb" || len(payload.Completed[1].Files) != 1 {
		t.Fatalf("per-completion detail missing for second entry: %+v", payload.Completed[1])
	}

	if payload.Completed[0].Item != "todo item 0" || payload.Completed[1].Item != "todo item 1" {
		t.Fatalf("window labels must be the pinned work items, not prompt boilerplate: %+v", payload.Completed)
	}
}

func TestSweepReplayNeverDuplicates(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	sw := newSweeperOrDie(t, s, 2)

	runAgentTask(t, s, 0, executor.AgentResult{})
	runAgentTask(t, s, 1, executor.AgentResult{})

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	// Re-sweep the same facts (idempotent by dedup).
	stats, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatalf("Sweep again: %v", err)
	}

	if stats.ReportsEnqueued != 0 {
		t.Fatalf("second sweep enqueued %d, want 0", stats.ReportsEnqueued)
	}

	if got := listByType(t, s, executor.TaskTypeStatus); len(got) != 1 {
		t.Fatalf("status tasks after replay = %d, want 1", len(got))
	}
}

// TestStatusCompletionsNeverTrigger is the loop guard: a completed status
// task must not count toward (or trigger) the next window.
func TestStatusCompletionsNeverTrigger(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	sw := newSweeperOrDie(t, s, 2)

	runAgentTask(t, s, 0, executor.AgentResult{})
	runAgentTask(t, s, 1, executor.AgentResult{})

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	reports := listByType(t, s, executor.TaskTypeStatus)
	if len(reports) != 1 {
		t.Fatalf("status tasks = %d, want 1", len(reports))
	}

	finishTask(t, s, reports[0].ID, json.RawMessage(`{"report":"docs/status/r.md","next_items":1}`))

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep after report completion: %v", err)
	}

	if got := listByType(t, s, executor.TaskTypeStatus); len(got) != 1 {
		t.Fatalf("status task completion minted another report: %d tasks", len(got))
	}
}

func TestInFlightReportSuppressesUntilLanded(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	sw := newSweeperOrDie(t, s, 2)

	runAgentTask(t, s, 0, executor.AgentResult{})
	runAgentTask(t, s, 1, executor.AgentResult{})

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	// The minted report is PENDING; two more completions must not mint.
	runAgentTask(t, s, 2, executor.AgentResult{})
	runAgentTask(t, s, 3, executor.AgentResult{})

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep in-flight: %v", err)
	}

	if got := listByType(t, s, executor.TaskTypeStatus); len(got) != 1 {
		t.Fatalf("in-flight guard failed: %d status tasks, want 1", len(got))
	}

	// Report lands; the next agent completion opens the new window's mint
	// (window counts completions since the report was created).
	reports := listByType(t, s, executor.TaskTypeStatus)
	finishTask(t, s, reports[0].ID, json.RawMessage(`{"report":"docs/status/r.md","next_items":0}`))

	runAgentTask(t, s, 4, executor.AgentResult{})

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep after landing: %v", err)
	}

	if got := listByType(t, s, executor.TaskTypeStatus); len(got) != 2 {
		t.Fatalf("next window never minted: %d status tasks, want 2", len(got))
	}
}

// TestBootstrapAtHeadNoHistoryReplay pins the watermark contract: completions
// recorded before the sweeper existed never trigger facts, but the first
// completion after start reports on the full backlogged window.
func TestBootstrapAtHeadNoHistoryReplay(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	for i := range 5 {
		runAgentTask(t, s, i, executor.AgentResult{})
	}

	sw, err := NewSweeper(context.Background(), s, SweeperConfig{Every: 2})
	if err != nil {
		t.Fatalf("NewSweeper: %v", err)
	}

	stats, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	if stats.Facts != 0 || stats.ReportsEnqueued != 0 {
		t.Fatalf("bootstrap replayed history: facts=%d reports=%d, want 0/0", stats.Facts, stats.ReportsEnqueued)
	}

	if got := listByType(t, s, executor.TaskTypeStatus); len(got) != 0 {
		t.Fatalf("bootstrap minted from history: %d status tasks", len(got))
	}

	runAgentTask(t, s, 5, executor.AgentResult{})

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	reports := listByType(t, s, executor.TaskTypeStatus)
	if len(reports) != 1 {
		t.Fatalf("post-start completion did not mint: %d status tasks", len(reports))
	}

	if payload := payloadPayload(t, reports[0]); len(payload.Completed) != 6 {
		t.Fatalf("backlogged window = %d, want all 6 completions", len(payload.Completed))
	}
}

// TestForeignAgentPayloadsAreSkipped: agent tasks whose payload does not
// decode (foreign/manual shapes) are skipped, not fatal.
func TestForeignAgentPayloadsAreSkipped(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)

	sw := newSweeperOrDie(t, s, 1)

	enq, err := s.Enqueue(context.Background(), task.New{
		Type:    executor.TaskTypeAgent,
		Project: "demo",
		Payload: json.RawMessage(`{"nope": true}`),
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	finishTask(t, s, enq.ID, nil)

	stats, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	if stats.Skipped != 1 || stats.ReportsEnqueued != 0 {
		t.Fatalf("stats = %+v, want skipped=1 enqueued=0", stats)
	}
}

func TestSweeperRequireCleanMirrorsAllowDirty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		allowDirty   bool
		wantReqClean bool
	}{
		{name: "default pool requires clean", allowDirty: false, wantReqClean: true},
		{name: "allow-dirty pool mints dirty-tolerant reports", allowDirty: true, wantReqClean: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := newTestStore(t)

			sw, err := NewSweeper(context.Background(), s, SweeperConfig{Every: 1, AllowDirty: tt.allowDirty})
			if err != nil {
				t.Fatalf("NewSweeper: %v", err)
			}

			runAgentTask(t, s, 7, executor.AgentResult{})

			if _, err := sw.Sweep(context.Background()); err != nil {
				t.Fatalf("Sweep: %v", err)
			}

			reports := listByType(t, s, executor.TaskTypeStatus)
			if len(reports) != 1 {
				t.Fatalf("status tasks = %d, want 1", len(reports))
			}

			payload := payloadPayload(t, reports[0])
			if payload.RequireClean == nil || *payload.RequireClean != tt.wantReqClean {
				t.Fatalf("require_clean = %+v, want %v", payload.RequireClean, tt.wantReqClean)
			}
		})
	}
}

func TestSweeperPinsTaskTimeoutIntoPayload(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)

	sw, err := NewSweeper(context.Background(), s, SweeperConfig{Every: 1, TaskTimeout: 90 * time.Minute})
	if err != nil {
		t.Fatalf("NewSweeper: %v", err)
	}

	runAgentTask(t, s, 9, executor.AgentResult{})

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}

	reports := listByType(t, s, executor.TaskTypeStatus)
	if len(reports) != 1 {
		t.Fatalf("status tasks = %d, want 1", len(reports))
	}

	payload := payloadPayload(t, reports[0])
	if payload.TimeoutMinutes != 90 {
		t.Fatalf("timeout_minutes = %d, want 90", payload.TimeoutMinutes)
	}
}
