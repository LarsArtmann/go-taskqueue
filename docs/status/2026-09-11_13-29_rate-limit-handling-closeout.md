# Provider rate-limit handling (Z.ai 429 usage windows, synthetic.new quota) — feature window close-out

- **When**: 2026-09-11, ~13:05–13:29 CEST (report written 13:29)
- **Branch**: master, unpushed (never pushes)
- **Dispatch type**: direct owner instruction in-session ("go-taskqueue needs to
  be properly able to deal with ratelimits especially for Z.ai and preferably
  also for synthetic.new") — NOT a harvested queue task, no `Task-Queue-ID`
  footer, no TQ_RESULT contract
- **Trigger evidence**: the live dead task `000001a08edfbc90bf02dd35ec0d5e7bf524`
  (pasted from the dashboard): Z.ai answered `429 "Usage limit reached for 5
  hour. Your limit will reset at 2026-09-11 19:40:34"`, crush gave up after its
  own 5s/10s/20s ladder, the task burned 3/3 attempts and dead-lettered at
  12:10 — after spending 07:17→12:09 in the dirty-tree preflight ladder.
- **Work commits**: landed via the auto-commit daemon
  (1a7f3a9, 39835a0, e5cc64a, 2467776, bc1e331 — code + tests + AGENTS.md).

## What shipped (one paragraph)

A failed agent turn's output is now scanned for provider exhaustion
(`executor.DetectRateLimit`, internal/executor/ratelimit.go). A Z.ai-style
reset timestamp ("Your limit will reset at <ts>"), an RFC3339 renews-at, or a
numeric `retry_after` yields a `*executor.RateLimitError{RetryAfter}` and the
worker requeues the task WITHOUT burning an attempt, parked until the provider
resets (±5% jitter capped ±1min; unparseable reset → 15min fallback, 6h hard
cap). synthetic.new's OpenAI-style quota 429 (`insufficient_quota`, no
timestamp in the body — verified against dev.synthetic.new docs) is covered by
the fallback. `AgentExecutor` also keeps an in-process gate: after one 429,
sibling agent/review/status runs in the same pool fast-refuse without spawning
crush until the window passes.

## a) FULLY DONE

1. **Research from primary sources**: Z.ai's 429 shape taken from the REAL
   dashboard paste (reset timestamp format `2006-01-02 15:04:05`, wall-clock,
   zone-less); synthetic.new's error convention verified via
   dev.synthetic.new (OpenAI-compatible 429, `insufficient_quota`, quotas
   endpoint has `renewsAt` but the error body carries no timestamp) — so the
   fallback path is not a guess.
2. **Executor layer**: `RateLimitError` (Cause + RetryAfter), idempotent
   `RateLimited()` constructor, `DetectRateLimit(output, now)` with the
   trust order reset-timestamp > retry-after > default, `maxRateLimitWait`
   6h cap, 30s claim grace, regex set that deliberately does NOT match
   crush's own `retry_delay=` ladder (5s/10s/20s is the agent's short game,
   not the quota window).
3. **Gate**: `rateLimitUntil atomic.Int64` on `AgentExecutor`; `armRateLimit`
   keeps the MAX of concurrent observations; `runAgent` fast-refuses before
   spawning when the gate holds; arms from BOTH the work turn and the
   close-out turn; detection sites sit after the ctx-cancelled branch so a
   cancelled run is never misclassified.
4. **Worker layer**: `*RateLimitError` branch requeues without attempt burn
   via `store.Requeue` (task.requeued fact carries reason + retry_in_ms —
   forensics for free), ±5% jitter capped ±1min, log line with the parsed
   wait. Placed before the permanent branch; preflight ladder untouched.
5. **cmd/tq copy fix**: `go vet` caught that `reviewExec := *agentExec` became
   a copylocks violation once the executor carried an atomic. Replaced with
   `WithoutCloseout()` — a field-wise clone (all runtime settings, close-out
   stripped, FRESH gate) — with a pinning test. Gates are per executor
   instance by design: cross-pool/cross-provider setups each re-learn their
   own window with one cheap probe.
6. **Tests, all green**: detection table (9 cases incl. past-reset timezone
   accident, cap, RFC3339, negative cases), gate semantics, idempotent
   wrapper, e2e stub-agent run (429 → classified → second Execute refuses
   without invoking the binary → gate expiry re-probes), worker contract
   (MaxAttempts=1 survives a 429, then completes without rescue).
7. **Full verification gate GREEN**: root `build + vet + test -count=1 -race`
   (exit 0, no failures), executor + worker modules with `-race`, `gofmt -l`
   clean, goimports correct. No new golangci/gosec findings in touched
   functions (advisory baselines untouched).
8. **AGENTS.md updated**: new "Provider rate limits" bullet under payload
   contracts (the contract surface, not a Known Issue — it's a feature).
9. **Session-start ritual honored**: git log/status/stash checked first; the
   daemon's mid-flight commits were read and built on, never reverted.

## b) PARTIALLY DONE

1. **Deploy path**: the fix is committed but the RUNNING pool
   (`tq-agent-pool` systemd, journal /mnt/pool/services/tq/tq.db) still runs
   the old binary. Pickup requires the SystemNix input flip + `nix run
   .#deploy` — owner-run by standing decision. Until then the production pool
   keeps dead-lettering on 429s.
2. **The already-dead task** `000001a08edf…` sits in the DLQ with 3/3 attempts
   burned. The new code cannot resurrect it; it needs `tq dlq` + rescue (or
   cancel) — owner action, and its work item (anti-ghost-archive gate) is
   ALREADY DONE by the f9 archive session (TODO item checked), so cancel may
   be the honest move, not rescue.
3. **Machine-wide gate**: the gate is in-process. Two pools on one host
   sharing one Z.ai account each pay one probe per outage window. Documented
   tradeoff, not a defect — but a flock-based shared gate file (the
   MaxConcurrent slot-lock pattern already exists) would make it host-wide.
4. **Dirty-tree interaction**: after a 429 mid-work, the agent's partial
   changes often leave the repo dirty, so the post-reset claim hits the
   preflight ladder (the 07:17→12:09 spin in the incident). The rate-limit
   requeue now prevents the ATTEMPT burn, but the re-claim still waits behind
   the dirty-tree guard as designed (RequireClean protects human WIP). A
   rate-limit-aware preflight exception ("repo dirtiness predates my claim")
   was deliberately NOT attempted — too easy to trample real human work.

## c) NOT STARTED

1. `nix build` / flake devShell verification of the change (no go.mod/go.sum
   changes were made — vendorHash drift risk is zero — but the nix test gate
   was not run).
2. `./scripts/smoke/status-loop.sh` / `dogfood-once.sh` smokes against a
   scratch DB (the verify gate + unit tests cover the logic; a live smoke
   would need a stub that emits 429-shaped output).
3. A TODO_LIST.md item capturing the deploy step (the pool task that installs
   the new binary) — the session ended before writing one.

## d) TOTALLY FUCKED UP

1. **Two wrong test expectations shipped in the first pass** (caught by my
   own test run, fixed within the window): the "past reset" case used a
   timestamp that was actually in the FUTURE relative to the fixed `now`, and
   the RFC3339 case hardcoded a UTC-relative duration that both ignored the
   6h cap and depended on the host timezone — the suite would have failed on
   any non-UTC runner. Lesson applied: compute expectations from absolute
   instants, never from wall-clock arithmetic in the test body.
2. **vet found the copylocks AFTER I claimed build success** twice: my first
   "BUILD-OK" ran only inside internal/executor; the root `go vet ./...` then
   flagged cmd/tq/agentpool.go. The module-scoped build was correct but the
   "done" feeling wasn't — the cross-module consumer check belongs in the
   first verification pass, not the third.
3. **Nothing else.** No reverts of concurrent work, no stray files, no string
   literals touched by edits, no scope creep into the preflight ladder or the
   dead task's rescue.

## e) WHAT WE SHOULD IMPROVE

1. Verification discipline: run the ROOT gate (build+vet+test) immediately
   after the first compile of a sub-module change — cmd/tq consumes executor
   types and vet's copylocks only fires at the consumer.
2. The rate-limit regex set is evidence-driven but frozen in code: when a new
   provider's 429 shape appears, detection silently degrades to the 15min
   fallback (safe but slow). A tiny `tq doctor --ratelimit-selftest` fed by
   real failure logs would keep the patterns honest.
3. `DetectRateLimit` is exported for tests/composability but only the agent
   family scans output; the `sh`/`http` executors could reuse it (an HTTP
   executor hitting a 429 today burns attempts exactly like the incident).
4. Per-provider gates: crush's output carries `provider=zai` in the very log
   line we match; extracting it would let one pool run two providers without
   cross-gating.
5. The gate's probe-on-expiry costs a full crush spawn (~seconds + potential
   billed tokens). A cheaper probe (a 1-token API ping) is out of scope for a
   queue that treats the agent binary as a black box, but worth noting.
6. Status-index hygiene: this report self-indexes (row added in the same
   commit) — the 2026-09-08 five-unindexed-reports incident keeps recurring
   because indexing is a separate manual step.

## f) UP TO 50 THINGS TO DO NEXT (prioritized top-down)

1. **Deploy**: SystemNix input flip to this commit + `nix run .#deploy` so
   the pool stops burning attempts on 429s (owner-run).
2. **Triage the DLQ**: `tq dlq` — rescue-or-cancel `000001a08edf…` (its item
   is already done; cancel is likely correct).
3. Verify the 19:40:34 Z.ai reset today actually re-opens the flow: after
   deploy, watch the next 429 requeue with `retry_in_ms ≈ 5h` and zero
   attempt increment.
4. File TODO_LIST items (harvestable):
   - flock-backed host-wide rate-limit gate file (reuse slot-lock pattern).
   - provider-tagged gates (extract `provider=<x>` from crush output).
   - `http` executor 429 → RateLimitError reuse.
   - rate-limit-aware preflight exemption for agent-caused dirt (design
     first; fail closed).
5. Add a fixture-based test pinning the EXACT paste output from the incident
   (the dashboard tail) as a committed regression case.
6. Consider `retry_after` units beyond seconds (HTTP allows HTTP-date) —
   currently only numeric seconds parse.
7. Document the gate's per-instance scope in the pool banner at startup
   (`--max-agents` users assume machine-wide semantics).
8. Fuzz `DetectRateLimit` (FuzzParseRepo-style seed corpus with real provider
   failure logs) — regexes on untrusted output deserve the campaign.
9. Sweep for other unbounded-retry sites that meet 429s: bridge HTTP calls to
   PapDashboard (`alert.triggered` posts) have their own retry behavior —
   confirm they back off on 429 rather than hammering.
10. `tq doctor`: surface "provider currently rate-limited (gate armed until
    <ts>)" so an idle pool is diagnosable as WAITING, not broken.
11. Web UI: render `task.requeued` facts with `reason` starting "rate
    limited" in a distinct color/label — the dashboard paste shows requeue
    reasons are truncated to unreadability.
12. Cap audit: confirm `maxRateLimitWait=6h` still makes sense if Z.ai ships
    monthly quota tiers (reset could legitimately exceed 6h).
13. Teach the harvester nothing new — confirm — but check `catchup:` items
    don't enqueue DURING a window and immediately probe (they will; one probe
    per task is the design cost; consider a pool-level "hold new enqueues
    while gate armed" flag).
14. Metric: expose rate-limit requeue counts in `tq stats` (operators should
    see "11 tasks parked until 19:40" at a glance).
15. Closeout turn cost: a 429 during close-out requeues the WHOLE task after
    a successful work turn — verify the re-claim resumes via `--session`
    (payload Session field persists? it does NOT for the work-then-closeout
    path: re-execution re-runs the WORK turn, doubling cost). **Real gap —
    design a resumable marker or accept the double work explicitly.**
16. Same-task double jeopardy: requeue-until-reset uses `store.Requeue`
    (status pending) — confirm lease-expiry reclaim cannot resurrect a
    RUNNING twin while parked (Requeue clears lease_owner; safe, but pin
    with a store test).
17. Windows CI: rate-limit tests are in untagged + unix-tagged files; the
    detection table runs on windows-latest — confirm no path assumptions
    (none known; time.Local only).
18. golangci advisory: new regexes/consts may trip mnd/gocritic in future
    baseline refreshes — pre-tag the constants file in the triage notes.
19. CHANGELOG: the docs-health status sweep will pick this up; if the sweep
    is delayed, append a line manually (append-only file).
20. FEATURES.md: add "provider rate-limit aware requeue (Z.ai/synthetic.new)"
    under DONE.
21. docs/DOMAIN_LANGUAGE.md: add "rate-limit window", "gate", "probe" terms.
22. Review executor: review tasks hitting 429 requeue as review tasks —
    confirm the sweeper's dedup (`review:<task-id>`) doesn't mint a SECOND
    review task while the first is parked (watermark logic — needs a read).
23. Status executor: same question for `status:<project>:<trigger-id>` dedup.
24. Budget interplay: `internal/budget` checks before pool ticks — a parked
    task still consumes a tick slot? Confirm parked (not_before future)
    tasks are simply not claimed (they aren't) and the budget projection
    doesn't count refused runs (they complete no facts beyond requeued).
25. Alert bridge: a rate-limit storm parks many tasks; dead-pool detection
    (`--dead-pool-ticks`) must not fire while the pool is healthily PARKED —
    verify ticks only count scan-failures, not parked claims (they do, but
    pin with a test).
26. Test the jitter bounds property-style (delay always within ±1min of
    RetryAfter, never negative for tiny values).
27. Add `DetectRateLimit` coverage for multi-line outputs where the 429 line
    is beyond the tail window we keep (we scan the FULL buffer — confirm the
    closeout path scans the merged buffer, it does).
28. Store conformance: add a postgres parity case for Requeue with a 5h delay
    (sqlite covered; postgres suite lacks a long-delay case).
29. README (sales page): one sentence under reliability — "provider rate
    limits park tasks until the window resets instead of burning retries".
30. Security review of the new code path: no new inputs cross trust
    boundaries (regexes run on agent output already stored as evidence);
    confirm no PII from provider messages lands in facts beyond what
    task.failed already carried (retry_in_ms + error text — same class).
31. Consider `RateLimitError.RetryAfter` in the requeued fact's reason text
    (currently only in the error prefix — it IS there; verify human-readable
    in `tq show`).
32. Add an e2e smoke: stub agent 429 → assert task pending with attempts=0
    and a requeued fact carrying retry_in_ms > 0 (worker test covers the
    store path; the smoke would prove the CLI binary wiring).
33. Gate visibility: log the gate arming once per instance ("provider
    rate-limited until ~19:40; refusing sibling runs") — currently only the
    detection log line exists.
34. Re-check AGENTS.md wording after deploy: it says "same pool" — confirm
    the multi-pool-per-host case is what operators actually run.
35. Explore sharing the gate through the existing `agentlock` flock files to
    get host-wide semantics without new infrastructure.
36. Unit test: `WithoutCloseout` on a zero-value executor (nil-safety of the
    fresh gate — trivially safe, pin anyway).
37. Check `tq api` (read-only API) exposes requeue reason so scripts can
    monitor rate-limit parks.
38. Timezone honesty: Z.ai's zone-less timestamp is parsed as agent-host
    local — if the provider means UTC, parks are off by the offset; add a
    sanity comment/test noting the fallback catches the error case.
39. Dead-letter text: if a task STILL dead-letters after rescue with repeated
    429+preflight interleaving, ensure LastError's "rate limited" prefix
    makes DLQ triage one-glance clear (it does via rl.Error()).
40. Pre-commit hook: status-index check ran? This report self-indexes; run
    `scripts/check-status-index.sh` before finishing the session.
41. Run `./scripts/check-dead-exports.sh` on the new exports
    (RateLimitError/RateLimited/DetectRateLimit/WithoutCloseout all used;
    advisory audit should stay quiet).
42. Nightly fuzz seeds: if FuzzParseRepo gains a rate-limit sibling, wire the
    nightly campaign the same way.
43. Consider exponential re-probe for the no-timestamp fallback (15min flat →
    15/30/60min ladder capped 6h) if providers prove slow to reset.
44. Pool banner: show the agent binary's provider model name so 429 triage
    knows WHICH provider's window to check.
45. Docs: SECURITY.md unaffected (no new endpoints/flags) — confirm no
    matrix drift, one-line check.
46. Verify `tq watermarks` consumers (bridge/sweepers) don't choke on a
    burst of task.requeued facts during a storm (at-least-once cursor —
    fine, but a storm is the first real burst; watch lag metric once).
47. Dogfood: enqueue a deliberate 429-shaped task against a scratch DB
    (TQ_DB exported!) to watch the full journal trail end-to-end.
48. Sample-size the grace: 30s grace assumed claim+spawn latency; measure
    real claim→crush-API latency from an actual post-reset claim and tune.
49. Consider surfacing "parked until" as a first-class query (`tq tasks
    --parked`) for operator sanity during long windows.
50. Post-deploy retro: one week later, count 429 requeues vs pre-deploy
    dead-letters to prove the feature's value with numbers.

## g) QUESTIONS I CAN NOT FIGURE OUT MYSELF (3)

1. **Z.ai timestamp timezone**: is "Your limit will reset at 2026-09-11
   19:40:34" in the account profile's timezone, the SERVER's timezone, or
   UTC? If it's UTC, my local-time parse parks tasks off by the offset
   (safe direction only if the true reset is EARLIER than parsed — an
   offset making the true reset LATER means one wasted probe, which the
   re-arm absorbs). Answer changes whether I should parse as UTC.
2. **Deploy timing**: should I hold the DLQ triage of `000001a08edf…`
   (rescue vs cancel) until after the SystemNix input flip, or is cancel
   correct NOW since its work item is verifiably done (f9 archive session)?
3. **Multi-provider intent**: are you actually running (or planning) Z.ai and
   synthetic.new pools side by side on the same host? If yes, I'll prioritize
   the provider-tagged gate (f4/f35) so one provider's window doesn't park
   the other's tasks; if no, the per-instance gate stands as-is.
