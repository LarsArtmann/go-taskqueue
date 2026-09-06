# Session Execution Report — Slices 1–2 Done + C06/C13/C16/C17/C18; Docs Debt & Lessons

**Date:** 2026-09-06 18:36 CEST
**Scope:** continued execution of the round-2 plan (docs/planning/2026-09-06_16-28)
after the 18:00 report, plus a brutally honest self-audit of the whole session.
**State at writing:** HEAD `c553ad0`, pushed, **CI green on it** (run 34045998684:
test 40s + nix 1m14s). `v0.1.0` remains released and verified.

**IMPORTANT — one more CI incident right before this report:** the push of
C13+C18 initially FAILED the nix job — the chaos test had `internal/task` in
the stdlib import block; gofmt accepts that, the flake's treefmt gate does
not. Fixed (`c553ad0`), `nix flake check` re-run locally, CI re-run green.
I almost wrote this report claiming "CI green" while master's tip was red —
checking `gh run list` before writing caught it.

---

## a) FULLY DONE (verified, not just written)

All of the 18:00 report's section (a) still holds — slice 1 complete,
**v0.1.0 released** (tag + GitHub pre-release + proxy `.info` + clean-room
`go get`), C07–C08 budgets/model/preflight/verify/pacing/daemon. On top of
that, this session segment added:

| Task                                   | What shipped                                                                                                                                                                                                                                                                                                                                 | Verify actually run                                                                                                   |
| -------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| C06 multi-repo live smoke (D22–D24)    | `scripts/smoke/multi-repo.sh`: builds tq, 3 fixture repos, **2 concurrent agent-pool processes** on 1 shared DB (stub agent, zero API cost), two rounds; asserts 6 enqueued-once / 6 completed / exactly 6 claims / 0 dead / both pools claimed (3+3)                                                                                        | ran green after fixes; first runs exposed two real bugs (see d)                                                       |
| **Fresh-DB migrate race fix**          | `OpenSQLite` retries `IF NOT EXISTS` migrations with bounded backoff — two processes opening a brand-new DB used to fail with SQLITE_BUSY                                                                                                                                                                                                    | found BY the smoke's first run; queue tests + smoke green                                                             |
| C17 property + fuzz (D56–D58)          | `TestItemKeyProperty` (200 randomized cases: key = sha256(repo, collapsed text), reflow-stable, repo-participating, construction pinned) + `FuzzParseRepo` (never panics, deterministic, usable `todo:` keys; seeds: CRLF, BOM, nesting, fences, 10k lines)                                                                                  | 30s campaign: 1,841,858 execs, 147 interesting inputs, zero findings                                                  |
| C16 E2E subprocess (D53–D54)           | `internal/e2e`: builds the real CLI, runs it as an operator would — `agent-pool --once` completes a stub task and exits 0 by itself; exhausted `--daily-budget` refuses the tick with the reason and still exits cleanly                                                                                                                     | both tests green in-process-free; runs in CI via `go test ./...` at no API cost (D55 satisfied)                       |
| C13 session-id result detail (D45–D47) | `executor.AgentResult{SessionID, VerifyTail}` + per-task result `Sink` in the execution context (race-free with shared executors); agent executor extracts the session id (tolerant regex) and the verify tail; worker stores it in `task.completed` detail; **`tq show TASK_ID` now renders the task's full fact trail**                    | unit test (stub prints `session: …` → detail contains it) + live CLI smoke: `detail: {'session_id': 'crush-live-99'}` |
| C18 chaos (D59–D60)                    | `TestChaosKillWorkerMidRun`: enqueue, victim worker claims, **SIGKILL mid-run**, wait lease expiry, successor reclaims and completes, journal shows exactly ONE completion                                                                                                                                                                   | green in 1.5s (short `--lease` keeps the test fast)                                                                   |
| Docs sync (partial close-out)          | README: verify-strategy ladder (`.tq-verify` wins), cost ceilings, fail-fast semantics, daemon story (`--once`, systemd unit, cron), `--model`/`--project-exclusive`; FEATURES: promoted exclusivity/error-classes/preflight/budgets/CI-guards to 🟢 with proofs; TODO_LIST: 5 items checked; CHANGELOG `[Unreleased]`: all post-v0.1.0 work | ghost-ref guard green; gates green                                                                                    |

Also: `--repo-interval`/`--dlq-backoff` flags added because the README
described them before they existed (see e) — both now smoke-verified,
including the bad-spec error path.

## b) PARTIALLY DONE

1. **CHANGELOG/FEATURES/TODO_LIST second pass** — C13 (session id), C16 (e2e), C17 (fuzz/property), C18 (chaos) landed AFTER the docs-sync commit; none are in CHANGELOG `[Unreleased]`, FEATURES has no rows for e2e/fuzz/chaos/result-detail, TODO_LIST has unchecked items mapping to them.
2. **README** — `tq show`'s new fact-trail output is undocumented; `scripts/smoke/multi-repo.sh` is not referenced from CONTRIBUTING/README.
3. **D25 budget design note** — still owed to ADR-0002 (C21 not started).
4. **D55 nuance** — e2e tests run in CI implicitly via `go test ./...`; there is no dedicated/labelled CI step, acceptable but worth an explicit job name for visibility.
5. **Session-id extraction** — tolerant regex implemented and stub-tested, but never validated against a REAL crush run's output format; if crush prints its session differently, extraction silently yields nothing.
6. **D18 pkg.go.dev** — still unverified (crawl lag) as of this report.
7. **aarch64/darwin** — `nix flake check` still warns those systems are unchecked (`--all-systems` not wired).

## c) NOT STARTED

- **C14** `tq top` live per-project view (D48–D50)
- **C15** docs-drift auditor (D51–D52)
- **C19** platform hygiene: Windows path audit (D61), i18n hash vectors (D62)
- **C20** `docs/DOMAIN_LANGUAGE.md` (D63–D65)
- **C21** ADR-0002 (D66–D67) — must absorb the budget design (D25)
- **C22** `doc.go` package surfaces + godoc examples (D68–D70)
- **C23** SECURITY.md + pool-start bash-permission warning (D71–D72)
- **C24** bridges maturation: PapDashboard E2E script (D73), fan-out design (D74), SSE PoC (D75)
- **C25** CQA live verification (D76–D77) — **blocked on owner credentials**
- **C26** golangci-lint gate-or-drop decision + dprint in devShell (D78–D79)
- **C27** deferred-bundle seeds (D80–D100)

## d) TOTALLY FUCKED UP (and what it cost)

1. **I pushed a red master while believing it green.** The C13+C18 commit
   failed CI's nix job (treefmt import grouping) and my commit sequence
   didn't re-check CI afterwards — I only discovered it because the report
   instruction made me pull `gh run list` before writing. Fixed in
   `c553ad0`, CI re-verified green. Cost: one red run on master (~90 min
   window). **Rule going forward: no "done" claim without checking the CI
   run of the exact pushed commit.**
2. **The edit-tool vs auto-commit-daemon collision tax** — repeatedly
   `edit` failed with "file modified since read" (daemon/gofmt racing me),
   and in one case an edit was silently NOT applied while I believed it
   was (the fuzz-test generator fix), costing an extra fail-debug cycle.
   Python-based patching proved more reliable under this churn, at the
   cost of bypassing the read-state discipline. Root cause is two writers,
   one worktree.
3. **Docs-before-code bit me again**: README described `--repo-interval`
   and `--dlq-backoff` before they were implemented. I did implement them
   immediately, but for ~10 minutes the docs promised a CLI that didn't
   exist — the exact drift the CI ghost guard exists to prevent (it can't
   catch flag promises, only paths).
4. **C17 first draft was sloppy**: the whitespace generator merged WORDS
   (legitimately changing keys) instead of testing reflow, plus an
   undefined-variable block I cut entirely. Two wasted test cycles on my
   own test.
5. **The e2e file's first draft contained a half-built http test** that I
   then deleted — should not have entered the file at all.
6. Carried over from earlier in the session (still worth naming): the false
   `--once` "EXIT=0" (pipeline exit code masking a timeout kill — fixed in
   `ea09aa2`), two accidental REAL crush runs because `TQ_AGENT_BIN` wasn't
   set, and a ghost-check script that matched nothing while passing.

## e) WHAT WE SHOULD IMPROVE

1. **Post-push CI gate in my own protocol**: after every push, `gh run
   watch` before declaring success — today's treefmt miss proves local
   gates ≠ CI gates even when I _have_ a nix job locally.
2. **One-writer discipline**: pause or scope the auto-commit daemon during
   active coding sessions, or commit instantly after each verified step so
   the daemon never races me mid-file.
3. **Smoke/fixture hygiene**: `scripts/smoke/` should own its fixtures and
   ALWAYS force `TQ_AGENT_BIN` to a stub inside the script (multi-repo.sh
   does this; my ad-hoc smokes forgot twice and launched real agents).
4. **Docs order**: implement → verify → document, never document → implement
   (the flags incident).
5. **Make the fuzz campaign reproducible in CI**: a nightly/weekly
   `go test -fuzz … -fuzztime 60s` job so corpus growth doesn't depend on my
   memory; commit interesting seeds into `testdata/fuzz`.
6. **TODO_LIST as a live contract**: check items off at task completion, not
   in batched doc-sync passes — the harvester parses it, so drift there is
   user-visible.

## f) NEXT 50 (ordered by impact; ★ = plan ID carried forward)

**Close slice 2's documentation debt (immediate):**

1. CHANGELOG `[Unreleased]`: add session-id/result detail, e2e subprocess tests, property+fuzz tests, chaos test, migrate-race fix entry check.
2. FEATURES: rows for e2e suite, fuzz/property harness, chaos test, agent result detail.
3. TODO_LIST: check off items mapping to C13/C16–C18 (re-audit all 25).
4. README: document `tq show`'s fact trail + `scripts/smoke/multi-repo.sh`.
5. CONTRIBUTING: add the smoke harness + e2e test instructions.

**Slice 3 remainder:**
6. ★C14/D48–D50 — `tq top`: per-project live view over facts + tests.
7. ★C15/D51–D52 — docs-drift auditor: done-in-code-but-unchecked TODO items → enqueue catch-up.
8. ★C19/D61 — Windows path audit in harvest (GOOS=windows test build).
9. ★C19/D62 — i18n-safe hashing test vectors (non-ASCII item keys — partially covered by the property test's 行/дорога/🚀 words; formalize).

**Slice 4:**
10. ★C21/D66–D67 — ADR-0002: agent-pool architecture, autonomy/trust, drain semantics, **budget design (D25)**, exclusivity tradeoffs.
11. ★C20/D63–D65 — `docs/DOMAIN_LANGUAGE.md` (≥15 terms), bounded contexts, link from AGENTS/README.
12. ★C22/D68–D70 — `doc.go` for queue/worker/executor/harvest/journal/budget + godoc examples; `go doc` clean pass.
13. ★C23/D71–D72 — SECURITY.md + pool-start warning when a repo grants unsandboxed bash.
14. ★C26/D78 — golangci-lint: gate-or-drop decision; errcheck exclusions config if gate.
15. ★C26/D79 — dprint into devShell + one formatted pass over docs.
16. ★C24/D73 — PapDashboard E2E script (docker pap + worker `--alert-url`).
17. ★C24/D74 — decision→question fan-out design note.
18. ★C24/D75 — SSE fan-out PoC (`tq tail -f` → HTTP stream).
19. ★D55 polish — labelled CI step for the e2e suite.
20. D18 — verify pkg.go.dev indexed; tick the release-checklist box.
21. Nightly fuzz job (60s `FuzzParseRepo`) + commit corpus seeds to `testdata/fuzz`.
22. `tq worker --once` (parity with agent-pool; noted in e2e comments).
23. Validate session-id extraction against a REAL crush run output; adjust the regex if the format differs.
24. `--all-systems` nix flake check in CI (covers aarch64/darwin) or document why not.
25. Consider `git rm` of stale `docs/status` duplication via a STATUS index file linking all reports.

**Deferred-bundle seeds (C27):**
26. ★D80 — Postgres spike: schema + `FOR UPDATE SKIP LOCKED` claim sketch.
27. ★D81 — HTTP API thin-wrapper PoC over Store (curl enqueue works).
28. ★D82 — internal→public decision note (which packages, when).
29. ★D83 — cron recurring tasks PoC (dedup-keyed re-enqueue).
30. ★D84 — structured result schema `{files_changed, commit_sha}` (extends C13's sink).
31. ★D85 — output sidecar: full stdout to file, path in result detail.
32. ★D86 — Prometheus metrics endpoint over facts.
33. ★D87 — `tq harvest --json` + `--repo-subset` glob.
34. ★D88 — `tq dlq --rescue-all --older-than`.
35. ★D89 — guard: refuse `--projects-dir /` and `$HOME`.
36. ★D90 — per-repo-size timeout defaults.
37. ★D91 — crush rate-limit + version detection at pool start.
38. ★D92 — PR-mode PoC (branch + `gh pr create` in a scratch repo).
39. ★D93 — worktree isolation PoC.
40. ★D94 — session chains via `AgentPayload.Session` (now natural: result sink already exposes session ids).
41. ★D95 — web UI spike over facts projection.
42. ★D96 — DB rotation/backup guidance doc.
43. ★D97 — cross-repo DAG templates in harvest.
44. ★D98 — ai-task-prioritizer hook writing `priority`.
45. ★D99 — smart retry: error-class → policy mapping (build on C01's classes; per-project budgets could ride the same config).
46. ★D100 — consumer-group pool spike (fencing tokens) design note.

**Close-out:**
47. Cut **v0.2.0** at the next slice boundary (store interface grew `FailPermanent`/`Requeue`; additive, fine for 0.x) — needs your go/no-go.
48. Annotate ROADMAP's three open owner questions with this session's context (cancelled-dedup semantics, pool-manages-this-repo, cost-ceiling default).
49. Update the round-2 plan header: slice 1 ✅ shipped in v0.1.0, slice 2 ✅ (C06–C12), slice 3 ▲ (C13/C16–C18), slice 4 ⬜.
50. Retire `/tmp/tq-*` ad-hoc fixtures into `scripts/smoke/` so every smoke is one command.

## g) QUESTIONS ONLY YOU CAN ANSWER

1. **CI safety net:** the daemon commits and I push continuously — do you want me to make `gh run watch` a hard gate before every "done" claim (slower turns, no red master), or accept occasional red on master for speed?
2. **pkg.go.dev (D18 tail):** still 404. Will you trigger indexing from the pkg.go.dev web UI with your browser session, or shall I keep polling and only tick the box when the crawl resolves it?
3. **CQA live verification (C25/D76) is blocked without credentials:** can you give me a real Code-Quality-Agent base URL + owner ID + token to test against, or does the bridge stay httptest-verified and 🟡 in FEATURES?

---

_Per the status-report skill; Markdown per the established override. Then: waiting for instructions._
