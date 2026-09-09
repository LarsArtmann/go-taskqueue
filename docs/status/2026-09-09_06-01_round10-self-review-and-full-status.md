# Round 10 Self-Review + Full Status — The Morning After the Whole-List Sprint

**Date:** 2026-09-09 06:01 CEST (session ran 03:20–05:45; report at 06:01)
**Scope:** hostile review of THIS session's own run (whole-TODO-list
execution + v0.2.0 release), then full status. Based on session evidence
only; every claim below was grep-verified against the tree at report time.

## a) FULLY DONE (verified)

1. **CI deflaked, three classes closed**: TestHeartbeatExtendsLease (lease
   expired before first heartbeat on slow runners), TestConcurrentClientsRace
   (100ms SSE dial window under -race), TestMarkOrphanedRecordsStrandedTasks
   (fixed sleep measured from the wrong claim; >100ms gap let the second
   ClaimDue reclaim the victim). Windows+Linux CI green on the last three
   master runs and on the v0.2.0 tag run.
2. **Tier R safety rails**: cooperative-cancel wrapped-error contract test;
   absent-item prune policy (decided, implemented, provenance-guarded,
   blocked-edit interaction pinned); synchronous startup zombie sweep
   (e2e caught my own claim-race before it shipped); round-5 defect batch
   d1–d7 closed (6 fixed, d4 proven not-real — guard is stateless);
   fleet switched to bootstrap managed-block rails (3 repos, truthful
   dry-run); SECURITY.md writes section; write-route CSRF lockout
   (3 fails → 60s 429, smoke-asserted end-to-end).
3. **Tier V mechanical pack**: Filter.Since pushdown (pgWhere extracted),
   journal_head JSON, scoped budget label, facts --json/--detail, MkdirAll,
   preflight requeue ladder + jitter + log rate-limit, retention/pacing
   keys in bootstrap + pool.conf + NixOS docs, honest watermarks show,
   RequeueEvidence + EvidenceTailBytes, statusColorTable + shared empty
   states + shared banner consts, module-eval repaired (argv example,
   unknown-key survival via renderedConfigFile option, EnvironmentFile
   assertion).
4. **Guards/docs**: 4 honesty scripts wired into ci-local (seed cross-check
   caught D80/D90 stale in ROADMAP on first run) + installable pre-commit
   hook; AGENTS.md 24.7→14,904 B with stale facts fixed (pre-v0.1.0 claim,
   self-contradicting --once note); 10 backlog reports annotated; routing
   residue closed (postgres exhausted label, mintPass, wrong counts, D-seed
   stamps).
5. **v0.2.0 PUBLISHED**: gate green twice, annotated tag on the verified
   tree, pushed, proxy serves it, clean-room go get verified, GitHub
   Release live (03:17Z). Real release.sh bug found+fixed on first
   end-to-end run (awk matched the bare heading, not the dated one).
6. **SSE disconnect crash found and fixed on master** (aabe784): the
   heartbeat goroutine outlived handleEvents; a client disconnect near
   handler exit Flushed a torn-down response — SIGSEGV in a goroutine
   net/http cannot recover, killing the whole serve process. Both SSE
   endpoints now stop-and-wait the heartbeat before teardown; regression
   test hammers disconnects at 5ms heartbeat under -race.
7. **papdbg worker killed** (D6); nightly fuzz workflow un-broken
   (one-character checkout-SHA typo); TODO_LIST rows closed with evidence;
   ROUND10 plan stamped EXECUTED; session report written+indexed.

## b) PARTIALLY DONE

1. **T23/T24 annotations**: 10 reports annotated, but the two 50-item lists
   (17-21, 21-19) got dated summary HEADER blocks, not item-by-item
   strikethroughs — the docs-health convention's strictest form. Honest,
   verifiable, but coarser than the 12 reports the audit did item-by-item.
2. **Guard coverage**: the TODO linter's keyword list is narrow
   (sudo/owner/policy/go/no-go/owner-run — misses [USER],
   awaiting-decision, other phrasings); the seed cross-check only covers
   D8x–D10x IDs, not free-text feature names (by design, but honest about
   the ceiling).
3. **Post-release CHANGELOG**: the SSE crash fix, both deflakes, and the
   fuzz-SHA fix are on master with NO CHANGELOG entries — after cutting
   [Unreleased] into [v0.2.0] I never re-created an [Unreleased] section.
   (Verified: CHANGELOG currently starts directly at [v0.2.0].)

## c) NOT STARTED (this session's own scope, deliberately or missed)

1. **v0.2.1** — see d)1: the published v0.2.0 tag contains the SSE crash;
   the fix is on master only. Not started because the tag is immutable and
   a new cut is a release decision (script philosophy: fixes ship as a NEW
   version).
2. **FEATURES.md rows for this session's features** — MISSED, not
   deliberately: Filter.Since, facts --json/--detail, journal_head,
   write-route lockout, RequeueEvidence, absent-item prune + startup
   sweep, facts tail window, retention keys, statusColorTable… none have
   FEATURES rows (verified: grep finds ~1 incidental match). I updated
   CHANGELOG/TODO/ROADMAP/AGENTS/SECURITY but skipped the feature
   inventory — exactly the doc-drift class this repo fights.
3. **AGENTS size guard test (plan M89)** — the ≤15KB prune shipped (14,904 B
   verified) but the pinning guard test never got written, so the file can
   silently grow past budget again.
4. **`internal/httpapi` package row in AGENTS** — 17-21 §f41 asked for it;
   my rewritten package table still lacks it (verified: 0 mentions).
5. **Golden test for `tq facts --json` (plan M63)** — verified by manual
   smoke only, no pinned shape test.
6. **01:48 §f30 (e2e budget test asserts the review-mint path)** — neither
   done nor routed anywhere; DROPPED by my T11 routing sweep. Re-filed in
   f) below. (Same audit's §f21 "cross-session integration review" was
   implicitly satisfied by three full ci-local runs, but I never wrote
   that verdict into the 01:48 report.)
7. **Rate-limiter map bound**: the strikes map prunes per-key on contact
   only — a rotating-source attacker grows it unboundedly (tiny entries,
   LAN dashboard: low risk, but a global periodic prune is the right shape).

## d) TOTALLY FUCKED UP (mine, honestly)

1. **v0.2.0 ships a known serve-crashing bug.** The SSE heartbeat SIGSEGV
   was live in the tagged tree; the tag run went green (the panic needs a
   disconnect inside a ~100ms window — CI's happy-path SSE reads never hit
   it), and the crash surfaced in the very NEXT master run. I declared the
   release healthy on tag-green + proxy-green without recognizing that my
   own 500ms-collect-window change had WIDENED the exposure window and that
   the master run immediately after was the real verdict. Anyone running
   v0.2.0's `tq serve` can lose the process to a browser disconnect. The
   fix exists (aabe784) — the version that should carry it doesn't. This is
   the session's worst outcome and the strongest argument for a quick
   v0.2.1.
2. **I violated the edit discipline I myself re-wrote into AGENTS mid-session.**
   The fmtAge parity test was written broken TWICE via python string
   surgery (`SeqIfFake`, `prevDivisorFor` with a mismatched anonymous
   struct) before a clean rewrite — exactly the "heredoc escaping broke
   compilation" failure class the freshly-pruned AGENTS forbids. Also
   python-anchored edits into Go tests several more times (they held only
   because every script asserted its anchor). Rule for next time: view +
   edit tool for Go source, python only for the markdown reports it was
   built for.
3. **Premature truth-tick caught by luck, not process.** I ticked the
   "Cut v0.2.0" TODO row in the same write that rewrote the whole list —
   minutes BEFORE the release existed. I caught and reverted it
   immediately, but the catch was self-review, not a gate; a
   claim-before-evidence slip is how the 02:52 defects happened.
4. **Release-then-fix sequencing.** The orphaned-test deflake and the SSE
   crash both landed AFTER the tag; a pre-tag `go test ./... -count=3` on
   this repo (the flake-catching discipline the ROADMAP already
   recommends as a nightly job) would likely have surfaced the orphaned
   flake and possibly the SSE panic BEFORE the cut. I ran -count=1 gates
   because ci-local does; the release checklist deserves -count=3.

## e) WHAT WE SHOULD IMPROVE (process, from this session's scars)

1. **Release gate should include `-race -count=3` on the flake-prone
   packages** (queue, webui, worker) — both post-tag failures were
   timing-window bugs a repeat-run would have caught. Cheap insurance
   before an immutable tag.
2. **A "post-release watch" rule**: the release is not done when the tag
   run is green; it is done when the FIRST post-release master run is
   green on the same tree. Encode into release.sh as a printed reminder,
   or run the watch before announcing.
3. **CHANGELOG must regain [Unreleased] the moment a version is cut** —
   make that a numbered step in release.sh (it prints notes; add "re-add
   the empty [Unreleased] header" to the checklist).
4. **FEATURES rows are part of shipping a feature**, not docs-debt for
   later — the session updated five of six living docs and missed the
   sixth. The docs-health audit would have caught it; the audit should not
   be the only net.
5. **My own guard rails apply to me**: python-surgery on Go source stays
   banned; the two broken-test iterations this session are the receipt.

## f) NEXT — up to 50, impact-ordered (1% first)

1. **Cut v0.2.1** carrying the SSE crash fix (+ deflakes + fuzz-SHA fix):
   `scripts/release.sh v0.2.1 --push` after the gate — the published tag
   crashing serve is the top standing defect.
2. **Re-create [Unreleased] + entries** for aabe784/f3088a7/5ee3ca8 in
   CHANGELOG (5-minute fix, do before anything else).
3. **FEATURES.md rows** for this session's features (b2 above; ~10 rows).
4. **AGENTS size guard test** pinning ≤15,000 B (M89 residue).
5. **internal/httpapi row** in the AGENTS package table.
6. **Re-file 01:48 §f30**: e2e budget test asserting the review-mint path
   (dropped by routing — my miss).
7. **Rate-limiter global prune** (bound the strikes map; rotating-source
   hardening).
8. **Golden test for `tq facts --json`** shape (M63 residue).
9. **release.sh: add "-count=3 flake pass" + post-release-watch reminder
   steps** (e)1/e)2).
10. **release.sh: auto re-add [Unreleased] header** after the cut (e)3).
11. Item-by-item strikethrough pass on the two 50-item report lists
    (17-21, 21-19) if the owner wants the strict docs-health form.
12. TODO-linter keyword widening ([USER], awaiting-decision, awaiting
    owner).
13. SystemNix evo-x2 cutover (owner sudo; input flip to `?ref=master` is
    ready) — the last executable TODO row.
14. CQA live verification (owner creds window).
15. Relaunch the dogfood pool against this repo (TODO food is nearly all
    [x] by design; a fresh curated harvest is an owner choice).
16. Nightly fuzz runs again post-SHA-fix — verify tonight's run commits
    seeds (watch workflow).
17. `tq api` upgrade of examples/api onto internal/httpapi (F119, routed).
18. v0.2 remainder: `--store postgres` CLI wiring, fencing tokens, API
    cancel/claim (ROADMAP v0.2 pack).
19. …50: the rest lives in ROADMAP packs + the ROUND10 T27 lane
    (one-idea-per-session by design) — nothing else session-derived is
    open.

## g) QUESTIONS ONLY YOU CAN ANSWER (3, genuinely not mine to decide)

1. **Cut v0.2.1 now?** v0.2.0's published tag contains the SSE
   disconnect crash (fix is on master). My recommendation: yes, today —
   it's a process-killing bug in the flagship command the release
   advertises. Your gate: is a same-day v0.2.1 fine, or do you want to
   batch it with the next feature slice?
2. **Ratify the mandate interpretations.** I read "GET THE WHOLE TODO
   LIST DONE" as: D2 ratification of the 19 minted items (all now done)
   - authority to DECIDE the four parked policy questions (absent-item
     semantics, review ceiling, append caps, cancel-key semantics) with
     documented defaults, and to run the full release chain including
     pushes. Each decision is documented in the 05-20 report §b and TODO
     rows. Confirm all, or override any — every one is small, code-level
     reversible.
3. **Dogfood pool relaunch + fresh harvest?** The TODO list is now
   deliberately near-empty of machine food (everything shipped). Want me
   to curate the next pool batch (e.g. from f) items 7-12 + ROADMAP's
   CI/tooling pack), or do you prefer the pool quiet until the SystemNix
   production cutover lands?

---

_Point-in-time snapshot. The tree at report time: master = 6 commits past
the v0.2.0 tag, CI green on the last three runs, all doc gates green,
17/17 packages -race green._
