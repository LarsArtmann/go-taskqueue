# Repair → Verify → Deepen Window Report (2026-11-12_06-45)

**Vehicle:** the owner-approved pareto plan
(`docs/planning/2026-11-12_04-29_repair-verify-deepen-pareto-plan.md`,
committed 74d59b64). This window executed its tiers in order: T01→T09,
T13, T15, T20 landed; T10-T12, T14, T16-T24 remain with the plan unchanged.

## a) WHAT WAS DONE, IN ORDER

1. **T01 — HEAD repair (5e7fc982).** Ground re-checked fresh: upstream
   go-cqrs-lite has **no `queue/v5` tag** (git ls-remote 2026-11-12, ceiling
   `queue/v4.0.3`; the concurrent session's uncommitted delta was only the
   v4-import removal from the same broken migration). Decision per plan M02:
   restore both facades to their exact pre-d77c0f68 v4 content as a forward
   commit. Root build+vet green after `go mod vendor`. HEAD compiles again;
   origin un-reds on push.
2. **T02 — battery with receipts** (dir
   `docs/status/assets/2026-11-12_04-54_repair-battery/`): root
   `go test ./... -race` RC=0 (20 ok pkgs); per-module loop (build/vet/test,
   GOWORK=off, jsonv2) RC=0 (24 ok pkgs); `test-cmd-tq.sh` RC=1 with exactly
   ONE failure — `TestDoctorHealthyEmptyDB` (tag-ancestry warn: v0.4.2
   unreachable, forked lineage) — the documented owner-gated pre-existing
   class, recorded in cmd-tq.log. ok-counts pinned in SUMMARY.txt.
3. **T13 — AGENTS.md prune** (d6b339c1): 27199→27125 B (75 B margin, guard
   OK); three dated task-pointer cites dropped, the glued
   Secrets-redaction/Enqueued-fact bullets split.
4. **T20 — AGENTS.md:NNN cite sweep** (1ff0927f + daemon sweep): all 16 tq
   line-number cites across 7 libs reports de-line-numbered to stable
   section names (they had already drifted: 399→403); the httputil-checkout
   cites verified still accurate and left.
5. **T03 — build-blocked verify lines executed** (8225ed03 + daemon):
   `TestHealth` 12 PASS / 0 FAIL (suite grew 9→12 since 11-08);
   `TestServerTimingHeaderBehindRequestLog` ok 0.009 s; the 3 in-repo
   caveats replaced with executed receipts.
6. **T05 — post-repair gates:** facade-parity OK; conform suites sqlitev4
   4.13 s + postgresv4 PASS; vendor-sync — see T15 below (green at run 5);
   nix vendorHash drifted after the go.sum purge → flake.nix updated with
   `got: sha256-+JIm293yLjRrZGzaZV2d+OEjfyDyL0judEoReC1Ir+Q=`, re-run RC=0;
   doc-refs OK; agents-size OK; status-index ok.
7. **T04 — 22/22 libs-report verify blocks replayed** at repaired HEAD
   (three runner generations; final receipt `verify-blocks-v3.log` +
   reconciliation in SUMMARY.txt): 12 blocks fully green, the remaining 20
   line-fails ALL classified as runner/semantics artifacts (14 expected-empty
   grep/rg rc=1 no-match "passes", 4 upstream-cwd artifacts whose claims were
   verified interactively, 2 bash-joiner artifacts on multi-line continuations
   whose tests ran separately with receipts). **Zero report drift.** The
   replay itself surfaced and fixed real problems: 3 report blocks carried
   `GOFLAGS=-mod=mod go list -m all` (see T15) and 3 carried stale
   broken-window worktree notes — replaced with in-tree executions.
8. **T15 — the lint-leak cluster, closed end-to-end:**
   - **Writer identified:** `scripts/build-webui-css.sh` ran
     `go list -m -mod=mod` at the repo root (vendor-mode Dir workaround),
     materializing the whole module graph into root go.sum — 202 stale
     entries (purged 04d9f772; re-purged c9e205fa after a live repro via a
     stale extracted script). Fixed by resolving module dirs in a throwaway
     /tmp module pinned to the exact repo-graph version (65ef3a8f); verified
     live: `nix run .#webui-css` compiles 164601 B with go.sum gaining zero
     entries.
   - **Second writer class:** bare golangci runs under an ambient
     `-mod=mod` — lint-annotations.sh now pins `GOFLAGS=-mod=readonly`.
   - **Report blocks:** the 3 `GOFLAGS=-mod=mod` verify lines dropped
     (133e3ef4).
   - **Gate upgrades** (check-gomod-vendor-sync.sh): per-module allowlist
     table; go.work.sum coverage; `dev.gaijin.team` added to the domain
     regex; `TQ_VENDOR_SYNC_EXTRA_SUMS` env hook + self-test Case 11 driving
     the leak fixture through the REAL gate path; ci-local switched to the
     shared `--self-test` flag. Self-test now 11 cases, all green; real gate
     green (vendor-sync-5.log).
   - **cmd/tq/go.work.sum deleted:** it carried golangci's full 91 KB dep
     graph (mtime 11-09). Experiments: `go work sync` does NOT prune it;
     deleting it regenerates NOTHING on the next workspace `go` command
     (workspace sums fully covered by member go.sums). All 5 lint-leak
     TODO rows ticked with evidence (go.work.sum row carries the
     agent-ruling marker).
9. **T06 — exhaustruct class consolidated** (38682b7a): the plan's
   options-tail premise was corrected — `Do`'s `opts ...Option` cannot
   silence exhaustruct while the literal omits the optional hook fields.
   Honest form: ONE `exhaustruct.exclude` entry for
   `github.com/larsartmann/go-retry.Config` (every field documented optional
   with safe defaults) + the 3 per-site nolints dropped. Verified: config
   verify OK; zero exhaustruct findings in scoped executor/retryx runs;
   executor 17.9 s / worker 4.8 s / retryx gates green.
10. **T07 — store-busy classifier** (daemon-swept; worker.go + 
    busy_classify_test.go): init-registered errorfamily classifier maps the
    SQLITE_BUSY family to Transient at the source; the worker's own retry
    hook stays busy-only (Classify's fail-open default must never widen the
    retry class). Driver-agnostic string predicate kept deliberately —
    promoting modernc's typed `*sqlite.Error` into the worker would couple
    the pool to the driver against the documented design.
    `TestStoreBusyClassifierRegistered` pins direct/wrapped/non-busy/nil.
11. **T08+T09 — health hardening + duration column** (daemon-swept
    1f6932f2): `writeHealthJSON` now sets `Cache-Control: no-cache`,
    coerces via `health.CoerceValidUTF8` before marshal, and on marshal
    failure logs + serves a static fallback JSON instead of swallowing the
    error; `DurationNanos` threaded through all 8 mkCheck sites with
    per-check timings (database/workers/queue/projection/live-sse measured;
    dlq/follower/panic paths explicit 0 = omitted). Build+vet green;
    TestHealth suite green post-change.

## b) FINDINGS / ISSUES

1. **d77c0f68 committed a non-compiling tree to the pushed branch** — the
   v5 migration switched usage to `v5.*` while no v5 package exists
   in-tree or upstream. Fixed by 5e7fc982; the finish path stays gated on
   an upstream `queue/v5` tag (watchlist item T23).
2. **The lint-leak writer was first-party:** our own CSS build script and
   our own report verify blocks (`GOFLAGS=-mod=mod go list -m all`) —
   not golangci itself, not buildflow. Root go.sum: 202 entries; every
   nested module go.sum polluted (golangci plugin graph). Fixed at all
   three sources; the scan's extended domains then caught the third
   reservoir (cmd/tq/go.work.sum).
3. **`go work sync` does not prune go.work.sum** — deletion + workspace
   regen is the only heal (verified: post-deletion `go list ./...`
   regenerates nothing). Documented in the gate's heal message.
4. **`test-cmd-tq.sh` red at HEAD on `TestDoctorHealthyEmptyDB`**
   (tag-ancestry: v0.4.2 unreachable) — the owner-gated shape decision
   (fixture vs exclusion) keeps every cmd/tq gate red locally; absorbed as
   the one documented class in the battery.
5. **Daemon attribution smear:** 24+ footer-less auto-commits this window;
   several of my staged commits were pre-empted by sweeps (content always
   landed — verified per file via `git log`). The trailer-empty check ran
   after every commit; no foreign contamination found inside my own
   commits.
6. **gopls stale diagnostics** reported `undefined: v5` long after the
   repair — Known Issue honored; CLI gates were the source of truth.
7. **Verify-block replay needs runner rails**: bare `cd` into upstream
   checkouts, expected-empty grep/rg rc=1, `\`-continuations, and rg's PATH
   all bit the mechanical replay. Convention candidate: verify blocks should
   avoid bare `cd` and `-mod=mod`, and prefer `grep` over `rg` (or the
   runner stubs tools).
8. **vendorHash drift is real after go.sum-only changes** in this repo's
   flake shape (the go-modules FOD sees go.sums) — recipe executed
   (got: → flake.nix), re-run green.

## c) WHAT'S NEXT / OPEN QUESTIONS

1. **Remaining plan work, unchanged:** T10 (cqrs `WithPrefetch` on
   TasksPage), T11 (templ-components v1.28 audit/bump + CopyButton
   NoScript), T12 (go-sse ssetest swap), T14 (status-index digest sweep,
   424>400), T16-T19 (health abandonment/Off, WrapOnce collapse, dash
   WithMetrics), T21 (2 could-not-verify closures), T22 (decision memo),
   T23 (upstream watchlist), T24 (TODO_LIST harvest + final battery).
2. **T08 tail:** the 2 pin tests (invalid-UTF-8 store error through the
   HTTP body; marshal-failure fallback writer) — the hardening itself is
   landed and covered by the existing suite; the pins are drafted but did
   not land this window.
3. **Owner §g answers** (previous report): (1) v5 ownership — the DECISION
   side is now resolved (no upstream v5 → v4 restored); the finish path is
   a watchlist item; (2) server_timing — trial re-verified green at this
   HEAD (pin test in battery); keep vs default-on still owner's; (3) §f
   harvest policy — still pending; note the lint-leak cluster self-resolved
   out of TODO_LIST this window, shrinking the harvest surface.
4. **Push:** branch is ~25 commits ahead of origin (HEAD repair, lint-leak
   purge, gate upgrades, docs). Pushing un-reds origin CI — owner ritual.

## d) WHAT WE SHOULD IMPROVE

1. **`TestDoctorHealthyEmptyDB` tag-ancestry:** every cmd/tq gate is red
   locally until the owner picks fixture-vs-exclusion — the TODO row is
   precise; a ruling would green the battery outright.
2. **Daemon vs edit→commit→battery:** a <60 s sweep cannot be outrun by
   multi-minute edit sequences; detailed messages fragment across chore
   commits. An in-flight allowlist (or per-session daemon pause channel) is
   a platform ask already ticketed; this window adds more evidence for it.
3. **vendor-sync gate UX:** it heals then fails on the pre-heal comparison;
   a "healed N files; re-run" line would make the first pass legible.
4. **Verify-block conventions:** pin (via check-script-syntax or a gate)
   that report How-to-verify blocks carry no bare `cd`, no `-mod=mod`, and
   prefer `grep`/explicit-PATH tools — the mechanical replay took three
   runner generations to classify all of this.
5. **golangci advisory noise on webui/health.go** (cyclop 16>12 on
   `evaluate`, contextcheck, varnamelen) is pre-existing baseline — not
   touched this window per the advisory contract.

## e) PROCESS NOTES

- Edit→commit→battery ordering honored; receipts captured under
  `docs/status/assets/2026-11-12_04-54_repair-battery/` (root-race,
  modules, cmd-tq, facade-parity, vendor-sync ×5, conform ×2, doc-refs,
  webui-health, server-timing, verify-blocks v2/v3, SUMMARY).
- TODO_LIST: 5 lint-leak rows ticked with evidence cites; advisory warns
  dropped 22→21.
- AGENTS.md untouched this window beyond T13's prune (no new durable
  convention — the go.work.sum heal lives in the gate itself); budget
  27125/27200, 75 B margin.
- No `rm` (trash used for the stale extracted scripts), no amend/rebase,
  no force-push; TQ_DB never touched (no `tq serve` ran); foreign
  concurrent-session files (ci.yml, flake.lock) were daemon-swept, never
  hand-committed by this window.

## f) NEXT WINDOW ITEMS

1. Land the 2 T08 pin tests (poisoned-UTF-8 store error through the HTTP
   body; marshal-fallback recording writer) + full webui suite green.
2. T10 WithPrefetch on TasksPage (O(1) repeat-window reads) + bench proof.
3. T11 templ-components bump audit (88-ahead upstream) + NoScript adoption.
4. T12 ssetest swap in webui_test.go (4 hand-rolled decode loops).
5. T14 status-index digest sweep (ANNOTATE 2026-09/10 clusters, archive,
   digest row, <400 live).
6. T16-T19 polish tier; T21-T24 closures/memo/watchlist/harvest.
(fresh:skip: items 2-6 mirror the approved plan's remaining tiers verbatim;
nothing new was discovered this window beyond item 1)

## g) QUESTIONS FOR THE OWNER

1. **Push ritual:** ~25 commits ahead (HEAD repair + lint-leak purge + gate
   upgrades). Push now to un-red origin CI, or hold until T10-T24 close?
2. **TestDoctorHealthyEmptyDB shape:** fixture repo vs excluding
   tag-ancestry from that test's worst-status aggregation — either greens
   every cmd/tq gate; which do you want?
3. **server_timing disposition:** stay opt-in (RequestLog-gated, current),
   or flip default-on?
4. **§f harvest policy:** after T24, convert the remaining plan tiers into
   TODO_LIST rows, or keep the plan doc as the single vehicle?
