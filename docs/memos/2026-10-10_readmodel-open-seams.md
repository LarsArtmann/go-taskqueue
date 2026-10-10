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

## Residual resolved (2026-10-11): the api pump adopts the managed host

TODO row "Give `tq api`'s long-running readmodel pump the managed host's
protection" — DECIDED: ADOPT. `httpapi.UseReadModel` now builds the pump
protection directly from internal/readmodel (the same platform seams the
composition root bundles for serve), and `ListenAndServe` drives
`Host.Start`/`Stop`/`Close` instead of the raw `Model.Run` loop:

- best-effort DLQ sidecar (`readmodel.OpenDeadLetters` over the
  deployment, warn-and-continue): a non-retryable poison fact
  dead-letters, the checkpoint advances past it, the fold keeps
  converging;
- platform restart budget (default 5, backoff 1s–30s): a retryable
  failure restarts under backoff, then fails LOUDLY (`WorkerFailed`)
  instead of the old silent forever-loop of "tail poll failed" at a
  wedged cursor.

The decision above STANDS: no `NewProjectionRuntime` adoption, no
composition import, no double-close — the model stays self-opened and
the api takes only the two host seams it needs. The delete-the-file
replay escape hatch keeps working: when the model's durable-cursor load
decides replay-from-zero (empty projection under a surviving
checkpoint), `UseReadModel` runs one serial `CatchUp` to converge the
home and re-seal the checkpoint before the host's own checkpoint load
would resume past the missing rows.

Follow-up found while landing this (not fixed here): the doctor's
projection-dlq check and `OpenDeadLetters` derive DIFFERENT sidecar
paths (`DLQPathFor(PathFor(db))` vs `DLQPathFor(db)`) — queued as its
own TODO row.
