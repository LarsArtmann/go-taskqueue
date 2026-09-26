package cqrsqlite

import (
	"context"
	"errors"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/companion"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// The tq same-DB extension (ADR-0019 S1): everything below reads and
// writes the engine's tables with tq's exact SQL and adds ONE companion
// table (priority_scores) — its DDL and migration live in
// companion.CompanionSchema/companion.Migrate. Facts appended here are
// schema-compatible with the engine's facts table, so they read back
// through the engine's own Facts reads.
//
// The implementations themselves live ONCE in internal/queue/companion
// (dedup ruling 2026-09-26: one home, no cross-backend mirrors) — this
// file keeps the fact-append seam and one-line delegations behind the
// SQLite dialect.

// AppendFact records a NON-task journal fact (session.opened /
// session.closed). Task facts are never written through it — every task
// mutation appends its fact inside its own operation's transaction.
func (s *Store) AppendFact(ctx context.Context, f journal.Fact) error {
	return companion.AppendFactTx(ctx, companion.SQLite, s.db, f)
}

// RecordAnswer records an owner's decision for a parked task's question
// and — when the task is PENDING — injects the answer into the task's
// JSON-object payload under the "answered" key and clears NotBefore, all
// in the SAME transaction. Idempotent per question ref.
func (s *Store) RecordAnswer(ctx context.Context, id task.ID, ans queue.AnswerRecord) error {
	return companion.RecordAnswer(ctx, companion.SQLite, s.db, id, ans)
}

// List returns tasks matching the filter.
func (s *Store) List(ctx context.Context, f queue.Filter) ([]task.Task, error) {
	return companion.List(ctx, s.cr, f)
}

// CountTasks counts the tasks matching the filter (COUNT(*) pushdown).
func (s *Store) CountTasks(ctx context.Context, f queue.Filter) (int, error) {
	return companion.CountTasks(ctx, s.cr, f)
}

// LastFacts returns the most recent limit facts in ascending Seq order.
func (s *Store) LastFacts(ctx context.Context, limit int) ([]journal.Fact, error) {
	return companion.LastFacts(ctx, s.cr, limit)
}

// CountFacts counts facts of one type recorded at or after since.
func (s *Store) CountFacts(ctx context.Context, ftype journal.FactType, since time.Time) (int64, error) {
	return companion.CountFacts(ctx, s.cr, ftype, since)
}

// FactsSince returns facts of one type recorded at or after since, in Seq
// order.
func (s *Store) FactsSince(
	ctx context.Context,
	ftype journal.FactType,
	since time.Time,
	limit int,
) ([]journal.Fact, error) {
	return companion.FactsSince(ctx, s.cr, ftype, since, limit)
}

// ProjectCounts counts tasks per project per status in one GROUP BY.
func (s *Store) ProjectCounts(ctx context.Context) (map[string]map[task.Status]int, error) {
	return companion.ProjectCounts(ctx, s.cr)
}

// ListWatermarks returns every consumer cursor, by consumer name.
func (s *Store) ListWatermarks(ctx context.Context) ([]queue.WatermarkEntry, error) {
	return companion.ListWatermarks(ctx, s.cr)
}

// SetWatermark overwrites a consumer cursor unconditionally (ops rescue
// hatch); unlike SaveWatermark it MAY move the cursor backwards.
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

// DeletePriorityScores removes the cached verdicts for the given item
// keys and returns how many rows went away.
func (s *Store) DeletePriorityScores(ctx context.Context, itemKeys []string) (int64, error) {
	return companion.DeletePriorityScores(ctx, s.cr, itemKeys)
}

// mustJSON aliases the companion helper for the conformance suite's
// fact-detail literals.
var mustJSON = companion.MustJSON

// ArchiveFactsBefore is NOT implemented in the S1 spike: the fact archive
// (facts_archive / journal_meta) has no upstream counterpart.
func (s *Store) ArchiveFactsBefore(ctx context.Context, cutoff int64) (int64, error) {
	return 0, errors.New("cqrsqlite: fact archive not implemented in the S1 spike")
}

// ArchiveSummary is NOT implemented in the S1 spike: see ArchiveFactsBefore.
func (s *Store) ArchiveSummary(ctx context.Context) (ArchiveStats, error) {
	return ArchiveStats{}, errors.New("cqrsqlite: fact archive not implemented in the S1 spike")
}

// ArchiveStats aliases the companion archive-summary shape (conform
// suite surface; the tq sqlite store's summary shape).
type ArchiveStats = companion.ArchiveStats
