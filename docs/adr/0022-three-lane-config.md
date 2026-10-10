# ADR-0022: Three-lane config — deployment, domain policy, CLI skin

Date: 2026-10-10
Status: Accepted
Depends on: ADR-0001 (facts-first), ADR-0011 (multi-module topology),
ADR-0016 (facades), ADR-0019 (go-cqrs-lite platform adoption), ADR-0020
(claim wake seam); landed by the 2026-10-09 config-system plan (C-track,
E-track).

## Context

Before the 2026-10-09 plan, tq's configuration was five smeared places
pretending to be one: the queue home's pragma literals lived in
`internal/queue/sqlite`, the projection home built its own set in
`internal/readmodel`, `TQ_SQLITE_SYNC` was read independently by each,
`--db`/`$TQ_DB` resolution was duplicated per command, and the pool's
config file parsed its own vocabulary. Every new surface (postgresv4,
the metaengine projection, a second pool) multiplied the drift risk:
two pragma lists could silently diverge, one env var could mean two
tiers, and nothing made "where does this setting live?" answerable.

The failure mode is architectural, not cosmetic: configuration that
crosses module boundaries must have exactly ONE owner per concern, or
every consumer re-derives it and they diverge under maintenance.

## Decision

tq's configuration has exactly three lanes, and every setting lives in
precisely one:

1. **Deployment lane — `config.Deployment`.** Where state lives and at
   what IO durability: driver (sqlite file / postgres DSN), path, sync
   tier, read-model toggle. ONE struct (`internal/config`), ONE env
   reader (`TQ_SQLITE_SYNC` via `config.SyncPolicyFromEnv`), ONE pragma
   builder per home (`QueuePragmas`, `ProjectionPragmas` — the literals
   exist nowhere else), ONE CLI frontend (`--store` on the long-running
   surfaces, `--db` as the legacy sqlite path everywhere else, resolved
   through `config.FromFlags`). Every opener takes the struct:
   `sqlite.OpenWithDeployment`, the composition root's `New` and
   `NewProjectionRuntime`. Doctor/audit build it through the same
   constructor and refuse postgres until live-instance verification.

2. **Domain policy lane — Go `Config` structs.** How the domain
   behaves: `harvest.Config`, executor payload structs, worker options,
   budget knobs. These are plain structs constructed at the composition
   root (or the pool's option parser); they never read the environment
   directly and never parse flags — their callers translate. A domain
   struct with an `os.Getenv` inside is a lane violation.

3. **CLI lane — the skin.** Flags, usage text, pool.conf keys, and
   service-unit env names are presentation over the other two lanes.
   They map ONTO structs (`storeFlag` → `FromFlags`; pool.conf keys are
   the flag spelling applied via the FlagSet) and carry no semantics of
   their own. Flag growth is gated: `scripts/check-flag-count.sh` pins
   per-command registration counts in ci-local, so widening the skin is
   a conscious, commit-documented act; the pool file exists precisely so
   the skin doesn't have to grow (flags stay as one-shot overrides).

**All-in gating on upstream v5.** The deployment lane's struct is
deliberately `DeploymentConfig`-shaped: when go-cqrs-lite v5 ships the
`system` composition root's `DomainConfig` + queue-instance work (the
D-track seam memo, owner-blocked), tq's `config.Deployment` converges
onto it and the projection home stops being a tq-side concept. Until
that tag exists, the lane stays proprietary and complete: no half-mapped
fields, no parallel tq-side DSN parser.

## Consequences

- One pragma change is one edit in `internal/config` with the conform
  suites as the contract test surface — no consumer sweep.
- New storage backends become a `Driver` + a pragma builder + an opener
  branch; nothing else in the repo learns the DSN syntax.
- The CLI lane can be re-skinned (new command names, config-file keys,
  service env) without touching the domain or deployment lanes; the
  flag-count guard keeps the skin's width visible.
- Env-var sprawl is capped by doctrine: a NEW environment variable
  needs a lane-1 justification (deployment contract like
  `TQ_SQLITE_SYNC`) or it belongs in a domain struct wired from the
  pool config file.
- The v5 convergence is a data-shaped refactor, not a re-architecture —
  the struct boundary this ADR pins is what makes the all-in swap
  cheap.
