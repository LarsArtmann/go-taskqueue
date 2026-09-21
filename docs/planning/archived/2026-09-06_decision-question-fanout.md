# Design note: decision → question fan-out (PapDashboard)

> **EXECUTED — ARCHIVED 2026-09-21 (docs-health archive sweep)** — SHIPPED 2026-09-17 (tq ask loop, AnswerPoller, questions-e2e smoke; FEATURES row FULLY_FUNCTIONAL); design note Accepted; ask-policy + expiry rulings routed as BLOCKED TODO rows.

**Status:** Accepted (2026-09-17) — SHIPPED as the fact-park model
described below, with two deliberate deviations (polling replaces the
POST sketch; parked-not-running replaces the running-with-deps blocking).
Implementation arc: the 2026-09-17 14-37 foundation report (design) +
21-04 report (all layers, gates). Open owner rulings: agent ask-policy
(no prompt teaches `tq ask` yet) and the 72h expiry default.

**What shipped (the short contract):**

- `tq ask --task <id>` validates the task is RUNNING, redacts the question,
  appends `task.question-asked`, and writes the per-run
  `$TQ_QUESTION_FILE` marker — the question is a FACT, not a task. The ref
  is sha256(taskID + normalized question), truncated to 16 hex; re-asking
  converges on the same ref (a pending ref re-arms the marker, an answered
  ref is a no-op).
- The agent's turn ENDS with `QuestionPendingError`; the worker requeues
  WITHOUT burning an attempt, NotBefore = the question's expiry (default
  72h, cap 7d) as the safety valve — expired questions re-enter the task.
- The PapDashboard bridge forwards the question (correlation tokens
  `task:<id>` / `qref:<ref>` LEAD the body so truncation can never sever
  the route home); the owner answers in the dashboard.
- The AnswerPoller polls answered questions back (watermark cursor on
  AnsweredAt, bootstrap-at-now, at-least-once — RecordAnswer is idempotent
  per ref) and the store injects the ruling into the payload (`answered`
  array), clears NotBefore, and appends `task.question-answered`. The
  resumed run sees "Answers from the owner" rendered into its prompt.

**Deviations from the sketch below:** (1) the question is NOT a task —
parking the asking task directly (requeue-without-burn, mirroring the
RateLimitError ladder) avoids a whole task lifecycle for one boolean and
keeps attempts honest; (2) the answer does NOT POST into the queue — the
poller PULLS it, keeping the zero-inbound-write-path posture (the queue
is never a server for the dashboard; the 2026-09-17 14-37 report chose
polling for exactly this).

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
2. **Blocking via deps, not polling.** The asking task is _not_ completed;
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
