# Status Report — ADR-0019 endgame verification tail: flake check, smokes, and an auto-upgrade hardening found by them

Report time: 2026-10-05 09:15 CEST · Continuation of
`2026-10-05_00-52_p4-composition-module-landing-gate-repairs.md` (P4 landed;
this window ran the deferred verification tail: nix flake check, webui/api
smokes, the dogfood dry-run, and the full ci-local matrix) · Plan:
`docs/planning/2026-10-03_platform-migration-endgame.md`

Scope: the four unblocked items from the prior report §f1-4. The three §g
owner questions (tag wave, dogfood cutover, P5 cadence) remain UNANSWERED —
nothing owner-gated was touched.

## a) Fully done (each verified green at landing)

1. **vendorHash verified current** despite 55 fleet commits since the P4
   landing (first action after the interruption-gap lesson from §d22 of
   the prior report).
2. **`nix flake check` GREEN — after root-causing a pre-existing eval
   collision** (it was never green; previous windows only ran `nix
   build`). The repo's `apps.<name>.meta.description` leaf-augmentations
   collide with go-nix-helpers' `mkDefault`-wrapped whole-app definitions:
   the priority filter drops the upstream attrset WHOLESALE before the
   leaf merge, so `apps.{default,lint,fmt}.program` had no value. Isolated
   repo-vs-upstream with a minimal consumer flake against the same locked
   helper rev (0fc140f0). Fixed by defining all three apps fully,
   mirroring the upstream programs (binary exe; golangci-lint with the
   tarball Go; treefmt wrapper). treefmt amended. Commits e62581a0 + amend.
3. **webui.sh + api.sh smokes GREEN** against the now-default readmodel
   path (`tq serve`/`tq api` compose through the S4 system/ root) — the
   "not run since P2's flip" debt is paid.
4. **Dogfood dry-run BUILT, GREEN, and wired into ci-local**:
   `scripts/smoke/legacy-serve-upgrade.sh` — seeds an honest pre-flip
   journal, runs `tq serve`, asserts auto-upgrade (exactly ONE
   `<db>.legacy-*.bak`), readmodel default-ON (`<db>.readmodel.db`),
   stats served from the projection, SIGTERM → exit 0 (GracefulClose
   ordering), then post-exit integrity (fact survived, head seq
   unchanged, re-open mints no second backup, migration log line).
   ci-local runs it right after the web UI smoke (c306269a).
5. **REAL BUG found by that smoke and FIXED** (the smoke's first run
   failed — honestly): auto-upgrade verify refused legacy journals that
   lack feature-era tables (deps / watermarks / priority_scores) with a
   confusing `no such table: priority_scores` SQL mismatch →
   restore-and-refuse. The in-place converge tolerated the absence (the
   engine CREATEs missing tables); only verify and the replay copiers
   didn't. Fixed in both paths with one `sourceTableExists` helper:
   absent source table = "nothing to carry" (the existing
   facts_archive/journal_meta doctrine generalized); verification still
   fails if the target holds INVENTED rows. Pinned by
   `TestUpgradeIfNeededToleratesAbsentFeatureTables` + the smoke's
   minimal fixture (the go unit test keeps the full-schema fixture — both
   legacy shapes now covered). AGENTS.md auto-upgrade line gained the
   tolerance clause within budget (15,397/15,400, cd451220). The fix was
   daemon-swept footer-less (ea7866b6) before my attribution commit
   landed — grandfathered in the daemon baseline, see a7.
6. **lint-baseline regenerated deliberately** (145cf459): the 10-03
   nixpkgs roll upgraded golangci-lint to 2.14.0 whose new default
   linters (contextcheck, godoclint, golines, inamedparam, mnd, nilerr,
   nilnil, noctx, paralleltest, usetesting, …) made 21 (module, linter)
   pairs "new classes". ZERO growth in existing classes — the ruler
   changed, not the code; the deliberate-regen clause of the gate's
   message applies. Post-regen `--check`: within baseline (1255 = 1255).
7. **daemon-sweep baseline +18 shas** (fleet had 16 pushed
   footer-less-shipping sweeps accumulate since its last update; 2 are
   this window's migration-fix sweep — pushed ⇒ heal forbidden, so
   grandfathered uniformly). Gate: 1086 baselined, 0 new.
8. **ci.yml SC2086 silenced** (79fe53e3): actionlint's bundled shellcheck
   flagged the gosec job's deliberate unquoted `$GOSEC_EXCLUDES`
   word-splits (09-28 code; the gate had not been reached by any matrix
   since — lint-baseline always died first). Directive + comment,
   behavior identical.
9. **dead-sha baseline +6** (the paperclip-m2 report's and README's
   rebased-away citations): patch-id mappings recovered and recorded in
   the baseline comment for future healers (058dbbe0→f5d773b9,
   2da0bad6→92932f2f, e0f65d93→d76c6437, e3fcd3c7→503d553e; 0b72db95 +
   26723259 have no reachable twin). Zero foreign-file edits.

## b) Partially done

10. **Full ci-local matrix**: run4 reached step 116/~135 — ALL smokes
    green in-matrix for the first time (webui, api lockout, and the new
    legacy-serve-upgrade at log lines 4605/4622) — then died at
    check-webui-css: a daemon sweep (1b76f84a, 08:53) committed a
    mid-regen app.css WHILE run4's comparison ran. The sweep itself
    landed the correct minified css; verified in sync afterwards
    (check-webui-css rc=0, no diff). Run5 launched at the fixed tree to
    capture the tail steps (status-loop, dogfood-once, bootstrap,
    release-gates, version-agreement, guard-wiring, ghost-archives,
    TODO_LIST, FEATURES/ROADMAP) — [verdict filled below].
11. **Transient dead-sha storm observed and correctly NOT chased**: a
    concurrent agent's git surgery (live `tmp_obj_*` in .git/objects,
    1565 unreachable commits materialized mid-session) made ~150 old
    short-sha citations transiently "exist but unreachable". By the time
    run4 reached the dead-sha step the objects were gone again — 0 hits.
    Lesson: dead-sha output is only meaningful on a quiescent .git.

## c) Not started (all owner-gated, unchanged from the prior report)

12. Tag wave (root v0.3.1 + six internal tags; then delete the
    check-go-mods pending-tag block + facade-parity re-pin).
13. P5 legacy deletion + docs truth pass (cqrsqlite deletion, dual-tally
    collapse, ADR/FEATURES/CHANGELOG/TODO_LIST rows, LOC/art-dupl delta).
14. Dogfood cutover (systemd restart against the production journal).

## d) Totallymente fucked up (honesty section)

15. **The daemon beat my migration-fix commit by seconds** — I ran the
    facade-module tests between staging and committing, and the sweep
    took the staged files footer-less (ea7866b6). For multi-file Go
    changes: commit immediately after the OWNING module's gate, run
    neighbor-module verification afterwards.
16. **Recreated the rc-capture trap**: `gate | tail; echo rc=$?` measured
    tail's rc once again (the stash-test in §b11's debugging). The AGENTS
    rule exists; I broke a variant of it within the same session that
    cited it.
17. **First smoke draft shipped a junk python fragment** (a
    `s.ssockname()[1] if False else …` leftover) — caught by re-reading
    before running, but it should never have been written.
18. **Nearly "fixed the fixture" instead of the product**: when the first
    smoke run failed on the missing priority_scores table, the tempting
    move was adding the table "to match the unit test". The honest
    minimal fixture instead exposed a real auto-upgrade bug that the
    canonical full-schema fixture structurally cannot find.
19. **golangci-lint cache clean raced a parallel lint run**
    (/mnt/buildcache unlinkat ENOTEMPTY) — retried once, then skipped the
    clean (the AGENTS regen ritual's clean is a hygiene guard, and a
    concurrent consumer makes it unwinnable).

## e) Improvements made beyond the direct ask

- The endgame's dogfood dry-run is now a first-class ci-local smoke —
  the exact flow the owner will rely on at the (owner-gated) production
  cutover: legacy journal → transparent auto-upgrade → projection-backed
  serve → GracefulClose → journal intact.
- The auto-upgrade safety net now tolerates every legacy schema era, not
  just the last one — older journals (pre-priority_scores) upgrade
  instead of refusing with a SQL-logic-error mismatch.
- The flake's apps are now self-contained (no silent dependency on
  upstream's mkDefault wrapping surviving future helper refactors).

## f) Up to 50 next things (roughly execution order)

1. If run5's tail exposes further foreign races (daemon mid-write
   commits), re-run only the affected step; expect full green on a
   quiescent tree.
2. `nix flake check` re-run at final HEAD (prior green predates the
   migration fix; FOD rebuild expected green — the module loop and
   cmd/tq gate already re-verified the code).
3. Owner answers §g (below) — tag wave, dogfood cutover, P5 cadence.
4. P5 item list: see prior report §f5-13 (cqrsqlite deletion, dual-tally
   collapse, docs truth pass, LOC/art-dupl delta).
5. Upstream (go-nix-helpers) issue candidate: leaf-augmentation silently
   deleting a whole mkDefault app is a footgun worth reporting (verify
   against their repo first per verify-before-filing).
6. Consider teaching heal-daemon-sweep.sh a task-less mode (footered
   marker with a free-form attribution) so windows like this one don't
   have to grandfather their own swept code.
7. The concurrent-agent git surgery (§b11) deserves a docs-health note:
   .git quiescence as a precondition for dead-sha triage.
8. Prior report §f24-34 remain open (paperclip date drift FIXED by its
   window this session; index bloat 194→195 rows still over threshold).

## g) Up to 3 questions (carry-over, still unanswered)

1. **Tag wave authorization** — replace-free cmd/tq and the public
   facades need root v0.3.1 + six internal tags before the migration is
   consumable outside the repo; `go mod verify` in cmd/tq rides the
   pending-tag WARN until then. Autonomous docs/release run this window,
   or stop at green-tree + staged checklist?
2. **Dogfood cutover** — with P1's transparent auto-upgrade (now hardened
   for every schema era), the next systemd restart converges the
   production journal in place. Restart + `tq doctor` verification now?
   Manual replay kept as documented fallback?
3. **P5 deletion cadence** — delete the hand tailer + store-read fallback
   immediately after the dogfood serves green on the projection, or
   short probation with `--read-model=false` as escape hatch first?

## Run5 verdict (to be filled with the ACTUAL result when the matrix finishes)

PENDING at report-draft time — run5 launched at the fixed tree; this
section gets the real rc and step summary before the report is committed.
