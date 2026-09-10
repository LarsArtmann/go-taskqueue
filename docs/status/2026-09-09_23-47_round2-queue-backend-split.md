# Round-2 Modularization (Store Backend Split) — Session Status (2026-09-09 23:47)

**Session scope:** the second `go-modularize` run — a direction-neutral
re-modularization assessment of the post-ADR-0011 module set, followed by the
split it dictated: `internal/queue` → contract + `queue/sqlite` +
`queue/postgres` (ADR-0012). Report covers this round only; the previous
report (`2026-09-09_23-21`) covers round 1 + the review sweep.

**End state:** `ALL CI GATES GREEN` on commit `1320cf4` — 7 internal modules
all build/vet/test green under `GOWORK=off` (disk-derived loops), root race
suite 12/12, vendorHash refreshed, `nix build` + `nix flake check` green,
`tq version` → 0.2.0, tree clean, nothing pushed.

---

## What did I forget?

1. **The proposal's key evidence claim was wrong, and only the compiler
   caught it.** I asserted "zero shared unexported helpers" between the
   backends based on a symbol-intersection check — the regex was too narrow.
   Reality: postgres.go used seven micro-helpers defined in sqlite.go
   (`ms`, `mustJSON`, `maybeJSON`, `failureDetail`, `cancelReasonDetail`,
   `cooperativeCancelDetail`, `escapeLike`). The build failed mid-extraction
   and I mirrored the helpers into a `mirror.go` (per the ADR-0007
   conformance-mirroring philosophy). The proposal now carries a correction
   card — but a trial-move compile BEFORE proposing would have made the
   proposal right the first time. This is the same lesson as round 1's
   go-install miss: **verify structural claims with the compiler, not with
   string tools.**
2. **Replace-path arithmetic off by one** in the first `queue/sqlite/go.mod`
   (`../../../journal` instead of `../../journal`) — `go mod tidy` failed
   loudly and it was a 30-second fix, but I wrote the path from memory
   instead of deriving it.
3. **Doc citations for moved files were not swept proactively.** The
   doc-ref guard caught `FEATURES.md` citing `internal/queue/sqlite.go` (and
   my own first fix introduced a second bad relative path
   `queue/sqlite/store_test.go` before the guard re-caught it); ADR-0007's
   citation also needed repointing. A file move should ship with a citation
   sweep in the same commit, not ride on the final guard.
4. **Import insertion by sed was blind.** I appended the sqlite import to
   every consumer file; files that no longer used `queue.*` ended up with
   unused imports, and `goimports` cleaned up after me. Should have run
   goimports as part of the transform pass from the start.
5. **A head-truncated rg gave me an incomplete consumer list** in early
   research (cmd/tq was missing from the first "who imports backend symbols"
   output because `head` cut it). The full re-run caught it; the lesson is
   `| head` on inventory commands lies about completeness.
6. **The first release.sh tag-derivation was a convoluted three-stage sed
   chain** — I drafted complex where a one-expression sed worked. Rewrote it
   cleanly and tested both the v0.3.0 derivation and the v0.2.0
   already-exists behavior.

## What could I have done better?

- **Compiled before proposing.** A five-minute trial `git mv` + build (then
  revert) would have falsified the helpers claim during Phase 2 research,
  not Phase 6 execution.
- **One mechanical pass, done completely.** The consumer rename mixed sed
  (qualified patterns), per-file edit-tool import fixes, and a follow-up
  goimports cleanup. A single scripted pass ending in goimports would have
  produced fewer intermediate broken states.
- **Treated the doc-ref guard as a step, not a net.** It worked — but the
  right order is sweep → commit, guard as confirmation.

## What could I still improve?

- The round-1 architecture D2 diagrams now show the old single queue box —
  they are snapshots (fine) but should be annotated when docs-health next
  runs, or regenerated as round-2 diagrams.
- The dead-export list from the round-1 review is stale: the split renamed
  or moved several of its entries (`queue.WatermarkEntry` is now contract,
  `SQLiteStore`/`OpenSQLite` are gone; `queue.ErrEmptyType`,
  `queue.RequeueEvidence`, `worker.ExpBackoff` etc. still stand). Re-derive
  before pruning.
- `nix run .#test` coverage question (root-only vs per-module) remains
  unverified from round 1.
- Postgres backend still has zero production consumers — the split made that
  fact visible and honest, but only ROADMAP CLI wiring resolves it.

---

## a) FULLY DONE

| Work                                                                                                                                                                                                                                 | Evidence                                                    |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------- |
| **Re-modularization assessment (Phase 1.5)** — scored all 6 modules; every one Keep except queue (cohesion 3, depth too coarse)                                                                                                      | scoring table in round-2 proposal                           |
| **Queue split executed (ADR-0012)** — contract (`internal/queue`, deps: task+journal ONLY) + `queue/sqlite` + `queue/postgres` driver-style modules; import paths per database/sql convention; 17 consumer files / ~57 sites renamed | all module gates green                                      |
| **Helper mirroring** — 7 micro-helpers mirrored one-for-one into `postgres/mirror.go` with the mirroring rationale documented                                                                                                        | compiler-clean; ADR-0012 §2                                 |
| **`WatermarkEntry` promoted to the contract** (returned by both backends)                                                                                                                                                            | queue.go                                                    |
| **Root drops pgx entirely**; contract-only importers stop compiling sqlite+pgx; queue's go.sum carries zero external deps                                                                                                            | root go.mod after tidy                                      |
| **worker gains queue/sqlite as second documented test-only dep** (real-store suite)                                                                                                                                                  | worker go.mod + ADR-0012 §5                                 |
| **Disk-derived tooling** — ci-local.sh (windows cross, module gates, lint loop, both hygiene audits), ci.yml (Linux + windows jobs), release.sh (tag cutting) all enumerate `find internal -name go.mod`                             | verified live: 7 modules gated, audits clean                |
| **Postgres conformance CI job repointed** to `internal/queue/postgres`                                                                                                                                                               | ci.yml                                                      |
| **Tags cut locally** — `internal/queue/{sqlite,postgres}/v0.2.0`; release.sh now cuts all internal-module tags at every release                                                                                                      | git tag listing; derivation tested for v0.3.0 + resume case |
| **vendorHash dance ×1 this round** + `nix build` + binary check (`tq version` 0.2.0)                                                                                                                                                 | green                                                       |
| **Docs** — ADR-0012; AGENTS.md (7-module map, new table rows, `sqlite.Open` names); FEATURES.md (names + repointed paths); CHANGELOG `[Unreleased]`; ADR-0007 path annotation; round-2 proposal + in-execution correction card       | committed c714847, 1320cf4                                  |
| **Full final gate** — `ALL CI GATES GREEN` (module isolation ×7, windows cross ×7, race suite, smokes, doc checks, nix)                                                                                                              | ci-local output on 1320cf4                                  |

## b) PARTIALLY DONE

1. **8 local tags unpushed** (5 round-1 + 2 round-2 + none for root since
   v0.2.0): the go-install fix and the new backend tags stay latent until an
   owner-gated push.
2. **Round-1 architecture diagrams** depict the pre-split queue module —
   accurate as snapshots, stale as current truth (annotate or regenerate).
3. **Dead-export prune prep**: list needs re-derivation after the renames;
   not started.
4. **docs-health HARVEST** of the two status reports' (f) lists into
   TODO_LIST/ROADMAP — still pending your instruction.
5. **Release-gate smoke script** (fixture go.mods for the allowlist/tag
   gates) — designed, not built.
6. **CI parity**: hygiene audits still run only in ci-local.sh, not ci.yml.

## c) NOT STARTED

- **Postgres CLI wiring** (ROADMAP): the consumer that would make
  `queue/postgres` import-relevant to the binary and re-add pgx to root —
  explicitly out of this round's scope.
- **bdd-testing / full file-by-file review / naming sweep of the new
  identifiers** (`mirror.go`, `postgres.Open` call sites) — carried over,
  still deliberately unstarted.
- **Black-box conformance-suite unification** — considered and REJECTED this
  round (mirroring is the repo's ADR-0007 stance); listed only so nobody
  re-opens it without an ADR.
- **govulncheck / gosec** in CI (carried over).

## d) TOTALLY FUCKED UP (all caught + fixed inside the round; nothing broken now)

1. **False proposal evidence** ("zero shared helpers") — compiler falsified
   it mid-execution; helpers mirrored; proposal corrected + ADR-0012
   documents the mirroring decision.
2. **Replace-path off-by-one** in queue/sqlite/go.mod — tidy failed loudly,
   fixed in minutes.
3. **Stale + half-wrong doc paths** — FEATURES cited the moved sqlite.go;
   my first fix cited a non-root-relative test path; both caught by the
   doc-ref guard, both fixed; ADR-0007 annotated.
4. **Unused-import fallout** from blind sed insertion — goimports cleaned.

## e) WHAT WE SHOULD IMPROVE

- **Compiler-verify structural claims before writing them into proposals**
  (trial-move + build beats any regex inventory; two rounds, two instances).
- **File moves ship with citation sweeps** — doc-ref guard is the net, not
  the step.
- **Never pipe inventory commands through `head`** when completeness is the
  point.
- **End mechanical rename passes with goimports**, always.
- The disk-derived tooling pattern this round set up is worth keeping as the
  house style: adding a module now requires zero script edits.

## f) UP TO 50 THINGS TO GET DONE NEXT

Carried from the previous report unless marked NEW (routing per docs-health:
most extra items are ROADMAP fuel; — BLOCKED marks owner-gated).

1. **P1 — owner-gated**: push master + all 8 local tags (go-install fix +
   backend tags stay latent until then).
2. **P1**: decide `internal/consumer`: wire into `tq serve` or delete (ADR
   outcome written either way).
3. **P1**: re-derive the dead-export list post-rename, then prune (task,
   journal, queue, worker, executor cores).
4. **P1**: owner decision on the three near-identical interfaces
   (`consumer.Source`, `budget.FactSource`, `papdashboard.FactSource`).
5. **P2**: Postgres CLI store wiring (ROADMAP) — gives queue/postgres its
   consumer; consider `--store postgres://…` flag on worker/serve.
6. **P2**: decompose `cmdAgentPool` (main.go:614–1271; now shifted by
   renames — re-locate) into flag-block + actor-wiring helpers.
7. **P2**: release.sh gate smoke script (fixture go.mods, positive+negative
   paths for the allowlist + require-tag gates).
8. **P2**: CI parity — hygiene audits into ci.yml.
9. **P2**: fix executor stub-agent `text file busy` flake (serialize re-exec
   or temp+rename).
10. **P2**: docs-health HARVEST both status reports into TODO_LIST/ROADMAP.
11. **P2**: annotate or regenerate the round-1 architecture D2 diagrams
    (queue box is pre-split).
12. **P2**: verify `nix run .#test` covers the sub-modules; extend if
    root-only.
13. **P2**: after push: verify proxy serves each internal tag
    (`go list -m -versions` per module), then run the clean-room
    `go install` once for real.
14. **P2 — NEW**: compile-time interface assertions in both backends
    (`var _ queue.Store = (*Store)(nil)`) — proposed in round-2 docs, not
    yet added.
15. **P2 — NEW**: naming sweep of round-2 identifiers (`mirror.go` file
    name, `postgres.Open` call-site readability).
16. **P3**: `meta.description` on the four flake apps.
17. **P3**: `nix flake check --all-systems --no-build` once post-split.
18. **P3**: data-model review of `task.Task`/`Status` (branded IDs?).
19. **P3**: dead-code audit script (exported-with-zero-importers) as a
    periodic check.
20. **P3**: govulncheck step in CI.
21. **P3**: gosec advisory scan.
22. **P3**: templ-components deep-dive audit.
23. **P3**: webui dedup deep pass (render 728 + handlers 527 LOC).
24. **P3**: httpapi/webui API-surface split-brain check.
25. **P3**: full-core example (worker + executor + queue) proving the embed
    story; now also a backend-choice example (sqlite vs postgres import).
26. **P3**: document the round-2 release flow in the release checklist doc.
27. **P3**: version-surface inventory doc (flake ×2, tag, CHANGELOG).
28. **P3**: lint-baseline slice-triage: wrapcheck (50) first.
29. **P3**: varnamelen slice-triage (50).
30. ~~**P3**: `ExitCause` → `ExitError` rename consideration (errname).~~ done at `bafc720`
31. ~~**P3**: `go mod verify` per module in CI.~~ done at `533f5bc`
32. ~~**P3**: golangci per-module runs in ci.yml.~~ done at `7f5d699`
33. **P3**: Dependabot/renovate policy decision.
34. **P3**: `tq doctor` multi-module awareness check.
35. ~~**P3**: multi-repo smoke against nix-built 0.2.0 binary.~~ done at `7e32b40`
36. **P3**: README architecture blurb (module map + gate commands — now 8
    modules).
37. **P3 — NEW**: README store-backend section: two driver modules, how an
    embedder picks one (import-line choice).
38. **P3**: `tq version` vs flake version golden test (kills the drift class
    at test level).
39. **P3**: docs-health VERIFY pass over TODO_LIST for split-invalidated
    items.
40. **P3 — NEW**: FEATURES.md row for the backend split (structural feature
    note; check-features-roadmap honesty).
41. **P3 — NEW**: evaluate `queue.Store` interface segregation (25+ methods;
    read-side vs write-side split) — data-model territory, next natural
    refinement IF ever.
42. **P3**: `internal/*/vX.Y.Z` version-bump reminder in the release
    checklist (requires bump when sub-modules change semantically).
43. **P3**: toolchain alignment gate across all 8 go.mod `go` directives.
44. **P3 — NEW**: conformance-suite gap check: sqlite's white-box suite vs
    postgres conformance — confirm the mirrored suites still cover the same
    behaviors post-move (name-level diff review).
45. **P3 — NEW**: `queue/queue.go` contract doc pass: the Store interface
    deserves package-level docs now that it stands alone as a module.
46. **P3**: agent-pool dogfood: stale queued tasks may verify with the OLD
    single-module default command — drain or cancel.
47. **P3**: confirm `scripts/smoke/multi-repo.sh` passes (not in ci-local's
    smoke list; README claims it).
48. **P3**: document the TQ_TEST_POSTGRES docker one-liner next to the
    conformance job.
49. **P3 — NEW**: measure CI time impact of the disk-derived module loops
    (`-count=1` forces rerun; tune if it dominates).
50. **P3**: after next real release, write the "first multi-module release"
    retrospective into docs/release.

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **`internal/consumer` — wire or delete?** (Carried, still open: the
   ADR-0009 dispatcher has zero production importers; wiring it into
   `tq serve`'s tailing vs deleting is your intent call.)
2. **Push authorization** — now ~30 commits and 8 local tags (root go-install
   fix + five round-1 module tags + two backend tags). The proxy-resolvable
   story is complete locally but latent until pushed. Push now, or wait for
   a release moment?
3. **Is Postgres CLI wiring slotted for v0.3?** The split made it the
   obvious next consumer (`--store postgres://…`); if yes, root re-adds pgx
   and the backend module gets its binary path — and I would sequence it
   BEFORE any public-API promotion so the driver story ships complete.

---

_Report by Crush (glm-5.3-flash), 2026-09-09 23:47 CEST · round-2 commits
ede6def (proposal) → 1320cf4 (path fixes) · final ci-local run: ALL CI GATES
GREEN on 1320cf4. Markdown per explicit user instruction (skill default is
HTML — override flagged, consistent with the 23:21 report)._
