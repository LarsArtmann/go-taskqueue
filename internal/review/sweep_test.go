package review

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
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

// finishTask claims and completes one pending task with result detail.
func finishTask(t *testing.T, s *queue.SQLiteStore, id task.ID, detail json.RawMessage) {
	t.Helper()

	ctx := context.Background()

	if _, err := s.ClaimDue(ctx, testOwner, testLease); err != nil {
		t.Fatalf("claim %s: %v", id, err)
	}

	if err := s.Complete(ctx, id, testOwner, detail); err != nil {
		t.Fatalf("complete %s: %v", id, err)
	}
}

// runAgentTask enqueues and completes one agent task so the journal holds a
// real task.completed fact with result detail.
func runAgentTask(t *testing.T, s *queue.SQLiteStore, payload executor.AgentPayload, result executor.AgentResult) task.Task {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal agent payload: %v", err)
	}

	enq, err := s.Enqueue(context.Background(), task.New{
		Type:     executor.TaskTypeAgent,
		Project:  "demo",
		Payload:  raw,
		DedupKey: "seed:" + payload.Prompt,
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

// runReviewTask enqueues and completes one review task.
func runReviewTask(t *testing.T, s *queue.SQLiteStore, payload executor.ReviewPayload, result executor.ReviewResult) task.Task {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal review payload: %v", err)
	}

	enq, err := s.Enqueue(context.Background(), task.New{
		Type:     executor.TaskTypeReview,
		Project:  "demo",
		Payload:  raw,
		DedupKey: "seed-review:" + payload.ReviewedTask,
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

// listByType returns every task of one type (the store does not expose
// dedup-key lookups; type filtering is enough to find what a sweep minted).
func listByType(t *testing.T, s *queue.SQLiteStore, taskType string) []task.Task {
	t.Helper()

	tasks, err := s.List(context.Background(), queue.Filter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	var out []task.Task

	for _, tk := range tasks {
		if tk.Type == taskType {
			out = append(out, tk)
		}
	}

	return out
}

func TestSweepEnqueuesOneReviewPerCompletedAgentTask(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	done := runAgentTask(t, s, executor.AgentPayload{
		Repo: "demo", Prompt: "add the frobnicator", Yolo: true,
	}, executor.AgentResult{CommitSHA: "abc1234", FilesChanged: []string{"frob.go"}})

	sw := NewSweeper(s, SweeperConfig{})

	stats, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.ReviewsEnqueued != 1 {
		t.Fatalf("reviews enqueued = %d, want 1 (stats %+v)", stats.ReviewsEnqueued, stats)
	}

	reviews := listByType(t, s, executor.TaskTypeReview)
	if len(reviews) != 1 {
		t.Fatalf("review tasks in store = %d, want 1", len(reviews))
	}

	if reviews[0].Project != "demo" {
		t.Fatalf("review project = %q, want demo", reviews[0].Project)
	}

	var got executor.ReviewPayload
	if err := json.Unmarshal(reviews[0].Payload, &got); err != nil {
		t.Fatalf("review payload: %v", err)
	}

	if got.Repo != "demo" || got.ReviewedTask != done.ID.String() ||
		got.Item != "add the frobnicator" || got.CommitSHA != "abc1234" ||
		len(got.FilesChanged) != 1 || !got.Yolo {
		t.Fatalf("review payload mismatch: %+v", got)
	}

	again, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}

	if again.ReviewsEnqueued != 0 || again.ReviewsKnown != 0 {
		t.Fatalf("second sweep must be a no-op on a quiet journal, got %+v", again)
	}
}

func TestSweepSkipsNonAgentCompletions(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	enq, err := s.Enqueue(ctx, task.New{Type: "sh", Payload: json.RawMessage(`{"cmd":"echo hi"}`)})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	finishTask(t, s, enq.ID, nil)

	sw := NewSweeper(s, SweeperConfig{})

	stats, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.ReviewsEnqueued != 0 {
		t.Fatalf("sh completions must not be reviewed, got %+v", stats)
	}
}

func TestSweepAutofixMintsFixTasksPerFinding(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	runAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "original item"}, executor.AgentResult{CommitSHA: "def5678"})

	runReviewTask(t, s, executor.ReviewPayload{
		Repo:         "demo",
		ReviewedTask: "t-reviewed",
		Item:         "original item",
		CommitSHA:    "def5678",
	}, executor.ReviewResult{
		Verdict: executor.VerdictRequestChanges,
		Findings: []executor.ReviewFinding{
			{Title: "nil map write", Severity: "high", Detail: "write to nil map in frob.go"},
			{Title: "missing test", Severity: "medium"},
		},
	})

	sw := NewSweeper(s, SweeperConfig{Autofix: true})

	stats, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.FixesEnqueued != 2 {
		t.Fatalf("fixes enqueued = %d, want 2 (stats %+v)", stats.FixesEnqueued, stats)
	}

	fixes := listByType(t, s, executor.TaskTypeAgent)
	if len(fixes) != 2 {
		t.Fatalf("agent fix tasks in store = %d, want 2", len(fixes))
	}

	var nilMapFix task.Task

	for _, fix := range fixes {
		var p executor.AgentPayload

		if err := json.Unmarshal(fix.Payload, &p); err != nil {
			t.Fatalf("fix payload: %v", err)
		}

		if strings.Contains(p.Prompt, "nil map write") {
			nilMapFix = fix

			for _, want := range []string{"original item", "high", "def5678", "no unrelated changes"} {
				if !strings.Contains(p.Prompt, want) {
					t.Errorf("fix prompt missing %q", want)
				}
			}
		}
	}

	if nilMapFix.ID == "" || nilMapFix.Project != "demo" {
		t.Fatal("fix task for 'nil map write' not found")
	}

	again, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("resweep: %v", err)
	}

	if again.FixesEnqueued != 0 {
		t.Fatalf("re-sweep must not duplicate fix tasks, got %+v", again)
	}
}

func TestSweepAutofixIgnoresApproveAndOffSwitch(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	reviewed := runAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "item"}, executor.AgentResult{})
	runReviewTask(t, s, executor.ReviewPayload{
		Repo: "demo", ReviewedTask: reviewed.ID.String(), Item: "item",
	}, executor.ReviewResult{Verdict: executor.VerdictApprove})

	swOff := NewSweeper(s, SweeperConfig{})
	statsOff, err := swOff.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep off: %v", err)
	}

	if statsOff.FixesEnqueued != 0 {
		t.Fatalf("autofix off must not mint fixes, got %+v", statsOff)
	}

	swOn := NewSweeper(s, SweeperConfig{Autofix: true})

	statsOn, err := swOn.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep on: %v", err)
	}

	if statsOn.FixesEnqueued != 0 {
		t.Fatalf("approve verdicts must not mint fixes, got %+v", statsOn)
	}
}

// TestSweepWatermarkResumesAcrossSweeps: only NEW completions after the
// sweeper's first pass get reviews — the watermark moves forward.
func TestSweepWatermarkResumesAcrossSweeps(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	runAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "first"}, executor.AgentResult{})

	sw := NewSweeper(s, SweeperConfig{})
	if _, err := sw.Sweep(ctx); err != nil {
		t.Fatalf("first sweep: %v", err)
	}

	runAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "second"}, executor.AgentResult{})

	stats, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}

	if stats.ReviewsEnqueued != 1 {
		t.Fatalf("only the new completion gets a review, got %+v", stats)
	}
}

func TestSweepWatermarkStartsAtHead(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	ctx := context.Background()

	runAgentTask(t, s, executor.AgentPayload{Repo: "demo", Prompt: "pre-existing"}, executor.AgentResult{})

	// A sweeper created AFTER the completion starts at the journal head:
	// historical completions are not replayed (documented gap semantics,
	// same as the papdashboard bridge watermark).
	sw := NewSweeper(s, SweeperConfig{})

	stats, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.ReviewsEnqueued != 0 {
		t.Fatalf("sweeper must not replay pre-start completions, got %+v", stats)
	}
}

func TestFixDedupKeyIsStableAndDistinct(t *testing.T) {
	t.Parallel()

	a := FixDedupKey(task.ID("r1"), "nil map write")
	b := FixDedupKey(task.ID("r1"), "nil map write")
	c := FixDedupKey(task.ID("r1"), "other finding")
	d := FixDedupKey(task.ID("r2"), "nil map write")

	if a != b {
		t.Fatal("same review + title must produce the same key")
	}

	if a == c || a == d {
		t.Fatal("different title or review must produce a different key")
	}
}
