# Pareto seventh window — crush-drift cured, gate pins landed, owner digest minted

- **Date:** 2026-11-18 19:45 CET (window: ~16:45–19:45, same-day continuation of the 14-33 sixth-window report)
- **Repo:** go-taskqueue · master · standing dispatch re-issued after the sixth-window WAIT

Batch: backlog-pareto master-plan continuation
(docs/planning/2026-11-17_19-53_backlog-pareto-master-plan.md). Concurrent
agent session + the pma auto-commit daemon share the repo (daemon swept
mid-edit 4×, every sweep healed by exactly-mine verify + soft-reset fold).

## a) Fully done

1. **Pushed twice, both spans watched**: `027f2c10..2ab1f2fc` (the
   sixth-window backlog) and `2ab1f2fc..b5b40786` (drift cures + T31). The
   first span's CI run was cancelled by the newer push (normal supersession);
   run 44354296677 rode the second span.
2. **/tmp wall resolved WITHOUT action** (§g1 of the sixth report): the
   tmpfs freed itself (47 G available at 18:00, the foreign fixtures are
   gone). Nothing of another session's was deleted; the 482 G go-build cache
   on / remains a standing prune row.
3. **Crush-drift row (TODO 1712) CURED and ticked** (`3a3a266a`) — the
   memory-shape tolerant-assert remedy (G2), both halves verified LIVE:
   - route M7 (`183ec887`): the post-run health probe accepts the v0.101.0
     idle-exit (the server exits when its last client disconnects); the
     routed-run rc stays the respawn proof. Full route smoke green rc=0
     (receipt drift-route-cure-verify.log).
   - pool verdict probe (`b5b40786`): polls running-then-completed — the
     stall-free verdict path completes INSIDE one 0.3 s poll gap (kept-scratch
     DB receipt: created==completed in the same second; the drift was poll
     visibility, not a dead server). Full pool smoke green rc=0, all five
     legs incl. clean SIGTERM teardown (drift-pool-cure-verify.log).
   - `bf672a89`: TQ_SMOKE_KEEP forensics aid in the pool smoke cleanup
     (dep-sweep precedent).
4. **T30 owner-decision digest** (`5e0b0749`): ONE file,
   docs/planning/2026-11-18_18-04_owner-decision-digest.md — all 19
   UNBLOCKED-OWNER-GATED advisories consolidated into D1–D19 (options +
   agent defaults) plus the sixth-window §g statuses; the 19 rows now carry
   `— BLOCKED:` pointers to it; todo-gate advisories 19 → 0.
5. **T31 row 66** (`d43f3a6c`): `tq tasks --json` emits `[]` not `null` on
   empty match sets; strict RAW-BYTES pin over plain/envelope/parked/
   parked-class (a decode-into-slice assert cannot see null — pinned that
   explicitly). Full cmd/tq module gate green.
6. **T19 row 135** (`fe28503a`): scripts/lib/gate-fixture.sh — shared
   negative-fixture battery (tree/write/expect/summary + 7-check self-test)
   with a source-safe `BASH_SOURCE[0] = $0` guard (the naive `${1:-}` guard
   hijacked any gate invoked with --self-test — caught live and fixed);
   agents-size self-test refactored onto it byte-identically (caught a
   second bug: printf '%s' dropped the fixture's trailing newline — byte
   drift broke a pinned "over by 2 B" assert); crush-server + gate-fixture
   lib self-tests wired into ci-local (sixth-report residue item 26).
7. **T23 complete — 8 rows of the 48-row commit-msg section landed**:
   - f2 reason-pinned hook cases (`f8d60524`): run_case takes a guard-reason
     fragment; 7 rejection shapes pinned against LIVE hook stderr (never
     invented), so a reason drift trips at green rc.
   - f3 merged-message consumer pin (`2a3eb67a`): a merge-shaped commit is
     FOUND by the queue's own scan (`git log --grep` + `%(trailers)`) —
     16 cases now, summary names consumer-scan.
   - f5 wait_completed fast-fail (`a901e582`): timeout dumps non-fresh task
     rows (dead or attempts>0) + pool-log tails — verdict-class causes
     surface in run 1.
   - f13 heal refusal cap (`97e73973`): blocker list capped at 5 + "…N more"
     (the 29-blocker flood class); self-test 45 checks green.
   - f17 push-timing env hooks pinned in the header (`bade0adc`).
   - f20 heal usage() names the --from exclusive-base recipe + terminator
     walk (`bade0adc`).
   - f4 rows 394/928 citation RESOLVED (`0ed5f093`): historical row 928
     (2e444fbd era) is the windows-green confirmation row, NOT a ruling row
     — the 19-04 closeout carried a wrong-premise assumption; its report now
     carries an ADDENDUM (annotate-never-rewrite) and the sweep verdicts
     stand as content-driven.
   - f6 eleventh-dispatch battery row evidence-ticked (`0ed5f093`): the
     cited SHAs are HEAD ancestors and full batteries have since run over
     them (104 countable ok-lines receipt).
8. **T32 complete** (`b8f5fa07`): facade-parity is HOOK-enforced — the
   pre-commit refuses commits touching `^(task|journal|queue|executor|
   worker)/` when check-facade-parity.sh exits nonzero (0.2 s warm);
   ADR-0016's parity-BEFORE-staging left memory-enforcement. Guard-wiring
   orphans row evidence-ticked (0 orphaned at HEAD: sibling-sourcing rule +
   the session-start ritual assert). Also flipped 3 stale DONE-annotated
   checkboxes my own earlier tick scripts had left unchecked (the gate's
   DONE-ANNOTATION-UNCHECKED advisory caught them — the gate works).

## b) Partially done

1. **T02 green battery**: run #5 stopped at docs-gates FAIL freshness — the
   known cure is a date-named report at HEAD (this file). 104 countable
   ok-lines passed before the stop; the smoke tail was verified separately
   (§a3). The green-to-completion run is the next window's first act.
2. **CI watch (T36)**: run 44354296677 was in progress on the second pushed
   span at last check; the 10 commits since (T30→T32) are unpushed and will
   trigger the next run. The CI-reds chain rows stay unticked until a green
   run covers them.

## c) Not started

- T21 (SSE 58-row triage) / T22 (keep-set first 5 pins): the plan itself
  routes this cluster to the pool ("most are 1-test pins; batch-execute via
  pool"); cluster located (rows 419–1912 scattered + the 000001a19eac4ba6
  appends block), triage not yet written.
- T33/T34 done-prompt residue batches; T35 per-report pool dispatch wave.
- §f residue smalls: stub-first-line-shebang invariant, battery df preflight,
  fixture block-count floor, go-build prune ritual, scripted-doc-mutation
  rail (row 84), row 1567 amend-recipe-in-refusal, monthly digest ritual
  (index 418/400), accuracy pass over the 6 self-authored pins.
- Other-repo items: T24 erraudit sibling, T25/T26 go-cqrs-lite archive.
- Owner-gated: T15/T16 deploy (sudo), D2 release-wave push, row 24 host unit.

## d) Totally fucked up

1. **Daemon races ×4**: route-cure, T31 files, T19 trio, status-loop helper
   each got swept into footer-less commits mid-sequence; every one healed by
   exactly-mine `git show --stat` verify + soft-reset fold, but four
   round-trips is four too many — edits sat uncommitted while I verified.
2. **multiedit corruption on check-commit-msg-hook.sh**: 1 of 10 edits
   failed silently mid-batch, leaving an unterminated echo and a swallowed
   function tail (fixtures slid INSIDE run_case); two repair rounds, caught
   by bash -n. Root cause: I composed old_string from memory of the shape
   instead of pasting the exact viewed text.
3. **Wrong-file multiedit**: the push-timing header edit was aimed at
   heal-daemon-sweep.sh's file path (both edits in one call). Caught by the
   tool's no-match failure immediately.
4. **Lib self-test hijack**: the naive `${1:-} = --self-test` guard fired the
   lib's branch inside any gate invoked with --self-test. Should have been
   designed with the BASH_SOURCE guard from the first draft, not patched
   after the live catch.
5. **Fixture byte-drift**: gate_fixture_write's printf '%s' dropped the
   trailing newline my hand-rolled fixture had — "over by 1 B" vs pinned
   "over by 2 B". Byte-exact refactors must diff fixture BYTES, not just
   summary lines.
6. **False "ticks atomic" claim**: a commit message said ticks rode along
   when my tick script had actually failed (quoted "$L17" became a python
   string, not an int). The claim was corrected by an amend within the same
   unpushed window — but the message lied for one commit.
7. **Battery ordering**: running the battery before writing this report
   guaranteed a freshness stop — the report IS the freshness cure.

## e) What we should improve

1. Edit→commit gap is the daemon-race window: commit within the same tool
   call as the edit whenever the change is self-tested.
2. multiedit on function bodies: paste exact viewed text, never reconstruct;
   verify with bash -n BEFORE the next edit stacks on top.
3. Every refactored fixture: assert old-bytes == new-bytes (a byte hash in
   the self-test would have caught the newline loss mechanically).
4. Source-safe guards (BASH_SOURCE check) are the default for any sourced
   lib that also carries a --self-test branch.
5. Report-before-battery: the freshness gate makes the report a battery
   dependency, not an epilogue.
6. Tick scripts: flip the checkbox IN the same mutation as the DONE note
   (three rows drifted exactly because the append-only path skipped it).

## f) Next (ranked)

1. Re-run `./scripts/ci-local.sh` to green completion at this report's HEAD
   → capture `grep -c '^ok:'` + rc to /var/tmp/pareto-battery/t02 receipts;
   tick T02's plan state.
2. Push this 10-commit span when the timing window opens; watch CI green;
   tick the CI-reds chain rows (1668/1690) on a green origin run.
3. T22 keep-set: land the first 5 webui SSE pins (rows 1535 header-matrix
   hub-less shape, 602 header e2e, 693 aria-sort, 1534 probe shrink, 1783
   window-comment convention).
4. T21 triage doc: classify the webui SSE cluster rows into keep/pool/
   obsolete (dated planning note; the pool dispatches the 1-test-pin tail
   per the pareto plan — most cluster members are one-pin asks, only the
   keep-set lands by hand) — minted as a residue row this window.
5. T33/T34 residue batches (oldest done-prompt sections) — pool food per
   the plan.
6. T35 pool dispatch wave: mint the harvest items for the per-report residue
   tail (~200 rows) and let the pool pay for them.
7. Row 1567: heal terminator refusal should PRINT the amend recipe inline
   (today it is only in usage()).
8. Stub-first-line-shebang invariant: a guard pinning that generated stubs
   carry #!/bin/sh as the FIRST heredoc body line (the exec-format-error
   class, sixth report §e2).
9. ci-local df preflight: refuse the battery when /tmp free space is under
   a floor (ENOSPC wall class; sixth report §e4) — minted as a residue row
   this window.
10. Fixture block-count floor: check-fixture-attribution gains a minimum
    heredoc count (94→100 class, §e6).
11. go-build prune ritual row: the 482 G cache on / at 94 % is one
    ENOSPC away from the next wall.
12. Run the 2026-11 monthly ANNOTATE+ARCHIVE digest sweep over
    docs/status/README.md (index 419/400, advisory fired all window) and
    land its `digest 2026-11` row per the ratified ritual — minted as a
    residue row this window.
13. Row 84: the scripted-doc-mutation rail (today's incidents are the cited
    evidence; convention + guard sketch).
14. Accuracy pass over the 6 fixture pins I authored (the gate enforces
    existence, not accuracy).
15. check-todo-list DONE-ANNOTATION-UNCHECKED: promote the advisory to a
    hard error for rows a batch itself touched (mirror of D10).
16. session-start.sh --self-test (hook/pre-push probe output pins) + decide
    countable-gate status.
17. session-start.sh --compact mode (skip CONTRIBUTING/CLAUDE heads).
18. check-push-timing --self-test: HEALED fresh-ALLOWED case (today only the
    backdated commit pins the allowed shape).
19. Time-hook audit: grep gates reading date +%s against fixed fixtures
    without a time-env override.
20. install-pre-commit.sh: honor core.hooksPath (write into the RESOLVED
    hooks dir) so a host-global hooksPath pointing at a missing dir cannot
    silently disable every guard (the standing open row in the commit-msg
    section owns this).
21. session-start.sh: detect the dangling hooksPath shape specifically and
    print the repo-local override remedy.
22. Host-wide hooksPath audit across other repos on this machine.
23. watch-write-revert.sh: inotifywait primary mode (sub-100 ms attribution).
24. watch-write-revert.sh: persist captures via archive-evidence.sh.
25. Live-fire the write-revert canary during the next multi-agent window.
26. O14 decision rubric (two-axis test) into check-release-docs.sh output.
27. Provider/requeue_class join on --parked-class WITHOUT --parked.
28. Sweep test files for json.Unmarshal into REUSED slices (stale-omitempty
    class).
29. Split TestParkedJSONSurfacesProvider into table-driven subtests.
30. Pin the --parked-class --json-envelope total semantics explicitly.
31. Direct table test for latestRequeueEvidence (last-wins, unparsable→zero,
    empty→unknown).
32. jq one-liner smoke on raw parked --json bytes post-fold.
33. ratelimit-e2e.sh negative-path proof for the provider JSON grep.
34. Fold latestFailureEvidenceSummary into one pass (parity with the
    requeue fold).
35. End-to-end tq doctor smoke: parked-provider "+N more" capping.
36. Webui parked badge tooltip: provider + reason join in title attr.
37. Webui fragments render test: pin the `parked: [<provider>]` badge
    OUTPUT for a provider-carrying requeue fact (render-level;
    TestParkedRequeueEvidenceProvider only pins the decode) — the standing
    open row owns this.
38. audit the reviewfix done-preflight weak-anchor false-cure class (row
    …173ee1329).
39. journal-drift.sh unix-milli negative test (seconds-granularity seed
    fires the phase-6 pin).
40. AGENTS.md: fix the three fused line-break bullets in §Payload/§Operational
    contracts (2 B headroom — paired compress required).
41. Other-repo: T24 erraudit sibling (push 4 commits, close #12/#13/#14).
42. Other-repo: T25/T26 go-cqrs-lite archive ritual (triage 187 reports).
43. T15/T16 (owner-run sudo): SystemNix input flip + deploy + tq rebuild.
44. D2 (owner): push master + cmd/tq/v0.4.3 and proxy-verify the wave.
45. Row 73's owner push + `go install .../cmd/tq@v0.4.3` verify (same act as
    44).
46. D5 (owner or agent-with-ruling): AGENTS compress-only prune to ~24k +
    re-tighten both twins in one commit.
47. D8 (owner confirm): close the obsolete local/origin divergence row.
48. D9/D12 (owner rulings): retryx graduation record + RULING-BY-AGENT
    convention line.
49. Backfill/dismiss the OPEN-no-row items from the 2026-10-31 ANNOTATE pass
    (row 1269 cluster).
50. Sweep test files for the line-contained-grep hazard class (row 878:
    check-doc-refs, check-facade-parity pins).

## g) Questions for the owner

1. **D2 / row 73 (release wave)**: the `cmd/tq/v0.4.3` tag is cut locally
   and everything else landed — may I include the tag push in my normal
   timing-window push flow and run the proxy verify (`go install
   .../cmd/tq@v0.4.3`), or do release pushes stay owner-only?
2. **G3 ratification**: push-on->600 s timing-window was the standing
   behavior and I used it twice this window (both CI-watched) — ratify as
   standing policy, or do you want per-window push approval?
3. **G2 retro-ratification**: the crush-drift cure chose tolerant asserts
   over pinning the crush input version (smokes now tolerate v0.101.0's
   idle-exit/fast-completion while keeping their invariants) — ratify, or
   should smoke hosts pin a crush version for reproducibility instead?
