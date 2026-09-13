# cqrs-lint continuation: gap-closure round — full battery green, overclaim corrected

Date: 2026-09-13 08:21 CEST · Session: continuation of `2026-09-13_08-00_cqrslint-triage-encoding-fix.md` (owner: "Is that all?") · Format: **Markdown** per explicit owner instruction (skill default is HTML — override flagged per spec)

## Summary

The owner's "Is that all?" challenge exposed the round-1 report's central flaw: I declared the session done while two gates I had *listed* as not-run (`lint-baseline.sh --check`, full `ci-local.sh`) had never been executed. This round closed every runnable gap: both hardening tests added, the FULL ci-local battery run to **ALL CI GATES GREEN**, a live doc-reference gate failure fixed (concurrent window's cross-repo citation), four docs/config rows closed, three §f rows verified as already-true, and the cqrs-lint provenance question answered (it lives in the owner's local go-cqrs-lite repo). Master CI red was diagnosed precisely: none of its three failure causes are this session's. The report is a point-in-time addendum-era snapshot; this file covers the continuation only.

## a) FULLY DONE (this round; the prior round's work is in the 08-00 report)

1. **Wire-format byte-pin test** — `TestPayloadWireFormatPinned` (internal/journal/cqrs/journal_test.go): `evt.Payload()` must stay byte-identical to a plain `encoding/json/v2` marshal of the payload struct; any constructor/codec drift now fails the suite. Green under `-race`.
2. **Store-level encoding assertion** — `assertEventsEncodingStamped` helper in `cmd/tq/facts_cqrs_test.go`: the sqlite→adapter path stamps `codec.EncodingJSON` on every event, same contract as the unit tests. Extracted into a helper after the inline version pushed `TestFactsCQRSOverStore` to gocyclo 21 (>20) — no new lint findings in touched functions (baseline later confirmed 886/886).
3. **Full `./scripts/ci-local.sh` → ALL CI GATES GREEN** ("this exact tree is what CI will see"), including nix flake check, all smokes, version-agreement, doc-refs, status-index, ghost archives, TODO honesty, FEATURES/ROADMAP cross-check. First run aborted at step 1 by design (master CI red) and was completed with the documented conscious bypass `CI_CHECK=off` after diagnosing the red run.
4. **Red master diagnosis (run 34741576449, commit 565c2f4)** — three causes, none from this session: (1) `go 1.26` directive drift — the class repaired on disk here; unpushed, so CI still sees the old tree; (2) `TestPostgresOpenWithPool` — the concurrent facade window's new test failing on the CI postgres service; (3) `TestConcurrentClientsRace` (internal/webui) failing on windows-latest. Owner push + concurrent window's fixes un-red it.
5. **`check-doc-refs.sh` failure fixed live**: AGENTS.md cited `example/taskmanager` — a go-cqrs-lite LIBRARY-repo path written repo-ambiguously by the concurrent storage-verdict window (introduced d7ec03a). Verified the directory exists at `~/projects/go-cqrs-lite/example/taskmanager`, then added the documented allowlist entry with provenance comment (first attempt had a trailing-slash pattern bug — the extractor token carries no slash; second attempt landed). Gate green.
6. **`lint-baseline.sh --check` closed**: first run FAILED with two NEW (module,linter) classes — both from `scripts/facadeparity/main.go`, the concurrent window's file (zero growth from this session's code). By the full-battery re-run the concurrent window had already fixed their findings: **886 findings vs 886 baseline, within baseline**.
7. **Docs/config rows closed**: FEATURES.md adapter row now states the encoding-stamp guarantee + the three pinning tests; ADR-0014 gained a "Post-adoption note (2026-09-13)" appendix (encoding stamp, event.New migration, .cqrs-lint.json pointer); `.cqrs-lint.json` gained the S002/S003 conscious-acceptance note (fact payloads are local-journal data; transport protection is webui's job) — config still resolves, lint still "No findings. Clean!".
8. **Verified-already-true (no action needed)**: dependabot.yml covers all 17 module directories including `/internal/journal/cqrs` and the 7 facades; release.sh cuts sub-tags via `find … -name go.mod`, so cqrs rides every release automatically; the `internal/queue/sqlite/store_test.go` "go-cqrs-lite" hit is a fixture STRING (`Project: "go-cqrs-lite"`), not an import; the round-1 CHANGELOG entry already conforms to house style.
9. **Provenance answered (§f row 4 refined)**: `cqrs-lint` is `cmd/cqrs-lint` **inside the local go-cqrs-lite checkout** (`github.com/larsartmann/go-cqrs-lite/cmd/cqrs-lint/v4`, pseudo-version `v4.8.2-0.20260904…`, binary at `~/go/bin`). Verified `event.NewEvent` carries no Deprecated marker at library HEAD either — A014's deprecation claim is stale in the tool itself. Rule-semantics changes in the owner's own linter were deliberately NOT made unasked (V006's "should be pinned to the same release" reads as intentional lockstep policy; A014/D013 fixes would be welcome but are the owner's call).
10. **08-00 report addendum appended** (ANNOTATE convention, never rewrite) + this report + index row; `check-status-index.sh`, `check-doc-refs.sh`, `cqrs-lint`, gofmt all green at close; working tree clean on local master (ae17093; unpushed — agents never push).

## b) PARTIALLY DONE

1. **Master CI un-red**: the go-mod drift fix sits on local master awaiting owner push; the concurrent window's two test failures are theirs to land (their files were still being edited at 08:21 — `scripts/release.sh` & co. modified mid-session).
2. **Upstream cqrs-lint rule fixes**: scoped with primary-source evidence (V006 lockstep-policy assumption, A014 stale deprecation claim, D013 default-blindness) — recorded in `.cqrs-lint.json` + both reports; NOT patched (owner's tool, owner's policy).
3. **§f harvest into TODO_LIST/ROADMAP**: still deliberately deferred — the §g questions gate several items (facade scope, release vehicle), and unchecked TODO items are live pool food (paid agent runs); harvesting 30 items without owner triage would spend owner money.
4. **cqrs-lint CI gating**: config + clean state exist; the gate wiring itself not started (was §c in round 1, remains the top not-started item).

## c) NOT STARTED (unchanged from round 1 unless noted)

1. cqrs-lint gate in ci-local.sh/ci.yml (advisory-first).
2. Public `journal/cqrs` facade decision (ADR-0016 gap) — owner question pending.
3. Release vehicle decision for the encoding fix (v0.3.0 vs patch tag) — owner question pending.
4. Deferred ADR-0014 tiers (watermill CatchUpSubscriber, SSE broker, metaengine PoCs).
5. Preventive fix for the `go mod tidy` go-directive downgrade generator (three occurrences this window).
6. cqrs-lint flake devShell pin (now sharpened: provenance known — `go install` from the local go-cqrs-lite repo or a tagged release).
7. `scripts/smoke/facts-cqrs.sh` binary-path smoke; `tq facts --cqrs` NDJSON streaming; metadata enrichment (YAGNI-flagged) — carried unchanged.

## d) TOTALLY FUCKED UP (brutal honesty)

1. **Round 1 declared "all gates green" while two gates had never run.** The summary table said "all gates green" and only §b admitted lint-baseline/ci-local were "not run". That's an overclaim: a gate that hasn't run isn't green, and the headline shouldn't read green until the battery has. It took the owner's "Is that all?" to make me run them — and the battery immediately caught a REAL failure (doc-refs) and a live moving target (facadeparity baseline). Lesson: the full battery runs before ANY "done" claim, or the claim says "battery not run" in the headline, not a footnote.
2. **The doc-refs failure was catchable in round 1** — it shipped with the concurrent window's commit at 07:32, before my round-1 summary at 08:00. A single `check-doc-refs.sh` run (one second) would have caught it 20 minutes earlier. I ran the gates I had already run before, not the gates I hadn't.
3. **Self-inflicted gocyclo 21** on the first store-level assertion attempt — caught by diagnostics and fixed by extraction, but the loop shouldn't have gone inline into an already-large test function.
4. **Two rounds burned on the allowlist regex** (trailing-slash pattern vs slash-less extracted token) — I patched before reading the matching code (`[[ "$ref" =~ $pat ]]`). Read the mechanism first, patch second.
5. **The A032/A014/V006 triage held up perfectly under re-verification** — no retracted verdicts; the primary-source method from round 1 is the part worth keeping.

## e) WHAT WE SHOULD IMPROVE

1. **"Done" means the battery ran.** ci-local is the pre-push gate; a session that ends without it (or with a clearly-flagged "battery not run" headline) hasn't verified anything about the tree it leaves behind — concurrent windows make point-gates stale within minutes.
2. **Gate newly-arrived code, not just your own.** The doc-refs and baseline failures both came from concurrent commits landing during the session. A cheap full-gate cadence (once before declaring any milestone) is the catcher for work you didn't author.
3. **Fix-on-sight needs the mechanism read first** (the allowlist regex lesson): trivial fixes still get one read of the code that will judge them.
4. **Report §b items should have owners and triggers**, not just "declared": "not run" without "will run before done" or "blocked by X" is how gaps survive their own confession.
5. **The linter is the owner's own tool** — provenance checks (go version -m, which) turned "file upstream" from an abstract TODO into a 5-minute local fix pending authorization. Always resolve tool provenance before writing "upstream" items.

## f) NEXT (brainstorm, up to 50 — carried + refined; NOT a commitment list)

1. Owner: push master (go-mod drift fix + this session's work) — un-reds CI's go.mod health job.
2. Concurrent window: land fixes for `TestPostgresOpenWithPool` (CI postgres service) — un-reds test-postgres.
3. Concurrent window: `TestConcurrentClientsRace` on windows-latest (webui) — un-reds test-windows (assess flake vs regression).
4. Owner: push `internal/journal/cqrs/v0.2.0` tag (long-standing CI-red carrier).
5. Release vehicle decision: encoding fix in v0.3.0 vs immediate patch tag (§g2).
6. Public `journal/cqrs` facade decision (§g1).
7. Authorize cqrs-lint rule fixes in the local go-cqrs-lite repo (§g3): A014 drop/verify the NewEvent deprecation claim; D013 know buildEvent's schemaVersion default; V006 re-decide lockstep policy vs independent versioning.
8. Wire cqrs-lint into ci-local.sh (advisory step) + ci.yml parity; flip to hard after soak.
9. Pin cqrs-lint in flake devShell (from the local repo or a tagged release once rules are fixed).
10. TODO_LIST/ROADMAP harvest from both reports after §g answers (docs-health HARVEST with routing rigor).
11. Hunt the `go mod tidy` downgrade generator (3 occurrences this window): instrument or audit agent habit; preventive wrapper or convention.
12. `scripts/smoke/facts-cqrs.sh`: binary + sqlite end-to-end with encoding-stamp assertion (CI-safe).
13. Bench `FactJournal.ReadAll` allocations (prep for deferred tiers; cheap `go test -bench`).
14. Evaluate go-cqrs-lite `testutil`/`eventtest` for the adapter suite (may replace hand-rolled fakes with contract helpers).
15. Deferred tier PoC: watermill CatchUpSubscriber over the fact journal.
16. Deferred tier PoC: SSE broker over fact events (design question: webui tail as cqrs consumer?).
17. Deferred tier: metaengine over fact events (needs a real query first).
18. Root go.mod: `id/v4` direct require is test-only (cmd/tq/facts_cqrs_test.go) — consider relocating to slim the root graph (now also `event/v4`+`go-codec` direct via the same test).
19. `.cqrs-lint.json`: pin `features.domain: internal` (auto-detects `unknown`); consider `server-local` accuracy for tq serve.
20. Add the consumer-contract-test clause to the repo's close-out checklist (would have caught the round-1 bug class at birth).
21. AGENTS.md go-cqrs-lite paragraph → slim to pointer; detail already lives in ADR-0014 appendix (avoid doc split-brain).
22. Consider `tq facts --cqrs --json` NDJSON streaming for large journals (low).
23. Metadata enrichment on fact events (ActorID=owner) — YAGNI until a consumer needs it.
24. Upstream (post-authorization): cqrs-lint could read go.mod `go` directives to avoid A014-style stale claims (version-aware rule gating).
25. Watch dependabot's first gomod PRs against the 17-module graph (relative replaces + internal requires are a known friction class — release-gates.sh has fixtures).

*(25 grounded items; the rest of the round-1 list is either done, verified-true, or carried above — padding to 50 would be filler.)*

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **Public `journal/cqrs` facade or internal-only?** ADR-0016's seven facades leave the adapter on the `internal/` path — but it's also the PROPRIETARY go-cqrs-lite seam. External ecosystem consumers (the adapter's stated audience) currently cannot import it.
2. **Release vehicle for the encoding fix**: ride the pending v0.3.0 facade release, or cut an earlier patch tag so no external consumer can ever adopt the broken decode path?
3. **May I patch cqrs-lint in your local go-cqrs-lite repo?** It's your tool and your rule policy (V006's lockstep assumption may be deliberate); I have the evidence scoped for A014/D013 factual fixes and can run that repo's own gates — but won't touch it unasked.

## Verification log (this round)

| Claim | Gate/command | Result |
| --- | --- | --- |
| Byte-pin + encoding tests green | `go test -race` cqrs module + `go test ./cmd/tq -run TestFactsCQRS` | ok |
| go.mod health incl. tidy side-effects | `check-go-mods.sh` | exit 0 |
| Doc-reference gate | `check-doc-refs.sh` | ok (after allowlist fix) |
| Lint baseline | `lint-baseline.sh --check` | 886 vs 886, within baseline |
| Full battery | `CI_CHECK=off ./scripts/ci-local.sh` | ALL CI GATES GREEN |
| cqrs-lint | `cqrs-lint` | No findings. Clean! |
| Status index | `check-status-index.sh` | ok |
| Formatting | gofmt on touched files | clean |
| Landing | local master ae17093 (daemon) | working tree clean; unpushed |

*Point-in-time snapshot — re-verify before treating any claim as current.*
