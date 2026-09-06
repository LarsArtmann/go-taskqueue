# TODO List

Short- and mid-term actionable work. Long-term direction lives in ROADMAP.md.

**This file is machine-consumed**: `tq harvest` turns every unchecked item
below into an agent task. Keep the `- [ ]` checkbox format, one item per
line — do not convert to tables. Mark done items `[x]` or delete them;
appending `— BLOCKED: <reason>` keeps an item out of the pool.

## High Impact

- [x] Permanent-vs-transient error classes: dirty tree, missing autonomy config, and unknown flags must dead-letter after ONE attempt instead of burning the retry budget like transient errors (executor can wrap a permanent-error type the worker honors)
- [x] Store-level per-project claim exclusivity (opt-in `WithProjectExclusivity` on the SQLite store): at most one running task per project across ALL worker processes, so multi-pool deployments get per-repo serialization without relying on harvester pacing alone
- [x] Long-task regression test: a task claimed after the pool has been up for minutes completes and records its outcome (guards the fixed drain-context bug; every existing worker test uses sub-100ms tasks)
- [ ] ADR-0002: agent-pool architecture — autonomy/trust model (repo-local `.crushrc`), pacing vs exclusivity, drain-context semantics
- [x] Daily/rolling cost budget per repo and global: agent tasks cost real money; `--max-per-tick` bounds a single harvest tick only (shipped: global `--daily-budget`, `--budget-cmd`, per-repo `--repo-interval` pacing)
- [x] v0.1.0 release prep: decide on squashing the auto-commit daemon's mid-edit history, then tag, GitHub release, pkg.go.dev surface — shipped 2026-09-06 as a pre-release, history kept unsquashed; pkg.go.dev indexes on the proxy's own crawl schedule

## Medium Impact

- [x] Pass a model override through `tq agent-pool --model` into agent payloads (`AgentPayload.Model` exists; harvest/CLI wiring does not)
- [x] `requireRepoAutonomy` false-positive fix: probe user-global crush permissions or add an explicit escape flag, so repos relying on global config are not refused
- [x] Multi-repo live smoke: ≥3 repos, concurrency 2, two agent-pool processes on one DB — dedup + pacing under real contention (`scripts/smoke/multi-repo.sh`)
- [x] systemd user unit (or `tq agent-pool --daemon`) so the pool runs continuously instead of in a terminal under `timeout` (`deploy/systemd/tq-agent-pool.service`)
- [x] Verify-step auto-detection beyond Go/npm (Makefile, flake.nix, cargo) or a per-repo `.tq-verify` file; harvested tasks currently get no verify on other stacks
- [x] Harvested tasks should carry a `verify` command sourced from repo config instead of relying on executor auto-detection (`.tq-verify` is pinned into every payload)
- [x] Harvester: per-repo poll interval, and back off repos whose items repeatedly land in the DLQ (a poisoned repo should not refill its attempt budget forever)

## Lower Impact

- [x] CI reliability bundle: nix build + `nix flake check` job, TODO_LIST harvest-parse guard (`tq harvest --repos . --dry-run` must succeed), ghost-reference check that every path cited in AGENTS.md/README/FEATURES exists (plan C05)
- [ ] Verify the CQA bridge against a live CQA API instance and fix contract drift (`internal/bridge/cqa` response shapes are httptest-informed guesses today); upgrade its FEATURES.md status after (plan C25)
- [ ] Re-runnable PapDashboard E2E verification script (docker pap + `tq worker --alert-url`) so the bridge's FULLY_FUNCTIONAL status is provable on demand (plan D73)
- [ ] `docs/DOMAIN_LANGUAGE.md`: glossary for task, fact, claim, lease, release, DLQ, rescue, harvest, dedup key, tick, verify gate (plan C20)
- [ ] `doc.go` package docs for queue/worker/executor/harvest/journal + godoc examples before the module goes public (plan C22)
- [ ] Tooling policy: golangci-lint either CI-gated with errcheck exclusions for idiomatic deferred Close or dropped from CONTRIBUTING; dprint added to the flake devShell and run over the living docs (plan C26)
- [x] `tq top`: live per-project view (pending/running/done/dead + last agent run duration) over the existing facts
- [x] Record agent transcript location (crush session id) as task result detail on completion, so `tq show` links to the agent's session
- [ ] Docs-drift auditor: periodic task re-checking harvested repos and enqueuing a catch-up item when an item is done in code but still unchecked
- [x] `TestShutdownDrains` 30 ms claim window flakes under heavy parallel-agent load; await claims explicitly instead of sleeping
- [x] E2E CLI test: spawn `tq agent-pool` as a subprocess with a stub agent binary (`$TQ_AGENT_BIN`) and assert the full loop from outside the process
- [x] Property test: harvest dedup keys are stable under whitespace reflow and unique across repos with identical item text
