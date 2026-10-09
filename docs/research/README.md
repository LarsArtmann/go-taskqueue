# Research notes

Point-in-time investigations kept for their citations and reasoning. Each
note names its source, pins what was verified, and separates "adopt" from
"rejected" so nobody re-litigates blind. Match the shape in
`docs/references/research-note-template.md`.

| Date       | Note                                                                                                                   | Source                                  | Why it matters                                                                                                    |
| ---------- | ---------------------------------------------------------------------------------------------------------------------- | --------------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| 2026-10-03 | [`2026-10-03_paperclip-competitive-analysis.md`](2026-10-03_paperclip-competitive-analysis.md)                         | paperclip (competitor)                  | Feature/positioning comparison; the format precedent for the lesson notes below.                                  |
| 2026-10-05 | [`2026-10-05_paperclip-lessons.md`](2026-10-05_paperclip-lessons.md)                                                   | paperclip                               | Engineering lessons that became the M-series TODO rows (budget cache, halt seam, wake).                           |
| 2026-10-05 | [`2026-10-05_go-cqrs-lite-metaengine-system-deep-dive.html`](2026-10-05_go-cqrs-lite-metaengine-system-deep-dive.html) | go-cqrs-lite (platform)                 | Deep-dive of the platform tq builds on (ADR-0019).                                                                |
| 2026-09-14 | [`2026-09-14_templ-components-deep-dive.html`](2026-09-14_templ-components-deep-dive.html)                             | templ-components (dependency)           | Web UI component-library adoption review.                                                                         |
| 2026-10-09 | [`2026-10-09_turnstone-lessons.md`](2026-10-09_turnstone-lessons.md)                                                   | turnstone (harness, the layer below tq) | 15 harness invariants checked against tq; produced the crash-reclaim effect-disposition fix + the narrowing rule. |

## Conventions

- **One source per note.** A note names exactly one external source; a
  sibling note covers a comparison.
- **Cite what you read.** The citations table names the exact files/paths
  read at source, not a summary — see the note template.
- **Adopt / reject explicitly.** Every lesson is DONE, a tracked TODO, or a
  recorded rejection. "Not applicable" is a valid, recorded outcome.
