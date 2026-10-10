# Status: A1 Alias Flip Shipped; cmd/tq Golden RED on Wire-Pin Fix

**Date:** 2026-10-10
**Window:** 2026-10-10 ~02:00 → 02:10 (resumed the B-track-done session; A-track execution)
**Plan:** `docs/planning/2026-10-09_20-23_CONFIG-SYSTEM-ALL-IN-BRIDGE.md` (Full Execution Mode)

## a) FULLY DONE

- **Todo list recreated** exactly per handoff (B1-D1-B2-B3-B4 completed; A1 in_progress at start).
- **Pinned-vendor verification before A1 code:** re-read `vendor/…/queue/v4/facts/facts.go` (v4.0.3 pin, NOT the ahead local checkout): 11 lifecycle constants byte-identical to tq's (`task.enqueued` … `task.reprioritized`); `facts.Fact` fields/JSON-tags identical except `Detail []byte` vs tq's `jsontext.Value`. Companion `UpstreamFact`/`JournalFacts` mappers confirmed as the conversion seam.
- **A1 code shipped:** `internal/journal/journal.go` rewritten — `type FactType = facts.FactType`, `type Fact = facts.Fact` (module now requires `go-cqrs-lite/queue/v4 v4.0.3`; go.sum created); local Fact struct + jsontext import deleted; tq-only families (Heartbeat, SessionOpened/Closed, QuestionAsked/Answered, ErrorObserved, IncidentTaskMinted) kept as constants of the aliased type; S2 package-doc paragraph rewritten (the "stays go-cqrs-lite-free" claim is superseded).
- **Conscious deviation, recorded:** the handoff said DELETE the 11 lifecycle constants; executing that literally broke ~46 files in 15 modules (own grep inventory) and contradicted the plan file's A1 gate ("compile-only change, no behavior", 5 files, 90m). Instead the constants are now TRUE identity re-exports (`const Enqueued = facts.Enqueued` …) — one spelling by construction, consumers keep the journal import surface, A5 collapses to verification. Deviation is in commit 91135c1e's message and here.
- **A1 gates GREEN:** journal module gate (build+vet+test -count=1+gofmt clean) with `GOWORK=off`; root `go mod vendor` RC=0 (queue/v4 already vendored, modules.txt unchanged).
- **A1 committed:** `8fa8a58d` (daemon chore: alias flip + go.mod/go.sum + memory_journal_test Enqueued→Heartbeat) + `91135c1e` (named: lifecycle identity re-exports). Tree was clean post-commit.
- **A5-pre sweep DONE (full inventory):** every non-test jsontext user and every Fact marshal/unmarshal site grepped repo-wide. SAFE classes: ~15 `json.Unmarshal(fact.Detail, &struct)` sites (raw-bytes input, unaffected); webui renders Facts as templ HTML (JSON writes are stats-only); task/executor/depsweep/httpapi jsontext use is task-PAYLOAD, not Fact. ONE real wire-format risk found: `tq facts --json` (cmd/tq/main.go:3029) marshals `[]journal.Fact` — Detail would silently become base64.
- **Wire-format pin caught before it lied:** `TestFactsJSONGolden` would have PASSED despite the change (base64 round-trips through its `[]journal.Fact` unmarshal) — the pin was too coarse. Fixed the presentation layer: `factJSONView` + `factJSONViews()` in cmd/tq/main.go (same fields/tags, `Detail jsontext.Value`), `--json` branch now encodes views. Gate-log output confirms embedded JSON (`"detail": {"project": …}`) — pre-A1 shape byte-compatible. Committed by daemon as `d5f590fb`.

## b) PARTIALLY DONE

- **cmd/tq golden fix (the A5-pre fix's last leg):** main.go is fixed and committed; `facts_json_test.go:184` still unmarshals the output into `[]journal.Fact`, which can no longer decode an object Detail → `TestFactsJSONGolden` FAILS ("cannot unmarshal JSON object into Go []uint8 within /0/detail", cmd/tq gate RC=1). One-line fix identified: `var facts []factJSONView` (same package). ALSO worth adding while there: assert the detail decodes as a JSON object, so the pin actually guards the wire shape. NOT yet applied; gate RED; nothing to commit beyond what the daemon already took.

## c) NOT STARTED

- A2 companion re-point (mappers may now collapse to identity); A3 memory-journal vocabulary check; A4 journal/cqrs adapter + its `--cqrs` detail wire check; A5 formal per-package verification pass; A6 S2 battery (all module gates + root build/vet + `-race` + smokes + ADR-0019 addendum); A7 tag wave (NO push pending owner); C1→C5; E1→E3; F3/F1/F2/F4; fold B4 serve smoke into `scripts/smoke/`; CHANGELOG for B1-B4+A1; plan-file DONE rows with citations; TODO row updates.

## d) TOTALLY FUCKED UP

1. **Pipe ate a gate rc and I initially believed green:** ran `./scripts/test-cmd-tq.sh 2>&1 | tail -8; echo RC=$?` — `$?` was tail's rc (0) while the suite had FAILED (PIPESTATUS unavailable, a documented session hazard I walked straight into anyway). Caught it on the disciplined re-run with output-to-file (`RC=1`, battery rule "rc TO A FILE"). Lesson enforced: rc to file, never through a pipe.
2. **Daemon won again:** the alias-flip rewrite + go.mod/go.sum + test fix landed in footer-less chore `8fa8a58d`; my named commit `91135c1e` carried only the re-export increment; the main.go wire-fix went into `d5f590fb`. The "commit immediately after each edit" rule was followed per-edit but the gate-then-commit cycle still exceeded the 60s sweep window mid-edit-batch.
3. **Executed the handoff letter before re-reading the plan row:** I wrote journal.go per the handoff's "DELETE the tq constants" without first re-reading the plan file's own A1 gate ("compile-only"). The 46-file inventory caught the contradiction before damage, and the resolution (identity re-exports) is strictly better, but the correct order was: read plan row → verify gate → then code.

## e) IMPROVE

- Always read the plan row's GATE column before writing code for a task, not the summary's step list.
- Strengthen coarse pins when they survive a semantic change silently (golden test should assert detail-as-object).
- One logical change per commit window, `git add` BEFORE running any gate.
- Record sweep inventories in the report (done here) so the next session doesn't re-grep.

## f) NEXT (priority order)

1. Fix `facts_json_test.go:184` → `[]factJSONView`; add detail-as-object assertion to the pin.
2. Re-run cmd/tq gate to GREEN (rc to file); amend/no further daemon risk by committing test+any main.go leftovers immediately.
3. A2: companion `JournalFacts`/`UpstreamFact` → identity collapses (aliases make conversions free) + `scanFacts` re-point + facts_bridge test still green; module gate; commit.
4. A3: memory journal vocabulary verification (already alias-compatible by construction; confirm tests hermetic); gate; commit.
5. A4: journal/cqrs adapter re-point + check `--cqrs` event Detail wire (facts_cqrs_test); gate; commit.
6. A5: formal verification pass over the 15-package inventory (build+test each module; expect zero diffs thanks to re-exports).
7. A6: S2 battery — all module gates, root build/vet, `go test ./... -race`, smokes.
8. A6b: ADR-0019 addendum paragraph (S2 now type-alias-enforced).
9. A7: tag wave + `check-facade-parity.sh` + vendor sync (NO push — owner Q2).
10. C-track C1→C5 (C3 ordering after B2 — satisfied).
11. E-track E1→E3 (E2 after C4+E1).
12. F3, F1, F2, F4 (F4 needs owner budget answer — prior Q3).
13. Fold B4 serve smoke into `scripts/smoke/`.
14. CHANGELOG entry covering B1-B4 + A1.
15. Mark plan-file B-track rows DONE with gate citations; add A1 row citation.
16. Re-ask owner Q1/Q2; surface new Q3 (wire stance below).
17. `check-gomod-vendor-sync.sh` after any remaining go.mod churn.
18. Re-run conform suites (companion vocabulary pin now trivially true — consider keeping as belt-and-braces, note in report).
19. Verify `readmodel/host.go:152` view Detail jsontext marshal path unaffected (listed safe; spot-check its writer).
20. Confirm `examples/api` Payload jsontext is fact-unaffected (payload domain, expect fine).
21. gofmt scoped check on touched trees (`executor.ScopedGofmtStage` policy).
22. lint-baseline delta check (`--check`) for new gosec/revive findings from the alias flip.
23. TODO_LIST: annotate A-track progress rows.
24. AGENTS.md: fold the "journal vocabulary rides upstream via aliases" fact into the architecture table bullet (respecting F4 budget decision).
25. Heal/verify no daemon-sweep footers missing on `scripts/heal-daemon-sweep.sh` list if attribution matters (optional; documented instead).
26. Persist the 46-file consumer inventory to the plan file appendix (currently only in this report).
27. Double-check `tq facts --detail` human path + `--commits` (non-JSON) unaffected.
28. Spot-run `internal/queue/sqlitev4` migration replay smoke (journal round-trip through aliased Fact).
29. Verify `internal/queue/companion/conform` vocabulary test still compiles/passes (it pins identity — now via aliases).
30. Consider de-duplicating factJSONView with readmodel's host view if shape-identical (judgment call, art-dupl policy).
31-50. Reserved: C/E/F sub-steps expand here as their rows are pulled (see plan file rows 10-23).

## g) QUESTIONS FOR THE OWNER (max 3)

1. **Q1 re-ask (still open):** accept the B2 gate rewording ("exactly ONE tq-owned constructor call + ONE pragma source; system-declared connection runs the identical union until the upstream seam lands") vs literal one-constructor-call, or block final B2 on upstream instance injection?
2. **Q2 re-ask (still open):** push the A7 S2 tag wave to origin when reached, or keep local for review? (No push without explicit yes.)
3. **NEW Q3 — `tq facts --json` wire stance:** I preserved the pre-alias wire format (Detail as embedded JSON) via a command-local view, since the alias flip would otherwise silently switch Detail to base64 and the golden pin couldn't tell. Keep embedded-JSON as the permanent contract (pin it harder), or is base64/detail-as-string acceptable post-bridge (then I drop the view and update the pin honestly)?
