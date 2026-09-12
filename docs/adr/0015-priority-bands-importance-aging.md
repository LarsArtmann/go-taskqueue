# ADR-0015: Priority bands, importance, aging, and mutable priority

**Status:** Accepted (2026-09-12)
**Context:** tq's claim order is `priority DESC, created_at ASC, id ASC`
(`internal/queue/sqlite/sqlite.go:361`, mirrored in postgres). Priority is a
per-task int set once at enqueue and never revisited; the only emitters today
are `tq enqueue --priority` (default 0), `tq harvest --priority` /
`--same-session-priority` (both default 0 / disabled), and the cqa bridge's
hardcoded 80 (`internal/bridge/cqa/cqa.go:225`). Minted tasks (review, dlqfix,
status, session-close) inherit their parent's priority. Nothing reads
per-project importance, nothing re-scores pending tasks, and low-priority
items can starve indefinitely behind higher ones — the failure modes that
make a 2000-issue / 50-project backlog drift into "not organized".

Two sibling systems hold the missing pieces: project-meta
(`~/projects/project-meta`) stores per-repo `importance` (0–100, named levels)
in each repo's flat `.config/metadata.yaml`; ai-task-prioritizer
(`~/projects/ai-task-prioritizer`, "ATP") demonstrates weighted scoring,
content-hash score caching, and fallback chains — and equally instructive
failure modes (two disjoint 0–10/0–100 scales, and a flagship path that runs
on label heuristics because its DB score bridge is stubbed at
`internal/cli/next_task_issue_conversion.go:101`).

**Decision:**

## 1. One scale, three bands

All priorities live on ONE integer scale with reserved upper bands:

| Band    | Range   | Who may set it                                            |
| ------- | ------- | --------------------------------------------------------- |
| backlog | 0–99    | humans (markers), importance, AI scores, keyword fallback |
| hot     | 100–149 | `--same-session-priority` only (session-scoped urgency)   |
| machine | 150+    | machine-generated operational tasks (cqa fixes today)     |

Backlog computations (importance base, keyword bumps, AI scores) CLAMP to
0–99: no scorer, keyword rule, or importance value can smuggle a task into
hot or machine. Crossing a band boundary requires an explicit human/flag
act. cqa migrates 80 → 150 ("concrete scanner findings outrank generic
backlog items" — including all of them — is a band statement, not a backlog
ranking). Minted tasks keep parent inheritance regardless of band.

## 2. Marker grammar: `— P1:` … `— P4:`

A TODO_LIST item may end with a priority marker following the established
`— BLOCKED:` suffix convention (em dash U+2014, `scripts/check-todo-list.sh:19`,
`internal/harvest/harvest.go:361`):

```
- [ ] Harden the token check — P1: security-adjacent
- [ ] Refresh vendored CSS — P3
```

Grammar: `— P[1-4]` optionally followed by `: <free-text note>`, at end of
line. `— BLOCKED:` keeps absolute precedence (a blocked item is skipped
whole; its marker is irrelevant). Mid-text "P1" (e.g. "add a P1 DNS record")
never matches — only the trailing em-dash segment counts. One marker per
item; first match wins.

Mapping: **P1=90, P2=70, P3=50, P4=30.** Adjacent levels differ by 20 >
MaxAgeBonus (10), so aging can reorder within a band but never across marker
levels — owner intent is decay-proof.

**Markers are stripped from item text BEFORE the dedup-key hash.** The key
hashes repo + marker-stripped text; editing a marker therefore never forks or
re-mints a task. Prune-stale's reword semantics (key changes when text
changes) are unaffected.

## 3. Precedence

When harvest computes an effective priority for an item:

1. **human marker** (P1–P4 → 90/70/50/30) — overrides everything below;
2. **hot** — `/tmp`-referencing same-session items go to
   `SameSessionPriority` (recommended: 120, i.e. hot band) regardless of
   marker (files won't survive the session — urgency dominates);
3. **AI score** — cached `priority_scores` row for the item key, if present
   (Phase 5; clamped 0–99);
4. **keyword bumps** — base importance + table below, clamped 0–99;
5. **default** — importance from `.config/metadata.yaml` (default 50).

Keyword table (rewritten from ATP's urgency-keyword idea per §7; one bump,
strongest match wins — not a stack):

| Match (case-insensitive, item text) | Bump |
| ----------------------------------- | ---- |
| `security`, `vulnerab`, `CVE`       | +30  |
| `critical`, `urgent`, `ASAP`        | +25  |
| `production`, `breaking`, `outage`  | +20  |

For mutable priority (`tq reprioritize`, sweepers, T27 verdicts) the same
ladder applies as a RESOLVER over the store's mechanical
`UpdatePendingPriority`: the resolver never writes a hot/machine task
(band protection) and skips no-op writes (same value ⇒ no fact).

## 4. Aging: a claim-query term, keyed on `created_at`

`ClaimDue` orders by `priority + LEAST(age/AgingDays, MaxAgeBonus)` (default:
+1 point per **3** days, capped at **+10**) instead of bare `priority` —
sqlite first (`MIN(a,b)` scalar form for compatibility), postgres mirrored
(`LEAST`). Aging is scheduling, not state: the stored priority never
changes, `tq tasks`/webui keep showing stored values, and the bonus is
recomputed per claim. Parameters live beside the claim constants and are
documented in one place.

Aging key is **`created_at`**, because:

- it is the only immutable timestamp in the row — `updated_at` is touched by
  every heartbeat and lease renewal (`sqlite.go:633`), so keying on it would
  make aging a function of worker liveness;
- a last-requeue timestamp would need a new column (schema churn; invariant
  3 forbids it in phase 1) and would double-punish retried tasks: attempts
  are already burned on failure, and rate-limit requeues are intentional
  parking gated by `not_before` — aging them faster inverts the park;
- the cap (+10 < 20 = smallest marker gap) bounds the distortion: aging can
  flip importance-vs-importance and importance-vs-keyword order, never
  marker-vs-marker or band-vs-band.

## 5. Mutable priority: `task.reprioritized` fact + PENDING-only update

Store gains `UpdatePendingPriority(taskID, newPriority, source, reason)`:
PENDING-only guard (running/terminal tasks untouched — invariant 4),
`RowsAffected()` re-check, and the `task.reprioritized` fact
`{task_id, old_priority, new_priority, source, reason}` appended IN THE SAME
TRANSACTION (invariant: facts in the same tx as state). Sources:
`marker | importance | ai | keyword | manual | unblock | migration`.
Same-value writes are no-ops (idempotent; no fact). The fact replays
through the journal tail / consumer cursors like every other fact.

## 6. Migration story

Live journal (dogfood pool) carries: default-0 harvest/enqueue tasks, cqa
80s, and minted inheritors. Migration (T22) is an OFFLINE `tq reprioritize`
run after backup: enumerate live values read-only first; map cqa 80 → 150;
leave 0-priority tasks to be re-scored by the importance pass. Row counts
are verified before/after; any mismatch ⇒ restore + stop (plan §7).

## 7. Provenance, licenses, and porting rules

Spot-verified at source (2026-09-12, flipping prior [a] tags to [v]):

- ATP weights Age .2 / Activity .3 / Dependency .25 / Impact .15 / Urgency .1
  sum to exactly 1.0 — `pkg/core/priority/score.go:71–76`, test
  `score_test.go:143`. Correction: `pkg/models/priority.go` DOES exist
  (config + ValidateWeights); earlier claim it didn't was wrong.
- ATP flagship stub confirmed: `fetchRealPriorityScore` returns
  `(0, "", 0)` with a "schema isn't fully implemented" comment —
  `internal/cli/next_task_issue_conversion.go:101–110`. Lesson encoded:
  the claim path must read the real column, never a label fallback.
- ATP cache: content-hash keyed (`models.NewIssueContent(issue).Hash()`),
  memory + disk tiers, `DefaultCacheTTL = 24h`
  (`pkg/constants/defaults.go:120`).
- ATP "dedupe" is go/ast code dedup (`pkg/utils/dedupe.go`) — nothing to
  port for issue-level dedup (tq's dedup-key hash stays exact-match).

**Porting rules:** ATP LICENSE is PROPRIETARY (all rights reserved) and
project-meta is PROPRIETARY — neither may be imported, copied, or
vendor-copied into this all-MIT tree. Every table, formula, and behavior
above is REWRITTEN FROM SPEC (this ADR is the spec) with ATP credited as
inspiration in prose only. project-meta is consumed exactly via its flat
`.config/metadata.yaml` file contract (documented in ADR-0015's context and
`docs/planning/2026-09-12_14-03` research): strict-subset reader, default
importance 50 when absent, malformed file ⇒ repo skip reason (the
`ReasonScanFailed` pattern).

## 8. Rejected alternatives

- **Materialized `effective_priority` column** — schema churn (invariant 3)
  plus an update storm: the aging component changes with time, so the column
  is stale between writes and must be recomputed on a timer. A query-time
  term is always correct and costs nothing to maintain.
- **ATP-style float scores on two scales** — two disjoint 0–10 / 0–100
  scales with two threshold sets is ATP's most confusing surface; one int
  scale with named bands is greppable, sortable, and human-settable.
- **Aging keyed on `updated_at` / last-requeue** — see §4 evidence.
- **Markers stored in payload or a side column** — invisible to the claim
  query and orthogonal to the dedup key; text-suffix parsing keeps the file
  the single source of truth and the key stable.
- **Importing project-meta / ATP modules** — license poison (§7).
- **Priority mutation of non-pending tasks** — a running task's lease and
  the worker's in-flight accounting assume enqueue-time priority; terminal
  tasks are history.

## 9. Risks

- Dual-backend drift of the aging term — mitigated by the mirrored
  conformance case landing WITH each backend change (plan invariant 8).
- Aging × NotBefore interactions — dedicated interaction tests (T08) gate
  phase 1 before anything builds on it.
- Marker parser vs. free text — fuzz corpus + the strict trailing-segment
  grammar; mid-text P1 never matches.
- Band migration on the live journal — offline, enumerated, backed up,
  verified (§6).

## 10. Deferred: effort-aware claims (ruling G2)

The score cache persists `effort_minutes` per item, but claim selection
stays priority+aging only. G2 (owner-ratified default): the daily budget
is GLOBAL-FLAT — every pending task competes on the same ladder; effort
does not bias claims while budget remains, and near-exhaustion preference
for cheap tasks is deliberately NOT built. Prerequisite plumbing (the
cache column) exists; the claim-side term would be
`ORDER BY ..., effort_minutes ASC` joined through the score cache, gated
on remaining-budget < threshold. Flip G2 first — building the term now
would ship a dead knob (YAGNI).
