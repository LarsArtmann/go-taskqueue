# Research-note template

Copy into `docs/research/<YYYY-MM-DD>_<source>-<topic>.md` and fill every
section. Delete this line and the guidance in italics. The shape is the
precedent from `docs/research/2026-10-03_paperclip-competitive-analysis.md`
and `docs/research/2026-10-09_turnstone-lessons.md`.

---

# <Source> lessons for go-taskqueue

**Date:** <YYYY-MM-DD> (source-verification window).
**Source:** [`<org>/<repo>`](url) @ `<pinned-sha-or-tag>` (<language>,
<license>). _State whether docs-only or code-read; name the exact files read._
**Companion notes:** <sibling research notes, or "none">.

## Why <source> is worth reading

_One short paragraph: what the source is and the specific reason its evidence
is actionable for tq. If it is a layer below/above tq, say so._

## Source-verified citations

_Every load-bearing claim gets a row. The "what it says" cell quotes or
paraphrases the SOURCE, not a summary of it._

| # | Path (`<repo>@<sha>`, verified <date>) | What it actually says |
| - | -------------------------------------- | --------------------- |
| 1 |                                        |                       |

## Lesson table — what <source> does, where tq stands

| # | Lesson | <Source> (verified) | tq before | tq now / action                          |
| - | ------ | ------------------- | --------- | ---------------------------------------- |
| 1 |        |                     |           | **ALIGNED / ADOPT / SHIPPED / ROADMAP.** |

## Rejected / not applicable (so nobody re-litigates blind)

_List what does NOT transfer and why. "Not applicable" is a recorded outcome,
not a gap._

## Verification trail

- <date>: the citations above were fetched from <urls/method> and read
  directly; no claim here rests on a subagent summary.
- tq-side claims verified against the working tree the same window:
  `<file:line>`, ….
