package companion

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	uqueue "github.com/larsartmann/go-cqrs-lite/queue/v4"
	ufacts "github.com/larsartmann/go-cqrs-lite/queue/v4/facts"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// Dialect rewrites the sqlite-style "?" placeholder SQL every companion
// body is written in into the backend's native form. SQLite is the
// identity; Postgres maps "?" to "$n" ordinals.
type Dialect func(query string) string

// SQLite passes the shared SQL through unchanged.
var SQLite Dialect = func(query string) string { return query }

// Postgres is the placeholder-rewriting dialect (pgq).
var Postgres Dialect = pgq

// pgq rewrites the sqlite-style "?" placeholders the shared companion SQL
// is written in into postgres "$n" ordinals. String literals are respected
// (the companion SQL contains none with "?", but the guard is cheap).
func pgq(query string) string {
	var b strings.Builder

	ordinal := 0
	inString := false

	for _, r := range query {
		switch {
		case r == '\'':
			inString = !inString
			b.WriteRune(r)
		case r == '?' && !inString:
			ordinal++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(ordinal))
		default:
			b.WriteRune(r)
		}
	}

	return b.String()
}

// Runner is the query surface every companion function operates on: a
// *sql.DB, a *sql.Tx, or the dialed wrapper For returns.
type Runner interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Beginner is the transaction-opening surface (*sql.DB). Tx-bodied
// companion functions take it so the dialect applies inside the tx too.
type Beginner interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// For bakes the dialect into a handle; adapters call it once per handle
// and companion functions keep their bodies dialect-free.
func For(d Dialect, r Runner) Runner {
	return dialed{fn: d, inner: r}
}

type dialed struct {
	fn    Dialect
	inner Runner
}

func (h dialed) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return h.inner.ExecContext(ctx, h.fn(query), args...)
}

func (h dialed) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return h.inner.QueryContext(ctx, h.fn(query), args...)
}

func (h dialed) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return h.inner.QueryRowContext(ctx, h.fn(query), args...)
}

// WithTx runs fn inside one transaction, handing it the DIALED tx. A
// non-nil error rolls the tx back; otherwise it commits.
func WithTx(ctx context.Context, db Beginner, d Dialect, fn func(Runner) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	if err := fn(For(d, tx)); err != nil {
		_ = tx.Rollback()

		return err
	}

	return tx.Commit()
}

// StoreOption configures optional store behavior (the adapters alias it so
// their public option surface stays put).
type StoreOption func(*storeOptions)

type storeOptions struct {
	projectExclusive bool
}

// WithProjectExclusivity mirrors internal/queue/sqlite's option: ClaimDue
// will not claim a task whose project already has another running task.
// The upstream engine has no such predicate, so ClaimDue is adapter-owned
// (divergence noted in the S1 report).
func WithProjectExclusivity() StoreOption {
	return func(o *storeOptions) { o.projectExclusive = true }
}

// ApplyProjectExclusivity resolves the option list into the boolean the
// ClaimDue predicate consumes.
func ApplyProjectExclusivity(opts []StoreOption) bool {
	var o storeOptions
	for _, opt := range opts {
		opt(&o)
	}

	return o.projectExclusive
}

// MapErr translates upstream engine errors onto tq's sentinels so callers
// program against one error vocabulary.
func MapErr(err error) error {
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

// UpstreamFact maps a tq journal fact onto the upstream facts.Fact.
// Type strings and detail bytes carry over verbatim; Seq is reassigned
// by the journal on append.
func UpstreamFact(f journal.Fact) ufacts.Fact {
	return ufacts.Fact{
		Time:    f.Time,
		TaskID:  f.TaskID,
		Type:    ufacts.FactType(f.Type),
		Owner:   f.Owner,
		Attempt: f.Attempt,
		Error:   f.Error,
		Detail:  []byte(f.Detail),
	}
}

// IdentityCodec passes payloads through byte-for-byte: tq payloads are
// jsontext.Value, already JSON; the default JSONCodec would base64-encode
// the bytes. A nil value binds as the EMPTY blob, never SQL NULL — the
// upstream tasks.payload column is NOT NULL and tq's zero-value payloads
// are empty, not null (tq's own schema stores the empty string the same
// way).
func IdentityCodec() uqueue.Codec[[]byte] {
	return uqueue.Codec[[]byte]{
		Encode: func(v []byte) ([]byte, error) {
			if v == nil {
				return []byte{}, nil
			}

			return v, nil
		},
		Decode: func(b []byte) ([]byte, error) { return b, nil },
	}
}
