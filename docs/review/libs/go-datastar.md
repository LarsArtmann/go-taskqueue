# go-datastar — Library Utilization Review

**Verdict:** ADOPTED — low (0.5/3 capability rows leveraged)
**Pinned:** root `go.mod:57` `go-datastar v0.6.3 // indirect` (a manual pin above go-health-dashboard's v0.6.2 requirement) and `go.mod:108` `go-datastar/static v0.6.1` (direct, the only tq-imported package); `cmd/tq/go.mod:75-76` carries `v0.6.2` + `static v0.6.1 // indirect` (tidy-drifts to v0.6.3, see §5).
**Latest:** v0.7.0 (local checkout /home/lars/projects/go-datastar, checked 2026-11-08; tag 2026-11-07, four-module lockstep: root, static, broadcast, datastartest — `broadcast/` is the NEW submodule)
**Prior coverage:** none
**Reviewed:** 2026-11-08 by `grep -rl --include='*.go' 'larsartmann/go-datastar' . | grep -v '^./vendor/'`, `go mod why github.com/larsartmann/go-datastar{,/static}`, `go mod graph | grep ' github.com/larsartmann/go-datastar@'`, `go mod tidy -diff` in cmd/tq, vendor-vs-checkout diff of static/static.go, full reads of /home/lars/projects/go-datastar README.md + FEATURES.md + CHANGELOG.md §[0.7.0]/§[0.6.3] + broadcast/, `go test ./internal/webui -run 'TestServeHeaderMatrixSDKBundle' -count=1` (ok 0.007s)

## 1. Usage inventory

Exactly one tq file imports go-datastar: `internal/webui/health.go` (verified 2026-11-08 by `grep -rl --include='*.go' 'larsartmann/go-datastar' . | grep -v '^./vendor/'`). The import is the `static` submodule only (health.go:20). tq serves the embedded Datastar runtime same-origin at `/health/datastar.js`: `serveDatastarSDK` sets Content-Type + `Cache-Control: public, max-age=86400` and writes `static.Bytes()` (health.go:363-369); the route is registered GET-only under the health headers wrapper (webui.go:277). The path is handed to go-health-dashboard as `DatastarJS: HealthSDKPath` (health.go:327), and the dashboard renders the script tag from it (`cfg.DatastarSrc = cfg.Routes.DatastarJS`, vendor go-health-dashboard/dashboard.go:151) — same-origin per the go-health-dashboard adoption posture, never the jsDelivr CDN (`SDKScript`/`CDN` appear nowhere in tq code; verified by grep).

The root module itself enters the graph only transitively: `go mod why` → `internal/webui → go-health-dashboard → go-datastar` (verified 2026-11-08). tq emits no Datastar patches and accepts no inbound Datastar requests; the only Datastar-speaking page is go-health-dashboard's `/health` (its `/health/sse` is mounted at webui.go:276).

## 2. Capability inventory

| Capability | Status | Evidence |
| --- | --- | --- |
| Embedded runtime bundle (`static.Bytes()`/`static.Version`) | 🟡 PARTIAL | `Bytes()` served (health.go:365-369); `Version` const unused — no cache identity anywhere on the tq side (static.go:21: `"1.0.3"` at the pinned v0.6.1) |
| `ScriptHandler` (ETag + Cache-Control + 304 revalidation + HEAD/405 contract) | 🔴 MISSED | hand-rolled `serveDatastarSDK` lacks ETag (health.go:363-369); the lib handler ships exactly this surface (checkout script_handler.go; FEATURES.md "Script Serving" row) |
| `ScriptTagVersioned` + `ETagFor` cache-busting (new in v0.7.0) | 🔴 MISSED | tq's bundle is served unversioned with a 24 h max-age — the exact old-client/new-server skew window v0.7.0's release note describes (CHANGELOG §[0.7.0] Added) |
| Patch vocabulary (`ElementsPatch`/`SignalsPatch`/`ScriptPatch` + sugar) and `Response` builder | ⚪ N/A | ADR-0003: dashboard live updates are payload-less ticks driving full server-rendered snapshots over go-sse (hub.go:15-38, handlers.go:349-356); tq renders no `data-*` Datastar attributes anywhere |
| `broadcast/` submodule (Hub, `http.Handler` fan-out, replay, heartbeat options) | ⚪ N/A | no Datastar patch stream exists to fan out; tq's payload-less fan-out lives directly on go-sse `Broadcaster[sse.Event]` (hub.go:20-22) |
| `MemoryStore`/`EventStore` replay | ⚪ N/A | snapshot-resume by design (handlers.go:349-356); there is no patch log to replay |
| Inbound helpers (`ReadSignals`, `LastEventID`) | ⚪ N/A | loopback read-only UI: no `datastar=` query strings or signal bodies arrive (writes are CSRF+lockout-guarded HTML forms per SECURITY.md posture) |
| `datastartest` module | ⚪ N/A | tq owns no Datastar-emitting endpoints; `/health/sse` is go-health-dashboard's contract — tq tests only mounting/headers (health_test.go:263-264, headermatrix_test.go:505,535,640) |

## 3. Findings

1. **`/health/datastar.js` serves without an ETag.** Offer: `datastar.ScriptHandler()` gives ETag + `Cache-Control` + 304 revalidation + GET/HEAD-only with 405 (checkout script_handler.go; `ETagFor(bundle)` exported in v0.7.0 for tests). We do instead: two hand-set headers + `static.Bytes()` (internal/webui/health.go:363-369); method filtering happens to exist because the route table is GET-only (webui.go:277). Impact: after the 86 400 s freshness window every browser re-downloads the 33.5 KB bundle instead of getting a 304. Example:
   ```go
   // instead of the hand-rolled handler:
   {"GET", HealthSDKPath, withHealthHeaders(datastar.ScriptHandler()).ServeHTTP},
   ```
2. **Client bundle one release behind, and unversioned.** Pinned `static v0.6.1` embeds the Datastar JS client v1.0.3 (vendor static/static.go:21, 33 538 bytes); latest `static v0.7.0` embeds v1.0.4 (checkout static/static.go:21, 33 553 bytes). Because the bundle ships with `max-age=86400` and no `?v=` cache key, a future pin bump leaves browsers up to 24 h on the old client against a new server — the precise skew `ScriptTagVersioned(path)` exists to close (CHANGELOG §[0.7.0]). Boundary note: the `<script>` tag is rendered by go-health-dashboard from the path tq passes (health.go:327 → vendor dashboard.go:151), so the fix is bump `static` to v0.7.0 for the client fix and pair it with the dashboard's tag rendering; verified scope ends at tq's serving side.
3. **Version split root v0.6.3 vs cmd/tq v0.6.2 — tidy drift, not divergence.** `cmd/tq/go.mod:75` says `v0.6.2 // indirect` while `GOWORK=off go mod tidy -diff` (run 2026-11-08) raises it to v0.6.3; the workspace already resolves v0.6.3 (`go list -m all` in cmd/tq). Even a replace-free ADR-0017 build reaches v0.6.3 by MVS through the root module's require (verified by `cd cmd/tq && GOWORK=off go mod graph | grep 'go-datastar@'` → `go-taskqueue@v0.4.2 → go-datastar@v0.6.3`). Context: v0.6.3 was a root-only cut whose submodule lockstep tags were skipped (CHANGELOG §[0.6.3], backfilled 2026-11-07) — so tq's `0.6.3 + static v0.6.1` pair matches that release shape exactly, not a mis-pin.
4. **Double-shipping check: none.** `templ-components/datastar v1.27.0` (also in the graph, via go-health-dashboard) requires the same `go-datastar/static v0.6.1`, so the vendor tree carries exactly one embedded bundle (verified 2026-11-08 by `ls vendor/github.com/larsartmann/go-datastar/static/` — one datastar.js, 33 538 bytes); tq never renders `SDKScript` (whose default is the CDN), so no second runtime ships and no cross-origin script enters the CSP.
5. **Architecture note (unscored).** The library's core idea — patches are values (`Patch.Event() sse.Event`) that can be broadcast, stored, filtered — maps one-to-one onto tq's hub model. If the main dashboard ever moves from payload-less ticks + full snapshots to fragment-level updates, `NewElementsPatch` + templ-components' Datastar wave is the natural path; today that stays deliberately out of scope per ADR-0003.
## 4. Score + top opportunities

N=8 rows; N/A=5; A=3; PARTIAL=1, MISSED=2 → Σ=0.5 → score = round(100×0.5/3) = **17 — low**. That is the honest reading: tq uses go-datastar as a same-origin JS-bundle vendor for the health dashboard and nothing else — which is exactly what the go-health-dashboard integration needs, but it leaves the protocol library itself (patches, response builder, broadcast, test helpers) untouched. Top: (1) swap `serveDatastarSDK` for `ScriptHandler`; (2) bump `static` → v0.7.0 (client v1.0.4) and close the stale-client window; (3) tidy the cmd/tq pin.

## 5. Version currency + risk

Not current: root v0.6.3 vs latest v0.7.0 (tagged 2026-11-07, one day before this review). The v0.7.0 cut is the four-module lockstep repair of v0.6.3's root-only defect and carries: `ScriptTagVersioned`, exported `ETagFor`, `broadcast.WithHeartbeatDisabled`, `broadcast/storecontract` test assertions, and the BREAKING `ScriptHandlerWith(bundle)` signature drop — none break tq, which uses only `static`. Upgrading `static` v0.6.1 → v0.7.0 is a client-runtime bump (v1.0.3 → v1.0.4) with wire-format goldens unchanged upstream. Risk: LOW for the current pin set (loopback, read-only dashboard, single consumer of the bundle); the v0.7.0 bump is a routine, well-documented step, gated by `check-gomod-vendor-sync.sh` + the VendorHash dance (AGENTS.md Known Issues).

## 6. Conclusion

go-datastar's adoption in tq is intentionally narrow: the health dashboard is Datastar's only surface, and tq's own job is to serve the runtime same-origin — done with six lines of hand-rolled handler code where the library ships a better one. Adopt `ScriptHandler`, ride the v0.7.0 lockstep when the next dep sweep runs, and leave the protocol vocabulary parked until ADR-0003's snapshot-tick design is ever revisited.

## How to verify

```bash
grep -rn 'go-datastar' --include=go.mod . | grep -v vendor          # root 0.6.3/static 0.6.1 + cmd/tq 0.6.2
grep -rl --include='*.go' 'larsartmann/go-datastar' . | grep -v '^./vendor/'   # health.go only
go mod why github.com/larsartmann/go-datastar                       # webui → go-health-dashboard → go-datastar
grep -n 'Version =' vendor/github.com/larsartmann/go-datastar/static/static.go     # "1.0.3"
grep -n 'Version =' /home/lars/projects/go-datastar/static/static.go               # "1.0.4"
grep -rn 'SDKScript\|jsdelivr' internal/webui --include='*.go' --include='*.templ' | grep -v _templ.go   # empty
cd /home/lars/projects/go-datastar && git describe --tags           # broadcast/v0.7.0-4-g39e415f
cd cmd/tq && GOWORK=off go mod tidy -diff 2>/dev/null | grep 'go-datastar'   # 0.6.2 → 0.6.3 drift
go test ./internal/webui -run 'TestServeHeaderMatrixSDKBundle' -count=1   # SDK route/header contract
```
All commands above were run literally on 2026-11-08 and produced the cited outputs (test run: ok 0.007s).
