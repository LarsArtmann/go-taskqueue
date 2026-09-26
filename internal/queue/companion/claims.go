package companion

import (
	"context"
	"database/sql"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"time"

	uqueue "github.com/larsartmann/go-cqrs-lite/queue/v4"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// TokenFor enforces tq's token gate for a finalize: the caller must
// present the claim token minted at ClaimDue (theft detection lives in
// the store — ADR-0019 S1). With requireLive the lease must also be
// unexpired; CancelOwned deliberately skips liveness (a worker may
// finish a stop just after lease lapse, before reclaim).
func TokenFor(ctx context.Context, r Runner, id task.ID, claim queue.Claim, requireLive bool) (string, error) {
	row := r.QueryRowContext(ctx, `
		SELECT status, COALESCE(lease_expires, 0), COALESCE(lease_token, '')
		FROM tasks WHERE id = ?`, id.String())

	var (
		status  string
		expires int64
		token   string
	)
	if err := row.Scan(&status, &expires, &token); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", task.ErrNotFound
		}

		return "", err
	}

	if status != "running" || token == "" || token != string(claim) {
		return "", task.ErrLeaseNotHeld
	}

	if requireLive && (expires == 0 || expires <= time.Now().UnixMilli()) {
		return "", task.ErrLeaseNotHeld
	}

	return token, nil
}

// ClaimDue atomically claims at most one due task for owner, with tq's
// project-exclusivity predicate (the upstream engine's candidate query has
// no such clause — S1 divergence). The claim mints an upstream-format
// fencing token and stamps it on the row, so engine-backed finalizes keep
// working for the claimed task.
func ClaimDue(
	ctx context.Context,
	d Dialect,
	db Beginner,
	projectExclusive bool,
	owner string,
	lease time.Duration,
) (task.Task, queue.Claim, error) {
	now := time.Now()

	var claimed task.Task

	claim := uqueue.NewClaimToken()

	finalizedCancel := false

	err := WithTx(ctx, db, d, func(r Runner) error {
		row := r.QueryRowContext(ctx, `
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
			ORDER BY t.priority + CASE WHEN (? - t.created_at) / 86400000.0 / ? < ?
			    THEN (? - t.created_at) / 86400000.0 / ? ELSE ? END DESC,
			  t.created_at ASC, t.id ASC
			LIMIT 1`,
			now.UnixMilli(), now.UnixMilli(), boolInt(projectExclusive),
			now.UnixMilli(), float64(queue.PriorityAgingDaysPerPoint), float64(queue.PriorityAgingMaxBonus),
			now.UnixMilli(), float64(queue.PriorityAgingDaysPerPoint), float64(queue.PriorityAgingMaxBonus))

		var id, st, prevOwner string
		if err := row.Scan(&id, &st, &prevOwner); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return queue.ErrNoTaskDue
			}

			return err
		}

		if st == "running" {
			requested, err := cancelRequestedTx(ctx, r, id)
			if err != nil {
				return err
			}

			if requested {
				reason, err := cancelRequestedReasonTx(ctx, r, id)
				if err != nil {
					return err
				}

				if err := appendFact(ctx, r, journal.Fact{
					TaskID: id, Type: journal.Released, Owner: prevOwner,
				}); err != nil {
					return err
				}

				res, err := r.ExecContext(ctx, `
					UPDATE tasks SET status = 'cancelled', updated_at = ?, lease_owner = '', lease_expires = NULL
					WHERE id = ? AND status = 'running'`, now.UnixMilli(), id)
				if err != nil {
					return err
				}

				if n, _ := res.RowsAffected(); n == 0 {
					return queue.ErrNoTaskDue
				}

				if err := appendFact(ctx, r, journal.Fact{
					TaskID: id, Type: journal.Cancelled, Owner: owner,
					Detail: cooperativeCancelDetail(reason, "lease-expiry"),
				}); err != nil {
					return err
				}

				finalizedCancel = true

				return nil
			}

			if err := appendFact(ctx, r, journal.Fact{
				TaskID: id, Type: journal.Released, Owner: prevOwner,
			}); err != nil {
				return err
			}
		}

		res, err := r.ExecContext(ctx, `
			UPDATE tasks
			SET status = 'running', lease_owner = ?, lease_expires = ?, lease_token = ?, updated_at = ?
			WHERE id = ? AND (
			    (status = 'pending' AND not_before <= ?)
			    OR (status = 'running' AND lease_expires IS NOT NULL AND lease_expires <= ?))`,
			owner, now.Add(lease).UnixMilli(), claim, now.UnixMilli(), id, now.UnixMilli(), now.UnixMilli())
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

		if err := appendFact(ctx, r, journal.Fact{TaskID: id, Type: journal.Claimed, Owner: owner}); err != nil {
			return err
		}

		claimed, err = loadTask(ctx, r, id)

		return err
	})
	if err != nil {
		return task.Task{}, "", err
	}

	if finalizedCancel {
		return task.Task{}, "", queue.ErrNoTaskDue
	}

	return claimed, queue.Claim(claim), nil
}

// Requeue returns a claimed task to Pending without counting an attempt.
// Adapter-side (not the engine's): tq's task.requeued evidence carries the
// resume_closeout flag, which the upstream RequeueEvidence lacks (S1
// divergence). The claim token fences the requeue; the recorded fact's
// owner is the claim's OWNER (read from the row before the release), so
// the journal keeps speaking owner vocabulary.
func Requeue(
	ctx context.Context,
	d Dialect,
	db Beginner,
	id task.ID,
	claim queue.Claim,
	errText string,
	delay time.Duration,
	resumeCloseout bool,
) error {
	return WithTx(ctx, db, d, func(r Runner) error {
		now := time.Now()

		var prevOwner string
		if err := r.QueryRowContext(ctx, `
			SELECT COALESCE(lease_owner, '') FROM tasks
			WHERE id = ? AND status = 'running' AND lease_token = ?`,
			id.String(), string(claim)).Scan(&prevOwner); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return leaseErr(ctx, r, id)
			}

			return err
		}

		res, err := r.ExecContext(ctx, `
			UPDATE tasks
			SET status = 'pending', not_before = ?, last_error = ?, updated_at = ?,
			    lease_owner = '', lease_expires = NULL, lease_token = ''
			WHERE id = ? AND status = 'running' AND lease_token = ?`,
			now.Add(delay).UnixMilli(), errText, now.UnixMilli(), id.String(), string(claim))
		if err != nil {
			return err
		}

		if n, _ := res.RowsAffected(); n == 0 {
			return leaseErr(ctx, r, id)
		}

		return appendFact(ctx, r, journal.Fact{
			TaskID: id.String(), Type: journal.Requeued, Owner: prevOwner, Error: errText,
			Detail: mustJSON(queue.RequeueEvidence{
				Reason: errText, RetryIn: delay.Milliseconds(), ResumeCloseout: resumeCloseout,
			}),
		})
	})
}

// AppendFactTx records a NON-task journal fact in its own transaction
// (session.opened / session.closed). Task facts are never written through
// it — every task mutation appends its fact inside its own operation's
// transaction.
func AppendFactTx(ctx context.Context, d Dialect, db Beginner, f journal.Fact) error {
	return WithTx(ctx, db, d, func(r Runner) error {
		return appendFact(ctx, r, f)
	})
}

func appendFact(ctx context.Context, r Runner, f journal.Fact) error {
	if f.Time.IsZero() {
		f.Time = time.Now()
	}

	detail := string(f.Detail)
	_, err := r.ExecContext(ctx,
		`INSERT INTO facts (time, task_id, type, owner, attempt, error, detail)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		f.Time.UnixMilli(), f.TaskID, string(f.Type), f.Owner, f.Attempt, f.Error, detail)

	return err
}

func cancelRequestedTx(ctx context.Context, r Runner, id string) (bool, error) {
	var requested bool

	err := r.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM facts WHERE task_id = ? AND type = 'task.cancel-requested')`, id).
		Scan(&requested)

	return requested, err
}

func cancelRequestedReasonTx(ctx context.Context, r Runner, id string) (string, error) {
	rows, err := r.QueryContext(ctx,
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

func leaseErr(ctx context.Context, r Runner, id task.ID) error {
	var st string

	err := r.QueryRowContext(ctx, `SELECT status FROM tasks WHERE id = ?`, id.String()).Scan(&st)
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
