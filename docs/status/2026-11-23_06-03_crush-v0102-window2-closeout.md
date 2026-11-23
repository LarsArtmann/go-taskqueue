# crush-v0.102.0 execution window-2 closeout (plan M6→M14, battery chase)

**Date:** 2026-11-23 04:00–06:03 (continuation window-2 of the 02-57 + 05-19 reports; owner directive "GET SHIT DONE — the WHOLE TODO LIST").
**Plan:** docs/planning/2026-11-23_00-57_SUPERB-crush-v0102-upgrade-channels-unlock.md.
**Verified at HEAD (at write time):** 1991f721-lineage — see §b for the battery's final rc caveat.

## a) FULLY DONE (verified this window)

- **M3 host flip + receipts.** crush v0.102.0 LIVE on evo-x2 (`crush version v0.102.0`, /run/current-system = the 03:56 deploy's toplevel — a concurrent deploy; my two chains died on the sudo publickey trap, see §d2). F13 route smoke 14 PASS rc=0; F15 supervisor smoke 11 PASS rc=0 (429 fast-refuse, keeper restart, mid-run-kill no-burn, verdict bridge, operator cancel — all green on 0.102.0; /tmp/tq-smoke-route-f13.log, /tmp/tq-smoke-super-f15.log). F16/F17: doctorCrushProvenVersion → 0.102.0 (f3e5e6ff), doctor receipt `ok crush-version crush 0.102.0 (proven pin 0.102.0)` (/tmp/tq-doctor-f17.log).
- **M4 observe-first (F18/F19).** Severed-run capture /tmp/tq-f18/observe.out (shared smoke-lib recipe + stalling mock): client rc=1, combined output carries BOTH the verbose WARN `Events stream ended error="unexpected EOF"` AND `Lost connection to the crush server before the run completed.` — v0.102.0 fail-on-drop confirmed LIVE; the crushdeath pins hold UNCHANGED (containsFold matches the display capitalization); the cancelled-run twin rode the supervisor smoke's operator-cancel leg (green). No test amendments needed.
- **M5 channels probe (F20–F23).** /tmp/tq-mcp-channels-probe/: the M9 `mcp add` rc grammar PARSES on the shipped binary; the GATE OPENS (`MCP channel enabled name=probe buffered=0`); the `notifications/claude/channel` push DELIVERED wire-clean; headless RENDER unconfirmed (no `<channel …>` wrapper — push raced turn start or TUI-only render). Erratum recorded: crush's client sends an extension `server/discover` (io.modelcontextprotocol/* _meta) BEFORE initialize that a Phase-2 tq mcp must tolerate. TODO row 1626 annotated HALF-DONE with receipts.
- **M6/M7 tq mcp COMPLETE (8972b9ca + c8aa26c1 + tip-amend).** mcpserver wiring RECOVERED from the foreign v0.4.3 campaign's named stash; missing `internal/mcpserver/go.mod` re-scaffolded (tidy-synced; go.sum inclusion caught by tip-amend after the first commit shipped without it); `ProjectCounts` seam fixed to the true per-status shape (compile error caught pre-commit); dispatch/usage/help-smoke/CHANGELOG wired; cmd/tq gate rc=0; check-gomod-sync rc=0.
- **M9 bootstrap `--mcp` (2bd5b30c, F39–F41).** Managed-block emission in the verified rc grammar (repeatable `--args`); absolute `--db` (workspace-cwd + TQ_DB-stripped shells); golden + idempotence + opt-out tests; doctor `mcp:<repo>` WARN (warn-only). Bootstrap/doctor tests green; grammar LIVE-verified by the M5 probe run.
- **M10 + M13 (5ecaf8b8).** Plan §6 design notes (doctor channels-probe design; official `mcp add --channel/--channel-reply` flags ruled for Phase 2) + DOMAIN_LANGUAGE rows (Routed run server-side cancel; Keeper idle arm + probe-holds-nothing; headless-never-respawns; NEW MCP tools surface row).
- **M12 (5be8bd5b).** Both crush smokes annotated with the v0.102.0 re-read (V2.10's rc-0 pin is the v0.101 lie that flips — executor M1 markers match both; IsBusy wideness watch-item); live legs green under F13/F15.
- **M14 ticks (e8c2f3db).** TODO rows 1623/1624/1625 ticked with SHAs + receipts; 1626 annotated HALF-DONE.
- **Window-2 docs (56eabdf6 + d0210cb4 + 64d72797).** Report + top-cluster index row + battery-outcome delta + f6 resolution.

## b) PARTIALLY DONE

- **The closeout ci-local battery: every summary GREEN, overall rc still 1 — final-leg diagnostic OPEN.** The chase cured, in order: check-ci divergence refusal (bypassed per the gate's own `CI_CHECK=off` — the red remote run sits on the foreign-rebuilt chain, not an ancestor); AGENTS size budget tripped by FOREIGN doctrine growth (ea496f69; conscious reset 27_600 → 27_900, 5fb19e3c, twin auto-derives, nothing pruned); erraudit's generated workspace vs the stripped root directive (root follows the re-lift, 609748b4); embed example directive (a77c9c63); the remaining 13 module directives (daemon sweep cb68a343 landed my `go mod edit` raise mid-gate — 51 checks ok, 0 failed); toolchain lockstep pins (daemon sweep 5e681e15 landed ci.yml ×7 + fuzz.yml + go.work → 1.27.2 mid-chain — 11 checks ok, 0 failed); docs-gates freshness (report delta d0210cb4 + f6 resolution 64d72797 — docs-gates now 9 PASS / 1 SKIP / 0 FAIL); budget-meter SC2034 dead `WORK` assignment (1991f721-era, removed this window). The LAST run's log ends with every visible summary ok (go-mods 51 ok, toolchain-pins 11 ok, docs-gates 9/1/0, lint-baseline self-tests all ok) yet rc=1: two suspect `SELF-TEST FAIL` lines at the gate-fixture leg ("wrong-rc pin (rc=3, want 1)", "fragment-miss pin") that contradict that leg's own `ok (7 checks)` summary — an unresolved contradiction needing one focused diagnostic pass (§f1).
- **Push NOT done.** master/origin DIVERGED (now ~24-25 local vs 3 remote): the foreign campaign rebuilt three pushed commits locally; a normal push is refused non-FF; force is guard-denied and foreign-work-destroying. All window work is committed locally.
- **Channels probe RENDER half** — row 1626 stays open for a TUI-observed run.

## c) NOT STARTED (owner-gated, unchanged)

- park_task write tool (M8); serve-per-repo gate; channels Phase-2 bootstrap emission + `tq mcp` push/channel_reply; doctor `mcp-channels:<repo>` probe implementation.

## d) TOTALLY FUCKED UP (honest)

1. **The mcp wiring sat uncommitted through the previous report-first directive and got STASHED by the foreign release-prep session** — the exact daemon-race loss the stage+commit-first convention exists to prevent; recovery cost a full re-verification pass. The handoff's claim that new-module.sh scaffolded go.mod was FALSE (the foreign session's row note "internal/mcpserver still has no go.mod" was the truth) — my first re-land commit (c5c17bb9) shipped without go.sum and needed a tip-amend within minutes.
2. **Deploy warm-tool misdiagnosis ×2:** I warmed with `nix flake prefetch` (fetches the flake's OWN source only) when the correct input-warming tool is `nix flake archive`; the real blocker was root's missing GitHub key (sudo deploys cannot fetch git+ssh inputs unworn). Two background chains died on it before I understood.
3. **Mid-gate daemon-sweep races cost three ci-local rounds:** my go.mod/toolchain edits were folded into footer-less sweeps (cb68a343, 5e681e15) while the gate read a pre-sweep snapshot — each cost a full re-run. The commit-before-gate discipline cannot help when the EDITS THEMSELVES are what the daemon sweeps between edit and commit; the only cure is edit→immediately-stage→commit in one breath.
4. **The battery chase took four ci-local rounds** because each previously-masked leg surfaced one at a time (check-ci → size → erraudit → embed → go-mods → toolchain-pins → docs-gates → syntax). A dry inventory of ALL gate legs against the known foreign changes (the strip campaign) would have predicted the go-directive cascade in ONE pass.

## e) WHAT WE SHOULD IMPROVE (process, from §d)

1. **Foreign-change cascade inventory before batteries:** when a foreign campaign touches shared invariants (go directives, size budgets), walk EVERY gate leg that reads them BEFORE running the battery — the strip→re-lift cascade was fully predictable from the strip commit's own message.
2. **`nix flake archive` (not prefetch) is the deploy warm** — write it into SystemNix's deploy docs; root cannot fetch git+ssh inputs unworn.
3. **Budget resets for foreign growth:** the reset is the sanctioned path (gate message prescribes it), but the FOREIGN session that grows AGENTS.md should reset in the same change — worth a convention line in AGENTS.
4. **Report-first directives vs uncommitted code WIP:** carve an exception — commit WIP before writing any report when a foreign campaign is known-active.
5. **The stash etiquette worked** (named, self-described, README-pointed) — generalize into a lesson entry for concurrent sessions.

## f) NEXT — up to 50 (residue first; 1–3 are this window's own)

1. Diagnose the ci-local rc=1 with all-summaries-green: the gate-fixture lib self-test prints two `SELF-TEST FAIL` pins ("wrong-rc pin (rc=3, want 1)", "fragment-miss pin") yet summarizes `ok (7 checks)` — is the battery negative fixture mis-counting, or is rc=1 leaking from that leg despite the ok summary?
2. Re-run ci-local to green after f1 and cite the receipt in the index row.
3. Resolve the master/origin divergence (foreign rebuild of 3 pushed commits) — heal/rebuild-lineage vs coordinated force-with-lease; needs the foreign session's SHA map or owner ruling; then push the window's ~25 commits.
4. TUI-observed channel render (row 1626 remainder) — one PTY `crush` session with the probe enrolled; closes the probe row.
5. Phase 2 (owner-gated): `tq mcp` gains `notifications/claude/channel` push + `channel_reply`; bootstrap `--channel` emission behind the serve-per-repo gate; doctor `mcp-channels:<repo>` probe implementation per plan §6.
6. park_task write tool (M8) — owner gate.
7. mcpserver: tolerate/implement crush's `server/discover` extension (observed in the F21 probe).
8. Tag internal/mcpserver v0.4.2 in the release train — cmd/tq's unpushed-tag require 404s proxy tidy (observed: `go: downloading .../internal/mcpserver v0.4.2` in the cmd/tq tidy attempt) until the tag exists.
9. v0.4.3 release train: after the cqrs-htmx/go-cqrs-lite re-tag lands, flip sibling pins dependency-ordered and tag the trains (foreign campaign's stated plan — coordinate, don't duplicate).
10. Index-bloat digest ritual (415 rows > 400 threshold) — standing monthly advisory.
11. AGENTS convention line: foreign sessions that grow AGENTS.md reset the budget in the same change (from §e3).
12. SystemNix docs: deploy warm = `nix flake archive` (from §e2); sudo deploys cannot fetch private inputs unworn.
13. Lesson entry (crush-config repo, by commit): named-stash handoff etiquette for concurrent sessions (from §e5).
14. Lesson entry: mid-gate daemon sweeps of EDIT-in-flight files — the only cure is edit+stage+commit in one breath; consider a pre-commit hook advisory when a repo-scope gate ran within N minutes of the last edit.
15. Monitor the foreign deploy's post-battery: the 03:56 deploy's nh hung ~1h on media-atlas-backup and its post-deploy phases resumed after — confirm its exit record eventually landed in /var/log/systemnix-deploys/.
16–50. The standing fleet residue is unchanged and owned elsewhere (dead-sha census row, dedup-receipt backfill rows 1919/1920, DEDUP-CLAIM triage, stale-batch-header annotations — 147 advisory warns in the TODO gate); the tq-mcp BUILD rows (2026-09-29 agent-autonomy section) own M8+; no NEW rows were minted this window (plan rail 3: zero-new-rows held).

## g) OWNER QUESTIONS (cannot self-resolve)

1. **Divergence reconciliation:** master carries a foreign REBUILD of three pushed commits (24-25 local vs 3 remote). Reconcile via heal/rebuild-lineage (needs the foreign session's SHA map) or a coordinated force-with-lease? I did NOT push.
2. **Deploy attribution + post-battery:** the successful 03:56 SystemNix deploy was NOT my session's. Confirm it was intended, and that its post-deploy battery's delayed resume (after the media-atlas-backup wedge) ended clean — its log has no `deploy exited` line yet.
3. **ci-local rc=1 with all-summaries-green** (the gate-fixture self-test contradiction): do you want me to keep chasing it to green now, or is the battery-good-enough verdict (every named summary ok) acceptable until the next window?
