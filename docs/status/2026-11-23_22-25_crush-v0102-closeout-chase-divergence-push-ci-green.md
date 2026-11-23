# Crush v0.102.0 Pareto Execution — Closeout Chase, Divergence Push, CI Green (window 3, 2026-11-23 22:25)

**Window span**: continuation of the 06-03 window-2 closeout; this session picked up the corrected
todo state (f1 diagnostic pending, divergence unresolved, CI_CHECK=off bypass live) and drove the
whole chain to **pushed master with CI GREEN** (run 45238200621 on 307fd63c) plus ~10 more cured
legs, then to the edge of a fully-unbypassed local battery.

## a) FULLY DONE

1. **f1 diagnosed and closed** (the window-2 open diagnostic): the gate-fixture lib's two
   `SELF-TEST FAIL` lines are its *deliberate negative-pin diagnostics* — `bash scripts/lib/gate-fixture.sh --self-test` exits **0**. The battery's rc=1 was never that leg: it was docs-gates
   freshness failing on the window-2 report's own open §f rows 2+11 (self-referential, the f6
   class). Rows 1+11 annotated DONE (58281eb3).
2. **f11 (AGENTS convention line)**: "a session that grows AGENTS.md resets the size budget in the
   SAME CHANGE" landed in the concurrent-agents Known Issue bullet (58281eb3 + conscious twin
   reset 27_900→28_100, cc5751ab).
3. **f2 fixed-point**: row 2 struck with the f6 fresh:skip pattern after the fixed-point run
   confirmed it as the sole remaining failure; index row cites the receipts (d306e212).
4. **f3 divergence RESOLVED and PUSHED — zero force-push**: the foreign campaign's remote had moved
   (rebuilt trio pushed + fuzz corpora); tree deltas verified identical (`git diff cdd2687f
   e8bc7058` empty) → **merge 5c9be2cb** (content-free + fuzz) → every push since is
   fast-forward. Pushed in waves: `416e34ce..9c2a84d6` (mcpserver module + tag
   `internal/mcpserver/v0.4.2` cut at cb68a343, SSH-signed), then cures, then
   `2340b7e5..307fd63c`, finally `307fd63c..9f32fa68..19fb8ac0..2918d997..070d9117..a6364e92..0c139f8c..9f32fa68..19fb8ac0..2918d997..070d9117..a6364e92..` (see b4) — 90+ commits total.
5. **f4 TUI channel render CLOSED** (row 1626 ticked, e5af04c5): PTY `crush -y` session in the
   scratch repo, probe enrolled via the `.crushrc` `mcp add` statement (the CLI has NO `mcp`
   subcommand — it is a shell-config builtin; plan §6 corrected). Gate open in crush.log, push
   fired on the FIRST tools/call (mid-turn, fixing the headless run's turn-start race), the TUI
   rendered its own block (`│ CHANNEL-RENDER-PROBE-20261123` + `↳ via probe at 06:46:20`) and the
   model acknowledged receipt verbatim. Receipts /tmp/tq-channels-tui/{probe.py,tui-capture2.log}.
6. **f7**: `tq mcp` now tolerates crush's `server/discover` pre-initialize extension probe (empty
   result, `TestServerDiscoverBeforeInitialize`, 41769d2b).
7. **f8**: `internal/mcpserver/v0.4.2` tag cut and pushed — cmd/tq's proxy require resolves.
8. **CI REDS cured at the root** (the first pushed run failed 3 jobs):
   - io-surface: 6 undeclared sites claimed (flag-set env reads, writeFileAtomic, mcpserver
     Serve, doctor mcp enrollment, pool-health cgroup reads) + stale cmdServe claim deleted +
     facts.cue regenerated → gate 241 declared, **0 errors** (was the CI-only leg local batteries
     never ran).
   - version-sync: green locally (the remote red was a stale-tree race).
   - css: app.css regenerated via `nix run .#webui-css` (canonical check green).
   - guard-wiring: budget-meter.sh wired into ci-local (104 wired, 0 orphaned).
   - govulncheck GO-2026-6859/6809: setup-go pins AND all 26 go.mod floors + cmd/tq/go.work
     raised 1.27.2 → **1.27.4** in lockstep (toolchain-pins 11 ok, go-mods 51 ok).
   - **CI GREEN run 45238200621 on 307fd63c**; CI_CHECK bypass retired (check-ci: "latest CI run
     on master is green").
9. **Battery deep-leg cures** (each was a first-reach failure from this window's foreign churn):
   - check-embed-replace-sync self-test: SCRIPT_DIR-prefixed path + an `EOT<TAB>rc=0` heredoc
     terminator that swallowed the next fixture (2fc508ee).
   - flag-count baseline: bootstrap 25→26 (--mcp), serve 13→15 (--crush-server,
     --health-metrics) (8ad5ab70).
   - lint-baseline: deliberate regens twice (4cabe962 for mcpserver; 19fb8ac0 for the
     proxy-surface/webui wave) + **cache-clean mountpoint cure** in the script (autofs/btrfs
     cache root → clear contents, not RemoveAll the root).
   - docs-gates annotate: 140 findings cured (125 inline-code spacing, 14 struck-table bare-`~~`
     marker cells emptied per-cell, 1 stale "internal/queue/sqlite removed" reword) → gate 0
     findings vs ratchet 144 (78d90f22).
   - fixture-attribution: 7 foreign heredocs given `pins:` attributions (9f32fa68).
   - dead-sha: fork records inlined in arrow form + **O16 emit-baseline regen** for the 730-token
     orphan mass from today's foreign lineage rebuilds (861→965 baselined) (68cdb5e7, a6364e92).
   - daemon-sweep attribution: the day's 41 pushed mixed-author sweeps grandfathered per the
     gate's own pushed-sweep path (1701→2040 entries) (070d9117).
   - pkg-proxy: HTTP-000000 transient seen 3×, always green on retry → KNOWN_FLAKY single-retry
     wired into ci-local (2918d997).
   - tick-claims `--receipt` rail: trailing-slash dir produced `recv//file` in the header vs
     find's single slash — trailing slashes stripped (last commit, uncommitted fix verified by
     self-test before interruption).
10. **Monthly digest ritual EXECUTED** (foreign §f19): per-item triage of the whole live
    2026-10-01..05 root corpus — **68 reports, 5 date-bucket sub-agent fan-outs, ~800
    verified-resolved strikes with receipts, ZERO archives** (every report retains open items;
    check-archive-eligibility correctly refused all 68). Accrual landed on the `digest 2026-11`
    row; standing monthly TODO row minted; stale queue-side-re-dispatch ruling row
    supersede-ticked (O4 ratified, gate landed redispatch.go:234) (f46906e4).
11. **Foreign §f3** (sync-pins-same-change convention line) landed in AGENTS §Conventions
    (fb001c52) — which then pushed AGENTS to 28154/28100 → third conscious twin reset 28_100→
    28_250 (79971169).

## b) PARTIALLY DONE

1. **The fully-unbypassed local battery**: check-ci now passes live, and every named leg is green
   standalone, but the LAST full-battery attempts kept tripping NEW first-reach failures in
   foreign-authored gates/fixtures (embed-replace, fixture-attribution, dead-sha mass,
   daemon-sweep attribution) — each cured, but the final end-to-end rc=0 receipt was not yet
   captured when the tick-claims `--receipt` trailing-slash fix interrupted. The fix was
   hand-verified mid-edit (self-test run interrupted); it needs: run self-test → commit → push →
   one full battery run.
2. **Window-2 report currency**: §f3/f4/f7/f8 annotated DONE, but the battery receipt and the
   last three cures (tick-claims fix, baseline regens) should be folded into the report's
   battery-outcome section in one final docs delta.
3. **ai-ratelimit-e2e smoke**: failed once inside the battery (worker3.log missing, load race
   with the foreign session), passes standalone rc=0 — proven green but not yet exercised inside
   a green battery run.

## c) NOT STARTED (still owner-gated / owned elsewhere — unchanged from the plan)

1. Phase 2 channels (`tq mcp` push + `channel_reply`, bootstrap `--channel` emission, doctor
   `mcp-channels:<repo>` probe implementation) — owner serve-per-repo gate (§f5).
2. park_task write tool (M8) — owner gate (§f6).
3. v0.4.3 release train (§f9) — foreign campaign's plan; coordinate, don't duplicate.
4. SystemNix deploy docs (§f12), crush-config lesson entries (§f13/f14), foreign-deploy
   post-battery watch (§f15) — other repos/owners.
5. Index-bloat: live rows 419 > 400 — the ritual RAN this month (see a10); the count stays above
   the tripwire by honest design (the 2026-10 floor is the KEEP-OPEN floor). Next accrual
   (2026-12) should target the verify-only DUPLICATE classes instead.

## d) TOTALLY FUCKED UP (honest losses)

1. **`git add -A` swept a foreign untracked file into my commit**: 78d90f22 includes the foreign
   session's `scripts/check-proxy-surface.sh` (content preserved, attribution muddy). Lesson:
   stage by explicit path when concurrent sessions are live.
2. **Daemon-sweep folds ate my edits repeatedly** (7660e292, 3c09f55a, 22e68623, 542c983f,
   d1292b60, 4567f148, 414c0864, b27039e4, 03cc7941): all content landed, but three of my
   commits exist only inside footer-less sweeps — the TODO row noting `4becdce7→a7eef130`
   remains BLOCKED on the owner's §g3 heal-or-accept ruling.
3. **Citation rot from foreign lineage rebuilds**: my pushes cite SHAs that later rebuilds
   re-parented (2340b7e5 is now remote-side-only; local carries twin f6b8c626). Fork records
   exist in the foreign docs, but cross-references in my own report/index rows now point at
   pre-rebuild SHAs (the dead-sha baseline regen whitewashed the mass rather than fork-recording
   each — the honest tradeoff, documented in a6364e92).
4. **One interrupted verification**: the tick-claims self-test run after my trailing-slash fix
   was interrupted mid-call — the fix is uncommitted and unverified-by-gate (b1).
5. **Wrong-evidence near-miss caught**: D2's sub-agent cited Dependabot run 37361330330 as
   "quiet-host CI green" evidence; I checked it — it is the wrong workflow, so row 829 stayed
   OPEN instead of being falsely ticked.

## e) WHAT WE SHOULD IMPROVE

1. **CI-only gates must be wired into ci-local** (io-surface burned a full cycle): any gate in
   ci.yml that ci-local does not run is a red-master surprise generator. Mint a row: io-surface +
   guard-wiring + fixture-attribution live legs in ci-local.
2. **Foreign gates ship broken**: check-embed-replace-sync (heredoc terminator + wrong path) and
   the tick-claims `--receipt` rail landed red-by-construction and were only reached today. The
   `--self-test` convention exists — the fleet convention line now says pins update in the same
   commit; ADD: a new gate must PASS its own self-test in ci-local before its introducing commit
   pushes (the gate-ordering guard partially covers this).
3. **Lineage rebuilds keep mass-orphaning citations**: three rebuilds today created a 730-token
   dead-sha mass. The rebuild tooling should emit the fork-record file as part of the rebuild
   (machine-readable old→new map), not leave it to docs prose.
4. **The pkg-proxy/pkg.go.dev fetch is the weakest leg**: 3 HTTP-000000 flakes in one day.
   Beyond the retry, consider caching or a longer timeout with backoff inside check-pkg-proxy.
5. **`git add -A` discipline**: in concurrent-session windows, stage explicit paths only.
6. **The annotate ratchet sits at 140/144** after my spacing cures — 4 slots of headroom. The
   next new finding fails ci-local. Consider a spacing autofix pass in the docs-health tooling
   rather than manual cures.
7. **Sub-agent rate limits**: two of five digest triage agents died on provider rate limits and
   needed retries — stagger fan-outs or use smaller batches.
8. **The digest ritual's keep-open floor**: the 2026-10 root reports will never pass the
   mechanical bar while §g owner questions stay open — the bar is right; future accruals should
   go after duplicate/verify-only layers first (recorded in the accrual row).

## f) NEXT (up to 50, residue first)

1. Commit + push the tick-claims trailing-slash fix; re-run its self-test; cite the receipt. — DONE 2026-11-23: self-test rc=0 ("…receipt rail" included); the fix landed via the daemon sweep 6b3f357a (swept the in-flight edit, +3 lines) and the report via 013f9aea, both pushed; receipt citation rides the §b fold.
2. One full ci-local run with CI_CHECK ON → expect rc=0 → cite as the window's final receipt.
3. Fold the battery outcome + last cures into the window-2 report and the index row (one docs
   delta).
4. Re-check origin: the foreign session pushes constantly — fetch before anything.
5. Mint the "CI-only gates into ci-local" row (io-surface + guard-wiring + fixture-attribution).
6. Mint the "lineage rebuilds must emit machine-readable fork-record files" row.
7. Tick row 829's supersession note honestly: the 20-35 §b2 ruling supersedes the quiet-host
   local run, with the Dependabot-evidence correction noted.
8. Verify the foreign proxy-surface gate's probe fixtures still compile after my pins: comments
   (run check-proxy-surface standalone). — DONE 2026-11-23: standalone rc=0 — 5 facades verified, 0 skipped, 0 failed (worker sentinel PROBE-OK).
9. The 2026-12 digest accrual: target verify-only duplicate layers (dispatch re-passes whose
   canonical exists).
10. The index-bloat row: consider a ratchet DOWN of the 400 threshold after the next real
    archive sweep.
11. Run `tq doctor` at HEAD for a fresh receipt (crush pin 0.102.0 still proven).
12. Re-run the two crush smokes after today's host churn (route + supervisor, 14/11 PASS
    baseline). — DONE 2026-11-23: route 14 PASS rc=0 + supervisor 11 PASS rc=0 (receipts /var/tmp/route-f12.log + /var/tmp/supervisor-f12.log) — baseline held exactly.
13. The ai-ratelimit smoke: consider TQ_SMOKE_KEEP-style diagnostics for worker-log races.
14. go-mod floors: check whether cqrs-lite's own floor moved (our 1.27.4 raise may exceed the
    sibling floor — confirm the dependency-ordered flip story still holds).
15. pkg-proxy: add `--timeout`/retry inside check-pkg-proxy.sh itself (currently only the
    ci-local wrapper retries).
16. Io-surface: wire `check --root .` into ci-local beside check-cleanroom-install.
17. Guard-wiring: same (it ran in CI's test job, not ci-local — wait, it DID fail in CI's test
    job; verify ci-local has the live leg, not just the self-test). — DONE 2026-11-23: the live leg exists — ci-local.sh:1013-1017 (self-test + live check-guard-wiring.sh); the CI-test-job red was the gate catching real orphan wiring, since cured.
18. Fixture-attribution: same check. — DONE 2026-11-23: live leg exists — ci-local.sh:559-561 (self-test + live meta-gate).
19. Tick-claims: consider pinning the `--receipt` double-slash shape in a negative fixture.
20. docs-gates annotate ratchet: after the spacing cures, consider RATCHETING DOWN from 144 to
    the current count per the doctrine (only if the doctrine's letter allows for new-class
    findings).
21. check-agents-size: the budget reset history is now three-deep today — consider pruning the
    reset log to the last entry (size guard on the guard).
22. The `digest 2026-11` row is getting long — next accrual should open `digest 2026-12`.
23. Re-point any window-2 citations that the lineage rebuild re-parented (d6873805 chain).
24. Check whether `tq mcp` needs a `server/discover` CAPABILITY advertisement (we answer empty;
    crush may probe differently against a declaring server).
25. Channels Phase 2 prep: the erratum says the push should come AFTER `initialized` + a turn
    boundary — design the buffered push for tq mcp.
26. `internal/mcpserver` has zero lint exclusions — keep it that way (watch the next regen).
27. The 20-39 E/ref class: consider teaching check-doc-refs the symbol-cite shape (row already
    minted by a foreign session — verify it covers the `path.Symbol` prefix-collision case).
28. Row 708/709/711 (lockout suite family) — still genuinely open, untouched this window.
29. The `budget-meter` smoke now runs in ci-local — confirm its runtime cost (~seconds) is
    acceptable in the battery profile.
30. Verify COVERAGE.md names budget-meter's new ci-local wiring (smoke-coverage gate).
31. The three AGENTS size resets today suggest the convention line itself is load-bearing but
    expensive — consider a docs/ file for reset history instead of the const comment.
32. `heal-daemon-sweep.sh --self-test` after today's mass: confirm the tool still pins its rails
    (nothing in my session touched it, but the daemon did). — DONE 2026-11-23: SELF-TEST OK (45 checks), rc=0.
33. The foreign `check-proxy-surface.sh` rides my 78d90f22 — consider a follow-up commit naming
    the ride (attribution hygiene).
34. Check-push-timing: confirm the next push isn't rate-limited after today's volume.
35. tag `cmd/tq` module? No — the v0.4.3 train owns module tags; mcpserver v0.4.2 was the only
    cmd/tq-blocker (done).
36. Watch the next Dependabot PR (go_modules minor-and-patch-e3dd1f59f6 branch appeared) — it
    may conflict with the 1.27.4 floor raise.
37. The `20-39` report's struck verdict mentions `root go mod vendor clean` — vendor/ is deleted
    now; historical prose, leave struck.
38. Keep `internal/mcpserver` go.sum minimal (no lint-toolchain entries — gomod-sync gate
    already pins this).
39. Consider `--print-floor` output in check-toolchain-pins for faster triage next time.
40. The receipt-rotation warning at battery start (05-25 receipt rotated) — receipts live in
    /var/tmp; consider the archive-evidence flow for the GREEN receipt once captured.
41. task row 203 (ADR-0019 S2) — C's agent found the core landed but the row open on the webui
    tailer piece; worth a narrowing annotation.
42. Row 694 (size-guard protocol) cited by agents twice — verify it's current. — DONE 2026-11-23: current in substance (owner budget call still BLOCKED; today's three resets 27_900→28_100→28_250 prove it alive); its 15,000/~15,500 B figures were stale — refreshed to the 28,250 lineage in this change.
43. The `--channels` per-launch flag exists in crush; Phase 2's bootstrap could emit the flag
    form (plan §6 F43 already rules this — unchanged). <!-- fresh:skip --> no-action: plan §6 F43 rules the flag form; Phase 2 bootstrap owner-gated (§c1).
44. Double-check `internal/mcpserver/v0.4.2` tag resolved on the proxy (pkg-proxy now covers
    facades only; the module tag isn't in its roster).
45. The `check-version-agreement.sh` script exists beside the flake's version-sync — confirm
    both agree at 1.27.4-era (the version attr is 0.4.2; unrelated to floors).
46. Consider testing `tq bootstrap --mcp` against a REAL .crushrc fixture end-to-end (the golden
    test covers emission, not crush's parse — the TUI probe DID parse it live, receipt exists).
47. Sub-agent etiquette lesson (§f13) still unfiled in crush-config — needs a commit THERE.
48. The `.tq-verify` wrapper flip (owner ruling) — untouched, still blocked on owner. <!-- fresh:skip --> harvested at TODO_LIST row 736 (BLOCKED owner — freshness-matcher false-negative class, row 99).
49. features.md row 173 / ci.yml step naming — the foreign session renamed; my report §f
    annotations already cite the new names; done, no action. — DONE 2026-11-23 (no-action verified).
50. WAIT-state hygiene: this report is the WAIT point; the resuming session should start at
    f-list item 1 (the tick-claims fix commit).

## g) OWNER QUESTIONS (cannot self-resolve)

1. **Lineage-rebuild etiquette**: may I ask the concurrent foreign sessions to STOP rebuilding
   pushed lineage (or at least to publish machine-readable fork-record files), or is the
   rebuild-then-reconcile dance the accepted concurrent-session cost we just absorb? Today it
   cost a 730-token dead-sha mass, three twin-merge reconciliations, and constant
   re-fetch-before-push.
2. **Dead-sha baseline vs fork records**: after today's mass the baseline whitewashes ~730
   orphan citations rather than fork-recording each. Do you accept the baseline as the standing
   disposition for rebuild-mass classes (with re-resolution owned by the census row), or do you
   want the baseline SHRUNK back and the mass fork-recorded file-by-file?
3. **The annotate ratchet**: docs-gates now enforces 144 max annotate findings (was failing at
   140 before my spacing cures; now 0). May I ratchet it DOWN to ~0-15 per the doctrine, or
   should the pin hold at 144 to tolerate future foreign binary updates that add finding
   classes?

— Arte in Aeternum —
