# Done-Prompt Window Report — Twentieth Dispatch (fourth paid pass) — 2026-10-28 20-57 CEST

**Task**: 000001a1815c3cc3048aaa3b632900000000 (done prompt) — the FOURTH paid
pass. The seventeenth dispatch (09-05) committed the full window derivation
(4a309b6f lineage); the eighteenth (11-02, f792fbc0) and nineteenth (17-50,
cf9c4cfa) wrote delta reports — ALL THREE exited without a recorded verdict,
so the queue re-claimed each time. This pass re-verifies the window at
20:5x HEAD, inventories the ~3 h delta since 17-50, and records the verdict
in the same shell as the commit (the interim discipline the 2026-10-20
14-48 index row has documented since October 20).

**Delta scope**: the seventeenth dispatch IS the full derivation of the
15-task window (2026-10-27 07:28 → 2026-10-28 08:03; all 13 closeout reports
read there and spot-verified again this pass; 2 tasks shipped with no
closeout — retro rows 1331/1332). This report re-verifies load-bearing
claims at HEAD and adds what only the delta can teach. Nothing re-derived.

## a) FULLY DONE — re-verified at 20:5x HEAD

1. **The window landing stands intact.** The parked-provider surface
   (AGENTS.md 429 bullet + `tq tasks --parked --json` provider field +
   doctor naming), the cmdTasks LastFacts dedup, check-guard-wiring in
   ci.yml + devmod require-drift preflight, dirty-tree park-instead-of-burn
   (`ErrDirtyTree` carve-out), retro closeout for the dep-sweep batch +
   landed-but-unticked sweep, tick-sweep + stats goldens + AGENTS prune,
   §b-partials sweep + TODO self-dedup, proxy-wait single-sourcing,
   lease-expiry batch-aware footer signal, fixture-attribution smoke
   extension + canary layer, canary FLEET sweep, report-body secret gate +
   widening, and the poisoned-repo DLQ backoff family (`internal/dlqbackoff`
   + pool-health + `tq dlq` + webui panel + netns tri-state). All five
   window test pins grepped present this pass:
   `TestParkedJSONSurfacesProvider` (cmd/tq/parked_test.go),
   `TestDirtyTreePreflightNeverBurnsAttempt` (internal/worker/worker_test.go),
   `TestDonePreflightBatchFooterRequiresAllMembersLanded`
   (internal/harvest/donepreflight_test.go),
   `TestReportBodiesCarryNoSecretShapedStrings`
   (internal/executor/redact_reports_test.go),
   `TestDLQBackoffPanelRenders` (internal/webui/backoff_test.go).
2. **The window's docs pass landed and holds**: README command-table rows
   for `tq dlq`/`tq pool-health` (4 refs at HEAD), FEATURES
   FULLY_FUNCTIONAL rows for pool-health `--dlq-backoff` + serve
   backoff surface with test-pin citations, AGENTS.md dirty-tree exemption
   + `internal/dlqbackoff` architecture row, CHANGELOG Added/Internal
   entries, index rows, and the 2026-10-20 done-prompt chain archived with
   the `digest 2026-10` row (14252455).
3. **Delta since 17-50 (concurrent agents, all verified this pass)**:
   - **`internal/retryx` graduated WITH attribution**: readmodel wiring +
     nil-Host pump demotion pin (34b703ba, b32214da), root module graph
     sync (d69b541c), executor depbump riding `retryx.RenameTransient`
     (3a92b1af) — the module is no longer the attribution-invisible
     daemon-swept orphan of the eighteenth §d3. Module gate green this
     pass (GOWORK=off build+vet+test rc=0).
   - **Fold-lock-loser staleness surface shipped** (78302c78/dc6c9a8c/
     d79fdb54) and its retro closeout landed (082add61: the eleventh-
     dispatch trio — CI confirmation, pump pin, follower contract —
     verified, ticked, CHANGELOGed).
   - **Report-body secret gate is GREEN at HEAD**: focused
     `TestReportBodiesCarryNoSecretShapedStrings` run this pass passed;
     the one trip (secret-shaped literals in the 05-23 report body) was
     elided by 74bc2a8d.
   - **cmd/tq windows cross-compile fix** (f6005692, unix-gated prompt
     footer test); the full cmd/tq devmod gate is green at HEAD
     (scripts/test-cmd-tq.sh rc=0, 107 s, this pass).
4. **Journal behavior (in passing, verified)**: this done-prompt task's own
   14-fact history rode the window's machinery as designed — the provider
   429 requeued to the 6 h cap with evidence (fact 16945), two dirty-tree
   preflight refusals parked without burn (16929: foreign
   readmodel/schema.go in flight; 16986: M TODO_LIST.md), verify-gate
   deaths requeued (16913, 16970). The queue preserved every pass's
   committed work across all re-dispatches. See §d2 for the cost side.

## b) PARTIALLY DONE

1. **SC1007 still red**: check-script-syntax = **111 ok / 1 failed** at
   HEAD (shellcheck SC1007, scripts/check-flag-count.sh:107, from the
   canary sweep 0ae5ff83). It has now ridden ~13 h across THREE
   dispatches, each docs-only and each deferring to row 1330. The SC2120
   advisory (proxy-wait.sh:41 netns_marker_gate unused args) also stands,
   report-only per the nineteenth's ruling.
2. **check-doc-refs is RED at HEAD**: AGENTS.md cites
   `scripts/check-erraudit.sh`, which exists in no ref (re-run this pass;
   rows 1081/1311, consolidation BLOCKED row 1333). A second standing red
   gate alongside SC1007.
3. **cmd/tq raw module path broken at HEAD, gate path green**: raw
   `GOWORK=off go test` in cmd/tq fails setup — internal/dlqbackoff is
   absent from the v0.3.4 tag the replace-free go.mod pins (reproduced
   this pass); the rowed cure awaits the next tag wave. The supported
   devmod gate passes (§a3).
4. **Dead-cite debt: residue, and still accreting**: 8 DEAD-SHA lines at
   HEAD (7 in the 06-59 closeout: f521cc94/aeb96824/a396d92b/d211095c; 1
   NEW — 18941a77, minted by the 19-38 eleventh-trio closeout on its
   landing day). The 1,072 → 8 drop is the 2026-10-20 chain ARCHIVE moving
   cites out of scan scope, not a cure; the consolidated cure row stays
   owner-blocked while the class proved it is live.
5. **Retro closeout rows 1331/1332 still open**: the canary fleet sweep
   (000001a17fd6b6f5) and the dlqBackoffUntil webui batch
   (000001a18120854c) still have no a)-g) reports.
6. **Divergence grew again**: local master is **47 commits ahead** of
   origin/master (33 at 17-50), and origin carries commits this branch
   lacks (the 19-38 closeout counted 2). Rows for reconcile + go-health
   deep verification accrue risk daily.
7. **AGENTS.md had no `internal/retryx` row despite adoption** — closed by
   THIS pass (architecture row + module-list entry; size guard green at
   23,339/23,600, twin test green via the devmod gate). The retryx
   verification row's remaining asks (check-go-mods, vendor-sync, flake
   vendorHash, graduation-intent confirmation) stay open — row NOT ticked.
8. **Index bloat**: 958 live rows vs the 400 warning threshold. October's
   monthly-digest ritual is satisfied (digest 2026-10 row), which the
   README convention accepts as the between-sweeps remedy; the November
   archive sweep is already owed.

## c) NOT STARTED (standing set the window skipped; all open at HEAD)

`scripts/check-erraudit.sh` landing (+ twin-consolidation BLOCKED row
1333), `generic_return` migration (~197 sites / 24 modules, smallest
first), postgres-conform smoke (1226), scripted unticked-sweep tool (1254
family), TODO-dedup detector (1300), heal-daemon-sweep span guard (1319) +
per-commit retitling (1289) + `--print-plan`, pool-health dirty-tree lane
split (1265) family, done-preflight signal-4 batch-awareness (973) +
report-EXISTS gate, windows vet battery + E2E/cross-compile decouple +
demoted-serve README (eleventh-trio appends). §f ideas minted across
closeouts remain report-only.

## d) TOTALLY FUCKED UP

1. **The verdict-exit hole recurred on its own remedy report — fourth paid
   pass.** The nineteenth dispatch NAMED the hole in §d1 and pointed at
   the rowed remedy, then exited without a verdict anyway. Four committed
   window reports, zero verdict receipts until this pass. The remedy is
   mechanical (the verdict-receipt done-preflight leg, rowed by the
   eighteenth); every re-dispatch since the first is a paid data point.
2. **One task, seven claims, five requeues, one environment failure**
   (journal facts 16889-16999, read this pass): two verify-gate deaths
   (16913, 16970), one provider-429 ride to the 6 h cap (16945), one
   environment-class failure (16936), two dirty-tree parks (16929, 16986).
   The no-burn classes held and evidence rode every requeue exactly as the
   window documented — but the open close-contract hole still converted
   the whole chain into paid round-trips, and the verify-gate deaths
   presumably burned attempts. Attempt-budget policy question minted (§g3).
3. **A red hard gate rode ~13 h while three paid passes re-documented it**
   (§b1): SC1007 shipped by the canary sweep, found by the seventeenth,
   re-found by the eighteenth, nineteenth, and now this pass — the fix
   belongs to row 1330's dispatch, but the fleet's "red at closeout must
   be attributed" rule (17th §e2) has no teeth yet (check-unticked.sh,
   row 1254 family).
4. **The dead-cite class minted a fresh instance on landing day** (§b4):
   the newest retro closeout cited a pre-heal SHA (18941a77) without a
   fork record, hours after two dispatches reported the debt as residual.
   The class is live while its cure is owner-blocked.
5. The seventeenth §d roster stands (heal-daemon-sweep misattributions ×3,
   edit→commit→battery inversion across 9 sessions, one-shot lies,
   standing reds) — not repeated item-by-item.

## e) WHAT WE SHOULD IMPROVE

1. **Land the verdict-receipt done-preflight leg before anything else** —
   four data points, all the same task. Until it lands, the only working
   discipline is same-shell commit+verdict with a `test -s` receipt: it is
   documented in this repo's own index (2026-10-20 14-48 row), was
   available to all three failed passes, and was still skipped — which is
   exactly why the fix must be mechanical, not behavioral.
2. **Delta-scope re-dispatches work**: the 17th's full derivation plus
   three delta passes kept every re-dispatch a fraction of first-pass
   cost while each still re-verified at its own HEAD. Keep the pattern.
3. **Graduate module → docs row in the same wave**: retryx shipped
   adopted into three modules with no AGENTS.md row until this pass; the
   verification row should own "record adoption" as a landing step, not
   an audit find.
4. Everything in the seventeenth §e stands (countable gates at closeout,
   heal fixes, commit-first tooling, CHANGELOG at landing time, receipt
   hygiene, dedup detector) — not repeated item-by-item.

## f) UP TO 50 NEXT THINGS — dedup-checked; ONE new row (the §g3 question)

Every actionable candidate this pass surfaced is already an open row; per
the dedup contract, nothing is re-minted. Top of the existing queue:

1. **Verdict-receipt done-preflight leg** — rowed (18th §f6); four
   instances of justification now.
2. **Re-green check-script-syntax** (SC1007) — row 1330.
3. **Retro closeouts** for the canary sweep + webui batch — rows 1331/1332.
4. **Land + wire scripts/check-erraudit.sh** — rows 1081/1311,
   consolidate per row 1333; also clears the check-doc-refs red (§b2).
5. **Cure the cmd/tq tidy red at the next tag wave** — rowed (vendor-sync
   section) alongside row 965's declared-tags hardening.
6. **Divergence reconcile + go-health deep verification** — NAF-session
   rows; 47 unpushed now, origin ahead too.
7. **Finish the retryx verification row** — module gate + adoption
   recording are now DONE (this pass, §a3/§b7); remaining:
   check-go-mods, vendor-sync, flake vendorHash, graduation-intent note.
8. **Dangling-cite cure** — consolidated row, owner-blocked; the fresh
   18941a77 instance rides it.
9. **DonePreflight signal 4 batch-aware + report-EXISTS gate** — row 973
   family (17th §e1).
10. **heal-daemon-sweep span guard + retitling + `--print-plan`** — rows
    1319/1289 + mint.
11. **check-unticked.sh gate-state probes** — row 1254 family; would have
    attributed SC1007 at closeout four dispatches ago.
12. **generic_return migration batches** — rowed, smallest modules first.

Report-only, not row-worthy: SC2120 netns_marker_gate advisory (fold into
the next proxy-wait dispatch, per the nineteenth's ruling); the November
archive sweep (digest ritual current); the done-prompt task's own requeue
anatomy (§d2 — evidence for the rowed leg + the new §g3 question, not a
work item); the seventeenth §f tail ideas (unchanged, none started).

## g) QUESTIONS FOR THE OWNER

1. **Verdict-exit enforcement shape** (eighteenth §g4, routed via the
   verdict-receipt leg row): may the executor withhold done-prompt
   completion until a verdict receipt exists? Four paid passes say the
   answer wants to be yes.
2. **Standing routed questions unchanged**: erraudit twin consolidation
   (BLOCKED row), half-closeout policy for landed-but-unreported tasks
   (BLOCKED row), battery-scope ruling (L827, standing since 2026-10-09).
3. **NEW — attempt-budget policy for re-dispatch storms**: this done-prompt
   consumed 7 claims / 5 requeues / 1 environment failure across 4 paid
   dispatches (2 verify-gate deaths, a provider-429 6 h ride, 2 dirty-tree
   parks) before one pass closed the contract. Should status-type tasks
   get an elevated attempt floor, should verify-gate deaths on them park
   instead of burn, or is the current budget correct because the no-burn
   classes held? Minted as a BLOCKED row this pass.

## h) BAND DRIFT (ADR-0015 accountability)

**None recorded.** `tq facts --type task.reprioritized` over the
production journal (/mnt/pool/services/tq/tq.db, 16,999 facts at read
time) returned 0 facts at 20:1x — the FOURTH consecutive verification
(seventeenth, eighteenth, nineteenth each verified the same). No priority
moved via marker, AI, unblock, or importance during the window or since;
claim-order changes came from aging + new enqueues only.

---

Verification receipts for THIS report's own claims (20:1x-20:5x HEAD):
check-script-syntax = 111 ok / 1 failed (SC1007 check-flag-count.sh:107;
SC2120 advisory proxy-wait.sh:41); check-doc-refs = MISSING `scripts/check-erraudit.sh` cite (rc=1 class, rows 1081/1311);
check-dead-sha-refs = 8 DEAD-SHA lines (06-59 closeout ×7: f521cc94,
aeb96824, a396d92b, d211095c; 19-38 closeout ×1: 18941a77);
check-agents-size = 23,339/23,600 after this pass's retryx row;
TestAgentsDocSizeGuard green via scripts/test-cmd-tq.sh (rc=0, 107 s);
TestReportBodiesCarryNoSecretShapedStrings green (focused run);
internal/retryx module gate green (GOWORK=off); five window test pins
grepped present (paths §a1); `tq facts --type task.reprioritized` = 0;
`git rev-list --count origin/master..HEAD` = 47; raw cmd/tq GOWORK=off
setup failure reproduced (internal/dlqbackoff absent from v0.3.4 tag,
rowed); done-prompt task facts read from the production journal (14 facts,
seqs 16889-16999).
