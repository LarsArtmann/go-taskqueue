# WebUI Operator-Stance Audit — M9 findings

**Audited:** 2026-10-05, `internal/webui` at `31b1998b` (read-only audit; no code changed by this document).
**Method:** screen-by-screen walk of the templ components and render paths against paperclip's three operator questions — (1) what is happening? (2) does it need me? (3) what do I do? Every finding was verified against the code at HEAD; "already fixed at the seam" findings are kept and marked as such so the next auditor does not re-chase them.
**Feeds:** M10 (systematic fixes). Disposition column says what happened.

## Screen inventory and verdicts

| # | Screen / surface | What is happening? | Does it need me? | What do I do? | Verdict |
|---|------------------|--------------------|------------------|---------------|---------|
| 1 | Nowband status cards (`StatusCards`, fragments.templ:25) | per-status counts, hot/alarm emphasis on running/dead | dead>0 lights alarm + a dedicated fault banner below | banner links straight to the dead view | GOOD — the model answer |
| 2 | Nowband meta row (`fragments.templ:56-85`) | parked / budget / loop-suspects / sessions / journal watermark / budget-today meter | parked and budget lamps only render when > 0 (no alarm fatigue); loop suspects names its threshold | tooltips name `tq stats` / `tq session list` | GAP F1: the loop-suspect and parked lamps are plain spans — the "what do I do" target is a CLI command the operator must type, while everything else on the page is a link |
| 3 | Task table — active tier (`taskActiveCard`) | queued+running always visible, priority-order explained in a footer note | active tier never collapses | row actions: stop/cancel (with reason form, CSRF) | GOOD |
| 4 | Task table — settled tier (`taskSettledDetails`) | completed/cancelled/dead fold behind a native `<details>` with counts | silent unless expanded (correct: settled work needs nobody) | expand to inspect; dead rows inside also show errors | GOOD |
| 5 | Task rows (`taskRows`, `statusCell`) | statuses lowercase; a RUNNING row with an expired lease renders `STALE` + remaining-lease countdown on healthy leases | STALE is the crash-reclaim candidate alarm | cancel/stop action on the row | GOOD — deliberate alarm marker, not vocabulary drift (F2 records the reasoning) |
| 6 | Dead-letter table (`DeadLetterTable`, `errorCell`) | attempts N/M, expandable error preview (title + data contract for row expansion) | table only renders when non-empty; empty state explains what lands there | rescue action (fresh attempt budget, gated on `--allow-writes`) | GOOD |
| 7 | Fact feed (`FactFeed`) | journal tail, newest last, per-type tone classes, task-id links | cancelled facts carry a distinct tone | click through to the task | GOOD |
| 8 | Board (`Board`, `boardColumn`) | one lane per lifecycle status, true counts, priority-band grouping, newest cards | honest `+N older` escape hatch into the filtered table | links into table view; lanes are read-only (ADR-0003 Phase D) | GOOD |
| 9 | Filter bar (`FilterBar`) | active filters as removable chips | — | one-click clear / chip removal | GOOD |
| 10 | Task detail (`TaskDetail`) | id + copy, status badges, record definition list, payload spec grid, timeline, retry trail | verdict/review/status/agent/prioritize result cards; `parked: budget` lamp with the why-inline | cancel/stop/rescue contextual to status; payload copy | GOOD |
| 11 | Retry strip (`retryStrip`) | distinct requeue/failure reasons ×counts, loudest first | — | reason text tooltips carry the full reason | GOOD |
| 12 | Agent run / status report / ai scoring cards | outcome badges + derived session usage (cost, tokens, messages) + log path | usage rendered whenever non-zero | copy buttons for paths | GAP F3: the usage line is a proportional-font gray span — cost/tokens are machine values and scan poorly next to the mono everywhere else |
| 13 | Priority provenance (`priorityProvenanceSection`) | stored value + band + harvested item + AI verdict + history | — | mirrors `tq show` | GOOD |
| 14 | Empty states (`taskEmptyState`, DLQ, feed) | one shared copy per state; filtered vs empty distinguished | honest | — | GOOD |

## Findings

| ID | Finding | Evidence | Severity | Disposition |
|----|---------|----------|----------|-------------|
| F1 | Loop-suspect and parked lamps are plain spans; the "what do I do" answer (a CLI command) is tooltip-only while every other element is a link | fragments.templ:57-63 | LOW | FOLLOW-UP (needs a real target view before a link lies — a `?parked=` filter exists now via `tq tasks --parked-class`, a webui equivalent is new surface). Left as finding, not half-fixed. |
| F2 | "STALE" is uppercase while statuses are lowercase — flagged as candidate vocabulary drift, verified DELIBERATE: it is an alarm marker on an expired lease, not a status token | components.go:34 comment, `statusCell` | NONE (verified) | CLOSED — document only (this row) |
| F3 | Session-usage line (cost/tokens/messages) renders proportional-font in all three result cards — the one remaining machine-value-not-mono surface | fragments.templ:731-733, 754-756, 776-778 | LOW | **M10.3 FIX** — extract one shared component, render mono |
| F4 | "Stale execution errors hide when superseded" (M10.4): verified ALREADY TRUE at the store seam — every engine clears `last_error` in the same transaction as completing (`UPDATE tasks SET last_error = '' WHERE id = ? AND status = 'completed'`), so a completed task can never show a red last-error alert | sqlitev4/adapter.go:185, postgresv4/adapter.go:228 | NONE (verified) | CLOSED — satisfied by the store contract |
| F5 | "Late terminal outcomes refresh silently" (M10.5): verified NO toast/notification system exists — SSE patches content in place, so a late terminal outcome updates the row/badges without an interruption surface | handlers.go SSE fragments; no toast component in the module | NONE (verified) | CLOSED — satisfied by absence; keep it that way |
| F6 | "Expected cancellation renders neutral" (M10.6): verified — cancelled is gray in the status vocabulary (components.go:34), the fact feed gives cancelled its own tone, and the cancel action is a deliberate two-step form with reason | components.go:34, fragments.templ:277-297 | NONE (verified) | CLOSED — satisfied |

## Summary

The webui already answers the three questions on every primary surface; the two-tier active/settled table, the fault banner, the two-step actions, and the honest empty states are exactly the paperclip stance. One real fix came out (F3 → M10.3), one follow-up is recorded for when it can be done honestly (F1), and three suspicion-as-findings were verified as already satisfied or deliberate (F2, F4-F6) — recorded here so the next window does not re-audit them from scratch.
