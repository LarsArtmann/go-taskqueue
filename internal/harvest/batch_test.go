package harvest

import (
	"context"
	"encoding/json/v2"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// batchedTodo is a two-section backlog: three adjacent Bugs items, a blocked
// breaker, then two Refactoring items. With BatchItems=3 the Bugs run splits
// around the blocker; the Refactoring items form their own run.
const batchedTodo = `# Project

## Bugs

- [ ] fix the null deref in claim
- [ ] fix the off-by-one in retry ladder
- [ ] fix the leak in heartbeat goroutine
- [ ] fix docs typo — BLOCKED: owner must decide wording
- [ ] fix the race in sweeper watermark

## Refactoring

- [ ] extract the prompt renderer
- [ ] split the survey pass
`

// harvestBatch runs the harvester once with batching and returns the pending
// tasks it created.
func harvestBatch(t *testing.T, dir string, batchItems, maxPerTick int) []task.Task {
	t.Helper()

	q := openQueue(t)
	h := New(q, Config{ProjectsDir: dir, BatchItems: batchItems, MaxPerTick: maxPerTick})

	if _, err := h.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	pending, err := q.List(context.Background(), pendingFilter("batchy"))
	if err != nil {
		t.Fatal(err)
	}

	return pending
}

// decodeBatchPayload reads a batched task's payload back.
func decodeBatchPayload(t *testing.T, tk task.Task) harvestPayload {
	t.Helper()

	var payload harvestPayload
	if err := json.Unmarshal(tk.Payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	return payload
}

// TestBatchGroupsAdjacentSameSectionItems pins the grouping contract: with
// BatchItems=3, the first harvest admits ONE task covering the three adjacent
// Bugs items; the blocked breaker excluded them from a wider run; the
// Refactoring run and the leftover Bugs item are paced out (one new task per
// repo per run).
func TestBatchGroupsAdjacentSameSectionItems(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "batchy", batchedTodo)

	pending := harvestBatch(t, dir, 3, 10)
	if len(pending) != 1 {
		t.Fatalf("pending tasks = %d, want exactly one batch:\n%+v", len(pending), pending)
	}

	payload := decodeBatchPayload(t, pending[0])
	if len(payload.ItemKeys) != 3 || len(payload.Items) != 3 {
		t.Fatalf("batch members = %v, want the three pre-blocker Bugs items", payload.Items)
	}

	for _, text := range []string{"fix the null deref in claim", "fix the off-by-one in retry ladder", "fix the leak in heartbeat goroutine"} {
		if !strings.Contains(strings.Join(payload.Items, "\n"), text) {
			t.Errorf("batch missing member %q: %v", text, payload.Items)
		}
	}

	if !strings.HasPrefix(payload.Dedup, BatchKeyPrefix) {
		t.Errorf("batch dedup = %q, want %s-prefixed", payload.Dedup, BatchKeyPrefix)
	}

	if payload.Item != payload.Items[0] {
		t.Errorf("Item = %q, want the FIRST member %q", payload.Item, payload.Items[0])
	}
}

// TestBatchIsOneTaskAgainstGates pins the cost accounting: a batch counts as
// ONE task against --max-per-tick.
func TestBatchIsOneTaskAgainstGates(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "batchy", batchedTodo)

	pending := harvestBatch(t, dir, 3, 1)
	if len(pending) != 1 {
		t.Fatalf("pending tasks = %d, want the one batch admitted under a max-per-tick of 1", len(pending))
	}
}

// TestBatchPayloadContract pins the batch prompt: numbered member list,
// footer placeholder, per-item BLOCKED escape, retry skip rule, follow-up
// grant, scaled timeout, and the max member marker pinned.
func TestBatchPayloadContract(t *testing.T) {
	dir := t.TempDir()
	todo := strings.Replace(batchedTodo,
		"- [ ] fix the leak in heartbeat goroutine",
		"- [ ] fix the leak in heartbeat goroutine — P2", 1)
	writeRepo(t, dir, "batchy", todo)

	pending := harvestBatch(t, dir, 3, 10)
	payload := decodeBatchPayload(t, pending[0])

	for _, want := range []string{
		"3 related items, ONE session",
		"1. fix the null deref in claim",
		"2. fix the off-by-one in retry ladder",
		"3. fix the leak in heartbeat goroutine",
		"Task-Queue-ID: {{TASK_ID}}",
		"— BLOCKED: <one-line reason>",
		"already ticked [x] is done",
		"you MAY append NEW unchecked",
		"Never push",
		".crushrc",
		".tq-verify",
	} {
		if !strings.Contains(payload.Prompt, want) {
			t.Errorf("batch prompt lost %q", want)
		}
	}

	if strings.Contains(payload.Prompt, "TQ_RESULT") {
		t.Errorf("batch prompt teaches the retired self-report line")
	}

	if payload.TimeoutMinutes != 90 { // 3 items x 30-min default
		t.Errorf("batch timeout = %d min, want 90 (3 x default 30)", payload.TimeoutMinutes)
	}

	if payload.MarkerLevel != 2 {
		t.Errorf("batch marker level = %d, want the max member marker 2", payload.MarkerLevel)
	}
}

// TestBatchKeyDeterminism pins the dedup semantics: the same member set in a
// different file order yields the same key; editing one member forks it.
func TestBatchKeyDeterminism(t *testing.T) {
	mk := func(texts ...string) []Item {
		items := make([]Item, len(texts))
		for i, text := range texts {
			items[i] = Item{Text: text, Key: ItemKey("batchy", text)}
		}

		return items
	}

	base := batchKeyOf(mk("a", "b", "c"))
	if base != batchKeyOf(mk("c", "a", "b")) {
		t.Error("same member set in different order must not fork the batch key")
	}

	if base == batchKeyOf(mk("a", "b", "edited")) {
		t.Error("edited member must fork the batch key")
	}

	if !strings.HasPrefix(base, BatchKeyPrefix) {
		t.Errorf("batch key = %q, want %s prefix", base, BatchKeyPrefix)
	}
}

// TestBatchSurveyTracksMembers pins the loop guard: after a batch is pending,
// the next harvest run must not re-attempt its member items (the survey
// knows them through the payload's member keys).
func TestBatchSurveyTracksMembers(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "batchy", batchedTodo)

	q := openQueue(t)
	h := New(q, Config{ProjectsDir: dir, BatchItems: 3, MaxPerTick: 10, MaxPendingPerRepo: 4})
	ctx := context.Background()

	for range 3 {
		if _, err := h.Run(ctx); err != nil {
			t.Fatal(err)
		}
	}

	pending, err := q.List(ctx, pendingFilter("batchy"))
	if err != nil {
		t.Fatal(err)
	}

	// Bugs batch + the post-blocker single + the Refactoring batch — and NO
	// member re-attempt: the survey tracked the first batch's members.
	if len(pending) != 3 {
		t.Fatalf("pending tasks after three runs = %d, want 3 (no member re-attempts):\n%+v", len(pending), pending)
	}

	covered := map[string]int{}

	for _, tk := range pending {
		payload := decodeBatchPayload(t, tk)

		if strings.HasPrefix(payload.Dedup, BatchKeyPrefix) {
			if len(payload.Items) != len(payload.ItemKeys) {
				t.Errorf("batch payload items/keys mismatch: %+v", payload)
			}

			for _, text := range payload.Items {
				covered[text]++
			}
		} else {
			covered[payload.Item]++
		}
	}

	for text, n := range covered {
		if n != 1 {
			t.Errorf("item %q covered by %d tasks, want exactly 1 (member re-attempt leak)", text, n)
		}
	}

	if len(covered) != 6 { // the six open items of batchedTodo
		t.Errorf("covered items = %d (%v), want all 6 open items", len(covered), covered)
	}
}

// TestBatchPriorityTakesMax pins that a batch never sinks below its hottest
// member: one P1 marker item lifts the whole batch to the marker band.
func TestBatchPriorityTakesMax(t *testing.T) {
	dir := t.TempDir()
	todo := `# H

- [ ] lowly chore one
- [ ] urgent thing — P1
- [ ] lowly chore two
`
	writeRepo(t, dir, "batchy", todo)

	pending := harvestBatch(t, dir, 3, 10)
	if len(pending) != 1 {
		t.Fatalf("pending = %d, want one batch", len(pending))
	}

	batch := pending
	if batch[0].Priority <= 0 {
		t.Errorf("batch priority = %d, want the max member priority (P1 marker band)", batch[0].Priority)
	}
}

// TestBatchPruneAllTicked pins the batch ticked rule: a pending batch is
// withdrawn only when EVERY member checkbox is [x].
func TestBatchPruneAllTicked(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "batchy", batchedTodo)

	q := openQueue(t)
	h := New(q, Config{ProjectsDir: dir, BatchItems: 3, MaxPerTick: 10})
	ctx := context.Background()

	if _, err := h.Run(ctx); err != nil {
		t.Fatal(err)
	}

	pending, err := q.List(ctx, pendingFilter("batchy"))
	if err != nil || len(pending) != 1 {
		t.Fatalf("setup: pending=%d err=%v", len(pending), err)
	}

	batchID := pending[0].ID
	payload := decodeBatchPayload(t, pending[0])

	// Partial: one member done by hand — the batch must SURVIVE.
	mustWrite(t, dir+"/batchy/"+DefaultTodoFile, strings.Replace(batchedTodo,
		"- [ ] "+payload.Items[0], "- [x] "+payload.Items[0], 1))

	res, err := h.PruneStale(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Cancelled) != 0 {
		t.Fatalf("partially-ticked batch was cancelled: %+v", res.Cancelled)
	}

	// All members done — the batch is a zombie and must be withdrawn.
	mustWrite(t, dir+"/batchy/"+DefaultTodoFile, `# Project

## Bugs

- [x] fix the null deref in claim
- [x] fix the off-by-one in retry ladder
- [x] fix the leak in heartbeat goroutine
- [ ] fix docs typo — BLOCKED: owner must decide wording
- [ ] fix the race in sweeper watermark

## Refactoring

- [ ] extract the prompt renderer
- [ ] split the survey pass
`)

	res, err = h.PruneStale(ctx)
	if err != nil {
		t.Fatal(err)
	}

	cancelled := false

	for _, c := range res.Cancelled {
		if c.TaskID == batchID {
			cancelled = true
		}
	}

	if !cancelled {
		t.Fatalf("all-ticked batch was not cancelled: %+v", res.Cancelled)
	}
}

// TestBatchPruneAllAbsent pins the batch absent rule: when every member key
// is gone from the file (reworded away), the pending batch is withdrawn —
// and the generic absent pass must NOT have judged the `batch:` key itself.
func TestBatchPruneAllAbsent(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "batchy", batchedTodo)

	q := openQueue(t)
	h := New(q, Config{ProjectsDir: dir, BatchItems: 3, MaxPerTick: 10})
	ctx := context.Background()

	if _, err := h.Run(ctx); err != nil {
		t.Fatal(err)
	}

	pending, err := q.List(ctx, pendingFilter("batchy"))
	if err != nil || len(pending) != 1 {
		t.Fatalf("setup: pending=%d err=%v", len(pending), err)
	}

	// Every batch member reworded beyond recognition.
	mustWrite(t, dir+"/batchy/"+DefaultTodoFile, `# Project

## Bugs

- [ ] completely different work item

## Refactoring

- [ ] extract the prompt renderer
- [ ] split the survey pass
`)

	res, err := h.PruneStale(ctx)
	if err != nil {
		t.Fatal(err)
	}

	found := false

	for _, c := range res.Cancelled {
		if c.TaskID == pending[0].ID {
			found = true
		}
	}

	if !found {
		t.Fatalf("all-absent batch was not cancelled: %+v", res.Cancelled)
	}
}

// TestBatchAuditMemberDrift pins drift semantics under batching: open
// members under a COMPLETED batch are stale-open and mint per-item
// catch-ups; a ticked member under an UNFINISHED batch is reported
// stale-done, never repaired.
func TestBatchAuditMemberDrift(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "batchy", batchedTodo)

	q := openQueue(t)
	h := New(q, Config{ProjectsDir: dir, BatchItems: 3, MaxPerTick: 10})
	ctx := context.Background()

	if _, err := h.Run(ctx); err != nil {
		t.Fatal(err)
	}

	pending, err := q.List(ctx, pendingFilter("batchy"))
	if err != nil || len(pending) != 1 {
		t.Fatalf("setup: pending=%d err=%v", len(pending), err)
	}

	batch := pending[0]

	// Scenario A: the batch COMPLETED; every member checkbox stayed open —
	// per-item stale-open, per-item catch-ups.
	if _, err := q.ClaimDue(ctx, "tester", time.Minute); err != nil {
		t.Fatal(err)
	}

	if err := q.Complete(ctx, batch.ID, "tester", nil); err != nil {
		t.Fatal(err)
	}

	res, err := h.Audit(ctx)
	if err != nil {
		t.Fatal(err)
	}

	staleOpen := 0

	for _, d := range res.StaleOpen {
		if d.TaskID == batch.ID {
			staleOpen++
		}
	}

	if staleOpen != 3 {
		t.Fatalf("stale-open members under completed batch = %d, want 3: %+v", staleOpen, res.StaleOpen)
	}

	if len(res.Enqueued) != 3 {
		t.Fatalf("catch-ups minted = %d, want one per open member: %+v", len(res.Enqueued), res.Enqueued)
	}

	// Scenario B (fresh state): a PENDING batch with one member ticked
	// early — stale-done, report-only.
	dir2 := t.TempDir()
	writeRepo(t, dir2, "batchy", batchedTodo)

	q2 := openQueue(t)
	h2 := New(q2, Config{ProjectsDir: dir2, BatchItems: 3, MaxPerTick: 10})

	if _, err := h2.Run(ctx); err != nil {
		t.Fatal(err)
	}

	batchyPending, err := q2.List(ctx, pendingFilter("batchy"))
	if err != nil {
		t.Fatal(err)
	}

	if len(batchyPending) != 1 {
		t.Fatalf("scenario B setup: pending=%d, want the batch", len(batchyPending))
	}

	second := decodeBatchPayload(t, batchyPending[0])
	mustWrite(t, dir2+"/batchy/"+DefaultTodoFile, strings.Replace(batchedTodo,
		"- [ ] "+second.Items[0], "- [x] "+second.Items[0], 1))

	res2, err := h2.Audit(ctx)
	if err != nil {
		t.Fatal(err)
	}

	foundStaleDone := false

	for _, d := range res2.StaleDone {
		if d.TaskID == batchyPending[0].ID {
			foundStaleDone = true
		}
	}

	if !foundStaleDone {
		t.Errorf("ticked member under unfinished batch not reported stale-done: %+v", res2.StaleDone)
	}
}

// TestSinglePathUntouchedByBatchConfig pins the fleet default: BatchItems 0/1
// behaves exactly like the legacy single-item path (one item per task, todo:
// dedup key, no batch fields in the payload).
func TestSinglePathUntouchedByBatchConfig(t *testing.T) {
	for _, batchItems := range []int{0, 1} {
		dir := t.TempDir()
		writeRepo(t, dir, "batchy", "# H\n- [ ] solo item\n- [ ] second item\n")

		pending := harvestBatch(t, dir, batchItems, 10)
		if len(pending) != 1 {
			t.Fatalf("batchItems=%d: pending = %d, want 1 (legacy pacing)", batchItems, len(pending))
		}

		payload := decodeBatchPayload(t, pending[0])
		if !strings.HasPrefix(payload.Dedup, "todo:") {
			t.Fatalf("batchItems=%d: dedup = %q, want a todo: key", batchItems, payload.Dedup)
		}

		if payload.Items != nil || payload.ItemKeys != nil {
			t.Errorf("batchItems=%d: single-item payload carries batch fields", batchItems)
		}
	}
}

// TestBatchExecutorPayloadFields pins that the executor-side payload contract
// decodes the batch fields (informational Items; the executor itself needs
// nothing from them — the prompt IS the batch).
func TestBatchExecutorPayloadFields(t *testing.T) {
	var payload executor.AgentPayload

	body := `{"repo":"r","prompt":"p","item":"first","items":["first","second"]}`
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatal(err)
	}

	if payload.Item != "first" || len(payload.Items) != 2 || payload.Items[1] != "second" {
		t.Fatalf("decoded = %+v, want item+items", payload)
	}
}
