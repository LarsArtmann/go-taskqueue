# Owner rulings package — O1–O7 (round-13 M21, O7 appended 05-5x)

- **When**: 2026-09-12 02:48 CEST
- **Purpose**: one sitting, seven rulings. Each question lists the options,
  the cost/benefit, and an ANSWER line to fill. Nothing here blocks a
  "no ruling yet" — but every answered question un-blocks a queued task
  (T9/T10/T14/T6/T25/T18/T26/T27).
- **Prepared by agents, pulled by owner only** (round-13 non-negotiable #4).

---

## O1 — SystemNix `GOEXPERIMENT` env line (un-blocks: the 51% fix; T19)

One line on the tq-agent-pool unit ends the env lie that burned 5+ windows.
Exact diff + deploy + post-flip verification:
`docs/planning/2026-09-11_14-45_FLIP-CHECKLIST-AND-DLQ-TRIAGE-RUNBOOK.md` §6
(bundled with the next input flip — zero extra deploys). Minted verifies are
now env-self-contained and `tq doctor` detects the lie either way; this line
is still the true fix.

**ANSWER**: [ ] apply §6 at next flip [ ] defer (accept known-lying gates; doctor is the reference)

## O2 — TQ_RESULT schema ruling (un-blocks: T9, T11)

Invented TQ_RESULT fields re-offended across 3 windows. Proposed contract:

1. **Fields allowlist** — `verdict` ("complete" | "request_changes"), `summary`,
   `commits` (array of full SHAs), `report` (repo-relative path). Anything
   else ⇒ executor gate FAILS the task (parseable but lying).
2. **SHA semantics** — `commits` must reference commits CREATED BY THE WORK
   TURN in THIS repo (record commits are cited in the report file, not in
   TQ_RESULT; one identity per artifact).
3. **Verification-only attempts** — a re-dispatch that only re-runs the
   verify gate (work already done) may re-emit the ORIGINAL TQ_RESULT
   verbatim; it must not invent new fields to explain itself.

**ANSWER**: [ ] as proposed [ ] amended: ______________ [ ] keep loose (gate stays parse-only)

## O3 — Report placement ruling (un-blocks: T10, T12)

01-06 (file-per-window) CONTRADICTS 00-54 §e3 (append). Every dispatch
currently guesses.

- **a) file-per-window** (status quo of most reports): one
  `docs/status/<ts>_<name>.md` per window; history = many files; index
  carries the ordering. Cost: file count growth; benefit: no concurrent
  append races (two agents appending to one file corrupt each other).
- **b) append**: one rolling file per project/topic. Cost: write races under
  the concurrent-agent reality; benefit: fewer files.

Recommendation: **a)** — the concurrent-agent reality makes appends a
lost-update generator (the 01-06 correction exists because of it).

**ANSWER**: [ ] a) file-per-window [ ] b) append [ ] other: ______________

## O4 — Webui per-project rulings (un-blocks: T14)

1. **Unknown `/project/{name}`**: [ ] 404 page [ ] empty dashboard with
   filter chip (current behavior). Recommendation: 404 — a typo'd URL should
   say so, not show a plausible-looking empty board.
2. **Budget scope**: [ ] keep budget GLOBAL with an explicit "global" marker
   when a project filter is active [ ] add per-project budget projection.
   Recommendation: global + marker now (the journal has no per-project cap
   concept yet); per-project budgets as a separate feature with real
   config, not a side effect of the filter.

**ANSWER**: 1: ______ 2: ______

## O5 — gosec gate-vs-advisory + CI-time budget (un-blocks: T6 final flip, T25)

1. gosec: with the FP triage encoded as config (T6, in flight), the job can
   go hard-gate or stay advisory-with-config. [ ] hard [ ] advisory.
   Recommendation: advisory until one fully-green runner week, then hard.
2. CI-time budget: full matrix ≈ N minutes per push × pushes/hour under the
   daemon. What is the acceptable ceiling? ______ (drives T25's retry /
   concurrency-group / scope decisions).

**ANSWER**: 1: ______ 2: ______

## O6 — Policies + credentials (un-blocks: T18, T26, T27)

1. **Backlog append-cap policy** (L136): cap status-appends per task?
   [ ] yes: ____ [ ] no, dedup keys suffice.
2. **Module-fetch trust** (L139): require `go mod verify` + checksum pin on
   release builds? [ ] yes [ ] advisory.
3. **CQA creds** (L99): provide test credentials for the live-verify
   checklist? [ ] attached [ ] skip live-verify.
4. **AllStatuses release call** (L124): export in next re-tag? [ ] yes
   [ ] hold.

## O7 — Daemon-folded commit attribution (un-blocks: L160; T12 residue)

When the auto-commit daemon folds a task's working-tree changes into a
footer-less `chore:` commit (lint triage: content in bebc35a/fc495e8,
footer-only eaf73a9), ticket↔content attribution has no footer to ride.

- **a) Report-side attribution (recommended)**: `chore:` commits are
  attribution deserts BY DESIGN; the task's own `TQ_RESULT` `commits`
  array + the close-out report's citations are the canonical map, and
  `tq show <id> --commits` is the forensics view. Cost: forensics needs
  the report; benefit: zero daemon changes, works for already-folded
  history.
- **b) Daemon carries footers**: the daemon embeds a
  `Task-Queue-ID` when the folded tree's TQ_RESULT names exactly one task.
  Cost: daemon change outside this repo; race when several tasks' changes
  interleave in one fold (the common case makes the footer a LIE).
- **c) File-level ledger**: a committed map (commit → task ids) appended
  by agents at close-out. Cost: a second source of truth that rots;
  benefit: greppable without the journal.

**ANSWER**: [ ] a) report-side [ ] b) daemon footers [ ] c) ledger [ ] other: ______________

---

_Answers recorded here close the loop: edit this file's ANSWER lines (or
reply in chat); the queued tasks read the ruling and land same-window._
