# go-health — Library Utilization Review

**Verdict:** ADOPTED — low (5.5/17 applicable capability rows leveraged): the wire schema is the backbone of `tq serve`'s whole health surface, but the probe engine (caching, classification, latch, handlers, hardening) is re-implemented by hand next to it.

**Pinned:** v0.10.0 — root `go.mod:110` (direct); `cmd/tq/go.mod:79` (indirect). Verified 2026-11-08 by `grep -rn 'go-health' --include=go.mod . | grep -v vendor`.

**Latest:** v0.10.0 (local checkout /home/lars/projects/go-health, checked 2026-11-08; HEAD = v0.10.0-1-g7e5d8d1, one docs-only commit past the tag).

**Prior coverage:** none (docs/review/libs/ has only httputil.md; AGENTS.md:399 covers the bundle decision, not this library alone).

**Reviewed:** 2026-11-08 by reading /home/lars/projects/go-health/{README.md,CHANGELOG.md,doc.go,types.go,checks.go,handlers.go,options.go,lifecycle.go,cache.go,accessors.go,checks/checks.go,docs/check-severity-override-design.md} plus /home/lars/projects/go-taskqueue/internal/webui/{health.go,webui.go,health_test.go}; greps and tests listed in §How to verify.

## 1. Usage inventory

Import sites (verified 2026-11-08 by `grep -rn 'larsartmann/go-health"' --include='*.go' . | grep -v '^./vendor/'`): `internal/webui/health.go:21` and `internal/webui/health_test.go:13`. The dashboard import (health.go:22) is covered by the sibling report.

Symbols consumed — types only, no machinery (verified by `grep -oh 'health\.[A-Za-z]*' internal/webui/*.go | sort | uniq -c`):

| Symbol | Where | Role |
| --- | --- | --- |
| `health.Response` | health.go:88, 114, 250 | wire document the dashboard + JSON probes serve |
| `health.Check` | health.go:163, 232-234 | per-check row |
| `health.Status` + `StatusPass/Warn/Fail` | health.go:88, 167-199, 232-248 | status vocabulary |
| `dashboard.Prober` methods hand-implemented | health.go:94-155 | cache, refresh interval, three probe handlers |

What tq builds on top: `queueProber` (health.go:60-104) lazily re-evaluates a store-backed batch every 5 s (`healthRefreshInterval`, health.go:42, 94-104), registers 4-5 derived checks — database (StatusCounts, health.go:167), workers (heartbeat facts in 10 min, health.go:176-190), queue (`CountStuckRunning`, health.go:192-195), dlq (dead count, health.go:197-200), projection (fold lag/follower, health.go:202-225) — folds the overall status itself (health.go:236-264) and hand-writes the three JSON handlers (health.go:112-155, 301-308). Mounted token-gated in `webui.go:265-283, 340-342, 358-364`.

## 2. Capability inventory

| Capability | Status | Evidence |
| --- | --- | --- |
| Wire schema: `Response`/`Check`/`Status` (pass/warn/fail/off) | 🟢 FULL | health.go:88, 163-256; the vocabulary every probe + the dashboard render |
| Probe construction from check funcs (`New`/`NewWithHealthCheck`/`NewChecks`/`NewWithRecorder`) | 🔴 MISSED | no `health.New*` anywhere; `queueProber` hand-rolled (health.go:60). `*health.Probe` satisfies `dashboard.Prober` natively (cache.go:21,101 + handlers.go:52,79,108 vs dashboard's interface, go-health-dashboard/dashboard.go:28-41) |
| Read accessors (`CachedResponse`/`RefreshInterval`) | 🟡 PARTIAL | implemented by hand to the same contract (health.go:94-108); duplicated cache/latch machinery |
| Three-probe handlers (liveness/readiness/startup) | 🟡 PARTIAL | hand-written (health.go:112-155); contract honored (liveness always 200; readiness 503 on fail; startup latches on first db pass) and test-pinned (health_test.go:110-206) |
| Critical/non-critical classification + startup latch | 🟡 PARTIAL | `finishEval` re-folds statuses itself (health.go:236-264); the library's two-tier classifier (classifier.go:44-105) cannot express per-check warn states — upstream knows: docs/check-severity-override-design.md:13-16 names go-taskqueue as the motivating consumer (PROPOSED, unshipped) |
| Per-check metadata `StatusSince`/`DurationNanos` | 🔴 MISSED | `mkCheck` never sets either (health.go:232-234); types.go:65-75 offer them; dashboard renders "since HH:MM (17m)" + duration only when present (go-health-dashboard/status.go:941-952) |
| Configured-off sentinel (`health.Off`/`StatusOff`) | 🔴 MISSED | projection check silently omitted when unmounted (health.go:202-218, asserted health_test.go:285-291) instead of a visible `off` row with the enable recipe (types.go:78-108) |
| Cadence: background loop / live mode / `WithLiveThrottle` | 🟡 PARTIAL | lazy 5 s TTL cache instead (health.go:94-104); no throttle, but the single-evaluator mutex makes floods moot |
| Batch deadline (`WithTimeout`, default 5 s) | 🟡 PARTIAL | own `healthEvalTimeout` = 5 s (health.go:47, 160-161) — same value, re-declared |
| Panic recovery + context abandonment per check | 🔴 MISSED | `evaluate` has no recover; a panicking store read inside the dashboard pusher goroutine crashes `tq serve` (pusher tick → `CachedResponse` → evaluate; go-health-dashboard/pusher.go:96-111,120 has no recover either). checks.go:94-126 recovers per check and abandons deadline-ignoring checks |
| Shutdown awareness (`ShuttingDown`, `MarkShuttingDown`, grace period) | 🔴 MISSED | webui.go:493-519 drains the HTTP server but `/readyz` stays 200 until the listener closes; lifecycle.go:170-212 flips readiness 503 immediately |
| Method-set guard (`WithAllowedMethods` → 405 + sorted `Allow`) | 🟢 FULL | equivalent via host mux method-qualified patterns (webui.go:275-281 + 337,341: `"GET "+pattern`); Go's ServeMux answers 405 with `Allow` |
| Version identity (`WithVersion` + `/version` endpoint) | 🟡 PARTIAL | version stamped by hand into every response (health.go:88, 252; test health_test.go:161-186); v0.10.0's default-mounted `GET /version` (handlers.go:37-50) not mounted anywhere in tq |
| `WithEvaluationHook` (metrics/alert seam) | 🔴 MISSED | options.go:65-73 fires on every classified evaluation; tq has no hook, so health transitions (store down, wedged fold, DLQ growth) are invisible to the alert channel that exists: internal/bridge/papdashboard (--alert-url, papdashboard.go:139,770) |
| `checks` batteries (Disk/Memory/HTTP/Database) | 🔴 MISSED | checks/checks.go:27,58,93,127; disk-full is a documented tq host failure class (AGENTS.md Known Issues: "host symlink → /nix/store ENOSPCs") and `checks.Disk` on the sqlite home would surface it as a warn row |
| Response hardening (`CoerceValidUTF8` + `Cache-Control: no-cache`) | 🔴 MISSED | tq's `writeHealthJSON` (health.go:301-308) marshals without coercion, without no-cache, and swallows the marshal error — invalid UTF-8 in a store error can yield a truncated/empty body; handlers.go:292-318 does all three (jsonv2 is the one known marshal-failure mode, per its comment) |
| `TotalLatencyMs` batch latency | 🔴 MISSED | never set (health.go:250-256); types.go:168-171 — the dashboard's latency stat card and tone feed on it (go-health-dashboard/pusher.go:138-140, status.go:388-418) and record 0 |
| `aggregate` sub-package | ⚪ N/A | one process, one probe — nothing to merge |
| `federation` sub-package | ⚪ N/A | tq federates nothing remote; the fleet hub is a go-health-dashboard concern, not tq's |
| samber/do integration (`New(injector)`, `AsShutdowner`, `HealthCheck`) | ⚪ N/A | no do container anywhere in tq |
| Programmatic API (`Status`/`Ready`/`AwaitReady`/`Healthz`/`CacheAge`/`Clone`) | ⚪ N/A | tq's only consumers are HTTP (page + probes); nothing in-process queries health |

## 3. Findings

1. **The engine is duplicated; `*health.Probe` would satisfy the dashboard directly.**
   Offer: `health.NewChecks(map[string]health.CheckFunc{...}, health.WithCriticalServices("database"))` (checks.go:44-50) returns a `*Probe` that already satisfies `dashboard.Prober` (accessors.go:64-68 + cache.go:21,101; the dashboard's own doc names it, go-health-dashboard/dashboard.go:23-25).
   We do instead: `queueProber` — own mutex cache (health.go:76-104), own status fold (health.go:236-248), own startup latch (health.go:261-263), own handlers (health.go:112-155), own JSON writer (health.go:301-308).
   Impact: ~120 lines re-implement what the pinned dependency ships, and the duplication is already behind: no `since`/`duration_ns` rows, no `TotalLatencyMs`, no UTF-8 coercion (§2 rows 6, 16, 17).
   Why it is not free: the two-tier classifier cannot express "this check warns" (idle pool, DLQ>0, fold lag) — per-check severities are tq's contract, and upstream's fix is still PROPOSED (docs/check-severity-override-design.md:5, 13-16, which cites go-taskqueue by name). Verdict: keep the hand-rolled prober until that ships, but lift the metadata into it (finding 2) rather than waiting.
2. **Row metadata is one struct away.** Offer: `Check.StatusSince` + `Check.DurationNanos` (types.go:49-76). We do instead: `mkCheck` (health.go:232-234) builds two-field checks, so the dashboard's "how long has it been like this / how slow is it" rows render empty. Impact: an operator seeing `dlq warn` cannot tell whether it is 2 minutes or 2 days old — exactly the triage question the dashboard is for. Example, inside `finishEval`:
   ```go
   // p.since[name] carries the last observed status + its first-seen time.
   if prev, ok := p.since[name]; !ok || prev.st != c.Status {
       p.since[name] = statusSince{st: c.Status, at: now}
   }
   checks[name] = health.Check{Status: c.Status, Error: errMsg, StatusSince: p.since[name].at}
   ```
3. **A panicking store read can kill `tq serve`.** Offer: per-check panic recovery and deadline abandonment (checks.go:94-126). We do instead: `evaluate` (health.go:159-228) runs raw store calls; the dashboard's pusher goroutine (started webui.go:449) calls `CachedResponse` every 5 s with no recover on either side (grep `recover()` in go-health-dashboard/{pusher,dashboard,handlers}.go → none). Impact: one driver panic = process crash, not a red row. Cheapest fix inside the current shape: wrap `p.evaluate(now)`'s body in a `defer func(){ if r := recover(); … }` that degrades the response to a `fail` database row.
4. **Unmounted projection should be `off`, not absent.** Offer: `health.Off(detail)` renders a visible `status:"off"` row carrying the enable recipe and never warns the roll-up (types.go:78-108). We do instead: the check is omitted entirely when no read model is mounted (health.go:202-218). Impact: minor — one row of "run `--read-model` to get fold-lag checks" visibility on every default serve.
5. **Health transitions have no alert path although tq owns an alert bridge.** Offer: `WithEvaluationHook` (options.go:65-73) — synchronous, every evaluation. We do instead: nothing; the PapDashboard bridge (internal/bridge/papdashboard, wired by `--alert-url/--alert-api-key`, papdashboard.go:139) only sees journal facts, so "database check failing since 10 min" never reaches the operator's alert dashboard. The hook (or the dashboard's `WithWebhook`, see the sibling report) is the two-line bridge.
6. **`writeHealthJSON` drops two hardening behaviors.** Offer: `writeResponse` coerces to valid UTF-8 before marshal and sets `Cache-Control: no-cache` (handlers.go:292-318). We do instead: direct `json.MarshalWrite` with the error swallowed (health.go:301-308). Impact: a store error carrying invalid UTF-8 (paths, driver messages) can produce a truncated body; probes can be cached by intermediaries. Reuse the library's semantics: `resp = health.CoerceValidUTF8(resp)` + the header.

## 4. Score + top opportunities

Inputs (from §2): N=21 rows; ⚪ N/A=4 (aggregate, federation, samber/do, programmatic API); A=17. FULL=2 (wire schema, method guard), PARTIAL=6 × 0.5 (read accessors, probe handlers, classification, cadence, batch timeout, version identity), MISSED=9 × 0 (2+6+9=17 ✓). Σ = 2 + 3 = **5.5**. Score = round(100 × 5.5 / 17) = **32 → low** (<50).

Top opportunities, cheapest first: (1) populate `StatusSince`/`TotalLatencyMs` in `finishEval` (finding 2 — pure win, no semantic change); (2) `CoerceValidUTF8` + no-cache in `writeHealthJSON` (finding 6); (3) recover around `evaluate` (finding 3); (4) evaluation hook → PapDashboard (finding 5); (5) revisit full `*health.Probe` adoption when the proposed severity override ships (finding 1).

## 5. Version currency + risk

- **Current**: v0.10.0 is the latest tag; the local checkout is exactly one docs-only commit past it (`git describe --tags` → v0.10.0-1-g7e5d8d1, verified 2026-11-08). No update available.
- **Risk**: the README self-describes as "v0.10.0 alpha" with breaking changes allowed in 0.x minors — v0.9.0 renamed `Check.Since`→`StatusSince` (wire-compatible) and v0.10.0 made `DefaultRoutes()` method-qualified + panicking on unstamped `/version` (CHANGELOG [v0.10.0]). tq touches only types + statuses, so it is insulated from the handler/route churn, but every minor bump deserves a skim of the CHANGELOG "Changed" section. The PROPOSED severity-override doc (2026-11-04) that names go-taskqueue is the release to watch: it is tq's ticket out of the duplicated classifier.
- Vendored: `vendor/github.com/larsartmann/go-health` present; `check-gomod-vendor-sync.sh` gates drift.

## 6. Conclusion

go-health is adopted as a **schema library**: its types define every health document tq serves, and the dashboard integration (sibling report) is built entirely on that shared vocabulary. The probe engine around it is re-implemented — defensibly, because tq's per-check warn semantics predate any upstream way to express them — but the duplication now costs real features the library already ships (`since` stamps, batch latency, UTF-8 hardening, panic containment) and leaves health transitions unalerted while an alert bridge sits unused in the same binary. Utilization is low; the fixes are small, local to `internal/webui/health.go`, and none require abandoning the hand-rolled classifier.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
grep -rn 'larsartmann/go-health"' --include='*.go' . | grep -v '^./vendor/'
# → ./internal/webui/health.go:21, ./internal/webui/health_test.go:13 (imports; dashboard import on health.go:22)
grep -rn 'go-health' --include=go.mod . | grep -v vendor
# → go.mod:110 v0.10.0 (direct), cmd/tq/go.mod:79 (indirect); dashboard on go.mod:111 / cmd/tq/go.mod:80
grep -oh 'health\.[A-Za-z]*' internal/webui/*.go | sort | uniq -c
# → Response/Check/Status/Status{Pass,Warn,Fail} only (the lone `health.go` count is comment text) — no New*/With*/Off/CoerceValidUTF8
cd /home/lars/projects/go-health && git describe --tags        # → v0.10.0-1-g7e5d8d1 (docs-only past tag)
GOCACHE=/tmp/go-build-cache GOEXPERIMENT=jsonv2 GOWORK=off \
  go test ./internal/webui/ -run TestHealth -count=1 -v | grep -c '^--- PASS'   # → 9, 0 FAIL
sed -n '232,234p' internal/webui/health.go    # mkCheck: no StatusSince/DurationNanos
sed -n '301,308p' internal/webui/health.go    # writeHealthJSON: no coercion, no Cache-Control
sed -n '44,50p' /home/lars/projects/go-health/checks.go          # NewChecks offer
sed -n '13,16p' /home/lars/projects/go-health/docs/check-severity-override-design.md  # names go-taskqueue
```

All commands above were run 2026-11-08; the test line reported `ok github.com/larsartmann/go-taskqueue/internal/webui` with 9/9 health tests PASS (captured via `tee /tmp/tq-health-test.log`, `grep -c '^--- PASS'` → 9).
