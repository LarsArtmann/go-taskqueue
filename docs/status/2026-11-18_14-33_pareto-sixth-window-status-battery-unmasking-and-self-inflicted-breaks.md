# Sixth window — battery unmasking, self-inflicted breaks, and the /tmp wall

- **Date:** 2026-11-18 14:33 CET (window: ~12:25–14:33, same-day continuation of the 13-27 fifth-window report)
- **Repo:** go-taskqueue · master · owner demand: full status NOW, then WAIT

Batch: backlog-pareto master-plan continuation (docs/planning/2026-11-17_19-53_backlog-pareto-master-plan.md).
This report covers the work AFTER the fifth-window report was committed.

## a) FULLY DONE

1. **T36 closed + both pushes green on origin**: cure span green on run
   **44294443292** (8/8 jobs, job-level `gh run view` receipts); the second
   span `02e657e2..027f2c10` green on run **44302476463**. Ticks citing the
   green run landed (rows 1687/1709/1385).
2. **16 TODO rows ticked across the window** — every one evidence-grepped
   against code first: T09/T10/T11 cure rows (105, 1825, 1835, 1817, 101,
   1872+1914 duplicate pair, 1411), T12/T13/T14 rows (1809, 1811, 1832,
   1834), T17 (1843), T18 (1828), plus the three CI-red rows.
3. **T12 — budget-twin drift class killed by GENERATION** (`c16a808d`):
   check-agents-size now DERIVES its budget from `agentsDocMaxBytes` (loud
   parse-fail exit 2; `AGENTS_BUDGET_BYTES` stays the test override). No
   second budget exists; drift is structurally impossible.
4. **T14 — check-agents-size --self-test** (`1656e384`): 5-check battery +
   headroom delta print + ci-local wiring. The battery caught my own fixture
   byte-arithmetic on first run.
5. **T13 — pre-edit size rule in the session loop** (`5e44dd68`):
   tq-edit-advice computes LIVE size/budget/headroom at hook time; the hook
   fired on my own AGENTS.md edit minutes later (dogfooded).
6. **AGENTS.md load-flaky phrasing drift cured** (`0f977393`): restored the
   gate-pinned per-test bullets compress-only (27,598/27,600 B) after
   check-verify exposed what check-ci's early abort had masked.
7. **T17 — shared stub-agent smoke lib** (`bb6f9714`): prologue + FILE-ONLY
   `stub_verdict` rail single-homed in scripts/smoke/lib/stub-agent.sh; four
   smokes green with a prebuilt TQ_BIN; lib --self-test green; ci-local
   wiring.
8. **T18 — crush_smoke_routed_run** (`467aa935`): the three hand-rolled
   routed-turn runners unified with knobs (BIN/TIMEOUT/PROMPT/PIN_FILES) +
   a --self-test over a FAKE crush binary; memory smoke (heaviest user)
   live-green.
9. **T20 — exec-bits gate** (`071223cc`): check-smoke-exec-bits.sh pins every
   `scripts/**/*.sh` executable (self-test: summary pin + 644-negative +
   755-positive); one-time chmod sweep of five 644 libs; guard-wiring 98/0.
10. **Crush-drift row MINTED with A/B proof** (`027f2c10`): route-M7/pool
    smoke reds are PRE-EXISTING — a scratch worktree at the pre-T18 parent
    fails identically; suspect = the 2026-11-17 21:02 system profile (new
    crush v0.101.0). Memory smoke green because its retries are
    idle-exit-tolerant by design.
11. **Fifth-window report** committed + indexed (`f152109f`); docs-gates
    battery went PASS at HEAD once the report existed (freshness gate).
12. **Fixture-attribution cure** (`5df7d4cd`): 10 fixture heredocs pinned
    (4 mine from the lib restructures + 6 pre-existing battery-masked);
    gate 100/100.
13. **Flag-count baseline re-pinned** (`6ff112a1`): serve 11→13 naming the
    health-dashboard alert-relay flags (--alert-url/--alert-api-key,
    ADOPTED 2026-11-17 per the AGENTS Relation row); gate 24/24 green.
14. **Junk purged** (`ae57e3e6`): work.txt (stub junk the daemon swept into
    git during a broken-script window) removed.
15. **T15/T16 disposition**: confirmed owner-gated (sudo on evo-x2, "an
    agent cannot execute this") — left BLOCKED, documented in both reports.

## b) PARTIALLY DONE

1. **T02 — the full-green ci-local receipt**: FOUR battery runs, each one
   unmasking a previously-hidden red the moment the earlier leg stopped
   aborting:
   - run 1: check-ci red (stale origin red) → cured by the push + green run;
   - run 2: check-verify red (AGENTS load-flaky phrasing drift) → cured
     `0f977393`;
   - run 3: fixture-attribution red (10 unattributed heredocs) → cured
     `5df7d4cd`; then flag-count red (serve 11 vs 13) → cured `6ff112a1`;
   - run 4: died at the clean-room consumer compile with
     **ENOSPC — /tmp tmpfs 100% full (48G)**.
   Deepest receipt: 210 ok-countable lines. The battery has NOT completed
   green at HEAD all day; the remaining blocker is host disk, not the repo.
2. **Crush-drift row**: evidence complete, cure undecided (see §g2).
3. **Route/pool smokes**: red at HEAD (pre-existing, A/B-proven), blocking
   the ci-local smoke tail until the drift row is cured.

## c) NOT STARTED

- **T19** shared negative-fixture helper for gate self-tests.
- **T21/T22** webui SSE 58-row triage + first 5 keep-set pins.
- **T23** commit-msg pin batch (first 8).
- **T24** erraudit sibling session (push 4 commits, close #12/#13/#14) —
  other repo.
- **T25/T26** go-cqrs-lite archive ritual (187 reports triage + execute) —
  other repo.
- **T27/T28** IO-review in-repo rows (counter-fold verify, claims.cue
  judgment, crush-store growth audit).
- **T29–T32** per plan (not yet re-read in detail — the plan table was read
  through T28 only).
- **T33–T35 + P3 pool dispatch wave**.
- **T15/T16** deploy cadence — NOT executable by an agent (owner sudo).

## d) TOTALLY FUCKED UP (mine, honestly)

1. **The six-line replacement bug**: my first python fixture-patch pass
   REPLACED the `cat >... <<EOF` lines with the pins comment instead of
   inserting after them — six fixture writers deleted across
   check-subtag/tick-claims/calibration/session-start. One damaged smoke ran
   its stub body as live smoke code and wrote **work.txt junk into the repo
   root, which the daemon swept INTO GIT** (needed `git rm`). Caught by
   shellcheck + the live calibration smoke — the gates did their job; I
   supplied the bug.
2. **The invisible blank line**: the repair left a BLANK FIRST HEREDOC LINE,
   so the generated stub began with `\n#!/bin/sh` → `fork/exec: exec format
   error` → three failed calibration runs while the script LOOKED right.
   Only a kept-scratch journal read (facts table: exec format error) +
   `cat -A` exposed it.
3. **The README index-row replacement**: my fifth-window index edit REPLACED
   the fourth-window row instead of inserting above it — permanent-memory-
   loss class, caught by a grep count and restored within one edit. The
   exact failure mode row-84's scripted-doc-mutation rail exists for.
4. **/tmp at 100%**: the battery died with ENOSPC on a tmpfs filled mostly
   by OTHER sessions' fixtures (adminqa*, nix-shell, pnpm-store...) — I did
   not bulk-delete others' scratch, but I also ran four heavyweight battery
   runs into a full tmpfs without a preflight df guard.
5. **Battery-abort blindness (process, not code)**: check-ci's early abort
   meant no battery had COMPLETED at HEAD for days — three real local reds
   (AGENTS phrasing, fixture pins, flag baseline) sat behind it, compounding
   into today's four-run whack-a-mole.
6. **Daemon-sweep train**: five+ sweeps folded my work mid-flight (T17,
   T18, pins, junk work.txt) — all healed via the exactly-mine verify +
   soft-reset folds, but the churn made the A/B state confusing twice.

## e) WHAT WE SHOULD IMPROVE

1. **Python doc-patch harness invariant**: every patch must carry the FULL
   original line + the insertion, plus a per-patch "no line lost" assertion
   (line count AND the specific command line still present). Today's §d1/d3
   are the live evidence for row-84's scripted-doc-mutation rail.
2. **Stub-heredoc invariant gate**: generated executables must have
   `#!/bin/sh` on the FIRST body line — a check-script-syntax or
   fixture-attribution extension would have caught the blank line at gate
   time instead of at fork/exec time.
3. **Battery-to-completion discipline**: when the tree is the subject under
   test, run ci-local with CI_CHECK=off rather than accepting a check-ci
   early abort as "the battery ran". Masked reds compound (§d5).
4. **Battery preflight disk guard**: a df threshold check before heavy legs
   (the battery already probes GOCACHE/GOROOT — extend it to /tmp free
   space on tmpfs hosts).
5. **go-build cache pruning ritual**: 482G on a 723G disk at 94% — a
   monthly `go clean -cache`-shaped ritual (or aged-entry prune) row.
6. **Fixture-attribution block-count pin**: deleting a heredoc "fixes" the
   gate silently (94 vs 100 taught me) — pin a non-regression floor or
   per-file minimum block count.
7. **A/B scratch-worktree classification** (mine-vs-preexisting) worked
   three times today — keep it as the standard move before blaming refactors.
8. **Wire crush-server.sh --self-test into ci-local** (stub-agent lib is
   wired; the crush lib's new self-test currently runs on demand only).
9. **Judgment-call honesty held**: route/pool reds were NOT papered over;
   T02 got no false green — keep that posture.

## f) NEXT (ordered, ~50)

1. /tmp emergency: identify the 48G tmpfs fill owners (other sessions'
   fixtures) and free space WITHOUT nuking live scratch — host act.
2. Prune the 482G go-build cache (aged-entry prune or `go clean -cache`),
   then re-baseline disk.
3. Battery run #5 to completion → the actual T02 green receipt.
4. Push the local span (`5df7d4cd`, `ae57e3e6`, `6ff112a1`, this report) —
   timing window opens ~10 min after commit.
5. Watch CI green on that span; tick any residual CI-red rows.
6. Cure the crush-drift row (§g2 ruling → route M7 + pool probe fix).
7. T19 shared negative-fixture helper.
8. T21 SSE 58-row triage into keep/pool/obsolete.
9. T22 SSE keep-set first 5 pins.
10. T23 commit-msg pin batch (first 8).
11. T24 erraudit sibling session (push 4 commits; close #12/#13/#14).
12. T25 go-cqrs-lite archive triage (187 reports).
13. T26 archive ritual execute + digest row.
14. T27 IO-review in-repo rows (counter-fold + poolhealth field).
15. T28 IO-review claims.cue + crush-store growth audit.
16. T29–T32: re-read the plan table rows precisely, then execute.
17. T33–T35 + P3 pool dispatch wave.
18. Apply the §g2 ruling as an AGENTS.md line (battery-to-completion) when
    budget allows — 2 B headroom means a paired compress.
19. Apply the §g1 ruling (archive-vs-routed) to the digest ritual.
20. Confirm /var/tmp/pareto-battery as the standing home (§g3) — already
    used for every receipt this window.
21. Index-bloat 417/400: run the ratified monthly ANNOTATE+ARCHIVE digest
    (advisory fired all window).
22. Add `— BLOCKED:` markers to the 19 UNBLOCKED-OWNER-GATED advisory rows
    the todo gate keeps listing.
23. Fix the checkbox-less rows near the old CI-reds block (pre-existing
    malformation: lines missing `- [ ]` prefixes).
24. Land row-84's scripted-doc-mutation rail (convention + guard sketch) —
    today's incidents are the cited evidence.
25. Small heal-script fix (row 1567): print the amend recipe in the same
    output when the terminator `--from` also refuses.
26. Wire crush-server.sh --self-test into ci-local.
27. Extend a self-test to exercise the amend-fallback exit (row-1411 tick
    honesty residue).
28. Battery preflight df guard (new row, from §e4).
29. Stub-first-line-shebang invariant (new row, from §e2).
30. Fixture-attribution block-count floor (new row, from §e6).
31. go-build cache prune ritual row (from §e5).
32. accuracy pass over the 6 pre-existing pins I authored (gate enforces
    existence, not accuracy).
33. Re-read T29–T32 plan rows verbatim before executing them.
34. Re-run scripts/test-cmd-tq.sh once (TestAgentsDocSizeGuard over today's
    AGENTS edits — the bash gate is green; the Go twin is the same number).
35. check-unticked red-claim matcher (row 1769) — candidate now that gates
    churned.
36. Re-evidence ci-local preflight at post-heal HEAD (row 1744) — battery 5
    covers it if green.
37. Mirror-baseline row: the 2 NEW HIDDEN cross-backend mirrors (11-13)
    still open.
38. generic_return owner ruling still pending (row-level BLOCKED).
39. review-fix re-anchor protocol doc (row 930) — docs/memos candidate.
40. verify fuzz.yml fetch-depth row after the next nightly.
41. Session-close bridge: `tq session begin/close` for this window so the
    review+status tasks mint (or record why not).
42. COVERAGE.md: confirm the two new lib self-tests need no leg rows
    (gate green 35/0 — re-verify after battery 5).
43. work.txt-class junk guard: extend the fixtures-under-/tmp advisory to
    smoke `>` redirects (owner-gated advisory row already exists — feed it
    today's instance).
44. Re-check row anchors next window (content-grep law; TODO_LIST moved
    again today).
45. Watch for the daemon sweeping the pins/fix commits — verify trailers
    after each future push.
46. Dead-SHA gate at HEAD after the next push (new shas cited in this
    report + ticks must not dangle).
47. Keep AGENTS headroom visible: 27,598/27,600 — ANY edit needs a paired
    prune (the hook now says so live).
48. The heal-daemon-sweep oscillation tick (1411) — one residual honesty
    item: the amend-fallback exercise is documented, not self-tested (see
    27).
49. Consider `docs/problems/` note for the tmpfs-full incident class
    (mirrors crush-session-store-growth.md).
50. Then WAIT-cycle: the owner's likely first question is "is T02 green?" —
    the exact remaining gap is /tmp space + one battery run.

## g) THREE QUESTIONS I CANNOT ANSWER MYSELF

1. **/tmp tmpfs is 100% full (48G) — mostly OTHER sessions' fixtures**
   (adminqa*, banner-proof, nix-shell-*, pnpm-store, admin-qa*). Is the
   cleanup mine to do (and if so, how aggressive — age-based? all non-live
   sessions?), or do you want to own the host-level purge? Without space,
   the battery's clean-room compile leg and any /tmp-heavy gate cannot run,
   so T02 stays blocked on this.
2. **Crush-drift row cure**: accommodate crush v0.101.0's idle-exit
   (route-M7 + pool asserts become memory-shape tolerant — weakens a pin)
   or pin the crush input version for smoke hosts (keeps the pin, couples
   deploy)? The row blocks the ci-local smoke tail either way.
3. **Push policy ratification (§g2 carried)**: today's evidence — check-ci's
   early abort masked three real local reds for days, and the cure span
   itself went green on origin. Do you ratify "run the battery TO COMPLETION
   (CI_CHECK=off) before any push" as the standing rule (with a df guard)?
