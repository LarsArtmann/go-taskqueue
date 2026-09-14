# ADR-0018: templ-components consumer sweep — fan-out home and license direction

Status: accepted (G1 default; owner may override)
Date: 2026-09-14
Deciders: execution of
[docs/planning/2026-09-14_12-34_templ-components-consumer-sweep-execution-plan.md](../planning/2026-09-14_12-34_templ-components-consumer-sweep-execution-plan.md)

## Context

The owner wants a library-maximization sweep ("adopt latest templ-components,
use it to the MAX") across all ~18 consumer repos reported by
project-dependency-graph (pdg) `who-uses`, executed by tq agent tasks. The
fan-out engine needs a home, and two verification results constrain it:

1. **pdg is PROPRIETARY** ("All rights reserved") and has **no sub-module
   tags** (root `v0.1.0`/`v0.2.0` only). Importing its discovery/graph SDK
   into tq-adjacent MIT tooling is license poison (the httputil precedent)
   and proxy-unresolvable besides.
2. **tq's public facades ARE tagged on the proxy** (v0.3.0 family, ADR-0016).
   MIT code may be consumed by proprietary code freely — so a bridge could
   legally live inside pdg, importing
   `github.com/larsartmann/go-taskqueue/queue` + `queue/sqlite`.

## Decision

**G1 default took effect: the fan-out lives in tq as a zero-dependency
script — `scripts/sweeps/fanout-libdive.sh` — driven by a committed cohort
fixture (`scripts/sweeps/templ-components-cohort.tsv`) and a version-pinned
prompt template (`scripts/sweeps/templ-components-prompt.tmpl`).**

License direction (unchanged from ADR-0016's world): tq stays MIT and
consumable from proprietary code through the facade modules. No pdg code
enters this repo. No pdg repo edits without explicit owner approval.

## Rationale

- The script is the reversible choice: if the owner later approves the
  pdg-resident bridge, the fixture, dedup keys
  (`libdive:templ-components:<key>@<version>`), and template carry over
  unchanged; the script retires and M8/M9 (pdg `who-uses --format json` +
  fan-out command over tq facades) become the engine.
- Idempotency is the core fan-out property: the CLI gained
  `tq enqueue --dedup-key` (same-key re-enqueue returns the stored task), so
  re-running the script never duplicates spend. An unchanged key set mints
  nothing.
- The script hard-requires `--db` — an inherited `TQ_DB` (the production
  dogfood journal) can never receive sweep mints by accident.

## Consequences

- Waves are staged by cohort fixture (`wave` column), staggered with
  `--delay`, and serialized (`--agents 2`) against the ONE shared provider
  account — see the 429 runbook in the execution plan §8.2.
- The who-uses tree parse is pinned by the script's `--self-test`
  (glyph-robust); the fixture stays the verified source of truth for
  dirs/pins (who-uses resolved versions diverged from go.mod requires once:
  Rolls-Royce v1.17.0 resolved vs v1.16.0 required).
- If sweeps become permanent rails (G3 = option C), revisit: the script
  graduates into the pdg bridge (M9) or into harvest-native cohort support,
  and this ADR gets a superseding decision.
