# go-health-dashboard — Library Utilization Review

**Verdict:** ADOPTED — high (15/19 applicable capability rows leveraged; was 13/19 before the 2026-11-09 landing): the operator-facing core is wired exactly as the library intends, and the two highest-value observability extras — webhook transition alerts (loopback relay → PapDashboard) and the trend/history/export ring — are now ON; only metrics and introspection stay deliberately off.

**2026-11-11 refresh (re-score pending resolved):** findings 1 and 2 LANDED — `WithWebhook(webhookURL)` passed when `--alert-url` is set (internal/webui/health.go:407-409; the loopback HealthRelay translates snapshots into PapDashboard alerts, webui.go:218) and `WithTrend(healthTrendSamples)` + the Trend/Export routes are mounted (health.go:400, webui.go:326-327). Finding 4's data gap half-closed via the go-health landing (`StatusSince` + `TotalLatencyMs` now populate the since column and latency card; `DurationNanos` still unset). Score re-derived: 68 → **79 (high)**. All line cites re-anchored against HEAD 2026-11-11; checkout re-checked (v0.13.2 = latest tag). `/metrics` remains deliberately OFF.

**Pinned:** v0.13.2 — root `go.mod:111` (direct); `cmd/tq/go.mod:80` (indirect). Re-verified 2026-11-11 by `grep -rn 'go-health' --include=go.mod . | grep -v vendor`.

**Latest:** v0.13.2 (local checkout /home/lars/projects/go-health-dashboard, re-checked 2026-11-11; HEAD = v0.13.2-26 — docs, CI permissions, dependency-pin restores, and a NixOS module for the separate health-hub binary; no library API change in [Unreleased]).

**Prior coverage:** none (AGENTS.md §Relation to other projects records the adoption decision; no prior utilization review).

**Reviewed:** 2026-11-08 by reading /home/lars/projects/go-health-dashboard/{README.md,CHANGELOG.md,dashboard.go,routes.go,options.go,handlers.go,csp.go,status.go,pusher.go,evidence.go,view.templ} plus /home/lars/projects/go-taskqueue/internal/webui/{health.go,webui.go,health_test.go} and cmd/tq/main.go:4300-4372; greps and tests in §How to verify.

## 1. Usage inventory

Import sites (re-verified 2026-11-11 by `grep -rn 'go-health-dashboard' --include='*.go' . | grep -v '^./vendor/'`): `internal/webui/health.go:25`, `internal/webui/webui.go:29`.

Symbols consumed (re-verified 2026-11-11 by `grep -oh 'dashboard\.[A-Za-z]*' internal/webui/*.go | sort -u`): `New`, `Prober`, `Dashboard`, `Option`, `Routes`, `RecommendedCSP`, `WithTitle`, `WithRoutes`, `WithEmbeddedDatastarSDK`, `WithCSSPath`, `WithNonceExtractor`, **`WithTrend`, `WithWebhook`** + `HealthRelay`/`NewHealthRelay`/`Config` (the loopback alert relay) + methods `Handler`, `SSEHandler`, `FaviconHandler`, `TrendHandler`, `ExportHandler`, `Start`, `Shutdown`.

Wiring shape:

- Construction: `newHealthDashboard` (health.go:385-413) passes title, `WithRoutes` (Dashboard/SSE/Favicon/probes/DatastarJS + **Trend/Export now enabled**, health.go:394-401; **Metrics/Introspect still `""`**), `WithEmbeddedDatastarSDK`, `WithCSSPath("/static/app.css")`, **`WithTrend(healthTrendSamples)`** (health.go:400), `WithNonceExtractor` (per-request nonce from tq's security-headers middleware), and — when `--alert-url` is set — **`WithWebhook(webhookURL)`** (health.go:407-409).
- Alert relay: the webhook target is a loopback `papdashboard.HealthRelay` (webui.go:199-236) that translates each transition snapshot into PapDashboard alerts, driven by the same `--alert-url/--alert-api-key` flags as the journal-fact bridge (cmd/tq/main.go:648).
- Mount: `healthBindings` (webui.go:312-328) registers /health, /health/sse, /health/datastar.js, /health/favicon.svg, **/health/trend, /health/export**, /healthz, /readyz, /startupz — every route wrapped in `withHealthHeaders` (strictened CSP + `X-Robots-Tag: noindex`, health.go:419-445), all GET-only, all inside `withTokenAuth` + `withSecurityHeaders` (webui.go:377-431). 401 without token is test-pinned (health_test.go:208-228) — the AGENTS.md "inside the token gate" contract.
- Lifecycle: `dash.Start(ctx)` before serving (webui.go:515), `dash.Shutdown()` after HTTP drain (webui.go:585). Push interval resolves from the prober's 5 s `RefreshInterval` (health.go:48, 109-121, 143 — "one ladder").

## 2. Capability inventory

| Capability | Status | Evidence |
| --- | --- | --- |
| HTML dashboard page (severity-grouped triage, banner, badges, filter, connection pill) | 🟢 FULL | health.go:385-413; healthBindings webui.go:322; page body test-pinned health_test.go:47-94 |
| Content negotiation on /health (HTML vs `Accept: application/json`) | 🟢 FULL | `dash.Handler()` unmodified (handlers.go:23-53) |
| Realtime SSE + pusher lifecycle (`Start`/`Shutdown`) | 🟢 FULL | webui.go:447-451, 519; SSE route mounted (health_test.go:261-265) |
| Push cadence resolved from probe `RefreshInterval` | 🟢 FULL | dashboard.go:187-201 + health.go:106-108 (5 s) |
| Same-origin embedded Datastar SDK | 🟢 FULL | `WithEmbeddedDatastarSDK` + `Routes.DatastarJS` (health.go:402, 404), versioned URL health.go:451-453, asset test health_test.go:230-249 |
| CSS theming via host stylesheet | 🟢 FULL | `WithCSSPath("/static/app.css")` (health.go:403) — tq's own compiled theme |
| CSP integration (nonce extractor + `RecommendedCSP`) | 🟢 FULL — and strictened | `WithNonceExtractor` (health.go:402); `withHealthHeaders` composes the library CSP with `base-uri 'none'` + `frame-ancestors`/`form-action 'none'` (health.go:419-445), regression-pinned health_test.go:60-87 |
| Route table customization (`WithRoutes`, disable-able endpoints) | 🟢 FULL | health.go:389-401 — favicon namespaced under /health; Metrics + Introspect deliberately disabled (see rows below); Trend/Export now enabled |
| Favicon route | 🟢 FULL | webui.go:324 (`/health/favicon.svg`), content-type test health_test.go:251-259 |
| Failure-evidence strip (proven vs unproven greens) | 🟢 FULL | zero-config: pusher accrues evidence every tick regardless of PushOnChange (pusher.go:120-127; evidence.go:175-216) |
| Per-check `since (17m)` + duration row metadata | 🟡 PARTIAL | rendering ships (status.go:289, 941-952; view.templ:499-506); the prober now feeds `StatusSince` (health.go:305-312, 2026-11-09 landing) and `TotalLatencyMs` (health.go:323) — `DurationNanos` still unset (mkCheck, health.go:288-290) — see the go-health report, finding 2 |
| Prometheus metrics endpoint (`WithMetrics`) | 🔴 MISSED | `Routes.Metrics: ""` (health.go:397); options.go:172-181, metrics.go — queue health has no scrape surface anywhere |
| Trend sparkline + history + JSON/CSV export (`WithTrend`, `WithTimelineMaxAge`) | 🟢 FULL — landed 2026-11-09 | `WithTrend(healthTrendSamples)` (health.go:400) + Trend/Export routes mounted (webui.go:326-327, behind the same token gate) — health now has memory across restarts of the page, and export answers "since when is it flapping" |
| Webhook transition alerts (`WithWebhook`/headers/HMAC) | 🟢 FULL — landed 2026-11-09 | `WithWebhook(webhookURL)` when `--alert-url` set (health.go:407-409) → loopback `papdashboard.HealthRelay` (webui.go:218) → PapDashboard alerts; the "database failing / fold wedged" classes now reach the operator's alert dashboard. (HMAC signing not configured — the relay is loopback-only, same trust domain) |
| Introspection endpoint (`WithIntrospection`) | 🔴 MISSED | `Routes.Introspect: ""` (health.go:398); deliberate min-surface — cheap to enable behind the token gate if /health misconfiguration ever needs debugging |
| Probe-cadence ingest (`WithEvaluationHook` → `dash.Observe`) | ⚪ N/A | requires a go-health evaluation hook; tq's hand-rolled prober has none (dashboard.go:354-381 offer) — blocked on the go-health side |
| `WithMiddleware` auth wrapper | ⚪ N/A | tq's `withTokenAuth` wraps the WHOLE mux incl. probes (webui.go:358-364; health_test.go:208-228) — strictly stronger than the library's probes-excluded design (README "Protecting the Dashboard"); correct for tq's no-unauthenticated-oracle posture |
| Shared rate limiting (`WithRateLimit`) | ⚪ N/A | loopback-default + token gate; write routes carry their own 3-strikes limiter (webui.go:348-356) |
| `WithPublicMode` anonymization | ⚪ N/A | token-gated surface, no untrusted audience; additionally `X-Robots-Tag: noindex` (health.go:431) |
| Grouping axis (severity vs source) | 🟢 FULL | default `GroupBySeverity` (dashboard.go:141) is correct — single-source page; GroupBySource is for aggregate/federation pages tq does not run |
| Healthy-group collapse controls (`WithHealthyGroupCollapse`/`WithPersistCollapse`/`WithHealthyGroupExpanded`) | 🟡 PARTIAL | default threshold 8 active (dashboard.go:140); `WithPersistCollapse` (options.go:209) not opted in — per-browser choice lost on reload |
| SSE connection management (`MaxSSEConnections`, `RetryInterval`, `HeartbeatInterval`, `MaxConnectionLifetime`, `ShutdownDrain`) | 🟡 PARTIAL | defaults only; heartbeat default 15 s (options.go:21) matches webui's own `DefaultHeartbeat` — coherent; no connection cap/lifetime/drain for a loopback-first surface (acceptable, but unbounded by choice, not decision record) |
| Pusher self-health (`HealthCheck` watchdog, `SubscriberCount`, `DroppedPatchCount`) | ⚪ N/A | no samber/do container to cascade into; feeding it into /health would violate tq's store-derived-only checks doctrine (health.go:4-7 "never from process state") |
| Title/description branding | 🟡 PARTIAL | `WithTitle` set (health.go:390); `WithDescription` (options.go:277 — meta description + OG tags) skipped, moot under noindex |

## 3. Findings

1. **~~Health transitions never reach the operator's alert dashboard~~ RESOLVED (2026-11-09).**
   Offer: `WithWebhook(url)` POSTs a JSON snapshot on every status change (options.go:303-345; pusher.go:140-144).
   Landed: `newHealthDashboard` appends `dashboard.WithWebhook(webhookURL)` when `--alert-url` is set (health.go:407-409); the URL is a loopback `papdashboard.HealthRelay` (webui.go:199-236, built webui.go:218) that translates each snapshot into PapDashboard alerts — sharing the journal bridge's flags (cmd/tq/main.go:648). The fold-follower demotion (finding 5) now alerts exactly on the transition it produces.
   Not taken (deliberate): `WithWebhookHeaders`/`WithWebhookHMAC` — the relay is loopback-only, same trust domain; the finding's example composed a Bearer header for a cross-boundary target.
   ```go
   // health.go:407-409 — current code
   if webhookURL != "" {
       opts = append(opts, dashboard.WithWebhook(webhookURL))
   }
   ```
2. **~~No memory: trend, history, export are all off~~ RESOLVED (2026-11-09).**
   Offer: `WithTrend(n)` renders a status sparkline, records transitions, and exposes `/health/trend` + `/health/export` (JSON/CSV) — options.go:183-193, README "Health Trend".
   Landed: `WithTrend(healthTrendSamples)` (health.go:400) and the routes mounted in healthBindings (webui.go:326-327: `dash.TrendHandler()` / `dash.ExportHandler()`, same token gate + strictened CSP as every other /health route). A `tq serve` restart no longer amnesia-wipes the "since when is it flapping" answer, and the export endpoint carries the history out.
3. **No metrics surface for queue health (STILL OPEN, deliberate).**
   Offer: `WithMetrics(true)` serves a zero-dependency Prometheus exposition at `/health/metrics` (options.go:172-181, metrics.go).
   We do instead: `Routes.Metrics: ""` (health.go:397); tq has no Prometheus surface at all, so external monitors can only poll `/readyz`'s 200/503 bit.
   Impact: low-to-medium (tq is a local-first tool); the option stays one flag away, token-gated like every other route. Recorded as the only MISSED observability row left.
4. **~~The page's richest per-row feature renders empty~~ HALF-RESOLVED (2026-11-09, at the go-health seam).**
   Offer: rows show "since 14:02:05 UTC (17m)" and per-check duration (status.go:941-952; view.templ:499-506) when the prober populates `Check.StatusSince`/`DurationNanos`, plus the latency stat card fed by `Response.TotalLatencyMs` (pusher.go:138-140).
   Landed: `StatusSince` stamped per check (health.go:305-312) and `TotalLatencyMs` per batch (health.go:323) — the since column and latency card now render probe-observed truth. Still open: `DurationNanos` (mkCheck, health.go:288-290), so the per-row duration stays empty (go-health report, finding 2's remainder).
5. **~~Follower demotion shows on the page but nothing downstream reacts~~ RESOLVED (via finding 1's landing).**
   `MarkFoldFollower` (webui.go:265-275) drops the evaluation cache so the next push carries the follower warning, and the webhook now fires on exactly that transition — a demoted serve reaches the alert relay, not just connected browsers.
6. **`ShutdownDrain` 0 means SSE clients are dropped the instant `dash.Shutdown` runs** (dashboard.go:249-268; webui.go:585 calls it after the HTTP drain, which already waited — so real impact is near-zero; recorded for completeness, not action).

## 4. Score + top opportunities

Inputs (re-derived 2026-11-11): N=24 rows; ⚪ N/A=5 (Observe, WithMiddleware, rate limit, PublicMode, pusher self-health); A=19. FULL=13 × 1 (the 11 original + trend/history/export + webhook alerts), PARTIAL=4 × 0.5 (row metadata — since/latency feed, duration still empty; collapse persistence; SSE tuning; branding), MISSED=2 × 0 (metrics, introspection) → Σ=15.0. Score = round(100 × 15 / 19) = **79 → high** (was 68 → moderate before the 2026-11-09 landing).

Top opportunities: (1) optionally `WithMetrics(true)` — now the only MISSED observability row (finding 3); (2) `DurationNanos` in `mkCheck` to finish the row-metadata PARTIAL (finding 4's remainder, cross-report with go-health); (3) `WithIntrospection` if /health debugging ever needs it. (Former #1 WithWebhook and #2 WithTrend both landed 2026-11-09.)

## 5. Version currency + risk

- **Current**: v0.13.2 is the latest tag; the checkout is ~26 commits past it with no library-surface change — [Unreleased] holds docs work, a CI permissions block, restored dependency pins (chromedp/go-sse/go-datastar/a-h/parse), and a NixOS module for the health-hub binary (CHANGELOG [Unreleased], re-checked 2026-11-11 by `git log --oneline v0.13.2..HEAD | wc -l`).
- **Risk**: low-to-medium. The stated compatibility policy allows breaking changes in minors only with a CHANGELOG note, with deprecations running one minor ahead (README "Compatibility Policy", [0.13.2] section); support line is the latest minor, which tq is on. tq's usage is concentrated in the stable core (`New`, `Routes`, `Handler`/`SSEHandler`/`FaviconHandler`, `RecommendedCSP`) and does not touch the churnier extras — adopting them (findings 1-3) widens the API surface tq exposes itself to, worth doing knowingly. Vendored copy present (`vendor/github.com/larsartmann/go-health-dashboard`).
- Build note: the library requires `GOEXPERIMENT=jsonv2` (README "Build") — tq already exports it for its gates, so no action.

## 6. Conclusion

go-health-dashboard is adopted the way it was meant to be: a single `dashboard.New` call, tq's own CSS and nonce, same-origin SDK, every route inside the token gate, and the security posture strengthened past the library's own recommendation — and since 2026-11-09 the page also FEEDS the operation: transitions alert PapDashboard through the loopback webhook relay, and the trend ring keeps history the live page cannot. Utilization moved 68 → 79. What is left is deliberate min-surface (metrics, introspection) and one prober data fix (per-check duration); none require re-architecting the mount.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
grep -rn 'larsartmann/go-health-dashboard' --include='*.go' . | grep -v '^./vendor/'
# → internal/webui/health.go:25, internal/webui/webui.go:29 (health.go:1 is the package comment)
grep -rn 'go-health' --include=go.mod . | grep -v vendor
# → go.mod:111 v0.13.2 (direct), cmd/tq/go.mod:80 (indirect)
grep -oh 'dashboard\.[A-Za-z]*' internal/webui/*.go | sort -u
# → … incl. WithTrend + WithWebhook + HealthRelay/NewHealthRelay/Config (the 2026-11-09 landings)
grep -n 'WithTrend\|WithWebhook\|Metrics:\|Introspect:\|Trend:\|Export:' internal/webui/health.go
# → health.go:397-404 Routes (Metrics/Introspect "", Trend/Export set), :400 WithTrend, :407-409 WithWebhook
sed -n '385,413p' internal/webui/health.go   # the full construction: title, routes, SDK, CSS, trend, nonce, webhook
sed -n '312,328p' internal/webui/webui.go    # healthBindings: all routes wrapped, GET-only, incl. trend + export
sed -n '377,431p' internal/webui/webui.go    # token gate + security headers wrap the health routes too
GOCACHE=/tmp/go-build-cache GOEXPERIMENT=jsonv2 GOWORK=off \
  go test ./internal/webui/ -run TestHealth -count=1 -v | grep -c '^--- PASS'   # blocked 2026-11-11: the in-flight v4→v5 facade migration breaks the vendored root build; re-run after it lands
cd /home/lars/projects/go-health-dashboard && git describe --tags   # → v0.13.2-26-ish, no library-surface change
git log --oneline v0.13.2..HEAD | wc -l
grep -n 'WithWebhook\b\|WithTrend(\|WithMetrics(' options.go          # → 172/183/303 = the funcs (293/325 are doc comments)
```

All commands above ran clean on 2026-11-08 (9/9 health tests PASS then, per the tee'd log) and were re-run 2026-11-11 except the test line, which is build-blocked by the UNCOMMITTED v4→v5 facade migration in `vendor/` (concurrent session's in-flight work — see go-cqrs-lite.md's refresh note); the landing was verified by grep against committed sources instead.
