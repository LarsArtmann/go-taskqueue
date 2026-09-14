# Batched Harvest + Agent Power Grant — Design

Date: 2026-09-14
Status: IMPLEMENTED (this session)
Owner prompt: "I feel like we could get more out of the AI Agent if we could batch
tasks more, and give the AI more power to do things, at least when it actually is
supposed to get shit done."

## Problem

Every backlog item pays a full cold agent session: process spawn, AGENTS.md
ingestion, repo re-exploration, session bootstrap. With one-new-item-per-repo-per-
tick pacing, a 10-item section costs 10 cold sessions (plus 10 reviews, plus
status cadence) spread over ~10 harvest ticks. During Z.ai 429 windows each cold
session is also a fresh request burst that can die independently. The model is
smart; the pipeline drips.

Second: a work agent that DISCOVERS follow-up work has no sanctioned way to grow
the backlog. Today discovery detours through close-out report f) → status task →
TODO_LIST append → harvest — two extra agent tasks of latency.

## Solution A — Batched harvest (`--batch-items N`, default 0 = off)

One queue task carries a run of up to N **adjacent open items from the same
TODO_LIST section** (same heading, consecutive in file order; blocked/known items
break the run). One crush session works them in order.

- **Payload**: `harvestPayload.Items` (member texts) + `ItemKeys` (member `todo:`
  keys) + `Dedup = batch:<hash>` over the sorted member keys. `Item` stays the
  FIRST member text so review quoting, status windows, and `tq show` provenance
  keep working. Deterministic key: unchanged set never re-mints; any member edit
  forks the batch (same semantics as single-item text edits).
- **Prompt**: `DefaultBatchPromptTemplate` — per-item commit + footer, per-item
  check-off, per-item `— BLOCKED:` escape (partial completion is a SUCCESS as
  long as the repo verify gate passes), retry-safety ("items already [x] are
  done — skip them"), follow-up append grant.
- **Pacing**: a batch IS one task — one NEW enqueue per repo per run (unchanged
  invariant), ONE slot against `--max-per-tick` and the daily budget (both gates
  are documented TASK caps; per-item cost is amortized, see flag help). Raise
  gates consciously when raising N.
- **Priority**: max over member resolved priorities; hot if ANY member is hot;
  `MarkerLevel` pins the max member marker (marker > AI precedence preserved).
- **Timeout**: payload `TimeoutMinutes` scales × member count (repo ladder or
  30-min default per item). The pool's `--task-timeout` remains the hard
  ceiling — operators raising `--batch-items` must raise it too.
- **prune-stale**: a PENDING batch cancels only when ALL member items are
  ticked (ticked rule) or ALL member keys gone from the file (absent rule);
  partial staleness leaves the task. The generic absent pass must SKIP
  `batch:` keys (they never match a `todo:` present key).
- **Audit (drift)**: member items resolve to their batch task via payload
  `ItemKeys` — stale-open mints per-item catch-ups as today; stale-done reports.
- **prioritize/repri**: batch keys (`batch:` prefix) are invisible to the
  `todo:`-keyed AI scorer — documented interaction, not a bug: markers and
  importance still rank batches.
- **Cap**: N clamped to 1..10 (flag validation) — context explosion guard.

## Solution B — Power grant (prompt-only, both templates)

Both work prompts gain an explicit contract point: the agent MAY append NEW
unchecked follow-up items to TODO_LIST.md (one per line, agent-executable,
correct section) instead of doing discovered work now. The queue's existing
gates — `--max-per-tick`, daily budget, repo intervals, priority clamps, review
sweeper — own admission, so the grant cannot bypass spend control (unlike direct
`tq enqueue`, whose budget bypass is a known open problem in the session-close
bridge design). Never an item describing the task's own work (no self-farming).

Explicitly NOT granted: editing `.crushrc`/`.tq-verify` (self-dealing guard
stays), direct enqueue, pushing.

## Alternatives rejected

- **Warm-session chaining** (pin derived session id into the next pending task):
  same token economics (later turns pay full history) but keeps per-item
  process spawns; also introduces temporal coupling to crush session storage.
  Batch-in-one-run shares exploration AND one spawn.
- **In-run claim loop** (`tq next` from the agent): needs CLI-side leases +
  heartbeats; inverts the worker model; unbounded fan-out. Revisit via ADR if
  batching proves insufficient.
- **Direct agent `tq enqueue`**: budget bypass (mintPass gates harvest, not the
  CLI), self-replication risk. The TODO_LIST route flows through every gate.

## Blast radius

Consumers of `PayloadItemOf` verified: prioritize (filters `todo:` — batches
skip), `tq show` provenance (batch key displays), status/review (quote `Item`).
`go install`-frozen payload contract untouched (new fields are omitempty).
Default OFF keeps the fleet byte-identical until the owner opts in.
