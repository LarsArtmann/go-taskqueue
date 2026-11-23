# Done-prompt window — webui polish, board lane cap, reviewer-fix wave 4 (2026-11-23 15:29)

**Window span**: 2026-11-22 ~15:14 → 2026-11-23 15:17 (12 tasks listed at dispatch)
**Dispatch dedup**: full pass — docs/status/README.md index grepped for the
window's distinctive task IDs (000001a1ffaad98a, 000001a205966c05, 000001a205966bda:
zero hits in root-level window reports). Nearest neighbors: 2026-11-22_16-05
done-prompt (covers a DIFFERENT 5-task window ending 15:14; shares only
000001a200f919594e, whose closeout commit 0e246016 is this window's listed
landing — the fix itself, 40ee6fc2, was that window's report) and the
2026-11-23 00:38→06:03 crush-v0102 narrative reports (no window task IDs).
This is a full derivation, not a same-window re-verification. Partial-overlap
accounting: 1 of 12 tasks was already reported → see §e1.

**Journal caveat**: this shell reads the cwd fallback `./tasks.db` (TQ_DB is
denylist-stripped in agent shells, Known Issue). That journal holds 189 facts
with a 2026-09-09 head — it is NOT the dispatching pool's journal and holds
zero entries for any of the 12 window task IDs. §h band drift shares this
caveat; every landing below is git-derived at HEAD 0071dff8 (footers,
closeout reports, tree greps), which is honest provenance per the trust
vocabulary.

**Fork records (checkpoint heals)**: the skeleton commit was daemon-swept
into footer-less d2ac3d7b mid-run and healed: d2ac3d7b→0071dff8; the filled
report + doc updates were daemon-swept into another footer-less chore
commit and amended twice (the second amend was this fork-record fix): the
landing commit carrying this report is 40d71fca (1c8d8a57→40d71fca).

## a) FULLY DONE (verified at HEAD 0071dff8)

1. **API parity pin bound to the live constructor** (000001a200f919562c):
   879b3357 replaced `TestAPIFlagSurfaceParity`'s `reflect.TypeFor` type
   literal (which asserted only itself — the NumIn check could never fail,
   httpapi.New was never inspected) with a compile-time assignment
   `var _ func(queue.Store, string, *slog.Logger) (*httpapi.Server, error) =
   httpapi.New` (cmd/tq/flag_parity_test.go). Signature drift now breaks the
   build at the pin. Verified: code present at HEAD, commit footer matches.
2. **Agent-pool flag-parity tripwire derivation** (000001a200f919594e):
   40ee6fc2 extracted `newAgentPoolFlagSet()` and derived
   `TestAgentPoolOptionsFieldParity` via `fs.VisitAll`; closeout report
   committed 0e246016. (Already reported by the 2026-11-22_16-05 window —
   §a1 there; this window's dispatch listed the closeout commit.)
3. **wait_dashboard dead-branch fix** (000001a200f9195d0e): 1991f721 made
   `_lib.sh wait_dashboard` RETURN 1 on timeout instead of exiting (the
   budget-meter `|| fail_smoke` branch is live again); closeout 48387a24.
   (Also already reported by the 16-05 window; verification-only here:
   `return 1` present in scripts/smoke/_lib.sh at HEAD.)
4. **Duplicate `fs.Parse` deletion** (000001a203dadcc0): effb5601 removed
   the second back-to-back args parse in `parseAgentPoolOptions`
   (cmd/tq/agentpool.go) left by the 40ee6fc2 seam extraction; closeout
   a3cd7584. Finding 1 of that ticket was a verified no-op (already fixed by
   40ee6fc2).
5. **Board lane cap + §f19/§f20/§f21-22/§f24 batch** (000001a203cc4aada):
   five landings, rows ticked by 4534f670 citing the sweep commits — lane
   cap `max-h-96 overflow-y-auto overscroll-contain` wrapper in
   boardColumnGroups with the design note as doc comments (c2efbba6,
   fragments.templ:551/577/581 verified at HEAD), panel tables
   overflow-x:auto so DLQ reason columns scroll instead of clipping
   (b3fd22b5), WebKit search-cancel ✕ dark-mode mask (6d8b2251),
   lowercase-label scope guard test `TestLowercaseLabelScope` (aa1c0eaf),
   client-side fact-feed type filter chips with sessionStorage persistence
   (a7957e7a).
6. **§f25–§f28 harvest + back-link eyebrow** (000001a203fea5a2): per-burst
   feed badge (5785d146), pause-on-hover pin suppression (a740ec35),
   `#finding-N` anchor permalinks + `TestReviewFindingsAnchorsMarkup`
   (81936b21), two-voice usage lines + review-card join (3d6af9f9 +
   fc85d227), back-link eyebrow lowercased (594dc0e8); closeout 12eb78f8.
7. **Exact burst counts via fact-seq threading** (000001a20410ecfe): every
   feed line carries `data-seq` (fragments_templ.go:3002 verified at HEAD);
   the badge count is the pre/post-swap seq gap — exact on full window
   shifts (harness test: 9 facts into a 2-line window badge "9 new facts");
   ephemeral `.tq-feed-burst` styling (6e862141), hover-pause accumulation
   ("N new while paused", 940161ff), `executor.LinkifyFindingCites` for
   fix-prompt anchors (b46f9aae), scroll-mt corrected 4→20 against the
   measured 56px sticky header (881cfcef); closeout 7a979cd9.
8. **Dangling pre-rebase SHA citations repointed** (000001a2057f896e42):
   bfa55a4c mapped five unreachable cites (6612d27f→594dc0e8, 8051c0f0→3d6af9f9,
   d7a12325→4688cad5, 18a59f9e→6f80974c + one extra catch) to
   reachable counterparts with ancestry-verified evidence; closeout 685b8232.
9. **Closeout cite correction to sweep-fold commits** (000001a205966bda):
   970c616c — the 17-39 closeout's §a2/§a4 now cite BOTH halves (sweep-fold
   a1335ee6/8ce3ef67 alongside 7de74932/940161ff) so every file claim is
   `git show`-verifiable; TODO_LIST DONE-notes corrected in the same commit.
10. **Per-feed pause burst count + gate-pin sync + research-note cite row**
    (000001a205966bd3): the pause accumulator was module-global — a
    detail-page feed would share it and a test-order leak flaked the
    harness; made per-feed (cdd2687f; tree-identical foreign-rebuild twin
    e8bc7058, reconciled by merge 5c9be2cb); `check-webui-js --self-test`
    summary pins re-observed and synced 12/13→21/22 negative + 22/22
    positive after the harness grew to 22 tests (13d1ce01 — ci-local was
    red at HEAD on the stale pin, cured); research-note template gained the
    `repo@tag <path>` upstream-cite example row (cdd46e3e). Gate verified at
    HEAD: `check-webui-js.sh --self-test` green this pass.
11. **Burst-badge comment truthing** (000001a205966c05): 2b0daaaa — the
    `.tq-feed-burst` comment promised an auto-clear timer nothing owns;
    reworded to the real lifecycle (persists until the next zero-new swap
    or the un-hover resurface rewrites it).

## b) PARTIALLY DONE

1. **:focus-visible parity on the nowband `.tq-seg` links**
   (000001a1ffaad98a): HALF-SHIPPED, and the shipped half landed BEFORE the
   dispatch — `.tq-seg:focus-visible` background parity exists at HEAD but
   arrived in footer-less daemon sweep 1c8219e6 (2026-11-22 02:56, mobile
   pass), so the dispatched gap description was stale at mint time. The
   task's own commit 0769b178 only minted the 375px browser-shot TODO row.
   The label-brighten half of hover parity is still missing: hover has
   `.tq-seg:hover .tq-seg-label { color: gray-200 }` but no
   `:focus-visible` twin (app.css 6512–6529 verified). Keyboard focus gets
   the tint, not the label brighten.
2. **Burst-count exactness** — exact with `data-seq`, window-size stays the
   fallback when the pre-swap tail carries no seq (by design, documented).
3. **Browser-level verification** of badge/hover/anchor/lane-cap behaviors:
   class + JS-harness level only; the 375px screenshot row (minted by this
   window's 0769b178) and row 696's evidence-bar question remain open.

## c) NOT STARTED

- The remaining webui design-pass harvest rows are all still open
  (verified in TODO_LIST tail): §f29 payload-window grouping, §f30 retry
  fold, §f31 provenance ordering, §f38 two-voice ADR, §f39 harness row,
  §f41 deep-link params, §f42 band-group counts, §f43 TUI label sync,
  §f45 content-visibility, §f46 meter tooltip, §f47 touch-hover guard,
  §f49 dead-state favicon.
- The 16-05 window's §f1/f2 minted rows (agent-pool negative-path pin;
  `newAPIFlagSet` VisitAll derivation) are still open — this window's API
  pin (879b3357) bound the CONSTRUCTOR signature but did NOT extract the
  flag-set seam (verified: no `newAPIFlagSet` at HEAD).
- The monthly digest ritual row stands open (index at 417 live rows, above
  the 400 threshold — warned on every commit this pass).

## d) TOTALLY FUCKED UP

1. **ci-local was RED at HEAD mid-window**: the webui windows grew the jstest
   harness 13→22 without syncing `check-webui-js`'s summary pins (12/13), so
   the gate failed on a stale pin, not on behavior. Cured in-window
   (13d1ce01, pins re-observed from live output). The class — grow a
   countable harness, forget the pin — remains one absent-minded edit away.
2. **A stale-mint wasted a paid dispatch**: 000001a1ffaad98a was dispatched
   to fix a gap (missing `.tq-seg:focus-visible`) that footer-less sweep
   1c8219e6 had already closed ~12.5 hours before the task ran; the agent
   correctly discovered this and spent its turn minting a TODO row instead.
   The DONE-on-arrival class is rowed (row 878: release + dogfood-pool
   restart gates the mint-time re-dispatch check) — this window adds one
   more data point.
3. **Fork-record/citation debt keeps compounding**: this window alone
   produced the 17-39 window's 9-commit heal (fork records printed but not
   persisted — row 1675 owns the cure), five dangling cites born by that
   heal and repaired by 000001a2057f896e42, closeout cites that named the
   wrong half of daemon-split landings (repaired by 000001a205966bda), and
   this report's own skeleton heal (d2ac3d7b→0071dff8). Each heal is
   individually sanctioned; the aggregate is a citation lattice that
   requires ancestry checks to read.
4. **Duplicate commit pair in history**: cdd2687f and e8bc7058 carry
   identical subjects and tree-identical diffs (foreign v0.4.3 rebuild,
   reconciled by merge 5c9be2cb). History is honest but confusing to
   `git log --grep` consumers; already accounted by the 06-03 report.
5. **Footer-commit hygiene**: 879b3357 (footer 000001a200f919562c) swept in
   three unrelated files (docs/review/libs/go-atomic-write.md,
   internal/webui/render.go, a cmd/tq/main.go worker-flag line) with the
   parity fix — the commit message describes one concern; the diff carries
   four. Attribution is correct (footer present); per-file provenance
   requires reading the diff.

## e) WHAT WE SHOULD IMPROVE

1. **Partial-window overlap is invisible to the dispatch dedup**: this
   window shared task 000001a200f919594e with the 2026-11-22_16-05 window
   (as the closeout-commit leg of an already-reported fix). The
   same-window grep checks whole-set identity only; per-task-ID hits
   across adjacent window reports would flag partial overlaps before a
   second full derivation spends on them.
2. **Grow-a-harness ⇒ sync-pins must be same-change**: 13d1ce01 cured a
   multi-window stale pin. The fleet convention pins countable summaries;
   the missing half is the rule that the change that grows the count
   updates the pin in the same commit (minted below).
3. **Closeout reports for reviewer-fix wrappers are thin**: 4 of this
   window's 7 reviewer-fix tasks have no closeout report — provenance is
   commit-message-only. That is acceptable for one-line fixes but the
   17-39-adjacent cases (970c616c's cite correction) show wrapper fixes
   can carry enough nuance to owe the report.
4. **The daemon race keeps taxing every window** (five separate heals
   across this window's tasks): `commit-task.sh` exists and works when
   used first; every heal receipt in this window's closeouts follows an
   initial heredoc-commit attempt. Row 1675 (persist fork records) remains
   the highest-leverage cure for the citation half.
5. **Index bloat advisory fires on every commit now** (417/400): the
   digest ritual row is minted and standing; each commit's warning wall
   trains skimming (row 930's class).

## f) NEXT THINGS (up to 50; top items appended to TODO_LIST, cap 10)

1. Add `.tq-seg:focus-visible .tq-seg-label` (theme.css twin of the hover
   label-brighten) + regen app.css — the parity half this window's
   focus-visible task named and did not deliver. ← minted
2. Go-side pin: FactFeed markup must carry `data-seq` on every line
   (feed_filter_test pattern) so the JS exactness contract has a Go gate
   twin (17-39 §f1, unrowed). ← minted
3. ~~Sync-pins-same-change convention line in AGENTS §Conventions (grow a
   countable gate → update its summary pins in the same commit).~~
   ← minted — DONE 2026-11-23: convention line landed in the AGENTS §Conventions countable-gates bullet (GROWING a countable gate updates its summary pins in the SAME COMMIT — a gate whose output grew while its pins stayed put is a lying pin)
4. aria-live announcement parity for the "N new while paused" badge
   variant (17-39 §f6). ← minted
5. forced-colors adjustment for the dashed-border burst badge (17-39 §f7).
   ← minted
6. Ruling: browser-screenshot evidence bar for client behaviors (row 696 /
   16-05-closeout §g1) — blocks G2 credit for badge/hover/anchor/lane-cap.
   ← minted as BLOCKED
7. Ruling: burst-badge lifecycle (persist-until-zero-new-swap vs
   auto-fade after N s) — 2b0daaaa made the comment match the shipped
   behavior; the UX choice itself is still open (17-18 §g2). ← minted as
   BLOCKED
8. Ruling: O14 CHANGELOG scope for webui presentation polish (two-voice
   usage lines et al.) — 17-18 §g3. ← minted as BLOCKED
9. Row 1675: make heal-daemon-sweep persist fork records into citing docs
   (standing; highest-leverage for §d3).
10. Row 878: cut the next cmd/tq release + restart the dogfood pool so the
    mint-time re-dispatch gate reaches live agents (standing; would have
    caught §d2).
11. §f29/§f30/§f31 harvest batch (payload-window grouping, retry fold,
    provenance ordering) — same files as this window's §f25–f28 work.
12. §f38 ADR: two-voice + lowercase-below-the-hull ruling (highest-leverage
    doc artifact of the sweep).
13. 375px browser-shot verification row (minted by 0769b178 this window).
14. Agent-pool negative-path tripwire pin (16-05 §f1 row, open).
15. `newAPIFlagSet` VisitAll derivation (16-05 §f2 row, open — 879b3357
    pinned the constructor, not the seam).
16. wait_dashboard TIMEOUT contract self-test pin (16-00 closeout §f1 row).
17. budget-meter end-to-end scratch run (16-00 closeout §f2 row).
18. Dead-fallback sweep + check-smoke-dead-branches.sh (16-00 closeout
    §f3/f7 rows).
19. ~~Monthly digest ritual (standing row; 417/400 now).~~ — DONE 2026-11-23: ritual EXECUTED this window — per-item triage of the whole live 2026-10-01..05 corpus (68 reports, 5 date-bucket fan-outs, 800 verified strikes, ZERO archives: every report keeps open items, the archive bar correctly refused all 68); accrual landed on the digest 2026-11 index row; standing monthly TODO row minted.
20. Suppress jstest stub-fidelity drift documentation (§f7 row, open).
21-50. Lower-value residue observed in passing and already rowed elsewhere:
    scroll-mt on non-finding anchors (17-39 §f5), harness stub changelog
    (17-18 §f7), `--record` fork persistence (= row 1675), GOWORK=off red
    cure (row 1868), warning-fatigue ledger (row 930), help-text -h pins
    (row 2094), worker both-directions parity extension (2026-11-22
    reviewer-fix appends), content-visibility decision (§f45), meter
    tooltip (§f46), touch-hover guard (§f47), favicon variant (§f49),
    band-group counts (§f42), TUI label sync (§f43), deep-link params
    (§f41), README screenshot (§f44-class), burst badge ephemeral-voice
    review in both themes, per-feed state hoist follow-through if a second
    feed lands, jstest seq-math compute-in-test convention, transform-
    script structural asserts, check-footer-attribution arg ergonomics,
    reviewer-dispatch "likely already landed" hints, daemon-race
    commit-task.sh default adoption, stale-LSP session-start note,
    forced-colors meter twin (done — do not re-mint), lowercase scope
    guard twin for /health dark surfaces (pinned aa1c0eaf), lane-cap
    header-sticky interplay at very small heights, "+N older" link target
    param carry (§f41 sibling), chips-row a11y focus order audit,
    sessionStorage chip-reset affordance, badge aria-live region reuse,
    feed pause indicator affordance, anchor-jump history-entry pollution
    check, review-card usage-line join parity on dlqfix cards, scroll-mt-20
    re-measure if the header height changes, data-seq watermark-derived
    feed paging (bigger-scope idea → ROADMAP-class), burst badge on the
    detail-page timeline if it gains live swaps, executor.LinkifyFindingCites
    doc.go mention for external consumers, jstest DOM-stub fidelity README,
    ci-local self-test leg count pin for future gate additions.

## g) QUESTIONS FOR THE OWNER (up to 3)

1. **Evidence bar for client behaviors**: is class-level + js-harness
   verification sufficient for the new client behaviors (burst badge,
   pause-on-hover, anchor jump, lane-cap scroll), or do you want archived
   browser screenshots (the 375px row's bar) before they count as
   G2-verified? (16-05-closeout §g1, still unanswered after two windows.)
2. **Burst-badge lifecycle**: should the badge persist until the next
   zero-new swap (shipped, comment-truthed by 2b0daaaa), or auto-fade /
   clear on operator interaction?
3. **O14 scope**: do webui presentation changes (two-voice usage lines,
   lowercase back-link, lane-cap scroll behavior) owe `[Unreleased]`
   CHANGELOG entries, or do they stay repo-internal polish? (17-18 §g3;
   this pass added the fact-feed chips entry as clearly operator-visible —
   see the docs-health note below — and left the polish boundary to you.)

## h) BAND DRIFT

**None recorded.** The journal readable from this shell is the cwd-fallback
`./tasks.db` (TQ_DB denylist-stripped, Known Issue): 189 facts, head
2026-09-09, zero `task.reprioritized` facts and zero entries for any of the
12 window task IDs. It is not the dispatching pool's journal, so no
priority-movement claims are made. The ADR-0015 accountability read must be
re-run from the pool's actual DB (owner-side `tq facts --type
task.reprioritized`) to be authoritative; this report claims nothing about
the window's band movements rather than inventing them.

## Docs-health pass notes (this window)

- **CHANGELOG [Unreleased]**: added one Added entry for the fact-feed type
  filter chips (operator-visible new dashboard capability, a7957e7a) and one
  Changed entry for the board lane cap (lanes now scroll internally at
  max-h-96 instead of growing the column, c2efbba6) — both verifiable at
  HEAD; the burst badge, pause-on-hover accumulation, search-✕ dark-mode
  fix, panel-scroll and focus-visible parity are presentation polish under
  O14 and stay out pending the §g3 ruling.
- **FEATURES.md**: `tq serve` row description extended with the shipped
  dashboard capabilities (type-filter chips, burst badge, hover pause,
  capped lanes) — status remains 🟢 FULLY_FUNCTIONAL, no status inflation.
- **README/ROADMAP**: no stale claims found touching this window's scope;
  no routed ideas minted (all window residue is task-sized, rowed above).
- **AGENTS.md**: unchanged this pass (size budget tight; the sync-pins
  convention rides a TODO row until the next deliberate AGENTS edit).
- **Archives**: no report moved to archived/ this pass — the newest live
  reports (16-05, the 2026-11-23 00:38→06:03 quartet) each retain open
  forward-looking items verified against TODO_LIST at HEAD; the standing
  digest-ritual row owns the 417-row index.
- The window's five closeout reports (15-55, 16-00, 16-57, 17-18, 17-39,
  00-28) live under docs/status/tasks/ per the O7 routing and are
  queue-derivable; not indexed, not archived.
