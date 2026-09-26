# Companion Extraction — Close-Out (production half landed; suites ratcheted)

- **Window:** 2026-09-26 06:00–07:00 (resumed session, executor: GLM-5.3-Flash via tq pool rails)
- **Mission:** execute the owner's dedup ruling ("accepts ≈ 0") — home the mirrored
  ADR-0019 spike-adapter surfaces in `internal/queue/companion` per the verified
  design, verify each step at the gate.
- **Scope honesty:** the PRODUCTION extraction landed completely (gate-verified);
  the conformance-suite consolidation did NOT land this window and is ratcheted
  instead (rationale §c2).

## a) Verdict

**DONE (production), DISCLOSED (test suites).** Every adapter surface the three
spikes mirrored now lives once in `internal/queue/companion`
(core/reads/claims/questions/schema); sqlitev4, postgresv4, and cqrsqlite are
one-line delegators. art-dupl reports **zero cross-backend adapter groups**
(31 at window open → 0). The 6 remaining cross-backend groups are all inside the
three mirrored conformance suites; they are count-pinned in
`scripts/mirror-baseline.txt` and the gate is now STRICT in ci-local
(any NEW group fails). Full battery green at HEAD except the 2 pre-existing
drift-test reds, whose root cause is upstream and now documented with a
concrete recommendation (§f3).

## b) What landed (with evidence)

1. **companion module (5 files):** `core.go` (Dialect/SQLite/Postgres, Runner,
   For, WithTx, pgq moved from postgresv4, MapErr, UpstreamFact,
   IdentityCodec, StoreOption trio), `reads.go` (Get/List/CountTasks,
   Facts/LastFacts/HeadSeq/FactsForTask/CountFacts/FactsSince,
   watermarks, priority scores, StatusCounts/ProjectCounts, scanners,
   listWhere/escapeLike), `claims.go` (TokenFor, ClaimDue with project
   exclusivity, Requeue with resume_closeout evidence, AppendFactTx,
   cancel-request scans, MustJSON/boolInt), `questions.go` (RecordAnswer,
   asked-question backfill, payload injection, mergeAnsweredPayload),
   `schema.go` (companion `priority_scores` DDL — BIGINT has INTEGER
   affinity in SQLite, one DDL serves both families — + Migrate).
2. **Rewired sqlitev4** (1486→~460 lines): engine-backed methods verbatim,
   companion surfaces delegate; module build+vet+full 3.6s conformance suite
   green.
3. **Rewired postgresv4** (1585→~470): same, behind `companion.Postgres`;
   build+vet+test-compile green (runtime = CI TQ_TEST_POSTGRES job — no live
   postgres here; disclosed). Kept `pgq`/`exec`/`mustJSON` package shims so
   the suite's seeded-SQL blocks compile untouched.
4. **Rewired cqrsqlite** (extras.go 744→~150): same treatment; build+vet+
   full conformance suite green (3.3s).
5. **Known-good cross-dialect normalizations** (behavior-preserving, all
   suite-pinned): MIN(expr,cap)→CASE WHEN (LEAST is pg-only), LIMIT -1→
   LIMIT max-int64 (offset-only pages; sqlite's -1 is invalid pg),
   INTEGER→BIGINT in the shared DDL, companion.Migrate unifies the three
   migrateCompanion twins.
6. **Module graph:** companion require+relative-replace added to the 7
   consumer modules the loop exposed (internal/queue/{sqlite,postgres,worker},
   queue/{sqlite,postgres}, worker, internal/readmodel) — the exact
   untagged-module pattern AGENTS.md's facade lesson predicts (replace in a
   dependency's go.mod is ignored by dependents); root go.mod + `go mod
   vendor` refreshed.
7. **Gate hardened:** `check-mirror-clones.sh` rewritten —
   `MIRROR_CLONES_STRICT=1` is now the ci-local default, group identities
   (category + sorted backend file pair, line-free) compared against
   `scripts/mirror-baseline.txt`; RESOLVED rows reported; the 3 residual
   per-backend boilerplate clusters (interface-assert + StoreOption alias,
   Close two-handle teardown, Get delegator) carry reasoned
   `// art-dupl:accept` directives — the tool's native mechanism, not gate
   weakening.

## c) What did NOT land + why

1. **companion/conform consolidation** (the 3 × ~3.5k-line mirrored suites →
   one parameterized runner). The suites are ~93% identical
   (sqlitev4↔postgresv4 diff: 235 lines; ↔cqrsqlite: 94) with a clean seam
   (8 direct-DB pokes, 2 constructors, ~10 helper signatures) — fully
   designed (docs/planning/2026-09-26_companion-extraction-design.md §
   conformance), but a 3-way 69-test merge cannot land complete and verified
   in one window. Half-transcribing the tree was rejected as the worst
   end state.
2. **The ratchet instead:** the repo already treats lint the same way
   (baseline growth = gate, shrink = deliberate regen). The 6 known test
   mirrors are baselined; any NEW cross-backend group anywhere fails ci-local
   immediately. The consolidation stays TODO (row filed) and the baseline
   shrinks to zero when conform lands.
3. **The 2 red drift tests stay red, owned by S2** — see §f3; Q1 is now
   answered with a root cause and a recommendation rather than a patch.

## d) What could have gone better

1. The module-graph fallout (7 consumer go.mods + root vendor) was
   predictable from AGENTS.md's facade-require lesson; the loop found them
   one by one instead of me pre-wiring every graph from `go mod graph`
   upfront. Next time: enumerate dependents BEFORE the first consumer build.
2. Three edit-tool failures from reconstructing old_string from garbled
   transcript views instead of re-viewing the file — the exact-match
   discipline (view immediately before edit) caught up with me twice;
   AGENTS.md already prescribes this for hot files.
3. The strict-flip design changed mid-window (blanket strict → baseline
   ratchet) once the boilerplate clusters proved non-extractable; stating
   the ratchet policy in the gate header first would have saved a rewrite.

## e) Gates at HEAD (all run this session, exit codes captured)

- Root: build ✅ vet ✅ `-race` 15/15 pkgs ✅ (incl. internal/e2e 20.6s)
- 19-module loop: build+vet+test ALL GREEN (sqlitev4 3.6s, cqrsqlite 3.3s,
  postgresv4 test-compile; runtime = CI postgres job)
- check-go-mods.sh: 45 ok / 0 failed
- nix vendor-hash fast gate: green (path-replaced internals don't move the
  go-modules FOD — no external deps added)
- facade parity: 7 facades ok; guard wiring: 35/0; check-todo-list ✅;
  check-doc-refs ✅
- mirror gate: `6 groups: 0 new, 6 baselined`, strict ✅
- cmd/tq gate: **2 pre-existing failures only** (TestJournalDrift*), zero new
- gofmt: only pre-existing vendor/ noise; tracked files clean

## f) Findings & recommendations

1. **companion is now the S4-surviving extras home** the ADR-0019 memo
   planned: questions/scores/reads/watermarks survive the spike deletion;
   S4's driver work inherits the Dialect seam instead of re-deriving it.
2. **Baseline ratchet policy:** mirror-baseline rows may only shrink
   (consolidation) or move with a recorded reason; regen is deliberate, like
   lint-baseline.
3. **Drift reds = upstream gap (Q1 closed):** the engine writes
   `task.enqueued` detail `{project,type}` only (verified in
   go-cqrs-lite queue/sqlite/v4@v4.0.0 enqueue.go: appendFact with a
   two-key map), so the journal-drift audit's priority/dedup_key coverage
   and the ADR-0019 projection-equality replay story are broken at the
   engine level, for both backends. Recommendation: enrich the upstream
   engines' enqueued detail with the `queue.EnqueueDetail` snapshot keys
   (the exact shape tq wrote 2026-09-24) behind the M4 ratification memo;
   tq-side hacks (second fact, detail rewrite) rejected — they bake a shape
   M4 hasn't ratified. Recorded as a TODO row.
4. **Session-forensics note:** cqrsqlite's suite passes unmodified over the
   companion bodies with three micro-divergences absorbed (deps-guard
   scanTask, non-panicking mustJSON, LastFacts(0) path) — all unreachable-
   in-practice behavior deltas, all pinned by the suite.

## g) Open questions for the owner

1. **Conform timing:** consolidate the suites now (before S2/S4 churn) or
   fold it into the S4 cutover where the spike suites die anyway? The
   ratchet makes either safe; this window defaulted to "before S4" in the
   design doc.
2. **Upstream M4:** may I draft the M4 ratification memo (or the upstream
   enrich PR) for go-cqrs-lite's enqueued detail? The drift reds cannot go
   green tq-side without it.
3. **Push/CI:** unchanged policy question from the morning — this window
   did not push; master-CI ownership remains with the owner per standing
   rails.
