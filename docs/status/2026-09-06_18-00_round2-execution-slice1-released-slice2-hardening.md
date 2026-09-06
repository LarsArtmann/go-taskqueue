# Session Execution Report — Round-2 Plan: Slice 1 Shipped, Slice 2 Nearly Done

**Date:** 2026-09-06 18:00 CEST
**Session scope:** execute docs/planning/2026-09-06_16-28_SUPERB-PLAN-ROUND2 (C01–C27,
D01–D100) end to end: READ → execute → verify per task, one step at a time.
**State at writing:** HEAD `ea09aa2`, all local gates green
(vet/build/test-race/gofmt/nix flake check), CI green on `v0.1.0` (run
34041720404), **v0.1.0 released**. Working tree clean.

---

## a) FULLY DONE (verified, not just written)

**Slice 1 — Correctness + Release Gate (complete):**

| Task                           | What shipped                                                                                                                                                                                                                                                                                                                                                 | Verify actually run                                                                                                                                                                                          |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| C01 error classes (D01–D05)    | `executor.PermanentError` + `Permanent()`; agent input-contract misses, unknown task type, non-zero `sh` exit, permanent HTTP statuses (4xx except 408/429) classified permanent; worker branch dead-letters after ONE attempt; `Store.FailPermanent`; dead-letter facts carry `Error` + `{"class":"permanent"/"exhausted"}`; `tq facts` renders `[class=…]` | unit tests; mutation test (revert = fail); live CLI smoke: MaxAttempts=9 task dead after 1 attempt, class visible in `tq facts`                                                                              |
| C02 regression tests (D06–D08) | `TestLongTaskCompletesAcrossShutdown` (drain-bug regression), `TestShutdownDrains` de-flaked (awaits first claim; asserts every claimed task terminates), worker package race-run                                                                                                                                                                            | mutation test: reverting `WithoutCancel` makes the test FAIL; `-race -count=5` green                                                                                                                         |
| C03 exclusivity (D09–D13)      | `queue.WithProjectExclusivity()` store option; claim guard skips projects with a running task (cross-handle, reclaim-safe, empty-project exempt); `--project-exclusive` on worker + agent-pool; sibling claimable again after completion                                                                                                                     | order-independent unit tests incl. two-store-handles "multi-process" test                                                                                                                                    |
| C05 CI bundle (D19–D21)        | CI: nix build + flake check job (pinned `cachix/install-nix-action@13d8dd5…` v31.11.1); TODO_LIST harvest-parse guard (`TestRepoTodoListParses`); ghost-reference check (`scripts/check-doc-refs.sh`, six living docs, allowlist)                                                                                                                            | both guards mutation-tested (planted ghost path fails; all-items-checked fails). The ghost check **immediately caught a real ghost**: `AGENTS.md` cited `task/errors.go` (fixed → `internal/task/errors.go`) |
| C04 release gate (D14–D18)     | CHANGELOG cut `[0.1.0] - 2026-09-06`; release checklist with owner decisions (no squash; v0.x = GitHub pre-release) in `docs/release/`; **annotated tag `v0.1.0` pushed; GitHub Release live (prerelease, latest)**                                                                                                                                          | all gates green in one run; CI green on the exact tagged commit (twice: master + tag ref); proxy `.info` returns v0.1.0 at `6a35ac7`; clean-room `go get …@v0.1.0` + `go mod verify` pass                    |

**Slice 2 — Pool Hardening (6 of 7 coarse tasks code-complete):**

| Task                          | What shipped                                                                                                                                                                                                                                                                      | Verify actually run                                                                                                                                                   |
| ----------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| C08 `--model` (D29–D31)       | `harvest.Config.Model` → `AgentPayload.Model`; `--model` on harvest + agent-pool                                                                                                                                                                                                  | new payload-parsing test; CLI help smoke                                                                                                                              |
| C09 preflight (D32–D34)       | `executor.PreflightError` + `Store.Requeue` (returns task to pending, **zero attempt burn**, delay-gated, `task.requeued` fact); dirty tree + missing autonomy now preflight instead of permanent; autonomy probe accepts user-global crush config (`~/.config/crush/crush.json`) | worker test: preflight → pending/0 attempts → completes after "fix" with same task; store Requeue test (lease guard, delay, fact); global-probe unit test             |
| C10 verify strategy (D35–D37) | `.tq-verify` file wins over payload over auto-detect; auto-detect adds Makefile→`make test`, flake.nix→`nix build && nix flake check`, Cargo→`cargo test --quiet`; harvest pins repo `.tq-verify` into payloads (`executor.ReadTQVerify`)                                         | precedence table test + end-to-end "file command gates the run" test; harvest payload test                                                                            |
| C11 pacing (D39–D41)          | `Config.RepoIntervals` (per-repo minimum gap); `Config.DLQBackoff` (poisoned repo: dead tasks, none completed, newest dead in window → skip with `poisoned:` reason)                                                                                                              | two new tests: poisoned pause + lift on completion; per-repo interval isolation                                                                                       |
| C12 daemon (D42–D43)          | `tq agent-pool --once` (one tick, drain, exit); systemd user unit `deploy/systemd/tq-agent-pool.service` (Restart=on-failure, 45min graceful stop)                                                                                                                                | `systemd-analyze verify` clean; `--once` live smoke: harvest→enqueue→agent→complete→**process exits 0 by itself**; budget-refusal run exits fast with the skip logged |
| C07 budgets (D25–D28)         | new `internal/budget`: `Guard{DailyCap, BudgetCmd}`; spend projected from `task.enqueued` facts since local midnight; budget command is final authority; agent-pool `--daily-budget` / `--budget-cmd` checked before every tick                                                   | 3 unit tests (facts counting, cap refusal, cmd authority); live smoke: second `--once` run refuses with `budget: skipping harvest tick`                               |

**Incidental:** `tq facts`/`tq tail` share a formatter incl. error class;
`boolInt` SQL helper; budget guard fail-open on journal errors (documented).

## b) PARTIALLY DONE

1. **C10 D38** — README snippet for `.tq-verify` not yet written (code + tests done).
2. **C12 D44** — daemon docs in README/AGENTS pending (unit + `--once` done).
3. **C07 D25** — budget design note was planned for ADR-0002; ADR-0002 not written yet.
4. **D18 pkg.go.dev** — proxy + `go get` verified; docs page still 404 (crawl lag; MIT license present). Left unchecked in the checklist on purpose.
5. **Slice-2 documentation debt** — FEATURES/TODO_LIST/CHANGELOG/README not yet synced with C07–C12 (CHANGELOG `[Unreleased]` is still "Nothing yet" while 6 features landed after the tag).

## c) NOT STARTED

- **C06** multi-repo live smoke (2 pools, ≥3 repos, no double-run per repo).
- **Slice 3:** C13 session-id result detail, C14 `tq top`, C15 docs-drift auditor, C16 E2E subprocess test, C17 property/fuzz, C18 chaos (SIGKILL), C19 Windows/i18n hygiene.
- **Slice 4:** C20 DOMAIN_LANGUAGE.md, C21 ADR-0002, C22 doc.go surfaces, C23 SECURITY.md, C24 PapDashboard E2E + fan-out design + SSE PoC, C25 CQA live verification (**blocked: needs a real instance + token**), C26 golangci-lint/dprint policy decision, C27 deferred-bundle seeds.
- Final full-repo docs sync + push of the session's work.

## d) TOTALLY FUCKED UP (and what it cost)

1. **The first `--once` "verification" was false.** I reported `EXIT=0` — but that was the exit code of `tail` in a pipeline; the actual process hung and `timeout` killed it. Root cause: `pool.Start` returns only when the caller's context is done, so `pool.Stop()` alone can never end `--once`. Caught only because I re-ran the smoke and watched a process refuse to die; fixed in `ea09aa2` (drain watcher now also cancels the context) and re-verified with real exit codes and durations. **Lesson: never assert an exit code through a pipeline.**
2. **Two smokes launched real crush agents** because I forgot `TQ_AGENT_BIN=stub`. One of them did real work in a scratch repo (marked the item `[x]`, created `.git`, committed ~35s run). Harmless here, but in a real repo that would be uncontrolled agent spend from a "smoke test".
3. **The ghost-check guard initially matched NOTHING** (the `write` tool escaped my backticks, so the grep regex was `\`` and matched zero lines) — and my first mutation test "passed" anyway because the mutation never reached the broken regex. A security-style guard that silently matches nothing gives maximal false confidence. Fixed and re-mutation-tested until the guard both caught planted ghosts and a real one.
4. **I pre-checked release-checklist boxes before verifying them** (docs-health's own cardinal rule), caught it in the same minute, and corrected to honest state before proceeding.
5. Smaller self-inflicted churn: ordering assumptions in three tests (queue order is priority/created_at/id, never FIFO), a test that executed the executor twice inside an error message, `claimOnce.TryClose()` (doesn't exist), a long-task test blocked past its own TaskTimeout, missing `time`/`json` imports, and `printf --` writing a 2-byte garbage fixture. Each cost a test cycle; none shipped.

## e) WHAT WE SHOULD IMPROVE

1. **Verify the verifier.** Every new guard/check should be mutation-tested twice: once to prove it fails on a real break, once to prove the test harness itself isn't a no-op (the escaped-regex trap).
2. **A tiny smoke harness** (`scripts/smoke/`): stub agent always, real exit codes, duration assertions, fixtures rebuilt heredoc-first. Today's smokes were ad-hoc shell with three traps.
3. **Update TODO_LIST/FEATURES/CHANGELOG as work lands**, not batched at the end — the docs drift this repo keeps catching in CI applies to me too.
4. **`--once` + budget interplay is now good but under-documented**; the systemd unit and cron examples should show `--once --daily-budget` together.
5. **Daemon-authored commits** interleaved with mine made `git log` noisy during the release (and once swept the `result` symlink into git). Keep gitignoring artifacts and prefer committing slice-boundaries myself immediately after gates.
6. **Property tests for claim ordering** — I hit the "not FIFO" assumption three times; one property test would kill this class for good (planned as C17).

## f) NEXT 50 (ordered by impact; IDs from the round-2 plan where they exist)

**Finish slice 2 (immediate):**

1. C06/D22–D24 — multi-repo live smoke: 3 scratch repos, 2 `--project-exclusive` pools, stub agents; assert no double-run per repo + captured log.
2. D38 — README `.tq-verify` snippet (verify strategy table).
3. D44 — daemon docs: README + AGENTS (`--once`, systemd unit, cron example).
4. D25 — budget design note → goes into ADR-0002 when written (next slice).
5. Docs sync pass 1: TODO_LIST checkboxes for D01–D44, FEATURES rows (error classes, exclusivity, preflight requeue, `.tq-verify`, budgets, `--once`), CHANGELOG `[Unreleased]`.

**Slice 3 (proof + observability):**
6. D45–D47 / C13 — crush session id into result detail, rendered by `tq show`.
7. D48–D50 / C14 — `tq top` live per-project view over facts.
8. D51–D52 / C15 — docs-drift auditor: done-in-code but unchecked TODO items → enqueue catch-up.
9. D53–D55 / C16 — E2E subprocess test: build tq, spawn agent-pool with `$TQ_AGENT_BIN` stub, full loop in-test; wire into CI (no API cost).
10. D56–D58 / C17 — property test (dedup keys stable/collision-free) + fuzz `ParseRepo` (CRLF/BOM/nesting), fix findings, document parser contract.
11. D59–D60 / C18 — chaos: SIGKILL pool mid-run → lease reclaim observable in facts; no-double-complete invariant test.
12. D61–D62 / C19 — Windows path audit in harvest (GOOS=windows test build) + i18n hash vectors.
13. Re-verify D18 — pkg.go.dev indexed; tick the checklist box.
14. Push this session's commits (currently only local? — verify `git status` vs origin before continuing).

**Slice 4 (docs, bridges, seeds):**
15. D63–D65 / C20 — `docs/DOMAIN_LANGUAGE.md` (≥15 terms: task, fact, claim, lease, harvest, dedup key, preflight, poisoned repo, budget guard…), bounded contexts, links from AGENTS/README.
16. D66–D67 / C21 — ADR-0002: agent-pool architecture, autonomy/trust model, drain semantics, **budget design** (D25), exclusivity tradeoffs; cross-check vs code.
17. D68–D70 / C22 — `doc.go` for queue/worker/executor/harvest/journal/budget + godoc examples; `go doc` clean pass (also unblocks pkg.go.dev quality).
18. D71–D72 / C23 — SECURITY.md (autonomy grants, blast radius of `bash`, budget ceilings) + pool-start warning when a repo grants unsandboxed bash.
19. D78 / C26 — golangci-lint: decide gate-or-drop; if gate: config with errcheck exclusions for idiomatic `defer Close` and make CI honest.
20. D79 / C26 — dprint in devShell + one formatted pass over docs.
21. D76–D77 / C25 — CQA live verification (BLOCKED on owner: instance URL + owner id + token); upgrade FEATURES row after.
22. D73 / C24 — PapDashboard E2E script (docker pap + tq worker `--alert-url`), run once green.
23. D74 / C24 — decision→question fan-out design note (docs/planning).
24. D75 / C24 — SSE fan-out PoC (`tq tail -f` → HTTP stream, `curl` shows live facts).
25. D82 — internal→public decision note (which packages, when, what breaks).
26. D80 — Postgres spike: schema + `FOR UPDATE SKIP LOCKED` claim sketch (docs/planning).
27. D81 — HTTP API thin-wrapper PoC over Store (curl enqueue works).
28. D83 — cron recurring tasks PoC (dedup-keyed re-enqueue).
29. D84 — structured result payload `{files_changed, commit_sha}`: schema + writer + reader.
30. D85 — output sidecar: full stdout to file, path in result detail.
31. D86 — Prometheus metrics endpoint over facts.
32. D87 — `tq harvest --json` + `--repo-subset` glob.
33. D88 — `tq dlq --rescue-all --older-than`.
34. D89 — guard: refuse `--projects-dir /` and `$HOME` with a clear error.
35. D90 — per-repo-size timeout defaults.
36. D91 — crush rate-limit + version detection at pool start.
37. D92 — PR-mode PoC (branch + `gh pr create` in a scratch repo).
38. D93 — worktree isolation PoC (agent touches worktree only).
39. D94 — session chains via `AgentPayload.Session`.
40. D95 — web UI spike over facts projection.
41. D96 — DB rotation/backup guidance doc.
42. D97 — cross-repo DAG templates in harvest.
43. D98 — ai-task-prioritizer hook writing `priority`.
44. D99 — smart retry: error-class → policy mapping (build on C01 classes).
45. D100 — consumer-group pool spike (fencing tokens) design note.

**Close-out:**
46. C04 follow-up — decide and cut **v0.2.0** (slices 2–4 are additive but the store interface grew `FailPermanent`/`Requeue`).
47. Annotate the three open owner questions still in ROADMAP (cancelled-dedup semantics, pool-manages-this-repo, cost ceiling default) with the session's new context.
48. Update the round-2 plan status header: PLANNED → EXECUTING (slice 1 ✅, slice 2 ✅-ish, …).
49. Consider `--all-systems` nix flake check on CI (currently warns aarch64/darwin are unchecked).
50. Retire/rotate the `/tmp/tq-*` smoke fixtures into `scripts/smoke/` fixtures (they live in /tmp today).

## g) QUESTIONS ONLY YOU CAN ANSWER

1. **pkg.go.dev (D18 tail):** docs are still 404 ~45 min after the tag. Do you want to trigger indexing from the pkg.go.dev web UI with your browser session, or shall I keep polling and only tick the box when it resolves itself?
2. **Versioning:** v0.1.0 is out; slices 2–4 (error classes refinement, exclusivity, budgets, `--once`, preflight) are already on master but NOT in the release. Cut **v0.2.0** at the next slice boundary, or hold until slice 4 (docs/ADR) is also done?
3. **CQA live verification (C25/D76) is blocked without credentials:** can you give me a real Code-Quality-Agent base URL + owner ID + token I may hit from a test, or should the bridge stay httptest-verified and PLANNED?

---

_Report generated per the status-report skill; Markdown override consistent with the 16:19 report._
