package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// Store is the networked Store twin of sqlite.Store (ADR-0007): the
// same facts-first semantics over PostgreSQL, for deployments where many
// producers/workers share a queue across machines. Claims use
// SELECT ... FOR UPDATE SKIP LOCKED instead of SQLite's single serialized
// writer: competing workers lock disjoint rows instead of queueing behind
// one connection.
//
// Storage mapping mirrors SQLite exactly (unix-milli BIGINT timestamps,
// deps table, partial unique dedup index) so the projections and the
// journal remain byte-compatible across backends.
type Store struct {
	pool             *pgxpool.Pool
	projectExclusive bool
}

// Store implements the queue contract at compile time; the conformance
// suite pins the semantics.
var _ queue.Store = (*Store)(nil)

const postgresSchema = `
CREATE TABLE IF NOT EXISTS tasks (
	id            TEXT PRIMARY KEY,
	project       TEXT NOT NULL DEFAULT '',
	type          TEXT NOT NULL,
	payload       TEXT NOT NULL DEFAULT '',
	deps          TEXT NOT NULL DEFAULT '[]',
	priority      INTEGER NOT NULL DEFAULT 0,
	attempts      INTEGER NOT NULL DEFAULT 0,
	max_attempts  INTEGER NOT NULL DEFAULT 3,
	not_before    BIGINT NOT NULL DEFAULT 0,
	status        TEXT NOT NULL DEFAULT 'pending',
	lease_owner   TEXT NOT NULL DEFAULT '',
	lease_expires BIGINT,
	last_error    TEXT NOT NULL DEFAULT '',
	created_at    BIGINT NOT NULL,
	updated_at    BIGINT NOT NULL,
	completed_at  BIGINT,
	dedup_key     TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_tasks_status_due ON tasks(status, not_before);
CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_dedup ON tasks(dedup_key) WHERE dedup_key != '';

CREATE TABLE IF NOT EXISTS deps (
	task_id TEXT NOT NULL,
	dep_id  TEXT NOT NULL,
	PRIMARY KEY (task_id, dep_id)
);
CREATE INDEX IF NOT EXISTS idx_deps_dep ON deps(dep_id);

CREATE TABLE IF NOT EXISTS facts (
	seq      BIGSERIAL PRIMARY KEY,
	time     BIGINT NOT NULL,
	task_id  TEXT NOT NULL,
	type     TEXT NOT NULL,
	owner    TEXT NOT NULL DEFAULT '',
	attempt  INTEGER NOT NULL DEFAULT 0,
	error    TEXT NOT NULL DEFAULT '',
	detail   TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_facts_task ON facts(task_id, seq);

CREATE TABLE IF NOT EXISTS watermarks (
	consumer   TEXT PRIMARY KEY,
	seq        BIGINT NOT NULL,
	updated_at BIGINT NOT NULL
);
`

// Open connects to dsn (e.g. "postgres://user:pass@host:5432/db"),
// applies the schema, and returns a ready store. maxConns bounds the pool
// (0 = pgx default).
func Open(ctx context.Context, dsn string, maxConns int32) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("queue: parse dsn: %w", err)
	}

	if maxConns > 0 {
		cfg.MaxConns = maxConns
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("queue: connect postgres: %w", err)
	}

	if _, err := pool.Exec(ctx, postgresSchema); err != nil {
		pool.Close()

		return nil, fmt.Errorf("queue: postgres migrate: %w", err)
	}

	return &Store{pool: pool}, nil
}

// Close releases the pool.
func (s *Store) Close() error {
	s.pool.Close()

	return nil
}

// withTx runs fn in one transaction; ANY error rolls back (same contract
// as sqlite.Store.withTx).
func (s *Store) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}

	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)

		return err
	}

	return tx.Commit(ctx)
}

func (s *Store) appendFact(ctx context.Context, tx pgx.Tx, f journal.Fact) error {
	if f.Time.IsZero() {
		f.Time = time.Now()
	}

	_, err := tx.Exec(ctx,
		`INSERT INTO facts (time, task_id, type, owner, attempt, error, detail)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		f.Time.UnixMilli(), f.TaskID, string(f.Type), f.Owner, f.Attempt, f.Error, string(f.Detail))

	return err
}

// scanTask reads one task row into the record (deps rehydrate from the
// deps table, like SQLite).
func scanPGTask(row pgx.Row) (task.Task, error) {
	var (
		t            task.Task
		id           string
		depsJSON     string
		payload      string
		leaseExpires *int64
		completedAt  *int64
	)

	var notBefore, createdAt, updatedAt int64

	err := row.Scan(&id, &t.Project, &t.Type, &payload, &depsJSON, &t.Priority,
		&t.Attempts, &t.MaxAttempts, &notBefore, &t.Status, &t.LeaseOwner,
		&leaseExpires, &t.LastError, &createdAt, &updatedAt, &completedAt)
	if err != nil {
		return task.Task{}, err
	}

	t.ID = task.ID(id)
	t.Payload = json.RawMessage(payload)
	t.NotBefore = time.UnixMilli(notBefore)
	t.CreatedAt = time.UnixMilli(createdAt)
	t.UpdatedAt = time.UnixMilli(updatedAt)

	if err := json.Unmarshal([]byte(depsJSON), &t.Deps); err != nil && depsJSON != "" && depsJSON != "[]" {
		return task.Task{}, fmt.Errorf("queue: decode deps: %w", err)
	}

	if leaseExpires != nil {
		le := time.UnixMilli(*leaseExpires)
		t.LeaseExpires = &le
	}

	if completedAt != nil {
		ca := time.UnixMilli(*completedAt)
		t.CompletedAt = &ca
	}

	return t, nil
}

const taskColumns = `id, project, type, payload, deps, priority, attempts, max_attempts,
                     not_before, status, lease_owner, lease_expires, last_error,
                     created_at, updated_at, completed_at`

func (s *Store) loadTaskTx(ctx context.Context, tx pgx.Tx, id string) (task.Task, error) {
	return scanPGTask(tx.QueryRow(ctx, `SELECT `+taskColumns+` FROM tasks WHERE id = $1`, id))
}

// Get returns the current task record.
func (s *Store) Get(ctx context.Context, id task.ID) (task.Task, error) {
	t, err := scanPGTask(s.pool.QueryRow(ctx, `SELECT `+taskColumns+` FROM tasks WHERE id = $1`, id.String()))
	if errors.Is(err, pgx.ErrNoRows) {
		return task.Task{}, task.ErrNotFound
	}

	return t, err
}

// Enqueue persists a new task and records task.enqueued; dedup keys make
// it idempotent (the partial unique index is the arbiter).
func (s *Store) Enqueue(ctx context.Context, n task.New) (task.Task, error) {
	n = n.Normalize()
	if n.Type == "" {
		return task.Task{}, queue.ErrEmptyType
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

	depsJSON, err := json.Marshal(t.Deps)
	if err != nil {
		return task.Task{}, fmt.Errorf("queue: marshal deps: %w", err)
	}

	suppressed := false

	err = s.withTx(ctx, func(tx pgx.Tx) error {
		if n.DedupKey != "" {
			var existingID string

			err := tx.QueryRow(ctx, `SELECT id FROM tasks WHERE dedup_key = $1`, n.DedupKey).Scan(&existingID)
			if err == nil {
				t.ID = task.ID(existingID)
				suppressed = true

				return nil
			}

			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		}

		if _, err := tx.Exec(ctx,
			`INSERT INTO tasks (id, project, type, payload, deps, priority, attempts, max_attempts,
			                    not_before, status, created_at, updated_at, dedup_key)
			 VALUES ($1, $2, $3, $4, $5, $6, 0, $7, $8, 'pending', $9, $10, $11)`,
			t.ID.String(), t.Project, t.Type, string(t.Payload), string(depsJSON), t.Priority,
			t.MaxAttempts, ms(t.NotBefore), now.UnixMilli(), now.UnixMilli(), n.DedupKey); err != nil {
			return err
		}

		for _, d := range t.Deps {
			if _, err := tx.Exec(ctx, `INSERT INTO deps (task_id, dep_id) VALUES ($1, $2)`,
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

func (s *Store) getTaskByDedupKey(ctx context.Context, key string) (task.Task, bool, error) {
	var id string

	err := s.pool.QueryRow(ctx, `SELECT id FROM tasks WHERE dedup_key = $1`, key).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return task.Task{}, false, nil
	}

	if err != nil {
		return task.Task{}, false, err
	}

	t, err := s.Get(ctx, task.ID(id))

	return t, err == nil, err
}

// ClaimDue atomically claims one due task for owner: FOR UPDATE SKIP
// LOCKED keeps competing workers on disjoint rows (the multi-machine
// replacement for SQLite's single serialized writer). Semantics otherwise
// match sqlite.Store: deps gate, expired-lease reclaim with task.released,
// pending cooperative cancels finalized at reclaim.
func (s *Store) ClaimDue(ctx context.Context, owner string, lease time.Duration) (task.Task, error) {
	now := time.Now()

	var claimed task.Task

	finalizedCancel := false

	err := s.withTx(ctx, func(tx pgx.Tx) error {
		var exclusive int

		if s.projectExclusive {
			exclusive = 1
		}

		row := tx.QueryRow(ctx, `
			SELECT t.id, t.status, t.lease_owner FROM tasks t
			WHERE ((t.status = 'pending' AND t.not_before <= $1)
			    OR (t.status = 'running' AND t.lease_expires IS NOT NULL AND t.lease_expires <= $1))
			  AND NOT EXISTS (
			    SELECT 1 FROM deps d JOIN tasks dt ON dt.id = d.dep_id
			    WHERE d.task_id = t.id AND dt.status != 'completed'
			  )
			  AND ($2 = 0 OR t.project = '' OR NOT EXISTS (
			    SELECT 1 FROM tasks r
			    WHERE r.project = t.project AND r.status = 'running' AND r.id != t.id
			  ))
			ORDER BY t.priority DESC, t.created_at ASC, t.id ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED`, now.UnixMilli(), exclusive)

		var id, st, prevOwner string
		if err := row.Scan(&id, &st, &prevOwner); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return queue.ErrNoTaskDue
			}

			return err
		}

		if st == "running" {
			var requested bool

			if err := tx.QueryRow(ctx,
				`SELECT EXISTS(SELECT 1 FROM facts WHERE task_id = $1 AND type = 'task.cancel-requested')`,
				id).Scan(&requested); err != nil {
				return err
			}

			if requested {
				reason, err := cancelRequestedReasonPgTx(ctx, tx, id)
				if err != nil {
					return err
				}

				if err := s.appendFact(ctx, tx, journal.Fact{
					TaskID: id, Type: journal.Released, Owner: prevOwner,
				}); err != nil {
					return err
				}

				tag, err := tx.Exec(ctx, `
					UPDATE tasks SET status = 'cancelled', updated_at = $1, lease_owner = '', lease_expires = NULL
					WHERE id = $2 AND status = 'running'`, now.UnixMilli(), id)
				if err != nil {
					return err
				}

				if tag.RowsAffected() == 0 {
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

		tag, err := tx.Exec(ctx, `
			UPDATE tasks
			SET status = 'running', lease_owner = $1, lease_expires = $2, updated_at = $3
			WHERE id = $4 AND (
			    (status = 'pending' AND not_before <= $3)
			    OR (status = 'running' AND lease_expires IS NOT NULL AND lease_expires <= $3))`,
			owner, now.Add(lease).UnixMilli(), now.UnixMilli(), id)
		if err != nil {
			return err
		}

		if tag.RowsAffected() == 0 {
			return queue.ErrNoTaskDue
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

	if finalizedCancel {
		return task.Task{}, queue.ErrNoTaskDue
	}

	return claimed, nil
}

// leaseErr distinguishes not-found from lease-not-held after a guarded
// UPDATE matched zero rows.
func (s *Store) leaseErr(ctx context.Context, q queryer, id task.ID, owner string) error {
	var status, leaseOwner string

	err := q.QueryRow(ctx, `SELECT status, lease_owner FROM tasks WHERE id = $1`, id.String()).
		Scan(&status, &leaseOwner)
	if errors.Is(err, pgx.ErrNoRows) {
		return task.ErrNotFound
	}

	if err != nil {
		return err
	}

	return task.ErrLeaseNotHeld
}

// queryer is the shared surface of a tx and the pool (pgx pool Tx-like).
type queryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Complete marks a Running task Completed.
func (s *Store) Complete(ctx context.Context, id task.ID, owner string, result json.RawMessage) error {
	now := time.Now()

	return s.withTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE tasks
			SET status = 'completed', completed_at = $1, updated_at = $2,
			    lease_owner = '', lease_expires = NULL, last_error = ''
			WHERE id = $3 AND status = 'running' AND lease_owner = $4 AND lease_expires > $2`,
			now.UnixMilli(), now.UnixMilli(), id.String(), owner)
		if err != nil {
			return err
		}

		if tag.RowsAffected() == 0 {
			return s.leaseErr(ctx, tx, id, owner)
		}

		return s.appendFact(ctx, tx, journal.Fact{
			TaskID: id.String(), Type: journal.Completed, Owner: owner,
			Detail: maybeJSON(result),
		})
	})
}

// Fail records a failed attempt: retry with backoff or dead-letter.
func (s *Store) Fail(
	ctx context.Context,
	id task.ID,
	owner string,
	errText string,
	backoff time.Duration,
	evidence json.RawMessage,
) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		now := time.Now()

		var attempts, maxAttempts int

		err := tx.QueryRow(ctx,
			`SELECT attempts, max_attempts FROM tasks WHERE id = $1 FOR UPDATE`, id.String()).
			Scan(&attempts, &maxAttempts)
		if errors.Is(err, pgx.ErrNoRows) {
			return task.ErrNotFound
		}

		if err != nil {
			return err
		}

		attempts++

		if attempts >= maxAttempts {
			tag, err := tx.Exec(ctx, `
				UPDATE tasks SET status = 'dead', attempts = $1, updated_at = $2, lease_owner = '',
				                 lease_expires = NULL, last_error = $3, not_before = 0
				WHERE id = $4 AND status = 'running' AND lease_owner = $5`,
				attempts, now.UnixMilli(), errText, id.String(), owner)
			if err != nil {
				return err
			}

			if tag.RowsAffected() == 0 {
				return task.ErrLeaseNotHeld
			}

			if err := s.appendFact(ctx, tx, journal.Fact{
				TaskID: id.String(), Type: journal.Failed, Owner: owner, Attempt: attempts, Error: errText,
				Detail: failureDetail(evidence, "exhausted"),
			}); err != nil {
				return err
			}

			return s.appendFact(ctx, tx, journal.Fact{
				TaskID: id.String(), Type: journal.DeadLettered, Owner: owner, Attempt: attempts, Error: errText,
				Detail: json.RawMessage(`{"class":"exhausted"}`),
			})
		}

		tag, err := tx.Exec(ctx, `
			UPDATE tasks SET status = 'pending', attempts = $1, updated_at = $2,
			                 not_before = $3, lease_owner = '', lease_expires = NULL, last_error = $4
			WHERE id = $5 AND status = 'running' AND lease_owner = $6`,
			attempts, now.UnixMilli(), now.Add(backoff).UnixMilli(), errText, id.String(), owner)
		if err != nil {
			return err
		}

		if tag.RowsAffected() == 0 {
			return task.ErrLeaseNotHeld
		}

		return s.appendFact(ctx, tx, journal.Fact{
			TaskID: id.String(), Type: journal.Failed, Owner: owner, Attempt: attempts, Error: errText,
			Detail: evidence,
		})
	})
}

// FailPermanent dead-letters regardless of the attempt budget.
func (s *Store) FailPermanent(
	ctx context.Context,
	id task.ID,
	owner string,
	errText string,
	evidence json.RawMessage,
) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		now := time.Now()

		var attempts int

		err := tx.QueryRow(ctx, `SELECT attempts FROM tasks WHERE id = $1 FOR UPDATE`, id.String()).
			Scan(&attempts)
		if errors.Is(err, pgx.ErrNoRows) {
			return task.ErrNotFound
		}

		if err != nil {
			return err
		}

		attempts++

		tag, err := tx.Exec(ctx, `
			UPDATE tasks SET status = 'dead', attempts = $1, updated_at = $2, lease_owner = '',
			                 lease_expires = NULL, last_error = $3, not_before = 0
			WHERE id = $4 AND status = 'running' AND lease_owner = $5`,
			attempts, now.UnixMilli(), errText, id.String(), owner)
		if err != nil {
			return err
		}

		if tag.RowsAffected() == 0 {
			return task.ErrLeaseNotHeld
		}

		if err := s.appendFact(ctx, tx, journal.Fact{
			TaskID: id.String(), Type: journal.Failed, Owner: owner, Attempt: attempts, Error: errText,
			Detail: failureDetail(evidence, "permanent"),
		}); err != nil {
			return err
		}

		return s.appendFact(ctx, tx, journal.Fact{
			TaskID: id.String(), Type: journal.DeadLettered, Owner: owner, Attempt: attempts, Error: errText,
		})
	})
}

// Requeue returns a claimed task to Pending without counting an attempt.
func (s *Store) Requeue(
	ctx context.Context,
	id task.ID,
	owner string,
	errText string,
	delay time.Duration,
) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		now := time.Now()

		tag, err := tx.Exec(ctx, `
			UPDATE tasks SET status = 'pending', updated_at = $1, not_before = $2,
			                 lease_owner = '', lease_expires = NULL, last_error = $3
			WHERE id = $4 AND status = 'running' AND lease_owner = $5`,
			now.UnixMilli(), now.Add(delay).UnixMilli(), errText, id.String(), owner)
		if err != nil {
			return err
		}

		if tag.RowsAffected() == 0 {
			return task.ErrLeaseNotHeld
		}

		return s.appendFact(ctx, tx, journal.Fact{
			TaskID: id.String(), Type: journal.Requeued, Owner: owner, Error: errText,
			Detail: mustJSON(queue.RequeueEvidence{Reason: errText, RetryIn: delay.Milliseconds()}),
		})
	})
}

// Heartbeat extends the lease of a Running task held by owner.
func (s *Store) Heartbeat(ctx context.Context, id task.ID, owner string, extend time.Duration) error {
	now := time.Now()

	tag, err := s.pool.Exec(ctx, `
		UPDATE tasks SET lease_expires = $1, updated_at = $2
		WHERE id = $3 AND status = 'running' AND lease_owner = $4`,
		now.Add(extend).UnixMilli(), now.UnixMilli(), id.String(), owner)
	if err != nil {
		return err
	}

	if tag.RowsAffected() == 0 {
		return task.ErrLeaseNotHeld
	}

	return nil
}

// Cancel withdraws a Pending task. A non-empty reason is stored in the
// task.cancelled fact detail ("reason" key).
func (s *Store) Cancel(ctx context.Context, id task.ID, reason string) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		var st string

		err := tx.QueryRow(ctx, `SELECT status FROM tasks WHERE id = $1 FOR UPDATE`, id.String()).Scan(&st)
		if errors.Is(err, pgx.ErrNoRows) {
			return task.ErrNotFound
		}

		if err != nil {
			return err
		}

		if st != string(task.Pending) {
			return fmt.Errorf("%w: %s -> cancelled", task.ErrInvalidTransition, st)
		}

		tag, err := tx.Exec(ctx, `
			UPDATE tasks SET status = 'cancelled', updated_at = $1, lease_owner = '', lease_expires = NULL
			WHERE id = $2 AND status = 'pending'`, time.Now().UnixMilli(), id.String())
		if err != nil {
			return err
		}

		if tag.RowsAffected() == 0 {
			return task.ErrInvalidTransition
		}

		return s.appendFact(ctx, tx, journal.Fact{
			TaskID: id.String(), Type: journal.Cancelled, Detail: cancelReasonDetail(reason),
		})
	})
}

// CancelRunning records the cooperative cancel request (idempotent fact).
// A non-empty reason rides the request fact's detail and is carried onto
// the final task.cancelled fact by CancelOwned / the reclaim finalize.
func (s *Store) CancelRunning(ctx context.Context, id task.ID, reason string) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		var st string

		err := tx.QueryRow(ctx, `SELECT status FROM tasks WHERE id = $1 FOR UPDATE`, id.String()).Scan(&st)
		if errors.Is(err, pgx.ErrNoRows) {
			return task.ErrNotFound
		}

		if err != nil {
			return err
		}

		if st != string(task.Running) {
			return fmt.Errorf("%w: %s -> cancel-requested (only running tasks)", task.ErrInvalidTransition, st)
		}

		var requested bool

		if err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM facts WHERE task_id = $1 AND type = 'task.cancel-requested')`,
			id.String()).Scan(&requested); err != nil {
			return err
		}

		if requested {
			return nil
		}

		return s.appendFact(ctx, tx, journal.Fact{
			TaskID: id.String(), Type: journal.CancelRequested, Detail: cancelReasonDetail(reason),
		})
	})
}

// CancelRequested reports a pending cooperative cancel request.
func (s *Store) CancelRequested(ctx context.Context, id task.ID) (bool, error) {
	var requested bool

	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM facts WHERE task_id = $1 AND type = 'task.cancel-requested')`,
		id.String()).Scan(&requested)

	return requested, err
}

// CancelOwned finalizes a cooperative cancel (lease holder). The
// operator's reason (from the cancel-requested fact) is carried onto the
// cancelled fact.
func (s *Store) CancelOwned(ctx context.Context, id task.ID, owner string) error {
	return s.withTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE tasks SET status = 'cancelled', updated_at = $1, lease_owner = '', lease_expires = NULL
			WHERE id = $2 AND status = 'running' AND lease_owner = $3`,
			time.Now().UnixMilli(), id.String(), owner)
		if err != nil {
			return err
		}

		if tag.RowsAffected() == 0 {
			return task.ErrLeaseNotHeld
		}

		reason, err := cancelRequestedReasonPgTx(ctx, tx, id.String())
		if err != nil {
			return err
		}

		return s.appendFact(ctx, tx, journal.Fact{
			TaskID: id.String(), Type: journal.Cancelled, Owner: owner,
			Detail: cooperativeCancelDetail(reason, ""),
		})
	})
}

// cancelRequestedReasonTx reads the reason a task's latest cancel request
// carried ("" when none): the forensics trail the cooperative-cancel
// finalizers copy onto the task.cancelled fact. Best-effort: an unparsable
// detail yields "", never an error — the finalize must not fail on cosmetics.
func cancelRequestedReasonPgTx(ctx context.Context, tx pgx.Tx, id string) (string, error) {
	var detail string

	err := tx.QueryRow(ctx, `
		SELECT detail FROM facts
		WHERE task_id = $1 AND type = 'task.cancel-requested'
		ORDER BY seq DESC LIMIT 1`, id).Scan(&detail)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}

	if err != nil {
		return "", err
	}

	var d struct {
		Reason string `json:"reason"`
	}
	if json.Unmarshal([]byte(detail), &d) != nil {
		return "", nil
	}

	return d.Reason, nil
}

// MarkOrphaned records stranded expired-lease Running tasks (idempotent).
func (s *Store) MarkOrphaned(ctx context.Context, cutoff time.Time) (int, error) {
	marked := 0

	err := s.withTx(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT t.id, t.lease_owner, t.lease_expires
			FROM tasks t
			WHERE t.status = 'running'
			  AND t.lease_expires IS NOT NULL
			  AND t.lease_expires < $1
			  AND NOT EXISTS (
			    SELECT 1 FROM facts f
			    WHERE f.task_id = t.id AND f.type = 'task.orphaned')`,
			cutoff.UnixMilli())
		if err != nil {
			return err
		}

		type orphan struct {
			id, owner string
			expires   int64
		}

		var found []orphan

		for rows.Next() {
			var o orphan

			if err := rows.Scan(&o.id, &o.owner, &o.expires); err != nil {
				rows.Close()

				return err
			}

			found = append(found, o)
		}

		if err := rows.Err(); err != nil {
			rows.Close()

			return err
		}

		rows.Close()

		for _, o := range found {
			detail := mustJSON(map[string]any{
				"owner":          o.owner,
				"leaseExpiredAt": time.UnixMilli(o.expires).UTC().Format(time.RFC3339),
			})

			if err := s.appendFact(ctx, tx, journal.Fact{
				TaskID: o.id, Type: journal.Orphaned, Owner: o.owner, Detail: detail,
			}); err != nil {
				return err
			}

			marked++
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	return marked, nil
}

// RescueDead re-queues a Dead task with a fresh attempt budget.
func (s *Store) RescueDead(ctx context.Context, id task.ID, maxAttempts int) error {
	if maxAttempts <= 0 {
		maxAttempts = task.DefaultMaxAttempts
	}

	return s.withTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE tasks
			SET status = 'pending', attempts = 0, max_attempts = $1, not_before = 0,
			    updated_at = $2, lease_owner = '', lease_expires = NULL, last_error = ''
			WHERE id = $3 AND status = 'dead'`, maxAttempts, time.Now().UnixMilli(), id.String())
		if err != nil {
			return err
		}

		if tag.RowsAffected() == 0 {
			return task.ErrInvalidTransition
		}

		return s.appendFact(ctx, tx, journal.Fact{TaskID: id.String(), Type: journal.Requeued})
	})
}

func pgOrderClause(f queue.Filter) string {
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

	return order
}

// pgWhere builds the shared WHERE clause + args for List/CountTasks
// (mirrors the sqlite store's listWhere, with numbered placeholders).
func pgWhere(f queue.Filter) (string, []any) {
	where := []string{"TRUE"}

	args := []any{}

	if f.Project != nil {
		args = append(args, *f.Project)
		where = append(where, fmt.Sprintf("project = $%d", len(args)))
	}

	if f.Status != nil {
		args = append(args, string(*f.Status))
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}

	if f.Type != nil {
		args = append(args, *f.Type)
		where = append(where, fmt.Sprintf("type = $%d", len(args)))
	}

	if f.Since != nil {
		args = append(args, f.Since.UnixMilli())
		where = append(where, fmt.Sprintf("created_at >= $%d", len(args)))
	}

	if f.Query != "" {
		args = append(args, "%"+escapeLike(f.Query)+"%")
		idx := len(args)
		where = append(where, fmt.Sprintf(
			`(id ILIKE $%d OR type ILIKE $%d OR project ILIKE $%d OR payload ILIKE $%d OR lease_owner ILIKE $%d OR last_error ILIKE $%d)`,
			idx,
			idx,
			idx,
			idx,
			idx,
			idx,
		))
	}

	return strings.Join(where, " AND "), args
}

// List returns tasks matching the filter.
func (s *Store) List(ctx context.Context, f queue.Filter) ([]task.Task, error) {
	where, args := pgWhere(f)

	q := `SELECT ` + taskColumns + ` FROM tasks WHERE ` + where + `
		` + pgOrderClause(f)

	if f.Limit > 0 || f.Offset > 0 {
		q += ` LIMIT $` + strconv.Itoa(len(args)+1)

		args = append(args, f.Limit)

		if f.Offset > 0 {
			q += ` OFFSET $` + strconv.Itoa(len(args)+1)
			args = append(args, f.Offset)
		}
	}

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []task.Task

	for rows.Next() {
		var (
			t            task.Task
			id           string
			depsJSON     string
			payload      string
			leaseExpires *int64
			completedAt  *int64
		)

		var notBefore, createdAt, updatedAt int64

		err := rows.Scan(&id, &t.Project, &t.Type, &payload, &depsJSON, &t.Priority,
			&t.Attempts, &t.MaxAttempts, &notBefore, &t.Status, &t.LeaseOwner,
			&leaseExpires, &t.LastError, &createdAt, &updatedAt, &completedAt)
		if err != nil {
			return nil, err
		}

		t.ID = task.ID(id)
		t.Payload = json.RawMessage(payload)
		t.NotBefore = time.UnixMilli(notBefore)
		t.CreatedAt = time.UnixMilli(createdAt)
		t.UpdatedAt = time.UnixMilli(updatedAt)
		_ = json.Unmarshal([]byte(depsJSON), &t.Deps)

		if leaseExpires != nil {
			le := time.UnixMilli(*leaseExpires)
			t.LeaseExpires = &le
		}

		if completedAt != nil {
			ca := time.UnixMilli(*completedAt)
			t.CompletedAt = &ca
		}

		out = append(out, t)
	}

	return out, rows.Err()
}

func scanFactRow(scanner interface{ Scan(...any) error }) (journal.Fact, error) {
	var (
		f             journal.Fact
		taskID, ftype string
		detail        string
		millis        int64
	)

	err := scanner.Scan(&f.Seq, &millis, &taskID, &ftype, &f.Owner, &f.Attempt, &f.Error, &detail)
	f.Time = time.UnixMilli(millis)
	f.TaskID, f.Type, f.Detail = taskID, journal.FactType(ftype), json.RawMessage(detail)

	return f, err
}

// Facts exposes the journal in Seq order after the cursor.
func (s *Store) Facts(ctx context.Context, after int64, limit int) ([]journal.Fact, error) {
	q := `SELECT seq, time, task_id, type, owner, attempt, error, detail
	      FROM facts WHERE seq > $1 ORDER BY seq ASC`
	args := []any{after}

	if limit > 0 {
		q += ` LIMIT $2`

		args = append(args, limit)
	}

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []journal.Fact

	for rows.Next() {
		f, err := scanFactRow(rows)
		if err != nil {
			return nil, err
		}

		out = append(out, f)
	}

	return out, rows.Err()
}

// LastFacts returns the most recent facts in ascending order.
func (s *Store) LastFacts(ctx context.Context, limit int) ([]journal.Fact, error) {
	if limit <= 0 {
		return s.Facts(ctx, 0, 0)
	}

	// Ascending output from a descending window: inner select takes the
	// tail, the outer re-sorts.
	rows, err := s.pool.Query(ctx, `
		SELECT seq, time, task_id, type, owner, attempt, error, detail FROM (
			SELECT seq, time, task_id, type, owner, attempt, error, detail
			FROM facts ORDER BY seq DESC LIMIT $1
		) tail ORDER BY seq ASC`, limit)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []journal.Fact

	for rows.Next() {
		f, err := scanFactRow(rows)
		if err != nil {
			return nil, err
		}

		out = append(out, f)
	}

	return out, rows.Err()
}

// HeadSeq returns the highest fact seq (0 when empty).
func (s *Store) HeadSeq(ctx context.Context) (int64, error) {
	var head int64

	err := s.pool.QueryRow(ctx, `SELECT COALESCE(MAX(seq), 0) FROM facts`).Scan(&head)

	return head, err
}

// FactsForTask returns one task's facts in Seq order; limit > 0 bounds to
// the MOST RECENT n facts (same contract as the SQLite store).
func (s *Store) FactsForTask(ctx context.Context, id string, limit int) ([]journal.Fact, error) {
	q := `SELECT seq, time, task_id, type, owner, attempt, error, detail
	      FROM facts WHERE task_id = $1`
	args := []any{id}

	if limit > 0 {
		q += ` ORDER BY seq DESC LIMIT $2`

		args = append(args, limit)
	} else {
		q += ` ORDER BY seq ASC`
	}

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var out []journal.Fact

	for rows.Next() {
		f, err := scanFactRow(rows)
		if err != nil {
			return nil, err
		}

		out = append(out, f)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if limit > 0 {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}

	return out, nil
}

// CountFacts counts facts of one type since a time.
func (s *Store) CountFacts(ctx context.Context, ftype journal.FactType, since time.Time) (int64, error) {
	var n int64

	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM facts WHERE type = $1 AND time >= $2`,
		string(ftype), since.UnixMilli()).Scan(&n)

	return n, err
}

// Watermark returns the persisted read cursor for a journal consumer and
// whether it ever checkpointed. seq 0 with exists=true is a valid cursor.
func (s *Store) Watermark(ctx context.Context, consumer string) (int64, bool, error) {
	var seq int64

	err := s.pool.QueryRow(ctx,
		`SELECT seq FROM watermarks WHERE consumer = $1`, consumer).Scan(&seq)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}

	if err != nil {
		return 0, false, err
	}

	return seq, true, nil
}

// SaveWatermark checkpoints a consumer cursor as a monotonic upsert: the
// stored seq never regresses. Consumer progress, not task state, so no
// fact is appended.
func (s *Store) SaveWatermark(ctx context.Context, consumer string, seq int64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO watermarks (consumer, seq, updated_at) VALUES ($1, $2, $3)
		ON CONFLICT(consumer) DO UPDATE SET
			seq = GREATEST(watermarks.seq, excluded.seq),
			updated_at = CASE
				WHEN excluded.seq > watermarks.seq THEN excluded.updated_at
				ELSE watermarks.updated_at
			END`,
		consumer, seq, time.Now().UnixMilli())

	return err
}

// ListWatermarks returns every consumer cursor, by consumer name — the
// admin read behind `tq watermarks show`.
func (s *Store) ListWatermarks(ctx context.Context) ([]queue.WatermarkEntry, error) {
	rows, err := s.pool.Query(ctx,
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

// SetWatermark overwrites a consumer cursor unconditionally — the ops
// rewind hatch (`tq watermarks set`); deliberately bypasses the monotonic
// guard because a rewind is an intentional force-replay.
func (s *Store) SetWatermark(ctx context.Context, consumer string, seq int64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO watermarks (consumer, seq, updated_at) VALUES ($1, $2, $3)
		ON CONFLICT(consumer) DO UPDATE SET
			seq = excluded.seq,
			updated_at = excluded.updated_at`,
		consumer, seq, time.Now().UnixMilli())

	return err
}

// StatusCounts counts tasks per status.
func (s *Store) StatusCounts(ctx context.Context) (map[task.Status]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT status, COUNT(*) FROM tasks GROUP BY status`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	out := make(map[task.Status]int)

	for rows.Next() {
		var st string

		var n int

		if err := rows.Scan(&st, &n); err != nil {
			return nil, err
		}

		out[task.Status(st)] = n
	}

	return out, rows.Err()
}

// ProjectCounts counts tasks per project per status.
func (s *Store) ProjectCounts(ctx context.Context) (map[string]map[task.Status]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT project, status, COUNT(*) FROM tasks GROUP BY project, status`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	out := make(map[string]map[task.Status]int)

	for rows.Next() {
		var project, st string

		var n int

		if err := rows.Scan(&project, &st, &n); err != nil {
			return nil, err
		}

		if out[project] == nil {
			out[project] = make(map[task.Status]int)
		}

		out[project][task.Status(st)] = n
	}

	return out, rows.Err()
}

// CountTasks counts tasks matching the filter.
func (s *Store) CountTasks(ctx context.Context, f queue.Filter) (int, error) {
	where, args := pgWhere(f)

	var n int

	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM tasks WHERE `+where, args...).Scan(&n)

	return n, err
}
