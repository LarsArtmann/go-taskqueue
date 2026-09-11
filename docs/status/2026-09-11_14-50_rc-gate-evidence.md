# RC gate at HEAD — release-candidate evidence (Round 11, T1)

- **When**: 2026-09-11 14:50 CEST
- **HEAD**: `097a8b1` at gate time (working tree: concurrent agent's format
  pass + this window's rate-limit armor, all gate-green together)
- **Verdict**: **RELEASE-GRADE** — every gate green; the pool flip is safe.

## Gates

| Gate | Result | Notes |
| --- | --- | --- |
| Root build+vet+`go test -race` (GOEXPERIMENT=jsonv2) | GREEN | cmd/tq 8s, e2e 18s, webui 11s |
| 7 sub-modules, GOWORK=off build+vet+test -race | GREEN | executor, journal, queue(+sqlite/postgres), task, worker |
| `./scripts/ci-local.sh` | GREEN | after aligning task/journal/queue go.mods to `go 1.26.7` (the check caught the drift the module split left); `go mod verify` flaked once on `/mnt/buildcache` (missing file), green on retry — environmental |
| `nix build` | GREEN | no vendorHash drift after the go.mod bumps |
| `nix flake check` | GREEN | x86_64-linux |
| Smoke webui / status-loop / dogfood-once / bootstrap-install / release-gates | ALL GREEN | |
| Smoke ratelimit-e2e (NEW, `scripts/smoke/ratelimit-e2e.sh`) | GREEN | stub 429 agent → parked pending, attempts 0, `retry_in_ms>0` fact, real binary, scratch DB |

## Rate-limit armor added this window (T3)

- `internal/executor/testdata/ratelimit_incident_000001a08edf.txt`: the
  EXACT lastError tail of the incident task, pinned in `TestDetectRateLimit`
  (both shapes: usage-limit-with-reset and request-rate-without).
- TZ sweep (UTC, +14, -11, IST, EST, Zurich): all green after making the
  RFC3339 case dynamic (the old fixed-timestamp expectation failed in
  negative-offset zones — caught, fixed).
- `TestParkedRequeueNotResurrectableByStaleLease` (sqlite + postgres
  conformance subtest): parked requeue clears the lease; stale-owner
  Heartbeat/Complete/Requeue/Fail all refuse. **Found + fixed a real
  sqlite gap**: `Store.Fail` skipped the RowsAffected re-check, so a stale
  owner could append `task.failed`/`task.dead-lettered` facts for a task it
  no longer holds (postgres already had the check; sqlite now matches).
- `FuzzDetectRateLimit`: seed corpus from real provider failure logs;
  15s campaign green. Found + fixed a real bug: far-future reset
  timestamps overflowed `time.Duration` in `reset.Sub(now)` (now compares
  via `now.Add(maxRateLimitWait)`); regression seed committed.

## Residual risks (not flip-blockers)

- Master CI nix job red since 06:25 (runner FOD mismatch) — T5 in the plan;
  local nix build is green.
- `tq mod verify` buildcache flake is environmental (retry clears it).
