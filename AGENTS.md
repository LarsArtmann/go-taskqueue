# go-taskqueue — Agent Guide

Projects-aware task work queue / worker pool for Go: embedded SQLite
journal, lease claims + crash reclaim, DAG deps, retries + DLQ, pluggable
executors (incl. AI agents). One Go binary, one file, zero services.

**STATUS: v0.3.0 shipped; MULTIPLE concurrent agents** — re-read files,
re-run tests; uncommitted parallel changes: read, judge, build on, never
revert. (Size guard: TestAgentsDocSizeGuard; twin check-agents-size.sh)

## Commands

```bash
./scripts/ci-local.sh # pre-push gate; transient foreign breaks retry 45s ×3
./scripts/root-gate.sh # root verify gate; ONE retry on known-flaky signatures
nix build # nix run .#test; .#webui-css
./scripts/fuzz/nightly.sh
```

- **Multi-module repo (ADR-0011)**: `internal/{task,journal,queue,executor,worker}` +
  `queue/{sqlite,postgres}` + `internal/journal/cqrs` sub-modules;
  `internal/{readmodel,composition}` own modules (S4 root; cmd/tq pins
  nominal v0.3.0); root = app layer. `cmd/tq` replace-free (ADR-0017),
  built via `scripts/build-tq.sh` (devmod shim). `./...` skips nested
  modules; per-module gate:

```bash
for m in $(find internal task journal queue executor worker -name go.mod -printf '%h\n'|sort); do (cd "$m" && GOEXPERIMENT=jsonv2 GOWORK=off go build ./... && GOWORK=off go vet ./... && GOWORK=off go test ./... -count=1)||exit 1; done
./scripts/test-cmd-tq.sh   # cmd/tq module gate (CMD_TQ_OS=windows cross-compile)
```

**Public facades (ADR-0016):** `task/ journal/ queue/ queue/sqlite/
queue/postgres/ executor/ worker/` re-export internals via type aliases —
the only external import surface; in-repo code imports `internal/…`. Facade
modules: require (real tag) + relative replace; facade tests import
internals, never sibling facades; parity via `scripts/check-facade-parity.sh`
BEFORE staging (pre-commit gates staged files only); postgres `OpenWithPool`
pools CALLER-OWNED. No go.work — replace-only (`go test ./internal/foo` from
root fails by design — cd in). Release flow: docs/release/ (rc-captured
proxy checks).

Smokes (CI-safe): `scripts/smoke/`. Guards: check-*.sh + release-gates +
lint-baseline. `scripts/new-module.sh <dir> [deps…]` scaffolds go.mods.

## Architecture

Facts-first: every state change is an immutable fact in an append-only
journal; queue views, retry state, DLQ are projections. Claim exclusivity =
lease TTL + expiry reclaim. Vocabulary: docs/DOMAIN_LANGUAGE.md.

| Package | Purpose |
| --- | --- |
| `internal/task` | Task record, Status, sentinels |
| `internal/journal` | Fact types + append-only Journal |
| `internal/journal/cqrs` | Read-only go-cqrs-lite adapter (ADR-0014) |
| `internal/queue` | Store contract, Filter, Queue facade |
| `internal/queue/{sqlite,postgres}` | Thin drivers over the v4 adapters; conform: `internal/queue/companion/conform` |
| `internal/queue/{sqlitev4,postgresv4,cqrsqlite}` | tq Store over the go-cqrs-lite queue engines |
| `internal/queue/companion` | Shared tq surfaces: reads, watermarks, scores, exclusivity |
| `internal/readmodel` | S3 metaengine projection (`<db>.readmodel.db`; `--read-model` default ON) |
| `internal/composition` | S4 root: `system.New` over the projection home (`tq serve`) |
| `internal/worker` | Claim → heartbeat → execute loop; requeue ladder |
| `internal/bridge` | papdashboard + cqa bridges → fix tasks |
| `internal/executor` | sh/HTTP/agent/review/status executors + registry |
| `internal/harvest` | TODO_LIST.md → tasks; drift audit; prune-stale |
| `internal/budget` | Daily-cap + session-usage projections per tick |
| `internal/dlqfix` | DLQ autopsies (`--dlq-fix`); gate-artifact auto-dismiss |
| `internal/review` | Review sweeper + `--review-autofix` |
| `internal/status` | Done-prompt report sweeper (`--status-every`) |
| `internal/prioritize` | AI batch scorer (`--prioritize`), priority_scores cache |
| `internal/depsweep` | Dependency-upgrade sweeper `--dep-sweep` |
| `internal/watermark` | Durable journal cursor shared by the sweepers |
| `internal/consumer` | Journal dispatcher, per-subscriber cursors (ADR-0009) |
| `internal/runactor` | run.Group actors, LIFO shutdown, InterruptOn |
| `internal/webui` | Live dashboard (`tq serve`): tailer→hub→SSE (ADR-0003) |
| `internal/httpapi`+`httpauth`/`lockout` | Machine API (`tq api`): token-mandatory, nosniff, lockout; shared bearer + 3-strikes |

### Store invariants

- **Single serialized writer**: `sqlite.Open` sets `MaxOpenConns(1)` + WAL +
  `busy_timeout`. Never drop the `RowsAffected()` re-checks.
- **Task contexts survive pool shutdown** (bounded only by
  `--task-timeout`) — never add a shared drain deadline.
- **Facts in the same tx as state**, or it didn't happen.

### Payload contracts (code owns detail)

- **`sh`**: payload is the shell line (raw / JSON string / `{"cmd":"…"}`).
- **`agent`**: `AgentPayload` (repo, prompt, verify, timeout). Verify: exit
  0, ENV-SELF-CONTAINED (minted verifies carry `GOEXPERIMENT=jsonv2` —
  agents never edit their own gate). Model+effort ONLY in the repo
  `.crushrc` managed block. `--task-closeout` resumes the EXACT session
  for its a)-g) report (`docs/status/<ts>_task-<id>.md`).
- **Derived outcomes**: the queue derives what a run did — commits via
  exactly ONE `Task-Queue-ID` footer as LAST trailer line
  (`executor.GitLogScanner`; a footer above the attribution block is
  invisible, hook rejects), files via `git diff-tree`, usage via
  go-crush-data. No stdout self-report.
- **Verdict channel**: paid turns record results via `tq verdict '<json>'`
  into `$TQ_RESULT_FILE` (file > legacy stdout, last-wins).
- **Batched harvest** (`--batch-items`, default OFF): N adjacent
  same-section items → one task (dedup `batch:` + SORTED-key hash; gates
  see ONE task); direct `tq enqueue` forbidden — work prompts grant the move.
- **`review`/`dlqfix`/`status`/`prioritize`/`depbump`**: verdict-gated or
  deterministic turns on the closeout-free agent clone; usage derived for
  ALL paid turns. review findings are commit-anchored (verbatim
  `anchor` or REJECTED). Dead agent tasks mint ONE autopsy (`dlqfix:<id>`);
  a gate-artifact death (`executor.IsGateArtifactDeath`) WITH shipped-proof
  auto-dismisses.
- **Session-close bridge**: `tq session begin/close` + `tq crush` wrapper;
  close scans `Crush-Session:` trailers → ONE review + ONE status task.
- **PapDashboard questions**: `tq ask --task <id>` parks the task WITHOUT
  burning an attempt; the AnswerPoller routes answers home.
- **Idempotent enqueue**: `DedupKey` re-enqueue returns the stored task;
  COMPLETED keys refused (`ErrTaskDone`); cancelled/dead keys still
  suppress (escape hatch: edit the item).
- **Rate limits (429)**: `executor.DetectRateLimit`; requeue WITHOUT burn
  (jittered wait, fallback 15min cap 6h); per-repo gates fast-refuse
  siblings; closeout 429 arms `closeoutPending` resume on re-claim.
  **Env-requeue breaker**: env requeues carry `requeue_class`; 3 consecutive
  burn an attempt + escalate (`env-streak`) — never uncap
  (169-loop = $36.62/d).
- **Secrets redaction** (default ON): every output tail passes
  `internal/executor/redact.go`; `tq audit --journal` reports SECRET
  EVIDENCE rows. Growth: a new provider/shape adds ONE `secretPatterns`
  entry + a fake-shape sample in `redact_test.go` (table-length pins) + a
  lint-baseline note if it trips golangci/gosec. `SecretHits` and
  `RedactSecrets` share the ONE table (pinned
  `TestSecretHitsAndRedactionCompileIdenticalTable`); redact_test.go
  build-tag-free, windows-gated per-module CI (00-55 §f9).
- **Enqueued-fact snapshots are THIN today** (`{project,type}`;
  `Caps.EnqueuedSnapshot=false` pinned in the conform suites).

### Operational contracts

- **SQLite migrations** in `migrate()`: schema const → pragma + ALTER
  for legacy DBs → indexes AFTER the column exists.
- **Watermarks**: checkpoint AFTER the batch's last accepted fact; failed
  checkpoint gates forwarding; `tq watermarks show/set`; replay idempotent
  (seq-derived keys).
- **Priority (ADR-0015)**: claim order = STORED priority + aging (3d/pt,
  cap 10); stored value never mutates. Markers `— P[1-4]` stripped before
  the dedup hash. ONE ladder everywhere: startup/reprioritize/AI cache
  (marker > AI > keyword). Probes enter `--priority ≥90` (fresh
  low-priority work starves behind aging).
- **prune-stale**: cancels PENDING tasks whose item is `[x]` or gone;
  pools sweep once at start.
- **Claim-time budget gate**: paid turns block once the daily cap/budget-cmd is spent — requeue class `budget`, no burn, parked to midnight (cap) / 15m (cmd), outside env-streak. Claim-path checks run CHEAP-FIRST (budget hook → preflight → executor) — a gated pool never spawns a process for a verdict the hook already knows.

## Conventions

- Table-driven tests, plain `testing`; sentinels in `internal/task/errors.go`.
- **Claims carry citations** (gate run or SHA; never a running gate);
  filter-scope claims cite file:line or a pin.
- **Verify-window battery**: cheap gates at HEAD (check-doc-refs, root
  build+vet, check-dead-sha-refs, date-named report) + one fresh delta;
  expensive gates inherit from a same-HEAD report; nested-module claims
  need in-module `GOWORK=off` tests; closeouts touching root-guard-parsed
  files (AGENTS/README/TODO_LIST, doc pins) cite ROOT build+vet+test
  -race rc. Re-dispatch: newest prior report + `tq show <id>` FIRST;
  battery rc TO A FILE (no PIPESTATUS); no-delta claim; dated DONE
  re-verified note; `-v` + PASS COUNT for conform `-run`.
- docs/status reports follow the a)-g) skeleton (incl. DONE-on-arrival
  re-dispatches).
- **Edit→commit→battery ordering**: stage+commit BEFORE running anything
  (daemon sweeps in <60 s, footer-less, takes STAGED files too); fold
  ferries only while local-only + contiguous + exactly-mine; mechanical
  form `scripts/commit-task.sh <id> <subject> <file>…`.
- Tab-bearing insertions go through python-heredoc replace — free-text
  edit glues tabs into comments as literal `\t`.
- Pure-Go deps only (`CGO_ENABLED=0`); Go 1.26+ idioms deliberate
  (`errors.AsType[E]`, `SplitSeq`, `for range n`) — don't undo.
- Retry loops use `github.com/larsartmann/go-retry` (supervisor loops and
  persisted domain backoff exempt).
- **Executor shared seams — never hand-roll**: `decodePayload[T]`,
  `recordRunOutcome`, `executor.Excerpt`, `prepareRepo`, `payloadTimeout`;
  mirror clones gated STRICT by `check-mirror-clones.sh` (shared surface
  `internal/queue/companion`). Residual art-dupl groups accepted; don't
  add new ones. `art-dupl:accept` directives only suppress when they sit
  within a few lines ABOVE the clone's first line: doc-position works for
  func-top clones, deeper ones need an inline comment adjacent to the
  start; the `fragments.templ` conditional-span pair is the ONE accepted
  residual (Go comments cannot live inside templ markup).
- POSIX-only suites carry `//go:build unix`; tests hermetic (nix checkPhase
  lacks host tools).
- Generated `*_templ.go` + minified `app.css` COMMITTED; after template
  edits run `templ generate` from REPO ROOT + `nix run .#webui-css`.
- Web UI uses `templ-components` (guard-pinned; table
  `internal/webui/ADOPTION.md`); REJECTED `display.Eyebrow`,
  `KanbanBoard`, errorpage, `icons.Render`.
- `TODO_LIST.md` machine-consumed: `- [ ]`, one per line, never tables;
  `— BLOCKED: <reason>`; items agent-executable (`check-todo-list.sh`);
  DONE-row notes collapse to ONE note + latest-report pointer.
- Status reports indexed on creation (`check-status-index.sh`; daemon
  bypasses hooks — AMEND MANEUVER for daemon-folded reports); ONE index-row
  write point: top chronological cluster. CHANGELOG append-only.
- Docs formatting MANUAL (dprint on-demand).
- Evidence archives: `scripts/archive-evidence.sh` (gated by
  `check-ghost-archives.sh`); `git check-ignore -v` BEFORE copying
  evidence in.

## Known Issues

- **Concurrent agents commit constantly** (auto-commit daemon): re-run
  `go test ./... -race` before declaring success; never generate Go source
  via heredocs; fixtures under /tmp (gated trees are daemon-food).
  Footer-less daemon `chore:` sweeps heal via
  `scripts/heal-daemon-sweep.sh [--from <ref>] <Task-Queue-ID>` (unpushed
  only; backup on failure; `--self-test` pins rails).
- **TestExactlyOnceUnderConcurrency is load-flaky** (worker): host build
  storms can drop it to "19/20 completed"; fails at clean parents too —
  re-run before attributing.
- **Agent shells inherit `TQ_DB`** (the PRODUCTION journal) — scratch
  smokes MUST export `TQ_DB=<scratch>`.
- **Session shell hazards**: no usable `PIPESTATUS`; bare `unset VAR` leaks
  to children (use `env -u VAR`); multi-file `tail` fails; `printf %.0s`
  yields empty; chained `&` backgrounds the wrong span.
- **Root builds auto-use `vendor/`** — after internal/ changes run
  `go mod vendor` first.
- **GOEXPERIMENT/GOTOOLCHAIN**: ci-local exports jsonv2; CI setup-go
  PINNED to 1.27.1 = go.mod floor; NEVER lower a `go` directive
  (`check-go-mods.sh` gates).
- **Host GOCACHE/GOROOT hazards**: gates run with `GOCACHE=/tmp/go-build-cache`
  (host symlink → /nix/store ENOSPCs); "package X is not in std" + a
  `$GOMODCACHE/toolchain@…` GOROOT = corrupt extraction (chmod -R u+w +
  trash, or `GOTOOLCHAIN=local`).
- **golangci-lint is advisory** (~1.4k baseline, growth gated by
  `scripts/lint-baseline.sh --check`); hard gates: vet+gofmt+tests; regen
  only on a green tree after `golangci-lint cache clean`.
- **gosec**: all findings triaged via `scripts/check-gosec.sh`; a new
  finding is a new class needing fresh triage.
- **`tq serve`/`tq api` security**: loopback + read-only default; write
  routes CSRF-guarded with lockout; non-loopback binds need `--auth-token`
  (SECURITY.md). No new write endpoints without it.
- **templ + cmd/tq LSP diagnostics are false positives** — trust the CLI
  gates, never "fix" them.
- **VendorHash drift**: after go.mod changes run
  `nix build .#checks.x86_64-linux.vendor-hash`, copy `got:` to flake.nix.
- **History policy**: NEVER reword/amend/rebase PUSHED commits; no force
  push; scripted edits via script files.
- **Kernel ETXTBSY**: fresh-binary execve intermittently fails — route
  exec sites through `execWithTransientRetry`.
- **gofmt gates are SCOPED to non-gitignored files**
  (`executor.ScopedGofmtStage`, doctor `gofmt:<repo>`):
  `gofmt -l . | git check-ignore --stdin -v --non-matching | grep '^::'` —
  unscoped re-plants the vendor-gofmt death class (94% of the DLQ); grep
  the FULL verify log before attributing a gate death.
- **Flakes see only git-tracked files** — `git add` before `nix build`.

## Relation to other projects

- **go-cqrs-lite IS the platform (ADR-0019)**: adoption in the
  `{sqlitev4,postgresv4,cqrsqlite}` + `companion` modules;
  `internal/journal/cqrs` PROPRIETARY, read-only — never extend or import
  below root. **Backward auto-upgrade (endgame P1)**: facade Open converges
  a legacy pre-flip DB in place (snapshot `<db>.legacy-*.bak`, verify,
  auto-restore on mismatch; absent feature-era tables tolerated); refuse
  via `TQ_NO_AUTO_UPGRADE=1`; manual: `go run ./replay` in
  internal/queue/sqlitev4.
- **PapDashboard bridge**: `--alert-url/--alert-api-key`; dead letters
  raise `alert.triggered`, completions resolve; `NotifyDeadPool` =
  direct dead-pool path.
- **go-health-dashboard** (MIT): ADOPTED — `/health` inside the token gate,
  tq CSS, same-origin Datastar.
- **httputil** NOT adopted (proprietary); **go-nix-helpers** flake-input
  only, never the local checkout.
