# templ-components — Library Utilization Review

**Verdict:** ADOPTED — high (7/9 capability rows leveraged; 18-row inventory)
**Pinned:** root `go.mod:126-128` `templ-components v1.27.0` + `/icons v1.27.0` + `/utils v1.27.0` (direct), `go.mod:66-67` `/datastar v1.27.0` + `/htmx v1.27.0` (indirect); `cmd/tq/go.mod:86-90` all five submodules at v1.27.0. No split — every module agrees.
**Latest:** v1.27.0 (2026-11-04) (local checkout /home/lars/projects/templ-components, checked 2026-11-08; HEAD is `charts/echarts/v1.27.0-87-g5c0d170a`, 87 in-flight commits)
**Prior coverage:** the 2026-09-14 templ-components deep-dive (docs/research/2026-09-14_templ-components-deep-dive.html, audited v1.16.0 at score 64/100) — superseded by this report (narrower + newer wins)
**Reviewed:** 2026-11-08 by `grep -rhoE '(display|feedback|layout|navigation|icons|utils)\.[A-Z][a-zA-Z]*' --include='*.go' --include='*.templ' internal/webui | sort | uniq -c`, `grep -c navigation internal/webui/ADOPTION.md`, reads of adoption_test.go + ADOPTION.md + vendored v1.27.0 sources, full reads of /home/lars/projects/templ-components README.md + CHANGELOG.md (Unreleased + v1.17.0-v1.27.0) + charts/echarts/doc.go, `go test ./internal/webui -run 'TestAdoptionTable' -count=1` (ok 0.007s)

## 1. Usage inventory

Files: `internal/webui/fragments.templ` (1097 ln), `internal/webui/layout.templ` (216 ln), `internal/webui/components.go` (983 ln), `internal/webui/render.go` (1296 ln) plus their generated `*_templ.go` (verified 2026-11-08 by `grep -rl --include='*.go' 'larsartmann/templ-components' . | grep -v '^./vendor/'`). Direct imports: `display`, `feedback`, `icons`, `layout`, `navigation`, `utils` — `utils` at 10 `BaseProps` sites (the 74-sites figure in circulation did not reproduce; verified by grep: 5 in .templ sources + 5 in generated files). `htmx`, `datastar`, `utils/{svg,tone,wire,cdn}` ride the graph transitively via display and go-health-dashboard — zero tq call sites (verified by grep).

Adopted surface (fragments.templ/components.go, all line-checked): `layout.Base` ×3 + `ThemeToggle` ×1 (layout.templ:9,111,120,152) configured through `dashboardProps` — noindex SEO, nonce, CSSPath/Favicon, theme colors, custom BodyClass, `HTMXVersion=""` deliberately stripping htmx (components.go:633-665, HTMXVersion at :644); display `Badge` ×12, `TableHeader` ×11, `Card` ×8, `EmptyState` ×5, `Scrollback`/`ScrollbackLine`/`ScrollbackTone`, `CopyButton` ×3 (fragments.templ:748,869,971), `DefinitionList`, `ListNote` (fragments.templ:201), `AreaChart` ×2 at Height 200 (fragments.templ:1058-1080), `Table` ×3 with `TypedHeaders` sort links (fragments.templ:150,183 + taskHeaders components.go:874-916), `Button`, `Grid`; `feedback.Alert`; `icons.ArchiveBox/CircleStack/Filter/Inbox` (4 of ~111 names); `navigation.Pagination` (fragments.templ:205); Go-invoked `display.RelativeTime` with nonce + AutoRefresh (components.go:459-466). Custom rows (ADOPTION.md, guard-pinned): tq-seg nowband, board columns/cards, filter inputs, page header/lamp. REJECTED: `display.Eyebrow`, `KanbanBoard`, errorpage, `icons.Render` (all still shipped upstream at checkout HEAD — the rejections stay valid).

Catalog: display 53 + feedback 15 + forms 26 + navigation 12 + layout 11 = **117 components** (README table, computed by upstream `utils.TestDocsCountDrift`, last verified 2026-11-04), ~111 icon names, plus `charts/echarts` (NEW v1.27.0 submodule — Tier-2 ECharts adapter: consumer feeds it go-echarts `RenderSnippet()` output, ~1 MB JS runtime injected via `SDKScript`, doc.go explicitly routes CSP/zero-JS consumers to the Tier-1 native SVG charts tq already uses). tq leans on ~22 of ~117 components + 4 icons.

## 2. Capability inventory

| Capability | Status | Evidence |
| --- | --- | --- |
| Page shell (`layout.Base`, `PageProps`: SEO/nonce/CSS/theme/Footer/HeadContent) | 🟢 FULL | dashboardProps exercises every operator-relevant field incl. `SEO.NoIndex` (components.go:633-665) |
| Theme system (`ThemeToggle` + `@theme` bridge in theme.css) | 🟢 FULL | remap-don't-replace bridge per the library recipe (prior deep-dive "already excellent" #1, unchanged); `ThemeToggle` layout.templ:152 |
| Data-display set (Badge/Card/Table+TypedHeaders/DefinitionList/EmptyState/Scrollback/ListNote/AreaChart/Grid/Button) | 🟢 FULL | 11 display components in daily use; sort links live since 6feaa7d (components.go:892-915) |
| Navigation (`navigation.Pagination`) | 🟡 PARTIAL | adopted (fragments.templ:205-208) but absent from ADOPTION.md (`grep -c navigation` = 0) and invisible to the guard regex (adoption_test.go:21-22 covers display/layout/feedback only) |
| Feedback (`feedback.Alert`) | 🟢 FULL | task-detail last-error (ADOPTION.md row; fragments.templ) |
| Icons | ⚪ N/A | 4 icons adopted deliberately for empty states/filter; broader accents ruled decorative-skip in the 2026-09-14 execution log §H — taste scope, not a capability gap |
| `RelativeTime` (Go-invoked, AutoRefresh) | 🟢 FULL | 30 s re-query of `time[data-tc-relative]` survives tq's raw-SSE innerHTML swaps (vendored display/shared.go relativeTimeJS); fact feed keeps wall-clock + data-age ticker by documented decision (ADOPTION.md) |
| `CopyButton` | 🟡 PARTIAL | 3 sites work, but v1.27.0 emits the singleton script inside every render (vendored copy_button.templ:115), so each SSE-swapped detail fragment re-ships an inert `<script>` (innerHTML-inserted scripts never execute) — dead bytes per tick, not a functional bug (see Finding 4) |
| Forms (26 components incl. `FilterBar`/`FilterChips`/`SegmentedControl`, v1.22-1.23) | ⚪ N/A | filter row is a pinned custom row (ADOPTION.md); tq ships neither runtime, so upstream auto-submit forms can't apply without reversing the 2026-09-14 htmx-OUT ruling (plan §G) |
| `wire` typed actions (ADR-0043 dual-transport htmx/Datastar) | ⚪ N/A | same no-runtime ruling; `HTMXVersion=""` (components.go:644) |
| `charts/echarts` Tier-2 | ⚪ N/A | dashboard charts are Tier-1 SVG by CSP doctrine — echarts doc.go itself routes zero-JS consumers to `display.AreaChart`; a ~1 MB runtime buys tooltips tq doesn't need on a loopback read-only dashboard |
| `layout.MetaRefresh` | 🔴 MISSED | tq hand-rolls the identical tag (layout.templ:213-216) while the component ships in the pinned version (vendored layout/meta_refresh.templ:5-23, shipped v1.22.0) |
| `Table.StickyHeader` | 🔴 MISSED | the v1.22.0 consumer flag is unused on the already-adopted Table (vendored display/table.templ:188-193,269) — long ledgers scroll their header out of view |
| Other surfaces (`StatusDot`/`LivePill`/`DataState`/`SegmentBar`/`Lightbox`/`CodeBlock`/CommandPalette/skeletons) | ⚪ N/A | no matching surface; nowband/board/lamp/error page are pinned custom-or-rejected rows |
| utils plumbing (`BaseProps` Nonce/Class/ID/Attrs; `Coalesce`) | 🟢 FULL | `BaseProps` carries the CSP nonce through every component call site (10 sites); `utils.Coalesce` (v1.27.0) has no tq use case |
| Feedback beyond Alert (Toast/LoadingOverlay/LoadingProgress/Skeleton) | ⚪ N/A | a snapshot-on-tick dashboard has no toasts or long operations; errors render inline via the adopted `feedback.Alert` |
| Layout beyond Base (`Minimal`, AppShell/SidebarNav) | ⚪ N/A | single two-page operator surface (dashboard + task detail); `Base` with custom BodyClass is the shell |
| Navigation beyond Pagination (Tabs/LoadMore/CommandPalette) | ⚪ N/A | one ledger view + detail; view state rides the URL (`view=board`), not tabs — MetaRefresh is scored separately above |

## 3. Findings

1. **`navigation.Pagination` is adopted but undocumented — and the guard can't see it.** ADOPTION.md contains zero navigation rows (verified 2026-11-08 by `grep -c navigation internal/webui/ADOPTION.md` = 0) while fragments.templ:205 invokes it; the guard's `componentCallRe = (display|layout|feedback)\.` (internal/webui/adoption_test.go:21-22) structurally misses the package, so `TestAdoptionTableCoversTemplates` cannot flag either rot direction here. We do instead: table + regex cover three packages. Impact: the doc understates adoption; a future Pagination removal would leave a stale doc and a silent drop. Fix: add the row and widen the regex to `(display|layout|feedback|navigation)`.
2. **Hand-rolled `refreshMeta` next to shipped `layout.MetaRefresh`.** Offer: `layout.MetaRefresh(delaySeconds, url)` with clamping and self-reload semantics (vendored layout/meta_refresh.templ:5-23). We do instead: a literal `<meta http-equiv="refresh" content="3600"/>` (internal/webui/layout.templ:213-216). Impact: trivial today, but the hand-roll can silently diverge from upstream fixes (the SSE-stall fallback is exactly the pattern the component documents). Example: `@layout.MetaRefresh(3600, "")`.
3. **`Table.StickyHeader` unused on the ledger table.** Offer: one boolean pinning the header row (vendored display/table.templ:188-193, emitted at :269). We do instead: default scrolling header inside the table card (fragments.templ:150-183). Impact: on long filtered ledgers the sort links and column identity scroll away. Example: `StickyHeader: true` on the existing `display.TableProps`.
4. **CopyButton's per-swap script bytes (unreleased fix on deck).** v1.27.0 appends `@copyButtonScriptComponent(props.Nonce)` to every CopyButton render (vendored copy_button.templ:115), and detail fragments are re-shipped by SSE via innerHTML, where inserted scripts never run — so each tick re-sends a dead script tag. tq is NOT functionally exposed: the delegation script registers at document level (`document.addEventListener('click', …closest('[data-tc-copy]'))`, vendored display/shared.go:167-185) and the detail page's initial server HTML contains the first CopyButton, so the singleton executes at page load and keeps serving swapped-in buttons. The upstream fix — `CopyButtonProps.NoScript` + a single `display.CopyButtonScript(nonce)` in the page shell (checkout display/copy_button.templ:33-58, Unreleased #63) — is the adopt-at-next-bump item; until then the cost is inert bytes per tick, not broken buttons.
5. **Supersession ledger vs the 2026-09-14 deep-dive** (all verified on today's tree):

   | Deep-dive roadmap item | Status 2026-11-08 | Evidence |
   | --- | --- | --- |
   | 1. Swap-guard + state re-apply | shipped 2026-09-14 (plan §B) | server half: `filtersHTML` byte-compare, handlers.go:418-434; client half in app.js |
   | 2. TypedHeaders sort UI | shipped 6feaa7d (audit retraction stands) | components.go:874-916 |
   | 3. RelativeTime adoption | shipped | components.go:459-466; ADOPTION.md Go-invoked note |
   | 4. navigation.Pagination | shipped — but undocumented | fragments.templ:205-208; ADOPTION.md gap (Finding 1) |
   | 5. Filter without full reload | shipped as custom fetch+pushState (§G ruling: htmx OUT) | forms.FilterBar stays N/A |
   | 6. CopyButton/ListNote/errorpage/icons | CopyButton+ListNote shipped; errorpage stays REJECTED; icons decorative-skip ruling stands | fragments.templ:748,869,971,201; ADOPTION.md |
   | 7. v1.17-era a11y pack | moot — v1.27.0 includes it all | pinned version |

   Two new misses the deep-dive could not see: MetaRefresh and StickyHeader (both shipped upstream after it).

## 4. Score + top opportunities

N=18 rows; N/A=9; A=9; FULL=6, PARTIAL=2, MISSED=2 → Σ=7.0 → score = round(100×7.0/9) = **78 — high** (prior deep-dive at v1.16.0: 64/100; the gap closed mostly via the 2026-09-14 same-day overhaul). Top: (1) document Pagination + widen the guard regex; (2) swap `refreshMeta` → `layout.MetaRefresh`; (3) flip `StickyHeader: true` — together worth ~+11 points (≈89, very high).

## 5. Version currency + risk

On latest: v1.27.0 = newest tag, every tq pin at v1.27.0, no cmd/tq split (verified 2026-11-08 by `go list -m all | grep templ-components`). The local checkout runs 87 commits past the tag with an in-flight wave directly relevant to tq: Datastar CSP nonce mode (`layout.DatastarNonceAttrs`), `CopyButtonScript`/`NoScript`, `Sparkline` Responsive/ZeroAware, `ScrollbackLine` ID/Class, `ListNote` custom Message, `feedback.LoadingProgress`, `wire.Action` Headers/Payload — on-deck at the next pin bump, none urgent. Guard pins vs latest: all four REJECTED surfaces still exist upstream; every adopted row still exists in the vendored v1.27.0 tree; `TestAdoptionTableCoversTemplates`/`TestAdoptionTablePinsCustomRows` pass (ok 0.006s). Risk: LOW — same-author, generated `*_templ.go` committed, templ+CSS regen rails documented (AGENTS.md), and the templ-LSP false-positive rule is already a Known Issue.

## 6. Conclusion

The dashboard's templ-components adoption has matured from the deep-dive's 64/100 to a genuinely high utilization: the shell, theme bridge, display set with live sort, pagination, relative time, and copy buttons are all library-first, and every divergence from the library is a guard-pinned ruling rather than an accident. What remains is small and mechanical — one documentation debt with a guard blind spot (Pagination), two one-line component swaps (MetaRefresh, StickyHeader) — plus a watch-list for the post-v1.27.0 wave, led by the CopyButton fragment-script fix that tq's SSE swap architecture will want the day it bumps pins.

## How to verify

```bash
grep -rhoE '(display|feedback|layout|navigation|icons|utils)\.[A-Z][a-zA-Z]*' --include='*.templ' internal/webui | sort | uniq -c | sort -rn | head -12
grep -c 'navigation' internal/webui/ADOPTION.md                        # 0 — Finding 1
sed -n '21,22p' internal/webui/adoption_test.go                       # regex covers display|layout|feedback only
grep -n 'MetaRefresh' vendor/github.com/larsartmann/templ-components/layout/meta_refresh.templ | head -2
sed -n '213,216p' internal/webui/layout.templ                          # hand-rolled refreshMeta
grep -n 'StickyHeader' vendor/github.com/larsartmann/templ-components/display/table.templ | head -3
grep -n 'copyButtonScriptComponent' vendor/github.com/larsartmann/templ-components/display/copy_button.templ
cd /home/lars/projects/templ-components && git describe --tags         # charts/echarts/v1.27.0-87-g5c0d170a
GOFLAGS=-mod=mod go list -m all | grep 'templ-components'              # all v1.27.0
go test ./internal/webui -run 'TestAdoptionTable' -count=1             # guard green
```
All commands above were run literally on 2026-11-08 and produced the cited outputs (guard run: ok 0.007s).
