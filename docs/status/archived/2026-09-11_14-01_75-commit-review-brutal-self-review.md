# 75-Commit Review — Brutal Self-Review + Status (session report)

**Session:** 2026-09-11, interactive review session (~13:30–14:01)
**Scope:** review of the last 75 commits (`521c2a1..1c8f078`, 09-10 08:41 → 09-11 13:38) and this session's own work. No unrelated research. Everything below is either verified this session or explicitly marked unverified.

---

## a) FULLY DONE (verified this session)

1. **Full 75-commit classification and diff review.** 19 substantive code commits, 36 `docs(status)` close-outs, 19 daemon auto-commits, 1 GitHub bot. Every substantive code diff read; every daemon auto-commit's file list inspected.
2. **Caught the real-code-hides-in-daemon-commits pattern early** and reviewed the code where it actually landed (dead-pool in `d53fbf7`, rate-limit in `39835a0`/`1a7f3a9`/`2467776`, `expandRepoSpecs` in `2c53c03`/`97ef325`) instead of only the named docs commits.
3. **Security-relevant guard bypass (`b37e941`) traced through live code**, not just its diff: `harvestConfigFromOptions` clears `ProjectsDir` when `--repos` is set (agentpool.go:349-361), payloads stay absolute (harvest.go:394-402), `repoDir` join at agent.go:316 unreachable for absolute repos. Verdict: sound; residual risk (manually-enqueued bare-name payloads + unchecked projectsDir) documented.
4. **Verification at the window tip (73610f0), in an isolated worktree:** root build + vet + full `-race` suite green; all 7 sub-module gates (GOWORK=off build/vet/test) green. Realized mid-session that root `./...` never tests the sub-modules where the rate-limit feature lives — caught it and ran the module loop.
5. **Webui test failure correctly isolated** to a post-window daemon commit (`fed88bb`: concurrent agent's in-flight payload-view work committed red by the daemon), not the reviewed window; proven by testing 73610f0 green in the worktree. Worktree cleaned up.
6. **Re-dispatch loop quantified from primary evidence:** 36/75 commits (48%) are close-out reports; single tasks re-fired up to 5× post-completion (reports self-tally "stale re-dispatch tally 7"); TODO_LIST unchecked items 79 → 91 across the window (measured, see §d).
7. **Lost-signal finding proven:** the "re-fire root cause" item — top next-item in 4+ close-out reports — does not exist anywhere in today's TODO_LIST.md. The report→TODO append mechanism dropped the loop's own most important fix.
8. **Process findings:** daemon commits red/partial state to master (observed live); mixed-concern mega-commit `3426afc` (jsonv2 + dependabot + formatting + docs, 6,840 insertions, zero provenance message); app.css unminified accident recurred (2nd time, reintroduced by 3426afc, fixed d7a0f2e); 7 commits unpushed incl. the entire rate-limit feature (CI-unvalidated).
9. **Dead-pool and rate-limit implementations reviewed line-level:** streak/once-per-streak semantics correct, resolve-on-first-readable correct, `RateLimitError` requeue-without-attempt-burn pinned by tests, `WithoutCloseout()` fixes a latent copylocks hazard.

## b) PARTIALLY DONE (sampled, not exhaustive — my choice, stated honestly)

1. **`docs(status)` report bodies:** ~5 of 36 read; the rest reviewed via subjects and the README.md index rows. Claims like "work shipped in 783f4bf / 9804683 / 3e05dfa / 8b5a6be / b0b2cad / 6ae5b99" were taken from commit messages and **never verified to exist or contain the claimed work**.
2. **Intermediate-commit red-state check:** only the window tip was tested, plus one live post-window observation (`fed88bb`). No bisect-style sampling of daemon code commits — e.g. whether master was red at `39835a0` (code) before `1a7f3a9` (tests) landed is unknown.
3. **ci-local.sh equivalence:** ran its build/vet/race + module-loop steps manually; the smoke and nix steps were NOT run.
4. **Rate-limit documentation coverage:** CHANGELOG (49376e2) and AGENTS.md (bc1e331) entries verified; the FEATURES.md row was NOT checked.

## c) NOT STARTED (admitted gaps of this review)

1. `nix build` / `nix run .#test` — the window touched flake.nix three times (`3426afc`, `a90a956`, formatting) and vendorHash drift is a known runner-vs-local hazard.
2. The repo's own gate scripts: `check-go-mods.sh` (3426afc churned go.mod/go.sum), `check-ghost-archives.sh`, `check-todo-list.sh`, `check-features-roadmap.sh` — none executed.
3. Smoke scripts touched by the window (`dogfood-once.sh`, `multi-repo.sh`) — read as diffs, never executed.
4. Task-Queue-ID footer forensics across the window's commits (the f26 three-ID cautionary tale) — skipped entirely.
5. The `.gitignore` 3-line change inside `110cc8a` — noted, never read.

## d) TOTALLY FUCKED UP (session-owned failures)

1. **A false claim shipped in my own final review report.** I wrote TODO_LIST "grew from ~20" unchecked items to 90. Measured now: **79 → 91**. Direction right, magnitude invented — an unverified assumption written into a verdict. This is exactly the verify-before-claiming class this repo has already burned itself on. The loop-fuel conclusion survives the correction (91 unchecked, +50-per-report appends), but a review that misstates numbers undermines itself.
2. **First verification pass ran against a contaminated tree.** Concurrent agents had live edits (components.go, fragments.templ, untracked payload.go); my first race-suite "webui FAIL" scare and partial results reflected foreign state. Fixed by re-running in a detached worktree — but the correct order was worktree-first from the start whenever the daemon + parallel agents are active.
3. **The in-window advisory-lint baseline growth was missed until after my verdict.** The diagnostics pane shows `agentpool.go:147-191` carrying ~200-char `lll` lines — the dead-pool flag help strings from `d53fbf7`. AGENTS.md says don't add new findings in functions you touch (advisory, but a guideline). My "quality: good" verdict understated this; I noticed it only post-delivery.
4. **Wasted round trips:** one malformed `git show --format='@@'` (invalid pretty-format), plus an unnecessarily hacky `tr/sed` shortstat pipeline. Small, but sloppy.

## e) WHAT WE SHOULD IMPROVE (session-derived)

1. **Never write an unverified number into a verdict** — measure it or omit it. (Own rule, violated once, corrected publicly here.)
2. **Run review gates in a detached worktree from the first command** when concurrent agents are live; the main tree is never a clean measurement surface in this repo.
3. **A commit-window review in this repo must include:** intermediate red-state sampling of daemon code commits, the repo's own gate scripts, smoke execution when smokes changed, `nix build` when flake.nix changed, and reading docs(status) bodies when the window's dominant artifact IS docs(status) reports.
4. **Verify hash cross-references before repeating them** — close-out commits cite daemon hashes as provenance; a reviewer repeating them unverified propagates any rot.
5. **The review's biggest lever is the re-dispatch loop, and the loop's fix already fell through its own documentation gap once** — closing it needs owner involvement (see questions), not another appended TODO item.

## f) Next things to get done (session-derived, ranked by impact; "up to 50" is a brainstorm, not a commitment)

1. **Fix the stale re-dispatch root cause** (the lost top item): dispatch-time or claim-time check "TODO item already `[x]`" → cheap verify-only path, or sweeper ticks the box before the next harvest tick can re-mint.
2. Re-add the "re-dispatch root cause" item to TODO_LIST.md — owner-gated: an unchecked item mints a paid pool dispatch (see question 1).
3. Cap the status-report "50 next items" append — the append is the loop's fuel; require top-N verified-undone only.
4. Status-append dedup: hash-match new items against existing TODO_LIST text before appending (reworded duplicates mint new dedup keys → new dispatches).
5. Push authorization for the 7 local commits — the rate-limit feature has never seen CI (see question 3).
6. Resolve `fed88bb` red master: confirm the payload-view agent is still active; if dead, decide who finishes or rolls back the partial work (see question 2).
7. Run `scripts/smoke/webui.sh`, `status-loop.sh`, `dogfood-once.sh` (stub mode) against the window tip.
8. `nix build` + `nix run .#test` at the window tip (flake.nix touched 3× in-window).
9. Run `scripts/check-go-mods.sh` (go.mod/go.sum churned by 3426afc).
10. Run `check-ghost-archives.sh`, `check-todo-list.sh`, `check-features-roadmap.sh`.
11. Bisect-sample daemon code commits (`39835a0`, `d53fbf7`, `2c53c03`, `3426afc`) for red intermediate master states.
12. Verify the hash cross-references in close-out commits (b0b2cad, 6ae5b99, 783f4bf, 9804683, 3e05dfa, 8b5a6be) exist and contain the claimed work.
13. Add the rate-limit FEATURES.md row (CHANGELOG + AGENTS verified; FEATURES gap).
14. Read the remaining ~31 docs(status) bodies from the window for claim-vs-reality drift.
15. Wrap the dead-pool flag help strings (agentpool.go:147-191) to clear the new `lll` advisory lines.
16. `WithoutCloseout()` drift guard: note on the AgentExecutor struct that new fields must ride the clone, or replace the manual copy with a constructor-based clone.
17. Document the guard-bypass residual (bare-name payload + unchecked projectsDir → agent.go:316 join) in AGENTS.md store invariants, or refuse risky projectsDir values in the executor when repos are all-absolute.
18. Read the `.gitignore` change inside `110cc8a` (3 lines, unread).
19. Verify PapDashboard accepts `alert.resolved` without `severity` (NotifyDeadPool resolve payload omits it — untested contract detail).
20. Test/document `deadPoolDetector.observe`'s `res.Repos == 0` early-return (streak not reset) semantics.
21. Add a line-count (or single-line) gate for `internal/webui/static/app.css` — the unminified accident has now happened twice.
22. Quantify money burned on stale re-dispatches from the journal (verification-only windows × per-turn cost) — makes the loop's cost concrete for the owner.
23. Commit-message hygiene gate: conventional-commit prefix check for agent work-turn commits (`b37e941` class had none).
24. Cap docs(status) commit subject length (several exceed 200 chars; `git log --oneline` readability suffers).
25. dependabot.yml: add labels/owner routing; sanity-check all 8 module directories resolve.
26. Review the payload-view webui change once it's green — currently the biggest unreviewed code on master.
27. Task-Queue-ID footer forensics over the window's commits (f26 three-ID class).
28. Owner fix still owed per AGENTS.md: `Environment=GOEXPERIMENT=jsonv2` on the NixOS tq-agent-pool module (pool verify env gap).
29. `tq pool-health` (existing TODO item; the dead-pool incident class makes it concrete).
30. Consider a merge-queue/green-master policy discussion: the daemon will keep committing red intermediates; decide if that's acceptable or needs gating.
31. Consider daemon policy: skip auto-commit while a test suite is red in the tree (process discussion, owner call).
32. Prove the dedup-key invariant actually prevents re-enqueue for reworded-but-equivalent appends (hash = repo + text; rewording defeats it by design — confirm intended).
33. Audit whether `fed88bb`-class partial commits can be pushed (never push without owner auth is enforced socially only).

## g) Questions I cannot figure out myself

1. **May I re-add the "re-dispatch root cause" fix to TODO_LIST.md knowing an unchecked item mints a paid agent dispatch the moment the live pool harvests it?** The item was the loop's top next-work in 4+ reports and then got lost; re-adding it spends your money immediately. Owner call, not mine.
2. **Is the payload-view agent (whose partial work the daemon committed red as `fed88bb`) still active?** I cannot distinguish "mid-flight, will go green soon" from "died mid-session, master stays red" from inside this session — and I must not touch a concurrent agent's tree. If it's dead, who finishes or rolls it back?
3. **Do you authorize pushing the 7 local commits (the rate-limit feature + docs) so CI validates them?** Push is owner-gated by policy; until then the feature's only validation is this session's local gates.

---

_Session artifacts: review executed at window tip 73610f0 (worktree-verified); this report indexed via check-status-index.sh. Format note: repo convention (and the explicit request) is Markdown — the status-report skill's HTML default was deliberately overridden._
