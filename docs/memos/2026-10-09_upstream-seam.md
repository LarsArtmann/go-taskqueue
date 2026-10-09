# Upstream Seam Memo: go-cqrs-lite system/ — tq projection-home integration

From: tq (go-taskqueue) config-system bridge execution, 2026-10-09
To: go-cqrs-lite maintainers
Pins in force at writing: system/v4 v4.11.0, sqliteengine/v4 v4.5.2,
metaengine/v4 v4.17.0 (tq vendor/modules.txt).

## Context

tq's S4 composition root (internal/composition) runs `system.New` over a
durable sqlite engine at the projection home (`<db>.readmodel.db`) with an
EMPTY domain by design — the queue engine's fact journal IS the journal, so
system's own event adapter stays unused. Separately, tq's readmodel (the
fold projector over the QUEUE journal) opens its own metaengine sqlite
engine on the same file. Today that is two constructors and two
connections to one WAL file with converging-but-duplicated pragma
literals.

We want ONE engine per file and ONE pragma source. The blocking surface
is small; everything below is verified against the pinned vendor source.

## Ask 1: engine-instance injection (the big one)

`DeploymentConfig.Engines` is data-only (`EngineConfig`: Driver, DSN,
Pragmas, Priority, MaterializedViews). `system.New` builds every engine
internally via `createEngineFromDriver` from the registry. There is no
way to hand system a CONSTRUCTED engine, and no `(*System).Engine(name)`
accessor afterwards (only `EngineNames()`); `MetaEngine()` returns the
store, not a named engine.

tq needs: `DeploymentConfig.Instances[i].Engine` (or Engines) to accept a
pre-built `metaengine.Engine`, e.g.

```go
type EngineConfig struct {
    // ... existing fields ...
    // Instance, when non-nil, is used as-is (Driver/DSN/Pragmas ignored;
    // system adopts but does NOT close it unless AdoptInstance=true).
    Instance metaengine.Engine
}
```

Why tq needs it: the alternative today is tq opening the engine itself
AND system opening a second decorative connection to the same DSN (the
current state), or system falling back to an in-memory event store when
we drop the declared engine (constructor.go's `sot.implicit_memory`
advisory path) — both worse. With injection, tq opens once, system
adopts, `Instance` roles resolve from the injected instance.

Secondary (falls out of Ask 1): `(*System).Engine(name) (metaengine.Engine, bool)`
for post-construction export — useful for hosts that want to hand the
root's engine to their own projection machinery without a second open.

## Ask 2: injectable projection event source (foreign journal)

tq's projections read a tq-owned journal (the queue fact journal), NOT a
go-cqrs-lite event store. Today readmodel implements its own
`event.SeekableJournal` adapter internally (internal/journal/cqrs is
PROPRIETARY read-only by ADR-0014/0019 — never extended downstream).

Ask: a documented, stable seam for running system's projection host (or
projectionhost directly) over a FOREIGN `event.SeekableJournal` — i.e.
make the empty-DomainConfig + projections-only deployment a supported,
non-screaming shape. Evidence this is currently second-class: with
`DomainConfig{}` and no engine declared, constructor.go logs
`sot.implicit_memory` and builds an in-memory event store that nothing
reads; the projection path works only because tq injects its own
row-source. A `DeploymentConfig.ProjectionSource event.SeekableJournal`
(or equivalent) would let external journals plug in without mirroring
events into a go-cqrs-lite store.

## Ask 3: engine pool-policy knob

`sqliteengine.NewSQLiteEngineFromDSNWith` hard-pins `MaxOpenConns(1)`.
That is the right default for the serialized-writer model, but consumers
need to (a) assert it (tq gates pin MaxOpenConns(1) as an invariant) and
(b) for read-mostly engines, potentially widen it. Ask: an
`EngineOption` for pool policy (`WithMaxOpenConns(n)`, or at minimum an
introspection accessor `(*sqliteEngine).MaxOpenConns() int`) so the
invariant is checkable instead of assumed.

## Non-asks

- No change to the WAL/busy_timeout prepend behavior — tq RELIES on it
  (it is half of our pragma union; see internal/composition/single_opener.md).
- No ask to make empty-DomainConfig deployments silent — only to give
  them a legitimate, documented shape (Ask 2).
- tq will keep its proprietary journal adapter regardless; this is about
  the system/ surface, not the event package.

## Priority

Ask 1 unblocks the literal "exactly one opener per file" gate tq's
architecture review wants; Ask 2 removes the split-brain smell
long-term; Ask 3 is hygiene. If only one lands: Ask 1.
