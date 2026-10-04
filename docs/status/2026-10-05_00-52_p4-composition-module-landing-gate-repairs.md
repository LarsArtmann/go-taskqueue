# Status Report — ADR-0019 endgame P4 landing: composition module promotion + gate repairs

Report time: 2026-10-05 00:52 CEST · Session spanned 2026-10-04 02:4x →
03:4x CEST and a verification tail at 2026-10-05 00:3x–00:5x (21 h
inter-session gap; fleet moved underneath — see §d) · Plan:
`docs/planning/2026-10-03_platform-migration-endgame.md` · Prior report:
`2026-10-04_02-36_platform-migration-endgame-status.md`

Scope: resume at the P4 mechanical blocker (the failed `write` to the
scaffolded `internal/composition/go.mod`) and drive P4 to a green,
nix-buildable tree. P0–P3 were already landed and green at resume.

## a) Fully done (each verified green at landing)

1. **P4.1 — composition module go.mod + tidy.** Wrote the corrected
   go.mod (module + go 1.27.1 + system/v4 v4.10.1, sqliteengine/v4
   v4.5.0, readmodel v0.3.0 nominal). First tidy+build FAILED: replacing
   readmodel with a directory does NOT propagate readmodel's own
   replaces (main-module-only semantics), so `internal/task`/`journal`
   resolved from stale v0.3.0 proxy tags (`t.DedupKey undefined`,
   `journal.QuestionAsked undefined`). Fixed by mirroring readmodel's
   full 7-replace set into composition's go.mod (`../task`, `../journal`,
   `../queue`, `../queue/{sqlite,sqlitev4,companion}`, `../readmodel`).
2. **P4.2 — composition module gate green** (build + vet + test,
   `TestNewBuildsSystemRootOverProjectionHome` ok).
3. **P4.3 — root wiring.** Deviation from the resume notes, reasoned:
   root gets the REPLACE only, not require+replace — nothing in the root
   module imports composition, so `go mod tidy` prunes the require; the
   load-bearing line for both the devmod shim and the nix FOD is the
   replace (both mirror `replace … => ./…` lines from root go.mod
   verbatim). Root tidy correctly pruned system/v4 from root entirely
   (its require moved into the composition module where it is actually
   imported) and demoted sqliteengine to indirect. `go mod vendor`;
   root build + vet + `test ./... -race -count=1` ALL GREEN.
4. **P4.4 — cmd/tq wiring; THE BLOCKER DIES.** Added
   `internal/composition v0.3.0` to cmd/tq's direct requires; the devmod
   shim picks up the mirrored replace automatically. Full cmd/tq gate
   (`scripts/test-cmd-tq.sh`) GREEN — the ambiguous-import class is
   resolved by longest-prefix module selection through composition's own
   module.
5. **Full nested-module loop green** (all modules incl. the new
   composition entry). `TestExactlyOnceUnderConcurrency` failed 3/4 runs
   under host load average 12–22 — triaged per AGENTS protocol: the
   test configures no Budget (the parallel agent's uncommitted
   worker.go diff touches only the `p.cfg.Budget != nil` path — provably
   inert for this test), failure signature is a completion-deadline
   timeout (18/20) with SQLITE_BUSY storm, NOT an exactly-once
   violation; passed in 0.04 s on a quiet host. Documented flake, not
   attributed to the migration.
6. **check-go-mods.sh pending-tag exception.** `go mod verify` in cmd/tq
   cannot resolve `internal/composition@v0.3.0` from the proxy before
   the owner-gated tag wave. Added a surgical, self-deleting downgrade:
   only unknown-revision lines naming the pending path WARN; every other
   verify error stays a hard FAIL. Gate: 47 ok / 0 failed. cmd/tq's
   libc/sqlite drift self-reconciled (root and cmd/tq now agree on
   libc v1.77.1 / sqlite v1.60.1).
7. **flake.nix eval collision fixed.** The go-nix-helpers input (locked
   2026-10-02, pre-dating this window) now generates its own
   `checks.vendor-hash` in go-standard.nix, colliding with the repo's
   hand-rolled drift gate at eval time (broke ANY eval of that attr).
   Kept the local documented ritual, disabled upstream's via its
   `enableVendorHashCheck = false` option.
8. **Clean-room consumer compile: root-caused + fixed.** The scratch
   module's PRUNED graph only sees depth-1 requires — cmd/tq's stale
   `templ-components v1.17.0` pin beat the local root module's v1.19.4
   requirement (root isn't a listed require of the scratch module), so
   local `internal/webui` compiled against an API without `MaxTicks`.
   Fixed by re-pinning cmd/tq's whole stale UI family via
   `go mod edit -require` (templ-components +icons/utils/datastar/htmx
   v1.19.4, go-datastar v0.6.2 + static v0.6.1, go-health v0.4.1,
   go-health-dashboard v0.10.2). A plain `go get` is impossible pre-tag
   (proxy fetch of internal/composition@v0.3.0 fails by design — this
   also reproduced the exact ambiguous-import error the prior session
   hit, confirming the diagnosis). Cleanroom: GREEN.
9. **AGENTS.md truth pass for the module topology.** Composition row in
   the architecture table + module listing (tightened twice to fit the
   budget). Second conscious budget reset 15,200 → 15,400 with dated
   comment (Go guard `agentsDocMaxBytes` + the bash twin
   `scripts/check-agents-size.sh` kept in sync — the twin was another
   agent's fresh work, synced not reverted). A concurrent formatter pass
   then padded the package table with ~1.7 kB of alignment whitespace,
   blowing the budget again — compacted content-preserving (identical
   rows, `---` separators restored).
10. **gofmt** on compose.go (import order) + companion/core.go (struct
    field alignment, P3-era) — gate green.
11. **nix build GREEN end-to-end.** Two follow-ups were needed: (a)
    vendorHash re-pinned a SECOND time — 23 fleet commits in the 21 h
    gap (journal/cqrs go.mod among them) moved the FOD graph after my
    first pin; (b) `TestDoctorTreeGofmt` (parallel agent's M11 work)
    shells out to `git`, absent in the hermetic checkPhase sandbox —
    extended its skip guard symmetrically to the existing gofmt skip.
    `./result/bin/tq version` → `tq 0.3.1, go1.27.1`.
12. **Master CI redness triaged.** The failing run (at 563c30c3,
    pre-window) failed on exactly two causes, both already fixed in-tree
    by this wave: nix vendorHash drift (fixed here) and readmodel go.sum
    drift (fixed by the prior session's P0). Next push should go green
    modulo the fleet's own new findings.

## b) Partially done

13. **Full `./scripts/ci-local.sh` matrix: every step I own is green;
    the run as a whole exits red on FOREIGN in-flight items only**:
    - `lint-baseline`: new findings live exclusively in parallel agents'
      files (internal/task/task.go varnamelen, internal/worker/worker_test.go
      inamedparam/tagliatelle, internal/webui/webui_test.go dupl,
      components.go modernize) — their sessions own the baseline regen.
    - `check-status-index`: `2026-10-05_00-33_paperclip-m1-…md` (another
      agent's fresh report) has filename/header date drift
      (2026-10-05 vs 2026-10-04).
    - `check-dead-sha-refs`: two orphaned SHAs in other windows' reports
      (daemon-rebase orphans).
    - `check-ci`: master's latest completed CI run is the pre-window red
      one (causes fixed locally, see a12).
    All other ~90 steps green including cleanroom, consumer-install,
    gosec, gofmt, facade parity, smokes-up-to-that-point, module loop,
    cmd/tq gate.
14. **`nix flake check`** not run this session (nix build + vendor-hash
    + version-sync derivation all green; flake check adds the remaining
    checks incl. webui-css and treefmt — deferred with the matrix).

## c) Not started

15. **P5 — legacy deletion + docs truth pass** (cqrsqlite deletion,
    dual-tally collapse, ADR-0019 stage markers, FEATURES/CHANGELOG/
    TODO_LIST rows, LOC/art-dupl delta) — still owner-gated on the
    deletion-cadence question.
16. **The tag wave** (root v0.3.1 + internal/composition + readmodel
    tags, then delete the check-go-mods pending-tag block) — owner-gated.
17. **Dogfood cutover** (systemd restart against the production
    pre-flip journal; P1 auto-upgrade makes it transparent) — owner-gated.

## d) Totalmente fucked up (honesty section)

18. **Ran test-cmd-tq.sh CONCURRENTLY with the background ci-local** —
    both drive the devmod shim in the same cmd/tq dir; one cleanup
    deleted the other's dev.mod ("open dev.mod: no such file"). Should
    have known: the shim's derived files are per-DIRECTORY, not
    per-invocation. Never overlap cmd/tq gates.
19. **Lost the first full ci-local verdict to /tmp churn** (log file
    vanished across the session gap) and re-ran the whole ~30 min
    matrix. Gate logs belong under a stable path (see e).
20. **Measured a FAILED nix build as rc=0**: `nix build … | tail -5;
    echo rc=$?` captures tail's exit, not nix's (no pipefail in the
    command). Caught it only by reading the output text. Same class as
    the AGENTS "battery rc TO A FILE (no PIPESTATUS)" rule — I violated
    it with a fresh variant.
21. **The AGENTS.md budget dance took four iterations** because I kept
    measuring after daemon folds landed between my read and my edit
    ("file modified since read" three times) and my first edit was
    verbose. Should have written tight the first time and re-read
    immediately before each edit.
22. **21 h interruption gap left my vendorHash pin silently stale** —
    the "verified" state from 03:0x was false at 00:3x. Any
    long-interrupted session must re-verify FOD hashes before trusting
    earlier greens (fleet churn is guaranteed).
23. **Foreign-work collisions consumed ~40% of the session**: papdashboard
    mid-write compile race in the cleanroom (waited for quiescence),
    the formatter's table padding, the budget-note budget pressure, the
    doctor-test sandbox gap. All resolved without reverting anyone's
    content, but the session was slower and noisier for it.

## e) Improvements made beyond the direct ask

- `check-go-mods.sh` now names the pending-tag mechanism explicitly and
  self-documents its deletion condition (tag wave).
- The clean-room depth-1 lesson is captured here (§a8): cmd/tq's
  committed pins must track the tree's UI-dep versions whenever local
  root-module code compiles against them — the version sweep normally
  rides the release, but the cleanroom gate exposes the skew mid-cycle.
- AGENTS.md architecture table now names the composition module and the
  budget gate; the twin budget gates are value-synced.
- The doctor tree-gofmt test is sandbox-safe (git skip), unblocking the
  nix package checkPhase for the whole fleet.

## f) Up to 50 next things (roughly execution order)

1. Re-run `./scripts/ci-local.sh` once the parallel windows land their
   lint-baseline regen + report fixes — expect full green.
2. `nix flake check` (webui-css, treefmt, templ-committed, version-sync
   together).
3. WebUI + httpapi smokes vs the now-default readmodel path
   (`scripts/smoke/webui.sh`, `api.sh`) — not run since P2's flip.
4. `tq serve` dogfood dry-run on a scratch TQ_DB seeded from a legacy
   fixture: auto-upgrade → readmodel default-on → GracefulClose
   ordering, one script (smoke candidate).
5. **P5**: delete `internal/queue/cqrsqlite` + its conform-suite wiring
   (suite keeps running for sqlitev4/postgresv4).
6. **P5**: root go.mod drops cqrsqlite references; module loop re-run.
7. **P5**: collapse dual tally paths in cmd/tq (tallyStats vs
   tallyModelRows) where the S3 design sanctions projection-backed.
8. **P5**: decide + execute the hand journal tailer's fate (ADR-0003
   Phase D read path) once Watcher/SSE serves the dogfood.
9. **P5**: ADR-0019 endgame addendum (S2/S3/S4 + auto-upgrade landed).
10. **P5**: FEATURES.md rows (auto-upgrade, readmodel-default, engine
    fact reads, composition root) with citations.
11. **P5**: CHANGELOG `[Unreleased]` entry for the whole endgame wave.
12. **P5**: TODO_LIST: close ADR-0019 S1/S2/S3 rows (stale), S4 row;
     keep dogfood-cutover row owner-blocked.
13. **P5**: LOC delta (tokei) + art-dupl `-t 4` recount vs the
     2026-09-23 baseline — the deletion dividend.
14. **Tag wave** (owner-gated, see §g): root v0.3.1 + internal/
     {queue/sqlite,queue/sqlitev4,queue/postgresv4,queue/companion,
     readmodel,composition} tags; proxy checks per docs/release/.
15. After the wave: delete the check-go-mods.sh pending-tag block
     (self-deleting by design).
16. After the wave: `check-facade-parity.sh` re-pin to real tags.
17. Dogfood deploy runbook note (P1 makes the restart transparent;
     manual replay = documented fallback; `<db>.legacy-*.bak` hygiene:
     prune after a green week).
18. Post-deploy `tq doctor` verification (auto-upgrade log line +
     backup present + head seq unchanged).
19. Upstream ratification memo M4 (owner-gated TODO row): enqueued-fact
     snapshot detail + CountFacts/FactsSince pushdown.
20. readmodel thin-enqueue side channel (RowSource) dies when upstream
    grows the enqueued detail — track via M4.
21. system.Lookup/metaengine query declarations for readmodel
    collections once upstream exposes journal injection.
22. Sweeper ticks → `system.ManageTimers` — stays no-fit; revisit only
    with a genuinely timer-shaped feature (map §4a).
23. `internal/journal/cqrs` public-facade ruling — owner-blocked TODO.
24. Fix the paperclip report's date drift (foreign file — coordinate or
    leave to its window's next pass).
25. Heal the two dead-SHA citations (fork records) — docs-health
    ANNOTATE material.
26. INDEX BLOAT: 194 live rows > 100 threshold — archive sweep or
    monthly digest row.
27. Make the devmod shim collision-proof: per-invocation dev.mod suffix
    (TMPDIR-based) so concurrent gates can't delete each other's modfile.
28. Adopt a stable gate-log dir convention (repo-root `.gates/` or
    `$XDG_STATE_HOME`), not bare /tmp files (this session lost one).
29. AGENTS.md: one line capturing the cleanroom depth-1 pin lesson
    (cmd/tq UI pins track the tree) — budget-conscious phrasing.
30. Consider extending `check-agents-size.sh` to a pre-commit hook so
    parallel doc editors get the budget signal at commit time, not at
    the next ci-local.
31. `TestExactlyOnceUnderConcurrency`: make the completion deadline
    load-aware (skip under load average > N) or tag as `testing.Short()`
    — it burned triage time in at least two sessions now.
32. Root module `tool` directive / vendor hygiene re-check after the
    next daemon-era bump (inconsistent-vendoring LSP noise persists for
    other agents).
33. The `for-each-module` loop should print PASS counts (conform `-run`
    convention) — observability nit from this session's loop runs.
34. Sweep the stale gopls/golangci vendoring diagnostics after the next
    clean vendor sync (harmless but noisy fleet-wide).

## g) Up to 3 questions (cannot figure out alone)

1. **Tag wave authorization (unchanged from the prior report, now with
   a concrete blocker list):** the replace-free cmd/tq and the public
   facades need root v0.3.1 + six internal tags before this migration
   is consumable outside the repo; until then `go mod verify` in cmd/tq
   rides the new pending-tag WARN. Run the docs/release flow
   autonomously this window, or stop at green-tree + staged checklist?
2. **Dogfood cutover (unchanged):** with P1's transparent auto-upgrade,
   the next systemd restart converges the production journal in place
   (snapshot kept, verified, restorable; `TQ_NO_AUTO_UPGRADE=1` refuse
   path). Do you want the restart + `tq doctor` verification now, and
   should the runbook keep manual replay as the documented fallback?
3. **P5 deletion cadence (unchanged):** delete the hand tailer +
   store-read fallback immediately after the dogfood serves green on
   the projection, or run a short probation with `--read-model=false`
   as the escape hatch first?
