# go-sse — Library Utilization Review

> **2026-11-11 refresh:** re-verified every pin, tag, symbol count, and file:line cite against the
> tree at HEAD `60c19070`. Two of the three 2026-11-08 findings landed on **2026-11-09**:
> Finding 1 resolved (commit `33689678` — `Broadcaster.Health()` surfaced on `/health` via
> `prober.liveSSE`, pinned by `TestHealthLiveSSECheck`) and Finding 3 resolved (commit `70284764`
> — the v0.6.2 residue in the 11 nested go.mods re-tidied to v0.7.0). Finding 2 improved, not
> closed (`FilterByType`/`MustReadEvents`/`EventsString` adopted; the Require* gap closed 2026-11-12 — see the assertions row).
> **Denominator correction:** the 2026-11-08 edition printed N=16/A=10, but its own capability
> table has 17 rows (9 FULL + 1 PARTIAL + 1 MISSED + 6 N/A) — A=11. With Finding 1 resolved the
> honest arithmetic is Σ=10.5 → round(100×10.5/11) = **95 — very high** (the 95 stands, now on
> inputs that reconcile). Guard tests run at a throwaway HEAD worktree (working tree unbuildable —
> in-flight sqlite v4→v5 migration, unrelated to webui): ok 0.154s.

**Verdict:** ADOPTED — very high (10.5/11 capability rows leveraged; re-scored 2026-11-11)
**Pinned:** `go-sse v0.7.0` (direct, root `go.mod:112`); `go-sse/ssetest v0.5.0` (direct, root `go.mod:113`); `go-sse/sseparse v0.2.1` (indirect, root `go.mod:59`). All 11 nested-module go.mods carry `v0.7.0 // indirect` (the 2026-11-08 v0.6.2 residue was re-tidied away on 2026-11-09, commit `70284764`; cmd/tq/go.mod:82 included; `GOWORK=off go mod tidy -diff` clean — verified 2026-11-11).
**Latest:** v0.7.0 (local checkout /home/lars/projects/go-sse, checked 2026-11-11; tag 2026-11-04; HEAD is `v0.7.0-2-gd79edb5` 2026-11-10 — two commits past the tag)
**Prior coverage:** none (the 2026-09-14 templ-components deep-dive audits the UI library; its SSE notes cover app.js swap mechanics, not the go-sse library)
**Reviewed:** 2026-11-08 by `grep -rn 'go-sse' --include=go.mod . | grep -v vendor`, `grep -rhoE 'sse\.[A-Z][a-zA-Z]*' --include='*.go' internal`, `grep -rhoE 'ssetest\.[A-Z][a-zA-Z]*' internal/webui/*_test.go | sort | uniq -c`, `GOFLAGS=-mod=mod go list -m all | grep go-sse`, `cd cmd/tq && GOWORK=off go mod tidy -diff | grep larsartmann/go-sse`, full reads of /home/lars/projects/go-sse FEATURES.md + fanout.go + stream.go + CHANGELOG.md, `go test ./internal/webui -run 'TestSSEFullSnapshotOnConnect|TestResumeAfterFactsBacklog' -count=1` (ok 0.149s). **Re-verified 2026-11-11** by the same greps against the current tree, checkout tag/HEAD checks, fresh line-number sweeps of hub.go/agentfeed.go/handlers.go/health.go/webui.go, and the two guard tests at HEAD `60c19070` (ok 0.154s; plus `TestHealthLiveSSECheck` ok 0.007s)

## 1. Usage inventory

Files: `internal/webui/hub.go`, `internal/webui/handlers.go`, `internal/webui/agentfeed.go`, and since 2026-11-09 `internal/webui/health.go` + `internal/webui/webui.go` (the broadcaster-health wiring); tests `internal/webui/{auth,board,evidence_stream,health,webui}_test.go` (verified 2026-11-11 by `grep -rl --include='*.go' 'larsartmann/go-sse' . | grep -v '^./vendor/'` — 10 files).

Library symbols in repo code (verified 2026-11-11 by `grep -rhoE 'sse\.[A-Z][a-zA-Z]*' --include='*.go' internal | sort | uniq -c | sort -rn`): `sse.Stream` ×7, `sse.Event` ×7, `sse.BroadcasterHealth` ×6 (NEW — the Finding 1 adoption), `sse.NewEventID` ×3, `sse.WithBufferSize` ×2, `sse.NewStream` ×2, `sse.Broadcaster` ×2, `sse.NewBroadcaster` ×2, `sse.LastEventIDFromRequest` ×1. ssetest in tests: `WithPath` ×14, `CollectN` ×8, `CollectWithTimeout` ×6, `Event` ×2, `WithLastEventID` ×1, `RequestOption` ×1, plus the Finding 2 additions `MustReadEvents` ×1, `FilterByType` ×1, `EventsString` ×1.

Two broadcasters: `Hub` fans payload-less `tick` events whose ID is the journal watermark (hub.go:15-38); `AgentFeed` fans typed `AgentEvent` payloads (agentfeed.go:25-40). Endpoints: `/api/events` + per-task stream share `runEventStream` (subscribe → snapshot → tick-pump, handlers.go:363-430); the agent live feed runs its own loop (handlers.go:288-328). Heartbeats ride `stream.Heartbeat` behind `startHeartbeat` (handlers.go:489-507). Buffer 128 with a documented drop-then-self-heal policy (hub.go:10-13). Since 2026-11-09 the hub's lifecycle is observable: `prober.liveSSE = func() *sse.BroadcasterHealth { snap := s.hub.bc.Health(); return &snap }` (webui.go:255-260) feeds subscriber/buffer state into the `/health` JSON (health.go:264-277).

| Surface | File:lines | Lib symbols |
| --- | --- | --- |
| Hub (payload-less ticks, watermark IDs) | internal/webui/hub.go:20-57 | `Broadcaster[sse.Event]`, `WithBufferSize`, `NewEventID`, `Shutdown` |
| AgentFeed (typed payload fan-out) | internal/webui/agentfeed.go:35-68 | `Broadcaster[AgentEvent]`, `WithBufferSize`, `Shutdown` |
| Dashboard SSE protocol | internal/webui/handlers.go:363-430 | `NewStream`, `Stream.Send`, `Stream.SendJSON`, `LastEventIDFromRequest` |
| Agent live feed | internal/webui/handlers.go:288-328 | `NewStream`, `Stream.SendJSON("agent")` |
| Heartbeat wrapper | internal/webui/handlers.go:489-507 | `Stream.Heartbeat` |
| Snapshot burst | internal/webui/handlers.go:521-547 | `Stream.SendJSON("frag")`, `Event.ID`, `Stream.Send` |
| Broadcaster health on `/health` | internal/webui/webui.go:255-260 + health.go:264-277 | `Broadcaster.Health()`, `BroadcasterHealth` |
| Handler tests | internal/webui/webui_test.go:208-233,470-490 | `ssetest.CollectN`, `CollectWithTimeout`, `WithPath`, `WithLastEventID` |

## 2. Capability inventory

| Capability | Status | Evidence |
| --- | --- | --- |
| Wire-format serialization (`Event`/`WriteEvent`/heartbeats) | 🟢 FULL | all emission via `stream.Send`/`stream.SendJSON` — zero hand-rolled `data:` lines (handlers.go:521-547; lib writer alloc-minimized per FEATURES.md) |
| Stream lifecycle (`NewStream`, headers, `Close`, context) | 🟢 FULL | `NewStream` + deferred `Close` at both endpoints, `Flusher` probe + `X-Accel-Buffering: no` first (handlers.go:295-303, 378-387); `r.Context()` used where lib offers equivalent `stream.Context()` (stream.go:192) |
| Heartbeat keepalive | 🟢 FULL | `stream.Heartbeat(hbCtx, s.cfg.Heartbeat)` wrapped in cancel+wait (handlers.go:489-507); the lib stops on ctx/closed/write-error (stream.go:246-272) but offers no wait contract, so tq's `<-done` guard is deliberate defensive code, not a misuse |
| `Last-Event-ID` extraction | 🟢 FULL | `sse.LastEventIDFromRequest(r)` + branded `EventID`, used for reconnect-lag logging (handlers.go:339-353) |
| Branded `EventID` type safety (`NewEventID`/`ParseEventID`/`MustParseEventID`) | 🟢 FULL | watermark IDs constructed at the boundary via `sse.NewEventID(formatSeq(seq))` (hub.go:33-38, handlers.go:537-540); untrusted header values validated inside the lib (rejects `\n`/`\r` per FEATURES.md) |
| Reconnection replay (`EventStore`/`Replay`/`ReplayFiltered`) | ⚪ N/A | snapshot-resume by design: ticks are payload-less, so "any Last-Event-ID is satisfied by a fresh snapshot" (handlers.go:334-353, 472-476); replaying ticks would re-send what the snapshot already covers |
| Fan-out `Broadcaster[T]` (Subscribe/Unsubscribe/Broadcast/Shutdown, `WithBufferSize`, drop policy) | 🟢 FULL | two broadcasters with explicit buffer + graceful drain (hub.go:20-57, agentfeed.go:35-68); agent feed is the generic-type showcase (`Broadcaster[AgentEvent]`) |
| Predicate subscription (`SubscribeFilter`) | ⚪ N/A | single-topic fan-out — every dashboard client receives every tick by design; no per-client topics exist |
| Broadcaster observability (`Health()`, `OnSubscribe`/`OnUnsubscribe` hooks, `BroadcastMany`) | 🟢 FULL | `BroadcasterHealth` surfaced on `/health` since 2026-11-09 (commit `33689678`): `prober.liveSSE` snapshots `h.bc.Health()` (webui.go:255-260) and the prober renders "N live subscriber(s), buffer M event(s)" into the check map (health.go:264-277); a closed hub fails the live check (pinned by `TestHealthLiveSSECheck`, health_test.go:429-446); the `OnSubscribe`/`OnUnsubscribe` hooks and `BroadcastMany` remain unused — no operator need |
| DataStar-pattern helpers (`SendKeyed`/`SendLines`/`KeyedLines`) | ⚪ N/A | tq payloads are JSON fragments via `SendJSON` ("frag" handlers.go:532 / "agent" handlers.go:323); Datastar-keyed emission belongs to go-health-dashboard's `/health/sse` |
| Retry directive (`Event.Retry`/`WriteRetry`) | ⚪ N/A | browser default reconnect cadence is fine; no custom retry hint needed on a loopback dashboard |
| `SendJSON` convenience | 🟢 FULL | fragment and agent payloads marshal-and-send in one call (handlers.go:532 "frag", handlers.go:323 "agent") — marshal-error and nil-value paths owned by the lib (FEATURES.md Stream table) |
| `OnDisconnect` callbacks | ⚪ N/A | subscriber cleanup rides channel close + request context (handlers.go:308-309, 391-392); no per-disconnect work exists to hook |
| ssetest collect helpers | 🟢 FULL | `CollectN`/`CollectWithTimeout` across 4 test files (webui_test.go:208,268,321,477; auth_test.go:134-135; board_test.go:225; evidence_stream_test.go:73) |
| ssetest request options | 🟢 FULL | `WithPath` ×14 + `WithLastEventID` with the v0.5.0 branded-`EventID` breaking change already adopted (webui_test.go:474) |
| ssetest assertions (`Require*`, `RequireDataJSON[T]`, `FilterByType`) | 🟢 FULL | closed 2026-11-12 (pareto T12): every fragment decode rides the one `fragmentEvents` helper (webui_test.go, `FilterByType` + single decode, malformed payload fails at the decode site) — the four hand-rolled `switch evt.Type`+`json.Unmarshal` loops (214-233/331/484/1671) and the three raw substring scans are gone; `EventsString` now carries the frag-count failure detail; `RequireDataJSON[T]` itself stays unused BY SHAPE — it exact-compares a whole decoded value and tq's fragment payloads carry variable HTML, so decode-and-inspect is the only consumer shape; `Collect`/`CollectPost`/`CollectWithRequest` unused (CollectN + CollectWithTimeout cover every need) |
| sseparse module (standalone parser/corpus) | ⚪ N/A | tq never parses SSE off-HTTP (verified 0 non-test `sseparse.` sites); the browser parses the wire; tests parse via ssetest's re-export |

## 3. Findings

1. **~~Broadcaster health is invisible~~ RESOLVED 2026-11-09 (commit `33689678`, fleet review f18).** The offered wiring exists nearly verbatim: `prober.liveSSE = func() *sse.BroadcasterHealth { snap := s.hub.bc.Health(); return &snap }` (internal/webui/webui.go:255-260), the prober field (health.go:87-91), and the check-map rendering "`%d live subscriber(s), buffer %d event(s)`" (health.go:264-277) — an operator now sees how many dashboards stream and that the hub is alive (a closed hub FAILS the live check while the process looks healthy). Pinned by `TestHealthLiveSSECheck` (health_test.go:429-446, ok 0.007s at HEAD). The lib's `OnSubscribe`/`OnUnsubscribe` hooks and `BroadcastMany` remain unused — nothing needs them on a two-broadcasters loopback surface.
2. **ssetest assertions still partly on the table (improved 2026-11-09).** The reconnect test now uses `ssetest.MustReadEvents` + `FilterByType` + `EventsString` (internal/webui/webui_test.go:396-409). Offer still open: `ssetest.RequireDataJSON[T]`, `RequireEventCount`, `RequireEventType` (checkout ssetest/compat.go:143-145+). Remaining hand-rolled switch + `json.Unmarshal` per event: internal/webui/webui_test.go:214-233, 331, 484, 1671. Impact: weaker failure messages (a bad fragment fails deep in an unmarshal error instead of a named wire diff) and per-test plumbing. Example:
   ```go
   frag := ssetest.RequireDataJSON[fragment](t, evt, fragment{}) // named diff on mismatch
   ```
3. **~~v0.6.2 pin residue in 11 nested go.mods~~ RESOLVED 2026-11-09 (commit `70284764`).** Every nested go.mod — cmd/tq, examples/embed, internal/{composition,readmodel,worker}, internal/queue/{sqlite,sqlitev4,postgres,postgresv4}, queue/{sqlite,postgres} — now carries `v0.6.2`→`v0.7.0 // indirect` (verified 2026-11-11 by `grep -rn 'go-sse' --include=go.mod . | grep -v vendor`: zero v0.6.2 lines); `cmd/tq/go.mod:82` is v0.7.0 and `GOWORK=off go mod tidy -diff` is clean. The fleet-tidy hygiene item is closed.

## 4. Score + top opportunities

N=17 rows; N/A=6; A=11; FULL=10, PARTIAL=1, MISSED=0 → Σ=10.5 → score = round(100×10.5/11) = **95 — very high**. Denominator note: the 2026-11-08 edition stated the same 95 from N=16/A=10/Σ=9.5, but its own table carries 17 rows (A=11) — an arithmetic slip this refresh corrects; with Finding 1 resolved the corrected inputs independently land on 95 (10.5/11). Top: (1) finish Finding 2 — replace the four hand-rolled decode loops with `RequireDataJSON`/`RequireEventType` (checkout ssetest/compat.go:143+); (2) no further adoption targets — the remaining N/A rows are ADR-0003/hub-design rulings, not gaps; (3) keep the nested-pin tidy-clean state green.

## 5. Version currency + risk

Root pins are current: v0.7.0 / sseparse v0.2.1 / ssetest v0.5.0 all equal latest (verified 2026-11-11 by `git -C /home/lars/projects/go-sse tag -l | sort -V | tail -1` + `GOFLAGS=-mod=mod go list -m all | grep go-sse`; the nested v0.6.2 residue is gone too). HEAD is `v0.7.0-2-gd79edb5` (2026-11-10), two commits past the tag. Checkout cite check: `BroadcasterHealth` struct at fanout.go:79, `DefaultSubscriberBuffer = 64` at fanout.go:19 (the 2026-11-08 edition's fanout.go:438 cite no longer matches the checkout — the file was reorganized upstream; re-verified at `d79edb5`), ssetest `FilterByType` compat.go:85 / `MustReadEvents` :47 / `RequireDataJSON` :143, `stream.Context()` stream.go:192, `Heartbeat` stream.go:246. The v0.6.2→v0.7.0 delta contains no wire change — the only breaking edge is ssetest's `WithLastEventID(sse.EventID)`, already adopted (webui_test.go:474). Risk: LOW — same-author library, 100% statement coverage claimed in FEATURES.md (measured upstream 2026-10-11), pins fully reconciled.

## 6. Conclusion

go-sse is the load-bearing transport of the webui and is used well: the broadcaster/fan-out model, branded event IDs as journal watermarks, heartbeats, broadcaster health on `/health`, and the test collectors are all idiomatic, and the snapshot-resume design deliberately (and correctly) skips `Replay`. The one remaining gap is test ergonomics — swapping the last hand-rolled decode loops for the lib's named-diff assertions — not transport correctness.

## How to verify

```bash
grep -rn 'go-sse' --include=go.mod . | grep -v vendor   # all pins v0.7.0 / ssetest v0.5.0 / sseparse v0.2.1 — no v0.6.2 residue
grep -rhoE 'sse\.[A-Z][a-zA-Z]*' --include='*.go' internal | grep -v '^vendor' | sort | uniq -c | sort -rn
grep -rhoE 'ssetest\.[A-Z][a-zA-Z]*' internal/webui/*_test.go | sort | uniq -c | sort -rn
go list -m all | grep 'go-sse'          # effective: v0.7.0 / v0.2.1 / v0.5.0 (readonly default — the -mod=mod prefix was a lint-leak writer, dropped 2026-11-12)
cd cmd/tq && GOWORK=off go mod tidy -diff 2>/dev/null | grep 'go-sse'   # empty — no drift
cd /home/lars/projects/go-sse && git describe --tags     # v0.7.0-2-gd79edb5
grep -n 'type BroadcasterHealth' /home/lars/projects/go-sse/fanout.go    # :79
sed -n '255,260p' internal/webui/webui.go                # prober.liveSSE = hub.bc.Health()
sed -n '264,277p' internal/webui/health.go               # liveSSE rendered into the /health check map
go test ./internal/webui -run 'TestSSEFullSnapshotOnConnect|TestResumeAfterFactsBacklog' -count=1
go test ./internal/webui -run 'TestHealthLiveSSECheck' -count=1
```
All commands above were run literally on 2026-11-11 and produced the cited outputs (greps against the working tree; tests at HEAD `60c19070` via a throwaway `git worktree` while the in-flight sqlite v4→v5 migration had the working tree unbuildable — runs: ok 0.154s and ok 0.007s). Re-executed in-tree 2026-11-12 after the facade repair (5e7fc982): both tests ok, rc under docs/status/assets/2026-11-12_04-54_repair-battery/; the block's `-mod=mod` prefix was dropped — it materialized the module graph into go.sum (the lint-leak writer class).
