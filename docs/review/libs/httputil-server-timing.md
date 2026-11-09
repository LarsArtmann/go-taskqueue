# httputil/server_timing — Library Utilization Review

**Verdict:** NOT ADOPTED — WORTH A TRIAL — the one httputil-family module that adds something tq lacks (per-handler latency visibility) with zero dependencies and a conditional predicate that fits the loopback/token posture; not urgent for a personal-scale dashboard.

**Pinned:** none (not a dependency; verified 2026-11-08 by `grep -rn 'httputil/server_timing' --include=go.mod . | grep -v vendor` → no output, same for import greps)

**Latest:** server_timing/v1.0.3 (local checkout /home/lars/projects/httputil/server_timing, checked 2026-11-08)

**Prior coverage:** none (sibling of the root-module review, docs/review/libs/httputil.md — KEEP OUT; this submodule is separately installable and stdlib-only by design, server_timing/README.md:7–9)

**Reviewed:** 2026-11-08 by reading /home/lars/projects/httputil/server_timing/{README.md,middleware.go,server_timing.go} and grepping internal/webui + internal/httpapi for existing timing.

## 1. Where the need exists today

The module wraps an http.Handler with a concurrency-safe `*ServerTiming` collector in the request context; handlers record named regions via `MeasureServerTiming`/`RecordServerTiming` (nil-safe without the middleware), and the `Server-Timing` header is stamped once at first write, including an auto `total` (TTFB) metric (server_timing/README.md:31–43; server_timing.go:222–275). `ServerTimingMiddlewareWhen(pred)` enables collection only for matching requests — their README warns Server-Timing leaks internal performance details, so gate it (README:52–55). `Flush`/`Hijack`/`Push`/`Unwrap` pass through, so SSE keeps working (README:57–58) — decisive for the webui, whose live surface is SSE (internal/webui/handlers.go:315+, ADR-0003).

What tq has today, per greps (`server-timing|servertiming` → zero hits repo-wide):

- **webui**: total-duration-only access logging, off by default — `withRequestLog` logs method/path/status/`time.Since(start)` to slog when `Config.RequestLog` is set (internal/webui/requestlog.go:13–29; config flag at webui.go:70–73). No per-segment measurement, nothing browser-visible.
- **httpapi**: no request timing or logging at all (grep `time.Since` in internal/httpapi → only /healthz uptime strings, httpapi.go via internal/webui/health.go:117). Handlers that would carry interesting segments: `GET /api/v1/stats` reads through `readmodel.StatusCounts` with the projection pump behind it (httpapi.go:450–468), and webui `GET /api/facts` pages the journal (`store.Facts`, handlers.go:143–186) — fold lag vs. query time is exactly the kind of split Server-Timing names.

## 2. Capability inventory (fit)

| Capability | Status | Evidence |
| --- | --- | --- |
| Per-handler `Server-Timing` header (DevTools/RUM-visible) | 🔴 MISSING-BUT-RELEVANT | zero hits for Server-Timing repo-wide (grep, 2026-11-08) |
| Auto `total` (TTFB) metric | 🟡 partial overlap | requestlog.go:23 computes total duration but only into slog logs, opt-in |
| Named sub-metrics from handler internals | 🔴 MISSING-BUT-RELEVANT | no context-based timing collector exists in webui/httpapi |
| SSE-safe wrapper (Flush/Hijack passthrough) | 🔴 MISSING-BUT-RELEVANT (required: webui is SSE-first) | server_timing.go:275–303 delegatingWriter |
| Conditional enablement (debug/role predicate) | 🔴 MISSING-BUT-RELEVANT (matches tq's loopback + token posture) | README:52–55 `ServerTimingMiddlewareWhen` |
| Injection-safe header serialization | ⚪ N/A (no existing header producer to harden) | fuzz-tested per README:56 |

## 3. Findings (buy vs cost)

1. **Buy:** zero-dependency module (server_timing/go.mod requires nothing) — adoption touches one go.mod (internal/webui lives in the root module) plus a ./vendor/ resync; the header lands where browser DevTools already look, no vendor agent; the `When` predicate could tie it to the existing `--auth-token`/loopback distinction so a LAN-exposed dashboard does not broadcast its fold-lag numbers.
2. **Buy:** it composes as a plain `func(http.Handler) http.Handler`, so it slots into webui's `Handler()` chain (webui.go:364–369) without touching any handler body; `MeasureServerTiming` is a no-op when the middleware is absent, so instrumentation can land in handlers ahead of the middleware.
3. **Cost:** the "measure before you write" gotcha (README:60–76) — the header is stamped at first `Write`; deferred `stop()` misses it except on SSE streams. Handlers must be disciplined at the ~3 instrumented call sites. Low value today: the dashboard is a loopback personal tool with one user; latency surprises are debugged with `--request-log`.
4. **Adoption site** (webui, gated):

```go
// internal/webui/webui.go Handler() — after withSecurityHeaders, before withRequestLog
handler = servertiming.ServerTimingMiddlewareWhen(func(r *http.Request) bool {
    return s.cfg.RequestLog // piggyback on the existing opt-in flag: debug surface, off by default
})(handler)

// internal/webui/handlers.go handleFacts — nil-safe even when the middleware is off
stop := servertiming.MeasureServerTiming(r.Context(), "store.Facts")
facts, err := s.store.Facts(r.Context(), after, limit)
stop() // BEFORE the write — a deferred stop() fires after the response commits
```

## 4. Fit assessment

This is the rare review where the library's capability set and tq's gap align one-to-one: tq measures nothing per-handler, the module does exactly that and nothing else, and its SSE passthrough matches the dashboard's streaming architecture. What keeps it at WORTH A TRIAL rather than ADOPT NOW is demand, not fit — a single-operator loopback dashboard gets little from header-based latency visibility, and httpapi's four routes (httpapi.go:193–201) are thin enough that timing would mostly measure SQLite. The trial shape is concrete: mount it behind the existing `RequestLog` flag on `tq serve`, instrument `handleFacts` and the stats read, and see whether the fold-vs-read split ever explains anything; remove it if it never does.

## 5. Maturity + version currency

Latest server_timing/v1.0.3 (tag commit 192539f 2026-10-24). Six submodule tags (v0.9.1, v0.10.0, v0.12.0, v1.0.1, v1.0.2, v1.0.3) — the 1.x line is ~3 releases old, MIT since v1.0.2 (checkout AGENTS.md:327: owner decision 2026-10-08). Stdlib-only, fuzz + bench + spec tests in the checkout (server_timing_{fuzz,bench}_test.go). Consumed by the httputil root module itself (httputil go.mod: `httputil/server_timing v1.0.3`), so it has a permanent in-house consumer.

## 6. Conclusion

Trial-worthy and only in the webui, gated. It is the cheapest observability upgrade available to `tq serve` (zero deps, ~10 lines) and deliberately independent of the root httputil module this repo already declined. Do not mount it unconditionally on the token-gated API — Server-Timing on an internet-exposed surface is a free performance oracle for attackers (their README's own security note).

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
grep -rn 'httputil/server_timing' --include=go.mod . | grep -v vendor      # → root go.mod (trial dep since 2026-11-09)
grep -rni 'server-timing\|servertiming' --include='*.go' internal cmd      # → webui.go mount + handlers.go facts/stats segments
sed -n '13,29p' internal/webui/requestlog.go                               # the pre-trial latency surface (slog only)
cat /home/lars/projects/httputil/server_timing/go.mod                      # zero requires
git -C /home/lars/projects/httputil tag -l 'server_timing/*' | sort -V     # 6 tags, latest v1.0.3
head -3 /home/lars/projects/httputil/server_timing/LICENSE                 # MIT
```

## Trial verdict (2026-11-09)

**TRIAL MOUNTED — KEEP (for now).** The trial from §4 ran: the middleware
rides `ServerTimingMiddlewareWhen(RequestLog)` in `internal/webui/webui.go`
so the header ships ONLY where the operator already opted into request
logging (default loopback posture never carries it — the header leaks
internal performance detail per the library's own README). Instrumented
segments: `stats` (the status-counts read) and `facts` (the journal page
read), both recorded before the first write per the stamps-at-first-write
contract; the auto `total` (TTFB) metric rides along. Pinned by
`TestServerTimingHeaderBehindRequestLog` (header present with `stats` +
`total` under RequestLog, absent without).

Keep-or-remove: revisit when (a) the read-model fold-vs-read split ever
needs explaining in anger, or (b) RequestLog usage shows nobody opens
DevTools on this surface — remove is a 3-line revert plus the dep.

