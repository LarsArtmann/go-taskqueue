# Done-Prompt Window Report — Twenty-Third Dispatch — 2026-10-30 03-35 CET

**Task**: 000001a1815c3cc3048aaa3b632900000000 (done prompt) — SEVENTH paid
pass, and the NINTH claim on the production journal since the window
closed. The seventeenth dispatch (docs/status/2026-10-28_09-05) holds the
canonical full derivation of the 15-task window (2026-10-27 07:28 →
2026-10-28 08:03); the eighteenth through twenty-second are re-verifying
deltas. THIS pass re-verifies at 03-3x HEAD, extends the re-dispatch
anatomy with two NEW incidents the journal records since the twenty-second
(two silent dispatch deaths + an ~18.6 h abandoned lease), and closes the
verdict contract in the same shell as the commit.

**Claims ledger for this task** (journal facts 16929–17186, read this
pass): fifteen claims total across workers 31494 (×2), 76767, 90066 (×2),
19964, 45578 (×3), 4571, 40764, 24668, 53564, 21570, 10961 (this pass).
Report-producing dispatches: seventeenth through twenty-second (six) plus
this one. Since the twenty-second's release (fact 17145, 2026-10-29
08:09:45, `[effect=unknown]` — 34 min after its report commit), TWO further
claims died leaving ZERO commits, ZERO reports, ZERO facts beyond
claim/release: worker-53564 (claimed 08:09:45, released 08:56:01) and
worker-21570 (claimed 08:56:01, released only when THIS claim's reclaim
fired at 2026-10-30 03:35:10 — an ~18.6 h exclusivity hold). Overnight
re-dispatch was additionally budget-gated (facts 17011/17025/17029: daily
budget 30/30 exhausted).

## a) FULLY DONE — the window landing, re-verified at 2026-10-30 03-4x HEAD

The landing is INTACT. Every load-bearing pin grepped present at HEAD;
gates re-run; docs verified current. Seventh consecutive confirmation of
the same 15/15 landing state — zero regressions found.

1. **Parked-surface provider batch (task 000001a17bfb2332, 3/3)**:
   AGENTS.md "Rate limits (429)" bullet records the provider riding
   `RequeueEvidence.provider` (409167ba); `tq tasks --parked --json` rows
   carry the additive `provider` field, pinned by
   `TestParkedJSONSurfacesProvider` (cmd/tq/parked_test.go, pin re-grepped
   present); `tq doctor`'s parked check names the exhausted provider.
   CHANGELOG Added rows present (parked JSON + doctor naming).
2. **cmdTasks LastFacts dedup (task 000001a17c354ee3, row 954)**: one
   `LastFacts` fetch hoisted above the class + provider joins (416ff2a0);
   postgres conform proof for `TestRequeueFactCarriesProvider` recorded as
   a reproducible recipe in the `TestStoreConformance` doc comment
   (internal/queue/postgresv4/store_test.go, fdb258b9); FEATURES.md
   sweeper rows converted to MOVE pointers (e0a6c3db).
3. **CI parity + devmod preflight (task 000001a17c558d2b)**:
   `check-guard-wiring` wired into .github/workflows/ci.yml (re-verified
   at line 236 this pass); the asymmetry comment trimmed to the one
   remaining ci-local-only gate (dead-sha); devmod shim gained the
   require-drift preflight (335613ae, `scripts/lib/cmd-tq-devmod.sh`
   re-grepped present) which caught 6 real stale v0.3.3 pins → bumped to
   v0.3.4. Item 1 (dead-sha into ci.yml) correctly stayed BLOCKED: the
   gate is RED at HEAD (see §d3) and its cure ruling is rowed.
4. **Dirty-tree preflight carve-out (task 000001a17ca0bd7c, a0a343eb)**:
   typed `executor.ErrDirtyTree` / `PreflightError.DirtyTree()`; dirty-tree
   preflight refusals park on the escalating ladder with NO attempt burn;
   pinned by `TestDirtyTreePreflightNeverBurnsAttempt`
   (internal/worker/worker_test.go, pin re-grepped present). CHANGELOG
   Added row with the explicit `(BEHAVIOR CHANGE …)` flag verified present.
5. **Dep-sweep retro closeout + unticked sweep + multi-pair exemption
   (task 000001a17ce9fa12)**: retro report
   docs/status/tasks/2026-10-27_11-41_task-000001a141d63a4bb487c2dcdf4d00000000.md
   written and indexed; landed-but-unticked sweep ticked rows 842/989 with
   HEAD evidence; the multi-pair fork-record exemption landed as
   header doc + self-test pin (f941edb5/d0b796fa) after live-verifying the
   premise was already true.
6. **Tick-sweep + stats goldens + AGENTS.md heal clause (task
   000001a17d0e9e8e)**: row 1218 ticked (print-floor self-test battery +
   ci-local floor-parse dedup); `cmd/tq/stats_json_test.go` wire goldens
   landed (`TestStatsPayloadGoldenJSON` + bare shape); the heal-refusal
   span-terminator clause landed in AGENTS.md Known Issues (present in the
   file this pass) with the compress-only prune that restored ~2 k
   headroom (further budget mechanics since re-tightened by the hook
   session — see §a14).
7. **§b-partials sweep + TODO_LIST self-dedup (task 000001a17d3c674b)**:
   128-closeout corpus swept, 2 unrowed partials minted, seven reworded
   duplicate pairs consolidated, the dead-SHA cure cluster collapsed 4 rows
   → 1 survivor, five landed rows ticked with HEAD evidence (510cf105,
   f4b5ca06).
8. **proxy-wait single-sourcing (task 000001a17d60fcfb)**:
   `SMOKE_MODULE`/`SMOKE_VERSION` vars + `TQ_PROXY_SERVES_FMT` constant
   (9d4c7525, 90f74bbe); smoke rc=0 with captured receipts; die/success
   prose has one source each.
9. **Lease-expiry reclaim pin (task 000001a17d734f96, 475f1af2)**: the
   done-preflight footer signal is batch-aware for multi-member batches
   (completes only when every member key is ticked-or-gone);
   `TestDonePreflightBatchFooterRequiresAllMembersLanded` +
   `TestDonePreflightAfterLeaseReclaimCompletesWithoutRerun` both
   re-grepped present.
10. **Fixture attribution widened + canary accuracy (task
    000001a17faa3cab)**: smoke heredocs attributed 53/53 (f8e3a10a);
    `scripts/lib/expect-branch.sh` canary helper with `--self-test`
    (c2b3d439); release-gates smoke converted + the latent subshell
    fail-tally bug fixed.
11. **Canary fleet sweep (task 000001a17fd6b6f5, 0ae5ff83 — NO closeout
    report, row 1064 ticked by the seventeenth)**: 8 check-* gates +
    smoke/archive-evidence.sh converted to branch-canary assertions;
    `check-flag-count` gained the `FLAG_COUNT_BASELINE` seam. RESIDUE: the
    sweep shipped `check-script-syntax` RED (SC1007) and no a)-g) report —
    rows 1330/1331 own both; see §d4.
12. **Report-body secret gate (task 000001a180325195, 5344d9b9 + widened
    5acaa708)**: `TestReportBodiesCarryNoSecretShapedStrings` (pin
    re-grepped present) walks docs/status + docs/problems + docs/planning
    + README through the SAME `RedactSecrets` table as output tails;
    sweep clean at HEAD.
13. **Poisoned-repo DLQ backoff family (tasks 000001a180e08acd,
    000001a18120854c)**: `tq pool-health --dlq-backoff` + `--config`
    seeding (01e2442f, 0c76d861); boundary pin
    `TestPoolHealthDLQBackoffBoundaryInstant` (aa4e7ab4, pin re-grepped
    present); summary count (1679c491); `tq dlq` cross-link (fc4d3afb);
    webui panel (744baa9d, row 1077 ticked with DONE note,
    `TestDLQBackoffPanelRenders` re-grepped present). NO a)-g) report for
    the webui batch — row 1332 owns the retro closeout.
14. **netns diagnostics (tasks 000001a18096fad8, 000001a180e08acd)**:
    tri-state `netns_is_empty` with the UNREADABLE-vs-non-loopback
    distinction (8b21505b) + the oracle-independent refusal self-test
    (a2a6d43e, sabotage-proven).
15. **Docs pass holds at HEAD** (landed by the seventeenth, re-verified
    this pass): README command rows for `tq dlq`/`tq pool-health` backoff
    surfaces (README.md:345/349); FEATURES FULLY_FUNCTIONAL rows with test
    cites (FEATURES.md:82/107/128); AGENTS.md dirty-tree exemption +
    `internal/dlqbackoff` architecture row + hooks bullet; CHANGELOG
    `[Unreleased]` carries every operator-visible window change with
    BEHAVIOR CHANGE flags where applicable; `agents-size` green
    (23,799/23,900 B — the budget constant was re-tightened 23,600 →
    23,900 by the concurrent hook session's 023bf446 with a paired
    compress).

## b) PARTIALLY DONE

1. **Two window tasks shipped without a)-g) closeout reports** (canary
   sweep 0ae5ff83; webui backoff panel 744baa9d..ca63b282): work verified
   landed and rows ticked, but the close contract's report leg is unmet.
   Retro-closeout rows 1331/1332 exist; the policy question is row 1334
   (BLOCKED owner). The canary sweep also shipped the SC1007 red (§d4).
2. **The dead-cite heal has NOT started**: 969 DEAD-SHA lines at this
   pass's re-run (964 at the twenty-second, +5 accrued — the accrual
   slowed to a trickle but continues). Rows 1349 (heal) and 1350
   (authorization) both open; ~57 h of continuous red.
3. **Signal 4 of DonePreflight (closeout-report signal) is still not
   batch-aware** (2026-10-28 00-09 closeout §b1) and the fall-through
   observability row is unbuilt — the partially-landed-batch completion
   hole this window's incident exposed remains open on the report signal.
4. **Several window batteries were scoped, not full ci-local** (disclosed
   in each closeout): the 07-28, 09-03, 00-52, 04-18, 05-23, 06-59
   sessions ran targeted module/surface gates. No window regression
   surfaced from this — seven full re-verifications found the landing
   green — but the full pre-push battery has never run on this stack.
5. **Existence-level verification residue**: the dep-sweep smoke,
   ci-local wiring, and `TQ_DEPSWEEP_LIVE_CALIB` calibration test are
   verified present-but-never-executed (rows 296/987/989 family; the
   12-03 closeout minted the execution row).
6. **Divergence grew 52 → 59 unpushed commits** (rev-list re-count this
   pass) with the only local work being five footer-less daemon sweeps of
   a concurrent session's Crush-hooks work (§d6) plus report commits —
   origin reconciliation (rowed) accrues merge risk daily.

## c) NOT STARTED (standing set, unchanged at HEAD)

- `scripts/check-erraudit.sh` landing + ci-local leg (check-doc-refs STILL
  rc=1 this pass — AGENTS.md cites the script, it exists in no ref; rows
  1081/1311, consolidation BLOCKED row 1333).
- `generic_return` erraudit migration (~197 sites / 24 modules, owner-ruled
  adopt, dedicated project).
- Postgres-conform smoke script (row 1226); scripted landed-but-unticked
  sweep tool (row 1254 family); TODO-dedup detector script (row 1300).
- heal-daemon-sweep span guard (1319), per-commit retitling (1289),
  `--print-plan`; mechanical history-rewrite guard (row 726, third
  incident of the class now evidenced).
- Verdict-receipt done-preflight leg (row 1335) + the verdict-receipt
  contract pin in the dispatch docs (row 1255).
- Pool-health dirty-tree skip-streak lane split; cmd/tq raw `GOWORK=off`
  build cure at the next tag wave (row 965 family); November archive
  sweep; divergence reconcile + go-health v0.8.1 deep verification (NAF
  rows); DonePreflight shared batch-landed predicate + owner MUST-policy
  ruling (row 973 family).

## d) TOTALLY FUCKED UP

1. **The verdict-exit hole has now consumed NINE claims and SEVEN
   report-producing dispatches on one window.** The twenty-second did
   everything right on paper — committed its report (105c1cb6, 07:35:10)
   and claims a same-shell verdict with a `test -s` receipt — yet the task
   was RELEASED at 08:09:45 (fact 17145, `[effect=unknown]`, ~40 min lease
   expiry) and re-claimed anyway. Whether its verdict file was lost to the
   mid-run release or never reached the queue is UNOBSERVABLE — that
   observability gap is exactly what row 1335's verdict-receipt leg exists
   to close, and it is still unbuilt, seven passes later. The queue design
   lets a done-prompt run die at lease expiry with a committed report and
   a written verdict file and still pay for a full re-derivation.
2. **Two silent dispatch deaths + an ~18.6 h abandoned lease (NEW this
   pass)**: worker-53564 (08:09→08:56) and worker-21570 (08:56 →
   2026-10-30 03:35) each claimed the task and produced ZERO recoverable
   artifact — no commit, no report, no facts beyond claim/release. The
   worker-21570 hold locked exclusivity for ~18.6 h (overnight, with the
   pool budget-gated so no competing claim could force the reclaim until
   this pass claimed at 03:35:10). Two paid turns burned with nothing to
   build on, and a day of re-dispatch latency added. No row previously
   captured this shape; minted this pass (§f new row 1 + §g3).
3. **The filter-branch dead-cite explosion stands unhealed**: 969 lines at
   this re-run (8 pre-incident → 964 at the twenty-second → 969 now). The
   heal (1349) is executable and unstarted; the authorization ruling
   (1350) is BLOCKED on the owner; check-dead-sha-refs has ridden red
   ~57 h across five dispatch windows. Third incident of the
   history-rewrite class; guard row 726 still unbuilt.
4. **SC1007 red riding its SIXTH dispatch**: check-script-syntax
   113 ok / 1 failed at this re-run (SC1007, scripts/check-flag-count.sh:107
   — introduced by the canary sweep 0ae5ff83 on 2026-10-28 02:41, shipped
   RED because that task ran no syntax gate and wrote no closeout). The
   fix is a one-line quoting change (row 1330). Every pass since the
   seventeenth has re-counted it; none fixed it. Documentation is not a
   cure — the row needs a dispatch, not a citation.
5. **check-doc-refs red riding alongside** (rc=1 re-run this pass):
   AGENTS.md cites `scripts/check-erraudit.sh`; the script exists in no
   ref. Rows 1081/1311/1333 own it; landing it is a small project that has
   now been reported red by four consecutive passes.
6. **Footer-less daemon sweeps carried a concurrent session's work again**
   (2026-10-29 17:54–19:50, five sweeps 17af902c/b8b40cda/b7c84026/
   023bf446/3b7de9b5: the `tq-bash-guard` + `tq-edit-advice` Crush
   PreToolUse hooks, `.crushrc` wiring, AGENTS.md hooks bullet + budget
   re-tighten, CHANGELOG row, facts_json_test + check-agents-size
   changes). Content read, judged sound, and built on (never reverted) —
   but it entered history unattributed, the exact class rows 1319/1289
   exist to bound. Not this window's work; noted in passing per the
   parallel-changes rule.

## e) WHAT WE SHOULD IMPROVE

1. **Make the verdict the deliverable, not the report.** Six passes wrote
   correct reports; the close failed on the last 60 seconds. Until row
   1335's receipt leg exists, the working discipline is: commit →
   `tq verdict` in the SAME bash invocation as the commit, then verify the
   task's completion fact exists before exiting (this pass does exactly
   that). The 22nd proves even that is not airtight while the release can
   race the exit — the leg is the cure.
2. **Checkpoint-early protocol for long tasks.** The two silent deaths
   wrote nothing in ~45 min each (the 21st at least had its report
   recoverable via the daemon sweep). A done-prompt run should write a
   report skeleton to docs/status/ and commit it EARLY (minute ~10, not
   minute ~40), then amend — converting a released run into recoverable
   progress instead of a total loss.
3. **Done-prompt briefs should cite the canonical derivation.** Once one
   report derives the window fully (the seventeenth), a re-dispatch's job
   is delta verification + the close contract — not re-reading 15
   closeouts from scratch. The dispatch text could name the canonical
   report (kin of row 1273's economics ruling, owner-gated; this pass
   operated on exactly that basis).
4. **Abandoned claims need a liveness-based reclaim path.** Lease-expiry
   reclaim today fires only when a competing claim arrives; a hung claimant
   + an idle/budget-gated pool = indefinite exclusivity (the 18.6 h hold).
   A heartbeat-staleness reaper, or a max-lease cap for status-type tasks,
   would have re-dispatched this window a day earlier.
5. **The mechanical fixes for the recurring classes remain unbuilt after
   being reported N times each**: span guard (1319), retitling (1289),
   history-rewrite guard (726), verdict receipt (1335), SC1007 (1330).
   The reporting loop is saturated; only dispatches close these. A
   "highest-leverage unbuilt tooling" dispatch (SC1007 + 1335 + 1319 in
   one batch) is the cheapest possible unblock.

## f) UP TO 50 NEXT THINGS (dedup-checked against 624 open unchecked rows; two NEW rows minted this pass, both listed first)

1. **NEW (rowed this pass)**: Diagnose the two silent done-prompt dispatch
   deaths (worker-53564 08:09→08:56 2026-10-29, worker-21570
   08:56→2026-10-30 03:35) — zero artifacts, zero facts beyond
   claim/release; determine the death mode (context/token exhaustion,
   crash, lease kill) from whatever traces exist and design the
   checkpoint-early protocol (§e2).
2. **NEW-BLOCKED (rowed this pass)**: Abandoned-claim reclaim policy —
   should a claim with no progress/heartbeat for >N hours be reclaimable
   WITHOUT a competing claimant (the 18.6 h worker-21570 hold), and should
   artifact-less deaths consume attempt budget differently than
   artifact-producing ones?
3. Heal the filter-branch dead-cite explosion, one incident battery (row
   1349) — 969 lines and accruing.
4. Owner authorization ruling on the 2026-10-28 double filter-branch (row
   1350, BLOCKED) — gates #3's shape.
5. SC1007 one-line re-green (row 1330) — six dispatches of re-counting.
6. Verdict-receipt done-preflight leg (row 1335) — nine data points now.
7. Verdict-receipt contract pin in docs/status/README.md a)-g) skeleton +
   dispatch templates (row 1255).
8. Land `scripts/check-erraudit.sh` + ci-local leg; consolidate the erraudit
   twins (rows 1081/1311/1333) — clears check-doc-refs.
9. Retro closeout for the canary sweep (row 1331) and the webui backoff
   batch (row 1332); half-closeout policy ruling (row 1334, BLOCKED).
10. Mechanical history-rewrite guard (row 726) — third incident of the
    class.
11. Re-dispatch closure economics ruling (row 1273, BLOCKED) — ratify or
    reject the thin-delta basis this pass and the twentieth/nineteenth
    used.
12. Attempt-budget policy for re-dispatch storms (row 1345, BLOCKED) — now
    fifteen claims of evidence.
13. heal-daemon-sweep span guard (1319) + per-commit retitling (1289) +
    `--print-plan` (sixth-dispatch appends).
14. Scripted landed-but-unticked sweep tool with gate-state probes (1254
    family).
15. TODO-dedup detector script (1300) — channels recorded at row 1044's
    DONE note.
16. Postgres-conform smoke script (1226) — convert the store_test.go recipe
    to one command.
17. cmd/tq raw `GOWORK=off` build cure at the next tag wave (965 family) —
    `internal/dlqbackoff` is post-v0.3.4.
18. Divergence reconcile: rebase/merge the 59-commit local line onto
    origin/master + go-health v0.8.1 deep verification (root suite,
    release-gates, vendorHash) — NAF rows, risk accruing daily.
19. DonePreflight: make signal 4 (closeout report) batch-aware via ONE
    shared batch-landed predicate (00-09 §b1/§e1); owner MUST-policy
    ruling (row 973 family); emit the fall-through reason as observable
    record (00-09 §b2).
20. Retro-cure task 000001a1382a1765's unlanded members (rows 225/226,
    silent work loss since 2026-10-14).
21. Pool-health dirty-tree skip-streak lane split (post-carve-out the WARN
    can mean nothing — 10-27 10-35 §e1).
22. DOMAIN_LANGUAGE.md rows owed by landed work: dirty-tree park vs
    env-streak burn retry taxonomy; "provider" as requeue-evidence
    concept; batch landed-ness vocabulary.
23. main.go usage-line consistency pass for pool-health/dlq
    `--dlq-backoff`/`--config` (06-59 §f5).
24. `tq pool-health --json` top-level `"backingOff"` aggregate (06-59 §f2);
    `tq dlq` trailing summary counts (06-59 §f12).
25. Shared "dead-letter a repo" test-seed helper in poolhealth_test.go (8
    -line prologue now 6×) (06-59 §f3); dlq_test.go seeding unify (06-59
    §f4).
26. Cross-surface prose pin: pool-health WARN + `tq dlq` row line share the
    `fix the repo or rescue dead tasks` tail (06-59 §f7).
27. Rename `applyPoolHealthConfigDefault` → `applyDLQBackoffConfigDefault`
    (06-59 §f8).
28. Same-millisecond dead-letter tiebreak pin for `lastLiveDead` (04-18
    §f27; unix-milli Known Issue).
29. proxy-wait: single-source the proxy base URL (3 copies) + pin the two
    poke-WARN lines + lib-source stdout-silence pin (13-51 §e1-3, harvested
    rows exist).
30. Execute `scripts/smoke/dep-sweep.sh` once + the
    `TQ_DEPSWEEP_LIVE_CALIB` calibration (rows 987/296 family) —
    existence-level → execution-level.
31. `SetFailureEvidence` for depbump (row 241, re-verified absent).
32. statsPayload population-path golden (CLI-capture level, 12-44 §f1).
33. `commit-task.sh --stage-only` mode (12-44 §f2) — closes the daemon-split
    class at tool level.
34. Bound heal-daemon-sweep blocker detection to the sweep window (08-30
    §f2); per-commit retitle pairs (12-44 §f3).
35. `--self-test` + negative-path battery for the devmod drift preflight
    (09-03 appends); devmod floor-parity consolidation (row 667 family).
36. `|| true` swallow audit across scripts/ (09-03 appends).
37. Subshell fail-tally audit across smokes/checks (fixture-attr 00-52 §f2).
38. Extend TestParkedJSONSurfacesProvider to the `--parked-class` JSON path
    (07-28 §f5); parked round-trip golden (07-28 §f11).
39. doctorParked provider-list cap (>3 → "+N more") (07-28 §f20).
40. Rate-limit e2e smoke grep pin for the JSON provider field (07-28 §f10).
41. Pool-health summary-line WARN for resolved-config-missing-dlq-backoff
    (05-23 §f3); `dlqBackoffSource` in JSON (05-23 §f19).
42. Webui parked view renders the provider tag (07-28 §f7) — surface parity
    with tasks/JSON/doctor.
43. Parked-class + provider one-pass `latestRequeueEvidence` decider (07-28
    §f24) — folds with row 954's shape.
44. generic_return migration batches (owner-ruled adopt; ~197 sites) — the
    erraudit gate drops `--disable generic_return` when done.
45. November archive sweep + digest row (status index cadence; 624 open
    TODO rows vs 400 threshold — index bloat sweep L1137 family).
46. statusresume.go forensics + re-land decision (12-03 §f1); daemon
    zero-byte guard (12-03 §f2).
47. Dirty-tree park smoke: dirty tree → park → commit → complete on a
    scratch TQ_DB (10-35 §f13); parked-dirty-tree hint line on
    `tq tasks --parked` (10-35 §f2).
48. Done-preflight negative-path test: reclaimed batch with not-done
    preflight re-runs the executor (00-09 §f6); clock seam for the reclaim
    test (00-09 §e3).
49. CHANGELOG `[Unreleased]` release-cut sweep when v0.4.0 planning starts
    (row 466 family; the section now spans 2026-10-20 → 10-29 entries).
50. README/FEATURES one-glance docs for `tq tasks --parked --json`
    `provider` field (07-28 §f18 — CHANGELOG exists, sales docs don't).

## g) QUESTIONS I CANNOT ANSWER MYSELF

1. **Attempt budget for re-dispatch storms (row 1345, evidence now 15
   claims)**: should status/done-prompt tasks get an elevated attempt
   floor, park instead of burn on verify-gate deaths, or is the current
   budget correct? Nine claims produced ONE close (this pass) — the
   economics of the current budget are now measurable and bad.
2. **Re-dispatch closure economics (row 1273)**: may a re-claim close with
   a thin delta-only report + verdict once a canonical derivation exists
   and its pins verify clean (the basis this pass used), or is full
   re-derivation required every re-claim? Ratify or reject retroactively —
   five passes have now used the delta basis.
3. **Abandoned-claim reclaim policy (NEW, rowed BLOCKED)**: should a claim
   with no observable progress for >N hours be reclaimable without a
   competing claimant (worker-21570 held this task ~18.6 h), and should
   artifact-less dispatch deaths (two this window) consume attempt budget
   differently than artifact-producing ones? Kin of rows 1345/1273 but a
   distinct queue-mechanics decision.

## h) BAND DRIFT (ADR-0015 accountability)

**None recorded.** `tq facts -type task.reprioritized` over the production
journal (/mnt/pool/services/tq/tq.db) returned **0 facts** at this pass —
the SEVENTH consecutive zero census. No task priority moved via marker,
AI, unblock, or importance during the window or since; claim order moved
only via aging + new enqueues.

## Docs-health pass notes (this pass's own doc work)

- **Living docs verified current against the window's landed work**:
  README backoff command rows (README.md:345/349), FEATURES
  FULLY_FUNCTIONAL rows with test-pin cites (FEATURES.md:82/107/128),
  AGENTS.md dirty-tree exemption + `internal/dlqbackoff` row + hooks
  bullet, CHANGELOG `[Unreleased]` carrying every operator-visible window
  change with BEHAVIOR CHANGE flags (parked provider, doctor naming,
  dirty-tree carve-out, pool-health/dlq/serve backoff family, summary
  count). No drift found requiring correction; ROADMAP needed no routing
  (every window forward-item is already a TODO_LIST row).
- **No archives this pass**: the seventeenth archived the 2026-10-20
  done-prompt chain with the `digest 2026-10` row (standing ritual
  current). The remaining live done-prompt reports (17th–22nd) all carry
  UNRESOLVED forward items (SC1007, dead-cite heal, erraudit gate, retro
  closeouts, divergence) — per-item triage says none qualifies for
  `archived/` yet; the November sweep owns the next pass.
- **Index row added** for this report at the top chronological cluster.

## Verification receipts for THIS pass (2026-10-30 03-3x, all commands run this session)

- `date` clock-first: 2026-10-30 03-35 at session start (claim fact 17186,
  worker-10961, 03:35:10).
- Window pins re-grepped PRESENT: `TestParkedJSONSurfacesProvider`
  (cmd/tq/parked_test.go), `TestDirtyTreePreflightNeverBurnsAttempt`
  (internal/worker/worker_test.go),
  `TestDonePreflightBatchFooterRequiresAllMembersLanded`
  (internal/harvest/donepreflight_test.go),
  `TestReportBodiesCarryNoSecretShapedStrings`
  (internal/executor/redact_reports_test.go),
  `TestDLQBackoffPanelRenders` (internal/webui/backoff_test.go),
  `TestPoolHealthDLQBackoffBoundaryInstant` (cmd/tq/poolhealth_test.go).
- Gates re-run: `check-script-syntax` 113 ok / 1 failed (SC1007
  check-flag-count.sh:107; SC2120 advisory proxy-wait.sh:41);
  `check-dead-sha-refs` **969** DEAD SHA lines (rc=1);
  `check-doc-refs` rc=1 (MISSING `scripts/check-erraudit.sh`);
  `check-status-index` ok (2 pre-existing f26-class TRAILER WARNINGs);
  `check-agents-size` OK 23,799/23,900 B.
- `git rev-list --count origin/master..HEAD` = 59 (52 at the
  twenty-second; delta = 5 footer-less hook-session daemon sweeps + report
  commits).
- Journal (facts 16929–17186 read verbatim): fifteen claims on this task;
  releases 17103/17125/17145/17169/17185 all `[effect=unknown]`;
  budget-gate requeues 17011/17025/17029; verify-gate/environmental deaths
  16936/16970/17008; the two silent-death claims 17146 (worker-53564) and
  17170 (worker-21570) with no facts between claim and release.
- Delta since the twenty-second's commit (105c1cb6): exactly five daemon
  sweeps (17af902c, b8b40cda, b7c84026, 023bf446, 3b7de9b5 — the
  concurrent Crush-hooks session) + zero code from done-prompt passes.
- `tq facts -type task.reprioritized` = 0 (band drift §h).
- All closeout reports for the window re-read this pass or carried from
  the seventeenth's full derivation (13 reports); the two footer-only
  tasks verified via `git show --stat` (0ae5ff83; 3814abe1..ca63b282 +
  tick commits dbcf677e/ca63b282) and row 1064/1077 DONE notes.
