# Window Status Report — advisory scan jobs (f20/f21), templ-components audit (f22), webui dedup (f23), stats split-brain (f24)

- **Written**: 2026-09-10 04:09 CEST
- **Window**: five tasks completed 2026-09-10 ~02:29–03:58 CEST (commits `1a11960`, `17a5940`, `4ce666e`, `4d2fd68`+`6bcfd1c`, `716eaca`+`20dafb9`), plus the post-window golines repair `c5c654c` (different task, noticed in passing).
- **Method**: every claim below was re-verified against the actual diffs (`git show`), the live code (`rg`/`view`), a fresh local root-module gate (`go build ./... && go vet ./... && go test ./... -race -count=1` — green), `gofmt -l`, `scripts/check-dead-exports.sh`, and the GitHub Actions history (`gh run list` / `gh run view --log-failed`). Format is Markdown per the task prompt's explicit `.md` instruction (overrides the status-report skill's HTML default for this run).

## TL;DR

| Area                       | Verdict                                                                                                                                                                                                                                                                                                                    |
| -------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| The five window tasks      | All five delivered what they claimed, verified in code/diffs                                                                                                                                                                                                                                                               |
| Local gates at report time | Green (build, vet, full race suite, gofmt)                                                                                                                                                                                                                                                                                 |
| **Master CI**              | **RED since ~01:15 CEST — before the window started; still red at HEAD** — RESOLVED same day: the red-master trio (windows test, release-gates identity, x/text) repaired 2026-09-10 (TODO_LIST L103-105 + CHANGELOG); the CI-red root-cause line was later re-diagnosed as the setup-go toolchain float (16-00 report §a) |
| Advisory scan jobs         | Both red on their first runner run: gosec by design (FP triage), govulncheck with a REAL reachable vuln in `internal/queue/postgres`                                                                                                                                                                                       |
| Verification honesty       | One false gate claim in the window (dedup "lint clean"), self-corrected by `c5c654c` 44 min later                                                                                                                                                                                                                          |

---

## a) FULLY DONE (verified)

### f20 — govulncheck CI job (`1a11960`, 02:29)

- New `govulncheck` job in `.github/workflows/ci.yml`: pinned `govulncheck@v1.8.0`, root module plus the disk-derived sub-module loop, job-level `continue-on-error: true`. Correctly documented as runner-only (live vuln DB fetch can never join the hermetic nix gates or ci-local.sh).
- CHANGELOG entry added; TODO_LIST flipped with a DONE note. Confirmed present in both files.

### f21 — gosec pass over all modules (`17a5940`, 02:41)

- gosec v2.29.0 over root + every sub-module: 48 findings triaged per rule class; the rationale is durably recorded in the AGENTS.md "gosec advisory baseline" note (verified present).
- The two REAL findings are genuinely fixed: `examples/api/main.go` and `examples/sse/main.go` now use `http.Server` with `ReadHeaderTimeout: 5s` instead of bare `http.ListenAndServe` (G114).
- Advisory `gosec` CI job added, mirroring govulncheck (verified in ci.yml).

### f22 — templ-components deep-dive audit (`4ce666e`, 02:56)

- Verified audit result: `go.mod` pins v1.16.0; the library's only `Deprecated` marker (forms.Form layout field) is in a package this repo never imports; v1.14→v1.16 was purely additive (KanbanBoard, icons.Render, PageProps.SEO), no Removed/Deprecated; the adoption table is pinned bidirectionally by the guard tests.
- The only drift found was documentation and it was fixed: stale "v1.14.x" → "v1.16.x" in AGENTS.md and FEATURES.md (verified in the diff and in the live files).
- Audit-only outcome with zero code churn is the correct shape for this task.

### f23 — webui dedup deep pass (`4d2fd68` auto-commit 03:15 + `6bcfd1c` 03:18)

- The five claimed helper consolidations all exist in the live tree (verified by symbol search): `completionDetail[T]` (render.go:194), `pageResults[T]` (render.go:242), `runEventStream` (handlers.go:247), `sendSnapshotPayload`+`queryErr` (handlers.go:357/368), `taskFromPath` (handlers.go:405). Dead `filterSuffix` is gone.
- Net −67 LOC in `internal/webui` (150 insertions / 217 deletions across handlers.go + render.go), matching the claim.
- Local full race suite green at report time, including `internal/webui` (10.4s) and `internal/httpapi`.

### f24 — stats split-brain fix (`716eaca` auto-commit 03:56 + `20dafb9` 03:58)

- The diagnosed split-brain was real and is closed: `tq serve` `/api/stats` and `tq api` `/api/v1/stats` previously computed the payload via two independent paths (dashboard: full snapshot projection keyed by HTML badge labels; API: raw GROUP BY rows dropping zero-count statuses). Both handlers now key from the status enum list and always emit all five statuses + total (verified webui handlers.go:147–180, httpapi.go `apiStatuses`).
- `TestStatsSurfacesAgree` (internal/webui/stats_contract_test.go) pins the two payloads equal over one seeded store and asserts the write API 404s on unversioned `/api/stats` — disjoint route namespaces locked in.
- Performance claim verified: webui `/api/stats` reads `StatusCounts` directly; the full-snapshot poll cost is gone.
- Dead label constants removed (`labelPending/Running/Dead/Cancelled`, `badgePending`).

---

## b) PARTIALLY DONE

1. **Dedup's verification claim was false at commit time.** The f23 DONE note says "`--new-from-rev` lint clean", but `runEventStream` and `pageResults` were left over the 120-column golines limit; `c5c654c` (04:02, a _different_ task) had to wrap both signatures. The claim is true only _after_ the repair. Self-corrected within the hour — but the window's own gate claim did not hold when written.
2. **gosec pass is triage-complete but structurally unfinished**: the advisory job exits 1 on the 48 triaged-as-FP findings (no suppression config exists), so it is red _by construction_ on every run (see d3). Triage recorded, enforcement story missing.
3. **govulncheck job is live but its "clean" premise died on first contact with the runner**: local scans covered root + sqlite only; the first CI run found a real reachable vulnerability in `internal/queue/postgres` (see d2). Advisory framing contained the damage exactly as designed — but the baseline is not clean.
4. **c5c654c (HEAD) has no CI run at report time** (04:09): the newest completed run is for `20dafb9`. HEAD is locally green (build/vet/race/gofmt re-run for this report) but CI-unverified.

---

## c) NOT STARTED (backlog the window skipped — verified still unchecked in TODO_LIST.md)

- f25 full-core example (worker + executor + queue, sqlite-vs-postgres import story).
- f26 `docs/release/RELEASE.md` (multi-module release flow), f27 version-surface inventory doc.
- f28/f29 lint-baseline slice-triage (wrapcheck ~50, varnamelen ~50), f30 `ExitCause`→`ExitError` rename window, f32 per-module golangci in ci.yml, f35 multi-repo smoke vs nix binary, f49 CI-time measurement, f46 dogfood-hygiene drain of stale queued tasks.
- The whole "Dogfood round" section (f8–f27 of the 02:00 report): cwd-dependence sweep, module-eval PATH assert, `checkProjectsDir` absolute-repos relaxation, skip-log change detection, `tq doctor` PATH warning, dead-pool PapDashboard alert, dogfood smoke, /tmp evidence archival, `tq pool-health`, worktree-per-agent design doc.
- Owner-blocked set (push authorization remainder, `internal/consumer` ghost-package decision, triple-interface decision, Postgres CLI wiring timing, dependabot policy, CQA live-instance verify) — unchanged, correctly BLOCKED.

The window spent itself entirely on its own five items; nothing from the surrounding backlog was picked up. That is consistent with the done-prompt contract, noted here for completeness.

---

## d) TOTALLY FUCKED UP

1. **Master CI has been red for ~3 hours and the whole window landed on it without noticing.** Verified timeline (gh): last green master run 23:23 CEST (Sep 9); first red run 01:15 CEST; every push since is red, including both window pushes (`6bcfd1c` 03:21 run, `20dafb9` 04:00 run). All five tasks reported local gates green — which was true — but none looked at master CI state, so five "DONE" verdicts were stamped onto a red tree. The red predates the window; it is not the window's regression, but the window's completion claims are blind to it.
2. **Hard-gate break #1 — release-gates smoke, exit 128, self-inflicted at 00:22 CEST.** `scripts/smoke/release-gates.sh:42` runs `git -C "$fixture" tag -a` _without_ the `-c user.email/-c user.name` that line 40 gives the fixture commit. Annotated tags need committer identity; GitHub runners have none → "Committer identity unknown" → the step fails on every CI run. Local runs pass because the host has a global git identity — the script was born un-hermetic. (Landed in auto-commit `adabaef` with the release-gates feature.)
3. **Hard-gate break #2 — test-windows, red since 01:38 CEST.** `TestHarvestConfigFromOptionsExpandsBareRepoNames` fails on windows-latest (subtests `mixed_entries_with_spacing`, `absolute_repos_stay_untouched`) — landed by the _pre-window_ dead-pool-fix session (`a0b720e`), almost certainly a path-separator/expansion assumption. Not this window's code, but this window's master.
4. **govulncheck: a real, reachable vulnerability on its first runner run.** `internal/queue/postgres` — GO-2026-5970, `golang.org/x/text@v0.29.0` infinite loop, fixed in v0.39.0, reached via `postgres.Open` → `pgxpool.NewWithConfig` → `norm.Form.*` (verified in the failed-run log). The task's commit message says "Local root + sqlite scans clean" — accurate but scope-narrow; the affected module was simply not part of the local spot-check. Needs an x/text bump across modules.
5. **gosec advisory job is permanently red by construction.** gosec exits 1 whenever it finds anything; the 48 FP findings have no in-repo suppression config (the triage lives as AGENTS.md prose). Result: an advisory job that is red on every run, training everyone to ignore it — the exact alert-fatigue pattern that will hide the first real gosec finding.
6. **CHANGELOG omission in f21.** The govulncheck job got a CHANGELOG entry; the gosec job and the two examples G114 fixes did not (verified: `17a5940` touches no CHANGELOG; no later commit added one). CHANGELOG is append-only and this is a permanent gap until backfilled.
7. **Commit archaeology noise around the window.** The dedup's actual code diff lives in an auto-commit (`4d2fd68` "chore: auto-commit 2 changed file(s)") that _precedes_ the task's own commit (`6bcfd1c`), whose diff is a TODO flip; same pattern for f24 (`716eaca` carries the code, `20dafb9` the cleanup). Anyone auditing "what did task X change" from commit messages alone gets the wrong answer.
8. **In passing (pre-existing, advisory)**: `check-dead-exports.sh` currently reports 48 zero-importer exports, including webui's `BudgetView`, `BoardColumn`, `DashboardData` — the dedup pass didn't look at export-level dead weight. And the err113 finding (`cmd/tq/doctor.go:472`, dynamic `errors.New`) hard-failed the lint-annotations step once at 01:15 CEST, then silently aged into the `--new-from-rev` baseline without ever being fixed — a live demonstration of how new findings get swallowed by baseline drift.

---

## e) WHAT WE SHOULD IMPROVE

1. **Master-CI awareness as a completion precondition.** Every tool needed exists (`gh run list`). One script + one ci-local step turns "landed on red master" from invisible to blocked. Cheapest possible fix for the d1 process hole.
2. **Hermetic git in smoke fixtures.** Any fixture `git` mutation (commit _and_ annotated tag) must carry identity explicitly (`-c user.*` or `GIT_AUTHOR_*`/`GIT_COMMITTER_*`), and ci-local should prove it by running the smoke under a sanitized `GIT_CONFIG_GLOBAL`/HOME. Runner ≠ laptop.
3. **Encode triage in config, not prose.** The gosec triage is excellent work stranded in AGENTS.md; a gosec config with the exclude-rule list converts it into an enforceable, green-by-default baseline where only _new_ classes make noise.
4. **Scope-qualify gate claims.** "Local scans clean" (root+sqlite) read as "clean". DONE notes should name the exact scope — the postgres finding would have been a 30-second catch instead of a CI surprise.
5. **Advisory red must be actionable red.** Job summaries/artifacts for gosec+govulncheck findings, and a policy to flip `continue-on-error` off once green. An advisory job that never goes green is a broken sensor.
6. **Line-length gate on changed lines.** The golines wrap regression (c5c654c class) is mechanically checkable in ci-local (lll/golines `--new-from-rev`-style); it should never depend on a second task noticing.
7. **Order-of-commits hygiene.** Task-owned commits should contain the task's code, with the auto-daemon mopping up only leftovers — the current interleaving makes history lie about which task did what.
8. **CHANGELOG discipline for CI/tooling changes** — same review reflex as code changes; f20 did it right, f21 didn't.

---

## f) NEXT THINGS (new items only — appended to TODO_LIST.md; the existing unchecked backlog in section (c) is NOT duplicated here)

> Resolution (docs-health 2026-09-11, pre-archive): done rows struck below
> (verified against code/CHANGELOG at HEAD); every unmarked row is still
> open and lives in TODO_LIST.md §"Window f20–f24 follow-ups" (f4→L106,
> f5→L107, f6→L108, f8→L110, f10→L112/L124 BLOCKED, f11→L113, f12→L114,
> f14→L116, f15→L117, f16→L118, f17→L119); §g questions → TODO_LIST L123/L124/L125
> (BLOCKED: owner). Nothing unowned remains.

1. ~~Fix test-windows: `TestHarvestConfigFromOptionsExpandsBareRepoNames` (2 subtests) fails on windows-latest — audit the bare-repo-name expansion for `filepath.Separator`/abs-path assumptions (landed `a0b720e`).~~ done 2026-09-10 (TODO_LIST L103; separator-aware logic confirmed)
2. ~~Fix release-gates smoke for identity-less environments: give `scripts/smoke/release-gates.sh:42`'s `git tag -a` the same `-c user.email/-c user.name` the init commit carries (or export GIT_COMMITTER_* at script top).~~ done 2026-09-10 (TODO_LIST L104; fixture commit+tag carry `-c` identity)
3. ~~Bump `golang.org/x/text` to ≥v0.39.0 in `internal/queue/postgres` (GO-2026-5970, reachable via `postgres.Open` → pgxpool) and sweep every module for the same x/text floor.~~ done 2026-09-10 (TODO_LIST L105; postgres go.mod at v0.41.0)
4. Add a gosec config encoding the 2026-09-10 FP triage (exclude-rule list for G204/G702/G703/G304/G306/G301/G302/G124/G710/G118/G104/G404/G202) so the advisory job goes green and new classes stand out.
5. Flip govulncheck's `continue-on-error` off once the x/text bump makes the job green.
6. ci-local.sh: run the release-gates smoke with `GIT_CONFIG_GLOBAL=/dev/null` (or empty HOME) so identity-dependent git ops fail locally the way they do on runners.
7. ~~Add a master-CI state gate: `scripts/check-ci.sh` (fail when the latest master run is a failure via `gh`), wired into ci-local.sh — five DONE verdicts landed on a red master without anyone looking.~~ done 2026-09-11 (verified: scripts/check-ci.sh wired at ci-local.sh:20-21)
8. Add a changed-lines line-length gate (lll/golines, 120 cols) to ci-local.sh so signature-wrap regressions (the `c5c654c` class) are caught before commit.
9. ~~Backfill CHANGELOG: gosec advisory job + the two examples G114 ReadHeaderTimeout fixes (`17a5940`) never got an entry.~~ done (docs-health pass 2026-09-10 06:25 — gosec job + G114 fixes backfilled under CHANGELOG Added)
10. Export the status enum list from `internal/task` (`task.AllStatuses`) and retire the twin lists (webui `allStatuses`, httpapi `apiStatuses`) — rides the next sub-module re-tag.
11. Publish gosec/govulncheck findings as job-summary artifacts so advisory-red runs are readable without log-diving.
12. Write the required-checks proposal (test-windows + release-gates smoke, then the scan jobs) into docs/planning/ for the next release window.
13. ~~Triage the webui zero-importer exports the dedup pass left behind (`BudgetView`, `BoardColumn`, `DashboardData`; part of the 48-symbol dead-export advisory).~~ done 15-10 (15-10 report M82: all LIVE — used in-package by templ fragments; zero dead exports)
14. Evaluate templ-components `KanbanBoard` (added v1.15+) against the custom board columns/cards in fragments.templ.
15. Evaluate templ-components `PageProps.SEO` + `icons.Render` for layout.Base and the filter/empty-state icons.
16. Harden the two example servers beyond `ReadHeaderTimeout` (IdleTimeout/ReadTimeout), keeping the SSE `WriteTimeout=0` exemption documented.
17. docs/planning/: verification-claims guidance — DONE notes must state gate scope (the govulncheck "local scans clean" claim was root+sqlite only while postgres carried the real finding).
18. ~~Fix the err113 finding at `cmd/tq/doctor.go:472` (dynamic `errors.New("failing checks found")`) as a static sentinel — it hard-failed lint-annotations once, then silently aged into the `--new-from-rev` baseline.~~ done 15-10 (static `var errDoctorFailed`, cmd/tq/doctor.go:41; verified at HEAD 2026-09-11)
19. ~~SECURITY.md: add the two advisory scan jobs (govulncheck, gosec) to the defense-layers matrix.~~ done (verified at HEAD 2026-09-11: defense-layers carries both)
20. ~~Confirm a CI run exists for the current HEAD (`c5c654c` had none at 04:09 despite being pushed at 04:02) and record the outcome.~~ done (runs exist for HEAD 7c5f5e0 (34436225021): RED — nix/test-windows/test/govulncheck/gosec; recorded in TODO_LIST + 06-25 report)

## g) QUESTIONS FOR THE OWNER

1. gosec gating policy: once a suppression config encodes the triage, should gosec become a blocking required check, or stay advisory permanently?
2. `internal/task` public surface: export `task.AllStatuses` in the next minor release to kill the httpapi/webui twin status lists, or keep the duplication + cross-module pin test as the permanent shape?
3. Pool policy when master CI is red: should agent DONE verdicts be refused (fix-forward or park required), or is report-only disclosure of the red state the intended ceiling?

---

_Point-in-time snapshot 2026-09-10 04:09 CEST. Re-verify before treating any claim as current — five concurrent-agent sessions are active on this repo._
