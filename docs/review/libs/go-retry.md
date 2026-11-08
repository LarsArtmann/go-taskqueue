# go-retry — Library Utilization Review

**Verdict:** ADOPTED — low (4/9 capability rows leveraged) — the core `Do` loop is used at every non-exempt retry site in the repo, but the ergonomic surface (`DoWithValue`, options tail) and the exhaustion/error-model features are untouched.

**Pinned:** v0.9.0 — direct requires: `internal/executor/go.mod:7`, `internal/worker/go.mod:48`, `internal/retryx/go.mod:5`; `// indirect` in 8 more modules. Verified 2026-11-08 by `grep -rn 'go-retry' --include=go.mod . | grep -v vendor`.

**Latest:** v0.9.0 (local checkout /home/lars/projects/go-retry, checked 2026-11-08 by `git -C … describe --tags` → `v0.9.0-4-g047d55a`; Unreleased empty).

**Prior coverage:** none (first libs-review report).

**Reviewed:** 2026-11-08 by reading /home/lars/projects/go-retry/{README.md,FEATURES.md,CHANGELOG.md,retry.go,config.go,options.go} + repo greps (`grep -rn --include='*.go' 'retry\.\(Do\|Config\)' . | grep -v '^./vendor/'` → 3 call sites; zero hits for `DoWithValue|ErrExhausted|WithOnRetry|FromPolicy|Backoff|ComputeDelay|JitterNone`).

## 1. Usage inventory

Convention (AGENTS.md:287-288): "Retry loops use go-retry (supervisor loops and persisted domain backoff exempt)." All three call sites — there are no other in-process retry loops outside the exemptions:

| Site | Config | Predicate | Hooks |
| --- | --- | --- | --- |
| internal/executor/agent.go:1123 `execWithTransientRetry` | 3 attempts, 50ms→100ms, ×2.0 | `errors.Is(err, syscall.ETXTBSY)` (kernel ETXTBSY class, AGENTS.md §Known Issues) | none |
| internal/worker/worker.go:399 `persistOutcome` | 3 attempts, 50ms→400ms, ×2.0 | `isTransientStoreBusy` (string sniff, worker.go:442-450) | `OnRetry` warn log w/ task, attempt, delay (worker.go:405-407) |
| internal/retryx/rename.go:24 `RenameTransient` | 3 attempts, 50ms→100ms, ×2.0 | default (`errorfamily.IsRetryable`, fail-open Transient — right for dynamic Windows sharing violations) | none |

All three pass struct literals carrying `//nolint:exhaustruct // optional hooks unset`. Exempt hand-rolled backoff (correctly outside the convention): the worker requeue ladder / `cfg.Backoff` (worker.go:63, 223-240 — persisted domain backoff), preflight/rate-limit jittered delays (worker.go:174-178, 238), and `internal/dlqbackoff` (no go-retry import — verified 2026-11-08 by `grep -rn 'go-retry' internal/dlqbackoff/` → no output).

## 2. Capability inventory

| Capability | Status | Evidence |
| --- | --- | --- |
| `Do` loop + `Config` | 🟢 FULL | all 3 non-exempt sites (§1); AGENTS.md:287 convention holds |
| Custom `IsRetryable` predicate | 🟢 FULL | agent.go:1126 (ETXTBSY sentinel), worker.go:404 (store-busy) |
| Default family predicate (`errorfamily.IsRetryable`) | 🟡 PARTIAL | only retryx rides it; the other two sites override |
| `DoWithValue[T]` | 🔴 MISSED | the one value-returning retry in the repo hand-rolls the closure dance (agent.go:1121-1137: `var out T; out, attemptErr = run()`) |
| Per-call options tail (`WithOnRetry`/`WithJitter`/…) | 🔴 MISSED | zero `retry.With*` uses; three `//nolint:exhaustruct` markers exist because of it |
| Jitter selection / deterministic RNG (`JitterNone`, `WithRandomSource`) | 🟡 PARTIAL | additive jitter rides the default everywhere; never deliberately chosen, no seeded-RNG delay tests |
| `OnRetry` hook | 🟢 FULL | worker.go:405-407 (task, attempt, delay, err) |
| `OnExhausted` hook | 🔴 MISSED | exhaustion surfaces only as callers' generic error log (worker.go:428-431); no "gave up after N attempts" signal anywhere |
| Sentinel discrimination (`ErrExhausted`/`ErrCanceled`/`ErrDeadlineExceeded`) | 🔴 MISSED | zero `errors.Is(err, retry.Err*)` sites (grep → no output); callers treat exhaustion opaquely |
| `DelayFunc` (server-provided delays) | ⚪ N/A | provider 429 reset windows are persisted requeue parking (rateLimitWait → requeue evidence), which the AGENTS.md:287 exemption covers — not an in-process loop |
| Last-attempt code promotion (v0.9.0) | ⚪ N/A | attempt errors at all three sites are uncoded raw errors (`syscall.ETXTBSY`, sql errors), so `errorfamily.Code` promotion is inert by construction |
| `Backoff`/`ComputeDelay` preview helpers | ⚪ N/A | no delay-preview/telemetry surface; the persisted ladder's math is documented where it lives (worker.go:223-240) |
| `FromPolicy` interop | ⚪ N/A | no `errorfamily.RetryPolicy` producer exists in this repo |
| `Config.Validate` as caller API | ⚪ N/A | all three configs are valid literals; `Do` validates internally |

## 3. Findings

**F1 — Convention compliance is complete and verifiable.** lib: `Do`/`DefaultConfig` (checkout retry.go, config.go). repo: the three sites in §1 are exactly the non-exempt retry loops; supervisor idle/heartbeat loops and the persisted requeue ladder stay hand-rolled per AGENTS.md:287. Nothing to widen — any new in-process loop should copy the retryx shape.

**F2 — `execWithTransientRetry` re-implements `DoWithValue`.** lib: `DoWithValue[T]` "removes the closure-plus-variable dance — the successful attempt's result comes straight back" (checkout retry.go `DoWithValue`; README "Retries that produce a value"; pinned by `ExampleDoWithValue`). repo: internal/executor/agent.go:1121-1137 declares `var out T`, mutates it inside the attempt closure, returns `out, err` — and `out` is garbage whenever the final attempt failed (zero value on the error path is exactly `DoWithValue`'s contract). Impact: 8 lines of boilerplate on a generic shared seam; identical semantics.

```go
// today (agent.go:1121): var out T; retry.Do(..., func(...) error { out, attemptErr = run(); return attemptErr }); return out, err
func execWithTransientRetry[T any](run func() (T, error)) (T, error) {
	return retry.DoWithValue(context.Background(), retry.Config{ //nolint:exhaustruct // optional hooks unset
		MaxAttempts:  3,
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		IsRetryable:  func(err error) bool { return errors.Is(err, syscall.ETXTBSY) },
	}, func(_ context.Context, _ int) (T, error) { return run() })
}
```

**F3 — Struct literals + exhaustruct nolint where the options tail exists.** lib: `WithIsRetryable/WithDelayFunc/WithOnRetry/WithExhausted/WithJitter/WithRandomSource` override per call, "the struct you passed is never modified" (checkout options.go; FEATURES "Per-call options"). repo: all three sites use `Config{...}` literals with `//nolint:exhaustruct` (agent.go:1123, worker.go:400, rename.go:24). Impact: cosmetic — the nolint markers exist only because unset optional fields are indistinguishable from forgotten ones. Migrating `persistOutcome`'s hook to `retry.WithOnRetry(...)` would delete one suppression class.

**F4 — Exhaustion is invisible to evidence.** lib: `OnExhausted(attempts, err)` fires once with the exact last error (checkout config.go; FEATURES "Exhaustion callback"), and `retry.ErrExhausted` is `errors.Is`-able while still unwrapping to the last attempt error (README "Errors"). repo: no caller checks `ErrExhausted` (grep → no output); `persistOutcome`'s caller logs the returned error generically (worker.go:428-431 `p.log.Error("done-gate complete failed", ...)`). Impact: a 3×-exhausted SQLITE_BUSY write is indistinguishable in logs from a first-attempt permanent failure; an `OnExhausted` hook (or an `errors.Is(err, retry.ErrExhausted)` branch) would mark "retried 3×, reclaim backstop now owns this task" — which AGENTS.md says is the designed backstop.

**F5 — retryx correctly leans on the fail-open default.** lib: unknown errors classify Transient → retryable (checkout classify.go default step; README "fail-open for retry"). repo: rename.go:24-37 retries any `os.Rename` error 3× in 180ms max — right for dynamic Windows sharing violations (package doc, rename.go:1-8), with `os.IsNotExist` mapped to success inside the attempt. Deliberate and correct; noted so nobody "fixes" it into a narrow predicate.

**F6 — No deterministic-jitter tests exist.** lib: `WithRandomSource` + `JitterNone` make delay sequences reproducible (checkout options.go; FEATURES "Deterministic RNG"). repo: no `retry.With*` anywhere, and no test pins the three sites' delay behavior (they sleep real 50-100ms). Impact: minor test-speed/flake exposure only; the loop logic is what the tests pin.

## 4. Score + top opportunities

Inputs: N=14 rows, N/A=5, A=9. FULL=3, PARTIAL=2 (1.0), MISSED=4, MISUSED=0 → Σ=4.0.
Score = round(100 × 4/9) = **44 — low**.

Top opportunities: (1) F2 — swap `execWithTransientRetry` to `DoWithValue` (pure refactor on a shared seam, deletes the closure dance); (2) F4 — `OnExhausted` or an `ErrExhausted` branch in `persistOutcome`'s caller so exhausted writes are labeled before the reclaim backstop takes over; (3) F3 — options tail at the two hook-bearing sites to drop `//nolint:exhaustruct`.

## 5. Version currency + risk

Pinned v0.9.0 == latest tag (checkout 4 commits past tag; Unreleased empty) — no action. v0.9.0 (2026-10-27, CHANGELOG) added last-attempt code promotion (N/A here, see table) and restored the `go 1.26`/error-family v0.11.0 pins after an auto-committed bump incident; tq's MVS resolves error-family v0.12.0 regardless, which is the intended graph. Risk: `go-retry` declares `go 1.27.3` (checkout go.mod:3), matching tq's floor — pin-safe. The 100%-coverage, nightly-fuzz/stress posture (README "Development") makes the pinned surface low-risk to lean on harder.

## 6. Conclusion

The convention does what AGENTS.md:287 says: every in-process transient retry in the repo is a `retry.Do`, and the exemptions (supervisor loops, persisted ladder, dlqbackoff) are honored rather than abused. What's left on the table is polish, not adoption: `DoWithValue` at the executor seam, exhaustion visibility in the worker, and options instead of exhaustruct suppressions. The library is at its latest tag; deepening use carries no version risk.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
grep -rl --include='*.go' 'larsartmann/go-retry' . | grep -v '^./vendor/'   # → agent.go, worker.go, rename.go
grep -n 'retry\.' internal/executor/agent.go internal/worker/worker.go internal/retryx/rename.go
# → agent.go:1123, worker.go:399, rename.go:24 (the only retry.* call sites)
grep -rn 'retry.DoWithValue\|retry.Err\|retry.With\|retry.FromPolicy\|retry.Backoff\|retry.ComputeDelay\|retry.JitterNone' --include='*.go' . | grep -v '^./vendor/'
# → no output (unused surface, §2 MISSED rows)
grep -n 'go-retry' internal/executor/go.mod internal/worker/go.mod internal/retryx/go.mod   # → v0.9.0 direct
grep -rn 'go-retry' internal/dlqbackoff/    # → no output (exempt module, §1)
sed -n '287,288p' AGENTS.md                 # the convention line
sed -n '1121,1137p' internal/executor/agent.go  # closure dance F2 replaces
sed -n '398,412p;442,450p' internal/worker/worker.go  # persistOutcome + string-sniff predicate
sed -n '63p;174,178p;238p' internal/worker/worker.go  # exempt hand-rolled ladder/jitter
git -C /home/lars/projects/go-retry describe --tags  # → v0.9.0-4-g047d55a
```
