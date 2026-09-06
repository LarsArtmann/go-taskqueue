# ROADMAP

Long-term direction and raw ideas. Actionable near-term work lives in the
SUPERB plan (`docs/planning/`) and the CHANGELOG; shipped work is tagged.

## v0.1.0 — Single-node foundation (in progress)

- [x] Facts-first core: journal, SQLite store, lease claims, deps, DLQ
- [x] `tq` CLI: enqueue / worker / stats / show / dlq / cancel / facts / tail
- [x] Executors: `sh`, HTTP, headless agent/crush with verify contracts
- [x] Harvest: TODO_LIST.md backlogs → agent tasks across a projects dir
- [x] Idempotent enqueue (dedup keys) + legacy-DB migration
- [x] flake.nix, CI, README, AGENTS.md, ADRs
- [ ] Tag v0.1.0 + GitHub release + pkg.go.dev

## v0.2.0 — Distribution seam

- Postgres store (`SELECT … FOR UPDATE SKIP LOCKED`) behind the existing
  Store interface — the interface is the seam, no semantics change
- HTTP API server so non-Go producers can enqueue (thin wrapper over Store)
- Consumer-group pool: multi-process safety with fencing tokens

## v0.3.0 — Ecosystem bridges

- **PapDashboard bridge**: DLQ dead-letter → PapDashboard
  `POST /api/ingest` `alert.triggered` (sourceApp=go-taskqueue), so broken
  tasks light up the ops dashboard with correlation IDs
- Decision requests → PapDashboard `question` aggregate (agent asks, human
  answers in the dashboard, queue proceeds)
- `tq tail -f` → PapDashboard SSE fan-out for a unified ops view

## v0.4.0 — Intelligence

- ai-task-prioritizer: ranking model writes the `priority` field
- Per-project concurrency limits and cost budgets (agent tasks cost real
  money; cap burn per repo)
- Smart retry policies keyed on error classification (permanent vs transient)

## Raw ideas (unrefined)

- Web UI over the projections (the journal already has everything needed)
- Cron-style recurring tasks (re-enqueue with dedup keys on completion)
- Task result artifacts: persist executor stdout to a blob sidecar table
- Multi-node `tq worker` over a shared NFS SQLite file (probably a bad idea;
  Postgres first)
