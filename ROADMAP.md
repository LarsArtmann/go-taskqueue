# ROADMAP

Long-term direction and raw ideas. Actionable near-term work lives in
TODO_LIST.md; shipped work is recorded in CHANGELOG.md and FEATURES.md.

## v0.1.0 — Single-node foundation (current)

- [x] Facts-first core: journal, SQLite store, lease claims, deps, DLQ
- [x] `tq` CLI: enqueue / worker / harvest / agent-pool / stats / audit / top / show / dlq / cancel / facts / tail / serve
- [x] Executors: `sh`, HTTP, headless agent with verify contracts
- [x] Harvest: TODO_LIST.md backlogs → agent tasks across a projects dir
- [x] Idempotent enqueue (dedup keys) + legacy-DB migration
- [x] flake.nix, CI, README, AGENTS.md, FEATURES.md, ADRs
- [x] Shipped 2026-09-06 as a GitHub pre-release (annotated tag, proxy +
      pkg.go.dev verified); checklist archived at
      `docs/release/archived/2026-09-06_v0.1.0_CHECKLIST.md`

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
- Retry-policy table keyed on error class (D99 sketch: per-class attempt
  overrides and backoff curves on top of the shipped permanent/transient
  classes)
- Per-project concurrency limits as a first-class store concept

## Raw ideas (unrefined)

### Queue core / scale

- Journal compaction design note (facts-first ADR-0001 flagged it; the web
  UI's full-journal scans raise its priority), then `tq journal compact
  --before SEQ`
- `tq journal verify`: checksum chain over facts for tamper-evidence
- Store hot-cold split: archive facts older than N days to a cold table
- `Store.List` filter pushdown (`q` at the SQL layer) so web UI/top search
  stops being an in-memory full scan
- SSE `Replay` + ring buffer to replace snapshot-per-tick for
  high-frequency queues
- Cron-style recurring tasks (time-bucketed dedup keys, D83 seed)
- Cross-repo DAG from harvest: configurable templates like "docs item
  depends on code item" (D97 seed)
- Session continuation chains via `AgentPayload.Session` (D94 seed)
- Rate-limit concurrent crush sessions per machine; detect the crush
  version at pool start to catch flag-contract drift early (D91)
- Heartbeat cadence scaled to lease for very long tasks
- Timeout defaults per repo size (small repos don't need 45m, D90 seed)
- Example corpus: runnable `examples/agent-pool/` demo repo with `.crushrc`
  - TODO_LIST.md

### Web UI polish (Phase D seeds beyond the v0.3 arc)

- Live-updating task detail page (`/task/{id}` is static HTML today)
- Table column sorting toggles (client-side, no server cost)
- `aria-live` regions on fragments for screen-reader announcements
- Journal viewer mode (`/facts?after=` with infinite scroll) as the human
  replacement for `tq tail -f`
- Per-project dashboard pages (`/project/{name}`) reusing the filter
  pipeline
- `tq serve --open` (browser auto-open); dark/light theme toggle (CSS
  variables already isolate colors); SSE `retry:` hint; humanized payload
  preview in table rows; inline-SVG favicon; journal-size + watermark stat
  card

### CI / tooling

- `concurrency:` group in ci.yml to cancel superseded runs (the
  auto-commit daemon pushes in bursts)
- Advisory lint cost: scope to changed packages (diff-based) or move to a
  scheduled job instead of recomputing a known ~400-finding result per push
- `govulncheck` step (binary already in the flake devShell); dependabot for
  actions + modules; upgrade pinned actions past the Node 20 deprecation
- Nightly `-race -count=3` full-suite job (flake-catching for the race
  gate)
- Fuzz `unwrapCommand` payload shapes (raw/JSON string/`{"cmd":...}`/
  hostile input)
- Sentinel errors per package (`errors.go` convention) to burn down the
  err113 findings — post lint-endgame decision
- Triage the 32 gosec findings: real issues vs false positives
- templ LSP false diagnostics (57 errors / 145 warnings against a green
  `go build`): investigate gopls/templ-lsp coexistence; until fixed, the
  rule is "LSP webui diagnostics are false positives, trust the CLI"
- Windows smoke variant of `webui.sh` (currently POSIX-only)

### Observability / ops

- `tq version` subcommand printing the ldflags-injected version (verify
  `main.version` is actually wired)
- `tq top --json` shape contract test (agents consume it; pin the output)
- dlq rescue UX: print the rescue plan before enqueueing on
  `--rescue-all --older-than`
- Web UI scale test: snapshot + SSE burst at 100k tasks (fragment size,
  render latency, memory) to put a number on the W18 pagination trigger
- Load test: 10k tasks / 100 projects claim-throughput baseline, recorded
  in FEATURES as a regression guard for SQL changes
- Budget telemetry → papdashboard alert (pool spend visible in the ops
  dashboard)
- Catch-up tasks with a cheaper executor: try `sh`+python vs agent for `tq
  audit` repairs; keep the agent only if measurably needed
- ADR-0004: lint policy decision record (advisory rationale, endgame
  options, what would re-gate it)
- Status-report index for docs/status: reports newest-first, superseded ones
  marked
- e2e coverage for `tq audit` and `tq top --json` on a seeded DB (both are
  unit + manual smoke only)
- Chaos variant: SIGKILL the pool (not just a worker) mid-drain under
  `--once`; assert systemd-restart safety
- Promote remaining shell smokes to Go e2e tests (multi-repo smoke →
  `internal/e2e`)

## Non-goals

- Becoming a general-purpose workflow engine — task queue + pool, not orchestration
- Replacing go-cqrs-lite or PapDashboard — compose with them, never absorb them
- GPU scheduling / compute placement

## Open questions (owner decisions)

- ~~Should the pool be allowed to work on go-taskqueue itself? This repo has a
  TODO_LIST.md full of pool food but deliberately no `.crushrc` — 5+
  concurrent agents already edit it.~~ ANSWERED 2026-09-07 (owner): the pool
  now runs on this repo — `.crushrc` (minimum autonomy) + `.tq-verify` (CI
  hard gates) are the rails; see
  `docs/planning/2026-09-07_20-47_SUPERB-PLAN-ROUND4-DOGFOOD-POOL-EATS-THIS-REPO.md`.
- What cost ceiling applies to a first production run (per day, per repo)?
- Cancelled-task dedup semantics: today a cancelled task's dedup key
  suppresses re-enqueue forever (escape hatch: edit the item text). Should
  cancellation instead release the key, accepting that "cancel" no longer
  means "stop bringing this back"? Store-schema-affecting.
- Lint endgame (19:33 report g2): (a) trim `.golangci.yml` to an
  enforceable set and re-gate, (b) keep advisory + a checked-in baseline
  file that can only shrink, or (c) permanent advisory? The current config
  documents a bar the codebase doesn't meet and enforces nothing.
- Master push workflow (19:33 report g3): accept direct-push +
  push-then-verify from agents and the auto-commit daemon, or add branch
  protection / a PR flow?
- PR-mode enablement: the PoC (`scripts/poc/pr-mode.sh`) proves branch +
  push mechanics; which repos may agents deliver to via real PRs
  (`OPEN_PR=1` policy)?

## Deferred-bundle seeds

Raw ideas with enough shape to act on live in
[docs/planning/2026-09-06_deferred-bundle-seeds.md](docs/planning/2026-09-06_deferred-bundle-seeds.md):
Postgres claim SQL sketch (D80), the internal→public promotion order (D82),
cron recurring tasks via time-bucketed dedup keys (D83), per-repo timeout
ladders (D90), session chains (D94), cross-repo DAG templates (D97), the
AI prioritizer hook (D98), retry-policy table (D99), fencing-token design
note (D100), and DB rotation/backup guidance (D96).
