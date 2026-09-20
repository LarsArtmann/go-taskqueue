# PapDashboard Questions — deep research, design, foundation layers

**Window:** 2026-09-17 ~13:45–14:37 CEST · **Feature:** decision → question fan-out
(ROADMAP v0.3.0 remainder arc; design note
`docs/planning/2026-09-06_decision-question-fanout.md`, still Proposed).
Owner intent: agents do what they know autonomously; what they are unsure
about goes to the owner ASYNCHRONOUSLY via PapDashboard, for all repos at once.

## 0. Environment found (turn 1)

- Repo mid-migration to Go 1.27.1 (commit 8d3de30, today 09:38): root + cmd/tq
  go.mods at 1.27.1, ALL other 17 module go.mods still at 1.26/1.26.7 — that
  state FAILS `check-go-mods.sh` (every module must declare the root's exact
  version) and the go 1.27 stdversion vet gate (`jsontext.Value requires
  go1.27` fires for go-1.26 modules under a 1.27 toolchain, GOEXPERIMENT
  notwithstanding — the exact gate class AGENTS.md documents for setup-go).
- Local toolchain: go 1.26.7 with `GOTOOLCHAIN=local` → root build impossible
  until `GOTOOLCHAIN=go1.27.1` (downloaded + cached). gopls/golangci-lint LSP
  diagnostics were phantom errors from the same mismatch.
- Session command prefix settled: `export GOEXPERIMENT=jsonv2 GOTOOLCHAIN=go1.27.1`.
- Tree was clean; no stash. CONTRIBUTING.md read (turn-1 ritual).

## 1. Research findings (all verified against source this window)

**PapDashboard side — the question surface ALREADY EXISTS, nothing to build
there:**

- SDK (`~/projects/PapDashboard/sdk/methods.go`): `CreateQuestion`
  (POST /api/questions), `ListQuestions` (filters: sourceApp/type/limit/
  offset/from/to/q), `AnswerQuestion` (PATCH /api/questions/:id/answer,
  answer = one string), `GetQuestionEvents`.
- Ingest route (`internal/api/api.go:397`, `handler_impl.go:202-231,344`):
  accepts `question.asked` with payload `{type,title,body,sourceApp,
  expiresAt?}`, honors Idempotency-Key (replay-safe, cached response), and
  RETURNS the created aggregate `{ID, Status, Version}`.
- Domain (`internal/domain/question/`): types `info|approval|confirmation|
  input` (types.go:19-32); statuses `pending → answered|expired` (terminal);
  optional `expiresAt` enforced by an ExpirationWorker
  (`internal/worker/expiration.go`). Title ≤ 500, body ≤ 10 000 chars
  (`internal/domain/content.go:5-9`). Auth: bearer on every /api route except
  health (`internal/middleware/auth.go`).
- `question.answered` is an event type with the answer in the payload
  (`internal/notify/render.go:58-67`).

**tq side — every needed precedent exists:**

- Store contract `internal/queue/queue.go` (Enqueue dedup, Requeue
  no-attempt-burn, watermarks, FactsForTask, CountFacts).
- Worker error-class ladder `internal/worker/worker.go:401-447`:
  PreflightError / RateLimitError → `store.Requeue` WITHOUT attempt burn +
  NotBefore parking (RateLimitError is the exact shape to copy for questions).
- Verdict channel `internal/executor/agent.go:412-431`: per-run temp file +
  env (`TQ_RESULT_FILE`) — the pattern for the ask marker.
- Bridge `internal/bridge/papdashboard/papdashboard.go`: read-only forward
  loop, watermark checkpoints, at-least-once with Idempotency-Key.
- `tq api` POST /api/v1/tasks is ENQUEUE-only (`internal/httpapi/httpapi.go:50`)
  — there is no Complete route, and building one would add an inbound write
  path.

**Design decision (deviation from design note §3, recorded for the note
update):** answers are PICKED UP by tq polling PapDashboard — tq stays the
sole network initiator, no new inbound write path into the queue, no
dashboard→tq coupling. The note's invariants all still hold (questions are
journaled facts; never silently lost; no generic write API — in fact zero
inbound write API).

## 2. The design (fact-park model)

1. **Ask** — `tq ask --task <id> [--type info|approval|confirmation|input]
   [--expires 72h] [--options "A|B"] "question"`: validates the task is
   Running/leased, redacts the text (`executor.RedactSecrets`), appends
   `task.question-asked` (detail: `queue.QuestionAskedDetail{ref=hash over
   task+normalized text, type, question, options, repo, expires_at}`), and
   writes the per-run marker `$TQ_QUESTION_FILE` (verdict-channel pattern).
   Re-ask after crash converges on the same ref (dedup).
2. **Park** — agent ends its turn having asked; `runAgent` reads the marker
   and returns `*QuestionPendingError{RetryAfter: until expiry,
   ResumeCloseout}`; the worker requeues WITHOUT burning an attempt;
   NotBefore = expiry (safety valve if the answer never comes). Task shows as
   parked, `--once` pools don't hold on it.
3. **Forward** — the (read-only) bridge gains a `QuestionAsked` case: posts
   PapDashboard ingest `question.asked` (type, title = truncated question,
   body = question + options + `Repo:` + `task:<id>` + `qref:<ref>` machine
   tokens, expiresAt forwarded, Idempotency-Key `question:<factSeq>`).
4. **Pick up** — NEW `papdashboard.AnswerPoller` (a PRODUCER, deliberately
   separate from the Bridge whose package doc forbids queue mutation): polls
   `GET /api/questions?sourceApp=go-taskqueue`, cursor = AnsweredAt
   checkpointed in watermarks (bootstrap at head, forward-only), correlates
   via the qref/task tokens, then calls the new atomic
   `Store.RecordAnswer`.
5. **RecordAnswer (contract + both backends, DONE, see §3)** — one tx:
   append `task.question-answered` + (task PENDING) inject the answer into
   the JSON-object payload under `answered` + clear NotBefore. Idempotent per
   ref (replayed pickups append nothing). Non-pending tasks: fact only.
   Raw-text payloads: fact only. Answered fact is self-contained (question
   text backfilled from the asked fact).
6. **Resume** — `AgentPayload.Answered []ResolvedQuestion` (new optional
   field) is rendered by the executor into the next run's prompt as owner
   rulings; follow-up questions converge (second ask parks again).
7. **See it** — `tq show` gains an open-questions section (from
   FactsForTask); `tq facts/tail` render the two new types.

## 3. What landed (a — fully done)

All committed by the daemon mid-window (expected behavior):

- **journal** (7188217): `QuestionAsked`/`QuestionAnswered` fact consts
  (`internal/journal/journal.go`) + facade aliases (`journal/journal.go`).
- **queue contract** (929e6d9): question type consts + `ValidQuestionType`,
  `QuestionAskedDetail`, `QuestionAnsweredDetail`, `AnswerRecord`,
  `Store.RecordAnswer` (doc states the invariants).
- **sqlite impl** (929e6d9 + c582577): `RecordAnswer` + `factDetailRefs` +
  `mergeAnsweredPayload` — idempotency scan in Go (never SQL LIKE on JSON),
  RowsAffected re-check honored (Store invariant), asked-text backfill.
- **postgres impl** (c582577): mirrored exactly (pgx, FOR UPDATE, $n
  placeholders) — `pgFactDetailRefs`, `pgMergeAnsweredPayload`.
- **go 1.27.1 module alignment** (c582577): ALL 19 go.mods raised to
  `go 1.27.1` (file-path `go mod edit`; raises only, never lowers). This
  completes the migration 8d3de30 started and unblocks every sub-module
  build/vet under the pinned toolchain.
- **queue + sqlite + postgres modules build + vet GREEN** after alignment.

## 4. Partially done (b)

- Store-layer verification: build+vet green; NO tests yet — conformance
  additions (sqlite store_test + postgres TestPostgresConformance subtests)
  are the immediate next step, before any higher layer lands.
- Design-note update: decision made and drafted in this report; the note file
  itself not yet amended (status Proposed → Accepted + polling deviation +
  fact-park model).

## 5. Not started (c)

executor (QuestionPendingError, TQ_QUESTION_FILE channel, AgentPayload.
Answered + prompt rendering, facade aliases) · worker park branch · bridge
forward case + AnswerPoller · cmd/tq ask + wiring + show/facts rendering +
usage · ALL tests incl. questions e2e smoke · full gates (root race suite,
per-module loop, cmd-tq gate, smokes, facade parity, lint --new-from-rev,
GOOS=windows) · docs (AGENTS.md payload contract, FEATURES, CHANGELOG,
ROADMAP/TODO_LIST, DOMAIN_LANGUAGE).

## 6. Fucked up this window (d) — honest

- **First `go mod edit` loop was malformed** (directory args through the
  shell handoff) and failed SILENTLY; I moved on and only caught it by
  re-checking state. Lesson applied: file-path invocation + explicit verify.
- **Dropped the RowsAffected re-check** in the first sqlite RecordAnswer
  draft (explicit Store invariant); self-caught, fixed pre-commit (c582577).
- **Burned a cycle on the stdversion vet gate**: ran vet under the 1.27
  toolchain before checking module-go.mod alignment — the AGENTS.md documents
  this exact gate; alignment check should have been turn-1 alongside
  `go version`.
- LSP diagnostics treated as noise from the start (correct call, but should
  have been recorded as "phantom until toolchain aligned" immediately).

## 7. What to improve (e)

- Turn-1 ritual for this repo: `go version` + `grep '^go ' go.mod` +
  per-module alignment check BEFORE any Go work (toolchain-first).
- Fixed session verify prefix (GOEXPERIMENT + GOTOOLCHAIN exported once) —
  never repeat it per command.
- Conformance-first ordering: tests with the impl, before dependents.
- Amend the design note BEFORE the layers that consume it (keeps the
  contract honest while the code lands).
- The daemon split this feature across 3 auto-commits (7188217/929e6d9/
  c582577) — attribution gap again; when explicit commits are authorized,
  commit per layer.

## 8. Next (f) — ordered, the 1% first

1. sqlite `RecordAnswer` unit tests (park→inject→unblock; idempotent replay;
   non-pending fact-only; raw payload fact-only; asked-text backfill;
   NotBefore cleared → ClaimDue picks it up)
2. postgres conformance subtests (mirror 1-6) + sqlite suite additions
3. executor: `QuestionPendingError` (+ constructor, ResumeCloseout field)
4. executor: `TQ_QUESTION_FILE` channel in runAgent (temp file, env, post-run
   read; question wins over verdict)
5. executor: `AgentPayload.Answered []ResolvedQuestion` (snake_case tags)
6. executor: prompt rendering of owner rulings; facade aliases
7. worker: park branch on QuestionPendingError (+ unit test)
8. bridge: QuestionAsked forward case (+ httptest stub tests)
9. `papdashboard.AnswerPoller`: paging + AnsweredAt cursor + watermark
   persistence + unknown-qref/cancelled-task handling (+ stub tests)
10. cmd/tq `ask` (flags, Running+lease validation, re-ask dedup, redaction,
    marker write) + usage text + help-text smoke
11. wire AnswerPoller under `--alert-url` (worker + agent-pool, runactor)
12. `tq show` open-questions section; `tq facts/tail/top` rendering
13. e2e smoke `scripts/smoke/questions-e2e.sh` (stub agent + stub pap) +
    ci-local + guard-wiring compliance
14. journal-drift audit: deliberate handling of the two new fact types
15. full gates: root build/vet/race, per-module loop, cmd-tq gate,
    papdashboard-e2e + fullcore + multi-repo smokes
16. facade parity + go-mods + script-syntax gates; lint `--new-from-rev`
    zero on touched files; GOOS=windows
17. design note → Accepted (fact-park + polling deviation)
18. AGENTS.md payload contract + bridge section; FEATURES row; CHANGELOG;
    ROADMAP arc + TODO_LIST leftovers; DOMAIN_LANGUAGE terms
19. v2 candidates (rows, not promises): expiry pickup (`question.expired`
    fact), ask-from-interactive-session bridge, webui surfacing, sh-task asks
20. release note: queue module version bump surfaces (sub-tags pre-cut before
    release gates)

## 9. Owner questions (g)

1. The 1.27.1 sub-module bump (19 go.mods, c582577) completes migration
   8d3de30 — keep it inside this feature's arc, or is another agent's window
   owning the migration (conflict risk)?
2. Answer return path: I chose POLLING (tq initiates; zero inbound write
   path) over the design note's sketch (dashboard POSTs into tq api). Is
   polling the ruling?
3. `tq ask --expires` default: is 72h the right owner-answer horizon, and
   should an EXPIRED question cancel the parked task or re-enter it for the
   agent to re-ask (my design: re-enter; expiry pickup is a v2 row)?
