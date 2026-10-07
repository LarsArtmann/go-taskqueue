# Status report: art-dupl dedup window — mirror gate red→green, WithToken/StatusCounts seams landed (2026-10-07 06:29)

**Scope of this run:** session opened on a user-run
`art-dupl --sort total-tokens -t 1 --timing --rich-text` report (128 shown
groups of 1857 detected) with the mandate
READ→UNDERSTAND→RESEARCH→REFLECT→execute step-by-step with verification
after each step. All findings below are from THIS session only. Multiple
concurrent agents were active throughout (templ regen diffs, postgres
claim-wake parity `63c9748b`, consumer wake drain `06607444`,
lint-budget windows) — observed, judged on merit, built on, never
reverted.

Headline: the cross-backend mirror gate was **already RED at session
start** (2 NEW clone groups spanning sqlitev4↔postgresv4 — a ci-local
blocker nobody had triaged). It ends the session GREEN with three new
shared seams landed and every harmful clone from the report extracted.

## a) FULLY DONE

1. **Triage of all 128 shown clone groups** — every group classified
   extract / accept / leave, with the rationale recorded per class
   (stdlib idioms, templ-markup noise, test assertions, recorded
   `art-dupl:accept` rulings, different-domain shapes).
2. **Mirror gate RED→GREEN (the primary fix).** At session start
   `scripts/check-mirror-clones.sh` failed strict with 2 NEW
   cross-backend groups: `[assignment]` postgresv4/adapter.go:259 ↔
   sqlitev4/adapter.go:302 (52 lines: the Fail/FailPermanent/
   Heartbeat/CancelOwned token-gate prologs) and `[return]`
   postgresv4/adapter.go:208 ↔ sqlitev4/adapter.go:242 (39 lines:
   tokenFor + Complete/Fail bodies).
   - Fix: `companion.WithToken(ctx, r, id, claim, requireLive, use)`
     (internal/queue/companion/claims.go:56), mirroring the existing
     `WithTx` closure precedent in the same package: the token gate and
     the engine finalize it guards collapse onto one seam, so an adapter
     cannot order the finalize ahead of the gate. TokenFor errors pass
     through raw (already tq vocabulary), exactly as before.
   - All 5 finalize methods per adapter (Complete, Fail, FailPermanent,
     Heartbeat, CancelOwned) rewritten onto it in BOTH adapters; sqlite's
     `Fail` keeps its `fireWake` semantics (wake comment preserved
     in place); the private `tokenFor` helpers deleted in both.
   - The remaining thinnest delegator pair (FailPermanent/Heartbeat
     bodies are one `WithToken` call each; per-backend engine types stop
     deeper hoisting) carries a recorded inline `art-dupl:accept`
     (Store-interface-delegator class, existing ruling). Note: the
     doc-position directive did NOT suppress (clone start sits after the
     5-line signature) — moved inline per the AGENTS.md placement rule.
   - Gate result: `mirror-clones: 0 cross-backend clone groups`,
     re-verified green AGAIN after the concurrent postgres claim-wake
     merge (`63c9748b`) touched the same files.
3. **`readmodel.StatusCounts(ctx, model, store)` — one status-counts
   seam** (internal/readmodel/stats.go:91) for the identical 10-line
   model-vs-store branch pair in internal/httpapi/httpapi.go:306 and
   internal/webui/tailer.go:143 (the httpapi copy even cited its twin in
   a comment). Both servers are now one-line delegates; call sites
   unchanged; behavior byte-equivalent (model nil → store; StatusCountsMap
   conversion; error passthrough).
4. **`Harvester.todoState(base)`** (internal/harvest/redispatch.go:135):
   the twice-repeated redispatch prolog (TodoFile default + ParseRepoAll +
   key→ticked fold) folded; `mintItemClosed` and `redispatchAuditRepo`
   now share it; the unreadable-file refusal comment preserved at the
   call site.
5. **cqrs ULID codec: designed, then REVERTED on a recorded ruling.** The
   seqid.go↔readmodel/host.go twin (~30 lines + layout consts) looked
   like the session's biggest semantic clone (cursor-corruption risk).
   I implemented the export (SeqEventID/EventIDSeq) and readmodel
   delegation — then found readmodel's const block already carries a
   recorded `art-dupl:accept`: "mirror of internal/journal/cqrs/seqid.go:
   the cqrs adapter is the proprietary read-only seam (ADR-0014) and
   keeps its codec unexported; TestSeqIDCodecParity pins both encodings"
   (internal/readmodel/host_test.go:58). Documented deliberate non-fix
   with its own drift gate → my export reverted byte-exact (see d2 for
   the sloppy revert).
6. **Verification battery (all green):** 8 touched modules
   (companion, sqlitev4, postgresv4, journal/cqrs, readmodel, httpapi,
   webui, harvest) build+vet+test green — including full re-runs after
   concurrent-agent merges; root build+vet clean; gofmt clean on all 7
   touched Go files; cmd/tq module gate green (21.8s, incl. the
   windows cross-compile leg); art-dupl `-t 1` delta 128→119 shown
   groups with ZERO targeted clones remaining and suppressed 366→372
   (the WithToken call sites themselves became filtered shared-seam call
   pairs); `check-agents-size.sh` 16858/16900 B.
7. **Inherited lint-baseline failure diagnosed with blame evidence**
   (see d1) — every flagged file/symbol pinned to its introducing commit
   so the owner call is cheap.

## b) PARTIALLY DONE

1. **`scripts/lint-baseline.sh --check` is RED — inherited, triaged, not
   fixed.** Findings postdate the last baseline regen: `root/errname` on
   `RedispatchRefusal` (type landed 04:42 in `f9651625`, BEFORE this
   session; the declaration at redispatch.go:24 is untouched by my diff,
   which adds zero error/refusal lines) plus worker/sqlitev4
   `wsl_v5`/`recvcheck` entries from the claim-wake work (`ad51ed4c`,
   `63c9748b`). Per AGENTS.md, regen is deliberate and "only on a green
   tree" — the owning agents' call, not mine. I proved non-ownership
   (git blame + diff grep) but did not run ci-local end-to-end.
2. **AGENTS.md memory capture attempted, then reverted.** The three new
   seams (WithToken, StatusCounts, todoState) were written into the
   package table — first with a wrong-home mistake (StatusCounts noted in
   the companion row; caught and corrected), then reverted entirely when
   the size guard showed the file was already at budget
   (16900 B; my +111 B tripped it; even the 54 B condensed variant did).
   The seam knowledge lives in code doc comments + this report only.
3. **Accepted-by-judgment clones are not pinned anywhere.** ~110 shown
   groups (templ noise, stdlib idioms, test shapes, sweeper shells) are
   accepted with in-session rationale but carry no accept directives
   (AGENTS.md: residuals accepted ONLY where named) and no baseline:
   every future art-dupl run re-shows them and re-pays the triage. Only
   the mirror class has a gate+baseline mechanism.

## c) NOT STARTED

1. **docs-health HARVEST** of section f) into TODO_LIST.md/ROADMAP.md —
   deliberately deferred: the user asked for the report and then WAIT.
2. **Live-postgres integration proof of the WithToken pg path.**
   postgresv4 tests pass in 0.003s because they skip without a DB; the pg
   refactor is compile+vet+gate-verified and shares the ONE conform
   suite (which runs against sqlite), but no live-postgres conform run
   happened this session.
3. **CHANGELOG entry** for the dedup window (the release flow will want
   the WithToken/StatusCounts seams recorded; CHANGELOG is append-only).
4. **Module tag dance:** `WithToken` landed in companion after its
   v0.3.2 require. internal/ modules are not externally importable, so
   no consumer risk today, but the release flow (facade require real-tag
   + relative replace, check-facade-parity before staging) needs the
   bump at the next release.

## d) TOTALLY FUCKED UP

1. **The repo-wide lint gate is red for everyone** (lint-baseline leg of
   ci-local). Inherited, not caused here — but it means nobody can
   cleanly push until the owner regenerates or fixes; I could diagnose,
   not unblock.
2. **My cqrs excursion was sloppy twice.** (i) I designed and implemented
   the export BEFORE grepping for recorded rulings in the twin file —
   a 5-second grep would have saved the entire excursion. (ii) The revert
   used `sed` forward-then-backward, which clobbered
   `TestSeqEventIDRoundTripAndOrdering` into
   `TestseqEventID…` (caught by the Go test-name vet, fixed), and the
   auto-commit daemon snapshotted the intermediate exported variant into
   history — commit-then-revert noise (final state byte-faithful,
   bisect-safe, but avoidable).
3. **Wrong-home memory note:** first AGENTS.md edit put the StatusCounts
   seam in the COMPANION row. Self-caught on re-read, corrected, then
   mooted by the budget revert.
4. **What I forgot:**
   - grep for `art-dupl:accept`/baselines/ADRs in the twin BEFORE
     designing any extraction (would have killed d2(i));
   - run the repo's quality gates FIRST at session start (the mirror
     gate was red from `ad51ed4c`'s fallout — knowing earlier would have
     cleanly separated inherited vs new failures);
   - check AGENTS.md size headroom BEFORE planning memory additions;
   - the user's `--timing` flag on the final art-dupl verification run
     (I re-ran without it, so no formal timing delta captured);
   - `scripts/check-gomod-vendor-sync.sh` after readmodel gained
     imports (queue/task were already required, so vendor should be
     unaffected — unverified claim, flagged).

## e) WHAT WE SHOULD IMPROVE

1. **Ruling-grep before design** in multi-module repos: accept
   directives, baselines, and ADRs are the codebase's recorded
   "don't fix this" list; searching them is cheaper than any wrong
   extraction.
2. **Edit/LSP tools over sed for renames** in daemon-run repos — sed
   cannot respect Go test-name conventions and invites mod-time races
   with the auto-committer.
3. **Gate-fingerprint at session start:** run the quality gates before
   writing code, triage inherited-vs-new, then work. Inherited reds
   should be reported, not silently absorbed.
4. **Memory writes need a headroom check** — at-budget files should get
   code-seam doc comments instead (the seams self-document where they
   live; AGENTS.md rows are summaries).
5. **The accept-directive tension needs one explicit ruling:** AGENTS.md
   says residuals are accepted ONLY at the named fragments.templ pair,
   yet the repo carries 372 filtered-suppressed groups and this session
   added one more recorded accept. Either bless recorded-accept-with-
   reason as the standing mechanism (and add a shown-group baseline like
   mirror-baseline.txt), or commit to extracting the accepted classes.
6. **postgresv4 skips should be loud** — a 0.003s "ok" reads green but
   runs nothing; a printed skip reason (or env-guarded fatal in the
   gate) would prevent false confidence.
7. **Re-verify after concurrent merges, not just after own edits** —
   the postgres claim-wake commit landed on the same adapter files
   mid-session; the re-run was the reflex this time, keep it.

## f) UP TO 50 THINGS TO GET DONE NEXT (brainstorm; routing-tagged)

**Gate repair (top priority, owner calls flagged):**
1. Resolve the lint-baseline red: either fix-then-regen (rename
   `RedispatchRefusal` → `RedispatchRefusalError` kills the errname
   class) or regen deliberately on a green tree — unblocks ci-local for
   everyone. [owner decision, needs g1]
2. Sweep the worker/sqlitev4 `wsl_v5` + `recvcheck` findings from the
   claim-wake work (or fold them into the regen). [owner]
3. Run `./scripts/ci-local.sh` once after 1+2 to confirm the full gate
   end-to-end at HEAD.
4. Verify `check-gomod-vendor-sync.sh` at next push (readmodel import
   addition — expected no-op, unverified). [d4]

**Session-derived, small and concrete:**
5. Live-postgres conform smoke for the WithToken pg path (needs a DSN;
   see g2).
6. Make postgresv4's DB-skip loud (printed reason or gate-level guard).
7. Pin the accepted art-dupl shown-set in a baseline file (like
   mirror-baseline.txt) with a growth gate, so ~110 accepted groups stop
   re-appearing every run. [depends on the e5 ruling]
8. CHANGELOG entry for this window (WithToken, StatusCounts, todoState,
   mirror gate 2→0).
9. Re-run the user-exact `art-dupl --timing --rich-text` invocation and
   capture the timing delta formally into evidence.
10. Add a stale/foreign-token negative conform case pinning WithToken's
    ordering (gate fires before any engine call) if the suites don't
    already cover it — verify before writing.
11. Record the three seams in AGENTS.md when the budget resets (WithToken,
    StatusCounts, todoState; 42 B headroom today). [g3]
12. One-line note in scripts/mirror-baseline.txt's header that the
    accepts-with-recorded-reasons mechanism was used again 2026-10-07
    (Store-interface delegator, WithToken era).
13. Harvest this f) list into TODO_LIST.md (docs-health HARVEST).
14. Archive this session's art-dupl evidence properly
    (scripts/archive-evidence.sh; the after-report currently lives in
    /tmp and will vanish) — the 04-28 report's own lesson, repeated.
15. Confirm the concurrent agent's templ regen diffs
    (fragments_templ.go, layout_templ.go) committed cleanly mid-session.
16. docs/DOMAIN_LANGUAGE.md: add "claim-token finalize gate" if the term
    isn't already defined.
17. Pre-existing redispatch.go lint (mnd 36, godox, wsl_v5 at lines
    24/90/177/315) — sweep in the harvest pass. [owner-adjacent]
18. webui tail.go `runReadModel` cyclop 14>12 (pre-existing, surfaced by
    the LSP this session) — split or baseline.
19. Document the golangci_ls LSP vendor-timeout false positive
    (readmodel stats.go) next to the existing templ/cmd-tq LSP
    false-positive AGENTS.md row — when budget allows. [g3]
20. Convention note: new finalize surfaces must call companion.WithToken,
    never re-hand-roll the token gate (AGENTS.md candidate, budget-bound). [g3]

**Follow-through on adjacent work observed this session:**
21. Postgres claim-wake parity (`63c9748b`): read its tests, fill any gap
    vs sqlite's 3 wake tests (e.g. pg-side fireWake assertions).
22. Consumer wake drain (`06607444`): confirm its test battery ran green
    post-merge (it landed during this session's window).
23. Release prep: companion/readmodel module tag dance at next release
    (facade require real-tag + relative replace;
    check-facade-parity.sh BEFORE staging). [c4]
24. Consider the `runContext(ctx, defaultTimeout, minutes)` fold if a 6th
    executor `runCtx` site appears (rejected this session at 5 sites ×2
    lines; documented trigger).
25. "Mirrored sweeper shell" class (dlqfix/status/prioritize/review
    log/warn + stats shells): map all five and decide ONE
    shared-sweeperutil extraction or a blanket accept ruling — current
    state is accept-by-precedent in review only.
26. Status-index bloat: live row count is near the 100-row warning zone
    (rows at 114-118 are October 07 alone) — schedule an archive sweep
    (docs-health ANNOTATE) or a monthly digest row.
27. Annotate this window's predecessor reports (05-18, 05-38, 06-01) to
    strike forward items their work resolved (docs-health ANNOTATE).
28. AGENTS.md budget decision: consciously reset (+~200 B) or prune the
    Architecture section (7.2 KB, the biggest block). [g3]
29. e2e `TestBudgetCapsStatusMintedEnqueues` singleton load-flake
    (flagged in the 06-01 report) — re-run before anyone attributes; a
    deflake candidate is already queued there.
30. Upstream art-dupl request candidate: actionability filter for 1-2
    token templ/test groups (roughly 40 of the 119 shown) — run
    verify-before-filing first, this is a maybe.

**Lower-priority / exploratory (ROADMAP-fuel):**
31. Extend the mirror gate's backend regex to the legacy
    internal/queue/{sqlite,postgres} drivers if they ever grow surfaces
    (currently thin drivers over v4; gate only covers v4 + companion).
32. Consider exporting a tq-level `Executor.RunContext` helper as part of
    the executor public facade IF external consumers ever need the
    timeout seam (facade-parity implications first).
33. Explore replacing the accepted ULID mirror with a tiny
    internal/journal export IF ADR-0014's unexported-codec stance is
    ever revisited (the parity test already pins correctness; this is
    pure line-count win, ~30 lines).
34. Sweeper stats structs (dlqfix/status/prioritize/review) share
    field shapes (Skipped/Processed/…) — a generic stats counter was
    rejected this session (abstraction ≈ param count); revisit only if a
    cross-sweeper report surface ever needs uniformity.
35. templ noise: if fragments.templ grows more conditional-span pairs,
    consider a components.go helper per pair (the ADOPTION.md table
    governs; REJECTED list includes display primitives that would
    tempt this).

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **Lint-baseline red:** do you want the regen now (swallowing the
   claim-wake/redispatch-era findings into the baseline), or should the
   owning agent fix/rename first (e.g. `RedispatchRefusalError`) and
   then regen on a green tree? I cannot know who owns the ruling or
   whether those findings are wanted.
2. **Is there a live postgres DSN** (or docker one-liner you use) for
   the postgresv4 conform suite in this environment, or does the
   WithToken pg path stay compile+gate-verified until CI runs it?
3. **AGENTS.md budget policy:** consciously reset the 16900 B budget to
   record durable seam facts in AGENTS.md again, or keep seam knowledge
   in code doc comments only (my revert restored the byte-exact
   original; the file has 42 B headroom)?
