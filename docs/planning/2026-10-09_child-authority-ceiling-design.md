# Design memo — child-task authority & budget ceiling (T13, gated)

**Date:** 2026-10-09 · **Author:** interactive owner window (non-queue)
**Companion:** `docs/research/2026-10-09_turnstone-lessons.md` (lesson #10),
`TODO_LIST.md` "Child-task authority + budget ceiling".
**Status:** DESIGN-GATED. No `internal/task`/claim-path change lands until the
owner rules on the parent-grant model (guardrail 1: hot-path design pauses).

## The rule (turnstone, verified)

`HYPOTHESIS.md`: a child holds at most a **subset** of the parent's grants; a
child's out-of-grant request **routes up** (never auto-widens). Two axes:
**budget** (a child may not out-spend its parent) and **authority** (a child
may not gain a capability the parent lacked).

## Why this is gated, not just done

tq has **no parent link on a task today** (`internal/task` has no `Parent`
field; nothing persists the parent→child edge as a queryable claim input).
Encoding the rule honestly needs, at minimum:

1. a parent reference (`ParentID`) on the enqueued task, persisted and
   indexed so a claim-time check is a cheap lookup — a schema + migration on
   the hot path; and
2. a definition of "authority" for a tq task. Candidates:
   - `Yolo` (agent autonomy) — a child must not be MORE autonomous than its
     parent;
   - effective priority — a child must not outrank its parent's band; and
   - the paid-turn budget — a child's paid turns subtract from the parent's
     cap, not a fresh one.

Each is a distinct owner decision with real behavioral blast radius (e.g. the
incident regression band deliberately raises priority above the first mint's
120 → 150 on purpose, which is a *reaction*, not a child grant — confirming
"authority" and "priority" are not the same axis and the rule must say which
it governs).

## Existing partial coverage (what already holds)

- **DAG deps**: a dependent task cannot run before its dependency completes
  (`ClaimDue` dependency predicate) — a coarse ordering guard, not a grant.
- **Incident minting**: inherits `Project`/`Repo`; the mint rule (not the
  child) chooses the band. It does not *widen* the child's capability — the
  child is an ordinary agent task.
- **Env denylist** (`envDenylistForAgents`) + the M24 minted-per-run
  allowlist design bound what ANY agent child receives, independent of parent.
- **Sweeper-minted children** (review/status/dlqfix) run on the closeout-free
  clone with verdict-gated output — bounded authority by construction.

So the *substance* of lesson #10 is largely already enforced by the ambient
boundary (M24/env denylist). What is missing is the **explicit parent-scoped
ceiling** — a child cannot exceed a *specific* parent rather than the global
floor.

## Recommendation

Do not add a speculative `ParentID` hot-path column yet. The cheap, correct
next rung is to **fold the ceiling into the M24 minted-capability work**: once
a task carries a per-run capability grant, the grant's source (parent or
owner) defines the ceiling, and "route up" is the existing `tq ask` channel.
Until then, this stays a tracked, owner-gated TODO rather than a half-built
hot-path check that could silently change claim behaviour.

## Decision needed from the owner

1. Which axis does the ceiling govern first: autonomy (`Yolo`), priority, or
   paid budget?
2. Is the parent link explicit (`ParentID`) or derived (incident fold,
   `tq ask` lineage)?
3. Fold into M24, or ship a narrow `ParentID` ceiling now?
