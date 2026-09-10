# Window Status Report — five tasks: two review-fixes, two release docs, lint-baseline triage

- **Written**: 2026-09-10 08:25 CEST
- **Window**: five tasks completed 2026-09-10 ~05:57–07:49 CEST
  (`…37826`→`b31f7fd`, `…899ed`→`16a15d8`, `…37be`→`41b817b`/reworded
  `cd09c1c`, `…e4919`→`e8a4c95`, `…ced7ba`→`eaf73a9` + daemon-swept code in
  `bebc35a`/`fc495e8`).
- **Method**: every claim re-verified in this pass — `git show` of all five
  commits plus the two daemon sweeps, `go build`/`go vet` on the root module
  (green), the full disk-derived per-module gate loop over all seven
  sub-modules (green), scoped AND full-config `golangci-lint` v2.13.2 runs
  (root module), spot-checks of the new docs' claims against
  `scripts/release.sh`, `scripts/lib/release-gates.sh`, `flake.nix`, and the
  git tag list; `git fetch` + rev-list for the origin topology.

## TL;DR

| Area                  | Verdict                                                                                                                         |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| The five window tasks | All five delivered; verified in HEAD                                                                                            |
| Release docs          | RELEASE.md + VERSION-SURFACES.md claims spot-verified against the scripts they describe; both now cross-linked (this pass)      |
| Lint triage           | wrapcheck 0 + varnamelen 0 reproduce under the project config; two raw sites surface under bare `--enable-only` diagnostics     |
| Gates                 | Root build/vet green; all 7 sub-module gates green (re-run this pass)                                                           |
| Lineage               | Local master and origin/master DIVERGED (14/1): the footer-fix reword forked the line; v0.2.0 tags descend from the remote side |
| TODO_LIST hygiene     | Three stale-done items found (98/99/100 — fixed per CHANGELOG, still unchecked)                                                 |

---

## a) FULLY DONE (verified in HEAD)

1. **Review-fix: README examples paragraph mentions `examples/fullcore`**
   (task `…37826`, `b31f7fd`). README.md:395 names fullcore after the
   api/sse PoC sentence; the caveat now correctly scopes to api+sse. Exact
   finding, exact scope.
2. **Review-fix: fullcore drain-deadline determinism** (task `…899ed`,
   `16a15d8`). The drain loop selected on `ctx.Done()` AND the deadline
   tick; since `Pool.Start` returns only after cancellation, both were ready
   at deadline and Go's random pick let the example exit 0 silently about
   half the time. The redundant done case is removed (2 lines); a deadline
   hit now always produces the intended fatal. Root build green.
3. **RELEASE.md — the round-2 multi-module release flow** (task `…37be`,
   `41b817b`, reworded to `cd09c1c` by the 07:49 footer fix). 99 lines
   documenting gate sequence, two-phase `--tag`/`--push` with resume,
   disk-derived sub-tag cutting, and the sibling-replace allowlist including
   the nested-module lesson. Spot-verified: `gate_gomod` lives in
   `scripts/lib/release-gates.sh` and is sourced by both `scripts/release.sh`
   and the fixture smoke; the archived v0.1.0 checklist exists at the cited
   path.
4. **VERSION-SURFACES.md — the seven version surfaces** (task `…e4919`,
   `e8a4c95`). Spot-verified: flake `version = "0.2.0"` + ldflags line match
   (flake.nix:37,70); exactly 7 `internal/*/v0.2.0` tags exist; the bump
   order matches what release.sh automates vs leaves manual.
5. **Lint-baseline slice-triage: wrapcheck AND varnamelen to zero** (task
   `…ced7ba`, `eaf73a9` for docs + `bebc35a`/`fc495e8` daemon commits
   carrying ~95 renames and the `.golangci.yml` policy). Reproduced this
   pass: full-config root run reports wrapcheck 0 and varnamelen 0; the
   AGENTS.md lint bullet records the policy; ~340 lines of pure renames
   landed across cmd/ + internal/ (insertions == deletions in both daemon
   commits).
6. **In passing, re-verified by this pass**: all seven sub-module gates
   (build + vet + test, disk-derived loop) green at HEAD — covers the 07:49
   report's open ask to re-run modules after the postgres go.mod bump.

## b) PARTIALLY DONE

1. **The release script has still never been executed.** Both release-doc
   tasks verified claims by line-reading `release.sh`; the 07:49 report
   re-flagged it and it remains true: the flow is documented, not rehearsed.
   The forked lineage below makes a fixture rehearsal MORE valuable, not
   less.
2. **"Zero findings" is config-relative.** Under the project's full config
   the claim holds; a bare `--enable-only varnamelen` diagnostic (which does
   not apply the full linters config) reports 2 raw sites:
   `cmd/tq/agentpool.go:278` (`o`) and `cmd/tq/poolconfig.go:26` (`f`) —
   both lines predate the task (blame 2026-09-08/09-10 00:12). Not a gate
   break (advisory class, zero under the real config), but anyone verifying
   scoped runs gets a different answer than the close-out claims. Filed for
   rename-or-document.
3. **The fullcore deadline fix has no automated test** — verified by hand
   (10/10 by the task's own report), and the planned hermetic fullcore smoke
   (TODO_LIST 06-25 section) doesn't exist yet, so the determinism pin is
   still tribal knowledge.
4. **Docs cross-linking**: the two release docs referenced no living doc and
   each other not at all. Fixed THIS pass (RELEASE.md ↔ VERSION-SURFACES.md
   cross-links + AGENTS.md pointer in the multi-module paragraph) — closed
   here so it needed no new task.
5. **The 18-41 report's designation as "fully annotated" is false** — this
   pass struck 3 more verifiably-shipped items (binary-runs check,
   free-port smoke, theme toggle) and left ~30 brainstorm items open; none
   of the six 2026-09-07 reports qualifies for archiving (all still carry
   open items).

## c) NOT STARTED (backlog the window skipped — verified still unchecked)

- The whole "Dogfood round" section (cwd-dependence sweep, module-eval PATH
  assert, `checkProjectsDir` relaxation, skip-log dedup, `tq doctor` PATH
  warning, dead-pool alert, dogfood smoke, /tmp evidence archival,
  `tq pool-health`, worktree-per-agent doc, session-close bridge).
- CI-hardening block from the 04-09 report: test-windows fix
  (CHANGELOG claims it fixed — TODO item not yet ticked, see d4),
  gosec config, CI-state gate (`check-ci.sh`), line-length gate, job
  summaries, required-checks proposal.
- The nix CI gate is still runner-red (FOD hash mismatch; FEATURES.md
  honestly marks the CI row 🔴 BROKEN) — untouched by this window.
- Owner-blocked set — unchanged and correctly BLOCKED, now plus the three
  lineage questions this window generated (see g).
- The window spent itself entirely on its five items; nothing else picked
  up. Consistent with the done-prompt contract.

## d) TOTALLY FUCKED UP

1. **The footer-fix reword forked the lineage and left origin/master
   permanently behind.** Local master (14 commits) descends from reworded
   `cd09c1c`; `origin/master` still tips at bad-footer `41b817b`; v0.2.0 and
   all seven `internal/*/v0.2.0` tags descend from the remote side and are
   no longer ancestors of master. The 07:49 report documented this honestly,
   but the fork is now the workspace every future session inherits: pushes,
   `git describe`, and release-gate tag checks all have untested behavior
   under it. Resolution is owner-gated (agents never push).
2. **Ticket↔content attribution split by the auto-commit daemon.** The lint
   triage's ~95 renames and config change ride daemon `chore:` commits
   (`bebc35a`, `fc495e8`) with NO Task-Queue-ID; the footer-carrying commit
   (`eaf73a9`) contains only docs. Anyone auditing task `…ced7ba` from git
   finds a 2-file commit claiming a code change it didn't carry. Same class
   as 06-25 d2's phantom-footer problem: the queue's audit trail and git's
   diverge whenever the daemon sweeps mid-task.
3. **CHANGELOG went dark for the window's own deliverables.** The fullcore
   deadline fix, both release docs, and the lint triage had no entries until
   this pass backfilled them (Fixed/Added/Changed under [Unreleased]). The
   06-25 report flagged this exact gap for the previous window; it repeated
   within hours.
4. **Three TODO_LIST items are stale-done**: line 98 (windows test — fixed
   per the red-master-trio CHANGELOG entry), line 99 (release-gates smoke
   identity — `-c user.email/-c user.name` present in
   scripts/smoke/release-gates.sh:40,44), line 100 (x/text bump — postgres
   module sits at v0.41.0). Done work reading as open work corrupts the
   harvest surface. Append-only discipline this pass forbids fixing them
   in-place; filed for a tick pass.
5. **The 06-25 continuation item ("annotate + archive the six 2026-09-07
   reports") aged without progress** until this pass, and this pass could
   only partially advance it: 3 strikes in 18-41, zero archives (none
   qualify — all six carry open items). Point-in-time reports accumulate
   faster than the per-pass annotation cadence resolves them.

## e) WHAT WE SHOULD IMPROVE

1. **Rehearse release.sh on a fixture before the next real release** — and
   specifically under the forked lineage (d1), since tag-ancestry assumptions
   (`git describe`, `rev-list <tag>..HEAD`) are now untested territory.
   The docs are good; the flow has never run.
2. **Make DONE verdicts update TODO_LIST in the same commit as the work** —
   the stale-done trio (d4) came from fixes landing under one work stream
   (red-master trio) while their TODO items waited for a docs pass. The
   close-out contract already makes reports append next items; verdicts on
   EXISTING items should follow the same path.
3. **Baseline counts for lint claims.** The wrapcheck/varnamelen close-out
   had no before/after number (its own report says so). A checked-in
   per-linter baseline (or a CI job-summary table) makes "triaged to zero"
   falsifiable and shows the next slice's size (goconst/mnd/paralleltest are
   each ≥50 by the caps).
4. **Footer-integrity mechanics** (07-49 items, still unbuilt): a commit-msg
   hook requiring exactly one matching Task-Queue-ID on non-daemon commits,
   and a queue-side commits-per-ID view, would have caught both the wrong
   footer (d1's origin) and the daemon attribution split (d2) at commit time.
5. **Scoped lint verification must use the full config.** `--enable-only`
   diagnostics disagree with configured runs (b2); verification claims should
   name the invocation, or the two raw sites should be renamed so scoped and
   full runs agree.
6. **Archive verdicts need the every-item check, not optimism** — none of
   the six 09-07 reports qualified; the cadence item should say "annotate +
   RE-EVALUATE for archive", since "archive" was the headline and "0
   archived" the result two passes running.

## f) NEXT THINGS (appended to TODO_LIST; existing backlog not duplicated)

1. Rename the two varnamelen raw sites (`o` cmd/tq/agentpool.go:278, `f`
   cmd/tq/poolconfig.go:26) or extend the ignore list, so scoped
   `--enable-only` diagnostics agree with full-config runs (2 vs 0
   discrepancy found 08:xx this pass).
2. Pin the fullcore drain-deadline determinism mechanically: extend the
   planned hermetic fullcore smoke to exercise the deadline path N times
   (16a15d8 was hand-verified only).
3. Lint-baseline slice-triage round 2: goconst (≥50), mnd (≥50),
   paralleltest (≥50), testpackage (30) — same triage-or-policy discipline
   as wrapcheck/varnamelen, never mass-fix.
4. Record a per-linter findings baseline (checked-in file or CI job
   summary) so shrink claims are measurable; backfill the current root count
   (~380 across 29 classes, wrapcheck/varnamelen 0) as the starting point.
5. Execute `scripts/release.sh` against a throwaway fixture repo, including
   the gates, `--tag`, and sub-tag cutting on fixture modules.
6. Audit `scripts/release.sh` + `scripts/lib/release-gates.sh` for
   tag-ancestry assumptions broken by the forked lineage (git describe,
   merge-base, rev-list patterns); add a doctor/release-gate warning for
   release tags not reachable from master.
7. RELEASE.md: add a tag-ancestry/rewrite section (what the flow assumes;
   the 2026-09-10 reword fork as the live example) and document the
   `--push` failure path when tags are already cut but the push failed.
8. Smoke sub-tag cutting end-to-end: cut a throwaway
   `internal/<mod>/vX.Y.Z` tag on a fixture and resolve it via local module
   resolution — the doc's most operationally risky claim.
9. `release.sh --push`: automate or one-command the "confirm CI green on
   refs/tags/vX.Y.Z" manual step (gh run list/watch on the tag ref).
10. Commit-msg hook (via scripts/install-pre-commit.sh): require exactly one
    `Task-Queue-ID` trailer matching the assigned-ID shape on non-daemon
    commits; reject an ID already used by another commit's trailer or a
    report filename.
11. `tq facts`/`tq show`: a commits-per-Task-Queue-ID view so wrong or
    duplicate footers surface mechanically instead of by reviewer eyeball.
12. check-status-index.sh family: cross-check each report's filename ID
    against some commit trailer (or mark report-task artifacts) — the f26
    three-ID cluster would have been flagged mechanically.
13. Extend check-status-index.sh: flag filename-date vs report "Written"
    line drift.
14. CHANGELOG audit: confirm the v0.2.0 section cites no commit SHAs from
    the pre-reword lineage (they would dangle).
15. Sweep non-status docs for references to `41b817b` (today only
    docs/status hits) — keep it that way; the remote-only bad-footer twin
    must not leak into living docs.
16. CI check: confirm no workflow assumes fast-forward from `origin/master`
    while the local/remote divergence (14/1) stands.
17. Mark the stale-done TODO items [x] with DONE verdicts after
    re-verification: windows-test fix, release-gates identity, x/text ≥0.39
    (all three fixed per the red-master-trio CHANGELOG entry; verified this
    pass at HEAD).
18. Attribution convention for daemon-swept work: when the auto-commit
    daemon folds a task's working-tree changes into a footer-less `chore:`
    commit (the lint triage: content in bebc35a/fc495e8, footer-only
    eaf73a9), how is ticket↔content attribution recorded? Draft a proposal
    in docs/planning/ (options: agents commit before verification ends;
    daemon carries a pending-task footer; queue-side mapping task→range).
19. Docs-health cadence: continue the annotation pass over the 2026-09-06
    and 2026-09-08/09 report batches (18-41 gained 3 strikes this pass; none
    of the six 09-07 reports qualified for archiving — all carry open
    items); re-evaluate archive eligibility per report instead of assuming.
20. Status-index maintenance: the README index grows ~10 rows/day — add a
    monthly digest row or an archived/-sweep cadence check so the index
    stays scannable.
21. Draft the history-rewrite policy line for AGENTS.md: when (if ever) a
    review-demanded reword of an already-pushed commit is allowed, and what
    must be recorded when it forks the lineage (the 07:49 incident as the
    case study).

## g) QUESTIONS FOR THE OWNER (from the window's own fallout)

1. **Origin reconciliation**: `origin/master` still tips at bad-footer
   `41b817b` while local master holds the corrected 14-commit lineage. Do
   you want a one-time `--force-with-lease` push to replace it (making the
   queue cross-reference resolve remotely), or is origin append-only and
   permanent divergence accepted? Agents never push, so this is owner-run
   either way.
2. **Tag-lineage fork**: v0.2.0 and all seven `internal/*/v0.2.0` tags
   descend from the remote (pre-reword) side and are no longer ancestors of
   master. Acceptable for release tooling and the module proxy (which only
   needs tag→commit), or should release.sh/release-gates/doctor be hardened
   against forked lineages before the next release is cut from the new
   lineage?
3. **Canonical ID for the f26 cluster**: the release-doc work now exists
   under three IDs — `a3864d95` (original commit + 06-55 report),
   `b141f022` (reviewer-assigned, on `cd09c1c` and the 07:49 report's
   footer), `c3919ce4` (the 07:49 report's filename). Which is the queue's
   canonical entry, and should the others be merged or marked duplicates
   queue-side?

---

_Point-in-time snapshot 2026-09-10 08:25 CEST. Re-verify before treating any
claim as current — multiple concurrent-agent sessions are active on this
repo._
