package harvest

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestReprioritize pins the sweep: marker edits reach pending tasks,
// hot/machine tasks are protected, same-value resolutions are no-ops,
// and dry-run changes nothing.
func TestReprioritize(t *testing.T) {
	ctx := context.Background()
	projects := t.TempDir()

	repo := writeRepo(t, projects, "repri", "- [ ] Fix the deploy — P1\n- [ ] unmarked item\n- [ ] gone from file\n")
	writeMetadataTo(t, repo, "importance: 80\n")

	deployKey := itemKey("repri", "Fix the deploy")

	tq := openQueue(t)
	h := New(tq, Config{
		Repos:          []string{repo},
		Type:           "agent",
		TodoFile:       DefaultTodoFile,
		MaxPerTick:     10,
		Priority:       5,
		PromptTemplate: "work {{ITEM}}",
	})

	// First run with importance OFF: the P1 item lands at 90 (marker is
	// unconditional), the unmarked at flat 5. The "gone" item never
	// enqueues (one-per-repo pacing admits one item per run — run until
	// all three exist).
	for range 3 {
		if _, err := h.Run(ctx); err != nil {
			t.Fatalf("Run: %v", err)
		}
	}

	pending := pendingPriorities(t, tq, "repri")
	if pending[deployKey] != 90 {
		t.Fatalf("P1 item priority = %d, want 90", pending[deployKey])
	}

	// Second phase: importance now ON, and the owner removed the marker
	// from the deploy item (plain text now, resolves to importance 80).
	if err := overwriteRepoTodo(t, repo, "- [ ] Fix the deploy\n- [ ] unmarked item\n- [ ] gone from file\n"); err != nil {
		t.Fatal(err)
	}

	h2 := New(tq, Config{
		Repos:          []string{repo},
		Type:           "agent",
		TodoFile:       DefaultTodoFile,
		MaxPerTick:     10,
		Priority:       5,
		UseImportance:  true,
		PromptTemplate: "work {{ITEM}}",
	})

	// Dry-run first: reports, changes nothing.
	changes, failures := h2.Reprioritize(ctx, true)
	if len(failures) != 0 {
		t.Fatalf("dry-run failures: %v", failures)
	}

	if len(changes) != 1 || changes[0].ItemText != "Fix the deploy" || changes[0].NewPriority != 80 {
		t.Fatalf("dry-run changes = %+v, want one 90->80 for the de-marked item", changes)
	}

	if pending := pendingPriorities(t, tq, "repri"); pending[deployKey] != 90 {
		t.Fatalf("dry-run mutated the store: %d", pending[deployKey])
	}

	// Real run: the de-marked task re-resolves 90 -> 80 with a fact.
	changes, failures = h2.Reprioritize(ctx, false)
	if len(failures) != 0 || len(changes) != 1 {
		t.Fatalf("run changes = %+v, failures = %v", changes, failures)
	}

	if pending := pendingPriorities(t, tq, "repri"); pending[deployKey] != 80 {
		t.Fatalf("post-run priority = %d, want 80", pending[deployKey])
	}

	// Idempotency: a second run is a no-op (no changes, no new facts).
	changes, failures = h2.Reprioritize(ctx, false)
	if len(failures) != 0 || len(changes) != 0 {
		t.Fatalf("second run changes = %+v, failures = %v — must be a no-op", changes, failures)
	}
}

// TestReprioritizeProtectsBands pins ADR-0015 §3's band protection: a
// hot-band task (same-session) is never rewritten by the sweep.
func TestReprioritizeProtectsBands(t *testing.T) {
	ctx := context.Background()
	projects := t.TempDir()

	repo := writeRepo(t, projects, "hot", "- [ ] chase /tmp/evidence now\n")
	writeMetadataTo(t, repo, "importance: 20\n")

	tq := openQueue(t)
	h := New(tq, Config{
		Repos:               []string{repo},
		Type:                "agent",
		TodoFile:            DefaultTodoFile,
		MaxPerTick:          10,
		SameSessionPriority: 120,
		UseImportance:       true,
		PromptTemplate:      "work {{ITEM}}",
	})

	if _, err := h.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if pending := pendingPriorities(t, tq, "hot"); pending[itemKey("hot", "chase /tmp/evidence now")] != 120 {
		t.Fatalf("hot item priority = %d, want 120", pending[itemKey("hot", "chase /tmp/evidence now")])
	}

	changes, failures := h.Reprioritize(ctx, false)
	if len(failures) != 0 || len(changes) != 0 {
		t.Fatalf("sweep touched a hot task: changes = %+v, failures = %v", changes, failures)
	}

	if pending := pendingPriorities(t, tq, "hot"); pending[itemKey("hot", "chase /tmp/evidence now")] != 120 {
		t.Fatalf("hot task rewritten: %d", pending[itemKey("hot", "chase /tmp/evidence now")])
	}
}

func overwriteRepoTodo(t *testing.T, repo, todo string) error {
	t.Helper()

	return os.WriteFile(filepath.Join(repo, DefaultTodoFile), []byte(todo), 0o644)
}

// pendingPriorities indexes one repo's PENDING tasks by their item dedup
// key (the same payload derivation the sweep matches on).
func pendingPriorities(t *testing.T, tq *queue.Queue, repoName string) map[string]int {
	t.Helper()

	ctx := context.Background()
	project := repoName
	taskType := "agent"

	tasks, err := tq.List(ctx, queue.Filter{Project: &project, Type: &taskType})
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	out := map[string]int{}

	for _, tk := range tasks {
		if tk.Status != task.Pending {
			continue
		}

		if key := payloadDedup(tk); key != "" {
			out[key] = tk.Priority
		}
	}

	return out
}

func itemKey(repoName, text string) string { return ItemKey(repoName, text) }

// TestReprioritizeAppliesCachedAIScores pins the cache feed of the repri
// pass (the same ADR-0015 §3 ladder the enqueue path uses): a cached
// verdict re-resolves an unmarked item to its clamped score with source
// "ai", while a marked item keeps its marker priority — marker > AI —
// even with a cached verdict of its own.
func TestReprioritizeAppliesCachedAIScores(t *testing.T) {
	ctx := context.Background()
	projects := t.TempDir()

	repo := writeRepo(t, projects, "airepri", `- [ ] plain work
- [ ] pinned work — P1
`)
	plainKey := itemKey("airepri", "plain work")
	pinnedKey := itemKey("airepri", "pinned work")

	tq := openQueue(t)
	h := New(tq, Config{
		Repos:             []string{repo},
		Type:              "agent",
		TodoFile:          DefaultTodoFile,
		MaxPerTick:        10,
		Priority:          5,
		PromptTemplate:    "work {{ITEM}}",
		MaxPendingPerRepo: 2,
	})

	for range 2 {
		if _, err := h.Run(ctx); err != nil {
			t.Fatalf("Run: %v", err)
		}
	}

	pending := pendingPriorities(t, tq, "airepri")
	if pending[plainKey] != 5 || pending[pinnedKey] != 90 {
		t.Fatalf("seed priorities = %v, want plain 5 / pinned 90", pending)
	}

	if err := tq.SavePriorityScore(ctx, queue.PriorityScore{
		ItemKey: plainKey, Score: 42, Source: "ai:batch-scorer",
	}); err != nil {
		t.Fatalf("cache plain score: %v", err)
	}

	if err := tq.SavePriorityScore(ctx, queue.PriorityScore{
		ItemKey: pinnedKey, Score: 10, Source: "ai:batch-scorer",
	}); err != nil {
		t.Fatalf("cache pinned score: %v", err)
	}

	changes, failures := h.Reprioritize(ctx, false)
	if len(failures) != 0 {
		t.Fatalf("failures: %v", failures)
	}

	if len(changes) != 1 || changes[0].ItemText != "plain work" ||
		changes[0].NewPriority != 42 || changes[0].Source != PrioritySourceAI {
		t.Fatalf("changes = %+v, want one 5->42 ai change for the plain item", changes)
	}

	pending = pendingPriorities(t, tq, "airepri")
	if pending[plainKey] != 42 || pending[pinnedKey] != 90 {
		t.Fatalf("post-repri priorities = %v, want 42 / 90 (marker beats AI)", pending)
	}

	// The enqueue path also pins the marker level into the payload, so
	// automated re-resolution can honor marker precedence from the store
	// alone.
	agentType := "agent"
	projectName := "airepri"

	tasks, err := tq.List(ctx, queue.Filter{Project: &projectName, Type: &agentType})
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	for _, tk := range tasks {
		item, ok := PayloadItemOf(tk)
		if !ok {
			t.Fatalf("harvest-minted task %s has no payload item", tk.ID)
		}

		wantMarker := 0
		if item.Key == pinnedKey {
			wantMarker = 1
		}

		if item.MarkerLevel != wantMarker {
			t.Fatalf("item %s markerLevel = %d, want %d", item.Key, item.MarkerLevel, wantMarker)
		}
	}
}
