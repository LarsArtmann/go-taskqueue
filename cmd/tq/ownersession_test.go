package main

import (
	"context"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/session"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestLiveOwnerRepos pins the repo-resolution join: a fresh ping inside a
// repo (at any depth) marks that repo live; stale pings, foreign dirs and
// the projects dir itself do not.
func TestLiveOwnerRepos(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	now := time.Now()

	entries := []session.RegistryEntry{
		{ID: "a", CWD: base + "/alpha", LastSeen: now.Add(-time.Minute)},
		{ID: "b", CWD: base + "/beta/sub/deep", LastSeen: now.Add(-2 * time.Minute)},
		{ID: "stale", CWD: base + "/gamma", LastSeen: now.Add(-time.Hour)},
		{ID: "foreign", CWD: "/tmp/elsewhere", LastSeen: now},
		{ID: "root", CWD: base, LastSeen: now},
	}

	live := liveOwnerRepos(entries, base, now)
	if !live["alpha"] || !live["beta"] {
		t.Fatalf("live = %v, want alpha+beta", live)
	}

	if live["gamma"] || live["elsewhere"] || live[""] {
		t.Fatalf("stale/foreign/root leaked into live set: %v", live)
	}
}

// TestParkBehindOwnerSessions pins the two-writer policy end-to-end: the
// repo with a live session parks (claim suppressed, one fact), the quiet
// repo stays claimable, and a second pass within the hold window mints no
// additional park fact.
func TestParkBehindOwnerSessions(t *testing.T) {
	base := t.TempDir()
	registry := base + "/registry.jsonl"
	t.Setenv("TQ_SESSION_REGISTRY", registry)

	store := journalAuditStore(t)
	ctx := context.Background()

	held, err := store.Enqueue(ctx, task.New{Project: "alpha", Type: "sh", Payload: []byte(`"true"`)})
	if err != nil {
		t.Fatalf("enqueue held: %v", err)
	}

	quiet, err := store.Enqueue(ctx, task.New{Project: "beta", Type: "sh", Payload: []byte(`"true"`)})
	if err != nil {
		t.Fatalf("enqueue quiet: %v", err)
	}

	if err := session.PingRegistry(registry, "s1", base+"/alpha/sub", time.Now()); err != nil {
		t.Fatalf("ping: %v", err)
	}

	parkBehindOwnerSessions(ctx, store, base, 15*time.Minute, nil)

	// Held repo: suppressed claims + exactly one park fact so far.
	heldFresh, err := store.Get(ctx, held.ID)
	if err != nil || !heldFresh.NotBefore.After(time.Now()) {
		t.Fatalf("held task not parked: %+v (%v)", heldFresh, err)
	}

	facts, err := store.FactsForTask(ctx, held.ID.String(), 0)
	if err != nil {
		t.Fatalf("facts: %v", err)
	}

	parkFacts := 0

	for _, f := range facts {
		if f.Type == "task.parked" {
			parkFacts++
		}
	}

	if parkFacts != 1 {
		t.Fatalf("park facts = %d, want 1", parkFacts)
	}

	// Quiet repo: untouched, still claimable.
	quietFresh, err := store.Get(ctx, quiet.ID)
	if err != nil || quietFresh.NotBefore.After(time.Now()) {
		t.Fatalf("quiet repo leaked a park: %+v (%v)", quietFresh, err)
	}

	// Second pass inside the hold window: already suppressed, no new fact.
	parkBehindOwnerSessions(ctx, store, base, 15*time.Minute, nil)

	facts2, err := store.FactsForTask(ctx, held.ID.String(), 0)
	if err != nil {
		t.Fatalf("facts2: %v", err)
	}

	parkFacts2 := 0

	for _, f := range facts2 {
		if f.Type == "task.parked" {
			parkFacts2++
		}
	}

	if parkFacts2 != 1 {
		t.Fatalf("park facts after second pass = %d, want still 1", parkFacts2)
	}
}
