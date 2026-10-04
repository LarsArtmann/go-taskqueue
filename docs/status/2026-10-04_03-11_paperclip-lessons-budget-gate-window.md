# Paperclip lessons window — budget claim gate shipped, wake coalescing adjudicated

**Window:** 2026-10-04 ~02:20–03:11 (single Crush session, no Task-Queue-ID — direct operator prompt)
**Scope:** research `github.com/paperclipai/paperclip` for lessons; execute the three highest-value ones (claim-time budget gate, retry-exhaustion journal event, task-level wake coalescing); full gate verification; docs.
**Verdict:** GREEN. All gates pass at HEAD; one foreign transient each (documented flake, concurrent-agent mid-restructure) diagnosed and re-verified.

## Self-critique (asked directly: what did you forget / do better / still improve?)

- **Forgot:** `scripts/ci-local.sh` (the full pre-push gate) never ran — I ran the equivalent gates piecemeal (per-module + root -race + facade parity + mirror clones + cmd gate + e2e) but ci-local bundles additional checks (doc-refs, dead-SHA, scoped gofmt, lint-baseline growth) I did not exercise this window.
- **Forgot:** the `budget` requeue class is invisible to `tq audit --journal`'s hint block (rate-limit has a dedicated hint; budget parks do not) and to any PapDashboard alert — observability half of the gate is open.
- **Forgot:** `tq work --agents` (the second pool, cmd/tq main.go ~594) carries no budget flags, so it stays ungated by design today — nothing documents that boundary.
- **Forgot:** a durable artifact of the paperclip comparison — the full lesson table lives only in the session chat + a one-line CHANGELOG provenance note; no docs/research/ capture.
- **Could do better:** paperclip internals (heartbeat/wake-queue/retry file:line claims) came from an AI fetch subagent, not from opening the cited sources myself; DESIGN.md was fetched raw, the rest trusted. Load-bearing tq-side claims were verified in-repo, but the paperclip-side detail behind lesson #2/#3 framing was never source-verified (verify-external-claims discipline).
- **Could do better:** first AGENTS.md bullet was written long (+495 B) and tripped the size guard; rewrote to one line. Should write doc additions in final compressed form from the start in a guard-pinned repo.
- **Could do better:** first test asserted `attempts == 1` after a clean completion — wrong model (attempts count BURNED failures, not claims); the test failure taught it and the assertion was replaced with the stronger fact-based proof (no `task.failed` fact). Could have read the burn semantics before writing the assertion.
- **Still improve:** the full root `-race` gate needed three runs (2 foreign flakes); a retry-once wrapper or known-flaky quarantine would stop re-adjudication cost every window.

## a) FULLY DONE

1. **Paperclip research** — DESIGN.md fetched raw; heartbeat/wake-queue/routines/retry/budget/session internals summarized with file paths; lessons extracted and each one verified against tq's code before acting (enqueue DedupKey semantics, `runRepo` pacing, `journal.Requeued` class set, budget Guard call sites).
2. **Claim-time budget gate SHIPPED** — `internal/worker/worker.go`: `Config.Budget` hook (nil = ungated, keeps the worker module dependency-free), gate at top of `execute()` before heartbeat machinery; blocked claims requeue WITHOUT attempt burn, jittered park to next local midnight (daily cap) or 15m cooldown (budget-cmd), class `budget`, deliberately OUTSIDE the env-streak breaker (policy gate, not environment defect). `internal/queue/queue.go`: `RequeueClassBudget`. `queue/queue.go` facade alias added after `check-facade-parity.sh` caught the skew (guard worked as designed). `cmd/tq/main.go`: `budgetClaimGate` adapter wired into the agent-pool `worker.New`; `budgetCmdRequeueWait` const.
3. **TestBudgetGateBlocksPaidTurn** — pins: zero executor runs while blocked; fact detail carries class `budget`, the gated reason, no `resume_closeout`, ~40ms jittered retry; task returns to PENDING with attempts untouched; completes after the gate opens with no `task.failed` fact anywhere. Green 3× under `-race`.
4. **Lesson adjudication #3** — retry-exhaustion journaling ALREADY EXISTS (`journal.DeadLettered` `task.dead-lettered` + the environmental dead-letter path). Verified, not assumed; no code change.
5. **Lesson adjudication #2 (wake coalescing)** — tq ALREADY implements the admit/defer/drain cycle as pacing: `runRepo` enforces "at most one live task per repo", changed items skip-while-occupied and re-enqueue naturally after completion (content-hash dedup keys make this sound). The one real residue — suppressed triggers leave no journal trace — scoped as a design task and filed as a TODO_LIST row (see b1).
6. **Docs** — AGENTS.md operational-contract bullet (one line, within the size guard after compression; a concurrent agent consciously reset `agentsDocMaxBytes` 15,200→15,400 in the same window); CHANGELOG `[Unreleased]/Added` entry with the gap it closes; TODO_LIST dated section with the wake-trace row (`check-todo-list.sh` green).
7. **Verification** — per-module gates for queue, worker, queue facade; `check-facade-parity.sh` green after fix; `check-mirror-clones.sh` 0 groups; `scripts/test-cmd-tq.sh` green (includes the size guard); `internal/e2e` green on re-run; `go mod vendor` clean; FULL root gate `build+vet+test -race` green (final run: zero failure lines).
8. **Foreign-failure triage** — `TestExactlyOnceUnderConcurrency` 19/20 (AGENTS.md-documented load flake; passes solo) and an e2e devmod `missing module declaration` break caused by a concurrent agent's mid-flight `internal/composition` module promotion (commit 0f88b17c, healed by their next commit) — both re-verified green, neither mine, neither reverted.

## b) PARTIALLY DONE

1. **Wake-trace durable evidence** — assessed and ROWED (TODO_LIST, dated section), not implemented: a `task.wake` fact (or RequeueEvidence-pattern equivalent) ripples through `internal/readmodel/events.go`, `internal/webui/components.go`, `cmd/tq/journalaudit.go` + their pinned tests; fact-shape-first design owed before touching backends.
2. **Budget-gate observability** — the gate works but nothing SEES it: no journalaudit hint, no papdashboard alert class, no webui reason rendering for budget-parked tasks (requeue class already rides the facts; consumers unaware).
3. **Paperclip WebUI lessons** — researched (operator stance "what is happening / does it need me / what do I do", one status vocabulary, monospace machine values, contextual-feedback expiry rules) but never compared against `tq serve`'s current UI; open assessment, nothing adopted.
4. **Remaining paperclip ideas** — failure-classification taxonomy, session-escalation ladder (`same_session → fresh_session → safer`), stranded-work human notices, zombie-run filter, routines/cron with concurrency+catch-up policies, goal-ancestry in prompts, secret-injection vs redaction: noted in chat, none designed, none filed outside the single wake-trace row.
5. **Paperclip-side source verification** — subagent-derived file:line claims unverified at source; DESIGN.md-only claims verified by direct fetch.

## c) NOT STARTED

1. Release flow for the change: no tag, no module release, no proxy checks, no `docs/release/` rc capture — `queue` + `worker` content waits for the next release window (per the standing release-flow contract).
2. Budget gate under the postgres backend: worker tests are sqlite; the gate is store-agnostic (plain `Requeue`), but no postgres-backend exercise ran this window.
3. `tq work --agents` pool budget flags (the ungated second pool).
4. Per-project (not just global) daily caps; hysteresis on the cap boundary (block at 100%, release <90%) — pure ideas, zero work.
5. Windows/CI-matrix confirmation for the new worker test (locally unix-only; test is plain `testing` + sqlite, likely fine, unconfirmed).
6. ci-local full run (see self-critique).

## d) TOTALLY FUCKED UP

Nothing at the code or tree level — HEAD is green across every gate this window ran. The two incidents worth recording honestly:

1. **Size-guard churn (self-inflicted, minor):** my first AGENTS.md addition tripped `TestAgentsDocSizeGuard` (+495 B, 3-line bullet). Fixed by compression, but in a repo where this guard is a known rake, writing long-then-trimming is the predictable mistake. Relatedly, my edit hit a stale-read rejection because a concurrent agent touched AGENTS.md mid-edit — the re-read-before-edit protocol caught it; no damage.
2. **Root-gate race attribution (process, not code):** my definitive root -race run raced the concurrent composition-module restructure and failed in `internal/e2e` with a cryptic devmod error. Correctly diagnosed as foreign (my diff touches no build scripts) and confirmed by re-run after their commit landed — but for a few minutes the tree LOOKED red from my perspective. Cheaper diagnosis next time: check `git log` freshness FIRST when a failure names infrastructure I did not touch.

## e) WHAT WE SHOULD IMPROVE

1. Source-verify external repo claims before they drive design framing (open 2–3 cited paths in paperclip next window; the lessons survived this time because every tq-side claim was verified locally).
2. Doc-size discipline: draft guard-pinned doc edits at final length; the reset-to-15,400 by a parallel agent + my compression shows the budget protocol is doing live work — keep bullets single-line by default.
3. Budget-gate visibility (b2) is the highest-leverage follow-up: a spend gate nobody can see is half a gate.
4. Flake economics: two of three root-gate runs were foreign flakes; retry-wrapping the full gate or tagging known-flaky tests would pay back every window.
5. Test semantics first: read the attempt-burn model before writing budget/ladder assertions (cost: one failed run).
6. Comparison-note durability: multi-source research (paperclip) should land a docs/research note in the same window, not live in chat.
7. Concurrent-agent coordination: AGENTS.md already says re-read/judge/build-on; this window validated it twice (AGENTS.md edit collision, composition restructure). Consider a tiny `git log -1 --format=%cd` freshness check before attributing any infra-shaped failure.

## f) Up to 50 things we should get done next (brainstorm — HARVEST routing required, most are ROADMAP fuel)

1. Design + ship the wake-trace fact (the rowed TODO item): fact shape first, then harvest skip wiring, then journalaudit + webui consumers.
2. `tq audit --journal`: add a budget-class hint block (like the rate-limit hint) + budget streak callout.
3. PapDashboard bridge: alert class for budget-BLOCKED claims (distinct from enqueue-refusal exhaustion alerts).
4. Webui: render `parked: budget` (class from fact detail) on task cards; status-sweep style badge.
5. `tq top`/`tq tasks`: budget-parked filter chip.
6. e2e test: task enqueued before the cap must not run after the cap (the exact money lesson, currently pinned only at worker level).
7. BudgetCmd result caching (short TTL) — avoid `sh -c` per claim on the cmd-gated path.
8. Pin the midnight computation against DST (23/25h days) with a table test.
9. Hysteresis option: block at cap, release below cap−1 to stop midnight-boundary thrash.
10. Per-project daily caps (importance-weighted) — paperclip has per-agent budgets.
11. Budget flags + gate wiring for the `tq work --agents` pool (or document its deliberate ungatedness in AGENTS.md).
12. `tq doctor`: extend `doctorBudget` with a claim-gate wiring self-check.
13. Failure-classification layer for retries (transient vs non-transient taxonomy; provider `retryNotBefore` from body, not just header).
14. Session-escalation ladder on agent retries (same_session → fresh_session → safer invocation).
15. Stranded-work escalation: human-visible notice for blocked-but-alive states (dep-blocked, uninvokable target), never auto-recovering human-assigned work.
16. Zombie-run filter: exclude in-process-dead runs as resume/coalesce targets in worker claims.
17. Document the lock/CAS invariant (serialized writer + `RowsAffected` re-checks) explicitly in AGENTS.md store-invariants.
18. Goal-ancestry injection in harvest payloads (item → section → repo purpose).
19. Secret-injection seam (minted per-run env, forbidden-key stripping, managed HOME allowlist) as the successor to output redaction.
20. Responsible-user attribution for agent GitHub operations.
21. Routines (cron/webhook recurring tasks) with `always_enqueue | skip_if_active | coalesce` + capped catch-up — ROADMAP.
22. `docs/research/paperclip-lessons.md`: capture the full comparison (wake model, budget 3-point enforcement, retry taxonomy, webui stance) durably.
23. Webui stance audit: does every `tq serve` screen answer what-is-happening / does-it-need-me / what-do-I-do.
24. Webui: one status vocabulary across badges/rows/charts; audit for drift.
25. Webui: machine values (IDs, cost, tokens, timestamps) monospace + consistent helpers.
26. Webui contextual feedback: stale execution errors auto-hide when superseded; late terminal outcomes refresh silently.
27. Root-cause or crisply suppress the 48 cmd/tq gopls false-positive diagnostics.
28. Robustify or quarantine-tag `TestExactlyOnceUnderConcurrency`.
29. Size-guard protocol: document when a conscious `agentsDocMaxBytes` reset is legitimate (stop ad-hoc 15.2k→15.4k churn).
30. Pre-commit ordering for facade parity (run `check-facade-parity.sh` before staging, not post-hoc like this window).
31. Release window: cut v0.3.x with the budget gate; module tags for `queue`/`worker`; proxy checks per docs/release flow.
32. DOMAIN_LANGUAGE.md: add "wake" vocabulary when the fact ships.
33. `docs/DOMAIN_LANGUAGE.md` sweep for "budget" terms introduced by the gate (cap vs guard vs gate).
34. Devmod self-check: `scripts/build-tq.sh` should fail with a clear message when `cmd/tq/go.mod` loses its module line (this window's cryptic foreign break).
35. journalaudit `--json`: expose budget-class requeue counts.
36. Readmodel projection: budget-class requeues in stats projections.
37. Worker `Config.Budget` example in `example_test.go` for library consumers.
38. Consider claim-gate ordering contract: budget check BEFORE prefetch/preparation seams if any appear (currently first — pin with a comment test if structure grows).
39. Verify paperclip heartbeat claims at source (open wake-queue use-cases.ts, routines.ts) before citing specifics in the research note.
40. ROADMAP: org-scale framing (approvals, org charts, budget pause semantics) parked tq-shaped.
41. Budget-parked tasks vs `--max-pending` working-set accounting (do parked tasks hold repo slots? verify + pin intended semantics).
42. `tq ask`/question flow interaction with budget parking (a parked-budget task that also has a pending question — precedence untested).
43. Windows CI matrix confirmation for the new worker test.
44. Run `scripts/ci-local.sh` end-to-end once on a green tree (owed from self-critique).
45. Retry-once wrapper for the full root gate (flake economics).
46. ARCHITECTURE note: claim-time vs enqueue-time gate ordering (cheap-first, paperclip's `loadGateFacts` lesson) as a documented pattern.
47. Sweep: any other pool constructor paths (examples/, embed) that should document the Budget hook.
48. CHANGELOG: link the budget gate entry to the research note once #22 lands.
49. Session-close bridge: does a budget-blocked window still mint review/status tasks correctly (budget refusal ordering vs sweepers) — e2e probe.
50. Archive this window's predecessors: the 02-36/02-38 reports' §f items triaged (dedup against the list above).

## g) Up to 3 questions I can NOT figure out myself

1. **Budget-park vs cancel semantics:** when the cap is spent, parked-until-midnight is my shipped behavior; paperclip instead CANCELS the run. Should sustained over-cap days instead dead-letter (or cancel) queued work so the morning queue isn't a wall of stale tasks — or is park-until-midnight exactly the operator intent?
2. **Wake-trace shape (owner architecture call, like the existing three-interfaces row):** new journal fact type `task.wake` (expressive, ripples through readmodel/webui/journalaudit + pinned tests) versus evidence key on existing facts (cheap, less queryable)? This decides the TODO row's effort 10x.
3. **Concurrent restructure coordination:** is the `internal/composition` module promotion (0f88b17c) the first of several splits landing tonight? If more module surgery is imminent, I should hold cross-module doc claims (facade counts, module lists in AGENTS.md) until it settles instead of re-editing after each commit.

*Report follows the a)-g) skeleton; §f is brainstorm input for docs-health HARVEST, not a commitment list.*
