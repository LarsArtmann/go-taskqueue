# Done prompt — heal tool + review-fix triplet + requeues-audit window (2026-10-03 01:05)

**Scope**: the eight tasks completed 2026-10-02 06:06 – 08:06 CEST (ids
`000001a0f97e…` through `000001a0fb27da5703…`), their commits, their
close-out reports, and what was noticed in passing. All eight close-out
reports were read in full; every claim below was re-verified at HEAD
(commit stats, file presence, gate runs) — nothing is taken from the task
text alone.

## a) FULLY DONE

1. **Worktree open questions lifted into ROADMAP + owner Q1 row minted**
   (task …f97e, commit c931413f + the routing commit 80427fc4). All 9
   open questions from
   `docs/planning/2026-09-12_worktree-per-agent-design.md` are enumerated
   under ROADMAP "Open questions (owner decisions)"; the gating Q1
   merge-policy decision is a BLOCKED TODO row (TODO_LIST.md:111).
   The window also confessed a false-routing discovery: the 01-43
   report's strike-through claimed worktree rows already existed — they
   did not; the lift was re-done from the design doc.
2. **Status-index re-sweep** (task …fad0, commit f6fa1619 + df6915f4):
   258 live reports triaged, **113 archived** (85 fully-done, 28
   verify-only duplicates), index live-rows **270 → 157**, all doc gates
   green, root battery `-race` rc=0, and 4 unrouted KEEP items harvested
   into TODO rows (which became the rest of this window's queue).
3. **SQLITE_BUSY TODO row filed** (task …faf5, commit d07ba6e9/05bc9eca):
   verified the retry/backoff is genuinely NOT shipped (only
   `busy_timeout(5000)`, `internal/queue/sqlitev4/adapter.go:84`), filed
   the implementation row (TODO_LIST.md:102, BLOCKED on the owner
   open-path ruling) and struck the filing chore.
4. **`scripts/heal-daemon-sweep.sh` shipped and hardened** (task …afea,
   commits f126b00a, 9e920db4, 406192cf): scripted filter-branch heal of
   daemon-swept footer-less commits on the UNPUSHED range only, with five
   mandatory verifications (subjects, per-commit stats, one well-formed
   trailing footer, byte-equal tree, no tags), backup ref kept on
   failure, fork records printed, `--self-test` green (10 checks),
   routed in AGENTS.md's concurrent-agents bullet, and dogfooded: the
   window's own daemon sweeps were healed by the tool mid-session.
5. **Requeue facts surfaced in `tq audit --journal`** (tasks …fb1a windows
   1+2, commits eaf971c5/82e3ac0d, verify+unblock f4b749da/c7cd5655):
   requeues summary (total, per-class with legacy facts normalized to
   `unknown`, resume-closeout parks, rate-limit/env-streak hint) in text
   and JSON (`cmd/tq/journalaudit.go`); the fact feed already carried
   `[class=…]`. The second window also unblocked the RED
   `TestAgentsDocSizeGuard` by pruning AGENTS.md to 14,639/15,000 B
   (commit 0dbc35e5).
6. **Three reviewer findings fixed, one commit each** (tasks
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
7. **internal/status ULID flake fixed and adjudicated** (immediately after
   this window, same queue arc, commits 93ef8642/de98ed96/2fa5376d):
   `task.NewID` made process-monotonic; TODO row 105 struck; the
   queue's automatic re-dispatch was adjudicated as done-on-arrival
   without a duplicate fix.

## b) PARTIALLY DONE

1. **The heal tool's self-test does not pin its three newest rails**: no
   tagged-range case (08-01 §c1), no metacharacter/empty-id refusal case
   (08-04 §e1), no multi-branch backup-resolution case (08-06 §e1). The
   refusals exist and were hand-probed, but nothing prevents silent rot.
2. **The daemon race is mopped, not fixed.** Every window in this pass
   lost the race at least once (4th–6th recorded instances); the heal
   tool + amend maneuvers recovered all of them, but the daemon-side
   short-delay/lockfile (00-27 §f3) is still unbuilt — this window is
   the strongest evidence yet that the mop does not scale.
3. **AGENTS.md budget headroom is ~360 B by design**: the doc-size guard
   went red twice in two days and each closeout that touches AGENTS.md
   re-risks it. The size-gate-at-edit-time script (07-53 §f6) is rowed,
   not built.
4. **ci-local was NOT green-run on any window's final tree** — one window
   was refused by the foreign-red preflight (a red master CI run older
   than the tree), the others ran the underlying gates directly. The
   pre-push gate is effectively unexercised on this lineage.
5. **Status-index live rows remain 157 > 100** — defensible KEEP-OPEN
   floor per the 06-44 report's convention analysis, but the bloat
   WARNING still fires by design and the digest row + eligibility-script
   levers (06-44 §f3/§f1) are both still unbuilt.

## c) NOT STARTED

1. **Worktree implementation itself** (`--worktrees`, 01-43 §f11–20) —
   correctly gated on the owner's Q1 merge-policy answer; only the
   question-routing landed.
2. **SQLITE_BUSY retry/backoff implementation** — the row is filed and
   BLOCKED on the owner production-vs-smoke ruling; no code.
3. **`scripts/check-archive-eligibility.sh`** — second full judgment
   sweep (06-44) has now run without it; the mechanical bar remains
   TODO.
4. **Daemon short-delay/lockfile** — carried across five windows now,
   never started.
5. **The 06-44 sweep's own improvement backlog** (spot-verify 10% of
   archive verdicts, digest row, stale-row hygiene ruling, threshold
   re-derivation) — none started.

## d) TOTALLY FUCKED UP

1. **The repo's own footer rule was violated by the tool author's first
   commit** — the heal enforcer's author placed `Task-Queue-ID:` ABOVE
   the attribution block (6e142726), the exact invisible-footer
   anti-pattern AGENTS.md documents; only the tool's own refusal caught
   it. Healed, but the pattern "write an enforcer, trip it immediately"
   repeated across the window (edit→battery→commit ordering lost to the
   daemon in four separate windows despite the AGENTS rule saying
   commit-first).
2. **One battery ran without `-race` and was briefly claimed as
   verified** (06-44 §d1); re-run with `-race` before anything shipped,
   no damage, but the claim order was wrong.
3. **A soft-reset fold consumed a daemon commit** (06-44 §d2): safe this
   time (verified exactly-mine, after the fact), aggressive by
   construction — the pre-reset `git diff --stat` evidence check is now
   a rowed convention candidate.
4. **Dispatch-text staleness nearly caused a duplicate build**: the
   …fb1a re-dispatch asserted "unshipped and unrouted" 14 minutes after
   the feature window had shipped it; only a first-tool-call
   `git log` check prevented re-implementing `requeueSummary`. Second
   same-hour instance of this class in recent history.
5. **Nothing is currently broken at HEAD**: root build+vet+`test -race`
   rc=0, doc gates green (re-verified this pass: `check-doc-refs.sh` ok —
   the `companion/conform` failure flagged by the 06-06 report has since
   healed), `heal-daemon-sweep.sh --self-test` 10/10, tree clean.

## e) WHAT WE SHOULD IMPROVE

1. **Commit-first is still not the default reflex** — four windows in one
   day re-learned it mid-session after a daemon sweep. The heal tool
   makes recovery cheap; the lockfile (or a pre-commit daemon pause)
   makes it not happen. File the lockfile as this window's top row.
2. **Every new rail needs a self-test case in the same commit** — all
   three review fixes shipped behavior that only manual probes verify.
   One bundle row covers the three missing cases.
3. **Dispatch claims need a freshness re-check as step zero** (grep
   `git log` + TODO row state before any other call) — cheap, and it
   just saved a full duplicate window.
4. **Sub-agent archive verdicts still ride unverified** — 113 `git mv`s
   on six agents' word; the 10% spot-check and the eligibility script
   are the two standing levers.
5. **The script-only change class has no mandated gate**: `bash -n` +
   self-test were self-chosen each time; the repo-wide
   `scripts/check-script-syntax.sh` gate exists but ci-local wiring for
   script-only windows is inconsistent (one window was refused by the
   foreign-red preflight instead).

## f) NEXT THINGS (impact-sorted; deduped against TODO_LIST at HEAD)

1. Daemon short-delay/lockfile: skip auto-commit of files touched in the
   last N seconds (00-27 §f3, sixth recorded race instance this window).
2. Wire `heal-daemon-sweep.sh --self-test` into ci-local as a hard step
   (07-20 §f2) so the refusal rails cannot rot silently.
3. Grow the heal self-test: tagged-range refusal, metacharacter/empty-id
   refusal, multi-branch backup-ref resolution (08-01 §f1, 08-04 §f1,
   08-06 §e1).
4. Diagnose/fix the pre-existing red master CI run that makes ci-local
   refuse every invocation until consciously bypassed (08-01 §b2/§f3).
5. Heal-script polish: `run_filter` failure-path stderr diagnostics +
   verify_heal check #5 comment naming the pre-flight tag rail (08-06
   §e2, 08-01 §e1).
6. Mint the worktree implementation slice rows (01-43 §f11–20) ready for
   the moment the owner answers Q1 (06-06 §f5).
7. `scripts/check-archive-eligibility.sh` — the mechanical archive bar,
   standing since before the 06-44 sweep (06-44 §f1).
8. Spot-verify a 10% sample of the 06-44 sweep's 85 ARCHIVE-DONE
   verdicts at HEAD; demote+restore any mis-archived report (06-44 §f2).
9. SQLITE_BUSY repro harness: run the smoke pair concurrently N times,
   count busy_timeout exhaustions, give the owner ruling data (06-57 §f3).
10. Add a `docs/status` digest row for the 06-44 sweep per the README's
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
  101–105 verified `[x]`); new rows appended in a dated section below.
