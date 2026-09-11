# Deferred-bundle seeds (plan round 2, C27 / D80–D100)

Small sketches and decisions deferred from round 2. Each item is a seed:
enough shape that the next session can act on it without re-deriving the
problem. Implemented-in-this-round items are marked ✅ with pointers.

## Documented seeds

### D80 — Postgres store sketch ✅ SHIPPED

_(Shipped 2026-09-08: `internal/queue/postgres` driver module —
conformance-tested. The `--store postgres` CLI wiring was aspirational and
is still BLOCKED (TODO_LIST "Owner-blocked decisions"); the sketch below is
the historical seed.)_

Replace `internal/queue/sqlite.go` behind the existing `Store` interface
(ADR-0001 seam). Claim becomes:

```sql
UPDATE tasks SET status='running', owner=$1, lease_until=now()+$2
WHERE id = (
  SELECT t.id FROM tasks t
  WHERE t.status='pending' AND t.not_before <= now()
    AND NOT EXISTS (SELECT 1 FROM tasks d WHERE d.id = ANY(t.deps) AND d.status <> 'completed')
    AND NOT EXISTS (SELECT 1 FROM tasks r WHERE r.status='running' AND r.project = t.project AND r.id <> t.id) -- exclusivity
  ORDER BY t.priority DESC, t.created_at, t.id
  FOR UPDATE OF t SKIP LOCKED LIMIT 1
) RETURNING *;
```

Facts append in the same transaction. `MaxOpenConns(1)` disappears; the
unique partial index on dedup keys becomes `CREATE UNIQUE INDEX … WHERE
dedup_key IS NOT NULL`. Migration question to settle first: lease comparison
in UTC everywhere.

### D82 — internal → public decision

**Decision (recorded 2026-09-06):** keep everything `internal/` until the
v0.2 Postgres store lands. Rationale: the public API would freeze the
`Store` interface before a second implementation proves it; the CLI is the
product until multi-node exists. First packages to promote, in order:
`internal/task` (pure types), `internal/journal`, `internal/queue`
(`Store` + `Filter`), `internal/executor` (Registry) — via nested modules
(`go-modularize` direction), not a flat export. NOT for export:
`internal/e2e`, `internal/bridge/*` (follow the consumers that need them).

### D83 — Cron recurring tasks

Pattern: a schedule owner (systemd timer or the pool loop) enqueues with a
**time-bucketed dedup key** — `cron:<name>:<YYYY-MM-DDTHH>` — so retries and
re-harvests within the bucket dedup, and the next bucket is new work. No
scheduler component; the journal proves which buckets ran. PoC candidate:
10 lines on top of `queue.Enqueue` + a table in DOMAIN_LANGUAGE.

### D90 — Per-repo-size timeout defaults ✅ SHIPPED (as `--repo-timeout`)

_(Shipped 2026-09-08 as the operator-specified ladder `--repo-timeout
name=duration` pinned into harvested payloads — `Config.RepoTimeouts` in
`internal/harvest`; the size-derived default ladder below remains a seed.)_

Seed: `TimeoutMinutes` already exists per payload; the missing piece is a
default ladder keyed on repo size (e.g. <10k LOC → 15m, <100k → 30m, else
45m) resolved at harvest time into the payload so the queue records what
it promised. Measure real agent durations first (`tq top` last-dur gives
the data); do not guess the ladder.

### D91 — Crush rate-limit + version detection at pool start ✅ SHIPPED

_(Shipped 2026-09-08: the startup version probe + missing-binary warning —
`cmd/tq` "agent binary" line; rate-limit detection stays deferred exactly
as the seed says.)_

Seed: at pool start run `crush --version` and record it in the startup line;
warn when the binary is missing (agents will preflight-refuse anyway — the
warning just saves a tick). Rate-limit detection needs a real 429 observed;
when the budget command exists, prefer surfacing THAT as the cause. Defer
active probing.

### D94 — Session chains via `AgentPayload.Session`

`AgentPayload.Session` is already plumbed into the argv (`--session`). Seed
for follow-up tasks: harvest could emit a `followup:<key>` task whose
payload carries the completed task's session id (from result detail), so
the next agent resumes context. Open question: context resume vs. fresh
eyes for quality — decide with data.

### D97 — Cross-repo DAG templates

Seed: extend TODO_LIST item syntax with `deps: <repo>/<item text>`; harvest
resolves sibling items to task IDs and fills `Deps`. Keep templates in the
todo files (human-readable, reviewable) — never in queue config.

### D98 — AI task prioritizer hook

Seed: a `priority` front-matter on TODO_LIST items, set by a prioritizer
(AI or rules), flows through `Config.Priority` → per-item override in
`Item`. Harvest already carries Priority; the change is Item-level parsing
plus "unprioritized = 0" semantics. Guard: prioritizer may only rank, never
add/remove items (parse-guarded like the checkbox format).

### D99 — Smart retry: error-class → policy mapping

Seed: today `PermanentError` → dead-letter-now, `PreflightError` → requeue
(no burn), everything else → ExpBackoff. The extension is a small policy
table: `{class → maxAttempts override, backoff curve, budget burn?}`. E.g.
verify failures → 2 attempts max (re-running an agent on its own half-done
work rarely improves), network → standard. Implement as `Policy func(error)
RetryPolicy` on worker.Config; the classes already exist.

### D100 — Consumer-group pool spike (fencing tokens)

_(D80 has shipped, so this is now live backlog — routed to ROADMAP v0.2
"consumer-group pool / fencing tokens".)_

Design note only: multi-pool without store-level exclusivity could use
fencing tokens (epoch numbers on claims; stale epochs rejected at
Complete/Fail). In SQLite this buys nothing over the serialized writer +
lease model; it matters for the Postgres store where multiple writers are
real. Revisit with D80, not before.

## Operational seed (D96) — DB rotation & backup

The queue database is one file (`$TQ_DB`, default `./tasks.db`, WAL mode).
Guidance:

- **Backup:** `sqlite3 tasks.db ".backup /backup/tasks-$(date +%F).db"` —
  safe against a running pool (WAL-aware), unlike copying the file.
- **Rotation:** the journal grows unboundedly by design (facts-first).
  Archive cold facts instead of deleting: monthly `tq facts --after <seq>`
  dumps to `facts-<month>.ndjson`, then a future `tq journal compact` (not
  built) may truncate below the watermark. Until then: move the whole file
  and start fresh — tasks are rebuildable from harvest; history lives in
  the archive.
- **Restore drill:** `tq stats` + `tq facts | head` against the backup
  proves readability. No restore drill = no backup.

## Implemented this round (pointers)

- ✅ D84 structured result payload (`files_changed`, `commit_sha` via
  `TQ_RESULT:` line) — `internal/executor/result.go`, unit-tested
- ✅ D85 output sidecar (`TQ_LOG_DIR`, path recorded in result detail) —
  `internal/executor/agent.go`, unit-tested
- ✅ D87 `tq harvest --json` / `--repo-subset` glob
- ✅ D88 `tq dlq --rescue-all --older-than`
- ✅ D89 `--projects-dir /` and `$HOME` refused with remediation
- ✅ D81/D86/D95 PoC server (`examples/api`): enqueue API, `/metrics`
  (Prometheus text), live stats page — smoke-verified, localhost-only
- ✅ D75 SSE stream PoC (`examples/sse`) — live-verified
- ✅ D92 PR-mode mechanics (`scripts/poc/pr-mode.sh`, local mode;
  real PRs gated behind `OPEN_PR=1` + owner's remote)
- ✅ D93 worktree isolation mechanics (`scripts/poc/worktree-isolation.sh`)
