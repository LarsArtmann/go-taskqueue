# Owner Rulings Sheet — Pareto T1 batch (2026-10-05 15:00)

> Outcomes of the bundled owner-ruling session (plan: `2026-10-05_13-55_SUPERB-PARETO-PLAN-ALL-TODOS.md` T1).
> 17 rulings, collected live from the owner. Rows discharged are listed per ruling;
> TODO_LIST.md was edited in the same pass to carry the outcomes (BLOCKED notes
> stripped or rows deleted where the ruling closed the row's only substance).
> This file is the durable citation for every `O*` ruling below.

## Rulings

**O1 — Push authorization (rows 23/53).** Agents may push master autonomously
WHEN the full `scripts/ci-local.sh` gate passes rc=0. No force push, ever.
Footer/placement rules unchanged. Row 53 deleted (answered); row 23's residual
work (backend-tag proxy check + clean-room install) stays in the Fullcore section.

**O2 — Budget cap unit (row 197).** The daily cap is TOKEN/COST-denominated
(real spend), not task-count. Task-count stays a secondary signal.
`UsageToday` semantics + cap-unit ADR remain as executable work on the row.

**O3 — Priority scoring reimagined: JIT frontier scoring (row 198).** Score
ONLY the next-K claimable items; keep a tiny rolling cache that refreshes as
the frontier drains; re-score on item-edit/requeue facts. Time-based TTL and
the cache-aging question are RETIRED (DefaultScoreTTL included). Spend scales
with work claimed, not backlog size. Row rewritten to the implementation work.

**O4 — Duplicate-claim policy (rows 94/131/162/179/358).** Refuse terminal-ID
re-dispatch at mint/enqueue time; `--force-redispatch` is the escape hatch.
A FORCED verify-only window mints the stop artifact (row re-verify annotation +
footer commit) by default. Row 94 deleted; 131/162/358 carry the ruling cite;
179 rewritten to the forced-window stop-artifact work.

**O5 — Daemon-fold attribution (rows 93/125/157/183/266/417/435).**
Footer-first convention is POLICY: work commit BEFORE docs/TODO edits, so the
footer commit carries the pinned bytes. `heal-daemon-sweep.sh` (scripted,
unpushed-only, five verifications) is ratified as the STANDARD remedy for
daemon-swept work, and the footer-first rule gets a check script. Sanctioned
footer shape stays `Task-Queue-ID` as the FINAL trailer line (after
attribution). Rows 93/125/157/266/417/435 deleted as answered; row 183 stays
as the enforcement work with the ruling cite.

**O6 — Session mints + ask policy (rows 92/166).** Session-close-minted
review/status tasks are OPERATOR-CLASS: they bypass the daily budget
(accountability must never be budget-blocked). `tq ask` is taught in work
prompts with a ≤2-asks/task cap, budget-counted. Rows deleted as answered;
implementation rows (168 et al.) unchanged.

**O7 — Task sizing + report placement (rows 50/51/74).** The real policy:
tasks are sized ~60 MINUTES of work (owner: "most batches should probably be
around 60 minutes plus"). `--batch-items` batching becomes default-ON, sized
so a task ≈ one 60-min outcome; harvest groups sub-30-min micro-items into
batches instead of minting overhead-dominated singles; mint-time refusal (O4)
kills the repeat-dispatch half of the waste. Closeout reports move to
`docs/status/tasks/` (root stays for window reports); report TODO-appends are
capped at ~10/report. Row 74 deleted; 50/51 rewritten as executable work.

**O8 — Ghost consumer + interface triplication (rows 24/25).** DELETE
`internal/consumer` (zero production importers; the S1/S3 tailer replaced its
role) and record the ADR-0009 outcome. The three near-identical FactSource
interfaces are DOCUMENTED duplication (different semantics), revisited at S4.

**O9 — Gosec posture, lint budget, accuracy bar (rows 45/63/64).** Gosec stays
ADVISORY permanently (growth-gated baseline; no promotion to a required check).
Lint-loop budget target: ≤10 minutes. TODO-item accuracy is verified by the
EXECUTING AGENT at pickup (harvest stays cheap). Rows 45/64 deleted; 63 rewritten.

**O10 — cqrs seam (rows 87/88).** `internal/journal/cqrs` stays INTERNAL-ONLY
(proprietary seam; no public facade, ADR-0016 stands). Agent may patch
`cqrs-lint` rules in go-cqrs-lite ON A BRANCH for owner merge (the three
scoped fixes: A014 stale deprecation, D013 default-blindness, V006
lockstep-policy assumption). Row 87 deleted; 88 rewritten.

**O11 — Worktree merge policy + CSP (rows 105/109).** Worktree delivery =
AUTO-MERGE `tq/*` on review-approve AND verify-gate green (b; a; c → (a)+gates).
CSP relaxes to `form-action 'self'` so the no-JS filter fallback survives.
105 rewritten with the ruled policy; 109 rewritten as the executable CSP change.

**O12 — Platform commitment: 100% metaengine + system (rows 31/33/35/367).**
Owner: "I want to run 100% on go-cqrs-lite/metaengine and go-cqrs-lite/system."
Hand-rolled engines, cqrsqlite, and mirrored clones all die; S2 (journal on
`facts.Fact`) and S4 (`system.New` composition) complete; the dogfood cutover
(owner-run) follows. `TQ_NO_AUTO_UPGRADE` stays as escape hatch, auto-upgrade
default-ON. Row 367 rewritten to "delete cqrsqlite"; S2/S4/cutover carry the
ruling cite and raised priority.

**O13 — Archive/purge cadence + vendor-guard (rows 117/487).** Archive
eligibility + [x]-purge sweeps run WEEKLY (docs-health pass cadence). The
vendor-freshness guard is YAGNI (premise refuted 2026-10-05); row 487 deleted.

**O14 — CHANGELOG policy (rows 62/116).** `[Unreleased]` carries
OPERATOR-VISIBLE changes only (internal renames skip; the pool deploys master
as a rolling release, so external/API behavior changes get an explicit
behavior-change flag). Gate/smoke additions get ONE Added line when they gate
behavior. Row 62 deleted; 116 rewritten as the AGENTS.md codification.

**O15 — Pool-on-red, unit env, Dependabot (rows 26/46/73).** Red master is
DISCLOSE-ONLY: verdicts disclose the red state, work continues (healing CI is
itself a task); no DONE-verdict refusal. `Environment=GOEXPERIMENT=jsonv2` on
the SystemNix tq-agent-pool unit is RULED IN (still owner sudo to apply).
Dependabot is ON for the module tree; `--dep-sweep` is RETIRED after
Dependabot's first week. Row 46 deleted; 26 rewritten executable; 73 cites ruling.

**O16 — Dead-SHA gate remedy (row 469).** Mechanical path ratified:
`check-dead-sha-refs.sh --emit-baseline` generator lands; heal-daemon-sweep
mints fork-records at rewrite time; TODO-touching batteries may scope the
dead-sha leg; the baseline shrinks by human curation. Row rewritten executable.

**O17 — Cancelled-run classification (row 104).** `check-ci.sh` classifies a
CANCELLED master run as its own NEUTRAL class (not red): print
"cancelled — not red", treat as non-red, hint `gh run rerun`. Negative test
pins the branch. Row rewritten executable.

## Discharged-row ledger

Deleted (answered by ruling, decision lives here): 23-partial→residual moved,
45, 46, 53, 62, 64, 74, 87, 92, 93, 94, 116→rewritten, 125, 157, 166, 266,
417, 435, 487.
Rewritten to executable work: 50, 51, 63, 88, 98/198 (JIT), 105, 109, 179,
197, 367, 469 (dead-sha), 104/row-104 (cancelled-run), 26 (Dependabot).
Cited (ruling noted, work unchanged): 31, 33, 35, 73, 117, 131, 162, 183,
354, 358.
