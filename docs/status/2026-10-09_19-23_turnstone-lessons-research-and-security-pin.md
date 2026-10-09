# Turnstone lessons research + learned-check security pin

**Date:** 2026-10-09 19:23 (+0200) · **HEAD:** `fdbba7fe`
**Session type:** interactive owner research window (NOT a dispatched queue task;
no `Task-Queue-ID`, no closeout report in `tasks/`).
**Provenance:** all source claims read directly from
`raw.githubusercontent.com/turnstonelabs/turnstone/main/…` this window (PRIMER,
HYPOTHESIS, docs/architecture, docs/security, docs/judge, docs/governance). All
tq-side claims read from the working tree the same window.
**Artifacts:** `docs/research/2026-10-09_turnstone-lessons.md` (new),
`SECURITY.md` (+ "Learned checks may only narrow"), `TODO_LIST.md` (+ Turnstone
lessons section, 5 rows).

## a) FULLY DONE (verified at HEAD, not just claimed)

- **Source research.** Read the six load-bearing turnstone files at source and
  captured a source-verified citations table (6 rows) + a 15-row lesson table in
  `docs/research/2026-10-09_turnstone-lessons.md`. Each lesson names what
  turnstone does (verified), where tq stood, and the disposition.
- **Framing landed.** The note's central claim — turnstone's *harness* is what
  tq's `agent` executor delegates to (`crush`); **tq IS turnstone's "the loop"**
  one level up — is stated with the HYPOTHESIS "The loop" quotes that make it
  act on tq's own gate/auto-dismiss class.
- **Real gap found + filed.** Effect-disposition on crash reclaim: an expired
  lease is `Released` and re-claimed as if nothing happened
  (`internal/queue/companion/claims.go:165`); a crash mid-effect (push/PR/API) is
  indistinguishable from a clean run. Filed as the top TODO row with turnstone's
  `EffectStatus` enum as the target shape.
- **Security pin SHIPPED.** `SECURITY.md` gained "Learned checks may only
  narrow": a deterministic check owns its verdict; a learned check may only
  tighten (`max` + union; never lower a tripwire; never grant authority;
  auto-dismiss stays deterministic). Cites the turnstone invariant + the note.
- **4 TODO rows minted** (agent-executable, evidence-cited): effect disposition,
  verdict→outcome calibration dataset, narrowing regression guard, child-task
  authority/budget ceiling, model-keyed priority cache.
- **Guards green:** `check-todo-list` rc=0, `check-doc-refs` rc=0,
  `check-agents-size` rc=0 (18491/18500 B, untouched), `check-status-index` rc=0
  (warn-only pre-existing messages).
- **Line-pin corrected:** an early `claims.go:158-175` cite was wrong; fixed to
  the exact append at `:165` in both the note and the TODO row after verifying
  with `grep -n`.

## b) PARTIALLY DONE

- **"Execute the lessons" vs "file the lessons."** Only ONE lesson (the two
  learned-check rules) reached working-tree prose; the other four are TODO rows
  waiting on the harvest/pool. Defensible (a core journal fact-shape change
  needs a design rung and a ruling) but this window did **not** implement the
  highest-value item (effect disposition) — it only specified it.
- **Research note discoverability.** The note lives in `docs/research/` (matching
  the paperclip precedent) but research docs have **no index** (unlike
  `docs/status/`). Nothing mechanically points a future session at it except the
  TODO rows and the SECURITY.md citation.
- **AGENTS.md relation bullet.** Deliberately NOT added: AGENTS.md is 9 bytes
  under its 18500 B guard, and the paperclip precedent set no AGENTS.md bullet
  either. Left as-is on purpose, not overlooked.
- **The note's own edits were split across the daemon sweep.** The first write
  was swept into `fdbba7fe`; the lesson-table refinements (rows 5/6 → SHIPPED,
  citation fixes) are still uncommitted in the working tree.

## c) NOT STARTED

- Any implementation of the five TODO rows (by design — this is a research
  window; the pool harvests them).
- A regression guard that mechanically fails if a learned/merged verdict can
  lower a deterministic finding (the SECURITY.md pin is prose only).
- No verification of the *other* turnstone claims (Provider/ModelRegistry lanes,
  truncation policy, MCP circuit breakers, skill scanner) beyond reading — they
  were classed out-of-scope, not tested.
- No cross-check of the turnstone claims against a second source / a pinned
  turnstone tag (read at `main`, which may drift under the note).

## d) TOTALLY FUCKED UP

- **A false precision in a citation.** I wrote `claims.go:158-175` before
  `grep -n` confirmed the reclaim `Released` append is at `:165` (the range
  straddled the cancel-path append at `:136`). Caught and fixed this window, but
  it is the exact "line-pinned cite that survives only while the surrounding
  text is stable" hazard the repo's own drift-smoke rows warn about.
- **A guard trip on first try.** The first effect-disposition TODO row said "the
  owner ruling the paperclip wake note names" and `check-todo-list.sh` correctly
  failed it as an unblocked owner-gated item. Rephrased. (Not destructive, but it
  cost a round-trip — the guard's marker list should have been in my head before
  writing.)
- **Editing without the View tool.** The first `edit` to TODO_LIST.md was
  rejected ("must read the file before editing") because I had read it via
  `bash cat/head/tail`, not `view`. Re-read with `view`, then edited. Wasted a
  call.
- **No commit.** Per the standing instruction I did not commit; but the repo
  convention is edit→commit→battery, and the note's post-sweep edits are now
  sitting uncommitted where the daemon may or may not fold them cleanly.

## e) WHAT WE SHOULD IMPROVE

1. **Read the guard's marker list before writing TODO rows** — one `sed` of
   `check-todo-list.sh` up front would have avoided the owner-mark trip.
2. **Grep line numbers before citing them.** Cite by `grep -n` output, never by
   eyeballed ranges; the note's verification trail should have been produced
   *before* the first write, not after.
3. **Decide "research vs implement" explicitly at the top of such a window** and
   state it in the note (I chose research; it should be a stated decision, not an
   implicit one).
4. **Give research notes a light index** (a `docs/research/README.md` row) so
   they are discoverable like status reports — currently only grep finds them.
5. **Turn the SECURITY.md prose pin into a test** so the narrowing rule is
   machine-checked, not vibes.
6. **Pin the turnstone source to a tag** in the note (I cited `main`, which will
   drift) — a `turnstone@vX.Y.Z` or a date-stamped commit SHA would make the
   citations durable.
7. **Reconcile with the paperclip M24 row:** turnstone independently reaches the
   minted-per-run-capability conclusion — worth an explicit `corroborated by`
   note on that TODO row so the two aren't re-litigated separately.

## f) UP TO 50 NEXT THINGS

1. Implement effect-disposition on reclaim (TODO row 1) — design rung first:
   what the released/cancelled fact detail should carry (`unknown`/`none`), and
   whether it is a fact-type change or an evidence key.
2. Surface effect disposition in `tq facts` + the webui task timeline + the
   `--dlq` autopsy (so an operator can tell a double-run from a clean one).
3. Emit `unknown` when ADR-0005's SIGKILL-to-group kills an in-flight effect.
4. Add a regression test pinning the SECURITY.md narrowing rule (fail if a
   learned/merged path lowers `SecretHits`, a verify-gate rc, or auto-dismiss).
5. Build the verdict→outcome calibration dataset (TODO row 2).
6. Add a `docs/research/README.md` index row for this + the paperclip notes.
7. Re-anchor the turnstone citations to a tag/SHA (durability).
8. Add an AGENTS.md known-issue line for the reclaim effect gap **only if** bytes
   are freed (budget is 9 B) — else it stays in the note + TODO.
9. Model-key the `priority_scores` cache (TODO row 5).
10. Child-task authority/budget ceiling (TODO row 4).
11. Audit whether `--prioritize` scores ever compare across model versions today.
12. Add a `corroborated-by-turnstone` note to the paperclip M24 secret-injection
    row.
13. Draft the `EffectStatus` Go enum + fact-detail schema in a design memo.
14. Decide whether a reclaimed task should re-run at all when its last effect is
    `unknown` (or park for operator triage like `tq ask`).
15. Add a doc-pin (like `check-release-docs`) for the SECURITY.md narrowing
    wording so it can't silently rot.
16. Write a falsifier test named for turnstone's state-ablation probe (the
    crash-resume test already IS one — name it).
17. Add a "reads are not free" note to SECURITY.md for bridge fetches
    (PapDashboard/CQA) as injection/exfil vectors.
18. Verify the batch-gate overdraw claim: confirm budget facts are serialized by
    the single writer across pools (no per-pool read-then-write).
19. Consider a scheduled self-audit of DLQ/incident/memory (turnstone's daemon
    "scheduled resets") — ROADMAP scale.
20. Add a turnstone row to FEATURES.md "Relation to other projects" if that
    section exists (it does not — maybe ROADMAP instead).
21. Cross-check the note's judge.md claims against turnstone's `core/judge.py`.
22. File the two corroborations (M24, batch-overdraw) so they stop being
    re-derived.
23. Add a research-note template to `docs/references/` so the next deep-dive is
    uniform.
24. Re-run `check-doc-refs` after any note reflow (done this window).
25. Pin the note's six citation URLs as a test that they resolve (cheap, catches
    upstream deletion).

## g) QUESTIONS (only the owner can answer)

1. **Should this window have implemented effect-disposition rather than only
   filing it?** The reclaim fact-shape change touches the core journal and looks
   like one of the "BLOCKED: owner architecture decision" classes, but you asked
   me to "execute" — I chose to specify + file. Was that the right call, or did
   you want the design memo (and the enum) written now?
2. **Is `EffectStatus` on the released/cancelled fact the shape you want, or a
   separate `task.effect-unresolved` fact-type?** This is the same
   fact-type-vs-evidence-key fork the paperclip wake note left open (§g-2 there);
   I did not want to pre-empt it.
3. **Do you want research notes indexed/first-classed** (a `docs/research/README.md`
   + maybe an AGENTS.md pointer), or is grep-discoverable + TODO-cited enough?

## h) BAND DRIFT

None recorded — priority bands untouched; `$TQ_DB` unset; no `task.reprioritized`
facts minted this window.
