# go-datastar — Library Utilization Review

> **2026-11-22 refresh:** every pin, route cite, and test cite re-verified against HEAD `1c8219e6` (339 commits
> past the 11-11 tree `60c19070`; code identical through the daemon commits that landed during the refresh —
> they touch sibling reports and `internal/webui/theme.css` only). Library-side: root tag v0.7.0 unchanged
> (checkout root describe `v0.7.0-10-g24a808f`), but the static train moved — **new tag `static/v0.8.0`**
> (`f909928`, 2026-11-17; checkout HEAD `static/v0.8.0-2-g24a808f`, 2026-11-21). It is additive-only: +37
> lines in `static/static.go` exporting `Version1_0_4`/`Version1_0_3`/`Version1_0_2` and `VersionIsValid`;
> `Version` is still `"1.0.4"` and `datastar.js` is byte-identical to v0.7.0 (33 553 bytes; `cmp` against
> tq's vendored bundle → identical) — v0.7.0 already carried the 1.0.4 runtime (v0.6.1 was 1.0.3 / 33 538
> bytes), so what tq has not taken is two compile-time conveniences, not a runtime. `script_handler.go` is
> unchanged since the root tag. tq-side: pins unchanged — root `go.mod:107-108` + `cmd/tq/go.mod:75-76` at
> v0.7.0 — so the static pin is now ONE TAG BEHIND at zero functional cost; in the module graph
> `templ-components/datastar@v1.27.0` and `go-health-dashboard@v0.13.2` still REQUIRE `static v0.6.1` (the
> dashboard also root `v0.6.2`), and tq's direct v0.7.0 wins MVS for the whole build. templ-components
> v1.27.1 and v1.27.2 (the two newer tags in the local checkout) moved its datastar module to `static v0.8.0`; tq pins templ-components v1.27.0, so the static bump
> arrives with that sweep. The live budget meter (`83800183` + `56c1db19`, 2026-11-21, `tq serve
> --daily-budget`) adds ZERO Datastar surface: a server-rendered `role="meter"` span in the stats fragment
> (fragments.templ:72), recomputed per snapshot (render.go:937-943) and delivered by the existing go-sse
> tick→snapshot→`SendJSON("frag")` path — a grep for `data-signal|data-on-|@get|@post|signals.` across
> `internal/webui` returns nothing. **Score: unchanged 83 — high**, re-derived with one new N/A row for the
> v0.8.0 version constants (N=12, N/A=9, A=3; FULL=2, PARTIAL=1 → Σ=2.5/3). Guard test in-tree at HEAD:
> `TestServeHeaderMatrixSDKBundle` ok 0.011s. Verify-block repairs: `go list -m all` →
> `go list -mod=readonly -m all` (the bare form fails under the vendored root module), `sed` range and
> `describe` output re-pointed, `cd`-back lines added so the block replays sequentially.

> **2026-11-11 refresh:** the 2026-11-09 post-wave re-score (83) left sections 1-5 describing the
> pre-wave tree; this refresh reconciles the whole report with the wave as it actually landed and
> re-verifies every claim against HEAD `60c19070`. Verified: pins root + cmd/tq + static ALL at
> v0.7.0 with `go mod tidy -diff` clean; `datastar.ScriptHandler()` serving with the SECURITY.md
> ETag row (commit `29109c05`); versioned `healthSDKSrc`; `static` client v1.0.4 both sides. The
> band label is normalized to the repo rubric (83 = **high**, 70-84 — the 11-09 text said
> "moderate-high"). Guard test run at a throwaway HEAD worktree (working tree unbuildable —
> in-flight sqlite v4→v5 migration, unrelated to webui): ok 0.006s.

**Verdict:** ADOPTED — high (2.5/3 capability rows leveraged; re-scored 2026-11-09, re-verified 2026-11-11, re-derived 2026-11-22) — the v0.7.0 lockstep + ScriptHandler adoption flipped two MISSED rows and the cache-identity PARTIAL: bundle serving now rides the library's own handler.

**Post-wave delta (2026-11-09, plan M2 — landed in commits `29109c05`, `33689678`, `70284764`, re-verified 2026-11-22):** pins = root + cmd/tq + static ALL at
v0.7.0 (split healed; the static train has since cut an additive v0.8.0 upstream — see the 11-22 refresh); the `serveDatastarSDK` hand-roll is DELETED — the SDK is
served by `datastar.ScriptHandler()` (ETag/304/HEAD/405; the root SECURITY.md
matrix gained the ETag row at SECURITY.md:147, pinned by `serveSDKRow`
headermatrix_test.go:319-324) and the dashboard loads it via
`healthSDKSrc = HealthSDKPath + "?v=" + static.Version`
(health.go:593-597; the version query collapses the 24 h stale-client window; built inline
because the dashboard owns tag rendering — vendor go-health-dashboard/dashboard.go:150-152 copies
`Routes.DatastarJS` into `DatastarSrc` under `WithEmbeddedDatastarSDK` (tq sets it, health.go:548)
and vendor view_templ.go:291-295 renders the tag — `ScriptTagVersioned` itself
is unreachable from this integration shape, hence row 3 counts PARTIAL).
Re-score: row 1 PARTIAL→FULL (static.Version now the cache identity),
row 2 MISSED→FULL (ScriptHandler), row 3 MISSED→PARTIAL (cache-busting
outcome achieved; the named helper unused). Σ = 1 + 1 + 0.5 = 2.5/3 →
**83 — high** (was 17). N=11, N/A=8 at the re-score (12/9 since 2026-11-22, when one N/A row was added): the protocol
library (patches/broadcast/stores) stays deliberately untouched —
ADR-0003's payload-less tick design is not a Datastar-patch design.
**Pinned:** root `go.mod:107` `go-datastar v0.7.0` + `go.mod:108` `go-datastar/static v0.7.0` (both direct since the wave — internal/webui imports root AND static, health.go:22-23); `cmd/tq/go.mod:75-76` `v0.7.0` + `static v0.7.0 // indirect`; `GOWORK=off go mod tidy -diff` clean (re-verified 2026-11-22: rc 0, zero lines — the v0.6.3/v0.6.2/v0.6.1 drift described in the 2026-11-08 edition is gone).
**Latest:** root v0.7.0 (local checkout /home/lars/projects/go-datastar, checked 2026-11-22; tag 2026-11-07, four-module lockstep: root, static, broadcast, datastartest); static train: **static/v0.8.0** (2026-11-17, additive-only — named version constants + `VersionIsValid`, same 1.0.4 runtime bytes); checkout HEAD `static/v0.8.0-2-g24a808f` 2026-11-21 (root describe `v0.7.0-10-g24a808f`, ten commits past the root tag; broadcast + datastartest still at v0.7.0)
**Prior coverage:** none
**Reviewed:** 2026-11-08 by `grep -rl --include='*.go' 'larsartmann/go-datastar' . | grep -v '^./vendor/'`, `go mod why github.com/larsartmann/go-datastar{,/static}`, `go mod graph | grep ' github.com/larsartmann/go-datastar@'`, `go mod tidy -diff` in cmd/tq, vendor-vs-checkout diff of static/static.go, full reads of /home/lars/projects/go-datastar README.md + FEATURES.md + CHANGELOG.md §[0.7.0]/§[0.6.3] + broadcast/, `go test ./internal/webui -run 'TestServeHeaderMatrixSDKBundle' -count=1` (ok 0.007s). **Re-verified 2026-11-11** by the same greps against the current tree, checkout tag/HEAD checks, and `go test ./internal/webui -run 'TestServeHeaderMatrixSDKBundle' -count=1` at HEAD `60c19070` (ok 0.006s). **Re-verified 2026-11-22** by the same greps at HEAD `1c8219e6`, `go list -mod=readonly -m all` + `go mod graph`, per-train tag checks and a tag→HEAD diff of the checkout (`static/static.go`, `datastar.js`, `script_handler.go`), a `cmp` of the vendored vs checkout bundle, a Datastar-attribute sweep of `internal/webui` (budget meter included), and the guard test in-tree (ok 0.011s)

## 1. Usage inventory

Exactly one tq file imports go-datastar: `internal/webui/health.go` (verified 2026-11-22 by `grep -rl --include='*.go' 'larsartmann/go-datastar' . | grep -v '^./vendor/'`). Since the 2026-11-09 wave the import is BOTH submodules' surfaces: the root module for `ScriptHandler` and the `static` submodule for the bundle + version (health.go:22-23). tq serves the embedded Datastar runtime same-origin at `/health/datastar.js` via the library's own handler: `healthSDKScript = datastar.ScriptHandler()` (health.go:599-603) carries the ETag, conditional-GET/304, and HEAD-vs-405 contract; the route is registered GET-only under the health headers wrapper (webui.go:331, route table :329-337). The URL handed to go-health-dashboard is versioned: `healthSDKSrc = HealthSDKPath + "?v=" + static.Version` (health.go:593-597), configured as `DatastarJS: healthSDKSrc` (health.go:531), and the dashboard renders the script tag from it (vendor go-health-dashboard/dashboard.go:150-152 → view_templ.go:291-295) — same-origin per the go-health-dashboard adoption posture, never the jsDelivr CDN (the library's `SDKScript`/CDN default appears nowhere in tq code; the only grep hits are tq's own `healthSDKScript` identifier and the webui.go:331 route row — verified 2026-11-22).

The root module is now a DIRECT dependency (no longer transitive via go-health-dashboard): `go mod why` → `internal/webui → go-datastar` and `internal/webui → go-datastar/static` (verified 2026-11-22). tq emits no Datastar patches and accepts no inbound Datastar requests; the only Datastar-speaking page is go-health-dashboard's `/health` (its `/health/sse` is mounted at webui.go:330). The 2026-11-21 budget meter changes none of this: it is a templ-rendered `<span role="meter" aria-valuemin aria-valuemax aria-valuenow aria-valuetext>` inside the stats fragment (`StatusCards`, fragments.templ:35; the meter itself at :72), fed from `BudgetView{Cap, Spent}` computed per snapshot (render.go:937-943, `Config.DailyBudget` ← `tq serve --daily-budget`) — no signals, no `data-*` attributes, re-rendered on the same go-sse tick as every other fragment.

Integration map (all rows verified 2026-11-22 against internal/webui):

| Route | tq wiring | Library surface |
| --- | --- | --- |
| `GET /health` | webui.go:329 → `s.dash.Handler()` | go-health-dashboard rendering (Datastar markup), path config health.go:520-532 |
| `GET /health/sse` | webui.go:330 → `s.dash.SSEHandler()` | go-health-dashboard's patch emission over go-sse |
| `GET /health/datastar.js` | webui.go:331 → `healthSDKScript` | `datastar.ScriptHandler()` (health.go:599-603) over `go-datastar/static` |
| `GET /healthz /readyz /startupz` | webui.go:335-337 → queueProber | none (go-health, outside this review's scope) |

Not in the tq graph: `datastartest` (test-only module) and `broadcast/` (verified 2026-11-22 by `go list -mod=readonly -m all | grep go-datastar` → only root + static; `go mod graph` carries no `broadcast/` edge).

## 2. Capability inventory

| Capability | Status | Evidence |
| --- | --- | --- |
| Embedded runtime bundle (`static.Bytes()`/`static.Version`) | 🟢 FULL | bundle served same-origin via the lib handler; `static.Version` is the cache identity of the served URL (`healthSDKSrc`, health.go:593-597); vendored static.go:21 = `"1.0.4"` — client CURRENT (re-verified 2026-11-22): the newest static tag (v0.8.0) ships the same `"1.0.4"` and a byte-identical `datastar.js` (`cmp` vendored vs checkout → identical, 33 553 bytes) |
| `ScriptHandler` (ETag + Cache-Control + 304 revalidation + HEAD/405 contract) | 🟢 FULL | `healthSDKScript = datastar.ScriptHandler()` (health.go:599-603) on the GET-only route (webui.go:331) under `withHealthHeaders`; ETag shape pinned by `serveSDKRow`/`quotedETag` (headermatrix_test.go:319-335) and the SECURITY.md matrix row (SECURITY.md:147) — adopted 2026-11-09, commit `29109c05` |
| `ScriptTagVersioned` + `ETagFor` cache-busting (new in v0.7.0) | 🟡 PARTIAL | the cache-busting outcome is achieved by the inline `?v=static.Version` query (health.go:593-597) and `ETagFor`'s digest shape is pinned by `quotedETag` (headermatrix_test.go:326-335); the named helper itself is unreachable — the tag is rendered by go-health-dashboard from the path tq passes (health.go:531 → vendor dashboard.go:150-152 → view_templ.go:291-295) |
| Named runtime version constants + `VersionIsValid` (new in static v0.8.0) | ⚪ N/A | tq pins `static v0.7.0`, which predates them (vendored static.go is 29 lines; the checkout's is 66); nothing in tq validates a runtime version string — the `?v=` query takes `static.Version` directly (health.go:597), so the one-tag lag has no consumer |
| Patch vocabulary (`ElementsPatch`/`SignalsPatch`/`ScriptPatch` + sugar) and `Response` builder | ⚪ N/A | ADR-0003: dashboard live updates are payload-less ticks driving full server-rendered snapshots over go-sse (hub.go:15-38, handlers.go:343-368/383-435); tq renders no `data-*` Datastar attributes anywhere — the 2026-11-21 budget meter included, which is a plain server-rendered `role="meter"` span (fragments.templ:72) |
| `broadcast/` submodule (Hub, `http.Handler` fan-out, replay, heartbeat options) | ⚪ N/A | no Datastar patch stream exists to fan out; tq's payload-less fan-out lives directly on go-sse `Broadcaster[sse.Event]` (hub.go:20-22) |
| `MemoryStore`/`EventStore` replay | ⚪ N/A | snapshot-resume by design (handlers.go:344-363, 383-401); there is no patch log to replay |
| Inbound helpers (`ReadSignals`, `LastEventID`) | ⚪ N/A | loopback read-only UI: no `datastar=` query strings or signal bodies arrive (writes are CSRF+lockout-guarded HTML forms per SECURITY.md posture) |
| `datastartest` module | ⚪ N/A | tq owns no Datastar-emitting endpoints; `/health/sse` is go-health-dashboard's contract — tq tests only mounting/headers (health_test.go:93, 366-369, 392-393; headermatrix_test.go:521, 550-562, 658-662) |
| HTTP attribute helpers (`GetSSE`/`PostSSE`/`PatchSSE`…) | ⚪ N/A | generate `@get`/`@post`-style attribute strings for datastar-marked elements — tq renders none |
| Error system (typed `errorfamily` codes) | ⚪ N/A | zero tq call sites (verified by grep); errors surface inside go-health-dashboard's own handlers |
| `version` package (`-ldflags` injection) | ⚪ N/A | binary versioning is tq's own plumbing; the package is example-binary-oriented (version.go doc comment) |

## 3. Findings

1. **~~`/health/datastar.js` serves without an ETag~~ RESOLVED 2026-11-09 (commit `29109c05`).** The hand-rolled `serveDatastarSDK` was deleted; the route now binds `healthSDKScript = datastar.ScriptHandler()` (internal/webui/health.go:599-603) inside the security-headers wrapper (webui.go:331) — exactly the example from the 2026-11-08 edition, with the caveat honored: `withHealthHeaders` stays for the security posture, the lib handler owns ETag/304/HEAD, and the merged header set is pinned by `serveSDKRow` (headermatrix_test.go:319-324, `TestServeHeaderMatrixSDKBundle` :550 — green in-tree at HEAD `1c8219e6`, ok 0.011s). Operators now get 304 revalidation instead of re-downloading the 33.5 KB bundle after every 86 400 s freshness window.
2. **~~Client bundle one release behind, and unversioned~~ RESOLVED 2026-11-09 (commits `29109c05`/`70284764`).** `static` is pinned at v0.7.0 (root go.mod:108), so the vendored client is v1.0.4 (vendor static.go:21, 33 553-byte datastar.js — byte-identical to the checkout at `24a808f`, verified 2026-11-22 by `cmp`), and the served URL carries `?v=static.Version` (health.go:593-597), collapsing the 24 h stale-client window to zero. Boundary note stands: the `<script>` tag is rendered by go-health-dashboard from the path tq passes (health.go:531 → vendor dashboard.go:150-152 → view_templ.go:291-295), so `ScriptTagVersioned` itself remains unused — the PARTIAL in row 3.
3. **~~Version split root v0.6.3 vs cmd/tq v0.6.2 — tidy drift~~ RESOLVED 2026-11-09 (commit `70284764`).** All pins — root `go.mod:107-108`, `cmd/tq/go.mod:75-76`, and the vendored module — sit at v0.7.0, the four-module lockstep cut (CHANGELOG §[0.7.0]: root, static, broadcast, datastartest tagged on one commit, repairing v0.6.3's root-only defect). `cd cmd/tq && GOWORK=off go mod tidy -diff` is clean (re-verified 2026-11-22, exit 0, no larsartmann lines). The BREAKING v0.7.0 `ScriptHandlerWith(bundle)` signature drop cost tq nothing — the wave adopted the new no-arg `ScriptHandler()` (health.go:603).
4. **Double-shipping check: none (re-verified 2026-11-22).** `templ-components/datastar v1.27.0` (in the graph — root go.mod:65, indirect) and `go-health-dashboard v0.13.2` both REQUIRE `static v0.6.1` (the dashboard also root `v0.6.2`), while tq requires both modules at v0.7.0 directly — MVS selects v0.7.0 once for the whole build (`go mod graph`: tq → `go-datastar@v0.7.0` + `static@v0.7.0`; `go-datastar@v0.7.0` → `static@v0.7.0`; the downstream templ-components root + datastar modules and go-health-dashboard → `static@v0.6.1`), so the vendor tree carries exactly one embedded bundle (`ls vendor/github.com/larsartmann/go-datastar/static/` — one datastar.js, 33 553 bytes, Version "1.0.4"); tq never renders the library's `SDKScript` (whose default is the CDN), so no second runtime ships and no cross-origin script enters the CSP.
5. **Pin currency: `static` is one tag behind, additively (2026-11-22).** `static/v0.8.0` (`f909928`, 2026-11-17) adds only the named constants `Version1_0_4`/`Version1_0_3`/`Version1_0_2` and `VersionIsValid(v string) bool` to `static/static.go` (29 → 66 lines); `const Version = "1.0.4"` and `datastar.js` are unchanged from v0.7.0 (`git diff --stat static/v0.7.0..static/v0.8.0 -- static/datastar.js` → empty). tq uses `static.Version` and the lib handler only, so there is nothing to adopt; the bump will arrive by itself when templ-components moves past v1.27.0 (its v1.27.1/v1.27.2 datastar modules require `static v0.8.0`). The root, `broadcast/`, and `datastartest/` trains have no newer tag than v0.7.0, so the v0.7.0 "four-module lockstep" statement stays true for them — the lockstep is simply no longer four-way at the static edge.
6. **Architecture note (unscored).** The library's core idea — patches are values (`Patch.Event() sse.Event`) that can be broadcast, stored, filtered — maps one-to-one onto tq's hub model. If the main dashboard ever moves from payload-less ticks + full snapshots to fragment-level updates, `NewElementsPatch` + templ-components' Datastar wave is the natural path; today that stays deliberately out of scope per ADR-0003 — and the budget meter, the newest live widget, deliberately took the snapshot route (§1).

## 4. Score + top opportunities

N=12 rows; N/A=9; A=3; FULL=2, PARTIAL=1, MISSED=0 → Σ=2.5 → score = round(100×2.5/3) = **83 — high** (repo rubric: very high ≥85, high 70-84 — the 2026-11-09 note's "moderate-high" label normalized here; was 17 on 2026-11-08; unchanged on 2026-11-22 — the re-derivation's added row is N/A, so A and Σ are untouched). tq now uses go-datastar as more than a same-origin JS-bundle vendor: the library owns the serving contract (ETag/304/HEAD) and its version const is the cache identity. What remains untouched is the protocol library itself (patches, response builder, broadcast, test helpers) — by ADR-0003 ruling, not by accident. Top: (1) nothing required today — let the static v0.8.0 pin ride the next templ-components sweep (its v1.27.1 and v1.27.2 tags require it, so MVS raises it with no tq code change); (2) if the main dashboard ever moves to fragment-level updates, `NewElementsPatch` + `broadcast/` is the path (ADR-0003 watch item); (3) keep the cmd/tq tidy-clean state green (it drifted once already).

## 5. Version currency + risk

Current: root, cmd/tq, static, and vendor ALL at v0.7.0 (verified 2026-11-22 by go.mod reads, `go list -mod=readonly -m all`, and `go mod tidy -diff` in cmd/tq — clean). Root v0.7.0 is still the newest root tag; the `static` submodule alone has moved on — `static/v0.8.0` (2026-11-17) is additive-only (finding 5), so tq's lag there is a currency blemish, not a behavior gap: the served runtime bytes are identical. The 2026-11-08 upgrade mechanics (pin bump → `go mod vendor` → VendorHash sync) were executed by the 2026-11-09 wave; no pending bump is forced. The local checkout HEAD is `static/v0.8.0-2-g24a808f` (2026-11-21; root describe `v0.7.0-10-g24a808f`) — the two commits past static/v0.8.0 are workflow, flake, and go.mod chores plus one `static_test.go` line, and the root's ten post-tag commits change no root Go source (`script_handler.go`, `script.go` identical to v0.7.0); nothing in that delta is required by tq, which uses only root + static surfaces. Risk: LOW — loopback, read-only dashboard, single consumer of the bundle, serving contract library-owned and header-matrix-pinned.

## 6. Conclusion

go-datastar's adoption in tq has matured from the 2026-11-08 edition's "six lines of hand-rolled handler code where the library ships a better one" to library-first serving: `ScriptHandler` owns the caching contract, `static.Version` owns the cache identity, the client bundle is current (v1.0.4 — the same bytes the newest static tag ships), and every tq pin agrees at v0.7.0 (the static train's additive v0.8.0 is the one tag not yet taken, and it has no consumer here). The protocol vocabulary stays parked by ADR-0003 ruling — revisit `NewElementsPatch` + `broadcast/` only if the main dashboard ever trades payload-less ticks for fragment-level updates; the 2026-11-21 budget meter shows the snapshot route still absorbing new live widgets without it.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
grep -rn 'go-datastar' --include=go.mod . | grep -v vendor          # root 0.7.0/static 0.7.0 + cmd/tq 0.7.0/static 0.7.0
grep -rl --include='*.go' 'larsartmann/go-datastar' . | grep -v '^./vendor/'   # health.go only
go mod why github.com/larsartmann/go-datastar                       # webui → go-datastar (direct since the wave)
go mod why github.com/larsartmann/go-datastar/static                # webui → static
go list -mod=readonly -m all | grep 'go-datastar'                   # root 0.7.0 + static 0.7.0 only — no broadcast/datastartest (bare form fails under the vendor dir)
go mod graph | grep -E ' github.com/larsartmann/go-datastar(/static)?@' | awk '{print $1" -> "$2}' | sort -u   # who requires what: tq → v0.7.0 (both modules); go-datastar@v0.7.0 → static@v0.7.0; templ-components@v1.27.0 + templ-components/datastar@v1.27.0 → static@v0.6.1; go-health-dashboard@v0.13.2 → static@v0.6.1 + root@v0.6.2
grep -n 'Version =' vendor/github.com/larsartmann/go-datastar/static/static.go     # :21 "1.0.4"
grep -n 'Version =' /home/lars/projects/go-datastar/static/static.go               # :21 "1.0.4"
cmp vendor/github.com/larsartmann/go-datastar/static/datastar.js /home/lars/projects/go-datastar/static/datastar.js && echo identical   # identical — v0.8.0 ships the same runtime bytes
grep -rn 'SDKScript\|jsdelivr' internal/webui --include='*.go' --include='*.templ' | grep -v _templ.go   # only tq's own healthSDKScript identifiers (+ the webui.go:331 route row)
grep -rnE 'data-signal|data-on-|@get|@post|signals\.' internal/webui --include='*.templ' --include='*.go' | grep -v _templ.go   # no matches — zero Datastar attributes on tq's own pages (budget meter included)
grep -n 'ETag' SECURITY.md | head -2                                # :147 — ScriptHandler ETag row
sed -n '593,603p' internal/webui/health.go                          # healthSDKSrc + healthSDKScript = datastar.ScriptHandler()
sed -n '319,324p' internal/webui/headermatrix_test.go               # serveSDKRow pins the ETag header
cd /home/lars/projects/go-datastar && git describe --tags           # static/v0.8.0-2-g24a808f
git -C /home/lars/projects/go-datastar tag -l 'static/*' | sort -V | tail -2   # static/v0.7.0 then static/v0.8.0
git -C /home/lars/projects/go-datastar diff --stat static/v0.7.0..static/v0.8.0 -- static/datastar.js static/static.go   # datastar.js unchanged; static/static.go +37
cd /home/lars/projects/go-taskqueue/cmd/tq && GOWORK=off go mod tidy -diff 2>/dev/null | grep 'go-datastar'   # empty — no drift
cd /home/lars/projects/go-taskqueue && go test ./internal/webui -run 'TestServeHeaderMatrixSDKBundle' -count=1   # SDK route/header contract — in-tree at HEAD 1c8219e6, 2026-11-22: ok
```
All commands above were run literally on 2026-11-22 against HEAD `1c8219e6` (go1.27.4; `go env GOEXPERIMENT`=jsonv2, no root go.work) and produced the cited outputs — in-tree, no worktree: the package builds (guard test ok 0.011s on the ship-gate sequential replay, 0.007s on the first in-tree run; the whole `internal/webui` package ok 15.578s). Block repairs this refresh: `go list -m all` gained `-mod=readonly` (the bare form now errors out under the vendored root module; `-mod=mod` stays excluded for the 11-12 lint-leak reason — re-checked, `-mod=readonly` left `git status` unchanged), the `sed` range moved with `health.go` (442-452 → 593-603), the `describe` comment was updated for the static tag, bundle-parity / attribute / tag commands were added, and the `cd` lines were made absolute with a `cd` back so the block replays sequentially in one shell. Daemon commits that landed during the refresh (at least `b1984388`, `f19acb32`) touch only sibling reports and `internal/webui/theme.css` — `git diff --name-only 1c8219e6..HEAD | grep -v '^docs/'` prints just that CSS file — so the Go tree is identical to `1c8219e6`. History: 2026-11-11 the block ran against `60c19070` (guard test via a throwaway `git worktree` while the in-flight sqlite v4→v5 migration had the working tree unbuildable — ok 0.006s); 2026-11-12 re-executed in-tree after the facade repair (5e7fc982), guard test ok.
