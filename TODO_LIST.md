# TODO List

Short- and mid-term actionable work. Long-term direction lives in ROADMAP.md.

**This file is machine-consumed**: `tq harvest` turns every unchecked item
below into an agent task. Keep the `- [ ]` checkbox format, one item per
line — do not convert to tables. Mark done items `[x]` or delete them
(the safer default is `[x]`: deletion removes the item from
`tq harvest --prune-stale`'s match surface — see the caveat in AGENTS.md);
appending `— BLOCKED: <reason>` keeps an item out of the pool.

Items carry their evidence: a code path and/or the status report that filed
them (`docs/status/<date>_<slug>.md`). Completed items live in CHANGELOG.md,
not here.

## High Impact

- [x] `--prune-stale` zombie-matching gap: absent item text is now stale too — pending harvested tasks (payload dedup key, `catchup:` prefixes stripped) whose item is gone from the file are cancelled with a key-carrying reason; external tasks untouched. Policy decided + implemented + tested 2026-09-09 (`internal/harvest/prune.go`, round-10 session)
- [x] Pin the cooperative-cancel finalize contract in a test: wrapped `context.Canceled` through `runAgent` finalizes as Cancelled, never Failed, no `task.failed` fact, attempts unburned (`TestCooperativeCancelWrappedErrorFinalizesAsCancelled`, 01:48 report d7/e5)
- [x] `tq bootstrap` parity check against the hand-rolled sibling-repo rails — fleet switched to the managed-block form (one commit per repo), bootstrap dry-run now reports truthful change state (22:42 §d1/f2, 01:35 §c1/f1)
- [x] SECURITY.md: the `--allow-writes` blast radius, token/CSRF/loopback matrix, defense layers, residual risks (01:35 §c5/f6)
- [x] `tq harvest --prune-stale` sweep inside `agent-pool` startup — synchronous, before any actor (the worker's first claim would race an in-actor sweep), `--prune-stale=false` opts out; e2e pins zero-zombie relaunches (01:48 f11)
- [x] Rate-limit the two write endpoints: 3 failed CSRF tokens → 60s lockout (429 before CSRF, reads unlimited, success resets), smoke-asserted (01:35 f29)

## Medium Impact

- [x] `Filter.Since` SQL pushdown in both stores (`pgWhere` extracted, CountTasks shares it); `tq tasks --since` rides it (01:48 f3)
- [x] `journal_head` in `tq stats --json`; budget line labeled `(all projects)` when the table is scoped (01:48 f4/f5)
- [x] Pool `MkdirAll` on `--log-dir` at startup (23:10 e5/f4)
- [x] Dirty-tree requeue backoff ladder (base×2ⁿ capped 15m, ±20% jitter) + per-task refusal log rate-limit (23:10 e7/f7)
- [x] `tq facts --json` + `--detail` non-truncating rendering (23:10 e8/f6)
- [x] Retention keys (`log-dir-max-age`/`-max-bytes`) in `renderPoolConfig` + bootstrap flags + NixOS docs; `--repo-interval`/`--dlq-backoff` passthrough too (01:48 f7/f8, 23:10 e9)
- [x] `status-sweeper` consumer never starts when `--status-every 0` (verified); `tq watermarks show` labels current cursors and explains that lagging may mean off (23:10 e11/f27)
- [x] Structured evidence on `task.requeued` facts: `RequeueEvidence{reason, retry_in_ms}` in both stores; ONE `FailureEvidence.Tail` constant (`EvidenceTailBytes`) (01:48 b3/f2/f23)

## Web UI

- [x] Status-color vocabulary: ONE `statusColorTable` produces badge + accent (00:21 e5/f5, `internal/webui/components.go`)
- [x] Shared empty-state templ helper for TaskTable + Board (00:21 e4/f4)
- [x] Serve banner text shared const (`webui.BannerPrefix`…) used by the CLI and the e2e parser (00:21 e3/f21)
- [x] `checks.module-eval` repaired: argv-correct extraArgs example, unknown-poolSettings-key survival branch (via the new `renderedConfigFile` option), `authTokenFile`→`EnvironmentFile` assertion (23:48 e5/f16-f19)

## Fleet / deploy

- [ ] SystemNix host-side deploy + round-9 cutover (sudo-gated, user-run): `nix run .#deploy` on evo-x2 starts tq-agent-pool/tq-serve/tq-storage-dir/tq-bootstrap (all wiring shipped 2026-09-08: rev-pinned git+file input 7890cc9, house module `modules/nixos/services/tq-agent-pool.nix`, ports.tq 8100, tq.home.lan vHost + DNS, Gatus checks, btrbk-pool subvolume, dedicated sops bridge template, post-deploy smoke); then the one-time manual-pool cutover in SystemNix `docs/services/tq.md` (stop the `/tmp/tq` processes — DONE 2026-09-10, optionally copy the dogfood tasks.db onto the pool journal). The origin push is done — flip the SystemNix input to `github:LarsArtmann/go-taskqueue?ref=master` — 2026-09-10: the deployed pool was found DEAD (every tick skipped all repos: bare repo names resolved via service cwd + no git/go/crush on the service PATH + silent scan-fail logging); all three fixed on master, so the input flip + redeploy is what brings the Flash pool to life — BLOCKED: owner-run (sudo on evo-x2 — an agent cannot execute this; pool must not pick it up)

## Owner-blocked decisions (BLOCKED items are skipped by the pool)

- [ ] Push authorization (updated 2026-09-10): master + root v0.1.0/v0.2.0 + the five round-1 module tags are now on the remote (owner pushed); still latent: the two backend tags `internal/queue/{sqlite,postgres}/v0.2.0` — after their push run the per-module proxy check (`go list -m -versions`) + the real clean-room `go install` (docs/status/2026-09-09_23-47_round2-queue-backend-split.md f1/f13); post-split executor/queue content waits for the next release's module tags (published v0.2.0 tags are immutable; restored after a mistaken local re-cut) — BLOCKED: owner push authorization (never push without it)
- [ ] `internal/consumer`: wire into `tq serve` journal tailing or delete the ghost package (ADR-0009 dispatcher, zero production importers; write the ADR outcome either way) (23:47 f2/g1) — BLOCKED: owner intent call
- [ ] Decide the three near-identical interfaces (`consumer.Source`, `budget.FactSource`, `papdashboard.FactSource`) — one interface or documented duplication (23:47 f4) — BLOCKED: owner architecture decision
- [ ] Postgres CLI store wiring (`--store postgres://…` on worker/serve/agent-pool): gives queue/postgres its consumer; root re-adds pgx; sequence BEFORE any public-API promotion (23:47 f5/g3) — BLOCKED: owner release-timing call (v0.3?)
- [ ] Dependabot/renovate policy for the 8-module tree (23:47 f33) — BLOCKED: owner policy decision
- [ ] After the next real release: write the "first multi-module release" retrospective into docs/release/ (23:47 f50) — BLOCKED: needs the release to happen first

## Post-split follow-ups (harvested from docs/status/2026-09-09_23-47, verified 2026-09-10)

- [x] Dead-export audit script as a periodic check: exported-with-zero-importers detector (substring matching, NOT `rg -w` — it misses suffixed references like NewSink/NewCommandExecutor and undercounts); park it next to scripts/check-go-mods.sh (23:47 f19; manual re-derivation lives in the 2026-09-10 session) — DONE 2026-09-10: scripts/check-dead-exports.sh (advisory report, `--strict` to gate), wired into ci-local.sh
- [ ] govulncheck step in CI (non-blocking job first; needs network on the runner) (23:47 f20)
- [ ] gosec advisory scan pass over the modules (23:47 f21)
- [ ] templ-components deep-dive audit: adoption table says v1.16 — verify we use the current component APIs and none of the retired ones (23:47 f22; internal/webui/fragments.templ)
- [ ] webui dedup deep pass: render path 728 + handlers 527 LOC have grown similar branches (23:47 f23; internal/webui/render.go, handlers.go)
- [ ] httpapi/webui API-surface split-brain check: overlapping handler + route definitions between `tq api` and `tq serve` (23:47 f24)
- [ ] Full-core example: worker + executor + queue proving the embed story, including the sqlite-vs-postgres backend-choice import (23:47 f25; examples/)
- [ ] Document the round-2 multi-module release flow as docs/release/RELEASE.md (sub-tag cutting, sibling-replace allowlist, two-phase --tag/--push; scripts/release.sh owns the code) (23:47 f26)
- [ ] Version-surface inventory doc: flake.nix version attr + ldflags + root tag + per-module tags + CHANGELOG — who must move when (23:47 f27; the flake checks.version-sync check covers attr↔binary, not the rest)
- [ ] Lint-baseline slice-triage: wrapcheck findings (~50) first, then varnamelen (~50) — shrink the ~400 advisory baseline, never mass-fix (23:47 f28/f29; .golangci.yml)
- [ ] `ExitCause` → `ExitError` rename consideration (errname finding; exported rename needs a v0.3 window) (23:47 f30; internal/executor/agent.go)
- [ ] Per-module golangci runs in .github/workflows/ci.yml (ci-local.sh already loops modules; ci.yml runs root only) (23:47 f32)
- [ ] Multi-repo smoke against the nix-built 0.2.0 binary (`TQ_BIN=result/bin/tq ./scripts/smoke/multi-repo.sh`) (23:47 f35)
- [ ] Measure CI time impact of the disk-derived `-count=1` module loops; tune if it dominates the job (23:47 f49; .github/workflows/ci.yml)
- [ ] Dogfood hygiene: stale queued tasks in the live pool DB may verify with the OLD single-module default command — drain or cancel them (`tq tasks`, `tq cancel`) (23:47 f46)

- [x] Cut v0.2.0: full ci-local gate + nix-binary smoke green, annotated tag v0.2.0 pushed, module proxy verified, clean-room go get verified, GitHub Release published (also fixed a release.sh bug: the awk section cut matched the bare heading, not the dated one) — 2026-09-09 `v0.2.0`, module proxy + GitHub release + smokes (19:49 f2; v0.1.0 shipped 2026-09-06)
- [x] Kill the stray `/tmp/papdbg` worker (PID 1039418) — killed 2026-09-09 (second-signal force-exit; audit had verified it self-contained and inert)
- [x] `--status-every` enablement: N=20 chosen + documented in the `deploy/systemd` sample's recommended pool.conf keys (cost/verbosity balance; the live smoke shipped 2026-09-08) (20:56 g1/f17)
- [x] Status-report review ceiling: DECIDED (default policy 2026-09-09): the mechanical file-exists + verify contract stays the ceiling — reports are exempt from review minting like reviews themselves; loop safety stays structural (sweeper exempts the type) + budgetary (every enqueue caps) (20:56 g2)
- [x] Hard mechanical cap on status-agent TODO_LIST appends: DECIDED (default policy 2026-09-09): prompt-level caps + the budget guard remain the ceiling — a diff-parsing hard cap would reject legitimate multi-item reports and add a fragile parser; blast radius is bounded by `--daily-budget`/`--max-per-tick` on every minted item (20:56 g3)
- [x] `tq cancel` dedup-key release: DECIDED (default policy 2026-09-09): keys stay suppressive after cancel — releasing them would let a stale harvest re-arm withdrawn work; the documented escape hatch (edit the item text → new key) is deliberate and now also re-arms correctly since absent items are pruned
- [ ] Verify the CQA bridge against a live CQA API instance and fix contract drift (`internal/bridge/cqa` response shapes are httptest-informed guesses today); upgrade its FEATURES.md status after (plan C25) — BLOCKED: needs a live CQA instance URL + owner ID + token from the owner
