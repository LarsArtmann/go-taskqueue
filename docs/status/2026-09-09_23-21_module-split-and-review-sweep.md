# Module Split + Review Sweep — Session Status (2026-09-09 23:21)

**Session scope:** execution of the `go-modularize` skill end-to-end (7 phases),
then the follow-up review-skill sweep (error modernization, code quality scan,
ecosystem upgrade, architecture review + D2 visualization, nix review, scoped
code review, brutal self-review). No research beyond this session's own work.

**End state:** `ALL CI GATES GREEN` on the committed tree (ci-local.sh: vet,
build, windows cross per module, race tests ×6 modules, module isolation gates,
go.mod hygiene audits, gofmt, smokes, nix build + flake check). Tree clean.
Nothing pushed. `tq version` → 0.2.0.

---

## What did I forget?

1. **The consumer story, until it was almost too late.** I designed, executed,
   and verified the five-module split testing everything — build, vet, race,
   nix, smokes — _except the one thing the split changes for outsiders_:
   `go install github.com/larsartmann/go-taskqueue/cmd/tq@latest`. With
   internal requires pinned at `v0.0.0` (unresolvable on the proxy), the next
   release would have broken the README's documented install path. Caught in
   the second review pass, fixed with real `internal/*/v0.2.0` tags + a
   clean-room `go install` gate — but the correct order was: design the
   consumer story BEFORE the split, not two reviews later.
2. **My own predicted trap.** The adversarial review flagged that directory
   patterns (`go test ./internal/queue`) stop crossing module boundaries —
   and minutes later I ran exactly that command from root and stared at the
   failure. Knowing a rule and wiring it into your own muscle memory differ.
3. **The proposal's first draft contained a false mitigation** (risk #6:
   "fuzz/smoke resolve via the root module — unchanged"). Wrong: directory
   patterns don't cross modules. The adversarial pass caught it the same day
   and the proposal was corrected, but it was written wrong first.
4. **The release.sh conflict was discoverable earlier.** I read the ADRs and
   build scripts during Phase 2 research, but didn't grep release.sh's gates;
   the split-vs-release-script collision (release.sh rejected ALL root
   replaces) surfaced only via the review agent.
5. **A `\t` I wrote myself.** My first ci-local hygiene audit used
   `grep '\t'` — which in POSIX ERE is a literal `t`, so the gate matched
   nothing while looking vigilant. (The pre-existing release.sh audit had the
   same latent bug.) Both rewritten with `[[:space:]]` and proven by
   extracting real matches.

## What could I have done better?

- **Made "consumer installability" an explicit acceptance test of the split
  plan** (Phase 3 of modularization), instead of discovering it in review.
- **Written the AsType migration correctly the first time** — my first edit
  produced invalid Go (variable declaration inside a `switch` case clause);
  sloppy composition of a mechanical change.
- **Checked version surfaces proactively.** flake.nix still said 0.1.0 after
  v0.2.0 shipped (pre-existing drift from this morning's release, not mine —
  but a five-minute surface audit would have caught it hours earlier, and the
  go-ecosystem-upgrade skill explicitly warns about version-surface drift).
- **Treated the auto-commit daemon as part of the plan from step 1.** Several
  per-step "single revertable commits" got split across daemon auto-commits +
  mine; I adapted mid-flight rather than designing for it (e.g. staging files
  immediately before each commit).
- **Skipped loading the status-index convention until commit time** — worked,
  but by luck of reading the repo docs earlier, not by process.

## What could I still improve?

- The **P1–P3 roadmap** produced by the reviews (below, section f) — none of
  it is started.
- **Gate testing culture:** release.sh now has regex+git gates with no test
  harness; today's fixes were verified by manual extraction only.
- **Docs-time-travel hygiene:** same-day documents (proposal/execution-plan
  HTMLs) still contain the superseded `v0.0.0` versioning stance; ADR-0011
  governs, but a reader landing on the proposal first gets a stale story.
- **Speed:** the review sweep ran serially where the metric-gathering part
  could have been parallelized earlier.

---

## a) FULLY DONE

| Work                                                                                                                                                                                                                                            | Evidence                                                                  |
| ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------- |
| **Five-module split (ADR-0011)** — `internal/{task,journal,queue,executor,worker}` are sub-modules; app layer stays in root; import paths unchanged; DAG compiler-enforced                                                                      | `go list` verified acyclic; per-module builds green                       |
| **Replace-only strategy** — requires at real tags (`internal/*/v0.2.0`) + relative replaces; no go.work (FM#4 eliminated by construction)                                                                                                       | hygiene audits green                                                      |
| **Per-module CI gates** — `GOWORK=off` build/vet/test loops in ci-local.sh AND ci.yml (Linux + test-windows jobs), per-module `GOOS=windows` cross-compiles                                                                                     | ci-local green run                                                        |
| **Release path repaired** — sibling-replace allowlist, sub-tag existence gate, tag cutting + push for internal tags, two-phase `--tag`/`--push` resume, flake version-sync gate, clean-room `go install` + binary run, SIGPIPE-proof tag checks | `bash -n` + manual gate extraction tests + review agent verdict folded in |
| **Subdirectory tags cut** — `internal/{task,journal,queue,executor,worker}/v0.2.0` (annotated, LOCAL ONLY — push owner-gated)                                                                                                                   | `git tag` listing                                                         |
| **Version drift fixed** — flake.nix 0.1.0 → 0.2.0 (both version attr + ldflags); `tq version` reports 0.2.0                                                                                                                                     | nix rebuild + binary run                                                  |
| **errors.As → errors.AsType** — all 4 sites (runactor ExitCause, executor PermanentError, 2 test counterparts); sentinels kept; erraudit re-run: 0 As findings                                                                                  | commit c370ea9                                                            |
| **Duplication reduced** — harvest audit/prune 20-line preamble extracted to `projectTaskIndex`; 4 remaining clone groups judged intentional (sqlite/postgres conformance mirrors)                                                               | art-dupl re-run; harvest suite green                                      |
| **CI/fuzz/agent-verify multi-module wiring** — fuzz campaigns cd into package dirs; Postgres job `cd internal/queue`; agent default verify walks nested go.mod (with behavioral test proving a broken nested module fails)                      | fuzz smoke 5s green; executor tests green                                 |
| **flake.nix** — vendorHash refreshed (fakeHash dance), `GOWORK = "off"` pinned, version 0.2.0                                                                                                                                                   | `nix build` + `nix flake check` green                                     |
| **Docs** — ADR-0011; AGENTS.md (multi-module ground rules); README dev section; CHANGELOG `[Unreleased]`; proposal + execution plan HTMLs; architecture review + D2 diagrams; code-quality scan report; brutal self-review report               | committed                                                                 |
| **Verification battery** — root race suite green (12 pkgs), per-module GOWORK=off build/vet/test/`go mod verify` green ×5, hygiene audits green, full ci-local green twice                                                                      | output captured                                                           |
| **Dependency sweep** — verified no-op: all direct deps at latest; only transitive test-dep updates available (correctly not churned)                                                                                                            | `go list -m -u` across 6 modules                                          |

## b) PARTIALLY DONE

1. **Subdirectory tags**: cut locally, **not pushed** — proxy won't serve
   them until an owner-gated push; `go install` fix is therefore latent
   until then.
2. **Consumer story**: `go install` gate is in release.sh (runs at release
   time); it has never executed end-to-end against a real pushed release.
3. **internal/consumer**: identified as a ghost system (zero production
   importers) — reported in the architecture review; wire-vs-delete decision
   pending owner input.
4. **Dead exported API**: 15+ zero-reference exports catalogued (task,
   journal, queue, worker, executor); prune not started.
5. **cmdAgentPool decomposition**: identified (658 lines, highest-churn
   file); not started.
6. **Docs-time-travel**: proposal/plan HTMLs superseded on the versioning
   stance by ADR-0011; annotation pass not done.
7. **CI parity**: go.mod hygiene audits run in ci-local.sh only, not ci.yml.
8. **docs-health HARVEST**: this report's section (f) should feed
   TODO_LIST.md/ROADMAP.md — not done (you said wait for instructions).
9. **nix flake check warnings**: 4 apps lack `meta.description` — cosmetic,
   not addressed.
10. **Executor flake**: `text file busy` on stub-agent re-exec observed once
    (pre-existing); passes on retry; root cause not addressed.

## c) NOT STARTED

- **bdd-testing skill** — deliberately skipped: repo convention is plain
  table-driven `testing`; introducing Ginkgo would fight the codebase.
  Needs an explicit owner decision if wanted at all.
- **Full file-by-file code review** (every file, per the full-code-review
  skill) — only a scoped review of session-touched files ran.
- **naming-review sweep** of the new identifiers (`projectTaskIndex`,
  `TAG_EXISTS`, gate names).
- **data-model-review** of `task.Task`/`Status` (e.g. branded-ID evaluation —
  `go-branded-id` is already an indirect dep).
- **library-deep-dive** audits (templ-components utilization beyond the
  adoption table; modernc.org/sqlite usage depth).
- **deduplicate-code deep pass** on webui (2,680 LOC, largest app package).
- **govulncheck / gosec** in CI (how-to-golang recommendations, absent here).
- **Public-API promotion** out of `internal/` (owner-gated, post-prune).

## d) TOTALLY FUCKED UP (found + fixed inside the session; nothing is broken now)

1. **`go install` would have broken at the next release** — the split's
   `v0.0.0` requires resolve nothing on the proxy. Fixed (real tags +
   requires + release.sh gate); never shipped because nothing was pushed.
2. **release.sh vs the split**: the script rejected ALL root replaces — the
   very next release would have died (or tempted someone to strip the
   replaces, breaking the build). Fixed with a precise allowlist + sub-tag
   gates.
3. **Two-phase `--tag` → `--push` flow was unreachable** — resume died on the
   order/exists gates. Fixed with tag-at-HEAD resume logic.
4. **Version drift shipped with v0.2.0**: flake.nix said 0.1.0, so nix
   binaries misreported `tq version` since this morning's release. Fixed
   (0.2.0), gated for future releases.
5. **Two silently-vacuous gates** (`\t`-grep bug class): release.sh's old
   replace gate and ci-local's old pin audit matched nothing. Fixed and
   proven with real extractions.
6. **Stale webui CSS artifact**: committed app.css was an unminified 6k-line
   build; the a11y gate (prefers-reduced-motion:reduce) failed master-wide.
   Regenerated via the documented build step. (Pre-existing; fixed.)

## e) WHAT WE SHOULD IMPROVE

- **Design acceptance tests for cross-cutting claims** before structural
  changes: "does `go install` still work from a clean room" should have been
  a modularization-phase gate, not a review finding.
- **Every new gate must demonstrate its negative path once** (the `\t` class
  survives on gates that never fired).
- **Version surfaces need one owner**: flake.nix version + ldflags + git tag
  - CHANGELOG — release.sh now gates the first three; keep it that way and
    never hand-bump.
- **Kill dead subsystems early**: consumer sat unnoticed because nothing
  fails when a package has zero importers — an "unimported packages" audit
  belongs in the periodic review set.
- **Retire the lint baseline in slices** (wrapcheck 50, varnamelen 50 are the
  biggest classes) instead of letting the advisory number grow.
- **Prefer behavior tests over string asserts** for generated shell/commands
  (the defaultVerify behavioral test pattern worked well).

## f) UP TO 50 THINGS TO GET DONE NEXT

P1 = do first; owner-gated items marked. (Brainstorm list — HARVEST into
TODO_LIST/ROADMAP with routing rigor; many are roadmap fuel, not commitments.)

1. **P1, owner-gated**: push master + the 6 local tags
   (`internal/*/v0.2.0`) — the go-install fix is latent until the tags land.
2. **P1**: decide `internal/consumer`: wire into `tq serve` journal tailing
   (replacing the ad-hoc tailer) or delete. Then write the outcome ADR.
3. **P1**: prune the 15 dead exports in the core modules (list in the
   architecture review).
4. **P1**: owner decision on the three near-identical interfaces —
   `consumer.Source`, `budget.FactSource`, `bridge/papdashboard.FactSource`
   (consolidate or document why three).
5. **P2**: decompose `cmdAgentPool` (main.go:614–1271) into flag-block +
   actor-wiring helpers, target <200 lines each.
6. **P2**: release.sh gate smoke script (fixture go.mods, positive+negative
   for the replace-allowlist and require-tag gates).
7. **P2**: CI parity — move go.mod hygiene audits into ci.yml.
8. **P2**: fix the executor stub-agent `text file busy` flake (serialize
   re-exec or write-to-temp+rename).
9. **P2**: docs-health HARVEST this report's (f) into TODO_LIST/ROADMAP.
10. **P2**: annotate the proposal/execution-plan HTMLs (superseded v0.0.0
    stance → point at ADR-0011).
11. **P2**: verify `nix run .#test` actually covers the five sub-modules
    (suspected root-module-only coverage).
12. **P2**: after tags are pushed: verify proxy serves each
    `internal/*/v0.2.0` (`go list -m -versions`), then run the clean-room
    `go install` once for real.
13. **P3**: add `meta.description` to the four flake apps (kills the nix
    flake check warnings).
14. **P3**: `nix flake check --all-systems --no-build` once for the split.
15. **P3**: naming sweep of session-introduced identifiers.
16. **P3**: data-model review of `task.Task`/`Status` (branded IDs?).
17. **P3**: dead-code audit script (exported-with-zero-importers) as a
    periodic report, so consumer-style ghosts get caught in CI-adjacent time.
18. **P3**: govulncheck step in CI (advisory or gated — owner call).
19. **P3**: gosec advisory scan.
20. **P3**: templ-components deep-dive audit (adoption table exists; look
    for missed components).
21. **P3**: webui dedup deep pass (render.go 728 + handlers.go 527).
22. **P3**: httpapi/webui API-surface split-brain check.
23. **P3**: examples: add a full-core example (worker + executor + queue) to
    prove the embeddable story end-to-end.
24. **P3**: document the release flow changes in the release checklist doc.
25. **P3**: version-surface inventory doc (flake ×2, tag, CHANGELOG).
26. **P3**: lint-baseline slice-triage: wrapcheck (50) first.
27. **P3**: varnamelen slice-triage (50).
28. **P3**: runactor `ExitCause` → `ExitError` rename consideration (errname
    advisory; domain term may justify keeping — decide explicitly).
29. **P3**: run `go mod verify` in CI per module (currently only locally).
30. **P3**: golangci per-module runs in ci.yml (currently ci-local only).
31. **P3**: consider Dependabot/renovate policy per how-to-golang (owner
    call; repo pins toolchain tightly).
32. **P3**: `tq doctor` multi-module awareness check (does it assume a
    single-module repo when inspecting this one?).
33. **P3**: multi-repo smoke against a nix-built 0.2.0 binary
    (`TQ_BIN=result/bin/tq`).
34. **P3**: README architecture blurb for contributors (module map + gate
    commands).
35. **P3**: CHANGELOG discipline reminder: `[Unreleased]` accrues from next
    session.
36. **P3**: evaluate `go.work` generation helper for devs who want workspace
    mode (optional; replace-only must stay the source of truth).
37. **P3**: budget/consumer tests currently duplicate journal fakes — shared
    testhelpers module evaluation (skill pattern) or keep inline (repo
    convention leans inline).
38. **P3**: fuzz: consider a corpus-growth cap review (MAX_SEEDS default
    1000; executor corpus at 212 and growing +7/session).
39. **P3**: snapshot/golden test for `tq version` output vs flake version
    (kills the drift class permanently at test level).
40. **P3**: sweep TODO_LIST.md for stale items invalidated by the split
    (docs-health VERIFY mode).
41. **P3**: FEATURES.md: add the multi-module split as a structural feature
    note (check-features-roadmap guard expects shipped-vs-planned honesty).
42. **P3**: add `internal/*/vX.Y.Z` version-bump reminder to the release
    checklist (requires bump when sub-modules change semantically).
43. **P3**: review whether `sub/go.mod`-style nested modules need `go
    1.26.7` toolchain alignment enforcement (all six currently match — keep
    a gate?).
44. **P3**: consider `go vet` per module with `-tags` matrix (race_on/off
    files exist in queue).
45. **P3**: prune or wire the `queue.RequeueEvidence`/`ArchiveStats`-class
    dead exports specifically (they're fact-contract adjacent — decide,
    don't leave).
46. **P3**: agent pool dogfood check: this repo's own pool tasks may now
    fail on module-unaware verify commands in OLD queued tasks (drain
    or cancel stale ones).
47. **P3**: confirm `scripts/smoke/multi-repo.sh` still passes (not run this
    session — it's in README but not in ci-local's smoke list).
48. **P3**: ADR-0007 Postgres store: `go test . -run TestPostgres` needs a
    live DB locally — document the docker one-liner next to the script.
49. **P3**: `-count=1` in per-module CI loops disables cache — measure and
    tune if module gates dominate CI time.
50. **P3**: after the next real release, write the "first multi-module
    release" retrospective into docs/release (institutionalize what we
    learned).

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **`internal/consumer` — wire or delete?** It's the ADR-0009 journal
   dispatcher with zero production importers. I can wire it into `tq serve`'s
   journal tailing (replacing the webui's ad-hoc tailer) or delete it — but
   the intent behind ADR-0009 (why was it built without a consumer?) is yours
   to rule on: was it ahead of its time (keep + wire) or superseded by the
   webui tailer (delete)?
2. **Push authorization**: master (≈20 commits: the split, release-path
   repairs, review artifacts) plus the 6 local tags are unpushed. The
   `go install` fix stays latent until the tags land on the proxy. Push now
   (owner-gated), or wait for a specific moment?
3. **Is public-API promotion targeted for v0.3?** The answer changes how hard
   I should push the dead-export prune and interface-consolidation work
   (items 3–4): if promotion is near, pruning and API hygiene jump to P1; if
   it's far away, they can stay P3 backlog.

---

_Report by Crush (glm-5.3-flash), 2026-09-09 23:21 CEST · every claim
verified against this session's runs and commits (043a89e…5496663) ·
final ci-local run: ALL CI GATES GREEN on 5496663._
