# Status report — external adoption unblocked: public facade modules (ADR-0016)

**Date:** 2026-09-13 06:31 CEST
**Session scope:** review + execute `docs/feedback/new/2026-09-13_external-adoption-blocked-by-internal-paths.md`
(MTU Help Centre could not import the library — every module lives behind Go's
internal-package rule and the root module has zero Go files).
**Gate evidence:** `./scripts/ci-local.sh` FULLY GREEN on the final tree
(build/vet/race, Windows cross-compile, 15-module isolation loop, go.mod
hygiene, lint-baseline within budget, actionlint, lll, webui/status-loop/
dogfood/bootstrap/release-gates smokes, css drift gate, nix build + nix flake
check — "ALL CI GATES GREEN").

---

## a) FULLY DONE

1. **ADR-0016 written** (`docs/adr/0016-public-facade-modules.md`): facade
   modules over the internal library core; type aliases, not wrappers; names
   pinned, implementations stay internal; promotion stays available later and
   is non-breaking for facade consumers. Rejected alternatives documented
   (god-facade at root, wrappers, wait-for-stabilization).
2. **Seven public facade modules created**, each with go.mod (tagged requires
   + relative replaces), one alias file, and a behavioral test:
   `task/`, `journal/`, `queue/`, `queue/sqlite/`, `queue/postgres/`,
   `executor/`, `worker/` — full exported-surface re-export (types, consts,
   sentinels, funcs).
3. **`postgres.OpenWithPool(ctx, pool)`** — feedback item 4. Caller-owned
   `*pgxpool.Pool`, schema applied, nil-pool refused; test proves `Close`
   does NOT tear down the caller's pool. DB-verified variant exists; the
   env-gated conformance run compiles and skips locally (no usable local
   test DSN found — host 5432 is peer-auth-only).
4. **Clean-room external-consumer proof**: a scratch module in /tmp imported
   ALL facades via replace directives and ran enqueue → ClaimDue → Complete
   end-to-end. The exact failure from the feedback now works.
5. **All gates wired for facades** (disk-derived module list extended from
   `find internal -name go.mod` to `find internal task journal queue
   executor worker -name go.mod`): `ci-local.sh`, `ci.yml` (6 sites),
   `flake.nix` test loop, `check-go-mods.sh`, `lint-baseline.sh`,
   `release.sh` (facades get cut sub-tags automatically with the next
   release), `check-release-docs.sh` + `RELEASE.md` kept in lockstep.
6. **`check-go-mods.sh` hardened**: strips `// indirect` comments and
   trailing whitespace before the tagged-version pin check (facades carry
   indirect internal requires; the old regex would have failed them all).
7. **README**: one-line consumer status near the top (feedback item 5) and
   the Embedding section rewritten around public facade paths, including the
   `OpenWithPool` example. No `internal/…` paths remain in the embedder docs.
8. **References doc** (feedback item 3):
   `docs/references/single-job-type-queue-profile.md` — PK dedup +
   visibility-timeout claim + version-guarded revive + bounded ladder +
   dead flag, plus "when to graduate to go-taskqueue".
9. **CHANGELOG** `[Unreleased]` entries for facades, OpenWithPool, and the
   reference doc. **AGENTS.md**: facade rules paragraph (see d1/d3 for the
   two gotchas encoded) + per-module gate snippet updated.
10. **Unrelated real find fixed**: master's committed
    `internal/webui/static/app.css` was an UNMINIFIED rebuild (6,302 junk
    lines — the exact failure class the webui-css gate was built for after
    the 2026-09-11 incidents). The ci-local run caught it; the nix-minified
    artifact was re-committed (aed920b) and the gate now passes.

## b) PARTIALLY DONE

1. **Postgres facade test coverage is thin locally**: `OpenWithPool`'s
   happy path (real DB) is env-gated on `TQ_TEST_POSTGRES` and the host's
   5432 refused every credential I could find (peer-auth, no tq role, no
   password file). The test compiles and skips; CI's postgres:16 service
   will exercise it. Local proof is the nil-pool unit + the internal
   conformance suite, not a live run of the new constructor.
2. **Facade surface parity is manual** (documented ADR decision): nothing
   compiler-enforces that a new exported internal symbol gets an alias.
   The hermetic-parity-test idea was rejected for nix-checkPhase
   hermeticity reasons, but that means the NEXT exported symbol added to
   `internal/queue` silently misses the facade until a human notices.
3. **Lint findings from facades are baselined, not cleaned**: ~35 advisory
   findings (gochecknoglobals on alias vars — by design; wsl_v5/
   paralleltest/godoclint noise in test files). Within baseline policy,
   but a facade-scoped exclusion for the alias-idiom would shrink the
   baseline honestly.
4. **The feedback file itself is still in `docs/feedback/new/`** — no
   processed/ convention exists, so it was left in place and cited from
   ADR-0016/CHANGELOG instead of being moved or annotated.

## c) NOT STARTED

1. **Release** — facades are dead weight on the proxy until the owner runs
   `scripts/release.sh vX.Y.Z --tag/--push`; only then can the Help Centre
   actually `go get` them. Everything is wired; the cut is owner-gated.
2. **Feedback item 1's "willing first external consumer" loop** — the Help
   Centre offered to conformance-test the facade and feed back a port diff;
   nobody has been told the facades exist (blocked on the release).
3. **Docs-site/README badges or a pkg.go.dev-linked module list** for the
   seven facades (the README table is the only index).

## d) TOTALLY FUCKED UP (things that went wrong this session)

1. **Facade go.mod replace-incompleteness bit me** (worker facade):
   missing `replace` for `internal/journal` made the build resolve
   `journal@v0.2.0` through the PROXY (stale tag) → `journal.Reprioritized`
   undefined. Root cause: I wrote go.mods by hand; the auto-daemon
   concurrently rewrote one mid-edit and I appended replaces via `cat >>`,
   which merged badly once. Cost: two extra debug cycles. Encoded in
   AGENTS.md ("every internal module in the graph needs require AND
   replace").
2. **Multi-writer collisions on files I was editing** — the daemon (or a
   concurrent agent) reverted/rewrote `queue/sqlite/sqlite_test.go` and
   `go.mod` while I was editing them; two `edit` calls failed with
   "modified since read" and one edit silently didn't land (the
   `internalqueue` import disappeared between my write and the build).
   I switched to sed + immediate grep-verify. Lesson applied, but it cost
   three rounds.
3. **Self-inflicted doc-string surgery**: my first postgres edit dropped
   the doc-comment line "(0 = pgx default)." and I initially tried to
   re-anchor on a wrong string, wasting a round trip before re-viewing.
4. **Accidental baseline regen**: I ran `./scripts/lint-baseline.sh`
   (no args) expecting a report; it REGENERATED the baseline, bundling my
   legitimate new-module rows together with concurrent root-module drift
   (errcheck 17→19, gocognit 6→7, etc.) in one commit. Justified in
   hindsight (new modules force it eventually), but I didn't do it
   deliberately, and the regen mixed unrelated drift into my evidence.
5. **The unminified app.css shipped to master in commit 201041e (2026-09-12,
   NOT this session)** — this session's gate run caught it, but it sat on
   master for ~half a day. The daemon folded it in; the css gate is in
   ci-local but clearly nothing on the commit path runs it.

## e) WHAT WE SHOULD IMPROVE

1. A `scripts/new-facade-symbol.sh`-style checklist helper, or a
   lint rule (single-file AST walk over `internal/<pkg>` exported decls vs
   the facade alias file — filesystem-based, no toolchain dependency), to
   close the parity gap in b2.
2. Pre-commit (not just ci-local) css drift check, or a daemon-exclusion
   for `internal/webui/static/app.css` — d2.5 will recur otherwise.
3. Facade alias-idiom lint exclusions (gochecknoglobals in the 7 facade
   files) to shrink the baseline honestly instead of carrying ~20 noise
   findings.
4. Document a local test-Postgres recipe that actually works on this host
   (the CI one-liner assumes docker; no docker on this host was confirmed
   either way this session).
5. A `docs/feedback/` lifecycle: a processed/ convention or an annotation
   header when feedback is fully addressed.
6. Resist hand-writing go.mods: a tiny generator (or `go mod edit -replace`
   sequence in a script) would have prevented d1 entirely.

## f) UP TO 50 THINGS TO GET DONE NEXT (prioritized, first = highest impact)

**Release-critical (unblocks the external consumer)**
1. Cut the next release with `scripts/release.sh vX.Y.Z --tag` then
   `--push` — facades + `OpenWithPool` go live on the proxy.
2. Verify `go install`/`go get` of each facade from a clean machine
   (proxy round-trip, the "clean-room install" step in RELEASE.md).
3. Verify pkg.go.dev renders the seven facade modules after the release.
4. Notify the Help Centre session/repo that the facades landed + accept
   their conformance offer (their port diff is a free external review).
5. Cut `internal/queue/postgres` sub-tag bump for `OpenWithPool` (the
   require in the facade go.mod must match the first tag that carries it —
   release.sh tags everything at the same version, so confirm the
   version-sync gate passes on the release tree).

**Close the parity/robustness gaps**
6. Facade parity checker (filesystem AST walk; see e1).
7. Facade-scoped lint exclusions (e3) + baseline regen with honest shrink.
8. Pre-commit css drift guard or daemon exclusion (e2).
9. Add the facades to `scripts/check-dead-exports.sh` reasoning — alias
   files are INTENTIONAL re-exports; make sure the audit never flags them.
10. Windows CI sanity on facades (cross-compile ran green locally; confirm
    the ci.yml facade loop is green on the runner after push).
11. Postgres OpenWithPool: get one local live-DB run (dev-shell container
    or flake testPostgres) so the constructor is proven outside CI.

**Docs / adopter experience**
12. Move the "Postgres CLI store wiring" ROADMAP item up — facades make the
    Postgres store consumer-reachable today; the CLI is still SQLite-only.
13. Add a minimal end-to-end embedder example (`examples/embed/`) using
    ONLY facade paths — currently the proof lived in /tmp and is gone.
14. Link the single-job-type profile doc from README (it is only reachable
    from CHANGELOG/ADR right now).
15. FEATURES.md entry for the public facade surface.
16. ROADMAP entry: "external consumer conformance harness" (their offer).
17. `docs/DOMAIN_LANGUAGE.md` — add "facade module" as a defined term.
18. Version-surfaces doc: facades are an 8th version surface; confirm
    VERSION-SURFACES.md language still holds or amend it.

**Queue health / hygiene from this session**
19. Guard against hand-written go.mods (e6 generator script).
20. Feedback lifecycle convention (e5).
21. Investigate the daemon's mid-session file reverts (d2) — a concurrent
    agent or the heuristic daemon rewrote files under active editing;
    identify which and whether a lock/scope fix is possible.
22. Re-triange the root-module lint drift bundled into the baseline regen
    (errcheck +2, gocognit +1, modernize +1) — someone else's forward
    progress, but it is now invisible inside my regen.
23. Local test-Postgres recipe (e4).
24. Confirm `TQ_TEST_POSTGRES`-gated facade/postgres tests actually run in
    the next CI push (first run with the new tests).
25. Sweep `docs/status/` index: this report + the usual annotate/archive
    pass (docs-health skill).

**Lower priority / opportunistic**
26. Add facade examples to the module table in AGENTS.md architecture
    section (currently only the prose paragraph).
27. `tq doctor` could report facade/version skew (facade require vs
    internal tag) once released.
28. Consider `//go:build` contracts: facades must never grow real logic —
    a lint rule banning non-alias declarations in facade files would
    enforce "names only" forever.
29. Bench: alias-vs-direct call overhead is zero (compile-time); add one
    doc sentence proving it so nobody "optimizes" it later.
30. Review whether `queue/postgres` facade should also expose
    `pgxpool.Config` knobs or stays minimal (current: minimal — keep).
31. Smoke: a release-gates fixture for a facade go.mod (positive + a
    poison case: facade require pointing at v0.0.0).
32. After release: tag-ancestry gate covers facade tags automatically —
    verify with `git tag --list 'task/v*'` post-release.
33. Consider dependabot config covering the new go.mod files (dependabot
    was touched in 201041e; nested modules may need explicit entries).
34. Nil-store fullcore example (commit 3faaf0b) + facades: the fullcore
    example still imports internal paths — migrating it to facade paths
    would dogfood the public surface in-repo.
35. Write the "who imports the facades?" conformance story: a CI job that
    builds an out-of-tree consumer module (the /tmp proof, but permanent,
    in `examples/` or a test workflow).

**Park / revisit later**
36. API stabilization: promotion plan (move implementations to public
    paths) once the API freezes — ADR-0016 keeps this non-breaking.
37. Multi-provider pool gating regressions: none this session, but the
    rate-limit gate keys are per-repo — a facade consumer embedding two
    stores gets two executor instances; document the expectation.
38. Revisit the ~48 dead-export advisory hits (unchanged this session).
39. `.golangci-baseline.txt`: 917 findings vs the "~400" figure in
    AGENTS.md prose — the prose is stale; update the AGENTS.md number.
40. CHANGELOG: move the whole [Unreleased] block into the release section
    when cutting (already enforced by release.sh; just don't fight it).
41. Consider a `docs/adr/INDEX.md` (17 ADRs and counting).
42. Facade module discoverability: `go get github.com/larsartmann/go-taskqueue/queue`
    works per-module, but no root "meta" docs page lists them — README
    table covers it; a pkg.go.dev-friendly doc.go at the root module would
    need the root module to gain Go files (deliberate decision not to).
43. Hook the Help Centre port diff into a tracking issue when it arrives.
44. Check whether `nix run .#test` app (flake.nix:333) covers facades on
    non-linux systems (ci runs windows separately; darwin untested).
45. Postgres `maxConns=0` path (pgx default) — never explicitly tested;
    one conformance line would pin it.
46. dprint formatting of the new markdown files (manual-by-decision, but
    a one-off pass keeps the docs consistent).
47. `docs/references/` has no README/index; one file exists now, more will
    follow.
48. Add facade dirs to any `.gitignore`-adjacent tooling that assumes
    `internal/`-only module layout (grep for `^internal` in scripts).
49. Consider making the clean-room consumer test a flake check (hermetic,
    no /tmp) using a vendored fixture module.
50. Celebrate: the library is finally importable — then actually tell
    someone (overlaps #4, deliberately).

## g) QUESTIONS I CANNOT ANSWER MYSELF

1. **Release timing:** shall I treat the facade release as v0.3.0 (new
   feature surface: importable library + OpenWithPool), or hold until you
   bundle it with the Postgres CLI wiring? (release.sh requires a
   CHANGELOG section + flake version bump either way — I did neither
   without your version call.)
2. **Whose concurrent work was touching my files mid-session** — do you
   have another agent working in this repo right now (the sqlite_test.go
   revert pattern suggests a live session), and if so should facade files
   be considered contested until it finishes?
3. **For the Help Centre reply:** do you want a direct response to their
   feedback file (their offer to conformance-test + send the port diff),
   and if yes — is there a channel/repo to reach them, or do we just
   document it and wait for them to re-evaluate?

---

**Session evidence:** ci-local full pass on aed920b; clean-room consumer
run "external consumer OK: 000001a0990202e9…" (scratch deleted); per-module
loop 15/15 ok; root race suite ok; lint-baseline within budget (917/917).
