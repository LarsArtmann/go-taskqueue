# MAJOR OPEN PROBLEMS — Dispatch prompting & batch granularity

**Severity: MAJOR** · **Status: OPEN — awaiting owner go/no-go on the proposed direction**
· Discovered + measured 2026-10-11 (session reports
[`docs/status/2026-10-11_00-04_deployment-log-triage-readmodel-outage-fixed.md`] and
[`docs/status/2026-10-11_02-23_prompting-batch-size-comparison-owner-claim-measured.md`])

Owner's hypothesis, confirmed by measurement: _"we split work in WAY too small batches and
treat the AI agents like we do not trust them at all... our prompting is pathetic."_

---

## Problem 1 — Distrust-bloated dispatch prompts (MAJOR)

Every pooled dispatch carries a ~400-word rulebook that restates AGENTS.md, assumes bad faith,
and buys no safety.

**Measured** (CV `.crush/crush.db`, 600-message window, 572 non-empty user messages; the pool
runs crush with cwd=repo, so the DB holds both populations):

| Population                                                                             | n   | median          | words | framing                   |
| -------------------------------------------------------------------------------------- | --- | --------------- | ----- | ------------------------- |
| Pool dispatch contracts (agent/review/reviewfix/autopsy)                               | 168 | **2,675 chars** | 393   | ~12 distrust markers each |
| Machine status dispatch ("FULL COMPREHENSIVE STATUS UPDATE!")                          | 213 | 661 chars       | 109   | ceremony                  |
| Owner template ("READ, UNDERSTAND... keep going until everything works... great job!") | 137 | 242 chars       | 40    | 100% autonomy-positive    |
| Owner freeform                                                                         | 54  | 121 chars       | 18    | "done?"                   |

- Distrust density per pool prompt: **6.2× "never"**, plus per-prompt "no scope creep",
  "smallest correct change", "self-dealing", "EXACTLY", "do NOT" (caveat: "never" includes
  benign footer boilerplate like "Never push"; magnitude slightly inflated, direction robust).
- Owner's language: 149× "keep going", 286× "until everything works/done", 149× "you did a
  great job", **zero** distrust markers.
- **Only ~9% of recent "user" messages in CV are the owner speaking freely** — the queue
  out-talks him 10:1 in his own repo (213 status dispatches alone).
- The prose empirically prevented NONE of the real failure classes: the 169-loop was a
  toolchain/GOEXPERIMENT issue, env-streak deaths are dirty-repo environmental, gate-dead is a
  red base revision. **The verify gates, budget, and DLQ caught everything — the words caught
  nothing.** Rule-stuffed prompts add instruction dilution, not safety.
- Token tax: 393 words × 627 agent tasks (plus every review/autopsy/status prompt) ≈ hundreds
  of thousands of prompt tokens paid to say what AGENTS.md already says.

## Problem 2 — Micro-batch work units + ceremonial derived-task overhead (MAJOR)

The unit of work is one TODO checkbox, regardless of scope; derived ceremony doubles the task
count.

**Measured** (production journal `/mnt/pool/services/tq/tq.db`, read-only):

- Task mix all-time: **627 agent : 258 review : 49 status : 20 dlqfix = 52% derived overhead**
  on top of real work.
- Work unit = one TODO_LIST line: **median 38 words, 18% ≤20 words, smallest 5 words**.
  Exemplar: a reviewer finding "swap two duration numbers in TODO_LIST.md" became a full paid
  agent run — 2.7 KB contract, verify gate, review cycle.
- Status cadence `--status-every 5` produced **213 status-report agent runs in CV alone**;
  session titles self-document the pathology: _"Fix eleventh no-op report contract violation"_.
- Autopsy churn: environmental deaths (env-streak, dirty repo) mint paid dlqfix autopsies that
  cannot fix anything — ~7 futile cycles on 2026-10-10.
- Throughput ceiling: `daily-budget=30` enqueued, `max-per-tick=3`, 5 m ticks ≈ **~10 real
  items/day across 3 repos**. One owner session ("Execute docs-health skill on all 2026 md
  files", 558 msgs) outperforms the queue's week.

**Interaction effect (why it bites)**: micro-batches × ceremony burn the count-based budget on
cheap derived tasks — 2026-10-10 the queue parked ALL real work at "30/30 enqueued" while
actual model spend was **$0.07**. Small batches don't just underuse agents; they starve the
queue behind its own overhead.

## Proposed direction (pending owner decision)

1. **Batch = section/day-level goal**, not a line: "work §storage until every item lands or is
   BLOCKED — one commit per item, footer per commit". One claim, one agent; the verify gate
   still verifies per commit.
2. **Prompt = goal + context pointer + verify**, ≤5 lines: the item(s), "AGENTS.md owns the
   contract", the footer, the verify command. Delete the rule dump (AGENTS.md and the gates
   already carry every constraint the prose tries to restate).
3. **Cut ceremony**: status per-session-or-per-day (not per-5); review on risky task types
   only (code vs docs); autopsy only for non-environmental death classes.

Keep the safety rails exactly as they are — gates, budget, DLQ, env-streak breaker. The claim
is not "trust agents blindly"; it is "spend the trust budget on verification, not on prose."

## Sources & method

- `~/projects/CV/.crush/crush.db` (messages table, role=user, is_summary_message=0, newest 600;
  classification by template head-frequency; classifier NOT spot-checked — freeform share is
  soft, the 10× size gap is robust).
- `/mnt/pool/services/tq/tq.db` read-only (tasks table: type mix, payload.item word counts).
- systemd journal + pool conf (`daily-budget=30`, `interval=5m`, `max-per-tick=3`,
  `status-every=5`).
- Follow-ups queued in the 02-23 report §f: outcome-join (do long contracts fail less?),
  batch simulation replay, status/review yield audits, A/B template dispatch.
