# ADR-0021: Incident pipeline — error reports as commands, incidents as facts, fix tasks as reactions

Date: 2026-10-08
Status: Accepted
Depends on: ADR-0001 (facts-first), ADR-0008 (machine API), ADR-0009
(journal-as-bus / exact consumers), ADR-0015 (priority bands),
ADR-0016 (facades), ADR-0019 (v4 platform), the session-bridge synthetic
stream precedent.

## Context

Production errors (web-client beacons, server panics/5xx) need to become
review-and-auto-fix work in this queue WITHOUT a second event-sourcing
system: the journal already is the event store, watermarked sweepers
already are the exact-consumer seam (ADR-0009), and the agent executor
already is the reaction surface. A naive design — error →
`POST /api/v1/tasks` directly — is write-side coupling with no incident
concept: no grouping, no occurrence counting, no regression signal, and
dedup windows that either block regressions forever (stable key) or
duplicate storms (no key).

## Decision

The pipeline is commands → facts → projection → policy, all inside tq:

1. **Command**: `ReportError` (`incident.Recorder.Record`). The wire body
   is `incident.Report` — a type with NO place for headers, cookies, or
   request bodies (secrets unrepresentable); free text is capped by
   `Clip` before anything is journaled. Exposed as `POST /api/v1/errors`
   on the machine API (token-guarded like every write, 503 when the
   recorder is not wired). It appends exactly one fact and reacts to
   nothing.

2. **Event**: `error.observed` rides the journal on the synthetic
   `incident:<fingerprint>` stream — the session:* precedent: an identity
   no enqueue/claim machinery can ever pick up. Fingerprint = sha256
   (project | kind | normalized message | top stack frame), numbers and
   quoted strings collapsed; release deliberately EXCLUDED so one bug
   groups across releases.

3. **Projection**: `incident.State` is a pure fold over
   `error.observed`, `incident.task-minted`, `task.completed`, and
   `task.dead-lettered` — occurrences, first/last seen, mints, status
   (open → fix-dispatched → resolved | fix-failed), regressions. Never a
   mirrored column; `tq incidents` renders it read-only (no watermark
   side effects).

4. **Policy** (`incident.Policy`, watermark key `incident-policy`):
   reacts on the sweeper seam (the shared `watermark.Cursor` pump — same
   shell as review/dlqfix/status, NOT the consumer dispatcher, which has
   no production host yet; migrating onto `Dispatcher.Subscribe` is the
   ADR-0009 convergence path and changes nothing here because reactions
   are per-fact functions). Rules: first observation of an incident mints
   a hot-band (120) `agent` fix task; observations while a fix is in
   flight count only (storms: 1,000 facts → 1 task); an observation after
   the latest fix task reached a terminal state is a REGRESSION — reopens
   the incident and mints at machine band (150). Mints journal
   `incident.task-minted` (taskID, sourceSeq, regression, priority).

5. **Idempotency** (at-least-once delivery, crashes between enqueue and
   mint-fact, replayed pages): the mint's task dedup key is
   `err:<fingerprint>:<sourceSeq>` — a replayed fact converges on the
   stored task; a NEW fact (regression) carries a new Seq and mints
   afresh; the fold's seq guard makes redelivery a no-op; an in-memory
   pending guard covers the same-page window between Enqueue and the
   mint fact folding back.

6. **First-run replay**: unlike the review/status sweepers (bootstrap at
   head so pre-feature completions do not mint a stale flood), a FIRST
   incident-policy run checkpoints at 0 and replays the whole journal —
   the family is brand new (nothing predates it), the API records while
   no pool runs, and replay is bounded to one mint per incident stage by
   storm dedup. Rewind any time with
   `tq watermarks set incident-policy SEQ`.

## Consequences

- The API process and the pool decouple cleanly: `tq api` can record
  standalone; the next pool start mints for everything observed.
- `Report.Project` MUST name a repo resolvable under the pool's
  `--projects-dir` (the minted agent task fails permanent otherwise —
  the correct classification; the DLQ autopsy path owns it).
- Known crash window: Enqueue succeeds, the process dies before the mint
  fact lands, AND a new occurrence arrives before restart → two tasks
  for one stage (dedup keys differ by Seq). Bounded by project
  exclusivity and harmless to correctness; the fold records both mints.
- Known reorder edge: a mint fact landing after its task's terminal fact
  (append delayed across a restart while a worker completed the task)
  leaves the incident fix-dispatched until the next observation. Heal:
  next occurrence re-evaluates on latest mint.
- No dashboard surface yet (v1 is API + CLI read); the webui board and
  an incidents view are follow-ups. Occurrence counts are fold-derived,
  not appended per fact.
- Not in scope: rate limiting the HTTP route beyond the API's lockout —
  producers sit behind their own app servers, which own per-IP limits.

## Verification

`internal/incident` tests (real SQLite store): fingerprint stability
(digit/quote/whitespace noise), clip/validate caps, recorder contract,
storm (1,000 → 1 task), replay convergence, regression band, dead-letter
→ fix-failed, restart resume, first-run replay. `internal/httpapi`:
503/400/202 + fact-journaled + nothing-journaled-on-invalid. Binary E2E:
`tq api` POST → `tq incidents` → `tq agent-pool --once` mints → task
dead-letters on missing repo → fold shows fix-failed.
