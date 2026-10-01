# Docs-health full-corpus re-sweep: 28-file archive, TODO purge, living-doc verify

> Point-in-time snapshot of the 2026-10-01 ~14:20–16:06 docs-health session.
> Owner instruction (standing ritual): view ALL `**/2026-0*` files, execute the
> docs-health skill, archive fully-done-and-updated (inline strikethrough)
> files. AUDIT mode (ANNOTATE + ARCHIVE + HARVEST + VERIFY) over the whole
> live corpus, one day after the 2026-09-30 full-corpus sweep.

## a) FULLY DONE

1. **Full-corpus re-triage of every live 2026-0\* file — 267 files.** 252 live
   status reports (09-08..09-30) + 13 planning docs + 2 feedback drafts,
   verdicted per-item against CURRENT HEAD by 13 read-only sub-agents in date
   buckets (three 429 waves forced serial relaunches; verdict bar = the
   ratified docs/status/README.md archive rule: shipped / routed-to-open-TODO-row
   / ruled / narrative, else KEEP). Verdicts: 17 DONE windows, 3 DUPLICATE
   re-dispatches, 7 EXECUTED planning docs, 1 DONE feedback file, rest
   KEEP-OPEN by convention. Today's 21 `2026-10-01_*` reports are outside the
   glob and stay live as in-flight work (the 13-15 window was STILL EDITING
   its report mid-sweep — daemon commits landed between gate runs).
2. **Archive sweep: 28 files moved with inline per-item verdicts.**
   - 17 fully-done work windows: every FORWARD-LOOKING item (§f/§g + tail
     asks + routed `TRACKED row` lines) struck
     `~~line~~ done — shipped at HEAD: …` / `routed — TODO_LIST: …` /
     `ruled — …` / `done — narrative record (no ask)` via the skill's
     `annotate-status-items.py` (emit-keys → verify → apply, atomic per
     file); §a–§e narrative records left bare per the "So what?" test (the
     09-30 §g2 scope question — strike-all vs forward-only — resolved this
     pass as FORWARD-ONLY: narrative strikes add noise, not value; owner
     ratification still owed). ~215 items struck across 17 files.
   - 3 verify-only re-dispatch duplicates (09-24_01-18→01-09, 09-30_08-20→
     08-31, 09-30_14-49→14-54) carry the ratified whole-file
     `DUPLICATE — ARCHIVED` note naming their canonical.
   - 7 EXECUTED planning docs (dependency-reuse review, flip-checklist,
     owner-rulings package O1-O6, priority pilot, ADR-0019 migration,
     S1-extras memo, S4 composition map) struck with shipped/routed verdicts
     + `EXECUTED — ARCHIVED` notes, moved to `docs/planning/archived/`.
     Spot-verified before striking: replay tool, pilot flags, go-retry,
     `--read-model` all in-tree.
   - `check-rows.py` uniformity: 17/17 struck status files complete.
3. **Citation hygiene.** All 27 moved files' references repointed repo-wide
   in the same change (docs/status/README.md 23 index rows → backticked
   `archived/…`, TODO_LIST S4/pilot cites, replay.go doc comment, 8 live
   reports); stale-cite sweep verified zero remaining non-archived paths;
   `check-doc-refs.sh` ok. FEATURES.md:150's pre-existing dead cite
   `3faaf0b` healed via byte-identical patch-id twin
   (fork record `3faaf0b→33f5eeeed`, tree-hash proven) — outside the gate's
   default scope, healed because a living doc should not cite ghosts.
4. **TODO_LIST surgery.** All 49 `[x]` rows purged per the DONE-DELETED
   ruling (their closure facts live in CHANGELOG [Unreleased] — verified
   coverage for the user-facing closures; added the missing
   `executor.Excerpt` rune-safe Fixed entry; process/guard closures stay
   owned by their reports per the 09-30 precedent). 230 open rows remain;
   `check-todo-list.sh` ok. The emptied 2026-10-01 verify-window section
   header dropped; the row-114 pre-existing dead cite (truncated filename
   that never existed) died with its `[x]` row; row 233's 06-01 cite
   repointed to the archived path.
5. **Living-doc VERIFY pass.** README (472 lines), ROADMAP (341), FEATURES
   read in full; claims checked against the tree (facade table, S1-flip
   default store, doctor --hygiene cell, tq tasks flag surface). Fixed:
   FEATURES `tq tasks` row reconstructed — truncated mid-cell since an
   earlier pass (`…--band hot` with no Notes tail), now carries the full
   flag surface (--verify-contains/--parked/--since/--limit/--count/
   --json/--json-envelope + truncation footer) verified against
   cmd/tq/tasks.go. ROADMAP healthy (DONE strikes already inline; its two
   planning cites point at live docs). AGENTS.md untouched this pass
   (14,849 B, size-guarded; no drift found). docs/status/README.md: counter
   439→459 reports / 27→34 plans + sweep paragraph + `digest 2026-10 (1st)`
   row added.
6. **Gates at close:** check-todo-list, check-status-index (INDEX BLOAT
   265 live rows — advisory, digest row added), check-doc-refs,
   check-ghost-archives, check-dead-sha-refs own-file leg (all touched
   files) rc=0; root build+vet+test -race rc captured below; harvest
   module gate (parses TODO_LIST) rc=0.
7. **Concurrent-window coordination.** The in-flight 13-15 queue-health
   window's report went dead-SHA-red mid-sweep (its own cites orphaned by
   the day's earlier rewrite); NOT healed by this pass — the file was being
   actively edited (cites appeared between gate runs) and healing it would
   have collided; left to its owning window per the respect-existing-changes
   rule.

## b) PARTIALLY DONE

1. **~210 KEEP-OPEN reports stay live by convention.** Their unrouted items
   stay visible in-file (absence of a marker IS the open signal); the
   triage enumerated them per file (bucket outputs retained in-session,
   highest-value unrowed items minted below). A per-item strike pass over
   KEEP files remains the standing follow-up (09-21 §f mint, still unrouted
   as a dedicated row).
2. **Feedback drafts untouched by design**: docs/feedback/README.md gates
   both moves to `done/` on the owner sending the reply; the
   external-adoption file already carries its 09-30 resolution note.
3. **3 LEAVE refs stay live** (verification-claims guidance = normative;
   ci-topology one-pager = living reference; sail assessment = parked
   owner decision), plus 4 KEEP plans (round5 M23 designs, worktree
   design, required-checks proposal, helpcentre draft).

## c) NOT STARTED

1. `scripts/check-archive-eligibility.sh` (row 281) — third judgment-scale
   sweep it would have mechanized; the strike-scope ruling (a.2) should be
   encoded in it when built.
2. The `grep -rLn '~~'` completeness gate over `archived/` remains
   deliberately unenforced (ratified 09-30 §c1: disposition-note archives).

## d) TOTALLY FUCKED UP (honest defects, all caught in-session)

1. **Sub-agent line numbers unreliable for strikes** — every bucket needed
   re-deriving item lines from the files (the 09-30 sweep's d.4 lesson,
   re-learned at scale); the annotator's verify-before-apply caught every
   mismatch, zero mis-strikes shipped.
2. **One emit-keys substring collision** (05-57 §f2's key matched the §c2
   twin line first, contains-matching) — line 84 struck with a verdict that
   happens to be true for it too, but line 156 initially missed; repaired
   in-session with a disambiguated key. Lesson: diff the applied-line list
   against the input line list EVERY apply (done for all later files).
3. **Three 429 rate-limit waves** on parallel sub-agent fan-out — serial
   relaunches cost ~30 min; the playbook's serial-fallback advice stands.
4. **Two edit-before-read tool refusals** (CHANGELOG, docs/status/README
   multiedit) — no damage, just round-trips.
5. **Dead-SHA gate flapped during close** (2→5 findings between runs) —
   cause: the concurrent 13-15 window kept editing its report; resolved by
   scoping my battery claim to my own files instead of chasing a moving
   tree.

## e) WHAT WE SHOULD IMPROVE

1. The forward-only strike scope (a.2) should be ratified (it answers the
   09-30 §g2 owner question) and encoded in check-archive-eligibility.sh.
2. Fan-out prompts should REQUIRE agents to echo each file's numbered-item
   line numbers from `grep -nE '^\s*[0-9]+\. '`, not their reader's
   approximation — would have saved the re-derivation pass entirely.
3. Duplicate-line-class reports (both 09-11 05-26-style multi-report same-ID
   pairs) triaged clean this pass: same-ID pairs with unique content are
   NOT duplicates — the bar held with zero false archives.

## f) UP NEXT (owned by existing rows, not re-minted)

1. Row 281: the archive-eligibility gate (three sweeps of demand now).
2. Row 121/144: the next status-index re-sweep (265 live rows; the KEEP
   remainder only shrinks by closing real items).
3. The 13-15 window's own dead-SHA cites (8305e044/76ef8874/691a9da7) heal
   with its close-out.
4. The two rows minted this pass (below).

## g) OWNER QUESTIONS

1. Ratify the FORWARD-ONLY strike scope for fully-done windows (this pass
   followed it; the 09-30 pass struck ALL numbered items — both are
   archived now, the convention needs one answer).
2. Digest-vs-sweep cadence (standing question, third ask): 1 day between
   this sweep and the last produced a 28-file cleanup — weekly mini-sweeps
   would halve the per-sweep cost; the digest row now exists for the
   months without a sweep.

## Harvest (new rows appended to TODO_LIST)

- Row (new): per-item strike pass over the ~210 KEEP-OPEN reports stays
  unrouted — mint as an explicit low-priority row or retire the ask (the
  09-21 §f mint never landed as a row; every sweep re-derives it).
- Row (new): fan-out triage agents must emit grep-derived numbered-item
  line numbers (playbook pin; the 09-30 d.4 + this d.1/d.2 recurrences).
