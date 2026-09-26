# go-taskqueue — Agent Guide

Projects-aware task work queue / worker pool for Go. Embedded SQLite journal,
lease-based claims with crash reclaim, DAG dependencies, retries with a
dead-letter queue, pluggable executors (including headless AI coding agents).
Zero external services — one Go binary, one file.

**STATUS: v0.1.0 shipped 2026-09-06, v0.2.0 shipped 2026-09-09; v0.3.0
(facade release) tags pushed 2026-09-13 — facades live on the module
proxy, master CI green; actively developed by MULTIPLE concurrent
agents.** Re-read files and re-run tests before editing; expect
uncommitted changes from parallel sessions — read them, judge them, build on
them, never revert them.

## Commands

```bash
./scripts/ci-local.sh     # the pre-push gate: full CI replicant (vet/build/race/smokes/nix); tree-reading Go gates retry transient foreign breaks (sleep 45s ×3, then fail with "concurrent edit in flight" context — 15-39 f9/e2; TRANSIENT_POLL_SECS/TRANSIENT_MAX_POLLS overridable)
export GOEXPERIMENT=jsonv2 GOTOOLCHAIN=auto; go build ./... && go vet ./... && go test ./... -race   # standard verify gate (ROOT MODULE ONLY — see below; GOTOOLCHAIN=auto is REQUIRED outside the flake devShell because go.mod declares go 1.27.1 since 2026-09-16 and this host's shell pins GOTOOLCHAIN=local on a 1.26.7 binary — without it every go command dies with "go.mod requires go >= 1.27.1"; GOEXPERIMENT=jsonv2 rides along as an accepted NO-OP on 1.27, where json/v2 is stable std — on 1.26 it was load-bearing, keep it so every surface carries one identical env story until a toolchain-policy ruling drops it)
nix build                 # reproducible build; nix run .#test = tests; nix run .#webui-css = stylesheet
./scripts/fuzz/nightly.sh # 60s FuzzParseRepo campaign; nightly workflow commits new seeds
```

**Multi-module repo (ADR-0011):** `internal/{task,journal,queue,executor,worker}`
are sub-modules plus `internal/queue/{sqlite,postgres}` backend modules
and the `internal/journal/cqrs` go-cqrs-lite adapter module (ADR-0011 +
ADR-0012 + ADR-0014; import paths unchanged); the root module is the app
layer. `cmd/tq` is ITS OWN replace-free module (ADR-0017) so
`go install …/cmd/tq@vX.Y.Z` works — in-repo builds go through the
generated devmod shim (`scripts/build-tq.sh` for binaries,
`scripts/test-cmd-tq.sh` for the CLI gate; plain `go build` inside cmd/tq
fails with "ambiguous import" until the next root tag containing the split
lands on the proxy). `./...` never descends into nested modules —
per-module gates (disk-derived, same as CI):

```bash
for m in $(find internal task journal queue executor worker -name go.mod | sed 's|/go.mod$||' | sort); do
  ( cd "$m" && export GOEXPERIMENT=jsonv2 && GOWORK=off go build ./... && GOWORK=off go vet ./... && GOWORK=off go test ./... -count=1 ) || exit 1
done
./scripts/test-cmd-tq.sh   # cmd/tq module (ADR-0017): devmod shim gate; CMD_TQ_OS=windows for cross-compile
```

**Public facades (ADR-0016):** `task/`, `journal/`, `queue/`,
`queue/sqlite/`, `queue/postgres/`, `executor/`, `worker/` are facade
MODULES re-exporting the internal implementations via type aliases — the
only importable surface for external consumers. In-repo code keeps
importing `internal/…` directly. Rules: every internal module in a
facade's dependency graph needs a require (vX.Y.Z) AND a relative replace
in the facade go.mod (missing replaces resolve through the proxy and hit
stale tags — worker's facade failed exactly that way with
`journal.Reprioritized` undefined); when adding an exported symbol to a
facaded internal package, add the alias in the same change — parity is
GATED since 2026-09-13 by `scripts/check-facade-parity.sh` (go/parser
walk in `scripts/facadeparity`, wired into ci-local + CI; the audit's
first catch was the missing `PrioritizeExecutor` alias); facade tests may
import internal packages (same-path rule), never sibling FACADES (would
pin unpublished versions). `postgres.OpenWithPool` pools are
CALLER-OWNED: `Store.Close` closes only pools the store opened via `Open`
(ownsPool flag; the first live TQ_TEST_POSTGRES run caught Close tearing
down caller pools — the env-gated CI postgres job runs that test, so a
broken ownership model is a RED MASTER, not a local skip).
`examples/embed` is a separate module importing ONLY facade paths (the
adopter on-ramp; ci-local builds it as a rot guard). Release gates run
`gate_gomod` over EVERY module go.mod (containment-based replace rule +
single-line-require tag check — fixtures in smoke/release-gates.sh), so
after a version sweep the bumped sub-tags must be PRE-CUT before
`release.sh` gates run.

Internal requires point at real tagged versions (never `v0.0.0` —
`go install` resolves them via the proxy; `internal/*/vX.Y.Z` subdirectory
tags ride every release) + relative `replace` for local dev (NO go.work —
replace-only by decision); `go test ./internal/foo` from root FAILS by design
(cd into the module instead — and a from-root directory pattern can even
silently PASS via the root's require+replace while a sibling fails setup in
the same invocation, 2026-09-21 00-39: budget ok / sqlite setup-failed; the
in-module GOWORK=off run is the only canonical gate). Release flow and version surfaces are
documented in `docs/release/RELEASE.md` (two-phase --tag/--push, sub-tag
cutting, allowlist gates) and `docs/release/VERSION-SURFACES.md` (the seven
surfaces and their bump order).

Smokes (all CI-safe; `TQ_BIN=result/bin/tq` smokes the nix-built binary):

```bash
./scripts/smoke/webui.sh        # worker + tq serve + HTTP/SSE + write-route lockout assertions
./scripts/smoke/status-loop.sh  # stub agent; sweeper mint → report → TODO append → re-arm
./scripts/smoke/dogfood-once.sh  # stub agent; harvest → work (footer commit) → review approve; TQ_DOGFOOD=1 runs the real-agent proof (spends money)
./scripts/smoke/bootstrap-install.sh  # --install renders unit + pool.conf against a fake $HOME
./scripts/smoke/journal-drift.sh # tq audit --journal over a seeded scratch fixture (ADVISORY, O5)
./scripts/smoke/help-text.sh    # every tq subcommand help: no parenthesized-identifier artifacts (rename-leak class, 08:42 e3/f2)
./scripts/smoke/multi-repo.sh   # two agent-pool processes, one DB, three repos: per-project exclusivity + dedup (D24)
./scripts/smoke/papdashboard-e2e.sh # stub dashboard: dead letter raises alert.triggered, dlq --rescue posts alert.resolved
./scripts/smoke/questions-e2e.sh # stub agent + stub dashboard: tq ask → parked (no attempt burn) → forwarded → answered → unblocked → completed (D-questions)
./scripts/smoke/ratelimit-e2e.sh # hermetic: a Z.ai-429 stub failure parks the task (no attempt burned)
./scripts/smoke/fullcore.sh  # examples/fullcore drains 4/4 on sqlite + deadline path xN (50ms timeout calibrated: trips on this host at 250ms worker-start latency; DEADLINE_RUNS/DEADLINE_TIMEOUT_MS knobs; TQ_TEST_POSTGRES adds the postgres variant)
./scripts/smoke/reviews.sh   # stub reviewer; approve + request_changes + autofix loop
./scripts/smoke/session-close.sh # session-close bridge: begin → footer commit → close → one review + one status + replay-safe second close
./scripts/check-guard-wiring.sh # orphaned-guard audit: every check-*/smoke script must be referenced by ci-local/ci.yml/flake or be deleted
./scripts/smoke/release-gates.sh # fixture go.mods: release allowlist/tag gates, positive + negative; runs IDENTITY-BLIND via ci-local (GIT_CONFIG_GLOBAL=/dev/null + user.useConfigOnly — bare /dev/null is INERT, this host autodetects GECOS identity) and self-tests the property with a stripped-commit fixture — never "simplify" the sanitizer or the -c user.* flags away
./scripts/check-go-mods.sh      # replaces, pins, toolchain alignment, go mod verify (all modules)
./scripts/check-dead-exports.sh # advisory dead-export audit: zero-importers detector, substring matching (NOT rg -w)
./scripts/check-script-syntax.sh # bash -n + shellcheck (severity >= warning) hard gate over every tracked *.sh — zero-findings policy since birth (2026-09-15)
./scripts/check-facade-parity.sh # ADR-0016 gate: go/parser walk, every internal export needs a kind-compatible facade alias (scripts/facadeparity)
./scripts/check-transient-retry.sh # durable behavior pin for ci-local's with_transient_retry: sed-extracts the SHIPPED helper, marker-guards the extraction, exercises heal/exhaust/TRANSIENT_MAX_POLLS=0 semantics sub-second (01-46 f2; ci-local step)
./scripts/new-module.sh <dir> [deps…] # scaffold a new module go.mod (latest cut tag + relative replace per dep; no hand-writing go.mods)
nix run .#test                  # full multi-module suite (root + every internal/* module)
scripts/build-tq.sh /tmp/tq     # CLI scratch build (cmd/tq is its own replace-free module, ADR-0017; the script runs the devmod shim)
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

| Package                                            | Purpose                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| -------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/task`                                    | Task record, Status enum with `CanTransitionTo`, sentinel errors                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| `internal/journal`                                 | Fact types, append-only Journal interface, MemoryJournal                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| `internal/journal/cqrs`                            | Read-only adapter: the fact journal as go-cqrs-lite `event.Journal`/`event.SeekableJournal`; synthetic seq-encoded ULIDs (ADR-0014); `tq facts --cqrs` consumes it                                                                                                                                                                                                                                                                                                                                   |
| `internal/queue`                                   | Store contract: interface, Filter, Queue facade, watermarks entry (deps: task+journal only)                                                                                                                                                                                                                                                                                                                                                                                                          |
| `internal/queue/sqlite`, `internal/queue/postgres` | Driver-style backend modules (`sqlite.Store`/`Open`, `postgres.Store`/`Open`); mirrored helpers + conformance suites (ADR-0007/0012)                                                                                                                                                                                                                                                                                                                                                                 |
| `internal/worker`                                  | Claim → heartbeat → execute loop; concurrency, panics, drain, preflight requeue ladder                                                                                                                                                                                                                                                                                                                                                                                                               |
| `internal/bridge`                                  | Outbound bridges: papdashboard (alerts), cqa (findings → fix tasks)                                                                                                                                                                                                                                                                                                                                                                                                                                  |
| `internal/executor`                                | Pluggable execution: `sh`, HTTP, agent (headless AI), review, status, registry                                                                                                                                                                                                                                                                                                                                                                                                                       |
| `internal/harvest`                                 | Scans repos' TODO_LIST.md into agent tasks; drift audit (`tq audit`); prune-stale sweeps                                                                                                                                                                                                                                                                                                                                                                                                             |
| `internal/budget`                                  | Daily-cap + budget-command projections over the journal, checked before each pool tick; `UsageToday` additionally sums derived session tokens/cost from the day's completion facts (AgentResult + PrioritizeResult + ReviewResult + StatusResult share the usage json keys, drift-pinned by budget_test marshalling the real executor types) — surfaced in the Check refusal reason and `tq stats` (`budget.session_usage`), cap SEMANTICS stay task-count pending the owner's token-vs-count ruling |
| `internal/dlqfix`                                  | DLQ-autopsy sweeper (`--dlq-fix`): dead agent tasks gain ONE autopsy task; `fixed` verdict rescues, `wontfix` dismisses (`Dead → Cancelled` via `DismissDead`)                                                                                                                                                                                                                                                                                                                                       |
| `internal/review`                                  | Sweeper: completed agent tasks gain ONE review task; `--review-autofix` mints fix tasks                                                                                                                                                                                                                                                                                                                                                                                                              |
| `internal/status`                                  | Sweeper: every N agent completions per project mint ONE done-prompt report task (`--status-every`)                                                                                                                                                                                                                                                                                                                                                                                                   |
| `internal/prioritize`                              | Score-cache sweeper (`--prioritize`): repos holding unscored backlog items mint ONE machine-band batch-scorer task; verdicts cache into `priority_scores` and re-rank PENDING tasks (marker > AI > keyword)                                                                                                                                                                                                                                                                                          |
| `internal/depsweep`                                | Dependency-upgrade sweeper (`--dep-sweep`, 2026-09-15): depgraph `update-plan --format json` → depbump tasks (stale builds → release tasks, consumers → exact-pin bump tasks wired by the plan's DAG); trap rows skip; plan JSON is the ONLY seam (no depgraph import)                                                                                                                                                                                                                               |
| `internal/watermark`                               | Durable journal cursor shared by the four journal-driven sweepers (review/dlqfix/status/prioritize): watermarks-table row, paged Facts reads, checkpoint AFTER each page, head bootstrap; `Cursor.Sweep` serializes and owns the pump semantics                                                                                                                                                                                                                                                      |
| `internal/consumer`                                | Journal dispatcher: per-subscriber cursor, at-least-once in-order, lag observability (ADR-0009)                                                                                                                                                                                                                                                                                                                                                                                                      |
| `internal/runactor`                                | run.Group actors, LIFO `OnShutdown`, `InterruptOn` (2nd signal = exit 130), detached task contexts                                                                                                                                                                                                                                                                                                                                                                                                   |
| `internal/webui`                                   | Live dashboard (`tq serve`): journal tailer → hub → SSE server-rendered fragments (ADR-0003)                                                                                                                                                                                                                                                                                                                                                                                                         |
| `internal/httpapi`                                 | Machine API (`tq api`): queue projection + enqueue over HTTP; mandatory token on every bind, nosniff everywhere, 3-strikes bearer lockout (2026-09-16 hardening; both surfaces now share `internal/lockout`)                                                                                                                                                                                                                                                                                         |
| `internal/lockout`                                 | Shared 3-strikes strike limiter behind webui's write routes and httpapi's bearer auth: fixed-window lockout, per-contact idle pruning, bounded map (global sweep + LRA eviction), injectable clock, `OnLock` hook                                                                                                                                                                                                                                                                                    |
| `cmd/tq` (module, ADR-0017)                        | CLI: enqueue / worker / harvest / agent-pool / bootstrap / stats / tasks / audit / top / show / dlq / cancel / ask / facts / tail / watermarks / session / serve / api / doctor / crush / version                                                                                                                                                                                                                                                                                                    |

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
  Verify must exit 0, and MINTED Go verifies (bootstrap auto-detect) must
  be ENV-SELF-CONTAINED: they carry `export GOEXPERIMENT=jsonv2;` so the
  gate is identical inside and outside the flake devShell (the pool unit
  env carries no GOEXPERIMENT — the env lie that burned 5+ windows,
  2026-09-11 task 000001a08ebf; this repo's `.tq-verify` is owner-owned
  and prelude-free until that unit-env fix lands — an agent must never
  edit its own gate, review ruling 2026-09-12). A
  payload model makes the executor pass `crush run
  -m`, which RESETS reasoning effort — the repo `.crushrc` managed block
  (`tq bootstrap`) is the only model+effort carrier (v0.94.1 DID add
  `crush run --reasoning-effort <level>` — verified 2026-09-14 on this
  host: accepted for zai/glm-5.3-flash incl. `xhigh`; a mistyped level
  fails with a misleading "does not support reasoning effort" instead of
  listing accepted values, upstream-issue candidate — payload-carried
  effort stays a non-goal until a pinned-model task needs it). Owner
  ruling 2026-09-14: glm-5.3-flash levels are low|high|xhigh and the pool
  ALWAYS wants xhigh. Crush v0.93.1 added `option request_timeout <s>`
  (per-request LLM timeout, default 60s of stream inactivity) — the knob
  if reasoning stalls ever kill runs; none in the DLQ as of 2026-09-14
  (429 storms dominate; the 2026-09-11 06-07h 429 deaths predate
  DetectRateLimit, created 15:07 that day). The managed block's
  tool grant IS the agent's toolset (headless mode denies unlisted tools,
  no prompts): `view ls grep glob edit multiedit write bash fetch download
  todos` + `option metrics false` (headless telemetry off — every run
  otherwise pays a PostHog flush at shutdown; `fetch` verified empirically
  2026-09-12). Agents inherit the user's global crush config (providers,
  skills, LSPs, qmd MCP) since the pool runs as the same user; repo-local
  `lsp add <name> --disabled true` overrides broken host LSPs (statix on
  this host has no LSP mode — disabled in SystemNix 2026-09-12 after 900+
  init-timeout failures). `--yolo` without a
  repo-local `.crushrc` fails fast by design (argv pinned by
  `TestAgentExecutorArgvContract`). With `--task-closeout` the work turn is
  followed by a close-out turn that resumes the EXACT session (`--session`,
  never `--continue` — concurrent agents) to run the brutal a)-g) self-review
  (it no longer re-emits any result line — derivation covers it);
  the report lands at `docs/status/<ts>_task-<id>.md`. Reviews and status
  tasks run a close-out-free clone (they ARE the second opinion).
- **Derived outcomes (2026-09-14, owner ruling "no self-report")**: the
  queue DERIVES what an agent run did — commits via the `Task-Queue-ID`
  footer (`executor.GitLogScanner`, moved from internal/session), files via
  `git diff-tree` over those commits, and session usage (cost/tokens/
  messages) via `go-crush-data` (executor dep, read-only, registry lookup
  repo→dataDir; only when the run's session id was extractable). Derivation
  fills `AgentResult{Commits, FilesChanged, CommitSHA, Session*}` after
  verify; the legacy `TQ_RESULT` stdout self-report only fills gaps for
  in-flight tasks. NO prompt teaches the self-report anymore (pinned by
  `TestAgentPromptsDropSelfReport`). The recurring no-op-sha-semantics
  ruling asks died with the choice: a no-op re-dispatch derives "zero
  footer commits" automatically. Design + MCP rejection:
  docs/planning/archived/2026-09-14_derived-outcomes-verdict-channel.md.
- **Verdict channel (`tq verdict` + `$TQ_RESULT_FILE`, 2026-09-14)**:
  runAgent hands every agent process a per-run temp file via the
  `TQ_RESULT_FILE` env; verdict-gated tasks record their structured result
  by running `tq verdict '<one-line JSON>'` (validates JSON, writes the
  file, no DB access). runAgent appends the file's content to the returned
  output as the LAST `TQ_RESULT:` line — file outranks any stdout line
  (`ResultLine` now honors its documented last-line-wins semantics;
  FindStringSubmatch had silently read the FIRST line for the regex's whole
  life, pinned by `TestVerdictFileOutranksStdoutLine`). A stdout
  `TQ_RESULT:` line remains the LEGACY fallback (in-flight pool tasks,
  stub smokes) — delete it only after the live pool shows derived
  outcomes.
- **Batched harvest + backlog grant (`--batch-items N`, 2026-09-14, default
  OFF)**: with `--batch-items` > 1 the harvester groups up to N ADJACENT
  admissible items of the same TODO_LIST section into ONE agent task (one
  session, `DefaultBatchPromptTemplate`: per-item commit+footer+checkoff,
  per-item `— BLOCKED:` escape = partial success, retry skips `[x]`
  members). Dedup = `batch:` + hash over the SORTED member keys (reorder
  never forks, member edit forks); payload carries `items` +
  `itemKeys` (`Item` stays the first member — review quoting/status windows
  keep working); `TimeoutMinutes` scales per member (raise `--task-timeout`
  with it); priority = max over members; marker level = max. A batch is ONE
  task against every gate (--max-per-tick, daily budget — per-item cost is
  amortized, so raise gates CONSCIOUSLY). prune-stale cancels a pending
  batch only when EVERY member is ticked/absent; `tq audit` maps member
  items to the batch (catch-ups stay per-item); batch keys are invisible to
  the `todo:`-keyed AI scorer (documented interaction). BOTH work prompts
  (single + batch) grant the backlog move: agents MAY append NEW unchecked
  follow-up items to TODO_LIST.md — the pacing/budget/priority gates own
  admission, so the grant cannot bypass spend control (direct `tq enqueue`
  stays forbidden: mint-bypass). Pinned by `internal/harvest/batch_test.go`
  - batch rows in the prompt guardrail pins. Design + rejected
    alternatives: docs/planning/archived/2026-09-14_batched-harvest-agent-power.md.
- **`review`**: `ReviewPayload` JSON. Both verdicts COMPLETE the task; the
  mechanical gate is a valid verdict JSON recorded via `tq verdict`
  (`TQ_RESULT_FILE` channel; legacy stdout line still honored).
  Session usage (cost/tokens) is DERIVED for review turns too (2026-09-21,
  09-52 §f3): `deriveOutcome` fills ReviewResult's Session* fields after the
  run, same json keys as AgentResult/PrioritizeResult, so reviews join the
  budget token projection via the shared parse (drift-pinned in
  budget_test's usage test); reviews make no commits, so only usage is
  surfaced.
  Findings are commit-anchored (2026-09-15 hardening, the 05-58
  stale-anchor lesson): every `request_changes` finding must carry a
  quoted verbatim `anchor` — bare positions ("lines 12-18", "x.go:34",
  "12-18", "L40") are REJECTED at parse time by the re-anchoring
  pre-flight, so the attempt fails while the reviewer can re-file — and
  gets `commit_sha` backfilled from the review payload when the model
  omits it. Fix prompts carry the anchor text, the finding's sha (payload
  sha as fallback), and the dangling-commit disposition (`git cat-file -e`
  first; re-anchor via the quoted text; anchor gone = finding no longer
  applies, never invent a change). Footer ruling: exactly ONE
  `Task-Queue-ID` footer per commit — the fix ticket's own; the original
  task's lineage lives in the queue, not a second footer.
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
  Session usage (cost/tokens) is DERIVED for report runs too (2026-09-21,
  09-52 §f4): `deriveOutcome` fills StatusResult's Session* fields after
  the run, same json keys as the other result types, so report turns join
  the budget token projection via the shared parse. The reporter COMMITS
  its report, but StatusResult deliberately surfaces usage only (the item
  asked for usage fields; the report path is already recorded on the
  result).
  (DEDUP-GATED since 2026-09-17, the 14-01 report's top fix for the
  re-dispatch loop where reworded duplicates minted fresh dedup keys and
  one task re-fired up to 5x: the prompt requires a dedup check against
  existing unchecked items, HARD CAP 10 new items, and permits
  evidence-backed `[x]` ticks of already-done items, the only allowed
  edit to existing lines — unticked-but-done items were the proven
  LOST-signal root cause).
  Its docs-health pass annotates (never rewrites) 2026-* reports, keeps
  TODO_LIST/CHANGELOG/AGENTS/README/ROADMAP/FEATURES current with what
  the window's tasks actually shipped, and archives fully-done reports
  to `docs/status/archived/`; hard scope: docs only, never code/config.
  Two gates: the recorded verdict naming an existing REPO-RELATIVE
  report file (`tq verdict` channel), and the repo verify command. One report in flight per
  project; `status:<project>:<trigger-id>` dedup.
- **Idempotent enqueue**: `DedupKey` set → re-enqueue returns the stored
  task unchanged. A cancelled/dead task's key still suppresses re-enqueue;
  for harvested items the escape hatch is editing the item text (the key
  hashes repo + text).
- **`dlqfix` (DLQ autopsies, `--dlq-fix`)**: `DLQFixPayload` JSON (repo,
  dead_task, work, FailureEvidence, yolo). Dead AGENT tasks mint ONE
  `dlqfix:<dead-id>`-deduped autopsy task (type scope IS the loop guard: a
  dead autopsy never mints another; sh/review/status deaths stay human
  surfaces). Both verdicts COMPLETE; mechanical gate is the
  recorded `{"verdict":"fixed|wontfix","summary":...}` JSON (`tq verdict`
  channel) — wontfix
  without a summary is a failed attempt. The sweeper disposes: fixed →
  `RescueDead` with the dead task's ORIGINAL budget; wontfix →
  `DismissDead` (Dead → Cancelled, reason + `dismissed_by` on the
  cancelled fact; operators have `tq dlq --dismiss`). A rescued task that
  dies AGAIN gets NO second autopsy (dedup is forever) — the second death
  is a human surface. Autopsies run the closeout-free agent clone,
  dirty-capable by DEFAULT (a dead agent's partial work is evidence;
  only explicit `require_clean=true` restores the preflight), budget-gated
  like every mint, and the `{{TASK_ID}}` in the fix-commit footer resolves
  to the AUTOPSY's id. Cursor `dlqfix-sweeper` (head-bootstrapped,
  rewindable). Design: docs/planning/archived/2026-09-12_dlq-autopsy-design.md.
- **`prioritize` (AI batch scorer, `--prioritize`)**: `PrioritizePayload`
  JSON (repo, batch items with dedup keys, model/yolo/clean knobs). A
  repo holding UNSCORED backlog items (pending agent tasks with `todo:`
  dedup keys, uncached + not claimed by any minted batch) mints ONE
  machine-band (150) batch task, dedup
  `prioritize:<repo>:<hash-of-key-set>` — an unchanged set never mints
  twice (a dead batch recovers only on the next key-set change; the DLQ
  is the human surface until then). The scorer run is READ-ONLY,
  closeout-free (review-pattern clone); mechanical gate is the strict
  recorded `{"verdicts":[...]}` JSON (`tq verdict` channel; every item
  exactly once, scores
  0-100 — `executor.ParsePrioritizeResult`). The sweeper
  (cursor `prioritize-sweeper`, head-bootstrapped, budget-gated via the
  pool's mintPass) caches verdicts into `priority_scores`
  (`ai:batch-scorer`) and re-ranks matching PENDING tasks via
  `UpdatePendingPriority` source `ai`: marker items are protected by the
  payload-pinned `markerLevel` (marker > AI), hot/machine by
  `RepriMutable`, and scores clamp to the backlog band. `tq reprioritize`
  and the startup sweep feed the same cache (one ladder everywhere).
  Cache maintenance (2026-09-21): verdicts AGE (`DefaultScoreTTL` 30d,
  `SweeperConfig.ScoreTTL`; negative disables) — a stale item re-joins
  the next mint even though an old batch covered it, and the re-score's
  upsert overwrites in place; ORPHANED keys (item reworded/re-keyed,
  checked off, task terminal) are evicted at every scorer completion
  (`SweepStats.CachePruned`; store surface `PriorityScores` +
  `DeletePriorityScores` in both backends).
  Default OFF — it is AI spend.
- **`depbump` (deterministic dependency bumps, 2026-09-15)**:
  `DepBumpPayload` JSON (repo, exact `bumps[{module,version}]`, optional
  `release{version,push}`, require_clean, timeout_minutes). Flow:
  payload misses are Permanent; dirty tree is a Preflight requeue (someone
  else holds the repo); red baseline fails untouched; then `go get` exact
  pins under go.work quarantine → tidy → vendor-if-present → templ
  regenerate (soft skip without the binary; `TemplBin` override for
  pinned deployments/tests) → pin re-check → build+test verify → commit →
  dir-prefixed annotated tag (+ optional push, the irreversible step).
  Git scope honesty: add/checkout FATAL on pathspecs matching nothing, so
  the `*_templ.go`/`*_templ.txt` globs join the stage scope PER GLOB via
  `ls-files`, and rollback checks out CONCRETE tracked paths + cleans
  untracked leftovers on a DETACHED context (a checkout carrying one
  unmatched pathspec aborts restoring NOTHING — the 2026-09-15 lesson;
  rollback must survive the very timeout that triggered it). A re-bump
  that stages nothing SUCCEEDS (goal already met — two tasks racing to
  one pin must not DLQ the second). Minted by the depsweep sweeper
  (`--dep-sweep{,-bin,-dir,-interval,-push,-unreleased}`, default OFF):
  dedup `depsweep:<repo>:release:<v>` / `depsweep:<repo>:bump:<hash>`,
  machine-band priority, budget-gated via the pool's mintPass; skips
  never-released / suggested-major / trap rows (downgrades, `-dev`,
  not-newer). Design + research: the 2026-09-15 status report; planner
  side: project-dependency-graph `update-plan --format json`.
- **Session-close bridge** (`tq session begin/close` + the `tq crush`
  wrapper, prototype
  2026-09-12): interactive sessions get the pool close-out — begin mints
  `session.opened`; close scans `Crush-Session: <id>` git trailers (git ≥
  2.15 `%(trailers)`, ONE log call) and direct-enqueues ONE review (dedup
  `review:session:<id>`) + ONE status task (dedup
  `status:<project>:session:<id>`) — keys share the sweeper namespaces —
  then appends `session.closed` carrying commits + minted IDs. The
  synthetic `session:<id>` TaskID can never collide with a real task; zero
  attributed commits → fact only, nothing minted. Minted payloads pin
  `Yolo` and omit `Model` (the repo `.crushrc` owns model+effort);
  `--allow-dirty` mirrors into `RequireClean=false`. Close is enqueue-only
  (SessionEnd-hook budget). `(*sqlite.Store).AppendFact` is the ONLY
  sanctioned non-task fact write; never write task facts through it. Open:
  trigger automation (crush #3146), daemon-commit attribution gap, budget
  bypass, postgres parity; session/repo model documented (one close per
  session, first-repo wins — review dedup key is repo-blind, so a second
  repo's close is a replay that mints no fresh review; true multi-repo close
  is deferred) — docs/planning/archived/2026-09-12_session-close-bridge-design.md.
  `tq crush -- <crush args>` is the automation-friendly wrapper: runs a
  crush session and, on exit (clean or crashed), runs the replay-safe
  close using the session id from `$CRUSH_SESSION_ID` or the child's
  output; with no id derivable the child's exit code is preserved and
  nothing is closed.
- **PapDashboard questions (`tq ask`, 2026-09-17, SHIPPED)**: an agent
  parked on a decision asks the owner — `tq ask --task <id>` (RUNNING
  only) redacts, appends `task.question-asked`, writes the per-run
  `$TQ_QUESTION_FILE` marker (`QuestionAskedDetail` JSON); the executor
  converts the marker into `QuestionPendingError` (work-turn question
  skips verify+closeout; closeout-turn question arms `closeoutPending`
  resume) and the worker requeues WITHOUT burning an attempt, NotBefore =
  the question's expiry (`--expires` default 72h, cap 7d — the safety
  valve re-enters the task). Ref = sha256(taskID + normalized question)
  16-hex; re-ask converges (pending ref re-arms the marker, answered ref
  is a no-op). The bridge forwards with `task:`/`qref:` tokens LEADING the
  body; the AnswerPoller (runs under `--alert-url` in worker + agent-pool,
  watermark cursor `papdashboard-answers:<endpoint>`, bootstrap-at-now,
  at-least-once) routes answers home via `RecordAnswer` (idempotent per
  ref; injects `answered` into object payloads + clears NotBefore; raw
  payloads are fact-only). `tq show` renders the questions section
  (`questions[0].answered`). Store-level sentinels:
  `queue.ErrEmptyAnswerRef` / `queue.ErrEmptyAnswer` (both stores).
  DELIBERATELY NOT DONE: no prompt teaches `tq ask` — agents won't
  discover it until the owner rules on ask-policy (§g of the 21-04
  report). Design: docs/planning/archived/2026-09-06_decision-question-fanout.md
  (Accepted 2026-09-17 — fact-park + polling deviation from the original
  POST sketch).
- **`Task-Queue-ID` commit footer**: every prompt contract tells agents to
  end commits with it; the executor resolves the placeholder at RUN time.
  The footer must be the LAST line of the message — git's trailer parser
  (and therefore `executor.GitLogScanner.CommitsByTrailer`, the derivation
  channel, internal/executor/gitscan.go:145) reads only the final
  paragraph, so a footer placed ABOVE an attribution block
  (Crush/Co-Authored-By) is INVISIBLE to task attribution and invites
  re-dispatch; the 2026-09-19 0-target window had to msg-filter two
  unpushed commits to heal it (d035911/ca514a8 over dd543ec/e9ef358).
  Never hardcode the placeholder inside backtick raw strings (a backtick
  terminates the literal). The footer must carry the queue-assigned ID from
  the task prompt VERBATIM — if a reviewer or a second artifact supplies a
  different ID, use that one and report the discrepancy; never merge or
  silently pick between IDs (the f26 three-ID cluster is the cautionary
  tale).
- **Fact forensics**: `task.failed` carries `FailureEvidence{stage,
  exit_code, tail}` (tail size: one `EvidenceTailBytes` constant);
  `task.requeued` carries `RequeueEvidence{reason, retry_in_ms}`.
- **Enqueued-fact task snapshot (2026-09-24)**: the plain enqueue's
  `queue.EnqueueDetail` carries the FULL task snapshot — `payload`
  (key marshaled EXPLICITLY even when empty; key presence distinguishes
  post-growth facts from legacy thin ones), `deps`, `max_attempts`,
  `not_before`/`created_at` as unix millis (the task-row storage
  format) — in BOTH backends, so journal replay needs no task-row
  side-channel (ADR-0019 S1; pinned by the `enqueue fact detail carries
  identity` conformance pins in both backend suites). The upstream
  engine's own enqueued detail stays thin until the M4 ratification
  memo lands, and pre-growth journals keep the
  `internal/queue/sqlitev4/replay` verbatim copy (decision memo §6).
  RescueDead's re-emission stays `{"rescue":"true"}` marker-only — the
  rescued row already exists.
- **Secrets-in-logs redaction (default ON, `--redact=false` /
  `$TQ_REDACT=false` to disable, worker + agent-pool)**: every output tail
  goes through the redaction pass (`internal/executor/redact.go`) BEFORE
  the tail is cut — provider-token-shaped strings (sk-/sk-ant-/ghp_/AKIA/
  AIza/xoxb/bearer/auth-header/secret-assignments) are masked to
  `[REDACTED]` in evidence tails, the error text that embeds them
  (facts, worker logs, LastError) and both sidecar writers. `tailBytes` is
  the choke point for byte tails; depbump's line-based `tailOutput`
  delegates to the same redactOutput pass — a new output→evidence site
  needs no extra work.
  Escape hatch is per-POOL (a redacted debug tail is the tradeoff). The
  audit half: `tq audit --journal` scans stored fact error/detail of the
  output-derived fact types (failed/dead-lettered/completed/requeued —
  enqueue payloads are user-provided, never scanned) via
  `executor.SecretHits` and reports `SECRET EVIDENCE` rows (seq/task/
  field/hit-count, never the secret itself; a hit is a distinct secret
  LOCATION — overlapping pattern spans merge, so one
  `Authorization: Bearer <token>` line is ONE hit) so facts written before
  the pass shipped are findable (17-21 report #18; 20-58 f33).
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
- **Priority mechanics (ADR-0015)**: claim order = STORED priority +
  aging (`queue.PriorityAgingDaysPerPoint`=3d/pt, cap
  `queue.PriorityAgingMaxBonus`=10) — aging is scheduling, the stored
  value never mutates; constants live ONCE in the contract, both
  backends mirror them, conformance pins them. Markers (`— P[1-4]`) are
  stripped before the dedup hash. The repri sweep (startup + `tq
  reprioritize`) is O(repos × (parse + one filtered List)) — bounded by
  the working set, not the warehouse. `tq show` carries a priority
  provenance section (band, cached verdict, repri history).
- **prune-stale**: cancels PENDING tasks whose item is now `[x]` OR whose
  item text is gone from the file (done-and-deleted / reworded — harvest
  provenance via payload dedup key guards external tasks; `catchup:`
  prefixes stripped). agent-pool runs one sweep synchronously before any
  actor starts (`--prune-stale=false` to skip); running tasks are reported,
  never stopped.

## Conventions

- Table-driven tests with plain `testing`; sentinel errors in
  `internal/task/errors.go`, checked with `errors.Is`
- **Claims carry citations** (2026-09-12): DONE notes, close-outs, and
  verify-then-close annotations cite the gate run or commit SHA they rest
  on — an uncited claim is a hypothesis; a stale-DONE row is closed only
  with commit + gate evidence, never memory. The citation may only be
  WRITTEN after the cited gate's output is in hand — never cite a gate
  that is still running (the 2026-09-12 02-38 window claimed "real gate
  re-run green" for a background gate whose exit code had not been
  captured). Filter-scope/behavior claims
  (what a view, filter, or code path displays/restricts) additionally cite
  file:line or a pinning test name in reports and handoffs — a confident
  wrong scope claim burned the 2026-09-12 01-47 §d3 budget-display answer
  (source: docs/status/2026-09-12_01-47_per-project-ui-question.md)
- **Verify-window minimum battery**: a verify-only re-dispatch window must
  re-run the CHEAP gates itself at HEAD (check-doc-refs.sh + root build+vet
  under GOEXPERIMENT=jsonv2 — seconds each) and produce at least ONE fresh
  delta or state explicitly why none exists; expensive gates (race suite,
  nix) may be inherited only from a same-HEAD prior report (03-27 §e2 set
  the bar; the 03-39 window regressed by citing inherited gate runs in its
  stop verdict — promotion of the TODO row after a second confirming
  window, 2026-09-20)
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
- **Executor shared seams — never hand-roll a new copy**: payload parsing
  via `decodePayload[T]` (`internal/executor/payload.go`), outcome/sidecar
  recording via `recordRunOutcome` (`internal/executor/result.go`), display
  excerpts via `executor.Excerpt` (`internal/executor/excerpt.go`),
  repo+clean-tree preflight via `prepareRepo` and timeout shape via
  `payloadTimeout` (`internal/executor/preflight.go`). Copies collapsed on
  2026-09-23: Excerpt 3, recordRunOutcome 5, decodePayload 4, prepareRepo 4,
  payloadTimeout 5; art-dupl at `-t 4` catches recurrences. Dedup ruling
  2026-09-26 (owner): acceptances ≈ 0 — cross-BACKEND mirror clones are
  gated by `scripts/check-mirror-clones.sh` (advisory in ci-local until the
  companion extraction lands, then `MIRROR_CLONES_STRICT=1`; it counts 31
  sqlitev4↔postgresv4↔cqrsqlite groups today, extraction designed in
  docs/planning/2026-09-26_companion-extraction-design.md, TODO row filed).
  Residual same-file `-t 3` groups (mutex/defer/flag-parse prologs,
  `deriveUsage`+`recordRunOutcome` pairs, payload-type twins) are shared-
  seam CALL pairs, not duplication — do not abstract them into existence.
- Platform honesty: POSIX-only suites carry `//go:build unix`; CI runs the
  rest on windows-latest. Tests must be hermetic (nix checkPhase has no
  host tools — a test once assumed `crush` on PATH and broke the nix build)
- Generated `*_templ.go` and the minified `app.css` are COMMITTED (Nix
  builds vendor source without `templ generate`); after template edits run
  `templ generate` + `nix run .#webui-css`. Run the generate from the REPO
  ROOT, never from inside the module dir: the FileName paths embedded in
  `templ.Error` are CWD-relative, so a module-dir run rewrites EVERY line
  of both generated files to the short form (`layout.templ` vs the
  canonical `internal/webui/layout.templ`) — a huge noisy diff with zero
  content change (2026-09-21 usage-cards window). Since 2026-09-13 the
  pre-commit hook (install-pre-commit.sh) ALSO guards staged app.css
  (nix rebuild byte-equal when nix exists, line-count heuristic
  otherwise); the daemon still bypasses hooks, so ci-local remains the
  catcher — and the hook's checks are sequential now (the old
  `exec A && exec B` chain never reached the TODO gate) (build script scans the
  module-cache copies of templ-components AND go-health-dashboard — rerun on
  version bumps of either);
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
  (v1.17.0); tokens in `internal/webui/theme.css`; JetBrains Mono woff2
  subsets are SIL OFL 1.1 (© JetBrains). Design language refinements
  (2026-09-21 pass): two-voice typography — mono is the MACHINE voice (ids,
  counts, timestamps, commands), sans the HUMAN voice (project names, error
  prose); lowercase terminal labels everywhere BELOW the hull chrome (the
  topbar/nowband bezel keeps its engraved uppercase micro-labels); board
  cards are status-left-ruled queue chips, not SaaS cards; the nowband
  budget readout carries a CSP-safe quantized spend meter (`.tq-meter` —
  `style-src 'self'` forbids inline widths, so the server emits one of
  twenty 5%-step `.tq-meter-fill-N` classes; pinned by
  `TestBudgetMeterToneAndQuantization`). The fact feed is sticky-bottom
  (app.js keeps a pinned `.journal-scroll` pinned across swaps) and the
  newest fact line settles in with a reduced-motion-guarded 240ms fade.
- The `FuzzParseRepo` seed corpus is COMMITTED and grows via nightly
  campaigns; a fuzz crasher must be fixed, never committed
- `TODO_LIST.md` is machine-consumed: `- [ ]` checkboxes, one item per
  line, never tables; `— BLOCKED: <reason>` keeps an item out of the pool;
  unchecked items must be agent-executable (checked by
  `scripts/check-todo-list.sh` in ci-local)
- Status reports are indexed on creation (`check-status-index.sh` +
  pre-commit hook via `scripts/install-pre-commit.sh`); CHANGELOG is
  append-only; `check-features-roadmap.sh` guards shipped-vs-planned drift.
  Daemon-folded reports: the auto-commit daemon bypasses that hook, so a
  report riding into a `chore:` commit unindexed is the KNOWN hole (it
  silently lost reports twice, 2026-09-09) — the rule is the AMEND
  MANEUVER: amend the index row into the daemon commit when it hasn't
  shipped, otherwise a follow-up indexing commit immediately;
  `check-status-index.sh` in ci-local/CI is the catcher, not the preventer.
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
  green; ci.yml sets the env workflow-wide for the same reason, and
  fuzz.yml got the same env after its 2026-09-12 nightly died at setup
  while the script misreported the build failure as a fuzz crasher —
  nightly.sh now says CAMPAIGN SETUP FAILED when no crasher was written)
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

| Library component                                                                                                                                          | Status  | Where                                                                                                                                                             |
| ---------------------------------------------------------------------------------------------------------------------------------------------------------- | ------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `layout.Base`, `ThemeToggle`                                                                                                                               | adopted | `layout.templ`                                                                                                                                                    |
| `display.Card/Table/EmptyState`                                                                                                                            | adopted | `fragments.templ` (Card since 2026-09-16 only on the DETAIL page — the main-page table/chart wrappers moved to the custom `tq-panel`; Table/EmptyState unchanged) |
| `display.Badge/DefinitionList/Scrollback`                                                                                                                  | adopted | `fragments.templ`                                                                                                                                                 |
| `display.AreaChart`                                                                                                                                        | adopted | metrics row: fact-rate sparkline + completion histogram (`fragments.templ`)                                                                                       |
| `display.Button`                                                                                                                                           | adopted | filter bar (apply)                                                                                                                                                |
| `display.CopyButton`                                                                                                                                       | adopted | detail page: task-id copy, payload lede + raw payload, status-report path (2026-09-14 overhaul)                                                                   |
| `feedback.Alert`                                                                                                                                           | adopted | task detail (last error)                                                                                                                                          |
| `navigation.Pagination`                                                                                                                                    | adopted | task table pager (2026-09-15: numbered pages + ellipsis replace the hand-rolled prev/next)                                                                        |
| `display.ListNote`                                                                                                                                         | adopted | task table pager: "showing N of M" truncation note (2026-09-15)                                                                                                   |
| `icons.ArchiveBox/CircleStack/Filter/Inbox`                                                                                                                | adopted | empty-state + filter icons (`fragments.templ`)                                                                                                                    |
| status nowband (tq-seg), board columns/cards, filter inputs, page header/lamp, section hairlines, instrument panels (tq-topbar/tq-panel/tq-label/tq-fault) | custom  | `fragments.templ`/`layout.templ`/`theme.css` (StatCard retired for the nowband)                                                                                   |

Guarded by `TestAdoptionTableCoversTemplates` + `TestAdoptionTablePinsCustomRows`
(both directions of rot fail the suite).

Adopted 2026-09-14 (overhaul), Go-invoked so OUTSIDE the template-scoped
table: `display.RelativeTime` renders the detail definition list's
created/updated/completed rows (AutoRefresh + CSP nonce via
`relativeTimeComponent` in components.go). The fact feed keeps wall-clock
timestamps + the client `data-age` ticker by decision — a terminal tail
reads absolute, and per-line components would each re-render on swap.

Evaluated and REJECTED 2026-09-15 (`errorpage.WriteError`/`ErrorPage`,
separate errorpage MODULE): the library error pages render their OWN page
chrome (full standalone document), while every tq surface is an in-chrome
view of the themed, auth-gated, noindex dashboard shell (`layout.Base`) —
a library-chrome 404/500 would visually strand the operator mid-session.
The one full-page error that matters (task id does not resolve) already
renders the in-chrome styled 404 (`renderTaskNotFound`), and the remaining
`http.Error` sites are form-post/fragment contexts (cancel/rescue 409/500)
where a plain text + browser-back is honest. Adopting would also add the
errorpage module require (+ vendorHash dance) for four error lines. Do not
re-litigate without a chrome-less variant upstream.

Retired 2026-09-16 (instrument pass): `display.Eyebrow` — its rendered
treatment (ALL-CAPS, 0.18em tracked mono) is exactly the templated-chrome
tell, repeated on every section; replaced by the custom `tq-label`
convention (lowercase mono, 0.08em, gray-500/400), which also retunes the
legacy `tq-kicker`/`tq-settled-title`/`tq-payload-label` classes in
theme.css. Do not re-adopt without a design reversal. Same pass: the
main-page table/chart wrappers moved from `display.Card` to custom
`tq-panel` (seam-bordered, no shadow-box islands), the page header became
the always-dark `tq-topbar` hull bar, and dead>0 renders a `tq-fault`
action line under the nowband.

Evaluated and REJECTED 2026-09-13: `display.KanbanBoard` (library v1.15+)
does not fit the custom board — its value is the drag/keyboard move
exchange (inert on a read-only board per ADR-0003 Phase D), its count
badge shows the truncated `len(col.Cards)` instead of the true count,
there is no hook for the "+N older" escape-hatch link or the per-status
accent rule, and it renders `data-tc-kanban-column` instead of the
`data-status` attributes board_test.go pins. Do not re-litigate without
an API change in the library.

Evaluated 2026-09-13 (`PageProps.SEO` + `icons.Render`, 04-09 f22):
`SEO` is adopted for `NoIndex` ONLY (`dashboardProps` in
components.go emits robots noindex — the operator surface carries task
prompts, repo names, and failure evidence, and the security model
documents non-loopback `--auth-token` binds plus shareable `?token=`
URLs; noindex is one-line defense-in-depth against a crawler indexing a
tunneled/leaked URL). Canonical/hreflang/JSON-LD stay zero — meaningless
without a public crawler surface, and JSONLD is a verbatim Raw-injection
field. `icons.Render` is REJECTED: it is the extension point for
CONSUMER-supplied `CustomIcon` path data (brand logos, foreign icon
sets); the filter/empty-state icons are built-in typed `icons.Name`
constants already adopted via `display.EmptyStateProps.Icon`, and
routing them through Render would hand-copy library path data into this
repo, losing type safety and upstream path fixes. The adoption table is
template-call-scoped by the guard tests, so both verdicts live in this
prose, not the table.

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
- ⚠️ **vendorHash drift**: after go.mod/go.sum changes run
  `nix build .#checks.x86_64-linux.vendor-hash` — the fast gate realizes
  ONLY the go-modules FOD, so drift fails in seconds — and copy the
  failure's `got:` hash into flake.nix's `vendorHash` literal (the old
  `vendorHash = lib.fakeHash` dance is stale: nothing binds `lib` where
  vendorHash lives, and a full `nix build` wastes minutes reaching the
  same mismatch — 09-44 report §e1). NOTE
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
  ×1) are pinned to `1.27.1` matching the go.mod toolchain floor + the
  toolchain-alignment gate (the 1.26.7 pins went STALE when the go.mods
  moved to 1.27.1 on 2026-09-16..20: runners run GOTOOLCHAIN=local, so
  every go step — vet, govulncheck, windows, postgres, cqrs-lint — died
  with "go.mod requires go >= 1.27.1" and master stayed red 09-20..21
  until the 09-22 pin bump); a GOEXPERIMENT-only fix reproduces locally
  and still fails on runners — verify against the environment that
  failed, not just locally. The pin must equal the go.mod floor in BOTH
  directions: floor above pin kills every runner step (this incident),
  pin above floor re-floats the stable bug (the original one).
- ⚠️ **Never lower a module's `go` directive — zero-dep leaves have no
  floor** (2026-09-12 red master, run 34726154600): every go.mod must
  declare the root's exact version (`go 1.27.1` since the 2026-09-16
  sweep; `check-go-mods.sh` is the gate). The T38–T40 window aligned go.mods DOWN; `go mod tidy` reverted
  the dep-bearing modules but the stdlib-only leaves (`internal/task`,
  `internal/journal` — no require lines) have NO dependency floor, so the
  downgrade survived there, rode daemon commit 201041e, and failed the CI
  `go.mod health` step after push. Repro: `go mod tidy` on a leaf with
  `go 1.26` silently leaves it. Restore with
  `go mod edit -go=$(awk '$1 == "go" { print $2; exit }' go.mod)` in the
  drifted module, then run `./scripts/check-go-mods.sh` BEFORE the daemon
  sweeps the drift up. If a toolchain refuses the pinned version, fix the
  toolchain (GOTOOLCHAIN / devShell), not the directive.
- ⚠️ **Flakes only see git-tracked files**: `git add` new files before
  `nix build`.
- ⚠️ **templ LSP diagnostics are false positives** (phantom syntax errors
  against a green `go build`; the cache goes stale, not the sources). Trust
  the CLI, not the LSP, for webui/templ.
- ⚠️ **`tq serve` security model**: loopback-only and read-only by default;
  `--allow-writes` adds exactly two CSRF-guarded admin routes (cancel,
  rescue) with a failed-attempt lockout (3 bad CSRF tokens → 60s 429);
  the strikes map is bounded against rotating source IPs (1024-key cap:
  global idle sweep + least-recently-active eviction; live lockouts
  survive both);
  non-loopback binds (incl. `:port`, hostnames) refuse to start without
  `--auth-token` (constant-time bearer/`?token=`). Full matrix:
  SECURITY.md. Don't add write endpoints without the same treatment.
  `tq api` (internal/httpapi) carries the SAME hardening (2026-09-16,
  03-05 report f2/f3 — decision CLOSED, do not re-open): mandatory token
  on every bind, nosniff on every response, and a 3-strikes bearer-auth
  lockout (per client IP, all routes, 60s 429 + Retry-After, success
  resets, `authRateLimiter` mirrors webui's writeRateLimiter).
- ⚠️ **golangci-lint is advisory** (`continue-on-error`, ~890-finding
  baseline): never mass-"fix" the baseline; don't add new findings in
  functions you touch. Hard gates: vet + gofmt + tests. Baseline GROWTH is
  now a ci-local GATE (round-13 T5): `scripts/lint-baseline.sh --check`
  (wired after the advisory lint step) fails on any per-module/linter count
  above `.golangci-baseline.txt`. REGEN POISONING (2026-09-26): a regen run
  while a module's tree is BROKEN silently NARROWS the baseline — the
  cmd/tq lint run of the 01:41 readmodel regen recorded only a transient
  `typecheck: 1` and all 29 of its rows dropped; the next honest run
  reported them as new-class growth, so the gate sat RED on a poisoned
  baseline. Regen only on a green tree and grep the fresh baseline for
  `typecheck:` rows before trusting it (the heal + TODO row for a regen
  refusal guard are in TODO_LIST). Historic regen ledger: (2026-09-14
  fourth regen: 500 findings, 87
  module/linter rows — policy change: goconst/mnd/paralleltest
  POLICY-DISABLED (slice-triage round 2, rationale comment in
  .golangci.yml; testpackage already excluded via the _test.go rule);
  earlier regens 886/111 (gochecknoglobals excluded for the seven ADR-0016
  facade alias files, whose package-level re-exports ARE the
  facade pattern), 850/104, 887/114, 917/125 — the 917 bundled
  concurrent drift. The 2026-09-12 regen also added the path-scoped
  tagliatelle exclusion: machine-payload wire structs (executor payloads,
  queue fact evidence, harvest RepriChange) are snake_case BY CONTRACT —
  keys are quoted in agent prompts and persisted in journal facts; core
  domain types stay camelCase; 2026-09-15 fifth regen: 1089 findings, 145
  rows — the regen added cmd/tq to the lint+baseline loops (ADR-0017 module
  was never linted; +268 findings/31 rows via the devmod+devwork shims in
  `scripts/lib/cmd-tq-devmod.sh` — golangci-lint rejects -modfile in its env
  probes, so tooling lint runs under a derived go.work; non-cmd/tq counts
  byte-identical across the regen); 2026-09-16 surgical repair:
  executor mnd 29→31 / gochecknoglobals 4→5 / varnamelen 16→17 — drift
  from the secrets-in-logs redact.go (c7ca518) landing after the fifth
  regen — plus cmd/tq tagliatelle 9→10 / varnamelen 6→7 and root mnd
  44→45 from concurrent post-regen windows; the repairing window's own
  diff was lint-clean (`--new-from-rev HEAD~1`: zero findings));
  2026-09-16 sixth regen: 1101 findings, 140 rows — the health-adoption
  window fixed its OWN growth (root/errchkjson: writeHealthJSON's
  discarded Encode error, now checked) and absorbed concurrent drift
  (worker nestif from the 03-30 window, cmd/tq gosmopolitan from the
  12-54 64-file window); the window's webui `--new-from-rev 3c8e858`
  was zero findings after two wsl_v5 blank-line fixes);
  2026-09-18 seventh regen: 1113 findings, 134 rows — the
  session-forensics drift repaired IN CODE (cmdStats session-volume
  extraction under the cyclop cap, gitscan err113 sentinel + mnd
  constants + unnamed returns, webui sessionStats unnamed returns +
  renamed accumulator, prune-test prealloc); cmd/tq tagliatelle
  path-excluded for `cmd/tq/(main|top|journalaudit)\.go` (tq CLI wire
  payloads are snake_case BY CONTRACT, whole statsPayload family
  predates the linter and stats_test.go pins the exact keys); sqlite
  varnamelen corrected 32→33 to the CLEAN-cache truth (the committed 32
  was recorded on a warm cache — `golangci-lint cache clean` before
  generating or judging baseline rows, CI is always clean-cache)
  or on a NEW (module, linter) class;
  2026-09-23 eighth regen: 1410 findings, 168 rows — coverage fix, not
  drift: the ADR-0019 spike modules `internal/queue/sqlitev4` and
  `internal/queue/cqrsqlite` had ZERO baseline rows (the last regen
  predated them), so the gate was structurally blind to new findings
  there; clean-cache --check first CONFIRMED the blindness (gate failed
  on the new classes), then regen admitted them (sqlitev4: 152 findings/17
  rows, cqrsqlite: 122 findings/13 rows — dominated by paralleltest/
  wrapcheck test-noise classes consistent with the mirrored sqlite
  backend, no new triage policy); --check green at 1410 vs 1410 after
  or on a NEW (module, linter) class;
  2026-09-24 ninth regen: 1549 findings, 185 rows — the ADR-0019 S1
  postgres spike `internal/queue/postgresv4` admitted proactively at
  module birth (17 rows, paralleltest/wrapcheck/varnamelen test-noise
  profile matching the sqlite spike — this time --check had no blindness
  window because the regen ran before any gate did);
  2026-09-24 tenth regen: 1550 findings, 183 rows — the S1 replay tool
  package `internal/queue/sqlitev4/replay` (verbatim projection copy +
  projection-equality gate, the ADR-0019 data-migration tool) grew the
  existing sqlitev4 rows by its residual wrapcheck/paralleltest noise;
  everything cheap was fixed in code first (errcheck, goconst, cyclop,
  forbidigo, makezero, predeclared, mnd, godoclint all driven to zero in
  the new package);
  shrink is advisory-only — regenerate deliberately when a policy change
  owns it. Config resolution (verified 2026-09-12): the ROOT `.golangci.yml`
  is the only config — there are no per-module files, and golangci-lint
  ascends from the module cwd to find it, so sub-module runs
  (`cd internal/<mod> && GOWORK=off golangci-lint run ./...`) apply the root
  policy; never add a nested config without re-baselining every module.
  `*_templ.go` is
  lint-excluded (`templ fmt` owns `.templ`). wrapcheck + varnamelen were
  triaged to zero (2026-09-10, task 000001a089c3): wrapcheck ignores
  internal-package globs + stdlib idioms + tests; varnamelen ignores tests
  - `w`/`r`/`fs`/`db`/`id` idioms; remaining sites were renamed, not
    suppressed. That sweep's renames leaked into string literals twice
    (`q.Get("query")` deadened the webui filter, `task(store)` mangled
    `tq dlq --max-attempts` help; fixed c95adb8/d5ee08d): a variable rename
    must never change a string literal — before calling a rename done, grep
    the diff's quoted lines when the variable name equals a nearby param
    name, JSON tag or flag text. The advisory lint step loops every
    `internal/*` sub-module in ci.yml too (f32, parity with ci-local.sh).
- ⚠️ **gosec advisory baseline is all FP/by-design** (triaged 2026-09-10,
  v2.29.0, 48 findings over root + all sub-modules; advisory CI job, f21;
  round-13 T6 ENCODED the triage as excludes 2026-09-12: since 2026-09-19
  the ci.yml gosec job DERIVES pin + excludes from scripts/check-gosec.sh
  via sed (fail-closed on an empty derivation — a bump is one edit there)
  and `.golangci.yml` gosec.excludes is parity-GATED against the same
  GOSEC_EXCLUDES by `check-gosec.sh --self-test` (set-compare since
  2026-09-20; a triage change is one edit in the script — drift fails
  ci-local) — post-config scan = 0 findings on every module, so any
  future gosec finding is a NEW class needing a fresh triage note, and the
  gate-vs-advisory flip is owner ruling O5; the canonical one-command gate
  entry is `./scripts/check-gosec.sh` — pins the same v2.29.0 + the same
  exclude set, scans root `./...` plus every sub-module with `GOWORK=off`,
  and hard-fails any Files:0 silent skip AND a broken/zero-target OR
  short module enumeration (a failing or empty `scripts/for-each-module.sh`
  would otherwise narrow the gate to the root scan and exit 0; since
  2026-09-20 the enumeration must also contain the five pinned canary
  sub-modules internal/{task,journal,queue,executor,worker} — a renamed
  sub-module dropping out of `find` leaves survivors that scan and exit 0,
  invisible to both older guards; rename/removal must consciously update
  the canary list in scripts/check-gosec.sh); since 2026-09-20 the
  --self-test mode ALSO byte-guards ci.yml's sed derivation (the
  workflow's exact lines are lifted from ci.yml, run against the shipped
  script, and compared with the pinned constants — the derivation runs
  only on runners inside a continue-on-error job, so drift was otherwise
  advisory-only in CI)):
  G204/G702 (exec with variable) — executors and bootstrap RUN commands
  from task payloads/`.tq-verify`/user config as their core feature, argv
  is never shell-interpolated; the post-config 2026-09-12 re-scan confirms
  the NEWER exec(git) sites stay in this same triaged class at ZERO
  findings (v2.29.0, excludes as encoded): the session-forensics scanner
  (`internal/session` `pgrep -f`, executor `git -C` in gitscan.go/outcome.go/
  agent.go, and the cmd/tq session call sites — doctor tag checks,
  bootstrap add/commit, main.go `git log` footer scan) all run fixed
  subcommands with repo/user-supplied operands only, never shell-
  interpolated; G703/G304 (path taint) — a local CLI
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
- ⚠️ **The session (mvdan) shell has NO usable `PIPESTATUS`**: it expands
  EMPTY, and `$?` after a pipeline reports the LAST stage's status, so
  `gate | filter; echo $?` reports a red gate as green. Confessed in report
  prose by the 02-38 window (§d) and RE-HIT verbatim by the 2026-09-19 04-32
  verify window's first battery command — codified only at the second hit.
  Gate invocations from agent sessions must redirect to a file and capture
  `$?` directly, counts grepped from the file
  (`cmd >/tmp/x.log 2>&1; rc=$?; grep -c ... /tmp/x.log`). ci-local and CI
  run stock bash where PIPESTATUS works — the hazard is agent-session
  CLAIMS only.
- ⚠️ **Root-module builds auto-use `vendor/` — stale vendored internals
  poison root builds** (2026-09-17 questions arc, d2): after changing ANY
  internal/ module, root `go build ./...` can fail with misleading
  "undefined: queue.AnswerRecord"-style cascades while per-module
  `GOWORK=off` builds stay green — the root module resolves
  `github.com/larsartmann/go-taskqueue/internal/...` from the committed
  `vendor/` tree, not the workspace. Fix: `go mod vendor` before root
  builds. The three-layer dep-graph lesson: facade requires+replaces →
  root vendor refresh → per-module GOWORK=off builds.
- ⚠️ **golangci-lint's cache is not invalidated by Go toolchain flips**
  (2026-09-17 lint-fix arc): after the 19-go.mod 1.27.1 bump, a WARM cache
  serves pre-bump analysis results while changed files re-analyze under
  1.27.1 semantics — `./scripts/lint-baseline.sh --check` then flip-flops
  between near-green and broad "growth" on unchanged files. Trust the
  CLEAN-cache reading: `golangci-lint cache clean` before judging the
  gate (CI is always clean-cache, so the clean reading is the CI truth).
  Also: golangci's embedded golines diverges from a standalone
  `/home/lars/go/bin/golines` (different version) — always finish
  formatting with `golangci-lint fmt` for byte-parity with the linter.
- ⚠️ **Session-start ritual**: run `scripts/session-start.sh [<task-id>…]` (covers all steps below; 000001a0bd48), then: run `git log --oneline -5` over the WHOLE
  repo (not just `-- internal` — concurrent work lands in cmd/, scripts/,
  and docs/ too) plus `git status` and `git stash list` before editing —
  concurrent agents land real changes mid-flight (worker's go-retry
  require, flake vendorHash fixes, AGENTS.md corrections have all arrived
  mid-session). Build on them; never revert.
  **Turn-1 additions (codified 2026-09-15 after Nth-recurrence misses in
  every closeout since 09-12):** (1) grep prior reports for the SAME task
  ID before doing anything (`rg -l <task-id> docs/status/`) — repeat
  dispatches are routine, and the fastest correct response to a DONE row
  is verify → cite → stop; (2) check for and read `CONTRIBUTING.md` (and
  `CLAUDE.md` if present) at turn 1 — the task contract requires it,
  `CONTRIBUTING.md` exists in this repo, and skipping it is a documented
  recurring miss (00-52 d4, 02-17 d3).
- ⚠️ **Scripted history edits under the auto-commit daemon** (2026-09-10
  reword incident): inline-quoted `GIT_EDITOR`/`GIT_SEQUENCE_EDITOR` values
  get mangled through the mvdan/sh → git handoff and can SILENTLY no-op —
  use script files, and dry-run the edit against `git log -1 --format=%B`
  first; the daemon may commit mid-rebase (re-count the `Rebasing (N/N)`
  replay); a reword forks the lineage from any pushed twin and from release
  tags — tags are immutable, so the fork persists; verify by path
  (`git log -1 -- <path>`), not by `--grep` phrase. Full playbook:
  docs/status/archived/2026-09-10_07-49_task-000001a089c3919ce4a6807036db62c5aee7.md §e.
- ⚠️ **History-rewrite policy**: NEVER reword/amend/rebase any commit that
  has been PUSHED (including daemon commits already on origin/master) —
  wrong Task-Queue-ID footers, typos, and bad messages get a follow-up
  correcting commit or a queue-side note, never a rewrite; tags are
  immutable, so a rewrite forks the lineage permanently (the 2026-09-10
  incident forked v0.2.0 + seven sub-tags from the pre-reword side; that
  specific fork was later healed by merge-append, but only because someone
  noticed — treat every fork as permanent until proven otherwise). Exception: owner-approved only, for a defect that a follow-up
  commit cannot fix (e.g. a footer poisoning the queue↔git
  cross-reference beyond repair), scripted per the daemon rules above. If
  approved and executed, RECORD the fork in the same session: (a) the
  old + new SHAs and the fork point in a status report, (b) a note that
  release tags and remote twins now descend from the abandoned side, (c)
  a `tq doctor` / release-gate check for `git tag --no-merged master`
  before the next release, and (d) no doc may cite the dead SHAs
  (CHANGELOG cites no SHAs at all for this reason).
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
- **templ-components consumer sweep rails** (2026-09-14, ADR-0018): the
  fan-out lives HERE (`scripts/sweeps/fanout-libdive.sh` + cohort TSV +
  version-pinned prompt template; `--self-test` pins the who-uses tree
  parse; `--db` hard-required — never mints into an inherited TQ_DB). pdg
  is PROPRIETARY with no sub-module tags — its SDK never enters this repo;
  the pdg-resident bridge option stays owner-gated. Waves serialize against
  the shared Z.ai account (429 runbook: plan §8.2).
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
- ⚠️ **The minted verify gate is structurally red on dev hosts with
  `vendor/`** (2026-09-21, §f4 attempt-3 discovery): the bootstrap
  auto-detect verify — gofmt tail `test -z "$(gofmt -l .)"`, verbatim in
  the repo `.tq-verify` — walks the GITIGNORED `vendor/` tree (root
  builds need it since the 2026-09-17 vendor arc, .gitignore:63), so the
  gate dies at its LAST stage with rc=1 while every earlier stage is
  green (44 vendor/*.go flagged, all third-party; `git ls-files '*.go'`
  is gofmt-clean; every package ok incl. e2e). Deterministic
  dead-letter machine since vendor/ materialized: 5 go-taskqueue tasks
  dead 3/3 on this gate since 09-20 (000001a0bd9a…, 000001a0c124… tails
  confirmed all-ok). Fix is OWNER-ONLY (agents never edit their own
  gate): scope the gofmt stage to tracked files
  (`gofmt -l $(git ls-files '*.go')`) in BOTH `.tq-verify` and the mint
  template; CI is unaffected (runners have no vendor/). A task.verify
  failure whose tail shows all-ok packages is THIS bug, not a work
  defect. Separately, internal/e2e under -race is load-marginal vs the
  180s stage cap (240s kill at 02:45, 181.6s ok at 03:05) — tracked in
  its own TODO row.
- ⚠️ **Crush client/server mode stays OFF for pool agents until a
  per-repo experiment passes** (`CRUSH_CLIENT_SERVER` unset everywhere,
  verified 2026-09-14): first-wins `--yolo` is a non-issue under the
  all-yolo fleet (owner ruling 2026-09-14 — headless yolo is already
  allowlist-restricted), but the real blockers remain: one shared server
  collapses the per-project `.crush` data-dir isolation (needs per-repo
  servers), and short-lived `crush run -m` workspace-joiner behavior is
  unverified. Analysis + experiment design:
  `~/.config/crush/docs/research/2026-09-14_crush-client-server-mode.md`.
- ⚠️ **GitHub Push Protection false-positives on shape-valid FAKE token
  fixtures** (2026-09-16: a push of 68 commits blocked over the
  `redact_test.go` Slack fixture, the only fixture whose shape matched
  GitHub's pattern — HYPOTHESIS, unverified against GitHub's unpublished
  regexes: the AWS `…EXAMPLE` and checksummed-`ghp_` fixtures pass
  because they fail the stricter real patterns). The scanner matches
  token SHAPES in source, not realness. Fix class: COMPOSE the fixture
  literal (`"xox" + "b-…"`) so no scanner can match the source while the
  runtime value still exercises the redactor byte-identically (redact.go's
  own regex literals are safe — bracketed char classes never match token
  shapes). Push protection scans EVERY commit in the push, so a follow-up
  commit is useless once the fixture sits in unpushed history — the fix is
  a scripted `git filter-branch --tree-filter` over
  `origin/master..master` (script file per the daemon playbook; unpushed
  range only, never PUSHED commits), verified by: worktree test of the
  exact transformation first, then count + author/date/subject metadata +
  net-diff equality before vs after, pickaxe zero over the token string,
  and `refs/original` kept until the push lands.

## Relation to other projects

Semantics proven in go-cqrs-lite (facts/journal) and PapDashboard (worker
pools over durable queues). PapDashboard is a runtime bridge only;
go-cqrs-lite is now also a CODE dependency at exactly one seam:
`internal/journal/cqrs` (ADR-0014) exposes the fact journal as a
go-cqrs-lite `SeekableJournal` (read-only; seq-encoded ULIDs; the store
invariants are untouched). PROPRIETARY license — owner-authorized
2026-09-12; do not extend the surface (no `event.Store` write path) and
do not import it below the root module without revisiting ADR-0014.
`.cqrs-lint.json` (2026-09-13) pins that intent for `cqrs-lint`
(read-only, library-framework) and documents the triaged false positives —
verified against event/v4 v4.11.0 source: `event.NewEvent` is NOT
deprecated, `buildEvent` already defaults schemaVersion to 1, and V006's
"mixed pins" are each module's latest tag (the library versions its
modules independently). Real defect the 2026-09-13 lint pass fixed: the
adapter's events carried an EMPTY encoding stamp, so every downstream
`event.DecodePayloadAuto` failed (`codec.ForEncoding("")` errors);
`factEvent` now builds via `event.New` + `WithCodec(JSONCodec)` (pinned
by `TestPayloadDecodesThroughLibraryAPI`).

**go-cqrs-lite IS the platform — ADOPTION RE-OPENED AND RULED 2026-09-22
(ADR-0019)**: the owner restated the founding intent ("the whole idea of
this project was that it uses go-cqrs-lite system/ + metaengine/") and
the 2026-09-13 "NOT adopted" verdict is OVERRULED — its premises rotted:
the upstream `queue/` family SHIPPED and is pushed (`queue/v4.0.0`,
`queue/{sqlite,postgres,mysql}/v4.0.0`, `claiming/v4.0.0`, verified on
origin 2026-09-22), its `Store[T]` contract is transcribed from THIS
repo's contract (queue/README names tq the spec donor; lease claims +
crash reclaim, dedup enqueue, retries→DLQ, RescueDead/DismissDead,
MarkOrphaned, cooperative cancels, DAG-dep claim gating, bounded priority
aging, same-tx facts, watermarks — ONE shared conformance suite), and
token-fenced finalizes (upstream ADR-0134) supersede tq's owner-string
finalizes. The upstream v5 direction (ADR-0123) makes `metaengine` Store

- `system` composition root the blessed surface (v1 read-model tiers and
  `stack/` presets die in v5 — do not adopt them now). Staged plan in
  `docs/adr/0019-go-cqrs-lite-platform-adoption.md`, tracked in TODO_LIST
  ("go-cqrs-lite platform adoption"): S1 swap the hand-rolled engines for
  thin drivers over `queue/sqlite|postgres/v4` (tq extras — questions/
  RecordAnswer, PriorityScores, CountFacts/FactsSince/LastFacts,
  ProjectCounts — as same-DB companion tables unless upstream grows them;
  since 2026-09-26 those extras' single home is the scaffolded
  `internal/queue/companion` module — the mirrored adapters still own the
  code until the extraction window lands, see
  docs/planning/2026-09-26_companion-extraction-design.md),
  S2 unify the journal on `facts.Fact` (open FactType; tq-specific fact
  types stay tq constants), S3 read models on metaengine
  (`Watcher`/`ServeSSE` replacing the hand tailer fan-out), S4 composition
  via `system/` DomainConfig + DELETE the mirrored backends (the 12
  art-dupl mirror clone groups as of 2026-09-23 — 31 by the 2026-09-26
  `-t 3` recount, advisory-gated by scripts/check-mirror-clones.sh — die
  there or into companion). Facts-first replay is the
  migration story (projection-equality verify; dogfood cutover owner-run).
  The historical module-by-module assessment (storage/ = per-stream event
  store; metaengine/ = cost-based read side; system/ = composition root;
  scheduling/ `ClaimingTimerStore` = the extracted claim core) remains
  accurate as DESCRIPTION — it is no longer a verdict. tq is upstream's
  named first consumer; upstream's open owner-gate (dep-validation
  ratification M4 §f1) does not block S1 — Reply A behavior assumed.

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

**go-health-dashboard** (`~/projects/go-health-dashboard`, sibling library,
MIT): ADOPTED 2026-09-16 (owner ruling "just add it", hours after a
NOT-adopted assessment whose blockers were then engineered away one by one).
`tq serve` mounts it at `/health` (+ `/health/sse`, `/health/datastar.js`,
`/health/favicon.svg`, JSON probes `/healthz` `/readyz` `/startupz`). The
adapter (`internal/webui/health.go`, `queueProber`) implements the
consumer-side `dashboard.Prober` over `queue.Store` — the store-backed
`tq doctor` subset (database via StatusCounts, worker heartbeats via
CountFacts(Heartbeat, 10m window), expired-lease stuck running, DLQ depth)
on a 5s evaluation cache that doubles as the SSE push cadence; NO samber/do
(any DI stays out — the Prober interface is the whole seam). Integration
rails, each a considered tradeoff: ALL health routes sit INSIDE the
existing withTokenAuth gate — the library's kubelet probes-bypass-auth
default was rejected on the ADR-0008 "no unauthenticated oracle" ruling;
the /health page + SSE run `dashboard.RecommendedCSP` (the Datastar SDK
needs 'unsafe-eval') as a per-route override INSIDE withSecurityHeaders —
every other route keeps nonce'd `default-src 'none'` (pinned by
`TestHealthTaskDashboardCSPUnchanged`); the SDK ships same-origin
(WithEmbeddedDatastarSDK + self-served go-datastar/static bytes, never the
jsdelivr CDN the library falls back to without it); the stylesheet is tq's
own `/static/app.css` (WithCSSPath — without it the library falls back to
the Tailwind Play CDN) and `build-webui-css.sh` scans the library's
module-cache copy for its tailwind classes (rerun on version bumps, same
as templ-components). cmd/tq/go.mod hand-pins the new modules as
indirects (its FOD downloads per the committed replace-free go.mod,
ADR-0017 — plain `go mod tidy` there fails on the known cmd/tq ambiguity,
and a stray hand-edit race with a concurrent tidy self-resolved). Gates:
`TestRoutesAreReadOnly` walks the health table, `internal/webui/health_test.go`
pins probes/CSP/auth/store-failure 503s, the webui smoke asserts the live
page + probes + SDK bundle. Versions: go-health-dashboard v0.8.1 — the
PROXY is ahead of the local v0.7.0 checkout (there `health.Check` still
carried `Since`; v0.8.1 is `{status, error}` only), go-health v0.1.3,
go-datastar(+static) v0.5.0, samber/do v2.1.0 + go-type-to-string (both
indirect only).

**go-nix-helpers** (`~/projects/go-nix-helpers`, flake input): the flake's
only build-time dependency — `flakeModules.go-standard` (flake.nix:24)
provides the whole Go flake scaffolding (`go-standard.*` option surface,
packages/apps/devShell/treefmt; only `apps.test` is `mkForce`-overridden for
the multi-module loop, flake.nix:318). Consumed as
`github:LarsArtmann/go-nix-helpers/master` (HTTPS for keyless CI runners)
and pinned in flake.lock — the local checkout is NEVER a build input;
local-pin equality is coincidental freshness, not consumption (verified
2026-09-13: lock rev 16c3184 == local HEAD == origin/master; no registry
override exists). ~40 sibling flakes consume it the same way. A local edit
reaches this build only after push + `nix flake update go-nix-helpers` (a
one-off `--override-input go-nix-helpers <path>` is the testing escape
hatch; recipe not yet documented).
