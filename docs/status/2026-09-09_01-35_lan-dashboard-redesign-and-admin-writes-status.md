# Status: LAN Dashboard Fixes → UI Redesign → Admin Control

**Date:** 2026-09-09 01:35 CEST
**Session scope:** everything after the 22:42 fleet-onboarding report: live-view troubleshooting (CSP/cookies/ghosted app.js), the design critique + redesign (now band, two-tier table), and the control layer (`--allow-writes`, CSRF, cancel/rescue). Compares against the 22:42 report's own promises — two of its items are still open on purpose.
**Companion:** brutal-self-review answers folded into (d)/(e) per the single-report instruction.

---

## 0. One-paragraph summary

The dashboard went from a read-only generic admin template to a working operations console: LAN token auth now survives subresource requests (session cookie), the CSP admits only nonce'd inline scripts, the ghosted `app.js` SSE client is back, the page leads with an always-dark instrument band and a two-tier active/settled table, and — the big one — `tq serve --allow-writes` adds exactly two CSRF-guarded admin routes (cancel pending/running, rescue dead) with row-level affordances. Every piece was verified: unit suite race-green, smoke green, and a real scratch task cancelled through the live HTTP flow landing as `task.cancelled` with the reason in the fact detail. Along the way I broke the shared build twice in concurrent-agent files and fixed it; both incidents are in (d).

---

## a) FULLY DONE

| # | Work | Evidence |
| - | ---- | -------- |
| 1 | **LAN auth fixed** — `?token=` now issues an `HttpOnly SameSite=Lax` session cookie (`tq_token`); subresources (CSS/JS/favicon/SSE) authenticate via cookie; constant-time checks unchanged; `Secure` only over TLS | `internal/webui/auth.go`; tests `TestTokenAuthCookieSession`; live probe: css with cookie → `200 text/css`, no-token → 401 |
| 2 | **CSP nonce regime** — per-request 128-bit nonce, `script-src 'self' 'nonce-…'` (never `unsafe-inline`), published via request context, stamped onto templ-components' theme bootstrap + ThemeToggle inline scripts | `webui.go` `withSecurityHeaders`/`ctxNonce`; `TestCSPNonceCoversInlineScripts` (nonce present, ≥2 stamped, per-request freshness) |
| 3 | **Ghosted app.js restored** — the 09-07 templ-components sweep (`a9768c2`) had silently dropped `<script src="/static/app.js" defer>`; page rendered once, SSE never ran, all gates stayed green because they fetched app.js directly | Restored in `layout.templ`; pinned by `TestPageLoadsAppJS` so regeneration cannot lose it silently again |
| 4 | **Now band** — StatusCards' 7-card grid replaced by an always-dark instrument strip: RUNNING (cyan, glow when hot) / PENDING / DEAD (alarms red only when non-zero) / COMPLETED / CANCELLED as ledger segments; total/journal/budget as quiet meta readouts; project chips below | Screenshot-verified (`/tmp/ui/final*.jpg`); marker classes (`card-*`) preserved so all existing tests held |
| 5 | **Two-tier task table** — `ACTIVE NOW` (running/pending, cyan pulse on running) always visible; settled rows folded into a native `<details>` ("26 completed · 7 cancelled") — the 30-row wall is gone; project cells stopped wrapping; cancelled feed lines dim | `taskActiveCard`/`taskSettledDetails`; visible in screenshots |
| 6 | **Admin writes, opt-in** — `tq serve --allow-writes` (`$TQ_SERVE_WRITES=1`) registers exactly `POST /task/{id}/cancel` (pending → `Cancel` with reason; running → cooperative `CancelRunning`) and `POST /task/{id}/rescue` (`RescueDead`, fresh budget 1–10); banner states writes-enabled | Live e2e: scratch task cancelled over real HTTP → `task.cancelled` fact with reason `live e2e verify`; banner prints "(writes ENABLED…)" |
| 7 | **CSRF for the write forms** — `withCSRFIssue` issues `tq_csrf` on page GET + publishes to render chain; forms embed it; `withCSRF` verifies field↔cookie constant-time; forged token → 403 | `TestWriteFlowCancelAndRescue` covers forged rejection + all three mutations end-to-end |
| 8 | **Row/detail affordances** — actions column after status (visible, not clipped): `stop` (running) / `cancel` (pending) / `rescue` (dead) open native `<details>` reason forms — zero JS under CSP; actions strip on the detail page | Screenshot shows STOP on the running row, CANCEL on the pending review |
| 9 | **Styled 404** — `task not found` renders through the layout with a way back (was a bare `http.Error` void) | `TaskNotFoundPage` (concurrent agent's version kept; my duplicate removed) |
| 10 | **Tests + docs** — cookie/CSRF/write-flow/route-absence/app-js/CSP-nonce tests added; webui suite race-green; `scripts/smoke/webui.sh` green; ADR-0003 amendment written; AGENTS.md guardrail bullet updated to describe the shipped escape hatch | Commit `5d9f1fd` (4 files) + daemon-swept companion commits |

## b) PARTIALLY DONE

| # | Work | Works now | Remains open | Blocker / effort |
| - | ---- | --------- | ------------ | ---------------- |
| 1 | Dark mode | Full `dark:` variant surface + my band is theme-independent by design; ThemeToggle is library-tested | **Never visually verified** — headless firefox `prefers-color-scheme` override didn't take; both screenshots are light | S/M — needs a working headless-dark profile or a `?theme=` override |
| 2 | My table changes × concurrent agent's table refactor | Final render shows their header set + my actions column working together; suite race-green at commit time | After my commit, other agents' in-flight edits (`executor/status.go` syntax error, harvest drift.go) left the tree transiently unbuildable — **not my packages**, unverified whether settled | Their WIP — S once they land |
| 3 | Writes UX | Cancel/stop/rescue from table rows + detail page | No bulk actions, no rescue-all, no enqueue-from-UI (deliberate — scope) | Awaiting owner priority (§g2) |
| 4 | Session hardening | Cookie issued per browser, HttpOnly, Lax | No `Max-Age` (session-scoped by design?), no rotation, no revocation story | Owner policy question (§g1) |
| 5 | Two TODO items queued at 22:42 (daemon-backed discovery, watch trigger) | Discovery implementation was observed mid-flight (`internal/harvest/discovery.go` + tests, pool agent) | Not verified complete; watch trigger not started | Pool's own backlog — ongoing |

## c) NOT STARTED

| # | Item | Why | Still wanted? |
| - | ---- | --- | ------------- |
| 1 | **`tq bootstrap` parity check** — carried over from the 22:42 report §f2; the sibling-repo rails are still hand-rolled, never compared against the repo's own onboarding tool | Forgotten again under redesign pressure | Yes — S, should be first |
| 2 | Mobile/narrow-viewport pass (the band wraps, the table will need the overflow scroll everywhere) | Desktop screenshot only | Yes — S–M |
| 3 | CHANGELOG entry for the webui/redesign/writes work | Daemon churn ate the tree; docs beyond ADR/AGENTS untouched | Yes — S |
| 4 | Browser-driven e2e for the write flow (real form fill → submit → assertion) | Verified via python HTTP, not a real browser form | Nice-to-have — the `playwright-chromium-headless-shell.drv` already sits in the nix store |
| 5 | SECURITY.md update for the writes surface (blast radius now includes UI-originated cancels) | Missed | Yes — S |

## d) TOTALLY FUCKED UP (radical honesty — all fixed, all mine)

| # | What | Severity | Root cause | Fix |
| - | ---- | -------- | ---------- | --- |
| 1 | **I repeatedly mangled `cmd/tq/main.go`** — the banner edit went through bash heredoc→python with `\\n` escaping that collapsed into real newlines inside Go string literals; three consecutive build breaks in a file TWO other agents were editing simultaneously. At one point I "fixed" it into a duplicated read-only branch | Medium — shared hotspot file, transient red builds for everyone | Wrong tool + wrong escaping path (heredoc python) for a surgical string edit in a hot file; should have used the edit tool from the start after one clean read | Final state verified: `if *allowWrites {…} else {…}` compiles, banner prints correctly |
| 2 | **Appended a duplicate `TaskNotFoundPage`** — wrote my own 404 templ without grepping for an existing one; the concurrent agent had already shipped theirs → `redeclared in this block` build error | Low-Medium | Wrote before researching the current tree state (the file had changed under me minutes earlier — I KNEW the tree was hot) | Kept theirs, deleted mine; should have been: grep symbol → read → extend |
| 3 | **My taskRows string-replace produced a malformed row** — the status-cell patch duplicated the badge cell and left a stray `</td>` → templ parse error | Low (caught by generate immediately) | I patched templ with python string surgery instead of precise edit-tool anchors on freshly-read content | Rewrote the block properly with asserted anchors; lesson in (e) |
| 4 | **Trusted stale evidence twice**: declared the redesign verified from a screenshot served by a stale binary; then read `actions -> True` from HTML and nearly repeated the mistake — markup present ≠ affordance visible (the column was CLIPPED beyond the card edge) | Medium — false "done" claims to the operator | Screenshot taken against the wrong process generation; probe checked existence, not visibility | Re-shot after restart; moved the actions column next to status (the clip-prone tail was the design bug) |
| 5 | **My cookie test needed two rewrites** — asserted "exactly 1 cookie" before the CSRF issuer existed, then asserted "0 re-issued" when the CSRF cookie legitimately appears on fresh browsers | Low | Wrote the test against an imagined final surface instead of the actual two-cookie contract | Test now asserts exactly `tq_token` + `tq_csrf`, and no session re-issue |
| 6 | **Raced a concurrent agent redundantly** — updated the AGENTS.md adoption table for icons my band removed; their edit had already done it (assert-failed anchor). Also watched their board test fail mid-TDD and nearly "fixed" their in-flight test | Low | Didn't re-check sibling edits before editing shared docs/tests | Asserted anchors now fail loudly — that IS the mitigation |
| 7 | **Process-control rabbit hole** — the bash `kill` builtin is unsupported in this shell; burned three attempts (including `kill -9`) before switching to `python3 os.kill`. Also briefly had two `tq serve` instances fighting over 8090 | Low | Reaching for the habitual tool instead of checking what works in this environment | python `os.kill` is the reliable path here |
| 8 | **Carried-over miss**: the 22:42 report's own §f2 (bootstrap parity check) did not happen — it was ranked first in "next" and still isn't done | Medium (process honesty) | Redesign pressure ate the follow-through | §f1 here, again, ranked first |

**Did I lie to you?** No. But twice I presented verification that was weaker than it sounded: "verified" screenshots against stale binaries (d4), and "actions present" from an HTML probe while the visible UI showed nothing — the column was clipped, which is itself a real design bug the probe masked.
**Ghost systems?** One caught + fixed this segment: the un-referenced `app.js` (dead SSE client behind green tests). None new: the write routes are wired, tested, and live-verified.
**Split brains?** The table now has two authors (their headers, my actions column) — currently consistent, but it's the hottest file in the repo with three agents in it; expect friction.
**Tests?** 6 new tests, all meaningful; but I wrote the cookie test against an imagined contract (d5) — write tests against the built surface, not the plan.

## e) WHAT WE SHOULD IMPROVE

1. **In hot files: read → grep symbols → edit tool. Never heredoc-python string surgery.** The two build breaks were both "wrote code without checking the current tree state" (d1, d2).
2. **Binary freshness before visual claims**: rebuild → restart → verify the serving PID → THEN screenshot. A screenshot is evidence only of the process that served it.
3. **Probe what the user SEES, not what the HTML contains** — clipped columns, `visibility`, and overflow made an HTML probe lie. Screenshot > grep for UI truth.
4. **Both themes, every time** — dark mode shipped unverified; the fix is a working headless-dark profile captured once and reused.
5. **Tests mirror the final surface** — write the assertion list after the feature's full shape exists (cookies ×2, not 1).
6. **Process control here = python `os.kill`** — the shell builtin doesn't do signals; stop re-trying it.
7. **When the tree is hot, commit early in small slices** — my 4-file commit rode on top of daemon sweeps of 11–37 files; attribution and revertability suffered.

## f) Up to 50 things we should get done next (brainstorm — ROADMAP fuel; ~top-12 are real queue candidates)

1. **`tq bootstrap` parity check vs the hand-rolled sibling-repo rails** (carried over twice now — do it first)
2. Verify the pool agents' in-flight work landed clean (`executor/status.go`, harvest drift, discovery feature) and the tree builds at HEAD
3. Dark-mode visual verification (working headless-dark profile) + polish any broken `dark:` variants
4. Mobile/narrow-viewport pass: band wrap, table overflow-x scroll everywhere, actions column on small screens
5. CHANGELOG entry for: LAN auth fixes, redesign, `--allow-writes` control layer
6. SECURITY.md: the writes blast radius (UI-originated cancels/rescues, CSRF model)
7. Browser e2e for the write forms via the in-store `playwright-chromium-headless-shell`
8. CSRF cookie policy: `Max-Age`, rotation on token change, revocation on `--auth-token` change
9. Bulk actions: rescue-all / rescue-older-than from the DLQ section (CLI parity)
10. Stop-request surfacing: a running task with a pending cancel request should SHOW it in the table (currently only the fact feed knows)
11. `last error` column: stop clipping mid-word — tooltip exists, but a fixed-width ellipsis + detail-link is honest
12. Worker liveness insight: distinct lease owners of running tasks in the band ("2 workers")
13. Budget as a progress bar in the band, not `2/5` text
14. Fact feed: pause button + jump-to-now (SSE already streams; needs client state)
15. Table: live-ticking age cells (data-age exists; needs a tiny interval in app.js)
16. Copy-task-id button on rows and detail page
17. Rescue max-attempts validation errors surfaced in the form (currently a bare 500)
18. Write-action audit strip: last N cancels/rescues with reasons on the dashboard
19. `/api` write endpoints documentable for scripting (same CSRF or token-header path)
20. Filter chips: keyboard focus states + `/` focus already exists — verify focus rings against CSP nonce'd styles
21. Board view (concurrent agent's): finish aria-current toggle test, then drag = Phase D writes behind the same gate
22. Deterministic screenshot harness: check-in script that serves a seeded fixture DB and captures light/dark/desktop/mobile
23. Resolve the sdk TODO split brain (carried over: queue-facing checkboxes vs status tables)
24. Overview × tq cross-link (carried over, parked)
25. Multi-model fleet decision (carried over §g1 from 22:42 — still unanswered)
26. `--repo-model` feature if §g2 answer wants per-repo model routing
27. `tq doctor` check: serve writes-flag on LAN without token (config lint)
28. systemd unit sample with `--allow-writes` + token env
29. Rate-limit the write endpoints (3 failed CSRF = short lockout) — cheap brute-force hygiene
30. Cookie `Secure` + HSTS notes for TLS-terminating proxies
31. Confirm-dialog semantics for cancel (no-JS: the open `<details>` IS the confirm — document it)
32. Table column preferences (persist which columns are hidden) — only if operators ask
33. Fact feed virtualization if journal bursts (>500 lines) ever jank
34. `TestNixGates`-style coverage for the new band classes (CSS drift gate already covers app.css; add class-presence assertions)
35. Delete the stale `/tmp/tq-fleet-trial.db` + old serve logs (hygiene)
36. README screenshot refresh (the UI looks different now — the README should sell the band)
37. Demo gif/GIF-loop of a live task moving pending→running→completed (HyperFrames skill exists)
38. Accessibility pass on the new band/details/actions (focus order, aria-expanded on `<details>`)
39. `prefers-reduced-motion` respect for the new glow/pulse effects (the pulse already gates; glow is static — fine; document)
40. i18n audit — deliberately none (single-operator tool); record the decision
41. Pool config file docs for the 4-repo fleet (carried over from onboarding report)
42. Per-repo `--repo-timeout overview=60m` in the persistent fleet launch (carried over)
43. Review-loop visibility: filter "needs changes" reviews to the top of ACTIVE (they're terminal but actionable)
44. `tq tasks --since` CLI (shipped by an agent mid-session? noticed in usage text — verify + smoke)
45. Nightly screenshot diff CI job (catch silent UI regressions like the app.js ghost)
46. Extract the shared "status colors" Go helper so band/badges/board cannot drift
47. Kill the `.tq-verify` gate duplication: sibling repos' gates vs `tq bootstrap` generated gates (ties to #1)
48. Document the concurrent-agent etiquette for webui (three of us edited one template tonight — a lock/claim comment convention may help)
49. Add `Task-Queue-ID` footer awareness to the UI: detail page could show the originating TODO item text (data exists in payload)
50. Decide the dashboard's name/tagline — "read-only projection" copy is now wrong in two places (footer fixed, ADR quotes remain)

## g) Top 3 questions I can NOT figure out myself

1. **Writes default policy:** should `--allow-writes` auto-enable for loopback-only binds (localhost is single-user; CSRF still guards) and stay opt-in only for LAN — or must it always be explicit, everywhere? This decides what `tq serve` means for every future script and the systemd unit.
2. **Control scope:** which actions belong in the UI next — bulk rescue/rescue-all, enqueue-from-UI (paste a task), cancel-with-reason on the band itself, or is cancel/stop/rescue the ceiling until multi-operator auth exists?
3. **Session & dark-mode priority:** do you want cookie `Max-Age`/rotation + a dark-mode visual pass NOW (polish tier), or should the next session go to the carried-over integration work (bootstrap parity, daemon discovery completion) first?

---

*Format override: `.md` per explicit instruction (skill default is HTML). Point-in-time snapshot — annotate, don't rewrite. Compares against `docs/status/2026-09-08_22-42_fleet-onboarding-agent-pool-integration-status.md`.*
