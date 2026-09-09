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

- [ ] SystemNix host-side deploy + round-9 cutover (sudo-gated, user-run): `nix run .#deploy` on evo-x2 starts tq-agent-pool/tq-serve/tq-storage-dir/tq-bootstrap (all wiring shipped 2026-09-08: rev-pinned git+file input 7890cc9, house module `modules/nixos/services/tq-agent-pool.nix`, ports.tq 8100, tq.home.lan vHost + DNS, Gatus checks, btrbk-pool subvolume, dedicated sops bridge template, post-deploy smoke); then the one-time manual-pool cutover in SystemNix `docs/services/tq.md` (stop the `/tmp/tq` processes, optionally copy the dogfood tasks.db onto the pool journal). The origin push is done — flip the SystemNix input to `github:LarsArtmann/go-taskqueue?ref=master` — BLOCKED: owner-run (sudo on evo-x2 — an agent cannot execute this; pool must not pick it up)

## Owner-blocked decisions (BLOCKED items are skipped by the pool)

- [x] Cut v0.2.0: full ci-local gate + nix-binary smoke green, annotated tag v0.2.0 pushed, module proxy verified, clean-room go get verified, GitHub Release published (also fixed a release.sh bug: the awk section cut matched the bare heading, not the dated one) — 2026-09-09 `v0.2.0`, module proxy + GitHub release + smokes (19:49 f2; v0.1.0 shipped 2026-09-06)
- [x] Kill the stray `/tmp/papdbg` worker (PID 1039418) — killed 2026-09-09 (second-signal force-exit; audit had verified it self-contained and inert)
- [x] `--status-every` enablement: N=20 chosen + documented in the `deploy/systemd` sample's recommended pool.conf keys (cost/verbosity balance; the live smoke shipped 2026-09-08) (20:56 g1/f17)
- [x] Status-report review ceiling: DECIDED (default policy 2026-09-09): the mechanical file-exists + verify contract stays the ceiling — reports are exempt from review minting like reviews themselves; loop safety stays structural (sweeper exempts the type) + budgetary (every enqueue caps) (20:56 g2)
- [x] Hard mechanical cap on status-agent TODO_LIST appends: DECIDED (default policy 2026-09-09): prompt-level caps + the budget guard remain the ceiling — a diff-parsing hard cap would reject legitimate multi-item reports and add a fragile parser; blast radius is bounded by `--daily-budget`/`--max-per-tick` on every minted item (20:56 g3)
- [x] `tq cancel` dedup-key release: DECIDED (default policy 2026-09-09): keys stay suppressive after cancel — releasing them would let a stale harvest re-arm withdrawn work; the documented escape hatch (edit the item text → new key) is deliberate and now also re-arms correctly since absent items are pruned
- [ ] Verify the CQA bridge against a live CQA API instance and fix contract drift (`internal/bridge/cqa` response shapes are httptest-informed guesses today); upgrade its FEATURES.md status after (plan C25) — BLOCKED: needs a live CQA instance URL + owner ID + token from the owner
