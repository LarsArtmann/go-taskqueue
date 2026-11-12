# templ-components — Library Utilization Review

> **2026-11-11 refresh:** re-verified every pin, tag, and file:line cite against the tree at HEAD
> `60c19070` (grep/sed/git-show; guard test run at a throwaway HEAD worktree because the working
> tree does not build — in-flight `internal/queue/sqlite` v4→v5 migration, unrelated to webui).
> All three 2026-11-08 top opportunities landed on **2026-11-09** (commit `2f9c938c`):
> `navigation.Pagination` documented in ADOPTION.md + guard regex widened, `layout.MetaRefresh`
> adopted, `StickyHeader: true` on both ledger tables → re-scored **78 → 94 (very high)**.
> Finding 4 (CopyButton per-render script) stays open — the fix is still upstream-unreleased.

**Verdict:** ADOPTED — very high (8.5/9 capability rows leveraged; 18-row inventory)
**Pinned:** root `go.mod:127-129` `templ-components v1.27.0` + `/icons v1.27.0` + `/utils v1.27.0` (direct), `go.mod:65-66` `/datastar v1.27.0` + `/htmx v1.27.0` (indirect); `cmd/tq/go.mod:86-90` all five submodules at v1.27.0. No split — every module agrees (verified 2026-11-11 by go.mod reads + `GOFLAGS=-mod=mod go list -m all`).
**Latest:** v1.27.0 (2026-11-04) (local checkout /home/lars/projects/templ-components, checked 2026-11-11; HEAD is `charts/echarts/v1.27.0-88-g2ad4cb74` 2026-11-10, 88 in-flight commits unreleased)
**Prior coverage:** the 2026-09-14 templ-components deep-dive (docs/research/2026-09-14_templ-components-deep-dive.html, audited v1.16.0 at score 64/100) — superseded by this report (narrower + newer wins)
**Reviewed:** 2026-11-08 by `grep -rhoE '(display|feedback|layout|navigation|icons|utils)\.[A-Z][a-zA-Z]*' --include='*.go' --include='*.templ' internal/webui | sort | uniq -c`, `grep -c navigation internal/webui/ADOPTION.md`, reads of adoption_test.go + ADOPTION.md + vendored v1.27.0 sources, full reads of /home/lars/projects/templ-components README.md + CHANGELOG.md (Unreleased + v1.17.0-v1.27.0) + charts/echarts/doc.go, `go test ./internal/webui -run 'TestAdoptionTable' -count=1` (ok 0.007s). **Re-verified 2026-11-11** by the same greps against the current tree, `git -C /home/lars/projects/templ-components` tag/HEAD checks, upstream `utils.TestDocsCountDrift` rerun (PASS at checkout HEAD `2ad4cb74`), and `go test ./internal/webui -run 'TestAdoptionTable' -count=1` at HEAD `60c19070` (ok 0.007s)

## 1. Usage inventory

Files: `internal/webui/fragments.templ` (1099 ln), `internal/webui/layout.templ` (211 ln), `internal/webui/components.go` (983 ln), `internal/webui/render.go` (1296 ln) plus their generated `*_templ.go` (verified 2026-11-08 by `grep -rl --include='*.go' 'larsartmann/templ-components' . | grep -v '^./vendor/'`; line counts re-verified 2026-11-11). Direct imports: `display`, `feedback`, `icons`, `layout`, `navigation`, `utils` — `utils` at 10 `BaseProps` sites (the 74-sites figure in circulation did not reproduce; verified by grep: 5 in .templ sources + 5 in generated files, re-verified 2026-11-11). `htmx`, `datastar`, `utils/{svg,tone,wire,cdn}` ride the graph transitively via display and go-health-dashboard — zero tq call sites (verified 2026-11-11 by grep: no `templ-components/htmx` or `/datastar` imports outside generated files).

Adopted surface (fragments.templ/components.go, all line-checked 2026-11-11): `layout.Base` ×3 + `ThemeToggle` ×1 (layout.templ:9,111,120,152) configured through `dashboardProps` — noindex SEO, nonce, CSSPath/Favicon, theme colors, custom BodyClass, `HTMXVersion=""` deliberately stripping htmx (components.go:638-664, HTMXVersion at :644), `layout.MetaRefresh(3600, "")` as HeadContent (components.go:660 — adopted 2026-11-09, commit 2f9c938c, replacing the deleted hand-rolled `refreshMeta`); display `Badge` ×12, `TableHeader` ×11, `Card` ×8, `EmptyState` ×5, `Scrollback`/`ScrollbackLine`/`ScrollbackTone`, `CopyButton` ×3 (fragments.templ:750,871,973), `DefinitionList`, `ListNote` (fragments.templ:203), `AreaChart` ×2 at Height 200 (fragments.templ:1060-1082), `Table` ×3 — the two ledger tables with `TypedHeaders` sort links AND `StickyHeader: true` (fragments.templ:148-156,181-190; sticky adopted 2026-11-09), the DLQ table plain (fragments.templ:402-412), `taskHeaders` at components.go:878-920 (sort links :892-919), `Button`, `Grid`; `feedback.Alert`; `icons.ArchiveBox/CircleStack/Filter/Inbox` (4 of the 111 names shipped in the pinned tree); `navigation.Pagination` (fragments.templ:207-210) — documented in ADOPTION.md:21 since 2026-11-09; Go-invoked `display.RelativeTime` with nonce + AutoRefresh (components.go:455-467, call at :462). Custom rows (ADOPTION.md, guard-pinned): tq-seg nowband, board columns/cards, filter inputs, page header/lamp. REJECTED: `display.Eyebrow`, `KanbanBoard`, errorpage, `icons.Render` (all still shipped upstream at checkout HEAD `2ad4cb74` — the rejections stay valid; ADOPTION.md REJECTED list intact).

Catalog: display 53 + feedback 15 + forms 26 + navigation 12 + layout 11 = **117 components** (README/FEATURES table, guarded by upstream `utils.TestDocsCountDrift` — rerun 2026-11-11 at checkout HEAD `2ad4cb74`: PASS; the FEATURES headline "138 templ components" adds htmx 9 + datastar 6 + icons 8, which tq doesn't count), 111 icon names in the pinned v1.27.0 vendored tree (icon_names.go, counted 2026-11-11; the unreleased wave trims toward FEATURES' "102"), plus `charts/echarts` (v1.27.0 submodule — Tier-2 ECharts adapter: consumer feeds it go-echarts `RenderSnippet()` output, ~1 MB JS runtime injected via `SDKScript`, doc.go explicitly routes CSP/zero-JS consumers to the Tier-1 native SVG charts tq already uses). tq leans on ~23 of ~117 components + 4 icons (MetaRefresh joins the set since 2026-11-09).

## 2. Capability inventory

| Capability | Status | Evidence |
| --- | --- | --- |
| Page shell (`layout.Base`, `PageProps`: SEO/nonce/CSS/theme/Footer/HeadContent) | 🟢 FULL | dashboardProps exercises every operator-relevant field incl. `SEO.NoIndex` (components.go:638-664) |
| Theme system (`ThemeToggle` + `@theme` bridge in theme.css) | 🟢 FULL | remap-don't-replace bridge per the library recipe (prior deep-dive "already excellent" #1, unchanged); `ThemeToggle` layout.templ:152 |
| Data-display set (Badge/Card/Table+TypedHeaders+StickyHeader/DefinitionList/EmptyState/Scrollback/ListNote/AreaChart/Grid/Button) | 🟢 FULL | 11 display components in daily use; sort links live since 6feaa7d (components.go:892-919), StickyHeader since 2026-11-09 (fragments.templ:153,187) |
| Navigation (`navigation.Pagination`) | 🟢 FULL | adopted (fragments.templ:207-210), documented in ADOPTION.md:21 and covered by the widened guard regex (adoption_test.go:21) since 2026-11-09 (commit 2f9c938c) |
| Feedback (`feedback.Alert`) | 🟢 FULL | task-detail last-error (ADOPTION.md row; fragments.templ) |
| Icons | ⚪ N/A | 4 icons adopted deliberately for empty states/filter; broader accents ruled decorative-skip in the 2026-09-14 execution log §H — taste scope, not a capability gap |
| `RelativeTime` (Go-invoked, AutoRefresh) | 🟢 FULL | 30 s re-query of `time[data-tc-relative]` survives tq's raw-SSE innerHTML swaps (vendored display/shared.go relativeTimeJS); fact feed keeps wall-clock + data-age ticker by documented decision (ADOPTION.md) |
| `CopyButton` | 🟡 PARTIAL | 3 sites work, but v1.27.0 emits the singleton script inside every render (vendored copy_button.templ:115 — re-verified 2026-11-11), so each SSE-swapped detail fragment re-ships an inert `<script>` (innerHTML-inserted scripts never execute) — dead bytes per tick, not a functional bug (see Finding 4) |
| Forms (26 components incl. `FilterBar`/`FilterChips`/`SegmentedControl`, v1.22-1.23) | ⚪ N/A | filter row is a pinned custom row (ADOPTION.md); tq ships neither runtime, so upstream auto-submit forms can't apply without reversing the 2026-09-14 htmx-OUT ruling (plan §G) |
| `wire` typed actions (ADR-0043 dual-transport htmx/Datastar) | ⚪ N/A | same no-runtime ruling; `HTMXVersion=""` (components.go:644) |
| `charts/echarts` Tier-2 | ⚪ N/A | dashboard charts are Tier-1 SVG by CSP doctrine — echarts doc.go itself routes zero-JS consumers to `display.AreaChart`; a ~1 MB runtime buys tooltips tq doesn't need on a loopback read-only dashboard |
| `layout.MetaRefresh` | 🟢 FULL | adopted 2026-11-09: `props.HeadContent = layout.MetaRefresh(3600, "")` (components.go:660, commit 2f9c938c); the hand-rolled `refreshMeta` templ was deleted from layout.templ in the same commit |
| `Table.StickyHeader` | 🟢 FULL | adopted 2026-11-09 on both TypedHeaders tables (fragments.templ:153,187, commit 2f9c938c) — long ledgers keep their header pinned; component ships in the pinned version (vendored display/table.templ:188-193, emitted at :269) |
| Other surfaces (`StatusDot`/`LivePill`/`DataState`/`SegmentBar`/`Lightbox`/`CodeBlock`/CommandPalette/skeletons) | ⚪ N/A | no matching surface; nowband/board/lamp/error page are pinned custom-or-rejected rows |
| utils plumbing (`BaseProps` Nonce/Class/ID/Attrs; `Coalesce`) | 🟢 FULL | `BaseProps` carries the CSP nonce through every component call site (10 sites); `utils.Coalesce` (v1.27.0) has no tq use case |
| Feedback beyond Alert (Toast/LoadingOverlay/LoadingProgress/Skeleton) | ⚪ N/A | a snapshot-on-tick dashboard has no toasts or long operations; errors render inline via the adopted `feedback.Alert` |
| Layout beyond Base (`Minimal`, AppShell/SidebarNav) | ⚪ N/A | single two-page operator surface (dashboard + task detail); `Base` with custom BodyClass is the shell |
| Navigation beyond Pagination (Tabs/LoadMore/CommandPalette) | ⚪ N/A | one ledger view + detail; view state rides the URL (`view=board`), not tabs |

## 3. Findings

1. **~~`navigation.Pagination` is adopted but undocumented~~ RESOLVED 2026-11-09 (commit `2f9c938c`).** ADOPTION.md now carries the adopted row (internal/webui/ADOPTION.md:21, "`navigation.Pagination` | adopted | `fragments.templ` (task pager under the ledger table)") and the guard regex was widened to `(display|layout|feedback|navigation)` (internal/webui/adoption_test.go:21), so `TestAdoptionTableCoversTemplates` now flags rot in both directions. Verified 2026-11-11 by grep (`grep -c navigation` = 1) and a green guard run at HEAD (`ok 0.007s`).
2. **~~Hand-rolled `refreshMeta` next to shipped `layout.MetaRefresh`~~ RESOLVED 2026-11-09 (commit `2f9c938c`).** `dashboardProps` now sets `props.HeadContent = layout.MetaRefresh(3600, "")` (internal/webui/components.go:660) — clamping and self-reload semantics owned by the component (vendored layout/meta_refresh.templ:5-23); the hand-rolled templ was deleted (layout.templ 216 → 211 lines, diff verified via `git show 2f9c938c`).
3. **~~`Table.StickyHeader` unused on the ledger table~~ RESOLVED 2026-11-09 (commit `2f9c938c`).** `StickyHeader: true` sits on both ledger tables (internal/webui/fragments.templ:153,187); the header row pins via the component's `sticky top-0 z-10` emission (vendored display/table.templ:269). The DLQ table (fragments.templ:402-412) deliberately stays plain — 5 fixed columns, no sort links to lose.
4. **CopyButton's per-swap script bytes (unreleased fix on deck) — still open 2026-11-11.** v1.27.0 appends `@copyButtonScriptComponent(props.Nonce)` to every CopyButton render (vendored copy_button.templ:115 — unchanged, re-verified), and detail fragments are re-shipped by SSE via innerHTML, where inserted scripts never run — so each tick re-sends a dead script tag. tq is NOT functionally exposed: the delegation script registers at document level (`document.addEventListener('click', …closest('[data-tc-copy]'))`, vendored display/shared.go:157-189, delegation listener at :178) and the detail page's initial server HTML contains the first CopyButton, so the singleton executes at page load and keeps serving swapped-in buttons. The upstream fix — `CopyButtonProps.NoScript` + a single `display.CopyButtonScript(nonce)` in the page shell (checkout display/copy_button.templ:33-58, CHANGELOG Unreleased #63 — both re-verified 2026-11-11 at checkout HEAD `2ad4cb74`) — is the adopt-at-next-bump item; until then the cost is inert bytes per tick, not broken buttons.
5. **Supersession ledger vs the 2026-09-14 deep-dive** (all verified on today's tree):

   | Deep-dive roadmap item | Status 2026-11-11 | Evidence |
   | --- | --- | --- |
   | 1. Swap-guard + state re-apply | shipped 2026-09-14 (plan §B) | server half: `filtersHTML` byte-compare, handlers.go:432-439; client half in app.js |
   | 2. TypedHeaders sort UI | shipped 6feaa7d (audit retraction stands) | components.go:878-920 (sort links :892-919) |
   | 3. RelativeTime adoption | shipped | components.go:455-467; ADOPTION.md Go-invoked note |
   | 4. navigation.Pagination | shipped AND documented (2026-11-09) | fragments.templ:207-210; ADOPTION.md:21; guard widened adoption_test.go:21 |
   | 5. Filter without full reload | shipped as custom fetch+pushState (§G ruling: htmx OUT) | forms.FilterBar stays N/A |
   | 6. CopyButton/ListNote/errorpage/icons | CopyButton+ListNote shipped; errorpage stays REJECTED; icons decorative-skip ruling stands | fragments.templ:750,871,973,203; ADOPTION.md |
   | 7. v1.17-era a11y pack | moot — v1.27.0 includes it all | pinned version |

   The two 2026-11-08 misses the deep-dive could not see — MetaRefresh and StickyHeader — were both adopted on 2026-11-09 (Finding 2/3 above).

## 4. Score + top opportunities

N=18 rows; N/A=9; A=9; FULL=8, PARTIAL=1, MISSED=0 → Σ=8.5 → score = round(100×8.5/9) = **94 — very high** (2026-11-08 edition: 78 — high; the +16 came entirely from commit `2f9c938c` resolving the three then-open rows: Navigation PARTIAL→FULL, MetaRefresh MISSED→FULL, StickyHeader MISSED→FULL; prior deep-dive at v1.16.0: 64/100). Top: (1) adopt `CopyButtonProps.NoScript` + a single `display.CopyButtonScript(nonce)` in the page shell at the next pin bump (unreleased upstream, #63 — Finding 4); (2) evaluate the post-v1.27.0 wave at the same bump (Datastar CSP nonce attrs, Sparkline Responsive/ZeroAware, ScrollbackLine ID/Class, ListNote custom Message, feedback.LoadingProgress, wire.Action Headers/Payload); (3) keep the widened guard green — it now covers all four adopted packages.

## 5. Version currency + risk

On latest: v1.27.0 = newest tag, every tq pin at v1.27.0, no cmd/tq split (verified 2026-11-11 by `go list -m all | grep templ-components` + go.mod reads). The local checkout runs 88 commits past the tag (HEAD `2ad4cb74`, 2026-11-10) with an in-flight wave directly relevant to tq: Datastar CSP nonce mode (`layout.DatastarNonceAttrs`), `CopyButtonScript`/`NoScript`, `Sparkline` Responsive/ZeroAware, `ScrollbackLine` ID/Class, `ListNote` custom Message, `feedback.LoadingProgress`, `wire.Action` Headers/Payload — on-deck at the next pin bump, none urgent. Guard pins vs latest: all four REJECTED surfaces still exist upstream (templ sources re-grepped 2026-11-11); every adopted row still exists in the vendored v1.27.0 tree; `TestAdoptionTableCoversTemplates`/`TestAdoptionTablePinsCustomRows` pass (ok 0.007s, run at HEAD `60c19070`). Risk: LOW — same-author, generated `*_templ.go` committed, templ+CSS regen rails documented (AGENTS.md), and the templ-LSP false-positive rule is already a Known Issue.

## 6. Conclusion

The dashboard's templ-components adoption has matured from the deep-dive's 64/100 past the 2026-11-08 edition's 78 to 94/100: the shell, theme bridge, display set with live sort and sticky headers, pagination (documented and guard-covered), relative time, meta refresh, and copy buttons are all library-first, and every divergence from the library is a guard-pinned ruling rather than an accident. What remains is a single open item — the CopyButton per-render script bytes, fixable only once upstream releases `NoScript`/`CopyButtonScript` (#63) — plus a watch-list for the post-v1.27.0 wave at the next pin bump.

## How to verify

```bash
grep -rhoE '(display|feedback|layout|navigation|icons|utils)\.[A-Z][a-zA-Z]*' --include='*.templ' internal/webui | sort | uniq -c | sort -rn | head -12
grep -c 'navigation' internal/webui/ADOPTION.md                        # 1 — Finding 1 resolved 2026-11-09
sed -n '21p' internal/webui/adoption_test.go                           # regex covers display|layout|feedback|navigation
grep -n 'MetaRefresh' internal/webui/components.go                     # 660: props.HeadContent = layout.MetaRefresh(3600, "")
grep -n 'StickyHeader' internal/webui/fragments.templ                  # 153, 187
grep -n 'MetaRefresh' vendor/github.com/larsartmann/templ-components/layout/meta_refresh.templ | head -2
grep -n 'StickyHeader' vendor/github.com/larsartmann/templ-components/display/table.templ | head -3
grep -n 'copyButtonScriptComponent' vendor/github.com/larsartmann/templ-components/display/copy_button.templ   # 115 — still per-render at v1.27.0
grep -n 'NoScript' /home/lars/projects/templ-components/display/copy_button.templ | head -3                     # unreleased fix
cd /home/lars/projects/templ-components && git describe --tags         # charts/echarts/v1.27.0-88-g2ad4cb74
GOFLAGS=-mod=mod go list -m all | grep 'templ-components'              # all v1.27.0
go test ./internal/webui -run 'TestAdoptionTable' -count=1             # guard green — run at HEAD (worktree): the working tree does not build while the in-flight sqlite v4→v5 migration is open
```
All commands above were run literally — the greps on 2026-11-11 against the working tree, the guard test on 2026-11-11 at HEAD `60c19070` via a throwaway `git worktree` (working tree unbuildable, see refresh note; run: ok 0.007s). The upstream catalog guard `TestDocsCountDrift` (utils submodule of the checkout) was also rerun 2026-11-11: PASS.
