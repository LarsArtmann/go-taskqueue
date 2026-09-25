// Package readmodel is the ADR-0019 S3 read model: tq's task ledger and
// status counts as go-cqrs-lite metaengine Store collections (planned
// tables), folded from the fact journal, with metaengine Watcher/ServeSSE
// as the live-fragment mechanism that replaces the hand journal-tailer →
// hub fan-out (ADR-0003 Phase D read path) once the S3 flip lands.
//
// The Model opens its OWN sqlite database beside the queue DB (path
// decision: separate file — the projection is disposable and rebuildable
// from the journal, and a second writer connection on the queue file would
// contend with the store's single serialized writer). Feeding is
// journal-first: every lifecycle fact maps to an event applied through
// metaengine Store.ApplyRecord, so the collections are a fold projection
// of the journal exactly like every other tq consumer (ADR-0001).
//
// THIN-ENQUEUE SIDE CHANNEL (interim): the upstream engine's task.enqueued
// fact carries only {project, type} today — priority, dedup key and the
// creation timestamp are not in the journal (S1 divergence, M4
// ratification memo pending). While that holds, the Projector fills those
// fields from the store row on first sight of a task (RowSource; nil
// disables). The side channel dies with the upstream detail growth: the
// fold for task.enqueued then reads everything from the fact.
//
// The package is read-only over the queue: it never mutates task state.
package readmodel

// TaskRow is one ledger row in the tasks collection: the fields the
// queue views (board, table, stats) render, folded from the journal.
type TaskRow struct {
	ID string `json:"id"`
	// Project is the task's repo/project scope (empty = unscoped).
	Project string `json:"project"`
	// Type is the executor task type (sh, agent, review, ...).
	Type string `json:"type"`
	// Status is the task.Status value (pending, running, completed,
	// dead, cancelled).
	Status string `json:"status"`
	// Priority is the STORED priority (ADR-0015); aging is scheduling
	// and never mutates it.
	Priority int `json:"priority"`
	// Attempts counts claimed leases — every claim is one attempt start.
	Attempts int `json:"attempts"`
	// CreatedAt/UpdatedAt are unix millis — the task-row storage format.
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
	// DedupKey is the harvest/enqueue identity (empty for ad-hoc tasks).
	DedupKey string `json:"dedup_key"`
	// LastError is the most recent failure text (empty when none).
	LastError string `json:"last_error"`
}

// TaskFilter selects the Tasks read. Nil fields mean "no filter"; the
// values are the raw queue vocabulary (task.Status string, project name).
type TaskFilter struct {
	Status  *string
	Project *string
}

// StringPtr is a small helper for building a TaskFilter inline.
func StringPtr(s string) *string { return new(s) }
