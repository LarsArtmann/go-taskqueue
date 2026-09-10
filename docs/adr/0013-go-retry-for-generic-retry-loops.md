# ADR-0013: Generic retry loops use go-retry

Date: 2026-09-10
Status: Accepted

## Context

The codebase accumulated three independent hand-rolled retry loops — the
executor's ETXTBSY absorption, the sqlite fresh-DB migration race retry,
and the harvest watch reconnect supervisor — each a private re-derivation
of exponential backoff with its own subtle semantics (ctx handling, error
wrapping, cap behavior). LarsArtmann already ships
[`go-retry`](https://github.com/larsartmann/go-retry): a dependency-light
retry loop with exponential backoff + jitter and a pluggable retryable
predicate. Hand-rolling it again is exactly the "reimplemented everywhere,
each with its own bugs" failure mode.

## Decision

1. **Any new generic retry loop MUST use `github.com/larsartmann/go-retry`.**
   Configure via `retry.Config`; pass a predicate (typically
   `errors.Is(err, SomeSentinel)`) instead of relying on the default
   error-family classification — taskqueue errors are not family-classified
   and must not be pushed into families to please the library.
2. **Use `retry.Do` with an outer capture variable when the retried
   operation produces a value needed on the error path** (e.g. captured
   process output as failure evidence). `retry.DoWithValue` returns the
   zero value when all attempts fail — this cost us a SIGSEGV before a
   test caught it.
3. **Wrap-chain contract (test-pinned):** exhaustion wraps
   `retry.ErrExhausted` but `errors.Is(err, originalErrno)` stays true.
   `TestExecWithTransientRetry` fails if that regresses.
4. **Documented exceptions — do NOT migrate these to go-retry:**
   - `internal/harvest/watch.go` `Run`: a reconnect _supervisor_ whose
     success case is "the stream ended" — `retry.Do`'s nil-stops-retrying
     semantics are the wrong shape.
   - The queue's NotBefore backoff ladder (`worker.Backoff`,
     `queue.Fail(backoff)`): persisted journal-fact domain state, not a
     loop. Both sqlite and postgres backends share its semantics via the
     conformance suites.
   - `sleepContext`-style pauses inside state machines (pace/debounce),
     which are scheduling, not retry.

## Consequences

- go-retry (and its go-error-family dependency) appear in the executor and
  queue/sqlite module graphs as a direct dependency, and in worker/root as
  transitive.
- Behavior deltas from the old hand-rolled loops, all beneficial and
  intentional: backoff now includes additive jitter (old ladders were
  deterministic — thundering-herd-prone when several workers raced the same
  transient failure), and retry backoff sleeps honor context cancellation
  (the sqlite migration loop ignored ctx).
- Verified during adoption: no runtime backoff duplication with
  third-party code — the `cenkalti/backoff/v4` indirect in the root module
  belongs to templ's codegen CLI, not to any runtime path.
