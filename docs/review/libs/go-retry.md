# go-retry — Library Utilization Review

**Verdict:** ADOPTED — high (7/9 capability rows leveraged; was 4/9 before the 2026-11-09/10 landings) — the core `Do` loop is used at every non-exempt retry site, `DoWithValue` now serves the executor seam, and exhaustion is loud and sentinel-discriminated; only the per-call options tail stays untouched.

**2026-11-11 refresh:** two top opportunities LANDED since 2026-11-08 — F2 (`execWithTransientRetry` now rides `retry.DoWithValue[T]`, internal/executor/agent.go:1129) and F4 (exhaustion visibility: `OnExhausted` hook at internal/worker/worker.go:413-416 with the loud backstop label + `TestPersistOutcomeExhaustionIsLabeled` pinning `errors.Is(err, retry.ErrExhausted)`, internal/worker/worker_test.go:1786-1806). Score re-derived: 44 → 78 (inputs below). §1/§2 cites re-verified against HEAD 2026-11-11 by grep; go-retry checkout now `v0.9.0-5-ga3ed997`, still pinned v0.9.0 = latest tag.

**Pinned:** v0.9.0 — direct requires: `internal/executor/go.mod:7`, `internal/worker/go.mod:48`, `internal/retryx/go.mod:5`; `// indirect` in 7 more modules. Verified 2026-11-08 by `grep -rn 'go-retry' --include=go.mod . | grep -v vendor`.

**Latest:** v0.9.0 (local checkout /home/lars/projects/go-retry, re-checked 2026-11-11 by `git -C … describe --tags` → `v0.9.0-5-ga3ed997`; Unreleased empty).

**Prior coverage:** none (first libs-review report).

**Reviewed:** 2026-11-08 by reading /home/lars/projects/go-retry/{README.md,FEATURES.md,CHANGELOG.md,retry.go,config.go,options.go} + repo greps (`grep -rn --include='*.go' 'retry\.\(Do\|Config\)' . | grep -v '^./vendor/'` → 3 call sites; zero hits for `DoWithValue|ErrExhausted|WithOnRetry|FromPolicy|Backoff|ComputeDelay|JitterNone`).

## 1. Usage inventory

Convention (AGENTS.md §Conventions, retry-loop bullet): "Retry loops use go-retry (supervisor loops and persisted domain backoff exempt)." All three call sites — there are no other in-process retry loops outside the exemptions:

| Site | Config | Predicate | Hooks |
| --- | --- | --- | --- |
| internal/executor/agent.go:1129 `execWithTransientRetry` | 3 attempts, 50ms→100ms, ×2.0 | `errors.Is(err, syscall.ETXTBSY)` (kernel ETXTBSY class, AGENTS.md §Known Issues) | none |
| internal/worker/worker.go:398 `persistOutcome` | 3 attempts, 50ms→400ms, ×2.0 | `isTransientStoreBusy` (string sniff, worker.go:442-450) | `OnRetry` warn log w/ task, attempt, delay (worker.go:405-408); `OnExhausted` ERROR log naming the lease-reclaim backstop (worker.go:413-416) |
| internal/retryx/rename.go:24 `RenameTransient` | 3 attempts, 50ms→100ms, ×2.0 | default (`errorfamily.IsRetryable`, fail-open Transient — right for dynamic Windows sharing violations) | none |

All three pass struct literals carrying `//nolint:exhaustruct // optional hooks unset`. Exempt hand-rolled backoff (correctly outside the convention): the worker requeue ladder / `cfg.Backoff` (worker.go:63, 223-240 — persisted domain backoff), preflight/rate-limit jittered delays (worker.go:174-178, 238), and `internal/dlqbackoff` (no go-retry import — verified 2026-11-08 by `grep -rn 'go-retry' internal/dlqbackoff/` → no output).

## 2. Capability inventory

| Capability | Status | Evidence |
| --- | --- | --- |
| `Do` loop + `Config` | 🟢 FULL | all 3 non-exempt sites (§1); AGENTS.md retry-loop convention holds |
| Custom `IsRetryable` predicate | 🟢 FULL | agent.go:1134 (ETXTBSY sentinel), worker.go:404 (store-busy) |
| Default family predicate (`errorfamily.IsRetryable`) | 🟡 PARTIAL | only retryx rides it; the other two sites override |
| `DoWithValue[T]` | 🟢 FULL | agent.go:1129 — F2 landed 2026-11-09/10; the closure dance is gone |
| Per-call options tail (`WithOnRetry`/`WithJitter`/…) | 🔴 MISSED | zero `retry.With*` uses; three `//nolint:exhaustruct` markers remain (agent.go:1129, worker.go:398, rename.go:24) |
| Jitter selection / deterministic RNG (`JitterNone`, `WithRandomSource`) | 🟡 PARTIAL | additive jitter rides the default everywhere; never deliberately chosen, no seeded-RNG delay tests |
| `OnRetry` hook | 🟢 FULL | worker.go:405-408 (task, attempt, delay, err) |
| `OnExhausted` hook | 🟢 FULL | worker.go:413-416 — F4 landed; ERROR label names the lease-reclaim backstop, pinned by worker_test.go:1786-1806 |
| Sentinel discrimination (`ErrExhausted`/`ErrCanceled`/`ErrDeadlineExceeded`) | 🟢 FULL | `errors.Is(err, retry.ErrExhausted)` pinned at worker_test.go:1806 (exhaustion contract test); ErrCanceled/ErrDeadlineExceeded remain unused — but the exhaustion sentinel, the one this repo's loops can emit, is discriminated |
| `DelayFunc` (server-provided delays) | ⚪ N/A | provider 429 reset windows are persisted requeue parking (rateLimitWait → requeue evidence), which the AGENTS.md retry-loop exemption covers — not an in-process loop |
| Last-attempt code promotion (v0.9.0) | ⚪ N/A | attempt errors at all three sites are uncoded raw errors (`syscall.ETXTBSY`, sql errors), so `errorfamily.Code` promotion is inert by construction |
| `Backoff`/`ComputeDelay` preview helpers | ⚪ N/A | no delay-preview/telemetry surface; the persisted ladder's math is documented where it lives (worker.go:223-240) |
| `FromPolicy` interop | ⚪ N/A | no `errorfamily.RetryPolicy` producer exists in this repo |
| `Config.Validate` as caller API | ⚪ N/A | all three configs are valid literals; `Do` validates internally |

## 3. Findings

**F1 — Convention compliance is complete and verifiable.** lib: `Do`/`DefaultConfig` (checkout retry.go, config.go). repo: the three sites in §1 are exactly the non-exempt retry loops; supervisor idle/heartbeat loops and the persisted requeue ladder stay hand-rolled per the AGENTS.md retry-loop exemption. Nothing to widen — any new in-process loop should copy the retryx shape.

**F2 — ~~`execWithTransientRetry` re-implements `DoWithValue`~~ RESOLVED (landed 2026-11-09/10).** lib: `DoWithValue[T]` "removes the closure-plus-variable dance — the successful attempt's result comes straight back" (checkout retry.go `DoWithValue`; README "Retries that produce a value"; pinned by `ExampleDoWithValue`). repo: internal/executor/agent.go:1128-1137 is now exactly the recommended shape — `retry.DoWithValue[T](…)`, no `var out T`, no closure mutation:

```go
// agent.go:1128 — current code, the F2 recommendation as-written
func execWithTransientRetry[T any](run func() (T, error)) (T, error) {
	return retry.DoWithValue[T](context.Background(), retry.Config{ //nolint:exhaustruct // optional hooks unset
		MaxAttempts:  3,
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		IsRetryable:  func(err error) bool { return errors.Is(err, syscall.ETXTBSY) },
	}, func(_ context.Context, _ int) (T, error) { return run() })
}
```

**F3 — Struct literals + exhaustruct nolint where the options tail exists.** lib: `WithIsRetryable/WithDelayFunc/WithOnRetry/WithExhausted/WithJitter/WithRandomSource` override per call, "the struct you passed is never modified" (checkout options.go; FEATURES "Per-call options"). repo: all three sites use `Config{...}` literals with `//nolint:exhaustruct` (agent.go:1129, worker.go:398, rename.go:24). Impact: cosmetic — the nolint markers exist only because unset optional fields are indistinguishable from forgotten ones. Migrating `persistOutcome`'s hooks to `retry.WithOnRetry(...)`/`retry.WithExhausted(...)` would delete one suppression class. This is now the ONLY remaining MISSED row.

**F4 — ~~Exhaustion is invisible to evidence~~ RESOLVED (landed 2026-11-09/10).** lib: `OnExhausted(attempts, err)` fires once with the exact last error (checkout config.go; FEATURES "Exhaustion callback"), and `retry.ErrExhausted` is `errors.Is`-able while still unwrapping to the last attempt error (README "Errors"). repo: `persistOutcome` now carries the hook — worker.go:413-416 logs ERROR "outcome write exhausted store-busy retries; the lease-reclaim backstop owns the task now" with task, attempts, err — and `TestPersistOutcomeExhaustionIsLabeled` (worker_test.go:1786-1806) pins BOTH halves: the `retry.ErrExhausted` wrapping and the ERROR-level label. A 3×-exhausted SQLITE_BUSY write now reads as a known handoff to the reclaim backstop, exactly as AGENTS.md describes the design.

**F5 — retryx correctly leans on the fail-open default.** lib: unknown errors classify Transient → retryable (checkout classify.go default step; README "fail-open for retry"). repo: rename.go:24-37 retries any `os.Rename` error 3× in 180ms max — right for dynamic Windows sharing violations (package doc, rename.go:1-8), with `os.IsNotExist` mapped to success inside the attempt. Deliberate and correct; noted so nobody "fixes" it into a narrow predicate.

**F6 — No deterministic-jitter tests exist.** lib: `WithRandomSource` + `JitterNone` make delay sequences reproducible (checkout options.go; FEATURES "Deterministic RNG"). repo: no `retry.With*` anywhere, and no test pins the three sites' delay behavior (they sleep real 50-100ms). Impact: minor test-speed/flake exposure only; the loop logic is what the tests pin.

## 4. Score + top opportunities

Inputs (re-derived 2026-11-11): N=14 rows, N/A=5, A=9. FULL=6 (Do, custom predicate, DoWithValue, OnRetry, OnExhausted, sentinel discrimination), PARTIAL=2 (1.0), MISSED=1 (options tail), MISUSED=0 → Σ=7.0.
Score = round(100 × 7/9) = **78 — high** (was 44 before the landings).

Top opportunities, cheapest first: (1) F3 — options tail at the three sites to drop the last `//nolint:exhaustruct` class (cosmetic, the only MISSED row left); (2) F6 — deterministic-jitter delay tests via `WithRandomSource`/`JitterNone` (test-speed, not behavior). (Former #1 DoWithValue and #2 exhaustion labeling both landed 2026-11-09/10.)

## 5. Version currency + risk

Pinned v0.9.0 == latest tag (checkout 5 commits past tag, `v0.9.0-5-ga3ed997` re-checked 2026-11-11; Unreleased empty) — no action. v0.9.0 (2026-10-27, CHANGELOG) added last-attempt code promotion (N/A here, see table) and restored the `go 1.26`/error-family v0.11.0 pins after an auto-committed bump incident; tq's MVS resolves error-family v0.12.0 regardless, which is the intended graph. Risk: `go-retry` declares `go 1.27.3` (checkout go.mod:3), matching tq's floor — pin-safe. The 100%-coverage, nightly-fuzz/stress posture (README "Development") makes the pinned surface low-risk to lean on harder.

## 6. Conclusion

The convention does what AGENTS.md §Conventions says: every in-process transient retry in the repo is a `retry.Do`/`retry.DoWithValue`, and the exemptions (supervisor loops, persisted ladder, dlqbackoff) are honored rather than abused. The 2026-11-09/10 landings closed both former top gaps — the executor seam now returns values through `DoWithValue`, and exhausted store-busy writes are loud and sentinel-discriminated — moving utilization 44 → 78. What's left is polish, not adoption: the options tail (the last MISSED row) and deterministic-jitter tests. The library is at its latest tag; deepening use carries no version risk.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
grep -rl --include='*.go' 'larsartmann/go-retry' . | grep -v '^./vendor/'   # → agent.go, worker.go, rename.go (+ tests)
grep -n 'retry\.' internal/executor/agent.go internal/worker/worker.go internal/retryx/rename.go
# → agent.go:1129 (DoWithValue), worker.go:398 (Do), rename.go:24 (Do) — the only retry.* production call sites
grep -rn 'retry.DoWithValue\|retry.ErrExhausted' --include='*.go' internal | grep -v '^./vendor/'
# → agent.go:1129 + worker_test.go:1806 (both F2/F4 landings, verified 2026-11-11)
grep -rn 'retry.With\|retry.FromPolicy\|retry.Backoff\|retry.ComputeDelay\|retry.JitterNone' --include='*.go' . | grep -v '^./vendor/'
# → no output (options tail still unused, the one MISSED row)
grep -n 'go-retry' internal/executor/go.mod internal/worker/go.mod internal/retryx/go.mod   # → v0.9.0 direct
grep -rn 'go-retry' internal/dlqbackoff/    # → no output (exempt module, §1)
sed -n '290p' AGENTS.md                     # the convention line
grep -n 'OnExhausted' internal/worker/worker.go    # → 413 (the F4 landing)
sed -n '1786,1806p' internal/worker/worker_test.go  # exhaustion contract test
sed -n '398,421p;442,450p' internal/worker/worker.go  # persistOutcome + string-sniff predicate
sed -n '63p;174,178p;238p' internal/worker/worker.go  # exempt hand-rolled ladder/jitter
git -C /home/lars/projects/go-retry describe --tags  # → v0.9.0-5-ga3ed997
```
