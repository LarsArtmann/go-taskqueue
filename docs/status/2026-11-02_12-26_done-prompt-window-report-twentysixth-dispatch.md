# Done-prompt window report — twenty-sixth dispatch

Written: 2026-11-02 12:18 CET. Window: the sixteen tasks completed
2026-10-30 05:31 → 2026-10-31 04:18 (thirteen with a)-g) closeout
reports under `docs/status/tasks/`, two reviewer-fix tickets that
completed via free preflight dismissal, one push-master ops task).
Method: all 13 closeout reports read first; every headline landing
re-verified at HEAD (file:line greps + gate runs); journal read
read-only at `/mnt/pool/services/tq/tq.db` (production `TQ_DB` of the
running `tq serve`); TODO dedup-checked against 615 open rows.

## a) FULLY DONE (verified, not just claimed)

1. **Web UI header-matrix hardening chain** (tasks …b0ce2017, …b1a9a232,
   …b317d28d + reviewer-fix tickets …b317e4294f, …b317e9d01, …b6d0061a):
   - serve-429 duplicate row collapsed + favicon row hoisted
     (`serve429Row`/`serveFaviconRow` package-level,
     internal/webui/headermatrix_test.go:203-211).
   - The 180-line matrix test split into six per-surface functions
     (`TestServeHeaderMatrix{TaskPage,401,429Lockout,HealthPage,
     HealthProbes,SDKBundle}`) plus the `TestServeHeaderMatrixRouteCoverage`
     meta-guard that walks `routeBindings()`+`healthBindings()` and fails
     on any new/renamed binding without an expectation
     (headermatrix_test.go:287-450). gocognit + noctx verified silent by
     scoped lint in the 05-51 closeout.
   - `withContentType` per-surface media-type pins (favicon image/svg+xml,
     probes application/json, task HTML text/html, SSE text/event-stream,
     SDK text/javascript) — observed-first, then pinned on the surface
     (headermatrix_test.go:41-46); `with(rows …)` is the single merge
     mechanism, `maps.Copy` exactly once (:139).
   - Reviewer findings cured in-ticket: BLOCKED annotation on the §g1 row
     (ae43dff0), both-rows overclaim corrected in report+index (c3e2d0d9),
     filterChip doc-comment fragment restored (74d34566).
2. **Failure-evidence surface** (tasks …b486056d, …b6cff639):
   `executor.LastFailureEvidence` + `LastFailureEvidenceFromTrail`
   (internal/executor/evidence.go:25/36) replaced both hand-rolled scans;
   `Summary()` renders the one `evidence=` suffix shape for `tq dlq` rows
   and the webui; `MatchesEvidenceStageSet` (:109) gives
   `tq dlq --evidence-stage` comma-separated terms with `!`-negation and
   the webui filter rides the same matcher; dead-only BY DESIGN pinned by
   `TestRequeuesCarryNoFailureEvidence`; real-binary smoke
   `scripts/smoke/dlq-evidence-stage.sh` (exists, COVERAGE.md-indexed).
   CHANGELOG Added rows present; README `tq dlq` row documents the
   grammar (README.md:384).
3. **Gates + pins** (tasks …b8d0aff5, …b9ac5f8e, …ba890359):
   - `check-status-index.sh` live-row path-existence branch
     (`row_path_violations`, :61) with negative-path self-test — and the
     reviewer-found VACUOUS first battery re-built with mutation-proofed,
     polarity-corrected fixtures (83b453a2, the …ba892a7020f ticket).
   - `executor.TestSecretPatternSamplesRawCountExactlyOne`
     (redact_test.go:245): every canonical sample counts exactly 1 RAW.
   - `tq audit --journal --json` wire contract pinned at Go level
     (cmd/tq/journalaudit_secret_json_test.go) + journal-drift.sh phase-6
     entry-cell pins.
   - AGENTS.md dry-run-first convention bullet landed WITH the conscious
     budget raise 23,900→24,400 (cmd/tq/facts_json_test.go:94) and the
     size-guard twin synced in-commit;
     `scripts/check-prompt-const-folds.sh` (+ self-test) wired into
     ci-local (ci-local.sh:172-175); check-commit-msg-hook case count
     derived, not asserted.
4. **Status-index hygiene** (task …fe3eac52): `digest 2026-11`
   PROSPECTIVE row minted (docs/status/README.md:130; since accrualed to
   "4 reports archived so far" by the T13 November sweep), INDEX BLOAT
   warning re-worded to the single ratified ritual, and the ANNOTATE pass
   over all 32 live 2026-09-16..09-25 reports (inline dispositions, 4
   archive candidates — since archived).
5. **Push-master ops task** (…f208e533): vendorHash resync landed as
   e65b8447 and IS pushed (ancestor of origin/master). Origin CI is green
   on the current master incl. the previously-red `nix` and `consumer`
   jobs — verified run 41362835745: govulncheck/test-windows/nix/
   test-postgres/test/gosec/consumer/cqrs-lint all success (2026-11-02).
6. **The two footer-less reviewer tickets** (…ba892ab94, …ba892bb30)
   completed at 2026-10-30 23:23:49 with `session_cost_usd: 0` — preflight
   dismissal "fix ticket cured (anchor text absent at HEAD)", not a paid
   no-op. Correctly deduped against sibling-ticket cures.

## b) PARTIALLY DONE

1. The 05-31 batch landed 1 of 3 items: the two SECURITY.md rows are
   owner-gated by construction (§g1 generation-vs-pin ceiling, §g2
   bare-probe Cache-Control posture) and correctly sit BLOCKED
   (TODO rows at 1201/1202 today). The reviewer round-trips those two
   omissions caused are the honest cost — the annotations eventually
   landed (ae43dff0).
2. The digest ritual's archive half for this window's flagged candidates
   is done (T13 moved 4), but the ritual remains an open cadence — the
   `digest 2026-11` row accrues in place; no further sweep ran in-window.
3. Attribution fragmentation: several window batches interleaved with
   footer-less daemon sweeps (83a6a9ca/ea3bc185 in the 04-18 batch;
   aeff1489-class multi-item folds in the 08-06 batch). All work is at
   HEAD and correct; the footer channel cannot claim it. One orphaned
   sweep (56d949ad→3c875802) stayed dead-cited until repaired by this pass (§d1).

## c) NOT STARTED

1. The two owner-gated SECURITY.md rows (no agent may force them) — still
   the oldest BLOCKED pair from this window.
2. Both zero-footer reviewer tickets' *finding* was already cured by
   siblings, so nothing of theirs was left to start (verified by the
   preflight dismissal detail in the journal).
3. Window §f follow-ups were exhaustively rowed; the concurrent T13-T27
   batches (2026-10-31→11-01) have since TICKED most of them (rows
   1403, 1408-1421, 1433, 1458 marked DONE 2026-10-31). The residue is
   tracked, not lost — see (f).

## d) TOTALLY FUCKED UP

1. **A live broken gate greeted this pass**: `check-dead-sha-refs.sh`
   rc=1 at HEAD — `56d949ad→3c875802` (the 00-25 wave-5 heal's own daemon sweep,
   orphaned by its rewrite) cited un-repaired at
   docs/status/README.md:132 and in the 00-25 report. The heal PRINTED
   fork records but never wrote the `old→new` arrows into the citing
   docs. Repaired here (fork record `56d949ad→3c875802`, tree-identical
   twin verified; gate now rc=0, 1752 baselined / 0 new) — but the class
   is structural: heals that don't write arrows leave the gate red.
2. **The window shipped a vacuous gate battery that stood ~5 days**: row
   1209's path-gate self-test had unbackticked fixtures the extraction
   never matched AND an inverted rc assertion (found by reviewer, cured
   83b453a2). A battery that cannot fail launders a gate through review.
   The fleet-wide mutation-proof leg is still open (row 1438).
3. **Reviewer-finding tickets cost 4 paid dispatches** for what were
   mostly honesty defects of the window's own batches (1b2fee1d "both
   rows blocked" claim vs a one-line diff; the df3ce1c0-class vacuous
   battery). Two of the four re-dispatches then completed free via
   preflight dedup — the queue's dedup worked, but the findings were
   self-inflicted.
4. **AGENTS.md budget is structurally exhausted**: raised 23,900→24,400
   inside this window (08-21 batch), then pruned, then grown again —
   24,398/24,400 at this pass (2 B headroom). Every doc touch re-trips
   TestAgentsDocSizeGuard; the re-tighten row (1302) is still open.
5. **Daemon-race self-flags in six consecutive windows** (edit → battery
   → commit inversions; multi-item sweeps needing manual soft-reset
   splits). The conventions exist; enforcement does not. Open rows: heal
   interleave shape (1395), `--split` mode (1221), span guard (1330).

## e) WHAT WE SHOULD IMPROVE

1. **Mutation-proof every negative battery at landing** — the 00-15
   incident is the existence proof; row 1438 owns the sweep. Green rc
   proves nothing about a battery.
2. **Heal should WRITE the fork records, not print them** — either rewrite
   citing lines old→new directly or persist a fork-records file the
   dead-sha gate consumes; today the gate stays red until a later session
   hand-repairs (new row appended, §f6).
3. **Commit-per-edit-batch** remains the highest-leverage discipline (the
   window's daemon-race flags are all ordering inversions); `--split` heal
   support (row 1221) removes the risky manual recovery.
4. **AGENTS.md needs a terminal budget policy**, not another raise (§g3).
5. **Re-dispatch dedup should prefer row-text over footer grep** — two of
   this window's near-double-mints were caught only by the TODO dedup
   header, not the specified footer check (08-06 §d3; convention line
   still unlanded for budget reasons).
6. **Verdict channel**: early-window dispatches (05-31..06-22) ran without
   `TQ_RESULT_FILE` and flagged half-closeouts; later windows record
   same-session. The doctor verdict-channel probe (2026-11-01) now warns
   on the PATH gap — keep the ritual.

## f) UP TO 50 NEXT THINGS

Ranked; the six genuinely-new items were appended to TODO_LIST (§g
questions as BLOCKED rows there too; cap 10 respected). Rowed items cite
the standing row — do NOT re-mint them.

1. NEW — Decide the hop-by-hop `Connection` header's home: global
   transportHeaders ignore vs per-SSE-row conditional (05-51 §f8;
   appended).
2. NEW — Pin Cache-Control absence for the whole API column (observed
   absent everywhere; only serve rows pin today) (06-09 §f14; appended).
3. NEW — Harden the route meta-guard tripwires: non-empty sorted-deduped
   boundPaths + mutation-proof that removing an expectations entry fails
   (05-51 §f13/f14; appended).
4. NEW — One `-race` exercise of TestServeHeaderMatrixRouteCoverage in
   ci-local (05-51 §f18; appended).
5. NEW — Sweep check-*.sh summaries for arithmetic not derived from a
   counter (the `fails+1` fabricated-ok class) (00-15 §f10; appended).
6. NEW — heal-daemon-sweep: write fork records into citing docs or a
   gate-readable file (this report §d1/e2; appended).
7. Row 1201/1202 (BLOCKED): SECURITY.md §g1/§g2 owner rulings — oldest
   open pair; every other header-matrix follow-up depends on one of them.
8. Row 1390: consumer-job CI red diagnosis — NOTE: origin CI is green now
   (run 41362835745 consumer success); the row needs the diagnosis
   recorded, the red itself is gone.
9. Row 1438: mutation-proof the four convention-named countable gates,
   then the fleet (the (d2) cure at scale).
10. Row 1220: journal-drift phase-6 sabotage leg (prove the entry-cell
    pins can fail).
11. Row 1221: heal-daemon-sweep `--split` for multi-item sweeps.
12. Row 1395: heal interleave shape (footer–sweep–footer).
13. Row 1330: heal span guard (foreign-content refusal).
14. Row 1302: consciously re-tighten the AGENTS budget constant (~22k).
15. Row 1404: promote UNBLOCKED-OWNER-GATED to hard gate for a batch's
    own anchors.
16. Row 1406: index-row BLOCKED/count-claim audit across the last 7 days.
17. Row 1407: finding-envelope dedup hash (platform ask).
18. Rows 1400/1401: failingTB withContentType negative path; SECURITY.md
    Content-Type column (both gated on §g1/§g2 shape).
19. Row 1402/1422: cite a root-gate.sh rc in a window report (twice
    minted, still uncited).
20. Row 1439: `--self-test-paths` fixture marker (vacuous-against-real-
    repo guard).
21. Row 1440: row_path_violations per-path vs per-row counting ruling.
22. Row 1441: full ci-local end-to-end green run post-83b453a2.
23. Rows 1426/1427: templ drift gate + CLI/go.mod version alignment
    (v0.3.1020 vs v0.3.1070 skew).
24. Row 1428: comment-fragment lint (orphaned doc-comment class).
25. Row 1429: heal refusal should print the amend fallback recipe.
26. Row 1430/1431/1432: path-gate `--mine` scoping; countable summary on
    the real run; digest-row exemption pin.
27. Row 1434/1435: `--self-test-paths` seam documentation; RULING-BY-AGENT
    marker convention (backfill row 1216).
28. Row 1171: finish retryx graduation (executor/depbump.go:586+590).
29. Row 1173/1174 (BLOCKED): review-fix twin semantics; fleet-red posture.
30. Rows 1230-1232: commit-msg hook CRLF case; fixup/merge footer shapes;
    stale footer-wording sweep.
31. Row 1233: AGENTS Known Issues line for prompt-const dedup-invisibility
    (the DETECTOR landed; the LINE is the deliverable — do not mis-tick).
32. Rows 1253/1220-class: milli-granularity seeding in journal-drift
    phase 5.
33. Row 1266: open-row triage via the digest ritual (landed-work prune).
34. Row 1267/1263: dead-SHA citation cure + heal blocker bounding.
35. Row 1273: audit open rows citing unreachable anchor SHAs.
36. Row 1287/1288: statusresume.go forensic + zero-byte sweep guard.
37. Row 1290: scripted landed-but-unticked sweep tool (check-unticked.sh).
38. Row 1294: re-anchor "row N" cites inside scripts to content anchors.
39. Row 1306: `{name, sample}` struct fixture for secretPatternSamples.
40. Row 1311: TODO-dedup detector (check-todo-dedup.sh).
41. Row 1337: TestConcurrentClientsRace -race triage (webui).
42. Row 1341: SC1007 re-green in check-flag-count.sh (verify first —
    00-20 reported script-syntax 127/0; the row may be landable).
43. Rows 1344: consolidate the erraudit gate rows (owner ruling pending).
44. Row 1365: checkpoint-early protocol for long done-prompt tasks.
45. Row 1366 (BLOCKED): abandoned-claim liveness policy.
46. Row 1370: known-issue doc pass for the mixed-reset maneuver.
47. Row 1448: L4 client-server pilot + memory measurement (owner-gated).
48. Row 1452: TestShowJSONCarriesFailureEvidence order-dependence.
49. Rows 1453-1457: hook-hatch table, rm→trash advisory, ADOPTION.md
    guard note, fixtures-under-/tmp advisory.
50. Row 1470/1471: OKF additive-growth pin + signals-not-scores placement
    (docs-only, small).

## g) QUESTIONS (cannot self-answer)

1. **Band-drift channel is silent**: the production journal holds ZERO
   `task.reprioritized` facts — ever, not just this window (verified
   read-only 2026-11-02) — while FEATURES.md ships the fact +
   `UpdatePendingPriority`. Should marker/keyword/unblock bumps emit the
   fact so ADR-0015 accountability has data, or is the silent channel
   accepted (nothing has ever re-prioritized post-enqueue)?
2. **Row-1216 agent-made ruling**: "indexed-as-convention WINS" for
   tasks/ closeout rows (deletion requires same-change row repoint) was
   decided by an agent under the standing dispatch mandate. Confirm, or
   flip to deletable-as-drift (one comment + one self-test case)?
3. **AGENTS.md size-budget terminal policy**: the budget was raised
   23,900→24,400 in-window and sits at 2 B headroom. Rule the default:
   compress-only with raises requiring an explicit owner row, or
   agent-side raise remains acceptable when batch-mandated?

## h) BAND DRIFT

**None recorded.** The production journal
(`/mnt/pool/services/tq/tq.db`, read-only query 2026-11-02 ~12:10) holds
zero `task.reprioritized` facts for the window (2026-10-30 00:00 UTC →
now) and zero since the beginning of the journal — the fact type exists
in code (FEATURES.md priority row) but has never fired. No marker, AI,
unblock, or importance move is therefore explainable or missing: there is
nothing to account for. (Corollary filed as question §g1.)

## Battery (this pass)

- Root: `go build ./... && go vet ./...` rc=0 (GOCACHE=/tmp/go-build-cache,
  GOEXPERIMENT=jsonv2; /tmp/tq-root-battery.log).
- `scripts/check-agents-size.sh`: OK 24,398/24,400 B.
- `scripts/check-todo-list.sh`: rc=0 (10 advisory warns, 0 structural).
- `scripts/check-status-index.sh`: rc=0 (2 pre-existing f26 TRAILER
  WARNINGs, owner-gated class).
- `scripts/check-doc-refs.sh`: rc=0 (the check-erraudit.sh red from the
  window era is cured — the script exists now).
- `scripts/check-dead-sha-refs.sh`: rc=1 before this pass (2 new dead
  cites, 56d949ad→3c875802) → repaired with the verified fork record →
  rc=0 (1753 baselined, 0 new; /tmp/tq-dead-sha5.log).
- CI: origin master green — run 41362835745, all 8 jobs success
  (verified via `gh run view`, 2026-11-02).
