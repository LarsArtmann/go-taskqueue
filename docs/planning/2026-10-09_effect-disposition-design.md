# Design memo — effect disposition on reclaim (T1)

**Date:** 2026-10-09 · **Base HEAD:** `795d4da7`
**Author:** interactive owner window (non-queue; no `Task-Queue-ID`)
**Companion:** `docs/research/2026-10-09_turnstone-lessons.md` (lesson #2),
`docs/planning/2026-10-09_19-32_SUPERB-turnstone-lessons-execution-plan.md` (T1/T3)
**Ruling:** owner authorized execution of the plan's critical path; this memo
picks the fact shape so T2/T3 can land without a speculative model.

---

## 1. The problem

`ClaimDue` reclaims a `running` task whose lease expired and whose worker never
recorded a terminal fact (crash, SIGKILL, host death). Today the reclaim path
appends a bare `task.released` fact (`internal/queue/companion/claims.go:165`)
and the task is re-claimed and re-run **as if it had never executed**.

The executed side effect — the agent's commit, the shell command's write — may
already have landed. A clean re-run is then a *double run*, and the journal
cannot tell it apart from a first attempt. Turnstone's `HYPOTHESIS.md` names
this exactly: **"crashes aren't finishes"; `unknown` is not `none`.** A crash
mid-effect must be recorded as an *effect disposition*, not silently retried.

## 2. What the reclaiming observer actually knows

At reclaim the transaction sees only: the task row (`status='running'`, expired
`lease_expires`), and the task's fact history. It does **not** see whether the
executor's process ran, what it wrote, or whether it finished. The only
write-ahead marker is the `task.claimed` fact (journal-before-dispatch, lesson
#1) — which says *the loop intended to dispatch*, not that it did. There is no
"dispatch started" fact, and `task.heartbeat` is written only at cadence
(`lease/4`), so its absence does **not** prove the effect never happened.

Consequence: **the reclaim observer cannot soundly assert `none`.** Any attempt
to infer "never dispatched" from a missing heartbeat would mislabel a real
mid-effect crash as clean — the exact bug we are fixing, one level down.
Turnstone's own direction ("a learned check may only narrow"; never lower a
certainty) fixes the answer: **stamp `unknown`.**

## 3. Options

### Option A — `EffectStatus` on the existing `task.released` detail (CHOSEN)
- `Released` already *is* the event; today its `Detail` is empty. Populate it
  with `{"effect":"unknown","reason":"lease-expiry"}` (a small additive struct).
- Add `EffectStatus` to `internal/journal` with the full turnstone vocabulary:
  `committed | none | unknown | partial | rolled_back`.
- Re-export the type + constants through the `journal/` facade (SCHEDULING).
- **Pros:** purely additive (no new fact type, no readmodel event mapping, no
  bridge-test churn); mirrors turnstone (`EffectStatus` rides the TOOL turn's
  `meta`, it is not a separate event); the reclaim path is a two-line change.
- **Cons:** consumers that switch on `Released` must tolerate a detail.

### Option B — a new `task.effect-unresolved` fact type
- Append a second fact alongside `Released` whenever an effect is unresolved.
- **Pros:** a dedicated event a projection can subscribe to.
- **Cons:** a new vocabulary constant must be mirrored upstream
  (`facts_bridge_test`), taught to `readmodel`/`webui`, and kept in sync across
  the facade — for zero extra information over A. It also risks two facts
  describing one transition, violating "facts in the same tx as state".

**Decision: Option A.** It is the smaller, more turnstone-faithful change.

## 4. Chosen shape

```go
// internal/journal
type EffectStatus string

const (
    EffectCommitted   EffectStatus = "committed"   // effect observed to have landed
    EffectNone        EffectStatus = "none"        // provably never executed (reserved)
    EffectUnknown     EffectStatus = "unknown"     // crashed mid-effect; may have landed
    EffectPartial     EffectStatus = "partial"     // some of a multi-step effect landed
    EffectRolledBack  EffectStatus = "rolled_back" // effect compensated
)

// ReleasedDetail is the `task.released` fact detail: the prior attempt's
// effect disposition, so a reclaim is distinguishable from a clean first run.
type ReleasedDetail struct {
    Effect EffectStatus `json:"effect"`
    Reason string       `json:"reason,omitempty"` // lease-expiry | cancelled-mid-run
}
```

Writers (T3):
- Reclaim branch `claims.go:165` → `Effect: EffectUnknown, Reason: "lease-expiry"`.
- Cancel branch `claims.go:136` (cancel requested, worker unreachable) →
  `Effect: EffectUnknown, Reason: "cancelled-mid-run"`.

`EffectNone` is **defined but unproduced** today: tq has no path that *proves* an
effect never ran. That is honest — turnstone reserves the value for a journaled
`pending` with a confirmed no-execute; tq's observer lacks that proof. A future
explicit never-dispatched release path may stamp it; tests assert we never
mislabel a reclaim as `none`.

## 5. Invariants preserved

- **Additive only.** No change to claim ordering, lease math, or the release
  trigger. The fact *detail* is written; the state transition is untouched.
- **Facts in the same tx as state.** The detail rides the existing `Released`
  append, still inside `ClaimDue`'s transaction.
- **No authority granted.** A disposition is an observation, never a gate.
- **Upstream opaque.** `UpstreamFact`/`JournalFacts` carry `Detail` through
  unchanged; the engine never parses it.

## 6. Verification (T3/T7/T8)

- Conform reclaim test: reclaim a lease-expired running task, assert the
  `Released` fact detail parses to `EffectUnknown`.
- Worker test: kill/expire a run, reclaim, assert disposition + re-run.
- e2e chaos: kill the worker mid-effect; the reclaim shows `unknown`.
- `tq facts` / webui render the disposition (T5).
