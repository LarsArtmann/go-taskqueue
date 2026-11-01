# Session status — SUPERB adoption execution: verdict/question bridges, death classification, RSS watch, stuck-turn detector, upstream issue filed; V2.10 leg in flight

**2026-11-01 18:14 CET · Owner STOP for status. Window 17:20→18:14, executing
docs/planning/2026-11-01_06-08_SUPERB-CRUSH-SERVER-ADOPTION.md §V1+§V2 waves
under the "GET SHIT DONE — the WHOLE list" directive. Parallel agents active
all window (daemon sweeps landed 5+ times mid-edit; nothing reverted).**

## a) Work commits (footer-less docs/daemon-fold convention; code rode daemon `chore:` sweeps)

- `b851444f` docs(agents): verdict channel row gains the --file rail (A1/V1.4) + size-guard trims
- daemon folds carry the code: verdict `--file` + rails + tests (cmd/tq, executor), crushdeath.go + worker case + queue class, agentpool RSS + poolhealth fold + tests, doctor proven-pin + test updates, ask `--file` + question rail + tests, smoke edits (route bridge leg, pool mid-run leg), COVERAGE rows, flake.nix routed eval branch, deploy/nixos crushServerSocket, planning-doc LIVE-STATUS + decision memo, CHANGELOG rows, TODO 1451 tick, upstream issue draft

## b) Done this window (verified)

1. **A1 verdict bridge SHIPPED**: `tq verdict --file` (flag > env fallback, table test); executor `verdictRailTmpl` baked into work+closeout prompts (always-on; local env channel byte-identical); 4 existing prompt-shape tests updated; route-smoke BRIDGE leg green ×1 — a routed bash tool runs the REAL `tq verdict --file`, file lands with ZERO env dependency.
2. **A3 upstream issue FILED**: all five verify-before-filing gates run (duplicate search clean; source re-verified FRESH at main `ef4993a3` — bash.go `executeBashCommand` → `bgManager.Start` (no env param), shell.go:95-97 `os.Environ()` fallback, agent.go:315 `Env: append(os.Environ(), ws.Env...)` quoted verbatim); drafted in docs/drafts/, voice checker 0 FAIL/0 WARN; **filed as charmbracelet/crush#4210, verified OPEN**; TODO row 1451 ticked with the full note.
3. **A8 crush-server death classification SHIPPED**: probe FIRST (mid-run kill = rc 0 + `WARN Events stream ended error="unexpected EOF"`; normal run = zero such lines); `crushdeath.go` (CrushServerDeathError typed, DetectCrushServerDeath two-sided); Execute wiring (err + rc-0 paths, routed-only); `queue.RequeueClassCrushServer`; worker requeues WITHOUT burn, env-streak-EXEMPT; `IsEnvironmentalDeath` marker (no autopsy); executor + worker pins green.
4. **Seam bug found+fixed by the new leg**: executor passed `-H <bare-path>` — crush rejects scheme-less hosts ("Invalid host URL"); `crushHostFlag` normalizes to `unix://`, pinned 3-way in TestAgentCrushServerSeam. A real production-class bug only the E2E leg could catch.
5. **A9 RSS watch SHIPPED**: actor Start/Wait + atomic server pid; keeper samples `/proc/<pid>/status` VmRSS per dial (stderr line + `<journal-dir>/crush-server-rss.log`); `tq pool-health` folds to `crushServerRss` (samples/last/max; WARN > 2 GiB); 3 unit pins; LIVE-VERIFIED in smoke output (`crush server rss=59028 kB`).
6. **A10 stuck-turn detector SHIPPED**: webui `stallTracker` (fake-clock pinned), touch on every SSE data frame, 10-min bar, busy-session gated (quiet-idle never warns), 30-min re-warn spacing; full webui suite green.
7. **A13 doctor proven-pin SHIPPED**: `doctorCrushProvenVersion` 0.99.0; mismatch WARN names the re-run ritual (both smokes) + the pin-bump rule; ensureServer mismatch-replace documented as the upgrade path (V2.17); 3 doctor tests updated, cmd/tq suite green.
8. **A4 question bridge SHIPPED**: decision memo in the planning doc (file-path rail OVER SSE-park — one bridge shape, no new failure domain, AnswerPoller unchanged); `tq ask --file <path>` (flag > env); `questionRailTmpl` on work+closeout prompts gated `!secondOpinion`; second-opinion-clone absence pinned.
9. **V1.7 upstream half SHIPPED**: `deploy/nixos/tq-agent-pool.nix` gains `crushServerSocket` (default null; wires BOTH units); flake module-eval routed branch pins exact ExecStarts + default flag-free rollback; `nix build .#checks.x86_64-linux.module-eval` green.
10. **Docs**: planning doc LIVE-STATUS + question-bridge memo (V1.11/V1.14); CHANGELOG 7 operator-visible rows (V3.5); route-smoke COVERAGE bridge row; AGENTS verdict-rail row (V1.4).

## c) Tests / receipts (this window, on the touched code)

- internal/executor full suite green ×2 (rail, seam 3-way, crushdeath classify, question-rail gate)
- internal/worker full green (TestCrushServerDeathRequeuesWithoutAttemptBurn: pending/0-attempts → completes same budget)
- cmd/tq full green ×2 (verdict table, ask flag, doctor pin, RSS fold, size guard)
- internal/webui full green (stall tracker fake-clock pin)
- route smoke green ×1 with the new bridge leg (8 legs total)
- pool smoke: 429 + keeper/supervisor legs green every run; mid-run kill leg iterated (see d)
- `check-script-syntax.sh` green after each smoke edit; module-eval nix check green

## d) Totally fucked up / mistakes (honest)

1. **Partial-edit blindness (3×)**: multiedit reported "3 of 4 applied" on the pool-smoke mock handler and I did NOT verify the heredoc — cost TWO full smoke runs (~6 min each) before I found the tool-scripting branch had never landed. Same class hit the doctor const (caught by build) and one test edit. Rule I broke: grep the file for the new content whenever the tool says partial.
2. **Shell quoting**: `$((` adjacency (SC1102) + a `tq --db X tasks` flag-order bug (usage dump instead of task list) in the new leg — each cost a run. My /tmp debug script had the SAME flag-order bug I then copied into the smoke.
3. **Kill-timing design flaw**: killed at claim-detection; the client's ensureServer respawned the server and the run completed normally (no severed stream). Fix: gate the kill on the mock's stall-entry marker.
4. **Mock content-key truncation**: my tool-branch key read only the first 80 chars of the last user message — the harvested prompt says "verdict" ~600 chars in, so the mock answered inertly and the task completed in 300 ms (attempts 0). Diagnosed via the completed fact's session detail (17k prompt tokens, 1 completion token). Fixed to full-content key.
5. AGENTS size guard blown by 174 B — two extra trim rounds; should have checked the budget before writing.
6. Two inverted/wrong test assertions on first write (Detect dial-unix; worker ran≥2) — fixed in place.

## e) What to improve

- After ANY partial multiedit, verify by grep before running the consumer.
- Check AGENTS.md byte budget BEFORE adding rows (wc -c is cheap).
- Timing-dependent kills must gate on an observable in-window marker, never on task-state polls.
- `tq --db X <subcommand>` order class: consider a bash-guard advisory or a tq "flags go after the subcommand" hint.
- The route smoke's mock and the pool smoke's mock now differ (content-key windows); a future consolidation would prevent the truncation class from recurring.

## f) Next up to 50

1. Land the V2.10 leg (run 8 in flight at STOP: full-content mock key + stall-marker-gated kill)
2. COVERAGE.md row for the pool mid-run kill leg
3. README "Server mode" section (V3.1)
4. FEATURES rows: L1 bridge/status-truthful L2 (V3.2)
5. AGENTS rows: env-gap caveat + R4 `-H`-alone trap (V3.3; size-guarded — needs trims)
6. DOMAIN_LANGUAGE: routed run, thin client, workspace, keeper, supervisor ladder, question-channel ruling (V3.4)
7. L4 GO/NO-GO sheet draft with data slots (V3.7)
8. trash /tmp probe stacks (tq-crush-death-probe.*, tq-crush-normal.*, tq-midrun-debug.*, pool-smoke-run*.log) + AGENTS probe-trap pointer (V3.10)
9. `go mod vendor` + check-gomod-vendor-sync
10. Full quiet-tree ci-local battery, rc to file (V3.8)
11. lint-baseline --check (new code growth gate) + erraudit per module root
12. Root build+vet+test -race + test-cmd-tq.sh (windows cross-compile)
13. Facade parity check before staging (check-facade-parity.sh)
14. Push-state audit + push (V3.9 — needs owner §g, see questions)
15. SystemNix: float the go-taskqueue lock to pushed master, set `crushServerSocket`, update tests/test-tq-agent-pool.nix serve-ExecStart suffix pin + add routed case, docs/services/tq.md deploy recipe row (V1.7/V1.8 completion)
16. Owner-run sudo deploy = the flip (V1.9)
17. Watch one routed tick wave; LIVE-STATUS wave receipts (V1.10)
18. Enricher real-closeout verification on that wave (V2.19/A15)
19. Routed-question pane leg — SSE prompt frames in QuestionsPane, M6 ordering (V2.14/A11)
20. Cancel-vs-server leg — dashboard cancel on a routed running turn (V2.15/A12)
21. Dogfood-once routed variant ruling — the V2.10 leg may already BE the pool-level routed E2E; confirm scope with owner (A14/V2.18)
22. canary-flip ritual cross-ref: smoke header ↔ issue #4210 ↔ TODO row (V3.6/A17 completion)
23. Question-bridge smoke leg (`tq ask --file` routed round-trip — canary covers verdict only today)
24. Tick-audit: grep verbatim batch anchors, unchecked-landed = paid round-trip (V3.11)
25. check-todo-list / check-release-docs / check-status-index / check-doc-refs gates
26. daemon-sweep-baseline grandfathers for new foreign folds if the battery trips attribution
27. doctor proven-pin bump ritual note in both smoke headers (discoverable pin)
28. crush#4210 follow-up: watch for triage; canary flips red automatically when fixed
29. row 1448 evidence: first dogfood-week RSS samples after the flip
30. L4 fleet GO/NO-GO sheet OWNER ruling after ≥1 week telemetry (A18)

## g) Questions

1. **Push GO?** V3.9 wants this batch pushed to origin master. The standing constraint from the 06:03 session was "don't push past a report without owner §g" — this is that report: with one word I push after the battery (step 10-14) goes green.
2. **SystemNix flip timing**: should the house module carry `crushServerSocket = "/mnt/pool/services/tq/crush-pool.sock"` NOW (the flip rides your next `nix run .#deploy`) or stay unset until you've reviewed one manual routed wave?
3. **V2.18 scope**: is the pool-smoke mid-run leg (real pool, real routed agent turn, verdict file, kill-no-burn) sufficient as the pool-level routed E2E, or do you also want the full happy-path leg (enqueue → routed run → verdict → done)?
