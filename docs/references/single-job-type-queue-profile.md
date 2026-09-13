# Reference: the minimal single-job-type durable queue profile

Derived from the go-taskqueue semantics (facts-first journal, SKIP LOCKED
claims, retries + DLQ) as applied by the first external adoption attempt
(MTU Help Centre, 2026-09-13 — see
`docs/feedback/new/2026-09-13_external-adoption-blocked-by-internal-paths.md`).
It is the right-sized pattern when you need ONE durable background job type
and a full queue library is too much. It generalizes beyond this repo.

## The five ingredients

1. **One durable row per job, in your own database.**
   Primary key on the natural job identity gives idempotent enqueue for
   free: inserting a job twice is one row, not two runs. No separate queue
   table, no second database.

2. **Claim via `FOR UPDATE SKIP LOCKED` + visibility timeout.**
   The claim transaction selects candidate rows with
   `SELECT … FOR UPDATE SKIP LOCKED SKIP LOCKED`-style semantics (Postgres:
   `FOR UPDATE SKIP LOCKED`; SQLite: a single serialized writer needs no
   lock skipping at all), marks the job `running`, and stamps a deadline.
   A crashed worker's jobs become claimable again when the deadline passes
   — crash reclaim WITHOUT a lease owner column or heartbeats. This is the
   single-process profile; multi-process fleets want go-taskqueue's
   owner + heartbeat lease instead.

3. **Version-guarded idempotent enqueue / revive.**
   `ON CONFLICT (id) DO UPDATE … WHERE excluded.version > jobs.version`
   revives only when the incoming row is NEWER, resetting attempts and the
   dead flag. Replay of old events, log rebuilds, and duplicate deliveries
   become harmless by construction — ordering bugs degrade to no-ops
   instead of double work.

4. **Bounded backoff ladder.**
   Fixed delays (e.g. 1m → 5m → 15m → 30m) rather than unbounded
   exponential growth: a bounded ladder keeps retry pressure predictable
   and the table small (jobs park dead after ~4 retries in minutes, not
   after days).

5. **Dead-letter flag at exhaustion.**
   Attempts exhausted → set `dead = true`, keep the row, keep the last
   error. The dead flag IS the DLQ for a single job type: one query lists
   everything needing a human, and "revive" is resetting the flag and
   attempts.

## When to graduate to go-taskqueue

Multiple job types with different executors, DAG dependencies between
jobs, per-project budgets/pacing, an immutable audit trail of every state
change (the facts journal), or a live dashboard — that is the point where
the ~500-LOC profile forks and drifts, and embedding the real library
(facades under `github.com/larsartmann/go-taskqueue/queue/…`) is cheaper
than maintaining the fork.
