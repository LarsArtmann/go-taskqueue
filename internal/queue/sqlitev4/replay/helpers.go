package main

import (
	"database/sql"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// timeFromMillis converts a unix-millis column value (the journal's
// timestamp encoding) back to a time.
func timeFromMillis(millis int64) time.Time { return time.UnixMilli(millis) }

// jsonText wraps a stored detail string as its JSON value type.
func jsonText(detail string) jsontext.Value { return jsontext.Value(detail) }

// jsonUnmarshalDeps decodes the stored deps JSON array.
func jsonUnmarshalDeps(depsJSON string, out *[]task.ID) error {
	return json.Unmarshal([]byte(depsJSON), out)
}

// scanOldTask mirrors the hand-rolled store's task scan: NULL lease
// expiry and completed time stay nil pointers, payload is the raw JSON
// text, deps ride as the stored JSON array.
func scanOldTask(rows *sql.Rows) (task.Task, error) {
	var (
		one                             task.Task
		payload, depsJSON               string
		priority, attempts, maxAttempts int64
		notBefore, createdAt, updatedAt int64
		leaseExpires, completedAt       sql.NullInt64
	)
	if err := rows.Scan(&one.ID, &one.Project, &one.Type, &payload, &depsJSON, &priority,
		&attempts, &maxAttempts, &notBefore, &one.Status, &one.LeaseOwner, &leaseExpires,
		&one.LastError, &createdAt, &updatedAt, &completedAt, &one.DedupKey); err != nil {
		return task.Task{}, err
	}

	one.Payload = jsonText(payload)

	one.Deps = []task.ID{}
	if depsJSON != "" && depsJSON != "[]" {
		if err := jsonUnmarshalDeps(depsJSON, &one.Deps); err != nil {
			return task.Task{}, err
		}
	}

	one.Priority = int(priority)
	one.Attempts = int(attempts)
	one.MaxAttempts = int(maxAttempts)
	one.NotBefore = timeFromMillis(notBefore)
	one.CreatedAt = timeFromMillis(createdAt)
	one.UpdatedAt = timeFromMillis(updatedAt)

	if leaseExpires.Valid {
		expiry := timeFromMillis(leaseExpires.Int64)
		one.LeaseExpires = &expiry
	}

	if completedAt.Valid {
		done := timeFromMillis(completedAt.Int64)
		one.CompletedAt = &done
	}

	return one, nil
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

// equalTasks compares task projections field-by-field: identity, then
// scheduling state, then timestamps.
func equalTasks(a, b task.Task) bool {
	return equalTaskIdentity(a, b) &&
		equalTaskScheduling(a, b) &&
		equalTaskTimestamps(a, b) &&
		sameDeps(a.Deps, b.Deps) &&
		sameTimePtr(a.LeaseExpires, b.LeaseExpires) &&
		sameTimePtr(a.CompletedAt, b.CompletedAt)
}

func equalTaskIdentity(a, b task.Task) bool {
	return a.ID == b.ID &&
		a.Project == b.Project &&
		a.Type == b.Type &&
		string(a.Payload) == string(b.Payload) &&
		a.DedupKey == b.DedupKey
}

func equalTaskScheduling(a, b task.Task) bool {
	return a.Priority == b.Priority &&
		a.Attempts == b.Attempts &&
		a.MaxAttempts == b.MaxAttempts &&
		a.Status == b.Status &&
		a.LeaseOwner == b.LeaseOwner &&
		a.LastError == b.LastError &&
		a.NotBefore.Equal(b.NotBefore)
}

func equalTaskTimestamps(a, b task.Task) bool {
	return a.CreatedAt.Equal(b.CreatedAt) && a.UpdatedAt.Equal(b.UpdatedAt)
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
