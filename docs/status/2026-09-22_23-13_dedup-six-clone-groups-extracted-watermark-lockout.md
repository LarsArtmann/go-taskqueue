# Status Report — Deduplication Session (art-dupl, 2026-09-22)

**Window:** 2026-09-22 ~21:40–23:13 CEST · **Base:** `16d347f` · **Head:** `ba8e92a` (19 daemon commits, 29 files, +700/−824 — net **−124 lines** while adding two packages and one facade symbol)
**Task:** `deduplicate-code` on the user-supplied `art-dupl --type-aware -t 5` report (12 clone groups)
**Method:** read/judge every group → extract/accept per the dedup skill → gates after each step → full battery at the end.

---

## a) FULLY DONE

All six extractions below are complete, wired end-to-end, and verified by the named gates.

1. **`internal/watermark` — the durable sweeper cursor (group 1, the big one).** The identical watermark pump (pending-checkpoint gate, paged Facts reads, checkpoint AFTER each page, eager head bootstrap) existed in FOUR root packages: `internal/review/sweep.go:124-167`, `internal/dlqfix/sweep.go:129-172`, `internal/status/sweep.go:145-188`, `internal/prioritize/sweep.go:186-237`. All four now delegate to `watermark.New` + `Cursor.Sweep` (internal/watermark/watermark.go). Error strings stay byte-identical via the `Domain` config ("review sweep: checkpoint %d: %w" etc.), so log-greps survive. New documented behavior: a handler error aborts the sweep before the page checkpoint (at-least-once) — currently unused by all four callers (handlers return nil), so zero behavior change. Sweeper mutexes: review/dlqfix/status dropped theirs (the cursor serializes); prioritize keeps its own for `bootMinted`.
2. **`executor.sessionUsage` + `deriveUsage` (group 2).** The five `Session*` fields with identical json tags existed on `AgentResult`, `ReviewResult`, `StatusResult`, `PrioritizeResult` (a fifth copy on `derivedOutcome`), with the four-line copy loop pasted at four run sites (agent.go:313-325, review.go:209-219, status.go:186-196, prioritize.go:177-186). Now one embedded `sessionUsage` struct (result.go:13-33) + `deriveUsage` which RETURNS the `derivedOutcome` so agent.go reuses the single git+crush pass (the naive version would have paid derivation twice). Budget token projection json keys unchanged — `internal/budget` suite green (rc=0).
3. **`executor.resolveRepoDir` (group 6).** `AgentExecutor.repoDir` ≅ `DepBumpExecutor.repoDir` (20 lines, only the error prefix differed). Extracted to `resolveRepoDir(domain, projectsDir, repo)` in agent.go:381-405; both methods are one-line delegations; error strings preserved exactly.
4. **`postgres.scanFacts` twin (group 11).** sqlite had a shared `scanFacts(rows *sql.Rows)` (sqlite.go:1896); postgres repeated the Query→Close→scanFactRow loop at FOUR sites (Facts, LastFacts, FactsForTask, FactsSince — postgres.go:1431-1449/1460-1483/1509-1537/1564-1582). Added the mirrored `scanFacts(rows pgx.Rows)` next to `scanFactRow`; all four sites now Query+defer+scanFacts in sqlite's exact compact shape. Both backend suites green.
5. **`internal/lockout` — the shared 3-strikes limiter (group 5).** `httpapi.authRateLimiter` and `webui.writeRateLimiter` were ~90-line twins (struct, strikes type, locked/add/reset, pruneLocked, boundLocked with the subtle LRA-eviction policy). Now one `lockout.Limiter` with `Config{MaxHits, Lockout, IdleKeep, MaxKeys, Now, OnLock}`; webui keeps its `wrap` (recorder + CSRF interplay — exactly one of Add/Reset/nothing fires per request, semantics proven equivalent), httpapi's guard calls Locked/Add/Reset. All five lockout behavior tests ported (webui_test.go:1712/1806, httpapi_test.go TestAuthLockout/IsPerClient/StrikesResetOnSuccess) — all green under the fake clock.
6. **`queue.CountStuckRunning` (group 7, cross-module).** `doctorStuckRunning` (cmd/tq/doctor.go:227-244) ≅ `countStuckRunning` (internal/webui/health.go:228-245) — the expired-lease oracle duplicated across the CLI and the health dashboard. Extracted into the queue contract module (internal/queue/stuck.go) with facade alias in queue/queue.go:66 — facade-parity gate green ("7 facades mirror their internal packages").
7. **devmod shim env-lie fix.** `scripts/lib/cmd-tq-devmod.sh` exported `GOTOOLCHAIN=auto` only AFTER the shim's `go mod tidy` — whose failure is swallowed (`|| true`), so outside a devShell the gate died at build with "updates to go.mod needed". Root-caused via `bash -x`, fixed by exporting before tidy (cmd-tq-devmod.sh:15-19); gate now passes from a pinned-GOTOOLCHAIN shell (ok cmd/tq 8.091s).
8. **Concurrent-drift lint repairs (fix-on-sight, unblocking the shared gate).** Fixed 8 fresh wsl_v5/nlreturn/makezero/golines/modernize findings in budget_test.go:191-193, repo_todo_test.go:82, board_test.go:282, render.go:239/247/253, webui_test.go:933/969, components.go:691-699 (min/max modernize), sqlite.go:1813 + postgres.go:1695 (makezero twin — both backends fixed identically to preserve the mirror).
9. **AGENTS.md** package table: added `internal/watermark` and `internal/lockout` rows; httpapi row now names the shared limiter.
10. **Final battery (all green):** root `build+vet+test -race` rc=0; all 7 sub-modules `GOWORK=off` build/vet/test green; `./scripts/test-cmd-tq.sh` ok; `check-facade-parity.sh` OK; `check-script-syntax.sh` 56/56; `art-dupl` final count **8 groups (from 12)**, all remaining accepted (below).

## b) PARTIALLY DONE

1. **Lint-baseline gate (`scripts/lint-baseline.sh --check`): my footprint is clean, but the GATE is still RED on concurrent-window drift.** Remaining growth rows, none attributable to this session: `root cyclop 9>7, err113 22>21, exhaustive 9>8, gocognit 8>7, tagliatelle 10>6, varnamelen 18>16`, `executor test wsl 6>0 + golines 4>0` (gitscan/prioritize/question/redact _test files — not touched here), `sqlite errcheck 9>7, paralleltest 45>44`, `postgres golines 1>0 (FactsSince signature), varnamelen 34>33`. Attribution evidence: postgres.go was last touched by daemon commits `36242d2/d36d117/9f91fd0` (the 2026-09-21 cache work) AFTER the last baseline repair `9c39220`; sqlite/go.mod work likewise postdates the 2026-09-18 seventh regen. One residual uncertainty, stated honestly: I did NOT per-file verify that NONE of the six root-linter growths live in my new files (watermark/lockout) — I verified my early trio run showed only varnamelen (then fixed), but the later root counts were taken while other agents were still committing. A file-level grep of the findings would close this.
2. **AGENTS.md accuracy:** table rows added, but the Operational-contracts bullet ("Journal-consumer watermarks … Checkpoint AFTER the batch's last accepted fact") and the per-sweeper "cursor dlqfix-sweeper (head-bootstrapped, rewindable)" phrases predate the shared cursor — still TRUE but no longer name the owner (`internal/watermark`). A one-line repoint is owed.
3. **`TestSweepPinsCloseoutReportPaths` flake:** failed ONCE mid-session on the new code (status sweep_test.go:260, empty report path), then 10/10 green on the new code AND 10/10 on the pre-change commit via a `/tmp/gtq-pre` worktree. Classified pre-existing flake with evidence, but NOT root-caused and NOT filed as a TODO row. The failure signature (empty report glob) touches no cursor code path — but "touchs no cursor path" is an argument, not a proof.
4. **JSON key-order shift (wire-visible, semantically inert):** embedding `sessionUsage` at the TOP of the four result types reorders marshalled keys (e.g. StatusResult now emits session keys before `report`). All known consumers are key-based (budget parse, `tq show`, conformance fixtures) and all suites pass — but this is a wire-format ORDER change for facade consumers with no CHANGELOG entry and no byte-compare test asserting the new order.

## c) NOT STARTED (this session)

1. `./scripts/ci-local.sh` full run (the pre-push gate) — I ran its components, not the replicant; master-CI state check also not run.
2. Smokes touching refactored surfaces: `scripts/smoke/webui.sh` (live write-route lockout), `ratelimit-e2e.sh`, `status-loop.sh`, `fullcore.sh` (postgres variant).
3. `nix build` + `nix run .#checks.x86_64-linux.vendor-hash` — `go mod vendor` was run (root builds demanded it), vendor/ is daemon-committed, but the fast vendorHash gate was not re-verified (go.mod/go.sum unchanged, so drift is unlikely — unverified).
4. CHANGELOG entry (append-only policy) for: two new internal packages, `queue.CountStuckRunning` facade surface, wire key-order note.
5. TODO_LIST.md integration: no rows added for the items in §f; the flaky status test has no row; harvest of this report's §f not performed.
6. Direct unit tests for `internal/watermark` (especially the NEW handler-abort-skips-checkpoint path — zero coverage today) and a slim direct test for `internal/lockout` (currently pinned only through both consumers' suites).
7. `check-dead-exports.sh` advisory re-run; `check-doc-refs.sh` + `check-status-index.sh` cheap battery after the AGENTS.md/report edits (index row being added with this report).
8. Release-flow note: `queue.CountStuckRunning` means cmd/tq references an UNRELEASED internal/queue symbol until the next sub-tag cut — in-repo gates pass via the shim, but the next release must pre-cut the internal/queue sub-tag (documented two-phase flow, not exercised here).
9. Deliberately out of scope (decisions, not omissions): migrating the papdashboard answer-poller to `watermark.Cursor` (its bootstrap-at-now differs); deduplicating `DLQFixResult.SessionID` (autopsies derive no usage — embedding the full block would be dishonest modeling).

## d) TOTALLY FUCKED UP (process incidents — all caught by gates, zero damage landed)

1. **sed string-surgery on Go source — twice** (import deletion in review/dlqfix; the `s`→`entry` rename in lockout). The second left `s = &strikes{}` / `s.lockedUntil` undefined and broke the build. This violates the repo's explicit "never patch Go source via shell heredocs or string surgery" rule — codified from prior windows, and I re-committed the sin anyway. Fixed via proper multiedit after viewing.
2. **multiedit partial-apply orphaned code in prioritize/sweep.go:** edit 5 of 5 failed (doc-comment text mismatch), leaving the OLD pump loop dangling after the NEW Sweep function — a guaranteed syntax error I then nearly doubled by writing a follow-up edit whose old_string covered only half the block. Recovered only by viewing the region and deleting the orphan explicitly.
3. **auth.go brace surgery:** my `newWriteRateLimiter` replacement was missing a closing brace; my first "fix" added the wrong nesting (`})\n})\n}`), making it worse before the clean `})}` rewrite. Three compile attempts for one function tail.
4. **`rg -rn ""` typo** dumped a near-repo-wide match list into context — pure self-inflicted noise, and the session's context budget paid for it.
5. **Right answer, wrong order (twice):** placed `sessionUsage` mid-struct in 3 of 4 types (embeddedstructfieldcheck wants embedded fields first), forcing a second pass over all four files; and ran `golangci-lint fmt` on flagged files only AFTER the baseline gate had failed four times — the one-pass sweep (built at the end) should have been the FIRST response to the first baseline failure.

## e) WHAT WE SHOULD IMPROVE

1. **Default to `lsp_replace_symbol` for whole-function/type replacements.** All three broken-edit incidents (d1-d3) were exact-match multiedits on large blocks. Symbol-scoped replacement cannot orphan code or miscount braces.
2. **Never sed/awk-edit Go files** — the rule exists because it keeps being re-learned. Put it in the turn-1 checklist next to the CONTRIBUTING.md read.
3. **When a gate fails N times in a row, switch to the one-pass diagnostic** (my end-of-session counts-vs-baseline sweep) instead of iterating gate→fix→gate. Would have collapsed four background runs into one.
4. **New-package lint pre-check:** before finishing a NEW package, run its linters locally (varnamelen/err113/gocognit bites) — new (module,linter) classes are guaranteed baseline-gate failures when count was 0.
5. **Embedded-field placement is a lint rule here** — any future embedding goes at the TOP of the struct, first try.
6. **Flake attribution A/B worktree pattern worked well** (`git worktree add /tmp/x <base>` + N-run loop both sides) — codify it in AGENTS.md next to the verify-window battery.
7. **Mirror-honesty check:** every sqlite/postgres fix must ask "does the twin need the same edit?" — applied for makezero, should be a reflex.
8. **Report-time index+cheap-gates** (doc-refs, status-index, todo-gate) belong in the same closing step as the report, not "not started".

## f) 50 THINGS TO GET DONE NEXT (brainstorm — HARVEST with routing rigor; most §f items beyond ~15 are ROADMAP fuel)

**Gates & verification**

1. Run `./scripts/ci-local.sh` end-to-end at HEAD (the real pre-push gate).
2. Deliberately regenerate `.golangci-baseline.txt` with policy note "absorbs 2026-09-21/22 concurrent drift rows" (owner ruling — see §g Q1).
3. File-level verify of the six root-linter growths (confirm none live in watermark/lockout — closes §b1's stated uncertainty).
4. `nix build` + `nix run .#checks.x86_64-linux.vendor-hash` after the `go mod vendor` refresh.
5. `scripts/smoke/webui.sh` (live 3-strikes CSRF lockout path through the shared limiter).
6. `scripts/smoke/ratelimit-e2e.sh` + `status-loop.sh` + `dogfood-once.sh` (executor call-site proximity).
7. `scripts/smoke/fullcore.sh` incl. the postgres variant if TQ_TEST_POSTGRES is available.
8. `scripts/check-go-mods.sh` + `go mod verify` (cheap; go.mod untouched but confirm).
9. Re-run `check-dead-exports.sh` (advisory) — confirm no new orphans from the deletions (~90 lines of limiter code, 4 pump loops).
10. Run the cheap battery: `check-doc-refs.sh`, `check-status-index.sh`, `check-todo-list.sh` after report+index land.

**Tests**
11. Unit tests for `watermark.Cursor`: handler-error aborts BEFORE the page checkpoint (the new path, zero coverage).
12. Unit test: watermark bootstrap eagerly persists head even when head=0 (the seq-0-with-row case).
13. Slim direct lockout test file (lockout-at-MaxHits, Reset clears, eviction under maxKeys) so the package isn't pinned only via consumers.
14. Add a wire-order pin: marshal the four result types and assert the exact key order (locks the §b4 decision in).
15. A conformance-side pin for `queue.CountStuckRunning` (both backends return the same stuck count on a seeded expired lease).
16. Root-cause the `TestSweepPinsCloseoutReportPaths` flake (report-glob race under parallel load) and file/close a TODO row.
17. Sweep for OTHER tests poking removed limiter internals (`grep -rn "writeStrikes\|authStrikes\|pruneLocked\|boundLocked"`) — currently zero, keep it that way.

**Docs**
18. CHANGELOG (append-only): two new packages, `queue.CountStuckRunning`, wire key-order note, devmod GOTOOLCHAIN fix.
19. AGENTS.md Operational contracts: repoint the watermark paragraph to `internal/watermark.Cursor` as the owner of checkpoint-after-page.
20. AGENTS.md Known Issues: add the "GOTOOLCHAIN before swallowed tidies" variant of the env-lie lesson.
21. `docs/DOMAIN_LANGUAGE.md`: add "watermark cursor" / "strike limiter" terms if absent.
22. Annotate the sweepers' doc comments that still say "a mutex serializes sweeps" (review/dlqfix/status now serialize via the cursor).
23. Mirror.go contract comment: extend to name `scanFacts` as the newest mirrored pair.
24. Record the `DLQFixResult.SessionID` non-dedup rationale (autopsies derive no usage) where the next agent will look for it.

**Queue / release hygiene**
25. Next release must pre-cut the `internal/queue` sub-tag BEFORE release gates (cmd/tq now references `CountStuckRunning`).
26. Add `queue.CountStuckRunning` to the facade-adopted surface list if FEATURES.md tracks facade symbols.
27. Assess migrating the papdashboard answer-poller to `watermark.Cursor` (bootstrap-at-now delta — needs a `Bootstrap` mode, ADR-lite first).
28. Assess migrating `internal/depsweep` (no cursor today) if it ever grows a journal cursor.
29. Consider `watermark.Cursor` gaining a `Pending()` accessor if `tq watermarks show` wants live (unpersisted) cursors.
30. File the §g Q2 ruling (wire key order) as an ADR-note if the owner wants byte-stability.

**Refactor follow-ons (next windows, not this one)**
31. Regen-cycle: fix the six root-linter growths at their real sites (cyclop/gocognit offenders are concurrent windows').
32. `internal/bridge/cqa` + `depsweep` mint paths likely share more mint-boilerplate with review/dlqfix — art-dupl at -t 4 on those dirs only.
33. `remoteHost` duplicated in webui + httpapi (accepted as below-threshold; revisit if a third surface appears).
34. sqlite `scanFacts` still has an inline Scan block — could mirror postgres's `scanFactRow` split (tightens the twin).
35. `cmd/tq` flag-registry: accepted 5-line boilerplate; revisit only if a third clone group forms around it.
36. Extract the shared "finish result" tail (deriveUsage+LogPath+Marshal+SetResultDetail) IF a fifth paid-turn executor appears — not before (abstraction cost > 6 lines × 3).
37. LSP: try `lsp_replace_symbol` adoption for the NEXT refactor of this scale and compare incident rate (meta-experiment from §e1).

**Tooling / process**
38. Add `golangci-lint run` for NEW packages to the verify-window minimum battery.
39. Codify the A/B worktree flake-attribution recipe in AGENTS.md.
40. Consider a `scripts/lint-diff.sh` helper wrapping the one-pass counts-vs-baseline sweep I built ad hoc.
41. investigate why `art-dupl` reported 9 then 8 groups post-refactor (group-1 residual vanished — threshold artifact or daemon commit; confirm no code vanished).
42. Verify the daemon's 19 chore commits contain EXACTLY the session's 29 files (daemon commit-stat diff against the intended set — the silent-loss-triangle check).
43. Confirm `vendor/` diff in those commits contains ONLY the expected internal-package refresh (no stray module drift).
44. Sweep TODO_LIST.md for rows this session made stale (any dedup/DRY rows → [x] with citation).
45. Run docs-health HARVEST on this report's §f into TODO_LIST/ROADMAP (skill contract — §f must not stay entombed).
46. Check whether any prior status report cites `doctorStuckRunning`/`countStuckRunning`/`authRateLimiter` (dead-SHA-style reference rot for symbols).
47. Next session turn-1: grep `docs/status/` for "internal/lockout|watermark" to catch dispatch collisions on the same files.
48. Evaluate `deriveUsage`'s ctx usage: it takes parent ctx but status/prioritize/review call it AFTER runCtx is done — confirm derivation timeout semantics unchanged (15s window on parent, pre-existing shape).
49. `tq doctor` + `/health` live parity spot-check on the production journal (both surfaces read the same DB; a 30-second manual diff of stuck counts).
50. Retire this session's `/tmp/gtq-pre` worktree reference from any docs (already removed — verify `git worktree list` stays at the pre-existing parity-neg one).

## g) THREE QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **Baseline policy ruling:** the lint-baseline gate is red on ~10 rows of concurrent-window drift (root cyclop/err113/exhaustive/gocognit/tagliatelle/varnamelen, executor test wsl/golines, sqlite errcheck/paralleltest, postgres golines/varnamelen). Do you want a deliberate regen NOW to absorb the 2026-09-21/22 windows (my recommendation — shrink+absorb with a policy note), or should the owning windows repair their own findings first?
2. **Wire key order:** embedding `sessionUsage` moved the session keys to the FRONT of the four result types' marshalled JSON (key SET unchanged; all suites green). Acceptable for external facade consumers, or do you require byte-stable field order (I'd then move the embed to the END of each struct and re-pin)?
3. **The status-sweep flake:** `TestSweepPinsCloseoutReportPaths` failed once, then 20/20 green across old/new code. File it as its own TODO row for root-causing now, or batch it with the existing internal/e2e race-margin row as one "flaky-under-parallel-load" work item?

---

_Gates cited: root `build+vet+test -race` rc=0 (23:09); sub-module loop ALL-MODULES-GREEN; `test-cmd-tq.sh` ok (8.091s / 8.771s); `check-facade-parity.sh` "7 facades mirror"; `check-script-syntax.sh` 56/56; `art-dupl -t 5` 12 → 8 groups; pre-change A/B via worktree at `16d347f` (10/10 vs 10/10). Session file set: 29 files, daemon range `16d347f..ba8e92a`._
