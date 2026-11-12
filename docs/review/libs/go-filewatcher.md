# go-filewatcher — Library Utilization Review

**Verdict:** NOT ADOPTED — KEEP OUT — tq has zero local-filesystem watch sites: every live-update path is journal-polling, daemon-SSE, or interval+watermark driven, so there is nothing for an fsnotify wrapper to watch.

**2026-11-11 refresh:** no new tag (still v2.5.0, HEAD v2.5.0-4-g9f44dee unchanged since 2026-11-08); fixed drifted internal/harvest/watch.go line cites — the file condensed since 2026-11-08, so the "purely additive" quote moved :84-85 → :75-77, the Watcher struct :85 → :78, the connect loop :113 → :110, and WatchConfig.RepoIntervals :66-71 → :58-63.

**Pinned:** none (not a dependency; verified 2026-11-08 by `rg -i 'go-filewatcher' -g 'go.mod' -g 'go.sum' -g '!vendor/**' .` → no output, rc=1; same for `--glob '*.go'` import greps; re-verified 2026-11-11 by the same greps — still 0 hits in go.mod and *.go, vendor excluded)

**Latest:** v2.5.0 (local checkout /home/lars/projects/go-filewatcher, checked 2026-11-08; HEAD = v2.5.0-4-g9f44dee, docs-only past the tag)

**Prior coverage:** none (docs/review/libs/ has no watcher entry; AGENTS.md records no watching decision).

**Reviewed:** 2026-11-08 by reading /home/lars/projects/go-filewatcher/{README.md,go.mod,watcher.go,watcher_poll.go,options.go,backend.go,debouncer.go,errors.go} plus internal/webui/tailer.go, internal/webui/hub.go, internal/harvest/watch.go, internal/harvest/discovery.go, internal/docsgates/bridge.go, cmd/tq/main.go (`tq tail`); greps in §How to verify.

## 1. Where the need exists today

go-filewatcher wraps fsnotify with: automatic recursion, 20 built-in filters + AND/OR/NOT composition, 19 middleware, global+per-path debounce, .gitignore awareness, ENOSPC resilience/self-healing, and an NFS/FUSE polling fallback (README:13-30; watcher.go:79-90, options.go:204-215).

The places tq updates "live", and how they actually work — none is a filesystem watch:

- **Dashboard feed** (`tq serve`): the webui "tailer" polls the JOURNAL, not files — `store.Facts(ctx, watermark, 1000)` every `cfg.Poll`, notifying the go-sse hub per batch (internal/webui/tailer.go:13-52; hub.go:11-14). With `--read-model` the ADR-0019 pump replaces it: every folded ledger update wakes the hub (tailer.go:59-75).
- **Harvest triggers**: `internal/harvest/watch.go` subscribes to project-discovery-daemon's `GET /v1/watch` SSE stream over a unix socket (Watcher, watch.go:78-107; connect loop with backoff, :110-148; hand-rolled WHATWG-subset SSE parse, :204-247; unix-socket client, discovery.go:171-204). The daemon pushes project add/change events; the interval tick stays the fallback heartbeat — "purely additive: a dead or missing watch stream never stalls the pool" (watch.go:75-77). Local TODO_LIST.md changes reach tq through the daemon or the interval, by design.
- **Sweepers** (review, status, docsgates, harvest): interval loops + the durable journal watermark (internal/watermark/watermark.go), not file events.
- **`tq tail -f`**: follows the journal (`fs.Bool("f", …)` over `store.Facts`, cmd/tq/main.go:3949+), not a log file.
- **Config hot-reload**: none — flags resolve once via `config.FromFlags` (ADR-0022: one deployment description), so there is no config file to watch.

## 2. Capability inventory (fit)

| Capability | Status | Evidence |
| --- | --- | --- |
| Recursive fsnotify watching | ⚪ N/A — no code path watches the filesystem; `rg -i 'fsnotify\|inotify'` over non-vendor `*.go` returns only the readmodel's metaengine `Watcher` (an in-process change feed, not fsnotify) | readmodel/model.go:123,278; grep in §How to verify |
| Debounce (global + per-path) | 🟡 partial overlap — harvest already owns per-repo debouncing keyed by `--repo-interval` | harvest/watch.go:305 (`debounceElapsed`), WatchConfig.RepoIntervals :58-63 |
| .gitignore-aware filters | ⚪ N/A — no directory scanning consumer | — |
| ENOSPC resilience / self-healing watches | ⚪ N/A — no inotify watch limits in play | watcher.go:90,130 |
| NFS/FUSE polling fallback | ⚪ N/A — no watched path, and the journal lives on SQLite (busy_timeout/WAL own its IO story) | options.go:204-215 |
| `GOEXPERIMENT=jsonv2` requirement | compatible — ci-local already exports jsonv2 (AGENTS.md GOEXPERIMENT/GOTOOLCHAIN) | go-filewatcher README:56-61 |
| Pure Go, `CGO_ENABLED=0` | compatible — fsnotify/gogenfilter/go-gitignore/x-text/x-time are pure Go | go-filewatcher/go.mod |

## 3. Findings (buy vs cost)

1. **Buy (hypothetical only):** if the pool ever needed daemon-free local TODO_LIST.md reactivity, `filewatcher.New(projectsDir, WithDebounce(...), WithIgnoreDirs(".git"))` would replace a hand-rolled mtime-poll loop. That loop does not exist: the daemon watch stream (watch.go) already provides sub-second project-change triggers with graceful degradation, and the harvest interval owns progress when it is down.
2. **Cost:** five direct deps (fsnotify, gogenfilter, sabhiram/go-gitignore, x/text, x/time) plus indirects into the vendor tree of a multi-module repo that must resync `vendor/` per module and re-pin `vendorHash` in flake.nix (AGENTS.md: VendorHash drift).
3. **Cost:** a watcher is a new always-on failure surface (inotify limits, missed events, restart races) for a process whose correctness model is facts + watermarks: a missed event can never lose work, only delay it to the next interval tick — which is exactly the guarantee the current design leans on (watch.go:75-77).
4. **Adoption site if it ever happened** (daemon-less harvest trigger; not recommended):

```go
w, err := filewatcher.New([]string{projectsDir},
    filewatcher.WithExtensions(".md"),          // TODO_LIST.md only
    filewatcher.WithDebounce(2*time.Second),
    filewatcher.WithIgnoreDirs(".git", "vendor"),
)
```

## 4. Fit assessment

go-filewatcher is a serious fsnotify wrapper, but its subject — the local filesystem — is not a tq input surface. The queue's truth is the SQLite journal; its live UI polls that journal; its harvest triggers arrive as daemon SSE events; its sweepers are watermark-paced. Every "watching" need in the repo decomposes into one of those three, each with an owned, tested implementation (tailer under internal/webui tests, harvest watch_test.go). Introducing an fsnotify dependency would add machinery whose only possible job — noticing file mtime changes nobody consumes — has no caller.

## 5. Maturity + version currency

Latest tag v2.5.0 (2026-11-06); 15 root tags since v0.1.0 (2026-04-20) — the oldest library in this batch; last commit 9f44dee 2026-11-06 (docs: backlog ranking — housekeeping, not feature work); checkout v2.5.0-4-g9f44dee. MIT; race-tested per README badge. Go floor 1.26.7 with a jsonv2 build requirement — compatible with tq's 1.27.3 + jsonv2 export.

## 6. Conclusion

Keep out. There is no filesystem-watch site today and the architecture deliberately routes reactivity through the journal and the discovery daemon instead. Revisit only if a future feature consumes local file events directly (e.g. watching `.tq/verdicts/` marker files — but those are already read by harvest sweeps at verdicts.go:124-147); nothing on ROADMAP.md does.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
rg -i 'go-filewatcher' -g 'go.mod' -g 'go.sum' -g '!vendor/**' .          # → no output (rc=1)
rg -i 'go-filewatcher' --glob '*.go' --glob '!vendor/**' .                 # → no output (rc=1)
rg -n -i 'fsnotify|inotify\(' --glob '*.go' --glob '!vendor/**' --glob '!*_test.go' internal/ cmd/
# → no output (rc=1); the readmodel "Watcher" hits are the metaengine change feed, not fsnotify
sed -n '13,52p' internal/webui/tailer.go        # journal poll, not file tail
sed -n '75,107p' internal/harvest/watch.go      # daemon SSE watcher, interval fallback
sed -n '3949,3952p' cmd/tq/main.go              # tq tail -f: journal follow
git -C /home/lars/projects/go-filewatcher describe --tags  # → v2.5.0-4-g9f44dee
git -C /home/lars/projects/go-filewatcher tag -l | grep -v / | wc -l      # → 15
```
