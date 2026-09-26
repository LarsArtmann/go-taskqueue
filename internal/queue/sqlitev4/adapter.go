// Package sqlitev4 is the ADR-0019 S1 spike: a tq queue.Store implemented
// over go-cqrs-lite queue/sqlite/v4 (the upstream engine transcribed from
// this repo's contract) plus tq-side companion surfaces over the SAME
// database. The companion surfaces themselves live ONCE in
// internal/queue/companion (dedup ruling 2026-09-26: one home, no
// cross-backend mirrors); this adapter keeps the engine-backed lifecycle,
// the single-writer handle, and one-line delegations.
//
// Division of labor:
//
//   - Engine-backed (upstream queue/sqlite/v4, the S1 target): Enqueue,
//     Complete, Fail, FailPermanent, Heartbeat, Cancel, CancelRunning,
//     CancelRequested, CancelOwned, MarkOrphaned, RescueDead, DismissDead,
//     UpdatePendingPriority.
//   - Companion-owned (one home in internal/queue/companion): ClaimDue
//     (upstream has no project exclusivity — the per-repo agent-pool
//     guarantee, D24), Requeue (tq's evidence carries resume_closeout),
//     all reads, RecordAnswer, AppendFact (non-task facts), watermarks,
//     priority scores.
//
// Known divergences are catalogued in
// docs/status/2026-09-23_16-28_adr0019-s1-sqlitev4-conformance-divergence-report.md
// (D1 finalizes, D2 enqueued detail, D3 heartbeat facts, D4 fact archive,
// D5 payload storage class; D6 exclusivity/resume-closeout are
// adapter-implemented, not divergences).
package sqlitev4

import (
	"context"
	"database/sql"
	"encoding/json/jsontext"
	"fmt"
	"time"

	usqlite "github.com/larsartmann/go-cqrs-lite/queue/sqlite/v4"
	uqueue "github.com/larsartmann/go-cqrs-lite/queue/v4"
	utask "github.com/larsartmann/go-cqrs-lite/queue/v4/task"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/companion"
	"github.com/larsartmann/go-taskqueue/internal/task"
	_ "modernc.org/sqlite" // pure-Go SQLite driver (CGo-free)
)

// Store is the S1 spike store: tq's queue.Store contract over the upstream
// engine + companion surfaces on the same SQLite file.
type Store struct {
	engine *usqlite.Store[[]byte]
	db     *sql.DB

	// cr is the pre-dialed companion runner over db.
	cr companion.Runner

	projectExclusive bool
}

// Store implements the tq queue contract at compile time.
//
// art-dupl:accept interface-assert boilerplate: every Store backend must
// restate this check; there is no logic to extract.
var _ queue.Store = (*Store)(nil)

// StoreOption configures optional Store behavior.
//
// art-dupl:accept public-surface alias: the option type must be restated
// in each backend's package so callers import the backend, not companion.
type StoreOption = companion.StoreOption

// WithProjectExclusivity mirrors internal/queue/sqlite's option: ClaimDue
// will not claim a task whose project already has another running task.
// The upstream engine has no such predicate, so the adapter owns ClaimDue
// entirely (divergence noted in the S1 report).
var WithProjectExclusivity = companion.WithProjectExclusivity

// Open opens (creating if needed) the queue database at path.
func Open(path string, opts ...StoreOption) (*Store, error) {
	projectExclusive := companion.ApplyProjectExclusivity(opts)

	engine, err := usqlite.Open[[]byte](path, usqlite.WithCodec(companion.IdentityCodec()))
	if err != nil {
		return nil, fmt.Errorf("sqlitev4: open engine: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		_ = engine.Close()

		return nil, fmt.Errorf("sqlitev4: open companion db: %w", err)
	}

	db.SetMaxOpenConns(1)

	store := &Store{
		engine:           engine,
		db:               db,
		cr:               companion.For(companion.SQLite, db),
		projectExclusive: projectExclusive,
	}

	if err := companion.Migrate(context.Background(), store.cr); err != nil {
		_ = db.Close()
		_ = engine.Close()

		return nil, err
	}

	return store, nil
}

// Close releases the engine and the companion handle.
//
// art-dupl:accept two-handle teardown: each backend owns its engine and
// companion db handles; companion cannot close them.
func (s *Store) Close() error {
	errEngine := s.engine.Close()
	errDB := s.db.Close()
	if errEngine != nil {
		return errEngine
	}

	return errDB
}

// Enqueue persists a new task via the upstream engine and re-reads it so
// the returned record carries every tq field (DedupKey included — the
// upstream task struct does not surface it).
func (s *Store) Enqueue(ctx context.Context, n task.New) (task.Task, error) {
	if n.Type == "" {
		return task.Task{}, queue.ErrEmptyType
	}

	deps := make([]utask.ID, 0, len(n.Deps))
	for _, d := range n.Deps {
		deps = append(deps, utask.ID(d.String()))
	}

	notBefore := n.NotBefore
	if notBefore.IsZero() {
		notBefore = time.UnixMilli(0)
	}

	created, err := s.engine.Enqueue(ctx, utask.New[[]byte]{
		Project:     n.Project,
		Type:        n.Type,
		Payload:     []byte(n.Payload),
		Deps:        deps,
		Priority:    n.Priority,
		MaxAttempts: n.MaxAttempts,
		NotBefore:   notBefore,
		DedupKey:    n.DedupKey,
	})
	if err != nil {
		return task.Task{}, companion.MapErr(err)
	}

	return s.Get(ctx, task.ID(created.ID.String()))
}

// tokenFor enforces tq's claim-token gate for a finalize (the gate itself
// lives in companion; the adapter supplies its dialed runner).
func (s *Store) tokenFor(ctx context.Context, id task.ID, claim queue.Claim, requireLive bool) (string, error) {
	return companion.TokenFor(ctx, s.cr, id, claim, requireLive)
}

// Complete marks a Running task Completed (owner gate, engine finalize).
// tq parity (divergence D1): tq's sqlite clears last_error in the same
// UPDATE that completes the task; the upstream engine leaves a failed
// attempt's error on the completed row, so the adapter clears it after
// the finalize. A completed task can never Fail again, so the follow-up
// UPDATE cannot race a new error onto the row.
func (s *Store) Complete(ctx context.Context, id task.ID, claim queue.Claim, result jsontext.Value) error {
	token, err := s.tokenFor(ctx, id, claim, true)
	if err != nil {
		return err
	}

	if err := s.engine.Complete(ctx, utask.ID(id.String()), token, []byte(result)); err != nil {
		return companion.MapErr(err)
	}

	_, err = s.cr.ExecContext(ctx,
		`UPDATE tasks SET last_error = '' WHERE id = ? AND status = 'completed'`,
		id.String())

	return err
}

// Fail records a failed attempt: retry with backoff or dead-letter.
func (s *Store) Fail(
	ctx context.Context,
	id task.ID,
	claim queue.Claim,
	errText string,
	backoff time.Duration,
	evidence jsontext.Value,
) error {
	token, err := s.tokenFor(ctx, id, claim, true)
	if err != nil {
		return err
	}

	return companion.MapErr(s.engine.Fail(ctx, utask.ID(id.String()), token, errText, backoff, []byte(evidence)))
}

// FailPermanent dead-letters immediately (permanent error class).
func (s *Store) FailPermanent(
	ctx context.Context,
	id task.ID,
	claim queue.Claim,
	errText string,
	evidence jsontext.Value,
) error {
	token, err := s.tokenFor(ctx, id, claim, true)
	if err != nil {
		return err
	}

	return companion.MapErr(s.engine.FailPermanent(ctx, utask.ID(id.String()), token, errText, []byte(evidence)))
}

// Heartbeat extends the lease of a Running task held by owner.
func (s *Store) Heartbeat(ctx context.Context, id task.ID, claim queue.Claim, extend time.Duration) error {
	token, err := s.tokenFor(ctx, id, claim, true)
	if err != nil {
		return err
	}

	return companion.MapErr(s.engine.Heartbeat(ctx, utask.ID(id.String()), token, extend))
}

// Cancel withdraws a Pending task (engine; error vocabulary mapped).
func (s *Store) Cancel(ctx context.Context, id task.ID, reason string) error {
	return companion.MapErr(s.engine.Cancel(ctx, utask.ID(id.String()), reason))
}

// CancelRunning records a cooperative cancel request for a Running task.
func (s *Store) CancelRunning(ctx context.Context, id task.ID, reason string) error {
	return companion.MapErr(s.engine.CancelRunning(ctx, utask.ID(id.String()), reason))
}

// CancelRequested reports whether a cooperative cancel request is pending.
func (s *Store) CancelRequested(ctx context.Context, id task.ID) (bool, error) {
	requested, err := s.engine.CancelRequested(ctx, utask.ID(id.String()))

	return requested, companion.MapErr(err)
}

// CancelOwned finalizes a cooperative cancel. tq's gate is status+owner
// (no live-lease requirement — a worker may legitimately finish the stop
// just after the lease lapsed but before a reclaim), so requireLive=false.
func (s *Store) CancelOwned(ctx context.Context, id task.ID, claim queue.Claim) error {
	token, err := s.tokenFor(ctx, id, claim, false)
	if err != nil {
		return err
	}

	return companion.MapErr(s.engine.CancelOwned(ctx, utask.ID(id.String()), token))
}

// MarkOrphaned appends task.orphaned facts for expired-lease Running tasks.
func (s *Store) MarkOrphaned(ctx context.Context, cutoff time.Time) (int, error) {
	n, err := s.engine.MarkOrphaned(ctx, cutoff)

	return n, companion.MapErr(err)
}

// RescueDead re-queues a Dead task with a fresh attempt budget.
func (s *Store) RescueDead(ctx context.Context, id task.ID, maxAttempts int) error {
	return companion.MapErr(s.engine.RescueDead(ctx, utask.ID(id.String()), maxAttempts))
}

// DismissDead cancels a Dead task with a recorded reason.
func (s *Store) DismissDead(ctx context.Context, id task.ID, reason, by string) error {
	return companion.MapErr(s.engine.DismissDead(ctx, utask.ID(id.String()), reason, by))
}

// UpdatePendingPriority changes a PENDING task's priority (in-tx fact).
func (s *Store) UpdatePendingPriority(ctx context.Context, id task.ID, newPriority int, source, reason string) error {
	return companion.MapErr(s.engine.UpdatePendingPriority(ctx, utask.ID(id.String()), newPriority, source, reason))
}

// ClaimDue atomically claims at most one due task for owner, with tq's
// project-exclusivity predicate (companion-owned; S1 divergence).
func (s *Store) ClaimDue(ctx context.Context, owner string, lease time.Duration) (task.Task, queue.Claim, error) {
	return companion.ClaimDue(ctx, companion.SQLite, s.db, s.projectExclusive, owner, lease)
}

// Requeue returns a claimed task to Pending without counting an attempt
// (companion-owned; tq's evidence carries the resume_closeout flag).
func (s *Store) Requeue(
	ctx context.Context,
	id task.ID,
	claim queue.Claim,
	errText string,
	delay time.Duration,
	resumeCloseout bool,
) error {
	return companion.Requeue(ctx, companion.SQLite, s.db, id, claim, errText, delay, resumeCloseout)
}

// AppendFact records a NON-task journal fact (session.opened /
// session.closed). Task facts are never written through it.
//
// S2 (ADR-0019): the append rides the engine's FactTx sink — the same-tx
// fact-append capability is engine-enforced, not the adapter's hand
// INSERT. The tq journal.Fact maps onto facts.Fact verbatim (open string
// FactType + raw detail bytes).
func (s *Store) AppendFact(ctx context.Context, f journal.Fact) error {
	return s.engine.WithFacts(ctx, func(sink uqueue.FactSink) error {
		return sink.Append(ctx, companion.UpstreamFact(f))
	})
}

// RecordAnswer records an owner's decision for a parked task's question
// (companion-owned; same-tx payload injection + NotBefore clear + fact).
func (s *Store) RecordAnswer(ctx context.Context, id task.ID, ans queue.AnswerRecord) error {
	return companion.RecordAnswer(ctx, companion.SQLite, s.db, id, ans)
}

// Get returns the current task record.
//
// art-dupl:accept Store-interface delegator: the method must exist on each
// backend; its body is already the companion call.
func (s *Store) Get(ctx context.Context, id task.ID) (task.Task, error) {
	return companion.Get(ctx, s.cr, id)
}

// List returns tasks matching the filter.
func (s *Store) List(ctx context.Context, f queue.Filter) ([]task.Task, error) {
	return companion.List(ctx, s.cr, f)
}

// CountTasks counts the tasks matching the filter (COUNT(*) pushdown).
func (s *Store) CountTasks(ctx context.Context, f queue.Filter) (int, error) {
	return companion.CountTasks(ctx, s.cr, f)
}

// Facts returns journal facts with Seq > after, ascending, bounded to the
// most recent limit when > 0.
func (s *Store) Facts(ctx context.Context, after int64, limit int) ([]journal.Fact, error) {
	return companion.Facts(ctx, s.cr, after, limit)
}

// LastFacts returns the most recent limit facts in ascending Seq order.
func (s *Store) LastFacts(ctx context.Context, limit int) ([]journal.Fact, error) {
	return companion.LastFacts(ctx, s.cr, limit)
}

// HeadSeq returns the current highest fact Seq (0 when the journal is empty).
func (s *Store) HeadSeq(ctx context.Context) (int64, error) {
	return companion.HeadSeq(ctx, s.cr)
}

// FactsForTask returns one task's facts in Seq order, bounded to the most
// recent limit when > 0.
func (s *Store) FactsForTask(ctx context.Context, id string, limit int) ([]journal.Fact, error) {
	return companion.FactsForTask(ctx, s.cr, id, limit)
}

// CountFacts counts facts of one type recorded at or after since.
func (s *Store) CountFacts(ctx context.Context, ftype journal.FactType, since time.Time) (int64, error) {
	return companion.CountFacts(ctx, s.cr, ftype, since)
}

// FactsSince returns facts of one type recorded at or after since.
func (s *Store) FactsSince(
	ctx context.Context,
	ftype journal.FactType,
	since time.Time,
	limit int,
) ([]journal.Fact, error) {
	return companion.FactsSince(ctx, s.cr, ftype, since, limit)
}

// Watermark returns the persisted read cursor for a journal consumer.
func (s *Store) Watermark(ctx context.Context, consumer string) (int64, bool, error) {
	return companion.Watermark(ctx, s.cr, consumer)
}

// SaveWatermark checkpoints a consumer cursor as a monotonic upsert.
func (s *Store) SaveWatermark(ctx context.Context, consumer string, seq int64) error {
	return companion.SaveWatermark(ctx, s.cr, consumer, seq)
}

// ListWatermarks returns every consumer cursor, by consumer name.
func (s *Store) ListWatermarks(ctx context.Context) ([]queue.WatermarkEntry, error) {
	return companion.ListWatermarks(ctx, s.cr)
}

// SetWatermark overwrites a consumer cursor unconditionally (ops rescue).
func (s *Store) SetWatermark(ctx context.Context, consumer string, seq int64) error {
	return companion.SetWatermark(ctx, s.cr, consumer, seq)
}

// SavePriorityScore upserts one cached item score (ADR-0015 score cache).
func (s *Store) SavePriorityScore(ctx context.Context, score queue.PriorityScore) error {
	return companion.SavePriorityScore(ctx, s.cr, score)
}

// PriorityScore returns the cached verdict for an item key, if any.
func (s *Store) PriorityScore(ctx context.Context, itemKey string) (queue.PriorityScore, bool, error) {
	return companion.PriorityScore(ctx, s.cr, itemKey)
}

// PriorityScores returns every cached verdict, ordered by item key.
func (s *Store) PriorityScores(ctx context.Context) ([]queue.PriorityScore, error) {
	return companion.PriorityScores(ctx, s.cr)
}

// DeletePriorityScores removes the cached verdicts for the given item keys.
func (s *Store) DeletePriorityScores(ctx context.Context, itemKeys []string) (int64, error) {
	return companion.DeletePriorityScores(ctx, s.cr, itemKeys)
}

// StatusCounts counts tasks per status in one GROUP BY.
func (s *Store) StatusCounts(ctx context.Context) (map[task.Status]int, error) {
	return companion.StatusCounts(ctx, s.cr)
}

// ProjectCounts counts tasks per project per status in one GROUP BY.
func (s *Store) ProjectCounts(ctx context.Context) (map[string]map[task.Status]int, error) {
	return companion.ProjectCounts(ctx, s.cr)
}
