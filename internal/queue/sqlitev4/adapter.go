// Package sqlitev4 is the ADR-0019 S1 spike: a tq queue.Store implemented
// over go-cqrs-lite queue/sqlite/v4 (the upstream engine transcribed from
// this repo's contract) plus tq-side companion surfaces over the SAME
// database (RecordAnswer/questions, the priority_scores cache, the
// CountFacts/FactsSince/LastFacts/ProjectCounts read pushdowns, AppendFact,
// watermark list/set, and tq's project-exclusivity claim).
//
// Division of labor:
//
//   - Engine-backed (upstream queue/sqlite/v4, the S1 target): Enqueue,
//     Complete, Fail, FailPermanent, Heartbeat, Cancel, CancelRunning,
//     CancelRequested, CancelOwned, MarkOrphaned, RescueDead, DismissDead,
//     UpdatePendingPriority.
//   - Adapter-side (same DB file, own single-writer handle): ClaimDue
//     (upstream has no project exclusivity — the per-repo agent-pool
//     guarantee, D24), Requeue (tq's evidence carries resume_closeout),
//     all reads, RecordAnswer, AppendFact, watermarks, priority scores.
//
// Known divergences are catalogued in docs/status (S1 spike report).
package sqlitev4

import (
	"context"
	"database/sql"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"time"

	uqueue "github.com/larsartmann/go-cqrs-lite/queue/v4"
	usqlite "github.com/larsartmann/go-cqrs-lite/queue/sqlite/v4"
	utask "github.com/larsartmann/go-cqrs-lite/queue/v4/task"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
	_ "modernc.org/sqlite" // pure-Go SQLite driver (CGo-free)
)

// Store is the S1 spike store: tq's queue.Store contract over the upstream
// engine + companion surfaces on the same SQLite file.
type Store struct {
	engine *usqlite.Store[[]byte]
	db     *sql.DB

	projectExclusive bool
}

// Store implements the tq queue contract at compile time.
var _ queue.Store = (*Store)(nil)

// StoreOption configures optional Store behavior.
type StoreOption func(*storeOptions)

type storeOptions struct {
	projectExclusive bool
}

// WithProjectExclusivity mirrors internal/queue/sqlite's option: ClaimDue
// will not claim a task whose project already has another running task.
// The upstream engine has no such predicate, so the adapter owns ClaimDue
// entirely (divergence noted in the S1 report).
func WithProjectExclusivity() StoreOption {
	return func(o *storeOptions) { o.projectExclusive = true }
}

// Open opens (creating if needed) the queue database at path.
func Open(path string, opts ...StoreOption) (*Store, error) {
	var options storeOptions
	for _, opt := range opts {
		opt(&options)
	}

	engine, err := usqlite.Open[[]byte](path, usqlite.WithCodec(identityCodec()))
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

	store := &Store{engine: engine, db: db, projectExclusive: options.projectExclusive}

	if err := store.migrateCompanion(context.Background()); err != nil {
		_ = db.Close()
		_ = engine.Close()

		return nil, err
	}

	return store, nil
}

// identityCodec passes payloads through byte-for-byte: tq payloads are
// jsontext.Value, already JSON; the default JSONCodec would base64-encode
// the bytes.
func identityCodec() uqueue.Codec[[]byte] {
	return uqueue.Codec[[]byte]{
		Encode: func(v []byte) ([]byte, error) { return v, nil },
		Decode: func(b []byte) ([]byte, error) { return b, nil },
	}
}

// companionSchema is the tq-side table set the upstream engine does not
// carry (the tasks/facts/deps/watermarks tables come from the engine's own
// migrate — same-DB companion surfaces read and extend them but never
// redefine them).
const companionSchema = `
CREATE TABLE IF NOT EXISTS priority_scores (
	item_key       TEXT PRIMARY KEY, -- the harvest dedup key (repo + item text)
	score          INTEGER NOT NULL, -- 0-100
	effort_minutes INTEGER NOT NULL, -- estimated agent effort
	source         TEXT NOT NULL,    -- scorer identity, e.g. "ai:<model>"
	reasoning      TEXT NOT NULL,    -- one-line why
	tokens         INTEGER NOT NULL, -- what the verdict cost
	scored_at      INTEGER NOT NULL  -- unix millis
);
`

func (s *Store) migrateCompanion(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, companionSchema); err != nil {
		return fmt.Errorf("sqlitev4: migrate companion: %w", err)
	}

	return nil
}

// Close releases the engine and the companion handle.
func (s *Store) Close() error {
	errEngine := s.engine.Close()
	errDB := s.db.Close()
	if errEngine != nil {
		return errEngine
	}

	return errDB
}

// mapErr translates upstream engine errors onto tq's sentinels so callers
// program against one error vocabulary.
func mapErr(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, uqueue.ErrNoTaskDue):
		return queue.ErrNoTaskDue
	case errors.Is(err, uqueue.ErrEmptyType):
		return queue.ErrEmptyType
	case errors.Is(err, uqueue.ErrNotFound):
		return task.ErrNotFound
	case errors.Is(err, uqueue.ErrLeaseNotHeld):
		return task.ErrLeaseNotHeld
	case errors.Is(err, uqueue.ErrInvalidTransition):
		return fmt.Errorf("%w: %w", task.ErrInvalidTransition, err)
	default:
		return err
	}
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
		return task.Task{}, mapErr(err)
	}

	return s.Get(ctx, task.ID(created.ID.String()))
}

// tokenFor resolves the current claim token for an owner-fenced finalize
// and enforces tq's owner-string gate: the caller must hold the live lease.
// This is the S1 bridge between tq's owner-string finalizes and upstream's
// token-fenced ones — theft detection stays at this gate (tq semantics),
// not in the engine (upstream semantics).
func (s *Store) tokenFor(ctx context.Context, id task.ID, owner string, requireLive bool) (string, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT status, lease_owner, COALESCE(lease_expires, 0), COALESCE(lease_token, '')
		FROM tasks WHERE id = ?`, id.String())

	var (
		status   string
		leaseOwn string
		expires  int64
		token    string
	)
	if err := row.Scan(&status, &leaseOwn, &expires, &token); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", task.ErrNotFound
		}

		return "", err
	}

	if status != "running" || leaseOwn != owner {
		return "", task.ErrLeaseNotHeld
	}

	if requireLive && (expires == 0 || expires <= time.Now().UnixMilli()) {
		return "", task.ErrLeaseNotHeld
	}

	if token == "" {
		return "", task.ErrLeaseNotHeld
	}

	return token, nil
}

// Complete marks a Running task Completed (owner gate, engine finalize).
func (s *Store) Complete(ctx context.Context, id task.ID, owner string, result jsontext.Value) error {
	token, err := s.tokenFor(ctx, id, owner, true)
	if err != nil {
		return err
	}

	return mapErr(s.engine.Complete(ctx, utask.ID(id.String()), token, []byte(result)))
}

// Fail records a failed attempt: retry with backoff or dead-letter.
func (s *Store) Fail(
	ctx context.Context,
	id task.ID,
	owner string,
	errText string,
	backoff time.Duration,
	evidence jsontext.Value,
) error {
	token, err := s.tokenFor(ctx, id, owner, true)
	if err != nil {
		return err
	}

	return mapErr(s.engine.Fail(ctx, utask.ID(id.String()), token, errText, backoff, []byte(evidence)))
}

// FailPermanent dead-letters immediately (permanent error class).
func (s *Store) FailPermanent(
	ctx context.Context,
	id task.ID,
	owner string,
	errText string,
	evidence jsontext.Value,
) error {
	token, err := s.tokenFor(ctx, id, owner, true)
	if err != nil {
		return err
	}

	return mapErr(s.engine.FailPermanent(ctx, utask.ID(id.String()), token, errText, []byte(evidence)))
}

// Heartbeat extends the lease of a Running task held by owner.
func (s *Store) Heartbeat(ctx context.Context, id task.ID, owner string, extend time.Duration) error {
	token, err := s.tokenFor(ctx, id, owner, true)
	if err != nil {
		return err
	}

	return mapErr(s.engine.Heartbeat(ctx, utask.ID(id.String()), token, extend))
}

// Cancel withdraws a Pending task (engine; error vocabulary mapped).
func (s *Store) Cancel(ctx context.Context, id task.ID, reason string) error {
	return mapErr(s.engine.Cancel(ctx, utask.ID(id.String()), reason))
}

// CancelRunning records a cooperative cancel request for a Running task.
func (s *Store) CancelRunning(ctx context.Context, id task.ID, reason string) error {
	return mapErr(s.engine.CancelRunning(ctx, utask.ID(id.String()), reason))
}

// CancelRequested reports whether a cooperative cancel request is pending.
func (s *Store) CancelRequested(ctx context.Context, id task.ID) (bool, error) {
	requested, err := s.engine.CancelRequested(ctx, utask.ID(id.String()))

	return requested, mapErr(err)
}

// CancelOwned finalizes a cooperative cancel. tq's gate is status+owner
// (no live-lease requirement — a worker may legitimately finish the stop
// just after the lease lapsed but before a reclaim), so requireLive=false.
func (s *Store) CancelOwned(ctx context.Context, id task.ID, owner string) error {
	token, err := s.tokenFor(ctx, id, owner, false)
	if err != nil {
		return err
	}

	return mapErr(s.engine.CancelOwned(ctx, utask.ID(id.String()), token))
}

// MarkOrphaned appends task.orphaned facts for expired-lease Running tasks.
func (s *Store) MarkOrphaned(ctx context.Context, cutoff time.Time) (int, error) {
	n, err := s.engine.MarkOrphaned(ctx, cutoff)

	return n, mapErr(err)
}

// RescueDead re-queues a Dead task with a fresh attempt budget.
func (s *Store) RescueDead(ctx context.Context, id task.ID, maxAttempts int) error {
	return mapErr(s.engine.RescueDead(ctx, utask.ID(id.String()), maxAttempts))
}

// DismissDead cancels a Dead task with a recorded reason.
func (s *Store) DismissDead(ctx context.Context, id task.ID, reason, by string) error {
	return mapErr(s.engine.DismissDead(ctx, utask.ID(id.String()), reason, by))
}

// UpdatePendingPriority changes a PENDING task's priority (in-tx fact).
func (s *Store) UpdatePendingPriority(ctx context.Context, id task.ID, newPriority int, source, reason string) error {
	return mapErr(s.engine.UpdatePendingPriority(ctx, utask.ID(id.String()), newPriority, source, reason))
}

// ClaimDue atomically claims at most one due task for owner, with tq's
// project-exclusivity predicate (the upstream engine's candidate query has
// no such clause — S1 divergence). The claim mints an upstream-format
// fencing token and stamps it on the row, so engine-backed finalizes keep
// working for the claimed task.
func (s *Store) ClaimDue(ctx context.Context, owner string, lease time.Duration) (task.Task, error) {
	now := time.Now()

	var claimed task.Task

	finalizedCancel := false

	err := s.withTx(ctx, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			SELECT t.id, t.status, t.lease_owner FROM tasks t
			WHERE ((t.status = 'pending' AND t.not_before <= ?)
			    OR (t.status = 'running' AND t.lease_expires IS NOT NULL AND t.lease_expires <= ?))
			  AND NOT EXISTS (
			    SELECT 1 FROM deps d JOIN tasks dt ON dt.id = d.dep_id
			    WHERE d.task_id = t.id AND dt.status != 'completed'
			  )
			  AND (? = 0 OR t.project = '' OR NOT EXISTS (
			    SELECT 1 FROM tasks r
			    WHERE r.project = t.project AND r.status = 'running' AND r.id != t.id
			  ))
			ORDER BY t.priority + MIN((? - t.created_at) / 86400000.0 / ?, ?) DESC, t.created_at ASC, t.id ASC
			LIMIT 1`, now.UnixMilli(), now.UnixMilli(), boolInt(s.projectExclusive),
			now.UnixMilli(), float64(queue.PriorityAgingDaysPerPoint), float64(queue.PriorityAgingMaxBonus))

		var id, st, prevOwner string
		if err := row.Scan(&id, &st, &prevOwner); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return queue.ErrNoTaskDue
			}

			return err
		}

		if st == "running" {
			requested, err := cancelRequestedTx(ctx, tx, id)
			if err != nil {
				return err
			}

			if requested {
				reason, err := cancelRequestedReasonTx(ctx, tx, id)
				if err != nil {
					return err
				}

				if err := s.appendFact(ctx, tx, journal.Fact{
					TaskID: id, Type: journal.Released, Owner: prevOwner,
				}); err != nil {
					return err
				}

				res, err := tx.ExecContext(ctx, `
					UPDATE tasks SET status = 'cancelled', updated_at = ?, lease_owner = '', lease_expires = NULL
					WHERE id = ? AND status = 'running'`, now.UnixMilli(), id)
				if err != nil {
					return err
				}

				if n, _ := res.RowsAffected(); n == 0 {
					return queue.ErrNoTaskDue
				}

				if err := s.appendFact(ctx, tx, journal.Fact{
					TaskID: id, Type: journal.Cancelled, Owner: owner,
					Detail: cooperativeCancelDetail(reason, "lease-expiry"),
				}); err != nil {
					return err
				}

				finalizedCancel = true

				return nil
			}

			if err := s.appendFact(ctx, tx, journal.Fact{
				TaskID: id, Type: journal.Released, Owner: prevOwner,
			}); err != nil {
				return err
			}
		}

		res, err := tx.ExecContext(ctx, `
			UPDATE tasks
			SET status = 'running', lease_owner = ?, lease_expires = ?, lease_token = ?, updated_at = ?
			WHERE id = ? AND (
			    (status = 'pending' AND not_before <= ?)
			    OR (status = 'running' AND lease_expires IS NOT NULL AND lease_expires <= ?))`,
			owner, now.Add(lease).UnixMilli(), uqueue.NewClaimToken(), now.UnixMilli(), id, now.UnixMilli(), now.UnixMilli())
		if err != nil {
			return err
		}

		n, err := res.RowsAffected()
		if err != nil {
			return err
		}

		if n == 0 {
			return queue.ErrNoTaskDue
		}

		if err := s.appendFact(ctx, tx, journal.Fact{TaskID: id, Type: journal.Claimed, Owner: owner}); err != nil {
			return err
		}

		claimed, err = loadTaskTx(ctx, tx, id)

		return err
	})
	if err != nil {
		return task.Task{}, err
	}

	if finalizedCancel {
		return task.Task{}, queue.ErrNoTaskDue
	}

	return claimed, nil
}

// Requeue returns a claimed task to Pending without counting an attempt.
// Adapter-side (not the engine's): tq's task.requeued evidence carries the
// resume_closeout flag, which the upstream RequeueEvidence lacks (S1
// divergence).
func (s *Store) Requeue(
	ctx context.Context,
	id task.ID,
	owner string,
	errText string,
	delay time.Duration,
	resumeCloseout bool,
) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		now := time.Now()

		res, err := tx.ExecContext(ctx, `
			UPDATE tasks
			SET status = 'pending', not_before = ?, last_error = ?, updated_at = ?,
			    lease_owner = '', lease_expires = NULL
			WHERE id = ? AND status = 'running' AND lease_owner = ?`,
			now.Add(delay).UnixMilli(), errText, now.UnixMilli(), id.String(), owner)
		if err != nil {
			return err
		}

		if n, _ := res.RowsAffected(); n == 0 {
			return leaseErr(ctx, tx, id, owner)
		}

		return s.appendFact(ctx, tx, journal.Fact{
			TaskID: id.String(), Type: journal.Requeued, Owner: owner, Error: errText,
			Detail: mustJSON(queue.RequeueEvidence{
				Reason: errText, RetryIn: delay.Milliseconds(), ResumeCloseout: resumeCloseout,
			}),
		})
	})
}

// AppendFact records a NON-task journal fact (session.opened /
// session.closed). Task facts are never written through it.
func (s *Store) AppendFact(ctx context.Context, f journal.Fact) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.appendFact(ctx, tx, f)
	})
}

func (s *Store) appendFact(ctx context.Context, tx *sql.Tx, f journal.Fact) error {
	if f.Time.IsZero() {
		f.Time = time.Now()
	}

	detail := string(f.Detail)
	_, err := tx.ExecContext(ctx,
		`INSERT INTO facts (time, task_id, type, owner, attempt, error, detail)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		f.Time.UnixMilli(), f.TaskID, string(f.Type), f.Owner, f.Attempt, f.Error, detail)

	return err
}

// RecordAnswer records an owner's decision for a parked task's question.
// Same-transaction payload injection + NotBefore clear + fact, idempotent
// per question ref — tq semantics over the shared tasks/facts tables.
func (s *Store) RecordAnswer(ctx context.Context, id task.ID, ans queue.AnswerRecord) error {
	if ans.Ref == "" {
		return queue.ErrEmptyAnswerRef
	}

	if strings.TrimSpace(ans.Answer) == "" {
		return queue.ErrEmptyAnswer
	}

	now := time.Now()

	answeredAt := ans.AnsweredAt
	if answeredAt.IsZero() {
		answeredAt = now
	}

	return s.withTx(ctx, func(tx *sql.Tx) error {
		answered, err := factDetailRefs(ctx, tx, id.String(), journal.QuestionAnswered)
		if err != nil {
			return err
		}

		if _, done := answered[ans.Ref]; done {
			return nil
		}

		var status string

		var payload string

		err = tx.QueryRowContext(ctx,
			`SELECT status, payload FROM tasks WHERE id = ?`, id.String()).
			Scan(&status, &payload)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return task.ErrNotFound
			}

			return err
		}

		question := ans.Question
		if question == "" {
			if question, err = askedQuestionText(ctx, tx, id, ans.Ref); err != nil {
				return err
			}
		}

		detail := queue.QuestionAnsweredDetail{
			Ref:        ans.Ref,
			Question:   question,
			Answer:     ans.Answer,
			PapID:      ans.PapID,
			AnsweredAt: answeredAt.UnixMilli(),
		}

		if status == "pending" {
			if err := unblockParkedTask(ctx, tx, id, payload, question, ans, now, answeredAt); err != nil {
				return err
			}
		}

		return s.appendFact(ctx, tx, journal.Fact{
			TaskID: id.String(),
			Type:   journal.QuestionAnswered,
			Detail: mustJSON(detail),
		})
	})
}

// askedQuestionText backfills the question text from the task's
// task.question-asked fact when the answer record does not carry it.
func askedQuestionText(ctx context.Context, tx *sql.Tx, id task.ID, ref string) (string, error) {
	asked, err := factDetailRefs(ctx, tx, id.String(), journal.QuestionAsked)
	if err != nil {
		return "", err
	}

	return asked[ref], nil
}

// unblockParkedTask injects the answer into a PARKED task's payload and
// clears its NotBefore so the re-claim is immediate.
func unblockParkedTask(
	ctx context.Context,
	tx *sql.Tx,
	id task.ID,
	payload, question string,
	ans queue.AnswerRecord,
	now, answeredAt time.Time,
) error {
	merged, ok, err := mergeAnsweredPayload(payload, ans, question, answeredAt)
	if err != nil {
		return err
	}

	if !ok {
		return nil
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE tasks
		SET payload = ?, not_before = ?, updated_at = ?
		WHERE id = ? AND status = 'pending'`,
		string(merged), now.UnixMilli(), now.UnixMilli(), id.String())

	return err
}

// factDetailRefs scans a task's facts of one question type and returns
// ref -> text.
func factDetailRefs(ctx context.Context, tx *sql.Tx, taskID string, ftype journal.FactType) (map[string]string, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT detail FROM facts WHERE task_id = ? AND type = ?`,
		taskID, string(ftype))
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	out := map[string]string{}

	for rows.Next() {
		var detail string

		if err := rows.Scan(&detail); err != nil {
			return nil, err
		}

		switch ftype {
		case journal.QuestionAsked:
			var parsed queue.QuestionAskedDetail
			if err := json.Unmarshal(jsontext.Value(detail), &parsed); err != nil {
				continue
			}

			out[parsed.Ref] = parsed.Question
		case journal.QuestionAnswered:
			var parsed queue.QuestionAnsweredDetail
			if err := json.Unmarshal(jsontext.Value(detail), &parsed); err != nil {
				continue
			}

			out[parsed.Ref] = parsed.Answer
		default:
		}
	}

	return out, rows.Err()
}

// mergeAnsweredPayload injects one answered question into a JSON-object
// payload's "answered" array. Raw (non-object) payloads report ok=false.
func mergeAnsweredPayload(
	payload string,
	ans queue.AnswerRecord,
	question string,
	answeredAt time.Time,
) (jsontext.Value, bool, error) {
	trimmed := strings.TrimSpace(payload)
	if !strings.HasPrefix(trimmed, "{") {
		return jsontext.Value(payload), false, nil
	}

	var obj map[string]any
	if err := json.Unmarshal(jsontext.Value(trimmed), &obj); err != nil {
		return jsontext.Value(payload), false, nil
	}

	answered, _ := obj["answered"].([]any)
	answered = append(answered, map[string]any{
		"ref":         ans.Ref,
		"question":    question,
		"answer":      ans.Answer,
		"pap_id":      ans.PapID,
		"answered_at": answeredAt.UnixMilli(),
	})
	obj["answered"] = answered

	merged, err := json.Marshal(obj)
	if err != nil {
		return jsontext.Value(payload), false, err
	}

	return merged, true, nil
}

// Get returns the current task record.
func (s *Store) Get(ctx context.Context, id task.ID) (task.Task, error) {
	return loadTaskTx(ctx, s.db, id.String())
}

// listWhere builds the shared WHERE clause for List and CountTasks.
func listWhere(f queue.Filter) (string, []any) {
	where := []string{"1=1"}
	args := []any{}

	if f.Project != nil {
		where = append(where, "project = ?")
		args = append(args, *f.Project)
	}

	if f.Status != nil {
		where = append(where, "status = ?")
		args = append(args, string(*f.Status))
	}

	if f.Type != nil {
		where = append(where, "type = ?")
		args = append(args, *f.Type)
	}

	if f.Since != nil {
		where = append(where, "created_at >= ?")
		args = append(args, f.Since.UnixMilli())
	}

	if f.Parked != nil && *f.Parked {
		where = append(where, "status = 'pending' AND not_before > ?")
		args = append(args, time.Now().UnixMilli())
	}

	if f.PriorityMin != nil {
		where = append(where, "priority >= ?")
		args = append(args, *f.PriorityMin)
	}

	if f.PriorityMax != nil {
		where = append(where, "priority <= ?")
		args = append(args, *f.PriorityMax)
	}

	if f.Query != "" {
		like := "%" + escapeLike(strings.ToLower(f.Query)) + "%"

		where = append(where, `(id LIKE ? ESCAPE '\' OR type LIKE ? ESCAPE '\' OR
			project LIKE ? ESCAPE '\' OR payload LIKE ? ESCAPE '\' OR
			lease_owner LIKE ? ESCAPE '\' OR last_error LIKE ? ESCAPE '\')`)
		args = append(args, like, like, like, like, like, like)
	}

	return strings.Join(where, " AND "), args
}

// List returns tasks matching the filter.
func (s *Store) List(ctx context.Context, f queue.Filter) ([]task.Task, error) {
	where, args := listWhere(f)

	order := `ORDER BY priority DESC, created_at ASC`
	if f.SeverityOrder {
		order = `ORDER BY CASE status
			WHEN 'dead' THEN 0
			WHEN 'running' THEN 1
			WHEN 'pending' THEN 2
			WHEN 'cancelled' THEN 3
			ELSE 4 END, created_at DESC`
	}

	switch f.Sort {
	case "age-asc":
		order = `ORDER BY created_at ASC, id ASC`
	case "age-desc":
		order = `ORDER BY created_at DESC, id DESC`
	case "priority-asc":
		order = `ORDER BY priority ASC, created_at ASC, id ASC`
	case "priority-desc":
		order = `ORDER BY priority DESC, created_at ASC, id ASC`
	case "attempts-asc":
		order = `ORDER BY attempts ASC, created_at ASC, id ASC`
	case "attempts-desc":
		order = `ORDER BY attempts DESC, created_at DESC, id DESC`
	}

	q := `SELECT id, project, type, payload, deps, priority, attempts, max_attempts,
	             not_before, status, lease_owner, lease_expires, last_error,
	             created_at, updated_at, completed_at, dedup_key
	      FROM tasks WHERE ` + where + `
	      ` + order

	if f.Limit > 0 || f.Offset > 0 {
		if f.Limit > 0 {
			q += " LIMIT ?"

			args = append(args, f.Limit)
		} else {
			q += " LIMIT -1"
		}

		if f.Offset > 0 {
			q += " OFFSET ?"

			args = append(args, f.Offset)
		}
	}

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []task.Task

	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}

		out = append(out, t)
	}

	return out, rows.Err()
}

// CountTasks counts the tasks matching the filter (COUNT(*) pushdown).
func (s *Store) CountTasks(ctx context.Context, f queue.Filter) (int, error) {
	where, args := listWhere(f)

	var n int

	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE `+where, args...).Scan(&n)

	return n, err
}

// Facts returns journal facts with Seq > after, ascending, bounded to the
// most recent limit when > 0.
func (s *Store) Facts(ctx context.Context, after int64, limit int) ([]journal.Fact, error) {
	query := `
		SELECT seq, time, task_id, type, owner, attempt, error, detail
		FROM facts WHERE seq > ? ORDER BY seq ASC`
	args := []any{after}

	if limit > 0 {
		query += ` LIMIT ?`

		args = append(args, limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanFacts(rows)
}

// LastFacts returns the most recent limit facts in ascending Seq order.
func (s *Store) LastFacts(ctx context.Context, limit int) ([]journal.Fact, error) {
	query := `
		SELECT seq, time, task_id, type, owner, attempt, error, detail
		FROM facts`
	args := []any{}

	if limit > 0 {
		query += ` ORDER BY seq DESC LIMIT ?`

		args = append(args, limit)
	} else {
		query += ` ORDER BY seq ASC`
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	facts, err := scanFacts(rows)
	if err != nil {
		return nil, err
	}

	if limit > 0 {
		for i, j := 0, len(facts)-1; i < j; i, j = i+1, j-1 {
			facts[i], facts[j] = facts[j], facts[i]
		}
	}

	return facts, nil
}

// HeadSeq returns the current highest fact Seq (0 when the journal is empty).
func (s *Store) HeadSeq(ctx context.Context) (int64, error) {
	var seq int64

	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq), 0) FROM facts`).Scan(&seq)

	return seq, err
}

// FactsForTask returns one task's facts in Seq order, bounded to the most
// recent limit when > 0.
func (s *Store) FactsForTask(ctx context.Context, id string, limit int) ([]journal.Fact, error) {
	query := `
		SELECT seq, time, task_id, type, owner, attempt, error, detail
		FROM facts WHERE task_id = ?`
	args := []any{id}

	if limit > 0 {
		query += ` ORDER BY seq DESC LIMIT ?`

		args = append(args, limit)
	} else {
		query += ` ORDER BY seq ASC`
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	facts, err := scanFacts(rows)
	if err != nil {
		return nil, err
	}

	if limit > 0 {
		for i, j := 0, len(facts)-1; i < j; i, j = i+1, j-1 {
			facts[i], facts[j] = facts[j], facts[i]
		}
	}

	return facts, nil
}

// CountFacts counts facts of one type recorded at or after since.
func (s *Store) CountFacts(ctx context.Context, ftype journal.FactType, since time.Time) (int64, error) {
	var n int64

	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM facts WHERE type = ? AND time >= ?`,
		ftype, since.UnixMilli()).Scan(&n)

	return n, err
}

// FactsSince returns facts of one type recorded at or after since.
func (s *Store) FactsSince(
	ctx context.Context,
	ftype journal.FactType,
	since time.Time,
	limit int,
) ([]journal.Fact, error) {
	query := `
		SELECT seq, time, task_id, type, owner, attempt, error, detail
		FROM facts WHERE type = ? AND time >= ? ORDER BY seq ASC`
	args := []any{ftype, since.UnixMilli()}

	if limit > 0 {
		query += ` LIMIT ?`

		args = append(args, limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanFacts(rows)
}

// Watermark returns the persisted read cursor for a journal consumer.
func (s *Store) Watermark(ctx context.Context, consumer string) (int64, bool, error) {
	var seq int64

	err := s.db.QueryRowContext(ctx,
		`SELECT seq FROM watermarks WHERE consumer = ?`, consumer).Scan(&seq)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}

	if err != nil {
		return 0, false, err
	}

	return seq, true, nil
}

// SaveWatermark checkpoints a consumer cursor as a monotonic upsert.
func (s *Store) SaveWatermark(ctx context.Context, consumer string, seq int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO watermarks (consumer, seq, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(consumer) DO UPDATE SET
			seq = excluded.seq,
			updated_at = excluded.updated_at
		WHERE watermarks.seq < excluded.seq`,
		consumer, seq, time.Now().UnixMilli())

	return err
}

// ListWatermarks returns every consumer cursor, by consumer name.
func (s *Store) ListWatermarks(ctx context.Context) ([]queue.WatermarkEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT consumer, seq, updated_at FROM watermarks ORDER BY consumer`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []queue.WatermarkEntry

	for rows.Next() {
		var e queue.WatermarkEntry

		if err := rows.Scan(&e.Consumer, &e.Seq, &e.UpdatedAt); err != nil {
			return nil, err
		}

		out = append(out, e)
	}

	return out, rows.Err()
}

// SetWatermark overwrites a consumer cursor unconditionally (ops rescue).
func (s *Store) SetWatermark(ctx context.Context, consumer string, seq int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO watermarks (consumer, seq, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(consumer) DO UPDATE SET
			seq = excluded.seq,
			updated_at = excluded.updated_at`,
		consumer, seq, time.Now().UnixMilli())

	return err
}

// SavePriorityScore upserts one cached item score (ADR-0015 score cache).
func (s *Store) SavePriorityScore(ctx context.Context, score queue.PriorityScore) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO priority_scores
			(item_key, score, effort_minutes, source, reasoning, tokens, scored_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(item_key) DO UPDATE SET
			score = excluded.score,
			effort_minutes = excluded.effort_minutes,
			source = excluded.source,
			reasoning = excluded.reasoning,
			tokens = excluded.tokens,
			scored_at = excluded.scored_at`,
		score.ItemKey, score.Score, score.EffortMinutes, score.Source, score.Reasoning, score.Tokens, score.ScoredAt)

	return err
}

// PriorityScore returns the cached verdict for an item key, if any.
func (s *Store) PriorityScore(ctx context.Context, itemKey string) (queue.PriorityScore, bool, error) {
	var score queue.PriorityScore

	err := s.db.QueryRowContext(ctx, `
		SELECT item_key, score, effort_minutes, source, reasoning, tokens, scored_at
		FROM priority_scores WHERE item_key = ?`, itemKey).
		Scan(&score.ItemKey, &score.Score, &score.EffortMinutes, &score.Source, &score.Reasoning, &score.Tokens, &score.ScoredAt)
	if errors.Is(err, sql.ErrNoRows) {
		return queue.PriorityScore{}, false, nil
	}

	if err != nil {
		return queue.PriorityScore{}, false, err
	}

	return score, true, nil
}

// PriorityScores returns every cached verdict, ordered by item key.
func (s *Store) PriorityScores(ctx context.Context) ([]queue.PriorityScore, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT item_key, score, effort_minutes, source, reasoning, tokens, scored_at
		FROM priority_scores ORDER BY item_key`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var scores []queue.PriorityScore

	for rows.Next() {
		var score queue.PriorityScore
		if err := rows.Scan(&score.ItemKey, &score.Score, &score.EffortMinutes,
			&score.Source, &score.Reasoning, &score.Tokens, &score.ScoredAt); err != nil {
			return nil, err
		}

		scores = append(scores, score)
	}

	return scores, rows.Err()
}

// DeletePriorityScores removes the cached verdicts for the given item keys.
func (s *Store) DeletePriorityScores(ctx context.Context, itemKeys []string) (int64, error) {
	if len(itemKeys) == 0 {
		return 0, nil
	}

	args := make([]any, 0, len(itemKeys))
	placeholders := strings.Repeat("?,", len(itemKeys))
	placeholders = placeholders[:len(placeholders)-1]

	for _, key := range itemKeys {
		args = append(args, key)
	}

	res, err := s.db.ExecContext(ctx,
		`DELETE FROM priority_scores WHERE item_key IN (`+placeholders+`)`, args...)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}

// StatusCounts counts tasks per status in one GROUP BY.
func (s *Store) StatusCounts(ctx context.Context) (map[task.Status]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM tasks GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[task.Status]int)

	for rows.Next() {
		var (
			st task.Status
			n  int
		)
		if err := rows.Scan(&st, &n); err != nil {
			return nil, err
		}

		out[st] = n
	}

	return out, rows.Err()
}

// ProjectCounts counts tasks per project per status in one GROUP BY.
func (s *Store) ProjectCounts(ctx context.Context) (map[string]map[task.Status]int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT project, status, COUNT(*) FROM tasks GROUP BY project, status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]map[task.Status]int)

	for rows.Next() {
		var (
			project string
			st      task.Status
			n       int
		)
		if err := rows.Scan(&project, &st, &n); err != nil {
			return nil, err
		}

		if out[project] == nil {
			out[project] = make(map[task.Status]int)
		}

		out[project][st] = n
	}

	return out, rows.Err()
}

// --- internals ---

func (s *Store) withTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		_ = tx.Rollback()

		return err
	}

	return tx.Commit()
}

func loadTaskTx(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id string,
) (task.Task, error) {
	row := q.QueryRowContext(ctx, `
		SELECT id, project, type, payload, deps, priority, attempts, max_attempts,
		       not_before, status, lease_owner, lease_expires, last_error,
		       created_at, updated_at, completed_at, dedup_key
		FROM tasks WHERE id = ?`, id)

	t, err := scanTaskRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return task.Task{}, task.ErrNotFound
		}

		return task.Task{}, err
	}

	return t, nil
}

type scanner interface{ Scan(dest ...any) error }

func scanTask(rows *sql.Rows) (task.Task, error) { return scanTaskRow(rows) }

func scanTaskRow(r scanner) (task.Task, error) {
	var (
		t            task.Task
		id           string
		payload      string
		deps         string
		notBeforeMS  int64
		status       string
		leaseExpires sql.NullInt64
		completedAt  sql.NullInt64
		createdAtMS  int64
		updatedAtMS  int64
	)
	if err := r.Scan(&id, &t.Project, &t.Type, &payload, &deps, &t.Priority, &t.Attempts,
		&t.MaxAttempts, &notBeforeMS, &status, &t.LeaseOwner, &leaseExpires, &t.LastError,
		&createdAtMS, &updatedAtMS, &completedAt, &t.DedupKey); err != nil {
		return task.Task{}, err
	}

	t.ID = task.ID(id)
	t.Status = task.Status(status)
	t.Payload = jsontext.Value(payload)
	t.NotBefore = time.UnixMilli(notBeforeMS)
	t.CreatedAt = time.UnixMilli(createdAtMS)
	t.UpdatedAt = time.UnixMilli(updatedAtMS)
	if leaseExpires.Valid {
		le := time.UnixMilli(leaseExpires.Int64)
		t.LeaseExpires = &le
	}

	if completedAt.Valid {
		ca := time.UnixMilli(completedAt.Int64)
		t.CompletedAt = &ca
	}

	if deps != "" && deps != "[]" {
		if err := json.Unmarshal([]byte(deps), &t.Deps); err != nil {
			return task.Task{}, fmt.Errorf("sqlitev4: unmarshal deps for %s: %w", id, err)
		}
	}

	return t, nil
}

func scanFacts(rows *sql.Rows) ([]journal.Fact, error) {
	var out []journal.Fact

	for rows.Next() {
		var (
			f      journal.Fact
			ms     int64
			detail string
		)
		if err := rows.Scan(&f.Seq, &ms, &f.TaskID, &f.Type, &f.Owner, &f.Attempt, &f.Error, &detail); err != nil {
			return nil, err
		}

		f.Time = time.UnixMilli(ms)
		if detail != "" {
			f.Detail = jsontext.Value(detail)
		}

		out = append(out, f)
	}

	return out, rows.Err()
}

func cancelRequestedTx(ctx context.Context, tx *sql.Tx, id string) (bool, error) {
	var requested bool

	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM facts WHERE task_id = ? AND type = 'task.cancel-requested')`, id).
		Scan(&requested)

	return requested, err
}

func cancelRequestedReasonTx(ctx context.Context, tx *sql.Tx, id string) (string, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT detail FROM facts WHERE task_id = ? AND type = 'task.cancel-requested' ORDER BY seq ASC`, id)
	if err != nil {
		return "", err
	}

	defer func() { _ = rows.Close() }()

	reason := ""

	for rows.Next() {
		var detail string
		if err := rows.Scan(&detail); err != nil {
			return "", err
		}

		if detail == "" {
			continue
		}

		var parsed struct {
			Reason string `json:"reason"`
		}
		if err := json.Unmarshal(jsontext.Value(detail), &parsed); err != nil {
			continue
		}

		if parsed.Reason != "" {
			reason = parsed.Reason
		}
	}

	return reason, rows.Err()
}

func leaseErr(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id task.ID, _ string,
) error {
	var st string

	err := q.QueryRowContext(ctx, `SELECT status FROM tasks WHERE id = ?`, id.String()).Scan(&st)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return task.ErrNotFound
		}

		return err
	}

	return task.ErrLeaseNotHeld
}

func cooperativeCancelDetail(reason, after string) jsontext.Value {
	detail := map[string]string{"cooperative": "true"}
	if after != "" {
		detail["after"] = after
	}

	if reason != "" {
		detail["reason"] = reason
	}

	return mustJSON(detail)
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	s = strings.ReplaceAll(s, "_", `\_`)

	return s
}

func mustJSON(v any) jsontext.Value {
	b, err := json.Marshal(v)
	if err != nil {
		return jsontext.Value("{}")
	}

	return b
}

func boolInt(b bool) int {
	if b {
		return 1
	}

	return 0
}
