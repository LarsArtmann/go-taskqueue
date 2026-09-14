# Window Status Report — five tasks: three review-finding fixes, the ExitError rename, per-module CI lint

- **Written**: 2026-09-10 09:27 CEST
- **Window**: five tasks completed 2026-09-10 ~07:15–08:47 CEST:
  | task        | work commit | what                                                                                 |
  | ----------- | ----------- | ------------------------------------------------------------------------------------ |
  | `…c3919ce4` | `cd09c1c`   | review fix: Task-Queue-ID footer on the release-docs commit (history reword)         |
  | `…e39c8dbf` | `bafc720`   | TODO f30: `runactor.ExitCause` → `ExitError` (errname)                               |
  | `…f1582f2c` | `da8f331`   | review fix: webui `?q=` parse regression (`query.Get("query")`)                      |
  | `…f15854a1` | `9b7c46b`   | review fix: `tq dlq --max-attempts` help literal (`task(store)`)                     |
  | `…a11634e8` | `7f5d699`   | TODO f32: per-module golangci runs in ci.yml (3e0cb85 = its close-out report commit) |
- **Method**: every claim re-verified in this pass — `git show` of all five
  commits; the fixed code read at HEAD (handlers.go:48, main.go:1391,
  runactor.go, ci.yml lint step); `rg` for `ExitCause` remnants (zero);
  root `go build`/`go vet`/`go test -count=1` green; the full disk-derived
  per-module gate loop green over all 7 sub-modules; webui module
  `-race` suite green; remote CI state pulled via `gh run view`; the
  divergence re-counted (`git rev-list --left-right --count`).

## TL;DR

All five window tasks delivered exactly their one finding/item, verified at
HEAD, with correct footers and zero scope creep — the review-response loop
worked as designed. The residue is structural, not per-task: the lineage
fork deepened (local master now 26 ahead / 1 behind, remote CI red on five
jobs whose fixes exist ONLY locally), the rename-sweep that caused two of
the review findings was a class, not a one-off (now encoded as an AGENTS.md
rule), and TODO-item accuracy (f30's wrong package + wrong version-window
claim) cost the repo a day for a 15-minute rename. This pass: 22 new
TODO_LIST items + 3 owner questions harvested, the three stale-done f20–f24
items marked `[x]` after re-verification, four items struck inline in the
23:47 round-2 report, CHANGELOG backfilled for the two rename-leak fixes and
the CI lint loop, FEATURES CI row corrected, ROADMAP gained the string-corpus
idea. No archives (no report qualified).

---

## a) FULLY DONE (verified in HEAD this pass)

1. **Footer fix via history reword** (`…c3919ce4`, `cd09c1c`): the
   round-2 release-docs commit now carries its assigned ID
   (`000001a089b141f0…`) as the `Task-Queue-ID` trailer; the rewrite was
   proven message-only in-task (`git diff 41b817b cd09c1c` empty), 11
   commits replayed, author dates preserved, tags and origin untouched.
   The full fallout (lineage fork, origin divergence, the f26 three-ID
   cluster) is honestly documented in the 07:49 report and routed as owner
   questions since the 08:25 pass.
2. **`ExitCause` → `ExitError`** (`…e39c8dbf`, `bafc720`, TODO f30): zero
   `ExitCause` hits remain in Go code (`rg` clean this pass);
   `internal/runactor/runactor.go` defines `ExitError` with `Error`/
   `Unwrap`, tests renamed, doc comment updated. The task also corrected
   the record: the type lives in `internal/runactor`, not the TODO item's
   claimed `internal/executor/agent.go`, and never needed a "v0.3 window"
   (an `internal/` import path has zero external importers). TODO item
   closed with the corrected annotation.
3. **webui `?q=` parse regression fixed** (`…f1582f2c`, `da8f331`):
   `internal/webui/handlers.go:48` reads `query.Get("q")` again (the
   lint-triage rename had leaked `query.Get("query")` into the literal,
   silently deadening the dashboard text filter while all tests stayed
   green). New `TestParseFilterQuery` (board_test.go:250) pins the param
   name AND a `filterHref`↔`parseFilter` round-trip, closing the exact
   test gap that let it ship. Webui module gates + smoke green in-task;
   module `-race` suite re-run green this pass.
4. **`tq dlq --max-attempts` help restored** (`…f15854a1`, `9b7c46b`):
   cmd/tq/main.go:1391 prints "attempt budget for rescued task(s)" again
   (the s→store rename regex had mangled it to "task(store)"). The task
   swept the parent diff (`eaf73a9`) for further parenthesized-identifier
   literal corruptions: zero additional hits.
5. **Per-module golangci in CI** (`…a11634e8`, `7f5d699`, TODO f32): the
   advisory lint step in `.github/workflows/ci.yml` now loops the root run
   plus every disk-derived `internal/*` sub-module (`GOWORK=off`, same
   pattern as the module-isolation and govulncheck steps) — module-local
   findings stopped being invisible to CI. YAML validated in-task; the
   loop pattern verified at HEAD this pass.
6. **Repo health at close** (this pass, not the window): root
   build/vet/test green; all 7 sub-module gates (build+vet+test) green;
   `check-todo-list.sh` + the harvest-parse guard green after the TODO
   edits below.

## b) PARTIALLY DONE

1. **Vocabulary remnant from the rename**: the local variable `exitCause`
   in `Run()` (internal/runactor/runactor.go:162) still carries the old
   name. Cosmetic, but a rename task that leaves a derived identifier
   behind is half-finished vocabulary work. Filed as TODO.
2. **errname clearance is inferred, not confirmed**: the close-out
   verified the rename via compiler + `rg` while the golangci LSP showed a
   stale `errname` warning; the linter that filed the original finding was
   never re-run scoped to the package. Filed as TODO.
3. **Per-module CI lint is runner-verified only in theory**: the edited
   step can fully prove itself only on a GitHub runner; locally only YAML
   - pattern-parity were checkable. Its runtime cost folds into the still-
     open "measure CI time impact" item (f49), which now lacks a budget
     number (owner question below).
4. **Round-trip coverage is one field deep**: `TestParseFilterQuery` pins
   `q` only; Project/Status/Sort/View have no emitter↔parse pins. The
   reviewer's "ideally" is half-honored by design (the other fields were
   never broken).
5. **CHANGELOG went dark for the window's own deliverables again** —
   backfilled by THIS pass: one combined Fixed entry for the two
   rename-leak regressions, one Changed entry for the CI lint loop. The
   `ExitError` rename deliberately got NO entry (internal-only) — the
   policy question is now explicit (see g1) instead of silently judged.

## c) NOT STARTED (the window spent itself entirely on its five tasks — correct per contract, listed for visibility)

1. **The entire 08:25 window harvest** (21 items) remains open: release.sh
   fixture rehearsal, tag-ancestry audit under the fork, RELEASE.md
   tag-ancestry section, sub-tag-cutting smoke, commit-msg footer hook,
   commits-per-ID queue view, varnamelen raw sites, lint round 2
   (goconst/mnd/paralleltest/testpackage), per-linter baseline, daemon
   attribution proposal, history-rewrite policy.
2. **Dogfood round** (02:00 harvest): cwd-dependence sweep, PATH assert,
   `checkProjectsDir` relaxation, skip-log dedup, `tq doctor` PATH warning,
   dead-pool alert, dogfood smoke, `tq pool-health`, worktree-per-agent
   doc, session-close bridge — untouched.
3. **CI-hardening block** (04-09 harvest): gosec suppression config,
   `check-ci.sh` master-state gate, line-length gate, job summaries,
   required-checks proposal — untouched.
4. **Footer-integrity mechanics** (07:49 asks): no commit-msg hook, no
   queue-side commits-per-ID view — the wrong-footer finding class is
   still caught only by reviewer eyeballs.

## d) TOTALLY FUCKED UP

1. **The remote is red on five CI jobs whose fixes exist only locally.**
   `origin/master` still tips at pre-reword `41b817b`; local master is now
   **26 ahead / 1 behind** (was 14/1 at the 08:25 pass — the fork deepens
   every hour agents commit). The latest remote run (34439012027) fails
   test-windows, test (release-gates smoke), gosec, govulncheck and nix —
   every one of those classes was FIXED in the unpushed local lineage
   (path-aware harvest test, `-c user.email/-c user.name` fixture tags,
   x/text v0.41.0, vendorHash resync). Until the owner reconciles or
   pushes, master's public CI cannot go green no matter what agents fix.
   (The three stale-done f20–f24 items marked `[x]` by this pass are part
   of that unpushed set.)
2. **The rename sweep was a class, and it shipped twice in one commit
   window.** The same wrapcheck/varnamelen triage corrupted a URL param
   parse (`?q=`) and a flag help string (`task(store)`); both user-visible,
   both invisible to every gate. The webui break sat on rolling-release
   master (the dogfood pool rebuilds from it) for ~1h. One of the two
   systemic test gaps is now closed (`TestParseFilterQuery`); flag help
   text is still exercised by nothing. The rule "a variable rename must
   never change a string literal" is now in AGENTS.md (this pass), but no
   mechanical check exists.
3. **The f26 three-ID cluster persists** (`a3864d95` / `b141f022` /
   `c3919ce4`) — still unresolved pending the owner's canonical-ID
   decision. Positive: this window added NO new IDs to the cluster; all
   five work commits carry their exact assigned footers (verified), the
   first clean footer window since the incident.
4. **A wrong TODO item cost a day.** f30 named the wrong package and a
   false "needs a v0.3 window" precondition, and seven status reports
   re-copied it forward unverified before this window's task executed it
   in minutes. Harvested-item accuracy has no bar; the question is routed
   (g3) but the risk stays live for every future item.
5. **Daemon attribution split remains possible**: this window's commits all
   carry correct footers, but repo tooling itself still lands footer-less
   (e.g. `scripts/check-go-mods.sh` exists only inside daemon commit
   `533f5bc`), so ticket↔content archaeology stays unreliable whenever the
   daemon sweeps mid-task.

## e) WHAT WE SHOULD IMPROVE

1. **Rename hygiene is now a written rule — make it a cheap step**: grep
   the diff's quoted lines whenever a renamed variable's name matches a
   nearby query param, JSON tag, or flag text (AGENTS.md, this pass).
   The 08:39/08:42 tasks proved the class; the next sweep should prove
   the fix.
2. **Round-trip tests as the default for every parse/render pair** — the
   emitter-side pin that let the `?q=` bug ship green is the repo's
   standard test shape; pairing it with a parse-side assertion is the
   whole fix.
3. **Build the footer gate already**: a commit-msg hook (via
   `scripts/install-pre-commit.sh`) requiring exactly one shape-valid
   `Task-Queue-ID` trailer would have prevented the 07:49 finding class
   entirely. It has been asked for in three consecutive reports.
4. **Local workflow validation**: `actionlint` in the devShell/ci-local
   would make CI-yml edits verifiable without a 5-minute runner round
   trip — the slowest feedback loop this window touched.
5. **Kill the module-loop copy-paste**: the `find internal -name go.mod`
   loop now exists in ~6 places (4× ci.yml, ci-local.sh, AGENTS.md); one
   `scripts/for-each-module.sh` makes the seventh consumer free and the
   set provably identical.
6. **Re-run the linter that filed the finding as the acceptance test** —
   compiler-clean is not linter-clean (b2 above).
7. **Keep the review-response SLA**: three findings, three same-day
   single-commit fixes, zero unrelated changes — this window's shape is
   the template, say so when staffing future reviewfix tasks.

## f) UP TO 50 NEXT THINGS (22 appended to TODO_LIST this pass, deduped against the existing list; the highlights by theme)

**Finish the rename window's blast radius**

1. Rename the `exitCause` local (runactor.go:162); re-run errname scoped
   to `internal/runactor`; sweep the `eaf73a9` diff repo-wide for further
   literal leaks (`git log -S` probes).
2. Extend round-trip pins to Project/Status/Sort/View; add a handler-level
   `GET /?q=sh` end-to-end test; verify `filterHref` URL-encodes query
   values; one source of truth for the six query param names; check the
   unbounded-`q` LIKE-scan cost.

**Harden the string-literal class mechanically**
3. Help-text smoke asserting no parenthesized-identifier artifacts in
`tq` flag strings; ROADMAP idea filed: user-facing string corpus
snapshot test.

**CI lint loop follow-through**
4. Extend or deliberately exclude `scripts/lint-annotations.sh` from
sub-modules; verify root `.golangci.yml` applies inside sub-module dirs;
`scripts/for-each-module.sh`; `actionlint` locally; per-module baseline
quantification; fixture-go.mod pickup check; unify the golangci pin.

**Small honest cleanups**
5. Delete the (probably) unused `waitFor` webui test helper; CI topology
one-pager; report the session-snapshot ghost (work-commit hashes
pre-appearing in session-start snapshots) upstream.

**Plus the standing, higher-leverage backlog the window skipped** (already
in TODO_LIST, not duplicated): release.sh fixture rehearsal under the
forked lineage, commit-msg footer hook, tag-ancestry audit, lint round 2,
per-linter baseline, dead-pool alert, `tq doctor` PATH warning, dogfood
smoke, CI-state gate.

## g) QUESTIONS I CANNOT ANSWER MYSELF

1. **CHANGELOG policy for non-release-boundary changes**: do
   `internal/`-only renames and same-day introduce+fix regressions (which
   never crossed a release, but the dogfood pool deploys master as a
   rolling release) get entries, or is `[Unreleased]` net-since-last-
   release only? This pass added one combined entry for the two
   rename-leak fixes and none for the `ExitError` rename — confirm or
   overrule.
2. **CI-time budget**: what number should "tune the module loops if they
   dominate" (f49) optimize against (e.g. lint step ≤ N minutes)? Without
   a threshold the per-module lint/govulncheck/gates loops can't be tuned
   mechanically.
3. **TODO-item accuracy bar**: should the harvester verify each item
   against code at harvest time (file/package claims — f30's wrong
   pointer cost a day across seven reports), or is pickup-time
   verification by the executing agent the accepted contract?

---

_Point-in-time snapshot 2026-09-10 09:27 CEST. Re-verify before treating
any claim as current — multiple concurrent-agent sessions are active on
this repo._
