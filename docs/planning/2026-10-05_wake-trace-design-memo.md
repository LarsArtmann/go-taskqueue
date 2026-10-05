# Wake-trace design memo — durable evidence for suppressed harvest triggers (M3)

**Date:** 2026-10-05. **Status:** DESIGN ONLY — implementation (plan M4) is gated on owner ruling §g-2 (new fact type vs evidence key). This memo is the ruling's input; nothing here is merged behavior.
**Gap (verified 2026-10-04):** harvest paces repos at one live task each ("paced: one new item per repo per run"); a trigger suppressed by the pacing rule leaves NO journal trace. When work sits in TODO_LIST for days, drift forensics cannot answer "was this ever seen?".
**Paperclip's verified model:** `agent_wakeup_requests` rows carry `coalesced_count` + full lifecycle (`docs/research/2026-10-05_paperclip-lessons.md`, citation 2) — suppressed wakes merge INTO a durable row, never vanish.

## 1. The two options

|               | **A: new fact type `task.wake`**                                                                                           | **B: evidence key on the paced skip**                                                                                         |
| ------------- | -------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| Meaning       | "a trigger fired and was merged/suppressed" is a DOMAIN EVENT in its own right                                             | the skip is EVIDENCE attached to some other fact — but no other fact exists on the skip path                                  |
| Write path    | `store.AppendFact` (the session-fact seam, `internal/session/session.go:125` — synthetic task id, no Store-surface change) | NONE available: the skip produces no store call today; inventing a carrier fact for its evidence is option A with extra steps |
| Consumer cost | each consumer's fact switch gains a case (or ignores it, as the bridge's deliberate subset does)                           | consumers must parse `task.requeued` details they currently classify — risk of misclassifying skips as retries                |
| Audit query   | `tq facts -type task.wake` / CountFacts pushdown, symmetric with every other question                                      | hidden inside free-text evidence; no pushdown                                                                                 |
| Cost          | one const + one case per consuming surface                                                                                 | looks cheaper, buys a lies-in-its-channel fact                                                                                |

**Recommendation: A.** The journal IS tq's domain-event log (ADR-0001 facts-first); a suppressed trigger is a fact about the world, not a footnote. Paperclip's `coalesced_count` is the same shape rendered as a row. Option B has no honest carrier.

## 2. Draft fact shape (option A)

```go
// internal/journal/journal.go
Wake FactType = "task.wake" // a harvest trigger fired while the repo was
// occupied (one-live-task-per-repo pacing); the item was seen and merged
// into the running task's backlog view, NOT enqueued. Detail carries the
// evidence; the task id is synthetic (wake:<project>).
```

```go
// internal/harvest (WakeDetail), mirroring the RequeueEvidence pattern:
type WakeDetail struct {
    Repo      string `json:"repo"`      // repo base name (the pacing scope)
    Item      string `json:"item"`      // the TODO_LIST item text seen
    ItemKey   string `json:"item_key"`  // SORTED-key hash (dedup parity)
    Reason    string `json:"reason"`    // "paced: one live task per repo"
    Coalesced int    `json:"coalesced_count"` // merges since the live task started
}
```

Semantics: ONE `task.wake` fact per repo per tick (deduped, seq-derived idempotency — replay-safe by construction); `coalesced_count` increments ride the NEXT tick's fact, keeping writes append-only (no update path). Synthetic task id `wake:<project>` mirrors `session:<id>`.

## 3. Consumer ripple inventory (pinned-test checklist, M3.3)

| Surface          | File                                                                                                                                             | What M4 must add                                                | Pinned tests to update                               |
| ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------- | ---------------------------------------------------- |
| journal consts   | `internal/journal/journal.go`                                                                                                                    | `Wake` const + doc comment                                      | fact-count tables in `internal/journal` tests        |
| harvest write    | `internal/harvest/harvest.go` (occupied-skip path in `runRepo`)                                                                                  | AppendFact on skip                                              | `TestSelfManagingLoop` family (known-flaky — re-run) |
| journalaudit     | `cmd/tq/journalaudit.go` (10-case switch)                                                                                                        | wake count line in the requeues block                           | `journalaudit_test.go` fixtures                      |
| webui            | `internal/webui/payload.go` retryTrail switch EXCLUDES wake deliberately (a wake is not a retry); badge = CountFacts-based lamp in `StatusCards` | wake lamp + test                                                | `webui_test.go` segment pins                         |
| readmodel        | `internal/readmodel/events.go` (9-case projection)                                                                                               | `nWake` counter + column + parity                               | readmodel parity suite (both engines)                |
| papdashboard     | `internal/bridge/papdashboard/papdashboard.go` forward switch — DELIBERATE subset (nolint:exhaustive)                                            | NO case needed (not owner-actionable)                           | none                                                 |
| conform          | `internal/queue/companion/conform`                                                                                                               | ONLY if Store surface changes — it does not (AppendFact exists) | none                                                 |
| webui facts feed | `internal/webui` journal browser                                                                                                                 | renders unknown types generically (verify)                      | snapshot if generic rendering changes                |
| DOMAIN_LANGUAGE  | `docs/DOMAIN_LANGUAGE.md`                                                                                                                        | "wake" entry (M4.7)                                             | `check-doc-refs`                                     |

## 4. What the ruling decides (§g-2)

1. Option A (recommended) vs B.
2. Fact NAME: `task.wake` vs `harvest.wake` (the trigger is harvest-owned; `task.` prefixes task-lifecycle facts today — a `harvest.` namespace would be the first non-task fact family, which `session.*` already implicitly started under `task.`-less naming… note: session facts use `session.opened`, NOT `task.*` — so precedent supports `harvest.wake`). The memo drafts `task.wake` only because M4.1 in the plan says "journal: add fact type const"; the name is cheap to change pre-implementation.
3. Coalesced-count-as-new-fact vs the paperclip exact-row model (a mutable wakeup row). tq's journal is append-only — the new-fact-per-tick shape is the faithful translation.
