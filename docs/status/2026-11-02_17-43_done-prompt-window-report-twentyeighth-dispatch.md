# Done-prompt window report — twenty-eighth dispatch (2026-11-02 17:43)

**Window**: 2026-10-30 05:2x – 2026-10-31 04:21 (17 tasks; footer commits `df2addff` … `1b5a7825`).
**Method**: every task's own closeout report read first, then each cited work commit verified to exist at HEAD (`git log -1` per hash); journal queried read-only for band drift. NOTE: the 26th (12:26) and 27th (17:16) dispatches already covered this identical 17-task window — this pass is the third read of the same evidence; the dispatch-time dedup row minted by the 26th dispatch (last row of its §f block) names exactly this repetition class.

## a) FULLY DONE (verified)

1. **Header-matrix test hygiene, webui** — `internal/webui/headermatrix_test.go` fully reworked across four tasks:
   - Duplicate `serve429` row collapsed onto the package-level row; favicon row hoisted (`df2addff` window; work commits per closeout 494b4820).
   - `TestServeHeaderMatrix` split into six per-surface functions (task page / 401 / 429 / health page / probes / SDK bundle) behind `newServeServer`; gocognit 53>25 cured, verified by scoped lint (`cb85ee51` window; work `ee2673dd`). noctx modernization (7 `http.Get` → context-aware helper) and `TestServeHeaderMatrixRouteCoverage` (route-coverage guard over `routeBindings()`+`healthBindings()`) landed in the same batch.
   - Content-Type pinned per surface (probes/API `application/json`, HTML `text/html; charset=utf-8`, SSE `text/event-stream`, favicon `image/svg+xml`, SDK bundle `text/javascript`) — observe-first honored with a live capture that caught a wrong `/healthz` guess (`c5fa8f9d` window; work `f62299d1`).
   - `with(rows …)` merge helper collapsed the file's merge idioms; `maps.Copy` down to one site.
2. **Shared failure-evidence reader** (`12c3049a` window; work `ad3e85cb`): `executor.LastFailureEvidence`/`LastFailureEvidenceFromTrail` in `internal/executor/evidence.go` — ONE backward journal scan, both hand-rolled copies (dlqfix `lastFailureEvidence`, cmd/tq `failureEvidenceFor`) deleted. `FailureEvidence.Summary()` makes `tq dlq` rows self-describing (` evidence=…` suffix, one shape CLI+web).
3. **Webui DLQ evidence-stage filter** (work `6b7ce2c0`): `?evidence=` FilterState + chip + select in the FilterBar; matcher moved into executor so alias knowledge is ONE copy. Templ regenerated, CSS rebuilt, CHANGELOG Added rows landed.
4. **`tq dlq --evidence-stage` set filter** (`e9168c2c` window; work `bccc75ac`): comma-separated terms with `!`-negation (`gate-env,!gate-slow`), ONE set matcher ridden by CLI flag and web filter; 7-case unit pin + e2e case. Non-death refusals pinned dead-only-BY-DESIGN (`TestRequeuesCarryNoFailureEvidence`, work `758ad3bb`). New smoke `scripts/smoke/dlq-evidence-stage.sh` (5 filter shapes through the real write path, work `4f9f03a5`). CHANGELOG operator rows verified present.
5. **filterChip doc-comment fragment cured** (work `74d34566`, window `ef2f843e`): reviewer finding on the FilterBar insertion fixed, templ regenerated, webui module green.
6. **Status-index path gate** (work `e7a9f71f`, window `df3ce1c0`): every live `docs/status/README.md` row's backticked `.md` path must resolve; negative-path self-test battery (env-hooked) per the fleet countable-gate convention; tasks/-row ruling (indexed-as-convention WINS) encoded in the gate. Raw sample-count pin `TestSecretPatternSamplesRawCountExactlyOne` (`f80a8ae5`) landed here too.
7. **Vacuous-battery cure** (`4aa9ceec` = work `83b453a2`): reviewer HIGH finding — the path battery's fixtures were unbackticked (zero paths checked) and its negative branch asserted inverted polarity. Fixed: backticked fixtures, rc=1 + pinned `PATH MISSING` line + derived `row-paths N ok / M missing` summary, non-vacuity PROVEN by two mutation probes. Verified in the gate at HEAD today (`status index ok`).
8. **Secret-evidence pinning trio** (`ce417938`; work `cad2c9a1`, `8b85a841`): golden-JSON pins for `tq audit --journal --json` secret rows; journal-drift.sh phase 6 entry-cell assertions; raw-count row closed in place.
9. **Conventions trio** (`9b90487f`; work `0f933f09`, `e0b1fc4f`, `25d3ea2d`): dry-run-first norm for doc/index mutation scripts landed in AGENTS.md §Conventions (visible in the current file); `scripts/check-prompt-const-folds.sh` guard + self-test wired into ci-local; hook case count derived instead of hand-maintained.
10. **Prospective digest row** (`1b5a7825`; work `66eadfa0`): `digest 2026-11` empty-accrual row minted in docs/status/README.md; shape pinned in the gate self-test. INDEX BLOAT wording reworded to the single ratified ritual (`ea6032a6`, render-verified). ANNOTATE pass over 32 live 09-16..09-25 reports (`f5998ae2`).
11. **Reviewer-finding fixes** (`4d2ac0f4` work `ae43dff0`: §g1 row BLOCKED-annotated; `d1ae7a60` work `c3e2d0d9`: both-rows-BLOCKED overclaim corrected inline in report §b1 + index row).

## b) PARTIALLY DONE

1. **Push-master task `…f208e533` — complete, with an attribution subtlety**: its footer commit `e65b8447` (vendorHash resync after the v0.4.0 module sweep) was PUSHED in the v0.4.1 release saga and CI ran green on the healed tree (run 41362835745, per the 17:16 dispatch's verification). The 7 commits now sitting in `origin/master..HEAD` are later 11-02 local work, not this task's remainder.
2. **ANNOTATE pass depth** (10-31 04-18 report §b): 32 ANNOTATED blocks are triage-grade; sub-agent citations not mechanically re-verified (row 1160 owns the re-check).

## c) NOT STARTED

1. **Zero-footer reviewer tickets `…ba892ab9452a36f1c2a` and `…ba892bb308b33c15`**: no commits under either footer and no closeout report of their own. The 26th dispatch verified in the production journal they were completed with attempts=0 (findings cured by sibling `83b453a2` under `…ba892a70…`) — dispositioned free, but they still consumed queue slots.
2. The §g1 (SECURITY.md generation-vs-pin ceiling) and §g2 (bare-probe Cache-Control) owner rulings from the 05-31 closeout remain open — correctly BLOCKED-annotated, agent-unlandable.

## d) TOTALLY FUCKED UP

Nothing destructive landed; all gates named below were re-verified green at HEAD where checkable. Honest residue, severity order:

1. **The BLOAT-warning echo shipped a format bug** (10-31 window §d2): literal-`\(` backslashes would have printed to operators; the self-test had no bloat leg so the gate stayed green. Caught only by a manual forced-threshold render. Cured in `ea6032a6`; the missing bloat-warning self-test branch is rowed (TODO 1159) and still OPEN.
2. **Daemon-sweep fragmentation recurred across the whole window**: multiple closeouts (06-52 §d1, 07-37 §d1-d2, 08-06 §d1-d2, 10-31 04-18 §b/§d6) lost commits to footer-less sweeps; one heal attempt orphaned `aeff1489` and briefly dropped a tick commit (recovered); multi-item sweeps still have no heal path (`--split` rowed, TODO 1223, open).
3. **A chained verification rode an unexecuted `&&`-tail** (08-21 §d1): the size-guard claim was green-on-paper while its command never ran (cmd/tq bare-`go` trap). No rc-to-file discipline that session.
4. **A fabricated-ok summary shipped in a gate** (`fails+1` class, found by the 00-15 reviewer): derived counting now in place, and a fleet-wide sweep row for the same arithmetic exists (26th-dispatch appends).
5. **AGENTS.md budget raised again** (23,900→24,400, the "lazy branch" per the session's own flag): 24,398/24,400 B today — 2 B headroom; the compress-pass row and the owner budget-policy question both still open.
6. **Pre-existing reds left standing**: `check-doc-refs` red on the missing `scripts/check-erraudit.sh` cite (rows 1083/1324/1346 own it); templ CLI version skew v0.3.1020 vs go.mod v0.3.1070 (rowed 1429).

## e) WHAT WE SHOULD IMPROVE

1. **Triple-minted reviewer-fix tickets consumed queue slots** — the queue minted three identical "fix this finding" tasks (…a70/…ab9/…bb3); only one did work, the other two completed free only because an agent noticed the sibling cure. A dedup on (finding anchor, rejected commit) at enqueue time would stop the class.
2. **Dispatch-side window dedup is still not enforced**: the same 17-task window got three full done-prompt passes in one day (12:26, 17:16, this one). The agent-side row exists; the queue could refuse a done-prompt re-dispatch while an unarchived report for the same window IDs exists.
3. **Detector gates accumulate duplicated machinery** (daemon-subject regex mirrored ≥3 scripts; ~60 lines shared between the two daemon-fold detectors) — `scripts/lib/daemon-scan.sh` extraction is the standing cure.
4. **Self-tests without negative-path legs keep shipping**: the prompt-const guard (selection-only) and journal-drift phase 6 (never seen red) repeat the vacuous-battery class the 00-15 fix just paid for. Both rowed below.
5. **Owner-gated rows keep burning dispatches** despite the advisory warn (05-31/06-16 closeouts) — the UNBLOCKED-OWNER-GATED warn list ages unactioned; per-batch BLOCKED tick-audit remains a manual virtue.

## f) NEXT THINGS (highest-value; appended to TODO_LIST where not already rowed)

Already rowed by this window's own closeouts (do not re-dispatch): prompt-const list sync (1303), Known Issues dedup-invisibility line (1235), path-gate --mine/countable/digest-exemption/self-test-paths/RULING-BY-AGENT cluster (1432-1441), bloat-warning pin (1159), heal `--split` (1223), templ skew (1429), erraudit gate rows (1083/1324/1346), November sweep + citation re-verify + annotation-date codify (1157/1160/1161), dispatch-time window dedup (26th-dispatch block), verdict-rail wording cluster (26th-dispatch block), band-drift silence owner row.

New appends (dedup-checked against 630 open rows): see the TODO_LIST section "Done-prompt twentyeighth-dispatch appends".

## g) QUESTIONS (owner only)

1. The window's §g1/§g2 owner rulings (SECURITY.md generation-vs-pin ceiling; bare-probe Cache-Control posture) plus the row-1216 indexed-as-convention ruling and the AGENTS size-budget policy remain unanswered — they are BLOCKED-rowed; a single ruling pass would unblock four rows.
2. The two zero-footer reviewer tickets completed with attempts=0 — should the queue mint such duplicates at all (enqueue-time dedup), or is free-completion the accepted cost?
3. Done-prompt re-dispatch of an already-reported window happened three times today — queue-side refusal (see §e) vs accepted redundancy: which?

## h) BAND DRIFT

**None recorded.** The production journal (`tasks.db`, read-only query 2026-11-02 17:4x) contains ZERO `task.reprioritized` facts — the fact type does not even appear among recorded fact types (`task.enqueued/claimed/completed/requeued/failed/dead-lettered/cancelled` only), across the window AND the journal's whole history. FEATURES.md ships `task.reprioritized` + `UpdatePendingPriority` as FULLY_FUNCTIONAL; the journal shows the channel has never fired. This is the same finding the 26th dispatch rowed as a BLOCKED owner question (marker/keyword/unblock bumps emit nothing) — it stands, now verified twice.
