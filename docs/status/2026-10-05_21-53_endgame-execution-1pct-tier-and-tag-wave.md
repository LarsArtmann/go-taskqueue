# Session status: endgame execution — 1% tier landed, release gate at the lint line

Task-Queue-ID: none (owner-directed session, 2026-10-05 ~20:35–21:53 CEST)

## a) FULLY DONE (gates green on the committed content)

- **M02 — pushdown stats counters** (commit a45d9f08, part daemon-swept in
  33a76ac9): `readmodel.Model.Stats/StatusCounts/ProjectCounts` serve
  by-status and per-project×status counts as metaengine `GroupedCount`
  SQL GROUP BY pushdowns over the planned tasks table — O(groups), zero
  rows loaded, exact by construction. **Design deviation from the plan,
  documented:** the cookbook's stateless Delta-counter recipe was
  rejected with evidence — `RescueDead` re-enqueues from dead and
  `DismissDead` cancels from dead (verified in the upstream queue
  facts/finalizeReclaim), so no statically knowable from-status exists;
  a Delta counter provably diverges. `tq stats` (both paths),
  `/api/v1/stats`, and the webui ride the counters; `tallyStats` +
  `tallyModelRows` deleted; store escape hatch reads the store's own
  GROUP BY surfaces and narrows in Go. Pinned by
  `TestStatsParityLifecycle` (lifecycle incl. rescue + dismiss + filters
  + store parity). Gates: readmodel module (build/vet/test/-race), root
  build/vet/test -race, cmd/tq gate — all green.
- **M03 — durable readmodel cursor** (573cff3f + sweeps 5bf66469/56d10d4f):
  `WithDurableCursor` checkpoints into the `watermarks` table after
  every applied batch (consumer `readmodel`), resumes on open, and an
  empty projection under a nonzero checkpoint replays from zero once
  (delete-the-file escape hatch preserved). Checkpoint failures log +
  retry next batch (a missed checkpoint costs replay, never a skipped
  fact — documented divergence from the sweeper gating rule). Wired into
  serve/api/stats. Pinned by `TestDurableCursorSkipsReplay`. Ends the
  measured 9,523-fact restart replay.
- **M04 — cqrsqlite deleted** (swept as c8755d16): module `git rm`'d,
  conform/doc comment wiring cleaned, mirror-clone allowlist entry
  removed, AGENTS references updated, TODO row closed. All 21 modules +
  root build/vet/test green after.
- **M06 — dogfood cutover assist** (67164cec): verified LIVE (read-only):
  serve+pool on the 0.3.1 nix binary since 19:41, stats parity exact
  (805 = 1/461/319/24, matches the deep-dive), zero legacy snapshots.
  Cutover + rollback runbook landed at
  `docs/release/2026-10-05_CUTOVER-RUNBOOK.md`.
- **M05 (immutable half)** — the five missing internal tags
  (`internal/{composition,queue/companion,queue/postgresv4,queue/sqlitev4,readmodel}/v0.3.0`)
  CUT (annotated, house style), PUSHED, and **proxy-verified**
  (`go list -m -versions` from a neutral dir resolves all five).
  `check-go-mods.sh` pending-tag bridge deleted (a7144563 sweep);
  release-gates smoke ALL GREEN; facade parity green against real tags.
- **M07 — docs truth pass** (874e1934): ADR-0019 endgame addendum (S1
  done incl. auto-upgrade; S2 = the single remaining stage; S3
  load-bearing; S4 done, empty-DomainConfig deviation justified; P5
  started; tag wave complete), FEATURES platform section (6 rows),
  CHANGELOG endgame bullets (Added ×3 + Removed), TODO_LIST **301 → 297
  open rows** (S3/S4/cutover/cqrsqlite closed).
- **M10 (partial but real)**: red-master root causes were (1) missing
  internal tags — FIXED by the wave above; (2) flake.nix `outputs =
  let…in` thunk breakage — fixed by a concurrent agent (I reproduced +
  minimal-flake-verified the mechanism); (3) vendorHash drift — now
  verifies; (4) AGENTS size-guard breaches — fixed twice (compact-table
  restore c917b8f3, then the documented fourth conscious budget reset
  15,700→16,400 + shell-twin sync 7876754f); (5) harvest
  `TestSelfManagingLoop` CI flake — unreproducible in 19 local -race
  runs (incl. GOMAXPROCS=2 whole-package ×4), instrumented with a
  full-Result dump (be9bead4 sweep).

## b) PARTIALLY DONE

- **M05 — root tag v0.3.1 NOT yet cut.** `release.sh v0.3.1` ran three
  times; runs 1–2 failed on the AGENTS size guard (fixed mid-flight),
  run 3 failed the **lint-baseline gate**: 5 advisory findings in files
  I authored (gocritic appendAssign + varnamelen in stats.go; wrapcheck
  in model.go loadCursor; embeddedstructfieldcheck + unused nolint +
  gocyclo ×2 in parity_test.go). At stop: stats.go (slices.Concat +
  rename), model.go (wrap), countingStore (embed separation + nolint
  removal) fixes were applied and daemon-swept UNVERIFIED; the two
  gocyclo test splits (22/24 > 20) are NOT yet done. Next gate run will
  tell.
- **M10**: master CI goes green only after the release gates pass +
  push (the queued CI run on a7144563-era master still carries the old
  reds until then).
- **M11/M12**: researched only. M11's plan rows (119/149/168/344/447)
  turned out to describe different work than the F046–F050 names —
  needs the 09-15/10-15 report §sections to disambiguate the real
  mint/dispatch code sites before editing.

## c) NOT STARTED (from the 27-task plan; unblocked unless noted)

M08 projectionhost+DLQ (dep: M03 ✓); M09 health visibility (dep: M08);
M13 S2 vocabulary flip (dep: M04 ✓ — the last S-stage); M14–M25 (20%
tier, all independent); M26 upstream filings (post-auth); M27
docs-health + long-tail burn (dep: M07 ✓ — the ~180-row tail, now
297-row TODO_LIST); F001 owner decision pack is MOOT for most items —
this session's owner directive authorized the wave; F004/F008 remain
genuine upstream design questions.

## d) TOTALLY FUCKED UP / mishandled

- **Two edit attempts bounced off the stale-file guard** (stats.go,
  model.go) because the daemon touched the files between my read and my
  edit; I lost two round-trips and then applied the fixes without a
  fresh verifying run before the sweep took them. Content preserved,
  verification owed.
- **The release-gates run-1 timing**: I deleted the check-go-mods
  pending-tag bridge BEFORE pushing the tags, momentarily making the
  gate hard-red (proxy can't see unpushed tags). Recovered by pushing
  immediately; correct order is push-then-delete.
- **`rg -r` confusion**: my own grep flags (`-r` = replace) mangled
  output into phantom "ln" renames and sent me on a 10-minute
  concurrent-agent-collision wild goose chase that wasn't there.
- Daemon attribution: roughly half my session's commits landed as
  footer-less `chore: auto-commit` sweeps (33a76ac9, 5bf66469, 56d10d4f,
  c8755d16, be9bead4, 4fd42f50…). Content always verified intact after;
  attribution loss accepted per precedent; the baseline will need one
  grandfathering pass at the next ci-local.

## e) WHAT WE SHOULD IMPROVE (session lessons)

1. Commit BEFORE long gate runs, immediately after edits — every
   attribution loss this session was a >60s gap between edit and commit.
2. The lint baseline should run per-module DURING development (I ran
   golangci only at release time and paid a gate cycle).
3. AGENTS.md is a shared resource under a hard byte budget with FOUR
   resets in 26 days — the F034 platform rewrite (shrink Architecture/
   Known Issues, migrate lesson detail to references/) is now overdue,
   or the budget will keep ratcheting.
4. The plan's F-row "Covers" references drifted from the actual TODO
   rows (M11) — future plan writes should cite row numbers + opening
   words, not just numbers.

## f) NEXT (top ~20, priority order)

1. Finish the 2 gocyclo test splits (parity_test) + verify all 5 lint
   fixes → lint-baseline green.
2. Re-run `release.sh v0.3.1` → `--tag` → `--push` → proxy + cleanroom
   `go install` verify (completes M05, F019–F026).
3. Push master; watch the CI run go green (closes M10).
4. Update AGENTS STATUS line + `docs/release/RELEASE.md` for v0.3.1
   (F026 residue).
5. Grandfather the session's daemon sweeps in the attribution baseline
   (one pass).
6. M08: projectionhost skeleton over FactJournal (F035) → watermarks
   CheckpointStore adapter (F036) → DLQ + options (F037) → pump shrinks
   to Handle (F038) → poison-fact DLQ test (F039) → lag gauge (F040).
7. M09: doctor projection section (F041) → HealthCheckDetailed into
   /health (F042–F043).
8. M13: S2 flip — Fact alias (F058), Detail sweep 79 sites (F059–F060),
   companion scanFacts (F061), memory journal + cqrs re-point (F062),
   facade parity + gates (F063–F064). Closes the LAST ADR-0019 stage.
9. M11/M12 disambiguation from the 09-15/10-15 reports, then execute.
10. M14–M25 as scheduled (evidence hardening, spend caps, batching,
    worker safety, security pins, audit drill-downs, doctor liveness,
    ci.yml parity, webui surfaces, SQLITE_BUSY class).
11. M27 long-tail burn: 297 → 0 in standing batches (F122–F149).
12. M26 upstream filings (post-auth): F004 seam, F008 consumer fate,
    ratification memo (F116).

## g) QUESTIONS ONLY THE OWNER CAN ANSWER

1. **S2 vocabulary flip (M13) tonight or next window?** It sweeps 79
   `Detail` sites across every module while concurrent agents keep
   landing code — high collision risk. Recommend: I do it in ONE
   dedicated window with a repo-wide edit freeze ask, not interleaved.
2. **F008 consumer fate:** `internal/consumer` (ADR-0009 dispatcher) is
   now a ghost — no live subscribers since the S3 flip; delete it
   (P5-class) or keep it as the documented integration seam? The
   deep-dive called it BLOCKED on this ruling.
3. **The durable cursor + counters rebuild the dogfood's service
   binary only on the next nix/deploy cycle — restart `tq serve` now to
   pick up v0.3.1-with-M03 (ends the 4.2 MB WAL replay churn tonight),
   or let it ride until you next rebuild?** (Restart is operator-run per
   O12; I did not touch the service.)

— Session stopped per owner directive at 21:53; tree clean except
daemon-swept lint fixes awaiting their verifying run.
