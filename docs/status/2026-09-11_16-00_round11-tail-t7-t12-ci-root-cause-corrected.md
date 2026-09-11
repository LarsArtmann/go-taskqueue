# Status Report — 2026-09-11 16:00 CEST — Round-11 tail: T7/T12 landed, CI root cause CORRECTED (runner "stable" floated to go 1.27.1)

Session scope: continuation of the 15:10 Round-11 execution window. Landed
T7/T12/T18 + M55/M58, then chased the still-red master CI and **corrected my
own earlier root-cause diagnosis** — the honest headline of this window.

---

## a) FULLY DONE

| Work | Evidence |
| --- | --- |
| **T12 index cross-checks (M42/M43)**: check-status-index.sh now (1) cross-checks a `task-NNN` report filename's ID against git-history `Task-Queue-ID` trailers, (2) flags filename-date vs report-body date drift. The trailer check **mechanically flagged the f26 cluster on first run**: the 07-49 report filename says `...c3919ce4…` but its commit trailer says `...b141f022…` — exactly the class the check was built for. WARNING-level until the owner's canonical-ID ruling (TODO 166), then flip to failing | `scripts/check-status-index.sh`; TODO item 154 marked done with the finding |
| **T7 backend-tag verification (M24–M26)**: `go list -m -versions` resolves `v0.2.0` for BOTH backend modules via the proxy; clean-room `go mod tidy` + `go get` fetch both with full dependency resolution; the compile step correctly refuses importing `internal/` packages from outside the repo (Go internal rule — by design, root module is the app layer). Supply-side multi-module promise PROVEN | TODO item 130 marked done with the honest compile-refusal caveat |
| **CI red root cause CORRECTED (the big one)**: my 14:47 GOEXPERIMENT fix was necessary but NOT sufficient. Evidence trail: run 34606066545 (commit 550fec1, containing the fix) still failed test/test-windows/test-postgres; the failing step's env block PROVES `GOEXPERIMENT: jsonv2` was set; Vet+Build PASS while only the cross-compile VET fails → not an env problem. `setup-go` log: **"stable version resolved as 1.27.1"** — on go 1.27, encoding/json/v2 is stable but stdversion-gated to modules declaring `go 1.27`; our go.mods say 1.26.7, so vet's stdversion fails regardless of GOEXPERIMENT (the experiment satisfies 1.26's availability gate, not 1.27's language-version gate). FIX: all 7 `setup-go` steps (ci.yml ×6, fuzz.yml ×1) pinned to `1.26.7`, matching go.mod + the toolchain-alignment gate + the flake | `.github/workflows/ci.yml`, `.github/workflows/fuzz.yml`; AGENTS.md gotcha drafted (see §d4) |
| **T18/M68**: confirmed the three stale-done items already carry `[x]` + DONE verdicts; meta-item marked done | TODO item 159 |
| **M55 lint-pin dedup**: ci-local now DERIVES the golangci-lint version from ci.yml (single source; the two can never drift) | `scripts/ci-local.sh` |
| **M58 testdata go.mod check**: repo-wide sweep — exactly 8 go.mods (root + 7 sub-modules), zero fixture/testdata go.mods; release-gates fixtures build in mktemp dirs outside the repo | TODO item 187 marked done |
| **Concurrency note filed**: full-suite re-run needed `TMPDIR`/`GOTMPDIR` redirect — /tmp (48G tmpfs) hit 100% from other projects' artifacts (monitor365 ~24G, playwright/pnpm); other sessions' files untouched | 15:10 report environmental note |

## b) PARTIALLY DONE

1. **CI green verification**: the 1.26.7 pin is on disk but UNPUSHED (working
   tree) — the next push's run is the only real proof. One run is
   `in_progress` right now (13:59 UTC) but it predates the pin.
2. **AGENTS.md gotcha for the toolchain drift**: written, then CLOBBERED by a
   concurrent writer before the daemon swept it (second clobber this window —
   the 14:22-redesign agent is still active). The knowledge lives HERE and in
   the ci.yml change; re-add the AGENTS.md bullet when the concurrent session
   ends.
3. Round-11 plan: T1–T13 complete; T14–T20, T22–T24, T26 and T25/T27 tails
   remain (unchanged from the 15:10 report §c).

## c) NOT STARTED

Unchanged from 15:10 §c: T14 (release.sh on fixture), T15 (ancestry
hardening), T16 remaining (M56/M57/M59), T17 (lint truth), T19 (docs-health
annotate six 09-07 reports), T20 (webui filter pins), T22 (fullcore/postgres
proof), T23 (pool-health), T24 (design docs), T26 (session-close bridge).

## d) TOTALLY FUCKED UP

1. **My first T5 diagnosis was WRONG, and I declared it fixed**: "no
   GOEXPERIMENT on CI runners" passed a local reproduction but was only HALF
   the cause. I never asked what `go-version: stable` resolves to on the
   runner — 13 minutes of `gh run view --log` would have shown
   `stable version resolved as 1.27.1` immediately. The corrected diagnosis
   (toolchain float, not env) is the real fix; the env fix stays (correct for
   1.26 runners) but the pin is what closes the gate. Lesson: verify the fix
   against the ENVIRONMENT THAT FAILED (CI runners), not just locally —
   "reproduced locally both ways" proved my local env theory, not the
   runner's.
2. **Second concurrent-writer clobber**: the AGENTS.md toolchain note I
   wrote was overwritten before commit. I keep losing edits in hot files to
   writers who snapshot-then-write whole files. Edit-verify-re-apply is not
   enough; window-end verification must RE-GREP every file I touched.
3. **Smoke re-runs failed on a full /tmp I can't clean** (other projects'
   24G+ artifacts) — burned two background-job cycles before identifying
   os.TempDir (not GOTMPDIR) as the path git-init and cgo actually use.
   TMPDIR (not GOTMPDIR) is the complete redirect.

## e) WHAT WE SHOULD IMPROVE

1. **Pin toolchains everywhere as policy**: setup-go now pinned, but the
   class recurs (golangci install, govulncheck, gosec all compile with
   whatever go is on PATH). A devShell-tool parity check (nix go version ==
   ci go version == local go version) in ci-local would catch the next float
   at pre-push time.
2. **check-ci.sh should WARN on "older than the push"**: it correctly flags
   red, but the red run it flagged was for a DIFFERENT tree than the one
   being pushed (the fix commit); the message could say "red run predates
   your tree — pushing a fix? bypass with CI_CHECK=off consciously".
3. The f26 WARNING in check-status-index needs a flip-to-fail trigger tied to
   the owner's canonical-ID ruling — file that linkage in the ruling item
   itself so it isn't forgotten (done in TODO 154's text).
4. Concurrent-writer protocol: whole-file snapshot writers + my surgical
   edits = lost work. Re-grep every touched file at window end; consider a
   tiny `scripts/check-window-edits.sh` listing my expected invariants
   (ci.yml pin count, AGENTS gotchas, hook file) for a one-command verify.
5. The clean-room backend proof should ride into RELEASE.md as the documented
   verification step for every module-tag push (it's currently only in TODO
   prose).

## f) Up to 50 things we should get done next

1. PUSH the toolchain pin (ci.yml ×6 + fuzz.yml) and WATCH the run — this is the actual T5 closure.
2. If green: update check-ci + the 15:10 report's "residual" line; if still red: the next suspect is runner go-modules cache, diff via the plan's differential-dump (M19/M20).
3. Re-add the AGENTS.md toolchain-drift gotcha (clobbered; text in this report §a).
4. OWNER: SystemNix flip + `nix run .#deploy` (runbook §2) — still THE lever.
5. OWNER: DLQ triage per runbook §4 (21 dead tasks).
6. OWNER: `--allow-writes` on tq-serve in the same window.
7. OWNER: canonical-ID ruling for the f26 cluster → then flip check-status-index's trailer check to failing (TODO 154).
8. OWNER: /tmp cleanup or tmpfs size bump (other projects' 24G+).
9. T14: execute release.sh on a fixture repo (M47–M49).
10. T15: release.sh ancestry audit + doctor warning for unreachable release tags (M50–M53).
11. T16: actionlint into devShell + ci-local (M56).
12. T16: advisory lint end-of-step summary (M57); changed-lines lll gate (M59).
13. T17: prove root .golangci.yml resolution inside a sub-module; document (M60/M61).
14. T17: per-module lint baseline quantification + baseline file (M63/M69).
15. T17: slice-triage round 2 — goconst/mnd/paralleltest/testpackage (M64–M67).
16. T19: docs-health annotate the six 2026-09-07 reports (M70–M72).
17. T20: webui FilterState round-trip pins + `?q=` e2e + LIKE cap (M73–M78).
18. T21: KanbanBoard + SEO/icons evals (M83/M84).
19. T22: fullcore hermetic smoke + drain-deadline pin + postgres example (M85–M89).
20. T23: `tq pool-health` + harvest /tmp hot-item flag (M90–M93).
21. T24: worktree-per-agent design doc (M94/M95); daemon-attribution proposal (M96).
22. T24: history-rewrite policy line (M97); consumer ghost ADR (M98); interface-dedup comparison (M99).
23. T26: session-close bridge design + PreToolUse prototype (M110–M113).
24. T27: required-checks proposal (M116); gosec/govulncheck CI summaries (M117).
25. T27: gosec FP config encoding the triage (M118) — would ALSO fix the gosec job showing as "failure" in gh.
26. T27: govulncheck hard gate (M119); release-gates GIT_CONFIG_GLOBAL fix (M120); AllStatuses prep (M121).
27. T27: ghost-report upstream draft (M122); dependabot policy (M123).
28. M124: post-deploy retro on 2026-09-18 (runbook §5).
29. Add `--commits` view to `tq facts` too (show-only surface misses the facts-first workflow).
30. Parked-pin extension: MarkOrphaned vs parked-PENDING twin interplay.
31. Persist "will resume at closeout" into the requeue fact detail (observability).
32. http executor: HTTP-date Retry-After test via fixture server (unit-pinned only).
33. stats parked count: JSON-shape contract test.
34. doctor parked check: name the earliest not_before ("until 19:40").
35. Log armed provider tag when output carries `provider=<x>` (cheap observability).
36. Bridge: same 429-transient treatment for NotifyDeadPool direct posts (audit covered ingest only).
37. ratelimit-e2e: assert the SECOND claim happens AFTER not_before.
38. Postgres conformance: assert fact COUNT didn't grow on stale-fail (pin the sqlite fix's parity).
39. ci.yml: concurrency group to cancel superseded runs.
40. Runbook: regenerate the DLQ table at flip time (dead set grows).
41. go mod verify buildcache flake: pin GOMODCACHE or retry-once in check-go-mods.
42. RELEASE.md: document the multi-commits-per-task norm + clean-room backend verification step.
43. `tq show --commits`: clear note when git is missing from PATH.
44. WebUI: surface the parked count in the nowband (stats payload already carries it).
45. RequeueEvidence: add absolute `retry_in_until` alongside retry_in_ms.
46. WithoutCloseout: test that repo-armed gates don't leak to the review clone.
47. Hook installer: worktree-safe (.git-as-file) hooksPath handling.
48. Add ratelimit-e2e smoke to ci-local's smoke section.
49. docs: check-ci "red predates your tree" wording (§e2).
50. Sweep the re-dispatch loop finding from the 14-01 report: status-append cap/dedup (their top item, still open).

## g) Questions I can NOT figure out myself

1. **Toolchain policy**: now that setup-go is pinned to 1.26.7, do you want the whole repo bumped to go 1.27 (go.mods + pin together) in the NEXT release window instead — json/v2 stable without the experiment, one less env to carry? Or stay on 1.26.7 until the rate-limit fix has baked in production?
2. **f26 canonical ID**: the trailer check now flags `...c3919ce4` (filename) vs `...b141f022` (trailer) mechanically. Which is canonical — or should the check whitelist the 07-49 report permanently?
3. **Concurrent-writer protocol**: two of my edits were clobbered today by the 14:22-redesign agent's whole-file writes. Are parallel agent sessions on this repo intentional (budgeted) for this window, or should the pool pause during owner-directed windows like this one?

---

Gates at window end (with TMPDIR/GOTMPDIR redirect; /tmp host-full): root
build/vet/test GREEN; all 7 sub-modules GREEN; webui full re-run GREEN (one
SSE timing flake under my own parallel background load — passed standalone);
doc gates (todo/status-index/features/ghost-archives/doc-refs/go-mods) GREEN;
CI: one run in_progress (pre-pin), the pin itself UNPUSHED awaiting the next
push.
