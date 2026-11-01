# Owner ruling brief: is there a trust tier?

**Created:** 2026-11-01 21:05 (CET). **Author:** undispatched agent session.
**Status:** ONE-PAGE BRIEF for an owner product call. Nothing here is
implemented; the TODO row this unblocks is `— BLOCKED: owner design ruling`
(Q1 of `docs/planning/2026-11-01_20-17_SUPERB-okf-receipt-attester-pareto-plan.md`).
**Predecessors:** `docs/research/2026-11-01_okf-lessons.md` lesson 5;
`docs/planning/2026-11-01_20-49_reresolve-verify-retire-or-narrow.md` (same shape,
a different question).

---

## The question

OKF v0.2 derives a **trust tier** from its `verified` list: no `verified` key ⇒
*unverified*, machine-only entries ⇒ *machine-confirmed*, any `human:<id>` entry
⇒ *human-reviewed* — and the spec is explicit that the tiers are "advisory
signals, not access control".

tq derives what a run DID (footer commits, diff-tree files, go-crush-data
usage) and, since this window, what its GATE proved (`VerifyReceipt` +
`AttestVerify`). It records nothing about who CONFIRMED the work.

**The ruling needed: should tq compute and store a trust tier per task, or is
"no tier" the answer?**

## What exists today (so the decision is about a delta, not a vacuum)

| Confirmation | Where it lives now | Machine-readable? |
| --- | --- | --- |
| The gate passed | `AgentResult.Verify` + `.Attestation` (completion detail) | Yes |
| A reviewer approved | `review` executor's verdict (`tq verdict '<json>'`) on its own task | Yes, but on a DIFFERENT task |
| A human merged it | nothing — the merge is a git fact, not a queue fact | No |
| The report was written | `docs/status/tasks/…` file existence | No |

So the raw material for a tier already exists, spread across two task records
and one unrecorded git event. A tier would be a *join*, not new data.

## Option A — no tier (recommended)

Do not compute it. Instead:

1. Keep the two confirmations separate and queryable where they already are
   (`tq show` renders the receipt + verdict; the review task carries the
   approval).
2. Add nothing to the fact schema.

**Cost:** zero. **What it gives up:** a single field a future surface could
filter on (`--verified human-reviewed`). **Why that is acceptable:** there is
no such surface today, and building the join before the consumer is OKF's own
premature-generalization trap — the spec warns about scores for exactly this
reason.

## Option B — computed, stored advisory tier

Derive `unverified | machine-confirmed | human-reviewed` per task at completion
time and store it on the completion detail, next to the receipt.

**Cost, concretely:**

- A new derived field + its absence meaning (the additive-growth contract).
- The join: completion detail × the review task's verdict × (optionally) the
  merge. The review task is enqueued AFTER the completion, so the tier is not
  final at completion time — it needs a second write or a fold. **That is the
  expensive part**: it is a projection over two streams, not a field.
- A migration story for the 1,600+ existing completions (or a documented
  "tier absent = pre-tier", which is what the additive contract allows).
- Retention: the tier must be recomputable or it rots — the same argument that
  put the verdict beside the receipt instead of re-deriving it later.

**What it buys:** one filter term, once a consumer exists.

## Option C — computed, NOT stored (render-time)

Compute the tier in `tq show` from the records it already reads. No schema
change, no fold, no migration; the query is per-task and cheap.

**Cost:** a joiner in the CLI (which cannot use new executor symbols until a
release re-pins it — the ADR-0017 lag this window hit twice). **Buys:** the
filter term without the storage. **Risk:** two implementations of the tier rule
(CLI + any future surface) unless the rule lives in the executor and the CLI
twin is pinned — the same shape as `cmd/tq/verifyreceipt.go`.

## Recommendation

**Option A now, Option C when a consumer appears, Option B never.**

The reason is the one the OKF spec itself gives: a trust signal is worth
building when something *filters* on it. Today nothing does — the reviewer
sweeper reads the review task, the operator reads `tq show`, and no surface
asks "show me only human-reviewed completions". Option C is the cheap version
of the answer and it stays available; Option B pre-pays for a fold.

If the owner's answer is "B", the ruling should also say **which write owns the
tier** (completion detail vs a separate fold) — that is the decision that costs
weeks, and it is not one an agent should make by default.

## What this does NOT change

- `AttestVerify`'s verdict is not a trust tier: it grades ONE receipt against
  the evidence that produced it. A task can be `machine-confirmed` and still
  have shipped bad work.
- An AI priority score never contributes to any tier, in any option (§ the
  signals-not-scores pin in AGENTS.md).
