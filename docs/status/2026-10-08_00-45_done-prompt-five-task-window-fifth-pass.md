# Done-prompt — five-task window, fifth pass (2026-10-08 00-45)

Date: 2026-10-08 00:43 CEST · Repo: go-taskqueue · Window: five completed
tasks (2026-10-06 01-18 → 2026-10-08 00-32) · Verified against HEAD
`a24472c5` (tree clean at report time).

## The window's tasks (all five verified closed)

| Task | Subject | Work commit | Closeout report |
| ---- | ------- | ----------- | --------------- |
| 000001a10e4940564abd8369113400000000 | Review-fix: install-proof misattribution corrected in the 00-23 report | `f82f9b61` | docs/status/2026-10-06_01-18_task-….md |
| 000001a10e9ba6313abc0d9a3d1200000000 | O7: closeout reports move to `docs/status/tasks/` | `578a16c7`/`664f2340` (re-verified DONE-on-arrival) | docs/status/tasks/2026-10-07_01-37_task-….md |
| 000001a113973f383d2f30ec2b9500000000 | Widen `check-dead-sha-refs` to `docs/status/tasks/*.md` | `f78f2966` | docs/status/2026-10-07_01-47_task-….md |
| 000001a113e5f4f4cd460540688600000000 | HARD CAP 10 TODO-appends per closeout | `a48b25d3` | docs/status/tasks/2026-10-07_03-27_task-….md |
| 000001a11493e81a577bde2bfdfa00000000 | cqrs-lint A014/D013/V006 branch in go-cqrs-lite | `c1fcca41` (re-dispatch; work `d543c4c3`+`3ed2949e`, branch `b4f29e517`) | docs/status/tasks/2026-10-08_00-32_task-….md |

## a) FULLY DONE

1. **O7 report-placement flip verified first-hand at HEAD**: the closeout
   prompt (`executor.DefaultCloseoutPrompt`, internal/executor/agent.go:890)
   names `docs/status/tasks/<ts>_task-<id>.md`; docs/status/README.md:15
   documents the routing (root = window reports, tasks/ exempt from the
   index gate by routing); the done-preflight and status sweeper resolve
   both locations. TODO row 49 ticked with the DONE note.
2. **Dead-sha scope widened and load-bearing on day one**
   (scripts/check-dead-sha-refs.sh:231 globs `docs/status/tasks/*.md`):
   the first widened run caught exactly the predicted escape class — the
   O7 closeout's two healed-sweep cites, converted to sanctioned fork
   records. Gate green at HEAD (300 baselined, 0 new, re-run this pass).
   TODO row 50 ticked.
3. **TODO-append cap shipped and pinned**: the closeout prompt now carries
   HARD CAP 10 + the mandatory dedup-check wording
   (internal/executor/agent.go:892), pinned by
   `TestDefaultCloseoutPromptTodoAppendCap`; TODO row 51 ticked with the
   battery citations.
4. **Review-fix landed cleanly**: `f82f9b61` is single-file, one footer,
   and its finding-sweep went beyond the echo list (five rot sites
   corrected, not three) — see the 01-18 closeout §a for the per-layer
   claim table (go.mod SHA evidence primary, toolchain mechanism via
   reviewer repro, sandbox demoted to secondary).
5. **cqrs-lint branch staged for owner review**: `cqrs-lint/a014-d013-review`
   (tip `b4f29e517`, 8 files) drops the stale A014 entry (pinned),
   makes D013 default-aware, documents V006 as already-fixed upstream
   2026-10-03; suite rc=0 at the branch, tq seam advisory gate clean.
   TODO row 83 ticked; merge is the owner's call. The DONE-on-arrival
   re-dispatch window burned only two tool calls before recognizing
   closure — the previous closeout's report made recognition cheap.
6. **Foreign red found by the cap window is already cured**:
   `TestFactsCQRSOverStore` (cmd/tq) failed against go-cqrs-lite event
   v4.13.1's `StreamMarker:` StreamID prefix; fixed in `e28548a4`
   ("Fix cmd/tq cqrs test for StreamMarker display prefix") — the cmd/tq
   module is not red on that class anymore.
7. **Doc gates green at HEAD** (re-run in this pass): check-status-index
   ok (the 00-40 unindexed-report finding from the O7 closeout §b1 is
   resolved — the row exists now), check-todo-list ok, check-dead-sha-refs
   ok (300 baselined, 0 new).

## b) PARTIALLY DONE

1. **The live dogfood pool still runs the pre-O7 prompt binary** (cmd/tq
   pinned at the old release): both shipped prompt changes (tasks/ path +
   HARD CAP 10) reach agents only at the next cmd/tq release. Every
   closeout since has written a routing note explaining the stale path —
   harmless but unpaid-for friction until the release lands.
2. **cmd/tq gosec coverage** and other pre-existing blocked rows are
   untouched by this window (correctly — no overlap).
3. **Index bloat worsened**: 247 live index rows at this pass (233 at the
   O7 closeout, threshold 100). No archive sweep ran in this window; the
   next sweep targets (row 108) are the three remaining 09-15 reports and
   then 09-16..09-25.

## c) NOT STARTED

1. The rulings-sheet appendix with the branch ref (TODO row: "post the
   branch ref in the rulings-sheet follow-up" residue, rowed separately)
   — deliberately left for a fresh dispatch per the 00-32 closeout.
2. docs/planning stale "three scoped fixes" phrasing sweep; RULES.md
   hand-maintained-vs-generated probe; go-cqrs-lite TODO_LIST provenance
   note (all three rowed at the tail of TODO_LIST.md).
3. Everything downstream of the owner's merge decision on
   `cqrs-lint/a014-d013-review` (tag, pin flip, HARD gate flip — BLOCKED
   rows).
4. Downstream of the O7/cap work: harvest work/batch prompt contracts
   keep the uncapped append wording (owner cap-scope ruling pending).

## d) TOTALLY FUCKED UP (honest)

1. **The DONE-on-arrival re-dispatch class persists**: task
   000001a11493e… was re-minted ~9 minutes after its DONE note committed.
   The re-dispatch window handled it cheaply (two tool calls), but a full
   paid turn still burned. The v0.3.3 mint-time re-dispatch gate ships —
   this window predates its deployment to the live pool (same live-binary
   staleness as b1).
2. **The 01-47 window lost the edit→commit→daemon race again** (fork
   record 41c53cf3→2f247c59; healed, but the heal's FORK-RECORD printout
   is self-identical — the known printout bug, still unfixed and rowed).
3. **Three consecutive windows on the O7-verification task**
   (01-13/01-16/01-35/01-37 all re-verified the same landed change) — the
   queue paid ~4 windows for one landed change before the mint gate could
   refuse. Attribution is per-window honest, but the pacing was waste.
4. **The cap window's source delta rode a mixed footer-less daemon fold**
   (3587eb9a = executor files + foreign readmodel go.mod/go.sum); the
   row-165 heal was correctly REFUSED (not exactly-mine) — so the cap's
   source change is attributed only via annotation, not git. Ninth
   datapoint for the footer-first rule row.
5. **No regression introduced by this window**: no gate went red from the
   window's deltas; the cmd/tq red was foreign and is fixed (a6).

## e) WHAT WE SHOULD IMPROVE

1. **Deploy the shipped fixes to the live pool faster**: the O7 path flip,
   the append cap, and the mint-time done-check are all committed but
   invisible to live agents until the next cmd/tq release + pool restart.
   Every week of delay re-mints the routing notes and re-dispatch classes
   the fixes exist to kill.
2. **Fold the rulings-sheet citation step into the parent task's contract**
   (00-32 e2): splitting "post the branch ref" from the work step spawned
   a follow-up row for work the parent promised.
3. **Cross-repo claim rot**: V006 sat stale 9 days after the upstream fix;
   a periodic re-verification sweep over sibling-repo claims is rowed —
   keep it cheap (grep + HEAD check, not full audits).
4. **dead-sha baseline growth is invisible** (263→300 between passes,
   only `TQ_DEAD_SHA_VERBOSE=1` shows it); a growth signal or periodic
   `--emit-baseline` cadence needs an owner ruling (baseline curation
   stays human per O16).
5. **Warm-cache verify-gate retries** would stop the re-dispatch storms
   at attempt 2 instead of 4 (01-37 e2, three+ windows burned on one ID).

## f) UP TO 50 NEXT THINGS (top picks; rowed ones not repeated)

1. Cut the next cmd/tq release so the tasks/ closeout path + HARD CAP 10
   + mint-time re-dispatch gate reach the live pool (b1/e1) — highest
   leverage, unblocks three fix classes at once.
2. One-shot audit: scan ALL `docs/status/tasks/*.md` for dead commit-SHA
   cites that are neither arrow-escaped nor baselined (01-47 f1; confirm
   the O7 report was the only escape instance).
3. Add a `docs/status/tasks/report.md` fixture to the dead-sha self-test
   pinning the widened scope (01-47 e2).
4. Document the status-index ROW FORMAT contract (date column semantics,
   NOTES density cap) so rows stop being assembled by neighbor-matching
   (01-47 d3).
5. Write the rulings-sheet appendix with the branch ref + V006-already-
   fixed fact (rowed).
6. Archive sweep batch 3: the three remaining live 09-15 reports, then
   09-16..09-25 (row 108; live count 247 and rising).
7. De-flake or quarantine `TestBudgetRefusalSubprocess` (rowed).
8. Fix heal-daemon-sweep.sh's self-identical FORK-RECORD printout (rowed).
9. Pool-restart visibility log line for the baked closeout path (01-37 f6).
10. After owner merge of `cqrs-lint/a014-d013-review`: tag, flip the
    ci-local/devShell pin, then the HARD gate flip (BLOCKED rows).

Items 5–10 are already rowed and listed here for ranking only.

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **Harvest-side append cap**: should the HARD CAP 10 extend to the
   harvest work/batch prompt contracts (internal/harvest), or do work
   turns stay uncapped? Owner cap-scope ruling.
2. **dead-sha baseline cadence**: 300 baselined hits and growing; is a
   periodic `--emit-baseline` re-curation scheduled anywhere (O16 keeps
   curation human), or does the baseline only ever grow?
3. **Foreign dead-sha sites**: the four foreign `99458a91→22e8c50f` sites (00-44
   report ×3 + README's archived row) — arrow-escape them to green the
   remaining warnings, or baseline them per O16's human-curation rule?

## h) BAND DRIFT

`task.reprioritized` facts in the journal (whole journal, supersets the
window's timespan): **none recorded** — the journal holds zero
reprioritized facts, so no band moves happened in or before this window.
Priority behavior this window was static ladder (markers/aging), which
ADR-0015 requires no move-by-move accounting for.
