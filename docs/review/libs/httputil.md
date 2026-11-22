# httputil — Library Utilization Review

**Verdict:** NOT ADOPTED — KEEP OUT — every load-bearing middleware tq needs (token gate, CSRF, lockout, CSP nonce) is already solved, test-pinned, and deliberately shaped for these surfaces; httputil would duplicate, not replace.

> **2026-11-22 refresh:** re-verified at tq HEAD 1c8219e6 — verdict unchanged. Library side moved: NEW ROOT TAG **v1.9.0** (2026-11-14, tag commit 0e62158; the 11-11 refresh saw v1.8.0 as newest — now 30 root tags). v1.9.0 re-litigated against tq's surfaces: (a) CSRF now treats HTTP QUERY (RFC 10008) as spec-safe (BEHAVIOR CHANGE) — tq has zero QUERY-method routes (grep `http.Method` in internal/ → POST/GET only) and its CSRF is POST-only double-submit bound to server-rendered templ forms (auth.go:140–199), so nothing tq pins crosses the change; (b) compression no longer strips net/http's Content-Type sniffing (issue #49) — tq mounts no compression and has no Accept-Encoding handling anywhere (grep → zero hits), repairing a capability this review already rules N/A; (c) Pusher delegation + `Capabilities.Pusher`, `KeyedRateLimiterConfig.Now`, `httpspec.ExpectNotModifiedDynamic` are additive and land where tq either owns the semantic (lockout ≠ keyed limiter, §3) or lacks the need. [Unreleased] recommends building on go1.27.4+ (GO-2026-6859 / CVE-2026-97054, an `http.MaxBytesReader` crash) — directly relevant to tq, which wraps `http.MaxBytesReader` itself (httpapi.go:330, 418): tq already bumped to `go 1.27.4` (root go.mod:3, commit 5ed73719, 2026-11-13). tq side: webui.go grew under budget-meter/health-row work — Handler() now 397–449, securityHeaders 459–471, CSP string 461–463; auth.go drifted (withTokenAuth 75–108, withCSRFIssue 140–169, withCSRF 176–199); httpapi route block now 195–198; AGENTS.md ruling cite → :424; checkout AGENTS.md MIT record → :348. Checkout HEAD = v1.9.0-24-g34dbf05 (2026-11-21; the 24 post-tag commits are chores/docs plus an unreleased endpoints Accept-line fix).

> **2026-11-11 refresh:** all claims re-verified; verdict unchanged. What moved since 2026-11-08: (a) AGENTS.md's stale "proprietary" note is FIXED — AGENTS.md §Relation to other projects now reads "**httputil** NOT adopted (MIT v1.5.0+; fleet verdicts: docs/review/libs/, 2026-11-09 …)"; (b) the server_timing trial has LANDED (root go.mod:126 pins v1.0.3; mounted webui.go:416–423), so the family is no longer dependency-absent for tq — the root module itself is still unimported; (c) the trial mount shifted webui.go line cites (Handler() now 377–431, securityHeaders 439–450); (d) httputil checkout HEAD moved 616b62f→9dd1f89 (flake.lock only, 2026-11-10).

**Pinned:** none for the root module — but the family is no longer absent from the tree: root go.mod:126 pins the server_timing submodule (`github.com/larsartmann/httputil/server_timing v1.0.3`, the 2026-11-09 trial, kept). The root module itself is unimported — verified 2026-11-22 by `grep -rn 'larsartmann/httputil' --include=go.mod . | grep -v vendor` → the single go.mod:126 hit, and import greps hit only the two servertiming imports (internal/webui/webui.go:34, handlers.go:18); no `"github.com/larsartmann/httputil"` import exists (2026-11-08 review greps: zero hits — pre-trial).

**Latest:** v1.9.0 (local checkout /home/lars/projects/httputil; NEW since the 11-11 refresh — tag commit 0e62158, 2026-11-14; re-verified 2026-11-22 by `git -C /home/lars/projects/httputil tag -l | grep -c '^v'` → 30 root tags, newest v1.9.0; checkout HEAD is 34dbf05 2026-11-21 = v1.9.0-24-g34dbf05 — chores/docs plus an unreleased endpoints Accept-line fix since the tag)

**Prior coverage:** AGENTS.md §Relation to other projects records the decision — updated 2026-11-09 to drop the stale "proprietary" reason: "go-health-dashboard (MIT): ADOPTED — /health inside the token gate …; **httputil** NOT adopted (MIT v1.5.0+; fleet verdicts: docs/review/libs/, 2026-11-09 — … server_timing trial KEEP)". The MIT relicense is checkout fact (LICENSE; owner decision 2026-10-09, checkout AGENTS.md:348, CHANGELOG [1.5.0]), and AGENTS.md no longer misstates it; the conclusion stands on overlap grounds alone (§3).

**Reviewed:** 2026-11-08 by reading /home/lars/projects/httputil/{README.md,go.mod,LICENSE,AGENTS.md}, plus internal/webui/{webui,auth,requestlog}.go, internal/httpapi/httpapi.go, internal/httpauth/httpauth.go, internal/lockout/lockout.go.

## 1. Where the need exists today

httputil is a composable stdlib-net/http middleware kit: Chain/Compose/MiddlewareStack, CORS, ClientIP, ResponseRecorder (now with Pusher delegation), SecurityHeaders + CSP nonce, RequestID, Recovery, Timeout, Logging, Compression, Decompression with bomb protection, ETag (via go-etag), server lifecycle, health checks, CSRF (nosurf), keyed rate limiting, language negotiation, error classification (README:9, feature sections README:54–571; go.mod: justinas/nosurf + go-error-family + go-etag/server + httputil/server_timing + golang.org/x/time).

The code it would touch is tq's two HTTP surfaces, both hand-rolled:

- **Dashboard chain** (internal/webui): `Handler()` composes withCSRFIssue → withTokenAuth → withSecurityHeaders → RequestLog-gated Server-Timing (the 2026-11-09 trial) → optional withRequestLog (webui.go:416, 429, 434, 436–444, 446–448 — cites re-verified 2026-11-22; the block was 377–431 at the 11-11 refresh); AllowWrites write routes are mux-registered with the rate-limit wrap (webui.go:418–427). Token auth with session cookie for SSE subresources (internal/webui/auth.go:75–108), double-submit CSRF bound to server-rendered templ forms (auth.go:140–169 issue, 176–199 verify), strict CSP with per-request nonce (webui.go:459–471), 3-strikes write limiter (auth.go:202–218, internal/lockout).
- **Machine API** (internal/httpapi): `guard()` enforces nosniff, per-client lockout with Retry-After, constant-time bearer-token compare, and `Cache-Control: no-store` on every authed response (internal/httpapi/httpapi.go:209–251); 4 routes on a plain ServeMux (httpapi.go:195–198); body caps via http.MaxBytesReader (httpapi.go:330,418); shared primitives in internal/httpauth (HashToken/TokenMatches/Policy, httpauth.go:45–119) and internal/lockout (lockout.go:22–28: 3 hits / 1 min / 1024 keys).

## 2. Capability inventory (fit)

| Capability | Status | Evidence |
| --- | --- | --- |
| Bearer-token auth gate | ⚪ N/A (httputil has none; tq's core contract — httpapi.go:209–251, auth.go:75–108) | httputil README offers CSRF/rate-limit, no token middleware |
| CSRF protection | 🟡 overlap | tq form-embedded double-submit (auth.go:140–199) vs nosurf header-token — different mechanism, wired into templ rendering |
| Security headers + CSP nonce | 🟡 overlap | tq securityHeaders + nonce (webui.go:459–471) pinned by internal/webui/security_test.go, headermatrix_test.go |
| Keyed rate limiting / lockout | 🟡 overlap | tq internal/lockout (3-strikes lockout, lockout.go:22–28) ≠ httputil x/time fixed-window limiter; shapes pinned by TestAuthPlaneBodyShape |
| Panic recovery middleware | 🔴 MISSING-BUT-RELEVANT (low: net/http isolates per-connection panics; tq chains have none) | no Recovery in webui.go:397–449 or httpapi.go:195–198 |
| Request ID | 🔴 MISSING-BUT-RELEVANT (low) | requestlog.go:13–29 logs method/path/status/duration, no ID |
| Response compression | ⚪ N/A (loopback-first dashboard, small fragments, SSE) | no Accept-Encoding handling anywhere in webui/httpapi |
| CORS | ⚪ N/A (deliberately same-origin: CSP connect-src 'self', webui.go:461–463) | — |
| Client IP behind proxy | 🟡 overlap | tq remoteHost reads RemoteAddr only, documented as accepted (httpapi.go:283–292) |
| Server-Timing / endpoints listing / ETag | Server-Timing: trial mounted + kept 2026-11-09 (webui.go:441–444); endpoints/ETag keep-out | docs/review/libs/httputil-{server-timing,endpoints}.md, docs/review/libs/go-etag.md |
| Health checks | ⚪ N/A (already adopted go-health-dashboard at /health, webui.go:319–352) | AGENTS.md §Relation |

## 3. Findings (buy vs cost)

1. **Buy:** one hardened, behaviorally-spec-tested middleware battery (~70 linters README:900, 98.7% coverage badge README:5, govulncheck-clean badge README:6 — verified in the checkout 2026-11-22); panic recovery and request IDs are tq's only real gaps; error classification via go-error-family, already an indirect dep of cmd/tq (cmd/tq/go.mod:77).
2. **Cost (dominant):** adopting any security-relevant piece means swapping a pinned contract for a near-miss — nosurf's header-token CSRF would break the server-rendered templ write forms (auth.go:140–169 issues tokens into GET pages; v1.9.0's QUERY-as-safe change does not help here: tq registers no QUERY routes); the keyed rate limiter has no lockout semantics (3-strikes + Retry-After is the API's documented auth plane, httpapi.go:216–227); httputil has **no token gate at all**, so `guard()` and `withTokenAuth` stay regardless. Net effect: httputil could only wrap the *outside* of chains tq already owns — Recovery + RequestID + maybe Compression — while adding 4 dependencies (root go.mod is curated; ./vendor/ must be resynced) and a second error-code style (go-error-family's const-code sentinels vs tq's stdlib errors.New).
3. **Cost:** AGENTS.md documents a deliberate NOT-adopted decision, refreshed 2026-11-09 with the fleet verdicts (MIT v1.5.0+, server_timing trial KEEP — AGENTS.md §Relation to other projects). The license question that motivated the original note is resolved; flipping the NOT-adopted verdict itself would still need an owner ruling, and nothing above motivates one.
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

httputil is a good library aimed at greenfield services that need a default security posture. tq is the opposite case: two small surfaces whose auth, CSRF, lockout, nosniff and CSP behavior are each load-bearing, documented (ADR-0003, ADR-0008, AGENTS.md §Architecture package table), and pinned by dedicated tests (internal/webui/security_test.go, headermatrix_test.go, internal/httpapi TestAuthPlaneBodyShape). The only capabilities tq lacks (Recovery, RequestID, compression) are worth perhaps 40 lines of local code and no new dependencies; every capability tq cares about is either absent from httputil (token gate) or semantically different (CSRF mechanism, rate-limit semantics). Fit is poor not because the library is weak but because the problem is already solved in a way tq's tests depend on.

## 5. Maturity + version currency

Latest tag v1.9.0 (tag commit 0e62158 2026-11-14; 30 root tags v0.1.0 (2026-06-08) → v1.9.0 — re-verified 2026-11-22; checkout HEAD 34dbf05 2026-11-21 = v1.9.0-24-g34dbf05, chores/docs + the unreleased endpoints Accept fix since the tag). MIT since v1.5.0 (owner decision 2026-10-09; checkout AGENTS.md:348). Go floor: go directive `go 1.27` with a 1.27.1+ effective floor since go-etag v0.5.0 (checkout README:11) — tq modules declare go 1.27.4 (root go.mod:3, bumped in lockstep 2026-11-13 by 5ed73719), so no toolchain conflict; note the checkout's own [Unreleased] recommends go1.27.4+ for consumers of its `MaxBodySize` middleware (CVE-2026-97054 in `http.MaxBytesReader`) — tq, which wraps `http.MaxBytesReader` directly at httpapi.go:330/418, is already there. Maintained same-author alongside go-etag and server_timing submodules (the latter now also a tq dependency, v1.0.3, via the kept trial).

## 6. Conclusion

Keep out. The "proprietary" note this review flagged has since been corrected in AGENTS.md (§Relation ruling bullet, now :424), and the suggested server_timing trial has been run and kept (companion review) — but the adoption verdict stands for the same reason: tq's HTTP surfaces are already smaller, test-pinned, and exactly shaped for their contracts than anything a general middleware kit could swap in. v1.9.0 (2026-11-14) was re-litigated above and moves nothing. If a concrete gap ever itches (panic recovery, request IDs), write the ~40 lines locally rather than adopting the root module.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
grep -rn 'larsartmann/httputil' --include=go.mod . | grep -v vendor          # → go.mod:126 ONLY (server_timing v1.0.3; root module absent)
grep -rn 'larsartmann/httputil' --include='*.go' . | grep -v vendor          # → the two servertiming imports only (webui.go:34, handlers.go:18)
grep -rn '"github.com/larsartmann/httputil"' --include='*.go' . | grep -v vendor  # → no output (root module unimported)
grep -n 'NOT adopted' AGENTS.md                                         # line 424: "**httputil** NOT adopted (MIT v1.5.0+; fleet verdicts…"
sed -n '348,350p' /home/lars/projects/httputil/AGENTS.md                     # MIT relicense record
head -3 /home/lars/projects/httputil/LICENSE                                 # MIT
git -C /home/lars/projects/httputil tag -l | grep -c '^v'                    # 30 root tags (newest v1.9.0, 2026-11-14)
git -C /home/lars/projects/httputil describe --tags                          # → v1.9.0-24-g34dbf05 (HEAD 2026-11-21)
sed -n '397,449p' internal/webui/webui.go                                    # tq's hand-rolled chain incl. the server_timing mount (441–444)
sed -n '209,251p' internal/httpapi/httpapi.go                                # guard(): tq owns token+lockout+nosniff
```
