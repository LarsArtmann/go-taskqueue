// Package cqrs exposes the task-queue fact journal as a go-cqrs-lite
// event.Journal / event.SeekableJournal, so the go-cqrs-lite ecosystem
// (projections, watermill.CatchUpSubscriber, SSE brokers, metaengine) can
// consume queue facts directly.
//
// The adapter is read-only by contract: facts are written exclusively by
// the queue stores inside their state-mutating transactions. Event IDs are
// synthetic, sequence-derived ULIDs (see seqEventID): lexicographic ULID
// order matches ascending Seq, no schema migration is needed, and the
// zero EventID decodes to position 0 (the journal start).
package cqrs
