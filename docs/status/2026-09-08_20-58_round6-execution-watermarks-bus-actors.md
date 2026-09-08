# Status Report — ROUND6 Execution Session (the whole TODO list)

**Date:** 2026-09-08 20:58 CEST
**Session type:** EXECUTION. Owner instruction: "NOW GET SHIT DONE! The
WHOLE TODO LIST! … DO NOT STOP UNTIL THE ENTIRE LIST IS FINISHED and
VERIFIED."
**Plan executed:** `docs/planning/2026-09-08_07-36_SUPERB-PLAN-ROUND6-WATERMARKS-REVIEWS-AND-THE-BUS.md`
(P1–P21 + verified pre-done P17/P18), against a repo that had moved
~80 commits under the plan (round-5 M1–M27, Postgres store, write API,
status-executor WIP by a concurrent agent).

**Gates held throughout:** every slice ended `go build ./... && go vet ./...
&& go test <affected> -race -count=1` green; Postgres conformance re-run
against a fresh ephemeral postgres:16 container; webui + papdashboard e2e
smokes green; one full `go test ./... -race` pass green (17 packages).
The final `./scripts/ci-local.sh` on the session's last tree has NOT run
yet (see b/partial).

---

## a) FULLY DONE (code-verified this session)

### Tier 1 — persisted watermarks (the 1% → 51% correctness gap)

1. **P1 — watermarks storage layer.** `watermarks` side table (consumer
   PK/seq/updated_at) in both stores; `Watermark`/`SaveWatermark` on
   `queue.Store` as a monotonic upsert (SQLite `WHERE seq < excluded`,
   PG `GREATEST`); admin `ListWatermarks`/`SetWatermark` on the concrete
   stores (deliberately off the interface — forced rewind is ops-only).
   Tests: absent→0/false, roundtrip, monotonic guard, zero-cursor
   existence, legacy-DB migration, PG conformance (TRUNCATE isolation
   includes the table).
2. **P2 — bridge resume-from-checkpoint.** `WatermarkStore` port injected
   into `papdashboard.New` (nil = legacy volatile); 3-branch
   `startWatermark` (`FromSeq > persisted > head`) with the branch named
   in the startup log; first run EAGERLY persists head (crash before the
   first batch resumes exactly there; never replays history); batch-end
   checkpoint AFTER the last accepted fact; a pending checkpoint GATES
   further forwarding; checkpoint failure = forward failure. Package doc
   rewritten (at-least-once across restarts, idempotent re-sends).
3. **P3 — the alerted map is dead.** Alert-resolve correlation is derived
   from the task's own fact trail (`FactsForTask` read on `FactSource`):
   a completion closes the alert of any task that ever dead-lettered,
   even when the dead-letter predates the process. The map and its
   bookkeeping are deleted.
4. **P4 — restart test battery** (`restart_test.go`): mid-batch crash
   loses zero facts with bit-identical idempotency keys; failing
   checkpoint store blocks later batches until healed (never
   at-most-once); `FromSeq` beats the persisted checkpoint + log names
   the branch; first run bootstraps at head (no history replay);
   resolve-after-restart closes a pre-restart alert. All 3×-repeated
   under `-race`.
5. **P5 — `tq watermarks show|set`.** Table output with per-consumer lag
   (`head − cursor`); `set` is the rewind hatch (bypasses the monotonic
   guard deliberately; replay is idempotent). AGENTS.md gained the
   watermarks contract section; DOMAIN_LANGUAGE gained
   watermark/consumer-key/checkpoint/lag.
6. **P6 — review sweeper catch-up.** Sweeper cursor persisted under
   consumer `review-sweeper` (same rules: after-page checkpoint, pending
   gate, dedup-safe replay). `TestSweepCatchesUpAcrossRestarts` pins that
   completions while no pool ran still get reviewed.
7. **Watermark EXISTENCE semantics** (bug the battery caught): seq 0 is a
   real cursor ("consumed nothing yet"), not "no row" —
   `Watermark(...) (seq, exists, err)` across both stores. Without it, a
   consumer bootstrapped on an empty journal silently skipped every fact
   that arrived before its first sweep.

### Tier 2 — reviews visible

8. **P7 — webui review verdicts.** Completed review tasks render an
   approve/request-changes badge next to status in the dashboard table
   and an "agent review" card (summary + findings with severity badges)
   on the detail page; parsed best-effort from the completion-fact
   detail; quiet when absent. `templ fmt` + regenerated `*_templ.go` +
   `nix run .#webui-css` + webui smoke green.

### Tier 3 — the bus + structural lifecycle

9. **P10 — ADR-0009 (journal subscription policy).** Subscriber classes
   (exact = at-least-once + block; signal = hub, drop/coalesce — not
   configurable, structural); `journal.Journal` demoted to test double;
   dispatcher is a new seam over Store bounded reads (NOT on `Store`);
   v1 wake = poll, v2 = notify-after-commit outside the mutation tx;
   compaction resync must be loud (keyed on the retention floor, never
   seq-gap detection).
10. **P21 — compaction caveat codified.** The loud-resync rule written
    into ADR-0006's consequences; inventory §3.4 marked resolved.
11. **P11 — dispatcher phase 1 (`internal/consumer`).** Paging fan-out
    over `Facts(since, limit)` with the bridge's loop shape; per-fact
    cursor advances only after the handler accepts; a failing handler
    pauses only that subscriber (per-class backpressure); `Lag()`,
    `Cursor()`, unsubscribe. Tests pin ordering, a 3.5-page burst,
    flaky-handler isolation with at-least-once redelivery, unsubscribe,
    lag. The race detector caught a real cursor data race — fixed with
    per-subscriber locking.
12. **P12 — lag observability.** `Dispatcher.Lag` + a once-a-minute lag
    log; `tq stats` renders every persisted consumer cursor with its lag
    behind the head.
13. **P16 — SSE reconnect-lag log.** A stream request with a stale
    `Last-Event-ID` logs `head − id` (still served a full snapshot —
    ADR-0003 unchanged). Test pins log + snapshot.
14. **P13 — `internal/runactor` (actor pilot).** Stdlib+errgroup
    run.Group shape: named actors, first exit decides (error = cause;
    clean = graceful stop), LIFO teardown, interrupt actor (second signal
    → exit 130), `ExecutionScope` (shutdown-uncancellable, timeout-
    bounded). `tq serve` rewired (defer-Close → ordered teardown).
15. **P14 — actor rollout.** `worker` and `agent-pool` compose through
    runactor: alert bridge, tick loop, and once-drain watcher are named
    actors; store teardown registered FIRST so it closes LAST (bridge
    checkpoints always land before the DB closes). Deliberate behavior
    change: a bridge that cannot start now fails the command (exit 1)
    instead of silently stopping it with exit 0.
16. **P15 — shutdown-ordering e2e (real binaries).** SIGTERM → the
    claimed sleeper COMPLETES (execution scope survives), the bridge's
    persisted watermark covers the dead-letter fact it forwarded
    (checkpoint landed before store close), worker exits 0; serve: SSE
    stream closes, exit 0. Invariant-sensitive work owned start-to-finish
    by this session (not pooled), per the plan's §8.

### Small debt

17. **P19 — dprint decision (formal: manual).** Trial measured 26 files /
    ~815 lines one-time churn; wiring into treefmt is impossible today
    (go-standard's treefmt has no dprint program; dprint.json's remote
    wasm plugins cannot be fetched in the `nix flake check` sandbox) and
    operationally unwise (multiple concurrent agents write
    docs/status nonstop — a hard docs gate would red-master). Decision
    recorded in AGENTS.md; dprint stays in the devShell as an on-demand
    tool.
18. **P20 — sidecar retention.** `executor.SweepSidecars` (age-based,
    *.log-only, best-effort); `tq agent-pool --log-dir-max-age` (env
    `TQ_LOG_DIR_MAX_AGE`, 0 = off) sweeping each tick; `--log-dir` help
    now warns that sidecars are PLAINTEXT and may contain repo paths.
    Aged/live/foreign-file tests.
19. **P17/P18 — verified pre-done** (no code needed): TestCheckProjectsDir
    table tests exist; the agent `--version` probe + missing-binary
    warning ships. TODO_LIST rows to be ticked.

### Incidental fixes found on the way

20. `scripts/smoke/papdashboard-e2e.sh` fixed-port collision: port 18099
    is held by the operator's REAL pap-raw-server — the stub silently
    failed to bind and the smoke asserted against the wrong dashboard.
    Now derives an ephemeral port + stub-liveness check
    (`PAP_SMOKE_PORT` override).
21. go-sse `EventID.String()` returns a branded debug format
    (`SSEEvent:1`), not the wire value — the reconnect-lag log now uses
    `Get()` (probe-verified outside the repo).

## b) PARTIALLY DONE

- **Closing docs/chores for this session's ships** (next actions, ~30min):
  TODO_LIST rows for P6/P7/P10–P15/P17–P20 not yet ticked with evidence;
  FEATURES.md/CHANGELOG.md rows not yet written; AGENTS.md package map
  lacks `internal/consumer` + `internal/runactor`.
- **Final `./scripts/ci-local.sh`** on the session's last tree not yet
  run (last full-gate was mid-tier-3; the P20/AGENTS.md edits after it
  are build+test-green but nix/treefmt-gate-unverified).
- **Bridge on the dispatcher:** deliberately NOT migrated (ADR-0009
  allows it only with the battery green; left as the pilot candidate).

## c) NOT STARTED (blocked or out of scope by design)

- **P8 — cut v0.2.0** (owner go/no-go; `scripts/release.sh v0.2.0` ready).
- **P9 — CQA live verification** (owner URL/token/ID).
- **R1 daemon-mode ADR, R2 plugin-era cordis triggers, R3 v0.2 core
  arc** (Postgres CLI wiring, HTTP API hardening, consumer-group pool) —
  owner/roadmap gated, correctly not scheduled by ROUND6.
- Concurrent-agent work in flight at session end: the `status` executor
  (`internal/status`, `--status-every`, AGENTS.md rows appeared
  mid-session) — theirs, untouched by me.

## d) TOTALLY FUCKED UP (honest ledger)

1. **Three botched multiedits** (truncated `fakeSource.Get`; duplicated
   `fail()` tail; dropped the enqueue line in postgres_test) — each
   caught immediately by compiler/vet, each repaired, but the pattern
   (old_string surgery on unread-exact regions) was sloppy.
2. **Existence-vs-zero-value designed late.** The first watermark
   implementation conflated "row with seq 0" and "no row"; the sweeper
   catch-up test exposed it (`Facts:0`); the fix churned the Store
   signature across both stores, the bridge, the sweeper, and six test
   call sites. Should have been in the data model from the start (the
   AGENTS data-models-first rule, violated by speed).
3. **Two test-semantics reworks:** (a) the mid-stream test asserted
   persisted==100 right after cancel — raced the drain's checkpoint and
   mis-modeled that the tick-gate legitimately persists the ACCEPTED
   PREFIX after a forward failure (correct at-least-once behavior);
   restructured with a long-poll bridge and crash-before-next-tick.
   (b) the final-watermark assertion raced cancel; now waits for the
   checkpoint before cancelling.
4. **Sloppy first draft of shutdown_test.go** (junk helper stubs,
   `t.Context` misuse, a fake serveAddr) — fully rewritten before it
   ever ran; the rewrite is the committed version.
5. **Probe litter + a dead-end copy trial:** an out-of-repo probe module
   cracked the go-sse question (kept, then cleaned); the dprint trial's
   first `cp -r` diff attempt failed silently before the proper
   `git worktree` measurement.
6. **One transient `TestRenderPoolConfig` failure** in a full-suite run
   (passes standalone and on 2× re-run). Most plausibly the concurrent
   agent mid-editing `poolconfig_test.go` at that moment; root cause NOT
   verified — flagged, not buried.

## e) WHAT WE SHOULD IMPROVE

- **Model edge-value semantics (exists/zero) before writing SQL.**
- **Write test timelines (who-waits-for-what) before orchestrating
  multi-goroutine assertions** — most of this session's reworks were
  assertion-vs-race mismatches, not product bugs.
- **Re-run ci-local before ANY "tier done" claim** — I let the gate lag
  behind the last two slices.
- The editor's ~400 advisory lint findings were correctly ignored per
  policy; the new packages add a few (varnamelen `f`, nonamedreturns) —
  same class as baseline.

## f) NEXT — up to 50 (ordered by leverage; ⭐ = ready now)

1. ⭐ Run `./scripts/ci-local.sh` on the final tree (the session gate).
2. ⭐ Tick TODO_LIST rows (P6, P7, P10–P15, P17, P18, P19, P20) with
   evidence pointers.
3. ⭐ FEATURES.md + CHANGELOG.md rows for watermarks/verdicts/bus/actors.
4. ⭐ AGENTS.md package map: `internal/consumer`, `internal/runactor`.
5. ⭐ DOMAIN_LANGUAGE: dispatcher / exact consumer / signal consumer.
6. Verify + integrate the concurrent agent's `status` executor ship.
7. P8: cut v0.2.0 (owner gate) — `scripts/release.sh v0.2.0`.
8. Push authorization: master is ~30 commits ahead of origin.
9. P9: CQA live dry-run (owner creds).
10. Migrate the papdashboard bridge onto `consumer.Subscribe` (pilot;
    battery must stay green).
11. Notify-after-commit hook (v2 wake) for the dispatcher — outside the
    mutation tx.
12. Webui lag card (persisted consumer lag on the dashboard).
13. `tq stats --json` parity incl. consumer lag.
14. Round5 defect d1: webui-screenshots.sh detail-page URL builds from a
    JSON object (garbage URL) — never executed.
15. Round5 d2: check-webui-css.sh references a ghost nix app
    (`webui-css-drift-check`).
16. Round5 d3: FilterBar form GET drops an active `?sort=` (hidden input).
17. Round5 d4: budget telemetry undercounts after bridge restart (seed
    from CountFacts).
18. Round5 d5: journal-browser "load older" actually pages newer.
19. Round5 d6: data-age client/server fmtAge parity test.
20. Round5 d7: delete PostgresStore.migrateOnOpenFail dead field.
21. Round5 d8: first-push verification of test-windows/test-postgres jobs.
22. Round5 d9: cmdAPI CLI wiring tests.
23. Link reviewed-agent task ↔ its review task in webui detail pages.
24. `tq doctor`: watermark sanity (cursor > head → warn).
25. Review-verdict badge on the REVIEWED agent task row (not just the
    review task's own row).
26. Dispatcher slow-consumer soak test (sustained burst + lag ceiling).
27. Enforcement for the journal.Journal demotion (non-test import guard).
28. `--store postgres` CLI wiring (round5 leftover).
29. `tq journal compact` CLI command (ADR-0006 prototype exists).
30. Fuzz `unwrapCommand` (round5 M26 leftover).
31. Nightly `-race -count=3` job.
32. govulncheck + dependabot; gosec triage.
33. Secrets-in-logs test + `--redact`.
34. Shell completions; DLQ rescue preview; full `--json` sweep (M25).
35. Agent result-schema validation (M24).
36. HTTP executor auth; SDK contract; Windows process-group kill (M24).
37. UI pills; D2 diagram; website launch (M27/website flow).
38. Webui: review-verdict column sorting/filtering.
39. `tq watermarks` — flag/documentation in README quickstart.
40. Cron scheduler slice (M23 design exists).
41. Per-repo budgets slice (M23 design exists).
42. Retry policies / DAG templates / session chains / webhooks (M23).
43. `/metrics` exposition (M23).
44. Tailer onto dispatcher — keep the batch jump signal-only (ADR-0009).
45. R1 daemon-mode ADR (owner gate g1).
46. R2 plugin-era executor/bridge plugin API (ADR-0004 triggers).
47. R3 v0.2 core arc: consumer-group pool, Postgres HA story.
48. Postgres compaction design (fencing tokens when double-write lands).
49. Consider a scratch e2e for `--log-dir-max-age` end-to-end via CLI.
50. Re-check TestRenderPoolConfig flake root cause (concurrency suspect).

## g) OWNER QUESTIONS (cannot be answered from the repo)

1. **v0.2.0 go/no-go (P8):** additive-only since v0.1.0 (watermarks,
   reviews, write API, Postgres store, bus phase 1); gate green pending
   the final ci-local run. Say go and `scripts/release.sh v0.2.0` does
   the rest — or hold.
2. **Push authorization:** master sits ~30 commits ahead of origin
   (concurrent sessions + this one); push after the final gate?
3. **The dogfood agent-pool colliding with this session:** its
   auto-commits interleaved with my slices all day (and one probable
   test flake). Keep it running while I finish the closing chores, or
   stop it until the tree is quiet?
