# Status Report: Web UI Redesign on templ-components ("the ledger & the lamp")

**Generated:** 2026-09-07 23:08 CEST
**Session scope:** full visual redesign of the `tq serve` dashboard on
`github.com/larsartmann/templ-components` v1.14 (triggered by owner verdict:
"ugly, boring, learn from templ-components and use it well").
**Format note:** skill default is a styled HTML report; user explicitly
requested `.md` — override honored, not propagated back into the skill.

---

## a) FULLY DONE

1. **Full UI rewrite on the library.** `internal/webui/layout.templ` and
   `fragments.templ` rebuilt on `layout.Base`, `ThemeToggle`, `StatCard`,
   `Grid`, `Card`, `Table` (Body slot), `Badge`, `EmptyState`, `Eyebrow`,
   `DefinitionList`, `Button`, `Alert`, `Scrollback`. Evidence: build green,
   all webui tests + `scripts/smoke/webui.sh` pass end-to-end (stats →
   fragments → SSE).
2. **Design system landed as code.** `internal/webui/theme.css`: steel-navy
   neutral ramp + signal-cyan accent remapped over the library's `blue-*`
   (status hue language: amber=pending, cyan=running, green=completed,
   red=dead, gray=cancelled), JetBrains Mono identity face, header lamp
   (live/reconnecting/stale), journal scroll pane, kbd styling.
   Evidence: compiled CSS contains all classes; screenshots verified.
3. **Identity font embedded.** JetBrains Mono woff2 subsets (400/500/600)
   under `internal/webui/static/fonts/` — every LAN viewer gets the type,
   zero external services. Evidence: `git ls-files` shows them tracked;
   screenshots render the face.
4. **CSS build pipeline.** `scripts/build-webui-css.sh` + flake app
   `nix run .#webui-css`; minified output `internal/webui/static/app.css` is
   committed and go:embed'ed (same policy as `*_templ.go`); reproducible —
   rerun produces byte-identical output (`git status` clean after rerun).
5. **Dependency wired cleanly.** `github.com/larsartmann/templ-components`
   v1.14.0 (+icons, utils; htmx indirect) in go.mod; vendorHash dance done
   (fakeHash → `got: sha256-xbSEDxrY54nC+QIzsgnB77XEs1q9+FLw79H5x9PZ/eY=`);
   `nix build` + `nix flake check` green (vendor-hash + binary-runs checks).
6. **Domain→visual mapping centralized.** `internal/webui/components.go`:
   `statusBadgeType`, `factTone`, `factLines`, `detailItems`, `detailFacts`,
   `dashboardProps` (HTMX fully suppressed — vanilla EventSource preserved),
   id-tail display (`…48b973b2`) because task IDs are time-prefixed.
7. **Contracts preserved.** All `#frag-*` container ids, `#conn` class
   contract, `#search-box`, section ids, keyboard nav, `/api/*` untouched.
   Evidence: `webui_test.go` skeleton tests unchanged except the asset-path
   update; smoke asserts fragments + SSE.
8. **Visual verification with real screenshots.** Headless Chromium shots of
   dark dashboard, light dashboard, and task detail page; both themes and the
   mono type confirmed rendering.
9. **Docs updated.** ADR-0003 amendment (CSS build policy), AGENTS.md
   (webui-css command, theming convention, templ-components adoption table),
   CHANGELOG `[Unreleased]` Added entry. Evidence: `./scripts/check-doc-refs.sh`
   passes.
10. **Lint hygiene on my files.** Scoped `golangci-lint` clean for webui
    (fixed the findings I introduced: goconst → label constants, golines,
    varnamelen). Advisory baseline untouched by design.

## b) PARTIALLY DONE

1. **LAN availability of the new UI.** Verified locally only (ephemeral
   127.0.0.1:8093 during screenshots, since killed). The owner-facing
   `tq serve --addr 0.0.0.0:8090` (PID 3654482) is STILL RUNNING THE OLD
   BINARY — the LAN dashboard shows the pre-redesign UI until restarted.
   Remaining: restart serve with the new binary. Effort: S. No blocker
   beyond "don't kill the owner's process without a nod".
2. **Testing of the new UI.** Existing skeleton tests + smoke pass, but zero
   NEW unit tests for the new mapping helpers (`statusBadgeType`, `factTone`,
   `shortIDTail`) and no full-page golden/snapshot test. Remaining: add them.
   Effort: S–M.
3. **Design QA breadth.** Desktop (1440px, 1100px) verified in both themes;
   mobile/narrow viewport, keyboard-only focus traversal, reduced-motion
   lamp behavior NOT verified. Effort: S–M.
4. **Doc completeness.** AGENTS/CHANGELOG/ADR updated, but FEATURES.md still
   describes the dashboard generically (no redesign, theming, fonts, CSS
   build row) and TODO_LIST/CONTRIBUTING weren't touched. Effort: S.
5. **Session-adjacent pool state.** The dogfood agent pool (PID 3117483) is
   alive and productive (TODO_LIST open count 21 → 15 this evening; new
   commits `71823c1` audit fix, `082e54b` cmd refactor, `5be4c70` CLI tests,
   `4d48a2d` `worker --once`), but pool persistence (systemd), budget
   decision, and C22 review remain open from the previous report. Effort: M.

## c) NOT STARTED

1. **W16 serve auth** (TODO_LIST item) — blocked on owner posture decision
   (see g2). Still wanted.
2. **CSP headers** — no `Content-Security-Policy` served; ThemeScript/
   ThemeToggle inline scripts run nonce-less because nothing enforces CSP.
   Hardening for the day the UI leaves loopback.
3. **v0.2.0 release** — CHANGELOG `[Unreleased]` is release-shaped and
   owner-gated; no tag cut.
4. **Push to remote** — local commits (redesign + agent work) unpushed;
   no authorization in this session.
5. **`ci-local.sh` full pre-push gate run** — individual gates ran green;
   the full replicant wasn't executed.
6. **Previous report's owner questions** (pool systemd persistence +
   daily budget, LAN auth posture, `--model` pin) — unanswered, unblocked
   only by owner input.
7. **Journal pane click-through** — fact lines are text-only (library
   Scrollback limitation); linking to task detail needs a decision
   (upstream feature vs custom layer). Not started.

## d) TOTALLY FUCKED UP

Nothing currently broken in the tree — all gates green. Radical-honesty
confessions of things I got wrong this session (all found and fixed, listed
so they don't repeat):

1. **Wrong LICENSE file briefly shipped with the fonts.** I copied the
   source repo's PROPRIETARY LICENSE as the font license. JetBrains Mono is
   SIL OFL 1.1 (© JetBrains). Caught and removed within a minute; attribution
   now documented in AGENTS.md instead of a fabricated license file.
   Severity if uncaught: legal/attribution error in a committed artifact.
2. **CHANGELOG edit raced the auto-commit daemon and landed in the WRONG
   release section** (`[0.1.0]` instead of `[Unreleased]` — the same anchor
   text exists twice after daemon restructuring). Fixed: entry now only in
   `[Unreleased]`/Added; verified by grep. Lesson: `view` (not bash `sed`)
   a file immediately before editing when the daemon is live.
3. **Dependency add fumbled.** First `go get` + `go mod tidy` silently
   PRUNED templ-components (nothing imported yet) and I misread the failure.
   Wasted a cycle. Correct order: import → go get → tidy.
4. **Wrote a fabricated placeholder vendorHash into flake.nix** before doing
   the fakeHash dance properly. Never built (replaced within the same step),
   but the pattern "guess first, derive second" is exactly what the AGENTS.md
   known-issue warns against.
5. **Ran gofmt on `.templ` files** (error-spam, wrong tool — `templ fmt`
   owns `.templ` per house rules). No damage; noisy.
6. **Stale UI presented to the owner right now** (see b1): the LAN serve
   process predates the redesign. Not a code bug — an operations miss from
   this session.

## e) WHAT WE SHOULD IMPROVE

1. **`loadSnapshot` loads the ENTIRE journal every burst.**
   `render.go:180` calls `s.store.Facts(ctx, 0)` then slices the last 50 —
   O(journal size) per 500ms tailer burst per the ADR's own "consequences"
   note. Fix: `Facts(ctx, watermark-50)`-style bounded query. Real perf
   issue, cheap fix.
2. **Daemon-edit races on hot files.** CHANGELOG/AGENTS.md edits collided
   with the auto-commit daemon twice this session. Fix (practice): always
   `view` the file in the SAME tool call sequence as the edit; prefer
   targeted `edit` over `python` rewrites for daemon-hot files.
3. **Font licensing needs a real artifact, not prose.** Ship the OFL-1.1
   text in `internal/webui/static/fonts/` (fetch from the jetbrains-mono
   source) so provenance travels with the binary, not just AGENTS.md.
4. **templ-components adoption table is a convention, not enforced.** A tiny
   guard test (or grep in CI) asserting the table lists every
   `templ-components` package imported under `internal/webui/` would stop
   silent hand-rolling drift.
5. **Screenshots were manual one-offs.** A `scripts/webui-screenshots.sh`
   (headless chromium, both themes, desktop+narrow) would make visual QA
   repeatable and reviewable in PRs.
6. **Status reports keep rediscovering running-process state** (pool PID,
   serve PID, ports). A tiny `scripts/tq-session-status.sh` printing live
   PIDs/ports/DB paths would end that per-session archaeology.
7. **New helpers shipped untested** (see b2) — mapping functions are pure
   and trivially table-testable; add them before more UI layers stack on
   top.

## f) NEXT TASKS (up to 50 — brainstorm for HARVEST, ranked by impact)

| #  | Task                                                                                      | Impact | Effort | Category      |
| -- | ----------------------------------------------------------------------------------------- | ------ | ------ | ------------- |
| 1  | Restart `tq serve --addr 0.0.0.0:8090` with the new binary so LAN shows the redesign      | High   | S      | Ops           |
| 2  | Bound the fact query: `loadSnapshot` should fetch only the last N facts, not the journal  | High   | S      | Bug           |
| 3  | Add unit tests for `statusBadgeType`, `factTone`, `shortIDTail`, `factTimestamp`          | High   | S      | Quality       |
| 4  | Push pending commits (redesign + agent work) once authorized; then run `ci-local.sh`      | High   | S      | Ops           |
| 5  | C22 dogfood review: inspect each pool-agent commit for quality/self-mod checks            | High   | M      | Quality       |
| 6  | Hand the pool to `deploy/systemd/tq-agent-pool.service` (survives session death)          | High   | M      | Ops           |
| 7  | Decide + implement W16: serve auth (token?) or narrow bind; document the posture          | High   | M–L    | Feature       |
| 8  | Cut v0.2.0 (redesign + agent-pool features + `--once` + audit fix) after owner go         | High   | M      | Release       |
| 9  | Update FEATURES.md Web UI section (redesign, theming, fonts, CSS build, adoption table)   | Medium | S      | Documentation |
| 10 | HARVEST this report's (f) list into TODO_LIST/ROADMAP                                     | High   | S      | Documentation |
| 11 | Mobile/narrow (390px) screenshot pass + fix any grid/aside stacking issues                | Medium | S      | Quality       |
| 12 | Add CSP header (+ nonces via `layout` nonce plumbing) for the webui                       | Medium | M      | Security      |
| 13 | Journal click-through: link fact lines to task detail (upstream Scrollback links or shim) | Medium | M      | Feature       |
| 14 | Show daily-budget usage (internal/budget projection) as a stat card / progress bar        | Medium | M      | Feature       |
| 15 | Live relative-age refresh (ages/timestamps tick without waiting for a burst)              | Low    | M      | Feature       |
| 16 | Sortable task table via library `DataTable`                                               | Medium | M      | Feature       |
| 17 | Pause-on-hover + "N new facts" jump pill for the journal pane                             | Low    | M      | Feature       |
| 18 | Expandable error detail (popover) instead of 60-char truncation tooltips                  | Low    | M      | Feature       |
| 19 | Keyboard-shortcut overlay (`?`) documenting `/`, `1–4`                                    | Low    | S      | Feature       |
| 20 | Per-project drilldown page (filtered by project, reuse fragments)                         | Medium | M      | Feature       |
| 21 | Rescue/retry affordances behind the future `--allow-writes` flag (ADR-0003 Phase D)       | Medium | L      | Feature       |
| 22 | Webui full-page golden/snapshot tests (light+dark) in the test suite                      | Medium | M      | Quality       |
| 23 | Headless-Chromium screenshot script for repeatable visual QA                              | Medium | S      | Quality       |
| 24 | a11y audit of the new page (focus order, ARIA on lamp/feed, contrast)                     | Medium | M      | Quality       |
| 25 | Verify + document reduced-motion behavior (lamp pulse, stagger)                           | Low    | S      | Quality       |
| 26 | ETag/cache headers for embedded static assets                                             | Low    | S      | Quality       |
| 27 | Ship OFL-1.1 license file with the font subsets                                           | Medium | S      | Cleanup       |
| 28 | Guard test: templ-components adoption table vs actual imports                             | Low    | S      | Quality       |
| 29 | Build script: fall back to `nix shell nixpkgs#tailwindcss_4` when CLI missing             | Low    | S      | Cleanup       |
| 30 | CONTRIBUTING: document the CSS build step for non-Nix contributors                        | Low    | S      | Documentation |
| 31 | Pool metrics on the dashboard: fact-rate sparkline, task duration histogram               | Medium | M      | Feature       |
| 32 | `--model` pin decision (owner) then propagate to payloads/docs                            | Medium | S      | Decision      |
| 33 | Daily-budget value decision (owner) + document in AGENTS.md                               | Medium | S      | Decision      |
| 34 | `tq serve --open` flag (launch browser)                                                   | Low    | S      | Feature       |
| 35 | Update README screenshots section with the new dashboard                                  | Medium | S      | Documentation |
| 36 | Demo video of the new UI via the website-launch flow                                      | Low    | L      | Documentation |
| 37 | SSE client: reconnect jitter/backoff note + verify EventSource retry behavior             | Low    | S      | Quality       |
| 38 | Journal feed pagination/compaction (ADR-0003 known consequence) beyond the N-facts fix    | Medium | L      | Bug           |
| 39 | Consider HTMX upgrade path for filters (ADR-0003 abort path) if interactivity grows       | Low    | M      | Feature       |
| 40 | Vendored-path `@source` evaluation if `vendor/` ever lands (build script robustness)      | Low    | S      | Cleanup       |
| 41 | Dark-mode selection/ring color spot-checks against the cyan remap                         | Low    | S      | Quality       |
| 42 | Move `label*` vocabulary next to the table headers usage (single source check)            | Low    | S      | Cleanup       |
| 43 | Add 700-weight mono subset only if a real use appears (avoid speculative bytes)           | Low    | S      | Cleanup       |
| 44 | Pool agent contract: teach agents the new UI exists for verification (`tq serve`)         | Low    | S      | Documentation |
| 45 | `tq top` TUI parity: budget + project exclusivity columns                                 | Low    | M      | Feature       |
| 46 | og:/theme-color meta polish + per-section deep-link titles                                | Low    | S      | Feature       |
| 47 | Smoke test asserting the compiled CSS contains critical classes (drift guard)             | Medium | S      | Quality       |
| 48 | Retry-backoff visibility: show `notBefore`/next-attempt in the table                      | Medium | S      | Feature       |
| 49 | Consider `RelativeTime` component for human timestamps in the table                       | Low    | S      | Feature       |
| 50 | Clean `/tmp/tq-visual.db` artifacts + remove visual-demo DB from `tq stats` noise         | Low    | S      | Cleanup       |

HARVEST note: items 1–10 are TODO_LIST-grade; most of 11–50 are ROADMAP fuel
(brainstorm, not commitment). Owner-gated decisions: 4, 6, 7, 8, 32, 33.

## g) QUESTIONS ONLY YOU CAN ANSWER

1. **Restart the LAN dashboard now?** `tq serve --addr 0.0.0.0:8090`
   (PID 3654482) is still running the OLD binary — the LAN is looking at the
   pre-redesign UI. I can kill + relaunch it with the new binary in seconds;
   I didn't because it's your long-running process. Say the word.
2. **Auth posture for the LAN-exposed, zero-auth dashboard (W16):** keep it
   read-only + no-auth on the LAN, narrow it back to loopback, or build a
   token/auth story first? This gates both item 7 and the CSP work.
3. **Cut v0.2.0 now?** The redesign plus the pool agents' evening output
   (audit fix, `worker --once`, CLI tests, cmd refactor) make a fat, honest
   release. Tag now, or wait for pool persistence/systemd first?

---

_Prepared by Crush. Point-in-time snapshot — running processes verified live
at generation time (pool PID 3117483, serve PID 3654482)._
