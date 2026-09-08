# Session Status Report — `tq bootstrap`: The One-Command Meta Harness Bootstrap

| Field               | Value                                                                                                                    |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| Date                | 2026-09-08 19:59 CEST                                                                                                    |
| Repo                | `go-taskqueue` (work happened here; started as a CV conversation)                                                        |
| HEAD at report time | `440bf04 feat(taskqueue): add --no-run flag and improve repo state reporting` (daemon-committed)                         |
| Session type        | **Implementation** — new feature shipped, tested, gated, E2E-verified                                                    |
| Scope               | This session's run only: research → probes → `tq bootstrap` implementation → tests → gates → E2E. No unrelated research. |

## What This Session Did

Owner asked for **one command** that ensures all bootstrap steps for 1..N projects, with 1..N agents, proper model/provider/reasoning (default: max possible) — the meta layer: continuously run Crush agents that improve projects until they run out of good ideas, runnable while the owner sleeps.

Research → design → implement → verify, one step at a time:

1. **Research**: `crush run` flags (no reasoning flag — config-level only), `AgentPayload` schema, verify resolution chain (`.tq-verify` > payload > auto-detect), autonomy preflight (`requireRepoAutonomy`), `cmdAgentPool` flags, harvest parsing (`- [ ]` checkboxes, `— BLOCKED:` opt-out), systemd unit, pool config loader, crushrc config surface (`model large <m> --reasoning-effort …`; `model add` requires a DECLARED provider).
2. **Empirical probes (cheap, decisive)**:
   - `model add zai/…` in a scratch `.crushrc` → **hard error** ("provider does not exist") → managed blocks must use `model large` slot lines only.
   - `crush run -m zhipu-coding/glm-5.3` → "large model not found" → `-m` needs a USABLE (declared+keyed) provider, not just a catalog entry.
   - `crush run -m zai/glm-5.3-flash` → resolves correctly, then **429**: the z.ai 5-hour usage quota was exhausted (resets 02:30). Key operational discovery for overnight runs.
   - Global crushrc already pins `--reasoning-effort xhigh` on `zai/glm-5.3-flash` → "max possible" = `xhigh` on this machine.
3. **Implemented** (see a) below for the full list).
4. **Verified**: full suite `-race` green, `ci-local.sh` ALL GREEN (twice — first run caught gofmt drift), E2E on a throwaway repo (files written → committed → generated `.crushrc` **loads in crush**), live `--dry-run` against the real CV + go-taskqueue repos.

**Mid-session discovery:** parallel sessions had already converted CV's TODO_LIST to **96 open checkbox items** and pinned CV's `.tq-verify` (`templ generate && bash scripts/go-change-gate.sh`) — my same-evening advice that "CV would harvest ~zero tasks" was stale within the hour. The only remaining CV gap was `.crushrc` autonomy, which `bootstrap` now writes.

---

## a) FULLY DONE

1. **`tq bootstrap` command** (`cmd/tq/bootstrap.go`, ~550 lines) — the one command:
   - 1..N repos (positional args and/or `--repos`, **any flag order** — custom `reorderBootstrapArgs` because Go's `flag` package stops at the first positional)
   - `--agents N` → pool concurrency AND machine-wide agent cap
   - `--model provider/model` + `--reasoning` (default `xhigh`) pinned into each repo's managed `.crushrc` block + payload `-m`
   - `.tq-verify` ensure with honest 4-state reporting (`wrote`/`override`/`kept`/`none` — the dry-run override misreport bug was caught and fixed via tests)
   - Managed-block writer: idempotent (byte-identical re-runs report no change; edge-blank-line accumulation bug caught and fixed), preserves all user content outside markers
   - Commits exactly `.crushrc` + `.tq-verify` (`chore(tq): bootstrap agent autonomy + verify pin`), never pushes
   - TODO preview (open-item counts via the real `harvest.ParseRepo`), dirty-tree WARN before the pool refuses at task time
   - `--dry-run` (nothing written, prints the would-run argv), `--no-run` (ensure + exit; pool starts later), `--install` (systemd user unit rendered with the real binary path + `pool.conf` + `daemon-reload` + `enable --now` + linger)
   - Delegates to the existing `cmdAgentPool` — all safety rails (budgets, review+autofix, project-exclusive, DLQ, lease/reclaim) inherited, nothing reimplemented
2. **`executor.DetectVerify` export** — bootstrap reuses the executor's detection instead of duplicating it.
3. **10 new tests** (`bootstrap_test.go`): managed-block idempotency + user-content preservation + block replacement, verify states (override rewrites, dry-run doesn't, existing kept, none honest), commit-stages-only-its-files + second-run no-op, argv composition, pool.conf rendering, **unit-template parity test against `deploy/systemd/`** (kills the embedded-copy split brain at test time), arg validation table, dry-run-writes-nothing, mixed flag/positional ordering.
4. **Docs**: README "One command from zero" section, AGENTS.md command line, full `--help` usage text with examples.
5. **Gates**: `go build`, `go vet`, `go test ./... -race` (all green), `./scripts/ci-local.sh` **ALL GREEN** (vet → build → windows cross-compile → race tests → gofmt → advisory lint → harvest-parse guard → web UI smoke → doc-reference → git add -A → nix build → nix flake check).
6. **E2E proof**: throwaway repo → bootstrap `--no-run` → `.crushrc` + `.tq-verify` written, committed (2 files exactly, clean tree for those), and `crush models` from that repo confirms the generated config **loads in crush without error**.
7. **Live dry-run** against real repos: CV → 96 open items, verify override honored in report; go-taskqueue → 11 open items, existing `.tq-verify` kept.

## b) PARTIALLY DONE

1. **Reasoning-attribution residual uncertainty**: probes proved the slot line loads and `-m` resolves the model, but the z.ai 429 blocked the final check that `-m` selection inherits the slot's reasoning effort. Mitigated by design (slot line pins the SAME model the payload pins) but not proven end-to-end. Not documented in the README — only in this report and conversation. **Open item.**
2. **The "while I sleep" end-state**: built and unit/E2E-verified, but **zero real agent tasks have run through bootstrap**, and `installService()` (systemctl/loginctl execution) was **never executed live** — deliberately (enabling a persistent daemon is a machine mutation = owner call), but it means the install path is render-tested only. First supervised `--once` tick not run (quota was exhausted).
3. **CV enrollment**: CV now harvests 96 items, but the ROW QUALITY was never audited this session (are they single-session-sized? are owner-gated rows marked `— BLOCKED:`?). Unknown → untrusted until audited.
4. **Repo conventions**: FEATURES.md and CHANGELOG.md were NOT updated for the new feature (daemon committed code+README+AGENTS only). Gap against this repo's own docs conventions.
5. **Previous CV status report** (18-23, advisory session): its section (f) still awaits a docs-health HARVEST — unchanged this session.

## c) NOT STARTED

- First supervised pool tick (`tq bootstrap … --once`) on any real repo.
- Overnight/systemd activation on this machine (`--install` execution).
- CV's 96-row quality audit + owner-gated-row blocking pass.
- The 6 dead-lettered tasks in the tq DB (`tq dlq`) — noticed early this session (21 done / 6 dead), never investigated.
- GitHub Actions billing fix (CV) — carried over, still untouched.
- Provider strategy for overnight runs under zai's 5h quota window (gemini/kimi/synthetic keys exist via sops; never evaluated as `--model` targets).
- `TQ_LOG_DIR` wiring: sidecar logs are OFF by default (`writeOutputSidecar` no-ops when unset) — bootstrap doesn't render it into pool.conf yet.

## d) TOTALLY FUCKED UP

**Nothing destructive this session.** Honest finds:

1. **The zai 5h quota is exhausted right now** — an overnight pool on the default model would burn 429-retries until 02:30. Surfaced in time by the probe; not fixed (provider choice is the owner's).
2. **Attribution noise in git history**: the daemon swept a PARALLEL session's webui changes into "my" commit `84763e2` alongside bootstrap.go + DetectVerify. Known daemon behavior; noted, not mine to rewrite.
3. **Edit-tool race with the daemon**: `--no-run` edits were refused once ("file modified since read") — the daemon's sweep + gofmt had reshaped the file mid-edit. Re-read, re-applied. The documented edit-window discipline worked, but it cost a round trip.
4. **Same-session staleness strikes again**: my previous-turn claim ("CV harvests ~zero tasks") was wrong within the hour (parallel sessions converted TODO_LIST). Corrected by the dry-run's live parse — but it validates, again, that nothing in this ecosystem is true until you run the check NOW.

## e) WHAT WE SHOULD IMPROVE

Brutal, session-scoped:

1. **No `git pull`/remote check at session start** — AGENTS.md explicitly warns "multiple concurrent agents, `git pull` your assumptions". I read the local AGENTS.md and re-read files when the edit tool refused, but never checked origin state or unpushed counts before building on the tree. Lucky, not disciplined.
2. **FEATURES.md/CHANGELOG skipped** — I updated README/AGENTS (usage docs) but not the repo's feature inventory/changelog. The docs conventions exist; I applied them selectively.
3. **The install-path decision was silent** — I deliberately never executed `--install` live (system mutation), but I didn't explicitly FLAG that as an owner decision in the wrap-up; it surfaced only in this report. Big side-effect surfaces should be named decision points, not implicit omissions.
4. **Residual uncertainty left undocumented in code** — the `-m`-inherits-reasoning question deserves a README caveat or a first-run verification step in bootstrap itself (e.g., grep `crush run --verbose` output once). Shipped without it.
5. **Verification asymmetry**: e2e used `--no-run`, so the DELEGATION path (bootstrap → cmdAgentPool argv handoff in a real process) ran only in dry-run print form. A supervised `--once` on a scratch repo would close it (skipped for quota reasons — the right call, but it remains open).
6. **Test for the daemon-race class**: nothing pins bootstrap's behavior when the tree changes mid-run (the ensure→commit window). Low probability, but the repo's own AGENTS.md says to expect it.

## f) TOP NEXT THINGS (up to 50 — sourced from this session's observations; 28 honest items beat 50 padded ones)

Legend: **[O]** owner-gated · **[M]** machine-executable · **[P]** process/decision

| #  | Thing                                                                                                                    | Source                                      |
| -- | ------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------- |
| 1  | Investigate the 6 dead tasks (`tq dlq`, root-cause each) before trusting overnight runs                                  | Session observation (stats: 21 done/6 dead) |
| 2  | First supervised tick after quota reset: `tq bootstrap CV,go-taskqueue --once`                                           | Session deliverable, never run              |
| 3  | Execute `tq bootstrap --install` on this machine (systemd + linger)                                                      | b2 — owner call                             |
| 4  | Audit CV's 96 checkbox rows: single-session size, owner-gated rows get `— BLOCKED:`                                      | b3 — dry-run only counted                   |
| 5  | Fix GitHub Actions billing (CV)                                                                                          | Carried P0                                  |
| 6  | Verify `-m` inherits reasoning effort (one cheap probe post-reset) or document the residual                              | b1                                          |
| 7  | Update go-taskqueue FEATURES.md + CHANGELOG for `bootstrap`                                                              | b4 — conventions gap                        |
| 8  | Decide the overnight provider: zai (5h window) vs gemini/kimi/synthetic (keys exist)                                     | Probe finding                               |
| 9  | Wire `TQ_LOG_DIR` into pool.conf rendering (sidecar logs off by default today)                                           | Code read: writeOutputSidecar               |
| 10 | Review first agent task outputs (`tq show`, sidecar logs) to calibrate the prompt template                               | Depends on #2                               |
| 11 | Per-repo timeout ladder for CV (`--repo-timeout CV=60m`?) — CV's gate (`go-change-gate` full) may exceed the 30m default | Config knowledge + CV AGENTS                |
| 12 | Enroll-set decision: CV (contains gitignored PII) vs code-only repos first                                               | Session risk note                           |
| 13 | SystemNix enrollment decision — autonomous NixOS-config edits are a bigger blast radius                                  | Owner call                                  |
| 14 | `tq serve` + auth token as the morning-oversight surface (one tab, DLQ visible)                                          | README capability                           |
| 15 | PapDashboard alert wiring (`--alert-url`) so DLQ/budget exhaustion pages the owner                                       | Pool flag exists, unwired                   |
| 16 | Budget sizing: calibrate `--daily-budget` against real per-task cost after #2                                            | Depends on #2                               |
| 17 | Review-pass cost check: confirm `--review` + `--review-autofix` economics on real tasks                                  | Depends on #2                               |
| 18 | CQA bridge verification (still httptest-informed guesses; blocked on live CQA instance)                                  | Harness TODO (BLOCKED row)                  |
| 19 | Cut v0.2.0 (CHANGELOG finalize + tag + nix-binary smoke) — blocked on owner go/no-go                                     | Harness TODO (BLOCKED row)                  |
| 20 | Harvest the 18-23 CV advisory report's section (f) into TODO_LIST                                                        | Carried from previous report                |
| 21 | Push CV `master` (was ahead 1 with the x/sys bump; re-check)                                                             | Carried                                     |
| 22 | Decide daemon-vs-agent commit race policy (pause auto-git daemon during pool runs, or accept noisy history)              | Session observation d2                      |
| 23 | Add a bootstrap README caveat for the `-m` reasoning residual (if #6 stays unproven)                                     | b1                                          |
| 24 | Consider a bootstrap first-run verification step (grep verbose output for reasoning_effort)                              | e4                                          |
| 25 | `--verify` for SystemNix would default to `nix build && nix flake check` (heavy) — scope or override consciously         | autoDetectVerify behavior                   |
| 26 | Agent commit identity: decide attribution policy for agent-made commits (crushrc attribution options)                    | Noticed in crush-config skill               |
| 27 | docs-health audit to reconcile the TODO_LIST checkbox conversion done by parallel sessions                               | Mid-session discovery                       |
| 28 | Log rotation/retention for `TQ_LOG_DIR` sidecars once enabled (#9)                                                       | Follow-on to #9                             |

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF (3)

1. **May I execute `--install` on this machine?** It enables a persistent autonomous daemon (systemd user unit + linger = starts at boot, survives logouts). I deliberately never ran it live; it's render-tested only.
2. **Which provider for overnight runs?** zai's 5-hour quota was exhausted today at ~19:00 (resets 02:30). Stay on `zai/glm-5.3-flash` with backoff, or pin another declared provider (gemini/kimi/synthetic keys exist in sops) for the sleep window?
3. **Is CV in the autonomous enroll set?** Agents with bash can read gitignored PII (`data/apply-identity.json`, platform sessions). Code-only repos are the low-risk start; CV enrollment (96 tasks waiting) is your risk call.

---

## Evidence Trail

- Probes: `crush run --help`, `crush models`, three scratch-`.crushrc` load/run probes, one 429-observed run.
- Code: `internal/executor/agent.go` (full), `cmd/tq/main.go` (cmdAgentPool + registry), `internal/harvest/harvest.go` (Item/ParseRepo/buildPayload), `deploy/systemd/tq-agent-pool.service`, `cmd/tq/poolconfig.go`, global `~/.config/crush/crushrc`.
- Gates: `go test ./... -race` green; `ci-local.sh` ALL GREEN ×2 (gofmt fix between runs).
- E2E: throwaway repo bootstrap → commit `a5e8375` (2 files) → `crush models` load OK; live dry-runs vs CV (96 items) + go-taskqueue (11 items).
- Daemon swept the work into `d742ade`, `d5b0a66`, `440bf04` (+`84763e2`, `1d95e3f`, `4f8010b` earlier); tree clean at report time.

_Point-in-time snapshot — goes stale. Later sessions: docs-health ANNOTATE mode, never rewrite._

## Completion notes (2026-09-08 ~21:15 session)

- **#6 CLOSED — the answer is NO, and it changed the design**: `crush run -m` does NOT
  inherit the slot's reasoning effort. Discriminator (zai quota was back): same repo,
  same `.crushrc` (`--reasoning-effort xhigh`), crush debug telemetry — WITHOUT `-m`
  the run logs `reasoning effort:xhigh`; WITH `-m` it logs `reasoning effort:` (empty).
  Consequence: bootstrap no longer composes `--model` into pool args OR pool.conf
  (`composePoolArgs` + `renderPoolConfig`; tests pin the absence) — the repo `.crushrc`
  managed block is the single model+effort carrier, honored by both agent runs and
  interactive crush. Documented in README + CHANGELOG.
- **#9 CLOSED — `--log-dir` shipped**: `tq agent-pool --log-dir DIR` (env `TQ_LOG_DIR`
  default, config-file `log-dir =` key, env-backed precedence like cqa-*) sets the
  env the sidecar writer reads; bootstrap defaults it ON at
  `~/.local/state/tq/logs` (rendered into pool.conf + composed argv; `--log-dir ""`
  disables). Retention/size cap still open (item #28).
- **DLQ emptied**: all 6 dead tasks root-caused (4× transient DNS i/o timeout to
  api.z.ai, 1× missing templ-components go.mod entry since fixed by the webui work,
  1× gofmt drift since fixed) and rescued via `tq dlq --rescue-all` — tree verified
  healthy first (build + gofmt clean).
- **CV TODO_LIST audit**: 16 owner/billing/manual-gated rows now carry
  `— BLOCKED: <reason>` markers (re-login, OWNER decisions, billing-gated CI, root
  shells, Firecrawl key) — verified live: `tq harvest --dry-run` skips them. 79 rows
  remain harvestable (some multi-item; sizing uneven).
- Full gate re-run after the changes: build + vet + `go test ./... -race -count=1`
  green (one transient vet failure in `internal/status` was a parallel session's
  mid-edit snapshot — resolved by them within minutes, not touched by this session).
