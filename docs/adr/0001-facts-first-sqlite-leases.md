# ADR-0001: Facts-first core, SQLite-embedded, lease-based claims

**Status:** Accepted (2026-09-05)
**Context:** Cross-project work (builds, releases, scrapes, agent tasks) is
scattered across cron jobs, shells, and memory. Existing options were rejected:
full CQRS event-sourcing (go-cqrs-lite) is heavyweight for a queue; Redis-based
queues (asynq, machinery) demand infrastructure I don't want to babysit;
Taskwarrior is not embeddable in Go and has no worker pool.

**Decision:**

1. **Facts-first.** Every mutation appends an immutable fact (`task.enqueued`,
   `claimed`, `completed`, `failed`, `dead-lettered`, `cancelled`, `rescued`,
   `lease-released`) in the same transaction as the state change. Task rows and
   stats are projections over facts. This is the go-cqrs-lite insight with the
   ceremony stripped: no aggregates, no command bus — just a journal that can
   always answer "what exactly happened".
2. **SQLite embedded (modernc.org/sqlite, CGo-free).** One binary, one file,
   zero services. Single serialized writer connection makes claim exclusivity
   trivial. The `Store` interface is the distribution seam: a Postgres store
   (`FOR UPDATE SKIP LOCKED`) is the v0.2 path to multi-machine workers with
   identical semantics.
3. **Lease-based claims.** Claim = exclusive lease with TTL, renewed by
   heartbeat. Crashed worker → lease expires → any worker reclaims. At-least-once
   execution, no stuck tasks, no external lock service.
4. **Pluggable executors.** Workers resolve task `type` to an `Executor`
   (interface, 1 method). Built-ins: `sh` (payload-as-command or template),
   `http` (POST). Go users register funcs directly.
5. **DAG deps in the claim query.** A task whose deps aren't all `completed`
   is simply not selectable — no scheduler component, one `NOT EXISTS` clause.

**Consequences:** Journal grows unboundedly (compaction is a later concern);
single-node until the Postgres store lands; executors must be idempotent
(at-least-once contract).

**Alternatives rejected:** asynq (Redis dependency, opaque internals),
machinery (stale), River (Postgres-only, too early), do-it-inside-go-cqrs-lite
(wrong repo scope — the example taskmanager is a demo, not a library).
