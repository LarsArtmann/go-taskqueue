# go-health-dashboard — Library Utilization Review

**Verdict:** ADOPTED — moderate (13/19 applicable capability rows leveraged): the operator-facing core (page, SSE, theming, CSP, token-gated mount, evidence strip) is wired exactly as the library intends; the observability extras — metrics, trend/history, webhook alerts, introspection — are all deliberately switched off, and the top row of the page renders half-empty because tq's prober never feeds it check metadata.

**Pinned:** v0.13.2 — root `go.mod:111` (direct); `cmd/tq/go.mod:80` (indirect). Verified 2026-11-08 by `grep -rn 'go-health' --include=go.mod . | grep -v vendor`.

**Latest:** v0.13.2 (local checkout /home/lars/projects/go-health-dashboard, checked 2026-11-08; HEAD = v0.13.2-14-gd15fcb6 — docs, CI permissions, dependency-pin restores, and a NixOS module for the separate health-hub binary; no library API change in [Unreleased]).

**Prior coverage:** none (AGENTS.md:399 records the adoption decision; no prior utilization review).

**Reviewed:** 2026-11-08 by reading /home/lars/projects/go-health-dashboard/{README.md,CHANGELOG.md,dashboard.go,routes.go,options.go,handlers.go,csp.go,status.go,pusher.go,evidence.go,view.templ} plus /home/lars/projects/go-taskqueue/internal/webui/{health.go,webui.go,health_test.go} and cmd/tq/main.go:4300-4372; greps and tests in §How to verify.

## 1. Usage inventory

Import sites (verified 2026-11-08 by `grep -rn 'go-health-dashboard' --include='*.go' . | grep -v '^./vendor/'`): `internal/webui/health.go:22`, `internal/webui/webui.go:29`.

Symbols consumed (verified by `grep -oh 'dashboard\.[A-Za-z]*' internal/webui/*.go | sort | uniq -c`): `New`, `Prober`, `Dashboard`, `Routes`, `RecommendedCSP`, `WithTitle`, `WithRoutes`, `WithEmbeddedDatastarSDK`, `WithCSSPath`, `WithNonceExtractor` + methods `Handler`, `SSEHandler`, `FaviconHandler`, `Start`, `Shutdown`.

Wiring shape:

- Construction: `newHealthDashboard` (health.go:313-333) passes exactly five options — `WithTitle("tq queue health")`, `WithRoutes` (paths for Dashboard/SSE/Favicon/probes/DatastarJS; **Metrics/Trend/Export/Introspect set to `""`**, health.go:323-326), `WithEmbeddedDatastarSDK`, `WithCSSPath("/static/app.css")`, `WithNonceExtractor` (per-request nonce from tq's security-headers middleware).
- Mount: `healthBindings` (webui.go:265-283) registers /health, /health/sse, /health/datastar.js, /health/favicon.svg, /healthz, /readyz, /startupz — every route wrapped in `withHealthHeaders` (strictened CSP + `X-Robots-Tag: noindex`, health.go:335-361), all GET-only, all inside `withTokenAuth` + `withSecurityHeaders` (webui.go:340-342, 358-364). 401 without token is test-pinned (health_test.go:208-228) — the AGENTS.md:399 "inside the token gate" contract.
- Lifecycle: `dash.Start(ctx)` before serving (webui.go:447-451), `dash.Shutdown()` after HTTP drain (webui.go:519). Push interval resolves from the prober's 5 s `RefreshInterval` (health.go:40-42, 106-108 — "one ladder").

## 2. Capability inventory

| Capability | Status | Evidence |
| --- | --- | --- |
| HTML dashboard page (severity-grouped triage, banner, badges, filter, connection pill) | 🟢 FULL | health.go:313-333; webui.go:275; page body test-pinned health_test.go:47-94 |
| Content negotiation on /health (HTML vs `Accept: application/json`) | 🟢 FULL | `dash.Handler()` unmodified (handlers.go:23-53) |
| Realtime SSE + pusher lifecycle (`Start`/`Shutdown`) | 🟢 FULL | webui.go:447-451, 519; SSE route mounted (health_test.go:261-265) |
| Push cadence resolved from probe `RefreshInterval` | 🟢 FULL | dashboard.go:187-201 + health.go:106-108 (5 s) |
| Same-origin embedded Datastar SDK | 🟢 FULL | `WithEmbeddedDatastarSDK` + `Routes.DatastarJS` (health.go:327,329), served at health.go:363-369, asset test health_test.go:230-249 |
| CSS theming via host stylesheet | 🟢 FULL | `WithCSSPath("/static/app.css")` (health.go:330) — tq's own compiled theme |
| CSP integration (nonce extractor + `RecommendedCSP`) | 🟢 FULL — and strictened | `WithNonceExtractor` (health.go:331); `withHealthHeaders` composes the library CSP with `base-uri 'none'` + `frame-ancestors`/`form-action 'none'` (health.go:335-361), regression-pinned health_test.go:60-87 |
| Route table customization (`WithRoutes`, disable-able endpoints) | 🟢 FULL | health.go:316-328 — favicon namespaced under /health; four endpoints deliberately disabled (see rows below) |
| Favicon route | 🟢 FULL | webui.go:278 (`/health/favicon.svg`), content-type test health_test.go:251-259 |
| Failure-evidence strip (proven vs unproven greens) | 🟢 FULL | zero-config: pusher accrues evidence every tick regardless of PushOnChange (pusher.go:120-127; evidence.go:175-216) |
| Per-check `since (17m)` + duration row metadata | 🟡 PARTIAL | rendering ships (status.go:289, 941-952; view.templ:499-506) but tq's `queueProber` never populates `StatusSince`/`DurationNanos` (health.go:232-234) — see the go-health report, finding 2 |
| Prometheus metrics endpoint (`WithMetrics`) | 🔴 MISSED | `Routes.Metrics: ""` (health.go:323); options.go:172-181, metrics.go — queue health has no scrape surface anywhere |
| Trend sparkline + history + JSON/CSV export (`WithTrend`, `WithTimelineMaxAge`) | 🔴 MISSED | `Routes.Trend/Export: ""` (health.go:324-325); options.go:183-193, history.go, trend.go — no record of health over time, no "since when is it flapping" answer |
| Webhook transition alerts (`WithWebhook`/headers/HMAC) | 🔴 MISSED | no webhook option passed (health.go:313-332); options.go:303-345 — fires exactly on status change (pusher.go:140-144). tq already runs an alert channel (internal/bridge/papdashboard via `--alert-url`, papdashboard.go:139) that never hears health transitions |
| Introspection endpoint (`WithIntrospection`) | 🔴 MISSED | `Routes.Introspect: ""` (health.go:326); deliberate min-surface — cheap to enable behind the token gate if /health misconfiguration ever needs debugging |
| Probe-cadence ingest (`WithEvaluationHook` → `dash.Observe`) | ⚪ N/A | requires a go-health evaluation hook; tq's hand-rolled prober has none (dashboard.go:354-381 offer) — blocked on the go-health side |
| `WithMiddleware` auth wrapper | ⚪ N/A | tq's `withTokenAuth` wraps the WHOLE mux incl. probes (webui.go:358-364; health_test.go:208-228) — strictly stronger than the library's probes-excluded design (README "Protecting the Dashboard"); correct for tq's no-unauthenticated-oracle posture |
| Shared rate limiting (`WithRateLimit`) | ⚪ N/A | loopback-default + token gate; write routes carry their own 3-strikes limiter (webui.go:348-356) |
| `WithPublicMode` anonymization | ⚪ N/A | token-gated surface, no untrusted audience; additionally `X-Robots-Tag: noindex` (health.go:350) |
| Grouping axis (severity vs source) | 🟢 FULL | default `GroupBySeverity` (dashboard.go:141) is correct — single-source page; GroupBySource is for aggregate/federation pages tq does not run |
| Healthy-group collapse controls (`WithHealthyGroupCollapse`/`WithPersistCollapse`/`WithHealthyGroupExpanded`) | 🟡 PARTIAL | default threshold 8 active (dashboard.go:140); `WithPersistCollapse` (options.go:209) not opted in — per-browser choice lost on reload |
| SSE connection management (`MaxSSEConnections`, `RetryInterval`, `HeartbeatInterval`, `MaxConnectionLifetime`, `ShutdownDrain`) | 🟡 PARTIAL | defaults only; heartbeat default 15 s (options.go:21) matches webui's own `DefaultHeartbeat` (webui.go:38) — coherent; no connection cap/lifetime/drain for a loopback-first surface (acceptable, but unbounded by choice, not decision record) |
| Pusher self-health (`HealthCheck` watchdog, `SubscriberCount`, `DroppedPatchCount`) | ⚪ N/A | no samber/do container to cascade into; feeding it into /health would violate tq's store-derived-only checks doctrine (health.go:4-7 "never from process state") |
| Title/description branding | 🟡 PARTIAL | `WithTitle` set (health.go:315); `WithDescription` (options.go:277 — meta description + OG tags) skipped, moot under noindex |

## 3. Findings

1. **Health transitions never reach the operator's alert dashboard.**
   Offer: `WithWebhook(url)` POSTs a JSON snapshot on every status change, `WithWebhookHeaders` carries auth, `WithWebhookHMAC` signs deliveries (options.go:303-345; pusher.go:140-144).
   We do instead: nothing — the only push consumer of health state is a browser tab on /health; the PapDashboard bridge (internal/bridge/papdashboard) forwards journal facts (dead letters, pool deaths — papdashboard.go:770,821,880,929) but is deaf to "database failing", "fold wedged behind the head", or "serve demoted to fold follower" (health.go:192-225).
   Impact: a wedged fold or store outage is silent unless someone watches the page; the fold-follower demotion is exactly the class an operator must not miss (health.go:220-225 exists because of it).
   Example, composing with the existing alert config in `cmd/tq` (main.go:4300-4310 builds `webui.Config`; the URL would ride the same flags):
   ```go
   if alertURL != "" {
       opts = append(opts,
           dashboard.WithWebhook(alertURL+"/health-events"),
           dashboard.WithWebhookHeaders(map[string]string{"Authorization": "Bearer " + alertKey}))
   }
   return dashboard.New(pro, opts...)
   ```
2. **No memory: trend, history, export are all off.**
   Offer: `WithTrend(n)` renders a status sparkline, records transitions, and exposes `/health/trend` + `/health/export` (JSON/CSV) — options.go:183-193, README "Health Trend".
   We do instead: `Routes.Trend/Export: ""` (health.go:324-325) — a fresh `tq serve` has amnesia; "has the DLQ check been warning since Tuesday?" is unanswerable from the surface.
   Impact: medium — the evidence strip (row 10) proves greens only since process start, and restarts reset it (evidence.go:38); the trend ring is the library's own answer to that limitation.
3. **No metrics surface for queue health.**
   Offer: `WithMetrics(true)` serves a zero-dependency Prometheus exposition at `/health/metrics` including per-check status/duration gauges and SSE viewer counts (options.go:172-181, metrics.go).
   We do instead: `Routes.Metrics: ""` (health.go:323); tq has no Prometheus surface at all, so external monitors can only poll `/readyz`'s 200/503 bit.
   Impact: low-to-medium (tq is a local-first tool); but the option is free, token-gated like every other route, and would make the dashboard's own latency histogram observable.
4. **The page's richest per-row feature renders empty — data gap at the go-health seam.**
   Offer: rows show "since 14:02:05 UTC (17m)" and per-check duration (status.go:941-952; view.templ:499-506) when the prober populates `Check.StatusSince`/`DurationNanos`, plus the latency stat card fed by `Response.TotalLatencyMs` (pusher.go:138-140).
   We do instead: `queueProber` sets only Status+Error (health.go:232-234) and never TotalLatencyMs (health.go:250-256).
   Impact: "no worker heartbeat — pool idle or down" (health.go:186-187) with no age is materially less actionable; the fix belongs to the prober, not this library (go-health report, finding 2).
5. **Follower demotion shows on the page but nothing downstream reacts.**
   Offer: webhook + metrics again, but specifically: `MarkFoldFollower` (webui.go:224-232) drops the evaluation cache so the next push carries the follower warning (health.go:220-225) — correct push-path design.
   We do instead: the warning reaches connected browsers only; with findings 1-3 unaddressed, a demoted serve is invisible to anything automated.
   Impact: folds into finding 1's fix — the webhook fires on the very transition the demotion produces.
6. **`ShutdownDrain` 0 means SSE clients are dropped the instant `dash.Shutdown` runs** (dashboard.go:249-268; webui.go:519 calls it after the HTTP drain, which already waited — so real impact is near-zero; recorded for completeness, not action).

## 4. Score + top opportunities

Inputs (from §2): N=24 rows; ⚪ N/A=5 (Observe, WithMiddleware, rate limit, PublicMode, pusher self-health); A=19. FULL=11 × 1, PARTIAL=4 × 0.5 (row metadata, collapse persistence, SSE tuning, branding), MISSED=4 × 0 → Σ=13.0. Score = round(100 × 13 / 19) = **68 → moderate** (50-69, one point shy of high).

Top opportunities: (1) `WithWebhook` → PapDashboard (finding 1 — the only change that makes silent-failure classes loud); (2) `WithTrend` + re-enable Trend/Export routes (finding 2); (3) populate `StatusSince`/`TotalLatencyMs` in the prober (finding 4, cross-report); (4) optionally `WithMetrics(true)` (finding 3).

## 5. Version currency + risk

- **Current**: v0.13.2 is the latest tag; the checkout is 14 commits past it with no library-surface change — [Unreleased] holds docs work, a CI permissions block, restored dependency pins (chromedp/go-sse/go-datastar/a-h/parse), and a NixOS module for the health-hub binary (CHANGELOG [Unreleased], verified 2026-11-08 by `git log --oneline v0.13.2..HEAD`).
- **Risk**: low-to-medium. The stated compatibility policy allows breaking changes in minors only with a CHANGELOG note, with deprecations running one minor ahead (README "Compatibility Policy", [0.13.2] section); support line is the latest minor, which tq is on. tq's usage is concentrated in the stable core (`New`, `Routes`, `Handler`/`SSEHandler`/`FaviconHandler`, `RecommendedCSP`) and does not touch the churnier extras — adopting them (findings 1-3) widens the API surface tq exposes itself to, worth doing knowingly. Vendored copy present (`vendor/github.com/larsartmann/go-health-dashboard`).
- Build note: the library requires `GOEXPERIMENT=jsonv2` (README "Build") — tq already exports it for its gates, so no action.

## 6. Conclusion

go-health-dashboard is adopted the way it was meant to be: a single `dashboard.New` call, tq's own CSS and nonce, same-origin SDK, every route inside the token gate, and the security posture strengthened past the library's own recommendation — the operator page itself is best-practice and test-pinned. What is left on the table is everything the page could feed: transitions to the alert bridge that already exists in the binary, history for the questions a live page cannot answer, and a scrape surface. All four gaps are option-flag flips plus one data fix in the prober; none require re-architecting the mount.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
grep -rn 'larsartmann/go-health-dashboard' --include='*.go' . | grep -v '^./vendor/'
# → internal/webui/health.go:22, internal/webui/webui.go:29 (health.go:1 is the package comment)
grep -rn 'go-health' --include=go.mod . | grep -v vendor
# → go.mod:111 v0.13.2 (direct), cmd/tq/go.mod:80 (indirect)
grep -oh 'dashboard\.[A-Za-z]*' internal/webui/*.go | sort | uniq -c
# → New/Prober/Dashboard/Routes/RecommendedCSP/WithTitle/WithRoutes/WithEmbeddedDatastarSDK/WithCSSPath/WithNonceExtractor only
sed -n '313,333p' internal/webui/health.go   # the five options; 323-326 disable Metrics/Trend/Export/Introspect
sed -n '265,283p' internal/webui/webui.go    # healthBindings: all routes wrapped, GET-only
sed -n '358,364p' internal/webui/webui.go    # token gate + security headers wrap the health routes too
GOCACHE=/tmp/go-build-cache GOEXPERIMENT=jsonv2 GOWORK=off \
  go test ./internal/webui/ -run TestHealth -count=1 -v | grep -c '^--- PASS'   # → 9, 0 FAIL
cd /home/lars/projects/go-health-dashboard && git describe --tags   # → v0.13.2-14-gd15fcb6
git log --oneline v0.13.2..HEAD | wc -l                             # → 14
grep -n 'WithWebhook\b\|WithTrend(\|WithMetrics(' options.go          # → 172/183/303 = the funcs (293/325 are doc comments)
```

All commands above were run 2026-11-08; the test line reported `ok github.com/larsartmann/go-taskqueue/internal/webui` with 9/9 health tests PASS (`grep -c '^--- PASS'` over the tee'd log → 9, zero `--- FAIL` lines).
