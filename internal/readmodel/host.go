package readmodel

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"
	"time"

	"github.com/larsartmann/go-cqrs-lite/event/v4"
	"github.com/larsartmann/go-cqrs-lite/id/v4"
	"github.com/larsartmann/go-cqrs-lite/projection/v4"
	"github.com/larsartmann/go-cqrs-lite/projectionhost/v4"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	cqrs "github.com/larsartmann/go-taskqueue/internal/journal/cqrs"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/oklog/ulid/v2"
)

// The projectionhost adoption (ADR-0019 M08): the fold runs as a managed
// projection over the fact journal instead of the hand-rolled catch-up
// loop — restart budget, poison-fact DLQ, checkpoint batching, and lag
// reporting come from the platform, and the pump shrinks to Handle.

// seqEpochLen and seqBytesLen are the synthetic event-ID layout inside
// the 16 ULID bytes: [0:6] zero timestamp (the sequence-derived marker),
// [6:14] fact Seq big-endian, [14:16] zero tail. Canonical ULID string
// order equals ascending Seq because the first byte is constant '0'.
//
// art-dupl:accept mirror of internal/journal/cqrs/seqid.go: the cqrs
// adapter is the proprietary read-only seam (ADR-0014) and keeps its
// codec unexported; TestSeqIDCodecParity pins both encodings to the same
// byte layout, so a drift fails a gate instead of corrupting cursors.
const (
	seqEpochLen = 6
	seqBytesLen = 8
)

func seqToEventID(seq int64) (id.EventID, error) {
	return SeqToEventID(seq)
}

func eventIDToSeq(eventID id.EventID) (int64, bool) {
	return EventIDToSeq(eventID)
}

// SeqToEventID encodes a journal sequence as the synthetic event ID the
// cqrs journal stamps on every fact event. Exported for the checkpoint
// adapter's consumers (doctor/health surfaces, composition root).
func SeqToEventID(seq int64) (id.EventID, error) {
	if seq <= 0 {
		return id.EventID{}, fmt.Errorf("readmodel: sequence %d is not positive", seq)
	}

	var raw ulid.ULID
	binary.BigEndian.PutUint64(raw[seqEpochLen:seqEpochLen+seqBytesLen], uint64(seq))

	eventID, err := id.ParseEventID(raw.String())
	if err != nil {
		return id.EventID{}, fmt.Errorf("readmodel: encode sequence %d as event id: %w", seq, err)
	}

	return eventID, nil
}

// EventIDToSeq decodes a sequence-derived event ID back to its journal
// sequence; ok is false for any ID not minted by this layout (a random
// or foreign cursor), which callers must treat as "no position".
func EventIDToSeq(eventID id.EventID) (int64, bool) {
	raw := eventID.Get()

	for _, b := range raw[:seqEpochLen] {
		if b != 0 {
			return 0, false
		}
	}

	for _, b := range raw[seqEpochLen+seqBytesLen:] {
		if b != 0 {
			return 0, false
		}
	}

	return int64(binary.BigEndian.Uint64(raw[seqEpochLen : seqEpochLen+seqBytesLen])), true
}

// WatermarkCheckpoints adapts the queue store's watermarks table to the
// platform CheckpointStore, under the projection's own consumer name.
func WatermarkCheckpoints(src queue.Store) event.CheckpointStore {
	return watermarkCheckpoints{src: src}
}

// watermarkCheckpoints adapts the queue's watermarks table (one monotonic
// integer cursor per consumer) to the platform's event.CheckpointStore.
// The projection's Name IS the watermarks consumer, so the host-managed
// pump and the in-model durable cursor (WithDurableCursor) share one
// checkpoint slot by construction.
type watermarkCheckpoints struct {
	src queue.Store
}

// Load returns the stored checkpoint; no row (or a zero seq) is the zero
// Checkpoint — the platform's "no prior progress" shape, replaying from
// the journal start.
func (w watermarkCheckpoints) Load(ctx context.Context, name string) (event.Checkpoint, error) {
	seq, exists, err := w.src.Watermark(ctx, name)
	if err != nil {
		return event.Checkpoint{}, fmt.Errorf("readmodel: load watermark %s: %w", name, err)
	}

	if !exists || seq <= 0 {
		return event.Checkpoint{}, nil
	}

	eid, err := seqToEventID(seq)
	if err != nil {
		return event.Checkpoint{}, fmt.Errorf("readmodel: checkpoint %s: %w", name, err)
	}

	return event.Checkpoint{EventID: eid}, nil
}

// Save stores the checkpoint's event ID as the consumer's seq. A
// non-sequence-derived ID cannot come from this journal's events and is
// refused instead of silently wedging the cursor.
func (w watermarkCheckpoints) Save(ctx context.Context, name string, cp event.Checkpoint) error {
	seq, ok := eventIDToSeq(cp.EventID)
	if !ok {
		return fmt.Errorf("readmodel: checkpoint %s: event id %s is not sequence-derived", name, cp.EventID)
	}

	if err := w.src.SaveWatermark(ctx, name, seq); err != nil {
		return fmt.Errorf("readmodel: save watermark %s: %w", name, err)
	}

	return nil
}

// foldPayload is the wire mirror of the cqrs journal's fact payload
// (journal/cqrs factEvent stamps it through the JSON codec). Field names
// and tags must match exactly; TestFoldProjectionParity proves the
// round-trip through the real journal.
type foldPayload struct {
	TaskID  string           `json:"taskId"`
	Type    journal.FactType `json:"type"`
	Owner   string           `json:"owner,omitempty"`
	Attempt int              `json:"attempt,omitempty"`
	Error   string           `json:"error,omitempty"`
	Detail  json.RawMessage  `json:"detail,omitempty"`
}

// FoldProjection feeds platform-delivered fact events through the same
// fold the hand pump uses (Model.apply) — one fold, two drivers.
type FoldProjection struct {
	m *Model
}

// NewFoldProjection wraps the model's fold as a platform projection.
func NewFoldProjection(m *Model) FoldProjection {
	return FoldProjection{m: m}
}

// Name is the projection identity and the watermarks consumer.
func (p FoldProjection) Name() string { return CursorConsumer }

// EventTypes delivers every fact family, matching the hand pump's
// deliver-everything behavior exactly: the fold's default arm skips the
// no-ledger families, and a future family must reach Handle without an
// edit here.
func (p FoldProjection) EventTypes() []event.Type {
	return []event.Type{
		event.Type(journal.Enqueued),
		event.Type(journal.Claimed),
		event.Type(journal.Heartbeat),
		event.Type(journal.Completed),
		event.Type(journal.Failed),
		event.Type(journal.DeadLettered),
		event.Type(journal.Cancelled),
		event.Type(journal.CancelRequested),
		event.Type(journal.Released),
		event.Type(journal.Requeued),
		event.Type(journal.Orphaned),
		event.Type(journal.Reprioritized),
		event.Type(journal.SessionOpened),
		event.Type(journal.SessionClosed),
		event.Type(journal.QuestionAsked),
		event.Type(journal.QuestionAnswered),
	}
}

// Handle reconstructs the fact from the platform event and folds it. The
// payload was stamped by the cqrs journal's factEvent (event.New with the
// JSON codec), so DecodePayloadAuto is the matching decode path.
func (p FoldProjection) Handle(ctx context.Context, evt event.Event) error {
	payload, err := event.DecodePayloadAuto[foldPayload](evt)
	if err != nil {
		return fmt.Errorf("readmodel: decode fact payload seq event %s: %w", evt.ID(), err)
	}

	seq, ok := eventIDToSeq(evt.ID())
	if !ok {
		return fmt.Errorf("readmodel: event %s is not sequence-derived", evt.ID())
	}

	return p.m.apply(ctx, journal.Fact{
		Seq:     seq,
		TaskID:  payload.TaskID,
		Type:    payload.Type,
		Owner:   payload.Owner,
		Attempt: payload.Attempt,
		Error:   payload.Error,
		Detail:  jsontext.Value(payload.Detail),
		Time:    evt.OccurredAt(),
	})
}

// ProjectionHostOptions are the platform knobs the composition root may
// tune; zero values take the projectionhost defaults except where the
// queue's operating posture demands one (checkpoint batching on).
type ProjectionHostOptions struct {
	// CheckpointEvery batches live-phase checkpoint saves; 0 keeps the
	// platform default.
	CheckpointEvery int
	// BatchSize caps the journal read per drain loop; 0 keeps the
	// platform default.
	BatchSize int
	// MaxRestarts bounds the crash-restart budget per worker; 0 keeps
	// the platform default (5).
	MaxRestarts int
	// DeadLetterStore captures poison facts after Threshold failures;
	// nil disables the DLQ.
	DeadLetterStore projectionhost.DeadLetterStore
	// DeadLetterThreshold is the failure count that routes a fact to the
	// DLQ; 0 keeps the platform default when a store is configured.
	DeadLetterThreshold int
}

// NewProjectionHost builds the managed host over the fact journal with
// the model's fold registered as its projection. Start/Stop stay with
// the caller (the composition root owns the lifetime); the journal is
// the queue store itself, read-only (Facts).
func NewProjectionHost(src queue.Store, m *Model, opts ProjectionHostOptions) (*projectionhost.Host, error) {
	if src == nil {
		return nil, ErrNoSource
	}

	hostOpts := []projectionhost.HostOption{}

	if opts.CheckpointEvery > 0 {
		hostOpts = append(hostOpts, projectionhost.WithCheckpointEvery(opts.CheckpointEvery))
	}

	if opts.BatchSize > 0 {
		hostOpts = append(hostOpts, projectionhost.WithBatchSize(opts.BatchSize))
	}

	if opts.MaxRestarts > 0 {
		hostOpts = append(hostOpts, projectionhost.WithMaxRestarts(opts.MaxRestarts))
	}

	if opts.DeadLetterStore != nil {
		threshold := opts.DeadLetterThreshold
		if threshold <= 0 {
			threshold = 1
		}

		hostOpts = append(hostOpts, projectionhost.WithDeadLetterStore(opts.DeadLetterStore, threshold))
	}

	host, err := projectionhost.New(cqrs.NewFactJournal(src), watermarkCheckpoints{src: src}, hostOpts...)
	if err != nil {
		return nil, fmt.Errorf("readmodel: build projection host: %w", err)
	}

	if err := host.Register(NewFoldProjection(m)); err != nil {
		return nil, fmt.Errorf("readmodel: register fold projection: %w", err)
	}

	return host, nil
}

// Lag reports the host's aggregate fold lag — the doctor/health surface
// (M09) reads it through the host the caller owns.
func Lag(h *projectionhost.Host) time.Duration {
	return h.LagDuration()
}

// compile-time shape pins: the queue store is a fact source, and the
// fold satisfies the platform projection contract.
var (
	_ cqrs.FactSource       = queue.Store(nil)
	_ projection.Projection = FoldProjection{}
)
