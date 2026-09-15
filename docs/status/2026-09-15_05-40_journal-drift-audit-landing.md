# Journal-Drift Audit Landing — Session Status

**Date:** 2026-09-15 05:40 CEST
**Scope:** this session's run (one interactive window) + what it observed on master. Other windows' work is credited as observed, not claimed.
**Task lane:** TODO_LIST "High Impact" row — journal-drift audit (`tq audit --journal`), 2026-09-14 §g2/§g3 rulings (hermetic fixtures only; operator command; advisory smoke, never a hard gate).

**Session-shape honesty up front:** this window did NOT land alone. A concurrent
pool/interactive session built the audit engine, the smoke, the CHANGELOG entry,
and the TODO harvest ON TOP of this window's backend foundation, while this
window built the foundation ON TOP of theirs. The final master state is the
union; master tests green. One real collision occurred (§d1) and was caught by
tooling, not by me.

---

## a) FULLY DONE

1. **Four-dimension journal-drift audit shipped end-to-end.** `tq audit
   --journal` replays the fact journal and diffs status, attempts, priority,
   and dedup key against the tasks-table projection; text + `--json` output;
   read-only, no repair mode; advisory footer points at `tq show`/`tq facts
   -detail`. Evidence: `scripts/smoke/journal-drift.sh` PASS (all three
   phases: truthful lifecycle → no drift; seeded drift → all four fields
   surface; JSON machine-readable); `scripts/test-cmd-tq.sh` green
   (build+vet+full suite, 31.7s). Files: `cmd/tq/journalaudit.go`,
   `cmd/tq/journalaudit_test.go`, `cmd/tq/audit.go` (flag, pre-existing).

2. **Journal enrichment makes priority + dedup key derivable going forward.**
   New `queue.EnqueueDetail` contract type (snake_case wire tags, `Priority
   *int` so priority 0 stays distinguishable from a legacy thin fact); plain
   enqueue facts in BOTH backends now carry `project`, `type`, `priority`,
   `dedup_key`; RescueDead re-emits `{"rescue":"true"}` through the same
   type (byte-identical JSON to the old map). Facade alias added (queue/
   queue.go:19, by the concurrent session). Conformance-safe: no test pinned
   the old detail shape. Files: `internal/queue/queue.go`,
   `internal/queue/sqlite/sqlite.go:309-318,1068-1073`,
   `internal/queue/postgres/postgres.go:333-342,958-962`.

3. **`task.Task.DedupKey` projection field** — the row projection can now be
   diffed on dedup key at all (the Go struct previously couldn't see the
   column). Threaded through both backends: sqlite SELECT lists (List +
   loadTaskTx) + `scanTaskRow`; postgres `taskColumns` + `scanPGTask`; the
   postgres List loop's duplicated inline scan was collapsed into
   `scanPGTask` (one scan to maintain, not two). Round-trip covered by the
   seeded-drift test.

4. **Postgres RescueDead fact parity fix (ADR-0007).** Postgres emitted
   `task.requeued` (attempts unchanged) where sqlite re-emits a rescue
   `task.enqueued` (attempts reset to 0) — a divergence that would have made
   the audit report false drift on every rescued postgres task. Aligned
   postgres to the sqlite shape. This was found by replaying the fact table
   against the audit's semantics, not by any backend test — see §e4.

5. **Rescue-attempts replay bug fixed.** The concurrent session's engine
   never reset attempts to 0 on the rescue-enqueued fact → every RescueDead
   on sqlite would report false drift. Fixed in `replayProjection` + two new
   tests: `TestReplayProjectionRescueResetsAttempts` (unit, hand-seeded
   facts) and `TestJournalDriftNoDriftAfterRescue` (real store: enqueue
   MaxAttempts 1 → Fail → dead → RescueDead → no drift).

6. **go-directive drift repaired (three leaf modules).** `internal/journal`,
   `internal/task`, and `task/` (facade) had committed `go 1.26` vs the
   root's `go 1.26.7` — exactly the 2026-09-12 red-master failure class
   (zero-dep leaves have no floor). Restored via `go mod edit -go=1.26.7`;
   `check-go-mods.sh` exit 0. NOT my drift (committed before this session;
   introduction point not chased down) — but it was red while per-module
   tests stayed green, which is why it survived; see §d5.

7. **Gates run this session (all green at finish):** per-module
   build/vet/test-race for task, queue, sqlite, postgres (env-gated skip
   locally), journal; cmd/tq gate + `CMD_TQ_OS=windows` cross-compile;
   `check-facade-parity.sh` OK (7 facades); `check-go-mods.sh` exit 0;
   gofmt clean on all touched dirs; journal-drift smoke PASS. Hermetic rule
   honored: the production journal was never opened — scratch `t.TempDir()`
   DBs only, and the smoke builds its own fixture.

---

## b) PARTIALLY DONE

1. **Lint exposure unmeasured.** I touched ~8 files across 5 modules and
   never ran golangci-lint on any of them. The baseline-growth gate
   (`lint-baseline.sh --check`, 500 findings / 87 rows) only runs at
   ci-local; if my new code added findings (new struct, new switch cases in
   cmd/tq), that surfaces as a red gate I haven't seen. Open, effort S: run
   `golangci-lint run` per touched module and compare against baseline.

2. **No backend-level pins for the NEW fact shapes.** The enriched enqueue
   detail and the rescue-fact type are exercised only through cmd/tq tests
   and the smoke. The mirrored conformance suites (ADR-0007) do not yet pin
   "enqueue fact detail carries priority+dedup_key" or "RescueDead emits
   rescue-enqueued, not requeued". A future refactor could silently revert
   either and cmd/tq tests would only catch it indirectly. Open, effort S/M:
   one pin per backend suite.

3. **Full ci-local.sh never run this session.** Targeted gates only (listed
   in a7). The pre-push gate — including the advisory journal-drift smoke
   step, lint baseline, and CI-state check — is still the judge. Effort M
   (runtime), zero code.

4. **nix build unverified after the go-directive edits.** No requires changed,
   so vendorHash should not move — but "should" is not a gate. Effort S:
   `nix build`.

5. **Coverage dimension dropped by accident.** My draft design reported
   per-field coverage counts (how many tasks the journal could re-derive
   priority/dedup key for) so operators understand sparse checks on legacy
   journals. When I adopted the concurrent session's flat-DriftRow engine,
   the idea silently evaporated instead of being decided on. The output
   today would show few/no priority+dedupKey rows on the production
   journal's pre-enrichment facts with no explanation. Open decision: add
   `coverage` to DriftReport (additive JSON), effort S.

6. **AGENTS.md not updated for the enriched fact shape.** The payload-
   contracts section is now the doc of record for fact detail shapes and
   doesn't mention enqueue facts carrying priority/dedup_key, nor the
   postgres rescue-parity fix. docs-health HARVEST material, effort S.

7. **Observed, not mine, mid-flight:** the dep-sweep feature sits UNCOMMITTED
   in the worktree (`cmd/tq/agentpool.go`, `cmd/tq/main.go`,
   `internal/executor/depbump.go` — depbump pool registration + sweeper
   wiring). The owning window's lap is not finished; its CI parity and its
   interaction with this window's cmd/tq changes are unverified. Listed
   here because master CI judges BOTH windows' union.

---

## c) NOT STARTED

1. **Drift-audit `--repair` mode** (rebuild the table from the journal) —
   ROADMAP brainstorm item 15 from the 2026-09-13 source report; the read-
   only audit is its prerequisite and now exists. Deliberately not built:
   the row says "no repair mode yet" and repair is the dangerous half.

2. **Release sub-tag set for this session's module changes.** task, queue,
   sqlite, postgres (+ their facades) and cmd/tq all gained code; the next
   `release.sh` run needs the sub-tags PRE-CUT before its gates (documented
   release flow). Not started — see §g2, it's a cadence decision.

3. **Standing TODO rows untouched by this window** (observed, not started
   here): postgres session bridge (`AppendFact` on postgres + conformance),
   `internal/consumer` wire-or-delete, executor usage `tokens` parsing,
   `--prioritize` pool enablement, SystemNix deploy/validation rows,
   journal/cqrs facade ruling. No work was begun; they remain pool food.

4. **Mint-time done-check** (the recurring "paid re-dispatch of DONE tasks"
   loop every recent report flags) — still unbuilt; this window neither
   suffered nor fixed it, but its report joins the pile asking for it.

---

## d) TOTALLY FUCKED UP

1. **I attempted a blind full-file `write` over `journalaudit.go` that I had
   read ~20 minutes earlier — and a concurrent session had landed a complete,
   committed engine in that file in between.** The write guard rejected it
   ("file modified since last read"). If the guard had not existed I would
   have clobbered their committed implementation with my parallel draft —
   the daemon would have swept it onto master and the union's tests would
   have caught it only partially (their engine referenced my backend types,
   my draft didn't reference their tests). Root cause: I violated the
   repo's own hot-file rule ("in hot files re-read immediately before every
   write") because the file "was mine" in my head — it never was; two
   sessions were building the same TODO row simultaneously. Cost: one
   wasted full-file draft. Severity: process failure with near-miss data
   loss; mitigation worked (guard + never-revert + build-on).

2. **I nearly shipped a false completeness claim.** Mid-session I framed the
   smoke as a "deliberate skip" (ruling says advisory → I read that as
   optional → planned to report it as such). The concurrent session built
   it, and it is the ONLY end-to-end verification through the real tq
   binary — my framing would have undersold the feature's verification
   story and, had I been the only window, the row's intent would have
   landed half-built. Ruling language lesson: "advisory, never a hard gate"
   constrains GATING, not whether to build it.

3. **The lint rule was silently unenforced by me.** AGENTS.md: "don't add
   new findings in functions you touch." I cannot know whether I did,
   because I never ran the linter. That is exactly the shape of the
   "gate-that-lies by absence" pattern this repo keeps documenting — the
   gate didn't lie, I just never ran it. See b1.

4. **The postgres parity bug survived every existing backend test.** Neither
   the sqlite suite nor the postgres conformance suite pins RescueDead's
   fact — the divergence between the two backends (a factual wire-shape
   difference in the journal stream) was invisible to ADR-0007's mirror
   machinery until an audit CONSUMER needed the semantics. This is not my
   bug, but my fix's only guard is one cmd/tq test; §b2 is the debt.

5. **My verification order hid a red gate for most of the session.** I ran
   deep gates (per-module tests, race, windows cross-compile) before the
   cheap cross-cutting one (`check-go-mods.sh`) — which was red the whole
   time on the go-directive drift. Cheap broad gates first, deep gates
   second; the order I used could have shipped work validated against a
   known-bad module graph.

6. **Not fucked up but embarrassing:** my first cmd/tq gate run failed on
   `internal/executor` (another window's mid-refactor `runVerify` signature
   change). I first assumed transient, waited, re-checked — correct call,
   self-healed in ~2 min. Zero action taken on their in-flight code; noted
   because "wait and re-verify" beat "fix their call site" here, and that
   instinct should be the documented default for observed breakage in
   files another window owns.

---

## e) WHAT WE SHOULD IMPROVE

1. **Hot-file re-read must be mechanical, not remembered.** Impact: one
   near-clobber this session; the repo has warned about concurrent agents
   since v0.1. Suggested fix: never `write` a whole file in this repo; use
   targeted `edit`/`multiedit` with fresh reads (smaller collision surface,
   honest merge), and treat any read older than ~2 minutes in cmd/ or
   internal/executor/ as stale.

2. **Gate ordering: broad-and-cheap before deep.** Impact: §d5 — a red
   module-health gate coexisted with an hour of green deep runs. Suggested
   fix: session-start ritual should include `check-go-mods.sh` +
   `check-facade-parity.sh` (both <10s) before any editing; ci-local order
   already does this right for pushes.

3. **Fact-shape changes must land WITH their backend pins.** Impact: §d4 —
   backend parity diverged and no suite noticed. Suggested fix: a rule in
   AGENTS.md next to the conformance-suite note: "a change to any fact's
   detail/type updates the sqlite store_test AND postgres conformance pins
   in the same change," so the mirror machinery guards wire shapes, not
   just claim SQL.

4. **Audit-consumer tests are parity tests in disguise.** The drift audit
   found a backend divergence no mirror test knew to look for. Suggested
   fix: when a new consumer derives semantics from facts, add its
   invariants (rescue resets attempts; requeue doesn't) to the conformance
   suites so BOTH backends prove the same replay story.

5. **"Advisory" needs a vocabulary fix in rulings.** Impact: §d2 — "advisory"
   was one misread away from an unbuilt smoke. Suggested fix: rulings in
   TODO rows should say "advisory (build it; must not gate)" vs "optional"
   — two words, no ambiguity, benefits every future agent row.

6. **Parallel-window same-row work needs a claiming convention.** Two
   windows built the same feature from the same row and only tooling
   prevented data loss. Suggested fix: before starting a TODO row, `git
   log --since='30 minutes ago' -- <predicted files>`; if another window
   is mid-flight, build on the landed skeleton instead of drafting in
   parallel (exactly what the second half of this session did right).

7. **Adopted engines deserve a deliberate feature-port pass.** When my draft
   lost to the landed engine, its good ideas should have been triaged one
   by one (coverage counts lost by accident, §b5). Suggested fix: after any
   "adapt, don't overwrite" pivot, list the draft's unique ideas and
   decide keep/drop explicitly in the close-out.

8. **Postgres CI variant remains the only exerciser of the postgres store
   changes.** The env-gated suite skipped locally (no TQ_TEST_POSTGRES), so
   this session's postgres edits (scan changes, taskColumns, rescue fact)
   are verified by compile + mirror discipline + whatever CI runs. The
   05-13 report already flagged CI test-postgres as the verifier for the
   fullcore work — same ride, same risk. No new action beyond watching the
   next CI run.

---

## f) TOP 35 THINGS WE SHOULD GET DONE NEXT

(Honest brainstorm, ranked by impact; HARVEST should route items 1-14 to
TODO_LIST and the rest to ROADMAP. Padding to 50 rejected — the remaining
15 slots would be filler.)

| # | Task | Impact | Effort | Category |
|---|------|--------|--------|----------|
| 1 | Run `golangci-lint run` on all 5 touched modules; fix or justify any finding vs `.golangci-baseline.txt` before ci-local surprises us | Critical | S | Quality |
| 2 | Run full `./scripts/ci-local.sh` (the pre-push gate) over the union state | Critical | M | Process |
| 3 | Add conformance pins: enqueue fact detail carries priority+dedup_key; RescueDead emits rescue-enqueued (both backends) | High | S/M | Quality |
| 4 | Land the release sub-tag set for task/queue/sqlite/postgres/facades/cmd-tq before the next release.sh (gates demand pre-cut tags) | High | M | Process |
| 5 | `nix build` after the go-directive fixes to confirm vendorHash did not move | High | S | Quality |
| 6 | Add `coverage` (per-field compared counts) to DriftReport so legacy thin facts' sparse priority/dedupKey checks are explained in output | High | S | Feature |
| 7 | Watch next CI run for the postgres-gated job as the real verifier of this session's postgres edits (rides the fullcore postgres variant) | High | S | Process |
| 8 | Update AGENTS.md payload-contracts: enqueue fact detail shape + postgres rescue parity + audit's derivability rules (what makes a field "unknown") | High | S | Documentation |
| 9 | Mint-time done-check in the harvester/pool: skip dispatch when the row is already `[x]` (kills the paid re-dispatch loop flagged by 4+ reports) | High | M | Feature |
| 10 | Land `--repair` design doc BEFORE any repair code: replay-vs-table conflict policy, idempotency, who may run it (owner-gated) | High | S | Documentation |
| 11 | Verify/finish the concurrent window's dep-sweep work sitting uncommitted in the worktree (agentpool.go/main.go/depbump.go) — CI parity + flags + docs | High | M | Feature |
| 12 | Extend the drift smoke's seeded-drift fixture with a rescue scenario (rescue + corrupt → detector must stay correct on the reset path) | Medium | S | Quality |
| 13 | Add a requeue-vs-rescue conformance assertion (requeue keeps attempts; rescue resets) so the replay story is pinned at the store layer | Medium | S | Quality |
| 14 | Fold §e1/e2/e3 into AGENTS.md session-ritual + conformance sections (hot-file re-read; broad gates first; fact-pins rule) | Medium | S | Documentation |
| 15 | Fuzz the new `EnqueueDetail` parse path (`replayProjection` unmarshals attacker-shaped journal bytes if a journal is ever adversarial) — one FuzzParseRepo-style campaign | Medium | S | Quality |
| 16 | `tq facts` renderers: surface the new priority/dedup_key keys in fact detail views (webui + CLI) so the enrichment is visible, not just machine-read | Medium | S | Feature |
| 17 | Journal-drift smoke into ci.yml (currently ci-local-only; the guard-wiring gate accepts either) — advisory, same O5 caveat | Medium | S | Process |
| 18 | Domain language: does `replay`/`projection`/`coverage` belong in docs/DOMAIN_LANGUAGE.md? The audit just gave the terms operational meaning | Medium | S | Documentation |
| 19 | Consider `dedup_key` on the webui task detail (field exists in Task now; detail page doesn't show it) | Medium | S | Feature |
| 20 | Baseline regen policy check: does the audit's new cmd/tq code change the per-module lint rows? Fold into #1 | Medium | S | Quality |
| 21 | Postgres session bridge row (standing TODO): still open, still the biggest postgres parity gap | Medium | L | Feature |
| 22 | `internal/consumer` wire-or-delete decision (standing TODO row 27): the drift audit is now a second potential consumer — decide before it grows a third | Medium | M | Cleanup |
| 23 | Executor usage parsing (`tokens` field, standing TODO row 124): derived-outcomes work left it always-empty | Medium | M | Feature |
| 24 | `--prioritize` pool enablement (standing TODO row 129) — gated on scorer cost measurement, unchanged | Medium | S | Feature |
| 25 | Batched-harvest interaction note: batch tasks mint BEFORE this enrichment — their enqueue facts carry priority+dedup_key like everything else; verify one live batch task audits clean | Medium | S | Quality |
| 26 | SystemNix pool validation rows (standing TODOs 22/157): owner-gated, unchanged | Medium | S | Process |
| 27 | ADR for the audit? Ruling says operator command, no policy growth — write the one-paragraph decision record only if --repair revives the gate-vs-advisory question (source report brainstorm 18) | Low | S | Documentation |
| 28 | journal/cqrs facade ruling (standing TODO row 127): ADR-0016 surface question, unchanged by this session | Low | S | Decision |
| 29 | Check whether `tq audit --journal` should accept `--project`/`--status` filters (List already supports them; full-table scan is fine at current scale) | Low | S | Feature |
| 30 | Rename-hygiene/self-review scans on the new code paths (the repo's own new scanners should eat their own dogfood: run check-rename-hygiene.sh scoped to this diff) | Low | S | Quality |
| 31 | Docs: one FEATURES.md row for the audit under a "operator tools" grouping if FEATURES groups that way (check format first) | Low | S | Documentation |
| 32 | Consider exposing `FactsForTask` in the drift output when drift exists (auto-attach the task's fact trail to the advisory line) | Low | M | Feature |
| 33 | Delete my abandoned draft's unique-but-rejected ideas explicitly in the next docs-health pass (coverage → item 6 keeps it; pointer fields in replayState → dropped, note why) | Low | S | Cleanup |
| 34 | Enrichment backfill consideration: NEVER rewrite history (policy), but document that pre-enrichment journals permanently lack priority/dedupKey derivability — sets expectations for §6's coverage numbers | Low | S | Documentation |
| 35 | Kill zero TODO_LIST rows this window — if HARVEST runs, close the loop on items 1-14 within the same session, not "later" | Medium | S | Process |

---

## g) THREE QUESTIONS I CANNOT ANSWER MYSELF

1. **Do you ratify the postgres RescueDead fact alignment?** I unilaterally
   changed postgres's rescue fact from `task.requeued` to a rescue
   `task.enqueued` (sqlite's existing shape) because the audit's replay
   semantics require "rescue resets attempts" to be a fact. I verified no
   conformance test pinned the old shape and the dogfood journal is sqlite,
   so no live data depends on the old postgres shape — but I cannot know
   whether any external postgres deployment (or your future migration plan)
   treats `task.requeued` on rescue as load-bearing. Keep aligned (my
   change stands) or do you want postgres to keep the requeued fact and
   the audit to learn a postgres-specific replay rule instead?

2. **Release cadence for this session's module changes:** cut the sub-tag
   set now as a v0.3.x patch wave (task, queue, sqlite, postgres + facades
   + cmd/tq all changed; release gates need pre-cut sub-tags), or batch
   with the in-flight dep-sweep and batched-harvest work into v0.4.0? I
   tried to infer the answer from VERSION-SURFACES.md and the release doc
   (both describe mechanics, not cadence policy) — the call is yours
   because it decides whether the enrichment ships to `go install` users
   before or with the bigger features.

3. **Should legacy journals get an explicit "unenriched" disclosure in the
   audit output** (the §b5/§f6 coverage counts), or is silence acceptable
   ("fields simply aren't compared when unknown")? I tried to answer it
   from the operator's seat — on the production journal, every task
   enqueued before this session will show priority/dedupKey coverage of
   zero forever — but whether that noise (honesty about limits) or silence
   (cleaner output) fits how you actually run `tq audit --journal` is a
   preference I cannot derive from the rulings.

---

*Recorded 2026-09-15 05:40 CEST. Format note: user explicitly requested
`.md`; the status-report skill's canonical format is a styled HTML
dashboard — override honored, flagged here per skill contract, not
propagated back as a default. §f feeds docs-health HARVEST (items 1-14 →
TODO_LIST, rest → ROADMAP); if the session continues without a harvest,
run it before closing. Waiting for instructions.*

---

## Close-out (2026-09-15, later session) — the two named gaps are closed

Both gaps this report named are now landed and gated:

1. **Coverage counts shipped** (§b5/§f6): `DriftReport.Coverage`
   (additive `coverage` JSON field + a text line) counts, per diffed
   field, how many compared tasks the journal could actually verify —
   legacy thin-fact tasks consume no priority/dedup coverage and the
   counts say so, which also answers question 3 below by default: the
   disclosure exists; silencing it is now a one-line owner preference.
   Pinned by `TestDiffProjectionCoverageSkipsLegacyThinFacts` (pure
   diff function, extracted for exactly this testability) plus coverage
   assertions in the rescue and seeded-drift store tests.
2. **Backend fact-shape pins shipped** (ADR-0007/0012 same-change rule):
   sqlite `TestEnqueueFactDetailCarriesIdentity` +
   `TestRescueDeadEmitsRescueEnqueue`; postgres mirrors both as
   conformance-battery subtests, verified against a live throwaway
   postgres container on this host, not just compile-skipped locally.

Also this session, root-cause fixes for the red master CI run (02:50,
commit 534d088 — both failure classes predate today's tree):

- **vendorHash refreshed** to the real FOD hash after the
  dependabot-driven graph drift (go-retry v0.6.0, templ-components
  v1.17.0).
- **cmd/tq committed pins bumped** to the same versions: the nix build
  assembles cmd/tq's replace graph WITHOUT the devmod shim's tidy step,
  so stale committed requires fail hermetically with a terse
  "updates to go.mod needed" (reproduced and verified fixed in a
  no-tidy replica before touching the flake). go.sum gained only the
  new versions' entries; the internal-module v0.3.0 sums the proxy
  install path needs are untouched.
- **`.golangci-baseline.txt` one-row repair**: the 06:03 regen recorded
  sqlite varnamelen 32 from a partial lint load; every stable run of
  unchanged code yields 33. Corrected that row only — the executor rows
  that differ under the CURRENT worktree belong to the in-flight
  dep-sweep window and were deliberately NOT re-baselined.
- Formatter pass (treefmt) picked up an import-ordering slip in the
  coverage test and the dep-sweep window's `sweep_test.go`.

Verification lap: `./scripts/test-cmd-tq.sh` (incl. windows
cross-compile) green, sqlite/postgres/queue/task module suites green
(postgres against the live container), `check-facade-parity.sh` +
`check-go-mods.sh` green, `lint-baseline.sh --check` green (growth
owned, shrink advisory), `journal-drift.sh` smoke PASS, full
`CI_CHECK=off ./scripts/ci-local.sh` = ALL GATES GREEN, `nix build`
produces `tq 0.3.0, go1.26.7-X:jsonv2`. The three §g owner questions
stand except that question 3 is now answered in code by default.
