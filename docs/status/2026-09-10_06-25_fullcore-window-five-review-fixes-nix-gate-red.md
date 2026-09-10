# Window Status Report — fullcore example + four review-fixes; nix gate red on the remote

- **Written**: 2026-09-10 06:25 CEST
- **Window**: five tasks completed 2026-09-10 ~04:02–05:06 CEST. Cited commits are the
  pre-rebase hashes the queue filed; all five were rewritten onto `origin/master` by the
  05:45 rebase — mapping: `c5c654c`→`0006f45`, `c4e8b9f`→`2d3340a`, `0275120`→(dropped as
  duplicate of upstream `20dafb9`, content preserved), `9b38039`→`0e2003c`,
  `be82e46`→`1ac345c`.
- **Method**: every claim re-verified against the live tree — `git show` of all five
  commits, `git log --all --grep` per task ID, `go build`/`go vet`/`TestStatsSurfacesAgree`
  green, `go run ./examples/fullcore` executed end-to-end (sqlite), 120-column audit of
  `internal/webui`, `gh run list`/`gh run view --log` over the four master CI runs of the
  night, and local nix forensics (`nix build`, `checks.vendor-hash`, and a build of the
  EXACT failed CI derivation via `builtins.getFlake ?rev=`).

## TL;DR

| Area                  | Verdict                                                                                                                  |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| The five window tasks | All five delivered their fix/work; verified in HEAD                                                                      |
| fullcore example      | Runs end-to-end on sqlite (12 completed, retry proof exercised live this session)                                        |
| **Nix CI gate**       | **RED on the remote since the window's push (05:45 CEST) — runner-only, locally green with the exact failed derivation** |
| Master CI overall     | Red: nix (new), test (release-gates smoke), test-windows, govulncheck, gosec — only nix is new                           |
| Review loop           | Worked as designed: four findings, four exact-scope fixes, one self-caught test blind spot                               |

---

## a) FULLY DONE (verified in HEAD)

1. **Review-fix: golines-clean baseline restored** (task `…037feea`, `0006f45`).
   `runEventStream` (internal/webui/handlers.go:247) and `pageResults` signatures are
   wrapped multi-line; `awk` length audit of handlers.go + render.go finds zero lines over
   120 columns. The f23 dedup claim ("`--new-from-rev` lint clean") is now true.
2. **Full-core embed example** (task `…13b8ec`, `2d3340a`). `examples/fullcore/main.go`
   exists; root go.mod carries the `internal/queue/postgres v0.2.0` require; flake
   vendorHash updated (`6acd7e2`). **Executed live this session**: `go run
   ./examples/fullcore` drains 12 tasks through custom + shell executors, proves retry
   ("flaky: succeeded on retry"), prints final counts. The README mention (follow-up task
   `…37826`, `b31f7fd`) is in place (README.md:395).
3. **Review-fix: stats wire contract pinned** (task `…cf6f1c`, `0275120` — content landed
   upstream as `20dafb9`). Both `tq serve` `/api/stats` and `tq api` `/api/v1/stats` derive
   keys from the task status list and always emit all five statuses + total; disjoint route
   namespaces asserted; dead label constants removed. `TestStatsSurfacesAgree` passes.
4. **Review-fix: stats contract test hardened against the both-drop blind spot**
   (task `…cf71af`, `0e2003c`). The seed walks a task into every status but pending, and a
   store-truth loop fails when a store-reported status is missing as a wire key from either
   payload. Verified by reading the test and running it.
5. **Review-fix: root go.mod points at the local postgres backend module** (task
   `…3f024e5`, `1ac345c`). The missing relative `replace` (go.mod:73) means root builds now
   follow local edits instead of the proxy's tagged copy — consistent with the other
   internal modules' local-dev layout. Verified present; `go build ./...` green.

## b) PARTIALLY DONE

1. **The nix gate is green locally and red on the remote — the window shipped it in that
   state.** The first nix-failing run coincides EXACTLY with the first push carrying the
   fullcore require + replace (see d1). The committed vendorHash is _locally_ correct; the
   failure exists only in the runner's module fetch. Untested: the postgres backend-choice
   path of the example (see c2).
2. **Stats contract: structural fix parked by design.** The two hand-mirrored status lists
   (webui `allStatuses`, httpapi `apiStatuses`) remain; the test doc names the parked fix
   (export `task.AllStatuses`) pending the sub-module API-surface call (TODO_LIST blocked
   item). A new status omitted from both lists AND the seed still slips through —
   documented residual, not a surprise.
3. **Backend tags on the remote, verification not run.** `git ls-remote` shows
   `internal/queue/{sqlite,postgres}/v0.2.0` tags now exist (TODO_LIST's push item premise
   is outdated); the per-module proxy check and clean-room `go install` they gate have not
   been run.
4. **README fullcore caveat scoping** — the loopback/no-auth warning sits between the PoC
   sentence and the fullcore sentence (05-58 report's finding). **Fixed in this docs-health
   pass** (README.md now names `examples/api` + `examples/sse` as the caveat's subjects).

## c) NOT STARTED (backlog the window skipped — verified still unchecked)

- f26 `docs/release/RELEASE.md`, f27 version-surface inventory doc.
- Lint-baseline slice-triage (wrapcheck/varnamelen), `ExitCause`→`ExitError` window,
  per-module golangci in ci.yml, multi-repo smoke vs nix binary, CI-time measurement,
  dogfood-hygiene drain.
- The whole "Dogfood round" section (cwd-dependence sweep, module-eval PATH assert,
  `checkProjectsDir` relaxation, skip-log dedup, `tq doctor` PATH warning, dead-pool alert,
  dogfood smoke, /tmp evidence archival, `tq pool-health`, worktree-per-agent doc,
  session-close bridge).
- The CI-hardening block filed by the 04-09 report (test-windows fix, release-gates smoke
  identity, x/text bump, gosec config, CI-state gate, line-length gate, job summaries,
  required-checks proposal) — all still open, and d1/d2 below are fresh evidence for the
  CI-state gate item.
- Owner-blocked set — unchanged and correctly BLOCKED.

The window spent itself entirely on its five items; nothing else was picked up.
Consistent with the done-prompt contract, noted for completeness.

## d) TOTALLY FUCKED UP

1. **The window's push broke the nix gate on the remote, and nobody looked.** Exact
   correlation: nix SUCCESS at 01:21Z (`6bcfd1c`) and 02:00Z (`20dafb9`); FAILURE from the
   first push carrying the fullcore require + go.mod replace (`98794b5`, 03:45Z) and again
   on current HEAD (`7c5f5e0`, 04:12Z). Forensics: the runner fails the go-modules
   fixed-output derivation with `specified: sha256-ZyjpU20K…` (= the committed vendorHash)
   `got: sha256-/rKFWqGR…` — **the same got-hash on two different trees**. Locally the
   exact failed derivation (`61cshlxl…-go-taskqueue-0.2.0-go-modules.drv`, evaluated from
   the failed run's commit via `builtins.getFlake ?rev=`) **builds green**, as do
   `checks.vendor-hash` and a full `nix build`. So: committed hash correct, fetch content
   diverges only in the runner environment, mechanism unidentified. The five DONE verdicts
   and two pushes (05:45, 06:03) landed on this red gate — the same
   no-master-CI-check failure mode the 04-09 report flagged, repeated within hours.
2. **Review-target/queue-footer audit trail is broken for one window task.** Task
   `…cf6f1c`'s commit `0275120` carries `Task-Queue-ID: …0ca7ae2` — an ID with ZERO
   commits (`git log --all --grep` empty) and not in this window. Meanwhile the rebase
   rewrote the other four commits with corrected footers but this one was dropped as an
   upstream duplicate. Anyone auditing ticket `…cf6f1c` from git cannot close the loop;
   anyone auditing `…0ca7ae2` finds a phantom. Same class as 05-58's d1 (dangling
   `c4e8b9f`), still unresolved as a process.
3. **CHANGELOG went dark for the whole window.** No entries exist for: the fullcore
   example, the stats wire-contract fix (a user-visible API shape change), the go.mod
   replace fix, nor the older gosec job + G114 fixes gap the 04-09 report already flagged.
   Backfilled in this docs-health pass — the gap itself is the finding.
4. **The stats test shipped 26 minutes with a self-admitted blind spot** (`0275120` →
   `0e2003c`): its doc claimed "any divergence fails" while a status dropped by BOTH
   surfaces passed. The follow-up task caught and fixed it in-window — good — but the
   first version's claim was false at commit time, the exact "scope-qualify gate claims"
   failure the 04-09 report demanded we stop repeating.
5. **In passing**: `check-dead-exports.sh` still reports the 48-symbol advisory baseline
   (webui `BudgetView`/`BoardColumn`/`DashboardData` among them); the err113 sentinel at
   `cmd/tq/doctor.go:472` still ages in the baseline. Both already filed; unchanged.

## e) WHAT WE SHOULD IMPROVE

1. **Make the master-CI state gate REAL — it is now two windows of evidence.** Five DONE
   verdicts landed on red master in the 04-09 window; five more plus two pushes repeated it
   here, and this time the push itself carried the break. `scripts/check-ci.sh` (TODO_LIST,
   04-09 f7) must land before the next release window, not with it.
2. **Diagnose the runner module-fetch divergence with a differential dump**: build the FOD
   on a runner with `vendorHash = lib.fakeHash`-captured content vs the local content and
   diff the module trees. Everything needed is in d1's evidence package. Then decide:
   pin/mirror the fetch (vendor/) or chase the environment.
3. **Footer hygiene for review-fix and rebase-dropped commits**: a commit may carry
   exactly one footer today; review-fix workflows and upstream-duplicate drops both break
   ticket↔commit auditability. Needs a queue-side convention (dual-footer or a
   rejected/duplicate disposition record) — owner-grade, filed as a question.
4. **Example-landing checklist** (from 05-58, repeated here): example + README mention +
   FEATURES row + CHANGELOG entry in ONE commit; this pass did the last three for the
   window's example.
5. **Scope-qualify test-name claims**: "pins X" must name the mechanism (the stats test
   now does — its doc comment is the model); "any divergence fails" claims get a
   both-sides-drop counterexample check before commit.

## f) NEXT THINGS (new items appended to TODO_LIST; existing backlog not duplicated)

**From this window's evidence (all appended):**

1. Restore the nix CI gate: diagnose the runner-only go-modules FOD hash mismatch (full
   evidence package in this report, d1).
2. Run the now-unblocked backend-tag verification: per-module `go list -m -versions` proxy
   check + clean-room `go install` for `internal/queue/{sqlite,postgres}/v0.2.0`.
3. Run the fullcore example once against a throwaway postgres DB (`--backend postgres`;
   the example applies its schema — keep it off any real instance).
4. Hermetic `scripts/smoke/fullcore.sh` (sqlite path, explicit `TQ_DB=<scratch>` export).
5. Status-task window payload: include the window's closeout/report paths so the done
   prompt reads them directly (05-30 f25, still unfilled).
6. Docs-health continuation: annotate + archive the six 2026-09-07 reports (batch left
   for the next pass — this pass verified and archived none of them).

**Already filed, re-escalated by this window (do NOT re-append; listed for the next task
picker):** test-windows fix, release-gates smoke identity, x/text ≥v0.39.0, gosec config,
master-CI state gate (`check-ci.sh`), changed-lines line-length gate, job-summary
artifacts, `task.AllStatuses` export, CHANGELOG backfill (DONE this pass), webui
dead-export triage, dogfood-round section.

**Remaining window-adjacent items NOT appended (flood discipline):** CHANGELOG policy for
docs-only fixes (left in the 05-58 report's list), review-finding content-anchoring
(queue-side, owner), rejected-commit disposition rule (folded into question g1 below).

## g) QUESTIONS FOR THE OWNER

1. **Ticket `…cf6f1c` close-out**: its commit carries a different task's footer
   (`…0ca7ae2`, phantom) and the content landed via upstream twin `20dafb9`. Should the
   queue mark it complete from the window manifest, and should review-fix/rebase-dropped
   commits get a dual-footer or disposition convention?
2. **Master push authorization**: AGENTS.md says agents never push, yet master moved at
   05:45 and 06:03 today (rebase result + docs). Were those owner-run (sanctioned), and is
   autonomous push now permitted for agent sessions — yes/no changes what the pool may
   claim.
3. **Module-fetch trust for the nix gate**: if the runner-only FOD divergence (d1) is
   confirmed as proxy/fetch-content drift, do you want the build to vendor modules
   (`vendor/` committed or fetched in a pinned FOD) — trading repo size/hermeticity for
   gate determinism — or keep proxy fetches and accept investigation-per-incident?

---

_Point-in-time snapshot 2026-09-10 06:25 CEST. Re-verify before treating any claim as
current — multiple concurrent-agent sessions are active on this repo._
