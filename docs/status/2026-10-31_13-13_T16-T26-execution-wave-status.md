# Session status — T16–T26 execution wave (interrupted pre-final-battery)

**2026-10-31 13:13 CET · Continuation of the SUPERB pareto plan under the 2026-10-30 blanket GO · Scope this wave: T16 remainder, row 1405 half, release.sh leg, T17, T18, T21 tail, T24 slice, T25 slice, T26 keeper. INTERRUPTED by owner STOP before the final battery + push — 44 commits sit UNPUSHED on master (HEAD 165b8558).**

---

## a) FULLY DONE (this wave; each with its landing commit)

1. **T16 evidence cluster (8 rows)** — `tq dlq --json` ({count,total,evidence_read} summary lead + JSONL rows); bounded evidence prefetch (cap 200, filtered runs always read); unknown-term typo warning (executor.KnownEvidenceTerms/UnknownEvidenceTerms — ONE knowledge home, webui datalist delegates to it); FilterBar evidence control is a datalist-backed TEXT input (set-spec grammar round-trips); webui "N of M dead tasks match" count context (narrowed-to-zero ≠ empty-state); smoke row counting via the JSON summary; wrapper `evidenceStageMatches` deleted; executor-side matcher table pin; README grammar row. Commits 14424324 + ticks 66767d8b. Row 446 stays owner-ruled (autopsy-outcome ruling pending).
2. **Row 1405 both halves** — commit-message SUBJECT claims scanner in check-tick-claims.sh (blocking-action regex; cited-sha-else-own-diff verification; both/two/N count enforcement); live run catches the df2addff "two rows blocked" residue; self-test 6/7/1 counter battery. Commits ~5c0a58e5..31fddce2 (daemon-swept), tick b66890a0.
3. **release.sh fresh-GOMODCACHE re-tidy gate** — root + cmd/tq re-tidied in a scratch copy (fresh cache, GOPRIVATE unset, public proxy forced); committed go.mod/go.sum must be byte-identical; **live-verified against HEAD: both modules IDENTICAL, rc=0**; chmod-before-rm cleanup (0444 extracted cache). Commit e62390bd.
4. **T17 armor rows (11)** — dirty-tree park OWN pool-health lane (dirtyTreeStreak, WARN at 96 ≈ one day of ladder parks; SkipStreak stays env-only) pinned TestPoolHealthDirtyTreeOwnLane; `tq tasks --parked` dirty-tree hint line + requeue_class on JSON rows; DOMAIN_LANGUAGE dirty-tree-park vs env-streak-burn row; ErrDirtyTree renders "… [dirty tree]" (pinned); **production measurement: ZERO parked/dirty population (read-only sweep)**; env-streak burns classified in `tq audit --journal` (FailureSummary total/burns/byVerifyStage); pool-health `backingOff` JSON aggregate; `tq stats` verify_deaths_by_stage counter; tq api lockout surfaces (usage + startup policy banner); httpapi auth-plane body-shape pin (TestAuthPlaneBodyShape) + package-doc contract line. Commits 61dbd5e8, 9e18023e, a6b6193e, 005e07cc + ticks aefac8a2.
5. **T18 readmodel rows (9)** — DLQ-sidecar derivation seam (dlqPathForDeployment + exported DeploymentDLQPath/LegacyDLQPath; doctor composes nothing); doctor both-sidecars WARN row (projection-dlq-legacy); pragma-union same-source pin (ProjectionEnginePragmas + per-tier twins both sides); stats/doctor documented as DELIBERATE light-reader exceptions to the runtime seam; **transition-helper table FOUND A LIVE BUG — same-status transitions netted -1 (silent per-event counter drain); both helpers now return the empty delta**; stats read-only mount + lag note; WithFoldLockWait injectable (composition suite 5.1s→0.2s); applied-sync-tier probe (PRAGMA synchronous per tier through the real driver). Commits 5939303a-region, 17d6fa44, 9484ac41, bec1afeb, b57f448e + ticks 8a7a316c.
6. **T24 slice** — depbump structured failure evidence on every failure path (Stage=depbump, exit code, tail; KnownEvidenceTerms grows the stamped word, pinned). Commit ce2f95a7-era (daemon-swept), tick in 8a7a316c.
7. **T25 slice** — companion LIKE case parity: LOWER() both sides on Query/PayloadContains; mixed-case divergence pinned in the conform suite and **verified green on BOTH backends (sqlitev4 suite + a live scratch postgres cluster)**; scripts/smoke/postgres-conform.sh — the scratch trust-auth cluster as ONE command (free port, readiness probe, zero-pass=FAIL), live-proven rc=0 with 76 cases green. Commits 4996d120 + ticks acdc2c1c.
8. **T26 slice (row 1449)** — crush-server keeper + supervisor: client-shaped keepalive dial (45s) + backoff-ladder restart on unexpected exit (idle suicide can no longer kill the pool's run.Group). Commit 48c5633c.
9. **Daemon attribution** — every mid-edit sweep grandfathered with dated rationale (1493→1510 baselined, 0 new at STOP).

## b) PARTIALLY DONE

1. **Row 852 lint baseline regen** — `lint-baseline.sh --regen` RAN to completion after cache clean ("209 module/linter rows, 2430 findings") and the daemon swept the file, but the follow-up `--check` printed **FAIL** (unclear whether growth-gate or the warm-cache-after-regen class; AGENTS says regen only green after cache clean — mine cleaned, the check may need its own clean run). The baseline file is committed via daemon sweep; the --check green proof is MISSING. Re-run `golangci-lint cache clean && ./scripts/lint-baseline.sh --check` next session.
2. **T21 httpapi tail** — the three executable rows (205/586/587) landed; the cluster's remaining ~7 rows (nosniff/CSP completions etc.) untouched.
3. **CHANGELOG** — NO [Unreleased] entries were appended for this wave's operator-visible changes (`tq dlq --json`, evidence filter UX, dirty-tree lane, backingOff, verify-deaths, LIKE parity is internal, keeper internal). O14 requires these rows before the next release.

## c) NOT STARTED (cluster remainder)

- **T20 L3 features** — rows 1445 (browser-answerable questions pane), 1446 (structured closeout from messages API), 1447 (dashboard cancel endpoint), 1450 (E-gap routed-run smoke; also covers the never-created scripts/smoke/crush-server-route.sh from L1.8).
- **T22 nix/vendorHash automation** (~10 rows: drift detector, got→flake copy helper, drop-day watch).
- **T23 journal/facts deep rows** (~15; several BLOCKED on the M4 upstream ratification memo / owner filing GOs).
- **T24 remainder** (~9 rows: verdict-channel edges, excerpt/decodePayload refinements).
- **T25 remainder** (~5 rows: drift-audit EXECUTED parity row 462, fullcore fatal path row 534 — both now CHEAPER because the scratch-cluster recipe is a smoke).
- **T27 research/doc tail** (webui-live-contract updates, drafts triage; DOMAIN_LANGUAGE drift half landed via the dirty-tree row).
- **Final battery + closeout + push + CI watch** — the wave's mandated exit ritual (see §f1).

## d) TOTALLY FUCKED UP (worst first)

1. **The daemon ate a helper mid-edit** — my compose.go two-phase python write lost the ProjectionEnginePragmas function while keeping its call site; I then committed the BROKEN state (9484ac41) because `git add -A` ran before the module gate. Caught by composition vet, fixed in bec1afeb. Lesson applied after: run the owning module's gate BEFORE any `git add -A` commit of that module's files.
2. **Test-file surgery via multiedit mid-function** — the httpapi_test insertion split TestAuthLockoutIsPerClient and left an orphaned tail (compile error, two repair rounds). Should have appended a standalone test from the start.
3. **Postgres smoke port collision** — the first smoke run bound the default 55432 while my own earlier manual scratch cluster still held it; I then tried to kill pid 2145 which turned out to be the HOST's postgres fleet (Operation not permitted — correctly refused). Fixed with a free-port picker; the stray manual cluster was stopped via its own pg_ctl.
4. **Baseline --check left un-green** — shipped the regen without the post-regen --check receipt (see b1).

## e) WHAT TO IMPROVE

1. Per-module gate before EVERY commit touching that module (the d1 class).
2. The keeper lacks a live E2E proof (compile + suite green only); the routed-run smoke (row 1450) would prove keeper+supervisor under a real idle gap.
3. Smoke scripts should dump their log tails INTO the FAIL branch from day one (I shipped the "see $TMP/pg.log" pointer whose trap deleted the log).

## f) NEXT (ranked, ≤50)

1. Run `golangci-lint cache clean && ./scripts/lint-baseline.sh --check` → green receipt (row 852 close-out).
2. Append CHANGELOG [Unreleased] rows for the operator-visible surfaces (O14: tq dlq --json + evidence UX; dirty-tree lane; backingOff; verify-deaths; api lockout banner; keeper under Internal).
3. Full final battery: `./scripts/ci-local.sh` + `./scripts/root-gate.sh`; per-module GOWORK=off loop; fix what's red.
4. Push master; watch CI (`gh run watch <id> --exit-status`); confirm green.
5. Tick row 852 with the --check receipt; index this report (top chronological cluster).
6. T20: row 1447 dashboard cancel endpoint (CSRF+lockout+token rails per AGENTS serve security).
7. T20: row 1445 questions pane (rides the L2 feed; attach-order pin first).
8. T20: row 1446 structured closeout from the messages endpoint.
9. Row 1450: scripts/smoke/crush-server-route.sh (health + routed run + hooks deny + keeper idle-gap proof; also closes L1.8).
10. T22: vendorHash drift detector (nix build .#checks…vendor-hash, compare, print got→flake recipe).
11. T22: drop-day watch doc for formatterWithGo/goTarball.
12. T23: non-BLOCKED journal rows (watermark edge classes, fact-shape pins).
13. T24 remainder: verdict-channel edge rows.
14. T25: row 462 postgres drift-audit EXECUTED (ride the new smoke's cluster).
15. T25: row 534 fullcore fatal path live run.
16. T27: webui-live-contract doc updates; drafts triage.
17. Evaluate a v0.4.3 tag wave (the executor/webui/composition additions incl. KnownEvidenceTerms want publishing; cmd/tq's warn + parked requeue_class ride the same wave — the row-1348 class is why they were composed-from-published until now).
18. Re-run the production dirty-tree measurement when a population exists (1278 note).
19. Row 446 after the autopsy-outcome-surface owner ruling.
20. Owner ruling bundle #2 (the 146 BLOCKED rows) still the plan's #1 lever.

## g) QUESTIONS (cannot figure out myself)

1. **Push authority**: 44 commits sit unpushed (the STOP arrived before the battery+push step). Confirm the intended exit: push after battery green, or hold for review?
2. **lint-baseline --check FAIL after a cleaned regen**: if it persists after a fresh `cache clean`, is a small growth acceptable under the growth-gate policy, or should the regen land with the growth triaged row-by-row?
3. **v0.4.3 tag wave timing**: several surfaces are gated on publishing executor/composition (KnownEvidenceTerms consumers; cmd/tq parked requeue_class); cut the wave this weekend or batch with T20?

— reported 2026-10-31 13:13 CET; scratch clusters/dbs under /tmp (stopped/cleaned); production journal only read (measurement row); no pushes this wave.
