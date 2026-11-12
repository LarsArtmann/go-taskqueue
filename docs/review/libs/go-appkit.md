# go-appkit — Library Utilization Review

**Verdict:** NOT ADOPTED — KEEP OUT — a greenfield HTTP-service framework whose core composes httputil (explicitly NOT adopted, AGENTS.md §Relation) and whose every module overlaps an existing, ADR-pinned surface: composition root, webui/httpapi, runactor, go-health-dashboard, go-sse.

> **2026-11-11 refresh:** re-verified 2026-11-11 — **no new release since the review**: v0.12.0 (2026-11-07) is still the newest root tag (13 root tags, 72 total — both unchanged), and v0.12.0..HEAD has added only chores plus lint/proxy-hash fixes (HEAD ed79e4f, 2026-11-10; the httpx wrapcheck lint fix is the only non-chore code touch). The core's httputil requirement is unchanged (go.mod:8-9, v1.8.0 + server_timing v1.0.3), so the decisive keep-out rationale stands exactly as written. No section below changes materially.

**Pinned:** none (not a dependency; verified 2026-11-11 by `rg -i 'go-appkit' -g 'go.mod' -g 'go.sum' -g '!vendor/**' .` → no output, rc=1; same for `--glob '*.go'` import greps)

**Latest:** root series v0.12.0 (2026-11-07, 34edcd2 — still newest as of 2026-11-11); newest sub-module series httpx/v0.2.0 (2026-11-08) — `git tag -l | sort -V | tail -8` → … v0.10.0 v0.11.0 v0.12.0 (local checkout /home/lars/projects/go-appkit, checked 2026-11-11; HEAD = httpx/v0.2.0-11-ged79e4f, 2026-11-10)

**Prior coverage:** none directly; adjacent rulings: AGENTS.md §Relation to other projects (go-health-dashboard ADOPTED; **httputil** NOT adopted), ADR-0003 (webui architecture), ADR-0004 (lifecycle stance), ADR-0008 (HTTP API), ADR-0019 (go-cqrs-lite platform).

**Reviewed:** 2026-11-08 by reading /home/lars/projects/go-appkit/{README.md,go.mod,config.go,doc.go} plus sub-module tables (cqrs/, realtime/, security/, systemd/, health/, errorpages/, otel/, flightrecorder/, httpx/) and internal/composition, internal/webui/{webui.go,hub.go,tailer.go}, internal/httpapi/httpapi.go, internal/runactor/runactor.go; greps in §How to verify.

## 1. Where the need exists today

go-appkit is a general application toolkit: a `Service` core (HTTP server with graceful drain, middleware stack = ServerTiming → Recovery → RequestID → Logging → Timeout → SecurityHeaders, /health endpoints, charmbracelet/log, go-error-family classification — README:44-46) plus independently-versioned modules: cqrs (event store/projections/DLQ over go-cqrs-lite), realtime (SSE hub: broadcast, replay, heartbeat, Last-Event-ID), security (CSRF, rate limiting, CSP nonce), systemd (sd_notify), errorpages, otel, flightrecorder(health), docs (OpenAPI/AsyncAPI from types).

The tq code each piece would touch — all exist, all ADR-pinned:

- **Service lifecycle/composition root**: `internal/composition` is the S4 root (`system.New`, `ProjectionRuntime`; AGENTS.md package table); servers run under runactor groups with LIFO teardown (internal/runactor/runactor.go:1-14, ADR-0004). webui drains its HTTP server on shutdown (internal/webui/webui.go:579-591).
- **Middleware/health/dashboard**: the dashboard's chain (token gate → CSRF → rate-limit → security headers) is hand-rolled and test-pinned (see the httputil review, docs/review/libs/httputil.md §1); /health rides the ADOPTED go-health + go-health-dashboard pair (AGENTS.md §Relation).
- **realtime (SSE hub)**: tq's hub is 45 lines over the pinned `github.com/larsartmann/go-sse` Broadcaster with watermark event IDs for Last-Event-ID resume (internal/webui/hub.go:11-45) — projection-not-payload by design (ADR-0003).
- **cqrs module**: tq consumes go-cqrs-lite DIRECTLY as "the platform" (ADR-0019, adopted in sqlitev4/postgresv4 + companion); wrapping the same engine a second time in appkit's facade adds a layer between tq and its own store invariants.
- **systemd module**: the pool unit is a hand-rendered template, default Type=simple (no explicit Type= line; KillSignal=SIGINT + 45min drain; cmd/tq/bootstrap.go:39-74, embedded from deploy/systemd/tq-agent-pool.service) — no sd_notify consumer (see the go-daemon review).
- **httpx (newest module, v0.2.0)**: HTTP client/server helpers — tq's outbound HTTP is the daemon SSE client and bridge POSTs (internal/harvest/discovery.go:171-184, internal/bridge/papdashboard.go), each with owned timeout semantics.

## 2. Capability inventory (fit)

| Capability | Status | Evidence |
| --- | --- | --- |
| Service lifecycle + graceful drain | 🟡 partial — runactor + webui.go:579-591 already own exactly this, deliberately framework-free (ADR-0004) | ADR-0004 Decisions:1 |
| Default middleware stack | ⚪ N/A — the stack's pieces are either ruled against (httputil NOT adopted, AGENTS.md §Relation) or already owned (token gate, CSRF, lockout — see httputil.md review) | docs/review/libs/httputil.md §2 |
| Health + live health dashboard wiring | 🟡 partial — go-health + go-health-dashboard are ADOPTED at /health inside the token gate | AGENTS.md §Relation (go-health-dashboard ADOPTED) |
| realtime SSE hub (replay/heartbeat/Last-Event-ID) | 🟡 partial — go-sse Broadcaster already pinned; hub carries watermark IDs (Last-Event-ID semantics preserved); no replay wanted: clients re-snapshot | hub.go:11-45, tailer.go:59-75 |
| cqrs module | ⚪ N/A — double-wrapping the platform tq already consumes directly (ADR-0019); the proprietary `internal/journal/cqrs` adapter stays read-only | AGENTS.md §Relation (go-cqrs-lite IS the platform) |
| security module (CSRF/rate-limit/CSP nonce) | ⚪ N/A — same overlap ruled in the httputil review: tq's auth plane is a pinned contract, not a middleware swap | httputil.md §3 |
| systemd sd_notify module | ⚪ N/A — unit is Type=simple; runactor owns signals | bootstrap.go:39-60 |
| errorpages / charmbracelet/log / go-error-family | ⚪ N/A — tq uses slog + templ rendering + stdlib errors (erraudit rules legacy shapes; no error-code family) | AGENTS.md erraudit section |
| docs (OpenAPI/AsyncAPI generation) | ⚪ N/A — the machine API is 4 routes (httpapi.go:193-201), documented in ADR-0008 | httpapi.go:193-201 |
| otel / flightrecorder | ⚪ N/A — no telemetry pipeline in tq (loopback, single-operator posture) | — |
| Pure Go, `CGO_ENABLED=0` | compatible — httputil/go-error-family/charmbracelet stack is pure Go; Go floor 1.27.3 matches tq exactly | go-appkit/go.mod |

## 3. Findings (buy vs cost)

1. **Buy (for a NEW service):** one import gives a production-shaped service skeleton — lifecycle, middleware, health, docs, SSE. tq already built each of those, under ADRs, with tests pinned to its contracts.
2. **Cost (decisive) — transitive adoption of a ruled-against dependency:** core requires `larsartmann/httputil v1.8.0` (go-appkit/go.mod) — the library AGENTS.md §Relation explicitly records as NOT adopted. Importing appkit core imports httputil; the ruling cannot survive the adoption.
3. **Cost:** every module lands on a surface whose behavior is asserted by dedicated tests (internal/webui/security_test.go, headermatrix_test.go, internal/httpapi TestAuthPlaneBodyShape; hub Last-Event-ID semantics in webui tests). A framework swap re-litigates ADR-0003, ADR-0004, ADR-0008, and the httputil ruling at once, for zero new capability.
4. **Cost — youth:** root v0.12.0 in ~5 months (72 tags total incl. sub-modules since 2026-06-12; v0.10→v0.12 in two days, httpx/v0.2.0 the day of this review — still no newer root tag as of 2026-11-11, but the churn pattern, not the calendar, is the point). Pre-1.0 churn across 10+ sub-modules is the wrong floor for a stable, multi-module, vendorHash-pinned repo.
5. **Adoption site if it ever happened** (only plausible shape: a brand-new standalone service, not retrofit):

```go
svc, _ := appkit.NewService(appkit.ServiceConfig{Addr: "127.0.0.1:8095"})
svc.Mux.HandleFunc("GET /{$}", handleHome)
err := svc.Run(ctx) // drain, signals, /health — none of which tq lacks today
```

## 4. Fit assessment

go-appkit is the right default for a greenfield Go service in this ecosystem — it bundles the paved road (httputil middleware, go-health, structured logging, cqrs wiring) into one `NewService`. But go-taskqueue is not greenfield: it IS its own paved road, assembled deliberately — a composition root over go-cqrs-lite (ADR-0019), a runactor lifecycle (ADR-0004), two token-gated HTTP surfaces (ADR-0003/0008), an adopted health pair, and a pinned SSE library. Every appkit module maps onto an existing decision, and the one mapping that could be "new" (the middleware stack) is transitively blocked by the httputil NOT-adopted ruling. Nothing in tq's ROADMAP.md (channels plan, MCP surface) needs a service framework.

## 5. Maturity + version currency

Root series 13 tags v0.1.0 (first commit 2026-06-12) → v0.12.0 (2026-11-07; still newest, checked 2026-11-11); 72 tags counting independently-versioned sub-modules (unchanged since review); newest series httpx/v0.1.0→v0.2.0 within 2026-11-07/08; last commit ed79e4f 2026-11-10 (chore; since the review only chores + a httpx wrapcheck lint fix + integration proxy-hash fixes landed). Extremely active, very young, pre-1.0 everywhere. MIT. Go floor 1.27.3 — identical to tq's root go line, jsonv2 default-on.

## 6. Conclusion

Keep out. This is the toolkit tq would have started from in June; adopting it now would mean re-deriving five accepted ADRs to reach the same place with younger dependencies — re-verified 2026-11-11, no release since the review changes that. Watch it for its `httpx` and `realtime` modules if a future standalone service (e.g. the planned MCP channel endpoint) is spun up outside the existing surfaces — that would be a `WORTH A TRIAL` scoped to the new binary only.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
rg -i 'go-appkit' -g 'go.mod' -g 'go.sum' -g '!vendor/**' .            # → no output (rc=1)
rg -i 'go-appkit' --glob '*.go' --glob '!vendor/**' .                   # → no output (rc=1)
grep -n 'NOT adopted' AGENTS.md | head -2   # httputil ruling (§Relation to other projects)
grep -n 'larsartmann/httputil' /home/lars/projects/go-appkit/go.mod     # core requires httputil
git -C /home/lars/projects/go-appkit tag -l | sort -V | tail -8         # … v0.10.0 v0.11.0 v0.12.0
git -C /home/lars/projects/go-appkit tag -l | grep -v / | wc -l         # → 13 root-series tags
git -C /home/lars/projects/go-appkit tag -l | wc -l                     # → 72 incl. sub-modules
git -C /home/lars/projects/go-appkit describe --tags                    # → httpx/v0.2.0-11-ged79e4f
git -C /home/lars/projects/go-appkit log v0.12.0..HEAD --oneline        # chores + lint/proxy-hash fixes only
git -C /home/lars/projects/go-appkit log -1 --format='%ad %h' --date=short v0.12.0  # 2026-11-07 34edcd2
sed -n '11,45p' internal/webui/hub.go     # go-sse hub already owns realtime
sed -n '1,14p' internal/runactor/runactor.go  # lifecycle already owned
```
