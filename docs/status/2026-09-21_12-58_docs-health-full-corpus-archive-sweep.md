# Docs-health full-corpus AUDIT: 209-report archive sweep + TODO purge + living-doc refresh

> Point-in-time snapshot of the 2026-09-21 ~12:00–13:30 docs-health session.
> Owner instruction: view ALL 2026-0* files, execute the docs-health skill,
> archive fully-done-and-updated files. Full AUDIT mode (BUILD + HARVEST +
> VERIFY + ANNOTATE).

## a) FULLY DONE

1. **Full-corpus triage of EVERY live 2026-0* file.** 363 live status
   reports (317 task close-outs + 46 named) + 22 live planning docs,
   triaged by 11 read-only sub-agents in date buckets (≤2 concurrent per
   the standing rate-limit lesson; zero 429s), each file read and every
   forward-looking item (§e/§f/§g, next/questions lists) verdict-checked
   against the tree and TODO_LIST routing.
2. **Archive sweep: 221 files moved.** 209 status reports — 198
   verify-only re-dispatch DUPLICATES (whole-file disposition naming the
   canonical), 9 fully-done work windows (per-item verified: 09-10 01-55/
   05-29/05-58, 09-14 02-48/17-35, 09-16 02-05, 09-17 00-25, 09-19 07-00,
   09-21 01-45), 2 pure records — plus 12 EXECUTED planning docs into
   `docs/planning/archived/`. Live index rows 365 → 158.
3. **Citation hygiene.** Every reference to a moved file repointed
   repo-wide (49 files touched); 209 index rows repointed to backticked
   `archived/…` paths; counter updated (137→346 reports, 14→26 plans,
   exact `ls | wc -l` math); digest row rewritten for this sweep.
4. **TODO purge: 100 `[x]` rows deleted** per the owner's
   DONE-ITEMS-ARE-DELETED ruling (safe: `--prune-stale` semantics);
   CHANGELOG coverage sampled first (derived-session-usage, score-cache
   aging/eviction, board-band entries all present; known gaps stay routed
   via rows 204/66). 169 open rows survive; empty sections collapsed.
5. **HARVEST: 20 curated rows minted** in a dedicated section (ci.yml
   gate-parity sweep, session-bridge CHANGELOG catch-up, CONTRIBUTING
   accuracy pass, gosec stamped-binary + provenance, budget cap-unit
   ruling, score-TTL ruling, session-sweep automation, dep-sweep e2e,
   questions-loop surfaces, review-anchor residue, archive-eligibility
   gate, round-10/11 micro-residue, webui-health bundle, P2 webui batch,
   legacy TQ_RESULT deletion, …) — each with report + code citations,
   deduped against the 149 pre-existing open rows, BLOCKED-marked where
   owner-gated; `check-todo-list` green.
6. **VERIFY fixes:** ci-topology one-pager refreshed (nix-job css-pin
   step, lint-annotations loop truth, concurrency group, capture-form +
   GOTOOLCHAIN migration context — row done-on-arrival and deleted);
   AGENTS.md templ-components v1.16.x→v1.17.0; ROADMAP concurrency row
   struck DONE; two dead-SHA citations healed with arrow-form fork
   records (0f0c1bf→66d65ef amend twin, 4f696cc→74fb357 patch-id twin —
   concurrent windows' red, fixed on sight); dogfood-proof SHA256SUMS
   regenerated after the citation repoint touched its README.
7. **Conventions written down** (17-15 §f3 ask): the archive-sweep
   conventions paragraph now lives in `docs/status/README.md` (archive
   bar, disposition forms, citation repointing rule, BODY-DATE exemption
   rationale). AGENTS.md verify-gate line now carries
   `GOTOOLCHAIN=auto` with the go-1.27.1 env-lie precedent.
8. **Gates green at close:** check-status-index, check-doc-refs,
   check-todo-list, check-features-roadmap, check-dead-sha-refs,
   check-ghost-archives, root build+vet (GOEXPERIMENT=jsonv2
   GOTOOLCHAIN=auto).

## b) PARTIALLY DONE

1. **Per-item inline strikethroughs inside the 158 KEEP-OPEN live
   reports were NOT applied this pass.** The full-corpus triage produced
   the verdicts, but striking resolved items inside live files is a
   dedicated per-item pass (the 17-15 precedent: ~1.3k markers over 39
   files took a full session). The open items of every KEEP-OPEN file
   are now minted/visible in TODO_LIST, making each file's future
   archiving a re-triage away.
2. **`check-archive-eligibility.sh` not built** (minted as a row — the
   judgment version scaled to 363 files once, barely).
3. **CHANGELOG entry for the sweep itself**: none (docs-only; policy
   row 66 BLOCKED — consistent with all prior sweeps).
4. **Annotation depth on the 9 fully-done windows**: disposition notes
   with per-item evidence summaries; files already carrying 14-Sep
   strike sets (05-29, 05-58, 02-48) were accepted as-is rather than
   re-struck.

## c) NOT STARTED

1. The skill's generic `grep -rLn '~~'` completeness gate over archived/
   is deliberately NOT enforced: the repo's ratified convention is
   disposition NOTES for archived files (whole-file DUPLICATE / ARCHIVED
   / EXECUTED forms), not retro-strikethroughs of historical text — 104
   pre-existing archived files follow the note convention.
2. check-rows.py table-row completeness: no table-row annotations were
   made this pass (dispositions only).

## d) TOTALLY FUCKED UP (honest defects, all caught in-session)

1. **Build first "failed":** ran the verify gate with the session
   shell's `GOTOOLCHAIN=local` on go 1.26.7 against go.mod's 1.27.1 —
   the documented env-lie family, hit fresh. Fixed with
   `GOTOOLCHAIN=auto` and codified into AGENTS.md; the first rc=1 was an
   environment lie, not a regression.
2. **Dead-SHA gate went red mid-sweep from CONCURRENT windows**
   (0f0c1bf→66d65ef/4f696cc→74fb357 citations landed after the last
   ci-local run): healed on
   sight with patch-id/amend-twin arrow records rather than blaming the
   sweep; one site needed three whack-a-mole iterations before I grepped
   ALL sites at once — should have grepped globally on the first hit.
3. **The citation repoint changed the dogfood-proof README hash**,
   tripping the ghost-archive STALE manifest check — a foreseeable
   interaction (any file change under an archive requires SHA256SUMS
   regen); caught by the gate, regenerated.
4. **Two triage agents double-covered boundary files** (09-15 02-44/46,
   09-20 07-07/13/19/21) — harmless overlap, verdicts agreed; the
   boundary-whole-group instruction made the dup list generation
   mechanical (later same-ID minus 7 agent-flagged work-window
   exceptions), which caught the agreement.
5. **TODO purge executed before the harvest mint** — meant the harvest
   dedup had to run against the purged list; no row duplication
   resulted, but the safer order is mint-then-purge.

## e) WHAT WE SHOULD IMPROVE

1. The mechanical archive-eligibility gate (minted) would have made the
   198-duplicate classification a script run instead of 11 agents.
2. The re-dispatch loop itself is the corpus's biggest generator
   (198/209 archives were verify-only duplicates): the DONE-on-arrival
   dispatch dedup rows (209/280/313) are the systemic fix — the sweep
   treats the symptom.
3. Sub-agent triage at 2-concurrent worked with zero 429s; keep it the
   standing fan-out number.
4. A periodic (weekly) mini-sweep beats the quarterly big-bang: live
   rows grow ~10/day and the threshold trips at 100.

## f) UP TO 50 THINGS TO GET DONE NEXT

_(1–20 are minted as the "Docs-health harvest (2026-09-21 archive
sweep)" TODO rows verbatim; 21+ are this session's own residue.)_

21. Per-item strikethrough pass over the 158 KEEP-OPEN live reports
    (start with the 52 pre-annotated ones — extend, don't redo).
22. Build `scripts/check-archive-eligibility.sh` (minted row).
23. Digest-row-vs-sweep cadence ruling (docs/status/README.md owner
    question, still open).
24. Fold the 01-59 §f6 one-pager row-number reconciliation if the
    one-pager gains rows again.
25. Re-run `./scripts/ci-local.sh` end-to-end (row 190) — this session
    ran the doc gates + root build/vet only.

## g) OWNER QUESTIONS

1. Same as the harvest rows' BLOCKED markers: budget cap unit
   (token-vs-count), score-TTL calibration, gosec provenance, session
   sweep cadence — all rider rulings on shipped work.
2. Should the `[x]`-deletion purge also apply to TODO rows whose work
   shipped but whose CHANGELOG entry is missing (policy row 66 still
   BLOCKED)?
