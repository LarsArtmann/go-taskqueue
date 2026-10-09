# Done-Prompt Window — Release-Hardening Sweep, DONE-on-Arrival Re-Dispatch (Ninth Pass)

**Date:** 2026-10-09 01:15 CEST
**Window:** the five release-hardening tasks of 2026-10-08 00:07 → 01:53
**Task-Queue-ID:** `000001a11d8b7eeb8f33667e4ed600000000` — the SAME id already closed by the sixth pass (`9d848da4`, `2026-10-09_00-40_release-hardening-done-prompt-window.md`), seventh (`45073cb8`), and eighth (`a064d73a`-era, `2026-10-09_01-10_...-eighth-pass.md`) passes. This file is the short re-dispatch record per the DONE-on-arrival convention; the canonical full report is the sixth pass.

## a) FULLY DONE — re-verified in-tree first-hand this run

- **Demand-fill poke** (task …8221472e, master commit `4e6d7f88`): `scripts/release.sh:170-178` — `info_poke_ok` flag, every-attempt `.info` poke, `poked demand-fill:` line. TODO row 35 `[x]`.
- **5/5 die split** (task …8fdb517f, `8fed04f3`): `scripts/release.sh:188-191` — "all .info pokes failed — network is dead" vs "@v/list never listed … proxy lag" branches on `info_poke_ok`. TODO row 37 `[x]`.
- **Root-gate bash -n guard** (task …9031c, `bb280c05`): `scripts/root-gate.sh:21-28` — release.sh + scripts/lib/*.sh + itself. TODO row 38 `[x]`.
- **Evidence archive** (task …a6bebc, `a27edabe`): honest BLOCKED — the /tmp log was pruned pre-pickup; row 39 carries `— BLOCKED:`, nothing fabricated. Disposition unchanged.
- **Reflow-hazard annotation** (task …411c, `1075248c`): `scripts/check-release-docs.sh:43-46` hazard comment above the `need_in_both` pins. TODO row 40 `[x]`.

The re-dispatch prompt still cites the window's commits via the ABANDONED pre-rewrite lineage (`a24472c5`, `7c7a9063`, `9ea63fd9`, `c46151a0` — `git branch -a --contains a24472c5` empty this run) while master carries the content-twins (`4e6d7f88` family). Same finding as eighth pass §a; pool-side, out of reach.

## b) PARTIALLY DONE

Nothing new. Standing partials from the sixth pass: evidence forensics lost (row 39 permanently BLOCKED), reflow hazard documented-not-fixed (row 45), die paths unexercised against a live outage until the next real release.

## c) NOT STARTED

Nothing owed by the re-dispatch. Open follow-ups already rowed: 42, 43, 44, 45, 46, 47, the ci-local fast-pass / guard-self-test / script-set-naming trio, the 00-40 harvest block, the 00-47 review-fix pair (unindexed-closeout sweep, advisory tasks/ mode), and the three-plus BLOCKED owner questions.

## d) TOTALLY FUCKED UP

**The dispatch itself, not any work.** Fourth paid DONE-on-arrival burn on this window's done-prompt id (sixth 00-40, seventh 00-52, eighth 01-10, this 01-15). Root cause unchanged: the live pool runs the pre-guard cmd/tq binary, so the mint-time done-check never fires. No destructive change anywhere; this run's only writes are this report, its index row, and the commit.

## e) WHAT WE SHOULD IMPROVE

1. **cmd/tq release remains the highest-leverage deploy in the repo** (carried from every pass since 00-40).
2. **Mint-side already-done guard** (carried): footer-grep at CLAIM, not closeout — row 156 keeps not being deployed.
3. **Prompt hash provenance** (carried, eighth-pass §e3): unreachable-lineage hashes still reach prompts.
4. **Index bloat** (carried): ~350 unchecked rows vs the 100 threshold; the deferred archive sweep matters more than this row.
5. **Re-dispatch tombstone**: after ≥2 DONE-on-arrival passes on one id, a tombstone that stops the pool from paying again is worth more than any new report content — still report-only.

## f) NEXT THINGS

**Zero appended.** Every candidate dedup-matches an existing row (sixth-pass rows 42-47 + BLOCKED trio, the 00-40 harvest block, the 00-47 pair). Re-appending mints duplicate paid dispatches — the exact loop the 2026-09-11 75-commit review killed.

## g) QUESTIONS

**None new.** The live BLOCKED set stands: sub-tag .info pokes scope call, root-gate bash -n guard scope end-state, dead-evidence re-anchor edit class (row 39), plus the scheduling/curation trio from the 00-45 report.

## h) BAND DRIFT

**None recorded.** The reachable journal (`./tasks.db`, 397 facts) holds ZERO `task.reprioritized` facts — grepped this run. The production pool journal remains unreachable from this shell (same vantage gap as passes 6-8, filed and standing). Zero priority moves visible anywhere.

---

_Point-in-time snapshot 2026-10-09 01:15. Fourth re-dispatch of `000001a11d8b7eeb8f33667e4ed600000000`; canonical report: `2026-10-09_00-40_release-hardening-done-prompt-window.md`. 0 TODO appends. Nothing pushed._
