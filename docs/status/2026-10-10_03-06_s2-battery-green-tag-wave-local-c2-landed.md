# Status: S2 Battery GREEN; Tag Wave Tagged Local-Only; C-Track Landed Through C2

**Date:** 2026-10-10
**Window:** 2026-10-10 ~02:10 → 03:06 CEST (resumed the A1-shipped/golden-red session; A-track completion, S2 battery, tag wave, C-track start)
**Plan:** `docs/planning/2026-10-09_20-23_CONFIG-SYSTEM-ALL-IN-BRIDGE.md` (Full Execution Mode)

## a) FULLY DONE

- **Todo list recreated** per handoff (B1/D1/B2/B3/B4, A1-alias, A1-go.mod, A5-pre = completed; golden-test fix = in_progress at start).
- **A1-final shipped:** `cmd/tq/facts_json_test.go:184` decode retargeted `[]journal.Fact` → `[]factJSONView`, plus a new tripwire asserting Detail unmarshals as a JSON **object** — a base64 wire regression can never pass silently again (`d67a1360`). cmd/tq shim gate **RC=0** (rc-to-file; no pipe-rc repeat).
- **Session-start vendor trap found and fixed:** root build was RED (`vendor/.../journal` missing the lifecycle re-exports) — the vendor tree predated `91135c1e`; `go mod vendor` re-synced it (vendor/ is gitignored, nothing to commit) and root build+vet went GREEN.
- **A2 shipped:** companion `UpstreamFact`/`JournalFacts` collapsed to identity (alias flip made the types one); `UpstreamFact` keeps Seq-zeroing, the journal-reassigns contract (`247d77df`); bridge-test doc no longer claims journal stays go-cqrs-lite-free. Companion gate GREEN.
- **A2 regression caught and fixed (the big one):** the identity `JournalFacts` let the engine's **empty non-nil** Detail blobs through; an empty `jsontext.Value` fails payload marshal ("unexpected EOF within /detail"), wedging the projection worker on the first `task.claimed` — `TestProjectionHostTailsLiveFacts` RED. Root-caused via solo run + full WARN log; blamed via pre-flip worktree (pre-flip PASSED → my regression). Fix: the one genuinely-needed rule restored — empty normalizes to nil at the seam — plus a regression pin `TestJournalFactsNormalizesEmptyDetail` (`9143cc84`). Companion + readmodel gates GREEN after.
- **A3 verified:** journal module gate RC=0 with zero changes — the memory journal is unified vocabulary by construction (aliases).
- **A4 shipped:** `internal/journal/cqrs` go.sum gained the transitively-required `queue/v4 v4.0.3` (`bd43d319`, tidy-only); gate RC=0. `--cqrs` Detail wire confirmed stable by construction (`factPayload` keeps `Detail jsontext.Value`); its cmd-side test (`facts_cqrs_test.go`) passed inside the green cmd/tq gate.
- **A5 shipped:** seven modules (4 facades + internal/executor + internal/queue + worker) needed the propagated `queue/v4` go.sum/go.mod entries (`80e49eee`, mechanical tidy). Full 15-module sweep (`find … -name go.mod`, GOWORK=off build+vet+test -count=1): **FAILED_MODULES empty**.
- **A6 S2 battery GREEN:** gofmt clean across all modules + cmd/tq; root build+vet RC=0; root `go test ./... -race -count=1` **18/18** (after the race fix below); smokes `legacy-serve-upgrade.sh` + `webui.sh` both PASS on a scratch `TQ_DB`; ADR-0019 endgame addendum updated **S2 → DONE (2026-10-10)** with the alias/identity/wire/battery summary (`1d9d9f1b`).
- **Pre-existing webui data race fixed on sight (A6 blocker):** `TestStatsReadFromReadModel` DATA RACE — pump goroutine wrote `Server.model` while HTTP handlers + health prober read it. Reproduced at pre-flip commit via throwaway worktree (race predates S2; B-track residue), but it blocked the `-race` gate, so fixed: `model` is now `atomic.Pointer[readmodel.Model]` (Store/Load at all 5 sites incl. the prober closure) (`7f2ba9ad`); full webui suite `-race` RC=0.
- **A7 tag wave executed, LOCAL-ONLY (no push — owner Q2 parked):** floors journal+companion v0.3.3→v0.4.0 across all 20 consuming go.mods (`7936e6b8`); annotated tags `internal/journal/v0.4.0` + `internal/queue/companion/v0.4.0` cut at that commit. cmd/tq (replace-free, resolves via proxy) floor **held at v0.3.3** — unpushed tags are unresolvable there (`8b2ef1e5`); its floor moves at push time. `check-go-mods.sh` 45/45; facade parity GREEN after adding the missing executor-facade re-exports (RiskLow/Medium/High, Finding, MergeFindings — concurrent-agent drift, `ed23f0a4`); `go mod vendor` synced.
- **C1 shipped:** new leaf module `internal/config` — `Deployment{Driver, DBPath, DSN, SyncPolicy, ReadModelEnabled}` with shape validation (mixed/empty deployments error at construction), `FromFlags` merging `--store` (bare path=sqlite, `postgres://`=postgres) with `TQ_SQLITE_SYNC` (name + full|normal|off vocabulary unchanged), and the **repo's ONE pragma literals**: `QueuePragmas()` (full DSN union, order load-bearing) + `ProjectionPragmas()` (caller-pragma pair), both derived from one sync tier. Tagged `internal/config/v0.3.3` (`c5e36cef` + `10ff7a6d`); module gate GREEN after a two-bug fix-forward (see d1).
- **C2 shipped:** `sqlitev4.OpenWithDeployment(config.Deployment)` consumes the struct — DSN rendered from `cfg.QueuePragmas()` (the old pragma literal deleted), sync tier arrives resolved (no env read); `Open(path)` remains as the env-semantics-preserving thin wrapper for facades/tests (`synchronousPolicy()` env reader deleted; `config.SyncPolicyFromEnv` is the ONE reader). Migration DSN builders untouched (stay FULL per the IO policy). sqlitev4 gate incl. the TQ_SQLITE_SYNC escape-hatch tests **RC=0** — commit is daemon-swept `8ac0d541` (footer-less; see d4).

## b) PARTIALLY DONE

- **C-track:** C1+C2 landed; **C3 not started** (composition.New takes the Deployment; readmodel's `ProjectionHomeCallerPragmas` must delegate to `config` so the literal count drops to one — currently readmodel still owns the projection pair); **C4 not started** (`--store` flag mapping across serve/api/worker/agent-pool; postgres path needs the owner decision in g2); **C5 not started** (doctor/audit consume the struct; AGENTS.md store-invariants row + README config section + CHANGELOG).
- **C2 residue:** `go mod vendor` NOT yet re-run after sqlitev4's go.mod change (vendor stale again); cmd/tq shim gate NOT yet re-run after the `8b2ef1e5` floor revert.
- **E-track (E1→E3), F-track (F3, F1, F2, F4), wrap-up items:** not started (CHANGELOG entries, plan-file DONE marks with citations, 46-file consumer inventory into the plan appendix, B4 serve smoke folded into `scripts/smoke/`, AGENTS.md lesson fold).

## c) NOT STARTED

- D2 upstream memo filing (owner-gated, still `— BLOCKED: owner go for upstream filing`).
- Post-bridge releases: the v0.4.0 module tags are cut locally but nothing is pushed, no root release cut (`scripts/release.sh vX [--tag]` remains the flow once the wave is pushed).

## d) TOTALLY FUCKED UP (all caught; honesty section)

1. **Committed before gating, twice.** C1: named commit `c5e36cef` landed, THEN the module gate found two real bugs — `parseSyncPolicy` matched uppercased env input against lowercase constants (TQ_SQLITE_SYNC=full was rejected!) and `Postgres()` carried no default SyncPolicy (shape validator rejected the constructor's own output). Fix-forward `10ff7a6d` + the local tag re-cut at the fixed commit (never pushed, so mutable). C2: same miss inverted — gate ran, then the daemon swept the edit into footer-less `8ac0d541` before I could name it. The edit→commit→battery ordering protects ATTRIBUTION, not correctness; a red named commit is still a red commit.
2. **Garbled code on first write:** `QueuePragmas` contained a nonsense slice-literal expression (leftover of a bad construction); caught on immediate re-read, fixed pre-commit. The write tool is not a reviewer.
3. **Wrong tag path on the journal wave:** cut `journal/v0.4.0` (facade-style) instead of `internal/journal/v0.4.0`; `check-go-mods.sh` failed 44/1 with "unknown revision internal/journal/v0.4.0". Deleted and re-cut correctly (local-only). The gate did its job — but the manual sed/tag loop has now misfired once and would benefit from a script (see e2).
4. **Vendor staleness is a recurring self-inflicted class:** session START was red because the previous session's vendor sync predated a commit; C2 re-staled it within the same session. Every go.mod change needs the vendor sync in the SAME breath as the commit.
5. **Daemon attribution keeps fragmenting:** 3 of this session's commits are footer-less chores (`30e2b702`, `8ac0d541`, plus earlier `8fa8a58d`-class). `heal-daemon-sweep.sh` exists but needs a Task-Queue-ID footer to target — queued for wrap-up.

## e) WHAT WE SHOULD IMPROVE

1. **Gate-before-named-commit for risky edits:** new modules and signature changes get the fast module gate run BEFORE the named commit; the 60s daemon window only justifies speed, never skipped verification.
2. **A `scripts/tag-wave.sh`:** bump floors + cut correctly-pathed annotated tags + run check-go-mods + facade parity in one invocation; today that loop is manual sed + `git tag -a` + re-verify and it already misfired once (d3).
3. **`new-module.sh` should auto-wire the root go.mod** (require + replace); I hand-edited it for internal/config.
4. **Vendor sync belongs inside the per-commit reflex** (same command chain as the commit, not an afterthought) — or a check-gomod-vendor-sync run in the local loop, not just ci-local.
5. **AGENTS.md has not absorbed this session's durable lessons yet** (internal/config exists as the deployment lane; empty-Detail→nil is a pinned companion contract; cmd/tq floor parked pending push; atomic model field pattern). F4 covers part of it, but the size guard makes it a deliberate edit — queued, not forgotten.

## f) NEXT (prioritized)

1. `go mod vendor` + root build/vet (C2 residue).
2. Re-run `./scripts/test-cmd-tq.sh` (rc-to-file) post-`8b2ef1e5` — the floor revert is unverified.
3. **C3:** `composition.New(ctx, config.Deployment)`; readmodel projection pragmas delegate to `cfg.ProjectionPragmas()` (literal count → 1); composition + readmodel gates + compose/projection_runtime test updates.
4. **C4:** `--store` flag → `config.FromFlags` across serve/api/worker/agent-pool wiring; postgres driver selection as a data change; Postgres verification path per g2; smokes.
5. **C5:** doctor/audit consume the struct; AGENTS.md store-invariants row; README config section; CHANGELOG.
6. **E1:** TQ_POOL_CONFIG README section + parser hardening (unknown-key warn, typed decode).
7. **E2:** agent-pool flag diet (file-configurable defaults; flags override).
8. **E3:** `check-flag-count.sh` per-command pin wired into ci-local.
9. **F3:** ADR-0022 three-lane config model + all-in gating note.
10. **F1:** webui tailer retirement eval → ADR-0003 addendum decision.
11. **F2:** internal/queue/sqlite legacy thinning post-S2.
12. **F4:** AGENTS.md single-opener + deployment-struct rows; size-guard check.
13. Wrap-up: CHANGELOG for A/B/C tracks; plan-file DONE marks with gate citations; persist the consumer inventory to the plan appendix; fold B4 serve smoke into `scripts/smoke/`; heal-daemon-sweep the footer-less chores if IDs exist.
14. Carry g1–g3 answers into execution as soon as the owner provides them.

## g) QUESTIONS FOR THE OWNER (cannot self-answer)

1. **(Q2 re-ask) Push authorization:** may I push the S2 wave tags (`internal/journal/v0.4.0`, `internal/queue/companion/v0.4.0`, `internal/config/v0.3.3`) and move cmd/tq's floors to v0.4.0 at push time? Until then cmd/tq stays pinned to the pre-alias vocabulary and no `@latest` consumer can see the wave.
2. **(new) C4 Postgres verification:** the verify matrix allows "`--store postgres://` works … **or documented gated**". This environment has no Postgres instance. Accept documented-gated, or point me at a scratch Postgres DSN to verify serve/worker against?
3. **(Q3 re-ask) `tq facts --json` Detail wire:** embedded JSON is now pinned permanently (`factJSONView` + object-shape tripwire, `d67a1360`). Ratify embedded JSON as the permanent wire, or prefer base64 post-bridge (drop the view, re-pin honestly)?
