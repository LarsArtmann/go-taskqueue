# Done-Prompt Window Report — Doctor DLQ WARN, Daemon-Sweep Fallback Doc, Readmodel Lint Nits, API Poison Wire Pin, Review-Fix Index (five tasks, 2026-10-12 00:04–01:49)

Date: 2026-10-12 02:05 CEST. Window: five agent tasks closed between
2026-10-12 00:04 and 01:49 (four work dispatches + one review-fix ticket).
This report reflects on the window, feeds the backlog, and runs a focused
docs-health pass. Every claim below was re-verified at HEAD 56c8421c
(commit footers, test symbols, gate output, closeout reports) — nothing
invented.

---

## a) FULLY DONE

All five tasks landed with footer-carrying commits (verified: each carries
exactly its `Task-Queue-ID` as last trailer) and closed their TODO_LIST loop.

1. **Doctor WARN on stale legacy pre-fix DLQ sidecar** (task
   `000001a128a865…`, commits eb581e87 + 3540356d): `doctorProjectionDLQ`
   (cmd/tq/doctor.go:981) now warns "stale legacy sidecar — inspect then
   remove/migrate" when the pre-fix `<db>.dlq.db` sidecar exists AND the
   corrected projection-beside sidecar is absent; the legacy path surfaces
   as a checkResult Item. No auto-move. Pinned by
   `TestDoctorProjectionDLQLegacySidecar` (cmd/tq/doctor_dlq_test.go,
   verified present); cmd/tq devmod gate rc=0. TODO row ticked. CHANGELOG
   [Unreleased]/Added + FEATURES `tq doctor` row (line 101) both carry the
   2026-10-12 entry — docs consistent.
2. **Daemon-sweep manual fallback documented** (task `000001a12d046…`,
   commits 2e68f2a4 + 11454c13): AGENTS.md Known Issues now documents that
   `heal-daemon-sweep.sh` refuses a non-contiguous footer-less span and the
   sanctioned remedy is verify-exactly-mine + direct `git commit --amend`
   on the unpushed HEAD, with provenance pinned to task 000001a1288ceb2f.
   TODO row ticked in place (no reword). check-todo-list + check-doc-refs
   green; check-dead-sha-refs failure proven pre-existing (worktree diff at
   HEAD~1, zero delta).
3. **Readmodel schema-path lint nits** (task `000001a12d1221…`, commits
   fc9a5c2d + 0c7ba1b6): `ConvergeProjectionHome`/`probeMissingColumns`
   thread ctx (QueryRowContext/QueryContext, verified in
   internal/readmodel/schema.go) and promote `db.Close`/`rows.Close`
   errors to the returned error via named results. Scoped
   `-Enoctx,errcheck` lint on schema.go: zero findings (was 2+2).
   readmodel + composition per-module gates rc=0. CHANGELOG
   [Unreleased]/Fixed entry present. TODO row deleted per the
   done-items-are-deleted ruling; completion lives in CHANGELOG.
4. **API pump poison-DLQ wire pinned end-to-end** (task
   `000001a12d3c10…`, commits e1866f82 + d57d6bec):
   `TestUseReadModelDeadLettersPoisonThroughWiredStore`
   (internal/httpapi/httpapi_test.go, verified present) folds a
   corruption-class poison fact through the api-armed host, polls
   `srv.dlq.Count`, asserts stats stay served across the poison, and
   re-opens the derived sidecar file post-shutdown. Mutation-verified
   in-session (dropping the `DeadLetterStore` wire turns the test red in
   exactly the silent-downgrade mode). httpapi `-race -count=1` rc=0,
   vendor-hash check green. TODO row ticked with full DONE note. No
   CHANGELOG entry — test-only delta, per the closeout's own ruling.
5. **Review-fix: index the 00-04 closeout** (task `000001a12d40a4…`,
   commits 912cde9b + b6f9a99f): the rejected closeout's single finding
   (missing docs/status/README.md row) fixed — one row inserted in the top
   chronological cluster; `check-status-index.sh` rc=0 ("status index ok",
   verified this pass). Minimal one-file delta honored.

Also landed just outside the window (01:57, `bcce882d`): the
single_opener.md status flip (DESIGN → LANDED with f6492a00 evidence) —
its TODO row 53 is ticked; noted here for window continuity.

## b) PARTIALLY DONE

1. **The "poison invisible" class is closed end-to-end, but with soft
   edges** (from the 00-04 + 01-41 closeouts): the both-sidecars-present
   deployment shape still leaves legacy entries unmentioned in doctor
   output (rowed); the wire-pin's red path fails by 10s timeout instead of
   ~100ms marker poll (rowed 28); the mutation check was one-time, not a
   durable pin (question in §g); the poison fixture is a registered
   synthetic projection mirrored across two packages, not a real malformed
   journal fact (rowed below as fixture dedup).
2. **Legacy-sidecar detection lives hand-composed in cmd/tq**
   (`readmodel.DLQPathFor(d.DBPath)` to FIND the legacy file) — deliberate
   for the minimal delta, but it re-plants the wrong shape at a new call
   site; the readmodel-owned helper extraction is rowed.
3. **The lint cure is half-shipped**: schema.go is at zero under the
   scoped lint, but the lint baseline still carries the 4 now-fixed
   accepted entries (masks a reintroduction) and schema_test.go keeps
   noctx ×4 + errcheck ×3 — both rowed (regen + fix-or-policy).
4. **Verification scope per task was delta-scoped** (module gates, not
   full ci-local) — in-contract per the verify-window convention, but no
   full-battery claim is made for any window task.

## c) NOT STARTED

No window task skipped its own scope. The standing backlog the window
observed but did not touch (all pre-tracked): B10 pool-half detector
wiring; dead-sha-refs cure (still red at HEAD — re-verified this pass,
289+ dangling citations); projection-home postgres seam scoping;
postgres CLI store wiring; dispatch-prompting redesign (MAJOR OPEN
PROBLEMS); S2 journal unification; SystemNix deploy (owner sudo); push
master for green CI (owner button). Status-index bloat worsened: 299 live
rows vs threshold 100 (re-verified this pass).

## d) TOTALLY FUCKED UP

Nothing in the window destroyed work or landed a regression — honest
defect inventory:

1. **ci-local's dead-sha step is STILL red at HEAD for everyone**
   (carried from 00-15's finding, re-verified this pass: gate prints DEAD
   SHA rows incl. TODO_LIST.md:627/676/680/681). The cure row exists but
   is still unpaid work; every day of drift widens the mass behind which
   a real dead citation hides. This is the window's top systemic debt.
2. **Index bloat crossed to 299 live rows** (was 298 at 01:49). The
   archive-sweep row predates the window; every dispatch worsens it. The
   remedy (ANNOTATE sweep or digest row) still needs an owner pick
   between the two.
3. **Process debt repeated across three of five tasks**: the
   edit→commit→battery ordering rule was violated twice in-window
   (00-04's daemon sweep needed a `--from` heal; 01-00's needed a manual
   amend fold; 01-41's needed a soft-reset fold) — the rule works, the
   discipline keeps costing a red round each. Also: read-before-edit
   gate trips (×2) and check-status-index's `maxdepth 1` blind spot let
   the 00-04 report ship unindexed in the first place (the gate, not the
   agent, is the root cause — extension row exists).
4. **heal-daemon-sweep.sh has two live bugs/classes** now evidenced:
   non-contiguous refusal UX (rowed) and the span-terminator walk-past
   (footer commit at HEAD, rowed 29). The script is the busiest
   small tool in the repo and both its refusal paths have now misfired
   in production use.

## e) WHAT WE SHOULD IMPROVE

1. **Make "index row" a skeleton step** — the 01-49 fix proved the a)-g)
   closeout skeleton omits the one step the review gate checks; a row
   exists (add the step to AGENTS.md convention line).
2. **Dedupe the poison fixture**: `corruptionPoison` (readmodel
   host_test) and `apiPoison` (httpapi_test) are the same fixture twice —
   a shared helper keeps the corruption classification from drifting
   between the two pins (minted below).
3. **Commit-first discipline**: three in-window heal dances all trace to
   gates/tests run before the stage+commit. The behavioral fix is free;
   the alternative (more heal tooling) is being paid for already.
4. **Fast-fail red paths**: the wire pin's 10s timeout red is correct but
   slow; the WorkerFailed-marker poll remedy is rowed.
5. **Dead-sha gate failure summary header** (count + oldest date +
   baseline staleness) so "mass or mine" is one glance — rides the cure
   pass (already in the cure row's contract).

## f) UP TO 50 NEXT THINGS (top slice; deduped against the live 415-row TODO_LIST)

The window's closeouts minted their high-value rows already (dead-sha
cure, fast-fail wire pin, span-terminator, lint-baseline regen,
schema_test nits, postgres projection-home seam, LegacyDLQPathFor
extraction, both-sidecars doctor surfacing, heal UX, status-index
advisory mode, skeleton index-row step). Remaining dedup-checked items
appended to TODO_LIST this pass:

1. Opt-in `tq doctor --migrate-legacy-dlq` (copy entries from the legacy
   sidecar into the corrected one after snapshot backup, idempotent,
   refuse on count mismatch) — owner-gated: auto-move was explicitly
   rejected for the WARN task; a flag is a new decision.
2. Shared poison-projection test fixture between internal/readmodel and
   internal/httpapi.
3. Mutation-pin house style ruling (durable check-pin that wiring tests
   keep their red path, vs one-time in-session mutation verification).
4. Dead-sha arbitration ruling (fork-record pairs vs emit-baseline
   addendum; scope question: should the gate include immutable
   docs/status history at all, given the row-22 citation guard exempts it
   deliberately).

Beyond these, the highest-value open rows at HEAD (unchanged ranking):
dead-sha cure (§d1), B10 detector wiring, lint-baseline regen + test-file
nits, postgres projection-home seam, status-index archive/digest ruling,
both-sidecars doctor surfacing, LegacyDLQPathFor extraction, push master
(owner), SystemNix deploy (owner).

## g) QUESTIONS ONLY THE OWNER CAN ANSWER

1. **`--migrate-legacy-dlq`**: is an opt-in migration flag wanted at all,
   and if yes, must it dedup against entries already present in the
   corrected sidecar (disjoint-vs-overlapping fact sets in the wild
   decides)? (00-04 §g1/§e3.)
2. **Mutation-pin house style**: for wiring-critical tests like the api
   poison pin, is a durable gate that re-runs the mutation wanted, or is
   one-time in-session mutation verification the accepted standard?
   (01-41 §g2.)
3. **Dead-sha cure arbitration**: emit-baseline addendum vs mechanical
   fork-record pairs for the 289 dangling citations, and should the gate
   scope to living docs with the historical mass baselined once (the two
   citation guards currently disagree about docs/status)? (00-15 §g1/§g3.)

## h) BAND DRIFT

**None recorded.** `tq facts` over the live journal (grep for
`task.reprioritized`) returns zero facts — all-time, including this
window's span (2026-10-12 00:00–02:00). This re-confirms the 00-39
report's finding: the ADR-0015 reprioritize machinery has never fired on
the dogfood pool; the corresponding owner question (is the machinery dead
code in practice, or should one marker-row bump prove the pipeline?) is
already a BLOCKED TODO row.

---

## Docs-health pass (this run)

- **Living docs verified current**: CHANGELOG [Unreleased] carries both
  shipped code changes (doctor legacy-DLQ WARN; readmodel schema probe
  ctx + close errors); FEATURES `tq doctor` row 101 carries the
  2026-10-12 WARN (verified verbatim at HEAD); the api wire pin is
  test-only and correctly has NO CHANGELOG entry (ruling recorded in the
  01-41 closeout); README and ROADMAP unaffected by this window's delta.
- **Historical reports**: the recent cluster (2026-10-11 23-58, 2026-10-12
  00-39) had its forward items already harvested into TODO_LIST (verified:
  B10 wiring, replay-guard warn, live-open probe, freeze sweep, prompt-lag
  question all rowed); no report in the 2026-10 cluster is fully resolved,
  so nothing qualifies for `archived/` this pass (archive bar: every item
  struck). No rewrites, no annotations forced.
- **Gates re-run this pass**: check-status-index ok (299 live rows,
  bloat warning standing); check-dead-sha-refs red (pre-existing, cure
  rowed). Index bloat and dead-sha remain the two standing doc-health
  debts, both rowed.
- **TODO_LIST**: dedup-checked before appending (415 unchecked rows at
  read time); 4 new items added (one owner-blocked + one blocked ruling +
  one fixture dedup + one style ruling); no existing item reworded or
  deleted; the window's own ticks (doctor WARN, daemon-sweep fallback,
  wire pin, single_opener flip) verified already in place.
