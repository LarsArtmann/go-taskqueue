package main

import (
	"database/sql"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// jsonUnmarshalDeps decodes the stored deps JSON array.
func jsonUnmarshalDeps(depsJSON string, out *[]task.ID) error {
	return json.Unmarshal([]byte(depsJSON), out)
}

// msToTime converts a unix-millis column value (the journal's timestamp
// encoding) back to a time.
func msToTime(ms int64) time.Time { return time.UnixMilli(ms) }

func jsontextValue(s string) jsontext.Value { return jsontext.Value(s) }

// scanOldTask mirrors the hand-rolled store's task scan: NULL lease
// expiry and completed time stay nil pointers, payload is the raw JSON
// text, deps ride as the stored JSON array.
func scanOldTask(rows *sql.Rows) (task.Task, error) {
	var (
		t                                                                task.Task
		payload, depsJSON                                                string
		priority, attempts, maxAttempts, notBefore, createdAt, updatedAt int64
		leaseExpires, completedAt                                        sql.NullInt64
	)
	if err := rows.Scan(&t.ID, &t.Project, &t.Type, &payload, &depsJSON, &priority,
		&attempts, &maxAttempts, &notBefore, &t.Status, &t.LeaseOwner, &leaseExpires,
		&t.LastError, &createdAt, &updatedAt, &completedAt, &t.DedupKey); err != nil {
		return task.Task{}, err
	}

	t.Payload = jsontext.Value(payload)
	t.Deps = []task.ID{}
	if depsJSON != "" && depsJSON != "[]" {
		if err := jsonUnmarshalDeps(depsJSON, &t.Deps); err != nil {
			return task.Task{}, err
		}
	}

	t.Priority = int(priority)
	t.Attempts = int(attempts)
	t.MaxAttempts = int(maxAttempts)
	t.NotBefore = msToTime(notBefore)
	t.CreatedAt = msToTime(createdAt)
	t.UpdatedAt = msToTime(updatedAt)

	if leaseExpires.Valid {
		exp := msToTime(leaseExpires.Int64)
		t.LeaseExpires = &exp
	}

	if completedAt.Valid {
		done := msToTime(completedAt.Int64)
		t.CompletedAt = &done
	}

	return t, nil
}

// equalFacts compares facts field-by-field, detail bytes included.
func equalFacts(a, b journal.Fact) bool {
	return a.Seq == b.Seq &&
		a.Time.Equal(b.Time) &&
		a.TaskID == b.TaskID &&
		a.Type == b.Type &&
		a.Owner == b.Owner &&
		a.Attempt == b.Attempt &&
		a.Error == b.Error &&
		string(a.Detail) == string(b.Detail)
}

// equalTasks compares task projections field-by-field.
func equalTasks(a, b task.Task) bool {
	if a.ID != b.ID ||
		a.Project != b.Project ||
		a.Type != b.Type ||
		string(a.Payload) != string(b.Payload) ||
		a.Priority != b.Priority ||
		a.Attempts != b.Attempts ||
		a.MaxAttempts != b.MaxAttempts ||
		!a.NotBefore.Equal(b.NotBefore) ||
		a.Status != b.Status ||
		a.LeaseOwner != b.LeaseOwner ||
		a.LastError != b.LastError ||
		a.DedupKey != b.DedupKey ||
		!a.CreatedAt.Equal(b.CreatedAt) ||
		!a.UpdatedAt.Equal(b.UpdatedAt) {
		return false
	}

	if !sameDeps(a.Deps, b.Deps) {
		return false
	}

	if !sameTimePtr(a.LeaseExpires, b.LeaseExpires) || !sameTimePtr(a.CompletedAt, b.CompletedAt) {
		return false
	}

	return true
}

func sameDeps(a, b []task.ID) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func sameTimePtr(a, b *time.Time) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return a.Equal(*b)
	}
}
