# go-daemon — Library Utilization Review

**Verdict:** NOT ADOPTED — KEEP OUT — tq's long-runners are foreground HTTP servers whose lifecycle `internal/runactor` already owns (ADR-0004's framework-free stance); go-daemon's socket-server machinery is for a serving shape tq doesn't have, and its one genuinely attractive primitive (spec-pinned SSE parsing) covers 40 already-tested lines.

**2026-11-11 refresh:** no new tag (still v0.4.0); HEAD advanced e8e1a1f → 82bb135 (2026-11-10, auto-commit housekeeping); refreshed the webui drain cite — internal/webui/webui.go moved since 2026-11-08: `Server.Run` is now :500, `ListenAndServe` :559, `Shutdown` drain :579.

**Pinned:** none (not a dependency; verified 2026-11-08 by `rg -i 'go-daemon' -g 'go.mod' -g 'go.sum' -g '!vendor/**' .` → no output, rc=1; same for `--glob '*.go'` import greps; re-verified 2026-11-11 by the same greps — still 0 hits in go.mod and *.go, vendor excluded)

**Latest:** v0.4.0 (local checkout /home/lars/projects/go-daemon, checked 2026-11-11; HEAD = v0.4.0-33-g82bb135)

**Prior coverage:** none (docs/review/libs/ has no daemon entry; AGENTS.md records no daemon-library decision).

**Reviewed:** 2026-11-08 by reading /home/lars/projects/go-daemon/{README.md,go.mod,doc.go,socket.go,notify.go,negotiation.go,sse.go,flightrecorder.go} plus internal/runactor/runactor.go, internal/webui/webui.go, internal/httpapi/httpapi.go, internal/harvest/{watch.go,discovery.go}, cmd/tq/bootstrap.go (systemd unit template); greps in §How to verify.

## 1. Where the need exists today

go-daemon is NOT a fork/daemonize library (no fork/setsid/supervise anywhere — verified by `rg -i 'fork|daemonize|setsid|supervis' *.go` → no hits). It is building blocks for unix-socket daemons: `SocketServer` lifecycle (stale-socket cleanup, 0600 perms, drain, ordered Lifecycle hooks), `sd_notify` READY/STOPPING/Watchdog, JSON/CBOR content negotiation, WHATWG-conformance `ParseSSEData`, a unix-socket HTTP client, and a flight recorder (README:19-39; socket.go:158-237).

The tq code it would touch:

- **`tq serve` / `tq api` lifecycle**: both are foreground `net/http` servers — `ListenAndServe` under an `internal/runactor` group (internal/webui/webui.go:500 `Server.Run`, ListenAndServe at :559, Shutdown drain at :579; internal/httpapi/httpapi.go:515 `Server.ListenAndServe`). `runactor.Group` owns named actors, LIFO teardown, and the SIGINT/SIGTERM interrupt story (runactor.go:1-14, ADR-0004). Only cmd/tq/main.go uses it.
- **Process supervision**: systemd — `tq bootstrap --install` renders the unit (cmd/tq/bootstrap.go:39-60, `systemdUnitTemplate` embedded from deploy/systemd/): default `Type=simple`, `KillSignal=SIGINT`, `Restart=on-failure`, `TimeoutStopSec=45min`. No `Type=notify`, so sd_notify has no consumer; readiness is systemd's process-alive heuristic, deliberately.
- **The one real overlap — SSE client parsing**: `internal/harvest/watch.go` hand-rolls a WHATWG-subset SSE parser for the daemon's `/v1/watch` stream (scanWatchStream, watch.go:204-247: blank-line dispatch, comment/heartbeat skip, one-space strip, 1 MiB scan cap), plus a tuned unix-socket HTTP client for it (discovery.go:171-204: `daemonHTTPClient`/`unixSocketClient`, `ResponseHeaderTimeout` because `Client.Timeout` would kill the stream).

## 2. Capability inventory (fit)

| Capability | Status | Evidence |
| --- | --- | --- |
| `SocketServer` unix-socket lifecycle (stale cleanup, hooks, drain) | ⚪ N/A — tq serves TCP loopback (`--addr 127.0.0.1:…`), no unix-socket server exists | webui.go:559, httpapi.go:515 |
| sd_notify READY/STOPPING/Watchdog | ⚪ N/A — the deployed unit is default Type=simple with KillSignal=SIGINT; runactor answers signals directly | bootstrap.go:39-60, runactor.go:1-14 |
| Supervision/forking | 🔴 MISSING from the library itself — it is not a daemonizer; supervision stays systemd's job, which is the documented deployment (README "systemd-adjacent") | `rg -i 'fork\|daemonize'` over checkout → no hits |
| `ParseSSEData` (WPT/Chromium-corpus SSE parsing) | 🟡 partial overlap — would replace harvest's 40-line scanWatchStream, which is already pinned by internal/harvest/watch_test.go | watch.go:204-247; go-daemon README:31 |
| `NewHTTPClientNoTimeout` (unix-socket transport for SSE) | 🟡 partial overlap — `unixSocketClient` already solves the no-`Client.Timeout`-for-streams problem with a documented rationale | discovery.go:186-204 |
| JSON/CBOR negotiation | ⚪ N/A — both surfaces speak JSON only (jsonv2); the API's contract is 4 fixed routes | httpapi.go:193-201 |
| FlightRecorder + Prometheus | ⚪ N/A — tq has no metrics endpoint; health/alerting ride go-health-dashboard and the PapDashboard bridge | AGENTS.md §Relation (go-health-dashboard ADOPTED) |
| Pure Go, `CGO_ENABLED=0` | compatible — go-systemd, go-codec, sseparse, prometheus client are pure Go | go-daemon/go.mod |

## 3. Findings (buy vs cost)

1. **Buy (narrow):** `ParseSSEData` is conformance-pinned to the WPT/Chromium corpus — stronger than harvest's hand-rolled subset. But the subset is exactly matched to the daemon's wire format (data-only frames, heartbeat comments), tested (watch_test.go), and 40 lines; the conformance edge buys nothing against a single well-known peer implementation.
2. **Buy (hypothetical):** if `tq api` ever moved to a unix socket (removing the loopback TCP port), `SocketServer` would hand tq stale-socket cleanup and permissioning. No such plan exists; the loopback default is a documented security posture (AGENTS.md: `tq serve`/`tq api` security).
3. **Cost (dominant):** +4 direct deps (go-systemd, go-codec, go-flightrecorder, go-sse/sseparse, prometheus/client_golang) into a curated dependency tree, to wrap two servers that runactor already drains correctly under signal-driven shutdown — the exact concern ADR-0004 settled with "patterns now, dependencies later" (ADR-0004 Decisions:1).
4. **Adoption site if the SSE primitive were ever taken alone** (the only defensible slice — and still not recommended while the subset passes its tests):

```go
// internal/harvest/watch.go — scanWatchStream's dispatch half
for {
    data, ok, err := godmn.ParseSSEData(r) // replaces the bufio.Scanner loop
    ...
}
```

## 4. Fit assessment

go-daemon targets a serving shape tq deliberately does not have: a unix-socket daemon supervised via sd_notify under `Type=notify`. tq's long-runners are TCP-loopback HTTP servers owned by runactor groups inside a systemd `Type=simple` unit whose stop semantics (SIGINT + 45-minute drain) encode the agent-pool's real requirement — long in-flight tasks — not fast readiness flips. The library's genuinely overlapping primitives (SSE parse, unix-socket client) are already implemented, documented, and tested in internal/harvest against the exact peer (project-discovery-daemon) the library itself was extracted from. Adopting would trade ~60 owned lines for a five-dependency lifecycle framework with no lifecycle gap to fill.

## 5. Maturity + version currency

Latest tag v0.4.0 (2026-10-03); 5 root tags since v0.1.0 (2026-09-28) — young but production-extracted (project-discovery-daemon lineage); last commit 82bb135 2026-11-10 (auto-commit housekeeping; re-verified 2026-11-11); checkout v0.4.0-33-g82bb135 — 33 commits past the tag, actively developed (v5-scaffold-adjacent work in sibling repos). MIT. Go floor 1.27.2 — compatible.

## 6. Conclusion

Keep out. Lifecycle is a solved, ADR-pinned concern (runactor + systemd unit), and the single overlapping seam — SSE client parsing — is 40 tested lines inside harvest. If tq ever grows a unix-socket serving surface or switches the pool unit to `Type=notify`, revisit `SocketServer`/`NotifyReady` specifically; neither is on ROADMAP.md.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
rg -i 'go-daemon' -g 'go.mod' -g 'go.sum' -g '!vendor/**' .            # → no output (rc=1)
rg -i 'go-daemon' --glob '*.go' --glob '!vendor/**' .                   # → no output (rc=1)
rg -n -i 'sd_notify' --glob '*.go' --glob '!vendor/**' --glob '!*_test.go' .  # → no output (rc=1)
sed -n '1,14p' internal/runactor/runactor.go    # runactor owns the lifecycle story
sed -n '39,60p' cmd/tq/bootstrap.go             # systemd unit: simple default, KillSignal=SIGINT
sed -n '204,247p' internal/harvest/watch.go     # hand-rolled SSE parse (the overlap)
rg -n -i 'fork|daemonize|setsid|supervis' /home/lars/projects/go-daemon/*.go  # → no daemonizer
git -C /home/lars/projects/go-daemon describe --tags   # → v0.4.0-33-g82bb135
git -C /home/lars/projects/go-daemon tag -l | grep -v / | wc -l          # → 5
```
