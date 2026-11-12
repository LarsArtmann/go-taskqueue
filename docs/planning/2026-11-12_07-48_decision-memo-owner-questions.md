# Decision memo — the owner questions from the 2026-11-12 window

**Date:** 2026-11-12 (repair-verify-deepen window, pareto plan
`2026-11-12_04-29`; the four §g questions of
`docs/status/2026-11-12_06-45_repair-verify-deepen-window-report.md`)

Status at writing: master ~40 commits ahead of origin, tree clean, every
gate green at HEAD unless noted. Each question carries a recommendation
and, where the owner's blanket execute-directive already covered it, what
was done.

## 1. Push ritual (26 → ~40 commits ahead)

**Recommendation: push is owner ritual — STAYS unpushed.** The window
report's question stands; nothing in this execution window changes the
calculus, and the local/origin divergence row (TODO_LIST, 2026-10-28 §g2)
already names the reconcile decision as owner-owned. Concretely at stake
on push: cmd/tq's `GOWORK=off` graph loads heal (the tagged-but-unpublished
window closes, VERSION-SURFACES row unblocks), CI runs on the repaired
tree, and the erraudit devwork-shim symptom class dies. Cost of waiting:
every proxy-shaped graph load stays broken by design. **Owner action: a
plain `git push` when ready; no force, no rebase needed (local line is a
strict superset of origin/master at 1bc6f499).**

## 2. `TestDoctorHealthyEmptyDB` shape (fails every cmd/tq run)

**Recommendation: exempt the tag-ancestry check from this test's
worst-status aggregation** (option B of the TODO row), not a fixture repo.
Reasons: (a) the failure is an orphaned-lineage WARN (v0.4.2 unreachable
from the test's empty fixture DB) — the check is CORRECT, the fixture is
just not a real deployment; (b) a fixture repo would need git history in
cmd/tq's testdata, heavier and still synthetic; (c) the exclusion is one
map/toggle edit in the doctor's worst-status walk with a comment citing
this memo. **Not executed here: cmd/tq is the one module whose gates run
only via `scripts/test-cmd-tq.sh` in this window's battery and the row is
owner-gated by an earlier ruling — the edit is 15 minutes once you
acknowledge this memo.**

## 3. server_timing disposition (the trial)

**Recommendation: KEEP the trial as-is (mounted where the operator
already opted in), decide promotion after the fleet dashboards show a
need.** The 11-11 fleet verdict already landed on KEEP for the trial;
nothing in this window produced new evidence against it (the
`TestServerTimingHeaderBehindRequestLog` guard passes at HEAD — battery
receipt, T03). No code action.

## 4. Harvest policy (§f → TODO_LIST)

**Executed under the blanket directive, with the conservative default:**
capability rows whose owner-ruling was pending were NOT force-ticked; the
one new row filed this window is the T10 re-arm row (upstream-gated,
agent-executable when the cqrs fix lands). The window report's §f rows
carry `fresh:skip` markers per the freshness gate. **Standing default
proposal: a new report's §f harvests automatically into TODO_LIST when a
row is agent-executable, and stays report-only with a `— BLOCKED:` note
when owner-gated.** That keeps the queue dispatchable without forcing
owner rulings; adopt or veto in one line.

## Cadence proposal (libs series)

The 22 libs reports refreshed 11-08/11-09/11-11/11-12: **proposed cadence
= one verify-block replay per month (the ratified digest ritual already
exists) + an upstream tag watchlist round per week (this window's T23
round was zero-drift and cost ~10 minutes)**. Full re-audits stay
event-driven: a major-version bump, a breaking-change changelog line, or
a guard failure. This replaces ad-hoc per-window "should we re-check the
libs" deliberation with two named, cheap rituals.

## Already-executed capability landings this window (for the record)

T08 pins, T10 attempted+reverted (upstream tie-lossy cursor, re-arm row
filed), T11 audit (no tag cut), T12 ssetest swap, T16 abandonment,
T17 off-sentinel, T18 WrapOnce ruled out, T19 metrics flag, T21 both
could-not-verify closures, T23 watchlist (zero drift). Reports
re-scored: go-health-dashboard 79→87, go-health 41→59,
go-error-family 21→29.
