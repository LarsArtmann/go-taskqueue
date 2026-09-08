# ADR-0009: Journal Subscription Policy (dispatcher phase 1)

**Date:** 2026-09-08
**Status:** Accepted
**Depends on:** ADR-0001 (facts-first), ADR-0003 (web UI projection model),
ADR-0004 (framework-free stance), ADR-0006 (compaction — contract
interaction), the subscription-surface inventory
(`docs/planning/2026-09-08_journal-subscription-surface-inventory.md`)
and the persisted-watermark design
(`docs/planning/2026-09-08_persisted-bridge-watermark-design.md`, shipped).

## Context

The journal is the bus (ADR-0004's verdict): `queue.Store` already exposes
the pull half (`Facts(after, limit)`, `HeadSeq`, `FactsForTask`) and, since
the watermark ship, persisted consumer cursors (`Watermark`/`SaveWatermark`).
What does not exist is a push seam: every consumer polls independently
(latency floor = its poll interval; each pays its own read cost), there is
no delivery contract per subscriber class, no lag observability, and
`journal.Journal` is a second, production-dead abstraction next to the
Store (the inventory's split brain).

Before dispatcher code exists, the policy questions must be answered —
otherwise every subscriber reinvents semantics (today: the bridge delivers
exactly, the tailer jumps batches, the hub drops on overflow — each correct
for its class, but undefined as a contract).

## Decisions

### D1. Two subscriber classes, two delivery contracts

| Class | Members today | Contract | Slow-consumer policy | Cursor |
| --- | --- | --- | --- | --- |
| **Exact** | papdashboard bridge, review sweeper, future outbound bridges | every fact, in Seq order, **at-least-once** | **block** (backpressure): the dispatcher never skips or drops for an exact consumer; the consumer persists/checkpoints at its own pace | per-fact, persisted via the `watermarks` table, monotonic |
| **Signal** | webui hub/tailer (projection notifiers) | a wake-up when facts changed; payloads stay out (ADR-0003 wire format) | **drop + coalesce**: overflow drops the notification, never blocks the dispatcher; the next snapshot re-renders truth | none (snapshot-at-head IS the resume) |

A consumer declares its class by which API it uses. Exact consumers use
`Subscribe` (per-fact delivery); signal consumers keep the hub. The
dispatcher never converts between classes implicitly.

### D2. Slow-consumer semantics are per-class, not configurable

Block/drop is not a per-subscriber knob — it is the class's defining
property. An exact consumer that cannot keep up applies backpressure (its
checkpoint gate already does: the bridge stops its drain on checkpoint
failure). A signal consumer that cannot keep up loses ticks, which is
correct because ticks carry no state. Making this configurable would let a
future caller silently downgrade at-least-once to at-most-once; the policy
is therefore structural.

### D3. `journal.Journal` is demoted to a test double

Production code reads facts through `queue.Store` only. `journal.Journal` /
`MemoryJournal` (zero production consumers at decision time) remain for
tests that want an in-memory fact log; nothing new may import them outside
tests. The alternative — unify by growing `Journal` with pagination — would
enrich an interface with no production consumers while the Store already
expresses everything the bus needs. This resolves the inventory's gap 4.

### D4. The dispatcher is a new seam over Store bounded reads — not on `Store`

`internal/consumer` (phase 1) pages via `Facts(since, limit)` with the
bridge's loop shape (cursor = last delivered seq, short batch = drained),
fans out per-fact to exact subscribers under D1 semantics, and exposes
`Lag() = HeadSeq − cursor` per subscriber. The `Store` interface stays
pull-only; no `Subscribe` method is added to it (the inventory's §5 stance,
now binding). Wake strategy v1 is a shared poll ticker; a
notify-after-commit hook is the documented v2 path and MUST fire outside
the mutation transaction (single-serialized-writer invariant, ADR-0001).

### D5. Compaction interplay: loud resync, never silent skip

If journal compaction (ADR-0006) ever removes facts below a subscriber's
persisted cursor, the dispatcher must refuse to guess: it surfaces a loud
resync condition (error/log naming the retention floor) instead of
advancing silently. Gap detection via `first-returned-seq > cursor+1` is
explicitly NOT a signal — AUTOINCREMENT seq gaps from rolled-back
transactions are normal. The rule keys on the archive's retention floor
(`ArchiveFactsBefore`), which is authoritative.

## Consequences

- The bridge could later migrate onto `Subscribe` (it is the reference
  exact consumer) — only after its restart battery stays green; until then
  the bridge keeps its own drain loop (no forced migration).
- The hub stays payload-less regardless (ADR-0003); migrating the tailer
  onto the dispatcher must preserve the batch-jump's signal-only nature —
  the jump never becomes a delivery path.
- `tq stats`/logs gain per-subscriber lag (D4) — the ops surface for both
  classes.
- Postgres and SQLite both satisfy the seam (both implement the watermark
  and paging methods).
