# Status Report: Hardcore Prompt Review — All Agent Contracts Upgraded

**Date:** 2026-09-07 21:44 CEST
**Scope:** THIS SESSION ONLY (per instruction: no unrelated research). Session = "Review ALL prompts like HARDCORE, learn from /home/lars/projects/SKILLS/**".
**Verification status at write time:** `go vet` clean · `gofmt` clean · `go test ./... -race` green (all 12 packages) · working tree clean.

## Brutal Self-Review (the questions asked first)

- **What did I forget?** (1) The full pre-push gate `./scripts/ci-local.sh` — I ran vet + gofmt + race tests, but not the CI replicant (advisory lint run, webui smoke, doc-reference check). (2) A dprint format check on my CHANGELOG edit (living docs are dprint-formatted). (3) The live dogfood pool implication: in-flight agents run the OLD prompt until new enqueues happen; I changed contracts without checking how the pool daemon picks up the new template, and did not verify a real agent actually emits `TQ_RESULT:` yet. (4) `docs/DOMAIN_LANGUAGE.md` was not re-read before writing prompt prose (I reused existing wording, so risk is low).
- **What could I have done better?** (1) Pin the `— BLOCKED:` convention through the REAL `blockedReason` parser, like I did for `TQ_RESULT` — my guardrail test only asserts the substring exists. (2) Add a one-line `TQ_RESULT` mention to AGENTS.md "Payload contracts worth memorizing" while I was in there. (3) Add a cheap test that rendered payloads contain no stray `{{PLACEHOLDER}}`. (4) Caught my own compile error (`got.Template` on a `task.New`) via diagnostics before ever running tests — better to have designed the test against the real signature first.

---

## a) FULLY DONE

| # | Work                                                                                                                                                                                                                                    | Evidence                                                                                                                         |
| - | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| 1 | Learned prompt-craft rules from the SKILLS repo (`how-to-write-skills.md`: output-format examples, name-the-#1-failure-mode, explain-the-why, verify-against-source, restraint)                                                         | Read in full (567 lines) this session                                                                                            |
| 2 | Found ALL prompts in the repo (grep sweep): exactly 3 exist — `DefaultPromptTemplate` (harvest work item), `DefaultCatchupPrompt` (drift catch-up), cqa fix prompt                                                                      | `rg "[Pp]rompt"` sweep; tests pin all 3                                                                                          |
| 3 | All 3 prompts now teach the `TQ_RESULT:` self-report line (single-line JSON example matching `resultLineRe`) — closes documented debt items 44 (2026-09-06 report) and the queue-side half of CHANGELOG "structured result self-report" | Commit `a8ba7c3`; `TestAgentPromptsTeachParsableResult` parses each taught example with the REAL `executor.ExtractResultPayload` |
| 4 | Self-modifying-autonomy guardrail in all 3 prompts: forbid editing `.crushrc`/`crush.json`/`.tq-verify`, with the why — closes accepted-risk item 34 (2026-09-07 round4 report) and dogfood plan §7 mitigation                          | Commit `a8ba7c3`; `TestAgentPromptsGuardrails`                                                                                   |
| 5 | cqa prompt anti-scanner-gaming rule ("never weaken tests, never blanket-suppress") + explicit commit permission + "smallest correct change" (dogfood plan contract wording) in all prompts                                              | Commit `a8ba7c3`; `TestFixTaskPromptContract`                                                                                    |
| 6 | Contract-pinning regression tests: 84 lines in `harvest_test.go`, 58 in `cqa_test.go` — prompt drift now fails tests instead of silently breaking `tq show` results                                                                     | Commit `0e8ecfb`; PASS under `-race`                                                                                             |
| 7 | CHANGELOG `[Unreleased]` Changed entry documenting the prompt upgrade                                                                                                                                                                   | Commit `0e8ecfb`                                                                                                                 |
| 8 | Bonus: rewrote the two over-length (132/138 char) cqa prompt lines — both lll findings in that file gone                                                                                                                                | `awk length>120` clean on all touched files                                                                                      |

## b) PARTIALLY DONE

| # | Work                           | Works now                           | Missing                                                                                                                  | Effort            |
| - | ------------------------------ | ----------------------------------- | ------------------------------------------------------------------------------------------------------------------------ | ----------------- |
| 1 | Verification depth             | vet + gofmt + full race suite green | `./scripts/ci-local.sh` (the actual pre-push gate: advisory lint, webui smoke, doc-reference check) not run this session | S                 |
| 2 | Real-world TQ_RESULT adoption  | Parser + prompts + tests aligned    | Zero observed agent runs with the NEW prompt — emission rate unproven; need next pool run + `tq show` spot-check         | M (waits on pool) |
| 3 | BLOCKED-convention pinning     | Substring asserted in prompt        | `blockedReason` parser not fed the prompt's marker by a test (asymmetry vs. the TQ_RESULT pin)                           | S                 |
| 4 | Docs truth-sync for debt items | Fixed in code + CHANGELOG           | Old status reports (items 34/44) not annotated as resolved (docs-health ANNOTATE not run)                                | S                 |

## c) NOT STARTED

| # | Work                                                                                                                                                     | Why                                                            | Still wanted?                             |
| - | -------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------- | ----------------------------------------- |
| 1 | AGENTS.md payload-contract line documenting the `TQ_RESULT` emission convention                                                                          | Chose minimal docs churn this session                          | Yes — it is the agent-facing contract doc |
| 2 | Placeholder-hygiene test (no stray `{{...}}` in rendered payloads)                                                                                       | Nice-to-have; not noticed broken                               | Yes, cheap                                |
| 3 | `{{REPO}}` placeholder cleanup (documented but unused in `DefaultPromptTemplate`)                                                                        | Cosmetic; left per don't-churn rule                            | Low                                       |
| 4 | Prompt version marker (e.g. `contract v2` line) for journal forensics                                                                                    | Idea born this session; not designed                           | Roadmap                                   |
| 5 | Advisory lint debt in touched functions (gocognit `runRepo` 38>30, cyclop `Collect` 16>12, exhaustive switch, err113) — untouched per no-mass-fix policy | Policy: fix in passing only when next touching those functions | Yes, opportunistically                    |

## d) TOTALLY FUCKED UP

Nothing in the repo is broken by this session — build, vet, gofmt, and the full race suite are green, and the contract tests pin the new behavior. The honest Fucked-Up list is process-level:

1. **Contract changed under a LIVE pool without operational follow-through.** Severity: medium (no data loss; dedup keys and verify gates unaffected; worst case = old-prompt agents keep omitting `TQ_RESULT` until re-enqueue). Root cause: I treated prompts as code, not as a rolling deployment. Mitigation: none applied yet — pool restart/next-enqueue check is section (f) item 2 and question 2 below.
2. **Trusted stale LSP diagnostics twice before independently verifying.** The gopls layer kept reporting lll on `cqa.go:201/202` after my fix (they were the OLD lines) and still reports false-positive webui noise; I verified via `awk` + build per the "independently verify tool output" rule — but only after the first confusing read. Severity: low (caught), but it is the exact failure class AGENTS.md warns about.
3. **Almost shipped a non-compiling test** (`got.Template` on a `task.New` — `renderTask` returns `task.New` directly). Caught by diagnostics pre-test-run, fixed in one edit. Severity: none at commit time; process note only.

## e) WHAT WE SHOULD IMPROVE

1. **Prompts are deployment artifacts, not constants.** Changing one under a running pool needs a "reload/observe" step, like a config rollout. Suggested fix: after any prompt change, (a) restart pool, (b) verify one fresh agent run emits `TQ_RESULT` (scripted grep over `tq show --json`).
2. **Pin conventions through their real parsers.** The `TQ_RESULT` test (taught example → real parser) is the strongest pattern this session produced; apply it to the `— BLOCKED:` marker → `blockedReason`, and to placeholder rendering.
3. **Contract tests live next to a "conventions ledger".** AGENTS.md's payload-contract section should name `TQ_RESULT` so docs, prompts, and tests cite one convention. (Docs drift is how item 44 aged for a day.)
4. **Old-vs-new prompt eval.** The SKILLS repo's "old-skill vs new-skill" eval pattern applies: run N scratch-repo tasks with old vs. new prompt, measure `TQ_RESULT` emission rate — turns "reads better" into "measured better".
5. **Answer-first tool discipline held up:** awk/build over LSP truth; keep doing that (it prevented two wrong turns this session).

## f) 50 THINGS WE SHOULD GET DONE NEXT

> Brainstorm per your instruction (50), not a commitment list. Sorted roughly by impact. This section is the primary input for `docs-health` HARVEST.

| #  | Task                                                                                                                                         | Impact | Effort | Category      |
| -- | -------------------------------------------------------------------------------------------------------------------------------------------- | ------ | ------ | ------------- |
| 1  | Run `./scripts/ci-local.sh` (full pre-push gate) before next push                                                                            | High   | S      | Quality       |
| 2  | Restart/verify the dogfood agent-pool so new enqueues use the upgraded prompt                                                                | High   | S      | Ops           |
| 3  | After next pool run, `tq show` a completed task and confirm `files_changed`/`commit_sha` are populated                                       | High   | S      | Verification  |
| 4  | Add `TQ_RESULT` convention line to AGENTS.md payload-contracts section                                                                       | High   | S      | Documentation |
| 5  | Extend contract test: feed prompt's `— BLOCKED:` marker through real `blockedReason`                                                         | High   | S      | Quality       |
| 6  | Add test: rendered agent payload contains no stray `{{PLACEHOLDER}}` for all 3 prompts                                                       | Medium | S      | Quality       |
| 7  | Annotate old status reports (items 34, 44) as resolved — docs-health ANNOTATE                                                                | Medium | S      | Documentation |
| 8  | HARVEST this report's section (f) into TODO_LIST.md / ROADMAP.md                                                                             | High   | S      | Documentation |
| 9  | Measure old-vs-new prompt `TQ_RESULT` emission rate on scratch repos (eval harness)                                                          | High   | M      | Quality       |
| 10 | Script: grep `tq show --json` for result-detail presence rate (adoption metric)                                                              | Medium | S      | Feature       |
| 11 | Verify `tq harvest` CLI actually surfaces `Config.PromptTemplate`/`Model` (I saw Config fields, did not verify flag wiring)                  | Medium | S      | Verification  |
| 12 | Verify worker captures the `TQ_RESULT` line when it appears mid-stream vs. final output (where `ExtractResultPayload` is invoked on stdout)  | Medium | S      | Verification  |
| 13 | Confirm `MaxAttempts` "0 = default" default value matches its doc comment (noticed, unverified)                                              | Low    | S      | Verification  |
| 14 | Remove or use the dead `{{REPO}}` placeholder in `DefaultPromptTemplate`                                                                     | Low    | S      | Cleanup       |
| 15 | Add prompt version marker (e.g. `contract: 2026-09-07`) for journal forensics                                                                | Medium | S      | Feature       |
| 16 | Reduce `runRepo` cognitive complexity 38>30 (when next touched, per policy)                                                                  | Low    | M      | Quality       |
| 17 | Reduce cqa `Collect` cyclomatic complexity 16>12 (when next touched)                                                                         | Low    | M      | Quality       |
| 18 | Add missing switch cases/default in harvest status switch (exhaustive linter)                                                                | Low    | S      | Quality       |
| 19 | Replace dynamic `errors.New` with sentinel errors in harvest (err113)                                                                        | Low    | S      | Quality       |
| 20 | Fix wrapcheck/varnamelen findings in functions when next touched (policy)                                                                    | Low    | S      | Quality       |
| 21 | `docs/DOMAIN_LANGUAGE.md`: add "self-report" / `TQ_RESULT` term                                                                              | Low    | S      | Documentation |
| 22 | Check `SECURITY.md` claim accuracy now that prompts teach `TQ_RESULT` (line is stored verbatim in journal — already covered, verify wording) | Low    | S      | Documentation |
| 23 | Consider `internal/agentprompt` consolidation IF a 4th prompt appears (YAGNI until then)                                                     | Low    | M      | Roadmap       |
| 24 | `docs/AGENT_CONTRACTS.md`: human-readable page rendering all active prompts                                                                  | Low    | S      | Documentation |
| 25 | Extend cqa prompt test to pin the rendered issue-line format (existing test only counts issues)                                              | Low    | S      | Quality       |
| 26 | Document resultLineRe's single-line limitation in `result.go` (multiline JSON silently unparseable)                                          | Low    | S      | Documentation |
| 27 | ROADMAP: prompt A/B versioning infrastructure (pick template per cohort)                                                                     | Low    | L      | Roadmap       |
| 28 | Consider `tq prompts` subcommand printing active contracts for humans                                                                        | Low    | S      | Roadmap       |
| 29 | Verify papdashboard bridge FEATURES.md claims still true after this session (untouched; cheap re-check)                                      | Low    | S      | Verification  |
| 30 | vendorHash fakeHash dance IF go.mod changes later (gate exists; reminder)                                                                    | Medium | S      | Ops           |
| 31 | Add cqa fix-task pacing (RepoIntervals-style) if scan floods ever observed                                                                   | Low    | M      | Roadmap       |
| 32 | Live end-to-end cqa bridge test against a real CQA API (blocked: server availability — question 1)                                           | Medium | M      | Verification  |
| 33 | Post-run dogfood review: check `tq dlq` for any agent that interpreted new rules oddly                                                       | Medium | S      | Ops           |
| 34 | Spot-check that `.crushrc`/`.tq-verify` mtimes are unchanged after next pool run (guardrail worked in practice)                              | Medium | S      | Verification  |
| 35 | Add smoke script asserting `go vet` + gofmt stay clean on prompt files (trivial, CI has it — skip unless drift)                              | Low    | S      | Quality       |
| 36 | Sweep for other docs-only conventions agents never learn (prompts are the only channel pool agents read)                                     | Medium | M      | Quality       |
| 37 | Evaluate teaching the verify-command contract (`​.tq-verify` semantics) in prompts so agents pre-verify with the SAME command                 | Medium | S      | Feature       |
| 38 | Consider emitting `TQ_RESULT` guidance for `sh` executor too (JSON self-report for shell tasks)                                              | Low    | M      | Roadmap       |
| 39 | Table-driven golden substrings: move all prompt assertions into one want/have table per prompt                                               | Low    | S      | Cleanup       |
| 40 | Run dprint on CHANGELOG.md and any docs touched this session                                                                                 | Low    | S      | Cleanup       |
| 41 | Confirm gopls stale lll diagnostics clear after daemon commits (LSP restart if not)                                                          | Low    | S      | Cleanup       |
| 42 | Add `tq facts` example to docs showing a task whose completion fact carries result detail                                                    | Low    | S      | Documentation |
| 43 | Review whether catch-up tasks should skip the commit-permission line (they only edit TODO_LIST.md — wording is fine, but confirm intent)     | Low    | S      | Verification  |
| 44 | Cross-repo: backport the "taught-example → real parser" test pattern to PapDashboard worker prompts if any exist                             | Low    | M      | Quality       |
| 45 | Check `git log` attribution/daemon commits are not drowning meaningful history (daemon wrote 4 of last 6 commits)                            | Medium | S      | Process       |
| 46 | Consider labeling daemon auto-commits with a session hint (heuristic message is opaque in `git log`)                                         | Low    | S      | Process       |
| 47 | ROADMAP: journal-searchable prompt template hash per task (which contract produced this run)                                                 | Low    | M      | Roadmap       |
| 48 | Add `examples/` snippet showing an AgentPayload with a hand-written prompt (onboarding)                                                      | Low    | S      | Documentation |
| 49 | Re-read `docs/DOMAIN_LANGUAGE.md` glossary before next prompt-wording change (skipped this session)                                          | Low    | S      | Process       |
| 50 | Celebrate + next hardcore audit target: worker heartbeat/drain prompt-adjacent behavior (untouched this session)                             | Low    | M      | Quality       |

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **Is a CQA API server available anywhere (URL/token) for a live end-to-end bridge test, or is the cqa bridge PoC-only for now?** I can test against httptest stubs only; whether a real deployment exists decides if item 32 is a Verification task or a Roadmap wish.
2. **Is the dogfood agent-pool still running right now, and do you want it restarted so new enqueues pick up the upgraded prompt?** I cannot see the daemon's process state from this session, and restarting a live pool is an operator call (in-flight agents must finish).
3. **Commit policy:** the auto-commit daemon landed both of this session's commits (`a8ba7c3`, `0e8ecfb`) — do you want me to commit future work (and status reports like this one) myself with authored messages, or always leave commits to the daemon/you?

---

**Handoff:** Section (f) is the primary input for `docs-health` HARVEST — items 4, 5, 6, 7, 8, 37 are TODO_LIST-grade; the rest route to ROADMAP/verify lists. Point-in-time snapshot: verify claims against code before acting on them later.
