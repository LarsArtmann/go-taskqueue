# go-idempotency — Library Utilization Review

**Verdict:** NOT ADOPTED — KEEP OUT — the journal's DedupKey already guarantees at-most-once enqueue at the only layer that matters (permanent domain keys, not TTL-windowed request keys), and the go-cqrs-lite platform dedup this library defers to is already in the dependency tree.

**2026-11-11 refresh:** no data drift (still v0.3.2, HEAD 9a82577 unchanged; dedup/v4 still an indirect dep of exactly 12 go.mods) except one doc cite: AGENTS.md's idempotent-enqueue contract moved :162-164 → :166-168, updated in the three places below.

**Pinned:** none (not a dependency; verified 2026-11-08 by `grep -rn 'larsartmann/go-idempotency' --include=go.mod . | grep -v vendor` → no output, same for import greps; re-verified 2026-11-11 by the same greps — still 0 hits in go.mod and *.go, vendor excluded)

**Latest:** v0.3.2 (local checkout /home/lars/projects/go-idempotency, checked 2026-11-08)

**Prior coverage:** none; AGENTS.md:166–168 documents the domain-level idempotent-enqueue contract this review compares against.

**Reviewed:** 2026-11-08 by reading /home/lars/projects/go-idempotency/{README.md,middleware,contract} plus internal/queue/queue.go, internal/httpapi/httpapi.go, internal/queue/companion/conform/tests.go, and go.mod greps for cqrs dedup.

## 1. Where the need exists today

go-idempotency offers: a three-method `Store` interface (`Seen`/`Record`/`CheckAndRecord` — the last required atomic), `ErrDuplicate`/`ErrInvalidTTL` sentinels classified via go-error-family, a 13-invariant contract test suite for custom backends, `middleware.Command`/`NewCommand`, a stdlib `middleware.HTTP` adapter (middleware/http.go:30), and a deprecated dev-only `MemoryStore` — deliberately **no production backends**; you implement the Store against Redis/SQL yourself (README:39–53, 151–166).

tq already has the guarantee this library sells, at the domain layer and **permanent** rather than TTL-windowed:

- **Journal-level dedup**: `DedupKey` re-enqueue returns the stored task; a COMPLETED key's re-dispatch is refused with `ErrTaskDone`; cancelled/dead keys still suppress (AGENTS.md:166–168, re-verified 2026-11-11). Implementation: `Queue.Enqueue` (internal/queue/queue.go:595–605) returns `(stored, ErrTaskDone)` when the stored task is Completed; sentinel at queue.go:49–57. Cross-backend behavior is pinned by the conform suite (internal/queue/companion/conform/tests.go:793–859 — `TestEnqueueDedupKey`, `TestEnqueueWithoutDedupKeyIndependent`, legacy-schema migration).
- **HTTP passthrough**: `POST /api/v1/tasks` carries `dedupKey` in the body, passed verbatim into `task.New` (internal/httpapi/httpapi.go:304, 341–348) — the wire idempotency unit is the *domain object*, not the request.
- **Platform dedup already present**: `github.com/larsartmann/go-cqrs-lite/dedup/v4 v4.3.0` is an **indirect dependency in 12 go.mods** across the repo (root, cmd/tq, examples/embed, internal/{composition,worker,readmodel}, internal/queue/{sqlite,sqlitev4,postgres,postgresv4}, and the queue facades — verified by grep, 2026-11-08; re-verified 2026-11-11, still exactly 12). go-idempotency's own README says the go-cqrs-lite path is the engine capability: "On the go-cqrs-lite stack? You do not need to write a backend" — `metaengine.DedupStore` (its ADR-0142), with that repo's own idempotency stores retired in its favor (README:53–57).

## 2. Capability inventory (fit)

| Capability | Status | Evidence |
| --- | --- | --- |
| Atomic check-and-record key store | ⚪ N/A (store-level dedup already enforced: unique constraint + conform tests) | queue.go:595–605; conform/tests.go:793–859 |
| HTTP idempotency-key middleware | ⚪ N/A (tq's wire key names the task — stronger semantics: identical outcome, not identical request) | httpapi.go:304, 341–348 |
| Permanent completion guard (refuse re-dispatch of done work) | 🟡 solved better in-repo (permanent key + explicit ErrTaskDone escape hatch "edit the item") | queue.go:49–57 vs lib's TTL expiry that RE-ALLOWS the same key |
| Contract test suite for a Store impl | ⚪ N/A (conform suite already pins enqueue dedup per backend) | internal/queue/companion/conform/suite.go:227–229 |
| TTL-windowed retry dedup for at-least-once consumers | 🔴 MISSING-BUT-RELEVANT (no such consumer: journal dispatcher uses durable watermarks, not redelivery) | internal/consumer per-subscriber cursors (ADR-0009) |

## 3. Findings (buy vs cost)

1. **Buy:** a rigorous, well-documented SDK for exactly-once command handling; the 13-invariant contract suite and the pitfalls section (TOCTOU, store-failure-is-not-permission, TTL sizing) are excellent reference material.
2. **Cost (dominant) — the double-idempotency question, answered no:** inserting a TTL key store in front of `Queue.Enqueue` would add a *second, weaker* layer: its claims expire (README "Common pitfalls": TTL window, crash-between-claim-and-effect leaves the key claimed until expiry), whereas the journal's keys never expire and the COMPLETED refusal is a *feature* (ErrTaskDone, with the documented escape of editing the item text to re-key — AGENTS.md:166–168). Two layers also split the truth: a TTL layer could reject a retry the journal would have accepted (or vice versa), and every divergence becomes an operator puzzle.
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

   The 409-with-no-stored-task hole (the lib cannot replay the original response — its README names this gap) is exactly why the domain DedupKey, which returns the stored task (queue.go:599–601), is the better primitive.

## 4. Fit assessment

This is the clearest KEEP OUT of the batch: the need the library exists to fill — turning at-least-once submission into at-most-once effect — is already guaranteed by the facts-first journal with strictly stronger semantics (permanent keys, stored-task replay, explicit done-guard). The library's own documentation points go-cqrs-lite users at the platform dedup capability that tq already transitively depends on (12 go.mods). An HTTP-level layer would be redundant where correct and divergent where it matters, and would introduce a new store to back, monitor, and size TTLs for. The only future trigger would be a consumer-facing API accepting *arbitrary* retried commands with no natural domain key — nothing in ADR-0008 or the roadmap proposes that.

## 5. Maturity + version currency

Latest v0.3.2 of 7 tags (checked 2026-11-08: `git -C /home/lars/projects/go-idempotency tag -l | sort -V | tail -1`; re-verified 2026-11-11 — unchanged, no tag newer than v0.3.2); last commit 9a82577 2026-11-04 (still HEAD on 2026-11-11) — active days before this review. MIT (LICENSE). Single runtime dependency: go-error-family v0.12.0 — the same version cmd/tq already carries indirectly (cmd/tq/go.mod:77, re-verified 2026-11-11). Go floor 1.27; well-tested (200-goroutine atomicity, property + fuzz suites, self-tested contract suite).

## 6. Conclusion

Keep out. tq's idempotency lives in the journal where it belongs; adopting an HTTP TTL layer would add a weaker duplicate guarantee beneath a stronger one and a store nobody needs to operate. Keep the library bookmarked as the reference for any future at-least-once *ingestion* surface that lacks a domain key.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
grep -rn 'larsartmann/go-idempotency' --include=go.mod . | grep -v vendor   # → no output
grep -rn 'larsartmann/go-idempotency' --include='*.go' . | grep -v vendor   # → no output
grep -rn 'go-cqrs-lite/dedup/v4' --include=go.mod . | grep -v vendor | wc -l # 12 modules already carry platform dedup
sed -n '49,57p;595,605p' internal/queue/queue.go                            # ErrTaskDone + done-guard Enqueue
sed -n '793,859p' internal/queue/companion/conform/tests.go                 # dedup pinned cross-backend
grep -n 'DedupKey' internal/httpapi/httpapi.go                              # wire passthrough (304, 347)
git -C /home/lars/projects/go-idempotency tag -l | sort -V | tail -1        # v0.3.2
head -3 /home/lars/projects/go-idempotency/LICENSE                          # MIT
```
