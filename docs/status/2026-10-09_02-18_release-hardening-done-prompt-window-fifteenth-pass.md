# Done-Prompt Window — Release-Hardening Sweep, DONE-on-Arrival Re-Dispatch (Fifteenth Pass)

**Date:** 2026-10-09 02:18 CEST
**Window:** the five release-hardening tasks of 2026-10-08 00:07 → 01:53
**Task-Queue-ID:** `000001a11d8b7eeb8f33667e4ed600000000` — the SAME id already closed by the sixth pass (`2026-10-09_00-40_release-hardening-done-prompt-window.md`, canonical full report) and re-dispatched TEN times since (01-10 through the 01-52 fourteenth pass). DONE-on-arrival short re-dispatch record.

## a) FULLY DONE — re-verified in-tree first-hand this run

- **Demand-fill poke** (task …8221472e): `scripts/release.sh` module-proxy wait pokes `proxy.golang.org/$MODULE/@v/$VERSION.info` before each `@v/list` poll, with the v0.3.3 rationale comment (passive @v/list never triggers the on-demand fill). TODO row 35 `[x]`; CHANGELOG [Unreleased]/Fixed entry present; AGENTS.md carries the release-flow note (the 00-38 §f50 memory item).
- **5/5 die split** (task …8fdb517f): the `attempt=5` branch dies "all .info pokes failed — network is dead…" when `info_poke_ok=false`, vs the proxy-lag branch otherwise; both `die` paths verified in the script source this run. TODO row 37 `[x]`.
- **Root-gate bash -n guard** (task …9031c): `scripts/root-gate.sh` syntax-checks `scripts/release.sh` + `scripts/lib/*.sh` + itself ahead of the Go gates. TODO row 38 `[x]`; CHANGELOG Added entry present.
- **Evidence archive** (task …a6bebc): honest BLOCKED — `/tmp/tq-release-push-v033c.log` was pruned before pickup; TODO row 39 carries `— BLOCKED:` and cites only the surviving prose in the 2026-10-07_16-50 report. Nothing fabricated; proven correct: the file is gone from /tmp (nothing to archive exists at this pass either).
- **Reflow-hazard annotation** (task …411c): FALSE-FAIL HAZARD comment above the `need_in_both` pins in `scripts/check-release-docs.sh`; TODO row 40 `[x]`; CHANGELOG Fixed entry present; reflow-tolerance follow-ups already rowed (rows 45–47).

**Prompt-lineage note (unchanged since pass six):** the dispatch cites `a24472c52b…`, `7c7a9063…`, `9ea63fd9…`, `c46151a0…`; those objects exist in the object store but NO branch contains them (`git branch -a --contains a24472c5` is empty). Master carries the content-twins (`8fed04f3` die split, `4e6d7f88` poke, `bb280c05` guard, `1075248c` annotation) — the work itself is fully landed; only the cited hashes are unreachable, the auto-commit-daemon rewrite signature.

## b) PARTIALLY DONE

Nothing new. Standing from the canonical pass: evidence forensics permanently lost (row 39 BLOCKED — the only task in the window that did NOT fully land, by external cause, not agent fault); the reflow hazard is documented-not-fixed (normalized-grep row 45 open); neither release.sh die path has been exercised against a live outage (evidence deferred to the next real release).

## c) NOT STARTED

Nothing owed by the re-dispatch. All follow-ups already rowed in TODO_LIST (rows 36, 42–47 cover the closeouts' promoted items). Zero appends this pass.

## d) TOTALLY FUCKED UP

**The dispatch itself.** Tenth paid DONE-on-arrival burn on this id. Root cause unchanged per the canonical report: the live pool runs the pre-guard cmd/tq binary, so the mint-time done-check never fires; the cited commit hashes are daemon-rewritten dangling objects, which defeats ref-based provenance checks. Zero destructive change this pass.

## e) WHAT WE SHOULD IMPROVE

Carried, unchanged from passes 6–14: (1) cmd/tq release + pool redeploy ends this burn class; (2) mint-side footer-grep done-check; (3) object-based (not ref-based) provenance done-checks; (4) a tombstone after ≥2 DONE-on-arrival passes — ten passes deep, grossly overdue.

## f) UP TO 50 NEXT THINGS

**Zero appended.** Every candidate dedup-matches an existing TODO_LIST row (35, 37, 38, 39, 40 carry the five window tasks; 42–47 carry the follow-ups; 36/106/123 carry the pool/deploy items). Re-appending would mint duplicate paid dispatches — the exact loop the 2026-09-11 75-commit review documented. The highest-value real work remains: rows 42–47 (release-path hardening), 36 (`tq pool-health`), and the deploy row 123 (production cannot exercise any of this until the SystemNix input flip).

## g) QUESTIONS

**None new.** The live BLOCKED set stands in TODO_LIST: sub-tag `.info` poke scope (00-55 §g1), bash -n guard end-state (01-02 §g1), dead-evidence re-anchor edit class (01-47 §g2), and the 00-45 scheduling trio.

## h) BAND DRIFT

**None recorded.** Reachable journal `./tasks.db` (189 facts, queried via `tq facts --db tasks.db` this run) holds ZERO `task.reprioritized` facts; the production pool journal remains unreachable ($TQ_DB unset this session, `tq facts` errors) — same vantage gap as passes 6–14.

---

_Point-in-time snapshot 2026-10-09 02:18. Tenth re-dispatch of `000001a11d8b7eeb8f33667e4ed600000000`; canonical report: `2026-10-09_00-40_release-hardening-done-prompt-window.md`. 0 TODO appends. Nothing pushed._
