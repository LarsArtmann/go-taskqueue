# Pool-flip checklist & DLQ triage runbook (Round 11, T2/T4)

- **When**: 2026-09-11 14:45 CEST
- **Purpose**: everything the owner needs to (1) flip SystemNix to the
  rate-limit-fixed HEAD, (2) verify the flip took, (3) resolve the 21 dead
  tasks in the production DLQ with one decision each, (4) measure the fix a
  week later.
- **Inputs**: RC gate evidence at HEAD (this window), live DLQ listing at
  14:40 (production journal `/mnt/pool/services/tq/tq.db`), AGENTS.md
  operational contracts.

---

## 1. Pre-flip state (verified this window)

| Gate | Result |
| --- | --- |
| Root: build + vet + `go test -race` | GREEN |
| All 7 sub-modules (GOWORK=off, race) | GREEN |
| `./scripts/ci-local.sh` | GREEN after toolchain alignment (task/journal/queue go.mods bumped to 1.26.7); `go mod verify` flaked once on buildcache, green on retry |
| `nix build` + `nix flake check` | see §4 evidence note |
| Smokes: webui, status-loop, dogfood-once, bootstrap-install, release-gates, **ratelimit-e2e (new)** | ALL GREEN |
| Rate-limit armor | incident fixture, TZ-sweep (6 zones), parked-requeue pin (sqlite+postgres), `FuzzDetectRateLimit` (found+fixed a Duration-overflow bug) |

**Known master-CI caveat**: ~~the nix CI job was red at the last push (runner
FOD mismatch, T5)~~ — the FOD premise was SUPERSEDED 2026-09-11 (16-00
report §a): the real cause was `setup-go: stable` floating to go 1.27.1
(stdversion gate), fixed by pinning all 7 setup-go steps to 1.26.7; the nix
job is green on recent runs — local `nix build` is green; do not block the flip on CI,
but re-run `nix build` right before `nix run .#deploy`.

## 2. The flip (owner, ~5 min)

1. In SystemNix: bump the go-taskqueue input to the release commit
   (cut a tag first if you want a pinned version; HEAD works too).
2. `nix run .#deploy` (or your SystemNix equivalent).
3. Same window, if desired: add `--allow-writes` to the `tq-serve` unit
   (WebUI rescue/cancel; CSRF + lockout already enforced).

## 3. Post-flip smoke checklist (M7)

| # | Check | Command | Expect |
| --- | --- | --- | --- |
| 1 | Pool alive | `systemctl status tq-agent-pool` | active, fresh log lines |
| 2 | First harvest | `journalctl -u tq-agent-pool -n 50` | `harvest: enqueued` per repo, no `scan failed` |
| 3 | PATH fix live | `tq doctor` (or doctor line in pool log) | `tool:git/go/crush found` (agentPath env) |
| 4 | GOEXPERIMENT fix live | first go-taskqueue verify result | NOT `build constraints exclude all Go files` |
| 5 | 429 handling live | next Z.ai 429 (or force one) | log: `provider rate limited; requeued without attempt burn`, task stays pending, attempts stay 0 |
| 6 | Dashboard | open 127.0.0.1:8100 | fresh facts streaming; (writes ENABLED) banner if you flipped `--allow-writes` |

## 4. DLQ triage — 21 dead tasks at 14:40 (M8/M9)

**Rules** (decision per class, then one command per task):

- **RESCUE** — the fix makes a retry meaningfully different (429 class).
  Command: `tq dlq --rescue <ID> --max-attempts 3`.
- **CANCEL** — the work item is (still) live in TODO_LIST.md and the pool
  will re-harvest it fresh with a clean 3-attempt budget; rescuing would
  just resume a stale payload. Command: `tq dlq --rescue` has no cancel twin
  in the CLI — use `tq cancel <ID>` (write path; the WebUI cancel button
  needs `--allow-writes`).
- **VERIFY-then-decide** — the failure was environmental (the
  `GOEXPERIMENT=jsonv2` verify-gate lie, 2026-09-11) or the work already
  landed via another agent; check the repo's git log for the
  `Task-Queue-ID` footer before deciding.

| Task | Project | Class | Work item | Verdict | Why |
| --- | --- | --- | --- | --- | --- |
| 000001a08eb1f904c4 | go-taskqueue | 429 | dogfood smoke: scripts/smoke/dogfood-once.sh … | RESCUE after flip | 429 burned the attempts; smoke now exists — a rescue validates the fix live |
| 000001a08edfbc90bf | go-taskqueue | 429 | Anti-ghost-archive gate … | CANCEL | the gate shipped via another window (check-ghost-archives.sh is in ci-local) |
| 000001a08ef20c1a4e | SystemNix | 429 | Reboot into kernel 7.2.2 + FastFlowLM … (USER-gated) | CANCEL | owner-gated item; pool should never have eaten it |
| 000001a08ef69ff8e5 | SystemNix | 429 | (item text gone from TODO_LIST) | CANCEL | harvest provenance gone; stale |
| 000001a08f2d8e6f94 | CV | 429 | Pixel-baselines CI experiment … | RESCUE after flip | 429 class, item presumably still open |
| 000001a08f4905b7fe | SystemNix | 429 | btrbk-data marker-gate fast-fail … | VERIFY | check SystemNix TODO_LIST; if still unchecked, RESCUE after flip |
| 000001a08e0d2a20c5 | CV | closeout | Inbound recruiter leads → funnel … | VERIFY | closeout deadline; work turn may have committed — check CV git log for the footer |
| 000001a08e0d2a27ec | SystemNix | closeout | /data corruption repair … | VERIFY | same; large risky item — decide by hand |
| 000001a08a4851ce7f | go-taskqueue | verify | cwd-dependence sweep … | CANCEL | verify gate caught it; item re-harvests fresh |
| 000001a08a4851e129 | go-taskqueue | verify | (empty item) | CANCEL | stale payload |
| 000001a08e51d73180 | go-taskqueue | verify | `tq doctor --repos` expand bare names … | VERIFY | doctor bare-name expansion LANDED this window (doctor.go) — confirm, then CANCEL |
| 000001a08e5f906aeb | CV | verify | SOMI attachment verification (OWNER) … | CANCEL | owner-gated |
| 000001a08e6424643b | go-taskqueue | verify | module-eval hardening … | CANCEL | re-harvest fresh |
| 000001a08e71df95f8 | go-taskqueue | verify | `checkProjectsDir` skip when all repos absolute … | CANCEL | LANDED this window (agentpool.go diff) — confirm, then CANCEL |
| 000001a08e7f9b3428 | go-taskqueue | verify | harvest skip-log change detection … | VERIFY | partial work may exist in cmd/tq/main.go (groupedSkips) — confirm, then CANCEL |
| 000001a08e88c2f67d | CV | verify | Architecture naming test vs `*_scripts.go` … | CANCEL | CV-local; re-harvest |
| 000001a08e88c2fb41 | go-taskqueue | verify | `tq doctor`: warn when crush/git/go missing from PATH … | CANCEL | LANDED (doctorEnvironment tool checks, this window's diff) |
| 000001a08e967ebc09 | go-taskqueue | verify | dead-pool detection via PapDashboard … | CANCEL | shipped earlier (NotifyDeadPool, FEATURES.md) |
| 000001a08eb1f5d0be | CV | verify | internal/cli/crm TestRunSync … | CANCEL | CV-local; re-harvest |
| 000001a08ebfb1c792 | go-taskqueue | verify | archive dogfood evidence durably … | VERIFY | evidence archive may exist under docs/status/assets — check, then decide |

**Bulk path** (if you accept the table): rescue the 429-class RESCUE rows
after the flip, cancel the rest:

```bash
# after the flip:
tq dlq --rescue 000001a08eb1f904c4... --max-attempts 3
tq dlq --rescue 000001a08f2d8e6f94... --max-attempts 3
# cancel the rest (loop over the CANCEL rows of the table)
```

## 5. Post-flip retro scaffold (M17, run 2026-09-18)

| Metric | Definition | Source |
| --- | --- | --- |
| 429 requeues / week | count of `task.requeued` facts whose reason contains `rate limited` | `tq facts` / journal query |
| 429 dead-letters / week | dead tasks whose lastError matches the 429 wall (target: 0, was 6 in the incident window) | `tq dlq` + lastError grep |
| attempts burned by 429 | dead 429 tasks × attempts (target: 0) | DLQ listing |
| pool idle-vs-waiting | `tq doctor` shows the armed gate (after M32) vs silent idle | doctor output |
| baseline | 2026-09-11: 27 dead total, 6 of them 429-class, 21 in the 14:40 listing | this runbook §4 |

Record the numbers in a status report (`docs/status/`, indexed) per TODO
item "Post-deploy retro" (M124).

## 6. Flip v2 — the GOEXPERIMENT env fix (round-13 T4, 2026-09-12)

The pool unit env carries no `GOEXPERIMENT`, so every root-module verify
dies on `encoding/json/v2` build constraints REGARDLESS of repo state
(5+ windows burned, 2026-09-11 task 000001a08ebf). Minted verifies are now
env-self-contained (round-13 T2), and `tq doctor` carries an env-lie
detector (round-13 T3) — but the true fix is one line only the owner can
set. Bundling it into the next flip costs zero extra deploys.

**The one line** (SystemNix module, wherever `services.tq-agent-pool` is
declared — mirrors the existing `agentPath` treatment):

```nix
services.tq-agent-pool.environment.GOEXPERIMENT = "jsonv2";
```

If the module has no `environment` option yet, the raw NixOS override on
the generated unit is equivalent:

```nix
systemd.services.tq-agent-pool.environment.GOEXPERIMENT = "jsonv2";
```

**Deploy sequence** (same window as the input flip, §2):

1. Add the line above next to the existing `services.tq-agent-pool.agentPath`.
2. Bump the go-taskqueue input (needs a build with the T2/T3 code — the
   detector ships with `tq doctor`).
3. `nix run .#deploy`.

**Post-flip verification (new `tq doctor` env check, run ON the host)**:

```bash
systemctl status tq-agent-pool            # active, fresh log lines
sudo -u <pool-user> env systemctl show tq-agent-pool -p Environment | grep GOEXPERIMENT
# expect: Environment=GOEXPERIMENT=jsonv2 (backticked file §6 evidence: doctor exit 0)
TQ_DB=/mnt/pool/services/tq/tq.db /run/current-system/sw/bin/tq doctor | grep go-env
# expect: ok   go-env   encoding/json/v2 builds in this environment — no env lie
```

A `FAIL go-env ENV-LIE ...` verdict after this flip means the line did not
reach the unit — re-check `systemctl show` before anything else.

Answer format for the owner: apply §6 now (one line + the scheduled flip),
or defer explicitly — but then every jsonv2 verify failure in the pool is
a KNOWN lie, and `tq doctor` is the reference for it.
