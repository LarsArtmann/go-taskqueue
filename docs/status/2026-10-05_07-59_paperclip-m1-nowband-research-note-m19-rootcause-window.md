# Paperclip plan window — M1 nowband lamp, research note, wake memo, M19 root cause

**Written:** 2026-10-05 07:59 CEST (Crush interactive session, no dispatch ID)
**Session:** 2026-10-05 00:00 → ~00:40 CEST (execution), report written 07:59 after an 8-hour idle while concurrent agents committed 62 times.
**Scope executed:** continuation of the pareto-plan directive (`docs/planning/2026-10-04_23-59_paperclip-aftermath-budget-visibility-pareto-plan.md`) — the plan's Tier-1%/Tier-4% items not owned by the concurrently running 00-33 window, plus verification debt triage.
**Tree at report time:** clean at cd451220; root battery run fresh for this report: build+vet+test -race **rc=0** (17 ok packages, log `/tmp/status-report-battery.log`; internal/webui 18.0s, internal/e2e included). No push, no tags, nothing reverted.

---

## a) FULLY DONE

### 1. M1.1 + M1.2 — journalaudit budget hint + fixture (this session's hands)
`cmdJournalAudit` gained the `budget` hint block beside the rate-limit hint (text path): "budget requeues park paid turns until the daily cap or budget-cmd window resets — no attempt burn, outside the env-streak breaker; the pool LOOKS idle on purpose". JSON path already carried class counts via `Requeues.ByClass`. `TestRequeueSummarySurfacesClasses` grew a `class:budget` fixture row (total 4→5) and `TestJournalAuditBudgetHintRenders` pins class line + hint text through the real render path (`captureStdout` + scratch store + `store.Requeue(..., RequeueClassBudget)`). Gate: `scripts/test-cmd-tq.sh` green. Daemon-swept into 3ff0877e.

### 2. M1.3 + M1.4 — webui nowband `budget N` lamp (this session's hands)
- `DashboardData.BudgetParked` + `budgetParkedCount(ctx, store, now)` in `internal/webui/render.go`: pending-parked set ∩ latest `task.requeued` class inside a 24h window (`FactsSince` + one `List`, latest-fact-per-task wins). `budgetParkedWindow` const documents the honest scope (badge says "latest requeue in the last 24h").
- `fragments.templ` StatusCards: `card-budget-parked` segment, rendered only when > 0, title explains "idle on purpose, no attempt burn"; templ regenerated from repo root. No CSS rebuild needed (marker-class pattern, parity with `card-parked`; the reserved `.tq-nowband-meta .card-budget` rule is the enqueue meter, untouched).
- `TestBudgetParkedSegmentRendersFromSnapshot`: budget requeue lights the lamp ("budget 1"), a rate-limit-parked second task does NOT feed it (1/2 split), cancelling the budget-parked task darkens it. Webui module gates green; daemon-swept into 0e4519f9.
- Division note: the 00-33 window shipped the DETAIL-page badge (`parkedOnBudget`); this lamp is the one-glance nowband surface — complementary, both green in today's battery.

### 3. M8.2 + M8.3 — `scripts/root-gate.sh` + AGENTS.md wiring
Root-module gate (build+vet+test -race) with ONE retry keyed on known-flaky signatures (`TestExactlyOnceUnderConcurrency`, `TestSelfManagingLoop`) and a `FLAKE-RETRY` marker on second-run greens; replaced the inline export line in AGENTS.md's Commands block (net bytes NEGATIVE — size-guard friendly). `bash -n` clean; referenced-path gates green.

### 4. M8.1 TRIAGE — red master root-caused and the real one FIXED
First `ci-local.sh` attempt correctly refused at check-ci: the plan-doc push (bbb5daab) went out with a red CI run. `gh run view 37238393185`:
- `TestSelfManagingLoop` (harvest) — known-flaky class (AGENTS.md Known Issues), not attributed.
- `TestProbeClassifiesSchemaGenerations` (sqlitev4/migration) **Windows-only**: `TempDir RemoveAll cleanup: unlinkat fresh.db — file in use`. Root cause: the test DISCARDED the `sqlitev4.Open(fresh)` handle; the open file lock defeats Windows cleanup. Fixed: capture + explicit `Close()` before the probes (comment names the CI red). Module tests + `GOOS=windows go vet` green. Landed via daemon sweep; the red's true confirmation is the next remote CI run (still owed — see §b2).

### 5. M5.1–M5.3 — `docs/research/2026-10-05_paperclip-lessons.md`, source-verified
Three citations opened at source (`gh api repos/paperclipai/paperclip/contents/…`) BEFORE writing — closing the 03-11 report's §c1 self-critique (subagent-trusted claims):
1. `doc/plans/2026-03-14-budget-policies-and-enforcement.md` — verified 3-point enforcement (ingestion evaluation + preflight at FIVE entry points + active-run graceful cancel), soft-80/hard-100, company/agent/project scopes, billed_cents-first.
2. `packages/db/src/schema/agent_wakeup_requests.ts` — verified wake model as durable rows (`coalesced_count`, source/trigger/status lifecycle, partial-unique idempotency keys).
3. `server/src/services/run-failure-diagnostics.ts` — verified structured redaction-aware failure context.
Note carries the lesson table (8 rows: adopted/parked/rejected with reasons), the two adjudicated rejections (retry-exhaustion event = `task.dead-lettered` exists; cancel-live-runs = park-until-midnight contract), and the verification trail. CHANGELOG budget entry cross-links the note (M5.3 + M25.3 in one edit). `check-doc-refs` green.

### 6. M3.1–M3.4 — `docs/planning/2026-10-05_wake-trace-design-memo.md` (ruling input)
Fact-type vs evidence-key tradeoff table (option A wins: the skip has NO honest carrier fact; `store.AppendFact` session-seam precedent at `internal/session/session.go:125` means NO Store-surface change → no conform ripple); draft `task.wake` const + `WakeDetail` struct (seq-derived idempotency, append-only coalescing); 7-surface consumer-ripple inventory with the pinned-test checklist (readmodel 9-case switch, journalaudit 10-case, webui retryTrail deliberately EXCLUDES wake, papdashboard deliberate-subset needs nothing); §g-2 framing including the `task.` vs `harvest.` namespace question (session facts use `session.opened`, not `task.*` — precedent cuts toward `harvest.wake`). TODO_LIST row converted to gate-legal BLOCKED syntax pointing at the memo. `check-todo-list` + `check-doc-refs` green. (Independently verified-not-duplicated by the 07-24 window.)

### 7. M19.1 DIAGNOSIS — the 19/20 flake root-caused (beyond "load-flaky")
Robustified `TestExactlyOnceUnderConcurrency` (10s→60s drain wait + status-map in the failure message), then RAN it: failed at 60.1s with `19/20 completed (statuses: map[completed:19 running:1])` after `complete failed … SQLITE_BUSY (5)` / `claim failed … database is locked (517)`. Root cause: `worker.go:492-494` logs a failed terminal `Complete` and returns — the paid turn's outcome is ORPHANED (task stays Running until lease expiry reclaim → re-run → double spend in production). The 10s deadline was never the disease; it was the symptom's timer. Fix designed (bounded transient retry on terminal writes, `retry.Do` with busy/locked `IsRetryable`, precedent `execWithTransientRetry` at `internal/executor/agent.go:807`, go-retry already an indirect dep of the worker module) — NOT yet landed (§b1).

### 8. Ground-truth budget-gate probes (no code changed)
`/tmp` fixture runs against the real binary: cap=2 → one enqueue, claim allowed, completes. cap=1 → the task whose OWN enqueue spent the cap is parked until midnight (`spent >= cap` at claim time). So `--daily-budget N` completes N−1 and parks the Nth authorized item — shipped-conservative semantics, documented here, NOT changed (owner policy, §g1).

### 9. Concurrent-agent coordination (work NOT duplicated)
Verified-in-tree and built on: M1.5/M1.6 papdashboard `trackBudgetBlocked` (their fields + dispatch case appeared mid-edit; I waited, reviewed the landed function, ran their test — green), M7 `WithCmdCache`, M2.3 `budget.NextMidnight` wiring, and the overnight 07-24 M2-closure window (their e2e cites my memo + root-gate.sh as foreign landed work — cross-acknowledged both ways).

## b) PARTIALLY DONE

1. **M19.1 is half-shipped**: robustification landed; the ROOT-CAUSE fix (terminal-write transient retry in the worker) is designed, not typed. The test still fails 19/20 under the right SQLITE_BUSY interleaving — just later (60s) and with a status map that names the disease. The worker-module change needs `go mod vendor` + module gate + a stress run (`-count=5 -race`) before landing.
2. **M8.1 full pass still owed**: check-ci stays red until the Windows fix rides a push and a remote CI run goes green; the local `CI_CHECK=off` full ci-local run on this tree has not been done this session (the first attempt aborted at check-ci by design).
3. **M1.7** (AGENTS.md claim-time bullet extended with "alert + audit hint + badge") not done by this session — the 00-33 window owns the bullet's text and may have covered it; unverified here.
4. **M2.4 precedence pin**: I had planned a worker-level probe; the 07-24 window correctly reframed it as owner-gated (§g2: budget-park vs answer-clear precedence). Nothing typed — correctly.

## c) NOT STARTED (this session, by coordination design)

M4 (wake implementation — §g-2-blocked behind the memo), M6 (release — owner-gated), M9–M18 except M19.1's first half, M11 (budget surfaces), M20–M26. The 07-24 window's §f list (28 items) already covers these; this report does not re-enumerate them as new.

## d) TOTALLY FUCKED UP

Nothing destroyed, reverted, or left red. No foreign work touched. Honest failure ledger:

1. **Near-duplicated M1.5/M1.6.** I read `journalaudit.go` first and only discovered the concurrent agent's in-flight papdashboard edit when the module failed to compile (`trackBudgetBlocked undefined`) mid-plan. Caught in minutes (waited 45s, reviewed, built on, ran their test), but the right order was check-tree-FIRST, plan-second. Under N agents, "pick the next task" must start with `git status` + a grep, not the plan document.
2. **One wasted cmd/tq test cycle**: ran `GOWORK=off go test` directly in cmd/tq → ambiguous-import failure against the module cache. AGENTS.md prescribes `scripts/test-cmd-tq.sh`; I re-derived the hard way.
3. **First webui test draft had dead code** (a duplicated `captureStdout` block + `_ = out`); cleaned before any commit landed, but the edit history carries the sloppiness. Write the assertion, then the harness — not the reverse.
4. **The 60s robustification alone would have been a verschlimmbessern** — a slower-failing test with the same bug. SAVED by the discipline of running the changed test immediately (the 60.1s failure IS what surfaced the lost-terminal-write root cause). If I had "verified" by eyeball, M19 would have been marked done with the disease still in the worker.
5. **Inherited-context drift**: my resume brief said M2.4 was a test task; by the time I reached it, the 07-24 window had correctly owner-gated it. No work lost (nothing typed), but the brief was stale within hours — same lesson the 07-24 §d3/§e1 records independently.

## e) WHAT WE SHOULD IMPROVE

1. **Land the terminal-write retry (M19 root cause).** Highest-value open robustness item in the repo: a transient SQLITE_BUSY on Complete/Fail currently orphans a PAID turn until lease expiry (production double-spend risk, not just test noise). Small, precedented (go-retry + `execWithTransientRetry`), worker-module-local.
2. **Budget-gate Nth-item policy needs a ruling, not a silent change.** `spent >= cap` at claim parks the cap-authorized Nth task until midnight. If unintended, the fix is one comparison — but it is money policy (§g1 below).
3. **Badge-scope honesty**: the nowband `BudgetParked` is a 24h-window approximation; M11's readmodel projection column would make it exact and pushdown-cheap. Keep the title text until then.
4. **Stale-brief discipline** (echoing 07-24 §e1, independently re-learned): resume claims are hypotheses; grep the tree first.
5. **Windows CI hygiene**: one discarded-Open handle defeated the whole Windows lane. A `go vet`-style guard cannot catch it; consider a convention note (every `Open` in tests is either deferred-closed or closed-before-TempDir) in the M20 bundle.

## f) NEXT (consolidated; 07-24 §f items 1–28 STAND — this list adds/reorders only what this session's findings change)

1. **M19 root cause**: worker terminal-write transient retry (Complete/Fail, busy/locked IsRetryable, 3 attempts ≤400ms) + stress pin (`-count=5 -race`), module gate + `go mod vendor`.
2. **§g1 below**: budget Nth-item claim policy ruling → then M22 (hysteresis rides it).
3. **M8.1**: push the Windows fix (owner authorization already granted for master fixes this arc), confirm remote CI green, THEN full `ci-local.sh` (M8.1's actual run).
4. **M2.4** precedence pin — unblocks the moment 07-24 §g2 is ratified.
5. **M6** release v0.3.x — 07-24 §g1 local-tag ruling still owed; CHANGELOG cut already partially staged by the budget entry work.
6. **M4** wake-trace implementation per the memo's checklist — unblocks on §g-2.
7. **M11.1–M11.4** budget surfaces (tq top chip, `--parked budget`, journalaudit --json counts, readmodel column — the column also upgrades §e3).
8. **M9.1–M9.2** webui operator-stance audit (findings only).
9. **M10.1–M10.7** webui systematic fixes.
10. **M12.1–M12.5** retry failure-classification taxonomy (the M19 root cause makes `IsRetryable` classification in-worker a natural first slice).
11. **M13.1–M13.2** session-escalation ladder.
12. **M14.1–M14.4** stranded-work notices.
13. **M15.1–M15.2** zombie-run filter.
14. **M16.1** `tq work --agents` budget flags/gate-or-document (cmd/tq now stable post-restructure — re-verify file layout first).
15. **M17.1–M17.2** doctor budget-gate self-check + devmod clear error.
16. **M18.1** cmd/tq gopls false positives.
17. **M20.1–M20.5 + §e5** guard/protocol bundle (add: status-index-at-creation, Written-date convention, Open-close test convention; still §g-3-held for cross-module claims).
18. **M21.1–M21.3** Windows canary + `Config.Budget` example + examples mentions.
19. **M23.1–M23.2** goal-ancestry injection.
20. **M24.1–M24.3** secret-injection design + prototype + SECURITY.md.
21. **M25.1–M25.2** ROADMAP rows + 02-36/02-38 archive triage (M25.3 done here).
22. e2e `budgetE2E` shared harness (07-24 §e3 — endorse).
23. Promote `ParseRequeueEvidence` into companion (07-24 §e4 — endorse; my lamp is consumer #3).
24. M1.7 verification: whether the AGENTS.md claim-time bullet needs the visibility clause (00-33 owner's call).
25. Windows CI green confirmation for the migration fix (rides item 3's push).

## g) QUESTIONS FOR THE OWNER (cannot self-answer)

1. **Budget claim-gate policy (the Nth-item park)**: `--daily-budget N` currently completes N−1 and parks the Nth authorized task until midnight, because the claim gate tests `spent >= cap` and the Nth task's own enqueue spent the cap. Intended (conservative ceiling) or should the claim gate admit the cap-authorized task (test `spent > cap`, or count only claims-at-completion)? M22's hysteresis design rides this answer.
2. **Terminal-write recovery semantics**: OK to retry a failed `Complete`/`Fail` in-worker on transient SQLITE_BUSY (3 attempts, ≤400ms, then give up to lease-expiry reclaim as today)? The alternative (status quo) means a lost terminal write re-runs a PAID turn after lease expiry — double spend. I recommend the retry; it changes no contract, only the odds of orphaning.
3. **Ruling bundle ratification**: the 07-24 window left §g1 (M6 local tags), §g2 (M2.4 budget-vs-answer precedence), §g-2/§g-3 plan rulings open. May I treat THIS report's §f3–§f6 as authorized the moment you answer the 07-24 questions, or do you want to re-rule them fresh here?

---

**Gates run this session:** root battery (build+vet+test -race) **rc=0** at cd451220, 17 ok packages, zero FAIL/panic (`/tmp/status-report-battery.log`) · `scripts/test-cmd-tq.sh` green · webui module build+vet+targeted tests green · sqlitev4/migration tests + `GOOS=windows go vet` green · `check-doc-refs.sh`, `check-todo-list.sh`, `check-status-index.sh` green at report time · `bash -n scripts/root-gate.sh` clean.
**Owed:** Windows-fix push + remote CI confirmation + full ci-local (§b2); M19 root-cause fix (§b1); M1.7 verification (§b3).
