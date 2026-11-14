# Done-prompt window report — 2026-11-14 06-36 (reviewer-fix preflight trio + calibration batch + 2026-11-09 harvest batch + window-dedup rail)

Task `000001a1d87df6898b142b994cae00000000`. Window: seven tasks —
000001a1d27555… (dispatch-time window-report dedup rail), the reviewer-fix
trio 000001a1d4459f0752 / …f1fd / …f2d0, 000001a1d441108a… (calibration
batch: tuning-pass tick + live-path smoke), 000001a1d4459f192a (lockstep-pin
reviewer fix), 000001a1d5b3d6d9 (2026-11-09 harvest batch: tag-ancestry
exclusion, check-subtag.sh, rebuild-lineage, TQ_ASK_DB). Verification method:
each window task's closeout report read FIRST (tasks/2026-11-13_02-26,
2026-11-13_17-05, 2026-11-13_17-11, 2026-11-14_06-10), then its claims
re-verified against git history, the pool journal
(`/mnt/pool/services/tq/tq.db`, read-only), and the tree at HEAD c0453833.
Skeleton committed per checkpoint-early and amended at finish.

## a) FULLY DONE

1. **Dispatch-time window-report dedup rail** (task 000001a1d27555, commits
   89150c91 + e54fe307 → landed as 583a8260): `statusPrompt` now carries a
   section ordering the reporting agent to grep the docs/status/README.md
   index for the window's task IDs/timestamps BEFORE deriving, downgrading a
   same-window hit to a verification-only pass. Pinned by
   `TestStatusPromptCarriesWindowDedup` (internal/executor/status_test.go),
   executor module gate green per closeout §a7. TODO row 1642 ticked,
   CHANGELOG Internal row landed (CHANGELOG.md:125). Verified at HEAD: rail
   text greps green in internal/executor/status.go; row 1642 reads `- [x]`.
2. **Reviewer-fix trio completed at ZERO cost via done-preflight dedup**
   (tasks 000001a1d4459f0752 / …f1fd / …f2d0): all three were completed
   attempts=0, session_cost_usd=0 — the done-preflight judged each finding
   already cured at claim time ("fix ticket cured (SHA fb5a9794 /
   56e4c606 / 7102684a …)" in the completion facts, pool journal seq
   19526–19533). All three SHAs verified reachable at HEAD: fb5a9794
   (calibration failure-path rows), 56e4c606 (29th-dispatch ticks +
   CHANGELOG sync), 7102684a (64 MiB calibration retention cap). No agent
   turns ran; no reports owed.
3. **Calibration batch** (task 000001a1d441108a, commits 95ab1ee6 + d9270e53,
   closeout 2026-11-13_17-05): item 1 (first data-driven tuning pass) was
   already landed (report 2026-11-13_10-39, indexed); only its fold-lost
   tick was re-verified and re-stamped. Item 2 landed
   `scripts/smoke/calibration.sh` — LIVE rc=0: scratch TQ_DB + TQ_LOG_DIR,
   stub agent completes one turn, exactly ONE CalibrationRow asserted on
   kind/task_id/RAW verdict/derived commit+file counts/receipt+attestation,
   failure half absent; wired into ci-local + COVERAGE.md (35 legs, 0
   unindexed). Re-verified at HEAD: smoke file present, ci-local step
   present.
4. **Lockstep-pin reviewer fix** (task 000001a1d4459f192a, commits 1fad15c5
   + 6e0e9551, closeout 2026-11-13_17-11): AGENTS.md Known Issues toolchain
   pin 1.27.3→1.27.4 matching flake `goToolchainVersion` + 8 setup-go pins
   (verified first per the reviewer finding). Size guard green 27142/27200.
5. **2026-11-09 harvest batch** (task 000001a1d5b3d6d9, closeout
   2026-11-14_06-10, commits c9fb0ae7, fbb93b6c, 22f7bb29, dd97adae et al):
   all 5 items landed and ticked — doctor tag-ancestry exclusion
   (cmd/tq/doctor_test.go, hermetic coverage; the harvest task's original
   ask "TestDoctorHealthyEmptyDB stops failing every cmd/tq module run");
   heal-daemon-sweep hardening trio (dry-run, --tip-only retitle, named-tag
   refusals); rebuild-lineage.sh verified (self-test 13 green) and executed
   LIVE twice by its own session; NEW `scripts/check-subtag.sh` clean-room
   `<module>@<tag>` verifier LIVE rc=0 on root@v0.4.2 AND cmd/tq@v0.4.3;
   the `tq ask` "task: not found" root cause (M24 denylist strips TQ_DB)
   FIXED via the pool-injected `TQ_ASK_DB` ask-only channel, pinned by
   `TestAskDBChannelInjected` + `TestAskDBPathPrecedence`. Three footer-less
   daemon sweeps healed live; attribution 9/9 OK.
6. **This window's own reporting**: the abandoned same-window skeleton
   2026-11-13_10-33 (checkpointed by a session that died before filling it)
   annotated DUPLICATE and archived with its index row repointed — the
   docs-health pass's one archive this sweep.

## b) PARTIALLY DONE

1. **TQ_ASK_DB fix is landed but never production-observed**: no live
   dispatched run has asked a real question through the new channel yet
   (harvest closeout §b1). Pinned by tests only; the next natural owner
   question in a pool run is the receipt (TODO row minted).
2. **check-subtag.sh is standalone**: self-test green but wired into neither
   release.sh --push nor ci-local preflight (rows 1898/1901 own both
   halves).
3. **Window-dedup rail is prompt-level only and never live-fired**: the
   sweeper still mints mechanically; the verification-only downgrade has
   zero live runs (2026-11-13_02-26 closeout §b1/§b2).
4. **Verification scope of the harvest batch was scoped, not full-battery**:
   ci-local full run, root `-race`, lint-baseline were skipped (closeout
   scope line) — row 1904 carries the gap.

## c) NOT STARTED

1. **Owner-gated rulings parked across the window** (untouched, correctly
   out of scope): cmd/tq/v0.4.2 retract-vs-supersession (BLOCKED row 54
   lineage heal), postgres-pool ask routing, non-github discovery escape
   for check-subtag, the November index-digest scheduling (this window
   minted the blocked row).
2. **Harvest batch items 6–7** (owner calls) — untouched per closeout §c1.
3. The `go: open dev.mod` noise in test-cmd-tq output — observed twice,
   never investigated (row 1900).

## d) TOTALLY FUCKED UP

1. **A same-window session DIED leaving a committed skeleton forever
   unfilled** (2026-11-13_10-33): checkpoint-early worked exactly as
   designed — the artifact was recoverable — but no recovery ever happened;
   the placeholder sat indexed for a day until this pass archived it. The
   dead session also left its window unreported until this report.
2. **Daemon-sweep races remained the window's constant tax**: three
   footer-less sweeps in the harvest session alone (healed live, fork
   records in the 06-10 closeout §a.6), and THIS session lost its skeleton
   commit to one (cbc6ed44, healed via fold-marker → 830d98bc). The
   edit→commit→battery convention keeps being violated under time pressure.
3. **A fold dropped a tick**: the calibration tuning-pass row 1672's tick
   did not survive a concurrent session's fold (DONE-ANNOTATION-UNCHECKED),
   costing a re-verification round-trip (17-05 closeout §a1).
4. **AGENTS.md size-guard death march** (~10 gate reruns, ~15 edit rounds,
   one WRONG mid-flight edit reverted) — harvest closeout §d1; the
   budget-first discipline is minted as row 1906's delta-print ask.
5. **Observe-first inversion in check-subtag's first design** (fixture
   before real path; a silent no-op `go build ./...` proof nearly shipped)
   — harvest closeout §d2.

## e) WHAT WE SHOULD IMPROVE

1. **Checkpoint-early needs a resume convention**: the skeleton is the
   recoverable progress, but nothing routes a dead session's skeleton to a
   successor. Proposal: the done-preflight should treat an unfilled
   same-window skeleton report as a window-dedup hit (it already names the
   window), and the recovering pass must annotate + archive it rather than
   mint a twin. (Done here manually; worth a prompt line.)
2. **Done-preflight zero-cost cures are the channel's best ROI** — three
   paid-looking tickets closed for $0 this window (§a2). Row 1887's ask
   (document the channel in AGENTS §Payload contracts) is still open; it
   would stop future windows from auditing "ghost" tasks.
3. **Budget-first AGENTS.md editing** (harvest §e1): measure the byte delta
   before writing, prune in the same edit.
4. **Real-path-before-stub for network-shaped tools** (harvest §e2): run
   the smallest real invocation, then build the hermetic CI fixture.
5. **Fold-marker fold discipline**: this session's sweep hit was closed in
   one command (`scripts/fold-marker.sh`) because the sweep was tip-only
   and exactly-mine — the fast path exists; prefer it over heal/rebuild
   when the shape allows.

## f) NEXT THINGS (deduped against the standing TODO_LIST; the harvest batch's
50-item list was already harvested as rows 1898–1906+ by its own closeout)

1. Record the first live verification-only done-prompt pass receipt for the
   window-dedup rail (zero live runs since 583a8260).
2. Confirm TestDoctorHealthyEmptyDB green in-repo at HEAD post-exclusion and
   annotate row 54's stale "fails in-repo" symptom claim.
3. Postgres-pool ask routing ruling (owner BLOCKED).
4. November index-digest ritual scheduling (owner BLOCKED; WARN fired all
   window, 407→408 live rows during this very report).

## g) QUESTIONS (owner-only)

1. **Postgres-pool dispatched asks**: teach `tq ask`'s fallback to open
   postgres via TQ_ASK_DB DSN, or pin ask sqlite-pool-only? Today the fix
   silently no-ops for postgres (`askChannelDB` returns ""). Harvest
   closeout §g1 — minted as a BLOCKED row.
2. **check-subtag for non-github private modules**: go's `?go-get=1`
   discovery query hits the real host before insteadOf — github-only
   permanent scope, or add a GOVCS/--discovery-off escape? (Standing row
   1902.)
3. **Digest scheduling**: should the next dispatch include the November
   digest sweep as a batch item, or do you want to schedule it manually?
   (Harvest closeout §g3; minted as a BLOCKED row.)

## h) BAND DRIFT

**None recorded.** Read-only probe of the production journal
(`/mnt/pool/services/tq/tq.db`, facts table): zero `task.reprioritized`
facts exist — not in this window's timespan, not ever (the fact-type census
carries no such type; consistent with the standing none-census streak
recorded by every prior pass since the twenty-sixth dispatch). Priority
movement is therefore explainable trivially: nothing moved.
