# Window report — docs close-out + row-28 wire-pin fast-fail (2026-10-12 12:25 CEST)

Session: continuation of `20261012-c16-park-forensics` after the 11-47
WAIT. Owner directive this window: continue executing (docs pass first,
then row-28), then deliver a full status report and STOP before row-29.

## a) FULLY DONE (and verified)

1. **Docs pass (C16 window close-out)** — commit `9632302b`
   (daemon split my staging across `e315cfab` + `93898602`; healed from
   `1dd03be0`, fork records `e315cfab→48e81fe1`, `93898602→9632302b`):
   - CHANGELOG `[Unreleased]`: **Added** — `tq park` verb (7d ceiling,
     `--release`, pending-only refusal copy), forensics filters
     (`tq tasks --dedup`, `tq audit --journal --task-id`), owner-session
     park policy (`--owner-session-park` default ON, `--owner-session-hold`
     15m, PRE-claim sweep semantics); **Changed** — `tq stats` census flip
     to ONE `Store.ClaimCounts` grouped query.
   - TODO_LIST ticks with DONE notes: **408** (park verb, `9177f28a`,
     §g-ratification flag), **409** (`Store.ClaimDueTypes`, store-surface
     report citation), **410** (ClaimCounts pushdown + `b61db8d0` flip),
     **494** (forensics, `278fbc12`).
   - FEATURES rows for BOTH windows: Queue core gains _Operator parks_,
     _Claim-time executor-type filter_, _Census pushdowns_; Agent pool
     gains _Owner-session park policy_; CLI gains the `tq park` row plus
     `--dedup KEY` in the tasks row and `--task-id` in the audit row.
   - `docs/problems/major-prompting-and-batching.md`: status
     OPEN→**EXECUTED 2026-10-11/12** with the shipped numbers (work
     prompt 1,802→1,206 chars / 282→184 words / distrust 5→2, pinned by
     `TestAgentPromptsOutcomeShaped`; ceremony cuts `1d921835`; batch
     default 3) and the honest remainder (section/day-level goal
     batching + the outcome-join/A-B audits stay open).
   - `check-todo-list.sh` OK after the ticks.
2. **Row-28: api poison-DLQ wire-pin fast-fail** — test edit commit
   `996fa025` (healed daemon sweep `549ba3c4→996fa025`), tick commit
   `4d07b291`. `TestUseReadModelDeadLettersPoisonThroughWiredStore`'s
   Count poll now ALSO reads `readmodel.ReadWorkerFailedMarker(d.DBPath)`
   and fails the moment the death marker lands, naming worker +
   failed-at + "the opts.DeadLetterStore wire is dropped". Deadline
   raised 10s→45s (covers worst-case restart-budget exhaustion); green
   path 0.03s.
   - **Correction to the row's estimate**: the "~100ms" red was wrong.
     Without the store, `handleProcessEventError` returns the corruption
     error and the worker rides FIVE jittered restart backoffs
     (1s→16s, vendor `worker.go` run loop) before `WorkerFailed` stamps
     the marker — measured red at **18.677s** with the precise
     diagnosis (vs the old bare 10s timeout with a vague message). The
     tick note records this so future rows budget the backoff ladder.
3. **Verification** (rc to files, never pipes): mutation check —
   commented `opts.DeadLetterStore = dlq.Store()`, test failed in
   18.677s with `poison fact hit the restart budget instead of the
   sidecar (worker "api-poison" failed at 2026-10-12T10:23:30Z): the
   opts.DeadLetterStore wire is dropped`, mutation restored immediately
   (`git restore`, my own change, verified clean). Green targeted run
   0.03s; full `internal/httpapi` package `-count=1` green 0.149s;
   `go vet ./internal/httpapi/` OK. No cmd/tq or store files touched
   this window — the 11-47 window's gates stand for those.

## b) COMMIT ORDER (this window, post-heal SHAs)

`1dd03be0` (base, prior window) → `48e81fe1` (daemon docs span,
footer-stamped) → `9632302b` (docs close-out) → `996fa025` (row-28
test, healed) → `4d07b291` (row-28 tick) → this report's docs commit.
All carry `Task-Queue-ID: 20261012-c16-park-forensics`.

## c) NOT DONE / OPEN

- **Row-29 (heal-daemon-sweep span-terminator fix)**: untouched — owner
  ordered STOP after the report. The last window's reassessment stands:
  the feared `[footer-less span]+[footer HEAD]` shape healed cleanly 6×
  (2 more this window); the real residual risk is a SIBLING commit
  inside the range (what blocked the SystemNix heal), which refuses by
  design.

## d) ISSUES NOTICED (own work, honest)

- The daemon raced my docs staging again (split a 4-file commit in
  two). Recovery identical to the validated pattern; total heals this
  session lineage now 8, all clean. Reinforces that row-29's danger
  claim is overstated, not that the script is fine.
- Row-28's original timing estimate was fabricated by a prior session
  without reading the vendor restart loop — caught only because I
  mutation-tested before committing. Rows citing platform behavior
  should cite the code path, not intuition.
- Standing: status-index bloat (README grows past 308 rows with this
  report).

## e) WHAT I FORGOT / COULD DO BETTER

- The first FEATURES `tq park` row insert failed on a trailing-space
  exact-match and I nearly shipped without it — the post-edit content
  verification pass caught it. Always count the inserted rows, not just
  the edit success.
- One diagnostic detour: `rg -rn` (replace flag) garbled symbol names
  and briefly pointed at a nonexistent API. Correct flags matter when
  tracing code.

## f) NOT STARTED (next session's queue)

- Row-29 span-terminator fix + shape reproduction (blocked on owner GO
  per this window's stop order).
- CV reds (`TestArchitectureValidation` + `pipeline/handlers` 600s
  hang) — awaiting owner Q3 ownership ruling; do NOT use the shared
  GOMODCACHE without the Q2 trash ruling.
- pg conform run — awaiting a sanctioned `TQ_TEST_POSTGRES` DSN.
- Production pool restart (activates done-preflight + owner-session
  park defaults) — owner-gated.
- B11.1 dismissal-tier extension (126/70/51-claim class).

## g) QUESTIONS FOR THE OWNER (carried, still unanswered)

1. Ratify the shipped park defaults: 7d `--until` ceiling (re-park to
   extend) + shared `not_before` visibility (no operator-park fact join
   in `tq tasks --parked`)? Both remain small CLI follow-ups if
   overturned.
2. Permission to trash the corrupt
   `/mnt/buildcache/go-mod/cache/vcs/3af5bce8…` entry — CV evidence
   proved it is NOT CV's red cause, but it still blocks normal `go`
   runs for CV + templ-components.
3. CV reds ownership: fix here as the C15.1/C15.2 continuation, or mint
   CV-side TODO rows and stay on go-taskqueue tracks?
4. Standing: pool restart timing; B11.1 dismissal-tier; pg DSN.

— end of report; session WAITING.
