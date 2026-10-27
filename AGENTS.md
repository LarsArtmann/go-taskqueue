# go-taskqueue — Agent Guide

Projects-aware task queue / worker pool for Go — see README.md.

**STATUS: v0.3.4 shipped (tags pushed, proxy verified); MULTIPLE
concurrent agents** — re-read files, re-run tests; uncommitted parallel
changes: read, judge, build on, never revert. (Size guard: TestAgentsDocSizeGuard)

**MAJOR OPEN PROBLEMS**: [dispatch prompting + batch granularity](docs/problems/major-prompting-and-batching.md) —
redesign SHIPPED v0.3.4; follow-ups tracked in the doc.

**READER ROUTE — read only your lanes** (dense by design):
dispatched executors → §Payload contracts + §Conventions (claims,
battery, gate receipts); review/dlqfix/status agents → §Conventions +
§Operational contracts; gate/verify runners → §Commands + §Known Issues;
store/queue code → §Store invariants; platform/docs work → §Relation to
other projects. Everyone: scan §Known Issues before attributing
failures. Package-level reader maps live in each package's doc comment.

## Commands

```bash
./scripts/ci-local.sh # pre-push gate; transient foreign breaks retry 45s ×3
  # TRANSIENT_MAX_POLLS=0: NO retry (check-transient-retry pins);
  # nix/check-ci/daemon-attribution unwrapped.
./scripts/root-gate.sh # root verify gate; ONE retry on known-flaky signatures
./scripts/lint-baseline.sh --check --clean-cache # opt-in stale-cache clean before a warm --check (regen ALWAYS cleans)
nix build # nix run .#test; .#webui-css
./scripts/fuzz/nightly.sh
```

- **Multi-module repo (ADR-0011)**: `internal/{task,journal,queue,
  executor,worker}` + `queue/{sqlite,postgres}` + `internal/journal/
  cqrs` sub-modules; `internal/{readmodel,composition}` own modules (S4
  root; cmd/tq pins the release floor); root = app layer. `cmd/tq`
  replace-free (ADR-0017), built via `scripts/build-tq.sh` (devmod
  shim). `./...` skips nested modules; per-module gate:

```bash
for m in $(find internal task journal queue executor worker -name go.mod -printf '%h\n'|sort); do (cd "$m" && GOEXPERIMENT=jsonv2 GOWORK=off go build ./... && GOWORK=off go vet ./... && GOWORK=off go test ./... -count=1)||exit 1; done
./scripts/test-cmd-tq.sh   # cmd/tq module gate (CMD_TQ_OS=windows cross-compile)
```

**Public facades (ADR-0016):** `task/ journal/ queue/ queue/sqlite/
queue/postgres/ executor/ worker/` re-export internals via type aliases
— the only external import surface; in-repo code imports `internal/…`.
Facades: require (real tag) + relative replace; tests import internals,
never sibling facades; parity via `scripts/check-facade-parity.sh`
BEFORE staging; postgres `OpenWithPool` pools CALLER-OWNED. No go.work
at root — replace-only (exception: tracked `cmd/tq/go.work` dev
workspace, ADR-0017). When a module's `go` directive bumps, bump
`cmd/tq/go.work`'s `go` line (`go work edit -go=<ver>`) in the same
change — a stale line fails every cmd/tq go command. Release flow:
docs/release/.

Smokes (CI-safe): `scripts/smoke/`. Guards: check-*.sh +
release-gates + lint-baseline. `scripts/new-module.sh <dir> [deps…]`
scaffolds go.mods.

## Architecture

Facts-first: every state change is an immutable fact in an append-only
journal; queue views, retry state, DLQ are projections. Claim
exclusivity = lease TTL + expiry reclaim. Vocabulary:
docs/DOMAIN_LANGUAGE.md.

| Package | Purpose |
| --- | --- |
| `internal/task` | Task record, Status, sentinels |
| `internal/journal` | Fact types + append-only Journal |
| `internal/journal/cqrs` | Read-only go-cqrs-lite adapter (ADR-0014) |
| `internal/queue` | Store contract, Filter, Queue facade, Waker |
| `internal/queue/{sqlite,postgres}` | Thin drivers over the v4 adapters; conform: `internal/queue/companion/conform` |
| `internal/queue/{sqlitev4,postgresv4}` | tq Store over the go-cqrs-lite queue engines |
| `internal/queue/companion` | Shared tq surfaces: reads, watermarks, scores, exclusivity, token finalize (`WithToken`) |
| `internal/{readmodel,composition}` | S3 projection `<db>.readmodel.db` (`--read-model` ON) + S4 root: `system.New`, `ProjectionRuntime` (model+DLQ+host, `tq serve`) |
| `internal/worker` | Claim → heartbeat → execute loop; requeue ladder |
| `internal/bridge` | papdashboard + cqa bridges → fix tasks |
| `internal/executor` | sh/HTTP/agent/review/status executors + registry |
| `internal/harvest` | TODO_LIST.md → tasks; drift audit; prune-stale |
| `internal/budget` | Daily-cap + session-usage projections per tick |
| `internal/{dlqfix,review,status}` | DLQ autopsies (`--dlq-fix`, auto-dismiss); review sweeper (`--review-autofix`); done-prompt sweeper (`--status-every`) |
| `internal/prioritize` | AI batch scorer (`--prioritize`), priority_scores cache |
| `internal/depsweep` | Dependency-upgrade sweeper `--dep-sweep` |
| `internal/watermark` | Durable journal cursor shared by the sweepers |
| `internal/consumer` | Journal dispatcher, per-subscriber cursors (ADR-0009) |
| `internal/runactor` | run.Group actors, LIFO shutdown, InterruptOn |
| `internal/webui` | Live dashboard (`tq serve`): tailer→hub→SSE (ADR-0003) |
| `internal/httpapi`+`httpauth`/`lockout` | Machine API (`tq api`): token-mandatory, nosniff, lockout, shared bearer + 3-strikes |
| `internal/incident` | Error reports → fix tasks (ADR-0021) |

### Store invariants

- **Single serialized writer**: `sqlitev4.Open` = ONE shared `*sql.DB`
  (MaxOpenConns(1), WAL, busy_timeout) ridden by engine (`usqlite.OpenDB`)
  AND companion — never a second write pool; never drop the
  `RowsAffected()` re-checks. The projection home elects separately:
  the `.fold-lock` flock every folding readmodel Open must win — a
  loser mounts read-only (no fold, no checkpoint); the stale zero-byte
  sidecar is steady state (never unlinked).
- **One deployment description (ADR-0022)**: `config.Deployment` is the
  only store-opening input — flags resolve via `config.FromFlags` (ONE
  TQ_SQLITE_SYNC reader), openers take the struct, pragma literals live
  ONLY in config; ONE engine per sqlite home — no second write pool or
  pragma copy.
- **SQLite IO policy (2026-10-07)**: steady-state handles run at the
  configured sync tier (NORMAL = checkpoint fsync, lost-tail-tolerant);
  migrations stay FULL. Claim probes use partial RUNNING-only indexes
  (companion.Migrate), never project history. Idle worker loops back
  off (cap `IdlePollMax` 2s, reset on claim/wake).
- **Task contexts survive pool shutdown** (bounded only by
  `--task-timeout`) — no shared drain deadline.
- **Facts in the same tx as state**, or it didn't happen.
- **Unix-milli timestamps**: stored timestamps (upstream engine) are
  MILLISECOND-granular — ordering-sensitive code must not assume
  sub-millisecond completion order; sorts need deterministic non-time
  tiebreaks or set-wise assertions (status-window flake, 2026-10-17).

### Payload contracts (code owns detail)

- **`sh`**: payload is the shell line (raw / JSON string / `{"cmd":…}`).
- **`agent` worktrees** (`--worktrees`, OFF): branch `tq/<task-id>`;
  review-approve auto-merges + reaps remnants.
- **`agent`**: `AgentPayload` (repo, prompt, verify, timeout). Verify: exit
  0, ENV-SELF-CONTAINED (verifies carry `GOEXPERIMENT=jsonv2` — agents
  never edit their own gate). Model+effort ONLY in the repo `.crushrc`
  managed block. `--task-closeout` resumes the EXACT session for its
  a)-g) report (`docs/status/tasks/<ts>_task-<id>.md`).
- **Derived outcomes**: the queue derives what a run did — commits via
  ONE `Task-Queue-ID` footer as LAST trailer (attribution above it is
  invisible; hook rejects), files via `git diff-tree`, usage via
  go-crush-data. No stdout self-report.
- **Verdict channel**: paid turns record via `tq verdict '<json>'` into
  `$TQ_RESULT_FILE` (file > legacy stdout, last-wins).
- **Batched harvest** (`--batch-items`, default 3): N adjacent
  same-section items → one task (dedup `batch:` + SORTED-key hash;
  gates see ONE task); direct `tq enqueue` forbidden — work prompts
  grant the move. Prompts are OUTCOME contracts: rails only, AGENTS.md
  owns the rules.
- **`review`/`dlqfix`/`status`/`prioritize`/`depbump`**: verdict-gated
  or deterministic turns on the closeout-free agent clone; usage derived
  for ALL paid turns; findings commit-anchored (verbatim `anchor` or
  REJECTED); dead agent tasks mint ONE autopsy (`dlqfix:<id>`);
  gate-artifact death WITH shipped-proof auto-dismisses
  (`executor.IsGateArtifactDeath`).
- **Session-close bridge**: `tq session begin/close` + `tq crush`;
  close scans `Crush-Session:` trailers → ONE review + ONE status task.
- **PapDashboard questions**: `tq ask --task <id>` parks without burning
  an attempt; the AnswerPoller routes answers home.
- **Idempotent enqueue**: `DedupKey` re-enqueue returns the stored
  task; COMPLETED keys refused (`ErrTaskDone`); cancelled/dead keys
  still suppress (escape: edit the item).
- **Rate limits (429)**: `executor.DetectRateLimit`; requeue WITHOUT
  burn (jittered wait, fallback 15min cap 6h); per-repo gates
  fast-refuse siblings; closeout 429 arms `closeoutPending` resume;
  the provider RIDES the requeue evidence (`RequeueEvidence.provider`,
  rendered on `tq tasks --parked`).
  **Env-requeue breaker**: env requeues carry `requeue_class`; 3
  consecutive burn an attempt + escalate (`env-streak`) — never uncap.
- **Secrets redaction** (default ON): every output tail passes
  `internal/executor/redact.go`; `tq audit --journal` reports SECRET
  EVIDENCE rows. Growth: ONE `secretPatterns` entry + fake-shape sample
  in `redact_test.go` (gosec note if tripped); ONE shared table
  (`TestSecretHitsAndRedactionCompileIdenticalTable`), windows-gated
  (00-55 §f9). Counts = distinct secret LOCATIONS (span-merge `4ba81d8`;
  before it, raw pattern matches).
- **Enqueued-fact snapshots are THIN** (`{project,type}`;
  `Caps.EnqueuedSnapshot=false` pinned in conform suites).

### Operational contracts

- **SQLite migrations** in `migrate()`: schema const → pragma + ALTER
  for legacy DBs → indexes AFTER the column exists.
- **Watermarks**: checkpoint AFTER the batch's last accepted fact;
  failed checkpoint gates forwarding; `tq watermarks show/set`; replay
  idempotent (seq-derived).
- **Priority (ADR-0015)**: claim order = STORED priority + aging (3d/pt,
  cap 10); stored value never mutates. Markers `— P[1-4]` stripped
  before the dedup hash. ONE ladder everywhere (marker > AI > keyword).
  Probes enter `--priority ≥90`.
- **Failure-evidence surface**: `tq dlq --evidence-stage <class>` filters
  the DLQ by the failed fact's Stage/VerifyStage/Class plus the
  gate-env/gate-dead/gate-slow aliases; `tq show` renders a `failure`
  JSON section; webui detail trail lines carry an `evidence=` suffix.
  Matcher: `cmd/tq/evidence.go` — the alias list is a THIRD copy of the
  death-class knowledge; a fourth must not appear.
- **prune-stale**: cancels PENDING tasks whose item is `[x]` or gone;
  pools sweep once at start.
- **Claim-time budget gate**: paid turns block once the daily
  cap/budget-cmd is spent — requeue class `budget`, no burn, parked to
  midnight (cap) / 15m (cmd), outside env-streak. Claim checks run
  CHEAP-FIRST (budget → preflight → executor — gated pools never spawn
  a process the hook can refuse).

## Conventions

- Table-driven tests, plain `testing`; sentinels in `internal/task/errors.go`.
- **Claims carry citations** (gate run or SHA; never a running gate);
  filter-scope claims cite file:line or a pin.
- **Verify-window battery**: cheap HEAD gates (check-doc-refs, root
  build+vet, check-dead-sha-refs, date-named report) + one fresh delta;
  expensive gates inherit a same-HEAD report; nested-module claims
  need in-module `GOWORK=off` tests; root-parsed-file closeouts cite
  ROOT build+vet+test -race rc. Re-dispatch: `scripts/redispatch-brief.sh`;
  battery rc TO A FILE (no PIPESTATUS); dated DONE re-verified note;
  `-v` + PASS COUNT for conform `-run`.
- **Gate receipts**: a report's "N ok / M fail" count is
  `grep -c '^ok:'` over a CAPTURED gate-output file — never counted in
  context or taken from a summary line (17-vs-16 class).
- **Fleet convention — countable gate summaries are PINNED, not advisory**
  (2026-10-24, TODO 01-12 §e7/§g2): every countable sibling gate
  (check-script-syntax, check-go-mods, check-dead-exports,
  check-guard-wiring; gosec already had it) carries `--self-test` pinning
  its summary format + a negative-path counter battery via an env hook;
  wired into ci-local. New countable gates: same contract.
- docs/status reports follow the a)-g) skeleton; indexed on creation
  (`check-status-index.sh`; daemon bypasses hooks — AMEND MANEUVER for
  daemon-folded reports); ONE index-row write point (top chronological
  cluster); CHANGELOG append-only; close exit: record `tq verdict`
  before session end (report without verdict = half-closeout).
- **Closeout TICK-AUDIT**: before the FINAL commit of any batch/fix
  dispatch, grep TODO_LIST.md for each verbatim batch anchor; every
  finished row must read `- [x]` (landed + unchecked = paid round-trip
  — 000001a1699ba37fc704570b24d800000000).
- **Tick-claims cite the tick commit**: "rows N-M ticked/`[x]`" claims
  in index rows + closeout reports cite the tick commit when the ticks
  are not in the report's own commit (bare = authorship-tree);
  WARN gate `scripts/check-tick-claims.sh`.
- **O14 CHANGELOG policy**: `[Unreleased]` carries OPERATOR-VISIBLE
  changes only — repo-internal tooling and non-operator rows go in the
  `### Internal` subsection at the bottom of the release section, never
  Added/Changed/Fixed; external/API behavior changes get an explicit
  `(BEHAVIOR CHANGE for …)` flag; guarded by the O14 check in
  `scripts/check-release-docs.sh`.
- **Clock-first filenames**: run `date +%Y-%m-%d_%H-%M` BEFORE
  composing a `<ts>_<slug>` filename — a guessed `<ts>` lies.
- **Edit→commit→battery ordering**: stage+commit BEFORE running anything
  (daemon sweeps <60 s, footer-less, take STAGED files too); fold only
  while local-only + contiguous + exactly-mine (`scripts/commit-task.sh
  <id> <subject> <file>…`). During MULTI-STEP git surgery modified-
  unstaged AND untracked files are daemon-food — a mid-sequence sweep
  makes the span non-contiguous and the heal refuses; between steps the
  tree must be CLEAN, not merely "nothing left to stage" (2026-10-13
  01-54 §e).
- Tab-bearing insertions go through python-heredoc replace — free-text
  edit glues tabs into comments as literal `\t`.
- Pure-Go deps only (`CGO_ENABLED=0`); Go 1.26+ idioms deliberate
  (`errors.AsType[E]`, `SplitSeq`, `for range n`) — don't undo.
- Retry loops use `github.com/larsartmann/go-retry` (supervisor loops and
  persisted domain backoff exempt).
- **Executor shared seams — never hand-roll**: `decodePayload[T]`,
  `recordRunOutcome`, `executor.Excerpt`, `prepareRepo`, `payloadTimeout`;
  mirror clones gated STRICT by `check-mirror-clones.sh` (shared surface
  `internal/queue/companion`); art-dupl residuals accepted ONLY where
  named here (the `fragments.templ` conditional-span pair — Go comments
  cannot live inside templ markup); don't add new ones.
  `art-dupl:accept` suppresses only within a few lines ABOVE the clone's
  first line (func-top doc position; inline comment for deeper ones).
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
- Docs formatting MANUAL (dprint on-demand).
- Evidence archives: `scripts/archive-evidence.sh` (gated by
  `check-ghost-archives.sh`); `git check-ignore -v` BEFORE copying
  evidence in.

## Known Issues

- **Concurrent agents commit constantly** (auto-commit daemon): re-run
  `go test ./... -race` before declaring success; never generate Go
  source via heredocs; fixtures under /tmp (gated trees are daemon-food).
  Footer-less daemon `chore:` sweeps heal via
  `scripts/heal-daemon-sweep.sh [--from <ref>] [--retitle <subject>] <Task-Queue-ID>`
  (unpushed only; backup on failure; `--self-test` pins rails). REFUSES
  non-contiguous spans. A refusal naming the span's PARENT (a foreign
  footer BELOW an otherwise-exactly-mine span) is the span-terminator
  shape: run the script's printed `--from <oldest-blocker>` invocation —
  proven to heal cleanly (2026-10-20 02-27 window); residue fallback:
  verify exactly-mine via `git show --stat`, then `git commit --amend`
  the unpushed HEAD (2026-10-11 §d4; 02-03 §f11).
- **`TestExactlyOnceUnderConcurrency` is load-flaky (worker);
  `TestSelfManagingLoop` is load-flaky (harvest)**: build storms drop them
  at clean parents — re-run before attributing; both ride `KNOWN_FLAKY` in
  `scripts/lib/verify-retry.sh`.
- **Agent shells inherit `TQ_DB`** (PRODUCTION journal) — scratch
  smokes MUST export `TQ_DB=<scratch>`.
- **Session shell hazards**: no usable `PIPESTATUS`; bare `unset VAR`
  leaks to children (`env -u VAR`); multi-file `tail` fails; `printf
  %.0s` yields empty; chained `&` backgrounds the wrong span. POSIX ERE:
  `\t` inside a grep bracket class is a literal `t` (not tab) — use
  `[[:space:]]`/`[[:alnum:]]` classes, never `\t`/`\d` (2026-10-27: a
  `[^ \t]` class silently skipped the cqrs `event` module and the
  fixture "passed" wrong).
- **Root builds auto-use `vendor/`** — internal/ changes need `go mod
  vendor` first; dep bumps also per-module tidy
  (`check-gomod-vendor-sync.sh` gates both).
- **GOEXPERIMENT/GOTOOLCHAIN**: ci-local exports jsonv2; CI setup-go +
  go.mod floors + flake tarball PINNED IN LOCKSTEP (1.27.3, toolchain
  sweep 2026-10-24); NEVER lower a `go` directive
  (`check-go-mods.sh`).
- **Host GOCACHE/GOROOT hazards**: gates run `GOCACHE=/tmp/go-build-cache`
  (host symlink → /nix/store ENOSPCs); "package X is not in std" +
  `$GOMODCACHE/toolchain@…` GOROOT = corrupt extraction (chmod -R u+w +
  trash, or `GOTOOLCHAIN=local`).
- **golangci-lint is advisory** (~1.4k baseline, growth gated by
  `scripts/lint-baseline.sh --check`); hard gates: vet+gofmt+tests;
  regen only green after `golangci-lint cache clean`.
- **gosec**: all findings triaged via `scripts/check-gosec.sh`; a new
  finding is a new class needing fresh triage.
- **`tq serve`/`tq api` security**: loopback + read-only default;
  writes CSRF+lockout-guarded; non-loopback needs `--auth-token`
  (SECURITY.md); no new write endpoints without it.
- **templ + cmd/tq LSP diagnostics are false positives** — trust the CLI
  gates, never "fix" them.
- **erraudit** gates as `erraudit lint ./... --max --disable generic_return
  --disable legacy_is` run from EACH module root (`lint` REJECTS
  `--modules`/`--violations-only`; the root hint naming `--modules=all` is
  wrong for `lint`). `legacy_as` = migrate to `errors.AsType` (zero since
  2026-10-27); `legacy_is` sentinel sites KEEP `errors.Is` +
  `//nolint:legacyerrors // <reason>` on the call line (default lint honors
  it; `--max` implies `--no-suppress`, hence the disable; never run
  `nolint-audit --fix` — it deletes honored tokens). `generic_return`
  (`--enforce-generic-return`, ~200 fleet-wide under `--max`) is a
  DELIBERATE NON-FIX: returning `error` is the idiom (sentinels + `%w`
  wrap; the check ships off-by-default for exactly that reason) —
  per-function error types are a go-error-family migration decision, not
  lint debt.
- **VendorHash drift**: after go.mod changes run
  `nix build .#checks.x86_64-linux.vendor-hash`, copy `got:` to
  flake.nix.
- **readmodel schema drift crash-loops the fold** (planned tables get
  no column migration): `ConvergeProjectionHome` snapshots divergent
  homes aside (`.legacy-schema-*.bak`); crash remedy: quarantine
  `<db>.readmodel.db*` + restart.
- **History policy**: NEVER reword/amend/rebase PUSHED commits; no
  force push; scripted edits via script files.
- **Kernel ETXTBSY**: fresh-binary execve intermittently fails — route
  exec sites through `execWithTransientRetry`.
- **Nix 2.34 flake gotcha**: `outputs = let … in <lambda>` reads as a
  thunk ("expected a function but got a thunk"); keep the `let` INSIDE
  the lambda.
- **formatterWithGo drop-day** (flake.nix): `goTarball*` builds the
  toolchain from the go.dev tarball (floor > nixpkgs go; sandbox blocks
  the download) and `formatterWithGo` wraps templ fmt/goimports with it;
  ALL FOUR blocks drop together when nixpkgs ships go ≥ 1.27.3 — hashes
  stay paired (`goTarballHash`).
- **templ-components sibling pins**: zero pseudo-versions
  (`v…00001010100000-…`) in a dep's root go.mod pass local builds but
  kill consumer graph loads (Nix FOD) — `grep -r 00010101` the dep
  before blaming the flake/vendorHash.
- **gofmt gates are SCOPED to non-gitignored files**
  (`executor.ScopedGofmtStage`, doctor `gofmt:<repo>`):
  `gofmt -l . | git check-ignore --stdin -v --non-matching | grep '^::'`;
  unscoped re-plants the vendor-gofmt death class (94% of the DLQ) —
  grep the FULL verify log before attributing.
- **Flakes see only git-tracked files** — `git add` before `nix build`.

## Relation to other projects

- **go-cqrs-lite IS the platform (ADR-0019)**: adopted in
  `{sqlitev4,postgresv4}` + `companion`; `internal/journal/cqrs`
  PROPRIETARY, read-only — never extend or import below root.
  Facade Open backward-auto-upgrades legacy pre-flip DBs in place
  (snapshot `<db>.legacy-*.bak`, verify, restore on mismatch; absent
  feature-era tables tolerated); refuse via `TQ_NO_AUTO_UPGRADE=1`;
  manual: `go run ./replay` in internal/queue/sqlitev4.
- **PapDashboard bridge**: `--alert-url/--alert-api-key`; dead letters
  raise `alert.triggered`, completions resolve; `NotifyDeadPool` =
  direct dead-pool path; pool detectors add worker-death,
  frozen-sweepers, budget-all-day.
- **go-health-dashboard** (MIT): ADOPTED — `/health` inside the token
  gate, tq CSS, same-origin Datastar. **httputil** NOT adopted
  (proprietary); **go-nix-helpers** flake-input only, never local.
