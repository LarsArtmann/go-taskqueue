# Done-prompt status report, 2026-09-17: six-task CI-resilience + observability window + docs-health pass

- **Date**: 2026-09-17 00:25 CEST
- **Window**: 2026-09-16 ~01:00 → 2026-09-17 00:25 CEST
- **Tasks**: 000001a0a7682e60 (ci-local transient-retry wrapper), 000001a0a76ccb60 (review fix: depbump tails bypassed redaction), 000001a0a76cd080 (review fix: SecretHits double-counted overlapping spans), 000001a0a7b1684f (durable self-test pin for the retry loop), 000001a0a7c3b5f4 (rate-limit/observability batch, row 90), 000001a0a7faa45c (ci.yml concurrency + check-ci wording + go-mod verify retry, row 91)
- **Source documents**: each task's close-out report (read first, never re-derived), verified against HEAD by spot-checks: all six footer commits exist (`3e0f941`, `ac54a38`, `4ba81d8`, `025cd6b`, `2d0997e`, `d4125d1`); CHANGELOG entries for the pin, the redaction pass (+depbump delegation, +span-merge semantics), and the ratelimit-e2e second-claim pin are present; TODO rows 88/90/91 are `[x]` with evidence.
- **Written by**: the done-prompt status task (000001a0ac3c677a7c5de3e8d2ffe1b39806), 2026-09-17 00:25.

## a) FULLY DONE (verified)

1. **ci-local transient-retry wrapper** (`with_transient_retry` in `scripts/ci-local.sh`): 11 wrapped tree-reading Go gates, 45s×3 poll budget (env-overridable), hard-fail with "concurrent edit in flight" context; nix steps deliberately unwrapped (documented in-script). Pinned by its close-out's cited gate battery. Row 88 `[x]`.
2. **Depbump tail redaction** (`ac54a38`): `tailOutput` (internal/executor/depbump.go) applies `redactOutput` before the 12-line cut — the LAST tail bypass, so the "every output tail goes through the redaction pass" claim is now true. Pinned by `TestDepBumpTailOutputRedactsSecrets`. The same lap repaired a five-row lint-baseline drift (gate exit 0, 1084 vs 1097).
3. **SecretHits span-merge fix** (`4ba81d8`): `tq audit --journal` hit-counts now count distinct secret LOCATIONS (overlapping pattern spans merge — one `Authorization: Bearer <token>` line is one hit, was two). Pinned by `TestSecretHitsMergesOverlappingPatternSpans`; redaction output untouched; AGENTS.md + CHANGELOG reconcile the semantics.
4. **Durable self-test pin** (`scripts/check-transient-retry.sh`, `025cd6b`): sed-extracts the SHIPPED helper (marker-guarded), 8 sub-second assertions over defaults/heal/exhaust/escape-hatch semantics, wired as a ci-local step BEFORE the Go gates, negative-proven on broken /tmp copies. Replaces the 01-46 throwaway test.
5. **Rate-limit/observability batch (row 90)**: all 13 sub-items verified shipped on HEAD — 12 by concurrent windows plus the ratelimit-e2e second-claim assertion (f37) shipped this lap, which also fixed a fixture-decay bug (frozen past reset date had silently degraded the smoke to the 15-min fallback since ~09-12). Row 90 `[x]`.
6. **CI burst-cancellation + honest check-ci + verify retry (row 91, `d4125d1`)**: `ci.yml` concurrency group with cancel-in-progress (actionlint-clean); `check-ci.sh` compares the red run's headSha against local HEAD and says "red predates your tree" when true (exercised LIVE against the red run f7fabfe); `check-go-mods.sh` retries `go mod verify` once with an observable WARN. BONUS root-cause repair: six zero-dep leaf modules had drifted to `go 1.26` vs root `1.26.7` (the documented `go mod tidy`-on-a-leaf class) — restored, `check-go-mods.sh` green. Row 91 `[x]`.
7. **Delivery of the red-master repair**: at window start origin/master was red (run f7fabfe) with the repair unpushed (55 ahead); a push landed 2026-09-16 18:15 (origin at `d5dd92a`) and the repo is now only 7 ahead — the 04-21 §g1 push-standoff resolved without owner intervention.

## b) PARTIALLY DONE

1. **Row 89 smoke half**: extending `with_transient_retry` to the smokes is still BLOCKED on the owner runtime-budget ruling (third window asking; recommendation on record: two-phase tree-quiet probe). The pin half is done.
2. **The pin pins the helper, not its integration**: no call-site census — a future ci-local edit that stops CALLING the wrapper loses retry coverage with the pin green. Also ci-local-only: CI runners never execute the pin (hermetic, sub-second — cheap to add).
3. **45s×3 is report-inherited, never measured**: zero datapoints on real foreign-edit window durations; the wrapper has never healed an actual break (no soak).
4. **f41 retry is mitigated, not proven**: no durable pin for check-go-mods' retry loop; underlying stderr swallowed on retry failure.
5. **Redaction audit count breadth**: only the bearer×auth-header overlapping pair is pinned; a future pattern could reintroduce the class silently. No redaction e2e smoke yet (row 195).
6. **CHANGELOG gap (fixed in this pass)**: the wrapper itself, the ci.yml concurrency group, the check-ci predates wording, and the go-mod verify retry had NO entries (only the pin and row-90 items did) — an Added bullet is appended with this report.

## c) NOT STARTED (backlog the window skipped, still open)

- Mint-time done-check for repeat dispatches (the loop's standing top fix; the 12-11 and 00-xx re-dispatch pattern continues — a verify-only re-dispatch of the status-append task closed at 00:24 today, an entire paid lap for a verify).
- Row 93 (dead `factLines` + DOMAIN_LANGUAGE terms), row 195 (redaction e2e smoke), row 201 (secretPatterns/SecretHits table pin), row 206 (wrapper polish bundle), the httpapi live-socket smoke, the archive/ANNOTATE sweep (live index ~143 rows vs threshold 100), deploy-readiness bundle (all BLOCKED rows unchanged).
- fuzz.yml concurrency decision (deliberately scoped out of row 91, never ruled).

## d) TOTALLY FUCKED UP

Nothing in this window shipped broken — every task closed green with cited gates. The honest disclosures from the close-outs, aggregated:

1. **The daemon-fold attribution wart is now FOUR instances** (3e0f941, 025cd6b, 9648a19, d4125d1): work content rides footer-less `chore:` daemon commits; the footer lands on a docs/marker commit. Forensics work only because messages cite the daemon SHAs — prose, not machine-readable. Protocol question re-asked five times, never answered (§g2).
2. **Two self-inflicted verification lies in the 02-37 lap** (caught and repaired in-lap, but both are the exact trap class row 193 documents): a `--new-from-rev` lint run quoted as clean when the daemon had already folded the diff (empty diff = scanned nothing), and an exit code captured after `| head`. Also one blind amend of a daemon commit (content-exact, verified after — one concurrent sweep away from folding a stranger's file).
3. **Turn-1 ritual misses recurred in every lap** (grep-prior-reports late in 01-46/03-04, CONTRIBUTING.md read late in 04-21 — the documented N+1-th recurrences). Benign every time; the ritual exists for the time it isn't.
4. **Six leaf go.mods drifted to `go 1.26` onto master via a daemon commit** (f7fabfe) — the known `go mod tidy`-on-a-leaf class reached origin because daemon commits bypass hooks and pushes don't run ci-local. Repaired same window; the structural fix (push-side gate) is still open.
5. **A fixture silently rotted for ~4 days**: ratelimit-e2e's frozen past reset date degraded the smoke to the fallback path while staying green — the frozen-date class (sweep filed).

## e) WHAT WE SHOULD IMPROVE

1. **Commit content FIRST, docs second**: with a daemon this aggressive, the first completed edit should get an owned footer-commit immediately; docs/TODO edits last. Would end the attribution wart unilaterally.
2. **A pin is not done until shown to FAIL** — the 03-04 lap's negative-testing standard should be universal (check-* scripts AND smokes).
3. **Never quote a diff-scoped or piped gate as green** without confirming the diff is non-empty and yours, and capturing the exit code pipe-free.
4. **Every silent retry/absorption gets a WARN** (done for check-go-mods; audit the rest of scripts/).
5. **Turn-1 ritual must be mechanical** (template first call), not aspirational — six+ named recurrences.
6. **Behavior changes land their CHANGELOG entry in the same lap** — this pass caught four CI-facing changes with none.
7. **Time-based fixtures compute timestamps at run time** — frozen dates rot silently (the ratelimit-e2e lesson, generalizable to a sweep + small gate).

## f) UP TO 50 NEXT THINGS (honest ~25; appended to TODO_LIST this pass)

Wrapper/retry orbit: calibration of 45s×3 from git-history evidence; first real ci-local soak recording poll-hit rate; call-site census pin; pin-in-ci.yml parity; split row 89 at next harvest; check-go-mods retry pin + stderr in FAIL line; silent-retry WARN audit; GNU-date portability guard in ratelimit-e2e; ratelimit-e2e into ci.yml; smoke stub-log assertion for the provider line; frozen-date fixture sweep (+small gate if ruled).
Redaction orbit: pairwise-overlap pin; N-fake-tokens property test; RedactSecrets single-marker output pin; structural third-tail-helper guard; `journal-drift.sh` re-run at HEAD pre-release; hits→locations wording (owner call); annotate pre-fix SECRET EVIDENCE counts in old reports; SetFailureEvidence for depbump.
CI/orbit: check-ci.sh `--jq` consolidation + local-sha print + ancestry-precise predates; CONTRIBUTING.md individual-gates sweep; mint-time done-check; provider tag in the parked surface; doctor not-today branch pin; parked×project filter pin.
Full list with citations in the TODO_LIST append below.

## g) QUESTIONS ONLY THE OWNER CAN DECIDE

1. **Smoke-retry runtime budget** (row 89's blocked half, third ask): whole-smoke re-run ×3, two-phase tree-quiet probe (recommended), or wrap-only-cheap-smokes?
2. **Daemon-race attribution protocol** (fifth recorded instance): ratify the daemon-SHA-citation fallback as permanent, or bless content-first explicit commits (work commit BEFORE any docs/TODO edits) so the footer carries the pinned bytes?
3. **fuzz.yml concurrency semantics**: should the scheduled nightly fuzz campaign opt OUT of cancel-in-progress (a manual push would otherwise kill a campaign mid-write)? "ci.yml only" was scope discipline, not a ruling.

## h) BAND DRIFT

`task.reprioritized` facts in the journal for the window's timespan: **none recorded** (0 of 3,938 facts are reprioritized events). No marker/AI/unblock/importance moves to account for this window; stored-priority claims proceeded on the enqueue bands alone (ADR-0015 aging applies at claim time but never mutates stored values, so nothing to report there either).

---

**Gates this report rests on**: commit existence verified for all six footer SHAs; TODO rows 88/90/91 `[x]` with evidence; CHANGELOG entries verified present for pin/redaction/ratelimit-e2e; `git status -sb` push-state verified (origin at d5dd92a, 7 ahead); journal scanned for reprioritized facts (0). Nothing pushed by this task.
