# 2026-10-05 16:02 — art-dupl -t 3 Tail Zero: Dedup Sweep Closeout

Session type: interactive dedup sweep (deduplicate-code skill), continuing the
14-02 `2026-10-05_14-02_art-dupl-t4-zero-dedup-sweep.md` window which shipped
the -t 4 zero but left "the -t 3 canonical tail (16 idiom groups) judged not
zeroed" open. This session closed that tail.

Input: user-supplied `art-dupl --sort total-tokens -t 3 --type-aware --html`
report — 227 detected groups, 211 suppressed, **16 actionable groups / 36
clones** shown. Every flagged site was read at file:line before disposition.

## a) FULLY DONE

1. **Full 16-group triage.** Every clone site read and dispositioned:
   extract / accept-with-directive / accept-residual. No group skipped.
2. **Group #8 extracted (the one real clone).** cqrsqlite `fromUFacts`
   (internal/queue/cqrsqlite/adapter.go, 20 lines byte-identical to
   `companion.JournalFacts`) deleted; both call sites now delegate to
   `companion.JournalFacts`. cqrsqlite joins sqlitev4/postgresv4 in the
   ADR-0019 "adapters delegate to internal/queue/companion" pattern; the
   now-unused `ufacts` import removed with it.
3. **36 art-dupl:accept directives with recorded reasons** across 14 files
   (21 doc-position + 17 inline), covering 14 of the 15 remaining groups:
   SQL query prologs + scan-loop tails (companion/reads.go ×4 funcs + tails,
   sqlitev4/migration oldStatusCounts/oldAllFacts), mutex prologs
   (lockout ×3), conformance idioms (conform/tests.go ×6 tests), doctor
   check prologs (×2), exec buffer wiring (agent/verifygate/command/depbump),
   executor entry seams (review/status), subcommand wiring (cmdDLQ/cmdFacts/
   cmdWatermarks/sessionList).
4. **Directive placement semantics established empirically** (new, recorded
   in AGENTS.md): a directive only suppresses when it sits within a few lines
   ABOVE the clone's first line. Doc-position works for func-top clones;
   mid-function clones need an inline comment adjacent to the start.
5. **Report zeroed except one documented residual.** Final canonical run:
   **2 clones / 1 group** (was 36/16), 225 suppressed. The survivor is the
   `fragments.templ` conditional-span pair (sessions lamp vs attempts/error
   preview) — same shape, different domain content, and Go comments cannot
   live inside templ markup without being rendered as literal text.
6. **Verification battery, all green:**
   - per-module build+vet+test: companion, sqlitev4, lockout, executor
     (11.1s), cqrsqlite (3.7s pre + 3.9s post companion edits)
   - cmd/tq module gate `test-cmd-tq.sh`: ok 12.8s
   - root build + vet + test: green (root module incl. webui, session,
     status, facadeparity)
   - `go mod vendor` clean; `check-mirror-clones.sh`: **0 cross-backend
     clone groups**; `check-facade-parity.sh`: 7/7 facades OK
   - `gofmt -l` on every touched package: clean
7. **4 em-dash house-rule violations in my own new comments** caught and
   fixed (conform/tests.go directives).
8. **AGENTS.md updated** with the directive-placement gotcha + the named
   templ residual (Conventions, executor-seams bullet).

## b) PARTIALLY DONE

1. **Foreign doc-ref break investigation (stopped on instruction).**
   `check-doc-refs.sh` rc=1: ROADMAP.md:287 cites `internal/budget.SessionUsage`
   which the checker reports MISSING. The symbol EXISTS
   (internal/budget/budget.go:174) — the checker appears to resolve
   symbol-style citations as file paths. One-line ROADMAP citation change
   fixes it; investigation and fix halted because the status request landed
   mid-investigation. Pre-existing/foreign: my edits touch neither file.
2. **Verification depth.** Targeted gates green, but NOT run:
   `root-gate.sh` full suite, `-race` on the 6 touched modules (the
   concurrent-agent noise rule), nix build checks.
3. **No CHANGELOG entry** for the sweep (policy question, see g3).
4. **HARVEST of section (f) into TODO_LIST** not run (awaiting instructions;
   docs-health owns the routing rigor).

## c) NOT STARTED

1. Long-term disposition of the templ residual: upstream art-dupl feature
   request for directives usable inside templ markup, and/or a repo-level
   art-dupl config (`-c` JSON) exclusion so canonical runs can show literally
   zero groups. Currently: accepted-by-documentation only.
2. Foreign pre-existing item observed, untouched: gopls warns
   `internal/queue/sqlitev4/go.mod:11` requires `internal/queue/sqlite`
   but the module never uses it.
3. Provenance cross-check of my 14 accept-reasons against the recorded
   12-27 dupe-type verdicts ledger (alignment assumed, not re-read).

## d) TOTALLY FUCKED UP

Nothing broke in the tree; every gate ends green. Honest missteps, in order:

1. **Round-1 placement assumption wrong.** Applied 21 doc-position
   directives believing they suppress any clone inside the function; deep
   clones (cmd/tq wiring, scan-loop tails, conform assertion pairs) survived
   the re-run (36→19 clones, 16→9 groups) and needed a second full cycle
   with inline directives. Cost: one extra ~3-minute full type-aware scan
   and a second edit round.
2. **Two aborted batch scripts.** First insertion batch died at insert 10/17
   (`t.Fatalf("Requeue: %v", err)` anchor matched 2 sites), leaving a
   partially applied batch; second batch died at insert 2/8
   (`RescueDead` anchor matched 2). Both recovered with wider disambiguating
   context; no double-application occurred (each `sub` asserts count==1
   before writing).
3. **Anchor-uniqueness pre-checked too late.** The `grep -c` pass existed but
   ran before only SOME batches; two anchors slipped through it and aborted
   scripts mid-flight.
4. **Em dashes in source comments.** Wrote 4 before recalling the house
   rule; fixed same session.
5. Minor mechanics: one `git add -A ':!vendor'` pathspec error; one staging
   window where the daemon swept `adapter.go` between my edits (verified
   benign via `git show` before continuing).

## e) WHAT WE SHOULD IMPROVE

1. **Empiricism before scale:** probe the suppression window with a DEEP
   clone first, then batch. One 30-second probe would have saved the whole
   round-1/round-2 split.
2. **`--json` plumbing from step 1** for exact clone ranges; sed-context
   guessing produced both aborted batches.
3. **Assert anchor counts for every anchor up front** (grep -c table), not
   inline during the batch.
4. **Cite rulings in directive reasons** (e.g. "12-27 verdicts") so accepts
   carry durable provenance instead of only a local rationale.
5. **Prune redundant directives:** the round-1 doc-position directives on
   cmdDLQ/cmdFacts/cmdWatermarks/sessionList/review/status suppress nothing
   (their clones are deep) and now duplicate the inline ones' meaning — a
   byte-neutral prune would slim the comments.
6. **Report format note:** this report is `.md` per the prompt's explicit
   path demand (status-report skill's HTML-canonical default overridden by
   explicit user instruction; standing task-queue exception shape).

## f) NEXT (grounded in this session's observations; impact-first)

| #  | Item                                                                              | Why / cite                                                                                                                        |
| -- | --------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| ~~ | 1                                                                                 | Fix ROADMAP.md:287 citation to a real path (`internal/budget/budget.go`)                                                          |
| ~~ | 2                                                                                 | Drop unused `internal/queue/sqlite` require from sqlitev4/go.mod; then vendor-hash check                                          |
| ~~ | 3                                                                                 | `root-gate.sh` certification at this HEAD                                                                                         |
| ~~ | 4                                                                                 | `-race` re-run across the 6 touched modules                                                                                       |
| ~~ | 5                                                                                 | CHANGELOG row for the -t 3 tail zero (links 14-02 report)                                                                         |
| ~~ | 6                                                                                 | Prune redundant round-1 doc directives (cmd/tq, review, status)                                                                   |
| ~~ | 7                                                                                 | HARVEST this report's (f) into TODO_LIST (docs-health)                                                                            |
| 8  | File upstream art-dupl issue: directives inside templ markup                      | would let the last group carry an accept too                                                                                      |
| ~~ | 9                                                                                 | Decide config-exclusion vs documented residual for fragments.templ pair                                                           |
| 10 | Commit an art-dupl config (-c) pinning the canonical invocation                   | ad-hoc runs then match the gate flags by default                                                                                  |
| 11 | Cross-check 14-02 report's open items (mutation-verify pins, evidence filing)     | my sweep complements, did not execute them                                                                                        |
| 12 | Re-read 12-27 verdicts ledger; align my 14 accept reasons                         | provenance pass (e4)                                                                                                              |
| ~~ | 13                                                                                | Symbol-style citation audit across docs (`.pkg.Symbol` vs path)                                                                   |
| 14 | Index-bloat check: live status rows vs 100-row warning threshold                  | 12-27 report cited 206 files vs threshold                                                                                         |
| ~~ | 15                                                                                | Proofread directive texts for house style (terse, no em dash)                                                                     |
| ~~ | 16                                                                                | `nix build .#checks.x86_64-linux.vendor-hash` after item 2                                                                        |
| ~~ | 17                                                                                | Consider an AGENTS.md tightening: replace "residual groups accepted" with the single named residual + directive-semantics pointer |
| 18 | Optional: nightly fuzz unaffected — verify ci-local stays green once at this HEAD | routine post-sweep certification                                                                                                  |

Deliberately NOT invented: items requiring research outside this session's
scope (worker internals, release engineering, platform work) — those belong
to their own sweeps, not to a dedup closeout's backlog.

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

~~1. **Foreign break ownership:** ROADMAP.md:287's dead citation predates my~~ answered 2026-10-05: fixed on sight at 4be4a8ef (one-line citation-format fix under the fix-on-sight owner grant)
window (neither file touched by me). Fix it myself now (fix-on-sight),
or leave it for whichever agent owns the roadmap rows (never-touch-what
-you-didn't-author)?
~~2. **Templ residual endgame:** is "accepted residual, documented in~~ answered 2026-10-05: documented residual is the durable answer; config exclusion declined (see item 9)
AGENTS.md" the durable answer for the fragments.templ pair, or do you
want canonical art-dupl runs to show literally zero groups (config
exclusion, or upstream templ-directive support)?
~~3. **CHANGELOG policy:** does an internal-only dedup sweep (zero API or~~ answered 2026-10-05: no row for internal-only sweeps; dated status reports are the record (see item 5)
behavior change) earn a CHANGELOG row in this repo, or is CHANGELOG
reserved for user-facing deltas?

## Verification receipts

- art-dupl final: 1 group / 2 clones (templ pair), 225 suppressed — canonical
  invocation, type-aware, -t 3
- gates: cqrsqlite 3.9s, executor 11.1s, companion/sqlitev4/lockout green,
  cmd/tq 12.8s, root build+vet+test green, mirror-gate 0 groups, facade
  parity 7/7, gofmt clean, `go mod vendor` clean
- check-doc-refs rc=1 solely on the foreign ROADMAP row (item f1); no
  MISSING rows from this session's edits
