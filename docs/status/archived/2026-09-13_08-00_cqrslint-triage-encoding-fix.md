# cqrs-lint review: triage of 8 findings, real encoding-stamp bug fixed, read-only intent encoded

Date: 2026-09-13 08:00 CEST · Session: interactive (owner pasted `cqrs-lint` output → "review!", then "doctor/rules — what we WANT TO BE, not what we are") · Format: **Markdown** per explicit owner instruction (skill default is HTML — override flagged per spec)

## Summary

The owner ran `cqrs-lint` (domain-aware linter for go-cqrs-lite consumers, 203 rules) over the repo and asked for a review. Verdict after primary-source verification of every claim: **1 real defect, 7 false positives / by-design**. The real defect is severe for ADR-0014's stated purpose: the journal adapter's events carried an **empty encoding stamp**, so `event.DecodePayloadAuto` — the decode path every go-cqrs-lite consumer (projections, watermill bridges, metaengine) uses — **failed on every fact event**. Fixed by migrating `factEvent` to `event.New` + `WithCodec(codec.JSONCodec{})` + explicit `WithSchemaVersion(1)`. The "what we WANT to be" ask produced `.cqrs-lint.json`: `library-framework` preset + pinned `command-flow: read-only` + dated rationales for every disabled rule; `cqrs-lint` now reports **"No findings. Clean!"** with 3 inline suppressions carrying reasons.

## a) FULLY DONE

1. **Every linter claim verified against primary sources** (module cache, v4.11.0/v0.2.0 sources, proxy version lists) — nothing taken from the linter's word:
   - A014's premise ("`event.NewEvent` deprecated") is **false** in event/v4 v4.11.0 — no Deprecated marker on `NewEvent`; the deprecated things are elsewhere (tombstones, alias methods).
   - D013 is a **false positive**: `buildEvent` already defaults `schemaVersion` to 1 for BOTH constructors.
   - V006 is a **false positive**: `go list -m -versions` shows every pin IS that module's latest tag (event v4.11.0, id v4.6.0, metadata v4.7.0, record v4.5.0) — go-cqrs-lite versions its modules independently (its own ADR-0011 analog); "aligning" would mean pinning stale releases.
   - The real bug was found by tracing the CONSUMER contract, not the API-freshness angle: `codec.ForEncoding("")` returns `ErrUnknownEncoding`; `NewEvent` never stamps encoding; `event.New` does.
   - Codec safety check before migrating: go-codec has v1/v2 build-tag flavors; v1 would base64-corrupt `jsontext.Value`. Moot in practice (the adapter can only compile with GOEXPERIMENT=jsonv2, which also flips go-codec to its v2 flavor), and the chosen fix keeps the stdlib json/v2 marshal semantics.
2. **The fix** (commit 28cfcb7): `internal/journal/cqrs/journal.go` — `factEvent` now builds via `event.New(factPayload{…}, WithCodec(codec.JSONCodec{}), WithEventID, WithOccurredAt, WithSchemaVersion(1))`; payload bytes unchanged (still JSON, same tags); `encoding` stamped `json`.
3. **Regression test** `TestPayloadDecodesThroughLibraryAPI` (journal_test.go): asserts `Encoding()==codec.EncodingJSON` + `SchemaVersion()==1` on every event AND a full `event.DecodePayloadAuto[factPayload]` round-trip — pins the actual downstream consumer contract that was broken.
4. **Inline suppressions with reasons**: C025 ×2 (journal.go:125 area, seqid.go:30 area — validation errors built from data, no wrapped cause exists) and A032 (factPayload.TaskID — wire mirror of the leaf module's plain string; branding buys coupling, not safety).
5. **`.cqrs-lint.json`** (commits 28cfcb7 + 536b5da): `library-framework` preset (read-only pinned; F-series adoption coaching disabled — it contradicts ADR-0014's recorded deferred-tiers decision), explicit `features.command-flow: read-only`, and dated rationales for disabling V006, A009 (adapter IMPLEMENTS a journal; no stack to wire), A018 (never calling Save/Publish/Dispatch IS the contract).
6. **Repo-wide single-site confirmation**: `internal/journal/cqrs` is the only `event.New*` call site; cmd/tq consumes via the adapter only.
7. **All gates green**: cqrs module build/vet/`go test -race`; all-16-module loop (incl. the concurrent facade modules); root build/vet + full `-race` suite; `check-go-mods.sh` exit 0; `nix build` exit 0 (vendorHash unchanged, as predicted — no new dep sources); gofmt clean; final `cqrs-lint` clean.
8. **Consumer-path smoke**: `/tmp` scratch DB (never bare `tq` — production env discipline held), `enqueue` → `tq facts --cqrs` renders byte-equivalent raw-JSON payloads (`taskId`, `detail` intact, no base64 corruption).
9. **Docs**: CHANGELOG `[Unreleased] → Fixed` entry; AGENTS.md "Relation to other projects" go-cqrs-lite paragraph extended with the config pointer + the three falsified claims + the fixed defect (commit 565c2f4); this report + index row.
10. **Concurrent-drift repair**: the parallel facade-adoption window had re-downgraded `go` directives to `1.26` in `internal/task`, `internal/journal`, `task`, `journal` (the exact documented red-master class — same four modules repaired at HEAD in 1cc32d6). Restored to `1.26.7` per the documented repair; left ALL other concurrent work untouched. `check-go-mods.sh` went FAIL → exit 0.

## b) PARTIALLY DONE

1. **Verification depth on the Go gates**: build/vet/tests/-race/nix/check-go-mods all green, but `scripts/lint-baseline.sh --check` (golangci growth gate) and the full `./scripts/ci-local.sh` (smokes, webui-css check, master-CI state) were NOT run this session — declared, not hidden. New-code lint deltas are likely zero (vet is clean; the additions are tests/comments) but that is a hypothesis until the baseline check runs.
2. **CHANGELOG style conformance**: entry written and committed; a deliberate style pass against house conventions (trailing date stamps, entry ordering) was not done.
3. **§f harvest**: the next-steps list below is a brainstorm per the status-report skill; TODO_LIST.md/ROADMAP.md harvesting (docs-health HARVEST) deliberately not started — owner said "report, then wait".
4. **Upstream linter fixes**: the three falsified rules (V006, A014, D013) are documented here and in `.cqrs-lint.json`, but NOT reported upstream to cqrs-lint (verify-before-filing gate would be step one there).
5. **FEATURES.md**: adapter row still accurate (FULLY_FUNCTIONAL remains true), but the DecodePayloadAuto compatibility guarantee is worth a phrase — not edited this session.

## c) NOT STARTED (identified, consciously untouched)

1. `cqrs-lint` CI gating (ci-local step + ci.yml job, advisory-first) — the config exists, the gate doesn't.
2. Upstream issues/PRs to cqrs-lint for the three rule-quality defects.
3. Public facade question for the adapter: ADR-0016's seven facades do NOT include `journal/cqrs`; an external ecosystem consumer is back on the `internal/` path — the exact problem ADR-0016 was created to solve. Owner call (PROPRIETARY seam).
4. Deferred ADR-0014 tiers: watermill `CatchUpSubscriber` / SSE broker / metaengine over fact events.
5. Wire-format byte-pin test (`evt.Payload()` byte-equal to `json.Marshal(factPayload)`) — beyond the round-trip test, protects against silent codec drift.
6. `cqrs-lint` binary provenance pinning in flake devShell (it's currently ad-hoc on PATH; version unpinned).
7. Preventive fix for the recurring `go mod tidy` go-directive downgrade (it has now bitten twice: the T38 window and this session's concurrent window).

## d) TOTALLY FUCKED UP (brutal honesty)

1. **The bug shipped in the 2026-09-12 adapter window and its 12-test suite missed it.** The window's tests asserted payload round-trip via raw bytes — the one thing that CANNOT catch a missing encoding stamp. The consumer contract (`DecodePayloadAuto`) was never exercised. If any external consumer had adopted the adapter on the v0.2.0 tag, their first decode would have failed. That is the facade window's whole point, so this was one daemon-commit away from embarrassing us publicly.
2. **I initially framed the work as "review the linter findings" instead of "trace what the consumer sees".** The linter's wrong "deprecated" framing nearly became the story ("migrate because deprecated"). The real defect surfaced only when I read `DecodePayloadAuto`/`ForEncoding` as the contract. Lesson recorded: review API-migration findings from the wire/consumer side first; API freshness is the weakest signal in the report.
3. **The recurring go-directive drift class recurred AGAIN mid-session** (4 modules, concurrent window, would have failed CI's go.mod health step on push). The gate caught it — but the gate is a catcher, not a preventer, and this is now a two-time genus. Nobody has removed the generator (bare `go mod tidy` in zero-dep leaves).
4. **Two burned smoke rounds on `tq` flag syntax** (global vs subcommand `-db`; `--type` before knowing flag names) — trivial, but I fired `enqueue` flags from memory instead of `--help` first, and the first `facts --cqrs` ran against an empty DB producing a misleading `[]`.
5. **Nothing catastrophic this session.** No reverts of others' work (the go-directive restoration is the documented repair for a known regression, not a revert of forward progress — the facade require-promotion in the same files was preserved).

## e) WHAT WE SHOULD IMPROVE

1. **Test new adapters against their consumer contract, not their own bytes.** A "decode it the way the ecosystem will" test belongs in the FIRST test pass, not after a linter stumbles into the gap. Consider a convention note in AGENTS.md or the how-to-golang skill.
2. **Gate what you configure.** `.cqrs-lint.json` without a CI step is documentation, not a gate; it rots exactly like the webui-css orphan guard did before it was wired.
3. **Kill recurring failure classes at the generator, not the catcher.** The go-directive downgrade has a mechanical trigger; find and remove it (wrapper, convention, or toolchain pin enforcement).
4. **Linter rule quality is part of trust.** 3 of 8 findings were false on their face (deprecated-claim, schema-version claim, version-alignment claim). A linter that cries wolf gets disabled wholesale — upstream fixes protect the tool's value.
5. **Verify external claims BEFORE triaging, not during.** `go list -m -versions` took one command and falsified V006 instantly; the module-cache greps falsified A014/D013. The triage table should be built only from verified rows.

## f) NEXT (brainstorm, up to 50, sorted by impact — NOT a commitment list; most rows 10+ are ROADMAP fuel)

1. Owner: push `internal/journal/cqrs/v0.2.0` + master (long-standing CI-red carrier; the encoding fix should ride the next tag anyway).
2. Decide whether the encoding fix ships in the v0.3.0 facade release or forces an earlier patch tag (external adoption is the stated motivation; shipping a known-broken decode path in the adoption release would be self-defeating).
3. Gate `cqrs-lint` in ci-local.sh (advisory step first, flip to hard after a soak window) + ci.yml parity.
4. File upstream cqrs-lint fixes: V006 (respect independent per-module versioning / compare against each module's own latest), A014 (drop the false "deprecated" claim for `NewEvent` in v4.11.0 or pin the claim to a version), D013 (know `buildEvent` defaults schemaVersion=1) — verify-before-filing applies.
5. Decide public `journal/cqrs` facade vs internal-only (ADR-0016 gap; PROPRIETARY seam; external ecosystem consumers are the adapter's stated audience).
6. Wire-format byte-pin test: `evt.Payload()` byte-equal `json.Marshal(factPayload)` (guards against codec flavor drift / future constructor migrations).
7. `scripts/smoke/facts-cqrs.sh`: binary + real sqlite store end-to-end (enqueue → facts --cqrs → assert encoding-stamped render), CI-safe.
8. Add `Encoding()`/`DecodePayloadAuto` assertion to the store-backed integration test (cmd/tq/facts_cqrs_test.go) — same contract at store level, not just fakeSource level.
9. Run `scripts/lint-baseline.sh --check` (declare the golangci delta from this session's changes).
10. Run full `./scripts/ci-local.sh` once the concurrent facade window settles.
11. Prevent the `go mod tidy` downgrade at the source: identify the trigger (toolchain rewriting leaf directives), then either a wrapper (`tq-tidy`?), an AGENTS.md hard rule with the `go mod edit` alternative, or a pre-tidy hook.
12. Pin `cqrs-lint` (version + provenance) in flake devShell for reproducible runs.
13. TODO_LIST/ROADMAP harvest from this report (docs-health HARVEST) once the session resumes work.
14. FEATURES.md adapter row: add the DecodePayloadAuto-compatibility guarantee + `.cqrs-lint.json` pointer.
15. ADR-0014 appendix: encoding-stamp contract + `event.New` migration note (keeps AGENTS.md lean, avoids doc split-brain).
16. Confirm dependabot config (landed concurrently this session) covers all 17 modules including `internal/journal/cqrs` and the new facades.
17. Confirm the facade release plan's 16-tag enumeration includes cqrs (plan doc says yes — verify against release.sh allowlist).
18. Evaluate go-cqrs-lite `testutil`/`eventtest` for the adapter suite (B015 was preset-suppressed; the library's own test utilities may strengthen the round-trip tests for free).
19. Config polish: pin `features.domain: internal` (it's auto-`unknown` today) and consider `server-local: true` accuracy for `tq serve`.
20. Root go.mod: `id/v4` direct require exists only for `cmd/tq/facts_cqrs_test.go` — consider moving that test into the adapter module to slim the root graph.
21. Explicit comment in `.cqrs-lint.json` for the S002/S003 preset disables (fact payloads carry prompts/repo names into a LOCAL journal — encryption/signing consciously N/A; make the consciousness visible).
22. Deferred tier PoC: watermill `CatchUpSubscriber` consuming the fact journal (ADR-0014's headline use case, never exercised end-to-end).
23. Deferred tier PoC: go-sse/SSE broker over fact events for `tq serve` (would the webui journal tail become a cqrs consumer? — design question first).
24. metaengine over fact events (cost-planned queries) — deferred tier, needs a real query to justify.
25. `cqrs-lint doctor` as a cheap devShell sanity app (`nix run .#cqrslint-doctor`?) — only if the tool gets flake-pinned (row 12).
26. Bench: `FactJournal.ReadAll` allocations at journal head sizes (prep for any metaengine/SSE tier; cheap `go test -bench`).
27. Sweep `internal/queue/sqlite/store_test.go`'s go-cqrs-lite import (found in the repo-wide check; likely a test-side id/event assertion — confirm it needs nothing from the encoding change).
28. Docs: AGENTS.md go-cqrs-lite paragraph is growing past "concise enduring context" — move detail to ADR-0014 appendix at the next natural docs pass.
29. Verify the daemon landed this report + its index row in companion commits (AMEND MANEUVER if the index row rides a later commit — `check-status-index.sh` is the catcher).
30. CHANGELOG style pass over the new Fixed entry (house conventions: trailing date, wording).
31. Teach the review/self-review prompt contract to include "consumer-contract test" for adapter seams (the a)-g) close-out checklist is repo convention — one clause would have caught this bug class).
32. Consider `tq facts --cqrs --json` streaming output for large journals (currently full array encode; NDJSON would stream — low priority, real journal is small).
33. `WithSource`/`ActorID=Owner` metadata enrichment on fact events — YAGNI flag: only when a consumer needs attribution metadata (ADR-0014 deferred tiers).

*(Stopped at 33 grounded items — the remaining rows would be filler; the skill says extra N is brainstorm, not commitment.)*

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **Facade scope for the adapter**: should `internal/journal/cqrs` get a public facade module (the ecosystem consumers ADR-0014 names currently hit the `internal/` path — the exact blocker ADR-0016 removed for the other seven), or is the adapter deliberately internal-only because of the PROPRIETARY go-cqrs-lite seam?
2. **Release vehicle for the encoding fix**: does the fix ride the v0.3.0 facade release (the adoption-motivated release, currently pending tag push), or do you want an earlier patch tag so no external consumer ever sees the broken decode path? (Owner call — release timing is yours; I can prepare either.)
3. **The go-directive downgrade keeps coming back** (T38 window, now the facade window, same four leaf modules, same bare-`go mod tidy` trigger): do you want me to hunt down the generator and fix it preventively (wrapper/convention/hook), or is `check-go-mods.sh` catching it pre-push good enough?

## Verification log (claims → evidence)

| Claim | Gate/command | Result |
| --- | --- | --- |
| Adapter fix correct | `cd internal/journal/cqrs && go build/vet/test -race -count=1` | ok |
| Repo-wide green | 16-module loop (build+vet+test per module) | ALL-MODULES-OK |
| Root green | `go build ./... && go vet ./... && go test ./... -race` | ok |
| go.mod health | `./scripts/check-go-mods.sh` | exit 0 (after drift repair) |
| Nix reproducibility | `nix build` | exit 0, vendorHash unchanged |
| Formatting | `gofmt -l internal/journal/cqrs/` | clean |
| Linter | `cqrs-lint` | No findings. Clean! (3 inline suppressions) |
| Consumer path | scratch-DB `enqueue` → `tq facts --cqrs` | raw-JSON payload, no corruption |
| Single fix site | repo-wide grep `event.NewEvent` / `go-cqrs-lite` | only `internal/journal/cqrs` constructs events |
| Landing | daemon commits 28cfcb7 (code+config), 536b5da (config edit), 565c2f4 (AGENTS/CHANGELOG/go-mod repairs) | all on local master |

*Point-in-time snapshot — re-verify before treating any claim as current.*

## Addendum (same session, ~08:20 — owner asked "Is that all?"; continuation closed the declared gaps)

§b/§f rows executed in the continuation (all verified):

- **Row 6 DONE**: `TestPayloadWireFormatPinned` — byte-exact pin of `evt.Payload()` against a plain json/v2 marshal of the payload struct (guards constructor/codec drift).
- **Row 8 DONE**: `assertEventsEncodingStamped` helper in `cmd/tq/facts_cqrs_test.go` — store-level (sqlite → adapter) encoding assertion; extracted to a helper after my inline loop pushed gocyclo to 21 (gate discipline: no new findings in touched functions).
- **Row 9 DONE**: `lint-baseline.sh --check` — first run FAILED on two NEW classes from `scripts/facadeparity/main.go` (concurrent window's file, zero growth from mine); by the full-battery re-run the concurrent window had fixed it: **886 vs 886, within baseline**.
- **Row 10 DONE**: full `ci-local.sh` (first run aborted at step 1 — master CI red, by design). Diagnosed the red run 34741576449: (1) go-mod drift — the class I repaired on disk, needs owner push; (2) `TestPostgresOpenWithPool` — concurrent facade window's new test; (3) `TestConcurrentClientsRace` webui on windows. None from this session's changes. With `CI_CHECK=off` (conscious bypass, root cause documented): **ALL CI GATES GREEN — "this exact tree is what CI will see"**, including nix flake check.
- **Rows 14/15/21 DONE**: FEATURES.md adapter row now states the encoding-stamp guarantee + test pins; ADR-0014 gained a "Post-adoption note (2026-09-13)" appendix; `.cqrs-lint.json` gained the S002/S003 conscious-acceptance note.
- **Rows 16/17 CLOSED (verified, no action)**: dependabot covers all 17 module dirs incl. `/internal/journal/cqrs` + facades; release.sh sub-tags via `find … -name go.mod` — cqrs rides automatically.
- **Row 27 CLOSED (non-issue)**: the `internal/queue/sqlite/store_test.go` "go-cqrs-lite" hit is a fixture string (`Project: "go-cqrs-lite"`), not an import.
- **Row 30 DONE**: CHANGELOG entry already conforms (trailing date, house shape) — verified, no edit.
- **NEW FIX (found by running the full battery)**: `check-doc-refs.sh` failed on `AGENTS.md` citing `example/taskmanager` — a go-cqrs-lite LIBRARY-repo path written repo-ambiguously by the concurrent storage-verdict window (d7ec03a). Verified the path exists in `~/projects/go-cqrs-lite/example/`, added the documented allowlist entry with provenance comment. Doc-refs gate green.
- **§f row 4 clarified (provenance)**: `cqrs-lint` is `cmd/cqrs-lint` **inside the local go-cqrs-lite repo** (owner's own tool, `v4.8.2-0.20260904…`). A014's "NewEvent deprecated" claim verified FALSE at library HEAD too (no Deprecated marker). Rule-semantics changes (V006 policy, A014 fix, D013 default-awareness) are the owner's call in his own tool — precise evidence recorded here + in `.cqrs-lint.json` instead of patching the sibling repo unasked.

Gates after continuation: adapter module `-race` green, `cmd/tq` facts tests green, `check-go-mods.sh` 0, doc-refs ok, lint-baseline within, ci-local ALL GREEN, `cqrs-lint` clean, gofmt clean. Working tree clean (daemon swept; latest local master f33b675 — unpushed, agents never push).
