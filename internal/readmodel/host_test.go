package readmodel_test

import (
	"context"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	"github.com/larsartmann/go-cqrs-lite/id/v4"
	"github.com/larsartmann/go-cqrs-lite/projectionhost/v4"
	"github.com/larsartmann/go-taskqueue/internal/journal/cqrs"
	"github.com/larsartmann/go-taskqueue/internal/queue/sqlite"
	"github.com/larsartmann/go-taskqueue/internal/readmodel"
	"github.com/larsartmann/go-taskqueue/internal/task"
	"github.com/oklog/ulid/v2"
)

func wantEventIDForSeq(t *testing.T, seq int64) string {
	t.Helper()

	eid, err := readmodel.SeqToEventID(seq)
	if err != nil {
		t.Fatalf("seq %d: %v", seq, err)
	}

	return eid.String()
}

func mustEventID(t *testing.T, seq int64) id.EventID {
	t.Helper()

	eid, err := readmodel.SeqToEventID(seq)
	if err != nil {
		t.Fatalf("seq %d: %v", seq, err)
	}

	return eid
}

func foreignEventID(t *testing.T) id.EventID {
	t.Helper()

	eid, err := id.ParseEventID(ulid.Make().String())
	if err != nil {
		t.Fatalf("parse random ulid: %v", err)
	}

	return eid
}

// TestSeqIDCodecParity pins the host-side seq↔EventID codec to the cqrs
// journal's: every event the real FactJournal mints must carry exactly
// the ID the watermark adapter decodes. A drift here would checkpoint
// cursors the journal can never resume from.
func TestSeqIDCodecParity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store, err := sqlite.Open(t.TempDir() + "/queue.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	f := &fixture{t: t, store: store}
	for range 3 {
		f.enqueue("web", "sh", 1, "codec")
	}

	facts, err := store.Facts(ctx, 0, 100)
	if err != nil {
		t.Fatalf("read facts: %v", err)
	}

	if len(facts) == 0 {
		t.Fatal("no facts seeded")
	}

	jr := cqrs.NewFactJournal(store)
	events, err := jr.ReadAll(ctx)
	if err != nil {
		t.Fatalf("journal read all: %v", err)
	}

	if len(events) != len(facts) {
		t.Fatalf("journal mapped %d events, want %d", len(events), len(facts))
	}

	for i, evt := range events {
		if got := evt.ID().String(); got != wantEventIDForSeq(t, facts[i].Seq) {
			t.Fatalf("event %d id = %s, want the seq-derived form", i, got)
		}
	}
}

// TestWatermarkCheckpointsRoundtrip pins the watermarks-backed
// CheckpointStore: no row is the zero checkpoint, a saved checkpoint
// reads back as the same seq, and a non-sequence-derived ID is refused
// instead of wedging the cursor.
func TestWatermarkCheckpointsRoundtrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store, err := sqlite.Open(t.TempDir() + "/queue.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	f := &fixture{t: t, store: store}
	_ = f.enqueue("web", "sh", 1, "wm")

	cp := readmodel.WatermarkCheckpoints(store)

	got, err := cp.Load(ctx, readmodel.CursorConsumer)
	if err != nil {
		t.Fatalf("load empty: %v", err)
	}

	if !got.IsZero() {
		t.Fatalf("empty load = %s, want the zero checkpoint", got)
	}

	seq := int64(2)
	eid := mustEventID(t, seq)

	if err := cp.Save(ctx, readmodel.CursorConsumer, event.Checkpoint{EventID: eid}); err != nil {
		t.Fatalf("save: %v", err)
	}

	wm, exists, err := store.Watermark(ctx, readmodel.CursorConsumer)
	if err != nil || !exists || wm != seq {
		t.Fatalf("watermark = %d exists=%v err=%v, want %d", wm, exists, err, seq)
	}

	got, err = cp.Load(ctx, readmodel.CursorConsumer)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if got.EventID.String() != eid.String() {
		t.Fatalf("loaded checkpoint = %s, want %s", got.EventID, eid)
	}

	foreign := foreignEventID(t)
	if err := cp.Save(ctx, readmodel.CursorConsumer, event.Checkpoint{EventID: foreign}); err == nil {
		t.Fatal("save accepted a non-sequence-derived event id")
	}
}

// TestFoldProjectionParity proves the two drivers fold identically: one
// model folded by the hand pump (CatchUp), one folded by FoldProjection
// over the journal's events, over the same seeded lifecycle — the rows
// and counters must be indistinguishable.
func TestFoldProjectionParity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store, err := sqlite.Open(t.TempDir() + "/queue.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	f := &fixture{t: t, store: store}

	t1 := f.enqueue("web", "sh", 2, "parity-a")
	f.enqueue("api", "sh", 1, "parity-b")

	tk, claim := f.claim()
	if tk.ID != t1.ID {
		t.Fatalf("claim = %s, want t1", tk.ID)
	}

	f.must("complete", f.store.Complete(ctx, t1.ID, claim, jsontext.Value(`{}`)))

	jr := cqrs.NewFactJournal(store)
	events, err := jr.ReadAll(ctx)
	if err != nil {
		t.Fatalf("journal read all: %v", err)
	}

	if len(events) == 0 {
		t.Fatal("no events")
	}

	pumped := openBare(t, store)
	folded := openBare(t, store)

	if err := pumped.CatchUp(ctx); err != nil {
		t.Fatalf("pump catch up: %v", err)
	}

	fold := readmodel.NewFoldProjection(folded)
	for _, evt := range events {
		if err := fold.Handle(ctx, evt); err != nil {
			t.Fatalf("fold handle %s: %v", evt.ID(), err)
		}
	}

	wantStatus, err := pumped.StatusCounts(ctx)
	if err != nil {
		t.Fatalf("pump status counts: %v", err)
	}

	gotStatus, err := folded.StatusCounts(ctx)
	if err != nil {
		t.Fatalf("fold status counts: %v", err)
	}

	if fmt.Sprint(wantStatus) != fmt.Sprint(gotStatus) {
		t.Fatalf("status counts diverge: pump %v fold %v", wantStatus, gotStatus)
	}

	pumpRows, err := pumped.Tasks(ctx, readmodel.TaskFilter{})
	if err != nil {
		t.Fatalf("pump rows: %v", err)
	}

	foldRows, err := folded.Tasks(ctx, readmodel.TaskFilter{})
	if err != nil {
		t.Fatalf("fold rows: %v", err)
	}

	if len(pumpRows) != len(foldRows) {
		t.Fatalf("rows diverge: pump %d fold %d", len(pumpRows), len(foldRows))
	}

	for i := range pumpRows {
		if pumpRows[i].ID != foldRows[i].ID || pumpRows[i].Status != foldRows[i].Status {
			t.Fatalf("row %d diverges: pump %s/%s fold %s/%s",
				i, pumpRows[i].ID, pumpRows[i].Status, foldRows[i].ID, foldRows[i].Status)
		}
	}
}

// TestProjectionHostAdvancesPastPoison pins the platform contract the
// adoption buys: a poison event is dead-lettered after the threshold and
// the checkpoint advances past it — one bad fact wedges nothing.
func TestProjectionHostAdvancesPastPoison(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	store, err := sqlite.Open(t.TempDir() + "/queue.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	t.Cleanup(func() { _ = store.Close() })

	f := &fixture{t: t, store: store}
	tk := f.enqueue("web", "sh", 1, "poison")

	proj := t.TempDir() + "/projection.db"

	m, err := readmodel.Open(ctx, proj, store, readmodel.WithDurableCursor())
	if err != nil {
		t.Fatalf("open model: %v", err)
	}

	t.Cleanup(func() { _ = m.Close() })

	dlq := projectionhost.NewMemoryDeadLetterStore()
	host, err := readmodel.NewProjectionHost(store, m, readmodel.ProjectionHostOptions{
		BatchSize:           10,
		MaxRestarts:         2,
		DeadLetterStore:     dlq,
		DeadLetterThreshold: 1,
	})
	if err != nil {
		t.Fatalf("build host: %v", err)
	}

	// The poison: the projection under test refuses the seed fact.
	host.Register(boomProjection{})

	runCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	go func() { _ = host.Start(runCtx) }()

	waitFor(t, ctx, func() bool {
		n, err := dlq.Count(ctx)
		return err == nil && n > 0
	})

	if err := host.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// The checkpoint moved past the poison: the watermark is nonzero and
	// the tail after it is empty (nothing wedged, nothing re-fetched).
	wm, exists, err := store.Watermark(ctx, readmodel.CursorConsumer)
	if err != nil || !exists || wm == 0 {
		t.Fatalf("watermark after poison = %d exists=%v err=%v, want advanced", wm, exists, err)
	}

	entries, err := dlq.List(ctx, readmodel.CursorConsumer)
	if err != nil || len(entries) == 0 {
		t.Fatalf("dlq entries = %v %v, want the poison captured", entries, err)
	}

	// Sanity: the queue itself is untouched by the poison path.
	got, err := store.Get(ctx, tk.ID)
	if err != nil || got.Status != task.Pending {
		t.Fatalf("seed task = %s %v, want pending", got.Status, err)
	}
}

// boomProjection refuses every event, deterministically poisoning the
// stream.
type boomProjection struct{}

func (boomProjection) Name() string { return readmodel.CursorConsumer }

func (boomProjection) EventTypes() []event.Type {
	return []event.Type{event.Type("task.enqueued")}
}

func (boomProjection) Handle(context.Context, event.Event) error {
	return errors.New("boom: deliberate poison")
}

func openBare(t *testing.T, store *sqlite.Store) *readmodel.Model {
	t.Helper()

	m, err := readmodel.Open(context.Background(), t.TempDir()+"/projection.db", store)
	if err != nil {
		t.Fatalf("open bare model: %v", err)
	}

	t.Cleanup(func() { _ = m.Close() })

	return m
}

func waitFor(t *testing.T, ctx context.Context, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("condition never held within 10s")
}
