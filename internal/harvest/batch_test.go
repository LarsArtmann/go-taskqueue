package harvest

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/executor"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// batchedTodo is a two-section backlog: the Bugs section has four fresh
// items around a blocked breaker; Refactoring has two. With BatchItems=3
// the Bugs section chunks [3,1]; the blocked breaker is skipped with its
// own reason and does not fork the chunks; Refactoring forms its own run.
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

// TestBatchGroupsSectionItems pins the grouping contract: with
// BatchItems=3, the first harvest admits ONE task covering the first three
// admissible Bugs items; the blocked breaker is excluded with its own skip
// reason instead of forking the section; the Refactoring run and the
// leftover Bugs item are paced out (one new task per repo per run).
func TestBatchGroupsSectionItems(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "batchy", batchedTodo)

	pending := harvestBatch(t, dir, 3, 10)
	if len(pending) != 1 {
		t.Fatalf("pending tasks = %d, want exactly one batch:\n%+v", len(pending), pending)
	}

	payload := decodeBatchPayload(t, pending[0])
	if len(payload.ItemKeys) != 3 || len(payload.Items) != 3 {
		t.Fatalf("batch members = %v, want the first three admissible Bugs items", payload.Items)
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
		"already-ticked item is done",
		"append NEW unchecked",
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

	// Bugs chunk + the leftover Bugs item + the Refactoring batch — and NO
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

// TestBatchSpansSectionGaps pins the section-scoped grouping (2026-11-03):
// a done item BETWEEN fresh items of one section no longer forks the batch —
// under adjacency grouping this fixture minted a 1-item and a 2-item task;
// now the three fresh items form ONE batch bridging the ticked gap.
func TestBatchSpansSectionGaps(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "batchy", `# Project

## Bugs

- [ ] fix the null deref in claim
- [x] fix the off-by-one in retry ladder
- [ ] fix the leak in heartbeat goroutine
- [ ] fix the race in sweeper watermark
`)

	pending := harvestBatch(t, dir, 3, 10)
	if len(pending) != 1 {
		t.Fatalf("pending tasks = %d, want one gap-bridging batch:\n%+v", len(pending), pending)
	}

	payload := decodeBatchPayload(t, pending[0])

	want := []string{
		"fix the null deref in claim",
		"fix the leak in heartbeat goroutine",
		"fix the race in sweeper watermark",
	}
	if len(payload.Items) != len(want) {
		t.Fatalf("batch members = %v, want the three fresh items around the ticked gap", payload.Items)
	}

	for i, text := range want {
		if payload.Items[i] != text {
			t.Errorf("member %d = %q, want %q (file order preserved)", i, payload.Items[i], text)
		}
	}

	if strings.Contains(strings.Join(payload.Items, "\n"), "off-by-one") {
		t.Error("batch must not carry the ticked (done) item")
	}
}

// TestBatchChunksLargeSections pins the cap chunking: a section with more
// admissible items than BatchItems splits into file-order chunks (7 items,
// cap 3 → 3+3+1), admitted one new task per repo per run.
func TestBatchChunksLargeSections(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "batchy", `# Project

## Bugs

- [ ] item one
- [ ] item two
- [ ] item three
- [ ] item four
- [ ] item five
- [ ] item six
- [ ] item seven
`)

	q := openQueue(t)
	h := New(q, Config{ProjectsDir: dir, BatchItems: 3, MaxPerTick: 10, MaxPendingPerRepo: 7})
	ctx := context.Background()

	for range 3 {
		if _, err := h.Run(ctx); err != nil {
			t.Fatal(err)
		}
	}

	pending, err := q.List(ctx, pendingFilter("batchy"))
	if err != nil || len(pending) != 3 {
		t.Fatalf("pending = %d (err %v), want the three chunks admitted one per run", len(pending), err)
	}

	wantSizes := map[int]int{3: 2, 1: 1} // two 3-member chunks + one 1-member
	sizes := map[int]int{}

	for _, tk := range pending {
		payload := decodeBatchPayload(t, tk)
		sizes[len(payload.Items)]++
	}

	for size, count := range wantSizes {
		if sizes[size] != count {
			t.Errorf("chunks of size %d = %d, want %d (sizes: %v)", size, sizes[size], count, sizes)
		}
	}
}

// TestSectionRunsFoldsByEffortBudget pins the effort-aware fold
// (2026-11-03): the summed member estimates cap a run below the context
// cap, a monster estimate ships solo (never starves its siblings), and
// FLAT estimates under the derived budget (BatchItems x
// DefaultBatchTimeoutMinutes) reproduce the count cap exactly — the
// estimator-less default changes nothing.
func TestSectionRunsFoldsByEffortBudget(t *testing.T) {
	t.Parallel()

	items := []Item{
		{Heading: "A", Key: "a1"},
		{Heading: "A", Key: "a2"},
		{Heading: "A", Key: "a3"},
		{Heading: "B", Key: "b1"},
	}
	all := []bool{true, true, true, true}
	effort := map[string]int{"a1": 60, "a2": 60, "a3": 60, "b1": 10}
	effortOf := func(item Item) int { return effort[item.Key] }

	// Budget 150: a1+a2=120 fits, +a3=180 exceeds → [a1,a2] then [a3];
	// the section boundary starts b1 fresh.
	runs := sectionRuns(items, all, 10, 150, effortOf)

	if got, want := runKeys(runs), [][]string{{"a1", "a2"}, {"a3"}, {"b1"}}; !sameRuns(got, want) {
		t.Fatalf("runs = %v, want %v", got, want)
	}

	// A monster estimate (240) over the budget still forms its own run;
	// the sibling after it is not swallowed or starved.
	monster := []Item{{Heading: "A", Key: "m1"}, {Heading: "A", Key: "a1"}}
	monsterEffort := func(item Item) int {
		if item.Key == "m1" {
			return 240
		}

		return 30
	}

	runs = sectionRuns(monster, []bool{true, true}, 10, 150, monsterEffort)
	if got := runKeys(runs); !sameRuns(got, [][]string{{"m1"}, {"a1"}}) {
		t.Fatalf("monster runs = %v, want the 240m item solo then its sibling", got)
	}

	// Flat equivalence: flat DefaultBatchTimeoutMinutes estimates under
	// the derived budget (3 x 30) chunk exactly at the count cap.
	flat := func(Item) int { return DefaultBatchTimeoutMinutes }
	four := []Item{
		{Heading: "A", Key: "f1"},
		{Heading: "A", Key: "f2"},
		{Heading: "A", Key: "f3"},
		{Heading: "A", Key: "f4"},
	}

	runs = sectionRuns(four, all, 3, 3*DefaultBatchTimeoutMinutes, flat)
	if got := runKeys(runs); !sameRuns(got, [][]string{{"f1", "f2", "f3"}, {"f4"}}) {
		t.Fatalf("flat runs = %v, want count-cap chunking 3+1", got)
	}
}

func runKeys(runs [][]Item) [][]string {
	out := make([][]string, len(runs))
	for i, run := range runs {
		out[i] = make([]string, len(run))
		for j, item := range run {
			out[i][j] = item.Key
		}
	}

	return out
}

func sameRuns(got, want [][]string) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if len(got[i]) != len(want[i]) {
			return false
		}

		for j := range got[i] {
			if got[i][j] != want[i][j] {
				return false
			}
		}
	}

	return true
}

// fakeEffortEstimator stubs Config.EffortEstimator with a text→minutes
// table for the end-to-end effort tests.
type fakeEffortEstimator struct {
	minutes map[string]int
}

func (f fakeEffortEstimator) EstimateEffort(_ context.Context, items []Item) map[string]int {
	out := make(map[string]int)

	for _, item := range items {
		if minutes, ok := f.minutes[item.Text]; ok {
			out[item.Key] = minutes
		}
	}

	return out
}

// TestBatchSizesRunsByEffort pins the estimator-driven sizing end to
// end: estimates split a section below the context cap, the minted
// payload records per-member EffortMinutes, and the payload timeout is
// the MAX of the count ceiling and the effort sum (estimates raise, the
// count floor holds).
func TestBatchSizesRunsByEffort(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "batchy", `# Project

## Bugs

- [ ] item one
- [ ] item two
- [ ] item three
- [ ] item four
`)

	q := openQueue(t)
	h := New(q, Config{
		ProjectsDir:        dir,
		BatchItems:         10,
		BatchEffortMinutes: 150,
		MaxPerTick:         10,
		EffortEstimator: fakeEffortEstimator{minutes: map[string]int{
			"item one":   40,
			"item two":   45,
			"item three": 80,
			"item four":  15,
		}},
	})

	if _, err := h.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	pending, err := q.List(context.Background(), pendingFilter("batchy"))
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending = %d (err %v), want the first effort-sized batch", len(pending), err)
	}

	payload := decodeBatchPayload(t, pending[0])

	// 40+45=85 fits the 150 budget; +80 exceeds → the run stops at two.
	if len(payload.Items) != 2 || payload.Items[0] != "item one" || payload.Items[1] != "item two" {
		t.Fatalf("batch members = %v, want [item one, item two] (effort split below the cap)", payload.Items)
	}

	if len(payload.EffortMinutes) != 2 || payload.EffortMinutes[0] != 40 || payload.EffortMinutes[1] != 45 {
		t.Fatalf("effortMinutes = %v, want [40 45] aligned with Items", payload.EffortMinutes)
	}

	if payload.TimeoutMinutes != 85 { // max(count 2x30=60, effort 40+45=85)
		t.Errorf("timeout = %d min, want 85 (effort sum over the count floor)", payload.TimeoutMinutes)
	}
}

// TestBatchEffortNeverShrinksTimeout pins the floor: under-estimates
// cannot truncate a batch — the count ceiling always holds, while the
// low estimates are still recorded for the later outcome-join audit.
func TestBatchEffortNeverShrinksTimeout(t *testing.T) {
	dir := t.TempDir()
	writeRepo(t, dir, "batchy", `# Project

## Bugs

- [ ] item one
- [ ] item two
- [ ] item three
`)

	q := openQueue(t)
	h := New(q, Config{
		ProjectsDir: dir,
		BatchItems:  3,
		MaxPerTick:  10,
		EffortEstimator: fakeEffortEstimator{minutes: map[string]int{
			"item one":   5,
			"item two":   5,
			"item three": 5,
		}},
	})

	if _, err := h.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	pending, err := q.List(context.Background(), pendingFilter("batchy"))
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending = %d (err %v), want one batch", len(pending), err)
	}

	payload := decodeBatchPayload(t, pending[0])

	if len(payload.Items) != 3 {
		t.Fatalf("batch members = %v, want all three (5m estimates pack under any budget)", payload.Items)
	}

	if payload.TimeoutMinutes != 90 { // max(3x30 floor, 15 sum) — never below count
		t.Errorf("timeout = %d min, want 90 (the count floor; estimates only raise)", payload.TimeoutMinutes)
	}

	if len(payload.EffortMinutes) != 3 || payload.EffortMinutes[2] != 5 {
		t.Fatalf("effortMinutes = %v, want the low estimates recorded anyway", payload.EffortMinutes)
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
	_, claim, err := q.ClaimDue(ctx, "tester", time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	if err := q.Complete(ctx, batch.ID, claim, nil); err != nil {
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
