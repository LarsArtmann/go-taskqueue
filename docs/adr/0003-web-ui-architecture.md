# ADR-0003: Read-only live web UI over the facts projection

**Status:** Accepted (2026-09-07)
**Context:** The queue is only observable through the terminal (`tq stats`,
`tq top`, `tq tail -f`). During unattended agent-pool runs there is no single
place to watch the system live. The facts-first architecture (ADR-0001) means
queue views ARE projections — a web UI is a pure consumer of `Store.Facts`
and `Store.List` and needs zero store, schema, worker, or executor changes.

**Decision:**

1. **`tq serve` + `internal/webui`.** One subcommand, one new internal
   package. Examples stay PoCs. Read-only by construction: the UI can only
   render stale data, never mutate the journal. Write actions appear only
   behind a future explicit `--allow-writes` flag (Phase D).
2. **Architecture: tailer → Hub → SSE → server-rendered fragments.**

   ```text
   journal (SQLite, facts) ──► tailer (1 goroutine, poll Facts(after), 500ms)
                                   │ coalesced bursts
                                   ▼
                                 Hub (subscriber fan-out)
                                   │ per-client SSE (id = fact Seq)
                                   ▼
                        browser (vanilla JS EventSource)
                                   │ patch-by-id
                                   ▼
                        DOM fragments (server-rendered)
   ```

   **Projection, not client state:** on each coalesced burst the server
   re-queries `List` + status counts and re-renders every fragment; the
   client swaps `innerHTML` by container id. No missed-event drift, no
   client-side reducer. A full snapshot ships on (re)connect; SSE
   `Last-Event-ID` maps back to the journal watermark for resume
   (proven in `examples/sse`).

3. **Tech defaults** (each with an abort path that leaves this plan intact):

   | Decision    | Default                              | Why                                            | Abort path                                        |
   | ----------- | ------------------------------------ | ---------------------------------------------- | ------------------------------------------------- |
   | Rendering   | `templ` (generated `*_templ.go` committed) | House style (samber-do-auditlog, templ-components); typed fragments | `html/template` — plain string fragments, same container ids |
   | Fan-out     | `github.com/larsartmann/go-sse` `Broadcaster` | Own lib, pure Go, proven in auditlog; ring replay | Hand-rolled ~80-line hub (channels + mutex)       |
   | Client JS   | Vanilla (~30 lines: EventSource + patch + badge) | No 56 KB vendored runtime for one-way flow | Datastar upgrade when interactivity outgrows it   |
   | HTTP server | stdlib `http.ServeMux` + `http.Server` WITH Read/Header/Write/Idle timeouts | No framework needed; kills the G114 no-timeout class the PoCs carry | — |

   **Abort checkpoints:** if templ codegen fights CI/Nix (the auditlog v0.9.0
   retract class: generated `*_templ.go` MUST be committed or Nix vendor
   builds break) or go-sse proves non-CGO-free, switch to the documented
   abort path before proceeding — the fragment/SSE architecture is
   independent of both.

4. **Default bind `127.0.0.1:8090`;** flags `--addr --db --poll`. Localhost
   only until an explicit auth story (Phase D); multi-process SQLite access
   beside workers is already proven (WAL + `busy_timeout`, papdashboard
   bridge).

5. **Guardrails:** no store/schema/worker/executor changes, no migrations;
   single-writer `MaxOpenConns(1)` invariant untouched; examples/ stay PoC.

**Consequences:** one more dependency (templ runtime + go-sse); journal
`Facts` scan cost grows with the journal (compaction/pagination are later
concerns); the UI is a stale-page-worst-case consumer by design.

**Alternatives rejected:** WebSocket (bidirectional, no resume semantics we
need; SSE has `Last-Event-ID` built in); client-side framework (server
fragments + swap is the projection model, zero build step); polling-only
page (examples/api already proves it and it lags).
