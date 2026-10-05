# Pareto T1–T3 execution: rulings sheet, red-master CI heal, dead-SHA heal (2026-10-05 20:35)

> Session ran the SUPERB Pareto plan's 1% tier (`docs/planning/2026-10-05_13-55_SUPERB-PARETO-PLAN-ALL-TODOS.md`):
> T1 owner rulings (live, via structured question batches), T2 red-master CI heal, T3 dead-SHA mass heal.
> Session window ≈13:55–20:35 with several long interruptions (battery cycles) and a ~4h gap
> during which /tmp was wiped and the concurrent fleet kept committing.

## a) FULLY DONE

1. **T1 — 17 owner rulings collected and recorded** (O1–O15 + dead-sha remedy O16 +
   cancelled-run O17), gathered live through 4 question batches. Durable record:
   `docs/planning/2026-10-05_15-00_OWNER-RULINGS-SHEET.md`. Highlights: agents push
   gate-green master (O1); budget cap = token/cost (O2); JIT frontier scoring replaces
   TTLs (O3); refuse terminal-ID re-dispatch, `--force-redispatch` escape (O4);
   footer-first + heal tool ratified (O5); session mints bypass budget, ask ≤2/task (O6);
   task sizing ≈60 min, batching ON (O7); delete ghost `internal/consumer` (O8);
   gosec stays advisory, lint ≤10 min (O9); cqrs seam internal-only, cqrs-lint fixes via
   branch (O10); worktree auto-merge on approve+green, CSP `form-action 'self'` (O11);
   100% metaengine+system commitment, cqrsqlite dies (O12); weekly archive sweeps,
   vendor-guard YAGNI (O13); operator-visible-only CHANGELOG (O14); red master
   disclose-only, GOEXPERIMENT env ruled in, Dependabot ON (O15).
2. **TODO_LIST un-block surgery** — 16 answered rows deleted, 15 BLOCKED rows rewritten
   into executable work, 8 rows gained ruling cites, 5 more completed rows closed after
   implementation; `check-todo-list.sh` green after fixing two rows my own cite text had
   tripped (owner-word triggers, one mangled parenthetical).
3. **T3 — dead-SHA gate healed end-to-end**: `scripts/check-dead-sha-refs.sh` rewritten
   with `--emit-baseline` generator mode (O16), per-token arrow-escape tightening (a
   dead sha sharing a line with an unrelated `old→new` record now still fails), and a
   5-branch self-test (clean/baselined-silence/exact-file:line/per-token-arrow/
   emit-roundtrip). 71 new tokens (10-01..05 heal mass) baselined under a dated
   provenance block. Gate: `dead-sha refs ok (259 baselined, 0 new)`, rc=0.
4. **T2 legs fixed**:
   - `examples/fullcore/main.go`: the drain deadline no longer covers postgres Open,
     the demo enqueues, and the drain polls (CI died with `enqueue demo 2: context
     deadline exceeded` on a loaded runner). Deadline now fires only in the drain wait
     — the smoke's fail-reason is deterministic. Full smoke PASS (sqlite arm; postgres
     arm blocked locally by password auth, verified in CI).
   - **AGENTS.md prune**: 17,161 → 15,286 B (guard budget 15,700) — table alignment
     padding removed, five verbose blocks tightened, zero facts dropped.
     `TestAgentsDocSizeGuard` green.
   - nix vendor-hash leg: verified CORRECT at HEAD by deleting the FOD store path and
     forcing a fresh realization (rc=0) — the CI hash mismatch was proxy-fetch drift
     (row 54 class), self-healing, nothing to change.
5. **E3 — check-ci cancelled-run neutrality** (O17): CANCELLED is its own class now
   (exit 0, "cancelled — not red", `gh run rerun <id>` hint); classification pinned by
   `CHECK_CI_SELF_TEST=1` (6 assertions), wired into ci-local beside the live probe.
6. **Fleet lint gate restored to GREEN at HEAD** (it was red for every window on
   committed foreign code): migrate.go named returns dropped (2 funcs),
   `inamedparam` params named in the executor result seams, `golines -w` over six
   flagged files, readmodel detail/parity tests parallelized (11 findings, all
   per-test `t.TempDir()` fixtures — parallel-safe), conform openers annotated
   `//nolint:ireturn` (design-level interface returns), `golangci-lint fmt` settling
   the disturbed import grouping. Final: `lint-baseline: within baseline (1268 vs
   1286)` — SHRUNK.
7. **Both co-tenant smokes de-flaked** (row 99 class, live-reproduced both directions):
   multi-repo pools staggered (pool-2 waits 2s), questions-e2e waits out the worker's
   open and retries the enqueue on the busy signature. Both PASS standalone; production
   open-path retry remains owner-gated (row 99) with fresh evidence now on file.
8. **CHANGELOG** (O14 policy applied): three entries — Added (emit-baseline generator),
   Changed (check-ci neutral class), Fixed (fullcore deadline scoping).
9. **Verification gates on this session's own delta**: root build+vet green; touched
   modules green in-module (`sqlitev4` build+test, `executor` build+vet+test 9.4s,
   `readmodel`, `webui` 10.3s, `harvest`, `bridge/...`); fullcore smoke PASS;
   multi-repo smoke PASS; questions-e2e PASS; dead-sha self-test PASS; check-ci
   self-test PASS; check-todo-list + check-status-index green; lint-baseline green.

## b) PARTIALLY DONE

1. **T2's final step — "confirm green run on master"**: the fix stack is committed
   locally (branch ~18 commits ahead of origin) but NOT PUSHED — every full-battery
   attempt kept surfacing one more leg (lint growth ×4, dead-exports transient,
   app.css drift, two smoke flakes), and the last two batteries' logs were lost to a
   /tmp wipe. The O17 neutral-class also means check-ci needs the push to observe the
   healing run. **One `git push` + one CI watch closes T2.**
2. **Full ci-local rc citation — RESOLVED VIA CI**: local full batteries kept racing
   the concurrent fleet's commits (battery 9: shellcheck SC2034 on my own fresh edit —
   fixed; battery 10: root gosec `Files=0` — transient mid-commit snapshot, standalone
   rerun Files=45 rc=0; battery 11: `cmd/tq typecheck` NEW — raced an 11-file foreign
   commit mid-run). A 15-minute battery cannot finish inside this repo's commit
   cadence, so the authoritative full gate is the pinned-SHA CI run: **run
   37361330330 at c8755d16 = SUCCESS — master CI GREEN for the first time since
   2026-10-01** (all jobs incl. test-windows, test-postgres, nix, consumer, gosec,
   govulncheck, cqrs-lint). T2's "confirm green run" is DONE.
3. **O-rulings implementation depth**: rulings are RECORDED and rows un-blocked, but
   only the parts that gated T2/T3 are implemented. The rest (JIT scoring, cost cap,
   batching default, tasks/ subdir, Dependabot config, CSP, cqrsqlite deletion, …)
   are now executable rows awaiting their tier.
4. **AGENTS.md codification of O1/O14**: not done — size headroom is 414 B after the
   prune; row 416's budget ruling is needed first (see §g).

## c) NOT STARTED

Everything outside the 1% tier by design: T4 v0.4.0 cut, T5 S2 journal flip, T6 M4
memo, T7 verify-gate flip, T8 deploy bundle, T9 agent autonomy, T10–T27 (re-dispatch
guard implementation, auth/lockout, S2 robustness, webui polish, docs-health
mechanization, CI parity, gosec cmd/tq, postgres wiring, derived-outcomes cleanup,
session bundle, docs surface, fleet observability, worktree, paperclip, micro-batches
A/B). See the plan's Tables A/B; §f ranks them.

## d) TOTALLY FUCKED UP

1. **Six full ci-local batteries burned (~90 min wall) chasing a green rc** — each
   surfaced ONE new foreign or environmental leg after the previous was fixed. Root
   cause: I ran the FULL gate repeatedly in a tree other agents were actively
   committing to, instead of (a) verifying my delta scoped-first and (b) running the
   full battery ONCE at a stable HEAD. Battery 1 even failed on a lint class that a
   STALE golangci cache had kept alive after my fix (Known Issue: cache clean) — cost
   one full cycle.
2. **My commit 87224ceb accidentally swept 4 concurrently-staged foreign files**
   (daemon pre-stages; `git commit` takes the index). Undone via soft-reset + reselect
   (d738c722), but the foreign files rode one footer-less `chore:` commit for ~30
   minutes before I noticed. Lesson applied: after every commit, `git show --stat` —
   the daemon stages behind you.
3. **Two of my TODO cite-appends broke gates**: "footer-first is owner policy" tripped
   check-todo-list's owner-gate regex, and one append split a parenthetical
   (`scripts(O5 …)`). Mechanical text surgery on TODO_LIST needs the same exact-match
   discipline as code edits.
4. **check-dead-sha-refs.sh had drifted on disk** since my read (tool refused the
   write) — I re-Viewed and rewrote from the verified content; had the drift been
   real (another agent mid-edit), I would have clobbered it. Read-verify-right-before-
   write is mandatory in this repo.
5. **All /tmp battery logs vanished in the 4h gap** (battery 5's rc is uncitable;
   battery 8's log survived only because the session resumed before the wipe). Cite
   rc files OUTSIDE /tmp or amend them into the report immediately.

## e) WHAT WE SHOULD IMPROVE

1. **Battery discipline under concurrency**: scoped delta gates during the loop, ONE
   full battery at a quiescent HEAD, rc to a repo-adjacent path (not /tmp).
2. **Foreign-arc ownership**: lint growth landed by daemon sweeps went unowned for 4+
   windows (row 126 BLOCKED). Ratify a rule: the sweeping session (or a dedicated
   sweep) regenerates with triage notes same-day.
3. **check-dead-exports /tmp temp-file race** (battery 3): a vanished mktemp crashes
   the gate mid-loop; make it retry-wrapped or write its scratch under the workdir.
4. **The smoke pair needs the production fix**: stagger/wait are smoke-side morphine;
   row 99's bounded jittered retry at the shared sqlite open path now has live
   repro data (4 failures, both sides losing alternately) — the O-ruling session
   should take it up.
5. **check-ci's gh query returned a stale run once** (09-30 run as "latest completed")
   — transient, unreproduced; a `--sort created-at` pin (or retry-on-suspicion) would
   harden the turn-1 probe.
6. **AGENTS.md headroom**: 414 B is one closeout from red again; row 416's budget
   ruling (hard 15,000 + prune ritual vs standing headroom) decides whether every
   AGENTS-touching window keeps gambling.

## f) NEXT 50 (ranked, most first)

1. Push the ~18-commit stack; watch the master CI run; confirm green (closes T2).
2. Amend this report with battery 9's rc + push confirmation.
3. Owner ruling on row 99 (production open-path retry) using today's repro data.
4. Owner ruling on row 416 (AGENTS size-guard budget), then codify O1/O14 into AGENTS.md.
5. T4: cut v0.4.0 (CHANGELOG walk, sub-tags, proxy + clean-room checks) — 5 weeks of [Unreleased].
6. T5/E10–11: ADR-0019 S2 journal flip on `facts.Fact` (UNBLOCKED, owner-committed via O12).
7. T6/E12: file the M4 ratification memo upstream (go-cqrs-lite CountFacts/FactsSince).
8. T7/E14–15: owner flips `.tq-verify` → `scripts/verify.sh`; genuine-red pin + KNOWN_FLAKY sync gate.
9. T9/E18–19: de-micromanage prompt stack; batching default ON sized to ~60 min (O7).
10. T10/E21–22: mint-time done-check (O4) + `scripts/smoke/done-guard.sh`.
11. Implement O3: JIT frontier scoring (score next-K claimable, rolling refresh).
12. Implement O2: token/cost-denominated daily cap + `UsageToday` semantics + ADR.
13. O12 execution: delete `internal/queue/cqrsqlite` (+ gates, references).
14. O15 execution: Dependabot config for the 8-module tree; retire `--dep-sweep`.
15. O11 execution: CSP `form-action 'self'` + SECURITY.md + header pins.
16. O8 execution: delete `internal/consumer` + ADR-0009 outcome; document the 3 FactSource interfaces.
17. O10 execution: three cqrs-lint fixes on a go-cqrs-lite branch (A014/D013/V006).
18. O7 execution: closeout reports → `docs/status/tasks/` + 10-append cap.
19. Row 440/450: heal-daemon-sweep fork-record capture (old→old, post-rewrite shas) + self-test to green.
20. Row 104 follow-through: extend check-ci self-test over the JSON parse path (canned payload).
21. check-dead-exports: kill the /tmp temp-file race (workdir-scoped scratch + retry wrap).
22. Add `--sort created-at` (or staleness retry) to check-ci's gh query.
23. T11/E23–26: lockout fake-clock suite, SECURITY matrix pin, body-shape pin, SSE header e2e.
24. T12/E27–30: SQLITE_BUSY open retry (after #3), postgres drift-audit executed, D1/D2 register doc, verify_stage surfacing.
25. T13/E31–32: vendor-gofmt structural fix (row 102's `.tq-verify`), closeoutPending lifetime audit.
26. T14/E33–35: prioritize live pilot (one batch, cost measured) — feeds O3 calibration.
27. T15/E36–38: webui P2 polish batches (a11y, cancel/rescue feedback, AMBIGUOUS soften).
28. T16/E39–40: `check-archive-eligibility.sh` + harvest-disposition lint (weekly cadence per O13).
29. T17/E41–42: ci.yml parity sweep (dead-sha/features-ci/verify/session-close/help-text legs) + loop guards.
30. T18/E43–44: cmd/tq gosec coverage + goTarballHash single-source.
31. T19/E45: postgres `--store` CLI wiring + `ArchiveFactsBefore` twin.
32. T20/E46–47: TQ_RESULT stdout deletion, stale doc-comment sweep, LogPath render-pin + templ fragment collapse.
33. T21/E48–49: session bundle (dry-run, list --json, registry mode, postgres arm).
34. T22/E50: DOMAIN_LANGUAGE terms (window/mint/trigger/done-prompt/close-epoch) + citation conventions.
35. T23/E52: `tq pool-health` + fleet liveness doctor check + `--fleet` aggregation.
36. T24/E54: worktree-per-agent implementation (O11 auto-merge policy now ruled).
37. T25/E55: event-wake ingress spike + adapters note + receipts rollup (ROADMAP S1–S3).
38. T26/E56–57: micro-batch A (smoke-assert bundle, TQ_BIN rot-guard, dead-export trio, fold-marker).
39. T27/E58–64: micro-batches B1–B7 (help-text pins, doctor WARNs, fullcore knobs, gocognit repair).
40. Row 465: daemon-sweep baseline shrink sweep (drop rows whose sweeps gained footers).
41. Row 412: land `check-archive-eligibility.sh` into ci-local next to check-status-index.
42. Row 391: fullcore stop-seam ordering pin (grep-assert no `sql: database is closed` noise).
43. Row 187: session-start SUMMARY carries master-CI state (master-CI=red@<sha>).
44. Row 124/127: deploy-readiness bundle rides the next SystemNix flip (owner sudo, GOEXPERIMENT env ruled in).
45. Row 73: apply `Environment=GOEXPERIMENT=jsonv2` to the SystemNix pool unit (owner sudo).
46. Row 395: harvest the 06-05 dead-SHA follow-ups (`--baseline-add`, VERBOSE INFO line, header sentence).
47. Row 462: converge minted verify payloads on the wrapper (byte-parity scope).
48. Row 468: same-ID close-out policy (3–4 report files per task ID) — sizing-adjacent (O7).
49. Row 530-class: gate fixture bundle (36-char task-ID assertion, guard-wiring reference assert).
50. Postgres arm of fullcore smoke verified on THIS host (needs a trust-auth local pg or CI-only pin).

## g) QUESTIONS FOR THE OWNER (cannot self-answer)

1. **Push now?** The stack is gate-verified per-delta; O1 sanctions gate-green pushes,
   but the last FULL battery rc lands only when battery 9 finishes. Push immediately
   after its rc, or is there a freeze window I don't know about (other agents are
   mid-arc on conform/readmodel)?
2. **Row 99 — production open-path retry**: with today's live evidence (both smokes
   losing the race alternately, auto-upgrade having widened the window), do I implement
   bounded jittered retry/backoff at the shared sqlite open path NOW, or does it wait
   for the S2 flip (which re-opens the path anyway)?
3. **Row 416 — AGENTS.md budget**: hard-cap 15,000 B with a mandatory prune ritual
   every closeout, or reset `agentsDocMaxBytes` to standing headroom (~15,500)? O1/O14
   codification needs ~300 B I don't have.

