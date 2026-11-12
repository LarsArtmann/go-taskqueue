# Status Report — Continuation Window Self-Audit (T10–T24 executed; what was forgotten, what sucked, what's next)

**Date:** 2026-11-12 12:05 CET
**Session scope:** Brutal self-audit of the 2026-11-12 continuation window
(07:05–07:56) that executed pareto-plan tasks T10–T24
(`docs/planning/2026-11-12_04-29_repair-verify-deepen-pareto-plan.md`),
per the owner's what-did-you-forget prompt. No new research; everything
here is from that window's run. Predecessor reports:
`2026-11-12_06-45_repair-verify-deepen-window-report.md` (morning window)
and `2026-11-12_07-55_repair-verify-deepen-continuation-report.md` (the
work closeout this report audits).

**State at writing:** tree clean; local master == origin/master tip
(`9325f509`, 09:11 — the owner pushed the 52-commit line during the
gap; the memo's push question is OBSOLETE). All window gates were rc=0
at the closeout commit.

## a) FULLY DONE

1. **T08 pin tests** — poisoned-UTF-8 coercion + marshal-fallback body
   pinned (58dd6a58); suite 12+2 green.
2. **T12 ssetest swap** — 4 hand-rolled decode loops + 3 substring scans
   → one `fragmentEvents` helper; go-sse report row PARTIAL→FULL
   (dd4dda5c).
3. **T16 per-check abandonment** — `boundedCheck` on all five blocking
   probes, fail-closed at the batch deadline, panic contract preserved
   (re-panic on caller stack, post-abandonment panics fold to errors);
   pin test + race clean (7eeb4964/ec166052).
4. **T17 health.Off sentinel** — unmounted read model emits the visible
   off-row with the `--read-model` recipe; both sibling omit-assertions
   flipped (85e5c4c8).
5. **T19 metrics flag** — `Config.Metrics` → `WithMetrics(true)` +
   `/health/metrics` binding inside the token gate; 404/401/200 pinned;
   default off (owner no-op path documented).
6. **T18 WrapOnce ruled out** — domain-type break proven (AsType miss →
   transient degradation); report rows MISSED→REJECTED BY DESIGN.
7. **T21 both could-not-verify closures** — go-sse coverage re-measured
   (100.0% statements, root pkg, fresh receipt); favicon Set-site
   located in production code (go-health-dashboard favicon.go:23).
8. **T22 decision memo** — 4 owner questions + libs cadence proposal
   (`docs/planning/2026-11-12_07-48_decision-memo-owner-questions.md`).
9. **T24 harvest** — 3 agent-executable capability rows into TODO_LIST
   (shutdown awareness, checks.Disk, WithIntrospection).
10. **T14 digest sweep (archive half)** — 28 window reports bannered +
    archived with manifest rows, 12 legacy root closeouts routed to
    tasks/ (O7), `digest 2026-11` row landed (e09b3404); index +
    ghost-archives gates green; 425-row bloat warning now silent.
11. **T11 audit** — v1.28 not cut (per-family ls-remote receipt); pins
    all at family max; templ report re-checked line added.
12. **T23 watchlist (partial scope, see b8)** — cqrs/health/templ/
    cmdguard/credentials-store/appkit: zero drift; receipt archived.
13. **Reports re-scored + de-drifted** — dashboard 79→87, health 41→59,
    errfam 21→29; index.html "Top opportunity" cells fixed (templ row
    carried stale go-health text); CHANGELOG O14 entries landed.
14. **T10 disposition** — attempted, caught by the pinned round-trip
    (6/6 deterministic loop), reverted cleanly (d52c8b50), trace
    documented, upstream-gated re-arm row filed. The revert-decision
    itself was correct and fast once diagnosed.

## b) PARTIALLY DONE

1. **T10 capability value NOT delivered** — O(1) repeat-window reads do
   not exist; the ledger is blocked on the upstream tie-lossy cursor
   (metaengine v4.23.0). What shipped is the honest block + re-arm row.
2. **T11 bump + NoScript adoption NOT delivered** — blocked on the
   upstream tag (#63); audit-only.
3. **T14 annotate half is THIN** — the plan's M44/M45 specified per-item
   verdict banners; I shipped ONE bulk banner per report asserting
   "open items live in TODO_LIST" WITHOUT verifying that per report.
   True for the harvest-convention era, unverified for September-era
   reports.
4. **T19 cmd/tq flag surface not wired** — the config seam exists; no
   operator can actually turn it on from the CLI yet.
5. **T22 recommendations not executed** — by design (owner-gated), but
   the memo is advice until you rule.
6. **T23 scope narrower than the fleet** — 6 of ~10+ lib families
   re-checked; go-sse/go-health-dashboard/go-etag/go-output/go-retry/
   go-idempotency/go-datastar tags were NOT re-checked this round
   (the 11-11 refresh covered them 24h earlier — mitigation, not
   completion).
7. **index.png NOT re-rendered** despite score moves (87/59/29) — the
   T04 precedent says re-render on moves; I updated index.html only and
   did not even record a skip receipt. Forgotten deliverable.
8. **FEATURES.md NOT updated** for the new operator surfaces (metrics
   flag, off-row, abandonment, durations) — inventory drift by omission.

## c) NOT STARTED

1. Upstream issue FILED for the tie-lossy cursor (repro + trace are
   receipted; filing not taken — the two-tick convention leaves the
   file-upstream tick open).
2. Upstream ask to EXPORT `runBoundedCheck` (tq hand-rolls it because
   upstream keeps it unexported — same conversation as 1).
3. Local-checkout experiment: mount go-cqrs-lite `main` via replace to
   test whether the tie loop is already fixed on main (would sharpen
   the upstream report).
4. The 3 harvested capability rows (shutdown awareness, checks.Disk,
   introspection).
5. Doctor-test exemption patch (draft not written; 15-min once acked).
6. `--metrics` CLI flag wiring.
7. Full root `go test ./... -race` battery on the final tree (my final
   battery covered touched modules only; root build+vet ran, root suite
   did not).
8. `check-release-docs.sh` run after the CHANGELOG edits (O14 gate —
   not run; the entries were written to the policy by hand).
9. lint-baseline delta check on the new code (advisory deltas
   unmeasured); gosec triage on the new test stores unrun.
10. index.png re-render (as b7).
11. FEATURES.md sync (as b8).
12. Digest-sweep script promotion: it lives in /tmp only — not committed
    as a `scripts/` tool with `--self-test` + negative cases.
13. DOMAIN_LANGUAGE.md terms for the new surfaces (off-row, abandonment,
    page cache) — untouched.
14. index.html "moves" table: no 79→87/41→59/21→29 rows added (the
    historical moves table stops at the 11-09 moves).

## d) TOTALLY FUCKED UP (own the failures)

1. **Committed bfaa229f with a FAILING suite.** The T17 off-row commit
   chained `go test … | tail -1` into `&&` — the pipe masked the test
   rc, `tail` succeeded, the commit sailed through red. The AGENTS rule
   "rc TO A FILE (no PIPESTATUS)" exists for exactly this and I
   violated it. Fixed forward in 85e5c4c8 within minutes, but the red
   commit is on the (now pushed) master line. Worst thing I did.
2. **The T10 revert misdiagnosis loop.** The daemon had swept my WIP
   edits (c11b497e) before my revert; `git checkout --` then restored
   the BROKEN content, not the clean tree. I read the 6/6 failures as
   a tie-boundary flake and burned two more test runs before checking
   `git log -1 -- <file>` — the check the Known-Issues section
   mandates BEFORE trusting tree state. The final `git revert` was
   right; getting there wasted ~15 minutes and produced one wrong
   hypothesis (tie-flake) that never got written down anywhere except
   my head.
3. **Sloppy prose shipped in a committed report** — go-health.md's
   Inputs line briefly read "PARTIAL=5 × 0.5 (… → 6? no: … recount:
   …)" — my scratch arithmetic leaked into a commit. Cleaned next
   edit; it still rode one commit on a public line.
4. **Lost facts in an edit** — replaced the errfam 2026-11-11 refresh
   line wholesale (losing the call-site inventory 8→14 and checkout
   facts) before restoring it as a stack. The memory rule says confirm
   every distinct concept is carried before replacing; I caught it on
   re-read, but the violation happened.
5. **Score arithmetic done by hand, twice wrong** (FULL/PARTIAL/MISSED
   recounts in two reports). A 10-line counting script would have made
   both exact on the first pass.
6. **One edit misfire** dropped `projStart := time.Now()` from health.go
   (replaced-then-restored). Exact-match discipline lapse; harmless in
   effect, sloppy in method.

## e) WHAT WE SHOULD IMPROVE

1. **Gate rc discipline is non-negotiable**: every gate run writes rc to
   a file; `cmd | tail` chains never gate a commit. This one rule would
   have prevented d1 entirely.
2. **Read the wiring BEFORE drafting** (T17/T19 pattern): three
   test-caught drafts this window (wrong branch, routes-by-value,
   bindings bypass). Each was caught fast, but read-first turns three
   red runs into zero.
3. **Daemon-state check before ANY tree-changing git op**: `git log -1
   -- <file>` before checkout/revert/restore, every time, no exceptions
   — d2 was a 15-minute tax on a 5-second check.
4. **Bulk sweeps must cite per-item evidence or soften the claim** —
   the T14 banner asserts TODO_LIST coverage I didn't verify; either
   sample-audit or phrase the banner as "assumed per harvest
   convention".
5. **Score inputs get a counting script**, not mental math; ship the
   script with the report so the arithmetic is re-derivable.
6. **Forgot-the-follow-through pattern**: three deliverables (png
   re-render, FEATURES.md sync, release-docs gate) were implied by the
   work but not done or even logged as skipped. Closeout checklists
   should include "what does this change make STALE?" — png, FEATURES,
   O14 gate were all stale and I touched none.
7. **Test-only mutability of prod timing** (`healthEvalTimeout` package
   var) — cleaner as an unexported field on the prober; small
   refactor, better parallel-test hygiene.
8. **Report skeleton worked well** — a)–g) discipline, receipts, and
   forward-fixed honesty kept every mistake recoverable. Keep.

## f) UP TO 50 THINGS TO GET DONE NEXT

*Owner-ritual items marked (OWNER); everything else agent-executable.*

1. (OWNER) Doctor-test exemption: acknowledge option B (exclude
   tag-ancestry from TestDoctorHealthyEmptyDB's worst-status walk) —
   then it's a 15-min patch.
2. File the upstream cqrs issue: tie-lossy cursor under PrefetchCache
   (repro + trace receipted; needs your github-voice pass).
3. Same upstream conversation: ask to export `runBoundedCheck`.
4. Local-checkout experiment: go-cqrs-lite main via replace — is the
   tie loop already fixed on main? (evidence for 2).
5. Re-render index.png at the new scores (87/59/29) + visual check.
6. Full root `go test ./... -race` battery on the final tree + receipt.
7. Run `check-release-docs.sh` against the new CHANGELOG entries.
8. FEATURES.md rows: metrics flag, off-row, abandonment, durations.
9. lint-baseline `--check --clean-cache` delta over the window's new
   code; triage any new advisories.
10. `check-gosec.sh` over the new test stores (utf8PoisonStore,
    wedgedWorkersStore, recordingWriter).
11. Wire `--metrics` into cmd/tq serve → webui Config.Metrics.
12. Shutdown-awareness adoption (harvested row; upstream
    lifecycle.go:170-212 shape).
13. checks.Disk on the sqlite home (harvested row; ENOSPC class).
14. WithIntrospection flag following the T19 pattern (harvested row).
15. Promote the digest-sweep script from /tmp to `scripts/` with
    `--self-test` + negative-path cases (fleet convention: countable
    gate summaries are PINNED).
16. Complete the T23 watchlist for the unchecked families (go-sse,
    go-health-dashboard, go-etag, go-output, go-retry,
    go-idempotency, go-datastar, cqrs-htmx, loginpage).
17. T14 annotate-depth pass: sample-audit 5 archived September reports
    — are their open items actually in TODO_LIST? Correct banners
    where not.
18. Doctor-test patch DRAFT now (so ack→land is one step, no re-derive).
19. index.html moves-table: add the 79→87 / 41→59 / 21→29 rows.
20. T10 re-arm row upgrade: attach the repro script + local-main
    experiment result so the future attempt starts from evidence.
21. BoundedCheck timeout → unexported prober field (kill the package
    var; re-enable t.Parallel on the abandonment test).
22. noctx advisory sweep in webui_test (getBody/http.Get class,
    ~30+ sites) → RequestWithContext.
23. paralleltest advisory disposition: document why SSE tests skip
    t.Parallel or fix them (advisory-only today).
24. Score-counting script (`scripts/score-libs.sh` style) so report
    Inputs lines are re-derivable.
25. FEATURES.md inventory sync (same as 8 if not yet taken).
26. CHANGELOG `### Internal` rows for the window's repo-internal work
    (ssetest swap, digest sweep, gate self-tests) per O14.
27. DOMAIN_LANGUAGE.md: off-row, abandonment, page-cache terms.
28. 2026-10 cluster digest prep: pre-annotate candidates for next
    month's sweep (the ritual recurs ~12-13).
29. Spot-check one routed-to-tasks closeout for the O7 queue-
    derivability claim.
30. Retry-nightly: `scripts/fuzz/nightly.sh` last run date check.
31. Facade-parity + conform suites re-run receipt on the final tree
    (morning window green; nothing queue-level changed since — cheap
    re-stamp).
32. verify-blocks replay delta: the three re-scored reports (dashboard,
    health, errfam) changed verify blocks — re-run their How-to-verify
    blocks against the new citations.
33. AGENTS.md: does any Known Issues line need the T10 tie-lossy fact?
    (budget-guarded — prune-same-commit rule applies).
34. Upstream templ-components: watch #63 (NoScript/CopyButtonScript);
    bump ritual when v1.28 cuts (grep 00010101, vendorHash, templ
    generate, webui-css).
35. Upstream go-health: severity-override design — add tq demand note
    upstream (tq is the named motivating consumer; PROPOSED since
    11-04).
36. Adopt-or-veto the harvest default (memo §4) — one line.
37. Adopt-or-veto the libs cadence (memo §5) — one line.
38. server_timing promotion decision (memo §3) after fleet-need
    evidence or drop the trial.
39. Standing: generic_return migration (owner-blocked).
40. Standing: calibration.jsonl retention (owner-gated).
41. Standing: daemon `--no-verify` hook bypass probe (owner-gated).
42. Standing: `tq ask --task` not-found investigation.
43. Standing: heal-daemon-sweep oscillation fixture.
44. Standing: AGENTS.md budget re-tighten project (~25.3k/25.4k in the
    row's premise — verify current bytes first).
45. Rewrite-rate adaptive default (owner-gated standing).
46. Fixtures-under-`>`-redirect advisory extension (owner lean
    recorded; small hook change).
47. go-sse coverage claim: add the measurement command to the report's
    How-to-verify block so future refreshes re-derive it in one line.
48. T12 follow-through: sweep the remaining `RequireDataJSON`-shaped
    exact-compare opportunities in auth_test/board_test
    (assertions-row completeness).
49. T17 follow-through: does the doctor/API surface need the same
    off-row visibility (projectionhealth.go renders mounted-only)?
50. `index.png` + `index.html` generation: write the render recipe into
    docs/review/libs/README (headless firefox steps) — the "how" lives
    only in old session reports today.

## g) QUESTIONS FOR YOU (un-derivable from the repo)

1. **Upstream filing**: do you want the tie-lossy-cursor issue filed on
   go-cqrs-lite NOW (I have the repro + code-level trace receipted;
   I'd draft it in your voice, two-tick style), or does upstream
   filing stay strictly owner-ritual for this repo pair?
2. **The September-archive trust question**: the digest banners assert
   open items live in TODO_LIST for 28 reports I did not per-item
   verify. Do you accept that as the digest standard, or do you want
   the sample-audit (f17) to be a HARD step before any future digest
   row lands?
3. **Metrics surface timing**: wire the `--metrics` CLI flag now
   (f11 — one flag, the surface is already token-gated and pinned), or
   keep it config-seam-only until a pool actually asks for tq series?
