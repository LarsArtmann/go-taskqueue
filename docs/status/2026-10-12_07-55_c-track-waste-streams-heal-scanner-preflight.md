# C-Track Waste Streams — Heal Script Cured, Scanner Sees Glued Footers, Done-Preflight Default ON (C12.4, B11, C12.2/3, C13)

Window: 2026-10-12 ~07:15–07:55 CEST. Resumed from the B10 WAIT under the
owner's execute-the-plan directive. Five plan rows shipped (C12.4, B11.1,
B11.2, C12.2, C12.3, C13.1–C13.3) with module gates green at every landing;
C15.1 opened with a diagnosis that re-scopes it (CV is blocked by a corrupt
shared module-cache entry, not the planned data/-dir defect). The C12.4 heal
fix was exercised for real three times this window — every daemon race that
would previously have destroyed attribution was recovered in minutes.

## a) DONE

1. **C12.4 — heal-daemon-sweep.sh: true fork records + session-scope ids**
   (commit `c4f03193`, docs `bf498abc`). Two defects root-caused and fixed
   red→green (self-test 15→18 checks):
   - Fork records were derived AFTER the msg-filter, when `base..HEAD`
     already lists post-rewrite SHAs and the backup chain is disjoint from
     HEAD — every pair printed as identity (rows 69/455; the 04-14/02-53
     false-old-side class). The old side is now captured before the rewrite
     and pinned (healed and preserved commits both asserted).
   - The id argument accepted hex only, so session-scope footers
     (`20261011-b10-alert-routing-pack`) made spans unhealable — the exact
     blocker that cost the B10 window attribution 5×. Slugs are now valid
     ids (charset `[0-9A-Za-z._-]`), self-tested.
   - TODO rows closed: distinct-SHA self-test pin, identity-pair diagnosis,
     post-rewrite-sha capture, root-cause unblock (file lines
     88/464/474/678) + CHANGELOG Fixed.
   - The B10 window's lost sweeps (88e93daf, 8005938e-class, cbd413e8) were
     verified PERMANENTLY unhealable (interleaved foreign-footer commits —
     the script correctly refuses; row 682 class) — recorded, not forced.
   - **Immediate payoff**: healed this window's own daemon races
     (`2e7facfe→dd0de99b`, `560b5739→03dd1970` + re-parent
     `dd0de99b→dd51d150`, `6e843bc5→d53e6f49`) onto
     `20261012-c-track-waste-streams` — three recoveries that would all
     have been losses yesterday.
2. **B11.1 — pre-breaker loop suspects dismissed** (queue-side, reasons
   recorded). Read-only claim-count query found the exact named class:
   `…126208a7` (477 claims) and `…9b399dd` (476 claims), both dead
   go-taskqueue/agent verify-deaths from the pre-breaker era; both
   dismissed via `tq dlq --dismiss` with futile-env reasons. The 169-claim
   CV task was already cancelled. Next tier (126/70/51-claim) surfaced but
   NOT dismissed — left as audit surface (owner question 3).
3. **B11.2 — autopsy-yield audit is now mechanical** (commit `ab0d9334`).
   One-off production read: 23 autopsies minted, 5 fixes landed, 11 wontfix
   (sweeper), 3 operator dismissals, 4 dead-again, 1 unrun. The query now
   lives in `tq audit --journal` (AutopsyYield block: fixes / wontfix /
   in-flight / dead-again / unrun) over `dlqfix.DedupPrefix` (new single
   source for the namespace). Pinned by TestAutopsyYieldClassification;
   dlqfix + cmd/tq module gates green.
4. **C12.2+C12.3 — GitLogScanner %B body fallback, quoted-footer guarantee
   kept** (scanner work = healed commits `03dd1970`+`dd51d150`, docs
   `929443e9`). One git-log invocation now carries SHA, subject, trailer
   values AND raw body; a commit whose footer git refuses to parse as a
   trailer (row-276 glue class: footer stranded above the attribution
   block — the 59%-invisible gap) is attributed from its column-0 body
   line. Exact-line matching keeps the guarantee: `> `-prefixed or indented
   quotes never attribute (unit-pinned + e2e-pinned). Real-git quirk
   handled: a non-empty trailer field makes %(trailers) end with a newline,
   so the record's body begins on the next `\x1f`-prefixed line
   (bodyStart parser state). Both characterization tripwires flipped to the
   new truth (demoted-footer and harness-shaped commits now VISIBLE; a
   quoted reply stays invisible). Executor/root/cmd-tq gates green. TODO
   rows 206/207 ticked + CHANGELOG Fixed.
5. **C13.1–C13.3 — re-dispatch dedup regression closed** (code = healed
   `d53e6f49`, docs `6d51e6e4`). Diagnosis split the class: row-32 was
   footer-less landings (attribution — cured by C12.2 and the daemon now
   footer-ing its own sweeps); row-34 was a task legitimately minted while
   its item was open, claimed 20 min after the landing with the claim-time
   done gate OFF by default. Fix: `--done-preflight` defaults ON (fail-open
   by design; `=false` stays the documented escape). Pinned by the timeline
   repro TestDonePreflightCatchesLandingAfterMint (mint open → land → claim
   caught; BOTH landing shapes: ticked row, footer-only commit) and
   TestAgentPoolDonePreflightDefaultOn. Harvest + cmd/tq gates green; TODO
   row 676 ticked with the diagnosis + CHANGELOG Changed.

## b) PARTIALLY DONE

1. **C15.1 (CV unblock)**: diagnosis only — see c). The planned fix (data/
   dir creation in verify path + dead-portal test fixture) was NOT started;
   the gate currently cannot even reach test execution.
2. FEATURES.md rows for this window's user-visible changes (autopsy-yield
   audit section, scanner fallback, done-preflight default) not yet added —
   queued in f).

## c) NOT STARTED

C15.2/C15.3, C16.1–C16.4, row-48 re-scope, docs pass (plan rows were
sequenced behind C15/C16; the owner's report directive arrived first).

## d) TOTALLY FUCKED UP

1. **I violated the edit→commit→battery ordering twice.** After C12.4 made
   healing cheap, I still ran FULL module gates before committing — the
   daemon swept the scanner work (split across TWO sweeps, `560b5739` +
   `2e7facfe`, because it caught a mid-edit state) and the C13 work
   (`6e843bc5`). All recovered, but the commit-at-focused-green rule
   exists precisely for this and I knew it.
2. **Mischaracterized `560b5739` as a foreign window's sweep** in my first
   heal ("excluding the foreign daemon commit below it") — it was MY OWN
   intermediate edit state. Caught by checking `git show --stat` and healed
   in a second pass, but the first heal's framing in the transcript was
   wrong. Verify exactly-mine BEFORE labeling a sweep foreign.
3. **First %B-fallback parser design was wrong** (global `\n\x1f` → `\x1f`
   normalization broke on multi-value trailer fields); caught by the unit
   fixtures, rewritten as the bodyStart state machine. A scratch-repo repro
   of the real git output BEFORE designing the parser would have shown the
   split-line shape immediately.
4. One python-anchor mismatch (guessed 2-tab indent where the file had 1)
   — should have `cat -A`'d the target block first. One edit-tool
   stale-modtime refusal (expected under concurrency; python fallback is
   sanctioned).

## e) WHAT WE SHOULD IMPROVE

1. When parsing git plumbing output, build the scratch-repo byte-level
   repro FIRST and design against actual output (the trailer-newline quirk
   was visible in 6 lines of `cat -A`).
2. A heal-script convenience: when the refusal reason is
   "different Task-Queue-ID footer" inside an otherwise exactly-mine span,
   print the segmented `--from` invocations it WOULD accept (row 30's UX —
   this window performed that dance manually, correctly, twice).
3. Shared `/mnt/buildcache/go-mod` is a recurring single-point failure
   (badRefContent class this window; the unlinkat clean-broken class
   05-30). A cheap `fsck` probe in doctor or a fetch retry that bypasses
   the corrupt entry would turn a dead verify gate into a self-heal.
4. FEATURES.md discipline: code+CHANGELOG+TODO landed this window but
   FEATURES rows were not — fold into the next docs pass rather than a
   separate commit (f-1).

## f) Up to 50 things we should get done next (full list; report-only)

1. C15.1: repair the corrupt vcs cache entry (owner question 2), re-run
   CV's gate, THEN land data/-dir + dead-portal fixture if still failing.
2. C15.2: dispatch one CV canary task; confirm gate green at base.
3. C15.3: SystemNix park policy while owner sessions active (needs C16.1).
4. C16.1: `tq park TASK_ID --until DUR` (row 389).
5. C16.2: bare-worker agent-claim refusal (row 390) + test.
6. C16.3: forensics filters — `tq audit --task-id`, `tq tasks --dedup`
   (row 475).
7. C16.4: ClaimCount GROUP BY pushdown (row 391).
8. Row-48: re-read the 01-41 api-pump poison-DLQ report; execute only the
   seam its mutation-verified wire pin left open.
9. Docs pass: mark major-prompting-and-batching EXECUTED with landed
   numbers; FEATURES rows for this window (b-2); fold smoke-hermeticism
   lessons into the next AGENTS prune (budget 19,970/20,000 — do NOT touch
   AGENTS without pruning).
10. Row 29: heal-script span-terminator (footer-bearing HEAD should end an
    exactly-mine span).
11. Row 30: bare-form refusal prints the exact `--from` invocation.
12. Row 454: heal-script contract-honesty bundle (inclusive --from mode,
    post-rewrite topology re-derivation, post-heal trailer-verify line).
13. Row 551: post-heal citation check (grep docs/status for rewritten SHAs).
14. Row 373: doctor derivation-blind census surface.
15. Row 374: one-shot historical derivation-blind census since v0.1.0.
16. C12.5: historical derivation-blind census (rows 354/355).
17. C14.1: verify-runner failing-stage output capture (row 196).
18. C14.2: enlarge FailureEvidence tail with FAIL+cause lines.
19. C14.3: `tq dlq --evidence-stage` + `tq show` render (row 374).
20. C14.4: audit --journal verify-deaths-by-stage + requeues drill-down
    (rows 630/409).
21. Row 83: commit-task.sh optional body arg + session-slug id support —
    my hand-rolled heredoc commits exist because of this gap.
22. B11.1 extension (owner question 3): dismiss the 126/70/51-claim tier.
23. Root `go test ./... -race` full pass at a quiet parent (convention for
    windows touching internal/executor + internal/harvest).
24. `scripts/ci-local.sh` full pass (heal self-test step now runs 18
    checks; flag-count pins elsewhere may need +1 for the new tests).
25. Full `scripts/root-gate.sh` run for the window's combined delta.
26. D19.2-class: lint-baseline check after this window's additions
    (autopsyYield + parser changes could add findings; advisory gate).
27. README/FEATURES: done-preflight default documented where the flag
    manifest lives (docs/planning/2026-10-10_agent-pool-flag-manifest.md
    tracks env vars only — add a defaults column or a note).
28. The B10 report's still-open §g questions (release timing, budget
    values, daemon commit policy) — fold into the owner ruling sheet.

## g) OWNER QUESTIONS (things I cannot figure out myself)

1. **Production pool restart timing**: the done-preflight default flip
   takes effect on the production agent-pool only after a restart
   (systemctl is owner-gated). Restart soon to activate the no-op burn
   protection, or wait for the next release/natural restart? (Folds in the
   B10 §g release-timing question.)
2. **Shared module-cache repair**: CV (and any consumer of
   templ-components pseudo-versions via /mnt/buildcache) is blocked by one
   corrupt vcs cache entry (`3af5bce8…`, five `badRefContent` tags,
   fsck-verified). May I trash that single cache directory so `go` re-
   fetches it, or is /mnt/buildcache strictly agent-hands-off?
3. **B11.1 scope**: extend the pre-breaker dismissals to the next
   loop-suspect tier (dead tasks with 126/70/51 claims — same era, same
   env-churn shape), or leave them visible in the audit as evidence?

## Fork records (this window's heals)

- `2e7facfe5ae65c9bdea98b7ec838ba7f69e8c7ed→dd0de99be3cbb9eeb41c8ab63bc3ffb54b9b5845`
  (re-parented to `dd51d15074cd8811019da5b6b629b2156d01cd1b` by the
  560b5739 heal)
- `560b573987c80a62bd7f9f8f8d1edc890e90fca1→03dd1970aa25037d39677834f2c05877bea69337`
- `6e843bc52a673267ee3722f8e0f13809041c1bd6→d53e6f4916018f6fbefc45b7ce0be9c0b825226e`

All attributed to `Task-Queue-ID: 20261012-c-track-waste-streams`.
