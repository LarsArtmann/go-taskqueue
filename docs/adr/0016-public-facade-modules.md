# ADR-0016: Public facade modules over the internal library core

**Status:** Accepted (2026-09-13)
**Context:** An external adoption attempt (docs/feedback/new/
2026-09-13_external-adoption-blocked-by-internal-paths.md) failed on pure
structure: every library module lives under `internal/`, so Go's
internal-package rule forbids any import from outside the repo tree. The
evaluator read the README, ADRs, and the Store contract before discovering
no importable path exists, then rebuilt a ~500-LOC fork of the semantics.
ADR-0011 deliberately deferred promotion ("API-stabilization decision"),
but the effect is that the tagged, tested, released library core is
unreachable in practice — tags without importability are releases in name
only.

**Decision:**

1. **Facade modules, not promotion.** Each consumer-facing internal module
   gains a public facade module at the idiomatic path that re-exports its
   ENTIRE exported surface via type aliases (`type X = internal.X`) and
   var/const re-exports:

   | Facade                          | Re-exports                |
   | ------------------------------- | ------------------------- |
   | `.../task`                      | `internal/task`           |
   | `.../journal`                   | `internal/journal`        |
   | `.../queue`                     | `internal/queue`          |
   | `.../queue/sqlite`              | `internal/queue/sqlite`   |
   | `.../queue/postgres`            | `internal/queue/postgres` |
   | `.../executor`                  | `internal/executor`       |
   | `.../worker`                    | `internal/worker`         |

   The internal-package rule is path-prefix based: a facade at
   `github.com/larsartmann/go-taskqueue/queue` shares the
   `github.com/larsartmann/go-taskqueue/...` prefix with `internal/queue`,
   so the import is legal — no matter which module the two packages live
   in. Facades are the smallest change that makes the library consumable
   without a big-bang API freeze: implementations stay internal and free
   to refactor, only NAMES are pinned.
2. **Type aliases, not wrappers.** `type Store = internal.Store` preserves
   the identity, every method, and interface satisfaction — a facade
   `queue.Store` IS an `internal/queue` `Store`, so the two backends, the
   worker, and any in-repo consumer stay interoperable with zero glue.
   Wrappers (methods forwarding one by one) would be hundreds of lines of
   drift surface for nothing.
3. **Versioning follows ADR-0011 exactly:** each facade is its own module,
   requires its internal counterpart at a real tagged version, and carries
   a relative `replace` for in-repo dev. Release tooling tags facades
   alongside the internals (the module enumeration is extended from
   `find internal -name go.mod` to cover the facade dirs; release.sh,
   ci-local.sh, ci.yml, flake.nix test loop, check-go-mods.sh,
   lint-baseline.sh all share the disk-derived list).
4. **Surface parity is a review convention, not a compiler guarantee:**
   an alias list can silently miss a newly added internal export. ADR
   note: when adding an exported symbol to a facaded package, add the
   alias in the same change (the facade files are the checklist). A
   hermetic parity test was considered and rejected — enumerating the
   internal package's exports from inside `go test` requires
   source-parsing or the go toolchain on PATH, neither of which is
   guaranteed hermetic in the nix checkPhase.
5. **Postgres pool reuse ships with the facades:** `postgres.OpenWithPool`
   accepts a caller-owned `*pgxpool.Pool` (schema applied, store wrapping
   it), so consumers with an existing pool do not inherit a second one —
   the external evaluator's explicit ask.

**Consequences:** External consumers embed the queue today:
`go get github.com/larsartmann/go-taskqueue/queue/...` resolves through
the proxy using the same tags the internals already ship. The facade files
are pure re-exports (one file per module, ~100-200 lines), so per-module
gates stay cheap. The internal paths remain the in-repo import style —
root-module code does NOT switch to facades (zero-churn, per ADR-0011's
"paths unchanged" principle); facades are the external contract. When the
API stabilizes, promotion (moving implementations to public paths) remains
available and is non-breaking for facade consumers because aliases keep
resolving — only the internal paths would retire.

**Alternatives rejected:** promote all packages now (freezes the API
prematurely, violates the ADR-0001/0011 stance for zero additional
consumer value); one god-facade at the repo root (collides with the root
app module path, which has no Go files but owns the tq binary — mixing
library and app surfaces at one path is exactly the blur the module split
removed); wrappers instead of aliases (drift surface, identity split:
`facade.Store` would not satisfy `internal/queue.Store`); waiting for
stabilization (the evaluator forked and drifted; adoption-blocking costs
are paid per evaluator, not once).
