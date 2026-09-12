# DLQ-Autopsy Window — Full Status Report (brutal a)–g))

Written: 2026-09-12 08:32
Scope: THIS interactive session only (TODO item → design → full implementation of the
self-fixing dead-letter queue). Concurrent agents' work is mentioned only where it
collided with mine. Per instruction, report format is Markdown at `docs/status/`
(the status-report skill's HTML default is overridden by the explicit user ask).

---

## a) FULLY DONE

1. **Deep research before any design** (the READ/UNDERSTAND/RERESEARCH phase): journal
   fact model incl. `jsontext.Value` details, review + status sweeper patterns,
   executor `Sink`/result contract, sqlite/postgres store internals (`RescueDead`,
   `Cancel`, guarded updates, `RowsAffected` re-checks), `tq dlq`/`tq cancel` CLI
   semantics, pool tick wiring (`mintPass` budget guard), agent-pool flag/config
   plumbing, and the status machine. Every design claim traces to a file I read.
2. **Design doc** `docs/planning/2026-09-12_dlq-autopsy-design.md`: concept diagram,
   contracts (payload/result/dedup), six loop-safety invariants, rejected alternatives
   (agent-side rescue, executor-side disposition, death-epoch dedup keys, autopsying
   sh tasks), open questions, test plan. Deviations found during implementation are
   annotated inline (executor-level dirty default, summary required for both verdicts,
   stats heuristic behavior).
3. **New `Dead → Cancelled` transition** (`internal/task/status.go`) + transition-table
   test rows (legal pair + `Dead → Dead` self-loop refusal). Task sub-module gate green.
4. **`DismissDead` in BOTH stores** (sqlite + postgres), each mirroring its own twin's
   idioms (sqlite: guarded UPDATE + `RowsAffected` re-check; postgres: `FOR UPDATE` +
   status check), with `dismissReasonDetail` helpers in both mirrors, `Store`
   interface doc, sqlite `TestDismissDead` (happy/double/refusal/not-found/reason-in-fact)
   and postgres conformance subtest `dismiss dead cancels with reason`.
5. **`internal/executor/dlqfix.go`**: `TaskTypeDLQFix`, `DLQFixPayload`
   (self-contained: repo, dead_task, work, `FailureEvidence`, last error, attempts,
   model/yolo/require_clean/timeout), `DLQFixResult`, strict `ParseDLQFixResult`
   (fixed|wontfix, case-insensitive, **summary required for both verdicts**),
   `DLQFixExecutor` (payload decode → permanent-on-miss; repo resolution; dirty-capable
   BY DEFAULT with explicit-true opt-back-in to the preflight — deliberate deviation
   from review/status, because a dead agent's partial work IS evidence), autopsy
   prompt (evidence fence, decide-first workflow, {{TASK_ID}} → autopsy's own id via
   runAgent substitution, verbatim quoted work contract).
6. **`internal/dlqfix/sweep.go`**: sweeper mirroring the review pattern exactly —
   store-facts paging, head-bootstrap + eager persist, checkpoint-after-page,
   pending-checkpoint gate; mint on `DeadLettered` (agent-type only — THE loop guard),
   dispose on `Completed` (fixed → `RescueDead` with the dead task's ORIGINAL budget;
   wontfix → `DismissDead` with the summary as recorded reason;
   `dismissed_by=dlqfix-sweeper`); evidence extraction from the dead task's last
   `task.failed` fact; dedup `dlqfix:<dead-id>` forever.
7. **Executor tests** (unix-gated like review's): parser table (8 cases incl.
   wontfix-without-summary, unknown verdict, no line), both-verdicts-complete contract,
   prompt-evidence test that ALSO pins runAgent's {{TASK_ID}} → own-id substitution and
   verbatim quoted footer, dirty-tree-starts + explicit-clean-preflights both ways,
   payload-miss permanence.
8. **Sweeper tests** (real sqlite store): mint-once + payload assertions (work
   verbatim, evidence round-trip, yolo mirror, project/priority inheritance),
   replay-does-not-duplicate (via the `SetWatermark` ops hatch, made deterministic by
   claiming the autopsy first), agent-type scoping (sh/review/status/dlqfix deaths →
   4 skips, no mints), fixed→rescued (fresh attempts, original budget), wontfix→dismissed
   (reason + dismissed_by on the cancelled fact), human-race benignity (manual rescue
   first → skipped-not-crash), foreign-completion immunity.
9. **Store-level tests** as in (4); cmd-level tests: `--dlq-fix` flag plumbing
   (on + default-off) and `cmdDLQ --dismiss` e2e against a scratch store (dismissed
   output line, cancelled status, verified via `captureStdout`).
10. **CLI wiring**: `--dlq-fix` on agent-pool (flag → options → sweeper construction →
    tick hook under `mintPass` → summary log line), banner line, `tq dlq --dismiss
    ID [--reason WHY]` + usage line, `registerAgentExecutors` now registers
    `DLQFixExecutor` on the closeout-free clone (carry parity for `tq worker --agents`
    for free).
11. **Gates**: root build/vet/**full race suite** green (after the a11y fix below);
    all 7 sub-module build+test loops green; `internal/dlqfix` golangci-lint **0
    issues** (advisory classes fixed in new code: cyclop split, slices.Backward,
    varnamelen, wsl, golines); gofmt clean; `check-todo-list.sh` +
    `check-features-roadmap.sh` green.
12. **Smokes**: multi-repo (6/6 drained, two pools), status-loop, dogfood-once
    stub variant — all green (the pool paths the wiring rides).
13. **Docs**: CHANGELOG Unreleased entry; AGENTS.md architecture-table row +
    `dlqfix` payload-contract bullet; FEATURES.md FULLY_FUNCTIONAL row;
    DOMAIN_LANGUAGE.md new terms (Autopsy, Dismiss; Rescue reworded); TODO_LIST
    High-Impact item closed with gate evidence.
14. **Drive-by**: `TestA11yChrome` (red on master from concurrent agent's css rebuild
    in 68b2fde — `prefers-reduced-motion:` gained a space) relaxed to
    whitespace-tolerant regex; generated css untouched.
15. **Verified-safe** (post-implementation checks for this report): prune-stale
    provably cannot cancel pending autopsy tasks (no harvest dedup key → invisible to
    the absent-item sweep, prune.go:176-177).

## b) PARTIALLY DONE

1. **The feature itself**: core loop fully shipped and unit-tested, but **no
   dedicated e2e smoke** (`scripts/smoke/dlq-fix.sh` with a stub agent proving
   seed→mint→autopsy→disposition through the REAL pool wiring). The wiring is covered
   only by unit tests + declarative registration. For a feature whose whole point is
   autonomous loop closure, this is the largest gap.
2. **`tq show` integration — CONFIRMED split brain (see d1)**: the completion-detail
   switch in cmd/tq/main.go (~1573) handles agent/review/status results only; autopsy
   verdicts land in the journal but `tq show` renders nothing for them.
3. **Webui/httpapi rendering** of dlqfix tasks and `DLQFixResult`: NOT looked at
   beyond confirming nothing crashes the build. Generic payload fallback presumably
   applies; unverified.
4. **PapDashboard alert lifecycle**: a wontfix-dismissed task may leave its
   dead-letter alert firing forever (bridge resolves on rescue/completions — dismissal
   semantics unverified). Listed as an open question in the design doc; not resolved.
5. **Budget accounting for rescues**: `RescueDead` revives paid work but is not an
   `Enqueue`, so a rescue (by sweeper verdict or human) bypasses the "every enqueue
   counts" budget guard — the same bypass class the session-close bridge documents.
   Needs an explicit policy decision; currently undocumented behavior.
6. **Windows**: new files not cross-compiled (`GOOS=windows go build` unrun);
   dlqfix_test.go is unix-gated correctly, cmd test uses filepath — probably fine,
   unproven. Next push's test-windows will tell.
7. **ci-local.sh** (the full pre-push aggregate incl. nix + webui-css gate +
   lint-baseline check) was NOT run this session — component gates were run
   individually. The lint-baseline gate is red at HEAD pending triage (pre-existing)
   and my executor err113 additions grow it further.
8. **Deploy-side documentation**: `dlq-fix` is not in `renderPoolConfig`, the systemd
   sample's recommended keys, or SystemNix module docs (consistent with status-every's
   absence from renderPoolConfig, but still undocumented surface).

## c) NOT STARTED

1. `scripts/smoke/dlq-fix.sh` (the e2e smoke above).
2. `tq show`/webui/httpapi rendering for `DLQFixResult` and dlqfix payloads.
3. PapDashboard notify/resolve semantics for dismissals.
4. Review-minting for autopsy fix commits (design open question — autopsy commits
   currently cross-reference via `tq show --commits`, never reviewed).
5. Retroactive autopsies for the ~21 existing dead tasks (needs a watermark rewind =
   owner decision).
6. Budget-counting policy for rescues (b2/e-item).
7. `dlq-fix` in deploy docs/pool.conf/SystemNix.
8. Pinning test for the prune-stale invisibility verified manually in a15.
9. Fuzz corpus for `ParseDLQFixResult` (shares `ResultLine`; a tiny fuzz target is cheap).
10. Argv-contract pin for `DLQFixExecutor` (like `TestAgentExecutorArgvContract` —
    my prompt test logs argv but doesn't pin the no-`-m`-without-Model contract).
11. Rate-limit interplay pin (autopsies ride runAgent so 429s park them; untested).
12. Postgres conformance subtest has NEVER executed against a live PG
    (`TQ_TEST_POSTGRES`-gated; skipped locally) — same as the rest of that battery,
    but my new code path is in that boat.

## d) TOTALLY FUCKED UP

1. **The `tq show` switch.** I READ the completion-detail switch (cmd/tq/main.go,
   agent/review/status cases) during research, built a brand-new result type, and did
   not extend it. Result: a shipped feature whose verdicts are invisible in the
   primary forensics CLI. Classic renderer split brain; a "list every renderer of the
   result type" checklist item would have caught it. Fix is ~6 lines + a test and
   should be the FIRST next action.
2. **No e2e smoke for an autonomy feature.** Everything else was verified at the
   layer I happened to be in; nobody has yet watched the real pool mint an autopsy
   from a real dead task through the real tick. The status-loop smoke was the exact
   template and I didn't copy it.
3. **Test-authoring churn — four red-green cycles** on the sweeper tests: built tests
   against the DOCUMENTED head-bootstrap semantics (twice), then against the DOCUMENTED
   monotonic `SaveWatermark` (rewind needs `SetWatermark`), then a claim-race in the
   foreign-completion scenario. Plus: duplicate `json` import, misused
   `errors.AsType` (non-generic form), a trailing-comma syntax slip in the cmdDLQ
   flag block, and an invalid first `edit` attempt (unviewed file). Each was caught
   fast, but the pattern is "wrote tests before internalizing the documented
   semantics" — the opposite of the session's own research discipline.
4. **`rg -r` misuse twice** replaced match text in grep OUTPUT and I briefly read
   `Testn`/`func n` as a real varnamelen rename-leak in internal/task — one wasted
   research round trip and a false alarm that could have become a bogus "fix".
5. **Scope blur on the drive-by**: fixing TestA11yChrome was justified (2-minute
   on-sight rule) but I edited a test in a package I didn't otherwise touch WITHOUT
   first consulting `check-webui-css.sh` to learn which spelling the gated artifact
   actually produces — I matched the css I saw, not the gate's contract.

## e) WHAT WE SHOULD IMPROVE

1. **Renderer checklist**: any new payload/result type must enumerate its consumers
   (tq show, webui detail, httpapi, stats) as implementation steps, not afterthoughts.
2. **Feature ⇒ smoke pairing**: a new pool flag ships with its smoke script in the
   same window, wired into ci-local, or the window isn't done.
3. **Re-read documented semantics before writing tests** against them
   (head-bootstrap, monotonic watermarks) — both bites were pre-documented.
4. **Budget-class analysis for every new state change**: anything that REVIVES work
   (rescue, requeue, future un-cancel) needs an explicit does-this-count-against-the-
   cap decision, not an accident.
5. **Static sentinels where cheap**: the executor err113 growth was accepted for
   sibling-style consistency, but sentinel-izing the four fixed messages is 10
   minutes and shrinks the baseline instead of growing it.
6. **Cross-compile early**: `GOOS=windows go build` for touched packages before
   claiming gates green.
7. **CLI grep hygiene**: check `rg` flags before blaming the codebase (`-r` incident);
   verify tooling artifacts (gates) rather than eyeballing artifacts (css).
8. **Postgres conformance liveness**: a battery that silently skips is a battery that
   lies; either a CI service container or a loud skip-marker in the summary.

## f) TOP 50 NEXT ITEMS (impact-ordered; owner-gated marked 👤; most beyond #20 are ROADMAP fuel)

**Feature completion**

1. Extend `tq show` completion-detail switch with `TaskTypeDLQFix` → `DLQFixResult` (+ test). Fixes d1.
2. `scripts/smoke/dlq-fix.sh`: stub-agent e2e through real pool wiring (seed dead → mint → fixed + wontfix verdicts → assert rescue/dismiss), wire into ci-local.
3. Webui: dlqfix payload section (owner-approved collapsed conventions) + result rendering on the detail page.
4. httpapi: confirm task-detail result parity for dlqfix (extend if exhaustive).
5. Pin the prune-stale invisibility of autopsy tasks with a test (verified by reading, not pinned).
6. Pin `DLQFixExecutor` argv contract (no `-m` without Model; session/continuation shape) like the agent argv test.
7. Pin rate-limit interplay: a 429 in an autopsy run parks it without burning an attempt.
8. Carry-parity test: registry registers the closeout-free clone for dlqfix.

**Policy 👤**
9. 👤 Budget ruling: do rescued tasks count against `--daily-budget`? (rescue bypasses the enqueue guard).
10. 👤 Enable `--dlq-fix` on the live pool vs scratch burn-in; retroactive autopsies for the ~21 dead tasks via watermark rewind?
11. 👤 PapDashboard: should a wontfix dismissal resolve/annotate the dead-letter alert with the summary?
12. 👤 Review-sweeper: mint reviews for autopsy fix commits? (design open question).
13. 👤 Should autopsying `sh` tasks ever be allowed (currently excluded by type scope)?

**Ops/deploy**
14. `dlq-fix` in deploy/systemd sample recommended keys + SystemNix module docs.
15. `renderPoolConfig`/bootstrapOptions: render `dlq-fix` when owner wants it in installed pool.conf.
16. Full `ci-local.sh` run at HEAD (incl. nix, webui-css gate, check-ci) — the aggregate was not run this session.
17. Windows proof: `GOOS=windows go build` for executor/cmd/dlqfix + watch next push's test-windows.
18. Run the postgres conformance battery against a live PG (my DismissDead subtest included).
19. `tq doctor`: surface `dlqfix-sweeper` watermark/lag among consumer cursors.
20. `tq stats`: autopsies/rescued/dismissed counters (CountFacts pushdown is cheap).

**Consistency/debt noticed this session**
21. sqlite `RescueDead` appends `Enqueued{"rescue":"true"}` vs postgres `Requeued{}` — pre-existing twin divergence; pick one fact shape and align (needs a deliberate contract note, both suites change).
22. sqlite.go `failureDetail` unused func (gopls) — verify + delete.
23. store_test.go `ptrStatus` unused func (gopls) — verify + delete.
24. Lint baseline: triage-or-regen rows grown by dlqfix.go (executor err113) + the pre-existing red rows (BLOCKS push lineage-wide).
25. Run `check-dead-exports.sh` over the new package (advisory; confirm Sweeper/DedupKey/ConsumerKey all imported).
26. Sentinelize the four fixed parse errors in dlqfix.go to shrink err113 instead of growing it.
27. Fuzz target `FuzzParseDLQFixResult` + seed corpus (cheap; shares ResultLine).
28. Sweep dlqFixPrompt's evidence fence assumption (tail containing ``` breaks the fence visually) — document or strip fences from tails.

**Docs**
29. SECURITY.md blast-radius row: autopsies run WRITE-capable agents on dirty trees by design.
30. VERSION-SURFACES: note which sub-module tags must move at next release (task: transition; queue: interface; executor: new files; root: everything).
31. RELEASE.md/CHANGELOG policy check: DismissDead + transition documented as one grouped entry — confirm that matches the changelog policy ruling when it lands.
32. DOMAIN_LANGUAGE: DLQ row now mentions autopsies — cross-link the design doc from the glossary.
33. AGENTS.md smokes list: add the future dlq-fix smoke when it exists.
34. Design doc: close the three open questions as owner rulings land (keep it the canonical record).
35. CONTRIBUTING: document the a)–g) close-out contract for interactive sessions (currently AGENTS-internal lore).

**Hardening ideas**
36. Crash-storm bound: document/prove one sweep pass mints at most N autopsies (one mintPass per tick bounds enqueue; N = dead tasks seen).
37. `--dlq-fix-max-per-tick` cap if an incident ever dead-letters dozens at once.
38. Payload-size guard: dead agent prompts can be huge — consider capping `work` in DLQFixPayload (payload bloat in sqlite rows; unmeasured).
39. Autopsy timeout ladder per repo (reuse harvest's RepoTimeouts idea) for big repos.
40. `tq dlq` listing: annotate rows that already have an autopsy (dedup key exists) so humans don't duplicate the sweep's work.
41. Autopsy memory: feed prior autopsies for the same repo/stage into the prompt (ROADMAP).
42. Verdict `retry` (pure requeue without repo change) as a third disposition (ROADMAP; changes parser + disposition map).
43. Cross-repo root causes: wontfix pointing at a sibling repo → mint a TODO item there (ROADMAP).
44. DLQ weekly digest as a status task (ROADMAP).
45. Session-close bridge interaction: a dead review task from `tq session close` stays human-surface — confirm that's intended when the bridge ships.

**Session hygiene**
46. Re-run the built binary manually against a scratch DB + stub agent (the manual e2e I skipped; #2 automates it).
47. Add `errors.AsType`/generic-idiom examples to the personal checklist (misused once this session).
48. Stop using interactive `rg` flags (`-r`) un-checked; verify surprising grep output against the file before diagnosing.
49. Extract the duplicated seed/finish helpers in dlqfix sweep_test.go into a tiny shared testutil if a third consumer appears.
50. Close-out bookkeeping: this report's §f should be HARVEST-routed (TODO_LIST vs ROADMAP) by the next docs-health pass — items 1–8, 14–25, 29–30 are TODO-grade; 36–45 ROADMAP.

## g) THREE QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **Budget policy for rescues**: `RescueDead` revives paid work but is not an
   `Enqueue`, so neither the sweeper's `fixed` disposition nor a human rescue
   consumes a daily-budget slot. Should rescued tasks count against
   `--daily-budget` (and if so, at mint-time of the autopsy, at rescue-time, or at
   re-claim-time)? This decides whether an autopsy storm can route money past the cap.
2. **Live-pool enablement + retroactivity**: should `--dlq-fix` go live on the
   production dogfood pool now (input flip / pool.conf), or soak behind a scratch-DB
   burn-in first? And do you want the ~21 EXISTING dead tasks retroactively autopsied
   (one-line `tq watermarks set dlqfix-sweeper 0`), or left as human surface?
3. **Dismissal surface**: when an autopsy rules a task `wontfix`, should the
   PapDashboard dead-letter alert for that task be RESOLVED with the autopsy summary
   (today it likely keeps firing), or is DLQ-silence the intended end state?
