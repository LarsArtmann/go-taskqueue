# ART-DUPL T4 ZERO — DEDUP SWEEP (interactive, 13:00–14:00 window)

Status report per the a)-g) skeleton. Session goal (owner, verbatim):
"deduplicate to fucking zero" — drive the art-dupl `-t 4` report to zero
shown groups at HEAD. Start state: 11 shown groups (owner's `-t 4
--suggest-generics` paste) on top of my own earlier `-t 1` pass (same
session, `topLevelScalar` in internal/harvest/priority.go).

**HEAD at report time:** 80b8ed6c (daemon-planned Pareto plan commit).
**The report is clean: `art-dupl --sort total-tokens -t 4` → 122 clone
groups detected, 0 shown.** The canonical `-t 3 --type-aware` view
(the mirror gate's input) still shows 16 groups — all pre-existing
idiom (see §b/§e).

---

## a) FULLY DONE

Extractions — every one a named seam, built + tested at its module:

1. **`topLevelScalar(data, key)`** — internal/harvest/priority.go:181.
   `ReadImportance` + `repoPurpose` shared a hand-rolled "read one
   top-level scalar from the flat metadata file" loop; extracted (-t 1
   pass). Harvest module build+vet+test green, gofmt clean.
2. **`copyOptionalTable`** — internal/queue/sqlitev4/migration/migrate.go
   (~:370). The `sourceTableExists` guard + `copyQueriedRows` body
   duplicated across `copyWatermarks`/`copyPriorityScores`. sqlitev4
   module green (incl. migration tests).
3. **`withSource(fromPath, fn)` + `migrateInto`/`verifyAgainst`** —
   migrate.go:166. Both replay entry points (Migrate, Verify) shared the
   `openSource` + `defer Close` bracket; now one closure-scoped seam,
   stats/report preserved via named returns + pointer. Migration tests
   green (copy + verify paths both exercised).
4. **`agentRepo(ctx, agent, repo, cleanTree)`** — internal/executor/
   preflight.go:31. The nil-defaulting `base()` + `prepareRepo` prelude
   ran identically in dlqfix/prioritize/review/status; all four now call
   the seam and the four per-type `base()` methods are DELETED (zero
   callers, verified by grep before deletion).
5. **`finishParsedRun[T]`** — internal/executor/result.go:154. Generic
   over `interface{ *R; setLogPath; deriveUsage }` (same constraint
   pattern as `recordRunOutcome`): parse-error wrap (`"<kind>: %w"`) +
   `deriveUsage` + `recordRunOutcome` tail deduped across dlqfix +
   prioritize. Executor module green incl. `-race` (9.9 s) and
   GOOS=windows cross-build.
6. **Conform-suite helpers** — internal/queue/companion/conform/tests.go:
   `assertTaskState`, `assertNoTaskDue`, `startSeededStore`,
   `filterCase`+`assertFilterCounts`. Killed the 14-line assert pair
   (fail1/requeue), the 3×11-line seeded-store prologs, and the query
   loop twins. All three conform consumers green: sqlitev4 (3.7 s),
   cqrsqlite (3.7 s), postgresv4. NOTE: the requeue site now pins
   `LastError: "preflight: repo dirty"` — a deliberate strengthening
   (comment in place); the first draft over-pinned and broke all three
   backends loudly (see §d).
7. **WebUI badge/meta single-sourcing** — internal/webui/
   components.go:113 (`badgeInfo`, `metaChip`, `taskBadgeExtras`,
   `nowbandMetaChips`) + fragments.templ:23 (`badge`, `metaCount`
   templ components). taskBadges renders one status badge + a loop over
   the assembled stack; the nowband meta band renders one loop over
   assembled chips (sessions two-count/lamp + budget metered span stay
   inline by design). `templ generate` + `nix run .#webui-css` both run
   (fragments_templ.go + app.css regenerated, committed by daemon);
   webui tests green (10.2 s).
8. **Accept directives with recorded reasons** — the skill's "leave a
   one-line rationale" mechanism, already used by the spike adapters:
   - Sweeper shells (internal/dlqfix/sweep.go, internal/review/sweep.go):
     `art-dupl:accept mirrored sweeper shell … shared pump already lives
     in watermark.Cursor` — matches the 12-27 dupe-type review ruling.
   - `claimCount`/`ClaimCount` (cmd/tq/main.go, internal/queue/queue.go):
     replace-free module boundary, ADR-0017; the 12-line census helper is
     restated by necessity.
   - `*exec.Cmd` ×4 (cmd/tq/crush.go forwardSignals, internal/e2e/
     e2e_test.go runWithTimeout, executor processgroup_unix/windows):
     stdlib type plumbing / platform split; directives in all four.

**Gates at finish (all green):** art-dupl -t 4 → **0 shown**; scripts/
check-mirror-clones.sh → `0 cross-backend clone groups`; module gates
for queue, dlqfix, review, harvest, sqlitev4 (+migration), cqrsqlite,
postgresv4, executor (-race), companion vet, webui, cmd/tq (scripts/
test-cmd-tq.sh, 13.5 s); root `go build ./...` rc=0; scoped gofmt on all
touched files clean; `go vet` on webui+e2e rc=0.

## b) PARTIALLY DONE

1. **The `-t 3` tail (16 groups) is judged, not driven to zero.** All
   pre-existing idiom: reads.go QueryContext preambles, lockout mutex
   triples, cmd/tq flag-set prologs, error-return ladders, `var buf
   bytes.Buffer` pairs, conform `t.Fatalf` pairs, the review/status
   Permanent-payload prolog (reduced by agentRepo but still type-2
   similar at -t 3), freshStore prologs at tests.go:3286/3402. I ruled
   the owner's bar was the pasted `-t 4` report; -t 3 remains the mirror
   gate's input, which passes (it only fails cross-backend). §g-1 asks
   for the standing ruling.
2. **AGENTS.md dedup-convention line not updated.** The existing
   "Residual art-dupl groups accepted; don't add new ones" should name
   the `art-dupl:accept` directive as THE mechanism. Skipped: the file
   sits at 15,694/15,700 B (6 B headroom — the guard prunes, never
   pads); an edit must be byte-neutral or paired with a prune. The
   authoritative documentation lives in scripts/check-mirror-clones.sh's
   docstring meanwhile.
3. **copyDeps still hand-rolls its rows loop** (migrate.go ~:298) — same
   exists-guard as the two functions I deduped, but its insert loop
   doesn't use copyQueriedRows, so the extracted helper doesn't reach it.
   Not flagged at -t 4; left alone.
4. **`-race` coverage partial.** Executor got `-race`; sqlitev4,
   cqrsqlite, postgresv4, queue, dlqfix, review, harvest, webui ran
   single-pass only. The AGENTS known-issue (concurrent agents → re-run
   -race before declaring success) is satisfied for executor, not for
   the rest.
5. **Evidence not archived to files.** Gate rcs live in this report's
   prose from the session log, not in a dated evidence file (the
   "battery rc TO A FILE" convention). Nothing is in dispute — every
   gate is re-runnable in seconds — but the letter of the convention is
   unmet.
6. **e2e suite not run.** internal/e2e/e2e_test.go got a comment-only
   edit (accept directive); the package was vetted, its tests (heavy:
   build tq binary) were not executed.

## c) NOT STARTED

1. **`scripts/heal-daemon-sweep.sh` on this session's commits.** The
   daemon folded my work into footer-less `chore: auto-commit` commits
   throughout; no Task-Queue-ID footer exists on any of them (this
   session ran without a task id, so `--task-closeout` was never in
   play). Healing is available and unstarted.
2. **CHANGELOG entry** for the dedup pass (append-only file, untouched).
3. **FEATURES/TODO_LIST harvest** of the new seams (they are internal
   implementation, but the conform-suite invariants and the accept-
   directive convention are doc-worthy).
4. **ADR/status cross-link**: the 12-27 dupe-type review (row 12-27)
   ACCEPTED the sweeper shells and several twins; this session EXECUTED
   the extraction side. No pointer was added from that report to here or
   vice versa (only the index rows sit adjacent).
5. **lint-baseline / gosec re-check** after new code (components.go grew
   ~60 lines; golangci is advisory but `--check` gates growth).

## d) TOTALLY FUCKED UP (all caught; none shipped broken)

1. **I silently deleted the sessions spans from the nowband.** The
   metaCount-loop rewrite's old_string included the two sessions
   `if`-blocks and my replacement dropped them. I caught it by re-reading
   the region immediately after — but **webui tests had already passed
   with the spans gone**, meaning the meta band's session chips have NO
   test coverage. A one-read recovery, not a process catch. This is the
   session's most dangerous moment and it is a TEST GAP, not a save.
2. **First `assertTaskState` draft changed test semantics.** I pinned
   `LastError: ""` at the requeue site where the original asserted only
   status/attempts/leaseOwner — broke all three backends (sqlitev4,
   cqrsqlite identically). The store was right: requeue keeps the reason.
   Fixed by pinning the actual reason with a comment; but the reflex
   "normalize onto my helper" beat "preserve the test's intent" on the
   first try.
3. **Three edit-mechanics fumbles in one file batch:** sent the
   `agentRepo` rewrite to result.go instead of preflight.go (2-of-3
   multiedit masked it), issued a status.go multiedit whose second edit
   was old==new (silent no-op that left `e.base()` dangling against a
   deleted method — compiler caught it), and dropped review.go's Execute
   doc comment inside a larger replacement (caught on the very next
   read). Cost: three fix cycles. Root cause: batching edits beyond my
   verification bandwidth.
4. **Misattributed the vendor error.** I twice read the gopls
   "inconsistent vendoring (go-sse v0.2.1)" error as another agent's
   in-flight bump and ran `go mod vendor` defensively. The 13-31 report
   had already ruled this exact error the known LSP false-positive class
   (both sides at v0.2.1). Harmless (idempotent), but my diagnosis was
   wrong and I briefly reported it as "foreign in-flight breakage."
5. **cmd/tq gate failure attributed before reading.** I called the
   TestAgentsDocSizeGuard failure "another agent's in-flight work,
   self-healed" — right outcome, but I first spent a diagnostic cycle
   sizing AGENTS.md against a stale working-tree snapshot; the 13-31
   session's report (row 13-31) documents the whole 17,334→15,694 arc.
   The gopls "56 undefined symbols in cmd/tq" are also false (CLI gate
   green) — consistent with the AGENTS known-issue, which I should have
   checked FIRST.
6. **Report-side:** my first two "final" summaries cited `-t 4` numbers
   from runs the owner's own pastes predated (stale-tree confusion, the
   "???" moment) — resolved by always re-running at HEAD before judging.

## e) WHAT WE SHOULD IMPROVE

1. **Test the nowband meta band and the badge stack** (render-level):
   chips present/absent per count>0, session chips, badge stack order.
   §d-1 is the proof this surface is unprotected.
2. **Edit discipline:** one logical edit → immediate read-back for any
   multiedit over two edits; never let a replacement span content I'm
   not re-adding verbatim. The doc-comment drop and the sessions-span
   deletion share this root.
3. **Test-intent preservation rule:** when collapsing asserts into a
   helper, diff the PIN SET first (what the old asserts checked vs what
   the helper checks) and either match it exactly or strengthen
   deliberately with a comment.
4. **Check the known-issues list before diagnosing LSP/vendor/gate
   anomalies** — three of this session's confusions were already
   documented (templ+cmd/tq LSP false positives, vendor-error class,
   load-flaky exactly-once).
5. **Persist gate rcs to a dated file** during the pass, not only in the
   report (the re-dispatch convention exists because chat citations rot).
6. **Adopt `-race` as the default module-gate invocation** in this repo's
   docs; single-pass "ok" undersells the exactly-once guarantees.
7. **The dedup bar should be a pinned command**, not a per-session
   judgment: a tiny scripts/check-dedup.sh wrapping the chosen
   art-dupl invocation would make "zero" a gate instead of a vibe.

## f) NEXT (ranked, ≤50; session residue first, then adjacent debt)

1. WebUI render test: nowband meta chips (total always; parked/budget/
   stranded/loopsuspect/journal conditional; session chips; budget
   metered span) — closes §d-1's gap.
2. WebUI render test: taskBadges stack (status leads; extras order
   review→status→agent→prioritize; empty stack = status only).
3. Standing ruling on the dedup bar (-t 4 reporting vs -t 3 canonical)
   — §g-1; then pin it in scripts/check-dedup.sh.
4. Run scripts/heal-daemon-sweep.sh over this window's footer-less
   daemon commits (subject/stats/tree verification) — §c-1.
5. CHANGELOG entry: dedup-to-zero pass (7 seams + accept directives).
6. AGENTS.md: fold the art-dupl:accept mechanism into the existing
   dedup line byte-neutrally (prune-first if needed) — §b-2.
7. Full ci-local battery at current HEAD (the pre-push gate; my gates
   were scoped).
8. `-race` re-run for sqlitev4, cqrsqlite, postgresv4, queue, dlqfix,
   review, harvest, webui — §b-4.
9. Persist this session's gate battery to a dated evidence file — §b-5.
10. Run the e2e suite once over the comment-touched file — §b-6.
11. lint-baseline `--check` + check-gosec after components.go growth.
12. Route copyDeps through copyQueriedRows (or accept-directive the
    guard) — §b-3.
13. cmd/tq claimCount: implement the documented GROUP BY store method
    (comment at cmd/tq/main.go:1919 names it) → delete the accepted
    mirror entirely.
14. Consolidate the three mirrored conformance suites into
    companion/conform (mirror-baseline TODO_LIST item, per gate script).
15. Migrate replay/main.go onto withSource if it duplicates the bracket.
16. Conform: sweep remaining Get+multi-assert sites onto assertTaskState
    where the pin set matches (tests.go ~141-160 et al.).
17. Conform: adopt startSeededStore at tests.go:3286/3402 prologs (-t 3).
18. review/status Permanent payload prolog: either a shared
    contract-miss constructor or accept directives at the chosen bar.
19. reads.go QueryContext preambles (-t 3): row-scan helper or
    accept-with-reason per site.
20. lockout mutex-guard triples (-t 3): accept directive (concurrent
    access methods are intentionally repetitive).
21. cmd/tq flag-set prologs (watermarks show/set etc., -t 3): accept
    directives (flag CLI shape is inherently repeated).
22. `var buf bytes.Buffer` pairs (command/depbump/agent/verifygate,
    -t 3): accept directives (idiomatic exec plumbing).
23. Verify gopls cmd/tq false errors clear after a clean vendor + restart
    (AGENTS already declares the class; one confirmation closes it).
24. Cross-link the 12-27 dupe-type review report to this pass (the
    ACCEPTED entries now have extractions or directives).
25. internal/webui/ADOPTION.md: note the local badge/metaCount wrappers
    sit ON TOP of display.Badge (library usage unchanged).
26. Consider templ-components upstream: a two-count chip and a metered
    chip component if the sessions/budget spans ever multiply.
27. Decide whether the mirror gate should also span executor/ (its
    mirrors are currently convention+directive governed).
28. status.go/review.go tails: evaluate a shared post-parse phase ONLY
    if a third consumer appears (note recorded here).
29. filterCase table style: propagate to other conform tests with inline
    filter tables.
30. Session-close ritual: `tq session close` to mint the review/status
    tasks for this window (none exist — session ran bare).
31. Archive-sweep candidate: mark THIS report for the next docs-health
    pass once items 1-9 land.
32. Check whether any other sweeper-shell twins (prioritize/status
    sweep.go) drifted from the accepted mirror pair since 12-27.
33. Budget-facade: confirm finishParsedRun didn't change result JSON
    shapes (recordRunOutcome path unchanged — quick `tq show` smoke).
34. Smoke: `tq serve` render of the new nowband (visual regression
    against app.css regeneration).
35. Confirm daemon folded fragments_templ.go + app.css together with
    fragments.templ (no split-brain commit) — check git log -p once.
36. template lint: templ check in CI? (templ generate is manual today).
37. Replay round-trip test at HEAD (Migrate+Verify against a fixture DB)
    to certify the withSource refactor beyond unit gates.
38. Capture `art-dupl -t 4` HTML at zero as a baseline artifact for
    --diff-report future runs.
39. docs/DOMAIN_LANGUAGE.md: no new terms introduced (verified — no
    action; recorded to close the loop).
40. Owner ruling on heal-daemon-sweep vs leave-as-is for task-less
    windows (§g-3).

## g) QUESTIONS FOR THE OWNER (cannot self-answer)

1. **Which threshold is the standing bar?** `-t 4` is now zero; the
   mirror gate's canonical `-t 3 --type-aware` still shows 16 groups of
   idiomatic similarity (mutex triples, flag prologs, error ladders).
   Do you want `-t 3` driven to zero too (real churn, negative-value
   abstractions), or is the ruling "report at -t 4, gate cross-backend
   at -t 3"?
2. **Directive durability:** the accepted mirrors (Sweeper shells,
   claimCount, exec.Cmd) now carry `art-dupl:accept` reasons. Should a
   check script enforce that edits touching those regions keep the
   directives (or that groups suppressed by directives never regrow
   larger), or is the directive comment plus the mirror gate enough?
3. **Daemon commits from task-less windows:** this session's work was
   folded into footer-less `chore: auto-commit` commits with no
   Task-Queue-ID. Do you want heal-daemon-sweep run for windows like
   this one as a standing rule, or is the footer-less sweep acceptable
   provenance for interactive (non-task) sessions?
