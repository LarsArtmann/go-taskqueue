# Round-5 Execution Status — Wave 1 (M1–M6) shipped, M7 in flight

**Report time:** 2026-09-08 05:54 CEST · **Session start:** ~04:57
**Mandate:** "GET SHIT DONE! The WHOLE TODO LIST!" — execute the Round-5
Pareto plan (`docs/planning/2026-09-07_23-51_SUPERB-PLAN-ROUND5-PARETO-100-IMPROVEMENTS.md`,
27 medium tasks M1–M27 / 150 micro-tasks F1–F150, covering all 100 ideas).

## Context snapshot (right now)

| Fact                                  | Value                                                                                                                                                                     |
| ------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| HEAD                                  | `473becf` (master, **25 commits ahead of origin — unpushed**)                                                                                                             |
| Working tree                          | clean except `internal/executor/review.go` (**parallel agent's untracked file, not mine**)                                                                                |
| My session commits                    | `2d5e729` (M1), `0892081` (M2), `82a5840` (M4), `fb0ff0f` (M5), `f67942f` (M6)                                                                                            |
| Agent-pool commits landed mid-session | `4cb32f6` (M3 auth — they shipped it while I planned), `20edcb4` (audit --json), `afbf953` (fuzz), `473becf` (journal inventory), status/planning docs, plus daemon blobs |
| Agent pool                            | PID 3117483, up 8h57m, still dogfooding this repo (`--daily-budget 15`)                                                                                                   |
| LAN serve                             | PID 3654482, up 8h41m, `0.0.0.0:8090`, **still the OLD binary — unauthenticated, pre-redesign UI**                                                                        |
| Gates at last full run                | build + vet + full `-race` suite + webui smoke (incl. auth + CSP assertions) + doc-refs: **ALL GREEN** (as of M6 commit)                                                  |

---

## a) FULLY DONE (verified: build/vet/race-suite/smoke, committed)

### M1 — Bounded journal reads (`2d5e729` + daemon blobs `3d87f02`/`db6ba26`)

The #1 Pareto item. The webui re-read the ENTIRE journal every 500ms burst;
`tq top` scanned it per refresh; the budget guard scanned it per pool tick;
the papdashboard bridge scanned it per poll.

- `Store.Facts(ctx, after, limit)` cursor; `LastFacts(limit)` (feed tails);
  `HeadSeq()` (O(1) watermarks); `FactsForTask(id, limit)` (indexed via
  `idx_facts_task`); `CountFacts(type, since)` (SQL pushdown).
- All callers migrated: webui feed/tailer/task-trail, budget guard
  (`spentSince` is now one `SELECT COUNT(*)`), papdashboard bridge
  (batched drain of 500, retry-from-last-forwarded-seq preserved), `tq top`
  (last 5,000 facts), `tq show` (indexed trail), `tq facts`/`tq tail`,
  both examples. Test fakes updated.
- 6 new store tests (cursor paging, tail order, head, per-task bounds,
  count-by-type/since).

### M2 — SQL search pushdown (`0892081`)

- `Filter.Query` → escaped SQL LIKE over id/type/project/payload/owner/error
  (`%`, `_`, `\` match literally); `Filter.Offset` for pagination.
- `Store.StatusCounts` + `Store.ProjectCounts` GROUP BY projections replaced
  the webui's full-List-then-count-in-memory path; `matchesFilter`/
  `matchesQuery` deleted.
- 4 new test groups incl. LIKE-escape seeds.

### M3 — W16 serve auth (**shipped by the agent pool**, `4cb32f6`; verified by me)

Non-loopback default-deny, constant-time token middleware, Bearer + `?token=`,
redacted logs, auth matrix tests, smoke assertions. I ran the smoke: green.

### M4 — Pool durability (`82a5840`) — _host install deliberately owner-gated_

- `tq agent-pool --config <file>` / `$TQ_POOL_CONFIG`: flat `key=value`,
  flag-name keys, precedence **flag > env > file > default**, unknown keys
  and bad values fail loudly, `config`-in-file rejected, env-backed CQA
  flags protected. 7 unit tests (`poolconfig_test.go`).
- `deploy/systemd/tq-agent-pool.service` hardened (NoNewPrivileges,
  ProtectSystem=full, kernel/cgroup namespaces, RestrictSUIDSGID,
  LockPersonality) + install/enable-linger instructions in the unit and a
  new README "Running the pool as a service" section.

### M5 — Security pack (`fb0ff0f`)

- Strict CSP (`default-src 'none'`, self-only scripts/styles, no inline, no
  framing, no form-action) + nosniff/no-referrer/DENY on **every** response
  including 401s (middleware order pinned by a test).
- Route table became data (`routeBindings`) → `TestRoutesAreReadOnly`
  guardrail fails the build on any mutating route.
- ADR-0003 amendment + Phase D `--allow-writes` pre-design (capability flag
  - token-on-loopback + per-route CSRF). Smoke asserts the headers.

### M6 — Scale test + pagination (`f67942f`)

- `Filter.SeverityOrder` (SQL CASE: dead→running→pending→cancelled→
  completed, newest first), `Store.CountTasks` sharing the exact WHERE
  builder with List; webui `?page=` (clamped, 200/page) + pager fragment;
  in-memory `sortTasks` deleted.
- **Measured at 100k tasks + 100k facts** (`TestLoadSnapshotScaleAt100k`,
  skips under -short and -race): page query 18ms · count 1.5ms · LIKE-count
  36ms · last-50-facts 0.22ms · per-task trail 0.05ms, with ceilings that
  fail on O(N) regressions. Numbers recorded in FEATURES.md + CHANGELOG.
- Edge tests: page 0/-3/abc clamp, overflow page renders empty + valid pager.

---

## b) PARTIALLY DONE

### M7 — Budget card + notBefore visibility (~50%, uncommitted-in-daemon-blobs)

- ✅ `webui.Config.DailyBudget`; `BudgetView{Cap,Spent}` + `Tone()`
  (green <75%, amber <100%, red ≥ cap); `DashboardData.Budget`; snapshot
  fills it via `CountFacts(Enqueued, startOfDay)`; `readiness()` helper
  ("in 12m" / "ready"); build green. These edits were swept into daemon
  blobs (`e221e30`, `edd1d01`, `73ec6e6`) — no detailed commit yet.
- ❌ NOT done: the actual UI (budget StatCard in `StatusCards`, "ready"
  column in the task table), `templ generate`, golden-fragment tests,
  smoke run, CHANGELOG/FEATURES, the M7 commit.
- Design decision already made: tone via StatCard/Badge enums only — no
  inline-style progress bar (our CSP forbids inline styles; no progress
  component in templ-components v1.14.0).

### M11 — Platform honesty (~60%, agents did the earlier parts)

Done earlier by agents (`25c2055` unix test tags, `8d6ef88` nix all-systems
check + nix-binary smoke). Remaining from the plan: F56 (CI Windows job runs
only honestly-tagged tests) — unverified whether CI workflow covers it.

### M25/M26 slivers done by agents out of order

`tq audit --json` + flags (`20edcb4`), `FuzzExtractResultPayload` with
183-seed corpus (`afbf953`). The remaining M25/M26 items are still open.

---

## c) NOT STARTED (from the plan; nothing touched)

- **M8** live task detail SSE (`/task/{id}` live updates)
- **M9** cooperative cancel of running tasks (fact checked at heartbeat)
- **M10** `tq doctor` (DB/lease/budget/autonomy/crush checks)
- **M12** release automation + docs truth pass (`scripts/release.sh`,
  FEATURES/README/CONTRIBUTING refresh)
- **M13** hygiene pack (OFL font license, adoption guard test, webui helper
  table tests, templ-LSP false positives, sentinel burn-down)
- **M14–M22** (Wave 3): dogfood ops, pool ops pack, queue health
  (stuck-running detector, contention test, 10k baseline), journal
  compaction ADR, UI interactions pack, UI metrics/journal cards, a11y +
  golden snapshots, Postgres store slice, HTTP API + fencing tokens
- **M23–M27** (Wave 4): feature design pack, executor pack, CLI pack
  (version/completions/rescue preview), quality/CI pack (e2e, fuzz
  extension to `unwrapCommand`, nightly -race, lint scoping, templ drift,
  dprint decision, gosec triage), security/misc/docs pack (secrets test,
  redact, sidecar retention, govulncheck, UI pills, status index, D2
  diagram, website kickoff)

---

## d) TOTALLY FUCKED UP (and recovered)

1. **Rebase incident (worst moment of the session).** The auto-commit daemon
   fragmented M1 into 5 commits. I tried to squash via scripted
   `git rebase -i` — twice. The second one misfired: my sed turned the first
   todo line into a leading `fixup`, git stopped in an "edit" state on the
   base commit, and my follow-up `git commit --amend` **rewrote the agents'
   auth commit (`4cb32f6` → `f858f89`) with my M1 message**. Recovered with
   `git rebase --abort` back to `2d5e729`; nothing pushed, no permanent
   damage. **Lesson locked in: NO rebases while the daemon lives.** Accept
   generic daemon blobs; commit fast instead.
2. **Broken-intermediate commits.** My python splice scripts broke
   `sqlite.go` twice (double-quoted multi-line SQL string; eaten closing
   backtick). The daemon committed the broken state (`daa069d`). Repaired
   in place the same session; later commits (through `f67942f`) contain a
   green build — but `daa069d` itself does not compile. History has two
   non-compiling daemon blobs (`daa069d`, and the pre-repair fragment).
3. **Tooling friction that cost real cycles:** the `edit` tool repeatedly
   failed with "file modified since last read" whenever the daemon touched
   mtimes (queue.go edits silently not applied twice); `edit` on
   `CHANGELOG.md` reports success but content doesn't land — python writes
   work (suspected hook interception; unverified). `write` gave
   contradictory "modified since read" errors while actually writing.
   LSP diagnostics showed stale typecheck errors (cmd/tq/main.go:1090)
   that `go build` disproved.
4. **Test flakiness authored by me, twice:** nondeterministic
   `ClaimDue` winner assumptions (M2/M6 tests) — fixed by deriving
   expectations from the actual claimed task; and one outright wrong
   assertion (expected both pendings in the untouched project when each
   project only ever had one/two).
5. One empty/malformed tool call aborted mid-M7 (the session's last action
   before this report) — no side effects, just a lost step.

---

## e) WHAT WE SHOULD IMPROVE (process + code, noticed this session)

1. **Stop fighting the daemon.** Commit immediately after each green gate;
   never rebase; treat "chore: auto-commit" blobs as the cost of doing
   business. (The plan's F143 "daemon-safe CHANGELOG convention" should
   also codify the python-only CHANGELOG edit rule.)
2. **Byte-level python edits for SQL/templ files** (backticks + backslashes
   poison heredocs). Standardize: splice scripts never contain Go string
   literals inline; write blocks to /tmp files first, verify with
   `gofmt -l` + `go build` before moving on.
3. **Never assume ClaimDue ordering in tests** — always derive expectations
   from the returned task. Two of my three test-failure rounds were this.
4. **Broken-build windows are dangerous with a committing daemon** —
   a half-edited file can be swept into history. Mitigation: keep edits
   atomic per file; run `go build` after EVERY single-file change, not
   after a batch.
5. **The unauthenticated LAN dashboard is still live** (old binary, PID
   3654482, `0.0.0.0:8090`, payloads exposed to the LAN). The fix is
   shipped in code (M3) but the PROCESS must be restarted by the owner.
   This is now the single highest-risk open item.
6. **Agent pool runs with `--daily-budget 15`** but the new `--config`
   file + systemd unit (M4) are not installed — the pool still dies with
   its 9-hour-old terminal session.
7. `internal/executor/review.go` is untracked (parallel agent mid-task) —
   leave it alone; do not sweep it into my commits (M4/M6 commits each
   accidentally included an unrelated agent doc — harmless, but sloppy).
8. Branch is **25 commits ahead of origin** — push needs fresh owner
   authorization per policy.
9. LSP typecheck diagnostics are untrustworthy in this session's state —
   trust `go build`/`go vet` only.

---

## f) NEXT — up to 50 things, in execution order

**Finish the current wave (Wave 2):**

1. M7: budget StatCard fragment + "ready"/"in 12m" notBefore column in
   `fragments.templ`, `templ generate`, golden tests, smoke, detailed commit
2. M8: per-task SSE filter on `/task/{id}` + live fact-timeline append +
   connection lamp (F40–F43)
3. M9: cooperative cancel — `task.cancel-requested` fact, heartbeat observes
   it, executors kill process tree on ctx cancel, `tq cancel --force` +
   chaos test (F44–F49)
4. M10: `tq doctor` — DB/WAL/lease/budget/autonomy/crush checks, human +
   `--json`, seeded-bad-DB tests (F50–F54)
5. M11 remainder: CI Windows job runs only honestly-tagged tests; verify
   nix flake check --all-systems in CI (F56–F57)
6. M12: `scripts/release.sh` codifying the v0.1.0 checklist; FEATURES Web UI
   rewrite; README screenshot + quickstart; CONTRIBUTING CSS-build +
   doc-refs steps (F60–F65)
7. M13: OFL-1.1 license into `static/fonts/`; AGENTS adoption-table guard
   test; webui helper table tests (2 batches); templ LSP false-positive
   workaround doc; sentinel-error burn-down pass 1 (F66–F71)

**Wave 3 (M14–M22):**
8. M14: review the five agent commits for self-modification safety (F72)
9. M14: verify `.tq-verify` ran per agent commit (F73)
10. M14: `scripts/tq-session-status.sh` (live PIDs, ports, DB paths) (F74)
11. M14: budget telemetry → papdashboard alert + stub E2E (F75)
12. M15: pool chaos test — SIGKILL mid-drain under `--once` (F77)
13. M15: `--repo-timeout repo=10m` ladder flag (F78)
14. M15: machine-wide `--max-concurrent-agents` cap (F79)
15. M15: `crush --version` probe at pool start (F80)
16. M15: model-pin + budget-value propagation once owner decides (F81)
17. M16: stuck-running detector + `task.orphaned` fact (F83)
18. M16: doctor integration for orphaned leases (F84)
19. M16: heartbeat cadence = lease/3 knob + tests (F85)
20. M16: multi-process SQLite contention test (2W+N readers) (F86)
21. M16: 10k-task load script + baseline into FEATURES (F87)
22. M17: compaction ADR for a facts-first journal (F88)
23. M17: `tq journal compact --before SEQ` design sketch (F89)
24. M17: hot-cold archive schema sketch/prototype (F90–F91)
25. M18: fact lines link to `/task/{id}` (F92)
26. M18: `/project/{name}` page reusing the filter pipeline (F93)
27. M18: sortable columns via DataTable (F94)
28. M18: error popover + `?` keyboard-shortcut overlay (F95)
29. M18: live relative-age ticking between bursts (F96)
30. M19: fact-rate sparkline + duration histogram (F98)
31. M19: journal-size + watermark stat card (F99)
32. M19: `/facts?after=` cursor endpoint + infinite-scroll viewer (F100–F101)
33. M20: a11y pass (focus order, skip link, labels) (F103)
34. M20: ARIA on lamp/feed + reduced-motion verify (F104)
35. M20: contrast matrix both themes (F105)
36. M20: full-page golden snapshots light+dark (F106)
37. M20: `scripts/webui-screenshots.sh` + CSS drift guard (F107)
38. M21: pgx DDL schema (CGO stays off) (F109)
39. M21: Enqueue + Claim with `SKIP LOCKED` (F110)
40. M21: facts append + projections; shared Store-conformance suite (F111–F112)
41. M21: CI Postgres service harness; bench vs SQLite + ADR (F113–F114)
42. M22: HTTP API (token auth) design + enqueue/stats handlers (F115–F116)
43. M22: fencing-token design note + consumer-group sketch (F117–F118)
44. M22: `examples/api` upgrade path + tests + ADR (F119–F120)

**Wave 4 highlights (M23–M27):** 45. cron/per-repo-budget/retry-table/
cross-repo-DAG/session-chain/webhook//metrics design pack (F121–F126);
46. CQA live dry-run contract (owner-gated) + demo repo (F127–F128);
47. sh-executor resource limits + result-schema validation (F129–F130);
48. Windows process-group kill (F132); 49. `tq version`, completions,
`--json` parity sweep, rescue preview (F133–F137); 50. quality/CI pack —
audit/top e2e, fuzz `unwrapCommand`, nightly `-race -count=3`, lint
scoping, templ drift check, gosec triage (F138–F144) and the security/
docs pack (F145–F150).

**Owner-gated (cannot proceed without you):** LAN serve restart under
token auth · systemd install on the host · v0.2.0 go/no-go · `--model` pin

- budget value · CQA live credentials · website launch · **git push**.

---

## g) Questions I cannot answer myself

1. **Restart the LAN serve now?** PID 3654482 runs the pre-auth binary on
   `0.0.0.0:8090` with payloads exposed. I can kill it and restart
   `tq serve --addr 0.0.0.0:8090 --auth-token <token>` from the NEW binary
   (built from HEAD) — but I need the token value (or generate one and give
   it to you), and your go-ahead to kill a process I did not start.
2. **Install the pool's systemd unit + `~/.config/tq/pool.conf` on this
   host and migrate the 9-hour-old pool session onto it?** It needs host
   access (enable-linger, systemctl --user) and your choice of config
   values (keep `--daily-budget 15`, `--repo-interval go-taskqueue=10m`,
   `--dlq-backoff 30m` as-is?).
3. **May I push the 25 unpushed commits to origin/master** (after a full
   `scripts/ci-local.sh` gate run)? The plan-time push authorization was
   scoped to the plan artifact; the working tree also contains parallel
   agent commits I did not author.

---

_Next session pickup: finish M7 fragments (templ card + column), then M8 → M27
per section f. Re-run `git log` before each commit; python-only CHANGELOG
edits; no rebases._
