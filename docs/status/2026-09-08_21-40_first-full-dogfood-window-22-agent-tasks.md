# Status Report — the first full dogfood window: 22 queue-carried agent tasks

**Written:** 2026-09-08 21:40 CEST
**Written by:** the status loop itself — status task `8a3dcbfc`, minted by the
`--status-every 1` sweeper at 21:33:46 and executed by a real headless agent
against this repo. This report **is** the live smoke that TODO_LIST.md line 43
("Live dogfood the status loop …") asked for.
**Window:** 22 `agent`-type tasks for project `go-taskqueue`, completed
2026-09-07 21:05:58 → 2026-09-08 21:33:45 (≈24.5 h — the sweeper only went live
tonight, so this is the first accumulated window, not a 24 h of continuous
work; see §e). Every item below was verified against git log and the artifacts
themselves — no claims taken from prompts.

**Queue state at writing:** 22 completed · 1 running (this status task) ·
6 cancelled · 0 pending · 0 dead · DLQ empty · journal head 158 ·
status-sweeper lag 0. Build + vet green; full `go test ./... -race -count=1`
17/17 packages ok at HEAD; `gofmt -l .` empty.

---

## a) FULLY DONE (each verified against a commit or artifact)

### CI and verification infrastructure (5 tasks)

1. **`scripts/ci-local.sh` — the one-command pre-push gate** replicating the
   full CI sequence (vet → build → GOOS=windows → race → gofmt → advisory
   lint → harvest guard → webui smoke → doc refs → `git add -A` +
   `nix build` + `nix flake check`). Commit `115854a`; wired into AGENTS.md +
   CONTRIBUTING.md. It has since gated every session tonight (three reports
   cite "ALL GATES GREEN" through it).
2. **`checks.binary-runs` flake check** — `nix flake check` now executes
   `result/bin/tq --help` so a swallowed build can never yield an
   empty-but-successful store path. Commit `57d24f6`; present in flake.nix:97.
3. **`nix flake check --all-systems` + nix-built binary through the webui
   smoke** — pinned `go-standard.systems` (nixpkgs 26.11 dropped
   x86_64-darwin), eval now covers darwin/aarch64, and `webui.sh` gained
   `TQ_BIN` so the smoke runs the store-path binary. Commit `8d6ef88`.
4. **Windows honesty via `//go:build unix`** on `internal/e2e/{e2e,chaos}_test.go`
   and `internal/harvest/e2e_test.go` (all depend on `#!/bin/sh` stubs and
   signals) — verified in-tree; GOOS=windows build/vet stay green. Commit
   `25c2055` (+ `1bcda0e`).
5. **Nightly fuzz job** — `.github/workflows/fuzz.yml` runs
   `scripts/fuzz/nightly.sh` (60 s `FuzzParseRepo`, hermetic GOCACHE, seeds
   committed back, MAX_SEEDS=1000). Commit `cd9b2c9`; first batches landed
   (165 seeds, ~2.2 M execs, zero findings).

### Bridge and queue fixes (3 tasks)

6. **papdashboard startup-cancel fix** — `Bridge.Run` no longer returns the
   misleading "cannot read journal head" when ctx is already cancelled at
   entry. Commit `aaabe1c`.
7. **`scripts/smoke/papdashboard-e2e.sh` real-dashboard mode** — the
   assertion loops no longer grep the stub-only `INGEST_LOG`; the real mode
   verifies against the dashboard. Commit `07ee238` (script +68/−25).
8. **`harvest.Audit` reports unscannable repos** instead of a silent
   `continue`, with the phantom-parity comment fixed. Commit `71823c1`.

### CLI and serve UX (8 tasks)

9. **cmd/tq complexity hotspots extracted** below the cyclop cap
   (`cmdHarvest`, `aggregateTop`, `cmdStats`, `cmdDLQ` → focused helpers).
   Commit `082e54b`.
10. **First CLI-level tests**: golden `tq audit` drift output, dispatch exit
    codes for unknown command/help, table tests for `splitRepos`. Commits
    `c3d04bd` + `5be4c70`.
11. **`tq worker --once`** — drain-then-exit parity with agent-pool, reusing
    the `hasClaimableWork` loop. Commits `e665707` + `4d48a2d`; it now runs
    every smoke and CI script in this repo.
12. **`tq audit --json` + `--todo-file`/`--type`/`--max-attempts`** — flag
    parity with harvest, golden-pinned JSON. Commits `81d3a5f` + `4248d87`.
13. **Free-port selection in `scripts/smoke/webui.sh`** — kernel-chosen
    ephemeral port by default, `WEBUI_SMOKE_PORT` override, loud failure on a
    taken pinned port; collision-tested. Commit `610d203` (+ `a7f8599`).
14. **`tq serve --verbose` request logging** — middleware logs
    method/path/status/duration, SSE flush preserved, off by default.
    Commit `076e1e8`.
15. **`tq serve` token auth for non-loopback binds** — `--auth-token` /
    `TQ_SERVE_TOKEN`, constant-time middleware (Bearer or `?token=` for
    EventSource), non-loopback default-deny, redacted in verbose logs.
    Commit `7c0f390`. This closed the "whole LAN sees all payloads" hole.

### Fuzzing (1 task)

16. **`FuzzExtractResultPayload`** — the TQ_RESULT regex/JSON decode over
    untrusted agent output, never-panic + property pinned, 183-input
    committed corpus, clean 60 s campaign (~6.3 M execs). Commits
    `5236016` + `905dd00`. Note: this task needed **2 retries** (§d).

### Research and design docs (4 tasks)

17. **Persisted-bridge-watermark design** — storage shape, ack semantics,
    resend window, bootstrap precedence, §7 implementation checklist.
    Commit `e15864a` → `docs/planning/2026-09-08_persisted-bridge-watermark-design.md`.
    The next interactive session executed §7 the same day (watermarks table,
    `tq watermarks show/set`, restart battery) — the queue→design→implementation
    pipeline worked end to end.
18. **ADR-0004 lifecycle & streaming library stance** — cordis gated behind
    five trigger conditions, do/ro verdict persisted. Commit `64b668e`.
19. **SSE `Last-Event-ID` ↔ journal Seq resume spike** — verdict: mapping
    already exists on the wire; snapshot-at-head IS resume for a projection
    dashboard; the spike found the real gap (unfiltered EventSource) which
    task 20 then fixed. Commits `9bd2bf8` + `8347098`.
20. **Web UI: EventSource honors page filters** — `app.js` forwards
    `project`/`status`/`q`/`page`/`token` to `/api/events` so live ticks stop
    clobbering filtered views; pinned by `TestStreamSnapshotHonorsFilter`.
    Commits `b420aa8` + `c5336e1`.
21. **cordis test suite independently verified** — 5/5 packages under
    `-race`, 86.2 % statements, 176 PASS/0 FAIL in the fork at `61ec9f9`;
    ADR-0004 T3 amended from claim to verified. Commit `50711df` →
    `docs/planning/2026-09-08_cordis-test-suite-verification.md`.

### Micro-task (1 task)

22. **`gofmt -l .` dogfood micro-task** — tree already formatted, changed
    nothing, finished cleanly (re-verified independently: `gofmt -l .` is
    empty). Its completion at 21:33:45 is what minted this status task — the
    full loop (micro-task → sweeper → done-prompt agent → report) fired in
    production for the first time.

---

## b) PARTIALLY DONE

1. **"Live dogfood the status loop" (TODO_LIST.md line 43)** — satisfied by
   this very report (a real crush agent wrote `docs/status/*`, appended
   TODO_LIST items, and the completion fact will carry `StatusResult`), but
   the item stays unchecked because this agent is append-only. Someone should
   tick it after reading this file.
2. **`--status-every N` rollout (line 53)** — a pool is live with N=1 as of
   21:32 (`agent-pool --status-every 1 --once …`), but the ROUND4 launch
   command and `deploy/systemd` sample still don't carry the flag, and N=1
   was never consciously chosen by the owner (owner-blocked item).
3. **Sidecar retention** — age-based cap shipped today (interactive session),
   the size/count cap from the same TODO item remains open.
4. **Retry-path evidence** — two window tasks succeeded only after failed
   attempts (fuzz job: 1 retry; ExtractResultPayload fuzz: 2 retries). The
   queue's retry machinery worked; the _observability_ of those failures did
   not (§d4).

## c) NOT STARTED (backlog the window skipped)

Queue-verified: 0 pending tasks. TODO_LIST.md has 6 unchecked items:

1. **Cut v0.2.0** (CHANGELOG finalize, tag, release, nix-binary smoke) —
   BLOCKED on owner go/no-go. Everything else is staged for it.
2. **Verify the CQA bridge against a live CQA API** — BLOCKED on owner
   providing instance URL + credentials.
3. **Live dogfood the status loop** — being satisfied by this report (§b1).
4. **Review status reports too, or keep the file-exists contract?** — BLOCKED
   on owner trust policy.
5. **Mechanical cap on status-agent TODO_LIST appends vs prompt cap** —
   BLOCKED on owner blast-radius preference.
6. **Enable `--status-every N` in the ROUND4 launch command + systemd
   sample** — BLOCKED on owner picking N (and a live N=1 is running ad hoc
   right now, §b2).

## d) TOTALLY FUCKED UP (regressions, broken gates, debt — honest ledger)

No broken gates at HEAD: build, vet, gofmt, the full `-race` suite, DLQ, and
flake checks are all green, and zero tasks died. What follows is the debt and
damage the window left or exposed:

1. **Six zombie tasks sat in the queue for 5–24 hours.** Tasks harvested
   09-07 21:31 → 09-08 05:31 stayed `pending` overnight because the pool was
   down, then got cancelled wholesale at 21:31 tonight — with **empty
   cancellation detail**. All six were items interactive sessions had already
   done ([x] in TODO_LIST), so nothing was lost, but the pattern is
   structural: harvesting while no pool runs creates zombies, and `[x]`-ing
   an item never cancels its already-enqueued task (dedup key only suppresses
   _re_-enqueue).
2. **`taskid.txt` — test garbage committed to the repo root** by the
   auto-commit daemon (`aa1e6ed`, 18:31): a bare task ID
   `000001a081dc3646…` from a session that never cleaned up its scratch file.
   Tracked, un-ignored, meaningless to every future reader.
3. **A verification worker never died.** `/tmp/papdbg/tq worker --alert-url
   http://127.0.0.1:18100` (PID 1039418) has been running since 18:31 — a
   round-6 restart-battery leftover forwarding dead letters to a local
   dashboard for 3+ hours. Manual verification runs should use `--once`.
4. **`task.failed` facts carry no evidence** (`{}` on both retry-path
   failures in this window). When a task burns attempts, the journal cannot
   answer why — exit code and verify tail are dropped on the floor.
5. **Watermark table looks under-populated on live runs.** `tq watermarks
   show` lists only `status-sweeper`, although a papdashboard bridge has been
   running since 18:31 and the persisted-watermark feature shipped at 18:26.
   Either that binary predates the feature or the eager head-insert doesn't
   happen on this path — flagged, not diagnosed (scope).
6. **`docs/status/README.md` index is stale**: 5 of today's 8 reports
   (19:59, 20:56, 20:58, 21:19, 21:22) are missing from the index table; 25
   report files exist. Nothing enforces index parity.
7. **Disk-pressure blip during the window**: task `14f474fb`'s verify ran
   with "no space left on device" on `/mnt/buildcache` (resolved since —
   117 G free) — a near-miss that would have failed the task's verify gate
   on a fuller disk.
8. **Dogfood inversion (process)**: the pool idled 07:00 → 21:25 (~14 h)
   while interactive sessions did the day's real engineering (rounds 5/6/7)
   _outside_ the queue. The queue carried 21 items in two night bursts and
   none of the day's work — the dogfood story "the pool eats this repo" is
   still mostly aspiration.
9. **Minor agent-plot smell**: the window contains design tasks (17–19) whose
   findings were executed by later interactive sessions within hours — good —
   but the _queue items_ for that execution never existed; the loop from
   report back to queue ran through TODO_LIST ticks by humans/other agents,
   not through harvesting. Fine for now; worth watching as the pool takes on
   more scope.

## e) WHAT WE SHOULD IMPROVE

**Process**

1. **Close the zombie window automatically**: harvest should be able to
   cancel enqueued tasks whose TODO item is now `[x]` (dedup-key match), and
   `tq cancel` should record a reason. Tonight's 21:31 cleanup was manual and
   evidence-free.
2. **Make failures self-describing**: `task.failed` facts should carry exit
   code + verify-tail excerpt. Two retries happened this window and the
   journal can't say why.
3. **Traceability**: agent commits should carry the queue task ID in a
   footer (prompt-contract line) so `git log` ↔ `tq facts` cross-reference.
4. **Verification hygiene**: one-shot (`--once`) for manual runs, kill
   leftover processes, don't leave workers bound to scratch endpoints.
5. **Index duty**: each status run (or a doc-check gate) should reconcile
   `docs/status/README.md` with the directory; it already drifted within one
   day.
6. **Prefer the queue for real work**: interactive sessions doing everything
   directly starves the dogfood loop of honest data. Route at least the
   small, well-specified items (like tonight's gofmt micro-task — which
   worked perfectly) through the pool.

**Code** (each mapped to a §f item)

7. Failure-evidence in facts; `tq tasks` list view; ID-prefix `tq show`;
   `tq stats --json`; budget spend in `tq stats`; watermark eager-insert
   audit; fuzz-campaign rotation; Postgres conformance in CI; sidecar size
   cap; cancelled-with-reason surfacing in the web UI once reasons exist.

## f) NEXT THINGS (appended to TODO_LIST.md)

See the freshly appended `- [ ]` items at the bottom of TODO_LIST.md —
queue-hygiene automation, failure-evidence facts, watermark audit,
observability gaps, release-adjacent chores, and one live-review dogfood
window. Owner-decisions are filed as `— BLOCKED:` items, not work items.

## g) QUESTIONS ONLY THE OWNER CAN ANSWER

1. **Push authorization** — master is ~30+ commits ahead of origin and both
   of tonight's interactive reports already asked; the window's 22 tasks are
   part of that unpushed set. Push after the next green ci-local run, yes/no?
2. **The stray bridge worker** — is `/tmp/papdbg/tq worker --alert-url
   http://127.0.0.1:18100` (running since 18:31) an intentional live demo to
   keep, or killable test leftover?
3. **Cancel/dedup policy** — should `tq cancel` release a task's dedup key so
   un-checking a TODO item re-arms it? Today a cancelled task's key
   suppresses re-enqueue forever unless the item text changes (by design per
   AGENTS.md — but the zombie cleanup tonight is exactly the scenario where
   release-on-cancel would help).
