# Webui live contract — swap-guard, state re-apply, fragment streams

The live dashboard (`tq serve`, internal/webui) renders fully without JS,
then `internal/webui/static/app.js` swaps fragments over SSE (ADR-0003).
This document pins the invariants the swap-guard and state re-apply rely
on. The unit harness `internal/webui/jstest/app_test.mjs`
(`scripts/check-webui-js.sh`, wired into `scripts/ci-local.sh`) enforces
the behavior side; template drift is caught by the guard tests noted
below.

## Fragment ids

Streamed fragments target exactly these container ids:
`frag-filters`, `frag-stats`, `frag-table`, `frag-dlq`, plus the
task-scoped `frag-detail` / `frag-timeline` on detail pages. The filter
auto-submit path (`FRAG_IDS`) and the SSE `frag` events both address the
same ids; a rename here is a contract break, not a refactor.

## Swap-guard (busy deferral)

A fragment is **busy** — and must not be replaced mid-interaction — when
any of:

- the document's `activeElement` is an `input`/`select`/`textarea` or
  contenteditable inside the fragment;
- the fragment contains an open `<details>` (`details[open]`);
- the fragment contains an element with `data-expanded="1"`
  (expanded error cell).

Deferred fragments queue their **LATEST** payload per id (latest-wins) and
flush after a **2.5 s grace window** (`FLUSH_GRACE_MS`) from the queued
timestamp — data never goes stale by more than ~1 tick beyond the grace.
The flush re-arms itself on the youngest queued entry. Grace-forced swaps
run through the same capture/restore path as idle swaps (below), so the
operator's interaction state survives.

## State re-apply (capture before swap, restore after)

`swapIn` captures, then restores across the `innerHTML` replacement:

| State                   | Matched by                                     | Restore                                                                                 |
| ----------------------- | ---------------------------------------------- | --------------------------------------------------------------------------------------- |
| `<details>` open state  | `data-state-key`, else summary text (fallback) | re-open matching details                                                                |
| Expanded error cells    | `data-short` attribute                         | text → `data-error`, `data-expanded="1"`, `aria-expanded="true"`                        |
| Typed control values    | `data-state-key` / `name` / `id`               | value re-applied (hidden/submit/button skipped)                                         |
| Keyboard focus          | same key as controls                           | `focus()` onto the fresh subtree                                                        |
| `.journal-scroll` panes | DOM order                                      | `scrollTop` kept; bottom-pinned pane (within 24 px) re-pinned to the new `scrollHeight` |

**`data-state-key` is load-bearing**: the fallback (summary text) breaks
the moment a count in the summary changes. Every swappable `<details>`
and persisted control carries a stable `data-state-key` (set in
`internal/webui/fragments.templ`; no template-side guard test exists yet
— adding one is the a11y/harness row's half, not this contract's).

## Listener-drop invariant

`swapIn` replaces the subtree via `innerHTML`, dropping every per-element
listener on the old nodes. All interactivity is therefore **delegated at
`document` level** (click/keydown/submit/input/toggle). Never attach a
direct listener inside a fragment — it would die on the first swap.

## Server change-detection

Per-stream `filtersHTML` byte-comparison omits unchanged `frag-filters`
payloads from bursts (`handlers.go sendSnapshot`): a queued busy-deferred
filters fragment can only carry a real change.

## Verification

- Unit harness: `./scripts/check-webui-js.sh` (and `--self-test`; the
  pinned summary counters cover listener-drop, grace window, state-key
  restore, focus-after-swap, scrollTop pinning).
- Browser-level probes (chromedp captures, running>0 glow, forced-colors
  emulation) remain follow-up harness work — see TODO_LIST harness rows.
