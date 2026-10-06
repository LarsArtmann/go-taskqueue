# Done-prompt status report — 2026-10-06 02:35 CEST (second pass)

**Window:** five completed agent tasks, 2026-10-05 ~04:10 → 2026-10-06 ~01:00 (deliverable spans) plus the post-window overnight trail (01:05–02:23).
**Method:** all five closeout reports read FIRST, then verified against git log/show, live gate runs at HEAD, and the journal (`tq facts --type task.reprioritized`). A prior done-prompt report for this same window exists (`docs/status/2026-10-06_02-02_done-prompt-five-task-window-negation-pin-backendtags-dependabot-receiptfix.md`); this pass is the follow-up verification: what the 02-02 run changed since, what its own edits landed as, and current HEAD state.

## a) FULLY DONE (verified, not just claimed)

1. **archive-evidence gitignore-negation carve-out** (commit `82f1415c`, task …c3c7f3): Step 1 of `scripts/archive-evidence.sh` parses `git check-ignore -v` output and treats a leading `!` pattern as deliberate content (ok-line + proceed); only positive-rule hits refuse as ghost-risk. Root cause reproduced in a scratch repo BEFORE the fix; regression pinned as the smoke's 11th check; CHANGELOG Fixed bullet present; TODO row closed. Verified via `git show` + closeout §a + the 02-02 pass's re-check.
2. **check-verify.sh in-situ flake-heal pin** (commit `d76c6437`, task …bdd601): pin 5 executes `bash scripts/verify.sh` itself through a scripted flake-heal (stamp-file PATH shim: first `go` invocation fails with the known-flaky signature, later ones pass; shimmed `gofmt` keeps it hermetic and sub-second). Fail-closed by construction. The window's own headline receipt was wrong ("17 ok") and was corrected to **16** by the review-fix task below; re-verified this pass (`grep -c '^ok:'` on a fresh captured run = 16, rc=0).
3. **Backend-tag v0.2.0 release verification** (commits `dde3ae78` + healed `22e8c50f`, task …cf6100): per-module proxy check green (`internal/queue/{sqlite,postgres}` list v0.2.0+v0.3.0); remote annotated tags confirmed (both peeling to `0637d633`); clean-room in-namespace consumer compile green (sumdb-verified `go get`, tidy closes the graph, full store surface builds, GOWORK=off). Literal `go install <libmodule>@<tag>` proven impossible-by-design (facade go.mods carry relative replaces — toolchain refusal everywhere, CI included); the consumer path is the canonical proof. Bonus repair: fixed a ~26h-silently-red `internal/e2e` at HEAD (daemon sweep `1a403842` captured a mid-flight `Filter.Status` retype leaving cmd/tq `narrowCounts` on `*string`); `scripts/test-cmd-tq.sh` green, full `-race` battery rc=0.
4. **Dependabot full-tree coverage** (commits `9badd3bd` + `f34c63ac`, task …193000): phantom `cqrsqlite` entry replaced with the three missing post-S2 modules; mechanical cross-check: **23 gomod entries == 23 tracked module dirs, 0 dup/missing/phantom**. Also repaired a pre-existing doc-refs red (FEATURES cited nonexistent root `companion/conform`; the CHANGELOG's historical `internal/queue/cqrsqlite` backtick allow-listed with dated rationale). `--dep-sweep` retirement re-queued as a BLOCKED row (O15 time gate, earliest 2026-10-13).
5. **Review-fix: gate receipt 17 → 16** (commit `cfd09fe4`, task …5b05a0): the rejected work's miscount corrected at both named sites (status-index row + 04-55 report §a3) plus two same-receipt re-quotes (§b3/§d5); ground truth mechanical (`grep -c '^ok:'` = 16, matching the report's own 12+1+3 decomposition); different-metric receipts correctly untouched.
6. **Post-window residue, verified THIS pass at HEAD:** `check-dead-sha-refs` **GREEN** (rc=0, 263 baselined, 0 new — the 02-02 pass completed the fork-record cure for the dangling `99458a91→22e8c50f` citations); `check-todo-list` GREEN; check-doc-refs and check-status-index green per the overnight reports.

## b) PARTIALLY DONE

1. **Dependabot activation is config-only** (00-44 §b2): nothing pushed, repo-level "Dependabot version updates" setting unverified — the O15 week-1 clock (earliest 2026-10-13) has NOT started. Retirement execution correctly parked behind the gate.
2. **Pure-graph v0.2.0 installability proof** (TODO row, "Backend-tag verification window follow-ups"): the mixed-graph hazard (root v0.2.0 = pre-split `845f5f38` vs internal tags = post-split `0637d633`) is INFERRED from a live v0.2.0×v0.3.0 conflict. The row's own proof step is open — and this finding has now consumed FOUR windows: the 01-37 closeout plus THREE no-op re-verifications (01-48, 01-55, 02-21 reports, latest commit `a5fbe18e`), each re-proving the finding already cured at HEAD.
3. **The receipts-conflation class is patched per-instance, not mechanized**: two confirmed instances (02-11, 04-55/00-59) plus a near-miss; the canonical-receipt rule (AGENTS bullet) and summary-line re-prefix remain unlanded (harvested as TODO rows by the 02-02 pass).
4. **archive-evidence field-proof pending** (04-10 §b2): the negation path is pinned in scratch repos only; no real `.log` has ridden the un-ignore rule in this repo yet. The `--help` line documenting the carve-out is also still missing (04-10 §c3).
5. **Verify-wrapper coverage is heal-only in situ**: the genuine-red path (wrapper executed to rc=1, zero retries) is lib-level only; TODO row exists.
6. **ci-local/full-battery skip streak**: the window family ran scoped gates on prose rulings five-to-six times running; no full quiet-host ci-local with an archived matrix log has landed.

## c) NOT STARTED (carried backlog the window skipped — verified still open)

- Root build gate **RED at HEAD for everyone** (verified this pass): `d23f4380` pinned readmodel v0.3.1 in root go.mod without `go mod vendor` — every root build/test dies on "inconsistent vendoring". Rowed (`go mod vendor` + root-gate re-run + cite rc); not fixed here (hard docs scope).
- `.tq-verify` → `scripts/verify.sh` flip cluster (owner-BLOCKED; zero production callers).
- KNOWN_FLAKY ↔ Known Issues grep gate; genuine-red in-situ verify.sh pin (existing rows).
- Status-index archival sweep: index bloat measured at **224 live rows at 00-59**, 229 cited by the 02-02 pass — worst on record vs the 100 threshold; rowed.
- `--dep-sweep` retirement execution (O15 time gate); daemon post-sweep cheap gate (owner call); the consumer-ghost / near-identical-interfaces / postgres-wiring owner trio.

## d) TOTALLY FUCKED UP (verified regressions, debt, broken discipline)

1. **Nothing in the window shipped broken** — all five deliverables verified green, and the one broken-HEAD discovery (red e2e) was repaired in-window.
2. **The 04-55 window shipped a self-certified WRONG receipt as its headline** ("17 ok" contradicting its own 12+1+3=16 decomposition); the reviewer caught it, not the window. Verification-of-the-verification was the hole; the family had written the 02-11 confession one report up and re-offended in the next window.
3. **The 02-02 done-prompt's own edits landed FOOTER-LESS**: the dead-sha fork-record cure, the FEATURES row-86 de-stale, the 02-02 report + index row, and its 16-row TODO harvest all sit in footer-less `chore: auto-commit` daemon sweeps (`2cc73fc9`, `42e0058d`, `d5ca633a`, `b215d3ba`, `4c843e41`, `efb968d7`, 02:00–02:18). The done-prompt loop's own output is currently daemon-folded, unattributed — the exact race two window tasks confessed and paid the heal tax for. Healable (`scripts/heal-daemon-sweep.sh`, unpushed), rowed this pass.
4. **The mixed-graph no-op re-verification loop burned four windows** on one already-cured finding (b2) — the mint-time done-check (row 114, O4-ratified) and the gate-slow/report-exists guard (row 339) exist as rows but have not landed; the loop re-fired anyway.
5. **Daemon sweeps keep minting multi-hour breakage**: `1a403842` left internal/e2e red ~26h; the 10-05 foreign commits left doc-refs red ~18h; the dependabot config itself arrived in an 84-file unverified sweep and drifted within one week; `d23f4380`'s vendor drift has kept every root build red since 01:49.
6. **Process debt repeated and confessed each time**: edit→commit→battery ordering violated twice (00-23 d3, 04-55 — the second produced a daemon race needing a heal); `tail -1` on a 93-hit gate log (04-10 §d1); View-before-edit rejections (04-55 ×3, 00-44 ×2).

## e) WHAT WE SHOULD IMPROVE (concrete)

1. **Canonical receipt rule** (highest leverage): every "N ok / M fail" receipt quoted in a report must come from `grep -c '^ok:'` over a captured gate-output file — never counted in context, never taken from a summary line. One AGENTS.md sentence closes the twice-fired class. (Harvested.)
2. **Kill the trap at the source**: re-prefix check-verify.sh's final summary line so no ok-grep can count it; audit root-gate.sh's extraction and land both in one commit. (Harvested.)
3. **Post-daemon-sweep cheap gate** (doc-refs + todo-list + status-index after footer-less commits, ~1 min): would have caught the 26h and 18h red windows in minutes. Owner call on the hook point. (Harvested.)
4. **Dependabot parity mechanical**: `check-dependabot-coverage.sh` (1:1 yml↔go.mod) in ci-local + `new-module.sh` emitting the entry at creation. (Harvested.)
5. **Attribution of daemon-folded agent output**: this pass found a whole done-prompt run's work unfootered in sweeps (d3). `heal-daemon-sweep.sh` covers unpushed history; `commit-task.sh` sweep-detection (harvested) prevents the next one. Until then, every done-prompt pass should end by checking for its own footer-less folds.
6. **Edit→commit→battery is a daemon race, not a preference**: two violations, two heal taxes in one window. Mechanical enforcement beats confession.
7. **Cross-module edits need a walk-away gate**: root `./...` reaches cmd/tq only via internal/e2e; `scripts/test-cmd-tq.sh` before walking away is the cheap rule the 26h red window bought. (Harvested.)
8. **No-op re-dispatch loop**: three consecutive no-op re-verifications of one finding is the strongest signal yet for the row-114 mint-time done-check and row-160 stop-artifact minting — both already ratified, neither landed. Landing either ends this loop class.
9. **Confession-to-mechanism conversion**: the family confesses faster than it mechanizes. Rule of thumb: a confessed §d item with a one-line mechanical fix gets the fix in the same window or a TODO row with an effort estimate.

## f) NEXT THINGS (deduped against live TODO rows — the 02-02 pass already appended its harvest; genuine-red pin, KNOWN_FLAKY gate, dep-sweep retirement, coverage guard, receipt rule, summary-line re-prefix, heal citation check, commit-task sweep detection, index sweep, quiet ci-local, consumer-path proof, vendor heal, Dependabot settings, correction doctrine, daemon cheap gate, and the pure-graph row are ALL already rowed and are NOT re-listed)

1. Heal the 02-02 done-prompt's footer-less daemon folds (`2cc73fc9`…`efb968d7`, 02:00–02:18) onto its task ID via `scripts/heal-daemon-sweep.sh --self-test` first — restore attribution for the window's own done-prompt output.
2. `check-dead-sha-refs.sh`: tighten the line-wide arrow escape to per-token pair matching — any line containing an `old→new` pattern currently whitewashes EVERY hex token on it (04-10 §e3).
3. archive-evidence `--help`: document the negation carve-out; extend the smoke's `--help` check to assert it (04-10 §c3/§f23).
4. Instrument one battery run to count actual `verify.sh` invocations; settle 04-55 §b1's "runs the shipped bytes 17×" claim; correct or annotate (00-59 §f4).
5. VERSION-SURFACES.md: document the v0.2.0 tag divergence (root pre-split `845f5f38` vs internal post-split `0637d633` — same version number, two trees) regardless of the pure-graph row's outcome (00-23 §c2).
6. `check-pkg-proxy.sh`: per-module/per-tag mode (today it pins the LATEST root tag only) so one-off verifications reuse the gate (00-23 §f4).
7. Cross-module walk-away gate: run `scripts/test-cmd-tq.sh` before walking away from a `Filter`/cross-module retype; AGENTS bullet is size-guarded — fold into the next AGENTS edit with headroom (00-23 §f6).
8. Land the row-114 mint-time done-check (O4-ratified: refuse at mint, `--force-redispatch` escape) — three consecutive no-op re-verifications of the mixed-graph finding (01-48/01-55/02-21) are the loop's loudest recurrence yet.
9. Dogfood the archive-evidence negation carve-out on the next real evidence capture — the field proof 04-10 §b2 deferred (habit, not code).
10. After the vendor-drift heal lands, run the FULL quiet ci-local the family has skipped six windows running, matrix log archived via archive-evidence.sh — one run retires the streak AND dogfoods the negation path (pairs with 9).

## g) QUESTIONS (owner only)

1. **Is the repo-level "Dependabot version updates" setting enabled, and should Dependabot `ignore` the nominal `internal/*` pins (ADR-0017)?** Config verified complete; activation and the week-1 clock are repo-settings facts no sandbox can see; the nominal pins are release statements, not stale versions — weekly bump PRs against them would be permanent review churn. (Carried, unchanged.)
2. **Correction-vs-annotate doctrine for misreceipts**, and whether a fix finding's scope covers sibling claims from the same conflation (e.g. 04-55 §b1's "17×") — asked by 00-59 §g1, carried. (Harvested as a BLOCKED row by the 02-02 pass; not re-rowed.)
3. **Is a daemon post-sweep cheap gate acceptable policy?** Both multi-hour red windows and the vendor drift trace to footer-less sweeps bypassing all hooks; the fix touches shared daemon infra and stays owner-gated. (Harvested; carried.)

## h) BAND DRIFT (ADR-0015 accountability)

**None recorded.** `tq facts --type task.reprioritized` returns `(0 facts)` — the journal holds zero priority-move facts for this window or any other. No marker-, AI-, unblock-, or importance-sourced reprioritization has ever been written; claim order this window was governed by stored priority + aging only (ADR-0015 ladder). Accountability note: ADR-0015's reprioritize machinery exists in the queue surface but has never emitted a fact — if the ladder is meant to be auditable, the fact emission itself is a gap worth a row; not rowed here (would duplicate the 02-02 pass's §h note, which flagged the same).

## Docs-health pass notes (this run)

- Verified at HEAD this pass: check-dead-sha-refs GREEN (263 baselined, 0 new), check-todo-list GREEN, root build RED (vendor drift, `d23f4380`; already rowed by the 02-02 pass — left for its executor, hard docs scope here).
- CHANGELOG verified current for the window (negation carve-out, verify-wrapper + in-situ pin, Dependabot coverage, 0.3.1 cut all present); no user-visible change lacks an entry — the backend-tag verification and receipt fix are verification/docs-only.
- FEATURES row 86 (`--dep-sweep`) was de-staled by the 02-02 pass (now records the O15 retire ruling + queued retirement); no further drift found in a targeted grep.
- README/ROADMAP/AGENTS: no window delta touches their verifiable claims; AGENTS is size-guarded, so additive edits ride the harvested convention rows instead of this pass.
- Archive: no live 2026-10 report is fully resolved (each holds open §b/§f items or is cited by open rows); none moved to `docs/status/archived/` this pass. Index bloat (224–229 live rows vs 100) stays rowed.
- TODO_LIST: 8 new items appended this pass (deduped against the 02-02 harvest and all live rows); zero existing items reworded or deleted; nothing ticked (no row's work landed outside the window's own closures, which used the delete-on-done convention).
