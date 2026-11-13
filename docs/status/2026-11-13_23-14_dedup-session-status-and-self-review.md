# 2026-11-13 23-14 — Session status: art-dupl `-t 1` dedup sweep (self-review + a–g)

**Scope line — what this report VERIFIES vs NOT.** Everything under (a) was
verified this session with captured rc/output (per-module `go test` runs,
root `go build && go vet && go test ./... -race -count=1` rc=0,
`scripts/test-cmd-tq.sh` rc=0 rc-to-file, canonical + `-t 1` art-dupl JSON
scans in `.crush/shell-output/dedup-*.json`, gofmt/dead-exports/mirror/
todo-list/status-index/doc-refs/release-docs gate outputs, AGENTS byte count
27,190/27,200). NOT verified and why: the FULL `scripts/ci-local.sh` battery
(not run — scoped gates + root race + cmd/tq only; foreign-break retry
budget unspent), `lint-baseline.sh --check` growth gate (not run), gosec
triage over the new `internal/slogcapture` package (not run; stdlib-only
surface), the "pre-existing" provenance of the 2 hidden mirror groups
(asserted from untouched-files reasoning, no `git log` citation), the
`open dev.mod` line in the first test-cmd-tq run (observed, rc=0 anyway,
never root-caused). This report covers THIS session's work only — no
re-research of unrelated subsystems.

## a) FULLY DONE

1. **Recon of the governed dedup regime** — `scripts/art-dupl.json`
   (threshold 3, total-tokens, typeAware) found and adopted as canonical;
   ~60 in-tree `art-dupl:accept` directives inventoried; prior sweep
   reports (t5/t4/t3/t2) consulted for ledger precedent.
2. **Full `-t 1` triage** — all 176 actionable groups read and classified
   (rendered inventory + fragments in `.crush/shell-output/t1-tail.txt`);
   the 162 groups below the governed threshold accepted BY CLASS (idiom
   families: `stats.X++` fold tails, `if err != nil { continue }`,
   `case <-ctx.Done()`, CLI `mustOpenDB` prologs, templ formatInt call
   sites, conform assertion shapes).
3. **Nine extraction seams landed, each tested immediately**:
   `internal/slogcapture.Handler` + guard test (replacing 4 byte-identical
   `captureHandler` doubles in dlqfix/harvest/review/status);
   `runOverRepos` harvest fixture (4 sites);
   `openDoctorStore` (9 doctor_test sites, close moved to `t.Cleanup`);
   `bufferedProcessGroup` (executor agent/verifygate probe convention);
   conform `requeueEvidenceOf` + `trailOf` (3 requeue-decode loops);
   templ `fullOutputLine` (4 blocks — **TODO row 535 executed**);
   webui `openSSEStream` (2 SSE prologs); readmodel `statusCounts`
   delegating to `matchesFilter` (removed a live filter-rule drift
   hazard); slogcapture guard test pinning capture/Records/copy
   isolation.
4. **Canonical scan reduced 14→8 groups / 43→22 clones, zero harmful** —
   every survivor verdicted in the sweep report §c table (worker prolog,
   prioritize fold twin, seq-codec parity mirror ×2, CLI flag shape,
   templ card-badge twins ×2, readmodel loop skeletons).
5. **Verification battery (scoped)** — slogcapture/dlqfix/harvest/review/
   status `go test -count=1` ok; executor module build+vet+test ok 36.4s;
   companion + conform via sqlitev4 ok 5.0s + postgresv4 build ok;
   readmodel ok; webui (post `templ generate`) ok 15.5s; cmd/tq full
   suite ok 7.4s; root build+vet+`test -race -count=1` rc=0; gofmt clean
   over all 15 touched files; mirror gate rc=0; doc-refs/release-docs/
   status-index/check-todo-list green.
6. **AGENTS.md ledger updated in place** — templ-twin naming corrected
   (was: "sole templ twin" — stale), 11-13 sweep verdict pointer added,
   upstream-#9 wording tightened; 27,190/27,200 B,
   `TestAgentsDocSizeGuard` green.
7. **TODO_LIST**: row 535 → [x] with DONE pointer; config row
   (`scripts/art-dupl.json` canonical invocation) → [x]; NEW row minted:
   home-or-baseline the 2 NEW HIDDEN cross-backend mirrors.
8. **CHANGELOG `### Internal` row** recorded (doc-refs gate caught a
   backticked non-path cite `internal/slogcapture.Handler`; fixed to a
   real path cite).
9. **Sweep report #1 written + indexed**:
   `2026-11-13_21-01_dedup-t1-sweep-zero-harmful.md` with verdict table
   and receipts.
10. **Trigger audit (AGENTS.md)**: reads.go `scanInto` trigger audited
    DISSOLVED in-source (reads.go:150-152 design comment — a helper
    would defer Close before the caller's scan loop; loops already
    delegate to scanTask/scanFacts); doctor pending-listing and
    review/status validator triggers: no third site appeared.

## b) PARTIALLY DONE

1. **Verification scope** — scoped gates + root race + cmd/tq ran; the
   full `ci-local.sh` battery (with its retry budget and nix/check-ci
   legs) did NOT run. The tree is committed by the daemon; a full battery
   is the remaining confidence step before any push claim.
2. **`-t 1` tail verdicts** — recorded by CLASS in report §a/§c prose,
   not as a per-group machine-readable table. Auditable, but a future
   accept-list gate would want hash-level entries.
3. **Mirror provenance** — the 2 NEW HIDDEN [return] mirrors
   (sqlitev4/postgresv4 adapter.go + archive.go) were declared
   "pre-existing (none of this sweep's files)" WITHOUT a `git log -1`
   citation per group. Likely right, unproven.
4. **app.css artifact** — now convention-correct (minified, 164,601 B,
   key classes spot-checked) but the expanded→minified 8k-line transition
   rides daemon commits unreviewed; the race that produced the first
   stale artifact has a written lesson but NO wired guard.
5. **AGENTS headroom** — 10 bytes left. The ledger is accurate but the
   next content edit (e.g. row 1812's --suggest-generics pin) requires
   pruning elsewhere first.
6. **gosec / lint-baseline** — new package + touched files not passed
   through the growth gates; expected clean (stdlib-only), unproven.

## c) NOT STARTED (all observed this session, none researched beyond it)

1. Home-or-baseline the 2 hidden mirrors (row minted this session).
2. Full `ci-local.sh` run on the folded tree.
3. `lint-baseline.sh --check --clean-cache` growth check.
4. gosec triage pass over `internal/slogcapture`.
5. `git log` provenance citation for the 2 mirror groups (upgrade the
   TODO row from claim to cite).
6. Row 423 (accept-list gate in ci-local) — still BLOCKED on owner
   ruling; this sweep's 171-group `-t 1` tail is the data for it.
7. Row 761 — repoint hardcoded art-dupl flag invocations at
   `scripts/art-dupl.json`.
8. Row 1812 — pin `--suggest-generics` in AGENTS dedup prose (needs the
   byte-budget prune first).
9. Directive-class → ruling-doc ledger table under docs/ (prior row).
10. art-dupl #9 tracking → retire fragments.templ HTML-comment directives
    when upstream lands templ-aware suppression.
11. Monthly status-index digest ritual (405 live rows vs 400 threshold —
    warning fired during this session's gate run).
12. Root-cause the `open dev.mod` stderr line in `scripts/test-cmd-tq.sh`
    (script exits rc=0; unverified whether the line is expected shim
    chatter).
13. Wire the css-regen ordering lesson (run `nix run .#webui-css` only
    after the daemon folded template edits, or `git add` first — flakes
    see tracked files only) into a script note or the AGENTS convention
    line.
14. Experiment: `-t 1 --min-tokens 3` rerun to measure how much of the
    idiom tail a min-token floor collapses (informs question 1 below).
15. `tq verdict` for this session — deliberately skipped (interactive
    sweep, shells inherit production `TQ_DB`); recorded here as the
    convention deviation it is (report-without-verdict = half-closeout
    for paid turns; see question 3).

## d) TOTALLY FUCKED UP (own errors, in order of badness)

1. **Assertion-first citation in the sweep report** — I wrote the `-t 1`
   after-picture numbers into the report BEFORE the scan finished
   (invented "176→125 / 458→340" shape), then corrected to the observed
   171/440 after `job_output` returned. The exact class the repo bans
   ("observe-first for pinning"); nothing consumed the wrong numbers, but
   the report was wrong on disk for one edit cycle.
2. **css-race misdiagnosis** — the post-regen 8,407-line deletion looked
   like data loss; it was actually the committed convention (minified)
   correcting an expanded artifact. I briefly treated a format change as
   corruption before verifying content (sizes + class spot-checks). The
   REAL bug was the stale tree race, which I found second.
3. **Piped rc masking** — ran `./scripts/test-cmd-tq.sh | tail -6` and
   captured tail's rc, losing the script's — the precise PIPESTATUS-trap
   class the tq-bash-guard denies, repeated despite the AGENTS warning.
   Recovered with rc-to-file re-run (true rc=0).
4. **Two import-edit slips** in one multiedit batch: dropped `"testing"`
   from harvest/discovery_test.go (compiler caught: 12 errors) and
   inserted unused `"slices"` into review/sweep_test.go (grep caught).
   Root cause: reusing an edit template across files without re-reading
   each import block against its own usage.
5. **Wrong type name on first write** (`*Harvest` vs `*Harvester`) —
   didn't check the package's constructor return before writing the
   helper signature.
6. **Self-minted clone a37293bf** — my own `requeueEvidenceOf` fetch
   prolog immediately twinned `assertAnsweredFact`'s; had to extract
   `trailOf` to dissolve it. Iterate-to-zero applies to extractions too.

## e) WHAT WE SHOULD IMPROVE

1. **Capture all after-scan numbers BEFORE writing report prose** — the
   report's §d numbers must be paste-from-captured-output, never
   anticipated. (Direct fix for d1.)
2. **Provenance citations for every "pre-existing" claim** — one
   `git log -1 --format='%h %s' -- <file>` per file, cited in the row.
   (Direct fix for b3.)
3. **Multiedit discipline for imports** — per-file: read import block +
   grep usage BEFORE inserting; or compile immediately after each file,
   not after the batch. (Direct fix for d4.)
4. **Declare the verification scope in the final message**, not just the
   report — "scoped gates + root race; full ci-local NOT run" prevents a
   scoped-green being read as battery-green.
5. **Machine-readable accept ledger** — the class-level prose verdicts
   should become a committed hash/class table (feeds an eventual
   art-dupl `baseline`/`check` CI gate; upstream already ships those
   subcommands — no reinvention needed).
6. **Split-brain audit passed, keep it that way**: the ADR-0003
   path-not-link rationale now lives ONLY in the `fullOutputLine`
   component comment (trimmed from statusReportCard) — verified no
   residual double-home. Ghost-system check on the new package: all five
   Handler methods are exercised by the guard test directly, so the
   dead-exports blind spot (interface-only usage) is mitigated.
7. **Nothing useful was removed**: the 5th harvest prolog (autopsy-mint
   test) and the gofmt probe's 3-line variant were deliberately KEPT —
   both need state/behavior the helpers deliberately don't cover.
   Re-extraction pressure should go through the trigger list, not
   blanket merges.

## f) Top ~35 things to get done next (session-grounded, sorted by impact; beyond ~25 these are brainstorm/ROADMAP fuel per the skill)

| # | item | impact | effort |
| --- | --- | --- | --- |
| 1 | Run full `scripts/ci-local.sh` on the folded tree | high | M |
| 2 | `git log` citations for the 2 hidden mirrors; then home in companion (accepts-≈-0 ruling) or baseline with provenance | high | M |
| 3 | `lint-baseline.sh --check --clean-cache` growth check over the sweep's diff | med | S |
| 4 | gosec triage over `internal/slogcapture` | med | S |
| 5 | Owner ruling → build the art-dupl accept-list gate (`baseline`/`check` in ci-local) from this sweep's hash table | high | M |
| 6 | Machine-readable accept table (hash+class+verdict) committed under docs/ | med | S |
| 7 | Row 761: repoint hardcoded art-dupl invocations at `scripts/art-dupl.json` | med | S |
| 8 | Row 1812: pin `--suggest-generics` in AGENTS prose (prune 100+ bytes elsewhere first — only 10 B headroom) | med | S |
| 9 | Monthly status-index digest ritual (405/400 warning is live) | med | M |
| 10 | Root-cause `open dev.mod` line in test-cmd-tq.sh output (expected chatter vs bug) | low | S |
| 11 | Wire the css-regen-after-daemon-fold lesson into AGENTS convention line or a script guard | med | S |
| 12 | `-t 1 --min-tokens 3` experiment; quantify the floor's effect on the idiom tail | low | S |
| 13 | art-dupl #9 upstream tracking → retire HTML-comment directives in fragments.templ when fixed | low | S |
| 14 | Directive-class → ruling-doc ledger table under docs/ (prior open row) | med | M |
| 15 | Decide `tq verdict` policy for interactive sweeps (see question 3) | med | S |
| 16 | Re-verify the 5 canonical accept classes after the next behavior-change wave (sweep cadence precedent: `-t 3` after behavior changes) | med | M |
| 17 | conform suite: extract skip guards (`testing.Short` ×2, postgres-spike skip ×2) if a third appears (accepted class today) | low | S |
| 18 | calibration_read map-init guard twin — revisit at third site | low | S |
| 19 | postgresv4 same-file open-error-wrap pair — fold into the adapter-delegator home at next adapter touch | low | S |
| 20 | Excerpt/payloadOrEmpty fallback pair — revisit only if literals converge | low | S |
| 21 | defaultDB/parkBy env-fallback shape — a shared envx helper candidate at third occurrence | low | S |
| 22 | templ `tq-panel` Table twins — revisit during the next webui table touch | low | M |
| 23 | Add the `-c scripts/art-dupl.json` canonical invocation to AGENTS Commands block (byte budget again) | low | S |
| 24 | Run `check-dead-exports.sh --self-test` variant once to pin gate behavior for interface-only methods (slogcapture pattern) | low | S |
| 25 | Annotate the 21-01 sweep report if/when items 1-4 above change its §d scope claims (docs-health ANNOTATE, never rewrite) | low | S |
| 26 | Bake the `-t 1` tail classes into the dedup ledger doc so the next sweep triages by lookup, not re-reading | med | M |
| 27 | Verify `TestAgentsDocSizeGuard` failure message top-sections output still names the right prune targets after this session's rewrite | low | S |
| 28 | Sweep the LSP baseline: the `dupl` pair in dlqfix sweep_test 1257/1294 (sibling refusal-gate pins) — table-drive them or directive them at next dlqfix test touch | low | S |
| 29 | Ask upstream art-dupl for per-group suppression reasons in `--json` (this session bisected hashes by hand — prior upstream-feedback row still open) | med | S |
| 30 | Consider `"minTokens"`/`"minLines"` in scripts/art-dupl.json as the governed floor once question 1 is answered | low | S |
| 31 | Re-run the full `-t 1` scan after the mirror row lands (set-diff against `.crush/shell-output/dedup-t1-after.json`) | low | S |
| 32 | Move `.crush/shell-output/dedup-*.json` scan artifacts into an evidence archive if they're worth keeping (check-ghost-archives gate first) | low | S |
| 33 | Fold the "5th harvest prolog kept deliberately" rationale into the harvest test file header so the next dedup sweep doesn't re-litigate it | low | S |
| 34 | Same for the gofmt probe's non-process-group variant (verifygate comment) | low | S |
| 35 | After any future templ edit: `templ generate` → daemon fold → `nix run .#webui-css`, in that order — encode in the workflow, not memory | med | S |

## g) Three questions I can NOT figure out myself

1. **Accept-list gate policy (row 423, BLOCKED on you):** should the
   accepted clone set be ENFORCED mechanically — art-dupl
   `baseline`/`check` wired into ci-local so any NEW group fails CI — or
   does the AGENTS-ledger + report-pointer prose stay the verdict
   channel? This sweep produced the hash table that makes the gate
   buildable; only the policy call is missing.
2. **Canonical `suggestGenerics`:** your ad-hoc invocation carried
   `--suggest-generics`; the committed `scripts/art-dupl.json` does not
   set it (types erased from the hash → more cross-type structural
   groups surface). Pin it into the canonical config, or keep canonical
   conservative and treat `--suggest-generics` as a discovery-mode flag?
3. **`tq verdict` for interactive sweeps:** the convention says
   report-without-verdict = half-closeout. I skipped the verdict
   deliberately (interactive session; agent shells inherit the
   production `TQ_DB`, so a casual `tq verdict` writes to the live
   journal). Should interactive sweeps record a verdict through an
   explicit scratch `TQ_DB`, or is the verdict contract paid-turn-only?

WAITING FOR INSTRUCTIONS.
