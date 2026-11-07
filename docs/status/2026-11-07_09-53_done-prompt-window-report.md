# Done-Prompt Window Report — 2026-11-07 09:53

Window: ~38 dispatched tasks, closeouts dated 2026-11-02 17:06 → 2026-11-07 09:29
(task 000001a19bd5df299c… through 000001a1b38f7fcb…). This report's own task:
000001a1b52794852387151aa59c00000000. Every claim below cites the task's own
closeout report under `docs/status/tasks/` plus a spot-verification at HEAD where
performed.

## a) FULLY DONE

Verified landed (closeout evidence + artifact exists at HEAD):

1. **TQ_RESULT stdout-channel deletion residue** (…33286, closeout
   11-02_17-06): the executor flip to file-channel-only was already at HEAD
   (`internal/executor/result.go:379`); this task fixed the stale "stdout is
   only a fallback" lie in `verdictRailTmpl` (`internal/executor/agent.go:586`)
   + prompt pin; commit dcd2d542.
2. **Counter-key upstream recheck** (…423000, 11-02_17-38): clean-room repro
   harness in go-cqrs-lite `scratch/counterkey/repro_test.go` (6928da434); bug
   did NOT reproduce — nothing filed upstream (verify-before-filing honored).
3. **Cross-engine counterkey sweep** (…c8de500, 11-02_18-08):
   `cross_engine_test.go` over memory/pebble/badger/duckdb — nothing
   reproduces; search space closed (fcd675141). Two-tick verify-before-filing
   convention landed in AGENTS.md (57142420).
4. **Verdict→outcome calibration dataset** (…ba940000, 11-02_18-47):
   `executor.CalibrationRow` in `internal/executor/calibration.go` (verified at
   HEAD) — every paid run appends one JSONL row to
   `$TQ_LOG_DIR/calibration.jsonl` (990d38e5).
5. **CRLF commit-msg pin + fixup/squash verification** (…46e10000,
   11-02_19-04): hook pin cases in `scripts/check-commit-msg-hook.sh`
   (87e5aaf6, 20b34428); 15 hook cases green.
6. **Prompt-const dedup-invisibility Known Issue** (…c0000000,
   11-02_19-43): AGENTS.md bullet (73fc92da); pre-push guard now walks the full
   push range via rev-list (e29506c7). (This bullet is in AGENTS.md today.)
7. **Push-timing `--check` dry-run** (…35400000, 11-02_20-04 → closeout
   20-32): `scripts/check-push-timing.sh --check <sha>` with 0/1/2 exit
   contract + hermeticity hooks (25579d62); O14 Internal-row ruling recorded.
8. **Parked-JSON provider-join test extension** (…c0000000, 11-02_21-26):
   `TestParkedJSONSurfacesProvider` provider-join phase in
   cmd/tq/parked_test.go; ratelimit-e2e greps real JSON `"provider": "zai"`.
9. **Parked wire-shape golden pin + doctor cap + evidence fold** (…d0000000,
   11-02_22-23): `TestParkedTaskRowJSONWireShape` (0843a8d8), cappedProviderList
   in `cmd/tq/doctor.go` (bed52bca), single-pass `latestRequeueEvidence`
   serving 5 surfaces (7abed40d).
10. **Webui parked provider tag** (…00000000, 11-02_22-45): parked view renders
    `parked: [<provider>]` via `parkedRequeueEvidence`
    (`internal/webui/components.go`, 9af1014c). Premise-check-first and
    gate-receipt conventions landed in AGENTS.md (f2bbbb23, 55ab0d6d).
11. **Reviewer-fix disposition audit** (…80000000, 11-02_22-59):
    journal-verified (read-only probe of the production journal) that task
    …189a00000000 was completed zero-cost by the done-preflight; first wrong
    disposition retracted in-session (4116f8db).
12. **statusresume forensic** (…10000000, 11-02_23-15): empty file was a stray
    untracked artifact; deletion lossless (e03d0247); zero-byte source guard
    installed by the pre-commit installer, live-verified (39cd5471).
13. **check-unticked.sh** (…00000000, 11-02_23-38): landed-but-unticked sweep
    tool + self-test + ci-local wiring (f74b192d); real run 102 candidates /
    676 rows / 253 symbols.
14. **"row N" cite re-anchoring** (…00000000, 11-02_23-50): ~12 rotted cites
    re-anchored across 14 scripts (a22aa57c).
15. **erraudit gate + tool bugs** (…00000000 / …e2000000, 11-03_00-21/00-46):
    gate verified green (947195ec); issues #12/#13/#14 filed (ecf2b602); all
    three FIXED in the erraudit repo (05cddefb/24374215/4d6980ca — local only,
    see §b); generic_return first wave 4 modules/5 sites (91123a3d).
16. **TestConcurrentClientsRace flake** (…ee8000000, 11-03_00-55): root-caused
    (SSE stream never closes; 500 ms collector window at fault) and fixed
    500ms→5s (8f6f02c8).
17. **shellcheck SC1007 re-green** (…1c9fa000, 11-03_01-07):
    check-flag-count.sh env-routed assignment (73b7adb4); syntax gate 129/0.
18. **Windows-vet / ci.yml decouple / README demoted-serve** (…d9f1000000,
    11-03_01-46): ci.yml windows gate moved after the smoke cluster
    (350f2eca); README ruling paragraph (34755b87); windows-vet row ticked
    PREMISE-STALE (battery existed since 2026-09-07).
19. **Filter-branch dead-cite heal** (…c8590000, 11-03_02-06): 43 dead SHAs
    from the 2026-10-28 double rewrite healed into fork records across 16
    files (14a5e9e3).
20. **Commit-before-gate advisory + rewrite-rate probe + shrink chain**
    (…4c100000, 11-03_02-56 → 03-18/03-45/04-40/05-12): tq-bash-guard
    dirty-gate advisory scope-matched (2f85808a); `rewrite-rate-probe.sh` wired
    into ci-local WARN + nightly ledger with amend counting and per-actor
    attribution (7731e5f9, abe9c8d2); `check-dead-sha-refs --shrink/--check`
    with committed snapshot + ~4.5× perf (54s→12s gate) + `--json`
    (104917cc, 6459bd47); advisory side-channel `TQ_HOOK_ADVISORY_LOG`
    (cd2966fd); per-occurrence window-scan dedup bug fix surfaced 18 real dead
    cites, all healed (eb401d1a).
21. **Checkpoint-early protocol** (…b4000000, 11-03_05-28): status prompt now
    carries checkpoint-early; silent deaths diagnosed as pre-spawn executor
    death + lazy reclaim under budget gating (c7dcc202, pinned by
    `TestStatusPromptCarriesCheckpointEarly` — verified in
    internal/executor/status_test.go:400).
22. **consumer-job CI red triage** (…d6d00000, 11-03_05-41): go.sum/missing
    retryx tag-wave issue diagnosed, no code change needed (9ae55e91 et al.);
    `gh run view --log-failed` triage recipe landed in AGENTS.md (15f3c8c7).
23. **embed-tidy gate + testastguard module** (…c0000000, 11-03_06-13):
    `scripts/check-embed-tidy.sh` wired into ci-local (first live catch: 208
    stale go.sum entries); `internal/testastguard` v0.1.0 wired into every
    graph loader (verified in root go.mod + cmd/tq/go.work); embed
    replace-set KEEP-FOREVER ruling (4644bc3c).
24. **headermatrix negative path + SECURITY doc matrix census** (…00000000,
    11-03_06-49): withContentType mismatch fixture (ff87768e);
    `TestSecurityDocMatrixMatchesGoMatrix` bidirectional census
    (46429e9d/715702fd); root-gate rc=0 receipt.
25. **L4 memory pilot** (…5a00000000, 11-03_07-21): crush-server-memory.sh
    (f6093af1 et al.): shared-server RSS does NOT accumulate per wave
    (baseline 55-58 MB, peak 110-135 MB @4 concurrent, wave-2 quiescent never
    above wave-1); `TestShowJSONCarriesFailureEvidence` flake fixed via
    writer-parameterized `showTaskJSON` (d99afecc).
26. **Hook advisories trio** (…0540000000, 11-03_07-36): ADOPTION.md edit-advice
    note (20d71351), never-rm trash advisory (2283b056),
    heredoc-fixture-under-/tmp advisory (cfb5c0da).
27. **captureStdout sweep** (…300000000, 11-03_08-25): all 11 t.Parallel global
    capture tests converted to writer-parameterized renders + AST guard
    `TestNoParallelGlobalStdoutCapture` (de958597); shared
    `scripts/smoke/lib/crush-server.sh` (verified at HEAD); 12 dead SHAs
    healed (09a4f4e0).
28. **Mirror-clone baseline widening** (…fd9000000, 11-07_09-29, commit
    18ab39a4): known-acceptable adapter pairs pinned in
    scripts/mirror-baseline.txt + `--show-suppressed`; AGENTS.md channels
    one-liner + both standing nix receipts green.

## b) PARTIALLY DONE

- **erraudit repo unpushed**: all three bug fixes are LOCAL ONLY — 4 commits
  ahead of origin, issues #12-#14 still open, no CI run, no CHANGELOG entries
  (closeout 11-03_00-46 §b).
- **generic_return migration**: 5 of ~211 sites across 4 modules; remaining
  batches BLOCKED on the interface-satisfaction structural class (row 1392).
- **check-unticked.sh gate-state red-claim matcher**: deferred half, rowed
  (TODO 1690).
- **"row N" sweep**: only the named cite set fixed; ~70+ live cites remain
  repo-wide (closeout 11-02_23-50 §b).
- **Advisory side-channel**: code landed but nothing sets `TQ_HOOK_ADVISORY_LOG`
  in the real hook environment (crushrc is crush-config's, not this repo's) —
  rowed (TODO 1712).
- **Rewrite-rate ledger**: one row; the ~28-day backfill row is open (TODO
  1713); adaptive-p95 default flip is a deferred owner call.
- **ci.yml decouple**: structurally verified only; behavioral origin-CI
  confirmation rowed (TODO 1704).
- **L4 pilot**: measured at N=4 mock-provider floors; production one-repo
  flip is owner-gated (row 1535 et al.).
- **Calibration dataset**: no reader/consumer, success-path rows only
  (closeout 11-02_18-47 §b) — unrowed residue, minted below.
- **Hook advisory FP-free claim**: self-test-scoped only, no live advisory-log
  window yet; AGENTS hook bullet does not list the new rules (budget-gated).
- **Two-tick convention**: landed here, not fanned to go-cqrs-lite (closeout
  11-02_18-08 §b).

## c) NOT STARTED

- The window's own dispatches were all executed; no dispatched task was found
  skipped. The standing backlog (758 unchecked rows, 153 BLOCKED) was untouched
  outside each task's own residue — as designed: dedup held (see §e process
  note), so no re-fire loop occurred this window.

## d) TOTALLY FUCKED UP

- **Daemon vs agent race is THE recurring incident**: footer-less
  `chore: auto-commit` sweeps (with `NoVerify: true`, pma
  committer/committer.go:112) stole uncommitted work in at least 15 of the 38
  sessions; several sweeps were unhealable (non-contiguous spans, AI-titled
  subjects, one with a fabricated `Signed-off-by: Developer`), leaving tick-only
  footer commits while script content rode unclaimed sweeps (closeout
  11-03_03-45 §d). The edit→commit→battery ordering was violated repeatedly
  DESPITE being a pinned convention.
- **Typed-nil regression shipped then amended** (closeout 11-03_00-21 §d):
  the generic_return first wave broke cmd/tq dlq_test cross-module; caught and
  fixed in-session, but it shipped red through an amend.
- **Over-dropped shrink**: the first `--shrink --write` over-dropped 17 rows
  and turned the dead-sha gate RED (empty-scope bug; two restores,
  closeout 11-03_02-56 §d).
- **Three CHANGELOG Internal-row misses in one day** (O14 policy; closeouts
  11-02_18-47/19-04/19-43) — each cured late.
- **Committing over budget**: item committed with AGENTS.md 29 B over the size
  guard, caught late (closeout 11-03_04-40 §d); AGENTS.md is now at 26,396 /
  26,400 B — 4 bytes of headroom, any future edit lands red.
- **Templ CLI version skew** churned 384 unrelated lines into a feature commit
  (closeout 11-02_22-45 §d); pin row exists (TODO 1514).
- **Inference encoded as fact**: the reviewer-fix disposition was first wrong
  and had to be retracted (closeout 11-02_22-59 §d); assertion-first pinning
  and premise-skipping cost cycles in at least 5 sessions despite the
  premise-check convention landing mid-window.
- **Self-minted dead cites**: two sessions' own DONE notes/reports cited
  pre-amend or unpushed SHAs and had to be healed (closeouts 11-03_02-06,
  05-12).
- **Weak-anchor false-cure class**: the done-preflight can "cure" findings on
  anchors that can never match HEAD — audited, rowed, unguarded (TODO 1678).
- **Daemon hook bypass**: the zero-byte guard (and every repo hook) does not
  fire on daemon sweeps — the exact class it targets remains open on the
  daemon path (row 1685).

## e) WHAT WE SHOULD IMPROVE

1. **Make commit-first mechanical**: the ordering convention failed ~15 times
   in one window. A PreToolUse rule that DENIES running any gate/test while a
   tracked file is modified-unstaged would convert the convention from advice
   to enforcement.
2. **Heal-daemon-sweep needs AI-subject handling and blocker auto-walk** —
   both failures recurred (closeouts 11-03_08-25, 02-06).
3. **Closeout-time §f harvesting worked**: nearly every residue item was
   already rowed by the task's own session — dedup held window-wide. Keep
   this; it is the single best process win visible in the window.
4. **Budget gates should fire pre-commit, not post**: AGENTS.md size-guard
   overruns were caught after commits twice; the edit-advice hook should
   hard-warn at edit time when headroom < 50 B.
5. **CHANGELOG Internal-row reflex**: three same-day misses suggest the
   closeout prompt should carry an explicit O14 checklist line.
6. **Observe-first / premise-check are still being skipped under time
   pressure** (assertion-first pinning burned cycles in 5 sessions). The
   conventions landed; enforcement is the gap.
7. **Gate receipts**: logs living only in /tmp kept coming back (two sessions);
   archive-evidence routing should be part of the receipt convention.

## f) NEXT THINGS

Consolidated from all 38 closeouts (each task's §f items were verified against
TODO_LIST; the overwhelming majority were already rowed by their own sessions —
duplicates NOT re-minted here). Highest-value open items, existing rows
cited:

1. Row 1685 — close the daemon `--no-verify` hook bypass (owner ruling + pma
   mirror guard).
2. Row 1392/1394 — generic_return migration continuation (blocked on the
   interface-satisfaction owner ruling).
3. Row 1678 — guard the weak-anchor false-cure class in the done-preflight.
4. Row 1690/1691 — check-unticked red-claim matcher + prose-token pin.
5. Row 1712/1713 — advisory-log crushrc wiring + rewrite-rate ledger backfill.
6. Row 1704 — behavioral origin-CI verification of the ci.yml decouple.
7. Row 1535-follow-ups — N=8/16 crush-server memory sweep + production flip GO.
8. Row 60/48 — dead-SHA counterpart heuristic order pin + verify-fork-record.sh.
9. Row 1514 — templ CLI/go.mod version-alignment pin.
10. Row 1651 — O14 two-axis decision rubric in check-release-docs output.

NEW residue minted this pass (see TODO appends): parked-class JSON provider
parity gap; json.Unmarshal slice-reuse test sweep; latestRequeueEvidence unit
pins; calibration dataset consumer; SSE-test window-floor Known Issue +
upstream ssetest doc fix; shared stub-agent smoke lib; erraudit push/close
loop; webui parked-tag render pin; unknown-class normalization owner question.

## g) QUESTIONS FOR THE OWNER

1. **Parked-evidence normalization** (closeout 11-02_22-23 §g): should an
   unknown `requeue_class` normalize to a named-unknown tag or to
   zero-evidence (surfaces currently disagree on the edge)?
2. **Adaptive-p95 default** for the rewrite-rate probe: flip the default once
   the ledger has ≥10 real rows, or keep constant MAX=3 (row 65 defers this)?
3. **erraudit repo push policy**: issues #12-#14 are fixed in 4 unpushed local
   commits — push and close the issues now, or batch with the next erraudit
   release?

## h) BAND DRIFT

**none recorded.** Verified both in the local journal (tasks.db) and in a
read-only probe of the production journal (/mnt/pool/services/tq/tq.db): zero
`task.reprioritized` facts exist — the reprioritize machinery has never fired
on this pool (type census: claimed/enqueued/completed/failed/requeued/
cancelled/dead-lettered only). This matches the standing owner question that
the band-drift channel is silent; until a reprioritized fact is ever recorded,
this section will read "none recorded".
