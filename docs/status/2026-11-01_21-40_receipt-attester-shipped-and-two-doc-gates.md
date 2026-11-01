# The receipt and the attester shipped: every gate run now proves which gate ran

Session 20:28→21:40 (2026-11-01). Undispatched owner GO on the Pareto plan
(`docs/planning/2026-11-01_20-17_SUPERB-okf-receipt-attester-pareto-plan.md`):
"THE WHOLE TODO LIST", execute and verify one step at a time.

Scope note: "the whole list" here is the plan's 25 tasks / 102 micro-tasks (the
50 items derived from the OKF research window), not the pre-existing 619-row
`TODO_LIST.md` backlog, which has its own harvest and dispatch mechanism.

Concurrent agents + the auto-commit daemon ran throughout; fourteen footer-less
daemon sweeps absorbed this session's files as it went (content-attributed and
baselined, §a13).

## a) FULLY DONE

1. **The verify receipt exists and both paths carry it.** `executor.VerifyReceipt`
   (`internal/executor/receipt.go:23`) records `command`, `rc`, `head`, `ok`,
   `fail`; `runVerify` now returns a `VerifyOutcome` that builds it from what the
   run actually did; `AgentResult.Verify` (success) and `FailureEvidence.Verify`
   (failure) both carry it, and `StatusResult.Verify` too. The `verify_tail`
   alone could not answer "which gate ran" or "how many checks passed" — the two
   questions every closeout asks.
2. **The counts agree with the hand-grep by construction.** `ParseVerifyCounts`
   (`:57`) is the ONE implementation of the `^ok:` / `^FAIL` rule, and
   `TestParseVerifyCountsMatchesCloseoutGrep` compares it against a real
   `grep -c '^ok:'` subprocess — the emitted number and a reader's own count
   cannot diverge.
3. **The attester is a pure function with three comparisons.**
   `AttestVerify` (`:199`) grades (receipt, payload pin, repo `.tq-verify`,
   claimed HEAD, captured output): head equality with abbreviated-sha support,
   executed-command membership in the pinned set, and count re-derivation from
   the output. `TestAttestVerify` has 3 green + 3 red cases (one per comparison),
   `TestAttestVerifyWhitespaceToleranceIsNotVacuous` proves a REWRITTEN command
   still refuses, and `TestAttestVerifyReportsSkippedComparisons` proves a
   comparison that could not run is reported rather than read as a pass.
4. **The verdict is stamped at write time, on both paths.**
   `AgentResult.Attestation` / `FailureEvidence.Attestation` carry the
   `VerifyVerdict`; a refusal names its class (`head-mismatch`,
   `command-mismatch`, `count-mismatch`, `no-receipt`) plus the mismatch detail.
   `TestAgentRunStoresReceiptAndVerdict` and
   `TestVerifyFailureStoresReceiptAndVerdict` pin the wiring end to end.
5. **`--reresolve-verify`'s trust hole is now observable, and there is a test for
   it.** `TestAttestationRefusesReresolvedGate` runs a re-resolved claim (pin
   dropped, gate auto-detected from a `Makefile`) and requires
   `command-mismatch` naming the violated pin. This was the one finding of the
   OKF note that had no mechanism behind it.
6. **The receipt is surfaced, not just stored.** `tq show` renders a top-level
   `verify` section (verdict + fields); `tq dlq` rows carry `verify=<verdict>`
   and the JSON row carries the receipt. Both ride `cmd/tq/verifyreceipt.go`, a
   documented ADR-0017 twin whose JSON spelling is pinned against the REAL
   executor struct by `TestVerifyReceiptViewDecodesExecutorSpelling`.
7. **A cite-drift gate exists** (`scripts/check-cite-drift.sh`): re-reads every
   living-doc `path:NNN` claim and fails on a missing file, a line past EOF, or
   an anchor that is unique in the file but nowhere near the cited line. Six
   pre-existing drifts are grandfathered, and the gate caught a wrong line of
   MINE on its first real run (`cmd/tq/main.go:2599`, asserted from memory).
8. **A research-note shape gate exists** (`scripts/check-research-note-shape.sh`):
   every template-era note needs a citations TABLE (a heading plus prose is not a
   table) and a non-empty Verification trail; `stale_after` is advisory-counted.
   Two pre-template notes are grandfathered with reasons.
9. **The research-note template now states the two obligations those gates
   check**: record every acquisition pass including the failed ones (`fetch`
   empty is a tool mismatch, not a dead URL), and re-read each cite before
   committing.
10. **`stale_after` is now a template field, absolute rather than a TTL**, and
    the three template-era notes carry one with a justification (OKF v0.3 would
    invalidate specific lessons; turnstone is pinned at a sha; paperclip pins
    `@master`).
11. **Five trust words are named in `docs/DOMAIN_LANGUAGE.md`** — provenance,
    trust, freshness, lifecycle, attestation — each with the tq mechanism and the
    OKF name, plus the two rules the vocabulary exists to make obvious (trust
    never comes from a score; absence is not invalidity).
12. **Forward-compatibility is pinned at every remaining decode surface**:
    `internal/queue` (parser tolerance, malformed-detail skip, wire-key pin),
    `internal/journal/cqrs` (unknown fact type and unknown detail keys cross
    verbatim; malformed detail is refused at marshal, which is byte validity not a
    key whitelist), `internal/readmodel` (each of the eight ledgerless fact
    families skipped by NAME, with corrupt-looking detail that would fail a
    decode), and `internal/executor` (payload type mismatch + a canary comment for
    a future strict-by-default Go).
13. **All fourteen daemon sweeps attributed by content and baselined**, with a
    dated reason each; one mixed sweep says so. Exactly ONE unattributed sweep
    remains (`826aa46f`, a `.golangci-baseline.txt` regen this window did not
    make) and it is routed as TODO debt, not laundered.
14. **The nix job's red is explained and fixed.** Master CI has been red since
    before this window: `treefmt`'s nixfmt wanted the routed-`ExecStart`
    interpolation and the module-eval assertion wrapped. `nix run .#fmt --
    flake.nix` + `deploy/nixos/tq-agent-pool.nix`, formatting only; the
    byte-exact assertions are unchanged.
15. **Gates run, rc to files** (no `PIPESTATUS`): root-gate rc=0, mirror-clones
    rc=0, dead-exports rc=0 (+ `--self-test` rc=0), script-syntax rc=0 (127
    checks, shellcheck 0 findings), doc-refs, todo-list, agents-size
    (24398/24400), research-note-shape, cite-drift, test-cmd-tq rc=0, and the
    four touched module gates (queue, journal/cqrs, readmodel, executor) rc=0.

## b) PARTIALLY DONE

1. **`ci-local.sh` is not yet green end to end.** It found three real defects in
   sequence (a broken `is load-flaky` phrase my own AGENTS compression caused; a
   vendor/ drift from editing `internal/executor` after the last vendor pass; and
   a golines violation in the new cqrs test). All three are fixed and committed;
   the final run was in flight when this report was written. Not claimed green.
2. **The `--evidence-stage` receipt term is NOT implemented.**
   `verify-ok` / `verify-mismatch` cannot be added yet: cmd/tq resolves
   `internal/*` from the last published tag (ADR-0017) and the tagged
   `FailureEvidence` has no `Attestation` field, so the CLI cannot pass it to the
   matcher. Adding the term only in the executor would advertise a term the CLI
   warns is unknown. Filed as a TODO row that lands with the re-pin.
3. **The receipt's count comparison is tautological in-process** (both sides come
   from the same buffer). Its real teeth are post-hoc re-grading of a stored
   sidecar, which needs a command that does not exist yet. Filed as a TODO row.
4. **`--reresolve-verify`'s ruling is a brief, not a decision.** The
   retire-vs-narrow note recommends KEEP+NARROW with the observability half
   already shipped; the narrowing half (refuse to re-resolve from a `.tq-verify`
   the run modified) is owner-gated.
5. **The nix flake-check itself was not re-run.** The formatter was applied and
   the diff is formatting-only, but "treefmt is happy" is asserted from the
   formatter's own output, not from `nix build .#checks.x86_64-linux.treefmt`.

## c) NOT STARTED (with reasons)

1. **Removing the `cmd/tq/verifyreceipt.go` twin.** Blocked on the release that
   re-pins cmd/tq past the receipt.
2. **A stored-receipt audit command** (`tq verify attest`) — needs the sidecar as
   an input and a consumer; deferred by the plan itself.
3. **Fixing the six foreign dead-SHA cites.** They belong to three other
   windows' reports and the fix is a fork record only their author can assert.

## d) TOTALLY FUCKED UP

1. **I compressed AGENTS.md until a guard's cross-check phrase disappeared.**
   Rewording "`TestX` is load-flaky" into a parenthetical broke
   `check-verify.sh`'s bidirectional KNOWN_FLAKY↔AGENTS pin. The gate caught it;
   the lesson is that AGENTS.md is a MACHINE-READ gate input in places, and a
   compression pass must run the guards after, not before.
2. **I edited `internal/executor` after the last `go mod vendor`.** vendor/ holds
   copies of internal module sources, so a one-line doc-comment edit drifted the
   vendor hash and ci-local failed on it. The gate healed the tree and told me to
   re-run; the correct ritual is vendor AFTER the last internal edit, always.
3. **I wrote a 122-character line and a 125-character line in a new test file**
   and assumed `gofmt` covered formatting. It does not: golines is a separate
   formatter under the same golangci-lint baseline gate. Two ci-local runs burned
   on it.
4. **The cite-drift gate needed three passes to become usable, and each pass
   taught a rule**: bare basenames must be skipped (docs cite other projects'
   files), an anchor must be UNIQUE in the file (a helper used five times cannot
   locate a line), and only single-cite lines can attribute identifiers. The
   first version produced 86 findings, mostly noise — a gate that noisy gets
   disabled, so the tightening was not optional.
5. **Floorplan error found inside the drift gate itself**: a `trap ... RETURN`
   inside the scan function fired on every nested helper return, deleting the
   tracked-file index after the first cite. It made resolution silently stop
   working mid-run. Found by the baseline not matching its own emit output.
6. **I asserted a line number from memory in the very note that criticises
   that.** The drift gate's first real catch was `cmd/tq/main.go:2599` — my own
   cite, written from recall, off by a few lines after the edit that added it.
   Exactly the failure the note describes, one document later.

## e) WHAT WE SHOULD IMPROVE

1. **AGENTS.md edits must be followed by the guards that read it** —
   `check-verify.sh` at minimum, since it greps for exact phrases. A compression
   pass without it is one reworded bullet away from a red battery.
2. **Vendor after the LAST internal-module edit**, not after "the feature".
   vendor/ is a content hash over internal module sources; doc comments count.
3. **`gofmt -l` is not "formatted".** The repo gates gofumpt + goimports +
   golines; a new file must be run through the formatter set (or the module's
   lint run) before the battery.
4. **New gates need a precision budget, not just a self-test.** 86 findings at
   first run would have been disabled; the fixes were precision rules, and they
   belong in the script's header so the next author does not re-learn them.
5. **State what a stored verdict CANNOT do.** The write-time attestation is
   honest because it names the comparisons it performed; the next reader must not
   mistake "ok" for "every comparison ran" when a declaration was absent.
6. **One preflight pass beats three battery runs.** Both ci-local failures after
   the first were cheaply preflightable (grep AGENTS for the phrase the guard
   wants; `go mod vendor`; run the formatter).

## f) NEXT — up to 50, ordered by leverage

1. Re-run `ci-local.sh` to green and record the receipt.
2. Run `nix build .#checks.x86_64-linux.treefmt` (and `nixos-module-eval`) to
   convert "the formatter ran" into "the CI check passes".
3. Land the `verify-ok`/`verify-mismatch` evidence terms with the cmd/tq re-pin.
4. Drop the `cmd/tq/verifyreceipt.go` twin in that same change.
5. Build `tq verify attest` (re-grade a stored receipt against a stored sidecar).
6. Rule on `--reresolve-verify` (brief: `docs/planning/2026-11-01_20-49_…`).
7. If narrow: add the `.tq-verify`-modified check at gate time.
8. Rule on the trust tier (brief: `docs/planning/2026-11-01_21-05_…`).
9. Add `ParseVerifyCounts` to the doctor's hygiene output (counts on pending
   pins' last receipts).
10. Emit the receipt from the `review`/`dlqfix`/`prioritize` turns too (they run
    their own gates through the same `runVerify`).
11. Teach the closeout prompt to quote `verify.ok`/`verify.fail` instead of
    hand-grepping.
12. Delete the AGENTS "Gate receipts" prose rule once the emitted counts are
    trusted (it becomes a description, not an instruction).
13. Give `tq show` a `--verify-only` projection for scripts.
14. Pin the receipt in the conform suite (`internal/queue/companion/conform`).
15. Add the receipt to the webui task detail pane.
16. Expose `attestation.refusal` as a doctor warning class.
17. Route the six foreign dead-SHA cites to their windows.
18. Attribute or grandfather `826aa46f`.
19. Re-resolve the six baselined cite drifts in `TODO_LIST.md`.
20. Add the cite-drift gate to `root-gate.sh`'s cheap HEAD set.
21. Extend `check-cite-drift.sh` to `docs/planning/**` once its baseline is clean.
22. Add a `--emit-baseline` dry-run print (per the doc-mutation convention).
23. Teach `check-research-note-shape.sh` to require a lesson TABLE too.
24. Backfill `stale_after` on the two grandfathered notes.
25. Add the acquisition-pass rule to the closeout prompt for research tasks.
26. Move the `fetch`-empty rule into `.crushrc` hook advice (it is a tool trap,
    not a repo fact).
27. Give `VerifyReceipt` a `String()` for the row surfaces.
28. Include `verify.command` in `tq dlq --json`'s row (already in the receipt).
29. Add a `tq audit --verify` view over receipts.
30. Consider per-stage counts if a gate ever declares stages (recorded as
    REJECTED for now; see the note's §10.4 delta row).
31. Trim the 1.6k-line `VerifyReceipt` doc comment if it ever needs a pointer
    instead (it is currently the design record).
32. Add the receipt to `FEATURES.md`.
33. Add a CHANGELOG `Added` row if the receipt becomes operator-visible.
34. Re-run `scripts/fuzz/nightly.sh` (untouched this window).
35. Triangulate the `TestExactlyOnceUnderConcurrency` flakiness under load.
36. Add a smoke that asserts `tq show` renders `verify` for a fixture task.
37. Add a smoke that asserts `tq dlq` prints `verify=<refusal>` for a
    command-mismatch fixture.
38. Check whether `gitHeadRev` at gate time costs a measurable per-task latency.
39. Cache the gate-time HEAD for the whole claim when the repo has not moved.
40. Document the `VerifyOutcome` fields in `docs/DOMAIN_LANGUAGE.md`.
41. Add the attester to the `internal/executor` package doc's export map.
42. Re-read the receipt work for art-dupl clones against `review`/`status`.
43. Consider an `Attestation.Verified` timestamp (freshness of the verdict).
44. Decide whether `no-receipt` should be stored as a verdict rather than
    absent (currently absent, deliberately).
45. Add the receipt to the `internal/budget` docs if paid-turn accounting reads
    it.
46. Re-check `check-mirror-clones.sh` now that the executor grew a helper.
47. Re-run `check-gosec.sh` (new code in executor + cmd/tq).
48. Re-run `check-erraudit.sh` per module after the new helpers.
49. Fold this session's sweeps into a footered marker commit if the owner wants
    attribution without the baseline rows.
50. Re-tighten the AGENTS budget constant back toward 22k (the 14-byte headroom
    is a working hazard, not a feature).

## g) QUESTIONS FOR THE OWNER

1. **Receipt scope**: operator-visible v0.5 feature (CHANGELOG `Added`, docs,
   version bump) or repo-internal evidence promoted later? Today it is internal
   only — `CHANGELOG.md` has no row for it.
2. **`--reresolve-verify`**: retire, or keep-and-narrow as the brief recommends?
3. **Trust tier**: option A (no tier, recommended), B (stored fold), or C
   (render-time)?

## h) VERDICT

The 1% of the plan — the receipt, the attester, and the surfaces — is shipped
and pinned by nine new tests including a non-vacuity counterfactual. The guards
that keep documentation honest (cite drift, note shape) exist and caught my own
errors twice during this window. What is NOT done is the last mile of the
battery: `ci-local.sh` was still running at report time, and the two nix checks
were asserted from the formatter's output rather than from the checks
themselves. Everything else here is green with rc files to prove it.
