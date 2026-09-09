# ADR-0007: The Postgres store — same semantics, SKIP LOCKED claims

- Status: Accepted (first slice shipped and verified)
- Date: 2026-09-08
- Deciders: owner + agent session (round-5 plan, idea I49 / task M21)
- Relates to: ADR-0001 (facts-first core), ADR-0006 (journal compaction)

## Context

SQLiteStore serves the one-binary story perfectly, but its concurrency
model is one serialized writer (`MaxOpenConns(1)` + WAL + busy_timeout):
correct, and fine for a single host, but a ceiling for many producers and
workers across machines. The `Store` interface (ADR-0001's seam) exists
precisely so a networked backend can slot in without touching the worker,
harvester, webui, or bridges.

## Decision

`PostgresStore` (`internal/queue/postgres/postgres.go` since ADR-0012; originally `internal/queue/postgres.go`, jackc/pgx v5 — pure Go,
CGO stays off) is a SEMANTIC TWIN of SQLiteStore, not a redesign:

1. **Identical storage mapping**: unix-milli BIGINT timestamps, TEXT ids,
   the same `tasks`/`deps`/`facts` shapes, the same partial unique dedup
   index, the same allowlisted sort orders. Projections and the journal
   stay byte-compatible across backends; the conformance tests import the
   same vocabulary.
2. **Claims via `SELECT ... FOR UPDATE SKIP LOCKED`**: competing workers
   lock disjoint candidate rows instead of queueing behind one
   connection. The guarded `UPDATE ... WHERE status/lease` re-check from
   the SQLite path is kept as the second line of defense (SKIP LOCKED
   locks; the WHERE proves the row was still claimable at write time).
3. **Attempt-budget reads use `FOR UPDATE`** inside the tx (Fail,
   FailPermanent, Cancel): the row lock replaces SQLite's "single writer
   makes it safe" argument.
4. **Every mutation appends its fact in the same transaction** — the
   facts-first invariant is backend-neutral.
5. **Cooperative cancel, orphan marking, expired-lease reclaim with
   cancel-finalize**: ported verbatim; the reclaim path checks the
   `task.cancel-requested` fact exactly like SQLite.

Not ported (deliberately): `ArchiveFactsBefore`/`ArchiveSummary` are
SQLite admin operations off the interface; ADR-0006 compaction needs a
Postgres-specific design (partitioning) before it ships.

## Verification

- `TestPostgresLifecycle` pins the full semantic path against a real
  database (env `TQ_TEST_POSTGRES`; CI runs a `postgres:16` service).
- `TestPostgresClaimExclusivityUnderConcurrency` hammers ClaimDue from
  parallel pool connections: no task claimed twice.
- `TestPostgresOrphanMarking` pins the observation fact.
- Baselines (local unix-socket cluster, env-gated `TQ_BASELINE=1`):
  enqueue ~14.5k/s, claim+complete **~838/s at 1k depth** — vs SQLite's
  ~28.5k/s enqueue but only ~234/s claim+complete at 10k depth. The
  claim-side gap is the actual finding: SQLite's due-scan degrades with
  queue depth, Postgres's index + row locks do not. (Depths differ;
  re-measure at equal depth before quoting ratios.)

## Consequences

- `tq` CLI still opens SQLite only; wiring a `--store postgres` DSN flag
  comes with the v0.2 HTTP API surface (round-5 M22) — the store itself
  is ready for `queue.New` consumers today.
- CI grows a postgres service job; the SQLite suite stays the default
  gate so contributor machines need nothing new.
- Fencing tokens (M22 design note) become cheap later: the claim tx can
  return a monotonically increasing lease generation.
