# tq toolsdk provider for BuildFlow — evaluation and decision (2026-11-07)

Source: docs/status/2026-11-06_04-20_docs-gates-adoption-buildflow-doctrine.md §f4.
Verdict: **DELIBERATE NON-FIX for now** — the machine face already exists, the
provider is a thin shim, but nothing consumes it yet and the cost side
(binary floor, registry wiring) is real. Re-evaluate on the trigger below.

## What the provider would be

A `pkg/provider` module in go-taskqueue (the art-dupl pattern:
`art-dupl/pkg/provider/provider.go` registers one `toolsdk.Spec` via
`toolsdk.Register`, consumed by BuildFlow's
`BuildFlow/tools/providers/sdk_imports.go` blank import). The spec:

- `Name: "tq-debt"`, trigger `AnyLanguage` (or `OnGoModule` — tq runs Go
  repos only today).
- Detect shells the tq CLI (never import `internal/*` — the provider must
  decouple from the store, the binary is the contract):
  - `tq dlq --json --db <repo db>` → `dlqJSONRow` stream (fields verified
    live on a scratch db 2026-11-07: `project`, `type`, `attempts`,
    `last_error`, `evidence`).
  - `tq tasks --project <name> --status dead --json` → dead rows citing
    the repo.
- Converts each row to one go-finding Finding: rule id
  `tq-debt/dead-task` / `tq-debt/dlq`, position = repo root file 0 is
  FORBIDDEN (real positions only) — so the finding anchors on the repo's
  go.mod, severity `warning`, confidence `high` (a dead task is a fact,
  not a heuristic).

## Machine face: EXISTS (verified)

- `tq tasks [--project P] [--status S] [--json]` — `null` on empty
  (verified live, scratch db).
- `tq dlq --json` — `{"count":0,"total":0,"evidence_read":true}` lead
  summary + row stream (verified live).
- Both carry `project`; no new tq surface needed for a first provider.

## Why non-fix (deliberate)

1. **No consumer pull yet.** BuildFlow runs are per-repo dev loops; DLQ
   debt is already surfaced to operators where they actually look: `tq
   dlq`, the webui timeline, `pool-health`, and (since item 1 of this
   batch) the docs-gates bridge mints fix tasks from gate findings. A
   BuildFlow finding would duplicate that surface in a place nobody
   checks for queue state.
2. **Binary floor problem.** The provider shells `tq`, so every
   BuildFlow-covered consumer repo needs a tq binary ≥ the `dlq --json`
   contract version on PATH; a stale binary must fail LOUD (the
   docs-gates stale-binary lesson, AGENTS Known Issues). That version
   negotiation is design work, not a shim.
3. **Semantics unresolved.** Which counts are "debt"? Dead tasks age
   out; DLQ rows carry `dlq_backoff_until` (paused, not dead); the
   owner's bar ("dead/DLQ tasks citing the repo") needs a ruling on
   whether parked/requeued tasks count before a rule id can be stable
   (a shipped rule ID never changes — linter-building core rule).

## Build plan when the trigger fires

Trigger: an owner wants queue debt visible inside `buildflow run` (or a
consumer repo repeatedly rots from unattributed dead tasks). Then:

1. Owner ruling on the debt definition (which statuses/counts).
2. New module `provider/` (own go.mod, deps: toolsdk ≥ v1.15.0 +
   go-finding), `//go:build unix`, blank-import wired in BuildFlow's
   sdk_imports.
3. Pin the `dlq --json` / `tasks --json` contract with a consumer-side
   guard test against a fixture stream (the stale-binary loud-fail).
4. Table-driven tests over fixture JSON; hermetic (no live pool).

No TODO_LIST row beyond this record's pointer — the build plan lives here
and graduates on the trigger.
