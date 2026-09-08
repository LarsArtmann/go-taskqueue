# Status Report — Agent Reviews Shipped (can/should → built & verified)

**Date:** 2026-09-08 07:48 CEST
**Session scope:** "Can and should we leverage Agent 'reviews'?" — researched the
three reference projects, decided, designed, implemented, tested, verified,
documented. This report covers THIS session only (≈06:00–07:48), not the
parallel sessions that were committing alongside it (a836cd4 webui budget card,
0dd4409 ROUND6 plan both landed mid-session from other agents).

---

## a) FULLY DONE

| #  | Item                                                                                                                                                                                                                                                                                                                                   | Evidence                                                                                                                                                  |
| -- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1  | **Research verdict** — reviews in the three reference projects mapped onto go-taskqueue: mindwalk `internal/judge` (reviewer contributes findings, verdicts mechanical, invalid output retried), crush-daily `internal/app/review_service.go` (fact-stored reviews), go-crush-data (future session-transcript source)                  | this session's exploration; conclusion: can AND should — every primitive (facts, dedup keys, cqa findings→fix-tasks pattern, budget caps) already existed |
| 2  | **`review` executor** — `ReviewPayload`/`ReviewVerdict`/`ReviewFinding`/`ReviewResult` types, read-only reviewer prompt (item + commit SHA + files + repo contracts + exact `TQ_RESULT` output contract), `ParseResult` (strict verdict, lenient findings, severity normalization, `request_changes` without findings = invalid)       | `internal/executor/review.go`                                                                                                                             |
| 3  | **Executor test suite** — 11-case parse table, verdict contract (approve AND request_changes both complete; invalid output retryable, NOT permanent), prompt contract (item/SHA/files/read-only rule reach the agent argv), payload-contract-misses permanent, dirty tree = preflight requeue                                          | `internal/executor/review_test.go`                                                                                                                        |
| 4  | **`internal/review` sweeper** — journal-watermark sweep (starts at head at construction), agent completions → one review task (dedup `review:<task-id>`), review completions with `request_changes` + `--review-autofix` → fix tasks (dedup `reviewfix:<review-id>:<sha8(title)>`), fix prompt carries item + finding + severity + SHA | `internal/review/sweep.go`                                                                                                                                |
| 5  | **Sweeper test suite** — one-review-per-task, non-agent completions skipped, autofix minting + idempotent re-sweep, approve/off-switch no-mint, watermark resume across sweeps, head-start (no replay of pre-start completions), dedup-key stability                                                                                   | `internal/review/sweep_test.go`                                                                                                                           |
| 6  | **CLI wiring** — `tq agent-pool --review [--review-autofix]`; review executor registered in agent-pool (always) and `tq worker --agents` (so pools without `--review` carry review tasks minted elsewhere); sweep runs in the budget-gated tick AND in the `--once` drain watcher; startup log line                                    | `cmd/tq/main.go`                                                                                                                                          |
| 7  | **End-to-end smoke (stub agent)** — full cycle verified against a real pool process: agent task → review(request_changes + finding) → autofix minted fix task → fix ran → review of fix (approve) → queue drained, `--once` exited 0; verdict + findings stored in completion-fact detail, visible via `tq show`                       | `/tmp/tq-review-smoke` run, facts 1–12                                                                                                                    |
| 8  | **Full CI gate green** — `scripts/ci-local.sh`: vet, build, GOOS=windows, **all packages green under `-race`** (incl. new `internal/review`), gofmt, web UI smoke, doc-reference check, `nix build`, `nix flake check` all checks passed after the treefmt fix below                                                                   | session run 051 + re-run of `nix flake check`                                                                                                             |
| 9  | **Zero new lint findings** — `golangci-lint run ./... --new-from-rev` = 0 issues after fixing the 5 findings the annotation step caught on my lines (wsl ×2, lll, varnamelen ×2)                                                                                                                                                       | final lint run                                                                                                                                            |
| 10 | **Docs updated** — AGENTS.md (package table row + full `review` payload contract incl. loop-safety and watermark semantics), FEATURES.md (🟢 FULLY_FUNCTIONAL row with honest caveats), CHANGELOG Added section, README example with budget pairing, TODO_LIST 2 actionable seeds, ROADMAP idea                                        | respective files                                                                                                                                          |
| 11 | **On-sight fixes (not mine, fixed under the 5-minute rule)** — `internal/webui/fragments.templ` treefmt drift (parallel agent's pagination work broke `nix flake check`; `templ fmt` fixed it, re-check green), `internal/webui/render.go` import order                                                                                | git                                                                                                                                                       |

## b) PARTIALLY DONE

1. **Review feature is honest v1** — verdicts land in facts and `tq show`, but the
   web UI has no approve/request_changes badge or findings rendering (seeded in
   TODO_LIST Medium).
2. **Watermark gap is documented, not solved** — completions before pool start
   are never reviewed (same semantics as the papdashboard bridge). Catch-up
   (persisted watermark or `CompletedSince` + time index) is seeded in
   TODO_LIST Lower, cross-referenced to the bridge-watermark item.
3. **Fix tasks carry a degraded payload** — they inherit Repo/Model/Yolo from
   the review, but fall back to auto-detect verify; the reviewed task's
   `Verify` command and `TimeoutMinutes` are NOT propagated. Deliberate
   simplification, not yet revisited.
4. **`--once` review round-trip verified only by manual smoke** — the watcher
   sweep placement works (smoke proved it), but `internal/e2e` has no review
   scenario; nothing prevents regression there.
5. **Sweeper pagination loop (`Facts` page boundary)** is implemented but never
   exercised by a test with >PageSize facts — the `len(facts) < pageSize`
   termination logic is untested.
6. **`ParseResult` multi-marker behavior unpinned** — `resultLineRe` takes the
   FIRST `TQ_RESULT` line; no test pins what happens when a (weird) reviewer
   emits two.

## c) NOT STARTED

1. Web UI review-verdict rendering (TODO seed exists).
2. Review-watermark persistence / lookback catch-up (TODO seed exists).
3. Session-transcript reviews via go-crush-data (`AgentResult.SessionID` →
   transcript-fed reviewer; ROADMAP idea exists).
4. Committed review smoke script (`scripts/smoke/reviews.sh`) — my smoke was a
   throwaway in `/tmp`; CI has no committed reproduction of the loop.
5. `internal/e2e` review-loop coverage (stub agent + `agent-pool --once
   --review`).
6. Dogfood pool restarted WITH `--review` — feature shipped but the live pool
   is still running without it (owner decision, costs real money).
7. Rubric-style scored criteria (mindwalk's rubric layer) on top of the binary
   verdict — ROADMAP only.

## d) TOTALLY FUCKED UP

Nothing shipped broken — but full honesty on process failures this session:

1. **Smoke debugging burned ~4 failed runs on my own mistakes**: `TQ_DB` not
   exported for the pool process (twice), a bashism (`${!#}`) that silently
   broke under dash, and a `#!/bin/bash` shebang that doesn't exist on NixOS.
   The queue behaved CORRECTLY every time (permanent dead-letter for the
   missing repo, retryable failure for the bad verdict, correct drain-exit on
   not-due-soon requeues) — every "failure" was my stub, not the product. Still
   → that's exactly the kind of churn a committed smoke script would compress.
2. **First `NewSweeper` design was wrong against its own doc** — watermark was
   captured at first _Sweep_, not construction, so the doc comment lied and the
   first tests failed. Caught by tests within minutes, fixed by moving the head
   read into the constructor. Minor, but it shipped-wrong-then-fixed instead of
   right-first-time.
3. **LSP diagnostics repeatedly lied** (stale `strings` typecheck errors, gci
   warnings on lines that were already formatted). Per AGENTS.md I verified
   everything through the CLI, so nothing was damaged — but I did initially
   trust-and-act on one stale `typecheck` block before cross-checking.

## e) WHAT WE SHOULD IMPROVE

1. **Test the loop, not just the units** — add the review cycle to
   `internal/e2e` (the suite already drives `agent-pool --once` with a stub
   agent; one more scenario is cheap and prevents watcher-placement drift).
2. **Commit the smoke** — `scripts/smoke/reviews.sh` with the stateful stub
   agent (request_changes once, then approve) wired into ci-local so the loop
   can never silently rot.
3. **Propagate verify discipline into fix tasks** — carry the reviewed task's
   `Verify` (or at least `TimeoutMinutes`) so autofix output is held to the
   same gate as the original work.
4. **Split-brain watch: two "review" vocabularies** — executor-side
   (`ReviewPayload`/`ReviewResult` in `internal/executor`) vs sweeper-side
   (`internal/review` package owning the domain doc). Defensible split (run
   mechanics vs minting policy, mirroring executor/bridge), but DOMAIN_LANGUAGE.md
   doesn't define "review" yet — one sentence there would lock the terms.
5. **Watermark economics** — the review sweeper and the papdashboard bridge now
   share the exact same "journal starts at head per process" gap. One shared
   design (persisted watermark table or `CompletedSince` pushdown) should serve
   both — don't solve it twice.
6. **Dogfood with reviews on, deliberately** — the feature exists for the pool
   that eats this repo; shipping it enabled and watching one cycle (budget
   capped) is the only real validation left.
7. **Sweeper concurrency test** — mutex-protected, but no test fires two
   concurrent `Sweep` calls; cheap race-detector coverage.

## f) UP TO 50 THINGS TO GET DONE NEXT

_(brainstorm sorted by impact, not commitment — most items beyond the first
~10 are TODO_LIST/ROADMAP fuel for docs-health HARVEST)_

**Review feature maturation**

1. `internal/e2e` review-loop scenario (stub agent, `--once --review --review-autofix`)
2. Committed `scripts/smoke/reviews.sh` + wire into ci-local + CI
3. Web UI: review verdict badge + findings in task table/trail (from `ReviewResult` fact detail)
4. Propagate reviewed task's `Verify`/`TimeoutMinutes` into autofix fix tasks
5. Sweeper pagination test (>PageSize facts, page-boundary termination)
6. Concurrent `Sweep` race test
7. Pin `ParseResult` behavior on multiple `TQ_RESULT` lines (test + decide first-vs-last)
8. Review-watermark persistence OR `CompletedSince` pushdown + facts-time index (shared with bridge watermark design)
9. `--review-rounds` cap (explicit round counter) if ping-pong ever observed in dogfood
10. `tq review` catch-up subcommand (sweep without a pool, for cron-only setups)
11. Review metrics in `tq stats` (reviews/approves/requests per day) — CountFacts exists
12. `tq top` review column (review tasks in flight per repo)
13. Reviewer prompt: include repo's AGENTS.md excerpt mechanically instead of name-dropping it
14. Review `Extra` focus flag surface (`--review-focus "tests,security"`)

**Dogfood ops (this repo)**
15. Restart the live agent-pool with `--review` (+ budget headroom decision)
16. Post-run review of review quality: read 10 verdicts vs the diffs, tune the prompt
17. `crush --version` capture in pool startup line (existing D91-lite seed)
18. Sidecar log retention for `TQ_LOG_DIR` (existing seed)
19. Persisted bridge watermark design doc (merge with #8)

**Project backlog noticed / still open (from TODO_LIST, unaffected by this session)**
20. Cut v0.2.0 (BLOCKED on owner go/no-go; CHANGELOG is filling)
21. Verify CQA bridge against a live CQA instance (BLOCKED on owner URL/token)
22. Journal fact-stream `Subscribe(ctx, since Seq)` push design (inventory exists; dispatcher/slow-consumer policy are the real gaps)
23. SSE `Last-Event-ID` ↔ journal Seq resume mapping spike
24. ADR: lifecycle & streaming library stance (do/ro/cordis verdict distillation)
25. Web UI: `tq audit --json` golden already done — surface drift report in dashboard?
26. `TestCheckProjectsDir` unit tests (existing seed)
27. dprint gate decision for docs formatting (existing seed; today's templ drift shows format-gates matter)
28. Windows honesty follow-up: real `windows-latest` run for the e2e suites (marker option shipped)
29. `tq serve`: review-verdicts page (filtered list of review tasks + verdicts)
30. DLQ rescue UX for failed reviews (rescue with model override?)

**Nice-to-have / exploration**
31. Rubric-scored review criteria (mindwalk pattern) behind a flag
32. Session-transcript-fed reviewer via go-crush-data (ROADMAP)
33. Cross-repo review sampling: review 1-in-N harvest tasks from OTHER repos' pools (fleet learning)
34. Review-driven priority bump: repeated request_changes on the same repo lowers its harvest priority
35. `tq facts --type task.completed --detail` filter for verdict JSON grepping
36. Budget: separate review-budget vs work-budget caps (reviews are cheaper; today they share one cap)
37. Review executor: `--review-model` distinct from worker model (reviewer on a cheaper model)
38. Docs: DOMAIN_LANGUAGE.md entries for review/verdict/finding/fix-task
39. Dedup-key documentation page (review:/reviewfix:/seed:/harvest: naming convention table)
40. `tq show --json` for scripting (facts + payload machine-readable)
41. Flaky-watch: run the review smoke 10× in CI to catch ClaimDue-ordering races like the one I hit in tests
42. Consider `Deps`-based review ordering (review before dependent tasks claim) — probably YAGNI, write the one-paragraph rejection note
43. Review executor golden test for the full prompt text (pin regressions)
44. `budget.Guard` fail-open counting: reviews make silent budget loss louder — fail-closed option?
45. Changelog: backfill the auto-committed parallel-session features (budget stat card a836cd4) — verify they're in [Unreleased]
46. AGENTS.md: document the auto-commit daemon's `chore:` heuristic granularity (2/3/8-file commits make history noisy)
47. Templ formatting in CI: treefmt caught .templ drift only at flake-check stage — consider `templ fmt --check` earlier (pre-vet)
48. examples/: add a programmatic review-sweeper example (internal/review is importable in-repo)
49. `tq dlq` filter by error class (permanent vs transient) for triage
50. Docs-health HARVEST of this list into TODO_LIST/ROADMAP with routing rigor

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **Money**: should the live dogfood agent-pool be restarted with
   `--review --review-autofix`, and with how much extra daily-budget headroom
   (reviews roughly double enqueue volume)? Or should it run `--review`
   (verdicts only, no autofix) for one observation cycle first?
2. **Fix-task strictness**: when autofix mints a fix task, should it inherit the
   reviewed task's exact `Verify` command (same bar as the original work) or
   keep repo auto-detect as shipped? I can argue both; the call is a policy
   preference.
3. **Priority call**: with v0.2.0 still BLOCKED on your go/no-go and the review
   feature now shipping, which gets the next sessions — release closing
   (v0.2.0) or review-feature maturation (e2e + webui + dogfood enablement)?

---

_Format note: user explicitly requested `.md`; the status-report skill's HTML
default is overridden this once (per skill instructions, user instruction
wins). Point-in-time snapshot — stale the moment the pool restarts._
