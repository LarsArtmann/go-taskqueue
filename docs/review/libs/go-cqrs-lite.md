# go-cqrs-lite — Library Utilization Review

**Verdict:** ADOPTED — high (27/33) — the platform ADR-0019 named (queue engines, metaengine read side, system root) is ridden deep: every high-impact gap of the 2026-10-05 deep-dive has since landed (projectionhost M08, Delta counters, health/doctor); what remains is a version wave, scan caching, and optional OTel.

**Pinned:** (direct requires, per consumer module) event/v4 v4.15.2, id/v4 v4.7.4, metaengine/v4 v4.20.0, metaengine/sqliteengine/v4 v4.6.0, projection/v4 v4.4.2, projectionhost/v4 v4.7.1, queue/v4 v4.0.3, queue/sqlite/v4 v4.0.3, queue/postgres/v4 v4.0.3, record/v4 v4.6.2, system/v4 v4.16.0 (+ indirect: command v4.14.1, commandlifecycle v4.3.1 + projections v4.2.3, claiming v4.0.2, decider v4.8.0, dedup v4.3.0, dispatcher v4.5.2, metadata v4.7.3, otel v4.5.3, query v4.10.1, schema v4.7.1, snapshot v4.7.0, watermill v4.7.0, metaengine/projectionadapter v4.6.2). Verified 2026-11-08 by `grep -rn 'larsartmann/go-cqrs-lite' --include=go.mod . | grep -v vendor`.

**Latest:** event/v4.15.2 (=), id/v4.7.5 (+1), metaengine/v4.23.0 (+3), metaengine/sqliteengine/v4.8.0 (+3 tags), projection/v4.4.2 (=), projectionhost/v4.7.1 (=), queue family v4.0.3 (=), record/v4.6.2 (=), system/v4.18.0 (+2) (local checkout /home/lars/projects/go-cqrs-lite, checked 2026-11-08 by `git tag -l '<module>/v*' | sort -V | tail -1` per module).

**Prior coverage:** docs/research/2026-10-05_go-cqrs-lite-metaengine-system-deep-dive.html (58/100, 6/15 capabilities full, 0 versions behind). **Superseded by this report for go-cqrs-lite**: its findings F1 (projectionhost + durable checkpoints + DLQ), F2 (Delta counters), F3 (health/doctor), F5 (cursor pagination), F6 (metrics hooks) have all landed in the tree since (cited below); its version table rotted (three upstream waves since). None for cqrs-htmx (sibling report).

**Reviewed:** 2026-11-08 by: `grep -rl --include='*.go' 'larsartmann/go-cqrs-lite' . | grep -v '^./vendor/'` (25 files); line-level reading of internal/composition/{compose,projection_runtime}.go, internal/readmodel/{host,model,queries,stats,dlq,projectionhealth,workerfailed}.go, internal/queue/{sqlitev4,postgresv4}/adapter.go, internal/queue/companion/{core,claims,enqueue}.go + conform/, internal/journal/{journal.go,cqrs/}, cmd/tq/main.go (4385 lines); upstream checkout: per-module tags, CHANGELOG.md (13,626 lines, 2026-11-08 wave), module tree.

## 1. Usage inventory

25 repo files import go-cqrs-lite (verified 2026-11-08 by the grep above): 18 production files across 8 packages (cmd/tq, composition, readmodel, sqlitev4, postgresv4, companion, journal, journal/cqrs) + 7 test files (httpapi's only import is a test). Per-module map:

| Repo module | cqrs-lite modules | Key symbols (file:line) |
| --- | --- | --- |
| internal/journal | queue/v4 | `facts.FactType`/`facts.Fact` — tq vocabulary is TYPE-ALIASED to upstream (journal.go:39,44; identity re-exports :47-58) |
| internal/journal/cqrs (ADR-0014, read-only, proprietary) | event/v4, id/v4 | `FactJournal` = `event.Journal`+`event.SeekableJournal` (journal.go:36-43); seq-encoded `id.EventID` (seqid.go); codec-stamped payloads (journal.go:149) |
| internal/queue/{sqlitev4,postgresv4} (S1) | queue/v4, queue/sqlite/v4, queue/postgres/v4, queue/v4/task | `usqlite.OpenDB[[]byte]` + `IdentityCodec` (sqlitev4/adapter.go:152,201); token-fenced finalizes (`uqueue.ErrLeaseNotHeld`…:146-154); `engine.WithFacts` same-tx sink (:408); DAG deps (`utask.ID`:31) |
| internal/queue/companion | queue/v4 (+facts, +task) | shared tq surfaces (claims, enqueue deps, reads) over the SAME db; `ufacts.Fact` mappers (core.go:12) |
| internal/readmodel (S3) | metaengine/v4, sqliteengine/v4, record/v4, event/v4, id/v4, projection/v4, projectionhost/v4 | `metaengine.Plan` (model.go:260), `Query[TaskList,TaskRow]` + `WithColumnarLayout` (queries.go:60,133), Delta counters via `ExecuteTyped` (stats.go:30-40), `ScanPage`+`WithCursor` (model.go:604), `Watcher`+`ServeSSE` (model.go:278,640), `projectionhost.New` + `WatermarkCheckpoints` (host.go:300), `SQLiteDeadLetterStore` (dlq.go:88), `WithMetrics` worker-death marker (host.go:276) |
| internal/composition (S4) | system/v4, metaengine/sqliteengine/v4 | `system.New(DomainConfig{}, DeploymentConfig)` (compose.go:94); `ProjectionRuntime` with fold-lock election + `NewSQLiteEngineFromDSN` (projection_runtime.go:97) |
| cmd/tq | system/v4 (+ readmodel/composition re-exports) | `sys.GracefulClose` close ordering (main.go:4363); `tq facts --cqrs` render (main.go:3786); projection health in stats/api (main.go:2152, httpapi.go:479) |

**How the conform suite works** (internal/queue/companion/conform/suite.go:1-19): every S1 backend (sqlitev4, postgresv4) runs the SAME sequential `StoreSuite` through a thin harness — the backend owns `FreshDSN`/`OpenOn`/raw seeding (`Exec`/`Begin`, dialed via `Dialect`), while capability knobs (`Caps`) carry the S1 divergences (e.g. `Caps.EnqueuedSnapshot=false` pinned, per AGENTS.md) instead of per-backend skip edits. It exercises the tq contract SUPERSET (queue.Store + companion extras: facts reads, watermarks, priority scores, questions, archive, project counts — suite.go:29-80). This is tq's own suite; the upstream `queue/conformance` suite is not imported (verified: zero hits).

**ADR-0014 boundary (unchanged, healthy):** internal/journal/cqrs stays read-only and proprietary — writes live exclusively in the queue engines' transactions; the duplicated seq-codec in readmodel/host.go:39-113 is a deliberate `art-dupl:accept` mirror pinned by `TestSeqIDCodecParity` (host.go:30-34), not drift.

## 2. Capability inventory

Status legend: 🟢 FULL, 🟡 PARTIAL, 🔴 MISSED, ⚠️ MISUSED, ⚪ N/A (reason inline). No ⚠️ rows found.

| # | Capability (family) | Status | One-line reason / evidence |
| --- | --- | --- | --- |
| 1 | system.New runtime + DeploymentConfig engines/instances | 🟢 | one engine, RoleProjections instance (compose.go:76-94) |
| 2 | system GracefulClose close ordering | 🟢 | after runactor actors stop (cmd/tq/main.go:4363) |
| 3 | system DomainConfig domain wiring | 🟡 | empty BY DESIGN — no second journal (ADR-0019 P4); upstream adapter seam still pending (finding 6) |
| 4 | system HealthCheckDetailed | 🔴 | equivalent info read via readmodel.ProjectionHealth (httpapi.go:479) — system's own surface unused |
| 5 | system checkpoint engine | ⚪ | cursor homes in the queue watermarks table — ONE checkpoint home (ADR-0009; host.go:91-135) |
| 6 | system timers (scheduling integration) | ⚪ | sweeper ticking stays runactor until a timer-shaped feature lands (§4a ruling) |
| 7 | system.Standard canonical topology (v4.18.0) | 🔴 | ships in the un-pinned v4.18.0; tq's single-home shape differs anyway (§5) |
| 8 | metaengine planned tables + columnar layout | 🟢 | LayoutPlan via WithColumnarLayout — typed SQL columns (queries.go:57-60,133) |
| 9 | declarative filter/sort pushdown | 🟢 | FilterOnField/SortOnField (queries.go:130-133) |
| 10 | Delta counter aggregates (ExecuteTyped) | 🟢 | status + project\|status counters, O(1) (stats.go:30-40; queries.go:139-205) — prior F2, landed |
| 11 | cursor pagination (ScanPage + WithCursor) | 🟢 | TasksPage (model.go:572-604) — prior F5, landed |
| 12 | Watcher + ServeSSE live fragments | 🟢 | replay ring + heartbeat + timeout (model.go:278,640-644) |
| 13 | engine health/stats snapshot | 🟢 | HealthSnapshot + GetEngineStats in /health + doctor (projectionhealth.go:59-72) — prior F3, landed |
| 14 | Store.Verify/Backfill drift machinery | ⚪ | no event log attached by construction (upstream: system roots return ErrNoEventLog) |
| 15 | WithPrefetch scan cache (metaengine v4.21.0) | 🔴 | not pinned; TasksPage is exactly the coached cursor-scan shape (finding 3) |
| 16 | WithCap / WithCapPerGroup retention caps | ⚪ | the ledger keeps full history; journal compaction owns retention (ADR-0010) |
| 17 | TrimStream / TrimJournalBefore (metaengine v4.23.0) | ⚪ | tq's journal is the QUEUE engine's facts table, not a metaengine journal |
| 18 | memory engine (fast tests) | 🔴 | readmodel tests deliberately boot real sqlite (fold locks, WAL); only the memory DLQ store is used (host_test.go:275) |
| 19 | alternative engines (pg/pebble/bbolt/badger/duckdb/turso/mysql/bigtable/dgraph/iroh/graph) | ⚪ | sqlite-embedded stance; pg projection home = named future work (compose.go:70) |
| 20 | queue/v4 + queue/{sqlite,postgres}/v4 engines | 🟢 | both S1 drivers live (sqlitev4/adapter.go:152; postgresv4/adapter.go:39) |
| 21 | token-fenced finalizes (claim theft detection) | 🟢 | claim tokens, uqueue error mapping (sqlitev4/adapter.go:146-154,272-334) |
| 22 | FactTx.WithFacts same-tx fact sink | 🟢 | engine-enforced facts-in-same-tx (sqlitev4/adapter.go:408) |
| 23 | DAG deps at claim, priority aging, dedup-keyed enqueue | 🟢 | deps (adapter.go:31), UpdatePendingPriority (:334), DedupKey via engine — conform-pinned |
| 24 | upstream queue/conformance suite | 🟡 | tq runs its OWN conform suite over its superset contract (§1); upstream suite not imported — defensible, keeps companion surfaces under test |
| 25 | queue/mysql/v4 backend | ⚪ | no mysql deployment; postgresv4 proves the adapter pattern is cheap if ever needed |
| 26 | projectionhost managed host (restart budget, checkpoint batching) | 🟢 | M08 adoption (host.go:280-300) — prior F1-full, landed |
| 27 | durable checkpoints (CheckpointStore → watermarks) | 🟢 | WatermarkCheckpoints adapter (host.go:91-135) + WithDurableCursor (model.go:145) — prior F1-minimal, landed |
| 28 | poison-fact DLQ (SQLite + Memory stores) | 🟢 | sidecar beside projection home (dlq.go:88; host.go:297); memory store in tests |
| 29 | projectionhost WithMetrics hook | 🟢 | worker-death marker sidecar (host.go:276; workerfailed.go) — prior F6, landed |
| 30 | lag reporting (LagDuration) | 🟢 | doctor/stats/api read it (host.go:334; main.go:2152; httpapi.go:479) |
| 31 | projection.Projection contract | 🟢 | compile-time pin (host.go:341-342) |
| 32 | live event.Subscriber seam | 🟢 | tailSubscriber poll-tail behind WithSubscriber (host.go:263,343-463) — no bus needed |
| 33 | event.Journal/SeekableJournal adapter | 🟢 | ADR-0014, read-only (journal/cqrs/journal.go:36-43) |
| 34 | event.CheckpointStore | 🟢 | watermarks adapter (host.go:91) |
| 35 | codec-stamped payloads + DecodePayloadAuto | 🟢 | JSON codec + schema version pinned (journal/cqrs/journal.go:149; host.go:206) |
| 36 | id/v4 seq-encoded EventIDs | 🟢 | ULID[6:14] = Seq big-endian (journal/cqrs/seqid.go; host.go:52-113) |
| 37 | record/v4 typed folds | 🟢 | OnRecordTyped over the fact vocabulary (queries.go:63-127) |
| 38 | `tq facts --cqrs` operator surface | 🟢 | SliceSource → FactJournal → JSON (cmd/tq/main.go:3786) |
| 39 | watermill/v4 bus | ⚪ | no broker (ADR-0014 ruling); the tail subscriber covers live delivery |
| 40 | snapshot/v4 | ⚪ | projection is rebuildable; the durable cursor bounds replay (indirect only) |
| 41 | dedup/v4 engine dedup | 🟢 | DedupKey/ErrTaskDone semantics ride the engine (indirect, conform-pinned) |
| 42 | idempotency (sqlstore) | ⚪ | ADR-0014: tq's dedup is same-transaction with the enqueue — cannot be externalized |
| 43 | otel/v4 + prometheus/v4 observability stack | 🔴 | indirect only; no OTel provider, no /metrics surface (finding 4) |
| 44 | systemtest / systemscenario / benchkit / integration harnesses | ⚪ | no system DomainConfig domain to boot (empty root is deliberate); tq gates on its own conform + smokes |
| 45 | testutil FaultInjector / scenario DSL | ⚪ | event-store-shaped wrappers; readmodel failure pins are hand-built on real sqlite |
| 46 | saga / sync / retry (historical tags) | ⚪ | all three modules REMOVED upstream at HEAD (saga deletion commit 3d3802deb); workers are tq's process managers |
| 47 | scheduling | ⚪ | park/requeue ladder + runactor own all timing |
| 48 | deriver | ⚪ | incident→fix-task is an imperative bridge (internal/incident), not an event-bus derivation |
| 49 | listing | ⚪ | superseded by metaengine planned tables (ADR-0019: v1 tiers removed in v5) |
| 50 | encryption / signing | ⚪ | single-trust local deployment; secrets redaction is output-side |
| 51 | command/query/dispatcher/decider/commandlifecycle/metadata/schema | ⚪ | system-internal composition; tq declares no commands/queries of its own |
| 52 | catalog/graph/systemcatalog/stack/core/kv/middleware/flightrecorder | ⚪ | upstream authoring + fleet-observability tooling; no tq surface |

## 3. Findings

1. **The prior deep-dive's entire high-impact backlog landed — this report supersedes it.**
   Offer: /home/lars/projects/go-cqrs-lite/projectionhost (+ metaengine COOKBOOK §Counter Patterns).
   Current: managed host with restart budget, checkpoint batching, DLQ threshold, metrics hook (internal/readmodel/host.go:280-300); Delta counters (internal/readmodel/queries.go:139-205, stats.go:30-40); ScanPage cursor pagination (internal/readmodel/model.go:572-604); engine health in /healthz + stats (internal/readmodel/projectionhealth.go:59-72; internal/httpapi/httpapi.go:479; cmd/tq/main.go:2152); durable cursor via watermarks (host.go:91-135).
   Impact: the 2026-10-05 score of 58/100 no longer describes the tree — the projection's delivery machinery, the O(1) tallies, and the health surface are all platform-owned now. Recorded as verification, not new work.

2. **Version wave behind: metaengine v4.20.0→v4.23.0, sqliteengine v4.6.0→v4.8.0, system v4.16.0→v4.18.0, id v4.7.4→v4.7.5 (three upstream waves: 2026-11-06 ×2, 2026-11-08).**
   What the waves add (upstream CHANGELOG.md:43-110, verified 2026-11-08): `AggregateFn` SQL-injection whitelisting (security class — sqliteengine interpolated consumer-controlled strings raw; tq never calls AggregateFn directly, verified by `grep -rn 'AggregateFn' internal cmd` → zero hits, but the engine family tq ships is hardened); the EnforceCap-vs-fold deadlock fix (#123 — tq uses no caps, verified grep → zero hits, not triggered); `metaengine.WithPrefetch` (see finding 3); classified `Verify`/`Backfill` errors; journal trimming (`TrimStream`/`TrimJournalBefore`, N/A for tq today); `system.Standard` topology + MaxOpenConns plumbing (tq pins MaxOpenConns(1) deliberately — store invariant — so the pool escape hatch is N/A).
   Impact: no breakage risk (all additive per changelog; queue family + event + record + projectionhost unchanged at tags). Vehicle: `internal/depsweep` (`--dep-sweep`, cmd/tq/main.go:1422-1429) mints the depbump task wave over the release-overview plan; then per-module `go mod tidy`, root `go mod vendor`, and the nix vendorHash update (AGENTS.md gates).
   Example (one consumer module, then sweep the rest):
   ```go
   // internal/composition/go.mod + internal/readmodel/go.mod (tidy picks the rest)
   require (
       github.com/larsartmann/go-cqrs-lite/metaengine/v4 v4.23.0
       github.com/larsartmann/go-cqrs-lite/metaengine/sqliteengine/v4 v4.8.0
       github.com/larsartmann/go-cqrs-lite/system/v4 v4.18.0
   )
   ```

3. **WithPrefetch on the row-rich dashboard scans (needs the v4.21.0+ bump first).**
   Offer: /home/lars/projects/go-cqrs-lite/metaengine — `WithPrefetch(cache)` per-call scan option (CHANGELOG.md:72; the cqrs-lint F026 coached shape, shape-qualified cache key).
   Current: internal/readmodel/model.go:604 — `NewReader[TaskRow](…).ScanPage(ctx, opts…)` with no cache; every dashboard page render and webui fragment read re-fetches its window.
   Impact: cursor-paginated board/table views are exactly the coached consumer; a shared `metaengine.PrefetchCache` on the Model would serve repeat windows without doubling fetches on one-shot reads (an explicit `WithPrefetch(nil)` preserves export paths).
   ```go
   // Model field + TasksPage read path (after the version bump)
   m.prefetch = &metaengine.PrefetchCache{}
   opts = append(opts, metaengine.WithPrefetch(m.prefetch)) // cursor scans only, not Tasks()
   ```

4. **OTel/Prometheus stack idle — the one real observability gap (optional, costed).**
   Offer: /home/lars/projects/go-cqrs-lite/{otel,prometheus} — provider setup, engine/host metrics, `/metrics` bridge (otel/README.md:1-30; prometheus/README.md:1-16).
   Current: zero OTel references in tq (verified by `grep -rn 'go-cqrs-lite/otel\|go-cqrs-lite/prometheus' --include='*.go' . | grep -v vendor` → no output); observability = doctor + /healthz projection section + webui.
   Impact: a token-gated `/metrics` (loopback posture, same gate as /api/v1) would export fold lag, worker failures, engine health, and queue counters for the existing go-health-dashboard fleet view. Cost: the OTel SDK dependency tree enters the vendor set — for a single-operator dogfood tool that is a real price; only worth it if the fleet dashboards want tq series.
   ```go
   // internal/httpapi — beside handleHealthz, same token guard
   provider, _ := cqrsotel.Setup(cqrsotel.WithService("tq", version, host))
   mux.HandleFunc("GET /metrics", promhttp.HandlerFor(
       cqrsprom.NewRegistry(provider), promhttp.HandlerOpts{}).ServeHTTP)
   ```

5. **system.HealthCheckDetailed unused — small stitch, not a gap.**
   Offer: system/v4 per-engine health (system/engine surface).
   Current: cmd/tq/main.go:4363 uses the System for GracefulClose only; the equivalent per-engine info is read directly from the metaengine store (projectionhealth.go:59-72) — two entry points for one question.
   Impact: cosmetic (one less bespoke path) — fold `sys.HealthCheckDetailed()` into the same /healthz projection section when the composition root is next touched; not worth its own change.

6. **The empty-DomainConfig shell is now upstream-blessed; the strategic seam is still the unlock.**
   Current: internal/composition/compose.go:94 (`system.DomainConfig{}`) — deliberate (declaring tq collections as system projections would mirror the queue journal: a second source of truth).
   New fact since the prior report: upstream systemscenario now logs a hollow-root WARNING on empty `DomainConfig` boots (upstream CHANGELOG, 2026-10-24 wave) — tq's shell is the documented legitimate case, and accidental empties get caught upstream.
   Impact: carrying prior F4 forward unchanged: revisit declaring real system projections only if upstream's event adapter ever accepts an external `event.SeekableJournal` as its source. Until then projectionhost (row 26-32) is the sanctioned shape, and tq already rides it.

## 4. Score + top opportunities

Inputs (from §2): N=52 rows; ⚪ N/A=19 (rows 5,6,14,16,17,19,25,39,40,42,44,45,46,47,48,49,50,51,52); FULL=26 (rows 1,2,8,9,10,11,12,13,20,21,22,23,26,27,28,29,30,31,32,33,34,35,36,37,38,41); PARTIAL=2 × 0.5 (rows 3,24); MISSED=5 (rows 4,7,15,18,43); MISUSED=0.
A = 52 − 19 = **33** (26 + 2 + 5 = 33 ✓). Σ = 26 + 2×0.5 = **27 of 33**. Score = round(100 × 27/33) = **82 → high** (70-84; very high ≥85).

Top opportunities, cheapest first: (1) the version wave via depsweep (finding 2 — security-class engine hardening rides along); (2) WithPrefetch on TasksPage after the bump (finding 3); (3) token-gated /metrics if the fleet wants tq series (finding 4); (4) HealthCheckDetailed fold-in (finding 5, opportunistic); (5) upstream adapter-seam conversation (finding 6, no code).

## 5. Version currency + risk

| Module | Pinned | Latest (2026-11-08) | State |
| --- | --- | --- | --- |
| metaengine/v4 | v4.20.0 (2026-10-24 wave) | v4.23.0 (2026-11-08) | 3 waves behind |
| metaengine/sqliteengine/v4 | v4.6.0 | v4.8.0 (v4.7.0, v4.7.1, v4.8.0) | 3 tags behind |
| system/v4 | v4.16.0 | v4.18.0 (2026-11-08) | 2 behind |
| id/v4 | v4.7.4 | v4.7.5 (2026-11-06) | 1 patch behind |
| event/v4 | v4.15.2 | v4.15.2 | current |
| projection/v4, projectionhost/v4 | v4.4.2 / v4.7.1 | same | current |
| queue/v4, queue/sqlite/v4, queue/postgres/v4 | v4.0.3 | v4.0.3 | current |
| record/v4 | v4.6.2 | v4.6.2 | current |
| all indirects (command, commandlifecycle×2, claiming, decider, dedup, dispatcher, metadata, otel, query, schema, snapshot, watermill, projectionadapter) | — | same tags | current |

Risk notes: (a) the pinned wave (2026-10-24) is the last FULLY tagged state of the disk-truth tail; the three newer waves are additive per changelog, with no rename breaking tq's import surface — but the upstream FEATURES.md still marks `system` experimental and the v5 push renames some constructors, so a bump belongs on the depsweep rail (gates: conform suites + facade parity + vendorHash), not a hand-edit. (b) ADR-0014 boundary holds: internal/journal/cqrs is never extended (read-only adapter, writes stay in the engine transactions) and its in-repo importers are exactly root surfaces — cmd/tq/main.go + readmodel/host.go (+ their tests); verified 2026-11-08 by `grep -rln 'go-taskqueue/internal/journal/cqrs' --include='*.go' . | grep -v vendor` → 5 files. (c) The 2026-11-08 wave landed in the upstream checkout TODAY — expect one more re-audit of this table at the next depsweep tick.

## 6. Conclusion

go-cqrs-lite is not a dependency here — it is the substrate (ADR-0019): the queue engines hold the state, the metaengine planned tables + Delta counters serve every read, projectionhost owns delivery durability, and system owns the serve-runtime lifetime. Utilization moved from the prior report's 58/100 (concentrated hand-rolled gaps) to 82/33-applicable, with zero misuse rows. The remaining distance to very-high is a routine version wave, one scan-cache option, and an optional metrics surface — none structural. The one deliberately-unfinished seam (empty DomainConfig) is upstream-blocked and correctly parked.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
grep -rl --include='*.go' 'larsartmann/go-cqrs-lite' . | grep -v '^./vendor/' | wc -l   # → 25
grep -rn 'larsartmann/go-cqrs-lite' --include=go.mod . | grep -v vendor | wc -l        # → 152 require rows across modules
grep -n 'system.New(ctx' internal/composition/compose.go                                # → 94
grep -n 'projectionhost.New(' internal/readmodel/host.go                                # → 300
grep -n 'ExecuteTyped' internal/readmodel/stats.go                                      # → 30, 37
grep -n 'ScanPage' internal/readmodel/model.go                                          # → 604
grep -n 'usqlite.OpenDB' internal/queue/sqlitev4/adapter.go                             # → 152
grep -n 'WithFacts' internal/queue/sqlitev4/adapter.go                                  # → 408
grep -n 'facts.FactType$\|facts.Fact$' internal/journal/journal.go                      # → 39, 44
grep -rn 'AggregateFn\|EnforceCap\|WithPrefetch' internal cmd --include='*.go'          # → no output (rows 15/16 untriggered)
grep -rn 'go-cqrs-lite/otel\|go-cqrs-lite/prometheus' --include='*.go' . | grep -v vendor # → no output (row 43)
cd /home/lars/projects/go-cqrs-lite
for m in event id metaengine projection projectionhost queue record system snapshot; do git tag -l "$m/v4.*" | sort -V | tail -1; done
# → event/v4.15.2 id/v4.7.5 metaengine/v4.23.0 projection/v4.4.2 projectionhost/v4.7.1 queue/v4.0.3 record/v4.6.2 system/v4.18.0 snapshot/v4.7.0
git tag -l 'metaengine/sqliteengine/v4.*' | sort -V | tail -1                            # → metaengine/sqliteengine/v4.8.0
sed -n '43,110p' CHANGELOG.md                                                            # the three new waves
cd /home/lars/projects/go-taskqueue/internal/queue/sqlitev4 && GOWORK=off GOEXPERIMENT=jsonv2 go test -run TestStoreConformance -count=1 .   # conform suite green (ok, ~4s)
```
