# Done-Prompt Window Report — Twenty-First Dispatch (fifth paid pass) — 2026-10-29 06-13 CET

**Task**: 000001a1815c3cc3048aaa3b632900000000 (done prompt) — the FIFTH paid
pass. The seventeenth dispatch (2026-10-28 09-05) holds the full derivation
of the 15-task window (2026-10-27 07:28 → 2026-10-28 08:03, 13 closeout
reports read + spot-verified); the eighteenth, nineteenth, and twentieth
(20-57) wrote delta reports. ALL FOUR exited without a recorded verdict,
so the queue re-claimed each time — the twentieth's own §d1 diagnosed the
hole and then repeated it. This pass re-verifies at 06-1x HEAD, records
the zero-code delta since the twentieth, and writes the verdict in the
SAME shell as the commit (test -s receipt before exit).

**Delta scope**: the seventeenth IS the full window derivation; the
twentieth (823e69e3/47c440b0, committed 2026-10-28 21:0x) is the latest
full re-verification. Nothing re-derived. This report is a thin delta:
HEAD is BYTE-IDENTICAL to the twentieth's final commit (47c440b0 — `git
log 47c440b0..HEAD` is empty), so §a/§b/§c carry forward by re-verified
receipts, and the only new material is the fifth-pass evidence itself.

## a) FULLY DONE — re-verified at 2026-10-29 06-1x HEAD

The window landing stands intact; this pass re-confirmed the load-bearing
receipts directly:

1. **All five window test pins present at HEAD** (grep -l pass):
   `TestParkedJSONSurfacesProvider` (cmd/tq/parked_test.go),
   `TestDirtyTreePreflightNeverBurnsAttempt` (internal/worker/worker_test.go),
   `TestDonePreflightBatchFooterRequiresAllMembersLanded`
   (internal/harvest/donepreflight_test.go),
   `TestReportBodiesCarryNoSecretShapedStrings`
   (internal/executor/redact_reports_test.go),
   `TestDLQBackoffPanelRenders` (internal/webui/backoff_test.go).
2. **Window surface verified via the twentieth's receipts** (unchanged
   HEAD): parked-provider evidence surface, cmdTasks LastFacts dedup,
   check-guard-wiring in ci.yml, dirty-tree no-burn carve-out, dep-sweep
   retro closeout + landed-but-unticked sweep, tick-sweep, §b-partials
   sweep, proxy-wait single-sourcing + netns tri-state, lease-expiry
   batch-aware footer signal, fixture-attribution smoke extension +
   canary fleet sweep, report-body secret gate, poisoned-repo DLQ
   backoff family (`internal/dlqbackoff` + pool-health + `tq dlq` +
   webui panel).
3. **Docs pass holds**: README `tq dlq`/`tq pool-health` rows, FEATURES
   FULLY_FUNCTIONAL rows with test-pin citations, AGENTS.md dirty-tree
   exemption + `internal/dlqbackoff` + `internal/retryx` rows (size
   23,339/23,600, green at the twentieth), CHANGELOG entries, the
   2026-10-20 done-prompt chain archived with `digest 2026-10` row
   (14252455), index rows current through the twentieth.

## b) PARTIALLY DONE (all re-confirmed live at 06-1x HEAD)

1. **SC1007 still red**: check-script-syntax = **111 ok / 1 failed**
   (shellcheck SC1007, scripts/check-flag-count.sh:107, re-run this
   pass) + the SC2120 proxy-wait.sh:41 advisory (report-only, per the
   nineteenth's ruling). Riding ~33 h across FOUR dispatches now.
2. **check-doc-refs still RED**: AGENTS.md cites
   `scripts/check-erraudit.sh` which exists in no ref (re-run this
   pass; rows 1081/1311, consolidation BLOCKED row 1333).
3. **Dead-cite residue 8 lines** (re-run this pass): 7 in the 06-59
   closeout (f521cc94/aeb96824/a396d92b/d211095c) + 1 minted on landing
   day (18941a77, 19-38 closeout). Consolidated cure stays owner-blocked.
4. **Divergence grew 47 → 50 unpushed** (rev-list this pass) with zero
   new local commits since 47c440b0 — the origin/master ref moved under
   the branch; reconcile + go-health rows accrue risk.
5. **Retro closeouts for the canary sweep + dlqBackoffUntil webui batch
   still missing** (rows 1331/1332).
6. **cmd/tq raw GOWORK=off path broken** (internal/dlqbackoff absent
   from the v0.3.4 tag); devmod gate green — cure at the next tag wave.

## c) NOT STARTED

Standing set unchanged and all open at HEAD: `scripts/check-erraudit.sh`
landing + twin consolidation (row 1333), `generic_return` migration
(~197 sites / 24 modules), postgres-conform smoke (1226), scripted
unticked-sweep tool (1254 family), TODO-dedup detector (1300),
heal-daemon-sweep span guard (1319) + retitling (1289) + `--print-plan`,
pool-health dirty-tree lane split (1265 family), done-preflight signal-4
batch-awareness + report-EXISTS gate (973 family), windows vet battery +
E2E/cross-compile decouple + demoted-serve README paragraph (eleventh-trio
appends), November archive sweep (digest ritual current until then).

## d) TOTALLY FUCKED UP

1. **The verdict-exit hole recurred a FIFTH time.** Four committed window
   reports (17th → 20th), zero verdict receipts between them; the
   twentieth diagnosed the hole in its own §d1 and then exited through
   it anyway. Five paid passes for one done-prompt is now the repo's
   single most expensive standing process bug, and the cure (verdict-
   receipt done-preflight leg, rowed 18th §f6) remains unbuilt. THIS
   pass closes the contract same-shell — but the fix must be mechanical.
2. **The twentieth's §d roster stands unchanged** (no code delta since):
   SC1007 riding across paid passes while re-documented, the fresh
   landing-day dead cite, the heal/attribution debt of the seventeenth.
   Not repeated item-by-item.
3. **Divergence widened pass-over-pass** (47 → 50 with no local work):
   even a zero-delta re-dispatch watched repo risk grow.

## e) WHAT WE SHOULD IMPROVE

1. **Land the verdict-receipt done-preflight leg** — five data points,
   all this task, all the same exit path. Until the queue itself
   withholds completion, every docs-only done-prompt is one distraction
   away from a sixth paid round-trip.
2. **Red-gate triage SLA**: a hard gate (SC1007, check-doc-refs) has now
   ridden 4+ dispatches where each pass only re-counted it. The rowed
   check-unticked.sh gate-state probes (1254 family) would attribute the
   red at closeout; the SC1007 one-line fix itself (row 1330) is cheaper
   than any further documentation of it.
3. The twentieth's §e stands (delta-scope re-dispatches, module → docs
   row same wave, receipt hygiene, dedup detector) — not repeated.

## f) UP TO 50 NEXT THINGS — dedup-checked; ZERO new rows this pass

Every actionable candidate is already an open row (verified against the
unchecked set before writing); per the dedup contract nothing is
re-minted. Top of the existing queue, in impact order:

1. Verdict-receipt done-preflight leg (18th §f6 row) — five instances.
2. Re-green check-script-syntax, SC1007 one-liner — row 1330.
3. Land `scripts/check-erraudit.sh` + wire (clears check-doc-refs red) —
   rows 1081/1311, consolidate per row 1333.
4. Retro closeouts for the canary sweep + webui batch — rows 1331/1332.
5. Divergence reconcile + go-health deep verification — NAF rows; 50
   unpushed now, origin ahead too.
6. Cure cmd/tq raw-path tidy red at the next tag wave (row 965 family).
7. Finish the retryx verification row (check-go-mods, vendor-sync, flake
   vendorHash, graduation-intent note; module gate + adoption DONE).
8. Dangling-cite cure (consolidated row, owner-blocked; 8 live lines).
9. heal-daemon-sweep span guard + retitling + `--print-plan` (1319/1289).
10. check-unticked.sh gate-state probes (1254 family); generic_return
    migration batches; postgres-conform smoke (1226); TODO-dedup
    detector (1300); signal-4 batch-awareness (973 family); windows vet
    battery + E2E decouple + demoted-serve README (eleventh-trio rows).

## g) QUESTIONS FOR THE OWNER

None new this pass. The three standing routed questions are unchanged:
verdict-exit enforcement shape (routed via the verdict-receipt leg row),
attempt-budget policy for re-dispatch storms (BLOCKED row minted by the
twentieth), and the standing erraudit-twin / half-closeout /
battery-scope BLOCKED rows.

## h) BAND DRIFT (ADR-0015 accountability)

**None recorded.** `tq facts --type task.reprioritized` over the
production journal (/mnt/pool/services/tq/tq.db) returned 0 facts at
06-0x — the FIFTH consecutive verification. No priority moved via
marker, AI, unblock, or importance during the window or since; claim
order moved only via aging + new enqueues.

---

Verification receipts for THIS report's own claims (2026-10-29 06-1x):
`git log 47c440b0..HEAD` empty (zero-code delta); check-script-syntax
111 ok / 1 failed (SC1007 check-flag-count.sh:107; SC2120 advisory
proxy-wait.sh:41); check-doc-refs MISSING `scripts/check-erraudit.sh`
cite; check-dead-sha-refs 8 DEAD-SHA lines (06-59 closeout ×7, 19-38
closeout ×1: 18941a77); `git rev-list --count origin/master..HEAD` = 50;
`tq facts --type task.reprioritized` = 0; five window test pins grepped
present (paths §a1); TODO_LIST dedup pre-check run before §f (no new
rows, no ticks — nothing landed since 47c440b0).
