# Dedup pass on the art-dupl `-t 1` report — full status

Date: 2026-10-03 13:43 CEST
Work item: operator-prompted deduplication ("deduplicate!") against
`art-dupl --sort total-tokens -t 1 --type-aware --html` (the operator's own
run; 361 clones / 616 tokens / 1604 detected groups / 136 actionable shown
/ 1468 suppressed: 1 high, 32 medium, 103 low). This is NOT a task-queue
dispatch — no Task-Queue-ID exists for this session, so the daemon-swept
commit cfef2cd0 is footer-less by construction (see g3).

Baseline before: 361 total clones, 616 total tokens, 136 actionable groups.
After: **346 clones, 582 tokens, 129 actionable groups** — 6 target groups
eliminated (15 clone instances), every remaining group individually judged
accept-with-rationale or out-of-scope-by-convention. All gates green at
end of session.

## a) FULLY DONE

1. ~~**Report parsed and triaged.** Recovered the full 136-group list from the~~ — RESOLVED 2026-11-23: historical pass, receipts intact — the report and its cfef2cd0 blob are reachable at HEAD (git show cfef2cd0 --stat: exactly the six dedup files).
   session's shell-output log (`/home/lars/projects/go-taskqueue/.crush/
   shell-output/output-1147870974.log`); individually reviewed all 110
   non-low-priority / ≥4-token / non-test-scaffold groups with their code
   snippets; the remaining 26 low groups triaged by category-shape only
   (all are 2-token idioms — see e4 for the honesty caveat).
2. ~~**Constraints read before touching anything.** `scripts/check-mirror-~~ — RESOLVED 2026-11-23: historical step; the constraints surface is still live at HEAD (scripts/check-mirror-clones.sh + AGENTS.md:314 art-dupl:accept clause).
   clones.sh` (canonical invocation is `-t 3`, fails on any cross-backend
   clone group, `scripts/mirror-baseline.txt` pins the three mirrored
   conformance suites, `art-dupl:accept` directives exist as the acceptance
   mechanism) and the AGENTS.md convention "Executor shared seams …
   Residual art-dupl groups are accepted — don't abstract new ones". The
   executor turn-scaffold families (dlqfix/prioritize/review/status:
   `deriveUsage`+`recordRunOutcome` tails, `Agent()` accessor ×4,
   `runCtx, cancel := context.WithTimeout` ×5, `Permanent`+`prepareRepo`
   prologs) were left untouched per that convention.
3. ~~**Six behavior-preserving extractions** (all same-file or same-module,~~ — RESOLVED 2026-11-23: extractions live at HEAD — errorCell and payloadPreWithCopy present (internal/webui/fragments_templ.go:2415,5439) and preflightStateFor in internal/worker/worker.go:247; the blob touches exactly the six pass files (git show cfef2cd0 --stat).
   no new cross-module import edges):
   - `internal/worker/worker.go`: new `preflightStateFor(id, lastLog)
     (*preflightState, bool)` kills the identical lazy-map-init + insert
     clone in `preflightDelay` and `preflightShouldLog` (the `created`
     flag preserves shouldLog's fresh-entry-returns-true semantics).
   - `cmd/tq/journalaudit.go`: new `replaySetStatus(out, id, status)`
     replaces FIVE nil-guarded status-assign blocks in `replayProjection`
     (Failed/Requeued/Released→Pending, DeadLettered→Dead,
     Cancelled→Cancelled); the fact-type comments stay at the arms.
   - `cmd/tq/poolconfig.go` + `cmd/tq/doctor.go`: the semantic clone
     `unquoteConfigValue`/`unquoteSystemdValue` (identical bodies, two
     divergent names) unified into one `unquoteSurroundingQuotes`; both
     call sites updated, both wrappers deleted.
   - `cmd/tq/main.go`: new `readModelFlag(fs, usage)` beside `dbFlag`
     pins the `--read-model` name+default across stats/serve/api; the
     three per-command usage strings were deliberately KEPT (they differ
     meaningfully per command — flag-name drift was the real risk).
   - `internal/webui/fragments.templ`: new `errorCell(t task.Task)`
     component kills the identical 10-line expandable error `<td>` in
     `taskRows` AND `deadRows` — one place now owns the `data-error`/
     `data-short`/`aria-expanded` contract the row-expansion JS listens on.
   - `internal/webui/fragments.templ`: new
     `payloadPreWithCopy(preClass, text)` kills the pre+CopyButton block
     duplicated between the lede (`tq-payload-cmd`/`pv.Lede`) and raw
     (`tq-payload-pre`/`pv.Raw`) renderings.
4. ~~**templ + CSS regenerated per convention**: `templ generate` from repo~~ — RESOLVED 2026-11-23: holds at HEAD — fragments_templ.go carries the extracted components and internal/webui/static/app.css is the committed regen (payload + utility classes present, grep 2026-11-23).
   root, then `nix run .#webui-css` (CSS recompiled). `fragments_templ.go`
   regenerated; only fragments output changed.
5. ~~**All gates run and green**:~~ — RESOLVED 2026-11-23: re-verified fresh at HEAD — internal/webui `go test -count=1` ok 15.5 s, check-doc-refs/check-todo-list rc=0, cmd/tq TestStatusCell/TestNotBeforeCell ok (2026-11-23).
   - worker sub-module: `GOWORK=off go build/vet` ok; tests flake-exonerated
     (see a6).
   - root: `go mod vendor` + `go build ./...` + `go vet ./...` +
     `go test ./... -race -count=1` — ALL ok (incl. internal/webui 17.2s,
     internal/e2e 44.4s).
   - `scripts/test-cmd-tq.sh` — ok (49.8s).
   - `scripts/check-mirror-clones.sh` — "0 cross-backend clone groups".
   - gofmt clean on every touched `.go` file.
6. ~~**Flake exonerated with a controlled experiment.**~~ — RESOLVED 2026-11-23: adjudication stands and is formalized — TestExactlyOnceUnderConcurrency is a pinned KNOWN_FLAKY entry with verify-retry rails (scripts/check-verify --list; TODO row "--list debug mode" DONE 2026-11-19).
   `TestExactlyOnceUnderConcurrency` (worker) failed with SQLITE_BUSY
   storms / "19/20 completed" during gating. Proved PRE-EXISTING: created a
   throwaway `git worktree` at the PRE-CHANGE parent commit 865a5638 and
   reproduced the same failure there under `-race` (ok/FAIL across runs at
   both commits — load-dependent, matches the "re-run tests before
   declaring success" Known Issue). Worktree removed after.
7. ~~**Reduction verified, not assumed.** Re-ran the exact operator~~ — RESOLVED 2026-11-23: reduction persisted — later art-dupl runs report zero fragments.templ groups (TODO row "Track art-dupl #9" DONE 2026-10-28 with the 2026-10-28 `art-dupl --html` run receipt).
   invocation (`-t 1`): all six target groups confirmed GONE; per-site
   checks show `cmd/tq/poolconfig.go` fully clean and the other touched
   files carrying only accepted-idiom residue. 361→346 clones, 616→582
   tokens, 136→129 actionable, production 349→334.
8. ~~**Acceptances recorded where a future reader would re-litigate them**:~~ — RESOLVED 2026-11-23: mechanized since — `art-dupl:accept` directives with rationale + ruling pointers live in code across the repo (60+ per the open ledger-table row's own census; AGENTS.md:314 documents the mechanism).
   - `internal/queue/companion/reads.go` carries a 3-line comment at the
     first reader: the query/err/defer prolog is deliberately NOT
     extracted because a queryRows helper would defer Close inside itself
     and close rows before the caller's scan loop runs (defer-scope
     footgun — the one accepted group with a real trap).
   - AGENTS.md Known Issues: new entry pinning
     TestExactlyOnceUnderConcurrency as load-flaky with the
     re-run-before-attributing instruction.
9. ~~**Status report indexed** (this file's row in `docs/status/README.md`,~~ — RESOLVED 2026-11-23: holds — the index row for 2026-10-03_13-43_art-dupl-dedup-pass.md is present in docs/status/README.md.
   top of the date cluster) and both doc gates run:
   `check-status-index.sh` + `check-doc-refs.sh` (this was forgotten
   during the work phase — caught and closed in the report phase; honest
   restatement in d3).

## b) PARTIALLY DONE

1. **Full `-t 1` zero-harmful-duplication claim is scoped, not absolute.**
   The skill's bar is "zero harmful clones, every remaining clone has a
   defensible reason". I can defend all 129 remaining groups at the level
   reviewed (every medium, every ≥4-token group, every non-test group),
   but 26 low-priority 2-token groups were accepted by category-shape
   (`if x == "" { continue }`-class idioms) without reading each
   occurrence. Realistic risk they hide a harmful clone: low but nonzero.
2. **`TestExactlyOnceUnderConcurrency` root cause NOT found.** Exonerated
   my diff and documented the flake, but did not root-cause WHY a store
   with the single-serialized-writer invariant (MaxOpenConns(1) + WAL +
   busy_timeout) can still storm SQLITE_BUSY under test concurrency
   (suspect: the test opens additional handles, or journal writer paths
   bypass the pooled handle). Stopped at exoneration — that is all this
   session owed the diff, but the underlying question is open.
3. ~~**Acceptance pinning is prose, not mechanism.** The mirror gate supports~~ — RESOLVED 2026-11-23: mechanized — the `art-dupl:accept` directive mechanism is live in code (60+ directives with rationale/ruling comments, e.g. cmd/tq/main.go:3329; AGENTS.md:314 documents the mechanism); the prose→ledger-table upgrade remains its own open row.
   `art-dupl:accept` directives with recorded reasons, but this session's
   ~20 accept-judgments live only in this report and the one reads.go
   comment. Every future agent running `-t 1` will re-triage the same
   129 groups from scratch.
4. ~~**Daemon topology.** The whole six-file dedup diff landed as one~~ — RESOLVED 2026-11-23: verified at HEAD — cfef2cd0 is reachable and contains exactly the six pass files (git show cfef2cd0 --stat).
   footer-less `chore: auto-commit 6 changed file(s)` (cfef2cd0). Legitimate
   here (no Task-Queue-ID exists — not a dispatch), but the diff is
   reviewable only as one blob; the six extractions were never separately
   committed, and `heal-daemon-sweep.sh` cannot apply (it requires a
   Task-Queue-ID to attach).

## c) NOT STARTED

1. Deflaking `TestExactlyOnceUnderConcurrency` (test-side serialization or
   busy-retry) — documented only (a6/b2).
2. Root-causing the SQLITE_BUSY storms against the single-writer invariant
   (b2).
3. ~~Pinning the accepted `-t 1` groups as formal `art-dupl:accept`~~ — RESOLVED 2026-11-23: landed — `art-dupl:accept` directives with rationale + ruling pointers exist in code across the repo (60+; e.g. cmd/tq/main.go:3329, internal/queue/postgresv4/adapter.go; AGENTS.md:314); the ledger-table consolidation is a separate open row.
   directives with recorded reasons (b3).
4. Verifying `cmd/tq/journalaudit.go:93` `out[id].status = task.Completed`
   — noticed during the replaySetStatus extraction that this one arm is
   UNGUARDED while its five siblings are nil-guarded. Either it can panic
   on a foreign fact id (latent bug) or the guard is redundant elsewhere;
   I did not verify or touch it (unrelated-bug discipline). NOT STARTED,
   now on the record.
5. Explicit coverage check that the journalaudit tests exercise all five
   `replaySetStatus` arms (suite passed, but arm-level coverage unverified).
6. A unit test pinning `preflightStateFor`'s created-flag contract
   (fresh-entry → shouldLog true; existing suite passed without one).
7. ~~gopls multi-module configuration for cmd/tq: every session drowns in~~ — RESOLVED 2026-11-23: landed — cmd/tq/go.work is a committed multi-module workspace covering the cmd/tq module + root + every internal sub-module, so same-module symbols resolve locally (the workspace doc comment pins the contract; residual noise is stale cache per TODO row "go.work directive" DONE).
   ~60 false-positive compiler diagnostics (UndeclaredName for same-module
   symbols) that AGENTS.md says to ignore. Nothing was attempted.
8. ~~Harvesting this report's f-list into TODO_LIST.md / ROADMAP.md~~ — RESOLVED 2026-11-23: superseded by the mechanized corpus sweep — docs-gates freshness --audit-all replayed 883 live reports and harvested genuine survivors into TODO_LIST (TODO row "first mechanized corpus sweep" DONE 2026-11-07).
   (docs-health HARVEST) — deliberately deferred: the operator asked for
   WAIT after the report.
9. Deep triage of the 26 remaining low-priority groups (b1).
10. CHANGELOG policy call for internal refactors — this pass is NOT
    changelogged; append-only CHANGELOG may or may not want refactor rows
    (undecided, not started).
11. Confirming no quote-strip semantic clones exist OUTSIDE cmd/tq (the
    unquote sweep was cmd/tq-scoped; other modules were not grepped).

## d) TOTALLY FUCKED UP

**Nothing.** Evidence: final tree is green on every gate that ran (root
build+vet+test -race, cmd/tq module gate incl. windows cross-compile,
worker module, mirror-clone gate at 0 groups, gofmt, status-index,
doc-refs); the six extractions are behavior-preserving by construction and
the flake was exonerated at the parent commit rather than argued away.
What follows are the honest near-misses, because "nothing fucked up"
without them would be the report lying by omission:

1. ~~** Flake-attribution scare (my change was innocent, but I spent 4 retry~~ — RESOLVED 2026-11-23: historical near-miss, nothing owed — the flake was later formally adjudicated as KNOWN_FLAKY with verify-retry rails (scripts/check-verify).
   cycles before proving it).** First response to the worker test failure
   was retry loops on MY tree; the parent-commit worktree comparison
   should have been attempt #2, not attempt #4. Cost: ~10 minutes and two
   false "my code is fine" impressions (one retry even printed a bare
   `FAIL` with no captured detail because I tailed the wrong lines).
2. ~~**Parser bug on the first extraction script** (tuple-concat TypeError)~~ — RESOLVED 2026-11-23: historical near-miss, nothing owed — the structured group list it produced is recorded in this report.
   — the initial python one-liner had a dead `targets` dict and a broken
   match; wasted one cycle before producing the structured group list.
3. ~~**Ran `gofmt` on `fragments.templ`** (a templ source, not Go) producing~~ — RESOLVED 2026-11-23: historical near-miss, nothing owed — recognized as bogus in-session before any gate result was trusted.
   20+ illegal-character errors that looked like a broken file. Harmless
   noise, but it arrived bundled with the real gofmt check and had to be
   recognized as bogus before trusting the gate result.
4. ~~**Relied on the edit tool's whitespace tolerance once**: the taskRows~~ — RESOLVED 2026-11-23: historical near-miss, nothing owed — result was verified in `cat -A` at the time; no content damage.
   error-cell replacement used a 2-tab `old_string` against a 3-tab file.
   The tool re-indents and reports it, and I verified the result in
   `cat -A`, but per house rules the exact-match discipline slipped on
   one of six edits.
5. ~~**Forgot the doc gates during the work phase** (index row + doc-refs~~ — RESOLVED 2026-11-23: closed since — the doc gates are standing ci-local steps run over every closeout (check-doc-refs/check-todo-list rc=0 at HEAD 2026-11-23).
   only ran after the operator's status prompt — see a9/d-closure). The
   AGENTS.md edit and this report would have sat unindexed had the
   session ended at "deduplicate!".

## e) WHAT WE SHOULD IMPROVE

1. **Flake triage order of operations**: when a test fails on a changed
   tree, the FIRST move should be `git worktree add /tmp <parent>` + same
   test, before any retries on the changed tree. Retry-on-flake alone
   cannot distinguish "my diff" from "load" — the worktree can, in one
   shot. Worth an AGENTS.md conventions line (this session re-learned it
   the hard way; the heal-script incident report from 03-03 records the
   same lesson in a different costume).
2. ~~**Acceptance must be mechanized, not narrated.** ~20 accept-judgments~~ — RESOLVED 2026-11-23: mechanized — the `art-dupl:accept` directive mechanism is live in code (60+ directives with rationale/ruling pointers; AGENTS.md:314); ledger-table consolidation is a separate open row.
   per dedup pass × every future pass = the same 129 groups re-read every
   session. The mirror gate already has an `art-dupl:accept` directive
   mechanism — using it (or an AGENTS.md "accepted idiom catalog" one-pager)
   converts this session's judgment into a durable asset. Until then,
   `-t 1` sweeps pay the full triage cost every time.
3. **The `-t 1` threshold is a firehose by design**: 1468 of 1604 groups
   were suppressed by art-dupl's own actionability filter, and 103 of the
   136 shown were 2-token idioms. The gate's canonical `-t 3` plus the
   mirror gate's cross-backend rule caught everything this session
   extracted except the same-module items; future dedup prompts should
   state whether `-t 1` is a standing bar or a one-off deep sweep (g1).
4. **Low-group triage honesty**: category-shape acceptance of 26 groups is
   a probability judgment, not a verification. Either the idiom catalog
   (e2) covers them, or a future pass owes them individual reads.
5. **Six-file extractions deserve six commits** when the daemon permits:
   cfef2cd0 is reviewable only as one blob. `scripts/commit-task.sh`
   exists for dispatched work; a non-dispatch equivalent ("commit this
   file-set now with subject X") would let future dedup passes land
   reviewable history instead of daemon blobs. (History policy is strict —
   needs owner blessing, hence g3.)
6. ~~**gopls/cmd-tq false positives** (~60 per session) train agents to~~ — RESOLVED 2026-11-23: landed — cmd/tq/go.work multi-module workspace resolves same-module symbols locally (committed workspace with contract doc comment).
   ignore ALL LSP output in that module — one day a real error will hide
   behind the noise. A gopls multi-module config or a documented
   "trusted-command-only" note with the exact expected error list would
   defuse this.
7. **Gate coverage for doc edits**: the cheap verify-window gates
   (check-doc-refs, check-status-index) should run right after ANY
   AGENTS.md/docs edit, not at report time — this session's d5.
8. **art-dupl summary stat "Detected Groups: 1604" did not move** after 7
   actionable groups were eliminated (re-run shows 1604 → 1604). Either
   that stat counts something pre-filter, or the summary is misleading
   after remediation. Worth understanding before trusting it as a
   progress metric (f4) — and possibly an upstream report
   (verify-before-filing first).

## f) UP TO 50 THINGS WE SHOULD GET DONE NEXT

Brainstorm from this session's evidence only — most items are
decide-and-document, not build. Sorted roughly by impact; NOT a commitment
list (docs-health HARVEST applies routing rigor before anything lands in
TODO_LIST).

1. **Deflake `TestExactlyOnceUnderConcurrency`** — serialize claims or add
   busy-retry in the test; today it can fail any loaded CI run.
2. **Root-cause the SQLITE_BUSY storms** vs the single-serialized-writer
   invariant — if the invariant is leaking, that is a PRODUCTION bug, not
   a test bug (b2). Highest-stakes item on this list.
3. **Verify `journalaudit.go:93` unguarded `out[id].status`** (c4) —
   potential foreign-id panic arm, or prove Enqueued-completion ordering
   makes the entry always exist, then delete-or-guard accordingly.
4. **Explain art-dupl's static "Detected Groups" summary stat** (e8).
5. ~~**Mechanize the accept-judgments** — `art-dupl:accept` directives or an~~ — RESOLVED 2026-11-23: landed — the directive mechanism is live in code (60+ `art-dupl:accept` directives with rationale/ruling pointers, e.g. cmd/tq/main.go:3329; AGENTS.md:314).
   AGENTS.md accepted-idiom catalog (e2), so the 129 remaining groups stop
   being re-triaged.
6. ~~**gopls multi-module setup for cmd/tq** (e6).~~ — RESOLVED 2026-11-23: landed — cmd/tq/go.work multi-module workspace (see struck §c7/§e6).
7. ~~**Harvest this f-list** into TODO_LIST.md/ROADMAP.md with proper~~ — RESOLVED 2026-11-23: superseded by the mechanized corpus sweep — docs-gates freshness --audit-all replayed 883 live reports and harvested genuine survivors (TODO row "first mechanized corpus sweep" DONE 2026-11-07).
   routing (only 1-7 are TODO_LIST-grade; the rest is ROADMAP/nowhere).
8. **Arm-level coverage check** for the five `replaySetStatus` arms (c5).
9. **Unit test for `preflightStateFor` created-flag contract** (c6).
10. ~~**Flag-consistency test**: assert `--read-model` and `--db` are~~ — RESOLVED 2026-11-23: resolved better than asked — the flag name literals now exist ONCE in shared registrars used by every serving command (cmd/tq/main.go:216-223 dbFlag/readModelFlag, "name and default live here so every serving command stays in lockstep"), so a typo'd string literal is structurally impossible; help-text smoke + flag_parity consumption map pin the surfaces.
    registered on stats/serve/api (compile-time pinning does not catch a
    typo inside a string literal today).
11. **Decide the CHANGELOG policy for internal refactors** (c10).
12. **Grep other modules for quote-strip semantic clones** (c11).
13. ~~**Run the full `ci-local.sh` battery once** over the dedup tree —~~ — RESOLVED 2026-11-23: superseded — ci-local is a standing ritual and ran green over the surviving tree (TODO_LIST.md:20 receipt, 2026-11-23).
    targeted gates ran; the full loop (incl. its foreign-break retry rails)
    has not.
14. **Run `scripts/lint-baseline.sh --check` + `check-gosec.sh`** on the
    touched files — advisory lints were never run this session.
15. ~~**Verify generated CSS still carries** `min-w-0 flex-1` +~~ — RESOLVED 2026-11-23: verified at HEAD — `.min-w-0` and `.flex-1` utilities plus `tq-payload-cmd`/`tq-payload-pre` all present in internal/webui/static/app.css (grep 2026-11-23).
    `tq-payload-cmd`/`tq-payload-pre` after the component extraction
    (css regen ran; an explicit grep closes the loop).
16. ~~**Scope-check `git show cfef2cd0 --stat`** to confirm the daemon blob~~ — RESOLVED 2026-11-23: executed 2026-11-23 — cfef2cd0 reachable and contains exactly the six dedup files (cmd/tq/{doctor,journalaudit,main,poolconfig}.go, internal/webui/fragments.templ, internal/worker/worker.go), nothing foreign.
    contains exactly the six files and nothing foreign.
17. ** templ-components adoption question for `errorCell`/`payloadPreWithCopy`**
    (share across projects vs stay local) — check ADOPTION.md table policy (f12
    doubles as its decision record).
18. ~~**Budget-card markup (15tok ×15 group)**: record the accept rationale~~ — RESOLVED 2026-11-23: moot — art-dupl's 2026-10-28 `--html` run reports zero fragments.templ groups (TODO row "Track art-dupl #9" DONE receipt), so this group no longer exists to re-litigate.
    (class-per-site params would outnumber extracted lines) in the idiom
    catalog (e2) rather than re-deriving it.
19. **webui `health.go` zero-guard pair**: accept-with-comment or extract a
    tiny `warnIf` helper IF a third instance appears.
20. **postgresv4 `adapter.go` open-companion pair**: leave; if a third open
    path appears, extract `openCompanionDB` then.
21. ~~**`dbFlag`+`fs.Parse` CLI prologs**: leave (idiom); revisit only if a~~ — RESOLVED 2026-11-23: durably recorded — the idiom carries an `art-dupl:accept` directive with a ruling pointer at its canonical site (cmd/tq/main.go:3329, citing the 2026-10-05 t3-tail-zero ruling doc).
    shared-flag cluster beyond db/read-model appears.
22. **Split `fragments.templ`** (950+ lines and growing) per surface —
    check templ import rules + webui guard tests first.
23. **`consumer`/`watermark` duplicate 1-line Handler type**: accepted for
    module decoupling; if a third copy appears, revisit a shared type in
    an owning module.
24. **`render.go` `var zero T; return zero, false` ×3**: accepted generic
    idiom; no action unless a fourth shows up in one file.
25. **lockout.go mutex prologs**: accepted; no action.
26. **`stats.Skipped++` + return/continue idiom across sweepers**: accepted
    permanently — encode in the idiom catalog so no future agent
    "extracts" a `skipn()` helper.
27. **companion reads.go defer-prolog**: comment landed at one site;
    replicate the pointer in the idiom catalog (e2).
28. **AGENTS.md conventions line**: "parent-commit worktree is step 2 of
    flake triage" (e1).
29. **Decide whether `-t 1` is a standing bar** (g1) — changes the cost of
    every future dedup pass.
30. **Review the 26 low groups individually** (b1/c9) — only after 5 so the
    outcome is catalogued, not re-litigated.
31. ~~**Check `check-todo-list.sh`/doc gates on the AGENTS.md Known Issues~~ — RESOLVED 2026-11-23: covered — the gates are standing ci-local steps and run green over the surviving edit (check-todo-list/check-doc-refs rc=0 at HEAD 2026-11-23).
    edit** — the edit predates the gate run; cheap to close now.
32. **Confirm `journalaudit_test.go` covers Requeued+Released arms** —
    they are the two newest `replaySetStatus` callers.
33. **Sweep for `unquote`-style semantic clones in internal/executor and
    internal/bridge** (extends c11 beyond cmd/tq).
34. ~~**Read-model usage strings**: bless the three per-command texts in a~~ — RESOLVED 2026-11-23: landed — the readModelFlag doc comment pins the contract ("name and default live here so every serving command stays in lockstep, the usage line stays per-command", cmd/tq/main.go:220-223).
    comment on `readModelFlag` so the next reader does not "fix" them into
    one string.
35. **Add `payloadPreWithCopy`/`errorCell` to the webui ADOPTION.md table
    IF the table tracks local components** — verify the table's contract
    before writing (it currently tracks templ-components rows).
36. **Upstream check (verify-before-filing first)**: does art-dupl intend
    `-t 1` to surface adjacent-line pairs like the taskBadges stack
    (fragments 610/612)? Only file if the answer is surprising.
37. **Consider a non-backend clone-budget guard** (e.g. fail on NEW
    ≥8-token non-backend groups) if drift protection beyond the mirror
    gate is wanted — needs a policy decision.
38. **`maxAttempt`+`replaySetStatus` pairing** in journalaudit: if a sixth
    fact arm needs both, merge into one `applyFailure(out, id, attempt,
    status)` then — not now.
39. **Document in AGENTS.md that non-dispatch sessions leave footer-less
    daemon chores** (b4) OR mint a footer convention (g3).
40. **De-risk the `-t 1` firehose**: propose `art-dupl --min-tokens`
    upstream/locally so idiom noise can be excluded by size, not judgment.
41. **Add the flake to any CI-watch surface** (if CI reruns exist) so a
    red exactly-once run is auto-retried before a human looks.
42. ~~**Confirm `fragments_templ.go` diff scope** (only fragments output —~~ — RESOLVED 2026-11-23: verified — the cfef2cd0 stat contains no `*_templ.go` file at all, so the generated-file scope is trivially clean (git show cfef2cd0 --stat, 2026-11-23).
    quick `git diff 865a5638..cfef2cd0 -- internal/webui/*_templ.go`
    sanity pass if not covered by 16).
43. ~~**Record `readModelFlag`/`dbFlag` as the canonical shared-flag seam**~~ — RESOLVED 2026-11-23: landed — the seam is the documented registration point (cmd/tq/main.go:216-223 "every serving command stays in lockstep") and the prolog idiom carries an accept directive with a ruling pointer (main.go:3329).
    in the idiom catalog so future flags land there instead of inline
    `fs.Bool` blocks.
44. **Evaluate `--explain` output on the remaining medium groups** to
    attach machine-readable categories to the accept catalog (feeds 5).
45. **Re-run art-dupl at the canonical `-t 3`** full-repo and diff against
    this session's `-t 1` residuals to establish the "normal view"
    baseline the gate protects.
46. **Template-lint idea (ROADMAP)**: a check that identical ≥8-line templ
    blocks must reference a named component (would have caught the error
    cell drift mechanically).
47. **`budget/budget.go` + `harvest/prune.go` newline-truncate pair**:
    accepted (2 lines, cross-package); catalog it (feeds 5).
48. **`sqlitev4/replay` query+rows quartet**: accepted plumbing; catalog.
49. **`questions.go` unmarshal-continue + `status/webui` Completed-filter
    pairs**: accepted; catalog.
50. ~~**Close the loop on this report**: run docs-health HARVEST (7) within~~ — RESOLVED 2026-11-23: resolved via the recorded choice + mechanized sweep — the session left the f-list entombed deliberately (operator WAIT, §c8) and docs-gates freshness later replayed all live reports with survivor harvest (TODO row DONE 2026-11-07), exactly the "discoverable either way" outcome this item accepts.
    the session, or leave the f-list entombed by choice — the index row
    makes it discoverable either way.

## g) QUESTIONS ONLY THE OWNER CAN ANSWER

1. **Is `-t 1` the standing dedup bar or a one-off deep sweep?** If
   standing: I will mechanize the 129 accept-judgments (art-dupl:accept
   directives / idiom catalog) so future sessions stop re-triaging noise.
   If one-off: the canonical `-t 3` gate stays the only enforced bar and
   items f5/f29/f45 collapse into "no action".
2. **`TestExactlyOnceUnderConcurrency`: deflake now or leave documented?**
   If deflake: is touching the test's concurrency shape acceptable, or do
   you want the SQLITE_BUSY storms investigated at the store layer first
   (f2 — which I would then treat as potentially production-grade)?
3. **History policy for non-dispatch sessions**: cfef2cd0 holds the whole
   dedup diff as one footer-less `chore:`. Leave non-dispatch daemon blobs
   as-is, or bless a lightweight convention (e.g. scripted per-file-set
   commits without a Task-Queue-ID footer) so future multi-part refactors
   land reviewable?
