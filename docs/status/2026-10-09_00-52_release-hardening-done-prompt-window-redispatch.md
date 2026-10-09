# Done-Prompt Window — Release-Hardening Sweep, DONE-on-Arrival Re-Dispatch (Seventh Pass)

**Date:** 2026-10-09 00:52 CEST
**Window:** the five release-hardening tasks of 2026-10-08 00:07 → 01:53
**Task-Queue-ID:** `000001a11d8b7eeb8f33667e4ed600000000` — the SAME id the sixth-pass done-prompt already closed (commit `9d848da4`, report `docs/status/2026-10-09_00-40_release-hardening-done-prompt-window.md`, indexed at `docs/status/README.md` 2026-10-09 cluster). This run is a RE-DISPATCH of a task whose full status report already exists; per the DONE-on-arrival convention this file is the short re-dispatch record, not a second derivation.

## a) FULLY DONE — re-verified in-tree first-hand this run

- **Demand-fill poke** (task …8221472e, commit `4e6d7f88`): `scripts/release.sh:170-178` — `info_poke_ok` flag + every-attempt `curl -fsS …/@v/<ver>.info` poke + greppable `poked demand-fill:` line. TODO row 35 `[x]`.
- **5/5 die split** (task …8fdb517f, commit `8fed04f3`): `scripts/release.sh:188-190` — network-dead vs proxy-lag branches on `info_poke_ok`. TODO row 37 `[x]`.
- **Root-gate bash -n guard** (task …9031c, commit `bb280c05`): `scripts/root-gate.sh:21-28` — release.sh + lib scripts + self. TODO row 38 `[x]`.
- **Evidence archive** (task …a6bebc, commit `a27edabe`): honest BLOCKED — /tmp log pruned pre-pickup (5-way search); row 39 `— BLOCKED:`, no fabrication. Disposition unchanged.
- **Reflow-hazard annotation** (task …411c, commit `1075248c`): `scripts/check-release-docs.sh:43` hazard comment. TODO row 40 `[x]`.

## b) PARTIALLY DONE

Nothing new. The sixth-pass report's partials stand: evidence-forensics lost (prose survives in the 2026-10-07_16-50 reports), reflow hazard documented-not-fixed (row 45), die paths unexercised against a live outage until the next release.

## c) NOT STARTED

Nothing owed by the re-dispatch. Open follow-ups are already rowed: 42 (testable proxy-wait), 43 (http_code split), 44 (`--publish-steps-only`), 45 (reflow-tolerant pins), 46 (check-release-docs self-test), 47 (`poked demand-fill` pin), plus the ci-local fast-pass/guard-self-test trio from the 01-02 closeout.

## d) TOTALLY FUCKED UP

**The dispatch itself, not any work.** This is now the second DONE-on-arrival burn on this window's ids in ~12 minutes of pool time (the RELEASE.md timeout-fallback task burned two no-ops earlier tonight; the cmd/tq release that would end this class is still pending — the live pool runs the pre-guard binary). No destructive change; this run's only writes are this report, its index row, and the commit.

## e) WHAT WE SHOULD IMPROVE

1. **Mint-side already-done guard**: a task whose Task-Queue-ID footer already exists in `git log` should be refused or flagged at CLAIM time, not at closeout — grep-first works, but the dispatch still costs a paid turn. (Carried: the 00-13/00-22 closeouts filed this; it keeps paying.)
2. **Done-prompt ids deserve the same footer grep** the per-task closeouts run — this pass did it, but only because the closeout convention was followed; the done-prompt prompt itself does not ask for it.
3. Index bloat continues (257+ live rows, ~10/day) — the seventh no-op row this window adds to a backlog that needs the deferred archive sweep more than it needs this row.
4. cmd/tq release remains the highest-leverage deploy in the repo — every re-dispatch class traces back to the pool running stale binaries.

## f) NEXT THINGS

**Zero appended.** The sixth-pass report already appended 7 task rows + 3 blocked questions (rows 42-47 + 582 and siblings); every candidate this run derived dedup-matched an existing row. Re-appending would mint duplicate dispatches — the exact loop the 2026-09-11 75-commit review killed.

## g) QUESTIONS

**None new.** The three owner questions from the sixth-pass §g (sub-tag .info pokes, guard-scope end state, dead-evidence re-anchor edit class) are already filed as BLOCKED rows and remain the live set.

## h) BAND DRIFT

**None recorded.** The reachable journal (`./tasks.db`, TQ_DB inherited) holds 189 facts ending 2026-09-09 with zero `task.reprioritized` facts of any date; the production pool journal is not reachable from this shell. Identical vantage to the sixth-pass §h — the pool-era visibility gap stands as filed.

---

_Point-in-time snapshot 2026-10-09 00:52. Re-dispatch of `000001a11d8b7eeb8f33667e4ed600000000`; canonical report: `2026-10-09_00-40_release-hardening-done-prompt-window.md`. 0 TODO appends. Nothing pushed._
