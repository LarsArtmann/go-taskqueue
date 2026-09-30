# Docs-health full-corpus sweep: 352-file triage, 93-file archive, living-doc refresh

> Point-in-time snapshot of the 2026-09-30 ~07:45–08:25 docs-health session.
> Owner instruction: view ALL `**/2026-0*` files, execute the docs-health skill,
> archive fully-done-and-updated (inline strikethrough) files. AUDIT mode
> (HARVEST + VERIFY + ANNOTATE + ARCHIVE) over the whole live corpus.

## a) FULLY DONE

1. **Full-corpus triage of every live 2026-0\* file.** 352 tracked files —
   334 live status reports + 16 planning docs + 2 feedback drafts — read in
   full and verdicted per-item against CURRENT HEAD by 10 read-only
   sub-agents in date buckets (one 429 wave forced serial fan-out; verdict
   bar: shipped / routed-to-TODO-row / ruled / narrative, else KEEP).
2. **Archive sweep: 93 files moved.** 13 fully-done work windows carry
   inline per-item strikethrough verdicts (`done — shipped at HEAD: …` /
   `routed — TODO_LIST: …` / narrative) applied via the skill's
   `annotate-status-items.py` (emit-keys → verify → apply; atomic per file)
   and moved to `docs/status/archived/`; 79 verify-only re-dispatch
   duplicates carry the ratified whole-file `DUPLICATE — ARCHIVED` note
   naming their canonical; the executed companion-extraction design moved
   to `docs/planning/archived/` with an `EXECUTED` note. `check-rows.py`
   uniformity: 14/14 struck files complete.
3. **Citation hygiene.** All 92 moved status reports' index rows repointed
   to backticked `archived/…` paths; every repo-wide `docs/status/<name>`
   and the companion-plan path repointed (AGENTS.md, TODO_LIST.md, live +
   archived reports — 0 stale refs verified programmatically); counter
   347→439 reports / 26→27 plans; digest row added; status-report files
   334→243 live.
4. **TODO_LIST surgery.** 56 `[x]` rows purged per the DONE-DELETED ruling
   (commit `92216ac3`); 9 rows closed as verified-shipped at HEAD this
   session (cmd/tq LSP bullet, mvdan `&`-precedence bullet,
   edit→commit→battery codification, re-dispatch checklist, multi-file-tail
   + printf-`%.0s` hazards, `tq api` README row, `tq tasks --count`
   FEATURES row, httpauth CHANGELOG/FEATURES rows); the truncated duplicate
   `tq mcp` row (cut mid-sentence at "(JSON-") and an empty section header
   removed; 9 curated harvest rows minted with citations (flake tarball
   hash twin, evidence-stage surfaces, S1 divergence register, cqrsqlite
   disposition ruling, archive-evidence self-verify, crush twin-shadow
   WARN, Claude Channels spec research, README log-location section, the
   103-death rewind runbook). 268→258 open rows.
5. **Living-doc fixes.** FEATURES.md Release-runner cell reconstructed
   (truncated mid-flag since the 09-16 instrument pass, missing the cell
   text AND the closing pipe); ROADMAP's go-1.27 toolchain question struck
   DONE (go.mods + setup-go pins on 1.27.1); AGENTS.md/TODO_LIST citations
   to the archived plan repointed.
6. **Gates green at close:** check-todo-list, check-status-index,
   check-doc-refs, check-features-roadmap, check-rows (14 files), root
   build+vet rc=0 (GOEXPERIMENT=jsonv2 GOTOOLCHAIN=auto).
7. **Concurrent-window coordination.** The 07-56 and 08-20 windows landed
   mid-sweep; their reports were read, indexed (08-20 row added), and
   built on — including the 08-20 window verifying the very row this
   sweep's purge closed (it names `92216ac3` as the row's closer).

## b) PARTIALLY DONE

1. **243 KEEP-OPEN files stay live by convention.** Their unrouted items
   stay visible in-file (absence of a marker IS the open signal); only the
   ≤10 highest-value unrowed items were minted into TODO_LIST. The
   OPEN-MANY residue (hundreds of micro-items across the 09-15..09-29
   windows) is enumerated in the triage, not duplicated into the TODO file.
2. **Two duplicates have no same-ID canonical report** (the 09-24 replay
   tool and 09-29 e2e-trim windows) — their disposition notes name the DONE
   TODO row + work commit instead.
3. **`docs/planning/2026-09-13_verification-claims-guidance.md` left in
   place** — normative guidance still in force, not a snapshot; annotating
   or archiving it would misrepresent its status.

## c) NOT STARTED

1. The skill's `grep -rLn '~~'` completeness gate over `archived/` stays
   deliberately unenforced — the 79 duplicates carry the ratified
   disposition-note form (12-58 §c1 stands); inline strikethroughs were
   applied exactly to the fully-done class the owner demanded.
2. `scripts/check-archive-eligibility.sh` (row 281) remains unbuilt; this
   was the second judgment-scale sweep it would have mechanized.

## d) TOTALLY FUCKED UP (honest defects, all caught in-session)

1. **Turn-1 ritual skipped** — no `scripts/session-start.sh`, no
   CONTRIBUTING read until close-out (read it there); the documented
   recurring miss, re-hit.
2. **The move manifest omitted the 13 DONE files** — they sat
   annotated-but-unmoved until the status-index BODY-DATE-DRIFT check
   flagged them; caught by the gate, moved in the same session.
3. **A 429 rate-limit wave** — launched 2 triage sub-agents concurrently
   into a busier fleet; dropped to serial fan-out for the rest.
4. **Agent-emitted line numbers were approximate** — 4 of 13 strike
   batches failed emit-keys matching; fixed by re-deriving item-start
   lines from the files themselves; the annotator's atomicity caught every
   mismatch (zero mis-strikes).
5. **Redundant strikes attempted on an already-struck file** (the 09-10
   02-00 report was fully struck by the 09-14 pass) — the tool's
   already-struck refusal was the catch; disposition note adjusted instead.
6. **Digest-row live-count first written from file math (242)** instead of
   the gate's live-ROW count (257, which includes pre-existing unbackticked
   stale rows); corrected in the same session.

## e) WHAT WE SHOULD IMPROVE

1. `check-archive-eligibility.sh` would have caught the missed-move class
   mechanically (d.2).
2. The re-dispatch loop remains the corpus's mass generator: 79 of 93
   archives were verify-only duplicates — the mint-time done-check row
   (200) and duplicate-claim ruling row (231) are the systemic fix.
3. The strict sub-agent output contract (FILE/CLASS/STRIKE/OPEN blocks)
   made 352-file triage mechanical and auditable — keep it as the sweep
   playbook.
4. The dead-SHA repo-wide debt (438 unreachable-cite findings at close,
   grown from ~660-measured baseline by later heals — row 332 owns the
   mass triage) is starting to drown the gate's signal for NEW dead cites.

## f) UP NEXT (owned by existing rows, not re-minted)

1. Row 332: mass dead-SHA triage (438 findings).
2. Row 281: the archive-eligibility gate.
3. Master CI is RED at 470508620 — code legs at a docs-only HEAD, triaged
   as pre-existing by the 07-56/08-20 windows; needs the owner push of the
   heal.
4. The 9 rows minted in this sweep's harvest section (flake hash twin,
   evidence-stage surfaces, divergence register, cqrsqlite ruling, …).

## g) OWNER QUESTIONS

1. Digest-vs-sweep cadence (standing README question): 9 days between
   sweeps produced a 92-file cleanup; would weekly mini-sweeps be the
   standing ritual instead?
2. Archive-sweep strikethrough scope: this pass struck ALL numbered items
   in fully-done windows (the 00-43 precedent) — ratify, or restrict to
   forward-looking items only?
3. Given 79/93 of this sweep's archives are same-ID verify-only
   re-dispatches: pull the mint-time done-check (row 200) forward as the
   next queue-side work item?
