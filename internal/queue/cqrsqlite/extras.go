package cqrsqlite

import (
	"context"
	"database/sql"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// The tq same-DB extension (ADR-0019 S1): everything below reads and
// writes the engine's tables with tq's exact SQL and adds ONE companion
// table (priority_scores). Facts appended here are schema-compatible with
// the engine's facts table, so they read back through the engine's own
// Facts reads.

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
		return errors.New("cqrsqlite: migrate companion: " + err.Error())
	}

	return nil
}

// AppendFact records a NON-task journal fact (session.opened /
// session.closed). Task facts are never written through it — every task
// mutation appends its fact inside its own operation's transaction.
func (s *Store) AppendFact(ctx context.Context, f journal.Fact) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		return s.appendFact(ctx, tx, f)
	})
}

func (s *Store) appendFact(ctx context.Context, tx *sql.Tx, f journal.Fact) error {
	if f.Time.IsZero() {
		f.Time = time.Now()
	}

	detail := string(f.Detail) // empty string, never NULL
	_, err := tx.ExecContext(ctx,
		`INSERT INTO facts (time, task_id, type, owner, attempt, error, detail)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		f.Time.UnixMilli(), f.TaskID, string(f.Type), f.Owner, f.Attempt, f.Error, detail)

	return err
}

// RecordAnswer records an owner's decision for a parked task's question
// and — when the task is PENDING — injects the answer into the task's
// JSON-object payload under the "answered" key and clears NotBefore, all
// in the SAME transaction. Idempotent per question ref.
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
// ref -> text (QuestionAskedDetail: the asked text; QuestionAnsweredDetail:
// the answer).
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

		switch ftype { //nolint:exhaustive // only the two question fact types carry ref-keyed detail
		case journal.QuestionAsked:
			var parsed queue.QuestionAskedDetail
			if err := json.Unmarshal(jsontext.Value(detail), &parsed); err != nil {
				continue // unparseable detail: never silently dedup on it
			}

			out[parsed.Ref] = parsed.Question
		case journal.QuestionAnswered:
			var parsed queue.QuestionAnsweredDetail
			if err := json.Unmarshal(jsontext.Value(detail), &parsed); err != nil {
				continue
			}

			out[parsed.Ref] = parsed.Answer
		default:
			// Not a question fact; nothing to extract.
		}
	}

	return out, rows.Err()
}

// mergeAnsweredPayload injects one answered question into a JSON-object
// payload's "answered" array (creating it when absent). Raw (non-object)
// payloads report ok=false.
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
		// Unparseable payload: the fact appended by the caller still
		// records the ruling.
		return jsontext.Value(payload), false, nil //nolint:nilerr // fact-only path
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

// --- tq List/CountTasks: the tq filter surface (severity order, sort
// allowlist) over the engine's tasks table ---

// List returns tasks matching the filter.
func (s *Store) List(ctx context.Context, f queue.Filter) ([]task.Task, error) {
	where, args := listWhere(f)

	order := `ORDER BY priority DESC, created_at ASC`
	if f.SeverityOrder {
		// Display severity: dead, running, pending, cancelled, completed;
		// newest first within a status (the task table's rank order).
		order = `ORDER BY CASE status
			WHEN 'dead' THEN 0
			WHEN 'running' THEN 1
			WHEN 'pending' THEN 2
			WHEN 'cancelled' THEN 3
			ELSE 4 END, created_at DESC`
	}

	// Allowlisted column sorts — f.Sort is never interpolated into SQL.
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

type scanner interface{ Scan(dest ...any) error }

func scanTask(rows *sql.Rows) (task.Task, error) {
	var (
		t           task.Task
		payload     string
		depsJSON    string
		notBeforeMs int64
		status      string
		createdMs   int64
		updatedMs   int64
		completedMs sql.NullInt64
		leaseExpMs  sql.NullInt64
	)

	if err := rows.Scan(
		&t.ID, &t.Project, &t.Type, &payload, &depsJSON, &t.Priority, &t.Attempts,
		&t.MaxAttempts, &notBeforeMs, &status, &t.LeaseOwner, &leaseExpMs, &t.LastError,
		&createdMs, &updatedMs, &completedMs, &t.DedupKey,
	); err != nil {
		return task.Task{}, err
	}

	t.Payload = jsontext.Value(payload)
	t.Status = task.Status(status)
	t.NotBefore = time.UnixMilli(notBeforeMs)
	t.CreatedAt = time.UnixMilli(createdMs)
	t.UpdatedAt = time.UnixMilli(updatedMs)

	if err := json.Unmarshal(jsontext.Value(depsJSON), &t.Deps); err != nil {
		return task.Task{}, err
	}

	if leaseExpMs.Valid {
		lm := time.UnixMilli(leaseExpMs.Int64)
		t.LeaseExpires = &lm
	}

	if completedMs.Valid {
		cm := time.UnixMilli(completedMs.Int64)
		t.CompletedAt = &cm
	}

	return t, nil
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	s = strings.ReplaceAll(s, "_", `\_`)

	return s
}

// --- journal reads the upstream contract lacks ---

// LastFacts returns the most recent limit facts in ascending Seq order.
func (s *Store) LastFacts(ctx context.Context, limit int) ([]journal.Fact, error) {
	if limit <= 0 {
		return s.Facts(ctx, 0, 0)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT seq, time, task_id, type, owner, attempt, error, detail
		FROM facts ORDER BY seq DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	facts, err := scanFacts(rows)
	if err != nil {
		return nil, err
	}

	for i, j := 0, len(facts)-1; i < j; i, j = i+1, j-1 {
		facts[i], facts[j] = facts[j], facts[i]
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

// FactsSince returns facts of one type recorded at or after since, in Seq
// order.
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

// --- watermarks admin reads + priority_scores cache (companion) ---

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

// SetWatermark overwrites a consumer cursor unconditionally (ops rescue
// hatch); unlike SaveWatermark it MAY move the cursor backwards.
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

// DeletePriorityScores removes the cached verdicts for the given item
// keys and returns how many rows went away.
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

// --- shared internals ---

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

func mustJSON(v any) jsontext.Value {
	b, err := json.Marshal(v)
	if err != nil {
		panic("cqrsqlite: marshal fact detail: " + err.Error()) //nolint:exhaustruct // internal invariant: fact details always marshal
	}

	return b
}

// ArchiveFactsBefore is NOT implemented in the S1 spike: the fact archive
// (facts_archive / journal_meta) has no upstream counterpart.
func (s *Store) ArchiveFactsBefore(ctx context.Context, cutoff int64) (int64, error) {
	return 0, errors.New("cqrsqlite: fact archive not implemented in the S1 spike")
}

// ArchiveSummary is NOT implemented in the S1 spike: see ArchiveFactsBefore.
func (s *Store) ArchiveSummary(ctx context.Context) (ArchiveStats, error) {
	return ArchiveStats{}, errors.New("cqrsqlite: fact archive not implemented in the S1 spike")
}

// ArchiveStats mirrors the tq sqlite store's archive summary shape.
type ArchiveStats struct {
	Archived int64
	Hot      int64
}
