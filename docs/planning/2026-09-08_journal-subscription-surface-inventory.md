# Journal / Tailer Subscription Surface — Inventory and Subscribe Gap Analysis

**Date:** 2026-09-08
**Status:** DONE (pre-implementation gate: every claim below was verified by
reading the cited code this session; no code was changed)
**Scope:** Fulfill the TODO_LIST "High Impact" item: inventory the
journal/tailer subscription surface (`internal/journal`,
`internal/webui/tailer.go`), document the gaps vs the proposed
`Subscribe(ctx, since Seq)` fact-stream contract from the 05:24 report, and
confirm that `2d5e729`'s bounded journal reads cannot break replay-from-seq.
**Inputs:** 05:24 report (`docs/status/2026-09-08_05-24_cordis-samber-evaluation-journal-bus-recommendation.md`,
d1/f1/f3), commit `2d5e729`, AGENTS.md, ADR-0001/0003, and the source files
cited inline.

---

## Verdicts up front

1. **`2d5e729` is safe for replay-from-seq.** Every replay-capable read path
   either passes `limit=0` (unbounded, cursor semantics unchanged) or pages
   correctly (cursor = last delivered seq, loop until a short batch). The
   bounded reads that DO discard history (`LastFacts`, the webui tailer's
   batch jump, bounded `FactsForTask`) are feed/window renders, never replay
   paths. Detail in §3. No fix needed; one forward-looking caveat (compaction)
   recorded in §3.4.
2. **The 05:24 recommendation's pull half already exists.** `queue.Store`
   already speaks `Facts(after, limit)` + `HeadSeq()` — a paginated,
   O(limit)-per-page `Since` with an O(1) watermark. The report's d1
   self-flag ("designed without reading the Journal interface") was correct
   in an unexpected direction: the `journal.Journal` interface it targeted is
   production-dead; the real seam is `queue.Store`. The actual gaps are all
   on the push side (§4).
3. **A `Subscribe` implementation must NOT copy the tailer's watermark-jump
   trick** — it is correct only because the webui tailer is a change signal,
   not a fact deliverer (§2.1). The papdashboard bridge is the reference
   exact-delivery loop a dispatcher should generalize (§5).

## 1. Two parallel journal abstractions (split brain, pre-existing)

### 1.1 `internal/journal` — the nominal domain interface, production-dead

`journal.Journal` (`internal/journal/journal.go:41`) defines
`Append(ctx, Fact) / All(ctx) / Since(ctx, after)`. Facts:

- Its only implementation is `MemoryJournal` (mutex + slice).
- Its only consumers are `internal/journal/memory_journal_test.go` and
  `internal/budget/budget_test.go` (which hand-adapts `MemoryJournal` into
  the guard's `FactSource`). **No production code holds a `journal.Journal`.**
- `Since`/`All` are unbounded — `2d5e729` did not touch this package.
- What IS production-live from this package is the vocabulary:
  `journal.Fact` / `FactType` are the data types every consumer uses.

### 1.2 `queue.Store` — the real production fact seam

All persistent fact I/O flows through `queue.Store`
(`internal/queue/queue.go:58-74`), implemented by `SQLiteStore`:

- **Writes**: no public append. Facts appear only via the private
  `appendFact(ctx, tx, …)` (`internal/queue/sqlite.go:166`) inside each
  mutation's transaction — the "in-tx facts" invariant. There is no way to
  append a fact through any public API without a queue mutation.
- **Reads** (all shaped by `2d5e729`):
  - `Facts(ctx, after, limit)` — `WHERE seq > ? ORDER BY seq ASC [LIMIT ?]`;
    `limit=0` = unbounded. Cursor pagination, O(limit) per page via the seq
    primary key (`sqlite.go:711`).
  - `LastFacts(ctx, limit)` — most recent N ascending (DESC + reverse);
    `limit<=0` = whole journal (`sqlite.go:731`).
  - `HeadSeq(ctx)` — `COALESCE(MAX(seq),0)`, the O(1) watermark
    (`sqlite.go:757`).
  - `FactsForTask(ctx, id, limit)` — per-task trail via `idx_facts_task`
    (`sqlite.go:767`).
  - `CountFacts(ctx, type, since)` — count pushdown (`sqlite.go:787`).
- **Seq assignment**: `facts.seq INTEGER PRIMARY KEY AUTOINCREMENT`
  (`sqlite.go:119`) — monotonic, never reused.
- **Retention**: append-only in practice. There is no `DELETE FROM facts` or
  `VACUUM` path anywhere (grep-verified). ADR-0001 explicitly defers
  compaction; ROADMAP carries a "journal compaction design note" item.

**Consequence for `Subscribe`:** it must be designed against the
`queue.Store` read surface (or against a new seam composing it) — putting it
on `journal.Journal` would attach it to an interface with zero production
implementations, and `MemoryJournal.Since` cannot even express the
limit/pagination vocabulary the dispatcher needs. See §4 gap 4.

## 2. Every fact consumer today (all pull-based; no push path exists)

| # | Consumer | Seam & cadence | Watermark start | Batching / cursor | Delivery semantics |
|---|----------|----------------|-----------------|-------------------|--------------------|
| 1 | webui tailer (`internal/webui/tailer.go:18`) | `Facts(watermark, 1000)` every `cfg.Poll` (default 500ms) | `HeadSeq()` (no history replay; clients snapshot on connect) | **jumps watermark to the batch's last seq** — a burst >1000 skips middle facts | notification-only ("something changed"); never delivers facts |
| 2 | webui hub (`internal/webui/hub.go`) | `sse.Broadcaster[sse.Event]` fed by the tailer | event ID = watermark seq | per-client buffer 128, **drop on overflow**, payload-less "tick" | projection consumers self-heal via full re-render |
| 3 | webui SSE handler (`internal/webui/handlers.go:98`) | subscribe-then-snapshot; `Last-Event-ID` accepted but satisfied by a fresh full snapshot | n/a (snapshot is the resume) | n/a | client-side replay-from-ID does not exist (f40 spike) |
| 4 | papdashboard bridge (`internal/bridge/papdashboard/papdashboard.go:116`) | `Facts(watermark, 500)` every 5s, drain-to-head in a loop | `HeadSeq()` unless `Config.FromSeq` set — **the one existing replay-from-seq consumer** | cursor advances per fact after 2xx/permanent 4xx; failed forward stops the drain and retries from the last forwarded seq | at-least-once per process, Idempotency-Key = fact seq; watermark volatile (missed-incident gap unchanged) |
| 5 | budget guard (`internal/budget/budget.go:78`) | not a stream: `CountFacts(Enqueued, startOfDay)` pushdown per tick | none | n/a | exact count, no ordering |
| 6 | `tq facts --after` (`cmd/tq/main.go:1081`) | one-shot | `--after` (replay OK) | `Facts(after, 0)` unbounded | exact |
| 7 | `tq tail [-f] --after` (`cmd/tq/main.go:1124`) | 500ms poll | `--after` (replay OK) | `Facts(after, 0)` unbounded, cursor advances per fact | exact |
| 8 | `tq top` (`cmd/tq/top.go:178`) | per frame | none (window) | `LastFacts(5000)` | recent-window projection, no replay |
| 9 | `tq show <id>` (`cmd/tq/main.go` cmdShow) | one-shot | n/a | `FactsForTask(id, 0)` unbounded trail | exact |
| 10 | webui feed / detail (`internal/webui/render.go:264`, `tailer.go:59`) | on demand | n/a | `LastFacts(50)` / `FactsForTask(id, 500)` | window/trail render |
| 11 | `examples/api`, `examples/sse` | HTTP poll | 0 / request cursor | `Facts(after, 0)` unbounded | exact |

Reading of the table: **four independent pollers** (tailer, bridge,
`tq tail`, examples) each own a cursor and a timer; an append wakes nobody.
The hub is the only fan-out primitive, and it is payload-less and lossy by
design.

## 3. `2d5e729` vs replay-from-seq — confirmation of safety

### 3.1 Why cursor replay is exact

`Facts(after, limit)` replay correctness requires all of:

- **(a) strictly-greater cursor** — holds: `seq > ?` (`sqlite.go:714`). Seq
  gaps (AUTOINCREMENT skips values after rolled-back transactions) are
  harmless; no consumer assumes contiguity (the bridge ends its drain on
  `len(facts) < limit`, the tailer/tail take the last returned seq — none do
  seq arithmetic).
- **(b) monotonic, never-reused seq** — holds: `INTEGER PRIMARY KEY
  AUTOINCREMENT` (`sqlite.go:119`).
- **(c) append-only retention** — holds today: no delete path exists
  (grep-verified). See §3.4 for the caveat.
- **(d) bounded callers page correctly** — holds at every bounded call site:
  - Bridge: drains `Facts(watermark, 500)` in a loop, advances the watermark
    per accepted fact, stops on failure and retries from the last forwarded
    seq. Pinned by tests: `TestFactsCursorBounded` (pagination exactness,
    `internal/queue/store_test.go`) and the bridge retry tests.
  - Webui tailer: bounded but deliberately lossy-in-the-middle — see §3.2.

### 3.2 The tailer's batch jump is not a replay break

On a burst > `tailBatchLimit` (1000) the tailer sets its watermark to the
batch's last seq and the skipped middle facts are never delivered — but the
tailer's contract (its own comment, `tailer.go:11-17`) is "signal that
something changed": the hub tick triggers a full server-side re-render of
current projections. No fact consumer downstream of the hub receives
per-fact delivery, so nothing that expects replay gets loss. **The load-bearing
distinction for `Subscribe`: this jump is only valid for change-signal
consumers; a fact-delivering subscriber must advance its cursor per fact
(bridge-style), never batch-jump.**

### 3.3 History-discarding reads are never replay paths

`LastFacts` (feed: 50, top: 5000) and bounded `FactsForTask` (detail: 500)
render recent windows of a projection; no cursor is derived from them that
later feeds a "continue from here" expectation. `HeadSeq` merely replaced
the bridge's former "scan all facts, take max" with `MAX(seq)` — same value,
O(1).

### 3.4 Forward-looking caveat (record for the Subscribe contract)

If journal compaction ever lands (ROADMAP item), `Facts(after, …)` with an
`after` older than the retention floor would silently start at the oldest
retained fact. The `Subscribe` contract should define that case explicitly
(error vs silent-resync) BEFORE any compaction exists; today it is
unreachable. The one behavioral replay gap that exists today is unchanged by
`2d5e729` and out of its scope: the bridge restarts at the journal head, so
incidents fired while it was down are never replayed — a watermark
persistence gap (TODO_LIST H2), which `HeadSeq` actually made O(1).

## 4. Gaps vs the proposed `Subscribe(ctx, since Seq)`

### Already exists (d1 correction — do not rebuild)

- The **pull half**: `Facts(after, limit)` + `HeadSeq()` is a paginated
  `Since` with an O(1) watermark, on the production seam.
- A **replay-from-seq consumer**: `bridge.Config.FromSeq`.
- A **fan-out primitive**: the webui hub (payload-less, drop-on-overflow).

### The real gaps (what Subscribe would add)

1. **No push path / no shared dispatcher.** Appends wake nobody; N consumers
   poll independently (latency floor = 500ms UI, 5s bridge) and each pay
   their own read cost. Subscribe = one reader, notify-on-append, fan-out.
2. **No delivery contract per subscriber class.** Today each consumer picks
   its own semantics (tailer jumps, bridge exact, hub drops). Subscribe must
   define: per-fact cursor advance; slow-consumer policy — block / drop /
   ring + lag signal (05:24 f7); at-least-once vs at-most-once per consumer
   class. Drop-on-overflow is correct ONLY for projection consumers.
3. **Watermarks are volatile.** Bridge/tailer watermarks die with their
   process. Subscribe composes with — but does not replace — the persisted
   per-bridge watermark work (TODO_LIST H2).
4. **Journal-interface split brain.** `journal.Journal` has zero production
   consumers; `MemoryJournal.Since` has no limit parameter. Prerequisite
   decision: unify (make the interface express pagination) or demote
   `Journal` to the test-double it already is, and design Subscribe over
   `queue.Store`.
5. **Dispatch must not touch the write path.** The single serialized writer
   invariant (`MaxOpenConns(1)`, AGENTS.md) means fan-out must happen
   outside the mutation transaction — notify after commit, or page via
   bounded reads like every consumer today (05:24 f8).
6. **SSE client resume is snapshot-based.** `Last-Event-ID` is accepted but
   not mapped to journal replay (ADR-0003 projection model — still valid for
   the dashboard). Subscribe's per-connection `since` is what would make the
   f40 spike (Last-Event-ID ↔ Seq) implementable.
7. **No filtered streams.** Per-task/per-type views exist only as one-shot
   indexed reads (`FactsForTask`), not subscriptions.
8. **No lag observability.** `HeadSeq` makes per-subscriber lag trivially
   computable (`head − cursor`); nothing exposes it today (05:24 f16).

## 5. Stance for the implementation TODO (non-binding inputs)

- Build the dispatcher as a new seam composing `Store` bounded reads: page
  from `since` with the bridge's loop shape (cursor = last delivered seq,
  terminate on short batch), then poll-or-notify. Do not add `Subscribe` to
  the `Store` interface before the slow-consumer policy (gap 2) is decided.
- The bridge is the reference exact-delivery consumer; generalizing its loop
  must preserve per-fact cursor advance, at-least-once, and idempotency
  keys. The tailer's batch jump must never leak into delivery paths.
- The hub can stay a projection-notifier even after Subscribe exists; if it
  is migrated, keep payload-less ticks or the SSE wire format breaks
  (05:24 f12).
- Resolve gap 4 (interface unification) in the same change that introduces
  Subscribe — otherwise a third abstraction appears next to the split brain.

## Verification

- All paths/line numbers above were read from HEAD this session
  (working tree at session start: `78522cc`).
- Re-verified 2026-09-08 against HEAD `e221e30` (after `f67942f`
  shifted `sqlite.go` +32 and the webui feed render): every claim re-checked
  against the code, drifted line references corrected, no substance change.
- Gates: `go build ./...`, `go vet ./...`, `go test ./... -race` green
  (doc-only change).
