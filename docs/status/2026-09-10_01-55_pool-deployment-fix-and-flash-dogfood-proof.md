# Pool Deployment Fixed: Dead Pool Root-Caused, Flash Dogfood Loop Proven

_Session 2026-09-10 ~00:45–02:00. Question asked: "how can we use
go-taskqueue on itself to effectively use multiple GLM-5.3-Flash agents?"_

## TL;DR

The Flash workforce was already wired (all three pool repos pin
`zai/glm-5.3-flash --reasoning-effort xhigh` via bootstrap-managed
`.crushrc`) — but the production pool has been **completely dead since its
2026-09-09 05:03 deploy**: every harvest tick skipped every repo as
`scan failed`, zero tasks ever enqueued, and the aggregate log hid the
error for 20 hours. Three fixes shipped on master; a bounded `--once`
dogfood run then proved the full loop live: harvest → GLM-5.3-Flash agent
→ verify gate → review agent (`approve`) → TODO closure → commit with
`Task-Queue-ID` footer. The systemd pool comes to life when the owner
flips the SystemNix input and redeploys.

## a) The dead pool: diagnosis

Symptom chain (evidence: `journalctl -u tq-agent-pool`, `tq top` on
`/mnt/pool/services/tq/tq.db`):

1. `harvest: skipped reason="scan failed" count=3` every 5 minutes since
   deploy; `tq top` showed 0/0/0/0 forever.
2. The systemd unit (from OUR upstream module `deploy/nixos/tq-agent-pool.nix`)
   runs with `WorkingDirectory = dirOf(dbPath)` — and pool.conf carries
   bare repo names (`repos = CV,SystemNix,go-taskqueue`).
   `harvest.ParseRepoAll` Abs()es each entry against the **cwd**, so the
   pool scanned `/mnt/pool/services/tq/CV/TODO_LIST.md` — no such file,
   every repo, every tick.
3. `groupedSkips` classified skip reasons by cutting at the first colon,
   so the journal showed `reason="scan failed"` with the actual error
   dropped — a dead deployment read as a quiet one for 20 hours.
4. Even with the scan fixed, the next wall was already loaded: systemd's
   default service PATH contains no `git`, no `go`, no `crush` — the
   dirty-check preflight, the agent exec, and the `.tq-verify` gate would
   all have failed.

## b) Fixes (all on master this session)

| Fix                                                                                                                                                 | Where                                              | Test                                                                        |
| --------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------- | --------------------------------------------------------------------------- |
| Bare `--repos` names expand against `--projects-dir`, never cwd                                                                                     | `cmd/tq/agentpool.go` (`harvestConfigFromOptions`) | `TestHarvestConfigFromOptionsExpandsBareRepoNames`                          |
| Skip groups carry one full example reason; scan failures log at WARN; `harvest.ReasonScanFailed` const                                              | `cmd/tq/main.go`, `internal/harvest/harvest.go`    | `TestGroupedSkipsKeepsFullExample`, `TestGroupedSkipsTruncatesLongExamples` |
| Pool unit sets an agent-toolchain PATH (new `services.tq-agent-pool.agentPath` option: hermetic git+go + system profile + per-user profile + GOBIN) | `deploy/nixos/tq-agent-pool.nix`                   | `nix flake check --all-systems --no-build` (module-eval)                    |

Also this session (unrelated carry-over): the orphan `/tmp/tq serve :8090`
(2026-09-07 dogfood leftover, `--allow-writes`) was stopped per the
documented cutover; the repo-root `tasks.db` is now a retired journal.

## c) Dogfood proof (the "effectively" part)

Bounded run — `TQ_DB=/tmp/tq-dogfood.db tq agent-pool --repos go-taskqueue
--yolo --review --once --max-per-tick 1 --allow-dirty` from cwd `/tmp`
(bare name + foreign cwd exercises the fix exactly):

1. Harvest enqueued ONE item (dead-export audit script); skip log showed
   the new `example=` fields live (`blocked count=8`, `paced count=14`).
2. A GLM-5.3-Flash agent (`crush run`, repo `.crushrc` carries model +
   xhigh effort) executed the full prompt contract for ~8 minutes.
3. Verify gate passed: the task's `verify_tail` shows the entire race
   suite green.
4. Result: `scripts/check-dead-exports.sh` created (advisory,
   `--strict` to gate), wired into `ci-local.sh`, AGENTS.md note fixed,
   TODO item ticked with an honest DONE annotation, commit `1586ed5`
   carrying `Task-Queue-ID: 000001a0888b9f367676ffb26ebe48f49571`.
5. Review agent (second Flash run) verdict: **approve** — "spot-checks
   confirm no live symbol is wrongly flagged".
6. Pool drained and exited cleanly (`--once`).

## d) Effectiveness model (what makes Flash work well here)

- **Rails per repo**: `.crushrc` pins model+effort (a pool `--model` would
  reset reasoning effort — known trap); `.tq-verify` is the mechanical
  quality gate; the prompt contract bounds scope and forces the TODO loop
  closure + footer cross-reference.
- **Quality loop**: `--review` gives every completion a second Flash
  opinion; `--review-autofix` turns request_changes into fix tasks; the
  budget guard bounds the fix loop.
- **Self-feeding**: `--status-every N` mints done-prompt status reports
  whose TODO appends are the next harvest's food.
- **Pacing**: project-exclusive = one agent per repo (single working tree
  makes this a correctness requirement, not a throttle); concurrency
  across repos; `daily-budget` is the real spend cap (Flash at
  $0.15/$0.5 per M makes 30/day conservative).

Proposed SystemNix poolSettings (uncommitted diff in
`~/projects/SystemNix/modules/nixos/services/tq-agent-pool.nix`, owner
review): `concurrency 3`, `"review-autofix" = "true"`, `"status-every" =
"5"`.

## e) Owner handoff (the one thing an agent cannot do)

```
# SystemNix: flip the go-taskqueue input to master, then
nix run .#deploy    # sudo, evo-x2
```

The running 0.1.0 pool keeps harmlessly `scan failed`-ing until then.
After redeploy: `journalctl -u tq-agent-pool -f` should show real
`harvest: enqueued` lines within one interval; `tq top` against
`/mnt/pool/services/tq/tq.db` should start moving. The two backend tags
(`internal/queue/{sqlite,postgres}/v0.2.0`) remain unpushed (separate
BLOCKED item).

> **CORRECTION (02:00 self-review):** the working chain is push (owner) →
> flip input → `nix run .#deploy` — as written above, "flip to
> `ref=master`" silently assumes master was pushed; at report time the
> fixes were local-only (origin/master = 0637d63).

## f) Gates

- `go build/vet` + full root race suite: green (including the parallel
  session's `go-retry` adoption in `internal/executor`, which needed
  `go mod tidy` propagation to root and `internal/worker`).
- 7/7 internal modules: GOWORK=off build+vet+test green.
- `nix flake check --all-systems --no-build`: green; `nix build .#default`:
  green after the vendorHash dance (`sha256-AYpY…`); binary reports
  `tq 0.2.0`.
- `check-go-mods.sh`: green.
- Full `./scripts/ci-local.sh` on the final tree: run at session close
  (result recorded in the commit that carries this report if anything
  regressed).

> **UPDATE (02:00):** ci-local on the final tree: **ALL GATES GREEN**
> (race suite, smokes incl. the agent's new dead-exports check, all five
> flake checks). Post-report gap also closed: an `env -i` dry-run under a
> service-like restricted PATH from a foreign cwd probes crush v0.92.0
> and scans cleanly — the agentPath composition works.

## g) Parallel-session notes

- A concurrent session migrated `execWithTransientRetry` onto
  `github.com/larsartmann/go-retry` (executor module dep; sound, kept).
- The dogfood agent's own work landed interleaved with this session's —
  the auto-commit daemon folded both; `scripts/check-dead-exports.sh` and
  its ci-local wiring are the agent's, not mine.
