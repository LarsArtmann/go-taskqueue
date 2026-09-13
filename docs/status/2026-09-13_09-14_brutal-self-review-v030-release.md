# Brutal self-review + v0.3.0 release retro — execution session

Point-in-time snapshot of the session that executed the facade-adoption
Pareto plan T0–T14 and cut v0.3.0 (window ~08:00–09:10 CEST, 2026-09-13;
prior session report: `2026-09-13_09-06_facade-adoption-pareto-execution.md`).
Written against the brutal-self-review question set, scoped to THIS
session's run. No new research beyond verifying this session's own
leftovers (scratch dirs, tracking, release notes — all re-checked 09:14).

## a) FULLY DONE

1. **v0.3.0 RELEASED and PROVEN**: 16 tags pushed (root + 8 internal +
   7 facades), proxy serves every module, tag CI green, GitHub Release
   cut with disclosure, clean-room consumer (proxy-only, zero replaces)
   ran enqueue→fail→retry→complete on sqlite AND live Postgres via
   published `postgres.OpenWithPool`.
2. **Pre-release plan tasks T0.5, T3, T6, T7, T8, T10, T11, T12, T13,
   T14** — all executed at green gates (parity gate + negative test; css
   pre-commit guard block+green proven in a clone; live-DB proof; lint
   exclusion + honest 886/111 regen; docs coherence ×5 surfaces;
   new-module.sh + proof; release-gates containment rewrite + fixtures;
   feedback lifecycle README).
3. **Three real bugs found+fixed beyond the plan**, each with a gate or
   live run as the catcher: `Close` tearing down caller-owned pools
   (latent red master since 04:45); missing `PrioritizeExecutor` alias;
   single-line `require` skipping the tag-existence check.
4. **Two release-flow breaks repaired mid-flight**: dead ldflags literal
   grep (flake single-source refactor made it unmatchable — gate was
   silently dead); smoke needing tags on origin mid-window (sub-tags
   pushed early — deviation, see d3).
5. **Owner loop**: 3 questions asked, answered (push-now; gh_issue);
   scope challenge answered from the recorded 08:01 verdict; tracking
   issue #3 + voice-checked reply draft staged in docs/feedback/.
6. **Session hygiene**: status reports indexed (09-06 then this one);
   all doc gates green at close; master synced; scratch dirs verified
   gone; examples/embed go.sum verified tracked.

## b) PARTIALLY DONE

- **T2.4 evidence recording**: proxy/consumer evidence lives in the
  09-06 report; a dedicated release-retro doc was the plan's letter and
  does not exist. pkg.go.dev pages still ungenerated (lazy; requests
  queued) — no TODO row mints the re-check.
- **T14.3 dprint pass**: skipped (tool not on PATH; formatting
  manual-by-decision). Cosmetic only; gates green.

## c) NOT STARTED

- **T5** out-of-tree consumer CI job (post-release by design; T2 covered
  the ground once, nothing guards it per-push).
- **T15** park lot (5 items, unchanged, owner-independent, low value).
- **README → issue #3 cross-link**: README's consumer-status line does
  not point at the tracking issue (the reply draft does).

## d) TOTALLY FUCKED UP (own goals, honestly)

1. **The go-install regression shipped inside the tag.** `go install
   …/cmd/tq@v0.3.0` fails on the root module's replace directives —
   caught by release.sh's clean-room step AFTER `--push` (that step runs
   only in --push mode, post-immutability). Root cause predates this
   session, but I operated the release flow for hours without asking why
   the install proof runs last. The cheap fix — run the clean-room
   install check in GATES mode — was identified in §d of the 09-06
   report but never minted as a TODO row. Sloppy close-out.
2. **Self-inflicted mutation via a "guard" script**: a python heredoc I
   labeled a no-op guard actually INSERTED a broken placeholder line into
   executor/executor.go; emergency-removed one command later. In a
   daemon-swept tree that could have ridden a commit. The lesson
   (never write "guard" scripts that mutate; use the edit tool) was
   already canon — I violated it.
3. **Improvised release-order deviation**: pushed the 15 sub-tags BEFORE
   the root tag (release.sh's design pushes master+root+sub-tags
   together). Justified by the red-master window (smoke needs tags on
   origin), executed on my own judgment under pressure with the owner's
   generic release authorization. It worked; it was also not the
   documented flow, and nothing records it as a sanctioned variant.
4. **First clone test misdiagnosis**: "COMMIT-WENT-THROUGH-BAD" was a
   stale-clone race (daemon hadn't committed the installer yet), not a
   hook failure; I briefly treated my own tool as broken. Retested
   correctly, but the first read was wrong.
5. **T0.4 mechanism hand-wave**: GOPROXY=direct resolved internal
   modules but not facades; correct conclusion (remote tags), but the
   internal-via-local-replace fallback mechanism was never pinned in the
   report — recorded as "the tags work" without the why.

## e) WHAT WE SHOULD IMPROVE (process, from this session only)

- **Move the clean-room `go get` + `go install` check into release.sh
  gates mode.** It is the only step that catches proxy-installability,
  and today it runs after tags are immutable. Highest-value single fix.
- **The release window keeps master red by design** (swept pins +
  not-yet-visible tags fail the smoke on runners). Either teach the
  smoke a release-in-flight state, or make "push sub-tags first" the
  DOCUMENTED step instead of an improvisation.
- **CHANGELOG multi-writer convention**: Unreleased accumulated 4
  `### Fixed` / 3 `### Added` headers from concurrent sessions; I
  consolidated manually. One-block-per-type per session, or a hook.
- **release.sh resume logic** cannot re-tag at a newer HEAD even when
  the diff is bookkeeping-only (I deleted + re-cut the unpushed root tag
  manually). Codify the unpushed-tag re-cut rule.
- **ci-local↔ci.yml parity for new guards**: the examples/embed
  rot-guard landed only in ci-local. New gates should land in both call
  sites in the same change (the check-release-docs pattern exists for
  release.sh; nothing enforces it for ci-local/ci.yml).

## f) NEXT — honest items from this session (not padded)

1. release.sh: clean-room install check in gates mode (e1; ~15min, prevents d1's class)
2. cmd/tq as replace-free module — restores `go install` (TODO row exists; needs ADR)
3. README consumer-status line → link issue #3
4. pkg.go.dev generation re-check ×7 (TODO row; minutes, time-delayed)
5. T5: out-of-tree consumer CI job (also closes the embed ci.yml parity gap)
6. examples/embed build step into ci.yml (parity with ci-local guard)
7. Permanent self-test for scripts/facadeparity (negative case in-repo, not the manual one-shot)
8. Release-window smoke: skip/annotate real-tree check when pins reference unpushed tags
9. CHANGELOG consolidation hook or writer rule (e3)
10. release.sh: sanctioned unpushed-tag re-cut at bookkeeping-only HEAD (e4)
11. Reply draft → actual send (owner action; draft staged)
12. Move helpcentre reply draft into the feedback lifecycle dirs; repoint README (my own d-split-brain)
13. VERSION-SURFACES post-release refresh (column says "at v0.2.0"; values now 0.3.0)
14. T15 park lot (doctor facade-skew; alias-only lint rule; maxConns=0 conformance line; darwin loop check; ROADMAP promotion note)
15. cqrs-lint advisory gate wiring (concurrent session's TODO, still open)
16. Journal-drift audit (concurrent TODO, still open)
17. Plan doc pointer line → this retro (check-doc-refs-safe)
18. Confirm dependabot picks up v0.3.0 pins (entries exist; first bump PR is the proof)
19. dogfood pool: harvest may now see the new TODO rows — expect pool agents on items 2/3/4 if unchecked items are pool food (intentional?)
20. Release retro: fold 09-06 §a13-14 + this file into docs/release/ notes for v0.3.1 prep

## g) Questions I cannot answer myself

1. **Release-window policy**: is a temporarily-red master during a
   release cut acceptable (current de-facto behavior), or should the
   smoke learn a release-in-flight state and never go red mid-release?
2. **Help Centre channel**: what channel do I prepare the final reply
   for (the draft is channel-agnostic markdown) — or do you send it
   as-is?
3. **v0.3.1 timing**: cut a fast patch for the `go install` regression
   (needs the cmd/tq module restructure), or let it ride until v0.4.0
   with Postgres CLI wiring? (My read: the regression is disclosed and
   library consumers are unaffected — a deliberate v0.3.1 is justified
   only if binary adoption matters to you now.)

## Skill question coverage (brutal-self-review set)

Forgot: §d1/§b; Stupid-anyway: §e2/e3; Better: §d1/d2; Still improve:
§f; Lied: no — every DONE claim above re-verifiable in this file's
sources (gates output, gh runs 34742883062/34744075261, proxy listing,
live-DB runs); one inference flagged: "T9 satisfied" rests on the green
postgres job, the job log was not grepped for the test name. Ghost
systems: none (facadeparity is gate-wired; new-module.sh is a tool, not
a gate). Split brains: three small ones, all mine, listed in §d/§f12/§e5.
Removed-useful: no (the placeholder line removed in d2 was my own
garbage). Tests: this session ADDED one gate (parity) + two fixtures
classes (release-gates facade cases) + one live-DB suite activation;
weakest spot is the checker having no in-repo self-test (f7).
