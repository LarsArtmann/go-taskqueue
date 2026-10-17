# Done-prompt window 8 — eight-task harvest status report (2026-10-15 06-15)

Window: 2026-10-14 03:37 → 2026-10-15 05:59 CEST (first task enqueue →
last task completion; journal `/mnt/pool/services/tq/tq.db`, facts
11913–12436). Dispatcher: done-prompt loop. Sources (all read first, as
prompted): the eight per-task closeout reports under `docs/status/tasks/`
2026-10-14_03-39 … 2026-10-15_05-57. Verified at HEAD `9b51694e` (tree
clean, 3 commits ahead of origin at report time). Task
`000001a13db764ad71e3417244d800000000` (this report).

## a) FULLY DONE (every claim re-verified against HEAD, not just the closeouts)

1. **secretPatterns pin re-dispatch harvest (task …011, closeout 03-39).**
   All three pin tests were already at HEAD (landed by the 02-37 span-merge
   work without the batch footer); the session verified each and ticked its
   row. Verified present today: `TestSecretPatternsOverlapCensus` (all-11
   overlap census, `want = ["8×9"]` pair pin),
   `TestSecretHitsCountsInjectedTokensIndependentOfContext` (N×filler
   property), `TestRedactSecretsMasksAuthHeaderToExactlyOneMarker`.
   Footer commits `b829f600`/`20cc3fbc`/`c08af02a` + closeout `a911cd94`.
2. **secretPatterns-pin consolidation (task …cad, closeout 03-55).** Real
   test edits, 3/3: ONE ordered `secretPatternSamples` fixture
   (`internal/executor/redact_test.go:100`) now feeds BOTH the census and
   the identical-table test (a new provider pattern edit touches one
   table); `fakeJWT` rides the context-independence property;
   `TestRedactBytesMasksAuthHeaderToExactlyOneMarker` pins the byte-side
   path. Commits `3e10f7f4`/`ffb18534`/`5cb9bf26` + ticks `643152e2` +
   closeout `7adf8fea`.
3. **Post-redaction silence pin (task …3dc4 — item 1 of 3).**
   `TestPostRedactionSilence` (`redact_test.go:289`) asserts
   `SecretHits(RedactSecrets(s)) == 0` for every canonical sample,
   derived from the shared fixture; row 224 ticked inside work commit
   `2db9dde7`. Items 2-3 of this batch never ran — see b1, the window's
   biggest honesty finding.
   > [2026-10-16 ANNOTATION, docs-health — corrected by the re-dispatch
   > verdict report
   > (docs/status/2026-10-16_06-43_done-prompt-window-8-redispatch-verdict.md):
   > item 2 DID land — `internal/executor/redaction_guard_test.go`
   > (row 225's structural guard) arrived via footer-less daemon sweep
   > `9b4d2bc9` at 10-14 04:14, inside the …3dc4 lease, and is green at
   > HEAD after the word-boundary fix `2d6b3993`. Item 3 (row 226) is
   > genuinely unlanded. "Never ran" was wrong for item 2; the row was
   > ticked with this evidence on 2026-10-16.]
4. **Audit-honesty batch (task …2be, closeout 04-30).** 3/3:
   `journal-drift.sh` (4 phases) + `redaction.sh` re-PASS at HEAD;
   the pre-`4ba81d8` comparability operator note landed
   (`README.md:325`, `AGENTS.md:148`, both verified); the audit row label
   renamed `hits=` → `locations=` (`cmd/tq/journalaudit.go:627`, verified)
   with the CHANGELOG `[Unreleased]` Changed entry + JSON-shape-stable
   note; the three pre-fix 2026-09-16 archived reports carry inline
   `[2026-10-14 ANNOTATION, docs-health]` blocks (`a47a4612`). Commits
   `ee59222d`/`e91b1183`/`a47a4612` + closeout `f41fc094`.
5. **Harvest-residue batch (task …12b, closeout 05-16).** 3/3: the 04-30
   status-index row verified present (`check-status-index.sh` = `status
   index ok`, re-run green today); AGENTS.md pruned 20100 → 19896 B
   fact-preserving (`TestAgentsDocSizeGuard` PASS; still 19896 B today =
   204 B headroom); `journal-drift.sh` phase 5 seeds the
   `sk-ant-SMOKETEST-…` shape into a `task.failed` fact and pins the
   `locations=1` SECRET EVIDENCE row end-to-end
   (`scripts/smoke/journal-drift.sh:73`). Commits
   `6cb13616`/`9e6f4214`/`2b4f352c` + closeout `b40e5c88`.
6. **CI-resilience batch (task …f7c, closeout 00-44).** 3/3: frozen-date
   sweep converted the two real frozen fixtures (papdashboard `failedAt`,
   status-loop report filename) to run-time computation, ratified the
   verbatim incident capture as an allowlist entry, and shipped
   `scripts/check-frozen-dates.sh` (+self-test) wired into ci-local (2
   call sites, verified); the GNU-date guard in ratelimit-e2e fails loud
   (`caffd9ad`); ratelimit-e2e now runs on the ubuntu CI job
   (`.github/workflows/ci.yml:187`, `a2c99744`). Commits
   `72bdd058`/`caffd9ad`/`a2c99744`/`f34d5d36` + closeout `ee17ef83`.
   Known, disclosed cost: the daemon PUSHED footer-less `c7e95833`
   mid-fold, so item 1's substance is permanently unattributed (b3).
7. **Provider-tag batch (task …72ba, closeout 01-15).** 3/3: the
   ratelimit-e2e smoke greps the worker log for BOTH halves of the f35
   contract — the requeue line and `provider=zai`
   (`scripts/smoke/ratelimit-e2e.sh:113-119`, verified; smoke ran green
   twice); `RateLimitError.Provider` now rides requeue evidence through
   `Store.Requeue` → companion/sqlitev4/postgresv4, and `tq tasks
   --parked` rows prefix `[zai] …` (`latestRequeueProvider`,
   `cmd/tq/parked.go:48`; `TestRequeueFactCarriesProvider` +
   `TestParkedProviderJoinsOntoRows`); the doctor earliest-release row
   closed verify-only (already pinned by
   `TestDoctorParkedNamesEarliestRelease`). Commits
   `c5ac3d4d`/`9e23e1c5`/`8e976216` + closeout `86ca0004`.
8. **Usage-rendering placement (task …5f90, closeout 05-57).** The Web UI
   outcome-card row (`FEATURES.md:134`) now carries the
   `statusReportCard`/`prioritizeResultCard` rendering facts first-class
   (verified against `internal/webui/fragments.templ:748,773` by the
   closeout; row text verified today); TODO row ticked (`733aee68` +
   closeout `9afd7e22`).

Window totals: **20 TODO rows ticked** (3+3+1+3+3+3+3+1), **~22 rows
minted** (4+3+6+2+6+1 as reported per closeout), **17 footed commits**,
8/8 tasks COMPLETED in the journal. In passing, the ci.yml parity-sweep
task (…cde8, closeout 2026-10-15_06-10) completed at 06:15:04 — one
minute before this report — wiring the remaining ci-local-only gates into
ci.yml (`7e45d620`).

## b) PARTIALLY DONE

1. **Task …3dc4 completed 1/3 — and the queue still says COMPLETED.** The
   journal shows: claimed 04:07:25 (worker-42680), work commit `2db9dde7`
   at 04:12, **released on lease-expiry at 04:17:27.413**, re-claimed by
   worker-33599 at `.497`, and `task.completed` **84 ms later** with
   `session_cost_usd: 0, session_prompt_tokens: 0` — a derived zero-cost
   completion. Items 2-3 (TODO rows 225 structural redaction guard, 226
   depbump `SetFailureEvidence`) were never attempted, verified still
   unchecked at HEAD, and **no closeout report exists** for this task.
   Only the TODO rows keep the skipped work visible. See g1.
   > [2026-10-16 ANNOTATION, docs-health — see the correction at §a3:
   > item 2/row 225 landed via daemon sweep `9b4d2bc9` mid-lease (the
   > agent lost it to the daemon before any closeout); item 3/row 226
   > remains genuinely unlanded. The reclaim-laundering finding stands,
   > sharpened: the batch was 2/3-landed at reclaim, invisibly.]
2. **Provider contract is sqlite-proven only** (…72ba §b1): the postgresv4
   conform pin took the `TQ_TEST_POSTGRES` skip path ("ok" in 0.010s).
   Filed as row 839.
3. **Attribution gaps, two shapes**: (i) pushed footer-less `c7e95833`
   carries the f7c frozen-date conversions forever (heal refused:
   pushed); (ii) mixed footer-less `7caec61f` carries the 5f90 FEATURES
   edit + a concurrent agent's queue/worker code (heal refused:
   not-exactly-mine). Both disclosed by their closeouts; both mean the
   queue's file-derivation under-attributes those tasks.
4. **Parked provider surface is text-table-only**: `tq tasks --parked
   --json` rows do not carry the provider (row 836; contract decision
   pending, g-adjacent).
5. **Docs residue from the window's own feature work**: the
   operator-visible `[provider]` parked-surface change had NO CHANGELOG
   entry until this pass — row 834 existed because the 01-15 closeout
   filed it instead of writing it. **Fixed in this docs pass** (entry
   added under `[Unreleased]`/Added; row 834 ticked). The AGENTS.md
   rate-limits bullet still describes the provider as log-only (row 835,
   must pair a byte-shave; headroom is 204 B).
6. **One paid attempt burned to foreign dirt**: task …5f90 was
   preflight-requeued three times ("repo has uncommitted changes",
   facts 12427–12435) and the env-streak breaker burned attempt 1 at
   05:34:02 — the dirty tree belonged to concurrent sessions, not this
   task. Work completed on attempt 2 at 05:59.

## c) NOT STARTED

1. **Rows 225/226** — the …3dc4 batch's own skipped items (structural
   redaction guard for tail helpers; depbump `SetFailureEvidence`). The
   highest-priority residue of this window.
   > [2026-10-16 ANNOTATION, docs-health — row 225 landed (daemon sweep
   > `9b4d2bc9` + fix `2d6b3993`) and was ticked 2026-10-16; row 226
   > verified still open.]
2. **Row 239** (`tq stats --project X` parked-count × filter pin) — the
   only other open row of the 09-17 CI-resilience cluster this window
   worked from.
3. **Full ci-local battery at HEAD** — deferred by every session that
   touched the window (03-55 §b1, 05-16 §b1, 01-15 §b3); rows 197/210
   own the class.
4. **Index-bloat archive sweep** — `check-status-index.sh` reports 307
   live rows vs threshold 100 (grew 304 → 307 during the window); the
   open row re-justifies it every session instead of working it.
5. **worktree.sh fix-then-wire** (row 831) — `check-guard-wiring.sh`
   re-verified today: 57 wired, **1 orphaned** (scripts/smoke/worktree.sh).
6. The other ~460 open rows — untouched, as scoped; the window minted
   more rows than it retired (20 ticks vs ~22 mints plus this pass's 4).

## d) TOTALLY FUCKED UP

1. **Derived completion of a partially-landed batch** (b1) — the worst
   finding. The journal's COMPLETED status for …3dc4 is a lie about the
   batch; nothing in the queue surface (task status, derived outcomes)
   reflects the 2 skipped items. The session-close bridge also never ran
   for the dead session (no review/status tasks minted from it).
2. **Daemon races in 5 of 8 sessions**, one permanent: 04-30 (2 sweeps,
   healed), 05-16 (1 sweep healed + a footer-above-attribution first
   commit), 00-44 (**sweep PUSHED mid-fold — permanent**), 01-15 (4
   sweeps, one MIXING two items' files), 05-57 (mixed sweep, unhealable).
   Root cause identical every time: edit → gate → commit instead of
   edit → commit → battery. The discipline rule exists; the two most
   recent windows repeated it anyway.
3. **Heal-chain hash rot reaches dispatch payloads**: the dispatcher's
   own commit cites for 4 of 8 window tasks (`72da760e`, `35cd8b53`,
   `50131a15`, `748149ae`) are now **unreachable** (objects exist, not
   from HEAD) — later in-policy heals rewrote them. Worse, the brand-new
   06-10 closeout report cites `3f8ed458` and
   `check-dead-sha-refs.sh` is **RED at HEAD** for it at this report's
   write time (docs/status/tasks/2026-10-15_06-10_…md:28). Fresh closeouts
   are minting fresh dangling cites within hours.
4. **cmd/tq vendor clobbered by a naive `go mod vendor`** (01-15 §d2) —
   healed via the devmod regen, but the AGENTS.md note the agent needed
   ("regen via `-modfile=dev.mod`") still doesn't exist (headroom-blocked;
   rides row 835's paired shave).
5. **Standing reds/noise carried through the window unchanged**:
   guard-wiring orphan (row 831), AGENTS.md at 204 B headroom with the
   trap re-armed (row 830, BLOCKED), index bloat 307 vs 100 (open row),
   f26 TRAILER WARNINGs (owner ruling pending, printed on every index
   check).
6. **Env-streak breaker burned a paid attempt on foreign dirt** (b6) —
   the breaker cannot distinguish "my tree is dirty" from "another
   agent's in-flight files are dirty"; three preflight requeues later the
   task paid an attempt for someone else's race.

## e) WHAT WE SHOULD IMPROVE

1. **Reclaim must re-derive batch completeness.** On lease-expiry
   reclaim, a worker that finds _some_ landed work for a multi-item
   batch should verify the remaining item keys (footed commits / landed
   tests) before derived completion — or re-enqueue the unlanded keys.
   Today a fast reclaim launders a 1/3 batch into COMPLETED (d1).
   Mechanics row minted; policy ruling in g1.
2. **Same-breath committing is still not muscle memory.** Two
   consecutive windows, same §d root cause. The working rule from the
   00-44/01-15 reports: each item's edit + footed commit in ONE bash
   invocation, before any gate run. Row 32 (echo the commit hash) and
   row 827 (footer-position assert) remove two of the remaining
   frictions; both are cheap.
3. **Cite task ids, not hashes, wherever heals can reach.** Dispatch
   payloads, closeout bodies, and index rows that cite commit hashes rot
   on the first heal; the footer grep (`git log --grep
   Task-Queue-ID:<id>`) is heal-stable. Where a hash is unavoidable, a
   fork record (old→new) keeps it resolvable — the dead-SHA gate already
   enforces exactly this for docs; dispatch payloads aren't scanned.
4. **Operator-visible changes should carry their CHANGELOG hunk in the
   landing commit** (O14). The provider tag needed a next-day docs pass
   to get its entry; the closeout that filed row 834 could have written
   the entry in `9e23e1c5` at zero marginal cost.
5. **Closeouts should re-verify hash reachability at cite time** (or
   cite the task id): the 04-30 closeout cites post-heal hashes
   (`5d46b448`, `fdaca3b4`) that were themselves rewritten hours later
   by the next batch's heal — now-unreachable cites inside a
   point-in-time document, guaranteeing future dead-SHA findings.
6. **Skip-path "ok" is not coverage** (01-15 §e5): any store-contract
   claim should name WHICH engine executed it; row 839 exists for the
   provider pin and the convention deserves an AGENTS.md line when
   headroom allows.
7. **Targeted-vs-battery verification needs a ruling** (05-16 §g3): five
   windows in a row ran targeted gates and disclosed the residue; a
   sanctioned docs-only battery definition would stop the per-window
   improvisation.

## f) UP TO 50 NEXT THINGS (deduped against open rows; mints marked NEW)

NEW this pass (minted to TODO_LIST, see the window-8 section):

1. NEW — Heal the dead-SHA red + fork-record the window's five
   unreachable hashes (`3f8ed458` fresh red at HEAD; `72da760e`,
   `35cd8b53`, `50131a15`, `748149ae` pre-heal dispatch cites).
2. NEW — Investigate + pin the reclaim path for multi-item batches
   (the …3dc4 84 ms zero-cost completion; closeout-report and
   session-close-bridge skips included).
3. NEW — Owner ruling (BLOCKED row): reclaim-completion policy for
   partially-landed batches.
4. NEW — Investigate the env-streak burn on foreign-dirty-tree preflight
   requeues (facts 12427–12435; exempt-or-park proposal).

Strongest existing rows this window re-proved (no duplicates minted):

5. Rows 225/226 — the …3dc4 batch's skipped items; now the top of the
   redaction-hardening queue.
6. Row 835 — AGENTS.md rate-limits bullet + paired byte-shave (provider
   now rides evidence + parked surface; bullet still says log-only).
7. Rows 836-839 — parked provider follow-ups (JSON surface, doctor
   naming, LastFacts dedup, postgres conform proof).
8. Row 840 — FEATURES.md sweeper-row pointer swap (completes the 05-57
   MOVE ruling; kills the first-class/sweeper duplication).
9. Rows 841-846 — the 06-10 parity sweep's residue (check-dead-sha into
   ci.yml, guard-wiring into ci.yml, devmod tidy surfacing, cmd/tq
   vendor parity, local re-verify of the newly wired smokes, golangci
   version pin source).
10. Row 831 — worktree.sh fix-then-wire (guard-wiring orphan re-verified
    standing today).
11. Row 833 — push-timing guard (the 00-44 pushed sweep is the receipt;
    01-15's four sweeps are sighting #2's cause).
12. Row 830 — AGENTS.md headroom policy (BLOCKED; g3).
13. Row 827 — mechanical footer-position assert (third footer-placement
    sighting in the window's heal chains).
14. Rows 197/210 — full ci-local battery at HEAD (deferred by 4 window
    sessions).
15. Index-bloat ANNOTATE sweep (open row; 307 live rows, +3 this window).

Smaller, concrete, this-window-observed:

16. journal-drift phase-5 `--json` assertion (row 825 — already tracked,
    the phase's JSON side is unasserted).
17. Run the postgres conform suite with `TQ_TEST_POSTGRES` once before
    the next release wave so rows 839's question is answered with
    evidence, not policy.
18. Add the `"smoke fixtures compute dates"` convention line to AGENTS.md
    Conventions (00-44 §f10) — headroom permitting, rides row 835's
    paired shave.
19. Re-run `scripts/smoke/redaction.sh` + `journal-drift.sh` at HEAD once
    post-06-10 (the parity sweep wired smokes into ci.yml; a local green
    is row 845's ask — keep them paired).
20. `tq audit --journal --since <ts>` (existing idea, 02-26 §40 / 04-30
    §f27): would retire the whole pre/post-`4ba81d8` comparability
    caveat class the 04-30 batch documented.
21. Sweep the "Done-prompt harvest (2026-09-17)" TODO section header once
    rows 225/226/239 close — the section will be fully ticked and can be
    deleted wholesale (DONE items are deleted on closure).
22. Verify-only re-dispatch protection: the …011 task spent a whole paid
    session discovering its work was already landed (03-39 §e1); row 684
    (mint-side footer probe) owns the cure — pull it forward; a
    `git log -S <symbol>` beside the footer grep (03-55 §e1) is the
    cheap half.

## g) QUESTIONS ONLY THE OWNER CAN ANSWER

1. **Reclaim-completion policy**: when a multi-item batch's lease expires
   and the reclaiming worker finds partial work, must it re-verify the
   remaining item keys before a derived completion (or re-enqueue them)?
   The 2026-10-14 04:17 reclaim completed …3dc4 in 84 ms at zero cost
   with 2 of 3 items unlanded and no closeout. (Mechanics row minted
   unblocked; this ruling decides what the pin must enforce.)
2. **Daemon push grace**: should the auto-commit daemon hold footer-less
   sweeps unpushed for a healing grace window (or push only footed
   commits)? Asked 00-44 §g1 with the same evidence and still open;
   `c7e95833`'s permanent attribution loss is the standing cost. (The
   in-repo guard half is row 833; the daemon itself is owner-run.)
3. **AGENTS.md headroom**: standing ≥200 B reserve (prune-to-buffer) vs
   pay-as-you-add in the same commit? Row 830 is BLOCKED on this and
   rows 835/818-class AGENTS edits stall behind it; the file re-armed the
   trap at 19896 B / 204 B headroom this window.

## h) Band drift

**None recorded.** The production journal
(`/mnt/pool/services/tq/tq.db`, 12487 facts scanned via `tq facts
--type task.reprioritized --json`) contains **zero** `task.reprioritized`
facts — the type does not appear once, in this window or ever. All eight
window tasks ran exactly as dispatched: no stored-priority changes, no
marker/AI/unblock/importance moves to account for under ADR-0015. The
window's requeues were class-fact requeues (budget gate on …f7c at
enqueue, one 15m rate-limit wait, three preflight dirty-tree requeues on
…5f90) — claim-timing mechanics, not priority moves.

## Heal provenance (this report's own landing)

The auto-commit daemon swept this report + the CHANGELOG entry into
footer-less `cd0c482a` two minutes before the footed commit — the same
race this window's §d2 documents, now with a same-day receipt. Healed
via `scripts/heal-daemon-sweep.sh --from 9b51694e`; fork-records:
`cd0c482a→4d5ee9ae` (report + CHANGELOG entry, footer stamped) and
`c2700f78→ca7dd324` (index row + TODO tick + 4 mints).
