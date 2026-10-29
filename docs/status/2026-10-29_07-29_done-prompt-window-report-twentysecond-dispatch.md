# Done-Prompt Window Report — Twenty-Second Dispatch (sixth paid pass) — 2026-10-29 07-29 CET

**Task**: 000001a1815c3cc3048aaa3b632900000000 (done prompt) — the SIXTH paid
pass. The seventeenth dispatch (2026-10-28 09-05) holds the full derivation
of the 15-task window (2026-10-27 07:28 → 2026-10-28 08:03, 13 closeout
reports read + spot-verified); the eighteenth through twenty-first wrote
delta reports. The twenty-first run (06-13, worker-4571) wrote its full
report and even committed it — but via the DAEMON, footer-less (39207689,
06:37:58 + index row 33a95a92, 06:41:28), and the run itself was RELEASED
at 06:47:13 (journal fact 17103, `[effect=unknown]` — lease expiry class)
before its footer-commit and verdict. SIXTH re-dispatch. This pass
re-verifies at 07-2x HEAD, records the delta, appends the window's one
genuinely new incident, and writes the verdict in the SAME shell as the
commit with a `test -s` receipt before exit.

## a) FULLY DONE — re-verified at 2026-10-29 07-2x HEAD

1. **The window landing stands intact.** This pass re-grepped all five
   window test pins present:
   `TestParkedJSONSurfacesProvider` (cmd/tq/parked_test.go),
   `TestDirtyTreePreflightNeverBurnsAttempt` (internal/worker/worker_test.go),
   `TestDonePreflightBatchFooterRequiresAllMembersLanded`
   (internal/harvest/donepreflight_test.go),
   `TestReportBodiesCarryNoSecretShapedStrings`
   (internal/executor/redact_reports_test.go),
   `TestDLQBackoffPanelRenders` (internal/webui/backoff_test.go).
   The full surface (parked-provider evidence, cmdTasks LastFacts dedup,
   check-guard-wiring in ci.yml, dirty-tree no-burn carve-out, dep-sweep
   retro + tick-sweep + §b-partials sweep, proxy-wait single-sourcing +
   netns tri-state, lease-expiry batch-aware footer signal,
   fixture-attribution smoke + canary fleet sweep, report-body secret
   gate, poisoned-repo DLQ backoff family) carries the seventeenth's and
   twentieth's derivations — zero code delta since (§a3).
2. **Docs pass holds**: README `tq dlq`/`tq pool-health` rows, FEATURES
   FULLY_FUNCTIONAL rows with test-pin citations, AGENTS.md dirty-tree
   exemption + `internal/dlqbackoff` + `internal/retryx` rows, CHANGELOG
   entries, the archived 2026-10-20 done-prompt chain with
   `digest 2026-10`; index current through the twenty-first (README.md:130).
3. **Zero-code delta since the twentieth's final commit**: exactly two
   commits after 47c440b0 (39207689, 33a95a92), both daemon sweeps of the
   twenty-first report + its index row. `git log 47c440b0..HEAD` = 2, no
   Go/script source touched. §a–§c carry forward by re-verified receipts.

## b) PARTIALLY DONE

1. **NEW THIS PASS — the dead-cite debt EXPLODED 8 → 964 lines**
   (`check-dead-sha-refs.sh` re-run this pass, rc=1). Top offenders: the
   index docs/status/README.md (19+12+9 lines at :150/:176/:204), the
   13th/14th-dispatch reports, TODO_LIST.md:556+634-645, and the 06-59
   closeout + 19-38 closeout (the old 8). See §d1 for the cause.
2. **SC1007 still red**: check-script-syntax = **111 ok / 1 failed**
   (SC1007, scripts/check-flag-count.sh:107, re-run this pass) + the
   SC2120 proxy-wait.sh:41 advisory (report-only). Riding ~40 h across
   FIVE dispatches; row 1330.
3. **check-doc-refs still RED** (rc=1 this pass): AGENTS.md cites
   `scripts/check-erraudit.sh`, exists in no ref (rows 1081/1311,
   consolidation BLOCKED row 1333).
4. **Divergence grew 50 → 52 unpushed** (rev-list this pass) with the only
   local work being the twenty-first's daemon-committed report — origin
   ref movement + report commits; reconcile + go-health rows accrue risk.
5. **Retro closeouts for the canary sweep + dlqBackoffUntil webui batch
   still missing** (rows 1331/1332).
6. **cmd/tq raw GOWORK=off path broken** (internal/dlqbackoff absent from
   the v0.3.4 tag); devmod gate green — cure at the next tag wave.
7. **The twenty-first report is committed but footer-less** (daemon sweep
   39207689): its Task-Queue-ID exists only in its index row, not as a
   git trailer — the exact daemon-sweep attribution class
   `heal-daemon-sweep.sh` exists for (row 1319 family). Not healed this
   pass (the span is not exactly-mine; the script would refuse).

## c) NOT STARTED

Standing set unchanged and open at HEAD: `scripts/check-erraudit.sh`
landing + twin consolidation (row 1333), `generic_return` migration
(~197 sites / 24 modules), postgres-conform smoke (1226), scripted
unticked-sweep tool (1254 family), TODO-dedup detector (1300),
heal-daemon-sweep span guard (1319) + retitling (1289) + `--print-plan`,
pool-health dirty-tree lane split (1265 family), done-preflight
signal-4 batch-awareness + report-EXISTS gate (973 family), windows vet
battery + E2E/cross-compile decouple + demoted-serve README paragraph
(eleventh-trio appends), November archive sweep.

## d) TOTALLY FUCKED UP

1. **A DOUBLE git filter-branch rewrite on 2026-10-28 orphaned ~2,800
   commits and exploded the dead-cite debt 8 → 964.** Reflog evidence:
   `filter-branch: rewrite` at 20:02:13 (3a92b1af) and again at 21:07:10
   (47c440b0), sandwiching ordinary commits (d173763f, acbc9b5b,
   3aee33c4, 1bc86c88) that the rewrite then replaced. `git fsck
   --unreachable` counts ~2,800 orphaned commit objects. Every doc citing
   a pre-rewrite SHA went DEAD simultaneously — 964 cite-lines at HEAD —
   and NO heal followed: the hard red has ridden ~34 h while five done-
   prompt passes each reported the debt at its previous size (the 06-1x
   twenty-first still counted 8). This violates the repo's own history
   policy class (AGENTS.md: scripted edits via script files; row 277's
   lesson — every history-rewrite heal must re-run check-dead-sha-refs in
   its own battery — was not applied, again). The rewrite also plausibly
   explains the persistent "gate also fails at the pre-attempt rev"
   environmental verdicts on this task's verify gates (facts 16970/17008):
   pre-attempt revs ARE unreachable post-rewrite. New row appended (§f1);
   authorization question minted (§g2). Existing guard row: line 726
   (mechanical history-rewrite guard) — unbuilt while the class fired.
2. **The verdict-exit hole recurred a SIXTH time, in a NEW shape**: the
   twenty-first did everything right up to the commit — then was released
   mid-run (lease expiry, fact 17103) and its report reached git only via
   the footer-less daemon sweep. Six paid passes for one done-prompt; the
   verdict-receipt done-preflight leg (rowed 18th §f6) remains unbuilt.
3. **SC1007 + check-doc-refs have ridden five dispatches** where each pass
   only re-counted them (§b2/§b3). The fix is a one-liner (row 1330);
   documentation is not a cure.
4. The seventeenth/twentieth §d rosters (heal misattributions,
   edit→commit→battery inversion, landing-day dead cite 18941a77) stand —
   now subsumed by §d1's blast radius.

## e) WHAT WE SHOULD IMPROVE

1. **Land the verdict-receipt done-preflight leg** — six data points, one
   task, two distinct failure shapes (missing verdict; mid-run release).
   Mechanical, rowed, still unbuilt.
2. **History-rewrite guard + heal battery** (rows 726/277): §d1 is the
   third incident of this class; the guard refuses rebase/filter-branch
   touching origin-reachable commits with a printed escape. Cheapest
   high-value code in the repo right now.
3. **Gate-state attribution at closeout** (check-unticked.sh probes, row
   1254 family): five passes re-counted SC1007 and none attributed it in
   the fleet gate summary; §d1's explosion sat un-attributed for 34 h.
4. The twentieth's §e stands (delta-scope re-dispatches work — this pass
   is again a fraction of first-pass cost; module → docs row same wave;
   receipt hygiene).

## f) UP TO 50 NEXT THINGS — dedup-checked; TWO new rows appended

Every prior candidate is already rowed (dedup pre-check run against the
unchecked set); the only non-duplicates are §d1's incident and its
authorization question. Impact order:

1. **NEW (rowed this pass): heal the 2026-10-28 filter-branch
   dead-cite explosion** — re-resolve/baseline the 964 DEAD-SHA lines per
   the consolidated cure's procedure, as ONE incident heal with its own
   battery (row 277's rule).
2. **NEW (rowed this pass, BLOCKED): who ran the 2026-10-28 double
   filter-branch and was it authorized** — owner ruling; also decides
   re-resolve-vs-baseline mass policy for the 2,800 orphans.
3. Verdict-receipt done-preflight leg (18th §f6 row) — six instances.
4. SC1007 one-liner re-green — row 1330.
5. Land `scripts/check-erraudit.sh` + wire (clears check-doc-refs) —
   rows 1081/1311, consolidate per 1333.
6. Retro closeouts for the canary sweep + webui batch — rows 1331/1332.
7. History-rewrite guard — row 726.
8. Divergence reconcile + go-health deep verification — NAF rows; 52
   unpushed now.
9. Cure cmd/tq raw-path tidy red at the next tag wave (row 965 family).
10. Finish the retryx verification row; heal-daemon-sweep span guard +
    retitling (1319/1289); check-unticked.sh probes (1254 family);
    generic_return batches; postgres-conform smoke (1226); TODO-dedup
    detector (1300); signal-4 batch-awareness (973 family); windows vet
    battery + E2E decouple + demoted-serve README (eleventh-trio rows).

## g) QUESTIONS FOR THE OWNER

1. **Authorization of the 2026-10-28 double filter-branch**: the reflog
   shows two `filter-branch: rewrite` events (20:02:13, 21:07:10)
   orphaning ~2,800 commits and detonating 964 dead cites. Which session
   ran it, against which policy escape, and should the rewritten lineage
   stand (re-resolve cites) or be abandoned for the pre-rewrite lineage
   where reachable? (Minted as the §f2 BLOCKED row.)
2. **Standing routed questions unchanged**: verdict-exit enforcement shape
   (routed via the verdict-receipt leg row), attempt-budget policy for
   re-dispatch storms (BLOCKED row, now six dispatches of evidence), and
   the standing erraudit-twin / half-closeout / battery-scope BLOCKED rows.

## h) BAND DRIFT (ADR-0015 accountability)

**None recorded.** `tq facts --type task.reprioritized` over the
production journal (/mnt/pool/services/tq/tq.db) returned 0 facts at
07-0x — the SIXTH consecutive verification. No priority moved via marker,
AI, unblock, or importance during the window or since; claim order moved
only via aging + new enqueues.

---

Verification receipts for THIS report's own claims (2026-10-29 07-2x):
check-script-syntax 111 ok / 1 failed (SC1007 check-flag-count.sh:107;
SC2120 advisory proxy-wait.sh:41); check-doc-refs rc=1 (MISSING
`scripts/check-erraudit.sh` cite); check-dead-sha-refs rc=1 with **964**
DEAD SHA lines (top: docs/status/README.md:150 ×19, :176 ×12, :204 ×9;
13th/14th-dispatch reports; TODO_LIST.md:556/634-645; the prior 8 in the
06-59 + 19-38 closeouts); reflog `filter-branch: rewrite` ×2 (2026-10-28
20:02:13 → 3a92b1af, 21:07:10 → 47c440b0); `git fsck --unreachable` ≈
2,800 commits; `git log 47c440b0..HEAD` = 2 daemon commits (39207689,
33a95a92); `git rev-list --count origin/master..HEAD` = 52;
`tq facts --type task.reprioritized` = 0; five window test pins grepped
present; twenty-first release fact 17103 (06:47:13, `[effect=unknown]`)
read from the production journal; TODO_LIST dedup pre-check run before
appending (two non-duplicate rows).
