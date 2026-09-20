# Dependency-Upgrade Automation — v1 (driver) + v2 (tq-native depbump/depsweep)

Recorded 2026-09-15 (the session it documents ran 2026-09-14 12:06 → 2026-09-15 05:20).

Session 2026-09-14 12:06 → 2026-09-15 05:20. Prompt: "How could we automate
these upgrades? SMARTLY!?!" over the project-dependency-graph `update-plan`
output (178 modules, 78 stale builds, 237 consumer bumps, 5 waves), later
challenged ("best most scalable and reliable way?") and re-commissioned
("Try again!" → v2 in THIS repo). Report covers this session's work only.

## a) FULLY DONE (verified)

1. **Research** — depgraph internals (`update-plan` / `release-suggestions` /
   `release-overview` wave computation, `latestGitTag` reads LOCAL tags,
   `FetchBuildStatus` diffs go.mod vs tag), tq contracts (AgentPayload,
   `tq enqueue --deps` DAG, DedupKey-idempotent Enqueue returning the stored
   task with NO created/existing signal), `go-auto-upgrade` (code migrator,
   not a version bumper), prior art `~/projects/scripts/update-cqrs-lite-deps.sh`.
2. **Key semantic discovery** — "stale build" ≠ "needs bumping": the working
   tree ALREADY holds the newer pins (committed, unreleased); the work is
   verify → tag → push. Verified against ai-task-prioritizer (go.mod at
   lipgloss v2.0.6 while tag v0.1.0 pins v2.0.5). This reshaped the whole
   design into three work classes: release / consumer-bump / major(agent).
3. **project-dependency-graph changes** (other repo, daemon-committed there):
   `update-plan --format json|yaml` (parity with release-suggestions);
   `dir` enrichment on releases, outdated consumers, suggestions, overview
   modules (`NodeInfo.Dir` ← `Module.Dir`); camelCase JSON tag cleanup
   (OutdatedConsumers → outdatedConsumers etc.); new
   `update_plan_structured_test.go`; all 6 modules build+vet+test green;
   gofumpt; CHANGELOG + README + FEATURES.md rows.
   Live verification: 131/131 releases and 237/237 consumer updates carry
   dirs; driver counts reconcile EXACTLY with the manual plan (78 stale /
   211+26 consumer / 30 major).
4. **v1 reference driver** `~/projects/scripts/depgraph-upgrade-sweep.sh`
   (~700 lines, not a git repo): plan/execute/enqueue modes; scopes
   stale|consumers|unreleased|major|all; wave = dep-wave+1 ordering;
   traps (downgrade / `-dev` / never-released / major) visibly skipped;
   go.work→.bak dance; vendor/ + templ fixups; baseline build+test before
   any touch; `go build -o <tmpdir>` (no binary litter); one conventional
   commit per repo; local-first tagging with dir-prefix support; --push
   gate; tq enqueue emission via `--payload @file`; refuses --apply without
   --db/TQ_DB (exit 2); dirty-tree guards in BOTH passes.
   Fixture-verified end-to-end (release tags v0.1.1; consumer bump
   testify v1.10.0→v1.11.1 committed clean; dirty repo skipped, WIP intact).
5. **HTML research report** `~/projects/reports/2026-09-14_upgrade-automation-research.html`
   (Bauhaus dark, 8 sections, tag-balanced).
6. **Brutal self-review** (skill): honest defect table (concurrency hole,
   serial execution, bash substrate, agent-type-for-everything, dead-end
   red baselines, Renovate overlap) → v2 architecture decision. The one
   on-sight safety fix (v1 dirty-tree guards) applied + fixture-verified.
7. **v2 in THIS repo — shipped and green**:
   - `internal/executor/depbump.go`: `TaskTypeDepBump` ("depbump"),
     `DepBumpPayload` (V-versioned contract), deterministic flow — clean-tree
     preflight (PreflightError = requeue without attempt burn), baseline
     build+test, exact `go get module@version` + pin re-verification,
     go.work quarantine, tidy/vendor/templ, touched-path rollback on ANY
     post-mutation failure, conventional commit, dir-prefixed annotated
     tags, optional push; Permanent classes for payload/contract misses;
     `IsStableSemver`. Module builds + vets.
   - `internal/depsweep` (plan.go + sweep.go): wire-contract plan types +
     `ParsePlan` + `DepgraphSource` (subprocess depgraph, JSON is the ONLY
     seam — no depgraph import, zero new module deps); `BuildWork`
     (release/consumer/major classes with skip reasons; consumer grouping
     per repo; order-stable dedup keys `depsweep:<repo>:release:<v>` /
     `:bump:<hash>`); version-based major detection as defense-in-depth
     beyond the plan's flag; `Sweeper.Sweep` mints depbump tasks wired by
     the plan DAG (dep task IDs), dedup-key index for known-detection AND
     cross-sweep dep wiring.
   - depsweep tests GREEN: stale releases (planned version wins, patch
     fallback), unreleased opt-in, trap rows never mint (downgrade/major/
     not-newer each surfaced), grouping + order-stable dedup, Sweep
     mint→dedup→deps-wiring integration (sqlite), ParsePlan, semver table
     (pre-release sorts older — dev traps never look newer).
   - Facade aliases in `executor/executor.go` (types, payload types,
     TaskTypeDepBump, 3 error sentinels, NewDepBumpExecutor, IsStableSemver);
     `scripts/check-facade-parity.sh` OK (7 facades).
   - Pool wiring: `--dep-sweep` flag family (bin / dir / interval / push /
     unreleased) in cmd/tq/agentpool.go; depbump registered on EVERY pool
     with `ExtraEnv: GoEnvExperiment` (the pool-env GOEXPERIMENT lesson);
     mintPass-gated interval tick in cmdAgentPool's runTick;
     `scripts/test-cmd-tq.sh` shim gate GREEN (10.1s).
   - **Rescued a broken daemon-committed state from a concurrent session**:
     `runVerify` gained a `reresolve bool` param but `status.go:171` and
     `agent_test.go:563` weren't updated (build RED on master-pending);
     judged + fixed minimally (false — the pin is built at run time there),
     build green again, comment explains the judgment.

## b) PARTIALLY DONE

1. **depbump executor tests** — pure tests GREEN (IsStableSemver table,
   commit message, payload-contract permanents incl. future-version
   guidance). Unix flow tests: dirty-tree preflight + rollback +
   baseline-refusal passed in the first run; **BumpCommitAndTag RED** (see
   §d1). Overall executor module suite currently FAILING on that one test.
2. **Lint state of new files** — LSP reports prealloc (plan.go, likely
   stale — I made() the capacity), gocognit 36 on BuildWork (stale: I
   extracted releaseSpecs/consumerSpecs after), varnamelen `j`, mnd numbers
   (policy-disabled). Truth requires `scripts/lint-baseline.sh --check` —
   NOT yet run this session.
3. **v2 docs** — AGENTS.md payload contract entry, CHANGELOG, FEATURES.md
   rows: not written (was next when the status call arrived).

## c) NOT STARTED

- `-race` runs over the new packages; full multi-module gate loop
  (root + 12+ internal modules); `scripts/check-go-mods.sh`;
  `scripts/ci-local.sh`.
- AGENTS.md updates: depbump payload contract section, agent-pool flag
  docs, smokes list mention.
- CHANGELOG.md (root) v2 entry; FEATURES.md rows (depbump, depsweep,
  --dep-sweep).
- Report addendum (v2 section) in the HTML report.
- Real-pool dry run: `tq agent-pool --dep-sweep` against a scratch journal
  with a fake depgraph binary (a DepgraphSource fake-bin smoke).
- Majors routing config (MajorsToAgents → mint agent tasks for [MAJOR]
  rows) — designed, deliberately not implemented (AI spend gate).
- Housekeeping: dead-export audit for new symbols, check-dead-exports.sh.

## d) TOTALLY FUCKED UP

1. **TestDepBumpExecutorBumpCommitAndTag RED (current blocker)**:
   `commit()` runs `git add -- go.mod go.sum vendor *_templ.go *_templ.txt`
   and git FATALS when no `*_templ.go` exists in the repo
   (`pathspec '*_templ.go' did not match any files`) → stage aborts →
   commit fails → rollback fires → happy path broken. One-line class fix:
   add templ globs ONLY when `.templ` sources exist (or stage the exact
   dirty set from `git status --porcelain` intersected with the touched
   scope). The red state was daemon-committed (working tree clean at
   06f1751). Stopped mid-fix per the status-call instruction.
2. Session mistakes (all found + fixed same-session, honest record):
   bash TSV delimiter collapse (tab is IFS whitespace → `|`), missing
   `mode` column in jq rows (field shift), apostrophe in a jq comment
   terminating the shell single-quote, `git add vendor` pathspec fatal in
   v1 (same class as d1!), `go build ./...` littering binaries into repos
   (→ `-o tmpdir`), enqueue payload shell-quoting breakage (→ `@file`),
   mint/known detection via `Attempts==0 && Pending` (wrong for unclaimed
   dedup hits → dedup-key index), major detection trusting only the plan
   flag (→ version-compare defense), two multiedits rejected on stale reads
   mid concurrent edits (re-read + rebuild each time).
3. Judgment calls I'd grade down: shipped v1 with "verified" language while
   the concurrent-agent hazard was unguarded (caught only under challenge);
   enqueue mode minted `--type agent` for deterministic work (v2's depbump
   type is the correction); no run ledger in v1 (state only in git+stdout).

## e) WHAT WE SHOULD IMPROVE

- Finish d1 (templ pathspec) and re-run the full executor suite; then the
  gate ladder in §f. The rollback path deserves its own test for the
  templ-file case once fixed.
- The depsweep↔depgraph contract is pinned by tests on BOTH sides — keep it
  that way; any field rename is now a two-repo change (that is the point).
- Consider minting `sh`-typed probe tasks or a pre-tag proxy-resolution
  check before release-with-push (Phase 6 discipline).
- v1 bash driver: keep as reference implementation for the semantics; do
  NOT extend it further (substrate verdict stands).

## f) NEXT (up to 50, ordered)

1. Fix d1: conditional templ-glob staging in depbump commit(); re-run
   TestDepBumpExecutor* — suite must go green.
2. Add rollback-with-templ-files test (fixture with .templ source).
3. `scripts/lint-baseline.sh --check` (new-file lint truth; regenerate
   baseline only if the new rows are policy-owned).
4. Root + all internal-module gate loop (build/vet/test per AGENTS loop).
5. `-race` over internal/depsweep + internal/executor.
6. `scripts/check-go-mods.sh` (confirm zero dep changes — the x/mod
   avoidance should show no drift).
7. `scripts/check-dead-exports.sh` (new exports all referenced).
8. AGENTS.md: depbump payload contract + depsweep design note + flag docs.
9. CHANGELOG.md v2 entry (depbump executor, depsweep sweeper, --dep-sweep).
10. FEATURES.md rows for the three capabilities.
11. Report addendum: v2 section appended to the HTML report.
12. Scratch-journal smoke: agent-pool --dep-sweep with a fake depgraph
    binary printing a fixture plan; assert minted tasks + dedup on tick 2.
13. Decide + implement majors routing (MajorsToAgents, default off) with
    budget-gated minting and an agent prompt carrying the bump list.
14. Decide push policy (§g1) and wire default accordingly.
15. Proxy-resolution probe before release-push (go list -m dep@tag in a
    temp module, or GOPROXY=direct git ls-remote check).
16. DepgraphSource test via fake bin script (unix-gated).
17. Pin the depsweep JSON contract version (add plan schemaVersion handling
    forward-compat comment; depgraph side already versionless — document).
18. `tq doctor` awareness of depbump tasks? (probably no — but check the
    type switch at main.go:1845 covers unknown types gracefully).
19. WebUI: depbump tasks render as generic type — consider payload-section
    lede rule (item-less payloads lead with... bumps list) — defer.
20. Consider `--dep-sweep-max` (cap mints per sweep) for first-run safety
    on the real ecosystem (141 releases + 237 bumps mint at once today).
21. Multi-module repo support test: dir-prefixed tags for a monorepo
    sub-module fixture (executor release path).
22. quarantineGoWork: add a test with a go.work fixture proving rename
    - restore around go get.
23. depsweep: skip-log dedup (like the pool's skipLogExamples) so a steady
    141-skip plan doesn't flood journald every 15m tick.
24. Interval default sanity: 15m re-plan on a 4.7s plan is fine; consider
    jitter to avoid harvest-tick alignment.
25. Guard: depbump tasks under `--max-pending` accounting? (verify pool
    accounting treats them like any pending task).
26. Docs: SECURITY.md note if --dep-sweep-push becomes default anywhere.
27. Consider DepBumpPayload.TimeoutMinutes default override per pool flag.
28. tq show: depbump payload section (bumps table) — small UX win.
29. Facade example: extend examples/embed? (probably out of scope — check
    the on-ramp policy for new executors).
30. Cross-repo: depgraph side needs a release tag (v0.3.x?) carrying the
    dir/JSON contract before the DOGFOOD pool can use a proxy-built
    depgraph binary — coordinate release timing.
31. Dogfood rollout plan: run --dep-sweep on a SCRATCH journal first, then
    opt the production pool in behind owner approval.
32. Renovate question (§g2/§e): third-party tail ownership split.
33. Re-verify the concurrent session's runVerify change didn't regress
    verify semantics (I fixed compile; semantic review of reresolve=true
    path belongs to its author).
34. `git status` hygiene rule for depbump: assert `user.email` configured
    in target repos (commit fails otherwise → task fails; preflight-able).
35. Rollback edge: concurrent writer between guard and commit — the
    touched-scope rollback already avoids foreign files; add a test.
36. Test that a completed depbump task's dedup suppresses remints but a
    DEAD one ALSO suppresses (documented DLQ-is-the-surface ruling) — pin it.
37. Consider closeout N/A for depbump (deterministic; no self-review turn).
38. Consider budget.Guard exemption debate for depbump (free work blocked
    by cap today — deliberate per SECURITY.md; revisit if it bites).
39. Check worker claim behavior for depbump tasks with unmet deps
    (must skip cleanly, not fail) — pin with a test.
40. `tq tasks --type depbump` output niceties if needed later.
41. Sweep stats logging cadence (only-on-change today; add first-run INFO).
42. Index this report (done with the file) + amend-maneuver if daemon
    races it.
43. Archive v1 research once v2 lands (report lifecycle rule).
44. Re-run check-facade-parity + full ci-local.sh before declaring v2 done.
45. Session-close: verify master CI state via scripts/check-ci.sh (the
    gate ladder's step zero).

## g) QUESTIONS (cannot figure out myself)

1. **Push policy**: should `--dep-sweep-push` release tasks push
   master+tag autonomously in the dogfooded pool? The agents-never-push
   rule covers AGENT tasks; depbump is deterministic scripted work, and
   consumers cannot resolve un-pushed tags — but push is the irreversible
   step. Default OFF today (human pushes; consumers retry until it lands).
2. **Third-party tail ownership**: repos carry renovate.json — should
   Renovate own continuous third-party bumps (shrinking depsweep's job to
   own-library wave convergence), or is depsweep intended to be the single
   mechanism? Affects whether I mint third-party stale-release work at all.
3. **Concurrency protocol**: a parallel session (templ-components consumer
   sweep) landed changes in cmd/tq + executor mid-flight, including the
   daemon-committed broken state I fixed in §a7. Do you want serialization
   (one session in cmd/tq+facade at a time), or keep racing with re-reads
   as we did? Also: is the runVerify reresolve semantic review claimed, or
   should I take it?

— Session verdict: v1 shipped + verified end to end (reference driver);
v2 ~90% — one RED test (templ pathspec, §d1), gates + docs outstanding.
