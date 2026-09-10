# ADR-0012: Store backends become driver modules (round-2 split)

**Status:** Accepted (2026-09-09)
**Context:** After ADR-0011's five-module split, the re-modularization
assessment scored every module Keep except `internal/queue` (cohesion 3,
too coarse): a 182-LOC contract (`Store`, `Filter`, the `Queue` facade,
sentinel errors) sharing one package — and one go.mod — with two full store
implementations (SQLite 1,614 LOC + modernc.org/sqlite; Postgres 1,334 LOC

- pgx/v5). Every contract-only importer (webui, httpapi, harvest, review,
  status, worker) compiled both dependency trees; and the Postgres store had
  zero production importers (CLI store wiring is a ROADMAP item), so its pgx
  tree was pure dead weight in every build.

**Decision:**

1. **Three modules under `internal/queue/`.** The contract stays at
   `internal/queue` (deps: task, journal — nothing else).
   `internal/queue/sqlite` and `internal/queue/postgres` are backend
   modules in the `database/sql`-driver style: package `sqlite` /
   `postgres`, concrete `Store`, constructor `Open`. The old
   `SQLiteStore`/`OpenSQLite`/`PostgresStore`/`OpenPostgres` names are
   retired — they would stutter in their new packages.
2. **Shared micro-helpers are mirrored, not extracted.** The backends
   share seven tiny utilities (`ms`, `mustJSON`, `maybeJSON`,
   `failureDetail`, `cancelReasonDetail`, `cooperativeCancelDetail`,
   `escapeLike`). Duplicating them per backend (postgres carries a
   one-for-one mirror file) keeps each store a self-contained twin — the
   same philosophy as ADR-0007's mirrored conformance suites. A shared
   utils package would re-couple what the split just separated.
3. **`WatermarkEntry` moves to the contract** — both backends' admin
   read returns it; it is contract material, not sqlite detail.
4. **Disk-derived module tooling.** ci-local.sh, ci.yml and release.sh
   enumerate `find internal -name go.mod` instead of hardcoded module
   lists: `queue/postgres` is required by no in-repo module (tidy would
   drop it from root), yet must stay gated, tested and tagged.
5. **Test-dep leaks, both documented:** worker requires journal (ADR-0011)
   and now queue/sqlite — its suites run against a real store; that is the
   repo's integration-testing stance, and both leaks pull no code into
   worker's production build graph.

**Consequences:** Contract importers stop compiling sqlite/pgx; the root
module drops pgx entirely until the Postgres CLI wiring lands (smaller
vendor set, smaller nix closure); queue's go.sum carries no external deps;
`go install` consumers can later choose a backend by import, not by module.
Tags: every internal module ships at the release version (release.sh cuts
`internal/queue/{sqlite,postgres}/vX.Y.Z` with the rest; v0.2.0 tags cut
with this change). The mirrored helpers and conformance suites remain the
ADR-0007 contract — the diff between backends must stay semantic-only.

**Alternatives rejected:** one combined `queue/backends` module (keeps the
dep coupling the split exists to remove); shared `storeutil` package
(re-couples the twins); package-only split inside the queue module (no
dependency isolation, half the payoff for half the ceremony);
extracting the Postgres store to a top-level module (it belongs under the
queue path — it IS a queue backend).
