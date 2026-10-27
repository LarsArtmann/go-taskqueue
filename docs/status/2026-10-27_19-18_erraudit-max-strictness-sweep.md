# Erraudit `--max` Strictness Sweep + Error Modernization — Session Report

**Window:** 2026-10-27, ~18:45–19:25 CET · **Scope:** this session only
**Trigger:** `erraudit lint ./... --max` exit 2 with 57 root-module findings pasted by the operator.
**Report format note:** `.md` at explicit operator-demanded path (status-report skill's canonical format is HTML; the explicit path demand wins — recorded, not propagated as a new default).

## TL;DR

Root-module view under `--max` showed 57 findings (55 `generic_return` + 2 `legacy_as`). The per-module truth: **24 modules, 197 `generic_return`, 5 real `legacy_as` sites (2 root + 3 hidden in the `internal/executor` nested module), 1 `legacy_is` advisory that made even the DEFAULT lint red.** All 5 `errors.As` sites migrated to `errors.AsType`; the advisory triaged as a sentinel value-match and suppressed with a reason; `generic_return` ruled a deliberate non-fix and the policy landed in AGENTS.md §Known Issues. All 24 modules now green under BOTH the default lint and the composed `--max` invocation. Zero code regressions: every touched module's build+vet+test green (executor twice), cmd/tq devmod gate green, AGENTS.md size guard green.

Battery: **10 ok / 0 fail** (counts from captured gate-output files, §Receipts).

## a) FULLY DONE

1. **`legacy_as` fixed to zero, fleet-wide (5 sites).** `errors.As` → `errors.AsType[*T](err)` at `internal/harvest/citation.go:49` (`isDangling`, `*exec.ExitError`), `internal/session/registry.go:195` (`PgrepOwner.Owns`, same shape), `internal/executor/gitscan.go:152` (`git log` empty-repo probe), and `internal/executor/verifygate_test.go:266+312` (both `*VerifyGateError` — regressions against that file's own 5 existing `AsType` call sites, i.e. pre-session drift). Evidence: `erraudit lint ./... --type legacy_as` rc-clean in every module; daemon commits 4530d59b (harvest+session) and ce77552c (executor).
2. **`legacy_is` advisory triaged per the go-error-modernization decision tree — kept, not migrated.** `internal/executor/executor_test.go:154` flags `errors.Is(again, error(got))`: second arg is a conversion expression ("anything else" branch — linter is wrong); it pins the double-`Permanent`-wrap no-op by value identity, no fields to extract. Kept `errors.Is`, suppressed `//nolint:legacyerrors // value identity match: pins double-wrap no-op, no fields to extract` on the call line. This line was making **even the default (no-flag) lint exit 2** in `internal/executor` — pre-existing red, now cured (daemon commit 977b0dff).
3. **Fleet-wide `--max` enumeration.** Hand-rolled per-module-root sweep (`find . -name go.mod` loop, `erraudit lint ./... --max` in each of the 24 module roots): 197 `generic_return` / 5 `legacy_as` / 1 `legacy_is` / **0 findings of any other `--enforce-*` class** (deferred-close, coded-errors, go-error-family, samber-oops all clean under max strictness). Capture: `/tmp/erraudit-permod.txt` (24 rows).
4. **Green invocation established and verified in all 24 modules:** `erraudit lint ./... --max --disable generic_return --disable legacy_is` → rc=0 in 24/24 (captured `/tmp/erraudit-final.txt`, `grep -c 'rc=0'` = 24). Default lint also 24/24 green (`/tmp/ea-default-sweep.txt`, 24).
5. **Flag semantics verified empirically** (binary `erraudit 61a744c`): `--max` ⇒ `--type-aware + --no-suppress + --explain + --disable-extensions` + every `--enforce-*`; **`--disable` composes with `--max`** (disable wins); `--no-suppress` bypasses `//nolint`, so no suppression can green a `--max` run — only fixes or `--disable`; the `legacy_is` advisory alone fails the exit code (rc=2 `[Rejection:violations.found]`).
6. **Tool-interface discoveries, documented:** `lint` subcommand **rejects** `--modules` (fail-closed Post-Flag-Parse hook naming analyze/watch/tree/errorflow/nolint-audit/portfolio as its users) and `--violations-only` — the multi-module hint in the operator's original output ("rerun with --modules=all") is **wrong for `lint`**; per-module-root invocation is the only route. Recorded in AGENTS.md.
7. **Policy decision + memory write.** `generic_return` = DELIBERATE NON-FIX; rationale (check ships off-by-default with "returning error is standard Go practice"; repo idiom is sentinels + `%w` wrap + `errors.AsType` extraction; ~200 signature changes would churn DLQ classification, rate-limit detection, gate-death matching) written as a dense AGENTS.md §Known Issues bullet with the green gate command, the legacy_as/legacy_is stances, and the `nolint-audit --fix` trap warning. Size guard: 22,446 / 23,600 bytes — `TestAgentsDocSizeGuard` green via the cmd/tq devmod gate.
8. **Verification battery, all green:** per-module `GOWORK=off go build && go vet && go test -count=1` for `internal/harvest` (2.7s), `internal/session` (1.6s, `-race`), `internal/executor` (32.6s, re-run 17.5s after the nolint edit); root `go build ./... && go vet ./...`; `./scripts/test-cmd-tq.sh -run TestAgentsDocSizeGuard` (devmod shim builds against LOCAL siblings, so my executor changes compile into it); `gofmt -l` clean on all 5 touched files.

## b) PARTIALLY DONE

1. **`generic_return` policy is decided but not institutionalized.** What works: AGENTS.md bullet + verified green command. What remains: no TODO_LIST decision row, no `docs/problems/` writeup, and the CLI-flag form (`--disable … --disable …`) has no repo config file backing it — every future run must retype the invocation. Blocker: none, just not done (S effort). Needed: owner ruling (§g Q1) to know whether the end state is "non-fix forever" or "go-error-family migration project".
2. **The erraudit gate is not wired anywhere.** Nothing in `scripts/` or `nix` invokes erraudit — the repo's green state is enforced only by discipline. What works: the exact command + per-module loop are documented. What remains: a `scripts/check-erraudit.sh` (with the fleet `--self-test` countable-gate contract) + ci-local wiring (M effort). Blocked only by the advisory-vs-gate policy question (§g Q2).
3. **Evidence durability.** Gate receipts live in `/tmp` captures (quoted by `grep -c` in this report); not archived via `scripts/archive-evidence.sh`. Reproduce commands are cited, so claims stay checkable, but the raw captures will vanish on reboot (S effort).

## c) NOT STARTED

1. **Upstream filing of the erraudit tool bugs** (lint rejects `--modules`; hint text recommends a flag `lint` refuses; `--violations-only` rejected on `lint`; exit-code rc=2 vs the skill's documented rc=1 table). Why: no public repo known for the binary — needs the owner's repo pointer (§g Q3). Priority: high — the misleading hint will re-trip every future multi-module session.
2. **`go-error-modernization` skill update.** Today's verified facts (–-max shorthand semantics, `--disable` composition, `--no-suppress`-vs-max interplay, lint-subcommand flag rejections, rc=2 violation shape) contradict/refine the skill's flag and exit-code tables. Not started: out of repo (crush-config), commit-by-commit there. Priority: medium.
3. **TODO_LIST harvest of §f.** The 50 items below are a brainstorm, not commitments; docs-health HARVEST with routing rigor not yet run. Priority: medium — do it next session or on request.
4. **Regression-prevention tripwire for `legacy_as` drift** (the two root sites and the three executor sites all appeared AFTER the September zero-out — the class re-grows silently because nothing gates it). Not started, pending the gate decision (§g Q2). Priority: high if erraudit becomes a gate, else the cheap grep tripwire variant.

## d) TOTALLY FUCKED UP

Nothing at the code level — all shipped edits compile, test green, and are semantically verified. Two self-inflicted process failures, owned:

1. **Knew-but-anyway `PIPESTATUS` misuse.** AGENTS.md §Known Issues explicitly warns session shells have no usable `PIPESTATUS`; I still wrote `echo "erraudit rc: ${PIPESTATUS[0]:-$?}"` and briefly reported "erraudit rc: 0" from a pipeline whose rc was actually grep's. Caught it myself one step later with a clean rc-to-file re-run (which flipped to rc=2 and drove the real finding). Mitigation: none needed in code; lesson re-priced in §e. Severity: none shipped — but it *was* a wrong claim briefly made.
2. **Two wasted round trips on flag rejection.** Ran `lint ./... --max --modules=all --violations-only` (rejected `--violations-only`), then `lint ./... --max --modules=all` (rejected `--modules`) before reading the subcommand's actual contract — the root `--help` table documents both flags as if generic, which is exactly why the tool's own hint also lies. Cost: ~2 minutes. The lesson (subcommand help ≠ root help) is now in AGENTS.md §Known Issues.

Pre-existing, observed, NOT mine, deliberately untouched: plain `cd cmd/tq && go build` fails against proxy floors (`ReadWorkerFailedMarker`, `ScanWorktreeOrphans`, `DedupPrefix` undefined — the symbols exist in local siblings; ADR-0017 replace-free floors predate them). Expected state; the devmod shim gate is green. Mentioned because a future agent must not "fix" it by editing cmd/tq/go.mod by hand.

## e) WHAT WE SHOULD IMPROVE

1. **Read the known-hazards list before writing the command, not after the wrong result.** The PIPESTATUS slip was pure avoidable: the hazard was one screen up in AGENTS.md. Fix: habit — rc-to-file is the default form for any gate whose rc matters.
2. **Trust subcommand-specific help over root help.** Root `--help` in erraudit documents flags/defaults the subcommands reject. Fix: for any multi-command CLI, run `<cmd> help <subcommand>` before composing invocations.
3. **Ad-hoc loops should graduate to scripts the same session.** The per-module sweep loop worked but lives only in shell history + this report. If a check is worth running twice, it gets a `scripts/check-*.sh` with the fleet `--self-test` contract.
4. **Nested-module blindness is a recurring trap.** The operator's root-only run showed 57 findings; the truth was 203. The root command's warning text buries the count. Fix candidate: the future gate script always runs per-module, never root-only, and prints a fleet table.
5. **Policy decisions need a decision artifact, not just a Known-Issues bullet.** The `generic_return` ruling is the kind of thing that gets relitigated every quarter. Fix: one `docs/problems/` page (or ADR) with the ruling, the numbers, and the adoption criteria.
6. **Linter-to-zero impulses should trigger the skill, not bypass it.** The `generic_return` refactor temptation is exactly the go-error-modernization cargo-cult trap; loading the skill first is what produced the keep-and-document outcome. Worked this time; worth keeping as the standing reflex.

## f) 50 things to get done next (brainstorm — HARVEST routing applies; most are ROADMAP fuel)

| # | Task | Impact | Effort | Category |
|---|------|--------|--------|----------|
| 1 | Owner ruling on `generic_return`: permanent non-fix vs go-error-family migration project | Critical | S | Decision |
| 2 | Owner ruling: wire erraudit as a hard gate (ci-local) or keep advisory | Critical | S | Decision |
| 3 | Write `scripts/check-erraudit.sh`: per-module loop, composed flags, rc-to-file, fleet table, `--self-test` per the countable-gate convention | High | M | Quality |
| 4 | Wire the gate into `scripts/ci-local.sh` (and lint-baseline story: in-baseline or explicitly scoped out) | High | M | Quality |
| 5 | File erraudit upstream: `lint` rejects `--modules` while its own hint recommends it | High | S | Bug (external) |
| 6 | File erraudit upstream: `lint` rejects `--violations-only` (root-help lists it) | Medium | S | Bug (external) |
| 7 | File erraudit upstream: exit-code shape rc=2 `[Rejection:violations.found]` vs documented rc=1 (skill drift) | Medium | S | Bug (external) |
| 8 | Update `go-error-modernization` skill: `--max` semantics, `--disable` composition, `--no-suppress`×max interplay, lint-subcommand rejections, rc table (crush-config repo, by commit) | High | M | Documentation |
| 9 | Cheap `legacy_as` tripwire: grep-based check-script-style gate for `errors.As(` growth in non-vendor Go, independent of erraudit gate decision | High | S | Quality |
| 10 | `git log -S "errors.As"` archaeology on the 5 re-grown sites — find which merges reintroduced the class after the September zero-out; consider a review guard | Medium | M | Quality |
| 11 | Archive the session's sweep captures via `scripts/archive-evidence.sh` (git check-ignore first) so receipts outlive /tmp | Low | S | Documentation |
| 12 | `docs/problems/` page for the generic_return ruling: numbers (197 findings / 24 modules), rationale, adoption criteria | Medium | S | Documentation |
| 13 | TODO_LIST harvest of this §f (docs-health HARVEST, routing rigor) | Medium | S | Documentation |
| 14 | Targeted test pass pinning the migrated sites: `isDangling` nil-err path, `Owns` exit-1 path, gitscan empty-repo path (confirm coverage exists; add if thin) | Medium | M | Quality |
| 15 | Re-read `verifygate_test.go:266+312` diffs once for negative-assertion semantics (ok-branch equality) — cheap second look | Low | S | Quality |
| 16 | Run `erraudit fix ./... --type-aware` dry-run fleet-wide to confirm the fix-subcommand agrees with the hand edits (parity check, no writes) | Low | M | Quality |
| 17 | Decide the gate form: `--disable legacy_is` vs skill Option B `--type legacy_as` (equivalent intent; pick one, document) | Low | S | Decision |
| 18 | Investigate erraudit config-file support (would replace CLI `--disable` flags with a committed config) | Medium | S | Feature |
| 19 | Pin erraudit binary version/source (61a744c — where from, nix-pinned?) so fleet sessions run the same analyzer | Medium | S | Cleanup |
| 20 | `erraudit portfolio` fleet audit across all LarsArtmann projects (cross-project error-health view) | Medium | M | Quality |
| 21 | `erraudit errorflow` JSON export of tq's error surface for docs/architecture consumption | Low | S | Documentation |
| 22 | Verify GOEXPERIMENT=jsonv2 is actually required by erraudit (skill flags it unverified): A/B one module without it | Low | S | Quality |
| 23 | Curated `//nolint:legacyerrors` inventory: grep the fleet (1 site today), keep the list reasoned and minimal | Low | S | Cleanup |
| 24 | Confirm nolint-audit story: repo doesn't run it; if it ever does, scope to erraudit-token files and expect permanent-stale legacyerrors rows (AGENTS.md now warns) | Low | S | Documentation |
| 25 | Idempotency: re-run the composed green command ×2 to confirm determinism (findings are static; cheap sanity) | Low | S | Quality |
| 26 | `--enforce-deferred-close` currently 0 findings under `--max` — decide whether to keep it enabled-by-`--max` or explicitly disable to freeze the gate shape | Low | S | Decision |
| 27 | `--enforce-coded-errors` / go-error-family: separate, bigger policy question (error codes across tq's error surface) — park in ROADMAP unless Q1 adopts | Medium | L | Decision |
| 28 | CHANGELOG O14 check: confirm no operator-visible row needed for this session (internal tooling + internal code) — assert in next release review | Low | S | Documentation |
| 29 | Consider an erraudit `watch` mode loop during future error-refactor batches (exists as subcommand) | Low | S | Feature |
| 30 | erraudit `lsp` subcommand: evaluate editor/crush integration for inline legacyerror diagnostics | Low | M | Feature |
| 31 | Root-command `erraudit . --max` (analyze, not lint): does the root analyze path handle nested modules correctly (`--modules=all` is ITS flag)? If yes, it's the fleet one-shot the hint meant | Medium | S | Quality |
| 32 | Doc the "nested-module blindness" pattern in AGENTS.md §Known Issues (root-only linter runs under-count; generalizes beyond erraudit) | Medium | S | Documentation |
| 33 | Scripts convention: every new gate script's rc goes to a captured file by construction (helper in scripts/lib) | Medium | M | Quality |
| 34 | cmd/tq floor drift: plan the next release cut so cmd/tq floors carry ReadWorkerFailedMarker/ScanWorktreeOrphans/DedupPrefix and plain builds stop failing (go-release flow, NOT hand-edited floors) | Medium | L | Release |
| 35 | After the release cut (34): re-run `cd cmd/tq && go build ./...` to confirm plain-build green again | Low | S | Quality |
| 36 | Sweep the remaining `--enforce-*` classes' docs into AGENTS.md only if they ever fire (they're all clean today — no action, just a checkpoint note) | Low | S | Documentation |
| 37 | DOMAIN_LANGUAGE.md: error-taxonomy terms (PermanentError, VerifyGateError classes, sentinel conventions) — check for drift while the error surface is fresh | Low | S | Documentation |
| 38 | Explore `--explain` capture for one generic_return example and embed it in the problems doc (12) so the ruling shows the linter's actual claim | Low | S | Documentation |
| 39 | Confirm the daemon-swept commits (4530d59b, ce77552c, 977b0dff) carry exactly the 5 intended files + nolint line — `git show --stat` audit | Low | S | Quality |
| 40 | Commit the AGENTS.md policy bullet (working tree now; daemon will sweep — verify it lands intact given the size guard lives in cmd/tq tests) | Medium | S | Cleanup |
| 41 | Raise `TestAgentsDocSizeGuard` budget headroom policy question: 22,446/23,600 leaves ~1.1k — next Known-Issues bullet may trip it | Low | S | Cleanup |
| 42 | Session-shell hazard addition candidate: "linter root hints can name flags the subcommand rejects — verify per subcommand" (already in AGENTS.md erraudit bullet; consider generalizing) | Low | S | Documentation |
| 43 | Try `erraudit lint ./... --type legacy_as --modules`... n/a — instead: document the one-true per-module invocation in scripts/README or AGENTS.md Commands block | Low | S | Documentation |
| 44 | Cross-check facade packages (task/, queue/, worker/ …) for error-type re-export implications IF generic_return is ever adopted (facades re-export internals; new error types must appear there) | Low | S | Documentation |
| 45 | Bench: time the 24-module sweep (measured ~2–3 min this session) and record a CI budget number for the future gate | Low | S | Quality |
| 46 | Evaluate `--severity-threshold`/`--severity` flags once more against 61a744c (skill lists them broken; one cheap A/B while the binary is fresh) | Low | S | Quality |
| 47 | `--no-suppress` A/B on this repo to inventory ALL suppressed findings (audit mode) — expect the 1 legacyerrors site + unknown AST-heuristic skips; document | Medium | S | Quality |
| 48 | Add the composed green command to the repo's Commands block in AGENTS.md (currently only inside the Known-Issues bullet) if it becomes the standing manual gate | Low | S | Documentation |
| 49 | Next session: run full `./scripts/ci-local.sh` once to confirm nothing else drifted while this session worked (pre-push gate, foreign-break retry aware) | High | M | Quality |
| 50 | Close the loop on this report: index row added (done at write time); verify `scripts/check-status-index.sh --mine` passes after the daemon folds the file | Medium | S | Quality |

## §Receipts

Counts are `grep -c` over captured gate-output files, not summarized lines:

| Check | Receipt |
|---|---|
| Composed `--max` green, fleet | 24 ok / 0 fail — `grep -c 'rc=0' /tmp/erraudit-final.txt` (per-module loop, rc-to-file) |
| Default lint green, fleet | 24 ok / 0 fail — `grep -c 'rc=0' /tmp/ea-default-sweep.txt` |
| `--max` finding classes, fleet | 197 generic_return / 5 legacy_as / 1 legacy_is / 0 other — `/tmp/erraudit-permod.txt` (24 rows) |
| Module gates | harvest ok 2.7s · session ok -race 1.6s · executor ok 32.6s + 17.5s · root build+vet rc=0 · cmd/tq devmod targeted ok · gofmt 5 files clean |
| AGENTS.md size | 22,446 ≤ 23,600 bytes, `TestAgentsDocSizeGuard` ok |

Reproduce the fleet sweep: `find . -name go.mod -not -path "*/vendor/*" | sort | while read -r gomod; do d=$(dirname "$gomod"); (cd "$d" && GOEXPERIMENT=jsonv2 erraudit lint ./... --max --disable generic_return --disable legacy_is >/dev/null 2>&1); echo "$d rc=$?"; done`

## g) 3 questions I cannot answer myself

1. **`generic_return` end state:** keep the deliberate non-fix forever, or adopt per-function/go-error-family error types as a dedicated migration project? This decides f1/f12/f27/f44 and whether AGENTS.md's bullet is final or placeholder.
2. **Gate or advisory:** should the composed erraudit command become a hard `ci-local` gate (then f3/f4/f9 need the fleet `--self-test` contract), or stay a manual/advisory check?
3. **Where does erraudit live?** Repo/issue-tracker for the binary (61a744c) so the three tool bugs (f5–f7) can be filed, and whether it's version-pinned somewhere I should keep in sync.

## Owner rulings received (2026-10-27 19:35 CET, after this report was written)

1. **Q1 `generic_return`: ADOPT** — per-function/go-error-family error types as a dedicated migration project. The §a7 "DELIBERATE NON-FIX" wording and the same stance in AGENTS.md were corrected at ruling time (memory must not survive an overrule); the green gate command keeps `--disable generic_return` only until the migration lands.
2. **Q2 Gate: HARD** — `scripts/check-erraudit.sh` (fleet `--self-test` contract) wired into `ci-local.sh`.
3. **Q3 erraudit repo: `github.com/larsartmann/erraudit`** (local checkout `/home/lars/projects/erraudit`) — the three §c1 tool bugs are fileable.

Follow-through harvested as a 3-row dated cluster in `TODO_LIST.md` (gate script; migration project with per-module batch plan; upstream bug filing). §c items 1–3 are thereby ruled/unblocked, not forgotten.
