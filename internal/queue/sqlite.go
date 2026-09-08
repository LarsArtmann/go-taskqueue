package queue

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/task"
	_ "modernc.org/sqlite" // pure-Go SQLite driver (CGo-free)
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
	// projectExclusive: ClaimDue refuses to hand out a task whose project
	// already has another running task. See WithProjectExclusivity.
	projectExclusive bool
}

// StoreOption configures optional SQLiteStore behavior.
type StoreOption func(*storeOptions)

type storeOptions struct {
	projectExclusive bool
}

// WithProjectExclusivity turns on store-level per-project serialization:
// ClaimDue will not claim a task whose project already has another running
// task — across ALL pools and processes sharing the same database file, not
// just within one pool. This is the per-repo guarantee for agent pools: two
// agents never work the same repo simultaneously. Every pool sharing the DB
// must opt in; pools that do not opt in ignore the guard. Tasks with an
// empty project are exempt (they are not tied to a repo).
func WithProjectExclusivity() StoreOption {
	return func(o *storeOptions) { o.projectExclusive = true }
}

// OpenSQLite opens (creating if needed) the queue database at path.
func OpenSQLite(path string, opts ...StoreOption) (*SQLiteStore, error) {
	var o storeOptions
	for _, opt := range opts {
		opt(&o)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("queue: open sqlite: %w", err)
	}
	// Serialize writers: one connection makes every SELECT…UPDATE sequence
	// inside a transaction atomic without relying on BEGIN IMMEDIATE tricks.
	db.SetMaxOpenConns(1)
	s := &SQLiteStore{db: db, projectExclusive: o.projectExclusive}
	// Two processes opening a FRESH database race the schema writes: the
	// loser gets SQLITE_BUSY even with busy_timeout. The retry always
	// converges — IF NOT EXISTS migrations on an already-migrated DB are a
	// no-op — so a bounded backoff is the whole fix.
	var merr error

	for attempt := range 5 {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
		}

		merr = s.migrate(context.Background())
		if merr == nil {
			return s, nil
		}
	}

	_ = db.Close()

	return nil, merr
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
	completed_at   INTEGER,
	dedup_key      TEXT NOT NULL DEFAULT ''
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
	// Databases created before dedup_key existed need the column added
	// (CREATE TABLE IF NOT EXISTS cannot evolve an existing table). The
	// partial unique index is created here, after the column is guaranteed to
	// exist, so fresh and legacy databases take the same path.
	var dedupCol int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pragma_table_info('tasks') WHERE name = 'dedup_key'`).Scan(&dedupCol); err != nil {
		return fmt.Errorf("queue: migrate: check dedup_key: %w", err)
	}

	if dedupCol == 0 {
		if _, err := s.db.ExecContext(
			ctx,
			`ALTER TABLE tasks ADD COLUMN dedup_key TEXT NOT NULL DEFAULT ''`,
		); err != nil {
			return fmt.Errorf("queue: migrate: add dedup_key: %w", err)
		}
	}
	// Only tasks that opt into deduplication participate, so arbitrary tasks
	// without a key never collide.
	if _, err := s.db.ExecContext(ctx,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_dedup ON tasks(dedup_key) WHERE dedup_key != ''`); err != nil {
		return fmt.Errorf("queue: migrate: dedup index: %w", err)
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

// Enqueue persists a new task and records task.enqueued. When New.DedupKey
// is set and a task with that key already exists, the stored task is returned
// unchanged — no duplicate row, no duplicate fact (idempotent enqueue).
func (s *SQLiteStore) Enqueue(ctx context.Context, n task.New) (task.Task, error) {
	n = n.Normalize()
	if n.Type == "" {
		return task.Task{}, errors.New("queue: task type must not be empty")
	}

	if n.DedupKey != "" {
		if existing, found, err := s.getTaskByDedupKey(ctx, n.DedupKey); err != nil {
			return task.Task{}, fmt.Errorf("queue: enqueue dedup lookup: %w", err)
		} else if found {
			return existing, nil
		}
	}

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
	suppressed := false

	depsJSON, err := json.Marshal(t.Deps)
	if err != nil {
		return task.Task{}, fmt.Errorf("queue: marshal deps: %w", err)
	}

	payload := string(t.Payload) // empty string, never NULL

	err = s.withTx(ctx, func(tx *sql.Tx) error {
		if n.DedupKey != "" {
			// Re-check inside the transaction: a concurrent enqueuer may have
			// inserted the same key between our lookup and this write. The
			// unique partial index is the final arbiter.
			var existingID string

			err := tx.QueryRowContext(ctx, `SELECT id FROM tasks WHERE dedup_key = ?`, n.DedupKey).Scan(&existingID)
			if err == nil {
				t.ID = task.ID(existingID)
				suppressed = true

				return nil
			}

			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO tasks (id, project, type, payload, deps, priority, attempts, max_attempts,
			                    not_before, status, created_at, updated_at, dedup_key)
			 VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?, 'pending', ?, ?, ?)`,
			t.ID.String(), t.Project, t.Type, payload, string(depsJSON), t.Priority,
			t.MaxAttempts, ms(t.NotBefore), now.UnixMilli(), now.UnixMilli(), n.DedupKey); err != nil {
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

	if suppressed {
		return s.Get(ctx, t.ID)
	}

	return t, nil
}

// getTaskByDedupKey returns the stored task for a dedup key, if any.
func (s *SQLiteStore) getTaskByDedupKey(ctx context.Context, key string) (task.Task, bool, error) {
	var id string

	err := s.db.QueryRowContext(ctx, `SELECT id FROM tasks WHERE dedup_key = ?`, key).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return task.Task{}, false, nil
	}

	if err != nil {
		return task.Task{}, false, err
	}

	t, err := s.Get(ctx, task.ID(id))
	if err != nil {
		return task.Task{}, false, err
	}

	return t, true, nil
}

// ClaimDue atomically claims one due task for owner.
func (s *SQLiteStore) ClaimDue(ctx context.Context, owner string, lease time.Duration) (task.Task, error) {
	now := time.Now()

	var claimed task.Task

	err := s.withTx(ctx, func(tx *sql.Tx) error {
		// Candidate: pending-and-due OR running-with-expired-lease (crashed
		// worker reclaim), priority first, oldest first — and every
		// dependency completed (deps not met => not selectable). With
		// project exclusivity on, a project that already has a running task
		// yields nothing (except reclaiming that very task; empty projects
		// are exempt).
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
			ORDER BY t.priority DESC, t.created_at ASC, t.id ASC
			LIMIT 1`, now.UnixMilli(), now.UnixMilli(), boolInt(s.projectExclusive))

		var id, st, prevOwner string
		if err := row.Scan(&id, &st, &prevOwner); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNoTaskDue
			}

			return err
		}
		// Reclaiming an expired lease first records the release, so the
		// journal shows why the task moved between owners. A pending
		// cooperative cancel is finalized here instead: the task is never
		// re-executed after its cancel was requested.
		if st == "running" {
			requested, err := cancelRequestedTx(ctx, tx, id)
			if err != nil {
				return err
			}

			if requested {
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
					return ErrNoTaskDue // lost the race; another path finalized it
				}

				if err := s.appendFact(ctx, tx, journal.Fact{
					TaskID: id, Type: journal.Cancelled, Owner: owner,
					Detail: mustJSON(map[string]string{"cooperative": "true", "after": "lease-expiry"}),
				}); err != nil {
					return err
				}

				return ErrNoTaskDue // finalized; caller retries for another task
			}

			if err := s.appendFact(ctx, tx, journal.Fact{
				TaskID: id, Type: journal.Released, Owner: prevOwner,
			}); err != nil {
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
				Attempt: newAttempts, Error: errText,
			}); err != nil {
				return err
			}

			return s.appendFact(ctx, tx, journal.Fact{
				TaskID: id.String(), Type: journal.DeadLettered, Owner: owner, Attempt: newAttempts,
				Error: errText, Detail: json.RawMessage(`{"class":"exhausted"}`),
			})
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
			Attempt: newAttempts, Error: errText,
		})
	})
}

// FailPermanent dead-letters immediately: a permanent error means the
// identical retry would fail identically, so the remaining attempt budget is
// worthless (and, for agent tasks, expensive). The failing attempt is still
// counted. Facts: task.failed + task.dead-lettered with class "permanent".
func (s *SQLiteStore) FailPermanent(ctx context.Context, id task.ID, owner string, errText string) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		now := time.Now()

		var attempts int

		err := tx.QueryRowContext(ctx,
			`SELECT attempts FROM tasks WHERE id = ?`, id.String()).
			Scan(&attempts)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return task.ErrNotFound
			}

			return err
		}

		newAttempts := attempts + 1

		res, err := tx.ExecContext(ctx, `
				UPDATE tasks
				SET status = 'dead', attempts = ?, max_attempts = ?, last_error = ?, updated_at = ?,
				lease_owner = '', lease_expires = NULL
				WHERE id = ? AND status = 'running' AND lease_owner = ?`,
			newAttempts, newAttempts, errText, now.UnixMilli(), id.String(), owner)
		if err != nil {
			return err
		}

		if n, _ := res.RowsAffected(); n == 0 {
			return s.leaseErr(ctx, tx, id, owner)
		}

		if err := s.appendFact(ctx, tx, journal.Fact{
			TaskID: id.String(), Type: journal.Failed, Owner: owner,
			Attempt: newAttempts, Error: errText,
		}); err != nil {
			return err
		}

		return s.appendFact(ctx, tx, journal.Fact{
			TaskID: id.String(), Type: journal.DeadLettered, Owner: owner, Attempt: newAttempts,
			Error: errText, Detail: json.RawMessage(`{"class":"permanent"}`),
		})
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
			if err := tx.QueryRowContext(ctx, `SELECT status FROM tasks WHERE id = ?`, id.String()).
				Scan(&st); err != nil {
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
			if err := tx.QueryRowContext(ctx, `SELECT status FROM tasks WHERE id = ?`, id.String()).
				Scan(&st); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return task.ErrNotFound
				}

				return err
			}

			return fmt.Errorf("%w: %s -> pending", task.ErrInvalidTransition, st)
		}

		return s.appendFact(
			ctx,
			tx,
			journal.Fact{
				TaskID: id.String(),
				Type:   journal.Enqueued,
				Detail: mustJSON(map[string]string{"rescue": "true"}),
			},
		)
	})
}

// Get returns the current task record.
func (s *SQLiteStore) Get(ctx context.Context, id task.ID) (task.Task, error) {
	return s.loadTaskTx(ctx, s.db, id.String())
}

// List returns tasks matching the filter.
// listWhere builds the shared WHERE clause for List and CountTasks so the
// two can never disagree about what a filter matches.
func listWhere(f Filter) (string, []any) {
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

	if f.Query != "" {
		like := "%" + escapeLike(strings.ToLower(f.Query)) + "%"
		where = append(where, `(id LIKE ? ESCAPE '\' OR type LIKE ? ESCAPE '\' OR
			project LIKE ? ESCAPE '\' OR payload LIKE ? ESCAPE '\' OR
			lease_owner LIKE ? ESCAPE '\' OR last_error LIKE ? ESCAPE '\')`)
		args = append(args, like, like, like, like, like, like)
	}

	return strings.Join(where, " AND "), args
}

func (s *SQLiteStore) List(ctx context.Context, f Filter) ([]task.Task, error) {
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

	q := `SELECT id, project, type, payload, deps, priority, attempts, max_attempts,
	             not_before, status, lease_owner, lease_expires, last_error,
	             created_at, updated_at, completed_at
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
func (s *SQLiteStore) CountTasks(ctx context.Context, f Filter) (int, error) {
	where, args := listWhere(f)

	var n int

	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE `+where, args...).Scan(&n)

	return n, err
}

// Facts returns journal facts with Seq > after, ascending, bounded to the
// most recent limit when > 0. The seq primary key makes the cursor scan
// O(limit) regardless of journal size.
func (s *SQLiteStore) Facts(ctx context.Context, after int64, limit int) ([]journal.Fact, error) {
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
func (s *SQLiteStore) LastFacts(ctx context.Context, limit int) ([]journal.Fact, error) {
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

// HeadSeq returns the current highest fact Seq (0 when empty).
func (s *SQLiteStore) HeadSeq(ctx context.Context) (int64, error) {
	var seq int64

	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq), 0) FROM facts`).Scan(&seq)

	return seq, err
}

// FactsForTask returns one task's facts in Seq order, bounded to the most
// recent limit when > 0. Served by idx_facts_task (task_id, seq).
func (s *SQLiteStore) FactsForTask(ctx context.Context, id string, limit int) ([]journal.Fact, error) {
	query := `
		SELECT seq, time, task_id, type, owner, attempt, error, detail
		FROM facts WHERE task_id = ? ORDER BY seq ASC`
	args := []any{id}
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

// CountFacts counts facts of one type recorded at or after since.
func (s *SQLiteStore) CountFacts(ctx context.Context, ftype journal.FactType, since time.Time) (int64, error) {
	var n int64

	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM facts WHERE type = ? AND time >= ?`,
		ftype, since.UnixMilli()).Scan(&n)

	return n, err
}

// escapeLike escapes LIKE wildcards so a user query containing %, _ or \
// matches literally. Pair with ESCAPE '\' in the SQL.
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	s = strings.ReplaceAll(s, "_", `\_`)

	return s
}

// StatusCounts counts tasks per status in one GROUP BY.
func (s *SQLiteStore) StatusCounts(ctx context.Context) (map[task.Status]int, error) {
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
func (s *SQLiteStore) ProjectCounts(ctx context.Context) (map[string]map[task.Status]int, error) {
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
}, id string,
) (task.Task, error) {
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
}, id task.ID, owner string,
) error {
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

func boolInt(b bool) int {
	if b {
		return 1
	}

	return 0
}

// Requeue returns a claimed task to Pending without counting an attempt:
// the executor refused to start (preflight), so the task itself is fine and
// the environment is expected to become ready later. Claimable again after
// delay. Fact: task.requeued.
func (s *SQLiteStore) Requeue(
	ctx context.Context,
	id task.ID,
	owner string,
	errText string,
	delay time.Duration,
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
			return s.leaseErr(ctx, tx, id, owner)
		}

		return s.appendFact(ctx, tx, journal.Fact{
			TaskID: id.String(), Type: journal.Requeued, Owner: owner, Error: errText,
		})
	})
}
