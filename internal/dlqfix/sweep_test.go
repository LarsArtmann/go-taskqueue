package dlqfix

import (
	"context"
	"encoding/json/v2"
	"path/filepath"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

func newTestStore(t *testing.T) *sqlite.Store {
	t.Helper()

	s, err := sqlite.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	return s
}

const (
	testOwner = "test-worker"
	testLease = 5 * time.Minute
)

// seedDeadAgentTask enqueues an agent task with a one-attempt budget and
// fails its single attempt (carrying failure evidence) so it dead-letters —
// the real path a task takes into the DLQ.
func seedDeadAgentTask(
	t *testing.T,
	s *sqlite.Store,
	payload executor.AgentPayload,
) task.Task {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal agent payload: %v", err)
	}

	ctx := context.Background()

	enq, err := s.Enqueue(ctx, task.New{
		Type:        executor.TaskTypeAgent,
		Project:     "demo",
		Priority:    7,
		Payload:     raw,
		MaxAttempts: 1,
		DedupKey:    "seed:" + payload.Prompt,
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if _, err := s.ClaimDue(ctx, testOwner, testLease); err != nil {
		t.Fatalf("claim: %v", err)
	}

	evidence, err := json.Marshal(executor.FailureEvidence{
		Stage:    "verify",
		ExitCode: 2,
		Tail:     "FAIL: TestShipTheThing",
	})
	if err != nil {
		t.Fatalf("marshal evidence: %v", err)
	}

	if err := s.Fail(ctx, enq.ID, testOwner, "verify failed", 0, evidence); err != nil {
		t.Fatalf("fail: %v", err)
	}

	got, err := s.Get(ctx, enq.ID)
	if err != nil || got.Status != task.Dead {
		t.Fatalf("seeded task not dead: %+v (%v)", got, err)
	}

	return got
}

// finishTask claims and completes one pending task with result detail.
func finishTask(t *testing.T, s *sqlite.Store, id task.ID, detail executor.DLQFixResult) {
	t.Helper()

	ctx := context.Background()

	if _, err := s.ClaimDue(ctx, testOwner, testLease); err != nil {
		t.Fatalf("claim %s: %v", id, err)
	}

	raw, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("marshal verdict: %v", err)
	}

	if err := s.Complete(ctx, id, testOwner, raw); err != nil {
		t.Fatalf("complete %s: %v", id, err)
	}
}

func newTestSweeper(t *testing.T, s *sqlite.Store) *Sweeper {
	t.Helper()

	sw, err := NewSweeper(context.Background(), s, SweeperConfig{Model: "", Log: nil})
	if err != nil {
		t.Fatalf("NewSweeper: %v", err)
	}

	return sw
}

// pendingDLQFixTasks lists the dlqfix tasks in the store.
func pendingDLQFixTasks(t *testing.T, s *sqlite.Store) []task.Task {
	t.Helper()

	dlqfix := executor.TaskTypeDLQFix

	got, err := s.List(context.Background(), queue.Filter{Type: &dlqfix})
	if err != nil {
		t.Fatalf("list dlqfix tasks: %v", err)
	}

	return got
}

func TestSweeperMintsOneAutopsyPerDeadAgentTask(t *testing.T) {
	s := newTestStore(t)
	dead := seedDeadAgentTask(t, s, executor.AgentPayload{
		Repo:   "demo",
		Prompt: "ship the frobnicator\n\nTask-Queue-ID: {{TASK_ID}}",
		Yolo:   true,
	})

	sw := newTestSweeper(t, s)

	stats, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.FixesEnqueued != 1 {
		t.Fatalf("FixesEnqueued = %d, want 1 (stats %+v)", stats.FixesEnqueued, stats)
	}

	tasks := pendingDLQFixTasks(t, s)
	if len(tasks) != 1 {
		t.Fatalf("dlqfix tasks = %d, want 1", len(tasks))
	}

	autopsy := tasks[0]
	if autopsy.Project != dead.Project || autopsy.Priority != dead.Priority {
		t.Fatalf("autopsy did not inherit project/priority: %+v", autopsy)
	}

	var payload executor.DLQFixPayload
	if err := json.Unmarshal(autopsy.Payload, &payload); err != nil {
		t.Fatalf("autopsy payload: %v (%s)", err, autopsy.Payload)
	}

	if payload.Repo != "demo" || payload.DeadTask != dead.ID.String() || payload.DeadType != executor.TaskTypeAgent {
		t.Fatalf("payload identity wrong: %+v", payload)
	}

	if payload.Work != "ship the frobnicator\n\nTask-Queue-ID: {{TASK_ID}}" {
		t.Fatalf("payload work not verbatim: %q", payload.Work)
	}

	if !payload.Yolo {
		t.Fatal("payload must mirror the dead task's yolo")
	}

	if payload.Failure.Stage != "verify" || payload.Failure.ExitCode != 2 || payload.Failure.Tail != "FAIL: TestShipTheThing" {
		t.Fatalf("payload evidence wrong: %+v", payload.Failure)
	}

	if DedupKey(dead.ID) != "dlqfix:"+dead.ID.String() {
		t.Fatalf("DedupKey = %q", DedupKey(dead.ID))
	}

	// Idempotent by dedup: a second sweep (replayed page, extra tick) must
	// not mint a second autopsy.
	stats, err = sw.Sweep(context.Background())
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}

	if stats.FixesEnqueued != 0 || stats.FixesKnown != 1 {
		t.Fatalf("second sweep stats = %+v, want known-only", stats)
	}

	if tasks := pendingDLQFixTasks(t, s); len(tasks) != 1 {
		t.Fatalf("dlqfix tasks after second sweep = %d, want 1", len(tasks))
	}
}

// TestSweeperNeverAutopsiesNonAgentDeaths pins the loop guard: only agent
// tasks are autopsied — a dead autopsy can never mint another autopsy, and
// sh/review/status deaths stay human surfaces.
func TestSweeperNeverAutopsiesNonAgentDeaths(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for _, taskType := range []string{"sh", executor.TaskTypeReview, executor.TaskTypeStatus, executor.TaskTypeDLQFix} {
		enq, err := s.Enqueue(ctx, task.New{
			Type: taskType, Project: "demo", MaxAttempts: 1,
			Payload: []byte(`{}`), DedupKey: "guard:" + taskType,
		})
		if err != nil {
			t.Fatalf("enqueue %s: %v", taskType, err)
		}

		if _, err := s.ClaimDue(ctx, testOwner, testLease); err != nil {
			t.Fatalf("claim %s: %v", taskType, err)
		}

		if err := s.Fail(ctx, enq.ID, testOwner, "boom", 0, nil); err != nil {
			t.Fatalf("fail %s: %v", taskType, err)
		}
	}

	stats, err := newTestSweeper(t, s).Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.FixesEnqueued != 0 || stats.FixesKnown != 0 {
		t.Fatalf("stats = %+v, want no mints", stats)
	}

	if stats.Skipped != 4 {
		t.Fatalf("Skipped = %d, want 4 (stats %+v)", stats.Skipped, stats)
	}
}

func TestSweeperRescuesOnFixedVerdict(t *testing.T) {
	s := newTestStore(t)
	dead := seedDeadAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "p"})

	sw := newTestSweeper(t, s)

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("mint sweep: %v", err)
	}

	autopsy := pendingDLQFixTasks(t, s)[0]
	finishTask(t, s, autopsy.ID, executor.DLQFixResult{
		Verdict:   executor.VerdictFixed,
		Summary:   "root cause was a bad flag; fixed and proven",
		CommitSHA: "deadbee",
	})

	stats, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatalf("dispose sweep: %v", err)
	}

	if stats.Rescued != 1 {
		t.Fatalf("Rescued = %d, want 1 (stats %+v)", stats.Rescued, stats)
	}

	got, err := s.Get(context.Background(), dead.ID)
	if err != nil {
		t.Fatal(err)
	}

	if got.Status != task.Pending || got.Attempts != 0 || got.MaxAttempts != dead.MaxAttempts {
		t.Fatalf("rescued task wrong: %+v (want pending, fresh attempts, original budget)", got)
	}
}

func TestSweeperDismissesOnWontfixVerdict(t *testing.T) {
	s := newTestStore(t)
	dead := seedDeadAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "p"})

	sw := newTestSweeper(t, s)

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("mint sweep: %v", err)
	}

	autopsy := pendingDLQFixTasks(t, s)[0]
	finishTask(t, s, autopsy.ID, executor.DLQFixResult{
		Verdict: executor.VerdictWontFix,
		Summary: "needs credentials only the operator holds",
	})

	stats, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatalf("dispose sweep: %v", err)
	}

	if stats.Dismissed != 1 {
		t.Fatalf("Dismissed = %d, want 1 (stats %+v)", stats.Dismissed, stats)
	}

	got, err := s.Get(context.Background(), dead.ID)
	if err != nil {
		t.Fatal(err)
	}

	if got.Status != task.Cancelled {
		t.Fatalf("dismissed task status = %s, want cancelled", got.Status)
	}

	trail, err := s.FactsForTask(context.Background(), dead.ID.String(), 0)
	if err != nil {
		t.Fatal(err)
	}

	var detail struct {
		Reason      string `json:"reason"`
		DismissedBy string `json:"dismissed_by"`
	}

	sawFact := false

	for _, f := range trail {
		if f.Type != journal.Cancelled {
			continue
		}

		sawFact = true

		if json.Unmarshal(f.Detail, &detail) != nil {
			t.Fatalf("dismiss fact detail not JSON: %s", f.Detail)
		}
	}

	if !sawFact || detail.Reason != "needs credentials only the operator holds" || detail.DismissedBy != DismissedBySweeper {
		t.Fatalf("dismiss fact = %+v (fact seen: %v)", detail, sawFact)
	}
}

// TestSweeperDispositionIsBenignWhenMovedElsewhere pins the idempotency
// story: if a human rescued (or dismissed) the dead task between the
// autopsy's completion and the sweep, the disposition degrades to a skipped
// counter instead of crashing or double-acting.
func TestSweeperDispositionIsBenignWhenMovedElsewhere(t *testing.T) {
	s := newTestStore(t)
	dead := seedDeadAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "p"})

	sw := newTestSweeper(t, s)

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("mint sweep: %v", err)
	}

	autopsy := pendingDLQFixTasks(t, s)[0]
	finishTask(t, s, autopsy.ID, executor.DLQFixResult{
		Verdict: executor.VerdictFixed,
		Summary: "s",
	})

	// The operator got there first.
	if err := s.RescueDead(context.Background(), dead.ID, 3); err != nil {
		t.Fatalf("manual rescue: %v", err)
	}

	stats, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatalf("dispose sweep: %v", err)
	}

	if stats.Rescued != 0 || stats.Skipped != 1 {
		t.Fatalf("stats = %+v, want skipped-not-crash", stats)
	}
}

// TestSweeperIgnoresForeignCompletions pins that completed non-dlqfix tasks
// pass through the cursor without dispositions (they are not even skips —
// they are simply not ours).
func TestSweeperIgnoresForeignCompletions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	seedDeadAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "p"})

	sw := newTestSweeper(t, s)

	if _, err := sw.Sweep(ctx); err != nil {
		t.Fatalf("mint sweep: %v", err)
	}

	// A plain agent task completes (normal pool work).
	seed := seedDeadAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "other"})
	if err := s.RescueDead(ctx, seed.ID, 1); err != nil {
		t.Fatalf("rescue: %v", err)
	}

	if _, err := s.ClaimDue(ctx, testOwner, testLease); err != nil {
		t.Fatalf("claim: %v", err)
	}

	if err := s.Complete(ctx, seed.ID, testOwner, nil); err != nil {
		t.Fatalf("complete: %v", err)
	}

	stats, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("dispose sweep: %v", err)
	}

	if stats.Rescued != 0 || stats.Dismissed != 0 {
		t.Fatalf("stats = %+v, want no dispositions from foreign completions", stats)
	}
}
