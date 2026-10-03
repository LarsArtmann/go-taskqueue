# Done-Prompt Window Report — 2026-10-04 00:52 — review fixes + sort chip + httpauth seam

**Window:** 2026-10-03 02:53 → 2026-10-04 00:36 CEST (five dispatched tasks + the
immediately-following lockout de-sleep close-out 00-36, read for cross-evidence).
**Tasks:** 000001a0ff268ac2823ba481f13700000000 (STATUS column widen review fix),
000001a0ff268a2e81404f9e529000000000 (MaxTicks re-dispatch), 000001a0ff268ad0970e1f0db39600000000
(report-accuracy fix), 000001a103c819ec836a3128cddc00000000 (sort filter chip), 
000001a103d5d6871e9bd7ccf85f00000000 (httpauth extraction).
**Method:** every claim below verified against `git log`/`git show`, the five close-out
reports, TODO_LIST.md, CHANGELOG.md, FEATURES.md, the queue journal (9,178 facts, TQ_DB
read-only), and a fresh `heal-daemon-sweep.sh --self-test` run at HEAD.

## a) FULLY DONE

1. **STATUS column widen (task …8ac2, commit `47a6515d`, healed into the task footer).**
   `cmd/tq/tasks.go` format strings `%-10s` → `%-14s` (header + rows), fixing the
   reviewer's medium finding: `running 9m59s` (13 chars) no longer pushes
   PROJECT/TYPE/ATT/NOTBEFORE/LAST ERROR right on healthy RUNNING rows. cmd/tq module
   gate green per close-out; diff verified at HEAD (2 insertions, 2 deletions, one file).
2. **MaxTicks re-dispatch (task …5290, report 02-57).** Zero-diff verification window:
   work had already landed in `5e8bebc4` (MaxTicks=5 on both 560×200 metric AreaCharts,
   templ-components v1.19.4, TODO row 119 `[x]` — verified still `[x]` at HEAD). This
   dispatch contributed the independent verification (vendored prop present, both call
   sites carry MaxTicks, webui tests green) and the canonical-report pointer. Third
   zero-diff re-dispatch of the night — see §e.
3. **Report-accuracy fix (task …3960, commit `8bd7b054`).** The 02-14 report's false
   "one-commit HEAL OK" claim corrected at three points (a4/a5/d1) to the real topology:
   code landed in the daemon commit `6f561aa2` which DOES carry the task footer; the
   claimed heal never stuck. Finding verified at HEAD before editing (dangling `8af9c70e`
   confirmed, anchor text located). Doc-refs + status-index gates green.
4. **Sort filter chip (task …cddc, work commit `42a38886`, healed from two footer-less
   daemon sweeps).** Active `?sort=` renders as a removable `sort` chip via `filterChip`
   (internal/webui/fragments.templ:533-535), `clearSort` helper added
   (internal/webui/render.go), clear-all parity for sort-only state
   (fragments.templ:566), `TestSortChip` pinned (internal/webui/filter_test.go:261),
   templ regenerated, TODO row 120 `[x]` with DONE note. Theme duality verified at
   class level: chip reuses `filterChip`'s `dark:` variants; sort-header link verified
   in templ-components v1.16.0 `display/table.templ` (both-theme text + hover tokens,
   `aria-sort`). Full root build+vet+test -race rc=0 per close-out.
5. **httpauth/lockout extraction (task …f850, work commit `82d16be1` + closeout
   `5447c991`, both footer-carrying).** `httpauth.PresentationPolicy` with
   `APIPolicy()` (header+query, cookie never consulted) and `DashboardPolicy(cookieName)`
   (all three channels) replaces webui `presentedToken` and httpapi inline extraction;
   `lockout.DefaultMaxHits/Lockout/IdleKeep/MaxKeys` are the single constant source both
   limiters derive from (webui's bare literals and httpapi's near-duplicate consts gone;
   3 mnd warnings died as a side effect). `TestPresentationPolicyPresented` (9 cases)
   pins precedence + per-policy channel rejection. TODO row 125 `[x]` with DONE note.
   FEATURES.md rows already cite the shared `internal/httpauth` seam — no drift.
6. **Window-adjacent: TestAuthLockout de-slept** (task …3f3f6e, commit `93c7f4ad`,
   report 00-36, landed 6 min after this window's last listed task): fake-clock
   conversion, TODO row 126 `[x]`, root battery rc=0 — which also formally closes the
   httpauth close-out's §b2 open gate ("root full -race not re-run after final edits").

## b) PARTIALLY DONE

1. **"ONE tested implementation" (httpauth task):** the seam is shared, but
   `internal/lockout` itself has NO test files (`go test` → `[no test files]`); its
   mechanics ride transitively on webui/httpapi suites. The extraction item is
   satisfied; the in-module suite is not built. → TODO row appended.
2. **Sort chip "visual verification" is class-level, not eye-level:** every visible
   token has a `dark:` counterpart (verified in vendored source), but no browser
   screenshots in either theme exist. The close-out itself flags this. → guard row +
   owner question appended.
3. **The 02-14 report accuracy fix chose amendment over heal** (correct call at ~15
   commits of stack depth), but the underlying question — WHY the original heal printed
   HEAL OK while the rewrite did not survive — remains unexplained (see §d2/e).

## c) NOT STARTED

1. Both carried-open MaxTicks residues: browser/DOM tick-count assertion, and the
   ~25 KB app.css prune-vs-fresh-scan drift investigation (standing rows/f-sections
   since 02-46; untouched this window — the re-dispatch correctly did not re-open them).
2. The standing backlog the window skipped by design: re-dispatch suppression gate
   (blocked owner rows), status-index archive sweep (live-rows still ~176 vs threshold
   100), 401/429 body-shape ruling, `?token=` strike-path pin (TODO row at line 128,
   untouched, correctly out of the httpauth task's scope).
   ~~401/429 body-shape ruling, `?token=` strike-path pin~~ — both closed later this
   night: strike pin + lockout-over-enqueue landed as `d8809930` (row 128 `[x]`),
   body shape aligned via `6369ea75` (row 129 `[x]`); see the 01-06/01-17/01-12/01-29
   reports of 2026-10-04.
3. Nothing the window was asked to do was skipped.

## d) TOTALLY FUCKED UP

1. **`heal-daemon-sweep.sh --self-test` is RED at HEAD: 5 ok / 7 FAIL — re-verified by
   this report at HEAD (00:50).** Failures include `mixed-range heal should succeed`,
   `tag on base should not block the heal`, and a bizarre
   `line 388: scripts/heal-daemon-sweep.sh: No such file or directory` self-path
   resolution error. The 00-07 close-out flagged this and moved on; a day later the
   heal tool's own safety net is still untrusted while every window leans on the heal
   path. This is the window's worst standing damage. → TODO row appended (top).
2. **The heal tool keeps producing outcomes its operators don't fully verify:** the
   02-14 "HEAL OK that never stuck" (root-caused into an amended report, not a fixed
   script), the 00-07 window's `--from` exclusive-range mistake (footer landed on the
   TODO commit, the swept webui commit stayed footer-less; recovered via soft reset),
   and the httpauth window's two-footered-commit topology accepted without a
   per-commit file audit. Four windows in a row stumbled on the same tool. The script
   needs: post-rewrite self-verification (HEAL VERIFIED vs HEAL CLAIMED), documented
   `--from` exclusivity, and a green self-test.
3. **Daemon-sweep race cost repeated in every task of the window** (three heals across
   five tasks; the 00-36 close-out was swept 12 seconds ahead of its commit). No data
   loss — the heal mops — but the standing daemon-mute row remains unbuilt and each
   heal is a history rewrite under concurrent agents.
4. **Queue-side: an env-streak dead-letter at 00:43** (fact 9177/9178): task
   `000001a103c819c9b535101f25d700000000` died because /home/lars/projects/SystemNix
   has an uncommitted `scripts/check-buildcache-known-parity.sh` — the breaker burned
   the attempt after streak 3. Correct breaker behavior (the 169-claim-loop lesson
   working), but the work item is now dead-lettered on a foreign repo's dirty tree;
   the item needs rescue once SystemNix is clean. Reported, not fixed (out of repo).
5. **Zero-diff re-dispatch burn continued** (MaxTicks re-dispatch = the night's third).
   The suppression-gate rows exist and are blocked on the owner; each occurrence costs
   a full session window.

## e) WHAT WE SHOULD IMPROVE

1. **Fix the heal self-test before wiring it anywhere.** A rail whose fixture suite
   fails 7/12 cannot gate anything; the two open "wire/grow the self-test" rows both
   assume a green base that does not exist today.
2. **Heal-script output honesty:** HEAL OK must mean "post-rewrite topology re-derived
   from git", not "msg-filter exited 0". A final `git log <range>
   --format='%h %(trailers:key=Task-Queue-ID)'` inside the script (or in the AGENTS.md
   post-heal convention) would have caught both the 02-14 and 00-07 incidents.
3. **Class-level theme-duality verification should be mechanized** (grep rendered
   fragments for visible-text classes lacking `dark:` counterparts, allowlist-pinned),
   retiring the recurring "verify X against theme duality" manual rows.
4. **`internal/lockout` deserves its own fake-clock suite** now that it is the shared
   constant source for both surfaces — currently a limiter regression could pass both
   consumers' suites only by luck of coverage.
5. **Process: verify-after-mutate as a written rule.** The 02-14 fabrication (heal
   claimed, never re-checked) and the 00-07 range mistake share one root: tool stdout
   trusted over `git log` after the fact. One convention line in AGENTS.md beats five
   post-mortems.
6. **Root-gate closure hygiene:** the httpauth §b2 gap ("full root -race not re-run")
   was closed 11 minutes later by an unrelated window's battery run. Windows should
   cite the newest same-HEAD report (the verify-window battery rule already mandates
   this) instead of carrying the gap forward.

## f) NEXT THINGS (routed: actionable rows appended to TODO_LIST.md, deduped against
268 existing open rows; blocked rulings listed separately there)

1. Fix `heal-daemon-sweep.sh --self-test` (7/12 red at HEAD, self-path resolution
   error at line 388) — the heal rail's safety net is untrusted. ← appended
2. Heal `--from` range semantics: document exclusivity or add an inclusive mode
   (00-07 §d1/§e1 incident). ← appended
3. Post-heal trailer-verification convention in AGENTS.md + HEAL VERIFIED output
   contract in the script. ← appended (merged with 2's script half)
4. Render-level theme-duality guard for webui fragments. ← appended
5. `internal/lockout` direct table suite (fake clock, strike/lock/eviction). ← appended
6. Sort chip board-view interplay pin (FilterBar short-circuits differently on the
   board view; clearing sort from board must return to stored-priority order). ← appended
7. Sort-header `aria-sort` over SSE fragment swap (verify the swap path updates it). ← appended
8. FilterBar golden-fragment pin (all five chips + clear-all). ← appended
9. Carried, NOT re-appended (existing rows): re-dispatch suppression (blocked),
   status-index archive sweep, `?token=` strike-path pin, 401/429 body shape,
   LEASE-column ruling, ci-local env probe, lockout clock unification.
10. Not rowed (foreign-repo): rescue the 00:43 env-streak dead-letter after SystemNix
    is cleaned — operator action, lives in the queue not this file.

## g) QUESTIONS (owner only; appended to TODO_LIST.md as BLOCKED rows)

1. **Heal tool policy:** is the red self-test (7/12) + HEAL-OK-that-doesn't-stick class
   worth a priority fix dispatch now, and should `--from` become inclusive (healing the
   named commit) or stay exclusive with a docs fix? The four-incident streak says the
   tool's contract, not its usage, is the problem.
2. **Filter form semantics:** should `apply` preserve the active sort via the hidden
   input (current behavior) or reset sort when the user re-filters? UX-intent ruling
   (00-07 §g2).
3. **Evidence bar for "visual verification" rows:** is class-level `dark:`-counterpart
   verification sufficient to close G2 theme rows, or do you want archived browser
   screenshots (decides whether the guard row in f4 replaces or supplements the manual
   class)?

## h) BAND DRIFT

**None recorded.** The journal (9,178 facts, TQ_DB=/mnt/pool/services/tq/tq.db, read
read-only) contains zero `task.reprioritized` facts — grep across the full fact feed,
including this window's timespan (2026-10-03 02:53 → 2026-10-04 00:52). All priority
movement this window came from the standing ADR-0015 ladder (aging, markers), not from
explicit reprioritization facts. Nothing to account for.

## Verification appendix

- Window commits verified: `47a6515d`, `9ae53407` (report+row), `8bd7b054` (fix body),
  `865a5638` (report+row), `42a38886` (chip body, via heal from `187aa8f2`/`ef6f7fa5`),
  `12877dbf` (report+row), `82d16be1`+`5447c991` (httpauth body+closeout),
  `93c7f4ad`+`0f79f0ec` (lockout de-sleep, window-adjacent).
- TODO rows 119/120/125/126 confirmed `[x]` at HEAD; row 128 (strike-path pin) open.
- CHANGELOG gap filled this pass: sort chip + MaxTicks entries added under [Unreleased].
- heal self-test re-run at HEAD 00:50: `SELF-TEST FAILED (5 ok, 7 failed)` — matches
  the 00-07 close-out's observation, now a day old and still red.
- Journal: zero `task.reprioritized` facts; one env-streak dead-letter at 00:43 (§d4).
