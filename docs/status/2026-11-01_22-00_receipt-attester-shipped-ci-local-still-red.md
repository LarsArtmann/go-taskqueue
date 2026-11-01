# Receipt + attester shipped; ci-local still red on a worktree-merge assertion

Session 20:28→22:00 (2026-11-01). Undispatched owner GO on the Pareto plan
(`docs/planning/2026-11-01_20-17_SUPERB-okf-receipt-attester-pareto-plan.md`):
"THE WHOLE TODO LIST", execute and verify one step at a time.

Scope: the plan's 25 tasks / 102 micro-tasks (the items derived from the OKF
research window), not the pre-existing 619-row `TODO_LIST.md` backlog, which has
its own harvest and dispatch mechanism.

Concurrent agents + the auto-commit daemon ran throughout; sixteen footer-less
daemon sweeps absorbed this session's files (content-attributed and baselined).

## a) FULLY DONE

**The 1% — the receipt and the attester (R1–R4).**

1. `executor.VerifyReceipt` (`internal/executor/receipt.go:23`) records
   `command`, `rc`, `head`, `ok`, `fail`. `runVerify` returns a `VerifyOutcome`
   built from what the run actually did; `AgentResult.Verify` (success),
   `FailureEvidence.Verify` (failure) and `StatusResult.Verify` all carry it.
   `verify_tail` alone could not answer the two questions every closeout asks:
   which gate ran, and how many checks passed.
2. `ParseVerifyCounts` (`:57`) is the ONE implementation of the `^ok:`/`^FAIL`
   rule, and `TestParseVerifyCountsMatchesCloseoutGrep` compares it against a
   real `grep -c '^ok:'` subprocess — the emitted number cannot drift from the
   hand-grep it replaces. Measured justification recorded in the type's doc: 34
   living reports carry 59 hand-derived "N ok / M fail" receipts, and 26 files
   say the words `grep -c '^ok:'` outright.
3. `AttestVerify` (`:199`) is a pure function over (receipt, payload pin, repo
   `.tq-verify`, claimed HEAD, captured output) with three comparisons: head
   equality (abbreviated shas accepted), executed-command membership in the
   pinned set, count re-derivation from the output. `TestAttestVerify` has 3
   green + 3 red cases, one per comparison.
4. `TestAttestVerifyWhitespaceToleranceIsNotVacuous` proves a REWRITTEN command
   still refuses (re-wrapped whitespace passes, `true && …` does not) — the
   comparison cannot be satisfied by anything.
5. `TestAttestVerifyReportsSkippedComparisons` proves a comparison that could not
   run is reported in `Checked` instead of reading as a pass.
6. The verdict is stamped at WRITE time on both paths
   (`AgentResult.Attestation` / `FailureEvidence.Attestation`), naming its class
   (`head-mismatch`, `command-mismatch`, `count-mismatch`, `no-receipt`) plus the
   mismatch detail. `TestAgentRunStoresReceiptAndVerdict` and
   `TestVerifyFailureStoresReceiptAndVerdict` pin the wiring end to end.
7. `TestAttestationRefusesReresolvedGate` makes the `--reresolve-verify` trust
   hole OBSERVABLE: a re-resolved claim (pin dropped, gate auto-detected from a
   `Makefile`) refuses with `command-mismatch` naming the violated pin. That was
   the OKF note's one finding with no mechanism behind it.

**The 4% — the surfaces (R5).**

8. `tq show` renders a top-level `verify` section (verdict + fields); `tq dlq`
   rows carry `verify=<verdict>` and the JSON row carries the receipt.
9. Both ride `cmd/tq/verifyreceipt.go`, a documented ADR-0017 twin, whose JSON
   spelling is pinned against the REAL executor struct by
   `TestVerifyReceiptViewDecodesExecutorSpelling`; absence decodes to nil
   (`TestVerifyReceiptViewAbsentIsNil`), and the three surfaces have their own
   tests (`TestCmdShowRendersVerifySection`, `TestCmdDLQRowsCarryVerifySuffix`).

**The 20% — docs and guards.**

10. `scripts/check-cite-drift.sh`: re-reads every living-doc `path:NNN` claim;
    fails on a missing file, a line past EOF, or an anchor that is UNIQUE in the
    file but nowhere near the cited line. Six pre-existing drifts baselined with
    reasons; `--self-test` pins all branches; wired into `ci-local.sh`.
11. `scripts/check-research-note-shape.sh`: every template-era note needs a
    citations TABLE (a heading plus prose is not a table) and a non-empty
    Verification trail; a missing `stale_after` is advisory-counted; two
    pre-template notes grandfathered. `--self-test`, ci-local-wired.
12. The research-note template now states both obligations: record every
    acquisition pass including failures (`fetch` empty is a tool mismatch, not a
    dead URL), and re-read each cite before committing (with the command).
13. `stale_after` is a template field, absolute not TTL, and all three
    template-era notes carry one with a justification.
14. Five trust words in `docs/DOMAIN_LANGUAGE.md` (provenance, trust, freshness,
    lifecycle, attestation) with the tq mechanism and the OKF name beside each,
    plus the two rules the vocabulary exists to make obvious.
15. Forward-compat pinned at every remaining decode surface: `internal/queue`
    (parser tolerance, malformed-detail skip, wire-key pin), `internal/journal/
    cqrs` (unknown type and unknown keys cross verbatim; malformed detail is
    refused at marshal, which is byte validity, not a key whitelist),
    `internal/readmodel` (each of the eight ledgerless families skipped BY NAME,
    with detail that would fail a decode), `internal/executor` (payload
    type-mismatch + a canary for a future strict-by-default Go).
16. The note's receipt lesson moved from "planned" to SHIPPED with every cite
    re-read at HEAD, and §10.2/§10.4/§10.5 re-read against the shipped field set:
    seven deltas ruled (2 already-true, 2 rejected as ceremony, 1 recorded
    divergence — tq stores the verdict because its consumer reads months later).
17. `docs/planning/2026-11-01_20-49_reresolve-verify-retire-or-narrow.md` — the
    flag traced to its ONE fork, the incident it answered, the doctor's three
    staleness classes, both options costed, recommendation KEEP+NARROW with the
    observability half already shipped.
18. `docs/planning/2026-11-01_21-05_trust-tier-ruling-brief.md` — three costed
    options (A no tier / B stored fold / C render-time) with a recommendation and
    the load-bearing sub-question ("which write owns the field") named.
19. Six foreign dead-SHA cites and one unattributed daemon sweep ROUTED as
    `— BLOCKED:` TODO rows with reasons, never laundered; two ideas recorded as
    explicit REJECTIONS (a second lifecycle channel in the TODO grammar,
    provenance-on-closeout-claims without a consumer).
20. Three pre-existing reds found and FIXED at source: the nix job's treefmt
    failure (nixfmt on the routed-`ExecStart` interpolation, formatting only,
    byte-exact assertions intact), the prettified `app.css` (dprint had expanded
    the generated minified sheet to 8,154 lines — now excluded, with the gate's
    header recording why), and two smoke scripts committed 644 (Permission denied
    aborted the whole battery).

**Gates, rc to files (no `PIPESTATUS` used anywhere):** root-gate rc=0,
mirror-clones rc=0 (0 cross-backend clone groups), dead-exports rc=0 (+
`--self-test` rc=0), script-syntax rc=0 (127 checks, shellcheck 0 findings),
doc-refs ok, todo-list ok, agents-size 24398/24400, status-index ok,
research-note-shape ok, cite-drift ok, lint-baseline within baseline
(2399 = 2399), test-cmd-tq rc=0, and the four touched module gates
(queue, journal/cqrs, readmodel, executor) build+vet+test rc=0.

## b) PARTIALLY DONE

1. **`ci-local.sh` is NOT green.** It now reaches the worktree-per-agent live
   smoke, which fails on its SECOND tree assertion: the two deliveries merge
   (`review: merged` twice, ≥2 merge commits) but `work-taskone.txt` is missing
   from `HEAD`'s tree. The FIRST assertion (`worktree_dir` on both completions)
   was failing earlier because of my own regression (see §d1); with that fixed
   the smoke advances and stops here. Attribution is OPEN — see §f1.
2. **The `--evidence-stage` receipt term is not implemented.** `verify-ok` /
   `verify-mismatch` cannot be added yet: cmd/tq resolves `internal/*` from the
   last published tag (ADR-0017) and the tagged `FailureEvidence` has no
   `Attestation` field, so the CLI cannot pass it to the matcher. Adding the term
   only in the executor would advertise a term the CLI warns is unknown. Filed.
3. **The count comparison is tautological in-process** (both sides come from the
   same buffer). Its teeth are post-hoc re-grading of a stored sidecar, which
   needs a command that does not exist. Filed.
4. **The nix flake-check itself was not re-run.** The formatter was applied and
   the diff is formatting-only, but "treefmt is happy" is asserted from the
   formatter's own output, not from `nix build .#checks.x86_64-linux.treefmt`.
5. **`--reresolve-verify` and the trust tier are briefs, not decisions** — both
   owner-gated by design.

## c) NOT STARTED (with reasons)

1. Removing the `cmd/tq/verifyreceipt.go` twin — blocked on the release that
   re-pins cmd/tq past the receipt.
2. A stored-receipt audit command (`tq verify attest`) — deferred by the plan.
3. Fixing the six foreign dead-SHA cites — their author must assert the new sha.

## d) TOTALLY FUCKED UP

1. **I deleted a live statement with a "harmless" whitespace edit.**
   `result.WorktreeDir = runDir` got merged into the preceding comment line, so
   the assignment became comment text: every worktree completion recorded its
   branch but not its checkout path, the build stayed green, and only the
   worktree smoke could see it. Fixed in `eb512007`. **Lesson: an edit that
   reports success on a comment/assignment boundary must be re-read, not
   trusted.** The `edit` tool's whitespace normalisation is exactly the risk
   here — this was a multi-line block replaced with a version whose last line
   lost its newline.
2. **I compressed AGENTS.md until a guard's cross-check phrase disappeared.**
   Rewording "`TestX` is load-flaky" into a parenthetical broke
   `check-verify.sh`'s bidirectional KNOWN_FLAKY↔AGENTS pin. AGENTS.md is
   MACHINE-READ gate input in places; a compression pass must run the guards
   after it, not before.
3. **I edited `internal/executor` after the last `go mod vendor`.** vendor/ holds
   copies of internal-module sources, so a one-line doc-comment edit drifted the
   vendor hash and ci-local failed on it. Vendor AFTER the last internal edit.
4. **I wrote 122- and 125-character lines in a new test and assumed `gofmt`
   covered formatting.** It does not: golines is a separate formatter under the
   same baseline gate. Two ci-local runs burned on it.
5. **The cite-drift gate needed three precision passes to be usable** (86
   findings → 13 → 6): bare basenames must be skipped (docs cite other projects'
   files), an anchor must be UNIQUE in the file (a helper used five times cannot
   locate a line), and only single-cite lines can attribute identifiers. A gate
   that noisy gets disabled, so the tightening was not optional.
6. **A `trap ... RETURN` inside the drift gate's scan function fired on every
   nested helper return**, deleting the tracked-file index after the first cite
   and making resolution silently stop working mid-run. Found because the
   baseline did not match its own `--emit-baseline` output.
7. **I asserted a line number from memory inside the note that criticises that
   practice.** The drift gate's first real catch was my own
   `cmd/tq/main.go:2599`, written from recall minutes after the edit.
8. **I reported "the whole list" as done while ci-local was red.** The first
   honest status of this window is this report, and it says red.

## e) WHAT WE SHOULD IMPROVE

1. **Re-read every edit that touches a comment/code boundary.** A swallowed line
   is invisible to the compiler, invisible to `gofmt`, and visible only to a live
   smoke that may be red for other reasons for weeks.
2. **Run the guards that READ AGENTS.md after editing it** (`check-verify.sh` at
   minimum). Then re-check the size budget.
3. **Vendor after the LAST internal-module edit**, and treat `internal/*` doc
   comments as content (they are, to the vendor hash).
4. **Run the formatter set, not `gofmt`.** `gofumpt` + `goimports` + `golines`
   are all gated; a new file that "looks formatted" can still fail the battery.
5. **New gates need a precision budget.** A first run with 86 findings is a gate
   nobody keeps.
6. **A red pre-push gate must be reported as red.** "Everything green" plus a
   failed `ci-local` is the kind of claim this repo keeps paying for.
7. **Cheap gates first, battery last.** Four battery runs were spent on defects
   that a 30-second preflight (grep the phrase the guard wants; `go mod vendor`;
   run the formatter) would have caught.

## f) NEXT — up to 50, ordered by leverage

1. **Attribute and fix the worktree smoke's second failure** (`work-taskone.txt`
   missing from `HEAD` after two merges). Start from
   `internal/review/sweep.go`'s `mergeOnApprove` + `git ls-tree HEAD` on the
   kept fixture (`TQ_SMOKE_KEEP=1`). This is the only thing standing between
   `ci-local` and green.
2. Re-run `ci-local.sh` to green and record the receipt (rc to a file).
3. `nix build .#checks.x86_64-linux.treefmt` + `…nixos-module-eval` to convert
   "the formatter ran" into "the CI check passes".
4. Land `verify-ok`/`verify-mismatch` evidence terms with the cmd/tq re-pin.
5. Drop the `cmd/tq/verifyreceipt.go` twin in that same change.
6. Build `tq verify attest` (re-grade a stored receipt against a stored sidecar).
7. Rule on `--reresolve-verify` (`docs/planning/2026-11-01_20-49_…`).
8. If narrow: add the `.tq-verify`-modified check at gate time.
9. Rule on the trust tier (`docs/planning/2026-11-01_21-05_…`).
10. Add `ParseVerifyCounts` output to `tq doctor --hygiene`.
11. Emit receipts from `review`/`dlqfix`/`prioritize` (same `runVerify`).
12. Teach the closeout prompt to quote `verify.ok`/`verify.fail`.
13. Retire the AGENTS "Gate receipts" prose rule once the emitted counts are
    trusted.
14. Add a `tq show --verify-only` projection for scripts.
15. Pin the receipt in `internal/queue/companion/conform`.
16. Add the receipt to the webui task detail pane.
17. Expose `attestation.refusal` as a doctor warning class.
18. Route the six foreign dead-SHA cites to their windows.
19. Attribute or grandfather `826aa46f`.
20. Re-resolve the six baselined cite drifts in `TODO_LIST.md`.
21. Add the cite-drift gate to `root-gate.sh`'s cheap HEAD set.
22. Extend `check-cite-drift.sh` to `docs/planning/**` once clean.
23. Give `check-cite-drift.sh` a dry-run print for `--emit-baseline`.
24. Teach `check-research-note-shape.sh` to require a lesson TABLE.
25. Backfill `stale_after` on the two grandfathered notes.
26. Add the acquisition-pass rule to the closeout prompt for research tasks.
27. Move the `fetch`-empty rule into `.crushrc` hook advice.
28. Give `VerifyReceipt` a `String()` for row surfaces.
29. Add a `tq audit --verify` view over receipts.
30. Revisit per-stage counts only if a gate ever declares stages (REJECTED now).
31. Add the receipt to `FEATURES.md` when its scope is ruled.
32. Add a CHANGELOG `Added` row if the receipt becomes operator-visible.
33. Re-run `scripts/fuzz/nightly.sh` (untouched this window).
34. Triangulate `TestExactlyOnceUnderConcurrency` under load.
35. Smoke: `tq show` renders `verify` for a fixture task.
36. Smoke: `tq dlq` prints `verify=<refusal>` for a command-mismatch fixture.
37. Measure `gitHeadRev`'s per-task latency at gate time; cache per claim if real.
38. Document `VerifyOutcome`'s fields in `docs/DOMAIN_LANGUAGE.md`.
39. Add the attester to the `internal/executor` package doc's export map.
40. art-dupl re-read of the receipt work against `review`/`status`.
41. Decide whether `no-receipt` should be a STORED verdict (currently absent).
42. Re-run `check-gosec.sh` (new code in executor + cmd/tq).
43. Re-run `check-erraudit.sh` per module after the new helpers.
44. Re-run `check-mirror-clones.sh` now that the executor grew a helper.
45. Consider an `Attestation` freshness stamp (when was the verdict produced).
46. Fold this window's sweeps into a footered marker commit if the owner prefers
    attribution over baseline rows.
47. Audit for OTHER comment-swallowed statements (the class, not the instance):
    grep for `\.\t`-joined comment lines across the tree.
48. Re-tighten the AGENTS budget constant toward 22k (14 B headroom is a hazard).
49. Add a `ci-local` preflight mode that runs the cheap gates first.
50. Push and watch the first green CI run since before this window.

## g) QUESTIONS FOR THE OWNER

1. **Receipt scope**: operator-visible v0.5 feature (CHANGELOG `Added`, docs,
   bump) or repo-internal evidence promoted later? Today it is internal only.
2. **`--reresolve-verify`**: retire, or keep-and-narrow as the brief recommends?
3. **Trust tier**: option A (no tier, recommended), B (stored fold), or C
   (render-time)?

## h) VERDICT

The plan's 1% is shipped and pinned: every gate run now records which gate ran
and how it exited, a pure attester grades it against what was pinned, a refusal
is stored and rendered, and the `--reresolve-verify` hole is finally observable.
The doc-honesty gates exist and caught my own errors twice. What is NOT done is
the last mile: `ci-local.sh` is red on a worktree-merge assertion whose
attribution is open, two nix checks are asserted from the formatter rather than
the check, and the daemon-attribution gate is red by exactly one foreign sweep
that is routed rather than laundered. Everything else is green with an rc file
behind it.
