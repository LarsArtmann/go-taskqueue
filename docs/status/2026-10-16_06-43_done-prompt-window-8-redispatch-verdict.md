# Done-prompt window 8 — re-dispatch verdict report (2026-10-16 06-43)

Task `000001a13db764ad71e3417244d800000000`, **attempt 5**. The window (the
same eight harvest tasks as the 06-15 report) was already fully reported by
**attempt 1 of this very task**:
`docs/status/2026-10-15_06-15_done-prompt-window-8-task-harvest.md`
(committed `ca7dd324`, 2026-10-15 06:32) — but its verdict never landed:
the verify gate requeued it (`class=gate`, 06:35), the daily budget gate
blocked the retry (06:51), an env-streak burned attempt 2 on a foreign
dirty tree (2026-10-16 01:13), and two rate-limit requeues followed
(01:34, 01:49). This report therefore (1) re-verifies attempt 1's claims
at current HEAD `2a045f19`, (2) corrects two of them, (3) covers the
delta since 10-15 06:32, and (4) records the verdict. Verified: tree
clean, master == origin/master, journal `/mnt/pool/services/tq/tq.db`
(12,628 facts).

## a) FULLY DONE (re-verified at HEAD, not just re-read)

1. **secretPatterns pin harvest (…011, closeout 03-39)** — re-dispatch
   found all three pins already at HEAD; verified + ticked. Re-verified
   today: `TestSecretPatternsOverlapCensus` (pair pin `["8×9"]`),
   `TestSecretHitsCountsInjectedTokensIndependentOfContext`,
   `TestRedactSecretsMasksAuthHeaderToExactlyOneMarker` all present in
   `internal/executor/redact_test.go`.
2. **secretPatterns-pin consolidation (…cad, closeout 03-55)** — ONE
   ordered `secretPatternSamples` fixture now feeds census + identical
   table; `fakeJWT` rides the property; byte-side one-marker pin added.
   Commits `3e10f7f4`/`ffb18534`/`5cb9bf26` re-verified in history.
3. **Post-redaction silence pin (…3dc4, item 1)** —
   `TestPostRedactionSilence` (`redact_test.go:289`) asserts
   `SecretHits(RedactSecrets(s)) == 0` for every canonical sample; row
   224 ticked in work commit `2db9dde7`. Items 2–3: see b1 (the
   window-8 report's account needed a correction).
4. **Audit-honesty batch (…2be, closeout 04-30)** — journal-drift +
   redaction smokes re-PASS at HEAD; pre-`4ba81d8` comparability notes
   verified at `README.md:325` + `AGENTS.md:148`; `hits=`→`locations=`
   rename live (`cmd/tq/journalaudit.go`); the three 2026-09-16 archived
   reports carry their inline ANNOTATION blocks (`a47a4612`).
5. **Harvest-residue batch (…12b, closeout 05-16)** — status-index row
   for 04-30 present; AGENTS.md pruned to 19896 B (204 B headroom,
   `TestAgentsDocSizeGuard` budget intact today);
   `journal-drift.sh` phase 5 seeds `sk-ant-SMOKETEST-…` and pins the
   `locations=1` SECRET EVIDENCE row end-to-end (`2b4f352c`).
6. **CI-resilience batch (…f7c, closeout 00-44)** — frozen-date sweep +
   `scripts/check-frozen-dates.sh` (2 ci-local call sites, verified);
   GNU-date guard in ratelimit-e2e (`caffd9ad`); ratelimit-e2e wired
   into the ubuntu CI job (`.github/workflows/ci.yml`, `a2c99744`).
   Known cost carried: daemon PUSHED `c7e95833` mid-fold (item 1's
   substance permanently footer-less).
7. **Provider-tag batch (…72ba, closeout 01-15)** — the e2e smoke greps
   BOTH observability halves (requeue line + `provider=zai`);
   `RateLimitError.Provider` rides `RequeueEvidence` through
   store/adapters; `tq tasks --parked` rows prefix `[zai] …`; doctor
   row closed verify-only (`TestDoctorParkedNamesEarliestRelease`).
   Executor + root modules verified green today (executor gate 10.7 s).
8. **Usage-rendering placement (…5f90, closeout 05-57)** —
   `FEATURES.md:134` outcome-card row carries the
   `statusReportCard`/`prioritizeResultCard` rendering facts
   first-class; TODO row ticked (`733aee68`).

Window totals (attempt 1's count, re-checked): **20 TODO rows ticked**,
**~22 rows minted**, **17 footed commits**, 8/8 tasks COMPLETED in the
journal. The CHANGELOG `[Unreleased]` entries the window owed (provider
tag, locations rename) are present and current.

## b) PARTIALLY DONE

1. **Task …3dc4 was 2/3-landed at reclaim, not 1/3 — correcting the
   window-8 report's §a3/§b1/§c1 (annotated there).** Item 2 of the
   batch (row 225, the structural redaction guard) DID land:
   `internal/executor/redaction_guard_test.go` (100 lines, exactly row
   225's wording) appeared in footer-less daemon sweep `9b4d2bc9` at
   10-14 **04:14** — inside the …3dc4 lease (04:07–04:17), two minutes
   after the item-1 work commit — then was extended by `bd151455` and
   word-boundary-fixed by `2d6b3993` (10-16 02:45). The batch's agent
   evidently wrote it and lost it to the daemon before any closeout.
   Item 3 (row 226, `SetFailureEvidence` for depbump) is genuinely
   unlanded — verified today: `internal/executor/depbump.go` has no
   `SetFailureEvidence` call (the type exists for agent/command
   executors only). **Row 225 ticked this pass** with the evidence;
   row 226 stays open.
2. **This task's own verdict-loss chain (5 claims → 1 verdict)** — see
   the header. Attempt 1's complete, committed report produced no queue
   effect for 23 h; every retry re-stacked a different blocker
   (dead gate → budget → env-streak → rate limits). The re-dispatch
   burned real budget re-researching work whose report was already on
   master (I found it and built on it; attempts 2–4 died before
   research).
3. **Provider contract is sqlite-proven only** (row 839) — postgresv4
   conform pin took the `TQ_TEST_POSTGRES` skip path; unchanged.
4. **Attribution gaps carried**: pushed footer-less `c7e95833`
   (permanent), mixed footer-less `7caec61f` (unhealable by policy).
5. **AGENTS.md rate-limits bullet still says provider is log-only**
   (row 835; needs the paired byte-shave; headroom 204 B).
6. **In passing (NOT this window)**: the concurrent dep-sweep e2e task
   (…a4bb, completed 06:25:20 today, one minute before I claimed)
   landed `scripts/smoke/dep-sweep.sh` + ci-local wiring + a
   `DedupKey` generation suffix + a depbump git-add fix — but row 281
   is still unticked and no closeout report exists yet. Same
   landed-but-unticked shape as b1; left for its own session (it may
   still be finalizing).

## c) NOT STARTED

1. **Row 226** (`SetFailureEvidence` for depbump) — the …3dc4 batch's
   genuinely-skipped item; top of the redaction/evidence queue.
2. **Row 239** (`tq stats --project` parked-count × filter pin) — last
   open row of the 09-17 CI-resilience cluster.
3. **Full ci-local battery at HEAD** (rows 197/210) — deferred by every
   session; now doubly relevant because the root gate is RED (d1).
4. **Dead-SHA heal** (rows 30/852) — the dangling-cite count GREW
   108 (10-12) → **385** (today). Re-verified red this session.
5. **Index-bloat archive sweep** — 308 live rows vs threshold 100;
   no report sampled this pass was fully resolved, so nothing was
   archived; the corpus sweep remains row-owned.
6. **worktree.sh fix-then-wire** (row 831) — guard-wiring re-verified:
   58 wired, 1 orphaned.

## d) TOTALLY FUCKED UP

1. **The root verify gate is RED at HEAD — and has been for ~25 h, and
   no row tracked it until now.** Footer-less daemon sweep `5b30779a`
   (10-15 **05:54**, 4 files) bumped `go-sse/ssetest` v0.4.0 → v0.5.0
   in the ROOT go.mod mid-window. ssetest v0.5.0 changed
   `WithLastEventID` to take the branded `sse.EventID`;
   `internal/webui/webui_test.go:453` still passes a raw string →
   `go vet ./...` and `go test ./...` fail to compile the webui test
   binary. Verified this session: red at HEAD (vendor AND `-mod=mod`,
   i.e. the pinned graph itself), green at v0.3.4 (`eb81ddd1`), pin
   diff isolates the ssetest bump. This dead gate is exactly what
   requeued this task's attempt 1 (`class=gate`, 06:35 — the requeue
   even said "failure is pre-existing/environmental (gate also fails at
   the pre-attempt rev)"). Every root-gate dispatch since 05:54 has
   paid for one agentless dep bump. **Fix minted as a TODO row** (code
   change — out of this docs pass's scope). Root build is green; the
   failure is test-only; the executor and other module gates are green.
2. **Batch-reclaim laundering, sharpened (b1)**: the …3dc4 reclaim
   completed a batch whose item 2 had landed agentlessly (daemon-swept
   mid-lease) and item 3 not at all — no closeout, zero-cost, 84 ms.
   The window-8 §d1 finding stands; the correction makes the reclaim
   question subtler (partial work was REAL but invisible to the
   queue).
3. **Env-streak burned attempt 2 of THIS task on foreign dirt**
   (2026-10-16 01:13): three preflight requeues for
   `M internal/prioritize/sweep.go` — the concurrent dep-sweep task's
   in-flight file — then the breaker spent the attempt. Second sighting
   of the class in two windows; the investigation row (window-8 mint)
   now has a same-night second victim.
4. **The redaction guard shipped with a latent name-matching bug**: the
   daemon-swept guard used substring matching; when the concurrent task
   added `SetResultDetail`/`Detail` accessors (10-16 02:34) "Detail"
   matched "tail" and the executor suite went RED; fixed in 11 minutes
   (`2d6b3993`, word-boundary camelCase). Fast fix, but the guard sat
   in the tree for two days unexercised against name collisions because
   it landed without its author running the suite.
5. **Standing reds carried unchanged**: dead-SHA 385 dangling cites,
   guard-wiring orphan, index bloat 308 vs 100, f26 TRAILER WARNINGs,
   AGENTS.md at 204 B headroom with the trap re-armed.

## e) WHAT WE SHOULD IMPROVE

1. **Daemon sweeps must never carry dependency-graph edits.** A
   footer-less `chore:` sweep moved go.mod/go.sum (ssetest v0.5.0) and
   broke the root gate for every task behind it (d1). Dep bumps are
   dispatch-worthy work; agentless sweeps should stage-only exclude
   them (or the gate should refuse unfooted go.mod changes).
2. **Re-dispatched done-prompts need a resume preflight**: grep for a
   committed predecessor report for the OWN task id before re-deriving.
   This attempt cost a fraction of attempt 1's research because the
   report was on master; a preflight makes that the default, not luck.
   (Verdict-recording policy is a owner call — g1.)
3. **Gate-dead requeues should auto-mint a fix-first task.** The
   `class=gate` requeue named the failing gate verbatim in the journal;
   translating that into a TODO row took until this report. The webui
   breakage sat unowned for 25 h because no mechanism mints the row.
4. **Landed-but-unticked rows keep accumulating** (row 225 sat two
   days; row 281 now): every daemon-swept landing manufactures one.
   A cheap sweep — grep TODO rows' named tests/files at HEAD — would
   retire them in bulk (window-8 §f22's idea, still open).
5. **Fork-record automation for the heal loop**: dangling cites grew
   108 → 385 in four days; `heal-daemon-sweep.sh` already computes
   old→new pairs — have it append them to a tracked records file the
   dead-SHA gate consults, or the gate reds forever.
6. **Same-breath committing** (edit → footed commit in one invocation,
   before gates) remains the discipline every §d in this window's
   closeouts traces back to; two windows of receipts now.

## f) NEXT THINGS (strongest first; deduped against open rows — 3 minted)

NEW this pass (minted to TODO_LIST):

1. Fix the webui ssetest-v0.5.0 migration (`WithLastEventID` branded
   `sse.EventID`) — un-reds the root verify gate (d1).
2. Done-prompt resume preflight: detect a committed predecessor report
   for the own task id before re-deriving (b2/e2).
3. Owner ruling: verdict-from-predecessor-report policy for
   re-dispatched done-prompts (g1 — BLOCKED row).

Strongest existing rows this pass re-proved (no duplicates minted):

4. Row 226 — `SetFailureEvidence` for depbump (verified unlanded).
5. Row 239 — `tq stats --project` pin.
6. Rows 30/852 — dead-SHA heal, now 385 dangling cites.
7. Row 831 — worktree.sh fix-then-wire (orphan re-verified).
8. Rows 197/210 — full ci-local battery at HEAD (blocked-by d1 until
   the webui fix lands).
9. Row 835 — AGENTS rate-limits bullet + paired byte-shave.
10. Rows 836–839 — parked-provider family (JSON surface, doctor
    naming, LastFacts dedup, postgres conform proof).
11. Row 840 — FEATURES sweeper-row pointer swap (completes the 05-57
    MOVE ruling).
12. The reclaim-mechanics investigation + its BLOCKED owner ruling
    (window-8 mints) — b1 sharpens both.
13. The env-streak foreign-dirt investigation row — second sighting
    recorded in this report (b3/d3).
14. Row 281 (dep-sweep e2e) — foreign task's residue: work landed,
    row unticked, no closeout yet (b6); leave to its session, verify
    next pass.

Smaller, concrete, report-only:

15. Pin the redaction guard's camelCase word-boundary matcher with a
    case-collision fixture (`SetResultDetail`-shaped names) so the
    2d6b3993 fix cannot regress.
16. Auto-mint gate-fix tasks from `class=gate` requeue payloads (e3).
17. Bulk landed-but-unticked sweep over TODO rows whose named symbols
    exist at HEAD (e4).
18. Heal-script fork-record automation feeding the dead-SHA gate (e5).
19. `tq audit --journal --since <ts>` (standing idea): retires the
    pre/post-`4ba81d8` comparability caveat class.
20. Annotation sweep of the window-7 report (2026-10-14_03-46) — zero
    `~~` markers, 10 numbered items, archive-blocked until annotated.

## g) QUESTIONS ONLY THE OWNER CAN ANSWER

1. **Verdict-resume policy (NEW)**: when a re-dispatched done-prompt
   finds its OWN task id's predecessor report committed and verified
   current, may it record the verdict from that report (after
   re-verification), or is full re-derivation the sanctioned bar?
   This task paid five claims for one verdict; the resume path is the
   difference between a 25 h stall and a 15 min re-close.
2. **Standing (already minted as BLOCKED rows — do not re-answer
   here)**: batch reclaim-completion policy; daemon push grace window;
   AGENTS.md headroom policy. All three remain unanswered after two
   windows of asks (window-8 §g1–g3; rows cited there).

## h) Band drift

**None recorded.** The production journal
(`/mnt/pool/services/tq/tq.db`, 12,628 facts) holds **zero**
`task.reprioritized` facts — the type has never occurred. Re-verified
directly this session via `tq facts -type task.reprioritized` (and a
full-fact grep: the only four substring hits are agent progress
narration that leaked into `task.requeued` error tails on failed agent
runs — observability noise worth a passing note, not priority moves).
All eight window tasks ran exactly as dispatched; no ADR-0015 moves to
account for.

## Provenance

- Predecessor report (attempt 1): `docs/status/2026-10-15_06-15_done-prompt-window-8-task-harvest.md` — §a3/§b1/§c1 annotated inline this pass (the "items 2–3 never ran" claim corrected; item 2's guard landed via daemon sweep `9b4d2bc9`).
- Verification set: executor module gate green (build+vet+test, 10.7 s); root build green / root vet+test RED (d1, isolated to ssetest v0.5.0 via v0.3.4 worktree diff); check-status-index ok (308-row bloat warning standing); check-dead-sha-refs RED (385); check-guard-wiring 58/1 orphan; AGENTS.md 19896 B.
- No code, script, or config files touched by this pass: report + TODO_LIST (1 tick, 2 mints, 1 BLOCKED row) + inline annotations to the predecessor report only.
- This report's own landing: the daemon swept the two report files into footer-less `ad95e368` seconds before the footed commit; healed via `scripts/heal-daemon-sweep.sh --from 2a045f19`. Fork-records: `ad95e368→a7f60f6c` (report + predecessor annotations, footer stamped), `73fd8e6b→3d2aa51e` (TODO ticks/mints + index row).
