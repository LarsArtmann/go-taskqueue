# Done-Prompt Window Report — Eighteenth Dispatch (re-dispatch) — 2026-10-28 11-02 CEST

**Task**: 000001a1815c3cc3048aaa3b632900000000 (done prompt) — RE-DISPATCH of
the seventeenth dispatch (2026-10-28 09-05). The seventeenth pass committed
its full window report + docs pass with this task's footer
(e7c7a5f1/eb43fb38/eb016efe/4a309b6f healed lineage) but no verdict receipt
was recorded at exit — the exact half-closeout class the window itself
reported (§d2 there). The queue re-claimed the task; this pass closes it.

**Delta scope**: the seventeenth dispatch IS the full derivation of this
window (15 tasks, 2026-10-27 07:28 → 2026-10-28 08:03, all 16 closeout
reports read and verified there). This report re-verifies its load-bearing
claims at 11-02 HEAD, inventories the ~2 h delta since 09:05, and records
the re-dispatch itself as a §d finding. Nothing is re-derived or re-minted
that the seventeenth pass already landed.

## a) FULLY DONE — re-verified at 11-02 HEAD

1. **The seventeenth dispatch's own landing** (4a309b6f + healed siblings):
   window report committed and indexed (docs/status/README.md row dated
   2026-10-28); README/FEATURES backoff rows landed; AGENTS.md dirty-tree
   exemption + `internal/dlqbackoff` architecture row landed (size
   23,168 B / 23,600 cap — guard green, re-checked); CHANGELOG Internal
   canary entry present; TODO rows 1064/1086 ticked; rows 1330-1334
   appended (SC1007 fix, 2 retro closeouts, erraudit twin
   consolidation BLOCKED, half-closeout policy BLOCKED).
2. **Window test pins present at HEAD** (grep-verified):
   `TestParkedJSONSurfacesProvider` (cmd/tq/parked_test.go),
   `TestDirtyTreePreflightNeverBurnsAttempt` (internal/worker/worker_test.go),
   `TestDonePreflightBatchFooterRequiresAllMembersLanded`
   (internal/harvest/donepreflight_test.go),
   `TestReportBodiesCarryNoSecretShapedStrings`
   (internal/executor/redact_reports_test.go),
   `TestDLQBackoffPanelRenders` (internal/webui/backoff_test.go).
3. **Journal band-drift query**: `tq facts --type task.reprioritized`
   → 0 facts (production journal). See §h.
4. **docs/status hygiene**: the 2026-10-20 done-prompt chain archived with
   digest row (f0103a7a, pre-dates this dispatch — noted in passing;
   archive manifest conventions followed there).

## b) PARTIALLY DONE

1. **SC1007 still red** (re-run 11-02): check-script-syntax reports
   **111 ok / 1 failed** — shellcheck SC1007 at
   scripts/check-flag-count.sh:107, from the canary fleet sweep (0ae5ff83).
   Fix row 1330 open; still the only red hard gate this window introduced.
2. **Dead-SHA debt persists**: check-dead-sha-refs still flags the
   06-59 closeout's a396d92b/d211095c cites (commit exists, unreachable
   from HEAD); the consolidated dangling-cite row stays owner-blocked.
3. **Retro closeout rows 1331/1332 unworked**: the canary fleet sweep and
   the dlqBackoffUntil webui batch still lack their a)-g) reports.
4. **Divergence grew**: local master is now **32 commits ahead** of
   origin/master (was 22 at the 09-05 pass) — rows 1320/1321 (go-health
   verification + reconcile ritual) accrue risk with every unpushed day.

## c) NOT STARTED (unchanged; the window skipped these)

Standing unblock set per the seventeenth §c, all still open at HEAD:
`scripts/check-erraudit.sh` landing (rows 1081/1311 + consolidation row
1333), `generic_return` migration (~197 sites/24 modules, row 1312),
postgres-conform smoke (1226), scripted unticked-sweep tool (1254 family),
TODO-dedup detector (1300), heal-daemon-sweep span guard (1319) +
per-commit retitling (1289) + `--print-plan`, pool-health lane split
(1265) family, done-preflight signal-4 batch-awareness (973). §f ideas
minted across closeouts remain report-only.

## d) TOTALLY FUCKED UP

1. **The done-prompt itself half-closed**: the seventeenth dispatch did
   the entire job — report, docs pass, index row, ticks, appends — and
   then exited without `tq verdict`. The queue re-claimed the task and
   paid for THIS re-dispatch to write a report whose content already
   exists (what you are reading). This is the second proven instance of
   the class (the sixteenth dispatch was itself a fifth re-claim of
   000001a17bfb2501); the §e1 remedy is mechanical, not discipline.
2. **SC1007 red rode another 2 hours** (§b1) — unfixed, as scoped: this
   pass is docs-only and the fix belongs to row 1330's dispatch.
3. **Foreign in-flight work swept by the daemon unlabeled** (in passing,
   not this window's fault): commits a8f58aba/3cc59e16 carry
   "Daemon: true" footer-less subjects but contain a NEW
   `internal/retryx` module (go.mod, rename.go, rename_test.go) plus a
   22-line reduction in internal/readmodel/schema.go and a dependabot
   entry — concurrent-agent work that will be invisible to footer-based
   attribution if the authoring session does not footer-commit. Rowed in
   §f2 rather than touched (hard scope rule).

## e) WHAT WE SHOULD IMPROVE

1. **Enforce the verdict exit mechanically**: the close contract says
   "report without verdict = half-closeout" but nothing checks it at
   exit — a done-preflight leg asserting `$TQ_RESULT_FILE` was written
   (or the task's verdict fact recorded) before the executor releases a
   done-prompt task would have saved this paid re-dispatch. Distinct
   from row 1334 (which asks about REPORT-side policy; this is the
   VERDICT-side gate).
2. Everything in the seventeenth §e stands (report-side done-preflight
   signal, countable gates at closeout, heal-daemon-sweep fixes,
   commit-first tooling, CHANGELOG at landing time, receipt hygiene,
   dedup detector) — not repeated here item-by-item.

## f) UP TO 50 NEXT THINGS (dedup-checked against open rows)

Top of the queue, highest value first:

1. **Re-green check-script-syntax** — row 1330 (exists; still red, §b1).
2. **Retro closeouts** — rows 1331/1332 (exist).
3. **Land + wire scripts/check-erraudit.sh** — rows 1081/1311, consolidate
   per row 1333 once the owner rules.
4. **Dangling-cite cure** — consolidated row (owner-blocked).
5. **Divergence reconcile + go-health deep verification** — rows
   1320/1321; 32 commits unpushed now.
6. **Verdict-receipt done-preflight leg** — NEW, appended this pass (§e1).
7. **internal/retryx module verification** — NEW, appended this pass (§d3):
   confirm the graduation intent (relates to the row-1165
   renameWithTransientRetry ask), module gates under GOWORK=off,
   check-go-mods + vendor-sync green, and adoption recorded.

Already rowed — do NOT re-mint (cite, don't copy): 1330-1334 above,
generic_return 1312, erraudit tool bugs 1313, subshell fail-tally 1314,
attribution-drift pin 1315, span guard 1319, pool-config WARN 1322,
race flake 1326, heal retitling 1289, row-path re-anchor 1290, AGENTS
budget re-tighten 1291, secretPatternSamples 1295, dupe cross-links 1296,
dedup detector 1300, count-snapshot convention 1301, proxy-wait trio
1305-1307, signal-4 973, postgres-conform 1226, battery-scope ruling 827
(already the §g3 vehicle — no separate BLOCKED row minted).

Report-only, not row-worthy yet: everything in the seventeenth §f tail
(tq dlq --json, poisoned-warning task-id naming, backingOff aggregate,
same-ms tiebreak pin, prose-vocab pin, doctor poisoned-repo check,
PapDashboard DLQ-backoff detector, park-horizon draft, DOMAIN_LANGUAGE
batch-footer entry) — unchanged, none started.

## g) QUESTIONS FOR THE OWNER

All three of the seventeenth's questions are routed and stand unchanged:

1. **erraudit twin consolidation** (rows 1081/1311) — routed as BLOCKED
   row 1333.
2. **Half-closeout policy for landed-but-unreported tasks** — routed as
   BLOCKED row 1334.
3. **Battery-scope ruling (L827)** — standing row 827.

NEW this dispatch:

4. **Verdict-exit enforcement shape**: may the executor grow a
   done-preflight leg that withholds task completion until a verdict
   receipt exists for done-prompt tasks (§e1)? It converts this exact
   re-dispatch class into a loud in-task failure instead of a paid
   round-trip.

## h) BAND DRIFT (ADR-0015 accountability)

**None recorded.** `tq facts --type task.reprioritized` over the
production journal returned 0 facts at 11-02 (re-verified this pass;
the seventeenth dispatch verified the same over the full window span).
No priority moved via marker, AI, unblock, or importance during the
window or since. Claim-order changes came from aging + new enqueues only.

---

Verification receipts for THIS report's own claims (11-02 HEAD):
check-script-syntax = 111 ok / 1 failed (SC1007, check-flag-count.sh:107);
check-dead-sha-refs tail = a396d92b/d211095c still dead;
`tq facts --type task.reprioritized` = 0; five window test pins grepped
present (paths in §a2); AGENTS.md 23,168 B vs 23,600 cap;
`git rev-list --count origin/master..HEAD` = 32; seventeenth-dispatch
footer commits located via `git log --all --grep 000001a1815c3cc3`
(e7c7a5f1, eb43fb38, eb016efe, 4a309b6f).
