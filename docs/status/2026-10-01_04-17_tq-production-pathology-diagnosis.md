# Production Queue Pathology Diagnosis — CV crush-history audit (72h window)

**Session type:** owner-commissioned interactive diagnosis (NOT a task dispatch —
no Task-Queue-ID footer by design). **Scope:** read-only forensics across
`~/projects/CV/.crush/crush.db`, `/mnt/pool/services/tq/tq.db` (both opened
`mode=ro`), and this repo's working tree. **No code, config, or queue state was
changed.** Date-measured: 2026-10-01 04:17 CEST (filename timestamp); primary DB
readings 01:52–01:56, freshness re-check 04:14–04:17.

**Commission:** "Check the ~/projects/CV crush history of the last 72 hours and
tell me what is broken with ~/projects/go-taskqueue."

## Method (what was actually done)

1. Located the CV session history (`~/projects/CV/.crush/crush.db`, goose-migrated
   SQLite, `sessions`/`messages`/`files`/`read_files`; 3.4 GB).
2. Windowed sessions to 72h — **125 sessions** — and grouped titles: a single
   task family ("confirm live postgres major version on onprem host" +
   phrasing variants) accounts for **~79 of 125** sessions, 09-29 02:51 →
   10-01 01:54, cadence ~6–10 min.
3. Read full transcripts of two loop sessions (incl. the dispatch prompt, the
   `tq-preflight.sh` output showing 189 footer-commits / 226 unpushed at 01:54,
   and the `blocked_owner` cite-and-skip re-emit tail).
4. Cross-referenced the production journal `$TQ_DB` (`/mnt/pool/services/tq/tq.db`):
   task row, 310 facts (155 claims at first read), requeue classifications
   (`retry_in_ms` 97–127 s, `attempt=0`), 429 interleave, per-project dead/completed
   counts, task-type census, dlqfix census, pool config
   (`/nix/store/84nr…-tq-pool.conf`).
5. Locally reproduced the vendor-gofmt kill: `gofmt -l .` flags **44 files, all
   under gitignored `vendor/`**, matching the pool gate's
   `test -z "$(gofmt -l .)"` stage.
6. Delivered the three-finding verdict + offered remediation; owner then
   commissioned this report.

## a) FULLY DONE

- **72h CV history mined and characterized** — loop family identified, session
  count grouped (~79/125), cadence and variant titles tabulated.
- **Loop mechanics traced end-to-end** with journal evidence: claim → agent
  session does the cheap §3 `blocked_owner` re-emit (commits a status report in
  CV) → verify gate (`go build ./... && go test ./...` in CV) fails → worker's
  own pre-attempt-rev probe says pre-existing → classified
  *environmental* → requeue **without burning an attempt**, ~100–130 s jittered
  backoff → repeat. 429 rate-limit requeues (15 m fallback) interleave and set
  the effective ~6–10 min cadence.
- **vendor-gofmt environmental kill confirmed and REPRODUCED locally** (44
  gitignored `vendor/` files fail `gofmt -l .`): matches the
  `agent verify gate environmental signature [vendor-gofmt]` deaths — 6 in the
  2h before diagnosis (00:09→01:40), each `att1/1` after shipped work; project
  tally 176 dead vs 93 completed.
- **Missing repair loop found**: pool config has NO `dlq-fix` →
  **zero `dlqfix:` autopsies ever minted** (dedup census = 0 across all 676
  tasks), 317 dead letters globally, so the documented gate-artifact
  auto-dismiss path for exactly the vendor-gofmt class never runs.
- **Side-finding logged**: every dispatched session spams
  `Skill validation failed path=~/.config/crush/skills/go-cqrs-lite/SKILL.md
  error="description exceeds 1024 characters"`.
- Freshness re-check at 04:14: loop task now `pending att2/3` (rate-limited
  15 m), **164 total claims** (+9 since 01:55) — still churning.

## b) PARTIALLY DONE

- **Prediction verification, half-closed**: the in-flight dispatch seen at
  01:44 (PID 2814020, task `000001a0f4b4671…`, AGENTS.md M89 prune) was
  predicted to "die on vendor-gofmt". Reality: it shipped three real legs
  (reports 01-56 / 02-30 / 04-00 — prune DONE, adoption-table repair,
  leg-3 verify) and died `att3/3` at 04:04 with a generic
  `agent verify failed` whose tail I did NOT pull — vendor-gofmt vs a real
  red gate is **unclassified**. Right direction, unverified mechanism.
- **Root-cause split asserted, not independently confirmed**: "CV red at HEAD"
  rests on the worker's own *fails-at-pre-attempt-rev* classification; I never
  ran CV's gate myself to name the failing package/test.
- **Budget-burn claimed, never measured**: "eating daily-budget 30" is
  plausible (each loop claim is a paid session) but no paid-turns/day number
  was derived from the journal or budget projections.

> **§b annotation 2026-10-01 05:05 CEST (execution session):** P1 loop task
> `000001a0eebb…` DISPOSITIONED — `tq ask` recorded the question (fact 8610)
> but the park verb is **unreachable from operator sessions** (the marker
> channel `$TQ_QUESTION_FILE` exists only inside dispatched runs; a pending
> question fact does NOT suppress claims — `internal/queue/companion/
> questions.go:76` only unblocks on ANSWER). Fell back to the plan's R1(b)
> alternative: `tq cancel --force` at 05:02 → terminal `cancelled` at the
> worker's next heartbeat (05:04), claims frozen at 169; CV TODO row
> annotated with the disposition + re-arm path (CV `a53eb9c08`). New finding
> **P6: no operator-side park verb** — feeds M20.

## c) NOT STARTED

- All remediation: park/cancel the CV loop task; `trash vendor/` +
  `tq dlq --rescue`; `dlq-fix = true` + pool restart; skill-description trim;
  environmental-requeue cap (code); gitignore-aware gofmt gate stage (code);
  PapDashboard alert-path check (alert-url `http://127.0.0.1:8088` — did
  anything fire for 176 dead?); cross-posting findings into the CV repo's own
  docs; archiving this diagnosis's evidence (`scripts/archive-evidence.sh`).

## d) TOTALLY FUCKED UP (wrong claims, bad process — owned)

1. **Falsified overclaim**: "attempts frozen forever / can NEVER reach the
   DLQ". Refuted by the 04:14 state (`att2/3`): non-environmental failures
   (429-adjacent, closeout classes) DO burn attempts. Truth: environmental
   requeues are uncapped and burn nothing, so the loop is a **slow-drip
   burner**, not a literal infinity — the conclusion's severity was right,
   the mechanism claim was wrong, and a 2-minute re-query before answering
   would have caught it.
2. **Imprecise magnitude**: "~60+ sessions" — the title-group table actually
   sums ~79/125. I eyeballed the top-25 list instead of summing the groups.
3. **Secondhand numbers presented as current**: "226 unpushed / 189 footer
   commits" are the 01:54 dispatch's own preflight readings, not my git count.
4. **Wall-clock assumption unstated**: the 72h window used `MAX(updated_at)` as
   "now" (1790812500) instead of actual clock — acceptable, but asserted
   silently.
5. **Did not dogfood the tool under diagnosis**: hand-rolled SQL against the
   production journal instead of `tq stats` / `tq audit --journal` / `tq dlq`
   — the repo's own observability surface went unused in its own incident
   report.
6. **Trivial fix skipped on sight**: the ≤5-min skill-description trim was
   standing policy (owner permission 2026-09-06) and I only reported it.
7. **No file:line citations for code-behavior claims**: the environmental
   classification site in `internal/executor` was never located — behavior
   level only, violating the repo's claims-carry-citations convention.
8. **Evidence not archived**: loop stats live only in this chat + report.
9. **No todo list** for a multi-step diagnosis (process nit).

## e) WHAT WE SHOULD IMPROVE

- **Re-verify falsifiable claims before shipping them** (d1/d2): pre-answer
  freshness re-query is cheap; do it always for live-system diagnoses.
- **Cite source locations, not just DB symptoms**, for anything attributed to
  code behavior.
- **Use `tq`'s own commands in `tq` incidents** — dogfooding is also a
  live test of the observability surface; SQL bypasses hides its gaps
  (e.g. no obvious "claims per task" stat exposed — itself a finding).
- **Escalation design gap (the real product bug)**: environmental requeues
  need a circuit breaker — cap consecutive no-burn requeues, then burn an
  attempt / auto-park / alert. The 429 path has caps; this path has none.
- **Gate robustness gap**: a verify-gate stage failing on gitignored files
  (`gofmt -l .` over `vendor/`) is an environment bug being paid for as a
  task-death class; the gate should be gitignore-aware at the root, not
  healed per-death by auto-dismiss.
- **Repair-loop default**: `dlq-fix` off in the shipped pool config means the
  DLQ is a landfill by default (317 letters, 0 autopsies).
- **Anomaly visibility**: 155+ claims on one task raised no alert anywhere;
  a claims-per-task threshold in `tq stats`/dashboard would have caught this
  on day one.
- **Fix-on-sight discipline** applies to config noise (skill validation spam)
  even when the finding is "minor".

## f) NEXT (grouped, grounded in this session's findings; not all harvested)

**Unblock production (today):**
1. Decide + execute loop-task disposition: `tq ask --task 000001a0eebb…` park, or cancel + strike CV TODO row (§g1).
2. `trash vendor/` in this repo + `tq dlq --rescue` (documented vendor-gofmt recovery).
3. Add `dlq-fix = true` to the pool config; restart pool; confirm autopsies start minting.
4. Trim go-cqrs-lite `SKILL.md` description ≤1024 chars in its owning repo; verify via fan-out guard.
5. After 1–4: `tq stats` re-read; confirm the loop family stops appearing in CV crush history.
6. Check PapDashboard (127.0.0.1:8088): did dead-pool/dead-letter alerts fire for the 176 dead? If not, that's a new defect.
7. Pull the FULL `last_error` of the 04:04 prune-task death; classify vendor-gofmt vs real red.

**Code fixes (this repo):**
8. Circuit breaker for consecutive environmental requeues (burn attempt after N; escalate NotBefore).
9. Gitignore-aware (or tracked-files-only) `gofmt` stage in the pool verify gate — kills the vendor-gofmt class at root.
10. Auto-park routing for PREDICATE-suffixed blocked items (the `tq ask` path exists; requeue churn never uses it).
11. Structured `requeue_class` field on requeue facts (environmental / 429 / closeout / gate) for observability.
12. Claims-per-task anomaly threshold surfaced in `tq stats` + webui.
13. dlqfix sweeper: consider default-on, plus a guard test that enabled ⇒ dead letters mint autopsies.
14. `tq doctor`: probe for gitignored files failing gate gofmt (env-bug detector).
15. Model the 429-fallback × environmental-backoff interplay that produced the 6–10 min steady-state cadence; pick backoff constants deliberately.
16. Budget projection: count no-attempt-burn churn (sessions × paid turns) against the daily cap so loops are visible as budget, not just claims.

**Diagnostics / verification:**
17. Run CV's gate at HEAD myself; name the failing package/test (one command, closes §b).
18. Census the 106 dead CV tasks: how many looped environmentally before dying (same class as the star witness).
19. Measure actual paid-turn burn of the ~79-session loop family vs `daily-budget = 30`.
20. Verify the AnswerPoller/`tq ask` park→resume path end-to-end (never exercised here).
21. Check the 156 review tasks for the same churn signature.
22. Audit the other 2 `running` tasks seen at 01:54 for silent-loop symptoms.

**Process:**
23. Archive this diagnosis's evidence via `scripts/archive-evidence.sh` (git check-ignore first).
24. Cross-post the loop findings into CV's docs/status (its history is the victim; its TODO owns the blocked row).
25. TODO_LIST row for the environmental-requeue circuit breaker (design ruling §g3 first).
26. TODO_LIST row for the gitignore-aware gofmt gate stage.
27. Loop-detector smoke script (`scripts/smoke/`) asserting claims-per-task threshold alerts.
28. Consider a "no-burn requeue count" column in `tq stats` output.

## g) QUESTIONS (cannot be answered from the available evidence)

1. **Loop-task disposition**: the item's own TRIGGER says re-dispatch when
   192.168.1.100 answers — is the host outage known/in-progress owner work
   (→ park indefinitely via `tq ask`), or should the task be cancelled and the
   CV TODO row struck until the host is back (→ kills the churn now)?
2. **Production-change authority**: may an interactive session apply the
   remediations (vendor/ trash + `dlq --rescue`, `dlq-fix = true` + pool
   restart), or must they ride a dispatched task / owner hands, given parallel
   agents and the auto-commit daemon are mid-flight on this repo?
3. **Design ruling for the environmental requeue class**: when the verify gate
   fails pre-existing/environmental, should the task (a) burn an attempt after
   N consecutive environmental requeues, (b) auto-park (`tq ask`-style with
   owner question), or (c) keep no-burn requeue but cap backoff at hours?
   (This sets the circuit-breaker implementation in §f8.)

---
*No code or production state changed this session; docs-only output (this
report + index row). Battery: `scripts/check-status-index.sh` green post-edit;
root build/vet/test not applicable (no Go changes — docs-only session).*
