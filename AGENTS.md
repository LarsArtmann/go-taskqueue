# go-taskqueue — Agent Guide

Projects-aware task queue / worker pool for Go — see README.md.

**STATUS: v0.4.1 shipped; MULTIPLE concurrent agents** — re-read files,
re-run tests; uncommitted parallel
changes: read, judge, build on, never revert (size guard: TestAgentsDocSizeGuard)

**MAJOR OPEN PROBLEMS**: [dispatch prompting + batch granularity](docs/problems/major-prompting-and-batching.md) —
redesign SHIPPED v0.3.4; follow-ups in the doc.

**READER ROUTE — read only your lanes**: executors → §Payload contracts +
§Conventions; review/dlqfix/status → §Conventions + §Operational contracts; gate
runners → §Commands + §Known Issues; store/queue → §Store invariants;
platform/docs → §Relation to other projects. Scan §Known Issues before attributing
failures; per-package maps live in doc comments.

## Commands

```bash
./scripts/ci-local.sh # pre-push gate; transient foreign breaks retry 45s ×3
  # TRANSIENT_MAX_POLLS=0: NO retry (pinned); nix/check-ci/daemon-attribution
  # unwrapped. Preflight EVERY battery: agents-size + dead-exports --self-test
  # + module test + facade-parity + ROOT vet + quiet tree.
./scripts/root-gate.sh # root verify gate; ONE retry on known-flaky signatures
./scripts/lint-baseline.sh --check --clean-cache # opt-in; regen ALWAYS cleans
nix build # nix run .#test; .#webui-css
./scripts/fuzz/nightly.sh
```

- **Multi-module repo (ADR-0011)**: `internal/{task,journal,queue,
  executor,worker}` + `queue/{sqlite,postgres}` + `internal/journal/
  cqrs` sub-modules; `internal/{readmodel,composition,retryx,dlqbackoff}` own
  modules (S4 root; cmd/tq pins the release floor); root = app layer. `cmd/tq`
  replace-free (ADR-0017), built via `scripts/build-tq.sh` (devmod shim);
  `./...` skips nested modules; per-module gate:

```bash
for m in $(find internal task journal queue executor worker -name go.mod -printf '%h\n'|sort); do (cd "$m" && GOEXPERIMENT=jsonv2 GOWORK=off go build ./... && GOWORK=off go vet ./... && GOWORK=off go test ./... -count=1)||exit 1; done
./scripts/test-cmd-tq.sh   # cmd/tq module gate (CMD_TQ_OS=windows cross-compile)
```

**Public facades (ADR-0016):** `task/ journal/ queue/ queue/sqlite/
queue/postgres/ executor/ worker/` re-export internals via type aliases —
the only external import surface; in-repo code imports `internal/…`.
Facades: require (real tag) + relative replace; tests import internals, never
sibling facades; parity via `scripts/check-facade-parity.sh` BEFORE staging;
postgres `OpenWithPool` pools CALLER-OWNED. No root go.work — replace-only
(exception: tracked `cmd/tq/go.work`, ADR-0017); a module `go`-directive bump
bumps its `go` line (`go work edit -go=<ver>`) in the same change — stale
lines fail every cmd/tq go command. Release flow: docs/release/.

Smokes (CI-safe): `scripts/smoke/`. Guards: check-*.sh (incl. `check-cite-drift`,
`check-research-note-shape`) + release-gates + lint-baseline;
`scripts/new-module.sh <dir> [deps…]` scaffolds go.mods.

## Architecture

Facts-first: every state change is an immutable fact in an append-only journal;
queue views, retry state and the DLQ are projections. Claim exclusivity = lease
TTL + expiry reclaim. Vocabulary: docs/DOMAIN_LANGUAGE.md.


| Package | Purpose |
| --- | --- |
| `internal/task` | Task record, Status, sentinels |
| `internal/journal` | Fact types + append-only Journal |
| `internal/journal/cqrs` | Read-only go-cqrs-lite adapter (ADR-0014) |
| `internal/queue` | Store contract, Filter, Queue facade, Waker |
| `internal/queue/{sqlite,postgres}` | Thin v4-adapter drivers; conform suite: `internal/queue/companion/conform` |
| `internal/queue/{sqlitev4,postgresv4}` | tq Store over the go-cqrs-lite queue engines |
| `internal/queue/companion` | Shared tq surfaces: reads, watermarks, scores, exclusivity |
| `internal/{readmodel,composition}` | S3 projection `<db>.readmodel.db` (`--read-model` ON) + S4 root: `system.New`, `ProjectionRuntime` |
| `internal/retryx` | Shared transient-retry file ops (Windows sharing violations) |
| `internal/worker` | Claim → heartbeat → execute loop; requeue ladder |
| `internal/bridge` | papdashboard + cqa bridges → fix tasks |
| `internal/executor` | sh/HTTP/agent/review/status executors + registry |
| `internal/harvest` | TODO_LIST.md → tasks; drift audit; prune-stale |
| `internal/budget` | Daily-cap + session-usage projections |
| `internal/{dlqfix,review,status}` | DLQ autopsies (`--dlq-fix`); review sweeper (`--review-autofix`); done-prompt sweeper (`--status-every`) |
| `internal/dlqbackoff` | Own module: poisoned-repo backoff fold shared by `pool-health`, `tq dlq`, webui |
| `internal/prioritize` | AI batch scorer (`--prioritize`), priority_scores cache |
| `internal/depsweep` | Dependency-upgrade sweeper `--dep-sweep` |
| `internal/watermark` | Durable journal cursor shared by the sweepers |
| `internal/consumer` | Journal dispatcher, per-subscriber cursors (ADR-0009) |
| `internal/runactor` | run.Group actors, LIFO shutdown, InterruptOn |
| `internal/webui` | Live dashboard (`tq serve`): tailer→hub→SSE (ADR-0003) |
| `internal/httpapi`+`httpauth`/`lockout` | Machine API (`tq api`): token-mandatory, nosniff, lockout, 3-strikes |
| `internal/incident` | Error reports → fix tasks (ADR-0021) |

### Store invariants

- **Single serialized writer**: `sqlitev4.Open` = ONE shared `*sql.DB`
  (MaxOpenConns(1), WAL, busy_timeout) ridden by engine AND companion — never
  a second write pool; never drop the `RowsAffected()` re-checks. The
  projection home elects separately: the `.fold-lock` flock every folding
  readmodel Open must win — a loser mounts read-only (no fold, no checkpoint);
  the stale zero-byte sidecar is steady state.
- **One deployment description (ADR-0022)**: `config.Deployment` is the only
  store-opening input — flags resolve via `config.FromFlags` (ONE TQ_SQLITE_SYNC
  reader), openers take the struct, pragma literals live ONLY in config; ONE
  engine per sqlite home — no second write pool or pragma copy.
- **SQLite IO policy**: steady-state handles run at the configured sync tier
  (NORMAL = checkpoint fsync, lost-tail-tolerant); migrations stay FULL. Claim
  probes use partial RUNNING-only indexes (companion.Migrate), never history;
  idle loops back off (cap IdlePollMax 2s).
- **Task contexts survive pool shutdown** (bounded only by `--task-timeout`)
— no shared drain deadline.
- **Facts in the same tx as state**, or it didn't happen.
- **Additive-growth contract for detail structs** (OKF v0.2 §11): a new
  journal-detail field is OPTIONAL, documents its ABSENCE meaning, and gets a
  legacy fallback read — never required nor a second writer.
- **Unix-milli timestamps**: stored timestamps are MILLISECOND-granular, so
  ordering-sensitive code must not assume sub-millisecond completion order;
  sorts need deterministic non-time tiebreaks or set-wise assertions.

### Payload contracts (code owns detail)

- **`sh`**: payload is the shell line (raw / JSON string / `{"cmd":…}`).
- **`agent` worktrees** (`--worktrees`, OFF): branch `tq/<task-id>`;
  review-approve auto-merges + reaps remnants.
- **`agent`**: `AgentPayload` (repo, prompt, verify, timeout). Verify: exit 0,
  ENV-SELF-CONTAINED (verifies carry `GOEXPERIMENT=jsonv2` — agents never
  edit their own gate). Model+effort ONLY in the repo `.crushrc` managed
  block. `--task-closeout` resumes the EXACT session for its a)-g) report
  (`docs/status/tasks/<ts>_task-<id>.md`).
- **Derived outcomes**: the queue derives what a run did — commits via ONE
  `Task-Queue-ID` footer as LAST trailer (attribution above it is invisible;
  the hook rejects), files via `git diff-tree`, usage via go-crush-data — never
  a stdout self-report.
- **Verdict channel**: paid turns record via `tq verdict '<json>'` into
  `$TQ_RESULT_FILE` (file > stdout, last-wins); prompts bake the `--file` rails
  (verdict + ask — routed bash tools see only SERVER env, crush#4210; never
  drop). `-H` sans CRUSH_CLIENT_SERVER=1 is silently LOCAL.
- **Calibration dataset**: every paid run appends ONE `CalibrationRow`
  (raw verdict-channel body + `Verify`/`Attestation` join keys + derived
  outcome counts + session usage) to `$TQ_LOG_DIR/calibration.jsonl`
  (`internal/executor/calibration.go`); best-effort — no TQ_LOG_DIR, no
  dataset. Auto-dismiss/review rules tune FROM this data, not by feel.
- **Batched harvest** (`--batch-items`, default 3): N adjacent same-section
  items → one task (dedup `batch:` + SORTED-key hash; gates see ONE task);
  direct `tq enqueue` forbidden — work prompts grant the move. Prompts are
  OUTCOME contracts: rails only, AGENTS.md owns the rules.- **`review`/`dlqfix`/`status`/`prioritize`/`depbump`**: verdict-gated or
  deterministic turns on the closeout-free agent clone; usage derived for ALL
  paid turns; findings commit-anchored (verbatim `anchor` or REJECTED); dead
  agent tasks mint ONE autopsy (`dlqfix:<id>`); gate-artifact death WITH
  shipped-proof auto-dismisses (`executor.IsGateArtifactDeath`).
- **Session-close bridge**: `tq session begin/close` + `tq crush`; close
  scans `Crush-Session:` trailers → ONE review + ONE status task.
- **PapDashboard questions**: `tq ask --task <id>` parks without attempt
  burn; the AnswerPoller routes answers home.
- **Idempotent enqueue**: `DedupKey` re-enqueue returns the stored task;
  COMPLETED keys refused (`ErrTaskDone`); cancelled/dead keys still suppress
  (escape: edit the item).
- **Rate limits (429)**: `executor.DetectRateLimit`; requeue WITHOUT burn
  (jittered wait, fallback 15min cap 6h); per-repo gates fast-refuse siblings;
  closeout 429 arms `closeoutPending` resume; the provider RIDES the requeue
  evidence (`RequeueEvidence.provider`, `tq tasks --parked`).
  **Env-requeue breaker**: env requeues carry `requeue_class`; 3 consecutive
  burn an attempt + escalate (`env-streak`) — never uncap. Dirty-tree
  preflight refusals EXEMPT (typed `ErrDirtyTree`): ladder, NO attempt burn.
- **Secrets redaction** (default ON): every output tail passes
  `internal/executor/redact.go`; `tq audit --journal` reports SECRET EVIDENCE rows. Growth: ONE `secretPatterns` entry + fake-shape sample in
  `redact_test.go` (gosec); ONE shared table test, windows-gated. Counts =
  distinct secret LOCATIONS; zero-separator back-to-back tokens
  merge into ONE location — distinct counts need a non-token separator.- **Enqueued-fact snapshots are THIN** (`{project,type}`;
  `Caps.EnqueuedSnapshot=false` pinned in conform suites).
- **Unknown keys ≠ a newer contract**: unknown detail/payload KEYS are ignored,
  but a greater payload `V` is REFUSED — a misparsed execution payload runs the
  WRONG ACTION (`payload_test.go` pins both halves).
- **Verify receipts**: each gate run records `executor.VerifyReceipt` + its
  `AttestVerify` verdict on BOTH paths; `tq show`/`tq dlq` surface it as
  `verify=<verdict>`.

### Operational contracts

- **SQLite migrations** in `migrate()`: schema const → pragma + ALTER
  for legacy DBs → indexes AFTER the column exists.
- **Watermarks**: checkpoint AFTER the batch's last accepted fact; a failed
  checkpoint gates forwarding; `tq watermarks show/set`; replay is seq-derived
  (idempotent).
- **Priority (ADR-0015)**: claim order = STORED priority + aging (3d/pt,
  cap 10); stored value never mutates. Markers `— P[1-4]` stripped before the
  dedup hash. ONE ladder everywhere (marker > AI > keyword). Probes enter
  `--priority ≥90`. **Signals, never scores**: an AI score is a SCHEDULING
  signal, never a trust tier or authority grant — trust derives only from
  verification facts (the receipt's attestation).
- **Failure-evidence surface**: `tq dlq --evidence-stage <spec>` filters the
  DLQ by the failed fact's Stage/VerifyStage/Class plus gate-env/gate-dead/
  gate-slow aliases — comma-separated, `!`-negation; `tq show` renders a
  `failure` JSON section; `tq dlq` rows + webui timeline lines carry an
  `evidence=` suffix (matcher `executor.MatchesEvidenceStageSet`, ONE copy).
- **Claim-time budget gate**: paid turns block once the daily cap/budget-cmd
  is spent — class `budget`, no burn, parked to midnight (cap) / 15m (cmd),
  outside env-streak. Claim checks run CHEAP-FIRST (budget → preflight →
  executor: gated pools never spawn a process the hook can refuse).
- **prune-stale**: cancels PENDING tasks whose item is `[x]` or gone; pools
  sweep at start.

## Conventions

- Table-driven tests, plain `testing`; sentinels in `internal/task/errors.go`.
- **No absolute verification phrases**: write "verified <date> by <method>"
  plus the PASS history — never "nothing here rests on a subagent summary".
- **Claims carry citations** (gate run or SHA; never a running gate);
  filter-scope claims cite file:line or a pin.
- **Two-tick convention for verify-before-filing rows**: TWO unchecked
  items (rebuild repro; file upstream) so NOT-reproducible ticks row
  one — never a fused RESOLVED-WITHOUT-FILING row.
- **Verify-window battery**: cheap HEAD gates (check-doc-refs, root
  build+vet, check-dead-sha-refs, date-named report) + one fresh delta;
  expensive gates inherit a same-HEAD report; nested-module claims
  need in-module `GOWORK=off` tests; root-parsed-file closeouts cite
  ROOT build+vet+test -race rc. Re-dispatch: `scripts/redispatch-brief.sh`;
  rc TO A FILE (no PIPESTATUS); dated DONE re-verified note; `-v` + PASS
  COUNT for conform `-run`. **Premise-check first**: RUN a batch item's
  premise gate (e.g. `check-agents-size.sh`) BEFORE planning — a stale
  premise ("zero headroom" was actually 380 B red) burned a window.
  **Observe-first for pinning**: CAPTURE live output and pin what was
  observed — assertion-first pinning burned three windows.
- **Gate receipts**: a report's "N ok / M fail" count is `grep -c '^ok:'`
  over a CAPTURED gate-output file — never counted in context or from a
  summary line (`go test ./... | tee /tmp/gate.log` then grep it).
- **Fleet convention — countable gate summaries are PINNED, not
  advisory**: every countable sibling gate (check-script-syntax,
  check-go-mods, check-dead-exports, check-guard-wiring; gosec too)
  carries `--self-test` pinning its summary format + a negative-path
  counter battery via an env hook; wired into ci-local; new gates too.
- **Crush PreToolUse hooks** (`scripts/hooks/`, `--self-test`):
  `tq-bash-guard` (^bash$) denies the paid-trap classes (PIPESTATUS,
  `printf %.0s`, ERE bracket `\t/\d`, unscoped `gofmt`, smokes w/o `TQ_DB=`,
  force-push, `filter-branch`, redirect→`*.go`) and advises TQ_DB/unset/
  multi-tail/chained-`&`/stale vendor/GOCACHE/GOWORK/nix/dirty-tree-gate rows;
  `tq-edit-advice`: per-path notes (templ, TODO_LIST, AGENTS size, CHANGELOG
  O14, go.mod floor, docs/status creation). Hatches in deny messages
  (force-push hatchless); smoke rule is command-position.
- docs/status reports follow the a)-g) skeleton; indexed on creation
  (`check-status-index.sh`; daemon bypasses hooks — AMEND MANEUVER for
  daemon folds); ONE index-row write point (top cluster); CHANGELOG append-only; record `tq verdict` before session end
  (report without verdict = half-closeout).
- **Dry-run-first for doc/index mutation scripts**: one-off
  AGENTS/TODO_LIST/status-index sweeps print computed rows first; the FIRST
  filter runs against a copy of the real file shape — a wrong filter is
  free to print, fatal to run.
- **Closeout TICK-AUDIT**: before the FINAL commit of any batch, grep
  TODO_LIST.md for each verbatim batch anchor; finished rows must read
  `- [x]` — landed + unchecked = paid round-trip.
- **Tick-claims cite the tick commit**: non-report ticks cite the tick
  commit; WARN gate `check-tick-claims.sh` (also WARNs on tick cites that
  are real commits but not HEAD-ancestors — never cite an UNPUSHED sha
  without a fork record `old→new`; amending orphans the sha, 02-56 class).
- **O14 CHANGELOG policy**: `[Unreleased]` carries OPERATOR-VISIBLE changes
  only — repo-internal tooling and non-operator rows go in the `### Internal`
  subsection at the release section's bottom, never Added/Changed/Fixed;
  behavior changes get an explicit `(BEHAVIOR CHANGE for …)` flag (checked
  in `scripts/check-release-docs.sh`).
- **Clock-first filenames**: run `date +%Y-%m-%d_%H-%M` BEFORE
  composing a `<ts>_<slug>` filename — a guessed `<ts>` lies.
- **Edit→commit→battery ordering**: stage+commit BEFORE running anything
  (daemon sweeps <60 s, footer-less, take STAGED files too); after ANY commit
  re-read `git log -1 --format='%(trailers:key=Task-Queue-ID,valueonly)'` —
  EMPTY output = an in-flight daemon sweep landed between your edits (heal
  while the unpushed window is open). Fold only while
  local-only + contiguous + exactly-mine (`scripts/commit-task.sh <id>
  <subject> <file>…`). In MULTI-STEP git surgery, modified-unstaged AND
  untracked files are daemon-food: a mid-sequence sweep breaks span
  contiguity, so between steps the tree must be CLEAN, not merely
  "nothing left to stage".
- **Prompt-content changes are dedup-invisible when daemon-folded**:
  re-dispatch dedup scans footer-bearing commits only — prompt-const work
  in a footer-less chore sweep is invisible to it.
  `scripts/check-prompt-const-folds.sh` (e0b1fc4f) catches the fold AFTER
  it lands; never tick a prompt-content row off
  the guard's existence alone.
- Tab-bearing insertions go through python-heredoc replace — free-text
  edit glues tabs into comments as literal `\t`.
- Pure-Go deps only (`CGO_ENABLED=0`); Go 1.26+ idioms deliberate
  (`errors.AsType[E]`, `SplitSeq`, `for range n`) — don't undo.
- Retry loops use `github.com/larsartmann/go-retry` (supervisor loops and
  persisted domain backoff exempt).
- **Executor shared seams — never hand-roll**: `decodePayload[T]`,
  `recordRunOutcome`, `executor.Excerpt`, `prepareRepo`, `payloadTimeout`;
  mirror clones gated STRICT by `check-mirror-clones.sh` (shared surface
  `internal/queue/companion`); art-dupl residuals accepted ONLY where named
  here. Templ markup cannot carry Go-comment accepts — the HTML-comment
  directive suppresses but SHIPS IN PAGE OUTPUT (upstream
  LarsArtmann/art-dupl#9, open); the sole templ twin (layout.templ
  mobile/desktop pair) is intentional. `art-dupl:accept` suppresses only a
  few lines ABOVE the clone's first line.
- POSIX-only suites carry `//go:build unix`; tests hermetic (nix checkPhase
  lacks host tools).
- Generated `*_templ.go` + minified `app.css` COMMITTED; after template
  edits run `templ generate` + `nix run .#webui-css` (REPO ROOT).
- Web UI uses templ-components (guard-pinned; table
  internal/webui/ADOPTION.md); REJECTED display.Eyebrow,
  KanbanBoard, errorpage, icons.Render.
- `TODO_LIST.md` machine-consumed: `- [ ]`, one per line, never tables;
  `— BLOCKED: <reason>`; items agent-executable (`check-todo-list.sh`);
  DONE-row notes collapse to ONE note + latest-report pointer.
- Docs formatting MANUAL (dprint on demand). Evidence archives:
  `scripts/archive-evidence.sh` (gated by `check-ghost-archives.sh`);
  `git check-ignore -v` BEFORE copying evidence in.

## Known Issues

- **Concurrent agents commit constantly** (auto-commit daemon): re-run
  `go test ./... -race` before declaring success; never generate Go source via
  heredocs; /tmp fixtures (gated trees are daemon-food). Footer-less sweeps
  heal via `scripts/heal-daemon-sweep.sh [--from <ref>] [--retitle <s>] <id>`
  (unpushed only; backup on failure; `--self-test` pins rails): REFUSES
  non-contiguous spans; a PARENT-naming refusal (foreign footer BELOW an
  exactly-mine span) = terminator shape — run its printed
  `--from <oldest-blocker>`; fallback = amend the unpushed HEAD
  once `git show --stat` proves it mine.
- **`TestExactlyOnceUnderConcurrency` (worker) and `TestSelfManagingLoop`
  (harvest) are load-flaky**: build storms drop at clean parents — re-run
  before attributing; both ride KNOWN_FLAKY (scripts/lib/verify-retry.sh).
- **Agent shells inherit `TQ_DB`** (PRODUCTION journal) — scratch smokes MUST export TQ_DB=<scratch>.
- **Session shell hazards**: no `PIPESTATUS`; bare `unset VAR` leaks to
  children (`env -u VAR`); multi-file `tail` fails; zero-repeat `printf`
  format yields empty; chained `&` backgrounds the wrong span; `kill` cannot
  signal LIVE pids (rc=2) — python3 `os.kill`. POSIX ERE: `\t` in a grep
  bracket class is a literal `t` — use `[[:space:]]`/`[[:alnum:]]`, never `\t`/`\d`.
- **Root builds auto-use `vendor/`** — internal/ changes need `go mod vendor`
  first; dep bumps also per-module tidy (`check-gomod-vendor-sync.sh` gates).
- **GOEXPERIMENT/GOTOOLCHAIN**: ci-local exports jsonv2; CI setup-go + go.mod
  floors + flake tarball PINNED IN LOCKSTEP (1.27.3); NEVER lower a `go`
  directive (`check-go-mods.sh`).
- **`fetch` empty ≠ URL dead**: an empty body is a tool mismatch — retry with `download`.
- **Host GOCACHE/GOROOT hazards**: gates run `GOCACHE=/tmp/go-build-cache`
  (host symlink → /nix/store ENOSPCs); "package X is not in std" +
  `$GOMODCACHE/toolchain@…` GOROOT = corrupt extraction (chmod -R u+w + trash).
- **golangci-lint is advisory** (~1.4k baseline, growth gated by
  `scripts/lint-baseline.sh --check`); hard gates: vet+gofmt+tests; regen only
  green after `golangci-lint cache clean`; gosec triaged via `check-gosec.sh` —
  a NEW finding is a new class needing fresh triage.
- **`tq serve`/`tq api` security**: loopback + read-only default; writes
  CSRF+lockout-guarded; non-loopback needs `--auth-token`; no new write
  endpoint without it (SECURITY.md).
- **templ + cmd/tq LSP diagnostics are false positives — never "fix" them;
  cmd/tq `GOWORK=off` build/test fails at HEAD** (undefined internals vs
  the facade floor) — trust the CLI gates: go.work workspace or
  `scripts/test-cmd-tq.sh` (07-28 §d5).
- **erraudit**: HARD gate `scripts/check-erraudit.sh` — ruled flags live
  there (`--max` dev-build-only; CI pin v0.7.0 rejects it), run from EACH
  module root (`lint` REJECTS `--modules`/`--violations-only`). `legacy_as`
  → `errors.AsType` (done); `legacy_is` sites KEEP `errors.Is` +
  `//nolint:legacyerrors // <reason>` on the call line (gate passes
  `--no-suppress`; never run `nolint-audit --fix` — deletes honored tokens).
  `generic_return` (~197 sites / 24 modules): OWNER-RULED ADOPT, a migration
  project — gate keeps `--disable generic_return` until it lands. Wiring:
  gate + ci-local HARD; CI = `--self-test` only (repo PRIVATE).
  Bugs: github.com/larsartmann/erraudit.
- **VendorHash drift**: after a go.mod change run
  `nix build .#checks.x86_64-linux.vendor-hash` and copy `got:` into flake.nix.
- **readmodel schema drift crash-loops the fold** (planned tables get no
  column migration; `ConvergeProjectionHome` snapshots divergent homes
  aside): remedy = quarantine `<db>.readmodel.db*` + restart.
- **History policy**: NEVER reword/amend/rebase PUSHED commits; no force-push.
- **Kernel ETXTBSY**: fresh-binary execve intermittently fails — exec sites
  through `execWithTransientRetry`.
- **formatterWithGo drop-day** (flake.nix): `goTarball*` + `formatterWithGo` drop
  at nixpkgs go ≥ 1.27.3; hashes paired — 2026-11-01 note.
- **templ-components sibling pins**: a pseudo-version in a dep's root go.mod
  passes local builds but kills consumer graph loads (Nix FOD) — `grep -r 00010101`
  the dep before blaming the flake/vendorHash.
- **gofmt gates are SCOPED to non-gitignored files**
  (`executor.ScopedGofmtStage`, doctor `gofmt:<repo>`; recipe in the
  tq-bash-guard deny): unscoped re-plants the vendor-gofmt death class
  (94% of DLQ) — grep the FULL verify log before attributing.
- **Flakes see only git-tracked files** — `git add` before `nix build`.

## Relation to other projects

- **go-cqrs-lite IS the platform (ADR-0019)**: adopted in
  `{sqlitev4,postgresv4}` + `companion`; `internal/journal/cqrs` PROPRIETARY,
  read-only — never extend or import below root. Facade Open
  backward-auto-upgrades legacy pre-flip DBs in place (snapshot
  `<db>.legacy-*.bak`, verify, restore on mismatch; absent tables
  tolerated); refuse via `TQ_NO_AUTO_UPGRADE=1`; manual replay:
  `go run ./replay` (internal/queue/sqlitev4).
- **PapDashboard bridge**: `--alert-url/--alert-api-key`; dead letters raise
  `alert.triggered`, completions resolve; `NotifyDeadPool` = direct dead-pool
  path; pool detectors add worker-death, frozen-sweepers, budget-all-day.
- **go-health-dashboard** (MIT): ADOPTED — `/health` inside the token
  gate, tq CSS, same-origin Datastar. **httputil** NOT adopted
  (proprietary); **go-nix-helpers** flake-input only, never local.
