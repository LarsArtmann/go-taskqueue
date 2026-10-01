# go-taskqueue — Agent Guide

Projects-aware task work queue / worker pool for Go. Embedded SQLite journal,
lease-based claims with crash reclaim, DAG dependencies, retries with a
dead-letter queue, pluggable executors (including headless AI coding agents).
Zero external services — one Go binary, one file.

**STATUS: v0.3.0 shipped, master CI green; actively developed by MULTIPLE
concurrent agents.** Re-read files and re-run tests before editing; expect
uncommitted changes from parallel sessions — read them, judge them, build
on them, never revert them. (Size: plan M89 budget ≤15,000 B, enforced by
cmd/tq TestAgentsDocSizeGuard — prune in-place, never grow; incident
history: docs/status/, design docs: docs/planning/, docs/adr/.)

## Commands

```bash
./scripts/ci-local.sh     # pre-push gate: full CI replicant (vet/build/race/smokes/nix); transient foreign breaks retry (45s ×3)
export GOEXPERIMENT=jsonv2 GOTOOLCHAIN=auto; go build ./... && go vet ./... && go test ./... -race   # verify gate (ROOT MODULE ONLY; GOTOOLCHAIN=auto REQUIRED outside the flake devShell — go.mod declares go 1.27.1)
nix build                 # nix run .#test = tests; nix run .#webui-css = stylesheet
./scripts/fuzz/nightly.sh # FuzzParseRepo campaign
```

- **Multi-module repo (ADR-0011)**: `internal/{task,journal,queue,executor,worker}`
  are sub-modules plus `queue/{sqlite,postgres}` backends and the
  `journal/cqrs` adapter; root is the app layer. `cmd/tq` is its own
  replace-free module (ADR-0017) — build via `scripts/build-tq.sh` (devmod
  shim). `./...` never descends into nested modules; the canonical
  per-module gate:

```bash
for m in $(find internal task journal queue executor worker -name go.mod | sed 's|/go.mod$||' | sort); do
  ( cd "$m" && export GOEXPERIMENT=jsonv2 && GOWORK=off go build ./... && GOWORK=off go vet ./... && GOWORK=off go test ./... -count=1 ) || exit 1
done
./scripts/test-cmd-tq.sh   # cmd/tq module gate (CMD_TQ_OS=windows cross-compile)
```

**Public facades (ADR-0016):** `task/ journal/ queue/ queue/sqlite/
queue/postgres/ executor/ worker/` re-export internals via type aliases —
the only importable surface for external consumers; in-repo code imports
`internal/…` directly. Every internal module in a facade's graph needs a
require (real tag) AND a relative replace; facade tests may import internal
packages, never sibling facades. Parity gated by
`scripts/check-facade-parity.sh`; postgres `OpenWithPool` pools are
CALLER-OWNED. No go.work — replace-only by design; `go test ./internal/foo`
from root fails by design (cd into the module). Release flow + version
surfaces: docs/release/. Proxy verification is rc-captured (`go get @vX.Y.Z`
+ sentinel probe in /tmp).

Smokes (CI-safe): scripts/smoke/{webui,status-loop,dogfood-once,
bootstrap-install,journal-drift,help-text,multi-repo,papdashboard-e2e,
questions-e2e,ratelimit-e2e,fullcore,reviews,session-close}.sh. Guards:
check-guard-wiring.sh, check-go-mods.sh, check-dead-exports.sh, check-script-syntax.sh,
check-facade-parity.sh, check-transient-retry.sh, check-mirror-clones.sh,
smoke/release-gates.sh, check-gosec.sh, lint-baseline.sh.
`scripts/new-module.sh <dir> [deps…]` scaffolds module go.mods.

## Architecture

Facts-first: every state change is an immutable fact in an append-only
journal; queue views, retry state, and the DLQ are projections. Claim
exclusivity = lease TTL + expiry reclaim. Domain vocabulary:
docs/DOMAIN_LANGUAGE.md.

| Package | Purpose |
| --- | --- |
| `internal/task` | Task record, Status enum, sentinel errors |
| `internal/journal` | Fact types, append-only Journal, MemoryJournal |
| `internal/journal/cqrs` | Read-only go-cqrs-lite adapter (ADR-0014, PROPRIETARY dep) |
| `internal/queue` | Store contract, Filter, Queue facade |
| `internal/queue/sqlite`, `/postgres` | Backends; shared conformance suite in `internal/queue/companion/conform` |
| `internal/worker` | Claim → heartbeat → execute loop; requeue ladder |
| `internal/bridge` | papdashboard alerts, cqa findings → fix tasks |
| `internal/executor` | sh, HTTP, agent, review, status executors + registry |
| `internal/harvest` | TODO_LIST.md → tasks; drift audit; prune-stale |
| `internal/budget` | Daily-cap + session-usage projections, checked per tick |
| `internal/dlqfix` | DLQ autopsies (`--dlq-fix`); gate-artifact auto-dismiss |
| `internal/review` | Review sweeper + `--review-autofix` |
| `internal/status` | Done-prompt report sweeper (`--status-every`) |
| `internal/prioritize` | AI batch scorer (`--prioritize`), priority_scores cache |
| `internal/depsweep` | Dependency-upgrade sweeper (`--dep-sweep`) |
| `internal/watermark` | Durable journal cursor shared by the sweepers |
| `internal/consumer` | Journal dispatcher, per-subscriber cursors (ADR-0009) |
| `internal/runactor` | run.Group actors, LIFO shutdown, InterruptOn |
| `internal/webui` | Live dashboard (`tq serve`): tailer → hub → SSE (ADR-0003) |
| `internal/httpapi` | Machine API (`tq api`): token-mandatory, nosniff, lockout |
| `internal/httpauth`/`lockout` | Shared bearer primitives + 3-strikes limiter |
| `cmd/tq` | CLI (enqueue/worker/harvest/agent-pool/bootstrap/stats/audit/dlq/ask/serve/api/…, see `tq --help`) |

### Store invariants (do not break)

- **Single serialized writer**: `sqlite.Open` sets `MaxOpenConns(1)` + WAL +
  `busy_timeout`. Never drop the `RowsAffected()` re-checks.
- **Task contexts survive pool shutdown** (bounded only by
  `--task-timeout`) — never add a shared drain deadline.
- **Facts in the same tx as state**, or it didn't happen.

### Payload contracts (short form — code owns detail)

- **`sh`**: payload is the shell line (raw / JSON string / `{"cmd":"…"}`).
- **`agent`**: `AgentPayload` (repo, prompt, verify, timeout). Verify must
  exit 0 and be ENV-SELF-CONTAINED (minted verifies carry
  `GOEXPERIMENT=jsonv2` — agents never edit their own gate). Model+effort
  live ONLY in the repo `.crushrc` managed block (`tq bootstrap`; pool
  always wants xhigh). `--task-closeout` resumes the EXACT session for the
  a)-g) self-review at `docs/status/<ts>_task-<id>.md`.
- **Derived outcomes**: the queue derives what a run did — commits via
  exactly ONE `Task-Queue-ID` footer per commit, LAST trailer line
  (`executor.GitLogScanner`; a footer above an attribution block is
  invisible — the commit-msg hook rejects that), files via `git diff-tree`,
  session usage via go-crush-data. No stdout self-report.
- **Verdict channel**: paid turns record structured results via
  `tq verdict '<json>'` into `$TQ_RESULT_FILE` (file outranks the legacy
  stdout `TQ_RESULT:` line, last-line-wins).
- **Batched harvest** (`--batch-items`, default OFF): N adjacent
  same-section items → one task; dedup `batch:` + hash of SORTED member
  keys; gates treat a batch as ONE task. Direct `tq enqueue` stays
  forbidden; work prompts grant the backlog move.
- **`review`/`dlqfix`/`status`/`prioritize`/`depbump`**: verdict-gated or
  deterministic turns on the closeout-free agent clone; session usage
  derived for ALL paid turns (drift-pinned in budget_test). review findings
  are commit-anchored (verbatim `anchor` or REJECTED). Dead agent tasks
  mint ONE autopsy (`dlqfix:<id>`); a gate-artifact death
  (`executor.IsGateArtifactDeath`) WITH shipped-proof auto-dismisses.
- **Session-close bridge**: `tq session begin/close` + `tq crush` wrapper;
  close scans `Crush-Session:` trailers, mints ONE review + ONE status task.
- **PapDashboard questions**: `tq ask --task <id>` parks the task WITHOUT
  burning an attempt; the AnswerPoller routes answers home. No prompt
  teaches `tq ask`.
- **Idempotent enqueue**: `DedupKey` re-enqueue returns the stored task;
  COMPLETED keys are refused with `ErrTaskDone`; a cancelled/dead key still
  suppresses — the harvested-item escape hatch is editing the item text.
- **Rate limits (429)**: `executor.DetectRateLimit`; requeue WITHOUT
  burning an attempt (jittered wait, fallback 15min cap 6h); per-repo
  in-process gates fast-refuse siblings; a closeout 429 arms
  `closeoutPending` resume on re-claim.
- **Secrets redaction** (default ON, `--redact=false` off): every output
  tail passes `internal/executor/redact.go`; `tq audit --journal` reports
  SECRET EVIDENCE rows.
- **Enqueued-fact snapshots are THIN today** (`{project,type}`;
  `Caps.EnqueuedSnapshot=false` pinned in the conform suites).

### Operational contracts

- **SQLite migrations** in `migrate()`: schema const → pragma-check + ALTER
  for legacy DBs → new-column indexes AFTER the column exists.
- **Watermarks**: checkpoint AFTER the batch's last accepted fact; a failed
  checkpoint gates forwarding; `tq watermarks show/set`; replay is
  idempotent (seq-derived keys).
- **Priority (ADR-0015)**: claim order = STORED priority + aging (3d/pt,
  cap 10); stored value never mutates. Markers `— P[1-4]` stripped before
  the dedup hash. One ladder everywhere: startup/`tq reprioritize`/AI
  cache (marker > AI > keyword).
- **prune-stale**: cancels PENDING tasks whose item is `[x]` or gone;
  agent-pool sweeps once synchronously at start (`--prune-stale=false`
  skips).

## Conventions

- Table-driven tests, plain `testing`; sentinels in `internal/task/errors.go`.
- **Claims carry citations** (gate run or SHA; never cite a running gate);
  filter-scope claims cite file:line or a pinning test.
- **Verify-window minimum battery**: cheap gates at HEAD (check-doc-refs.sh,
  root build+vet, check-dead-sha-refs.sh, date-measured report filename) +
  one fresh delta; expensive gates inheritable only from a same-HEAD prior
  report; nested-module claims need in-module `GOWORK=off` tests.
- **Verify-only re-dispatch checklist** (a-g): substance-read at HEAD,
  re-cite, fresh battery with rc captured TO A FILE (no PIPESTATUS in
  agent sessions), no-delta statement, dated DONE re-verified annotation,
  cause, `-v` + PASS COUNT for conform `-run`.
- docs/status reports follow the a)-g) close-out skeleton.
- **Edit→commit→battery ordering**: commit doc edits immediately; no
  unstaged edits survive a >30s gate (the daemon sweeps footer-less);
  mechanical form `scripts/commit-task.sh <id> <subject> <file>…`.
- Pure-Go deps only (`CGO_ENABLED=0`); Go 1.26+ idioms deliberate
  (`errors.AsType[E]`, `strings.SplitSeq`, `for range n`) — don't undo.
- Retry loops use `github.com/larsartmann/go-retry` (supervisor loops and
  persisted domain backoff exempt).
- **Executor shared seams — never hand-roll**: `decodePayload[T]`,
  `recordRunOutcome`, `executor.Excerpt`, `prepareRepo`, `payloadTimeout`;
  cross-backend mirror clones gated STRICT by `check-mirror-clones.sh`
  (shared surface `internal/queue/companion`). Residual art-dupl groups
  are accepted classes — don't abstract them into existence.
- POSIX-only suites carry `//go:build unix`; tests hermetic (nix checkPhase
  has no host tools).
- Generated `*_templ.go` + minified `app.css` COMMITTED; after template
  edits run `templ generate` from the REPO ROOT + `nix run .#webui-css`.
- Web UI uses `templ-components` (adoption table:
  `internal/webui/ADOPTION.md`, pinned by in-package guard tests); REJECTED
  `display.Eyebrow`, `KanbanBoard`, errorpage, `icons.Render`.
- `TODO_LIST.md` machine-consumed: `- [ ]`, one per line, never tables;
  `— BLOCKED: <reason>`; items must be agent-executable
  (`check-todo-list.sh`).
- Status reports indexed on creation (`check-status-index.sh`; the daemon
  bypasses hooks — AMEND MANEUVER for daemon-folded reports); CHANGELOG
  append-only.
- Docs formatting MANUAL (dprint on-demand only).
- Evidence archives: `scripts/archive-evidence.sh`; gated by
  `check-ghost-archives.sh`; run `git check-ignore -v` BEFORE copying
evidence into the repo.

## Known Issues

- **Concurrent agents commit constantly** (auto-commit daemon): re-run
  `go test ./... -race` before declaring success; never generate Go source
  via heredocs; build fixtures under /tmp or in ONE create+assert+trash
  chain — scratch fixtures in gated trees are daemon-food.
- **Agent shells inherit `TQ_DB`** (the PRODUCTION journal) — scratch
  smokes MUST export `TQ_DB=<scratch>`.
- **Session shell hazards**: no usable `PIPESTATUS`; bare `unset VAR` still
  leaks to children (use `env -u VAR`); multi-file `tail` fails; `printf
  %.0s` yields empty; compound-chain `&` backgrounds the wrong span.
- **Root builds auto-use `vendor/`** — after internal/ changes run
  `go mod vendor` before root builds.
- **GOEXPERIMENT/GOTOOLCHAIN**: ci-local exports jsonv2 itself; CI setup-go
  is PINNED to 1.27.1 = the go.mod floor; NEVER lower a module's `go`
  directive (stdlib-only leaves have no floor; `check-go-mods.sh` gates).
- **golangci-lint is advisory** (~1.4k baseline, growth gated by
  `scripts/lint-baseline.sh --check`); hard gates: vet + gofmt + tests.
  Regen only on a green tree and after `golangci-lint cache clean`. A
  variable rename must never change a string literal.
- **gosec**: all findings triaged/excluded via `scripts/check-gosec.sh`;
  a future finding is a NEW class needing fresh triage.
- **`tq serve`/`tq api` security**: loopback + read-only default; write
  routes CSRF-guarded with lockout; non-loopback binds need `--auth-token`.
  SECURITY.md. No new write endpoints without the same treatment.
- **templ + cmd/tq LSP diagnostics are false positives** — trust the CLI
  gates, never "fix" them.
- **VendorHash drift**: after go.mod changes run
  `nix build .#checks.x86_64-linux.vendor-hash`, copy `got:` to flake.nix.
- **History policy**: NEVER reword/amend/rebase PUSHED commits; no force
  push; scripted history edits via script files.
- **Kernel ETXTBSY anomaly**: execve of fresh binaries intermittently fails
  — `execWithTransientRetry` covers runAgent; route new exec sites there.
- **Pool verify gate / vendor gofmt**: verify failures whose tail shows
  all-ok packages + a `gofmt -l` stage are the gitignored-vendor
  environmental bug (`vendor-gofmt`); recovery: trash vendor/ + rescue.
- **Flakes see only git-tracked files** — `git add` before `nix build`.

## Relation to other projects

- **go-cqrs-lite IS the platform (ADR-0019)**: `internal/queue/{sqlitev4,
  postgresv4,cqrsqlite}` + `internal/queue/companion` are the staged
  adoption (plan: docs/adr/0019). The `journal/cqrs` adapter is a
  PROPRIETARY read-only dep — never extend or import below root.
- **PapDashboard bridge**: `--alert-url/--alert-api-key` — dead letters
  raise `alert.triggered`, completions resolve; `NotifyDeadPool` is the
  direct dead-pool path (best-effort).
- **go-health-dashboard** (MIT): ADOPTED — `tq serve` mounts `/health`
  inside the token gate, tq's own CSS, same-origin Datastar SDK.
- **httputil**: NOT adopted (proprietary license + scope mismatch).
- **go-nix-helpers**: flake input only (`flakeModules.go-standard`);
  consumed from GitHub via flake.lock, never the local checkout.
