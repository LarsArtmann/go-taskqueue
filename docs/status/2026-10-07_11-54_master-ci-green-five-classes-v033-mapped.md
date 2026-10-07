# Status: master CI green again — five red classes settled, v0.3.3 mapped

**2026-10-07 11:54** · window opened on the handover of the 10-30 tail-skip
report · head **42cde7d9** (local == origin, tree clean at report time) ·
**CI run 37602432193 FULLY GREEN (8/8 jobs) — first green master since the
vendor-sync gate debuted** · local ci-local rc=1 with ONE red class left
(lint-baseline foreign growth) · next queued action: the v0.3.3 cut.

## a) FULLY DONE

1. **The ca6fd6ef CI watch resolved — it was red, and everything it red
   on is now fixed at source and pushed.** Five distinct classes, in
   order of discovery:
   - **treefmt** (`nix` job): `internal/harvest/redispatch.go` carried a
     one-line `Error()` method from the overnight errname rename — the
     only unformatted file in the tree. Split the body; no behavior
     change. Commit 437879a0.
   - **vendor-sync gate failed every fresh CI checkout** (`test` job,
     run 37593760430): the gate compares the gitignored `vendor/` tree
     hash across `go mod vendor`, but in CI vendor/ never exists at
     start — `before=absent` can never match, so the gate failed its
     own debut run by design. Fixed: an absent tree has nothing stale
     to detect (the regeneration IS the heal), so the gate passes it
     and still fails a present tree that moves. Self-test extended
     with two new pinned cases (absent-start pass + idempotent re-run).
     Commit 177e4fd2. The fixed gate then caught and healed a REAL
     stale local vendor tree on its first local run (re-run green,
     root build exercised).
   - **facade parity** (`test` job, run 37596720390): the overnight
     wake-seam work added `internal/queue.Waker` without the ADR-0016
     facade alias — external consumers of the public `queue` package
     could not see the seam at all. Added `Waker = internalqueue.Waker`
     to `queue/queue.go`; parity gate green (7 facades mirror).
     Commit 03113f96.
   - **webui CSS canonical** (`nix` job, runs 37596720390 +
     37597578541): daemon sweep 42ba266e had carried an UNMINIFIED
     `app.css` regeneration into the tree (pretty 6754-line form,
     byte-identical content modulo whitespace — verified with a
     whitespace-stripped compare). The build script and the canonical
     check both run `--minify`, so every nix CI run since failed on
     formatting drift alone. The pre-sweep canonical was 1 line;
     regenerated through `nix run .#webui-css` to the minified form.
     Commit 1164a1af.
   - **CI gofmt step** (`test` job, run 37602432193 — MY bug, see d1):
     the fixed vendor gate now GENERATES vendor/ early in the CI test
     job, and the repo-wide `gofmt -l .` step — which had never seen a
     vendor tree in CI — failed on vendored upstream code formatted by
     older gofmt. The local gate has always excluded vendor/; scoped
     ci.yml to match. Commit 42cde7d9.
2. **Master CI fully green on 42cde7d9** (run 37602432193: test, nix,
   test-windows, test-postgres, consumer, gosec, govulncheck,
   cqrs-lint — all success). The report commit be76a3ab was pushed by a
   concurrent agent during the window.
3. **Full local ci-local run on the settled tree**: every hard gate
   green — per-module builds/vet/tests (all 24 modules, GOWORK=off),
   facade parity, embedded-facade examples, go.mod hygiene, vendor-sync
   summary green, gosec self-test green, smokes. The ONLY red is
   lint-baseline foreign growth (b1).
4. **v0.3.3 release flow fully mapped** (research, no edits yet):
   CHANGELOG gate needs `## [v0.3.3] - 2026-10-07` (v-prefix — the
   0.3.2 section heading lost its v, but release.sh greps the literal);
   flake.nix `version` attr + ldflags derive from it; UNIFORM require
   sweep v0.3.2→v0.3.3 across root go.mod (13 internal requires) and
   the sub-module go.mods; `go mod vendor` + vendorHash regen after;
   14 `internal/*/v0.3.3` annotated sub-tags pre-cut on the release
   commit BEFORE the gates (check-go-mods.sh verifies pins against real
   tags); then `release.sh v0.3.3` probe → `--tag` → `--push`.
5. **`tq doctor --dlq` designed** (blocked only behind the tag): a
   `DLQ bool` on doctorOptions + `doctorProjectionDLQ` over
   `readmodel.DLQPathFor`/`OpenDeadLetters`/`Count`/`Recent` — the
   projection-sidecar DLQ (fold poison), distinct from the existing
   queue-side `dlq`/`dlq-repair` checks (doctor.go:314/329).
6. **M11 coordination check**: no fresh commits touching
   `internal/harvest/redispatch.go` in the window (mine is the newest) —
   the mid-flight agent from the last report appears settled.

## b) PARTIALLY DONE

1. **ci-local rc=1: lint-baseline is the sole red.** New foreign
   findings: root varnamelen 22→23; cmd/tq cyclop/goconst/staticcheck/
   varnamelen/wsl_v5 across agentpool/audit/bootstrap/doctor/main/
   journalaudit/ask/crush/tasks (+tests); postgresv4 paralleltest
   (openwithpool/wake/store tests); sqlitev4 err113 + noctx
   (adapter/archive/migration/pragma). All in files other agents own
   and are actively landing; my own classes are clean (lint-annotations
   HEAD~1 delta: 0). CI is unaffected (golangci is advisory there);
   this blocks a LOCAL rc=0 only.
2. **v0.3.3 is mapped, not cut.** Nothing edited: CHANGELOG, flake,
   go.mods, sub-tags all untouched. The two new CHANGELOG entries for
   this window (tail-skip fix + M09 slice 1) are drafted in my head,
   not in the file.
3. **Doctor --dlq is designed, not coded** — replace-free cmd/tq needs
   the v0.3.3 readmodel tag on the proxy first (OpenDeadLetters is not
   in released v0.3.2; proxy list confirmed v0.3.2 is the floor).

## c) NOT STARTED

- Composition-root `NewProjectionHost` factory (S4 completion; two
  real consumers: webui serve, httpapi).
- AGENTS.md/FEATURES.md size-guarded M09 fold-in; TODO_LIST
  reconciliation rows (tail fix / composition go.sum / M09 slice 1).
- M26 upstream filings (poll-subscriber lifecycle primitive — this
  window's leak is the evidence; go-nix-helpers mkDefault footgun;
  art-dupl templ suppression; M4 ratification memo; cqrs-lint branch).
- M11 burn killers (safe to start per the coordination check, still
  deferred until the release cut lands).
- Owner §g answers from the 10-30 report (all three still open).

## d) TOTALLY FUCKED UP

1. **My vendor gate shipped two latent breaks past its own step
   boundary.** Debut run 37593760430 red the test job on EVERY fresh
   checkout (before=absent); after I fixed that, the same gate's
   side effect — vendor/ now EXISTS in CI — red the NEXT job step
   (unscoped repo-wide gofmt) in run 37602432193's predecessor. I
   reviewed the gate's own logic twice and never traced "what does
   creating vendor/ do to the steps AFTER mine". The local gate was
   vendor-scoped from day one; diffing ci.yml's gofmt against it was a
   two-line check. Lesson recorded: any gate that mutates shared
   on-disk state needs a downstream-step review in the same change.
2. **The 6754-line app.css drift sat through multiple nix reds
   mis-attributed first.** The treefmt red (real, but tiny) masked the
   CSS canonical red behind it in the same nix job for two runs; the
   daemon sweep that carried the unminified file in was never
   suspected because its stat (report + README + flake.lock) looked
   innocent. The whitespace-identical compare was the moment it
   cracked: content right, FORM wrong.
3. **One interrupted job_output killed the first full ci-local run**
   (log vanished mid-battery, full restart). Cost: ~15 minutes and a
   duplicate battery. Gates' logs should live somewhere the tooling
   can't collect (fixed /tmp path was the mistake).

## e) WHAT WE SHOULD IMPROVE

- **Downstream-step review discipline** for gates/scripts that create
  or mutate shared state (d1). A check: "grep the workflow for steps
  that run after mine and read them against my side effects."
- **The CSS canonical guard should pin the FORM, not just bytes**: a
  minified-shape assertion (e.g. `wc -l ≤ 4`) would have turned the
  pretty-form commit into an instantly-attributable failure instead of
  a 6754-line diff nobody read.
- **lint-baseline under concurrent agents is on its third whipsaw**
  this window series (cmd/tq + postgresv4 + sqlitev4 classes keep
  moving). Either the owners fix-at-source within a bounded window
  (current discipline) or one sanctioned settle-window regeneration —
  owner call, still parked at §g.3.
- **Release readiness of CHANGELOG headings**: the 0.3.2 section lost
  its v-prefix and still shipped (how is unclear); the gate demands
  the literal. A guard pinning the heading shape would kill the class.
- Session-shell resilience: background job interruption destroyed a
  log path; gate logs belong under .crush/ or uniquely-named files.

## f) NEXT (prioritized)

1. Regenerate lint-baseline deliberately (or wait for owners per the
   §g.3 answer) — the ONLY local red on the settled tree.
2. **Cut v0.3.3**: CHANGELOG cut (add the two drafted entries: the
   exclusive-cursor tail fix with the leak fix, and the M09 DLQ
   sidecar + health projection check + doctor projection section) →
   flake version 0.3.3 → uniform require sweep v0.3.2→v0.3.3 (root +
   sub-module go.mods) → `go mod vendor` + per-module tidies →
   `nix build .#checks.x86_64-linux.vendor-hash` → pin the new hash →
   commit → pre-cut the 14 `internal/*/v0.3.3` annotated sub-tags →
   `release.sh v0.3.3` (probe) → `--tag` → `--push` (proxy wait,
   clean-room install, GitHub pre-release).
3. Land `tq doctor --dlq` against readmodel v0.3.3 (design ready, b3):
   DLQ flag + doctorProjectionDLQ + TestDoctorProjectionDLQ cases
   (absent sidecar / zero / count+recent render).
4. Composition-root `NewProjectionHost` factory wiring.
5. AGENTS.md/FEATURES.md size-guarded edit (M09 surfaces; budget 18.5k).
6. TODO_LIST reconciliation (rows closed by the tail fix, composition
   go.sum fix, M09 slice 1) + annotate the 10-30 report's index row.
7. M26 filings — poll-subscriber lifecycle primitive first (leak
   evidence: no owned stop channel, idle journals never deliver).
8. M11 redispatch burn killers (F046–F051) — coordination check now
   clean; start AFTER the release cut.
9. M12 daemon attribution + hooks tooling (F052–F056).
10. M13 S2 one-vocabulary flip (F058–F064).
11. M14 gates: `nix flake check` into ci-local (F066); load-aware
    skips (F067); devmod per-invocation dev.mod suffix (F068); per-user
    golangci cache (F069).
12. M15 quiet-host ci-local capture + evidence archive.
13. M16 token/cost-denominated cap (O2) + UsageToday settlement.
14. M17 JIT frontier scoring (O3), retire DefaultScoreTTL.
15. M18 DefaultBatchItems=3 + prompt outcome-contract rewrites.
16. M19 claim-time executor-type filter, `tq park`, closeoutPending
    leak audit.
17. M20 security pin bundle.
18. M21 audit drill-downs.
19. M22 doctor consumer-liveness + derivation-blind commit census.
20. M23 ci.yml parity + required-checks P1 + toolchain pins — now
    includes pinning the gofmt scoping landed in 42cde7d9.
21. M24 webui customer surfaces (budget meter, dlqfix cards, fragment
    collapse).
22. M25 SQLITE_BUSY class fix (repro harness, bounded open retry).
23. M26 remainder: go-nix-helpers mkDefault footgun; art-dupl templ
    suppression; M4 ratification memo; cqrs-lint branch.
24. M27 docs-health mechanization + standing tail burn (297→0).
25. Pin govulncheck in the flake (agent-runnable).
26. VERSION-SURFACES.md re-pin + release.sh bare-tag-at-gate-time
    redesign.
27. Sweep remaining fold error paths for error-family classification.
28. Smoke-level fold-lag print for diagnosability.
29. HealthCheckDetailed metadata (per-check Duration) if the dashboard
    renders it.
30. Dogfood serve restart (owner-run, O12) — the tail fix + green
    master make the projectionhost flip production-safe by my read.
31. v0.3.1 release-page warning (owner call, carried).
32. Dependabot first-week check (O15, earliest 2026-10-13).
33. `--dep-sweep` retirement row.
34. 13-13 f-list residue: fold-marker.sh, heal task-less mode,
    footer-attach heal script, install-pre-commit hooksPath fix,
    inert-hooks detection.
35. Add the minified-shape pin for app.css (e2 item) when touching the
    CSS guard next.
36. Record the d1 lesson (gate side effects vs downstream steps) in the
    project AGENTS.md known-issues if the owner wants it durable.

## g) OWNER QUESTIONS (3)

1. **v0.3.3 scope and button-press**: master is green (37602432193) and
   the tail-skip fix + M09 slice 1 + wake seam + five CI settles are
   all on it. Cut v0.3.3 NOW as a fix-release (my recommendation —
   replace-free cmd/tq needs the readmodel v0.3.3 tag on the proxy
   before `tq doctor --dlq` can land), or hold for the remaining M09
   slices? `--push` is the irreversible phase (tags immutable, proxy
   caches forever) — confirm the release goes out this window.
2. **lint-baseline policy** (carried from 10-30 §g.3, now with fresh
   evidence — b1's class list): keep the fix-at-source discipline with
   owning agents, or authorize ONE settle-window regeneration of
   `.golangci-baseline.txt`? Note the regen must happen on a quiet tree
   or it pins whatever is in flight.
3. **Dogfood cutover** (carried from 10-30 §g.1): with the tail fix
   pinned and master green, restart the production serve (805 tasks,
   still on the pre-durable-cursor pump with WAL churn) on a fresh
   42cde7d9 build NOW, or only after the v0.3.3 tag is cut and
   install-proven? Operator-run per O12 — your hands, my prep either
   way.
