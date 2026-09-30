# Archive window 2026-09-30: 2026-09-12_00-43 report → docs/status/archived/ (TODO 152)

- **When**: 2026-09-30, ~03:15–04:35 CEST
- **Branch**: master, unpushed (agents never push)
- **Work commit**: `9d4eb43c` (footer `Task-Queue-ID: 000001a0ef7bbc2124c2ce69fe7b00000000`)
- **Scope honored**: exactly TODO row 152 — per-item triage verification, git-mv,
  index repoint + counter. No code touched, no drive-by row closures.

## a) FULLY DONE

1. **Archive executed end-to-end.** `git mv
   docs/status/2026-09-12_00-43_task-000001a092a10a7edf3a3836f61deba98eed.md`
   → `docs/status/archived/` (100% rename); citations repointed in
   TODO_LIST.md:148 (§f citation) and the docs/status/README.md index row
   (was :372); archive counter bumped 346→347; TODO row 152 closed `[x]`
   with one DONE note (row-note accretion convention respected). Commit
   `9d4eb43c`, footer verified via `git interpret-trailers --parse` (count=1,
   well-formed, last trailer), tree clean after.
2. **Triage claims spot-verified at HEAD** (the three artifact-class
   assertions in the dispatch + the file's own strikethroughs):
   `scripts/archive-evidence.sh` exists (§f1/§g1), 
   `docs/planning/2026-09-12_worktree-per-agent-design.md` exists (§c3/§f6),
   the README "Evidence archives" block exists with the 01-15 index row
   confirming 676d8337 (§g3/§f2-supersession). The ARCHIVED disposition note
   and per-item inline strikethroughs were already authored in the file by
   the sweep-prep window — verified present, not re-authored.
3. **Citation census complete**: `rg -l` over the whole repo (hidden
   included, .git excluded) found exactly two files citing the moved path —
   TODO_LIST.md and docs/status/README.md. Both repointed; `check-doc-refs.sh`
   green after the move proves no straggler.
4. **Battery rc-captured, all green**: check-doc-refs=0, check-status-index=0,
   check-todo-list=0, check-ghost-archives=0, root `go build`=0, root
   `go vet`=0 (GOEXPERIMENT=jsonv2 + GOTOOLCHAIN=auto exported). Direct
   `rc=$?` capture to variables, counts/outputs not trusted to pipelines.
5. **Session-start ritual ran** (session-start.sh with the task ID; SUMMARY
   line `windows=1 done-row-hits=0` — no prior windows on this ID); tree
   clean at start; CONTRIBUTING.md read at turn 1 (CLAUDE.md absent). One
   documented ritual failure: see §d2.
6. **Daemon race healed per playbook.** The auto-commit daemon swept my
   staged 3-file diff footer-less as `d828104e` while the gate battery ran
   (receipt #7 for the class, TODO row 154). Healed via the documented
   unpushed soft-reset playbook: `git reset --soft HEAD~1` → recommit with
   the footer; verified content-identical (3 files, 4+/4−, rename 100%) and
   the swept commit was unpushed. See §d1 for why the race happened at all.

## b) PARTIALLY DONE

Nothing on the task itself. Two scope declarations (not partials):

- **Gate scope**: cheap battery only (four doc gates + root build/vet +
  close-out check-ci). NOT ci-local, NOT the race suite, NOT nix — docs-only
  change, no Go file touched. Same conscious deviation the archived 00-43
  report itself declared in its §b; the standing battery bar for docs-only
  windows is still unrated (row 355 owns the check-ci sub-question, BLOCKED).
- **Triage verification depth**: spot-verification of the artifact-class
  claims (3 checks), NOT a fresh re-derivation of every struck item — the
  strikethrough annotations were inherited from the sweep-prep window. The
  conventions say "per-item triage at CURRENT HEAD" without saying whether
  the closer may inherit the striker's derivation; my bar and the open
  question are §g1.

## c) NOT STARTED (noticed, deliberately not touched)

1. **Master CI is RED at `d0f68099`** (latest pushed commit, the daemon
   commit immediately before this window's work; check-ci rc=1 at 04:30,
   "failure, predates your tree"). Not investigated — out of scope, and row
   346 already names the probable chronic cause (the two M4-gated
   journal-drift cmd/tq tests; row 114 records the owner's "no test
   relaxation" ruling, making a structurally red job the documented steady
   state until row 346 lands). Carried here per the ritual contract.
2. **Row 153 looks closable** — it asks for the AGENTS.md mvdan
   `&`-precedence bullet, and AGENTS.md:1190 already carries exactly that
   bullet. Verify-and-close is a fresh window's work; a drive-by closure on
   someone else's row was refused.
3. **No backlog rows appended during the work phase** — nothing new was
   discovered mid-task. One row minted at close-out (§f1, the ritual SUMMARY
   CI-state gap, discovered in §d2).

## d) TOTALLY FUCKED UP

1. **LEAD: walked into the documented daemon race with the prevention
   in-context.** My edits sat uncommitted across the six-gate battery — the
   exact anti-pattern of the existing AGENTS.md "Edit→commit→battery
   ordering" convention ("commit doc edits immediately after the edit; run
   long gates AFTER the commit"), and receipt #6 (676d8337, TODO row 154)
   was the SAME shape earlier the same day. Cost: one failed commit attempt
   ("nothing to commit" — the daemon had swept the tree clean under me) plus
   a soft-reset history heal on an unpushed commit. The heal was clean and
   playbook-sanctioned, but the entire incident was one early `git commit`
   away from not existing. A convention I had loaded and still didn't follow
   is worse than an unknown trap.
2. **Ritual-output truncation re-hit** (documented recurring miss — the
   10-39 window confessed it three windows running): I piped
   `session-start.sh | tail -30`, which kept the SUMMARY line (row 299's
   mitigation worked as designed) but cut the master-CI probe and git-state
   block ABOVE it — so this window ran to close-out without knowing master
   was RED, and only recovered the fact by manually re-running
   `./scripts/check-ci.sh` during report prep (rc=1). The truncation didn't
   hide a duplicate; it hid the probe. That residual is now minted as §f1.
3. **Documented mvdan multi-file `tail` hazard re-hit verbatim**: `tail -3
   /tmp/dr.log /tmp/si.log …` → "option used in invalid context" — the
   exact failure AGENTS.md documents (2026-09-22 03-05 §f). Harmless (all
   four rcs were already captured directly) but it is a known-trap walk, and
   the file-view tool was the documented alternative sitting right there.
4. Minor: first multiedit rejected ("read the file before editing") — grep
   output does not satisfy View-before-Edit bookkeeping. One wasted round
   trip.
5. Minor: `mcp_qmd_get` on a repo-root doc returned "Document not found"
   (documented limitation — qmd doesn't serve repo files); fell back to
   View. Near-zero cost.
6. Minor: the first commit attempt chained add→commit→verify in one
   command and failed confusingly; a `git status` between gates and commit
   (the session-start reminder literally says to re-run it after gates)
   would have surfaced the daemon sweep before the failed attempt.

## e) WHAT WE SHOULD IMPROVE

1. **Make the ordering convention mechanical, not memoized.** Docs edits →
   commit (footer on) → gates. Row 154's wrapper-or-rule decision is
   receipt #7 old; this window endorses the cheap variant (an AGENTS.md rule
   hardening the existing ordering clause into a checklist item: "no gate
   starts while a related doc edit is unstaged").
2. **Survival-complete the SUMMARY line**: carry master-CI state in the
   final completion line (`SUMMARY: windows=N done-row-hits=N
   master-CI=red@<sha>`) — row 299 solved exactly this problem for the
   duplicate count; the probe result is the other truncation-fragile fact.
   Minted as a TODO row this close-out.
3. **`git status` as a named step between battery and commit** — it is the
   cheapest possible early-sweep detector and currently lives only in a
   session-start reminder, not in any checklist.
4. **Stop piping session-start.sh through tail/head** — the script is long;
   either cap its verbose sections or write full output to a temp file the
   window can View. Two windows in two days have re-hit the truncation class
   with the SUMMARY mitigation already in place.
5. **Codify the archive-closer's re-verification bar** (minimum evidence
   set when inheriting pre-striked reports) — as input to row 280's
   check-archive-eligibility.sh rather than a new gate; my window's
   artifact-class spot-check is a candidate floor (§g1 asks the owner to set it).

## f) Up to 50 things we should get done next

Not 50 — honest list, no padding. Existing rows are re-ranked, not re-duplicated.

1. **(NEW, minted this close-out)** session-start.sh: carry master-CI state
   in the FINAL SUMMARY line so `| tail` invocations cannot lose the probe
   result (§d2; row 299 is the precedent for the windows/done-row-hits half).
2. Row 154 daemon-race prevention — this window is receipt #7; the highest-
   leverage process row open (§d1/§e1).
3. Row 280 check-archive-eligibility.sh — this window is another
   manual-judgment archive; the mechanical bar would also settle §g1.
4. Row 153 (mvdan &-precedence bullet) appears CLOSED by AGENTS.md:1190 —
   verify and close with a DONE note (§c2).
5. Row 346 (S1 journal-drift replay coverage) — it owns the probable chronic
   master-CI red; until it lands every window carries a red master (§c1).
6. Row 355 (check-ci battery-bar ruling) — stands, BLOCKED on owner.
7. Row 194 (daemon-race footer policy, BLOCKED on owner) — this window adds
   a clean datapoint: the soft-reset playbook handled a 100%-rename + 2-file
   docs diff without loss.
8. Row 474 (daemon footer-attach heal script) — my manual soft-reset is
   exactly the case it would automate.
9. Rows 359–362 (commit-msg hook follow-ups) — stand; this window ran the
   manual interpret-trailers self-check that row 362 wants codified.
10. Rows 155/369 (session-start regression pin + hooks-liveness probe) — stand.
11–50: refused. The section-level backlog (process rows 246–260, footer
placement 390, re-dispatch protocol 310) already ranks the remainder; minting
filler here would be noise.

## g) Questions I can NOT figure out myself

1. **Inherited-triage bar for archive closers**: when a report arrives
   pre-striked (strikethroughs + disposition note authored by a sweep-prep
   window), must the closer re-derive EVERY struck item at HEAD, or is
   artifact-class spot-verification of the resolvable claims (my 3 checks)
   the accepted bar? The README conventions say "per-item triage at CURRENT
   HEAD" but not who owes the derivation or how fresh inherited strikes may
   be. Answer sets the floor for row 280's mechanical gate.
2. **Where does the unpushed-pile push-cadence ruling live?** The archived
   00-43 report's §g2 strikethrough routes it "resolved — via §g2 ruling"
   and its §c5/§e5 point back at the same ruling, but I found NO durable
   home: no TODO row names a push-cadence ruling (the cadence rows are
   index-bloat 165, deploy 196, session 273), and AGENTS.md carries no
   cadence line. If the ruling exists only in a chat/nowhere, the strike is
   circular and the item needs a home (row or AGENTS.md line) — or the
   strikethroughs need correcting.
3. **Chronic red-master policy**: if the d0f68099 failure is the row-346
   journal-drift class (per rows 114/346 it fails at HEAD by owner ruling
   until S1 replay coverage lands), master CI is structurally red for every
   window until then — every ritual probe reports red, and "predates your
   tree" is permanently true. Do you want an expected-failure/tolerated-job
   treatment (or a check-ci annotation naming the known cause) so a red
   master stays a SIGNAL instead of becoming the background hum it already
   is? The factual half (which job failed) a `gh` log read would settle;
   the policy half is yours.
