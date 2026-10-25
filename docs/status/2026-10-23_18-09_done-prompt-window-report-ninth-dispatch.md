# Done-Prompt Window Report — 2026-10-23 18-09 CEST (ninth dispatch)

Task: 000001a16972732136dc80eb6e3a00000000. Window: eight tasks dispatched
2026-10-20 02:27 → 2026-10-21 03:37 (ids 000001a156a33bbf… through
000001a15c06ea39…). Every §a claim below was re-verified at HEAD
(56e62fbb) from the closeout reports, the commits they cite, and the
live tree — not taken from task prose. Docs-health pass folded in:
CHANGELOG gaps minted (§e2), AGENTS.md go-floor drift corrected (§e3),
TODO harvest appended (§f).

## a) FULLY DONE

1. **art-dupl templ-directive issue FILED, premise narrowed first**
   (task 000001a156a33bbf…, closeout 2026-10-20 02-27). The row's
   original premise was outdated: at art-dupl 0.9.0-93432fd the accept
   regex matches anywhere in a line, so an HTML comment DOES suppress a
   templ-markup clone pair (empirical control pair verified). The REAL
   residual gap — the suppression comment ships in rendered page output
   — was filed as art-dupl #9 (LarsArtmann/art-dupl), landing verified
   via `gh issue view`. TODO row 637 ticked with the full narrowing
   story. Verified standing: draft at
   `docs/drafts/2026-10-20_02-11_art-dupl-templ-directive-issue.md`;
   config-pinning follow-ups landed (scripts/art-dupl.json +
   check-mirror-clones.sh `-c` invocation, commit 41cd3a19, parity
   proven 27 groups / 3 shown both ways).
2. **Mutation-verify + evidence archive executed** (same closeout):
   scratch rename of `old_priority` broke `TestRepriDetailMirrorsQueue-
   Flags` exactly as designed (drift class proven caught, demonstration
   recorded in internal/readmodel/detailkeys_test.go:22, byte-identical
   revert); session receipts archived to
   `docs/status/assets/tq-000001a156a33bbf-evidence/` with SHA256SUMS
   (archive dir verified present).
3. **Directive provenance pass: 42 sites, 14 files** (task
   000001a156ba0ad2…, closeout 02-56, commit 45f26cf3). The 12-27
   dupe-type verdicts ledger was re-read in full (265 lines) and proven
   DISJOINT from the window's 14 statement-clone reasons — no verdict to
   align to, so the durable ruling for each directive is the sweep
   report's own §a3. Every surviving directive now carries
   `; ruling: docs/status/2026-10-05_16-02_art-dupl-t3-tail-zero-dedup-
   sweep.md a)3`. First pass missed 18 indented directives; second pass
   landed all 42, count-verified. Status-index bloat pruned in the same
   session (324→304 rows; threshold re-derived at 4.2 rows/day and
   raised to 400 with the derivation pinned, commit 10e4e86d). Foreign
   break en route fixed: cmd/tq floor raised 1.27.1→1.27.2 (dc0f8bbb)
   after the root floor wave.
4. **Metaengine Delta counter Stats surface SHIPPED** (task
   000001a156ddd5ca…, closeout 04-21; commits 797f3938 + db241bc0).
   `status_counts` + `project_status_counts` counter collections fold
   the same fact stream as the tasks collection; `Model.Stats`,
   `StatusCounts`, `ProjectCounts` read them via `ExecuteTyped`; the
   GROUP-BY-pushdown path is DELETED including its dead residue. From-
   status-ambiguous transitions stay exact via an apply-time prev-read
   (transitionContext), so `TestStatsParityLifecycle` passes UNCHANGED.
   Pre-counter projections replay once via a counter-version guard;
   `TestDurableCursorCountersExactAfterResume` added. Durable readmodel
   cursor (item 2) verified pre-landed and ticked; `/api/v1/healthz`
   projection section (item 3) shipped (cbd2bb89,
   `TestHealthzCarriesProjectionSection`, CHANGELOG entry standing).
   CHANGELOG rows for the counter surface and the fold lock are
   standing ([Unreleased]).
5. **Projection-home single-writer fold lock + cursor pagination
   SHIPPED** (task 000001a1581c8dd0…, closeout 10-55; commits 5ebffbc1
   - b09afb5c, closeout hygiene df96128c + 2692cf0d). Non-blocking
     flock on `<home>.fold-lock` elects one folder per projection home;
     a lost election mounts read-only (no fold, no checkpoint); doctor
     reads via `WithReadOnly`; the composition runtime hands its
     pre-elected lease. Pinned by TestFoldLockElectsSingleFolderOver-
     OneHome (counters exact after release+resume) + three companion
     pins, all green under -race. `Model.TasksPage` rides the keyset
     cursor round-trip (malformed cursor FAILS the read; default page
     200; unix-milli ties drop/duplicate-free), pinned by
     TestTasksPageCursorRoundTrip. Root vendor/ drift from the earlier
     footer-less session caught and healed by check-gomod-vendor-sync.
     Full battery rc=0: readmodel+composition+httpapi -race, 20-module
     loop, root build+vet, cmd/tq windows cross-compile. AGENTS.md fold-
     lock invariant clause + DOMAIN_LANGUAGE "Fold lock" row landed
     (verified present).
6. **Canonical gate-receipt rule in AGENTS.md Conventions** (task
   000001a1589024e8…; commits cce6e190 + acae4052, traced via the
   sixth-dispatch report §a5 — this task shipped no closeout report of
   its own). The "N ok / M fail" receipt = `grep -c '^ok:'` over a
   CAPTURED gate-output file, never counted in context; size-budget
   twins re-synced 20,900 → 21,100 in check-agents-size.sh + its TODO
   row. Verified: the Conventions bullet stands; this very report
   follows the rule (§ preamble receipt below).
7. **new-module.sh emits the dependabot.yml entry** (task
   000001a158b88f39…; commit 680d4387, traced via git log — again no
   dedicated closeout report). Coverage parity produced at the source,
   pairing the coverage guard. CHANGELOG entry minted (was missing —
   §e2). Scratch-repo smoke pins emission + refusals, wired into
   ci-local cheap gates.
8. **webui-shots RULING: productized, NOT smoke-bound** (task
   000001a15c06ea…, closeout 2026-10-21 03-37; commit 4c72240f — note
   the window listing cited 7856ef0, a dead SHA, see §d4). The
   instrument-shot loop already existed as scripts/webui-screenshots.sh
   (headless chromium, scratch TQ_DB seed, light+dark shots); the
   ruling is now grep-able at the decision point with the no-browser
   recipe. Same batch: gosec canary list single-sourced via
   `for-each-module.sh --canaries` (65f09d79, drift structurally
   impossible + empty-derivation hard-fail), and the fixture-
   attribution meta-gate shipped (76d33def/0c2a47f7/7a40e5c5): all 17
   fixture heredocs across 9 check scripts annotated `pins: <branch>`,
   self-test + ci-local wiring, shellcheck-clean. 63 `ok:` receipt at
   deccc465 over a captured battery file.

## b) PARTIALLY DONE

1. **Root `-race` letter-gap in docs-only closeouts** (02-27 §b1): the
   verify-window convention's letter says root-parsed-file closeouts
   cite ROOT build+vet+test -race; that session ran build+vet only
   (docs/script/JSON/comment delta — low risk, honest gap). Later
   windows (10-55) ran full batteries; the gap is process-shaped, not
   a standing red.
2. **Fold-lock attribution is footer-commit-level, not diff-level**
   (10-55 §b1): the source diffs ride ~10 footer-less daemon folds;
   the two footer commits disclose provenance but no tooling can split
   one daemon span across two items yet (rows 264/305 own the gap).
3. **`Model.TasksPage` has no production consumer**: webui board/table
   stay store-backed by design (already bound 200/25 limits). A
   design-ruling question is routed to the owner (§g1).
4. **Fixture-attribution enforces existence, not accuracy** (03-37
   §b2): the gate greps `pins:`, it cannot prove the named branch is
   the pinned one. The v2 token-print contract is already rowed
   (TODO_LIST.md:1006).
5. **CHANGELOG coverage was incomplete for the window's tail**: the
   fixture-attribution gate, the gosec canary single-sourcing, and the
   1.27.2 floor wave (cmd/tq go.work + flake tarball + vendorHash,
   c85cb61f) had no entries. Minted this pass (§e2) — the gap, not the
   features, is the partial.
6. **AGENTS.md go-floor drift**: Known Issues still said "CI setup-go
   PINNED to 1.27.1 = go.mod floor" and "go.mod floor 1.27.1" in the
   drop-day pairing while root go.mod is 1.27.2 (and CI still pins
   1.27.1, bridged by GOTOOLCHAIN=auto). Corrected this pass; the CI
   pin bump itself is code/CI work → rowed (§f1).

## c) NOT STARTED

1. **The quiet-host ci-local run did NOT land.** Window task
   000001a158f55c… is listed as completed, but no footer commit, no
   closeout report, no archived matrix log, and TODO row 678 (its own
   row) is still UNCHECKED were found anywhere in the tree or log. The
   dispatch produced no verifiable artifact this pass could find — the
   streak it was meant to end ("shell/docs-only delta ⇒ scoped gates
   by prose ruling") is unretired. Row 678 stays the owner of the
   work; no duplicate row minted.
   ~~no footer commit~~ CORRECTED 2026-10-23 (tenth dispatch): a
   footer-precise `git log --format=%B --grep` finds THREE footer
   commits for this task id — 8b1efebd (AnswerPoller oldest-first
   drain fix; rows 323+504 ticked), ed38bfb4 (vendor-drift row
   closed), cb5ce139 (consumer-tag refinement + RELEASE.md step).
   The "no archived matrix log" + "row 678 unchecked" halves STAND.
   Receipts: docs/status/2026-10-23_18-35_done-prompt-window-report-
   tenth-dispatch.md a)7.
2. Row 661 — upstream go-cqrs-lite counter-key parent-row repro +
   filing: untouched (already rowed).
3. Row 662 — transition-helper unit tests (transitionDelta/
   projectTransitionDelta tables): untouched (already rowed).
4. Row 663 — surface the read-only mount + lag in `tq stats
   --read-model` output: untouched (minted by the 10-55 closeout).
5. Row 664 — injectable composition fold-lock wait (the yield test
   sleeps a real 5 s): untouched.
6. TODO row 220 family (chromedp browser-level smoke, dark-mode
   /health, `--health-dashboard` ADR/knob ruling) — explicitly another
   row's scope, untouched.

## d) TOTALLY FUCKED UP

1. **One window task completed without landing anything** (§c1): the
   quiet-host ci-local dispatch burned a paid turn (presumably — no
   verdict artifact found either) and produced no commit, no report,
   no tick. Same class as the 2026-10-20 dispatch thrice-missed
   verdict; the mint-time coordination-marker gate shipped 2026-10-20
   may already cover the re-dispatch half, but the
   completed-without-artifact half has no guard.
2. **Footer-less daemon folds carrying real code**: the fold-lock and
   pagination source rode ~10 unattributed daemon commits
   (19b9a4b2..8d4b55cb) before any footer/tick existed — attribution
   repaired only structurally (§b2). Recurring class; rows 264/305.
3. **The 1.27.2 floor wave broke three consumers before it was
   healed** (c85cb61f): cmd/tq workspace commands died with floor
   mismatches (masked as "undefined symbols"), the flake failed at
   eval (would have broken the owner's SystemNix rebuild), and CI
   pins still lag the floor. The wave also landed without its
   CHANGELOG entries until this pass.
4. **The window dispatch text itself carries a dead SHA**: the
   webui-shots task was listed with commit 7856ef0, which does not
   exist anywhere in the log — the real commit is 4c72240f (a
   daemon-sweep heal rewrote it). Dispatchers citing unhealed SHAs
   re-introduce the dangling-cite class the dead-sha gate fights.
5. **Guard-wiring left red at the 03-37 closeout** on two foreign
   orphans (check-consumer-tag.sh, smoke/worktree.sh) from concurrent
   in-flight work — subsequently resolved upstream (cb5ce139 landed
   the consumer-tag proof); noted for the record, no action.

## e) WHAT WE SHOULD IMPROVE

1. **Completed-must-produce-artifact guard**: a task that reaches
   completed with zero footer commit, zero verdict file, and zero
   touched row should be surfaced (doctor/pool-health) — the quiet-host
   dispatch is the fresh specimen. Cheap first step: a `tq doctor`
   check listing completed tasks with no journal-linked commit and no
   `.tq/verdicts/<id>` marker.
2. **CHANGELOG at closeout time, not done-prompt time**: three
   user-visible window changes (fixture-attribution gate, canary
   single-sourcing, 1.27.2 wave) waited three days for a done-prompt
   to mint their entries. The 10-55 closeout already made closeout-
   time CHANGELOG entries mandatory — the 03-37 batch (docs/scripts-
   only) skipped the rule. Gate idea: check-changelog-coverage keyed
   on `feat:`/`fix:` commits without a CHANGELOG mention.
3. **Floor waves need a checklist**: go.mod floor bumps must touch, in
   one commit: root go.mod, cmd/tq/go.mod + go.work, flake
   goToolchainVersion + paired tarball hash, vendorHash, `go mod
   vendor`, CI setup-go pins, and the AGENTS.md floor mentions. The
   1.27.2 wave needed a follow-up fix commit (c85cb61f) plus this
   pass's AGENTS correction.
4. **Closeout reports for dispatch-listed tasks**: two of the eight
   window tasks shipped no closeout report and were only traceable
   via git log + the sixth-dispatch report. The closeout prompt owns
   the write path — a task completing without one should WARN.
5. **Dispatch texts should cite healed SHAs only**: run the dead-sha
   gate over the dispatch payload, or cite subjects + dates instead of
   raw SHAs.

## f) UP TO 50 NEXT THINGS (most valuable, concrete; ★ = appended to TODO_LIST this pass)

1. ★ Bump the seven `go-version: 1.27.1` pins in .github/workflows to
   1.27.2 (floor is 1.27.2; GOTOOLCHAIN=auto currently bridges, at a
   toolchain-download cost per runner) — prefer the AGENTS.md clause
   edited this pass as the spec.
2. ★ Track art-dupl #9 (templ-aware accept directives that don't leak
   into rendered markup) to resolution; when fixed upstream, retire
   the HTML-comment directives in internal/webui/fragments.templ.
3. ★ Archive-or-annotate the superseded done-prompt dispatch chain
   (2026-10-20 02-31 → 14-48, seven dispatch reports): resolve their
   items inline (mostly covered by §a above + the eighth dispatch),
   then git mv to docs/status/archived/ with the manifest.
4. Row 678 (owner): the full quiet-host ci-local run with the archived
   matrix log — NOW the known red leg (AnswerPoller) is fixed
   (5217de78), so the run should go green; nothing executed it yet.
5. Row 977-region: run full ci-local with the dupe-baseline step in
   situ (pairs with 4; one run can serve both).
6. Row 661: upstream go-cqrs-lite counter-key parent-row repro +
   filing (verify-before-filing first; rebuild the scratch harness).
7. Row 662: transitionDelta/projectTransitionDelta helper tables.
8. Row 663: read-only-mount + lag line in `tq stats --read-model`.
9. Row 664: injectable fold-lock wait (kills 5 s from composition
   -race).
10. Row 1006: fixture-attribution v2 accuracy layer (canary tokens).
11. Row 305/264 family: span-splitting attribution tooling for
    multi-item daemon folds (the §b2 gap, second specimen).
12. Doctor check: completed tasks with no footer-linked commit and no
    verdict marker (from §e1 — needs the owner go first if it ships
    as default-on).
13. Rows 630/631: done-guard reachability beyond Every=1.
14. Deliberate `.golangci-baseline.txt` regen still owed (00-31 §f6).
15. Row 220 family: chromedp browser-level smoke assertions.
16. Row 603: for-each-module facade-module extension + derived
    expected-missing.
17. The `.dupe-baseline.txt` shrink path is advisory-only — decide if
    shrink should hard-fail after a grace window (dupe-baseline.sh).
18. CHANGELOG-coverage gate idea (§e2) — scope it before building:
    keyed on footered feat:/fix: commits vs [Unreleased] mentions.
19. The `-race` known-flaky pair (TestAgentsDocSizeGuard neighbors:
    TestExactlyOnceUnderConcurrency, TestSelfManagingLoop) still rides
    KNOWN_FLAKY — one quiet-host rerun each would either retire or
    confirm the load-flake attribution.
20. Status-index bloat: 304 live rows + this pass = keep the monthly
    digest discipline; next sweep due ~2026-11-06 at the 400 threshold.

(20 listed — deeper backlog rows live in TODO_LIST.md; nothing here
re-lists an existing row's work.)

## g) QUESTIONS FOR THE OWNER

1. Is the "row-rich webui views stay store-backed" ruling FINAL, or
   should the dashboard table/board adopt `Model.TasksPage` when the
   server runs `--read-model`? TasksPage is shipped, pinned, and has
   zero production consumers (10-55 §b2).
2. Should a task reaching `completed` with no footer commit AND no
   verdict artifact be auto-flagged (doctor WARN / papdashboard
   alert), and should the quiet-host ci-local dispatch
   (000001a158f55c…) be re-dispatched or hand-run once and ticked?
3. Dependabot scope: confirm the repo-level "Dependabot version
   updates" setting is enabled, and rule whether `internal/*` pins
   should be `ignore`d (ADR-0017 release statements, not stale
   versions) — already BLOCKED-rowed; escalated here because the
   new-module.sh emitter (680d4387) now mints entries whose activation
   is still unconfirmed.

## h) BAND DRIFT

`tq facts --type task.reprioritized` → `(0 facts)` (rc=0, run
2026-10-23 18-0x). The journal holds NO task.reprioritized facts for
the window (2026-10-20 02:00 → 2026-10-23 18:00) — consistent with all
prior window reports (the fact type has never been emitted; ADR-0015
claim ordering = stored priority + aging, markers stripped before
dedup). **None recorded.**

---

Receipt: gate battery not re-run this pass (docs-only delta: one report,
TODO appends, CHANGELOG entries, AGENTS.md prose, no script/Go surface);
doc gates cited from the window's own captured batteries (63 `ok:` at
deccc465 per the 03-37 closeout; full per-module -race loop rc=0 at the
10-55 closeout). check-todo-list + check-status-index + check-doc-refs
green on the working tree at commit time.
