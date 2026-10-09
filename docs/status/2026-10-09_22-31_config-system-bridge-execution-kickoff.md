# Status: CONFIG-SYSTEM-ALL-IN-BRIDGE execution kickoff — B1 research done, zero artifacts yet

**Date:** 2026-10-09 22:31
**Trigger:** Full Execution Mode ("NOW GET SHIT DONE! The WHOLE TODO LIST!") on
`docs/planning/2026-10-09_20-23_CONFIG-SYSTEM-ALL-IN-BRIDGE.md`.
**Session scope:** this report covers ONLY this session's run (post-trigger).
**Living backlog:** TODO_LIST.md §"go-cqrs-lite platform adoption".
**Repo state at trigger:** clean tree @ `51064df9`.

## a) FULLY DONE (this session)

1. **Plan + skill reload.** Full plan re-read (Pareto tiers, execution graph,
   24 medium tasks, 74 micro tasks, per-track verification gates, guardrails);
   pareto-planning SKILL.md Full Execution Mode confirmed. No `.buildflow.yml`
   in this repo → buildflow skill not applicable.
2. **Execution todo list created** — 24 entries mirroring plan tracks A–F
   (B1 marked in_progress).
3. **B1 research COMPLETE** (the session's substance; note file itself not yet
   written — see b):
   - `internal/composition/compose.go:41-55`: DeploymentConfig declares the
     `"projections"` sqlite engine @ `readmodel.PathFor(dbPath)` with pragmas
     `{journal_mode=WAL, busy_timeout(5000)}`; `DomainConfig{}` empty by
     design; `RoleProjections` instance.
   - `internal/readmodel/model.go:130-131`: the second opener —
     `sqliteengine.NewSQLiteEngineFromDSN(path, "synchronous=NORMAL",
     "cache_size=-32768")`. Upstream DSL
     (go-cqrs-lite `sqliteengine/dsl.go:99-120`) ALWAYS prepends
     `journal_mode=WAL` + `busy_timeout=5000` and sets `MaxOpenConns(1)` →
     **readmodel's connection already carries the full documented union**
     {WAL, busy_timeout 5000, synchronous NORMAL, cache_size -32768}.
     The DIVERGENT connection is the system-declared one (no
     synchronous=NORMAL → SQLite default FULL; no cache_size).
   - `internal/readmodel/dlq.go:37-56`: DLQ sidecar is a **separate file**
     (`modelPath + ".dlq.db"`) opened via raw DSN with
     WAL/busy_timeout(5000)/foreign_keys(1)/synchronous(NORMAL),
     MaxOpenConns(1) → NOT part of the readmodel.db double-open; already on
     the relaxed-fsync policy.
   - **Upstream system/ surface** (read from the LOCAL checkout
     `/home/lars/projects/go-cqrs-lite`):
     - NO `Engine(name)` accessor exists (only `EngineNames()`,
       `MetaEngine()`, `TimerEngine()`, introspection_extended.go).
     - `EngineConfig` is data-only (Driver/DSN/Pragmas/Priority/
       MaterializedViews, config_types.go:198-218) — no engine-instance
       injection seam.
     - `constructor.go:93-152`: system.New builds engines exclusively via
       `createEngineFromDriver` from the registry; engine-less deployments
       fall back to an **in-memory event store** (advisory scream
       `sot.implicit_memory`); empty DomainConfig → no projStore → no
       projection host. Dropping the engine from DeploymentConfig would make
       the S4 root strictly WORSE (memory-fallback shell).
     - `metaengine.Store` has no post-construction collection planning →
       readmodel's collections cannot graft onto `sys.MetaEngine()`.
   - **Conclusion:** plan micro **B2.2 as written ("accessor over the
     deployment-declared instance") is not implementable** against the
     current upstream surface. The review's P1a pre-authorized fallback
     applies: **invert — one tq-owned opener + one shared pragma source; the
     root references it** (upstream accessor/instance-injection becomes a D1
     seam ask, tightening to literally-one-connection post-v5).

## b) PARTIALLY DONE

- **B1 (single-opener design note):** research complete including the pragma
  union table (B1.1) and the accessor-direction decision with evidence
  (B1.2 substance). The note FILE in internal/composition is not written and
  nothing is committed.
- Nothing else touched.

## c) NOT STARTED

22 of 24 executable medium tasks: D1, B2, B3, B4, A1–A7, C1–C5, E1–E3, F1–F4.
D2 remains **BLOCKED: owner go** (upstream filing), tracked not executed.

## d) TOTALLY FUCKED UP

- **Nothing destructive:** no edits, no wrong commits, no reverts; working
  tree untouched; invariants unthreatened.
- **Velocity failure:** the entire session went into B1 research — a task
  scoped at 30 min — with **zero written deliverables and zero commits**
  after the execution trigger. The 1% tier (~3h) is untouched on disk.
- **Went dark:** no interim progress updates between research rounds.
- **Unverified version pin (real gap):** the "no accessor / data-only
  EngineConfig / memory-fallback" conclusions were read from the LOCAL
  go-cqrs-lite checkout; I never confirmed that checkout matches the version
  tq pins (`system/v4@v4.11.0` per vendor/modules.txt). B2's design and the
  D1 memo MUST re-verify against the pinned tag (or vendor/) before any
  claim lands in a committed artifact.

## e) WHAT WE SHOULD IMPROVE

1. Pin upstream-surface claims to the vendored/pinned version before writing
   them into committed docs (`git -C ../go-cqrs-lite describe --tags` or read
   `vendor/.../system/v4`).
2. Write-then-commit cadence: produce artifacts between research bursts;
   batch the B1 note + D1 memo into one mechanical docs commit.
3. Sub-10-word progress updates at track boundaries instead of silent deep
   research.
4. Record the B2 re-scope (P1a inversion) in the design note — plans are
   snapshots; deviations get written down where the next agent will read
   them.
5. Gate hygiene staged for execution: stage+commit BEFORE batteries (daemon
   races <60s); per-module `GOWORK=off` gates; scratch `TQ_DB` for smokes;
   `go mod vendor` after any internal/ change; battery rc to a file.

## f) NEXT (execution order; micro detail in plan §4)

1. B1.3 write the internal/composition design note (union table, invert
   decision, why-not-system-owned, pinned-version caveat).
2. Commit B1 (+D1 when done) as one mechanical docs wave.
3. D1.1–D1.5 seam memo in docs/memos/: (i) injectable projection event
   source over a foreign `event.SeekableJournal` (no journal mirroring —
   cite the empty-DomainConfig ruling), (ii) queue-family instances in
   DeploymentConfig, (iii) engine pool-policy knob (MaxOpenConns-class) +
   engine-instance injection/`Engine(name)` accessor; link from TODO_LIST D2
   row; commit.
4. B2 (rescoped): export the union pragma list from readmodel; composition
   references it in DeploymentConfig; `readmodel.WithEngine(eng)` option;
   composition-owned engine handed to ProjectionRuntime; delete the
   duplicate pragma literal; update compose/projection-runtime tests;
   composition module gate + root build+vet; commit.
5. B3: webui.Server optional runtime param (nil = old behavior); serve
   builds the runtime and injects; delete the webui→composition import
   (tailer.go:70); webui + root gates; commit.
6. B4: composition gate + serve smoke + pushdown-counter equality on a
   scratch DB; commit note.
7. A1: facts require (+replace per facade rules) → `journal.Fact` alias +
   `Detail []byte` sweep → journal module gate; commit.
8. A2: companion `scanFacts` re-point + tests; gate; commit.
9. A3: memory journal unified vocabulary; gate; commit.
10. A4: `journal/cqrs` adapter re-point (stays READ-ONLY); gate + ADR-0014
    pins; commit.
11. A5.1–A5.6 consumer sweep: webui (tailer, render) → sweepers (review,
    dlqfix, prioritize) → consumer + incident → cmd/tq (main, top, parked,
    ask, journalaudit) → queue.go signatures → root `-race`; commit.
12. A6: per-module gate loop; root build+vet+`test -race` (rc to file);
    smokes (legacy-serve-upgrade, serve) with scratch `TQ_DB`; ADR-0019
    addendum S2 DONE; commit.
13. A7: tag internal/journal + companion minors; `check-go-mods.sh`;
    facade parity BEFORE staging; `go mod vendor`; vendorHash if flake
    trips; ci-local; commit residue.
14. C1: deployment struct (DBPath, Driver, DSN, Pragmas, SyncPolicy,
    ReadModelEnabled + FromFlags-shaped ctor) + unit tests; commit.
15. C2: sqlitev4.Open consumes the struct; `TQ_SQLITE_SYNC` becomes a
    pragma override derived inside the struct; adapter tests; gates; vendor
    sync; commit.
16. C3: composition + readmodel consume the struct — one pragma source for
    both homes; delete the last literal; gates; commit.
17. C4: cmd/tq flags map onto the struct; `--store postgres://…` as data;
    end-to-end wiring; root `-race` + smokes; mark the Postgres CLI TODO
    row subsumed; commit.
18. C5: doctor/audit read the struct; AGENTS store-invariants + README
    config section + CHANGELOG; commit.
19. F3: ADR-0022 (three-lane config model + all-in gating on upstream v5);
    cross-link review + TODO rows; commit.
20. E1: README `TQ_POOL_CONFIG` section (flag > env > file precedence,
    agentpool.go:293) + parser hardening (unknown-key warn, typed decode) +
    tests; commit.
21. E2: agent-pool flag diet (53 registrations → file-configurable
    defaults, flags as overrides); docs; gates; commit.
22. E3: `scripts/check-flag-count.sh` pinning per-command counts; wire into
    ci-local; self-test pin; commit.
23. F1: webui tailer retirement eval (metaengine Watcher/ServeSSE vs hand
    tailer) + ADR-0003 addendum decide-and-record; commit.
24. F2: internal/queue/sqlite legacy thinning; conform wiring; module
    gates; root `-race` + smokes; commit.
25. F4: AGENTS.md single-opener rule + deployment-struct pointer (Store
    invariants); `TestAgentsDocSizeGuard`; commit.
26. Final sweep: full `scripts/ci-local.sh` + `scripts/root-gate.sh`; final
    status report; TODO_LIST harvest of anything new.

## g) QUESTIONS (cannot be self-answered)

1. **B2 verification bar:** the plan's B-track gate says "exactly one
   `sqliteengine` constructor call reachable from serve", but with today's
   upstream surface that forces either (a) dropping the engine from
   DeploymentConfig — which degrades the S4 root to the in-memory
   event-store fallback — or (b) waiting on the upstream seam. My
   recommendation: adopt the review P1a inversion NOW (ONE tq-owned opener
   - ONE shared pragma literal; the system-declared connection keeps
     running the identical union until the seam lands), and tighten the gate
     to literally-one-connection post-v5. Accept?
2. **A7 tag push:** the tag wave creates annotated tags for internal/journal
   - companion. Push them to origin when reached, or keep them local for
     your review first? (No push without an explicit yes.)

— Reported 2026-10-09 22:31; execution paused per instruction ("WAIT HERE").
