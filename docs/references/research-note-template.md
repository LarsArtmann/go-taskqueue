# Research-note template

Copy into `docs/research/<YYYY-MM-DD>_<source>-<topic>.md` and fill every
section. Delete this line and the guidance in italics. The shape is the
precedent from `docs/research/2026-10-03_paperclip-competitive-analysis.md`
and `docs/research/2026-10-09_turnstone-lessons.md`.

---

# <Source> lessons for go-taskqueue

**Date:** <YYYY-MM-DD> (source-verification window).
**Source:** `<org>/<repo>` (<URL>) @ `<pinned-sha-or-tag>` (<language>,
<license>). _State whether docs-only or code-read; name the exact files read._
**Companion notes:** <sibling research notes, or "none">.
**stale_after:** <YYYY-MM-DD>. _An ABSOLUTE date, not a TTL ("stale after six
months" cannot be evaluated later without doing the arithmetic). Set it to the
date this note is likely to be wrong — when the source's next major version, or
tq's next release, is expected to invalidate it. Consumers treat a past
`stale_after` as "re-read the source before relying on this", never as "delete
it" (the OKF freshness lesson: an absolute instant, not a relative one)._

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

_Two obligations, and the first one is the one that gets skipped._

- **Acquisition passes.** Record every attempt to obtain the source, in order,
  with its outcome — including the ones that failed. `fetch` returning an empty
  body is a TOOL MISMATCH, not a dead URL: retry with `download` and treat the
  empty result as an acquisition event to write down. A trail that says "the
  citations were fetched from <urls> and read directly" while every fact came
  through an intermediary is the 2026-11-01 false-verification failure.
- **Cite-then-verify.** Write the `file:line` cite, then RE-READ that exact
  line before committing. Four cites in the 2026-11-01 note were off by one or
  two lines because the numbers were written from memory of the file, not from
  a read of it. The mechanical check, run over the files you touched:

  ```bash
  ./scripts/check-cite-drift.sh docs/research/<your-note>.md
  ```
- tq-side claims verified against the working tree the same window:
  `<file:line>`, ….
