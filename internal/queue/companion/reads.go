package companion

import (
	"context"
	"database/sql"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// Get returns the current task record.
func Get(ctx context.Context, r Runner, id task.ID) (task.Task, error) {
	return loadTask(ctx, r, id.String())
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

		// payload is BLOB under the upstream engine (tq stores TEXT), so
		// the substring pushdown reads it through CAST — LIKE never
		// matches a BLOB operand against a TEXT pattern.
		where = append(where, `(id LIKE ? ESCAPE '\' OR type LIKE ? ESCAPE '\' OR
			project LIKE ? ESCAPE '\' OR CAST(payload AS TEXT) LIKE ? ESCAPE '\' OR
			lease_owner LIKE ? ESCAPE '\' OR last_error LIKE ? ESCAPE '\')`)
		args = append(args, like, like, like, like, like, like)
	}

	return strings.Join(where, " AND "), args
}

// List returns tasks matching the filter.
func List(ctx context.Context, r Runner, f queue.Filter) ([]task.Task, error) {
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

	if f.Limit > 0 {
		q += " LIMIT ?"

		args = append(args, f.Limit)
	} else if f.Offset > 0 {
		// Cross-dialect "no limit": sqlite's LIMIT -1 is invalid postgres
		// (LIMIT must not be negative), so the offset-only page binds the
		// biggest int64 instead — an unreachable row count either way.
		q += " LIMIT 9223372036854775807"
	}

	if f.Offset > 0 {
		q += " OFFSET ?"

		args = append(args, f.Offset)
	}

	rows, err := r.QueryContext(ctx, q, args...)
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
func CountTasks(ctx context.Context, r Runner, f queue.Filter) (int, error) {
	where, args := listWhere(f)

	var n int

	err := r.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE `+where, args...).Scan(&n)

	return n, err
}

// Facts returns journal facts with Seq > after, ascending, bounded to the
// most recent limit when > 0.
func Facts(ctx context.Context, r Runner, after int64, limit int) ([]journal.Fact, error) {
	query := `
		SELECT seq, time, task_id, type, owner, attempt, error, detail
		FROM facts WHERE seq > ? ORDER BY seq ASC`
	args := []any{after}

	if limit > 0 {
		query += ` LIMIT ?`

		args = append(args, limit)
	}

	rows, err := r.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanFacts(rows)
}

// LastFacts returns the most recent limit facts in ascending Seq order.
func LastFacts(ctx context.Context, r Runner, limit int) ([]journal.Fact, error) {
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

	rows, err := r.QueryContext(ctx, query, args...)
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
func HeadSeq(ctx context.Context, r Runner) (int64, error) {
	var seq int64

	err := r.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq), 0) FROM facts`).Scan(&seq)

	return seq, err
}

// FactsForTask returns one task's facts in Seq order, bounded to the most
// recent limit when > 0.
func FactsForTask(ctx context.Context, r Runner, id string, limit int) ([]journal.Fact, error) {
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

	rows, err := r.QueryContext(ctx, query, args...)
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
func CountFacts(ctx context.Context, r Runner, ftype journal.FactType, since time.Time) (int64, error) {
	var n int64

	err := r.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM facts WHERE type = ? AND time >= ?`,
		ftype, since.UnixMilli()).Scan(&n)

	return n, err
}

// FactsSince returns facts of one type recorded at or after since.
func FactsSince(
	ctx context.Context,
	r Runner,
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

	rows, err := r.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanFacts(rows)
}

// Watermark returns the persisted read cursor for a journal consumer.
func Watermark(ctx context.Context, r Runner, consumer string) (int64, bool, error) {
	var seq int64

	err := r.QueryRowContext(ctx,
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
func SaveWatermark(ctx context.Context, r Runner, consumer string, seq int64) error {
	_, err := r.ExecContext(ctx, `
		INSERT INTO watermarks (consumer, seq, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(consumer) DO UPDATE SET
			seq = excluded.seq,
			updated_at = excluded.updated_at
		WHERE watermarks.seq < excluded.seq`,
		consumer, seq, time.Now().UnixMilli())

	return err
}

// ListWatermarks returns every consumer cursor, by consumer name.
func ListWatermarks(ctx context.Context, r Runner) ([]queue.WatermarkEntry, error) {
	rows, err := r.QueryContext(ctx,
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
func SetWatermark(ctx context.Context, r Runner, consumer string, seq int64) error {
	_, err := r.ExecContext(ctx, `
		INSERT INTO watermarks (consumer, seq, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(consumer) DO UPDATE SET
			seq = excluded.seq,
			updated_at = excluded.updated_at`,
		consumer, seq, time.Now().UnixMilli())

	return err
}

// SavePriorityScore upserts one cached item score (ADR-0015 score cache).
func SavePriorityScore(ctx context.Context, r Runner, score queue.PriorityScore) error {
	_, err := r.ExecContext(ctx, `
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
func PriorityScore(ctx context.Context, r Runner, itemKey string) (queue.PriorityScore, bool, error) {
	var score queue.PriorityScore

	err := r.QueryRowContext(ctx, `
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
func PriorityScores(ctx context.Context, r Runner) ([]queue.PriorityScore, error) {
	rows, err := r.QueryContext(ctx, `
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
func DeletePriorityScores(ctx context.Context, r Runner, itemKeys []string) (int64, error) {
	if len(itemKeys) == 0 {
		return 0, nil
	}

	args := make([]any, 0, len(itemKeys))
	placeholders := strings.Repeat("?,", len(itemKeys))
	placeholders = placeholders[:len(placeholders)-1]

	for _, key := range itemKeys {
		args = append(args, key)
	}

	res, err := r.ExecContext(ctx,
		`DELETE FROM priority_scores WHERE item_key IN (`+placeholders+`)`, args...)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}

// StatusCounts counts tasks per status in one GROUP BY.
func StatusCounts(ctx context.Context, r Runner) (map[task.Status]int, error) {
	rows, err := r.QueryContext(ctx, `SELECT status, COUNT(*) FROM tasks GROUP BY status`)
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
func ProjectCounts(ctx context.Context, r Runner) (map[string]map[task.Status]int, error) {
	rows, err := r.QueryContext(ctx,
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

func loadTask(ctx context.Context, r Runner, id string) (task.Task, error) {
	row := r.QueryRowContext(ctx, `
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
			return task.Task{}, fmt.Errorf("companion: unmarshal deps for %s: %w", id, err)
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

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	s = strings.ReplaceAll(s, "_", `\_`)

	return s
}
