# Execution lap: docs-health harvest (fullcore batch, turn-1 ritual) + webui leftovers recon

**Date:** 2026-09-15 05:13 CEST. **Session type:** interactive (owner-directed, no Task-Queue-ID — the
pasted 2026-09-14 harvest + webui-leftover list, ordered "break into steps, execute and verify one at a
time"). **Commits:** daemon heuristic commits `743ca3c` (AGENTS.md) and `6e40681` (fullcore batch:
fullcore.sh, examples/fullcore/main.go, ci.yml, +2) — on local master, unpushed as of this report.

**Skills loaded at turn 1:** buildflow (verdict: this repo has NO `.buildflow.yml` → NOT BuildFlow-covered;
project tooling `ci-local.sh`/flake.nix stays authoritative), templ-components (consumer guide for the
webui adoptions), docs-health (harvest/annotate conventions for this close-out).

---

## a) FULLY DONE

1. **Turn-1 ritual codified in AGENTS.md** (`743ca3c`): the Session-start ritual bullet now carries the
   two Nth-recurrence misses as written rules — (1) grep prior reports for the SAME task ID
   (`rg -l <task-id> docs/status/`) before anything else; (2) check for and read `CONTRIBUTING.md` /
   `CLAUDE.md` at turn 1, with the recurrence citations (00-52 d4, 02-17 d3). Verified fresh:
   `CONTRIBUTING.md` exists, `CLAUDE.md` does not.
2. **AGENTS.md smoke-list gaps** (same commit): added the missing `fullcore.sh` entry (including the
   50ms/250ms calibration note — this closes 01-59 f7) and the missing `reviews.sh` entry; both were
   wired in ci-local but absent from the documented smoke list.
3. **examples/fullcore consumes `TQ_DB`** (`6e40681`): new `dbDefault()` uses `$TQ_DB` as the `--db`
   default (explicit flag wins), matching the repo-wide sqlite-location convention — the smoke's env
   export is now a REAL guard, not belt-and-braces. Gate: `GOEXPERIMENT=jsonv2 go build ./examples/...`
   → BUILD-OK; LSP finding count unchanged (14 pre-existing advisory warnings, none new).
4. **fullcore.sh hardening** (`6e40681`): `DEADLINE_RUNS` / `DEADLINE_TIMEOUT_MS` env knobs with
   committed defaults (3 / 50ms, 02-04 f2); the main drain run now OMITS `--db` so the TQ_DB default
   path is exercised, asserted mechanically (`[ -s "$TMP/tasks.db" ]`) plus the `rm -f fullcore.db`
   repo-hygiene guard; env-gated postgres variant (drain 4/4 + 50ms deadline lap on `$TQ_TEST_POSTGRES`,
   SKIP line when unset). Gate runs, fresh: `bash -n` PASS; smoke PASS (sqlite, 3 deadline laps, TQ_DB
   proof); knob run PASS (`DEADLINE_RUNS=1 DEADLINE_TIMEOUT_MS=250` — trips at the documented
   calibration point).
5. **ci.yml parity** (`6e40681`): "Fullcore embed smoke" step added to the `test` job (the smoke was
   ci-local-only, 02-04); "Fullcore postgres smoke" step added to `test-postgres` (job-level
   `TQ_TEST_POSTGRES` activates the variant).
6. **tq.exe recon — two paste claims STALE, closed with evidence:** `cmd/tq/tq.exe` (22 MB, on disk)
   is UNTRACKED (zero commits in its history) and ALREADY ignored (`.gitignore:59` `*.exe`). The
   "committed binary + no .gitignore entry" sub-items no longer hold; the surviving real gap is
   cmd/tq's absence from the advisory lint loops (→ f4/f5).

## b) PARTIALLY DONE

1. **Fullcore batch: 4 of 6 sub-items done.** Open: the bash/shellcheck gate script and the
   message-format assertion audit (both spec'd, see c1/c2). The postgres variant is WRITTEN but NOT
   locally verified — no local postgres (probe `/dev/tcp/127.0.0.1/5432` → PG-DOWN); the first CI
   `test-postgres` run is its verifier (step 5 above). Risk: sqlite-mirrored assertions, store path
   already CI-proven; 50ms trip depends on in-process worker-start latency, expected portable.
2. **Shell-syntax gate: measured, not built.** `bash -n` clean over all 46 tracked `scripts/*.sh`.
   shellcheck reachable via `nix shell nixpkgs#shellcheck` (batched sweep 3.7s). BUT the sweep's
   finding-counter grep (`: .* (SC`) was written against an UNVERIFIED output format — 12 files exited
   nonzero (findings present) yet printed "0 findings": check-doc-refs, check-features-ci,
   check-features-roadmap, check-ghost-archives, check-status-index, check-todo-list,
   lib/cmd-tq-devmod, smoke/papdashboard-e2e, sweeps/fanout-libdive, test-cmd-tq, tq-session-status,
   webui-screenshots. TRUE baseline unknown until re-run with `-f gcc` parsing. Nothing was mutated on
   the false reading.
3. **cmd/tq lint-loop gap:** mechanics understood (devmod shim → `GOFLAGS=-modfile=dev.mod GOWORK=off`;
   both ci.yml and ci-local.sh loops are advisory), fix not started. Adding the module creates NEW
   (module, linter) rows → the lint-baseline growth gate requires a DELIBERATE
   `.golangci-baseline.txt` regen with a policy note (AGENTS.md rule).

## c) NOT STARTED

1. **Message-format assertion audit** over `scripts/smoke/*.sh` (every smoke should assert an exact
   output message, not just exit codes — 01-59 §f / paste).
2. **Status-index gate hardening** (02-24 e1–e3): named threshold constant + rationale comment;
   `--self-test` fixture mode (live/archived/backtick/digest/malformed rows) wired into ci-local;
   backtick archived-row convention pin in the README cadence paragraph; digest-row format proposal.
   WARNING-vs-FAIL flip stays owner-BLOCKED.
3. **Webui adoptions:** `navigation.Pagination` (taskPager), `display.ListNote` ("+N more projects"
   chip), `errorpage.WriteError` (new errorpage MODULE dependency → root go.mod + vendorHash fakeHash
   dance; chrome-consistency verdict needed first — WriteError renders its own page chrome). Adoption
   table + guard tests are MANDATORY companions (TestAdoptionTableCoversTemplates).
4. **Upstream chart contribution:** MaxTicks/MaxYTicks verification against templ-components master
   (verify-before-filing + github-voice skills deliberately NOT loaded yet — they gate the step when it
   starts, not the recon).
5. **templ-components a11y-pack release check** → bump + version-pinned audit re-run IF shipped.
6. **Sort-state removable filter chip** (G2, owner-deferred) — park in TODO_LIST.
7. **CSP form-action ruling** — owner-BLOCKED, untouched by design.
8. **TODO_LIST.md recording of all survivors** (this report's f-list routing).
9. **Final gates:** root `build/vet/test -race`, per-module loop, `test-cmd-tq.sh`, webui smoke, full
   `ci-local.sh` replicant. Session interrupted mid-batch — the tree carries unpushed, partially gated
   work until these run.

## d) TOTALLY FUCKED UP

1. **The gate-that-lies, self-inflicted:** I grepped shellcheck findings against a guessed output
   format and reported 12 flagged files as "0 findings" — exactly the AGENTS.md pipeline-masking lesson
   ("verify the raw summaries, not the filtered tail") violated in the same session that read it. The
   correct move was one `shellcheck -f gcc <file> | head` BEFORE writing the parser. No mutation
   followed the false zero, so the damage is wasted signal, not broken code.
2. **nix-per-file sweep:** first shellcheck pass spawned `nix shell` once per file (46 spawns, >90s,
   killed); the batched redo took 3.7s. External-tool sweeps get ONE process invocation, always.
3. **No-op edit op:** my third multiedit op on main.go was old==new placeholder text. Harmless, caught
   immediately, fixed next call — but it shipped in a "3 edits" result and would have silently done
   nothing had I not followed up.
4. **Near-miss, self-caught pre-run:** the first fullcore.sh draft asserted TQ_DB consumption while the
   main run still passed `--db` explicitly — the assertion would have failed its own first execution.
   Caught by re-reading my own script before running it; fixed by dropping `--db` from the drain run.
5. **My own new rule, half-followed:** I did not run the same-ID/topic grep as step 1 (no task ID
   exists for an interactive session, and I did read every cited report up front — the substance —
   but the mechanical first-command ritual is what makes it robust; the fullcore topic HAD prior
   same-ID repeat-dispatch history that proves why).

## e) WHAT WE SHOULD IMPROVE

- Validate every gate's output parser against the tool's REAL output format before trusting a zero
  (one `| head` would have caught §d1).
- Harvest items rot fast in a multi-agent repo: re-verify every "X is broken" claim against HEAD
  before executing — 2 of 3 tq.exe sub-items were already resolved by an earlier session.
- Interactive (no-task-ID) sessions need a TOPIC grep fallback for the turn-1 ritual, not just the
  same-ID grep.
- Daemon heuristic commits fold a 5-file batch into one "chore:" — when explicit commits are
  authorized, commit per task (standing lesson, re-learned).
- CI-only paths (postgres variant) must carry an explicit "first CI run is the verifier" note in
  whatever report claims them — done here; worth a standing rule.
- ci-local.sh gained concurrent insertions mid-session (rename-hygiene scanner at :74-75) — reference
  scripts by name/grep, never by remembered line number, in a multi-agent window.

## f) NEXT (ordered; top 12 are this batch's direct continuations)

1. Re-run shellcheck with `-f gcc`; parse `warning:`/`error:`; get the TRUE baseline for the 12 files.
2. Write `scripts/check-smoke-syntax.sh`: bash -n HARD over tracked `scripts/*.sh`; shellcheck step
   (advisory first, hard after triage) with the verified parser.
3. Wire it into ci-local.sh + ci.yml (check-guard-wiring demands the reference; flake optional).
4. Add cmd/tq to both advisory lint loops via the devmod shim (`GOFLAGS=-modfile=dev.mod GOWORK=off`).
5. Deliberately regen `.golangci-baseline.txt` for the new cmd/tq rows + policy note.
6. Message-format assertion audit over `scripts/smoke/*.sh`; fix exit-code-only smokes.
7. `check-status-index.sh`: threshold → named constant + rationale comment.
8. `check-status-index.sh --self-test` (live/archived/backtick/digest/malformed fixtures); wire ci-local.
9. Pin the backtick archived-row convention in docs/status/README.md cadence paragraph.
10. Digest-row format proposal (date range, row count, top scopes) — pending §g1.
11. Emit live-row count on every run (trend visibility, 02-24 f15).
12. INDEX-BLOAT footer prompt in the report convention (02-24 e4) — needs owner nod.
13. Data-derive the threshold after a sweep window (02-24 f3).
14. Verify Pagination/ListNote/errorpage exist at the PINNED templ-components version (module cache, not docs).
15. Adopt navigation.Pagination in fragments.templ taskPager; templ generate + `nix run .#webui-css`.
16. Adoption-table rows + guard-test updates in the same change (mandatory companions).
17. Adopt display.ListNote for the "+N more projects" chip.
18. errorpage.WriteError chrome-consistency verdict BEFORE adopting (own page chrome vs tq's custom
    Base); a styled error fragment may be the honest resolution — document either way.
19. If adopted: root go.mod require + vendorHash fakeHash dance + `./scripts/check-go-mods.sh`.
20. Load verify-before-filing; verify MaxTicks/MaxYTicks against templ-components master source.
21. If absent upstream: load github-voice; file issue/PR per the upstream CONTRIBUTING.
22. Check templ-components releases for the a11y pack; bump + re-run the version-pinned audit if shipped.
23. Park sort-state chip (G2) in TODO_LIST with its duality-verification note.
24. Record ALL surviving items in TODO_LIST.md (routing per docs-health HARVEST).
25. Keep CSP form-action parked behind §g3.
26. Run the full root verify gate (`GOEXPERIMENT=jsonv2` build/vet/test -race).
27. Run per-module loop + `scripts/test-cmd-tq.sh` (untouched-code insurance).
28. Run webui smoke + full ci-local replicant before any push.
29. Watch the first CI test-postgres run — GREEN is the postgres variant's proof (or fix what it finds).
30. Decide/document fullcore.sh windows-job posture (runs nowhere on windows today — gate or document).
31. Record that the exact-message deadline assertion subsumes 02-04 f3's elapsed-floor idea (or build it).
32. Nightly higher-N deadline lap via DEADLINE_RUNS (01-59 f1) — wire into the nightly workflow.
33. Fold internal/depsweep into the AGENTS.md package table once the concurrent work settles.
34. `git log -S '*.exe' -- .gitignore` to date the ignore rule; close the tq.exe provenance loop.
35. CONTRIBUTING.md: add the pointer to the turn-1 ritual (AGENTS.md owns it).
36. Concurrent agent left `internal/executor/depbump_unix_test.go` UNTRACKED — owner session must land
    or trash it; this session must NOT touch it.
37. Batch the smoke preamble (mktemp/trap/GOEXPERIMENT/TQ_DB) into scripts/smoke/lib.sh (01-59 f24).
38. Consider `--concurrency=1` on the postgres deadline lap to shave flake surface.
39. Re-grep ci-local.sh for fullcore.sh reference by NAME after concurrent insertions shifted lines.
40. Re-run check-guard-wiring.sh after adding the new gate (it fails by design until the wiring lands).
41. Add shellcheck to the flake devShell so ci-local's fallback is not a per-run nix shell spawn.
42. ANNOTATE the templ-components deep-dive §05 leftovers after the webui batch (docs-health pass).
43. After webui batch: AGENTS.md adoption-table prose row for Go-invoked WriteError (RelativeTime precedent).
44. Give check-smoke-syntax.sh a --self-test golden (deliberately broken fixture under /tmp) proving
    the gate can fail.

## g) QUESTIONS (cannot resolve myself)

1. **Digest row vs archive sweep** as the standing monthly ritual for the status index (02-24 g2,
   still open): which do I codify into the README cadence paragraph, and does the digest-row format
   proposal (date range, row count, top scopes) get implemented as part of it?
2. **Shellcheck: HARD gate or advisory-with-baseline?** Triaging the 12 flagged files could be quick
   or a slog; the golangci precedent (advisory baseline + growth gate) is one option, a clean hard
   gate after one fix-up pass is the other. Same gate-vs-advisory class as ruling O5.
3. **CSP form-action:** rule between `form-action 'self'` (restores the no-JS filter fallback) and
   staying at `'none'` (JS-only filters) — or keep it parked behind the security-posture review?

---

_Not BLOCKED; the executed batch is verified (gates cited inline). §c/§f await the next instruction window._
