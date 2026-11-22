# go-etag — Library Utilization Review

**Verdict:** NOT ADOPTED — KEEP OUT — tq's surfaces deliberately serve `Cache-Control: no-store` (machine API) and SSE (dashboard); conditional GET solves a problem this architecture chose not to have.

> **2026-11-11 refresh:** verdict unchanged, but the surface drifted since 2026-11-08: the health dashboard gained the repo's first in-tree ETag producer — `datastar.ScriptHandler` serves conditional GETs with ETags on `/health/datastar.js` (health.go:450–452, import health.go:22–23; header + 32-hex-digest shape pinned headermatrix_test.go:319–334; landed 2026-11-09), and the favicon's `max-age=86400` pin moved (headermatrix_test.go:315–316; rationale comment health.go:443). Both come from the ADOPTED go-datastar/go-health-dashboard stack, not go-etag. httpapi's `no-store` (httpapi.go:247), the SSE posture, and the facts-window niche (handlers.go:139–195) are all unchanged; go-etag remains unimported (go.mod + import greps → no output, re-run 2026-11-11). go-etag checkout: v0.7.0 still latest of 18 tags; HEAD moved to 6e53195 2026-11-10 (TODO_LIST-only commit).

> **2026-11-22 refresh:** verdict unchanged (KEEP OUT) — **new tag v0.7.1** (2026-11-18; tags 18 → 23, HEAD now `v0.7.1-5-g39f9f37`, Unreleased empty). v0.7.1's substance does not touch tq's fit: a README/docs symbol checker, a file-layout refactor with "package paths and the public API … unchanged" (CHANGELOG), CI/toolchain pins, an error-family v0.12.0 bump, a settled minor-form `go 1.27` directive policy, and the root shim moving to `server` v0.6.2 — no new capability that tq's no-store/SSE architecture could consume, so the §3.4 flip-trigger is re-stamped verbatim. tq-side re-verified at HEAD 1c8219e6: go-etag still absent from every go.mod and import grep; httpapi's `no-store` holds (httpapi.go:247); the repo's one conditional-GET surface is still the adopted datastar stack (`datastar.ScriptHandler` now at health.go:603, imports health.go:22-23, ETag shape pins headermatrix_test.go:320-334, favicon max-age pin :315-316 with rationale comment now health.go:594) — drifted line numbers fixed below; httputil's root module now requires `go-etag/server v0.6.2` (rode the same nested train), and the etagmetrics→metrics changelog record moved to httputil CHANGELOG:218-228.

**Pinned:** none (not a dependency; verified 2026-11-08 by `grep -rn 'larsartmann/go-etag' --include=go.mod . | grep -v vendor` → no output, same for import greps; re-verified 2026-11-22 by the same greps — still no output, and no go-etag anywhere in the tree; the only httputil-family dep is server_timing, root go.mod:126)

**Latest:** v0.7.1 (local checkout /home/lars/projects/go-etag, checked 2026-11-22 by `git -C … tag -l | sort -V | tail -1` → v0.7.1 of 23 tags; checkout HEAD `v0.7.1-5-g39f9f37` 2026-11-18, [Unreleased] empty)

**Prior coverage:** none directly; AGENTS.md §Relation to other projects mentions httputil (which embeds go-etag) as NOT adopted. go-health-dashboard is the adopted HTTP-adjacent lib.

**Reviewed:** 2026-11-08 by reading /home/lars/projects/go-etag/{README.md,server,client,metrics} plus internal/httpapi/httpapi.go, internal/webui/{handlers,static,health}.go, and greps for etag/If-None-Match (zero hits repo-wide).

## 1. Where the need exists today

go-etag is a two-sided RFC 7232 library: `go-etag/server` middleware buffers GET/HEAD bodies, computes an FNV-64a ETag, and answers `If-None-Match` with 304 (weak comparison, HEAD handling, Content-Length-on-304 correctness); `go-etag/client` is a RoundTripper cache that replays validators and rebuilds 304s as 200s per RFC 9111; `entitytag` is the shared domain type; `metrics` ships atomic hit-ratio counters (README:1–30, 32–60 — structure re-verified 2026-11-11).

The tq surfaces it would touch:

- **Machine API GETs** — `GET /api/v1/stats` and `GET /api/v1/healthz` (internal/httpapi/httpapi.go:197–198; handlers at 450–473/475). But `guard()` stamps **`Cache-Control: no-store` on every authenticated response** (httpapi.go:247 — re-verified 2026-11-22) — the API's contract is explicitly "never cache this": stats move with every fold, and go-etag's own client honors no-store (README:39: "Never stores `Cache-Control: no-store` responses (RFC 9111 §3)"). A server-side ETag on a no-store endpoint is dead weight: the semantics tq chose already answer this question.
- **Dashboard GETs** — `/api/stats` (internal/webui/handlers.go:203–222) and `/api/facts?after=SEQ&limit=N` (handlers.go:139–195). The dashboard is SSE-driven (ADR-0003: tailer→hub→SSE fragment swaps, webui.go:1–5); browsers do not re-GET these endpoints, the page subscribes once. The genuinely immutable surface is a facts *window* (`after=N&limit=M` returns a deterministic journal slice) — but nothing in-repo or documented polls it; it exists for the infinite-scroll journal viewer (handlers.go:139–143).
- **Static assets** — served by `http.FileServerFS` (internal/webui/static.go:12–19), which already provides Last-Modified-based conditionals from net/http; the health favicon pins `Cache-Control: public, max-age=86400` — the production Set-site is the adopted dashboard library's handler (go-health-dashboard favicon.go:23, `w.Header().Set("Cache-Control", "public, max-age=86400")` — re-verified 2026-11-22; tq mounts it at webui.go healthBindings `FaviconHandler()`), verified tq-side by headermatrix_test.go:315–316 + rationale comment health.go:594. Since 2026-11-09 the datastar SDK bundle `/health/datastar.js` serves FULL ETag conditional GETs via the adopted stack's `datastar.ScriptHandler` (health.go:603, comment :601; ETag shape pinned headermatrix_test.go:320–334) — tq's one conditional-GET surface exists, and it is built on go-datastar, not go-etag.
- **Who polls?** ADR-0008 names the consumers: non-Go producers and "remote dashboards" (docs/adr/0008-http-api.md; httpapi.go:3–5). No in-repo poller exists (grep for /api/v1/stats consumers → docs only). The webui's AnswerPoller and bridges are push/subscribe shapes, not conditional-GET clients.

**Cross-reference on the author's own usage:** httputil's root module depends on `go-etag/server v0.6.2` (httputil go.mod:8 — re-verified 2026-11-22; it rode go-etag's v0.7.1 nested-module train, was v0.6.1 at the 11-11 refresh), and the `etagmetrics/v0.1.0` tag in the httputil checkout records that its metrics helper was moved INTO go-etag as the `metrics` package at v0.5.0 (httputil CHANGELOG:218–228) — including a fixed `HitRatio()` double-count bug found during the move. Pattern: Lars consumes go-etag in general-purpose web middleware, not in event-driven tools like tq.

## 2. Capability inventory (fit)

| Capability | Status | Evidence |
| --- | --- | --- |
| Server ETag + 304 for JSON GETs | 🔴 MISSING-BUT-RELEVANT (only for the immutable `/api/facts` windows; stats are no-store by contract) | httpapi.go:247 (`no-store`); handlers.go:139–195 |
| Weak/strong If-None-Match comparison, HEAD correctness | 🔴 MISSING-BUT-RELEVANT (same niche; the 2026-11-09 datastar SDK ETag covers the adopted health stack only, not tq-owned routes) | etag greps hit only health.go:594,601,603 + headermatrix_test.go:320–334 (re-grepped 2026-11-22); zero in httpapi |
| Client-side conditional-GET cache (RoundTripper) | ⚪ N/A (tq's HTTP clients are push consumers — bridges POST alerts; no polling GET loop) | internal/bridge/papdashboard (POST alert URL) |
| Metrics hooks (hit ratio, overflow) | ⚪ N/A (no middleware to instrument) | go-etag/metrics README |
| Streaming/bounded buffering (MaxBufferSize) | ⚪ N/A (bodies already capped: http.MaxBytesReader, httpapi.go:330,418) | — |

## 3. Findings (buy vs cost)

1. **Buy:** correct-by-construction RFC 7232/9111 handling — hand-rolling 304s (weak comparison, HEAD, no body on 304, Vary interplay) is exactly the class of subtle bug this library hardens; the client transport would make any future polling consumer free of bandwidth on unchanged responses.
2. **Cost (dominant):** it contradicts a stated contract. `Cache-Control: no-store` on the authed API (httpapi.go:247) is deliberate — a queue's stats are the last thing a cache should serve stale — and the dashboard's live updates flow over SSE, not re-GETs. An ETag middleware mounted under no-store produces tags nothing may ever conditionally use; loosening no-store to make the middleware useful would trade correctness of the API's freshness story for bandwidth nobody is missing.
3. **Cost:** new dependency edges — `go-etag/server` pulls go-error-family + entitytag (go-etag go.mod requires, verified 2026-11-11); interacts with SSE ordering rules (their own httputil README:531,539 documents ETag-after-Language and innermost-placement constraints — a chain tq doesn't have and shouldn't build for this).
4. **The one genuine niche:** `/api/facts?after=N&limit=M` windows are immutable journal slices — the ideal 304 candidate. But the only consumer is the dashboard's own journal viewer, in-page, scroll-driven. If a third-party poller ever appears on `/api/facts`, mount `etag.New(etag.DefaultETagConfig())` on that one route:

```go
// internal/webui/handlers.go — facts route only, NOT under the SSE stream
mux := http.NewServeMux()
mux.HandleFunc("GET /api/facts", s.handleFacts)
instrumented := etag.New(etag.DefaultETagConfig())(mux) // 304s for unchanged journal windows
```

## 4. Fit assessment

go-etag is a mature, well-scoped library whose subject matter — conditional caching of reusable representations — is adjacent to nothing tq currently serves. The machine API is explicitly no-store; the dashboard is push (SSE); static files already get Last-Modified conditionals from net/http. The single defensible application (immutable facts windows) has no polling consumer inside or outside the repo today. Fit is "correct tool, absent problem": revisit only if a documented consumer starts re-GETting `/api/facts` windows or if the no-store stance is ever relaxed for `/api/v1/stats`.

## 5. Maturity + version currency

Latest v0.7.1 of 23 tags (checked 2026-11-22: `git -C /home/lars/projects/go-etag tag -l | sort -V | tail -1` → v0.7.1, released 2026-11-18 with the nested-module train server/client/entitytag/metrics v0.6.2); HEAD `v0.7.1-5-g39f9f37` 2026-11-18 — actively maintained. v0.7.1 is infra/docs-shaped: the doc-snippet-refs checker, a file-split refactor with "package paths and the public API … unchanged", CI lint/toolchain pins (closing GO-2026-6859 via go1.27.4 GOTOOLCHAIN pins), an error-family v0.12.0 bump, the minor-form `go 1.27` directive policy (owner ruling 2026-11-18 — a patch-form floor "re-poisons every consumer's go directive through MVS"), and the root shim moving to `server` v0.6.2 (CHANGELOG §0.7.1). MIT (LICENSE). Four submodules (server/client/entitytag/metrics); root module is a deprecated alias shim removed in v1.0.0 (README:15; root go.mod:5 requires server v0.6.2). The metrics-package migration history (etagmetrics → go-etag/metrics at v0.5.0, with a fixed HitRatio bug — httputil CHANGELOG:218–228) shows the author dogfoods and corrects it. Go directive `go 1.27` across the modules with the README badge matching (README:5, "Go-1.27+", re-read 2026-11-22) — under tq's 1.27.4 (root go.mod:3).

## 6. Conclusion

Keep out. No adopted surface wants conditional caching, and the one that forbids it (`no-store` on httpapi) is the load-bearing contract. The library is healthy and correctly scoped — it is simply aimed at request/response polling services, which tq is not. Revisit on the narrow trigger in §3.4.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
grep -rn 'larsartmann/go-etag' --include=go.mod . | grep -v vendor        # → no output
grep -rni 'etag\|if-none-match' --include='*.go' internal cmd | grep -v vendor | grep -vi 'aggregate' # → only the health surface's datastar-ETag pins (headermatrix_test.go:320–334, health.go:594,601,603) — adopted-stack, not go-etag; zero httpapi hits (re-run 2026-11-22)
grep -n 'Cache-Control' internal/httpapi/httpapi.go internal/webui/health.go  # no-store (247) vs SDK max-age comment (594; served value pinned headermatrix_test.go:316)
sed -n '139,195p' internal/webui/handlers.go                             # the immutable facts-window niche
git -C /home/lars/projects/go-etag tag -l | sort -V | tail -1            # v0.7.1
git -C /home/lars/projects/go-etag log -1 --format='%h %ad %s' --date=short  # 39f9f37 2026-11-18
head -3 /home/lars/projects/go-etag/LICENSE                              # MIT
```
