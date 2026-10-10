# Agent-pool flag manifest (E2.1, config-system plan 2026-10-09)

Generated from `tq agent-pool -h` (55 registered flags; the plan's "53"
predates C4's `--store`). The config-file KEY vocabulary IS the flag
spelling (no dashes) — `tq agent-pool --config <file>` accepts any row
below as `key = value`; precedence is **flag > env > file > built-in
default**, unknown keys error loudly (cmd/tq/poolconfig.go, 8 parser
tests). The env column marks flags whose DEFAULT reads the environment
at registration time — for those, a live env setting beats the config
file.

`scripts/check-flag-count.sh` (E3, wired into ci-local) fails when any
command's registration count drifts from
`scripts/flag-count-baseline.txt`; a legit new flag updates the
baseline with `--update` and names itself in the commit.

| Flag (config key) | Env default |
| ----------------- | ----------- |
| `alert-api-key` | — |
| `alert-poll` | — |
| `alert-url` | — |
| `allow-dirty` | — |
| `batch-items` | — |
| `budget-cmd` | — |
| `concurrency` | — |
| `config` | — |
| `cqa-owner` | `CQA_OWNER_ID` |
| `cqa-token` | `CQA_TOKEN` |
| `cqa-url` | `CQA_URL` |
| `daily-budget` | — |
| `db` | — |
| `dead-pool-ticks` | — |
| `dep-sweep` | — |
| `dep-sweep-bin` | — |
| `dep-sweep-dir` | — |
| `dep-sweep-interval` | — |
| `dep-sweep-push` | — |
| `dep-sweep-unreleased` | — |
| `discovery-addr` | — |
| `dlq-backoff` | — |
| `dlq-fix` | — |
| `done-preflight` | — |
| `interval` | — |
| `lease` | — |
| `log-dir` | `TQ_LOG_DIR` |
| `log-dir-max-age` | — |
| `log-dir-max-bytes` | — |
| `max-concurrent-agents` | — |
| `max-pending-per-repo` | — |
| `max-per-tick` | — |
| `model` | — |
| `once` | — |
| `owner` | — |
| `poll` | — |
| `prioritize` | — |
| `priority-from` | — |
| `project-exclusive` | — |
| `projects-dir` | — |
| `prune-stale` | — |
| `redact` | `TQ_REDACT` |
| `repo-interval` | — |
| `repo-timeout` | — |
| `repos` | — |
| `reprioritize` | — |
| `reresolve-verify` | — |
| `review` | — |
| `review-autofix` | — |
| `starvation-after` | — |
| `status-every` | — |
| `store` | — |
| `task-closeout` | — |
| `task-timeout` | — |
| `yolo` | — |
