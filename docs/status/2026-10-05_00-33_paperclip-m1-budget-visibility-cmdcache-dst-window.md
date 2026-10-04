# Paperclip-Aftermath Execution Window — M1 Budget Visibility + M7 Cmd Cache + M2.3 DST Pin

**Written:** 2026-10-05 00:33 CEST (Crush interactive session, no dispatch ID)
**Session:** 2026-10-04 ~23:35 → 2026-10-05 00:33 CEST
**Scope executed:** the freshly committed Pareto plan
`docs/planning/2026-10-04_23-59_paperclip-aftermath-budget-visibility-pareto-plan.md`
(bbb5daab) — Tier "1%" (M1) complete, M7 complete, M2.3 complete, M2.1/M2.2
designed but NOT written (interrupted by this report).
**Tree state at report time:** clean at 2bd4315a (daemon swept everything);
master 6 commits ahead of origin/master (unpushed — never pushes without
owner authorization). Concurrent agents were active throughout; foreign
work was read, judged, built on, never reverted.

---

## a) FULLY DONE

### M1.3+M1.4 — webui `parked: budget` badge (files: internal/webui)

- `parkedOnBudget(t task.Task, facts []journalFactView)` in
  `internal/webui/components.go` (after `factReason`): true iff the task is
  PENDING **and** its most recent `task.requeued` fact carries
  `class == queue.RequeueClassBudget`; non-Pending is history, not state;
  legacy class-less facts read as not-parked.
- `internal/webui/fragments.templ`: `taskDetailTimeline` gained the
  `t task.Task` parameter; when parked, renders a `display.Badge`
  ("parked: budget", BadgeWarning, BadgeSizeSM) plus a one-line
  explanation ("the claim-time budget gate parked this paid turn — it
  resumes when the daily cap or budget command resets (no attempt
  burned)"); theme-duality safe (`text-gray-500 dark:text-gray-400`, no
  new CSS, so no `.#webui-css` step owed). Caller inside `TaskDetail`
  updated.
- **Second caller fixed:** `internal/webui/render.go` `renderTaskFragments`
  (the SSE live-swap path) — the badge therefore also survives fragment
  swaps, not just full page loads.
- `templ generate` run from repo root (fragments_templ.go regenerated).
- Tests (`internal/webui/components_test.go`): `TestParkedOnBudget`
  (8-case table: pending+budget, latest-requeue-wins, later-rate-limit-
  unparks, running, completed, other class, legacy no-class, no requeue
  fact) and `TestParkedBudgetBadgeRenders` (badge + container class
  rendered; a single park must NOT fabricate a retry trail —
  `retryTrail`'s two-occurrence floor is exactly why the badge exists;
  rate-limit class renders nothing; a moved-on task renders nothing).
- **Gate:** webui module `GOWORK=off go build/vet/test -count=1` →
  `ok ... 10.340s`.

### M1.5+M1.6 — papdashboard budget-BLOCKED alert class (internal/bridge/papdashboard)

- `Bridge.trackBudgetBlocked` + `budgetBlockedAggregate(day)`
  (`agent-pool-budget-blocked-YYYY-MM-DD`) in `papdashboard.go`;
  `forward` switch gained `case journal.Requeued`. First budget-class
  requeue of a day fires `alert.triggered` ONCE (severity
  `BudgetSeverity`); the prior day's alert resolves on the next day's
  first blocked claim (mirrors `trackBudget`'s rollover shape).
- **Deliberately independent of `Config.DailyBudget`**: a budget-CMD pool
  blocks claims while never configuring an enqueue cap — the alert fires
  from the facts alone. Distinct from the existing enqueue-side
  "daily budget exhausted" alert `trackBudget` owns.
- State: `blockedDay`/`blockedAlerted` fields on Bridge (per-day,
  once-per-day semantics identical to the enqueue twin).
- Tests (`papdashboard_test.go`): `budgetBlockedFacts` helper (budget
  requeues + one rate-limit requeue proving other classes never fire);
  `TestBudgetBlockedAlertsOncePerDay` (exactly 1 trigger, correct
  aggregate, ZERO config) and `TestBudgetBlockedAlertResolvesOnDayRollover`
  (trigger → resolve → re-trigger, day-scoped aggregates).
- **Gate:** papdashboard module build/vet/test → `ok ... 0.728s`.

### M7 — BudgetCmd short-TTL cache (internal/budget + cmd/tq)

- `internal/budget/budget.go`: `DefaultCmdCacheTTL = 30s`;
  `Guard.WithCmdCache(ttl)` arms a shared `*cmdVerdictCache` (mutex'd,
  stores allow/refuse verdict + timestamp; copies of the armed Guard
  share the pointer, so the harvest gate and the claim gate agree within
  the window); `Guard.runBudgetCmd` extracted; `Check` consults the cache
  ONLY on the BudgetCmd branch (the daily-cap path stays a live SQL
  pushdown). Zero/negative TTL returns the guard unchanged (inert).
- Wiring (M7.3, no signature change): `cmd/tq/main.go` cmdAgentPool guard
  construction is now `budget.Guard{…}.WithCmdCache(budget.DefaultCmdCacheTTL)`
  — ONE `sh -c` per 30s process-wide instead of per claim (and per
  harvest tick).
- Tests (`budget_test.go`): `TestCheckCachesBudgetCmdVerdict` (refusal
  cached — 3 checks, 1 exec; budget clearing mid-window is honored only
  after TTL; past-TTL re-run sees the clearance; the fresh allow is
  itself cached), `TestCheckCmdCacheSharedAcrossCopies` (two Guard
  copies, one exec — the value-copy trap pinned), `TestWithCmdCacheZeroTTLIsInert`.
- **Gate:** `go test ./internal/budget -race -count=1` → ok.

### M2.3 — DST-correct midnight + boundary pin (internal/budget + cmd/tq)

- **Real bug fixed:** `budgetClaimGate` (cmd/tq/main.go) computed the
  budget park as `today-midnight + 24h` — wall arithmetic that lands an
  hour LONG on spring-forward days and an hour SHORT on fall-back days.
  Replaced with new `budget.NextMidnight(now)` (`time.Date(y,m,d+1,…)`
  normalization = true local midnight).
- `TestNextMidnightAcrossDSTDays`: 6 cases — plain UTC day, US
  spring-forward 2026-03-08, US fall-back 2026-11-01, the fall-back
  transition instant itself, EU 2026-03-29, EU 2026-10-25 — plus
  contrast assertions so the +24h formula can never silently coincide
  again. Embedded `time/tzdata` keeps the table hermetic (nix checkPhase
  has no zoneinfo).
- **Gate:** `scripts/test-cmd-tq.sh` → `ok ... 6.607s` (covers the
  budgetClaimGate change); budget package tests green.

### Verified foreign work (not mine, built on, NOT reverted)

- **M1.1+M1.2 landed mid-session by a concurrent agent** (3ff0877e):
  journalaudit `budget` class hint
  ("budget requeues park paid turns until the daily cap or budget-cmd
  window resets — no attempt burn, outside the env-streak breaker…",
  cmd/tq/journalaudit.go:459) + its fixture test
  `TestJournalAuditBudgetHintRenders` (journalaudit_test.go:560, uses the
  real store seam). I discovered the test already expecting my planned
  hint text while researching; the agent had landed hint+test together.
  M1.1/M1.2 are therefore CLOSED (by them).
- Other foreign commits swept into the same daemon commits (observed,
  untouched): `internal/webui/webui_test.go` +90 lines and most of
  render.go's diff in 0e4519f9; `scripts/root-gate.sh` (M8.2 of the same
  plan — another agent is executing it too) + an AGENTS.md tweak in
  c2323857; a sqlitev4 migration-test change in 2bd4315a.

## b) PARTIALLY DONE

1. **M2.1/M2.2 — claim-gate e2e ("enqueued before cap must not run after
   cap")**: research 100% done, code NOT written. Design is settled and
   ready to type: `internal/e2e/budget_claim_test.go` — writeRepo with
   2 items → uncapped `tq harvest` (spent=2) → `tq agent-pool --once
   --daily-budget 1` with a marker-touching stub agent → assert marker
   absent (agent never ran), `assertFactCounts` (2 enqueued / 2 claimed /
   2 requeued / 0 completed / 0 failed / 0 dead-lettered), every requeue
   detail class == `budget`, Attempts == 0 (no burn), and each task's
   `NotBefore` ≈ `budget.NextMidnight(time.Now())` ±2min (M2.2's
   midnight assertion, process-skew tolerant — no `Guard.Now` seam is
   exposed through the CLI, so the notBefore equality IS the assertion).
   All seams verified: `assertFactCounts`/`writeRepo`/`openStore`
   (internal/e2e/e2e_test.go:170,191,202), `Task.NotBefore time.Time`
   (internal/task/task.go:63), `s.List(ctx, queue.Filter{})`,
   `s.Facts(ctx, 0, 0)`.
2. **Session verification debt**: the four touched module gates are green
   (webui, papdashboard, budget -race, cmd/tq), but the session ended
   BEFORE the full root battery (`go build ./... && go vet ./... && go
   test ./... -race` with GOEXPERIMENT=jsonv2) and the ritual
   `go mod vendor`. No new external deps were added, so vendor drift is
   unlikely — but the AGENTS.md ordering says run it, and the root
   battery is the closeout gate. OWED before this window can be called
   closed.

## c) NOT STARTED (from the plan, in priority order)

- **M2.4** precedence probe (question-park vs budget park) — decided
  during research (see §e/§f), zero code.
- **M3** wake-trace DESIGN note (M3.1 ruling-input memo → M3.4 design
  doc); **M4** wake-trace implementation is owner-blocked on §g-2 behind
  it.
- **M5** paperclip-lessons research note; **M6** release v0.3.x (local
  tags only, push is owner-gated); **M8** full ci-local run (M8.2's
  root-gate.sh script appeared mid-session from another agent);
  **M9–M25** all untouched by this session.

## d) TOTALLY FUCKED UP

- **Nothing destroyed, reverted, or left red.** No gate was left failing;
  no foreign work was touched; no push, no tags, no force anything.
- Honest self-critique instead of damage: (1) I wrote a broken test-clock
  seam on the first `TestCheckCachesBudgetCmdVerdict` attempt (1s-per-call
  granularity can never cross a 60s TTL) — compile/run caught it,
  reworked to a controllable offset before the second run. (2) I wrote
  `import _ "time/tzdata"` inside a function body (compile error) and
  then hit "file modified since read" twice on my own append-modified
  test file, wasting three edit round-trips before switching to a python
  heredoc replace — the AGENTS.md "tab-bearing/scripted edits via script
  files" lesson applies to append-then-edit sequencing too. (3) The
  session PAUSED mid-M2 (this report was requested) — the e2e should
  have been finished first; its design is fully recorded in §b1 so the
  next window types it, not researches it.

## e) WHAT WE SHOULD IMPROVE

1. **The "parked" count now lies by omission**: stats' parked line
   (cmd/tq/main.go:1772 "rate-limit requeues waiting out their window")
   and the webui nowband parked card (fragments.templ:51-52, same title)
   count pending-with-future-notBefore — which budget parks now inflate
   under a rate-limit label. The M1 bundle makes budget parks visible on
   detail pages/audit/alerts, but the ONE-GLANCE count still conflates
   the classes. Splitting it needs the requeue class beside each parked
   task (M11-adjacent).
2. **The badge is detail-page-only** — the dashboard TABLE row cannot
   show it without per-row fact reads (N+1). Either accept (detail page
   + audit + alert cover the operator) or design a parked-class column
   fed by the same projection M11 builds.
3. **papdashboard blocked-alert resolution is day-rollover-only** — if a
   budget-cmd pool unblocks mid-day, the alert stays open until midnight.
   Resolving on the first COMPLETED fact after a blocked day would be
   tighter; left out to keep the shape identical to `trackBudget`'s.
4. **Append-then-edit sequencing**: my own `cat >>` changed mtime under
   my subsequent `edit` calls. Use the python-heredoc replace for append
   + import-update in ONE pass next time (the repo's documented hazard,
   re-learned the hard way).
5. **Plan-vs-fleet coordination**: another agent is executing the SAME
   plan (root-gate.sh landed mid-session). TODO_LIST/plan-file row
   claiming before starting a task would save duplicated research; the
   heal-daemon-sweep rails handled the daemon sweeps fine here (6 commits,
   tree clean, all work accounted for).

## f) NEXT (prioritized; ~40 items)

1. Type `internal/e2e/budget_claim_test.go` exactly as specified in §b1
   (design complete); run the e2e package gate.
2. M2.4 precedence pin — cheapest true shape: answer-clear then budget
   spent → requeue class `budget` replaces the answer-clear, merged
   answer payload survives (RecordAnswer via type assertion on the
   concrete store, worker-level test), OR fold a `tq ask`-shaped
   assertion into the §f1 e2e; needs the §g2 ruling first.
3. Root battery + `go mod vendor` for this window's changes (owed §b2).
4. CHANGELOG [Unreleased] entries: budget-blocked alert class, parked:
   budget badge, journalaudit budget hint, BudgetCmd TTL cache,
   NextMidnight DST fix.
5. FEATURES.md rows for the four visibility surfaces.
6. AGENTS.md: extend the claim-time budget bullet with "alert + audit
   hint + badge" (M1.7; size-guard aware — conscious 15.2k reset is
   standing).
7. M3.1–M3.4: wake-trace design note (fact-type vs evidence-key tradeoffs
   → §g-2 ruling input; consumer ripple inventory: journalaudit/webui/
   readmodel + their pinned fact-type tables; recommendation).
8. M4 (owner-blocked on §g-2): wake-trace implementation.
9. M11.1–M11.4 budget surfaces: `tq top` chip, `tq tasks --parked budget`,
   journalaudit `--json` budgetCounts, readmodel projection — and fold
   the §e1 parked-count class split into this pass.
10. M5 paperclip-lessons.md (source-verify 2-3 cited paths while writing).
11. M6 release v0.3.x — M6.1 green battery first; local tags ONLY
    (queue+worker modules), no push; proxy/pkg.go.dev checks are
    post-push, so they stay owner-gated (see §g1).
12. M8.1 full `scripts/ci-local.sh` on this green tree (root-gate.sh
    exists now — use it for the root battery per M8.2's design).
13. M9 webui operator-stance audit (findings table only).
14. M10 webui systematic fixes (vocabulary sweep, monospace values,
    contextual feedback rules).
15. M12 retry failure-classification layer (taxonomy + provider
    retryNotBefore from BODY + class on requeue evidence).
16. M13 session-escalation ladder on agent retries.
17. M14 stranded-work notices (dep-blocked/uninvokable get human-visible
    facts, never auto-recovery).
18. M15 zombie-run filter (exclude in-process-dead runs as resume targets).
19. M16 `tq work --agents` budget gate or documented ungatedness.
20. M17 doctor budget-gate wiring self-check (extend `doctorBudget`) +
    build-tq.sh module-decl preflight.
21. M18 LSP false-positive root-cause or documented suppression recipe.
22. M19 TestExactlyOnceUnderConcurrency robustness/quarantine decision.
23. M20 guard/protocol docs — HELD for §g-3 (restructure settles).
24. M21 windows CI canary confirm + `Config.Budget` runnable example +
    examples doc mentions.
25. M22 budget policy v2 — owner-blocked on §g-1.
26. M23 goal-ancestry injection in harvest payloads.
27. M24 secret-injection seam design note (+ prototype stripping).
28. M25 ROADMAP parking rows + archive sweep of 02-36/02-38 §f +
    CHANGELOG↔research cross-links.
29. Table-row parked-class column design (fold-or-reject with M11, §e2).
30. papdashboard: consider completion-based resolution for the blocked
    alert (§e3) — design note or explicit rejection.
31. TODO_LIST row mint: one dated row noting M1+M7+M2.3 SHIPPED and M2.1/
    M2.2/M2.4 in-flight (check-todo-list green).
32. Status-index row (done in this pass — see below), then re-run
    `scripts/check-status-index.sh`.
33. check-doc-refs after any new docs (the design note in §f7).
34. `tq audit --journal` on the DOGFOOL journal (scratch TQ_DB!) to
    eyeball the new budget hint against real facts before the next
    pool deploy.
35. Consider surfacing the 30s cache TTL in `tq doctor` budget output
    (operators should know the verdict staleness bound).
36. webui smoke: assert the parked:budget badge appears on a seeded
    budget-parked fixture (scripts/smoke/webui.sh extension).
37. e2e attempts==0 assertion included in §f1 (double-pinned here so it
    is not forgotten).
38. Decide whether `budget.WithCmdCache` TTL belongs in pool.conf
    (`cmd-cache-ttl =`) or stays a constant — owner-config surface call.
39. Facade parity: `queue.RequeueClassBudget` already aliased (verified
    in queue/queue.go:344 region earlier); confirm check-facade-parity
    stays green after M4's fact-type work.
40. After M2.1 lands: re-run `TestBudgetGateBlocksPaidTurn` +
    `TestBudgetRefusalSubprocess` + the new e2e together as the budget
    regression triad in one window.

## g) QUESTIONS FOR THE OWNER (cannot self-answer)

1. **M6 release mechanics**: may I cut the v0.3.x root + queue + worker
   tags LOCALLY now (annotated, no push, per the plan's M6.3), or does
   the release wait for a fully owner-run window? The plan authorizes
   local tags, but tags are semi-irreversible and two agents are working
   the same tree — a ruling avoids a re-cut.
2. **M2.4 precedence semantics**: when a task's answer arrives while the
   budget is spent, is "budget park replaces the answer-clear; the merged
   answer survives in the payload and applies at resume" the ratified
   order to pin — or should a pending answer DEFER the budget park (let
   the answered task run first)?
3. **Parked-count labeling**: split the stats/webui "parked" count into
   budget vs rate-limit NOW (small, touches stats text + nowband card +
   their pins), or ride M11's budget-surfaces pass as one change?

---

**Gates run this session (all green):** webui module (build/vet/test,
10.340s) · papdashboard module (0.728s) · `go test ./internal/budget
-race -count=1` · `scripts/test-cmd-tq.sh` (6.607s).
**Owed:** root battery + `go mod vendor` (§b2); e2e package gate after
§f1.

---

## Closure (2026-10-05 ~01:15 CEST, session resumed and finished)

The §b debts are paid; receipts inline.

1. **§b1 M2.1/M2.2 e2e — LANDED.** `internal/e2e/budget_claim_test.go`
   (`TestBudgetClaimGateParksOverCapSubprocess`, unix-gated) pins the
   money lesson through the real CLI: two single-item REPOS harvested
   uncapped (spent=2 enqueued-today), then `agent-pool --once
   --daily-budget 1` claims both — the stub agent never runs (marker
   absent), facts land 2 enqueued / 2 claimed / 2 requeued / 0 completed
   / 0 failed / 0 dead-lettered, every requeue detail decodes with class
   `budget` (`queue.RequeueEvidence`), and both tasks stay PENDING with
   `Attempts==0` and `NotBefore` within ±2min of `budget.NextMidnight` —
   the DST-correct midnight proven end-to-end. **First-run correction,
   recorded honestly:** the settled §b1 design assumed one repo with two
   items; the gate test failed 1-enqueue-≠-2 because harvest coalesces
   ONE live task per repo (the wake-pacing behavior). Two repos is the
   honest fixture; not a product bug, a wrong test premise.
2. **§b2 vendor + root battery — DONE.** `go mod vendor` no-op (rc=0);
   full root battery rc=0 TWICE (before and after the e2e landed; second
   run 17 ok packages, internal/e2e 12.1s uncached). e2e package gate
   6.1s green; gofmt clean.
3. **M3 wake-trace DESIGN — closed by a CONCURRENT agent, not this
   window:** `docs/planning/2026-10-05_wake-trace-design-memo.md` covers
   M3.1–M3.4 (option tradeoffs + recommendation A `task.wake` fact,
   draft fact shape, pinned-test consumer checklist, §g-2 ruling
   framing); TODO row 473 tracks the owner-blocked implementation (M4).

§g owner questions 1–3 stand UNCHANGED; they gate M6 (tags), M2.4
(precedence pin), and the parked-count split (M11 fold-in vs now).
