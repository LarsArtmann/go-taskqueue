# ADR-0011: Five-module split — the library core becomes sub-modules

**Status:** Accepted (2026-09-09)
**Context:** The repo grew to 19 packages under a single go.mod. The package
DAG was clean, but nothing enforced it: a `queue` import of `internal/webui`
or a `worker` import of `internal/harvest` would compile without comment.
Layer discipline lived only in review attention, and every `./...` gate
treated all 19 packages as one undifferentiated unit. Modularization also
grounds ADR-0001's promise that Go users embed the queue ("register executor
funcs directly"): the embeddable core can now be consumed without the app
layer.

**Decision:**

1. **Five sub-modules, paths unchanged.** `internal/task`, `internal/journal`,
   `internal/queue`, `internal/executor`, and `internal/worker` each carry
   their own go.mod at their existing import path
   (`github.com/larsartmann/go-taskqueue/internal/<pkg>`). Zero import
   rewrites; the "internal/ until the API stabilizes" stance (ADR-0001/0002)
   is preserved exactly — nothing is importable from outside the repo tree.
2. **The app layer stays one root module.** cmd/tq, webui, httpapi, harvest,
   review, status, bridges, budget, consumer, runactor, e2e, examples remain
   in `github.com/larsartmann/go-taskqueue`. They co-change constantly
   (queue|webui 19, executor|harvest 15 co-changes in a 5-week window) and
   have no independent consumer — module boundaries there would be ceremony.
3. **Replace-directives only; no go.work.** Root pins every sub-module at
   `v0.0.0` with a relative `replace` (`=> ./internal/task`); sub-modules do
   the same for their siblings (`=> ../task`). Replace alone fully determines
   resolution, so there is no go.work/replace drift to police, and nix, CI,
   and developer builds are identical. This deviates from the dual
   go.work+replace default: the buildflow-managed .gitignore excludes
   go.work, and eliminating the second source of truth removes the drift
   failure mode entirely.
4. **Versioning: shared version, replace-gated.** A release bumps the root
   and every `internal/<pkg>/vX.Y.Z` subdirectory tag together (a lone root
   tag cannot satisfy a sub-module on the proxy). Until any module is
   promoted out of internal/, nothing is ever fetched — replace decides.
   `scripts/release.sh` must grow the subdirectory tags before any promotion.
5. **Every gate gets a per-module counterpart.** `./...` never descends into
   nested modules, so root gates keep passing while silently skipping the
   five modules. ci-local.sh and ci.yml (both Linux and Windows jobs) run
   `GOWORK=off` build/vet/test per module, cross-compile each module for
   windows, audit replaces for absolute paths, and audit internal requires
   stay pinned at v0.0.0. Fuzz campaigns run from inside their package dir.
   The agent pool's default Go verify command walks nested go.mod files.

**Consequences:** The layer DAG is compiler-enforced — an upward import now
fails with a missing require/replace, not a review note. `worker` requires
`journal` for tests only (the one documented test-dep leak; journal is
stdlib-only so it bloats nothing). Per-module `go mod tidy`/`go.sum` are now
independent (queue carries sqlite+pgx in its own go.sum). The nix vendorHash
covers the root module's vendored view (including replaced sub-module
sources) and changes with any go.mod/go.sum. golangci-lint runs per module
(advisory, as before).

**Alternatives rejected:** 19 module-per-package splits (single-consumer
packages like runactor/budget/consumer/bridge have no composability payoff);
go.work + replace dual strategy (drift surface with no benefit here);
promoting packages to public paths (API-stabilization decision, out of
scope — see ADR-0002).
