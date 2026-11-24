# AGENTS.md size-budget reset history


Moved out of `cmd/tq/facts_json_test.go` (window-3 §f21/§f31: the
in-code reset log grew three-deep in one day; the history file is the
memory, the const comment names it). Each entry: the reset, its trigger,
and the twin (`scripts/check-agents-size.sh`) re-sync state at the time.


agentsDocMaxBytes pins the AGENTS.md budget at the plan M89 value:
the 2026-09-09 file measured 14,904 B; the 2026-10 prune restored the
file to 14,998 B. 2026-10-04 conscious reset 15,000 → 15,200: the
platform-endgame prune cut ~1 kB of phrasing waste yet net-new load-
bearing knowledge landed (secrets-redaction growth policy, backward
auto-upgrade seam, v4-adapter/readmodel architecture rows) — prune
in-place instead of growing the file further. 2026-10-04 (later the
same day) second reset 15,200 → 15,400: the S4 composition-module
topology row + the claim-time budget-gate note are net-new load-
bearing; same doctrine applies. 2026-10-05 third reset 15,400 →
15,700: the guard-protocol clauses (facade-parity pre-commit ordering,
claim-path cheap-first) — and the size-guard reset RULES this comment
now pins: a reset is for net-new LOAD-BEARING knowledge only; formatter
padding is PRUNED (byte-identical content restored), never budgeted —
the 2026-10-05 table re-alignment incident grew the file +1,706 B of
pure alignment and was reverted to the compact form. 2026-10-05 fourth
reset 15,700 → 16,400: two net-new load-bearing Known Issues (the Nix
2.34 outputs-thunk flake trap, the templ-components sibling-pin
pseudo-version poison), net of the cqrsqlite deletion — the M07/F034
platform rewrite re-tightens the file within this wave. 2026-10-07
fifth reset 16,400 → 16,900: the IO-efficiency pass's store
invariants (ONE shared serialized *sql.DB for engine+companion, the
synchronous=NORMAL pragma policy with TQ_SQLITE_SYNC escape, partial
RUNNING-only claim indexes, worker idle backoff) are net-new
load-bearing, compacted to two bullets before budgeting.
Reset 2026-10-07 (guard-sanctioned): the wake-seam ADR-0020 docs, the
package table, and the dep-bump gate lines are legitimate platform
documentation; the old 16.9k budget predated them. Nothing pruned was
unique — the stale "nominal v0.3.0" pin was the only correction.
2026-10-09 conscious reset 18,500 → 18,750: ~300 B of phrasing waste
pruned in place, but the remaining bulk is load-bearing; the
config-system bridge (F4) owns the next real prune. 2026-10-10
conscious reset 18,750 → 19,000: the ADR-0022 deployment-lane
invariant (one config.Deployment store-opening input, pragma literals
config-owned) is net-new load-bearing, paid for by pruning the IO
policy's now-config-owned pragma enumeration in place. 2026-10-11
conscious reset 19,000 → 19,600: the MAJOR-problems pointer
(docs/problems/major-prompting-and-batching.md, owner-mandated) and
the readmodel schema-drift outage lesson are net-new load-bearing,
paid for by compacting both blocks in place (19,595 → 19,518 B).
2026-10-12 conscious reset 19,600 → 20,000: the daemon-sweep manual
fallback (heal-daemon-sweep refuses non-contiguous footer-less spans;
verify exactly-mine + direct amend on the unpushed HEAD) is net-new
load-bearing Known Issue; a full-file scan found no phrasing waste
left to pay with (the file was repeat-compacted to within 2 B of the
budget in the two prior windows).
2026-10-13 conscious reset 20,000 → 20,100: the worktree-per-agent
delivery contract (branch tq/<task-id>, review-approve auto-merge,
reaper) is net-new load-bearing payload-contract knowledge; the
addition was compacted in place and the art-dupl doc-position rule
tightened to pay part of the cost.
2026-10-20 conscious reset 20,100 → 20,600: the formatterWithGo/goTarball
drop-day pairing Known Issue (flake formatter fix documented per the
2026-09-18 05-45 report f9) is net-new load-bearing nix knowledge.
2026-10-20 conscious reset 20,600 → 20,900: the projection-home
single-writer election (the `.fold-lock` flock every folding readmodel
Open must win; read-only loser mount; never-unlinked sidecar) is
net-new load-bearing store invariant.
2026-10-20 conscious reset 20,900 → 21,100: the canonical gate-receipt
rule (a report's "N ok / M fail" is grep -c '^ok:' over a captured
gate-output file, never a context or summary-line count) is net-new
load-bearing convention. This reset also re-syncs the
scripts/check-agents-size.sh twin, which had lagged at its 2026-10-12
20,000 budget and sat 839 B red at HEAD.
2026-10-25 conscious reset 21,100 → 23,600: the cmd/tq go.work bump
obligation (ADR-0017 workspace is load-bearing; stale go line fails every
cmd/tq go command) plus external doc growth already committed at HEAD
landed the file 2.4 kB over. Prune candidates should be reviewed and the
budget re-tightened after; twin scripts/check-agents-size.sh re-synced.
2026-10-29 conscious reset 23,600 → 23,900: the Crush PreToolUse hook
fleet bullet is net-new load-bearing (a new enforcement layer every
agent session hits), compacted in place and paid for by moving
hook-owned detail out of the doc (the scoped-gofmt recipe and the ERE
incident walkthrough now ship in the tq-bash-guard deny messages at
point-of-use); twin scripts/check-agents-size.sh re-synced.
23_900 → 24_400 (2026-10-30): dry-run-first doc-sweep convention bullet.
24_400 → 24_700 (2026-11-02): two-tick verify-before-filing convention
bullet (row-683 closeout §e2), compacted to three lines before budgeting.
24_700 → 25_400 (2026-11-03, task 000001a19fda6cc4e0d67db8efff00000000):
the CI-log triage one-liner Known Issue (net-new load-bearing: every
CI-triage session hits it) plus a 350 B pre-existing drift re-sync —
the 2026-11-02 session landed the file 350 B over the 24_700 budget
without a budget bump (gate was red at HEAD 586c640a before this
reset). Re-tighten pass rowed in TODO_LIST.md; twin
scripts/check-agents-size.sh re-synced.
25_400 → 26_400 (2026-11-06, docs-gates adoption session): net-new
load-bearing Relation rows (docs-gates gate + tuned doctrine; BuildFlow
deliberate non-coverage) — a future agent adding .buildflow.yml
"helpfully" is exactly the split-brain the row prevents; twin re-synced.
26_400 → 26_800 (2026-11-07, dedup-ledger trigger pins): three named
art-dupl extraction triggers (doctor pending-listing, payload-validation,
companion/reads.go scan) moved out of status-report archaeology into the
Executor shared seams bullet; twin re-synced.
26_800 → 27_200 (2026-11-09, libs-fleet execution session): the owner
ruling (plan §7) restored the full fleet-verdict line — the 22-lib
review's outcome and its execution state (datastar/sse v0.7.0 current,
cqrs wave landed, server_timing trial mounted) is load-bearing
orientation every library-touching session needs, and the compact
pointer had already cost one stale-truth cycle; twin re-synced.
2026-11-17: conscious reset 27_200 → 27_600 for the network-outage park
contract (one bullet in §Payload contracts; AGENTS.md 27_453 at reset).
2026-11-23: conscious reset 27_600 → 27_900 for the docs-gates v0.5.0
annotate_corpus flip doctrine (commit ea496f69 widened the docs-gates
Relation bullet with the annotate knob semantics — net-new load-bearing
for every report-writing session; AGENTS.md 27_810 at reset, foreign
growth, pruned nothing; twin scripts/check-agents-size.sh re-synced).
2026-11-23 (second): conscious reset 27_900 → 28_100 for the f11
convention line in §Known Issues (foreign-session AGENTS growth resets
the budget in the same change — the line IS that convention; AGENTS.md
27_995 at reset, own growth, pruned nothing).
2026-11-23 (third): conscious reset 28_100 → 28_250 for the foreign
facade-rename session (9f8fdb42's dead facade-Open phrasing rewrites +
the sync-pins convention line folding through sweep 7660e292; AGENTS.md
28_154 at reset, foreign growth, pruned nothing).
