# Status Reports — Index

Point-in-time session reports, newest first. Each is a snapshot of what a
session did, verified, and left open; they are historical records, not
living documents (re-verify before treating any claim as current).

Every `docs/status/*.md` file must have a row here — `scripts/check-status-index.sh`
(wired into `scripts/ci-local.sh`) fails on unindexed reports. Reports whose
every forward-looking item is resolved (inline strikethroughs) move to
`archived/` — the row above points at the archived path.

Archive counter (update when moving files): 1 report in `archived/`,
6 plans in `docs/planning/archived/` (2026-09-09).

| Date       | Report                                                                              | Scope                                                                                             |
| ---------- | ----------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- |
| 2026-09-10 | `2026-09-10_06-39_task-000001a0893f0253e25fe15e759f78a899ed.md`                     | Review-finding close-out: fullcore drain-loop select race fixed (16a15d8, redundant done case removed, 10/10 deadline determinism proven by hand, gates green); honest gaps: sqlite-only manual verification, log.Fatal skips defers, no automated test yet; 50 next items + 3 owner questions |
| 2026-09-10 | `2026-09-10_06-25_fullcore-window-five-review-fixes-nix-gate-red.md`                 | Window: fullcore example (run live, sqlite) + four review-fixes verified in HEAD; nix CI gate RED on runners since the window's push (runner-only FOD hash mismatch, exact failed drv green locally); 6 new TODO items + 3 owner questions; docs-health pass annotated 4 reports, backfilled CHANGELOG, synced FEATURES/README/AGENTS |
| 2026-09-10 | `2026-09-10_05-58_task-000001a0893f0251882bc5b9b65979337826.md`                     | Review-fix task: README examples paragraph now mentions examples/fullcore (b31f7fd, gates green); review pipeline leaks owned — dangling review target c4e8b9f vs master twin 2d3340a, stale line anchors, footer-vs-auto-commit-daemon collision; 50-item follow-up brainstorm + 3 owner questions |
| 2026-09-10 | `2026-09-10_05-30_loop-engine-built-task-closeout-and-docs-health.md`                | Owner's loop system built: --task-closeout (two-turn brutal self-review per task) + docs-health status prompt; workforce 18 done/0 dead; closeout live-proof + full gate still pending; 2h timeslip + wrong-first zombie diagnosis owned |
| 2026-09-10 | `2026-09-10_05-29_readme-overhaul-tqdb-incident.md`                                  | README rewritten from verified facts (model-contradiction fix, 19-command map, status-every/prune/journal-browser docs); TQ_DB production incident contained + AGENTS.md trap recorded; LIVE evidence the deployed pool still requeue-loops all agent tasks on git-not-PATH (owner flip pending) |
| 2026-09-10 | `2026-09-10_04-09_scan-jobs-webui-contracts-and-red-master.md`                        | Window f20–f24 verified done (scan jobs, templ audit, webui dedup, stats contract); master CI red since 01:15 with two hard-gate breaks + a real postgres vuln found by the new govulncheck job |
| 2026-09-10 | `2026-09-10_03-05_httputil-assessment-and-permissions-policy.md`                     | httputil assessed, NOT adopted (Proprietary vs MIT + scope mismatch); Permissions-Policy header ported to webui, docs synced, gates green |
| 2026-09-10 | `2026-09-10_02-51_backlog-50-execution-complete.md`                               | 50-item backlog executed: real-ETXTBSY e2e test, payload v-gate, watch knobs, ADR-0013, live-pool verify, all gates green |
| 2026-09-10 | `2026-09-10_02-04_go-retry-adoption-and-dependency-review.md`                       | go-retry adopted for generic retry loops (ADR-0013), 8-module dep inventory, reuse-first verdicts, 50-item plan |
| 2026-09-10 | `2026-09-10_02-00_self-review-dead-pool-fix-flash-dogfood-session.md`               | Self-review: tag restore + dead-pool fix + Flash dogfood proof; handoff push-prerequisite gap owned |
| 2026-09-10 | `2026-09-10_01-55_pool-deployment-fix-and-flash-dogfood-proof.md`                   | Dead systemd pool root-caused (cwd-resolved repo names, missing service PATH, silent skip log); fixed + Flash dogfood loop proven live |
| 2026-09-10 | `2026-09-10_00-41_backlog-execution-and-gate-hardening.md`                          | 23:47 backlog executed: ETXTBSY root-caused, release-allowlist + darwin-eval bugs fixed, conformance gaps closed vs live PG, docs harvested |
| 2026-09-09 | `2026-09-09_23-47_round2-queue-backend-split.md`                                    | Round-2 modularization: queue split into contract + sqlite/postgres driver modules (ADR-0012)     |
| 2026-09-09 | `2026-09-09_23-21_module-split-and-review-sweep.md`                                 | Module split (ADR-0011) + review sweep: go-install fix, release-path repairs, next 50             |
| 2026-09-09 | `2026-09-09_06-01_round10-self-review-and-full-status.md`                           | Round-10 self-review: v0.2.0-ships-crash finding, misses, next 50                                 |
| 2026-09-09 | `2026-09-09_05-20_round10-whole-todo-list-executed.md`                              | Round-10: the whole TODO list executed, v0.2.0 released                                           |
| 2026-09-09 | `2026-09-09_02-52_docs-health-second-order-self-review-four-defects-fixed.md`       | Second-order self-review: four defects in the audit itself found + fixed                          |
| 2026-09-09 | `2026-09-09_02-16_docs-health-audit-annotations-archives.md`                        | Docs-health audit: six docs rebuilt, 12 reports annotated, 7 archived, prune-gap regression found |
| 2026-09-09 | `2026-09-09_01-48_brutal-self-review-and-full-status-after-backlog-sweep.md`        | Brutal self-review + full status after the backlog sweep                                          |
| 2026-09-09 | `2026-09-09_01-43_21-40-backlog-sweep-20-items-two-real-bugs.md`                    | 21:40 backlog sweep: 20 items done, budget-bypass + FactsForTask bugs fixed                       |
| 2026-09-09 | `2026-09-09_01-35_lan-dashboard-redesign-and-admin-writes-status.md`                | LAN dashboard redesign + admin writes (concurrent session)                                        |
| 2026-09-09 | `2026-09-09_00-21_board-view-shipped-concurrent-session-report.md`                  | Board view shipped (concurrent session)                                                           |
| 2026-09-08 | `2026-09-08_23-48_round8-systemnix-evo-x2-integration-execution.md`                 | SystemNix/Evo-x2 integration (NixOS module, flake wiring)                                         |
| 2026-09-08 | `2026-09-08_23-10_round9-dogfood-pool-relaunch-first-live-review-window.md`         | Round-9 dogfood relaunch: full loop + first live review approve                                   |
| 2026-09-08 | `2026-09-08_22-42_fleet-onboarding-agent-pool-integration-status.md`                | Fleet onboarding (discovery daemon, watch SSE)                                                    |
| 2026-09-08 | `2026-09-08_21-51_round8-status-loop-hardened-dogfooded-and-self-reviewed.md`       | Round-8: status loop hardened + self-reviewed                                                     |
| 2026-09-08 | `2026-09-08_21-40_first-full-dogfood-window-22-agent-tasks.md`                      | First full dogfood window (22 agent tasks, 21:40 report)                                          |
| 2026-09-08 | `2026-09-08_21-22_round6-closing-chores-and-self-review.md`                         | Round-6 closing chores + self-review                                                              |
| 2026-09-08 | `2026-09-08_21-19_dothings-session-status.md`                                       | "Do things?" execution session                                                                    |
| 2026-09-08 | `2026-09-08_20-58_round6-execution-watermarks-bus-actors.md`                        | Round-6 execution (watermarks, consumer bus, actors)                                              |
| 2026-09-08 | `2026-09-08_20-56_round7-status-loop-shipped.md`                                    | Round-7: the automated done-prompt loop shipped                                                   |
| 2026-09-08 | `2026-09-08_19-59_bootstrap-command-session.md`                                     | `tq bootstrap` one-command meta-harness bootstrap                                                 |
| 2026-09-08 | `2026-09-08_17-21_round5-brutal-self-review-and-full-status.md`                     | Brutal self-review + full status (defect list, 50 next items)                                     |
| 2026-09-08 | `2026-09-08_17-20_round5-complete-m11-m27.md`                                       | Round-5 execution complete (M11–M27)                                                              |
| 2026-09-08 | `2026-09-08_16-20_round5-m14-dogfood-ops.md`                                        | M14: agent-commit review, session-status script, budget alerts                                    |
| 2026-09-08 | `2026-09-08_15-30_round5-wave2-m7-m10-shipped-m11-started.md`                       | Wave 2 M7–M10 shipped, M11 audit                                                                  |
| 2026-09-08 | `2026-09-08_07-48_agent-reviews-ship.md`                                            | Agent reviews feature (executor + sweeper + smoke)                                                |
| 2026-09-08 | `2026-09-08_05-54_round5-execution-wave1-bounded-reads-pagination-security.md`      | Round-5 wave 1 (M1–M6)                                                                            |
| 2026-09-08 | `2026-09-08_05-24_cordis-samber-evaluation-journal-bus-recommendation.md`           | Library evaluation (cordis, samber/do)                                                            |
| 2026-09-08 | `2026-09-08_04-19_agent-pool-e2e-self-review.md`                                    | Agent-pool e2e self-review                                                                        |
| 2026-09-07 | `2026-09-07_23-08_webui-templ-components-redesign-ledger-and-lamp.md`               | Web UI redesign (templ-components)                                                                |
| 2026-09-07 | `2026-09-07_21-44_prompt-contracts-hardcore-review-TQ_RESULT-guardrails-shipped.md` | Prompt contracts: hardcore review, TQ_RESULT guardrails                                           |
| 2026-09-07 | `2026-09-07_21-19_round4-dogfood-pool-first-completions.md`                         | Round-4 dogfood: first TODO completions, LAN dashboard                                            |
| 2026-09-07 | `2026-09-07_20-32_docs-health-audit-six-docs-rebuilt-ghost-contract-fixed.md`       | Docs-health audit: living docs rebuilt, ghost contract fixed                                      |
| 2026-09-07 | `2026-09-07_19-33_master-ci-green-ci-policy-tooling-debt.md`                        | Master CI green; CI policy decided, tooling debt surfaced                                         |
| 2026-09-07 | `2026-09-07_18-41_round3-live-web-ui-executed-verified-committed.md`                | Round-3 live web UI executed and verified                                                         |
| 2026-09-06 | `2026-09-06_19-49_round2-complete-c14-c15-c19-c27-status-and-debt.md`               | Round 2 complete: tq top, audit, tooling policy                                                   |
| 2026-09-06 | `2026-09-06_18-34_round2-session-status-c06-c13-c16-c18-done-ci-incident.md`        | Round-2 slices 1–2 + CI incident                                                                  |
| 2026-09-06 | `2026-09-06_18-00_round2-execution-slice1-released-slice2-hardening.md`             | Round-2 slice 1 shipped, slice 2 hardening                                                        |
| 2026-09-06 | `2026-09-06_16-19_docs-health-audit-six-docs.md`                                    | Docs-health audit of the six living documents                                                     |
| 2026-09-06 | `archived/2026-09-06_15-45_self-managing-agent-pool.md`                             | Self-managing agent pool: first session report (fully resolved, archived 2026-09-09)              |

Earlier round-1 sessions (2026-09-05/06) live in `docs/planning/` alongside
their plans; the round-2 report series above is where the report convention
stabilized.
