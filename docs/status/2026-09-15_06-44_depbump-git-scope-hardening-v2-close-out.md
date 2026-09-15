# depbump git-scope hardening — v2 close-out (session report)

Session 2026-09-15 ~05:45–06:44, continuing `2026-09-15_05-20_dependency-upgrade-automation-v1-v2.md`
(its §d1 RED blocker → resolved here). All claims cite gate runs from THIS
session; daemon folded the changes (HEAD d687455 at report time).

## a) FULLY DONE

1. **§d1 RED test fixed** — `TestDepBumpExecutorBumpCommitAndTag` green.
   Root cause pinned EMPIRICALLY in a scratch repo before touching code
   (`git add` exit 128 on any unmatched pathspec, NOTHING staged).
2. **`rollback()` silent no-op fixed** — the bigger bug the red test was
   hiding: `git checkout HEAD --` with ONE unmatched pathspec aborts
   restoring EVERYTHING (proven: modified go.mod stayed modified), and
   rollback unconditionally passed `vendor` + templ globs → no-op in every
   non-vendor repo. Now: concrete tracked paths via `ls-files -z` →
   checkout, `clean -fqd` for untracked leftovers (fresh go.sum /
   first-ever templ artifacts), on a DETACHED context
   (`context.WithoutCancel` + 60s cap — rollback must survive the very
   timeout that triggered it). Pinned by
   `TestDepBumpExecutorRollbackRestoresRealChanges` (bump-1-lands +
   bump-2-fails; the pre-existing rollback test was VACUOUS — unresolvable
   `go get` never modifies go.mod).
3. **Per-glob templ scope** — `templGlobs()` checks `*_templ.go` and
   `*_templ.txt` independently (my first version returned both when either
   matched → `hello_templ.go` without `*_templ.txt` still fataled; the new
   templ test caught my own fix being wrong).
4. **`regenerateTempl` detection walk rewritten** — the old walk read
   `repoDir/<basename>` (nonexistent) and `repoDir/<leaf>` (wrong level):
   it NEVER found `.templ` sources, in any repo. First exercised by the
   new templ fixture test.
5. **Idempotent commit** — staging nothing now SUCCEEDS (goal already
   met): two tasks racing to the same pin must not dead-letter the second
   (`TestDepBumpExecutorRebumpIsIdempotent`, HEAD unchanged on re-run).
6. **`TemplBin` override** on `DepBumpExecutor` (mirrors GoBin/GitBin):
   pinned deployments + test stub injection (the templ fixture test
   stubs the generator; the real binary on this host never runs against
   fixtures lacking the templ runtime).
7. **Test suite**: depbump 8/8 green; full executor module green
   (`GOWORK=off go test ./...` — 24.0s final lap).
8. **Parallel-session rescue**: `cmd/tq/doctor_test.go:585,591` mid-refactor
   fallout from their `task.ID` type — completed in their direction
   (`[]task.ID` + `string(id)` at the stdlib boundary); shim gate green
   (`./scripts/test-cmd-tq.sh` ok 10.0s).
9. **Gates green**: per-module loop ALL MODULES GREEN (build+vet+test,
   15 modules) · `-race` root (15 ok) / executor / depsweep · facade
   parity (7 facades) · `check-go-mods` · dead-exports (advisory; depsweep
   wire-contract types flagged zero-importers — expected, they ARE the
   JSON contract) · help-text smoke (18 subcommands; `--dep-sweep` flags
   render clean) · doc gates (features/roadmap, doc-refs, todo-list).
10. **Docs**: AGENTS.md (package-table row + full `depbump` payload-contract
    bullet), CHANGELOG Unreleased entry, FEATURES.md rows (depbump
    executor + `--dep-sweep`, both honest 🟡 PARTIALLY_FUNCTIONAL — no
    live pool run).
11. **Research report addendum** — section 08 in
    `~/projects/reports/2026-09-14_upgrade-automation-research.html`
    (shipped/hardening/verification/open-rulings), including fixing my own
    first-draft CSS classes that didn't exist in that report's stylesheet.
12. **gogenfilter question answered** (user asked mid-session): zero edges
    in the depgraph `output.json` — outside the sweep's scope entirely;
    only third-party deps (Renovate stratum). In scope only if added to
    the planner's scan roots.

## b) PARTIALLY DONE

1. **Gate suite as a whole** — every component run individually, but
   `ci-local.sh` NOT run end-to-end this session, and `lint-baseline
   --check` is RED (see d5). Rationale: avoid racing the parallel
   session's in-flight migration; still an honesty gap in "full gates".
2. **depbump e2e coverage** — push path (`release().Push` → `git push
   --follow-tags origin HEAD`) NEVER executed by any test (fixtures have
   no origin); vendor-scope and go.work-quarantine paths untested
   end-to-end; release-after-idempotent-skip combination untested.
3. **v2 rollout** — executor+sweeper+wiring complete and green, live pool
   run still blocked on the three owner rulings (re-asked in g).

## c) NOT STARTED

- Live `--dep-sweep` pool run (blocked on rulings; default OFF).
- `scripts/smoke/dep-sweep.sh` e2e (stub planner → pool --once → assert
  minted + deduped + DAG-wired tasks) + guard-wiring into ci-local.
- `Task-Queue-ID` footer on depbump commits (see f2 — real gap).
- TODO_LIST harvest of the 05-20 §f list and this report's f-list.
- DOMAIN_LANGUAGE entries (depbump, depsweep, stale build, trap row, wave).
- depbump payload rendering in the webui task detail page.
- Planner-JSON forward-compat version guard (depsweep has none).
- `pool.conf` round-trip verification for the six `--dep-sweep*` flags.
- Investigation of `.github/dependabot.yml` that appeared in daemon commit
  5512bd9 (intersects ruling #2 — see g2).

## d) TOTALLY FUCKED UP (all self-inflicted items caught in-flight, none shipped)

1. **Two compile errors in ONE edit batch** — `depbumpTemplGlobs` without
   `var`, then `dpbumpTouchedPaths` typo. Cause: hand-typing identifiers
   in a rewrite instead of copy-pasting from the file. Both caught by the
   immediate build, but they should not have existed.
2. **Invalid lint methodology** — ran `golangci-lint run ./depbump.go
   ./depbump_unix_test.go` (per-FILE) → 19 typecheck phantom findings;
   package-level is the only valid invocation. Recognized and discarded,
   but I knew better before running it.
3. **help-text false alarm** — declared the smoke FAILED when I had passed
   `./scripts/build-tq.sh` a directory where it expects the output BINARY
   path. One wasted run; lesson: read the script's usage before invoking.
4. **I introduced a LYING COMMENT + scope split-brain** — `depbumpTouchedPaths`
   doc now claims "rollback AND the commit stage exactly this scope", but
   my commit() rewrite hand-builds its stage list (`go.mod go.sum` +
   Stat-gated vendor + templGlobs) and does NOT derive from the var.
   Two parallel definitions of the touched-path scope that must stay in
   sync = classic drift bait. NOT fixed (told to report and wait); f1.
5. **Inherited red gate** — `lint-baseline --check` fails repo-wide (31
   NEW classes in cmd/tq, module-wide mnd/paralleltest/goconst rows).
   Attribution: daemon commit 5512bd9 (09-14 18:21) re-enabled
   goconst/mnd/paralleltest WITH tuned settings; baseline + script were
   re-touched 06:03 today — a parallel session's migration MID-FLIGHT.
   Verified my session's depbump edits contribute ZERO new finding
   classes (gci fixed on the spot); NOT my regression, but the gate is
   red on master RIGHT NOW and I only documented it instead of
   coordinating (see g3).

## e) WHAT WE SHOULD IMPROVE

- **Single source of truth for the touched-path scope** (f1): one spec
  list; commit stages the tracked+untracked subset, rollback restores the
  tracked subset — derived, never duplicated.
- **My claim-scoping was too narrow**: "my files contribute zero new
  finding classes" was true for THIS session's depbump edits, but
  depsweep (yesterday's session, also me) carries real mnd/gocognit
  (36 > 25 on BuildWork)/prealloc debt that WILL grow the baseline the
  moment the parallel policy migration lands. The fuller truth belonged
  in the earlier claim.
- **Implicit test coupling**: depbump fixtures without `.templ` sources
  are protected from the REAL templ binary on PATH only by
  regenerateTempl's early-return ORDER (hasTempl check before LookPath).
  Nobody pins that ordering; a reorder silently makes tests host-dependent.
- **Copy-paste identifiers, read scripts before invoking them** (d1, d3).
- **Vacuous-test hygiene**: the old rollback test passing was taken as
  evidence for two sessions; the new real-rollback test should have been
  written when the rollback path was first authored.

## f) Next (impact-sorted; 30 real items, no filler)

1. Fix the lying comment + UNIFY commit()/rollback() scope derivation (d4).
2. Add `Task-Queue-ID` footer to depbump commits — the queue's OWN commits
   are currently invisible to derived-outcomes attribution
   (`executor.GitLogScanner` matches nothing).
3. Owner rulings (g) → first supervised live pool run (default stays OFF).
4. `scripts/smoke/dep-sweep.sh` e2e + wire into ci-local (guard-wiring gate).
5. Push-path test: local bare repo as origin; assert tag+master pushed.
6. Test release-after-idempotent-skip (tag must land on the no-commit path).
7. Vendor-dir fixture e2e (go mod vendor; commit scope + rollback restore).
8. go.work quarantine test (fixture WITH go.work; assert restored).
9. Run full `ci-local.sh` end-to-end.
10. Re-run `lint-baseline --check` after the parallel migration settles;
    deliberate regen if the policy change owns the growth.
11. Delete/merge the vacuous `TestDepBumpExecutorRollsBackFailedBump`.
12. Guard test: no `.templ` sources → templ binary never invoked (d/e
    coupling pin).
13. Refactor `BuildWork` (gocognit 36 → extract skip-classification).
14. docs-health HARVEST: 05-20 §f + this report → TODO_LIST.md.
15. DOMAIN_LANGUAGE: depbump, depsweep, stale build, trap row, wave.
16. depbump payload rendering in webui detail (repo + bumps lede).
17. Planner-JSON version guard in depsweep (mirror payload V pattern).
18. Verify `--dep-sweep*` flags round-trip through pool.conf persistence.
19. Investigate `.github/dependabot.yml` from 5512bd9 (feeds ruling #2).
20. depbump completion-fact forensics: record tag + applied bumps in the
    journal fact (parity with FailureEvidence).
21. Evaluate per-repo serialization for concurrent depbump tasks (today:
    dirty-preflight requeue wastes a baseline verify; a repo lock would
    avoid it).
22. Surface depsweep skip reasons in `tq stats`/observability.
23. Polish `--dep-sweep*` flag help strings (post-18-subcommand smoke read).
24. gogenfilter scope decision → add to planner scan roots if wanted.
25. Copy/symlink the research-report v2 addendum into this repo's docs
    (the HTML lives outside the repo; the repo docs cite it).
26. Mark the v1 bash driver header as frozen reference (owner ruling from
    05-20 session; one comment line).
27. Windows lap: verify depbump.go pathspecs/paths hold on windows-latest
    (pure tests run there; e2e is unix-only by design).
28. Watch depbump suite timing under parallel load (5.5s → 84.7s observed
    while lint runs hammered the host); consider CI timeout margins.
29. Pin the untracked-templ-artifact edge with a cheap test (commit
    detection includes untracked; rollback clean removes them).
30. Renovate config for the third-party stratum once ownership is ruled.

## g) Questions I can NOT figure out myself

1. **Push ruling** (3rd ask, unchanged): may depbump release tasks push
   master+tag autonomously once verified (`--dep-sweep-push`), or does a
   human push? Tags are the irreversible step; consumers block on
   proxy-resolvable tags either way.
2. **Dependabot/Renovate ownership** — daemon commit 5512bd9 added
   `.github/dependabot.yml` to THIS repo alongside the lint-config change.
   Was that deliberate (yours/another session)? It directly decides
   ruling #2: who owns third-party bumps vs depsweep's intra-ecosystem
   waves.
3. **Lint-policy migration protocol**: the parallel session's migration
   (config re-enabled + script/baseline churn at 06:03) is mid-flight and
   the gate is red. When it settles: is regenerating the baseline
   deliberately MY move, or entirely that session's? (I stayed out to
   avoid racing it.)

— Reported 2026-09-15 06:44; auto-commit daemon owns the commit (harness
rule: no manual commits without explicit request). Report format is
Markdown per explicit user instruction (skill default is HTML dashboard;
override flagged here).
