# Done prompt — heal tool + review-fix triplet + requeues-audit window (2026-10-03 01:05)

**Scope**: the eight tasks completed 2026-10-02 06:06 – 08:06 CEST (ids
`000001a0f97e…` through `000001a0fb27da5703…`), their commits, their
close-out reports, and what was noticed in passing. All eight close-out
reports were read in full; every claim below was re-verified at HEAD
(commit stats, file presence, gate runs) — nothing is taken from the task
text alone.

## a) FULLY DONE

1. ~~**Worktree open questions lifted into ROADMAP + owner Q1 row minted**~~ — RESOLVED 2026-11-23: receipts verified (80427fc4 lift commit; Q1 since ruled O11 and the whole worktree slice shipped, TODO_LIST row "Implement worktree-per-agent delivery" DONE 2026-11-01).
   (task …f97e, commit c931413f + the routing commit 80427fc4). All 9
   open questions from
   `docs/planning/archived/2026-09-12_worktree-per-agent-design.md` are enumerated
   under ROADMAP "Open questions (owner decisions)"; the gating Q1
   merge-policy decision is a BLOCKED TODO row (TODO_LIST.md:111).
   The window also confessed a false-routing discovery: the 01-43
   report's strike-through claimed worktree rows already existed — they
   did not; the lift was re-done from the design doc.
2. ~~**Status-index re-sweep** (task …fad0, commit f6fa1619 + df6915f4):~~ — RESOLVED 2026-11-23: both commits verified present.
   258 live reports triaged, **113 archived** (85 fully-done, 28
   verify-only duplicates), index live-rows **270 → 157**, all doc gates
   green, root battery `-race` rc=0, and 4 unrouted KEEP items harvested
   into TODO rows (which became the rest of this window's queue).
3. ~~**SQLITE_BUSY TODO row filed** (task …faf5, commit d07ba6e9/05bc9eca):~~ — RESOLVED 2026-11-23: filing receipts verified (both commits exist; the implementation row lives on open/BLOCKED as TODO_LIST row "SQLITE_BUSY retry/backoff", which this filing item does not own).
   verified the retry/backoff is genuinely NOT shipped (only
   `busy_timeout(5000)`, `internal/queue/sqlitev4/adapter.go:84`), filed
   the implementation row (TODO_LIST.md:102, BLOCKED on the owner
   open-path ruling) and struck the filing chore.
4. ~~**`scripts/heal-daemon-sweep.sh` shipped and hardened** (task …afea,~~ — RESOLVED 2026-11-23: script live at HEAD with --self-test wired into ci-local (scripts/ci-local.sh:201-205); receipts f126b00a/9e920db4/406192cf verified.
   commits f126b00a, 9e920db4, 406192cf): scripted filter-branch heal of
   daemon-swept footer-less commits on the UNPUSHED range only, with five
   mandatory verifications (subjects, per-commit stats, one well-formed
   trailing footer, byte-equal tree, no tags), backup ref kept on
   failure, fork records printed, `--self-test` green (10 checks),
   routed in AGENTS.md's concurrent-agents bullet, and dogfooded: the
   window's own daemon sweeps were healed by the tool mid-session.
5. ~~**Requeue facts surfaced in `tq audit --journal`** (tasks …fb1a windows~~ — RESOLVED 2026-11-23: feature live at HEAD (Requeues RequeueSummary in cmd/tq/journalaudit.go:179-198); receipts eaf971c5/82e3ac0d/f4b749da/c7cd5655/0dbc35e5 verified.
   1+2, commits eaf971c5/82e3ac0d, verify+unblock f4b749da/c7cd5655):
   requeues summary (total, per-class with legacy facts normalized to
   `unknown`, resume-closeout parks, rate-limit/env-streak hint) in text
   and JSON (`cmd/tq/journalaudit.go`); the fact feed already carried
   `[class=…]`. The second window also unblocked the RED
   `TestAgentsDocSizeGuard` by pruning AGENTS.md to 14,639/15,000 B
   (commit 0dbc35e5).
6. ~~**Three reviewer findings fixed, one commit each** (tasks~~ — RESOLVED 2026-11-23: all three rails live and pinned at HEAD (tag refusal scripts/heal-daemon-sweep.sh:195, hex-validated ID, backup-ref from symbolic-ref; receipts 69621553/3f9f497d/a623f7af/92c9193c verified; self-test pins all three).
   …fb27da3d/da4e/da57, commits 69621553, 3f9f497d/a623f7af, 92c9193c):
   - **Tag rail**: a tag pointing into the heal range now REFUSES the
     heal pre-flight (previously only new commits were checked, so a
     tagged old commit would dangle silently).
   - **Shell injection**: the Task-Queue-ID is no longer interpolated
     into the msg-filter (env-var transport `TQ_HEAL_FOOTER`) and is
     hex-validated up front — the exploit probe (`abc';touch /tmp/pwned;'`)
     is refused, verified dead.
   - **Backup-ref ambiguity**: the filter-branch backup ref resolves from
     `git symbolic-ref HEAD` with an explicit existence check instead of
     `head -n 1` over all `refs/original/*`.
     All three: `--self-test` green, root battery rc=0, fix footers exact.
7. ~~**internal/status ULID flake fixed and adjudicated** (immediately after~~ — RESOLVED 2026-11-23: fix + adjudication receipts verified (93ef8642/de98ed96/2fa5376d; internal/task -race green at HEAD 2026-11-23).
   this window, same queue arc, commits 93ef8642/de98ed96/2fa5376d):
   `task.NewID` made process-monotonic; TODO row 105 struck; the
   queue's automatic re-dispatch was adjudicated as done-on-arrival
   without a duplicate fix.

## b) PARTIALLY DONE

1. ~~**The heal tool's self-test does not pin its three newest rails**: no~~ — RESOLVED 2026-11-23: all three pins landed — tagged-range refusal, metacharacter/empty-id refusals, and the multi-branch backup-ref case are in the self-test (scripts/heal-daemon-sweep.sh, expect_refusal cases + "Multi-branch backup-ref resolution (92c9193c)" fixture).
   tagged-range case (08-01 §c1), no metacharacter/empty-id refusal case
   (08-04 §e1), no multi-branch backup-resolution case (08-06 §e1). The
   refusals exist and were hand-probed, but nothing prevents silent rot.
2. **The daemon race is mopped, not fixed.** Every window in this pass
   lost the race at least once (4th–6th recorded instances); the heal
   tool + amend maneuvers recovered all of them, but the daemon-side
   short-delay/lockfile (00-27 §f3) is still unbuilt — this window is
   the strongest evidence yet that the mop does not scale.
3. ~~**AGENTS.md budget headroom is ~360 B by design**: the doc-size guard~~ — RESOLVED 2026-11-23: superseded — budget re-derived to 26,800 B (cmd/tq/facts_json_test.go:107 "26_400 → 26_800"); AGENTS.md now 28.1 KB with the guard green.
   went red twice in two days and each closeout that touches AGENTS.md
   re-risks it. The size-gate-at-edit-time script (07-53 §f6) is rowed,
   not built.
4. ~~**ci-local was NOT green-run on any window's final tree** — one window~~ — RESOLVED 2026-11-23: superseded — full ci-local runs green as standing ritual (TODO_LIST.md:20, 2026-11-23 receipt).
   was refused by the foreign-red preflight (a red master CI run older
   than the tree), the others ran the underlying gates directly. The
   pre-push gate is effectively unexercised on this lineage.
5. ~~**Status-index live rows remain 157 > 100** — defensible KEEP-OPEN~~ — RESOLVED 2026-11-23: both levers landed since — check-archive-eligibility.sh (TODO row "mechanical archive bar" LANDED) and the digest row ritual (TODO rows "digest row" DONE; threshold re-derived 100→400).
   floor per the 06-44 report's convention analysis, but the bloat
   WARNING still fires by design and the digest row + eligibility-script
   levers (06-44 §f3/§f1) are both still unbuilt.

## c) NOT STARTED

1. ~~**Worktree implementation itself** (`--worktrees`, 01-43 §f11–20) —~~ — RESOLVED 2026-11-23: shipped — `--worktrees` + auto-merge + reaper + smoke all DONE (TODO_LIST "Implement worktree-per-agent delivery" umbrella closed 2026-11-01; scripts/smoke/worktree.sh green).
   correctly gated on the owner's Q1 merge-policy answer; only the
   question-routing landed.
2. **SQLITE_BUSY retry/backoff implementation** — the row is filed and
   BLOCKED on the owner production-vs-smoke ruling; no code.
3. ~~**`scripts/check-archive-eligibility.sh`** — second full judgment~~ — RESOLVED 2026-11-23: landed — script + --self-test wired into ci-local (scripts/ci-local.sh:933); used as the mechanical bar for this very triage.
   sweep (06-44) has now run without it; the mechanical bar remains
   TODO.
4. **Daemon short-delay/lockfile** — carried across five windows now,
   never started.
5. ~~**The 06-44 sweep's own improvement backlog** (spot-verify 10% of~~ — RESOLVED 2026-11-23: all four levers landed — digest row (TODO row DONE 2026-10-26), threshold re-derived 100→400 (TODO row DONE 2026-10-20), stale-row hygiene via annotate-never-rewrite convention, spot-verify superseded by mechanized corpus sweeps (docs-gates freshness, 883 reports replayed).
   archive verdicts, digest row, stale-row hygiene ruling, threshold
   re-derivation) — none started.

## d) TOTALLY FUCKED UP

1. ~~**The repo's own footer rule was violated by the tool author's first~~ — RESOLVED 2026-11-23: historical event (6e142726 verified); rule since mechanized (AGENTS.md:132 + scripts/check-footer-attribution.sh).
   commit** — the heal enforcer's author placed `Task-Queue-ID:` ABOVE
   the attribution block (6e142726), the exact invisible-footer
   anti-pattern AGENTS.md documents; only the tool's own refusal caught
   it. Healed, but the pattern "write an enforcer, trip it immediately"
   repeated across the window (edit→battery→commit ordering lost to the
   daemon in four separate windows despite the AGENTS rule saying
   commit-first).
2. ~~**One battery ran without `-race` and was briefly claimed as~~ — RESOLVED 2026-11-23: historical, re-run with -race before shipping as the item itself records; -race is now the ci-local default (scripts/ci-local.sh:383).
   verified** (06-44 §d1); re-run with `-race` before anything shipped,
   no damage, but the claim order was wrong.
3. ~~**A soft-reset fold consumed a daemon commit** (06-44 §d2): safe this~~ — RESOLVED 2026-11-23: historical; post-commit verification since codified (AGENTS.md:288 "after ANY commit" re-check; TODO row "Post-commit footer re-read convention" DONE 2026-10-31).
   time (verified exactly-mine, after the fact), aggressive by
   construction — the pre-reset `git diff --stat` evidence check is now
   a rowed convention candidate.
4. ~~**Dispatch-text staleness nearly caused a duplicate build**: the~~ — RESOLVED 2026-11-23: class mechanized — internal/harvest/donepreflight.go footer-based done-preflight + scripts/redispatch-brief.sh run at dispatch.
   …fb1a re-dispatch asserted "unshipped and unrouted" 14 minutes after
   the feature window had shipped it; only a first-tool-call
   `git log` check prevented re-implementing `requeueSummary`. Second
   same-hour instance of this class in recent history.
5. ~~**Nothing is currently broken at HEAD**: root build+vet+`test -race`~~ — RESOLVED 2026-11-23: historical green-tree closure; current HEAD also green on the named surfaces (internal/task -race ok 2026-11-23).
   rc=0, doc gates green (re-verified this pass: `check-doc-refs.sh` ok —
   the `companion/conform` failure flagged by the 06-06 report has since
   healed), `heal-daemon-sweep.sh --self-test` 10/10, tree clean.

## e) WHAT WE SHOULD IMPROVE

1. **Commit-first is still not the default reflex** — four windows in one
   day re-learned it mid-session after a daemon sweep. The heal tool
   makes recovery cheap; the lockfile (or a pre-commit daemon pause)
   makes it not happen. File the lockfile as this window's top row.
2. ~~**Every new rail needs a self-test case in the same commit** — all~~ — RESOLVED 2026-11-23: convention now standing (AGENTS.md:249 "--self-test pinning its summary format + a negative-path", :347 heal rails pin); all three missing cases landed in the self-test.
   three review fixes shipped behavior that only manual probes verify.
   One bundle row covers the three missing cases.
3. ~~**Dispatch claims need a freshness re-check as step zero** (grep~~ — RESOLVED 2026-11-23: mechanized — internal/harvest/donepreflight.go (footer-carry check, "no-op over 21 footer commits") + scripts/redispatch-brief.sh.
   `git log` + TODO row state before any other call) — cheap, and it
   just saved a full duplicate window.
4. ~~**Sub-agent archive verdicts still ride unverified** — 113 `git mv`s~~ — RESOLVED 2026-11-23: levers landed — scripts/check-archive-eligibility.sh mechanical bar (ci-local:933) + mechanized docs-gates freshness corpus sweeps (883 reports replayed).
   on six agents' word; the 10% spot-check and the eligibility script
   are the two standing levers.
5. ~~**The script-only change class has no mandated gate**: `bash -n` +~~ — RESOLVED 2026-11-23: landed — scripts/check-script-syntax.sh + --self-test wired as ci-local steps (scripts/ci-local.sh:521-525).
   self-test were self-chosen each time; the repo-wide
   `scripts/check-script-syntax.sh` gate exists but ci-local wiring for
   script-only windows is inconsistent (one window was refused by the
   foreign-red preflight instead).

## f) NEXT THINGS (impact-sorted; deduped against TODO_LIST at HEAD)

1. Daemon short-delay/lockfile: skip auto-commit of files touched in the
   last N seconds (00-27 §f3, sixth recorded race instance this window).
2. ~~Wire `heal-daemon-sweep.sh --self-test` into ci-local as a hard step~~ — RESOLVED 2026-11-23: wired — scripts/ci-local.sh:201-205 runs the self-test as a step ("refusal rail pin").
   (07-20 §f2) so the refusal rails cannot rot silently.
3. ~~Grow the heal self-test: tagged-range refusal, metacharacter/empty-id~~ — RESOLVED 2026-11-23: all three cases in the self-test (tag refusal "tag(s) reference commits inside the heal range", metacharacter/empty-id expect_refusal, multi-branch backup-ref fixture).
   refusal, multi-branch backup-ref resolution (08-01 §f1, 08-04 §f1,
   ~~08-06 §e1).~~ — RESOLVED 2026-11-23: continuation of item 3; same receipt (self-test cases present at HEAD).
4. Diagnose/fix the pre-existing red master CI run that makes ci-local
   refuse every invocation until consciously bypassed (08-01 §b2/§f3).
5. ~~Heal-script polish: `run_filter` failure-path stderr diagnostics +~~ — RESOLVED 2026-11-23: landed — verify_heal's check #5 carries the comment naming the pre-flight tag rail (scripts/heal-daemon-sweep.sh, "the pre-flight tag rail in check_rails (row 434…)").
   verify_heal check #5 comment naming the pre-flight tag rail (08-06
   §e2, 08-01 §e1).
6. ~~Mint the worktree implementation slice rows (01-43 §f11–20) ready for~~ — RESOLVED 2026-11-23: done — slice rows minted and ALL since ticked (TODO_LIST "Mint the worktree implementation slice rows" DONE; umbrella closed 2026-11-01).
   the moment the owner answers Q1 (06-06 §f5).
7. ~~`scripts/check-archive-eligibility.sh` — the mechanical archive bar,~~ — RESOLVED 2026-11-23: landed — TODO row "mechanical archive bar" LANDED; script + self-test run in ci-local (scripts/ci-local.sh:933).
   standing since before the 06-44 sweep (06-44 §f1).
8. ~~Spot-verify a 10% sample of the 06-44 sweep's 85 ARCHIVE-DONE~~ — RESOLVED 2026-11-23: superseded by mechanized verification — check-archive-eligibility gate + docs-gates freshness corpus sweeps (883 reports replayed; full ANNOTATE/archive pass DONE 2026-10-26).
   verdicts at HEAD; demote+restore any mis-archived report (06-44 §f2).
9. SQLITE_BUSY repro harness: run the smoke pair concurrently N times,
   count busy_timeout exhaustions, give the owner ruling data (06-57 §f3).
10. ~~Add a `docs/status` digest row for the 06-44 sweep per the README's~~ — RESOLVED 2026-11-23: landed — TODO row "digest row to docs/status/README.md" DONE 2026-10-26 (`digest 2026-10 (02nd)` row in the index).
    proposed format (never once used) so monthly intake has a non-live
    summary surface (06-44 §f3).

## g) QUESTIONS ONLY THE OWNER CAN ANSWER

1. **AGENTS.md size-guard budget policy**: keep 15,000 B hard with a
   mandatory per-closeout prune ritual, or reset `agentsDocMaxBytes` to
   a standing headroom (e.g. 15,500)? The guard has gone red twice in
   two days and will again (07-53 §g1).
2. **Is the scripted msg-filter heal ratified as the standard remedy**
   (unpushed-only, five verifications mandatory), so the AGENTS.md
   bullet stands as owner policy rather than one agent's routing?
   (07-20 §g1, carried.)
3. **Foreign-red ci-local refusal**: keep the conscious `CI_CHECK=off`
   friction, or document an automatic "red run predates tree → skip CI
   check" mode? (08-01 §g2.)

## h) BAND DRIFT

None recorded: `tq facts --type task.reprioritized` returns 0 facts, and
no `task.reprioritized` fact appears in the journal for the window's
timespan (2026-10-02 04:00 → now). No priority moved this window; claim
order was queue dispatch order throughout.

## Docs-health pass notes

- All eight window reports are fresh, indexed, and referenced by the
  queue lineage — none archived. The 2026-10-02 06-44 sweep already
  archived the stale backlog (113 files); no further archiving is due.
- CHANGELOG [Unreleased] gained entries for the window's three
  user-visible ships (requeues audit summary, heal-daemon-sweep.sh,
  process-monotonic task IDs) — they were missing.
- FEATURES.md row 94 (`tq audit --journal`) extended with the requeues
  summary; no other FEATURES claims changed status this window.
- AGENTS.md already carries the heal-tool routing (done by the 07-20
  window) and sits at 14,639/15,000 B — left untouched to preserve
  headroom.
- README.md and ROADMAP.md verified current: ROADMAP carries the 9
  worktree questions + Q1 pointer (06-06's own work); README's
  install/quickstart contract is untouched by this window.
- TODO_LIST.md: no existing rows reworded or deleted; the window's
  delivered rows were already struck by their own windows (lines
  ~~101–105 verified `[x]`); new rows appended in a dated section below.~~ — RESOLVED 2026-11-23: continuation of the docs-health bullet; the then-live rows 101–105 were struck and the corpus since archived (index rows for this window's reports are backticked `archived/…`-eligible material).
