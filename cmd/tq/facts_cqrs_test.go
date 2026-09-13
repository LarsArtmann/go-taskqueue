package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-codec"
	"github.com/larsartmann/go-cqrs-lite/event/v4"
	"github.com/larsartmann/go-cqrs-lite/id/v4"
	"github.com/larsartmann/go-taskqueue/internal/journal/cqrs"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestFactsCQRSOverStore pins the adapter contract against the real SQLite
// store: fact ordering, cursor resume, foreign-cursor safety, and the CLI
// render path all see the same event stream.
func TestFactsCQRSOverStore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "cqrs.db")

	store, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	defer store.Close()

	ctx := context.Background()

	if _, err := store.Enqueue(ctx, task.New{
		Type:        "sh",
		Project:     "cqrs-interop",
		Payload:     []byte(`"echo hi"`),
		MaxAttempts: 1,
	}); err != nil {
		t.Fatalf("enqueue first: %v", err)
	}

	if _, err := store.Enqueue(ctx, task.New{
		Type:        "sh",
		Project:     "cqrs-interop",
		Payload:     []byte(`"echo bye"`),
		MaxAttempts: 1,
	}); err != nil {
		t.Fatalf("enqueue second: %v", err)
	}

	claimed, err := store.ClaimDue(ctx, "worker-a", time.Minute)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if err := store.Complete(ctx, claimed.ID, "worker-a", nil); err != nil {
		t.Fatalf("complete: %v", err)
	}

	j := cqrs.NewFactJournal(store)

	events, err := j.ReadAll(ctx)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	if len(events) != 4 {
		t.Fatalf("ReadAll returned %d events, want 4 (enqueued x2, claimed, completed)", len(events))
	}

	assertEventsEncodingStamped(t, events)

	if string(events[3].StreamID().String()) != claimed.ID.String() {
		t.Fatalf("completed event stream = %s, want task %s", events[3].StreamID(), claimed.ID)
	}

	if string(events[3].Type()) != "task.completed" {
		t.Fatalf("fourth event type = %s, want task.completed", events[3].Type())
	}

	resumed, err := j.ReadFrom(ctx, events[0].ID(), 0)
	if err != nil {
		t.Fatalf("ReadFrom after first: %v", err)
	}

	if len(resumed) != 3 {
		t.Fatalf("resume returned %d events, want 3", len(resumed))
	}

	foreign, err := j.ReadFrom(ctx, id.NewEventID(), 0)
	if err != nil {
		t.Fatalf("ReadFrom with foreign cursor: %v", err)
	}

	if len(foreign) != 0 {
		t.Fatalf("foreign cursor drained %d events, want 0", len(foreign))
	}

	drained, err := j.ReadFrom(ctx, events[3].ID(), 0)
	if err != nil {
		t.Fatalf("ReadFrom at head: %v", err)
	}

	if len(drained) != 0 {
		t.Fatalf("head cursor drained %d events, want 0", len(drained))
	}

	var rendered []map[string]any

	out := captureStdout(t, func() {
		if err := cmdFacts([]string{"--db", dbPath, "--cqrs"}); err != nil {
			t.Errorf("cmdFacts --cqrs: %v", err)
		}
	})

	if err := json.NewDecoder(strings.NewReader(out)).Decode(&rendered); err != nil {
		t.Fatalf("rendered output is not a JSON array: %v\n%s", err, out)
	}

	if len(rendered) != 4 {
		t.Fatalf("rendered %d events, want 4", len(rendered))
	}

	if rendered[3]["type"] != "task.completed" || rendered[3]["version"] != float64(4) {
		t.Fatalf("rendered fourth event = %v", rendered[3])
	}
}

// assertEventsEncodingStamped pins the store-backed path to the encoding
// contract the adapter's unit tests carry: downstream DecodePayloadAuto
// refuses unstamped payloads (the bug the 2026-09-13 lint pass fixed).
func assertEventsEncodingStamped(t *testing.T, events []event.Event) {
	t.Helper()

	for i, evt := range events {
		if evt.Encoding() != codec.EncodingJSON {
			t.Fatalf("store event %d encoding = %q, want %q", i, evt.Encoding(), codec.EncodingJSON)
		}
	}
}
