# Status: Round 3 Live Web UI — Executed A–C, Verified, Committed (Not Pushed)

**Date:** 2026-09-07 18:41 CEST
**Session scope:** Execution of the round-3 plan
(`docs/planning/2026-09-07_16-25_SUPERB-PLAN-ROUND3-LIVE-WEB-UI.md`): a
read-only, live-updating web dashboard served by `tq serve` — Phases A, B, C
(W01–W14, 14 coarse / 76 fine tasks). Phase D (W15–W22) stays tracked-not-
scheduled by plan design.
**Format note:** skill default is a styled HTML report; the owner explicitly
requested `.md`, so this file is Markdown. Flagged per skill spec, not
propagated as a new default.

---

## Executive Summary

The 1% → 20% scope of the round-3 plan is **implemented end-to-end, gated,
and committed on `master`** (`c367bff` and predecessor commits; the
auto-commit daemon also swept intermediate states into `chore:` commits).
`tq serve` renders status cards, a live task table, the DLQ, per-project
chips, and a fact feed as server-rendered templ fragments pushed over SSE by
a single journal tailer. Every gate the plan defines is green: build, vet,
`go test ./... -race`, `CGO_ENABLED=0`, `GOOS=windows`, `nix build`,
`nix flake check`, the browser-free smoke script (wired into CI), the
ghost-reference guard, and a live run where a worker processed tasks and the
dashboard reflected them within one poll interval.

Two real landmines were found and defused during verification (vendorHash
drift producing an *empty but "successful"* Nix output, and the nixpkgs Go
toolchain lacking the `jsonv2` experiment go-sse needs). Both are documented
in AGENTS.md. One honest caveat runs through this whole report: **master CI
was already red before this round started** (pre-existing `cyclop`/
`contextcheck` lint findings in `cmd/tq` and `internal/bridge`), so "CI
green" in this report means "every local gate green", not "GitHub CI green".
That pre-existing redness is the single most important item in section (e).

---

## a) FULLY DONE

| Item | Evidence |
| --- | --- |
| W01 — ADR-0003 (`docs/adr/0003-web-ui-architecture.md`): read-only projection architecture, decision table with abort paths (templ/go-sse/vanilla JS/stdlib http) | File exists; decisions were followed exactly; guardrails held (zero store/schema/worker changes) |
| W02 — Toolchain: `go-sse v0.6.0` + `ssetest v0.3.0` deps, templ `tool` directive, flake devShell templ, CGO-free proof, `*_templ.go` commit policy in AGENTS | `go.mod`/`go.sum`; `CGO_ENABLED=0 go build ./...` green |
| W03 — `internal/webui` skeleton + `tq serve` (`--addr 127.0.0.1:8090 --db --poll`), `http.Server` with Read/Header/Idle timeouts, graceful SIGINT shutdown, routes `/`, `/task/{id}`, `/api/events`, `/api/stats`, `/static/*` (embedded) | `webui.go`, `handlers.go`, `cmdServe` in `cmd/tq/main.go`; skeleton tests |
| W04 — Single journal tailer (watermark starts at head), hub over `sse.Broadcaster` (buffer 128), burst coalescing, SSE handler with `tick` events, heartbeats, leak-free shutdown | `tailer.go`, `hub.go`; hub fan-out test, live-update test, 3-client race test |
| W05 — Dashboard v1: templ layout with stable fragment ids, status cards, fact feed, vanilla `app.js` (EventSource + patch-by-id + badge + keyboard nav), dark minimal `dashboard.css` | `layout.templ`, `fragments.templ`, `static/`; golden fragment test; live smoke passed (worker processed 3 tasks, dashboard showed 2 completed / 1 dead) |
| W06 — Live task table: id/project/type/status/attempts/age/last-error, severity sort (dead→running→pending→cancelled→completed, age desc), truncation with tooltip | `render.go` (`sortTasks`, `truncate`); tests |
| W07 — DLQ fragment + per-project chips (R/P/D aggregation) | `DeadLetterTable`, `projectSummaries`; DLQ test via `FailPermanent` |
| W08 — Snapshot-on-connect (subscribe BEFORE snapshot — the auditlog no-gap pattern), `Last-Event-ID` accepted, unknown/stale IDs fall back to full snapshot, connection badge live/reconnecting/stale, live `<title>` counts | `handleEvents`; resume test with 100-fact backlog × 3 Last-Event-ID variants |
| W09 — Task detail `/task/{id}`: full record + per-task fact timeline, plain links (no-JS works), 404 shape | `handleTaskDetail`, `factsForTask`; detail + 404 tests |
| W10 — URL filters `?project=&status=&q=` (substring over id/type/project/payload/owner/lastError), filter chips + clear, filters compose with burst re-render per-client (URL is source of truth) | `parseFilter`, `matchesFilter`, `FilterBar`; `TestFiltersNarrowTable` |
| W11 — Polish: keyboard nav (1–4 sections, `/` search), time-ago + title timestamps, empty states everywhere, favicon + title with running/dead counts, responsive CSS, long-payload overflow handling | `app.js`, `dashboard.css`, templ components |
| W12 — Test hardening: SSE e2e (enqueue→claim→dead-letter → fragment assertions), hub race tests, golden fragments, resume-under-load (100 facts), store-closed error paths (500 + zero SSE events) | `webui_test.go` (~14 tests, all green under `-race`) |
| W13 — Docs bundle: README "Watch it live" quickstart, FEATURES web-UI section (6 rows incl. honest PLANNED row for write actions), ROADMAP raw-idea → v0.3 arc, CHANGELOG Added entry, DOMAIN_LANGUAGE (serve/tailer/hub/fragment/projection), AGENTS package row + CLI row + known issues | All six files updated; ghost-reference guard passes |
| W14 — `scripts/smoke/webui.sh` (build → seed → worker+serve → Python HTTP/SSE assertions, CI-safe), wired into `.github/workflows/ci.yml` before ghost-check | Ran green locally; CI workflow updated |
| vendorHash dance | Old hash stale after go.sum changes; faked → got `sha256-aeybRM0eN5dsMuqXnx/i1lkDIxfbQ7ooz7Ys/Amwi3E=` → pinned; `checks.vendor-hash` green |
| New-package lint hardening | Fixed: predeclared shadowing (`max`, `clear`), perfsprint, modernize (minmax, `WaitGroup.Go`, `t.Context`), exhaustive fact-type switch, godoclint (templ header collision), goconst, makezero, mnd (named timeout/rank/length consts), contextcheck in own code (ctx threaded through `renderComponent`), lll, gci/golines via `golangci-lint fmt` |
| Commits | `d030b47` (ADR+toolchain), `cf2275d` (dashboard core; daemon swept the rest), `c367bff` (CI smoke + docs + flake fix + lint). **Not pushed** — awaiting instruction |

## b) PARTIALLY DONE

| Item | State | Missing |
| --- | --- | --- |
| CI health | Every *local* gate green; smoke wired into CI | **Master CI is red from before this round** (see section d) — the new smoke step has never run on a green CI |
| `tq serve --help` UX | Flags work; `dbFlag`-based `--db` supported | Help output is bare `flag` defaults ("Usage of serve:") — other subcommands share this, but serve is the most user-facing one |
| Phase D seeding | W15–W22 documented in plan + ROADMAP arc + FEATURES PLANNED row | Nothing implemented (by design this round), no ADR-level design yet for `--allow-writes`/CSRF |
| `internal/webui` lint | Production code clean of all fixable classes | 78 remaining findings are repo-baseline *test* classes (`noctx` from ssetest helpers, `paralleltest`, `varnamelen`, `wsl_v5`, `wrapcheck`, `testpackage`, 1 `gochecknoglobals` for `allStatuses`, 7 `contextcheck` false-positives from templ-generated internals) — consistent with house baseline, not zero |
| Observability of the serve process itself | slog errors on tailer/hub/shutdown failures | No request logging, no metrics endpoint, no `--verbose` control |

## c) NOT STARTED (planned-deferred, not forgotten)

Phase D per plan: W15 UI write actions (`--allow-writes`, CSRF, confirm) · W16 auth/bind hardening for non-localhost · W17 `/metrics` merged into serve · W18 pagination/windowing (>2k tasks) · W19 budget panel over `internal/budget` · W20 Datastar upgrade · W21 PapDashboard question fan-out compose · W22 multi-node view (needs v0.2 Postgres seam). Also: screenshot asset for README (W13.1 said "screenshot" — text quickstart shipped, no image), harvest of section-f items into TODO_LIST.md (see g/Q2), push to remote.

## d) TOTALLY FUCKED UP

1. **Master CI is red and has been red across the last two pushes before this session** — pre-existing `cyclop` (complexity >12 in `cmdAudit`/`main`/`cmdHarvest`/`cmdStats`) and `contextcheck` (`papdashboard.startWatermark`) lint failures. AGENTS.md still says "golangci-lint is not a CI gate" while `ci.yml` *has* a lint step that fails. That is a doc-vs-reality split brain sitting in our most-trusted file.
2. **Nix failure mode is silent corruption, not an error**: with a stale vendorHash, `nix build` **exited 0** and produced an *empty* store path (no `bin/`, no error in the default output). I initially reported "nix build green" on false evidence. The empty-output symptom is a footgun that will bite every future agent the same way. (Defused for this round by fixing the hash; the swallowing behavior itself is upstream/tooling, not fixed.)
3. **`encoding/json/v2` toolchain skew**: go-sse imports `encoding/json/v2` (Go 1.26 default experiment); the nixpkgs Go builds without it enabled, so the flake binary was broken until `GOEXPERIMENT=jsonv2` was set in build+shell env. Local `go build` never catches this class — only Nix does. Every future pure-Go dep now carries this hidden Nix-only risk.
4. **Process honesty item**: I initially ran `go get` + `go mod tidy` *before* any code imported the deps — tidy removed them, and I had to re-add. Small, but it wasted a cycle and the first `nix build` "success" taught me to distrust tool green-ness: I verified exit codes, not artifacts, twice.

Nothing in the shipped webui code is fucked up: no revert happened, no parallel-agent diff was clobbered, no store/schema/worker semantics were touched (guardrails held).

## e) WHAT WE SHOULD IMPROVE

1. **Make CI truthful again** — either fix the ~200 baseline lint findings (split by package: cmd/tq cyclop is refactor work, not lint-suppression work) or make the lint step non-blocking + remove the AGENTS contradiction. Red CI trains everyone (humans and agents) to ignore CI.
2. **Kill the empty-Nix-output footgun** — add a post-build assertion (`test -x result/bin/tq`) to the flake check / smoke so a swallowed build failure can never read as green again. This session nearly shipped a report claiming a broken build was fine.
3. **jsonv2 tripwire** — AGENTS documents it, but a flake-level check (e.g. try-build a probe import in `checks`) would catch the *next* dep that imports an experiment-gated package before it lands.
4. **Own-code contextcheck vs templ internals** — 7 false positives stem from templ-generated closure internals; consider an upstream issue or a `.golangci.yml` path rule for the call-chain rather than accumulating nolint noise.
5. **`varnamelen`/`wrapcheck`/`noctx` test baseline** — the repo-wide test-linter noise (~500 findings) makes real signal hard to see; a one-shot config decision (enable-with-exclusions vs disable) beats per-agent judgment calls every session.
6. **SSE scale path** — every tick re-renders 5 fragments per connected client (full `List` + full `Facts` scan each time). Fine at ops scale, quadratic-ish at journal scale; the compaction/pagination Phase-D items are the real fix — worth an ADR note *now* while W18 is cheap to design.
7. **Serve needs a story for journal growth** — `Facts(ctx, 0)` scans the whole journal on every snapshot; that's the same cost class as `tq facts`. Fine today, a latency cliff later.
8. **Smoke script port collision** — fixed port 8095 can collide on busy machines; derive a free port instead.
9. **Auto-commit daemon interleaving** — my logical commits got split across daemon `chore:` commits (e.g. dashboard core landed as 2-file commit + daemon sweeps). History is truthful but noisy; for multi-file rounds, consider committing sooner per coarse task.

## f) Up to 50 things we should get done next

*Brainstorm list — most items below the top ~15 are ROADMAP fuel, not commitments; route through docs-health HARVEST with rigor.*

**CI/truth (highest leverage)**
1. Fix master CI: resolve pre-existing `cyclop` findings in `cmd/tq` (extract subcommand helpers) — refactor, not suppress.
2. Fix pre-existing `contextcheck` in `internal/bridge/papdashboard.startWatermark`.
3. Decide lint policy in one place (blocking vs non-blocking + exclusions) and align AGENTS.md with `ci.yml` reality.
4. Add `checks.nix-binary-runs` (flake check that executes `result/bin/tq --help`) so empty-output builds can never pass again.
5. Add `GOEXPERIMENT`/jsonv2 probe check for future deps (fail fast with remediation message).
6. Push this round and watch the CI run including the new webui smoke step.

**Web UI hardening (Phase D seeds, cheap now)**
7. W17 `/metrics` (Prometheus text) merged into `tq serve` — ROADMAP raw idea, ~30 min.
8. W15 design note (not code): write-actions ADR (`--allow-writes`, CSRF, confirm) — unblocks W15–W16/W21 later.
9. W18 pagination/windowing design: server-side `LIMIT/OFFSET` + `Filter` extension, trigger at >2k tasks.
10. W19 budget panel sketch over `internal/budget` projections.
11. Free-port selection in `scripts/smoke/webui.sh` (no fixed 8095).
12. Request logging option for `tq serve` (`--verbose` or slog handler).
13. `tq serve --open` (browser auto-open) using the `cli/browser` module already in the dependency tree.
14. Dark/light theme toggle (CSS variables already isolate colors).
15. SSE `retry:` field hint to the client for faster reconnects.
16. Live-updating task detail page (detail is currently static HTML; a tick-driven refresh would make `/task/{id}` live too).
17. Humanize payload preview in table rows (currently only error truncates).
18. Table column sorting toggles (client-side on rendered rows — no server cost).
19. `aria-live` regions on fragments for screen-reader announcements.
20. Favicon inline SVG polish (current emoji data-URI is a placeholder).
21. Journal-size + watermark stat card ("journal: 12,431 facts") — free from existing snapshot.

**Architecture/debt**
22. Journal compaction design note (facts-first ADR-0001 flagged it; the UI's full scans raise its priority).
23. `Store.List` filter extension (`q` pushdown) so the UI search stops being an in-memory full scan.
24. Evaluate `sse.Replay` + ring buffer (auditlog's EventStore pattern) to replace snapshot-per-tick for high-frequency queues.
25. Extract `internal/webui` loadSnapshot/render pipeline behind a small interface for golden tests without SQLite (speed).
26. Sweep `examples/` READMEs to point at `tq serve` as the production path (examples stay PoC by guardrail, but links should not mislead).
27. `internal/e2e` coverage: subprocess test that drives the real `tq serve` binary (the smoke script does this in bash; a Go e2e would run in `-race`).
28. Benchmark: fragments render cost at 1k/5k/20k tasks to put a number on the W18 trigger.

**Docs/process**
29. Screenshot/GIF of the dashboard for README (the plan's W13.1 named one).
30. HARVEST this list: move items 1–2 (CI) and any accepted web-UI items into TODO_LIST.md per repo policy — with the `.crushrc`/agent-pool caveat (guardrail #4: this repo's TODO_LIST is pool food).
31. Annotate round-3 plan doc: mark Status EXECUTED with commit hashes (docs-health ANNOTATE mode).
32. AGENTS.md: document `scripts/smoke/webui.sh` usage next to the existing CLI smoke snippet.
33. CHANGELOG link the ADR + plan doc from the Added entry (currently only file references).
34. CONTRIBUTING: add "run `go tool templ generate` after editing `.templ`" (only implied today).
35. `docs/DOMAIN_LANGUAGE.md`: mark the new Observation terms with cross-links to ADR-0003.

**Pre-existing project debt noticed in passing (report-only)**
36. `wrapcheck` findings on idiomatic `fmt.Fprintf(os.Stderr, ...)` in `cmdServe` — same class AGENTS already declares out-of-scope; confirm policy covers it.
37. `cmd/tq/main.go` is a 1000+ line god-file with 14 subcommands; the cyclop fixes (item 1) are the natural moment to split per-command files (`cmd/tq/serve.go` etc. — note `audit.go` already exists as precedent).
38. `internal/webui` `loadSnapshot` does two full scans (List + Facts); consider one pass if both grow.
39. `timeAgo` returns bare units ("5m") — detail pages render "5m ago" via separate string; unify to avoid future copy drift.
40. `badgePending` doubles as a stats JSON key — semantically distinct uses share one constant; split if the API ever diverges from CSS classes.

**Bigger arcs (ROADMAP fuel)**
41. W16 auth/bind hardening (token auth, non-localhost docs).
42. W20 Datastar upgrade evaluation (conditional on interactivity growth).
43. W21 PapDashboard question fan-out compose (v0.3 arc).
44. W22 multi-node view (blocked on v0.2 Postgres seam).
45. v0.1.0 release runbook execution (repo is pre-v0.1.0; the web UI is a feature worth shipping with it).
46. flake `checks` for `nix run .#test` parity with CI test matrix.
47. Journal viewer mode in the UI (`/facts?after=` with infinite scroll) — replaces `tq tail -f` for humans.
48. Per-project dashboard pages (`/project/{name}`) reusing the filter pipeline.
49. WebSocket bridge consideration doc (only if bidirectional needs appear — SSE likely suffices).
50. Accessibility pass with a real screen reader beyond `aria-live` (item 19).

## g) Questions I cannot figure out myself

1. **Lint policy is contradictory at the authority level: which wins — AGENTS.md ("golangci-lint is NOT a gate; CI enforces vet+gofmt+tests only") or `.github/workflows/ci.yml` (which runs golangci-lint and fails the build on it)?** Master has been red for two pushes because of this. I can fix the findings or fix the config, but I can't know which one you *want* — that decision shapes every future session's definition of "done".
2. **Should any of section (f) be harvested into `TODO_LIST.md`?** Guardrail #4 of the round-3 plan says this repo's TODO_LIST is deliberately agent-pool food in a repo with no `.crushrc`, so enqueueing items costs real money and collides with concurrent agents — but the skill contract says a status report's next-steps list belongs in TODO_LIST, not entombed here. Policy call: harvest for humans only, harvest for the pool, or keep the plan-doc-only convention one more round?
3. **Push now?** The planning commit was pushed under your explicit instruction; this execution round ends in commits on local `master` (`c367bff` head). I did not push without a fresh instruction — confirm and I'll push (and then watch the CI run, which is also the first real exercise of the new CI smoke step).

---

*Point-in-time snapshot — everything above reflects this session's run
(2026-09-07, ~16:30–18:40 CEST). Verify before building on claims; the
verification protocol (build/vet/test -race/nix/smoke) is cheap to re-run.*
