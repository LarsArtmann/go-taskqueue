# Done-Prompt Window — Release-Hardening Sweep, DONE-on-Arrival Re-Dispatch (Eleventh Pass)

**Date:** 2026-10-09 01:29 CEST
**Window:** the five release-hardening tasks of 2026-10-08 00:07 → 01:53
**Task-Queue-ID:** `000001a11d8b7eeb8f33667e4ed600000000` — the SAME id already closed by passes six (`9d848da4` → `2026-10-09_00-40_release-hardening-done-prompt-window.md`, canonical full report), seven (`45073cb8`), eighth (01-10), ninth (01-15), and tenth (01-23). DONE-on-arrival short re-dispatch record; canonical report is the sixth pass.

## a) FULLY DONE — re-verified in-tree first-hand this run

- **Demand-fill poke** (task …8221472e, master commit `4e6d7f88`): `scripts/release.sh:170-191` — `info_poke_ok` flag, every-attempt `.info` poke, demand-fill before `@v/list` polling. TODO row 35 `[x]`.
- **5/5 die split** (task …8fdb517f, `8fed04f3`): `scripts/release.sh:188-191` — "all .info pokes failed — network is dead …" vs the proxy-lag branch, keyed off `info_poke_ok`. TODO row 37 `[x]`.
- **Root-gate bash -n guard** (task …9031c, `bb280c05`): `scripts/root-gate.sh:21-28` — release.sh + `scripts/lib/*.sh` + itself, ahead of the Go gates. TODO row 38 `[x]`.
- **Evidence archive** (task …a6bebc, `a27edabe`): honest BLOCKED — the /tmp log was pruned before pickup; row 39 carries `— BLOCKED:` and cites only surviving prose. Nothing fabricated.
- **Reflow-hazard annotation** (task …411c, `1075248c`): hazard comment above the `need_in_both` pins in `scripts/check-release-docs.sh:43-47`. TODO row 40 `[x]`.

Prompt-lineage note, refined this pass: the dispatch cites `a24472c5`/`7c7a9063`/`9ea63fd9`/`c46151a0`. This run confirms those objects EXIST in the object store (`git cat-file -t` → commit; `a24472c5` is a 2026-10-08 00:40:58 closeout commit whose subject twins master's `820e9dd8`) but are contained by NO branch (`git branch -a --contains` empty) — pre-rewrite lineage reachable only through loose objects/reflogs. Master carries the content-twins (`4e6d7f88`, `8fed04f3`, `bb280c05`, `a27edabe`, `1075248c`). Same finding as eighth–tenth passes, now with the objects-present-but-unreachable detail.

## b) PARTIALLY DONE

Nothing new. Standing partials from the sixth pass: evidence forensics permanently lost (row 39 BLOCKED), reflow hazard documented-not-fixed (normalized-grep row open), both die paths never exercised against a live proxy outage.

## c) NOT STARTED

Nothing owed by the re-dispatch. Open follow-ups already rowed (sixth-pass rows 42-47 + 00-40 harvest block + 00-47 review-fix pair + BLOCKED trio): ~350 unchecked rows in TODO_LIST.md this run — the archive sweep matters more than any append.

## d) TOTALLY FUCKED UP

**The dispatch itself, not any work.** Sixth paid DONE-on-arrival burn on this id (00-40, 00-52, 01-10, 01-15, 01-23, this 01-29). Root cause unchanged: the live pool runs the pre-guard cmd/tq binary, so the mint-time done-check never fires. Zero destructive change; this run writes only this report, its index row, and the commit.

## e) WHAT WE SHOULD IMPROVE

1. **cmd/tq release remains the highest-leverage deploy in the repo** (carried since 00-40) — it ends this burn class.
2. **Mint-side already-done guard** (carried): footer-grep at CLAIM; keeps not being deployed.
3. **Prompt hash provenance** (carried): unreachable-lineage hashes still reach prompts; this pass sharpens it — the objects survive while no ref reaches them, so ref-based done-checks miss what object-based ones would catch.
4. **Re-dispatch tombstone** (carried): after ≥2 DONE-on-arrival passes on one id, a tombstone beats another report; six passes deep, this is overdue.

## f) NEXT THINGS

**Zero appended.** Every candidate dedup-matches an existing row; re-appending mints duplicate paid dispatches — the exact loop the 2026-09-11 75-commit review killed.

## g) QUESTIONS

**None new.** The live BLOCKED set stands (sub-tag `.info` poke scope, bash -n guard end-state, dead-evidence re-anchor edit class for rows 35/39, plus the 00-45 scheduling trio).

## h) BAND DRIFT

**None recorded.** The only journal reachable from this shell (`./tasks.db`, mtime 2026-10-08 00:44, 397 facts) yields zero `task.reprioritized` facts; the production pool journal remains unreachable ($TQ_DB unset in this session) — same vantage gap as passes 6-10, filed and standing.

---

*Point-in-time snapshot 2026-10-09 01:29. Sixth re-dispatch of `000001a11d8b7eeb8f33667e4ed600000000`; canonical report: `2026-10-09_00-40_release-hardening-done-prompt-window.md`. 0 TODO appends. Nothing pushed.*
