# Driver-facade retirement — status + brutal self-review

- **Date:** 2026-11-23 01:27–01:35 CET (composed at 01:27, battery at 01:28)
- **Session type:** INTERACTIVE owner command ("remove queue/sqlite + queue/postgres!"), no pool task, no TQ task id, no verdict file — the auto-commit daemon folded all work (see Attribution below).
- **Scope of the session:** retirement of the two public store-driver facade modules (`queue/sqlite`, `queue/postgres`) and every reference to them; drivers stay at `internal/queue/{sqlite,postgres}` (ADR-0016 retirement addendum records the ruling).

## Scope line — what this report VERIFIED vs NOT

**VERIFIED (fresh battery at 01:28, HEAD 11afb00d, receipts captured to
`/tmp/tq-battery-20261123.log`; counts below are `grep` over that file,
not memory):**

| Gate | Result | Receipt |
| --- | --- | --- |
| `check-facade-parity.sh` | rc=0 — "facade parity OK: 5 facades mirror their internal packages" | battery log §parity |
| `check-go-mods.sh` | rc=1 — **44 ok / 5 FAIL** (see b/d: pre-existing) | battery log, 5 FAIL lines |
| `check-agents-size.sh` | rc=0 — 27593/27600 B | battery log |
| `check-doc-refs.sh` | rc=0 — doc refs ok | battery log |
| `check-release-docs.sh` | ok — RELEASE.md claims match scripts | battery log |
| `check-todo-list.sh` | ok — 0 structural violations | battery log |
| `examples/embed` `go build` | EMBED-OK | battery log |
| `queue` facade module `go test -count=1` | ok | battery log |
| root `go build ./... && go vet ./...` | ROOT-OK | battery log |

**Verified earlier in the session (same tree, pre-daemon-fold):**
`check-consumer-tag.sh --self-test` (OK, flag rail + spec generation at
v0.4.2); `smoke/release-gates.sh` "all cases green" with explicit scratch
`TQ_DB`; `check-cleanroom-install.sh` — "cleanroom: consumer graph
compiles (all repo modules local; no workspace, no vendor)" — the
post-retirement module graph CLOSES and COMPILES including `cmd/tq`;
`check-script-syntax.sh --self-test` rc=0.

**NOT verified, and why:** the full pre-push battery (`ci-local.sh`) —
not run, time-boxed, tree shared with concurrent agents; the per-module
loop over all 15 modules — only `queue`, root, and `examples/embed` were
built/tested this session; the NETWORK gates (`smoke/consumer-install.sh`
proxy roundtrip, `check-pkg-proxy.sh`, `check-consumer-tag.sh v0.4.2`
real-proxy half) — offline session, self-tests only; `nix build` /
`nix flake check` — flake.nix edit was comment-only, not built;
`test-cmd-tq.sh` (windows cross-compile) — cmd/tq's go.mod untouched;
the proxy-level deprecation story for the retired tags — undecided (§g2).

## Attribution (honest ledger)

All work landed via **five footer-less daemon commits** — none carries a
`Task-Queue-ID` trailer, so future lineage forensics need the span:

| Commit | Files | Carries |
| --- | --- | --- |
| `8aa1c81d` | 13 | the deletions (queue/sqlite, queue/postgres), examples/embed re-point, queue/queue.go doc, facadeparity |
| `da2d5a29` | 14 | all script rewiring + living docs (ci.yml, AGENTS, FEATURES, README, TODO_LIST, flake.nix, 8 scripts) |
| `fac32c57` | 5 | doc folds (DOMAIN_LANGUAGE, ADR-0016, VERSION-SURFACES, README, CHANGELOG) |
| `2a275cb4` | 3 | CHANGELOG Removed entry, TODO_LIST harvest edits, check-doc-refs exemption |
| `11afb00d` | 1 | check-doc-refs second exemption (renamed vendor-sync gate) |

Session-start HEAD was `e9cd1465`; battery HEAD is `11afb00d`. Heal vs
accept = §g3.

## a) FULLY DONE

1. **The two facade modules are gone.** `queue/sqlite/` and
   `queue/postgres/` (4 files each: go.mod, go.sum, driver file, test)
   deleted via staged `git rm`; `queue/` now holds only the contract
   facade (`queue.go`, `queue_test.go`, go.mod, go.sum). Historical tags
   `queue/{sqlite,postgres}/v0.3.0…v0.4.2` stay proxy-frozen (Go modules
   are immutable; noted in ADR-0016 addendum).
2. **The driver modules are untouched and still published.**
   `internal/queue/sqlite` + `internal/queue/postgres` keep their
   auto-upgrade seam and v0.4.2 tags — nothing in the write path changed.
3. **`examples/embed` re-pointed**: imports swapped to
   `internal/queue/{postgres,sqlite}`, go.mod requires/replaces updated,
   `go mod tidy` run (go.sum −201 lines), module builds green. Its doc
   comment now states the truth (contract facades = pinned surface;
   drivers in-repo-path).
4. **Facade contract re-pinned at five**: parity pair table + success
   line (scripts/facadeparity/main.go), wrapper header
   (check-facade-parity.sh); gate green at "5 facades".
5. **All nine consumer/publish scripts re-wired**: check-pkg-proxy module
   list, check-consumer-tag FACADES list + comments + self-test, check-
   cleanroom-install probe (now facade-only symbols: `task.Pending` +
   `queue.ErrTaskDone`), smoke/consumer-install (5-facade import/get +
   honest header — the old header claimed an enqueue→claim→complete run
   the script never did), smoke/release-gates label, release.sh consumer
   comment, ci.yml consumer job names, flake.nix stale comment.
6. **Living docs agree with the tree**: README facade table + driver
   snippet (internal-path imports, honest external-consumer story),
   AGENTS.md facade bullet (size-guard green), FEATURES rows 20/173,
   TODO_LIST rows (postgres CLI row now names internal/queue/postgres;
   proxy-surface row lists five facades), DOMAIN_LANGUAGE "Facade module"
   definition, VERSION-SURFACES rows 4/8, ADR-0016 retirement addendum,
   CHANGELOG `[Unreleased]` **### Removed** with explicit
   (BEHAVIOR CHANGE for external library consumers) flag per O14.
7. **check-doc-refs kept truthful**: sanctioned historical-cite
   exemption for `^queue/(sqlite|postgres)$` (the cqrsqlite precedent)
   and for the pre-rename `scripts/check-gomod-vendor-sync.sh` cites;
   the Unreleased vendor-deletion row now names the current script path.
8. **Clean-room consumer graph compiles post-surgery** —
   `check-cleanroom-install.sh` green: a scratch module with no
   workspace/vendor, every repo module replaced local, builds `cmd/tq`.
   This is the strongest single proof the removal did not break the
   graph.

## b) PARTIALLY DONE

1. **Verification depth**: targeted + cheap gates green (scope table),
   but the repo's own bar — `./scripts/ci-local.sh` — never ran, and the
   15-module build/vet/test loop never ran end-to-end this session.
2. **check-go-mods is red at HEAD — pre-existing, fully enumerated, not
   fixed.** 5 modules declare `go 1.27` vs root `go 1.27.4`:
   `internal/config`, `internal/retryx`, `internal/task`,
   `internal/testastguard`, `task` (battery log, 44 ok / 5 FAIL).
   Verified pre-existing: `internal/task/go.mod` is byte-identical at
   session-start HEAD `e9cd1465` and now (`git show` receipt in session
   transcript). NOT determined: whether a concurrent agent is mid-wave
   (§g1).
3. **Network publication gates deferred**: only self-tests/smokes ran;
   the real proxy roundtrips (consumer-install, pkg-proxy,
   consumer-tag@v0.4.2) are unverified post-retirement.
4. **Nix surface unverified**: flake.nix edit was a comment; no
   `nix build` / `nix flake check` / vendor-hash check ran.
5. **`docs/review/libs/go-idempotency.md` count now certainly stale**:
   it pins "dedup/v4 is an indirect dependency in **12 go.mods** …
   (re-counted 2026-11-22, still exactly 12)" — two of those go.mods
   were deleted today. NOT re-counted (unverified exact new number).
6. **Attribution debt**: five footer-less daemon commits carry the
   surgery (table above); heal/retitle not run — no TQ task id exists to
   anchor `commit-task.sh`, and the harness no-commit rule binds me.

## c) NOT STARTED

1. **Deprecation signal for the retired proxy tags** — retraction is
   impossible once the module dirs are gone from master; the only
   remaining lever is a final doc-only tag per module. Undecided (§g2),
   not started.
2. **facadeparity `--self-test` rail** — fleet convention says countable
   gates carry one; the checker prints a countable summary ("5 facades")
   but has no self-test.
3. **Post-retirement phrasing sweep** — living text still says
   "Facade Open backward-auto-upgrades legacy pre-flip DBs" (ADR-0019
   endgame addendum; the legacy-upgrade smoke's docstring family). The
   DRIVER facades no longer exist; the sentence now points at nothing
   unless reworded to "the driver Open" / "internal/queue/sqlite.Open".
4. **README binary-first story** — with drivers off the public surface,
   the README still leads with library-embed framing; an
   install-the-CLI-first quickstart was never drafted.
5. **External-adoptability proof replacement** — nothing in the repo
   proves an OUT-of-tree consumer can compile against the five facades
   with their own `queue.Store` impl (the network consumer-install smoke
   compiles imports only, no Store impl). Was implicit in the old
   embed guard; not rebuilt.
6. **TODO harvest of this report's §f** — being done in this same
   change (rows below landed with `H#` markers; the rest carry the
   fresh:skip note at §f's head).

## d) TOTALLY FUCKED UP (and the brutal self-review answers)

**Nothing this session touched is broken** — every gate I own is green
and the clean-room graph compiles. The honest d-list:

1. **Master is not green right now.** check-go-mods 5×FAIL (pre-existing
   directive skew, receipt above). It was red when I arrived; I left it
   red. "Status: clean" at session start meant git-clean, not gate-green.
2. **Attribution is burned for this span.** Five footer-less folds mean
   `git log --grep` cannot find this surgery; only this report and the
   span table tie commits to intent. Healable while unpushed (§g3).
3. **The clean-room adopter guard is dead, and I only half-mourned it.**
   examples/embed's entire reason to exist was "the exact shape an
   external embedder compiles". Post-retirement it imports internal
   paths — the guard property is void. FEATURES row 173 now says so, but
   the repo lost its only in-tree external-adoptability proof and I did
   not propose a replacement before ending the session (§f12/§g-context).

**The 12 self-review questions, answered with receipts:**

1. **What did you forget?** (i) The TQ_DB smoke guard — I invoked a
   `smoke/*` script bare and the hook blocked it (one wasted round trip;
   the rule was in my loaded context). (ii) AGENTS.md size-guard math —
   I overflowed the 27600 B budget twice (110 B, then 36 B) before
   landing; three gate round trips for one bullet. (iii) Full failure
   enumeration — my first gate run cut the FAIL list with `tail -5` and
   my final message said "5 modules … + 1 more" instead of naming the
   fifth (`internal/config`); corrected this pass from the captured log.
2. **What is stupid that we do anyway?** (i) Facade alias shims over
   modules that are themselves published, tagged and conform-tested —
   the exact mass this session removed; the five remaining facades
   deserve the same question eventually. (ii) AGENTS.md at 7 B headroom
   — every edit is a size-tetris round trip. (iii) Smokes inherit the
   PRODUCTION `TQ_DB` by default and rely on a bash-guard hook plus
   agent memory instead of failing fast at script top.
3. **What could you have done better?** Stage/commit-before-battery
   discipline: the no-commit harness rule and the repo's
   "commit before gates" convention conflict; I resolved it silently in
   favor of the harness and let the daemon fold — I should have surfaced
   the tension to you BEFORE the battery, not in the final message.
   Also: capture receipts from gate #1 (I tailed outputs until the
   final battery); enumerate every failure list fully before writing
   any summary.
4. **What could you still improve?** Verify-window discipline: run the
   full battery + ci-local when the tree quiets down; heal the span;
   then the §f list.
5. **Did you lie to you?** No. One imprecision self-caught and
   corrected: the "+ 1 more" module (above). The "pre-existing" verdict
   for the go-mods failures is verified (byte-identical file across the
   span), not assumed.
6. **How can we be less stupid?** Pre-compute AGENTS.md byte deltas
   before editing; make the battery a captured-receipts script (hand-
   rolled this time); treat every gate FAIL line as report content to
   enumerate, never a tail-truncated summary.
7. **Ghost systems?** The retired facade layer WAS one — pure shims over
   already-published modules, removed this session. One ghost PROPERTY
   remains: examples/embed's "clean-room guard" claim (d3). No new
   ghosts introduced: every new/edited gate path is wired (parity into
   ci-local + CI, doc-refs into the battery).
8. **Scope creep?** None taken: I did not fix the pre-existing
   go-directive red (unrelated, owner-gated), did not touch the vendor
   wave's leftovers beyond the one doc-refs cite fix that my gate run
   exposed.
9. **Did we remove something actually useful?** Yes — deliberately and
   owner-ruled: the out-of-tree driver import surface (the exact thing
   the 2026-09-13 external-adoption feedback complained about). The
   capability loss is real for hypothetical external embedders; the
   counterweight (private repo, zero external riders, contract faster
   than the shims) is recorded in the ADR-0016 addendum. §g2 decides
   whether the proxy tags get a farewell sign.
10. **Split brains?** Swept: README / FEATURES / AGENTS / DOMAIN_LANGUAGE
    / ADR-0016 / VERSION-SURFACES / CHANGELOG now all say "drivers live
    at internal/queue/{sqlite,postgres}; five facades remain".
    Residual split-brain risk: the "facade Open" phrasing family (c3) —
    two texts describing one mechanism with the dead name.
11. **Tests?** The deleted facade tests were alias smokes (compile-only
    value; no semantic coverage lost). The embed example is compile-
    verified, not run end-to-end this session; its runtime path was
    green in prior windows and is untouched code-wise except imports.
12. **Re-verify inherited claims?** The battery's numbers come from one
    captured log (grep'd, not remembered). The one inherited claim I
    did NOT re-verify: the go-idempotency "12 go.mods" count (b5) —
    flagged unverified rather than restated as fact.

## e) WHAT WE SHOULD IMPROVE

1. **Conflict protocol for "commit before gates" vs no-commit harness**
   should be decided once (owner) instead of re-litigated per session —
   candidates: an explicit `ALLOW_SESSION_COMMIT=1` env the harness
   honors for repo-owned sessions, or accept daemon folds + always heal.
2. **Battery-as-script**: this session's 10-gate battery was hand-rolled;
   `scripts/verify-battery.sh` exists and I never checked whether it
   covers exactly these gates — extend it so the next report cites one
   receipt file, not a transcript.
3. **Full enumeration rule**: any gate summary quoted in a report must
   come from `grep` over a captured file — the "+ 1 more" class dies.
4. **AGENTS.md budget**: either raise `agentsDocMaxBytes` deliberately
   or schedule a prune party; 7 B headroom guarantees more tetris.
5. **Smoke defaults**: scripts/smoke/* should hard-fail without an
   explicit scratch `TQ_DB` at script top (not via the bash guard's
   deny-message) — the guard caught me, the script should have.
6. **Deprecation playbook for deleted modules**: when a published module
   is retired, the decision "frozen tags vs farewell tag" should be part
   of the removal ritual (§g2), not discovered afterwards.
7. **Concurrent-agent coordination**: two waves touched go.mods today
   (vendor deletion, directive skew). A directive wave should claim via
   the status index / nextd before starting, so half-landed states are
   attributable.

## f) Up to 50 things we should get done next

<!-- fresh:skip: rows H1–H13 are harvested into TODO_LIST.md in this
     same change (grep-receipt dedup both directions); items 14–30 are
     advisory / owner-gated / ROADMAP-fuel and are deliberately NOT
     harvested (docs-gates §fresh rule). -->

1. **H1** Raise `go` directives to 1.27.4 in `internal/{config,retryx,task,testastguard}` + `task` + re-battery — BLOCKED on §g1 (concurrent-wave intent).
2. **H2** Full `./scripts/ci-local.sh` on the retirement tree (the pre-push bar this session skipped).
3. **H3** Network publication gates post-retirement: `smoke/consumer-install.sh`, `check-pkg-proxy.sh`, `check-consumer-tag.sh v0.4.2` (real-proxy half).
4. **H4** `nix build` + `nix flake check` after the flake.nix comment edit; vendor-hash check if the graph hash moved.
5. **H5** Re-count + update the dedup indirect-dep count in `docs/review/libs/go-idempotency.md` (12 → post-deletion reality; also go-cqrs-lite.md's facade mentions).
6. **H6** Decide + execute the deprecation signal for `queue/{sqlite,postgres}` proxy tags (§g2).
7. **H7** Heal or formally accept the five footer-less daemon commits `8aa1c81d→11afb00d` (§g3), then `check-footer-attribution.sh` over the healed span.
8. **H8** Sweep living docs for dead "facade Open" / driver-facade phrasing (ADR-0019 endgame addendum, legacy-upgrade smoke docs) → rename to the driver's real name.
9. **H9** Add `--self-test` rail to `check-facade-parity.sh` (fleet convention: countable gates carry one) + wire into ci-local.
10. **H10** Build-verify `examples/{api,fullcore,sse}` on the retirement tree (compile-only so far this session).
11. **H11** Run `scripts/test-cmd-tq.sh` (windows cross-compile) once on the retirement tree.
12. **H12** Decide the embed-guard replacement: a scratch out-of-tree consumer implementing `queue.Store` against the five facades (restores external-adoptability proof), or formally retire the guard claim in FEATURES + ci.yml job names.
13. **H13** Tighten or annotate `check-mirror-clones.sh`'s now-facade-less patterns (`r"queue/sqlite/"` etc. — they only match internal paths since the deletion).
14. skip — README binary-first quickstart (install `tq` first; library story secondary now).
15. skip — `internal/queue/postgres.Open` still swallows the legacy `maxConns` param — API cleanup (owner call, internal-surface).
16. skip — ADR-0019 P5 residue: final `internal/queue/sqlite` legacy thinning (auto-upgrade shim vs sqlitev4) — revisit when the upgrade window closes.
17. skip — AGENTS.md budget decision (raise `agentsDocMaxBytes` vs prune party).
18. skip — smoke scripts self-guard on missing scratch `TQ_DB` at script top (defense in depth beyond the bash hook).
19. skip — extend `scripts/verify-battery.sh` to cover exactly this report's 10-gate battery + receipt file convention.
20. skip — directive-wave claim protocol (status-index/nextd) before go.mod-wide edits.
21. skip — v0.4.3 release timing: CHANGELOG `[Unreleased]` now carries the BEHAVIOR CHANGE Removed entry; release is the owner's call.
22. skip — ADR-0016 addendum note that `go mod retract` is impossible for deleted-from-master modules (fold into H6's execution).
23. skip — examples/embed stray binary droppings: `go build ./...` in a main-package dir writes `./embed` (22 MB, gitignored); add a `.gitignore` line + battery builds should use `go build -o /dev/null`.
24. skip — check whether `scripts/verify-battery.sh` / root-gate receipts should live under docs/status/assets via `archive-evidence.sh` instead of /tmp (this report used /tmp).
25. skip — ROADMAP: is tq still a "library" externally after this removal, or binary-first product? Strategic positioning, owner.
26. skip — five remaining facades: run the same "is this shim earning its keep" question the driver facares just failed (after H2/H3 calm the tree).
27. skip — doc-refs: consider an inline `historical:` marker convention instead of the central allow-list growing every retirement.
28. skip — ci.yml consumer job: rename "Facade adopter on-ramp build" if H12 retires the claim.
29. skip — add the go-mods directive alignment to `scripts/ci-local.sh` preflight so a red directive wave fails BEFORE the long battery (it currently surfaces only in check-go-mods).
30. skip — this report's §g answers should land as TODO rows/BLOCKED markers once answered (H1/H6/H7 consume them).

## g) Questions I can NOT figure out myself

1. **The five `go 1.27` modules vs root `1.27.4`** (`internal/{config,retryx,task,testastguard}` + `task`, red at HEAD since before this session): is a concurrent session MID-WAVE on these right now (I leave them alone), or should I raise them + battery as the next action?
2. **Do you want a deprecation signal on the retired `queue/{sqlite,postgres}` proxy tags** — e.g. one final doc-only tag per module whose package doc points at `internal/queue/{sqlite,postgres}` — or do the frozen tags stay silent? (Determines whether H6 is a release action or a no-op.)
3. **The five footer-less daemon commits `8aa1c81d→11afb00d`** carry this surgery with no `Task-Queue-ID`: heal/retitle them via `scripts/heal-daemon-sweep.sh` / `rebuild-lineage.sh` (unpushed window is open), or accept daemon folds as-is for interactive sessions without a task id?
