# Done-Prompt Status Report — Five-Task Window: CI Parity, Cost Cap Close, Dep-Sweep E2E, art-dupl Batches 3+4 (2026-10-17 19-54 CEST)

**Done-prompt task:** `000001a14af06a6f409c9d455a7100000000`
**Window covered:** 2026-10-15 05:59 → 2026-10-17 19:35 CEST (five COMPLETED tasks, verified in the live journal `/mnt/pool/services/tq/tq.db`, head seq 13551)
**Verified at HEAD:** `a6dbf658` — root `go vet ./...` green, `internal/webui` tests green (11.9 s), `internal/bridge` papdashboard AnswerPoller test RED (pre-existing, row 323), dead-SHA gate RED (394 dangling cites, row-owned), AGENTS.md 20077/20100 B.

The five window tasks, each read from its own closeout report first:

| Task | Batch | Closeout |
| --- | --- | --- |
| `000001a13db76386…` | ci.yml parity sweep (wire ci-local-only gates or record asymmetry) | docs/status/tasks/2026-10-15_06-10_task-000001a13db763865bf27194cde800000000.md |
| `000001a13dc9b39f…` | token/cost daily cap (O2) | none written — closed via commit `63e56c34` (DONE-on-arrival) |
| `000001a141d63a4b…` | depbump/depsweep e2e + questions-loop surfaces + anchor residue (3-item batch) | **none written** — see §d1 |
| `000001a1478bb7c8…` | art-dupl follow-ups batch 3/3 (`-t 3` rerun, status-window flake, journal-drift) | docs/status/tasks/2026-10-17_19-08_task-000001a1478bb7c81e2c09da2abe00000000.md |
| `000001a14ade18eb…` | art-dupl follow-ups batch 4/4 (unix-milli invariant, cmd/tq go.work, heal `--retitle`) | docs/status/tasks/2026-10-17_19-29_task-000001a14ade18eb3cd0720b410600000000.md |

---

## a) FULLY DONE

1. **ci.yml parity sweep** (`7e45d620`, 2026-10-15): `golangci-lint config verify` wired as a HARD step (kills the silent-disabled-linter class), `smoke/reviews.sh` wired hard (hermetic), `smoke/journal-drift.sh` wired advisory (`continue-on-error`, mirroring the ci-local ruling), `check-module-loop-capture.sh` wired hard (its own first live exercise: loops=6, captures=6). Two deliberate asymmetries minted as follow-up rows (both still open, correctly pending their preconditions). One batch row corrected in passing (`smoke/ratelimit-e2e.sh` had been wired since the 04-03 batch — stale source claim).
2. **Cost-denominated daily cap: loop closed** (`63e56c34`, 2026-10-15): the O2 cap had already landed in `f6912468` (2026-10-11) — `budget.Guard.DailyCostCapUSD` refuses at/over today's derived spend, `UsageToday` settled FAIL-OPEN, cap-unit ruling recorded as ADR-0023, CLI `--daily-cost-cap`. The window task verified the budget module gate in-module and ticked TODO row 278 (now `[x]` with the full DONE note). Honest DONE-on-arrival re-dispatch: zero code churn, correct attribution.
3. **art-dupl `-t 3` rerun found and killed a real clone** (`3990c6e9`, 2026-10-17): the strict-threshold rerun (01-34 §f34) caught a NEW byte-identical cross-backend `Enqueue` mirror in `sqlitev4/adapter.go` + `postgresv4/adapter.go` that the `-t 4`-era passes had left. Homed ONCE in `companion.Enqueue` (engine-driven routing; both adapters are now one-line delegates) per the 2026-09-26 dedup ruling; mirror gate strict-green after (0 cross-backend groups). TODO row 319 ticked.
4. **Status-window flake root-caused for real and fixed** (`bf328e28` + `2ed7c221`, 2026-10-17): two-layer cause — second-granularity RFC3339 sort key (→ `RFC3339Nano`) masking the true layer: the upstream engine persists `updated_at` as unix **MILLIS**, so same-millisecond completions are order-indistinguishable and the sort fell to a random TaskID tiebreak. Window tests now pin the report SET by task ID instead of array index; package green 5× at `-race`. Rows 320 ticked; the 01-34/02-35 source reports annotated + archived (row 331).
5. **Journal-drift audit re-verified and honestly left BLOCKED** (`18a337e1`, 2026-10-17): both cmd/tq drift tests PASS at HEAD; vendored go-cqrs-lite still writes `task.enqueued` detail `{project,type}` only; the honest-degradation state (`Caps.EnqueuedSnapshot=false`, replay-ready handling) confirmed in place. Upstream enrich stays gated on the unwritten M4 memo; row 321 open with the blocker recorded.
6. **Unix-milli store invariant documented** (`30e86aba`, 2026-10-17): new AGENTS.md Store-invariants bullet (ordering-sensitive code must not assume sub-millisecond order; deterministic non-time tiebreaks or set-wise assertions), paid for by a 259 B gross prune; TODO row 327 ticked; `TestAgentsDocSizeGuard` green.
7. **Plain `cd cmd/tq && go test` works without the shim** (`0f0e8167`, 2026-10-17): root cause was the replace-free go.mod pinning pre-HEAD internal tags; fixed with a committed `cmd/tq/go.work` carrying the shim's use-set (replace-free discipline intact; nix + gates pin `GOWORK=off`; ADR-0017 amended). Verified this pass at HEAD: plain `go build`/`go vet` in cmd/tq green; `scripts/test-cmd-tq.sh` gate green. Row 328 ticked.
8. **`heal-daemon-sweep.sh --retitle <subject>`** (`c3cf5191`, 2026-10-17): healed footer-less sweeps can now get their true subject (footer-carrying commits pass through untouched); self-test grew to 21 checks; first real-world use healed a mid-batch sweep same-day. Row 329 ticked.
9. **Dep-sweep e2e smoke landed and is CI-gated**: `scripts/smoke/dep-sweep.sh` (stub planner → minted depbump DAG → go get/commit/tag happy path over scratch repos → convergence rule pin: completed dedup keys return as known hits, never sweep failures) is wired into `scripts/ci-local.sh:434`. Plus a `DedupKey` generation suffix and a depbump git-add fix rode the same batch (all verified present at HEAD).
10. **Live-calibration entry point for `--dep-sweep`**: `internal/depsweep/calibration_live_test.go` (env-gated `TQ_DEPSWEEP_LIVE_CALIB=1`) runs the real planner+sweeper over a real projects dir and reports what a first live pool run would mint — zero production side effects by construction. (The calibration itself has not been EXECUTED — see §b2.)
11. **Verification battery of the window, re-checked at HEAD this pass**: per-module gates green for every touched module (companion, sqlitev4, postgresv4, status, depsweep), root vet green, cmd/tq gate green, mirror gate strict-green.

## b) PARTIALLY DONE

1. **The depsweep 3-item batch (task `…a4bb`) completed with only item 1 substantially landed.** Item 1 (dep-sweep e2e + calibration entry) landed as §a9/§a10; item 2's FIRST slice (AnswerPoller `MaxAnswersPerPoll` bound, `answers.go:54`) landed but its pin test is RED (§d2); item 2's webui/CLI surfaces (parked-on-question badge, detail-page questions section, `tq tasks` open-questions filter, stats counters) are NOT in the tree; item 3 (review-anchor residue: webui finding-card anchor render, retry-trail rejections, dlqfix/cqa anchor discipline) is NOT in the tree. TODO rows 281/282/283 all correctly remain OPEN — nothing may be ticked beyond what landed.
2. **The "first live `--dep-sweep` pool run calibration" is prepared, not executed**: the env-gated test exists but nothing shows it has ever been run against the real projects dir. FEATURES.md's "no live pool run yet" caveats on the depbump/dep-sweep rows remain accurate.
3. **The plain cmd/tq workspace path is verified but not gated** (batch-4 closeout §b1): nothing prevents `cmd/tq/go.work` use-set rot; sync-guard + gate rows minted by that closeout (346/347/348) are open.
4. **`check-dead-sha-refs.sh` fork-record exemption is single-pair-per-line**: the batch-4 closeout's own three-pair fork-record lines (:49, :148) still flag as DEAD findings — the exemption exists but comma-separated multi-pair lines defeat it (§f7 below).
5. **ssetest v0.5.0 webui migration: landed, row still open** — the fix (branded `sse.NewEventID` at `webui_test.go:454`) arrived via footer-less daemon sweep `38e2c7cf`; root vet and the webui suite are green at HEAD, but TODO row 888 was never ticked. Ticked this pass with that evidence.
6. **AGENTS.md headroom is 23 B** (20077/20100): every future rule needs a prune-first ritual; the budget-policy row (350) is open and owner-gated.

## c) NOT STARTED

1. **M4 ratification memo** (owner artifact) — still gates the upstream enqueued-detail enrich (row 321); the single biggest unblock for the journal-drift blind spot.
2. **Row 283 review-anchor residue** (webui finding-card anchor render, retry-trail parse-time rejections, dlqfix/cqa anchor discipline) — untouched this window.
3. **Rows 282's webui/CLI question surfaces** (badge, detail section, open-questions filter, stats counters) — untouched.
4. **The missing closeout report for task `…a4bb`** — no report exists anywhere in docs/ for it (§d1).
5. **General art-dupl accept-list gate in ci-local** (row 318) — owner-ruling BLOCKED, unchanged.
6. **Dead-SHA cure decision pack** (rows 779/802) — dangling cites GREW again this window: 385 (10-16) → **394** (today).
7. **Index-bloat archive sweep** — 254 live reports vs threshold 100; this pass archives one fully-resolved report and leaves the corpus sweep row-owned.
8. **`tq mcp` stdio server** (row 427) — re-minted previously, never shipped.

## d) TOTALLY FUCKED UP

1. **Task `000001a141d63a4bb487c2dcdf4d00000000` completed with ALL of its work buried in seven pushed, lying `chore: auto-commit` daemon sweeps** (`c57fff37`, `df99c9af`, `a2e30bef`, `759496e0`, `0d54c898`, `400573eb`, `058923db`, 10-16 01:14–02:42) — real code (dep-sweep smoke + wiring, depsweep/depbump/prioritize changes, cmd/tq main), all footer-carrying (attribution survives) but subjects lie permanently (all pushed → history policy forbids reword). No closeout report, zero of its three TODO rows ticked, and the batch's item-2 half landed RED (`TestAnswerPollerBoundsAnswersPerPoll`, row 323) — the same "landed-but-unticked + unverified half" shape the 10-16 verdict report flagged in its §b6, one day before it hardened. This is the window's worst outcome and the top backlog item.
2. **The AnswerPoller bound shipped RED and stayed RED for ~41 h** (since `8f79dd51`, 10-16 02:48): verified RED at HEAD this pass (cursor advanced past an unapplied answer). It blocks full-battery green claims for every concurrent agent (both batch-3 and batch-4 closeouts scoped their gates around it). Contract fix owned by row 323; needs the drain-order ruling (§g1).
3. **Footer-less daemon sweeps keep landing REAL work**: `38e2c7cf` carried the ssetest migration fix (row 888 unticked for ~30 h); the whole `…a4bb` span (d1). The 06-43 report's "daemon sweeps must never carry dependency-graph edits" got its ssetest fix, but the broader class — real code landing agentlessly — recurred in every window this week.
4. **Dead-SHA gate red and growing** (385 → 394): the accretion includes the window's own fork-record lines the exemption parser cannot parse (b4) — the cure tooling and the gate are fighting each other.
5. **Carried standing reds, unchanged by the window**: guard-wiring orphan (`smoke/worktree.sh`), index bloat 254 vs 100, env-streak foreign-dirt burns (second sighting 10-16 01:13, investigation row open), pushed footer-less `c7e95833` attribution loss (permanent).
6. **No regressions were introduced by the window's own commits**: every closeout's scoped battery was green at its HEAD, and this pass re-verified the touched surfaces green. The window's damage is omission (d1/d2), not commission.

## e) WHAT WE SHOULD IMPROVE

1. **A batch that completes must reconcile its rows.** The `…a4bb` task derived COMPLETED while 2/3 items were unlanded and 0/3 rows were ticked. Until the reclaim/derived-completion mechanics are pinned (rows open), the cheap half is procedural: a done-prompt's re-dispatch check should include "does a closeout report exist for MY id, and are my rows' tick-states consistent with my commits' file lists?"
2. **Landed-but-unticked rows are now a pattern, not an incident** (225 last window; 281-283 + 888 this window). A mechanical sweep — grep each open row's named symbols/paths at HEAD — would retire them in bulk and should run per docs-health pass.
3. **Write the closeout BEFORE the last commit of a session.** Both zero-closeout tasks this window (`…a4bb`, and the cap task until a follow-up pass closed it) lost their self-review entirely; the report is the only artifact that survives the daemon.
4. **The dead-sha fork-record format and its parser need one contract**: multi-pair lines are the natural way humans write heal chains; either the parser learns them or the heal tool prints one pair per line and the convention says so.
5. **Per-item verification inside batches**: the AnswerPoller bound landed with an unsatisfiable pin test and no green run of its own package. One `go test ./internal/bridge/...` before the last commit would have caught it on day one.
6. **ci.yml parity needs a tail-gate for NEW ci-local steps**: dep-sweep.sh landed wired in ci-local only, after the parity sweep that would have caught it. The parity rule ("wire or record the asymmetry") should apply at landing time, not only during sweeps.
7. **FEATURES.md honest-caveat discipline works** — the "no live pool run yet" caveats on depbump/dep-sweep stayed truthful through this window; keep paying that tax when the calibration finally runs.

## f) UP TO 50 NEXT THINGS (deduped against open rows; MINTED items entered TODO_LIST)

**MINTED this pass (see the new TODO_LIST section):**

1. Write the missing closeout for task `000001a141d63a4bb487c2dcdf4d00000000` and give rows 281/282/283 evidence-based verdicts (pushed span `c57fff37`..`058923db`; subjects unrecoverable per history policy — document, don't rewrite). **MINTED**
2. Bulk landed-but-unticked sweep: mechanically verify open rows' named symbols/files at HEAD and tick with evidence (this window: 281-283 span, 888). **MINTED**
3. Extend the `check-dead-sha-refs.sh` fork-record exemption to comma-separated multi-pair lines. **MINTED**
4. Wire `scripts/smoke/dep-sweep.sh` into ci.yml or add the deliberate-asymmetry comment (landed post-sweep, currently silent). **MINTED**
5. Auto-mint a fix-first task when a `class=gate` requeue names a failing gate (the ssetest red sat 25 h before a human connected requeue payload → row). **MINTED**
6. Pin the redaction guard's camelCase word-boundary matcher with a `SetResultDetail`-shaped case-collision fixture (the `2d6b3993` fix has no regression pin). **MINTED**
7. Cheap session-start red canary (build + targeted tests over known modules) so sessions attribute foreign reds up front (19-08 closeout §e6; AnswerPoller cost two sessions scoping around it). **MINTED**

**Strongest existing rows this window re-proved (do NOT re-mint):**

8. Row 323 — fix `TestAnswerPollerBoundsAnswersPerPoll` (RED at HEAD, verified this pass).
9. Row 325 — ordering-sensitive sorts audit over store timestamps (the unix-milli class).
10. Row 326 — `companion.Enqueue` conformance pin (mirror gate catches clones, not re-divergence).
11. Rows 346/347/348 — cmd/tq go.work sync guard + GOWORK=off audit + plain-path gate.
12. Row 350 / 830 — AGENTS.md budget decision (23 B headroom; owner-gated).
13. Rows 321 + M4 memo — upstream enqueued-detail enrich unblock chain.
14. Row 318 — general art-dupl accept-list gate (owner ruling).
15. Rows 779/802 — dead-SHA cure pack + arbitration (394 dangling cites).
16. Rows 282/283 — questions-loop surfaces + review-anchor residue (the `…a4bb` batch's unlanded items).
17. Row 281 — finish the dep-sweep item honestly: the live calibration half still needs executing (via `TQ_DEPSWEEP_LIVE_CALIB=1`), then FEATURES caveats update.
18. Row 888 — now ticked this pass; verify-nothing-left class closed.
19. Rows 197/210 — full ci-local battery at HEAD (blocked until row 323 goes green).
20. Row 831 — worktree.sh fix-then-wire (guard-wiring orphan standing).
21. Rows 836-839 — parked-provider family (JSON surface, doctor naming, LastFacts dedup, postgres conform proof).
22. Row 226 — `SetFailureEvidence` for depbump (re-verified unlanded by the 10-16 pass; the `…a4bb` batch did not land it either).
23. Row 239 — `tq stats --project` parked-count × filter pin (last open row of the 09-17 CI-resilience cluster).
24. Row 889 — done-prompt resume preflight (would have saved the 10-16 verdict task four dead attempts).
25. Index-bloat ANNOTATE/archive sweep (254 live rows; row-owned corpus pass).

**Smaller, concrete, report-only:**

26. `heal-daemon-sweep.sh`: print a trailing "paste FORK-RECORDs into the window report" hint (records die in scrollback; 19-29 closeout f21).
27. `tq audit --journal --since <ts>` — retires the pre/post-`4ba81d8` comparability caveat class (standing idea, 02-26 §40 / 04-30 §f27).
28. Sweep the "Done-prompt window 8 follow-ups" TODO sections once their rows close (both sections will be fully ticked/stale and can fold).
29. gosec coverage for cmd/tq + converge the ci.yml gosec loop onto check-gosec.sh (rows 144/145, owner-gated).
30. Single pin source for the golangci-lint version (row exists — the parity sweep's §e5 residue).
31. shellcheck in the flake devShell + a check (heal script is tab-heavy, hand-verified only).
32. Evaluate nanosecond `updated_at` upstream (schema cost vs the whole same-milli class, 19-08 f5) — pairs with the M4 memo conversation.
33. Check whether go-cqrs-lite has an upstream issue for sub-milli timestamps; file after verification if not (verify-before-filing gate applies).
34. Gate invocations in cmd/tq use `go build -o <tmp>/tq` so binaries stop dropping into the module dir (19-29 f24).
35. Clean stale `cmd/tq/tq.exe` build artifact (gitignored, cosmetic; 19-29 f23).
36. Digest row or archive sweep for docs/status/README.md before the next 100-row doubling (conventions section pins both paths).

## g) QUESTIONS ONLY THE OWNER CAN ANSWER

1. **AnswerPoller drain contract (blocks row 323):** when a poll is bound-truncated, must answers apply OLDEST-first (FIFO, cursor=newest applied, lossless) or NEWEST-first (needs per-ref applied-dedup)? The committed test wants newest-first + no re-apply + full drain — mutually exclusive under any monotone cursor. Row 323 says "decide the contract" but the choice changes documented PapDashboard answer semantics: delegated to the fixing agent, or an owner ruling?
2. **Policy for PUSHED footer-carrying daemon commits with lying subjects (the `…a4bb` span):** history policy forbids rewording pushed commits, so the seven `chore: auto-commit` subjects carrying real dep-sweep/questions work are permanent. Accept-and-document (footer attribution + this report), or is a subject-only rewrite sanctioned for footer-carrying daemon commits whose FOOTER (the attribution contract) is already correct?

## h) BAND DRIFT

**None recorded.** The production journal (`/mnt/pool/services/tq/tq.db`, head seq 13551) holds **zero** `task.reprioritized` facts — all-time, not just this window (re-verified this pass via `tq facts --type task.reprioritized`; third consecutive done-prompt to confirm the type has never fired). All five window tasks ran exactly as dispatched at their stored priorities; no ADR-0015 accountability entries exist to explain. Two `task.requeued` detours inside the window are worth the record, neither a priority move: the cap task ate one 429 rate-limit requeue and one budget-gate park (15 m) before completing 10-16 01:06, and the depsweep task ate one preflight dirty-repo requeue plus one 429 before completing 10-16 06:25. The standing gap — a priority system that never reprioritizes is either perfectly stable or unexercised — remains flagged for the owner (previous done-prompts noted the same).

---

## Provenance

- Closeout reports read FIRST (per dispatch contract): the three files listed in the header table; the two unreported tasks reconstructed from `git log --grep Task-Queue-ID`, the live journal (`tq show`, `tq facts`), and the 10-16 verdict report's §b6 eyewitness note.
- SHAs cited are reachable from HEAD (checked); fork-record pairs avoided except where already gated.
- Live-doc checks this pass: root vet + webui suite green; AnswerPoller red reproduced; dead-sha count 394; AGENTS.md 20077 B; ci-local/ci.yml grep for dep-sweep wiring; FEATURES/README/ROADMAP spot-checks.
- Docs-health pass: 18-34 batch report archived (every item resolved: shipped / routed to open rows / narrative); row 888 ticked with landing evidence; CHANGELOG entries added for the window's developer-visible changes; no code, script, config, or test files touched.
