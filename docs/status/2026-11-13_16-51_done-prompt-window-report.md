# Done-prompt window report — 2026-11-13 16:51

Window: 14 completed agent tasks, 2026-11-09 07:25 → 2026-11-13 02:00 (reviewer-fix
batch on the docs-gates work, gate-script polish, guard wiring, OKF/receipt batch,
go-cqrs-lite cross-repo fan, and the 29th-dispatch CI-reds/calibration batch).
Every §-citation below points at the task's own closeout report under
`docs/status/tasks/`; all window commits verified present in git history
(`git log -1` per sha, this session). Verified 2026-11-13 by this session:
journal probe (read-only sqlite over the production tq.db), commit-set grep,
CHANGELOG/TODO_LIST gates.

## a) Fully done

1. **Reviewer-fix batch on the docs-gates bridge batch (3 tickets, all cured):**
   `binaryRunner` no longer aborts sweeps on the real exit-1-on-FAIL contract —
   parse-first, judge-second, with the negative-shape pin test the stub suite
   lacked (commit 7a7bb39a; closeout 2026-11-09_07-25); the corpus-sweep harvest
   duplicate row was collapsed and the false "deduped" claim corrected with the
   surviving-row pointer (52b2931b; 07-30); the lying `Cursor` doc comment
   (phantom bootstrap-at-head + `watermark.Note` cite) rewritten to the real
   absent-until-first-success contract, deliberately a docs-only non-fix (5b60d273; 07-33).
2. **docs-gates residue batch** (000001a1bf09ab1f): binary-version contract pinned in
   `scripts/check-docs-gates.sh` (in-tree > released v0.1.1, stale-binary fails loud,
   3515f38a); cite-drift MISSING-FILE finding resolved by baseline per documented
   precedent + a past-EOF cite re-anchored to `companion.MustJSON` (c682335f); the
   skipped verification receipts recorded — one full ci-local run, 99 ok-countable
   lines (ca6db5d4; closeout 08-10).
3. **Gate-script polish bundle** (000001a1bf29b81e): commit-task.sh arg-parse
   corruption repaired + `--amend` mode + self-test pin, session-start unstaged-age
   nudge, archive-evidence `--wait` smoke branch + POSIX guard + stderr guard, the
   scratch-repo factory extracted to `scripts/smoke/_lib.sh`, and a session-start
   with-args e2e smoke pin (ff0b846d, cc945843, 67bca1e1; closeout 15-36). Follow-up
   fix ticket: executable bits (644→755) on `_lib.sh`/`session-start.sh`, smoke
   re-run PASS 3/3 (95a1cd56; closeout 15-56).
4. **Guard-wiring orphan triage** (000001a1c0c5dfe5): row 1153 proven landed-but-unticked
   — check-consumer-tag.sh is release-flow-wired (`release.sh:90`,
   counted by check-guard-wiring) — ticked with fresh self-test receipts (4c8ab7b2;
   closeout 15-48); the two NEW orphans (`smoke/_lib.sh`, `session-start.sh`) rowed.
5. **Post-heal footer-attribution gate** (000001a1c0d845d0): `check-footer-attribution.sh`
   (trailer-parseable footer on every span commit + attribution-view == span-set, with
   fileset fingerprints in FAIL output; 4-check self-test), wired as the heal tail step
   BEFORE backup-ref drop, AGENTS bullet + CHANGELOG Internal row (9636d224→db41ffbe;
   closeout 16-15).
6. **OKF trust-signal batch** (000001a1c0ea5c27): the additive-growth pin and the
   signals-never-scores rule found already-landed-but-unticked and ticked with cite
   (dce51b0c); `verify-ok`/`verify-mismatch` attestation-derived evidence terms landed
   in the ONE matcher copy + known-term pin, the cmd/tq `verifyReceiptView` hand-rolled
   twin dropped for the real executor types with flat-JSON compatibility preserved, and
   a full E2E dlq-filter pin added (9c07fce1; closeout 16-36); the follow-up facade-parity
   skew fixed with the labeled const-block re-export (ab663ef9; closeout 2026-11-10_01-00);
   stale post-heal sha cites cured with fork-record arrows (20f43282); CHANGELOG rows for
   the new filter vocabulary landed (2e93b471).
7. **Two-tick fan to go-cqrs-lite + cross-repo batch** (000001a1c0f84e0e): two-tick
   convention + "engine named or row not filed" sub-rule landed in go-cqrs-lite
   AGENTS + gotchas (75f7dfbf2 by a concurrent dispatch, verified not redone); heal
   `--repo` verified complete (29-check self-test); the pseudo-version-hygiene batch
   premise REVERSED with evidence (BuildFlow's check only constrains `.`-replaces;
   sibling dev-overrides deliberately exempt) — the whipsaw third-lap prevented, plus
   busfixture tidy and the stale "ignore that doctor entry" advice retired (6ad3acae8,
   84803ef8a in go-cqrs-lite; closeout 2026-11-10_01-24).
8. **29th-dispatch batch** (000001a1d242f8c5): all four master-CI red legs cured locally —
   app.css regenerated, io-surface facts.cue regenerated (the undeclared site was the
   papdashboard `io.LimitReader`), cqrs-lint path fixed to `cmd/cqrs-lint/v4` in both
   ci.yml and ci-local, govulncheck GO-2026-6859 cured via a full Go 1.27.4 lockstep pin
   (setup-go ×8, flake, every module directive, cmd/tq go.work; check-toolchain-pins 11/11)
   (6a89c734, eb6339c8); dead `FuzzExtractResultPayload` target retired with its orphaned
   448-file corpus, RULING-BY-AGENT recorded (1b0551fc); `tq calibration` dataset reader
   shipped, first receipt = 1201 production rows parsed (0d4312ac); keep-the-tail 64 MiB
   retention trim wired into the pool tick (7102684a); failure-path calibration rows
   (optional `Failure` half, additive-growth compliant) stamped at judged-fail + 429 sites
   (fb5a9794); docs/ticks (56e4c606); captured battery 48 `ok:` anchors; three daemon-sweep
   heals, final `check-footer-attribution.sh` OK 7/7 (closeout 2026-11-13_01-59).
9. **Two window tasks completed FREE at claim-time preflight** (…c0aecff0, …c0cedaff):
   journal probe shows both `completed` with attempts=0, cost 0 — the reviewfix preflight
   detected the findings already cured and closed the tickets without a paid run. No
   missing work: their cure commits (7a7bb39a-lineage / 95a1cd56) are items 1 and 3 above.
10. **CHANGELOG sync** — this session (docs-health): the `tq calibration` reader and the
    retention/failure-rows entries are now actually IN `[Unreleased]` (they were claimed
    by 56e4c606's message but absent from its diff — see §d1).

## b) Partially done

1. **The CI-reds cure is locally green but observably unfinished**: the batch's own
   acceptance criterion (tick only on a green origin run) is unmet because push is
   forbidden — the four cured legs sit in unpushed footer commits; row 1666 carries the
   BLOCKED note and the re-tick row exists (29th closeout §b1).
2. **Failure-path calibration coverage is partial by design**: parse-miss deaths, verify
   deaths, permanent fails, env-streak burns and 429 parks record rows; the OTHER no-burn
   requeue classes (crush-server death, verify-gate requeue, owner-question park, plain
   env requeue) still write none — dataset under-counts parked attempts (rowed, §b3 of the 29th).
3. **Retention cap is a constant, not a flag** (64 MiB `CalibrationMaxBytes`); owner input
   on the value/surface outstanding.
4. **Attribution splits**: the gate-polish bundle's archive-evidence bulk rode footer-less
   daemon sweep 9b788272 between foreign footer commits — heal structurally refused
   (mixed span), attribution carried by tick commit citing the sweep sha (15-36 §b2);
   same shape for the twentyeighth-dispatch's heal `--repo` deliverable (be14730b, below
   foreign footers; 01-24 §b).
5. **go-cqrs-lite docs-gates debt** (4 error findings) observed and rowed, not cured
   (01-24 §c).

## c) Not started

1. The standing owner-gated backlog was untouched by design (each rowed): the orphaned
   v0.4.2 tag lineage red (`TestDoctorHealthyEmptyDB`, reds every cmd/tq run and the
   ci-local cmd/tq leg), AGENTS.md size over budget (~27.5k/27.2k), the status-index
   bloat digest ritual (402 live rows > 400 at this session's skeleton commit), the
   local/origin master divergence, the erraudit `generic_return` migration (~197 sites),
   the pma daemon `--no-verify` hook bypass, `tq ask` dead-channel routing.
2. No dispatched batch item was skipped: every window ticket closed whole or carries an
   explicit in-row BLOCKED (post-hoc receipt re-grade parked behind its data bar; the
   CI-green tick behind the push).

## d) Totally fucked up

1. **A closeout commit claimed CHANGELOG rows its diff never landed** (56e4c606: message
   says "Unreleased CHANGELOG gains the `tq calibration` reader and the dataset
   retention/failure-row entry"; the diff touches AGENTS.md + calibration.go only).
   Found by this session's docs-health CHANGELOG sweep; fixed here. Class: commit-message
   claims need the same citation discipline as tick notes — the three daemon heals that
   window are the likely loss point, which also means heal verification must diff the
   CLAIMED file-set, not just the footers (the new footer-attribution gate pins file-sets
   of the healed span, but the claim-vs-diff check is still manual).
2. **The docs-gates residue window committed a TODO_LIST truncation** — a python splice
   dropped 1745 lines and the destruction sat in a committed HEAD before the commit-stat
   tripwire caught it; healed via amend on the exactly-mine unpushed commit (08-10 §d1).
   The scripted-doc-mutation safety rail is rowed (row 64) and still unbuilt.
3. **Two shipped defects cost paid reviewer round-trips in this window's own lineage**:
   the `binaryRunner` exit-1 abort (tests written against a happy-path stub masked the
   documented exit contract) and the `Cursor` comment asserting behavior the code never
   had — plus the facade-parity skew (exported consts without re-export, parity gate not
   mechanically enforced; the pre-commit wiring row 1861 is open).
4. **Footer-less daemon sweeps split attribution in three separate windows** (9b788272,
   be14730b, and the 29th's three heals which WERE recovered) — the edit→commit ordering
   rule keeps getting paid for instead of followed; the `--fold` mechanization row is open.
5. **Premise rot burned windows twice**: the twentyeighth-dispatch batch carried two stale
   premises from the 11-02 report (the pseudo-version sed path that had to be REVERSED, and
   a "prose-only" claim stale on arrival); the OKF batch's item-3 premise was falsified by
   the v0.4.2 tag in a favorable direction. Premise-check-first exists as a convention; the
   batch-enqueue side keeps shipping stale premises.
6. Persisting reds this window did not create and did not cure: `TestDoctorHealthyEmptyDB`,
   check-ci red predating the tree (bypassed via the gate's own escape), AGENTS size guard
   over budget.

## e) What we should improve

1. **Claim-vs-diff receipts for docs commits**: a closeout/commit message that says "X
   gains a CHANGELOG entry" should carry the gate that proves it (`check-release-docs.sh`
   or a grep receipt) — this window found the one miss, mechanical enforcement costs one line.
2. **Ship the rowed rails instead of re-paying their classes**: scripted-doc-mutation
   invariant check (row 64), facade-parity pre-commit wiring (row 1861), smoke exec-bits
   gate (row 1856), binaryRunner diagnostics (rows 49/50). Four of this window's §d items
   are already perfectly rowed cures; the next windows should EXECUTE rows, not re-derive them.
3. **Premise-check at enqueue, not just at execution**: two batches this window carried
   falsified premises; the dispatching side should stamp rows with a resolved-at sha
   (rowed: "sweep TODO rows citing pinned-tag premises").
4. **Landed-but-unticked keeps taxing**: this window alone found rows 1153, the OKF items
   1+2, and the two free preflight completions all in that class — the digest pass (row 1358)
   remains the systemic fix.
5. **Attribution: commit first, gates second** — every attribution split this window traces
   to gate runs between edit and commit. The `--fold`/nearest-bounded-span mechanizations
   would make the residue cheap, but the ordering discipline is free today.
6. **The reviewfix preflight works** — two tickets closed at zero cost. Document it (rowed
   this session) and consider its shape as the model for enqueue-time finding dedup (row 1651).

## f) Up to 50 next things (minted subset appended to TODO_LIST, dedup-checked; see the
   appended block for the 7 survivors — everything below either already has a live row
   or was judged noise)

1. commit-task.sh `--amend` hint on own-footer HEAD *(minted)*.
2. tq-bash-guard footer-literal-vs-envelope advisory *(minted)*.
3. Document the claim-time reviewfix preflight channel *(minted)*.
4. heal-daemon-sweep nearest-bounded-span mode — BLOCKED on owner span semantics *(minted)*.
5. Push the unpushed CI-cure span so the green-origin-run ticks become observable — BLOCKED
   on the owner (push forbidden to agents) *(minted)*.
6. Calibration retention cap value/surface — BLOCKED on owner *(minted)*.
7. `verify-mismatch` dead-task disposition (requeue/mint vs terminal) — BLOCKED on owner *(minted)*.
8-50. Already rowed live (verified by grep this session, NOT re-minted): binaryRunner
   stderr/timeout diagnostics (49), Sweep gocognit/mint-loop refactor (50), comments-are-claims
   audit (51), docs-gates freshness matcher false-negative (62), scripted-doc-mutation rail (64),
   wire-or-exclude the two smoke-guard orphans, facade-parity pre-commit wiring (1861),
   exec-bits gate + sweep (1856/1857), bare-git-log audit (1852), receipt-renderer unification
   (1608), webui evidence-term validation/help (1609), verifyReceiptFor dead check (1614),
   ci-local end-to-end at HEAD (1713), `--fold` mode (this list's T-batch block), heal blocker-walk
   oscillation (85), index-bloat digest ritual (standing WARN), AGENTS size re-tighten (owner),
   doctor tag-ancestry fixture (owner), `TestDoctorHealthyEmptyDB` cure (owner), post-hoc receipt
   re-grade READY-WHEN (1615), CI-reds re-tick (1686/1688), master divergence (owner), generic_return
   migration (owner-gated), finding-envelope dedup (1651), pma `--no-verify` probe (owner),
   `tq ask` routing (owner), RULING-BY-AGENT convention, tagged-but-unpublished surface note (owner).

## g) Questions (up to 3; each also appended to TODO_LIST as a BLOCKED row)

1. **Who pushes, and when?** The window's most consequential work (four cured master-CI
   legs, the toolchain 1.27.4 lockstep) is stuck behind a forbidden push; three standing
   rows demand a "green origin run" nobody but you can produce. What is the push ritual
   and cadence for footer spans that are exactly-mine and gate-green?
2. **Calibration retention**: is 64 MiB the right keep-the-tail bound, and should it be a
   flag/env (TQ_CALIBRATION_MAX_BYTES) or stay a constant?
3. **`verify-mismatch` disposition**: when a verify receipt's stored attestation refused
   (head/command/count mismatch), should the dead task requeue or mint a fix task, or is
   the filterable DLQ row the intended terminal surface?

## h) Band drift

Query: read-only sqlite probe over the production journal
(`/mnt/pool/services/tq/tq.db`, `facts` table, window 2026-11-09 → 2026-11-13).
**None recorded** — zero `task.reprioritized` facts in the window (the window's fact mix:
60 claimed, 46 requeued — dominated by the `rate-limit-gate` class, 15 enqueued, 11
completed, 2 cancelled, 2 failed). No marker/AI/unblock/importance priority moves to
account for this window.

## Harvest record

7 rows appended to TODO_LIST.md (4 agent-executable + 3 BLOCKED owner questions), all
dedup-grepped against the 798 live unchecked rows first; zero existing rows reworded or
deleted; no evidence-backed tick of foreign rows was warranted (the window's own tasks
ticked their rows in their closeout commits).
