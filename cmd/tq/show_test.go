package main

import (
	"context"
	"encoding/json/v2"
	"path/filepath"
	"testing"

	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestBuildPriorityProvenance pins the tq show priority section: band and
// current value always, item identity + cached AI verdict for
// harvest-minted tasks, and the reprioritization history distilled from
// the fact trail.
func TestBuildPriorityProvenance(t *testing.T) {
	t.Parallel()

	store, got := seedProvenanceTask(t)

	trail, err := store.FactsForTask(ctxOf(t), got.ID.String(), 0)
	if err != nil {
		t.Fatalf("facts: %v", err)
	}

	provenance := buildPriorityProvenance(context.Background(), store, got, trail)

	if provenance.Current != 85 || provenance.Band != "backlog" {
		t.Fatalf("current/band = %d/%q, want 85/backlog", provenance.Current, provenance.Band)
	}

	if provenance.ItemKey != "todo:abc" || provenance.MarkerLevel != 0 {
		t.Fatalf("item identity = %q/%d, want todo:abc/0", provenance.ItemKey, provenance.MarkerLevel)
	}

	if provenance.CachedScore == nil || provenance.CachedScore.Score != 85 ||
		provenance.CachedScore.Source != "ai:batch-scorer" {
		t.Fatalf("cached score = %+v", provenance.CachedScore)
	}

	if len(provenance.RepriHistory) != 1 {
		t.Fatalf("repri history = %+v, want one event", provenance.RepriHistory)
	}

	event := provenance.RepriHistory[0]
	if event.Old != 50 || event.New != 85 || event.Source != "ai" || event.Reason != "unblocks the release" {
		t.Fatalf("repri event = %+v", event)
	}

	if event.At == "" {
		t.Fatal("repri event carries no timestamp")
	}
}

// seedProvenanceTask builds a scored, once-reprioritized harvest task over
// a scratch store and returns the store plus the task's current record.
func seedProvenanceTask(t *testing.T) (*sqlite.Store, task.Task) {
	t.Helper()

	ctx := context.Background()

	store, err := sqlite.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	payload, err := json.Marshal(map[string]any{
		"repo":        "demo",
		"prompt":      "work",
		"item":        "Fix the frobnicator",
		"dedup":       "todo:abc",
		"markerLevel": 0,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	enq, err := store.Enqueue(ctx, task.New{
		Type:     "agent",
		Project:  "demo",
		Priority: 50,
		Payload:  payload,
		DedupKey: "todo:abc",
	})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if err := store.SavePriorityScore(ctx, queuePriorityScore()); err != nil {
		t.Fatalf("save score: %v", err)
	}

	if err := store.UpdatePendingPriority(ctx, enq.ID, 85, "ai", "unblocks the release"); err != nil {
		t.Fatalf("reprioritize: %v", err)
	}

	got, err := store.Get(ctx, enq.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	return store, got
}

// TestBuildPriorityProvenanceForeignTask pins the section for a
// non-harvest task: band still reported, item identity and cached score
// omitted, no crash.
func TestBuildPriorityProvenanceForeignTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store, err := sqlite.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	raw, err := json.Marshal(map[string]any{"cmd": "echo hi"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	enq, err := store.Enqueue(ctx, task.New{Type: "sh", Project: "demo", Priority: 150, Payload: raw})
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	provenance := buildPriorityProvenance(ctx, store, enq, nil)

	if provenance.Current != 150 || provenance.Band != "machine" {
		t.Fatalf("current/band = %d/%q, want 150/machine", provenance.Current, provenance.Band)
	}

	if provenance.ItemKey != "" || provenance.CachedScore != nil || provenance.RepriHistory != nil {
		t.Fatalf("foreign task provenance = %+v, want band-only", provenance)
	}
}

func queuePriorityScore() queue.PriorityScore {
	return queue.PriorityScore{
		ItemKey:       "todo:abc",
		Score:         85,
		EffortMinutes: 30,
		Source:        "ai:batch-scorer",
		Reasoning:     "unblocks the release",
	}
}
