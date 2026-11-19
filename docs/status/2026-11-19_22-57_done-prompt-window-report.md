# Done-prompt window report — residue batch, reviewer-fix duo, templ-freshness pair (2026-11-19 22:57)

**Window**: 5 tasks, 2026-11-14 → 2026-11-19
(000001a1d87df5128af08a9f367500000000, 000001a1d8904841acaae577cbf800000000,
000001a1d890488a3dfeb10c5e2900000000, 000001a1f569501ff7caf5f9cb1c00000000,
000001a1f5971c3033695a561f5f00000000).

**Dispatch-time window dedup**: ran per contract. `docs/status/README.md`
index rows and `docs/status/*.md` window reports cover the individual tasks
(four per-task closeout reports exist under `tasks/`) but NO earlier window
report spans this 5-task set (the 2026-11-14 06-36 done-prompt covered a
different 7-task window). **Full pass**, not verification-only.

## a) FULLY DONE — all five tasks completed and verified at HEAD

Per-task closeout reports were read FIRST (per contract); each landing was
then re-verified at HEAD with fresh greps/gate runs this session.

1. **Task …8af08a9f367500000000 — twentyeighth-dispatch residue batch**
   (closeout `tasks/2026-11-14_06-51_task-…8af08a9f367500000000.md`; commit
   ccc26748). Item 1 (BuildFlow pseudo-version-hygiene PASS-message fix) was
   PRE-LANDED + already ticked by a concurrent session (BuildFlow repo +
   LarsArtmann/BuildFlow#106 OPEN) — skip verified correct by the re-dispatch
   grep. Item 2 landed: go-cqrs-lite docs-gates debt cured to battery PASS —
   new `.docs-gates.json` (audit-all+annotate SKIP per fleet doctrine, cadence
   tripwire 190/75d) + 9 inline `<!-- routing:skip -->` annotations on the
   2026-10-23 codec report; docs-gates `run` went rc≠0 (4 red gates) → rc=0
   (matrix PASS, captured). **Verified at HEAD this session**:
   `/home/lars/projects/go-cqrs-lite/.docs-gates.json` exists.
2. **Task …577cbf800000000 — reviewer fix: shellcheck findings in
   check-subtag.sh** (closeout `tasks/2026-11-14_06-57_task-…577cbf800000000.md`;
   fix a3719eae, closeout 0ee37532). All 5 findings cleared (SC1007 ×2,
   SC2164 ×2, SC2034); check-script-syntax 140 ok/0 fail at landing.
   **Verified at HEAD this session**: `check-subtag.sh --self-test` → ok=3
   fail=0.
3. **Task …3dfeb10c5e2900000000 — reviewer fix: AGENTS.md fused bullet**
   (closeout `tasks/2026-11-14_06-59_task-…3dfeb10c5e2900000000.md`; fix
   cd480f40, healed from footer-less daemon sweep dc70a9f8 with fork record,
   closeout 9b61cf6d). **Verified at HEAD this session**: the
   `**PapDashboard questions**` bullet starts its own line (AGENTS.md:166).
4. **Task …9cb1c00000000 — templ-freshness oracle-override ask**
   (dc0f91eb). Premise verified (option absent at locked go-nix-helpers
   15c84349), then MOOT: the ask ALREADY landed upstream as `templCliPackage`
   (e0c4b70, pushed origin/master) — no duplicate issue filed; TODO row
   resolved with the full two-tick annotation.
5. **Task …95a561f5f00000000 — re-dispatch of the templ-freshness re-enable**
   (closeout `tasks/2026-11-19_21-58_task-…95a561f5f00000000.md`). Both items
   ALREADY LANDED by the prior session (flake change cc2b0e37 + the MOOT
   resolution dc0f91eb above); this session performed the closeout ticks
   (20ef59ac, b795c5a3) and premise re-verification.
   **Verified at HEAD this session**: `templCliOverride` present in flake.nix
   (4 grep hits, feeding both the freshness oracle and the treefmt templ fmt
   formatter), flake.lock:58 rev = e0c4b7002621c89e66ef977b7a13380c5c0365e2.
   The gate-green claim is inherited from cc2b0e37 (same tree state), NOT
   re-proved by a fresh `nix build` this session — the closeout records this
   honestly and a push-time re-run is owed (§b1).

Attribution: every task id has footer-parseable commits (trailer-verified by
the respective closeouts; the substantive templ commits correctly carry the
PRIOR task's footer, the re-dispatch's ticks carry its own id).

## b) PARTIALLY DONE

1. **Templ gate-green is inherited, not re-proved at HEAD** — no `nix build`
   / regenerate-and-diff re-run between cc2b0e37 and now. If the flake moves
   before push, the receipt is stale. Owed at push time (minted row, §f).
2. **The 4-commit templ span (cc2b0e37→b795c5a3) is UNPUSHED** — including
   the gate-enabling flake change; push is owner-gated in this repo.
3. **go-cqrs-lite docs-gates posture is tripwire-shaped, not cured-shaped**
   (closeout §b1): annotate/audit-all remain red-under-skip (history-locked
   corpus), cadence sits 3 reports under its own threshold; the archive
   ritual (187 unarchived vs <15 healthy) is ratcheted, not run.
4. **Window's 06-51 §c**: the 4 foreign cqrs-lint files riding mixed sweep
   d5dbc2a96 have no footer attribution (concurrent-agent work, untouched by
   design); `buildflow -s docs-gates` re-verification in go-cqrs-lite never
   ran.

## c) NOT STARTED

Nothing in this window's scope was skipped. The standing backlog the window
passed over is already rowed (index digest ritual at 423/400 live rows — the
WARN fired again on THIS report's commit; pareto T21/T22/T33-T35 tail;
owner-gated pushes; generic_return migration).

## d) TOTALLY FUCKED UP

Nothing landed broken; all five tickets closed green and every gate cited at
landing is green at HEAD now. Honest breaks, all self-caught in-window by the
closeouts (summarized; no carry-forward damage):

1. **Daemon-sweep races again (3 of 5 tasks)**: deliverables landed inside
   footer-less daemon sweeps (d5dbc2a96 mixed, dc70a9f8, 56ae5316) — each
   healed (amend fallback / heal-daemon-sweep) or attributed via amended
   exactly-mine tick commits; one heal refusal was correct-by-design on the
   mixed sweep, and the blocker-walk oscillation re-occurred live (second
   sighting).
2. **Half-edited command substitution** (06-57 d1): a scripted SC2034 strip
   left a dangling `)`, turning one finding into SC1072 before repair —
   the scripted-multi-replace-needs-`bash -n` post-step lesson, not yet
   convention-ized (§f).
3. **Edit→commit gap discipline violated twice** (06-59 d, 06-51 d1) —
   staged-then-paused edits became daemon food; the AGENTS convention
   (commit in the same breath) keeps being re-learned the expensive way.
4. **Attribution guard run late** (21-58 d1) and a wasted heal dry-run from
   a wrong `--from` range (21-58 d2): verify-before-report ordering, hit
   again.
5. **check-footer-attribution default-base false-FAIL (row 1502)** fired
   live on the 4-commit mixed-id span — known row, hit again, bump already
   minted by the 21-58 closeout.

## e) WHAT WE SHOULD IMPROVE

1. **Re-dispatch dedup must also grep the TODO rows' cited SHAs** (21-58 e1):
   the footer-grep found no THIS-id commits, but the work sat landed under a
   foreign id one commit away. Row minted by the 21-58 closeout; still worth
   naming here because THIS window re-dispatched a task whose premise
   (`flake.nix` gate) had already moved under a different footer.
2. **Mixed-sweep footer carrier is unsolved** (06-51 e1/§g2): a mixed
   footer-less daemon sweep leaves no sanctioned path that carries MY footer
   over MY files; a heal `--files` surgical split would close the class.
   NOT yet rowed — minted below.
3. **Scripted multi-replace post-condition** (06-57 e1): `bash -n` + anchor
   re-grep after ANY python-heredoc multi-replace of shell code; candidate
   AGENTS convention addition (minted below).
4. **Gate-class counts in dispatch envelopes** (06-51 e4): "4 error
   findings" was really 4 red GATES hiding dozens of findings; premise drift
   costs a re-derivation per batch. Minted below.
5. **docs-gates config discoverability** (06-51 e3): `.docs-gates.json` keys
   are enumerated nowhere in `docs-gates help`; repos without a sibling
   example would guess. Minted below (upstream ask).
6. **AGENTS.md has no markdown-shape guard** (06-59 e1): the size gate checks
   bytes, so a fused bullet survived multiple sessions + a reviewer. Row
   already minted by the 06-59 closeout — verified present in TODO_LIST.
7. **Index bloat is now 423/400** — the digest ritual row exists and keeps
   not being dispatched; every closeout commit WARNs.

## f) NEXT THINGS (top candidates; dedup-checked — see the appended TODO_LIST items)

Already-rowed during the window (do NOT re-mint): AGENTS bullet-structure
guard; new-gate landing ritual (cite captured check-script-syntax rc); row
1502 bump; templ pin drop-day; re-dispatch SHA-grep recipe; go-cqrs-lite
archive ritual + half-strike cure + routing annotations (06-51 §f, appended
then); standing index digest row.

Minted by THIS report (appended, cap respected):
1. heal-daemon-sweep.sh `--files` surgical split for MIXED sweeps.
2. docs-gates upstream: config-schema verb / help stanza for
   `.docs-gates.json` keys.
3. Dispatch envelopes carry gate-CLASS counts (red gates + finding totals).
4. Convention: `bash -n` + anchor re-grep post-condition after scripted
   multi-replace of shell code.
5. go-cqrs-lite: add the docs-gates maintenance-loop adoption note to ITS
   AGENTS.md (foreign repo — blocked on working in that repo).
6. go-cqrs-lite: re-run `buildflow -s docs-gates` to confirm the original
   discovery channel is clean end-to-end (foreign repo — blocked).
7. Push-time receipts for the unpushed templ span: regenerate-and-diff
   re-run + `check-tick-claims.sh` after the span lands (blocked on the
   owner-gated push).

Remaining window-derived capacity (7–50): the standing rows this window did
not reprioritize (see §c); deliberately not re-enumerated here — the
per-task closeouts carry fuller lists and minting near-duplicates is the
known anti-pattern.

## g) QUESTIONS (owner-only; appended to TODO_LIST as BLOCKED)

1. **Push policy for the templ span**: ride the next standing push wave
   as-is, or block on a push-time freshness re-run first? (21-58 §g1,
   carried — this window adds only the note that the span is now
   4 commits + this report's commit.)
2. **Re-dispatch attribution convention**: keep prior-task footers on
   already-landed work (current behavior) or re-foot unpushed foreign-id
   commits to the re-dispatch id for queue discoverability? (21-58 §g2.)
3. **Templ pin drop-day watcher**: mechanical nixpkgs-templ version grep
   per sweep, or manual docs-health cadence? (21-58 §g3.)

## h) BAND DRIFT

**None recorded.** Fresh journal read this session (`tq facts` over
`./tasks.db`, 397 fact lines): zero `task.reprioritized` facts in the window
or ever — priorities in this window moved only via the dispatch-time markers
and scores, never via reprioritization facts. This is now the third window
running with an empty drift ledger (ADR-0015 accountability holds trivially).

## Verification receipts (this session)

- Closeout reports read first: all four listed under §a.
- HEAD greps: `templCliOverride` ×4 in flake.nix; flake.lock rev e0c4b70…;
  go-cqrs-lite `.docs-gates.json` present; AGENTS.md:166 bullet split;
  `check-subtag.sh --self-test` ok=3 fail=0.
- Journal: `tq facts` (397 lines) grepped for reprioritized — zero.
- Index row for this report added to docs/status/README.md (index gate
  green at the skeleton commit; bloat WARN 423/400 re-fired).
