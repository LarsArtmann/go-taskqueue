# go-sse — Library Utilization Review

> **2026-11-22 refresh:** every pin, symbol count, and file:line cite re-verified against the tree at HEAD
> `1c8219e6` (339 commits past the 11-11 tree `60c19070`; code identical through the daemon commits that
> landed during the refresh — they touch sibling reports and `internal/webui/theme.css` only). Library-side:
> tags unchanged (root v0.7.0, ssetest/v0.5.0, sseparse/v0.2.1); the checkout moved `v0.7.0-2-gd79edb5` →
> `v0.7.0-19-g8acf04c` (2026-11-21). The material commit is `a3808ed` (2026-11-21, "restore green master"):
> a 350-line file-size split at symbol seams (`Shutdown`/`Health`/`OnSubscribe`/`OnUnsubscribe`/`OnDrop` →
> `fanout_lifecycle.go`; `JoinLines`/`KeyedLines`/`WriteKeyedLines` → `lines.go`; sseparse
> `reader_lines.go`) plus a json/v2 re-encode of the sseparse WPT corpus generator — **no exported API
> change in any of the three trains** (tag→HEAD declaration diff: root 61 exported declarations, ssetest,
> sseparse — all identical) and `stream.go` byte-identical to the tag. tq-side: pins unchanged (14 go.mod
> rows); the only code delta on the SSE surface is the two endpoints' stream prolog folded into one
> `openSSEStream` helper (handlers.go:290-300, commit `8bd8d284` 2026-11-13) — `sse.NewStream` ×2→×1,
> `sse.Stream` ×7→×8; since the T12 helper landed (11-12) `ssetest.FilterByType` ×1→×3, `EventsString`
> ×1→×3, `Event` ×2→×3. **Score re-derived:** the 11-12 edit flipped the assertions row's text to FULL but left §4
> printing FULL=10/PARTIAL=1; at HEAD that row is honestly PARTIAL — the T12 helper covers all seven
> `webui_test.go` decode sites, but `board_test.go` and `evidence_stream_test.go` still hand-roll the
> type-filter + `json.Unmarshal` loop and `auth_test.go` a type-match substring scan, with zero
> `ssetest.Require*` call sites — so FULL=10, PARTIAL=1, MISSED=0 → Σ=10.5/11 → **95 — very high
> (unchanged), now on inputs that reconcile with the table**. Guard tests run IN-TREE this time (the
> sqlite migration landed; the package builds): ok 0.157s + ok 0.015s, whole `internal/webui` package ok
> 15.578s. One verify-block command had broken — the vendored root module makes bare `go list -m all`
> fail ("can't compute 'all' using the vendor directory") — and now carries `-mod=readonly`.

> **2026-11-11 refresh:** re-verified every pin, tag, symbol count, and file:line cite against the
> tree at HEAD `60c19070`. Two of the three 2026-11-08 findings landed on **2026-11-09**:
> Finding 1 resolved (commit `33689678` — `Broadcaster.Health()` surfaced on `/health` via
> `prober.liveSSE`, pinned by `TestHealthLiveSSECheck`) and Finding 3 resolved (commit `70284764`
> — the v0.6.2 residue in the 11 nested go.mods re-tidied to v0.7.0). Finding 2 improved, not
> closed (`FilterByType`/`MustReadEvents`/`EventsString` adopted; the Require* gap closed 2026-11-12 — see the assertions row; re-opened as PARTIAL 2026-11-22 — same row).
> **Denominator correction:** the 2026-11-08 edition printed N=16/A=10, but its own capability
> table has 17 rows (9 FULL + 1 PARTIAL + 1 MISSED + 6 N/A) — A=11. With Finding 1 resolved the
> honest arithmetic is Σ=10.5 → round(100×10.5/11) = **95 — very high** (the 95 stands, now on
> inputs that reconcile). Guard tests run at a throwaway HEAD worktree (working tree unbuildable —
> in-flight sqlite v4→v5 migration, unrelated to webui): ok 0.154s.

**Verdict:** ADOPTED — very high (10.5/11 capability rows leveraged; re-scored 2026-11-11, re-derived 2026-11-22)
**Pinned:** `go-sse v0.7.0` (direct, root `go.mod:112`); `go-sse/ssetest v0.5.0` (direct, root `go.mod:113`); `go-sse/sseparse v0.2.1` (indirect, root `go.mod:59`). All 11 nested-module go.mods carry `v0.7.0 // indirect` (the 2026-11-08 v0.6.2 residue was re-tidied away on 2026-11-09, commit `70284764`; cmd/tq/go.mod:82 included; `GOWORK=off go mod tidy -diff` clean — re-verified 2026-11-22: rc 0, zero lines).
**Latest:** v0.7.0 (local checkout /home/lars/projects/go-sse, checked 2026-11-22; tag 2026-11-04; ssetest/v0.5.0 tagged 2026-10-11, sseparse/v0.2.1 tagged 2026-10-03; HEAD is `v0.7.0-19-g8acf04c` 2026-11-21 — 19 commits past the tag, no newer tag on any train)
**Prior coverage:** none (the 2026-09-14 templ-components deep-dive audits the UI library; its SSE notes cover app.js swap mechanics, not the go-sse library)
**Reviewed:** 2026-11-08 by `grep -rn 'go-sse' --include=go.mod . | grep -v vendor`, `grep -rhoE 'sse\.[A-Z][a-zA-Z]*' --include='*.go' internal`, `grep -rhoE 'ssetest\.[A-Z][a-zA-Z]*' internal/webui/*_test.go | sort | uniq -c`, `GOFLAGS=-mod=mod go list -m all | grep go-sse`, `cd cmd/tq && GOWORK=off go mod tidy -diff | grep larsartmann/go-sse`, full reads of /home/lars/projects/go-sse FEATURES.md + fanout.go + stream.go + CHANGELOG.md, `go test ./internal/webui -run 'TestSSEFullSnapshotOnConnect|TestResumeAfterFactsBacklog' -count=1` (ok 0.149s). **Re-verified 2026-11-11** by the same greps against the current tree, checkout tag/HEAD checks, fresh line-number sweeps of hub.go/agentfeed.go/handlers.go/health.go/webui.go, and the two guard tests at HEAD `60c19070` (ok 0.154s; plus `TestHealthLiveSSECheck` ok 0.007s). **Re-verified 2026-11-22** by the same greps at HEAD `1c8219e6`, per-train tag checks + a tag→HEAD exported-declaration diff of the checkout, coverage re-measured at `8acf04c`, and the guard tests in-tree (ok 0.157s / ok 0.015s; whole `internal/webui` package ok 15.578s)

## 1. Usage inventory

Files: `internal/webui/hub.go`, `internal/webui/handlers.go`, `internal/webui/agentfeed.go`, `internal/webui/health.go` + `internal/webui/webui.go` (the broadcaster-health wiring, since 2026-11-09); tests `internal/webui/{auth,board,evidence_stream,health,webui}_test.go` (verified 2026-11-22 by `grep -rl --include='*.go' 'larsartmann/go-sse' . | grep -v '^./vendor/'` — 10 files, unchanged since 11-11).

Library symbols in repo code (verified 2026-11-22 by `grep -rhoE 'sse\.[A-Z][a-zA-Z]*' --include='*.go' internal | sort | uniq -c | sort -rn`): `sse.Stream` ×8, `sse.Event` ×7, `sse.BroadcasterHealth` ×6, `sse.NewEventID` ×3, `sse.WithBufferSize` ×2, `sse.NewBroadcaster` ×2, `sse.Broadcaster` ×2, `sse.NewStream` ×1, `sse.LastEventIDFromRequest` ×1. ssetest in tests: `WithPath` ×14, `CollectN` ×8, `CollectWithTimeout` ×6, `FilterByType` ×3, `EventsString` ×3, `Event` ×3, `WithLastEventID` ×1, `RequestOption` ×1, `MustReadEvents` ×1.

Two broadcasters: `Hub` fans payload-less `tick` events whose ID is the journal watermark (hub.go:15-38); `AgentFeed` fans typed `AgentEvent` payloads (agentfeed.go:25-40). Endpoints: `/api/events` + per-task stream share `runEventStream` (subscribe → snapshot → tick-pump, handlers.go:383-435); the agent live feed runs its own loop (handlers.go:303-338); since 2026-11-13 both stream loops (`handleAgentLive`, and `runEventStream` behind `/api/events` + `/task/{id}/events`) open through ONE prolog, `openSSEStream` (Flusher probe → `X-Accel-Buffering: no` → `sse.NewStream`, handlers.go:290-300). Heartbeats ride `stream.Heartbeat` behind `startHeartbeat` (handlers.go:498-512). Buffer 128 with a documented drop-then-self-heal policy (hub.go:10-13). Since 2026-11-09 the hub's lifecycle is observable: `prober.liveSSE = func() *sse.BroadcasterHealth { snap := s.hub.bc.Health(); return &snap }` (webui.go:263-267) feeds subscriber/buffer state into the `/health` JSON (health.go:385-407).

| Surface | File:lines | Lib symbols |
| --- | --- | --- |
| Hub (payload-less ticks, watermark IDs) | internal/webui/hub.go:20-57 | `Broadcaster[sse.Event]`, `WithBufferSize`, `NewEventID`, `Shutdown` |
| AgentFeed (typed payload fan-out) | internal/webui/agentfeed.go:35-68 | `Broadcaster[AgentEvent]`, `WithBufferSize`, `Shutdown` |
| Dashboard SSE protocol | internal/webui/handlers.go:383-435 | `NewStream` (via `openSSEStream` :290-300), `Stream.Send`, `Stream.SendJSON`, `LastEventIDFromRequest` (:349) |
| Agent live feed | internal/webui/handlers.go:303-338 | `Stream.SendJSON("agent")` (:333) |
| Heartbeat wrapper | internal/webui/handlers.go:498-512 | `Stream.Heartbeat` |
| Snapshot burst | internal/webui/handlers.go:529-552 | `Stream.SendJSON("frag")` (:537), `Event.ID`, `Stream.Send` |
| Broadcaster health on `/health` | internal/webui/webui.go:263-267 + health.go:385-407 | `Broadcaster.Health()`, `BroadcasterHealth` |
| Handler tests | internal/webui/webui_test.go:208-226 (the `fragmentEvents` helper), 228-250, 463-490 | `ssetest.CollectN`, `CollectWithTimeout`, `WithPath`, `WithLastEventID` |

## 2. Capability inventory

| Capability | Status | Evidence |
| --- | --- | --- |
| Wire-format serialization (`Event`/`WriteEvent`/heartbeats) | 🟢 FULL | all emission via `stream.Send`/`stream.SendJSON` — zero hand-rolled `data:` lines (handlers.go:529-552; re-verified 2026-11-22 by `grep -rnE 'data: ' --include='*.go' internal/webui \| grep -v _test.go` → one comment line only; lib writer alloc-minimized per FEATURES.md) |
| Stream lifecycle (`NewStream`, headers, `Close`, context) | 🟢 FULL | `NewStream` at the single `openSSEStream` site with deferred `Close` at both loop owners (handlers.go:314, 392); `Flusher` probe + `X-Accel-Buffering: no` first (handlers.go:290-300); `r.Context()` used where lib offers equivalent `stream.Context()` (stream.go:192) |
| Heartbeat keepalive | 🟢 FULL | `stream.Heartbeat(hbCtx, s.cfg.Heartbeat)` wrapped in cancel+wait (handlers.go:498-512); the lib stops on ctx/closed/write-error (stream.go:246-274) but offers no wait contract, so tq's `<-done` guard is deliberate defensive code, not a misuse — pinned by `TestSSEHeartbeatStopsBeforeHandlerExit` (webui_test.go:2348) |
| `Last-Event-ID` extraction | 🟢 FULL | `sse.LastEventIDFromRequest(r)` + branded `EventID`, used for reconnect-lag logging (handlers.go:349-363) |
| Branded `EventID` type safety (`NewEventID`/`ParseEventID`/`MustParseEventID`) | 🟢 FULL | watermark IDs constructed at the boundary via `sse.NewEventID(formatSeq(seq))` (hub.go:33-38, handlers.go:542-545); untrusted header values validated inside the lib (rejects `\n`/`\r` per FEATURES.md) |
| Reconnection replay (`EventStore`/`Replay`/`ReplayFiltered`) | ⚪ N/A | snapshot-resume by design: ticks are payload-less, so "any Last-Event-ID is satisfied by a fresh snapshot" (handlers.go:344-363, 376-382); replaying ticks would re-send what the snapshot already covers |
| Fan-out `Broadcaster[T]` (Subscribe/Unsubscribe/Broadcast/Shutdown, `WithBufferSize`, drop policy) | 🟢 FULL | two broadcasters with explicit buffer + graceful drain (hub.go:20-57, agentfeed.go:35-68); agent feed is the generic-type showcase (`Broadcaster[AgentEvent]`) |
| Predicate subscription (`SubscribeFilter`) | ⚪ N/A | single-topic fan-out — every dashboard client receives every tick by design; no per-client topics exist |
| Broadcaster observability (`Health()`, `OnSubscribe`/`OnUnsubscribe`/`WithOnDrop` hooks, `BroadcastMany`) | 🟢 FULL | `BroadcasterHealth` surfaced on `/health` since 2026-11-09 (commit `33689678`): `prober.liveSSE` snapshots `h.bc.Health()` (webui.go:263-267) and the prober renders "N live subscriber(s), buffer M event(s)" into the check map (health.go:385-407 — closed → FAIL :391-396, draining → WARN :397-398, live → PASS :400-404); a closed hub fails the live check (pinned by `TestHealthLiveSSECheck`, health_test.go:570-594 — pass + closed; the draining branch is rendered but unpinned); the `OnSubscribe`/`OnUnsubscribe`/`WithOnDrop` hooks and `BroadcastMany` remain unused — no operator need |
| DataStar-pattern helpers (`SendKeyed`/`SendLines`/`KeyedLines`) | ⚪ N/A | tq payloads are JSON fragments via `SendJSON` ("frag" handlers.go:537 / "agent" handlers.go:333); Datastar-keyed emission belongs to go-health-dashboard's `/health/sse` |
| Retry directive (`Event.Retry`/`WriteRetry`) | ⚪ N/A | browser default reconnect cadence is fine; no custom retry hint needed on a loopback dashboard |
| `SendJSON` convenience | 🟢 FULL | fragment and agent payloads marshal-and-send in one call (handlers.go:537 "frag", handlers.go:333 "agent") — marshal-error and nil-value paths owned by the lib (FEATURES.md Stream table) |
| `OnDisconnect` callbacks | ⚪ N/A | subscriber cleanup rides channel close + request context (handlers.go:318-319, 396-397); no per-disconnect work exists to hook |
| ssetest collect helpers | 🟢 FULL | `CollectN`/`CollectWithTimeout` across the same 4 test files — 14 call sites at HEAD (verified 2026-11-22): webui_test.go ×11 (:232, 276, 329, 476, 526, 1618, 1672, 1733, 1800, 2365, 2372); auth_test.go:134; board_test.go:225; evidence_stream_test.go:73 |
| ssetest request options | 🟢 FULL | `WithPath` ×14 + `WithLastEventID` with the v0.5.0 branded-`EventID` breaking change already adopted (webui_test.go:473) |
| ssetest assertions (`Require*`, `RequireDataJSON[T]`, `FilterByType`) | 🟡 PARTIAL | re-opened 2026-11-22 (the 11-12 "closed" text was true for `webui_test.go` only): the T12 helper (commit `dd4dda5c`) covers every decode site in that file — `fragmentEvents` (webui_test.go:208-226, `FilterByType` + single decode, malformed payload fails at the decode site) is called at :234, 287, 333, 480, 1625, 1688, 1804 and `EventsString` carries the frag-count failure detail (:239, 408, 1630); but the sibling files were never converted — board_test.go:229-243 and evidence_stream_test.go:77-91 still hand-roll `evt.Type != testFragEvent → continue` + `json.Unmarshal([]byte(evt.Data()), &frag)`, and auth_test.go:138-142 a type-match + `strings.Contains(evt.Data(), …)` scan; zero `ssetest.Require*` call sites (`grep -rn 'ssetest\.Require' internal` → no matches). `RequireDataJSON[T]` itself stays unused BY SHAPE — it exact-compares a whole decoded value and tq's fragment payloads carry variable HTML, so decode-and-inspect is the only consumer shape; `Collect`/`CollectPost`/`CollectWithRequest` unused (CollectN + CollectWithTimeout cover every need) |
| sseparse module (standalone parser/corpus) | ⚪ N/A | tq never parses SSE off-HTTP (re-verified 2026-11-22: 0 non-test `sseparse.` sites); the browser parses the wire; tests parse via ssetest's re-export |

## 3. Findings

1. **~~Broadcaster health is invisible~~ RESOLVED 2026-11-09 (commit `33689678`, fleet review f18).** The offered wiring exists nearly verbatim: `prober.liveSSE = func() *sse.BroadcasterHealth { snap := s.hub.bc.Health(); return &snap }` (internal/webui/webui.go:263-267), the prober field (health.go:129-133), and the check-map rendering "`%d live subscriber(s), buffer %d event(s)`" (health.go:385-407) — an operator now sees how many dashboards stream and that the hub is alive (a closed hub FAILS the live check while the process looks healthy; a draining hub WARNs). Pinned by `TestHealthLiveSSECheck` (health_test.go:570-594, ok 0.015s in-tree at HEAD `1c8219e6`). The lib's `OnSubscribe`/`OnUnsubscribe`/`WithOnDrop` hooks and `BroadcastMany` remain unused — nothing needs them on a two-broadcasters loopback surface (see §4 for the one optional `WithOnDrop` use).
2. **ssetest assertions partly on the table (PARTIAL — scope corrected 2026-11-22).** The reconnect test uses `ssetest.MustReadEvents` + `FilterByType` + `EventsString` (internal/webui/webui_test.go:395-409), and since 2026-11-12 every `webui_test.go` fragment decode rides `fragmentEvents` (:208-226). The 11-12 row text claimed "every fragment decode rides the one helper" — true only for that file: board_test.go:229-243 and evidence_stream_test.go:77-91 still carry a hand-rolled type-filter + `json.Unmarshal` loop, and auth_test.go:138-142 a type-match + substring scan (all three predate T12 — last touched 2026-09-26 / 2026-10-31 / 2026-09-29 by `git log -1`). Offer still open: `ssetest.RequireDataJSON[T]`, `RequireEventCount`, `RequireEventType`, `RequireDataContains` (checkout ssetest/compat.go:97-145). Impact: duplicated decode plumbing and weaker failure messages in three tests (a bad fragment in the two sibling files fails at a bare `t.Fatalf("decode frag: %v")` instead of the helper's named event context). The fix is the helper tq already owns:
   ```go
   for _, frag := range fragmentEvents(t, events) { // FilterByType + the one decode site, named failure detail
   ```
3. **~~v0.6.2 pin residue in 11 nested go.mods~~ RESOLVED 2026-11-09 (commit `70284764`).** Every nested go.mod — cmd/tq, examples/embed, internal/{composition,readmodel,worker}, internal/queue/{sqlite,sqlitev4,postgres,postgresv4}, queue/{sqlite,postgres} — now carries `v0.6.2`→`v0.7.0 // indirect` (re-verified 2026-11-22 by `grep -rn 'go-sse' --include=go.mod . | grep -v vendor`: 14 rows, zero v0.6.2 lines); `cmd/tq/go.mod:82` is v0.7.0 and `GOWORK=off go mod tidy -diff` is clean (rc 0, zero lines). The fleet-tidy hygiene item is closed.

## 4. Score + top opportunities

N=17 rows; N/A=6; A=11; FULL=10, PARTIAL=1 (the ssetest-assertions row), MISSED=0 → Σ=10.5 → score = round(100×10.5/11) = **95 — very high**. Re-derivation note: the 11-11 edition printed this 95 from exactly these inputs while its table showed the assertions row PARTIAL; the 11-12 T12 edit flipped that row's text to FULL without re-deriving §4 (a literal count of the table as it stood on 11-12 reads 11 FULL = 100). At HEAD the row is PARTIAL on evidence — three unconverted sibling test sites — so table and inputs agree again. Top: (1) finish Finding 2 — route the two sibling decode loops through `fragmentEvents` and the auth scan through `FilterByType` (checkout ssetest/compat.go:79-145); (2) optional observability: `sse.WithOnDrop` (checkout fanout.go:41-63) would give tq's own hub the per-subscriber drop counter go-health-dashboard already exposes for ITS pusher (`dashboard_sse_drops_total`, vendor go-health-dashboard/metrics.go) — the drop-then-self-heal policy (hub.go:10-13) is sound without it, so this is a nice-to-have, not a gap; (3) keep the nested-pin tidy-clean state green — there is no newer tag to ride (all three trains current).

## 5. Version currency + risk

Root pins are current: v0.7.0 / sseparse v0.2.1 / ssetest v0.5.0 all equal latest (verified 2026-11-22 by `git -C /home/lars/projects/go-sse tag -l | sort -V | tail -1` → `v0.7.0`, the same per train with `git tag -l 'ssetest/*'` / `'sseparse/*'`, + `go list -mod=readonly -m all | grep go-sse`; the nested v0.6.2 residue is gone too). HEAD is `v0.7.0-19-g8acf04c` (2026-11-21), 19 commits past the tag. Checkout cite check at `8acf04c`: `BroadcasterHealth` struct at fanout.go:74 and `DefaultSubscriberBuffer = 64` at fanout.go:14 (both 5 lines above their tag positions — the 2026-11-21 reorganization, commit `a3808ed`, moved the lifecycle methods and 5 import lines out to `fanout_lifecycle.go`; tq's vendored v0.7.0 copy — byte-identical to the tag — still has them at fanout.go:79 / :19), `Health()` implementation fanout_lifecycle.go:113, ssetest `FilterByType` compat.go:85 / `MustReadEvents` :47 / `RequireDataJSON` :145 (vendored copy: the same three lines), `stream.Context()` stream.go:192, `Heartbeat` stream.go:246. The v0.6.2→v0.7.0 delta contains no wire change — the only breaking edge is ssetest's `WithLastEventID(sse.EventID)`, already adopted (webui_test.go:473). The post-tag delta is moves and test splits: `git diff --stat v0.7.0..HEAD -- '*.go' ':(exclude)*_test.go' ':(exclude)example'` lists eight files — three symbol-seam moves (event.go→lines.go, fanout.go→fanout_lifecycle.go, sseparse/reader.go→reader_lines.go; six files) and two sseparse content changes, the WPT corpus schema and generator moving to `encoding/json/v2` (`corpus.go` tag `omitempty`→`omitzero`, `gen_corpus.go`, gated by `sseparse/corpus_encoder_boundary_test.go`). tq consumes none of it: it takes sseparse indirect purely for ssetest's parser re-export (`go mod why github.com/larsartmann/go-sse/sseparse` → internal/webui.test → ssetest → sseparse). Exported surface is unchanged — a declaration-grep diff over broadcaster/fanout/event/lines/stream/replay/constants reads 61 exported declarations at both v0.7.0 and HEAD, and ssetest v0.5.0→HEAD and sseparse v0.2.1→HEAD likewise. Risk: LOW — same-author library, 100.0% statement coverage RE-MEASURED 2026-11-22 at the checkout HEAD (`go -C /home/lars/projects/go-sse test . -cover -count=1` → `coverage: 100.0% of statements`; same result for the ssetest and sseparse packages; the example/ packages are outside the claim) — pins fully reconciled and nothing in the unreleased delta is waitable-on.

## 6. Conclusion

go-sse is the load-bearing transport of the webui and is used well: the broadcaster/fan-out model, branded event IDs as journal watermarks, heartbeats, broadcaster health on `/health`, and the test collectors are all idiomatic, and the snapshot-resume design deliberately (and correctly) skips `Replay`. The one remaining gap is test ergonomics — two sibling test files' hand-rolled decode loops and one type-match substring scan where the lib's filter/assert helpers fit — not transport correctness.

## How to verify

```bash
grep -rn 'go-sse' --include=go.mod . | grep -v vendor   # 14 rows, all v0.7.0 / ssetest v0.5.0 / sseparse v0.2.1 — no v0.6.2 residue
grep -rhoE 'sse\.[A-Z][a-zA-Z]*' --include='*.go' internal | grep -v '^vendor' | sort | uniq -c | sort -rn
grep -rhoE 'ssetest\.[A-Z][a-zA-Z]*' internal/webui/*_test.go | sort | uniq -c | sort -rn
grep -rn 'ssetest\.Require' internal                      # no matches — the PARTIAL assertions row
go list -mod=readonly -m all | grep 'go-sse'          # effective: v0.7.0 / v0.2.1 / v0.5.0 (bare form now fails: "can't compute 'all' using the vendor directory")
cd cmd/tq && GOWORK=off go mod tidy -diff 2>/dev/null | grep 'go-sse'   # empty — no drift
cd /home/lars/projects/go-sse && git describe --tags     # v0.7.0-19-g8acf04c
grep -n 'type BroadcasterHealth' /home/lars/projects/go-sse/fanout.go    # :74 (vendored v0.7.0 copy: fanout.go:79)
cd /home/lars/projects/go-taskqueue
sed -n '263,267p' internal/webui/webui.go                # prober.liveSSE = hub.bc.Health()
sed -n '385,407p' internal/webui/health.go               # liveSSE rendered into the /health check map
go test ./internal/webui -run 'TestSSEFullSnapshotOnConnect|TestResumeAfterFactsBacklog' -count=1
go test ./internal/webui -run 'TestHealthLiveSSECheck' -count=1
```
All commands above were run literally on 2026-11-22 against HEAD `1c8219e6` (go1.27.4; `go env GOEXPERIMENT`=jsonv2, no root go.work — the same conditions as the explicit `GOEXPERIMENT=jsonv2 GOWORK=off` used for the first full build) and produced the cited outputs — IN-TREE, no worktree: the package builds again. Runs: ok 0.157s and ok 0.015s (the ship-gate sequential replay of the block as printed; earlier runs the same session: 0.154s / 0.007s); the whole `internal/webui` package: ok 15.578s. Two block repairs this refresh: `go list -m all` gained `-mod=readonly` (the bare form now errors out under the vendored root module; `-mod=mod` stays excluded for the lint-leak reason recorded 2026-11-12 — re-checked, `-mod=readonly` left `git status` unchanged), and the `sed` ranges / `describe` output were re-pointed at the current lines (a `cd` back to the repo root was added after the absolute-path `cd` lines so the block replays sequentially in one shell). Daemon commits that landed during the refresh (at least `b1984388`, `f19acb32`) touch only sibling reports and `internal/webui/theme.css` — `git diff --name-only 1c8219e6..HEAD | grep -v '^docs/'` prints just that CSS file — so the Go tree is identical to `1c8219e6`. History: 2026-11-11 the same block ran against `60c19070` (tests via a throwaway `git worktree` while the in-flight sqlite v4→v5 migration had the working tree unbuildable — ok 0.154s / ok 0.007s); 2026-11-12 re-executed in-tree after the facade repair (5e7fc982), both tests ok (rc archived under docs/status/assets/2026-11-12_04-54_repair-battery/, directory still present). Re-replayed later the same day at HEAD `8395a969` — two further daemon commits (`a28c553b`, `8395a969`) landed in between, docs-only, and the non-docs diff from `1c8219e6` was then just `internal/webui/theme.css` — every command reproduces (ok 0.141s / ok 0.007s; symbol and pin counts byte-for-byte as printed above). One daemon commit after that (`42194237`) reworked cmd/tq worker-flag plumbing (main.go + flag_parity_test.go) — uncited by this report; no webui or go.mod surface moved.
