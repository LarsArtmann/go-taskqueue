# Window 4 — the residue chase vs the churn: §f list closed, receipts still racing foreign sessions

**Date**: 2026-11-24 04:31 CET. **Continuation of**: the window-3 report's §f list
(docs/status/2026-11-23_22-25_…, owner directive "GET SHIT DONE! The WHOLE TODO
LIST!"). **Head at composition**: 4fbbe6d4→a027c021 (unpushed tip; pre-rebuild cite, fork record from the 20261124T034030Z rebuild; pushed base afcd5f7c).
**Stop state**: this report is the WAIT point per the owner's directive.

## a) FULLY DONE

1. **§f1 tick-claims fix verified**: `check-tick-claims.sh --self-test` rc=0
   with the receipt rail included; the fix had landed via sweep 6b3f357a + the
   report via 013f9aea (both pushed).
2. **CI green twice on my pushes**: run 45269939378 (013f9aea) and run
   45293201185 (12m31s, the afcd5f7c wave — ~30 commits: the whole day's
   residue work). The pushed base is protected: heal rails only rewrite
   unpushed spans.
3. **The window-3 §f freshness fixed-point closed**: the 10 flagged rows
   resolved with real receipts (proxy-surface standalone 5 facades/0 skip/0
   fail; heal-daemon-sweep self-test 45 checks; both crush smokes re-run —
   route 14 PASS + supervisor 11 PASS, baseline held exactly; guard-wiring +
   fixture-attribution confirmed already-live in ci-local at :1013/:559) and
   then the WHOLE 50-row §f list annotated (35 cure edits) — docs-gates
   freshness 0 unharvested, repeatedly.
4. **Smoke mock keep-alive desync root-caused and fixed**: the 429-leg battery
   death (run #1's only failure) traced to python mocks answering branches
   without draining request bodies — an HTTP/1.1 keep-alive client desyncs the
   next request into the leftover body bytes (observed: the title-call retry
   arrived as "Bad request syntax ('{…}POST /v1/chat/completions HTTP/1.1')").
   All three crush mocks (pool-supervisor, server-route, server-memory) now
   drain-first; supervisor re-verified standalone 11 PASS.
5. **Lineage-citation healing across three rewrite waves**: fork records
   (arrow-adjacent `old→new` — backticks between the token and the arrow
   break the gate's regex) pasted across the 00-23/00-44/00-51 task reports,
   the README index row, and TODO_LIST rows 2217/2219; dead-sha gate green
   (2069 baselined, 0 new) at three separate points.
6. **The post-batch landed**: io-surface ci-local leg (§f16, verbatim
   ci.yml twin beside check-cleanroom-install); pkg-proxy one named internal
   retry on the HTTP-000000 class (§f15); tick-claims receipt slash negative
   pin (§f19); TQ_SMOKE_KEEP=1 in ratelimit-e2e.sh (§f13); the lockout
   fake-clock suite — 11 tests, package -race + vet green, row 708 TICKED
   (all in the batch commit whose content now rides sweep de7442c8 — see §d1).
7. **docs-gates annotate drained and ratcheted**: the 6 counted findings cured
   (3 art-dupl sweep rows + the planning CORRECTED item + the 00-52 backlog
   strike got verdict markers; the paperclip ask's quad-tilde strike repaired
   — `~~~~text~~~~` parses as two empty strikes leaving the ask path in the
   verdict text, which is what C/path fired on) + the two INFO
   unfinished-phrase leads; `.docs-gates.json` annotate_max ratcheted 144→0.
   Gate CLEAN at 1244 struck items / 0 findings.
8. **AGENTS size-budget reset history moved**: ~115 lines of in-const-comment
   lineage → docs/agents-size-resets.md; the const pruned to a pointer + the
   last reset; TestAgentsDocSizeGuard green via scripts/test-cmd-tq.sh
   (§f21/§f31).
9. **TODO_LIST work**: rows 829 (quiet-host) SUPERSEDED-TICKED with the
   Dependabot-evidence correction; row 694 figures refreshed to the 28,250
   lineage; row 203 NARROWED (the tailer is a signal consumer per ADR-0003 —
   its S2 share is a type re-point, not a dispatcher wire-up); digest-row
   targeting note (2026-12: verify-only duplicate layers first); mints: the
   io-surface row (immediately ticked — the leg landed in the same batch) and
   the fork-record-file row; §f40's archive row minted.
10. **ROADMAP Phase-2 prep section minted** (§f25): buffered push after
    initialized + turn boundary, the flag-form ruling pointer, the
    discover-contract note.
11. **§f11 doctor receipt**: crush-version ok 0.102.0 (proven pin); sole WARN
    the documented v0.4.2 orphaned-tag class (row 91, owner-gated).
12. **§f29 budget-meter measured**: 3.2s standalone — negligible in battery.
13. **Verifications with receipts**: §f14 (cqrs-lite floor `go 1.27` ≤ our
    1.27.4 — the flip story holds), §f23 (d6873805 HEAD-ancestor), §f24
    (empty discover probe is the honest contract), §f26 (zero mcpserver lint
    exclusions), §f27 (row 2182 names the exact prefix-collision shape),
    §f30 (COVERAGE.md:64), §f36 (no open Dependabot PRs), §f38 (0 lint
    entries in mcpserver go.sum), §f39 (`--print-floor` already exists at
    check-toolchain-pins.sh:40), §f44 (mcpserver v0.4.2 resolves on proxy),
    §f45 (version-agreement ok), §f46 (fresh:skip — live TUI receipt covers
    crush-parse; hermetic CI can't run crush).
14. **crush-config lesson filed** (§f47): the lineage-rebuild fork-record
    lesson committed there; the sub-agent etiquette lesson turned out already
    filed (3036120, 2026-11-23 02:27).
15. **lint-leak WRITER identified**: the golangci entries in cmd/tq/go.work.sum
    are written by THIS session's golangci-lint LSP (-mod=mod over cmd/tq on
    every file open — the leak's commit timestamp matches the first cmd/tq
    file read, and it RE-LEAKED after my first prune, mid-session, with no
    other actor). Pruned twice; see §c5/§e4 for the durable fix.

## b) PARTIALLY DONE

1. **The final rc=0 battery receipt**: six chase iterations, each cured an
   honest leg in order (docs-gates freshness → the smoke desync → dead-sha
   waves → attribution grandfathering of 6b3f357a → my own ROADMAP cite
   tripping doc-refs → the lint-leak + its re-leak). Every cure is landed and
   committed, but the green end-to-end run kept being interdicted by
   concurrent-session churn landing NEW footerless sweeps mid-run. The last
   run died at the dep-bump leg on the RE-LEAKED golangci entries (writer:
   this session's LSP — §a15). The receipt is one quiet-tree run away once
   the LSP class is pinned (§c5).
2. **§f40 receipt archival**: blocked on the rc=0 receipt existing; the row is
   minted and harvest-matched so freshness stays green while it waits.
3. **Attribution of the mixed unpushed span**: my commit-task commits for the
   code batch/reset-history/ticks were consumed by the churn — the content
   survives only inside footerless sweeps (883f31b7, 29c7535f, de7442c8 —
   predominantly mine; 2a3fefeb, 8f8151b8 foreign-dominant). The heal tool
   REFUSES the interleaved span (foreign footers are blockers; "never widen
   --from past a blocker"). The doc-gate detector's own contract made the
   immediate failure a GATE failure (doc-refs), not a sweeps-exist failure —
   so the battery is unblocked — but the attribution debt stands until a
   quiet window allows heal-or-grandfather (§f residue item 3).
4. **§f28**: row 708 DONE; rows 709/711 (sort-chip pins) belong to the active
   foreign webui design pass — left to them.
5. **§f6 fork-record-file tooling**: row minted by me; a foreign session is
   implementing it RIGHT NOW (WIP edits to heal-daemon-sweep.sh +
   rebuild-lineage.sh in the working tree — `fork_record_file` now emits an
   absolute path). Not mine to touch; verify + tick when it lands.

## c) NOT STARTED (owner-gated / owned elsewhere — unchanged)

1. Phase 2 channels build (owner serve-per-repo gate; ROADMAP prep section
   now minted).
2. park_task write tool (owner gate, §f6 of window-3).
3. v0.4.3 release train (the foreign campaign's; row 2221 carries the
   verified scope — cmd/tq at-pins GOWORK=off build failure enumerated).
4. The window-3 §g owner questions (lineage-rebuild etiquette, quiet-host
   bar, CI_CHECK friction policy) — still open for the owner.
5. **The golangci-lint LSP -mod=readonly pin**: the durable fix for the
   go.work.sum re-leak lives in crush-config's LSP wiring (workspace-wide
   config), not in this repo. Not filed yet (§f item 5 below).

## d) TOTALLY FUCKED UP (honest losses)

1. **My commit-task commits were consumed by the churn**: the code batch, the
   reset-history move, the ticks — none of the commit-task commits survive in
   history (git log --all --grep finds nothing); the CONTENT is alive via the
   daemon sweeps that raced them. The foreign session's §d1 lesson
   (commit-theft) happened to me in the reverse direction: I committed,
   something rewrote/folded, and the sweeps carry the content footerless.
   Lesson reinforced: commit-task + a live daemon + a live foreign healer is
   a three-writer race on the same refs.
2. **The amend-mine failure mode, live**: my final go.work.sum re-prune was
   swept by the daemon between `git add` and `git commit --amend` — the amend
   re-branded the DAEMON's sweep with my message (4fbbe6d4→a027c021 — pre-rebuild cite, fork record from the 20261124T034030Z rebuild). Content correct,
   attribution muddy; exactly the foreign session's §e1 "atomic amend-HEAD
   guard" row. I did not amend again (each re-amend risks another
   misbranding).
3. **I minted a gate trip**: the ROADMAP Phase-2 section backticked
   `server/discover` and check-doc-refs parsed it as a missing repo PATH —
   the battery died on a failure my own mint created (fixed 066e68c9 by
   unbackticking + "(MCP method name, not a path)").
4. **Three wrong test expectations in the lockout suite**: I asserted the
   docstring's implication instead of the code's branches — Reset KEEPS the
   entry (zeroes count+lock, only idle-prune drops it), the LRA pass is
   lock-blind (only the IDLE pass spares locked entries; under cap pressure
   the bound wins), and my idle-sweep test advanced the clock far enough to
   idle the "fresh" keys. Three red iterations before green.
5. **`nohup … &` battery launch** violated the no-background-& rule (killed
   and relaunched properly; no damage).
6. **The deepest one**: I chased local-battery receipts for ~3h while
   concurrent sessions guaranteed interdiction. The mid-session push (afcd5f7c)
   was the right move — the correct strategy the whole time was push-early,
   let CI validate, and run the final local battery only in a quiet window
   (§e1).

## e) WHAT WE SHOULD IMPROVE

1. **Push-then-validate under churn**: a local full battery (~40 min) is not
   a stable receipt while other sessions land commits every few minutes; CI
   on the pushed tip is. The pre-push-gate convention needs a churn-aware
   variant (battery legs standalone + CI as the end-to-end receipt).
2. **The golangci-lint LSP must run -mod=readonly** (or go.work.sum writes
   need suppressing): every cmd/tq file open from a crush session re-leaks
   the lint dep graph. This is crush-config wiring, not repo code.
3. **Commit-task should detect the theft**: when the named files are absent
   from the resulting commit it says so (good) — next step is auto-invoking
   the heal with the subject it already has (the foreign session's
   commit-theft-detection row now covers the same shape).
4. **Read the code's branch before asserting the docstring** in new tests —
   three of my lockout expectations failed on exactly this.
5. **Never mint backticked tokens that parse as paths** (server/discover);
   the doc-refs gate reads `x/y` in backticks as a file cite.
6. **Quiet-tree detection before battery launch**: a foreign WIP edit (their
   heal-daemon-sweep.sh right now) can red legs that are not mine; a
   pre-flight dirty-tree check with owner attribution would save 40-minute
   cycles.
7. **The daemon-sweep doc gate and the attribution gate disagree on remedy**
   for unpushed interleaved sweeps (heal blocked by blockers; grandfather is
   documented for pushed only) — the gate pair needs an explicit
   "unhealable-unpushed" posture (owner call, §g below).

## f) NEXT (residue first)

1. Pin the golangci-lint LSP to -mod=readonly (crush-config wiring) — the
   re-leak class dies; then ONE quiet-tree battery run yields the rc=0
   receipt at /var/tmp/tq-ci-final.log.
2. Fold the receipt into window-2 report line 47 + the index row + §b here
   (the pre-cited path), tick §f40's archive row after
   scripts/archive-evidence.sh has swallowed the log.
3. Heal-or-grandfather the mixed unpushed sweeps (883f31b7, 29c7535f,
   de7442c8) in the first quiet window — with a dated reason if grandfathered.
4. Verify + tick the §f6 fork-record-file row when the foreign WIP lands
   (their heal-daemon-sweep.sh/rebuild-lineage.sh edits are in-flight).
5. File the LSP-wiring lesson in crush-config (the writer identification is
   the lesson; the fix is the config change). — DONE 2026-11-24: filed
   (references/lessons.md, the -mod=mod re-leak lesson) — the config pin
   itself stays open as the durable fix.
6. Execute the foreign-minted commit-theft auto-heal row (twice-live class —
   I hit it again today, §d2).
7. Execute the atomic amend-mine.sh wrapper row (same report, same class).
8. Rows 709/711 sort-chip pins — coordinate with the active webui pass, don't
   collide.
9. The stale-batch-header advisory warns (nine headers, 2026-09/11-06 era,
   open rows, no landed SHAs) — one annotation sweep.
10. The index-bloat digest ritual (423 live rows > 400) — the standing row;
    2026-12 accrual targets verify-only duplicate layers.
11. UNBLOCKED-OWNER-GATED TODO_LIST warns: five rows need `— BLOCKED:` born
    markers (release wave, commit-theft ruling, amend guard, Store-impl
    hermetic pin, proxy-surface-in-release) — mechanical rephrase batch.
12. Re-run the full lockout suite inside a green battery (it passed
    standalone; it has not been exercised in-sequence yet). <!-- fresh:skip --> one-shot, rides the pending receipt run.
13. Tick-claims receipt rail: the new negative pin exercised once in-battery. <!-- fresh:skip --> one-shot, rides the pending receipt run.
14. Verify the foreign fork-record-file implementation's self-test when it
    lands (their WIP touches the heal tool's rails — the `--self-test` must
    still pin 45 checks).
15. Re-check `.docs-gates.json` annotate_max=0 survives the next foreign docs
    landing (any new struck item without a verdict now trips ci-local —
    intended, but expect the trip).
16. Sweep TODO_LIST for rows the day's verifications ticked implicitly (§f
    items 14/23/24/26/27/30/36/38/39/44/45 all have receipts; the TODO rows
    they belong to deserve the cites).
17. The `tq verdict` for THIS session (the report-without-verdict is a
    half-closeout) — record before the session ends.
18. Push this report + the re-prune commit (FF-safe check first; CI validates
    docs-only deltas).
19. Decide the doc-gate/attribution gate posture for unhealable-unpushed
    sweeps (owner §g3).
20. The v0.4.3 wave coordination: row 2221 is the foreign session's — when
    they cut it, re-run cmd/tq GOWORK=off as the receipt.
21. The dead-sha census row (2219's kin): the foreign 2026-11-19/20 reports
    still carry 11 dangling SHAs — verify-fork-record pairs or dated
    baselines.
22. Keep the check-tick-claims `--receipt` KEEP=20 rotation in mind — the
    /var/tmp receipt family from today (route-f12, supervisor-fix, dg*.out,
    tq-ci-final*.log) is archive-evidence material for the ones cited in
    docs.
23. The lockout suite's wsl_v5/paralleltest lint warns (~66 advisory) — the
    golangci baseline regen decision rides the next lint-baseline window (the
    file count moved 202→? with the new test file).
24. ROADMAP Phase-2 prep section: fold the plan §6 F43 reference into a
    clickable pointer when the channels plan lands in docs/planning.
25. The `digest 2026-12` opening note lives on the standing row — when the
    ritual runs, the accrual goes after verify-only duplicate layers FIRST
    (recorded; do not re-derive).

## g) OWNER QUESTIONS (cannot self-resolve)

1. **Push-then-validate as the churn posture**: may I make "push early, let
   CI be the end-to-end receipt, run the local battery only in quiet windows"
   the sanctioned chase order when concurrent sessions are live? Today it
   contradicts the pre-push-gate habit; under today's churn rate it is the
   only order that terminates.
2. **Mixed-author unhealable sweeps**: when heal is blocker-refused and the
   sweep mixes my content with a foreign session's (both live), do you want
   grandfathering into daemon-sweep-baseline.txt at push time (attribution
   debt accepted, dated reason in the baseline), or should the foreign
   session's own heal own them (my battery stays red meanwhile)?
3. **The golangci-lint LSP writer**: shall I pin the fix in crush-config's
   LSP wiring (`-mod=readonly` for the golangci-lint invocation, or suppress
   its go.work.sum writes), and is the go.work.sum prune commit-message
   naming the LSP the attribution you want recorded per occurrence?
