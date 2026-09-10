# Session Report — README overhaul from verified facts, TQ_DB production incident, live pool PATH evidence

- **Written**: 2026-09-10 05:29 CEST
- **Session window**: 2026-09-10 ~04:55–05:30 CEST, one session, docs-only deliverable plus one self-inflicted incident and one live-infrastructure discovery.
- **Scope note**: per the task prompt, this report covers ONLY this session's run and what it directly noticed — no repo-wide re-research. Claims marked "verified" were re-checked against live code, live CLI runs, or the live journal during this session.
- **Method**: read README/FEATURES/CHANGELOG-head/DOMAIN_LANGUAGE/cmd-tq source before writing; built `tq` and ran every documented quickstart command verbatim in an isolated DB; fetched the live dashboard over HTTP; grep-verified every flag cited; ran `check-doc-refs.sh`, `check-features-roadmap.sh`, `go build/vet`, the full root race suite (green, 7.9–16.1s per package), and manual markdown structure checks (fence parity, table shape, TOC anchors).

## TL;DR

| Area | Verdict |
| --- | --- |
| README rewrite | Done: +215/−180, every claim source- or runtime-verified, all doc gates green |
| `--model` contradiction | Fixed: pool-level model demoted to documented escape hatch, bootstrap `.crushrc` presented as the only effort-carrying mechanism |
| Duplicate systemd sections | Merged into one "Running unattended" section |
| Documentation coverage gap | Closed: 19-command map added; `--status-every`, `--prune-stale`, `--max-concurrent-agents`, journal browser, watermarks, `doctor`/`audit`/`top`/`tasks`/`tq api`/`facts --json` now documented |
| Self-inflicted incident | One smoke enqueue reached the PRODUCTION journal via inherited `$TQ_DB`; task completed harmlessly; trap recorded in AGENTS.md |
| **Live pool finding** | **The deployed production pool is requeue-looping EVERY agent task on `git` missing from PATH — live facts at 05:28 CEST. The fix has been on master since ~01:55; the SystemNix flip is still owner-blocked** |
| Master CI | Still red as of the latest run (02:00Z push, f24 stats commit) — pre-existing failures (test-windows, release-gates smoke), not from this session |

---

## a) FULLY DONE

1. **README.md rewritten from verified facts** (+215 insertions / −180 deletions, 365 → 399 lines):
   - **Fixed a harmful contradiction**: the old agent-pool example recommended `tq agent-pool --model …` while a later section explained payload-level models RESET reasoning effort (crush telemetry, 2026-09-08). The rewrite presents `tq bootstrap --model` (managed `.crushrc` block) as the only model+effort carrier and demotes the pool flag to a documented escape hatch.
   - **Merged the two duplicate systemd sections** ("Running it as a daemon" + "Running the pool as a service") into one "Running unattended" section; dropped the fragile module-cache `cp` hack in favor of the pool.conf + hardened-unit flow.
   - **New "Every command at a glance" table** (19 commands), closing a real coverage hole: `doctor`, `audit`, `top`, `tasks`, `watermarks`, `version`, `tq api`, `facts --json/--detail`, `dlq --rescue-all --older-than`, `worker --once` were previously undocumented on the sales page.
   - **Documented shipped-but-invisible features**: `--status-every N` done-prompt loop (the self-feeding TODO loop-back), `--prune-stale` (default-on zombie sweep), `--max-concurrent-agents`, the dashboard journal browser + keyboard shortcuts, Watermark as a concept, a TOC, a "facts first" pitch, and the `$TQ_DB`/`--db` override note.
   - **Trimmed internal noise**: golangci-baseline and per-module test-loop details moved out (AGENTS.md territory); kept the embedder-facing module map + store-picker sections.
2. **Every claim in the new README verified** (no doc written from memory):
   - Quickstart run verbatim in an isolated DB: enqueue (raw + JSON payload shapes) → `worker --once` drain → `stats` → `tasks` — all green.
   - `tq serve` started and fetched over HTTP: status cards, board link, filters, DLQ, fact feed, journal browser ("load older"), `/`-search and `1–4` hints all render as documented.
   - Flag existence + defaults verified in source: `--prune-stale` default `true` (agentpool.go), `bootstrap --agents` = "pool concurrency AND machine-wide agent cap" (bootstrap.go:73,199), `--status-every`/`--review-autofix`/`--log-dir-max-*`/`--max-concurrent-agents` all registered.
   - `go list -m -versions` confirms the module proxy has v0.1.0 + v0.2.0, so the documented `go install …@latest` resolves.
3. **Gates green at report time**: `scripts/check-doc-refs.sh` ok, `scripts/check-features-roadmap.sh` ok, `go build ./...` + `go vet ./...` ok, root-module race suite fully ok (cmd/tq 7.9s … internal/e2e 16.1s), markdown structure checks (even fences, no malformed table rows, all TOC anchors resolve) ok. Re-ran the doc gates + build + vet at 05:28 before writing this report — still green.
4. **AGENTS.md Known Issues gained the TQ_DB trap** (see d2): agent shells inherit `TQ_DB=/mnt/pool/services/tq/tq.db`, so bare `tq …` commands hit production even from scratch dirs; scratch smokes must set `TQ_DB`/`--db` explicitly.

## b) PARTIALLY DONE

1. **"Task → run → wait for the agent to be done" (the stated goal)**: the run side ships and is live-proven (enqueue → agent task → verify gate → completion fact → done-prompt report → TODO_LIST append). The **wait** side does not exist as a UX: there is no blocking `tq enqueue --wait` / `tq run` (verified against the full CLI usage surface). The dashboard and `tq tail -f` are the only observation surfaces today.
2. **The docs-health second loop ("every X tasks per project")**: `--status-every` mints the report task and the contract (`TQ_RESULT` + existing repo-relative report file + verify gate) is enforced mechanically — but the *quality* of the report (the docs-health skill execution) is prompt-contract only, unenforceable by tq by decided policy (TODO_LIST 20:56 g2/g3 decisions). Documented this session; not re-exercised live this session.
3. **Production pool health**: sh tasks complete (my accidental `demo` task ran to completion through the live pool), but agent tasks are 100% preflight-blocked right now (d1) — the loop is half-alive in production.
4. **Master CI verdict for this session's docs**: local gates are green, but the latest master run (02:00Z, pre-dating this session) is a failure from the two known non-docs breaks (test-windows, release-gates smoke). No CI run exists for the README HEAD yet.

## c) NOT STARTED (verified absent during this session)

1. **Blocking run UX** — nothing in the CLI waits for a specific task to reach a terminal state (`tq enqueue --wait`, `tq run`, or a `--follow` on show).
2. **Automated status-report archiving** — "reports whose every forward-looking item is resolved move to `archived/`" is a manual convention enforced by nobody; no strikethrough/annotation automation exists (the docs-health skill does it when an agent runs it, which is the loop above, not a guarantee).
3. **README-as-contract guard** — nothing fails CI when a README command/flag rots (the ghost-ref check covers repo paths only, not flags or subcommands).
4. **Env-safety guardrail** — no warning when `enqueue` runs with `$TQ_DB` pointing somewhere other than `./tasks.db` (the exact trap of d2).
5. **`--model` retirement decision** — the flag still exists and still resets reasoning effort when used; documenting the caveat (done) is the interim, not a fix.

## d) TOTALLY FUCKED UP

1. **The deployed production pool cannot run agents — measured live during this session.** Facts 159–165 (2026-09-10 05:28:06–05:28:20 CEST) show worker-3552 requeue-looping every claimed agent task with `preflight: agent: git status failed in /home/lars/projects/go-taskqueue: exec: "git": executable file not found in $PATH`; `tq tasks --type agent` shows pending agent tasks whose last error is the same preflight. Root cause is known and FIXED on master (01:55 session: bare repo names resolved via service cwd + systemd service PATH lacking git/go/crush, silent skip logging) — but the SystemNix input flip + redeploy is owner-run (sudo) and still pending (TODO_LIST "Fleet / deploy" BLOCKED item). The queue's safety rails work exactly as designed (preflight requeues burn zero attempts; the requeue backoff ladder paces the retries) — so nothing is lost, but the Flash pool has produced no agent work from the current journal generation. This is the "pool-deploy failure mode" from AGENTS.md, observed live and quantified: ~3.5h after the fix landed, the running deployment still predates it.
2. **My own incident: a smoke enqueue reached the PRODUCTION journal.** The session shell inherited `TQ_DB=/mnt/pool/services/tq/tq.db`; my README-verification enqueue (`demo`/`sh`/`echo hello…`, task `000001a089484eebba739ddfe8bb665299b5`, fact seq 132) bypassed the scratch dir entirely and was claimed by the live pool within seconds. Containment: the task was a harmless `echo` and completed cleanly (attempts 0); `tq cancel` correctly refused on terminal state, so the only residue is one extra completed task in an append-only journal. Recovery of the failure class: recorded as a Known Issue in AGENTS.md (c4 proposes the guardrail fix).
3. **Master CI is red and five sessions' worth of DONE verdicts have landed on red anyway** (04-09 report's finding, still true at the 02:00Z run). Not this session's breakage, but this session added commits to that branch with only local gates as evidence — the structural issue (no master-CI state gate; TODO_LIST item filed) remains open.

## e) WHAT WE SHOULD IMPROVE

1. **Make the deployment gap loud, not quiet.** The git-not-found requeue loop is invisible unless someone reads facts by hand. Two already-filed fixes cover it (`tq doctor` PATH warning 02:00-f18; dead-pool PapDashboard alert 02:00-f6) — both should jump the queue, because today the only monitoring is a human reading this report.
2. **Guard the environment, not just the docs.** A one-line stderr warning on `enqueue` when `$TQ_DB` is set (and where it points) would have prevented d2 entirely; cheaper and less annoying than refusing.
3. **Treat README commands as a contract.** The doc-rot guard should assert every documented flag exists in `cmd/tq` and every command in the map matches the dispatch table — the README is the install page; it rotted silently for weeks (19 commands, ~10 undocumented before today).
4. **Kill the model-reset trap at the source.** Documenting the caveat keeps the footgun loaded; either hard-fail `agent-pool --model` with remediation guidance (same UX as the yolo-without-crushrc refusal) or gate it behind an explicit `--i-know-this-resets-effort`-grade flag.
5. **Build the "wait" half of the goal.** Task → run exists; a blocking surface (`tq enqueue --wait [--timeout]` streaming the task's facts to the terminal) is the missing piece that makes the queue usable as a function call from scripts and other agents, not just as an unattended pool.

## f) WHAT WE SHOULD GET DONE NEXT (up to 50; ★ = new from this session, the rest are already filed in TODO_LIST — listed here re-prioritized by this session's evidence, BLOCKED ones marked)

**Do first — this session's evidence says so**

1. ★ Flip the SystemNix input to master + redeploy (owner-run, sudo) — the live pool is idle-broken (d1); everything needed is on master. — BLOCKED: owner
2. ★ `tq doctor`: warn when crush/git/go are missing from PATH (02:00-f18) — turns d1 from a journal-archaeology finding into a one-command diagnosis
3. ★ Dead-pool detection: PapDashboard alert when scan-fail/requeue streaks cover every repo for N ticks (02:00-f6) — d1 ran silently for hours
4. ★ `enqueue` warns when `$TQ_DB` is set and differs from `./tasks.db` (this session's d2 guardrail)
5. ★ Blocking run UX: `tq enqueue --wait [--timeout]` (and/or `tq run`) streaming a task's facts until terminal — completes the stated goal's missing half
6. ★ README contract guard: test that every flag/subcommand documented in README exists in `cmd/tq` (and vice-versa for the command map)
7. ★ Decide `agent-pool --model`'s fate: hard-fail with remediation vs keep as documented escape hatch (needs owner call; see g2)
8. ★ `tq pool-health` one-shot: per-repo skip streaks + last harvest activity from the journal (02:00-f27) — pairs with 2/3 for the same blind spot

**Red master CI (blocking trust in every DONE verdict)**
9. Fix test-windows red: `TestHarvestConfigFromOptionsExpandsBareRepoNames` subtests on windows-latest (filepath/abs-path assumptions in bare-repo-name expansion)
10. Fix release-gates smoke on runners: annotated `git tag -a` lacks `-c user.email/-c user.name` identity
11. Add the master-CI state gate (`scripts/check-ci.sh` via `gh run list`) to ci-local — stop landing DONE verdicts on red
12. Bump `golang.org/x/text` ≥v0.39.0 in `internal/queue/postgres` (GO-2026-5970, reachable per govulncheck) + sweep every module for the floor
13. Encode the gosec FP triage as an exclude-rule config so the advisory job goes green and new classes stand out
14. Flip govulncheck to a hard gate once #12 makes it green; same decision for gosec (#13) — BLOCKED: owner gate-vs-advisory call
15. Publish gosec/govulncheck findings as CI job-summary artifacts (stop log-diving)
16. ci-local: run release-gates smoke under `GIT_CONFIG_GLOBAL=/dev/null` so local matches runner failure modes
17. Changed-lines 120-col line-length gate in ci-local (the c5c654c signature-wrap class)
18. Add the two advisory scan jobs to SECURITY.md's defense-layers matrix
19. Confirm a CI run exists for current HEAD and record the outcome (04-09 f-item, still open)
20. Required-checks proposal (test-windows + release-gates first, then scan jobs) into docs/planning/

**Owner-blocked decisions (parked until answered; see g)**
21. Push the two backend tags `internal/queue/{sqlite,postgres}/v0.2.0` + rerun per-module proxy checks — BLOCKED: owner push authorization
22. Postgres CLI store wiring (`--store postgres://…`) — BLOCKED: owner release-timing call (v0.3?)
23. `internal/consumer`: wire into `tq serve` tailing or delete the ghost package — BLOCKED: owner intent
24. Unify or document the three near-identical fact-source interfaces — BLOCKED: owner architecture call
25. `task.AllStatuses` export vs twin-list permanence — BLOCKED: public-API/versioning call
26. Pool policy when master CI is red (refuse DONE verdicts vs disclose-only) — BLOCKED: owner completion-contract call
27. Dependabot/renovate policy for the 8-module tree — BLOCKED: owner policy
28. CQA bridge live-instance verification — BLOCKED: needs live CQA URL + owner ID + token
29. Per-repo daily budgets / decision→question fan-out / remaining C27 seeds — PLANNED (FEATURES ⚪ rows), not started

**Docs & release hygiene**
30. `docs/release/RELEASE.md`: the round-2 multi-module release flow (sub-tags, sibling-replace allowlist, two-phase --tag/--push)
31. Version-surface inventory doc (flake attr ↔ ldflags ↔ root tag ↔ module tags ↔ CHANGELOG)
32. Backfill CHANGELOG: gosec advisory job + examples G114 fixes never got an entry
33. After next release: multi-module-release retrospective into docs/release/ — BLOCKED: needs the release first
34. Status-report archiving automation: detect fully-resolved reports (all forward items struck through) and propose the `archived/` move + index row update

**Dogfood hardening (the loop that pays for everything else)**
35. cwd-dependence sweep over repo-name consumers (audit/prune/harvest) with tests
36. module-eval check asserts the pool unit's Environment PATH is non-empty — makes d1's class un-deployable
37. `checkProjectsDir` optional when every `--repos` entry is absolute
38. Harvest skip-log change-detection (log a skip class on change/first tick, not every 5m)
39. `scripts/smoke/dogfood-once.sh` (env-gated `TQ_DOGFOOD=1`; stub-agent variant ungated)
40. Archive the 2026-09-10 dogfood proof journal + review log out of /tmp before it reboots
41. Drain/cancel stale queued tasks that would verify with the old single-module verify command
42. Worktree-per-agent design doc (intra-repo parallelism) into docs/planning/
43. Session-close bridge prototype (interactive crush sessions get the pool's close-out; trigger research already done — PR #3146 route)

**Quality baseline (shrink, don't mass-fix)**
44. Lint slice-triage: wrapcheck (~50) then varnamelen (~50) findings
45. Per-module golangci runs in ci.yml (ci-local already loops)
46. `ExitCause` → `ExitError` rename consideration — needs a v0.3 window
47. err113 static sentinel at `cmd/tq/doctor.go:472` (hard-failed lint-annotations once, then aged silently)
48. Measure CI time impact of disk-derived `-count=1` module loops; tune if dominant
49. Multi-repo smoke against the nix-built v0.2.0 binary (`TQ_BIN=result/bin/tq …`)
50. templ-components evaluations: `KanbanBoard` vs custom board, `PageProps.SEO` + `icons.Render` adoption

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **Deploy now?** The live pool is requeue-looping every agent task on the git/PATH issue (d1); the fix is on master and the flip is sudo/owner-run. Should you run the SystemNix input flip + `nix run .#deploy` now — or is there a reason to keep the old deployment until v0.3?
2. **`tq agent-pool --model`: remove or keep?** It resets reasoning effort by design of crush's `-m` path; bootstrap's `.crushrc` block is the correct carrier (documented today). Hard-failing the flag (with remediation guidance, like the yolo-without-crushrc refusal) prevents the footgun; keeping it preserves an escape hatch for repos that genuinely want payload pinning. Which way do you want it?
3. **What should "wait for the agent to be done" look like?** `tq enqueue --wait [--timeout]` (flag on the existing command, streams that task's facts until terminal) vs a separate `tq run` verb (enqueue + attach + exit with the task's status) vs both. This is the missing half of your stated goal — which surface do you want, and should the exit code mirror the task's terminal state?

---

*Prepared per the done-prompt contract: report file exists at `docs/status/2026-09-10_05-29_readme-overhaul-tqdb-incident.md`; next items above are harvestable (unblocked ones carry no `— BLOCKED:` marker only where they are genuinely agent-executable). Waiting for instructions.*

**Post-script (05:32, collision noted)**: a concurrent session's report landed at 05:30 (`2026-09-10_05-30_loop-engine-built-task-closeout-and-docs-health.md`) implementing `--task-closeout` (the a)-g) close-out prompt) and a docs-health status task every N completions — that session's work supersedes the *implementation-status* framing of this report's b2/c3 (the close-out and docs-health loops now exist, unit-proven, not yet live-proven per its own TL;DR). This report's session-scoped claims (README overhaul, TQ_DB incident, d1 live pool evidence) are unaffected. Its items overlap this report's f-list: f34 (archive automation) and f5 (blocking wait) should be reconciled against that session's build before picking them up.
