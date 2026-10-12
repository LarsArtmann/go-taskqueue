# C16 store surface — operator park, claim-time type filter, claim-count pushdown — 2026-10-12 10:42

Session window (owner "READ, UNDERSTAND… execute and verify" continuation of the SUPERB
pareto plan, resumed from the 07-55 WAIT). Scope executed: the C16.1/C16.2/C16.4 STORE
surface across journal → queue → companion → both v4 adapters → worker, conform-pinned.
All commits carry footer `Task-Queue-ID: 20261012-c16-park-forensics`.

## a) FULLY DONE

1. **C16.1 store surface (row 408, store half)** — `journal.Parked` fact type
   ("task.parked", tq-only family beside question-*); `queue.ParkedDetail{reason,
   until_ms, by}`; `Store.Park(ctx, id, reason, until, by)` on the contract with the
   RecordAnswer shape: same-transaction NotBefore move (clamped to now) + task.parked
   fact, PENDING-only (`task.ErrInvalidTransition` on running/terminal,
   `task.ErrNotFound` on unknown), a past until IS the release. Implemented in
   `companion/parks.go`; both v4 adapters delegate; legacy sqlite/postgres get it free
   (pure v4 aliases); facades re-export (`journal.Parked`, `queue.ParkedDetail`) —
   `check-facade-parity.sh` OK (7/7).
2. **C16.2 claim-time executor-type filter (row 409, store+worker)** —
   `Store.ClaimDueTypes(owner, lease, types)`: companion.ClaimDue grew an IN-list on
   `t.type` (dialect-safe: pgq rebind rewrites the generated placeholders); no claim →
   no task.claimed fact → the loop-suspect census stays clean. Worker loop now claims
   via `ClaimDueTypes` with `Registry.Types()` (nil/empty registry = legacy any-type).
   `runExecutor`'s permanent unknown-type classification stays for genuinely-unknown
   types in a full registry.
3. **C16.4 counts pushdown (row 410, store half)** — `Store.ClaimCounts()`:
   `SELECT task_id, COUNT(*) FROM facts WHERE type = 'task.claimed' GROUP BY task_id`
   in companion, exposed by both adapters (StatusCounts family).
4. **C16.3 store half (row 494)** — `Filter.DedupKey` exact-match pushdown in
   companion `listWhere` (rides List AND CountTasks), column `dedup_key`.
5. **Conform pins** — 4 new suite tests registered in the `conformanceTests` table:
   TestParkSuppressesClaimsUntilRelease (park → ErrNoTaskDue → fact present →
   release → claimable), TestParkRefusesNonPendingAndUnknown,
   TestClaimDueTypesFiltersExecutorTypes (sh claimed, agent untouched: pending + zero
   Claimed facts, unfiltered claim still takes it), TestClaimCountsGroupsPerTask
   (requeue cycle → 2 vs 1). All PASS on sqlite via TestStoreConformance.
6. **Worker pin** — TestBareWorkerNeverClaimsUnregisteredTypes: sh-only pool
   completes the sh task and leaves the agent task pending with zero claim facts
   (the 11-01 near-miss, now impossible).
7. **Gates** — builds green in all 7 touched store modules (journal, queue,
   companion, sqlitev4, postgresv4, sqlite, postgres) + both facades; root
   `go build ./... && go vet ./...` green after `go mod vendor`; facade parity green;
   scoped gofmt green.
8. **Daemon-race recovery** — the chunk was swept footer-less into 3 daemon commits
   mid-session (see d1); healed with `heal-daemon-sweep.sh` under the window slug.
   Fork records: `124af4c1→eaf3b1d6`, `acabb451→adde9c52`, `e9a61e9e→dea5794c`,
   `7aa72a2e→aedd7e49` (the last = the gofmt fix, swept separately).

## b) PARTIALLY DONE

1. **C16.1 CLI half** — `tq park <id> --until DUR [--release] [--reason …]` verb not
   written; the store surface has no operator surface yet.
2. **C16.3 CLI halves** — `tq tasks --dedup <key>` flag and `tq audit --journal
   --task-id <id>` filter not wired (Filter.DedupKey exists; journalaudit needs the
   parameter threaded from audit.go).
3. **C16.4 caller flip** — `tq stats` loop-suspect detection still walks the cmd/tq
   claimCount twin; flip to one store.ClaimCounts call + retire the twin + update the
   queue.go ClaimCount doc comment (its art-dupl note about the twin goes stale).
4. **Postgres lane** — postgresv4 compiles but the 3 new conform tests ran only on
   sqlite (no TQ_TEST_POSTGRES instance this session).
5. **Filter serialization surface** — httpapi/webui JSON binding for the new
   DedupKey field unchecked (additive; low risk, unpinned).
6. TODO rows 408/409/410/494 deliberately UNTICKED until the CLI halves land.

## c) NOT STARTED

1. C15.3 SystemNix park policy (needs the `tq park` verb).
2. C15.2 CV canary (still gated on the 07-55 report's Q2 cache repair; the
   local-GOMODCACHE verification idea from that report's next-steps is also untried).
3. Row-48 fast-fail red path (httpapi WorkerFailed-marker poll beside srv.dlq.Count).
4. Row-29 heal span-terminator bug — still open and casting a shadow today (d5).
5. Docs pass: FEATURES rows for BOTH windows (autopsy-yield audit, %B fallback,
   done-preflight default, park/filter/counts), major-prompting EXECUTED numbers,
   CHANGELOG C16 entries (deferred to the CLI-half commit).
6. AGENTS.md smoke-hermeticism fold (budget guard: 19,970/20,000 — skipped on purpose).

## d) TOTALLY FUCKED UP

1. **Edit→commit ordering violated at window scale — again (3rd window running).**
   The entire focused-green chunk (17 files) sat uncommitted across the full companion
   suite, sqlitev4 conform suite, vendor sync, and parity run; the daemon folded it
   into THREE footer-less sweeps. Heal recovered attribution (4 fork records), but
   this is exactly what the stage-early rule exists to prevent. Root cause: treated
   "focused green" as one end-of-chunk monolith instead of committing the built
   store-surface before authoring the conform tests.
2. **Two edit-tool refusals** on conform/suite.go and the sqlitev4 adapter — I had
   only sed/grep'd them; the edit tool requires a View first. Two wasted round trips.
3. **Silent test skip (worst class): green lie.** The first conform run PASSED while
   my four new tests never executed — appended to tests.go but not registered in the
   `conformanceTests` table. Caught only because I grepped the -v output for my test
   NAMES and they were absent. Without that habit this report would claim coverage
   that never ran.
4. **One gofmt miss** (Filter.DedupKey comment alignment) caught at report time; the
   fix itself got daemon-swept within seconds and needed a second heal (aedd7e49).
5. **Heal friction**: the script refused on a dirty worktree (the gofmt diff),
   forcing a stash/heal/pop dance — the worktree-clean precondition is correct but
   the dance cost a cycle.

## e) WHAT WE SHOULD IMPROVE

1. **Commit at true checkpoints**: journal+queue built → commit; companion+adapters
   built → commit; conform green → commit. The daemon race is a WHEN, not an IF.
2. **Register conform tests in the same edit as the test body** — a missing
   conformanceTests row is invisible in green output. Better: a suite self-test that
   reflects over the package's Test* funcs and fails if any is absent from the table
   (kills the silent-skip class permanently).
3. **grep your test names in -v output** whenever a suite passes "too easily" —
   make name-presence a standard verification clause in closeout reports.
4. **Heal script UX**: accept/auto-stash a dirty worktree, and fix the row-29
   [footer-less]+[footer HEAD] span-terminator shape (today's stash dance existed
   only to avoid it).
5. **cmd/tq pin reminder**: when the CLI halves land, the new Store methods exist
   only via the devmod shim until the NEXT release walk (VERSION-SURFACES) bumps the
   pinned tags — proxy installs would break on the verbs otherwise.

## f) Up to 50 things we should get done next

(deduped against open rows; ranked)

1. `tq park` CLI verb + park_test.go (duration parse, refusal copy, --release).
2. Park ceiling decision + validation in the verb (see g1).
3. `tq tasks --dedup` flag wiring + test.
4. `tq audit --journal --task-id` + journalaudit param threading + test.
5. Stats loop-suspect flip to store.ClaimCounts + retire the cmd/tq twin + fix the
   queue.go ClaimCount doc note.
6. parkBy() identity helper (USER → "operator" fallback) shared by park/dismiss.
7. `tq tasks --parked` operator-park distinction (fact join) — after g2 ruling.
8. webui parked badge slice (E22.5) once parks are operator-minted.
9. httpapi Filter binding check for DedupKey (auth-matrix stays green).
10. Conform suite self-registration pin (e2) — reflect over Test* vs table.
11. Row-29 heal span-terminator fix + reproduce the [footer-less]+[footer HEAD] shape.
12. Row-48 api wire-pin fast-fail (WorkerFailed marker poll, ~100ms red).
13. Postgres conform lane for Park/ClaimDueTypes/ClaimCounts (needs g3 answer).
14. C15.3 SystemNix park policy (uses the verb).
15. C15.2 CV local-GOMODCACHE gate run as autonomous Q2 evidence.
16. CHANGELOG [Unreleased] entries for the whole C16 cluster.
17. FEATURES.md rows for both windows' landings.
18. major-prompting-and-batching doc: EXECUTED-with-numbers update.
19. Tick TODO rows 408/409/410/494 when their CLI halves land.
20. Doctor surface: operator-parked tasks named with reason (parked ≠ rate-limit).
21. `tq park` on a RUNNING task: actionable error (wait for claim release vs
    cancel-running) instead of a bare ErrInvalidTransition print.
22. Webui task detail: render task.parked facts in the timeline (operator rulings).
23. ClaimCounts on the webui now-band (replace per-task walks in render.go if hot).
24. Smoke: scratch-journal e2e exercising park → claim refusal → release → complete.
25. AGENTS.md prune + smoke-hermeticism fold (budget-capped; pair with the next
    prune, not a standalone edit).

## g) Questions I can NOT figure out myself

1. **Park ceiling**: cap `tq park --until` at 7d (mirrors ask's maxQuestionTTL,
   re-park to extend — bounded silent stalls) or trust the operator with unbounded
   parks (visibility via `tq tasks --parked` + doctor)? Gates the verb's validation.
2. **Operator-park visibility**: should `tq tasks --parked` (and doctor) distinguish
   operator parks (task.parked fact) from rate-limit/requeue parks in THIS window's
   CLI half, or is the shared not_before surface enough for now (distinction = a
   small fact-join in the parked listing)?
3. **Postgres verification instance**: is there a sanctioned scratch postgres DSN
   (TQ_TEST_POSTGRES) I may run the conform lane against now, or does pg proof of
   Park/ClaimDueTypes/ClaimCounts wait for the D20 owner-verified instance?
   (Standing and still unanswered from the 07-55 report: pool-restart timing (Q1),
   /mnt/buildcache vcs-entry repair permission (Q2), dismissal-tier scope (Q3).)

— WAITING for owner instructions.
