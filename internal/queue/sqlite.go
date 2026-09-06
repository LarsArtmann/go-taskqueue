package queue

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (CGo-free)

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// SQLiteStore is the embedded, durable Store. One queue per database file.
//
// Concurrency model: a single serialized write connection (MaxOpenConns(1))
// plus WAL journal mode. All task mutations and their journal facts happen in
// one transaction, so the journal can never disagree with the task table.
// Multiple processes may open the same file; busy_timeout + WAL serialize
// cross-process writers.
type SQLiteStore struct {
	db *sql.DB
}

// OpenSQLite opens (creating if needed) the queue database at path.
func OpenSQLite(path string) (*SQLiteStore, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("queue: open sqlite: %w", err)
	}
	// Serialize writers: one connection makes every SELECT…UPDATE sequence
	// inside a transaction atomic without relying on BEGIN IMMEDIATE tricks.
	db.SetMaxOpenConns(1)
	s := &SQLiteStore{db: db}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS tasks (
	id             TEXT PRIMARY KEY,
	project        TEXT NOT NULL DEFAULT '',
	type           TEXT NOT NULL,
	payload        TEXT NOT NULL DEFAULT '',
	deps           TEXT NOT NULL DEFAULT '[]', -- JSON array of task IDs
	priority       INTEGER NOT NULL DEFAULT 0,
	attempts       INTEGER NOT NULL DEFAULT 0,
	max_attempts   INTEGER NOT NULL DEFAULT 3,
	not_before     INTEGER NOT NULL DEFAULT 0, -- unix millis
	status         TEXT NOT NULL DEFAULT 'pending',
	lease_owner    TEXT NOT NULL DEFAULT '',
	lease_expires  INTEGER,                    -- unix millis, NULL when unleased
	last_error     TEXT NOT NULL DEFAULT '',
	created_at     INTEGER NOT NULL,
	updated_at     INTEGER NOT NULL,
	completed_at   INTEGER
);
CREATE INDEX IF NOT EXISTS idx_tasks_status_due ON tasks(status, not_before);
CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project);

CREATE TABLE IF NOT EXISTS deps (
	task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
	dep_id  TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
	PRIMARY KEY (task_id, dep_id)
);
CREATE INDEX IF NOT EXISTS idx_deps_dep ON deps(dep_id);

CREATE TABLE IF NOT EXISTS facts (
	seq      INTEGER PRIMARY KEY AUTOINCREMENT,
	time     INTEGER NOT NULL, -- unix millis
	task_id  TEXT NOT NULL,
	type     TEXT NOT NULL,
	owner    TEXT NOT NULL DEFAULT '',
	attempt  INTEGER NOT NULL DEFAULT 0,
	error    TEXT NOT NULL DEFAULT '',
	detail   TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_facts_task ON facts(task_id, seq);
`

func (s *SQLiteStore) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("queue: migrate: %w", err)
	}
	return nil
}

// Close releases the database connection.
func (s *SQLiteStore) Close() error { return s.db.Close() }

func (s *SQLiteStore) appendFact(ctx context.Context, tx *sql.Tx, f journal.Fact) error {
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

// Enqueue persists a new task and records task.enqueued.
func (s *SQLiteStore) Enqueue(ctx context.Context, n task.New) (task.Task, error) {
	n = n.Normalize()
	now := time.Now()
	t := task.Task{
		ID:          task.NewID(),
		Project:     n.Project,
		Type:        n.Type,
		Payload:     n.Payload,
		Deps:        n.Deps,
		Priority:    n.Priority,
		MaxAttempts: n.MaxAttempts,
		NotBefore:   n.NotBefore,
		Status:      task.Pending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if t.Type == "" {
		return task.Task{}, errors.New("queue: task type must not be empty")
	}
	depsJSON, err := json.Marshal(t.Deps)
	if err != nil {
		return task.Task{}, fmt.Errorf("queue: marshal deps: %w", err)
	}
	payload := string(t.Payload) // empty string, never NULL

	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO tasks (id, project, type, payload, deps, priority, attempts, max_attempts,
			                    not_before, status, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?, 'pending', ?, ?)`,
			t.ID.String(), t.Project, t.Type, payload, string(depsJSON), t.Priority,
			t.MaxAttempts, ms(t.NotBefore), now.UnixMilli(), now.UnixMilli()); err != nil {
			return err
		}
		for _, d := range t.Deps {
			if _, err := tx.ExecContext(ctx, `INSERT INTO deps (task_id, dep_id) VALUES (?, ?)`,
				t.ID.String(), d.String()); err != nil {
				return err
			}
		}
		return s.appendFact(ctx, tx, journal.Fact{
			TaskID: t.ID.String(), Type: journal.Enqueued, Attempt: 0,
			Detail: mustJSON(map[string]any{"project": t.Project, "type": t.Type}),
		})
	})
	if err != nil {
		return task.Task{}, fmt.Errorf("queue: enqueue: %w", err)
	}
	return t, nil
}

// ClaimDue atomically claims one due task for owner.
func (s *SQLiteStore) ClaimDue(ctx context.Context, owner string, lease time.Duration) (task.Task, error) {
	now := time.Now()
	var claimed task.Task
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		// Candidate: pending-and-due OR running-with-expired-lease (crashed
		// worker reclaim), priority first, oldest first — and every
		// dependency completed (deps not met => not selectable).
		row := tx.QueryRowContext(ctx, `
			SELECT t.id, t.status, t.lease_owner FROM tasks t
			WHERE ((t.status = 'pending' AND t.not_before <= ?)
			    OR (t.status = 'running' AND t.lease_expires IS NOT NULL AND t.lease_expires <= ?))
			  AND NOT EXISTS (
			    SELECT 1 FROM deps d JOIN tasks dt ON dt.id = d.dep_id
			    WHERE d.task_id = t.id AND dt.status != 'completed'
			  )
			ORDER BY t.priority DESC, t.created_at ASC, t.id ASC
			LIMIT 1`, now.UnixMilli(), now.UnixMilli())
		var id, st, prevOwner string
		if err := row.Scan(&id, &st, &prevOwner); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNoTaskDue
			}
			return err
		}
		// Reclaiming an expired lease first records the release, so the
		// journal shows why the task moved between owners.
		if st == "running" {
			if err := s.appendFact(ctx, tx, journal.Fact{
				TaskID: id, Type: journal.Released, Owner: prevOwner}); err != nil {
				return err
			}
		}
		res, err := tx.ExecContext(ctx, `
			UPDATE tasks
			SET status = 'running', lease_owner = ?, lease_expires = ?, updated_at = ?
			WHERE id = ? AND (
			    (status = 'pending' AND not_before <= ?)
			    OR (status = 'running' AND lease_expires IS NOT NULL AND lease_expires <= ?))`,
			owner, now.Add(lease).UnixMilli(), now.UnixMilli(), id, now.UnixMilli(), now.UnixMilli())
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNoTaskDue // lost the race (multi-process); caller retries
		}
		if err := s.appendFact(ctx, tx, journal.Fact{TaskID: id, Type: journal.Claimed, Owner: owner}); err != nil {
			return err
		}
		claimed, err = s.loadTaskTx(ctx, tx, id)
		return err
	})
	if err != nil {
		return task.Task{}, err
	}
	return claimed, nil
}

// Complete marks a Running task Completed.
func (s *SQLiteStore) Complete(ctx context.Context, id task.ID, owner string, result json.RawMessage) error {
	now := time.Now()
	return s.withTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE tasks
			SET status = 'completed', completed_at = ?, updated_at = ?,
			    lease_owner = '', lease_expires = NULL, last_error = ''
			WHERE id = ? AND status = 'running' AND lease_owner = ? AND lease_expires > ?`,
			now.UnixMilli(), now.UnixMilli(), id.String(), owner, now.UnixMilli())
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return s.leaseErr(ctx, tx, id, owner)
		}
		return s.appendFact(ctx, tx, journal.Fact{
			TaskID: id.String(), Type: journal.Completed, Owner: owner,
			Detail: maybeJSON(result),
		})
	})
}

// Fail records a failed attempt: retry with backoff or dead-letter.
func (s *SQLiteStore) Fail(ctx context.Context, id task.ID, owner string, errText string, backoff time.Duration) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		now := time.Now() // captured inside the tx: backoff counts from commit, not from call
		var attempts, maxAttempts int
		// Safe without FOR UPDATE: single serialized writer connection.
		err := tx.QueryRowContext(ctx,
			`SELECT attempts, max_attempts FROM tasks WHERE id = ?`, id.String()).
			Scan(&attempts, &maxAttempts)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return task.ErrNotFound
			}
			return err
		}
		newAttempts := attempts + 1
		if newAttempts >= maxAttempts {
			_, err = tx.ExecContext(ctx, `
				UPDATE tasks
				SET status = 'dead', attempts = ?, last_error = ?, updated_at = ?,
				    lease_owner = '', lease_expires = NULL
				WHERE id = ? AND status = 'running' AND lease_owner = ?`,
				newAttempts, errText, now.UnixMilli(), id.String(), owner)
			if err != nil {
				return err
			}
			if err := s.appendFact(ctx, tx, journal.Fact{
				TaskID: id.String(), Type: journal.Failed, Owner: owner,
				Attempt: newAttempts, Error: errText}); err != nil {
				return err
			}
			return s.appendFact(ctx, tx, journal.Fact{
				TaskID: id.String(), Type: journal.DeadLettered, Owner: owner, Attempt: newAttempts})
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE tasks
			SET status = 'pending', attempts = ?, last_error = ?, not_before = ?,
			    updated_at = ?, lease_owner = '', lease_expires = NULL
			WHERE id = ? AND status = 'running' AND lease_owner = ?`,
			newAttempts, errText, now.Add(backoff).UnixMilli(), now.UnixMilli(), id.String(), owner)
		if err != nil {
			return err
		}
		return s.appendFact(ctx, tx, journal.Fact{
			TaskID: id.String(), Type: journal.Failed, Owner: owner,
			Attempt: newAttempts, Error: errText})
	})
}

// Heartbeat extends the lease of a Running task held by owner.
func (s *SQLiteStore) Heartbeat(ctx context.Context, id task.ID, owner string, extend time.Duration) error {
	now := time.Now()
	res, err := s.db.ExecContext(ctx, `
		UPDATE tasks SET lease_expires = ?, updated_at = ?
		WHERE id = ? AND status = 'running' AND lease_owner = ? AND lease_expires > ?`,
		now.Add(extend).UnixMilli(), now.UnixMilli(), id.String(), owner, now.UnixMilli())
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return task.ErrLeaseNotHeld
	}
	return nil
}

// Cancel withdraws a Pending task.
func (s *SQLiteStore) Cancel(ctx context.Context, id task.ID) error {
	now := time.Now()
	return s.withTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE tasks SET status = 'cancelled', updated_at = ?, lease_owner = '', lease_expires = NULL
			WHERE id = ? AND status = 'pending'`, now.UnixMilli(), id.String())
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			var st string
			if err := tx.QueryRowContext(ctx, `SELECT status FROM tasks WHERE id = ?`, id.String()).Scan(&st); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return task.ErrNotFound
				}
				return err
			}
			return fmt.Errorf("%w: %s -> cancelled", task.ErrInvalidTransition, st)
		}
		return s.appendFact(ctx, tx, journal.Fact{TaskID: id.String(), Type: journal.Cancelled})
	})
}

// RescueDead re-queues a Dead task with a fresh attempt budget (DLQ rescue).
func (s *SQLiteStore) RescueDead(ctx context.Context, id task.ID, maxAttempts int) error {
	if maxAttempts <= 0 {
		maxAttempts = task.DefaultMaxAttempts
	}
	now := time.Now()
	return s.withTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE tasks
			SET status = 'pending', attempts = 0, max_attempts = ?, not_before = 0,
			    updated_at = ?, lease_owner = '', lease_expires = NULL, last_error = ''
			WHERE id = ? AND status = 'dead'`, maxAttempts, now.UnixMilli(), id.String())
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			var st string
			if err := tx.QueryRowContext(ctx, `SELECT status FROM tasks WHERE id = ?`, id.String()).Scan(&st); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return task.ErrNotFound
				}
				return err
			}
			return fmt.Errorf("%w: %s -> pending", task.ErrInvalidTransition, st)
		}
		return s.appendFact(ctx, tx, journal.Fact{TaskID: id.String(), Type: journal.Enqueued, Detail: mustJSON(map[string]string{"rescue": "true"})})
	})
}

// Get returns the current task record.
func (s *SQLiteStore) Get(ctx context.Context, id task.ID) (task.Task, error) {
	return s.loadTaskTx(ctx, s.db, id.String())
}

// List returns tasks matching the filter.
func (s *SQLiteStore) List(ctx context.Context, f Filter) ([]task.Task, error) {
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
	q := `SELECT id, project, type, payload, deps, priority, attempts, max_attempts,
	             not_before, status, lease_owner, lease_expires, last_error,
	             created_at, updated_at, completed_at
	      FROM tasks WHERE ` + strings.Join(where, " AND ") + `
	      ORDER BY priority DESC, created_at ASC`
	if f.Limit > 0 {
		q += " LIMIT ?"
		args = append(args, f.Limit)
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

// Facts returns journal facts with Seq > after.
func (s *SQLiteStore) Facts(ctx context.Context, after int64) ([]journal.Fact, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT seq, time, task_id, type, owner, attempt, error, detail
		FROM facts WHERE seq > ? ORDER BY seq ASC`, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []journal.Fact
	for rows.Next() {
		var f journal.Fact
		var ms int64
		var detail string
		if err := rows.Scan(&f.Seq, &ms, &f.TaskID, &f.Type, &f.Owner, &f.Attempt, &f.Error, &detail); err != nil {
			return nil, err
		}
		f.Time = time.UnixMilli(ms)
		if detail != "" {
			f.Detail = json.RawMessage(detail)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// --- internals ---

func (s *SQLiteStore) withTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
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

func (s *SQLiteStore) loadTaskTx(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id string) (task.Task, error) {
	row := q.QueryRowContext(ctx, `
		SELECT id, project, type, payload, deps, priority, attempts, max_attempts,
		       not_before, status, lease_owner, lease_expires, last_error,
		       created_at, updated_at, completed_at
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
		&createdAtMS, &updatedAtMS, &completedAt); err != nil {
		return task.Task{}, err
	}
	t.ID = task.ID(id)
	t.Status = task.Status(status)
	t.Payload = json.RawMessage(payload)
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
			return task.Task{}, fmt.Errorf("queue: unmarshal deps for %s: %w", id, err)
		}
	}
	return t, nil
}

func (s *SQLiteStore) leaseErr(ctx context.Context, q interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, id task.ID, owner string) error {
	var st string
	err := q.QueryRowContext(ctx, `SELECT status FROM tasks WHERE id = ?`, id.String()).Scan(&st)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return task.ErrNotFound
		}
		return err
	}
	_ = owner
	return task.ErrLeaseNotHeld
}

func ms(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}
	return b
}

func maybeJSON(r json.RawMessage) json.RawMessage {
	if len(r) == 0 {
		return nil
	}
	return r
}
