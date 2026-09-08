# Cordis Go Port — Local Test-Suite Verification (ADR-0004 T3 Input)

**Date:** 2026-09-08
**Status:** DONE (verification record; no code changed in either repo)
**Scope:** Fulfill the TODO_LIST "Medium Impact" item: run the cordis Go
test suite locally (`GOCACHE=/tmp/gocache`) and record verified
coverage/race results as the real input to the plugin-era decision —
replacing the vendor README claim (fork `README.md:37`: "race-tested,
~85% coverage") that the 05:24 report carried unlabeled (items b/d2) and
ADR-0004 T3 cited as never-run.
**Inputs:** fork `/home/lars/forks/cordis` at `61ec9f9` (`git describe`:
`v4.0.0-rc.9-48-g61ec9f9`, HEAD dated 2026-09-08 05:57 — the exact commit
ADR-0004's cross-check ledger recorded, so these results apply to the
state the ADR assessed), Go module `go/` (`github.com/LarsArtmann/cordis/go`,
`go 1.27`, five packages: root, `group`, `hmr`, `loader`, `timer`).

---

## Verdicts up front

1. **The vendor claim verifies, slightly exceeded.** Race-clean
   (zero `WARNING: DATA RACE`, zero panics) at **86.2% total statement
   coverage** (atomic mode), against the README's "race-tested, ~85%
   coverage". 176 passing test cases (151 top-level functions + 25
   subtests), 0 failures, 0 skips, across all 5 packages; every package
   `ok` in ~1.0–1.1s. The four golden-scenario tests ran and passed
   (`GOLDEN_UPDATE` unset, i.e. compare mode).
2. **ADR-0004 T3 is now satisfied** ("maturity verified locally"). The
   adoption verdict does NOT move: T1 (plugin-era go/no-go — not started),
   T2 (stable v1-class release — fork still pre-stability at
   `v4.0.0-rc.9`+48, CHANGELOG latest `0.1.0`), T4 (drain-invariant
   prototype behind a build tag), and T5 (framework-independent plugin
   API) all remain open. Framework-free stands; this run only replaces a
   vendor claim with a verified number in the gate table.
3. **The caveat that survives verification:** coverage is uneven. The
   `loader` package sits at 74.6% with a run of 0%-covered functions in
   its resolver/watcher/tree surface (`loader/resolver.go`,
   `loader/tree.go`, `loader/group.go`, `loader/entry.go` error paths —
   file-watching and hot-reload edges); root `context.go:49 Root` and
   `events.go:87 Error` are also 0%. If a plugin-era evaluation ever leans
   on HMR/live-reload behavior, the 74–90% per-package band — not the
   86.2% headline — is the number that matters. Benchmarks
   (`go/bench_test.go`) were not run: `-bench` was absent and throughput
   is not part of the claim being verified.

## Environment and exact commands (reproducibility)

- Toolchain: `go1.27.1 linux/amd64`. The module requires `go 1.27`; the
  system default is `go1.26.7` with `GOTOOLCHAIN=local` (which would
  refuse to run), so the run forces `GOTOOLCHAIN=go1.27.1` — that
  toolchain is already cached in `GOMODCACHE`, no network involved.
- `GOCACHE=/tmp/gocache` exactly as the TODO item prescribes
  (shared-mount-safe cache; the fork's tests write nothing into the fork's
  working tree).
- Fork working tree: one unrelated modified status doc (left untouched);
  no test-relevant dirt.

```sh
cd /home/lars/forks/cordis/go
GOCACHE=/tmp/gocache GOTOOLCHAIN=go1.27.1 \
  go test ./... -race -count=1 -v -coverprofile=/tmp/cordis-race-cover.out
# total: GOCACHE=/tmp/gocache GOTOOLCHAIN=go1.27.1 \
#   go tool cover -func=/tmp/cordis-race-cover.out | tail -1
```

`-race` implies `covermode=atomic`; the percentages below are from the
atomic profile, per-package lines as printed by `go test` plus the
`go tool cover -func` total.

## Results

| Package                              | Coverage | Result                              |
| ------------------------------------ | -------- | ----------------------------------- |
| `github.com/LarsArtmann/cordis/go` (root) | 91.7%    | ok (1.117s), race-clean             |
| `…/go/group`                         | 90.6%    | ok (1.011s), race-clean             |
| `…/go/hmr`                           | 90.0%    | ok (1.012s), race-clean             |
| `…/go/loader`                        | 74.6%    | ok (1.089s), race-clean             |
| `…/go/timer`                         | 88.9%    | ok (1.011s), race-clean             |
| **Total (statements)**               | **86.2%**| 176 PASS / 0 FAIL / 0 SKIP, 0 races |

## What this changes and does not

- **Changes:** ADR-0004 trigger T3's evidence basis — amended in place in
  `docs/adr/0004-lifecycle-streaming-library-stance.md` from "vendor
  README claim, never run" to the verified numbers above, dated. The
  05:24 report itself is a point-in-time snapshot and stays unedited; its
  d2 self-flag for the cordis claim is now closed by this record.
- **Does not change:** any go-taskqueue code, dependency, or the ADR's
  bottom line. The other two d2 items (ro "~200 operators" estimate, the
  degraded ro GitHub fetch) remain open as the separate low-priority
  matrix-correction item (05:24 f5) — deliberately not done here.
- **Freshness note:** these numbers are pinned to `61ec9f9`. The fork is
  actively developed (Go 1.27 alignment landed hours before this run); a
  plugin-era re-evaluation must re-run the suite at whatever HEAD is then.
