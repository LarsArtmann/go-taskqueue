# Status Report — 2026-10-05 14:09 CEST — Dupe-Review Execution Window (Parity Pins + Verification)

Session scope: execute the 12-27 dupe-review report's §f list — fix my own
index-table break, land the wire-key parity pins, close the safe doc fixes,
verify the LSP-noise classes via CLI gates, HARVEST the remainder into
TODO_LIST.md. Read-only-then-test session: code changes are test + comment
files only; the daemon folded every change (e2a7a825, 93245412, feb791f4,
7c3fd3c6); tree clean at report time. A concurrent window shipped its own
`art-dupl-t4-zero-dedup-sweep` report mid-session (folded in e2a7a825).

## Self-Critique (asked first: forgotten / better / still improvable)

**Forgotten:**

1. Mutation verification. The parity pins' entire VALUE is failing on drift,
   and I argued the mechanism ("renamed tag → zero values → assertion
   fails") instead of demonstrating it once. The repo's own 10-15 window
   dinged this exact class ("fixture-honesty was luck, not process"). The
   pin is green but its failure mode is unproven — f2 in the list below.
2. GOCACHE discipline on the FIRST go invocation (readmodel gate ran on the
   host GOCACHE symlink — the ENOSPC hazard AGENTS documents; small build,
   worked by luck). Adopted /tmp/go-build-cache only from the second batch.
3. Disposition of the three §f items I silently skipped (f23 cross-
   satisfaction test, f24 AGENTS line, f27 lessons.md): rationale lived in
   my head + chat, nowhere HARVEST will read. The shipped-item disposition
   DID land in the TODO section header; the skipped-item disposition didn't.
4. Post-hoc TQ_DB thought: I ran scripts/test-cmd-tq.sh without consciously
   exporting TQ_DB=<scratch> (AGENTS hazard: shells inherit the PRODUCTION
   journal). The script manages its own temp DBs (log shows /tmp fixtures),
   so no harm — but I noticed the hazard AFTER running, not before.

**Could have done better:**

1. My own README edit introduced a blank line INSIDE the index table
   (careless new_string construction). Caught by my own grep -A1 two steps
   later; `check-status-index.sh` would never have caught it (presence-only
   check). Table edits need an immediate neighborhood re-read as a FIXED
   step, not luck.
2. First cmd/tq background run piped to `tail -15` and lost everything but
   the FAIL tail; the re-run with a log file was the pattern to start with.
3. Root build+vet never ran (f3 said "root + per-module"; I covered the
   touched modules + cmd/tq only). Comment/test-only changes make root
   breakage near-impossible, but f3's letter is unmet.

**Can still improve:**

1. A "pin" test convention: every pin either demonstrates its failure once
   (scratch mutation, output recorded) or carries a self-drift fixture that
   asserts the zero-decode behavior directly.
2. Execution windows should annotate the report they execute (§f items →
   inline SHIPPED notes) in the SAME step as the TODO harvest, so the
   point-in-time report never reads as open work two hours later.
3. GOCACHE=/tmp/go-build-cache should be part of my default go invocation,
   not something adopted mid-session.

## a) FULLY DONE

1. Self-repair: blank-line table break in docs/status/README.md fixed (my
   own artifact); `scripts/check-status-index.sh` → `status index ok`; row
   contiguous with the table.
2. The 12-27 report's core finding SHIPPED:
   internal/readmodel/detailkeys_test.go — four tests / five cases:
   repriDetail↔queue.ReprioritizeEvidence, requeueDetail↔queue.RequeueEvidence
   (full + legacy-absent-class), enqueueDetail↔queue.EnqueueDetail, plus
   repriEvent's decode policy (malformed detail → error surfacing journal
   drift; valid → priority 7 folded). In-package (the unexported structs
   are unreachable from parity_test.go's external package). Precedent cited
   in the file comment: budget_test's marshal-the-real-type pin
   (internal/budget/budget.go:189-193).
3. readmodel module gate green: gofmt clean, build+vet clean, full suite
   `ok 0.157s -count=1` (final state, after all edits).
4. f17/f18 doc fixes: deliberate-shell rationale added to the status Sweeper
   (internal/status/sweep.go:83-91, parity with dlqfix/review siblings);
   TaskFilter doc pointer to TaskList's MUST-NOT-dispatch warning
   (internal/readmodel/readmodel.go:63-68). status module BUILD+VET OK.
5. LSP noise verified false positive via CLI gates (f10/f11): cqrs
   GOWORK=off build+vet OK; cmd/tq gate `rc=0, ok 15.605s` — all 58 gopls
   "undefined" errors on cmd/tq/main.go and the cqrs go.mod errors are the
   documented stale-gopls class. Nothing to fix.
6. The cmd/tq gate red incident root-caused: TestAgentsDocSizeGuard failed
   measuring AGENTS.md at 17,334 B (budget 15,700) at 12:40:46; the file was
   15,694 B (under budget) at my wc minutes later; daemon commits at
   13:15/13:17 touched it; gate re-run GREEN. Transient concurrent-window
   mid-edit state — not my change (I never touched AGENTS.md), not fixed by
   me, nothing to do.
7. f26/f25: "Scan failure" term added to docs/DOMAIN_LANGUAGE.md (Harvest
   loop table, beside Drift; names the deliberate per-sweeper vocabulary of
   harvest.ScanFailure/depsweep.Skip); budget-facade idea added to
   ROADMAP.md Raw ideas (group-5 disposition).
8. f29 closed by inspection: exactly three json.Unmarshal sites in
   internal/readmodel (events.go:213/233/247), all three now pinned.
9. HARVEST (f2): 7-row dedup-checked section appended to TODO_LIST.md
   (webui provenance twin, accepted-groups ledger, dupe-baseline gate,
   mirror-clones coverage, budgetUsageView golden pin, metaengine citation,
   advisory-lint pass) — `check-todo-list.sh` ok; index-sweep NOT re-rowed
   (rows 117/415 own it); row 488 confirmed distinct (rendering enrichment,
   not the builder twin).

## b) PARTIALLY DONE

1. f3 tree re-baseline: touched-module gates (readmodel, status, cqrs) +
   cmd/tq gate green; the ROOT build+vet and the full per-module loop NOT
   run. Remaining effort: S (one loop invocation with GOCACHE set).
2. The 12-27 report's shipped §f items are dispositioned in the TODO
   section header but the report file itself is un-annotated — snapshot
   semantics applied; a raw HARVEST reading its §f could still mint
   duplicate tasks before hitting the header's dedup note. Mitigated, not
   eliminated (Effort: S, item 4 below).
3. g1 family-policy ruling: executed the RECOMMENDED path (keep local wire
   structs + pins), but the ruling itself stays open; the direct-reuse
   alternative remains a clean swap (delete three structs, repoint three
   decode sites).

## c) NOT STARTED

1. The seven harvested TODO rows (webui provenance twin, accepted-groups
   ledger, dupe-baseline gate, mirror-clones coverage, budgetUsageView
   golden pin, metaengine citation, advisory-lint pass) — by design, they
   are the pool's work now.
2. f27 lessons.md (crush-config repo, cross-project commit) — skipped with
   rationale now recorded HERE (should have been recorded in-session).
3. f24 AGENTS.md memory line — superseded by the f9 ledger row (harvested).
4. Root build+vet (f3 remainder).
5. CHANGELOG bullet for the parity pins (repo convention: shipped work
   lives in CHANGELOG; nothing appended this session).

## d) TOTALLY FUCKED UP

Nothing shipped broken — every gate that ran is green and the tree is
clean. The honest fucks are mine, process-level:

1. I broke the status index table with my own edit (blank line between
   rows) minutes after lecturing about durability. Severity: docs-only,
   minutes, self-caught. Root cause: edit construction without an immediate
   post-edit neighborhood check.
2. The parity pin's failure mode is ARGUED, not DEMONSTRATED. Severity:
   latent — worst case the pin is a green scarecrow that never fires on the
   exact drift it exists for. Root cause: forward progress prioritized over
   self-verification. Fix: f2 below (S).
3. First go invocation ignored the GOCACHE=/tmp/go-build-cache doctrine
   (AGENTS ENOSPC hazard). Severity: none materialized; process debt.
4. TestAgentsDocSizeGuard gave a failure with NO provenance (who grew it,
   when) — today that cost a root-cause investigation across two windows
   for a transient. The guard's message is honest but blind. (Fix is f9
   below; listing here because today's confusion was real.)

## e) WHAT WE SHOULD IMPROVE

1. Pin tests must demonstrate their failure mode once. Impact: converts
   "trust me" pins into verified rails; cheap (one scratch mutation).
2. Fixed post-edit check for table/index edits: re-grep the neighborhood
   immediately, every time. Impact: kills my d1 class.
3. Execution windows annotate the report they execute (inline SHIPPED
   notes) in the same step as the TODO harvest. Impact: reports stop
   reading as open work; HARVEST never re-mints.
4. GOCACHE=/tmp/go-build-cache unconditional from the first invocation.
   Impact: removes an ENOSPC lottery.
5. Background gates log to a file from the start; tail the file, never the
   pipe. Impact: no lost output, no re-runs.
6. Guard failures should carry provenance hints (the size guard could cite
   `git log -1 --format=%as -- AGENTS.md`) so transient-concurrent states
   triage in one glance. Impact: today's incident cost two windows a
   cross-check.

## f) Top things to get done next (session-grounded, deduped against the 7 rows already in TODO_LIST — those are NOT re-listed; cut before padding toward 50 because the rest of the honest surface is already harvested)

1. Mutation-verify detailkeys_test.go once: scratch-rename a queue tag,
   watch the pin FAIL, record the demonstration in the test comment (or add
   a renamed-key fixture asserting the zero-decode). **Critical / S / Testing**
2. Finish f3: root build+vet + the full per-module gate loop at HEAD
   (GOCACHE=/tmp/go-build-cache, GOEXPERIMENT=jsonv2). **High / S / Quality**
3. Full ci-local certification at current HEAD — the tree moved across
   ≥4 windows today (size-guard churn, art-dupl sweep, my pins); nobody has
   a green full-run claim at THIS head. **High / M / Quality**
4. Annotate the 12-27 report §f inline (SHIPPED <ref> on items 1-5, 17, 18,
   26, 28, 29; disposition notes on 23/24/27) per docs-health ANNOTATE
   semantics. **Medium / S / Documentation**
5. Cross-read the concurrent art-dupl-t4-zero-dedup-sweep report before
   either window lands the accepted-groups ledger — one dedupe pass, one
   ledger, not two. **Medium / S / Process**
6. Size-guard provenance hint: append `git log -1 --format=%as -- AGENTS.md`
   output to TestAgentsDocSizeGuard's failure message (cmd/tq/
   facts_json_test.go:53) so transient concurrent-growth states triage in
   one glance. **Medium / S / Tooling**
7. Table-continuity check in check-status-index.sh: fail on an empty line
   between `| 20` rows (today's d1 class is invisible to the presence-only
   check). **Medium / S / Tooling**
8. Post-fold verification for THIS report: check-status-index.sh after the
   daemon folds it; amend-maneuver if row and file land in separate
   commits. **Medium / S / Process**
9. CHANGELOG [Unreleased] bullet for the detail-key parity pins +
   repriEvent policy pin (or an explicit fold-into-next-entry decision).
   **Medium / S / Documentation**
10. g1 follow-through: if the owner rules direct-reuse, delete the three
    local detail structs, repoint the three decode sites at the queue
    types, keep repriEvent's fail-on-malformed policy, and convert the pins
    to round-trip tests. **Medium / M / Quality (blocked: g1 ruling)**
11. g2/g3 disposition: file the outcomes of the dupe-baseline-gate and
    budget-facade questions into the rows that carry them (TODO_LIST 12-27
    section; ROADMAP raw idea). **Medium / S / Process (blocked: rulings)**
12. lessons.md candidates for crush-config references/lessons.md (by commit
    in THAT repo): "pins demonstrate their failure"; "review findings need
    same-session durable artifacts". **Low / S / Process**
13. If more dupe runs are planned: after the ledger (row 2) lands, re-run
    branching-flow dupe and expect accepted-groups-only output — the
    session's original tool run should end the day quietly. **Low / S /
    Tooling**

13 items — the honest residue; everything else this session surfaced is
either shipped (a), formally harvested (the 7 TODO rows), or an owner
ruling (g). 50 slots would be padding at this point.

## g) Questions (cannot answer from code; blocking nothing but shaping the next windows)

1. AGENTS.md endgame: the budget has been reset three times (now 15,700)
   and still transiently overflowed today under concurrent windows. Is the
   intended steady state periodic prune+reset forever, or a planned split
   (AGENTS keeps pointers, sections move to docs/) at the next reset? (The
   reset protocol is pinned — c974d66b — but the split-vs-prune preference
   is yours; it decides whether I ever propose a split.)
2. Reporting ritual for execution-only windows: this session produced TWO
   point-in-time reports (12-27 verdicts, 14-09 execution) and the first
   was half-stale within two hours. Should execution windows keep minting
   their own docs/status report, or fold into the next closeout/report and
   only leave the TODO harvest + annotate trail? (Decides whether §f of
   verdict reports should even be written as work lists.)
3. HARVEST dedup signal: is the TODO section-header disposition note
   ("SHIPPED this window / X is row N's territory") the canonical dedup
   surface docs-health HARVEST reads, or must the SOURCE report's §f items
   also be annotated/struck for a harvest pass to skip them? (You run
   HARVEST; the mechanics of what it actually greps are yours to rule.)

THEN WAIT FOR INSTRUCTIONS.
