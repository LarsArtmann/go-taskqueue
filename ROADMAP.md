# ROADMAP

Long-term direction and raw ideas. Actionable near-term work lives in
TODO_LIST.md; shipped work is recorded in CHANGELOG.md and FEATURES.md.

## v0.1.0 — Single-node foundation (current)

- [x] Facts-first core: journal, SQLite store, lease claims, deps, DLQ
- [x] `tq` CLI: enqueue / worker / harvest / agent-pool / stats / show / dlq / cancel / facts / tail
- [x] Executors: `sh`, HTTP, headless agent with verify contracts
- [x] Harvest: TODO_LIST.md backlogs → agent tasks across a projects dir
- [x] Idempotent enqueue (dedup keys) + legacy-DB migration
- [x] flake.nix, CI, README, AGENTS.md, FEATURES.md, ADRs
- Release itself (tag + GitHub release + pkg.go.dev) is tracked in TODO_LIST.md

## v0.2.0 — Distribution seam

- Postgres store (`SELECT … FOR UPDATE SKIP LOCKED`) behind the existing
  Store interface — the interface is the seam, no semantics change
- HTTP API server so non-Go producers can enqueue (thin wrapper over Store)
- Consumer-group pool: multi-process safety with fencing tokens

## v0.3.0 — Ecosystem bridges

- PapDashboard integration: dead-letter alerting is shipped; the remaining
  arc is decision → question fan-out (agent asks, human answers in the
  dashboard, queue proceeds) and surfacing PapDashboard questions inside
  `tq serve` (the `tq tail -f` → SSE fan-out shipped with the web UI —
  see ADR-0003 and
  `docs/planning/2026-09-07_16-25_SUPERB-PLAN-ROUND3-LIVE-WEB-UI.md`;
  remaining Phase D ideas: write actions behind `--allow-writes`, auth for
  non-localhost binds, `/metrics` merge, pagination, budget panel,
  Datastar upgrade)

## v0.4.0 — Intelligence

- ai-task-prioritizer: ranking model writes the `priority` field
- Smart retry policies keyed on error classification (permanent vs transient)
- Per-project concurrency limits as a first-class store concept

## Raw ideas (unrefined)

- Cron-style recurring tasks (re-enqueue with dedup keys on completion)
- Cross-repo DAG from harvest: configurable templates like "docs item
  depends on code item"
- `tq agent-pool --once` (single harvest+drain pass for scripts and tests)
- Structured per-task result payload: `{files_changed, commit_sha, verify_output_tail}`
- PR-mode: agent commits to a branch and opens a PR instead of committing directly
- Git worktree isolation option (agents never touch the user's checkout)
- Session continuation chains via `AgentPayload.Session` ("follow-up on previous item")
- Rate-limit concurrent crush sessions per machine; detect the crush version
  at pool start to catch flag-contract drift early
- Output sidecar: store full agent stdout to a blob file, keep only the tail in facts
- Metrics endpoint (Prometheus) over the facts projection
- Heartbeat cadence scaled to lease for very long tasks
- `tq harvest --json` for dashboards; `--repo-subset` glob filter
- Timeout defaults per repo size (small repos don't need 45m)
- `tq dlq --rescue-all --older-than` bulk rescue
- Guard: refuse `--projects-dir /` or `$HOME` (harvest scanning catastrophically wide)
- `.crushrc` permissions lint: warn when a repo grants `bash` to an unsandboxed pool
- Security.md documenting what autonomy grants mean and the blast radius of `bash`
- Fuzz the TODO parser (malformed markdown, CRLF, BOM); Windows path handling
  in harvest; i18n-safe item hashing
- Queue DB rotation/backup guidance (single file = single point of failure)
- Chaos test: SIGKILL a pool mid-agent-run; assert lease-expiry reclaim and
  no double-complete
- GitHub Actions job running the stub-agent e2e (no API cost)
- Example corpus: runnable `examples/agent-pool/` demo repo with `.crushrc` + TODO_LIST.md
- Decide and document when `internal/` packages become a public, importable
  library API

## Non-goals

- Becoming a general-purpose workflow engine — task queue + pool, not orchestration
- Replacing go-cqrs-lite or PapDashboard — compose with them, never absorb them
- GPU scheduling / compute placement

## Open questions (owner decisions)

- Should the pool be allowed to work on go-taskqueue itself? This repo has a
  TODO_LIST.md full of pool food but deliberately no `.crushrc` — 5+
  concurrent agents already edit it.
- What cost ceiling applies to a first production run (per day, per repo)?
- Cancelled-task dedup semantics: today a cancelled task's dedup key
  suppresses re-enqueue forever (escape hatch: edit the item text). Should
  cancellation instead release the key, accepting that "cancel" no longer
  means "stop bringing this back"? Store-schema-affecting.

## Deferred-bundle seeds

Raw ideas with enough shape to act on live in
[docs/planning/2026-09-06_deferred-bundle-seeds.md](docs/planning/2026-09-06_deferred-bundle-seeds.md):
Postgres claim SQL sketch (D80), the internal→public promotion order (D82),
cron recurring tasks via time-bucketed dedup keys (D83), per-repo timeout
ladders (D90), session chains (D94), cross-repo DAG templates (D97), the
AI prioritizer hook (D98), retry-policy table (D99), fencing-token design
note (D100), and DB rotation/backup guidance (D96).
