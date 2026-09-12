# Required checks proposal — making the CI gates branch-enforced

Status: PROPOSAL for the next release window (written 2026-09-13). Nothing
has been flipped; enabling required checks is an owner-run, admin-only
action. Work item: TODO_LIST.md "Window f20–f24 follow-ups" (harvested from
docs/status/archived/2026-09-10_04-09_scan-jobs-webui-contracts-and-red-master.md,
item f12; re-surfaced by round-13 M127).

## Problem

GitHub-side enforcement of merge/push quality is currently zero. Verified
2026-09-13: `gh api repos/LarsArtmann/go-taskqueue/branches/master/protection`
returns 404 "Branch not protected" — no required checks, no ruleset, nothing.

The consequence is already on record: the 04-09 report's d1 — five DONE
verdicts landed on a master that had been red for ~3 hours (broken
test-windows, release-gates smoke, govulncheck, gosec) without anyone
looking. The guards added since are all LOCAL: `scripts/check-ci.sh` (ci-local
pre-push gate, bypassable with `CI_CHECK=off`) checks the latest master run's
state, and ci-local replicates the gates — but nothing at GitHub stops a merge
or a push that skipped them. Required checks are the platform-side
complement: the repo's own green-traffic assumption becomes explicit and
enforced instead of customary.

## Current job inventory (`.github/workflows/ci.yml`)

| Job             | Today                                                                   | Runner evidence at hand                                                                                     |
| --------------- | ----------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| `test`          | Hard: vet/build/GOOS=windows/race/module gates/go-mods/gofmt + smokes. The release-gates smoke is a STEP inside this job. | Full-batch SUCCESS run 34670475702 (2026-09-12, e0dadb5)                                                    |
| `test-windows`  | Hard (POSIX-only suites drop out via `//go:build unix`; no `-race`)     | Same run 34670475702                                                                                        |
| `test-postgres` | Hard (conformance vs real postgres service)                             | No dedicated green citation at hand — pull a fresh run before touching it in any phase                      |
| `nix`           | Hard (build + flake check)                                              | Green on recent runs (2026-09-11 15-39 report a11)                                                          |
| `govulncheck`   | Hard since round-13 T7 (evidence-gated flip: locally clean root + all sub-modules, runner step-green run 34661978016) | Same evidence                                                                                                |
| `gosec`         | ADVISORY, job-level `continue-on-error: true` — the gate-vs-advisory flip is owner-ruling O5 (still unchecked; O5's recorded recommendation: advisory until one fully-green runner week, then hard). Post-config scan: 0 findings (triage-encoded excludes) | Local post-config scan v2.29.0 = 0 (AGENTS.md round-13 T6)                                                   |

## The mechanical wrinkle: required checks attach to JOB names

GitHub required status checks select check runs by job name. The
release-gates smoke is a step inside `test`, so "require the release-gates
smoke" has two implementations:

- **A (recommended): extract it into its own `release-gates` job.** Checkout +
  setup-go + `./scripts/smoke/release-gates.sh`; the script is already
  runner-hermetic (fixture commit and tag carry `-c user.*` identity since the
  2026-09-10 fix). Cost: one extra runner spin-up per push (~1–2 min incl.
  cached setup-go). Benefit: the required-check name documents intent, the
  smoke's signal is not masked by sibling steps in `test`, and it can flip
  independently.
- **B (rejected): require the whole `test` job.** Makes step one of the
  phased plan the heaviest Linux job and buries smoke regressions in its step
  list — the opposite of the item's "smoke first" intent.

## Phases

**Phase 1 — next release window, first push of the window.**

1. Extract the smoke into a standalone `release-gates` job (mechanical move of
   the existing step; the step inside `test` is deleted to avoid double runs).
2. Flip `continue-on-error`-free as today; require checks `test-windows` +
   `release-gates` on `master` (Settings → Branches → branch protection rule
   for `master` → Require status checks; owner-run, admin-only).
3. Evidence gate before flipping: the new job green on a runner run at HEAD
   — the same flip-with-evidence-not-hope discipline that round-13 T7 applied
   to govulncheck.

Why these two first: both are the checks whose breakage went unnoticed on
master in the 04-09 window, both are cheap-to-repair platform signals, and
neither bundles unrelated gates into the requirement.

**Phase 2 — scan jobs, after phase 1 has bedded in (≥1 week, one window).**

1. `govulncheck`: require it. Already a hard gate at step level; this only
   moves enforcement to the merge point. Needs no new evidence beyond a
   runner-green run at HEAD.
2. `gosec`: require ONLY after O5 rules hard. This proposal deliberately does
   not re-decide the gate-vs-advisory question; it carries O5's recorded
   condition (one fully-green runner week, then hard) forward as the trigger.
   The job is green-by-default since the triage-encoded excludes, so the flip
   is a settings change, not a repair.

**Deliberately deferred (owner question, not a phase):** `test`,
`test-postgres`, `nix`. They are already hard gates and the heaviest jobs;
adding them rides the same mechanism once phases 1–2 prove the workflow. For
this repo's direct-to-master flow their enforcement lives in the ci-local
pre-push ritual until the owner opts in.

## What this does and does not enforce

- Classic branch protection required checks gate PR MERGES ONLY. A direct
  push to `master` is not blocked by them (verified against GitHub docs
  2026-09-13). This repo's primary write path IS direct pushes (owner push
  flow; the pool daemon commits locally and never pushes), so phase 1's
  practical surface is PRs — narrow but real, and it hardens as PR traffic
  grows.
- If push-side enforcement is wanted, GitHub RULESETS can require checks
  against pushes to the branch (the push itself is rejected when the required
  checks have not passed on the pushed commits — same docs check). Proposed
  as a separate owner option; adopting a ruleset is a bigger policy step
  (bypass rules, include/exclude patterns) and should not ride this proposal.
- Admin bypass: unless "Do not allow bypassing the above settings" is set,
  admins can merge past failing checks. For a solo-owner repo this is the
  pragmatic default — the check's value is visibility and the ritual, not a
  lock against the owner. Owner's call at flip time.

## Failure modes and rollback

- A required check that STOPS REPORTING blocks merges forever: job renamed
  (required-check names are job names — renaming a job silently orphans the
  requirement), workflow disabled, runner outage. Mitigations: rename
  discipline when touching ci.yml job names, and the sanity audit below.
- Emergency bypass: temporarily edit the protection rule (owner-run) — a
  conscious act, never a silent one.
- Flaky `test-windows` under runner variance is the likeliest day-to-day
  friction; if it flakes repeatedly, fix the flake — do not relax the
  requirement silently.

## Sanity audit (companion follow-up, not built here)

From the 09-33 report item 24: a small `scripts/check-required-checks.sh`
that diffs the protection rule's actual required contexts (`gh api
.../branches/master/protection`) against the intended set from this doc,
wired into ci-local next to `scripts/check-ci.sh`. Turns required-check drift
from invisible to blocked, and keeps the pool's green-traffic assumption
honest. Scope it as its own TODO when phase 1 lands.

## Owner questions

1. Confirm the phase-1 set (`test-windows` + `release-gates`) and the job
   extraction.
2. O5 verdict on gosec (hard vs advisory) — this proposal inherits it
   verbatim.
3. Classic protection only, or a ruleset for push-side enforcement?
4. Do `test` / `test-postgres` / `nix` join in a later phase?
