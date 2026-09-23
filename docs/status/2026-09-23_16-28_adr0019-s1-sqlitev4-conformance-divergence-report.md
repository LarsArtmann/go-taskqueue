# Status Report — ADR-0019 S1 Spike (sqlite): sqlitev4 Conformance Run + Divergence Report

Date: 2026-09-23 16:28 CEST · Task: 000001a0cb25e97b458b379ff9d7aaeb4472
(TODO_LIST row 31, section "go-cqrs-lite platform adoption") · Contract:
docs/adr/0019-go-cqrs-lite-platform-adoption.md §S1 — a tq `queue.Store`
implemented over `queue/sqlite/v4.0.0`, tq extras as same-DB companion
tables, tq's existing sqlite suite run against it, every divergence
reported here.

## State on arrival

The spike module `internal/queue/sqlitev4` (adapter, 1,425 lines) already
existed — landed by a prior window (889c1888, 04:26) with NO tests and a
dangling package-comment promise ("known divergences are catalogued in
docs/status"). The 15:59 triage window (§b1/§c2) had flagged exactly this
gap: the suite had never been run against it and the divergence report was
unwritten. This window completed the row's definition of done on that
module; the cqrsqlite/sqlitev4 split brain itself remains owner-gated
(15-59 §g1) and is NOT resolved here.

## What was done

1. **tq's existing sqlite conformance suite copied into the module**
   (store_test.go 3,441 lines + sessionfact_test.go + race_on/race_off;
   package rename only — the suite is self-contained, verified: its only
   helpers `seedFacts`/`parkOnQuestion` are defined inside the test file
   itself, internal/queue/sqlite/store_test.go:1551/3062).
2. **Suite triaged to green against the adapter**: first run 42 failures,
   root-caused to TWO adapter defects (below), fixed in adapter.go; two
   genuine divergences skip-pinned with DIVERGENCE comments (cqrsqlite
   pattern). Final: `go test ./... -count=1` rc=0, **72 pass / 3 skip**;
   `-race` rc=0; `go build`/`go vet` rc=0; gofmt clean (all captured in
   /tmp/sqlitev4-suite4.log, /tmp/sqlitev4-race.log this session).
3. **Adapter fixes (defects, not divergences)**:
   - `identityCodec` nil-payload guard (adapter.go:107): upstream binds
     `[]byte(nil)` as SQL NULL and its `tasks.payload` is `NOT NULL` —
     every zero-payload enqueue died with `NOT NULL constraint failed:
     tasks.payload` (1299), cascading into 40+ claim failures (tq's suite
     ignores enqueue errors in several tests, so claims then found
     nothing). tq's own schema stores the empty string and never fails.
     Fix: nil encodes as the empty blob.
   - `listWhere` payload pushdown CAST (adapter.go:774): upstream stores
     payload as BLOB (its DDL: `payload BLOB`), tq as TEXT — SQLite LIKE
     never matches a BLOB operand against a TEXT pattern, so
     `TestListQueryPushdown` matched 0 for "hello". Fix:
     `CAST(payload AS TEXT) LIKE ?` (read-side only; engine schema
     untouched).
4. **Archive stub** (archive.go): `ArchiveFactsBefore`/`ArchiveSummary`
   exist only so the copied suite compiles; both return a not-implemented
   error, matching cqrsqlite's treatment.

## THE DIVERGENCE REPORT (the row's core deliverable)

### D1 — Finalizes: owner-string gate preserved over token-fenced engine

Upstream finalizes are TOKEN-fenced (ADR-0134): Complete/Fail/
FailPermanent/Heartbeat take the claim token minted at ClaimDue and check
`lease_token` + `lease_expires > now` (upstream lifecycle.go:29-31). tq's
contract is OWNER-string gated. The adapter bridges at `tokenFor`
(adapter.go:212): it resolves the live claim token for (id, owner) from
the tasks row and refuses with `task.ErrLeaseNotHeld` unless the caller IS
the current lease owner with a live lease (Complete/Fail/FailPermanent/
Heartbeat); `CancelOwned` gates on status+owner WITHOUT the liveness
requirement (adapter.go:316-326), matching tq's semantics (a worker may
finish a stop just after lease lapse, before reclaim). Theft detection
therefore stays at tq's owner gate, not the engine token.

Residual semantics deltas (spike-accepted, none suite-pinned yet):

- **Cross-process same-owner finalization is allowed**: the owner check
  reads the DB row, so a restarted process with the same owner string can
  finalize a lease it did not claim in-process (cqrsqlite's in-process
  ledger FORBIDS this — the two spikes diverge from each other here;
  upstream forbids it too, absent token export). tq's current backends
  also allow it, so adapter==tq here; this is the S1-flip question (TODO
  row: "worker finalizes move to claim tokens").
- **Complete does not reset `last_error`** (upstream lifecycle.go:25-31
  vs tq internal/queue/sqlite/sqlite.go:495-497 which sets
  `last_error = ''`). Observable: a task that failed then succeeded keeps
  a stale error string in `tq show`/webui on sqlitev4. Not covered by the
  suite (found by code diff) — queued as a follow-up TODO row.
- Engine Complete writes the result into the completed fact's detail
  (maybeBytes) — same observable shape as tq's maybeJSON; suite-pinned.

### D2 — Fact vocabulary: names identical, enqueued detail is thinner

All 11 upstream lifecycle fact-type strings are byte-identical to tq's
(`task.enqueued/claimed/completed/failed/dead-lettered/cancelled/
cancel-requested/released/requeued/orphaned/reprioritized` — upstream
queue/v4/facts/facts.go vs tq internal/journal/journal.go); upstream is
the transcribed donor and the suite's fact assertions (failed-evidence,
dead-letter detail class, cancel reason detail, reclaim finalize,
rescue-marker enqueued detail, orphaned detail) pass unmodified. ONE
shape divergence, skip-pinned by `TestEnqueueFactDetailCarriesIdentity`:
upstream's `task.enqueued` detail carries `{project, type}` only; tq
adds `priority` (explicit zero preserved) + `dedup_key` — the projection
fields `tq audit --journal` / journal-drift mapping read. Same divergence
cqrsqlite recorded. UPSTREAM-ISSUE CANDIDATE (donor-parity regression);
the adapter cannot fix it without appending a duplicate enqueued fact.

### D3 — Heartbeat facts: parity by mutual absence

Neither backend writes `task.heartbeat` facts. tq's `Store.Heartbeat` is
a pure lease extension (internal/queue/sqlite/sqlite.go:651) and so is
the engine's (upstream lifecycle.go:268); the adapter delegates. The
`journal.Heartbeat` constant exists ONLY tq-side (upstream's facts
package has no heartbeat type); its consumers (`tq doctor`
CountFacts(Heartbeat, 10m), `/health` worker-beat probe) read 0 today on
BOTH backends — identical behavior, no writer exists anywhere in tq
(verified: `rg 'journal.Heartbeat'` hits consumers only). Risk noted for
S2: if heartbeat observability is ever wanted, it must be added on both
sides or via the companion append path.

### D4 — Fact archive: not implemented (stub)

`facts_archive` / `journal_meta` (ADR-0006 prototype, tq-only) have no
upstream counterpart; sqlitev4 stubs return not-implemented errors and
`TestArchiveFactsBeforeKeepsProjections` skips with the divergence note.
Impact: hot journal grows unbounded on the adapter; the archive sweep has
no target. Same verdict as cqrsqlite. A decision-memo input (TODO row:
upstream-grown vs companion-table per surface).

### D5 — Payload storage class: BLOB vs TEXT (adapter-normalized)

Covered in the fix list: storage BLOB (upstream) vs TEXT (tq); read side
normalized via CAST for the query pushdown; empty payload = empty blob
(codec guard). Raw SQL tooling that greps payloads sees blobs; Go reads
are unaffected (scanned into string).

### D6 — Project exclusivity + resume-closeout requeue: implemented adapter-side

Not divergences in sqlitev4 (unlike cqrsqlite, which skips both):
`WithProjectExclusivity` is enforced by the adapter's own ClaimDue
predicate (adapter.go:355-373, D24 per-repo serialization) and `Requeue`
is adapter-owned writing tq's full `RequeueEvidence` incl.
`resume_closeout` (adapter.go:479-511). Both pinned by the copied suite
(`TestProjectExclusivitySerializesPerProject`,
`TestProjectExclusivityAcrossStoreHandles`,
`TestRequeueFactCarriesResumeCloseout` — all pass).

### Skip ledger (full)

| Test                                   | Why                                                                                                                    |
| -------------------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| TestArchiveFactsBeforeKeepsProjections | D4                                                                                                                     |
| TestEnqueueFactDetailCarriesIdentity   | D2                                                                                                                     |
| TestEnqueueClaimBaseline10k            | NOT a divergence — env-gated baseline (`TQ_BASELINE=1`), skips identically on tq's own suite (control run 16:2x, rc=0) |

## b) PARTIALLY DONE

1. **`last_error` reset divergence (D1) is documented, not fixed** — the
   adapter could post-clear it, but that means a second UPDATE outside
   the engine's tx; belongs in the decision memo (or an upstream fix).
   TODO row appended.
2. **Parity is suite-pinned, not exhaustively hand-verified** — the 72
   passing tests cover the tq contract surface; unpinned corners (D1's
   cross-process nuance, completed-fact detail byte-shapes) were verified
   by code reading and are labeled as such above.
3. **The split brain (cqrsqlite vs sqlitev4) persists** — this row's work
   landed on sqlitev4 (the module the TODO row describes: full extras +
   exclusivity + resume-closeout, all suite-pinned). Deleting the loser
   stays owner-gated (15-59 §g1).

## c) NOT STARTED (belongs to later rows)

Postgres spike (row 32), decision memo, replay tool, S1 flip, S2-S4 —
untouched.

## d) TOTALLY FUCKED UP

1. **I misread my own first suite run**: saw one failure in `tail -5`,
   concluded "only one failure", and wrote a fix for it before grepping
   the full log — the run had 42 failures. The CAST fix was correct but
   the diagnosis sequence was backwards (fix-first, evidence-later). The
   count comparison (`grep -c '^--- FAIL'` across both logs) is what
   exposed it. Claim discipline exists precisely for this.
2. **First build of the session ran without the env export** — no, it did
   not: exported `GOEXPERIMENT=jsonv2 GOTOOLCHAIN=auto` from the first
   go command (15-59 §d1's lesson taken). Recorded as a non-event so the
   next reader does not re-derive it.

## e) Verification battery (all rc captured this session)

- `internal/queue/sqlitev4`: gofmt clean; `go build`/`go vet` rc=0;
  `go test ./... -count=1` rc=0 (72 pass / 3 skip, skip ledger above);
  `go test ./... -count=1 -race` rc=0.
- Control: tq's own sqlite suite on the three divergent tests — archive +
  enqueued-detail PASS there (proving the skips are genuine divergences,
  not suite rot), baseline 10k SKIPs there too.
- Root gates re-run at wrap: root `go build ./...` + `go vet ./...` rc=0;
  docs gates (check-status-index, check-doc-refs, check-todo-list) rc=0.
