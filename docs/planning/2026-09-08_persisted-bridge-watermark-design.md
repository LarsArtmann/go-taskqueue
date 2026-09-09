# Persisted Bridge Watermark — Design Enumeration

**Date:** 2026-09-08
**Status:** DONE (pre-implementation design gate: every code claim below was
verified by reading the cited code this session at HEAD `747ea0f`; no code
was changed)
**Scope:** Fulfill the TODO_LIST "High Impact" item: enumerate what a
persisted bridge watermark needs — storage shape (side table vs metadata
fact), ack semantics, idempotent resend window — against the real
`startWatermark` code in `internal/bridge/papdashboard` (05:24 report §c and
f9/f10: "Persisted watermark storage: side table vs metadata-fact (projection
purity)" / "Migrate papdashboard bridge to resume-from-watermark"; the
missed-incident gap is cross-process and framework-independent).
**Companion doc:** `docs/planning/2026-09-08_journal-subscription-surface-inventory.md`
(the subscription-surface inventory this builds on; its §4 gap 3 named
volatile watermarks as a Subscribe-adjacent but independent need).

---

## Verdicts up front

1. **Storage shape: side table wins; metadata fact is rejected.** A bridge
   watermark is consumer progress, not domain truth — persisting it as a fact
   would put a synthetic non-task row into an append-only _task_ journal whose
   schema (`task_id TEXT NOT NULL`, `sqlite.go:122`), FactType vocabulary, and
   consumer switches all assume task lifecycle. Detail in §2.
2. **Ack semantics: checkpoint per drained batch, after the last fact is
   accepted — never before.** Per-fact persistence is correct but write-amplifies
   the single serialized writer connection for no delivery gain, because the
   resend window is already made harmless by the seq-derived Idempotency-Key.
   Detail in §3.
3. **Idempotent resend window: bounded by the checkpoint cadence, not by
   history.** Crash between checkpoints re-sends at most one drain batch
   (≤ `forwardBatchLimit` = 500 facts) with identical idempotency keys;
   PapDashboard-side key dedupe absorbs it. The window's honest caveat:
   it assumes PapDashboard remembers keys across the bridge's restart gap.
   Detail in §4.
4. **The watermark is only half the volatility.** `Bridge.alerted`
   (`papdashboard.go:78`) — the map that correlates a completion to the alert
   it should resolve — dies with the process too, and a persisted watermark
   makes its loss _more_ likely to matter (the DeadLettered fact is never
   re-delivered after restart, so the map cannot be rebuilt by replay alone).
   The fix belongs in the same change: derive the correlation from the task's
   own fact trail (`FactsForTask`) instead of memory. Detail in §5.
5. **One report correction:** 05:24 f11 ("Migrate cqa bridge likewise") does
   not apply — the cqa bridge is not a journal consumer at all (no
   `Facts`/`HeadSeq` usage in `internal/bridge/cqa/cqa.go`; it polls the CQA
   API and enqueues with dedup keys). Its idempotency is already durable via
   task dedup keys; persisted watermarks are a papdashboard-shaped need.

## 1. What the real code does today (the baseline being fixed)

`Bridge.Run` (`internal/bridge/papdashboard/papdashboard.go:116`) resolves its
starting cursor via `startWatermark` (`papdashboard.go:181-194`):

- `Config.FromSeq` set → that seq (replay mode; the only existing
  replay-from-seq consumer per the inventory doc §2 row 4).
- else → `HeadSeq()` (`sqlite.go:757`, O(1) `MAX(seq)`) — **forward-only from
  now**. The cursor then lives in a local variable for the process lifetime:
  `watermark = f.Seq` advances only after `forward` returns nil
  (`papdashboard.go:167`), i.e. after PapDashboard accepted the fact (2xx) or
  permanently rejected it (4xx — `post` logs and returns nil,
  `papdashboard.go:299-304`); 5xx/transport errors stop the drain and retry
  from the last forwarded seq on the next poll.
- `Bridge.alerted` (`papdashboard.go:78`) remembers the title of each alert
  raised this process so a later `Completed` fact resolves exactly that alert
  (`papdashboard.go:225-246`).

Consequence (package doc, `papdashboard.go:10-13`): restarts start at the
journal head — incidents fired while the bridge was down are never replayed
(the missed-incident gap), and completions that would resolve pre-restart
alerts are forwarded but silently dropped by the `alerted`-map miss.

Facts that constrain any persistence design:

- The journal is append-only with monotonic `INTEGER PRIMARY KEY
  AUTOINCREMENT` seq (`sqlite.go:119`); `Facts(after, limit)` is strictly
  greater (`sqlite.go:714`), so a persisted cursor replays exactly (inventory
  §3.1).
- There is **no public fact-append API**: facts appear only via the private
  `appendFact` inside queue-mutation transactions (`sqlite.go:166`) — the
  in-tx facts invariant (AGENTS.md).
- The store is a single serialized writer (`MaxOpenConns(1)`,
  `sqlite.go:64`): every checkpoint write shares that one connection with
  queue mutations.
- The bridge deliberately holds only the read-only `FactSource` slice
  (`Facts`/`HeadSeq`/`Get`, `papdashboard.go:32-36`) — "never mutates queue
  state" is the package's stated contract.
- The bridge usually lives inside `tq worker --alert-url` as a goroutine
  sharing the worker's store and signal context (`cmd/tq/main.go:295-309`);
  there is no dedicated bridge process today.

## 2. Storage shape: side table vs metadata fact

### 2.1 Option A — side table (recommended)

```sql
CREATE TABLE IF NOT EXISTS watermarks (
	consumer   TEXT PRIMARY KEY,  -- e.g. "papdashboard:<endpoint-hash>"
	seq        INTEGER NOT NULL,
	updated_at INTEGER NOT NULL   -- unix millis
);
```

plus on `queue.Store`:

```go
Watermark(ctx context.Context, consumer string) (int64, error)            // 0 when absent
SaveWatermark(ctx context.Context, consumer string, seq int64) error      // monotonic: never regresses
```

- **Projection purity (the 05:24 f9 criterion):** facts describe task
  lifecycle; a bridge's read progress is consumer state, the exact analogue
  of Kafka storing consumer offsets in `__consumer_offsets` instead of in the
  topic. The AGENTS.md invariant "a code change that mutates task state must
  append a fact in the same transaction" is untouched — a checkpoint mutates
  no task state.
- **O(1) everything:** PK read, PK upsert. No new index, no scan, no
  `MAX(...)`.
- **Migration-safe:** a brand-new table in the `schema` const
  (`sqlite.go:88`) is legacy-safe under the AGENTS.md migration rules — the
  "no indexes on new columns in the schema const" rule targets columns added
  by later `ALTER TABLE`; a fresh table's PK needs no dependent index.
- **The generic name earns its keep:** the same table serves any future
  resumable consumer (the SSE `Last-Event-ID` ↔ Seq mapping, 05:24 f40/f41)
  without a second mechanism.
- **Ops-friendly:** `tq watermarks show/set` (05:24 f41) is a trivial SELECT/
  UPDATE — the rescue hatch for a poisoned or migrated-forward checkpoint
  (rewind to force replay; the idempotency keys make replay safe).
- **Cost:** the bridge's write dependency must be injected. Do NOT widen
  `FactSource` (that would break the "bridge never mutates queue state"
  contract in spirit); instead compose a second, minimal port — e.g.
  `WatermarkStore { Watermark; SaveWatermark }` — which `SQLiteStore`
  already satisfies and tests fake in-process (the existing `fakeSource`
  pattern in `papdashboard_test.go:17` extends naturally). The package
  comment's "never mutates queue state" needs one honest line: the bridge
  now persists _its own_ cursor, still never task/fact state.

### 2.2 Option B — metadata fact (rejected)

Append a synthetic fact (e.g. type `bridge.checkpointed`, task_id
`"__bridge__"`, detail `{"consumer":"papdashboard","seq":1234}`) per
checkpoint.

- **Journal pollution:** the `facts` table's shape and every consumer's
  mental model are task-scoped (`task_id TEXT NOT NULL`, `sqlite.go:122`);
  bridge bookkeeping would render in `tq facts`, `tq tail`, the webui feed,
  and `FactsForTask` trails as noise.
- **Vocabulary churn:** a new `FactType` hits the `exhaustive`-linter
  switches in every consumer (the bridge's own switch,
  `papdashboard.go:199`, plus webui renders) and DOMAIN_LANGUAGE's
  "task.*" fact taxonomy.
- **Write-path invariant damage:** there is no public append today by
  design; adding one _only_ for checkpoints punches a hole in "facts appear
  only via queue mutations in-tx" (ADR-0001's seam) for a non-domain fact.
- **Worse reads:** finding the current checkpoint means `MAX(seq) WHERE
  type = ?` (needs a new index on the write-amplified column) instead of a
  PK lookup; interleaved writers make "the" latest checkpoint ambiguous
  where the side table's monotonic upsert is not.
- **Seq burn:** one journal row per checkpoint cadence, forever, in the one
  table that is deliberately append-only with no compaction path.

What Option B buys — checkpoint history in the audit trail — is better served
by the bridge's own log lines (`resumed from checkpoint N`), which are
already the diagnostic surface.

### 2.3 Decision

Side table (`watermarks`), consumer-keyed, with the two-method Store seam
above. Metadata fact documented as rejected for the four reasons in §2.2.

## 3. Ack semantics (when the persisted cursor advances)

The in-memory rule stays exactly as it is — advance past a fact only after
`forward` returns nil (2xx accepted **or** 4xx permanently rejected) — and
persistence adds one rule on top:

- **Checkpoint per drained batch, after the batch.** After the inner facts
  loop finishes a `Facts(watermark, forwardBatchLimit)` page with every fact
  forwarded (the same place the loop decides `len(facts) < limit` means
  drained, `papdashboard.go:170`), call `SaveWatermark(consumer,
  watermark)` once. Rationale: per-fact persistence would issue up to 500
  UPDATEs per poll on the single serialized writer connection shared with
  queue mutations — write amplification with no delivery benefit, because
  re-sends are idempotent (§4).
- **Never checkpoint ahead of acceptance.** Checkpoint-after-forward is what
  keeps the system at-least-once; checkpoint-before would silently convert it
  to at-most-once (facts lost on crash). The checkpoint write must be the
  LAST step of the batch, and its failure is handled like a forward failure:
  log, stop the drain, retry next poll with the in-memory cursor unchanged —
  never skip past an unpersistable checkpoint.
- **4xx counts as acked** (unchanged): a payload PapDashboard will never
  accept must not stall the cursor forever; it is logged and the checkpoint
  advances past it (`papdashboard.go:299-304` semantics carried over).
- **Shutdown mid-drain:** `Run` returns nil on ctx cancellation between
  facts (`papdashboard.go:134`, `:155`); facts forwarded since the last
  checkpoint are re-sent on the next start. Correct, bounded, deduped.
- **Monotonic guard:** `SaveWatermark` upserts with `WHERE seq < ?`
  semantics (or `max(seq, ?)`) so a lagging or misconfigured second process
  cannot drag the cursor backwards. Multi-process stance: two bridge
  processes sharing one DB and endpoint is tolerated exactly as today
  (duplicate sends, identical idempotency keys, PapDashboard dedupes);
  sharing one _consumer key_ means they interleave checkpoints — the
  monotonic guard keeps that safe-loss-free, and distinct endpoints should
  use distinct consumer keys (`papdashboard:<endpoint>`).

## 4. Idempotent resend window

- **Definition:** the set of facts re-forwarded after a restart =
  `(persisted seq, last in-memory seq]` — bounded by the checkpoint cadence:
  at most one drain batch (≤ 500 facts) plus whatever accumulated during
  downtime, each with its original key `go-taskqueue-{dlq|resolve}-<seq>`
  (`idempotencyKey`, `papdashboard.go:326`). The key is a pure function of
  the immutable fact seq, so re-sends are bit-identical requests.
- **Queue-side risk: none.** Forwarding is read-only against the store; facts
  are immutable history; replay can never re-execute or re-queue a task.
- **PapDashboard-side assumption (the honest caveat):** dedupe by
  Idempotency-Key must outlive the bridge's restart gap. If PapDashboard
  forgets keys (retention, DB reset) while the bridge replays its window,
  duplicates become duplicate alerts — low severity, operator-visible, and
  the failure mode of NOT persisting (silently missed incidents) is strictly
  worse.
- **Replay beyond the window** (ops-initiated, via `--from-seq` or
  `tq watermarks set`) rides the same keys: rewind is always safe on the
  queue side and deduped on the PapDashboard side.
- **Retention/compaction interplay (recorded, unreachable today):** if
  journal compaction ever lands (ROADMAP), a checkpoint older than the
  retention floor would silently resync to the oldest retained fact — same
  caveat class as the Subscribe contract (inventory §3.4). Define loud
  behavior (log a resync) when compaction becomes real; gap-detection via
  `first-returned-seq > checkpoint+1` is NOT reliable (AUTOINCREMENT gaps
  from rolled-back transactions are normal).

## 5. The second volatile half: alert-resolve correlation

Even with a persisted watermark, this fails: task dead-lettered and acked →
bridge restarts (checkpoint past the DL fact, so replay never re-delivers it
→ `alerted` map empty) → task rescued and completes → `Completed` fact hits
the `alerted` miss (`papdashboard.go:226-229`) → the alert stays open
forever. Today's head-start restart has the same hole; persistence makes it
the _only_ hole.

Options enumerated:

1. **Derive, don't store (recommended):** on `Completed`, ask the task's own
   history — `FactsForTask(id)` contains the `DeadLettered` fact if the task
   ever dead-lettered — and recompute the title via `alertTitle(t)`
   (deterministic from the task record, `papdashboard.go:312`). This turns
   `alerted` from mutable state into a projection over facts, which is the
   architecture's stated shape (ADR-0001). Contract change: "resolve only
   alerts this bridge raised" becomes "resolve the alert of any task that
   dead-lettered then completed" — safe because this sourceApp is the only
   writer of these alert titles, and a resolve for an alert PapDashboard
   never accepted lands as a logged permanent rejection (4xx path). Cost:
   widen `FactSource` with the read-only `FactsForTask` (a read, so the
   no-mutation contract holds).
2. **Persist the alerted set** in the same side table (consumer state grows
   a blob): rejected — it duplicates what the journal already answers, adds
   its own sync/retention problems (when is an entry removed?), and is a
   second source of truth for a derivable fact.
3. **Ask PapDashboard** (query open alerts on completion): rejected — new
   API dependency for something the local journal already knows, and couples
   bridge correctness to dashboard query semantics.

## 6. startWatermark grows one branch (bootstrap + precedence)

```
FromSeq (explicit) > persisted watermark > HeadSeq (first run)
```

- **FromSeq wins** (manual ops replay override) — and its result then
  checkpoints like any other cursor, so an explicit replay is also a
  one-shot "re-arm from here".
- **Persisted watermark wins over head** on restart — the entire point.
- **First run with no row: insert head**, preserving today's no-replay
  default. Persistence must NOT suddenly replay all history into existing
  deployments — bootstrap is forward-only, exactly like today.
- The startup log line should say which branch fired (`--from-seq override`
  / `resumed from checkpoint N` / `no checkpoint, starting at head N`) —
  that line is the operator's first diagnostic when alerts look wrong.

## 7. Implementation checklist (~~for the f10 migration task; nothing done here~~ all eight rows shipped 2026-09-08, ROUND6 P1–P5; see `docs/status/2026-09-08_20-58_round6-execution-watermarks-bus-actors.md` and the CHANGELOG `[Unreleased]` watermarks entry)

- [x] `watermarks` table in the schema const + `Watermark`/`SaveWatermark`
      on `queue.Store` (monotonic upsert) + store tests (absent→0, save,
      monotonic guard, legacy DB migrated).
- [x] `WatermarkStore` port injected into `papdashboard.New`; `FactSource`
      gains read-only `FactsForTask` (§5.1).
- [x] `startWatermark` three-branch resolution + startup log (§6).
- [x] Batch-end checkpoint in `Run`'s drain loop; checkpoint failure treated
      as a forward failure (§3).
- [x] Replace `alerted` map with `FactsForTask` derivation (§5.1); delete
      the map.
- [x] Tests (05:24 f15): restart-mid-stream loses zero facts; re-forwarded
      facts keep identical idempotency keys; checkpoint write failure does
      not advance the cursor; FromSeq precedence; first-run bootstrap = head;
      resolve-after-restart closes the alert that predated the restart.
- [x] `tq watermarks show/set` (05:24 f41) + AGENTS.md line: bridge
      checkpoints live in `watermarks`, bridge still never mutates task or
      fact state.
- [x] Package doc comment (`papdashboard.go:7-13`) rewritten: the
      across-restart story changes from "incidents while down are not
      replayed" to "resumed from checkpoint; re-sends are idempotent".

## Verification

- Every path/line number above was read from the working tree this session
  (HEAD `747ea0f`, tree clean): `papdashboard.go` (Run :116, watermark
  advance :167, alerted map :78/:223/:246, 4xx path :299-304, startWatermark
  :181-194, idempotencyKey :326), `sqlite.go` (schema :88-129, single writer
  :64, appendFact :166, Facts :711/:714, HeadSeq :757), `queue.go`
  (Store :58-74), `cqa/cqa.go` (no journal usage — §5 of the verdicts),
  `cmd/tq/main.go:295-309` (bridge wiring), DOMAIN_LANGUAGE.md (Tailer/
  watermark vocabulary).
- Gates: `go build ./...`, `go vet ./...`, `go test ./... -race` green
  (doc-only change).
