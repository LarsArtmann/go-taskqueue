# Brutal Self-Review & Full Status — After the 21:40 Backlog Sweep

Session: 2026-09-09 00:00–01:48 (this report: 01:48). Scope of this report:
ONLY what this session did and noticed, per instruction. Format note: written
as `.md` by explicit user instruction (the status-report skill's HTML default
was overridden); the skill-mandated self-commit is skipped — the repo's
auto-commit daemon owns commits here, and the hard rule is never commit
without an explicit ask.

---

## a) FULLY DONE (all verified, full `./scripts/ci-local.sh` green)

**Features (13):**
1. `tq harvest --prune-stale` — cancels pending tasks whose TODO item is
   `[x]`; reason on the cancel fact; running/dead reported-only;
   `--dry-run`/`--json`; 3 tests + CLI smoke.
2. Failure evidence on `task.failed` facts — `Store.Fail`/`FailPermanent`
   gained an evidence param (both stores); executors publish
   `FailureEvidence{stage, exit_code, tail}` via the sink (agent, agent
   verify — tail previously discarded on error — and sh); e2e worker test
   pins exit code + stderr tail on the fact.
3. `tq tasks` list view — `--project/--status/--type/--since/--limit/--json`.
4. `tq show`/`tq cancel` unique-prefix ID resolution (ambiguity names
   candidates); `TestResolveTaskPrefix`.
5. `Task-Queue-ID: {{TASK_ID}}` commit-footer contract — harvest/catch-up/
   status prompts carry the placeholder; the agent executor resolves it at
   run time (status/review route through the same `runAgent`);
   `TestAgentPromptTaskIDSubstitution`.
6. `tq stats --json` — aggregate `{by_status, by_project, budget,
   consumer_lag}` (replaces the old tasks-array dump).
7. Budget spend in `tq stats` — `budget today N/M enqueued` always, cap
   comparison with `--daily-budget N`.
8. Sidecar byte cap — `SweepSidecarsByBytes` + `agent-pool
   --log-dir-max-bytes`/`$TQ_LOG_DIR_MAX_BYTES`; `TestSweepSidecarsByBytes`.
9. Web UI cancel reason in the task trail (`detailFacts`/`factReason`);
   `TestDetailFactsSurfacesCancelReason`.
10. Nightly fuzz rotation — `nightly.sh` rotates FuzzParseRepo +
    FuzzExtractResultPayload; workflow commits both corpora; live-verified
    (+22 seeds).
11. `scripts/smoke/bootstrap-install.sh` — fake `$HOME`, stubbed
    systemctl/loginctl, drain-invariant assertions; wired into ci-local.
12. `scripts/check-status-index.sh` — ci-local gate; found and indexed 12
    unindexed reports total (10 beyond the five known, then 2 more from the
    sibling session mid-report).
13. ADR-0010 journal retention stance.

**Real bugs found by the new tests and fixed (2):**
14. **Budget bypass**: the tick's mint passes (review/status sweepers, cqa
    ingest) and the `--once` drain sweeps never checked the daily budget —
    a same-tick completion minted past the cap. Every minting pass now
    re-checks `guard.Check` with its own skip warning;
    `TestBudgetCapsStatusMintedEnqueues` (positive control + capped twin).
15. **`FactsForTask(limit>0)` first-n vs most-recent-n**: both stores
    returned the FIRST n facts while the interface documents the MOST
    RECENT n (webui detail budget + status windows are the bounded
    callers). Both stores now read the tail and flip to ascending; the old
    SQLite test that pinned the wrong behavior was re-pinned.

**Audits/hygiene (6):**
16. Papdashboard watermark audit — NO defect: the 18:31 bridge reads/writes
    its OWN `/tmp/papdbg/tasks.db` (CWD-relative default), which carries
    the eager head-insert row; the repo DB was never its store. Evidence
    appended to the blocked TODO item (safe to kill; forwards nothing —
    head still seq 4, :18100 is a local stub).
17. `TestRestartMidStreamLosesZeroFacts` deflake — bridge A's crash window
    200ms→600ms, `waitFor` 2s→5s; 8× green under -race.
18. `taskid.txt` removed + `.gitignore`d.
19. AGENTS.md: queue-tasks-outlive-TODO-items bullet + manual-worker
    `--once` runbook rule.
20. docs/status/README.md: every report indexed exactly (globs removed).
21. TODO_LIST: 20 items marked `[x]` with inline evidence; CHANGELOG +
    FEATURES updated; session report written and indexed.

Verification: full ci-local gate GREEN twice (the first run failed only on
the sibling session's cross-repo doc paths — fixed via the checker's
allowlist mechanism); Postgres battery verified 2× against a real
`postgres:16` container (removed afterwards).

---

## b) PARTIALLY DONE

1. **Postgres conformance coverage is CI-only in practice**: locally
   `go test ./internal/queue/` silently skips `TestPostgresConformance`
   (`TQ_TEST_POSTGRES` unset). Correct by design (ADR-0007 gating), but a
   dev can believe the battery ran when it skipped. No skip-summary line
   anywhere.
2. **Status-index discipline is only as good as the gate**: the check runs
   in ci-local, but reports land via the auto-commit daemon and sibling
   sessions — between my two ci-local runs, two MORE unindexed reports
   appeared (I indexed them). The gate catches it late (at gate time), not
   at write time.
3. **Failure evidence coverage**: only `task.failed` facts carry evidence;
   `task.requeued` (preflight refusals — dirty tree etc.) carries none.
   Deliberate scope cut, never written down anywhere until now.
4. **(f) of this report is NOT harvested into TODO_LIST/ROADMAP** — see
   section g, question 3.

---

## c) NOT STARTED (deliberately — owner-blocked or out of scope)

1. **Push master to origin** — hard-blocked on owner go/no-go (never push
   without an explicit ask; now even further ahead).
2. **Fate of the `/tmp/papdbg` worker (PID 1039418)** — killable per audit
   evidence, but killing a live process is the owner's call.
3. **`tq cancel` dedup-key release semantics** — owner policy decision
   (documented escape hatch today: edit the item text).
4. **`Filter.Since` SQL pushdown** — `tq tasks --since` filters CLI-side
   after a full `List`. Fine at current scale; flagged as follow-up only.
5. **Retention flags in generated configs** — `tq bootstrap --install`'s
   `renderPoolConfig` and the NixOS module's pool.conf renderer emit a
   fixed key set that predates `--log-dir-max-bytes` (and `-max-age`);
   generated configs carry no retention settings.
6. **Surfacing the commit-footer cross-reference in tooling** — the
   `Task-Queue-ID` footer now exists in the prompt contract, but nothing
   in `tq show`/`tq facts`/web UI greps git log for it yet.

---

## d) TOTALLY FUCKED UP (nothing shipped broken — the gate proves it; the honest list of near-misses and process failures)

1. **Raw-string backticks, twice**: I put `` `Task-Queue-ID: {{TASK_ID}}` ``
   inside Go raw-string prompt templates in drift.go AND status.go — a
   backtick terminates the literal; both files broke compilation. Same
   mistake, two files, one session.
2. **Python-heredoc unescaping, twice**: heredoc `"\n"` sequences became
   literal newlines inside Go source I generated via `python3 <<EOF`
   (status.go, agent_test.go), producing unterminated string literals.
   After the FIRST occurrence I repeated the pattern instead of switching
   to the edit tool.
3. **Deleted a test body by accident**: the edit that inserted
   `TestFailureEvidenceRidesFailedFact` consumed
   `TestUnknownTaskTypeDeadLettersImmediately`'s body through a bad anchor;
   caught by reading the diff, restored immediately.
4. **Line-number `sed` against live-concurrent files**: I fixed test call
   sites with `sed -e '100s/.../...'` while another session was editing
   the same package. One shifted file and I would have corrupted a
   stranger's line. (It worked. That was luck, not craft.)
5. **One edit raced the sibling session's `cmd/tq/main.go`** (mtime guard
   fired); recovered by re-reading and re-applying via a script. The
   correct move was a re-read + `edit`, not a python string replace.
6. **First full-gate run was at the END**: had the sibling session's ghost
   doc-paths (SystemNix `docs/services/tq.md`, `modules/nixos/...`) broken
   master, I would have found out after an hour of work instead of
   mid-session. AGENTS.md's own rule ("run ci-local before declaring
   success") invited exactly this late, single-shot usage.
7. **Almost broke cooperative cancel**: while threading failure evidence
   through `runAgent` I collapsed the cancelled-vs-failed branches; the
   merged error would no longer match `context.Canceled`, silently turning
   cooperative cancels into burned attempts. Caught re-reading my own
   diff, not by a test (no test would have caught it — that's a test gap,
   see e.7).

---

## e) WHAT WE SHOULD IMPROVE (lessons, ranked)

1. **Ban generated-source via shell heredocs** — for code edits, the edit
   tool (or write) is the only sanctioned path; heredocs cost this session
   four broken-file incidents. (Process rule worth adding to AGENTS.md.)
2. **Run `ci-local.sh` (or at least `go build ./... && go test ./...`) at
   MID-session checkpoints**, not only pre-finish — the multi-agent repo
   makes "green 30 minutes ago" meaningless.
3. **Excerpt helpers are a growing split brain** — this session ADDED
   `oneLine` (cmd/tq/tasks.go), `truncateItem` (harvest/prune.go), and
   `factReason`-adjacent flattening while `budget.firstLine`,
   `cmd/tq.truncate`, `executor.tailBytes`, `harvest`'s own text handling
   already existed. Consolidate into one well-tested truncate/first-line
   util per package boundary.
4. **JSON/text output surfaces drift easily** — `tq stats --json` lacks
   `journal_head` while the text output prints it; the web UI budget card
   and the CLI budget line now share semantics but not code. Define the
   stats payload once (there are three renderers now: text, JSON, webui).
5. **Test the error-message contracts that finalize state**: the
   cooperative-cancel matching (`errors.Is(err, context.Canceled)`) is a
   load-bearing string/wrapping contract with zero direct test (my
   near-miss d.7 proves it). Pin it: a cancelled agent run must finalize
   as Cancelled, not Failed.
6. **Skip-noise visibility**: env-gated suites (Postgres, TQ_BASELINE)
   should print a one-line summary of what was skipped when the env is
   unset, so "ok" never hides a silent skip of the whole battery.
7. **`printBudgetSpend` semantics under filters**: the spend line is
   global even when `tq stats --project X` scopes the table — either
   scope the spend or label it global; today it can mislead.
8. **The auto-commit daemon makes attribution fuzzy** — several of my
   changes landed as `chore: auto-commit N changed file(s) (heuristic)`
   while two got real messages (`fix(queue): conform to...`). Fine, but
   status reports must keep citing file paths (this one does) since
   history won't tell the story.

---

## f) Up to 50 things to get done next

Brainstorm from this session's observations — NOT yet harvested into
TODO_LIST (see g.3). Roughly impact-ordered within groups.

**Follow-ups to this session's work:**
1. Pin the cooperative-cancel finalize contract (agent run cancelled ⇒
   task Cancelled, never Failed) — the d.7 near-miss as a test.
2. Add evidence to `task.requeued` facts (preflight reason is already an
   error string; make it structured like FailureEvidence).
3. `Filter.Since time.Time` SQL pushdown in both stores; `tq tasks` uses
   it instead of the CLI-side filter.
4. Surface `journal_head` in `tq stats --json` (parity with text output).
5. Scope (or label) the `budget today` line when `tq stats --project` is
   set.
6. `tq facts`/`tq show`: detect a `Task-Queue-ID: <id>` footer pattern in
   output tails and cross-link commits (the footer contract's payoff).
7. Add `log-dir-max-age`/`log-dir-max-bytes` keys to `renderPoolConfig`
   (tq bootstrap --install) when retention flags are passed.
8. Same retention keys in the NixOS module's poolSettings docs/rendering.
9. Consolidate excerpt helpers (`truncate`, `firstLine`, `oneLine`,
   `truncateItem`, `tailBytes` families) into one tested util per package.
10. `resolveTask` prefix scan → SQL `LIKE 'prefix%'` pushdown if queue
    sizes ever make the full List visible in profiles.
11. `tq harvest --prune-stale` inside `agent-pool` startup (one sweep
    before the first tick) — makes relaunches self-cleaning.
12. Postgres: `TestPostgresConformance` in ci-local behind
    `TQ_CI_POSTGRES=1` (docker-less skip stays default).
13. Skip-summary: `go test ./...` output line listing env-gated suites
    that skipped (postgres, baseline, windows-excluded unix suites).
14. docs/status index check as a git pre-commit hook (catches at write
    time, not gate time).
15. Test that `tq stats --json` output round-trips into the webui budget
    card semantics (shared projection, not parallel implementations).

**Owner-blocked decisions (unchanged, evidence now attached):**
16. Push master to origin (go/no-go).
17. Kill or keep `/tmp/papdbg` worker (audit says: safe to kill, inert).
18. `tq cancel` dedup-key release semantics.

**Noticed in passing, worth queueing:**
19. AGENTS.md process rule: no shell-heredoc source edits (lesson e.1).
20. AGENTS.md: mid-session build/test checkpoints in multi-agent mode.
21. The `webui` write-actions session (AllowWrites/CSRF, board view, LAN
    redesign) landed concurrently — its own report exists; a cross-session
    integration review (my prune/cancel-reason work × their admin write
    paths) has never run end-to-end together until ci-local did.
22. `tq tasks` gained no webui twin (the dashboard table exists; a
    `--since` URL param would mirror it).
23. `FailureEvidence.Tail` has no size guarantee across executors (agent
    4096, verify 4096, command 4096 — pin one constant).
24. The failure-evidence JSON should be documented in AGENTS.md's payload
    contracts section (it lists sh/agent/review/status but not
    task.failed detail).
25. `check-doc-refs.sh` allowlist grew two entries — annotate in
    AGENTS.md that SystemNix paths are intentional cross-repo citations.
26. Nightly fuzz workflow could rotate targets per-day-of-week (full time
    budget each) instead of running both every night (cost parity choice).
27. Postgres `Fail` still labels its class `transient` in the exhausted
    path while SQLite says `exhausted` — pre-existing divergence my
    battery documented but did not fix (scope cut).
28. `printTaskList` truncates errors at 60 chars with no width flag.
29. `tq tasks --json` lacks a total-count field (scripts paginate blind).
30. e2e budget test could assert the review-mint path too (it currently
    pins status only; review sweeper got the same guard).
31. Bootstrap smoke: assert the .tq-verify content matches the flag
    exactly (currently only presence).
32. status-index check could also verify the DATE column matches the
    filename (two rows today have wrong dates? none — but the check
    doesn't look).
33. CHANGELOG "Changed" section is underused (the webui trail change
    landed there alone; consider moving behavior-visible entries
    consistently).
34. Consider `--prune-stale --json` field parity with `tq audit --json`
    (DriftResult vs PruneResult naming drifted: StaleDone vs Running/Dead).
35. The three `--once` drain-sweep guard checks could share one helper
    (mintPass(ctx, name, fn)) — cmdAgentPool grew four near-identical
    guard blocks this session.

**Bigger, pre-existing (from TODO_LIST context, not re-audited):**
36. SystemNix host-side deploy + round-9 cutover (TODO line 65, sudo-gated
    user-run).
37. Cut v0.2.0 (blocked on owner go/no-go).
38. CQA bridge live verification (blocked on owner credentials).
39. Status-report review policy (owner trust decision, TODO line 51).
40. Hard mechanical cap on status-agent TODO appends (TODO line 52).
41. `--status-every N` in dogfood launch + systemd sample (TODO line 53).
42. Journal compaction CLI per ADR-0010's demand triggers (≥50k facts).
43. Dispatcher phase 2 (notify-after-commit) per ADR-0009.
44. `journal.Journal` test-double demotion completion (ADR-0009 D-series
    leftovers).
45. Web UI `--allow-writes` hardening review (CSRF + auth matrix against
    the LAN threat model) once the sibling session's work settles.
46. Windows runtime coverage honesty (compile-only gate remains).
47. vendorHash/GOEXPERIMENT guardrails already documented — periodic
    re-verification cadence.
48. Per-repo budget accounting (global spend today; fleets may want
    per-project caps).
49. `tq doctor`: add a prune-stale dry-run hint when stale pending tasks
    exist (ties d1 detection to the new sweep).
50. Fleet: other sibling repos' TODO backlogs still lack prune-stale
    awareness in their runbooks (same zombie class exists there).

---

## g) Questions I cannot figure out myself

1. **Push go/no-go?** Master is far ahead of origin (this session alone
   added ~15 commits, mixed with the sibling webui session's work). The
   TODO item has been blocked on this answer since 21:40 — I will never
   push without it.
2. **May I kill `/tmp/papdbg`'s worker (PID 1039418)?** The audit evidence
   says it is self-contained and inert (own DB, stub dashboard, forwards
   nothing), but it is a live process you started; its fate is your call.
3. **Should section (f) be harvested into TODO_LIST.md now?** The
   status-report skill mandates HARVEST, but TODO_LIST is machine-consumed:
   harvesting items 1–35 means a live pool may mint agent tasks (real
   money) from this brainstorm. Harvest all, harvest a curated top slice,
   or leave as ROADMAP fuel until you say so?

---

*Point-in-time snapshot; re-verify before treating any claim as current
(the repo moves under concurrent sessions).*
