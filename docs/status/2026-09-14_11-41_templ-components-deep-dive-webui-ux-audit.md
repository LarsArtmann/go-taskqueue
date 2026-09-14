# Status Report — templ-components deep dive + webui UX audit, 2026-09-14 11:41 CEST

Session scope: user complaint "our UI/UX sucks a bit ass" → loaded all semi-relevant
skills (frontend-design, templ-components, library-deep-dive, html-report-kit,
architecture-review, full-code-review, status-report) → deep study of
`~/projects/templ-components` at v1.17.0 + unreleased → full audit of
`internal/webui` → gap analysis → deliverable
`docs/research/2026-09-14_templ-components-deep-dive.html` (self-contained HTML,
every claim cited file:line, capabilities verified against the pinned v1.16.0 tag
via `git show v1.16.0:…`). No production code was changed this session (read-only
audit by design); the repo's verify gates were therefore not re-run.

---

## a) FULLY DONE

1. **Skill surface loaded** — 7 skills read in full before acting; the templ-components
   authoring playbook (both consumer + author parts) is in context.
2. **Library deep study** — AGENTS.md (383 lines), FEATURES.md, CHANGELOG (v1.16.0,
   v1.17.0, Unreleased: M19–M22 a11y/layout/wire packs), 39 ADRs + 35 recipes
   inventoried; component source deep-reads: `Table` (TypedHeaders/Row.Href),
   `CollapsibleSection` (persistence singleton), `RelativeTime` (AutoRefresh JS in
   `display/shared.go:190-215`), `FilterInput`, `Pagination`, `ListNote`, `CopyButton`,
   plus the SSE-fragments / horizontal-filter-bar / theme-bridge recipes.
3. **Consumer audit** — every `internal/webui` file that matters read: layout.templ,
   fragments.templ (800 lines), components.go, render.go, hub.go, handlers.go,
   static/app.js (263 lines), theme.css (788 lines), adoption_test.go.
4. **Critical finding verified at source** (not inferred): every journal tick re-renders
   and SSE-swaps ALL five fragments including `#frag-filters` (search box) and
   `#frag-table` (open cancel/rescue forms, expanded error cells) — handlers.go:339 →
   render.go:741-757 → handlers.go:395-396 → app.js `applyFragment`. This is the
   concrete mechanism behind "the UI sucks".
5. **Gap analysis** — 16 capability areas ledgered in the report's appendix: 7 fully
   leveraged (theme-bridge exact, Base/SEO shell, table core, badges+one color table,
   charts, empty states, no-JS platform use), 4 partial/duplicate, customs (nowband,
   board) confirmed as documented, guard-tested decisions — the 2026-09-13
   KanbanBoard rejection still holds against v1.17's drag-move rework.
6. **Version-currency verification** — tq pins v1.16.0; every capability cited in the
   findings proven present in that tag; the a11y pack (touch targets, forced-colors
   focus, prefers-contrast, aria-live policy, LoadMore FocusOnSwap, AppShell tokens)
   is unreleased → bump deferred, not urgent.
7. **Deliverable written and self-checked** — docs/research/2026-09-14_templ-components-
   deep-dive.html: no template placeholders left, tags balanced (parser-verified),
   roadmap in impact×effort order, adoption ledger with verdicts.
8. **Post-hoc claim verification** — the report's "500s are bare" claim was flagged as
   unverified during writing and then verified: no `recover`/500 handler exists in
   webui (only the styled 404, handlers.go:514).

## b) PARTIALLY DONE

1. **The original complaint itself** — root cause found and documented, but zero code
   fixed; the dashboard still clobbers operator state on every tick.
2. **Dead code identified, not deleted** — `components.go:134 factLines` is unused
   (LSP-confirmed); deletion folded into the RelativeTime adoption step.
3. **Findings not harvested** — §f below is not yet in TODO_LIST.md (docs-health
   HARVEST pending owner instruction).
4. **Skill catalogue drift noticed, not fixed** — the installed templ-components skill
   table says "118 components"; FEATURES.md/TestDocsCountDrift say 121 primitives +
   4 recipes. Possibly the installed copy vs repo copy divergence; unjudged.
5. **Daemon handoff** — the research HTML sat untracked at session end; index
   conventions handled for THIS report (row added), not for the research file
   (daemon commits both if dirty together).

## c) NOT STARTED

1. All seven roadmap items (swap-guard, sort UI, RelativeTime, Pagination, filter
   submit decision, small wins, version bump) — nothing implemented.
2. Visual/screenshot pass — the frontend-design skill explicitly recommends looking
   at the thing; no `tq serve` render or light/dark/mobile capture was done. The
   entire critique is code-level.
3. Empirical tick-frequency measurement ("every few seconds" is inference from the
   dogfood pool, not a measured number).
4. `./scripts/smoke/webui.sh` not run this session.
5. Upstream ideas (Scrollback link support; Body-slot TableRow.Href) — not filed;
   verify-before-filing gate not even started.
6. Library a11y audit of tq's CUSTOM surfaces (board columns/cards, tq-action
   summaries, viewToggle) against the library's own touch-target/zoom standards.

## d) TOTALLY FUCKED UP

1. **The shipped defect** (master behavior, not this session): live SSE swaps destroy
   typing, open forms, expanded cells, fold state, and focus. On a busy pool the
   dashboard is look-don't-touch. This is what the user felt.
2. **Dead code on master**: `factLines` (components.go:134) — a whole helper orphaned
   when FactFeed went inline; the exact ghost-system class the repo keeps burning on.
3. **My own claims-verification miss, caught in-session**: I wrote "500s bare" into
   the deliverable BEFORE verifying it. It turned out true, but the order was wrong —
   that is the verify-external-claims discipline applied to my own output, and it
   only worked because I flagged the claim in §a8.
4. **Unjudged diagnostics pile**: ~20 gopls warnings visible all session (10+
   go.mod tidy warnings incl. unused requires, hub.go infertypeargs, 2 templ
   QF1003 hints) — noticed, not triaged, not separated into real vs multi-module
   noise. Fix-on-sight was consciously suspended for the read-only audit, but the
   triage itself never happened.

## e) WHAT WE SHOULD IMPROVE

1. **Verify claims before they enter a deliverable**, not after — the 500-claim miss.
2. **Look at the UI, not just the code** — a design critique with zero screenshots is
   half a critique; render + capture should be step one of any UI audit.
3. **Measure before characterizing** — tick frequency, swap latency, feed-scroll
   behavior under swaps all deserve one measured number each.
4. **Declare audit-vs-fix mode explicitly per session** — this session suspended
   fix-on-sight; a one-line note at the top would have prevented the ambiguity.
5. **Separate LSP noise from real findings mechanically** (root-module
   `go mod tidy -diff`, per-module builds) instead of letting 20 warnings blur.
6. **Harvest promptly** — findings that sit only in a timestamped report rot; §f
   should reach TODO_LIST/ROADMAP the same session.
7. **Use the vendored html-report-kit consistently** — done right this time; keep it.

## f) UP TO 50 THINGS TO GET DONE NEXT

P0 = operator pain, direct from the audit; P1 = quality/verification; P2 = ROADMAP
fuel (docs-health HARVEST must route these, not commit them).

| #  | P  | Item                                                                                                                                                                                            |
| -- | -- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1  | P0 | Swap-guard in app.js `applyFragment`: skip a fragment while it contains `document.activeElement`, an open `<details>`, or an expanded `[data-error]`; defer that fragment to the next tick      |
| 2  | P0 | Server-side: skip re-rendering `#frag-filters` unless `FilterState` changed (hash compare in renderFragments)                                                                                   |
| 3  | P0 | Re-apply fold/expanded state after each swap (port CollapsibleSection's re-apply singleton) — fixes settled-fold + error-cell resets                                                            |
| 4  | P0 | Fix feed-scroll reset: `#frag-feed` swap yanks `.journal-scroll` to top mid-read — preserve scrollTop across swap                                                                               |
| 5  | P0 | Sortable task-table headers via `Table.TypedHeaders` (Href from the existing 6-value sort allowlist; `SortDirection` from `data.Filter.Sort`)                                                   |
| 6  | P0 | Adopt `display.RelativeTime` on detail created/updated + fact feed; delete `fmtAge`/`tickAges`/`data-age` plumbing                                                                              |
| 7  | P0 | Delete dead `factLines` (components.go:134) — or in the same commit as #6                                                                                                                       |
| 8  | P1 | Replace hand-rolled `taskPager` with `navigation.Pagination` (numbered pages, ellipsis, rel attrs)                                                                                              |
| 9  | P1 | Owner ruling: may tq re-enable the embedded htmx runtime (zero-request, already in layout.Base) for the filter bar? Then: FilterInput wired to `#frag-table` OR ~15-line fetch-submit+pushState |
| 10 | P1 | `CopyButton` on task IDs (detail header), report path (statusReportCard), prompt/raw payload panes                                                                                              |
| 11 | P1 | Replace "+N more projects" chip with `display.ListNote`                                                                                                                                         |
| 12 | P1 | `errorpage.WriteError` for 500s — verified bare today; keep the chrome-consistent custom 404                                                                                                    |
| 13 | P1 | Harvest §f into TODO_LIST.md (docs-health HARVEST; P2 items → ROADMAP)                                                                                                                          |
| 14 | P1 | Measure tick frequency on the live pool (one number; sizes the swap-guard dwell)                                                                                                                |
| 15 | P1 | Screenshot pass: `tq serve` + light/dark/mobile captures; eyeball before judging visuals further                                                                                                |
| 16 | P1 | Run `./scripts/smoke/webui.sh` green before and after the P0 batch                                                                                                                              |
| 17 | P1 | A11y audit of custom surfaces vs library standards: board cards/columns, `tq-action` summaries, viewToggle touch targets (≥24px @375px)                                                         |
| 18 | P1 | forced-colors: focus outlines for custom links (viewToggle, clear-all, action summaries) — library restores rings; custom CSS needs the same                                                    |
| 19 | P1 | prefers-contrast: remap border grays in theme.css like the library's hardening                                                                                                                  |
| 20 | P1 | prefers-reduced-motion: audit `tq-pulse`, board hover glow, seg text-shadow                                                                                                                     |
| 21 | P1 | Search input a11y: placeholder-only labeling — add sr-only label or AriaLabel                                                                                                                   |
| 22 | P1 | Cancel/rescue POST feedback: verify current redirect UX; if silent, add inline confirmation                                                                                                     |
| 23 | P1 | Journal-browser "load older" button: align markup/a11y with `navigation.LoadMore`/`EndOfList` styling                                                                                           |
| 24 | P1 | Compact-age decision for the table band (keep "3m" vs RelativeTime's "3 minutes ago") — one deliberate call, documented                                                                         |
| 25 | P1 | Triage the gopls pile: `go mod tidy -diff` on root module; hub.go infertypeargs one-liner; 2× templ QF1003                                                                                      |
| 26 | P2 | Track upstream Unreleased; bump templ-components when the a11y pack ships (touch targets, forced-colors, FocusOnSwap, AppShell tokens)                                                          |
| 27 | P2 | Adoption-table rows in AGENTS.md for every component adopted from #5-#12 (guard test enforces)                                                                                                  |
| 28 | P2 | Upstream idea: Scrollback line-link support (tq FactFeed needs it; hand-rolled today) — verify-before-filing first                                                                              |
| 29 | P2 | Upstream idea: Body-slot `TableRow.Href` (whole-row click for rich rows) — verify-before-filing first                                                                                           |
| 30 | P2 | Sparkline for fact-rate (compact form) or Heatmap for hourly activity — design choice, nowband already carries counts                                                                           |
| 31 | P2 | Sticky-header anchor jump: check scroll-margin for `#sec-*` targets under the 56px header                                                                                                       |
| 32 | P2 | Dark-mode QA sweep of custom CSS additions (nowband is deliberately always-dark; verify the rest)                                                                                               |
| 33 | P2 | Physical CSS props in custom CSS: `border-inline-start` already logical; sweep the remainder                                                                                                    |
| 34 | P2 | Board horizontal-scroll on mobile: keyboard/AT reachability of `+N older` escape hatches                                                                                                        |
| 35 | P2 | DLQ table: rescue currently only in task table — verify DLQ-row action parity is intentional                                                                                                    |
| 36 | P2 | `?token=` on SSE URLs (non-loopback binds): verify it never lands in request logs                                                                                                               |
| 37 | P2 | `/project/{name}` route behavior: verify it redirects to filtered table vs dedicated page; document                                                                                             |
| 38 | P2 | Settled table at scale: `Table.LazyRows` (content-visibility) if pages exceed ~100 rows                                                                                                         |
| 39 | P2 | Print stylesheet for task detail (ops runbook printout)                                                                                                                                         |
| 40 | P2 | Empty-state affordances: EmptyState `Action` slot unused — "clear filters" button on the filtered variant                                                                                       |
| 41 | P2 | Budget chip in nowband: title-only explanation; consider tooltip or link to budget semantics                                                                                                    |
| 42 | P2 | Keyboard: `/` `1-4` `?` exist; consider `t`/`b` for table/board toggle (view state is URL-carried already)                                                                                      |
| 43 | P2 | FactFeed: timestamp column alignment (min-w on tag) — cosmetic pass with screenshots from #15                                                                                                   |
| 44 | P2 | i18n note: RelativeTime brings Intl.RelativeTimeFormat — decide en-only vs locale-follow before #6                                                                                              |
| 45 | P2 | Fix the installed skill catalogue drift (118 vs 121) if the installed copy is stale vs repo skill/SKILL.md                                                                                      |
| 46 | P2 | Consider `tc doctor` run against tq (v1.17 ships it): @source scanning + templ pin check for the webui CSS build                                                                                |
| 47 | P2 | Annotate ADR-0003 with the swap-guard amendment once #1-#4 land (docs-health ANNOTATE, not rewrite)                                                                                             |
| 48 | P2 | Arch-judgment: is `#frag-filters` in the swap set ever NEEDED live (chips reflect filter changes from links)? — answer belongs with #2                                                          |
| 49 | P2 | Research-report indexing: decide whether docs/research/*.html belong in the status index or stay gitignore-adjacent (current: neither; daemon will commit)                                      |
| 50 | P2 | Meta: repeat this audit after the P0 batch lands — re-score the 16-area ledger, expect 64 → 80+                                                                                                 |

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **Which pain is the real "sucks ass"?** The audit found interaction-state clobbering
   as the mechanical defect — but if your actual pain is visual style, information
   density, or a specific screen/moment I rated fine, the P0 list aims at the wrong
   target. Name the moment that made you say it.
2. **htmx ruling:** may tq re-enable the embedded htmx runtime (already inside
   layout.Base via go:embed, zero external requests) for the filter bar and future
   interactions, or must the UI stay htmx-free (SSE + vanilla JS only, ADR-0003
   spirit)? This gates item #9 and half of P2.
3. **Ownership:** should the P0/P1 list be harvested into TODO_LIST.md now as pool
   food (docs-health HARVEST), or do you want to assign the batch to a specific
   session/agent (it touches app.js + fragments + components — one coherent change
   set, one reviewer)?

---

## Addendum — visual pass (same day, 13:15 CEST)

After the owner said "it's just ugly", the dashboard was finally LOOKED at:
scratch journal (`TQ_DB=/tmp/tqshot/tasks.db`), `build-tq.sh` binary, worker +
`serve` on loopback, headless Chromium (nix store build) via a throwaway
chromedp harness in /tmp. Ten captures: table light+dark, board dark, detail
dark, mobile 375, tablet 768, populated active state, settled fold open,
chart-aging comparison. Screenshots + harness: `/tmp/tqshot/`.

**Corrections to this report (annotated, not rewritten):**

- §f item 5 (sort UI) is **RETRACTED**: `taskHeaders` (components.go:512)
  already ships `Sortable`+`Href`+`SortDirection` for prio/attempts/age —
  commit 6feaa7d, 2026-09-12 16:14, i.e. before this audit. The morning claim
  inferred "no UI" from the filter bar's hidden input without reading the
  header builder. The deep-dive HTML carries the same correction inline.
- §d3 pattern recurred and was caught same-session: the sort claim sat in two
  reports until the visual pass exposed it. Third verify-before-deliverable
  data point today (after the 500-claim miss).

**New visual findings (full detail: research report §04):**

1. **Worst bug, both themes**: chart y-axis labels overlap into an illegible
   smear — `lineChartMaxTicks=8` is a private library const with no prop; tq's
   `Height: 120` cannot fit it. tq stopgap: raise Height. Real fix upstream
   (tick thinning / MaxTicks prop).
2. **Board clips 2 of 5 columns** (5×w-64 ≈ 1344px inside ~880px column), no
   scroll affordance — DEAD/CANCELLED invisible.
3. Mobile 375: fixed `Width: 560` charts clip x-labels; filter row crushes the
   search box; nowband wraps to two rows.
4. Populated active table clips the last column at the card edge (scrollable
   via the library's overflow wrapper, no hint); DLQ reads as a wall of red;
   detail headline is the full 26-char ULID; aside/main column dead-ends in
   void; settled fold duplicates DLQ rows.
5. Tablet 768 stack is the best layout. What works: dark identity end-to-end,
   detail hierarchy, live ages (ticker verified across captures), retry
   readiness countdown, two-tier table when populated.
6. Hygiene: the `curl` ban rejected an entire seeding chain pre-execution —
   the first re-seed silently never ran and captures showed the stale dataset
   until ground-truthed via `tq tasks`; re-seeded without curl. Master CI
   green at 13:00 (a transient failure on a docs-only commit had self-healed;
   the scratch queue's ci-local task error was that stale state).

**Verdict for "it's just ugly":** the identity is NOT the problem — the
captures show a coherent instrument aesthetic. The ugliness is concentrated:
broken chart axes, clipped board/table edges, red flood, dead whitespace, and
the swap-clobber interaction defects from the morning audit.

---

_Snapshot written 2026-09-14 11:41 CEST; addendum 13:15 CEST. All file:line
claims read from source this session; two claims were written before
verification (500s bare — flagged §a8/§d3, later verified; sort-UI-missing —
retracted above). Index row added same-session per check-status-index.sh
convention._
