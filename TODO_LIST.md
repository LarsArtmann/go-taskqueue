# TODO List

Short- and mid-term actionable work. Long-term direction lives in ROADMAP.md.

**This file is machine-consumed**: `tq harvest` turns every unchecked item
below into an agent task. Keep the `- [ ]` checkbox format, one item per
line — do not convert to tables. Mark done items `[x]` or delete them;
appending `— BLOCKED: <reason>` keeps an item out of the pool.

Items carry their evidence: a code path and/or the status report that filed
them (`docs/status/<date>_<slug>.md`). Completed items live in CHANGELOG.md,
not here.

## High Impact

- [ ] Pin the cooperative-cancel finalize contract in a test: an agent run cancelled mid-flight (`context.Canceled` through `runAgent`) must finalize the task as Cancelled, never Failed — the `errors.Is` matching is a load-bearing wrapping contract that a 2026-09-09 session nearly broke silently with zero test coverage (01:48 report d7/e5, `internal/worker`)
- [ ] `tq bootstrap` parity check against the hand-rolled sibling-repo rails (`.crushrc`/`.tq-verify` shape, payload-pinned verify) — reconcile or switch the fleet to bootstrap; carried over THREE reports now (22:42 §d1/f2, 01:35 §c1/f1) and still first-ranked
- [ ] SECURITY.md: document the `--allow-writes` blast radius (UI-originated cancels/rescues, CSRF cookie model, token-vs-loopback matrix) — the admin write layer shipped without its security-doc update (01:35 report §c5/f6)
- [ ] `tq harvest --prune-stale` sweep inside `agent-pool` startup (one pass before the first tick) so relaunches are self-cleaning instead of inheriting zombies (01:48 report f11; complements the shipped CLI flag, `internal/harvest/prune.go`)
- [ ] Rate-limit the two write endpoints (e.g. 3 failed CSRF tokens → short lockout) — cheap brute-force hygiene now that `POST /task/{id}/cancel|rescue` exist (01:35 report f29)

## Medium Impact

- [ ] `Filter.Since time.Time` SQL pushdown in both stores; `tq tasks --since` uses it instead of the CLI-side post-filter (01:48 report f3, `internal/queue`)
- [ ] Surface `journal_head` in `tq stats --json` (parity with the text output) (01:48 report f4, `cmd/tq`)
- [ ] Pool `MkdirAll` on the `--log-dir` at startup — first sidecar sweep WARNs (and a sidecar write could fail) when the dir does not exist yet (23:10 report e5/f4, `internal/executor`)
- [ ] Dirty-tree requeue backoff/jitter + log rate-limit: under a sustained-dirty tree (long interactive session) preflight requeues bounce every poll (23:10 report e7/f7, `internal/worker`)
- [ ] `tq facts --json` + non-truncating detail rendering (multi-line verify tails are inlined and truncated in table output — hard to grep) (23:10 report e8/f6, `cmd/tq`)
- [ ] Retention keys in generated configs: `renderPoolConfig` (`tq bootstrap --install`) and the NixOS module's pool.conf renderer emit a fixed key set predating `--log-dir-max-age`/`--log-dir-max-bytes` (01:48 report c5/f7-f8)
- [ ] Suppress the `status-sweeper` consumer startup when `--status-every 0` — its watermark lags forever (12+) in `tq watermarks show` when the loop is off (23:10 report e11/f27, `internal/status`)
- [ ] `tq bootstrap`: `--repo-interval`/`--dlq-backoff` passthrough flags (parity with `agent-pool` for fine-grained windows) or document agent-pool-direct as the supported path (23:10 report e9/f12, `cmd/tq/bootstrap.go`)
- [ ] Structured evidence on `task.requeued` facts (preflight refusals carry a plain error string; make it a structured detail like `FailureEvidence`) (01:48 report b3/f2, `internal/queue`)
- [ ] Pin ONE `FailureEvidence.Tail` size constant — agent/verify/command tails each hardcode 4096 independently (01:48 report f23, `internal/executor`)

## Web UI

- [ ] Consolidate the status-color vocabulary: `statusBadgeType` (badge language) and `statusAccentClass` (border/board language) are two hand-synced maps — one table producing both outputs (00:21 report e5/f5, `internal/webui/components.go`)
- [ ] Extract the shared empty-state copy ("queue is empty" / "no tasks match" exist in TaskTable AND Board) into one templ helper (00:21 report e4/f4, `internal/webui/fragments.templ`)
- [ ] Serve banner text → shared const used by `cmd/tq` and the e2e banner-parse test (brittle string contract, nearly broke twice) (00:21 report e3/f21)
- [ ] Fix the broken-argv `extraArgs` example in `checks.module-eval` (`"--max-per-tick" "3"` — a copyable footgun) and add the unknown-poolSettings-key negative branch (23:48 report e5/f16/f18, `flake.nix`)

## Fleet / deploy

- [ ] SystemNix host-side deploy + round-9 cutover (sudo-gated, user-run): `nix run .#deploy` on evo-x2 starts tq-agent-pool/tq-serve/tq-storage-dir/tq-bootstrap (all wiring shipped 2026-09-08: rev-pinned git+file input 7890cc9, house module `modules/nixos/services/tq-agent-pool.nix`, ports.tq 8100, tq.home.lan vHost + DNS, Gatus checks, btrbk-pool subvolume, dedicated sops bridge template, post-deploy smoke); then the one-time manual-pool cutover in SystemNix `docs/services/tq.md` (stop the `/tmp/tq` processes, optionally copy the dogfood tasks.db onto the pool journal). After the origin push: flip the SystemNix input to `github:LarsArtmann/go-taskqueue?ref=master`

## Owner-blocked decisions (BLOCKED items are skipped by the pool)

- [ ] Cut v0.2.0: finalize the CHANGELOG `[Unreleased]`, release checklist, annotated tag, GitHub release, then run the nix-built binary as a smoke — additive changes only (web UI, pool hardening), fine for 0.x (19:49 report f2; v0.1.0 shipped 2026-09-06) — BLOCKED: owner go/no-go
- [ ] Verify the CQA bridge against a live CQA API instance and fix contract drift (`internal/bridge/cqa` response shapes are httptest-informed guesses today); upgrade its FEATURES.md status after (plan C25) — BLOCKED: needs a live CQA instance URL + owner ID + token from the owner
- [ ] Should status reports be reviewed too, or is the mechanical file-exists contract the right ceiling? (loop guard currently exempts the `status` type, like reviews) (20:56 report g2) — BLOCKED: owner trust-policy decision
- [ ] Hard mechanical cap on status-agent TODO_LIST.md appends (parse the diff, refuse runaway item counts) vs prompt-level caps + budget guard (20:56 report g3) — BLOCKED: owner blast-radius preference
- [ ] Enable `--status-every N` in the ROUND4 dogfood launch command and `deploy/systemd` sample once the live smoke passes (20:56 report g1/f17) — BLOCKED: owner picks N (cost/verbosity tradeoff) and go/no-go after the live smoke
- [ ] Push master to origin (~30+ commits ahead of origin; every session since 21:40 has re-asked) — BLOCKED: owner push go/no-go
- [ ] Decide the fate of the stray `/tmp/papdbg/tq worker --alert-url http://127.0.0.1:18100` (PID 1039418, running since 18:31) — AUDIT EVIDENCE 2026-09-09: fully self-contained (own `/tmp/papdbg/tasks.db`, head still seq 4, :18100 is a local `stub.py` dashboard, forwards nothing) — safe to kill; killing changes nothing for the repo queue — BLOCKED: owner call on a live process
- [ ] Decide whether `tq cancel` should release a task's dedup key so an un-checked TODO item re-arms (today a cancelled task's key suppresses re-enqueue forever unless the item text changes; AGENTS.md documents text-editing as the escape hatch) — BLOCKED: owner policy decision on cancel semantics
