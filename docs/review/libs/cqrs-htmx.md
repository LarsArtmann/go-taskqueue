# cqrs-htmx — Library Utilization Review

**Verdict:** NOT ADOPTED — KEEP OUT — it is a CQRS command/query HTTP framework with event-sourced user management; tq's HTTP surfaces are a read-only templ/Datastar dashboard and a 4-route machine API over store calls — the library's premise (dispatch-boundary handlers, users/tenants) is a problem tq deliberately does not have.

**Pinned:** none (not a dependency; verified 2026-11-08 by `grep -rn 'larsartmann/cqrs-htmx' --include='*.go' --include=go.mod . | grep -v vendor` → no output, including docs).

**Latest:** root v4.18.1 (2026-11-07); modules: huma/v4.0.0 (newest, 2026-11-07), usermgmt/v4.20.0, usermgmt/webauthn/v4.13.0, usermgmt/oauth2/v4.13.0, usermgmt/totp/v4.12.1, adminui/v4.17.0, dashboardui/v4.17.0, loginpage/v4.17.0, setup/v4.17.0, datastar/v4.12.2, health/v4.14.0, auditlog/v4.12.2, systemadapter/v4.13.0 (local checkout /home/lars/projects/cqrs-htmx, checked 2026-11-08 by `git tag -l '<mod>/*' | sort -V | tail -1`; 248 tags, last commit 2026-11-08).

**Prior coverage:** none for cqrs-htmx (sibling reports cover go-etag/go-health/etc.). Capability summary cross-checked against Lars's own adoption doc /home/lars/.config/crush/skills/cqrs-htmx/SKILL.md (read 2026-11-08; SSE-only per upstream ADR-0046 — WebSocket transport dropped).

**Reviewed:** 2026-11-08 by reading /home/lars/projects/cqrs-htmx/{CHANGELOG.md,go.mod,docs/adr/0046*,usermgmt,adminui,dashboardui} + the SKILL.md, plus tq's internal/webui/{webui,handlers,hub,components,health}.go, internal/httpapi/httpapi.go, docs/adr/0003-web-ui-architecture.md + 0008-http-api.md references in AGENTS.md.

## 1. Where the need exists today

tq's three HTTP surfaces and what they hand-roll:

- **`tq serve` dashboard** (internal/webui, ADR-0003): 8 GET routes (webui.go:249-257) + 3 opt-in CSRF-guarded POST admin routes (webui.go:298-304) rendered with templ + templ-components (ADR-0018); live updates via ONE journal tailer → hub → SSE (tailer.go, hub.go), with `Last-Event-ID` resume; go-health-dashboard (ADOPTED) mounts `/health*` on top (webui.go:317-321). HTMX is deliberately DISABLED — the layout props blank it, the board streams over vanilla EventSource, and only the health page rides Datastar, same-origin, CSP-pinned (components.go:633-646, health.go:20-32).
- **`tq api` machine API** (internal/httpapi, ADR-0008): exactly 4 routes — `POST /api/v1/tasks`, `POST /api/v1/errors`, `GET /api/v1/stats`, `GET /api/v1/healthz` (httpapi.go:195-198) — token-mandatory, lockout-guarded, `{error, fix}` JSON failure shape pinned by test, `Cache-Control: no-store` on every authed response (httpapi.go:247). Handlers are direct store/readmodel calls; there is no command/query dispatcher on this path.
- **Admin surfaces**: the webui POST routes above (cancel/rescue/answer) — domain actions, not user administration. tq has no users, tenants, roles, sessions, or login page anywhere.

What cqrs-htmx is (SKILL.md + root go.mod): an `App` builder that binds CQRS command/query dispatch to HTTP (`app.Command(...)`, `DecodeJSON`, error→RFC 9457 mapping), an SSE broadcaster, embedded HTMX assets, OpenAPI generation, and — the bulk of the module tree — event-sourced usermgmt (WebAuthn/OAuth2/TOTP, Casbin authz), adminui, loginpage, setup.

## 2. Capability inventory (fit)

| Capability | Status | Evidence / reason |
| --- | --- | --- |
| Command/query handler builder + dispatch middleware | ⚪ N/A — tq has no HTTP dispatch boundary; api handlers call the store directly (httpapi.go:195-198) |
| SSE broadcaster (drop stats, reconnect) | ⚪ N/A — webui hub + `metaengine.ServeSSE` already own this (hub.go; readmodel/model.go:640), purpose-built for TaskRow fragments |
| httperr → RFC 9457 error documents | 🟡 overlap — httpapi has its own pinned `{error, fix}` JSON contract (ADR-0008; httpapi_test.go TestAuthPlaneBodyShape); adopting would CHANGE a tested wire contract |
| Embedded HTMX assets / htmx extensions | ⚪ N/A — webui deliberately blanks HTMX (vanilla EventSource board; Datastar only on the adopted health page — components.go:633-646, ADR-0018) |
| ETagResults conditional query results (v4.18.1) | ⚪ N/A — same verdict as go-etag: the authed API is `no-store` by contract (httpapi.go:247) |
| OpenAPI 3.1/3.2 generation (+ huma typed ops) | 🔴 nearest miss — the machine API has no published spec; huma/v4.0.0 would generate one from typed handlers |
| usermgmt (+webauthn/oauth2/totp, Casbin) | ⚪ N/A — tq is single-operator; auth = one machine token + lockout (httpauth, lockout) |
| adminui (users/tenants/members/audit) | ⚪ N/A — no identity domain to administer |
| dashboardui (event-store observability panel) | 🟡 overlap — the closest module (read-only, projections health, live SSE), but tq's webui is domain-rich (board, evidence-stage filters, agent feed, verify receipts) on a different UI stack (templ/Datastar vs HTMX) |
| setup / loginpage / datastar / health / auditlog / systemadapter bridges | ⚪ N/A — health is already served by adopted go-health-dashboard; setup composes usermgmt panels tq does not have |

## 3. Findings (buy vs cost)

1. **Buy (the only genuine candidates):** (a) a generated OpenAPI contract for `tq api` (huma adapter, v4.0.0) — today the 4-route surface is documented in ADR-0008 prose only; (b) dashboardui's projection-health paneling — but every signal it shows (fold lag, engine health, DLQ counts) is already surfaced through `/api/v1/healthz`'s projection section (httpapi.go:479) and the webui health page.
2. **Cost (dominant — it is the wrong shape):** cqrs-htmx's value concentrates in the dispatch boundary (authorize → validate → guard → dispatch) and the identity domain. tq's writes are 5 POST routes total (3 webui + 2 api) with hand-written guards that ARE the tested contract (TestRoutesAreReadOnly, TestAuthPlaneBodyShape); its reads are projections behind templ fragments. Adopting the App builder would re-route trivially-few handlers through a framework whose per-handler stages exist to tame domains tq does not have.
3. **Cost — dependency + philosophy:** root module carries casbin, go-playground/form, ginkgo/gomega, httputil, go-etag, go-idempotency, go-sse in its direct require block (go.mod:6-27) — a large vendor set for 4 API routes. The UI philosophy also conflicts: cqrs-htmx is HTMX-first (SSE-only transport, ADR-0046); tq webui has REJECTED HTMX (templ-components + EventSource/Datastar, ADR-0018, components.go:633-646) — adopting would fork the frontend stack inside one dashboard.
4. **License/stance note:** PROPRIETARY (same author/policy as go-cqrs-lite — ADR-0014 records the owner-instruction exception for the platform). cqrs-htmx has no such instruction here; per the httputil ruling (AGENTS.md), tq keeps not-yet-instructed proprietary deps out.
5. **Fleet-convergence note (what adoption WOULD stitch together):** cqrs-htmx's root pins the exact sibling libs the sibling reviews ruled on — go-etag (KEEP OUT here), go-idempotency (KEEP OUT), httputil (NOT adopted), and its `health` bridge builds on the go-health family tq DID adopt — so if the owner ever standardizes fleet-wide on cqrs-htmx, tq's adopted subset (go-health-dashboard) composes cleanly first, and the rest arrives as one decision, not piecemeal bumps.

## 4. Fit assessment

No score — the library is not adopted, so there is no utilization to measure; this section assesses fit instead. cqrs-htmx solves "expose a CQRS domain to browsers with login, roles, and admin tooling". tq's domain exposure is a read-only operator dashboard (templ + Datastar, already ADR-0003/0018-ruled) and a token-gated 4-route producer API (ADR-0008) whose wire contracts are pinned by tests. Every overlap the library offers (SSE, error mapping, conditional GET, health paneling) is already owned by an adopted or purpose-built surface (metaengine ServeSSE, go-health-dashboard, `{error,fix}` JSON). Fit: correct library, absent problem.

## 5. Maturity + version currency

Very healthy: 248 tags, latest root v4.18.1 and huma/v4.0.0 both 2026-11-07, commits the day of this review; RFC 10008 QUERY support, ETag'd query results, asset sha-pins, and a strict re-export policy (ADR-0062) all landed within the last week (CHANGELOG.md:1-60). Go floor 1.27.3 — matches tq's. Were the verdict ever revisited, the huma adapter alone (no usermgmt, no HTMX) is the only slice that maps onto an existing tq surface.

## 6. Conclusion

Keep out. tq is a queue, not a web app: no users, no dispatch-over-HTTP, and a dashboard whose stack was deliberately chosen to be something else. Revisit only on a concrete trigger: (a) `tq api` grows enough routes that a published OpenAPI contract becomes an operational need (→ huma adapter slice), or (b) tq ever grows multi-user operation (→ usermgmt would then be the fastest correct path, and that is an owner decision, not a dependency bump).

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
grep -rn 'larsartmann/cqrs-htmx' --include='*.go' --include=go.mod . | grep -v vendor   # → no output
sed -n '249,257p' internal/webui/webui.go        # the 8 read-only dashboard routes
sed -n '195,198p' internal/httpapi/httpapi.go    # the 4 machine-API routes
sed -n '633,646p' internal/webui/components.go   # HTMX deliberately blanked (Datastar stack)
cd /home/lars/projects/cqrs-htmx
git tag -l | sort -V | tail -1                   # → v4.18.1
git tag -l 'huma/*' | sort -V | tail -1          # → huma/v4.0.0
git log -1 --format='%h %ad' --date=short        # → 2026-11-08
head -30 docs/adr/0046-drop-websocket-sse-only.md  # SSE-only ruling
```
