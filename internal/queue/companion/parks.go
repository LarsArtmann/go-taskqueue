package companion

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// Park suppresses claims on a PENDING task until the given instant (tq
// park): the operator's reversible disposition, recorded as a fact. Until
// moves NotBefore (clamped to now — a past until is the release); the
// task.parked fact lands in the SAME transaction, so "parked but still
// claimable" is unrepresentable and the journal keeps every ruling.
// Running/terminal tasks refuse with task.ErrInvalidTransition, unknown
// ids with task.ErrNotFound.
func Park(
	ctx context.Context,
	d Dialect,
	db Beginner,
	id task.ID,
	reason string,
	until time.Time,
	by string,
) error {
	return WithTx(ctx, db, d, func(r Runner) error {
		now := time.Now()

		var status string
		if err := r.QueryRowContext(ctx,
			`SELECT status FROM tasks WHERE id = ?`, id.String()).Scan(&status); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return task.ErrNotFound
			}

			return err
		}

		if status != "pending" {
			return task.ErrInvalidTransition
		}

		notBefore := until
		if notBefore.Before(now) {
			notBefore = now
		}

		res, err := r.ExecContext(ctx, `
			UPDATE tasks
			SET not_before = ?, updated_at = ?
			WHERE id = ? AND status = 'pending'`,
			notBefore.UnixMilli(), now.UnixMilli(), id.String())
		if err != nil {
			return err
		}

		if n, _ := res.RowsAffected(); n == 0 {
			return task.ErrInvalidTransition
		}

		return appendFact(ctx, r, journal.Fact{
			TaskID: id.String(),
			Type:   journal.Parked,
			Owner:  by,
			Detail: MustJSON(queue.ParkedDetail{
				Reason:  reason,
				UntilMS: notBefore.UnixMilli(),
				By:      by,
			}),
		})
	})
}
