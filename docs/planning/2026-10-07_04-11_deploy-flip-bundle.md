# SystemNix deploy-flip bundle — bring the Flash pool back to life (2026-10-07 04:11)

> **Owner-run request.** Every step below that touches evo-x2 needs sudo
> and cannot be executed by an agent (TODO_LIST rows 19, 56, 64, 89, 109,
> 111). This document is the complete, verified-against-HEAD bundle: flip
> instructions, flags matrix, unit env, post-deploy smokes, Gatus probe.
> Nothing here is new engineering — it is the assembly of everything
> shipped since the pool died 2026-09-10.

## 0. Why now (one paragraph)

The deployed pool binary is a 2026-09-08 rev-pin (28a8ae4) that predates
EVERY hardening, IO, redaction, budget, read-model, and UX window since.
The pool was found dead 2026-09-10 (bare repo names, missing PATH tools,
silent scan-fail logging); all three fixes are on master. Since then
master additionally gained: rate-limit armor (429 no-burn requeue),
secrets redaction (default ON), done-preflight, the claim-time budget
gate, the 2026-10-07 IO pass (36× fewer fsyncs, 35× faster deep-history
claims), partial claim indexes, idle backoff, pooled HTTP transports,
and the S3 read-model projection as the default stats surface. None of
it is reachable until the input flips.

## 1. The flip (owner, ~10 min)

```bash
# 1. In SystemNix (evo-x2 checkout):
#    bump the go-taskqueue input from the rev-pinned git+file input (28a8ae4)
#    to the rolling upstream:
#      github:LarsArtmann/go-taskqueue?ref=master
#    (or pin an exact sha for reproducibility — verify freshness first:)
git -C /path/to/go-taskqueue fetch origin && git -C /path/to/go-taskqueue log --oneline -1 origin/master

# 2. Unit env (TODO row 64, RULED IN O15): tq-agent-pool unit gains
#      Environment=GOEXPERIMENT=jsonv2
#    (same treatment as the existing agentPath env; ends the 5-window
#    re-dispatch burn from "build constraints exclude all Go files")

# 3. Optional same-window: add --allow-writes to the tq-serve unit flags
#    (TODO row 56) so WebUI DLQ rescue/cancel work; CSRF + lockout are
#    already enforced in-code, loopback-only bind stays.

# 4. Deploy:
nix run .#deploy    # starts/updates tq-agent-pool tq-serve tq-storage-dir tq-bootstrap

# 5. Verify units:
systemctl status tq-agent-pool tq-serve --no-pager
```

**Pre-flip sanity (agent-verified at 9e1c3cd0+, re-verify at flip time):**
root `./...` build+vet+test green; all 21 sub-modules green (GOWORK=off);
`scripts/test-cmd-tq.sh` green. Remote master CI is red on the b886a677
dep-bump lineage (run 37556713162) — that red predates the heals; do not
block the flip on it, but re-run `nix build` on the flip commit.

## 2. Flags matrix — deployed v0.3.0-era vs master HEAD

| Surface | Flag / default | Deployed (28a8ae4) | Master HEAD | Action |
| --- | --- | --- | --- | --- |
| worker | `--redact` | absent | present, **default ON** (env `TQ_REDACT`) | none — activates on flip |
| worker | `--reresolve-verify` | absent | present (agent verify re-resolution) | optional: add to unit if wanted |
| worker | `--daily-budget N` | absent | present (claim-time budget gate) | optional: set a cap |
| worker | `--batch-items N` | absent | present, default OFF | leave OFF until calibrated |
| worker IO | sqlite pragmas | FULL sync, two pools | **NORMAL sync, ONE shared conn**, partial claim indexes, idle backoff 2s cap | none — activates on flip; escape `TQ_SQLITE_SYNC=full` |
| serve | `--allow-writes` | absent | present (WebUI rescue/cancel) | optional: add to unit (row 56) |
| serve | `--read-model` | absent | present, **default ON** (S3 metaengine projection) | none |
| serve | `--auth-token` | — | required for non-loopback binds | keep loopback OR set token (SECURITY.md) |
| api | `tq api` | absent | present, token-mandatory, lockout | only if used |
| doctor | `--hygiene` | absent (errors on live binary) | present (verify-pin staleness report) | run once post-deploy (§3) |
| stats | `--read-model` | absent | default ON | none |

## 3. Post-deploy smoke checklist (in order, ~15 min)

| # | Check | Command | Expect |
| - | --- | --- | --- |
| 1 | Units up | `systemctl status tq-agent-pool tq-serve` | active (running), fresh log lines |
| 2 | First harvest | `journalctl -u tq-agent-pool -n 50` | `harvest: enqueued` per repo, NO `scan failed` |
| 3 | PATH fix live | pool log `tq doctor` line or run `tq doctor` | `tool:git/go/crush found` |
| 4 | GOEXPERIMENT fix live | first verify result in journal | NOT `build constraints exclude all Go files` |
| 5 | 429 armor live | next provider 429 | log: requeued WITHOUT attempt burn; task pending, attempts unchanged |
| 6 | Redaction live | any completed agent task | output tails masked; `tq audit --journal` shows no SECRET EVIDENCE |
| 7 | Read-model live | `tq stats` | answers from `<db>.readmodel.db` (file exists beside journal) |
| 8 | Dashboard | open `127.0.0.1:8100` (or tq.home.lan) | fresh facts streaming; writes banner if `--allow-writes` |
| 9 | Hygiene pass | `tq doctor --hygiene --json` | report renders (no flag error like deployed v0.3.0) |
| 10 | DLQ state | `tq dlq` | triage per the 2026-09-11 runbook classes (rescue 429-class AFTER flip) |

## 4. Gatus journal-head liveness probe (row 89)

Spec: a probe that fails when the journal stops advancing (dead pool with
a live dashboard is exactly the 2026-09-10 failure: everything green, no
work done).

- **Metric**: journal head sequence + mtime, exposed by the serve health
  endpoint. Master's `/health` (go-health-dashboard adoption) runs inside
  the token gate — Gatus should use the loopback bind with the token, or
  the SystemNix module should front an unauthenticated HEAD-only
  liveness route (owner decision, note in module PR).
- **Check 1 (up)**: `GET /health` → 200.
- **Check 2 (alive, the real one)**: poll `tq stats --json` (cron on the
  host, 15 min interval) → compare `head` to previous sample; alert if
  unchanged across a window longer than the pool's longest expected idle
  (suggest 2h — the pool ticks hourly at most).
- **Wire**: SystemNix Gatus config block for `tq.home.lan` + the
  head-advance condition; name: `tq-pool-journal-advancing`.

## 5. What this flip does NOT cover (parked, deliberately)

- postgres CLI wiring (M15) — sqlite stays the pool engine.
- `--prioritize` JIT scoring (M11) — needs calibration data first.
- Batching default-ON (row 321) — needs pool data first.
- v0.4.0 tag cut (M14) — flip can ride master; a tag follows the cut.
