# Git push "Invalid revision range" — diagnosis, fetch, concurrent-rebase standoff, guarded push

**Scope line.** VERIFIED this session, with artifacts: the push-failure mechanism (read `.git/hooks/pre-push` + `scripts/check-push-timing.sh` in full; hook's `git rev-list "$rsha..$lsha"` failing because `git cat-file -t a84573533…` returned "could not get object info" while `git ls-remote origin refs/heads/master` showed the remote AT that sha); the fetch (`4cb76971..a8457353 master -> master` + 6 new dependabot branches); the divergence (`git rev-list --left-right --count` = `2 9`); the concurrent rebase (`git reflog`: 9 re-picks, committer time 84s before detection; ahead 9 → 10 via the actor's new `69a835e4` Pareto commit); the guarded push (hook output "push-timing summary: 10 commits checked, 0 refused", `a8457353..69a835e4 master -> master`, rc=0); git-town state cleared (`git town continue` rc=0 → "finished successfully"; `git status -sb` = `## master...origin/master`). NOT verified: whether the concurrent session is still active or what it intends; the true root cause of the transient `fatal: stash failed` on my merge attempt (unexplained, moot state-wise); per-pick semantic identity of the 9 rebased commits (evidence = clean rebase picks only); no test battery ran (no Go code touched; this report's own gate runs at commit time). `TQ_RESULT_FILE` unset — not a tq-managed run, no verdict channel.

**Clock**: session ran 2026-11-17 ≈19:50–20:10 +0100 (`date` taken 20:10:32 +0100 before composing this filename).

## Timeline (what actually happened)

1. **~19:50 — failure state at session start.** User's `git sync` (git-town) had left a pending push step; every push died: `fatal: Invalid revision range a84573533…..0c9c96f51→c1a8e7c3…` / `…196f7a894→74fd1950…` then `error: failed to push some refs`. Branch: `master…origin/master [ahead 8→9]`, tree clean.
2. **Diagnosis.** `origin/master` locally = `4cb76971` (valid, 10 commits behind HEAD). `a84573533` — the LEFT side of the failing range — was **absent from the local object store**. The pre-push hook runs `scripts/check-push-timing.sh`, which walks `git rev-list <remote-advertised-sha>..<local-sha>` over the whole pushed range; the remote had moved 2 commits ahead (`test(fuzz): grow nightly seed corpora`, one new seed file each) that we had never fetched → rev-list fatal → hook exit → push aborted *before git even attempted the transfer*. Both failing tips (`0c9c96f5→c1a8e7c3`, `196f7a89→74fd1950`) existed locally; only the remote base was missing.
3. **Fix half 1: `git fetch origin`.** Objects restored, `origin/master` → `a8457353`. Divergence now explicit: remote ahead 2 (the two fuzz-seed commits, each one new file under `internal/harvest/testdata/fuzz/FuzzParseRepo/`, disjoint from our 30 changed files), local ahead 9.
4. **Integration decision.** `git town config`: main branch `master`, **perennial sync strategy: rebase**. But repo precedent `b476a4ca` ("merge: integrate the parallel fuzz-corpus push (7c3bb33d)") resolves *this exact scenario* — parallel fuzz-corpus push — with a **merge**, citing the "no-rebase-of-tagged-lineage doctrine" (merge keeps local history unwritten; tags + sweep baselines stay valid). Cite audit found 5 orphaning-prone cites (`0c9c96f5→c1a8e7c3` ×2 files, `f811bda2→792c43ed` ×3 files, all in `docs/status/`). Chose merge.
5. **Merge attempt → `fatal: stash failed` (rc=128), UNEXPLAINED.** Three wrong-layer hypotheses chased (merge alias, `merge.autostash`, git wrapper) — all negative. Re-run with `GIT_TRACE=1` printed `Already up to date.` — and the reflog explained everything: **a concurrent actor had manually rebased all 9 local commits onto the fetched `a8457353`** (picks committer-dated 84s earlier) and then added `69a835e4` ("docs: backlog Pareto master plan"). My merge was moot; nothing was created by it.
6. **New blocker: push-timing guard false positive.** The rebase re-dated the three footer-less `chore: auto-commit` commits to committer-time-now (age 84s); the guard refuses footer-less daemon chores younger than 600s keyed on **committer** date. Their AUTHOR dates were 14h–2 days old, so the fold window had long closed — pure re-dating artifact. Chose the honest path: background `sleep 480 && git push` (unblocks at 471s), no `--no-verify`.
7. **Push landed:** `push-timing summary: 10 commits checked, 0 refused`, `a8457353..69a835e4 master -> master`.
8. **State flushed:** `git town continue` ran its remaining push + `push --tags` (both "Everything up-to-date", rc=0) → "The previous Git Town command (sync) finished successfully." Repo fully in sync with origin.

## a) FULLY DONE

1. **Push failure root-caused with evidence** — hook mechanics, missing-object proof (`cat-file`), live remote sha (`ls-remote`), stale tracking ref identified. `docs/status` evidence trail above.
2. **Missing remote objects restored** (`git fetch origin`; `origin/master` `4cb76971`→`a8457353`) — this alone fixed the "Invalid revision range" class.
3. **Push landed and verified**: `a8457353..69a835e4`, guard 10 checked / 0 refused.
4. **Stale git-town pending state cleared** — `git town continue` rc=0; status now "finished successfully".
5. **Repo end-state verified in sync** — `## master...origin/master`, no ahead/behind, tree clean.
6. **Guard false positive avoided without bypassing the hook** — waited out the window; the documented `--no-verify` escape was NOT used.
7. **Operation residue checked clean** — no `MERGE_HEAD`, no `rebase-merge`/`rebase-apply` dirs, stash list empty.
8. **Citation blast-radius mapped** — all 5 orphaned sha-cite sites located precisely (see b1).
9. **Footer/attribution audit of the pushed range** — all 10 pushed commits carry NO `Task-Queue-ID` footer (verified by trailer scan), so no queue↔git cross-reference was rewritten by the rebase.
10. **This report + its index row** (`docs/status/README.md`, newest-first block).

## b) PARTIALLY DONE

1. **Post-rebase citation integrity** — orphaned cites found and reported, NOT repaired. Sites: `docs/status/README.md:129` (`0c9c96f5→c1a8e7c3`→`c1a8e7c3`), `:130` (`f811bda2→792c43ed`→`792c43ed`), `2026-11-17_18-31_dns-outage…:6` (`f811bda2→792c43ed`), `2026-11-17_19-40_io-review…:7,51,53` (`0c9c96f5→c1a8e7c3`, `f811bda2→792c43ed`). Deliberate: a concurrent session owns exactly those docs mid-flow (it created `69a835e4` minutes earlier); `check-tick-claims.sh` will WARN until repaired.
2. **Push-timing guard rebase-blindness** — diagnosed, workaround applied (wait), **not fixed**: the guard keys on committer date, which every rebase re-pick resets, so each divergent `git sync` (perennial strategy IS rebase) freezes pushes 10 min on old daemon chores. Not persisted to AGENTS.md/TODO_LIST yet (collision caution during the active session; now recorded HERE).
3. **Merge-vs-rebase strategy tension** — `git-town.sync-perennial-strategy=rebase` vs the `b476a4ca` merge doctrine surfaced with evidence; decision is the owner's. Note: merge would have avoided BOTH today's guard freeze AND the cite orphaning.
4. **§(f) harvest into `TODO_LIST.md`** — not done: the Pareto session restructured TODO_LIST minutes before my write window; user instruction was report-then-wait. Rows below are report-only until harvested.
5. **`fatal: stash failed` root cause** — unexplained (see d2); state self-resolved; no repro attempted.

## c) NOT STARTED

1. **Fork record** (old→new sha map) for the ~19:54 concurrent rebase — the conventions require one whenever cited unpushed shas get rewritten; nothing exists yet (map is recoverable from `git reflog`: `a8be0408→2edb2524`→`2edb2524`, `bcb644e6→9ec4452e`→`9ec4452e`, `70f84e26→10f3e0ab`→`10f3e0ab`, `40468e95→f9a4396e`→(pick), `8e234d0b→93468aac`→`93468aac`, `f811bda2→792c43ed`→`792c43ed`, `0c9c96f5→c1a8e7c3`→`c1a8e7c3`, `196f7a89→74fd1950`→`634674b6`, `b9b25fd3→634674b6`→(picked as parent of `69a835e4`'s predecessor — recover from reflog entries 5-9)).
2. **Guard hardening** (author-date basis + rebased-chore `--self-test` case).
3. **AGENTS.md Known-Issues row** for the re-dating blind spot.
4. **Stash-failed repro/autopsy** on git 2.56.0.
5. **Dependabot triage** — the fetch surfaced 6 open `origin/dependabot/go_modules/*` branches (root, cmd/tq, executor, queue/sqlite, queue/sqlitev4, readmodel); untouched.
6. **Fuzz-corpus parallel-pusher investigation** — two commits, identical subject, pushed from parallel machines (CI?): no dedup/authorship analysis done.

## d) TOTALLY FUCKED UP

Honest verdict: **nothing rose to catastrophic** — no data loss, no force-push, no history rewritten by me, no reverts of others' work, no gate left broken. The genuine failures were diagnostic-discipline failures:

1. **Three wrong-layer hypotheses before the one command that answered it.** After the inexplicable `fatal: stash failed`, I spent ~2 minutes on alias/config/wrapper archaeology instead of running `git reflog` + `git status` FIRST — in a repo whose own AGENTS.md opens with "concurrent agents commit constantly; re-read state". Reflog would have ended the detour in one command.
2. **The unexplained rc=128 was omitted from the final handoff.** I reported the happy path; the one anomaly I could not explain never made it into the message the user actually read. Handoff completeness fail — every unexplained anomaly belongs in the handoff, even when moot.
3. **Under-weighted the concurrency for history-touching moves.** I verified repo state before merge attempt #1 but treated that check as durable 4 minutes later for the retry — in this repo, state checks are single-use. Also my scheduled 8-minute-delayed background push widened a race window with an actor whose intentions I couldn't see (outcome benign: their push would simply no-op).
4. **The daemon's own `b9b25fd3→634674b6` commit 3.5 min old should have read as "session is HOT right now"** — the freshest possible signal that another writer was mid-flow, and I planned a merge + timed push anyway.

## e) WHAT WE SHOULD IMPROVE

1. **Reflog-first doctrine** for any anomalous git behavior: `git reflog` + `git status` before config archaeology. One command, full answer.
2. **Single-use state checks**: in a concurrent-agents repo, re-verify git state IMMEDIATELY before every history-touching command, not once per task.
3. **Handoff completeness rule**: unexplained errors get reported even when the state self-healed.
4. **Guard design fix**: fold-window check should key on AUTHOR date (rebase-stable) — keeps the c7e95833 protection, kills the re-dating false positive; `--check` output should surface author-vs-committer divergence as a hint.
5. **Merge precedent for master divergence** — codify or explicitly reject `b476a4ca`'s doctrine; today's incident is the second data point that merge is strictly cheaper here (no sha orphaning, no guard freeze).
6. **Persist durable findings at discovery** (AGENTS.md/TODO_LIST) when the tree is clean; if collision-risky, at minimum land them in the session report (done here) so harvest has a source.
7. **Mechanize fork records**: `check-tick-claims.sh` could accept a machine-readable old→new map instead of relying on prose memory.
8. **Coordination norm**: pushes are idempotent under concurrency (second push no-ops) — document that a delayed/background push is race-safe-but-slow, and prefer immediate push after integrate.

## f) Up to 50 things to get done next — session-grounded, impact-ordered

Immediate repair (high impact, cheap):
1. Repair orphaned cites in `docs/status/README.md:129-130` (`0c9c96f5→c1a8e7c3`→`c1a8e7c3`, `f811bda2→792c43ed`→`792c43ed`).
2. Repair `2026-11-17_18-31_dns-outage…:6` (`f811bda2→792c43ed`→`792c43ed`).
3. Repair `2026-11-17_19-40_io-review…:7,51,53` (`0c9c96f5→c1a8e7c3`, `f811bda2→792c43ed`).
4. Write the fork record (old→new map, §c1) where `check-tick-claims.sh` consumes it — verify the mechanism's accepted form first.
5. Run `scripts/check-tick-claims.sh` to capture the WARN baseline before/after 1–4.
6. Harvest this report's §(f) into `TODO_LIST.md` once the Pareto session's TODO restructure settles (avoid a collision).

Guard/tooling (high impact, small diff):
7. `check-push-timing.sh`: switch fold-window basis to author date; keep the c7e95833 protection.
8. Add a rebased-old-chore case to the guard's `--self-test` battery (fleet pin convention).
9. `--check` mode: print author-age alongside committer-age + divergence hint.
10. AGENTS.md Known-Issues row: rebase re-dates daemon chores → 10-min push freeze per divergent sync.
11. AGENTS.md line: pushes are idempotent under concurrent sessions.
12. `check-tick-claims.sh`: accept machine-readable old→new fork records.
13. Scratch-repro of `git merge` "fatal: stash failed" during a concurrent rebase (git 2.56.0); if real and reproducible, follow verify-before-filing upstream.

Strategy/decision (owner calls):
14. Decide master divergence policy: keep `git-town.sync-perennial-strategy=rebase` vs codify the merge precedent (`b476a4ca` doctrine). If merge: config change + doc note.
15. Decide cite-repair ownership: this session vs the active Pareto session (drives 1–3 timing).

Queue↔git cross-reference integrity:
16. Confirm `tq facts` git-scan has zero references to any of the 9 pre-rebase shas (they were all footer-less — trailer scan verified — so expected clean; confirm from the journal if a TQ_DB is handy).
17. Spot-verify one rebased pick is semantically identical to its pre-rebase original (`git diff 792c43ed f811bda2→792c43ed` class, per pick).

Fuzz-corpus pipeline (noticed via the incoming commits):
18. Identify WHO pushes the nightly seed corpora to master (author/committer of `c2e7123f`/`a8457353`) — CI bot or a second machine.
19. If CI pushes master directly: policy check — that path bypasses commit-msg/footer rails; document or gate it.
20. Duplicate-subject commits in one day (`test(fuzz): grow nightly seed corpora` ×2, parallel pushers) — make the seed-commit subject carry the seed file or campaign id.
21. Consider a merge-vs-rebase playbook for nightly-corpus pushes specifically (it is the recurring divergence source; `b476a4ca` is the second occurrence).

Dependabot (6 branches surfaced by the fetch):
22. Triage `dependabot/go_modules/minor-and-patch-e0df80be90` (root).
23. Triage `dependabot/go_modules/cmd/tq/minor-and-patch-f168878ca1`.
24. Triage `…internal/executor/minor-and-patch-f168878ca1`.
25. Triage `…internal/queue/sqlite/minor-and-patch-f168878ca1`.
26. Triage `…internal/queue/sqlitev4/minor-and-patch-f168878ca1`.
27. Triage `…internal/readmodel/minor-and-patch-f168878ca1`.
28. Any accepted bump: per-module tidy + vendor sync + `nix build .#checks.x86_64-linux.vendor-hash` ratchet (Known Issues wiring) in the same change.

Docs/report lifecycle:
29. After 1–4 land, mark this report's b1/b2 rows with inline strikethroughs so it can archive on the next sweep.
30. No `[Unreleased]` CHANGELOG row this session (no operator-visible change); add an `### Internal` row only when the guard change (#7) lands.

(30 items — everything above is grounded in what this session directly observed; padding to 50 would add unrelated backlog, which the scope forbids.)

## g) Questions I cannot figure out myself

1. **Who rebased, and who owns the cite repair?** The ~19:54 manual `git rebase origin/master` (9 re-picks) + the `69a835e4` Pareto-plan commit carry the same git identity as every local actor, and intent is invisible from the reflog. If that session is still active and plans to repair the orphaned docs cites, my items 1–5 would collide; if it's done, they're mine to take.
2. **Master divergence policy — rebase or merge?** Keep `git-town.sync-perennial-strategy=rebase` (each divergent sync re-dates daemon chores → 10-min guard freeze + orphaned cites, as today) or codify the `b476a4ca` merge precedent for master (no sha rewrites, no freeze, one merge commit per divergence)? This decides items 7, 12, 14.
3. **Chase or drop the `fatal: stash failed`?** `git merge` failed rc=128 with "stash failed" while a concurrent rebase was mid-flight (git 2.56.0); the state self-resolved. Do you want it reproduced in a scratch repo and possibly filed upstream, or dropped as moot?

## Self-review addendum (brutal questions, answered)

- **Did you lie?** No deliberate or known falsehood. One imprecision risk flagged: "the concurrent actor added `69a835e4` 84s ago" — the 84s was the commit's committer age at check time and the rebase picks' committer age; the *rebase* timestamp and the *Pareto commit* timestamp are distinct events I did not separately pin. Everything else in the handoff traces to a command output quoted above.
- **Ghost systems / split brains?** One live split brain found, not created by me: `git-town.sync-perennial-strategy=rebase` vs the `b476a4ca` merge doctrine — the same "how do we integrate master divergence" concept configured two ways (tool config vs doctrine precedent), and they diverged today. Resolution is question g2.
- **Scope discipline?** Held: no repo-wide sweeps, no TODO harvest, no unrelated research — report-then-wait as instructed.
