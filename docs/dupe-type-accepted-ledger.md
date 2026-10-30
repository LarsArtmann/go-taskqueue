# Accepted Type-Duplicate Groups — Ledger

Standing record of duplicate-Go-type groups that a duplicate review has
ACCEPTED with rationale, so the next `branching-flow dupe .` run never
re-litigates them. Established after the 2026-10-05 review
(docs/status/2026-10-05_12-27_dupe-type-review-verdicts.md: 11 groups read at
source, 10 accepted, 1 real finding fixed).

How to use: when a dupe run surfaces a group below, cite the row and move on.
A group NOT in this ledger and not machine-classified `intentional` /
`false_positive` by the tool is a fresh finding — review it at source.

Re-run expectation (verified 2026-10-30, `branching-flow dupe .`): 14 group
keys total. Ten `actionable` keys are exactly ledger rows 1-9 and 11 below
(row 10 is the machine-classified bucket); the remaining four keys come back
machine-classified `intentional` or `false_positive` (factPayload/foldPayload
DTO pair; single-field executor structs; empty context-key/marker structs;
distinct `Cause: error` sentinel types). Any OTHER actionable key = new
finding.

## The eleven accepted groups

1. **evt\* fold structs** — `internal/readmodel/events.go`. Required:
   go-cqrs-lite metaengine's OnRecordTyped dispatches on distinct Go types,
   so each journal fact needs its own struct (see the family header comment
   at events.go:13-15).
2. **Sweeper shells** — `internal/{dlqfix,review,status}/sweep.go`. The
   Sweeper struct shell deliberately stays per-sweeper; the shared
   mechanics (watermark cursor pump) ride `watermark.Cursor.Sweep`.
   Rationale recorded beside each shell's doc comment.
3. **FactSource twins** — `internal/journal/cqrs/journal.go` vs
   `internal/session/session.go`. Idiomatic consumer-defined interfaces;
   each package declares exactly the surface it consumes.
4. **ProvenanceEvent vs repriEventView** — `internal/harvest/provenance.go`
   vs `internal/webui/render.go`. Render-projection policy (documented in
   harvest): the web view is a projection; BOTH consumers build through the
   single shared `harvest.BuildProvenance` — no statement-level twin.
5. **StatusCompletion vs statusWindowView** — `internal/executor/status.go`
   vs `cmd/tq/main.go`. Differs only in wire struct tags; cmd/tq owns its
   JSON presentation types (ADR-0016/ADR-0017 boundary).
6. **SessionUsage vs budgetUsageView** — `internal/budget/budget.go` vs
   `cmd/tq/main.go`. cmd/tq cannot import internal/budget (facade rules,
   ADR-0016; replace-free cmd/tq, ADR-0017), so the view is forced.
   Wire keys pinned by the cmd/tq golden-JSON test.
7. **ReprioritizeEvidence vs repriDetail** — `internal/queue/queue.go` vs
   `internal/readmodel/events.go`. Accepted as part of the deliberate
   local-wire-struct family: readmodel's decode fails on malformed detail to
   surface journal drift, unlike queue's skip-don't-fail parser. Field/key
   parity PINNED by `internal/readmodel/detailkeys_test.go` (the 2026-10-05
   review's one real finding, shipped).
8. **TaskFilter vs TaskList** — `internal/readmodel/readmodel.go` vs
   `internal/readmodel/queries.go`. Documented role split: TaskFilter is the
   write-side filter, TaskList the query shape with a MUST-NOT-dispatch
   warning (queries.go:23-26).
9. **ScanFailure vs Skip** — `internal/harvest/drift.go` vs
   `internal/depsweep/sweep.go`. Two-field `{Repo, Reason}` pairs across
   modules; a shared dependency would cost more than the duplication.
   Per-sweeper vocabulary, deliberately unshared.
10. **Tool false-positive classes** — single-field executor wrapper structs
    (`*AgentExecutor`), empty context-key/marker structs, distinct
    `Cause: error` error types. The tool now self-classifies these
    `false_positive`; nothing to consolidate.
11. **FailureEvidence vs failureEvidenceView** — `internal/executor/result.go`
    vs `cmd/tq/evidence.go`. Differs only in wire struct tags (the view
    renders `stage,omitempty`; the domain fact pins `stage` because Stage is
    the taxonomic key). cmd/tq owns its JSON presentation types
    (ADR-0016/ADR-0017 boundary, same accepted class as rows 5-6); show wire
    keys pinned by `TestShowJSONCarriesFailureEvidence`. The matcher
    knowledge itself is already shared
    (`executor.MatchesEvidenceStageSet`, 2026-10-26 01-40 §b1).

## Policy

- New dupe runs: ledger-accepted groups and machine `intentional` /
  `false_positive` verdicts are noise; anything else is a finding.
- Growth beyond this set is gated by `scripts/dupe-baseline.sh --check`
  (wired into ci-local): a NEW actionable GroupKey fails the gate; accepted
  groups are recorded in `.dupe-baseline.txt` (regen after a fresh review).
