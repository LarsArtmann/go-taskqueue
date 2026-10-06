package main

import (
	"context"
	"encoding/json/jsontext"
	"path/filepath"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/readmodel"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TestDoctorProjectionSection pins the ADR-0019 projection visibility in
// `tq doctor`: cursor lag against the journal head, the projection
// file's presence (including the delete-to-replay escape hatch), and
// folded-count drift against the queue's own counts.
func TestDoctorProjectionSection(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "q.db")

	store, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	results, err := runDoctor(ctx, doctorOptions{DBPath: path})
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	if r := resultByName(results, "projection-cursor"); r.Status != checkOK {
		t.Errorf("fresh cursor check = %s (%s), want ok", r.Status, r.Detail)
	}

	if r := resultByName(results, "projection-db"); r.Status != checkOK {
		t.Errorf("fresh db check = %s (%s), want ok", r.Status, r.Detail)
	}

	// One folded task, cursor at head, no projection db yet: the missing
	// file under a nonzero cursor is the documented delete-to-replay
	// escape hatch, not a malfunction.
	if _, err := store.Enqueue(ctx, task.New{Project: "web", Type: "sh", Payload: jsontext.Value(`"x"`)}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if err := store.SaveWatermark(ctx, readmodel.CursorConsumer, 1); err != nil {
		t.Fatalf("save cursor: %v", err)
	}

	results, err = runDoctor(ctx, doctorOptions{DBPath: path})
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	if r := resultByName(results, "projection-db"); r.Status != checkOK {
		t.Errorf("replay-hatch check = %s (%s), want ok", r.Status, r.Detail)
	}

	// Lag: the cursor sits behind the head while serve is down.
	if _, err := store.Enqueue(ctx, task.New{Project: "web", Type: "sh", Payload: jsontext.Value(`"y"`)}); err != nil {
		t.Fatalf("enqueue 2: %v", err)
	}

	results, err = runDoctor(ctx, doctorOptions{DBPath: path})
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	if r := resultByName(results, "projection-cursor"); r.Status != checkWarn {
		t.Errorf("lag check = %s (%s), want warn", r.Status, r.Detail)
	}

	// Fold parity: fold the journal, then grow it — the stale fold drifts
	// from the queue's counts.
	projPath := readmodel.PathFor(path)

	m, err := readmodel.Open(ctx, projPath, store, readmodel.WithDurableCursor())
	if err != nil {
		t.Fatalf("open model: %v", err)
	}

	if err := m.CatchUp(ctx); err != nil {
		t.Fatalf("catch up: %v", err)
	}

	if err := m.Close(); err != nil {
		t.Fatalf("close model: %v", err)
	}

	time.Sleep(2 * time.Millisecond)

	if _, err := store.Enqueue(ctx, task.New{Project: "web", Type: "sh", Payload: jsontext.Value(`"y"`)}); err != nil {
		t.Fatalf("enqueue 2: %v", err)
	}

	results, err = runDoctor(ctx, doctorOptions{DBPath: path})
	if err != nil {
		t.Fatalf("runDoctor: %v", err)
	}

	if r := resultByName(results, "projection-db"); r.Status != checkWarn {
		t.Errorf("drift check = %s (%s), want warn", r.Status, r.Detail)
	}
}
