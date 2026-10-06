# Done-prompt status report — 2026-10-06 02:02 CEST

**Window:** five completed agent tasks, 2026-10-05 ~04:10 → 2026-10-06 ~01:00.
**Method:** every closeout report read first; every claimed delta verified against git log, gate re-runs, and the live tree at HEAD (post-window commits `57c6b5b6`…). Journal queried for `task.reprioritized` facts for §h.

**Tasks in the window:**

| Task | Deliverable | Work commit(s) | Closeout |
| --- | --- | --- | --- |
| 000001a109c3… | archive-evidence gitignore-negation carve-out | `82f1415c` | 2026-10-05_04-10 |
| 000001a109e0… | check-verify.sh in-situ flake-heal pin (pin 5) | `d76c6437` (+healed `3b136c41`) | 2026-10-05_04-55 |
| 000001a10e16… | Backend-tag v0.2.0 release verification | `dde3ae78` (+healed `22e8c50f`) | 2026-10-06_00-23 |
| 000001a10e32… | Dependabot full-tree coverage | `9badd3bd` + `f34c63ac` | 2026-10-06_00-44 |
| 000001a10e36… | Review-fix: gate receipt 17 → 16 | `cfd09fe4` | 2026-10-06_00-59 |

## a) FULLY DONE (verified at HEAD this pass)

1. **archive-evidence negation carve-out** (`82f1415c`): Step 1 of `scripts/archive-evidence.sh` now parses `git check-ignore -v` output and treats a leading `!` pattern as deliberate content (ok-line + proceed); only positive-rule hits refuse. Root cause reproduced in a scratch repo BEFORE the fix; regression pinned as the smoke's 11th check. CHANGELOG Fixed bullet present (line ~426); TODO row closed. Verified: `git show 82f1415c`, CHANGELOG grep, closeout §a.
2. **check-verify in-situ flake-heal pin** (`d76c6437`): pin 5 executes `bash scripts/verify.sh` itself through a scripted flake-heal (stamp-file PATH shim: first `go` invocation fails with the known-flaky signature, later ones pass; shimmed `gofmt` keeps it hermetic). Fail-closed by construction — battery drift turns the pin red, never silently green. Gate receipts re-verified THIS pass: `grep -c '^ok:'` on a fresh captured run = **16**, rc=0 (the window's own receipt bug is what task 5 fixed).
3. **Backend-tag v0.2.0 verification** (`dde3ae78`): per-module proxy check green (`internal/queue/{sqlite,postgres}` list v0.2.0 + v0.3.0); remote tags confirmed; clean-room in-namespace consumer compile green (sumdb-verified `go get`, tidy closes the graph, full store surface builds, GOWORK=off). Literal `go install <libmodule>@<tag>` proven impossible-by-design (facade go.mods carry relative replaces — toolchain refusal everywhere, CI included); the consumer path is the canonical proof. Bonus repair: fixed a ~26h-silently-red `internal/e2e` at HEAD (daemon sweep `1a403842` had captured a mid-flight `Filter.Status` retype leaving cmd/tq `narrowCounts` on `*string`); `scripts/test-cmd-tq.sh` green, full `-race` battery rc=0.
4. **Dependabot full-tree coverage** (`9badd3bd`): phantom `cqrsqlite` entry replaced with the three missing post-S2 modules; mechanical cross-check this pass of record: **23 gomod entries == 23 tracked module dirs, 0 dup/missing/phantom**, github-actions entry intact. Also repaired a pre-existing doc-refs red (`f34c63ac`: FEATURES cited nonexistent root `companion/conform`; CHANGELOG's historical `internal/queue/cqrsqlite` backtick allow-listed with dated rationale). `--dep-sweep` retirement re-queued as a BLOCKED row (O15 time gate, earliest 2026-10-13).
5. **Receipt-fix review task** (`cfd09fe4`): the rejected work's headline "17 ok" corrected to 16 at all three same-receipt sites (index row + 04-55 §a3/§b3/§d5); ground truth established mechanically (`grep -c '^ok:'` = 16, matching the report's own 12+1+3 decomposition). Different-metric receipts (go test 17-ok, "17×" invocations) correctly left untouched.
6. **Window-family residue, verified this pass:** all five work commits carry the correct Task-Queue-ID footer as last trailer; daemon sweeps in the window were healed onto their task IDs; **check-dead-sha-refs is GREEN at HEAD now** (this pass completed the O16 mechanical path the 00-44 window deferred: the four dangling `99458a91→22e8c50f` citations — the healed backend-tag sweep commit — now carry the fork-record arrow `99458a91→22e8c50f` in the 00-44 report and the status index; gate rc=0, 0 dead cites). check-doc-refs rc=0; check-status-index rc=0 (warnings only). Root build/vet/test NOT green at HEAD — pre-existing vendor drift, see §d5.

## b) PARTIALLY DONE

1. **Dependabot activation is config-only** (00-44 §b2): nothing pushed, repo-level "Dependabot version updates" setting unverified — the O15 week-1 clock (earliest 2026-10-13) has NOT started. Retirement execution correctly parked behind the gate.
2. **Pure-graph v0.2.0 installability proof** — the mixed-graph hazard (root v0.2.0 = pre-split `845f5f38` vs internal tags = post-split `0637d633`) is INFERRED from a live v0.2.0×v0.3.0 conflict; the row's own first step (fresh-module pure test, then document-or-retract) is open in TODO_LIST ("Backend-tag verification window follow-ups").
3. **The receipts conflation class is patched per-instance, not mechanized**: two confirmed instances (02-11, 04-55/00-59) plus a near-miss; the canonical-receipt rule (AGENTS bullet) and summary-line re-prefix remain unlanded (00-59 §e1/e2).
4. **archive-evidence field-proof pending** (04-10 §b2): the negation path is pinned in scratch repos only; no real `.log` has ridden the un-ignore rule in this repo yet.
5. **Verify-wrapper coverage is heal-only in situ**: the genuine-red path (wrapper executed to rc=1, zero retries) is still lib-level only — TODO row already exists (row 99 class).
6. **ci-local/full-battery skip streak**: the window family ran scoped gates with prose rulings five-to-six times running; no full quiet-host ci-local with archived matrix log has landed.

## c) NOT STARTED (backlog items the window skipped — carried rows, verified still open)

- `.tq-verify` → `scripts/verify.sh` flip cluster (owner-BLOCKED; the wrapper still has zero production callers).
- KNOWN_FLAKY ↔ Known Issues grep gate (TODO row ~98).
- Status-index archival sweep: **228 live rows vs the 100 threshold** (measured by the gate this pass; was 207 at 04-36, 224 at 00-59 — every dispatch worsens it).
- Genuine-red in-situ verify.sh pin (row ~99).
- `--dep-sweep` retirement execution (O15 time gate).
- Daemon post-sweep cheap doc/config gate (00-44 e4 — the ~26h red-e2e and ~18h red-doc-refs windows both happened because nothing runs between a daemon sweep and the next battery).
- The four near-identical-interface / consumer-ghost / postgres-wiring owner rows — untouched, correctly BLOCKED.

## d) TOTALLY FUCKED UP (verified regressions, broken gates, debt)

1. **Nothing in the window shipped broken** — all five deliverables verified green at HEAD, and the one broken-HEAD discovery (red e2e) was repaired in-window.
2. **The 04-55 window shipped a self-certified WRONG receipt as its headline** ("17 ok" contradicting its own 12+1+3=16 decomposition) and the reviewer — not the window — caught it. Its own §d5 confessed "no receipts captured at run time" while shipping the unverified count. Verification-of-the-verification was the hole; the family wrote the 02-11 confession one report up and re-offended in the next window.
3. **The backend-tag window's heal left citation residue**: three of its own report lines plus the index row cited `99458a91→22e8c50f` — the very commit it healed out of history — leaving check-dead-sha-refs red at HEAD until this pass completed the fork-record path (fixed here, see a6).
4. **Daemon sweeps keep minting breakage**: `1a403842` swept an incomplete cross-module edit and left internal/e2e red for ~26h; the 10-05 foreign commits left doc-refs red ~18h; the dependabot config itself arrived in an 84-file unverified sweep and drifted within a week (three missing modules + one phantom).
5. **Root build gate RED at HEAD right now (found by this pass, docs-only window):** `d23f4380` (the 0.3.1 changelog cut) pinned readmodel v0.3.1 in root go.mod without `go mod vendor` — vendor/modules.txt still says v0.3.0, so every root build/test dies on "inconsistent vendoring" (verified red at clean HEAD via git stash). Rowed in TODO_LIST; not fixed here (hard docs scope).
6. **Process debt repeated, confessed each time**: edit→commit→battery ordering violated twice in the window (00-23 d3, 04-55 d2 — the second produced a daemon race needing a heal); View-before-edit rejections recurred (04-55 ×3, 00-44 ×2); read-the-full-gate-output discipline violated again (04-10 `tail -1` on a 93-hit log).

## e) WHAT WE SHOULD IMPROVE

1. **Canonical receipt rule (highest leverage):** every "N ok / M fail" receipt in a report must come from `grep -c '^ok:'` over a captured gate-output file — never counted in context, never taken from a summary line. One AGENTS.md sentence closes the twice-fired class.
2. **Kill the trap at the source:** re-prefix check-verify.sh's final summary line (`summary: verify self-test …`) so no ok-grep can count it; audit root-gate.sh's extraction of that line and land both in one commit.
3. **Post-daemon-sweep cheap gate:** a footer-less sweep triggering doc-refs + todo-list + status-index (~1 min) would have caught both multi-hour red windows in minutes instead of half-days. Needs a safe hook point — owner call.
4. **Dependabot parity mechanical:** `check-dependabot-coverage.sh` (1:1 yml↔`git ls-files *go.mod`) in ci-local + `new-module.sh` emitting the entry at creation. The config drifted within one week; clerical parity will drift again.
5. **Heal tool leaves dangling cites:** a post-heal step grepping docs/status for the rewritten SHAs and printing the fork-record command would have prevented d3 entirely.
6. **Edit→commit→battery is a daemon race, not a preference:** two windows violated it and both paid the heal tax. The rule needs mechanical enforcement (commit-task.sh detecting a fresh footer-less sweep and offering the heal) more than another confession.
7. **Cross-module edits need a walk-away gate:** root `./...` reaches cmd/tq only via internal/e2e; `scripts/test-cmd-tq.sh` before walking away from a cross-module retype is the cheap rule the 26h red window bought.
8. **Confession-to-mechanism conversion:** the family confesses receipts-from-context faster than it mechanizes it. Rule: a confessed §d item with a one-line mechanical fix gets the fix in the same window or a TODO row with an effort estimate.

## f) NEXT THINGS (grounded; deduped against live TODO rows — genuine-red, KNOWN_FLAKY, dep-sweep retirement, and the pure-graph row already exist and are NOT re-listed)

1. Land the canonical-receipt rule in AGENTS.md Conventions (captured file + anchored grep; e1).
2. Re-prefix check-verify.sh's summary line; audit root-gate.sh's extraction first; one commit (e2).
3. Add `check-dependabot-coverage.sh` (yml↔go.mod 1:1) into ci-local cheap gates (e4a).
4. Extend `scripts/new-module.sh` to emit the dependabot entry at module creation (e4b).
5. heal-daemon-sweep.sh post-heal citation check printing the fork-record command (e5).
6. commit-task.sh: on "nothing to commit", detect a fresh footer-less `chore:` sweep on the same paths and offer/auto-run the heal (e6).
7. Daemon post-sweep cheap doc/config gate (e3; owner nod on hook point).
8. Status-index archival sweep or first monthly digest row — 228 live rows vs 100 threshold, worst on record (c).
9. One full quiet-host ci-local with the matrix log archived — ends the six-data-point prose-skip streak (b6).
10. Instrument one battery run to settle 04-55 §b1's "runs the shipped bytes 17×" claim; correct or annotate.
11. Wire a consumer-path proof (`go get`+`tidy`+`build`) of tagged facade modules into release.sh --push or CI — the only possible verification form now proven (00-23 f3).
12. VERSION-SURFACES.md: document the v0.2.0 tag divergence (root pre-split `845f5f38` vs internal post-split `0637d633`) regardless of the pure-graph outcome.
13. check-pkg-proxy.sh per-module/per-tag mode so one-off verifications reuse the gate (00-23 f4).
14. Adjudicate webui `TestConcurrentClientsRace` onto the root-gate known-flaky list or confirm it as a real defect (00-23 f7).
15. Dogfood the archive-evidence negation carve-out on the next real evidence capture (04-10 §b2 field proof).
16. archive-evidence `--help` line documenting the negation carve-out + smoke asserting it (04-10 §c3/§f23).
17. Cross-module walk-away gate: add "run scripts/test-cmd-tq.sh before walking away" to the cross-module-edit workflow (e7; AGENTS size-guarded — fold into the next AGENTS edit).
18. Re-verify pkg.go.dev rendering for the v0.2.0 backend tags specifically (00-23 f12; the 404-window class).

## g) QUESTIONS (owner only)

1. Is the repo-level **"Dependabot version updates" setting enabled**, and does the O15 week-1 clock start at push? The config is verified complete; whether GitHub will actually schedule updates (and thus whether 2026-10-13 retirement can fire) is a repo-settings fact no sandbox can see. Related: should Dependabot `ignore` the nominal `internal/*` pins (ADR-0017) — release statements, not stale versions, or we pay weekly PR churn on them?
2. **Correction-vs-annotate policy for misreceipts:** may decomposition-provable miscounts be corrected in place (as `cfd09fe4` did), or must every corrected receipt carry a `(corrected <date>)` marker? And when a fix finding exposes SIBLING claims from the same conflation (e.g. 04-55 §b1's "17×"), does the fix run correct them too or strictly the named sites?
3. Is a **daemon post-sweep cheap gate** acceptable policy (e3)? Both multi-hour red windows this window trace to footer-less sweeps bypassing all hooks; the fix touches shared daemon infra, so it stays owner-gated.

## h) BAND DRIFT (ADR-0015 accountability)

**None recorded.** The journal (9615 facts, full scan this pass) contains zero `task.reprioritized` facts — no marker-, AI-, unblock-, or importance-sourced priority moves exist for this window or any other. Claim order this window was governed by stored priority + aging only. (ADR-0015's reprioritize machinery exists in the queue surface, but it has never written a fact; if the ladder is expected to be auditable, the emission of `task.reprioritized` facts may itself be a gap worth a row.)

## Docs-health pass notes (this run)

- Fixed the four dangling `99458a91→22e8c50f` citations (fork record `99458a91→22e8c50f`) — check-dead-sha-refs red → green at HEAD.
- FEATURES row 86 (`--dep-sweep`) de-staled: the "first live pool run pending the rollout ruling" claim predated O15's retire ruling; now records the ruling and the queued retirement.
- CHANGELOG verified current for the window (negation carve-out, verify-wrapper + in-situ pin, Dependabot coverage all present); no user-visible change lacked an entry — the backend-tag verification and receipt fix are docs/verification-only.
- README/ROADMAP/AGENTS: no window delta touches their claims; AGENTS size guard (15,286 B) argues against additive edits this pass.
- Archive: no 2026-10 report in the live index is fully resolved (each still holds open §b/§f items or is referenced by open rows); none moved to archived/ this pass. The 228-row index bloat is rowed as next-item 8.
