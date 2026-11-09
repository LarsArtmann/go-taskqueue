# Library Fleet Review — Pareto Plan EXECUTION: Status Report (2026-11-09_08-54)

Execution session for `docs/planning/2026-11-09_03-30_libs-fleet-review-pareto-execution-plan.md`
(M1–M20 / F1–F55), launched by the owner's blanket "execute the whole
list" order. The two owner gates (M11, M16) were ruled by adopting the
plan's §5 recommendations — recorded in plan §7.

## Executive summary

**15 of 20 M-tasks done, 1 partial (M13), 4 not started (M14, M15, M19,
M20). 45 of 55 F-items done.** The 1%→51% core landed: the cqrs-lite
security-class version wave, datastar v0.7.0 lockstep, the health
crash-recover, and the PapDashboard webhook relay. All scoped tests green
per module; the FULL battery (M15) has NOT yet run over the final tree —
it is the gate that turns this into "verified done" and is the first
thing the next window should run.

## a) FULLY DONE

1. **M1 cqrs-lite version wave** (F1–F8): tags verified pushed upstream
   (`ls-remote`: system/v4.18.0, metaengine/v4.23.0, sqliteengine/v4.8.0,
   id/v4.7.5) → F2 noop; per-module `go get`+tidy across 13 modules (24
   pin sites, zero stale); root vendor; vendorHash FOD green
   (`nix build .#checks.x86_64-linux.vendor-hash` + direct
   `goModules` realize); facade parity OK; **20-module build+vet+test
   loop PASS**; root `go test ./... -race` PASS; `test-cmd-tq.sh` ran
   with ONE failure — pre-existing, see §e1. CHANGELOG `[Unreleased]`
   rows cut (F10).
2. **M2 datastar v0.7.0 + ScriptHandler** (F11–F14): root+cmd/tq+
   static → v0.7.0; `serveDatastarSDK` replaced by
   `datastar.ScriptHandler()` (ETag/304/HEAD/405) + versioned script URL
   (`?v=<static.Version>`); SECURITY.md gains the ETag row and the Go
   header matrix gains `serveSDKRow` (same-change rule honored).
3. **M3 health crash-recover + metadata** (F15–F17): `evaluateSafely`
   recovers panics into a degraded fail response (pusher can no longer
   kill `tq serve`); `StatusSince` carry-through + `TotalLatencyMs` in
   `finishEval`; pinned by `TestHealthEvaluationPanicRecovers`,
   `TestHealthStatusSinceAndLatency`.
4. **M4 webhook → PapDashboard** (F18–F20): `HealthRelay` in
   internal/bridge/papdashboard — a loopback translator (dashboard
   webhook schema → `/api/ingest` alert.triggered/alert.resolved, shared
   `ingester` seam extracted from Bridge.post); wired via
   `webui.Config.AlertURL/AlertAPIKey` + `tq serve --alert-url` /
   `--alert-api-key` flags ($TQ_PAP_URL/$TQ_PAP_API_KEY); pinned by
   `TestHealthRelayTransitionsToAlerts`, `TestHealthRelayWarnIsWarning`.
5. **M5 error-family collapse** (F21–F23): `PermanentError` →
   errorfamily.Rejection + `executor.permanent`, `PreflightError` →
   Transient + `executor.preflight` (concrete types kept, so
   `errors.AsType[*PermanentError]` call sites unchanged — zero behavior
   change); pinned by `TestErrorFamilyClassification`.
6. **M6 codec payload door** (F24–F25): `decodePayload[T]` routed
   through `codec.JSONCodec{}` (go-codec v0.3.3 promoted to direct in
   internal/executor); payload contract tests (unknown-keys / greater-V
   refusal) green in the full module run.
7. **M7 templ quick-wins** (F26–F29): `layout.MetaRefresh(3600, "")`
   replaces hand-rolled `refreshMeta`; `StickyHeader: true` on both
   ledger tables; `templ generate` + `nix run .#webui-css` ran
   (generated files updated); ADOPTION.md gains the
   `navigation.Pagination` row and the adoption guard regex widens to
   cover the package.
8. **M8 retry ergonomics** (F30–F31): `execWithTransientRetry` →
   `retry.DoWithValue` (buffers hoisted out of the attempt closures —
   see §d2 for the regression this fixed);
   `persistOutcome` gains the `OnExhausted` ERROR label naming the
   lease-reclaim backstop; pinned by `TestPersistOutcomeExhaustionIsLabeled`.
9. **M9 sse trio** (F32–F34): `live-sse` health check wired through
   `Broadcaster.Health()` (subscriber census; fail when hub closed);
   `TestReconnectLagLogged` migrated onto `ssetest.MustReadEvents` +
   `FilterByType` structural assertions; sse v0.7.0 residue swept across
   10 nested go.mods (rode the M2 wave).
10. **M10 server_timing trial** (F35–F38): mounted behind
    `ServerTimingMiddlewareWhen(RequestLog)`; `stats` + `facts` segments
    instrumented (recorded before first write); pinned by
    `TestServerTimingHeaderBehindRequestLog` (present under RequestLog,
    absent without); trial verdict KEEP appended to the report.
11. **M11 owner rulings** (F39): recorded in plan §7 — in-session
    execution (no paid minting), wave NOW, budget +400.
12. **M12 observability** (F40–F41): decision = trend+export ON
    (`WithTrend(72)`, routes + matrix rows), `/metrics` DEFERRED (no
    scraper exists; memo in plan §7).
13. **M16 AGENTS budget** (F47–F48): 26,800 → 27,200 (both twins:
    `TestAgentsDocSizeGuard` + `scripts/check-agents-size.sh`), full
    fleet-verdict line restored (27,028 B — green).
14. **M17 research README supersede markers** (F49): both deep-dive rows
    (cqrs 2026-10-05 AND templ 2026-09-14) struck with pointers to their
    fleet successors.
15. **M18 dead-SHA heal** (F50): patch-id analysis proved both dead
    cites content-identical to reachable commits — fork records added:
    `64b22f25` → `b9d09371`, `573acfee` → `ae1e13b8`.

## b) PARTIALLY DONE

1. **M13 re-score** (F42): both reports' §2/§4 rows and score math read
   and re-derived (datastar 17 → ~83: two MISSED rows now FULL, one
   PARTIAL now FULL; cqrs-lite 82 → 83: version-currency rows current);
   the delta sections + index.html row updates are NOT yet written.
2. Battery receipts are MID-SESSION: module loop, root race, and facade
   parity all passed, but BEFORE M4–M10's code landed; vendorHash was
   verified at the cqrs-wave point but codec + server_timing deps landed
   after. Everything needs the M15 re-run over the final tree.

## c) NOT STARTED (not fucked up — ordered by the plan's graph)

1. **M15 battery closure** (F44–F46) — now the FIRST next step: full
   `ci-local.sh` (tee log, `grep -c '^ok:'` receipt), vendorHash +
   `test-cmd-tq.sh` + root race re-runs over the final tree,
   `check-script-syntax.sh`, 3 random report verify-block re-runs.
2. **M13 completion** (F42): write the two delta sections + update
   index.html rows/scores.
3. **M20 harvest** (F54–F55): TODO_LIST.md completion notes for the
   executed subset (machine shape, dedup vs rows 155/380/1067
   neighborhoods), then `check-docs-gates.sh` +
   `check-todo-list.sh` green.
4. **M14 render-gate closure** (F43): vision pass on index.png or a
   scripted pixel-diff harness.
5. **M19 digest ritual** (F51–F53): docs/status/README at 422+ live rows
   > 400 threshold — ANNOTATE+ARCHIVE sweep + digest row.

## d) TOTALLY FUCKED UP (fixed in-session, recorded for the record)

1. **rc-capture trap**: `nix build … | tail; echo rc=$?` captured TAIL's
   rc, not nix's — the documented no-PIPESTATUS class. Caught by
   curiosity (vendorHash green after a dep change seemed impossible),
   re-ran with clean redirect capture. The first F5 receipt was
   meaningless; the second was genuinely green.
2. **DoWithValue nil-pointer regression**: the old hand-rolled
   `execWithTransientRetry` preserved the LAST attempt's value on error;
   `DoWithValue` returns the zero value — `buf.String()` panicked in
   `TestAgentExecutorContextCancelKillsAgent`. Fixed by hoisting the
   output buffers OUT of the attempt closures (work turn AND closeout —
   the second caught proactively before it bit). Lesson: when a review
   says "hand-rolled X re-implements library Y", the hand-roll may
   encode EXTRA semantics — diff them before deleting.
3. **defer-in-constructor bug (self-caught)**: first relay draft had
   `defer relay.Close()` inside `New()` — would close the listener the
   moment New returned. Rewrote to Server-owned lifecycle (Run closes).
4. Mechanical misses, all caught by tests/hooks the same minute:
   heredoc→`*.go` blocked twice (documented rule — used write/edit
   tools); a test function inserted before the import block; missing
   imports after test additions (sse, testing, http); `FindByType`
   returns the FIRST matching event (stats fragment, not table);
   `ssetest.Event` is a struct alias, not an interface (nil-compare
   invalid); one `MeasureServerTiming` call initially measured a
   zero-length span instead of wrapping the store read.

## e) BLOCKERS, HELP, THREATS

1. **PRE-EXISTING RED**: `TestDoctorHealthyEmptyDB` (cmd/tq) fails at
   HEAD — `tag-ancestry` warns "1 release tag unreachable (forked
   lineage): v0.4.2". `git merge-base --is-ancestor v0.4.2 HEAD` = NO.
   Documented in 2eff5d56 ("forked-lineage residue"); NOT caused by this
   session; needs an owner ruling (§g1).
2. **NEW dead SHAs from concurrent fleet**: three reports minted at
   07:25–07:33 today (not this session's) cite `9519eb06` — commit
   exists but unreachable from HEAD. Their owning sessions/daemon should
   heal them; healing others' reports while their sessions may still be
   writing felt wrong (§g3).
3. **Daemon co-tenancy**: the auto-commit daemon swept the session's
   work repeatedly (d315904f + others). M15's battery must start from a
   QUIET tree — re-read `git log -1 --format='%(trailers:key=Task-Queue-ID,valueonly)'`
   discipline applies.
4. **TQ_DB hook warnings** fired on most shell calls (inherited
   production journal path) — no `tq` command was run against it; only
   go/git/docs touched.

## f) Up to 50 things we should get done next

<!-- fresh:skip: M13-M20 completion items are the ratified plan's own next steps (docs/planning/2026-11-09_03-30 §7); minting them as paid tasks was ruled against (plan §7 ruling 1) -->

1. M15: full ci-local.sh over the final tree; tee log; `grep -c '^ok:'`
   receipt (rc to a FILE, never a pipeline tail).
2. M15: re-run `nix build .#checks.x86_64-linux.vendor-hash` (codec +
   server_timing landed after the last green receipt).
3. M15: re-run root `go test ./... -race` + `test-cmd-tq.sh` over the
   final tree (expect only the §e1 pre-existing failure).
4. M15: re-run the 20-module loop (code landed in 4 modules after the
   loop's green).
5. M15: `check-script-syntax.sh` (scripts/check-agents-size.sh was
   edited — syntax must be pinned).
6. M15: re-run 3 random report verify blocks; note deltas in Reviewed
   lines.
7. M13: append post-wave delta sections to go-cqrs-lite.md (82→83) and
   go-datastar.md (17→~83) with per-row flips cited.
8. M13: update docs/review/libs/index.html (score cells, verdict table,
   cross-cutting cards: "all adopted libs current" now true).
9. M20: TODO_LIST.md completion notes (machine shape) for M1–M12,
   M16–M18; dedup against rows 155/380/1067 neighborhoods.
10. M20: `check-docs-gates.sh` + `check-todo-list.sh` green after the
    harvest.
11. M19: digest ritual prep — candidate list of fully-resolved
    docs/status reports (check-archive-eligibility shape).
12. M19: ANNOTATE banners + archive moves + digest row (422+ live rows
    over the 400 threshold).
13. M14: vision pass on docs/review/libs/index.png (vision-capable
    session) or scripted pixel-diff harness.
14. webui.go Handler() docstring: add /health/trend + /health/export to
    the mounted-routes list (the bindings exist; the doc lags).
15. examples/embed: verify no datastar v0.6.x pin survived (root+cmd/tq
    bumped; embed rode the cqrs/sse loops but not F11's datastar get).
16. cmd/tq `go mod tidy -diff` check post-wave (datastar + static pins).
17. SECURITY.md Cache-Control row: mention trend/export carry
    `no-cache` (row currently names only health HTML/SSE).
18. `tq serve` startup banner: mention the health relay when --alert-url
    is active (operator confirmation the alerts channel is armed).
19. v0.4.3 release prep: CHANGELOG Unreleased is loaded; release flow
    per docs/release/ (owner-gated).
20. Owner push decision for the wave commits (§g2).
21. Heal `9519eb06` dead cites in the three 07:25–07:33 fleet reports
    once their sessions close (§g3).
22. Consider WithPrefetch on TasksPage reads (cqrs report §4 opportunity
    2, post-wave, cheap).
23. Optional: `WithMetrics(true)` revisit if a scraper ever appears
    (deferred, plan §7).
24. ADR/README note: webui ADOPTION.md guard now covers `navigation`
    (future rows can't be invisible again).
25. Fleet convention candidate: the "hoist the buffer out of the retry
    closure" pattern (§d2) is a seam doc candidate for go-retry
    consumers (upstream docs note?).

## g) Questions I cannot figure out myself

1. **v0.4.2 forked lineage**: `TestDoctorHealthyEmptyDB` is red at HEAD
   because the v0.4.2 tag (2f0bea71) is unreachable from master — cut on
   a forked tree, documented as residue in 2eff5d56. Healing means
   either history surgery (re-tag/merge — barred without owner), or
   baselining the tag-ancestry check. Which? Releases "must not cut
   until healed" — this blocks v0.4.3.
2. **Push**: the whole wave (security-class cqrs bump + everything
   above) sits in daemon-committed local commits, unpushed. Push now, or
   hold for the v0.4.3 train? (No push without an explicit order.)
3. **Concurrent-fleet dead cites**: the 07:25–07:33 reports citing
   `9519eb06` belong to other sessions — heal them in the next
   docs-health pass, or leave strictly to their owners?

— Recorded 2026-11-09_08-54 CET; session then WAITS FOR INSTRUCTIONS.
