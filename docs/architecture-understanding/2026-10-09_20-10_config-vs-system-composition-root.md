# Configs vs `system/`+`metaengine/` — should the next major go all-in?

**Review type:** architecture review (config & composition focus)
**Date:** 2026-10-09 20:10
**Question (owner):** "do we even want these configs or should we go all in on
`go-cqrs-lite/metaengine/` and `go-cqrs-lite/system/`? aka next major versions
system"
**Series context:** first review in this directory that is not a
module-structure map (prior: `2026-09-09_20-14_module-structure*`,
`2026-09-10_module-structure*`). Cross-referenced: ADR-0019 (platform
adoption + endgame addendum), ADR-0014 (journal adapter), ADR-0016 (facades),
ADR-0017 (cmd/tq floor), TODO_LIST "go-cqrs-lite platform adoption" section.

---

## Verdict

**All-in is the destination, not the next step.** Going all-in on
`system.New(DomainConfig, DeploymentConfig)` as _the_ configuration model is
the correct endgame — it is literally the founding intent (ADR-0019 owner
ruling 2026-09-22: "the whole idea of this project was that it uses
go-cqrs-lite system/ + metaengine/") and it is where upstream v5 is going
(cqrs-lite ADR-0123). But a literal all-in **today** is blocked by two
concrete upstream seams, one tq-side split-brain, and one pending stage (S2).
Meanwhile, "these configs" are 90% domain policy that `system` has no slot
for — and should not. The right move is a three-lane config model plus a
short, serialized bridge (roadmap below).

| Config lane                                                                        | Owner                     | Fate                                                        |
| ---------------------------------------------------------------------------------- | ------------------------- | ----------------------------------------------------------- |
| Engine/deployment axes (driver, DSN, pragmas, sync policy, pool policy)            | `system.DeploymentConfig` | **Consolidate here** — today smeared across 5 places        |
| Domain/operational policy (lease, backoff, budget, sweeper cadence, agent runtime) | tq's Config structs       | **Keep** — `system` must never grow slots for these         |
| CLI/UX surface (253 flag registrations, ~126 distinct)                             | cmd/tq flags              | **Keep as skin** — becomes a thin mapper onto the two lanes |

---

## 1. What "these configs" actually are (inventory)

- **253 flag registrations across 30 subcommand flagsets; ~126 distinct
  flags** (worst: `agent-pool`, 53). Registered per-command with
  `flag.NewFlagSet` (cmd/tq/main.go:257–3500, agentpool.go:82, bootstrap.go:196,
  doctor.go:1703, audit.go:20, tasks.go:22, …).
- **16 `Config`/options structs** in the app layer, ~120 fields total —
  worker (14 fields, internal/worker/worker.go:24), harvest (22,
  internal/harvest/harvest.go:123), webui (9), bridges (papdashboard 10+6, cqa
  8), sweepers (4–7 each), consumer/watermark/lockout (4–5 each), readmodel
  `ProjectionHostOptions` (5) + functional `Option`s.
- **~20 env vars** read directly (`TQ_DB`, `TQ_SQLITE_SYNC`, `TQ_POOL_CONFIG`,
  `TQ_REDACT`, `TQ_PAP_*`, `CRUSH_SESSION_ID`, …).
- **Engine/DSN policy in code, not config**: sqlitev4 hardcodes the DSN
  pragma chain incl. `MaxOpenConns(1)` (internal/queue/sqlitev4/adapter.go:120–131);
  only escape is `TQ_SQLITE_SYNC` (adapter.go:93–106). postgresv4 takes a DSN,
  pools fixed at 1 + caller-owned `OpenWithPool` (adapter.go:117–140).

Key observation: **only a sliver of this is deployment-shaped.** Roughly 10 of
126 distinct flags (db path, read-model toggle, addrs, auth tokens,
`TQ_SQLITE_SYNC`) select _where/how things run_. Everything else is domain
policy: lease TTL, requeue ladders, budget caps, harvest cadence, model
selection for AI turns. `system.DeploymentConfig` models exactly the first
sliver (Engines: driver+DSN+pragmas+priority+materialized views; Buses;
Instances by role — system/config_types.go:139–215) and `DomainConfig` is Go
closures for command/query/projection registration (config_types.go:20–126) —
**not** a policy-value bag. Collapsing tq's policy structs into either would
be a category error.

## 2. What the `system/` root actually owns in tq today (S4 as shipped)

`internal/composition` is the sanctioned S4 shape (compose.go:1–63): ONE
`DeploymentConfig` with a `"projections"` sqlite engine over
`<db>.readmodel.db`, `RoleProjections` instance, and — by justified design —
an **empty `DomainConfig`** (declaring tq collections as system projections
would mirror the queue journal: second source of truth, forbidden by
facts-first; ADR-0019 endgame addendum).

But traced through the wiring, the root is currently a **lifecycle shell**:

- `tq serve` opens it only for close ordering: `sys.GracefulClose` joined
  after the store close in the shutdown hook (cmd/tq/main.go:3542, :3560).
- The **load-bearing** projection engine on the _same file_ is opened
  elsewhere: `readmodel.Open` builds its own `sqliteengine` with hardcoded
  `synchronous=NORMAL`, `cache_size=-32768` (internal/readmodel/model.go:130),
  reached via `webui` → `composition.NewProjectionRuntime`
  (internal/webui/tailer.go:70).
- Result: **two engine instances on one projection home with two divergent
  pragma sources** (compose.go:46–49 vs model.go:130–131). Harmless at WAL
  today, but it is a split-brain in exactly the axis `DeploymentConfig` was
  supposed to make single-sourced — the deployment decision is declared in
  the root and then ignored by the component that actually runs.

Also note a layering wrinkle: `internal/webui` (a presentation package)
constructs the projection runtime. The runtime should be injected into the
server, not built by it.

## 3. What blocks literal all-in (evidence)

1. **The queue store family is not a `system` engine.** `system/` has zero
   imports of `queue/v4` or its drivers (grep of
   `/home/lars/projects/go-cqrs-lite/system/`, 2026-10-09). `RoleSourceOfTruth`
   requires a `metaengine.StreamLogBackend` (roles.go:129); the tq queue
   engines (`queue/sqlite/v4`, `queue/postgres/v4` ridden by sqlitev4/
   postgresv4 + companion) are a different family with claim/lease/DLQ
   semantics system cannot declare, pool-policy included — `EngineConfig`
   has no `MaxOpenConns`-class knob (config_types.go:198–215), and tq's
   single-serialized-writer invariant (MaxOpenConns(1), AGENTS.md store
   invariants) is driver-internal policy.
2. **Projections are bound to system's own event store.** The system
   constructor derives the projection host source from `sys.eventStore`
   (constructor.go:256). tq's folds must consume the **queue journal** (the
   one journal). Until `system` grows an injectable projection event source
   (fold from a foreign `event.SeekableJournal` without mirroring), tq's
   readmodel cannot move into `DomainConfig.Projections` — which is precisely
   why the empty-DomainConfig deviation exists.
3. **`system` is upstream-experimental** (go-cqrs-lite FEATURES.md §"System
   Package 🧪 EXPERIMENTAL"; v5 renames constructors per ADR-0123). tq's next
   major should target the **v5 cut**, not chase v4-experimental renames.
4. **S2 is still pending**: `journal.Fact` still shadows upstream
   `facts.Fact` (ADR-0019 addendum; TODO_LIST row unblocked 2026-10-05). Any
   deeper system-side projection adoption wants the unified vocabulary
   first.

## 4. Scores (rubric, evidence-cited)

| Dimension            | Score | Evidence                                                                                                                                                       |
| -------------------- | ----- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Coupling             | 4     | `queue.Store` boundary + companion shared surface + facades; system reached only through `internal/composition`; −1: webui→composition reach-in (tailer.go:70) |
| Cohesion             | 4     | Packages single-purpose (per ADR-0019 table); −1: cmd/tq agent-pool's 53 flags concentrate pool+executor+sweeper policy in the app layer                       |
| Modularity           | 4     | Multi-module enforced (ADR-0011), facade parity gated, conformance shared; boundaries mostly clean                                                             |
| **Composability**    | **3** | Deployment axes smeared over 5 places (§5); system root decorative (§2); queue store injectable but not declarable; the one place this review targets          |
| Scalability          | 4     | Engine-adopted store; new backend (mysql) free via conformance suite; read pushdowns on planned tables                                                         |
| Service orientation  | 3     | Single binary **by design** — monolith with clean module boundaries, extraction-ready; not a defect                                                            |
| Dependency direction | 5     | `internal/task`/`internal/journal` stay pure (ADR-0014 D2); adapters point inward; `journal/cqrs` read-only                                                    |

**Average 3.86 — "Good":** address the lowest dimension (Composability); no
urgent restructuring. This matches the prior module-structure reviews' read
of the repo; the friction is concentrated, not systemic.

## 5. The deployment axis, enumerated (the part that should be all-in)

Today one conceptual axis — "which engines, which files, what durability" —
lives in five places:

| Where                            | What it decides                             | file:line                                    |
| -------------------------------- | ------------------------------------------- | -------------------------------------------- |
| sqlitev4 `openSharedDB`          | queue DSN pragma chain, MaxOpenConns(1)     | internal/queue/sqlitev4/adapter.go:120–131   |
| `TQ_SQLITE_SYNC` env             | sync policy escape hatch                    | adapter.go:93–106                            |
| composition `DeploymentConfig`   | projection-home engine + pragmas            | internal/composition/compose.go:41–55        |
| readmodel `Open`                 | the _real_ projection-home engine + pragmas | internal/readmodel/model.go:130–131          |
| postgresv4 `Open`/`OpenWithPool` | DSN + fixed pools / caller-owned pool       | internal/queue/postgresv4/adapter.go:117–140 |

`system.DeploymentConfig` is the natural single home for this lane — it is
koanf-loadable (flags/env/YAML today, file later), validated at construction,
and already the sanctioned pattern for "swap sqlite → postgres by editing
Engines, never domain code" (compose.go:11–12).

## 6. Roadmap

Serialized; P0 first (it is also just ADR-0019's own remaining stage).

| #       | Action                                                                                                                                                                                                                                                                                                                                                                             | Where         | Effort                              | Depends on                                                         |
| ------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------- | ----------------------------------- | ------------------------------------------------------------------ |
| **P0**  | Ship **S2**: unify journal on upstream `facts.Fact`, re-point `journal/cqrs`, tailer, sweepers, bridges                                                                                                                                                                                                                                                                            | tq            | M                                   | — (already unblocked; prerequisite for any deeper system adoption) |
| **P1a** | **Kill the projection-home double-open**: exactly one engine owner for `<db>.readmodel.db` — hand the composition root's declared engine to `ProjectionRuntime`/`readmodel` (or, if the engine accessor is awkward, invert: readmodel owns it and the root references it) and delete the divergent pragma copy                                                                     | tq            | S                                   | P0 not strictly required; do now — it is today's split-brain (§2)  |
| **P1b** | **File the upstream seam ask** (sibling of the pending ratification memo): (i) injectable projection event source — fold from a foreign `SeekableJournal` (the queue journal) without mirroring; (ii) queue-family instances in `DeploymentConfig` (or a documented queue-store escape hatch); (iii) engine pool-policy knob sufficient for the single-serialized-writer invariant | go-cqrs-lite  | S (filing) / M–L (upstream landing) | owner go (mirrors existing memo row)                               |
| **P2a** | **One tq deployment struct**: single source for db path/driver/DSN/pragmas/sync policy that feeds sqlitev4 (or postgresv4), the composition root, and readmodel; `TQ_SQLITE_SYNC` folds in as a pragma override; `--store postgres://…` becomes a data change, killing the owner-blocked CLI-wiring TODO                                                                           | tq            | M                                   | P1a                                                                |
| **P2b** | When P1b lands + system v5 stabilizes: swap the seams — tq's folds move into `DomainConfig.Projections` (fed from the queue journal), queue engine declared as an instance; flags become a thin mapper onto Domain(deploy) lanes; delete the S1 thin-driver hand-wiring it replaces                                                                                                | tq + upstream | L                                   | P0, P1b, v5 cut                                                    |
| **P3**  | Flag diet for the worst surface: grow `TQ_POOL_CONFIG` (already read, agentpool.go:293) into the documented pool-config file; keep flags as the interactive skin                                                                                                                                                                                                                   | tq            | S–M                                 | P2a (rides the same config lane)                                   |

**Explicitly rejected:** deleting tq's Config structs in favor of
`DomainConfig` (category error — it is registration closures, not policy), and
mirroring the queue journal into a system event store to make
`DomainConfig.Projections` work today (violates facts-first / single journal,
ADR-0019's own ruling).

## 7. Answer, one paragraph

Keep the configs — most of them are tq's domain, and `system` deliberately
has no vocabulary for them. Go all-in on the _deployment lane_: that is what
`system.DeploymentConfig` is for, and today that lane is smeared across five
places including a live double-open on the projection home. The full
`system.New`-owns-everything picture is the right next-major destination and
is gated on upstream (external-journal projections, queue instances,
non-experimental v5) plus tq's own S2 — both of which are already on the
serialized ADR-0019 path. Ship S2, fix the double-open now, file the seam ask,
and the eventual "all-in" becomes a wiring swap instead of a rewrite.
