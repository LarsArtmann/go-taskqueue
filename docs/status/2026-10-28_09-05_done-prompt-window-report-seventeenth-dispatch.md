# Done-Prompt Window Report — Seventeenth Dispatch — 2026-10-28 09-05 CEST

**Task**: 000001a1815c3cc3048aaa3b632900000000 (done prompt) — reporting the
window 2026-10-27 07:28 → 2026-10-28 08:03 (15 agent tasks; 13 closeout
reports read in full; 2 tasks shipped with NO closeout report — §b1).
Method: every closeout report read first, claims re-verified at HEAD
(test pins grepped, gates re-run, journal queried); nothing re-derived
from task ids. This report's own doc work: README/FEATURES backoff rows
(row 1086), AGENTS.md dirty-tree exemption + dlqbackoff table row,
CHANGELOG Internal entry for the canary layer, TODO ticks 1064/1086,
5 appended rows, this index row.

## a) FULLY DONE — verified at HEAD

1. **Parked-surface provider batch (task 000001a17bfb2332, 3/3)**:
   AGENTS.md "Rate limits (429)" bullet records the provider riding the
   requeue evidence (409167ba); `tq tasks --parked --json` rows carry an
   additive `provider` field, pinned by `TestParkedJSONSurfacesProvider`
   (cmd/tq/parked_test.go — pin verified present) (a732f257, 1547e8c5);
   `tq doctor`'s parked check names the exhausted provider
   (TestDoctorParkedNamesExhaustedProvider) (390226e6). CHANGELOG Added
   rows present. AGENTS.md size guard re-balanced in the same commit
   (found 380 B RED at HEAD — stale batch premise).
2. **LastFacts dedup + postgres proof + FEATURES MOVE (task
   000001a17c354ee3, 3/3)**: cmdTasks `--parked --parked-class` now does
   ONE `LastFacts` fetch feeding both the class join and the provider
   join (416ff2a0); `TestRequeueFactCarriesProvider` PROVEN on a real
   scratch postgres cluster (full postgresv4 conform suite pass; recipe
   embedded in the `TestStoreConformance` doc comment) (fdb258b9);
   FEATURES sweeper rendering clauses collapsed to one-line pointers at
   the Web UI outcome-card row (e0a6c3db).
3. **ci.yml parity + devmod preflight (task 000001a17c558d2b, 2/3; 1
   premise-blocked)**: check-guard-wiring wired HARD into ci.yml
   (75 wired / 0 orphaned) (a73859de); the devmod shim gained a
   require-drift preflight + loud tidy failure — which caught SIX real
   stale cmd/tq pins (v0.3.3 vs v0.3.4) on first run (335613ae). Item 1
   (dead-sha into ci.yml) correctly NOT wired: premise false, gate red.
4. **Dirty-tree park-instead-of-burn (task 000001a17ca0bd7c)**: preflight
   dirty-tree refusals are typed (`executor.ErrDirtyTree` →
   `PreflightError.DirtyTree()`) and excluded from the env-streak breaker
   — they park on the escalating ladder (cap 15 min) with NO attempt
   burn (a0a343eb; `TestDirtyTreePreflightNeverBurnsAttempt` verified in
   internal/worker/worker_test.go; CHANGELOG BEHAVIOR CHANGE row
   present). Root cause: task 000001a13d99f1 burned attempt 1 on foreign
   dirt (facts 12427-12435). This report's pass carried the carve-out
   into the AGENTS.md "Env-requeue breaker" bullet (was still describing
   the uncarved breaker).
5. **Retro closeout + landed-but-unticked sweep + dead-sha multi-pair
   (task 000001a17ce9fa12, 3/3)**: the never-reported dep-sweep batch
   (000001a141d63a4b) got its closeout
   (docs/status/tasks/2026-10-27_11-41_…md; rows 296/297/298 verdicts
   re-verified); symbol-family probe ticked two provably-landed open rows
   (842 pool alert detectors, 989 redaction census); the multi-pair
   fork-record exemption turned out to already work — landed doc + pin
   instead of a false parser fix (f941edb5). En route: deleted a
   daemon-created zero-byte `internal/executor/statusresume.go` that was
   failing every executor gate (ae711575, footer-less by design).
6. **Tick-sweep + stats goldens + AGENTS prune (task 000001a17d0e9e8e,
   3/3)**: seven-channel tick sweep ticked row 1218 (print-floor
   self-test + ci-local floor-parse dedupe) and corrected its cite rot;
   `TestStatsPayloadGoldenJSON`/`TestStatsPayloadBareShape` pin the
   `tq stats --json` wire shape (b0d090b8, e9039a20; CHANGELOG Internal
   row present); the heal-refusal Known-Issues clause landed with a
   compress-only AGENTS.md prune 23,885 → 21,587 B (be5aec2b, 55f97983).
7. **§b-partials sweep + TODO self-dedup (task 000001a17d3c674b, 2/2)**:
   all ~40 actionable §b partials across 128 closeouts verified already
   rowed; two unrowed ones minted; TODO_LIST self-dedup consolidated 7
   reworded pairs, collapsed the dead-SHA 4-row cluster to ONE survivor
   (count snapshots 289/769/801/1044 of one debt), and ticked 5
   landed/superseded rows with HEAD evidence (510cf105, f4b5ca06).
8. **proxy-wait single-sourcing (task 000001a17d60fcfb, 2/2)**:
   `SMOKE_MODULE`/`SMOKE_VERSION` feed both the die-message derivation
   and drive() (9d4c7525); the success-path assertion derives from the
   lib's `TQ_PROXY_SERVES_FMT` constant (90f74bbe). Smoke green after
   each commit.
9. **Lease-expiry reclaim root-caused + batch-aware footer signal (task
   000001a17d734f96)**: the 2026-10-14 zero-cost reclaim of task
   000001a1382a1765 (1/3 members landed, 84 ms) came from the footer
   signal completing on ANY footed commit; for MULTI-member batches the
   footer now completes only when every member key is ticked-or-gone
   (475f1af2, bc79b3eb; `TestDonePreflightBatchFooterRequiresAllMembersLanded`
   verified in internal/harvest, end-to-end reclaim test in worker;
   CHANGELOG Internal row present). Signal 4 (closeout report) is still
   NOT batch-aware — row 973 owns it.
10. **Fixture-attribution smoke extension + canary layer (task
    000001a17faa3ca, 2/2)**: the meta-gate now scans
    `scripts/smoke/*.sh` (53/53 attributed — re-verified green at HEAD
    during this pass) and caught 4 drifted check-gate heredocs en route
    (f8e3a10a); `scripts/lib/expect-branch.sh` requires rc!=0 AND a
    branch-canary grep, self-test pinned both negatives (c2b3d439 —
    self-test re-run green at HEAD); release-gates smoke adopted it and
    a latent subshell fail-tally loss was fixed en route.
11. **Canary-accuracy FLEET sweep (task 000001a17fd6b6f5)**: one commit
    (0ae5ff83) converted 8 check-* gates + smoke/archive-evidence.sh to
    canary tokens and gave check-flag-count a `FLAG_COUNT_BASELINE` seam
    so its drift self-test re-runs the real gate. **Landed but not
    closed**: no closeout, row 1064 unticked (ticked by this pass), and
    the sweep shipped a shellcheck violation that is RED at HEAD — §d1.
12. **Report-body secret gate + widening (tasks 000001a18032519, NAF
    batch item 1)**: every docs/status report walks through the SAME
    `RedactSecrets` table as output tails
    (`TestReportBodiesCarryNoSecretShapedStrings` verified in
    internal/executor/redact_reports_test.go; sweep clean; 3 pinned
    benign classes) (5344d9b9); widened to docs/problems, docs/planning,
    README.md with per-surface emptiness guards (5acaa708).
13. **Poisoned-repo DLQ backoff surface family (5 tasks: NAF items 2-3,
    NAF re-dispatch 1071-1073, follow-ups 1074-1076, webui
    000001a18120854c)** — the window's biggest landing:
    `tq pool-health --dlq-backoff` folds dead-letter facts into a
    poisoned WARN + `dlqBackoffUntil` JSON (01e2442f); `--config`/
    `$TQ_POOL_CONFIG` seeds the flag default (flag > config > off; loud
    on unreadable/malformed) (0c76d861); expiry boundary pinned
    table-driven at `until == now` / 1ns before (aa4e7ab4, verified in
    cmd/tq/poolhealth_test.go); summary line counts `M in DLQ backoff`
    with explicit zero (1679c491); `tq dlq` rows cross-link the poisoned
    state via the shared `dlqBackoffDeadlines` fold (fc4d3afb); the
    extraction became `internal/dlqbackoff` (shared package) and the
    webui dashboard renders a `backoff N` chip + red DLQ panel
    (3814abe1…744baa9d + docs dbcf677e/ca63b282; `TestDLQBackoffPanelRenders`
    verified in internal/webui/backoff_test.go). netns tri-state
    diagnostic (UNREADABLE vs non-loopback) landed with fixture-driven
    real-detector legs (8b21505b).

## b) PARTIALLY DONE

1. **Two of fifteen window tasks have NO closeout report**: the canary
   fleet sweep (000001a17fd6b6f5) shipped without a report AND without
   ticking its own row 1064; the dlqBackoffUntil webui batch
   (000001a18120854c) ticked rows + CHANGELOG but wrote no a)-g) report.
   Both now have minted retro-closeout rows. The closeout contract
   ("report without verdict = half-closeout") currently has no symmetric
   teeth for "landing without report".
2. **The canary sweep is red-adjacent**: check-script-syntax reports
   **111 ok / 1 failed** at HEAD — shellcheck SC1007 at
   scripts/check-flag-count.sh:107 (`FLAG_COUNT_SELF_TEST= ` inside
   `$( )`), introduced by 0ae5ff83 and shipped because that task ran no
   syntax gate and wrote no closeout. This is a HARD ci-local gate.
   (Re-verified during this pass; minted as a fix row.)
3. **Dead-SHA dangling cites keep accruing**: 1,072 DEAD-SHA lines at
   HEAD during this pass (801 at 10-27 09-03 → 1,044 at 13-33 → 1,072
   now; the 06-59 report itself added a396d92b/d211095c). The cure is
   consolidated to one survivor row BLOCKED on the owner's cure-shape
   ruling; +~120/day while blocked.
4. **README/FEATURES lagged the backoff surfaces** (row 1086 open since
   the 06-59 closeout) — closed by THIS pass (README command-table rows
   for `tq dlq`/`tq pool-health`; FEATURES rows for pool-health + serve).
   Still open in code: main.go usage lines omit the flags (row 1090).
5. **Docs-only battery scope** was exercised per-closeout (root
   build/vet/race or module gates) but FULL ci-local ran nowhere in the
   window — the L827 verify-window-battery-scope ruling remains open, so
   every docs session re-litigates the same scope call.
6. **Push posture unchanged**: 22 local commits unpushed; local master
   diverged from origin/master (merge-base ddbecce0; origin 1bc6f499
   carries the same go-health bump + the 04-18 closeout). Reconcile row
   + go-health deep-verification row both open; go-health v0.8.1 has
   never seen a full root race suite / release-gates / vendorHash check.
7. **Webui `TestConcurrentClientsRace` fails under `-race`** — predates
   the window (reproduced at clean origin/master), rowed with triage
   options (fix vs KNOWN_FLAKY).

## c) NOT STARTED (skipped by the window, still open)

- The standing unblock set: dangling-cite cure execution (BLOCKED owner),
  `scripts/check-erraudit.sh` landing (rows 1081 + 1311 — now a
  verified DUPLICATE PAIR, see §g), `generic_return` migration (~197
  sites, per-module batches), postgres-conform smoke (row 1226),
  scripted unticked-sweep tool (row 1254), TODO-dedup detector row,
  statusresume forensics + daemon zero-byte guard + dep-sweep smoke
  execution (minted 10-27 12-03), pool-health dirty-tree lane split
  (row 1265) + parked-surface dirty-tree hint (1266) + glossary row
  (1267) + park-population measurement (1268).
- Window-8 siblings deliberately out of scope: rows 630/631 (Every>1),
  993 (verdict-at-exit), row 720 (verify-gate-deadline clause).
- §f ideas minted across the window's closeouts (batchFullyLanded shared
  predicate, done-preflight fall-through observability, stats
  population-path golden, `commit-task.sh --stage-only`, heal per-commit
  retitling, proxy URL base single-sourcing, poke-WARN constants,
  lib-source silence pin) — all report-only, none started.

## d) TOTALLY FUCKED UP

1. **The window's last landing shipped a RED hard gate** (§b2): the
   canary fleet sweep (0ae5ff83) left check-script-syntax failing
   (SC1007) with no closeout to catch it — the exact fleet-green-rot
   class that sweep exists to prevent, self-inflicted by its own author
   task. Found only because this done-prompt re-ran the gates.
2. **Closeout-contract holes**: 2/15 tasks produced no a)-g) report;
   1/15 never ticked its own row. Under the queue's own doctrine this
   is the half-closeout class — but for REPORTS (not verdicts) nothing
   re-claims automatically.
3. **heal-daemon-sweep misattribution, three times in one session**
   (05-23): a foreign go.mod/go.sum daemon sweep got this task's footer
   + subject three separate times, each needing manual commit-tree
   repair. Combined with the 06-59 five-hop heal walk, the 08-30
   days-stale `--from` suggestions, and the 07-28 exclusive-range
   misfires, the tool's span heuristic is the window's biggest repeated
   process cost (span-guard row 1319 open; `--print-plan` row minted).
4. **edit→commit→battery inversion, again and again**: at least 15
   daemon race windows won across 9 sessions (07-28 ×3, 08-30 ×2,
   09-03, 10-35, 12-44 ×2, 12-03, 00-09, 04-18 ×2, 05-23 ×4, 06-59 ×3).
   Every closeout re-confesses it; the rule is in the file being edited.
5. **One-shots that lied**: a "successful" facade edit that didn't land
   (10-35 §d2); a silent `printf --` no-op mint that claimed a follow-up
   row existed for ~6 minutes (12-03 §d1); a sideways `git reset --soft`
   that orphaned a fold (06-59 §d2, repaired via commit-tree with
   tree-hash assertion); a compile error shipped in a first commit
   (06-59 §d3, caught by the full gate a 10-second vet would have beat).
6. **Standing reds rode through the window**: dead-SHA 1,072 and the
   2 TRAILER-WARNING f26 rows pre-date it; TestAgentsDocSizeGuard was
   found RED at HEAD twice (23,980 then 23,885 B) before the prune
   restored 2,013 B headroom (now 23,168/23,600 after this pass's
   additions — twin gate green).

## e) WHAT WE SHOULD IMPROVE

1. **Close the report-side closeout hole mechanically**: the done-preflight
   footer signal became batch-aware; signal 4 (report) completing a
   task without a report existing at all is the remaining half. Row 973
   (batch-aware signal 4) + a "report EXISTS" gate at done time would
   close both (see §g2 for the policy side).
2. **Run the countable gates at closeout, not just the touched modules**:
   check-script-syntax would have caught §d1 for one script-run. The
   12-44 session's "gate-state probes" channel (folded into
   check-unticked.sh) is the right home — a red cheap gate at closeout
   must be attributed, not inherited silently.
3. **heal-daemon-sweep needs its three minted fixes**: span guard
   (refuse content-disjoint sweeps, row 1319), per-commit retitling,
   `--print-plan` for the hop chain. Sessions burned ~20 round-trips
   re-deriving refusals the script could print.
4. **Commit-first needs to be tooling, not discipline**: `--stage-only`
   (12-44 §f2) or staging inside the edit command; ten sessions of
   confessions is the evidence the discipline-only version failed.
5. **CHANGELOG at landing time** (O14 reflex): the canary layer shipped
   2026-10-28 with no row until this pass added it; two closeouts made
   the same confession. A landing-commit checklist line beats post-hoc
   archaeology.
6. **Receipt hygiene**: mixed `ok`/`ok:`/`ok   ` prefixes make
   `grep -c '^ok:'` unreliable fleet-wide (00-52 §f8); failing receipts
   got overwritten by re-runs (04-18 §d6). Run-numbered captures and a
   pinned summary shape are both minted/rowed.
7. **Duplicate-row class is real**: rows 1081/1311 (erraudit gate) are
   the same ask minted twice in 12 hours by different sessions; the
   dedup detector row exists — this window supplies its second proven
   instance after the dead-SHA cluster.

## f) UP TO 50 NEXT THINGS (report-only; 5 appended to TODO_LIST, dedup-checked)

Highest value first — unblocks and reds before new surface:

1. **Re-green check-script-syntax** (SC1007, scripts/check-flag-count.sh:107,
   from 0ae5ff83) — the only RED hard gate this window introduced.
   [APPENDED]
2. **Retro closeout for task 000001a17fd6b6f5** (canary fleet sweep) —
   owns the SC1007 red + the FLAG_COUNT_BASELINE seam design. [APPENDED]
3. **Retro closeout for task 000001a18120854c** (dlqBackoffUntil webui
   batch) — five same-subject commits need per-commit inventory.
   [APPENDED]
4. **Land `scripts/check-erraudit.sh`** (rows 1081/1311 — consolidate
   first, §g1) + ci-local HARD wiring; then the `generic_return`
   migration batches (per-module, smallest first).
5. **Cure the dangling-cite debt** (1,072 lines, one consolidated
   survivor row) once the owner's cure-shape ruling lands; the 06-59
   report's a396d92b/d211095c cites need fork records regardless.
6. **Reconcile local/origin divergence** (22 unpushed commits; rebase
   ritual row) + go-health v0.8.1 deep verification (root race,
   release-gates, vendorHash) — the bump rides master unverified.
7. **DonePreflight signal 4 batch-aware** (row 973) + fall-through
   observability (row minted 00-09 §b2 family) — closes §e1.
8. **heal-daemon-sweep span guard + `--print-plan`** (rows 1319 + the
   06-59 mint) — the window's biggest repeated process cost.
9. **check-unticked.sh with the gate-state-probe channel** (row 1254
   family) — mechanically catches landed-but-unticked rows AND red
   gates at closeout (would have caught §d1 and row 1064's drift).
10. **webui `TestConcurrentClientsRace` triage** (fix or KNOWN_FLAKY
    with reproduction) — blocks honest `-race` receipts on every root
    battery. [ALREADY ROWED — webui-batch section; named here for
    priority, not re-minted]

Already rowed (do NOT re-mint; cite, don't copy): dirty-tree pool lane
1265, parked dirty-tree hint 1266, glossary retry-taxonomy row 1267,
park-population measurement 1268, ErrDirtyTree suffix pin 1269,
postgres-conform smoke 1226, main.go usage lines 1090, pool-config WARN
1322, TODO-dedup detector + count-snapshot convention (13-33 section),
proxy URL base / WARN constants / lib-silence pins (13-51 section),
AGENTS budget re-tighten (12-44 section), secretPatternSamples struct
fixture + dupe cross-links (13-33 item-1 mints), `--parked-class` JSON
path pin + provider surface parity (07-28 §f5-§f10), heal blocker-scope
+ amend-first rule (08-30 §f), CI postgres DSN shape (08-30 §g2),
statusresume forensics + zero-byte guard + dep-sweep execution
(12-03 §f), stats population-path golden + `--stage-only` + per-commit
retitle (12-44 §f), report-body gate scope/archives + poisoned-warning
task-id + type-scoped backoff (04-18 §f/§g), receipt capture conventions
(04-18 §f22, 00-52 §f8), netns CI leg 824, divergence + go-health rows
(05-23 section), race-flake row (webui batch section).

Report-only, not row-worthy yet: `tq dlq --json` surface; poisoned
warning naming the last dead task id; `backingOff` aggregate in
pool-health JSON; same-millisecond dead-letter tiebreak pin; prose-vocab
cross-surface pin (poisoned WARN ↔ dlq row tail); `tq doctor`
poisoned-repo check; PapDashboard DLQ-backoff detector; park-horizon
policy draft (after 1268 measures); verdict JSON minimal shape
(owner-gated); batch footer rule into DOMAIN_LANGUAGE.md.

## g) QUESTIONS FOR THE OWNER

1. **erraudit twin consolidation**: rows 1081 and 1311 ask for the same
   `scripts/check-erraudit.sh` build+wire work (one via the
   check-doc-refs MISSING cite, one from the 19-18 sweep ruling). Which
   survivor owns the work, and may a worker mark the twin
   `— DUPLICATE … — BLOCKED: consolidated` per the mint-convention?
2. **Half-closeout policy for landed-but-unreported tasks**: tasks
   000001a17fd6b6f5 and 000001a18120854c shipped verified work without
   a)-g) reports (the canary task also without its row tick). Should
   the queue mint paid retro-closeout dispatches (the 000001a17ce9fa
   precedent), or is a done-prompt-side tick + minted retro row
   sufficient — and should DonePreflight grow a "closeout report EXISTS
   for agent tasks" signal so the class stops recurring?
3. **Battery-scope ruling (L827, open since 2026-10-09)**: docs-only
   windows re-litigate "root race battery or not" every session (four
   more confessions this window). A one-line ruling — which gates a
   docs-only closeout must cite — converts ten minutes per window into
   a citation.

## h) BAND DRIFT (ADR-0015 accountability)

**None recorded.** The production journal
(`/mnt/pool/services/tq/tq.db`, 16,898 facts at read time) holds ZERO
`task.reprioritized` facts — verified over the full journal, which
covers the whole window. No priority moved via marker, AI, unblock, or
importance during 2026-10-27 07:28 → 2026-10-28 09:05. Claim-order
changes in the window came from aging + new enqueues only.

---

Verification receipts for THIS report's own claims (all re-run at HEAD
2026-10-28 09:05-09:40): `tq facts --type task.reprioritized` = 0;
`check-dead-sha-refs` = 1,072 DEAD-SHA lines; `check-script-syntax` =
111 ok / 1 failed (SC1007); `expect-branch.sh --self-test` rc=0;
`check-fixture-attribution` = 53/53 ok; `check-agents-size` = 23,168 /
23,600 after this pass's AGENTS.md additions; `check-status-index` ok
(2 pre-existing TRAILER WARNINGs); test pins grepped present:
TestParkedJSONSurfacesProvider, TestDonePreflightBatchFooterRequiresAll
MembersLanded, TestPoolHealthDLQBackoffBoundaryInstant,
TestReportBodiesCarryNoSecretShapedStrings, TestDLQBackoffPanelRenders,
TestDirtyTreePreflightNeverBurnsAttempt, netns_marker_gate. 22 unpushed
commits; `git log --grep` verified exactly one footer commit for the
canary task (0ae5ff83) and seven for the webui task (3814abe1…ca63b282).
