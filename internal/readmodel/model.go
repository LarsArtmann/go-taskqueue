package readmodel

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	sqliteengine "github.com/larsartmann/go-cqrs-lite/metaengine/sqliteengine/v4"
	metaengine "github.com/larsartmann/go-cqrs-lite/metaengine/v4"
	"github.com/larsartmann/go-cqrs-lite/record/v4"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
)

// Defaults for the projection pump and the SSE surface.
const (
	// DefaultPoll is the journal tail interval (webui parity).
	DefaultPoll = 500 * time.Millisecond
	// DefaultBatch is the per-poll fact batch cap (webui tailBatchLimit
	// parity): the pump only needs to converge, so a longer burst
	// finishes over the next ticks.
	DefaultBatch = 1000
	// DefaultReplayCapacity bounds the SSE replay journal (Last-Event-ID
	// reconnection window).
	DefaultReplayCapacity = 1024
	// SSEHeartbeat is the keepalive cadence for the events stream.
	SSEHeartbeat = 15 * time.Second
	// SSETimeout caps one stream's lifetime so server shutdowns and
	// stuck clients reclaim their goroutines.
	SSETimeout = 30 * time.Minute
	// CursorConsumer is the watermarks-table key the durable cursor
	// checkpoints under (queue.Store.SaveWatermark): the projection
	// resumes from it instead of replaying the whole journal on every
	// open. One key per queue db — the projection file is PathFor-derived,
	// so one store has exactly one durable projection.
	CursorConsumer = "readmodel"
)

// ProjectionHomeCallerPragmas is the ONE pragma literal for the
// projection-home file, passed on top of the sqliteengine's own
// production defaults (NewSQLiteEngineFromDSNWith always prepends
// journal_mode=WAL + busy_timeout=5000 and pins MaxOpenConns(1)):
// synchronous=NORMAL trades tail-replay for checkpoint-only fsyncs, the
// same relaxed-fsync policy the queue store runs; cache_size rides the
// fold's page locality. The composition root references this list in
// its DeploymentConfig so BOTH connections to the projection home (the
// tq-owned model engine and system's declared engine) run the identical
// union — the single pragma source of the single-opener design
// (internal/composition/single_opener.md).
var ProjectionHomeCallerPragmas = []string{
	"synchronous=NORMAL",
	"cache_size=-32768",
}

// ErrNoSource reports an Open call without a journal source: the model is
// a projection, and without a journal to fold there is nothing to serve.
var ErrNoSource = errors.New("readmodel: nil journal source")

// PathFor derives the projection database path beside a queue database:
// one shared projection file per queue db (the sqliteengine opens it WAL
// + busy_timeout, so serve/api/stats processes can share it).
func PathFor(dbPath string) string {
	return dbPath + ".readmodel.db"
}

// Model is the metaengine-backed read model over one queue's fact journal:
// the tasks collection (planned table) plus the Watcher/ServeSSE live
// surface. Create with Open, pump with Run (or CatchUp for one pass), read
// with Tasks/StatusCounts, stream with EventsHandler.
type Model struct {
	eng   metaengine.Engine
	store *metaengine.Store
	src   queue.Store
	rows  RowSource
	poll  time.Duration
	batch int

	// cursor is the applied journal watermark: the seq of the last fact
	// folded into the collections. Atomic so live consumers can read the
	// pump's progress (JournalCursor) while it advances.
	cursor atomic.Int64
	// durable reports whether the cursor checkpoints to the queue db's
	// watermarks table after every applied batch (WithDurableCursor): a
	// restarted model resumes from the checkpoint instead of replaying
	// the journal from zero. Off (default) the cursor is in-process only
	// — the safe shape for tests and short-lived models over a store
	// whose watermark other models may own.
	durable bool
	watcher *metaengine.Watcher[TaskRow]
}

// Option configures the Model.
type Option func(*Model)

// WithPoll overrides the journal tail interval.
func WithPoll(d time.Duration) Option {
	return func(m *Model) { m.poll = d }
}

// WithBatch overrides the per-poll fact batch cap.
func WithBatch(n int) Option {
	return func(m *Model) { m.batch = n }
}

// WithRowSource overrides the thin-enqueue side channel (nil disables it).
// The default adapts the journal source's Get.
func WithRowSource(rs RowSource) Option {
	return func(m *Model) { m.rows = rs }
}

// WithDurableCursor checkpoints the projection cursor into the queue
// db's watermarks table after every applied batch and resumes from it on
// open: a restarted model folds only the facts appended since its last
// checkpoint instead of replaying the whole journal (the 9.5k-fact
// restart replay the adoption review measured). An empty projection
// under a nonzero checkpoint replays from zero once, so the documented
// "delete the file to force a replay" escape hatch keeps working.
// Checkpoint writes are monotonic upserts; a failed write is logged and
// retried on the next batch — a missed checkpoint only costs replay on
// the next restart, never a skipped fact.
func WithDurableCursor() Option {
	return func(m *Model) { m.durable = true }
}

// WithEngine adopts a caller-built engine instead of constructing one:
// the composition root opens the projection-home engine ONCE and hands
// it over (the single tq-owned constructor call per serve run). From
// adoption on the model owns the engine exactly as for a self-opened
// one — the FromDSN constructors mark their engine as the DB owner, so
// Close tears the connection down with the model.

// Open creates the Model over its own sqlite database file (the projection
// is disposable; delete the file to force a full journal replay on next
// open). src is the journal source — the same queue.Store the dashboards
// read; it is used read-only (Facts, HeadSeq, Get). ctx covers the
// durable-cursor load (watermark read + replay guard), not the pump
// (Run carries its own).
func Open(ctx context.Context, path string, src queue.Store, opts ...Option) (*Model, error) {
	if src == nil {
		return nil, ErrNoSource
	}

	m := &Model{
		src:  src,
		rows: StoreRows{Store: src},
		poll: DefaultPoll,
		batch: DefaultBatch,
	}

	for _, opt := range opts {
		opt(m)
	}

	// The projection db is DISPOSABLE by contract (delete the file to
	// force a full replay; the durable cursor lives in the queue's
	// watermarks table, not here), so it runs the shared relaxed-fsync
	// pragma union (ProjectionHomeCallerPragmas on top of the engine's
	// WAL + busy_timeout defaults) — one literal, referenced by the
	// composition root for its declared engine too.
	if m.eng == nil {
		eng, err := sqliteengine.NewSQLiteEngineFromDSN(path, ProjectionHomeCallerPragmas...)
		if err != nil {
			return nil, fmt.Errorf("readmodel: open projection db: %w", err)
		}

		m.eng = eng
	}

	store, err := metaengine.Plan([]metaengine.Engine{m.eng}, tasksQuery)
	if err != nil {
		_ = m.eng.Close()

		return nil, fmt.Errorf("readmodel: plan collections: %w", err)
	}

	m.store = store

	if m.durable {
		if err := m.loadCursor(ctx); err != nil {
			_ = m.store.Close()

			return nil, fmt.Errorf("readmodel: load cursor: %w", err)
		}
	}

	m.watcher = metaengine.NewWatcher[TaskRow](m.store, tasksCollection)
	m.watcher.WithReplay(DefaultReplayCapacity)

	return m, nil
}

// loadCursor resumes from the persisted checkpoint (WithDurableCursor).
// A nonzero checkpoint over an EMPTY projection means the projection
// file was deleted (or predates any task fact): replay from zero once so
// the delete-to-replay escape hatch keeps working — the folds are
// idempotent upserts, so the replay converges and the next checkpoint
// re-seals the cursor.
func (m *Model) loadCursor(ctx context.Context) error {
	seq, exists, err := m.src.Watermark(ctx, CursorConsumer)
	if err != nil {
		return err
	}

	if !exists || seq == 0 {
		return nil
	}

	rows, err := metaengine.NewReader[TaskRow](m.store, tasksCollection).Count(ctx)
	if err != nil {
		return fmt.Errorf("readmodel: probe projection rows: %w", err)
	}

	if rows == 0 {
		slog.Info("readmodel: empty projection under a nonzero cursor — replaying from zero", "checkpoint", seq)

		return nil
	}

	m.cursor.Store(seq)

	return nil
}

// Close releases the watcher and the projection database. The journal
// source is owned by the caller.
func (m *Model) Close() error {
	m.watcher.Close()

	if err := m.store.Close(); err != nil {
		return fmt.Errorf("readmodel: close projection: %w", err)
	}

	return nil
}

// Run pumps the journal until ctx is cancelled: catch up to the head, then
// tail for new facts. Without WithDurableCursor the cursor is in-process —
// a restarted model replays the journal from the beginning, which the
// folds converge on (every fold is an upsert keyed by task id); with it,
// the model resumes from the persisted checkpoint instead.
func (m *Model) Run(ctx context.Context) error {
	if err := m.CatchUp(ctx); err != nil {
		return err
	}

	ticker := time.NewTicker(m.poll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := m.catchUpOnce(ctx); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}

				slog.Error("readmodel: tail poll failed", "err", err)
			}
		}
	}
}

// CatchUp applies every journal fact up to the current head once.
func (m *Model) CatchUp(ctx context.Context) error {
	for {
		applied, err := m.catchUpOnce(ctx)
		if err != nil {
			return err
		}

		if applied < m.batch {
			return nil
		}
	}
}

// catchUpOnce applies one batch of new facts and reports how many it saw.
func (m *Model) catchUpOnce(ctx context.Context) (int, error) {
	after := m.cursor.Load()

	facts, err := m.src.Facts(ctx, after, m.batch)
	if err != nil {
		return 0, fmt.Errorf("readmodel: read facts after %d: %w", after, err)
	}

	for _, f := range facts {
		if err := m.apply(ctx, f); err != nil {
			return 0, err
		}
	}

	if len(facts) > 0 {
		last := facts[len(facts)-1].Seq
		m.cursor.Store(last)

		// Checkpoint after the batch's last applied fact. Unlike the
		// sweeper watermarks (where a missed fact is a missed dispatch),
		// a failed checkpoint here only costs replay on the next restart:
		// log and let the next batch retry the monotonic upsert.
		if m.durable {
			if err := m.src.SaveWatermark(ctx, CursorConsumer, last); err != nil {
				slog.Warn("readmodel: checkpoint cursor failed", "seq", last, "err", err)
			}
		}
	}

	return len(facts), nil
}

// JournalCursor reports the applied journal watermark: the seq of the
// last fact folded into the collections (0 before the first apply). Live
// consumers use it as the change-notification sequence, so a notification
// carries the same journal watermark the hand tailer used to report.
func (m *Model) JournalCursor() int64 {
	return m.cursor.Load()
}

// apply maps one fact to its fold input and feeds it through the store.
// Facts without a fold are skipped — the cursor still advances past them.
func (m *Model) apply(ctx context.Context, fact journal.Fact) error {
	evt, ok, err := eventFor(ctx, fact, m.rows)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	rec := record.Record{Type: string(fact.Type)}
	if err := m.store.ApplyRecord(ctx, rec, evt); err != nil {
		return fmt.Errorf("readmodel: apply %s seq %d: %w", fact.Type, fact.Seq, err)
	}

	return nil
}

// Tasks reads the ledger under the filter, newest creation first — the
// planned-table pushdown scan. Filters ride the documented TypedReader
// surface (conditional WithFilter options): the query-input dispatch
// (Store.ExecuteCtx) cannot express optional filters — a nil *string input
// field binds as a typed-nil interface and lands in SQL as `= NULL`
// (metaengine v4.14.0, extractValueByName), so every read here goes
// through the reader.
func (m *Model) Tasks(ctx context.Context, filter TaskFilter) ([]TaskRow, error) {
	opts := []metaengine.ScanOption{
		metaengine.WithSort("created_at", true),
		metaengine.WithLimit(0), // unbounded: the caller paginates
	}

	if filter.Status != nil {
		opts = append(opts, metaengine.WithFilter("status", metaengine.FilterEq, *filter.Status))
	}

	if filter.Project != nil {
		opts = append(opts, metaengine.WithFilter("project", metaengine.FilterEq, *filter.Project))
	}

	rows, err := metaengine.NewReader[TaskRow](m.store, tasksCollection).Scan(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("readmodel: task scan: %w", err)
	}

	return rows, nil
}

// Watch subscribes to live ledger changes: every fold update arrives as
// the folded TaskRow (buffered, drop-oldest on a slow consumer). The
// subscription ends with ctx. This is the Watcher half of the S3 live
// fragments; EventsHandler is its SSE transport.
func (m *Model) Watch(ctx context.Context) <-chan TaskRow {
	return m.watcher.Watch(ctx, nil)
}

// WatchSeq is Watch with the projection write sequence attached. The
// dashboard's notification pump consumes this: each delivery replaces one
// journal-tailer poll cycle — the change signal, not the payload.
func (m *Model) WatchSeq(ctx context.Context) <-chan metaengine.SeqValue[TaskRow] {
	return m.watcher.WatchWithSeq(ctx, nil)
}

// EventsHandler serves the live ledger as Server-Sent Events: every fold
// update streams as one JSON TaskRow, with `id: <seq>` for Last-Event-ID
// reconnection (metaengine's replay journal). This is the Watcher/ServeSSE
// mechanism the S3 flip installs behind the dashboard fragments.
func (m *Model) EventsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := metaengine.ServeSSE(w, r, m.watcher,
			metaengine.WithSSEHeartbeat(SSEHeartbeat),
			metaengine.WithSSETimeout(SSETimeout),
		)
		if err != nil && r.Context().Err() == nil {
			slog.Error("readmodel: sse stream failed", "err", err)
		}
	})
}
