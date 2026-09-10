# Status Report — httputil Assessment + Permissions-Policy Port

**Session:** 2026-09-10, ~02:10–03:05 CEST · **Repo:** go-taskqueue @ master
**Trigger:** "Could we benefit from /home/lars/projects/httputil/?"
**Method:** READ → UNDERSTAND → RESEARCH → REFLECT → execute the one worthwhile change → verify.

---

## Session summary in one paragraph

Assessed the sibling HTTP middleware library `httputil` for adoption into
go-taskqueue. Verdict: **NOT adopted as a dependency** — (1) its LICENSE is
**Proprietary** while this repo is MIT with an all-MIT dependency tree, so a
code-level `require` would poison every redistributed `tq` binary; (2) scope
mismatch — `tq serve` is a loopback SSE dashboard whose bespoke middlewares
are already stricter than httputil's generic defaults. The comparison surfaced
exactly one real gap — a missing `Permissions-Policy` header — which was
ported natively into `securityHeaders`, tested, and documented. Full root
gate re-run green (12/12 packages, `-race`, exit 0).

---

## a) FULLY DONE

1. **httputil dependency assessment, verdict recorded.**
   - Read httputil's README, FEATURES.md, `security.go`, go.mod, LICENSE, tags (latest v0.9.1).
   - Decisive finding: httputil LICENSE = PROPRIETARY; every existing taskqueue dep (go-sse, templ-components, go-retry, go-error-family, go-branded-id) = MIT. First proprietary dep would be a licensing regression for an MIT repo.
   - Scope comparison done header-by-header: taskqueue's CSP (`default-src 'none'`, per-request nonce, `form-action 'none'`) is strictly stronger than httputil's `RecommendedCSP` (`default-src 'self'`); `Referrer-Policy: no-referrer` stronger than httputil's `strict-origin-when-cross-origin`; SSE requires `WriteTimeout=0` + Flush forwarding that httputil's stack doesn't model; nosurf-based CSRF would displace the ADR-0003-reviewed CSRF/lockout pair for zero gain.
   - Evidence: decision record appended to `AGENTS.md` § "Relation to other projects" (committed by auto-daemon; grep-verified present post-commit).
   - Scope: AGENTS.md only.

2. **`Permissions-Policy` security header ported natively** (the one real gap).
   - `Permissions-Policy: camera=(), microphone=(), geolocation=()` set in `securityHeaders` — internal/webui/webui.go:231 — on every response including 401s and SSE (wrapper sits outermost except request logging).
   - Rationale in code: device capabilities a task-queue dashboard never needs; closes embed/iframe drift.
   - Evidence: `TestSecurityHeadersOnEveryResponse` extended (internal/webui/security_test.go) asserting the exact value across `/`, `/task/nope`, `/api/stats`, `/static/definitely-missing.css`; dedicated run green, then full suite green.

3. **Docs synced (append-only conventions respected).**
   - FEATURES.md: security-headers row now lists Permissions-Policy.
   - CHANGELOG.md: `[Unreleased] → Added` entry.
   - AGENTS.md: full adoption verdict + "re-run only if license changes or a second HTTP surface appears" trigger.
   - Evidence: grep-verified all three files carry the new content; survived auto-daemon commits (`f988396`, `29d6c95`).

4. **Verification gates green.**
   - `gofmt -l internal/webui/` → clean; `go vet ./internal/webui/` → clean.
   - `go test ./... -race -count=1` (root module) → **12 packages ok, exit 0, zero FAIL lines**, exit code captured via file redirect (not a filtered pipeline).
   - Scope: internal/webui/{webui.go, security_test.go} + three docs.

---

## b) PARTIALLY DONE

1. **Benefit extraction from httputil (pattern-level, not dependency-level).**
   - Works: the header-parity sweep produced one concrete port (Permissions-Policy); the license finding is durably recorded so future sessions don't re-litigate.
   - Open: the remaining httputil design space (Request-ID log correlation, `ParseUintQuery`-style helpers, health endpoints, server-lifecycle wrapper) was assessed as not-worth-it but that judgment lives only in a condensed AGENTS.md note, not a formal ADR.
   - Blocker: none; effort to formalize = S. Priority: only if a second HTTP surface appears (per the recorded trigger).

2. **httpapi (`tq api`) security posture — observed, intentionally not touched this session.**
   - Works: bearer auth mandatory (fail-closed), constant-time compare, `Cache-Control: no-store`.
   - Open: `internal/httpapi` sets **no** `X-Content-Type-Options: nosniff` and has **no** auth-failure lockout (unlike webui's 3-strikes CSRF lockout). Deliberately left out of scope (session focus was the httputil question against webui).
   - Blocker: a policy decision (see section g, Q2). Effort: M (lockout), S (nosniff).

---

## c) NOT STARTED

1. **HARVEST of section (f) into TODO_LIST.md / ROADMAP.md** — waiting for owner instruction (this report was written in "then wait" mode per the tasking). Without harvest, the (f) items die in this timestamped file.
2. **httputil re-license conversation** — nothing started; a licensing decision belongs to the owner alone (see g, Q1).
3. **Anything httputil-dependency-related** — deliberately zero code written toward adoption; no branch, no go.mod change.

---

## d) TOTALLY FUCKED UP

**Nothing in this session broke the tree** — full root `-race` suite green before, during (checkpoint builds), and after. Radical-honesty items that stop short of fucked-up:

1. **Pipeline-masked gate exit code (process slip, caught and corrected).**
   - First verification ran `go test ./... | rg -v ... | head; echo GATE_EXIT=$?` — `$?` captured `head`'s exit, not go test's. This repo's own memory warns about exactly this masking pattern.
   - Root cause: composing the gate through filters. Mitigation applied: re-ran with `> /tmp/tq-root-test.log; echo TEST_EXIT=$?` → explicit `TEST_EXIT=0`, `rg FAIL` → no matches. Severity: none in the end (result was genuinely green); the slip is repeatable-process debt, now re-demonstrated here so it stays expensive to repeat.

2. **Three wasted edit round-trips ("read the file before editing").**
   - Edited FEATURES.md and AGENTS.md from content obtained via grep/context instead of `view`; the edit tool (correctly) refused twice each. Cost: 3 dead turns.
   - Root cause: treated grep output as "read". The rule is view-then-edit, no substitutes.

---

## e) WHAT WE SHOULD IMPROVE

1. **View-then-edit discipline.** grep/rg output must never be treated as having "read" a file for edit purposes. Impact: wasted turns, risk of whitespace mismatches on exact-match edits. Fix: always `view` the target region immediately before `edit`, even when the content is "already known".
2. **Gate verification pattern.** Any CI-replicating command must be `cmd > log 2>&1; echo EXIT=$?` — never exit codes sampled from a filtered pipeline tail. Impact: false-green banners (this repo has been burned before; AGENTS.md documents it).
3. **Header asserts as a table.** `TestSecurityHeadersOnEveryResponse` asserts headers one `if` at a time; a header→want map would make the next header a one-line addition. Impact: S effort, pays off every security-header change. (Deliberately not refactored this session — surgical-change policy on existing code.)
4. **Advisory lint debt in the function I touched.** `goconst` (`"GET"` ×8 in the route tables) and `varnamelen` (`h`) fire on webui.go; pre-existing baseline, left per the advisory policy — but the route-table `GET` could become a named constant next time the route table is touched.
5. **Duplication worth extracting: bearer-token extraction exists twice.** webui `presentedToken` (header/cookie/query + SHA-256 compare) vs httpapi `bearerToken` (header/query + `subtle` compare) — same contract, two implementations, drifting semantics (cookie support, compare mechanism). If a third HTTP surface appears, extract `internal/httpauth` first.
6. **SECURITY.md has no response-header inventory.** The serve-side header set is documented only in passing (line ~79) + a FEATURES row. A short per-surface header matrix (serve vs api) would make gaps like the missing Permissions-Policy (or httpapi's missing nosniff) visible at audit time.
7. **No dedicated test for `redactedRequestURI`** (the `?token=` log-redaction helper). The behavior is security-relevant (token must not land in access logs); if it's covered only incidentally, it deserves a pinned table test.

---

## f) Top things to get done next (ranked; HARVEST input — route to TODO_LIST/ROADMAP)

| #  | Task                                                                                                                                  | Impact | Effort | Category      |
| -- | -------------------------------------------------------------------------------------------------------------------------------------- | ------ | ------ | ------------- |
| 1  | HARVEST this report's (f) into TODO_LIST.md/ROADMAP.md so items don't die here                                                          | High   | S      | Documentation |
| 2  | Add `X-Content-Type-Options: nosniff` (+ keep `Cache-Control: no-store`) to all `internal/httpapi` responses                            | High   | S      | Security      |
| 3  | Decide + implement (or document-out) failed-auth lockout for `tq api` bearer guard, mirroring webui's 3-strikes CSRF lockout            | High   | M      | Security      |
| 4  | Add per-surface response-header matrix (serve vs api) to SECURITY.md                                                                    | Medium | S      | Documentation |
| 5  | Pin `redactedRequestURI` with a dedicated table test (token never lands in logs)                                                        | Medium | S      | Quality       |
| 6  | Refactor `TestSecurityHeadersOnEveryResponse` to a header→want table for one-line future additions                                     | Low    | S      | Quality       |
| 7  | Extract shared bearer-token extraction/compare into `internal/httpauth` (webui + httpapi convergence)                                   | Medium | M      | Cleanup       |
| 8  | Document (or change) the webui session-cookie decision: cookie stores the raw token value, not a hash                                   | Medium | S      | Security      |
| 9  | Route-table `GET` string → named constant (goconst advisory in webui.go route tables)                                                   | Low    | S      | Cleanup       |
| 10 | Remove/inline dead helpers flagged by gopls this session: `factLines`, `filterSuffix` (webui), `waitFor` (webui_test), `ptr` (httpapi_test) | Low    | S      | Cleanup       |
| 11 | Fix `hub.go:26` unnecessary type argument + `QF1003` tagged-switch suggestions in fragments_templ.go source                             | Low    | S      | Cleanup       |
| 12 | Elevate the "no generic HTTP middleware dependency" AGENTS.md note into a short ADR if/when a second HTTP surface lands                  | Low    | S      | Documentation |
| 13 | Ask owner to relicense httputil (see g/Q1); if yes → re-run the full comparison; if no → close the topic permanently                     | Medium | S      | Decision      |
| 14 | Add `Permissions-Policy` (and future headers) to the webui smoke assertions (`scripts/smoke/webui.sh`) so smoke ≠ unit-divergent         | Low    | S      | Quality       |
| 15 | Re-run `scripts/check-dead-exports.sh` after the dead-helper cleanup (#10) to confirm zero new orphans (substring matching, not `-w`)    | Low    | S      | Quality       |
| 16 | Consider a board/fragment-level header e2e: assert SSE stream responses carry CSP + Permissions-Policy over real HTTP                    | Low    | S      | Quality       |
| 17 | Document writeRateLimiter's per-remoteHost keying assumption (all clients behind one LAN proxy share a bucket) in SECURITY.md            | Low    | S      | Documentation |
| 18 | Triage the two gosec advisory findings visible in httpapi.go (G118, contextcheck) next time that file is touched                        | Low    | S      | Security      |
| 19 | Give `writeRateLimiter` a `Retry-After` rounding test (off-by-one boundary at lockout expiry)                                           | Low    | S      | Quality       |
| 20 | Add `tq serve` response-header assertions to the SSE reconnect path in webui_test (resume + headers together)                            | Low    | S      | Quality       |
| 21 | If httpapi grows CORS needs (browser producers), adopt per-origin allowlist ONLY — never `AllowAllOrigins` + credentials                 | Low    | M      | Feature       |
| 22 | CHANGELOG: when v0.2.0 cuts, fold the `[Unreleased]` Added block (incl. Permissions-Policy) into the release section                     | Medium | S      | Documentation |
| 23 | AGENTS.md: add one line to the templ-components adoption table only if a new library component lands (not yet — guard tests own it)      | Low    | S      | Documentation |
| 24 | Keep `docs/status/README.md` archive counter honest when this report's items resolve → move to `archived/` per docs-health ANNOTATE      | Low    | S      | Documentation |
| 25 | Re-verify the "concurrent agents" ritual held: `git log --oneline -5 -- internal` at next session start (this session landed on top of 4ce666e cleanly) | Low    | S      | Process       |

_(25 substantive items; the ask was "up to 50" — the honest count from this session's observations is 25, and padding the rest with filler would poison HARVEST routing. Items 2–8 are the actionable core.)_

---

## g) Questions I cannot answer myself (max 3)

1. **Will httputil be relicensed to MIT (or dual-licensed)?** I verified the Proprietary/LICENSE vs MIT/taskqueue conflict and that every other larsartmann dep is MIT. Whether to relicense (or carve out) is an owner-only licensing decision; the answer either reopens the dependency question or closes it permanently (AGENTS.md trigger).
2. **Is `tq api` (httpapi) contractually machine-only and exempt from brute-force lockout + security headers?** I observed the guard has no failed-attempt lockout and sets no nosniff — defensible for an authenticated machine API, but it's a threat-model/policy call (LAN exposure intent), not a code question.
3. **Should the webui auth session cookie carry a hash of the token instead of the raw token value?** I read the code (`Value: token`, HttpOnly+SameSite=Lax, Secure-on-TLS) and the gosec G124 baseline triage, but whether raw-value-in-cookie is a deliberate simplicity tradeoff or an accepted-risk oversight is owner knowledge.

---

## Verification log (this session)

- `go test ./internal/webui/ -run 'TestSecurityHeaders|TestRoutesAreReadOnly|TestCSPNonce' -count=1` → ok
- `go build ./... && go vet ./internal/webui/` → clean
- `go test ./internal/webui/ -race -count=1` → ok (10.4s)
- `go test ./... -race -count=1 > log` → `TEST_EXIT=0`, 12 packages ok, zero FAIL
- `gofmt -l internal/webui/` → clean
- Post-auto-commit grep: all five touched files still carry the new content

_Report scope honored: nothing outside this session's run was re-researched; adjacent observations (httpapi gaps, dead helpers) are from files this session actually read._
