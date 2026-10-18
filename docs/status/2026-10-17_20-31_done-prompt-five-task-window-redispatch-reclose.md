# Done-Prompt Status Report — Five-Task Window, RE-DISPATCH RE-CLOSE (2026-10-17 20-31 CEST)

**Done-prompt task:** `000001a14af06a6f409c9d455a7100000000` — SECOND dispatch of this id.
**Predecessor report:** `docs/status/2026-10-17_19-54_done-prompt-five-task-window-ci-parity-depsweep-artdupl.md` (committed, indexed in docs/status/README.md row 2026-10-17, follow-ups already harvested into TODO_LIST).
**Window covered:** 2026-10-15 05:59 → 2026-10-17 19:35 CEST (five COMPLETED tasks).
**Re-verified at HEAD:** `d4c78425` — root `go vet ./...` rc=0; `internal/bridge` `TestAnswerPollerBoundsAnswersPerPoll` still RED (pre-existing, row 323, unchanged since `8f79dd51`); TODO_LIST 494 open / 121 ticked; journal head seq 13595.

Per the re-dispatch resume path (window-8 verdict report §b2, TODO rows since minted): this pass re-verified the predecessor's load-bearing claims against current HEAD instead of re-deriving the window, built on it, and closed the loop. The predecessor's §a–§g remain the authoritative detail; §1–§7 below are its claims re-confirmed at `d4c78425`, plus what moved since 19-54.

## The window's five tasks (all re-confirmed via closeout reports / commits)

| Task                | Batch                                                                       | Evidence                                                                                                                                                                                                                                   |
| ------------------- | --------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `000001a13db76386…` | ci.yml parity sweep                                                         | closeout 2026-10-15_06-10; commit `7e45d620`                                                                                                                                                                                               |
| `000001a13dc9b39f…` | O2 token/cost daily cap                                                     | DONE-on-arrival close `63e56c34`; ADR-0023; `internal/budget/budget.go:146`                                                                                                                                                                |
| `000001a141d63a4b…` | dep-sweep e2e + questions + anchors                                         | NO closeout (window's worst finding); work in 7 pushed daemon sweeps; item 1 landed (`scripts/smoke/dep-sweep.sh` at `scripts/ci-local.sh:434`, `internal/depsweep/calibration_live_test.go`), CHANGELOG §Added documents both (line 136+) |
| `000001a1478bb7c8…` | art-dupl batch 3/3 (`-t 3` rerun)                                           | closeout 2026-10-17_19-08; `companion.Enqueue` homing `3990c6e9`; flake root cause `bf328e28`+`2ed7c221`; drift re-verify `18a337e1`                                                                                                       |
| `000001a14ade18eb…` | art-dupl batch 4/4 (unix-milli invariant, cmd/tq go.work, heal `--retitle`) | closeout 2026-10-17_19-29; `30e86aba`, `0f0e8167`, `c3cf5191`; ADR-0017 amended                                                                                                                                                            |

## a) FULLY DONE (re-verified this dispatch)

1. **CI parity sweep wiring** — `golangci-lint config verify` (hard), `smoke/reviews.sh` (hard), `smoke/journal-drift.sh` (advisory), `check-module-loop-capture.sh` (hard) in ci.yml; the two deliberate asymmetries are correctly still open TODO rows with their preconditions (dead-SHA cure, worktree.sh orphan) both still standing — the asymmetry rows were right to wait.
2. **Cost cap loop closed** — `budget.Guard.DailyCostCapUSD` fail-open settle, ADR-0023 (docs/adr/0023-cost-budget-beside-count-cap.md), CLI `--daily-cost-cap` (cmd/tq/agentpool.go:169); row 278 ticked. **Docs drift found and fixed this pass:** the cost cap had NO CHANGELOG entry (the count cap `--daily-budget` did) — Added entry appended to `[Unreleased]` (docs-only, in-scope).
3. **art-dupl `-t 3` strict rerun** — caught + homed the real cross-backend `Enqueue` clone in `companion.Enqueue`; mirror gate strict-green (per batch-3 closeout; module tests were green and nothing touched the adapters since).
4. **Status-window flake two-layer root cause** — RFC3339Nano sort + the real unix-milli `updated_at` granularity, tests pin report SET by task ID; documented as the AGENTS.md Store-invariants bullet (verified present at HEAD).
5. **Journal-drift audit honestly BLOCKED** — upstream enrich gated on the M4 memo; row 321 open; confirmed unchanged.
6. **cmd/tq plain workspace path** — committed `cmd/tq/go.work`; CHANGELOG §Fixed documents it (`0f0e8167`); `scripts/test-cmd-tq.sh` gate green per batch-4 closeout, unrotted by the 7 intervening commits (no cmd/tq use-set churn since).
7. **`heal-daemon-sweep.sh --retitle`** — shipped + live-fire used (`c3cf5191`); the 20-23 session's closeouts used it twice more, confirming it works on second parties' sweeps.

## b) PARTIALLY DONE (unchanged verdict, re-confirmed)

1. **Dep-sweep batch (task `…a4bb`) completed 1/3-landed, zero rows ticked, NO closeout** — rows 281/282/283 still open (correct: items 2's webui/CLI half and item 3 never landed); the missing-closeout TODO row from the 19-54 harvest is still open. The live `--dep-sweep` calibration is prepared, not executed (`TQ_DEPSWEEP_LIVE_CALIB=1` never run); FEATURES.md "no live pool run yet" caveats remain accurate.
2. **`TestAnswerPollerBoundsAnswersPerPoll` still RED at HEAD** — re-run this dispatch, FAIL confirmed; row 323 + the BLOCKED drain-contract question stand. Blocks full-battery green claims for every concurrent agent.
3. **Plain cmd/tq path verified but not gated** — sync-guard/GOWOCK=off-audit/plain-path-gate rows (346/347/348) open.
4. **AGENTS.md at 20077/20100 B** — 23 B headroom; budget-policy ruling row open.

## c) NOT STARTED (unchanged)

M4 ratification memo (owner); questions-loop webui/CLI surfaces (row 282); review-anchor residue (row 283); general art-dupl accept-list gate (row 318, BLOCKED); dead-SHA cure pack (rows 779/802; the 19-54 harvest row for the fresh red remains open); index-bloat archive sweep (README index ~305 live rows vs threshold 100 — every session re-pardons it); `tq mcp` stdio server (row 427).

## d) TOTALLY FUCKED UP (carried; nothing new this dispatch)

1. **Task `…a4bb`'s seven pushed lying-subject daemon sweeps** — permanent (history policy); accept-and-document is the operative path pending the owner ruling (BLOCKED question already in TODO_LIST).
2. **AnswerPoller red ~42 h and counting** — the window's own half-landed item; row 323 owns it.
3. **Standing reds unchanged**: guard-wiring orphan (`smoke/worktree.sh`), dead-SHA gate red (394 dangling cites at last count), index bloat, env-streak foreign-dirt burns.
4. **No regressions from the window's own commits**; this dispatch introduced none (docs-only).

## e) WHAT WE SHOULD IMPROVE

1. **The re-dispatch resume path must become the default** — this second dispatch cost ~15 min instead of the first's full research; the minted resume-preflight row (done-prompt follow-ups, 19-54 section) + the BLOCKED verdict-bar question close this when answered.
2. **Landed-but-unticked rows keep manufacturing themselves** (daemon-swept landings); the mechanical sweep row is the durable fix.
3. **CHANGELOG lags for config-surface features**: `--daily-cost-cap` shipped 2026-10-11, ADR-0023 recorded, but no CHANGELOG line existed until this pass — the docs-health VERIFY leg should explicitly check new CLI flags against `[Unreleased]`, not just code.
4. **Batch-completion mechanics** (reclaim deriving COMPLETED with 1/3 landed) remain the top structural debt; rows open.

## f) NEXT THINGS

No new rows minted this dispatch — every candidate deduped against the open list (the 19-54 pass already minted its 7 + 2 BLOCKED questions; the 20-23 pass minted its 2). The highest-value open items this window re-proved: row 323 (poller red), rows 281-283 (the unlanded batch + missing closeout), rows 346-348 (go.work gating), rows 779/802 (dead-SHA cure), the missing-`…a4bb`-closeout row, and the M4 memo unblock chain (row 321).

## g) QUESTIONS

All three of this window's owner questions are already parked in TODO_LIST as BLOCKED items (19-54 section): poller drain-order contract, pushed lying-subject policy, and the re-dispatch verdict-recording bar. No new ones.

## h) BAND DRIFT

**None recorded.** The journal (`/mnt/pool/services/tq/tq.db`) holds ZERO `task.reprioritized` facts all-time (queried this dispatch: count 0 at head seq 13595) — no priority moves to account for in this window or any prior one. Every dispatch ordering so far has been harvest-order + lease-claim, not priority-driven; ADR-0015's accountability section stays trivially empty until the first reprioritization lands.
