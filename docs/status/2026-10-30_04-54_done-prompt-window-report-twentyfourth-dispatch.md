# Done-Prompt Window Report — twenty-fourth dispatch (2026-10-30 04-54)

**Task**: 000001a1815c3cc3048aaa3b632900000000 (the done-prompt). This is
the TWENTY-FOURTH dispatch of the re-dispatch storm (17th–23rd already
reported: docs/status/2026-10-28_09-05, _11-02, _17-50, _20-57,
2026-10-29_06-13, _07-29, 2026-10-30_03-35). The window (2026-10-27
07:28 → 2026-10-28 08:03, 15 tasks) has been FULLY LANDED since 2026-10-28
08:03; every prior dispatch re-verified it. This report re-verifies
cheaply, closes the window's last two bookkeeping debts (the two missing
retro closeouts, written alongside this report), and records the verdict
the storm exists to collect. Band-drift query note: the first
`task.reprioritized` query ran against the STALE repo-root `./tasks.db`
(189 facts, last write 2026-10-25 — zero facts) before re-pointing at
`$TQ_DB=~/.local/state/tq/tasks.db`; hazard rowed in §f.

## a) FULLY DONE (all 15 window tasks; each verified by footer commits + closeout report)

1. **Provider rides requeue evidence** (000001a17bfb2332): `tq tasks
   --parked` rows carry `RequeueEvidence.provider`; AGENTS.md bullet
   landed; doctor parked-check names the exhausted provider
   (6bca3238/a732f257 lineage; closeout 07-28, 3/3).
2. **cmdTasks LastFacts dedup** (000001a17c354ee3): one fetch, class +
   provider joins ride one slice (6258a80e); postgres conform proof for
   TestRequeueFactCarriesProvider embedded in the suite doc comment;
   FEATURES sweeper rows → MOVE pointers (closeout 08-30, 3/3).
3. **CI parity** (000001a17c558d2b): check-guard-wiring wired into
   ci.yml (db47c401); devmod shim surfaces tidy failures + caught 6 real
   stale cmd/tq pins at v0.3.3 (d3662957). Item 1 (dead-sha → ci.yml)
   correctly BLOCKED — premise false, 801 dangling cites then, ~964 now
   after the filter-branch explosion (rows 1148/1350).
4. **Dirty-tree preflight parks instead of burning** (000001a17ca0bd7c):
   typed `executor.ErrDirtyTree`, no attempt burn, escalating ladder
   (a0a343eb); BEHAVIOR CHANGE flagged in CHANGELOG (closeout 10-35).
5. **Retro closeout for 000001a141d63a4b** (000001a17ce9fa12): dep-sweep
   e2e batch reported; landed-but-unticked sweep ticked rows 842/989;
   multi-pair fork-record exemption premise corrected + pinned
   (f941edb5); zero-byte statusresume.go daemon artifact removed.
6. **TODO tick-sweep** (000001a17d0e9e8e): rows 1037/1218 ticked via 7
   evidence channels; statsPayload wire goldens (TestStatsPayloadGoldenJSON
   + bare shape); AGENTS.md heal-refusal clause + compress-only prune
   23,885 → 21,587 B (closeout 12-44).
7. **§b-partials sweep + TODO self-dedup** (000001a17d3c674b): ~40
   partial candidates all verified already rowed; 2 minted; 7 reworded
   duplicate pairs consolidated (510cf105/f4b5ca06).
8. **proxy-wait single-sourcing** (000001a17d60fcfb): SMOKE_MODULE/
   SMOKE_VERSION vars + TQ_PROXY_SERVES_FMT; assertions derive from the
   lib (9d4c7525/90f74bbe; closeout 13-51, 8 ok / 0 fail).
9. **Lease-expiry batch-aware done-preflight** (000001a17d734f96d): the
   2026-10-14 1/3-batch zero-cost completion root-caused (footer signal
   fired on one member); footer signal now requires all members
   ticked-or-gone (bc79b3eb) + two test batteries (closeout 00-09).
10. **Fixture-attribution ×2** (000001a17faa3cab): meta-gate extended to
    smoke heredocs (53/53 attributed) + canary v2 accuracy layer,
    `expect-branch.sh` with --self-test (f8e3a10a/c2b3d439).
11. **Canary fleet sweep** (000001a17fd6b6f5): 9 more scripts converted;
    FLAG_COUNT_BASELINE seam (0ae5ff83). Retro closeout written THIS
    dispatch (docs/status/tasks/2026-10-30_05-10_task-000001a17fd6b6f5…);
    owns the SC1007 red (§d1).
12. **Report-body secret gate** (000001a180325195b, item 1): walker test
    scans every docs/status report through RedactSecrets (5344d9b9);
    zero live-shaped hits.
13. **pool-health DLQ backoff** (000001a180325195b item 2 + 000001a18096fad8
    + 000001a180e08acd3): `--dlq-backoff` fold (01e2442f), pool-config
    seeding (c34679fb), expiry boundary pin (094f87f0), summary count
    (ae5de6fd), `tq dlq` cross-link (46571509).
14. **netns diagnostics** (000001a18096fad8 items 1,3 + 000001a180325195b
    item 3): tri-state netns_is_empty with UNREADABLE distinct diagnostic
    (35cbfde1), gate widened to docs/problems+planning+README (e3d9cbdc),
    netns refusal self-test (a2a6d43e).
15. **dlqBackoffUntil webui panel** (000001a18120854c): shared fold +
    dashboard panel (05a0d5a3…7d4b7ff8 lineage, 7 same-subject heal
    commits); CHANGELOG + FEATURES verified current at HEAD. Retro
    closeout written THIS dispatch (…05-10_task-000001a18120854c…).

## b) PARTIALLY DONE

1. Canary sweep shipped the check-script-syntax SC1007 red — STILL RED
   at HEAD (re-ran the gate this dispatch: 113 ok / 1 failed). Rowed
   (1330), never cured.
2. `internal/dlqbackoff` module birth left cmd/tq's replace-free tidy
   red (unpublished module vs v0.3.4 tag) — row 1337.
3. Window follow-up tail (dead-sha cure, attempt-budget policy, verdict
   receipt, retryx verification) — all rowed by dispatches 17–23, all
   still open.

## c) NOT STARTED (window backlog, unchanged)

Dead-sha dangling-cite cure (now ~964 cites post-filter-branch, rows
1148/1350, owner-ruling-gated); postgres CLI store wiring; consumer ghost
package decision; SystemNix deploy cutover; dispatch redesign go/no-go —
all standing BLOCKED owner rows.

## d) TOTALLY FUCKED UP

1. **The re-dispatch storm itself is the damage**: this window has been
   re-derived by ~7 paid dispatches because verdicts failed to record
   (row 1335) and two workers died silently (23rd dispatch §d1). Every
   dispatch since the 17th has found the same landed tree.
2. **SC1007 red rode for 2+ days** — the canary sweep's own deliverable
   fails the fleet syntax gate; no closeout existed to own it until now.
3. **heal-daemon-sweep misattribution recurred across nearly every
   window session** (three folds of a foreign go.mod sweep in one
   session; the 04-40 closeout needed a manual mixed-reset rebuild) —
   span guard rowed (23rd dispatch appends), unbuilt.
4. **filter-branch history rewrite** (2026-10-28, two events) orphaned
   ~2,800 commits and detonated dead-SHA cites 8 → 964 — rows 1349/1350,
   owner ruling pending, guard row 726 still unbuilt.

## e) WHAT WE SHOULD IMPROVE

1. **Verdict-first exit** (row 1335's remedy): record `tq verdict`
   BEFORE the final commit — a committed report without a verdict is a
   half-closeout that costs a paid dispatch. This dispatch does exactly
   that ordering.
2. **Premise-gate-first dispatch protocol**: run the batch's own gate
   (or `git log --grep`) as command one; the dead-sha item 1 wasted a
   dispatch on a false premise (09-03 closeout §d1).
3. **Window-cursor advance**: a done-prompt whose report commits should
   never see the same window twice — the queue-side fix is row 1335;
   the agent-side fix is to cite prior dispatch reports and only
   delta-verify (done here: ~20 minutes instead of a full re-derivation).
4. Retro closeouts are cheap (`git show --stat` + one battery); minting
   them immediately beats paying a re-dispatch later (both this
   dispatch's closeouts were one-pass writes).

## f) NEXT THINGS (appended to TODO_LIST; deduped against 633 open rows)

Appended (see the twenty-fourth-dispatch section at TODO_LIST tail):
1. Resolve the stale repo-root `./tasks.db` scratch journal trap
   (untracked; bare `tq facts` in the repo reads 189 stale facts and
   answers journal queries WRONG — hit live this dispatch) — owner
   data-disposal call.
2. Clear the proxy-wait.sh:41 SC2120 shellcheck warning alongside the
   SC1007 re-green.
3. Consolidate inline canary greps onto `expect-branch.sh`
   (from the retro closeout §f1).
Plus 1 blocked question (tasks.db disposal ruling folded into item 1).

## g) QUESTIONS (owner-only)

1. Is the untracked repo-root `./tasks.db` (189 facts, last write
   2026-10-25) disposable scratch an agent may trash, or does it hold
   unarchived evidence? (folded into the appended row as its BLOCKED
   clause.)
2. Should the queue advance the done-prompt window cursor when a
   dispatch's window report COMMIT lands, treating verdict as a
   formality re-check — or is the current "verdict or re-claim" contract
   correct despite the ~7-dispatch storm it produced? (Kin of rows
   1335/1345 but a distinct, narrower queue-mechanics call.)
3. (None further — remaining open policy calls are already rowed:
   1334, 1345, 1350, 1353.)

## h) BAND DRIFT

`task.reprioritized` facts in the production journal
(`$TQ_DB=~/.local/state/tq/tasks.db`, window 2026-10-27T07:00 →
2026-10-28T09:00): **none recorded** (zero such facts exist journal-wide
as of this query). Note: the first query against the stale repo-root
`./tasks.db` returned zero facts from the WRONG database — see §f1 —
the re-pointed production query is the authoritative answer above.

— Footer commit carries `Task-Queue-ID: 000001a1815c3cc3048aaa3b632900000000`.
