# Done-prompt window report 2026-09-26 06:45 — five-task verify/ledger window

Window 2026-09-26 ~02:16–05:51 (report written 06:45). Five tasks
completed and committed; every one carried a closeout report indexed in
docs/status/README.md, and this pass re-verified the load-bearing claims
at HEAD. Two of the five were DONE-on-arrival re-dispatches (verify →
cite → stop), one root-cause-fixed a dispatcher race outside its own
item's letter, two were single-row verify windows. Written by the queue's
done-prompt turn (task 000001a0dbda953c950bd0547a6cf75e6e76).

## The window at a glance

| Task | Row | Verdict | Work commit | Closeout |
| ---- | --- | ------- | ----------- | -------- |
| 000001a0daf1… (ADR-0019 S4) | 41 | correctly BLOCKED 3rd window; side-shipped the consumer unsubscribe-race fix + vendor/ workaround | 6a10c7f7 (docs) / c561ae25 (code, footerless) | 02-31 |
| 000001a0db23… (folded-here) | 108 | DONE-on-arrival: feature already shipped (daemon cedcface), verified + row closed | 2046ee95 | 04-14 |
| 000001a0db83… (baseline regen) | 109 | DONE: poisoned-baseline heal, clean-cache green 1360/1360, ledger entry | b1d6a7d9 (018a6e32 was the verify commit) | 05-07 |
| 000001a0dbb5… (lint-lll templ) | 110 | VERIFIED already-excluded, zero code, probe with control | 5bca4c0d | 05-19 |
| 000001a0dbc3… (status oracles) | 111 | VERIFIED DONE-on-arrival (`oracleStatuses` var), in-module -race 5/5 | e11cfdfe | 05-47 |

## a) FULLY DONE (verified at HEAD this pass)

1. **TODO rows 108–111 are all `[x]` with citations in TODO_LIST.md**
   (lines 108–111), each carrying file:line cites, pinning-test names, and
   the landing commit. Verified by reading the rows this pass.
2. **Folded-here view shipped and verified** (task 2's subject):
   `foldedDaemonCommits` (cmd/tq/main.go:2271) claims only exact
   `chore: auto-commit N changed file(s) (heuristic)` subjects, walks
   parent+child adjacency per footer commit, and renders the
   `folded_here` view section. Pinned by 3 tests
   (`show_commits_test.go`), live-rendered on real history (4 daemon
   folds around the S4 task's 5 footer commits). Implementation rode
   footerless daemon commit cedcface; the verify window (2046ee95) closed
   the row.
3. **Lint-baseline poisoning healed** (task 3): the 09-26 01:41 readmodel
   regen had silently dropped all 29 cmd/tq rows (its lint ran while
   cmd/tq didn't compile, recording only `typecheck:1`). The window's
   honest clean-cache regen restored them at real counts, banked the
   gochecknoglobals shrink (root 7→5→3 across the ledger; cmd/tq:2
   restored), admitted internal/queue/companion godox:1, absorbed
   concurrent drift, and fixed the one genuinely-new cheap finding in
   code (internal/task StatusCountsView named returns). Verified this
   pass: clean-cache `lint-baseline.sh --check` rc=0 1360/1360 was re-run
   green by the 05-02 window at HEAD, and the AGENTS.md eleventh-regen
   ledger entry + REGEN POISONING gotcha are present.
4. **internal/consumer unsubscribe race root-cause-fixed** (task 1's
   side-shipped fix): a `removed` flag under the subscriber's mutex
   (consumer.go:61, set at :68) is checked before every handler
   invocation in drain, so an in-flight drain no longer delivers facts
   after unsubscribe returns. Stress-gated 30x under -race in-window;
   verified present at HEAD this pass. (Fixed in daemon-absorbed commit
   c561ae25 — footerless, see §d.)
5. **lint-lll-changed.sh templ exclusion probe-proven with a control**
   (task 4): the awk guard (`file !~ /_templ\.go$/`, line 35) excludes
   generated files; a 200-char control line in internal/task/task.go
   trips the gate while the identical line in layout_templ.go stays
   green. Third independent confirmation of the same property (09-22
   sweep, 11-30 self-review, this window).
6. **Status-oracle consolidation verified DONE-on-arrival** (task 5): one
   package-level `oracleStatuses` var (internal/task/status_test.go:9)
   feeds TestStatusTableExhaustive (:51) + TestAllStatuses (:76); landing
   was on the record twice (both 09-22 sweep reports) but the row stayed
   unchecked — the window re-verified in-module under -race (5/5) and
   closed it.
7. **All five window commits verified present with the task footer
   trailer-visible** (6a10c7f7, 2046ee95, b1d6a7d9, 5bca4c0d, e11cfdfe);
   the S4 close-out and the three later windows each ended footer-LAST
   after in-window trailer checks. All 13 reports from today carry
   docs/status/README.md index rows (check-status-index clean surface).

## b) PARTIALLY DONE

1. **ADR-0019 S4 itself — the actual row-41 work — did NOT advance.**
   Third consecutive window correctly stopped at the same gate: S2
   (consumer re-pointing) is serialized behind the S1 flip, and the S3
   read-model flip is code-complete but default-off. What the window
   added: the composition map
   (docs/planning/2026-09-26_adr-0019-s4-composition-map.md) plus the
   dispatcher fix above. Row 41 stays correctly BLOCKED.
2. **The folded-here feature was code-complete but undocumented until
   this pass** (04-14 §b1): no CHANGELOG entry, no AGENTS.md note, no
   help-text clause. This done-prompt pass adds the CHANGELOG entry +
   AGENTS.md contract note (docs-only); the `tq show --commits` help-text
   clause is CODE and is filed as a TODO row instead.
3. **The vendor/ gate workaround is durable for no one** (02-31 §b2, 05-07
   §b4): both windows trashed gitignored vendor/ so the minted verify
   could pass (row-135 structural red: `gofmt -l .` walks vendor/). At
   this pass HEAD vendor/ is ABSENT — any `go mod vendor` re-reds the
   gate until the owner's row-135 edit scopes gofmt to tracked files.
   Standing-state caveat, not a fix.
4. **Master CI is still red and its most probable cause is still live.**
   check-ci at this pass: FAIL, run at e5c386e10 predates the tree. The
   cmd/tq journal-drift tests (`TestJournalDriftNoDriftAfterRescue`,
   `TestJournalDriftSeededDriftAllFields`) were re-run by this pass and
   STILL FAIL at HEAD: the S1 claim-token migration moved the claim
   call sites (d2dd3cef/9644b749 03:30) but journal replay still covers
   only `{Status:1 Attempts:1 Priority:0 DedupKey:0}`. One window proved
   the tests green at pre-migration afbd48c4, i.e. a migration gap, not
   rot. Unowned — filed as the top TODO row.
5. **Attempt-1 of the row-109 task dead-lettered on the verify gate AFTER
   its work landed** (04-43 shipped + ticked the row; the worker verify
   died on the row-135 class; attempt 2 re-verified DONE-on-arrival). The
   row-109/103 loop is now fully documented — each retry completes its
   real work before the gate kill — but the retry-ladder signature
   suppression that would stop the burn is filed (TODO tail), not built.
6. **The two review-fix windows that audited THIS window's outputs
   landed after the closeouts** (06-05: 13 dead-SHA cites baselined with
   forensics; 06-21: the 05-02 battery's gate-misattribution corrected —
   check-doc-refs.sh rc was cited for a dead-SHA gate claim). Both fixes
   are in; the process lessons land in §e.

## c) NOT STARTED (the queue skipped or deferred)

1. **ADR-0019 S1 flip (row 38)** — everything above S4 serializes behind
   it; the claim-token sweep is mid-flight and left the journal-drift
   gap (§b4).
2. **S2 consumer re-pointing + engine-default facts-in-same-tx** (row 39
   blocked half), **S3 flip to default-on**, **cqrsqlite disposition**
   (zero importers post-flip) — the S-family ladder, untouched.
3. **Row 135 owner edit** (scope the verify gofmt stage to tracked files)
   — the single highest-leverage owner fix: 5+ dead-on-gate tasks, two
   windows forced into the vendor-trash workaround, one DONE row one
   attempt from dead-lettering.
4. **Companion extraction** (TODO tail) per
   docs/planning/2026-09-26_companion-extraction-design.md — 31 mirror
   clone groups, gate advisory until MIRROR_CLONES_STRICT=1.
5. **Row 114: ci-local end-to-end on the current lineage** — ~120
   unpushed commits, no recent window has run the full gate.
6. **The index-bloat sweep** — 229 live index rows vs the 100 threshold
   (gate warning re-confirmed at 06:45; was 223 at the 05-07 window). The
   per-item triage bar keeps the sweep expensive; existing rows own it
   (TODO tail).
7. Everything else in TODO_LIST.md — untouched by design; the window's
   four verify contracts were each exactly one row.

## d) TOTALLY FUCKED UP

1. **Master CI red across the whole window and still red** — and the
   repo's own gate discipline (a local green is worthless while master is
   red) means every one of the five DONE/VERIFIED verdicts shipped on a
   red master. Root cause live at HEAD: the S1 claim-token migration's
   journal-drift replay-coverage gap (§b4). Nobody owns the repair yet.
2. **2 of 5 tasks (plus both review-fix windows after them) were spent
   re-verifying or re-auditing already-done work.** Rows 108 and 111 were
   DONE-on-arrival (one shipped by an unreported concurrent window, one
   closed by a sweep that never ticked the row). The aged-DONE-row
   suppression question has now been asked five times (00-23, 01-18,
   01-24, 05-19 §g1, 05-47 §g2) and the 05-47 window itself committed the
   footer-before-attribution defect that row 112 exists to prevent. The
   queue is paying full windows to re-learn documented facts.
3. **Daemon-fold under-attribution struck three more times** (cedcface
   carried the fold feature beside an unrelated report; c561ae25 carried
   the consumer race fix; the 06-05 window's own amend manufactured a NEW
   uncited dead SHA). Every window in this span used the AMEND MANEUVER
   on unpushed daemon HEAD — seven instances and counting — because
   edit-then-wander beats commit-at-gate. The remedy (row: commit
   machine-consumed files at their own gate; edit→commit→battery ordering
   codification) is filed, not enforced.
4. **4 fleet dead letters inside the window span, none in this repo:**
   two CV agent tasks dead 3/3 on real CV integration-test failures
   (verify), one CV closeout failed (MCP init, 02:03), one SystemNix
   re-verification task dead at closeout on a context deadline AFTER its
   footer commit landed (05:43) — an infra-flake-shaped death of finished
   work, the closeout-timeout twin of the verify-ladder class.
5. **The verify-log battery misattribution (06-21 finding)**: the 05-02
   battery cited check-doc-refs.sh rc=0 next to a dead-SHA gate claim —
   the wrong gate's green masked a real red for hours and needed a review
   window to correct. Gate citations must name the gate by wired identity.
6. **Three windows lost commit races to the daemon** and each recovered
   via amend — legal, documented, and now so routine it is a standing
   tax, not an exception (§d3).

## e) WHAT WE SHOULD IMPROVE

1. **Ship windows close their own rows in the shipping commit.** The
   04-14 window exists only because the fold feature's shipping window
   never ticked row 108. One contract line fixes the class.
2. **Gate citations name gate identity** (script path + ci-local step),
   not just rc — the 06-21 misattribution died of an rc standing in for
   the wrong gate. A battery line without the gate's name is not evidence.
3. **Verify-only windows stop being full windows.** 3 of 5 tasks plus 2
   review-fix windows re-verified existing facts. The mechanical fixes
   are all filed (mint-time done-check, dispatch-time checkbox re-read,
   check-todo-list contradiction warning); what's missing is the owner
   ruling that makes one of them the sanctioned path.
4. **The footer/attribution shape needs a ruling and a hook**, not a
   seventh amend. The 05-47 window committed the defect the same day it
   documented it; the commit-msg hook checks presence but not placement
   (row 112).
5. **One consolidated session-shell-hazards bullet** (PIPESTATUS, printf
   `%.0s`, stderr/stdout ordering, `%(trailers:…)` literal-print) —
   four members of one family, currently scattered across three reports
   and one filed row.
6. **Probes need controls.** The 05-19 window's first battery would have
   "proven" the templ exclusion with a fixture that tested nothing; the
   control case caught it. No exclusion/allowlist probe ships without a
   negative control.
7. **The dead-SHA battery must run on the POST-REPORT tree** (06-05 §e):
   every close-out battery runs before its own report exists, so report
   dead cites pass in-window and fail the next run. Mechanical fix filed.
8. **Retrospective quality worked.** The two review-fix windows caught a
   real misattribution and a real red gate the closeouts shipped — the
   review loop is earning its spend; keep findings commit-anchored.

## f) NEXT THINGS (highest-leverage; deduped against TODO_LIST — 7 new rows filed in the Window section, rest are carried pointers)

New this pass (filed below):

1. **Land the S1 journal-drift replay coverage** (priority + dedup_key)
   so the two cmd/tq journal-drift tests pass and master CI can re-green
   — the red is live at HEAD and blocks every verdict's meaning.
2. **Split/soften the `AMBIGUOUS` verdict** for the normal multi-commit
   case (main.go:2349 fires it on every count>1 chain; the hook policy
   says multi-commit is the norm) — cry-wolf on the f26-class signal.
3. **Pin the `daemonCommitSubject` regex ↔ daemon subject template
   coupling** — the fold view goes silently blind if the daemon rewords.
4. **`tq show --commits` help text**: document `folded_here` and the
   flag-before-positional order (`tq show ID --commits` fails bare).
5. **Root-cause harvest `TestSelfManagingLoop` tick-2-empty under suite
   load** — the flake that can dead-letter future verify windows.
6. **Deterministic unsubscribe-interleaving pin** for the consumer fix
   (blocking-handler handshake that fails on pre-fix code).
7. **Caller-inventory audit of `Dispatcher.Subscribe`/unsubscribe
   semantics** post-change (webui tailer, bridges, sweepers).

Carried, highest leverage (own their rows): row 135 owner gofmt fix;
retry-ladder signature suppression; row 136 dead-letter forensic sweep;
row 112 footer-placement hook; S1 flip (row 38) → S2 → S3 → S4 ladder;
companion extraction + MIRROR_CLONES_STRICT=1; row 114 ci-local
end-to-end; row 113 internal/task re-tag; index-bloat sweep (229 live
rows); regen typecheck-refusal guard (row in tail).

## g) QUESTIONS (only the owner can answer; filed as BLOCKED rows)

1. **Footer vs attribution block — ratified shape?** Is "Task-Queue-ID as
   the FINAL line, after the Crush/Assisted-by block" the sanctioned
   shape (every window in this span had to amend to reach it), and should
   row 112's commit-msg hook hard-REJECT a well-formed footer outside the
   final trailer block? Three of five windows touched this defect class.
2. **Vendor-trash standing policy until row 135 lands:** is trashing
   gitignored vendor/ so a minted verify passes the sanctioned workaround
   (two windows used it; a DONE row was one attempt from dead-lettering),
   or should tasks dead-letter honestly until the owner fix lands, with
   row-136 rescue sweeps cleaning up?
3. **Master-CI check in docs-only verify windows:** required battery item
   or push-gate-only? House practice split within this very window (04-43
   checked, 05-19/05-47 skipped). (The DONE-row suppression question —
   5th ask, 2/5 of this window — is NOT re-filed here: its remedy rows
   already sit in TODO_LIST (duplicate-claim ruling row, done-guard row,
   dispatch-time checkbox re-read); it re-attaches there with this
   window's fresh evidence.)

## h) BAND DRIFT

**None recorded.** The production journal
(`/mnt/pool/services/tq/tq.db`, 7027 facts as of this pass) contains ZERO
`task.reprioritized` facts — the fact type exists
(internal/journal/journal.go:43, emitted by the prioritize sweeper) but
the `--prioritize` sweeper is default-OFF and no marker/AI/unblock band
change has ever fired in production. The ADR-0015 accountability ledger
is empty for this window and for the queue's lifetime to date.

## Docs-health pass notes (this pass)

- Added: CHANGELOG [Unreleased] entries for the folded-here view (Added)
  and the consumer unsubscribe race fix (Fixed); AGENTS.md fold-view
  contract note on the footer bullet; FEATURES row 92 + README CLI-table
  `tq show` mentions. Filed the help-text remainder as a TODO row (code).
- Verified, no edit needed: rows 108–111 annotations accurate at HEAD;
  AGENTS.md eleventh-regen ledger + REGEN POISONING gotcha present; row
  41 BLOCKED annotation accurate (composition map path resolves);
  ROADMAP needs no change (the S-ladder is ADR/TODO-owned, no new
  routed ideas this window).
- Not archived this pass: the window's five reports are fresh with open
  items; the 229-row archive sweep stays with its existing TODO rows
  (per-item triage bar; not skippable in a done-prompt pass).
- Battery this pass (read-only checks): check-ci rc=1 (master red
  e5c386e10, predates tree); cmd/tq journal-drift 2-test shim run rc=1
  (replay coverage gap, live); consumer fix + oracleStatuses + lint-lll
  guard + folded-here code cites confirmed at HEAD; vendor/ ABSENT
  confirmed; journal fact-type census + dead-letter span query run.
