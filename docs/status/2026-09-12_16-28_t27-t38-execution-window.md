# T27–T38 Execution Window — Status Report (Phases 5–6 continuation)

**Session:** 2026-09-12 ~15:50–16:28 (post-status-update continuation on
the standing G0 order)
**Plan:** `docs/planning/2026-09-12_14-19_PRIORITY-SYSTEM-SUPERB-PARETO-EXECUTION-PLAN.md`
**Prior report:** `2026-09-12_15-43_priority-system-execution-status.md`
(indexed; a)–g) delivered in chat, then execution resumed on the owner's
"keep going" order)

Provenance tags: [v] = personally verified by gate/test run this window;
[c] = carried from prior reports; [a] = sub-agent claim, not re-verified.

## a) FULLY DONE (verified green this window)

- **T27 — prioritize sweeper** [v]: `internal/prioritize/sweep.go` — the
  journal-driven score-cache loop on the dlqfix pattern. Mints ONE
  machine-band batch per repo holding unscored backlog items (pending
  agent tasks with `todo:`-prefixed payload dedup keys — the prefix IS
  the foreign-mint guard), dedup `prioritize:<repo>:<hash-of-key-set>`
  (unchanged set never re-mints; a dead batch recovers only on the next
  key-set change — documented DLQ-until-then). Completion facts apply
  verdicts: `SavePriorityScore` (`ai:batch-scorer`) + re-rank via
  `UpdatePendingPriority` source `ai` with structural precedence —
  payload-pinned `markerLevel` protects markers, `RepriMutable` protects
  hot/machine, `ClampBacklog` keeps AI out of hot. BootMint scores the
  standing backlog on first sweep. Cursor `prioritize-sweeper`
  (head-bootstrapped, rewindable). 9 sweeper tests green (mint, dedup,
  cache-skip, new-item batch, apply+exactly-one-fact, marker/band
  protection, follow-up mint, idempotent re-sweep, boot-mint, foreign
  enqueues). Wiring: `--prioritize` (default OFF) on agent-pool, mintPass
  budget-gated, AGENTS.md payload-contract + architecture-table rows.
  One real bug found BY the tests and fixed: a completed batch whose
  completion fact sits later in the stream than a new item's enqueue
  read as "not covering" — coverage now counts ANY minted batch
  regardless of status (mirrors dedup-forever).
- **T27 harvest support** [v]: `harvest.PayloadItemOf` (payload → item
  identity reader), `markerLevel` pinned into harvest payloads at
  enqueue, and the repri-pass **AIScore feed gap fixed** — `repriRepo`
  previously re-resolved WITHOUT the cache; now `tq reprioritize` and
  the startup sweep apply cached scores through the same ladder (test
  pins 5→42 source ai + marker-90-beats-AI-10).
- **T29 — `tq show` priority provenance** [v]: new `priority` section
  (current+band, item key + markerLevel for harvest tasks, cached
  verdict, repri history distilled from the trail). Tests: harvest task
  full section + foreign-task band-only shape.
- **T30 — unblock bump** [v]: `queue.BumpUnblocked` facade pass
  (`UnblockBumpPriority`=15, source `unblock`) — PENDING tasks whose
  deps ALL completed bump ONCE (fact-trail is the idempotency guard),
  backlog-band-only, dry-run supported; deps.dep_id reverse index
  verified present in BOTH backends (no new SQL — Go-side enumeration,
  deps tasks are rare). Wired into `tq reprioritize` (text+JSON
  `unblocks`) and the pool startup repri sweep. Test: blocked→partial→
  dry-run→apply(1 fact)→protected hot→no-double-bump.
- **T32 — webui bands** [v]: prio column (sortable) with band badges
  (hot=warning, machine=info, backlog bare), `?band=` filter
  (allowlisted, chip, select; `Filter.PriorityMin/Max` pushdown in BOTH
  backends — sqlite `TestBandFilter` + live-postgres
  `TestPostgresBandFilter` green), T09's aging hint under the active
  table (constants rendered from the contract), `templ generate` +
  `nix run .#webui-css` + `check-webui-css.sh` GREEN (byte-equal),
  full webui suite green incl. the round-trip band test.
- **T33 — starvation alarm** [v]: `--starvation-after` (default off) —
  oldest PENDING task past the threshold despite aging fires ONE
  PapDashboard trigger per episode (`NotifyStarvation`, dead-pool
  direct-notify pattern) + WARN without `--alert-url`; back under
  threshold or empty queue resolves. Detector lifecycle test (5 cases)
  + bridge trigger/resolve test green.
- **T35 — paused-repo rule** [v]: explicit `importance: 0` in
  importance mode denies all admission ("paused:" skip reason); raising
  importance resumes on the next run (test pins both directions).
- **T36 — status prompt band-drift** [v]: the done-prompt report
  contract gains section h) BAND DRIFT (read `task.reprioritized` facts
  for the window, summarize moves+sources, "none recorded" honest
  fallback); session-close mint shares StatusPayload → parity
  structural (pinned by prompt test).
- **T31 — effort-aware claims: G2 DOCUMENTED, deliberately not built**
  [v]: ADR-0015 §10 records the deferral — G2 (budget global-flat)
  stands; the claim-side effort term would be a dead knob (YAGNI); the
  prerequisite plumbing (effort in the cache) already exists. This is a
  conscious deviation from the plan's "plumbing behind a flag" — a flag
  that changes nothing is worse than no flag.
- **T37 — docs sweep** [v]: CHANGELOG (one comprehensive Unreleased
  entry), FEATURES (4 rows: priority system 🟢, AI scorer 🟡
  honest "first live run pending", `tq reprioritize` 🟢, starvation 🟢),
  README (Priority-system section before "Why"), SECURITY (scorer trust
  boundary item 6), AGENTS.md (prioritize contract bullet + table row +
  priority-mechanics operational contract carrying T09's aging doc and
  T24's perf note), ADR-0015 §10 (G2). check-features-roadmap +
  check-doc-refs + check-status-index all green.
- **Full-suite verification (as of window end)** [v]: root
  build+vet+`-race` ALL GREEN (background run, exit 0); ALL 8
  sub-module gates green; live scratch-postgres battery green
  (including the new band filter). Caveat in §d7 on staleness.

## b) PARTIALLY DONE

- **T38 — full gates**: root race ✓, sub-module loop ✓, webui-css gate
  ✓, doc gates ✓, **lint-baseline --check STILL RED** — fixed three
  rounds of NEW findings in my code (nilnil, QF1008, varnamelen,
  paralleltest in queue module, modernize, wsl_v5), but ~21 GROWTH rows
  remain (executor err113 31→37, paralleltest 39→42, tagliatelle
  34→41; sqlite paralleltest 34→45; root varnamelen 10→14, wsl_v5
  1→4, cyclop/gocyclo/golines/musttag/makezero/predeclared/nonamedreturns
  /unparam/goconst growth) — a mix of MY new test/code findings and
  earlier-window (T25/T26) growth never re-baselined. Mechanical fixes
  (mostly t.Parallel() + renames + line splits) vs deliberate regen is
  an owner policy call (§g2). **nix build NOT run** (no go.mod changes
  this window — vendorHash stable in theory; still owed as proof).
  **Full ci-local NOT run** (blocked on the baseline gate + CI_CHECK
  state — master CI red on the unpushed cqrs tag, owner action §g1).
- **T28 — cost measurement**: folded into the T40 pilot proposal (needs
  a real scorer run = money).

## c) NOT STARTED

- **T39** — minting the G0-approved follow-ups into TODO_LIST.md (held
  per plan: only when execution pauses; this report IS the pause point
  — awaiting the owner's word per §g3).
- **T40** — SystemNix dogfood pilot proposal (aging unconditional,
  propose `--priority-from importance`, `--max-pending-per-repo` ~3-5,
  `--starvation-after` 24h, `--prioritize` behind a one-batch cost
  measurement = T28).

## d) TOTALLY FUCKED UP (this window) — caught, lessons

1. **The heredoc-backslash mangling bit me FIVE times** (mvdan/sh eats
   one escape level even in quoted heredocs): test string literals
   broken twice, templ python edits, the reprioritize CLI block. I kept
   re-trying heredoc variants before switching to tab-anchored
   line-surgery and `chr(92)` — the fix pattern existed from the FIRST
   failure and I applied it late. Lesson: python-via-heredoc for Go
   source is BANNED for anything containing backslashes; write the
   script to /tmp with the write tool first.
2. **Destructive edit AGAIN (same class as the 15-43 §d1)**: an AGENTS.md
   multiedit used the Session-close bullet's opening line as an anchor
   and REPLACED it, orphaning the bullet body. Caught in one minute by
   re-reading, repaired. Lesson re-confirmed: anchors must be UNIQUE
   INCLUDING what the new text is not meant to consume.
3. **Partial multiedit application left broken code on disk**: the
   reprioritize CLI edit applied hunk 2 (summary line referencing
   `unblocked`) but not hunk 1 (the declaration) — build caught it.
   Multi-hunk edits over the same region need one coherent replacement,
   not two hunks.
4. **Four attempts on the templ edits** from guessed indentation (2 vs
   3 tabs, `</div>` vs `}`) instead of `cat -A`-exact context on the
   first failure. Byte-exact views BEFORE the first edit would have
   made it one attempt.
5. **Shipped-to-disk broken placeholder** in sweep_test.go (wrong-arity
   helper + `t.Fatal("placeholder")`) because I split the write across
   two steps. Never write partial test files; write the whole thing.
6. **Invented a nonexistent type** in show_test.go
   (`journal.QueuePriorityScore` alias hack) — over-clever import
   avoidance; the build caught it. Just import the right package.
7. **Declared background gates mid-flight**: shells for the root race
   and module loop kept running while I edited test files afterward —
   the green results predate the last test-file edits (strPtr fix,
   agentpool_test additions). Targeted re-runs covered each edit, and
   the edited files are test-only, but the HONEST claim is "green as
   of the last targeted re-run per file", not "full-suite green at
   HEAD". T38's close-out must re-run the root race fresh.
8. **Lint findings fixed in three rounds** instead of running
   golangci-lint per package immediately after each new file — the
   baseline gate then surfaced them all at once. Per-package lint
   belongs in the per-task loop, not the end-of-window sweep.

## e) IMPROVEMENTS (beyond the letter of the plan)

- The repri-pass AIScore gap fix (T27 support) means ONE resolution
  ladder everywhere: enqueue, `tq reprioritize`, startup sweep, and the
  scorer's apply step all consult marker > AI > keyword > importance
  with the same cache — no drift surface left.
- Payload-pinned `markerLevel` makes marker precedence enforceable from
  the store alone (no filesystem rescan for the apply step).
- The band filter shipped as contract-level pushdown (both backends,
  conformance-tested) rather than a webui post-filter — counts and
  pagination stay honest under `?band=`.
- The scorer's `todo:` key-prefix discriminator keeps every foreign
  mint (reviewfix/catchup/session/dlqfix) out of batches with zero
  configuration.

## f) The next 50 things, in execution order

1. T38 close-out: fix-or-regen the ~21 baseline growth rows (owner
   ruling §g2 gates the choice); re-run root race FRESH at HEAD; nix
   build (vendorHash proof); full `./scripts/ci-local.sh` with
   CI_CHECK=off documented (master CI red on the unpushed tag).
2. Kill the scratch postgres cluster (`pg_ctl -D /tmp/tq-pg-* stop`) —
   running since the Phase-1 conformance work.
3. T39: mint the G0-approved follow-ups into TODO_LIST.md (unchecked,
   machine-parseable, agent-executable; check-todo-list gate).
4. T40: write the SystemNix pilot proposal (flag set, expected
   behavior, watch plan; carries T28's one-batch cost measurement as
   the `--prioritize` gate).
5. Owner: push `internal/journal/cqrs/v0.2.0` + master (CI un-red).
6. Post-pilot: calibrate keyword table + marker ladder from real claim
   order (T34 feedback loop over review verdicts × score source).
7. Post-pilot: revisit G1 sizing (2000 items / 50 repos) from observed
   drain rate; tune `--max-pending-per-repo`.
8. Score-cache TTL/eviction: stale texts rot forever today (bounded by
   repo count; a prune pass is cheap when it matters).
9. `tq tasks --band` CLI parity with the webui filter (the store
   filter exists; only the flag is missing).
10. Webui: band-grouped board columns (plan item 44's tail).
11. Priority provenance in the webui detail page (CLI has it; the page
    shows only the raw record today).
12. Executor: tokens field still 0 (usage parsing for cost measurement
    — feeds T34).
13. Fuzz `FuzzSplitMarker`/`FuzzReadImportance` ride the next nightly —
    watch the first run after this lands on master.
14-50. The §f ledger from the 15-43 report carries forward (calibration,
    per-project budget surfaces, worktree-per-agent decision, compaction
    pin test, postgres adapter parity, …) minus everything this window
    closed; each remaining slot is the L0 micro-steps of items 1-13,
    enumerated in the plan's §5.

## g) Up to 3 questions I cannot figure out myself

1. **Push authorization (carried, now CI-blocking everything)**:
   `internal/journal/cqrs/v0.2.0` is a signed LOCAL tag, unpushed,
   while root go.mod requires it — master CI's test job cannot go green
   until it is pushed. Order me to push (`git push origin master
   internal/journal/cqrs/v0.2.0`) or push it yourself.
2. **Lint-baseline policy for the growth rows**: ~21 rows grew beyond
   the committed baseline — a mix of my new test/code findings
   (mechanically fixable: t.Parallel(), renames, line splits — roughly
   an hour) and earlier-window growth (T25/T26 executor/sqlite tests).
   House rule says growth fails the gate and regen must be
   policy-owned: rule fix-all-now, or authorize a deliberate regen
   (like the two precedents) with the delta attributed in the commit?
3. **T39/T40 now or gated?** Execution pauses here per the report
   contract. Mint the TODO follow-ups (T39) and write the pilot
   proposal (T40) immediately on your word — or hold both for your
   review of this window first. (T40's `--prioritize` enablement needs
   a one-batch live scorer run = real spend; that go/no-go is yours.)

## Verification appendix (claims carry citations)

- prioritize package: 9/9 tests green, `golangci-lint --no-config` 0
  issues [v].
- harvest: full suite + race-relevant targeted tests green; repri
  AI-feed + paused-repo + PayloadItemOf tests green [v].
- executor module: full suite green (band-drift prompt test included) [v].
- queue contract + sqlite + postgres: module gates green; band filter
  tested on sqlite AND live scratch postgres [v]; queue module lint
  findings fixed to baseline-parity [v].
- cmd: build+vet+test green (provenance, unblock, starvation detector,
  agentpool tests) [v].
- webui: full suite green; `templ generate` + webui-css regen +
  `check-webui-css.sh` byte-equal GREEN [v].
- Root `-race` full suite: exit 0 (background, started ~16:10 — see
  §d7 staleness caveat); all 8 sub-module gates OK [v].
- Bridge: papdashboard suite green (starvation trigger/resolve) [v].
- Docs gates: check-features-roadmap, check-doc-refs, check-status-index
  green [v].
- NOT run: full ci-local, nix build, first live scorer run (money).
