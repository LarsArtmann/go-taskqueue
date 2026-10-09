# Done-Prompt Window — Release-Hardening Sweep, DONE-on-Arrival Re-Dispatch (Eighth Pass)

**Date:** 2026-10-09 01:10 CEST
**Window:** the five release-hardening tasks of 2026-10-08 00:07 → 01:53
**Task-Queue-ID:** `000001a11d8b7eeb8f33667e4ed600000000` — the SAME id the sixth-pass done-prompt (commit `9d848da4`, `docs/status/2026-10-09_00-40_release-hardening-done-prompt-window.md`) and the seventh-pass re-dispatch (commit `45073cb8`, `docs/status/2026-10-09_00-52_release-hardening-done-prompt-window-redispatch.md`) already closed. This file is the short re-dispatch record per the DONE-on-arrival convention; the canonical full report is the sixth pass.

## a) FULLY DONE — re-verified in-tree first-hand this run

- **Demand-fill poke** (task …8221472e, master commit `4e6d7f88`): `scripts/release.sh:166-179` — `info_poke_ok` flag, every-attempt `curl -fsS …/@v/<ver>.info` poke, greppable `poked demand-fill:` line. TODO row 35 `[x]`.
- **5/5 die split** (task …8fdb517f, `8fed04f3`): `scripts/release.sh:188-193` — network-dead ("all .info pokes failed — network is dead") vs proxy-lag ("@v/list never listed … proxy lag") branches on `info_poke_ok`. TODO row 37 `[x]`.
- **Root-gate bash -n guard** (task …9031c, `bb280c05`): `scripts/root-gate.sh:21-28` — release.sh + scripts/lib/*.sh + itself. TODO row 38 `[x]`.
- **Evidence archive** (task …a6bebc, `a27edabe`): honest BLOCKED — the /tmp log was pruned pre-pickup (5-way search, 01-47 closeout); row 39 carries `— BLOCKED:`, nothing fabricated. Disposition unchanged.
- **Reflow-hazard annotation** (task …411c, `1075248c`): `scripts/check-release-docs.sh:43-46` hazard comment. TODO row 40 `[x]`.

NEW this run: the re-dispatch prompt cites the window's landing commits as `a24472c5`, `7c7a9063`, `9ea63fd9`, `c46151a0` — those objects EXIST in this repo but on an ABANDONED pre-rewrite lineage (its work commit `52354e63` "release: poke proxy demand-fill…" is the content-twin of master's `4e6d7f88`); `git branch -a --contains a24472c5` is empty. The dispatch prompt therefore carries hashes unreachable from any branch while master carries the live twins. Work is unaffected; the mint/prompt hash-capture path is recording rewritten-lineage refs (noted in e).

## b) PARTIALLY DONE

Nothing new. Standing partials from the sixth pass: evidence forensics lost (prose survives in the 2026-10-07_16-50 reports; row 39 permanently BLOCKED), reflow hazard documented-not-fixed (row 45), die paths unexercised against a live outage until the next real release.

## c) NOT STARTED

Nothing owed by the re-dispatch. Open follow-ups already rowed: 42 (testable proxy-wait), 43 (http_code split), 44 (`--publish-steps-only`), 45 (reflow-tolerant pins), 46 (check-release-docs self-test), 47 (`poked demand-fill` pin), the ci-local fast-pass/guard-self-test/script-set-naming trio, and the 00-40 harvest block (sibling-script audit, /tmp lint, archive-at-filing, guard-wiring assertion, drift-smoke sweep, verify-battery audit, `--max-time`) plus the three BLOCKED owner questions.

## d) TOTALLY FUCKED UP

**The dispatch itself, not any work.** Third paid DONE-on-arrival burn on this window's done-prompt id in ~20 minutes of pool time (sixth pass 00-40, seventh 00-52, this 01-10), plus the sibling re-dispatch storm on the other two ids tonight. Root cause unchanged: the live pool runs the pre-guard cmd/tq binary, so the mint-time done-check and closeout-footprint detection never fire. No destructive change anywhere; this run's only writes are this report, its index row, and the commit.

## e) WHAT WE SHOULD IMPROVE

1. **cmd/tq release remains the highest-leverage deploy in the repo** — every re-dispatch class tonight traces to the pool running stale binaries. Already rowed (cut-next-release row); it keeps being the answer.
2. **Mint-side already-done guard** (carried): footer-grep at CLAIM, not closeout. Rowed; keeps paying.
3. **NEW: prompt hash provenance** — the dispatch prompt embedded commit hashes from an unreachable rewrite lineage (52354e63 family). Whatever captures hashes into prompts/mint briefs should verify reachability from a branch (or capture at dispatch time, not mint time). Report-only this pass; the mechanism lives pool-side, out of reach.
4. Index bloat continues (259 live rows at gate time, threshold 100) — the deferred archive sweep matters more than this report's row.
5. The done-prompt prompt itself does not ask for the footer grep that per-task closeouts run; this pass ran it, but it should be contract, not habit (carried from 00-52).

## f) NEXT THINGS

**Zero appended.** The sixth pass already appended 7 task rows + 3 blocked questions (rows 42-47 + the 00-40 BLOCKED trio); the 00-47 review-fix pass appended the unindexed-closeout sweep + advisory tasks/ mode; every candidate this run derived (including the §e3 hash-provenance observation) is either report-only or dedup-matches an existing row. Re-appending mints duplicate paid dispatches — the exact loop the 2026-09-11 75-commit review killed.

## g) QUESTIONS

**None new.** The live BLOCKED set from the sixth pass stands: sub-tag .info pokes scope call, root-gate bash -n guard scope end-state, dead-evidence re-anchor edit class (row 39's pruned-/tmp citation).

## h) BAND DRIFT

**None recorded.** The reachable journal (`./tasks.db`) holds 0 facts; the production pool journal is not reachable from this shell — same vantage gap as the sixth and seventh passes, filed and standing. Zero `task.reprioritized` facts visible anywhere this run.

---

_Point-in-time snapshot 2026-10-09 01:10. Third re-dispatch of `000001a11d8b7eeb8f33667e4ed600000000`; canonical report: `2026-10-09_00-40_release-hardening-done-prompt-window.md`. 0 TODO appends. Nothing pushed._
