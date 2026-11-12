# Status Report — Repair-Verify-Deepen Continuation Window (T10–T24)

**Date:** 2026-11-12 07:55 CET
**Session scope:** Execute the remainder of the owner-approved pareto plan
`docs/planning/2026-11-12_04-29_repair-verify-deepen-pareto-plan.md` under
the owner's blanket execute-directive, after the morning window closed at
T01–T09/T13/T15/T20 (report `2026-11-12_06-45_repair-verify-deepen-window-report.md`).
**Verification state:** root build+vet ok; webui suite ×1 + `-race` ×1 ok
(20.9s); readmodel/worker/executor module gates ok; doc-refs,
status-index, agents-size, dead-exports `--self-test`, ghost-archives,
todo-list, facade-agnostic gates all rc=0; receipts under
`docs/status/assets/2026-11-12_t10-prefetch-repro/`.

## a) What was executed (numbered, in order)

1. **T08 remainder** — the two drafted pin tests landed (58dd6a58):
   poisoned-UTF-8 store error coerces through the readiness body (valid
   JSON, valid UTF-8, no raw 0xff, text preserved, no-cache header) and
   the marshal-fallback static body is pinned via a recording writer.
   Baseline 12 + 2 pins green.
2. **T10 attempted + REVERTED** — the big finding of the window: wiring
   metaengine v4.23.0 `WithPrefetch` into `Model.TasksPage` broke the
   pinned cursor round-trip (6/6 deterministic FAIL, not flake): with a
   PrefetchCache attached, upstream ScanPage abandons the engine's
   compound sort-key cursor for the legacy bare-value cursor, and on
   tie-heavy ledgers every page mints the same timestamp cursor — the
   walk loops forever alternating two windows (receipt:
   prefetch-wired-roundtrip-FAIL.log, 10 identical pages). No partial
   adoption is safe (the poisoned derivation happens on page 1).
   Reverted (d52c8b50 + 3e4187a1); libs report finding 15/3 now documents
   the code-level trace; TODO_LIST carries the upstream-gated re-arm row.
3. **T11 templ-components audit** — v1.28 NOT cut upstream (fresh
   ls-remote per family, all at v1.27.0 = pin); the queued
   `NoScript`/`CopyButtonScript` adoption exists only in the 88
   unreleased commits (#63). Report Latest line cites the 2026-11-12
   re-check; bump + adoption stay blocked-on-tag.
4. **T12 ssetest swap** — the four hand-rolled `switch evt.Type` +
   `json.Unmarshal` loops and three raw substring scans in webui_test.go
   all ride one `fragmentEvents` helper (FilterByType + single decode
   site, EventsString in the failure detail). Report assertions row
   flips PARTIAL → FULL with the RequireDataJSON-by-shape note (dd4dda5c).
5. **T16 per-check abandonment** — evaluate's blocking probes
   (database/workers/queue/projection cursor+head) ride a `boundedCheck`
   wrapper (the upstream runBoundedCheck shape, hand-rolled — upstream
   keeps it unexported): a context-ignoring wedge is abandoned at the
   batch deadline fail-closed while the batch completes. The first draft
   missed the recover — the pre-existing panic test caught the crash
   class immediately; the final shape re-panics on the caller's stack
   (evaluateSafely's contract intact) and folds post-abandonment panics
   into errors. Pinned by TestHealthSlowCheckAbandonedAtBatchDeadline;
   suite green ×3 + race clean (7eeb4964 + ec166052 daemon sweep).
6. **T17 health.Off sentinel** — an unmounted read model emits a visible
   `off` row with the enable recipe (`--read-model`) instead of nothing;
   upstream grades StatusOff with pass, so readiness never flips. First
   draft put the row on the wrong branch (webui.New always wires the
   closure) — caught by the test, fixed forward (bfaa229f → 85e5c4c8);
   both sibling omit-assertions flipped to off-row assertions.
7. **T18 WrapOnce collapse RULED OUT** — the plan's premise fails on
   evidence: `WrapOnce` wraps into the library's `*Error`, but
   `PermanentError` is the domain type `ClassifyFailure` detects via
   AsType — a swap degrades every permanent refusal to the transient
   ladder and breaks the identity/prefix pins. Report rows flipped
   MISSED → REJECTED BY DESIGN (commit in docs).
8. **T19 metrics flag** — `Config.Metrics` (default off) enables
   `dashboard.WithMetrics` + the `/health/metrics` binding inside the
   same header/token stack; TestHealthMetricsFlag pins 404-off /
   401-unauth / 200-with-token. Two draft bugs caught by the test
   (Routes-by-value ordering; tq's explicit healthBindings bypass the
   dashboard mux) — fixed forward. cmd/tq flag surface deliberately
   unwired (owner no-op path). Reports re-scored:
   go-health-dashboard 79→87, go-health 41→59 (four rows landed via
   T08/T09/T16/T17), go-error-family 21→29 (T07); index.html rows
   updated.
9. **T21 could-not-verify closures** — go-sse's 100% coverage claim
   re-measured at the checkout HEAD (100.0% of statements, root
   package); go-etag's favicon Set-site located in production code
   (go-health-dashboard favicon.go:23, mounted via FaviconHandler).
   Zero unverifiable claims left in the two reports.
10. **T22 decision memo** — `docs/planning/2026-11-12_07-48_decision-memo-owner-questions.md`:
    the 4 §g questions with recommendations (push stays owner ritual;
    doctor-test exemption recommended with the 15-minute shape; server_timing
    keep; harvest default = auto-harvest agent-executable rows, BLOCKED-note
    owner-gated ones) + libs cadence proposal (monthly verify replay + weekly
    watchlist round).
11. **T23 watchlist round** — zero upstream drift since 11-11 across all
    six families (cqrs still no queue/v5 — T01's restore stands;
    severity-override still PROPOSED; receipt:
    upstream-watchlist-2026-11-12.log).
12. **T24 harvest** — 3 agent-executable capability rows into TODO_LIST
    (shutdown awareness, checks.Disk, WithIntrospection) per the memo's
    standing default.
13. **T14 digest sweep** — the ratified ANNOTATE+ARCHIVE ritual against
    the 425-row bloat warning: 28 window reports from the live 2026-09
    cluster banner-archived with manifest rows, 12 legacy root closeouts
    routed to tasks/ (O7), digest 2026-11 row landed (e09b3404). Index
    + ghost-archives gates green; the bloat warning is silent. One
    CHANGELOG cite repointed at an archived path (gate-caught).

## b) Findings (things discovered, not scheduled)

1. **The plan's T10 premise was wrong at the pinned version** —
   "TasksPage is exactly the coached cursor-scan shape" fails because
   the prefetch path swaps the compound cursor for a tie-lossy legacy
   one. The pinned round-trip test did exactly its job; the honest
   landing is revert + re-arm row, not a workaround.
2. **T18's premise was wrong at any version** — WrapOnce collapses the
   library's type; the domain guard IS the analog for the domain type.
   Ruled out, not deferred.
3. **Two draft bugs shipped and were caught by the pins within the same
   window** (T17's wrong branch; T19's routes-by-value ordering + the
   bindings bypass). The write-test-then-keep-red-then-fix rhythm paid
   twice; one slip (bfaa229f committed with a failing suite) was
   corrected forward within minutes — the chain-vs-test discipline gap
   is noted, not hidden.
4. **The daemon pre-empted most staged commits again** (footer-less
   chore sweeps: c11b497e landed my T10 edits mid-investigation and the
   `git checkout --` revert restored HEAD's content, not the clean tree
   — the "revert" needed a forward `git revert` instead). Content always
   landed; attribution is smeared as usual.
5. **TestHealthEvaluationPanicRecovers is load-bearing in a new way**:
   it now also pins that boundedCheck re-panics synchronously — the
   panic contract survived the abandonment refactor unbroken.
6. **golangci LSP diagnostics stayed stale all window** (the phantom
   `.golangci.yml` duplicate-settings error persists; `config verify`
   rc=0 at HEAD; the `v5` noise likewise) — CLI gates remain the only
   truth.

## c) Next items (owner input or next window)

1. **Push ruling** (memo §1): ~45 commits ahead; cmd/tq graph loads and
   CI heal the moment it lands.
2. **Doctor-test exemption** (memo §2): 15-minute edit once the owner
   acknowledges the recommended shape.
3. **The 3 harvested capability rows** (TODO_LIST): shutdown awareness,
   checks.Disk, WithIntrospection — all agent-executable with code
   paths cited.

## d) Improvements made (not in the plan)

1. CHANGELOG gained the three operator-visible entries (metrics flag,
   off-row BEHAVIOR CHANGE, abandonment + durations) per O14.
2. index.html "Top opportunity" cells de-drifted for templ-components
   (stale health text) and updated for the re-scored rows.
3. The digest sweep's script ran dry-run-first (40/40 rows matched in
   rehearsal) per the doc-mutation convention.

## e) Process notes

- Bash-guard rails hit: heredoc→Go blocked (used edit tools), non-Go
  fixture redirect warned (used /tmp), TQ_DB warning on every gate run
  (no tq serve ran this window; scratch-DB rule unbroken).
- `go -C <module>` avoided the cd-relative-path trap; one redirect still
  resolved against the wrong cwd and was re-run from the repo root.
- Evidence files under docs/status/assets/ get daemon-swept (committed)
  — used deliberately for receipts.

## f) Follow-ups harvested or skipped

1. T10 re-arm row — filed in TODO_LIST (upstream-gated).
2. Shutdown-awareness / checks.Disk / introspection rows — filed (T24).
3. Owner-gated rows (push, doctor-test, server_timing) — NOT
   force-executed; memo carries recommendations. `<!-- fresh:skip:
   owner-gated rows are memo'd in docs/planning/2026-11-12_07-48, not
   harvested against standing rulings -->`
4. Cadence proposal — memo'd, one-line adopt/veto asked.

## g) Owner questions

1. Push? (memo §1 — the standing recommendation is yes, plain push, no
   force.)
2. Acknowledge the doctor-test exemption shape? (memo §2 — then it's a
   15-minute edit.)
3. Adopt the harvest default (auto-harvest agent-executable, BLOCKED-note
   the rest)? (memo §4.)
4. Adopt the libs cadence (monthly verify replay + weekly watchlist)?
   (memo §5.)
