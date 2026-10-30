# Owner Ruling Bundle #2 — Sheet (2026-10-30)

Prepared per plan §O (T1). 143 true BLOCKED rows in TODO_LIST.md
(146 lines incl. 2 already-ticked + 1 prose mention; 5 further lines
quote the marker as convention text). Grouping verified by disjoint
grep sub-patterns summing to 143.

## §0 Rulings ALREADY on record (this session, owner words verbatim)

> "NOW GET SHIT DONE! The WHOLE TODO LIST! Keep going until everything
> works and you think you did a great job! WE HAVE ALL THE TIME IN THE
> WORLD, DO NOT STOP UNTIL THE ENTIRE LIST IS FINISHED and VERIFIED!"

Blanket GO, 2026-10-30, repeated twice in one message. Recorded
rulings derived from it:

| # | Question (plan §O2 / session g1–g3) | Ruling | Effect |
| --- | --- | --- | --- |
| R1 | g1: complete v0.4.0 publish (master push, cmd/tq tag, --publish-steps-only incl. GitHub Release) | GO (blanket) | T2 proceeds end-to-end |
| R2 | g2: dead-sha mass-red absorb vs investigate | BOTH: investigation executed (class verdict: never-pushed local-rewrite residue; 4 patch-id fork records), then principled --emit-baseline absorb (O16 curation satisfied by verified wave, dated comment) | Gate green 1751/0 |
| R3 | g3: erraudit CI pin v0.7.0 vs dev build | RESOLVED: gate retargeted to explicit-flag decomposition accepted by both; CI install path fixed to cmd/erraudit | Gate green 25/25 both binaries |
| R4 | Ladder GO: L1 executor seam now vs post-v0.99 | GO on the ladder (blanket covers T4 L1 `--crush-server` seam) | T4 unblocked |
| R5 | Hook self-tests as ci-local hard gates? | GO (T7 is in the list; blanket covers it) | T7 unblocked |
| R6 | v0.99 upgrade ownership | Agent executes T8 in crush-config repo; PUSHES to other repos stay gated on explicit owner words | T8 in progress scope |

Standing limits the blanket GO does NOT override (safety rules):
real-money spend (e.g. `--prioritize` dogfood tokens), sudo on hosts,
force-push class operations, pushes to repos other than go-taskqueue.

## §1 Owner-decision rows (121) — grouped decision clusters

One GO/ruling per cluster converts up to 121 rows. Clusters ordered by
unblock leverage.

| Cluster | Rows | One-line stakes |
| --- | --- | --- |
| Intent calls: wire-vs-delete / design-or-drop (e.g. consumer into tq serve, dispatch redesign follow-ons) | ~14 | Choose keep+wire or delete dead surfaces; blanket GO reads as agent-judgment-allowed — confirm or veto per row |
| Master/origin pushes + tag waves (push button rows) | ~8 | Mostly executed under R1; residue rows reference future releases |
| Real-money spend (AI prioritize dogfood, batch scorer on pool) | ~6 | Token spend on dogfood pool; NOT covered by blanket (standing limit) |
| Host sudo / systemd unit changes (evo-x2 units, GOEXPERIMENT env) | ~5 | Owner hands-on; agent prepares patches, owner applies |
| Policy ceilings (size-budget resets, generation-vs-pin, baseline resets) | ~9 | Set the number/rule once; several ride T1 |
| Queue semantics (liveness/attempt-accounting 1355-kin, re-dispatch dedup authority) | ~7 | Semantic contract decisions with blast radius across store+worker |
| Security surface (SECURITY.md pins, auth-token rotations, write-endpoint policy) | ~6 | Owner trust decisions, not agent calls |
| Docs/website/public presence (README rewrites, website launch rows) | ~10 | Tone/brand calls |
| Release-train mechanics (proxy pins, tag policy rows) | ~12 | Mostly executed under R1/R3; residual = future-release policy |
| Misc single owner rows (gitconfig, gosec install path, archive deletes) | ~44 | Straightforward per-row GOs; blanket arguably covers the reversible ones |

## §2 External-blocked rows (3)

| Row | Blocker |
| --- | --- |
| Session trigger #3 (crush #3146 SessionEnd) | upstream issue OPEN |
| Journal-drift cmd/tq red tests (2) | root cause upstream in go-cqrs-lite |
| cqrs-lint/a014-d013-review flip | branch merge in ~/projects/go-cqrs-lite |

None actionable in this repo until upstream moves; re-check at T8/T20.

## §3 Dependency-blocked rows (13)

Serialized behind survivor rows (consolidated duplicates) or upstream
tooling rows; they resolve mechanically as T4–T27 land. No owner input
needed. Rows: L21, 45, 76, 120, 622, 670, 738, 828, 911, 957, 971,
1179, 1249.

## §4 Carried bundle-#1 rulings

None found (grep OWNER-RULINGS refs: single row L739 is an unblocked
annotation task referencing the 2026-10-05 sheet, not a carried
ruling). Bundle #1 is fully consumed.

## §5 O4 — rulings recorded back into TODO_LIST.md

Under R4/R1 the following rows lost their BLOCKED suffix this session:
L70 (push master — GO under R1), L1370 (L1 seam ladder — GO under R4).
T7 (hook gates) has no TODO_LIST row of its own; its GO lives here
(§0 R5) and in the plan. All other §1
clusters stay BLOCKED pending the owner's per-cluster pass over this
sheet; reversible no-spend rows MAY be interpreted as covered by the
blanket GO at executing time with a TODO_LIST note naming this sheet.
