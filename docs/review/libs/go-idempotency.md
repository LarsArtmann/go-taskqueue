# go-idempotency — Library Utilization Review

**Verdict:** NOT ADOPTED — KEEP OUT (re-affirmed 2026-11-22 against v0.4.0 `Forever`, see §3.2) — the journal's DedupKey already guarantees at-most-once enqueue at the only layer that matters (permanent domain keys with stored-task replay, enforced natively by the engines' partial unique index), and the go-cqrs-lite platform dedup this library defers to is already in the dependency tree.

**2026-11-11 refresh:** no data drift (still v0.3.2, HEAD 9a82577 unchanged; dedup/v4 still an indirect dep of exactly 12 go.mods) except one doc cite: AGENTS.md's idempotent-enqueue contract moved :162-164 → :166-168, updated in the three places below.

> **2026-11-22 refresh:** BIG library movement since 2026-11-11 — v0.4.0/v0.5.0/v0.6.0 all released 2026-11-17, plus the idemtest/v0.1.0→v0.1.1 train; HEAD now `idemtest/v0.1.1-4-g220c218` (2026-11-20), latest root tag v0.6.0 of 10 (12 tags incl. idemtest). v0.4.0 adds **permanent claims (`idempotency.Forever`, ADR-006)** — the capability this report's old "TTL-windowed request keys solve a different problem" argument leaned against, so the KEEP OUT is re-litigated below: it SURVIVES, because tq's at-most-once enqueue is already guaranteed natively by the engines' partial unique index (`idx_tasks_dedup ON tasks(dedup_key) WHERE dedup_key != ''` — vendored cqrs-lite sqlite v4 open.go:129, postgres v4 ddl.go:29), tq's guard RETURNS THE STORED TASK where the lib's claim store returns only a boolean (replay stays a consumer-built recipe, doc.go:351), and the COMPLETED done-guard (refuse + "edit the item" escape) is domain policy a claim store cannot express; ADR-0019's platform adoption already carries the cqrs-side dedup (dedup/v4, still indirect in exactly 12 go.mods — re-counted). v0.5.0 = `idemtest` consumer test kit as a nested module (ADR-007); v0.6.0 = injectable `MemoryStore` clock (`WithClock`) — neither creates a tq need. tq-side cites re-verified against HEAD 1c8219e6: Enqueue done-guard drifted to queue.go:603-613, AGENTS.md contract to :170-172, conform pins to tests.go:833-899/968. No score (fit report); verdict unchanged.

**Pinned:** none (not a dependency; verified 2026-11-08 by `grep -rn 'larsartmann/go-idempotency' --include=go.mod . | grep -v vendor` → no output, same for import greps; re-verified 2026-11-22 by the same greps — still 0 hits in go.mod and *.go, vendor excluded)

**Latest:** v0.6.0 (local checkout /home/lars/projects/go-idempotency, re-checked 2026-11-22: `git -C … tag -l | sort -V | tail -1` → v0.6.0; the nested `idemtest` module rides its own train at v0.1.1; HEAD `idemtest/v0.1.1-4-g220c218` 2026-11-20)

**Prior coverage:** none; AGENTS.md §Payload contracts documents the domain-level idempotent-enqueue contract this review compares against.

**Reviewed:** 2026-11-08 by reading /home/lars/projects/go-idempotency/{README.md,middleware,contract} plus internal/queue/queue.go, internal/httpapi/httpapi.go, internal/queue/companion/conform/tests.go, and go.mod greps for cqrs dedup.

## 1. Where the need exists today

go-idempotency offers: a three-method `Store` interface (`Seen`/`Record`/`CheckAndRecord` — the last required atomic), `ErrDuplicate`/`ErrInvalidTTL` sentinels classified via go-error-family, an 18-invariant contract test suite for custom backends (13 TTL invariants + 5 `Forever` invariants since v0.4.0), `middleware.Command`/`NewCommand`, a stdlib `middleware.HTTP` adapter (middleware/http.go:30), a deprecated dev-only `MemoryStore` (now with an injectable clock — v0.6.0's `WithClock`), the `idempotency.Forever` TTL sentinel for permanent claims (v0.4.0, ADR-006), and a separate nested consumer-test-kit module `idemtest` (v0.5.0, ADR-007) — deliberately **no production backends**; you implement the Store against Redis/SQL yourself (README:39–57, 158+, re-verified 2026-11-22).

tq already has the guarantee this library sells, at the domain layer and **permanent** rather than TTL-windowed:

- **Journal-level dedup**: `DedupKey` re-enqueue returns the stored task; a COMPLETED key's re-dispatch is refused with `ErrTaskDone`; cancelled/dead keys still suppress (AGENTS.md §Payload contracts, now at AGENTS.md:170-172 — re-verified 2026-11-22). Implementation: `Queue.Enqueue` (internal/queue/queue.go:603–613; `ErrTaskDone` sentinel at queue.go:49–57) returns `(stored, ErrTaskDone)` when the stored task is Completed. Beneath it, the storage engines enforce at-most-once enqueue NATIVELY via a partial unique index — `CREATE UNIQUE INDEX idx_tasks_dedup ON tasks(dedup_key) WHERE dedup_key != ''` (vendored go-cqrs-lite queue/sqlite/v4/open.go:129, queue/postgres/v4/ddl.go:29), with the enqueue re-checking the key inside the same transaction (sqlite/v4/enqueue.go:67). Cross-backend behavior is pinned by the conform suite (internal/queue/companion/conform/tests.go:833–899 — `TestEnqueueDedupKey`; :901 — `TestEnqueueWithoutDedupKeyIndependent`; :968 — legacy-schema migration; registered at suite.go:227–229).
- **HTTP passthrough**: `POST /api/v1/tasks` carries `dedupKey` in the body, passed verbatim into `task.New` (internal/httpapi/httpapi.go:304, 347) — the wire idempotency unit is the *domain object*, not the request.
- **Platform dedup already present**: `github.com/larsartmann/go-cqrs-lite/dedup/v4 v4.3.0` is an **indirect dependency in 12 go.mods** across the repo (root, cmd/tq, examples/embed, internal/{composition,worker,readmodel}, internal/queue/{sqlite,sqlitev4,postgres,postgresv4}, and the queue facades — verified by grep, 2026-11-08; re-counted 2026-11-22, still exactly 12). go-idempotency's own README says the go-cqrs-lite path is the engine capability: "On the go-cqrs-lite stack? You do not need to write a backend" — `metaengine.DedupStore` (its ADR-0142), with that repo's own idempotency stores retired in its favor (README:53).

## 2. Capability inventory (fit)

| Capability | Status | Evidence |
| --- | --- | --- |
| Atomic check-and-record key store | ⚪ N/A (ASSESSED against v0.4.0 `Forever` at the 2026-11-22 refresh: the lib can now back permanent keys, but the at-most-once enqueue it would guard is already enforced natively — partial unique index `idx_tasks_dedup`, sqlite v4 open.go:129 / postgres v4 ddl.go:29 — and the queue returns the stored row, not a boolean) | queue.go:603–613; conform/tests.go:833–899 |
| HTTP idempotency-key middleware | ⚪ N/A (tq's wire key names the task — stronger semantics: identical outcome, not identical request) | httpapi.go:304, 347 |
| Permanent completion guard (refuse re-dispatch of done work) | 🟡 solved better in-repo, RE-LITIGATED 2026-11-22: `Forever` (store.go:46-63, README:144/149) now gives the lib parity on *permanence*, but tq's guard additionally returns the stored task (response replay built-in) and encodes domain policy — COMPLETED refuses with the documented "edit the item text to re-key" escape (AGENTS.md:170-172) — a third state a claim store cannot express | queue.go:49–57, 603–613 vs lib's claim/boolean semantics |
| Contract test suite for a Store impl | ⚪ N/A (conform suite already pins enqueue dedup per backend; the lib's suite grew 13 → 18 invariants at v0.4.0 — 5 new `Forever` invariants, README "eighteen invariants") | internal/queue/companion/conform/suite.go:227–229 |
| TTL-windowed retry dedup for at-least-once consumers | 🔴 MISSING-BUT-RELEVANT (no such consumer: journal dispatcher uses durable watermarks, not redelivery) | internal/consumer per-subscriber cursors (ADR-0009) |

## 3. Findings (buy vs cost)

1. **Buy:** a rigorous, well-documented SDK for exactly-once command handling; the 18-invariant contract suite (13 TTL + 5 `Forever`, since v0.4.0) and the pitfalls section (TOCTOU, store-failure-is-not-permission, TTL sizing, Forever-at-the-boundary) are excellent reference material.
2. **Cost (dominant) — the double-idempotency question, re-answered no against `Forever` (2026-11-22):** v0.4.0's permanent claims remove the old TTL-expiry contrast — `idempotency.Forever` never re-allows the key, and ADR-006 scopes it to exactly tq's shape ("consumer/event dedupe and permanent domain invariants", README:144). But inserting the lib in front of `Queue.Enqueue` would still add a *second, weaker* layer: (a) the guarantee it would sell — at-most-once enqueue per permanent key — is already enforced natively by the engines' partial unique index inside the same database that holds the tasks (sqlite v4 open.go:129; postgres v4 ddl.go:29; no new store, no new failure mode, no TTL sizing); (b) the lib's `Store` is a boolean claim store — it cannot replay the original resource, which for enqueue IS the point (the journal returns the stored task so a retried dispatch learns the existing task id; the lib's own replay story is a consumer-built recipe, doc.go:351, 372); (c) tq's done-guard is richer than a permanent claim: COMPLETED keys refuse with `ErrTaskDone` while cancelled/dead keys still suppress-and-return for rescue flows, and the documented escape is editing the item text to mint a fresh key (AGENTS.md:170-172) — three states a claim/boolean store cannot express. Two layers would also split the truth: every divergence between claim store and journal becomes an operator puzzle.
3. **Cost:** a Store implementation tq would have to build and operate (the lib ships none by design): backed by SQLite? Then it duplicates the dedup table the queue engines already maintain via go-cqrs-lite's `dedup/v4` — the platform path the library itself redirects cqrs users to (README:53–57). MemoryStore is deprecated and single-process — useless for `tq api`, whose whole point is cross-process producers (httpapi.go:3–8).
4. **What adoption would even look like** (for the record — not recommended):

```go
// POST /api/v1/tasks — request-scoped Idempotency-Key header, ONLY if a
// producer cannot name a domain dedupKey. Layer 2 of 2; the journal stays layer 1.
var key string
if key = r.Header.Get("Idempotency-Key"); key != "" {
    if err := idem.CheckAndRecord(r.Context(), "idem:enqueue:"+key, 24*time.Hour); err != nil {
        if errors.Is(err, idempotency.ErrDuplicate) {
            writeError(w, http.StatusConflict, "duplicate request key", "reuse the stored task id from the first response")
            return
        }
        writeError(w, http.StatusServiceUnavailable, "idempotency store unavailable", "retry")
        return // store failure is not permission to process
    }
}
```

   The 409-with-no-stored-task hole (the lib cannot replay the original response — its README names this gap and doc.go:351/372 ships the response-replay *recipe* as consumer-built scaffolding) is exactly why the domain DedupKey, which returns the stored task (queue.go:609-611), is the better primitive.

## 4. Fit assessment

This remains the clearest KEEP OUT of the batch, re-examined 2026-11-22 against v0.4.0's `Forever`: the need the library exists to fill — turning at-least-once submission into at-most-once effect — is already guaranteed by the facts-first journal with strictly stronger semantics (permanent keys enforced by the engines' own partial unique index, stored-task replay, explicit done-guard with escape hatch). `Forever` closes the lib's permanence gap but not the replay or policy gaps, and the layer it would add duplicates a guarantee the storage engines already enforce in the same database. The library's own documentation points go-cqrs-lite users at the platform dedup capability that tq already transitively depends on (12 go.mods) and that ADR-0019 made the load-bearing platform decision (its `Store[T]` contract — spec-donored by tq, "dedup-keyed enqueue" held by the shared conformance suite, docs/adr/0019-go-cqrs-lite-platform-adoption.md:26-29). An HTTP-level layer would be redundant where correct and divergent where it matters, and would introduce a new store to back, monitor, and size TTLs for. The only future trigger would be a consumer-facing API accepting *arbitrary* retried commands with no natural domain key — nothing in ADR-0008 or the roadmap proposes that.

## 5. Maturity + version currency

Latest v0.6.0 of 10 root tags + the idemtest train at v0.1.1 (12 tags total; re-checked 2026-11-22: `git -C /home/lars/projects/go-idempotency tag -l | sort -V | tail -1` → v0.6.0); HEAD `idemtest/v0.1.1-4-g220c218` 2026-11-20 — three releases shipped 2026-11-17 alone, actively maintained. MIT (LICENSE). Single runtime dependency: go-error-family v0.12.0 (go.mod:6) — the same version cmd/tq already carries indirectly (cmd/tq/go.mod:77, re-verified 2026-11-22). Go floor 1.27 (root and idemtest go.mod:3); well-tested (200-goroutine atomicity, property + fuzz suites — a fifth clock-jump fuzz target joined at idemtest/v0.1.1 — and the self-tested 18-invariant contract suite).

## 6. Conclusion

Keep out — re-affirmed 2026-11-22 after the v0.4.0 `Forever` re-litigation: tq's idempotency lives in the journal where it belongs, backed natively by the engines' partial unique index; adopting a claim-store layer (TTL or Forever) would add a weaker duplicate guarantee beneath a stronger one and a store nobody needs to operate. Keep the library bookmarked as the reference for any future at-least-once *ingestion* surface that lacks a domain key.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
grep -rn 'larsartmann/go-idempotency' --include=go.mod --include='*.go' . | grep -v vendor   # → no output (re-run 2026-11-22)
grep -rn 'go-cqrs-lite/dedup/v4' --include=go.mod . | grep -v vendor | wc -l # 12 modules already carry platform dedup
sed -n '49,57p;603,613p' internal/queue/queue.go                        # ErrTaskDone + done-guard Enqueue
sed -n '170,172p' AGENTS.md                                              # the idempotent-enqueue contract
sed -n '833,899p' internal/queue/companion/conform/tests.go              # dedup pinned cross-backend
sed -n '227,229p' internal/queue/companion/conform/suite.go              # suite registration of the dedup pins
grep -n 'idx_tasks_dedup' vendor/github.com/larsartmann/go-cqrs-lite/queue/sqlite/v4/open.go vendor/github.com/larsartmann/go-cqrs-lite/queue/postgres/v4/ddl.go  # → :129 / :29 native unique index
grep -n 'DedupKey' internal/httpapi/httpapi.go                           # wire passthrough (304, 347)
git -C /home/lars/projects/go-idempotency describe --tags                # idemtest/v0.1.1-4-g220c218
git -C /home/lars/projects/go-idempotency tag -l | sort -V | tail -1     # v0.6.0 (root train); idemtest rides v0.1.x
head -3 /home/lars/projects/go-idempotency/LICENSE                       # MIT
```
