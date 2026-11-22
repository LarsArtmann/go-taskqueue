# go-appkit — Library Utilization Review

**Verdict:** NOT ADOPTED — KEEP OUT — a greenfield HTTP-service framework whose core composes httputil (explicitly NOT adopted, AGENTS.md §Relation) and whose every module overlaps an existing, ADR-pinned surface: composition root, webui/httpapi, runactor, go-health-dashboard, go-sse.

> **2026-11-22 refresh:** re-verified against tq HEAD 1c8219e6 and the go-appkit checkout — verdict unchanged. Library side MOVED: new root tag **v0.13.0** (2026-11-21, tag commit 638548d — the 11-11 refresh saw v0.12.0; now 14 root tags, 75 total) — v0.13.0 adds a `ServiceConfig.Listener` injection seam (pre-bound listener: unix sockets, systemd socket activation; demand named by two external adoption reviews) and a `ServiceConfig.LogHandler` seam, plus CAPABILITIES/MODULE_INVENTORY docs; the newest sub-module series also moved: **security/v0.3.0** (2026-11-21; adopts `samber/lo` as a runtime dep for the CSRF `*`-strip filter and OriginCheck allowlist build) replaces httpx/v0.2.0 (2026-11-08), preset/v0.1.1 landed 2026-11-13, and an untagged `testkit/` module joined. None of it moves the mapping: the Listener seam lands where tq's runactor groups + webui drain already own lifecycle (and bootstrap's unit is Type=simple, no socket activation), the LogHandler seam where tq already owns slog, the security release on the same ruled-against surface. Decisive fact unchanged: the core STILL requires `larsartmann/httputil v1.8.0` (go-appkit/go.mod:8 — not yet bumped, even though httputil v1.9.0 shipped 2026-11-14), so the transitive not-adopted ruling stands verbatim. tq side: still zero references (rg greps → rc=1); cites drifted — webui drain now webui.go:596–611, hub.go now 57 lines (13–57), tailer runReadModel 67–75, bridge path internal/bridge/papdashboard/papdashboard.go; Go floors now align exactly: go-appkit go.work = `go 1.27.4` = tq root go.mod:3 (11 of 15 module go.mod directives at 1.27.4; root + config/flightrecorderhealth/realtime declare bare `go 1.27`). Checkout HEAD = security/v0.3.0-10-g10a17ab (2026-11-21; v0.13.0..HEAD = 10 auto-commit chores).

> **2026-11-11 refresh:** re-verified 2026-11-11 — **no new release since the review**: v0.12.0 (2026-11-07) is still the newest root tag (13 root tags, 72 total — both unchanged), and v0.12.0..HEAD has added only chores plus lint/proxy-hash fixes (HEAD ed79e4f, 2026-11-10; the httpx wrapcheck lint fix is the only non-chore code touch). The core's httputil requirement is unchanged (go.mod:8-9, v1.8.0 + server_timing v1.0.3), so the decisive keep-out rationale stands exactly as written. No section below changes materially.

**Pinned:** none (not a dependency; verified 2026-11-22 by `rg -i 'go-appkit' -g 'go.mod' -g 'go.sum' -g '!vendor/**' .` → no output, rc=1; same for `--glob '*.go'` import greps)

**Latest:** root series v0.13.0 (2026-11-21, 638548d — NEW since the 11-11 refresh); newest sub-module series security/v0.3.0 (2026-11-21; was httpx/v0.2.0 at 11-11) — `git tag -l | sort -V | tail -8` → v0.6.0 … v0.13.0 (the vN root tags sort last; local checkout /home/lars/projects/go-appkit, checked 2026-11-22; HEAD = security/v0.3.0-10-g10a17ab, 2026-11-21)

**Prior coverage:** none directly; adjacent rulings: AGENTS.md §Relation to other projects (go-health-dashboard ADOPTED; **httputil** NOT adopted), ADR-0003 (webui architecture), ADR-0004 (lifecycle stance), ADR-0008 (HTTP API), ADR-0019 (go-cqrs-lite platform).

**Reviewed:** 2026-11-08 by reading /home/lars/projects/go-appkit/{README.md,go.mod,config.go,doc.go} plus sub-module tables (cqrs/, realtime/, security/, systemd/, health/, errorpages/, otel/, flightrecorder/, httpx/) and internal/composition, internal/webui/{webui.go,hub.go,tailer.go}, internal/httpapi/httpapi.go, internal/runactor/runactor.go; greps in §How to verify.

## 1. Where the need exists today

go-appkit is a general application toolkit: a `Service` core (HTTP server with graceful drain, middleware stack = ServerTiming → Recovery → RequestID → Logging → Timeout → SecurityHeaders, /health endpoints, charmbracelet/log, go-error-family classification — README:63, and since v0.13.0 a `ServiceConfig.Listener` seam for pre-bound listeners/socket activation plus a `ServiceConfig.LogHandler` seam) plus independently-versioned modules: cqrs (event store/projections/DLQ over go-cqrs-lite), realtime (SSE hub: broadcast, replay, heartbeat, Last-Event-ID), security (CSRF, rate limiting, CSP nonce), systemd (sd_notify), errorpages, otel, flightrecorder(health), docs (OpenAPI/AsyncAPI from types), preset (one-call composition spine, v0.1.x), testkit (untagged test harness).

The tq code each piece would touch — all exist, all ADR-pinned:

- **Service lifecycle/composition root**: `internal/composition` is the S4 root (`system.New`, `ProjectionRuntime`; AGENTS.md package table); servers run under runactor groups with LIFO teardown (internal/runactor/runactor.go:1-14, ADR-0004). webui drains its HTTP server on shutdown (internal/webui/webui.go:596-611 — Shutdown → tailer join → dash → hub → agent feed, cites re-verified 2026-11-22).
- **Middleware/health/dashboard**: the dashboard's chain (token gate → CSRF → rate-limit → security headers) is hand-rolled and test-pinned (see the httputil review, docs/review/libs/httputil.md §1); /health rides the ADOPTED go-health + go-health-dashboard pair (AGENTS.md §Relation).
- **realtime (SSE hub)**: tq's hub is 57 lines over the pinned `github.com/larsartmann/go-sse` Broadcaster with watermark event IDs for Last-Event-ID resume (internal/webui/hub.go:13-57) — projection-not-payload by design (ADR-0003).
- **cqrs module**: tq consumes go-cqrs-lite DIRECTLY as "the platform" (ADR-0019, adopted in sqlitev4/postgresv4 + companion); wrapping the same engine a second time in appkit's facade adds a layer between tq and its own store invariants.
- **systemd module**: the pool unit is a hand-rendered template, default Type=simple (no explicit Type= line; KillSignal=SIGINT + 45min drain; cmd/tq/bootstrap.go:39-74, embedded from deploy/systemd/tq-agent-pool.service) — no sd_notify consumer (see the go-daemon review); v0.13.0's Listener/socket-activation seam is aimed at exactly the systemd style tq does not use.
- **httpx (v0.2.0, 2026-11-08)**: HTTP client/server helpers — tq's outbound HTTP is the daemon SSE client and bridge POSTs (internal/harvest/discovery.go:171-184, internal/bridge/papdashboard/papdashboard.go), each with owned timeout semantics.

## 2. Capability inventory (fit)

| Capability | Status | Evidence |
| --- | --- | --- |
| Service lifecycle + graceful drain | 🟡 partial — runactor + webui.go:596-611 already own exactly this, deliberately framework-free (ADR-0004) | ADR-0004 Decisions:1 |
| Default middleware stack | ⚪ N/A — the stack's pieces are either ruled against (httputil NOT adopted, AGENTS.md §Relation) or already owned (token gate, CSRF, lockout — see httputil.md review) | docs/review/libs/httputil.md §2 |
| Health + live health dashboard wiring | 🟡 partial — go-health + go-health-dashboard are ADOPTED at /health inside the token gate | AGENTS.md §Relation (go-health-dashboard ADOPTED) |
| realtime SSE hub (replay/heartbeat/Last-Event-ID) | 🟡 partial — go-sse Broadcaster already pinned; hub carries watermark IDs (Last-Event-ID semantics preserved); no replay wanted: clients re-snapshot | hub.go:13-57, tailer.go:67-75 |
| cqrs module | ⚪ N/A — double-wrapping the platform tq already consumes directly (ADR-0019); the proprietary `internal/journal/cqrs` adapter stays read-only | AGENTS.md §Relation (go-cqrs-lite IS the platform) |
| security module (CSRF/rate-limit/CSP nonce) | ⚪ N/A — same overlap ruled in the httputil review: tq's auth plane is a pinned contract, not a middleware swap | httputil.md §3 |
| systemd sd_notify module | ⚪ N/A — unit is Type=simple; runactor owns signals | bootstrap.go:39-74 |
| errorpages / charmbracelet/log / go-error-family | ⚪ N/A — tq uses slog + templ rendering + stdlib errors (erraudit rules legacy shapes; no error-code family) | AGENTS.md erraudit section |
| docs (OpenAPI/AsyncAPI generation) | ⚪ N/A — the machine API is 4 routes (httpapi.go:195-198), documented in ADR-0008 | httpapi.go:195-198 |
| otel / flightrecorder | ⚪ N/A — no telemetry pipeline in tq (loopback, single-operator posture) | — |
| Pure Go, `CGO_ENABLED=0` | compatible — httputil/go-error-family/charmbracelet stack is pure Go; Go floors now align: go.work `go 1.27.4` = tq root go.mod:3 | go-appkit/go.mod, go.work |

## 3. Findings (buy vs cost)

1. **Buy (for a NEW service):** one import gives a production-shaped service skeleton — lifecycle, middleware, health, docs, SSE. tq already built each of those, under ADRs, with tests pinned to its contracts.
2. **Cost (decisive) — transitive adoption of a ruled-against dependency:** core requires `larsartmann/httputil v1.8.0` (go-appkit/go.mod:8 — unchanged at HEAD even though httputil v1.9.0 shipped 2026-11-14; server_timing v1.0.3 rides at :9) — the library AGENTS.md §Relation explicitly records as NOT adopted. Importing appkit core imports httputil; the ruling cannot survive the adoption.
3. **Cost:** every module lands on a surface whose behavior is asserted by dedicated tests (internal/webui/security_test.go, headermatrix_test.go, internal/httpapi TestAuthPlaneBodyShape; hub Last-Event-ID semantics in webui tests). A framework swap re-litigates ADR-0003, ADR-0004, ADR-0008, and the httputil ruling at once, for zero new capability.
4. **Cost — youth:** root v0.13.0 in ~5 months (75 tags total incl. sub-modules since 2026-06-12; v0.12→v0.13 plus security/v0.3.0 plus preset/v0.1.1 all within 2026-11-13..21 — still no newer root tag as of 2026-11-22, but the churn pattern, not the calendar, is the point). Pre-1.0 churn across 10+ sub-modules is the wrong floor for a stable, multi-module, vendorHash-pinned repo.
5. **Adoption site if it ever happened** (only plausible shape: a brand-new standalone service, not retrofit):

```go
svc, _ := appkit.NewService(appkit.ServiceConfig{Addr: "127.0.0.1:8095"})
svc.Mux.HandleFunc("GET /{$}", handleHome)
err := svc.Run(ctx) // drain, signals, /health — none of which tq lacks today
```

## 4. Fit assessment

go-appkit is the right default for a greenfield Go service in this ecosystem — it bundles the paved road (httputil middleware, go-health, structured logging, cqrs wiring) into one `NewService`. But go-taskqueue is not greenfield: it IS its own paved road, assembled deliberately — a composition root over go-cqrs-lite (ADR-0019), a runactor lifecycle (ADR-0004), two token-gated HTTP surfaces (ADR-0003/0008), an adopted health pair, and a pinned SSE library. Every appkit module maps onto an existing decision, and the one mapping that could be "new" (the middleware stack) is transitively blocked by the httputil NOT-adopted ruling. Nothing in tq's ROADMAP.md (channels plan, MCP surface) needs a service framework.

## 5. Maturity + version currency

Root series 14 tags v0.1.0 (first commit 2026-06-12) → v0.13.0 (2026-11-21, 638548d; newest, checked 2026-11-22); 75 tags counting independently-versioned sub-modules (was 72 at the 11-11 refresh; +v0.13.0, +preset/v0.1.1 2026-11-13, +security/v0.3.0 2026-11-21); newest series security/v0.1.0→v0.3.0, with v0.3.0 (2026-11-21) taking over from httpx/v0.1.0→v0.2.0 (2026-11-07/08); last commit 10a17ab 2026-11-21 (chore; v0.13.0..HEAD = 10 auto-commit chores only). Extremely active, very young, pre-1.0 everywhere. MIT. Go floors: go.work `go 1.27.4` (11 of 15 module go.mods at 1.27.4, root + 3 satellites bare `go 1.27`) — tq root is 1.27.4, jsonv2 default-on.

## 6. Conclusion

Keep out. This is the toolkit tq would have started from in June; adopting it now would mean re-deriving five accepted ADRs to reach the same place with younger dependencies — re-verified 2026-11-22 against v0.13.0 + security/v0.3.0: the release adds seams (Listener, LogHandler), not capabilities tq lacks. Watch it for its `httpx` and `realtime` modules if a future standalone service (e.g. the planned MCP channel endpoint — AGENTS.md:141 TQ-MCP-CHANNELS-DESIGN-INPUT, ROADMAP.md:75 chat-channel bridge) is spun up outside the existing surfaces — that would be a `WORTH A TRIAL` scoped to the new binary only.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
rg -i 'go-appkit' -g 'go.mod' -g 'go.sum' -g '!vendor/**' .            # → no output (rc=1)
rg -i 'go-appkit' --glob '*.go' --glob '!vendor/**' .                   # → no output (rc=1)
grep -n 'NOT adopted' AGENTS.md | head -2   # httputil ruling (§Relation to other projects, :424)
grep -n 'larsartmann/httputil' /home/lars/projects/go-appkit/go.mod     # core requires httputil v1.8.0 (:8) + server_timing (:9)
git -C /home/lars/projects/go-appkit tag -l | sort -V | tail -8         # → v0.6.0 … v0.13.0 (root tags sort last)
git -C /home/lars/projects/go-appkit tag -l | grep -v / | wc -l         # → 14 root-series tags
git -C /home/lars/projects/go-appkit tag -l | wc -l                     # → 75 incl. sub-modules
git -C /home/lars/projects/go-appkit describe --tags                    # → security/v0.3.0-10-g10a17ab
git -C /home/lars/projects/go-appkit log v0.13.0..HEAD --oneline        # 10 auto-commit chores only
git -C /home/lars/projects/go-appkit log -1 --format='%ad %h' --date=short v0.13.0  # 2026-11-21 638548d
sed -n '13,57p' internal/webui/hub.go     # go-sse hub already owns realtime
sed -n '1,14p' internal/runactor/runactor.go  # lifecycle already owned
```
