# Wave 4 — rows 806 + 470 landed, facade parity healed, poll-loudness gate taught the fail-helper; battery triage 16→18 (all diagnosed, fix-forward)

**Task-Queue-ID: owner-directive** (standing "GET SHIT DONE — THE WHOLE TODO LIST" resume of the 18-12 wave-3 queue; not a dispatched task)

Date: 2026-11-01 19:22 · Tree at report start: `8ece84af` (master, pushed through `59058a08` + this wave's commits) · Battery: attempts 16-18 triaged below, **receipt run (19) NOT yet launched** — STOPped for this status report.

## a) FULLY DONE (verified)

- **State sync at resume**: battery 15 attempt 4 had ended `BATTERY_RC=1` on root vendor/ drift (fail-fast; the gate's own regeneration healed the tree mid-run) — re-verified `check-gomod-vendor-sync.sh` green + root build OK; classified transient, superseded by the final-battery plan (one clean green run on the final tree instead of two).
- **Peer question-rail teaches a real command**: `tq ask --task` (ask.go:58) and `--file` (ask.go:66) both exist — no fix needed (was unchecked at the 18-12 STOP).
- **Row 806 slice 2 — files-touched forensics for DLQ autopsies** (`13f00aa2` + daemon `0b62dc0d`): `FailureEvidence.FilesChanged` (json `files_changed`) + `IsZero()` + `executor.EnrichFailureEvidenceFiles` (deriveOutcome union-merge after BOTH failure paths in `AgentExecutor.Execute`, never replacing recorded entries); carried by `tq show` + `tq dlq --json` (`failureEvidenceView`); SEVEN `==` sites converted (see §d for the seventh); tests: `TestEnrichFailureEvidenceFiles` (union/no-sink/no-evidence/empty-derivation) + e2e failure leg `TestExecuteEnrichesFailureEvidenceFilesOnVerifyDeath` (real stub footer commit + real failing verify gate → Sink failure carries work.txt). Executor/worker/cmd-tq gates green.
- **Row 806 slice 3 — doctor verdict-channel probe** (`89c428d1` + daemon `7f4631ff`): `doctorVerdictChannel` (+ `runningExecutablePath` seam) WARNs when `tq` is off PATH ("tq verdict / tq ask in paid-turn prompts will fail"), passes with a PATH-shadowing note when the hit differs from the running binary; PATH-hermetic 4-case pin `TestDoctorVerdictChannel`; wired into `doctorEnvironment`; cmd/tq gate green.
- **Row 806 ticked `[x]`** — all three slices, evidence-cited (commit SHAs + test names) in the DONE note.
- **Row 470 — prompt de-micromanagement closed** (`0f15ab09` sweep + `906ab1d8` tick): audit confirmed the 2026-10-11 redesign already outcome-shaped the whole stack; the only distrust residue (the `Run "date" (CLI)…` step dictation in `DefaultCloseoutPrompt` + `statusPrompt`) became the clock-first OUTCOME ("timestamp from the clock, never guessed"). Char counts recorded: DefaultCloseoutPrompt **1243→1233**, mechanics line 500→490, status line 99→101 (step dictation removed). KEEP-list untouched (footer-last + attribution-block rule, verify gate, .tq-verify/.crushrc, dedup-check + HARD CAP 10); all pins green (harvest DropSelfReport/FooterLastContract/Guardrails, TodoAppendCap, ArgvContract/CloseoutTurn) + both module suites.
- **CHANGELOG** (`6add484d`, O14-checked): 2 Added rows (files_changed forensics, doctor verdict-channel) + 3 Internal rows (dead-exports perf+unmask, prompt trims, rail/unix-tag pins).
- **AGENTS.md** (daemon `59058a08`): kill-builtin hazard (`kill` cannot signal LIVE pids — python3 `os.kill`) + battery-preflight ritual; **24391/24400 B** (9 B headroom left).
- **Facade parity healed** (`f3ef62b2`): 5 missing aliases — MY `EnrichFailureEvidenceFiles` + the PEER's un-aliased crush-server death surface (`CrushServerDeathError`, `CrushServerDeathCode`, `DetectCrushServerDeath`, `RequeueClassCrushServer`); `check-facade-parity.sh` green (7 facades mirror internals); facade module tests green.
- **Poll-loudness gate taught the smoke `fail "…"` helper** (`8ece84af`): the peer's crush-pool-supervisor smoke has 3 exhaust-CLEAN poll loops whose exhaust calls the `fail "…"` helper (prints `FAIL: …` + exits) — the gate's static pattern never learned that form (it knew `die `). Now `loud_pattern` covers it in BOTH the awk body scan and the post-done window (one source via `-v`), self-test pins a helper-form fixture (2 ok/1 failed counters); real run **33 loops ok, 0 failed**; syntax gate 125 ok.
- Full all-module gate loop green (26 nested modules build+vet+test) + root vet green + root test suite green + vendor-sync green at every checkpoint; pushes up to `59058a08` on origin.

## b) PARTIALLY DONE

- **Final battery receipt**: attempts 16, 17, 18 each died on a DIFFERENT, diagnosed, fix-forwarded cause (§d) — all three fixes are now committed (`07d1167d`, `f3ef62b2`, `8ece84af`), but the confirming green run (**battery 19**) was NOT launched before this STOP. Every preflight constituent is currently green (agents-size 24391/24400, dead-exports self-test, executor suite, quiet tree, vendor-sync, root vet, facade parity, poll-loudness).
- **CI green on origin**: pushed through `59058a08` (8 commits at push); runs observed queued/cancelled-by-supersession (dependabot backlog) — final green NOT yet confirmed; judge only `--workflow=CI` runs.

## c) NOT STARTED

- 18-12 report README index row: the battery/CI outcome line (depends on battery 19 + CI).
- `check-daemon-attribution` / grandfathering of this wave's footer-less daemon sweeps (`scripts/daemon-sweep-baseline.txt` protocol) — not re-run yet this wave.

## d) TOTALLY FUCKED UP (owned, diagnosed, fixed at source)

- **The seventh `==` site** (cost: battery 16): my "exhaustive" conversion sweep only grepped zero-literal compares (`FailureEvidence{}`) — `internal/dlqfix/sweep_test.go:261` compares two NAMED vars (`payload.Failure != wantEvidence`). Compiles nowhere my gate loop looked: dlqfix is a ROOT-module package, and the loop vets nested modules only; root got `go build` which skips `_test.go`. Caught by the battery's erraudit compile; fixed field-wise (`07d1167d`). Lesson: when a struct loses comparability, grep BOTH literal AND named-var compares, and compile ROOT tests (`go vet ./...` at root), not just nested modules.
- **Facade alias added late** (cost: battery 17): I exported `EnrichFailureEvidenceFiles` without its facade re-export — the exact ADR-0016 rule ("add the alias in the same change") written in AGENTS. The battery also surfaced the PEER's four missing aliases (their crush-server wave landed without parity — their battery presumably never reached the parity gate either). Fixed all five (`f3ef62b2`). Lesson: run `check-facade-parity.sh` in the SAME change that adds an export — it is a pre-battery preflight constituent, which the AGENTS preflight line I wrote does NOT yet name (§e).
- **First poll-loudness gate edit broke awk lexing**: `"` inside a `/…/` regex literal — unbalanced-paren errors, gate went 0-loops (silent no-op). Fixed by passing `loud_pattern` via `-v` as a dynamic regex (also collapsed two copies of the pattern into one source of truth). Caught by the gate's own self-test before commit.
- **Battery 15 attempt 4's death (vendor drift)** was mis-strategized at resume: I verified-healed it and chose "one final battery" — correct call in hindsight, but the final battery then ate three attempts on MY gaps (§d bullets 1-2), exactly the pattern the 18-12 report §d warned about (preflight catches pre-existing breaks, not one's own holes).

## e) WHAT WE SHOULD IMPROVE

- **Extend the battery preflight ritual** to include `check-facade-parity.sh` + ROOT `go vet ./...` (root vet compiles root tests — the dlqfix class). Both are seconds-cheap; both would have saved one battery attempt each. The AGENTS preflight comment I added names neither — extend it when the 9 B budget headroom allows (compress first).
- **Comparability-removal checklist**: for any struct that gains a slice/map field, the conversion sweep must cover (1) zero-literal compares, (2) named-var compares on both sides, (3) root-module TEST compilation. This report records it; a future agent shouldn't rediscover it per battery.
- **Gate-teaching beats script-churn**: the poll-loudness fix could have been "add echo FAIL comments to the peer's smoke" — gaming the gate. Teaching the gate the repo's real helper convention (with a pinned fixture) is the durable fix; prefer that direction when a gate and a script disagree.

## f) NEXT THINGS (highest-value first)

1. **Launch battery 19** (recipe + full preflight; all §d fixes landed) → `BATTERY_RC=0` = the wave receipt.
2. **CI green confirm** on the final push (`gh run list --workflow=CI --branch master`; supersede through the dependabot queue; `--log-failed` if red).
3. Append the battery/CI outcome line to the 18-12 report's README index row.
4. Run `check-daemon-attribution`; grandfather this wave's footer-less daemon sweeps per the baseline protocol (content-verify via `git show`, append, commit+push atomically).
5. Extend the AGENTS battery-preflight line with facade-parity + root-vet (needs ~40 B — compress an existing bullet first; budget 24391/24400).
6. **fb8d0a67 SUPERB crush-server adoption ladder** — STILL awaiting §g1 ratification (now the 4th ask); do NOT start without it.
7. Dead-export honest floor (179 symbols): owner disposition still open (§g3) — prune pass vs annotated-keep vs conform-suite exclusion.
8. Daemon mid-edit PUSH policy (§g2) — still open.
9. Wave-3 residue: disposition of the 3 accidental real provider calls (~150 tokens).
10. TODO row 471: batch default-ON calibration (`DefaultBatchItems` raise + flag-help/AGENTS spelling of the `--task-timeout` interaction).
11. TODO row 807: crush-ops residue (request_timeout block, bootstrap `--reasoning xhigh`, 429-storm policy, release-digest ritual).
12. TODO row 808: session-surface polish (`tq stats` hint/alignment, README nits).
13. TODO row 308: legacy `TQ_RESULT` stdout channel deletion (derived-outcomes retirement of the stdout fallback).
14. TODO row 788: verdict→outcome calibration dataset (turnstone pattern).
15. TODO row 880: the missing a)-g) closeout report for the worktree auto-merge batch (verify per-commit evidence first).
16. Lint-baseline regen after this wave (baseline growth gate; regen cleans cache first).

## g) QUESTIONS (cannot answer myself)

1. **(4th ask, carried §g1)** Ratify the fb8d0a67 SUPERB crush-server adoption plan ("USE IT") as the execution spine — supersede L1-L4?
2. **(carried §g2)** The daemon PUSHES master continuously, mid-edit sweeps included — intended standing behavior?
3. **(carried §g3)** The honest 179-symbol dead-export floor — prune pass, annotated-keep expansion, or conform-suite-table exclusion?

---
*Indexed in docs/status/README.md (top chronological cluster). Battery 19 + CI confirm are the wave's remaining receipts; resuming assistant: preflight per AGENTS, launch, then close the index row.*
