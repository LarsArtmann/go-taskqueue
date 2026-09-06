# Status Report — Docs-Health Audit: Six Living Documents

**Date:** 2026-09-06 16:19 CEST
**Session scope:** Full docs-health AUDIT (BUILD + HARVEST + VERIFY) over
README.md, AGENTS.md, FEATURES.md, TODO_LIST.md, ROADMAP.md, CHANGELOG.md —
plus whatever the audit surfaced in code. Session start (as `docs-health` run):
2026-09-06 ~15:50; includes the earlier same-conversation turn that fixed the
broken test build and README fact-type drift.
**Co-development context:** Parallel agent sessions worked this repo
simultaneously the entire time (agent pool, bridges, dedup keys, flake.nix,
auto-commit daemon). Everything below is verified against the repo state AFTER
their latest commits (HEAD `f9d549e`). Zero of their work reverted.
**Verification state at report time:** `go vet` ✅, `go build ./...` ✅,
`go test ./... -count=1 -race -timeout 120s` ✅ (all 8 packages), `gofmt` ✅,
`nix build` ✅ (after vendorHash fix), `nix flake check` ✅,
`tq harvest --repos . --dry-run` parses TODO_LIST.md ✅ (19/19 items).
Working tree clean (auto-commit daemon).

---

## a) FULLY DONE

1. **Docs-health AUDIT executed end-to-end against the CURRENT repo.** All
   source files, tests, configs, and docs viewed — including the parallel
   sessions' new code (`internal/harvest`, `internal/bridge/{cqa,papdashboard}`,
   `internal/executor/agent.go`, process-group files, flake.nix, grown CLI).
   The audit initially ran on a stale inventory and was re-based mid-run when
   `go test` revealed packages that did not exist at session start.

2. **FEATURES.md built from code (was missing entirely).** Honest inventory
   by domain area (queue core, executors, agent pool, bridges, CLI, tooling,
   planned) with the 4-status vocabulary and evidence per row. Deliberately
   NOT rounded up: CQA bridge is `PARTIALLY_FUNCTIONAL` (httptest-tested only,
   never verified against a live CQA API); agent executor notes its real gaps
   (verify auto-detect = Go/npm only; no Windows process-group kill).

3. **TODO_LIST.md rebuilt as a machine-consumable contract.** Documented the
   `- [ ]` checkbox format as a hard requirement (`internal/harvest` parses
   it — tables would silently break the agent pool's food source). All 10
   pre-existing items preserved and verified still open; 9 unharvested items
   from the 15:45 status report's top-20 block added (error classes, long-task
   regression test, ADR-0002, cost budget, release prep, autonomy
   false-positive, multi-repo smoke, systemd/--daemon, verify auto-detect);
   ranked High / Medium / Lower impact.

4. **ROADMAP.md rebuilt.** Milestones corrected (v0.1.0 checklist now reflects
   shipped reality, release work pointed at TODO_LIST), the prior report's
   items 21–50 routed in as raw ideas (~20 new: cross-repo DAG, --once mode,
   PR-mode, worktree isolation, session chains, output sidecar, Prometheus,
   parser fuzzing, SECURITY.md, chaos test, examples corpus…), plus an
   "Open questions (owner decisions)" section preserving the three questions
   only the owner can answer.

5. **AGENTS.md improved in place (7.6 KB, within the 5–15 KB budget).**
   - Fixed the ghost `ADR-0002` reference (file never existed) → ADR-0001 +
     `docs/planning/` pointer; writing ADR-0002 became a TODO_LIST task.
   - Fixed content-ownership violations: feature-status/marketing language
     ("shipped, E2E-verified 2026-09-06") moved to FEATURES/CHANGELOG; the
     enduring PapDashboard *contract* (flags, env, idempotency, watermark
     semantics) stayed.
   - Added missing enduring invariants: single serialized SQLite writer
     (`MaxOpenConns(1)` is why claim atomicity works — never add a pool),
     task-execution context survives pool shutdown (guards the fixed
     drain-context bug), Go 1.26 idioms are deliberate (`errors.AsType`,
     `strings.SplitSeq`), TODO_LIST checkbox format is a machine contract,
     golangci-lint is not a CI gate, agent argv changes must update
     `TestAgentExecutorArgvContract`.

6. **README.md corrected against code.** Distribution paragraph no longer
   markets the PapDashboard bridge as future work (it ships today via
   `tq worker --alert-url`; CQA via `--cqa-url`); Concepts now lists the
   `agent` executor; the "register executors in Go" guidance carries the
   honest caveat that packages are `internal/` (CLI is the public surface);
   verify-gate line matches code (`-count=1`); Development section lists the
   real CI gates and the doc map. (Earlier same-session turn: fixed the
   fact-type list drift — code has `heartbeat`/`released`, README claimed
   nonexistent `rescued`/`lease-lost`.)

7. **CHANGELOG.md repaired.** Removed the phantom `## [0.1.0] - 2026-01-01`
   entry — no git tag exists, the date was wrong, and the project
   self-describes as pre-v0.1.0; the section documented a release that never
   happened (never-published draft, so no release history was destroyed; a
   real entry will be written at actual tag time). Dropped the meaningless
   "Initial project structure" bullet from `[Unreleased]`. All remaining
   entries verified true against code, including both yolo/drain bug fixes.

8. **`nix build` breakage found and fixed.** The flake's `vendorHash` had
   drifted after the go.sum bump (modernc.org/libc v1.75.7) — every
   `nix build` failed. Ran the fakeHash dance (`fakeHash` → build → capture
   `got:` → re-pin to `sha256-H2J6GJy6BTngW7A6qaYHVEHxA6wLGoquDa+kW+EXmjs=`).
   `nix build` now produces the `tq` binary; `nix flake check` passes
   including the `checks.vendor-hash` drift gate. Logged in CHANGELOG Fixed;
   the matching TODO item deleted (done items never stay in TODO_LIST).

9. **Test-build regression fixed (earlier turn, same conversation).** Commit
   `918c3ea`'s automated pass had broken compilation of `internal/queue` and
   `internal/worker` test files (`_ = t.Cleanup(...)` on a no-value function;
   single-value Enqueue assignments). Fixed mechanically; full suite green.

10. **Dogfood proof of the doc set:** `tq harvest --repos . --dry-run`
    parses the rebuilt TODO_LIST.md exactly as the agent pool will — 19 open
    items found, headings tracked, pacing/dedup behaving. The documentation
    is not just readable by humans; it is queue-ready.

11. **All quality gates green at report time:** vet, build, tests with
    `-race` (8 packages), gofmt, nix build, nix flake check.

## b) PARTIALLY DONE

1. **PapDashboard bridge verification is inherited, not independent.**
   FEATURES.md marks it `FULLY_FUNCTIONAL` based on the prior session's
   E2E claim plus this session's green httptest suite
   (`TestDeadLetterBecomesAlert`, `TestServerErrorRetriesWithSameIdempotencyKey`,
   …). I did not run a live PapDashboard instance. Remaining: an owned,
   re-runnable E2E verification script. Effort: M.

2. **CQA bridge left honestly `PARTIALLY_FUNCTIONAL`.** Unit tests fake the
   API with httptest; nobody has verified the real CQA contract
   (`/api/v1/projects`, `/scans`, `/issues?fixable_only=true`). Remaining:
   live verification, then upgrade the status. Blocker: need a live CQA
   instance + API certainty (question in section g). Effort: S once access
   exists.

3. **Markdown formatting not machine-verified.** `dprint.json` declares the
   markdown plugin, but `dprint` is not installed in this environment (and
   not in the flake devShell, which ships git/gotools/gofumpt only). The
   rewritten docs follow the existing style manually; no formatter has
   blessed them. Remaining: add dprint to the devShell, run it once over the
   six docs. Effort: S.

4. **AGENTS.md improvement stopped at enduring-context scope.** Package-level
   doc surfaces (`doc.go`, godoc examples — plan items P22/F37) and a domain
   glossary were considered and deliberately routed to section (c): they are
   real gaps but belong in their own files, not squeezed into AGENTS.md.

## c) NOT STARTED

1. **`docs/DOMAIN_LANGUAGE.md`** — the vocabulary is rich and load-bearing
   (task, fact, claim, lease, release, dead-letter, rescue, harvest, dedup
   key, item, tick, verify gate) but undocumented. Not started because the
   six core docs came first; glossary next. Priority: medium-high — harvested
   agent prompts literally instruct workers to "read AGENTS.md", which has no
   glossary.

2. **ADR-0002 (agent-pool architecture).** Autonomy/trust model, pacing vs
   exclusivity, drain-context semantics. Now a TODO_LIST High Impact item;
   no file written.

3. **golangci-lint policy.** CONTRIBUTING tells contributors to run it; CI
   runs only vet+gofmt+tests; the baseline carries errcheck findings on
   idiomatic `defer x.Close()` lines. Either gate it (and configure
   exclusions) or stop advertising it. Needs an owner decision (section g).

4. **dprint in the dev shell.** Declared in `dprint.json`, absent from
   `flake.nix` devShell packages. Not started; trivial.

5. **v0.1.0 release.** No tag exists; README's `go install …@latest`
   instruction cannot be honored until one does. Release prep (history-squash
   decision, tag, GitHub release, pkg.go.dev surface, `doc.go` files) is
   TODO_LIST High Impact; zero of it started.

## d) TOTALLY FUCKED UP!

Nothing is fucked up NOW (tree clean, every gate green, docs parse). Four
things in this session genuinely went wrong and were caught only by luck or
late verification:

1. **I stopped one step short of my own declared goal in the first turn.**
   I wrote "fixing first, then writing AGENTS.md," fixed the tests and one
   README line — and then never wrote AGENTS.md in that turn. The declared
   deliverable was left undone until the user re-prompted. Root cause: I let
   a discovered side-fix (broken test build) consume the turn's momentum and
   did not re-check my own checklist before yielding.

2. **The audit's first hours ran on a stale mental model.** I inventoried the
   repo, then parallel sessions added ~3,000 lines (a whole package, two
   bridges, a new executor, flake.nix). I only noticed when `go test` printed
   `internal/harvest` — a package that did not exist when I started. I
   re-inventoried and re-verified everything (correct outcome), but I should
   have re-run `git log` + `find` the moment any file's content contradicted
   my earlier read, not after.

3. **I printed a lying exit code.** My first `nix build` background run
   piped through `tail` without `pipefail` and echoed "NIX_EXIT: 0" while the
   build had FAILED — the vendorHash error was only visible in the log tail,
   which I happened to read. This is precisely the pipeline-masking trap:
   filters ate the failure and the banner said green. Had I trusted the
   banner, FEATURES.md would now claim a green Nix gate over a broken one.

4. **I understated a live breakage as a "gap" — then found the truth by
   running the gate.** FEATURES.md's first draft called the Nix flake
   "PARTIALLY_FUNCTIONAL … not re-run recently" while in fact every build was
   failing. VERIFY's own rule is "run the canonical command, do not
   substitute" — I wrote the status row BEFORE running `nix build`. The
   correct order is gates first, status claims second.

5. **Fragile write choreography against a live daemon.** Two write attempts
   on TODO_LIST.md bounced off read-tracking (I had `cat`ed via bash instead
   of using `view`), while the auto-commit daemon re-committed the file
   between my read and write. No content was lost — the daemon's changes were
   mtime-only — but with different timing that pattern clobbers a parallel
   session's work. Luck, not process, again.

## e) WHAT WE SHOULD IMPROVE!

1. **Gates before claims.** Any doc statement of the form "X is
   FULLY_FUNCTIONAL / command Y works" must be preceded by actually running
   X/Y in the same session. This session's only wrong status row (Nix) came
   from inverting that order. Concrete rule: run `nix build` +
   `nix flake check` before any FEATURES/AGENTS edit that mentions them.

2. **`set -o pipefail` unconditionally in verification compounds.** The
   masked-exit incident (d.3) is the second time in this repo's recent
   history a filter turned red green. A shell alias/habit is cheaper than
   another caught-by-reading-the-log save.

3. **Re-inventory at the start of every multi-file write session** (one
   `git log --oneline -5` + `git status` + `find`), and again whenever any
   file contradicts memory. With 5+ concurrent agents in this repo, an
   hour-old inventory is not an inventory.

4. **Use `view`, not `cat`, for any file I intend to write.** The write tool's
   read-tracking exists to prevent clobbering parallel work; bypassing it via
   bash produced two rejected writes and a real (if benign) race with the
   daemon.

5. **Docs-drift guards in CI.** The audit hand-fixed drift (README fact
   types, phantom release, ghost ADR) that cheap CI checks would catch
   forever after: (a) `tq harvest --repos . --dry-run` must parse TODO_LIST
   (protects the pool's food source), (b) every path referenced in
   AGENTS.md/README must exist, (c) `nix build` in CI (vendorHash drift
   reached master unnoticed).

6. **golangci-lint: pick a lane.** CONTRIBUTING recommends a linter CI
   doesn't run, whose baseline findings (errcheck on idiomatic deferred
   Close) invite agents to "fix" working code. Either gate it with a config
   that encodes the accepted idioms, or remove it from CONTRIBUTING.

7. **Stop trusting point-in-time claims, even in fresh docs.** The prior
   session's report was 34 minutes old and already partially stale (it
   predates the flake's vendorHash drift). Re-verify before encoding any
   external claim — done this session, worth institutionalizing.

## f) Up to 50 things we should get done next

Items 1–19 are this session's fresh findings; items marked ★ are already
TODO_LIST.md-grade (harvested into TODO_LIST.md this session — listed here
for one-view completeness; do NOT re-add); ROADMAP-fuel items are marked ◇.
Impact: Critical/High/Medium/Low. Effort: S <30min, M 30min–2h, L >2h.

| #  | Task                                                                                              | Impact   | Effort | Category      |
| -- | ------------------------------------------------------------------------------------------------- | -------- | ------ | ------------- |
| 1  | Verify CQA bridge against a live CQA API; fix contract drift; upgrade FEATURES status              | High     | S      | Quality       |
| 2  | Re-runnable PapDashboard E2E verification script (owned by the repo, not a session memory)         | Medium   | M      | Quality       |
| 3  | CI docs-drift guard: `tq harvest --repos . --dry-run` must parse TODO_LIST.md                      | High     | S      | Quality       |
| 4  | CI ghost-reference guard: every path cited in AGENTS.md/README/FEATURES must exist                 | Medium   | S      | Quality       |
| 5  | Add `nix build` (+ flake check) to CI so vendorHash drift fails on PR, not on next audit            | High     | S      | Quality       |
| 6  | Add dprint to flake devShell; run it once over all six living docs                                 | Low      | S      | Cleanup       |
| 7  | golangci-lint: either CI-gate with errcheck exclusions for idiomatic deferred Close, or drop from CONTRIBUTING | Medium | S   | Cleanup       |
| 8  | Write `docs/DOMAIN_LANGUAGE.md` (task, fact, claim, lease, release, DLQ, rescue, harvest, dedup key, tick, verify gate) | Medium | M | Documentation |
| 9  | `doc.go` per package + godoc examples before the module goes public (plan F37)                      | Medium   | M      | Documentation |
| 10 | FEATURES.md maintenance note: statuses must be re-verified after parallel sessions touch core packages | Low    | S      | Documentation |
| 11 | ★ Permanent-vs-transient error classes (dirty tree / missing autonomy config / unknown flags dead-letter after one attempt) | High | M | Feature |
| 12 | ★ Store-level per-project claim exclusivity (`WithProjectExclusivity`)                              | High     | L      | Feature       |
| 13 | ★ Long-task regression test: task claimed after minutes of pool uptime completes                    | High     | S      | Quality       |
| 14 | ★ ADR-0002: agent-pool architecture, autonomy/trust model, drain semantics                          | High     | M      | Documentation |
| 15 | ★ Daily/rolling cost budget per repo and global                                                     | High     | M      | Feature       |
| 16 | ★ v0.1.0 release prep: history-squash decision, tag, GitHub release, pkg.go.dev                     | High     | M      | Release       |
| 17 | ★ `tq agent-pool --model` pass-through into AgentPayload                                            | Medium   | S      | Feature       |
| 18 | ★ `requireRepoAutonomy` false-positive fix (user-global crush permissions)                          | Medium   | S      | Bug           |
| 19 | ★ Multi-repo live smoke: ≥3 repos, 2 pools, one DB — dedup + pacing under contention                | Medium   | M      | Quality       |
| 20 | ★ systemd user unit or `tq agent-pool --daemon` for always-on operation                             | Medium   | M      | Feature       |
| 21 | ★ Verify auto-detection beyond Go/npm or per-repo `.tq-verify`                                      | Medium   | M      | Feature       |
| 22 | ★ Harvested tasks carry a `verify` command from repo config                                         | Medium   | S      | Feature       |
| 23 | ★ Harvester per-repo poll interval + DLQ backoff for poisoned repos                                 | Medium   | M      | Feature       |
| 24 | ★ `tq top` live per-project view                                                                    | Medium   | M      | Feature       |
| 25 | ★ Agent transcript (crush session id) recorded on completion, linked from `tq show`                 | Medium   | S      | Feature       |
| 26 | ★ Docs-drift auditor task ("done in code but still unchecked")                                      | Low      | M      | Quality       |
| 27 | ★ `TestShutdownDrains` claim-window flake fix (await claims, don't sleep)                           | Low      | S      | Quality       |
| 28 | ★ E2E CLI-as-subprocess test with stub agent binary                                                 | Medium   | M      | Quality       |
| 29 | ★ Property test: dedup keys stable under reflow, unique across same-text repos                      | Low      | S      | Quality       |
| 30 | ◇ Cancelled-task dedup semantics decision (park forever vs re-open on next harvest)                 | Medium   | S      | Decision      |
| 31 | ◇ Cross-repo DAG from harvest (configurable dependency templates)                                   | Medium   | L      | Feature       |
| 32 | ◇ `tq agent-pool --once` single harvest+drain pass                                                  | Low      | S      | Feature       |
| 33 | ◇ Structured task result payload {files_changed, commit_sha, verify_output_tail}                    | Medium   | M      | Feature       |
| 34 | ◇ PR-mode: agents open PRs instead of committing to master                                          | Medium   | L      | Feature       |
| 35 | ◇ Git worktree isolation option for agents                                                          | Medium   | L      | Feature       |
| 36 | ◇ Session continuation chains via AgentPayload.Session                                              | Low      | M      | Feature       |
| 37 | ◇ Rate-limit concurrent crush sessions; crush version detection at pool start                       | Low      | S      | Feature       |
| 38 | ◇ Output sidecar: full agent stdout to blob file, tail-only in facts                                | Low      | M      | Feature       |
| 39 | ◇ Prometheus metrics endpoint over the facts projection                                             | Low      | M      | Feature       |
| 40 | ◇ `tq harvest --json` and `--repo-subset` glob filter                                               | Low      | S      | Feature       |
| 41 | ◇ Per-repo-size timeout defaults                                                                    | Low      | S      | Feature       |
| 42 | ◇ `tq dlq --rescue-all --older-than` bulk rescue                                                    | Low      | S      | Feature       |
| 43 | ◇ Refuse catastrophic `--projects-dir /` or `$HOME` scans                                           | Medium   | S      | Bug           |
| 44 | ◇ SECURITY.md: what autonomy grants mean, blast radius of `bash` permission                         | Medium   | S      | Documentation |
| 45 | ◇ Fuzz the TODO parser (malformed markdown, CRLF, BOM)                                              | Low      | M      | Quality       |
| 46 | ◇ Windows path handling in harvest; i18n-safe item hashing                                          | Low      | M      | Quality       |
| 47 | ◇ Queue DB rotation/backup guidance (single file = single point of failure)                         | Low      | S      | Documentation |
| 48 | ◇ Chaos test: SIGKILL pool mid-agent-run; lease-expiry reclaim, no double-complete                  | Medium   | M      | Quality       |
| 49 | ◇ Example corpus: runnable `examples/agent-pool/` with `.crushrc` + TODO_LIST.md                    | Low      | M      | Documentation |
| 50 | ◇ Decide when `internal/` packages become the public importable library API                         | High     | S      | Decision      |

**Harvest note:** this session ALREADY harvested the prior report into
TODO_LIST.md/ROADMAP.md (that is where the ★ items live). Items 1–10 are new
from this audit — route them with the next docs-health HARVEST run; items 30,
50 are owner decisions → ROADMAP "Open questions" if not answered first.

## g) Questions I can NOT figure out myself

1. **Is there a live CQA instance I can verify the bridge against, and is its
   API contract (`/api/v1/projects` → `/scans?limit=1` →
   `/issues?fixable_only=true`) confirmed stable?** I checked the tests
   (httptest-only) and the prior report (no live claim). Without this the
   CQA bridge stays `PARTIALLY_FUNCTIONAL` forever, and I cannot tell whether
   the response shapes in `internal/bridge/cqa/cqa.go` match reality or are
   an educated guess.

2. **What is the golangci-lint policy: CI-gate it (with errcheck exclusions
   for idiomatic deferred Close) or drop it from CONTRIBUTING?** Today
   CONTRIBUTING recommends a linter CI never runs, whose baseline findings
   invite every future agent to "fix" correct code. Both resolutions are
   defensible; which one is yours?

3. **Should the auto-commit daemon be pausable for paired/doc sessions?**
   This session it re-committed TODO_LIST.md between my read and write; I
   lost nothing because the change was mtime-only, but with 5+ concurrent
   agents the same race can silently fold one session's half-finished edits
   into another's commit. If you want auditability (squash-before-tag also
   depends on it), tell me the intended daemon etiquette and I'll encode it
   in AGENTS.md.
