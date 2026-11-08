# httputil/endpoints — Library Utilization Review

**Verdict:** NOT ADOPTED — KEEP OUT — four httpapi routes and a webui that already keeps its route table as data don't need a self-describing `/endpoints` listing from a one-tag, week-old module.

**Pinned:** none (not a dependency; verified 2026-11-08 by `grep -rn 'httputil/endpoints' --include=go.mod . | grep -v vendor` → no output, same for import greps)

**Latest:** endpoints/v0.1.0 (local checkout /home/lars/projects/httputil/endpoints, checked 2026-11-08)

**Prior coverage:** none (third member of the httputil family; root module reviewed at docs/review/libs/httputil.md — KEEP OUT; server_timing reviewed at docs/review/libs/httputil-server-timing.md — WORTH A TRIAL)

**Reviewed:** 2026-11-08 by reading /home/lars/projects/httputil/endpoints/{README.md,endpoints.go} against internal/httpapi/httpapi.go, internal/webui/webui.go, docs/adr/0008-http-api.md.

## 1. Where the need exists today

The module is a `Registry` that mounts each route on a stdlib `*http.ServeMux` AND records it, then serves `GET /endpoints` — JSON for curl, HTML table for browsers, chosen by RFC 9110 proactive negotiation (`Vary: Accept`, `Cache-Control: no-store`); zero dependencies, wiring errors are values not panics (endpoints/README.md:1–9, 21–44; API at endpoints.go:93–195).

The need it would fill — a listing that "can never drift from what is actually served" (README:7) — maps onto:

- **internal/httpapi**: exactly four routes registered inline (internal/httpapi/httpapi.go:193–201: POST /api/v1/tasks, POST /api/v1/errors, GET /api/v1/stats, GET /api/v1/healthz), documented once in docs/adr/0008-http-api.md:22–24 and the `tq api` usage text (cmd/tq/main.go usage block).
- **internal/webui**: route tables ALREADY exist as data — `routeBindings()` (webui.go:239–259), `healthBindings()` (webui.go:265–283), `writeBindings()` (webui.go:288–302) — kept as data specifically so the ADR-0003 read-only guardrail test (`TestRoutesAreReadOnly`) can prove no mutating handler is registered (webui.go:234–238). The dashboard therefore already has the registry pattern, minus the HTTP listing.

## 2. Capability inventory (fit)

| Capability | Status | Evidence |
| --- | --- | --- |
| Single-declaration route mount + record | 🟡 partial overlap | webui's `routeBindings` slice is the same pattern, test-driven (webui.go:239–259); httpapi's 4 inline routes are small enough to see at a glance |
| Self-describing `GET /endpoints` (JSON/HTML negotiated) | 🔴 MISSING-BUT-RELEVANT (marginal: 4 routes, documented in ADR-0008) | httpapi.go:193–201 |
| Drift-proof listing (mount IS the record) | ⚪ N/A (ADR-0008 is the accepted source of truth for the machine API) | docs/adr/0008-http-api.md |
| Concurrent-safe registry | ⚪ N/A (routes registered once at startup in both tq surfaces) | endpoints/README.md:47–50 |
| CSP-safe HTML listing | ⚪ N/A (nothing to render) | README:40 |

## 3. Findings (buy vs cost)

1. **Buy:** machine-discoverable API surface — a producer could `curl /endpoints` instead of reading ADR-0008; zero deps; `Vary: Accept` + `no-store` handled correctly out of the box (README:33–37).
2. **Cost (dominant):** maturity. One tag (endpoints/v0.1.0, cut 2026-11-06 — two days before this review; tag commit 02e6591 "docs: mark the endpoints first-release items resolved"). The repo's own AGENTS.md:327 calls the endpoints pkg.go.dev page brand new ("the `endpoints` page arrives with its first tag"). Adopting a v0.1.0 router-shape dependency into a surface whose routes are security-relevant is premature.
3. **Cost:** their README's own security note — "the listing documents your attack surface" (README:43–44). tq's httpapi is token-mandatory precisely because it writes (httpapi.go:1–8, 72–74); adding a route whose whole purpose is to enumerate that surface behind the token gate buys nothing for attackers-resistance-neutral visibility, and the webui (the surface with many routes) is a browser dashboard that already links its own pages.
4. **Cost:** a second source of truth. ADR-0008 documents the API contract including deferred designs (fencing tokens, consumer groups — docs/status/archived/2026-09-08 M22 note); an auto-generated listing cannot record that context, so drift moves from "ADR vs code" to "ADR vs listing vs code".
5. **Adoption site if it ever happens** (httpapi only):

```go
// internal/httpapi/httpapi.go Handler()
routes := endpoints.NewRegistry()
for _, r := range []struct{ p string; h http.HandlerFunc; s string }{
    {"POST /api/v1/tasks", s.handleEnqueue, "Enqueue one task (dedupKey passthrough)"},
    {"GET /api/v1/stats", s.handleStats, "Per-status counts + total"},
    // …
} {
    if err := routes.HandleFunc(mux, r.p, r.h, r.s); err != nil {
        return nil, err // wiring errors are values
    }
}
if err := endpoints.Register(mux, routes); err != nil { return nil, err } // mounts GET /endpoints
```

## 4. Fit assessment

The library solves a real problem — `*http.ServeMux` cannot enumerate its patterns — for services with enough routes that a listing pays for itself. tq is not that: the machine API has four routes fully specified in an ADR, and the dashboard already maintains route tables as data for a stronger reason (the read-only guardrail test). The genuine future trigger would be the API growing toward ADR-0008's deferred designs (fencing tokens, consumer groups), where a drift-proof listing starts earning its keep. Until the module also matures past v0.1.0, the four routes don't justify a new module edge, a vendor resync, and a security-surface question.

## 5. Maturity + version currency

Latest endpoints/v0.1.0 — the only tag (checked 2026-11-08; `git -C /home/lars/projects/httputil tag -l 'endpoints/*'`). Tag commit 02e6591, 2026-11-06; module MIT from creation (checkout AGENTS.md:327: "endpoints MIT from creation 2026-11-06"). Zero dependencies (endpoints/go.mod). Test coverage in the checkout is real (fuzz_test.go, bench_test.go, qvalue corpus) but the public API surface is one release old.

## 6. Conclusion

Keep out and revisit when either (a) the module has a 0.2+/1.0 track record, or (b) `tq api` grows past a handful of routes and a self-describing listing beats ADR-0008 for onboarding producers. Neither condition holds today.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
grep -rn 'httputil/endpoints' --include=go.mod . | grep -v vendor       # → no output
grep -rn 'httputil/endpoints' --include='*.go' . | grep -v vendor       # → no output
sed -n '193,201p' internal/httpapi/httpapi.go                           # the entire machine-API route table
sed -n '234,259p' internal/webui/webui.go                               # webui already keeps routes as data
git -C /home/lars/projects/httputil tag -l 'endpoints/*'                # endpoints/v0.1.0 (single tag)
git -C /home/lars/projects/httputil log -1 --format='%h %ad %s' --date=short endpoints/v0.1.0^{commit}
head -3 /home/lars/projects/httputil/endpoints/LICENSE                  # MIT
```
