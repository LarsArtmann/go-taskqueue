# Done-prompt status report — 2026-10-07 00:40 CEST (fourth pass)

**Window:** the same five completed agent tasks as the 02-02, 02-35, and 02-40 done-prompt passes (2026-10-05 ~04:10 → 2026-10-06 ~01:00): archive-evidence negation carve-out, check-verify in-situ flake-heal pin, backend-tag v0.2.0 verification, Dependabot full-tree coverage, review-fix receipt 17→16.
**Method:** all five closeout reports read FIRST (04-10, 04-55, 00-23, 00-44, 00-59), then re-verified against the live tree at HEAD `664f2340` (build rc=0; check-todo-list rc=0; check-dead-sha-refs rc=0, 263 baselined / 0 new; check-status-index rc=0; check-doc-refs rc=0) and the journal. **This is the FOURTH dispatch over the same window** — the strongest runtime vindication yet of the done-preflight gate that landed mid-pass (§e1); this pass adds no new claims about the window's work beyond what the three prior passes established, and its value is (1) fresh HEAD receipts, (2) the post-window delta inventory (§e), and (3) two evidence-backed TODO ticks (§f).

## a) FULLY DONE (re-verified at HEAD this pass)

1. **archive-evidence gitignore-negation carve-out** (commit `82f1415c`): Step 1 treats a leading `!` `check-ignore` pattern as deliberate content (ok-line + proceed); positive-rule hits still refuse as ghost-risk. Regression pinned as the smoke's 11th check. Unchanged since the 02-40 pass.
2. **check-verify.sh in-situ flake-heal pin** (commit `d76c6437`): pin 5 runs `scripts/verify.sh` through a stamp-file PATH-shim flake-heal, fail-closed by construction. The headline misreceipt ("17 ok") was corrected to 16 by the review-fix task; the corrected count matches the report's own 12+1+3 decomposition.
3. **Backend-tag v0.2.0 verification** (commits `dde3ae78` + healed `22e8c50f`): per-module proxy check green, remote annotated tags confirmed, in-namespace clean-room consumer compile green; literal `go install <libmodule>@<tag>` proven impossible-by-design (facade go.mods carry relative replaces — toolchain refusal everywhere), so the consumer path is the canonical proof. The bonus repair of the ~26h-red `internal/e2e` (`narrowCounts` retype) remains landed.
4. **Dependabot full-tree coverage** (commits `9badd3bd` + `f34c63ac`): phantom `cqrsqlite` entry replaced with the three missing post-S2 modules; 23 gomod entries == 23 tracked module dirs. The pre-existing doc-refs red was repaired in passing (re-verified green this pass). `--dep-sweep` retirement correctly parked behind the O15 week-1 gate (earliest 2026-10-13; TODO row 26).
5. **Review-fix receipt 17→16** (commit `cfd09fe4`): corrected at both named sites plus two same-receipt re-quotes; ground truth established mechanically via `grep -c '^ok:'`.
6. **Fresh HEAD receipts this pass:** root `go build ./...` **rc=0** (the vendor-drift red from the 02-02/02-35/02-40 passes is HEALED — `vendor/` is now absent at HEAD per the row-249 trash policy, so root builds resolve from the module cache); `check-dead-sha-refs` **GREEN** (263 baselined, 0 new); `check-todo-list` **GREEN**; `check-status-index` **GREEN** with the standing INDEX BLOAT WARNING now at **233 live rows** vs the 100 threshold (rowed, 509); `check-doc-refs` **GREEN**.

## b) PARTIALLY DONE (unchanged from the 02-40 pass, reconfirmed)

1. Dependabot activation is config-only and unpushed — the O15 week-1 clock has not started (rowed BLOCKED, 513).
2. Pure-graph v0.2.0 installability proof open (row 499); the mixed-graph hazard remains INFERRED and its no-op re-verification loop consumed four windows before the done-preflight gate landed.
3. Receipts-conflation class patched per-instance, not mechanized (canonical receipt rule 503, summary-line re-prefix 504 — unlanded).
4. archive-evidence field-proof + `--help` carve-out line (521); verify-wrapper genuine-red in-situ pin (100); KNOWN_FLAKY grep gate (99) — all rowed, all open.
5. The full quiet-host ci-local skip streak (six-plus prose-ruling skips) is unretired (rowed, 510).
6. Row 44 (widen dead-sha scan to `docs/status/tasks/`) is still open and NOW LOAD-BEARING: closeout reports have moved to `docs/status/tasks/` (O7 routing), and the gate's glob demonstrably does not cover that directory yet (verified: no `tasks` reference in `scripts/check-dead-sha-refs.sh`).

## c) NOT STARTED (carried backlog the window skipped — verified still open)

`.tq-verify` → `scripts/verify.sh` flip cluster (owner-BLOCKED); status-index archival sweep/digest (rowed 509, worst on record at 233); daemon post-sweep cheap doc gate (owner-BLOCKED, 515); consumer-path release proof wiring (511); VERSION-SURFACES.md tag-divergence note (523); per-tag proxy mode (524); correction-vs-annotate doctrine (514, owner); Dependabot settings + nominal-pin ignore ruling (513, owner); arrow-escape tightening (520); new-module.sh dependabot emission (506).

## d) TOTALLY FUCKED UP (the window's honest ledger + current reds)

1. **The window's one genuine self-inflicted wound stays the self-certified wrong receipt** ("17 ok" shipped with its own contradicting 12+1+3=16 decomposition in the same sentence, caught by the reviewer, not the window). The mechanizing rows (503/504) are still unlanded — the class remains one transcription away from re-firing.
2. **The window family burned FOUR full done-prompt dispatches on one five-task window** (02-02, 02-35, 02-40, this report): each mints a report, an index row (now 233 live vs 100 threshold — every dispatch worsens the bloat the reports themselves confess), and daemon-fold risk while adding near-zero new information.
3. **Daemon sweeps keep bypassing all hooks** (the ~26h red e2e, the ~18h red doc-refs, footer-less folds healed by later windows) — the post-sweep cheap gate stays owner-BLOCKED (515) and each new window pays the tax again.
4. No new reds at HEAD this pass: every cheap gate green, build green, tree clean.

## e) WHAT WE SHOULD IMPROVE + the post-window delta this pass observed

1. **The re-fire class got its mechanical fix mid-pass:** commit `f716c68b` (2026-10-06 23:57, "claim-time done-preflight gate — kill the re-fire class with zero agent spend") landed the O4 remedy as a claim-time DonePreflight (footer commits / ticked-or-gone todo / closeout present / fix superseded-or-cured → completes at claim, no attempt burn, no review), journal-proven root cause (one task claimed 15× over its own 21 footer commits). This pass is the gate's first live customer: **these §f ticks and this report's existence should be its last no-op lap.** Rows 145 and 526 ticked on this evidence (§f).
2. **O7 report routing landed** (`1c60ca59` + `664f2340`): closeout reports now live under `docs/status/tasks/`, but the dead-sha gate does not scan that directory yet — row 44 went from hygiene to correctness gap (b6).
3. **v0.3.2 recovery session** (13-13 report) landed the readmodel/projectionhost slice and lint repairs after this window — no window work was invalidated; the vendor-drift red the three prior passes rowed as 511 is resolved in the new vendor-absent equilibrium (row 249 documents the standing policy question; 511 is now stale-by-outcome but its heal content — "run go mod vendor" — was NOT executed, the tree moved instead: **left unticked**, the row's prescribed action never happened).
4. Index bloat crossed 229→233 across the four passes — the digest/archive sweep (509) compounds in value with every dispatch.
5. The canonical-receipt rule (503) remains the cheapest unlanded lever in the backlog; every report this window family produced had to re-derive receipt discipline by hand.

## f) NEXT THINGS + TODO writes

**Two evidence-backed ticks (the only TODO_LIST edits this pass):**

- Row 145 (queue-side dedup-key→COMPLETED short-circuit): landed in claim-time form by `f716c68b` (DonePreflight completes landed-work dispatches zero-cost at claim — broader than dedup-key alone).
- Row 526 (mint-time done-check for repeat dispatches): landed as claim-time done-preflight by `f716c68b` (refuse-at-mint realized as zero-spend claim-time completion with no burn; the row's intent — stop repeat paid dispatches — is mechanized).

**No new rows appended.** Every candidate this pass generated is already rowed by the 02-02/02-35 harvests (502–525) — the dedup rule forbids reworded duplicates, and this fourth pass found no genuinely new work item. The done-preflight gate exists precisely to make future passes of this shape free.

## g) QUESTIONS (owner — all carried, none new)

1. Is the repo-level "Dependabot version updates" setting enabled, and should Dependabot `ignore` the nominal `internal/*` pins (ADR-0017)? (rowed BLOCKED, 513)
2. Correction-vs-annotate doctrine for misreceipts, and sibling-claim scope for fix findings. (rowed BLOCKED, 514)
3. Is a daemon post-sweep cheap doc/config gate acceptable policy on shared daemon infra? (rowed BLOCKED, 515)

## h) BAND DRIFT (ADR-0015 accountability)

**None recorded.** `tq facts --type task.reprioritized` returns `(0 facts)`: the journal holds zero priority-move facts, for this window or any other. Claim order this window was governed by stored priority + aging only (ADR-0015 ladder). The standing observation from the prior passes remains: ADR-0015's reprioritize machinery exists on the queue surface but has never emitted a fact; if the ladder is meant to be auditable, fact emission itself is the gap.

## Docs-health pass notes (this run)

- Living-docs reconciliation at HEAD: CHANGELOG carries the window's user-visible entries (negation carve-out, verify-wrapper + in-situ pin, Dependabot 23-module coverage) — verified by the 02-40 pass, spot-checked this pass, no drift found. FEATURES row-86 (`--dep-sweep`) already de-staled. README/ROADMAP carry no window-touching verifiable claims. AGENTS.md is size-guarded; the additive conventions it needs (canonical receipts 503, walk-away gate 525) stay rowed.
- Archive sweep: NOT executed this pass. The 2026-10-05/06 cluster is fresh and referenced by this report; the backlog cluster the sweep would target (rows 485/486/509) is already rowed with the standing bloat warning at 233. Annotating/archiving 233 rows inside a fourth no-op dispatch would repeat the exact burn the pass confesses — it belongs to the dedicated sweep row.
- Point-in-time reports: no 2026-* report was rewritten; this pass's inline observations live here only.
