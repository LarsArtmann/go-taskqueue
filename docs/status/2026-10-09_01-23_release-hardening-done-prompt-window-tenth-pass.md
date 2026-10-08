# Done-Prompt Window — Release-Hardening Sweep, DONE-on-Arrival Re-Dispatch (Tenth Pass)

**Date:** 2026-10-09 01:23 CEST
**Window:** the five release-hardening tasks of 2026-10-08 00:07 → 01:53
**Task-Queue-ID:** `000001a11d8b7eeb8f33667e4ed600000000` — the SAME id already closed by passes six (`9d848da4` → `2026-10-09_00-40_release-hardening-done-prompt-window.md`, canonical), seven (`45073cb8`), eighth (`a064d73a`), and ninth (01-15 file). This is the short re-dispatch record per the DONE-on-arrival convention; the canonical full report is the sixth pass.

## a) FULLY DONE — re-verified in-tree first-hand this run

- **Demand-fill poke** (task …8221472e, master commit `4e6d7f88`): `scripts/release.sh:170-191` — `info_poke_ok` flag, every-attempt `.info` poke, demand-fill before `@v/list` polling. TODO row 35 `[x]`.
- **5/5 die split** (task …8fdb517f, `8fed04f3`): `scripts/release.sh:188-191` — "all .info pokes failed — network is dead …" vs the proxy-lag branch, keyed off `info_poke_ok`. TODO row 37 `[x]`.
- **Root-gate bash -n guard** (task …9031c, `bb280c05`): `scripts/root-gate.sh:21-28` — release.sh + lib scripts + itself, before the Go gates. TODO row 38 `[x]`.
- **Evidence archive** (task …a6bebc, `a27edabe`): honest BLOCKED — the /tmp log was pruned before pickup; row 39 carries `— BLOCKED:` and cites only the surviving prose. Nothing fabricated.
- **Reflow-hazard annotation** (task …411c, `1075248c`): hazard comment above the `need_in_both` pins in `scripts/check-release-docs.sh` (~lines 44-46). TODO row 40 `[x]`.

The re-dispatch prompt still cites the window's commits via the ABANDONED pre-rewrite lineage (`a24472c5`, `7c7a9063`, `9ea63fd9`, `c46151a0` — `git log --all` holds none of them this run) while master carries the content-twins (`4e6d7f88` family). Same finding as eighth/ninth passes §a; pool-side, out of reach.

## b) PARTIALLY DONE

Nothing new. Standing partials from the sixth pass: evidence forensics permanently lost (row 39 BLOCKED), reflow hazard documented-not-fixed (normalized-grep row open), both die paths never exercised against a live proxy outage.

## c) NOT STARTED

Nothing owed by the re-dispatch. Open follow-ups already rowed (sixth-pass rows 42-47 + the 00-40 harvest block + the 00-47 review-fix pair + BLOCKED trio): ~350 unchecked rows in TODO_LIST.md this run — the archive sweep matters more than any append.

## d) TOTALLY FUCKED UP

**The dispatch itself, not any work.** Fifth paid DONE-on-arrival burn on this id (00-40, 00-52, 01-10, 01-15, this 01-23). Root cause unchanged: the live pool runs the pre-guard cmd/tq binary, so the mint-time done-check never fires. Zero destructive change; this run writes only this report, its index row, and the commit.

## e) WHAT WE SHOULD IMPROVE

1. **cmd/tq release remains the highest-leverage deploy in the repo** (carried since 00-40) — it ends this burn class.
2. **Mint-side already-done guard** (carried): footer-grep at CLAIM; keeps not being deployed.
3. **Prompt hash provenance** (carried): unreachable-lineage hashes still reach prompts.
4. **Re-dispatch tombstone** (carried): after ≥2 DONE-on-arrival passes on one id, a tombstone beats another report.

## f) NEXT THINGS

**Zero appended.** Every candidate dedup-matches an existing row; re-appending mints duplicate paid dispatches — the exact loop the 2026-09-11 75-commit review killed.

## g) QUESTIONS

**None new.** The live BLOCKED set stands (sub-tag `.info` poke scope, bash -n guard end-state, dead-evidence re-anchor edit class for row 35/39, plus the 00-45 scheduling trio).

## h) BAND DRIFT

**None recorded.** The only journal reachable from this shell (`./tasks.db`, last modified 2026-10-08 00:44) yields zero `task.reprioritized` facts; the production pool journal remains unreachable ($TQ_DB unset in this session) — same vantage gap as passes 6-9, filed and standing.

---

*Point-in-time snapshot 2026-10-09 01:23. Fifth re-dispatch of `000001a11d8b7eeb8f33667e4ed600000000`; canonical report: `2026-10-09_00-40_release-hardening-done-prompt-window.md`. 0 TODO appends. Nothing pushed.*
