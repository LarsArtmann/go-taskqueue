# Status Report — 2026-10-13 14:09 — Pareto plan, push wave, CI un-red: 5 failing classes → 2

> Owner-directed planning + push session (the SUPERB pareto paste,
> 2026-10-13 ~12:10–14:05). Follows the 11-59 sweep-6 report. Work
> commits (all pushed): `1fc442eb` (plan), `3a0e12b4` (AGENTS prune,
> amended), `8aaeaeb8` (the 87-commit pile push), `cc0def2d` (facade
> parity + gofmt), `5c19a192` (triage rows + CHANGELOG), `b6346b75`
> (gofumpt). Origin CI: run 38386387345 on `b6346b75` — 7/9 jobs green.

## a) FULLY DONE

1. **Pareto decomposition + plan**: the 20%→80% / 4%→64% / 1%→51%
   ladders over the full open-work universe (439 open rows, 107 BLOCKED,
   the executed prompting/batching problem's follow-ups, 83→87 unpushed
   commits, a production pool 3 releases stale, a red dead-sha gate) +
   the other-20% tail; written with context + mermaid execution graph to
   `docs/planning/2026-10-13_12-10_SUPERB-PARETO-ACTIVATION-PLAN.md` —
   Table A (29 tasks, 30–100 min, ALL TODO areas, sorted), Table B (70
   micro-tasks ≤12 min), owner decision nodes D1–D4, explicit
   non-goals (no TODO_LIST mass-edit — verschlimmbesserung guard). Both
   table views reported inline as ordered.
2. **AGENTS.md size-guard prune (21,261→19,790 B)**: overnight windows
   grew the file 1,227 B past the 20,000 budget; the prune debt fell to
   this window mid-push-wave (M20 doctrine). Fact-preserving wording
   compression across ~18 entries; KNOWN_FLAKY pin phrasing restored
   after check-verify caught my rewording (§d1); agents-size
   19,790/20,000 OK, check-verify + check-doc-refs green.
3. **The push (owner-ordered)**: full ci-local run with the documented
   conscious `CI_CHECK=off` bypass (origin red predates tree — row 436
   class); local battery proved `tests (-race)` green incl.
   `internal/worker 3.842s` (the origin worker panic does NOT
   reproduce at HEAD — fixed inside the pile); push initially rejected
   (origin gained the nightly fuzz commit) → clean rebase of all 86
   commits (zero conflicts) → pushed `baa31407..8aaeaeb8`.
4. **CI un-red, three root-caused fixes**: (i) facade parity —
   `internal/executor.WorktreeOrphan` shipped without its facade type
   alias (daemon-swept past the pre-commit parity gate); alias added,
   parity 7/7 OK, `executor` module build+vet+test green → `test` job
   GREEN. (ii) treefmt — `RequeueClass*` const block misaligned in the
   queue facade + internal twin + worker test (gofmt-fixed), then the
   REAL formatter revealed: treefmt uses **gofumpt** (anonymous-struct
   param form in `worktree_test.go`, invisible to host gofmt);
   gofumpt-scanned all module dirs (single offender), `checks.treefmt`
   green locally → `nix` job GREEN. (iii) two remaining classes triaged
   into TODO rows with root causes.
5. **Final origin CI on `b6346b75` (run 38386387345): 7/9 green** —
   test, test-postgres, consumer, cqrs-lint, gosec, nix, fuzz; the two
   red jobs are PRE-EXISTING at origin (present in run 38239884399 on
   `d14689ca`, before this session's push) and now rowed:
   govulncheck (7+ stdlib net/http advisory call-sites — toolchain
   patch-bump triage) and test-windows (`t.TempDir unlinkat` on the
   projection db — a Windows open-handle leak in the readmodel close
   path; linux unaffected).
6. Process held where it mattered: every daemon race (4 this session:
   plan file, AGENTS prune, facade fix, + the earlier README) healed via
   the sanctioned exactly-mine unpushed soft-reset fold with the fold
   disclosed in the commit message.

## b) PARTIALLY DONE

1. **T1 (push wave)**: pushed and CI improved from FIVE failing classes
   to TWO — but "CI re-runs green" (row 38's wording) is not fully
   true: govulncheck + test-windows stay red (pre-existing, rowed).
   Row 38 left open pending those two.
2. **The plan is executed at M1–M5 only** (gate classification, push,
   CI watch); T2–T29 + D1–D4 preparation remain plan-on-paper.
3. **Dead-sha gate**: still red at HEAD (row 691, 289 dangling
   citations) — untouched this session beyond planning; the cure pack
   (T2) not built.

## c) NOT STARTED (deliberately, per plan sequencing)

1. T2 dead-sha cure pack; T3 deploy-prep bundle (gates D1, the single
   highest-leverage owner action); T4 outcome-join audit.
2. T6 check-archive-eligibility.sh; T7 dep-sweep retirement (O15 time
   gate OPEN since 2026-10-13 — today).
3. T8 2026-10 corpus sweep; T9 digest row; T10/T11/T13 prompting
   follow-ups (D2-gated); T20 owner-decision pack.
4. The 12-23 §f10 annotation strike — identified in the PREVIOUS
   session, re-listed as plan T5/M20, still not executed.

## d) TOTALLY FUCKED UP (honest defects)

1. **Committed through a failing gate**: the AGENTS prune commit went
   in while `check-verify.sh` was FAILING — I chained `bash check-verify |
   tail -2; bash check-doc-refs && git commit`, so the verify FAIL could
   not block the commit. Caught one command later (the KNOWN_FLAKY
   bidirectional pin rejected my merged bullet), fixed and amended
   (`3a0e12b4`) — but a gate-blind commit is the verify-then-commit
   violation class, full stop.
2. **The aborted battery masked real failures**: the first full
   ci-local died at the known row-687 drift step; I classified it
   "known class, proceed" and pushed — CI then caught facade parity +
   treefmt that the never-run LATER local steps (nix check, and the
   CI-only parity/treefmt legs) would have surfaced. The plan's own M2
   micro-step ("classify rc, continue past known-class red") was
   written AFTER I needed it. Two fix-push cycles paid for it.
3. **Daemon raced 4× in one session** (plan, prune, facade fix, README
   earlier): each heal was clean, but I repeatedly left multi-file
   batches uncommitted across long verification commands — the sweep-6
   §d1 lesson (commit at sub-phase boundaries) re-hit within 24 hours.
4. **First push rejected (non-fast-forward)**: no fetch before push in
   a repo where concurrent windows + the nightly fuzz job push
   routinely. One wasted round trip; rebase was clean.
5. **Formatter-blind verification**: the `cc0def2d` commit message
   claimed "scoped gofmt zero findings" — true but insufficient:
   treefmt formats .go with gofumpt, whose rules host gofmt cannot see.
   I verified against the wrong formatter and only the next CI run
   revealed it.
6. Minor: the first ci-local invocation predictably died at check-ci
   (row 436 documents that red-predates-tree aborts the battery) —
   could have gone straight to the documented conscious bypass.

## e) WHAT WE SHOULD IMPROVE

1. **fetch-before-push** as a mechanical pre-push step in this
   multi-agent repo (one `git fetch origin` + ahead/behind check).
2. **Batteries must not stop at the first known-class red**: run the
   remaining steps, classify EACH fail — an aborted battery is a false
   green for everything after the abort point.
3. **Verify with the gate's own formatter**: a cheap
   `nix build .#checks.x86_64-linux.treefmt` belongs in every
   push-prep battery (gofumpt ⊃ gofmt; host gofmt cleanliness proves
   nothing about treefmt).
4. **Never chain a gate whose FAIL must block** with `;` before
   `git commit` — rc-capture to a file, then commit (the AGENTS
   convention exists verbatim; I violated it once and it bit once).
5. **ci-local↔ci.yml parity gap exposed**: facade parity and treefmt
   are CI/pre-commit-only — a daemon-swept commit can break both with
   no local gate firing (row 228's sweep should add both legs).
6. **The M20 prune ritual needs teeth**: the size guard went red
   overnight and the debt landed mid-push-wave on an unrelated window.
   Options: per-closeout prune check (row 442-class ruling pending) or
   a daemon-side pre-commit size check.

## f) Up to 50 things we should get done next

Not 50 — honest, ranked; plan refs in parens.

1. T2: dead-sha cure pack (row 691) — the pre-push gate is red for
   every window; build the pack, apply on the D3 ruling.
2. Govulncheck triage row execution: toolchain patch-bump decision
   (go 1.27.1 → latest 1.27.x carrying the net/http fixes) vs
   accept+baseline — needs the floor/pins walked together.
3. Test-windows handle-leak: windows-repro loop (CI re-run with focus)
   on TestStatsReadModelSeam + TestUseReadModelReplaysDeletedProjectionHome;
   pin the leaking readmodel/sqlite handle (durable cursor? DLQ sidecar?
   srv close ordering vs LIFO t.Cleanup), fix close ordering.
4. T3: deploy-prep bundle — unblocks D1 (owner SystemNix flip), the
   plan's 1%→51% action.
5. T4: outcome-join audit (validates the v0.3.4 prompting redesign
   with data; feeds D2).
6. T6: check-archive-eligibility.sh (rows 239/438) — eighth manual
   sweep should never happen.
7. T7: dep-sweep retirement — O15 time gate opened TODAY; verify
   Dependabot week-1, retire flag+sweeper+docs (row 39).
8. T8+T9: 2026-10 corpus docs-health sweep + first digest row
   (index 293+ live rows).
9. T10/T11/T13: prompting follow-ups (de-micromanage, batching
   calibration, batch-sim + yield audits) — D2-gated.
10. T20: owner-decision pack — all 107 BLOCKED rulings ranked with
    one-line stakes; unblocks D1–D4 en masse.
11. T5/M20: the twice-dropped 12-23 §f10 strike + 04-11 row folds.
12. Row 228 extension: add facade-parity + treefmt legs to ci-local
    (this session's push exposed the gap).
13. Row 687: gomod-vendor-sync instability (aborted this session's
    first battery; tidy-scope ruling still open).
14. Row 442-class: AGENTS.md budget policy ruling (20,000 hard +
    prune ritual vs standing headroom) — tonight's mid-wave prune is
    the second datapoint in a week.
15. Row 38: close-or-annotate once govulncheck + test-windows resolve
    (push done; "CI green" 2 classes away).
    16–50: refused — Table A (T14–T29) already ranks the remainder
    with micro-breakdowns in Table B; minting filler is noise.

## g) Questions I can NOT figure out myself

1. **D2 — prompting follow-ups go/no-go** (row 40 tail + plan T10/T11/
   T13): execute the de-micromanage rewrite + batching raise now, and
   under what budget shape per batch (per-item vs per-batch cap)? One
   ruling unblocks three tasks and the A/B audits.
2. **Govulncheck posture**: may I bump the toolchain patch pin
   (1.27.1 → latest 1.27.x with the net/http fixes) across go.mods +
   setup-go + the flake goTarball pin (NEVER-lower-floor rule honored,
   but it moves the release floor and the vendorHash) — or is the
   stdlib-advisory red acceptable until the v0.4.0 window?
3. **AGENTS.md budget policy** (row 442-class, recurring): keep
   20,000 B hard with a mandatory per-closeout prune ritual, or reset
   to standing headroom (~21,000)? The guard works, but tonight the
   prune debt landed mid-push-wave on an unrelated session — second
   occurrence this week.
