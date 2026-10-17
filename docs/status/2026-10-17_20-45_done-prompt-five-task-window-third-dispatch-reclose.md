# Done-Prompt Status Report — Five-Task Window, THIRD DISPATCH RE-CLOSE (2026-10-17 20-45 CEST)

**Done-prompt task:** `000001a14af06a6f409c9d455a7100000000` — THIRD dispatch of this id.
**Predecessor reports (authoritative detail):** `docs/status/2026-10-17_19-54_done-prompt-five-task-window-ci-parity-depsweep-artdupl.md` (first dispatch, full window derivation) and `docs/status/2026-10-17_20-31_done-prompt-five-task-window-redispatch-reclose.md` (second dispatch, resume path). This pass re-verifies their load-bearing claims at current HEAD `2eba816f` and records the delta since the 20-31 close (`d4c78425`).
**Window covered:** 2026-10-15 05:59 → 2026-10-17 19:35 CEST (five COMPLETED tasks).
**Re-verified at HEAD `2eba816f`:** `TestAnswerPollerBoundsAnswersPerPoll` (internal/bridge/papdashboard/answers_test.go) still RED — re-run this dispatch, FAIL confirmed; AGENTS.md at 20077/20100 B (23 B headroom, unchanged); TODO_LIST 494 open / 123 ticked (two ticks since the 20-31 count, from the row-110 batch landings); journal `task.reprioritized` count re-queried: **0** (§h trivially empty again).

## The window's five tasks (all re-confirmed via closeout reports / commits)

| Task | Batch | Evidence |
| --- | --- | --- |
| `000001a13db76386…` | ci.yml parity sweep | closeout 2026-10-15_06-10; commit `7e45d620`; asymmetry rows correctly still open on standing preconditions |
| `000001a13dc9b39f…` | O2 token/cost daily cap | DONE-on-arrival close `63e56c34`; ADR-0023; CHANGELOG `--daily-cost-cap` entry (added by the 20-31 pass) |
| `000001a141d63a4b…` | dep-sweep e2e + live calibration | NO closeout (window's worst finding); item 1 landed (`scripts/smoke/dep-sweep.sh`, `internal/depsweep/calibration_live_test.go`); live calib still never executed |
| `000001a1478bb7c8…` | art-dupl batch 3/3 (`-t 3` rerun) | closeout 2026-10-17_19-08; `companion.Enqueue` homing `3990c6e9` |
| `000001a14ade18eb…` | art-dupl batch 4/4 (unix-milli invariant, cmd/tq go.work, heal `--retitle`) | closeout 2026-10-17_19-29; `30e86aba`, `0f0e8167`, `c3cf5191`; AGENTS.md bullet verified present at HEAD |

## a) FULLY DONE (re-verified at `2eba816f`)

1. All seven §a verdicts of the 20-31 report stand unrotted: ci.yml parity wiring, cost-cap loop closed (row 278), `companion.Enqueue` clone homed, status-window flake two-layer root cause + AGENTS.md invariant bullet, journal-drift audit honestly BLOCKED (row 321, M4 memo gate), cmd/tq plain workspace path, `heal-daemon-sweep.sh --retitle` live-proven.
2. **Delta since 20-31 (5 commits, all docs/CI-tooling, none from this window's tasks):** the row-110 follow-up batch (task `000001a14b22c768…`) shipped `scripts/check-index-phrase-drift.sh` as an advisory WARN ci-local step (`d98818e8`), filed its baseline-triage and hard-gate-flip follow-up rows (`137ca432`), annotated the falsified 05-47 §4 premise (`a890c406`), and closed out (`2eba816f`). This directly addresses the 19-54 report's index-drift finding — the window's fallout is being actively fixed by subsequent batches.

## b) PARTIALLY DONE (unchanged verdicts, re-confirmed)

1. Dep-sweep batch (task `…a4bb`): still 1/3 landed, zero rows ticked, NO closeout — the missing-closeout row stands.
2. `TestAnswerPollerBoundsAnswersPerPoll` RED ~43 h and counting — re-run FAIL this dispatch; row 323 + the BLOCKED drain-contract question own it.
3. Plain cmd/tq path verified but ungated (rows 346-348 open).
4. AGENTS.md at 20077/20100 B — budget-policy ruling row open.
5. Phrase-drift guard advisory-only: 10-WARN baseline (paraphrase quotes) must be triaged to zero before the hard-gate flip; both rows minted by `137ca432` (the flip row cites TODO row 313).

## c) NOT STARTED (unchanged)

M4 ratification memo (owner); questions-loop webui/CLI surfaces (row 282); review-anchor residue (row 283); general art-dupl accept-list gate (row 318, BLOCKED); dead-SHA cure pack (rows 779/802); index-bloat archive sweep (README index >100 live rows, re-pardoned again this pass); `tq mcp` stdio server (row 427); live `--dep-sweep` calibration execution (`TQ_DEPSWEEP_LIVE_CALIB=1` never set).

## d) TOTALLY FUCKED UP (carried; nothing new this dispatch)

1. Task `…a4bb`'s seven pushed lying-subject daemon sweeps — permanent per history policy; owner ruling still BLOCKED.
2. AnswerPoller red ~43 h — re-confirmed.
3. Standing reds unchanged: guard-wiring orphan (`smoke/worktree.sh`), dead-SHA gate (~394 dangling cites), index bloat, phrase-drift 10-WARN baseline.
4. This dispatch introduced no regressions (docs-only).

## e) WHAT WE SHOULD IMPROVE

1. **Third dispatch of the same done-prompt id in under two hours** — the resume path worked (this pass cost ~10 min), but a done-prompt re-firing three times suggests the verdict/acceptance signal between the queue and the done-prompt worker is not closing deterministically. The BLOCKED verdict-bar question (19-54 §f) is the fix; it needs the owner.
2. Everything else in 20-31 §e stands: resume path as default, mechanical landed-but-unticked sweep, CHANGELOG checks against new CLI flags, batch-completion mechanics as top structural debt.

## f) NEXT THINGS

**No new rows minted this dispatch.** Every candidate dedupes against the open list: the phrase-drift triage/flip rows were minted by `137ca432` minutes before this pass; poller (row 323), depsweep rows 281-283 + missing-closeout row, go.work gating rows 346-348, dead-SHA rows 779/802, M4 chain (row 321) — all already open. Highest-value unblocks: answer the three BLOCKED owner questions, execute the live dep-sweep calibration, triage the phrase-drift WARN baseline.

## g) QUESTIONS

No new ones. All three of this window's owner questions remain parked in TODO_LIST as BLOCKED items: poller drain-order contract, pushed lying-subject commit policy, done-prompt re-dispatch verdict-recording bar.

## h) BAND DRIFT

**None recorded.** Journal `task.reprioritized` count re-queried this dispatch: **0** all-time. No priority moves exist to account for; ADR-0015's accountability section stays trivially empty until the first reprioritization lands. Dispatch ordering remains harvest-order + lease-claim.
