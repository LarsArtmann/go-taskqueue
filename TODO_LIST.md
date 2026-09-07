# TODO List

Short- and mid-term actionable work. Long-term direction lives in ROADMAP.md.

**This file is machine-consumed**: `tq harvest` turns every unchecked item
below into an agent task. Keep the `- [ ]` checkbox format, one item per
line — do not convert to tables. Mark done items `[x]` or delete them;
appending `— BLOCKED: <reason>` keeps an item out of the pool.

Items carry their evidence: a code path and/or the status report that filed
them (`docs/status/<date>_<slug>.md`, cited as `(19:33 report …)` etc.).

## High Impact

- [x] Write `scripts/ci-local.sh` replicating the full CI test-job sequence locally (vet → build → GOOS=windows → race tests → gofmt → advisory lint → harvest-parse guard → web UI smoke → doc-reference check, then `git add -A` + `nix build` + `nix flake check` on a fully tracked tree) and wire it into AGENTS.md + CONTRIBUTING.md as the pre-push gate (19:33 report f1 — a stale-tree nix check and unverified pushes caused the 2026-09-07 red-master incident)
- [x] Fix the papdashboard watermark regression: `Bridge.Run` returns the misleading "cannot read journal head" error when the context is already cancelled at entry because `startWatermark(ctx)` reads under `Run`'s ctx (`internal/bridge/papdashboard/papdashboard.go:112`); guard `ctx.Err()` after init and add a test that cancels ctx before `Run` (19:33 report d1/f2)
- [x] Add a flake check that executes the nix-built binary (`result/bin/tq --help`) so a swallowed build can never produce an empty-but-"successful" store path again (18:41 report f4, 19:33 report f4; `flake.nix` currently has only `checks.vendor-hash`) — done as `checks.binary-runs`, which references the package directly (not the `result` symlink) so `nix flake check` runs it in CI
- [x] Verify that advisory lint findings actually surface as CI annotations on green runs (`continue-on-error` + annotation caps may swallow them); if not, adopt reviewdog or a scoped-lint step (19:33 report b/f6) — done: adopted the scoped-lint option: `scripts/lint-annotations.sh` re-runs the same binary/config with `--new-from-rev` and emits findings on changed lines as `::warning` annotations, wired into ci.yml + ci-local.sh (reviewdog rejected: needs `checks:write`/`pull-requests:write` permission elevation and still caps unscoped); re-verified 2026-09-07 on green run 34157520457: the 5 scoped warnings surfaced as file-bound `[warning]` annotations, but 10 stale baseline `[failure]` annotations also leaked — not from golangci-lint (v2.13.2 emits zero annotation commands, confirmed locally with `GITHUB_ACTIONS=true` on a package with findings) but from the `go` problem matcher actions/setup-go registers, which scraped the advisory step's `file.go:line:col:` text output (410 lines matched, 10-per-step cap kept 10); fixed with `::remove-matcher owner=go::` in the advisory CI step so only scoped new findings annotate
- [ ] Cut v0.2.0: finalize the CHANGELOG `[Unreleased]`, release checklist, annotated tag, GitHub release, then run the nix-built binary as a smoke — additive changes only (web UI, pool hardening), fine for 0.x (19:49 report f2) — BLOCKED: owner go/no-go (note: v0.1.0 already shipped 2026-09-06; the 09-07 reports' "cut v0.1.0" lines were stale)
- [ ] Verify the CQA bridge against a live CQA API instance and fix contract drift (`internal/bridge/cqa` response shapes are httptest-informed guesses today); upgrade its FEATURES.md status after (plan C25) — BLOCKED: needs a live CQA instance URL + owner ID + token from the owner

## Medium Impact

- [ ] Fix `scripts/smoke/papdashboard-e2e.sh` real-dashboard mode: the assertion loops grep `INGEST_LOG`, which only the stub writes, so `PAP_URL=…` mode always times out and fails — make the real mode verify against the dashboard (or split the script per mode) (19:49 report d1/f5)
- [ ] Make `harvest.Audit` report per-repo scan failures instead of a silent `continue` (`internal/harvest/drift.go:100`), and fix the comment above it that claims parity with `Harvester.Run` which does not exist (19:49 report d2/f6)
- [ ] Extract the remaining cmd/tq complexity hotspots the same way `main`/`cmdAudit` were fixed: `cmdHarvest` (17), `aggregateTop` (16), `cmdStats` (14), `cmdDLQ` (13) (19:33 report b/f9; advisory cyclop baseline)
- [ ] Add CLI-level tests: golden-output test for the `tq audit` drift report, dispatch test for unknown-command/help exit codes, table tests for `splitRepos` (empty, spaces, trailing comma) (19:33 report d4/f10 — the CLI refactor shipped with zero CLI-level tests)
- [ ] `tq worker --once`: run until the claimable queue is drained, then exit — parity with `tq agent-pool --once` for scripts and tests (18:34 report f22; the worker still has no one-shot mode)
- [ ] Nightly/weekly fuzz job (60s `FuzzParseRepo`) so corpus growth does not depend on session memory; commit interesting seeds into `testdata/fuzz` (18:00 report f21, 18:34 report f21)
- [ ] Run `nix flake check --all-systems` (darwin/aarch64 coverage) and smoke the nix-built binary through `scripts/smoke/webui.sh` — the smoke only ever exercises the `go build` binary today (19:33 report b/f15/f16)
- [ ] Windows honesty: run the e2e/chaos suite on a real `windows-latest` runner or mark those tests `//go:build unix` so the GOOS=windows compile gate stops implying runtime coverage it does not have (19:49 report f15)

## Lower Impact

- [ ] Add `TestCheckProjectsDir` unit tests (the `--projects-dir /` and `$HOME` refusal guard has manual smokes but no test in CI) (19:49 report f7)
- [ ] D91-lite: capture `crush --version` in the pool startup line and warn when the binary is missing, so flag-contract drift surfaces early (19:49 report f19, seeds D91)
- [ ] Wire dprint into the treefmt gate (markdown/json) or formally decide that docs formatting stays manual — today dprint is in the devShell but un-gated, so docs drift freely (19:33 report b/f12)
- [ ] Sidecar retention: `--log-dir-max-age` / size cap for `TQ_LOG_DIR` output logs, and document that sidecars are plaintext and may contain repo paths (19:49 report f20)
- [ ] Fuzz `ExtractResultPayload` (regex over untrusted agent output) with a seed corpus (19:49 report f18)
- [ ] `tq audit --json` plus `--todo-file`/`--type`/`--max-attempts` flags for parity with `tq harvest` (19:49 report f17)
- [ ] Free-port selection in `scripts/smoke/webui.sh` (fixed port 8095 collides on busy machines) (18:41 report f11)
- [ ] Request-logging option for `tq serve` (`--verbose` or an slog handler) — the serve process currently logs only tailer/hub/shutdown failures (18:41 report f12)
- [ ] `tq serve` auth for non-localhost binds: token auth (or basic) + docs, per plan W16 — the dashboard is now bound to the LAN (read-only by construction, ADR-0003) with zero auth, so anyone on the LAN sees all task payloads and error tails
