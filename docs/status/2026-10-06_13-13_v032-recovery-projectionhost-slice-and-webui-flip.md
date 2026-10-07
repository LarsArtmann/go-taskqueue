# Status Report — v0.3.2 recovery, projectionhost slice, and the webui flip that is not done

**Session:** 2026-10-06 ~00:30–13:15 CEST (resumed from the 21:53 handover; owner
re-issued the blanket execution directive over the Pareto master plan)
**Owner questions answered at the end (§g) — WAIT HERE after reading.**

---

## a) FULLY DONE

### Release track (M05, M10)

1. **readmodel lint blockers cleared.** Split the two gocyclo-heavy parity
   tests (`TestStatsParityLifecycle` 22→16 via the extracted
   `assertStatsFiltersAndAPIs`; `TestDurableCursorSkipsReplay` → the
   fresh-projection escape hatch moved into its own
   `TestDurableCursorFreshProjectionReplays` with its own fixture).
   golangci-lint readmodel: only the baselined gocognit remains.
2. **Root contextcheck fixed at the source.** `readmodel.Open` minted
   `context.Background()` for the durable-cursor load. Signature is now
   `Open(ctx, path, src, opts...)`; all 3 production callers
   (cmd/tq stats, httpapi `UseReadModel(ctx, …)`, webui tailer) and 8 test
   callers threaded; root contextcheck back to its baseline 14.
3. **cmd/tq advisory drift fixed.** The new `storeStats` pushed cyclop
   13>12, varnamelen, and 2×wsl_v5 over the line; a shared `narrowCounts`
   narrowing helper replaced the inline maps. Baseline gate now
   **OK (1261 findings vs baseline 1286 — shrunk)**. M05's blocker is gone.
4. **Master CI reds diagnosed.** The 19:43 failure was three independent
   jobs: Ghost-reference check (fixed by a concurrent agent between runs),
   nix (see the CSS item), govulncheck (`No vulnerabilities found` at HEAD
   via `nix run nixpkgs#govulncheck` — the CI 1h29m run was an infra stall).
   The nix job's real failure was the **Web UI CSS canonical check**:
   `go list -m` returns an empty Dir for a module whose source was never
   extracted, so `scripts/build-webui-css.sh` died on every cold CI module
   cache. Fixed by warming the cache (`go mod download`) before resolving
   the two out-of-repo `@source` dirs; cold-cache determinism proven
   locally (regenerated CSS byte-identical). Committed the regenerated
   canonical `app.css` (the committed asset had drifted).
5. **Harvest `TestSelfManagingLoop` CI flake killed at BOTH layers.**
   (a) Test-side: a 20ms-poll pool running during the harvest tick completes
   the task mid-scan; the pool now starts after tick 1's assertion.
   (b) Production telemetry bug (the real one): `admitItem`/batch admission
   classified its **own** mid-scan-claimed mint as
   `"tracked: running (enqueued concurrently)"` and reported
   `Enqueued: []`. Admission implies the scan cleared the dedup key
   (`itemDenial` denies known keys first), so a claimed dedup return IS this
   tick's mint — it is now always reported, with a `slog.Warn` for the
   theoretically-exterior case. 25× `-race` + full-package green locally;
   the corresponding CI run passed after rerun.
6. **Master CI is GREEN again** (ee00cbb7, run rerun success; the worker
   `TestEnvRequeueStreakBurnsAttempt` red was the documented load-flake
   class — 6/6 green locally, `gh run rerun --failed` per the O15/O17
   posture).

### v0.3.2 release (the recovery)

7. **Probe caught the poisoned tag wave before v0.3.2 repeated it.** The
   v0.3.1 require sweep's regex capped at two path segments, so every
   three-segment internal require (`internal/queue/sqlite`,
   `internal/queue/postgres`, `internal/journal/cqrs`,
   `internal/queue/companion`, and the v4 drivers) stayed at v0.3.0 while
   consumers' sources had moved — the proxy graph cannot close. The
   scratch-module probe (root + cmd/tq replaced locally, internals through
   the proxy) went RED on exactly this; in-repo gates had all passed
   because they build over local replaces.
8. **v0.3.1 retracted, v0.3.2 shipped.** `retract v0.3.1` in the root and
   CLI modules; every repo require swept to v0.3.2 (107 requires across 21
   go.mods, unbounded-segment regex this time, verified 0 stragglers);
   CHANGELOG `[0.3.2]` section; flake.nix version 0.3.2; 22 annotated tags
   (bare + 21 sub-modules) cut at the fixed tree and pushed; proxy serves
   v0.3.2 for all probed modules.
9. **Cleanroom proof, the real one.** Downloaded the released
   `cmd/tq@v0.3.2` source from the module proxy into a writable copy and
   compiled it against the released v0.3.2 graph — binary builds and
   `tq version` runs. (`go install` is blocked in agent shells; the
   scratch-module + module-cache build is the equivalent proof.)
10. **GitHub Release v0.3.2 published** (pre-release per v0.x policy) from
    the CHANGELOG section. The script's `--push` resume path is
    structurally unreachable after any post-tag commit (it requires the
    bare tag to point at HEAD), so the remaining steps were executed
    manually with explicit verification — documented as procedure.
11. **`docs/release/RELEASE.md` release log** written: the three-segment
    sweep class, probe-before-pre-cut, and the pre-cut/resume-lock
    ordering lesson; **AGENTS.md STATUS → v0.3.2 shipped**.

### M08 — projectionhost adoption, slice 1 (F035, F036, F037, F040 core)

12. **`internal/readmodel/host.go`** (~400 lines, committed):
    - `SeqToEventID`/`EventIDToSeq` — the synthetic seq-derived ULID codec,
      mirrored from the proprietary cqrs adapter with a **byte-parity
      test** against the real `FactJournal` (drift fails a gate instead of
      corrupting cursors);
    - `WatermarkCheckpoints` — the platform `event.CheckpointStore` over
      the queue's watermarks table; the projection's Name IS the consumer,
      so the host pump and `WithDurableCursor` share one checkpoint slot;
    - `FoldProjection` — `Handle` = decode the cqrs-stamped payload →
      `journal.Fact` → the same `Model.apply` the hand pump uses;
    - malformed repri/enqueue detail now fails as **Corruption-family**
      (`errorfamily.WrapCorruption`) so the platform **dead-letters poison
      facts instead of burning the restart budget**;
    - `tailSubscriber` — the live phase: the projectionhost is drain-only
      without a subscriber; the tail polls `ReadFrom` at `DefaultPoll`
      anchored at the consumer watermark (the webui/live contract);
    - `NewProjectionHost` factory with batch/restart/DLQ options.
13. **Test battery (all green, `-race`)**: codec parity, checkpoint
    roundtrip (empty → zero checkpoint; foreign IDs refused),
    **fold parity pump-vs-host over a real seeded lifecycle**,
    **advance-past-poison** (corruption poison → DLQ entry + checkpoint
    advances, queue untouched), malformed-detail classification.

### Hygiene

14. **30 daemon sweeps grandfathered** across two passes (21 + 9), every
    one content-verified before baselining; attribution gate green.
15. **readmodel module deps** added through the proxy (event/v4, id/v4,
    projection/v4, projectionhost/v4, go-error-family — all pure-Go);
    root vendor + `vendor-hash` check green after every step.

## b) PARTIALLY DONE

1. **M08 slice 2 / F038 — the webui serve pump flip.** `runReadModel` now
   opens the model, builds `NewProjectionHost`, and starts it instead of
   the hand `m.Run` goroutine; webui unit tests (10s) pass, and a new
   `TestProjectionHostTailsLiveFacts` pins the live phase. **BUT the
   end-to-end `scripts/smoke/webui.sh` still FAILS**: the fold converges
   correctly in manual repro (projection db reached completed=2/dead=1
   twice) yet the smoke's assert window catches a stale snapshot
   (running=1 residual) — a timing/lag gap that is NOT understood yet.
   The flip is on master (daemon-swept) and **must not ship**: the smoke
   is a release gate. Fix forward or revert — owner input in §g.
2. **F040 lag surface**: `host.LagDuration()/Status()` exist and are
   exported; nothing consumes them yet (doctor/health is M09).
3. **Master push**: local HEAD (M08 slice + grandfathering) is unpushed —
   O1 requires ci-local rc=0, which requires the smoke question settled.

## c) NOT STARTED

- M09 (doctor projection section, HealthCheckDetailed → /health) — design
  only.
- M11 (redispatch burn killers, O4), M12 (daemon attribution tooling, O5),
  M13 (S2 one-vocabulary flip, O12-committed), M14–M25 (the 20% tier),
  M26 (upstream filings), M27 (docs-health tail burn, 297→0).
- Composition-root host factory — deliberately deferred: with only the
  webui consuming today it would be an unused seam (ghost-code bar).
- `--dep-sweep` retirement row, Dependabot first-week check (O15, earliest
  2026-10-13).

## d) TOTALLY FUCKED UP

1. **The v0.3.1 tag wave shipped a proxy-poisoned release.** Root cause:
   my sweep regex capped at two path segments. The proof that would have
   caught it (the proxy-resolved probe) was run only AFTER the tags were
   live. Cost: a retracted version, a forced v0.3.2, 22 extra tags, and
   recovery choreography. The in-repo gates are structurally blind to this
   class — that gap is now written down, but it should have been written
   down BEFORE the wave.
2. **The webui pump flip was declared done before its end-to-end proof.**
   Unit tests + one manual repro passed; the actual gate (the smoke) fails
   and the failure mode is not yet understood (the fold converges in
   manual repro, the smoke's window catches it stale). I then let the
   daemon commit the flip to master instead of holding it on a branch.
   Resolving it (fix or revert) is the session's open blocker.
3. **Tag-wave ordering, twice.** The prior session pushed five internal
   tags that predated M02/M03 content; today I pre-cut and pushed v0.3.1
   tags before any probe. Immutability turned both into forced forward
   fixes. Sub-tags pre-cut early is fine — content-affecting pushes are
   not.

## e) WHAT WE SHOULD IMPROVE

1. **Probe-before-pre-cut as a REQUIRED gate**: a script that builds root +
   cmd/tq with only those two replaced and everything else through the
   proxy; wire it into release.sh gates ahead of any tag existence check.
2. **A real version-sweep script** with a tested grammar for go-taskqueue
   module paths (unbounded segments) — ad-hoc regexes caused this class
   twice today.
3. **Release ordering doctrine**: sub-tags may pre-cut early; the BARE tag
   is cut only when the tree is final and gates are about to run; `--push`
   resume depends on tag==HEAD (now in RELEASE.md — keep it enforced).
4. **webui smoke patience**: its assert window is too tight for a 500ms-poll
   tail on loaded machines; either poll the stats endpoint with a longer
   deadline or pin the tail interval the smoke assumes.
5. **The worker `TestEnvRequeueStreakBurnsAttempt` 5s timing test** is
   CI-load-flaky (2 reds today) — M14's load-aware skips should cover it;
   until then it will keep flaking the module-isolation job.
6. **A store-side created-vs-dedup signal on Enqueue** would make harvest's
   freshness exact instead of inferred from the scan; API change, worth an
   ADR line when the queue facade next moves.
7. **The seq codec duplication** (readmodel mirror + cqrs original, pinned
   by a parity test) — the clean end-state is one codec in
   `internal/journal` that both import (O10-constrained; needs a ruling
   that a mechanical extraction is not "extending the proprietary seam").
8. **govulncheck pinned in the flake** (`nixpkgs#govulncheck`) — agent
   shells cannot `go install`, so local CI-parity checks need the nix path
   (used it today, worked).
9. **Hold risky flips on a branch until their end-to-end gate passes** —
   the daemon commits everything, so "uncommitted WIP" is not protection;
   a branch is.

## f) NEXT (prioritized, ~50)

1. **Fix the webui smoke under the host flip** — pin the residual
   running=1 (add a lag metric print to the smoke; check whether the
   watermark anchor + dedup ring interact badly when the drain races the
   tail).
2. Failing 1 within a bounded effort: **revert the webui flip**, keep the
   host for one-shot/parity paths, re-file the serve adoption.
3. Push master + confirm green CI (O1) once 1/2 lands.
4. Composition-root factory `NewProjectionHost` wiring (S4 completion) —
   with a real consumer this time.
5. M09/F041: `tq doctor` projection section (engine stats, cursor lag, DLQ
   size, worker states).
6. M09/F042–F043: `HealthCheckDetailed` → token-gated `/health` rollup +
   tests.
7. Wire `projectionhost.NewSQLiteDeadLetterStore` into the composition
   wiring (the DLQ today is opt-in/memory in tests) + `tq doctor --dlq`.
8. M11/F046: mint-time done-check (O4).
9. M11/F047: `--force-redispatch` escape + help.
10. M11/F048: gate-slow re-dispatch guard → verify-only closeout.
11. M11/F049: harvest anti-race re-read at claim/dispatch (production half
    of today's fix).
12. M11/F050: mechanized first-batch recipe wrapper.
13. M12/F052: `scripts/fold-marker.sh`.
14. M12/F053: `heal-daemon-sweep.sh` task-less attribution mode.
15. M12/F054: daemon footer-attach heal script.
16. M12/F055: `install-pre-commit.sh` honors core.hooksPath.
17. M12/F056: inert-hooks detection + session-start probe.
18. M13/F058: journal `Fact` alias to `facts.Fact` (O12).
19. M13/F059–F060: `Detail` jsontext→[]byte, 79 sites in two slices.
20. M13/F061: companion scanFacts direct.
21. M13/F062: memory journal + journal/cqrs re-point (ADR-0014).
22. M13/F063–F064: facade parity + full gates + commit.
23. M14/F065: `.gates/` repo convention + AGENTS line.
24. M14/F066: `nix flake check` into ci-local.
25. M14/F067: load-aware skips (multi-repo, exactly-once) — also masks the
    env-streak CI flake.
26. M14/F068: devmod per-invocation dev.mod suffix.
27. M14/F069: per-user golangci cache dir (ends /mnt/buildcache races).
28. M15/F070–F071: quiet-host ci-local capture + evidence archive.
29. M16/F072–F074: token/cost-denominated cap (O2) + UsageToday settlement +
    cap-unit ADR.
30. M17/F075–F077: JIT frontier scoring (O3), retire DefaultScoreTTL.
31. M18/F078–F082: DefaultBatchItems=3 + prompt outcome-contract rewrites.
32. M19/F083: claim-time executor-type filter.
33. M19/F084: `tq park` verb (no attempt burn).
34. M19/F085–F086: closeoutPending leak audit + per-key map bounds.
35. M20/F087–F091: security pin bundle (header matrix, auth bodies, SSE e2e,
    lockout fake-clock).
36. M21/F092–F097: audit drill-downs (--requeues/--task-id/--dedup,
    evidence-stage, rescues line, parked-consistency fixture,
    hit-count→locations).
37. M22/F098–F101: doctor consumer-liveness, derivation-blind commit
    census, .crushrc shadow WARN.
38. M23/F102+: ci.yml parity + required-checks P1 + toolchain pins.
39. M24: webui customer surfaces (budget meter reachability, dlqfix/LogPath
    result surfaces, fragment collapse).
40. M25: SQLITE_BUSY class fix (repro harness, bounded open retry).
41. M26: upstream filings — go-nix-helpers mkDefault footgun,
    art-dupl templ suppression, M4 ratification memo, cqrs-lint branch
    (A014/D013/V006 per O10).
42. M27: docs-health mechanization + standing tail burn (297→0).
43. VERSION-SURFACES.md re-pin + release.sh bare-tag-at-gate-time redesign.
44. Pin govulncheck in the flake (agent-runnable).
45. Add a fold-lag assertion to the webui smoke (diagnosability for 1).
46. Sweep remaining fold error paths for error-family classification
    (heartbeat/cancel detail parsers).
47. Hand the dogfood serve restart decision to the owner (O12, operator-run).
48. TODO_LIST reconciliation pass: rows closed by today (CSS canonical, CI
    red, harvest race, tag wave) — docs-health VERIFY.
49. AGENTS.md platform-section rewrite for the projectionhost adoption
    (size-guarded).
50. FEATURES.md row for the managed-projection pump (after 1/2 resolves).

## g) OWNER QUESTIONS (3)

1. **The webui serve pump flip** (the smoke-red flip on master): fix
   forward — hunt the fold-lag now (my next move: instrument the tail's
   delivered-applied sequence against the smoke timeline) — or revert the
   flip, keep the projectionhost for one-shot folds + parity, and re-land
   the serve adoption deliberately with M09 (doctor) and the sqlite DLQ?
   I recommend the revert-and-re-land; say the word and it is done in
   minutes either way.
2. **v0.3.1's public record**: the version is proxy-live but retracted and
   has no GitHub Release page. Publish a one-paragraph Release on the
   v0.3.1 tag warning consumers ("broken module graph, use v0.3.2"),
   or leave it silent (zero known consumers, retraction visible via
   `go list -m -retracted`)?
3. **Dogfood cutover timing**: the production serve (805 tasks, nix 0.3.1
   binary) still folds with the pre-durable-cursor pump (WAL churn). The
   next rebuild would pick up the projectionhost tail — but not while the
   smoke question is open. Restart the service NOW on the current 0.3.2
   build (kills the WAL churn tonight; operator-run per O12), or hold
   until the flip is settled and rebuilt?

---

_Report ends. Waiting for instructions._
