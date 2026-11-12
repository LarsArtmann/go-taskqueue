# httputil/server_timing — Library Utilization Review

**Verdict:** ADOPTED — TRIAL MOUNTED 2026-11-09, KEEP (for now) — re-graded 2026-11-11 from WORTH A TRIAL because the §4 trial shape actually shipped: the module is pinned (root go.mod:126), mounted behind `RequestLog` (webui.go:416–423), and pinned by a test (requestlog_test.go:16). Removal stays a 3-line revert plus the dep.

> **2026-11-11 refresh:** trial executed and kept (fleet review f21, 2026-11-09). Verified 2026-11-11 by grep + committed sources: mount webui.go:416–423 (predicate `s.cfg.RequestLog`), segments `facts`/`stats` recorded before first write (handlers.go:166–173, 204–209), pin TestServerTimingHeaderBehindRequestLog (requestlog_test.go:16–38). Version currency unchanged (server_timing/v1.0.3 still newest of 6 tags; re-checked). The trial test cannot BUILD from today's working tree — the in-flight sqlite/postgres migration breaks the vendored root-module build (`vendor/…/queue/sqlite/sqlite.go: undefined: v5`) — a tree-state artifact unrelated to the trial. §1–§4 below keep the pre-trial assessment for the record, with line cites refreshed to the current tree.

**Pinned:** v1.0.3 — root go.mod:126 `github.com/larsartmann/httputil/server_timing v1.0.3`, the ONLY httputil-family module in the dependency tree (verified 2026-11-11 by `grep -rn 'larsartmann/httputil\|go-etag' --include=go.mod . | grep -v vendor` → single hit go.mod:126; imports at internal/webui/webui.go:34 and handlers.go:18). Pre-trial (2026-11-08): none.

**Latest:** server_timing/v1.0.3 (local checkout /home/lars/projects/httputil/server_timing, checked 2026-11-08; re-verified 2026-11-11 — still the newest of the 6 server_timing tags)

**Prior coverage:** none (sibling of the root-module review, docs/review/libs/httputil.md — KEEP OUT; this submodule is separately installable and stdlib-only by design, server_timing/README.md:7–9)

**Reviewed:** 2026-11-08 by reading /home/lars/projects/httputil/server_timing/{README.md,middleware.go,server_timing.go} and grepping internal/webui + internal/httpapi for existing timing.

## 1. Where the need exists today

The module wraps an http.Handler with a concurrency-safe `*ServerTiming` collector in the request context; handlers record named regions via `MeasureServerTiming`/`RecordServerTiming` (nil-safe without the middleware), and the `Server-Timing` header is stamped once at first write, including an auto `total` (TTFB) metric (server_timing/README.md:31–47, 53–55; server_timing.go:222–303, 317–348). `ServerTimingMiddlewareWhen(pred)` enables collection only for matching requests — their README warns Server-Timing leaks internal performance details, so gate it (README:62). `Flush`/`Hijack`/`Push`/`Unwrap` pass through, so SSE keeps working (README:66) — decisive for the webui, whose live surface is SSE (internal/webui/handlers.go:330+, ADR-0003).

What tq had before the trial (2026-11-08 greps: `server-timing|servertiming` → zero hits repo-wide); the same greps today return exactly the trial — webui.go:34, 416–423; handlers.go:18, 166–173, 204–209; requestlog_test.go:11–38 (verified 2026-11-11):

- **webui**: total-duration-only access logging, off by default — `withRequestLog` logs method/path/status/`time.Since(start)` to slog when `Config.RequestLog` is set (internal/webui/requestlog.go:13–29; config flag at webui.go:74–77). No per-segment measurement, nothing browser-visible — until the trial landed; the segments now ride this same flag (webui.go:421–423).
- **httpapi**: no request timing or logging at all — grep `time.Since` in internal/httpapi → zero hits (re-verified 2026-11-11; the /healthz uptime strings live in internal/webui/health.go:152, 321, outside the API). Handlers that would carry interesting segments: `GET /api/v1/stats` reads through `readmodel.StatusCounts` with the projection pump behind it (httpapi.go:450–468), and webui `GET /api/facts` pages the journal (`store.Facts`, handlers.go:139–195) — fold lag vs. query time is exactly the kind of split Server-Timing names.

## 2. Capability inventory (fit)

| Capability | Status | Evidence |
| --- | --- | --- |
| Per-handler `Server-Timing` header (DevTools/RUM-visible) | 🟢 TRIAL ADOPTED (2026-11-09) | webui.go:421 mounts `ServerTimingMiddlewareWhen`; header ships under RequestLog, pinned requestlog_test.go:28–38 |
| Auto `total` (TTFB) metric | 🟢 TRIAL ADOPTED | rides the mount (server_timing.go:345–348); requestlog.go:23 still logs the slog-only duration alongside |
| Named sub-metrics from handler internals | 🟢 TRIAL ADOPTED | `facts` + `stats` segments recorded before the first write (handlers.go:166–173, 204–209) |
| SSE-safe wrapper (Flush/Hijack passthrough) | 🟢 TRIAL ADOPTED (required: webui is SSE-first) | server_timing.go:275–303 delegatingWriter; "SSE-safe" note in the mount comment webui.go:420 |
| Conditional enablement (debug/role predicate) | 🟢 TRIAL ADOPTED (matches tq's loopback + token posture) | the predicate IS the RequestLog flag (webui.go:421–423; README:62) |
| Injection-safe header serialization | ⚪ N/A (no tq-owned header producer to harden) | fuzz-tested per README:64 |

## 3. Findings (buy vs cost)

1. **Buy:** zero-dependency module (server_timing/go.mod requires nothing) — adoption touches one go.mod (internal/webui lives in the root module) plus a ./vendor/ resync; the header lands where browser DevTools already look, no vendor agent; the `When` predicate could tie it to the existing `--auth-token`/loopback distinction so a LAN-exposed dashboard does not broadcast its fold-lag numbers.
2. **Buy:** it composes as a plain `func(http.Handler) http.Handler`, so it slots into webui's `Handler()` chain without touching any handler body — proven: the mount landed at webui.go:416–423 (2026-11-09), and `MeasureServerTiming` is a no-op when the middleware is absent, so instrumentation landed in handlers ahead of / alongside the middleware (handlers.go:166, 204).
3. **Cost:** the "measure before you write" gotcha (README:68–79) — the header is stamped at first `Write`; deferred `stop()` misses it except on SSE streams. Handlers must be disciplined at the instrumented call sites — both are (comments at handlers.go:170–172, 208). Low value in principle: the dashboard is a loopback personal tool with one user; latency surprises are debugged with `--request-log`.
4. **Adoption site** (webui, gated) — implemented 2026-11-09 as written, fleet review f21:

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

This is the rare review where the library's capability set and tq's gap aligned one-to-one: tq measured nothing per-handler, the module does exactly that and nothing else, and its SSE passthrough matches the dashboard's streaming architecture. What kept it at WORTH A TRIAL rather than ADOPT NOW was demand, not fit — a single-operator loopback dashboard gets little from header-based latency visibility, and httpapi's four routes (httpapi.go:193–201) are thin enough that timing would mostly measure SQLite. The trial ran exactly as shaped — mounted behind the `RequestLog` flag on `tq serve` (webui.go:421–423), `handleFacts` and the stats read instrumented (handlers.go:166, 204) — and earned KEEP; remove it if the fold-vs-read split never explains anything and nobody opens DevTools on this surface.

## 5. Maturity + version currency

Latest server_timing/v1.0.3 (tag commit 192539f 2026-10-24 — re-verified 2026-11-11 with `git log -1 … server_timing/v1.0.3^{commit}`). Six submodule tags (v0.9.1, v0.10.0, v0.12.0, v1.0.1, v1.0.2, v1.0.3), still the full set as of 2026-11-11 — the 1.x line is ~3 releases old, MIT since v1.0.2 (checkout AGENTS.md:327: owner decision 2026-10-08). Stdlib-only, fuzz + bench + spec tests in the checkout (server_timing_{fuzz,bench}_test.go — files present 2026-11-11). Consumed by the httputil root module itself (httputil go.mod: `httputil/server_timing v1.0.3`) and now by tq (root go.mod:126), so it has two permanent in-house consumers.

## 6. Conclusion

Trial-worthy and only in the webui, gated — which is exactly how it was adopted (2026-11-09 trial, KEEP for now). It is the cheapest observability upgrade `tq serve` ever gets (zero deps, ~10 lines) and deliberately independent of the root httputil module this repo declined. The gate stays: never mount it unconditionally on the token-gated API — Server-Timing on an internet-exposed surface is a free performance oracle for attackers (their README's own security note).

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
grep -rn 'httputil/server_timing' --include=go.mod . | grep -v vendor      # → go.mod:126: server_timing v1.0.3 (trial dep, kept)
grep -rni 'server-timing\|servertiming' --include='*.go' internal cmd      # → webui.go mount + handlers.go facts/stats segments + requestlog_test pin
sed -n '13,29p' internal/webui/requestlog.go                               # the pre-trial latency surface (slog only)
sed -n '416,423p' internal/webui/webui.go                                  # the mount, RequestLog-gated
go test ./internal/webui -run TestServerTimingHeaderBehindRequestLog -count=1  # → ok  github.com/larsartmann/go-taskqueue/internal/webui 0.009s (executed 2026-11-12 post-repair 5e7fc982, rc=0)
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
`total` under RequestLog, absent without). Verified in-tree 2026-11-11:
mount webui.go:416–423; segments handlers.go:166/204; pin
requestlog_test.go:16–38 (committed at HEAD; cannot BUILD from today's
working tree only because the sqlite/postgres migration breaks the vendored
root build — unrelated to the trial).

Keep-or-remove: revisit when (a) the read-model fold-vs-read split ever
needs explaining in anger, or (b) RequestLog usage shows nobody opens
DevTools on this surface — remove is a 3-line revert plus the dep.

