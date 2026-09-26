# 19-52 — Conformance-suite consolidation (companion/conform) — close-out

Companion-extraction RESIDUE row executed to **zero mirrors**. The three
mirrored spike suites (~3.5k lines ×3 + the byte-identical sessionfact
pair) are now ONE suite in `internal/queue/companion/conform`;
`scripts/mirror-baseline.txt` regenerated from 6 rows to **0**; the mirror
gate reports `0 cross-backend clone groups` at the canonical invocation.
Completed as the §g1 "consolidate now" path of the 07-12 window (the
repeated execution instruction read as the answer; the M4 upstream memo —
§g2 — stays owner-gated and untouched).

## a) What landed

- **`internal/queue/companion/conform`** (new package, 5 files):
  - `suite.go` — `Store` interface (the exact 37-method surface the suite
    exercises: queue.Store contract + companion extras incl.
    `ArchiveSummary`), `Caps{Exclusivity, ResumeCloseout, LegacyMigration,
    Archive, EnqueuedSnapshot}`, `Suite{FreshDSN, OpenOn, Exec, Begin,
    Dialect, Caps}`, sequential `StoreSuite(t, s)` runner over a 71-entry
    registry (test names preserved — they are now SUBtests of
    `TestStoreConformance`), and the `active`-Suite shims
    (`openTestStore`, `openTestStoreExclusive`, `freshDSN`, `openOn`,
    `dbExec`, `dbBegin`, `dial`) so test bodies survive verbatim.
  - `tests.go` / `sessionfact.go` — the sqlitev4 suite bodies, byte-copied
    (`cp`, never hand-retyped) then token-sed transformed: 6×
    `s.db.ExecContext(` → `dbExec(s, `, 1× `s.db.BeginTx` → `dbBegin(s,`,
    `mustJSON(` → `companion.MustJSON(`, 4 helper signatures `*Store` →
    `Store`, and 8 hand-edit sites: package+imports, the two opener
    helpers, the two-store exclusivity test (freshDSN + openOn ×2),
    Cap-gated skips for resume_closeout / legacy ×2 / archive /
    enqueued-snapshot (skip TEXTS preserved verbatim), and the scale-test
    tx prepares wrapped in `dial(...)` for postgres `$n` ordinals.
  - `race_on.go`/`race_off.go` — the build-tagged `raceDetector` const
    moved into conform (the scale test keeps its -race skip).
- **Harnesses** (~50 lines each): sqlitev4 + cqrsqlite (temp-file DSN,
  `Dialect: companion.SQLite`) and postgresv4 (keeps `testDSN` per-test
  schema management verbatim, `Dialect: companion.Postgres`,
  `LegacyMigration: false`). Caps: cqr `Exclusivity: false` +
  `ResumeCloseout: false` (its two divergences, skip texts carried over);
  pg `LegacyMigration: false` (its skip, carried over); Archive +
  EnqueuedSnapshot false EVERYWHERE (all three skipped those before — the
  bodies remain the future pins for upstream growth).
- **Coverage GAINED by cqrsqlite** (ungated, passed first try): the
  session-fact pair (`TestAppendFactRecordsNonTaskFact`,
  `TestAppendFactDoesNotMaterializeTaskRows`) and
  `TestCompleteResetsLastError` — tests its old mirrored file simply
  lacked.
- **`ArchiveStats` unified**: identical 3-field struct lived ×3 → moved to
  `companion.ArchiveStats` (reads.go), adapters carry
  `type ArchiveStats = companion.ArchiveStats` aliases.
- **Dead code deleted** (the consolidation orphans it): `mustJSON` vars ×3
  adapters, pg's `pgq` + `(*Store).exec` suite-only shims (the pg harness
  Exec inlines `cr.ExecContext(ctx, companion.Postgres(q), args...)`),
  and the `race_on.go`/`race_off.go` pairs ×3 modules (6 files,
  `git rm`). The `unused` findings that flagged them are gone, not
  baselined.
- **Baseline shrunken**: `scripts/mirror-baseline.txt` rewritten to 0 rows
  with an updated header (zero since consolidation; any NEW cross-backend
  group fails ci-local).
- **Docs**: AGENTS.md dedup paragraph rewritten (conform + Caps + zero
  baseline + TQ_TEST_POSTGRES note); TODO residue row 305 → `[x]` with
  gate citations; row 306's tail updated (EnqueuedSnapshot Cap is the new
  pin surface for the M4 enrich).

## b) Design notes worth keeping

- **Tests live in non-test files** in conform (`tests.go`, not
  `*_test.go`): the suite compiles into the module build and only the
  three backends' harnesses (`TestStoreConformance`) invoke it. No test
  entry point exists in the conform package itself, so
  `go test ./internal/queue/companion` stays meaningful.
- **`active` package var** spans exactly one StoreSuite call (suites are
  sequential, no t.Parallel anywhere — matches the old suites); previous
  value restored via t.Cleanup.
- **Direct-SQL seams**: `dbExec(s, ctx, q, args...)` and the tx
  `dbBegin`/`dial` pair are the ONLY places the suite touches engine SQL;
  each backend's harness decides the dialect (sqlite identity, pg via
  `companion.Postgres`). The 7 former `s.db`/`s.exec` pokes + the
  scale-test 2 prepared INSERTs are fully covered by these seams.
- **json/v2 is std** on the module's go 1.27.1 floor — conform's
  `jsontext`/`json/v2` imports added ZERO go.mod/go.sum churn anywhere
  (`go mod tidy` in companion was a no-op beyond formatting).
- Transcription integrity: the suite was `cp`-seeded from sqlitev4's file
  and transformed mechanically (sed token replaces + 8 targeted edits
  verified by view-then-edit), never hand-retyped; compile + full-suite
  gates are the truth, per the transcript-garbling lesson.

## c) Verification (all at final tree unless noted)

| Gate | Result |
| --- | --- |
| sqlitev4 module (build+vet+test, GOWORK=off) | ok, suite 3.6s |
| cqrsqlite module (build+vet+test) | ok, suite 3.7s |
| postgresv4 module (build+vet+test) | ok — suite compile+skip green; RUNTIME needs TQ_TEST_POSTGRES |
| companion module (build+vet+test) | ok |
| 19-module loop (find-derived, build+vet+test) | ALL ok, rc=0 |
| Root `go mod vendor` + build + vet | rc=0 |
| Root `-race -count=1` suite | 15/15 pkgs ok, rc=0 |
| `check-go-mods.sh` | rc=0 |
| `check-mirror-clones.sh` (strict) | `0 cross-backend clone groups`, rc=0; all 6 baseline rows RESOLVED |
| `art-dupl -t 4` sanity | 0 clone groups in backend dirs |
| `lint-baseline.sh` clean-cache regen + `--check` | OK — 163 rows / 1164 findings (was 183 / 1550: the mirrored test noise collapsed from 3 copies to 1) |
| `check-facade-parity.sh` / `check-guard-wiring.sh` | rc=0 / rc=0 |
| gofmt on touched dirs | clean (golangci-lint fmt for parity) |
| nix `vendor-hash` fast gate | rc=0 |
| cmd/tq gate | ONLY the 2 pre-existing upstream-gated drift reds (TestJournalDriftNoDriftAfterRescue, TestJournalDriftSeededDriftAllFields — row 306, M4-gated) |

**postgresv4 runtime disclosure (carried from 07-12):** a postgres IS live
on 127.0.0.1:5432 but peer auth rejects user `lars` (role absent) and
password auth has no credential available to this session (sudo banned);
`TQ_TEST_POSTGRES` remains unset locally, so the pg conform run executes
in CI's env-gated job exactly as before. Nothing about the runtime story
regressed — the suite previously had the same env gate; it now simply
runs the SHARED bodies.

## d) Friction log

1. **GOTOOLCHAIN=local env-lie struck again** on the first companion
   `go mod tidy` (go.mod requires 1.27.1) — the documented one-env
   discipline (`export GOEXPERIMENT=jsonv2 GOTOOLCHAIN=auto` on EVERY
   invocation) is the only fix; re-confirmed yet again.
2. **Python string surgery on Go source**: attempted once for the pg shim
   removal, failed harmlessly (substring mismatch before any write) and
   was abandoned for view+edit — the repo's ban exists because the failure
   mode is silent wrongness; here the failure was loud, but the tool is
   simply the wrong one for Go edits.
3. **The extraction window left companion's lint rows unregenerated**
   (its battery skipped lint-baseline; the 01:41 regen predated
   companion's 07:12 landing). This window's clean-cache regen absorbed
   BOTH that debt and the new conform classes in one deliberate regen —
   cited above. Lesson: any window introducing a new PACKAGE owes its own
   baseline regen before stopping; "new module = new rows" is now written
   into this report for the next extraction-style window.

## e) Not done here (deliberate)

- **M4 upstream enrich** (row 306's root cause; the 2 cmd/tq drift reds):
  owner-gated (§g2 of 07-12 — memo authorship + upstream releases in
  go-cqrs-lite). The `Caps.EnqueuedSnapshot=false` pin is the ready flip
  target the day it lands.
- **S4 cutover**: sqlitev4/postgresv4 still die there per ADR-0019; conform
  is written so the S4 window keeps the suite and drops only the two
  dead-end harnesses.
