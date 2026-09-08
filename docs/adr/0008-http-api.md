# ADR-0008: The v0.2 HTTP API — token-gated writes, fencing roadmap

- Status: Accepted (first slice shipped)
- Date: 2026-09-08
- Deciders: owner + agent session (round-5 plan, ideas I50/I51 / task M22)
- Relates to: ADR-0003 (dashboard read-only stance), ADR-0007 (Postgres store)

## Context

Non-Go producers (scripts, cron, webhooks, other services) need to
enqueue work without a Go dependency; consumers need queue stats. The
PoC (`examples/api`) proved the shape but binds loopback with no auth —
correct for an example, wrong for production. The dashboard (`tq serve`)
is deliberately read-only (ADR-0003); this new surface is its write
counterpart and must not weaken that stance.

## Decision

`tq api` (`internal/httpapi`) exposes a versioned, token-gated write API:

```
POST /api/v1/tasks   enqueue one task        {project, type, payload, ...}
GET  /api/v1/stats   per-status counts + total
GET  /api/v1/healthz liveness (token-gated too — no unauthenticated oracle)
```

1. **The token is mandatory at construction** — no loopback exemption,
   because this surface exists to be exposed to other machines.
   `Authorization: Bearer` or `?token=` (same EventSource-driven
   exception as the dashboard), constant-time compare, `WWW-Authenticate`
   challenge on 401, `Cache-Control: no-store` everywhere.
2. **Validation is explicit and actionable**: every 400 carries
   `{"error": what, "fix": how}` (missing type/payload, non-RFC3339
   notBefore, malformed JSON). A 1 MiB body cap bounds request memory.
3. **Dedup keys ride through**: idempotent enqueue is the API's
   at-least-once story for webhook-style retried producers (pinned by a
   test: same key, same id, one row).
4. **Payload is passed through verbatim**: the API does not interpret
   executor contracts; the executor's own validation remains the source
   of truth (and its errors land in `tq show`).

## Fencing tokens (design note, round-5 D100 / F117)

Problem: a worker that lost its lease (network pause, GC stall) may still
write a terminal outcome, racing the reclaiming worker. Today the store
guards with `lease_owner + lease_expires` in every terminal UPDATE —
correct, but the racing zombie gets a generic `lease not held` and the
journal cannot distinguish "lost lease" from "wrong owner".

Design: `ClaimDue` additionally returns a **lease generation**
(monotonic per task: `leases` column incremented on every claim). Every
terminal write carries the generation the worker was granted;
`lease generation mismatch` becomes a distinct, journalable error class,
and the fencing check is one integer compare instead of a timestamp
reasoning. Implementation fits both backends (SQLite: column + UPDATE
bump; Postgres: same, returned from the SKIP LOCKED tx). Deferred until a
real double-write incident makes the observability worth the schema
change — the lease guard already prevents the harm; fencing makes the
RACE legible.

## Consumer groups (sketch, round-5 F118)

Long polling workers claim via `ClaimDue(owner=...)`; competing groups
today share one global queue. A consumer-group claim path would add:

- `groups(name, id, heartbeat_at)` — durable group registry;
- claim SQL gains `AND (group_id IS NULL OR group_id = $me)` — tasks
  enqueued with `groupId` are only claimable by that group (the mirror
  of `--project-exclusive`, but opt-in per task instead of per pool);
- no routing daemon: groups are a WHERE clause, keeping the one-binary
  story. Dead-letter and rescue semantics are unchanged.

This stays a sketch until a workload needs exclusive queues; the
`DedupKey`+`groupId` combination (per-group idempotency) is the open
question to answer first.

## Consequences

- `examples/api` remains the Go-embedded example; `tq api` is the
  production path (`--auth-token` / `$TQ_API_TOKEN`, default
  `127.0.0.1:8091`).
- The read-only dashboard stance is intact: writes live behind a
  separately-started, separately-tokened process.
- v0.2's public surface now has three doors: Go embedding (`queue.New`),
  the dashboard (read), and this API (write) — each with one auth story.
