# ROADMAP

Long-term direction and raw ideas. Actionable near-term work lives in
TODO_LIST.md; shipped work is recorded in CHANGELOG.md and FEATURES.md.

## v0.1.0 — Single-node foundation (shipped 2026-09-06)

- [x] Facts-first core: journal, SQLite store, lease claims, deps, DLQ
- [x] `tq` CLI: enqueue / worker / harvest / agent-pool / stats / audit / top / show / dlq / cancel / facts / tail / serve
- [x] Executors: `sh`, HTTP, headless agent with verify contracts
- [x] Harvest: TODO_LIST.md backlogs → agent tasks across a projects dir
- [x] Idempotent enqueue (dedup keys) + legacy-DB migration
- [x] flake.nix, CI, README, AGENTS.md, FEATURES.md, ADRs
- [x] Shipped as a GitHub pre-release (annotated tag, proxy + pkg.go.dev
      verified); checklist archived at
      `docs/release/archived/2026-09-06_v0.1.0_CHECKLIST.md`

## v0.2.0 — Distribution seam

- [x] Postgres store first slice (ADR-0007): full Store semantics over pgx
      (`SELECT … FOR UPDATE SKIP LOCKED`), conformance battery in CI against
      postgres:16, parallel-claim hammer. Remaining: CLI `--store` wiring and
      the compaction twin (`ArchiveFactsBefore`) for parity.
- [x] HTTP API server (ADR-0008): `tq api` with mandatory token auth,
      actionable validation JSON, dedup passthrough, stats + healthz.
      Remaining slices: cancel + claim-over-HTTP; fencing tokens deferred
      until a real double-write justifies the schema change.
- Consumer-group pool: multi-process safety with fencing tokens (sketched in
  ADR-0008 as a WHERE clause, not a router)
- [x] Cut the release (`scripts/release.sh v0.2.0`) — shipped 2026-09-09
      (tag, module proxy, clean-room install, GitHub Release all verified)

## v0.3.0 — Ecosystem bridges

- PapDashboard integration: dead-letter alerting, resolve correlation, and
  budget-mirror alerts are shipped; the remaining arc is decision → question
  fan-out (agent asks, human answers in the dashboard, queue proceeds —
  design note
  `docs/planning/2026-09-06_decision-question-fanout.md`) and surfacing
  PapDashboard questions inside `tq serve`
- Web UI Phase D remainder: enqueue-from-UI and board drag-and-drop behind
  the existing `--allow-writes` + CSRF gate (cancel/rescue shipped);
  `/metrics` merge; Datastar upgrade path

## v0.4.0 — Intelligence

- ai-task-prioritizer: ranking model writes the `priority` field
- Retry-policy table keyed on error class (D99 sketch: per-class attempt
  overrides and backoff curves on top of the shipped permanent/transient
  classes)
- Per-project concurrency limits as a first-class store concept
- Deeper agent reviews: feed the reviewer the worker's full crush session
  transcript (via go-crush-data's typed session/message reads, keyed by the
  `AgentResult.SessionID` already stored in completion facts) instead of
  only the diff + item; also rubric-style scored criteria (mindwalk judge
  pattern) on top of the binary verdict

## Raw ideas (unrefined)

### Queue core / scale

- Data-model review of `task.Task`/`Status` (branded IDs? split lifecycle
  stages into distinct types?) — data-model territory, revisit before any
  public API promotion (2026-09-09 23:47 f18)
- `queue.Store` interface segregation: 25+ methods, candidate read-side vs
  write-side split — only if an embedder actually chafes (2026-09-09 23:47
  f41; ADR-0012 left the contract whole)
- SSE `Replay` + ring buffer to replace snapshot-per-tick for
  high-frequency queues
- Cron-style recurring tasks (time-bucketed dedup keys, D83 seed)
- Cross-repo DAG from harvest: configurable templates like "docs item
  depends on code item" (D97 seed)
- Session continuation chains via `AgentPayload.Session` (D94 seed)
- ~~`Filter.Since` SQL pushdown~~ DONE 2026-09-10 (CLI `--since` + SQL
  pushdown in both stores); the webui `?since=` twin remains below
- Batched facts-by-ids query in `queue.Store` for status-window detail
  lookups
- `tq journal compact --before`: turn the shipped ADR-0006 prototype
  (`ArchiveFactsBefore`) into a CLI command when ADR-0010's demand triggers
  fire (~50k facts or an operator ask); Postgres twin ships in the same
  change
- `tq journal verify`: checksum chain over facts for tamper-evidence
- ~~Transient-error (DNS/429) retry classification~~ 429 half DONE
  2026-09-11 (`DetectRateLimit`/`RateLimitError`, no attempt burn); generic
  DNS-timeout classification still open
- Example corpus: runnable `examples/agent-pool/` demo repo with `.crushrc`
  - TODO_LIST.md
- `tq daemon`: serve + agent-pool + bridges + harvest scheduler in one
  long-lived process — owner-gated ADR first (05:24 report g1, ROUND6 R1);
  the actor rollout (`internal/runactor`) is the structural prerequisite,
  already shipped
- Dispatcher phase 2: notify-after-commit push with poll fallback
  (ADR-0009); migrate the papdashboard bridge onto `internal/consumer`
  (ADR-0009 D4 — restart battery green, precondition met)
- Loud-resync drill: test a consumer restarting past the retention floor

### Agent pool / loop

- Worktree-per-agent (the intra-repo parallelism path): design written —
  `docs/planning/2026-09-12_worktree-per-agent-design.md` (claim → worktree →
  verify → merge → reap; the merge-policy owner call gates implementation)
- `tq status` subcommand: loop state per project (window count, last
  report, next mint at N) instead of deriving it from `stats` + facts
- `tq loop-stats` retrospective projection (reports minted, items appended,
  items completed per report) — does the loop converge or churn?
- `max_attempts=1` (or `--status-max-attempts`) for status tasks — kill the
  expensive whole-agent retry on verify misses (needs the retry-economics
  owner answer)
- Tell the status agent to pre-run `.tq-verify` before committing
- Status-report index page in the web UI (queue-side "all reports ever"
  from status-task facts)
- `StatusResult.Commit`: record the report commit SHA the agent already
  emits via `TQ_RESULT`
- Cross-repo status aggregation
- Zero-finding review approvals passed through mechanically (no second
  agent run) — needs loop-safety analysis, owner policy
- `--status-every` per-project overrides (`project=N,other=M`)

### Web UI polish

- Mobile/narrow-viewport pass (band wrap, table overflow, actions column)
- Dark-mode visual verification (working headless-dark profile) — shipped
  unverified
- Browser-driven e2e for the write forms (playwright shell is already in
  the nix store); deterministic screenshot harness (seeded fixture DB,
  light/dark/desktop/mobile) + nightly screenshot-diff job (would have
  caught the ghosted-app.js regression)
- Stop-request surfacing: a running task with a pending cancel request
  should SHOW it in the table (only the fact feed knows today)
- Write-action audit strip (last N cancels/rescues with reasons) on the
  dashboard; bulk rescue-all from the DLQ section (CLI parity)
- Cookie session hardening: `Max-Age`, rotation on token change,
  revocation story (write-endpoint rate limiting SHIPPED 2026-09-10)
- `Task-Queue-ID` footer awareness: detail page could show the originating
  TODO item text (data exists in the payload)
- Board follow-ups: per-project swimlanes, WIP highlighting, verdict badges
  on cards, keyboard `5` toggle, view-preserving `statusHref`,
  `/api/board` JSON projection, drag-and-drop behind the writes gate
- Fact feed pause + jump-to-now; copy-task-id buttons; budget as a progress
  bar; distinct lease-owner count in the band
- `tq serve --open` (browser auto-open); SSE `retry:` hint; humanized
  payload preview in rows; inline-SVG favicon
- Consolidate the status-color maps and shared empty-state copy — DONE
  2026-09-10 (one `statusColorTable` + shared empty-state helper)
- `?since=` URL param on the dashboard table (the `tq tasks --since` twin —
  the filter exists CLI-side only today)
- Define the stats payload once — text, `--json`, and webui budget card are
  three renderers of the same projection drifting independently

### Fleet (project-discovery + overview + SystemNix)

- Multi-model fleet: 2–3 pools with distinct `--model` sharing one DB, or a
  `--repo-model` flag (model pinned per repo at harvest) — owner decision
- Fleet onboarding runbook + smoke script (rails, triage, staleness
  screen, budget); fleet liveness `tq doctor` check (which repos have
  rails, which pools cover them)
- Budget aggregation across pools sharing one DB (`tq stats --fleet`)
- Per-repo verify-cost documentation (overview's nix-builds-in-tests is the
  most expensive verify in the fleet); consider `.tq-verify` fast/slow
  split
- Untracked-rails guard: `tq doctor` warns when `.crushrc`/`.tq-verify`
  exist but are uncommitted
- Resolve the sdk TODO split brain (queue-facing checkboxes vs status
  tables — drift guard or full migration)
- Web UI: project filter dropdown sourced from harvested projects

### CI / tooling

- `concurrency:` group in ci.yml to cancel superseded runs (the
  auto-commit daemon pushes in bursts)
- Advisory lint cost: scope to changed packages (diff-based) or move to a
  scheduled job instead of recomputing a known ~400-finding result per push
- ~~govulncheck step (binary already in the flake devShell); dependabot for
  actions + modules~~ DONE 2026-09-10 (advisory CI jobs + .github/dependabot.yml);
  upgrade pinned actions past the Node 20 deprecation
- Nightly `-race -count=3` full-suite job (flake-catching for the race
  gate); CI split (full `-race` suite takes >7 min)
- User-facing string corpus snapshot test: extract all flag help + error
  strings and freeze them against mechanical edits (the `task(store)`
  help-corruption class; 08:42 report f25)
- Fuzz `unwrapCommand` payload shapes (raw/JSON string/`{"cmd":...}`/
  hostile input); rotate nightly fuzz targets per-day-of-week
- Sentinel errors per package (`errors.go` convention) to burn down the
  err113 findings — post lint-endgame decision
- ~~Triage the 32 gosec findings: real issues vs false positives~~ DONE
  2026-09-10 (48 findings over all modules triaged FP/by-design; encode the
  triage as config — TODO_LIST)
- Detail-page follow-up ideas (2026-09-11 redesign residue): structured
  rendering of the prompt contract's numbered items; "+N more" cap on the
  retry-strip reasons; `Fact.Attempt` parity in the dashboard fact feed;
  retry-cause analytics; attempt heatmap; payload diff view; j/k navigation
  and copy-task-id buttons on the detail page
- templ LSP false diagnostics against a green `go build`: investigate
  gopls/templ-lsp coexistence; until fixed, "LSP webui diagnostics are
  false positives, trust the CLI"
- Windows smoke variant of `webui.sh` (currently POSIX-only)
- Skip-summary line for env-gated suites (postgres, baseline) so a silent
  skip never reads as "ok"; `TestPostgresConformance` in ci-local behind
  `TQ_CI_POSTGRES=1`
- Status-index check as a git pre-commit hook (catch unindexed reports at
  write time, not gate time)
- CLI polish pack: `printTaskList` error-width flag (60-char truncation is
  hardcoded), `tq tasks --json` pagination hint field, `--prune-stale --json`
  naming parity with `tq audit --json`
- Consolidate the excerpt helpers (`truncate`, `firstLine`, `oneLine`,
  `truncateItem`, `tailBytes` families) into one tested util per package

### Observability / ops

- dlq rescue UX: print the rescue plan before enqueueing on
  `--rescue-all --older-than`
- `tq doctor`: prune-stale dry-run hint when stale pending tasks exist;
  warn when a sweeper cursor exists but zero tasks completed despite N
  windows (silent-mint failure signal); pool fleet liveness check
- `tq watermarks show --json`; `tq top`: budget spend + in-flight reviews
- `tq show --yaml`/`--template` for scripting over result details
- `doctorWatermarkLiveness` lag-threshold knob (WARN at lag > K)
- Postgres `Fail` labels its exhausted class `transient` where SQLite says
  `exhausted` — pre-existing divergence, documented by the conformance
  battery
- Catch-up tasks with a cheaper executor: try `sh`+python vs agent for `tq
  audit` repairs
- Promote remaining shell smokes to Go e2e tests (multi-repo smoke →
  `internal/e2e`)
- Docs: DOMAIN_LANGUAGE.md entries for window, mint, trigger, done prompt

### Multi-agent process (this repo's reality)

- Auto-commit daemon: build-gate (or red-tag) before committing — it
  committed broken intermediates repeatedly; consider skipping files an
  agent just committed so `TQ_RESULT.commit_sha` attribution lands on
  feature commits
- Concurrent-agent file-ownership/lock convention (per-file claims in
  TODO_LIST, a `/.wip/` lock dir, or daemon-tagged in-flight files) —
  four collisions in one package in one session is pure luck, not design
- Agent prompt rule: code + tests first, changelog/TODO closure last,
  after verify is green; "if the item is already done in code, close it as
  stale" (the honest-loop behavior one trial agent showed — pin it)
- Mid-session build/test checkpoints in multi-agent mode ("green 30
  minutes ago" is meaningless); run the full gate at every "done", not at
  session end
- `.crushrc` managed-block migration for this repo (old unmarked single
  line → deliberate bootstrap re-run with reviewed diff)
- Per-pool sidecar subdirectory namespacing (concurrent pools share the
  default log dir; orphan-ID files observed)

## Non-goals

- Becoming a general-purpose workflow engine — task queue + pool, not orchestration
- Replacing go-cqrs-lite or PapDashboard — compose with them, never absorb them
- GPU scheduling / compute placement

## Open questions (owner decisions)

- ~~Should the pool be allowed to work on go-taskqueue itself?~~ ANSWERED
  2026-09-07 (owner): the pool runs on this repo — `.crushrc` (minimum
  autonomy) + `.tq-verify` (CI hard gates) are the rails; see
  `docs/planning/2026-09-07_20-47_SUPERB-PLAN-ROUND4-DOGFOOD-POOL-EATS-THIS-REPO.md`.
- What cost ceiling applies to a first production run (per day, per repo)?
- Review budget accounting: every completion costs 2 enqueues (work +
  review) against ONE cap, halving the effective work ceiling — exempt
  reviews, separate cap, or keep the conservative double-count?
- Standing budget level: the round-9 window raised 15 → 30 autonomously —
  ratify 30, revert, or wire `--budget-cmd` to real provider cost?
- Writes default policy: should `--allow-writes` auto-enable for
  loopback-only binds and stay opt-in just for LAN, or always be explicit?
- Pool × interactive-session coexistence: stay up through dirty-tree
  windows (requeue-and-wait) or pause during interactive sessions?
- Cancelled-task dedup semantics: today a cancelled task's dedup key
  suppresses re-enqueue forever (escape hatch: edit the item text). Should
  cancellation instead release the key? Store-schema-affecting.
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
- Board as the default landing projection, or opt-in per URL?
- Sibling-collision policy: gap-fill obviously-intended symbols of a
  concurrent session, or strictly hands-off + wait/report?
- Toolchain policy (16-00 report g1): bump the whole repo to go 1.27
  (go.mods + setup-go pin together) in the next release window — json/v2
  stable without the experiment — or stay pinned on 1.26.7 until the
  rate-limit fix bakes in production?
- Concurrent-writer protocol (16-00 report g3): are parallel agent sessions
  on this repo intentional/budgeted, or should the pool pause during
  owner-directed windows? Two edits were clobbered by whole-file writers on
  2026-09-11

## Deferred-bundle seeds

Raw ideas with enough shape to act on live in
[docs/planning/2026-09-06_deferred-bundle-seeds.md](docs/planning/2026-09-06_deferred-bundle-seeds.md):
the internal→public promotion order (D82), cron recurring tasks via
time-bucketed dedup keys (D83), session chains (D94), cross-repo DAG
templates (D97), the AI prioritizer hook (D98), retry-policy table (D99),
fencing-token design note (D100), and DB rotation/backup guidance (D96).
D80 (Postgres store), D90 (per-repo timeouts, shipped as `--repo-timeout`)
and D91 (agent-binary version probe) shipped 2026-09-08 — the seeds file
carries their ✅ stamps. The v0.3 design pack lives in
`docs/planning/2026-09-08_round5-m23-feature-designs.md`.

## go-cqrs-lite deferred tiers (ADR-0014)

The journal adapter (`internal/journal/cqrs`, 2026-09-12) is the adopted
seam; deeper integration ideas, in rough order of expected value:

- Per-task-stream `event.EventSource` on the adapter (`Load`/`LoadFromVersion`
  per task): facts(task_id, seq) is already indexed; needs a per-task
  ordinal version contract and a use case that wants stream replay.
- Watermill `CatchUpSubscriber` over the tq journal for cross-process
  projection hosts — requires a broker decision (tq is deliberately
  single-binary, zero external services; ADR-0009's consumer already
  covers in-process delivery).
- Branded task IDs via go-cqrs-lite `id/v4`: a cross-module earthquake
  (every sub-module, the CLI, the web UI, session/footer formats) with no
  interop payoff — only revisit alongside the internal→public promotion.
- go-cqrs-lite `middleware`/`metadata` tracing IDs on facts (correlation
  across the bridge boundaries): only if PapDashboard ingest grows
  multi-hop causality needs.
