# Status: 50-Item Backlog Execution — Retry Hygiene, Type Pins, Contract Versioning

**Date:** 2026-09-10 02:51 CEST
**Session scope:** full execution of the 50-item list from the 02-04 report's
§f — every item implemented, verified, or explicitly rejected with reasoning.
All gates green at HEAD; `master == origin/master`.

---

## a) FULLY DONE

1. **Real-kernel ETXTBSY e2e test** (`retry_e2e_test.go`): provokes a genuine
   `syscall.ETXTBSY` (execve of a binary held open for writing), releases the
   fd mid-ladder, proves go-retry absorbs the REAL errno — not just the stub
   value. Skips where the platform won't reproduce. PASS in 0.17s.
2. **Retry overhead benchmark**: bare 0.8ns vs wrapped ~10ns/op — wrapper
   cost pinned so it's never re-litigated without numbers.
3. **AgentPayload version gate** (item 16, scoped to non-breaking): `v` field
   added; absent decodes as v1 (all queued tasks stay valid); `v:>1` fails
   fast as a PERMANENT error (dead-letter, zero retry burn).
   `TestAgentPayloadVersionGate` pins both paths.
4. **WatchConfig knobs** (item 35): `InitialBackoff`/`MaxBackoff` exported
   with unchanged defaults (500ms→30s); tests no longer poke unexported fields.
5. **Watch drop-warning pin** (item 36): `TestWatcherLogsDropWarning` proves
   each dropped stream logs one warning naming the endpoint; first version
   failed because the addr rides a structured attr, not the message — fixed
   by capturing attrs in the test handler.
6. **Status transitions exhaustiveness** (item 12): new `Status` without a
   transitions row now fails `TestStatusTableExhaustive`.
7. **SQLite fresh-DB migration retry** (item 44's discovery): the last
   hand-rolled retry loop in the tree found and migrated to go-retry —
   identical delays (200ms→1600ms), now ctx-honoring and jittered.
8. **Documentation**: ADR-0013 (retry policy + two documented exceptions +
   jitter/ctx deltas), CHANGELOG entry, status-index row for the 02-04
   report (the index gate caught its own absence — gate works).
9. **Live-pool verification** (items 28–29, read-only): `tq stats`/`dlq`/
   `audit` — DLQ empty, no drift, 161 repos audited clean, nothing stranded.
10. **Environment checks**: go-sse v0.6.0 is the latest release (we're
    current); no direct deps outdated (all updates are transitive riding
    parent bumps — zero vendorHash risk); go-retry/error-family pins
    aligned across all modules.
11. **Final gates**: webui smoke + status-loop smoke against the nix binary,
    all-module build/vet/test, full `ci-local.sh` ALL GREEN, pushed.

## b) PARTIALLY DONE

1. **Item 30 (stale status-report sweep)**: the sweep mechanic was proven by
   the index-gate incident, but a full docs-health annotate pass over all
   historical reports was not run — it re-reads and edits every report and
   deserves its own session.
2. **Items 28's enqueue half**: harvesting open TODO items into the live
   pool enqueues REAL agent work (real money); I verified the pipeline
   read-only and left enqueueing to the owner.

## c) NOT STARTED (with reasons — not forgotten)

1. Item 41 (templ-components indirect trim): upstream conversation.
2. Item 43 (pgx read-path retry): store invariants say do-not-touch without
   an ADR; the conformance suites would need extending first.
3. Item 45 (screenshot-review executor): demand hasn't materialized; YAGNI.

## d) TOTALLY FUCKED UP (things I got wrong this session)

1. **Pipeline masking bit me live**: `ci-local.sh 2>&1 | tail -2` turned a
   RED treefmt gate into a green-looking tail and the `&&` chain pushed the
   broken tree. Exactly the AGENTS.md failure pattern ("filters + pipeline
   exit codes hide failures"), committed by me, detected on the next look.
   Fixed within the session (import grouping, re-ran gate, pushed), but the
   push should have come after the gate read with its own exit code.
2. **Two edit attempts lost to stale reads**: a daemon auto-commit landed
   between my read and edit twice (`retry_e2e_test.go` helper fix, watch.go
   knob wiring); one retry each. Also one edit was rejected because the
   gofmt pass had re-tabbed the file between view and edit.
3. **First watch-warning test asserted the wrong thing** (message contains
   addr) because I forgot slog puts attrs outside the message — the test
   failure was correct and taught the fix, but I should have known.
4. **My python patch script quoted `\"` inside a heredoc** and silently
   matched nothing; caught by the "unexpected content" branch printing
   instead of an exception — luck, not design. Shell-quote discipline.

## e) WHAT WE SHOULD IMPROVE

1. **Never branch on `| tail` output**: run the gate, capture `$?`, then
   look. The gate's verdict must be the exit code, not the last line.
2. **Re-read before every edit in daemon territory**: auto-commits change
   mod times; the edit tool's staleness check is doing real work, but each
   rejection is still a round trip.
3. **Golden-test framing**: assert the positive property ("ETXTBSY IS
   reachable"), not the inverse — my first wrap-chain assertion tested
   the wrong direction and would have passed on a regression.
4. **slog assertions should capture attrs by design**, not as an
   afterthought.
5. **The 50-item discipline worked**: writing the list with verify-steps
   attached made "done" checkable. Keep doing exactly this.

## f) UP TO 50 THINGS TO GET DONE NEXT (prioritized)

**Verify-and-harden (cheap, high trust)**

1. Extend the payload-version gate: render `v:1` explicitly in
   `RenderAgentPayload` so producers opt in deliberately.
2. Pin `EvidenceTailBytes` (4096) with a test that all three executors'
   tails measure exactly that.
3. Add `TestStatusTableExhaustive`-style pins for the review/status dedup
   key formats (`review:<id>`, `reviewfix:<id>:<hash>`, `status:<p>:<t>`).
4. Pin `Task-Queue-ID` footer resolution in a unit test with a golden argv
   (partially covered by argv contract — assert the rendered prompt too).
5. Property test: `ItemKey` never collides on 10k distinct (repo,text) pairs.
6. Add `tq doctor` check: journal-vs-table consistency count (facts
   behind/ahead of projections).
7. Golden test for `RequeueEvidence.RetryIn` monotonicity across attempts.
8. Fuzz `unwrapCommand` with malformed JSON shapes.
9. Hermetic re-run of the real-ETXTBSY test in CI (linux runner) so the
   skip path doesn't silently become permanent.
10. Confirm the govulncheck baseline (parallel agent's job) stays advisory.

**Type-model thread**
11. Brand `LeaseOwner` (now has 9 use sites — at the threshold).
12. `Priority` → typed enum with ordering guarantees.
13. `DedupKey` branded string; kill bare-string hashing at call sites.
14. Extract a shared `BackoffLadder` value type used by worker + both stores
(compute-only; ADR-0013 exception stays for the loop shape).
15. `WatermarkEntry` JSON tags + round-trip test.

**Dependency hygiene**
16. Schedule the templ/pgx/x-* indirect refresh for a quiet window (one
vendorHash dance, one commit).
17. Watch modernc.org/sqlite releases for a v2 announcement.
18. go-sse: check whether the GOEXPERIMENT=jsonv2 requirement can retire.
19. Audit `go.mod` toolchain lines across modules for drift (script exists —
run it in ci-local explicitly if not already).

**Observability**
20. Structured warning when a watcher's backoff hits MaxBackoff (saturated
reconnection is a state operators should see).
21. `tq top`: expose retry-attempt histogram from journal facts.
22. WebUI task detail: render `FailureEvidence` structurally (currently
only `tq facts` shows it — the audit found zero structured readers).
23. Log line when payload-version gate trips (with the version seen).

**Docs**
24. FEATURES.md row for the payload contract version.
25. ADR for the payload-version policy itself (v-field semantics).
26. Annotate the 2026-09-09 reports whose forward items this session closed.
27. AGENTS.md payload-contract section: add the `v` field line.
28. Domain language: define "evidence", "ladder", "supervisor" in
docs/DOMAIN_LANGUAGE.md.

**Ops**
29. Owner: flip govulncheck from advisory to gate once baseline is triaged.
30. Owner: decide enqueue policy for the open TODO items found in audit.
31. Nightly fuzz campaign seed-commit verification (cron claim).
32. `tq doctor` in the systemd services' ExecStartPre (catch rails drift).
33. Capture a fresh demo of `tq top` now that backoff saturation is
observable.
34. Archive fully-resolved status reports per the docs-health convention.
35. Add the session-start ritual to the global AGENTS.md template too.
36. Bench: add worker pool throughput baseline before/after any store change.
37. Consider `-race` for the new e2e test in CI (it spawns a goroutine).
38. Sweep `//nolint:exhaustruct` usage for a repo-wide lint policy note.
39. Run dprint/treefmt manually on docs before committing (learned twice).
40. Check `-count=1` discipline in all smoke scripts (cache-fresh runs).
41. Pin the stub-agent shell (`#!/bin/sh` vs bash) — busybox portability.
42. Review `MaxConcurrent` interplay with pool concurrency in agentpool.
43. Add timeout to the real-ETXTBSY helper's fd-release goroutine.
44. Move retry Configs to named constructors (`etxtbsyRetryConfig()`) so
magic numbers get one home.
45. Verify smoke scripts fail loudly when `TQ_BIN` points at a stale binary.
46. docs/planning: mark executed items in the 02-02 plan doc tables.
47. Cross-check FEATURES.md `PARTIALLY_FUNCTIONAL` rows against current code.
48. Consider conformance-suite extraction once a second Journal exists.
49. Template a "new executor" checklist (register + version + evidence).
50. Celebrate: two sessions, zero reverted parallel-agent work.

## g) QUESTIONS I CANNOT ANSWER MYSELF

1. **Enqueue authority**: the audit shows open TODO items eligible for the
   live pool. Should unattended sessions enqueue real agent work
   (money-spending) or is that strictly owner-run?
2. **Payload version rollout**: should `RenderAgentPayload` start stamping
   `v:1` explicitly now (producer-side opt-in), or keep emitting
   unversioned payloads until a v2 exists?
3. **govulncheck ownership**: a parallel session added the advisory CI job
   mid-flight. Is that session still active on CI hardening, so I stay out
   of `.github/`?

---

_Session footprint: 12 commits pushed (last: `f3afce4..2f5b342` plus
formatter fix), all gates green at HEAD (ci-local, nix build, smokes vs
nix binary, all-module tests, status-index)._
