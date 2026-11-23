# Status Report — ADR-0019 endgame: full platform migration (owner-directed session)

Report time: 2026-10-04 02:36 CEST · HEAD at session work: ~199c0bac+ (daemon
chore commits interleaved) · Plan: `docs/planning/2026-10-03_platform-migration-endgame.md`

Scope of this session: the owner's directive "FULLY migrate to
metaengine/ + system/ in a backward auto-upgradeable way; remove all
legacy ASAP", executed as phases P0–P5 of the endgame plan.

## a) Fully done (verified green at each landing)

1. ~~**P0 — Baseline green.** Root vendor/ resynced (the v4-family bump~~ — RESOLVED 2026-11-23: baseline green at HEAD — full root `go build && go vet && go test ./... -race` rc=0 (vendoring regime since changed: no vendor/ tree at HEAD).
   563c30c3 never re-vendored — root builds were broken on inconsistent
   vendoring); `internal/readmodel` + `internal/worker` go.sums tidied;
   root build+vet+test -race green; every nested module gate green.
2. ~~**P1 — Backward auto-upgrade on open (production unblock).**~~ — RESOLVED 2026-11-23: live at HEAD — internal/queue/sqlitev4/migration (upgrade.go, TQ_NO_AUTO_UPGRADE kill switch :39,:75); dogfood cutover 2026-10-05 (FEATURES.md:54, docs/release/2026-10-05_CUTOVER-RUNBOOK.md); pinned by scripts/smoke/legacy-serve-upgrade.sh.
   - The replay tool's core moved from `package main` into the importable
     `internal/queue/sqlitev4/migration` package; `replay/` stays as thin CLI.
   - `migration.UpgradeIfNeeded`: read-only schema probe (fresh/engine/legacy
     via the `lease_token` column) → `VACUUM INTO` snapshot kept as
     `<db>.legacy-<ts>.bak` (also checkpoints stale WAL) → in-place converge
     via the engine's own migrations → full projection-equality Verify
     against the snapshot → **auto-RESTORE on any mismatch** (tested live —
     the restore path fired during a fixture mismatch and recovered).
   - Kill switch `TQ_NO_AUTO_UPGRADE=1` refuses with the manual path named.
   - The facade `internal/queue/sqlite.Open` is the shim seam; direct
     sqlitev4 openers (tests, replay CLI) bypass deliberately.
   - Tests now seed TRUE legacy fixtures (raw pre-flip DDL recovered from
     f0643178^) — the old test seeded through the thin driver and produced
     an engine-shaped fixture (its docstring had rotted).
   - Gates green: migration pkg, sqlitev4, sqlite facade, root, cmd/tq.
3. ~~**P2 — S3 flip: the metaengine projection is the default read side.**~~ — RESOLVED 2026-11-23: pinned — TestReadModelFlagDefaultsOn (cmd/tq/readmodel_default_test.go:11, docstring names the S3 flip).
   `--read-model` defaults TRUE on `tq stats`/`tq api`/`tq serve`;
   `--read-model=false` is the explicit fallback; pinned by
   `TestReadModelFlagDefaultsOn`; cmd/tq gate green.
4. ~~**P3 — S2 journal unification (the last split brain dies).**~~ — RESOLVED 2026-11-23: companion.JournalFacts seam live (internal/queue/companion/core.go:182); vocabulary-drift test pins tq-vs-engine constants (companion test "vocabulary drift: tq %q != engine %q").
   - Both v4 adapters' `Facts`/`FactsForTask` now ride the ENGINE's own
     `queue.Store` reads; companion's mirrored SQL for those two is deleted;
     ONE conversion seam (`companion.JournalFacts`, inverse of `UpstreamFact`).
   - Every journal consumer (webui tailer, sweepers, bridges, readmodel
     projector) pages the engine journal; facts-in-same-tx is
     engine-enforced (FactTx sink since the earlier shipped slice).
   - Vocabulary contract pinned: tq lifecycle constants are byte-identical
     to upstream `facts` constants (new companion test); internal/journal
     stays go-cqrs-lite-free (DAG purity, ADR-0014 D2) — the open string
     type rides, not an import.
   - Gates green: companion, sqlitev4, postgresv4, queue, journal,
     readmodel, worker, journal/cqrs, both facades, cqrsqlite, root, cmd/tq.
5. ~~**AGENTS.md truth pass** — architecture table now names the thin-driver~~ — RESOLVED 2026-11-23: table live at HEAD — AGENTS.md:73 ("Thin v4-adapter drivers") and :76 ("internal/{readmodel,composition} … S3 projection + S4 root").
   - v4-adapter + readmodel reality; auto-upgrade seam documented; ~1 kB
     of phrasing waste pruned; doc budget consciously reset 15,000 → 15,200
     (dated comment in the guard — net-new load-bearing rows landed).

## b) Partially done (in flight right now)

6. ~~**P4 — S4 system composition root.**~~ — RESOLVED 2026-11-23: promotion completed — internal/composition is its own module (go.mod, compose.go), required + relative-replaced by root (go.mod:114,163) and imported by cmd/tq (cmd/tq/go.mod:13, main.go:32).
   - `internal/composition` package written and GREEN as a root package:
     `system.New(ctx, dbPath)` over a durable sqlite engine at the
     projection home (`<db>.readmodel.db`), empty domain by design (the
     queue engine journal IS the journal — no split brain), Growth seam
     for timers/deciders documented per the archived composition map.
   - `tq serve` wiring drafted: compose when read-model on, `GracefulClose`
     joined into runactor's OnShutdown (map §4c, per-surface adoption).
   - Root go.mod carries `go-cqrs-lite/system/v4 v4.10.1` (+ vendor sync).
   - **BLOCKER (mechanical, diagnosed):** cmd/tq is replace-free; importing
     a ROOT-module package (`internal/composition`) trips `ambiguous
     import` because the published v0.3.0 zip of the root module still
     ships `cmd/tq` files (the nested go.mod postdates that tag). Fix in
     flight: promote `internal/composition` to its own internal module
     (the repo's idiom for exactly this). `new-module.sh` scaffolded its
     go.mod (and refused the readmodel require — no cut tag yet; the
     nominal-v0.3.0 + relative-replace pattern is how root itself pins
     readmodel). I was writing the corrected go.mod when this report was
     requested. Remaining: move the two compose files in, tidy, root
     require+replace, cmd/tq require + import swap, devmod gate, root gate.

## c) Not started

7. ~~**P5 — legacy deletion + docs truth pass**: delete `internal/queue/cqrsqlite`~~ — RESOLVED 2026-11-23: executed — internal/queue/cqrsqlite deleted (no such dir at HEAD), absent from all go.mods; ADR-0019 + FEATURES/CHANGELOG truth rows live (docs/adr/0019-go-cqrs-lite-platform-adoption.md, FEATURES.md:54, CHANGELOG.md:1619); full ci-local ran 2026-11-23 (TODO_LIST.md:20).
   (superseded spike; conform suite keeps running for sqlitev4/postgresv4),
   collapse dual read paths where the S3 design sanctions it, ADR-0019
   stage markers, FEATURES/CHANGELOG rows, TODO_LIST ADR rows, LOC +
   art-dupl delta, vendorHash regen, full ci-local matrix, smokes.

## d) Totalmente fucked up (honesty section)

8. **The auto-commit daemon swept large parts of my named commits into
   footer-less `chore:` commits** (P3 landed entirely as chores; my P1
   named commit captured only 2 of its files). Content is fully landed and
   gates-verified, but the history narrative is lost. `heal-daemon-sweep.sh`
   doesn't apply (it needs a Task-Queue-ID; this is direct owner-directed
   session work).
9. **cmd/tq go.mod drifted** while adding the system require: tidy upgraded
   `modernc.org/libc`/`sqlite` indirects and pulled otel/watermill/koanf
   transitive deps — NOT yet reconciled against the nix vendorHash gate
   (not run since the go.mod changes) or `check-go-mods.sh`.
10. ~~**`new-module.sh` refused** the composition scaffold's readmodel~~ — RESOLVED 2026-11-23: the manual nominal-require+replace treatment completed — internal/composition is a required, relative-replaced module at root and cmd/tq (go.mod:163, cmd/tq/go.mod:13).
    require (no cut tag exists) — expected per its design, but it means the
    composition module's go.mod needs the manual nominal-require+replace
    treatment; I had just started that edit (the scaffolded file changed
    under me mid-write).
11. ~~**Incomplete verification breadth this session**: I ran targeted gates~~ — RESOLVED 2026-11-23: superseded — full ci-local matrix ran green 2026-11-23 (TODO_LIST.md:20) and the full root -race battery is rc=0 at HEAD.
    per phase + one full root `-race` pass (P0, P3) — the FULL ci-local
    matrix, smokes, and the nix build (vendor-hash + webui-css checks) have
    NOT run since the go.mod changes landed.

## e) Improvements made beyond the direct ask

- The migration library's legacy-fixture generator is reusable for any
  future schema-generation probe (raw DDL preserved verbatim in tests).
- `companion.JournalFacts` gives one documented seam for engine-fact →
  tq-vocabulary conversion (was implicit per-backend SQL before).
- `tq serve` now has a system-rooted close ordering story (store close +
  GracefulClose joined) instead of store-close-only.

## f) Up to 50 next things (roughly execution order)

1. ~~Finish the `internal/composition` module promotion: corrected go.mod~~ — RESOLVED 2026-11-23: done — internal/composition module (go.mod, compose.go) live at HEAD.
   (require readmodel v0.3.0 nominal + `=> ../readmodel` replace, system +
   sqliteengine requires), move `compose.go`/`compose_test.go` in.
2. ~~`go mod tidy` inside internal/composition; module gate green.~~ — RESOLVED 2026-11-23: module live with go.sum; covered by the green root battery at HEAD.
3. ~~Root go.mod: require + relative-replace `internal/composition`; root~~ — RESOLVED 2026-11-23: go.mod:114 require + :163 replace, battery green at HEAD.
   build/vet/test green; `go mod vendor`.
4. ~~cmd/tq: require `internal/composition v0.3.0`, swap the import, run~~ — RESOLVED 2026-11-23: import swapped — cmd/tq/go.mod:13 + cmd/tq/main.go:32 import internal/composition.
   `./scripts/test-cmd-tq.sh`.
5. Reconcile cmd/tq's drift (libc/sqlite bumps): keep if gates pass, else
   pin back; run `scripts/check-go-mods.sh`.
6. ~~Run `nix build .#checks.x86_64-linux.vendor-hash`; copy `got:` to~~ — RESOLVED 2026-11-23: superseded — nix build + flake check green 2026-11-23, vendor-hash unchanged (TODO_LIST.md:22).
   flake.nix (AGENTS vendorHash ritual).
7. ~~Full `./scripts/ci-local.sh` pass (the real matrix).~~ — RESOLVED 2026-11-23: ran 2026-11-23 — TODO_LIST.md:20 `[x]` (every gate green except two documented freshness items).
8. WebUI + httpapi smokes (`scripts/smoke/`), incl. `TestRoutesAreReadOnly`
   - health-CSP pins against the now-default readmodel path.
9. ~~~~`tq serve` dogfood dry-run on a scratch TQ_DB with a legacy fixture:~~ — RESOLVED 2026-11-23: shipped as scripts/smoke/legacy-serve-upgrade.sh (docstring: LEGACY pre-flip journal → transparent auto-upgrade → default-ON readmodel → GracefulClose ordering); dogfood cut over 2026-10-05 (FEATURES.md:54).
   auto-upgrade → readmodel default-on → GracefulClose ordering, one script.
10. ~~P5: delete `internal/queue/cqrsqlite` + its conform-suite wiring~~ — RESOLVED 2026-11-23: package deleted (absent at HEAD and from all go.mods).
    (harness keeps sqlitev4/postgresv4); delete `fromUFacts` duplication in
    cqrsqlite (dies with the package).
11. ~~P5: root go.mod drops cqrsqlite references; module loop re-run.~~ — RESOLVED 2026-11-23: 0 cqrsqlite references in go.mod/cmd/tq/go.mod at HEAD.
12. ~~P5: collapse dual tally paths in cmd/tq (tallyStats vs tallyModelRows)~~ — RESOLVED 2026-11-23: both identifiers gone from cmd/tq at HEAD (0 grep hits — dual path collapsed).
    where the S3 design says projection-backed (keep store-backed
    row-rich views).
13. P5: decide + execute the fate of the hand journal tailer (ADR-0003
    Phase D read path) — retire once Watcher/SSE serves the dogfood.
14. ~~Docs: ADR-0019 add the endgame addendum (S2/S3/S4 + auto-upgrade~~ — RESOLVED 2026-11-23: docs/adr/0019-go-cqrs-lite-platform-adoption.md carries the endgame state (addendum renamed to the real seam in the 2026-11-23 sweep, TODO_LIST row "Sweep living docs for dead facade phrasing" `[x]`).
    landed; S1 already); stage markers per section.
15. ~~Docs: FEATURES.md rows for auto-upgrade, readmodel-default, engine~~ — RESOLVED 2026-11-23: FEATURES.md:54 auto-upgrade row 🟢 FULLY_FUNCTIONAL with smoke + runbook citations.
    fact reads, composition root (status 🟢 with citations).
16. ~~CHANGELOG `[Unreleased]` entry for the whole endgame wave.~~ — RESOLVED 2026-11-23: entry exists — CHANGELOG.md:1619 "Backward auto-upgrade on facade Open (S1 flip residue, landed…)".
17. TODO_LIST: close ADR-0019 S1 row (stale — flip landed), S2 row,
    S3 row; S4 row once composition lands; keep dogfood-cutover row.
18. LOC delta report (tokei) + art-dupl `-t 4` recount vs the 2026-09-23
    baseline (the deletion dividend).
19. The tag wave (owner-gated, see questions): root v0.3.1 (finally
    excludes cmd/tq from the zip) + internal/{queue/sqlite,
    queue/sqlitev4, queue/postgresv4, queue/companion, readmodel,
    composition} tags; proxy checks per docs/release/.
20. ~~~~`scripts/check-facade-parity.sh` after the tag wave (facade requires~~ — RESOLVED 2026-11-23: re-ran at HEAD — "facade parity OK: 5 facades mirror their internal packages" (post-tag-wave: facades now at v0.4.2).
    re-pinned to real tags).
21. ~~Dogfood deploy: systemd unit TQ_DB swap is owner-run; with P1 the~~ — RESOLVED 2026-11-23: cut over 2026-10-05 — FEATURES.md:54 ("Dogfood cut over 2026-10-05") + docs/release/2026-10-05_CUTOVER-RUNBOOK.md.
    auto-upgrade makes it transparent — prepare the runbook note.
22. ~~Post-deploy verification: `tq doctor` against the production journal~~ — RESOLVED 2026-11-23: covered by the 2026-10-05 cutover receipt (runbook + FEATURES.md:54: snapshot kept, auto-upgrade transparent).
    (auto-upgrade log line + backup file present + head seq unchanged).
23. ~~Keep `<db>.legacy-*.bak` hygiene note in the runbook (owner may prune~~ — RESOLVED 2026-11-23: snapshot naming documented — FEATURES.md:54 ("snapshot `<db>.legacy-*.bak` → migrate → verify → auto-restore") + the cutover runbook.
    after a green week).
24. Upstream ratification memo (M4) — still blocked on owner go (TODO row
    stands): enqueued-fact snapshot detail + CountFacts/FactsSince
    pushdown.
25. readmodel: thin-enqueue side channel (RowSource) dies when upstream
    grows the enqueued detail — track via the M4 memo.
26. Consider system.Lookup/query declarations for the readmodel
    collections (full system projection adoption) once upstream exposes
    journal injection — currently out of reach without a second journal.
27. Sweeper ticks onto `system.ManageTimers` — still ruled no-fit for now;
    revisit only with a genuinely timer-shaped feature (map §4a).
28. `internal/journal/cqrs` public-facade ruling (TODO row) — still
    owner-blocked.
29. ~~Sweep stale LSP vendoring noise: the gopls/golangci stale-vendor error~~ — RESOLVED 2026-11-23: moot — no vendor/ tree exists at HEAD (vendoring regime retired), so the stale-vendor error class is gone.
    persists until the daemon-era vendor sync is picked up; harmless but
    noisy for other agents.
30. Agent-shell hazard: `new-module.sh` + concurrent daemon writes raced
    once this session (scaffold file changed mid-edit) — consider a
    `.gitignore` d scratch dir convention for scaffolds.

## g) Up to 3 questions (cannot figure out alone)

1. **Tag wave authorization**: the replace-free cmd/tq (and the public
   facades) need a coordinated tag wave — root v0.3.1 + the six internal
   tags — before this migration is consumable outside the repo. Release
   pushes are owner-gated per the docs; do you want me to run the
   docs/release flow autonomously this window, or stop at a green tree
   with the release checklist staged?
2. **Dogfood cutover**: TODO pins it owner-run. With P1's transparent
   auto-upgrade, the next systemd restart upgrades in place (snapshot
   kept, verified, restorable). Do you want the restart + `tq doctor`
   verification now, and should the runbook keep the manual replay path
   as the documented fallback?
3. **Legacy-read-path deletion cadence (P5)**: "remove all legacy ASAP"
   argues for deleting the hand tailer + store-read fallback immediately
   after the dogfood serves green on the projection; the safer read is a
   short probation with `--read-model=false` as the escape hatch. Which
   cadence do you want?

— End of report; pausing here per instruction.
