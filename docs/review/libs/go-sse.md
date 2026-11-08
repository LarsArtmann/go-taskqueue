# go-sse — Library Utilization Review

**Verdict:** ADOPTED — very high (9.5/10 capability rows leveraged)
**Pinned:** `go-sse v0.7.0` (direct, root `go.mod:112`); `go-sse/ssetest v0.5.0` (direct, root `go.mod:113`); `go-sse/sseparse v0.2.1` (indirect, root `go.mod:60`). 11 nested-module go.mods carry `v0.6.2 // indirect` via the go-cqrs-lite chain (cmd/tq, examples/embed, internal/{composition,readmodel,worker}, internal/queue/{sqlite,sqlitev4,postgres,postgresv4}, queue/{sqlite,postgres}).
**Latest:** v0.7.0 (local checkout /home/lars/projects/go-sse, checked 2026-11-08; HEAD is `v0.7.0-1-g443abdd`)
**Prior coverage:** none (the 2026-09-14 templ-components deep-dive audits the UI library; its SSE notes cover app.js swap mechanics, not the go-sse library)
**Reviewed:** 2026-11-08 by `grep -rn 'go-sse' --include=go.mod . | grep -v vendor`, `grep -rhoE 'sse\.[A-Z][a-zA-Z]*' --include='*.go' internal`, `grep -rhoE 'ssetest\.[A-Z][a-zA-Z]*' internal/webui/*_test.go | sort | uniq -c`, `GOFLAGS=-mod=mod go list -m all | grep go-sse`, `cd cmd/tq && GOWORK=off go mod tidy -diff | grep larsartmann/go-sse`, full reads of /home/lars/projects/go-sse FEATURES.md + fanout.go + stream.go + CHANGELOG.md, `go test ./internal/webui -run 'TestSSEFullSnapshotOnConnect|TestResumeAfterFactsBacklog' -count=1` (ok 0.149s)

## 1. Usage inventory

Files: `internal/webui/hub.go`, `internal/webui/handlers.go`, `internal/webui/agentfeed.go`; tests `internal/webui/{auth,board,evidence_stream,webui}_test.go` (verified 2026-11-08 by `grep -rl --include='*.go' 'larsartmann/go-sse' . | grep -v '^./vendor/'`).

Library symbols in repo code (verified 2026-11-08 by `grep -rhoE 'sse\.[A-Z][a-zA-Z]*' --include='*.go' internal | sort | uniq -c | sort -rn`): `sse.Stream` ×7, `sse.Event` ×7, `sse.NewEventID` ×3, `sse.WithBufferSize` ×2, `sse.NewStream` ×2, `sse.Broadcaster` ×2, `sse.NewBroadcaster` ×2, `sse.LastEventIDFromRequest` ×1. ssetest in tests: `WithPath` ×14, `CollectN` ×8, `CollectWithTimeout` ×6, `Event` ×2, `WithLastEventID` ×1, `RequestOption` ×1.

Two broadcasters: `Hub` fans payload-less `tick` events whose ID is the journal watermark (hub.go:15-38); `AgentFeed` fans typed `AgentEvent` payloads (agentfeed.go:25-38). Endpoints: `/api/events` + per-task stream share `runEventStream` (subscribe → snapshot → tick-pump, handlers.go:348-416); the agent live feed runs its own loop (handlers.go:270-310). Heartbeats ride `stream.Heartbeat` behind `startHeartbeat` (handlers.go:474-493). Buffer 128 with a documented drop-then-self-heal policy (hub.go:10-13).

| Surface | File:lines | Lib symbols |
| --- | --- | --- |
| Hub (payload-less ticks, watermark IDs) | internal/webui/hub.go:20-56 | `Broadcaster[sse.Event]`, `WithBufferSize`, `NewEventID`, `Shutdown` |
| AgentFeed (typed payload fan-out) | internal/webui/agentfeed.go:35-65 | `Broadcaster[AgentEvent]`, `WithBufferSize`, `Shutdown` |
| Dashboard SSE protocol | internal/webui/handlers.go:348-416 | `NewStream`, `Stream.Send`, `Stream.SendJSON`, `LastEventIDFromRequest` |
| Agent live feed | internal/webui/handlers.go:270-310 | `NewStream`, `Stream.SendJSON("agent")` |
| Heartbeat wrapper | internal/webui/handlers.go:474-493 | `Stream.Heartbeat` |
| Snapshot burst | internal/webui/handlers.go:509-530 | `Stream.SendJSON("frag")`, `Event.ID`, `Stream.Send` |
| Handler tests | internal/webui/webui_test.go:208-233,440-470 | `ssetest.CollectN`, `CollectWithTimeout`, `WithPath`, `WithLastEventID` |

## 2. Capability inventory

| Capability | Status | Evidence |
| --- | --- | --- |
| Wire-format serialization (`Event`/`WriteEvent`/heartbeats) | 🟢 FULL | all emission via `stream.Send`/`stream.SendJSON` — zero hand-rolled `data:` lines (handlers.go:517-527; lib writer alloc-minimized per FEATURES.md) |
| Stream lifecycle (`NewStream`, headers, `Close`, context) | 🟢 FULL | `NewStream` + deferred `Close` at both endpoints, `Flusher` probe + `X-Accel-Buffering: no` first (handlers.go:278-289, 363-374); `r.Context()` used where lib offers equivalent `stream.Context()` (stream.go:192) |
| Heartbeat keepalive | 🟢 FULL | `stream.Heartbeat(hbCtx, s.cfg.Heartbeat)` wrapped in cancel+wait (handlers.go:474-493); the lib stops on ctx/closed/write-error (stream.go:246-272) but offers no wait contract, so tq's `<-done` guard is deliberate defensive code, not a misuse |
| `Last-Event-ID` extraction | 🟢 FULL | `sse.LastEventIDFromRequest(r)` + branded `EventID`, used for reconnect-lag logging (handlers.go:324-338) |
| Branded `EventID` type safety (`NewEventID`/`ParseEventID`/`MustParseEventID`) | 🟢 FULL | watermark IDs constructed at the boundary via `sse.NewEventID(formatSeq(seq))` (hub.go:33-38, handlers.go:522-525); untrusted header values validated inside the lib (rejects `\n`/`\r` per FEATURES.md) |
| Reconnection replay (`EventStore`/`Replay`/`ReplayFiltered`) | ⚪ N/A | snapshot-resume by design: ticks are payload-less, so "any Last-Event-ID is satisfied by a fresh snapshot" (handlers.go:349-356, 457-461); replaying ticks would re-send what the snapshot already covers |
| Fan-out `Broadcaster[T]` (Subscribe/Unsubscribe/Broadcast/Shutdown, `WithBufferSize`, drop policy) | 🟢 FULL | two broadcasters with explicit buffer + graceful drain (hub.go:20-56, agentfeed.go:35-65); agent feed is the generic-type showcase (`Broadcaster[AgentEvent]`) |
| Predicate subscription (`SubscribeFilter`) | ⚪ N/A | single-topic fan-out — every dashboard client receives every tick by design; no per-client topics exist |
| Broadcaster observability (`Health()`, `OnSubscribe`/`OnUnsubscribe` hooks, `BroadcastMany`) | 🔴 MISSED | `BroadcasterHealth{SubscriberCount,BufferSize,Closed,Draining}` (fanout.go:438) never surfaced; `/health` reports store-derived checks only (health.go:60-190) |
| DataStar-pattern helpers (`SendKeyed`/`SendLines`/`KeyedLines`) | ⚪ N/A | tq payloads are JSON fragments via `SendJSON` ("frag" handlers.go:517 / "agent" handlers.go:308); Datastar-keyed emission belongs to go-health-dashboard's `/health/sse` |
| Retry directive (`Event.Retry`/`WriteRetry`) | ⚪ N/A | browser default reconnect cadence is fine; no custom retry hint needed on a loopback dashboard |
| `SendJSON` convenience | 🟢 FULL | fragment and agent payloads marshal-and-send in one call (handlers.go:517 "frag", handlers.go:308 "agent") — marshal-error and nil-value paths owned by the lib (FEATURES.md Stream table) |
| `OnDisconnect` callbacks | ⚪ N/A | subscriber cleanup rides channel close + request context (handlers.go:293-294, 376); no per-disconnect work exists to hook |
| ssetest collect helpers | 🟢 FULL | `CollectN`/`CollectWithTimeout` across 4 test files (webui_test.go:208,268,321,457; auth_test.go:134-135; board_test.go:225; evidence_stream_test.go:73) |
| ssetest request options | 🟢 FULL | `WithPath` ×14 + `WithLastEventID` with the v0.5.0 branded-`EventID` breaking change already adopted (webui_test.go:454-457) |
| ssetest assertions (`Require*`, `RequireDataJSON[T]`, `FilterByType`) | 🟡 PARTIAL | tests hand-roll decode loops (webui_test.go:213-233: `switch evt.Type` + `json.Unmarshal`) where `RequireDataJSON`/`FilterByType` would compress; `Collect`/`CollectPost`/`CollectWithRequest`/`MustRead*` unused |
| sseparse module (standalone parser/corpus) | ⚪ N/A | tq never parses SSE off-HTTP (verified 0 non-test `sseparse.` sites); the browser parses the wire; tests parse via ssetest's re-export |

## 3. Findings

1. **Broadcaster health is invisible.** Offer: `Broadcaster.Health()` returns `BroadcasterHealth{Closed, Draining, SubscriberCount, BufferSize}` (checkout fanout.go:438, struct at fanout.go `type BroadcasterHealth`); `OnSubscribe`/`OnUnsubscribe` hooks exist for logging. We do instead: `/health` checks derive only from the store projection (internal/webui/health.go:60-190); subscriber count and buffer state of the two broadcasters are unobservable (hub.go:20-56, agentfeed.go:35-65). Impact: an operator cannot see how many dashboards stream, or that slow clients are dropping (the documented self-heal, hub.go:10-13, is silent). Example:
   ```go
   func (h *Hub) Health() sse.BroadcasterHealth { return h.bc.Health() }
   // wire into queueProber's check map: {"sse-hub", h.Health().SubscriberCount, ...}
   ```
2. **ssetest assertions left on the table.** Offer: `ssetest.RequireDataJSON[T]`, `RequireEventCount`, `RequireEventType`, `FilterByType` (checkout ssetest/compat.go:97-145). We do instead: hand-rolled switch + `json.Unmarshal` per event (internal/webui/webui_test.go:213-233). Impact: weaker failure messages (a bad fragment fails deep in an unmarshal error instead of a named wire diff) and ~20 lines of per-test plumbing. Example:
   ```go
   frag := ssetest.RequireDataJSON[fragment](t, evt, fragment{}) // named diff on mismatch
   ```
3. **v0.6.2 pin residue in 11 nested go.mods.** The indirect `v0.6.2` lines ride go-cqrs-lite (verified 2026-11-08 by `cd internal/queue/sqlite && GOWORK=off go mod graph | grep go-sse` → `go-cqrs-lite/metaengine/v4@v4.20.0 go-sse@v0.6.2`); cmd/tq additionally tidy-drifts (`cmd/tq/go.mod:82` says v0.6.2, `GOWORK=off go mod tidy -diff` raises it to v0.7.0). No runtime effect — those modules never import go-sse code, and the root graph selects v0.7.0 (verified by `GOFLAGS=-mod=mod go list -m all | grep go-sse`) — but a fleet tidy sweep (`internal/depsweep`'s own domain) keeps `check-gomod-vendor-sync.sh` quiet and the pin story honest.

## 4. Score + top opportunities

N=16 rows; N/A=6; A=10; FULL=9, PARTIAL=1, MISSED=1 → Σ=9.5 → score = round(100×9.5/10) = **95 — very high**. The score rose with the row granularity: the initial 13-row pass folded `SendJSON`, `EventID` branding, and the two ssetest halves into single rows, undercounting real FULL adoptions. Top: (1) surface `Broadcaster.Health()` on `/health`; (2) adopt the ssetest `Require*` assertions; (3) tidy the v0.6.2 residue.

## 5. Version currency + risk

Root pins are current: v0.7.0 / sseparse v0.2.1 / ssetest v0.5.0 all equal latest (verified 2026-11-08 by `git -C /home/lars/projects/go-sse describe --tags` and CHANGELOG.md; HEAD is one gate-fix commit past the tag, fanout.go unchanged). The v0.6.2→v0.7.0 delta contains no wire change — the only breaking edge is ssetest's `WithLastEventID(sse.EventID)`, already adopted (webui_test.go:454). `DefaultSubscriberBuffer` (64, exported in v0.7.0, fanout.go:19) is relevant if `subscriberBufferSize` is ever retuned. Risk: LOW — same-author library, 100% statement coverage claimed in FEATURES.md (measured upstream 2026-10-11), pin residue is hygiene-only.

## 6. Conclusion

go-sse is the load-bearing transport of the webui and is used well: the broadcaster/fan-out model, branded event IDs as journal watermarks, heartbeats, and the test collectors are all idiomatic, and the snapshot-resume design deliberately (and correctly) skips `Replay`. The gap is observability and test ergonomics, not transport correctness — both are small, low-risk adoptions.

## How to verify

```bash
grep -rn 'go-sse' --include=go.mod . | grep -v vendor   # pins incl. the 11 v0.6.2 indirect lines
grep -rhoE 'sse\.[A-Z][a-zA-Z]*' --include='*.go' internal | grep -v '^vendor' | sort | uniq -c | sort -rn
grep -rhoE 'ssetest\.[A-Z][a-zA-Z]*' internal/webui/*_test.go | sort | uniq -c | sort -rn
GOFLAGS=-mod=mod go list -m all | grep 'go-sse'          # effective: v0.7.0 / v0.2.1 / v0.5.0
cd cmd/tq && GOWORK=off go mod tidy -diff 2>/dev/null | grep 'go-sse'   # 0.6.2 → 0.7.0 drift
cd /home/lars/projects/go-sse && git describe --tags     # v0.7.0-1-g443abdd
grep -n 'type BroadcasterHealth' -A18 /home/lars/projects/go-sse/fanout.go
go test ./internal/webui -run 'TestSSEFullSnapshotOnConnect|TestResumeAfterFactsBacklog' -count=1
```
All commands above were run literally on 2026-11-08 and produced the cited outputs (test run: ok 0.149s).
