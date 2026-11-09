# Done-Prompt Window — Release-Hardening Sweep, DONE-on-Arrival Re-Dispatch (Sixteenth Pass)

**Date:** 2026-10-09 02:40 CEST
**Window:** the five release-hardening tasks of 2026-10-08 00:07 → 01:53
**Task-Queue-ID:** `000001a11d8b7eeb8f33667e4ed600000000` — the SAME id already closed by the canonical sixth pass (`2026-10-09_00-40_release-hardening-done-prompt-window.md`) and re-dispatched ELEVEN times since (00-52 seventh through the 02-18 fifteenth pass). DONE-on-arrival short re-dispatch record.

## a) FULLY DONE — re-verified in-tree first-hand this run

- **Demand-fill poke** (task …8221472e): `scripts/release.sh:170-192` — every attempt pokes `proxy.golang.org/$MODULE/@v/$VERSION.info` and echoes `poked demand-fill: …` before the `@v/list` poll. TODO row 35 `[x]`.
- **5/5 die split** (task …8fdb517f): `release.sh:188-191` — `info_poke_ok=true` dies proxy-lag ("never re-tag"), false dies network-dead ("fix connectivity … never re-tag"). TODO row 37 `[x]`.
- **Root-gate bash -n guard** (task …9031c): `scripts/root-gate.sh:21-30` syntax-checks `scripts/release.sh` + `scripts/lib/*.sh` ahead of the Go gates. TODO row 38 `[x]`.
- **Evidence archive** (task …a6bebc): honest BLOCKED — `/tmp/tq-release-push-v033c.log` confirmed GONE from /tmp again this run; TODO row 39 carries `— BLOCKED:` citing only the surviving prose. Nothing fabricated.
- **Reflow-hazard annotation** (task …411c): FALSE-FAIL HAZARD comment at `scripts/check-release-docs.sh:43` above the line-contained `need_in_both` pins. TODO row 40 `[x]`; reflow-tolerance follow-ups already rowed (rows 45–47).

**Prompt-lineage note (unchanged since pass six):** the dispatch cites `a24472c5…`, `7c7a9063…`, `9ea63fd9…`, `c46151a0…` — unreachable pre-rewrite lineage objects; master carries the content-twins (`4e6d7f88`, `8fed04f3`, `bb280c05`, `a27edabe`, `1075248c`). The work is fully landed; only the cited hashes dangle.

## b) PARTIALLY DONE

Nothing new. Standing: row 39 permanently BLOCKED (forensic-raw evidence lost by /tmp hygiene — external cause); reflow hazard documented-not-fixed (normalized-grep row 45 open); neither die path exercised against a live outage yet.

## c) NOT STARTED

Nothing owed by the re-dispatch. Follow-ups already rowed (rows 42–47 + the deploy/pool-health rows). Zero appends this pass.

## d) TOTALLY FUCKED UP

**The dispatch itself.** Eleventh paid DONE-on-arrival burn on this id (~2 h since the fifteenth pass). Root cause unchanged: the live pool runs the pre-guard cmd/tq binary, so the mint-time done-check never fires; the prompt's hash capture records unreachable lineage. Zero destructive change this pass.

## e) WHAT WE SHOULD IMPROVE

Carried, unchanged from passes 6–15: (1) cmd/tq release + pool redeploy ends this burn class; (2) mint-side footer-grep done-check; (3) object-based (not ref-based) provenance done-checks; (4) a tombstone after ≥2 DONE-on-arrival passes — eleven passes deep, grossly overdue.

## f) UP TO 50 NEXT THINGS

**Zero appended.** Every candidate dedup-matches an existing TODO_LIST row (35, 37, 38, 39, 40 carry the five window tasks; 42–47 the follow-ups; the pool/deploy rows carry the release items). Re-appending would mint duplicate paid dispatches. Highest-value real work remains: rows 42–47 (release-path hardening), `tq pool-health`, and the production deploy row (the pool cannot exercise any of this until the cmd/tq release + SystemNix flip).

## g) QUESTIONS

**None new.** The live BLOCKED set stands in TODO_LIST: sub-tag `.info` poke scope (00-38 §g1), bash -n guard end-state (01-02 §g1), dead-evidence re-anchor edit class (01-47 §g2), and the 00-45 scheduling trio.

## h) BAND DRIFT

**None recorded.** The reachable journal (`./tasks.db`, queried via `tq facts` this run) holds ZERO `task.reprioritized` facts; the production pool journal remains unreachable ($TQ_DB unset this session) — same vantage gap as passes 6–15.

---

_Point-in-time snapshot 2026-10-09 02:40. Eleventh re-dispatch of `000001a11d8b7eeb8f33667e4ed600000000`; canonical report: `2026-10-09_00-40_release-hardening-done-prompt-window.md`. 0 TODO appends. Nothing pushed._
