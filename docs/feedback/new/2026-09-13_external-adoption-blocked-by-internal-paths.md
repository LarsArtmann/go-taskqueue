# Feedback: adoption attempt from an external consumer (MTU Help Centre)

**Date**: 2026-09-13
**Who**: Help Centre session (`Rolls-Royce-mtuGoHelpCenter-golang`)
**Question asked**: "Can we learn from go-taskqueue? or use it?"
**Outcome**: learned from it, could not use it — and the reason is purely
structural, not quality. The library is good. It is also unreachable.

---

## What happened

The Help Centre needed a durable background queue for exactly one job type
(generate article embeddings after CQRS writes). go-taskqueue's semantics
(facts-first journal, SKIP LOCKED claims, retries + DLQ, Postgres store)
are precisely that. So we tried to import it.

Import attempt (scratch module, local `replace` directives onto a checkout):

```
use of internal package github.com/larsartmann/go-taskqueue/internal/queue not allowed
```

This is Go's internal-package rule, not a bug: every implementation module
lives under `internal/` (`internal/queue`, `internal/queue/postgres`,
`internal/task`, `internal/journal`, `internal/executor`, `internal/worker`).
The root module `github.com/larsartmann/go-taskqueue` contains **zero Go
files** — only `replace` directives into `internal/`. There is no importable
path, from anywhere outside this repository.

## Why it stings

- The work is *tagged* but not *reachable*: per ADR-0012, release tooling
  tags the `internal/` sub-modules (v0.2.0 exists). A tagged module whose
  path Go forbids importing is a release in name only.
- ADR-0012 already made the right naming decision — `database/sql`-driver
  style (`queue/postgres`, constructor `Open`) — it just lives behind the
  `internal/` wall.
- The evaluation cost is real: we read the README, AGENTS.md, ADRs, the
  `Store` contract, and the `fullcore` example (which is *exactly* our use
  case) before discovering no import path exists. The README did not stop
  us earlier.

## What we shipped instead (evidence the semantics are the product)

A right-sized in-repo queue (~500 LOC + tests) copying the proven profile:

- durable rows in the app's own Postgres, one row per job (PK dedup)
- claim via `FOR UPDATE SKIP LOCKED` + visibility timeout (crash reclaim
  without a lease column or heartbeat — single-process profile)
- version-guarded idempotent enqueue: `ON CONFLICT` revives only when the
  stored version is older, resetting attempts/dead — replay- and
  rebuild-safe by construction
- bounded backoff ladder (1m/5m/15m/30m), dead-letter flag at exhaustion

All tested against a real Postgres container. It works — but it is a fork
of your ideas, not your code, and it will now drift.

## What I would like to see changed

1. **A public facade module per consumer-facing contract.**
   e.g. `github.com/larsartmann/go-taskqueue/queue` re-exporting
   `internal/queue` (types, `Store`, `Filter`, sentinels), plus
   `.../queue/postgres` re-exporting the store. Keep implementations
   internal as long as you like; the facade pins only names. This is the
   smallest change that makes the library usable from outside, without a
   big-bang promotion or freezing the API.
2. **If/when promoting**: the ADR-0012 driver-style layout is already the
   right public shape — move, don't rename.
3. **A references/ doc for the minimal single-job-type profile**:
   PK-dedup row per job + visibility-timeout claim + version-guarded
   enqueue/revive + bounded ladder + dead flag. It generalizes beyond this
   library and it is the part adopters will re-derive anyway.
4. **Constructor accepting an existing `*pgxpool.Pool`** (if not already
   planned): consumers with a pool do not want a second connection pool via
   `database/sql`.
5. **README: state consumer status in one line** near the top
   ("library is in-repo only until facades land" or similar), so the next
   evaluator spends minutes, not an afternoon.

## Offer

When a facade lands, the Help Centre is a willing first external consumer:
one job type, Postgres, real deadlines — a good conformance target. We will
also feed back the port diff.
