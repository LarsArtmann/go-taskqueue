package prioritize

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

// seedBacklogTask enqueues a PENDING harvested agent task: the payload
// carries the exact JSON shape the harvester mints (agent fields plus the
// harvest-pinned dedup key, item text and marker level).
func seedBacklogTask(
	t *testing.T,
	s *sqlite.Store,
	repo, itemText, key string,
	markerLevel, priority int,
) task.Task {
	t.Helper()

	raw, err := json.Marshal(map[string]any{
		"repo":        repo,
		"prompt":      "work: " + itemText,
		"item":        itemText,
		"dedup":       key,
		"markerLevel": markerLevel,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	ctx := context.Background()

	enq, err := s.Enqueue(ctx, task.New{
		Type:        executor.TaskTypeAgent,
		Project:     filepath.Base(repo),
		Priority:    priority,
		Payload:     raw,
		MaxAttempts: 1,
		DedupKey:    key,
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	return enq
}

// scorerTasks lists the prioritize tasks in the store.
func scorerTasks(t *testing.T, s *sqlite.Store) []task.Task {
	t.Helper()

	scorerType := executor.TaskTypePrioritize

	got, err := s.List(context.Background(), queue.Filter{Type: &scorerType})
	if err != nil {
		t.Fatalf("list prioritize tasks: %v", err)
	}

	return got
}

// completeScorer claims the highest-priority task (the machine-band batch)
// and completes it with the given verdicts as result detail — the real
// path a finished scorer task takes.
func completeScorer(t *testing.T, s *sqlite.Store, verdicts ...executor.PrioritizeVerdict) {
	t.Helper()

	ctx := context.Background()

	claimed, err := s.ClaimDue(ctx, testOwner, testLease)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if claimed.Type != executor.TaskTypePrioritize {
		t.Fatalf("claimed %s task, want the scorer batch", claimed.Type)
	}

	detail, err := json.Marshal(executor.PrioritizeResult{Verdicts: verdicts})
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}

	if err := s.Complete(ctx, claimed.ID, testOwner, detail); err != nil {
		t.Fatalf("complete: %v", err)
	}
}

func newTestSweeper(t *testing.T, s *sqlite.Store, cfg SweeperConfig) *Sweeper {
	t.Helper()

	sw, err := NewSweeper(context.Background(), s, cfg)
	if err != nil {
		t.Fatalf("NewSweeper: %v", err)
	}

	return sw
}

func TestSweeperMintsOneBatchForUnscoredItems(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	sw := newTestSweeper(t, s, SweeperConfig{})

	seedBacklogTask(t, s, "demo", "Fix the frobnicator", "todo:abc", 0, 50)
	seedBacklogTask(t, s, "demo", "Ship the widget", "todo:def", 0, 50)

	stats, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.BatchesEnqueued != 1 {
		t.Fatalf("batches enqueued = %d, want 1 (one batch per repo)", stats.BatchesEnqueued)
	}

	batches := scorerTasks(t, s)
	if len(batches) != 1 {
		t.Fatalf("scorer tasks = %d, want 1", len(batches))
	}

	batch := batches[0]

	var payload executor.PrioritizePayload
	if err := json.Unmarshal(batch.Payload, &payload); err != nil {
		t.Fatalf("decode batch payload: %v", err)
	}

	if payload.Repo != "demo" || payload.RepoName != "demo" {
		t.Fatalf("payload repo = %q/%q, want demo/demo", payload.Repo, payload.RepoName)
	}

	if len(payload.Items) != 2 {
		t.Fatalf("batch covers %d items, want 2", len(payload.Items))
	}

	if payload.Items[0].Key != "todo:abc" || payload.Items[1].Key != "todo:def" {
		t.Fatalf("batch keys = %v, want sorted [todo:abc todo:def]", payload.Items)
	}

	if payload.RequireClean == nil || *payload.RequireClean != true {
		t.Fatalf("batch RequireClean = %v, want clean-by-default", payload.RequireClean)
	}

	if batch.Priority != queue.MachineMin {
		t.Fatalf("batch priority = %d, want machine band %d", batch.Priority, queue.MachineMin)
	}

	// Dedup identity is proven through the idempotent-enqueue contract:
	// re-enqueueing the same key set returns the SAME task.
	wantDedup := DedupKey("demo", []string{"todo:abc", "todo:def"})
	again, err := s.Enqueue(context.Background(), task.New{
		Type:     executor.TaskTypePrioritize,
		Project:  "demo",
		Priority: queue.MachineMin,
		Payload:  batch.Payload,
		DedupKey: wantDedup,
	})
	if err != nil {
		t.Fatalf("re-enqueue: %v", err)
	}

	if again.ID != batch.ID {
		t.Fatalf("re-enqueue minted %s, want the existing batch %s", again.ID, batch.ID)
	}
}

func TestSweeperDedupSuppressesRemint(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	sw := newTestSweeper(t, s, SweeperConfig{})

	seedBacklogTask(t, s, "demo", "Fix the frobnicator", "todo:abc", 0, 50)

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("first sweep: %v", err)
	}

	stats, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}

	if stats.BatchesEnqueued != 0 || stats.BatchesKnown != 0 {
		t.Fatalf("second sweep minted again: %+v", stats)
	}

	if got := len(scorerTasks(t, s)); got != 1 {
		t.Fatalf("scorer tasks after re-sweep = %d, want 1", got)
	}
}

func TestCachedItemsLeaveTheBatch(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	sw := newTestSweeper(t, s, SweeperConfig{})

	if err := s.SavePriorityScore(context.Background(), queue.PriorityScore{
		ItemKey: "todo:abc",
		Score:   80,
		Source:  SourceScorer,
	}); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	seedBacklogTask(t, s, "demo", "Already scored", "todo:abc", 0, 80)
	seedBacklogTask(t, s, "demo", "Fresh work", "todo:def", 0, 50)

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	batches := scorerTasks(t, s)
	if len(batches) != 1 {
		t.Fatalf("scorer tasks = %d, want 1", len(batches))
	}

	var payload executor.PrioritizePayload
	if err := json.Unmarshal(batches[0].Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	if len(payload.Items) != 1 || payload.Items[0].Key != "todo:def" {
		t.Fatalf("batch covers %v, want only the uncached todo:def", payload.Items)
	}
}

func TestNewItemMintsItsOwnBatch(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	sw := newTestSweeper(t, s, SweeperConfig{})

	seedBacklogTask(t, s, "demo", "First item", "todo:abc", 0, 50)

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("first sweep: %v", err)
	}

	// A second item joins the working set while the first batch is still
	// in flight: the next sweep mints a batch for the uncovered item only.
	seedBacklogTask(t, s, "demo", "Second item", "todo:def", 0, 50)

	stats, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}

	if stats.BatchesEnqueued != 1 {
		t.Fatalf("batches enqueued = %d, want 1 for the new item", stats.BatchesEnqueued)
	}

	batches := scorerTasks(t, s)
	if len(batches) != 2 {
		t.Fatalf("scorer tasks = %d, want 2", len(batches))
	}

	var payload executor.PrioritizePayload
	if err := json.Unmarshal(batches[1].Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	if len(payload.Items) != 1 || payload.Items[0].Key != "todo:def" {
		t.Fatalf("second batch covers %v, want only todo:def", payload.Items)
	}
}

func TestApplyCachesVerdictsAndReprioritizes(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	sw := newTestSweeper(t, s, SweeperConfig{})

	work := seedBacklogTask(t, s, "demo", "Fix the frobnicator", "todo:abc", 0, 50)

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("mint sweep: %v", err)
	}

	completeScorer(t, s, executor.PrioritizeVerdict{
		ItemKey:       "todo:abc",
		Score:         85,
		EffortMinutes: 30,
		Confidence:    70,
		Reasoning:     "unblocks the release",
	})

	stats, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatalf("apply sweep: %v", err)
	}

	if stats.VerdictsCached != 1 || stats.TasksReprioritized != 1 {
		t.Fatalf("apply stats = %+v, want 1 cached / 1 reprioritized", stats)
	}

	cached, ok, err := s.PriorityScore(context.Background(), "todo:abc")
	if err != nil || !ok {
		t.Fatalf("cached verdict missing: %v (%v)", ok, err)
	}

	if cached.Score != 85 || cached.Source != SourceScorer || cached.Reasoning != "unblocks the release" {
		t.Fatalf("cached verdict = %+v", cached)
	}

	if cached.EffortMinutes != 30 {
		t.Fatalf("cached effort = %d, want 30", cached.EffortMinutes)
	}

	got, err := s.Get(context.Background(), work.ID)
	if err != nil {
		t.Fatalf("get work task: %v", err)
	}

	if got.Priority != 85 {
		t.Fatalf("work priority = %d, want 85 (clamped verdict)", got.Priority)
	}

	facts, err := s.FactsForTask(context.Background(), work.ID.String(), 0)
	if err != nil {
		t.Fatalf("facts for work task: %v", err)
	}

	repri := 0

	for _, f := range facts {
		if f.Type != journal.Reprioritized {
			continue
		}

		repri++

		var evidence queue.ReprioritizeEvidence
		if err := json.Unmarshal(f.Detail, &evidence); err != nil {
			t.Fatalf("decode repri evidence: %v", err)
		}

		if evidence.OldPriority != 50 || evidence.NewPriority != 85 || evidence.Source != "ai" {
			t.Fatalf("repri evidence = %+v", evidence)
		}
	}

	if repri != 1 {
		t.Fatalf("reprioritized facts = %d, want exactly 1", repri)
	}
}

func TestApplyHonorsMarkerAndBandProtection(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	sw := newTestSweeper(t, s, SweeperConfig{})

	marker := seedBacklogTask(t, s, "demo", "Human-pinned work", "todo:marker", 1, 90)
	hot := seedBacklogTask(t, s, "demo", "Same-session urgent", "todo:hot", 0, 120)
	plain := seedBacklogTask(t, s, "demo", "Ordinary work", "todo:plain", 0, 50)

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("mint sweep: %v", err)
	}

	completeScorer(t, s,
		executor.PrioritizeVerdict{ItemKey: "todo:marker", Score: 10},
		executor.PrioritizeVerdict{ItemKey: "todo:hot", Score: 10},
		executor.PrioritizeVerdict{ItemKey: "todo:plain", Score: 10},
	)

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("apply sweep: %v", err)
	}

	ctx := context.Background()

	// Every verdict is cached — protection governs application, not caching.
	for _, key := range []string{"todo:marker", "todo:hot", "todo:plain"} {
		if _, ok, err := s.PriorityScore(ctx, key); err != nil || !ok {
			t.Fatalf("verdict for %s not cached: ok=%v err=%v", key, ok, err)
		}
	}

	for name, seed := range map[string]struct {
		id       task.ID
		priority int
	}{
		"marker item": {marker.ID, 90},
		"hot task":    {hot.ID, 120},
	} {
		got, err := s.Get(ctx, seed.id)
		if err != nil {
			t.Fatalf("get %s: %v", name, err)
		}

		if got.Priority != seed.priority {
			t.Fatalf("%s priority = %d, want protected %d", name, got.Priority, seed.priority)
		}
	}

	got, err := s.Get(ctx, plain.ID)
	if err != nil {
		t.Fatalf("get plain: %v", err)
	}

	if got.Priority != 10 {
		t.Fatalf("plain priority = %d, want applied 10", got.Priority)
	}
}

func TestApplyFollowUpMintsUncoveredItems(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	sw := newTestSweeper(t, s, SweeperConfig{})

	seedBacklogTask(t, s, "demo", "First item", "todo:abc", 0, 50)

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("mint sweep: %v", err)
	}

	// A second item joins while the batch is in flight; the batch's
	// verdicts cover only the first — the completion must mint the next
	// batch for the uncovered item.
	seedBacklogTask(t, s, "demo", "Late item", "todo:def", 0, 50)

	completeScorer(t, s, executor.PrioritizeVerdict{ItemKey: "todo:abc", Score: 60})

	stats, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatalf("apply sweep: %v", err)
	}

	if stats.BatchesEnqueued != 1 {
		t.Fatalf("batches enqueued = %d, want 1 follow-up for todo:def", stats.BatchesEnqueued)
	}

	batches := scorerTasks(t, s)
	if len(batches) != 2 {
		t.Fatalf("scorer tasks = %d, want 2", len(batches))
	}

	var payload executor.PrioritizePayload
	if err := json.Unmarshal(batches[1].Payload, &payload); err != nil {
		t.Fatalf("decode follow-up payload: %v", err)
	}

	if len(payload.Items) != 1 || payload.Items[0].Key != "todo:def" {
		t.Fatalf("follow-up covers %v, want only todo:def", payload.Items)
	}
}

func TestApplyIsIdempotentOnResweep(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	sw := newTestSweeper(t, s, SweeperConfig{})

	work := seedBacklogTask(t, s, "demo", "Fix the frobnicator", "todo:abc", 0, 50)

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("mint sweep: %v", err)
	}

	completeScorer(t, s, executor.PrioritizeVerdict{ItemKey: "todo:abc", Score: 70})

	if _, err := sw.Sweep(context.Background()); err != nil {
		t.Fatalf("apply sweep: %v", err)
	}

	stats, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatalf("idle sweep: %v", err)
	}

	if stats.BatchesEnqueued != 0 || stats.VerdictsCached != 0 || stats.TasksReprioritized != 0 {
		t.Fatalf("idle sweep did work: %+v", stats)
	}

	facts, err := s.FactsForTask(context.Background(), work.ID.String(), 0)
	if err != nil {
		t.Fatalf("facts: %v", err)
	}

	repri := 0

	for _, f := range facts {
		if f.Type == journal.Reprioritized {
			repri++
		}
	}

	if repri != 1 {
		t.Fatalf("reprioritized facts = %d, want exactly 1 after re-sweeps", repri)
	}
}

func TestBootMintScoresStandingBacklog(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)

	seedBacklogTask(t, s, "alpha", "Standing work", "todo:one", 0, 50)
	seedBacklogTask(t, s, "beta", "Other repo work", "todo:two", 0, 50)

	// Constructed AFTER the backlog exists: the cursor head-bootstraps past
	// both enqueue facts, so only BootMint can reach the standing items.
	sw := newTestSweeper(t, s, SweeperConfig{BootMint: true})

	stats, err := sw.Sweep(context.Background())
	if err != nil {
		t.Fatalf("boot sweep: %v", err)
	}

	if stats.BatchesEnqueued != 2 {
		t.Fatalf("batches enqueued = %d, want one per repo (2)", stats.BatchesEnqueued)
	}

	if got := len(scorerTasks(t, s)); got != 2 {
		t.Fatalf("scorer tasks = %d, want 2", got)
	}
}

func TestForeignEnqueuesNeverMint(t *testing.T) {
	t.Parallel()

	s := newTestStore(t)
	sw := newTestSweeper(t, s, SweeperConfig{})

	ctx := context.Background()

	// A review mint: agent-family type is "review", payload has no harvest
	// item — never a batch trigger.
	raw, err := json.Marshal(map[string]any{"repo": "demo", "task": "0001"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if _, err := s.Enqueue(ctx, task.New{
		Type:    "review",
		Project: "demo",
		Payload: raw,
	}); err != nil {
		t.Fatalf("enqueue review: %v", err)
	}

	// A loop-closing agent task: right type, non-todo dedup key.
	loop, err := json.Marshal(map[string]any{
		"repo": "demo", "prompt": "close out", "item": "close out", "dedup": "catchup:xyz",
	})
	if err != nil {
		t.Fatalf("marshal loop payload: %v", err)
	}

	if _, err := s.Enqueue(ctx, task.New{
		Type:     executor.TaskTypeAgent,
		Project:  "demo",
		Payload:  loop,
		DedupKey: "catchup:xyz",
	}); err != nil {
		t.Fatalf("enqueue loop task: %v", err)
	}

	stats, err := sw.Sweep(ctx)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}

	if stats.BatchesEnqueued != 0 {
		t.Fatalf("foreign enqueued minted %d batches", stats.BatchesEnqueued)
	}

	if got := len(scorerTasks(t, s)); got != 0 {
		t.Fatalf("scorer tasks = %d, want 0", got)
	}
}
