# Status Report — Self-Managing Agent Pool (go-taskqueue)

**Date:** 2026-09-06 15:45 CEST
**Session scope:** Build the "self-managing pool of Crush agents that eats all TODOs/backlogs and manages cross-repo work" on top of go-taskqueue. Report covers THIS session only.
**Co-development context:** A parallel agent session worked the same repo simultaneously the entire time (dedup keys, agent executor, CQA/PapDashboard bridges, CLI renames). Several features below are joint work — I note which parts were mine.
**Verification state at report time:** `go build ./...` ✅, `go vet ./...` ✅, `go test ./... -race` ✅ (all packages), golangci-lint ✅ for session-owned files (pre-existing warnings remain in parallel-session files), working tree clean (auto-commit daemon).

---

## a) FULLY DONE

1. **The self-managing loop exists and is proven with a real crush agent.**
   End-to-end smoke (2026-09-06 15:37, sandbox repo): harvest enqueued a TODO item → headless crush agent created the file, marked the item `[x]`, committed ("Add hello.txt with smoke test text and close TODO item") → task completed → next tick idle. 44 s enqueue-to-done. Exactly the contract.
2. **`internal/harvest` (mine)** — TODO_LIST.md → agent tasks: markdown checkbox parsing (code-fence aware, heading context, whitespace-collapsed stable dedup keys), per-repo pacing (≤1 in-flight item/repo), global `--max-per-tick` cost cap, DLQ/cancelled/busy skip reasons, `DryRun`. 8 unit tests + full-loop e2e test with a stub agent.
3. **CLI (joint)** — `tq harvest [--dry-run --allow-dirty --max-per-tick]` and `tq agent-pool [--yolo --interval --concurrency --task-timeout --cqa-url ...]`. I wrote the first `agent-pool`/`harvest` wiring, then consolidated with the parallel session's `supervise` implementation (theirs won the drain-on-shutdown structure; command name converged to `agent-pool`). Duplicate switch cases and dead helpers removed.
4. **FIXED: `--yolo` killed every autonomous run (mine).** `crush run` has NO yolo flag (v0.92: "Unknown flag: --yolo") — the executor injected it into every run; all yolo tasks failed instantly and dead-lettered. Root-caused via live DLQ inspection + manual crush flag experiments. Autonomy now = repo-local `.crushrc` (`permissions allow view ls grep glob edit write bash`) — verified empirically (agent created a file headlessly, exit 0). Pool cannot over-grant what a repo never offered. Fail-fast guard + guidance when yolo is requested but no repo-local crush config exists.
5. **FIXED: 30-second execution cap on ALL tasks (mine).** The worker's shutdown-drain timer leaked into every task's execution context: any task >30 s died with "context deadline exceeded", and after 30 s of pool uptime every subsequent task failed instantly. This made agent pools (minutes-long tasks) categorically impossible. Tasks (execution + heartbeats + terminal writes) now run under a shutdown-surviving context bounded only by `--task-timeout`; graceful stop lets in-flight agents finish.
6. **Argv-contract regression test (mine)** — pins the exact `run --quiet --cwd … [--model …] -- PROMPT` command line; the stub-based tests can't catch flag-order regressions (that gap is exactly how the yolo bug shipped).
7. **Docs (joint)** — README agent-pool section (quickstart + safety rails) was parallel-session work; I corrected the autonomy bullet to the verified `.crushrc` mechanism and added the snippet. CHANGELOG Fixed/Changed entries for both bugs. **`TODO_LIST.md` created (mine)** — this repo now dogfoods: its own unchecked items are pool food.
8. **Consolidation without collisions** — my redundant `CrushExecutor` deleted (`git rm`), harvest rewired onto the stronger `AgentExecutor` (verify gate + clean-tree guard + process-group kill, parallel session's), payload schema converged (`AgentPayload` + harvest `dedup` field). Zero reverts of their work; zero lost work.

## b) PARTIALLY DONE

1. **Per-repo serialization** — enforced only via harvester pacing (≤1 in-flight harvested item/repo). Manually enqueued tasks, or a second pool with different harvest configs, can still co-run agents in one repo. Store-level `WithProjectExclusivity` claim guard was designed (SQL sketched) but abandoned to avoid colliding with the parallel session; unimplemented.
2. **Cross-repo todo management** — shared queue + per-project stats exist, DAG `deps` exist for manual orchestration, but harvest never creates cross-repo dependencies (e.g. "docs repo item depends on code repo item"). The user's cross-repo vision is ~half real.
3. **Safety model** — clean-tree guard + verify gate + repo-local autonomy are in, but: `requireRepoAutonomy` false-positives on repos that rely on user-global crush permissions (no repo-local file); no daily cost cap; retries of a "repo is dirty" task burn attempts pointlessly (permanent error retried as transient).
4. **Verify step** — auto-detects Go/npm only; other stacks get NO verification unless a payload `verify` is set. Harvested tasks don't set one.
5. **Observability** — `tq facts`/`stats`/DLQ work, but no link from a completed task to the crush session/transcript, no per-repo last-run-duration view, no "completed but item still unchecked" drift detector.

## c) NOT STARTED

1. **ADR for the agent-pool architecture** (`docs/adr/0002-…`) — planned at session start, never written; the autonomy/trust model (repo-local `.crushrc`) and the drain-context semantics deserve a decision record.
2. **Nix gates** — `nix build` / `nix flake check` never run this session (go-only verification). AGENTS.md warns about vendorHash drift; likely fine (no go.mod changes) but unverified.
3. **Multi-repo / multi-pool contention testing** — smoke was 1 repo, concurrency 1. The one-in-flight-per-repo invariant under N repos × M pools is unit-tested only, never exercised live.
4. **Any systemd/user-service unit or supervisor config** for running `tq agent-pool` continuously (the "at all times" part currently means "in a terminal or under timeout").
5. **Cost telemetry** — no token/cost accounting per task/repo anywhere.

## d) TOTALLY FUCKED UP!

Nothing is fucked up now (suite green, loop proven). But three things genuinely went wrong mid-session and were only caught by verification:

1. **I nearly shipped a corrupted edit.** My first `main.go` multiedit contained garbled stub code (`*int0` type, dummy functions). It was rejected only because the parallel session had touched the file's mtime — luck, not process. Had it applied, the build would have broken confusingly.
2. **Split-brain duplication happened twice before I caught it.** I wrote `CrushExecutor` while the parallel session wrote `AgentExecutor` (same purpose, same package), and later `cmdHarvest` while they wrote their own — the duplicate was only spotted via a `duplicate case "harvest"` compile error. I designed and built without first checking for in-flight parallel work.
3. **I trusted green tests over the real binary contract.** The stub-based executor tests pass while argv was wrong (stubs ignore arguments). The first two smoke runs burned the full attempt budget (6 crush invocations) on a bug a 5-second manual `crush run --yolo` would have exposed. I verified `--quiet/--cwd` early but never re-verified `--yolo` after it entered the arg path.

## e) WHAT WE SHOULD IMPROVE!

1. **Verify the external CLI contract before wiring flags** — every flag passed to an external binary needs an empirical check (or a contract test pinning it, which now exists for crush).
2. **Check for parallel work BEFORE designing, not after collisions** — `grep`/`git log`/`git status` first; this session lost ~5 tool rounds to rework after mid-air collisions (harvest Config flip-flopped restore→strip→restore).
3. **Small edits, immediate builds** — the times I batched big multiedits were exactly the times garbage slipped in (`} }`, `falseVal`, `jsonUnmarshal`).
4. **Long-task semantics need a dedicated test** — the 30 s drain-context cap survived every existing worker test because all test tasks were <80 ms. One test with a multi-second task under a pool that outlives 30 s would have caught it years early. (Same class: nothing tests a task claimed after pool uptime >30 s.)
5. **Fail-fast preflight beats retry-burning** — the `requireRepoAutonomy` pattern (validate precondition before spending an agent run) should extend to: dirty-tree (currently burns attempt 1 to discover it), missing verify toolchain, missing TODO file.
6. **Transient-vs-permanent error classification** is absent: "repo dirty" and "unknown flag" retry exactly like network blips.
7. **The auto-commit daemon hides mid-edit states into history** — add+delete churn for my executor; consider squashing before v0.1.0 tag.

## f) Up to 50 things to get done next

Top block (actionable now, high impact):

1. ~~Store-level `WithProjectExclusivity` claim guard (opt-in) — per-repo serialization across ALL pools/processes, not just harvest pacing~~ done (shipped in v0.1.0 (WithProjectExclusivity, FEATURES queue-core row))
2. ~~Permanent-vs-transient error classes: dirty-tree/unknown-flag/missing-config dead-letter after ONE attempt, no backoff burn~~ done (shipped in v0.1.0 (executor.PermanentError, dead-letter after one attempt))
3. ~~`--model` pass-through: `tq agent-pool --model provider/model` → `AgentPayload.Model` (field exists, wiring missing)~~ done (shipped in v0.1.0 (--model on harvest + agent-pool))
4. ~~ADR-0002: agent-pool architecture + autonomy/trust model + drain semantics~~ done (docs/adr/0002-agent-pool-autonomy-pacing-drain.md)
5. ~~Long-task regression test: task claimed after pool uptime >30 s completes (would have caught the drain bug)~~ done (shipped in v0.1.0 (long-task regression test pins the drain fix))
6. ~~Multi-repo live smoke: ≥3 repos, concurrency 2, two agent-pool processes on one DB (dedup + pacing under real contention)~~ done (scripts/smoke/multi-repo.sh (3 repos, 2 pools, 1 DB))
7. ~~systemd user unit (or `tq agent-pool --daemon`) for the "at all times" requirement; Restart=on-failure~~ done (deploy/systemd/tq-agent-pool.service + tq agent-pool --once)
8. ~~Daily/rolling cost budget per repo and global (agent runs cost real money; `--max-per-tick` is per-tick only)~~ done (shipped in v0.1.0 (--daily-budget/--budget-cmd, internal/budget))
9. ~~`requireRepoAutonomy` false-positive fix: probe global permissions config or add `--assume-global-autonomy` escape~~ done (autonomy probe accepts the user-global crush config)
10. ~~Harvest: per-repo poll interval + DLQ backoff (a poisoned repo must not refill attempts forever)~~ done (shipped in v0.1.0 (--repo-interval + --dlq-backoff))
11. ~~Link completed agent tasks to crush session IDs (payload result detail) so `tq show` points at the transcript~~ done (session id recorded in task.completed, rendered by tq show)
12. ~~Docs-drift auditor: periodic task re-checking harvested repos for "completed but still unchecked" items~~ done (tq audit (internal/harvest/drift.go))
13. ~~`tq top`: live per-project view (pending/running/dead + last agent duration)~~ done (tq top)
14. ~~`TestShutdownDrains` claim-window flake fix (30 ms sleep → explicit claim await)~~ done (TestShutdownDrains awaits claims explicitly)
15. ~~E2E CLI-as-subprocess test: spawn `tq agent-pool` with `$TQ_AGENT_BIN` stub from outside the process~~ done (internal/e2e subprocess suite, runs in CI)
16. ~~Verify-step auto-detection beyond Go/npm (Makefile, flake.nix `nix build`, cargo, pip) or per-repo `.tq-verify` file~~ done (.tq-verify wins; Makefile/flake.nix/cargo auto-detect)
17. ~~Harvested tasks should carry a `verify` command sourced from repo config~~ done (harvest pins the repo .tq-verify into every payload)
18. ~~`nix build` + `nix flake check` in this session's follow-up (vendorHash dance if needed)~~ done (vendorHash re-pinned; nix build + nix flake check green (16:19 report))
19. ~~Pre-v0.1.0: history squash decision for the daemon's mid-edit commits, then tag + release + pkg.go.dev~~ done (tag v0.1.0 + GitHub pre-release, proxy + pkg.go.dev verified (2026-09-06/07))
20. Cancelled-task dedup semantics: decide whether `dedup_key` should ignore cancelled rows (re-open support) — schema-affecting

Second block (hardening/polish):
21. Cross-repo DAG from harvest (configurable: "docs item depends on code item" templates)
22. `tq agent-pool --once` (single harvest+drain pass; scripts/tests/schedulers)
23. Structured per-task result payload: {files_changed, commit_sha, verify_output_tail}
24. Heartbeat cadence scaled to lease for very long tasks (alert if lease renewal approaches RTT)
25. Output tail truncation is 4–8 KB; store full agent stdout to a sidecar file, reference by path
26. `tq harvest --json` for dashboards
27. Rate-limit concurrent crush sessions per machine (crush itself may not like N parallel instances)
28. `.crushrc` permissions lint: warn when a repo grants `bash` but pool runs without sandboxing
29. Git worktree isolation option for agents (never touch the user's checkout)
30. PR-mode: agent commits to a branch + opens PR instead of committing to master
31. Session continuation: `AgentPayload.Session` exists — wire "follow-up on previous item" chains
32. Timeout defaults per repo size (small repos don't need 45 m)
33. `tq dlq --rescue-all --older-than` bulk rescue
34. Metrics endpoint (Prometheus) over the facts projection
35. `tq tail -f` → SSE/PapDashboard fan-out (already on ROADMAP v0.3)
36. Property test: dedup keys stable under whitespace reflow, unique across same-text repos
37. Fuzz the TODO parser (malformed markdown, CRLF, BOM)
38. Windows path handling in harvest (repo-name keys, separators) — untested platform
39. i18n-safe item extraction (non-ASCII TODO text hashing)
40. `internal/` → public packages decision (ADR-0002 follow-up: importable library API before external users)
41. Example corpus: `examples/agent-pool/` runnable demo repo with `.crushrc` + TODO_LIST.md
42. Docs: SECURITY.md (what autonomy grants mean, blast radius of `bash` permission)
43. Guard: refuse `--projects-dir /` or `$HOME` (harvest scanning catastrophically wide)
44. `tq harvest --repo-subset` glob filter (296 mirrors → curated subset)
45. Queue DB rotation/backup guidance (single file = single point of failure)
46. Chaos test: SIGKILL a pool mid-agent-run; assert lease-expiry reclaim + no double-complete
47. GitHub Actions job running the stub-agent e2e (no API cost, catches CLI regressions)
48. `crush` version detection at pool start (flag contract drift early-warning, fail with guidance)
49. Teardown: clean `/tmp/tq-smoke`, `/tmp/tq`, `/tmp/tq-crush-test` artifacts from this session
50. Decide: should the pool manage go-taskqueue itself (needs `.crushrc` here — see questions)

(Items 1–20 are TODO_LIST.md-grade; 21–50 are ROADMAP fuel — TODO_LIST.md already carries the actionable subset.)

## g) Questions I can NOT figure out myself

1. **Should the pool be allowed to work on go-taskqueue itself?** This repo now has a TODO_LIST.md full of pool food, but no `.crushrc` — deliberately, because 5+ concurrent agents already edit this repo. Adding one means the pool spawns MORE agents into an already-contended working tree. Your trust/chaos call, not mine.
2. **What are the cost limits for a first production run?** Agent tasks spend real money per run. Give me a ceiling (e.g. €X/day, or N agent-runs/day, repos included) and I'll wire the budget guardrails instead of guessing.
3. **Cancel semantics for dedup keys: permanent parking or re-openable?** Today a cancelled task's dedup key suppresses re-enqueue forever (escape hatch: edit the item text). Should cancellation instead release the key (auto re-open on next harvest), accepting that "cancel" no longer means "stop bringing this back"? It changes store semantics, so it's your product decision.
