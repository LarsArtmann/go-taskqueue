# Status: Master CI Green — CI Policy Decided, Tooling Debt Surfaced

**Date:** 2026-09-07 19:33 CEST
**Session scope:** Post-round-3 follow-up. Resumed from a stale session
summary ("all done, waiting on push authorization"), found reality diverged
(master already pushed, CI RED on 4 consecutive runs), diagnosed all three
root causes, fixed them, pushed, verified master CI green end-to-end.
**Format note:** skill default is a styled HTML report; the owner explicitly
requested `.md`, so this file is Markdown. Flagged per skill spec, not
propagated as a new default.

**Evidence anchors:** commits `132554e` (CLI refactor), `61d2044` (bridge ctx
fix), `94bff0f` (CI/docs/flake), `af848cd` (doc-refs allowlist), `16ce98c`
(CHANGELOG policy) — CI runs `34147286516` and `34147397689` both **success**
(test + nix jobs, all steps green), first green master since the lint step
existed.

---

## Self-Review (brutal)

**What did I forget?**

1. **I introduced a small behavior regression** and only caught it during
   this review: `startWatermark(ctx)` (`internal/bridge/papdashboard/papdashboard.go:159`)
   now reads the journal under `Run`'s ctx. If ctx is already cancelled at
   `Run` entry, `Facts(ctx, 0)` errors and `Run` returns the misleading
   `"papdashboard: cannot read journal head"` error — before my change it
   read under `context.Background()`, succeeded, and returned a clean `nil`
   on the first ctx check. Rare path, no test covers it, bridge tests pass —
   but it is a regression I shipped while "fixing" a lint finding. The fix is
   a 3-line `ctx.Err()` guard; deliberately NOT coded now (owner said report
   then wait). Filed in (f) item 2.
2. **The previous session's report contained false gate claims and I almost
   echoed them.** The inherited summary said "ALL GREEN including nix flake
   check, not pushed". Both claims were stale-wrong. I re-verified before
   repeating them (good), but I only did so because "verify the summary" was
   on my todo list — not as a reflex. It must be step 1 of every resumed
   session.
3. **I did not verify that advisory lint findings actually surface as CI
   annotations on a green run.** The green runs show only the Node
   deprecation annotation in what I looked at; if annotations get swallowed
   on `continue-on-error` steps, the "findings stay visible" claim in
   AGENTS.md/CHANGELOG is half-true. Unverified → listed in (b).
4. **No test pins the CLI behaviors I refactored.** `cmdAudit`'s output,
   `main`'s dispatch, `splitRepos` edge cases — zero CLI-level tests existed
   or were added. My "verified" claim rests on the package race suite +
   smoke, which exercise none of that output surface.

**What is stupid that we do anyway?**

5. **CI steps run sequentially behind a gate that was red for days** — the
   lint step blocked harvest-guard, webui smoke, and doc-refs from ever
   running, which is how a latent stdlib false positive lived unseen. One
   red blocking step concealed three other problems.
6. **~2 minutes of every CI run spent computing a result we decided to
   ignore** (400-finding lint baseline, now advisory). Known cost, paid
   every push.
7. **The templ LSP poisons every tool response** with 57 errors / 145
   warnings that are all false positives (`go build ./...` is green).
   Every session re-derives "the LSP lies, trust the CLI" from scratch.
8. **Multi-agent + auto-commit daemon all direct-push master.** This session
   my own work landed as three `chore: auto-commit` blobs (logical units
   shredded), and `0c3bf86` was pushed by someone that isn't me and can't be
   attributed. Push-then-verify means CI is a smoke detector, not a gate.

**Did I lie?** No. But the summary I inherited contained lies (green gates,
push state) and my first reply would have repeated them had I not checked
`git ls-remote` + `gh run list` before writing.

**Ghost systems?** One: the dead `catchups` map in `cmdAudit` (write-only,
never read — a leftover from a planned output feature). Removed in
`132554e`. Nothing else found; nothing built this session that isn't wired.

**Split brains?** The old status report vs. reality (fixed via inline
annotation). `CONTRIBUTING.md`'s gate list vs. new advisory-lint reality:
unverified — possible residual drift (b).

**Scope creep?** Resisted: did not fix the other 21 cyclop findings, did not
harvest TODO_LIST, did not start Phase D, did not code the ctx regression
fix. Each is a filed decision, not an omission.

**Removed something useful?** `catchups` — verified dead (full-file read,
87 lines, write-only) before removal. `context.Background()` in
`startWatermark` — removal was the point, but see regression above.

---

## a) FULLY DONE

| Work | Evidence |
| ---- | -------- |
| Diagnosed why master CI was red ×4 (three independent root causes: lint baseline, treefmt vs `*_templ.go`, latent doc-refs false positive) | `gh run view 34145174624`, local `golangci-lint` (v2.13.2 = CI version), CI nix job log |
| CLI complexity refactor: 14-case switch → command map (`main` 18→6), `cmdAudit` 16→8 via `printDriftReport` extraction, dead `catchups` map removed, `splitRepos()` dedupes 3 copies of repo-list parsing | `132554e`; `golangci-lint run ./cmd/...` no longer flags `main`/`cmdAudit` |
| Real context bug fixed: papdashboard watermark init now inherits `Run`'s cancellation instead of `context.Background()` | `61d2044`; contextcheck finding gone on fresh targeted lint; bridge tests `-count=1` pass |
| Lint policy resolved: CI lint step advisory (`continue-on-error`), aligned with AGENTS.md's documented policy; contradiction removed from AGENTS.md, CHANGELOG made honest | `94bff0f`, `16ce98c`; CI green with lint step ✓ |
| treefmt gate fixed for templ: `*_templ.go` excluded, `enableTempl = true` adds `templ fmt` gate for `.templ` sources (verified both files templ-fmt-clean), manual `pkgs.templ` devShell entry deduped | `94bff0f` flake.nix; local `nix flake check` → "all checks passed"; CI nix job green |
| Doc-refs false positive allowlisted (`encoding/json/v2` stdlib path) with provenance comment | `af848cd`; `./scripts/check-doc-refs.sh` → "doc refs ok" |
| Full local gate battery green BEFORE push: build, vet, `GOOS=windows` build+vet, gofmt, `go test ./... -race -count=1` (12 pkgs ok, raw exit code 0 — re-run after catching my own `| tail` pipeline masking), webui smoke (stats/fragments/SSE ok), doc-refs, `nix flake check` | session transcript; CI then confirmed independently |
| Pushed and watched CI to green: two consecutive successful runs (test job 9/9 steps, nix job 6/6) | `gh run list` → `34147286516` success, `34147397689` success |
| Round-3 plan annotated **EXECUTED** with status-report cross-link | `94bff0f` (plan doc diff) |
| Previous report's false-green claims annotated non-destructively (per docs-health ANNOTATE) | this session, uncommitted at report time — committed together with this file |
| Todo list synced to reality (W01–W14 completed; session tasks tracked to completion) | todos tool |

## b) PARTIALLY DONE

| Work | What works | What remains | Effort |
| ---- | ---------- | ------------ | ------ |
| Lint remediation | `main`, `cmdAudit` fixed; papdashboard contextcheck fixed | 4 more cmd/tq cyclop (`cmdHarvest` 17, `cmdStats` 14, `cmdDLQ` 13, `aggregateTop` 16) + ~21 repo-wide cyclop + ~370 other baseline findings; advisory so nothing forces it | L (baseline) / M per function |
| Advisory lint visibility | Step goes ✓, findings computed | Unverified that annotations actually surface on green runs (`continue-on-error` + annotation caps); if not, need reviewdog or scoped lint | S to verify |
| Formatter story for docs | dprint in devShell; treefmt gates Go+templ+nixfmt | dprint NOT wired into treefmt check — markdown formatting is un-gated; intent unclear (round-2 report said "treefmt owns Go, dprint owns docs") | S |
| nix gate coverage | `nix flake check` green on x86_64-linux (eval covers darwin/aarch64) | `--all-systems` builds never exercised; nix-built binary not smoke-tested post-`enableTempl` (checks.build compiles+tests it, but nothing runs `result/bin/tq`) | S |
| ~~Doc consistency for lint policy~~ | ~~AGENTS.md, CHANGELOG, ci.yml aligned~~ | ~~`CONTRIBUTING.md` gate list not re-read against advisory reality~~ done 2026-09-07 (gate list updated: web UI smoke, advisory lint note, templ generate) | ~~S~~ |
| ~~Next-steps backlog~~ | ~~Previous report's ~50 items + this report's (f) list written~~ | ~~HARVEST into TODO_LIST.md/ROADMAP.md blocked on owner decision~~ done 2026-09-07 (owner-directed docs-health run; open items routed, owner-gated ones BLOCKED) | ~~M~~ |
| CHANGELOG | Honest Changed entry for lint policy | `[Unreleased]` keeps growing; no version cut anywhere in sight (repo is pre-v0.1.0) | M |

## c) NOT STARTED

All deliberate, none forgotten mid-flight:

- **Phase D of round-3** (W15–W22 polish/hardening of the web UI) —
  plan design says tracked-not-scheduled.
- **Lint endgame program** — trim `.golangci.yml` to an enforceable set and
  re-gate, vs. burn-down per-package-on-touch, vs. permanent advisory.
  Needs a decision (see g) before any work starts.
- **Release engineering** — v0.1.0 cut (CHANGELOG → release notes → tag).
- **Branch protection / PR flow** for multi-agent master — needs decision (g).
- **Local CI replicant** (`scripts/ci-local.sh`) — run the exact ci.yml
  sequence before pushing. Would have prevented ALL of this session's
  failures at the original push.
- **templ LSP repair** (or documented suppression) — root cause not even
  investigated this session.
- Windows CI smoke for `serve` (smoke script is POSIX-only).

## d) TOTALLY FUCKED UP

1. **My own regression, shipped this session** (found in self-review, not by
   any gate): cancelled-at-start ctx makes `Bridge.Run` return the misleading
   error `"papdashboard: cannot read journal head"` instead of a clean `nil`.
   Severity: LOW (rare path, bridge logs it, retries don't exist at that
   stage). Root cause: mine. Mitigation: 3-line `ctx.Err()` guard + a test
   that cancels ctx before `Run`. Not yet applied — owner said report+wait.
2. **The previous report's "verified" claim was false and sat in git for an
   hour as a lying document** — `nix flake check` had never seen the
   committed `*_templ.go` files (git-add timing), so "green" was measured on
   a different tree than the one pushed. Root cause: no rule forcing nix
   checks to run against a fully-tracked tree. Mitigation: annotated the old
   report; rule proposed in (e).
3. **CI's step ordering concealed three failures for days.** Lint (red,
   blocking) sat BEFORE harvest-guard, webui smoke, and doc-refs, so those
   steps' own problems (the stdlib false positive was already live) never
   got signal. Root cause: nobody noticed the lint step had silently turned
   the rest of CI into theater. Mitigation: lint is now advisory; ordering
   is defensible post-fix, but a green "Lint ✓" on a 400-finding baseline is
   still cosmetic.
4. **The lint config is aspirational fiction.** ~100 linters enabled, ~400
   open findings, max-issues caps truncate reporting. It documents a quality
   bar the codebase doesn't meet and now enforces nothing. Not fucked as in
   broken — fucked as in the config lies about the repo's actual contract
   until an endgame is chosen (g).
5. **Multi-agent + daemon direct-push master with push-then-verify.** This
   session: 3 logical units of work shredded into `chore:` blobs by the
   daemon mid-task, and an unattributable push of `0c3bf86` that flipped
   "awaiting push authorization" into "live and red" without anyone deciding.
   Severity: process risk (race conditions between agents pushing, no
   pre-landing gate, forensics impossible). Mitigation = (g) question 3.
6. **I repeated a documented anti-pattern.** `go test ... | tail -5` in an
   `&&` chain masks the test exit code — this exact pipeline-masking lesson
   is written in the global AGENTS.md. Caught it myself, re-verified with
   raw exit codes (`test-exit=0`, 12×`ok`, 0×`FAIL`), but the mistake was
   free real estate for a false green. Also burned a cycle on `rg -rn`
   (display-replace flag, not "recursive").

## e) WHAT WE SHOULD IMPROVE

1. **Resume protocol: ground truth before anything.** Every resumed session
   starts with `git ls-remote`, `gh run list`, `go build` — in that order,
   before reading any summary or file. This session's summary was wrong on
   both claims that mattered; only external state told the truth.
2. **Ban filters on verification commands.** `| tail`, `| head`, `| rg` on
   gates without `set -o pipefail` manufacture false greens. Rule: raw exit
   code first, pretty output second (or `cmd; echo exit=$?`).
3. **`scripts/ci-local.sh`** replicating the ci.yml test job exactly (vet →
   build → windows → race → gofmt → advisory-lint-report → harvest guard →
   smoke → doc-refs) plus `git add -A && nix flake check`. One command, run
   before every push. Highest leverage item in this report.
4. **Nix checks only against a fully-tracked tree.** Post-run assertion:
   `git status --porcelain` empty AND `test -x result/bin/tq` (the old
   report already proposed the binary-runs check — still not implemented).
5. **Commit logical units immediately after verification** — beat the daemon
   to the punch so history tells the story (3 chore blobs this session).
6. **Annotate stale reports the moment they're known-stale** — done here for
   the 18-41 report; make it reflexive: any report contradicted by later
   evidence gets an inline ANNOTATION block, not silence.
7. **Lint burn-down mechanics** (post-decision): never fix baseline classes
   en masse; when a function is touched for real reasons, leave it
   findings-clean; track per-linter counts in a checked-in baseline file
   (`golangci-lint` output format) so the number can only go down.
8. **Kill or fix the templ LSP noise** — investigate gopls/templ-lsp
   coexistence (likely gopls ignoring `*_templ.go` or missing templ LSP
   registration); until fixed, document "LSP webui diagnostics are false
   positives; trust `go build`" in AGENTS.md Known Issues.
9. **CI minutes hygiene**: `concurrency:` group to cancel superseded runs
   (daemon pushes bursts), and consider moving advisory lint to a scheduled
   job so every push doesn't recompute a known 400-finding result.
10. **Actions housekeeping**: checkout/setup-go pin Node-20-era versions —
    deprecation warnings on every run.

## f) Top 40 things to get done next

Ranked by impact. HARVEST note: TODO_LIST.md is agent-pool food — routing
needs owner sign-off (question g1). Impact: Critical/High/Medium/Low.
Effort: S <30min, M 30min–2h, L >2h.

| # | Task | Impact | Effort | Category |
| - | ---- | ------ | ------ | -------- |
| 1 | Write `scripts/ci-local.sh` replicating the full CI sequence (incl. tracked-tree nix check) and wire it into AGENTS.md as the pre-push gate | Critical | M | Quality |
| 2 | Fix `startWatermark` regression: guard `ctx.Err()` after init so a pre-cancelled ctx returns nil, with a test cancelling ctx before `Run` | High | S | Bug |
| 3 | Decide lint endgame (g2), then execute: trim config + re-gate OR baseline-file burn-down OR keep advisory | Critical | M | Quality |
| 4 | Add `checks.nix-binary-runs` flake check executing `result/bin/tq --help` (kills the empty-output footgun the old report nearly shipped) | High | S | Quality |
| ~~5~~ | ~~Decide TODO_LIST harvest policy (g1), then HARVEST this report's (f) + the 18-41 report's 50 items into TODO_LIST/ROADMAP with routing rigor~~ done — executed 2026-09-07 (owner-directed docs-health AUDIT: TODO_LIST rebuilt with verified open items, ROADMAP synced) | ~~High~~ | ~~M~~ | ~~Documentation~~ |
| 6 | Verify advisory-lint annotations actually surface on green CI runs; if not, adopt reviewdog or a scoped-lint step | Medium | S | Quality |
| 7 | Decide master push workflow (g3): branch protection + PR, or documented direct-push acceptance | High | S | Process |
| ~~8~~ | ~~Cut v0.1.0: finalize CHANGELOG Unreleased, tag, verify `nix build` artifact runs (go-release lifecycle)~~ done — v0.1.0 already shipped 2026-09-06 — the real next cut is v0.2.0 (TODO_LIST, owner-gated) | ~~High~~ | ~~M~~ | ~~Release~~ |
| 9 | Fix remaining cmd/tq cyclop: `cmdHarvest` 17, `aggregateTop` 16, `cmdStats` 14, `cmdDLQ` 13 (same extraction pattern as this session) | Medium | M | Quality |
| 10 | Add CLI-level tests: golden-output test for `tq audit` drift report, dispatch test for unknown-command/help exit codes, table tests for `splitRepos` (empty, spaces, trailing comma) | Medium | M | Quality |
| 11 | Investigate and fix the templ LSP false diagnostics (57 errors/145 warnings); document the resolution or the "ignore LSP, trust CLI" rule in AGENTS.md | Medium | M | Quality |
| 12 | Wire dprint into treefmt (markdown/json gate) or formally decide docs formatting stays manual | Low | S | Quality |
| 13 | Add `concurrency:` group to ci.yml to cancel superseded runs (daemon pushes bursts of commits) | Medium | S | Process |
| 14 | Scope advisory lint to changed packages in CI (diff-based) or move it to a scheduled weekly job to stop paying ~2 min per push | Medium | M | Process |
| 15 | Run `nix flake check --all-systems` (eval + darwin/aarch64 build coverage) and record result | Medium | M | Quality |
| 16 | E2E-run the nix-built binary through `scripts/smoke/webui.sh` (smoke currently only exercises the `go build` binary) | Medium | S | Quality |
| 17 | Triage 32 gosec findings: real issues vs. false positives; fix reals, document exclusions with provenance | Medium | M | Quality |
| 18 | Introduce sentinel errors per package (`errors.go` convention, `errors.Is`-compatible) to burn down the 32 err113 dynamic-error findings | Medium | L | Quality |
| 19 | Extract magic numbers in cmd/tq (mnd: 36 findings; poll intervals, HTTP codes, truncation lengths) into named constants | Low | S | Quality |
| 20 | httpserve security pass per ADR-0003: CSP/X-Content-Type-Options headers, `/api/events` backpressure limit, document read-only guarantee test | Medium | M | Security |
| 21 | Web UI scale test: dashboard snapshot + SSE burst with a 100k-task DB (fragment size, render latency, memory) | Medium | M | Feature |
| 22 | Extend webui smoke: task detail page (`/task/{id}`) and URL filter round-trip assertions | Medium | S | Feature |
| 23 | SSE reconnect-storm test: N clients, reconnect with same Last-Event-ID during burst (hub fan-out correctness) | Medium | M | Quality |
| 24 | Worker graceful-shutdown E2E under SIGTERM in CI (in-flight task completes + records outcome — the invariant AGENTS.md calls out) | Medium | M | Quality |
| 25 | Add govulncheck to CI (binary already in the flake devShell) | Medium | S | Security |
| 26 | Enable dependabot for GitHub Actions versions + go module updates | Medium | S | Process |
| 27 | Upgrade pinned actions past Node 20 deprecation (checkout, setup-go) | Low | S | Cleanup |
| 28 | `tq version` subcommand printing the ldflags-injected version (verify `main.version` is actually wired) | Low | S | Feature |
| 29 | Fuzz `unwrapCommand` payload shapes (raw/JSON string/`{"cmd":...}`/hostile input) | Medium | M | Quality |
| 30 | Windows smoke variant of webui.sh or a CI skip-with-reason (currently POSIX-only) | Low | M | Quality |
| 31 | ADR-0004: lint policy decision record (advisory rationale, endgame options, what would re-gate it) | Low | S | Documentation |
| ~~32~~ | ~~docs-health VERIFY sweep: FEATURES.md claims vs. post-round-3 reality (serve row, smoke row)~~ done — docs-health AUDIT executed 2026-09-07 (all six living docs re-verified against code) | ~~Medium~~ | ~~M~~ | ~~Documentation~~ |
| ~~33~~ | ~~ROADMAP sync: web UI raw idea → shipped (point at FEATURES/webui), promote Phase D items to raw-idea status~~ done — ROADMAP synced 2026-09-07 (shipped raw ideas pruned, new ideas + open questions routed) | ~~Low~~ | ~~S~~ | ~~Documentation~~ |
| 34 | Nightly `-race -count=3` full-suite job (flake-catching for the race gate) | Low | S | Quality |
| 35 | `tq top --json` stability contract test (agents consume it; pin the shape) | Low | S | Quality |
| 36 | dlq rescue output UX: confirm `--rescue-all --older-than` prints a plan before enqueueing | Low | S | Feature |
| 37 | Agent-pool budget telemetry → papdashboard alert (pool spending visible in the ops dashboard) | Low | M | Feature |
| 38 | docs/status/README.md index: list reports newest-first, mark superseded ones | Low | S | Documentation |
| 39 | Quiet the `golangci_lint_ls` channel in Crush config (it duplicates the CLI's baseline noise into every tool response) | Low | S | Process |
| ~~40~~ | ~~CONTRIBUTING.md gate-list review against advisory-lint reality (b) + explicit "run ci-local.sh before push" instruction~~ done — CONTRIBUTING gate list reviewed and updated 2026-09-07 (web UI smoke, advisory lint, templ generate) | ~~Medium~~ | ~~S~~ | ~~Documentation~~ |

## g) Questions I cannot figure out myself

1. ~~**Harvest policy:** May I route this report's (f) list (and the 18-41~~ done (answered 2026-09-07 — owner directed the docs-health AUDIT/HARVEST run; verified items routed into TODO_LIST.md/ROADMAP.md, owner-gated ones carry the BLOCKED marker (now enforced in code))
   ~~report's 50 items) into `TODO_LIST.md` / `ROADMAP.md`, and which of them~~
   ~~may the agent pool pick up autonomously? TODO_LIST is machine-consumed~~
   ~~pool food — every routed item is real compute spend, and I can't decide~~
   ~~your budget. (Tried: re-reading the guardrails; the guardrails say *how*~~
   ~~to route, not *what's funded*.)~~
2. **Lint endgame:** The `.golangci.yml` enables ~100 linters with ~400 open
   findings — currently an advisory annotation stream. Which endgame do you
   want: (a) trim the config to an enforceable set and make it a hard gate
   again, (b) keep advisory + checked-in baseline file that can only shrink,
   (c) permanent advisory? I measured the baseline and the tradeoffs; only
   you can set the quality bar the config should enforce.
3. **Push workflow:** Master is direct-push by you, multiple concurrent
   agents, and the auto-commit daemon — this session the daemon pushed
   mid-decision and my work got shredded into `chore:` blobs. Do you accept
   direct-push + push-then-verify as the price of speed, or should I add
   branch protection (PRs only, daemon commits via a bot branch)? Both are
   defensible; it's your risk appetite, and switching affects every other
   agent's workflow.

---

*Point-in-time snapshot. When later work makes this stale, ANNOTATE it —
don't rewrite. Section (f) is HARVEST fodder, not a commitment list.*
