# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]
### Added
- **Priority system (ADR-0015)**: one 0-100 priority scale with three
  bands — backlog 0-99 (markers, importance, AI scores, keywords), hot
  100-149 (same-session promotion), machine 150+ (operational tasks;
  cqa migrated 80 -> 150). Claim order = stored priority + aging
  (+1 per 3 days waiting, capped +10; constants live once in the queue
  contract, mirrored in both backends and pinned by conformance tests
  run against a live postgres). TODO_LIST items accept trailing
  `— P[1-4]` markers (90/70/50/30; stripped before the dedup hash, so
  marker edits never fork tasks) and `tq harvest`/`tq agent-pool` gain
  `--priority-from importance` (reads each repo's
  `.config/metadata.yaml` importance, default 50; malformed files skip
  the repo) plus keyword bumps (security/critical/urgent...). New
  `task.reprioritized` fact + `Store.UpdatePendingPriority` (PENDING
  only, same-tx evidence, value-idempotent) power `tq reprioritize`
  (dry-run/JSON; also applies unblock bumps: pending tasks whose deps
  ALL completed gain +15 once) and the pool's default-on startup
  repri sweep. `--max-pending-per-repo` turns the queue into a working
  set (cap replaces the legacy one-task-per-repo rule). An explicit
  `importance: 0` pauses a repo from auto-admission. AI batch scorer:
  `priority_scores` cache table (both backends) + `internal/prioritize`
  sweeper (`--prioritize`, default OFF): repos holding unscored backlog
  items mint ONE machine-band batch task (dedup
  `prioritize:<repo>:<hash-of-key-set>`) whose READ-ONLY scorer verdicts
  cache scores and re-rank pending tasks (marker > AI > keyword
  precedence; hot/machine protected; scores clamp to backlog).
  `tq show` gains a priority provenance section (band, cached verdict,
  repri history). Web UI: prio column with band badges, `?band=`
  filter (stored-priority range pushdown in both backends), and the
  aging hint under the active table. Starvation alarm
  `--starvation-after` (oldest pending task past the threshold despite
  aging -> PapDashboard trigger/resolve, dead-pool pattern).
- **go-cqrs-lite journal adapter (`internal/journal/cqrs`, `tq facts
  --cqrs`)**: the fact journal is now consumable as a native
  go-cqrs-lite `event.Journal`/`event.SeekableJournal` (event/v4
  v4.11.0), so projections, `watermill.CatchUpSubscriber`, and SSE
  replay tooling from the go-cqrs-lite ecosystem can read queue facts
  directly. Read-only by contract (writes stay in the queue stores'
  transactions); synthetic sequence-encoded ULIDs mean zero schema
  migration, lexicographic ID order equals Seq order, the zero event ID
  is the journal start, and foreign cursors drain empty (the
  dangling-cursor contract — never a replay). Facts map verbatim
  (`task.*`/`session.*` → event types, task/session stream types, fact
  body JSON as payload, global Seq as Version). Pinned by adapter unit
  tests plus a store-backed integration test through real `sqlite.Store`
  and the CLI render path. PROPRIETARY license adopted by owner
  instruction 2026-09-12 — decision + deferred tiers (per-task
  `EventSource`, watermill bus, branded IDs, idempotency stores) in
  ADR-0014. Lint baseline regenerated deliberately (892 findings, 107
  rows; new-module err113 + repo-wide paralleltest drift).
- **DLQ autopsies — the self-fixing dead-letter queue (`--dlq-fix`)**: when
  an AGENT task dead-letters, the new `internal/dlqfix` sweeper mints ONE
  autopsy task (dedup `dlqfix:<dead-id>`, agent-type deaths only — so a
  dead autopsy can never spawn another) whose second agent diagnoses the
  failure from the journal's own evidence (`FailureEvidence` stage/exit/tail
  + the dead task's original prompt, riding in a self-contained
  `DLQFixPayload`). Strict `TQ_RESULT` verdict contract: `fixed` rescues the
  original with its original attempt budget; `wontfix` (reason REQUIRED)
  dismisses it — a new `Dead → Cancelled` transition with the reason +
  `dismissed_by` recorded on the `task.cancelled` fact (mirrored
  `DismissDead` in sqlite + postgres, conformance-pinned). Operators get the
  same lever as `tq dlq --dismiss ID --reason WHY`. Dispositions are
  idempotent (transition-guarded, race-with-human benign), every mint runs
  under the pool's budget guard, autopsies run the closeout-free agent
  clone and are dirty-tree-capable by default (a dead agent's partial work
  IS evidence), and one watermark cursor (`dlqfix-sweeper`,
  head-bootstrapped, rewindable) drives mint + dispose. Design:
  `docs/planning/2026-09-12_dlq-autopsy-design.md`.
- **Session-close bridge prototype (`tq session begin/close`)**: interactive
  crush sessions can now get the same close-out pool agents get. Begin
  records a `session.opened` journal fact; close attributes the session's
  `Crush-Session: <id>` footer commits (one trailer-scoped `git log` scan),
  directly enqueues ONE review over the commit range and ONE done-prompt
  status task (dedup keys shared with the sweeper namespaces, so replays
  never duplicate work), and appends the `session.closed` fact carrying the
  lineage. Close is enqueue-only — fast enough for a future SessionEnd hook
  (crush PR #3146); the pool does everything else. New `internal/session`
  package, `(*sqlite.Store).AppendFact` as the one sanctioned non-task fact
  write, and the design/tradeoffs doc
  `docs/planning/2026-09-12_session-close-bridge-design.md`. Trigger
  automation, daemon-commit attribution, budget routing and Postgres
  `AppendFact` parity remain open (documented in the design doc).
- **`tq doctor` now diagnoses the Go build environment (2026-09-12)**: a
  new check builds a synthetic `encoding/json/v2` module twice — once with
  the ambient environment and once with `GOEXPERIMENT=jsonv2` — and
  classifies the result. The failure mode it names is the real one that
  burned three pool attempts (2026-09-11, task 000001a08ebf): a
  jsonv2-only repo verified in an environment without the experiment set
  reports "build constraints exclude all Go files" and the fix is the
  environment (`GOEXPERIMENT=jsonv2`), not the repo. Green when either
  build passes; a verdict line states which side failed and why.
- **Project filter UX on the dashboard (2026-09-12)**: an active project
  filter now renders a chips row (all-projects reset, running/pending/dead
  counts, visible-projects cap at 12 with a +N overflow chip), and
  per-project views drop the now-redundant project cells from task rows
  and board cards instead of repeating the filtered value 50 times.
- **Advisory-lint growth gate and version-agreement gate in ci-local
  (2026-09-12)**: `.golangci-baseline.txt` pins the per-module,
  per-linter finding counts of the documented advisory sea (~400
  findings, AGENTS.md); `scripts/lint-baseline.sh --check` fails the gate
  on growth or a new finding class while shrink stays advisory, and
  regeneration is the sanctioned deliberate path for policy-owned
  changes. `scripts/check-version-agreement.sh` pins the flake version
  string, the root `.version` file and the CHANGELOG's latest release
  heading to the same value.

### Changed
- **The status enum has one canonical list (`task.AllStatuses()`)**: the
  webui `allStatuses` and httpapi `apiStatuses` twin slices are retired;
  the board columns, both `/stats` handlers (dashboard + `/api/v1`) and
  the filter dropdown now range the exported list, so a new Status can no
  longer render as a missing board column or a missing stats key. A pin
  test in `internal/task` hardcodes the expected set as an oracle. No wire
  change; the exported list rides the next `internal/task` sub-module
  re-tag for proxy consumers.
- **CI security scanning is triage-encoded, not ignored (2026-09-12)**:
  the gosec job passes its 14 triaged all-FP classes as `-exclude` rules
  (matching `.golangci.yml`), so a NEW gosec class fails loudly instead of
  drowning in the baseline, and findings land in the job summary with the
  re-triage rule spelled out. govulncheck loses `continue-on-error` on the
  modules it gates and reports a per-module table in the summary.

### Fixed
- **Review prompts no longer stamp the review's own task id into the quoted
  work contract**: the reviewer prompt resolves the quoted `{{TASK_ID}}`
  placeholder to the REVIEWED task's id (what the work agent actually saw)
  and gains an explicit queue-cross-reference criterion for footer-bearing
  contracts. Previously `runAgent`'s blanket substitution resolved the
  quoted placeholder to the REVIEW task's id, so diligent reviewers flagged
  the work run's correct `Task-Queue-ID` commit footer as a foreign id and
  rejected sound changes (2026-09-12 Hermes cron review). Fix-task prompts
  resolve the quoted original the same way and now carry an explicit
  own-footer instruction (`{{TASK_ID}}` resolves to the fix task's id at
  run time).
- **`scripts/release.sh` gates could print GREEN after a failed nix
  build**: a command substitution inside an env-prefix assignment
  (`TQ_BIN="$(nix build …)/bin/tq" step`) does not abort under `set -e` —
  the substitution's failure vanished and the gate read the step's exit
  status, so a dead `nix build` still ended in "GATES GREEN". The
  out-path is now captured in a standalone assignment, which does abort.
  Found by the first live fixture execution of the release flow
  (round-12 T14).
- **`tq version` printed `dev` for nix-built binaries**: the flake built
  with Go's default flags and never passed `-ldflags "-X
  main.version=…"`, so release binaries could not be told apart from
  scratch builds. `flake.nix` now threads the flake version through
  `buildFlagsArray` (2026-09-12).
- **Dashboard filter links URL-escape their query strings and cap query
  length**: `QueryString` escapes metacharacters (`&`, `#`, `%`, spaces)
  so a project named `a&b` can no longer corrupt the link or smuggle
  markup into `href`s, and `parseFilter` clamps the `query` parameter to
  200 chars (2026-09-12).

### Added
- **Fuller agent toolset in the bootstrap managed block**: the headless
  allow list grows from 7 tools to `view ls grep glob edit multiedit write
  bash fetch download todos` — in non-interactive mode unlisted tools are
  denied, so the list IS the agent's toolset; agents can now read web docs
  (fetch, verified empirically headless), pull artifacts (download), batch
  edits (multiedit), and track multi-step work (todos). The block also sets
  `option metrics false`: every headless run previously paid a PostHog
  flush at shutdown (observed "Failed to flush PostHog events" + "shutdown
  timeout exceeded" in dead task tails). Existing repos pick the upgrade up
  on the next `tq bootstrap` run.
- **Worktree-per-agent design doc**
  (`docs/planning/2026-09-12_worktree-per-agent-design.md`): the
  intra-repo parallelism path (claim → dedicated git worktree → verify →
  merge → reap) with the three-layer serial lock named, executor/review/
  rate-limit/daemon contract interactions, a tradeoffs table, and an
  explicit open-questions section (merge policy is the gating owner call).
  Design only — nothing built (02:00 self-review f15).
- **Provider rate-limit regression armor**: the exact Z.ai 429 output of the
  dead-lettered incident task is pinned as testdata; parked-requeue-vs-stale-
  lease contract pinned on sqlite AND postgres conformance; new hermetic
  e2e smoke `scripts/smoke/ratelimit-e2e.sh` (stub 429 agent, real binary,
  scratch DB — asserts the requeue without attempt burn); `FuzzDetectRateLimit`
  campaign with real-provider seeds. The fuzzer and the pins found and fixed
  three real bugs: a `time.Duration` overflow on far-future reset timestamps,
  a timezone-fragile RFC3339 test expectation, and sqlite `Store.Fail`
  missing the lease re-check (a stale owner could append failure facts for a
  task it no longer holds — postgres already had the check).
- **Per-repo rate-limit gates**: a 429 observed in one repo no longer parks
  sibling tasks of other repos in the same pool (the repo's `.crushrc`
  fixes its provider, so repo = provider isolation key).
- **Closeout resume after 429**: a rate-limited CLOSE-OUT turn requeues the
  task without attempt burn and the re-claim resumes at closeout — the paid
  work turn is never re-run because of a provider 429 (in-process registry).
- **HTTP executor 429 classification**: webhook 429s now surface as
  `*executor.RateLimitError` (Retry-After header and body reset honored,
  same no-attempt-burn requeue as agent tasks).
- **Parked-task visibility**: `tq tasks --parked` filter (SQL pushdown on
  both stores), a parked count in `tq stats` (text + JSON), and a `tq
  doctor` parked check — an idle pool with parked tasks diagnoses as
  WAITING, not broken.
- **Task-Queue-ID commit-msg hook**: `scripts/install-pre-commit.sh` also
  installs a commit-msg guard — exactly one well-formed footer per commit,
  catching the malformed/duplicate-footer cross-reference corruption class
  at write time.
- **Commits-per-ID view**: `tq show --commits` scans the task's payload
  repo git log and verdicts MISSING FOOTER / AMBIGUOUS / ok.
- **Master-CI state gate**: `scripts/check-ci.sh` (wired into ci-local.sh)
  fails the pre-push gate when the latest CI run on the branch is red; CI
  workflows now set `GOEXPERIMENT=jsonv2` at workflow level (the missing
  env made CI red while local builds passed via `~/.config/go/env`).
- **CSS drift gate wired into ci-local**: the round-5 `check-webui-css.sh`
  guard (committed `app.css` must byte-equal the tailwind rebuild) existed
  since 2026-09-08 but was wired into NOTHING — unminified or hand-edited
  artifacts shipped to master twice (04aace4, 20d1a69), each breaking the
  minified-CSS a11y test after the fact. It now runs as a ci-local step
  (needs only nix, not a devShell — the rebuild goes through
  `nix run .#webui-css`), failing the pre-push gate while leaving the
  regenerated canonical file in place to commit.
- **Anti-ghost-archive gate for evidence archives**: new
  `scripts/check-ghost-archives.sh` (gated in ci-local + CI) asserts every
  evidence file a `docs/status/assets/*/README.md` promises is actually
  git-tracked. The global `*.log` gitignore plus the auto-commit daemon's
  add-everything sweep can otherwise ship an archive as README-only, losing
  the evidence at the next checkout (the f9 dogfood-archive near-miss, 06-41
  report §d1).
- **Dead-pool detection alert for agent pools**: `tq agent-pool
  --dead-pool-ticks N` (default 3, `0` = off) raises a PapDashboard alert
  when EVERY watched repo scan-fails for N consecutive harvest ticks — the
  pool reads no TODO_LIST at all, so it silently enqueues nothing (the
  2026-09-10 pool-deploy incident stayed invisible for 20h). One
  `alert.triggered` fires per streak via the bridge's new direct
  `NotifyDeadPool` path (harvest skips are process-local observations, not
  journal facts), a WARN is logged even without `--alert-url`, and the
  first tick with a readable repo posts `alert.resolved` and resets the
  streak.

### Changed
- **Task detail page renders payloads as content, not escaped JSON** (2026-09-11): the
  `/task/{id}` payload row — previously one break-all escaped-JSON line squeezed into the
  record definition list — is now a type-aware section between the record and the fact
  timeline. Agent tasks show the work item as the lede (prompt-as-lede when the item is
  empty), the executor-contract spec grid (repo, branch, model, verify gate, timeout), the
  prompt contract in a collapsed fold, and the raw payload as pretty JSON in a second
  fold; review/status/`sh` payloads get the same treatment, parsed with the executor's own
  payload structs plus a new exported `executor.CommandFromPayload` so the displayed shell
  line cannot drift from the executed one; unknown or unparseable payloads fall back to
  the raw view. The section is static (payloads are immutable) and sits outside the
  SSE-swapped fragments, so open folds survive live updates. Repeated requeues stop
  stuttering: Error/Detail.reason duplicates on a fact collapse to the fuller text, facts
  carry `(attempt N)`, and ≥2 requeued/failed/released/dead-lettered events aggregate into
  a `retry trail ×N` strip (distinct reasons × counts, loudest first) above the timeline —
  the final dead-letter reason is included, so the page answers "why did it give up?" too.

### Fixed
- **Dashboard and write-API stats surfaces pinned to one wire contract**: `tq serve`'s
  `/api/stats` and `tq api`'s `/api/v1/stats` computed the same payload through two
  independent paths (dashboard keyed by HTML badge labels; API emitted raw GROUP BY rows
  that dropped zero-count statuses), so the shape could drift silently. Both now derive
  keys from the task status list and always emit all five statuses plus total;
  `TestStatsSurfacesAgree` pins the payloads equal over one seeded store and locks the
  disjoint route namespaces (`/api/v1/*` vs `/api/*`).
- **Root module now builds against the local postgres backend**: the
  `internal/queue/postgres` require had no relative `replace`, so root builds compiled the
  proxy's tagged copy and local edits to the backend never applied. The replace matches the
  other internal modules' local-dev layout.
- **Executor absorbs the kernel-7.2 `ETXTBSY` flake**: `execve` of a freshly
  written binary intermittently returned "text file busy" with no writer
  holding the file (reproduced standalone on kernel 7.2.3 under process
  churn, tmpfs and btrfs alike — temp+rename did NOT fix it). `runAgent` and
  `AgentVersion` now retry ETXTBSY twice (50ms, 100ms); the previously flaky
  stub-agent test set is 40x green.
- **release.sh allowlist could not match nested modules**: after the
  `internal/queue/sqlite` split, the sibling-replace allowlist read the
  legit replace as poison and the require-tag check silently skipped the
  nested module. The gates moved to `scripts/lib/release-gates.sh` with
  fixture-tested positive AND negative paths
  (`scripts/smoke/release-gates.sh`, wired into CI + ci-local).
- `nix flake check --all-systems` failed at eval time on aarch64-darwin
  (`checks.module-eval` used `mkIf`, leaving a dangling option); the check
  now uses `optionalAttrs`.
- **Postgres backend off the vulnerable `golang.org/x/text`**: the module
  still resolved `x/text v0.29.0` (GO-2026-5970, reachable via
  `postgres.Open` → `pgxpool` → `norm.Form`) while every other module sat on
  v0.41.0 — the advisory govulncheck job flagged it on its first CI run.
  Bumped to v0.41.0; `govulncheck ./...` in the module now reports no
  vulnerabilities.
- **The red-master CI trio repaired**: (1) the release-gates smoke cut fixture
  tags without a committer identity, so every CI runner died at `git tag`
  with exit 128 (local hosts masked it with a global git identity) — the tag
  now carries the same `-c user.email/-c user.name` as the fixture commit;
  (2) `TestHarvestConfigFromOptionsExpandsBareRepoNames` asserted POSIX
  string literals and failed windows-latest — inputs and expectations now
  follow the running OS's path rules (separator- and volume-aware
  absolutes); (3) the flake `vendorHash` resynced after the root go.mod
  postgres replace moved the module graph.
- **`examples/fullcore` drain deadline reported deterministically**: the
  drain loop selected on both `ctx.Done()` and the deadline tick, and since
  `Pool.Start` returns only after cancellation both were ready at the
  deadline — Go's random pick let the example exit 0 with no report about
  half the time. The redundant done case is gone; a timeout always produces
  the intended fatal.
- **Two mechanical-rename leaks corrupted user-facing string literals** (both
  fixed within the same morning, before any release): the wrapcheck/varnamelen
  rename sweep changed the webui filter to read `query.Get("query")` while
  every emitter still sends `?q=`, silently deadening the dashboard text
  filter and view-toggle scope with all tests green — restored plus a
  `filterHref`↔`parseFilter` round-trip regression test; the same sweep's
  regex turned `tq dlq --max-attempts` help into "rescued task(store)" —
  restored, and the parent diff swept for further corrupted literals
  (none found).
### Added
- `examples/fullcore`: the full library embed demo in one file — enqueue, custom + shell
  executors (including a retry proof), a worker pool draining the queue, and the
  sqlite-vs-postgres backend picked at run time from two store imports that both satisfy
  `queue.Store`. Run with `go run ./examples/fullcore` (sqlite) or `--backend postgres`
  against a local test database.
- `gosec` CI job (advisory, mirrors the govulncheck job): gosec v2.29.0 over the root
  module plus every sub-module; the 48 findings it surfaced are triaged FP/by-design with
  per-rule rationale in AGENTS.md, and the two real findings (G114 timeout-less HTTP
  servers in `examples/api` and `examples/sse`) are fixed with `ReadHeaderTimeout`.
- Dashboard responses now carry `Permissions-Policy: camera=(), microphone=(),
  geolocation=()` alongside the strict CSP — device capabilities a task-queue
  dashboard never needs are denied outright, closing the last header gap the
  httputil comparison surfaced.
- `govulncheck` CI job (advisory): scans the root module plus every
  sub-module (disk-derived loop) with a pinned govulncheck; job-level
  `continue-on-error` keeps it non-blocking until the findings baseline is
  triaged. Runner-only — the live vuln DB fetch means it can never run in
  the hermetic nix gates or ci-local.sh.
- `cmdAgentPool` decomposed (652 lines → orchestrator + `cmd/tq/agentpool.go`:
  flag/config parsing, harvest-config assembly, startup banner); the
  agent-family executor registration is one shared helper used by
  `tq worker --agents` too. Live-smoked via `agent-pool --once`.
- Postgres conformance battery extended with the six gaps found in the
  suite-parity diff: dedup keys, watermark monotonic roundtrip, head seq,
  lease-expiry reclaim, exactly-once concurrent claims (the SKIP LOCKED
  proof), and LIKE-metacharacter escaping — all verified against a live
  Postgres 16, not just name-diffed.
- `checks.version-sync` flake check: the nix-built `tq version` output must
  equal the flake's own version attr (kills the 0.1.0-binary-from-0.2.0-flake
  drift class at gate level).
- `scripts/check-go-mods.sh`: go.mod hygiene (portable replaces, pinned
  internal requires), toolchain alignment across all 8 modules, and
  `go mod verify` — one script, wired into both ci.yml and ci-local.sh.
- `nix run .#test` now runs the FULL multi-module suite (root + every
  internal/* module); flake apps carry meta descriptions.
- `HTTPExecutor` gained a test suite (status classification, wire envelope,
  empty-payload validity, malformed-URL permanence) — it was documented
  FULLY_FUNCTIONAL with zero tests.
- Compile-time contract assertions in both store drivers
  (`var _ queue.Store = (*Store)(nil)`).
- README: module map + store-backend picker section; package docs on the
  queue contract module; FEATURES row for the backend split; local-database
  one-liner next to the CI Postgres conformance job.
- `docs/release/RELEASE.md`: the round-2 multi-module release flow as
  `scripts/release.sh` implements it — three-step gate sequence, two-phase
  `--tag`/`--push` with resume, disk-derived `internal/<mod>/vX.Y.Z`
  sub-tag cutting, sibling-replace allowlist with the nested-module lesson.
- `docs/release/VERSION-SURFACES.md`: all seven release-version surfaces
  (flake attr, ldflags, root tag, per-module tags, internal requires,
  CHANGELOG, toolchain) with the gate or manual step that verifies each,
  the manual bump order, and the deliberate coverage gaps.
### Changed
- **Store backends become driver modules (ADR-0012)**: `internal/queue`
  keeps only the Store contract (deps: task + journal); the SQLite and
  Postgres implementations move to `internal/queue/sqlite` and
  `internal/queue/postgres` in the database/sql driver style
  (`sqlite.Store`/`sqlite.Open`, `postgres.Store`/`postgres.Open` — the
  stuttering old `SQLiteStore`/`OpenSQLite` names are retired). The two
  backends share seven micro-helpers, mirrored one-for-one per the
  ADR-0007 conformance philosophy. CI gates, hygiene audits and release
  tag-cutting are now disk-derived over all internal modules, so the
  Postgres backend stays gated and tagged even though its CLI wiring is
  still on the ROADMAP; the root module drops pgx entirely.
- **Multi-module split (ADR-0011)**: the library core — `internal/task`,
  `internal/journal`, `internal/queue`, `internal/executor`,
  `internal/worker` — is now five sub-modules (import paths unchanged;
  app layer stays in the root module). The layer DAG is compiler-enforced;
  every CI gate gained a per-module `GOWORK=off` counterpart. Requires point
  at real subdirectory tags (`internal/*/v0.2.0`, cut with the split) so
  `go install` keeps working; relative `replace` directives serve local dev.
- All `errors.As` call sites migrated to the Go 1.26 generic
  `errors.AsType[E]`; sentinel `errors.Is` matches deliberately kept.
- `scripts/release.sh` now allows exactly the sibling-relative sub-module
  replaces, verifies every internal require has its subdirectory tag before
  a release, and cuts/pushes the internal tags with the release.
- The agent pool's default verify command for Go repos walks nested
  `go.mod` files, so multi-module repos verify for real (a `find -execdir`
  variant was rejected: it swallows inner exit codes).
- **`--task-closeout`** (2026-09-10): every agent task gets a SECOND
  conversation turn — the owner's brutal self-review + status-report
  prompt (`executor.DefaultCloseoutPrompt`), answered by the same agent
  session before verify runs. The work turn switches to `--verbose` so the
  session id can be extracted (`ExtractSessionID` also matches the crush
  verbose marker now); the closeout resumes that exact session (`--continue`
  would race across concurrent pool agents) and must end by re-emitting the
  work turn's `TQ_RESULT` line (the gate reads the last one). Per-task
  reports land at `docs/status/<ts>_task-<id>.md`.
- **Status sweeper = docs-health pass** (2026-09-10): the `--status-every`
  done-prompt now mandates the docs-health skill: read every 2026-* status
  report, reconcile TODO_LIST/CHANGELOG/AGENTS/README/ROADMAP/FEATURES with
  what the window actually shipped, archive fully-done reports into
  `docs/status/archived/`, and verify claims against code; the scope rule
  still forbids touching code/config.
- **Advisory lint baseline shrunk: wrapcheck and varnamelen triaged to
  zero.** House policy encoded in `.golangci.yml` (stdlib idioms,
  internal-package seams, test files, idiomatic short names) and the
  genuinely vague call-site variables renamed — the baseline shrank via
  policy plus renames, not suppressions.
- The CI advisory lint step now runs golangci-lint over the root module AND
  every `internal/*` sub-module (disk-derived loop), mirroring ci-local.sh —
  module-local findings stopped being invisible to CI when the split landed.
### Fixed
- Version-surface drift: flake.nix still declared 0.1.0 after the v0.2.0
  release, so nix-built binaries reported the wrong version (now 0.2.0).
- Nightly fuzz campaigns and the Postgres conformance CI job used directory
  package patterns that silently stop matching across module boundaries;
  both now run from inside their module directory.
- `internal/harvest`: the audit and prune sweeps shared an identical
  20-line repo/task-index preamble; extracted into `projectTaskIndex`.
- Sub-module tags `internal/{executor,queue}/v0.2.0` had been re-cut
  locally at the new tree on the false premise they were unpushed; both
  were already published, so the published signed tags were restored
  (tags are immutable once on the remote/proxy — the post-split content
  ships with the next release's module tags, cut automatically by
  `scripts/release.sh`).
- agent-pool harvested nothing under systemd: bare `--repos` names
  resolved against the process working directory (the unit's is the DB
  dir), so every tick skipped every repo as `scan failed`; names now
  expand against `--projects-dir`. The NixOS module's pool unit
  additionally sets an explicit agent-toolchain PATH (new
  `services.tq-agent-pool.agentPath`: hermetic git+go, system and
  per-user profiles, GOBIN) — systemd's default service PATH has no
  git/go/crush, which would have failed every preflight, agent exec and
  verify gate even with the scan fixed.
- The aggregate `harvest: skipped` log cut every reason at the first
  colon, hiding the underlying error (a dead deployment read as a quiet
  one for 20h); groups now carry one full example reason, and scan
  failures log at WARN.

### Removed
- Deprecated pre-convergence aliases `executor.CrushPayload`,
  `executor.RenderCrushPayload`, `executor.TaskTypeCrush` (zero in-repo
  users; ships with the next `internal/executor` version tag — the
  published v0.2.0 tag is immutable and stays at its release tree).

## [v0.2.0] - 2026-09-09
### Added
- **Self-cleaning pool relaunches + absent-item prune policy** (2026-09-09):
  `--prune-stale` now also cancels pending harvested tasks whose item text
  is gone from TODO_LIST.md entirely (done-and-deleted per the docs
  convention, or reworded — which arms a new key and a new task), guarded
  by harvest provenance (payload dedup key; `catchup:` prefixes stripped)
  so external work is never touched. `tq agent-pool` runs one prune sweep
  synchronously before any actor starts (`--prune-stale=false` opts out) —
  synchronous because the worker's first claim would race an in-actor
  sweep and promote cancellable zombies to running. E2e pins the
  zero-zombie relaunch.
- **Write-route rate limiting** (2026-09-09): three failed CSRF tokens lock
  the offending client IP out of both admin write POSTs for 60s (429 before
  CSRF runs; reads unlimited; success resets the strikes), smoke-asserted
  in `scripts/smoke/webui.sh`.
- **`tq facts --json` / `--detail`** (2026-09-09): machine-readable fact
  list and per-fact full non-truncating detail rendering below each line —
  multi-line verify tails stay intact and greppable.
- **`tq stats --json` `journal_head`** + honest budget labeling (2026-09-09):
  JSON parity with the text output's journal watermark; the budget line is
  labeled `(all projects)` whenever `--project` scopes the table.
- **`Filter.Since` SQL pushdown** (2026-09-09): inclusive `created_at`
  window in both stores (`pgWhere` extracted; CountTasks shares it),
  `tq tasks --since` rides the pushdown instead of a CLI-side filter.
- **`RequeueEvidence` on `task.requeued` facts** (2026-09-09): structured
  `{reason, retry_in_ms}` detail in both stores (was a plain error string).
- **Docs-honesty guards** (2026-09-09): `scripts/check-todo-list.sh`
  (unblocked owner-gated items fail), `scripts/check-features-roadmap.sh`
  (seed shipped-vs-planned contradictions; caught D80/D90 still listed as
  raw ideas in ROADMAP on its first run), a DATE-column check in
  `check-status-index.sh`, and `scripts/install-pre-commit.sh` wiring both
  into a local pre-commit hook; all run in `ci-local.sh`.
- **Bootstrap parity + passthrough** (2026-09-09): `--repo-interval`,
  `--dlq-backoff`, `--log-dir-max-age`, `--log-dir-max-bytes` flow through
  `tq bootstrap` into pool args and the generated `pool.conf`; the dry-run
  now reports truthful crushrc change state (it always claimed "changed").

### Changed
- **Dirty-tree preflight requeues escalate** (2026-09-09): consecutive
  refusals double the base backoff (capped at 15m) with ±20% jitter so a
  sustained-dirty repo stops bouncing at the base delay, and the per-task
  refusal log is rate-limited to once per minute. `agent-pool` also
  `MkdirAll`s its `--log-dir` at startup instead of warning on the first
  sweep.
- **One status-color table + shared empty states + shared banner consts**
  (2026-09-09): badge and board-accent colors come from the same
  `statusColorTable`; the task table and board share one empty-state
  helper; the serve banner text is a `webui` const shared by the CLI print
  and the e2e parser (twice nearly broken by string drift).
- **Journal browser opens at the live tail** (2026-09-09): `/api/facts`
  gains a `after=-N` tail window and the browser's "load older" now pages
  backward from the newest facts (prepending), instead of forward from
  seq 0 while labeled "older".
- **Postgres exhausted-path fact detail fixed** (2026-09-09): the final
  `task.failed` fact classified as `transient` (SQLite says `exhausted`)
  and the dead-letter fact carried no class detail — both now match the
  SQLite store.

### Fixed
- **CI flakes deflaked** (2026-09-09): `TestHeartbeatExtendsLease` (40ms
  lease expired before the first heartbeat on slow runners; now 500ms) and
  `TestConcurrentClientsRace` (100ms SSE collect window deadlined during
  dial under `-race`; now 500ms) — the two recurring red-master causes.
- **Filter form no longer drops an active sort** (hidden `sort` input),
  the screenshots script's detail URL uses a real task id (was a JSON
  fragment), the fmtAge client/server parity is pinned by test, the ghost
  `webui-css-drift-check` reference is gone, and the dead
  `PostgresStore.migrateOnOpenFail` field is deleted (round-5 defect
  batch, 2026-09-09).

- **Admin writes in the dashboard — `tq serve --allow-writes`** (2026-09-09):
  an opt-in control layer (`$TQ_SERVE_WRITES=1`) registering exactly two
  CSRF-guarded routes — `POST /task/{id}/cancel` (pending → direct cancel
  with reason; running → cooperative `CancelRunning`) and
  `POST /task/{id}/rescue` (`RescueDead`, fresh budget). Forms embed a
  `tq_csrf` cookie-backed token verified constant-time (forged → 403), open
  as native `<details>` reason forms (zero JS under the CSP), and appear as
  row/detail affordances (stop/cancel/rescue); a banner states
  writes-enabled. Live-verified over real HTTP (a scratch task cancelled
  end-to-end, reason on the `task.cancelled` fact). Read-only remains the
  default; enqueue-from-UI and bulk actions stay out of scope.
- **LAN dashboard hardening + redesign** (2026-09-09): `?token=` now issues
  an `HttpOnly SameSite=Lax` session cookie (`tq_token`) so subresources
  (CSS/JS/favicon/SSE) authenticate; the CSP moved to a per-request 128-bit
  nonce regime (`script-src 'self' 'nonce-…'`, never `unsafe-inline`); the
  ghosted `app.js` SSE client (silently dropped by the templ-components
  sweep) is restored and pinned by test; the overview gained an always-dark
  instrument band (RUNNING/PENDING/DEAD/COMPLETED/CANCELLED ledger
  segments) and a two-tier ACTIVE NOW/settled task table; unknown task ids
  get a styled 404 through the layout.

- **`tq tasks` list view** (2026-09-09): one row per task with
  `--project/--status/--type/--since DUR/--limit/--json` — reconstructing a
  completion window stops requiring raw sqlite reads (21:40 §e7). Full IDs
  (they are the `tq show`/`tq cancel` handle), newest first, attempts and a
  last-error excerpt per row.
- **`tq harvest --prune-stale`** (2026-09-09): cancels PENDING queue tasks
  whose TODO_LIST item is now `[x]` (dedup-key match) so a pool relaunch
  never inherits zombies; the reason lands on the `task.cancelled` fact.
  Running tasks are reported only (a cooperative stop stays an operator
  `tq cancel --force` decision); dead ones are listed for the DLQ flow.
  `--dry-run` and `--json` honored.
- **Failure evidence on `task.failed` facts** (2026-09-09): executors
  publish `FailureEvidence{stage, exit_code, tail}` (agent run, agent
  verify — whose output tail was previously discarded on error — and sh
  command) and both stores append it to the failed-attempt fact — a failed
  task is debuggable from the journal alone instead of an empty `{}`.
- **`Task-Queue-ID` commit footer contract** (2026-09-09): the agent,
  catch-up and status prompt contracts tell agents to end commit messages
  with `Task-Queue-ID: {{TASK_ID}}`; the executor resolves the placeholder
  at run time (the queue ID does not exist when the harvester renders the
  prompt), so `git log` and `tq facts` cross-reference.
- **`tq show`/`tq cancel` accept task-ID prefixes** (2026-09-09): a unique
  ULID prefix resolves; an ambiguous one names its candidates instead of
  guessing.
- **`tq stats --json` aggregate + budget spend** (2026-09-09): `--json`
  emits `{by_status, by_project, budget{spent_today,cap}, consumer_lag}`
  for scripts and dashboards (the raw task list is `tq tasks --json` now),
  and the text output always shows `budget today N/M enqueued` (compare
  against a cap with `--daily-budget N`).
- **Sidecar byte budget — `agent-pool --log-dir-max-bytes`** (2026-09-09):
  caps the total size of the `--log-dir` sidecar directory (oldest `*.log`
  deleted first each tick; `$TQ_LOG_DIR_MAX_BYTES`). Closes the round-6
  retention item's open half — age sweeps cannot bound a high-traffic dir.
- **Postgres conformance battery** (2026-09-09): `TestPostgresConformance`
  runs the full queue semantics (DAG gating, delay/priority order, retry
  ladder with evidence, permanent dead-letter, requeue, cancel reasons,
  heartbeat, filter/list/count, bounded fact reads) against the Postgres
  store in CI's `-run TestPostgres` job. Also fixed a divergence it
  caught: `FactsForTask(limit>0)` returned the FIRST n facts on both
  stores while the interface documents the MOST RECENT n — both now read
  the tail and return it ascending.
- **`scripts/smoke/bootstrap-install.sh`** (2026-09-09): asserts
  `tq bootstrap --install` renders the unit + pool.conf into a fake
  `$HOME` with stubbed systemctl/loginctl and never touches the host;
  wired into ci-local.sh.
- **`scripts/check-status-index.sh`** (2026-09-09): ci-local fails when a
  `docs/status/*.md` report is missing from the index; every report is now
  indexed exactly (10 more stragglers beyond the five known were found).
- **Nightly fuzz campaign rotation** (2026-09-09): `scripts/fuzz/nightly.sh`
  rotates over every fuzz target (`FuzzParseRepo` +
  `FuzzExtractResultPayload`) and the nightly workflow commits both seed
  corpora.
- **ADR-0010: journal retention stance** (2026-09-09): compaction stays
  operator-owned and manual, the CLI surface waits for real demand, the
  retention floor becomes first-class observability when it lands.

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
  [docs/planning/archived/2026-09-07_16-25_SUPERB-PLAN-ROUND3-LIVE-WEB-UI.md](docs/planning/archived/2026-09-07_16-25_SUPERB-PLAN-ROUND3-LIVE-WEB-UI.md);
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
### Changed
- **Web UI task trail surfaces cancellation reasons** (2026-09-09): a
  `task.cancelled` fact carrying `{"reason": ...}` renders `— <reason>` in
  the detail timeline, so a withdrawn task answers "why" inline.

- **Board view — `tq serve` kanban projection** (2026-09-09): `/?view=board`
  swaps the task table for a read-only kanban board — one column per
  lifecycle status (pending → running → completed, with dead/cancelled
  exits), status-colored column rules matching the badge hue language,
  newest 25 cards per column (live ages, readiness countdowns, lease
  owners, dead-card attempt/error tails), true per-status counts, and a
  "+N older" link into the status-filtered table when a column truncates.
  The view rides the URL like every other filter (project/query apply; a
  status filter is dropped on the board — columns ARE the statuses), the
  table/board toggle sits in the tasks heading, `app.js` forwards `view`
  to `/api/events` so SSE snapshots render the on-screen projection, and
  the board reuses the `#frag-table` container so the swap protocol is
  unchanged. Zero new routes (read-only guardrail untouched); drag-and-
  drop card movement is queue mutation and stays behind the future
  `--allow-writes` gate (ADR-0003 Phase D). Covered by `board_test.go`
  and the webui smoke script.
- **NixOS module — `flake.nixosModules.default` / `deploy/nixos/tq-agent-pool.nix`**
  (2026-09-08, ROUND8 A1-A6): declares `services.tq-agent-pool` (enable,
  package, user, dbPath, `poolSettings` → flat `key = value` pool.conf with
  flag > env > file precedence, `extraArgs`, and a `serve` sub-module for
  the read-only dashboard) with the drain invariants baked in
  (`KillSignal=SIGINT`, `KillMode=process`, `TimeoutStopSec=45min`,
  `ProtectSystem=full`, no ProtectHome restriction — agents write/commit
  inside `$HOME`). Pool dbPaths outside `/var/lib` get a
  `RequiresMountsFor` mount gate (bank-sync pattern). A
  `checks.module-eval` flake check instantiates the module on both the
  default and the deployment shape so option typos fail `nix flake check`.
  Deployed on evo-x2 via SystemNix (see that repo's `docs/services/tq.md`).
- **Daemon-backed repo discovery — `tq harvest` / `tq agent-pool --discovery-addr`**
  (2026-09-08): point the harvester's repo enumeration at a
  project-discovery-daemon (`POST /v1/discover` over a unix socket, `unix://`
  prefix, or host:port; env `TQ_DISCOVERY_ADDR`) instead of the depth-1
  TODO_LIST scan, so discovery cost is amortized daemon-side and
  activity/exclusion/language filters apply. The response mapping is
  contract-identical to the scan: project paths become harvestable repos
  (absolute, deduplicated, gated on the todo file, sorted). Additive by
  contract — the scan stays the default (zero external services), and an
  unreachable or erroring daemon costs ONE warning per tick plus the local
  scan fallback; a tick never fails because of discovery. `--repo-subset`
  filters the daemon result too.
- **Cancel reasons — `tq cancel <task-id> --reason "<why>"`**
  (2026-09-08): a non-empty reason is stored as `{"reason": ...}` in the
  `task.cancelled` fact's detail, so cancellations are no longer
  forensics-blind. The pending path records it directly; the cooperative
  `--force` path rides it on the `task.cancel-requested` fact and both
  finalizers (worker `CancelOwned`, lease-expiry reclaim) carry it onto the
  final `task.cancelled` fact next to `cooperative: true`. An empty reason
  keeps the detail empty. The documented `cancel <id> --reason why` order
  parses too (flags are hoisted ahead of positionals; `Store.Cancel` /
  `CancelRunning` take the reason, SQLite + Postgres).
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
  `docs/planning/archived/2026-09-08_cordis-test-suite-verification.md`.
### Fixed
- **Budget guard now gates EVERY minting pass** (2026-09-09): the agent
  pool's review/status sweepers, cqa ingest, and the `--once` drain sweeps
  never checked the daily budget — a completion inside the same tick could
  spend the last slot and still mint status/review tasks past the cap,
  contradicting SECURITY.md's "caps EVERY enqueue incl. status-minted".
  Each pass now re-checks `guard.Check` and skips with a logged reason
  (pinned end-to-end by `TestBudgetCapsStatusMintedEnqueues`).
- **Deflaked `TestRestartMidStreamLosesZeroFacts`** (2026-09-09): bridge
  A's crash window widened (200ms poll could pre-checkpoint seq 549 before
  the cancel) and the wait deadline raised for loaded `-race` machines.

- `scripts/smoke/papdashboard-e2e.sh` real-dashboard mode no longer
collides with a locally running pap-raw-server on the fixed port 18099:
the mode now derives an ephemeral port and liveness-checks the dashboard
before driving it.

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

