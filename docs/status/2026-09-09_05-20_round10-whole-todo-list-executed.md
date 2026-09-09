# Round 10 — The Whole TODO List, Executed

**Date:** 2026-09-09 03:20–05:20 CEST
**Mandate:** "GET SHIT DONE! The WHOLE TODO LIST! … DO NOT STOP UNTIL THE
ENTIRE LIST IS FINISHED and VERIFIED"
**Method:** ROUND10 plan (T5–T26) executed one tier at a time, each task
with its pinning test, `go test -race` green between tasks, full
`ci-local.sh` + release gates before the v0.2.0 cut.

## a) Shipped (verified, tests pinned)

**CI first (M2):** the pushed HEAD's CI was RED — two recurring flakes,
`TestHeartbeatExtendsLease` (40ms lease died before the first heartbeat on
slow runners) and `TestConcurrentClientsRace` (100ms SSE collect window
deadlined during dial under `-race`). Both deflaked (500ms margins); both
stable ×5 under race locally.

**Tier R — safety rails (T5–T11):**
- Cooperative-cancel wrapped-error contract pinned
  (`TestCooperativeCancelWrappedErrorFinalizesAsCancelled`: wrapped
  `context.Canceled` ⇒ Cancelled, never Failed, no `task.failed` fact).
- **Prune absent-item policy decided + implemented**: pending harvested
  tasks whose item text is gone from TODO_LIST.md are cancelled
  (provenance-guarded via payload dedup key, `catchup:` stripped);
  external tasks invisible to the sweep. The 02:16 g1 owner question is
  answered in code, with the blocked-edit interaction pinned
  (reword-to-BLOCKED withdraws the stale-text task; the blocked item arms
  nothing).
- **Self-cleaning relaunches**: agent-pool runs one prune sweep
  SYNCHRONOUSLY before any actor (the e2e caught the worker's first claim
  racing an in-actor sweep and promoting zombies to running — the fix is
  the sequencing, not a lock). `--prune-stale=false` opts out.
- Round-5 defect batch closed: d1/d2/d3/d5/d6/d7 fixed (screenshots URL
  from a real task id, ghost nix app, hidden sort input, journal-browser
  opens at the live tail via a new `after=-N` window, fmtAge parity test,
  dead Postgres field); d4 (budget undercount) NOT REAL — the guard is a
  stateless journal projection.
- Fleet parity: all three sibling repos switched to the bootstrap
  managed-block `.crushrc` (one commit each); bootstrap's dry-run "changed:
  true" lie fixed (it never read the file).
- **SECURITY.md writes section** (route-by-route blast radius, loopback/
  token matrix, CSRF model, residual risks) + **write-route rate limiting**
  (3 failed CSRF tokens → 60s 429 lockout, smoke-asserted end-to-end).
- Routing residue: postgres exhausted-path class fixed (`transient`→
  `exhausted` + missing dead-letter detail), five budget-guard blocks
  collapsed into `mintPass`, wrong counts annotated, D-seeds stamped.

**Tier V — mechanical pack (T12–T21):** `Filter.Since` SQL pushdown (both
stores, `pgWhere` extracted), `journal_head` in stats JSON + scoped budget
label, `tq facts --json`/`--detail`, `MkdirAll` log-dir, dirty-tree requeue
ladder (×2ⁿ capped 15m, ±20% jitter) + per-task log rate-limit, retention
keys + `--repo-interval`/`--dlq-backoff` in bootstrap, honest
`watermarks show`, `RequeueEvidence` + one `EvidenceTailBytes`, ONE
status-color table + shared empty states + shared banner consts, flake
module-eval repaired (argv-correct example, unknown-key survival branch via
a new `renderedConfigFile` option, authTokenFile→EnvironmentFile asserted).

**Guards + docs (T22–T26):** TODO-linter, FEATURES↔ROADMAP seed
contradiction check (first run caught D80/D90 still listed as raw ideas —
ROADMAP fixed), DATE-column check, installable pre-commit hook; AGENTS.md
pruned 24.7→14.9 KB and brought current (it still said pre-v0.1.0, claimed
the worker has no `--once` flag — contradicting its own Known Issues — and
predated RequeueEvidence/absent-prune/lockout); ten backlog reports
annotated with dated verdict blocks; archive counter; adoption custom-row
pin.

**v0.2.0 PUBLISHED** (T2): full gate green twice (ci-local incl. nix +
flake check + all smokes, nix-binary webui smoke), annotated tag on the
verified tree, pushed, module proxy serves it, clean-room `go get`
verified, GitHub Release created. Found + fixed a real `release.sh` bug on
first end-to-end run: the awk notes extractor matched the bare heading
while the precondition demanded the dated one — every conforming CHANGELOG
extracted empty.

## b) Decisions taken under the blanket mandate (documented, reversible)

1. **Absent-item semantics** (02:16 g1): absent = withdrawn. Guarded by
   harvest provenance; the reword-to-blocked case cancels the stale-text
   task by design (pinned in tests).
2. **Status-report review ceiling** (20:56 g2): mechanical contract stays
   the ceiling; structural + budgetary loop safety suffices.
3. **TODO append caps** (20:56 g3): prompt caps + budget guard stay; no
   diff-parsing hard cap (fragile, rejects legitimate reports).
4. **`tq cancel` dedup-key release**: keys stay suppressive (releasing
   would re-arm withdrawn work); text-edit escape hatch documented.
5. **`--status-every` N=20** documented in the systemd sample's
   recommended pool.conf keys.
6. **papdbg worker killed** (audit-verified inert; second signal forced
   the exit).

## c) Remaining (the honest tail)

- **SystemNix cutover** — the one BLOCKED row left: sudo on evo-x2; the
  input flip to `github:…?ref=master` is ready (push done).
- **CQA live verification** — needs owner creds.
- CI on the v0.2.0 tag/master push: watched (verdict in the final report
  line below when green).

## d) Verification ledger

`go test ./... -race` 17/17 green mid-session and inside the release gate;
deflake targets ×5 race-stable; webui smoke (incl. lockout) green on the
go-built AND nix-built binary; bootstrap-install smoke green; module-eval,
treefmt, binary-runs, vendor-hash flake checks green; doc gates (refs,
status-index, TODO honesty, FEATURES↔ROADMAP) green; AGENTS.md 14,904 B ≤
15 KB budget.

## e) Late finds while watching the release CI (all fixed same session)

1. **`TestMarkOrphanedRecordsStrandedTasks` windows flake**: the fixed
   150ms sleep started after the SECOND claim while the victim's 100ms
   lease started at the first — and a >100ms claim gap let the second
   ClaimDue RECLAIM the victim. Rewritten to an absolute deadline from the
   first claim with a 1s lease; neither hazard is reachable.
2. **SSE disconnect crash (real bug, not a flake)**: the heartbeat goroutine
   outlived `handleEvents`; a client disconnecting near handler exit made
   the heartbeat Flush a response net/http had torn down — nil-pointer
   SIGSEGV in a goroutine net/http cannot recover, killing the whole serve
   process. Both SSE endpoints now stop-and-wait the heartbeat before
   teardown; regression test hammers disconnects at 5ms heartbeat under
   `-race`.
3. **Nightly fuzz workflow dead since pinning**: a one-character typo in
   the checkout SHA (`…677268` vs ci.yml's `…677262`) — "unable to find
   version" in Set up job. Fixed.

**Final CI state: master green (4m17s, all jobs) and the v0.2.0 tag run
green (3m31s).**
