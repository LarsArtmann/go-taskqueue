# Done-Prompt Window Report — Twenty-Fifth Dispatch

**Written:** 2026-10-30 05:20 CEST · **HEAD at write:** 7f9a8bb6 · **Queue task:** 000001a18b084a6b90d2c7ec5e0700000000

**Window covered:** tasks completing between 2026-10-28 ~18:20 and 2026-10-30 05:15 — the eleventh-dispatch trio closeout (19-38 report), the retryx graduation task, the eleventh-dispatch survivor row 1177 (webui smoke ruling), and the redaction-pin trio. Six tasks, every one with a verifiable closeout trail.

## Gate receipts (this session, captured)

| Receipt | Result |
| --- | --- |
| `tq facts -db ~/.local/state/tq/tasks.db -type task.reprioritized` | 0 facts (band-drift §h) |
| `gh run list` + `gh run view 40999208406` | master CI run at HEAD-adjacent push (b8befe0d, 03:46): **X consumer** (Facade adopter on-ramp, examples/embed), **X nix** (Nix build); test-postgres/govulncheck/gosec green |
| TODO_LIST open-row census | 633 unchecked rows |

## a) FULLY DONE

1. **Art-dupl #9 tracked to resolution posture** (task 000001a1815c1b2f, commit `11eb9ce2`). The templ-directive residual (fragments.templ) dissolved from the canonical art-dupl report at 2026-10-28 HEAD; AGENTS.md §Executor shared seams now pins the layout.templ mobile/desktop twin as intentional and records the upstream ask (LarsArtmann/art-dupl#9, still open) with the "HTML-comment directives SHIP IN PAGE OUTPUT" trap. Verified: the AGENTS.md text at HEAD and the 2026-10-28 canonical-report claim are in place.
2. **Smoke COVERAGE.md freshness gate** (task 000001a181d30052, ticked via `eabab7d3`). `scripts/smoke/COVERAGE.md` leg-freshness is now gated: a smoke file whose first-comment contract line is missing from the fleet table fails the gate, so the leg→assertion index cannot rot silently (01-50 §f5 ask). Row ticked in place with the daemon-sweep landing note.
3. **CI linux smoke confirmation + three foreign cures** (task 000001a1820565e7, closeout `docs/status/tasks/2026-10-28_19-38_task-…308b7b93a300000000.md`, receipts table captured). The linux "Web UI live smoke (no browser)" step confirmed SUCCESS on runs 40207620242 + 40219299484, both merge-base-verified to contain 14cb413a (the fold-lock SIGSEGV fix). In passing the session: (i) root-caused and cured the windows cross-compile red (`//go:build unix` on prompt_footer_test.go, `f6005692`; origin runs 40618199506/40626592985 were the evidence), (ii) landed the nil-Host projection pump module pin (`TestProjectionPumpRunNilHostParksUntilDone`), (iii) shipped the demoted-serve staleness contract on three surfaces (STALE FOLD banner, `/readyz`+`/health` fold-follower WARN, MarkFoldFollower cache invalidation) with the O14 CHANGELOG row.
4. **renameWithTransientRetry graduated to `internal/retryx`** (task 000001a183cffb27, commits `ab201ed0`/`1208cc3a` + the 19-38 session's wiring cures). The windows sharing-violation class was confirmed; the shared home exists (module + `RenameTransient`), readmodel/workerfailed.go:81 is swapped, and readmodel+composition+root go.mods are wired (a concurrent half-landing that left three module gates red was finished and healed in the same window). See §b for the remaining call sites.
5. **Webui smoke auth-phase owner ruling executed** (task 000001a184c3709d, closeout `docs/status/tasks/2026-10-30_04-40_task-…4ef6ec00000000.md`, commit `550047ba`). Ruling: the second serve STAYS on the LIVE smoke db (its fold-lock contention caught the real production SIGSEGV). The row's "if live-db stays" clause executed: the bearer-accepted `/api/stats` body is now PINNED (`completed=2, dead=1`) instead of a vacuous 200-check; full webui smoke green. A daemon-sweep misattribution inside the session was healed with a manual mixed-reset rebuild (attribution now correct).
6. **Redaction same-line pin trio** (task 000001a18af16ef6, closeout `docs/status/tasks/2026-10-30_05-12_task-…c9f600000000.md`, commits `ba0185fe`/`cc536cb7`/`c59e2a66`). Output-side mirror landed (2 RedactMarkers for two same-line adjacent secrets, zero plaintext survival) + touching-but-distinct span pin (strict-`<` merge loop guarded against equality-merge regressions) + the zero-separator limitation documented in AGENTS.md. Executor module suite green; rows 1185/1186/1187 ticked with evidence.

## b) PARTIALLY DONE

1. **retryx graduation row 1165 is still open**: `executor/depbump.go:586+590` remain raw `os.Rename` (the restore-path call site was outside the original audit's cite). Swapping both closes the row. Effort S.
2. **Row 1336 (retryx adoption/ownership verification) untouched**: the module landed via footer-less daemon sweeps; nobody has evidenced the module-gate/vendorHash adoption record.
3. **Demoted-serve contract is surface-complete, policy-open**: the in-page dashboard banner (row 1056), a machine staleness flag on `/api/stats`, and the re-election-vs-stale-forever policy all remain open (owner rulings rowed/blocked).
4. **Origin master CI is green-er but NOT green** (verified this session, run 40999208406): the windows cross-compile red that starved every linux smoke leg is cured at HEAD, but two jobs remain red — (i) `nix`: Nix build hash mismatch (vendorHash drift under the retryx/go-health graph changes — the mechanical flake edit has not landed), and (ii) `consumer`: "Facade adopter on-ramp build (examples/embed)" fails, plausibly the same untagged-root-package release-floor class as the cmd/tq tidy leg (dlqbackoff/retryx absent from v0.3.4) — undiagnosed.

## c) NOT STARTED

- The dangling-citation cure (964-line dead-SHA detonation from the 2026-10-28 double filter-branch; rows 1349/1350 + the consolidated survivor) — no heal commits in the window.
- The erraudit `generic_return` migration project and the `check-erraudit.sh` gate script (rows 1081/1311/1333) — the check-doc-refs red it owns is still standing.
- SC1007/SC2120 script-syntax re-greens (rows 1330 + twentyfourth append) — still open.
- The silent done-prompt death diagnosis + checkpoint-early protocol (twentythird append) — two worker runs produced zero artifacts; uninvestigated.
- Crush client/server relations ladder L1/L2 — blocked on owner GO, nothing started.

## d) TOTALLY FUCKED UP

1. **Origin master CI is red on two fronts at the window's close** (run 40999208406, verified): nix (vendorHash) + consumer (examples/embed facade on-ramp). Every "tick on a green origin run" row (904, 1262, and the 19-38 session's own) stays starved. The vendorHash half is a mechanical flake edit; the consumer half is undiagnosed.
2. **The release-floor class keeps hardening**: cmd/tq's vendor-sync tidy leg AND now plausibly the examples/embed on-ramp both cannot go green until a tag wave exists (dlqbackoff/retryx are post-v0.3.4 root packages). Master-stays-red is being elected by default because the release step is owner-gated — that is a policy outcome nobody chose.
3. **Daemon-sweep misattribution struck again** (04-40 closeout §d1): the heal script stamped a foreign agent's gofmt sweep under this window's footer; manual mixed-reset rebuild fixed it, but fork-records from the first heal are now dangling (unconsumed, per §g1 of that closeout). The attribution-guard row (twentythird append) is written and unbuilt.
4. **Historic damage carried, not healed**: the 2026-10-28 double filter-branch dead-cite explosion (~964 dangling cites) sat ~2 days with no heal; the scratch `./tasks.db` trap (untracked, 189 facts, silently answers bare `tq facts` wrong) bit the twentyfourth dispatch and is rowed BLOCKED on owner disposal.
5. **Nothing in this window's own six tasks broke anything**: all three closeouts report zero rework, first-run-green tests, and healed attribution. The debt above is inherited, not introduced — except the consumer-job red, whose cause is unknown (flagged for §f diagnosis rather than attributed).

## e) WHAT WE SHOULD IMPROVE

1. **Diagnose-before-rowing for CI reds.** The consumer-job red is rowed nowhere because nobody opened the failing step. A session-start CI triage probe (19-38 §e3 idea) mapping failing jobs to signatures would have caught it days earlier. The vendorHash cure being unlanded despite being mechanical says the same thing: mechanical cures rot while owner-gated pushes block their verification halves.
2. **Release-floor reds need an explicit posture row**, not per-incident rows: cmd/tq tidy, examples/embed, and the nix job are all "green is owned by the release process" gates. One policy row (advisory-until-wave vs master-stays-red) replaces three spiraling diagnoses.
3. **The heal-script attribution guard is the highest-leverage unbuilt tooling** — two windows in a row (twentythird append, 04-40 §d1) paid the manual-rebuild tax. Same for the fork-record lifecycle question.
4. **Vacuous-assert hygiene generalizes** (04-40 §e4): the `/api/stats` body pin was the first of its class; a sweep across scripts/smoke/* is rowed and should stay cheap.
5. **Redaction fixture coupling** (05-12 §e1): the AWS×Google touching-span fixture is silently pattern-width-dependent; a test-time width self-check would keep the pin honest if the pattern table evolves.

## f) NEXT THINGS (report-only; the TODO_LIST got the deduped top slices)

1. Bump flake.nix vendorHash at HEAD (nix-job cure; mechanical) — NEW ROW.
2. Diagnose + cure the consumer job's examples/embed facade-on-ramp red (run 40999208406) — NEW ROW.
3. Swap `executor/depbump.go:586+590` onto retryx; close row 1165.
4. Land row 1336: retryx module verification + adoption record (GOWORK=off gate, vendorHash, ownership).
5. Owner ruling: release-floor gate posture (cmd/tq tidy + examples/embed + nix vs tag wave v0.3.5).
6. Tag wave v0.3.5 (dlqbackoff, retryx, eleventh-dispatch fixes) — unblocks 5.
7. Build the heal-daemon-sweep attribution guard (per-commit file-stats + `--from` requirement on multi-commit spans).
8. Resolve the fork-record consumer question / add `--prune-forks`.
9. Dead-SHA mass heal for the filter-branch explosion (rows 1349/1350).
10. Land `check-erraudit.sh` + clear the check-doc-refs red.
11. Row 1188: SecretHits interval-union oracle property test (subsumes the two span pins).
12. Byte-side 2-marker redactBytes mirror (rowed by the twentyfourth dispatch — execute it).
13. In-page demoted-serve banner (row 1056) riding the MarkFoldFollower seam.
14. Machine staleness flag on `/api/stats`.
15. Smoke assert sweep: status-code-only JSON asserts → body-pinned.
16. Silent done-prompt death diagnosis + checkpoint-early protocol (twentythird append).
17. Abandoned-claim liveness policy sheet (owner-blocked; draft ready to review).
18. SC1007 + SC2120 script-syntax re-greens.
19. Scratch-db trap cure (owner data-disposal ruling; until then document "always pass -db or export TQ_DB").
20. Crush client/server ladder L1 (`--crush-server` executor seam) once owner GO lands.

## g) QUESTIONS FOR THE OWNER

1. **Release-floor posture**: should the release-owned gates (cmd/tq vendor-sync tidy leg, examples/embed facade on-ramp, nix vendorHash-between-waves) be ADVISORY until a tag wave lands, or is master-stays-red the accepted contract? (Kin of the 19-38 closeout's g2.)
2. **Push/tag cadence**: ~50+ local commits including every CI cure remain unpushed-adjacent while rows 904/1262 starve on "green origin run" contracts — who runs the push + v0.3.5 tag wave, and can it be authorized as a standing agent batch?
3. **Consumer-job red triage**: if the examples/embed on-ramp failure IS the untagged-root-package class, do facade adopters build against the tag (making the wave mandatory) or against master (making the gate graph-sensitive)? This decides whether item 2 of §f is an S cure or rides the release.

## h) BAND DRIFT

`tq facts -type task.reprioritized` over the production journal (`~/.local/state/tq/tasks.db`) returns **0 facts** — no priority moves recorded in the window or the journal's lifetime. **None recorded.**

---
*Verdict recorded in-shell at close. Window: 6/6 tasks landed and verified; 2 new TODO rows appended (deduped against 1165/1336/1320 and all twentythird/twentyfourth appends).*
