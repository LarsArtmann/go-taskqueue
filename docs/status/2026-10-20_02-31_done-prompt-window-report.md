# Done-Prompt Window Report — 2026-10-20 02:31 CEST

Window: 5 agent tasks completed 2026-10-20 00:31–02:03 (the docs-health
fourth-sweep tail + the dupe-review follow-up batches). Parallel in the
same wall-clock window but OUTSIDE this window's task set: the art-dupl
tail-zero batch (task …a33bbf18, closeout
`docs/status/tasks/2026-10-20_02-27_task-000001a156a33bbf182143ae24a800000000.md`)
and the concurrent floor-bump churn (root go.mod → 1.27.2).

## a) FULLY DONE (verified against git log + closeout reports + file state)

1. **Lint-gate polish bundle** (`70f62dd9`): `scripts/lint-baseline.sh`
   regen mode now always runs `golangci-lint cache clean`; new opt-in
   `--clean-cache` for `--check`; formatterWithGo/goTarball drop-day
   pairing documented in AGENTS.md Known Issues; gosec summary
   pluralization fixed with the self-test stub updated
   (`--self-test` green). Verified: both flags present in the script,
   AGENTS.md section in place at HEAD.
2. **Sessions-lamp cluster** (`d385faa1`): `card-sessions-lamp` amber CSS
   shipped (minified `app.css` carries the rule), `tq stats --since DUR`
   windowed session band on existing `CountFacts(since)` plumbing with a
   3-case print pin (`cmd/tq/stats_session_test.go`), webui smoke asserts
   the lamp + `1 open`. User-visible; CHANGELOG entry minted by THIS
   report (see e4/CHANGELOG [Unreleased]).
3. **dlqfix-autopsy prune semantics** (`c0fe4d38`): ruling pinned —
   PENDING `dlqfix:<id>` autopsies are never prune-cancellable; absent
   invariants documented at internal/harvest/prune.go and pinned by
   `TestPruneStaleNeverTouchesDlqfixAutopsies`; TODO row ticked
   (`37463ba8`).
4. **CHANGELOG coverage audit** (`2de0c2dd`): the 2026-09-14 webui
   overhaul's 22 behavioral workstreams had zero changelog rows (the
   v0.3.1 "instrument pass" entry self-scopes as "zero behavior
   changes"); one [Unreleased] Changed entry added, append-only
   respected, `check-doc-refs.sh` green.
5. **Webui priority-provenance verification** (`acc519da`): confirmed NO
   hand-mirror exists — webui `priorityProvenanceFor`
   (internal/webui/render.go:80) and cmd/tq `buildPriorityProvenance`
   both call `harvest.BuildProvenance`; only the stale "mirrors tq show"
   comment remained and was corrected. Art-dupl already collapsed the
   clone in the 2026-09-26 sweep.
6. **Accepted dupe-groups ledger + gate** (`b84528f6`, `fd95b61f`):
   docs/dupe-type-accepted-ledger.md records all 10 accepted groups with
   rationale + anchors; re-run of `branching-flow dupe` matched the
   ledger (9 actionable keys = ledger rows 1-9); new
   `scripts/dupe-baseline.sh` (regen/--check, growth-fail semantics like
   lint-baseline) wired into ci-local after the lint step, negative-tested.
7. **f13 mirror-clones scope ruling** (`88bc4480`): the three sweeper
   shells are intentionally OUTSIDE check-mirror-clones.sh (regexes match
   only cross-backend dirs); ruling recorded verbatim beside the
   per-sweeper comments in all three sweep.go files.
8. **f14 budgetUsageView golden pins** (`d1a062a8`):
   cmd/tq/budget_json_test.go pins exact wire JSON incl. the
   `session_usage` omitempty boundary.
9. **f19 upstream citation** (`85dbd9a2`): readmodel evt* family comment
   cites metaengine v4.17.0 `OnRecordTyped` dispatch
   (record_fold.go:43-46 + ontyped_test.go:64-66), verified against the
   pinned module cache.
10. **f21/f22 status lint pass** (`a786a876`): `maybeMint` (gocognit
    31>25) split into a five-helper pipeline; all three err113 sites
    replaced with static sentinels; in-module gocognit+err113 = 0.
11. **Red done-guard test FIXED** (`3125a000`):
    `TestSweepDoneGuardIsKnownNotRefusal` — root cause was the window
    recency cutoff making the done-guard unreachable on watermark-rewind
    replays (trigger filtered out before the dedup probe); fix exempts
    the trigger from the cutoff; test green, module `-race` green.
12. All five closeout reports written under docs/status/tasks/ with
    TODO_LIST rows ticked per item.

## b) PARTIALLY DONE

1. **Full `ci-local` battery not run in ANY of the five windows** — all
   claims rest on scoped per-module gates. Justified at the time (the
   concurrent floor-bump churn broke cmd/tq legs for foreign reasons),
   but the wired dupe-baseline step has never been exercised in situ
   (existing row).
2. **Done-guard reachability pinned only at Every=1**: at `Every>1` a
   rewind replay can reassemble a sub-Every window and still silently
   no-op. Pre-existing, strictly widened by the fix; rows minted
   (2026-10-20 02-03 §b1).
3. **`lint-baseline.sh --check --clean-cache` live-fire ended GROWTH**
   from a parallel agent's in-flight test churn; baseline correctly NOT
   regenerated — deliberate regen still owed (row minted 00-31 §f6).
4. **Dupe-gate tool provenance unrecorded**: baseline seeded by whatever
   branching-flow build was on PATH; a schema rename would read as
   silent shrink (row minted 01-28 §f2).
5. **CHANGELOG for the 00-31 sessions-lamp batch** was deferred to the
   next release cut by its own closeout; this report mints the entry now
   (CHANGELOG [Unreleased]) so it is no longer partial.

## c) NOT STARTED

- The webui-overhaul self-review §f siblings other than §f2 (before/after
  capture strip, livecheck assertions, capture matrix, `data-state-key`
  guard test, JS harness, detail-page livecheck) — pre-tracked rows,
  untouched by this window.
- The remaining 2026-10-05 16-02 section rows (art-dupl upstream issue —
  subsequently FILED by the parallel 02-27 window as art-dupl #9,
  config pin — landed as scripts/art-dupl.json, ledger alignment,
  status-index bloat).
- No backfill sweep for changelog gaps beyond the one audited window.

## d) TOTALLY FUCKED UP

1. **A foreign feat commit was flattened** (00-31 window §d2): a
   soft-reset across a parallel agent's CHANGELOG commit destroyed its
   commit identity (content preserved, staged for its owner). Root
   cause: edit→gate→commit ordering violated repeatedly despite the
   explicit AGENTS.md rule; the heal script refused on the
   non-contiguous span. Third strike on this class in three windows;
   the `--pick <files>` row (minted 00-31 §f7) targets exactly this.
2. **Repo-wide broken state from the floor-bump churn**: root go.mod at
   1.27.2 vs `cmd/tq/go.work` pinned 1.27.1 kills every plain `go`
   invocation in cmd/tq and breaks `scripts/test-cmd-tq.sh` +
   half of ci-local for everyone (hit live in three of the five
   windows; row 935 + 00-31 §f1 own the fix — still unresolved at this
   report's HEAD).
3. **A red test sat observable for a window** (01-47): the status
   done-guard test failed deterministically and was diagnosed-then-
   ticketed rather than fixed (conflict-prudence, defensible) — the
   next window fixed it within 20 minutes, validating the ticket route,
   but the interim state was a red module at HEAD.
4. **Near-miss, vacuous gate**: the dupe-baseline script's first draft
   had a pipe/`||` precedence bug that would have shipped an
   always-green gate; caught only by the pre-commit seed+check+negative
   triple (01-28 §d1). A silent-success gate without a pinned self-test
   remains one edit away from rot (row minted).
5. **Stale-edit race**: two edit calls refused mid-refactor on sweep.go
   (02-03 §d1); python-heredoc replace landed them. No corruption.

## e) WHAT WE SHOULD IMPROVE

1. **Commit-first discipline is still not mechanical** — three of five
   windows needed daemon-sweep heals or worse; the 00-31 window lost a
   foreign commit to it. The `--pick` mode and the pre-commit
   `git diff --cached --name-only` eyeball are the two concrete
   mitigations already rowed.
2. **Resolve the floor pairing once** — it degraded five windows'
   verification posture (no full battery anywhere) and broke cmd/tq
   gates repo-wide. Highest-leverage unblock of the window.
3. **Silent-success gates need `--self-test`** at birth: lint-baseline
   has none, dupe-baseline has none; both are one careless edit from
   always-green. House pattern exists (check-status-index.sh,
   check-transient-retry.sh).
4. **CHANGELOG-at-landing beats CHANGELOG-at-release-cut**: the webui
   coverage gap survived three sweeps precisely because its entry was
   deferred; this window closed it, then immediately created the same
   gap shape for the sessions-lamp batch (now closed by this report).
5. **Scope lint greps to the module you touched** — a root-level grep
   nearly produced a false regression claim (02-03 §d3).
6. **In-module runbooks for host friction** (GOCACHE/PATH/GOTOOLCHAIN
   workarounds) exist only as prose scattered across AGENTS.md; one
   runnable env script would stop the recurring rediscovery burn.

## f) NEXT THINGS (deduped against open TODO_LIST rows — the five

closeout reports already minted their cap-10 batches; items below are
what THIS window-level pass adds beyond those; the two questions of §g
ride TODO_LIST as BLOCKED rows)

Already rowed by the closeouts (do not re-mint): floor pairing (935),
dupe-baseline --self-test + provenance + full ci-local + nolint ruling

- growth attribution, Every>1 done-guard + neutrality pin + rewind
  smoke + mirror-clones header scope line, day-suffix durations, stats
  --json band fields, heal `--pick`, baseline regen, AGENTS --clean-cache
  line, webui-css loud fallback, gosec N==1 stub, nowband windowing,
  smoke band assert, changelog-audit generalization, 23rd workstream,
  read-level changelog audit, v0.3.1 cross-ref. Genuinely new from this
  pass:

1. CHANGELOG entry for the sessions-lamp + `tq stats --since` batch —
   DONE BY THIS REPORT (CHANGELOG [Unreleased]); listed for the audit
   trail, not re-rowed.
2. TODO_LIST tick-sweep: 516 open unchecked rows against shipped
   evidence; at least three of this window's own family were
   done-but-unticked before their sessions caught them (01-28 §f6 —
   minted there as a closeout append, verify it survived dedup).
3. Golden pins for the remaining statsPayload wire surfaces (by_status,
   by_project, consumer_lag, claim_anomalies) — carried from 01-47 §e5,
   confirmed still unrowed beyond budgetUsageView.
4. AGENTS.md heal-script Known-Issues clause for the foreign-footer
   PARENT refusal variant (02-03 §f11).
5. Sweep docs/status/tasks/ closeouts for §b partials that never became
   rows (02-03 §f12) — the systematic version of what this report did
   for its own window.

## g) QUESTIONS (owner-only; also filed as BLOCKED TODO rows)

1. **Done-guard semantics at Every>1 rewind replays** (02-03 §g1):
   probe the dedup key directly and count known without a full window,
   or accept the silent no-op and scope the pin to Every=1?
2. **Sentinel-as-control-flow house ruling** (02-03 §g3): `errInFlight`
   sentinel unwrapped with errors.Is vs an honest `(time.Time, bool)`
   return — rule before row 503's sibling sweepers copy the pattern.
3. **Prune-stale dlqfix visibility** (00-31 §g1): keep total
   invisibility of autopsies (today's pinned ruling) or add a read-only
   reporting lane (`PruneResult.Notice`)?

## h) BAND DRIFT

The journal holds NO `task.reprioritized` facts — the fact-type census
at HEAD is claimed/enqueued/completed/failed/requeued/cancelled/
dead-lettered only. None recorded in this window; every claim-order
shift observed was ADR-0015 aging + stored-priority mechanics, not a
reprioritization event. No marker/AI/importance moves to account for.

## Appendix — docs-health pass notes (02:31)

- CHANGELOG [Unreleased]: minted the deferred sessions-lamp/`stats
  --since` Added entry + the status done-guard Fixed entry (this pass;
  closes the b2 partial).
- TODO_LIST: 7 rows appended (4 tasks + 3 BLOCKED rulings); all five
  closeout reports' own appends were already present — dedup held.
- Report indexed in docs/status/README.md (top 2026-10-20 cluster);
  `check-status-index.sh` ok, `check-todo-list.sh` ok.
- Archive sweep SKIPPED this pass: 324 live index rows (bloat warning
  firing since the threshold-100 convention), but a full ANNOTATE sweep
  over the 2026-09/2026-10 corpus is a dedicated multi-hour pass, not a
  window tail; no file in this window's blast radius was fully
  resolved-and-unreferenced. The bloat is itself the strongest signal
  for the digest/archive cadence row already in TODO_LIST's ecosystem.
- Advisory noted in passing: TODO_LIST row "dlqfix sweeper
  mint-refusal checkpoint hardening" carries a 2026-10-18 DONE
  annotation but is unchecked — one of its two cited pin tests
  (`TestSweeperMintRefusalHoldsCheckpoint`) verifies in
  internal/dlqfix/sweep_test.go:995, the other
  (`TestCursorRedeliversRejectedFactsOnRetrySweep`) was NOT found by
  grep, so the tick was withheld pending verification.
