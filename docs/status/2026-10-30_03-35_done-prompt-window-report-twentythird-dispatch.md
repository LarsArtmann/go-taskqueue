# Done-Prompt Window Report — Twenty-Third Dispatch — 2026-10-30 03-35 CET

**Task**: 000001a1815c3cc3048aaa3b632900000000 (done prompt) — SEVENTH
report-producing pass over the SAME 15-task window (2026-10-27 07:28 →
2026-10-28 08:03), NINTH claim in the journal. The seventeenth dispatch
(2026-10-28 09-05) holds the full derivation; the eighteenth through
twenty-second wrote re-verification deltas. Method this pass: all 13
window closeout reports re-read + the two report-less tasks
re-verified from their footer commits and journal facts; six window
test pins re-grepped at HEAD; the three known-red gates re-run for
current counts; journal re-censused. Nothing re-derived.

## The re-dispatch anatomy since the twenty-second (NEW this pass)

- The twenty-second committed its report (`105c1cb6`, 2026-10-29 07:35)
  and its text claims a same-shell verdict receipt — yet the task was
  RELEASED at 08:09:45 (fact 17145, `[effect=unknown]`, the ~40 min
  lease-expiry class) and re-claimed. The close did not take. Whether
  the verdict file was lost to the mid-run release or never accepted is
  unobservable from the repo — exactly the hole the verdict-receipt
  done-preflight leg (row 1335) exists to close, still unbuilt.
- TWO further claims then died SILENTLY: worker-53564 (claimed 08:09:45,
  released 08:56:01, fact 17169) and worker-21570 (claimed 08:56:01,
  released 2026-10-30 03:35:10, fact 17185) — zero commits, zero
  reports, zero facts beyond claim/release each. The worker-21570 hold
  locked exclusivity for ~18.6 h overnight (budget-gate requeues
  17011/17025/17029 had already parked the pool at midnight), so a
  full day passed with the window still unclosed. This pass (claim
  17186, worker-10961) is the ninth claim.
- In passing (delta since the twenty-second, `git log 105c1cb6..HEAD`):
  five footer-less daemon sweeps (2026-10-29 17:54–19:50: `17af902c`,
  `b8b40cda`, `b7c84026`, `023bf446`, `3b7de9b5`) carrying a concurrent
  session's Crush PreToolUse-hooks work — `scripts/hooks/tq-bash-guard.sh`,
  `scripts/hooks/tq-edit-advice.sh`, `.crushrc` wiring, the AGENTS.md
  hooks bullet, a CHANGELOG Internal row, `cmd/tq/facts_json_test.go` +
  `check-agents-size.sh` + `cmd/tq/go.work.sum`. Content judged sound
  and built-on (the AGENTS.md hooks convention is live in this
  session's own guard rails); attribution remains the daemon-sweep
  class (row 1319 family). Divergence 52 → 59 unpushed.

## a) FULLY DONE — the window landing re-verified at this HEAD

1. **Parked-surface provider batch (task 000001a17bfb2332, 3/3)**:
   AGENTS.md "Rate limits (429)" bullet records the provider riding the
   requeue evidence (409167ba); `tq tasks --parked --json` rows carry
   the additive `provider` field — pin `TestParkedJSONSurfacesProvider`
   re-grepped present (cmd/tq/parked_test.go); `tq doctor` names the
   exhausted provider. CHANGELOG Added rows present (provider JSON +
   doctor wording).
2. **LastFacts dedup (task 000001a17c354ee3, 3/3)**: one fetch feeding
   both the class join and the provider join in `cmdTasks` (416ff2a0);
   the postgres conform proof ran the full postgresv4 suite with the
   recipe embedded in the `TestStoreConformance` doc comment; FEATURES
   sweeper rows now MOVE-pointer to the Web UI outcome-card row.
3. **CI parity batch (task 000001a17c558d2b, 2/3 + 1 blocked-true)**:
   `scripts/check-guard-wiring.sh` wired into `.github/workflows/ci.yml`
   (verified at ci.yml:236) with the asymmetry comment trimmed to the
   one ci-local-only gate (dead-sha); the devmod shim gained the
   require-drift preflight (present in scripts/lib/cmd-tq-devmod.sh)
   which immediately caught 6 stale v0.3.3 pins → bumped to v0.3.4.
   Item 1 (check-dead-sha-refs into ci.yml) correctly stayed BLOCKED:
   the gate is red at HEAD (969 lines, §d3).
4. **Dirty-tree no-burn carve-out (task 000001a17ca0bd7c)**: landed
   (a0a343eb) — `executor.ErrDirtyTree` typed classification,
   `PreflightError.DirtyTree()`, parks on the escalating ladder with NO
   attempt burn; breaker stays armed for everything else; pin
   `TestDirtyTreePreflightNeverBurnsAttempt` re-grepped present
   (internal/worker/worker_test.go); CHANGELOG Added row with the
   explicit `(BEHAVIOR CHANGE …)` flag present.
5. **Retro + sweeps batch (task 000001a17ce9fa12)**: the missing
   closeout for 000001a141d63a4b written
   (docs/status/tasks/2026-10-27_11-41_…md); landed-but-unticked sweep
   ticked rows 842/989; the multi-pair fork-record exemption proved the
   premise false and shipped the honest doc+pin terminal state instead
   of a parser "fix".
6. **Tick-sweep + goldens + heal clause (task 000001a17d0e9e8e, 3/3)**:
   seven-channel tick-sweep ticked row 1218 (with the one-char cite-rot
   fix); `cmd/tq/stats_json_test.go` wire goldens landed (exact-JSON +
   bare-shape pins); the Known-Issues heal-refusal clause landed plus
   the compress-only AGENTS.md prune (restored ~2 k headroom at the
   time).
7. **§b-partials sweep + TODO self-dedup (task 000001a17d3c674b, 2/2)**:
   128-closeout corpus swept — two unrowed partials minted, everything
   else verified already rowed; the self-dedup consolidated SEVEN
   reworded pairs, collapsed the dead-SHA cure cluster 4 rows → 1, and
   ticked five landed rows with HEAD evidence (open rows 604 → 600 at
   commit time; 624 unchecked now, churn from five dispatch windows of
   appends).
8. **proxy-wait single-sourcing (task 000001a17d60fcfb, 2/2)**:
   `SMOKE_MODULE`/`SMOKE_VERSION` vars feed both the die_lag derivation
   and `drive()` (9d4c7525); the success line derives from the lib's
   `TQ_PROXY_SERVES_FMT` constant (90f74bbe). Smoke rc=0 at closeout;
   gates green this pass (§Receipts).
9. **Lease-expiry reclaim pin (task 000001a17d734f96d)**: root cause
   traced (claim-time DonePreflight footer signal completed a 3-item
   batch on ONE member's footer 84 ms after reclaim); mechanics landed
   batch-aware (475f1af2) — multi-member batches complete on the footer
   signal only when every member key is ticked-or-gone; pins
   `TestDonePreflightBatchFooterRequiresAllMembersLanded` +
   `TestDonePreflightAfterLeaseReclaimCompletesWithoutRerun` re-grepped
   present.
10. **Fixture attribution + canary layer (task 000001a17faa3cab)**:
    attribution gate widened to scripts/smoke (53/53 attributed),
    catching 4 real drifts en route; `scripts/lib/expect-branch.sh`
    canary helper landed with `--self-test`; release-gates smoke
    converted (12 ok / 0 fail) and the latent subshell fail-tally-loss
    bug fixed en route.
11. **Canary fleet sweep (task 000001a17fd6b6f5)**: landed (0ae5ff83,
    2026-10-28 02:41) — 8 check-* gates + smoke/archive-evidence
    converted to branch canaries; `check-flag-count` gained the
    `FLAG_COUNT_BASELINE` seam; row 1064 ticked by the seventeenth
    dispatch. NOT done by the task itself: the a)-g) report (§b1) and
    it shipped the SC1007 red (§d4).
12. **Report-body secret gate (task 000001a180325195b)**: walker gate
    over docs/status through the SAME `RedactSecrets` table
    (5344d9b9), widened to docs/problems + docs/planning + README.md
    (5acaa708, test renamed `TestReportBodiesCarryNoSecretShapedStrings`,
    re-grepped present); sweep clean at both HEADs.
13. **netns diagnostics (task 000001a18096fad8)**: tri-state
    `netns_is_empty` (0/1/2) with the UNREADABLE-vs-non-empty distinct
    diagnostic + `TQ_NETNS_DEV_FILE` fixture-driven self-test of the
    REAL detector (8b21505b); the netns refusal self-test legs landed
    the pass before (a2a6d43e).
14. **DLQ-backoff surface family (tasks 000001a180e08acd + 000001a18120854c)**:
    pool-health `--dlq-backoff` fold (01e2442f), config seeding
    `--config`/`$TQ_POOL_CONFIG` (0c76d861), boundary pin
    `TestPoolHealthDLQBackoffBoundaryInstant` (aa4e7ab4, re-grepped
    present), summary count `N repos, M in DLQ backoff` (1679c491),
    `tq dlq` row cross-links (fc4d3afb), and the webui dashboard panel
    (744baa9d — row 1077 ticked with receipts;
    `TestDLQBackoffPanelRenders` re-grepped present in
    internal/webui/backoff_test.go). CHANGELOG carries all of it with
    BEHAVIOR CHANGE flags (verified: [Unreleased] Added/Changed rows).

## b) PARTIALLY DONE

1. **Two window tasks shipped without their a)-g) closeout reports**:
   the canary fleet sweep (000001a17fd6b6f5, work complete + ticked by
   the 17th, but the report that owns its SC1007 red and seam design is
   unwritten — row 1331) and the dlqBackoffUntil webui batch
   (000001a18120854c, work complete + rows/CHANGELOG done — row 1332).
   Half-closeout policy ruling rowed BLOCKED (1334).
2. **The dead-cite heal has NOT started**: 969 DEAD-SHA lines at this
   HEAD (964 at the 22nd, +5 accrued) — the filter-branch explosion
   heal (row 1349) and its authorization (row 1350) both open, red now
   ~57 h.
3. **Signal-4 (closeout-report done signal) is still not batch-aware**
   and the batch-footer fall-through still has no observable record —
   the two follow-ups the 00-09 closeout minted remain open.
4. **Execution-level verification debt**: several window closeouts ran
   scoped batteries (no full ci-local) and the dep-sweep smoke has
   still never been EXECUTED (rows 296/987 family) — landed ≠ exercised
   on those lanes.
5. **Docs residue from the 06-59 batch**: main.go usage lines omit
   `--dlq-backoff`/`--config`, the DOMAIN_LANGUAGE rows (retry
   taxonomy, provider, batch landed-ness) are unwritten — README/
   FEATURES/AGENTS/CHANGELOG are otherwise current (verified §Docs
   pass below).

## c) NOT STARTED (standing set, unchanged and open at HEAD)

`scripts/check-erraudit.sh` landing + twin consolidation (rows
1081/1311/1333; check-doc-refs STILL red on the AGENTS.md cite); the
ruled `generic_return` erraudit migration (~197 sites / 24 modules);
postgres-conform smoke (1226); scripted unticked-sweep tool (1254
family); TODO-dedup detector (1300); heal-daemon-sweep span guard
(1319) + per-commit retitling (1289) + `--print-plan`; mechanical
history-rewrite guard (726); verdict-receipt done-preflight leg (1335)
+ the verdict-contract convention pin (1255); pool-health dirty-tree
lane split (1265 family); cmd/tq raw GOWORK=off tidy cure at the next
tag wave (965 family); divergence reconcile + go-health v0.8.1 deep
verification (NAF rows); the retro cure of rows 225/226
(000001a1382a1765); November archive sweep; windows vet battery +
E2E/cross-compile decouple + demoted-serve README paragraph
(eleventh-trio appends).

## d) TOTALLY FUCKED UP

1. **The verdict-exit hole has now consumed NINE claims and SEVEN
   report passes on one window.** The 22nd did everything the
   convention asks (commit + claimed same-shell verdict receipt) and
   still lost the close to a lease-expiry release; two further
   claimants produced zero recoverable artifacts. Seven-plus paid
   passes re-verified the same landing. The un-observability of a
   predecessor's verdict is the root cause; the leg that fixes it
   (1335) has been rowed since the eighteenth dispatch and remains
   unbuilt — the repo is paying for its own known cure seven times.
2. **The ~18.6 h abandoned lease**: worker-21570 held the task from
   2026-10-29 08:56 to 2026-10-30 03:35 with no observable progress;
   lease-expiry reclaim only fires when a competing claim arrives, and
   the budget-gated pool had none overnight. One stuck claimant can
   freeze a queue lane for a day.
3. **The filter-branch dead-cite explosion stands unhealed** (964 →
   969 lines, ~57 h red; rows 1349/1350). The post-rewrite
   "pre-attempt rev" environmental-gate verdict class it plausibly
   caused (facts 16970/17008) kept burning attempts on THIS task while
   the cure sat blocked.
4. **SC1007 has ridden SIX dispatch windows** (113 ok / 1 failed,
   check-flag-count.sh:107; SC2120 advisory proxy-wait.sh:41) since the
   canary sweep shipped it red (0ae5ff83). One-line fix, rowed (1330),
   re-counted by every pass including this one. Counting is not
   fixing.
5. **Half-closeout class ×2** (§b1): two paid turns landed verified
   work and met neither the report nor (for one) the tick obligation —
   the close contract's enforcement remains prose.
6. **Footer-less daemon sweeps carried another session's work again**
   (§The re-dispatch anatomy): five `chore:` commits, no footers, no
   per-commit subjects — the attribution debt row 1319 owns is
   accruing on every concurrent session.

## e) WHAT WE SHOULD IMPROVE

1. **Make the verdict receipt OBSERVABLE, not just claimed**: land row
   1335's done-preflight leg (a verdict fact / receipt file the next
   dispatch can read) and row 1255's convention pin. Seven passes of
   evidence say this is the highest-leverage code in the repo.
2. **Checkpoint-early protocol for long tasks**: both silent deaths
   wrote nothing in ~45 min; committing a report skeleton (and TODO
   appends) EARLY converts a released run into recoverable progress —
   the 21st's report survived only because the daemon swept it.
3. **Done-prompt briefs should name the canonical derivation** once a
   window has one (here: the 17th report) so paid re-dispatches verify
   forward instead of re-reading 15 closeouts from scratch (kin of the
   row-1273 economics ruling, owner-gated).
4. **Abandoned-claim liveness**: a claim with no heartbeat-visible
   progress for hours should be reclaimable without waiting for a
   competing claimant (pairs with the attempt-budget ruling, row 1345).
5. **Closeout self-flags repeat across the window**: edit→commit→battery
   inversion, heal-script misfires, receipt hygiene, premise-gate-first
   — every closeout confesses some; the mechanical cures (stage-only
   staging, heal span guard, receipt capture at gate time) are all
   rowed and none built. The process memory is written; the tooling is
   not.

## f) UP TO 50 NEXT THINGS (deduped — existing open rows cited, NOT re-minted; 2 new rows appended, see TODO_LIST)

1. Heal the filter-branch dead-cite explosion (row 1349; now 969 lines).
2. Owner ruling: filter-branch authorization + mass policy (row 1350, BLOCKED).
3. Re-green SC1007 (`FLAG_COUNT_SELF_TEST=''`, row 1330) — one line.
4. Land `scripts/check-erraudit.sh` + ci-local leg; consolidate the twins (rows 1081/1311/1333) — clears check-doc-refs.
5. Retro closeout: canary fleet sweep (row 1331).
6. Retro closeout: dlqBackoffUntil webui batch (row 1332).
7. Done-preflight verdict-receipt leg (row 1335).
8. Verdict-contract convention pin in docs/status/README.md + templates (row 1255).
9. Mechanical history-rewrite guard (row 726).
10. Re-dispatch closure economics ruling (row 1273, BLOCKED).
11. Attempt-budget policy for re-dispatch storms (row 1345, BLOCKED — now nine claims of evidence).
12. Half-closeout policy ruling (row 1334, BLOCKED).
13. heal-daemon-sweep span guard (row 1319) + per-commit retitling (1289) + `--print-plan` walk list.
14. NEW: diagnose the two silent dispatch deaths + abandoned-lease reclaim policy (appended this pass).
15. Scripted unticked-sweep tool with gate-state probes (row 1254 family).
16. TODO-dedup detector script (row 1300).
17. postgres-conform smoke script (row 1226).
18. cmd/tq raw GOWORK=off tidy cure at the next tag wave (row 965 family).
19. Divergence reconcile (59 unpushed) + go-health v0.8.1 deep verification.
20. DonePreflight signal-4 batch-awareness + ONE shared batchFullyLanded predicate (00-09 §b1/§f4; 973 family).
21. Observable record for batch-footer fall-throughs (00-09 §b2).
22. Retro cure of rows 225/226 (000001a1382a1765) — re-enqueue or evidence-tick.
23. Pool-health dirty-tree skip-streak lane split (1265 family).
24. generic_return erraudit migration batches (owner-ruled adopt, ~197 sites).
25. November archive sweep + digest row ritual (index bloat cadence rows).
26. DOMAIN_LANGUAGE rows: retry-taxonomy park/burn, provider, batch landed-ness (00-09 §f10, 12-03 §f, dirty-tree closeout §f4).
27. main.go usage-line consistency pass for `--dlq-backoff`/`--config` (06-59 §f5).
28. `tq dlq` trailing summary counts (06-59 §f12).
29. `tq pool-health --json` top-level `"backingOff": N` aggregate (06-59 §f2).
30. Shared "dead-letter a repo" test-seed helper (06-59 §f3).
31. Cross-surface prose pin: pool-health WARN ↔ `tq dlq` line share the advice tail (06-59 §f7).
32. Rename `applyPoolHealthConfigDefault` → `applyDLQBackoffConfigDefault` (06-59 §f8).
33. Same-millisecond dead-letter tiebreak pin (04-18 §f27).
34. Parked-class JSON: single-pass `latestRequeueEvidence` decider (07-28 §f24) + `--parked-class` JSON-path provider pin (07-28 §f5).
35. statsPayload population-path golden (12-44 §f1).
36. `commit-task.sh --stage-only` (12-44 §f2).
37. Subshell fail-tally audit across smokes/gates (00-52 §f2).
38. proxy-wait: single-source the proxy base URL + WARN-constant pins + lib-silence pin (13-51 appends).
39. AGENTS.md budget headroom re-tighten follow-through (the hook session already moved it to 23,900 — verify the twin stays in sync; done this pass: 23,799/23,900 green).
40. statusresume.go forensics + daemon zero-byte guard (12-03 mints).
41. Execute the dep-sweep smoke once (12-03 mint; row 296 family).
42. Re-anchor rows citing moved headermatrix paths (12-44 §f4).
43. Windows cross-compile battery for worker/executor + E2E decouple (eleventh-trio rows).
44. webui parked view provider tag + `tq top` parked chip (07-28 §f7/§f9).
45. Parked-horizon measurement for dirty-tree parks (10-35 §f5).
46. Park-then-clean cycle smoke on a scratch TQ_DB (10-35 §f13).
47. Canaries for the remaining bare-rc!=0 fixtures — verify the 0ae5ff83 sweep's claimed completeness with a fresh parallel pass (row 1064's residue note).
48. check-release-docs O14 heuristic for new user-visible output strings (07-28 §f15).
49. Fixture-attribution drift pre-commit pin (00-52 §e1).
50. Cure the dead-sha RED's fastest slice first: the 5 cites accrued SINCE the 22nd (docs written against reachable SHAs only until the mass heal lands).

## g) QUESTIONS FOR THE OWNER

1. **Attempt budget for re-dispatch storms** (row 1345, restated with
   nine claims of evidence): should status/done-prompt tasks get an
   elevated attempt floor or park-on-verify-gate-death, given the
   window was verified correct SIX passes ago and every pass since
   re-bought the same conclusion?
2. **Re-dispatch closure economics** (row 1273): this pass closed
   thin-delta against the seventeenth's canonical derivation (the
   twentieth established the precedent) — ratify or reject, and name
   the verdict-receipt surface a re-dispatch may read.
3. **NEW — abandoned-claim policy**: may a claim with no
   heartbeat-visible progress for >N hours be reclaimed WITHOUT a
   competing claimant (the 18.6 h worker-21570 hold), and should
   silent deaths (zero artifacts) count differently against attempt
   budget than artifact-producing deaths? (Minted as the BLOCKED half
   of this pass's new TODO row.)

## h) BAND DRIFT (ADR-0015 accountability)

**None recorded.** `tq facts -type task.reprioritized` over the
production journal (/mnt/pool/services/tq/tq.db) returned 0 facts —
the SEVENTH consecutive zero census. No priority moved via marker, AI,
unblock, or importance during the window or since; claim order moved
only via aging, new enqueues, and the budget-gate park overnight.

## Docs-health pass (this dispatch's own doc work)

- **Living docs verified current against the window** (no edits
  needed): README command table carries `tq dlq`/`tq pool-health`
  backoff rows (README.md:345/:349); FEATURES rows FULLY_FUNCTIONAL
  with test-pin citations (FEATURES.md:82/:107/:128); AGENTS.md carries
  the dirty-tree exemption bullet, the `internal/dlqbackoff` +
  `internal/retryx` architecture rows, the hooks convention, and the
  size guard green at 23,799/23,900 (re-run this pass); CHANGELOG
  [Unreleased] carries every operator-visible window change with
  BEHAVIOR CHANGE flags (dirty-tree carve-out, provider JSON, doctor
  naming, dlq-backoff family, demoted-serve); ROADMAP unchanged — the
  window minted no routed ideas beyond already-rowed TODO items.
- **No archives this pass**: the seventeenth/eighteenth/nineteenth
  dispatch reports still hold open forward items (SC1007, retro
  closeouts, erraudit gate, divergence) — not every-item-resolved, so
  they stay live per the archive-sweep conventions. The 2026-10-20
  chain was archived with the `digest 2026-10` row by the seventeenth.
- **Index row added** for this report (top chronological cluster).

## §Receipts (2026-10-30 03:3x–03:5x HEAD)

- Window test pins re-grepped present: `TestParkedJSONSurfacesProvider`
  (cmd/tq/parked_test.go), `TestDirtyTreePreflightNeverBurnsAttempt`
  (internal/worker/worker_test.go),
  `TestDonePreflightBatchFooterRequiresAllMembersLanded`
  (internal/harvest/donepreflight_test.go),
  `TestReportBodiesCarryNoSecretShapedStrings`
  (internal/executor/redact_reports_test.go),
  `TestDLQBackoffPanelRenders` (internal/webui/backoff_test.go),
  `TestPoolHealthDLQBackoffBoundaryInstant`
  (cmd/tq/poolhealth_test.go).
- `scripts/check-script-syntax.sh`: **113 ok / 1 failed** (SC1007,
  scripts/check-flag-count.sh:107; SC2120 advisory proxy-wait.sh:41).
- `scripts/check-dead-sha-refs.sh`: rc=1, **969 DEAD SHA lines** (964
  at the 22nd; +5 accrued since).
- `scripts/check-doc-refs.sh`: rc=1 (AGENTS.md cites
  `scripts/check-erraudit.sh`, exists in no ref — rows 1081/1311/1333).
- `scripts/check-agents-size.sh`: OK 23,799/23,900.
- `scripts/check-status-index.sh`: ok (2 pre-existing f26-class TRAILER
  WARNINGs).
- Journal: claims/releases for this task read from
  /mnt/pool/services/tq/tq.db facts 16929–17186 (release facts 17103,
  17125, 17145, 17169, 17185 all `[effect=unknown]`; budget-gate
  requeues 17011/17025/17029); `task.reprioritized` = 0 facts.
- All SHAs cited in this report were read from `git log` at this HEAD
  (reachable); no dead cites introduced.
