# Status Report — art-dupl 70-Group Alarm: ADR-0019 S1 Spike Triage (Diagnostic Window)

Date: 2026-09-23 15:59 CEST · Scope: THIS session's run only — a single
owner-prompted diagnostic window ("What the fuck happened!??!?" over an
`art-dupl --sort total-tokens -t 5 --type-aware` paste showing 70 clone
groups), plus this mandated a-g report. NO source files were edited by this
window. Zero commits by this window (daemon folded nothing of mine — nothing
of mine existed to fold). Format override note: the status-report skill's
canonical output is HTML; the owner's instruction pinned `.md`, so `.md` wins
for this report.

Session span: 2026-09-23 ~15:45 → 15:59 CEST. Concurrent context: the tree
carries UNCOMMITTED work in `internal/queue/sqlitev4/` (adapter.go, go.mod,
go.sum) from a session that is not this one — read, judged, built on, never
reverted (the AGENTS.md rule); everything below treats that work as live
forward progress.

## Headline (the answer to the alarm)

Nothing broke. The 70 clone groups are the **ADR-0019 S1 migration spikes
landing on top of the freshly deduplicated tree**. Timeline, all from git
log over the whole repo + the dated reports:

| When (2026-09-23) | Event | Evidence |
| --- | --- | --- |
| 01:34 | Dedup pass #1: 22→18 actionable groups at `-t 4`, 3 extractions | `docs/status/2026-09-23_01-34_art-dupl-dedup-pass.md` |
| 02:35 | Dedup pass #2 (continuation): `decodePayload`/`prepareRepo`/`payloadTimeout` extracted; end state **9 shown / 65 total** at `-t 5` | `docs/status/2026-09-23_02-35_dedup-continuation-window.md` §a5 |
| 03:50 (`db0fa088` + f37ec7a6/5080ec56/b81496ad) | `internal/queue/cqrsqlite` lands (daemon commits): adapter 455 + extras 744 + store_test **3,440** lines — the migration plan's C03 scaffold | `git log -- internal/queue/cqrsqlite`; plan M011 |
| 04:26 (`889c1888` + d8c94e26) | `internal/queue/sqlitev4` lands (daemon commits): adapter **1,425** lines — a SECOND S1 spike module over the same upstream `queue/sqlite/v4.0.0` | `git log -- internal/queue/sqlitev4` |
| now | sqlitev4 still being edited (uncommitted) | `git status` |

The user's paste: **70 total, 18 shown** (24 non-actionable, 28 filtered
suppressed). Recount against the paste, group by group: **13 of the 18 shown
groups involve the two spike modules** (5 sqlite.go↔sqlitev4/adapter.go, 4
cqrsqlite/store_test.go↔sqlite/store_test.go, 3 cqrsqlite/extras.go↔sqlite.go,
1 cqrsqlite/extras.go↔sqlitev4/adapter.go); the remaining **5 are the
long-adjudicated accepted twins** (postgres mirror pairs, dlqfix/review
sweeper shells, httpapi/webui stats twins). The duplication class is
explicitly sanctioned: ADR-0019 S4 deletes the mirrored backends at cutover,
the same acceptance the 01-34/02-35 windows applied to the sqlite↔postgres
mirror mass ("side-by-side diff IS the contract").

Correction against my first chat answer: I said "14 of the 18"; the recount
is **13**. Owned in §d3.

## a) FULLY DONE (each with its evidence)

1. **The alarm root-caused to a timeline, not a regression.** Read `git log
   --oneline -25` over the WHOLE repo (not `-- internal`), `git status
   --short`, per-path `git log` for both new modules, and `ls
   internal/queue/`; reconciled the 02:35 end state (9 shown / 65 total)
   against the 70/18 paste. The delta is exactly the two spike modules. No
   accepted group from the 01:34/02:35 adjudications reappeared as harmful.
2. **The uncommitted sqlitev4 delta read in full and judged.** `git diff
   internal/queue/sqlitev4/`: (a) dead parity helpers removed
   (`cancelReasonDetail` + its `var _ =` keep-alive block + the `strconv`
   import and the anonymous-struct stub in `tokenFor`) — cleanup of
   scaffold-era dead code; (b) `queue/v4`, `queue/sqlite/v4`, and
   `modernc.org/sqlite` promoted indirect→direct in go.mod with +66 go.sum
   lines (test-dep transitive closure: ginkgo/gomega/rapid/pgx arrive via
   the upstream module's test graph). Verdict: hygiene, not breakage.
3. **Both spike modules build-verified green on the dirty tree.**
   `cd internal/queue/sqlitev4 && GOWORK=off go build ./...` rc=0 and same
   for `internal/queue/cqrsqlite` rc=0, under `GOEXPERIMENT=jsonv2
   GOTOOLCHAIN=auto` (rc captured by echo, PIPESTATUS rule honored). First
   attempt without `GOTOOLCHAIN=auto` failed with the documented env death —
   see §d1.
4. **The three prior reports + the migration plan read and reconciled.**
   00-21 (ADR-0019 accepted, CI repair, fork heal), 01-34, 02-35, and
   `docs/planning/2026-09-22_23-49_go-cqrs-lite-platform-migration.md`
   (C03 = scaffold `cqrsqlite`; M011/M025/M037/M044/M066/M070 chain). This
   is how the "who made these modules and why" question got answered
   without asking anyone.
5. **The two genuine defects isolated from the sanctioned noise:**
   - **Split brain (live, will drift):** TWO parallel S1 spike modules
     implement the same job. The plan (C03, M011) names `internal/queue/
     cqrsqlite`; a second module `internal/queue/sqlitev4` landed 36 minutes
     later duplicating the same tq-extras surface — art-dupl itself flags
     `cqrsqlite/extras.go:47` ↔ `sqlitev4/adapter.go:516` as a clone pair.
     Whichever survives, the other must be deleted before S1 work forks
     further.
   - **Missing divergence report:** TODO_LIST row 31 makes the divergence
     report ("token- vs executor-string finalizes, fact vocabulary,
     heartbeat facts", citing ADR-0019 §S1) part of S1's definition of done.
     Neither spike session has written it; `rg 'sqlitev4|cqrsqlite' *.md`
     matches only the plan and the 00-21 report's next-steps list. The code
     landed; the report didn't.
6. **The owner's question answered in-chat** with the timeline table, the
   sanctioned-vs-genuine split, and pointers to both defects — the alarm was
   resolved in one turn without new research beyond the session's scope.
7. **This report + its index row** (docs/status/README.md, appended per
   oldest→newest practice); gates cited in §b4.

## b) PARTIALLY DONE

1. **Verification depth: builds, not tests.** Both spike modules compile;
   NEITHER was run through tq's sqlite conformance suite by this window.
   Defensible here (a concurrent session owns the dirty files mid-edit —
   a test run racing their writes yields false reds), but "the spike works"
   remains UNPROVEN by this window; only "it builds" is claimed. Blocker:
   none once the live session settles. Effort: M (suite run + triage).
2. **The split-brain finding is recorded but not routed.** It lives in this
   report and the chat answer; TODO_LIST/AGENTS.md were not edited by this
   window (no owner ruling yet on which module survives — editing either
   file to name a winner would preempt the ruling). HARVEST should mint the
   row after §g1 is answered. Effort: S. Blocker: §g1.
3. **The spike sessions' own reports are not mine to write.** The divergence
   report requires the per-surface diff matrix (which surfaces diverge, with
   file:line) — that analysis belongs to the window that built the adapter.
   This window established only its absence. Effort: L if this window had to
   do it. Blocker: ownership (see §g2).
4. **Gates run by this window (rc=0 each, captured):** `date` (15:59:10),
   `check-status-index.sh` after the index append, `check-doc-refs.sh` after
   the report landed. NOT run (docs-only window, per the docs-only battery
   practice; tree is mid-flight under another session): root build/vet, any
   test suite, ci-local, lint baseline, nix.

## c) NOT STARTED (noticed this session; no code written)

1. **Resolving the cqrsqlite-vs-sqlitev4 split brain** — needs the §g1
   ruling first (plan says cqrsqlite; live work is on sqlitev4).
2. **S1 divergence reports for either spike** (TODO row 31's definition of
   done; also M044 in the plan).
3. **Everything downstream in the S1→S4 chain** per TODO_LIST rows 32-35:
   postgres spike, decision memo, replay tool, default flip — untouched as
   of this window.
4. **Dedup accept-list gate** (02-35 §f33 / TODO row): the 70-group paste is
   the strongest argument yet — an enforced baseline with expiry-annotated
   acceptances would have turned this alarm into "18 shown, 13
   expected-spike, 5 accepted" mechanically.
5. **Full ci-local green run on the current tree** — carried from 00-21 §b1
   (never completed end-to-end); nothing observed since suggests it ran.
6. **Fuzz workflow red since 2026-09-22 03:32** — carried from 00-21 §c3,
   still unowned.
7. **vendorHash fast gate + a nix build over the new module set** — the two
   spike go.mods are new FOD inputs; flakes see tracked files, and both are
   tracked now. Not run (owner's scope bar: no unrelated research).

## d) TOTALLY FUCKED UP

This window edited no source and risked no data — but the section demands
radical honesty, so:

1. **I burned my first build command on the documented GOTOOLCHAIN=local
   death.** AGENTS.md Commands states `GOTOOLCHAIN=auto` is REQUIRED outside
   the flake devShell ("without it every go command dies with go.mod
   requires go >= 1.27.1"), and the 00:21 window fixed ci-local for the
   exact same hazard hours ago. I ran the build without the export, ate the
   documented error, then re-ran with it. Datapoint N+1 on
   documented-miss-class failures; the env story is one line and I knew it
   before typing.
2. **The session-start ritual was skipped at turn 1.** No
   `scripts/session-start.sh`, no `git stash list`, no CONTRIBUTING.md check
   — I went straight to `git log`/`git status`, which happened to cover the
   ground, but the ritual exists so that coverage doesn't depend on luck.
   The 08-23/09-10/10-07 reports log this same miss class repeatedly;
   this window adds another datapoint instead of breaking the streak.
3. **I quoted "14 of the 18" in chat before recounting; the true count is
   13.** Caught during this report's group-by-group reconciliation. Small,
   but this repo's citation discipline (claims cite the counted artifact)
   exists precisely because confident near-miss numbers compound. The
   correction is recorded here and stated in the closing message.
4. **Tree-level damage diagnosed but NOT of this window's making — recorded
   here because it is actively harmful:** the two-module split brain (§a5)
   means tq-extras logic (RecordAnswer, PriorityScores, facts plumbing) now
   exists twice and WILL drift if both modules keep receiving work; and the
   missing divergence reports mean S1's definition of done is unmet while
   downstream rows risk being worked against an unreviewed adapter. Both are
   §f items 1-3, gated on §g.

## e) WHAT WE SHOULD IMPROVE

1. **Pin exact module paths in migration-plan rows.** The plan said
   `internal/queue/cqrsqlite` (C03/M011); the executing window chose
   `internal/queue/sqlitev4` — and BOTH now exist. A plan deviation should
   edit the plan first (or note the rename in a report), never fork into a
   second parallel artifact. Impact: a whole module's deletion + this alarm.
   Fix: one line in the ADR-0019/plan workflow ("path deviations update the
   plan in the same change").
2. **Make migration-period clone noise mechanically expected.** The
   accept-list gate (§f4) with expiry annotations ("dies at S4") converts
   spike landings from panic-paste to green-with-notes. Without it, every
   S1→S4 step re-triggers this window for the next reader. Impact: high
   during the whole migration. Fix: the gate, plus seeding it with the 13
   spike groups + 5 accepted twins.
3. **Gate new-queue-module → divergence-report pairing.** S1's definition of
   done includes a report; the code landed without it. A cheap
   `docs-health` VERIFY pass (or a check-script: new `internal/queue/*`
   module dir must be referenced by a docs/status report + the ADR) would
   catch the class. Impact: S. Fix: script or checklist row.
4. **Kill the GOTOOLCHAIN first-command tax.** Same class as the
   GOEXPERIMENT story that already needed a ci-local export: the repo could
   ship a `.envrc`/wrapper (or document `export GOTOOLCHAIN=auto` as step
   zero of session-start.sh output). Impact: one wasted command per session,
   indefinitely. Fix: S.
5. **Stop re-deriving "which reports are mine to read" — the ritual fold-in
   (carried).** §d2's miss repeats because the ritual lives in AGENTS.md
   prose, not in a script that prints at session start. Carried from 02-35
   §f34; this window is the next datapoint, not the fix.

## f) UP TO 50 THINGS WE SHOULD GET DONE NEXT (ranked; grounded, no padding — 45 items)

Impact: Critical/High/Medium/Low · Effort: S (<30min) / M (30min-2h) / L (>2h)

| # | Task | Impact | Effort | Category |
| --- | --- | --- | --- | --- |
| 1 | §g1 ruling → keep ONE S1 spike module (cqrsqlite per plan OR sqlitev4 per live work), delete the other (`git rm` + go.mod/facade/plan references updated) | Critical | S | Cleanup |
| 2 | Land the in-flight sqlitev4 edits (dead-helper removal + dep promotion) only behind a green in-module run of tq's sqlite suite against the new store | Critical | M | Quality |
| 3 | Write the S1 sqlite divergence report (finalizes semantics, fact vocabulary, heartbeat facts, tq-extras inventory) citing ADR-0019 §S1 — TODO row 31's definition of done | Critical | L | Documentation |
| 4 | Run tq's sqlite conformance suite against the surviving spike store; per-subtest verdict table (plan M025/M037) | Critical | M | Quality |
| 5 | Build the art-dupl accept-list gate (02-35 §f33): accepted-groups file, expiry annotations, growth = red — seed with 13 spike groups + 5 twins | High | M | Quality |
| 6 | S1 postgres spike over `queue/postgres/v4.0.0` judged by TQ_TEST_POSTGRES suite (TODO row 32) | High | L | Feature |
| 7 | S1 decision memo per tq-extra surface: upstream-grown vs companion-table (TODO row 33) | High | M | Documentation |
| 8 | S1 replay tool: journal → fresh engine store, projection-equality verify (TODO row 34) | High | L | Feature |
| 9 | S1 flip: default store swap + worker token finalizes + facades/vendorHash follow (TODO row 35) | High | L | Feature |
| 10 | Root go.mod + facade wiring for the surviving spike module per ADR-0016 containment (require + relative replace per facade) | High | M | Feature |
| 11 | Decide tq-fact append path (upstream escape vs companion journal) BEFORE S2 starts | High | M | Documentation |
| 12 | Full ci-local green run on the current tree (carried 00-21 §b1) | High | M | Quality |
| 13 | Master CI verdict on the post-a9e9332 lineage + spike commits (00-21 §c5 carried) | High | S | Quality |
| 14 | vendorHash fast gate (`nix build .#checks.x86_64-linux.vendor-hash`) after spike go.mods + a9e9332; copy `got:` if drifted | High | S | Bug |
| 15 | `nix build` over the tree with the two new tracked modules (new FOD inputs) | High | M | Quality |
| 16 | Diagnose fuzz workflow red since 2026-09-22 03:32 (00-21 §c3) | Medium | S | Bug |
| 17 | AGENTS.md: record the split-brain + surviving module name once ruled (§e1 class: no next session re-derives) | Medium | S | Documentation |
| 18 | HARVEST this report's §f into TODO_LIST (docs-health) | Medium | S | Process |
| 19 | Update ADR-0019 + migration plan module-name references if the survivor is NOT cqrsqlite (plan/ADR currently say cqrsqlite) | Medium | S | Documentation |
| 20 | prioritize/depbump: migrate to `decodePayload[T]` + resolve the `errors.Is` sentinel split brain (02-35 §a3 residual) | Medium | S | Cleanup |
| 21 | Rune-safe `executor.Excerpt` follow-up (02-35 TODO harvest row) | Medium | S | Bug |
| 22 | Dedicated `TestRecordRunOutcome` unit test (02-35 §b4a) | Medium | S | Quality |
| 23 | Move `LogPath` into `sessionUsage` (02-35 §b4b; wire-identical, zero literals) | Medium | S | Cleanup |
| 24 | Accept-rationale one-liners at any remaining un-annotated accepted sites (01-34 §b1 residual) | Low | S | Documentation |
| 25 | Dedup autopsy re-run at `-t 3` AFTER module resolution (02-35 §f; running it on the split brain now wastes the pass) | Medium | M | Quality |
| 26 | Owner-only: scope `.tq-verify` gofmt stage to tracked files (vendor dead-letter machine, 5+ tasks dead — AGENTS.md known issue) | High | S | Bug |
| 27 | Fold the full session-start ritual into `scripts/session-start.sh` output (carried 02-35 §f34; §d2 datapoint) | Medium | S | Process |
| 28 | Script the cheap verify battery as `scripts/verify-quick.sh` (02-35 §f33, fourth re-derivation noted there) | Medium | S | DX |
| 29 | Repo-level GOTOOLCHAIN=auto default (.envrc or session-start export) to kill the first-command tax (§e4) | Medium | S | DX |
| 30 | New-queue-module → report pairing check (§e3: script or checklist row) | Low | S | Process |
| 31 | Root-cause `TestSweepPinsCloseoutReportPaths` flake (02-35 §g1; repro protocol in that row) | Medium | M | Bug |
| 32 | tq-side prior-report-for-this-task-id lookup in session-start.sh (00-31 §d class, carried) | Medium | S | DX |
| 33 | tq-show gap: consult the queue record at session start (recurring miss class, carried 08-23/09-10) | Medium | M | DX |
| 34 | Gates-then-commit hard rule for footer commits vs daemon folds (carried, Nth repeat across reports) | Medium | S | Process |
| 35 | Dead-letter signature grouping in `tq dlq` (01-46 §f) | Low | M | Feature |
| 36 | Pre-claim DONE-row refusal in the mint path (01-46 §f) | Medium | S | Bug |
| 37 | Stop-report template for zero-delta verify windows (01-46 §f) | Low | S | Process |
| 38 | Harvester mint-skip for already-DONE rows (02-06 §f) | Medium | S | Bug |
| 39 | Sibling health-path triage lead from 02-01 §f14 | Low | M | Cleanup |
| 40 | Generic ci-local↔ci.yml parity gate (`check-parity.sh`, 00-55 §f) | Low | M | Quality |
| 41 | Sanitizer single-ownership: fold env into release-gates smoke (00-48 §e) | Low | S | Cleanup |
| 42 | Review log-path surface (09-52 §f mint) | Low | S | Feature |
| 43 | `tq api` verify row (09-52 §f mint) | Low | S | Quality |
| 44 | Verify CHANGELOG carries the ADR-0019 adoption + spike-module entries (append-only policy; not checked this window) | Low | S | Documentation |
| 45 | FEATURES.md: spike modules' status row once S1 lands (PLANNED → PARTIALLY DONE) | Low | S | Documentation |

Items 6-11 restate TODO_LIST rows 32-35 + plan chain (they are the critical
path; a §f without them would be incomplete). Items 26-43 are carried rows
from prior reports — listed because §f is the harvest ground and they were
re-confirmed live during this window's reading.

## g) THREE QUESTIONS I CANNOT ANSWER MYSELF

1. **Which S1 spike module survives — `cqrsqlite` (named by the migration
   plan C03/M011 and TODO row 31's scaffold step) or `sqlitev4` (landed 36
   minutes later, holds the newer uncommitted work)?** I tried: per-path git
   history (both arrived via daemon commits with no message explaining the
   fork), the plan text, TODO rows, and `rg` over docs — no artifact names
   sqlitev4 at all. The answer decides §f1 (a module deletion), §f19 (plan
   edits), and which divergence report gets written. If the intent was a
   rename, the plan needs the one-line update; if it was a redo, the corpse
   should be trashed now.
2. **Is the sqlitev4 window still live?** Uncommitted adapter/go.mod/go.sum
   edits sit in the tree. I cannot distinguish "agent mid-turn, will return"
   from "window died at 04:26, orphaned dirty state". The answer decides
   whether other sessions may take over/clean those files or must keep hands
   off (the never-revert rule).
3. **Dedup policy during the migration (the ruling behind §f5):** accept ALL
   spike-mirror clones until S4 deletes them (my recommendation — it matches
   the mirror-twin acceptance and the side-by-side-diff-is-the-contract
   principle), or cap/suppress them in an enforced accept-list with expiry?
   This is the same gate-vs-advisory class the owner rules on (gosec
   precedent, O5); I can build either, but the enforcement level is yours.

---

*Report by the diagnostic window (15:59). Evidence is first-hand: git
log/status per path, full `git diff` of the uncommitted spike changes, both
module builds rc=0 at the dirty tree (GOWORK=off, GOEXPERIMENT=jsonv2,
GOTOOLCHAIN=auto), and the four docs read. Point-in-time snapshot — the
concurrent sqlitev4 session may invalidates file-level claims at any moment;
re-verify before treating claims as current.*
