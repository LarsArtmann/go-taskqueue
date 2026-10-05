**Written:** 2026-10-05 07:24 CEST (Crush interactive session, no dispatch ID)
**Session:** 2026-10-05 ~00:40 → 07:24 CEST — resumed continuation of the
00-33 window (which had paused mid-M2 mid-report).
**Scope executed:** the 00-33 report's §b debts, per owner instruction
"Break this down into multiple actionable steps… Execute and Verify them
one step at the time. Repeat until done."
**Tree at report time:** clean at 058dbbe0 (daemon swept everything,
2da0bad6 → 058dbbe0); concurrent agents landed foreign work mid-window
(`budget_status_test.go`, wake-trace memo, `e0f65d93` verify.sh flake-pin,
`058dbbe0` postscript). Master remains ahead of origin/master, UNPUSHED —
never pushes without owner authorization.

---

## a) FULLY DONE

### 1. Status-index debt paid (00-33 report + a foreign orphan)
- My `2026-10-05_00-33_…` report was still UNINDEXED — inserted at the top
  of the data rows with a full scope row.
- The gate then flagged a SECOND orphan: `2026-10-04_02-36_platform-
  migration-endgame-status.md` (ADR-0019 endgame, hours old, author
  session long closed) — indexed it too, honestly scoped from its header
  and marked "row added by the budget-visibility window".
- **BODY-DATE DRIFT on my own report** (filename 10-05, first body date
  10-04): fixed with a `**Written:** 2026-10-05 00:33 CEST` line ahead of
  the session range. Gates: `check-status-index.sh` rc=0 (remaining
  TRAILER/BLOAT warnings pre-existing, owner-blocked), `check-doc-refs.sh`
  rc=0.

### 2. §b2 — vendor + FULL root battery, twice
- `go mod vendor` no-op (rc=0; no new external deps).
- Root battery (`GOEXPERIMENT=jsonv2`, build + vet + test -race) rc=0
  **twice**: once before the e2e landed, once after (second run: 17 ok
  packages, `internal/e2e` 12.123s uncached, zero FAIL/panic).

### 3. M2.1 + M2.2 — claim-gate budget e2e LANDED
`internal/e2e/budget_claim_test.go` (`TestBudgetClaimGateParksOverCapSubprocess`,
unix-gated, t.Parallel) pins the money lesson through the REAL CLI:
- two single-item REPOS harvested uncapped → spent=2 enqueued-today
  (`Guard.SpentToday` counts `journal.Enqueued` facts since local
  midnight — verified in source before typing the assertions);
- `agent-pool --once --daily-budget 1` claims both → the stub agent NEVER
  runs (marker file absent), facts land exactly 2 enqueued / 2 claimed /
  2 requeued / 0 completed / 0 failed / 0 dead-lettered;
- every requeue detail decodes with class `budget`
  (`queue.RequeueEvidence`, the M1 visibility contract);
- both tasks stay PENDING, `Attempts==0` (park without burn), and each
  `NotBefore` is within ±2min of `budget.NextMidnight(time.Now())` — the
  DST-correct midnight (M2.3) proven END-TO-END through the real binary,
  not just unit tables.
Gates: targeted run PASS, full e2e package 6.146s green, gofmt clean,
root battery green (above). Daemon swept the file into git.

### 4. Report closure + honest bookkeeping
- Closure section appended to the 00-33 report (§b items → CLOSED with
  receipts, including the two-repo lesson below); index row re-annotated
  from "OWES" to "CLOSED same day"; doc gates re-run green.

### 5. M3 wake-trace DESIGN — verified DONE by a concurrent agent
The resume context claimed M3 was next-work-if-instructed; a TODO_LIST
grep showed `docs/planning/2026-10-05_wake-trace-design-memo.md` already
landed covering M3.1–M3.4 (option A `task.wake` recommended, draft fact
shape, pinned-test consumer checklist, §g-2 ruling framing; TODO row 473
BLOCKED on §g-2). Verified substance, built on it, did NOT duplicate.

## b) PARTIALLY DONE

1. **M2 (medium task) is 3 of 4**: M2.1/M2.2 (this window) + M2.3 DST pin
   (prior) landed; **M2.4 precedence pin is owner-gated** (§g-2 of the
   00-33 report): the proposed order — "budget park replaces the
   answer-clear; the merged answer survives in the payload and applies at
   resume" — cannot be pinned by a test until ratified (or refuted).
2. **Plan Tier 1% ("the 51%")**: fully executed except the M2.4 pin. Tier
   4%: M7 done (prior), M3 done (foreign agent), M4/M5/M6 remain.
3. **Self-improvement adoption**: the `**Written:**`-date-first convention
   is now applied in practice (two reports), but it is not yet written
   down anywhere — candidate clause for the M20 guard/protocol bundle.

## c) NOT STARTED (from the plan, untouched by THIS window by design)

- **M4** wake-trace implementation — owner-blocked on §g-2 behind the
  (landed) design memo.
- **M5** paperclip-lessons research note; **M6** release v0.3.x —
  owner-gated §g-1 (local tags).
- **M8.1** full ci-local run (M8.2's `scripts/root-gate.sh` appeared from
  another agent last window; its AGENTS.md wiring M8.3 unverified by me).
- **M9–M25** all untouched. M22 additionally §g-1-gated; M20 §g-3-gated.

## d) TOTALLY FUCKED UP

**Nothing destroyed, reverted, or left red.** No gate was left failing;
foreign work (`budget_status_test.go`, wake-trace memo, verify.sh pin,
postscript) was read, judged, built on, never reverted; no push, no tags.
Honest failure ledger:

1. **The "research 100% done, design settled" claim from 00-33 was wrong
   in its fixture premise**: first e2e run failed in 0.17s — 1 enqueue ≠
   2 — because harvest coalesces ONE live task per repo (wake pacing).
   The plan's OWN ground-truth section documents exactly this
   (wake coalescing as one-live-task-per-repo pacing); I designed a
   fixture that violated documented behavior and called the design
   settled. Fix was cheap (two-repo fixture, PASS on rerun); the lesson
   is not: "settled design" claims must re-read the plan's ground truth
   before typing. Recorded in the report closure note.
2. **My 00-33 report shipped unindexed AND with a body-date drift** —
   last session ended without running `check-status-index.sh`, so the
   drift sat undetected overnight until this window's indexing. The
   creation-time gate run is now part of how I close reports.
3. **Nearly duplicated M3 off a stale resume context** — the inherited
   "NOT STARTED / proceed to M3" claim was ~7h stale; only the TODO_LIST
   grep prevented re-researching a landed memo. Under N-concurrent-agents,
   every inherited done/not-done claim is a hypothesis until checked
   against the tree.
4. **Sequencing waste**: battery #1 ran before the e2e landed, forcing a
   second full battery (~minutes of compute). E2e-first, battery-once
   would have been the right order.

## e) WHAT WE SHOULD IMPROVE

1. **Resume contexts rot within an hour under concurrent agents** — this
   window's inherited brief was stale on three axes (M3 state, foreign
   e2e file, HEAD hash). A mechanical "window brief" (HEAD, my prior
   commits, plan-item states greped fresh) would beat prose summaries.
2. **Run `check-status-index.sh` at report CREATION**, not at indexing
   time — cheap hook/CI wiring; would have caught the 00-33 drift 7
   hours earlier. M20-bundle adjacent.
3. **The e2e suite now carries three near-identical budget harnesses**
   (enqueue-refusal, status-mint cap [foreign], claim-gate park [mine]):
   ~40 lines of shared stub/repos/pool boilerplate want one
   `budgetE2E` helper so the next budget pin is 5-minute work.
4. **Requeue-detail decoding now has three hand-rolled sites** (webui
   `parkedOnBudget`, journalaudit, my e2e raw `json.Unmarshal`) — no
   `ParseRequeueEvidence` exists yet. A fourth consumer should trigger
   promoting one parser into the shared companion surface.
5. **Advisory lint**: the new file adds wsl_v5/noctx warnings in line with
   the suite baseline — fine under the growth gate, but the next
   `lint-baseline` regen window should be run on a green tree.

## f) NEXT (prioritized; no new research — plan items + this session's residue)

1. **M6** release v0.3.x — CHANGELOG cut + queue/worker/root local tags +
   proxy checks (owner ruling §g-1 required first).
2. **M2.4** precedence pin test (owner ruling §g-2-adjacent, question 2).
3. **Parked-count split** stats/webui budget vs rate-limit (owner ruling
   §g-3: now vs M11 fold-in).
4. **M4** wake-trace implementation — harvest skip-path fact + consumers,
   per the landed memo (owner ruling §g-2 required).
5. **M5** `docs/research/paperclip-lessons.md` + source-verify 2-3 cited
   paperclip paths.
6. **M8.1** full `scripts/ci-local.sh` on the green tree; triage foreign
   breaks per retry protocol.
7. **M8.3** verify/wire `scripts/root-gate.sh` into AGENTS.md commands
   (foreign-landed; unverified by me).
8. **M11.1–M11.4** budget surfaces: `tq top` chip, `tq tasks --parked
   budget`, journalaudit `--json` budget counts, readmodel column.
9. **M9.1–M9.2** webui operator-stance audit (findings table only).
10. **M10.1–M10.7** webui systematic fixes (vocabulary, monospace,
    contextual-feedback rules + tests).
11. **M12.1–M12.5** retry failure-classification taxonomy + provider
    `retryNotBefore` from body + class on requeue evidence + conform pin.
12. **M13.1–M13.2** session-escalation ladder on agent retries.
13. **M14.1–M14.4** stranded-work human notices (never auto-recover
    human-assigned).
14. **M15.1–M15.2** zombie-run filter for resume/coalesce targets.
15. **M16.1** `tq work --agents` budget: flags+gate OR documented
    ungatedness.
16. **M17.1–M17.2** doctor budget-gate wiring self-check + devmod
    clear-error.
17. **M18.1** LSP cmd/tq false-positive root-cause or crisp suppression.
18. **M19.1** `TestExactlyOnceUnderConcurrency` robustify/quarantine +
    re-run protocol doc.
19. **M20.1–M20.5** guard/protocol docs bundle (HOLD until §g-3 settles;
    add: status-index-at-creation clause, Written-date convention).
20. **M21.1–M21.3** Windows CI confirm + `Config.Budget` example +
    examples doc mentions.
21. **M22.1–M22.4** budget policy v2 (per-project caps, hysteresis,
    working-set) — §g-1 gated.
22. **M23.1–M23.2** goal-ancestry injection in harvest payloads.
23. **M24.1–M24.3** secret-injection seam design + prototype + SECURITY.md.
24. **M25.1–M25.3** ROADMAP rows, archive sweep, CHANGELOG cross-links.
25. e2e `budgetE2E` shared harness helper (e3 above).
26. Promote `ParseRequeueEvidence` into the shared companion surface when
    the fourth consumer lands (e4 above).
27. Wire `check-status-index.sh` into report-creation flow (e2 above).
28. Record the "re-verify stale resume claims against the tree" lesson as
    an AGENTS.md one-liner (concurrent-agents section).

## g) QUESTIONS FOR THE OWNER (cannot self-answer)

1. **M6 release mechanics**: may I cut the v0.3.x root + queue + worker
   tags LOCALLY now (annotated, no push)? Tags are semi-irreversible and
   several agents share the tree — a ruling avoids a re-cut.
2. **M2.4 precedence semantics**: when a task's answer arrives while the
   budget is spent, is "budget park replaces the answer-clear; merged
   answer survives in the payload, applies at resume" the ratified order
   to pin — or should a pending answer DEFER the budget park?
3. **Parked-count labeling**: split the stats/webui "parked" count into
   budget vs rate-limit NOW (small, touches stats text + nowband card +
   their pins), or ride M11's budget-surfaces pass as one change?

---

**Gates run this session (all green):** check-status-index + check-doc-refs
rc=0 · go mod vendor rc=0 · root battery rc=0 ×2 (second: 17 ok packages,
internal/e2e 12.123s uncached) · e2e targeted PASS + package 6.146s ·
gofmt clean.
**Owed:** nothing from this window; §g 1–3 gate the next moves.
