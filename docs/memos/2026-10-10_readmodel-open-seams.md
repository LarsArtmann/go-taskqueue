# Readmodel open seams: `tq api` keeps the self-open fallback; internal/composition needs no facade

2026-10-10 · decision record for TODO row "Decide `tq api`'s readmodel open"
(source report: docs/status/tasks/2026-10-10_06-32_task-000001a12409809846dec087009600000000.md §f3)

## Decision

`tq api` does NOT adopt `composition.NewProjectionRuntime`. Its readmodel
open stays the self-open fallback (`readmodel.Open` with
`WithDurableCursor`, internal/httpapi/httpapi.go `UseReadModel`) — the
sanctioned per-surface exception, same shape as `tq stats` / `tq doctor`
pending their own row's ruling.

## Rationale

1. **The api needs only the Model.** `NewProjectionRuntime` bundles
   engine + model + DLQ sidecar + managed projection host. The api runs
   its own pump (`Model.Run` in `ListenAndServe`) and never starts a
   host; adopting the runtime hands it a never-started host and a DLQ
   sidecar its pump cannot route to (poison routing is a host option),
   plus a double-close (httpapi closes the model; `Runtime.Close` would
   close it again).
2. **No correctness gap in the open itself.** Each process (serve / api
   / stats / doctor) opens the projection home exactly once — no
   concurrent double-open. The self-open derives its caller pragmas from
   the repo's ONE pragma source (`ProjectionHomeCallerPragmas` →
   `config.Deployment.ProjectionPragmas`, internal/readmodel/model.go),
   the same source the composition root's engine build uses. The open
   question of whether the zero-deployment self-open resolves the
   `TQ_SQLITE_SYNC` tier is its own queued row — not re-decided here.
3. **Layering precedent.** serve keeps composition OUT of surface
   packages: cmd/tq adapts `ProjectionRuntime` to the narrow
   `webui.ProjectionPump` seam (fields, not methods) precisely so webui
   does not import composition. Importing composition into
   internal/httpapi would invert that layering for zero gain.
4. **ADR-0016 facade-vs-internal-import, resolved for the record:** the
   question is moot under this decision, and it stays answerable should
   any surface later adopt the runtime — internal/composition is an S4
   app-composition module, not a library surface: no facade exists, and
   none is warranted. In-repo (root-module) code imports `internal/…`
   directly per ADR-0016 ("facades are the external contract; root-module
   code does NOT switch to facades"). internal/httpapi is root-module
   (no own go.mod), and the root go.mod already requires+replaces
   internal/composition — a direct import would be legal and
   policy-clean. The only caution is rationale 3: prefer the cmd/tq
   adapter pattern over surface-package imports.

## Residual (backlog, not this decision)

The api's raw `Model.Run` pump lacks the projectionhost restart budget
and poison-fact DLQ that serve's composed host provides: a poison fact
wedges the cursor at repeated error logs (never skipped, never
quarantined). Queued as its own TODO row; it is a lifecycle gap, not an
open-seam one.
