# Library Fleet Review — Session Self-Audit (2026-11-09)

Owner-demanded post-run audit of the 22-library utilization review session
(ran across 2026-11-08→11-09; no task footer; direct interactive session).
Report scope: THIS session only.

**Scope line.** VERIFIED 2026-11-09 by: `ls docs/review/libs/*.md | wc -l` → 22;
`grep -l '^\*\*Verdict\|^## How to verify' *.md` → 22/22; `./scripts/check-doc-refs.sh` → ok (3 runs, last after final AGENTS.md edit);
`go test -run TestAgentsDocSizeGuard` in cmd/tq via tracked go.work → ok; index.html structural checks (python3 HTMLParser: 20/20 relative
links resolve, 0 broken anchors, 0 demo placeholders; ffprobe 1440x3400 render). NOT verified: (1) visual appearance of index.png — this
session's model rejected image input, so the render was verified structurally, not by eye; (2) the 22 per-report "How to verify" blocks were
executed by the writing subagents (each reported corrections applied), re-run sampled (go-sse, go-retry tags/go.mod greps) but not all 22 by
the lead; (3) root `go build/vet` and full ci-local NOT run — change set is docs + one AGENTS.md line, no Go source touched;
(4) `check-dead-sha-refs.sh` NOT run (cheap-battery member that should have been).

## a) FULLY DONE

- 22 library reports at `docs/review/libs/<lib>.md` (one per requested lib incl.
  the httputil root + server_timing + endpoints trio), ~1,986 lines total;
  uniform skeleton (Verdict/Pinned/Latest/Prior/Reviewed + §1-6 + How to verify);
  every report dated 2026-11-08, verdicted, with a verify block (verified by
  counts above).
- Version currency per lib pinned against local checkouts (`git tag -l`) —
  findings: 8/9 adopted on latest; go-cqrs-lite 4 modules behind
  (metaengine v4.23.0, sqliteengine v4.8.0, system v4.18.0 w/ AggregateFn
  SQL-injection hardening, id v4.7.5); go-datastar behind v0.7.0 with a
  root-v0.6.3 / cmd-tq-v0.6.2 split.
- `docs/review/libs/index.html` — self-contained Bauhaus-dark dashboard
  (kit template copied + body edited in place, per skill rule): stat cards,
  9-row adopted score table (sorted desc), 13-row verdict table, 4
  cross-cutting cards, verify block. `index.png` (728 KB, 1440x3400)
  headless-rendered via firefox and kept in-repo, linked from the index.
- Adoption scores with shown inputs: go-sse 95, cqrs-lite 82 (27/33),
  templ-components 78, health-dashboard 68, codec 63, retry 44, health 32,
  datastar 17, error-family 13; zero MISUSED rows anywhere.
- Two prior deep-dives explicitly superseded and cited (2026-10-05 cqrs,
  2026-09-14 templ — templ 64→78 since September).
- Reachability: `docs/research/README.md` row added; AGENTS.md stale
  "httputil NOT adopted (proprietary)" corrected to "(MIT v1.5.0+; see
  docs/review/libs/)" within the size budget — `TestAgentsDocSizeGuard` green.
- Subagent orchestration recovered from provider failures (see §d) and all
  six writer agents completed with self-run verify blocks.

## b) PARTIALLY DONE

- **Memory**: AGENTS.md carries only a compact pointer; the durable verdict
  detail (datastar behind, server_timing trial, cqrs wave) lives in the
  research README row + reports, not AGENTS — the size guard (26,8xx/26,800
  headroom was 190 B) forced the trim. Deliberate, but thinner than the
  library-deep-dive skill's memory bar.
- **Render verification**: structural + dimension checks passed; no human/model
  eyeball on index.png (model cannot view images this session).
- **Lead-side verify sampling**: index.html's own verify commands run by lead;
  per-report blocks trusted from subagent reports + spot samples, not re-run
  in full.
- **TODO_LIST harvest**: NOT done — §f below carries `<!-- fresh:skip -->`
  pending the owner decision (§g q1); the docs-gates doctrine wants harvest
  or skip-marker, skip-marker chosen to avoid minting paid tasks unasked.

## c) NOT STARTED

- Action items the reviews recommend (all of §f 1-23): review-only session by
  instruction; nothing was implemented, upgraded, or refactored.
- `check-dead-sha-refs.sh` NOT run during the window; run in the post-report
  amendment — clean for this session's files (2 pre-existing dead SHAs in a
  foreign task report routed to §f33). `check-script-syntax.sh` still unrun
  (docs-only change; §f28).
- Full `scripts/ci-local.sh` battery (docs-only change; scoped gates run).
- Linking the new series from README.md / FEATURES.md (not requested;
  discovery currently via research README + AGENTS pointer).

## d) TOTALLY FUCKED UP!

- **Subagent stampede → 4 instant failures**: dispatched all 6 writer agents
  at once; the provider rate-limited 4 of them to immediate death. Recovered
  by staggering (≤3 concurrent) + one provider hop; athena additionally died
  on gemini "payment required" before landing on zai/glm-5.3-flash. Cost:
  ~2 wasted wakeups and a mid-run session salvage (athena had 101 tool calls
  of research and zero files written when it first died).
- **AGENTS.md size-guard whack-a-mole**: edited first, read the budget after —
  then iterated the same line 4 times (+401 → +102 → +86 → +15 → ok). Should
  have read `agentsDocMaxBytes` and measured the file BEFORE the first edit.
- **Vendor-polluted counts early on**: my import-count grep used `-h`, which
  defeated the `grep -v vendor/` filter (reported 122 files for go-error-family
  vs the true 4). Caught minutes later; did not leak into any report.
- **Two dirty-tree gate runs**: ran `check-doc-refs.sh` with AGENTS.md
  uncommitted twice; the tq-bash-guard commit-before-gate advisory fired both
  times (2026-11-03 §e2 class). No daemon race materialized, but the pattern
  is exactly the one that burned the …918bf0 session.

## e) WHAT WE SHOULD IMPROVE!

- Stagger write-subagents by default (2-3 per provider, first wave smallest);
  a 6-wide burst on one provider is a self-inflicted outage.
- Measure guarded budgets before editing guarded files (size guard cost 4
  iterations ≈ 4 wasted gate runs).
- Cheap verify-window battery has members I skipped (dead-sha-refs); docs-only
  is not an excuse — the battery exists precisely because docs land cheap.
- Render gates need a vision-capable check or a pixel-diff harness; structural
  HTML checks prove well-formedness, not appearance.
- Lead should re-run a random sample (≥3) of subagent verify blocks at
  closeout, not rely on summaries (this time: sample of 2).

## f) Up to 50 things we should get done next

<!-- fresh:skip: owner decision pending (§g q1) on minting tq tasks from this list; items also live in docs/review/libs/*.md §4 + index.html -->

1. depsweep: metaengine/v4 v4.20.0→v4.23.0 (go-cqrs-lite.md §5)
2. depsweep: metaengine/sqliteengine/v4 v4.6.0→v4.8.0
3. depsweep: system/v4 v4.16.0→v4.18.0 (AggregateFn SQL-injection hardening — security class)
4. depsweep: id/v4 v4.7.4→v4.7.5
5. go-datastar lockstep bump v0.6.x→v0.7.0 incl. static v0.7.0 + heal root/cmd-tq v0.6.3/v0.6.2 split
6. webui: replace hand-rolled serveDatastarSDK with ScriptHandler (ETag + versioned client v1.0.4)
7. executor: route decodePayload[T] through codec.JSONCodec{}.Decode (one JSON door)
8. error-family: give PermanentError a family; collapse Permanent onto WrapOnce (kills parallel taxonomy at internal/executor/executor.go:45-63)
9. retry: execWithTransientRetry → DoWithValue (agent.go:1121-1137) <!-- fresh:skip: pending §g q1 ruling; canonical home go-retry.md §4 -->
10. retry: OnExhausted/ErrExhausted signal in persistOutcome for 3x-exhausted store-busy writes <!-- fresh:skip: pending §g q1 ruling; canonical home go-retry.md §4 -->
11. webui health: populate StatusSince/TotalLatencyMs in finishEval (dashboard rows render empty)
12. webui health: recover() around evaluate (pusher-goroutine panic crashes tq serve)
13. health-dashboard: WithWebhook → existing PapDashboard alert channel (store down, wedged fold)
14. health-dashboard: WithTrend + WithMetrics after webhook
15. templ: document adopted navigation.Pagination in internal/webui/ADOPTION.md (invisible to guard regex)
16. templ: swap hand-rolled refreshMeta → layout.MetaRefresh <!-- fresh:skip: pending §g q1 ruling; canonical home templ-components.md §4 -->
17. templ: Table.StickyHeader: true on the ledger table <!-- fresh:skip: pending §g q1 ruling; canonical home templ-components.md §4 -->
18. sse: surface Broadcaster.Health() on /health (subscriber/buffer state) <!-- fresh:skip: pending §g q1 ruling; canonical home go-sse.md §4 -->
19. sse: adopt ssetest.Require* assertions in webui tests <!-- fresh:skip: pending §g q1 ruling; canonical home go-sse.md §4 -->
20. sse: tidy v0.6.2 residue across 11 nested go.mods
21. trial: httputil/server_timing behind RequestLog flag on tq serve (the one WORTH A TRIAL)
22. cqrs-lite: WithPrefetch on TasksPage reads
23. cqrs-lite: optional token-gated /metrics (otel/prometheus) — decide yes/no on observability need
24. owner ruling: harvest §f 1-23 into TODO_LIST.md (mint tasks) or leave report-only
25. vision pass on docs/review/libs/index.png (or pixel-diff harness) to close the render gate properly
26. run ci-local once after the daemon settles (this window never ran the full battery)
27. re-run 3 random report verify blocks (lead-side sampling debt)
28. run check-script-syntax for this window (amendment: check-dead-sha-refs run post-report — clean for this session's files; 2 pre-existing dead SHAs found elsewhere, §f33) <!-- fresh:skip: half-executed in this amendment; residue routed to §f33 -->
29. consider agentsDocMaxBytes conscious reset so AGENTS can carry full fleet verdicts (guard message offers it)
30. post-wave refresh trigger: re-audit go-cqrs-lite.md + go-datastar.md once the bumps land (score deltas)
31. cqrs-lite.md §3 scan-caching opportunity (52-row inventory) — fold into the wave task <!-- fresh:skip: folds into §f1-4 wave; canonical home go-cqrs-lite.md §4 -->
32. research README: mark the 2026-10-05 cqrs row SUPERSEDED inline (it is only noted in the new row today)
33. heal dead SHAs 64b22f25 / 573acfee in docs/status/tasks/2026-11-08_18-16 task closeout — pre-existing, surfaced by this window's post-report dead-sha gate run, not this session's citations <!-- fresh:skip: not this session's report; routed for the next docs-health pass -->
34. INDEX BLOAT: 421 live rows in docs/status/README.md > 400 threshold — run the ratified monthly digest ritual (ANNOTATE+ARCHIVE sweep + digest row) <!-- fresh:skip: fleet ritual, owner-scheduled (08-25 f20); this window added 1 row to a 420-row index -->

## g) Questions I cannot figure out myself

1. **§f harvest policy**: mint tq tasks from f1-f23 (they become paid,
   dispatched work) or keep them as report-only recommendations until you
   cherry-pick? (docs-gates freshness currently satisfied by the skip marker.)
2. **Upgrade train**: one `--dep-sweep` run covering the cqrs-lite 4-module
   wave + go-datastar v0.7.0 now, or hold both for the v0.4.3 release train
   (system v4.18.0 carries a security-class fix — does that force "now")?
3. **AGENTS.md budget**: consciously reset `agentsDocMaxBytes` upward so fleet
   verdicts fit in memory, or keep the compact pointer and accept the reports
   as the detail surface?

— Recorded 2026-11-09_00-50; session then WAITS FOR INSTRUCTIONS.
