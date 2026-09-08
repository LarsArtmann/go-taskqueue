# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added

- **Automated done-prompt loop — `tq agent-pool --status-every N`**
  (2026-09-08): every N completed agent tasks per project mint ONE `status`
  task (watermarked `status-sweeper`, same cursor semantics as the review
  sweeper; the window is derived from queue state, so restarts never
  desynchronize it). The status agent runs the done prompt: a full status
  report at `docs/status/<YYYY-MM-DD_HH-MM>_<name>.md` (done / partial /
  not started / broken / improvements / up to 50 next items / up to 3
  questions) AND appends the next items to the repo's TODO_LIST.md —
  questions as `— BLOCKED:` items the harvester skips until a human answers
  — closing the loop back into harvest. Mechanical contract:
  `TQ_RESULT: {"report":"...","next_items":N}` naming an existing,
  repo-relative report file (path escapes refused; misses are retryable).
  Loop safety: only `agent` completions count (status tasks never report on
  themselves), one report in flight per project, every enqueue
  budget-gated. The executor is registered in every agent-capable pool
  (`agent-pool`, `tq worker --agents`), so pools without the flag still
  carry status tasks minted elsewhere. Ops: `tq watermarks show/set
  status-sweeper`.
- **Journal-consumer watermarks** (2026-09-08): a `watermarks` side table
  gives every journal consumer a durable cursor — `Store.Watermark` /
  `SaveWatermark` with a monotonic upsert (a save can never lower the
  stored seq; SQLite and Postgres), admin `ListWatermarks`/`SetWatermark`
  (forced rewind) kept off the interface. The papdashboard bridge
  checkpoints after each accepted batch (a pending checkpoint gates
  forwarding; a checkpoint failure is a forward failure and heals on
  restart), `startWatermark` resolves FromSeq > persisted > head so first
  runs bootstrap at head instead of replaying history, and the volatile
  `alerted` map is gone — `alert.resolved` correlation derives from the
  task's fact trail via `FactsForTask`, so resolve-after-restart works.
  The review and status sweepers checkpoint the same way
  (`review-sweeper`/`status-sweeper`), so pool-downtime gaps are caught
  up instead of skipped. Pinned by a restart battery: zero-loss
  mid-stream restart with identical idempotency keys,
  checkpoint-failure gating, FromSeq precedence, resolve-after-restart
  (`internal/bridge/papdashboard/restart_test.go`). Ops: `tq watermarks
  show/set` (`show` prints lag vs head; `set CONSUMER SEQ` is the
  deliberate re-delivery escape hatch after a fix).
- **Journal subscription policy (ADR-0009) + `internal/consumer`
  dispatcher** (2026-09-08): consumer classes now have decided contracts
  — exact consumers (bridges, workers) require in-order at-least-once
  delivery with block-not-drop slow-consumer semantics; signal consumers
  (dashboards) may skip. The dispatcher polls `Store` bounded reads with
  a per-subscriber cursor (deliberately OFF the `Store` interface;
  notify-after-commit is the v2 upgrade path), and `journal.Journal` is
  demoted to a test double. Compaction readiness is codified: a consumer
  resyncs loudly from the retention floor — never from a seq-gap guess.
  Observability: `tq stats` gained a consumer-lag table (watermark vs
  `HeadSeq`) and `tq serve` logs reconnect lag (`head − Last-Event-ID`).
- **Actor rollout (`internal/runactor`)** (2026-09-08): `serve`,
  `worker` and `agent-pool` are composed from a run.Group (errgroup +
  CancelCause): named actors with first-exit cancel, LIFO `OnShutdown`
  teardown (the store is registered first so it closes last), and
  `InterruptOn` (second signal exits 130). Task execution contexts are
  detached via `ExecutionScope` (`WithTimeout(WithoutCancel(parent))`)
  and stay bounded only by `--task-timeout` — a Ctrl-C still lets
  in-flight agents finish and record their outcome. One deliberate
  behavior change: a bridge startup failure now fails the command
  (exit 1) instead of running bridge-less. Pinned by a real-binary
  SIGTERM e2e (`internal/e2e/shutdown_test.go`): the in-flight claimed
  task completes, the bridge watermark is ≥ the dead-letter fact seq,
  and SSE closes before exit 0.
- **Review verdicts in the web UI** (2026-09-08): completed review tasks
  surface their outcome — an approve/request_changes badge in the task
  table's status cell and a findings card (title + severity) on the task
  detail page, read best-effort from the completion fact; pages without
  finished reviews render exactly as before
  (`TestReviewVerdictBadgeAndFindings`).
- **Status-loop hardening** (2026-09-08): the `status` executor now runs
  the repo verify gate after the report contract (`StatusPayload.Verify`,
  resolved `.tq-verify` file → payload → auto-detect like agent tasks —
  the reporter commits, so a broken tree fails the attempt), and the done
  prompt states a hard scope rule (report + append-only TODO_LIST.md +
  own commit; report-don't-fix). Minted payloads are richer and more
  robust: every window entry carries its own commit + files (read from
  the task's completion fact) and the raw TODO_LIST item
  (`AgentPayload.Item`, pinned by the harvester) instead of the prompt
  template's first line; the sweeper propagates the pool's
  `--allow-dirty` stance and `--task-timeout` budget into the payload
  (`RequireClean`/`TimeoutMinutes`), so real repos — effectively always
  dirty under concurrent agents — are not deadlocked by the preflight or
  starved by the 15m executor default. End-to-end coverage:
  `scripts/smoke/status-loop.sh` (stub-agent: mint → status run → report
  → TODO_LIST append → harvest re-arm → dedup holds), wired into
  `ci-local.sh`.
- **Status results on every ops surface** (2026-09-08): completed status
  tasks render a "status: report +N next" badge in the web UI table and
  a report card (next-items badge, repo-relative report path, log path)
  on the detail page (`TestStatusResultBadgeAndCard`); `tq show` decodes
  the completion detail into a typed `result` (AgentResult /
  ReviewResult / StatusResult by task type); `tq doctor` checks
  `review-sweeper`/`status-sweeper` watermark liveness — a cursor behind
  the journal head warns that the sweeper is not running
  (`doctorWatermarkLiveness`).
- **Sidecar retention** (2026-09-08): `tq agent-pool --log-dir-max-age`
  (env `TQ_LOG_DIR_MAX_AGE`, default off) sweeps `TQ_LOG_DIR` of `*.log`
  output logs older than the age once per tick — live tasks' logs are
  kept and non-`.log` files are never touched (`executor.SweepSidecars`).
- **`tq bootstrap` — one command from zero to a running agent pool** (2026-09-08):
  per repo it validates the checkout, pins the verify contract into
  `.tq-verify` (auto-detected or `--verify name=cmd`), writes a managed
  autonomy+model block into the repo's `.crushrc` (markers bracket the block;
  user content outside is never touched; `--model` + `--reasoning xhigh`
  default pins the provider/model for unattended runs), commits exactly those
  files (never pushes), previews the harvest, then delegates to
  `tq agent-pool` — or `--install` renders the systemd user unit +
  `~/.config/tq/pool.conf` and enables linger, or `--no-run`/`--dry-run`
  exit after ensuring. Idempotent by design.
- **Full output sidecars**: `tq agent-pool --log-dir DIR` (or a
  `log-dir =` config key, or `$TQ_LOG_DIR`; flag > env > file) writes each
  task's complete agent + verify output to `DIR/<task-id>.log` (0600) and
  records the path in the result detail. `tq bootstrap` enables it by
  default at `~/.local/state/tq/logs` — daemon pools need their logs.
- **Reasoning-effort fix (bootstrap)**: bootstrap no longer composes
  `--model` into the pool args or pool.conf — a payload model makes the
  executor pass `crush run -m`, which resets reasoning effort to the
  provider default (verified via crush debug telemetry: with `-m` the
  effort telemetry is empty, without it the `.crushrc` slot's effort
  applies). The repo `.crushrc` managed block (model +
  `--reasoning-effort xhigh` by default) is the single carrier; agent runs
  and interactive crush both honor it.
- **Wave-4 slices**: `tq version` reports the build (ldflags-injected
  release version in nix builds, VCS revision from build info in
  go-builds); the `sh` executor gained resource guards
  (`CommandExecutor.MemoryLimitMB` via ulimit, `Nice` for scheduling
  priority - POSIX-wrapped before exec so limits bind the payload's
  whole tree); an e2e pins `tq audit --json` and `tq top --json` against
  a seeded database; and docs/planning/2026-09-08_round5-m23-feature-designs.md
  records the v0.3 design sketches (cron via time-bucketed dedup keys,
  per-repo budgets as a grouped fact projection, retry-policy payloads,
  DAG templates as enqueue sugar, session chains as a payload convention,
  generalized project concurrency, webhooks and /metrics as bridges).
- **Production write API (ADR-0008)**: `tq api` serves non-Go producers
  with POST /api/v1/tasks, GET /api/v1/stats, and GET /api/v1/healthz
  behind a MANDATORY bearer token (no loopback exemption - this surface
  exists to be exposed). Validation errors are actionable JSON
  {error, fix} documents, request bodies are capped at 1 MiB, dedup keys
  ride through for webhook-style at-least-once producers, and the
  payload passes verbatim to the executor contract. The ADR also records
  the fencing-token design (lease generations; deferred until a real
  double-write makes the observability worth the schema change) and the
  consumer-group claim-path sketch (a WHERE clause, not a router).
- **Postgres store, first slice (ADR-0007)**: `queue.OpenPostgres` is a
  semantic twin of the SQLite store over jackc/pgx (pure Go, CGO stays
  off) - same tables, same facts-in-transaction invariant, same dedup and
  cancel semantics, with claims via SELECT ... FOR UPDATE SKIP LOCKED so
  workers across machines lock disjoint rows instead of queueing behind
  one writer. Verified against a real cluster: a lifecycle conformance
  test (enqueue/dedup/claim/lease/cancel/retry/dead-letter/rescue/orphan/
  facts), a parallel-claim exclusivity hammer (no task claimed twice),
  and an env-gated baseline: claim+complete ~838/s at 1k depth vs
  SQLite's ~234/s at 10k depth (re-measure at equal depth before quoting
  ratios). CI grows a postgres:16 service job; the SQLite suite stays
  the default gate.
- **Web UI a11y + QA pack**: a skip-to-content link (screen-reader first
  tab stop), the connection lamp is now a polite live region
  (role=status) so connection loss is announced, and a test pins the
  a11y chrome plus the committed CSS's prefers-reduced-motion handling.
  scripts/webui-screenshots.sh renders light + dark + detail pages via
  headless chromium (fails clearly where no browser exists - this host
  class has none), and scripts/check-webui-css.sh guards the committed
  stylesheet against tailwind-rebuild drift.
- **Dashboard metrics + journal browser**: the overview gains a journal
  watermark stat card (the seq every SSE/bridge consumer resumes from)
  and two pure-SVG charts - a fact-rate sparkline (facts per 5-minute
  slice of the last hour) and a time-to-complete histogram (queue wait +
  run, honestly labeled, bucketed 1m/5m/15m/60m). GET /api/facts?after=
  pages forward through the whole journal in seq order, and a collapsible
  browser under the live feed uses it to scroll through history (lazy,
  no-JS safe). The M13 adoption guard caught AreaChart being undocumented
  within one test run - the table is updated.
- **Dashboard interactions pack**: the task table's age and attempts
  columns are server-side sortable (allowlisted ORDER BY pushdown; header
  links cycle none -> desc -> asc -> none, aria-sort announced); project
  names link to /project/{name}, a shareable dashboard pinned to one
  project through the same filter/sort/pagination pipeline; every fact
  line in the journal feed links to its task's detail page (a thin
  linked variant of the library Scrollback, which renders plain text
  only); truncated error cells expand on click; relative ages keep
  ticking between SSE bursts via data-age attributes; and "?" opens a
  keyboard-shortcut overlay (built via the CSSOM to stay inside the
  strict CSP).
- **Journal compaction design + prototype (ADR-0006)**: the facts log
  gains a hot-cold split - `SQLiteStore.ArchiveFactsBefore` moves the
  fact trails of TERMINAL tasks (whole trail or nothing, never splitting
  a task) into `facts_archive` in one transaction, records the high-water
  mark in `journal_meta`, and `ArchiveSummary` reports hot/archived
  counts. Projections are untouched: a test proves tasks, `Facts()`, and
  the watermark all survive archiving, and that a second pass moves
  nothing. Deletion and seq-granular splits were rejected in the ADR;
  the `tq journal compact --before` command sketch rides with it.
- **Queue health pack**: stranded Running tasks (expired lease, nobody
  reclaimed) can now be recorded in the journal - `tq doctor
  --mark-orphans` appends an idempotent `task.orphaned` fact per task
  (owner + lease-expiry detail) without touching task state, and the
  stuck-queue check points at the flag. The heartbeat cadence contract is
  pinned by a test (default lease/4, tighter than the planned lease/3,
  configurable via Config.Heartbeat). A multi-process contention e2e
  drives two worker processes plus a facts reader against one DB and
  proves exactly-once completion. A 10k-op baseline test measures SQLite
  throughput for the record: enqueue ~28.5k tasks/s, claim+complete
  ~234/s at 10k pending depth (claim cost grows with queue depth - the
  number to beat in the Postgres slice).
- **Pool ops pack**: `--max-concurrent-agents N` caps agent processes
  MACHINE-WIDE across every tq pool on the host (flock'd slot files in
  $TMPDIR/tq-agent-slots; a SIGKILLed pool releases its slots via the
  kernel - verified by a serialization test); `--repo-timeout
  name=duration` pins per-repo agent-task ceilings into harvested
  payloads (big repos get long ladders, quick ones stay tight); and the
  pool probes the agent binary's `--version` at startup so a missing
  crush is a warning before the first task, not a dead-letter after it.
  A new chaos test SIGKILLs an `agent-pool --once` mid-drain and proves
  the restarted pool reclaims the expired lease, finishes the work, and
  leaves exactly one completion in the journal. (Model pin and daily
  budget VALUE choices stay owner-gated; the propagation mechanisms are
  covered by flag and payload tests.)
- **Dogfood ops pack**: `tq agent-pool` can now forward alerts
  (`--alert-url`/`--alert-api-key`, env `TQ_PAP_URL`/`TQ_PAP_API_KEY`) -
  previously only `tq worker` could. The PapDashboard bridge additionally
  mirrors `--daily-budget`: the day the cap is reached exactly one
  warning alert fires (synthetic aggregate `agent-pool-budget-<date>`,
  idempotent by fact seq) and the first enqueue of the next day resolves
  it. `scripts/tq-session-status.sh` prints every live pool/worker/serve
  process with uptime, DB path, serve addr, and journal freshness. An
  audit of the dogfooding agent commits found no self-modification
  violations and green verify tails on all recent completions
  (docs/status/2026-09-08_16-20_round5-m14-dogfood-ops.md).
- **Hygiene pack**: the JetBrains Mono font subsets now ship with their
  SIL OFL 1.1 license (`internal/webui/static/fonts/OFL.txt`); a guard
  test cross-checks the AGENTS.md templ-components adoption table against
  the actual template sources in both directions (its first run caught
  `ThemeScript` listed but unused, and nine icons in use but unlisted);
  table tests pin every webui mapping helper (status badges, fact tones,
  id tails, timestamps, detail items, status hrefs, row classes); the
  duplicated harvest no-repos error became the `harvest.ErrNoRepos`
  sentinel, empty-type enqueue failures became `queue.ErrEmptyType`, and
  the dead `factBadgeClass` helper was removed.
- **Release runner**: `scripts/release.sh vX.Y.Z [--tag|--push]`
  codifies the v0.1.0 release checklist into one command - the full
  ci-local gate suite, go.mod hygiene (no replace/pseudo-versions),
  CHANGELOG-section checks, an annotated tag cut from the CHANGELOG,
  module-proxy + clean-room `go get` verification, and a pre-release
  GitHub Release. The safe default runs the gates only; pushing and
  publishing stay behind explicit flags. The docs also got a truth pass:
  FEATURES gained design-system, Windows-CI, and pool `--config` rows,
  and CONTRIBUTING documents the committed-web-CSS build step.
- **Platform honesty for Windows + the nix sandbox**: POSIX-only test
  suites now carry `//go:build unix` (agent/review shell-stub suites, the
  worker stub-agent test split into its own file) and `internal/e2e` grew
  a `!unix` placeholder so `go test ./...` stays green on Windows. CI
  gained a `test-windows` job that runs the remaining suites on
  `windows-latest` (POSIX suites drop out via the tags; no `-race` there,
  race coverage stays on Linux). `nix flake check --all-systems` passes
  at eval time for all pinned systems, and the release smoke now runs the
  nix-built binary through `scripts/smoke/webui.sh`. The nix sandbox also
  caught a hermeticity bug: `tq doctor`'s healthy-DB test implicitly
  required `crush` on PATH and now points the agent-binary check at the
  test binary itself.
- **`tq doctor`** answers "why is nothing happening?" in one command:
  SQLite integrity + WAL mode, queue mix with expired-lease detection,
  worker liveness (no heartbeats while work waits = FAIL), budget spend
  against `--daily-budget`, the agent binary on PATH, and per-repo
  autonomy files via `--repos`. Human table by default, `--json` for
  scripts; exit 1 on failing checks so cron/systemd can alert on it.
- **Cooperative cancel of running tasks** (`tq cancel --force`,
  ADR-0005): the request is a `task.cancel-requested` fact — the journal
  is the flag, no schema change. The executing worker observes it at its
  next heartbeat, kills the whole process tree (the `sh` executor now
  sets a process group like the agent one), and finalizes Running ->
  Cancelled without burning the attempt. A crashed worker's expired
  lease finalizes the cancel at reclaim instead of re-executing the
  task. Plain `tq cancel` on a running task refuses with the --force
  remedy. Verified mid-run by an e2e subprocess test (30s sleep stopped
  within a heartbeat) plus store and worker race tests.
- **Live task detail pages**: `/task/{id}` now updates in place over a
  task-scoped SSE stream (`GET /task/{id}/events`). The record card
  (`#frag-detail`) and fact timeline (`#frag-timeline`) re-render on every
  journal burst using the same subscribe → snapshot → tick protocol and
  watermark-id resume semantics as the dashboard stream; the connection
  lamp and no-JS fallback carry over unchanged. Unknown task ids get a
  404 before the stream opens. `app.js` routes detail pages to the
  task-scoped endpoint (token still rides the query for EventSource).
- **Budget + retry visibility in the dashboard** (`tq serve`): a budget
  stat card ("spent/cap" for today, green/amber/red by 75%/100% of the
  daily cap) appears whenever the pool runs with `--daily-budget`, and
  the task table gains a "ready" column showing when a pending task
  becomes claimable ("in 12m", "ready", empty when immediately
  claimable; exact `notBefore` on the tooltip).
- **Agent reviews**: `tq agent-pool --review` gives every completed agent
  task one review by a second headless agent (`internal/review` sweeper +
  a `review` executor in `internal/executor`). The reviewer reads the
  original item, the reported commit and changed files, and must end its
  output with a parseable `TQ_RESULT` verdict (`approve` or
  `request_changes` with actionable findings) — the verdict lands in the
  completion fact detail, visible via `tq show`. `--review-autofix` mints
  a deduped agent fix task per finding, closing the loop. Loop safety is
  structural (reviews are never reviewed; one review per task via dedup)
  plus budgetary (review enqueues count against `--daily-budget`).
  Reviewed end-to-end by a stub-agent smoke: agent → review
  (request_changes) → fix task → re-review (approve) → clean `--once`
  drain.

### Changed

- The dashboard task table paginates in SQL: `?page=` (clamped, 200 rows
  per page) with a prev/next pager and a "page N of M — K matching tasks"
  line driven by a new `Store.CountTasks` pushdown that shares the exact
  WHERE builder with List. Table order moved into SQL as
  `Filter.SeverityOrder` (dead, running, pending, cancelled, completed;
  newest first within a status), so page boundaries are deterministic and
  the in-memory re-sort is gone. Measured at 100k tasks + 100k facts:
  page query 18ms, count 1.5ms, LIKE search count 36ms, fact feed 0.2ms,
  per-task trail 0.05ms — pinned by `TestLoadSnapshotScaleAt100k`
  (skipped under -short and -race) with ceilings that fail on any O(N)
  regression.

- The dashboard now sends strict security headers on every response,
  including auth rejections: `Content-Security-Policy` (default-src 'none',
  self-only styles/scripts, no inline, no framing, no form action),
  `Referrer-Policy: no-referrer`, `X-Content-Type-Options: nosniff` and
  `X-Frame-Options: DENY`. The HTTP route table became data
  (`routeBindings`) so a new read-only guardrail test fails the build the
  moment anyone registers a mutating handler — the dashboard's "stale
  dashboard, never journal corruption" guarantee is now enforced, not
  aspirational. ADR-0003 records the headers and the Phase D
  `--allow-writes` pre-design (capability flag + token-on-loopback +
  per-route CSRF) for whenever writes are seriously proposed. The webui
  smoke script asserts the headers.

- The agent pool is now a first-class service: `tq agent-pool --config
  <file>` (or `$TQ_POOL_CONFIG`) loads flat `key=value` settings with
  flag-name keys applied to every flag not given explicitly — precedence
  flag > environment > file > default, unknown keys fail loudly. The
  shipped `deploy/systemd/tq-agent-pool.service` unit gained a
  conservative hardening profile (NoNewPrivileges, ProtectSystem=full,
  kernel/cgroup namespace isolation, RestrictSUIDSGID, LockPersonality)
  that still lets agents write repos and reach the network, plus
  install/linger instructions so the pool survives reboots instead of
  dying with the terminal session.

- Task search is a SQL filter, not a scan: `queue.Filter` gained `Query`
  (case-insensitive substring over id, type, project, payload, lease
  owner, last error — pushed into escaped SQL LIKE) and `Offset`
  (pagination), and the dashboard's status counters and per-project
  overview chips now come from two GROUP BY projections
  (`Store.StatusCounts` / `Store.ProjectCounts`) instead of walking
  every task in memory per burst. `%`, `_` and `\` in search boxes now
  match literally.

- Every journal read is now bounded or cursor-based, so the dashboard,
  `tq top`, the papdashboard bridge and the daily-budget guard stay O(1)
  per tick as the journal grows past 100k facts. `queue.Store.Facts` takes
  a limit, `LastFacts` feeds the webui fact feed (last 50, not the whole
  journal per 500ms burst), `HeadSeq` resolves tailer/bridge watermarks
  without loading history, `FactsForTask` serves task detail trails via
  the (task_id, seq) index, and `CountFacts` pushes the budget
  spent-today count into SQL. `tq show <id>` uses the indexed trail read;
  `tq top` frames read the most recent 5,000 facts. The papdashboard
  bridge drains backlogs in batches of 500 while keeping its
  retry-unsafe-forward guarantee (a failed forward still stops the drain
  and retries from the last forwarded seq).

- All three agent prompts (harvest work item, drift catch-up, cqa fix task)
  now teach the `TQ_RESULT:` self-report line, so pool agents fill in
  `files_changed`/`commit_sha` in `tq show` instead of leaving the structured
  result empty; every prompt also forbids editing `.crushrc`/`.tq-verify`
  (self-modifying autonomy was an accepted risk in the dogfood plan), grants
  explicit commit permission, and the cqa prompt forbids scanner-gaming
  (weakened tests, blanket suppressions). Contract-pinning tests parse the
  taught `TQ_RESULT:` example with the executor's real parser, so prompt and
  parser cannot drift silently.
- The CI lint step (golangci-lint) is advisory (`continue-on-error`): the
  enabled linter set carries a ~400-finding repo-wide baseline that kept
  master permanently red and buried real signal. The full run is log-only;
  the hard gates are vet, gofmt, tests, the web UI smoke and the
  doc-reference check. Generated `*_templ.go` output is excluded
  from lint and from the treefmt/gofumpt gate (`templ fmt` owns the
  `.templ` sources); CLI dispatch (`main`), `cmdAudit` and the papdashboard
  watermark startup were cleaned up where the baseline pointed at real
  complexity/context bugs.

- ADR-0004's cordis adoption trigger T3 ("maturity verified locally") is
  now evidence-backed instead of vendor-claimed: the cordis Go port's test
  suite ran locally in the fork at the exact commit the ADR assessed
  (`61ec9f9`) — 5/5 packages race-clean under `-race`, 86.2% total
  statement coverage, 176 PASS / 0 FAIL / 0 SKIP (`loader` weakest at
  74.6%). The framework-free verdict is unchanged: gates T1/T2/T4/T5
  remain open. Record and reproducibility commands:
  `docs/planning/2026-09-08_cordis-test-suite-verification.md`.

### Fixed

- `scripts/smoke/papdashboard-e2e.sh` real-dashboard mode no longer
collides with a locally running pap-raw-server on the fixed port 18099:
the mode now derives an ephemeral port and liveness-checks the dashboard
before driving it.

### Added

- `tq serve --auth-token` / `$TQ_SERVE_TOKEN`: token auth for the
  dashboard, plan W16. Non-loopback binds are now default-deny —
  `webui.Config.Validate` (enforced by `Server.Run` and the CLI) refuses to
  start on addresses that bind beyond loopback (including the empty host
  `:port` and non-`localhost` hostnames) without a token, because the
  read-only dashboard still renders every task payload and error tail.
  With a token set, a constant-time middleware guards every route (pages,
  `/api/*`, `/static/`, SSE): requests present it as `Authorization:
  Bearer <token>` or `?token=<token>` (EventSource cannot set headers, so
  the client JS forwards the page's `token` param to `/api/events`), and
  failures get a 401 with a `WWW-Authenticate: Bearer` challenge.
  `--verbose` access logs redact the `token` query parameter so the
  credential never lands in logs. Loopback serves without a token are
  unchanged (ADR-0003 amendment; smoke-asserted in
  `scripts/smoke/webui.sh`).
- Nightly fuzz job (`.github/workflows/fuzz.yml`): a 60s `FuzzParseRepo`
  campaign (`scripts/fuzz/nightly.sh`, runnable locally with a custom
  fuzztime) whose coverage-interesting inputs are synced into the committed
  seed corpus under `internal/harvest/testdata/fuzz` and pushed back to
  master by the workflow — corpus growth no longer depends on session
  memory (every committed seed also runs as a test case on each
  `go test`). The initial batch: 165 seeds from ~2.2M executions. The
  script runs under a private `GOCACHE` because on shared-cache mounts the
  fuzz corpus never lands; a crasher found by a campaign stays a red job
  (content dumped to the log) and is never committed.
- `FuzzExtractResultPayload` (`internal/executor/result_fuzz_test.go`): the
  `TQ_RESULT:` regex + JSON decode parse fully untrusted agent output, so they
  now have a fuzz target (never panics, deterministic, `ok` implies a marker
  line) with a committed 183-input seed corpus under
  `internal/executor/testdata/fuzz/FuzzExtractResultPayload` — initial 60s
  campaign: ~6.3M execs, zero findings; every seed also runs as a test case
  on each `go test`.
- Lint annotations for new findings: golangci-lint v2 emits no GitHub
  annotation commands (the v1 `github-actions` output format is gone), so
  the advisory lint run never actually surfaced findings as annotations —
  and the ~400-finding baseline would exceed GitHub's 10-warnings-per-step
  cap anyway. A new CI step (`scripts/lint-annotations.sh`, mirrored in
  `ci-local.sh`) re-runs the same binary and config scoped to
  `--new-from-rev` and emits findings on changed lines as `::warning`
  annotations, so regressions a commit introduces show up on green runs
  (verified empirically on v2.13.2: default output produces zero annotation
  commands even with `GITHUB_ACTIONS=true`).
- `tq serve --verbose`: per-request access logging (method, path, status,
  duration) via slog at Info level on the default logger (stderr). The
  wrapper forwards `Flush`, so SSE streaming through it is unchanged; SSE
  connections log once, when the stream closes. Off by default.

- Live web dashboard: `tq serve` (default `127.0.0.1:8090`, read-only)
  renders status cards, a live task table, the DLQ, per-project chips and
  the fact feed as server-rendered fragments pushed over SSE. A single
  journal tailer coalesces change bursts; every client gets a full
  snapshot on connect and after each burst (reconnect-safe), with URL
  filters/search (`?project=&status=&q=`) and per-task detail pages at
  `/task/{id}` (`internal/webui`; decision record:
  [docs/adr/0003-web-ui-architecture.md](docs/adr/0003-web-ui-architecture.md),
  execution plan:
  [docs/planning/2026-09-07_16-25_SUPERB-PLAN-ROUND3-LIVE-WEB-UI.md](docs/planning/2026-09-07_16-25_SUPERB-PLAN-ROUND3-LIVE-WEB-UI.md);
  smoke: `scripts/smoke/webui.sh`).
- Deferred-bundle seeds (plan C27): structured result self-report from
  agents (`TQ_RESULT:` line → `files_changed`/`commit_sha` in the result
  detail), full-output sidecar logs (`TQ_LOG_DIR`), `tq harvest --json` and
  `--repo-subset` glob, `tq dlq --rescue-all --older-than`, a guard that
  refuses `--projects-dir /` or the home directory with remediation, and a
  PoC server (`examples/api`: enqueue endpoint, Prometheus `/metrics`, live
  stats page) next to the SSE stream PoC (`examples/sse`) and the
  PR-mode/worktree PoC scripts; sketches for the rest in
  `docs/planning/2026-09-06_deferred-bundle-seeds.md`
- Tooling policy decided and enforced: golangci-lint wired into CI
  (`.golangci.yml` with errcheck exclusions for idiomatic deferred Close and
  HTTP body/rows Close), dprint joins the flake devShell and the living docs
  are formatted with it; CONTRIBUTING lists all local gates
- Operator documentation set: `docs/DOMAIN_LANGUAGE.md` (the ubiquitous
  language: task, fact, claim, lease, tick, drift, catch-up, …), ADR-0002
  (agent-pool autonomy, pacing, budgets and drain semantics), and SECURITY.md
  (trust model, blast radius of agent `bash`, hardening checklist)
- The pool prints an autonomy warning at start under `--yolo`: agents may
  run shell commands unsandboxed per repo `.crushrc`, with pointers to the
  budget caps that bound the blast radius
- Windows/i18n hygiene: `GOOS=windows` build + vet is now a CI gate; golden
  dedup-key vectors pin non-ASCII item hashing (CJK, emoji, Unicode
  whitespace collapsing) and keys are proven independent of path spelling,
  so harvesting a repo via relative or absolute paths never double-enqueues
- Docs-drift auditor (`tq audit`): compares repos' TODO_LIST.md checkboxes
  with terminal task states and repairs the drift harvest can't see — work
  an agent completed but never ticked off gets a dedup-keyed catch-up task
  (armed once, re-audits never pile up); hand-ticked items with unfinished
  tasks are reported for the operator to cancel
- `tq top`: live per-project view over the journal — pending/running/done/
  dead counts, the last run duration and the active run's elapsed time
  (`--once`, `--json`, `--interval`; repaints on terminals)
- Task result detail: completed agent tasks record the crush session id and
  the verify output tail in the `task.completed` fact; `tq show TASK_ID`
  renders the task together with its full fact trail, so an operator can
  trace exactly what an agent did and how the work was proven
- End-to-end CLI suite (`internal/e2e`): builds the real `tq` binary and
  drives `agent-pool --once` as a subprocess with a stub agent — the full
  harvest → claim → work → verify → complete loop is proven from outside the
  process, in CI, at zero API cost
- Property test for harvest dedup keys (stable under whitespace reflow,
  distinct across repos with identical item text) and a fuzz harness for the
  TODO parser (CRLF, BOM, nesting — 1.8M executions, zero findings)
- Chaos test: a worker SIGKILLed mid-task is reclaimed via lease expiry and
  the work completes exactly once in the journal
- Cost ceilings for unattended pools: `--daily-budget` (max enqueues per
  calendar day, projected from the journal), `--budget-cmd` (your own
  accounting vetoes each tick), `--repo-interval` (per-repo enqueue gap),
  `--dlq-backoff` (pauses poisoned repos whose recent work all died)
- `tq agent-pool --once`: one harvest tick, drain, exit — cron/timer
  friendly, with a systemd user unit in `deploy/systemd/`
- Preflight refusals: `executor.PreflightError` + `Store.Requeue` — a dirty
  tree or missing autonomy config requeues a task WITHOUT burning an
  attempt (claimable again after a delay); the autonomy probe also accepts
  a user-global crush config
- Verify strategy: the repo's `.tq-verify` file wins over payload and
  auto-detection; auto-detection adds Makefile, flake.nix and Cargo repos;
  the harvester pins a repo's `.tq-verify` command into every payload
- `--model` on `tq harvest` / `tq agent-pool`: pin the crush model in every
  harvested agent payload
- The web dashboard (`tq serve`) got a full visual redesign on
  `github.com/larsartmann/templ-components` v1.14: steel-navy/cyan
  "ledger & lamp" theme (`internal/webui/theme.css`), JetBrains Mono
  identity face (embedded OFL woff2 subsets), tone-iconed stat cards that
  link into filtered views, library tables/badges/empty states, a restyled
  DLQ and fact feed, and a task detail page with definition list + error
  alert. Dark/light mode with a header toggle. New dev command
  `nix run .#webui-css` recompiles the Tailwind v4 stylesheet into the
  committed, embedded `internal/webui/static/app.css`. The SSE fragment
  architecture (ADR-0003) and all container/element id contracts are
  unchanged.

### Fixed

- A filtered dashboard view was clobbered by the next live tick: the client
  opened `/api/events` bare, so every SSE snapshot rendered the unfiltered
  page-1 table. The page's filter query (`project`, `status`, `q`, `page`)
  now rides the EventSource URL (merged with the auth `token` param), so
  live updates and reconnects restore the same filtered projection; the
  server already honored the params (`parseFilter`), pinned by
  `TestStreamSnapshotHonorsFilter`
- Items marked `— BLOCKED: <reason>` in a TODO_LIST.md are now skipped by
  the harvester instead of being enqueued as fresh agent tasks. The marker
  was part of the documented agent contract (the prompt tells agents to
  append it when they cannot finish an item) and of the backlog file's own
  header, but no code honored it — and because the marker changes the item
  text, it even re-armed a fresh dedup key on every blocked attempt
- `tq agent-pool --once` hung after draining: the pool stopped but the
  process waited for a signal, because `Start` only returns when the
  caller's context is done — the drain watcher now cancels it too
- Two processes opening a brand-new queue database raced the schema writes
  and the loser failed with SQLITE_BUSY; the migration now retries with a
  bounded backoff (always converges on `IF NOT EXISTS` DDL)
- A task claim racing shutdown logged a spurious "claim failed" error

## [0.1.0] - 2026-09-06

### Added

- **Agent pool**: `tq agent-pool` runs a self-managing loop — periodic
  harvest of repos' TODO_LIST.md backlogs into dedup-keyed `agent` tasks,
  worked by a pool of headless crush agents with an enforced verify gate
  (build + tests), clean-tree protection, per-repo pacing and cost caps
- `tq harvest` command: idempotent backlog ingestion (`--dry-run`,
  `--max-per-tick`, `--allow-dirty`)
- `agent` executor (`internal/executor/agent.go`): runs crush headlessly per
  task, process-group kill on timeout, output tails in errors
- Idempotent enqueue via `DedupKey` (partial unique index, legacy-DB
  migration) — repeated harvests never double-enqueue
- CQA bridge (`internal/bridge/cqa`): Code-Quality-Agent scan findings become
  per-file agent fix tasks (`--cqa-url` on `tq agent-pool`)
- PapDashboard bridge (`internal/bridge/papdashboard`): dead-lettered tasks
  raise alerts; rescues resolve them (`--alert-url` on `tq worker`)
- Permanent-vs-transient error classes: `executor.PermanentError` marks
  failures retrying can never fix (bad payload, missing repo, dirty tree,
  missing autonomy, unknown task type, non-zero sh exit, permanent HTTP
  statuses); the worker dead-letters them after ONE attempt instead of
  burning the retry budget — for agent tasks every retry is real money
- Store-level per-project claim exclusivity (`queue.WithProjectExclusivity`,
  `tq worker/agent-pool --project-exclusive`): across ALL pools and processes
  sharing the database, a project never has two running tasks at once — the
  per-repo serialization guarantee for multi-pool agent setups
- CI reliability bundle: nix build + flake check job, TODO_LIST
  harvest-parse guard, and a ghost-reference check that fails when a living
  doc cites a repo path that does not exist

### Changed

- `tq enqueue --type sh` accepts a raw shell line as `--payload` (previously
  JSON-only, contradicting the README quickstart): non-JSON payloads are
  wrapped as JSON strings; the `sh` executor unwraps all three payload shapes
  (raw text, JSON string, `{"cmd":...}`)
- Agent autonomy is granted per-repo, not per-flag: `crush run` has no
  `--yolo` flag (v0.92: "Unknown flag: --yolo"), so `--yolo` now requests
  the repo's own `.crushrc` permissions model — yolo tasks on repos without
  a project-local crush config fail fast with remediation guidance instead
  of stalling headless runs on permission prompts

### Deprecated

### Removed

### Fixed

- Worker pool executed every task under the 30-second drain context created
  at pool start: any task claimed after 30 seconds of uptime ran with an
  already-expired context, its Complete/Fail write failed with
  "context deadline exceeded", and the task was stuck `running` forever.
  Tasks (execution, heartbeats, terminal writes) now run under a
  shutdown-surviving context bounded only by `--task-timeout`; graceful stop
  lets in-flight agents finish and record their outcome. Pinned by a
  long-task regression test (fails on revert) and a de-flaked shutdown test
- `--yolo` injected a nonexistent flag into every autonomous agent run (all
  yolo tasks failed instantly with "Unknown flag: --yolo"); the agent argv
  is now pinned by a contract regression test so flag placement cannot
  silently regress again
- Test-only: removed invalid `_ = t.Cleanup(...)` / single-value Enqueue
  assignments that broke compilation of queue and worker test files
- `nix build` failed with a vendor-hash mismatch after the go.sum bump
  (modernc.org/libc v1.75.7); `vendorHash` re-pinned via the fakeHash dance,
  `nix build` and `nix flake check` verified green

### Security

