# Done-Prompt Window Status Report 2026-10-14 — 7-task harvest window (2026-10-13 02:08 → 03:30)

- **Written:** 2026-10-14 03:46 CEST (clock verified via `date` before naming this file)
- **Task:** 000001a1380e9c4330c1dcba6e6400000000
- **Window:** seven completed agent tasks spanning check-verify hardening, heal-script rail verification, journal fast-refuse parks, CHANGELOG O14 hygiene, ci-local polish, and a scripts-wide silent-retry audit. Every claim below was re-verified against HEAD (code greps, closeout reports, git log) on 2026-10-14.
- **Method:** each task's closeout report read FIRST; code claims spot-verified at HEAD (`RequeueClassRateLimitGate` at internal/queue/queue.go:396 + facade queue/queue.go:64; `healed on retry poll` in ci-local.sh:87 pinned in check-transient-retry.sh:97; check-verify.sh carries 12 `is load-flaky` anchors; check-ci.sh:73 `@tsv` single-call rework; FastRefused in internal/executor/agent.go:645).

## a) FULLY DONE

1. **Bidirectional KNOWN_FLAKY pin** (`5f67636e`, task …91d427). check-verify.sh fails when a `KNOWN_FLAKY` entry lacks an AGENTS.md "is load-flaky" mention, plus an empty-list guard. The gate caught a real gap on first run (`TestSelfManagingLoop` wording). Red-path spot-checks run and reverted; gate green.
2. **heal-daemon-sweep.sh below-foreign-ID shape: refusal is BY DESIGN** (`221e432e`, task …32a424). Empirically reproduced the 01-54 shape in a scratch repo: any rewrite below a foreign-ID commit cascades into that commit's SHA, so healing is structurally impossible. Documented in the script header with escape hatches; pinned in `--self-test` (20 checks green). This RESOLVES the 02-08 window's §g1 question (their `--from` evidence + this session's repro agree).
3. **AGENTS.md daemon-race rationale strengthened** (`ce0967e1`): during multi-step git surgery the "anything" in stage-commit-first is literal — modified-unstaged AND untracked files are daemon-food; tree must be CLEAN between steps.
4. **O14 `[Unreleased]` rollout audit** (task …32a424): 6 internal-only rows moved to a new `### Internal` subsection; 4 missing BEHAVIOR CHANGE flags added (webui CSP, owner-session park default, `--done-preflight` default flip, GitLogScanner attribution); `release.sh` cut verified compatible. Follow-ups minted (rows 791–793).
5. **Failure-at-TEST-stage genuine-red pin** (`251f9250`, task …32b67b): shim passes build+vet, fails `go test` non-signature; pin asserts rc=1, exactly ONE go-test invocation, zero retry lines. Plus **empty-extraction self-test** for pin 6 (`722010d8`): a reworded Known Issues bullet can no longer silently unbind the bidirectional pin; gate still sub-second (~0.07s).
6. **09-15 archive sweep** (`8d6b7a67`): the row's premise was stale (2 of 3 named reports already archived); the 02-28 retro closeout annotated across all 32 items, `check-rows` COMPLETE, archived. Dispositions include 3 NEW rows minted (fast-refuse journaling — since landed; clock-first naming — since landed; check-status-index ownership).
7. **Journal fast-refuse parks** (`93acad87`, task …32c8c2): `executor.RateLimitError.FastRefused` + `queue.RequeueClassRateLimitGate` const ("rate-limit-gate") + facade re-export; worker picks the gate class so the journal now distinguishes sibling gate-parks from genuine 429s; readers wired (poolSkipClasses, journalaudit); pinned by `TestFastRefusedGateRequeueClass`. Verified at HEAD.
8. **Clock-first naming ritual** (`97ba6b2e`): AGENTS.md convention bullet (run `date` BEFORE composing `<ts>_<slug>` filenames) citing the 02-30 filename lie.
9. **check-status-index.sh `--mine` mode** (`0177a0ad`): scopes the report loop to session-touched reports; default sweep unchanged; negative-tested.
10. **rate-limit-gate const sweep verdict: BLOCKED-by-design** (task …32f1f6): tag archaeology proved no pushed internal/queue release carries the const (v0.3.4 predates it); cmd/tq floor cannot reach it; row 22 annotated BLOCKED (owner push button). Fix-on-sight: the missing CHANGELOG `[Unreleased]/Added` entry landed (`7a132b84`) — verified present at HEAD.
11. **ci-local transient-retry polish** (`b942c668`, task …32fb21): "OK: … healed on retry poll N." line + self-test pin; AGENTS.md `TRANSIENT_MAX_POLLS=0` escape-hatch note; unwrapped check-ci rationale comment. Verified at HEAD.
12. **check-script-syntax nix-fetch separation** (`7a0dde80`): quiet separate `nix build nixpkgs#shellcheck` prefetch; nix fully out of the capture path; gate then caught a REAL SC1087 in check-verify.sh (fixed same commit).
13. **closeoutPending lifetime audit + worktree-reaper fix** (`27c21de2`/`e7587106`): in-memory residue ruled bounded and harmless; the OBSERVABLE leak (cancelled tasks' worktrees pinned until restart via WorktreeInUse) fixed — `worktreeInFlight` lets terminal store status win, pinned by `TestWorktreeInFlightTerminalStatusWins`.
14. **Silent retry/absorption audit** (`d8b0a464`, task …37f7cb): every retry/poll loop under scripts/ classified loud/silent; three silent sites fixed (webui-screenshots server probe WARN; papdashboard/questions stub socket waits now FAIL; questions worker-death WARN). Verified by reading shipped bytes; syntax-gated, not smoke-executed (§d caveat).
15. **check-ci.sh rework** (`6376d90a`): ONE `gh --jq @tsv` call replaces four python subprocesses; local HEAD printed in FAIL line; ancestry-precise predates/diverged wording via `git merge-base --is-ancestor`; self-test green; exercised LIVE against the actually-red master run f96dfb986.
16. All 7 closeout reports written and indexed; batch TODO rows ticked with evidence (`e8ba755c`, `557b3801`, `f09673d6`); every batch commit carries its footer (several recovered via heal-daemon-sweep, all HEAL OK).

## b) PARTIALLY DONE

1. **cmd/tq rate-limit-gate literal→const swap** (row 22): worked to its honest terminal state (verified blocked, annotated); the code change is physically impossible until an owner-gated queue release carries the const. 03-51 §e proposes mint-time precondition probing for "at-next-release" rows — good idea, not landed.
2. **Pin-numbering drift in check-verify.sh**: header says pins 5/6, inline comments Pin 6/Pin 7, closeouts say 6/7 — three coexisting schemes. Known and minted (row 196), NOT fixed in this window.
3. **Audit scope**: the silent-retry audit covers scripts/ only; `.github/workflows/` and Go-source retry paths were not audited (03-30 §b).
4. **09-16 archive continuation**: two live 09-16 reports remain; deliberately stopped after the 09-15 sweep to keep sessions scoped (weekly cadence per O13).
5. **Verification depth honesty**: the 03-30 smoke fixes and the check-ci rework are syntax-gated, not execution-gated (§f8 minted); the 02-54 window skipped the ROOT battery for root-parsed TODO edits (row 31 minted).

## c) NOT STARTED

- **cmd/tq const swap** (row 22, BLOCKED on owner release) and the queue v0.3.5 release cut itself — both owner-gated.
- **108 dangling-SHA re-resolve** (row 30) and the **archive-sweep backlog** (live index >600 rows vs 100 threshold) — pre-existing rows, untouched by this window despite the index growing by 7 closeout reports.
- **Pin-6 literal-marker redundancy audit** (02-08 §f5), **shim-boilerplate extraction** (§f6), **shellcheck-for-gates** (§f7) — report-only, not minted, not started.
- **`--mine` rollout decision** (03-38 §f2) and **fixture-rot grep gate** (§f4) — proposed, not started.
- Every BLOCKED row untouched by design (SystemNix deploy, owner pushes, upstream filings — see the BLOCKED cluster in TODO_LIST).

## d) TOTALLY FUCKED UP (regressions, damage, and repeat offenses)

1. **Daemon-sweep races are endemic to this window — five separate sessions were swept mid-edit** (02-08 §d2, 03-38 §d1 ×2, 04-11 §d1 ×2). Every one was healed via `heal-daemon-sweep.sh --from` (HEAL OK each time; the tool's rails held), but the "stage+commit BEFORE running anything" rule was under-applied five times in seven sessions. The behavioral fix keeps not landing; the 03-30 §f5 pre-commit-hook warning candidate and 04-11 §f5 guard candidate are the right escalation — neither is minted yet (minted this report, item 8).
2. **Dead doc-refs gate is a shared red for everyone**: the 03-38 session's own commits created unreachable SHAs (21a6ebf4, a11023e9) cited in its report, ADDING to the pre-existing 108 dangling cites from the 2026-10-12 rebase era. `check-dead-sha-refs.sh` is RED at HEAD for the whole window; nobody healed it. Row 30 owns the bulk heal; the 03-38 session's contribution needs its own fork-record heal.
3. **03-38 broke cmd/tq locally first** (wired cmd/tq to the not-yet-released const before checking the facade floor) and **tested into a foreign panic** (concurrent `ClaimDueTypes` interface change without fixture update) — the emptyStore fixture fix was correct fix-on-sight, but the interface-without-fixture class remains unguarded (03-38 §f4).
4. **Footer placement collision persists**: the 03-30 session's first commit had the footer ABOVE the Assisted-by trailer — the invisible-to-GitLogScanner shape — self-caught and amended, but the dispatch template still contradicts AGENTS.md (footer-last). Every naive agent re-mints this class; the source template is still unfixed (question 2 below).
5. **Repeat process offense**: 02-08 §d1 wrote item 2's edits before committing item 1 (the exact lesson the 01-54 closeout had just recorded); recovered with the strip-and-reapply dance, zero damage this time, but the discipline is being learned by each session from scratch instead of being enforced.
6. **Pre-existing red inherited, not caused**: master CI red at f96dfb986 (govulncheck stdlib CVEs; windows handle-cleanup leak — triaged in 14-09 report), root build gate RED from the d23f4380 vendor/modules.txt skew (row 609). This window's work is not the cause, but every closeout ran against a partially red tree, which weakens "gate green" claims.

## e) WHAT WE SHOULD IMPROVE

1. **Enforce commit-before-gate mechanically, not memorially**: five daemon races in one window is a systemic signal. The pre-commit `git status` dirty-warning guard (03-30 §f5) or a gate-side assertion is the durable fix.
2. **Fix the dispatch commit template at the source** (footer-as-LAST-trailer in `executor.DefaultCloseoutPrompt` or the dispatch prompt text) — §d4's class is minted-per-session otherwise.
3. **Mint-time precondition probing for release-conditional rows**: rows 22/49/650 hang on release timing with no shared record of WHICH release unblocks them (03-51 §e2); a tag-symbol check helper (row 26, minted) plus a mint-time probe would have saved the 03-51 dispatch entirely.
4. **Make audits durable**: the silent-retry loudness classification is a point-in-time grep+read snapshot (03-30 §e5); a grep guard (every poll loop within N lines of a FAIL/WARN) would make the next silent clone a gate event. Same pattern for the interface-without-fixture class (03-38 §f4).
5. **Pin what you rework**: check-ci's @tsv parse and merge-base branch selection shipped with only the classify-function self-test (03-30 §e3); the 04-11 SC1087 escape proved a landed commit can outlive a red gate — pin depth rulings needed (question in the 03-30 §g3).
6. **Index/normalize at creation**: report indexing still is not automatic (02-08 §e: the 01-54 closeout went unindexed; check-status-index `--mine` shipped to fix the class — wire it into agent-local pre-push flows).
7. **Archive-sweep debt compounds**: the live index grew by 7 closeout reports this window against a 100-row threshold it already exceeded by ~500. Weekly cadence per O13 is not keeping up.

## f) NEXT THINGS (top candidates; top 10 minted below, dedup-checked)

1. webui-screenshots: fail the run when a screenshot PNG is trivially small (dead-server capture guard) — the WARN names it, the exit code should too (03-30 §f1).
2. Hermetic pin for check-ci's red-branch selection (same-tree/ancestor-predates/diverged) via a scratch git repo in CHECK_CI_SELF_TEST (03-30 §f2).
3. check-ci.sh: delete the unreachable trailing ok echo (dead since the ok case exits inside the case) (03-30 §f3).
4. Grep guard: every poll/absorption loop in scripts/ must have a FAIL/WARN within its block (makes the d8b0a464 audit durable) (03-30 §f5).
5. Fix the dispatch closeout commit template to emit footer-as-LAST-trailer (03-30 §f6; §d4 class).
6. Run the three touched smokes (papdashboard-e2e, questions-e2e, webui-screenshots) once on a quiet tree to convert syntax-level to execution-level verification (03-30 §f8).
7. Executor-side test-only injection point for parked closeoutPending entries; extend TestWorktreeInFlightTerminalStatusWins to pin the cancelled+parked shape (04-11 §f1).
8. Pre-commit guard warning when `git status` shows modified files while a gate runs (encode commit-before-gate at tool level) (04-11 §f5; §d1 class).
9. Audit pin 3's literal-name markers for redundancy against pin 6's bidirectional extraction — decide keep/drop with a rationale comment (02-08 §f5).
10. WorktreeInUse consumers audit: grep for other gates consulting executor state alongside store state for the terminal-status-wins gap (04-11 §f2).

(High-value non-minted: `--mine` rollout wiring (03-38 §f2); interface-without-fixture grep gate (03-38 §f4); check-verify shim-boilerplate extraction + shellcheck-for-gates (02-08 §f6/§f7); release-conditional-row state sweep (03-51 §f6); facade-parity × CHANGELOG check (03-51 §f5); heal-script "nearest healable span" ergonomics (03-38 §e3, partially covered by existing rows 46/47); CHANGELOG entry for the bidirectional pin visibility ruling (02-08 §f48, folded into O14 audit).)

## g) QUESTIONS (owner only)

1. **Absorbed-wait doctrine**: is the intended rule "absorbed transient → WARN+continue, never-came-up → FAIL" (the 03-30 session's read, shipped inconsistently: screenshots WARN, smokes FAIL), or should all silent-absorption fixes stay WARN-only so CI keeps producing partial artifacts? (03-30 §g1)
2. **Footer placement authority**: the dispatch prompt's commit template puts the Task-Queue-ID footer BEFORE the attribution line; AGENTS.md and the pre-commit hook require it LAST (hook rejects / scanner misses). Which text owns the contract — fix the template, or drop attribution from queue-driven commits? (03-30 §g2)
3. **Release notes for `### Internal`**: should `release.sh`'s release-notes awk FILTER OUT the `### Internal` CHANGELOG subsection from published GitHub release notes (repo-only record), or is publishing verbatim the intent? (02-31 §g1)

## h) BAND DRIFT

`jq`-equivalent journal query (`tq facts -type task.reprioritized`) returns **0 facts** journal-wide — no task.reprioritized events exist, so none were recorded in this window's timespan (2026-10-13 02:00 → 2026-10-14 03:46). **None recorded.** The window's effective prioritization happened via the harvest queue itself (minted-row order), not via priority moves; per ADR-0015 stored priorities and the aging ladder were never touched.
