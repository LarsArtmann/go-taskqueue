# go-datastar — Library Utilization Review

> **2026-11-11 refresh:** the 2026-11-09 post-wave re-score (83) left sections 1-5 describing the
> pre-wave tree; this refresh reconciles the whole report with the wave as it actually landed and
> re-verifies every claim against HEAD `60c19070`. Verified: pins root + cmd/tq + static ALL at
> v0.7.0 with `go mod tidy -diff` clean; `datastar.ScriptHandler()` serving with the SECURITY.md
> ETag row (commit `29109c05`); versioned `healthSDKSrc`; `static` client v1.0.4 both sides. The
> band label is normalized to the repo rubric (83 = **high**, 70-84 — the 11-09 text said
> "moderate-high"). Guard test run at a throwaway HEAD worktree (working tree unbuildable —
> in-flight sqlite v4→v5 migration, unrelated to webui): ok 0.006s.

**Verdict:** ADOPTED — high (2.5/3 capability rows leveraged; re-scored 2026-11-09, re-verified 2026-11-11) — the v0.7.0 lockstep + ScriptHandler adoption flipped two MISSED rows and the cache-identity PARTIAL: bundle serving now rides the library's own handler.

**Post-wave delta (2026-11-09, plan M2 — landed in commits `29109c05`, `33689678`, `70284764`, re-verified 2026-11-11):** pins = root + cmd/tq + static ALL at
v0.7.0 (split healed); the `serveDatastarSDK` hand-roll is DELETED — the SDK is
served by `datastar.ScriptHandler()` (ETag/304/HEAD/405; the root SECURITY.md
matrix gained the ETag row at SECURITY.md:147, pinned by `serveSDKRow`
headermatrix_test.go:319-324) and the dashboard loads it via
`healthSDKSrc = HealthSDKPath + "?v=" + static.Version`
(health.go:442-446; the version query collapses the 24 h stale-client window; built inline
because the dashboard owns tag rendering — vendor go-health-dashboard/dashboard.go:151 —
`ScriptTagVersioned` itself
is unreachable from this integration shape, hence row 3 counts PARTIAL).
Re-score: row 1 PARTIAL→FULL (static.Version now the cache identity),
row 2 MISSED→FULL (ScriptHandler), row 3 MISSED→PARTIAL (cache-busting
outcome achieved; the named helper unused). Σ = 1 + 1 + 0.5 = 2.5/3 →
**83 — high** (was 17). N=11, N/A=8 unchanged: the protocol
library (patches/broadcast/stores) stays deliberately untouched —
ADR-0003's payload-less tick design is not a Datastar-patch design.
**Pinned:** root `go.mod:107` `go-datastar v0.7.0` + `go.mod:108` `go-datastar/static v0.7.0` (both direct since the wave — internal/webui imports root AND static, health.go:22-23); `cmd/tq/go.mod:75-76` `v0.7.0` + `static v0.7.0 // indirect`; `GOWORK=off go mod tidy -diff` clean (verified 2026-11-11 — the v0.6.3/v0.6.2/v0.6.1 drift described in the 2026-11-08 edition is gone).
**Latest:** v0.7.0 (local checkout /home/lars/projects/go-datastar, checked 2026-11-11; tag 2026-11-07, four-module lockstep: root, static, broadcast, datastartest — `broadcast/` is the NEW submodule; checkout HEAD `broadcast/v0.7.0-5-g67d99c6` 2026-11-10, five commits past the tag, no newer tag)
**Prior coverage:** none
**Reviewed:** 2026-11-08 by `grep -rl --include='*.go' 'larsartmann/go-datastar' . | grep -v '^./vendor/'`, `go mod why github.com/larsartmann/go-datastar{,/static}`, `go mod graph | grep ' github.com/larsartmann/go-datastar@'`, `go mod tidy -diff` in cmd/tq, vendor-vs-checkout diff of static/static.go, full reads of /home/lars/projects/go-datastar README.md + FEATURES.md + CHANGELOG.md §[0.7.0]/§[0.6.3] + broadcast/, `go test ./internal/webui -run 'TestServeHeaderMatrixSDKBundle' -count=1` (ok 0.007s). **Re-verified 2026-11-11** by the same greps against the current tree, checkout tag/HEAD checks, and `go test ./internal/webui -run 'TestServeHeaderMatrixSDKBundle' -count=1` at HEAD `60c19070` (ok 0.006s)

## 1. Usage inventory

Exactly one tq file imports go-datastar: `internal/webui/health.go` (verified 2026-11-11 by `grep -rl --include='*.go' 'larsartmann/go-datastar' . | grep -v '^./vendor/'`). Since the 2026-11-09 wave the import is BOTH submodules' surfaces: the root module for `ScriptHandler` and the `static` submodule for the bundle + version (health.go:22-23). tq serves the embedded Datastar runtime same-origin at `/health/datastar.js` via the library's own handler: `healthSDKScript = datastar.ScriptHandler()` (health.go:448-452) carries the ETag, conditional-GET/304, and HEAD-vs-405 contract; the route is registered GET-only under the health headers wrapper (webui.go:324, route table :322-330). The URL handed to go-health-dashboard is versioned: `healthSDKSrc = HealthSDKPath + "?v=" + static.Version` (health.go:442-446), configured as `DatastarJS: healthSDKSrc` (health.go:399), and the dashboard renders the script tag from it (vendor go-health-dashboard/dashboard.go:151) — same-origin per the go-health-dashboard adoption posture, never the jsDelivr CDN (the library's `SDKScript`/CDN default appears nowhere in tq code; the only grep hits are tq's own `healthSDKScript` identifier — verified 2026-11-11).

The root module is now a DIRECT dependency (no longer transitive via go-health-dashboard): `go mod why` → `internal/webui → go-datastar` and `internal/webui → go-datastar/static` (verified 2026-11-11). tq emits no Datastar patches and accepts no inbound Datastar requests; the only Datastar-speaking page is go-health-dashboard's `/health` (its `/health/sse` is mounted at webui.go:323).

Integration map (all rows verified 2026-11-11 against internal/webui):

| Route | tq wiring | Library surface |
| --- | --- | --- |
| `GET /health` | webui.go:322 → `s.dash.Handler()` | go-health-dashboard rendering (Datastar markup), path config health.go:399 |
| `GET /health/sse` | webui.go:323 → `s.dash.SSEHandler()` | go-health-dashboard's patch emission over go-sse |
| `GET /health/datastar.js` | webui.go:324 → `healthSDKScript` | `datastar.ScriptHandler()` (health.go:448-452) over `go-datastar/static` |
| `GET /healthz /readyz /startupz` | webui.go:328-330 → queueProber | none (go-health, outside this review's scope) |

Not in the tq graph: `datastartest` (test-only module) and `broadcast/` (verified 2026-11-11 by `GOFLAGS=-mod=mod go list -m all | grep go-datastar` → only root + static).

## 2. Capability inventory

| Capability | Status | Evidence |
| --- | --- | --- |
| Embedded runtime bundle (`static.Bytes()`/`static.Version`) | 🟢 FULL | bundle served same-origin via the lib handler; `static.Version` is the cache identity of the served URL (`healthSDKSrc`, health.go:442-446); vendored static.go:21 = `"1.0.4"` — client CURRENT (re-verified 2026-11-11) |
| `ScriptHandler` (ETag + Cache-Control + 304 revalidation + HEAD/405 contract) | 🟢 FULL | `healthSDKScript = datastar.ScriptHandler()` (health.go:448-452) on the GET-only route (webui.go:324) under `withHealthHeaders`; ETag shape pinned by `serveSDKRow`/`quotedETag` (headermatrix_test.go:319-334) and the SECURITY.md matrix row (SECURITY.md:147) — adopted 2026-11-09, commit `29109c05` |
| `ScriptTagVersioned` + `ETagFor` cache-busting (new in v0.7.0) | 🟡 PARTIAL | the cache-busting outcome is achieved by the inline `?v=static.Version` query (health.go:442-446) and `ETagFor`'s digest shape is pinned by `quotedETag` (headermatrix_test.go:326-334); the named helper itself is unreachable — the tag is rendered by go-health-dashboard from the path tq passes (health.go:399 → vendor dashboard.go:151) |
| Patch vocabulary (`ElementsPatch`/`SignalsPatch`/`ScriptPatch` + sugar) and `Response` builder | ⚪ N/A | ADR-0003: dashboard live updates are payload-less ticks driving full server-rendered snapshots over go-sse (hub.go:15-38, handlers.go:330-358/363-430); tq renders no `data-*` Datastar attributes anywhere |
| `broadcast/` submodule (Hub, `http.Handler` fan-out, replay, heartbeat options) | ⚪ N/A | no Datastar patch stream exists to fan out; tq's payload-less fan-out lives directly on go-sse `Broadcaster[sse.Event]` (hub.go:20-22) |
| `MemoryStore`/`EventStore` replay | ⚪ N/A | snapshot-resume by design (handlers.go:334-353, 363-372); there is no patch log to replay |
| Inbound helpers (`ReadSignals`, `LastEventID`) | ⚪ N/A | loopback read-only UI: no `datastar=` query strings or signal bodies arrive (writes are CSRF+lockout-guarded HTML forms per SECURITY.md posture) |
| `datastartest` module | ⚪ N/A | tq owns no Datastar-emitting endpoints; `/health/sse` is go-health-dashboard's contract — tq tests only mounting/headers (health_test.go:264-267, headermatrix_test.go:521,550-561,659-661) |
| HTTP attribute helpers (`GetSSE`/`PostSSE`/`PatchSSE`…) | ⚪ N/A | generate `@get`/`@post`-style attribute strings for datastar-marked elements — tq renders none |
| Error system (typed `errorfamily` codes) | ⚪ N/A | zero tq call sites (verified by grep); errors surface inside go-health-dashboard's own handlers |
| `version` package (`-ldflags` injection) | ⚪ N/A | binary versioning is tq's own plumbing; the package is example-binary-oriented (version.go doc comment) |

## 3. Findings

1. **~~`/health/datastar.js` serves without an ETag~~ RESOLVED 2026-11-09 (commit `29109c05`).** The hand-rolled `serveDatastarSDK` was deleted; the route now binds `healthSDKScript = datastar.ScriptHandler()` (internal/webui/health.go:448-452) inside the security-headers wrapper (webui.go:324) — exactly the example from the 2026-11-08 edition, with the caveat honored: `withHealthHeaders` stays for the security posture, the lib handler owns ETag/304/HEAD, and the merged header set is pinned by `serveSDKRow` (headermatrix_test.go:319-324, `TestServeHeaderMatrixSDKBundle` :550 — green at HEAD, ok 0.006s). Operators now get 304 revalidation instead of re-downloading the 33.5 KB bundle after every 86 400 s freshness window.
2. **~~Client bundle one release behind, and unversioned~~ RESOLVED 2026-11-09 (commits `29109c05`/`70284764`).** `static` is pinned at v0.7.0 (root go.mod:108), so the vendored client is v1.0.4 (vendor static/static.go:21, 33 553-byte datastar.js — matching the checkout exactly), and the served URL carries `?v=static.Version` (health.go:442-446), collapsing the 24 h stale-client window to zero. Boundary note stands: the `<script>` tag is rendered by go-health-dashboard from the path tq passes (health.go:399 → vendor dashboard.go:151), so `ScriptTagVersioned` itself remains unused — the PARTIAL in row 3.
3. **~~Version split root v0.6.3 vs cmd/tq v0.6.2 — tidy drift~~ RESOLVED 2026-11-09 (commit `70284764`).** All pins — root `go.mod:107-108`, `cmd/tq/go.mod:75-76`, and the vendored module — sit at v0.7.0, the four-module lockstep cut (CHANGELOG §[0.7.0]: root, static, broadcast, datastartest tagged on one commit, repairing v0.6.3's root-only defect). `cd cmd/tq && GOWORK=off go mod tidy -diff` is clean (verified 2026-11-11, exit 0, no larsartmann lines). The BREAKING v0.7.0 `ScriptHandlerWith(bundle)` signature drop cost tq nothing — the wave adopted the new no-arg `ScriptHandler()` (health.go:452).
4. **Double-shipping check: none (re-verified 2026-11-11).** `templ-components/datastar v1.27.0` (also in the graph, via go-health-dashboard) and tq both resolve `go-datastar/static v0.7.0`, so the vendor tree carries exactly one embedded bundle (`ls vendor/github.com/larsartmann/go-datastar/static/` — one datastar.js, 33 553 bytes, Version "1.0.4" matching the checkout); tq never renders the library's `SDKScript` (whose default is the CDN), so no second runtime ships and no cross-origin script enters the CSP.
5. **Architecture note (unscored).** The library's core idea — patches are values (`Patch.Event() sse.Event`) that can be broadcast, stored, filtered — maps one-to-one onto tq's hub model. If the main dashboard ever moves from payload-less ticks + full snapshots to fragment-level updates, `NewElementsPatch` + templ-components' Datastar wave is the natural path; today that stays deliberately out of scope per ADR-0003.

## 4. Score + top opportunities

N=11 rows; N/A=8; A=3; FULL=2, PARTIAL=1, MISSED=0 → Σ=2.5 → score = round(100×2.5/3) = **83 — high** (repo rubric: very high ≥85, high 70-84 — the 2026-11-09 note's "moderate-high" label normalized here; was 17 on 2026-11-08). tq now uses go-datastar as more than a same-origin JS-bundle vendor: the library owns the serving contract (ETag/304/HEAD) and its version const is the cache identity. What remains untouched is the protocol library itself (patches, response builder, broadcast, test helpers) — by ADR-0003 ruling, not by accident. Top: (1) nothing required today — at the next dep sweep, ride the post-v0.7.0 checkout wave (`broadcast/v0.7.0-5-g67d99c6`) as a routine bump; (2) if the main dashboard ever moves to fragment-level updates, `NewElementsPatch` + `broadcast/` is the path (ADR-0003 watch item); (3) keep the cmd/tq tidy-clean state green (it drifted once already).

## 5. Version currency + risk

Current: root, cmd/tq, static, and vendor ALL at v0.7.0 = newest tag (verified 2026-11-11 by go.mod reads, `go list -m all`, and `go mod tidy -diff` in cmd/tq — clean). The 2026-11-08 upgrade mechanics (pin bump → `go mod vendor` → VendorHash sync) were executed by the 2026-11-09 wave; no pending bump remains. The local checkout runs five commits past the tag (`broadcast/v0.7.0-5-g67d99c6`, 2026-11-10) with no newer tag — nothing in that delta is required by tq, which uses only root + static surfaces. Risk: LOW — loopback, read-only dashboard, single consumer of the bundle, serving contract now library-owned and header-matrix-pinned.

## 6. Conclusion

go-datastar's adoption in tq has matured from the 2026-11-08 edition's "six lines of hand-rolled handler code where the library ships a better one" to library-first serving: `ScriptHandler` owns the caching contract, `static.Version` owns the cache identity, the client bundle is current (v1.0.4), and every pin agrees at the v0.7.0 lockstep. The protocol vocabulary stays parked by ADR-0003 ruling — revisit `NewElementsPatch` + `broadcast/` only if the main dashboard ever trades payload-less ticks for fragment-level updates.

## How to verify

```bash
grep -rn 'go-datastar' --include=go.mod . | grep -v vendor          # root 0.7.0/static 0.7.0 + cmd/tq 0.7.0/static 0.7.0
grep -rl --include='*.go' 'larsartmann/go-datastar' . | grep -v '^./vendor/'   # health.go only
go mod why github.com/larsartmann/go-datastar                       # webui → go-datastar (direct since the wave)
grep -n 'Version =' vendor/github.com/larsartmann/go-datastar/static/static.go     # "1.0.4"
grep -n 'Version =' /home/lars/projects/go-datastar/static/static.go               # "1.0.4"
grep -rn 'SDKScript\|jsdelivr' internal/webui --include='*.go' --include='*.templ' | grep -v _templ.go   # only tq's own healthSDKScript identifiers
grep -n 'ETag' SECURITY.md | head -2                                # :147 — ScriptHandler ETag row
sed -n '442,452p' internal/webui/health.go                          # healthSDKSrc + healthSDKScript = datastar.ScriptHandler()
sed -n '319,324p' internal/webui/headermatrix_test.go               # serveSDKRow pins the ETag header
cd /home/lars/projects/go-datastar && git describe --tags           # broadcast/v0.7.0-5-g67d99c6
cd cmd/tq && GOWORK=off go mod tidy -diff 2>/dev/null | grep 'go-datastar'   # empty — no drift
GOFLAGS=-mod=mod go list -m all | grep 'go-datastar'                # root 0.7.0 + static 0.7.0, no broadcast
go test ./internal/webui -run 'TestServeHeaderMatrixSDKBundle' -count=1   # SDK route/header contract — run at HEAD (worktree): the working tree does not build while the in-flight sqlite v4→v5 migration is open
```
All commands above were run literally on 2026-11-11 and produced the cited outputs (greps against the working tree; guard test at HEAD `60c19070` via a throwaway `git worktree`: ok 0.006s).
