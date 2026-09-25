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
)

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
	cursor  atomic.Int64
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

// Open creates the Model over its own sqlite database file (the projection
// is disposable; delete the file to force a full journal replay on next
// open). src is the journal source — the same queue.Store the dashboards
// read; it is used read-only (Facts, HeadSeq, Get).
func Open(path string, src queue.Store, opts ...Option) (*Model, error) {
	if src == nil {
		return nil, ErrNoSource
	}

	eng, err := sqliteengine.NewSQLiteEngineFromDSN(path)
	if err != nil {
		return nil, fmt.Errorf("readmodel: open projection db: %w", err)
	}

	store, err := metaengine.Plan([]metaengine.Engine{eng}, tasksQuery)
	if err != nil {
		_ = eng.Close()

		return nil, fmt.Errorf("readmodel: plan collections: %w", err)
	}

	m := &Model{
		eng:   eng,
		store: store,
		src:   src,
		rows:  StoreRows{Store: src},
		poll:  DefaultPoll,
		batch: DefaultBatch,
	}

	for _, opt := range opts {
		opt(m)
	}

	m.watcher = metaengine.NewWatcher[TaskRow](m.store, tasksCollection)
	m.watcher.WithReplay(DefaultReplayCapacity)

	return m, nil
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
// tail for new facts. The cursor is in-process — a restarted model replays
// the journal from the beginning, which the folds converge on (every fold
// is an upsert keyed by task id).
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
		m.cursor.Store(facts[len(facts)-1].Seq)
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

// StatusCounts counts the ledger rows per status — the stats projection.
func (m *Model) StatusCounts(ctx context.Context) (map[string]int, error) {
	rows, err := m.Tasks(ctx, TaskFilter{})
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int, len(rows))
	for _, r := range rows {
		counts[r.Status]++
	}

	return counts, nil
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
