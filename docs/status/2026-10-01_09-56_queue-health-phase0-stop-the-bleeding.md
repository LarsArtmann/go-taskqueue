# Queue-Health Restoration — Phase 0 (stop the bleeding) close-out

**Date:** 2026-10-01 09:56 CEST (session window 05:00–05:25 CEST)
**Commission:** owner order "GET SHIT DONE — the WHOLE TODO LIST" over
`docs/planning/2026-10-01_04-27_SUPERB-QUEUE-HEALTH-RESTORATION.md`.
**Scope of this report:** this session's Phase-0 execution only (M1–M5 +
M6 design legs). Parallel agents kept landing throughout (see §b note).

## Method (what was actually done)

Read-only state assessment first (git/queue/facts), then owner-gated
production actions one at a time through the `tq` CLI, each verified
against the journal before the next. Code changes in-repo with module
gates; cross-repo edits (CV, SystemNix, go-cqrs-lite) committed via each
repo's own hooks. Rulings recorded in the plan before first action
(R1=park, R2=granted, R3=(a) N=3; commit `a5a12de0`).

## a) FULLY DONE

- **F1 rulings recorded** — plan §rulings annotated + committed (`a5a12de0`).
- **M1 loop task dispositioned** — `tq ask --task 000001a0eebb…` recorded
  the question (fact 8610) but could NOT park: **P6 (new finding): no
  operator-side park verb** — the `$TQ_QUESTION_FILE` marker channel exists
  only inside dispatched runs, and a pending question fact does NOT
  suppress claims (`internal/queue/companion/questions.go:76` unblocks on
  ANSWER only). Fell back to the plan's R1(b) alternative: `tq cancel
  --force` 05:02 → terminal `cancelled` at the worker heartbeat 05:04,
  claims frozen at 169 (terminal status structurally beats the 30-min
  watch; final re-check rides F70). CV TODO row 82 annotated with the
  disposition + re-arm path (CV `a53eb9c08`); diagnosis §b annotated with
  P6 (go-taskqueue `d3432c1c`).
- **M2 vendor-gofmt kill ended** — `trash vendor/` (44 flagged files → 0);
  full `.tq-verify` gate rc=0 green WITHOUT vendor/ (build+vet+`-race`
  tests+gofmt); green-dispatch probe completed end-to-end (enqueued at
  priority 90 after a P0 probe starved behind aging loopers — P7; claimed
  + completed 05:18:36, facts 8627/8628).
- **M3 dlq-fix enabled at source** — pool conf source located (SystemNix
  `modules/nixos/services/tq-agent-pool.nix` poolSettings; running conf is
  the nix-store render), `"dlq-fix" = "true"` added with rationale
  comment; pre-commit treefmt/deadnix/statix/`nix flake check` (eval-only)
  all green. Landed via SystemNix daemon commit `c180144e` after a ref
  race (see §d).
- **M4 skill noise killed** — go-cqrs-lite SKILL.md description measured
  properly (multi-line YAML parse: 1043 chars; first-line regex lied 1022),
  trimmed the ", etc. — see `metaengine/*engine`" tail → **1010 ≤ 1024**,
  all trigger phrases intact; committed `8ad3967f9`; fan-out verified by
  readlink resolution + reading the trimmed length THROUGH the link.
- **M5 04:04 prune-death ruled + fixed** — full verify-failure tail pulled:
  death was `TestRestartMidStreamLosesZeroFacts` (papdashboard) "dlq-550
  key seen 2 times, want 1" — **NOT vendor-gofmt** (test stage died before
  the gofmt leg). Reproduced at HEAD (~1/5 standalone, 1/12 under -v):
  over-pinned exact-count assertions on an at-least-once bridge (re-send
  before the batch-end checkpoint is by design; the dashboard dedupes on
  idempotency keys). Assertions relaxed to lower bounds (≥2 / ≥1) with a
  comment citing the death; **20/20 stable** after; root build+vet+bridge
  tests green; committed `ca849564` (amended the daemon's generic ferry
  while still local-only).
- **M6 design legs F18–F19** — citations: `verifyFor`
  `internal/executor/agent.go:944` (`.tq-verify` file → payload pin →
  `autoDetectVerify` :1023); minted Go default (:1008–1010) has NO gofmt
  stage — the gofmt leg lives in repo `.tq-verify` files. Scoped-gofmt
  idiom chosen and **empirically verified** in a scratch git repo:
  `test -z "$(gofmt -l . | git check-ignore --stdin -v --non-matching |
  grep '^::')"` — gitignored vendor/bad.go passes, untracked non-ignored
  sub/new_untracked.go correctly fails the gate (tracked-only listing was
  REJECTED: agents write new files pre-commit and must be gofmt-checked).
  Note: `git check-ignore --non-matching` requires `-v`; `::\t<path>` =
  non-ignored.

## b) PARTIALLY DONE

- **M6** — design + idiom verified; implementation (executor stage, tests,
  repo `.tq-verify` switch, module+root gates) NOT started. Detection-side
  (vendor-gofmt signature classification) already existed and is pinned by
  `verifygate_test.go` — M6 is the prevention side.
- **M3 runtime half** — deploy/pool restart is **owner-gated**: `sudo` and
  `systemctl` are banned in this shell; killing the pool would only
  restart it with the same immutable store conf. First `dlqfix:` autopsy
  mint check deferred to post-deploy.
- **M2/F7 deviation** — mass `tq dlq --rescue` deliberately NOT run: 167
  gofmt-class dead letters span 09-11→10-01 with work landed;
  `--rescue-all` is human-gated by CLI contract; per-task rescue/dismiss
  delegated to the dlqfix autopsies once deployed. Dead count unchanged
  (318 at session start).
- **Parallel-agent context (noticed, not mine):** the dirty-repo looper
  `000001a0f539…` (go-taskqueue agent task cycling on parallel uncommitted
  files — preflight refuses pre-spawn, so no paid burn) got a real run
  05:13–05:18 and died on an `internal/harvest TestSelfManagingLoop`
  "tick 2 enqueued=[]" failure I could NOT reproduce (3/3 pass isolated);
  suspected load-timing flake racing parallel edits — un-chased, needs a
  decision whether to triage like M5's flake. Later index rows (05-10
  liveness, 06-35 smoke forensics, 07-30 sweep incl. a 317-dead census)
  show parallel sessions already working adjacent plan lanes — re-verify
  before editing anything they touched.

## c) NOT STARTED

- M7–M21 in full (breaker memo/impl, requeue_class, stats anomaly, doctor
  probe, dlqfix guard, alert-path verify, loop smoke, census, budget
  model, CV cross-post, evidence archive + close-out, TODO harvest,
  ask park→resume e2e, AGENTS lessons), F70 final battery.

## d) TOTALLY FUCKED UP (owned)

1. **`tq ask --expires 720h`** bounced off the 168h cap — should have read
   the flag contract before the first invocation (re-ran at 168h).
2. **Skill description mis-measured at 1022** via a first-line-only regex;
   the real multi-line frontmatter value was 1043. Multi-line YAML needs a
   block-aware parse.
3. **Probe enqueued at P0** then starved behind the aging ladder (P7);
   cancelled + re-enqueued at priority 90. Probes should enter at marker
   band (≥90) by default.
4. **Nearly ran a bare `tq worker` against the production DB** — it
   registers only the sh executor but `ClaimDue` has no type filter and
   unknown type = PERMANENT error, so it would have dead-lettered the
   pending CV/go-taskqueue agent tasks. Caught by reading `claims.go:75`
   and `worker.go:567` first; this footgun is undocumented — see §f.
5. **SystemNix commit lost a ref race** with a parallel session's HEAD
   move (`cannot lock ref 'HEAD'`) AFTER all pre-commit checks passed;
   recovered only because the daemon had already ferried the content
   (`c180144e`). In daemon-swept foreign repos, commit immediately after
   staging; retry-once on lock failure is the right shape.
6. **First M1 watcher polled a non-matching regex** against `tq show`
   output (12 empty samples ≈ 4 min wasted) — read the output shape before
   writing the poller.

## e) WHAT WE SHOULD IMPROVE

- **Operator park verb (P6)**: either a `tq park TASK_ID --until DUR`
  command or `tq ask` learning an operator mode (park without the marker
  channel). Today the operator's only disposition is cancel — destructive
  and attempt-losing.
- **Probe/one-shot ergonomics (P7)**: fresh low-priority tasks starve for
  a long time; a `--priority` hint in docs or a fast-lane for sh probes
  would prevent the cancel/re-enqueue dance.
- **Bare-worker safety (P8)**: `tq worker` without `--agents` should
  refuse agent-type claims (or filter by registered types at claim time)
  instead of permanently failing them.
- **Daemon-race posture in foreign repos**: stage → immediate commit; if
  the ref lock bounces, inspect what the daemon ferried instead of
  assuming loss.
- **Gofmt-stage scoping at the mint**: the executor's minted Go default
  carries no gofmt leg while repo `.tq-verify` files hand-roll `gofmt
  -l .` unscoped — the scoped idiom belongs in the mint so new bootstraps
  inherit it (feeds M6).
- **Multi-line frontmatter tooling**: any SKILL.md length check must parse
  the block, not the line.

## f) NEXT (grouped; plan rows + session-born)

1. M6/F20: add the scoped gofmt stage (verified idiom, §a) to the minted
   Go default in `defaultVerify` + export the stage constant.
2. M6/F21: executor test — gate green with gitignored unformatted
   vendor/ present; red on untracked non-ignored unformatted file
   (reuse `writeVendorRepo` fixture shape).
3. M6/F22: `cd internal/executor && GOWORK=off` module gate.
4. M6/F23: root build/vet/`-race` (vendor/ deliberately absent — no
   `go mod vendor` until a parallel session needs it).
5. M6: switch THIS repo's `.tq-verify` gofmt leg to the scoped idiom.
6. M6/F24: commit with footer via `scripts/commit-task.sh` if task-ridden.
7. M7/F25: circuit-breaker memo (R3 = (a): N=3 consecutive env requeues →
   burn attempt + exponential NotBefore + alert fact) in docs/planning.
8. M7/F26: map requeue call sites (worker/executor) with file:line
   citations — include `worker.go:421` env-class comment as the anchor.
9. M7/F27: counter persistence decision (in-memory streak vs
   fact-derived) recorded in the memo.
10. M8/F28: consecutive-env-requeue counter + attempt burn after N.
11. M8/F29: exponential NotBefore escalation on the env streak.
12. M8/F30: alert fact / PapDashboard notify on streak ≥ N.
13. M8/F31: worker test — 3 env requeues burn an attempt.
14. M8/F32: conform-suite pin — env class never loops > N.
15. M8/F33: root `-race` battery + `scripts/test-cmd-tq.sh`.
16. M8/F34: footer commit.
17. M9/F35–F38: `requeue_class` fact detail field + journal round-trip
    test + legacy-`unknown` backfill note + commit.
18. M10/F39–F42: `tq stats` claims-per-task flag + webui badge + tests +
    commit (CHECK parallel 07-30 session's stats work first — overlap
    risk).
19. M11/F43–F45: `tq doctor` gitignored-gofmt probe (`git ls-files -o` ∩
    `gofmt -l`) + hermetic unix test + commit.
20. M12/F46–F48: dlqfix default-on memo + guard test (enabled ⇒ autopsies
    mint) + commit.
21. M13/F49–F50: query PapDashboard API for recent `alert.triggered`;
    rule fired/silent for the 176-dead window; file defect if silent.
22. M14/F51–F54: loop-detector smoke (red on 30-claim fixture, green on
    healthy) + ci-local wiring + commit.
23. M15/F55–F57: CV dead census by class + review-churn audit + census
    table into the close-out (parallel session already ran a 317-dead
    census — reconcile, don't duplicate).
24. M16/F58–F59: loop-family paid-turn total from CV crush.db vs cap 30.
25. M17/F60–F61: CV cross-post status report + CV TODO row for their red
    gate (coordinate: CV repo is hot with parallel agents).
26. M18/F62–F63: `git check-ignore -v` then `scripts/archive-evidence.sh`
    for session evidence; a–g close-out indexed.
27. M19/F64–F65: harvest code-fix TODO rows (ONLY after M2+M6 verified —
    grinder guard) + first harvested task survives the gate.
28. M20/F66–F67: ask park→resume e2e (fixture task, answer, resume) +
    predicate-truth re-dispatch path.
29. M21/F68–F69: AGENTS.md lessons (env-requeue class, gate hygiene,
    scoped-gofmt idiom, P6/P7/P8) within the 15,000 B guard.
30. F70: full `scripts/ci-local.sh` battery + M1 final no-new-claims
    re-check.
31. P6: implement the operator park verb (design rides M20's e2e).
32. P7: docs/priority-band guidance for probes (or fast-lane).
33. P8: bare-worker agent-claim refusal guard + test.
34. Triage `TestSelfManagingLoop` (harvest) like M5: reproduce under load,
    then unflake or rule race-with-parallel-edits.
35. Post-deploy: confirm first `dlqfix:` autopsy mints; verify the
    gofmt-class letters auto-dismiss with shipped proof; dead count drops.
36. Ratify (or overturn) the F7 no-mass-rescue deviation once autopsies
    run.
37. Push posture: unpushed session commits sit in 4 repos (go-taskqueue,
    SystemNix, CV, go-cqrs-lite) — see §g3.
38. Reconcile this session's plan rows against the parallel 05-10/06-12/
    06-35/07-30 reports before executing M10/M11 (overlap).
39. Fan-out guard `check-skill-fanout.sh` absent on this machine — see §g.

## g) QUESTIONS (cannot be answered from here)

1. **M3 deploy**: `sudo`/`systemctl` are banned in my shell, so the
   dlq-fix enablement cannot reach the running pool from this session —
   will you run `scripts/deploy.sh` (SystemNix) now-ish so the autopsy
   backfill (318 dead) starts, or should the deploy ride a later window?
2. **Dirty-repo looper `000001a0f539…`**: it cycles cheaply (preflight
   refuses before any paid spawn) but rhythmically occupies a pool slot
   and just died once on the un-reproduced harvest flake — cancel it like
   M1, or let it cycle until M8's breaker bounds the class?
3. **Push posture**: session commits are unpushed in four repos
   (go-taskqueue ~9 ahead incl. my Phase-0 legs; SystemNix `c180144e`;
   CV `a53eb9c08`; go-cqrs-lite `8ad3967f9`) — push now, or leave local?
