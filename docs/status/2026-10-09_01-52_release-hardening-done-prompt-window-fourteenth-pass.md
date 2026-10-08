# Done-Prompt Window — Release-Hardening Sweep, DONE-on-Arrival Re-Dispatch (Fourteenth Pass)

**Date:** 2026-10-09 01:52 CEST
**Window:** the five release-hardening tasks of 2026-10-08 00:07 → 01:53
**Task-Queue-ID:** `000001a11d8b7eeb8f33667e4ed600000000` — the SAME id already closed by the sixth pass (`9d848da4` → `2026-10-09_00-40_release-hardening-done-prompt-window.md`, canonical full report) and re-dispatched nine times since (45073cb8, 01-10, 01-15, 01-23, 01-29, 01-45, 01-47 thirteenth, and this 01-52). DONE-on-arrival short re-dispatch record.

## a) FULLY DONE — re-verified in-tree first-hand this run

- **Demand-fill poke** (task …8221472e): `scripts/release.sh:168-192` — `info_poke_ok` flag; every attempt curls `@v/<ver>.info` before the `@v/list` poll, with the v0.3.3 rationale comment. TODO row 35 `[x]`.
- **5/5 die split** (task …8fdb517f): `scripts/release.sh:188-193` — the `attempt=5` branch dies "all .info pokes failed — network is dead…" when `info_poke_ok=false`, vs the proxy-lag branch otherwise. TODO row 37 `[x]`.
- **Root-gate bash -n guard** (task …9031c): `scripts/root-gate.sh:23-30` — `bash -n` over `scripts/release.sh`, `scripts/lib/*.sh`, and the gate itself, ahead of the Go gates. TODO row 38 `[x]`.
- **Evidence archive** (task …a6bebc): honest BLOCKED — `/tmp/tq-release-push-v033c.log` was pruned before pickup; TODO row 39 carries `— BLOCKED:` and cites only the surviving prose. Nothing fabricated.
- **Reflow-hazard annotation** (task …411c): FALSE-FAIL HAZARD comment at `scripts/check-release-docs.sh:43-47` above the `need_in_both` pins. TODO row 40 `[x]`.

Prompt-lineage note (unchanged from passes six–thirteen): the dispatch cites `a24472c5`/`7c7a9063`/`9ea63fd9`/`c46151a0`; those objects exist in the object store but no branch contains them — master carries the content-twins above.

## b) PARTIALLY DONE

Nothing new. Standing: evidence forensics permanently lost (row 39 BLOCKED); the reflow hazard is documented-not-fixed (normalized-grep row open); neither release.sh die path has been exercised against a live outage.

## c) NOT STARTED

Nothing owed by the re-dispatch. All follow-ups already rowed in TODO_LIST; zero appends this pass.

## d) TOTALLY FUCKED UP

**The dispatch itself.** Ninth paid DONE-on-arrival burn on this id. Root cause unchanged: the live pool runs the pre-guard cmd/tq binary, so the mint-time done-check never fires. Zero destructive change.

## e) WHAT WE SHOULD IMPROVE

Carried, unchanged: (1) cmd/tq release + pool redeploy ends this burn class; (2) mint-side footer-grep done-check; (3) object-based (not ref-based) provenance done-checks; (4) a tombstone after ≥2 DONE-on-arrival passes — nine passes deep, overdue.

## f) NEXT THINGS

**Zero appended.** Every candidate dedup-matches an existing TODO_LIST row (rows 35, 37, 38, 39, 40 already carry the five window tasks); re-appending would mint duplicate paid dispatches (the exact loop the 2026-09-11 75-commit review documented).

## g) QUESTIONS

**None new.** The live BLOCKED set stands: sub-tag `.info` poke scope, bash -n guard end-state, dead-evidence re-anchor edit class, and the 00-45 scheduling trio.

## h) BAND DRIFT

**None recorded.** Reachable journal `./tasks.db` (397 facts, queried via `tq facts --db tasks.db` this run) holds zero `task.reprioritized` facts; the production pool journal remains unreachable ($TQ_DB unset this session) — same vantage gap as passes 6–13.

---

*Point-in-time snapshot 2026-10-09 01:52. Ninth re-dispatch of `000001a11d8b7eeb8f33667e4ed600000000`; canonical report: `2026-10-09_00-40_release-hardening-done-prompt-window.md`. 0 TODO appends. Nothing pushed.*
