# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

### Added

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

