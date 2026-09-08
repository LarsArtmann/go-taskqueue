# go-taskqueue — Agent Guide

Projects-aware task work queue / worker pool for Go. Embedded SQLite journal,
lease-based claims with crash reclaim, DAG dependencies, retries with a
dead-letter queue, pluggable executors (including headless AI coding agents).
Zero external services — one Go binary, one file.

**STATUS: pre-v0.1.0, actively developed by MULTIPLE concurrent agents.**
Before editing, `git pull` your assumptions: re-read files, re-run tests.
Expect uncommitted changes from parallel sessions — read them, judge them,
build on them, never revert them.

## Commands

```bash
./scripts/ci-local.sh     # the pre-push gate: full CI replicant, run before every push
go build ./...            # compile everything
go vet ./...
go test ./... -race       # the standard verify gate
nix build                 # reproducible build (flake, go-standard module)
nix run .#test            # tests via flake app
nix run .#webui-css       # recompile the web UI stylesheet (output is committed)
./scripts/fuzz/nightly.sh # 60s FuzzParseRepo campaign; syncs new seeds into internal/harvest/testdata/fuzz (nightly .github/workflows/fuzz.yml commits them)
```

No Makefile, no justfile — flake.nix owns automation. Pure Go
(modernc.org/sqlite): `CGO_ENABLED=0` is set in the flake and safe everywhere.

CLI smoke test (worker has no --once flag; use timeout + background):

```bash
go build -o /tmp/tq ./cmd/tq
TQ_DB=/tmp/tq-smoke.db /tmp/tq enqueue --type sh --project demo --payload 'echo hi'
timeout 5 /tmp/tq worker --poll 100ms
TQ_DB=/tmp/tq-smoke.db /tmp/tq stats
```

Browser-free web UI smoke (worker + `tq serve` + HTTP/SSE assertions,
CI-safe; wraps `serve` in `timeout` itself):

```bash
./scripts/smoke/webui.sh
```

Set `TQ_BIN=result/bin/tq` to smoke the nix-built binary instead of a
fresh `go build`.

## Architecture

Facts-first: every state change is an immutable fact in an append-only
journal; queue views, retry state, and the DLQ are projections of those
facts. Claim exclusivity comes from lease TTL + expiry reclaim.

| Package             | Purpose                                                                                                         |
| ------------------- | --------------------------------------------------------------------------------------------------------------- |
| `internal/task`     | Task record, Status enum with `CanTransitionTo`, sentinel errors                                                |
| `internal/journal`  | Fact types, append-only Journal interface, MemoryJournal                                                        |
| `internal/queue`    | Store interface + SQLite store; every mutation appends facts in-tx                                              |
| `internal/worker`   | Claim → heartbeat → execute loop; concurrency, panics, drain                                                    |
| `internal/bridge`   | Outbound bridges: papdashboard (alerts), cqa (findings → fix tasks)                                             |
| `internal/executor` | Pluggable execution: `sh` command, HTTP, agent (headless AI), registry                                          |
| `internal/harvest`  | Scans repos' TODO_LIST.md and enqueues work items as agent tasks; drift audit (`tq audit`)                      |
| `internal/budget`   | Daily-cap + budget-command projections over the journal, checked before each pool tick                          |
| `internal/review`   | Fact-stream sweeper: completed agent tasks gain ONE review task; `--review-autofix` mints fix tasks from findings |
| `internal/webui`    | Read-only live dashboard (`tq serve`): journal tailer → hub → SSE server-rendered fragments (ADR-0003)          |
| `cmd/tq`            | CLI: enqueue / worker / harvest / agent-pool / stats / audit / top / show / dlq / cancel / facts / tail / serve |

`internal/` layout is deliberate until the API stabilizes (ADR-0001 core,
ADR-0002 agent-pool policies: `docs/adr/0002-agent-pool-autonomy-pacing-drain.md`;
`docs/planning/` holds the broader plan); the module is not importable
externally yet. Domain vocabulary (task, fact, claim, lease, tick, drift, …)
is defined once in `docs/DOMAIN_LANGUAGE.md` — use those terms exactly.

### Invariants worth knowing before you touch the store

- **Single serialized writer**: `OpenSQLite` sets `MaxOpenConns(1)` + WAL +
  `busy_timeout`. Claim atomicity and the in-tx facts guarantee depend on that
  one connection — do not add a connection pool or drop the `RowsAffected()`
  re-checks (they are the multi-process race guard).
- **Task execution context survives pool shutdown** (bounded only by
  `--task-timeout`): a Ctrl-C lets in-flight agents finish and record their
  outcome. Never reintroduce a shared drain deadline into the task context —
  that bug stranded tasks in `running` forever.

### Payload contracts worth memorizing

- **`sh` executor**: payload is the shell line itself. Accepted shapes: raw
  text, JSON string (`"echo hi"`), or `{"cmd":"..."}`. The CLI wraps non-JSON
  payloads for `--type sh` as JSON strings; `unwrapCommand` unwraps all three.
  Other task types require valid JSON payloads — the CLI errors otherwise.
- **`agent`/`crush` executors**: payload is `AgentPayload`/`CrushPayload`
  JSON (repo, prompt, verify command, timeout). Verification must exit 0.
- **`review` executor** (`internal/executor/review.go` + `internal/review`):
  payload is `ReviewPayload` JSON (repo, reviewed_task, item, commit SHA,
  files changed, model, yolo). Both verdicts COMPLETE the task — only the
  mechanical contract gates it: output must end with a parseable
  `TQ_RESULT: {"verdict":"approve"|"request_changes",...}` line (invalid
  JSON is a retryable failed attempt; `request_changes` without findings is
  invalid too). Verdict + findings land in the completion fact detail; the
  sweeper (watermark starts at journal head — completions before pool start
  are never replayed, same as the papdashboard bridge) turns agent
  completions into `review:<task-id>`-deduped review tasks and, with
  `--review-autofix`, request_changes findings into
  `reviewfix:<review-id>:<hash>`-deduped agent fix tasks. Loop safety is
  structural (sweeper only reviews the `agent` type) plus budgetary (every
  enqueue counts against `--daily-budget`). Every agent-pool and
  `tq worker --agents` registers the review executor, so pools without
  `--review` can still CARRY review tasks another pool minted.
- **Idempotent enqueue**: `task.New.DedupKey` set → re-enqueue returns the
  stored task unchanged (no duplicate row, no duplicate fact). Backed by a
  partial unique index; `dedup_key` is added to legacy DBs by migration.
  A cancelled/dead task's key still suppresses re-enqueue — for harvested
  TODO items the escape hatch is editing the item text (the key is a hash of
  repo + text), so a wording change re-arms the item.
- **PapDashboard ingest contract**: `userId` is a REQUIRED metadata property
  (huma schema — the field has no omitempty); omit it and ingest returns 422.
  The bridge always sends `userId: ""`.

### SQLite migrations

Schema evolution happens in `migrate()` (sqlite.go): run the `CREATE TABLE IF
NOT EXISTS` schema, then pragma-check per column and `ALTER TABLE ADD COLUMN`
for legacy DBs, then create dependent indexes AFTER the column is guaranteed.
Indexes on new columns must NOT go into the schema const — legacy DBs would
fail with "no such column" before the ALTER runs.

## Conventions

- Table-driven tests with plain `testing` (no Ginkgo here, unlike PapDashboard)
- Sentinel errors in `internal/task/errors.go`; check with `errors.Is`
- Facts are the source of truth: a code change that mutates task state must
  append a fact in the same transaction
- Pure-Go deps only; keep `CGO_ENABLED=0` valid
- Platform honesty: POSIX-only test suites (shell stubs, process groups)
  carry `//go:build unix`, and the CI `test-windows` job runs everything
  else on windows-latest. `internal/e2e` keeps a `!unix` doc.go
  placeholder so `go test ./...` does not fail there. Tests must be
  hermetic: `nix build`'s checkPhase runs `go test` in a sandbox with no
  host tools (a test once assumed `crush` on PATH and broke the nix
  build)
- Generated `*_templ.go` files are COMMITTED, never gitignored (the
  samber-do-auditlog v0.9.0 retract lesson: Nix builds vendor source
  without running `templ generate`)
- The web UI is themed with `github.com/larsartmann/templ-components`
  (v1.14.x). Design tokens live in `internal/webui/theme.css` (@theme
  remap: steel-navy neutrals, cyan accent, JetBrains Mono); Tailwind
  classes come from the library's Go/templ source, so CSS MUST be
  recompiled after template changes: `nix run .#webui-css` — the minified
  `internal/webui/static/app.css` is COMMITTED and go:embed'ed (same
  policy as `*_templ.go`). The build script scans the module-cache copy
  of templ-components, so a version bump must rerun it. JetBrains Mono
  woff2 subsets under `internal/webui/static/fonts/` are SIL OFL 1.1
  (© JetBrains)

### templ-components adoption

| Library component                                   | Status  | Where          |
| --------------------------------------------------- | ------- | -------------- |
| `layout.Base`, `ThemeToggle`                        | adopted | `layout.templ` |
| `display.Grid/StatCard/Card/Table/EmptyState`       | adopted | `fragments.templ` |
| `display.Badge/Eyebrow/DefinitionList/Scrollback`   | adopted | `fragments.templ` |
| `display.Button`                                    | adopted | filter bar (apply) |
| `feedback.Alert`                                    | adopted | task detail (last error) |
| `icons.ArchiveBox/Bolt/Calculator/CheckCircle/CircleStack/Clock/Filter/Fire/Inbox` | adopted | stat-card + filter icons (`fragments.templ`) |
| filter inputs, page header/lamp, section hairlines  | custom  | `layout.templ`/`fragments.templ` (thin, SSE-fragment-specific) |
- Go 1.26 idioms are deliberate (`errors.AsType[E]`, `strings.SplitSeq`,
  `for range n`) — do not "modernize" them back to older equivalents
- The `FuzzParseRepo` seed corpus under `internal/harvest/testdata/fuzz` is
  COMMITTED (every seed also runs as a test case on each `go test`) and grows
  via `scripts/fuzz/nightly.sh` (60s campaign, nightly
  `.github/workflows/fuzz.yml` commits new seeds; the script uses a private
  GOCACHE because the corpus never lands on shared-cache mounts). A fuzz
  crasher written into testdata by a failing campaign must be fixed, never
  committed (it would redden `go test` forever).
- `TODO_LIST.md` is machine-consumed by the harvester (`internal/harvest`
  parses `- [ ]` checkboxes and the nearest heading): keep that format, one
  item per line, never convert it to tables. An item with `— BLOCKED:
  <reason>` appended is skipped by the harvester (the marker the agent
  contract tells agents to append when they cannot finish an item)

## Known Issues

- ⚠️ **Concurrent agents commit constantly**: the working tree may gain
  changes mid-task (e.g. a new executor, a migration). Re-run `go test ./...
  -race` right before declaring success; treat unexpected diffs as someone
  else's forward progress.
- ⚠️ **vendorHash drift**: after go.mod/go.sum changes run the fakeHash dance
  (`vendorHash = lib.fakeHash` → `nix build` → copy `got:`). The
  `checks.vendor-hash` gate fails fast on drift.
- ⚠️ **GOEXPERIMENT=jsonv2 in flake.nix**: go-sse imports `encoding/json/v2`
  (a Go 1.26 default experiment); the nixpkgs toolchain builds without it
  enabled, so the flake sets `GOEXPERIMENT = "jsonv2"` in build + shell env.
  The symptom if removed: "build constraints exclude all Go files in
  encoding/json/v2" — and an EMPTY output path (build failure swallowed).
- ⚠️ **Flakes only see git-tracked files**: `git add` new files before
  `nix build` or Nix cannot see them.
- ⚠️ **tq worker runs until signalled** unless `--once` is passed (drain the
  claimable queue, then exit — same semantics as `tq agent-pool --once`);
  without it scripts must wrap the worker in `timeout`/supervisor. Same for
  `tq serve` — it blocks
  until signalled; smoke/tests wrap it in `timeout` (see
  `scripts/smoke/webui.sh`).
- ⚠️ **templ LSP diagnostics are false positives**: the templ/gopls LSP
  layer reports dozens of errors/warnings against `internal/webui` while
  `go build ./...` is green. Never trust LSP webui diagnostics — verify
  with the CLI (build/vet/test) before acting on them.
- ⚠️ **`tq serve` binds 127.0.0.1 by default and is read-only**: never add
  write endpoints without an explicit `--allow-writes`-style flag + CSRF
  story (ADR-0003 guardrail). Non-loopback binds (incl. `:port` and
  hostnames) are default-deny: `webui.Config.Validate` refuses to start
  without `--auth-token`/`TQ_SERVE_TOKEN`; with a token, a constant-time
  middleware guards all routes (`Authorization: Bearer` or `?token=`,
  needed because EventSource cannot set headers; the `--verbose` access
  log redacts the token). Keep the refusal: it is the payload
  confidentiality guardrail for LAN serves.
- ⚠️ **golangci-lint runs in CI but is advisory** (`continue-on-error`): the
  config enables ~100 linters against a ~400-finding repo-wide baseline
  (wrapcheck/varnamelen/paralleltest lead). The hard gates are vet + gofmt +
  tests. golangci-lint v2 emits no GitHub annotation commands itself, but
  actions/setup-go registers a `go` problem matcher that would scrape the
  advisory step's `file.go:line:col:` text lines into 10 capped [failure]
  annotations on every green run (ci.yml removes it with
  `::remove-matcher owner=go::`); `scripts/lint-annotations.sh` re-runs it
  scoped to `--new-from-rev` and turns findings on changed lines into
  `::warning` annotations (10-per-step cap → overflow stays log-only).
  Policy: never
  mass-"fix" the baseline (it would rewrite the whole codebase); when you
  touch a function, don't add new findings, and fixing that function's
  findings in passing is welcome. Generated `*_templ.go` files are excluded
  from lint and from the treefmt/gofumpt gate (`templ fmt` owns `.templ`).
- ⚠️ **Agent tasks need repo-local autonomy**: `crush run` has no yolo flag;
  a `--yolo` task on a repo without a project-local `.crushrc` fails fast by
  design. When touching the agent argv, update
  `TestAgentExecutorArgvContract` — stub-based tests cannot catch flag drift.
- ⚠️ **This repo is dogfooded (since 2026-09-07)**: an `agent-pool` may run
  against THIS repo — `.crushrc` (minimum autonomy) + `.tq-verify` (build,
  vet, race tests, gofmt) are the rails, and unchecked TODO_LIST items are
  live pool food. Expect autonomous commits on master (never pushes) and
  extra working-tree churn alongside the daemon. Launch command and
  budget flags: `docs/planning/2026-09-07_20-47_SUPERB-PLAN-ROUND4-DOGFOOD-POOL-EATS-THIS-REPO.md`.
  If the pool misbehaves: stop it, review `tq dlq`, rescue or cancel.

## Relation to other projects

Semantics proven in go-cqrs-lite (facts/journal) and PapDashboard (worker
pools over durable queues); composes with both, depends on neither.

**PapDashboard bridge contract:** run
`tq worker --alert-url http://<pap>:8080 --alert-api-key <PAP_API_KEY>` (env:
`TQ_PAP_URL`/`TQ_PAP_API_KEY`). Dead-lettered tasks raise `alert.triggered`
(sourceApp `go-taskqueue`, severity critical, task ID as correlationId, fact
Seq as Idempotency-Key); a later completion of an alerted task posts
`alert.resolved` with the same derived title, so rescue flows close their own
alerts. The journal watermark starts at the head per bridge process —
incidents fired while it was down are not replayed (review via `tq dlq`).
Status and verification details: FEATURES.md, CHANGELOG.md.
