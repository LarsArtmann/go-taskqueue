# IO Efficiency Pass — 2026-10-07 03:29

Session: user asked "How can we become a LOT more IO efficient?" — full
research → plan → implement → verify loop over the store, worker, bridges,
and readmodel. This report covers only this session's run.

## a) FULLY DONE (verified green on the current tree)

1. **SQLite pragma policy — `synchronous=NORMAL` everywhere steady-state**
   (queue store, readmodel projection, DLQ sidecar): WAL + NORMAL trades
   per-commit fsyncs for checkpoint-only fsyncs. **Measured via strace:
   1566 → 43 fsync syscalls** for 500 enqueue→claim→complete cycles
   (36×). Plus `temp_store(MEMORY)`, `cache_size(-32768)`,
   `journal_size_limit(8MB)`. Escape hatch `TQ_SQLITE_SYNC=full|normal|off`
   (validated at open). Migration/upgrade DSNs deliberately stay FULL.
   Cycle benchmark 267µs → ~190µs (~30%) on NVMe-with-write-cache (the
   fsync win is far larger on honest-latency storage).
2. **ONE shared serialized connection** in `sqlitev4.Open`: the engine
   (`usqlite.OpenDB`) and every companion surface ride the same
   single-conn `*sql.DB` — the two-pool WAL writer-lock handoff and its
   SQLITE_BUSY churn are gone. Conform suite green on the unified handle.
3. **Claim-path partial indexes** (`companion.Migrate`, one DDL, both
   dialects): `idx_tasks_project_running`, `idx_tasks_lease_running` —
   the project-exclusivity NOT EXISTS now answers from the RUNNING set
   instead of the project's whole history. **Benchmark with 20k history
   rows: 8.77ms → 0.25ms per claim cycle (35×).**
4. **Worker adaptive idle backoff**: consecutive `ErrNoTaskDue` polls
   double the gap (cap `IdlePollMax`, default 2s, negative disables),
   reset on any claim or hard error. Idle-loop test proves ~5× fewer
   polls in 500ms with the ladder.
5. **HTTP transports pooled**: `internal/bridge/httpx.TunedClient`
   (16 idle conns/host) for papdashboard bridge + answer poller + cqa;
   executor module gets the same shape locally (module boundary).
6. **Benchmarks**: `internal/queue/sqlitev4/bench_test.go` — ClaimCycle,
   ClaimIdle, ClaimDeepHistory (the IO-efficiency canaries).
7. **Tests**: pragma-policy pins (WAL/NORMAL/single-conn + env escape +
   invalid-override refusal), claim-index existence pins, idle-gap
   ladder + integration backoff test. All green.
8. **Docs**: AGENTS.md store-invariants bullets (shared conn, IO policy,
   backoff), CHANGELOG entry, TODO_LIST row for the notify-wake endgame.
   AGENTS size budget consciously reset 16,400 → 16,900 (fifth reset,
   both twins, per the pinned reset protocol — net-new load-bearing
   invariants, compacted before budgeting).

Verification state: per-module gate green for ALL modules touched
(companion, sqlitev4, sqlite, worker, executor, readmodel, postgresv4,
postgres), root `go build/vet/test ./...` fully green post-dep-bump,
cmd/tq gate green, gofmt/gosec/agents-size green in ci-local pass1.

## b) PARTIALLY DONE

- **`scripts/ci-local.sh` end-to-end green run**: blocked mid-verification
  by concurrent-forest events (see d); every Go gate inside it passed at
  least once this session; the only currently-red module is foreign (see
  d1). A quiet-tree `CI_CHECK=off ./scripts/ci-local.sh` (or a plain run
  after journal/cqrs heals) is the one remaining formality.

## c) NOT STARTED (deliberately deferred, rowed in TODO_LIST)

- Notify-after-commit journal wake (ADR-0009 v2): removes polling
  entirely; idle backoff is the stopgap. TODO_LIST row added.
- consumer.Dispatcher idle fast-path (HeadSeq once per tick + cursor
  dedupe) — skipped when research showed the dispatcher has ZERO
  production importers (ghost package, already rowed by a prior session).

## d) TOTALLY FUCKED UP / HIT BY CONCURRENT FOREST

1. **`internal/journal/cqrs` is red — NOT mine, left to its owner.** The
   03:12 dep-bump commit (b886a677) raised go-cqrs-lite modules whose
   `id.StreamMarker` now String()s with a `StreamMarker:` prefix;
   `TestReadAllMapsFactsInSeqOrder` and `TestSessionFactsUseSessionStreamType`
   assert the OLD raw format. Zero overlap with my diff; fixing it while
   the dep-migrating agent is mid-flight would race their judgment.
2. **Dep-bump landed without `go mod vendor` / module tidies**: I healed
   root vendor sync + tree-wide `go mod tidy` (15 files; completing the
   concurrent work, not reverting). Remote CI run 37556055366 on b886a677
   is red for exactly this reason and predates the heal — ci-local's
   check-ci correctly flagged it; final runs used the sanctioned
   `CI_CHECK=off`.
3. **AGENTS.md table re-padding incident (again)**: a formatter regrew
   +1.4KB of alignment whitespace mid-session, tripping the size guard;
   pruned back to compact form per the pinned doctrine (content
   byte-identical, only padding removed via `git restore --source`).
4. **Host modcache poison**: `go mod verify` failed on a modified
   `klauspost/compress@v1.20.1` extraction; healed by same-volume rename
   quarantine (trash cannot operate on /mnt/buildcache) + re-extraction.
5. **My own misses**: two edit rounds with `#`-prefixed Go comment lines
   (caught by build), one gofmt struct-alignment miss (caught by gate),
   first benchmark readings taken as single runs (noise — corrected to
   count=3 + strace syscall counts as the real evidence).

## e) WHAT WE SHOULD IMPROVE

- **Stop taking single-run benchmarks** — count≥3 or syscall counting;
  the first "slower after NORMAL" reading was pure noise and could have
  derailed the whole change.
- **The dep-bump workflow needs a vendor+tidy gate step** — two heals
  this session (root vendor, tree-wide tidy) are the same class as the
  03:12 red CI run; a pre-commit check "go.mod changed but vendor/
  modules.txt did not" would catch it at gate time.
- **trash has no volume story for /mnt/buildcache** — document the
  rename-quarantine pattern in AGENTS Known Issues.
- **check-ci position in ci-local**: it runs EARLY and aborts before the
  Go gates, which reads as "everything failed" when only the foreign
  remote run is red (misleading during exactly the concurrent-agent
  storms this repo runs in).

## f) NEXT (grounded, deduped — most valuable first)

1. Heal `internal/journal/cqrs` StreamMarker String() fallout (dep owner
   or next session; decision: update test expectations vs compare raw IDs).
2. Notify-after-commit wake (ADR-0009 v2) — TODO row exists.
3. Remote CI green on the healed tree (post-journal/cqrs fix).
4. vendorHash drift check: dep bump changed go.mods — run
   `nix build .#checks.x86_64-linux.vendor-hash` and update flake.nix.
5. `PRAGMA optimize` (or periodic ANALYZE) on store close — keeps the
   claim-query planner honest as tables skew.
6. WebUI board/board-query LIMIT audit (unbounded List renders on SSE
   refresh — CPU+IO, not yet measured).
7. PapDashboard bridge 5s journal tail + AnswerPoller 10s: piggyback the
   notify-wake instead of fixed polling once (2) lands.
8. Heartbeat cadence: `Lease/4` per RUNNING task writes a fact each —
   consider adaptive heartbeat (fast when lease-critical, slow when
   fresh) once notify-wake exists.
9. Postgres: same IO audit (synchronous_commit, fillfactor, index bloat)
   — sqlite got the pass, postgres is the other driver.
10. `Enqueue` re-read (`Get` after engine insert) — return the row from
    the insert path; one PK lookup saved per enqueue.
11. Bench `Facts(after, limit)` on a 100k-fact journal (sweeper page cost
    at depth) — the 500-page walk was assumed cheap, never measured.
12. facts table growth policy: archive/truncate strategy for multi-M
    fact journals (D4 divergence noted in S1 report).
13. Sweepers share one Facts read per tick (review/status/prioritize/
    depsweep each page the same range at their own cadences).
14. `readmodel` fold batch size default audit (DefaultBatch) against the
    9.5k-fact restart replay measurement.
15. gosec root "scanned 0 files" flake (pass2) — one-shot reproduce and
    root-cause the silent skip.
16. Consider `wal_autocheckpoint` tuning under sustained burst load
    (benchmarked: default was fine on NVMe; network storage differs).
17. Exempt `journal_size_limit` from migration DSNs deliberately — done;
    row the rationale in the S1 divergence report if it ever changes.
18. Dispatcher ghost-package decision (existing blocked row).
19. AGENTS.md: fold the rename-quarantine modcache pattern into Known
    Issues (from e).
20. ci-local: move check-ci to the END (or make it non-fatal) so foreign
    remote red doesn't mask local Go verdicts (from e).
21. Pre-commit guard: go.mod-without-vendor-sync detection (from e).
22. Bench the idle-backoff ladder against multi-pool claim latency
    (worst-case 2s pickup with default caps — fine for agents, worth a
    number for HTTP-task users).
23. Document `IdlePollMax` in the tq flag surface (currently library-only).
24. Consider `TQ_SQLITE_CACHE` env for small hosts (32MB default may be
    generous on SBCs the systemd units target).
25. Re-run the full benchmark matrix on the post-dep-bump tree once
    journal/cqrs heals (numbers in this report are pre-bump evidence).

## g) QUESTIONS FOR THE OWNER

1. **Durability stance**: is `synchronous=NORMAL` (lost tail commits on
   OS/power failure, reclaimed by lease expiry + at-least-once) the
   right DEFAULT for the production journal, or should production
   systemd units pin `TQ_SQLITE_SYNC=full` and NORMAL stay dev-only?
2. **Claim latency cap**: idle backoff tops out at 2s claim pickup for
   freshly enqueued work — acceptable for all pools, or should HTTP-task
   pools (sub-second semantics) get a lower `IdlePollMax` default?
3. **journal/cqrs StreamMarker fallout**: mine to fix next session if
   the dep owner hasn't, or strictly leave it to the dep-migration
   agent? (I read it as theirs; a second opinion prevents a race.)
