# Worktree-per-agent — the intra-repo parallelism path

**Date:** 2026-09-12. Provenance: TODO_LIST "Dogfood round" item from the
02:00 self-review §f15 ("claim → dedicated worktree → verify → merge; the
real intra-repo parallelism; needs owner merge-policy input first").
**Status: DESIGN ONLY — nothing here is built.** The gating input is an
owner merge-policy decision (Open Question 1); everything else in this
doc is decidable without it.

## Problem

One task per repo at a time, enforced at three layers:

1. **Store-level claim exclusivity** — `--project-exclusive`
   (`sqlite.WithProjectExclusivity()`, wired at cmd/tq/main.go:311 and
   cmd/tq/main.go:662) makes the CLAIM layer refuse a project that already
   has a task in flight across every pool sharing the DB. Intra-repo
   parallelism is structurally 1.
2. **Dirty-tree preflight** — `assertCleanTree`
   (internal/executor/agent.go:351) refuses to start an agent in a repo
   with uncommitted changes; the worker requeues without attempt burn on a
   backoff ladder (`preflightMaxBackoff`, internal/worker/worker.go:115).
   A human's interactive WIP blocks the pool for the repo's whole dirty
   window.
3. **Machine-wide slot cap** — flock'd slot files
   (internal/executor/agentlock_unix.go) bound total agent processes, but
   say nothing about WHERE they run.

Consequences: a slow repo serializes behind its own tasks; every human
editing session parks the pool (ROADMAP open question "Pool ×
interactive-session coexistence"); and in this repo specifically,
interactive agents + the auto-commit daemon + pool agents all write one
checkout — the clobbered-edit incidents live in ROADMAP's
"Concurrent-writer protocol" question.

## Prior art (already in-repo)

- `scripts/poc/worktree-isolation.sh` (plan D93): proves the mechanics —
  `git worktree add -b tq/agent-work` gives a linked checkout whose
  commits never touch the main HEAD, with the human's WIP intact.
- `scripts/poc/pr-mode.sh`: branch + push + open-PR mechanics (the
  delivery endgame of policy (b) below).

## Pipeline: claim → worktree → verify → merge

1. **Claim — unchanged.** Lease-TTL claims, heartbeats, and the
   requeue ladder stay exactly as they are; they are the safety net when
   worktree creation itself refuses.
2. **Worktree create (new executor preflight step).**
   - Branch `tq/<task-id>` created from the repo's current HEAD (base
     choice: Open Question 2), worktree at a path OUTSIDE the main
     working tree — `TQ_WORKTREE_DIR` or
     `<projects-dir>/.tq-worktrees/<repo>/<task-id>`. Inside the tree the
     new directory would surface as untracked content, tripping
     `assertCleanTree` on the main checkout and confusing harvest scans.
   - Fail fast when the rails are missing from the worktree: uncommitted
     `.crushrc`/`.tq-verify` do NOT propagate to a fresh worktree (a
     worktree is a clean checkout). `tq doctor` already warns about
     untracked rails; worktree mode turns that warning into a hard,
     permanent-failure check.
   - If `.gitmodules` exists: `git submodule update --init` inside the
     worktree (submodules need the explicit step per worktree).
   - Note what does NOT block: the MAIN tree may be dirty. A worktree
     branches from HEAD regardless of WIP, so the human keeps working
     while agents fork from the last commit. This is the property that
     dissolves the dirty-tree parking problem.
3. **Execute — unchanged contract, new cwd.** `runAgent` spawns in the
   worktree: same argv, same session handling, same `Task-Queue-ID`
   footer contract. The close-out turn (`--task-closeout`) MUST resume
   the session in the SAME worktree, so the in-process `closeoutPending`
   registry carries the worktree path alongside repo+session. An executor
   restart still degrades to a full re-run (never a lost close-out) — the
   worktree just makes the re-run land in a fresh checkout.
4. **Verify — runs in the worktree.** `.tq-verify` / payload verify
   executes at the branch tip. This proves the BRANCH, not the
   integration (see Tradeoffs; Open Question 4 decides whether the merge
   point re-verifies).
5. **Merge — policy-owned, machine-executed.** Three candidate policies:
   - (a) tq merges (`--ff-only` or a merge commit) back to the base
     branch once the review verdict approves;
   - (b) push the branch and open a real PR (pr-mode PoC mechanics);
   - (c) leave `tq/<task-id>` branches for the owner to merge by hand.
     Today's dogfood shape is "commit straight to master, review after" —
     policy (a) is the least disruptive (review gates the merge instead of
     trailing it), (b) is the most conservative, (c) is the zero-code
     fallback that still wins isolation. This choice is Open Question 1 and
     blocks implementation, not the rest of the design.
6. **Reap.** `git worktree remove` + branch delete after merge, cancel,
   or dead-letter. A crash between create and reap leaves an orphan
   worktree + branch, so the pool tick runs a reaper sweep:
   worktrees/branches matching the `tq/` prefix with no in-flight task id
   are removed (`git worktree prune` cleans stale admin files). Forensic
   retention: Open Question 7.

## Encoding (facts-first, smallest shape)

No new fact types in v1. The worktree path + branch name ride the
existing completion-fact result detail (`AgentResult` JSON already
carries SessionID/CommitSHA/LogPath — add WorktreeDir/Branch), and
failures already carry `FailureEvidence` tails. The reaper keys off git
metadata (`git worktree list --porcelain` + the `tq/` branch prefix),
not journal state. Revisit a dedicated `task.worktree` fact only if
forensics actually demand it — same bar that produced
`RequeueEvidence`.

## Contract interactions

- **`require_clean` semantics shift.** Today it protects the human's
  tree. In worktree mode the human's tree is untouchable by
  construction; the flag's residual meaning becomes "refuse if the BASE
  commit is not what I expect" (Open Question 2's base-drift concern),
  not "refuse a dirty tree".
- **`--project-exclusive` relaxes.** With worktrees, N tasks of one
  project can safely be in flight; the flag stays as the conservative
  default and worktree mode becomes the opt-in that unlocks concurrency
  (per-repo parallelism cap: Open Question 6). The machine-wide slot cap
  remains the global cost bound and is unchanged.
- **Review flow.** Today the sweeper mints `review:<task-id>` after
  completion and the reviewer reads the repo at HEAD (the work is
  already on master). Under policy (a) the review gates the merge, so
  the review payload must name the branch/commit-range, and the worktree
  must stay alive until the verdict — reaping waits for the review, not
  just completion. Under merge-then-review, today's payload works
  unchanged. Either way the review executor itself needs no worktree
  (reviews and status tasks already run close-out-free).
- **Status tasks.** Docs-only work (TODO_LIST appends, status reports)
  should stay serial on master and exempt from worktrees: one status
  window per project is already dedup-enforced, and exempting avoids
  TODO_LIST.md merge conflicts between concurrent windows. The one
  status-task per project invariant makes this safe.
- **Rate-limit gates.** Unchanged and still correct: gates are keyed per
  repo (`rateLimitGates`) and a worktree shares the repo's `.crushrc`,
  hence its provider. Parallel agents per repo multiply 429 pressure;
  the existing fast-refusal + requeue-without-attempt-burn handles it.
- **Footer forensics.** `tq show --commits` scans the payload repo's git
  log; pre-merge it must also scan the task's branch, or the
  MISSING-FOOTER verdict fires wrongly for work still on `tq/<task-id>`.
- **Auto-commit daemon (this repo).** The daemon commits the MAIN
  checkout only. Worktree work becomes invisible to it — which also
  untangles the daemon-folds-agent-work attribution gap — but the base
  can move while an agent works (daemon commits to master mid-task).
  Merges must tolerate base drift: merge commit or rebase-then-merge at
  the merge point (Open Question 4 territory).
- **Gitignored/untracked state does NOT propagate.** Machine-local
  files (`.env`, `result/`, scratch DBs) are absent in a fresh worktree;
  a verify command that depends on them fails. The repo-side rule that
  falls out — verify must not depend on untracked state — matches the
  existing "Flakes only see git-tracked files" philosophy. Escape hatch
  if a repo genuinely needs seeding: Open Question 5.
- **Nix repos.** A worktree is a full git checkout, so flake builds see
  exactly the tracked files they expect; `vendorHash` is unaffected.
- **Windows.** `git worktree` is portable and the executor's process
  handling is already cross-platform; nothing worktree-specific is
  unix-gated.

## Tradeoffs

| Gain                                                                        | Cost                                                                                                                                                         |
| --------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Intra-repo parallelism N× (store exclusivity relaxable per repo)            | Merge-conflict surface replaces file-collision luck: two agents editing the same file now collide at MERGE time, visibly, instead of at write time, silently |
| Human interactive sessions no longer park the pool                          | One checkout per in-flight task (disk) + worktree admin (admin metadata, reaper, prune)                                                                      |
| Agent work is daemon-invisible → attribution gap shrinks                    | Two new failure modes to reap and diagnose: orphan worktrees, base drift before merge                                                                        |
| Verify moves to a disposable tree (rerun-friendly, throwaway on failure)    | Branch-tip verify proves the branch, not the integration — the merged state may differ from what passed                                                      |
| Review can gate the merge (stronger guarantee than today's trailing review) | Review payload contract grows branch/range; worktree lifetime couples to review completion                                                                   |

## Rollout (smallest correct change)

1. `--worktrees` flag on `tq agent-pool` / `tq worker --agents`, OFF by
   default; executor-level option (the AgentPayload v1 contract stays
   untouched — worktree-ness is an operator deployment choice, not task
   input).
2. Executor preflight branch: create worktree (or fail permanent with
   the rails-not-committed guidance), execute, verify, then merge/reap
   per the decided policy; result detail carries WorktreeDir/Branch.
3. Stub-agent smoke (`scripts/smoke/worktree.sh`): scratch repo, two
   concurrent same-repo tasks, assert both complete, branches merged
   per policy, main checkout untouched, no orphan `tq/*` branches.
4. Dogfood on this repo first (rails are committed; daemon interaction
   is live), then fleet opt-in per repo — a repo opts in by committing
   its rails, which `tq doctor` already screens for.

## Open questions (explicit; ⭐ = owner decision)

1. ⭐ **Merge policy (the gating question):** tq auto-merge on review
   approve (a), PR-mode delivery (b), or leave `tq/*` branches for the
   owner (c)? §f15 flagged this first; nothing implements until it is
   answered.
2. **Branch base:** local HEAD or `origin/master`? Fleet repos accept
   agent pushes to master today, so HEAD can be ahead of or behind the
   remote; base choice determines how often merges rebase.
3. **Review ordering:** review-before-merge (stronger, couples worktree
   lifetime to review verdict, review payload gains branch/range) vs
   merge-then-review (today's shape, zero payload change)?
4. **Integration verify:** re-run `.tq-verify` at the merge point on the
   merged result, or trust the branch-tip run? (Serializing merges makes
   the re-run cheap; skipping it accepts the verify-proves-branch gap.)
5. **Gitignored seeding:** is a per-repo allowlist of files copied into
   fresh worktrees ever justified, or is the rule simply "verify and
   prompts must not depend on untracked state" (fail fast instead)?
6. **Per-repo parallelism cap:** default N worktrees per repo
   (`--repo-parallel name=N` ladder, mirroring `--repo-timeout`), and
   does the budget guard need per-repo awareness at the same time?
7. **Orphan reaping:** pool-tick sweep vs a standalone `tq worktrees
   reap`; how long do dead tasks' worktrees/branches survive for
   forensics before the reaper takes them?
8. **Status-task exemption:** confirm status/review tasks stay on master
   serially (recommended above) or get worktrees for symmetry?
9. **WORKTREE_DIR layout:** under `<projects-dir>/.tq-worktrees/`
   (sibling of the repos it forks, one scan root) vs a per-pool
   `TQ_WORKTREE_DIR` (fast local disk for fleet nodes with network
   projects dirs)?
