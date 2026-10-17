# Status Report — art-dupl -t 3 Audit + Closeout-Flake Root Cause + Journal-Drift Re-check (2026-10-17 18:34 CEST)

> **ARCHIVED 2026-10-17** Per-item triage at HEAD `a6dbf658`: every item
> shipped and re-verified (a1 `companion.Enqueue` homing `3990c6e9`; a2
> RFC3339Nano + set-wise window pins `bf328e28`/`2ed7c221`; a3 drift
> re-verification `18a337e1`), the one forward-looking item routed to its
> open row (b1 → row 321, BLOCKED on the M4 memo), §d/§f/§g narrative.
> Batch 4/4 (the follow-up session) closed in
> docs/status/tasks/2026-10-17_19-29_task-000001a14ade18eb3cd0720b410600000000.md;
> window verification: docs/status/2026-10-17_19-54_done-prompt-five-task-window-ci-parity-depsweep-artdupl.md §a3-§a6.

**Session scope:** autonomous queue batch `000001a1478bb7c81e2c09da2abe00000000`
(TODO_LIST "art-dupl dedup pass follow-ups", 3 items, one session). No
re-dispatch history found (footer grep clean before work started). Concurrent
agents expected; daemon sweeps healed via `scripts/heal-daemon-sweep.sh`.

## a) FULLY DONE

1. **Item 1 — art-dupl `-t 3` rerun (01-34 §f34).** The condition "once the
   accept-list gate exists" is met: `scripts/check-mirror-clones.sh` runs the
   canonical `-t 3 --type-aware` invocation strict in ci-local. The rerun at
   HEAD caught a real regression: the gate was RED — a NEW cross-backend
   mirror (byte-identical `Enqueue` bodies in `internal/queue/sqlitev4/adapter.go`
   and `postgresv4/adapter.go`, missed by the `-t 4` era). Homed ONCE in
   `companion.Enqueue` per the 2026-09-26 dedup ruling (one shared home, no
   acceptance); both adapters now one-line delegates. Remaining 9 groups at
   `-t 3` are same-file micro-clones (conditional/expression/assignment/call
   categories) under art-dupl's actionability filter — nothing actionable.
   Commit 3990c6e9 (post-heal SHA) + TODO row 319 ticked.
2. **Item 2 — `TestSweepPinsCloseoutReportPaths` flake ROOT-CAUSED and fixed
   (02-35 §c1).** NOT the report-dir scan. True root cause, found in two
   steps: (i) window entries sorted by a second-granularity RFC3339
   `CompletedAt` string — fixed with RFC3339Nano; (ii) the `-race` full
   suite STILL reproduced it: the upstream engine persists `updated_at` as
   unix MILLIS, so two completions inside one millisecond are
   order-indistinguishable at the source and the sort falls to the random
   TaskID tiebreak. The window tests (Report paths ×2, labels/detail) now
   pin the report SET and per-task detail by task ID instead of array
   index — the store cannot express true sub-milli order, so index
   assertions were pinning noise. Package tests green 5× at `-race`.
   Commits bf328e28 + fd72e247, row 320 ticked.
3. **Item 3 — journal-drift audit re-verified, upstream gate confirmed absent.**
   Both cmd/tq drift tests PASS at HEAD (via `scripts/test-cmd-tq.sh`, 11.5 s
   — the plain `cd cmd/tq && go test` build breaks on the pinned papdashboard,
   the documented shim is the gate). Vendored go-cqrs-lite still writes
   `task.enqueued` detail `{project,type}` only; the honest-degradation state
   (coverage 0 for never-diffed fields, `Caps.EnqueuedSnapshot=false` in both
   store_test.go files, replay-ready enriched-detail handling) verified in
   place. The root-cause fix remains the UPSTREAM enrich, gated on the M4
   ratification memo — not written, so the row stays open with a BLOCKED
   annotation (commit 18a337e1).

## b) PARTIALLY DONE / GATED

1. Item 3's substance (upstream enqueued-detail enrich carrying
   `queue.EnqueueDetail`'s snapshot keys) cannot land from this repo: it needs
   the M4 ratification memo (owner artifact) plus a go-cqrs-lite release and
   consumer bump. The tq-side contract is already shaped for the flip.

## c) VERIFICATION

- Per-module gates for every touched module: `companion`, `sqlitev4`,
  `postgresv4` build+vet+test green; `internal/status` build+vet+test green
  (flake target 10×); `scripts/test-cmd-tq.sh` green;
  `check-mirror-clones.sh` strict green (0 cross-backend groups).

## d) INCIDENTS

1. Daemon sweep 7745b640 folded item-1's three code files footer-less mid-item;
   healed via `scripts/heal-daemon-sweep.sh --from 8d434737` (fork-records:
   7745b640→8ee9930a, 210ac365→3990c6e9). First heal attempt REFUSED: the
   footer was not the last trailer (attribution lines below it) — amend put
   `Task-Queue-ID:` last, heal passed. Third sighting of the
   footer-vs-attribution placement class (cf. 2026-10-14 03-46 §d).

## f) FOLLOW-UPS

None minted: the only forward item (upstream enrich) is already carried by
the open row itself; a new row describing this batch's work is forbidden.

## g) LESSONS

1. The `Task-Queue-ID:` footer must be the commit message's LAST trailer —
   attribution lines go ABOVE it (hook rejects otherwise; heal tool refuses
   otherwise).
2. Second-granularity timestamps as sort keys are a latent ordering flake
   anywhere entries can complete in the same wall-clock second under load.
