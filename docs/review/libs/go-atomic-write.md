# go-atomic-write — Library Utilization Review

**Verdict:** NOT ADOPTED — KEEP OUT — tq's one atomic-replace file write already rides the deliberate `internal/retryx` seam for the Windows-sharing-violation class; go-atomic-write's real strengths (TOCTOU fingerprints, multi-writer flock) solve concurrent-writer problems tq's single-serialized-writer doctrine says do not exist here.

**Pinned:** none (not a dependency; verified 2026-11-08 by `rg -i 'go-atomic-write' -g 'go.mod' -g 'go.sum' -g '!vendor/**' .` → no output, rc=1; same for `--glob '*.go'` import greps)

**Latest:** v0.6.2 (local checkout /home/lars/projects/go-atomic-write, checked 2026-11-08; HEAD = v0.6.2-39-g5688158)

**Prior coverage:** none (docs/review/libs/ predates this report without an atomic-write entry; AGENTS.md's package table names `internal/retryx` as the shared seam).

**Reviewed:** 2026-11-08 by reading /home/lars/projects/go-atomic-write/{README.md,go.mod,atomicwrite.go,commit.go,stage.go,rename_windows.go,rename_unix.go,fingerprint.go} plus internal/retryx/rename.go, internal/readmodel/{workerfailed.go,schema.go,foldlock.go,foldlock_unix.go,foldlock_windows.go}, internal/executor/{calibration.go,depbump.go,sidecar.go,agent.go}, cmd/tq/verdict.go, internal/watermark/watermark.go, internal/queue/companion/{schema.go,reads.go}; greps in §How to verify.

## 1. Where the need exists today

go-atomic-write offers: `Write`/`WriteWithPerm` (unique tmp + fsync + atomic rename + dir fsync), `WriteVerified` (adds xxhash64 fingerprint + gofrs/flock TOCTOU check → `ErrConcurrentModification`), `WriteIfChanged`, `WriteFunc*` (atomicwrite.go:17-167, commit.go:20-67, fingerprint.go:26-35). Its Windows path retries the rename itself (5 attempts, 1 ms base backoff, `isRetryableRename`; rename_windows.go:13-33).

The tq code it would touch:

- **The deliberate seam**: `internal/retryx` is its own module (S4 root) — `RenameTransient(src, dst)` = 3 attempts, 50→100 ms, 2× multiplier, riding `go-retry`, tolerant of a missing source (rename.go:20-37). Package doc: "Each helper graduates here instead of growing a copy per module" — an explicit AGENTS.md-blessed seam, proven live by the 2026-10-25 Windows CI red on `TestConvergeProjectionHome`.
- **The one atomic-replace-shaped write**: the worker-death marker — `os.WriteFile(tmp)` then `retryx.RenameTransient(tmp, path)` (internal/readmodel/workerfailed.go:68-90, write at :78, rename at :82).
- **Plain transient renames atomicwrite cannot express** (no temp-commit shape, no exported rename helper): projection-home snapshot-aside moves (internal/readmodel/schema.go:170) and depbump's go.mod `.bak`/restore pair (internal/executor/depbump.go:588, :592).
- **Single-writer file channels that need no protection**: `$TQ_RESULT_FILE` verdicts, last-wins by contract (cmd/tq/verdict.go:94, `os.WriteFile` 0600; read post-exit by the executor), the ask marker (cmd/tq/ask.go:202), per-run agent log (internal/executor/agent.go:466), sidecar excerpt (internal/executor/sidecar.go:156).
- **Append-shaped writes (out of scope by design)**: `$TQ_LOG_DIR/calibration.jsonl` (internal/executor/calibration.go:126-127, `O_APPEND|O_CREATE|O_WRONLY`) and the session registry (cmd/tq/agentpool.go:1286) — atomicwrite is replace-shaped, not append-shaped.
- **Not file-backed at all**: `priority_scores` is a SQLite table (internal/queue/companion/schema.go:14, writes reads.go:340), watermark checkpoints are store writes (internal/watermark/watermark.go:50-64, :160-165), readmodel sidecars are SQLite DBs (internal/readmodel/dlq.go:62-84).

## 2. Capability inventory (fit)

| Capability | Status | Evidence |
| --- | --- | --- |
| Crash-durable atomic replace (`Write`: tmp+fsync+rename+dirfsync) | 🟡 partial overlap — exactly one site (workerfailed marker :78-82) already covered by retryx | atomicwrite.go:17-40 vs workerfailed.go:78-86 |
| Windows sharing-violation rename retry | 🟡 overlaps retryx but is UNEXPORTED, internal to atomicwrite's own commit path — cannot subsume the seam | rename_windows.go:13-33; retryx users at schema.go:170, depbump.go:588/:592 |
| Standalone transient rename (retryx's job) | 🔴 MISSING — no exported plain-rename helper; missing-source tolerance is retryx-specific | full API scan atomicwrite.go:17-167 |
| TOCTOU fingerprint verification (`WriteVerified`) | ⚪ N/A — tq has no multi-writer file: verdict file is single-writer-per-run last-wins; the marker has exactly one writer per fold-lock-elected host (foldlock.go) | AGENTS.md store invariants: "Single serialized writer"; workerfailed.go:68 |
| Exclusive flock (`gofrs/flock` dep) | ⚪ N/A — tq hand-owns a cross-platform flock for the `.fold-lock` (syscall.Flock / LockFileEx) — a second flock implementation would duplicate a tested seam | foldlock_unix.go:14-25, foldlock_windows.go:30-41, foldlock.go:80 |
| fsync/dir-fsync crash durability | ⚪ N/A — tq's durability doctrine lives in SQLite sync tiers (AGENTS.md: "NORMAL = checkpoint fsync, lost-tail-tolerant"); the file channels (verdict, markers, calibration) are deliberately best-effort/lost-tail-tolerant | calibration.go:126-131 swallows errors by design |
| `WriteIfChanged` (no spurious diffs) | ⚪ N/A — no generator/idempotent-rewrite write among tq's files (bootstrap writes configs once; markers/log lines are new content each time) | bootstrap.go:537-768 write sites |
| `WriteFunc` streaming writes | ⚪ N/A — every write is small marshaled JSON | — |
| Pure Go, `CGO_ENABLED=0` | compatible — deps are cespare/xxhash/v2 + gofrs/flock + x/sys, all pure Go | go-atomic-write/go.mod |

## 3. Findings (buy vs cost)

1. **Buy (small):** at the worker-death marker, `atomicwrite.Write(path, raw)` would replace three lines and add fsync durability plus a unique-tmp name (workerfailed.go:78-86 currently uses a fixed `path+".tmp"`, so two concurrent marker writers could collide — but only one writer can exist: the fold-lock-elected host).
2. **Cost (dominant):** adoption does not subsume `retryx` — the library's Windows retry is private to its commit path, and 2 of 4 retryx call sites (schema.go:170 snapshot moves, depbump.go:588/:592 bak/restore) are plain renames atomicwrite cannot express. The result would be two mechanisms for one failure class, and `internal/retryx` would stay anyway.
3. **Cost:** +2 direct deps (xxhash, gofrs/flock) and vendor growth, for machinery (fingerprints, concurrent-writer detection) whose premise — competing writers — tq's architecture forbids at every cited site.
4. **Adoption site if it ever happened** (the marker only; retryx stays for everything else):

```go
// internal/readmodel/workerfailed.go — writeWorkerFailedMarkerAt
if err := atomicwrite.Write(path, raw); err != nil { // replaces WriteFile(tmp)+RenameTransient
    return fmt.Errorf("readmodel: publish worker-death marker: %w", err)
}
```

## 4. Fit assessment

go-atomic-write is a good library aimed at config files and shared state exposed to competing writers. tq is the opposite consumer: its durable state lives in SQLite under a single-serialized-writer doctrine, and its file channels are deliberately ephemeral, best-effort, or single-writer. The one legitimate overlap — the tmp+rename marker — is already served by `internal/retryx`, which AGENTS.md treats as a deliberate shared seam with its own module, its own tests (rename_test.go pins the transient-conflict healing), and two call sites the library cannot reach. Adopting would duplicate the seam, not retire it.

## 5. Maturity + version currency

Latest tag v0.6.2 (2026-11-04); 12 root tags since v0.1.0 (2026-06-03); last commit 5688158 2026-11-06; checkout is v0.6.2-39-g5688158 (39 commits past the tag — active, same-owner ecosystem as go-retry/go-cqrs-lite, already an indirect dep of go-credentials-store). MIT. Go floor 1.27 — below tq's 1.27.3, no toolchain conflict.

## 6. Conclusion

Keep out. The need atomic-write targets — concurrent writers racing a file — is structurally absent from go-taskqueue, and the transient-rename failure class it does address is owned by `internal/retryx` at four sites. Revisit only if a future surface introduces a genuinely multi-writer file (e.g. a shared cache file outside SQLite); nothing on the roadmap (ROADMAP.md channels plan) does.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
rg -i 'go-atomic-write' -g 'go.mod' -g 'go.sum' -g '!vendor/**' .          # → no output (rc=1)
rg -i 'go-atomic-write' --glob '*.go' --glob '!vendor/**' .                 # → no output (rc=1)
rg -n 'retryx\.' --glob '*.go' --glob '!vendor/**' --glob '!internal/retryx/**' .
# → readmodel/workerfailed.go:82, readmodel/schema.go:170, executor/depbump.go:588+592 (4 sites)
sed -n '20,37p' internal/retryx/rename.go          # RenameTransient: 3 attempts, 50ms→100ms
sed -n '78,86p' internal/readmodel/workerfailed.go # the ONE tmp+rename site
sed -n '13,33p' /home/lars/projects/go-atomic-write/rename_windows.go  # retry is UNEXPORTED path
git -C /home/lars/projects/go-atomic-write describe --tags  # → v0.6.2-39-g5688158
git -C /home/lars/projects/go-atomic-write tag -l | grep -v / | wc -l      # → 12
```
