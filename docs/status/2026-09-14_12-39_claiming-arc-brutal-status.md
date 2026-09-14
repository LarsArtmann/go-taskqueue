# Claiming-arc status — extraction kept honest, queue/ still gated

**Session:** 2026-09-14, ~08:30–12:40 CEST (interactive; spans go-taskqueue +
go-cqrs-lite). Continuation of the 2026-09-13 cqrs-storage-verdict arc
(08-01 report). Format: `.md` per explicit owner instruction (status-report
skill default is HTML — override flagged per skill contract).

**One-paragraph reality:** the owner opened by demanding the whole todo
list get done. I started executing P0 my way — designing a speculative
`claiming/` Spec-DSL module — got interrupted twice ("back to the basics",
"did you research to the MAX?"), did the max sweep, discovered the repo
had far more queue machinery than anyone had synthesized AND that a
continuation session had already executed the (stale) P0 row into landed,
wired work. The owner then ruled the extraction "may actually be a good
idea" IF faithful; I verified, trimmed the one speculative knob, and
closed every ruling this side of the queue/ green-light.

## a) FULLY DONE (verified by a gate, not a claim)

| # | Item | Evidence |
|---|------|----------|
| 1 | **`claiming/` made an honest extraction**: removed the one field with no counterpart in the original sqlstore code (`Spec.And` + `andSuffix` + 3 stmt call sites + test assertions + sqlstore `timersSpec` literal). Module is now purely extracted claim SQL + minimal parameterization (Table/ID/Due/Lease/Returning/OrderBy). | claiming build+vet+test ok 0.091s (byte-exact PG/SQLite/MySQL/renew pins, spec-knob test, SQLite round-trip incl. lease-expiry reclaim); `rg '\bAnd\b|andSuffix'` over claiming/ + sqlstore/ = zero hits |
| 2 | **scheduling/sqlstore re-verified after trim** (delegation unchanged, `And` literal removed after build catch) | full module suite ok 25.101s (incl. the continuation session's race-stress + counter-scope pins) |
| 3 | **api-stability golden regenerated** for the API change (Spec.And removal) + TestEvery | "Updated docs/api_surface.txt (6856 exports)"; TestEvery ok |
| 4 | **doc-check green** incl. the pre-existing claiming row in references/modules.md (the continuation session's flagged loose end turned out already closed, line 66) | "All 1132 references valid across 49 packages", exit 0 |
| 5 | **§g1 answered with first-hand evidence** (the fold-depth question): metaengine `MapUpdate` is per-key RMW under single-row `FOR UPDATE` (metaengine/engine.go:278, pgengine/map_update.go:81); planned tables single-collection; ADR-0117 lifecycle = tq's facts model. The AGENTS.md verdict note's claims held verbatim. | source-cited in session; 08-01 report §g addendum written |
| 6 | **§g2+§g3 ruled and recorded**: journal-drift audit minted as agent-executable TODO_LIST work (High Impact row: hermetic fixtures ONLY, production journal stays operator-invoked) with the end-state ruling baked in (read-only operator command; ci-local smoke advisory at most — never a hard gate on task state) | `scripts/check-todo-list.sh` exit 0; 08-01 report addendum |
| 7 | **tq AGENTS.md verdict note updated**: PROPOSED → IN PROGRESS upstream; P0 shipped as `claiming/` (tq consumes nothing yet); re-open bar unchanged (conformance-suite parity) | `scripts/check-doc-refs.sh` exit 0 after edit |
| 8 | **go-idempotency usage verdict** (owner question): used by 6 in-repo modules (idempotency/{sqlstore,kvstore}, commandlifecycle, middleware, integration, example/taskmanager) + ≥6 sibling repos (DiscordSync, Standup-Killer, cqrs-htmx, crush-daily, go-localsync, github-local-sync); latest tag v0.3.0; **not** in go.work `use` (AGENTS.md "sibling checkouts" claim is stale for it) | rg over go.mod/imports + tag list |
| 9 | **Max-sweep inventory** (the "to the MAX" challenge): projectionhost (worker loop: backoff/drain/restart-budget/DLQ+replay), TWO deliberately-split DLQs (ADR-0043), middleware retry (go-retry — same lib as tq), ADR-0134 claim tokens (Proposed), commandlifecycle, watermill Nack-redelivery, taskmanager flagship, deriver sagas — the queue gap narrowed to ONE missing piece: the claimable task store | all file:line-cited in session transcript |
| 10 | **Alternatives analysis after the "basics" challenge** (A core-first / B queue-first / C parameterize-sqlstore / D port-tq / E nothing; recommended B — consumer-shaped SQL, extraction demoted to rule-of-three) | presented; owner's subsequent ruling superseded the sequencing question for P0 |

## b) PARTIALLY DONE

1. **`claiming/` end-state**: trimmed + locally gated, but NOT through the
   full `nix run .#verify-ci` matrix, and NOT tagged — it rides an
   unpublished sibling replace (`claiming/v4 v4.0.0` + `=> ../../claiming`
   in sqlstore + example/scheduler-otel-status). Finish = tag wave (owner
   release process). Effort S once ruled.
2. **cqrs proposal doc** (`docs/planning/2026-09-13_durable-work-queue-module.md`):
   the TODO_LIST row was updated to "P0 done" by the continuation session,
   but the PROPOSAL doc's own phasing section still reads P0 as proposed —
   I only noticed while writing this report (forgot during the trim).
   One-paragraph fix pending. Effort XS.
3. **cqrs AGENTS.md go.work drift** (use-block claim vs reality for
   go-idempotency): found, reported, not fixed. Effort XS.
4. **Rejection-propagation process rule**: learned (see d1) but not yet
   encoded in any AGENTS.md. Effort XS once owner blesses wording.

## c) NOT STARTED

1. **queue/ module (P1+)** — fully designed (three layers: lean contract /
   SQL claim engines on claiming / metaengine read-adapter later; ADR-0134
   tokens day one; dedup via go-idempotency seam; consumer side composes
   projectionhost+middleware). Blocked ONLY on owner green-light.
2. **Journal-drift audit feature** — row is live pool food; zero code.
3. **ADR-0134 implementation** anywhere (still Proposed upstream).
4. **metaengine read-side adapter for queue** — later phase by design.
5. **08-01 report §f remainder** (items 6, 7, 10–14: facts-emission
   machine check, postgres facts_archive parity, mechanical-checklist
   script, …) — untouched this session by design; pool/human food.

## d) TOTALLY FUCKED UP!

1. **Design-before-research inversion, twice over.** I wrote a whole
   speculative `claiming/` module (Spec DSL incl. an `And` knob, guessed
   shape for a hypothetical consumer) BEFORE sweeping the host repo. The
   owner's "bad idea" was correct. Worse: after the rejection I deleted
   the files but did NOT kill the stale TODO row — the continuation
   session (05-25) executed the rejected work into landed, wired,
   gate-swept infrastructure. My rejection was silently overridden by the
   machine. The final state (keep as faithful extraction + trim) is good,
   but it took TWO owner interventions to get here; the process failed
   twice around a correct instinct.
2. **First test file quality**: hand-rolled `contains`/`indexOf` instead
   of `strings.Contains`, and a round-trip test with a lease-equality bug
   (claim at now == lease_until re-opens the row — my own step choices
   would have flaked). Both caught pre-run during rewrite, but both were
   written.
3. **Trim blast-radius miss**: removed `Spec.And` grepping only claiming/
   → sqlstore build broke on `timersSpec`'s `And: ""` literal. The build
   caught it instantly (delete ⇒ build first, the known doctrine — I DID
   build immediately, which is why this is a footmark, not a wound).
4. **`rg -rn` flag slip**: `-r` is rg's REPLACE flag; output lines showed
   mangled `"github.com/n"` evidence. Caught by understanding the output,
   not by a broken conclusion — but the slip happened.
5. **Pipeline exit masking** (`cmd | tail; echo $?` reports tail) —
   committed early in-session, the exact documented sin; corrected to
   file-redirect + explicit `$?` for every later gate.
6. **One stale-read edit failure** on spec.go (continuation session had
   touched it) — recovered by re-view + re-apply; should have re-viewed
   first under known concurrency.

## e) WHAT WE SHOULD IMPROVE!

1. **Foreign-repo pre-design ritual**: module map + ADR index + planning
   dir + TODO_LIST sweep BEFORE designing anything in a repo I don't own
   day-to-day. Twice-burned now (CONTRIBUTING last session, max-sweep this
   one). The owner should not have to ask "did you research to the MAX?".
2. **Rejection propagation rule**: an owner-rejected direction must kill
   its TODO row / proposal phasing in the SAME session — the pool executes
   stale rows within hours. Encode in tq AGENTS.md (dogfood-loop lesson;
   this incident is the evidence).
3. **Grep scope = blast radius**: removing an API symbol ⇒ repo-wide grep,
   not my directory (d3).
4. **No stdlib reinvention in tests; derive equality edges from the
   predicate (`<=`) before writing steps** (d2).
5. **Standing issues I only reported**: cqrs repo-wide lint red
   (gci-vs-treefmt split-brain, continuation session escalated — owner
   ruling pending) and tq status-index bloat (157 rows > 100 threshold).
   Both predate me; both still open.

## f) Next things (session-scoped; padding to 50 rejected)

1. Owner ruling: **queue/ P1 green-light** (thin shape, §c1) — everything
   else in the queue track queues behind this.
2. **Tag wave**: cut `claiming` v4.0.0 + bump dependent pins (sqlstore,
   example) + replace-strip sweep (release process, owner-run).
3. Fix the **proposal doc P0 phasing** (b2; XS).
4. Fix **cqrs AGENTS.md go.work use-block drift** (b3; XS).
5. Encode the **rejection-propagation rule** in tq AGENTS.md (e2; XS).
6. **gci-vs-treefmt lint ruling** (e5; escalated, owner).
7. **status-index archive sweep** (157→<100; docs-health ANNOTATE; S).
8. **Drift-audit build** (live pool food — the row carries scope+tests).
9. queue/ P1 first artifact: **conformance suite** mirroring tq ADR-0007.
10. queue/ **contract slimming** decision: which of tq's 25+ Store methods
    form the lean core (design note, S).
11. Confirm **ADR-0134 tokens day one** as queue/ design input (vs
    tq's owner-string).
12. **go-idempotency seam** for dedup'd enqueue in queue/ (design note).
13. **projectionhost-as-queue-consumer** evaluation note (P1 input).
14. Run cqrs **`#verify-ci` full matrix** over the trim (pre-tag
    insurance; S).
15. 08-01 §f6: **facts-emission machine check** (shared conformance
    invariant; pairs with queue/ conformance).
16. 08-01 §f7: **postgres facts_archive parity** verification vs
    sqlite.go:793 (S).
17. 08-01 §f10: **mechanical checklist script** (turn-1 ls/stash/grep
    ritual; this session added d4/d5 data points).
18. Annotate 08-01 §f1–3 as superseded-by-addendum during the next
    docs-health pass.
19. cqrs CHANGELOG: verified the Unreleased claiming entry cites no
    removed symbol (no action) — re-check only if the entry is edited
    again before tagging.
20. ROADMAP fuel: **PapDashboard as second queue consumer** evaluation
    once P1 lands.

## g) Questions I cannot answer myself

1. **queue/ P1 sequencing**: green-light NOW (queue-first, thin shape),
   or after the claiming tag wave settles? Building first means the
   conformance suite and the tag ride the same release train.
2. **The knob line**: I kept `Spec.OrderBy` (parameterizes the ORDER BY
   the original SQL already had) and trimmed `Spec.And` (no original
   counterpart). Right line — or zero knobs until queue/ proves each one?
3. **Rejection-override policy**: when owner-rejected work has already
   landed via the pool (this incident), default = keep-landed-and-re-rule
   (what happened) or auto-revert on propagation? This is a dogfood-loop
   policy only you can set.

— Recorded 2026-09-14 12:39 CEST. Waiting for instructions.
