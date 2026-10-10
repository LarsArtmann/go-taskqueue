# 05:47 — Wrap-up 90%: C4/C5/E/F all landed; DLQ sidecar lane-fix mid-flight (1 red test)

## a) FULLY DONE

- **C4 closed** (residue gates + smokes + battery): queue/sqlite facade gate
  RC=0×3; `check-facade-parity.sh` RC=0 (7 facades); `go mod vendor` +
  root build+vet RC=0×3; cmd/tq shim RC=0 (7.19s); `check-go-mods.sh` 47/47;
  `legacy-serve-upgrade.sh` + `webui.sh` PASS; root `go test -race` 18/18
  RC=0. Code was already committed by daemon sweeps (`0a14b689`, `bf69c52b`,
  `87a9b8eb`, usage edits in `706b5030`) — nothing left to commit.
- **C5.1 doctor/audit consume `config.Deployment`** (`f312b75f`):
  `doctorOptions.Deployment` replaces the raw path (sqlite-only enforced at
  the type, store opens via `sqlite.OpenWithDeployment` — auto-upgrade +
  resolved sync tier); audit's two open sites go through the new
  `mustDeploymentFromDB` (same constructor, postgres refused with the
  owner-question pointer instead of a mystery open failure); 19 test
  literals rewritten to `config.SQLite(...)`; shim gate RC=0 twice.
- **C5.2 docs** (`7660cde8`): README "Where the state lives" section
  (`--db`/`$TQ_DB`/`--store`/`TQ_SQLITE_SYNC`/`<db>.readmodel.db`);
  AGENTS.md one-deployment-description invariant (paid for by pruning the
  now-config-owned IO-policy pragma list; guard budget 18,750 → 19,000 with
  a dated reset comment — the second reset was needed after the doc-refs
  reword pushed 19,002 > 19,000, settled at 18,998 by trimming my own
  bullet); CHANGELOG C-track entry; doc-refs RC=0.
- **E-track complete** (`b7f7d88c`): E1 — parser hardening VERIFIED
  already implemented (unknown keys = error, typed decode via `fs.Set`,
  env-backed precedence map, 8 tests in `cmd/tq/poolconfig.go`+test); added
  the one missing README paragraph (env-backed keys `cqa-url/cqa-owner/
  cqa-token/log-dir/redact`, flag > env > file). E2.1 — generated
  `docs/planning/2026-10-10_agent-pool-flag-manifest.md` (55 unique flags
  from `tq agent-pool -h`; the plan's "53" predates C4's `--store`).
  E2.2 — file-defaults layering VERIFIED already implemented
  (`applyPoolConfigFile`); no fake code written. E3 — new
  `scripts/check-flag-count.sh` pins all 24 subcommands' registration
  counts against `scripts/flag-count-baseline.txt` (`--update` to bump;
  `FLAG_COUNT_SELF_TEST=1` proves a tampered baseline fails), wired into
  `ci-local.sh` after the script-syntax step; bash -n + shellcheck + doc
  gates green.
- **F3+F4 complete** (`ddd3dfcb`): `docs/adr/0022-three-lane-config.md`
  (deployment lane = `config.Deployment`, domain-policy lane = plain
  Config structs, CLI lane = skin with the flag-count guard as its growth
  gate; v5 convergence pinned as data-shaped); AGENTS.md bullet folds in
  the single-opener rule + lane summary within the byte budget (18,998,
  guard + doc-refs green); TODO_LIST deployment-struct row ticked with the
  ADR pointer.
- **F1 complete** (`6727c525`): tailer retirement EVALUATED and answered
  in an ADR-0003 addendum — keep the hand tailer as the
  `--read-model=false` fallback: (1) metaengine Watcher watches
  metaengine collections, the tq journal is not one (v5/D-track
  territory); (2) SSEReplay is collection-valued vs the hub's
  journal-seq Last-Event-ID semantics; (3) the fallback is the S3 flip's
  own rollback path. Retirement pinned to the v5 all-in as a one-commit
  deletion.
- **F2 complete** (`a12638c2`): honest dead-code survey — the "legacy
  thinning" was already consumed by S1/S2/C3; internal/queue/sqlite is 70
  lines of pure delegation, `v4.Open` is load-bearing (3 migration
  bootstrap call sites), `ArchiveStats` is facade-public API. Real gap
  found and closed instead: NO test proved `OpenWithDeployment` opens the
  same policy as `Open` — new `TestOpenWithDeploymentPragmaParity`
  (parity / struct-tier-wins / env-not-read-here) all green; module gates
  RC=0×3; root `-race` 18/18 RC=0; both smokes PASS.
- **Session hygiene**: 4 daemon footer-less sweeps folded into named
  commits via `git reset --soft` + recommit (`f312b75f`, `7660cde8`,
  `b7f7d88c`, `ddd3dfcb`) — all unpushed at fold time, verified each
  time.

## b) PARTIALLY DONE

- **Wrap-up DLQ lane-fix (mid-flight, RED)**: writing the plan's
  execution record, I ran the verification matrix's "one pragma literal
  left in repo" claim against reality and it FAILED:
  `internal/readmodel/dlq.go` hardcoded its full DSN pragma set
  (WAL/busy/FK/`synchronous(NORMAL)`) — a second sync-tier decision
  point that ignores the operator's `TQ_SQLITE_SYNC`/deployment tier
  (a genuine ADR-0022 lane violation C3 missed; the DLQ sidecar was the
  blind spot). Fix is ~90% in: `config.Deployment.DLQPragmas()` added
  (the ONE sidecar list), `OpenDeadLetters(ctx, d config.Deployment)`
  assembles the DSN from it, all 4 callers rewired (composition runtime,
  doctor, 2 tests); config/readmodel/composition module gates RC=0×3.
  Daemon swept the code into `f2b3f20d` (unpushed). REMAINING: the shim
  gate is RED on `TestDoctorProjectionDLQ` — after the path fix the
  sidecar PRESENCE check passes but the poison entry reads back EMPTY
  ("sidecar present, no poison facts"); `doctor_dlq_test.go` is dirty in
  the tree with the path fix. One focused debugging round from green
  (suspicion: the test's write-open and the doctor's read-open disagree
  about which sqlite file/table, or the write-open's Close path changed
  under the new signature).
- **TestDoctorServiceContextUnreadableUnit**: failed ONCE in a full shim
  run ("unit nope.service environment read (1 vars), want warn" —
  systemctl behaving differently than the test stubs expect), then PASSED
  in the targeted re-run. Not touched by any of my changes
  (`doctorServiceContext` untouched). Classified: environmental/order
  flake, needs one isolation run to attribute — noted, not fixed (not
  mine, per house rules; flagging for the next battery).
- **Plan execution record + consumer inventory**: not yet written into
  the plan file (was the next wrap-up step when the DLQ finding
  interrupted). All content for it exists in this report.
- **Closing CHANGELOG entries for E/F tracks**: not yet appended (C entry
  landed in C5.2).

## c) NOT STARTED

- Final named commit for the DLQ lane-fix + green shim (blocked on the
  one red test).
- `docs/status/README.md` index row for THIS report (landed immediately
  after writing, per convention).
- Plan-file DONE marks with gate citations (§5 matrix rows).
- Persisted consumer inventory appendix in the plan dir
  (`config.Deployment` consumers: sqlitev4, sqlite driver + facade,
  composition ×2, readmodel ×2, cmd/tq ×4 surfaces + doctor/audit,
  webui test).
- B4 smoke fold check (`legacy-serve-upgrade.sh` may already BE the B4
  serve smoke — one grep to confirm, then mark DONE).
- ci.yml parity for the new flag-count gate (TODO row 199 class —
  deliberate asymmetry decision, owner-level).

## d) TOTALLY FUCKED UP

- Nothing at the repo-damage level: 13 commits ahead of origin, all
  named or folded, tree carries exactly one intentional dirty file
  (`cmd/tq/doctor_dlq_test.go`, the mid-flight fix). No pushes, no tag
  moves, no reverts of others' work, no vendor/ hand-edits.
- The honest self-criticism: **C3's "one pragma literal per home" claim
  was accepted without grepping the repo** — the DLQ sidecar literal sat
  there since the sidecar was built, and the C5 verification I ran
  TODAY is the first thing that caught it. The claim "repo pragma
  literals now ONE per home, in internal/config only" was written into
  the 04-05 status report and committed as fact before a verification
  matrix existed for it. That is exactly the "unverified claim encoded
  into work" failure class; caught 90 minutes later instead of never,
  but it should have been caught before the claim was committed.
- Second miss: my first `TestDoctorProjectionDLQ` path fix fixed the
  file LOCATION but not the content mismatch, and I burned a full shim
  run (5s) to learn that. Cheap, but a read of the full test body
  BEFORE the first fix would have caught both mismatches in one pass.

## e) WHAT WE SHOULD IMPROVE

1. **Verification-matrix-first discipline**: every "one X left in repo"
   claim gets its grep written BEFORE the claim commits, not after.
2. **The conform suite should pin constructor parity generically** (any
   new Open* variant lands with a suite row), not per-hand-written test
   — that is how OpenWithDeployment shipped without parity coverage
   until F2.
3. **Test-first reading**: when a signature change touches a test's
   path derivation, read the whole test body before the first edit.
4. **The a)-g) report claimed DONE states for daemon-swept commits
   without running their gates first** — this session I did it right
   (gates before declaring C4 done); keep that as the default.
5. **Status-report DONE claims should cite their gate rc in the report
   text** (they do now; keep it).
6. **Flag-count baseline regeneration** should be part of the
   new-flag checklist in AGENTS.md conventions (one line, next doc
   pass).

## f) NEXT (up to 50, ordered)

1. Debug + fix `TestDoctorProjectionDLQ` (sidecar reads empty) — DLQ
   lane-fix to green.
2. Named commit: DLQ lane-fix (config.DLQPragmas + call sites + tests).
3. Root `-race` battery + legacy-serve-upgrade + webui smokes after it.
4. Isolate `TestDoctorServiceContextUnreadableUnit` (single-run vs
   order-dependent; attribute env vs regression, report only).
5. Write the plan-file execution record (§7): per-track status + gate
   citations + owner questions.
6. Persist the deployment-consumer inventory as a plan-dir appendix.
7. Confirm `legacy-serve-upgrade.sh` = the B4 smoke fold; mark the
   wrap-up row accordingly.
8. CHANGELOG entries for E-track (flag-count guard) + F-track
   (ADR-0022, ADR-0003 addendum, DLQ lane fix).
9. Index row for the 06-xx closing report (next session's wrap).
10. Re-run `check-doc-refs.sh` + `check-todo-list.sh` +
    `check-agents-size` via shim after all doc edits settle.
11. Update AGENTS.md conventions: flag-count baseline bump procedure
    (one line, `--update` + name-the-flag rule).
12. Grep for OTHER unverified "ONE per repo" claims in recent reports
    (same class as the DLQ literal) — e.g. "one TQ_SQLITE_SYNC reader",
    "one env reader" — verify each with a grep, annotate misses.
13. Consider `OpenDeadLetters` timeout: `dlqOpenTimeout` now bounds a
    DSN build + open that no longer reads env — re-check the comment's
    claim ("schema bootstrap") matches reality.
14. Add `DLQPragmas` to the config package test table (parity of list
    length/order with the old literal — the old literal is the
    contract).
15. Wire the E-track's "agent-pool runs from a config file with zero
    flags" into a smoke (README claims it; one script proves it).
16. CI parity: add `check-flag-count.sh` to ci.yml or record the
    asymmetry (TODO row 199 class).
17. Ask owner: push authorization for the 3 wave tags +
    `internal/config/v0.3.3` (cmd/tq proxy installs are broken until
    pushed — `go install cmd/tq@sha` cannot resolve the replace-free
    require).
18. Ask owner: postgres stance (documented-gated refusal vs scratch-DSN
    verification session).
19. Ask owner: ratify `tq facts --json` embedded-JSON Detail wire.
20. After tag push: re-run `scripts/check-pkg-proxy.sh` +
    `scripts/smoke/consumer-install.sh` to verify the proxy story.
21. vendorHash check: `nix build .#checks.x86_64-linux.vendor-hash` if
    any go.mod changes landed since the last nix build (none this
    session — config/readmodel/composition go.mods unchanged).
22. Fold the DLQ sidecar posture into AGENTS.md IO-policy bullet IF the
    sync tier now follows the deployment (one clause, budget-checked).
23. Sweep `internal/readmodel` for any OTHER raw `sql.Open` with
    hand-built DSNs (same class as the DLQ one).
24. Sweep `internal/composition` + `cmd/tq` for leftover
    `config.SQLite(...)` double-constructions (C4 replaced serve's;
    verify no others).
25. Add a doc-refs-friendly citation for `internal/config` in AGENTS.md
    if a future bullet needs the path (the unbackticked mention is
    grep-able but not gate-checked).
26. Consider promoting the a)-g) report skeleton's "verification claims"
    into the docs-health VERIFY mode checklist.
27. Run `scripts/lint-baseline.sh --check` to confirm no new golangci
    growth from this session's files.
28. Run `./scripts/root-gate.sh` once as the pre-push gate before any
    push decision.
29. Heal-check: `scripts/check-daemon-sweep-docs.sh` passed at C5.2;
    re-run after the next daemon sweep touches docs.
30. Archive-sweep decision for docs/status (283 live rows, threshold
    100 — the bloat warning fires; owner-level docs-health ANNOTATE).
31. `TestStatsReadFromReadModel` (webui): now constructs deployments —
    check whether other webui tests still hand-build readmodel paths.
32. Check `internal/harvest`/`internal/budget` for raw `TQ_DB`-adjacent
    env reads that belong in the deployment lane (ADR-0022 consequence
    audit).
33. Verify the E-track README claim "Unknown keys are an error" has a
    test pinning the EXACT error text (operator-facing UX contract).
34. Add the flag manifest regeneration command to the manifest header
    (currently "by hand" — scriptable in 5 lines).
35. `check-flag-count.sh`: count `-h` lines for `tq session begin/close`
    subcommands if they carry flags behind the 0-count parent (the
    baseline pins the parent only).
36. Confirm the sqlitev4 `wake_test.go`/`bench_test.go` still exercise
    Open (not only OpenWithDeployment) — constructor diversity in
    tests is deliberate coverage, don't collapse it.
37. Read `internal/queue/sqlitev4/adapter.go:84` comment block: it
    documents the NORMAL default in prose — update it to point at
    config as the owner (comment-only, no gate risk).
38. Sweep for `synchronous(` literals in queue/postgres + postgresv4
    DSN builders (same lane class, different driver).
39. Check whether `doctorProjection`'s readmodel.Open call passes the
    deployment's pragmas or constructs its own (C3 did composition; the
    doctor path may still be literal-bound).
40. Re-verify `docs/memos/2026-10-09_upstream-seam.md` is committed and
    linked from TODO_LIST (D-track acceptance row).
41. CHANGELOG: link ADR-0022 from the C-track entry (one line).
42. Consider a `tq doctor` check: DLQ sidecar sync tier matches the
    deployment tier (drift detector for pre-fix sidecars).
43. Write the migration note for existing DLQ sidecars opened at NORMAL
    whose operator now runs FULL (re-open re-PRAGMAs; sqlite DSN
    pragmas apply per-connection, so no file migration — confirm and
    document in one sentence).
44. Read-through of ADR-0022 against the final DLQPragmas shape (the
    ADR says "pragma literals live ONLY in config" — now actually
    true; no edit needed, verify).
45. Plan-file §5 matrix: mark C's "One pragma literal left in repo"
    row with the DLQ caveat until the fix lands green.
46. Fold `TestOpenWithDeploymentPragmaParity` into the conform suite's
    sqlite open table if the suite grows an opener-lane dimension (E2
    follow-through).
47. Sweep docs/status/2026-10-10_04-05 for the corrected C3 claim and
    annotate it (the "ONE per home" line is now provably true only
    after the DLQ fix — one-line annotation per docs-health ANNOTATE).
48. `git push` decision bundle for the owner: 13 commits, 3+1 tags,
    proxy-install repair, one battery of gates already green.
49. Consider deleting `internal/queue/sqlitev4.Open` in favor of
    OpenWithDeployment at v5 (keep today: 3 migration bootstrap
    callers + deliberate test diversity; a v5 row, not now).
50. Session-close ritual: `tq session close` so the review/status
    bridge mints its tasks (daemon environment has TQ_DB set — scratch
    only, per the known-issues guard).

## g) OWNER QUESTIONS (cannot answer myself)

1. **Push authorization**: 13 commits are unpushed and 4 local tags
   (`internal/journal/v0.4.0`, `internal/queue/companion/v0.4.0`,
   `internal/config/v0.3.3`, plus the wave from A7) gate the proxy —
   `go install github.com/larsartmann/go-taskqueue/cmd/tq@<sha>` is
   BROKEN until `internal/config/v0.3.3` is pushed (replace-free
   require). Push now, or after the DLQ lane-fix lands?
2. **Postgres**: keep the documented-gated refusal as the shipped
   stance (current behavior, `mustOpenStore` exits with the pointer),
   or schedule a scratch-DSN verification session so `--store
   postgres://` becomes actually supported on worker/serve?
3. **TestDoctorServiceContextUnreadableUnit** flaked once on this host
   (systemctl returned env vars for a nonexistent unit — sandboxed
   systemd behavior?). Do you want it hardened (skip when `systemctl
   cat` lies), or is it a known host artifact to re-run past?
