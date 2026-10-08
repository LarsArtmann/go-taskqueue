# Done-Prompt Window — Release-Hardening Five-Task Sweep (Sixth Pass)

**Date:** 2026-10-09 00:40 CEST
**Window:** 2026-10-08 00:07 → 2026-10-09 00:24 (five dispatched tasks + the RELEASE.md timeout-fallback landing and its two no-op re-dispatches)
**Sources:** the five closeout reports under `docs/status/tasks/2026-10-08_*`, git log 4e6d7f88→a909d57f, TODO_LIST.md, CHANGELOG.md, the reachable journal (`./tasks.db`).

## a) FULLY DONE (verified against commits, not just claimed)

| Task | Landed as | Verified by this pass |
|---|---|---|
| 000001a1188221472e (demand-fill poke) | commit `4e6d7f88` (+ closeout 820e9dd8) | `git log` shows the work commit; TODO row 35 `[x]` in TODO_LIST.md; closeout cites bash -n + check-release-docs + build/vet green |
| 000001a1188fdb517f (5/5 die split: network-dead vs proxy lag) | commit `8fed04f3` (+ closeout 9cea2d33) | closeout documents every-attempt poke + `info_poke_ok` flag; both die branches simulation-verified; TODO row 37 `[x]` |
| 000001a11899031c (root-gate bash -n syntax guard) | commit `bb280c05` (+ closeout d34d1fb7) | guard covers scripts/release.sh + scripts/lib/*.sh + itself, ~10 ms, before the Go gates; TODO row 38 `[x]`; full root build/vet/test -race claimed green in closeout |
| 000001a118a6bebc (archive /tmp evidence log) | commit `a27edabe` | honest outcome: the log was ALREADY pruned from /tmp (5-way source search); row turned `— BLOCKED:` instead of fabricating evidence; TODO row 39 marked BLOCKED |
| 000001a118c6ca4411c (reflow-hazard annotation) | commit `1075248c` (+ closeout cce14bf9) | annotation at scripts/check-release-docs.sh call site; TODO row 40 `[x]`; live gate run green |

Additionally in-window (adjacent task 000001a11d8b7eae, not in this window's dispatch set but landed between the fifth closeouts): RELEASE.md timeout-fallback paragraph rewritten (`ca5e80f7`, tick `d8d261c4`), then re-dispatched twice producing two verified NO-OP closeouts (`8f5a7e69`, `a909d57f`).

## b) PARTIALLY DONE

- **Evidence archive (task 000001a118a6bebc):** the dispatch was filed late and /tmp hygiene won — the raw v0.3.3 release-push log is unarchivable forever. The informational substance survives as prose in `docs/status/2026-10-07_16-50_v033-published-e2e-doctor-dlq-factory-m26-filed.md`, so the loss is forensic-raw only. Row 39 correctly BLOCKED, never fabricated.
- **Reflow hazard mitigation (task …411c):** the hazard is documented, not fixed — a RELEASE.md reflow will still false-fail check-release-docs.sh; the comment only converts the confusing failure into a quick diagnosis. Reflow-tolerance itself is rowed (`need_in_both` normalized-grep row).
- **Die-branch evidence:** both new release.sh die paths have never fired against a live proxy outage; bash -n + branch simulation + the docs gate are the only proxy evidence until the next real release.
- **Done-prompt self-application:** the live pool still runs the pre-routing-note cmd/tq binary (cmd/tq release pending — standing row), which is why the RELEASE.md task re-dispatched twice as DONE-on-arrival no-ops.

## c) NOT STARTED (backlog the window skipped)

Nothing owed by the five items. Open follow-up rows promoted by this window's closeouts and still unchecked: proxy-wait loop extraction + offline test, curl `-w '%{http_code}'` failure-mode split, `--publish-steps-only` resume, reflow-tolerant pins, check-release-docs `--self-test`, `poked demand-fill` log-line pin, ci-local bash -n fast-pass hoist, guard self-test, release-path script-set naming. All are already in TODO_LIST.md — no new dispatch needed beyond what rows carry.

## d) TOTALLY FUCKED UP

- **The one real loss is scheduling, not execution:** the v0.3.3 evidence log aged out of /tmp between filing (01-47 closeout's predecessor §e5, ~00:38) and pickup (~01:47). An evidence-citing row with a deferral window is a ticking row; the lesson (archive-at-filing) is encoded below in (e)/(f).
- **Re-dispatch burn continues:** the RELEASE.md timeout-fallback task consumed 3 dispatches in ~20 minutes for 1 landing + 2 no-op closeouts. Root cause is structural (live pool runs the old binary without the mint-time already-done guard); the grep-first contract worked exactly as designed each time — zero wrong work, but real paid turns.
- **Nothing destructive:** no reverted foreign work, no broken gates observed at HEAD (this report's own gates are run below), no force-anything in the window's commits.

## e) WHAT WE SHOULD IMPROVE

1. **Archive-at-filing discipline:** a closeout citing a /tmp artifact must run `scripts/archive-evidence.sh` in the SAME session; the deferral window is the kill window (this window paid for it once).
2. **Aging-evidence lint:** check-todo-list.sh should flag unchecked rows citing `/tmp/` paths — they are likely-dead by pickup time.
3. **Mint-side already-done guard deployment:** every DONE-on-arrival re-dispatch persists until the next cmd/tq release reaches the pool; the standing row exists — it is the highest-leverage release in the backlog.
4. **A comment is the weakest mitigation:** the reflow hazard annotation is correct but bug-shaped; the normalized-grep fix is ~5 lines and already rowed — prioritize it over further annotation work in this class.
5. **Band-drift observability gap (noted in passing):** the journal reachable from a task shell (`$TQ_DB` unset, `./tasks.db` stale at 2026-09-09) cannot see pool-era `task.reprioritized` facts; the ADR-0015 accountability section below is written from a provably-blind vantage. A row documenting the expected TQ_DB for done-prompt sessions would close this.

## f) NEXT THINGS (top of the backlog; dedup-checked against live rows — appends below)

1. Audit sibling release scripts (docs/release/ facade flow, scripts/check-pkg-proxy.sh) for the same passive-poll-only proxy verification — internal sub-tags may still carry the v0.3.3 failure mode (00-38 closeout f9).
2. check-todo-list.sh: flag unchecked rows citing `/tmp/` paths as aging/dead (01-47 closeout §e3).
3. Closeout convention: REQUIRE in-line archiving of any /tmp evidence at filing time — one paragraph in AGENTS.md conventions + the closeout prompt (01-47 §f1).
4. Guard-wiring existence assertion: every root-gate.sh step's target must exist (one-shot check, same pattern as check-guard-wiring.sh's session-start assertion) (01-02 §e4/f5).
5. Sweep other check-* drift smokes for the line-contained-grep hazard class (check-doc-refs, check-facade-parity pins) — one audit row (01-51 §f3).
6. Audit whether verify-battery.sh invokes shell scripts root-gate's syntax list misses (01-02 §g3).
7. Add `--max-time` to the release.sh .info poke curl so a hung connection cannot stall an attempt (00-38 f23; rides the existing http_code-split row).
8. DONE-row evidence hygiene: demand-fill row 35 cites the dead /tmp log as primary evidence — re-anchor to the surviving prose report (needs an owner-sanctioned edit class; see g3).

## g) QUESTIONS ONLY THE OWNER CAN DECIDE

1. Should facade sub-module tags get their own `.info` pokes in the same proxy-verify loop, or does the clean-room `go get` step make that redundant? (00-38 §g2)
2. Is the root-gate bash -n guard's release-path scoping the intended end state, or should it grow to all tracked `*.sh` (duplicating check-script-syntax earlier)? (01-02 §g1)
3. May checked DONE rows ever be annotated (re-anchoring row 35's evidence from the dead /tmp path to the surviving report), or is tick-only absolute even for dead evidence citations?

## h) BAND DRIFT

**None recorded.** The only journal reachable from this session (`./tasks.db`) holds 189 facts ending 2026-09-09 with zero `task.reprioritized` facts of any date; the production pool journal is not visible from a done-prompt shell ($TQ_DB unset in this session), so pool-era priority moves cannot be enumerated here. Consistent with the fifth-pass report (00-45), which likewise recorded zero reprioritized facts journal-wide. No priority change in this window is therefore unexplained — but the vantage gap itself is filed as (e)5/(f)-adjacent work.

---
*Point-in-time snapshot 2026-10-09 00:40. 10 TODO appends (7 new tasks + 3 blocked questions). Nothing pushed.*
