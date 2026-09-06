# Design note: decision → question fan-out (PapDashboard)

**Status:** Proposed (2026-09-06) — plan row C24 / D74. Not implemented; this
note is the contract any implementation must satisfy.

## Problem

The agent pool is autonomous within its budgets, but some work items hit
genuine decision points: "which of these two APIs do we adopt?", "may I
delete this table?", "is this behavior a bug or intended?". Today the agent
has exactly two honest options: guess (scope creep, trampled intent) or
append `— BLOCKED: <question>` and leave the box unchecked (work stalls,
question buried in a todo file nobody is watching).

## Goal

An agent mid-task can ask its human one structured question; the pool keeps
working other items; the answer flows back and unblocks the exact task.

## Proposed mechanism (facts-first, no new infrastructure)

1. **Question as a task.** A new task type `question` (or a `question`
   payload field on `agent` tasks): `{repo, item_key, question, options[],
   asked_by_task}`. Enqueued with a dedup key `question:<asked_by_task>:<hash>`
   so re-asking after a crash cannot duplicate it.
2. **Blocking via deps, not polling.** The asking task is *not* completed;
   it stays `running` (lease + heartbeats, `--task-timeout` bounds the wait)
   and the `question` task is recorded in a `task.question-asked` fact.
3. **Dashboard surface.** The PapDashboard bridge (or a small exporter)
   forwards question facts as a `question.opened` event; the human answers
   in the dashboard UI. Answer arrives at the queue as an `answer` task
   completion: the dashboard POSTs to the (future) HTTP API
   (`Complete(answerTaskID, result)`), which appends `task.completed` with
   the answer as result detail. No bridge ever mutates queue state directly —
   answering goes through the same Store seam as every producer.
4. **Resume.** The waiting agent task's executor watches (or is re-invoked
   with) the answer: simplest correct version is fail-with-preflight — the
   task requeues with the answer injected into the payload (`answered: {...}`),
   so the next attempt starts knowing the decision. The question task's
   dedup key changes with the answer hash, so a follow-up question stays
   possible.

## Invariants

- Questions are tasks: journaled, replayable, visible in `tq facts` —
  the journal remains the single answer to "what happened".
- A question can never be lost silently: `task.question-asked` without a
  later `task.completed` (answer) shows up in `tq top`/audit as an open
  question with the asking task's age.
- Budget: answering is human work, not API spend — question tasks cost
  nothing and are exempt from `--daily-budget` projections (they are not
  `agent`-typed).
- Blast radius: the dashboard's write path into the queue is `Complete`
  on answer tasks ONLY, enforced server-side by task type, never a generic
  write API.

## Rejected alternatives

- **Interactive stdin/side-channel to the agent process:** breaks the
  process-per-task model, unjournalable, unreplayable.
- **Email/Slack fan-out first:** fine as additional surfaces later, but the
  queue must own the blocking state, so task-as-question comes first.
- **Long-poll the dashboard from the executor:** couples executor to
  dashboard availability; deps + facts keep both sides independently
  restartable.
