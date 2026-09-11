# SSE `Last-Event-ID` ↔ Journal Seq — Client-Resume Mapping

**Date:** 2026-09-08
**Status:** DONE (design spike: every code claim below was verified by reading
the cited code at HEAD this session, and the one behavioral question (filter
scope of the stream) was verified with a scratch test run against the real
handler, then removed; no production code was changed)
**Scope:** Fulfill the TODO_LIST "Medium Impact" item: design the SSE
`Last-Event-ID` ↔ journal `Seq` client-resume mapping — the webui twin of
bridge watermarks (05:24 report f40, finding text: "client-side twin of bridge
watermarks").
**Companion docs:**
`docs/planning/2026-09-08_journal-subscription-surface-inventory.md` (§2 row 3
and §4 gap 6 are the entries this spike answers) and
`docs/planning/archived/2026-09-08_persisted-bridge-watermark-design.md` (§2.1 already
anticipated this spike; §5 below mirrors its §6 bootstrap/persistence/garbage
answers with the client-side twist).

---

## Verdicts up front

1. **The mapping already exists on the wire and is correct.** Every snapshot's
   closing event carries `id: <journal seq>` (`hub.go:30-37` → `handlers.go:184-191`),
   so a reconnecting browser returns exactly a journal Seq in `Last-Event-ID`.
   What does not exist is any server-side _use_ of that returned Seq beyond a
   Debug log (`handlers.go:124-130`) — and for this dashboard that is mostly
   right (verdict 2). The missing uses are small: reconnect-lag observability
   and an id on the initial snapshot (§3).
2. **Per-fact replay — the bridge twin's exact-delivery semantics — is
   correctly absent and should stay absent.** The bridge is a fact deliverer;
   the dashboard is a projection consumer whose every event is a full
   re-render. Replaying `(N, head]` fact-by-fact would send k near-identical
   snapshots for zero fidelity gain; one snapshot at head IS the exact resume.
   The resume contract is: **snapshot at head, scoped to the client's view**
   (§2, §4).
3. **The real f40 gap is not freshness, it is scope.** `app.js` opens
   `/api/events` without the page's filter parameters (`app.js:37-39`), so
   live ticks _and_ reconnects push the **unfiltered** projection — a filtered
   page's table is clobbered on the first tick. Scratch-test-verified (§4).
   `Last-Event-ID` restores freshness; only the URL-carried query restores the
   view. Resume = (scope, freshness).
4. **No server-side persistence — that is the point of the twin.** A bridge
   watermark dies with its process and needs the `watermarks` side table; the
   SSE client's cursor lives in the browser and survives _server_ restarts by
   SSE design. Never write a browser cursor into the side table. The client
   twin's hard requirements are instead: garbage-tolerance (any unparseable or
   beyond-head id maps to fresh-connect, never an error) and a defined lag
   signal (`head − N` at reconnect) (§3, §5).
5. **ADR-0003's decision-2 sentence overclaims.** "SSE `Last-Event-ID` maps
   back to the journal watermark for resume (proven in `examples/sse`)" reads
   as delta replay; the implementation maps the id on the wire but resumes by
   snapshot (`handlers.go:98-104` is explicit). Amended in ADR-0003 this
   session; `examples/sse` remains the per-fact twin (id = Seq,
   header → `after`, `examples/sse/main.go:56-58`) — the PoC for the shape the
   dashboard deliberately does not use.
6. **The id↔seq mapping is the forward-compatible seam to Subscribe.** If the
   wire ever carries per-fact events (dispatcher/delta era, inventory §4 gaps
   1/6/7), the same ids feed `Facts(N+1, …)` replay with zero renumbering.
   Building Subscribe is NOT justified by the dashboard's resume needs (§6).

## 1. The wire mapping today (verified baseline)

The full path of one Seq, with every hop read at HEAD:

| Hop                        | Code                                   | Fact                                                                                                                                                                                         |
| -------------------------- | -------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| journal → tailer watermark | `tailer.go:34,43-48`                   | poll `Facts(watermark, 1000)`; watermark jumps to batch's last seq (change-signal semantics; middle facts of a >1000 burst are skipped — harmless here, see §2)                              |
| watermark → hub tick       | `hub.go:30-37`                         | `Notify(seq)` broadcasts a payload-less `tick` event with `ID = formatSeq(seq)` (`tailer.go:63-65`)                                                                                          |
| hub → all clients          | `hub.go:40-47`, `handlers.go:121-122`  | per-client buffer 128, drop on overflow (`hub.go:9-12`) — an in-connection drop is invisible to the client and healed by the next snapshot; `Last-Event-ID` says nothing about it, by design |
| client render              | `handlers.go:151-158` → `sendSnapshot` | the tick's seq is parsed back out of the event id and re-attached to the closing event of the snapshot                                                                                       |
| snapshot → browser cursor  | `handlers.go:184-191`                  | only the `title` event carries `id:`; browsers advance `lastEventId` on any event with an id field, so after a burst the cursor = the seq that caused it                                     |
| reconnect → server         | browser → `handlers.go:127`            | `Last-Event-ID: <seq>` (go-sse `stream.LastEventID()`, header-only, wire-unsafe values zeroed by `ParseEventID`; `go-sse@v0.6.0/stream.go:207,276`)                                          |
| server → action            | `handlers.go:124-130`                  | Debug log only; snapshot follows regardless (`TestResumeAfterFactsBacklog`, `webui_test.go:348-385`, already covers `""`, `"1"`, `"999999"`)                                                 |

Two load-bearing details:

- **The id rides the LAST event of each burst** (title, sent after the five
  fragments, `render.go:426-435` + `handlers.go:178-191`). A connection that
  dies mid-burst reconnects with the _previous_ snapshot's id — which the
  snapshot resume answers exactly. Keep the id on the burst's last event; do
  not add per-fragment ids (the client never renders fragments partially).
- **The initial snapshot carries no id** (`watermarkUnknown = -1`,
  `handlers.go:132,163-164`). A client that connects and drops before the
  first tick reconnects with no `Last-Event-ID` → snapshot. Correct, but it
  makes reconnect lag unmeasurable for quiet dashboards; §3 fixes that cheaply.

## 2. Why the bridge-twin semantics are rejected for the dashboard

| Question           | papdashboard bridge                                | webui SSE client                       |
| ------------------ | -------------------------------------------------- | -------------------------------------- |
| Consumer class     | fact deliverer (inventory §2 row 4)                | projection consumer (ADR-0003)         |
| Event payload      | one fact per event                                 | five fragments + title, full re-render |
| Missed events mean | missed incidents (the f9/f10 gap)                  | nothing — next snapshot subsumes them  |
| Cursor advance     | per fact, after acceptance                         | per snapshot burst (id = causing seq)  |
| Resume after gap   | replay `(N, head]` with idempotency keys           | one snapshot at head                   |
| Cursor store       | server process → dies → side table (companion doc) | browser → survives server restart      |

Replaying `(N, head]` on the dashboard would re-render the same projection
once per fact: k× the SQLite reads and templ renders, zero client-visible
difference. The projection model is what makes the snapshot resume _exact_,
not a compromise — the client holds no state that could drift (the only
client state is the DOM, fully replaced per burst; `app.js:23-32`).

The library even ships the delta machinery (`sse.Replay` + `EventStore`,
`go-sse@v0.6.0/replay.go`) and `examples/sse` uses the raw pattern
(`Last-Event-ID` → `after`, per-fact events, `examples/sse/main.go:52-58`).
That is the right shape for a fact feed, and the wrong shape for fragments.

**ADR-0003 correction (applied this session):** decision 2's sentence now
reads as snapshot resume with the id↔seq mapping as a freshness token; the
old "(proven in `examples/sse`)" clause conflated the PoC's per-fact replay
with the dashboard's snapshot resume. See the amendment at the end of
`docs/adr/0003-web-ui-architecture.md`.

## 3. The resume contract (what to implement)

The client cursor is a _freshness token_. State machine, exhaustively:

1. **Connect, no `Last-Event-ID`** → snapshot at head (unchanged today).
   _Change:_ attach `id = HeadSeq()` (O(1), `sqlite.go:757`) to the initial
   snapshot's closing event instead of `watermarkUnknown`, so a client that
   drops during a quiet period still reports a cursor. Caveat: the render
   happens after the `HeadSeq` read, so the id can lag the rendered content
   by at most one burst — harmless for a freshness token, corrected by the
   next tick.
2. **Reconnect, `Last-Event-ID = N`, numeric, N ≤ head** → snapshot
   (unchanged). _Change:_ replace the Debug log with an observability line —
   `slog.Info("webui: sse resume", "last_seq", N, "head", head, "behind", head-N)`
   when `behind > 0`, Debug otherwise. This is the twin of the bridge's
   "resumed from checkpoint N" startup line (companion doc §6): the operator
   answer to "was my dashboard blind while I slept?".
3. **Reconnect, N > head** (journal replaced/recreated, or a foreign client)
   → treat as case 1, Debug log. Never error, never clamp: there is nothing
   to resume from.
4. **Reconnect, non-numeric** (only possible from a foreign client — this
   server only ever emits `strconv` ids) → case 1. go-sse already zeroes
   newline-corrupt ids before they reach the handler.

Auth interplay: `?token=` rides the reconnect URL automatically (EventSource
re-requests the same URL, header added by the browser; `app.js:34-39` exists
because EventSource cannot set headers). Any future query params must MERGE
with it, not overwrite it.

## 4. The real gap: resume ignores view scope (verified bug)

`handleEvents` renders every snapshot under the _request's_ filter
(`parseFilter(r)`, `handlers.go:17-33,132,167`). The browser's stream request
is built by `app.js:37-39` — `/api/events` plus only the `token` param — so
the filter the user applied (a full-page GET navigation:
`FilterBar`'s `<form method="get" action="/">`, `fragments.templ:236`) never
reaches the stream. Scratch test this session (against the real handler, then
removed): page `/?project=alpha` renders only `alpha`; the `/api/events`
snapshot carries `alpha` AND `beta`. A user on any filtered (or paginated)
view has it replaced by the unfiltered page-1 table on the next tick —
reconnect/resume fidelity is moot while the live path is already wrong.

Fix (queued as its own TODO_LIST item; not done in this spike): forward the
page's filter query (`project`, `status`, `q`, `page`) from
`window.location.search` to the EventSource URL, merged with `token`. Server
needs zero changes — `parseFilter` already reads them; reconnects then restore
the same scope automatically (the browser reuses the URL). This also makes
`TestFiltersNarrowTable`-style assertions possible against the stream.

## 5. Same three questions, opposite answers (mirror of the watermark doc)

The companion watermark doc had to answer bootstrap, persistence, and garbage
cursors for a server-side consumer. The client twin answers the same three
with opposite mechanics — which is why it must NOT reuse the `watermarks`
side table:

| Question              | Bridge (companion doc)             | SSE client (here)                                                            |
| --------------------- | ---------------------------------- | ---------------------------------------------------------------------------- |
| Bootstrap (no cursor) | start at head, checkpoint forward  | snapshot; cursor = HeadSeq on the closing event                              |
| Persistence           | side table (process dies)          | the browser (connection dies, cursor survives; server restart loses nothing) |
| Garbage/stale cursor  | monotonic upsert guards regression | map to fresh-connect; lag = max(0, head − N)                                 |

Writing browser cursors server-side would add a row per tab, a retention
question, and a write path into a read-only package (ADR-0003 guardrail,
`routeBindings` + `TestRoutesAreReadOnly`) for information the browser already
keeps.

## 6. What this means for Subscribe (inventory §4 gap 6, corrected)

The inventory said: "SSE client resume is snapshot-based … Subscribe's
per-connection `since` is what would make the f40 spike implementable."
Half right, now testable against this spike: f40 is implementable — and
implemented — at snapshot fidelity _without_ Subscribe, because snapshot
resume is not a degraded form of replay for a projection consumer; it is the
exact form (§2). Per-connection `since` replay becomes valuable only if the
wire ever carries events that are NOT full projections (per-fact feed,
delta fragments, filtered streams — inventory gaps 1/6/7). When that lands,
the seam is already in place: ids are journal Seqs today, so
`Facts(N+1, …)` needs no ID translation layer. Decision: do not build
Subscribe for the dashboard's sake.

## 7. Implementation checklist (follow-ups; none done in this spike)

- [ ] `app.js`: forward `project/status/q/page` from the page URL to the
      EventSource URL, merged with `token` (the §4 bug — tracked as its own
      TODO_LIST item).
- [ ] `handleEvents`: reconnect-lag log line per §3.2; `N > head` and
      non-numeric → fresh-connect handling per §3.3/3.4 (defensive
      `strconv.ParseInt` on the header value).
- [ ] `sendSnapshot`: id the initial snapshot with `HeadSeq()` per §3.1
      (replaces `watermarkUnknown`).
- [ ] Tests: extend `TestResumeAfterFactsBacklog` shapes with a
      beyond-head and non-numeric id asserting snapshot-not-error; add a
      filtered-stream test once §4 lands.
- [ ] Optional polish, not blocking: expose per-client lag
      (`head − last sent seq`) in the request log line for SSE streams.

## Verification

- Files read at HEAD `4a99ee9` (plus this doc's own commits): `handlers.go`
  (parseFilter :17-33, handleEvents :98-161, watermarkUnknown :163-164,
  sendSnapshot :166-194), `hub.go` (:9-12 buffer, :30-37 Notify),
  `tailer.go` (:11-17 contract, :34/:43-48 loop, :63-65 formatSeq),
  `render.go` (:426-435 renderFragments), `app.js` (:23-40),
  `fragments.templ` (FilterBar form :236-267), `webui_test.go`
  (:348-385 TestResumeAfterFactsBacklog, :288-308 TestFiltersNarrowTable),
  `examples/sse/main.go` (:52-58), `sqlite.go` (:757 HeadSeq),
  ADR-0003 (:35), inventory doc (§2 row 3, §4 gap 6), watermark doc (§2.1,
  §5, §6), `go-sse@v0.6.0` (`stream.go:207/:276`, `replay.go:7-63`,
  `ssetest` WithLastEventID).
- The §4 filter-clobber claim was verified by running a scratch test against
  `srv.Handler()` (page `/?project=alpha` narrowed; `/api/events` snapshot
  unfiltered), then deleted — not committed.
- Gates after the doc + TODO_LIST + ADR amendment changes: `go build ./...`,
  `go vet ./...`, `go test ./... -race`, `gofmt -l .` (doc-only change).
