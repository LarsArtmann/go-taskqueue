# Queue-Health Restoration — full session close-out (Phase 0 + M6 design)

> **DUPLICATE — ARCHIVED 2026-10-02** Canonical report: `2026-10-01_17-05_queue-health-restoration-plan-closeout.md`. phase report consolidated by the final plan closeout (M6-M21 there)


**Date:** 2026-10-01 11:01 CEST (session windows 05:00–05:25 and
09:56–11:01 CEST; interim = parallel agents only)
**Commission:** owner order "GET SHIT DONE — the WHOLE TODO LIST" over
`docs/planning/2026-10-01_04-27_SUPERB-QUEUE-HEALTH-RESTORATION.md`;
this report supersedes/absorbs `2026-10-01_09-56_queue-health-phase0-
stop-the-bleeding.md` (same session, first cut — keep both, the 09-56
index row stays the Phase-0 record).
**Scope:** exactly what this session did and noticed. No unrelated
research.

## Method

Read-only assessment first, then owner-gated production actions one at a
time through the `tq` CLI, each verified against the journal before
proceeding. In-repo code changes behind module gates; cross-repo edits
committed through each repo's own hooks. Rulings recorded in the plan
before the first action (R1=park, R2=granted, R3=(a) N=3; `2a63b1dc7ce57818527d89078465a16b4a1fbd42`).

## a) FULLY DONE

- **F1 rulings recorded** in the plan (R1=park default, R2=granted,
  R3=(a) N=3) — commit `2a63b1dc7ce57818527d89078465a16b4a1fbd42`.
- **M1 loop task `000001a0eebb…` dispositioned.** `tq ask` recorded the
  question (fact 8610) but could not park — **P6 (new): no operator-side
  park verb**; the `$TQ_QUESTION_FILE` marker channel exists only inside
  dispatched runs and a pending question fact does NOT suppress claims
  (`internal/queue/companion/questions.go:76` unblocks on ANSWER only).
  Fell back to the plan's R1(b): `tq cancel --force` 05:02 → terminal
  `cancelled` at the worker heartbeat 05:04, claims frozen at 169
  (terminal status structurally beats the 30-min watch; final re-check
  rides F70). CV TODO row 82 annotated with disposition + re-arm path
  (CV `a53eb9c08`, pre-commit battery green); diagnosis §b annotated
  with P6 (`0be27fd5608aee9afa3d59b7f14a3ce46c295c62`, `check-status-index.sh` green).
- **M2 vendor-gofmt death class ended.** `trash vendor/` (44 flagged
  files → 0); full `.tq-verify` gate rc=0 green WITHOUT vendor/; green
  dispatch probe completed end-to-end (claim+complete 05:18:36, facts
  8627/8628) after re-enqueue at priority 90.
- **M3 dlq-fix enabled at source.** Pool conf source located (SystemNix
  `modules/nixos/services/tq-agent-pool.nix` poolSettings → nix-store
  render at deploy); `"dlq-fix" = "true"` added with rationale comment;
  treefmt/deadnix/statix/`nix flake check` (eval) all green; landed via
  SystemNix daemon ferry `c180144e` after a ref race (§d5).
- **M4 skill-validation noise killed.** go-cqrs-lite SKILL.md description
  properly measured (multi-line YAML: 1043 chars), trimmed to **1010 ≤
  1024** with every trigger phrase intact; committed `8ad3967f9`;
  fan-out verified by readlink + reading the length THROUGH the link.
- **M5 04:04 prune-death ruled AND fixed.** Full verify-failure tail:
  `TestRestartMidStreamLosesZeroFacts` (papdashboard) "dlq-550 key seen
  2 times, want 1" — **NOT vendor-gofmt** (test stage died before the
  gofmt leg). Reproduced at HEAD (~1/5): over-pinned exact counts on an
  at-least-once bridge (re-send before batch-end checkpoint is by
  design; dashboard dedupes on idempotency keys). Relaxed to lower
  bounds (≥2/≥1) with a comment citing the death; **20/20 stable**;
  root build+vet+bridge tests green; committed `5d54c1298b39fa122633a207e030c8ef31e434fb` (amended the
  daemon's generic ferry while still local-only).
- **M6 design legs F18–F19.** Citations: `verifyFor`
  `internal/executor/agent.go:944` (`.tq-verify` file → payload pin →
  `autoDetectVerify` :1023); minted Go default (:1008–1010) has NO
  gofmt stage — the gofmt leg lives in repo `.tq-verify` files;
  detection-side vendor-gofmt signature already pinned
  (`verifygate_test.go`). Scoped idiom chosen and **empirically
  verified** in a scratch git repo:
  `test -z "$(gofmt -l . | git check-ignore --stdin -v --non-matching | grep '^::')"`
  — gitignored `vendor/bad.go` passes, untracked non-ignored
  `sub/new_untracked.go` correctly fails. Tracked-only listing was
  REJECTED (agents write new files pre-commit and must stay gofmt-
  checked). Learned en route: `--non-matching` requires `-v`.
- **Session report #1** (09-56) written, indexed, committed (daemon
  ferry `faba1b8af2080602e801c95dbabc940028d6c669`); `check-status-index.sh` green.

## b) PARTIALLY DONE

- **M6** — design + verified idiom only; implementation (executor stage,
  tests, this repo's `.tq-verify` switch, module+root gates, commit)
  NOT started.
- **M3 runtime half** — deploy/pool restart owner-gated (`sudo`/
  `systemctl` banned in this shell; killing the pool only restarts the
  same immutable store conf). First `dlqfix:` autopsy mint check rides
  the post-deploy window.
- **M2/F7 deviation (deliberate)** — no mass `tq dlq --rescue`: 167
  gofmt-class dead letters span 09-11→10-01 with work landed,
  `--rescue-all` is human-gated by CLI contract; per-task rescue/dismiss
  delegated to dlqfix autopsies. Dead count unchanged (318).
- **Noticed, not chased:** dirty-repo looper `000001a0f539…` (cycles
  cheaply — preflight refuses before any paid spawn; parallel agents'
  uncommitted files) got a real run 05:13–05:18 and died on
  `internal/harvest TestSelfManagingLoop` "tick 2 enqueued=[]" which I
  could NOT reproduce (3/3 isolated) — suspected load-timing flake
  racing parallel edits. Also noticed in the index: parallel sessions
  already worked adjacent lanes (05-10 liveness, 06-35 smoke forensics,
  07-30 sweep incl. a 317-dead census + stats/doctor touches) —
  reconcile before executing M10/M11.

## c) NOT STARTED

- M7–M21 (breaker memo/impl, requeue_class, stats anomaly, doctor
  probe, dlqfix guard, alert-path verify, loop smoke, census write-up,
  budget model, CV cross-post, evidence archive + final close-out, TODO
  harvest, ask park→resume e2e, AGENTS lessons) and F70 final battery.

## d) TOTALLY FUCKED UP (owned)

1. `tq ask --expires 720h` bounced off the 168h cap — read the flag
   contract BEFORE the first invocation next time.
2. Skill description mis-measured 1022 via a first-line-only regex; the
   multi-line frontmatter block was 1043. Block-aware YAML parse or it
   didn't happen.
3. Probe enqueued at P0 and starved behind the aging ladder (P7);
   cancelled + re-enqueued at 90. Probes enter at marker band (≥90).
4. Nearly ran a bare `tq worker` against the production DB — it
   registers only `sh` but `ClaimDue` has no type filter and unknown
   type = PERMANENT error (`worker.go:567`): it would have dead-lettered
   the pending agent tasks. Caught by reading `claims.go`/`worker.go`
   first; footgun now on record as P8.
5. SystemNix commit lost the ref race to a parallel HEAD move AFTER all
   pre-commit checks (`cannot lock ref 'HEAD'`); recovered only because
   the daemon had ferried the content (`c180144e`).
6. First M1 watcher polled a non-matching regex against `tq show` JSON
   (12 empty samples, ~4 min) — read the output shape before polling.
7. The 09-56 report commit ALSO raced the daemon (`git add` then
   "nothing to commit" — ferry `faba1b8af2080602e801c95dbabc940028d6c669` got both files). Twice in one
   session: in daemon-swept repos, `add+commit` is one step or the
   daemon owns the message; verify the ferry landed rather than assuming
   loss (both times it did).

## e) WHAT WE SHOULD IMPROVE

- **P6**: operator park verb (`tq park TASK_ID --until DUR` or an
  operator mode of `tq ask`) — cancel is today's only disposition and it
  is destructive/attempt-losing.
- **P7**: fresh low-priority tasks starve for a long time; document the
  marker band for probes or add a fast-lane.
- **P8**: bare `tq worker` should refuse agent-type claims instead of
  permanently failing them (claim-time type filter).
- **Daemon-race posture**: stage → immediate commit, retry once on ref
  lock, then verify what the daemon ferried. Applies to every repo in
  this fleet.
- **Scoped gofmt at the mint**: the minted Go default carries no gofmt
  leg while repo `.tq-verify` files hand-roll unscoped `gofmt -l .` —
  the verified scoped idiom belongs in `defaultVerify` so new bootstraps
  inherit it (M6 implementation).
- **Flake posture**: two flaky-assertion classes found in one session
  (papdashboard exact-count pins; suspected harvest tick-timing). Exact
  pins on at-least-once/timing surfaces are the anti-pattern.

## f) NEXT (grouped, up to 50)

1. M6/F20: add the verified scoped gofmt stage to `defaultVerify`'s Go
   branch + export the stage constant for reuse.
2. M6/F21: executor test — green with gitignored unformatted vendor/
   present; red on untracked non-ignored unformatted file (reuse the
   `writeVendorRepo` fixture shape).
3. M6/F22: `cd internal/executor && GOWORK=off` build/vet/test gate.
4. M6/F23: root build/vet/`-race` (vendor/ deliberately absent).
5. M6: switch this repo's `.tq-verify` gofmt leg to the scoped idiom.
6. M6/F24: footer commit via `scripts/commit-task.sh` if task-ridden.
7. M7/F25: circuit-breaker memo (R3=(a): N=3 consecutive env requeues →
   attempt burn + exponential NotBefore + alert fact), docs/planning.
8. M7/F26: requeue call-site map with citations (`worker.go:421` env
   class is the anchor).
9. M7/F27: streak-counter persistence decision (in-memory vs
   fact-derived) in the memo.
10. M8/F28: consecutive-env-requeue counter + attempt burn after N.
11. M8/F29: exponential NotBefore escalation on the env streak.
12. M8/F30: alert fact / PapDashboard notify on streak ≥ N.
13. M8/F31: worker test — 3 env requeues burn an attempt.
14. M8/F32: conform-suite pin — env class never loops > N.
15. M8/F33: root `-race` battery + `scripts/test-cmd-tq.sh`.
16. M8/F34: footer commit.
17. M9/F35: `requeue_class` field in requeue fact detail.
18. M9/F36: journal/sqlite round-trip test.
19. M9/F37: legacy-facts-`unknown` backfill note in the memo.
20. M9/F38: commit.
21. M10/F39: `tq stats` claims-per-task column + threshold flag
    (RECONCILE with parallel 07-30 stats work first).
22. M10/F40: webui badge for anomalous claim counts (templ + css regen
    committed).
23. M10/F41: stats/webui tests.
24. M10/F42: commit.
25. M11/F43: `tq doctor` gitignored-gofmt probe (`git ls-files -o` ∩
    `gofmt -l` ≠ ∅) — reconcile with parallel doctor touches.
26. M11/F44: hermetic `//go:build unix` doctor test.
27. M11/F45: commit.
28. M12/F46: dlqfix default-on memo (pros/cons vs opt-in).
29. M12/F47: guard test — dlq-fix on ⇒ autopsy minted (journal fixture).
30. M12/F48: commit.
31. M13/F49: query PapDashboard API for recent `alert.triggered`.
32. M13/F50: rule fired/silent for the 176-dead window; defect if silent.
33. M14/F51: smoke red on a 30-claim fixture journal.
34. M14/F52: smoke green on a healthy fixture.
35. M14/F53: wire `scripts/smoke/loop-detector.sh` into ci-local list.
36. M14/F54: commit.
37. M15/F55: CV dead census by last_error class (reconcile with the
    parallel 317-dead census, don't duplicate).
38. M15/F56: review-task churn signature check (156).
39. M15/F57: census table into the close-out report.
40. M16/F58–F59: loop-family paid-turn total from CV crush.db vs cap 30
    - committed §-note.
41. M17/F60–F61: CV cross-post status report + CV TODO row for their
    red gate (CV is hot with parallel agents — coordinate).
42. M18/F62: `git check-ignore -v` then `scripts/archive-evidence.sh`
    for session evidence (scratch-repo idiom demo, DB extracts).
43. M18/F63: final a–g close-out + index row + `check-status-index.sh`.
44. M19/F64: harvest code-fix TODO rows ONLY after M2+M6 verified
    (grinder guard).
45. M19/F65: verify first harvested task survives the gate.
46. M20/F66–F67: ask park→resume e2e (fixture, answer, resume) +
    predicate-truth re-dispatch path.
47. M21/F68–F69: AGENTS.md lessons (env-requeue class, gate hygiene,
    scoped-gofmt idiom, P6/P7/P8) within the 15,000 B guard.
48. F70: full `scripts/ci-local.sh` battery + M1 final no-new-claims
    re-check.
49. P6/P7/P8 implementation rows (park verb, probe band docs, bare-
    worker claim filter) once M7–M9 land.
50. Triage `TestSelfManagingLoop` (harvest) like M5: reproduce under
    load, unflake or rule race-with-parallel-edits.

## g) QUESTIONS (cannot be answered from here)

1. **M3 deploy timing**: dlq-fix is committed in SystemNix but the
   running pool still has no repair loop until `scripts/deploy.sh` runs
   (sudo-gated, banned here). Deploy now so the 318-dead autopsy
   backfill starts, or ride a later window?
2. **Dirty-repo looper `000001a0f539…`**: cycles cheaply (no paid
   spawns) but rhythmically takes a pool slot and just died once on the
   un-reproduced harvest flake — cancel like M1, or let it cycle until
   M8's breaker bounds the class?
3. **Push posture**: unpushed session commits in four repos
   (go-taskqueue ~22 ahead incl. my Phase-0 legs and both reports;
   SystemNix `c180144e`; CV `a53eb9c08`; go-cqrs-lite `8ad3967f9`) —
   push now or leave local?
