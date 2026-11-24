# ADR-0024: Two-voice typography and lowercase-below-the-hull

Date: 2026-11-24
Status: Accepted
Decided: 2026-09-16 (first pass) and 2026-09-21 (refinement, explicit
bezel carve-out) — recorded here as one artifact per design-pass §f38.
Depends on: the webui theme system (theme.css is the design source of
truth, compiled into `static/app.css` by `scripts/build-webui-css.sh`).

## Context

The `tq serve` dashboard grew by accretion: typography inherited browser
defaults, labels mixed uppercase, lowercase, and sentence case at random,
and machine facts (ids, timestamps, counts) were visually identical to
human content (project names, error prose). During the 2026-09-16 and
2026-09-21 webui design passes, the owner ruled on a deliberate
typographic system instead of per-element taste. The rulings lived in
pass reports and the theme.css header; this ADR promotes them to a
decision record so future surfaces (new fragments, `/health` consumers,
the board) extend the system rather than diverge from it.

## Decision

1. **Two typographic voices, one rule: machine facts and human names
   never share a face.**
   - **Machine voice = JetBrains Mono**: wordmark, headings, stat values,
     tags, timestamps, ids, commands.
   - **Human voice = system sans**: project names, prose, error text.
   - A table (or card) scans by voice before it scans by value. When a
     new surface renders a fact next to a name, the face answers "which
     is which" with zero labels.
2. **Label discipline: engraved uppercase is BEZEL-only; everything
   below the hull is quiet lowercase.**
   - Topbar and nowband (`#conn` lamp, engraved micro-labels) keep
     uppercase — the bezel chrome is a different register by explicit
     ruling (2026-09-21 pass §2).
   - Below the hull, every label follows the lowercase terminal
     convention (the 2026-09-16 sweep: view toggle, filter-chip kind,
     "clear all", fold summaries, column headers, band labels).
   - Lowercase is the DEFAULT for new labels; a new uppercase label is a
     bezel-placement decision, not a styling choice.
3. **Two-voice application is deliberate, not inherited.** Where a line
   mixes machine and human content (result-card usage lines), the split
   is explicit per segment (`sessionUsageSegments`: numbers in
   `font-mono` spans, prose sans) — pinned by
   `TestSessionUsageLineTwoVoice`.

## Consequences

- Presentation polish that re-voices EXISTING text with no capability or
  behavior delta owes NO `[Unreleased]` CHANGELOG row (ruled 2026-11-23;
  O14's bar is a new surface/capability, a behavior change, or an
  operator-facing fix — see TODO_LIST row 2110's receipts).
- The vocabulary is glossary-bound: "bezel" and "queue chip" live in
  docs/DOMAIN_LANGUAGE.md (§ Webui design vocabulary).
- The lowercase coherence contract extends to sister surfaces: the
  `/health` page renders the go-health-dashboard's own `tc-*` classes and
  httpapi is JSON-only, so neither couples to the tq label voice (pinned
  by `TestLowercaseLabelScope` — the two `text-transform:lowercase`
  rules in theme.css are `tq-*`-scoped).
- New webui surfaces should read theme.css's header comment (the live
  design source of truth) and cite it; this ADR explains WHY the header
  says what it says.

## Sources

- 2026-09-16 design pass (dashboard typography overhaul).
- 2026-09-21 design pass (§a2 lowercase sweep + bezel ruling, §a1
  two-voice discipline, §a3 queue chips) —
  `docs/status/archived/2026-09-21_13-31_webui-design-pass.md`.
- `internal/webui/theme.css` header (live design language).
