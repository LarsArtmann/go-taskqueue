# 2026-09-14 17:35 — UI stunning overhaul: full plan executed (webui)

**Task window:** owner GO on `docs/planning/2026-09-14_13-24_ui-stunning-overhaul.md`;
all 23 workstreams A1-I3 executed 15:50-17:30 on a scratch journal
(`/tmp/tqshot/tasks.db`; production untouched). Full evidence log:
`docs/research/2026-09-14_templ-components-deep-dive.html` §05.

## What shipped (by tier)

- **1% (51%)**: chart height 120→200 (y-axis smear gone); SSE swap-guard +
  state re-apply (typing/folds/error-expands survive live bursts; grace
  flush ≤2.5s with latest-wins) + server `frag-filters` change-detection.
- **4% (64%)**: board full-width + shrink-to-fit lanes (5-across ≥1280) +
  honest overflow fade; DLQ red wall → calm list (left-rule severity,
  neutral error text everywhere); mobile filter row reflow; nowband 3+2
  uniform grid at 375.
- **20% (80%)**: detail headline rework (short ID + copy + quiet ULID);
  `display.CopyButton` adoption (id/payload/raw/report — adoption table
  row); `display.RelativeTime` on detail rows (prose-documented,
  Go-invoked); filter auto-submit via fetch+pushState (debounced search,
  SSE re-scope, popstate, busy cue; zero new deps — htmx ruled out);
  empty-state "clear filters" action.
- **100%**: a11y (forced-colors focus restore, prefers-contrast hairlines,
  aria-expanded + keyboard expand, reduced-motion audit closed the last
  gap on the fold chevron), running-glow pulse, hygiene (factLines
  deleted, 2× QF1003 → switch, tidy clean).

## Gates

- Root + every sub-module: build/vet/test green (webui race suite green).
- `smoke/webui.sh` green (read-only contract intact).
- treefmt green (templ switch indentation fixed via `templ fmt`).
- lint-baseline gate green (554 findings vs 558 baseline — shrink).
- `nix run .#webui-css` byte-rebuild after every theme.css/templ change.
- Live proofs: chromedp soak (LIVECHECK-OK), auto-submit check
  (PUSHSTATE-OK), 375/768/1024/1440 matrix captures.

## Follow-ups minted

Seven leftover P2s harvested into TODO_LIST.md (Pagination, ListNote,
errorpage 500s, upstream MaxTicks filing, a11y-pack version bump, sort
chip polish, CSP form-action owner ruling — BLOCKED).

## Notable finds for the next agent

- `sse.WithBufferSize` type arg is NOT inferable (T in return position
  only) — keep the explicit `sse.WithBufferSize[sse.Event](...)`.
- The old bare-substring assertion in `TestFiltersNarrowTable` tripped on
  the empty-state button's `shadow-xs` class — element assertions now use
  `>sh<`. Before renaming any var, grep quoted strings (the rule bit
  again in reverse: a class name collided with a test substring).
- Static assets are `embed`-ded: `app.css`/`app.js` changes need a tq
  rebuild before browser verification, not just `nix run .#webui-css`.
- Master CI was red 12:17-12:31Z from the depsweep window (facade
  parity skew, G602, treefmt) — all three fixed on HEAD by 17:30
  (parity by the other agent, treefmt by this window's templ fmt,
  G602 nosec by the other agent). ci-local run with CI_CHECK=off on
  that basis; the push will re-prove it on runners.
