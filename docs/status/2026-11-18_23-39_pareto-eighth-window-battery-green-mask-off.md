# Pareto eighth window — the battery finally went GREEN, and the mask came off

- **Date:** 2026-11-18 23:39 CET (window: ~19:45–23:39, same-day continuation of the 19-45 seventh-window report)
- **Repo:** go-taskqueue · master · owner demand: full status NOW, then WAIT

Batch: backlog-pareto master-plan continuation
(docs/planning/2026-11-17_19-53_backlog-pareto-master-plan.md). Standing
dispatch re-issued after the seventh-window report; the daemon raced mid-edit
3 more times (all healed by exactly-mine verify + soft-reset fold).

## a) Fully done

1. **T02 GREEN BATTERY — rc=0, 330 countable ok-lines, "ALL CI GATES GREEN"**
   (receipts /var/tmp/pareto-battery/t02-cilocal-green.{log,rc}). The
   unmasking chain took seven runs: 104 → 194 → 254 → 280 → 304 → 330
   ok-lines, each peel a REAL fix below. The last run's exact tree is what
   CI sees.
2. **Freshness harvest cure**: the docs-gates freshness gate harvests every
   §f item of the newest report — 5 items needed harvest; minted 3 residue
   rows (T21 triage doc, ci-local df preflight, the 2026-11 monthly
   ANNOTATE+ARCHIVE digest sweep) and reworded 2 items to carry their
   standing rows' distinctive tokens (d20692e3).
3. **Status-loop exhaust line** (879cd4ad): wait_completed now prints its
   own FAIL at poll exhaust — the smoke-coverage audit flags poll loops
   without an in-block failure message (silent-retry class).
4. **gosec v2.30.0 pin + G112 root-cause fix** (d2df0b1f): the system
   profile moved gosec 2.29→2.30; the gate refuses mismatches, so the pin
   in scripts/lib/tool-pins.sh follows (crush-drift precedent). The new
   version flagged ONE new class — G112 (Slowloris, missing
   ReadHeaderTimeout) in internal/bridge/papdashboard/healthrelay.go:170 —
   fixed at root cause (10s ReadHeaderTimeout), NOT excluded. Gate 25/25.
5. **Lint baseline regenerated for golangci-lint 2.15.0** (d58eb2d5): the
   profile bump changed finding shapes; regen under a FRESH
   GOLANGCI_LINT_CACHE dir (the shared ~/.cache/golangci-lint is held busy
   by the fleet's five live langserver LSPs — clean-cache could not remove
   it); 196 rows / 2498 findings; --check verified green under BOTH caches.
6. **REAL BUG: dead-sha shrink never twinned** (d17793dd) — the 2026-11-03
   batching edit wrote the twins map under UNSPACED keys (`tree|subj`)
   while every lookup reads the SPACED form (`tree | subj`): shrink always
   reported "0 of N now-reachable twins". The self-test leg pinned it since
   11-03 but sat behind the battery mask (origin CI never runs ci-local).
   One-line key fix; the full self-test battery (drop/keep/--write,
   collision body tiebreak, daemon tree+subject path) now green.
7. **Live dead-sha cites cured** (a6cecbca): with shrink working, the live
   pass flagged heal-orphaned cites — 5 provable pairs rewritten as
   old→new fork records across 4 pareto reports + the README index;
   3 daemon-sweep tokens (ambiguous heuristic subjects, no provable
   counterpart) baselined with a dated reason. Gate: 1778 baselined, 0 new.
8. **Cite-drift repoints** (c140bbe0): the webui-design-pass cite repointed
   to its archived/ path; the foreign go-health checks.go citation
   de-pathed (the matcher greps raw slash-paths — foreign files now name
   their repo in prose).
9. **52 pushed daemon sweeps grandfathered** (3e7d5f85): the
   daemon-sweep-attribution gate (a fresh leg behind the mask) flagged 52
   unattributed-shipping sweeps from the 2026-11-12..18 multi-agent window
   — all PUSHED, so history healing is barred; the gate's own
   deliberate-grandfathering remedy applied with a dated reason. Audit now
   1701 baselined, 0 new.
10. **T32 completed** (b8f5fa07, from the seventh window's tail): facade
    parity is HOOK-enforced (pre-commit refuses facade-dir commits on
    broken parity); guard-wiring orphans evidence-ticked (0 orphaned at
    HEAD); 3 stale DONE-annotated checkboxes flipped (the gate's advisory
    caught my own earlier append-without-flip ticks).
11. **T23 completed — all 8 rows** of the commit-msg section (f2
    reason-pins f8d60524, f3 consumer-scan 2a3eb67a, f5 fast-fail a901e582,
    f13 cap-5 97e73973, f17+f20 doc pins bade0adc→00023057, f4 the 394/928
    resolution + closeout addendum 0ed5f093, f6 evidence-tick).
12. **Pushes + CI green**: six pushes this window (b5b40786, 55cc0ef2,
    d20692e3, 8232caa2, d58eb2d5 chain, 3e7d5f85); origin runs 44354296677,
    44374311735, 44406624797, 44409265095 all SUCCESS — the CI-reds chain
    row 1924 evidence-ticked (the owner-only-push premise is stale under
    the timing-window policy).

## b) Partially done

1. Nothing material: the battery's own chain is COMPLETE (green at HEAD).
   The T22 keep-set pins and T21 triage doc remain as P2 tail (§c) — the
   battery green was the plan's stated precondition for them ("ticks land on a
   green battery"), so their ground is now clean.

## c) Not started

- T21 SSE 58-row triage doc + T22 first-5 keep-set pins (minted rows exist:
  the residue block carries both asks).
- T33/T34 done-prompt residue batches; T35 pool dispatch wave.
- §f residue smalls (rows minted + standing): df preflight row, digest
  sweep row, row 1567 amend-recipe-in-refusal, shebang invariant, fixture
  floor, go-build prune (482 G on /), accuracy pass over my 6 pins.
- Other-repo: T24 erraudit sibling, T25/T26 go-cqrs-lite archive.
- Owner-gated: T15/T16 deploy (sudo), D2 release-wave push, D1–D19 rulings.

## d) Totally fucked up

1. **Daemon races ×3 more** (status-loop tick, gosec trio, dead-sha line):
   every sweep healed by exactly-mine verify + soft-reset fold, but the
   edit→commit gap keeps costing round-trips; the discipline holds only
   when I commit in the SAME tool call as the edit.
2. **The "ticks atomic" false claim** (seventh window, one commit): a tick
   script failed silently (string-vs-int bug in a shell-interpolated python
   literal) while the commit message claimed the ticks rode along — fixed
   by amend inside the unpushed window, but the message lied for one
   commit. The root cause: my tick helper builds python code via shell
   interpolation; the quoted "$L" made line numbers strings.
3. **Battery ordering (seventh window)**: ran the battery BEFORE writing
   its own freshness dependency (the report) — a guaranteed stop.
4. Nothing new this window beyond the above: the six-peel chain was the
   SYSTEM working (each stop a real latent defect), not operator error.

## e) What we should improve

1. The unmasking chain is now EMPTY at HEAD — keep it empty by running the
   battery to completion at every window's END (after the report lands),
   not before.
2. Assoc-array keys in bash: normalize key construction and lookups to ONE
   literal form (the shrink bug was exactly a spaced-vs-unspaced drift
   between write and read). A tiny style pin in the gate-header conventions
   would prevent the class.
3. Tick helpers: pass line numbers as real ints (heredoc-quoted python +
   shell variables mix dangerously); or move ticks to a single committed
   tool with --self-test.
4. The daemon-race fold recipe is now practiced 7× — consider promoting it
   from the handoff prose to an AGENTS.md line (2 B headroom: paired
   compress required).
5. When a toolchain bump (gosec/golangci/git) lands via the system profile,
   run a battery EARLY: the profile changed THREE gates this window; early
   detection would have collapsed the chain into one pass.

## f) Next (ranked)

1. T22: land the first 5 webui SSE keep-set pins (rows 1535, 602, 693,
   1534, 1783) — the ground is green now.
2. T21: write the SSE cluster triage doc (keep/pool/obsolete; minted row).
3. T33/T34: the two oldest done-prompt residue batches (pool food) —
    minted as a residue row this window.
4. T35: mint the per-report residue harvest wave (~200 rows via the pool —
   "the system working" per the plan).
5. Run the monthly ANNOTATE+ARCHIVE digest (minted row; index 419/400).
6. ci-local df preflight row (minted; /tmp free-space floor).
7. Row 1567: amend recipe printed INSIDE the terminator refusal output.
8. Stub-first-line-shebang invariant (sixth report §e2 class) — minted
    as the owed residue row this window.
9. Fixture block-count floor for check-fixture-attribution.
10. go-build prune ritual row (482 G cache on a 94 % disk).
11. Accuracy pass over the 6 fixture pins I authored (existence is gated,
    accuracy is not).
12. session-start.sh --self-test + countable-gate decision (row 1729 area).
13. session-start.sh --compact mode (skip CONTRIBUTING/CLAUDE heads and
    index tail) — the standing open row owns this.
14. check-push-timing --self-test: the HEALED (footer-bearing) fresh
    commit asserting the ALLOWED verdict pin — standing row owns this.
15. Time-hook audit across check-*/smoke gates (date +%s vs fixtures).
16. install-pre-commit.sh: honor core.hooksPath (write hooks into the
    RESOLVED hooks dir) so a host-global hooksPath pointing at a missing
    dir cannot silently disable every guard — standing row owns this.
17. session-start.sh: dangling-hooksPath detection + printed remedy.
18. Host-wide hooksPath audit across sibling repos.
19. watch-write-revert.sh inotifywait primary mode.
20. watch-write-revert capture persistence via archive-evidence.sh.
21. Live-fire the write-revert canary in the next multi-agent window.
22. O14 two-axis rubric into check-release-docs.sh output.
23. Provider/requeue_class join on --parked-class WITHOUT --parked.
24. Sweep test files for reused-slice json.Unmarshal (stale omitempty).
25. Split TestParkedJSONSurfacesProvider into table-driven subtests.
26. Pin the --parked-class --json-envelope total semantics.
27. Direct table test for latestRequeueEvidence (3 contracts).
28. jq one-liner smoke on raw `tq tasks --parked --json` bytes post-fold
    (the Go-decode wire-shape pin covers decode, not the raw byte
    contract) — standing row owns this.
29. ratelimit-e2e.sh negative-path proof for the provider JSON grep.
30. Fold latestFailureEvidenceSummary into one pass.
31. tq doctor parked-provider "+N more" e2e smoke.
32. Webui parked badge tooltip (provider + reason in title attr).
33. Webui parked badge render-level OUTPUT pin (row 1755).
34. Reviewfix done-preflight weak-anchor false-cure audit.
35. journal-drift.sh unix-milli negative test.
36. AGENTS.md three fused line-break bullets (paired compress).
37. T24 erraudit sibling session (other repo): push 4 commits, close
    #12/#13/#14 — minted as a residue row this window. <!-- fresh:skip: other-repo (github.com/larsartmann/erraudit) -->
38. T25/T26 go-cqrs-lite archive ritual (other repo): triage 187 reports,
    then execute + digest — minted as residue rows this window. <!-- fresh:skip: other-repo (github.com/larsartmann/go-cqrs-lite) -->
39. T15/T16 (owner sudo): SystemNix input flip + deploy + tq rebuild.
40. D2/row 73 (owner or sanctioned agent): push master + cmd/tq/v0.4.3 +
    proxy verify.
41. D5: AGENTS compress-only prune to ~24k + twin re-tighten.
42. D8 (owner confirm): close the obsolete divergence row.
43. D9/D12 (owner rulings): retryx graduation record + RULING-BY-AGENT.
44. D16 (owner ruling): pma --no-verify mirror-fix (zero-byte refusal).
45. D17: flip the rewrite-rate default to adaptive p95.
46. Row 1269: backfill/dismiss the OPEN-no-row ANNOTATE items.
47. Row 878: line-contained-grep hazard sweep (check-doc-refs, parity).
48. Row 1201: poolhealth shared dead-letter seed helper (6× prologue).
49. Row 387/155 class: history-rewrite heals must re-run dead-sha in their
    OWN battery — the seventh window proved this live.
50. Amend the plan's T02 line: "battery to green" — DONE this window; the
    plan's remaining critical path is T22/T21 → T33–T35.

## g) Questions for the owner

1. **The 52 grandfathered sweeps**: baselining pushed footer-less sweeps is
   the only history-legal disposition, but it permanently retires their
   attribution. Ratify the grandfathering as the standing disposition for
   PUSHED sweeps (with the dated-reason comment), or do you want a periodic
   report of grandfathered shas so you can audit them?
2. **Toolchain-bump batteries**: the profile bump moved gosec, golangci-lint
   AND git behavior in one day. Should ci-local grow an early
   toolchain-drift leg (pin the three tool versions next to gosec's, fail
   fast with "re-pin deliberately"), or is pinning every tool overkill?
3. **P3 pool wave (T35)**: the plan prices it at 25 owner-hours paid by the
   pool — may I mint the harvest wave for the ~200-row residue tail now
   (it dispatches paid turns), or hold until the D1–D19 rulings land?
