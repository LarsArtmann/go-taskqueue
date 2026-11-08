# httputil — Library Utilization Review

**Verdict:** NOT ADOPTED — KEEP OUT — every load-bearing middleware tq needs (token gate, CSRF, lockout, CSP nonce) is already solved, test-pinned, and deliberately shaped for these surfaces; httputil would duplicate, not replace.

**Pinned:** none (not a dependency; verified 2026-11-08 by `grep -rn 'larsartmann/httputil' --include=go.mod . | grep -v vendor` → no output, and the same for `--include='*.go'` import greps)

**Latest:** v1.8.0 (local checkout /home/lars/projects/httputil, checked 2026-11-08)

**Prior coverage:** AGENTS.md:399–401 records the decision: "go-health-dashboard (MIT): ADOPTED — /health inside the token gate …; **httputil** NOT adopted (proprietary)". Note: the "proprietary" reason is **stale** — the checkout LICENSE is MIT (owner decision 2026-10-09, visible from v1.5.0; recorded in the checkout's AGENTS.md:327 and CHANGELOG [1.5.0]) — but the conclusion stands on overlap grounds alone (§3).

**Reviewed:** 2026-11-08 by reading /home/lars/projects/httputil/{README.md,go.mod,LICENSE,AGENTS.md}, plus internal/webui/{webui,auth,requestlog}.go, internal/httpapi/httpapi.go, internal/httpauth/httpauth.go, internal/lockout/lockout.go.

## 1. Where the need exists today

httputil is a composable stdlib-net/http middleware kit: Chain/Compose/MiddlewareStack, CORS, ClientIP, ResponseRecorder, SecurityHeaders + CSP nonce, RequestID, Recovery, Timeout, Logging, Compression, Decompression with bomb protection, ETag (via go-etag), server lifecycle, health checks, CSRF (nosurf), keyed rate limiting, language negotiation, error classification (README:9, feature sections README:56–543; go.mod: justinas/nosurf + go-error-family + go-etag/server + httputil/server_timing + golang.org/x/time).

The code it would touch is tq's two HTTP surfaces, both hand-rolled:

- **Dashboard chain** (internal/webui): `Handler()` composes withCSRFIssue → write-route rate-limit wrap → withTokenAuth → withSecurityHeaders → optional withRequestLog (internal/webui/webui.go:327–372). Token auth with session cookie for SSE subresources (internal/webui/auth.go:75–108), double-submit CSRF bound to server-rendered templ forms (auth.go:136–199), strict CSP with per-request nonce (webui.go:380–435), 3-strikes write limiter (auth.go:212+, internal/lockout).
- **Machine API** (internal/httpapi): `guard()` enforces nosniff, per-client lockout with Retry-After, constant-time bearer-token compare, and `Cache-Control: no-store` on every authed response (internal/httpapi/httpapi.go:209–251); 4 routes on a plain ServeMux (httpapi.go:193–201); body caps via http.MaxBytesReader (httpapi.go:330,418); shared primitives in internal/httpauth (HashToken/TokenMatches/Policy, httpauth.go:45–119) and internal/lockout (lockout.go:22–28: 3 hits / 1 min / 1024 keys).

## 2. Capability inventory (fit)

| Capability | Status | Evidence |
| --- | --- | --- |
| Bearer-token auth gate | ⚪ N/A (httputil has none; tq's core contract — httpapi.go:209–251, auth.go:75–108) | httputil README offers CSRF/rate-limit, no token middleware |
| CSRF protection | 🟡 overlap | tq form-embedded double-submit (auth.go:136–199) vs nosurf header-token — different mechanism, wired into templ rendering |
| Security headers + CSP nonce | 🟡 overlap | tq securityHeaders + nonce (webui.go:380–435) pinned by internal/webui/security_test.go, headermatrix_test.go |
| Keyed rate limiting / lockout | 🟡 overlap | tq internal/lockout (3-strikes lockout, lockout.go:22–28) ≠ httputil x/time fixed-window limiter; shapes pinned by TestAuthPlaneBodyShape |
| Panic recovery middleware | 🔴 MISSING-BUT-RELEVANT (low: net/http isolates per-connection panics; tq chains have none) | no Recovery in webui.go:327–372 or httpapi.go:193–201 |
| Request ID | 🔴 MISSING-BUT-RELEVANT (low) | requestlog.go:13–29 logs method/path/status/duration, no ID |
| Response compression | ⚪ N/A (loopback-first dashboard, small fragments, SSE) | no Accept-Encoding handling anywhere in webui/httpapi |
| CORS | ⚪ N/A (deliberately same-origin: CSP connect-src 'self', webui.go:382–384) | — |
| Client IP behind proxy | 🟡 overlap | tq remoteHost reads RemoteAddr only, documented as accepted (httpapi.go:283–292) |
| Server-Timing / endpoints listing / ETag | covered by the three companion reviews | docs/review/libs/httputil-{server-timing,endpoints}.md, docs/review/libs/go-etag.md |
| Health checks | ⚪ N/A (already adopted go-health-dashboard at /health, webui.go:261–283) | AGENTS.md:399 |

## 3. Findings (buy vs cost)

1. **Buy:** one hardened, behaviorally-spec-tested middleware battery (~70 linters, 98.7% coverage badge, govulncheck-clean badge — checkout README:3–6); panic recovery and request IDs are tq's only real gaps; error classification via go-error-family, already an indirect dep of cmd/tq (cmd/tq/go.mod:77).
2. **Cost (dominant):** adopting any security-relevant piece means swapping a pinned contract for a near-miss — nosurf's header-token CSRF would break the server-rendered templ write forms (auth.go:140–168 issues tokens into GET pages); the keyed rate limiter has no lockout semantics (3-strikes + Retry-After is the API's documented auth plane, httpapi.go:216–227); httputil has **no token gate at all**, so `guard()` and `withTokenAuth` stay regardless. Net effect: httputil could only wrap the *outside* of chains tq already owns — Recovery + RequestID + maybe Compression — while adding 4 dependencies (root go.mod is curated; ./vendor/ must be resynced) and a second error-code style (go-error-family's const-code sentinels vs tq's stdlib errors.New).
3. **Cost:** AGENTS.md documents a deliberate NOT-adopted decision. The recorded reason ("proprietary") is outdated — MIT since v1.5.0 — but flipping a recorded decision needs an owner ruling, and nothing above motivates one.
4. **Adoption site if it ever happened** (outer wrappers only, everything security-critical stays tq's):

```go
// internal/httpapi/httpapi.go Handler(), outer wrap only
import "github.com/larsartmann/httputil"

return httputil.Chain(
    s.guard(mux), // tq token gate stays INNERMOST-owned — httputil cannot replace it
    httputil.Recovery(s.log),
    httputil.RequestID(httputil.DefaultRequestIDConfig()),
)
```

## 4. Fit assessment

httputil is a good library aimed at greenfield services that need a default security posture. tq is the opposite case: two small surfaces whose auth, CSRF, lockout, nosniff and CSP behavior are each load-bearing, documented (ADR-0003, ADR-0008, AGENTS.md:87–88), and pinned by dedicated tests (internal/webui/security_test.go, headermatrix_test.go, internal/httpapi TestAuthPlaneBodyShape). The only capabilities tq lacks (Recovery, RequestID, compression) are worth perhaps 40 lines of local code and no new dependencies; every capability tq cares about is either absent from httputil (token gate) or semantically different (CSRF mechanism, rate-limit semantics). Fit is poor not because the library is weak but because the problem is already solved in a way tq's tests depend on.

## 5. Maturity + version currency

Latest tag v1.8.0 (tag commit c4094bb 2026-11-03; 29 root tags v0.1.0→v1.8.0 since ~2026-05; last commit 616b62f 2026-11-06). MIT since v1.5.0 (owner decision 2026-10-09; checkout AGENTS.md:327). Go floor 1.27.1 — tq modules declare go 1.27.3 (go.mod:3), so no toolchain conflict. Maintained same-author alongside go-etag and server_timing submodules.

## 6. Conclusion

Keep out. The stale "proprietary" note in AGENTS.md should eventually be corrected to reflect the MIT relicense, but the adoption verdict is right for a different reason: tq's HTTP surfaces are already smaller, test-pinned, and exactly shaped for their contracts than anything a general middleware kit could swap in. If a concrete gap ever itches (panic recovery, request IDs), write the ~40 lines locally or trial the zero-dep `server_timing` submodule (see companion review) rather than the root module.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
grep -rn 'larsartmann/httputil' --include=go.mod . | grep -v vendor          # → no output
grep -rn 'larsartmann/httputil' --include='*.go' . | grep -v vendor          # → no output
grep -n 'NOT adopted' AGENTS.md                                         # line 400: "**httputil** NOT adopted"
sed -n '327,330p' /home/lars/projects/httputil/AGENTS.md                     # MIT relicense record
head -3 /home/lars/projects/httputil/LICENSE                                 # MIT
git -C /home/lars/projects/httputil tag -l | grep -c '^v'                    # 29 root tags
sed -n '327,372p' internal/webui/webui.go                                    # tq's hand-rolled chain
sed -n '209,251p' internal/httpapi/httpapi.go                                # guard(): tq owns token+lockout+nosniff
```
