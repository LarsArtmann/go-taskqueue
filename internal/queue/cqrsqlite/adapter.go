// Package cqrsqlite is the ADR-0019 S1 spike: a thin driver implementing
// tq's queue.Store over the go-cqrs-lite queue/sqlite/v4 engine, plus the
// tq-specific surfaces the upstream contract lacks as a same-DB extension
// (companion tables / companion reads): RecordAnswer and the question
// facts, the priority_scores cache, CountFacts/FactsSince/LastFacts,
// ProjectCounts, the tq List filter surface (severity/sort orders), and
// the out-of-band AppendFact seam.
//
// KNOWN DIVERGENCES (spike report, docs/status/):
//   - Owner-string finalizes are emulated over upstream claim tokens via
//     an in-process ledger: the adapter records the token minted at
//     ClaimDue and checks the finalize's owner against it. Token fencing
//     is preserved within one process; a restarted process cannot
//     finalize a lease it did not claim (the owner string alone no longer
//     authorizes).
//   - Requeue drops the resumeCloseout flag: upstream task.requeued facts
//     carry no resume_closeout key.
//   - Project exclusivity (WithProjectExclusivity) has no upstream
//     equivalent and is not implemented.
//   - Fact-archive (facts_archive) and journal_meta have no upstream
//     counterpart and are not implemented.
//   - The task.enqueued fact detail carries {project, type} only (no
//     priority/dedup_key projection fields).
package cqrsqlite

import (
	"context"
	"database/sql"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"sync"
	"time"

	usqlite "github.com/larsartmann/go-cqrs-lite/queue/sqlite/v4"
	uqueue "github.com/larsartmann/go-cqrs-lite/queue/v4"
	ufacts "github.com/larsartmann/go-cqrs-lite/queue/v4/facts"
	utask "github.com/larsartmann/go-cqrs-lite/queue/v4/task"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/companion"
	"github.com/larsartmann/go-taskqueue/internal/task"
	_ "modernc.org/sqlite" // pure-Go SQLite driver (CGo-free)
)

// Store is the S1 spike store: tq's queue.Store over the go-cqrs-lite
// sqlite engine plus the tq same-DB extension.
type Store struct {
	eng *usqlite.Store[[]byte]
	db  *sql.DB

	// cr is the pre-dialed companion runner over db.
	cr companion.Runner

	mu     sync.Mutex
	claims map[task.ID]string
}

// Store implements the tq queue contract at compile time.
var _ queue.Store = (*Store)(nil)

// Open opens (creating if needed) the queue database at path and migrates
// the tq companion tables onto the engine's schema.
func Open(path string) (*Store, error) {
	eng, err := usqlite.Open[[]byte](path, usqlite.WithCodec[[]byte](rawCodec()))
	if err != nil {
		return nil, fmt.Errorf("cqrsqlite: open engine: %w", err)
	}

	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)",
		path,
	)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		_ = eng.Close()

		return nil, fmt.Errorf("cqrsqlite: open companion db: %w", err)
	}

	db.SetMaxOpenConns(1)

	s := &Store{eng: eng, db: db, cr: companion.For(companion.SQLite, db), claims: map[task.ID]string{}}

	if err := companion.Migrate(context.Background(), s.cr); err != nil {
		_ = db.Close()
		_ = eng.Close()

		return nil, err
	}

	return s, nil
}

// rawCodec passes payloads through verbatim: tq's payloads are already
// JSON text and the adapter surfaces them as jsontext.Value.
func rawCodec() uqueue.Codec[[]byte] {
	return uqueue.Codec[[]byte]{
		Encode: func(v []byte) ([]byte, error) { return v, nil },
		Decode: func(b []byte) ([]byte, error) { return b, nil },
	}
}

// Close releases both the engine and the companion connection.
func (s *Store) Close() error {
	err := s.eng.Close()
	if cerr := s.db.Close(); err == nil {
		err = cerr
	}

	return err
}

// --- core lifecycle: delegated to the engine, owner strings mapped to
// claim tokens through the in-process ledger ---

// Enqueue persists a new task and records the enqueued fact (idempotent
// per DedupKey, per the engine).
func (s *Store) Enqueue(ctx context.Context, n task.New) (task.Task, error) {
	payload := []byte(n.Payload)
	if payload == nil {
		payload = []byte{}
	}

	un := utask.New[[]byte]{
		Project:     n.Project,
		Type:        n.Type,
		Payload:     payload,
		Deps:        toUIDs(n.Deps),
		Priority:    n.Priority,
		MaxAttempts: n.MaxAttempts,
		NotBefore:   n.NotBefore,
		DedupKey:    n.DedupKey,
	}

	got, err := s.eng.Enqueue(ctx, un.Normalize())
	if err != nil {
		return task.Task{}, translateErr(err)
	}

	return s.fromUTask(ctx, got)
}

// ClaimDue atomically claims at most one due task for owner. The minted
// engine claim token IS the returned Claim — finalizes are token-fenced.
func (s *Store) ClaimDue(ctx context.Context, owner string, lease time.Duration) (task.Task, queue.Claim, error) {
	claim, err := s.eng.ClaimDue(ctx, owner, lease)
	if err != nil {
		return task.Task{}, "", translateErr(err)
	}

	s.mu.Lock()
	s.claims[task.ID(claim.Task.ID)] = claim.Token
	s.mu.Unlock()

	t, err := s.fromUTask(ctx, claim.Task)

	return t, queue.Claim(claim.Token), err
}

// Complete marks a Running task Completed (lease must be held).
func (s *Store) Complete(ctx context.Context, id task.ID, claim queue.Claim, result jsontext.Value) error {
	token, err := s.tokenFor(id, claim)
	if err != nil {
		return err
	}

	return translateErr(s.eng.Complete(ctx, utask.ID(id), token, []byte(result)))
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
	token, err := s.tokenFor(id, claim)
	if err != nil {
		return err
	}

	return translateErr(s.eng.Fail(ctx, utask.ID(id), token, errText, backoff, []byte(evidence)))
}

// FailPermanent dead-letters a Running task immediately.
func (s *Store) FailPermanent(ctx context.Context, id task.ID, claim queue.Claim, errText string, evidence jsontext.Value) error {
	token, err := s.tokenFor(id, claim)
	if err != nil {
		return err
	}

	return translateErr(s.eng.FailPermanent(ctx, utask.ID(id), token, errText, []byte(evidence)))
}

// Requeue returns a claimed task to Pending WITHOUT counting an attempt.
// DIVERGENCE: the resumeCloseout flag has no upstream carrier and is not
// persisted; the task.requeued fact carries reason and retry_in_ms only.
func (s *Store) Requeue(
	ctx context.Context,
	id task.ID,
	claim queue.Claim,
	errText string,
	delay time.Duration,
	resumeCloseout bool,
) error {
	token, err := s.tokenFor(id, claim)
	if err != nil {
		return err
	}

	return translateErr(s.eng.Requeue(ctx, utask.ID(id), token, errText, delay))
}

// Heartbeat extends the lease of a Running task held by owner.
func (s *Store) Heartbeat(ctx context.Context, id task.ID, claim queue.Claim, extend time.Duration) error {
	token, err := s.tokenFor(id, claim)
	if err != nil {
		return err
	}

	return translateErr(s.eng.Heartbeat(ctx, utask.ID(id), token, extend))
}

// Cancel withdraws a Pending task.
func (s *Store) Cancel(ctx context.Context, id task.ID, reason string) error {
	return translateErr(s.eng.Cancel(ctx, utask.ID(id), reason))
}

// CancelRunning records a cooperative cancel request for a Running task.
func (s *Store) CancelRunning(ctx context.Context, id task.ID, reason string) error {
	return translateErr(s.eng.CancelRunning(ctx, utask.ID(id), reason))
}

// CancelRequested reports whether a cooperative cancel request is pending.
func (s *Store) CancelRequested(ctx context.Context, id task.ID) (bool, error) {
	requested, err := s.eng.CancelRequested(ctx, utask.ID(id))

	return requested, translateErr(err)
}

// CancelOwned finalizes a cooperative cancel.
func (s *Store) CancelOwned(ctx context.Context, id task.ID, claim queue.Claim) error {
	token, err := s.tokenFor(id, claim)
	if err != nil {
		return err
	}

	return translateErr(s.eng.CancelOwned(ctx, utask.ID(id), token))
}

// MarkOrphaned appends orphaned facts for expired leases, idempotently.
func (s *Store) MarkOrphaned(ctx context.Context, cutoff time.Time) (int, error) {
	marked, err := s.eng.MarkOrphaned(ctx, cutoff)

	return marked, translateErr(err)
}

// RescueDead re-queues a Dead task with a fresh attempt budget.
func (s *Store) RescueDead(ctx context.Context, id task.ID, maxAttempts int) error {
	return translateErr(s.eng.RescueDead(ctx, utask.ID(id), maxAttempts))
}

// DismissDead cancels a Dead task with a recorded reason.
func (s *Store) DismissDead(ctx context.Context, id task.ID, reason, by string) error {
	return translateErr(s.eng.DismissDead(ctx, utask.ID(id), reason, by))
}

// UpdatePendingPriority changes a PENDING task's priority (same-tx fact).
func (s *Store) UpdatePendingPriority(ctx context.Context, id task.ID, newPriority int, source, reason string) error {
	return translateErr(s.eng.UpdatePendingPriority(ctx, utask.ID(id), newPriority, source, reason))
}

// Get returns the current task record.
func (s *Store) Get(ctx context.Context, id task.ID) (task.Task, error) {
	got, err := s.eng.Get(ctx, utask.ID(id))
	if err != nil {
		return task.Task{}, translateErr(err)
	}

	return s.fromUTask(ctx, got)
}

// StatusCounts counts tasks per status in one GROUP BY.
func (s *Store) StatusCounts(ctx context.Context) (map[task.Status]int, error) {
	counts, err := s.eng.StatusCounts(ctx)
	if err != nil {
		return nil, translateErr(err)
	}

	out := make(map[task.Status]int, len(counts))
	for st, n := range counts {
		out[task.Status(st)] = n
	}

	return out, nil
}

// Facts exposes the journal: facts with Seq strictly greater than after.
func (s *Store) Facts(ctx context.Context, after int64, limit int) ([]journal.Fact, error) {
	facts, err := s.eng.Facts(ctx, after, limit)
	if err != nil {
		return nil, translateErr(err)
	}

	return fromUFacts(facts), nil
}

// FactsForTask returns one task's facts in Seq order, bounded to the most
// recent limit when > 0.
func (s *Store) FactsForTask(ctx context.Context, id string, limit int) ([]journal.Fact, error) {
	facts, err := s.eng.FactsForTask(ctx, utask.ID(id), limit)
	if err != nil {
		return nil, translateErr(err)
	}

	return fromUFacts(facts), nil
}

// HeadSeq returns the current highest fact Seq (0 when the journal is
// empty).
func (s *Store) HeadSeq(ctx context.Context) (int64, error) {
	seq, err := s.eng.HeadSeq(ctx)

	return seq, translateErr(err)
}

// Watermark returns the persisted read cursor for a journal consumer.
func (s *Store) Watermark(ctx context.Context, consumer string) (int64, bool, error) {
	seq, exists, err := s.eng.Watermark(ctx, consumer)

	return seq, exists, translateErr(err)
}

// SaveWatermark checkpoints a consumer cursor as a monotonic upsert.
func (s *Store) SaveWatermark(ctx context.Context, consumer string, seq int64) error {
	return translateErr(s.eng.SaveWatermark(ctx, consumer, seq))
}

// --- translation helpers ---

// tokenFor maps a finalize's presented claim onto the engine's claim
// token recorded at ClaimDue. An unknown or foreign claim is
// task.ErrLeaseNotHeld — theft detection lives in the store (ADR-0019 S1).
func (s *Store) tokenFor(id task.ID, claim queue.Claim) (string, error) {
	s.mu.Lock()
	token, ok := s.claims[id]
	s.mu.Unlock()

	if !ok || token != string(claim) {
		return "", task.ErrLeaseNotHeld
	}

	return token, nil
}

// fromUTask converts an engine task row into tq's task record, backfilling
// the DedupKey projection field the engine's Task type does not carry.
func (s *Store) fromUTask(ctx context.Context, ut utask.Task[[]byte]) (task.Task, error) {
	t := task.Task{
		ID:           task.ID(ut.ID),
		Project:      ut.Project,
		Type:         ut.Type,
		Payload:      jsontext.Value(ut.Payload),
		Deps:         fromUIDs(ut.Deps),
		Priority:     ut.Priority,
		Attempts:     ut.Attempts,
		MaxAttempts:  ut.MaxAttempts,
		NotBefore:    ut.NotBefore,
		Status:       task.Status(ut.Status),
		LeaseOwner:   ut.LeaseOwner,
		LastError:    ut.LastError,
		CreatedAt:    ut.CreatedAt,
		UpdatedAt:    ut.UpdatedAt,
		CompletedAt:  ut.CompletedAt,
		LeaseExpires: ut.LeaseExpires,
	}

	var dedup string

	err := s.db.QueryRowContext(ctx,
		`SELECT dedup_key FROM tasks WHERE id = ?`, ut.ID.String()).Scan(&dedup)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return task.Task{}, fmt.Errorf("cqrsqlite: dedup backfill: %w", err)
	}

	t.DedupKey = dedup

	return t, nil
}

func toUIDs(deps []task.ID) []utask.ID {
	out := make([]utask.ID, 0, len(deps))
	for _, d := range deps {
		out = append(out, utask.ID(d))
	}

	return out
}

func fromUIDs(deps []utask.ID) []task.ID {
	out := make([]task.ID, 0, len(deps))
	for _, d := range deps {
		out = append(out, task.ID(d))
	}

	return out
}

func fromUFacts(facts []ufacts.Fact) []journal.Fact {
	out := make([]journal.Fact, 0, len(facts))
	for _, f := range facts {
		jf := journal.Fact{
			Seq:     f.Seq,
			Time:    f.Time,
			TaskID:  f.TaskID,
			Type:    journal.FactType(f.Type),
			Owner:   f.Owner,
			Attempt: f.Attempt,
			Error:   f.Error,
		}
		if len(f.Detail) > 0 {
			jf.Detail = jsontext.Value(f.Detail)
		}

		out = append(out, jf)
	}

	return out
}

// translateErr maps engine sentinels onto the tq contract's sentinels so
// callers' errors.Is checks stay stable across the S1 flip.
func translateErr(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, uqueue.ErrNoTaskDue):
		return queue.ErrNoTaskDue
	case errors.Is(err, uqueue.ErrEmptyType):
		return queue.ErrEmptyType
	case errors.Is(err, uqueue.ErrLeaseNotHeld):
		return task.ErrLeaseNotHeld
	case errors.Is(err, uqueue.ErrInvalidTransition):
		return task.ErrInvalidTransition
	case errors.Is(err, uqueue.ErrNotFound):
		return task.ErrNotFound
	default:
		return err
	}
}
