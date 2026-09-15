# Journal-Drift Audit: Gaps Closed + Red-Master Healed — Session Report

**Window:** 2026-09-15, ~05:45–07:00 CEST (continuation of the 05:40
`journal-drift-audit-landing` session; same feature, second lap)
**Scope:** the two named gaps of the landing report (coverage counts,
backend conformance pins), the gates that exposed them, and two
root-cause fixes for the red master CI run — nothing else researched.
**Verdict:** the journal-drift audit TODO row is now functionally
COMPLETE end-to-end and the exact tree passes the full ci-local
replicant with ALL GATES GREEN. Master on origin is still red (pushes
are owner-run); every failure cause on the pushed tip is fixed locally.

---

## a) FULLY DONE

1. **Per-field coverage counts** (`DriftReport.Coverage`, additive
   `coverage` JSON field + a rendered text line,
   `cmd/tq/journalaudit.go:152-166`): counts per diffed field how many
   compared tasks the journal could actually verify. Legacy thin-fact
   tasks consume no priority/dedup coverage — sparseness now says
   "nothing to diff against" instead of being silently folded into a
   clean-looking report.
2. **Pure diff extraction** (`diffProjection`,
   `cmd/tq/journalaudit.go:168`): the store-diff loop moved out of
   `journalDrift` into a pure function, extracted specifically so the
   legacy/thin-fact behavior is testable without a store.
3. **Coverage tests**: `TestDiffProjectionCoverageSkipsLegacyThinFacts`
   (thin fact + ghost row + enriched task in one fixture), coverage
   assertions added to `TestJournalDriftNoDriftAfterRescue` (proves
   explicit-zero priority is covered while the empty dedup key is
   honestly NOT) and `TestJournalDriftSeededDriftAllFields` (all four
   fields diffable despite all four drifting).
4. **sqlite fact-shape pins** (ADR-0007 same-change rule):
   `TestEnqueueFactDetailCarriesIdentity` (detail carries
   project/type/dedup key AND priority as an EXPLICIT &0 — a thin detail
   would silently downgrade audit coverage) and
   `TestRescueDeadEmitsRescueEnqueue` (rescue re-emits `task.enqueued`
   with the rescue marker, not `task.requeued`) —
   `internal/queue/sqlite/store_test.go`.
5. **postgres fact-shape pins, verified against a LIVE database**: both
   pins mirrored as conformance-battery subtests
   (`internal/queue/postgres/conformance_test.go`), executed via a
   throwaway `postgres:16-alpine` docker container on this host
   (TQ_TEST_POSTGRES) — full battery green, then container destroyed.
   Not compile-skipped: actually ran.
6. **Windows cross-compile gate** (`CMD_TQ_OS=windows
   ./scripts/test-cmd-tq.sh`) green for the coverage changes.
7. **Red-master cause #1 — vendorHash**: refreshed to the real FOD hash
   after dependabot-driven graph drift (go-retry v0.5.0→v0.6.0,
   templ-components v1.16.0→v1.17.0). `flake.nix:40`.
8. **Red-master cause #2 — cmd/tq stale committed pins**: discovered the
   nix build assembles cmd/tq's replace graph WITHOUT the devmod shim's
   tidy step, so internal modules' new dep versions fail hermetically
   with a terse two-line "updates to go.mod needed". Reproduced in a
   no-tidy local replica, bumped cmd/tq's committed requires to the
   graph versions, refreshed go.sum via targeted `go mod download` only
   (the internal-module v0.3.0 sums the proxy-install path needs are
   untouched), re-verified in the replica BEFORE re-running nix.
9. **nix build green end-to-end**: `tq 0.3.0, go1.26.7-X:jsonv2`
   produced from `result/bin/tq`.
10. **Lint baseline gate green** with a documented one-row measurement
    repair (see §d2): `lint-baseline.sh --check` → "within baseline
    (1087 findings vs baseline 1090)"; growth owned, shrink advisory
    (the executor golines 4→3 shrink belongs to the in-flight dep-sweep
    window and was deliberately NOT re-baselined).
11. **Full `CI_CHECK=off ./scripts/ci-local.sh` → ALL GATES GREEN**
    ("this exact tree is what CI will see. Safe to push."). CI_CHECK=off
    is the documented conscious bypass: master is red on the OLD pushed
    tip; both causes are fixed locally; the auto-commit daemon never
    pushes, so check-ci cannot go green from here.
12. **Per-module gates re-verified**: sqlite/postgres/queue/task suites,
    `check-facade-parity.sh` (7 facades), `check-go-mods.sh`,
    `journal-drift.sh` smoke PASS (its greps are additive-safe against
    the new coverage line).
13. **Docs closed out**: CHANGELOG journal-drift entry extended with the
    coverage semantics + the both-backend pins; close-out addendum
    appended to `2026-09-15_05-40_journal-drift-audit-landing.md`
    (answers its §b gaps and its question 3 by default); formatter made
    idempotent (0 changed on re-run); `check-status-index.sh` ok.
14. **Session discipline held**: zero contact with the dep-sweep window's
    in-flight files beyond the treefmt pass their own gate demanded;
    production journal never touched; all scratch DBs in /tmp with
    explicit `--db`; §g owner questions from the landing report NOT
    unilaterally resolved (question 3 is answered in code as the default
    the owner can now simply keep or quiet).

## b) PARTIALLY DONE

1. **The lint flake is repaired, not root-caused.** Gate run #1 reported
   `journal` + `queue/postgres` `typecheck: NEW` classes that vanished
   in every manual re-run; the 06:03 regen (committed by another window,
   `2bbb37b`) recorded sqlite varnamelen 32 while every stable run of
   unchanged code yields 33. I repaired the row surgically, but the
   underlying intermittent partial-load (my top suspect: golangci-lint
   v2.13.2 is BUILT WITH go1.27.1 while the tree targets go 1.26.7 with
   GOEXPERIMENT-gated `encoding/json/jsontext` imports) is unconfirmed
   and unfixed. A gate that intermittently lies in BOTH directions
   (false growth, poisoned regens) remains the biggest open risk.
2. **The audit row is code-complete but unshipped**: release cadence
   (sub-tag wave vs v0.4.0 batch) is an owner question, so no tags are
   cut and `go install` users still get the pre-enrichment audit.
3. **Master on origin is still red** (02:50 run, commit 534d088). All
   failure causes are fixed and verified locally; the fix train
   (go 1.26.7 directive repair from the landing session + vendorHash +
   cmd/tq pins) reaches master only when the owner pushes.
4. **This report's §f list is a brainstorm, not a backlog** — HARVEST
   into TODO_LIST/ROADMAP has NOT run (waiting for instructions, per
   this window's contract).
5. **Landing report's §f items 15–35** (routing: ROADMAP) untouched this
   window; items 1–14 were the prior window's HARVEST obligation and I
   did not verify whether that harvest fully landed in TODO_LIST.
6. **Status index bloat**: 136 live rows (threshold 100) —
   `check-status-index.sh` passes but warns; archive sweep pending.

## c) NOT STARTED

1. Archive sweep / digest-row restructuring for `docs/status/README.md`
   (docs-health ANNOTATE).
2. Release mechanics for this session's changes: pre-cut sub-tags, two
   phase --tag/--push, VERSION-SURFACES ordering — blocked on owner Q2.
3. Post-release verification lap (module proxy propagation, pkg.go.dev,
   `go install …/cmd/tq@vX` smoke) — blocked on Q2.
4. Any postgres-specific replay rule IF the owner rejects the rescue-
   enqueue alignment (Q1) — currently moot because the alignment stands.
5. Upstream crush issue for the misleading "does not support reasoning
   effort" error (AGENTS-flagged candidate; needs verify-before-filing +
   github-voice) — untouched this window.
6. A local convenience script wrapping the postgres conformance docker
   invocation (I used a hand-rolled container; the recipe is only in
   this report and my shell history).
7. Verification that the CI `test-postgres` job picks up the two new
   subtests on the next push (they live inside `TestPostgresConformance`,
   which the job `-run TestPostgres`-catches — expected fine, unverified
   on a real run).
8. Fuzz coverage for `replayProjection`'s detail unmarshal path
   (malformed `jsontext.Value` details currently silently degrade to
   "unknown" — honest but unmeasured; see §f35).

## d) TOTALLY FUCKED UP

1. **I wrote a replay-semantics test fixture from MEMORY instead of from
   the store code — and it was wrong twice.** My first
   `TestDiffProjectionCoverageSkipsLegacyThinFacts` gave the legacy task
   `attempts: 2` with no failed facts (an IMPOSSIBLE state: the store
   burns attempts only on Failed/DeadLettered, claims never do — I went
   and read `ClaimDue`/`Fail` in sqlite.go only AFTER the gate caught
   it), and asserted zero drift while the ghost row is BY DESIGN a drift
   row. The gate failure was my test, never the engine. Cost: one red
   gate lap and the exact class of sloppiness the repo's
   "claims carry citations" rule exists to prevent.
2. **I nearly shipped a wrong lint-baseline regen on a miscount.** My
   grep counted golangci's `* varnamelen: 33` summary line as a finding
   (34 vs 33) and I briefly treated the committed 32 vs my 34 as a
   two-finding drift; only the deliberate two-run diff (STABLE: 34 lines
   = 33 findings + summary) exposed it. I then made it worse-in-idea by
   running a FULL regen whose diff baked the dep-sweep window's
   UNCOMMITTED worktree state into executor rows — reverted to the
   committed file and applied a one-row sed instead. The correct
   first move (snapshot, surgical row fix, verify --check) was my THIRD
   move.
3. **The 06:03 baseline regen (another window, daemon-swept) recorded
   counts from partial lint loads and got committed.** The gate then
   failed for honest runs on unchanged code. Not my commit — but I
   passed through that state twice (two failing gate runs) before
   treating "the baseline itself is the bug" as a hypothesis worth
   testing.
4. **Two wasted round trips on tooling ritual**: a `multiedit` with an
   empty `old_string` mid-array (atomically rejected — fine — but the
   plan should never have contained it), and a manual
   `source scripts/lib/cmd-tq-devmod.sh` whose BASH_SOURCE-relative root
   resolution broke outside the blessed scripts (I should have used the
   gate script's own invocation path from the start). Plus one wrong
   nix attribute guess (`.#treefmt-check` is a flake check, not a
   package).
5. **gopls phantoms on cmd/tq still tax every edit** (3 permanent
   compiler errors in tool output all session). Documented and trusted
   past, but the cost recurs every window; nobody has spent the hour to
   make the LSP match the devmod reality (see §f17).

Nothing landed broken: every mistake above was caught by a gate, a
guard, or a deliberate verification step before it could ship — which is
the system working, not an excuse for causing it.

## e) WHAT WE SHOULD IMPROVE

1. **Kill the lint partial-load flake at the toolchain layer** (§b1):
   pin golangci-lint through the flake / a go1.26.7-built binary and
   re-measure stability across N runs before ANY future baseline regen.
2. **Make `lint-baseline.sh` refuse poisoned regens**: a `typecheck` row
   is never a real finding — the script should fail loudly (naming the
   module + stderr) instead of recording it as a baseline row, and the
   regen should require two consecutive identical runs (stability
   assertion) before writing.
3. **Add a cmd/tq pin-drift gate**: compare cmd/tq's committed requires
   against the max versions required across internal/root go.mods. Today
   the ONLY catcher is ci-local's nix step at the very end, with the
   least helpful error text in the repo.
4. **Fixture discipline**: replay-semantics tests must be written AFTER
   re-reading the store's fact-writing code (or against a copy of the
   fact matrix in `docs/DOMAIN_LANGUAGE.md`), never from session memory.
5. **Bless a local postgres-conformance recipe** (script or AGENTS
   paragraph) so fact-shape pins are run live in every window that
   touches them, not just in CI.
6. **Red-master dwell**: 02:50→07:00+ red with verified fixes sitting
   local. Either a faster owner push cadence or an agreed policy for
   auto-pushing mechanical gate fixes (owner call).
7. **Index bloat**: run the archive sweep before the next report lands
   (136 live rows and growing by 1-2 per session).
8. **Silence the gopls phantoms on cmd/tq** (workspace config, or an
   accepted tooling note) — recurring attention cost in every window.

## f) UP TO 50 THINGS WE SHOULD GET DONE NEXT

_Brainstorm ranked by impact; HARVEST should route 1–13 to TODO_LIST,
the rest to ROADMAP unless the owner pulls them forward._

| #  | Thing                                                                                                                          | Impact | Effort |
| -- | ------------------------------------------------------------------------------------------------------------------------------ | ------ | ------ |
| 1  | Root-cause golangci partial-load flake (try go1.26.7-built golangci-lint; N-run stability measurement)                         | High   | M      |
| 2  | lint-baseline.sh: fail on typecheck rows + require two identical consecutive regen runs                                        | High   | S      |
| 3  | cmd/tq pin-drift gate (committed requires vs internal/root graph max)                                                          | High   | S      |
| 4  | Owner Q2 → cut v0.3.x sub-tag wave OR fold into v0.4.0; run release flow                                                       | High   | M      |
| 5  | Owner Q1 → ratify postgres rescue-enqueue alignment (pins already protect either way)                                          | High   | S      |
| 6  | Owner push: heal red master (all causes verified local)                                                                        | High   | S      |
| 7  | HARVEST this report's §f into TODO_LIST/ROADMAP                                                                                | High   | S      |
| 8  | Post-release lap: proxy propagation, pkg.go.dev, `go install cmd/tq@vX` smoke                                                  | Med    | S      |
| 9  | Local postgres-conformance script (wrap the docker recipe)                                                                     | Med    | S      |
| 10 | Verify CI test-postgres job runs the two new subtests (next push)                                                              | Med    | S      |
| 11 | Archive sweep for docs/status (136>100 rows)                                                                                   | Med    | M      |
| 12 | Run `tq audit --journal` read-only on the production journal once; record real coverage deficit + drift state                  | Med    | S      |
| 13 | AGENTS.md Known Issues: "nix builds cmd/tq WITHOUT tidy — stale committed requires fail with terse 'updates to go.mod needed'" | Med    | S      |
| 14 | Review templ-components v1.17.0 release notes vs webui adoption table (AGENTS pins v1.16.x)                                    | Med    | S      |
| 15 | Review go-retry v0.6.0 changelog for executor retry-path behavior changes                                                      | Med    | S      |
| 16 | FEATURES.md: confirm journal-drift audit entry reflects coverage + pins                                                        | Low    | S      |
| 17 | gopls-on-cmd/tq phantom fix (devmod-aware workspace or documented tooling filter)                                              | Med    | M      |
| 18 | `tq audit --journal --help`: document coverage semantics + "(no facts)" sentinel                                               | Low    | S      |
| 19 | Count malformed enqueue details as a visible report dimension instead of silent "unknown"                                      | Med    | S      |
| 20 | Coverage line: optionally name the first unenriched seq so operators can bound the legacy region                               | Low    | S      |
| 21 | `DriftReport`: consider streaming replay (factsPageSize pages) instead of whole-journal slice                                  | Med    | M      |
| 22 | Measure `tq audit --journal` runtime on a large journal before promising O(n) comfort                                          | Med    | S      |
| 23 | Document "pre-enrichment journals permanently lack priority/dedup derivability" (landing report item 34)                       | Low    | S      |
| 24 | Fuzz seeds for replayProjection detail parsing (malformed JSON shapes)                                                         | Low    | M      |
| 25 | Drift-test the batch-task paths (member tick → prune-stale cancel) through the audit                                           | Med    | M      |
| 26 | Align CI postgres service image with the locally-proven 16-alpine (or vice versa)                                              | Low    | S      |
| 27 | sqlite varnamelen: rename the two oldest (09-05) findings for an honest shrink                                                 | Low    | S      |
| 28 | `DriftRow.Replayed="(no facts)"` stringly sentinel → structured flag (JSON stability tradeoff, needs decision)                 | Low    | S      |
| 29 | Review the dep-sweep window's now-committed depbump/agentpool/main sweep with a fresh test lap once that window closes         | Med    | M      |
| 30 | Confirm landing-report items 1–14 actually landed in TODO_LIST (HARVEST audit)                                                 | Med    | S      |
| 31 | Upstream crush issue: misleading "does not support reasoning effort" error (verify-before-filing → github-voice)               | Med    | M      |
| 32 | check-script-syntax.sh (dep-sweep window): confirm ci-local/CI wiring + shellcheck presence on runners (check-guard-wiring)    | Med    | S      |
| 33 | check-status-index digest row (08-25 f20) as the bloat fix alternative                                                         | Low    | S      |
| 34 | `examples/embed` post-bump smoke is covered by ci-local — note it in the release checklist explicitly                          | Low    | S      |
| 35 | Consider `--since`/watermark option for the audit on huge journals (only after #22 measures)                                   | Low    | M      |
| 36 | Session-close bridge: note SessionOpened/Closed are observation-only in audit help/docs                                        | Low    | S      |
| 37 | Add the local postgres recipe to AGENTS.md commands section                                                                    | Low    | S      |
| 38 | Pre-existing advisory `unusedparams` on postgres.go:503 — fix in passing next time that file is open                           | Low    | S      |
| 39 | CHANGELOG: remember release date stamp when tags are cut                                                                       | Low    | S      |
| 40 | Suggest recording `lint-baseline.sh` stability protocol in AGENTS.md next to the baseline notes                                | Low    | S      |
| 41 | Make the drift smoke also assert the coverage line (guards future output refactors)                                            | Low    | S      |
| 42 | Race-lap cmd/tq tests once more after dep-sweep window lands (hot-file rule)                                                   | Med    | S      |
| 43 | Consider auto-push policy discussion for mechanical red-master fixes (owner)                                                   | Med    | S      |
| 44 | Pre-cut sub-tag DRY-RUN (release.sh --tag in dry mode) so Q2's answer executes in minutes, not hours                           | Med    | S      |
| 45 | Rewrite the two nix-related AGENTS gotchas into one "nix × cmd/tq graph" section with the no-tidy explanation                  | Low    | S      |
| 46 | Add `tq audit --journal` example output (text+JSON) to README operator section                                                 | Low    | S      |
| 47 | Sweep for other gates that silently partial-load (varnamelen-class undercounts) by diffing counts across 3 runs                | Med    | S      |
| 48 | Keep `check-go-mods.sh` green-lap after every dependabot merge (it caught nothing this time only by luck of ordering)          | Med    | S      |
| 49 | Index row + close-out cross-links for THIS report (done at write time) — keep the chain unbroken                               | Low    | S      |
| 50 | After Q1/Q2 resolve: re-run the landing report's §f remaining ROADMAP items through docs-health triage                         | Low    | S      |

## g) THREE QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **Release cadence (carried from the landing report, now blocking):**
   do I prepare the v0.3.x sub-tag wave NOW (enrichment + pins + audit
   ship to `go install` users ahead of the dep-sweep/batched-harvest
   features), or batch everything into v0.4.0? I can prepare a dry-run
   of the sub-tag cuts either way, but the version-surface decision
   (and the CHANGELOG stamping) is yours.

2. **Lint toolchain policy:** golangci-lint on this host is v2.13.2
   built with go1.27.1, linting a go1.26.7 tree whose stdlib imports are
   GOEXPERIMENT-gated — my top suspect for the intermittent
   partial-load/typecheck flake that poisoned the 06:03 baseline regen.
   May I pin golangci-lint via the flake (built with the repo's own
   toolchain) and, if that kills the flake, encode the stability
   requirement (two identical consecutive regen runs) into
   `lint-baseline.sh`? Or do you prefer the flake-independent binary
   pinned another way?

3. **Production journal audit timing:** may I run `tq audit --journal`
   READ-ONLY against `/mnt/pool/services/tq/tq.db` now to record the
   real-world baseline (how big the legacy coverage deficit is, whether
   any drift exists at all), or should the first production audit wait
   until after your Q1/Q2 decisions so the report reflects a released
   shape? Read-only either way — this is purely a timing and
   report-narrative question.

---

_Recorded 2026-09-15 07:00 CEST. Format note: user explicitly requested
`.md`; the status-report skill's canonical format is a styled HTML
dashboard — override honored, flagged here per skill contract, not
propagated back as a default. §f is HARVEST input (docs-health), not a
commitment list; §g blocks: release flow, lint policy, production audit
timing. WAITING FOR INSTRUCTIONS._
