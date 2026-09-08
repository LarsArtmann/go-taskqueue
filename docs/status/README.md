# Status Reports — Index

Point-in-time session reports, newest first. Each is a snapshot of what a
session did, verified, and left open; they are historical records, not
living documents (re-verify before treating any claim as current).

Every `docs/status/*.md` file must have a row here — `scripts/check-status-index.sh`
(wired into `scripts/ci-local.sh`) fails on unindexed reports.

| Date       | Report                                                                         | Scope                                                          |
| ---------- | ------------------------------------------------------------------------------ | -------------------------------------------------------------- |
| 2026-09-08 | `2026-09-08_23-48_round8-systemnix-evo-x2-integration-execution.md`             | SystemNix/Evo-x2 integration (NixOS module, flake wiring)      |
| 2026-09-08 | `2026-09-08_23-10_round9-dogfood-pool-relaunch-first-live-review-window.md`     | Round-9 dogfood relaunch: full loop + first live review approve |
| 2026-09-08 | `2026-09-08_22-42_fleet-onboarding-agent-pool-integration-status.md`            | Fleet onboarding (discovery daemon, watch SSE)                 |
| 2026-09-08 | `2026-09-08_21-51_round8-status-loop-hardened-dogfooded-and-self-reviewed.md`   | Round-8: status loop hardened + self-reviewed                   |
| 2026-09-08 | `2026-09-08_21-40_first-full-dogfood-window-22-agent-tasks.md`                  | First full dogfood window (22 agent tasks, 21:40 report)       |
| 2026-09-08 | `2026-09-08_21-22_round6-closing-chores-and-self-review.md`                     | Round-6 closing chores + self-review                           |
| 2026-09-08 | `2026-09-08_21-19_dothings-session-status.md`                                   | "Do things?" execution session                                 |
| 2026-09-08 | `2026-09-08_20-58_round6-execution-watermarks-bus-actors.md`                    | Round-6 execution (watermarks, consumer bus, actors)           |
| 2026-09-08 | `2026-09-08_20-56_round7-status-loop-shipped.md`                                | Round-7: the automated done-prompt loop shipped                |
| 2026-09-08 | `2026-09-08_19-59_bootstrap-command-session.md`                                 | `tq bootstrap` one-command meta-harness bootstrap              |
| 2026-09-08 | `2026-09-08_17-21_round5-brutal-self-review-and-full-status.md`                | Brutal self-review + full status (defect list, 50 next items)  |
| 2026-09-08 | `2026-09-08_17-20_round5-complete-m11-m27.md`                                  | Round-5 execution complete (M11–M27)                           |
| 2026-09-08 | `2026-09-08_16-20_round5-m14-dogfood-ops.md`                                   | M14: agent-commit review, session-status script, budget alerts |
| 2026-09-08 | `2026-09-08_15-30_round5-wave2-m7-m10-shipped-m11-started.md`                  | Wave 2 M7–M10 shipped, M11 audit                               |
| 2026-09-08 | `2026-09-08_07-48_agent-reviews-ship.md`                                       | Agent reviews feature (executor + sweeper + smoke)             |
| 2026-09-08 | `2026-09-08_05-54_round5-execution-wave1-bounded-reads-pagination-security.md` | Round-5 wave 1 (M1–M6)                                         |
| 2026-09-08 | `2026-09-08_05-24_cordis-samber-evaluation-journal-bus-recommendation.md`      | Library evaluation (cordis, samber/do)                         |
| 2026-09-08 | `2026-09-08_04-19_agent-pool-e2e-self-review.md`                               | Agent-pool e2e self-review                                     |
| 2026-09-07 | `2026-09-07_23-08_webui-templ-components-redesign-ledger-and-lamp.md`          | Web UI redesign (templ-components)                             |
| 2026-09-07 | `2026-09-07_21-44_prompt-contracts-hardcore-review-TQ_RESULT-guardrails-shipped.md` | Prompt contracts: hardcore review, TQ_RESULT guardrails     |
| 2026-09-07 | `2026-09-07_21-19_round4-dogfood-pool-first-completions.md`                     | Round-4 dogfood: first TODO completions, LAN dashboard         |
| 2026-09-07 | `2026-09-07_20-32_docs-health-audit-six-docs-rebuilt-ghost-contract-fixed.md`   | Docs-health audit: living docs rebuilt, ghost contract fixed    |
| 2026-09-07 | `2026-09-07_19-33_master-ci-green-ci-policy-tooling-debt.md`                    | Master CI green; CI policy decided, tooling debt surfaced      |
| 2026-09-07 | `2026-09-07_18-41_round3-live-web-ui-executed-verified-committed.md`           | Round-3 live web UI executed and verified                      |
| 2026-09-06 | `2026-09-06_19-49_round2-complete-c14-c15-c19-c27-status-and-debt.md`          | Round 2 complete: tq top, audit, tooling policy                |
| 2026-09-06 | `2026-09-06_18-34_round2-session-status-c06-c13-c16-c18-done-ci-incident.md`   | Round-2 slices 1–2 + CI incident                               |
| 2026-09-06 | `2026-09-06_18-00_round2-execution-slice1-released-slice2-hardening.md`        | Round-2 slice 1 shipped, slice 2 hardening                     |
| 2026-09-06 | `2026-09-06_16-19_docs-health-audit-six-docs.md`                               | Docs-health audit of the six living documents                  |
| 2026-09-06 | `2026-09-06_15-45_self-managing-agent-pool.md`                                 | Self-managing agent pool: first session report                 |

Earlier round-1 sessions (2026-09-05/06) live in `docs/planning/` alongside
their plans; the round-2 report series above is where the report convention
stabilized.
