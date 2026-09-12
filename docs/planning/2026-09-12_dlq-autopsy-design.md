# DLQ Autopsy — the self-fixing dead-letter queue

Status: DESIGN → IMPLEMENTED 2026-09-12 (this document describes the shipped
shape; deviations are annotated inline).

Idea origin: owner TODO item "self fixing dead-lettered queue with AI Agents?"
(added to TODO_LIST.md 2026-09-12).

## Problem

Dead-lettered tasks are where autonomous work goes to be forgotten. The
machinery already records WHY a task died — `task.failed` facts carry
`executor.FailureEvidence{stage, exit_code, tail}` and the dead task keeps its
full fact trail — but turning that evidence into a decision (fix it? discard
it?) is a human-only step. The pool's cheapest failure mode is therefore a
silently growing DLQ.

## Concept

One sentence: **when a task dead-letters, mint ONE agent "autopsy" task in the
same repo; the autopsy agent diagnoses the failure from the journal's own
evidence, either applies the fix and rescues the original, or rules the task
unfixable with a reasoned verdict that dismisses it from the DLQ.**

The loop is journal-driven and symmetric:

    task.dead-lettered (agent type) ──mint──▶ dlqfix task ──runs autopsy agent──▶
        verdict "fixed"   ──▶ store.RescueDead(dead task)   (Dead → Pending, fresh budget)
        verdict "wontfix" ──▶ store.DismissDead(dead task)  (Dead → Cancelled, reason fact)

Everything downstream of the verdict is mechanical. The agent only contributes
the diagnosis — the same division of labor the review sweeper established.

## Components

| Piece                          | Home                       | Shape                                                       |
| ------------------------------ | -------------------------- | ----------------------------------------------------------- |
| `TaskTypeDLQFix`, payload, result, strict verdict parser, autopsy prompt | `internal/executor/dlqfix.go` (executor sub-module) | Mirrors `review.go`: wraps the closeout-free `AgentExecutor` clone, both verdicts COMPLETE, malformed output is a failed attempt |
| `Sweeper` (mint + dispose over one watermark cursor) | `internal/dlqfix/sweep.go` (root module) | Mirrors `internal/review/sweep.go`: store facts paging, `ConsumerKey = "dlqfix-sweeper"`, head-bootstrap, checkpoint-after-page |
| `Dead → Cancelled` transition  | `internal/task/status.go` + both stores | `DismissDead(ctx, id, reason)` alongside `RescueDead`; reason rides the `task.cancelled` fact detail |
| Pool wiring                    | `cmd/tq`                   | `--dlq-fix` flag (agent-pool), sweep tick under the budget guard, executor registered by `registerAgentExecutors` (carry parity with `tq worker --agents`) |
| Operator lever                 | `tq dlq --dismiss ID --reason WHY` | The human gets the same disposition the sweeper has |

## Contracts

### DLQFixPayload (JSON)

    repo              required — same resolution rules as AgentPayload.Repo
    dead_task         required — the dead task's queue id (lineage + disposition target)
    dead_type         the dead task's type (today always "agent")
    work              the dead task's instruction: the agent prompt (the contract the
                      agent actually saw, {{TASK_ID}} already resolved by runAgent)
    failure           executor.FailureEvidence from the dead task's LAST task.failed
                      fact (stage, exit_code, tail) — may be zero when the journal
                      predates evidence
    last_error        the dead task record's LastError text
    attempts          attempts burned at death
    model/yolo/require_clean/timeout_minutes  same meaning as ReviewPayload

### DLQFixResult (the TQ_RESULT contract)

    TQ_RESULT: {"verdict":"fixed","summary":"...","commit_sha":"..."}
    TQ_RESULT: {"verdict":"wontfix","summary":"why this is not fixable from the repo"}

Parse rules (strict where it matters, mirroring `ParseResult`):
- verdict must be exactly `fixed` or `wontfix` (case-insensitive)
- `wontfix` without a non-empty summary is INVALID (a failed attempt) — an
  unexplained dismissal is exactly the DLQ behavior we are automating away
- `commit_sha` is optional (a wontfix autopsy must change nothing)
- both verdicts COMPLETE the task; the disposition is the sweeper's job

### Dedup and lineage

    DedupKey = "dlqfix:" + <dead-task-id>

One autopsy per dead task id, EVER. If a rescued task dies again, the same key
suppresses a second autopsy — the second death is deliberately a human surface
(the first autopsy's verdict and the fresh failure evidence are both in the
journal). This is the same suppressive-after-terminal semantics harvested items
already have (documented escape hatch: humans act, agents don't rewind keys).

## Loop safety (the invariants that make this shippable)

1. **Type scoping**: autopsies are minted ONLY for dead `agent`-type tasks.
   `sh` tasks may encode arbitrary operator commands (auto-rescuing one is an
   owner call, not a default), and review/status tasks are cheap second-opinion
   runs. This rule also makes `dlqfix` tasks UNAUTOPSiable — a dead autopsy can
   never mint another autopsy. The loop is structurally closed.
2. **No reviews of autopsies**: the review sweeper's type switch handles
   `agent` and `review` only; `dlqfix` completions fall through. Autopsies are
   second-opinion instruments (they run the closeout-free agent clone, like
   reviews and status tasks).
3. **Budget**: every mint pass runs under the pool's `mintPass` guard — the
   daily cap counts autopsy tasks like every other enqueue (SECURITY.md).
4. **Idempotent disposition**: `RescueDead`/`DismissDead` are transition-guarded
   SQL updates with the `RowsAffected` re-check; a replayed page, a manual
   rescue racing the sweeper, or a double tick degrades to a skipped counter,
   never a duplicate or a crash.
5. **One cursor**: mint and dispose share one watermark (`dlqfix-sweeper`), so
   a disposition is only ever computed from a completion fact the sweeper has
   actually consumed — no out-of-band polling, no replay hazards beyond dedup.
6. **Project exclusivity**: the autopsy inherits the dead task's project, so
   `WithProjectExclusivity` pools serialize the fix against sibling work in the
   same repo, exactly like harvested items.

## The autopsy prompt (shape)

The prompt quotes the dead task's original contract and evidence, then demands
a diagnosis-first workflow:

- diagnose the root cause from `failure.tail` + the repo state (READ the code,
  run the failing stage if cheap);
- fix ONLY if the root cause lives inside this repository — minimal fix, then
  prove it by re-running the failed stage; commit with the
  `Task-Queue-ID: {{TASK_ID}}` footer ({{TASK_ID}} = the AUTOPSY's own id, the
  runAgent substitution convention);
- if the root cause is outside the repo (provider outage, operator error,
  missing external resource, needed change in another repo): change NOTHING,
  verdict `wontfix` with the reason;
- end with exactly one TQ_RESULT line (the contract above).

The `work` text carries the dead task's prompt verbatim; its footer references
stay as the dead agent saw them (no re-resolution) — the same
quoted-contract discipline the review prompt learned from the Hermes incident.

## Store change: DismissDead

    Dead → Cancelled, one `task.cancelled` fact whose detail carries
    {"reason": ...} (+ "dismissed_by": "dlqfix-sweeper" | "operator")

Why a new transition instead of leaving dead tasks dead: the DLQ is a
projection, and "unfixable, with a recorded reason" is a different end state
from "dead, awaiting a human". Cancelled leaves the DLQ listing, the journal
keeps every fact (the death evidence is never deleted — facts are append-only),
and the `tq show` trail explains the dismissal. The transition table gains
exactly one edge; both stores implement the same guarded update (ADR-0007
mirroring), and the conformance suites pin: happy path, wrong-status refusal,
unknown id, reason-in-fact.

## Rejected alternatives

- **Agent runs `tq dlq --rescue` itself**: would hand the production journal
  path to a sandboxed agent and make disposition un-auditable from the journal.
  Disposition stays store-side, sweep-side.
- **Disposition inside the executor** (store injected into `DLQFixExecutor`):
  breaks the executor layer's purity (process runner, no store) and drags the
  queue contract into the executor module's dependency DAG.
- **Second-autopsy attempts keyed by death epoch**: bounded cost beats maximal
  autonomy; a fix→die→fix treadmill on one task id is the exact runaway the
  budget guard exists to prevent. Humans own round two.
- **Autopsying `sh` tasks**: an agent judging arbitrary shell commands for
  re-execution safety is an owner-gated policy, not a default.

## Open questions (deliberately unanswered here)

- Should a `wontfix` autopsy optionally notify the PapDashboard bridge with its
  summary (the dead-letter alert currently resolves only on rescue)? Left out:
  the alert lifecycle is "dead pool" semantics, not per-task commentary.
- Should the web UI render a dedicated autopsy payload section (it falls back
  to the generic payload JSON today)? Fine for v1; revisit with the
  detail-page payload conventions if autopsies become noisy.
- Review sweep minting reviews for the FIX commits an autopsy lands (today: no
  review — the commit carries the autopsy's `Task-Queue-ID` and `tq show
  --commits` cross-references it). If unreviewed fix commits become a quality
  gap, this is a one-case addition to the review sweeper's type switch.

## Test plan

- `internal/task`: transition table row Dead→Cancelled.
- stores (both, mirrored): DismissDead happy/refusal/missing/reason-in-fact.
- `internal/executor`: payload validation (permanent), verdict parser table
  (fixed/wontfix/unknown/no-line/wontfix-without-summary), prompt carries dead
  task id + evidence + footer contract, both verdicts complete.
- `internal/dlqfix`: mint-once (dedup hit on re-sweep), agent-type scoping
  (sh/dlqfix dead tasks skipped), fixed→rescued (fresh budget, pending),
  wontfix→dismissed (reason on the cancelled fact), replayed completion →
  skipped-not-crash, vanished dead task → skipped.
- smokes: existing pool smokes stay green (flag off by default).
