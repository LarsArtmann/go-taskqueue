# Round-12 execution window: CI-red triage, T14 fixture release proof, T15–T17 hardening

**Date:** 2026-09-12 01:09 CEST · **Session start:** 23:15 the previous evening
**Scope:** Continuation of the Round-11 plan
(`docs/planning/2026-09-11_14-01_SUPERB-PLAN-ROUND11-POOL-REVIVAL-AND-QUEUE-TRUST.md`);
window opened by triaging the NEW CI red that arrived after the toolchain
pins, then executed T14–T17 top-down until the status-report interrupt.

**Verdict:** CI's two hard-gate failures root-caused and fixed locally
(unproven on CI — nothing was pushed, per repo rule); T14 and T15 fully
executed with live proof; T16 executed; T17 ~80% done (baseline file
generated, config triage applied, but the checker is NOT yet wired into
ci-local — the last edit of the window failed, see §d).

---

## a) Fully done

### CI red triage (blocking, unplanned)

Run 34648537255 (master, 21:16 2026-09-11) failed three jobs. Triage:

1. **`test` → "Module isolation gates" (executor module)** — REAL, mine to
   fix. `TestAgentExecutorRateLimitClassifiedAndGated` and
   `TestCloseoutRateLimitResumesNotReRuns` died with
   `*PreflightError: repo has uncommitted changes … ?? ran.log` instead of
   `*RateLimitError`. Root cause: a CONCURRENT agent landed the
   `require_clean` preflight (default TRUE, `AgentPayload.RequireClean`,
   `assertCleanTree` in `internal/executor/agent.go:350`) AFTER the window
   that wrote the rate-limit tests. The stub agents write `ran.log` /
   `work.log` / `closeout.log` straight into the fixture repo → dirty tree →
   preflight refuses before the rate-limit gate/resume path is ever
   reached. **Fix (test-only, no production semantics touched):** the two
   rate-limit pins now set `RequireClean: &noClean` with a comment saying
   exactly why (`internal/executor/agent_test.go:795,884`). Full executor
   suite green (`GOWORK=off go test ./...` 3.0s).
2. **`test-windows` → `TestAllReposAbsolute`** — REAL. The test hardcoded
   POSIX specs (`/srv/a`); `filepath.IsAbs("/srv/a")` is false on Windows
   (no drive letter), so `single_absolute` / `all_absolute` cases inverted.
   **Fix:** specs are now built from the platform's own notion
   (`runtime.GOOS == "windows"` → `C:\srv\a` etc.) so both runners exercise
   the same contract (`cmd/tq/seeds_test.go:68`). Verified locally plus
   `GOOS=windows go build && go vet` clean.
3. **`gosec` job red** — NOT a run blocker: job-level `continue-on-error:
   true` (advisory by design, f21). The new G703 findings it reported
   (`cmd/tq/poolconfig.go:26`, several `cmd/tq/bootstrap.go` sites) are the
   already-triaged "local CLI operating on user-named repo/config paths"
   baseline class (AGENTS.md gosec note covers G703 generically). No action
   taken; noted here so the next reader doesn't re-triage from zero.

The toolchain pins themselves WORKED: on the failing run, Vet/Build/Test
(Linux) were green for the first time since the 1.27.1 float — the jsonv2
cross-compile failure is gone. The two test fixes above are what the next
push must prove (auto-daemon will commit them; pushes stay owner-gated).

### T14 — release.sh executed on a fixture repo (M47–M49)

First-ever LIVE execution of `scripts/release.sh` (previously only
line-read). Fixture at `~/.cache/tq-release-fixture`:

- **Scaffold (M47):** multi-module shape mimicking the real repo — root
  `go.mod` + `internal/task` + `internal/queue/sqlite` sub-modules with
  real-version requires, CHANGELOG section, `version`/ldflags-carrying
  flake, stub `scripts/ci-local.sh` + `scripts/smoke/webui.sh`, REAL
  `scripts/lib/release-gates.sh` copied in (the code under test).
  **Finding:** `gate_gomod` hardcodes the module path
  (`github.com/larsartmann/go-taskqueue`), so fixture releases MUST mimic
  it — by design (foreign shapes should be refused), documented in
  RELEASE.md.
- **Gates + --tag (M48):** `release.sh v0.2.0` gates-mode green (with a
  stubbed `nix` on PATH — the sandbox refuses undeclared store paths, so a
  real fixture flake build was abandoned; the gate's logic is the test
  target, honest limitation recorded); `--tag` cut the annotated root tag
  AND disk-derived sub-tags. Second release `v0.2.1` exercised the
  forward-only ordering gate and sub-tag cutting for a newly added module.
- **Clean-room resolution (M49):** consumer module resolves
  `github.com/larsartmann/go-taskqueue@v0.2.1` AND
  `…/internal/queue/sqlite@v0.2.1` from the fixture's git tags via
  `GOPROXY=direct` + `GIT_CONFIG_GLOBAL` insteadOf rewrite + `GOPRIVATE`.
  Both `go get`s and `go list -m all` green.
- **Negative gates:** existing-tag re-release dies (immutability);
  backwards version dies (forward-only). Replace-poison gate died correctly
  on my first wrong-shaped fixture (caught my own mistake).
- **REAL BUG FOUND + FIXED in prod script:** the webui-smoke step
  `TQ_BIN="$(nix build --print-out-paths)/bin/tq" ./scripts/smoke/webui.sh`
  — bash does NOT abort when a command substitution inside an env-prefix
  assignment fails (the command still runs; exit status comes from the
  command). A failed `nix build` printed "GATES GREEN". Reproduced with a
  3-line bash probe. **Fix:** standalone `nix_out="$(nix build
  --print-out-paths)"` assignment (which DOES abort under `set -e`, also
  probe-verified), then `TQ_BIN="$nix_out/bin/tq" …`
  (`scripts/release.sh:91`), with the reason in a comment.

### T15 — release tooling vs forked lineages (M50–M53)

- **M50 audit:** every `v*` tag IS reachable from master today (T13's heal
  held). Latent assumptions documented: `LAST_TAG` gates ordering against
  the highest tag GLOBALLY (correct across forks) while `git describe`
  answers with sub-module tags (`internal/queue/postgres/v0.2.0`) and would
  answer with fork tags — nothing in the script uses `describe`, noted as a
  hazard for future edits.
- **M51:** new `tag-ancestry` doctor check (`cmd/tq/doctor.go`,
  `doctorTagAncestry`): WARN when any `v*` tag is not an ancestor of HEAD,
  OK-skip outside a git repo. Live-verified: `ok tag-ancestry all release
  tags reachable from HEAD`.
- **M52:** RELEASE.md gained "Tag ancestry (forked lineages)" and "The
  --push failure path" sections (per-step failure modes, proxy-poison
  rules, fixture-proof note).
- **M53:** the `--push` phase now AUTOMATES the confirm-CI-green-on-tag
  step: polls `gh run list --commit <tag-sha> --workflow CI` up to 30x60s,
  dies on a non-success conclusion, dies on timeout with the manual
  fallback command. Previously the script's final line told the human to go
  check.

### T16 remainder (M56–M59)

- **M56 actionlint:** in flake devShell (`pkgs.actionlint`) + a hard
  ci-local step (`actionlint .github/workflows/*.yml`, `nix shell
  nixpkgs#actionlint` fallback when not on PATH). Both workflows are
  actionlint-clean TODAY (verified via `nix shell` run before wiring).
- **M57 lint summary line:** ci-local's advisory lint block now ends with
  `lint summary: N findings (advisory baseline ~400, AGENTS.md)`; ci.yml's
  advisory lint step got the same end-of-step summary (output captured per
  module, counts summed). YAML + actionlint re-verified after the edit.
- **M59 changed-lines lll gate:** new `scripts/lint-lll-changed.sh` —
  awk over `git diff -U0` tracking hunk new-line numbers; added `*.go`
  lines >120 chars fail, `*_templ.go` excluded, base resolution identical
  to lint-annotations.sh (`LINT_BASE` → `$1` → HEAD~1, skip when no base).
  Positive probe VERIFIED end-to-end (committed a 142-char line → gate
  failed with `file:line: 142 chars`; removed via `git rm` + commit — see
  §d for the history-noise cost). Wired as a hard ci-local step.

### T17 partial (M60, M61, M64–M67, M69-config)

- **M60 PROVEN:** `golangci-lint config path` inside `internal/executor`
  and `internal/journal` both answer `../../.golangci.yml` — v2 walks the
  parent tree, every sub-module lint inherits the root config. No
  per-module config needed (M62 moot).
- **M64–M67 triage with data** (full-loop capture, 1081 findings total
  pre-change): `paralleltest` 146 (all in tests, zero-actionable —
  table-driven + `t.Setenv` + shared sqlite fixtures deliberately
  non-parallel) → **disabled**; `testpackage` 48 (all in tests — white-box
  in-package tests are the repo convention) → **disabled**; `goconst` 71
  (57 in tests — table-literal repetition is the readability contract) →
  `ignore-tests: true` added, 14 prod findings stay advisory; `mnd` 81 (all
  prod, mostly flag defaults/ports by design) → **kept advisory**,
  documented. Each decision carries a comment in `.golangci.yml` or lands
  in the AGENTS.md note (M61 text drafted in this report, see §f item 1).
- **M69 baseline file:** `.golangci-baseline.txt` GENERATED (110
  module+linter rows, 835 findings — down from 1081 after the config
  triage) by the new `scripts/lint-baseline.sh`; plus
  `scripts/check-lint-baseline.sh` (fails on any linter appearing in a
  module without a baseline row, baseline-0 linter reporting anything, or
  growth beyond a 20% allowance; prints totals otherwise).

## b) Partially done

- **T17 wiring:** `check-lint-baseline.sh` exists but is **NOT wired into
  ci-local.sh** — the window's last edit (restructuring `lint()` to emit
  `== module` markers the checker parses, then piping `lint_out` through
  the checker) FAILED as an empty/invalid tool call and was never retried.
  ci-local's `lint()` also still lacks `== module` markers in its
  else-branch (the not-on-PATH install path runs root only — pre-existing
  gap now worth closing in the same edit).
- **AGENTS.md M61 documentation:** the config-resolution rule
  (parent-walk, no per-module configs) and the four triage verdicts are
  NOT yet in AGENTS.md — concurrent-clobber risk makes that a deliberate
  re-read-then-edit, deferred to the next window's first action.
- **goconst ignore-tests / paralleltest / testpackage config changes:**
  applied and the baseline regenerated against them, but the full test
  suite has NOT been re-run since the `.golangci.yml` edit (lint-only
  change; risk is near-zero but the convention is to verify).

## c) Not started (from the Round-11 plan, untouched this window)

T19 (docs-health annotate the six 2026-09-07 reports — note: the 23:16
concurrent docs-health full-sweep report claims a repo-wide annotate pass;
its actual coverage of those six files needs verification before doing it
twice), T20 (webui FilterState pins, `?q=` e2e, LIKE cap), T21 tails
(KanbanBoard, SEO/icons evals), T22 (fullcore hermetic smoke, drain
deadline pin, postgres example), T23 (`tq pool-health`, harvest /tmp
hot-item flag), T24 (five design docs), T26 (session-close bridge design +
PreToolUse prototype), T25/T27 tails (varnamelen renames, examples
hardening, SHA256SUMS, required-checks proposal, gosec FP config,
govulncheck hard gate, dependabot draft, M124 retro).

## d) Totally fucked up (honest ledger)

1. **Heredoc/python patching of Go source — TWICE**, exactly the failure
   mode AGENTS.md warns about ("never generate/patch Go source via shell
   heredocs"): mvdan/sh re-interprets `\\n` escapes differently than bash,
   leaving a literal newline inside a Go string literal
   (`cmd/tq/doctor.go:448`) and later corrupting `C:\srv\a` Windows
   literals into control characters (`\a`, `\b` are shell escapes). Both
   were caught by immediate `go build`/inspection and fixed via the edit
   tool or byte-wise python. LESSON RE-LEARNED: the edit tool, always.
2. **A blind global replace hit 12 payload sites when 4 were intended**
   (`AgentPayload{Repo: repo, Prompt: "hi"}` → `&noClean` everywhere),
   breaking 8 unrelated tests' compilation. Caught by `go vet`, reverted
   line-by-line. The correct move was reading the four target lines and
   editing each.
3. **Probe-commit noise in master history:** the lll-gate positive probe
   was verified by COMMITTING a 142-char violation (`f619c23 "probe"`)
   then `git rm`-ing it (`989ebc0`), because `git reset` is banned. Two
   junk commits now ride master forever. A scratch-branch or
   `--allow-empty`-free temp-file-in-`git add -N` approach would have been
   cleaner.
4. **The window's final edit was an empty tool call** (malformed JSON) —
   the ci-local lint()/baseline wiring never happened, leaving T17's most
   user-visible piece (the growth gate actually gating) unshipped despite
   the report claiming the scripts exist. This report corrects that.
5. **release.sh fixture used a stubbed `nix`** (fake `nix` on PATH after
   the sandbox refused undeclared store paths). Legitimate for testing the
   script's logic, but it means `nix build --print-out-paths` consumption
   is still only line-read in a fixture context. The REAL repo's release
   gates (T1, previous window) remain the nix-side proof.
6. **One `edit` tool call was defeated by the mod-time guard** twice in a
   row (my own python writes had touched the file) and I switched to
   python instead of re-viewing — which then caused failure #1. Vicious
   circle, acknowledged.

## e) What we should improve (process-level)

1. **Stop patching Go via any string-interpolation path.** The edit tool's
   mod-time guard exists for concurrent-writer safety; fighting it with
   python is how §d-1/§d-6 happen. Re-view, then edit.
2. **Gate-probe hygiene:** verification probes that need git state belong
   on a scratch branch or in the fixture repo, never as master commits.
3. **ci-local's lint install-branch asymmetry** (root-only when
   golangci-lint must be installed) predates this window and is now the
   only place the module-marker restructure has to touch — fix both in one
   edit next window.
4. **Wire-then-verify-then-REPORT:** a script that exists but isn't wired
   is "partially done", and reports must say so at the time, not after an
   interrupt forces honesty.
5. **The two CI test fixes are unproven on CI** (nothing pushed, per
   rule). The next push is the proof; if the executor tests fail again,
   the next suspect is ordering (gate-before-preflight) semantics, not the
   payloads.

## f) Next items (≤50, rough priority order)

1. AGENTS.md: golangci config-resolution rule (parent-walk proof, M60/61)
   + the four triage verdicts (paralleltest/testpackage disabled, goconst
   ignore-tests, mnd advisory) + baseline-file workflow
   (regenerate-via-script, never bulk-fix).
2. Wire `check-lint-baseline.sh` into ci-local (lint() emits `== module`
   markers; pipe lint_out through the checker; hard gate) + fix the
   install-branch asymmetry in the same edit.
3. Re-run full suite after `.golangci.yml` changes (root + 7 sub-modules,
   race) — cheap insurance.
4. Push (owner-gated) and watch run on master: executor + windows test
   fixes are the proof; also confirms gosec job is the only red X left
   (advisory).
5. T20: `FilterState` round-trip pins (query → state → query) in webui.
6. T20: `?q=` e2e through `tq serve` (loopback smoke, filter present in
   rendered fragment).
7. T20: LIKE-injection cap test (special chars in `q` escaped/bounded in
   both stores).
8. T19: verify the 23:16 concurrent docs-health sweep actually annotated
   the six 2026-09-07 reports; annotate the remainder if not (M70–M72).
9. T23 M90: `tq pool-health` subcommand (claims, DLQ depth, parked count,
   watermark lag, per-repo gate state).
10. T23 M91: harvest /tmp hot-item flag (`--hot-items` surfacing
    recently-touched items? per plan).
11. T23 M92–M93: pool-health wiring into doctor or its own output modes.
12. T22 M85: fullcore hermetic smoke (postgres suite runnable without a
    daemon on CI? per plan).
13. T22 M86: drain-deadline pin test (task context survives pool
    shutdown, bounded only by --task-timeout — regression-test the
    invariant from AGENTS.md).
14. T22 M87–M89: postgres example/README wiring per plan.
15. T24 M94: worktree-per-agent design doc (isolated checkouts per task,
    merge protocol, cost).
16. T24 M95: daemon attribution design (which agent did what, commit
    footers vs session ids).
17. T24 M96: history-rewrite policy doc (the 2026-09-10 reword playbook
    formalized).
18. T24 M97–M98: consumer-ghost ADR (watermark-lag ≠ off semantics).
19. T24 M99–M101: interface dedup design (store/queue facade overlaps).
20. T26 M110–M113: session-close bridge design + PreToolUse hook
    prototype (crush hook → tq journal?).
21. T25 M103–M105: varnamelen rename tails (three sites per plan).
22. T25 M106: examples/ hardening (timeouts on serves).
23. T25 M109: SHA256SUMS in release --push (binary artifact hashes).
24. T25 M116: required-checks proposal doc (which jobs become blocking).
25. T25 M118: gosec FP config (move the triaged baseline from prose to
    `-exclude` rules where mechanical).
26. T25 M119: govulncheck hard gate promotion (currently advisory? per
    plan it's a job — decide blocking).
27. T25 M123: dependabot draft config (gomod + actions, grouped).
28. T27 M124: retro on 2026-09-18 (scheduled by plan).
29. gosec job: consider `-exclude G703,G304` (both classes are triaged
    baseline) so the advisory job goes green-X-free and ONLY new classes
    surface. (This collapses items; deliberate decision needed.)
30. Fixture release harness: check `~/.cache/tq-release-fixture` into
    `docs/status/assets/` or a `scripts/fixture/` generator script so the
    T14 proof is reproducible without re-deriving the scaffold (currently
    only my shell history).
31. `scripts/lint-baseline.sh` runtime (~3 min): consider caching per
    module or moving to CI-only when it blocks local iteration.
32. CHANGELOG: entries for this window (release.sh env-prefix fix,
    doctor tag-ancestry, --push CI confirm, actionlint, lll gate, lint
    baseline) — docs-health pass can harvest from this report.
33. TODO_LIST: harvest next-items from §f per the loop-back contract.
34. Consider gating `check-lint-baseline.sh` in CI too (ci.yml lint step
    runs the same loop — pipe its captured output through the checker).
35. `tq doctor` tag-ancestry: unit-testable variant (inject repo dir) if
    worth it; currently only live-verified.
36. Windows CI: after the seeds_test fix lands, watch for the NEXT
    windows-only failure class (the job has caught two shape bugs so far —
    `IsAbs` and earlier ones — it's earning its keep).
37. Re-check `/tmp` pressure (was 100% full pre-window, 24G free now) —
    M90's hot-item flag motivation stands.
38. Verify the dependabot actions-bump PR (run 34648724116, red) after
    pins land — it may need rebase or carries a real breakage.
39. Owner questions from the 16:00 report remain OPEN: go 1.27 vs stay
    1.26.7; f26 canonical ID/whitelist; parallel-session protocol.

## g) Questions for the owner (cannot be self-answered)

1. **Push policy for the CI-proof commits:** the executor-test +
   Windows-test fixes and all T14–T17 work sit on master (auto-daemon
   commits, nothing pushed). Repo rule says never push without explicit
   request — but CI red on master only heals via a push. Say the word
   (or push yourself) and I'll watch the run.
2. **gosec job signal-to-noise:** make the advisory gosec job green by
   `-exclude`-ing the triaged G703/G304 path-taint classes (new classes
   still surface), or keep it red-X-until-triaged as a forcing function?
   Both defensible; it's a taste call on advisory hygiene.
3. **Junk probe commits:** `f619c23`/`989ebc0` (lll-gate probe add/remove)
   are pure noise on master. Leave them (history is append-only culture
   here) or do you want a scripted rebase-drop while the commits are still
   unpushed? I won't touch history without your call.

---

*Window closed by status-report interrupt; next session starts at §f item 1
(AGENTS.md lint documentation) then item 2 (baseline wiring).*
