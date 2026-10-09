# Done-Prompt Window — Release-Hardening Sweep, DONE-on-Arrival Re-Dispatch (Twelfth Pass)

**Date:** 2026-10-09 01:45 CEST
**Window:** the five release-hardening tasks of 2026-10-08 00:07 → 01:53
**Task-Queue-ID:** `000001a11d8b7eeb8f33667e4ed600000000` — the SAME id already closed by the sixth pass (`9d848da4` → `2026-10-09_00-40_release-hardening-done-prompt-window.md`, canonical full report) and re-dispatched seven times since (45073cb8, 01-10, 01-15, 01-23, 01-29, and this 01-45). DONE-on-arrival short re-dispatch record.

## a) FULLY DONE — re-verified in-tree first-hand this run

- **Demand-fill poke** (task …8221472e, master `4e6d7f88`): `scripts/release.sh:170-191` — `info_poke_ok` flag, every-attempt `@v/<ver>.info` poke (demand-fill) before the `@v/list` poll. TODO row 35 `[x]`.
- **5/5 die split** (task …8fdb517f, `8fed04f3`): `scripts/release.sh:188-191` — "all .info pokes failed — network is dead…" vs the proxy-lag branch, keyed off `info_poke_ok`. TODO row 37 `[x]`.
- **Root-gate bash -n guard** (task …9031c, `bb280c05`): `scripts/root-gate.sh:23-30` — release.sh + `scripts/lib/*.sh` + itself, ahead of the Go gates. TODO row 38 `[x]`.
- **Evidence archive** (task …a6bebc, `a27edabe`): honest BLOCKED — the /tmp log was pruned before pickup; row 39 carries `— BLOCKED:` and cites only surviving prose. Nothing fabricated.
- **Reflow-hazard annotation** (task …411c, `1075248c`): hazard comment at `scripts/check-release-docs.sh:43-47` above the `need_in_both` pins. TODO row 40 `[x]`.

Prompt-lineage note (unchanged from passes eight–eleven): the dispatch cites `a24472c5`/`7c7a9063`/`9ea63fd9`/`c46151a0`; those objects exist in the store but no branch contains them — master carries the content-twins above.

## b) PARTIALLY DONE

Nothing new. Standing: evidence forensics permanently lost (row 39 BLOCKED); reflow hazard documented-not-fixed (normalized-grep row open); neither die path exercised against a live outage.

## c) NOT STARTED

Nothing owed by the re-dispatch. All follow-ups already rowed; the archive sweep matters more than any append.

## d) TOTALLY FUCKED UP

**The dispatch itself.** Seventh paid DONE-on-arrival burn on this id. Root cause unchanged: the live pool runs the pre-guard cmd/tq binary, so the mint-time done-check never fires. Zero destructive change.

## e) WHAT WE SHOULD IMPROVE

Carried, unchanged: (1) cmd/tq release + pool redeploy ends this burn class; (2) mint-side footer-grep done-check; (3) object-based (not ref-based) provenance done-checks; (4) a tombstone after ≥2 DONE-on-arrival passes — seven passes deep, overdue.

## f) NEXT THINGS

**Zero appended.** Every candidate dedup-matches an existing row; re-appending mints duplicate paid dispatches.

## g) QUESTIONS

**None new.** The live BLOCKED set stands (sub-tag `.info` scope, bash -n guard end-state, dead-evidence re-anchor edit class, 00-45 scheduling trio).

## h) BAND DRIFT

**None recorded.** Reachable journal `./tasks.db` (397 facts) holds zero `task.reprioritized` facts; the production pool journal remains unreachable ($TQ_DB unset this session) — same vantage gap as passes 6–11.

---

_Point-in-time snapshot 2026-10-09 01:45. Seventh re-dispatch of `000001a11d8b7eeb8f33667e4ed600000000`; canonical report: `2026-10-09_00-40_release-hardening-done-prompt-window.md`. 0 TODO appends. Nothing pushed._
