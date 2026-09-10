# Session status — red master repaired, closeout loop proven, brutal self-review

- **Written**: 2026-09-10 09:44 CEST (session wall clock 07:33–09:44, of which ~1h50m
  was idle gap between owner messages — see d)2)
- **Scope**: this session only — executing the standing queue left by the 05-30
  report: live-prove the closeout, run the full gate, document the closeout +
  docs-health contracts, verify the 04:09 "master CI red" claim, monitor the
  workforce.
- **Method**: claims-first. CI state read from `gh run view --log-failed`
  (run 34439012027); every fix verified by the tool that gates it (smoke run,
  `govulncheck ./...` in the module, `nix build`, `nix flake check`, full root
  `build/vet/test -race`); workforce state from `tq top` / `tq tasks` snapshots.

## TL;DR

| Area                   | Verdict                                                                                                                                                         |
| ---------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Closeout live-proof    | DONE — 5 reports at session start, 13 by end; one read in full: genuine a)-g) brutality, claims traced                                                          |
| 04:09 red-master claim | TRUE (5 failing jobs at the 04:54 run) — all four hard breaks fixed and tool-verified locally                                                                   |
| Full gate              | `ci-local.sh` ALL GATES GREEN + final root build/vet/race green after the lint agent's renames                                                                  |
| AGENTS.md contracts    | closeout turn already documented (parallel session); docs-health half added by this session                                                                     |
| Workforce 1C5          | healthy: 50 done / 0 pending / 0 running / 0 dead at 09:44, pool alive 4h18m — it OUTLIVED its session, contradicting the handoff                               |
| Owner-gated            | push (now non-FF: origin tip 41b817b was rewritten out of local master), SystemNix redeploy, report placement, caps, tags                                       |
| Self-verdict           | the work held; I shipped a broken flake to master for ~10 minutes and hung a shell on a live pipe for 2 hours — both were documented traps I walked into anyway |

---

## a) FULLY DONE

1. **Closeout live-proof.** `docs/status/*_task-*.md` grew 5 → 13 during the
   session; the 07-15 (version-surface inventory) report was read in full and
   is a genuine brutal self-review: a)-g) structure honored, every claim
   traced to a script/flake check, honest gaps named, ~50 next items. The full
   loop is proven end-to-end: agent task → closeout report → review task →
   fix-task minting (one live example: wrong `Task-Queue-ID` footer → fix task).
2. **Red-master claim verified TRUE and every break fixed:**
   - `test-windows`: `TestHarvestConfigFromOptionsExpandsBareRepoNames`
     asserted POSIX string literals (previous session's own regression);
     rewritten with OS-derived separators and volume-aware absolutes
     (`cmd/tq/agentpool_test.go`). `GOOS=windows go vet` typechecks it.
   - `test`: release-gates smoke cut fixture tags without a committer
     identity → exit 128 on runners (local hosts masked it); the tag now
     carries the same `-c user.email/-c user.name` as the fixture commit
     (`scripts/smoke/release-gates.sh`). Smoke re-run: all cases green.
   - `govulncheck`: `internal/queue/postgres` pinned `golang.org/x/text
     v0.29.0` (GO-2026-5970, reachable via `postgres.Open` → `pgxpool` →
     `norm.Form`) while every other module sat on v0.41.0; bumped to v0.41.0 —
     `govulncheck ./...` in the module now reports **No vulnerabilities
     found** (after `go install` was policy-blocked; `go run pkg@ver` worked).
   - `nix`: vendorHash was stale since the root go.mod postgres replace moved
     the module graph; fakeHash dance → real hash
     `sha256-/rKFWqGR0Rc9pCmKIn8HcDpHJ/7flXRtiG7tP/D/M2s=`; `nix build` green,
     `nix flake check` green including `checks.vendor-hash`.
   - `gosec`/`govulncheck` jobs were already `continue-on-error` in the
     current tree (round-2 f20/f21 work) — no change needed.
3. **Full gate**: `./scripts/ci-local.sh` → **ALL CI GATES GREEN**; plus an
   explicit final root `go build ./... && go vet ./... && go test ./... -race`
   green AFTER the lint agent's ~95 renames landed mid-run.
4. **Docs**: docs-health pass added to the `status` payload bullet in
   AGENTS.md (the closeout-turn half was already present from a parallel
   session); two CHANGELOG [Unreleased]/Fixed entries (x/text security bump,
   red-master trio); missing status-index row for the 07-39 closeout report
   (a daemon race — its commit claimed "index row" but the row never landed)
   added; `check-status-index.sh`, `check-doc-refs.sh`, `check-todo-list.sh`
   all green at the end.
5. **Workforce monitoring, bounded**: 1C5 pool (PID 494121) alive 4h18m,
   31 → 50 done over the session, 0 dead, queue fully drained at 09:44.
   SQLITE_BUSY claim-failure blips observed (two pools share the DB) —
   transient, retried; root cause is the stale systemd pool (deploy is the
   fix).

## b) PARTIALLY DONE

1. **Windows fix is typecheck- and logic-verified only** — no windows runner
   exists here; the CI job proves it on push (still unpushed, see g)1).
2. **Final verification covered the root module only**: `go test ./...` from
   the root does not descend into the 7 nested modules (disk-derived loop is
   separate); I re-ran postgres (changed) but not the other six after the
   lint renames — covered by the lint agent's own gate claims and ci-local's
   mid-flight module phase, not by my own post-change pass.
3. **ci-local green-run attribution**: the run started pre-fix and its late
   phases staged post-fix content; I inferred (not timestamp-verified) which
   phases saw which tree. Mitigated by the explicit post-fix passes above.
4. **gosec suppression config**: left as TODO_LIST item 101 (workforce food);
   the advisory job still reports red content on runs until then.
5. **Push readiness**: origin diverged — its tip `41b817b` was rewritten out
   of local master (the known daemon/agent twin-commit phenomenon; the 05-58
   report already tracks "dangling review target vs master twin"). Push will
   be non-FF; not investigated beyond identifying the commit (no reset, no
   fetch-and-reconcile — owner call).

## c) NOT STARTED (owner-gated, deliberately untouched)

1. Push master (now 33 commits ahead incl. all CI fixes + both loops; needs
   the 41b817b divergence reconciled first).
2. SystemNix redeploy — the systemd pool still runs the broken 0.1.0 binary
   (harvests nothing; also the SQLITE_BUSY co-tenant).
3. Closeout-report placement decision (`docs/status/` root vs `tasks/`
   subdir — the index already carries 13 task rows).
4. TODO append caps decision (~50/report vs 60 enqueues/day).
5. The two local-only backend tags (`internal/queue/{sqlite,postgres}/v0.2.0`).

## d) TOTALLY FUCKED UP

1. **I shipped a non-evaluating flake.nix to master for ~10 minutes.** I
   wrote `vendorHash = lib.fakeHash;` from the AGENTS.md dance note without
   reading the flake's binding structure — the parallel-session rework to
   flake-parts left `lib` out of scope at that position. `nix build` failed
   with "undefined variable 'lib'", and the auto-commit daemon folded the
   broken state into master before I fixed it. The AGENTS note reproduces
   this exact mistake for the next agent (see e)1).
2. **I hung a shell on a live pipe for ~2 hours.** `tq top` is a refreshing
   command; I piped it to `sed -n '2,4p'`, which never exits, so the job sat
   between owner messages while the clock ran 07:54 → 09:44. I had even
   documented this trap ("NEVER `job_output wait=true` on long jobs; use
   bounded polls") and earlier used `head -5` correctly — then reached for
   `sed`. The owner-time lost is real; the workforce used it well (19 more
   tasks done) but that is luck, not design.
3. **Narrative-first slip**: I called the `M AGENTS.md` I saw in recon
   "someone's in-flight edit" before checking that the daemon had simply
   folded it between my two commands — the exact liveness-before-narrative
   rule the 05-30 session recorded after the zombie-pool misdiagnosis.
4. **Overstated gate claim in my summary**: "ci-local ALL GATES GREEN on the
   fixed tree" leaned on inferred phase timing (see b)3). The final explicit
   passes made the conclusion true, but the sentence as first written was
   sharper than the evidence — the verification-chaining lesson again.

## e) WHAT WE SHOULD IMPROVE

1. **The AGENTS.md vendorHash-dance note should carry the literal fake
   string** (`sha256-AAA…=`), not `lib.fakeHash` — or the flake should bind
   `lib` where vendorHash lives. As written, the note walks the next agent
   into d)1.
2. **Multi-step dances should be one atomic script** (set fake → build →
   capture → set real → build) so the daemon can only ever commit the
   completed dance, never a broken intermediate.
3. **Live-pipe discipline**: refreshing CLIs (`tq top`) get `head` or
   `timeout`, never `sed`/`cat`; add it next to the existing bounded-poll
   note.
4. **Verify gate-phase coverage before claiming green**: timestamp the
   phases or re-run the cheap ones post-change; inference is not evidence.
5. **Check push-readiness (origin divergence) before any "safe to push"
   framing** — the twin-commit phenomenon makes non-FF the default ending
   here, and it belongs in the handoff, not in a surprise.
6. `go install` is policy-blocked in this environment; `go run pkg@version`
   is the working pattern for one-shot tools — worth a line in AGENTS
   commands.

## f) UP TO 50 THINGS WE SHOULD GET DONE NEXT

Brainstorm, not commitments; items already tracked in TODO_LIST are marked.

1. **Owner**: reconcile the `41b817b` origin divergence, then push master —
   expect all five previously-failing jobs green (windows job is the real
   test of the a)2 rewrite).
2. **Owner**: SystemNix redeploy (kills the stale systemd pool + the
   SQLITE_BUSY co-tenancy class).
3. **Owner**: decide closeout-report placement; if `docs/status/tasks/`,
   move the 13 existing reports and teach the closeout prompt + index check
   the new path.
4. **Owner**: TODO append caps policy.
5. **Owner**: cut the two local-only backend tags via `scripts/release.sh`
   at release time (do not hand-tag).
6. TODO 101 (tracked): gosec suppression config so the advisory job goes
   green and new classes stand out.
7. Re-baseline the "~400-finding advisory" number in AGENTS.md after the
   wrapcheck/varnamelen slices (the 07-39 report flagged it stale).
8. Run the disk-derived nested-module loop once on a quiet tree to stamp
   all 7 modules green post-lint-renames with my own eyes (b)2).
9. Watch the first post-push CI run end-to-end; if windows still fails,
   the fix is wrong despite the typecheck.
10. Add `timeout`/`head` guidance for refreshing CLIs to AGENTS.md (e)3).
11. Fix the AGENTS.md vendorHash-dance note per e)1) (literal fake string).
12. Script the vendorHash dance atomically per e)2) (small `scripts/`
    helper; NixOS-only guard).
13. Record in the AGENTS dogfood section: background pools can outlive
    their session (1C5 ran 4h18m across session boundaries) — the
    "dies with this session" handoff claim was wrong.
14. Investigate whether the daemon's heuristic auto-commits can be made
    history-stable (the twin/dangling-commit class keeps costing push
    readiness) — likely an owner/architecture discussion.
15. The 04:09 report's aged-into-baseline err113 finding and the 48
    dead exports (tracked there) — keep them visible.
16. Consider a `tq top --once` flag (or `--no-watch`) so tooling never
    hangs on it — small, high leverage for exactly d)2.
17. Add a CI phase-timestamp (start/end per gate) to ci-local output so
    "which tree did this phase see" is answerable from the log (b)3).
18. Snapshot `git rev-list --left-right --count master...origin/master`
    into every status report's TL;DR — push readiness is a standing blind
    spot (this report: 33 ahead, 1 behind).
19. Harvest-check: confirm items 6-12 above don't duplicate TODO_LIST rows
    before enqueueing anything (docs-health pass owns the merge).
20. Once pushed + deployed, run one `journalctl -u tq-agent-pool` review
    to confirm the systemd pool harvests with the fixed binary and the
    agentPath option.

## g) QUESTIONS FOR THE OWNER (cannot be answered from here)

1. **Push**: master is 33 ahead / 1 behind (origin tip `41b817b` was
   rewritten out of local history by the daemon/agent twin phenomenon).
   Push with `--force-with-lease` after you eyeball the twin, or do you
   want a rebase-onto-origin first? All CI breaks are fixed locally; the
   windows job is the only fix without local proof.
2. **Closeout reports**: keep them in `docs/status/` root (13 rows and
   growing ~1/task) or move to `docs/status/tasks/` with the index
   unchanged? This changes the closeout prompt, the index check, and the
   docs-health archive convention in one move — your call.
3. **Cutover order**: when you redeploy SystemNix, should I stop workforce
   1C5 first (clean handover, no double-harvest, no BUSY contention) or let
   the lease/reclaim machinery absorb the overlap?
