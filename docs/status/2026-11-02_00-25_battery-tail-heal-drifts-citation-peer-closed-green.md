# Battery tail: heal under owner-directive, drift wars, citation fix — peer closed it green + pushed

Session window 2026-11-01 20:26 → 2026-11-02 00:25 (active work to ~21:05,
then a tree-settle wait and this report; the long tail was consumed by
concurrent-agent landing storms). Continuation of the 20-26 battery-chase
window. Concurrent agents + daemon active throughout.

## a) FULLY DONE

- **Prompt-const fold HEALED under the established slug**: the gate
  flagged daemon sweep 56d949ad (my agent.go prompt-rail work + a peer's
  status.go VerifyReceipt change, folded together footer-less).
  Precedent-checked first (`git log` footers): `owner-directive` is the
  standing session-scope slug for owner windows — `heal-daemon-sweep.sh
  owner-directive` rewrote all 5 unpushed commits msg-only (autostash
  around the rewrite, tree preserved, FORK-RECORDs logged, backup
  dropped after verify). Gate green.
- **Pre-existing stale citation FIXED**: `check-doc-refs` (surfaced by
  the peers' new conditional daemon-sweep doc gate) flagged AGENTS.md's
  `queue/companion/conform` → corrected to
  `internal/queue/companion/conform`; doc-refs AND daemon-sweep-docs
  gates rc=0 after.
- **Peer-gate fixture attributions**: the peers' brand-new
  `check-research-note-shape.sh` (5 heredocs) and `check-cite-drift.sh`
  (1) landed without `pins:` rows mid-storm — added honest line-above
  attributions (positive control, missing-heading, prose-vs-table,
  empty-trail, legacy-grandfather; in-window vs drifted cite), both
  self-tests still green, attribution gate green at 82 blocks.
- **Mid-run drift diagnosed, not "fixed"**: two battery deaths were the
  peers' receipt-lane refactors landing BETWEEN battery stages
  (`printDLQ*` signatures; `mustOpenStore` → `config.Deployment`) —
  cmd/tq compiled green immediately after each; no code change was
  mine to make. Root cause: batteries re-read the tree per stage during
  an active landing storm.
- **Quiet-tree strategy**: watched for an 80 s no-commit window before
  re-firing the battery (the V3.8 "quiet-tree" requirement taken
  literally); got one, ran the battery — which then tripped only the
  NEW gates the storm had shipped in the meantime (attributions,
  citation), not my code.
- **AGENTS size contention resolved by the peer**: a peer grew AGENTS.md
  to 24,891 B (budget 24,400) mid-edit; I applied only my orthogonal
  citation fix on the live file and WAITED — they pruned to 24,386
  themselves. Current 24,398 ≤ 24,400. No foreign bytes touched.

## b) PARTIALLY DONE

- **THE BATTERY RECEIPT — closed by a PEER, not me**: my chase ended
  with every gate I'd touched individually green but full-battery red on
  moving-tree noise. The 22:20→00:20 peer window (report
  `2026-11-02_00-20_ci-local-green-three-reds-root-caused-healed-pushed.md`)
  finished the job: ci-local **rc=0 over 450 steps**, three remaining
  reds root-caused (rotted smoke fixture, six foreign dead-SHA cites,
  one attribution sweep), **36+ commits PUSHED, CI green twice**.
  Origin and local are reconciled. My constituents (5-leg smoke,
  verbose-fix, rails, docs, SystemNix staging) ride that green.

## c) NOT STARTED (unchanged, owner-gated)

- SystemNix lock float + staged `crushServerSocket` activation + deploy;
  first routed-wave watch; V2.19/A15 enricher real-closeout; V1.16
  parked question under routing; A11/V2.14 routed prompts in the
  browser pane; A17 canary-flip (crush#4210 open); L4 sheet telemetry
  fill + A18 ruling.
- NOTE: my 20-26 §g "push GO?" question was answered by action — the
  peer pushed. Flip timing and the baseline-settle ratification remain
  open.

## d) TOTALLY FUCKED UP

- **Six battery launches to close what one quiet-window run could
  have**: I re-fired the full battery after every single-gate cure
  (fixture → gofmt → golines → baseline → heal → vendor×2 → drift×2)
  instead of (a) running every cheap gate once up front AND (b) waiting
  for the landing storm to stop. The 18-12 "30-second preflight"
  lesson — repeated a THIRD time by me, now with the added dimension:
  preflight must include a QUIET-TREE CHECK during concurrency storms.
- **`$(date +%H%M)` twice in one command** (20-26 window §d, closed
  here): battery rc appended to a different file than the log. All
  later runs used literal filenames.
- The heal initially felt like my call to make; it was — but the id
  choice deserved the precedent check BEFORE the gate taught it to me.
  Cheap: `git log --format='%(trailers:key=Task-Queue-ID,valueonly)'`
  over recent history answers "what slug do owner windows use" in
  seconds.

## e) WHAT WE SHOULD IMPROVE

- **Battery preflight, final form**: (1) every cheap self-test gate for
  files the window touched; (2) scoped `gofmt -l` on changed .go files;
  (3) `lint-baseline --check`; (4) doc-refs + daemon-sweep-docs; (5) a
  60-80 s quiet-tree watch. Only THEN one battery. This composes into
  the `--preflight` scripts/ candidate already noted in 20-26 §f.
- **Heal id etiquette**: the owner-directive slug exists precisely for
  this; name it in AGENTS Known Issues (the heal section) so the next
  session doesn't re-derive it. (Byte budget: 2 spare — needs a prune
  or the owner reset first.)
- **Concurrent batteries are mutually destructive**: two agents running
  ci-local simultaneously serialize on the same scratch/caches and read
  each other's half-landed trees. A fleet convention (battery lock
  file, or "only the quietest agent batteries") would save hours —
  owner call.

## f) NEXT (owner-prioritized; not harvested into TODO_LIST — most are
owner-gated; the pool dispatches TODO rows as paid tasks)

1. Owner: SystemNix lock float + uncomment the staged socket line +
   `nix run .#deploy` — the flip is now fully staged AND master is
   pushed/green; nothing else blocks it.
2. Watch the first routed wave (tq tasks / dlq / closeout facts /
   /agents/live pane) per the runbook row.
3. Record flip date + wave deltas in the planning doc LIVE-STATUS
   (paragraph pre-written).
4. V2.19/A15: enricher real-closeout pass on the first routed closeout.
5. V1.16: one parked question routes home end-to-end under routing.
6. A11/V2.14: routed prompts surfaced in the browser QuestionsPane.
7. A17: canary-flip ritual when crush#4210 ships.
8. L4 sheet telemetry fill after ≥1 week; A18 ruling (sheet drafted).
9. Owner: flip timing confirmation (staging assumes with-the-float).
10. Owner: baseline-settle ratification (≈2.4k advisory pile — the
    pushed-green peer window rode it; cleanup appetite still open).
11. Owner: AGENTS.md budget ruling (24,398/24,400 as of this report;
    three windows have now done prune surgery — a reset is due a
    decision).
12. `scripts/ci-local.sh --preflight` candidate (cheap gates + quiet
    check composed) — dispatchable to the pool as a normal task.
13. Battery-lock convention for concurrent agents (owner call).
14. Owner-directive slug note into AGENTS heal section (after budget).
15. `tq tasks --json` last_error/evidence column (owner appetite).
16. Retire the #4210 draft into the planning doc's R2 when upstream
    moves.
17. Advisory-lint top-offenders triage (executor err113/wrapcheck/wsl).

## g) QUESTIONS FOR THE OWNER (cannot self-answer)

1. **Flip timing, final**: master is pushed and CI-green; the SystemNix
   socket line is staged with exact activation instructions. Float +
   flip + deploy NOW (one change per the version-coupling rule), or
   hold for one more in-process watched wave?
2. **AGENTS.md budget**: 24,398/24,400 with three windows of prune
   surgery behind us today. Reset `agentsDocMaxBytes` to a conscious
   new ceiling (e.g. 25,000), or keep prune-to-fit?
3. **Concurrent-battery convention**: when multiple agents share this
   repo, should full ci-local runs coordinate (lock file / designated
   batter), or is last-one-green acceptable? Today it cost ~6 wasted
   battery runs across two agents.

— recorded 2026-11-02 00:25; the battery receipt and push were closed by
the 00-20 peer window (cited, not claimed); no verdict channel
(owner-directed window, no task id).
