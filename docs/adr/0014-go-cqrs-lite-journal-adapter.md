# ADR-0014: go-cqrs-lite journal adapter (`internal/journal/cqrs`)

Date: 2026-09-12
Status: Accepted

## Context

The fact journal (ADR-0006/0009/0010) and go-cqrs-lite's event journal are
the same shape: append-only, strictly ordered, cursor-readable records from
which every view is a projection. The sibling ecosystem (PapDashboard,
overview, CV, DiscordSync, bank-sync and ~15 more repos) already depends on
go-cqrs-lite; go-taskqueue alone only "composed in spirit" — nothing
type-level connected the two. An owner instruction (2026-09-12) directs
maximum practical adoption of go-cqrs-lite here.

Direct storage adoption was rejected: rewiring the queue stores onto
go-cqrs-lite event stores would break the recorded store invariants
(single serialized writer, facts appended in the same transaction as state,
`MaxOpenConns(1)`), force a migration of the live dogfood journal, and buy
nothing the invariants don't already guarantee. Maximal adoption ≠ maximal
rewrite; the right unit of integration is the journal contract itself.

## Decision

1. **New leaf sub-module `internal/journal/cqrs`** adapts any
   `FactSource` (the store `Facts(after, limit)` contract) to
   go-cqrs-lite `event.Journal` + `event.SeekableJournal`
   (`event/v4` v4.11.0, pinned). The adapter is READ-ONLY by contract:
   writes stay exclusively inside the queue stores' state-mutating
   transactions. Implementing `event.Store`'s optimistic-concurrency
   write path is deliberately NOT adopted — a generic Save would bypass
   task-state invariants.
2. **Leaf placement, not `internal/journal`:** the DAG's purest modules
   (task, journal) must not grow the go-cqrs-lite module graph, or every
   module transitively requiring journal would carry it. Only consumers
   of the adapter (root module) pay the dependency.
3. **Synthetic sequence-encoded event IDs** (no schema migration): the
   16 ULID bytes are `[0:6]` zero timestamp, `[6:14]` Seq big-endian,
   `[14:16]` zero. Lexicographic ULID order equals ascending Seq; the
   zero `id.EventID` decodes to position 0 (journal start); any
   non-conforming ULID is foreign and drains empty — which satisfies the
   go-cqrs-lite dangling-cursor contract (unknown cursor → empty, nil;
   never a replay). Seq positioning is SAFER than ID positioning under
   compaction gaps (ADR-0006/0010): a cursor can never cause duplicate
   delivery into projections.
4. **Event mapping:** `FactType` → `event.Type` verbatim
   (`task.enqueued`, `session.opened`, …); `Fact.Time` → `OccurredAt`;
   task ID → `StreamID` with StreamType `Task` (`Session` for the
   synthetic `session:` identities); global Seq → `event.Version`
   (documented deviation from per-stream versions — read-path consumers
   of Journal/SeekableJournal do not use Version); payload = the fact
   body JSON (`taskId`, `type`, `owner`, `attempt`, `error`, `detail`
   as raw JSON), marshaled with encoding/json under
   GOEXPERIMENT=jsonv2, `Detail` inlined via jsontext.Value.
5. **CLI surface:** `tq facts --cqrs` renders the journal as
   go-cqrs-lite events through the adapter (`SliceSource` over the
   store's facts) — the interop is operator-visible and smoke-tested
   end-to-end (`TestFactsCQRSOverStore`).
6. **License, recorded:** go-cqrs-lite is PROPRIETARY
   (© 2026 Lars Artmann, all rights reserved; transitive first-party deps
   go-branded-id/go-codec/go-error-family share the author; the only
   third-party code deps are Apache/MIT: fxamacker/cbor, oklog/ulid/v2,
   x448/float16). The httputil ruling (2026-09-10) rejected a
   proprietary dep to protect tq's all-permissive redistribution story.
   This ADR records the owner's explicit instruction of 2026-09-12 to
   adopt go-cqrs-lite anyway — as copyright holder the owner authorizes
   his own use, and accepts that tq binaries redistributing with this
   module compiled in carry proprietary code. If tq redistribution to
   third parties ever becomes a goal, relicensing go-cqrs-lite (or
   dropping this module) is the gate. Recorded, not re-litigated.
7. **Lint baseline regen (892 findings, 107 rows, 2026-09-12):** the new
   module ships with err113 findings (dynamic wrapped errors — the
   repo-wide wrap style, see sqlite's existing err113 row) and
   paralleltest rows; the same regen absorbed a repo-wide paralleltest
   drift that landed in ~10 modules the same day (config change outside
   this work). varnamelen and wsl_v5 findings in the new module were
   FIXED, not baselined.

## Consequences

- Any go-cqrs-lite consumer (projections, `watermill.CatchUpSubscriber`
  with a broker, SSE replay tooling, metaengine) can read the task
  queue's fact journal as a native seekable journal.
- The contract is pinned by adapter unit tests (ordering, batching,
  foreign-cursor safety, payload round-trip, ULID round-trip + ordering
  for seq 1..500) AND a store-backed integration test through real
  `sqlite.Store` + the CLI render path.
- Not adopted (considered, deferred to ROADMAP): per-task-stream
  `event.EventSource` (needs per-task ordinal versioning — the facts
  index already supports it if a use case appears), watermill bus
  integration (tq has no broker; `internal/consumer` + watermarks
  already implement at-least-once in-order delivery per ADR-0009),
  branded task IDs via `id/v4` (cross-module earthquake, no interop
  payoff), and `dedup`/`idempotency` stores (tq's dedup is
  same-transaction with the enqueue — an external idempotency store
  cannot provide that guarantee).
- flake vendorHash updated (`sha256-8zjS/KNmEm6Es2n4Xyj/a6Mn+oLntLycRXp
  GNTkWPWg=`); watch for the runner-variant hash mismatch class
  (2026-09-10) on the first CI nix run.

## Post-adoption note (2026-09-13)

A `cqrs-lint` pass found the adapter's events carried an **empty encoding
stamp**: `event.NewEvent` never stamps `encoding`, so every downstream
`event.DecodePayloadAuto` — the decode path this adapter exists to serve —
failed with `codec.ErrUnknownEncoding`. `factEvent` now builds events via
`event.New` + `WithCodec(codec.JSONCodec{})` (payload bytes unchanged) and
pins `WithSchemaVersion(1)` explicitly. The consumer contract is pinned by
`TestPayloadDecodesThroughLibraryAPI`, a store-level encoding assertion
(`cmd/tq/facts_cqrs_test.go`), and a byte-exact wire-format pin
(`TestPayloadWireFormatPinned`). Read-only intent + triaged linter rules
live in `.cqrs-lint.json`.
