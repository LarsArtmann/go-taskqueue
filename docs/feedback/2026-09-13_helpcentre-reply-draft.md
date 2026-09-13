# Help Centre reply draft (owner sends)

_Re: "Can we learn from go-taskqueue? or use it?" — reply via the private
evaluator channel (no public tracking issue: their identity is confidential)._

---

Your feedback file landed exactly as written and drove the change: v0.3.0
(cut 2026-09-13) ships seven public facade modules, and they are live on
the module proxy now —

```sh
go get github.com/larsartmann/go-taskqueue/queue/postgres@v0.3.0
```

(resolves with its full internal graph; all seven — `task`, `journal`,
`queue`, `queue/sqlite`, `queue/postgres`, `executor`, `worker` — list
v0.3.0 via `go list -m -versions`).

Item by item:

1. **Facade per consumer-facing contract** — shipped (ADR-0016). Type
   aliases, not wrappers: `queue.Store` IS the store, both backends and
   the worker interoperate with zero glue. Implementations stay internal
   and refactor freely.
2. **Promotion: move, don't rename** — recorded in the ADR as the
   stabilization path; facades keep resolving either way.
3. **References doc for the single-job-type profile** — shipped at
   docs/references/single-job-type-queue-profile.md.
4. **Constructor taking an existing `*pgxpool.Pool`** — shipped:
   `postgres.OpenWithPool(ctx, pool)`. The pool stays yours — `Close`
   does not tear it down (we proved that against a live cluster before
   cutting the release; the first live run caught the opposite behavior
   and it was fixed pre-tag).
5. **README consumer status line** — shipped near the top.

Verified from a clean room: proxy-only resolution (no replaces), a
consumer doing enqueue → deliberate failure → retry → complete against
both sqlite and a live Postgres cluster, through the public imports.

One limitation, disclosed rather than discovered: the `tq` CLI binary
cannot be `go install`ed from the proxy (the root module carries
dev-time replace directives; Go refuses @version installs of replaced
modules). Library consumption is unaffected. Clone + `go build ./cmd/tq`
or nix work; a proper fix is tracked.

Your offer stands open and accepted: #3 is the tracking issue — the port
diff of your ~500-LOC fork against the profile doc is exactly the review
input we want, and the conformance suite for the Postgres path is ready
to run against your workload shape.

💘 Generated with Crush
