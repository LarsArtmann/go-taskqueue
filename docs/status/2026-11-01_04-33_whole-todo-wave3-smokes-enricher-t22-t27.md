# Session closeout — whole-TODO wave 3: E-gap smokes, row-1446 enricher, T22/T23/T24/T25/T27 batch, twin heal

**2026-11-01 04:33 CET · Continuation under the owner's standing
"WHOLE TODO LIST" GO (handoff: docs/status/2026-11-01_02-24_e-gap-proofed-ci-red-fixed.md).
A CONCURRENT agent worked the same row-1450 territory in parallel the
whole session (their crush-server-route.sh landed first and CORRECTLY —
see §d1); this session reconciled with their findings, shipped the
companion legs, and closed the T22/T23/T24/T25/T27 batches.**

## a) Work commits (this session, footered; sweeps grandfathered)

- `594adec3` twin reconciliation (rebase dropped local 631e7e08 vs origin
  9aea94ee, identical patches) + `bdb8a148`/`594adec3` grandfather tails
- `ef04b683` planning doc E7 — v0.99.0 HTTP envelope captured live
- `e44de143` crush-pool-supervisor smoke + row-1450 tick (+COVERAGE)
- `e48fae38` ci-local wiring: postgres-conform (the CI-red orphan) + both
  crush smokes
- `b18b6162`..`8c611207` row-1446 enricher landing + T25 grandfathers
- `7a34a89e` check-vendor-hash.sh + drop-day/provenance runbook (T22)
- `13a0585f` postgres seeded-drift + fullcore EXECUTED (rows 462/534)
- `4412ae25` httpapi replay-guard warn + ADR-0016 S4 addendum (row 843)
- `9ebea110` status-rewind smoke (row 654) + `daa97758` .tq-verify restore
- `786adaed` consumer caller-inventory closure (row 399)
- `ae3d7763` e2e derived-outcome through Execute (row 806 slice 1)
- `843d8d13` webui-live-contract pane section + drafts retired (T27)
- `1567bb18` windows-red cures (unix tag, CloseReadModel, treefmt)

## b) Done (verified)

1. **Twin reconciled** — `git pull --rebase` dropped the same-subject
   twin (patches byte-identical modulo the footer); attribution green at
   every step (1539 baselined at close).
2. **E7 HTTP envelope** (planning doc): `/v1/docs/openapi.json` is the
   72-path route authority; workspace registration doorway (POST
   {client_id, path}); sessions/messages wire shapes captured raw
   (tool_call/tool_result/finish parts, input+metadata as JSON strings).
3. **Row 1450 E-gap** — TWO smokes + one upstream-blocked canary:
   concurrent agent's `crush-server-route.sh` (wire legs, green) + this
   session's `crush-pool-supervisor.sh` (routed 429 fast-refuse under
   the executor's exact `CRUSH_CLIENT_SERVER=1`+`-H` argv — rc!=0 +
   rate-limit text, env-gap-INDEPENDENT; keeper/supervisor live: kill -9
   → "restart 1" + new pid, SIGTERM → clean pool exit, no orphan). Both
   ci-local-wired (crush-gated SKIP); COVERAGE rows; tick cites the
   upstream-blocked env-channel residue.
4. **Row 1446 structured closeout enricher** — internal/executor/
   crushserver.go: routed runs register their own workspace over the run
   repo, resolve the session (stdout id WINS on match; window discovery
   ONLY when stdout gave nothing; mismatch → nil, never split-brain),
   fill usage from the summary, fold parts into AgentResult.ToolCalls
   (command redacted, duration, working dir, error flag). Fallback
   pinned (unreachable server leaves stdout values byte-identical);
   live-unix-socket server leg + E7 fixtures; facade alias; CHANGELOG.
5. **T22** — scripts/check-vendor-hash.sh (fast FOD realization +
   got→flake recipe + countable --self-test, ci-local-wired); runbook
   docs/planning/2026-11-01_03-30_vendorhash-drift-and-drop-day-watch.md
   (provenance: vendorHash = f(go.mod graph) ONLY; four-block drop-day
   list); rows 1389 + 504 ticked (no drift at HEAD).
6. **T25 EXECUTED** — row 462: TestJournalDriftSeededDriftAllFieldsPostgres
   (raw UPDATE tasks vs the store schema; status/attempts divergence +
   priority/dedup excluded per S1 D2 + honest coverage) green on a live
   scratch cluster; ci.yml step widened to `TestJournalDrift.*Postgres`.
   Row 534: fullcore.sh postgres variant PASS live (drain 4/4 + deadline
   exit-1 path).
7. **T23 batch** — row 843 (replay-guard wmErr branch WARNs + stateful
   second-read-failure pin); ADR-0016 S4 composition addendum; row 654
   status-rewind.sh (watermarks set 0 → re-sweep → done-guard holds
   exactly one report; stub work-turn stays commit-free — see §d3);
   row 399 closed: internal/consumer has ZERO production importers, the
   removed-flag blast radius is empty (per-caller verdicts in the tick).
8. **T24 slice** — row 806 slice 1: TestExecuteDerivesOutcomeFromRealRun
   (real stub-agent process, real footer commit, real verify gate; Sink
   detail must carry derived commits/subject/SHA/files/verify-tail with
   NO self-report). Slices 2-3 annotated open.
9. **T27** — webui-live-contract gains the questions-pane + answer-route
   section (busy-deferral on the answer input, 409 semantics,
   next-snapshot clearing); both drafts triaged FILED+retired (go-cqrs-
   lite#56 2026-10-07, art-dupl#9 2026-10-20).
10. **CI-red cured twice** — postgres-conform guard-wiring orphan (the
    10-31 smoke landed unwired; wired into ci-local initdb-gated) and the
    windows-red trio on my own tree (unix build tag on the git e2e test,
    CloseReadModel before TempDir cleanup, treefmt alignment).

## c) Not started / blocked (unchanged)

- Owner-blocked: M4 ratification memo + upstream seam filings; S2
  journal unification; ADR-0019 S2; L4 pilot; verdict-channel env cure
  (upstream crush ws.Env→tool exec).
- Row 806 slices 2-3 (FailureEvidence.FilesChanged, doctor verdict-PATH
  probe); prompt-stack de-micromanagement (row 470); the harvest tail.

## d) Totally fucked up (worst first)

1. **I re-committed the previous session's env-propagation false
   positive.** My 02:49-style "verdict lands through $TQ_RESULT_FILE"
   probe ran `-H` WITHOUT `CRUSH_CLIENT_SERVER=1` — silently LOCAL (rc
   0, sessions in the repo db regardless). I nearly shipped a smoke
   asserting the opposite of the concurrent agent's (correct) canary.
   Adjudicated by adversarial isolation (variant matrix on a virgin
   port, /proc/<pid>/environ proving the server never saw the var):
   `-H` alone = local+env-present; `+CRUSH_CLIENT_SERVER=1` = routed+
   env-BROKEN. Their M8 amendment and canary are RIGHT; my E7 draft was
   corrected in place by them. Lesson: the routing DISCRIMINATOR is the
   workspace listing mid-run, never rc-0.
2. **My debug stub-agent committed to the MAIN repo.** Running the stub
   directly (not via the pool) left cwd = go-taskqueue; its `git add -A
   && git commit` overwrote the REAL .tq-verify with the smoke fixture
   (eb55abe1 "work done") and the daemon pushed it. Cured by
   content-restoring commit `daa97758` (no history rewrite — it was
   pushed). Lesson: stub agents that commit must `cd` into their repo
   argument or refuse to run; never invoke one with a bare cwd.
3. **The rewind smoke's first cut dead-lettered its own task** — the
   stub's work branch committed an unchanged tree (git exit 1) three
   times. Diagnosed via `tq tasks` on the scratch db (LAST ERROR carried
   the stub's own output). Fix: work branch stays commit-free (comment
   in the smoke).
4. **Battery runs 5/7/8/9 died to four distinct classes**: shared-
   toolchain mid-extraction race (battery 5, known class), `go mod
   verify` walking the 16G shared module cache (~25 min PER module
   verify — battery 7 stalled 40+ min; cured with a PRIVATE
   GOMODCACHE/GOCACHE for the battery), check-ci racing the in-flight
   fix run (battery 9; conscious CI_CHECK=off per the gate's own
   ancestor-red guidance), and a vendor drift healed in-place by the
   gate itself (battery 10, re-run).
5. **Daemon races ate five staged commits** (enricher, T25, row-654,
   row-806, red-cures) — every one recovered by content-verify +
   grandfather; nothing lost, but each cost a verify-and-baseline round.

## e) Lessons (candidates for AGENTS if they recur)

- `go mod verify` on a SHARED 16G GOMODCACHE is hours-cold: run the
  battery with a private GOMODCACHE when the shared cache is under
  concurrent churn (or gate the verify leg's cache walking).
- The crush client-server ROUT discriminator: `GET /v1/workspaces` lists
  the workspace ONLY under `CRUSH_CLIENT_SERVER=1`; `-H` alone is a
  silently-local run (rc 0, same dbs). Probes must assert the listing,
  never rc.
- Stub agents that git-commit are cwd-loaded guns: invoke via the pool
  (it sets --cwd) or make the stub cd into its argument.

## f) Next (impact-ordered)

1. Battery-11 receipt + green CI watch (in flight at report time).
2. Row 806 slices 2-3 (FailureEvidence.FilesChanged for DLQ autopsies;
   doctor verdict-PATH probe).
3. Row 470 prompt-stack de-micromanagement (big, needs a quiet window).
4. The env-channel cure decision: drop `CRUSH_CLIENT_SERVER=1` from
   agentEnv IF `-H` alone routes (it does NOT — `-H` alone is LOCAL;
   the cure is upstream ws.Env→tool-exec, or move verdict/question out
   of env entirely).
5. L4 pilot + shared-server memory measurement (owner-gated).

## g) Owner questions (carried, still open from the 02-24 report)

1. The auto-git daemon PUSHES go-taskqueue master — known/intended?
   (This session again treated every local commit as CI-live within
   minutes; two red windows were live on origin before cures landed.)
2. Twin-commit reconciliation policy (this session: rebase-drop when
   patches are byte-identical — confirm as the standing rule).
3. Disposition of the 3 accidental real provider calls (~150 tokens,
   2026-11-01 §d1 prior report) — still unruled.
