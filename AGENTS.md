# go-taskqueue — Agent Guide

Projects-aware task work queue / worker pool for Go. Embedded SQLite journal,
lease-based claims with crash reclaim, DAG dependencies, retries with a
dead-letter queue, pluggable executors (including headless AI coding agents).
Zero external services — one Go binary, one file.

**STATUS: v0.1.0 shipped 2026-09-06; actively developed by MULTIPLE
concurrent agents.** Re-read files and re-run tests before editing; expect
uncommitted changes from parallel sessions — read them, judge them, build on
them, never revert them.

## Commands

```bash
./scripts/ci-local.sh     # the pre-push gate: full CI replicant (vet/build/race/smokes/nix)
go build ./... && go vet ./... && go test ./... -race   # standard verify gate (ROOT MODULE ONLY — see below)
nix build                 # reproducible build; nix run .#test = tests; nix run .#webui-css = stylesheet
./scripts/fuzz/nightly.sh # 60s FuzzParseRepo campaign; nightly workflow commits new seeds
```

**Multi-module repo (ADR-0011):** `internal/{task,journal,queue,executor,worker}`
are sub-modules plus `internal/queue/{sqlite,postgres}` backend modules
(ADR-0011 + ADR-0012; import paths unchanged); the root module is the app
layer. `./...` never descends into nested modules — per-module gates
(disk-derived, same as CI):

```bash
for m in $(find internal -name go.mod | sed 's|/go.mod$||' | sort); do
  ( cd "$m" && GOWORK=off go build ./... && GOWORK=off go vet ./... && GOWORK=off go test ./... -count=1 ) || exit 1
done
```

Internal requires point at real tagged versions (never `v0.0.0` —
`go install` resolves them via the proxy; `internal/*/vX.Y.Z` subdirectory
tags ride every release) + relative `replace` for local dev (NO go.work —
replace-only by decision); `go test ./internal/foo` from root FAILS by design
(cd into the module instead).

Smokes (all CI-safe; `TQ_BIN=result/bin/tq` smokes the nix-built binary):

```bash
./scripts/smoke/webui.sh        # worker + tq serve + HTTP/SSE + write-route lockout assertions
./scripts/smoke/status-loop.sh  # stub agent; sweeper mint → report → TODO append → re-arm
./scripts/smoke/bootstrap-install.sh  # --install renders unit + pool.conf against a fake $HOME
./scripts/smoke/release-gates.sh # fixture go.mods: release allowlist/tag gates, positive + negative
./scripts/check-go-mods.sh      # replaces, pins, toolchain alignment, go mod verify (all modules)
nix run .#test                  # full multi-module suite (root + every internal/* module)
go build -o /tmp/tq ./cmd/tq    # CLI scratch: enqueue/worker/stats (--once drains then exits)
```

No Makefile — flake.nix owns automation. Pure Go (modernc.org/sqlite):
`CGO_ENABLED=0` everywhere. Manual/verification workers and `tq serve`
ALWAYS get `--once` or a `timeout` wrapper — no process outlives its session.

## Architecture

Facts-first: every state change is an immutable fact in an append-only
journal; queue views, retry state, and the DLQ are projections of those
facts. Claim exclusivity comes from lease TTL + expiry reclaim. The library
core (task, journal, queue, executor, worker) is split into sub-modules
whose DAG the compiler enforces; everything above them is the root module.

| Package             | Purpose                                                                                                                                                                   |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/task`     | Task record, Status enum with `CanTransitionTo`, sentinel errors                                                                                                          |
| `internal/journal`  | Fact types, append-only Journal interface, MemoryJournal                                                                                                                  |
| `internal/queue`    | Store contract: interface, Filter, Queue facade, watermarks entry (deps: task+journal only)                                                                              |
| `internal/queue/sqlite`, `internal/queue/postgres` | Driver-style backend modules (`sqlite.Store`/`Open`, `postgres.Store`/`Open`); mirrored helpers + conformance suites (ADR-0007/0012) |
| `internal/worker`   | Claim → heartbeat → execute loop; concurrency, panics, drain, preflight requeue ladder                                                                                    |
| `internal/bridge`   | Outbound bridges: papdashboard (alerts), cqa (findings → fix tasks)                                                                                                       |
| `internal/executor` | Pluggable execution: `sh`, HTTP, agent (headless AI), review, status, registry                                                                                            |
| `internal/harvest`  | Scans repos' TODO_LIST.md into agent tasks; drift audit (`tq audit`); prune-stale sweeps                                                                                  |
| `internal/budget`   | Daily-cap + budget-command projections over the journal, checked before each pool tick                                                                                    |
| `internal/review`   | Sweeper: completed agent tasks gain ONE review task; `--review-autofix` mints fix tasks                                                                                   |
| `internal/status`   | Sweeper: every N agent completions per project mint ONE done-prompt report task (`--status-every`)                                                                        |
| `internal/consumer` | Journal dispatcher: per-subscriber cursor, at-least-once in-order, lag observability (ADR-0009)                                                                           |
| `internal/runactor` | run.Group actors, LIFO `OnShutdown`, `InterruptOn` (2nd signal = exit 130), detached task contexts                                                                        |
| `internal/webui`    | Live dashboard (`tq serve`): journal tailer → hub → SSE server-rendered fragments (ADR-0003)                                                                              |
| `cmd/tq`            | CLI: enqueue / worker / harvest / agent-pool / bootstrap / stats / tasks / audit / top / show / dlq / cancel / facts / tail / watermarks / serve / api / doctor / version |

`internal/` layout is deliberate until the API stabilizes (ADR-0001,
ADR-0002: `docs/adr/`; plans in `docs/planning/`). Domain vocabulary is
defined once in `docs/DOMAIN_LANGUAGE.md` — use those terms exactly.

### Store invariants (do not break)

- **Single serialized writer**: `sqlite.Open` sets `MaxOpenConns(1)` + WAL +
  `busy_timeout`. Claim atomicity and the in-tx facts guarantee depend on
  it — no connection pool, never drop the `RowsAffected()` re-checks.
- **Task execution context survives pool shutdown** (bounded only by
  `--task-timeout`): a Ctrl-C lets in-flight agents finish and record their
  outcome. Never reintroduce a shared drain deadline into the task context.
- **Facts in the same tx as state**: a code change that mutates task state
  must append its fact in the same transaction, or it didn't happen.

### Payload contracts (short form — code owns the detail)

- **`sh`**: payload is the shell line; accepted shapes raw text / JSON
  string / `{"cmd":"..."}` (`unwrapCommand`). Other types need valid JSON.
- **`agent`**: `AgentPayload` JSON (repo, prompt, verify command, timeout).
  Verify must exit 0. A payload model makes the executor pass `crush run
  -m`, which RESETS reasoning effort — the repo `.crushrc` managed block
  (`tq bootstrap`) is the only model+effort carrier. `--yolo` without a
  repo-local `.crushrc` fails fast by design (argv pinned by
  `TestAgentExecutorArgvContract`).
- **`review`**: `ReviewPayload` JSON. Both verdicts COMPLETE the task; the
  mechanical gate is a parseable final `TQ_RESULT: {"verdict":...}` line.
  The sweeper (watermark head-bootstrapped — never replays pre-start
  completions) mints `review:<task-id>`-deduped review tasks and, with
  `--review-autofix`, `reviewfix:<id>:<hash>`-deduped fix tasks. Every
  agent-pool and `tq worker --agents` registers the executor (carry parity).
- **`status`**: `StatusPayload` JSON; the done-prompt agent writes
  `docs/status/<ts>_<name>.md` and appends next items (questions as
  `— BLOCKED:`) to TODO_LIST.md — that append IS the harvest loop-back.
  Two gates: the `TQ_RESULT` contract naming an existing REPO-RELATIVE
  report file, and the repo verify command. One report in flight per
  project; `status:<project>:<trigger-id>` dedup.
- **Idempotent enqueue**: `DedupKey` set → re-enqueue returns the stored
  task unchanged. A cancelled/dead task's key still suppresses re-enqueue;
  for harvested items the escape hatch is editing the item text (the key
  hashes repo + text).
- **`Task-Queue-ID` commit footer**: every prompt contract tells agents to
  end commits with it; the executor resolves the placeholder at RUN time.
  Never hardcode the placeholder inside backtick raw strings (a backtick
  terminates the literal).
- **Fact forensics**: `task.failed` carries `FailureEvidence{stage,
  exit_code, tail}` (tail size: one `EvidenceTailBytes` constant);
  `task.requeued` carries `RequeueEvidence{reason, retry_in_ms}`.
- **PapDashboard ingest**: `userId` is a REQUIRED metadata property (no
  omitempty) — omit it and ingest 422s. The bridge always sends `userId: ""`.

### Operational contracts

- **SQLite migrations** happen in `migrate()` (sqlite.go): CREATE TABLE
  schema → pragma-check + ALTER TABLE per column for legacy DBs → dependent
  indexes AFTER the column exists. New-column indexes never go into the
  schema const.
- **Journal-consumer watermarks** (bridge, sweepers) live in the
  `watermarks` table (monotonic upsert via the store's Watermark and
  SaveWatermark methods). Checkpoint
  AFTER the batch's last accepted fact, never before; a failed checkpoint
  gates forwarding. `tq watermarks show/set` — set may rewind (replay is
  idempotent via seq-derived keys). A lagging cursor may simply mean the
  consumer is off.
- **prune-stale**: cancels PENDING tasks whose item is now `[x]` OR whose
  item text is gone from the file (done-and-deleted / reworded — harvest
  provenance via payload dedup key guards external tasks; `catchup:`
  prefixes stripped). agent-pool runs one sweep synchronously before any
  actor starts (`--prune-stale=false` to skip); running tasks are reported,
  never stopped.

## Conventions

- Table-driven tests with plain `testing`; sentinel errors in
  `internal/task/errors.go`, checked with `errors.Is`
- Pure-Go deps only (`CGO_ENABLED=0` valid); Go 1.26 idioms are deliberate
  (`errors.AsType[E]`, `strings.SplitSeq`, `for range n`) — do not
  "modernize" them back
- **Generic retry loops use `github.com/larsartmann/go-retry`** (v0.5.0,
  executor module): exponential backoff + jitter, pluggable retryable
  predicate. Do NOT hand-roll new retry/sleep loops. Exceptions (verified
  2026-09-10): reconnect *supervisors* whose success case is "operation
  ended" (`harvest/watch.go` Run — retry.Do's nil-stops semantics don't
  map) keep their own loop; domain backoff (queue NotBefore ladder,
  worker.Backoff) stays — it's persisted journal-fact state, not a loop.
- Platform honesty: POSIX-only suites carry `//go:build unix`; CI runs the
  rest on windows-latest. Tests must be hermetic (nix checkPhase has no
  host tools — a test once assumed `crush` on PATH and broke the nix build)
- Generated `*_templ.go` and the minified `app.css` are COMMITTED (Nix
  builds vendor source without `templ generate`); after template edits run
  `templ generate` + `nix run .#webui-css` (build script scans the
  module-cache copy of templ-components — rerun on version bumps)
- Docs formatting stays MANUAL by decision (2026-09-08): dprint is an
  on-demand devShell tool, NOT gated — don't re-litigate without solving
  plugin pinning AND the multi-writer problem
- The web UI is themed with `github.com/larsartmann/templ-components`
  (v1.14.x); tokens in `internal/webui/theme.css`; JetBrains Mono woff2
  subsets are SIL OFL 1.1 (© JetBrains)
- The `FuzzParseRepo` seed corpus is COMMITTED and grows via nightly
  campaigns; a fuzz crasher must be fixed, never committed
- `TODO_LIST.md` is machine-consumed: `- [ ]` checkboxes, one item per
  line, never tables; `— BLOCKED: <reason>` keeps an item out of the pool;
  unchecked items must be agent-executable (checked by
  `scripts/check-todo-list.sh` in ci-local)
- Status reports are indexed on creation (`check-status-index.sh` +
  pre-commit hook via `scripts/install-pre-commit.sh`); CHANGELOG is
  append-only; `check-features-roadmap.sh` guards shipped-vs-planned drift

### templ-components adoption

| Library component                                                                                | Status  | Where                                                                           |
| ------------------------------------------------------------------------------------------------ | ------- | ------------------------------------------------------------------------------- |
| `layout.Base`, `ThemeToggle`                                                                     | adopted | `layout.templ`                                                                  |
| `display.Card/Table/EmptyState`                                                                  | adopted | `fragments.templ`                                                               |
| `display.Badge/Eyebrow/DefinitionList/Scrollback`                                                | adopted | `fragments.templ`                                                               |
| `display.AreaChart`                                                                              | adopted | metrics row: fact-rate sparkline + completion histogram (`fragments.templ`)     |
| `display.Button`                                                                                 | adopted | filter bar (apply)                                                              |
| `feedback.Alert`                                                                                 | adopted | task detail (last error)                                                        |
| `icons.ArchiveBox/CircleStack/Filter/Inbox`                                                      | adopted | empty-state + filter icons (`fragments.templ`)                                  |
| status nowband (tq-seg), board columns/cards, filter inputs, page header/lamp, section hairlines | custom  | `fragments.templ`/`layout.templ`/`theme.css` (StatCard retired for the nowband) |

Guarded by `TestAdoptionTableCoversTemplates` + `TestAdoptionTablePinsCustomRows`
(both directions of rot fail the suite).

## Known Issues

- ⚠️ **Concurrent agents commit constantly**: re-run `go test ./... -race`
  right before declaring success; unexpected diffs are someone else's
  forward progress. In hot files (cmd/tq/main.go, webui templates) re-read
  immediately before every write and checkpoint with `go build ./...`
  mid-session. Never generate/patch Go source via shell heredocs or python
  string surgery — heredoc escaping broke compilation repeatedly.
- ⚠️ **vendorHash drift**: after go.mod/go.sum changes run the fakeHash
  dance (`vendorHash = lib.fakeHash` → `nix build` → copy `got:`).
- ⚠️ **GOEXPERIMENT=jsonv2 in flake.nix** (go-sse imports
  `encoding/json/v2`): removing it yields "build constraints exclude all
  Go files" AND an empty output path — the build failure is swallowed.
- ⚠️ **Flakes only see git-tracked files**: `git add` new files before
  `nix build`.
- ⚠️ **templ LSP diagnostics are false positives** (phantom syntax errors
  against a green `go build`; the cache goes stale, not the sources). Trust
  the CLI, not the LSP, for webui/templ.
- ⚠️ **`tq serve` security model**: loopback-only and read-only by default;
  `--allow-writes` adds exactly two CSRF-guarded admin routes (cancel,
  rescue) with a failed-attempt lockout (3 bad CSRF tokens → 60s 429);
  non-loopback binds (incl. `:port`, hostnames) refuse to start without
  `--auth-token` (constant-time bearer/`?token=`). Full matrix:
  SECURITY.md. Don't add write endpoints without the same treatment.
- ⚠️ **golangci-lint is advisory** (`continue-on-error`, ~400-finding
  baseline): never mass-"fix" the baseline; don't add new findings in
  functions you touch. Hard gates: vet + gofmt + tests. `*_templ.go` is
  lint-excluded (`templ fmt` owns `.templ`).
- ⚠️ **Kernel 7.2 ETXTBSY anomaly**: `execve` of freshly written binaries
  intermittently fails with "text file busy" on this host (kernel 7.2.3,
  reproduced standalone with NO writer holding the file; tmpfs + btrfs;
  temp+rename does NOT help). `runAgent`/`AgentVersion` retry it
  (`execWithTransientRetry`) — if another exec site starts flaking the same
  way, route it through that helper instead of chasing a writer.
- ⚠️ **Dead-export audits must use SUBSTRING matching**: `rg -w Symbol`
  misses suffixed references (`NewSink`, `NewCommandExecutor` use `Sink`,
  `CommandExecutor`) and undercounts — the 2026-09-10 re-derivation found
  most "dead" exports alive once matching corrected.
- ⚠️ **This repo is dogfooded (since 2026-09-07)**: an `agent-pool` may run
  against THIS repo — `.crushrc` (minimum autonomy) + `.tq-verify` are the
  rails; unchecked TODO_LIST items are live pool food. Sibling repos on the
  same (now bootstrap-managed) rails: `project-discovery-sdk` (queue-facing
  "Open Work" checkbox section only — the status tables below are NOT
  parsed), `overview` (checkbox backlog; owner-gated items carry
  `— BLOCKED:`), `project-discovery-daemon` (rails only). Expect autonomous
  commits on master (never pushes). If the pool misbehaves: stop it, review
  `tq dlq`, rescue or cancel.

## Relation to other projects

Semantics proven in go-cqrs-lite (facts/journal) and PapDashboard (worker
pools over durable queues); composes with both, depends on neither.

**PapDashboard bridge**: `tq worker --alert-url http://<pap>:8080
--alert-api-key <KEY>` (env `TQ_PAP_URL`/`TQ_PAP_API_KEY`). Dead letters
raise `alert.triggered` (fact Seq = Idempotency-Key); a later completion
posts `alert.resolved` — rescue flows close their own alerts. The watermark
starts at head per bridge process; incidents fired while down are not
replayed (review via `tq dlq`). Details: FEATURES.md, CHANGELOG.md.
