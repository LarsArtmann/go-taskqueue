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
export GOEXPERIMENT=jsonv2; go build ./... && go vet ./... && go test ./... -race   # standard verify gate (ROOT MODULE ONLY — see below; the export is REQUIRED outside the flake devShell: go-sse imports encoding/json/v2, and without it the build dies with "build constraints exclude all Go files" while gopls shows the same phantoms)
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
(cd into the module instead). Release flow and version surfaces are
documented in `docs/release/RELEASE.md` (two-phase --tag/--push, sub-tag
cutting, allowlist gates) and `docs/release/VERSION-SURFACES.md` (the seven
surfaces and their bump order).

Smokes (all CI-safe; `TQ_BIN=result/bin/tq` smokes the nix-built binary):

```bash
./scripts/smoke/webui.sh        # worker + tq serve + HTTP/SSE + write-route lockout assertions
./scripts/smoke/status-loop.sh  # stub agent; sweeper mint → report → TODO append → re-arm
./scripts/smoke/dogfood-once.sh  # stub agent; harvest → work (footer commit) → review approve; TQ_DOGFOOD=1 runs the real-agent proof (spends money)
./scripts/smoke/bootstrap-install.sh  # --install renders unit + pool.conf against a fake $HOME
./scripts/smoke/release-gates.sh # fixture go.mods: release allowlist/tag gates, positive + negative
./scripts/check-go-mods.sh      # replaces, pins, toolchain alignment, go mod verify (all modules)
./scripts/check-dead-exports.sh # advisory dead-export audit: zero-importers detector, substring matching (NOT rg -w)
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

| Package                                            | Purpose                                                                                                                                                                   |
| -------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/task`                                    | Task record, Status enum with `CanTransitionTo`, sentinel errors                                                                                                          |
| `internal/journal`                                 | Fact types, append-only Journal interface, MemoryJournal                                                                                                                  |
| `internal/queue`                                   | Store contract: interface, Filter, Queue facade, watermarks entry (deps: task+journal only)                                                                               |
| `internal/queue/sqlite`, `internal/queue/postgres` | Driver-style backend modules (`sqlite.Store`/`Open`, `postgres.Store`/`Open`); mirrored helpers + conformance suites (ADR-0007/0012)                                      |
| `internal/worker`                                  | Claim → heartbeat → execute loop; concurrency, panics, drain, preflight requeue ladder                                                                                    |
| `internal/bridge`                                  | Outbound bridges: papdashboard (alerts), cqa (findings → fix tasks)                                                                                                       |
| `internal/executor`                                | Pluggable execution: `sh`, HTTP, agent (headless AI), review, status, registry                                                                                            |
| `internal/harvest`                                 | Scans repos' TODO_LIST.md into agent tasks; drift audit (`tq audit`); prune-stale sweeps                                                                                  |
| `internal/budget`                                  | Daily-cap + budget-command projections over the journal, checked before each pool tick                                                                                    |
| `internal/review`                                  | Sweeper: completed agent tasks gain ONE review task; `--review-autofix` mints fix tasks                                                                                   |
| `internal/status`                                  | Sweeper: every N agent completions per project mint ONE done-prompt report task (`--status-every`)                                                                        |
| `internal/consumer`                                | Journal dispatcher: per-subscriber cursor, at-least-once in-order, lag observability (ADR-0009)                                                                           |
| `internal/runactor`                                | run.Group actors, LIFO `OnShutdown`, `InterruptOn` (2nd signal = exit 130), detached task contexts                                                                        |
| `internal/webui`                                   | Live dashboard (`tq serve`): journal tailer → hub → SSE server-rendered fragments (ADR-0003)                                                                              |
| `cmd/tq`                                           | CLI: enqueue / worker / harvest / agent-pool / bootstrap / stats / tasks / audit / top / show / dlq / cancel / facts / tail / watermarks / serve / api / doctor / version |

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
  `TestAgentExecutorArgvContract`). With `--task-closeout` the work turn is
  followed by a close-out turn that resumes the EXACT session (`--session`,
  never `--continue` — concurrent agents) to run the brutal a)-g) self-review
  and re-emit the work turn's `TQ_RESULT` (the gate reads the last line);
  the report lands at `docs/status/<ts>_task-<id>.md`. Reviews and status
  tasks run a close-out-free clone (they ARE the second opinion).
- **`review`**: `ReviewPayload` JSON. Both verdicts COMPLETE the task; the
  mechanical gate is a parseable final `TQ_RESULT: {"verdict":...}` line.
  The sweeper (watermark head-bootstrapped — never replays pre-start
  completions) mints `review:<task-id>`-deduped review tasks and, with
  `--review-autofix`, `reviewfix:<id>:<hash>`-deduped fix tasks. Every
  agent-pool and `tq worker --agents` registers the executor (carry parity).
  The review prompt quotes the work run's contract with its `{{TASK_ID}}`
  resolved to the REVIEWED task's id, plus a `6. Queue cross-reference`
  judging criterion for footer-bearing quotes — runAgent's blanket
  substitution would otherwise stamp the REVIEW's own id into the quoted
  contract and a diligent reviewer flags the work run's correct footer as
  foreign (the 2026-09-12 Hermes review, task 000001a092aa: a sound fix got
  request_changes over exactly that). Fix prompts resolve the quoted
  original the same way and carry exactly one remaining `{{TASK_ID}}` —
  the fix run's OWN footer instruction.
- **`status`**: `StatusPayload` JSON; the done-prompt agent writes
  `docs/status/<ts>_<name>.md` and appends next items (questions as
  `— BLOCKED:`) to TODO_LIST.md — that append IS the harvest loop-back.
  Its docs-health pass annotates (never rewrites) 2026-* reports, keeps
  TODO_LIST/CHANGELOG/AGENTS/README/ROADMAP/FEATURES current with what
  the window's tasks actually shipped, and archives fully-done reports
  to `docs/status/archived/`; hard scope: docs only, never code/config.
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
  terminates the literal). The footer must carry the queue-assigned ID from
  the task prompt VERBATIM — if a reviewer or a second artifact supplies a
  different ID, use that one and report the discrepancy; never merge or
  silently pick between IDs (the f26 three-ID cluster is the cautionary
  tale).
- **Fact forensics**: `task.failed` carries `FailureEvidence{stage,
  exit_code, tail}` (tail size: one `EvidenceTailBytes` constant);
  `task.requeued` carries `RequeueEvidence{reason, retry_in_ms}`.
- **Provider rate limits (429 / usage limit)**: a failed agent turn's
  output is scanned (`executor.DetectRateLimit`); a Z.ai-style reset
  timestamp ("Your limit will reset at <ts>"), an RFC3339 renews-at, or a
  numeric `retry_after` yields a `*executor.RateLimitError` carrying the
  wait, and the worker requeues WITHOUT burning an attempt (±5% jitter
  capped ±1min; unparseable reset falls back to 15min, capped at 6h).
  synthetic.new's OpenAI-style quota 429 (`insufficient_quota`, no
  timestamp in the body) is covered by the fallback. `AgentExecutor` also
  keeps an in-process gate (`rateLimitUntil`): after one 429, sibling
  agent/review/status runs in the same pool fast-refuse without spawning
  the binary until the window passes — gates are per executor instance
  (`WithoutCloseout()` clones start fresh), so cross-pool/cross-provider
  setups each re-learn their own window with one probe. Gates are keyed
  PER REPO (`rateLimitGates`, 2026-09-11): a repo's `.crushrc` fixes its
  provider, so a Z.ai 429 in repo A must never park repo B's synthetic.new
  tasks; repo-less evidence falls back to the shared gate. The `http`
  executor classifies 429 responses the same way (Retry-After header first,
  then body detection). A 429 during the CLOSE-OUT turn registers
  `closeoutPending` (task.ID → repo+session) so the re-claim RESUMES at
  closeout instead of re-running the paid work turn (in-process only; an
  executor restart degrades to a full re-run, never a lost close-out).
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
  2026-09-10): reconnect _supervisors_ whose success case is "operation
  ended" (`internal/harvest/watch.go` Run — retry.Do's nil-stops semantics don't
  map) keep their own loop; domain backoff (queue NotBefore ladder,
  worker.Backoff) stays — it's persisted journal-fact state, not a loop.
- Platform honesty: POSIX-only suites carry `//go:build unix`; CI runs the
  rest on windows-latest. Tests must be hermetic (nix checkPhase has no
  host tools — a test once assumed `crush` on PATH and broke the nix build)
- Generated `*_templ.go` and the minified `app.css` are COMMITTED (Nix
  builds vendor source without `templ generate`); after template edits run
  `templ generate` + `nix run .#webui-css` (build script scans the
  module-cache copy of templ-components — rerun on version bumps);
  ci-local GATES the artifact via `check-webui-css.sh` (byte-equal tailwind
  rebuild; needs nix, not a devShell — unminified/hand-edited css shipped to
  master twice in 2026-09-11 before the orphaned guard got wired)
- Detail-page payload-section decisions (owner-approved 2026-09-11, 14-22/15-39
  reports): agent payloads WITH a work item lead with the item and keep the
  prompt COLLAPSED; item-less payloads lead WITH the prompt (no separate
  hint). Dead-lettered reasons COUNT into the retry-trail strip
  (`journal.DeadLettered` in `retryTrail`, `TestRetryTrailCountsDeadLetter`).
- Docs formatting stays MANUAL by decision (2026-09-08): dprint is an
  on-demand devShell tool, NOT gated — don't re-litigate without solving
  plugin pinning AND the multi-writer problem
- The web UI is themed with `github.com/larsartmann/templ-components`
  (v1.16.x); tokens in `internal/webui/theme.css`; JetBrains Mono woff2
  subsets are SIL OFL 1.1 (© JetBrains)
- The `FuzzParseRepo` seed corpus is COMMITTED and grows via nightly
  campaigns; a fuzz crasher must be fixed, never committed
- `TODO_LIST.md` is machine-consumed: `- [ ]` checkboxes, one item per
  line, never tables; `— BLOCKED: <reason>` keeps an item out of the pool;
  unchecked items must be agent-executable (checked by
  `scripts/check-todo-list.sh` in ci-local)
- Status reports are indexed on creation (`check-status-index.sh` +
  pre-commit hook via `scripts/install-pre-commit.sh`); CHANGELOG is
  append-only; `check-features-roadmap.sh` guards shipped-vs-planned drift.
  Archiving a report/plan (`git mv` to `archived/`) requires REPOINTING
  every citation to the moved path FIRST (ADR-0004/0009 + CHANGELOG all
  cited two 2026-09-08 planning docs; `check-doc-refs.sh` fails the move
  otherwise) and updating the index row to `archived/<name>` + the archive
  counter in `docs/status/README.md`
  The installer also writes a commit-msg hook (2026-09-11): a commit
  carrying `Task-Queue-ID:` footers must carry EXACTLY ONE, well-formed
  (hex, 16+ chars) — duplicates/malformed footers corrupt the queue↔git
  cross-reference; multiple COMMITS per task remain the norm (work +
  close-out), so ID reuse across commits is NOT rejected
- `ci-local.sh` checks master-CI state first (`scripts/check-ci.sh`, gh):
  a local green gate is worthless if master is red (five DONE verdicts
  shipped on a 3h-red master before this). Bypass consciously with
  `CI_CHECK=off`. It also exports `GOEXPERIMENT=jsonv2` itself — never
  rely on `~/.config/go/env` (that dependence made CI red while local was
  green; ci.yml sets the env workflow-wide for the same reason)
- Evidence archives (`docs/status/assets/*/README.md`) are gated by
  `scripts/check-ghost-archives.sh` (ci-local + CI): every bare filename a
  README promises must be git-tracked (GHOST = on disk but ignored,
  MISSING = absent, UNTRACKED README = whole archive uncommitted;
  backticked placeholders/globs/paths/dotfiles are exempt) — the global
  `*.log` ignore otherwise silently drops logs from daemon commits (the f9
  near-miss). Every archive must also ship a git-tracked SHA256SUMS
  manifest; the gate re-verifies hashes and completeness against the dir's
  file set (NO MANIFEST / UNTRACKED / STALE / INCOMPLETE — UNTRACKED
  short-circuits the other two, so commit the manifest when adding files).
  Regenerate from inside the archive after ANY file change:
  `ls | grep -v '^SHA256SUMS$' | sort | xargs sha256sum > SHA256SUMS`
  (manifest filenames have no spaces — the gate's awk coverage check
  assumes it)
- Evidence copies into the repo must run `git check-ignore -v <targets>`
  FIRST (copy second) and diff the auto-commit daemon commit's `--stat`
  against the intended file set — a clean `git status` is not a complete
  archive while the daemon is live: global ignores + the daemon + a
  clean-looking commit form the silent-loss triangle (f9 near-miss: 16
  `*.log` files invisible to git behind an archive README, caught only by
  reading the daemon commit's stat; 06-41 report §d1/§e1-2)

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
  Negative-test fixtures created INSIDE gated trees are daemon-food: the
  2026-09-12 manifest task's scratch archive was auto-committed to master
  within minutes (needed a follow-up deletion commit). Build fixtures under
  /tmp or create+assert+trash in ONE shell chain.
- ⚠️ **vendorHash drift**: after go.mod/go.sum changes run the fakeHash
  dance (`vendorHash = lib.fakeHash` → `nix build` → copy `got:`). NOTE
  (2026-09-10): a runner-ONLY variant exists — CI's nix job failed with a
  FOD hash mismatch while the committed hash verified green locally (even
  rebuilding the exact failed drv from the failed commit), same got-hash
  across two trees, first failing run = the push carrying the fullcore
  postgres require + go.mod replace. A local fakeHash re-run does NOT
  diagnose this class; differential-dump the runner's module fetch first
  (docs/status/2026-09-10_06-25 report d1).
- ⚠️ **GOEXPERIMENT=jsonv2 in flake.nix** (go-sse imports
  `encoding/json/v2`): removing it yields "build constraints exclude all
  Go files" AND an empty output path — the build failure is swallowed.
- ⚠️ **setup-go must stay PINNED, never `stable`** (2026-09-11, 16-00 report
  §a): `go-version: stable` floated to go 1.27.1 on runners — on go 1.27,
  encoding/json/v2 is stable but stdversion-gated to modules declaring
  `go 1.27`, so vet fails with "json.Unmarshal requires go1.27" REGARDLESS
  of GOEXPERIMENT (the experiment satisfies 1.26's availability gate, not
  1.27's language-version gate). All 7 setup-go steps (ci.yml ×6, fuzz.yml
  ×1) are pinned to `1.26.7` matching go.mod + the toolchain-alignment gate;
  a GOEXPERIMENT-only fix reproduces locally and still fails on runners —
  verify against the environment that failed, not just locally.
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
  lint-excluded (`templ fmt` owns `.templ`). wrapcheck + varnamelen were
  triaged to zero (2026-09-10, task 000001a089c3): wrapcheck ignores
  internal-package globs + stdlib idioms + tests; varnamelen ignores tests
  - `w`/`r`/`fs`/`db`/`id` idioms; remaining sites were renamed, not
    suppressed. That sweep's renames leaked into string literals twice
    (`q.Get("query")` deadened the webui filter, `task(store)` mangled
    `tq dlq --max-attempts` help; fixed da8f331/9b7c46b): a variable rename
    must never change a string literal — before calling a rename done, grep
    the diff's quoted lines when the variable name equals a nearby param
    name, JSON tag or flag text. The advisory lint step loops every
    `internal/*` sub-module in ci.yml too (f32, parity with ci-local.sh).
- ⚠️ **gosec advisory baseline is all FP/by-design** (triaged 2026-09-10,
  v2.29.0, 48 findings over root + all sub-modules; advisory CI job, f21):
  G204/G702 (exec with variable) — executors and bootstrap RUN commands
  from task payloads/`.tq-verify`/user config as their core feature, argv
  is never shell-interpolated; G703/G304 (path taint) — a local CLI
  operating on user-named repo/config paths (G304 already golangci-excluded);
  G306/G301/G302 — 0644 repo/unit files and shared 0o1777 slot-lock dirs
  are deliberate (pool.conf correctly 0600); G124 — auth cookies set
  HttpOnly+SameSite, `Secure` deliberately conditional on TLS (LAN binds);
  G710 — redirects target `/task/<id>` where the id must first resolve in
  the store; G118 — canonical fresh-context graceful shutdown; G104 —
  `os.Setenv`; G404 — backoff jitter (non-crypto); G202 — sort keys come
  from a fixed allowlist switch, values bind via `?`. New gosec classes on
  code you touch: triage before assuming baseline. The two REAL findings
  (G114 timeout-less serves in examples/) are fixed.
- ⚠️ **Kernel 7.2 ETXTBSY anomaly**: `execve` of freshly written binaries
  intermittently fails with "text file busy" on this host (kernel 7.2.3,
  reproduced standalone with NO writer holding the file; tmpfs + btrfs;
  temp+rename does NOT help). `runAgent`/`AgentVersion` retry it
  (`execWithTransientRetry`) — if another exec site starts flaking the same
  way, route it through that helper instead of chasing a writer.
- ⚠️ **Agent shells inherit `TQ_DB=/mnt/pool/services/tq/tq.db`**: `tq`
  commands (enqueue included) hit the PRODUCTION dogfood journal even when
  `cd`'d into a scratch dir — `defaultDB()` prefers the env over `./tasks.db`
  (cost one stray `demo` enqueue + a live-pool claim, 2026-09-10). Any
  scratch-DB smoke MUST export `TQ_DB=<scratch path>` (or pass `--db`)
  explicitly; assume every bare `tq …` in a session shell touches production.
- ⚠️ **Session-start ritual**: run `git log --oneline -5` over the WHOLE
  repo (not just `-- internal` — concurrent work lands in cmd/, scripts/,
  and docs/ too) plus `git status` and `git stash list` before editing —
  concurrent agents land real changes mid-flight (worker's go-retry
  require, flake vendorHash fixes, AGENTS.md corrections have all arrived
  mid-session). Build on them; never revert.
- ⚠️ **Scripted history edits under the auto-commit daemon** (2026-09-10
  reword incident): inline-quoted `GIT_EDITOR`/`GIT_SEQUENCE_EDITOR` values
  get mangled through the mvdan/sh → git handoff and can SILENTLY no-op —
  use script files, and dry-run the edit against `git log -1 --format=%B`
  first; the daemon may commit mid-rebase (re-count the `Rebasing (N/N)`
  replay); a reword forks the lineage from any pushed twin and from release
  tags — tags are immutable, so the fork persists; verify by path
  (`git log -1 -- <path>`), not by `--grep` phrase. Full playbook:
  docs/status/2026-09-10_07-49 report §e.
- ⚠️ **Dead-export audits must use SUBSTRING matching**: `rg -w Symbol`
  misses suffixed references (`NewSink`, `NewCommandExecutor` use `Sink`,
  `CommandExecutor`) and undercounts — the 2026-09-10 re-derivation found
  most "dead" exports alive once matching corrected.
- ⚠️ **This repo is dogfooded (since 2026-09-07)**: the production pool is
  the SystemNix systemd deployment (`tq-agent-pool` + `tq-serve` on
  127.0.0.1:8100, journal `/mnt/pool/services/tq/tq.db`, binary pinned by
  the SystemNix input — input flip + `nix run .#deploy` are owner-run)
  working CV, SystemNix and THIS repo; agents are GLM-5.3-Flash via each
  repo's bootstrap-managed `.crushrc` (model+effort live ONLY there — a
  pool `--model` would reset reasoning effort). `.tq-verify` is the gate;
  unchecked TODO_LIST items are live pool food; expect autonomous commits
  on master (never pushes). Sibling repos on the same rails:
  `project-discovery-sdk` (queue-facing "Open Work" checkbox section only —
  the status tables below are NOT parsed), `overview` (checkbox backlog;
  owner-gated items carry `— BLOCKED:`), `project-discovery-daemon` (rails
  only). If the pool misbehaves: stop it, review `tq dlq`, rescue or
  cancel. The repo-root `tasks.db` is the retired 2026-09-07 dogfood
  journal (its orphan serve was stopped 2026-09-10).
- ⚠️ **Pool-deploy failure mode** (2026-09-10): `harvest: skipped
  reason="scan failed"` for EVERY repo means environment, not data — bare
  `--repos` names once resolved via the service cwd (now expanded against
  `--projects-dir`), and systemd's default service PATH has no git/go/crush
  (the NixOS module now sets `services.tq-agent-pool.agentPath`). The
  aggregate skip log carries one full example reason; scan failures log at
  WARN.
- ⚠️ **Pool verify runs WITHOUT GOEXPERIMENT=jsonv2 → env-only gate
  failures** (2026-09-11, task 000001a08ebf, 3 attempts burned): the
  tq-agent-pool unit env has no GOEXPERIMENT and `~/.config/go/env` is an
  EMPTY home-manager store symlink (`go env -w` is refused against the
  read-only store; the user's `env.local` jsonv2 line and `env.backup`
  show a prior manual attempt being eaten at activation). So every
  root-module `.tq-verify` run outside the flake devShell dies on the
  encoding/json/v2 build constraints REGARDLESS of repo state. Owner fix:
  `Environment=GOEXPERIMENT=jsonv2` on the NixOS tq-agent-pool module
  (same treatment as agentPath). Until then: a task.verify failure with
  that error is the environment lying, not a regression — re-run the gate
  with the export before judging the work.

## Relation to other projects

Semantics proven in go-cqrs-lite (facts/journal) and PapDashboard (worker
pools over durable queues); composes with both, depends on neither.

**PapDashboard bridge**: `tq worker --alert-url http://<pap>:8080
--alert-api-key <KEY>` (env `TQ_PAP_URL`/`TQ_PAP_API_KEY`). Dead letters
raise `alert.triggered` (fact Seq = Idempotency-Key); a later completion
posts `alert.resolved` — rescue flows close their own alerts. The watermark
starts at head per bridge process; incidents fired while down are not
replayed (review via `tq dlq`). Dead-pool detection
(`tq agent-pool --dead-pool-ticks`, default 3) is the one DIRECT-notify
path: `NotifyDeadPool` posts trigger/resolve outside the journal because
harvest skips are process-local observations; delivery is best-effort.
Details: FEATURES.md, CHANGELOG.md.

**httputil** (`~/projects/httputil`, sibling middleware library): assessed
2026-09-10, verdict NOT adopted as a dependency. Two reasons: (1) LICENSE is
Proprietary while this repo is MIT with an all-MIT dep tree — a code-level
require would poison every tq binary redistribution (don't re-litigate
without relicensing); (2) scope mismatch — tq serve is a loopback SSE
dashboard whose bespoke middlewares are deliberately narrower and stricter
than httputil's generic defaults (nonce-CSP `default-src 'none'` vs
`RecommendedCSP` `default-src 'self'`; `no-referrer` vs
`strict-origin-when-cross-origin`; SSE needs `WriteTimeout=0` + Flush
forwarding the generic stack doesn't model). The one real gap found —
`Permissions-Policy` — was ported natively into `securityHeaders`
(webui.go), and the write-route CSRF/lockout pair must keep its ADR-0003
treatment regardless of library availability. Re-run the comparison only if
the license changes or a second HTTP surface appears.
