# go-health — Library Utilization Review

**Verdict:** ADOPTED — low (7/17 applicable capability rows leveraged; was 5.5/17 before the 2026-11-09 landings): the wire schema is the backbone of `tq serve`'s whole health surface and the metadata/recovery rows just landed, but the probe engine (construction, classifier, handlers, hardening) is still hand-rolled next to it.

**2026-11-11 refresh (re-score pending resolved):** three capability rows moved on the 2026-11-09 landing — row 6 MISSED→PARTIAL (`StatusSince` now stamped per check in `finishEval`, internal/webui/health.go:305-312; `DurationNanos` still unset), row 10 MISSED→PARTIAL (`evaluateSafely` recover-wrap now contains a panicking evaluation as a fail `prober` row, health.go:124-142 — exactly finding 3's fix; per-check context abandonment still absent), row 17 MISSED→FULL (`TotalLatencyMs` set from batchStart, health.go:323). Score re-derived: 32 → **41** (inputs in §4). Also landed via the SIBLING surface (see go-health-dashboard.md): the alert gap of finding 5 is closed by the dashboard's `WithWebhook` → PapDashboard relay (health.go:408, webui.go:218, cmd/tq/main.go:648) — row 15's capability itself stays MISSED but its operator harm is gone. All line cites re-anchored against HEAD 2026-11-11; checkout re-checked (v0.10.0 = latest tag).

**Pinned:** v0.10.0 — root `go.mod:110` (direct); `cmd/tq/go.mod:79` (indirect). Verified 2026-11-08 by `grep -rn 'go-health' --include=go.mod . | grep -v vendor`.

**Latest:** v0.10.0 (local checkout /home/lars/projects/go-health, checked 2026-11-08; HEAD = v0.10.0-1-g7e5d8d1, one docs-only commit past the tag).

**Prior coverage:** none (docs/review/libs/ has only httputil.md; AGENTS.md:399 covers the bundle decision, not this library alone).

**Reviewed:** 2026-11-08 by reading /home/lars/projects/go-health/{README.md,CHANGELOG.md,doc.go,types.go,checks.go,handlers.go,options.go,lifecycle.go,cache.go,accessors.go,checks/checks.go,docs/check-severity-override-design.md} plus /home/lars/projects/go-taskqueue/internal/webui/{health.go,webui.go,health_test.go}; greps and tests listed in §How to verify.

## 1. Usage inventory

Import sites (re-verified 2026-11-11 by `grep -rn 'larsartmann/go-health"' --include='*.go' . | grep -v '^./vendor/'`): `internal/webui/health.go:24` and `internal/webui/health_test.go:14`. The dashboard import (health.go:25) is covered by the sibling report.

Symbols consumed — types only, no machinery (verified by `grep -oh 'health\.[A-Za-z]*' internal/webui/*.go | sort | uniq -c`):

| Symbol | Where | Role |
| --- | --- | --- |
| `health.Response` | health.go:105, 149, 317 | wire document the dashboard + JSON probes serve |
| `health.Check` | health.go:288-290 + evaluate | per-check row |
| `health.Status` + `StatusPass/Warn/Fail` | health.go:105, evaluate/finishEval ranges | status vocabulary |
| `dashboard.Prober` methods hand-implemented | health.go:109-192 | cache, refresh interval, three probe handlers |

What tq builds on top: `queueProber` (health.go:60-121) lazily re-evaluates a store-backed batch every 5 s (`healthRefreshInterval`, health.go:48, 109-121), registers 4-5 derived checks — database (StatusCounts, health.go:204), workers (heartbeat facts in 10 min, health.go:215-227), queue (`CountStuckRunning`, health.go:229-233), dlq (dead count, health.go:235-239), projection (fold lag/follower, health.go:242-260) — folds the overall status itself (`finishEval`, health.go:292-334) and hand-writes the three JSON handlers (health.go:145-192, 370-378). Mounted token-gated via `newQueueProber` + the dashboard in `webui.go:209-237` and the healthBindings table (`webui.go:312-328`; doc comment 363-367).

## 2. Capability inventory

| Capability | Status | Evidence |
| --- | --- | --- |
| Wire schema: `Response`/`Check`/`Status` (pass/warn/fail/off) | 🟢 FULL | health.go:105, 204-334; the vocabulary every probe + the dashboard render |
| Probe construction from check funcs (`New`/`NewWithHealthCheck`/`NewChecks`/`NewWithRecorder`) | 🔴 MISSED | no `health.New*` anywhere; `queueProber` hand-rolled (health.go:60). `*health.Probe` satisfies `dashboard.Prober` natively (cache.go:21,101 + handlers.go:52,79,108 vs dashboard's interface, go-health-dashboard/dashboard.go:28-41) |
| Read accessors (`CachedResponse`/`RefreshInterval`) | 🟡 PARTIAL | implemented by hand to the same contract (health.go:94-108); duplicated cache/latch machinery |
| Three-probe handlers (liveness/readiness/startup) | 🟡 PARTIAL | hand-written (health.go:145-192); contract honored (liveness always 200; readiness 503 on fail; startup latches on first db pass) and test-pinned (health_test.go:110-206) |
| Critical/non-critical classification + startup latch | 🟡 PARTIAL | `finishEval` re-folds statuses itself (health.go:292-334); the library's two-tier classifier (classifier.go:44-105) cannot express per-check warn states — upstream knows: docs/check-severity-override-design.md:13-16 names go-taskqueue as the motivating consumer (PROPOSED, unshipped) |
| Per-check metadata `StatusSince`/`DurationNanos` | 🟡 PARTIAL | `StatusSince` stamped per check in `finishEval` — carried across stable statuses, reset on transitions (health.go:305-312, 2026-11-09 landing); `DurationNanos` still never set (mkCheck, health.go:288-290); dashboard renders "since HH:MM (17m)" when present (go-health-dashboard/status.go:941-952) |
| Configured-off sentinel (`health.Off`/`StatusOff`) | 🔴 MISSED | projection check silently omitted when unmounted (health.go:242-260, asserted health_test.go:285-291) instead of a visible `off` row with the enable recipe (types.go:78-108) |
| Cadence: background loop / live mode / `WithLiveThrottle` | 🟡 PARTIAL | lazy 5 s TTL cache instead (health.go:109-121); no throttle, but the single-evaluator mutex makes floods moot |
| Batch deadline (`WithTimeout`, default 5 s) | 🟡 PARTIAL | own `healthEvalTimeout` = 5 s (health.go:53, 194-196) — same value, re-declared |
| Panic recovery + context abandonment per check | 🟡 PARTIAL | `evaluateSafely` (health.go:124-142) recovers a panicking batch into a fail `prober` row + ERROR log with stack (2026-11-09 landing — the crash class of finding 3 is contained); per-check deadline abandonment (checks.go:94-126) still not adopted |
| Shutdown awareness (`ShuttingDown`, `MarkShuttingDown`, grace period) | 🔴 MISSED | webui.go:575-585 drains the HTTP server but `/readyz` stays 200 until the listener closes; lifecycle.go:170-212 flips readiness 503 immediately |
| Method-set guard (`WithAllowedMethods` → 405 + sorted `Allow`) | 🟢 FULL | equivalent via host mux method-qualified patterns (webui.go:312-328 healthBindings: `"GET "+pattern`); Go's ServeMux answers 405 with `Allow` |
| Version identity (`WithVersion` + `/version` endpoint) | 🟡 PARTIAL | version stamped by hand into every response (health.go:105, 319; test health_test.go:161-186); v0.10.0's default-mounted `GET /version` (handlers.go:37-50) not mounted anywhere in tq |
| `WithEvaluationHook` (metrics/alert seam) | 🔴 MISSED | options.go:65-73 fires on every classified evaluation; tq has no go-health hook. NOTE: the operator harm this row described is CLOSED via the sibling surface — the dashboard's `WithWebhook` now relays transitions to PapDashboard (health.go:408, webui.go:218, `--alert-url` cmd/tq/main.go:648; see go-health-dashboard.md) — but the capability itself remains unused |
| `checks` batteries (Disk/Memory/HTTP/Database) | 🔴 MISSED | checks/checks.go:27,58,93,127; disk-full is a documented tq host failure class (AGENTS.md Known Issues: "host symlink → /nix/store ENOSPCs") and `checks.Disk` on the sqlite home would surface it as a warn row |
| Response hardening (`CoerceValidUTF8` + `Cache-Control: no-cache`) | 🔴 MISSED | tq's `writeHealthJSON` (health.go:370-378) marshals without coercion, without no-cache, and swallows the marshal error — invalid UTF-8 in a store error can yield a truncated/empty body; handlers.go:292-318 does all three (jsonv2 is the one known marshal-failure mode, per its comment) |
| `TotalLatencyMs` batch latency | 🟢 FULL | set per batch in `finishEval` (health.go:323, 2026-11-09 landing) — the dashboard's latency stat card and tone now feed on probe-observed truth |
| `aggregate` sub-package | ⚪ N/A | one process, one probe — nothing to merge |
| `federation` sub-package | ⚪ N/A | tq federates nothing remote; the fleet hub is a go-health-dashboard concern, not tq's |
| samber/do integration (`New(injector)`, `AsShutdowner`, `HealthCheck`) | ⚪ N/A | no do container anywhere in tq |
| Programmatic API (`Status`/`Ready`/`AwaitReady`/`Healthz`/`CacheAge`/`Clone`) | ⚪ N/A | tq's only consumers are HTTP (page + probes); nothing in-process queries health |

## 3. Findings

1. **The engine is duplicated; `*health.Probe` would satisfy the dashboard directly.**
   Offer: `health.NewChecks(map[string]health.CheckFunc{...}, health.WithCriticalServices("database"))` (checks.go:44-50) returns a `*Probe` that already satisfies `dashboard.Prober` (accessors.go:64-68 + cache.go:21,101; the dashboard's own doc names it, go-health-dashboard/dashboard.go:23-25).
   We do instead: `queueProber` — own mutex cache (health.go:60-121), own status fold (health.go:292-334), own startup latch (health.go:330-333), own handlers (health.go:145-192), own JSON writer (health.go:370-378).
   Impact: ~120 lines re-implement what the pinned dependency ships, and the duplication is already behind: no `since`/`duration_ns` rows, no `TotalLatencyMs`, no UTF-8 coercion (§2 rows 6, 16, 17).
   Why it is not free: the two-tier classifier cannot express "this check warns" (idle pool, DLQ>0, fold lag) — per-check severities are tq's contract, and upstream's fix is still PROPOSED (docs/check-severity-override-design.md:5, 13-16, which cites go-taskqueue by name). Verdict: keep the hand-rolled prober until that ships, but lift the metadata into it (finding 2) rather than waiting.
2. **~~Row metadata is one struct away~~ RESOLVED (2026-11-09, half).** Offer: `Check.StatusSince` + `Check.DurationNanos` (types.go:49-76). Landed: `finishEval` now carries `StatusSince` across stable statuses and resets it on transitions — "probe-observed truth, not render time" (health.go:304-312). Still open: `DurationNanos` (mkCheck, health.go:288-290, builds status+error only). Impact delivered: the dashboard's "how long has it been like this" column renders per-check truth; the duration column stays empty.
   ```go
   // health.go:305-312 — current code
   if prev, ok := p.resp.Checks[name]; ok && prev.Status == c.Status && !prev.StatusSince.IsZero() {
       c.StatusSince = prev.StatusSince
   } else {
       c.StatusSince = now
   }
   ```
3. **~~A panicking store read can kill `tq serve`~~ RESOLVED (2026-11-09).** Offer: per-check panic recovery (checks.go:94-126). Landed: `evaluateSafely` (health.go:124-142) wraps the whole evaluation — a panic logs ERROR with stack and degrades to a fail `prober` row instead of crashing the SSE pusher goroutine. Remaining nicety (not the crash class): per-check deadline abandonment so one slow check cannot stall the whole batch.
4. **Unmounted projection should be `off`, not absent.** Offer: `health.Off(detail)` renders a visible `status:"off"` row carrying the enable recipe and never warns the roll-up (types.go:78-108). We do instead: the check is omitted entirely when no read model is mounted (health.go:242-260). Impact: minor — one row of "run `--read-model` to get fold-lag checks" visibility on every default serve.
5. **~~Health transitions have no alert path~~ RESOLVED VIA THE SIBLING SURFACE (2026-11-09).** Offer: `WithEvaluationHook` (options.go:65-73). What landed instead: the go-health-dashboard's `WithWebhook` relaying to PapDashboard (health.go:408; `--alert-url`, cmd/tq/main.go:648) — the operator outcome ("database check failing reaches the alert dashboard") is delivered through the sibling report's capability, not this one. The go-health hook remains unused; adopting it would add per-evaluation sync signaling the webhook path does not need today.
6. **`writeHealthJSON` drops two hardening behaviors.** Offer: `writeResponse` coerces to valid UTF-8 before marshal and sets `Cache-Control: no-cache` (handlers.go:292-318). We do instead: direct `json.MarshalWrite` with the error swallowed (health.go:370-378). Impact: a store error carrying invalid UTF-8 (paths, driver messages) can produce a truncated body; probes can be cached by intermediaries. Reuse the library's semantics: `resp = health.CoerceValidUTF8(resp)` + the header.

## 4. Score + top opportunities

Inputs (re-derived 2026-11-11): N=21 rows; ⚪ N/A=4 (aggregate, federation, samber/do, programmatic API); A=17. FULL=3 (wire schema, method guard, TotalLatencyMs), PARTIAL=8 × 0.5 (read accessors, probe handlers, classification, cadence, batch timeout, version identity, per-check metadata, panic recovery), MISSED=6 × 0 (3+8+6=17 ✓). Σ = 3 + 4 = **7.0**. Score = round(100 × 7.0 / 17) = **41 → low** (<50; was 32 before the landings).

Top opportunities, cheapest first: (1) `CoerceValidUTF8` + `Cache-Control: no-cache` in `writeHealthJSON` (finding 6 — still the only unchecked hardening); (2) `DurationNanos` in `mkCheck` (finishes row 6); (3) per-check deadline abandonment (finishes row 10); (4) `health.Off` sentinel for the unmounted projection (finding 4); (5) revisit full `*health.Probe` adoption when the proposed severity override ships (finding 1). (Former #1 StatusSince/TotalLatencyMs and #3 recover landed 2026-11-09; #4 alert gap closed via the dashboard webhook.)

## 5. Version currency + risk

- **Current**: v0.10.0 is the latest tag; the local checkout is exactly one docs-only commit past it (`git describe --tags` → v0.10.0-1-g7e5d8d1, verified 2026-11-08). No update available.
- **Risk**: the README self-describes as "v0.10.0 alpha" with breaking changes allowed in 0.x minors — v0.9.0 renamed `Check.Since`→`StatusSince` (wire-compatible) and v0.10.0 made `DefaultRoutes()` method-qualified + panicking on unstamped `/version` (CHANGELOG [v0.10.0]). tq touches only types + statuses, so it is insulated from the handler/route churn, but every minor bump deserves a skim of the CHANGELOG "Changed" section. The PROPOSED severity-override doc (2026-11-04) that names go-taskqueue is the release to watch: it is tq's ticket out of the duplicated classifier.
- Vendored: `vendor/github.com/larsartmann/go-health` present; `check-gomod-vendor-sync.sh` gates drift.

## 6. Conclusion

go-health is adopted as a **schema library**: its types define every health document tq serves, and the dashboard integration (sibling report) is built entirely on that shared vocabulary. The 2026-11-09 landing closed the metadata and crash-containment gaps (score 32 → 41) — the response now carries since-stamps and batch latency, and a panicking evaluation degrades to a red row instead of a dead process — while the alert gap closed through the sibling webhook. What remains duplicated is the probe ENGINE itself (construction, classifier, handlers, hardening) — defensibly, because tq's per-check warn semantics predate any upstream way to express them — but it still costs the UTF-8/no-cache hardening and the off-sentinel visibility. Utilization is low-to-moderate in effect; the remaining fixes stay small and local to `internal/webui/health.go`.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
grep -rn 'larsartmann/go-health"' --include='*.go' . | grep -v '^./vendor/'
# → ./internal/webui/health.go:21, ./internal/webui/health_test.go:13 (imports; dashboard import on health.go:22)
grep -rn 'go-health' --include=go.mod . | grep -v vendor
# → go.mod:110 v0.10.0 (direct), cmd/tq/go.mod:79 (indirect); dashboard on go.mod:111 / cmd/tq/go.mod:80
grep -oh 'health\.[A-Za-z]*' internal/webui/*.go | sort | uniq -c
# → Response/Check/Status/Status{Pass,Warn,Fail} only (the lone `health.go` count is comment text) — no New*/With*/Off
grep -n 'StatusSince\|TotalLatencyMs\|evaluateSafely' internal/webui/health.go
# → 127 (evaluateSafely), 305-312 (StatusSince), 323 (TotalLatencyMs) — the 2026-11-09 landings
cd /home/lars/projects/go-health && git describe --tags        # → v0.10.0-1-g7e5d8d1 (docs-only past tag)
GOCACHE=/tmp/go-build-cache GOEXPERIMENT=jsonv2 GOWORK=off \
  go test ./internal/webui/ -run TestHealth -count=1 -v | grep -c '^--- PASS'   # → health tests, 0 FAIL
sed -n '288,290p' internal/webui/health.go    # mkCheck: no DurationNanos (row 6 half-open)
sed -n '124,142p' internal/webui/health.go    # evaluateSafely recover (row 10 landing)
sed -n '370,378p' internal/webui/health.go    # writeHealthJSON: no coercion, no Cache-Control (finding 6 open)
sed -n '44,50p' /home/lars/projects/go-health/checks.go          # NewChecks offer
sed -n '13,16p' /home/lars/projects/go-health/docs/check-severity-override-design.md  # names go-taskqueue
```
