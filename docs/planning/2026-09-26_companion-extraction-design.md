# Companion Extraction — Design (executes the 2026-09-26 dedup ruling)

Owner ruling 2026-09-26: dedup acceptances are ≈ 0; the ADR-0019 spike
adapters (sqlitev4, postgresv4, cqrsqlite) are doomed by S4 (system/ +
metaengine), so their mirrored companion surfaces must live in ONE place —
`internal/queue/companion` — instead of being "accepted" as clones.

## Verified facts this design rests on

- `postgresv4/adapter.go` is `sqlitev4/adapter.go` + 251 diff lines
  (85% identical); the delta is: `pgq` placeholder rewriter, `s.query/exec/
  queryRow` wrappers that apply it, engine/driver imports, doc comments.
  The shared SQL is already written sqlite-style (`?`).
- `cqrsqlite/extras.go` carries the same surface minus the divergences its
  package doc lists (no exclusivity in ClaimDue, Requeue drops
  resume_closeout, owner-string ledger emulation, no archive).
- Engine-backed methods (Enqueue, Complete, Fail, FailPermanent, Heartbeat,
  Cancel*, MarkOrphaned, RescueDead, DismissDead, UpdatePendingPriority)
  are thin `tokenFor + mapErr(s.engine.X(...))` calls — they stay in the
  adapters (per-engine types), they are not clone mass.
- The facade-parity checker (scripts/facadeparity/main.go:144-146) uses a
  pinned pair list; `internal/queue/companion` needs no facade.
- The spike modules are untagged; consumers use the require-v0.3.0 +
  relative-replace pattern (internal/worker/go.mod:19/48, fixed into
  worker/go.mod by the dedup window).

## Core mechanism: pre-dialed Runner

```go
type Dialect func(query string) string

var SQLite Dialect = func(q string) string { return q }
var Postgres Dialect = pgq // pgq moves here verbatim from postgresv4

type Runner interface {
    ExecContext(context.Context, string, ...any) (sql.Result, error)
    QueryContext(context.Context, string, ...any) (*sql.Rows, error)
    QueryRowContext(context.Context, string, ...any) *sql.Row
}

// For bakes the dialect into a handle; adapters call it once per handle.
func For(d Dialect, r Runner) Runner
```

*sql.DB and *sql.Tx satisfy Runner. Companion functions keep the EXACT
bodies from sqlitev4 with `s.db` → `r` — no other transformation. Adapters
hold one `cr Runner` field (set in Open via `companion.For(dial, db)`) and
their methods become one-line delegators.

## Move order (always-green; per-module gate after each batch)

1. `companion/core.go`: Dialect, pgq, Runner, For, scanner/scanTask/
   scanTaskRow/scanFacts, boolInt, escapeLike, mustJSON,
   cooperativeCancelDetail, upstreamFact, identityCodec, mapErr,
   withTx, StoreOption/storeOptions/WithProjectExclusivity.
2. `companion/reads.go`: Facts, LastFacts, HeadSeq, FactsForTask,
   CountFacts, FactsSince, Watermark, SaveWatermark, ListWatermarks,
   SetWatermark, SavePriorityScore, PriorityScore, PriorityScores,
   DeletePriorityScores, StatusCounts, ProjectCounts, listWhere,
   List, CountTasks, Get (bodies verbatim from sqlitev4).
3. `companion/claims.go`: tokenFor, ClaimDue (with the exclusivity SQL —
   sqlitev4:377-505 today), Requeue, leaseErr, cancelRequestedTx,
   cancelRequestedReasonTx.
4. `companion/questions.go`: AppendFact, appendFact, RecordAnswer,
   askedQuestionText, unblockParkedTask, factDetailRefs,
   mergeAnsweredPayload.
5. Rewire sqlitev4 → delegators; module gate. Rewire postgresv4 →
   delegators passing Postgres dial; build+vet+test-compile locally
   (runtime is the CI postgres job — TQ_TEST_POSTGRES is not available
   on this host; disclose in the close-out). Rewire cqrsqlite subset;
   module gate. cqrsqlite keeps its divergent ClaimDue/Requeue/ledger —
   only the surfaces it shares move.
6. Flip `MIRROR_CLONES_STRICT=1` in scripts/ci-local.sh; the gate must
   report 0 cross-backend groups.

## Conformance-suite consolidation (second half of the window)

The three `store_test.go` files (~2000 lines each) mirror one suite. Move
it to `companion/conform` with `StoreSuite(t *testing.T, s Suite)` where
`Suite` carries the opener plus capability knobs the suites already encode
as per-backend expectation edits (enqueued-fact snapshot fields,
resume_closeout presence, exclusivity on/off). Backend-specific tests
(openwithpool_test.go, ledger semantics) stay local. sqlitev4's suite is
the verification base locally; cqrsqlite's suite verifies the knobs.

## Release bookkeeping

- companion joins the disk-derived module loops automatically (no edits);
- its require lands in the three spike go.mods (v0.3.0 + replace, the
  untagged-module pattern); docs/release/VERSION-SURFACES.md gains the
  eighth surface at the next release-sweep window;
- root go.mod/vendorHash: only if root's graph gains companion (it does
  not until cmd/tq or the root app imports it — expected: no FOD drift).

## Non-goals

- No behavior change of any kind; the delegators keep signatures identical.
- The 19 non-spike -t 3 groups (mutex/defer/flag-parse prologs,
  shared-seam call pairs, distinct payload types) are NOT duplication —
  each line calls already-shared code; AGENTS.md records the ruling.
