# Done-prompt window report — 2026-11-09 07-36

Window: six task closeouts spanning 2026-11-07 → 2026-11-09 (tasks
…b23689cfad, …f987c4438af, …1778e0fd6a9, …f594f400000, …6fddece79d, …d3a2b200000),
read first-hand from each closeout under `docs/status/tasks/` and verified
against the tree at HEAD 5195cd5f.

## a) FULLY DONE

1. **Dedup-ledger extraction triggers pinned in AGENTS.md** (task
   …b23689cfad, commit 2aee0247 after a daemon-sweep heal): all three
   extraction triggers (doctor pending-listing, executor payload-validation,
   `companion/reads.go` scanInto) live in the Executor-shared-seams bullet
   with the source citation — verified by grep at HEAD (AGENTS.md:298-301).
   Size budget consciously raised 26_400 → 26_800 with both twins re-synced.
2. **Vendor-sync lint-leak regression pin** (task …f987c4438af, commit
   656f90cb): `lint_leak_scan` in `scripts/check-gomod-vendor-sync.sh`
   names the CI 41877954180 go.sum leak class at gate time; self-test
   Case 8 pins leak/clean/absent paths; `--self-test` flag added. Verified:
   5 `lint_leak_scan` hits in the script at HEAD.
3. **docs-gates bridge sweeper shipped** (task …1778e0fd6a9, commit
   9519eb06): `internal/docsgates` (Bridge.Sweep, per-repo `docs-gates run
   --json`, dedup `docs-gates:<repo>:<gate>`, journal watermark
   `docsgates-bridge`, no-checkpoint-on-runner-failure test-pinned), wired
   as `tq agent-pool --docs-gates*`. Verified: package present at HEAD
   (bridge.go + bridge_test.go); shipped in the v0.4.2 CHANGELOG §Added;
   two reviewer-fix follow-up tickets already closed on top (flag-count
   baseline, Cursor-comment + exit-1 minting heals, commits 5b60d273,
   7a7bb39a, f9e6803d). Reviewer finding (task …6fddece79d) dispositioned
   ALREADY-HEALED-AT-HEAD: baseline `agent-pool 69` == actual 69, gate
   rc=0.
4. **v0.4.2 release wave cut, verified, and PUSHED** (tasks …f594f400000 +
   …d3a2b200000): root `v0.4.2` tag + all module sub-tags verified
   complete by the disk-derived module→tag loop (zero missing; CHANGELOG
   §[v0.4.2] present, flake version synced); cmd/tq floor bumped to root
   v0.4.2 (go.mod verified at HEAD); cmd/tq go.sum re-tidied onto proxy
   v0.4.2 hashes (fresh-cache, commit 7755165a, GOWORK=off build+vet
   green); `cmd/tq/v0.4.3` supersession tag cut with a clean-room
   go get/list/build proof; GitHub Release v0.4.2 published
   (--prerelease, verified live via `gh release view` 2026-11-09);
   windows-CI red on the tagged tree cured at HEAD (bridge_test.go
   `//go:build unix`, bb08489e). **Since the closeout was written, the
   owner push landed**: origin/master == c84d2b5a, 25 internal sub-tags +
   root v0.4.2 on the remote (verified `git ls-remote` 2026-11-09).
5. **Bridge exit-1 minting fix** (post-window, 7a7bb39a): the sweeper now
   mints on FAIL gates despite the binary's exit-1 contract, closeout
   f9e6803d.

## b) PARTIALLY DONE

1. **cmd/tq/v0.4.3 supersession is still LOCAL-ONLY**: `git ls-remote`
   (2026-11-09) shows the master push and the 26 wave tags landed but
   `refs/tags/cmd/tq/v0.4.3` is NOT on the remote — the proxy verify
   (`go install …/cmd/tq@v0.4.3`) therefore still cannot run. TODO row 53
   stays open; its BLOCKED note is now narrower than written (only the
   supersession-tag push + verify remain, master itself is pushed).
2. **The orphaned-lineage state persists**: root `v0.4.2` sits on 2f0bea71,
   verified ORPHANED from HEAD (`git merge-base --is-ancestor` fails).
   Consequence now CI-level, not just local: master CI run 42551677517
   FAILS in the test job on `TestDoctorHealthyEmptyDB`
   (tag-ancestry Status:warn, v0.4.2 unreachable). Owner-blocked row 54
   owns the heal call; the test-fixture half is minted as a new row.
3. **Corpus sweep**: newest wave triaged (1 harvest + 1 skip-mark); the
   2381-item historical pile stays deliberately SWEEP-only per doctrine
   (monthly digest ritual owns it; index at 423 live rows > 400, warning
   live on every commit).

## c) NOT STARTED

- Lint-leak WRITER identification and the four sibling follow-ups (writer
  hunt, end-to-end fixture, ci-local `--self-test` flag switch, allowlist,
  go.work.sum scope) — all deliberately ticketed, none started (rows 32-36).
- The annotate-on flip (docs-gates#4) — upstream knob still unimplemented,
  row BLOCKED with the binary-version precondition pinned.
- The monthly digest/archive sweep (index-bloat ritual) — not run this
  window; the 423-row warning stands.

## d) TOTALLY FUCKED UP

1. **NEW: master CI nix job is RED on vendorHash drift** (run 42551677517,
   push of c84d2b5a): the cmd/tq go.sum re-tidy (7755165a) changed the
   module graph but flake.nix's vendorHash was not refreshed — hash
   mismatch in `go-taskqueue-0.4.2-go-modules.drv`. The Known-Issue ritual
   (`nix build .#checks.x86_64-linux.vendor-hash` → copy `got:`) was
   skipped by the release-completion session. Minted as a TODO row (top of
   the harvest section); NOT fixed here (hard scope: docs only).
2. **Master CI test job RED on tag-ancestry** (same run): the lineage
   state that was a local test nuisance is now a red master CI job. Not a
   regression introduced by the window (the orphan was born at the wave
   cut), but the window's push made it public.
3. **heal-daemon-sweep.sh retitled a whole 14-commit foreign span**
   (…d3a2b200000 §d1): `--retitle` applied MY subject + footer to 13
   foreign daemon chores; recovered via manual commit-tree rebuild
   (tree-identical verified). Tool gap + operator error (script not read
   first); hardening trio minted as TODO rows.
4. **ci-local skip debt materialized** (…6fddece79d §d): adding four
   agent-pool flags without running the cheap `check-flag-count.sh` left
   the pre-push gate red for a day and cost a reviewer round-trip
   (23dd25e7 healed externally).
5. **Daemon-sweep races are structural**: at least four of the six sessions
   paid heal loops (fork records in the …1778e0fd6a9 and …d3a2b200000
   §d's). No data loss, no pushed-history rewrites.

## e) WHAT WE SHOULD IMPROVE

1. **The VendorHash ritual needs a mechanical gate in the release path**:
   both Known Issues (vendor drift, go-directive lockstep) are
   convention-enforced; the release-completion session violated one within
   days. A post-tidy check inside the release/TODO recipe (or ci-local
   re-run before the completion session's final commit) would have caught
   it.
2. **Release-completion sessions should end with a master-CI probe**: the
   push landing is exactly when master CI flips; one `gh run list` check
   after the owner push (or at the next done-prompt) closes the drift
   class this window found.
3. **heal-daemon-sweep.sh is now the single largest recurring tax**: three
   of six sessions hit it; the dry-run/--tip-only/tag-naming hardening and
   the commit-tree recipe rows own the fix.
4. **Cheap-gate discipline**: `check-flag-count.sh` is seconds-scale and
   was skipped under a known risk; the tq-edit-advice hook row for
   `cmd/tq/*.go` flag edits (minted by the fix closeout §f1) is the
   right shape.
5. **Stale BLOCKED notes need a refresh path**: row 53's blocker is now
   narrower than its text (master pushed, supersession tag not); the
   tick-claims rule forbids rewording, so the refresh happened here in the
   report — a convention for annotating stale BLOCKED notes (dated inline
   update without rewording the ask) is worth a ruling.
6. **AGENTS.md headroom is now ZERO** (26800/26800 after the STATUS-line
   refresh): the standing compress-to-~24k row is urgent; every
   AGENTS-touching session is one byte from a red guard.

## f) NEXT (up to 50; the deduped top set harvested to TODO_LIST)

Harvested (8 new rows, deduped by domain-noun greps against the 784 open
rows):

1. Fix master CI nix vendorHash drift (flake.nix `got:` refresh). <!-- fresh:skip: harvested as TODO_LIST row 1802; freshness matcher rejects it because the token set is all-DF>=8 generic -->
2. Doctor tag-ancestry test fixture/exemption so
   TestDoctorHealthyEmptyDB stops failing master CI + local module runs.
3. heal-daemon-sweep.sh hardening trio (dry-run, --tip-only, tag-naming
   refusals).
4. Pin the commit-tree lineage-rebuild recovery recipe/script.
5. scripts/check-subtag.sh — reusable clean-room `<module>@<tag>`
   verifier.
6. Investigate `tq ask` "task: not found" TQ_DB routing in dispatched
   sessions.
   <!-- fresh:skip: harvested as TODO_LIST row 1807; matcher rejects the token set as generic -->
7. BLOCKED (owner): retract the poisoned cmd/tq/v0.4.2 via a v0.4.4, or
   accept v0.4.3 supersession.
8. BLOCKED (owner): rule the live-pool docs-gates bridge repo list +
   interval.

Standing rows already on the list (cited, not re-minted): lint-leak
writer + 4 siblings (rows 32-36); bridge follow-ups `[]`-json,
frozen-sweeper coverage, threshold-rail enforcement (rows 46-48); docsgates
Sweep refactor + comments-are-claims audit; release.sh supersession WARN +
VERSION-SURFACES window note; docs-gates#4 flip (BLOCKED upstream);
lineage reconcile (row 54, BLOCKED owner); wave completion (row 53, push
residue); AGENTS compress-to-~24k; UNBLOCKED-OWNER-GATED hard-gate
promotion; index-bloat digest ritual. Additional report-only candidates:
release.sh `--supersede <module>` flag (fold into the row-59 scope),
re-run the full cmd/tq module gate on a healed lineage, confirm windows CI
green at next push, extend check-prompt-const-folds re-check to push/CI
time, AGENTS flag-count Known-Issue one-liner, baseline-parse `--self-test`
row for check-flag-count.sh, ALREADY-HEALED disposition convention,
ci-local standalone flag-count invocation.

## g) QUESTIONS (up to 3)

1. **Retract the proxy-poisoned cmd/tq/v0.4.2?** It installs the v0.4.1-era
   sums; `cmd/tq/v0.4.3` supersedes it but nothing steers consumers away.
   Shipping `retract v0.4.2` in cmd/tq/go.mod needs a v0.4.4 — worth it,
   or supersession-only? (Rides the BLOCKED TODO row.)
2. **Docs-gates bridge production scope**: which repos under
   `--docs-gates`, at what interval? (Rides the BLOCKED TODO row; the
   sweeper is otherwise production-ready.)
3. **Stale-BLOCKED-note convention**: when a BLOCKED row's blocker shrinks
   (row 53: master now pushed, only the supersession tag remains), may
   future sessions append a dated blocker-status update to the row without
   counting it as a reword, or should the row stay frozen and the report
   carry the delta (what this report did)?

## h) BAND DRIFT

none recorded — the production journal (read-only sqlite probe,
`facts` table) contains ZERO `task.reprioritized` facts, matching the
2026-11-07 window report's finding ("zero … ever"). No priority moves to
account for in this window.
