# Status Report — 2026-09-10 00:41: Backlog Execution & Gate Hardening

**Session**: continuation of the 2026-09-09 modularization session. Charter:
execute the 23:47 report's 50-item backlog (minus the four owner-gated
decisions) until everything works. **Final state: `ALL CI GATES GREEN` on the
exact tree (full `scripts/ci-local.sh` pass: vet, build, windows cross-compile,
-race suite, all 7 sub-module isolation gates, go.mod health, gofmt, smokes
incl. the new release-gates fixture smoke, doc/todo/features gates, nix build,
nix flake check with the new version-sync check).**

## TL;DR

19 of the 50 backlog items executed (18 closed, 1 closed-by-routing); 15 more
routed into TODO_LIST as live pool food; 6 owner decisions parked as BLOCKED;
2 design ideas routed to ROADMAP. Two REAL latent defects found and fixed on
the way (the release allowlist could not match nested modules — it would have
false-positived the real tree at the next release; `nix flake check
--all-systems` was broken on darwin eval). One flake root-caused to the
kernel, not the code. One honesty gap closed (HTTPExecutor: documented
FULLY_FUNCTIONAL, had zero tests — now tested).

## a) FULLY DONE

1. **P2-9 — ETXTBSY flake**: root-caused to a kernel 7.2.3 anomaly, NOT test
   code. Standalone repro (write-0755-then-exec, 32 goroutines): 630/6400
   failures with zero writers; inotify showed exactly one close_write per
   stub (creation); it reproduces on tmpfs AND btrfs; temp+rename does NOT
   fix it (534 failures); pure exec-churn with NO rewrites still fails
   (4/6400); strace makes it vanish (heisenbug). Fix: `execWithTransientRetry`
   in `runAgent` + `AgentVersion` (ETXTBSY-only, 2 retries, 50/100ms;
   `internal/executor/agent.go`), pinned by `TestExecWithTransientRetry`.
   The previously-flaky Agent|Review|Status set: **40x green**.
2. **P2-7 — release-gate smoke** (found a real bug): the sibling-replace
   allowlist regex `[a-z0-9-]+` could not match nested `internal/queue/sqlite`
   — release.sh would have DIED on the real tree, while the internal-require
   tag check silently SKIPPED the nested module (the vacuous-gate class,
   third occurrence). Gates extracted to `scripts/lib/release-gates.sh`
   (nesting-tolerant), tested by `scripts/smoke/release-gates.sh` (real
   go.mod + 5 fixture go.mods, positive AND negative), wired into ci.yml +
   ci-local.sh. Also fixed a pre-existing shellcheck SC1007 in release.sh.
3. **P3-17 — all-systems flake check** (found a real bug): `checks.module-eval`
   used `mkIf`, leaving a dangling option on aarch64-darwin —
   `nix flake check --all-systems` failed at EVAL time. Fixed via
   `optionalAttrs` (all four checks now one whole-value definition).
   `nix flake check --all-systems --no-build`: all checks passed.
4. **P1-3 — dead-export re-derive + prune**: methodology corrected first
   (`rg -w` undercounts: NewSink/NewCommandExecutor are suffix references —
   with substring matching most of the arch-review's "~15 dead exports" are
   ALIVE). Pruned the 3 deprecated aliases (CrushPayload, RenderCrushPayload,
   TaskTypeCrush). Kept CanTransitionTo/ExpBackoff (documented domain API /
   default backoff). HTTPExecutor: kept AND given its missing test suite
   (status classification incl. permanent-vs-transient, wire envelope,
   empty-payload JSON validity, malformed-URL permanence) — FEATURES claimed
   FULLY_FUNCTIONAL with zero tests.
5. **P2-6 — cmdAgentPool decomposition**: main.go 2144 → 1864 lines. New
   `cmd/tq/agentpool.go`: `agentPoolOptions` + `parseAgentPoolOptions`
   (flags, --config merge, env fallbacks, validation),
   `harvestConfigFromOptions` (repo-timeout/interval ladders),
   `registerAgentExecutors` (shared with cmdWorker — kills the registration
   duplication), `printAgentPoolBanner` (incl. the agent-binary probe).
   Verified live: enqueue→worker --once→stats AND `agent-pool --once` (banner,
   probe, tick, drain, exit 0).
6. **P2-12 — nix run .#test**: now runs the FULL multi-module suite (root +
   all 7 internal/* modules, disk-derived loop, GOWORK=off, GOEXPERIMENT
   pinned); verified green end-to-end. Hermetic `checks.test` deliberately
   stays root-scope (sandbox vendors only root's dep graph; queue/postgres
   needs pgx).
7. **P2-8 + P3-31 + P3-43 — CI parity**: `scripts/check-go-mods.sh` (portable
   replaces, pinned internal requires, toolchain alignment across all 8
   go.mod files, `go mod verify` per module) — one script, wired into BOTH
   ci.yml and ci-local.sh. Shellcheck-clean.
8. **P3-44 — conformance suite parity**: name-level diff found 6 semantic
   gaps in the Postgres battery: dedup keys, watermark monotonic roundtrip,
   head seq, lease-expiry reclaim, exactly-once concurrent claims (the SKIP
   LOCKED differentiator), LIKE-metacharacter escaping. All six ADDED and
   verified against a **live Postgres 16** (disposable docker container on
   :5433) — not just name-diffed. Battery: 16 subtests green.
9. **P3-38 — version drift gate**: new `checks.version-sync` flake check —
   the nix-built `tq version` output must equal the flake's own version attr
   (kills the 0.1.0-binary-from-0.2.0-flake class at gate level).
10. **P2-14 — contract assertions**: `var _ queue.Store = (*Store)(nil)` in
    both drivers (sqlite.go, postgres.go).
11. **P2-15 — naming sweep**: 3 stale `SQLiteStore` comment refs fixed in
    postgres.go (now `sqlite.Store`). mirror.go KEPT (name is honest: it
    encodes the ADR-0007 mirroring maintenance contract, and its header
    documents it). postgres.Open signature untouched (API churn on a tagged
    module with only test call sites — not justified).
12. **P2-11 — diagrams**: new `docs/architecture-understanding/
    2026-09-10_module-structure.d2` + rendered SVG (8 modules, contract
    hexagon, two driver cylinders, postgres ROADMAP note); the two round-1
    diagrams annotated SUPERSEDED in-line.
13. **P3-16 — flake app descriptions**: meta.description on default, test,
    lint, fmt (webui-css already had one).
14. **P3 docs batch — 36/37/40/42/45/48**: README module map + "Picking a
    store backend" section + disk-derived dev loop (+ `nix run .#test`);
    FEATURES row for the ADR-0012 backend split; queue contract
    package-doc rewrite (it said "the sqlite Store is the embedded default"
    — pre-split language); release.sh header now documents the sub-module
    version-bump dance; the TQ_TEST_POSTGRES docker one-liner sits next to
    the CI conformance job.
15. **P2-10 — docs-health HARVEST** (skill-loaded): the 23:47 backlog
    re-verified item-by-item against the tree, then routed: 15 actionable
    items → TODO_LIST (live pool food), 6 owner decisions → the BLOCKED
    section (push, consumer, interfaces, Postgres timing, Dependabot,
    release retrospective), 2 design ideas (task data-model review, Store
    interface segregation) → ROADMAP raw ideas. check-todo-list,
    check-features-roadmap, check-doc-refs all green.
16. **P3-34 — doctor**: verified healthy against a scratch DB post-split
    (integrity/WAL/queue/dlq/sweepers/agent-binary all ok).
17. **P3-47 — multi-repo smoke**: green (3 repos, 2 pools, 6/6 completions,
    dedup + pacing + exclusivity proven).
18. **CHANGELOG [Unreleased]**: Fixed (ETXTBSY, release allowlist, darwin
    eval) / Added (conformance battery, version-sync, check-go-mods, nix
    .#test, HTTP tests, assertions, README/docs, decomposition) / Removed
    (the three aliases).
19. **AGENTS.md**: new commands (release-gates smoke, check-go-mods, nix
    .#test) + two gotchas (kernel ETXTBSY anomaly + the substring-matching
    audit lesson).

## b) PARTIALLY DONE

1. **Sub-module tag re-cut NOT yet performed** — identified, blocked on
   bookkeeping order: `internal/{executor,queue,queue/sqlite,queue/postgres}/
   v0.2.0` still point at pre-session commits (888cb77/fe3d948) while their
   module content changed (prunes, assertions, conformance battery, docs).
   All four are LOCAL and unpushed → re-cut is safe (`git tag -d` + re-tag at
   the settled HEAD), and root's requires are replace-resolved in-repo so
   nothing breaks meanwhile. **Do this immediately after this report lands**
   (the report + index commit should be IN the tagged tree), then re-run
   `./scripts/smoke/release-gates.sh` (its require-tag check validates the
   real go.mod against exactly these tags).

## c) NOT STARTED (deliberate — routed, not abandoned)

- The 15 TODO_LIST items filed by the harvest (govulncheck, gosec, dead-export
  script, templ deep-dive, webui dedup, split-brain check, full-core example,
  release checklist doc, version-surface doc, lint slices, ExitCause rename,
  per-module golangci in ci.yml, nix-binary multi-repo smoke, CI timing,
  dogfood drain) — they are live pool food now; the dogfood agent-pool may
  pick them up at any tick.
- The 6 owner-blocked decisions and 2 ROADMAP ideas (see TODO_LIST/ROADMAP).
- Item 50 (release retrospective) — blocked on the release itself.

## d) TOTALLY FUCKED UP (all caught + fixed inside the session; nothing broken now)

1. **My dead-export audit was wrong on the first pass** — `rg -w` word-boundary
   matching silently missed suffixed references (`NewSink`, `NewCommandExecutor`)
   and would have pruned LIVE exports (Sink, HTTPExecutor, CommandExecutor all
   falsely read as dead). Caught by manually tracing the Sink→worker bridge
   before touching anything. Lesson written into AGENTS.md.
2. **Two malformed edit-tool edits** (agent_test.go func line, flake.nix
   webui-css block) that silently swallowed a newline after `{` — both caught
   by immediate post-edit inspection, both repaired before any build.
3. **Conformance closure bug**: the exactly-once goroutine used its own
   `owner` parameter inside its argument expression — caught before compile,
   rewritten with a loop-local owner.
4. **Conformance reclaim subtest stole the wrong task**: a leftover
   priority-250 pending task from the head-seq subtest won the claim —
   diagnosed from the failure shape, fixed with priority 260 + a comment.
5. **task.Task.DedupKey does not exist** (dedup lives on task.New only) — vet
   caught it; subtest simplified to a project-scoped count.
6. **Doc-ref gate caught MY OWN new CHANGELOG cite** (`internal/executor/
   v0.2.0` reads as a path) — full ci-local run one failed on exactly that;
   reworded; second full run ALL GREEN. The guard works on its author too.
7. **Three nix syntax iterations** during the optionalAttrs restructure
   (missing paren, missing attrset semicolon, duplicate `checks` definition) —
   each caught immediately by `nix-instantiate --parse` / `nix fmt` /
   eval error; final structure verified by a full all-systems eval.

## e) WHAT WE SHOULD IMPROVE

1. **Regex gates need fixtures the day they are written** — the release
   allowlist bug is the THIRD vacuous/false-positive regex gate this repo has
   shipped (after the two `grep '\t'` incidents). The release-gates smoke is
   now the pattern: extract the gate into a lib, fixture-test positive AND
   negative. Apply the same to any future regex gate by default.
2. **The ETXTBSY root cause is behavioral, not mechanical** — I proved it is
   kernel-level (no writer fd, fs-independent, strace-sensitive) and that the
   retry absorbs it, but I did NOT identify the kernel mechanism. If it
   recurs on other hosts or other kernels, report upstream; the executor
   retry is the correct containment either way.
3. **Suite-parity diffing should be a script** — the sqlite-vs-postgres
   name-level diff was manual; the planned dead-export/audit-script TODO item
   could grow a conformance-diff mode cheaply.
4. **Full ci-local is ~8 minutes** — fine, but the nix flake check step now
   builds more checks (version-sync realizes the whole package). Watch CI
   timing (TODO item filed).

## f) UP TO 50 THINGS TO GET DONE NEXT

Everything actionable now lives in TODO_LIST.md (15 items, pool-executable)
and the two ROADMAP ideas — not duplicated here per docs-health routing
(dupe lists drift). Owner-gated: the 6 BLOCKED decisions in TODO_LIST.

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **Push authorization** (carried, now larger): ~40 commits + 8 local tags,
   all gates green on the exact tree. Push now, or hold for a release moment?
   (If held: the 4 stale sub-module tags get re-cut locally either way.)
2. **`internal/consumer` — wire or delete?** (carried, unchanged: zero
   production importers; ADR-0009 dispatcher; the ADR outcome should be
   written either way.)
3. **Postgres CLI wiring for v0.3?** (carried: gives queue/postgres its
   consumer; root re-adds pgx; should sequence BEFORE any public-API
   promotion so the driver story ships complete.)

---

*Report by Crush (glm-5.3-flash), 2026-09-10 00:41 CEST · session commits
under the auto-daemon's heuristic batch commits from `533f5bc` through
`587602a` · final full gate: `ALL CI GATES GREEN` on `587602a` + this report.
Markdown per the established user instruction (status-report skill default is
HTML — override flagged, consistent with the 23:21/23:47 reports).*
