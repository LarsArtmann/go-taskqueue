# Done-Prompt Window Report — 2026-10-04 01:48 — lockout hardening + auth-plane polish + re-dispatch pair

**Window:** 2026-10-04 00:36 → 01:37 CEST (five dispatched tasks; a sixth concurrent
window, the secrets-pass polish task 000001a10423a76d…, landed at 01:44 and is noted
in passing only).
**Tasks:** 000001a103e3910001325f3f6e6000000000 (de-sleep lockout tests),
000001a103f14c77140a6d272b6000000000 (warn-once bound eviction + clock unification),
000001a104039be310eaf8fcb79700000000 (?token= strike pin — done-on-arrival),
000001a1040cc4256c810bd6d46d00000000 (401/429 body shape — done-on-arrival),
000001a1040cc47766093ba7230000000000 (review-fix re-dispatch — done-on-arrival).
**Method:** all five close-out reports read first; claims re-verified at HEAD
01:48 against `git log`/`git show`, greps over the touched code, module gates
re-run (httpapi 0.025 s ok, webui 10.2 s ok), `heal-daemon-sweep.sh --self-test`
re-run, `check-dead-sha-refs.sh` re-run, TODO_LIST dedup-greps, and the queue
journal read read-only (TQ_DB) for `task.reprioritized` facts. A fresh root
build+vet+`test -race` battery was launched at report time (rc cited in the
verification appendix below if it completed in the window).

## a) FULLY DONE

1. **Lockout tests de-slept** (task …3f3f6e, commit `93c7f4ad`, healed with the
   sweep under `0f79f0ec`, report 00-36). `TestAuthLockout` now uses a
   `lockout.Config.Now` fake clock (was a real 60 ms sleep); webui
   `TestWriteRateLimit*` were verified ALREADY on fake clocks, so the item's
   webui half was done-on-arrival. TODO row 126 `[x]`. Re-verified: httpapi gate
   green at HEAD.
2. **Warn-once on first bound eviction + clock unification** (task …2b6000,
   work commit `68e568c7`, healed, report 00-57). `internal/lockout/lockout.go`
   `boundLocked` fires a one-time `slog.Warn` via `sync.Once` on the
   least-recently-active eviction path; the clock-unification half was confirmed
   already resolved by the 00-25 lockout extraction (both `Retry-After` paths
   flow through the injected `nowFunc`; zero limiter `time.Until` hits). TODO
   row 127 `[x]`. Re-verified at HEAD: the warn sites exist and the module gates
   (httpapi, webui consumer) pass.
3. **Review finding fixed: warn gated on an actual eviction** (task …00000000 →
   ticket …776600, fix commit `6116e23a`, canonical report 01-25, re-dispatch
   closeout 01-34). The reviewer rejected the first warn for firing even when the
   idle sweep alone recovered the map; the fix counts LRA evictions
   (`evicted++`/delete in the oldest-first loop) and gates `evictWarn.Do` on
   `evicted > 0`. Re-verified at HEAD: `lockout.go:228` shows
   `if evicted > 0` guarding the `Do`.
4. **401/429 body shape aligned** (task …46d0000, fix commit `6369ea75`,
   canonical report 01-17, re-dispatch closeout 01-29). Both `http.Error` call
   sites in the httpapi guard replaced with the shared `writeError` seam
   (`{error, fix}` JSON); headers (`WWW-Authenticate`, `Retry-After`, nosniff)
   unchanged. Re-verified at HEAD: `grep -n 'http.Error' internal/httpapi/*.go`
   returns ZERO hits. TODO row 129 `[x]`.
5. **`?token=` strike path + lockout-over-enqueue pins** (task …2b6000→ re-dispatch …2b6…,
   actually task …fcb7970, work commit `d8809930`, reports 01-06 + 01-12). Both
   pins verified present (`TestQueryTokenFailureStrikes`, `TestLockoutCoversEnqueue`);
   the 01-12 dispatch was a pure no-op re-verification. TODO row 128 `[x]`.
6. **Docs-health deltas this pass:** CHANGELOG [Unreleased]/Changed gained the two
   missing user-visible entries (401/429 JSON wire change; eviction warn-once);
   the 00-52 window report's §c2 staleness (strike pin + body shape listed as open)
   inline-corrected with commit evidence; 5 deduped TODO rows appended (§f).

## b) PARTIALLY DONE

1. **`internal/lockout` still has zero test files** (verified at HEAD: package
   contains only `lockout.go`). The eviction-warn contract has now been WRONG
   once (the 01-25 review finding) and re-verified twice, all by reading — TODO
   row 459 (fake-clock table suite with the three-case warn pin) remains the
   highest-leverage open item in this arc.
2. **The 401/429 body shape is unpinned by tests** (01-17 §b1, still true): guard
   tests assert status codes only; a regression to plain text would not fail CI.
   → row appended this pass.
3. **No httpapi package-doc line** for the new body contract (01-29 §f3) —
   appended as a row (it touches a .go comment, outside this pass's doc-only
   scope).
4. **Root `-race` battery:** several window reports cite module gates only; the
   00-36 task ran the root battery at ITS head, but heads moved 6+ commits since.
   This report's own battery run covers the gap (appendix).
5. **check-dead-sha-refs.sh still rc=1 at HEAD** (re-run 01:48): the 13-cite
   cluster is unchanged — pre-existing, tracked as row 414, correctly untouched
   by every window task.

## c) NOT STARTED

1. `heal-daemon-sweep.sh --self-test` fix — re-verified RED at HEAD 01:48
   (`5 ok, 7 failed`, same `line 388` self-path error). Row 456; no window
   picked it up.
2. The status-index archive sweep (live rows at 178 + 5 new) — untouched,
   standing rows.
3. Everything blocked on owner rulings the window correctly did not touch
   (re-dispatch suppression row 451, footer-shape rows 283/311, warn-payload
   shape — appended as a blocked row this pass).

## d) TOTALLY FUCKED UP

1. **Re-dispatch burn is now the dominant cost of the window:** 3 of 5 tasks
   (…fcb7970, …46d0000, …776600) arrived with the work already landed — two were
   same-window re-dispatches of tasks whose canonical reports already existed.
   Each cost a full agent turn to rediscover "done". The suppression-gate row
   (451) stays owner-blocked; three more data points.
2. **Daemon-sweep heals continued to burn one attempt per task** (00-36 heal took
   three calls to find the right fork point; 00-57's first `--from` was exclusive-
   range-blind). The `--from` semantics trap has now hit THREE windows (00-07,
   00-57, and per the 00-52 report) — row 457 exists, still unbuilt. The
   attribution-block-after-footer ordering trap also recurred (00-57 §d2) and is
   covered by owner-blocked rows 283/311.
3. **The heal tool's own safety net remains untrusted:** self-test 7/12 red,
   re-verified this pass, AND the 00-57 heal printed identity `FORK-RECORD`
   pairs (old→old) on a range that WAS rewritten (§b4 of its report) — an
   unexplained verification-story weakness, appended as a diagnosis row this
   pass.
4. **No code damage.** Zero test failures, zero reverts, no gate regressions
   introduced; all five tasks' module gates green at HEAD per this pass's
   re-runs. The worst verifiable standing damage in the repo remains the
   pre-existing dead-SHA red (row 414) and the heal self-test red (row 456) —
   both inherited, neither minted here.

## e) WHAT WE SHOULD IMPROVE

1. **Dispatch-time done-dedup is the single highest-ROI queue fix** (§d1): a
   mint-time check (terminal fact or footer-bearing closeout commit exists →
   refuse/mint as `reverify`) would have made this window ~60% cheaper. Row 451
   holds the decision; the evidence pile grows nightly.
2. **Row 459 first, always:** every task in the lockout arc ended by noting the
   missing `internal/lockout` suite. A suite with the three-case eviction-warn
   pin (~30 lines, fake clock) retires a whole class of verify-by-reading
   closeouts.
3. **Pin what you ship the same window:** the 401/429 JSON change and the warn
   both landed with zero behavior pins; both follow-up rows had to be minted
   later. A closeout contract line "every behavior change lands with its pin or
   cites the row that owns it" would close this.
4. **heal `--from` ergonomics** (row 457): auto-extend to the parent when the
   named ref is footer-less; three windows of identical mistakes is a script
   bug, not an attention bug.
5. **Fork-record honesty** (new row): resolve whether `old→old` identity pairs
   are a print-order bug or a no-op rewrite before the next heal is trusted.

## f) UP TO 50 NEXT THINGS (routed: 5 appended to TODO_LIST this pass, deduped
against 274 pre-existing open rows; the rest carry standing rows)

Appended this pass (docs/status/2026-10-04_01-48_…):
1. Pin the auth-plane 401/429 `{error, fix}` body + `Content-Type: application/json`.
2. One-sentence body-shape contract in the internal/httpapi package doc.
3. Diagnose the heal FORK-RECORD identity (old→old) output; pairs with row 456.
4. Ruling (BLOCKED): eviction-warn payload shape (over=/evicted/surface) before
   the row-459 suite pins it.
5. Ruling (BLOCKED): do `tq serve` write endpoints adopt `{error, fix}` JSON or
   stay an HTML/text UX surface?

Standing top-of-mind (NOT re-appended; existing rows):
6. Row 459 — `internal/lockout` fake-clock table suite incl. the three-case warn pin.
7. Row 456 — fix the heal self-test (7/12 red, re-verified 01:48).
8. Row 457 — heal `--from` semantics + HEAL VERIFIED contract.
9. Row 414 — dead-SHA cite heal (still rc=1 at HEAD).
10. Row 451 + 148/179 — re-dispatch suppression (owner-blocked; three fresh
    data points from this window).
11. Row 89 — HTTP-date Retry-After fixture (pins the nowFunc-unified clock path).
12. Route-inventory structural guard for the all-routes auth/lockout matrix
    (01-06/01-12 §e).
13. Lockout-test helper extraction (limiter + fake-clock boilerplate, ~5 tests).
14. Row 430 — daemon short-delay/lockfile (six-plus sweep incidents and counting).
15. Row 433 — pre-existing red master CI blocking ci-local.
16. Rows 460-462 — sort-chip/FilterBar webui pins (untouched this window).
17. Row 458 — theme-duality render guard.
18-50. The remainder of the 00-52 §f list, the 01-17 §f list, and the carried ⛳
   items in the 00-36 report §f9-24 (screenshot loops, `/health` version feed,
   upstream CSP parity, SECURITY.md matrix, CHANGELOG wording pass before next
   release, closeoutPending 429-parking audit, v0.3.0 go install clean-room,
   postgres parity freshness) — all verified absent from TODO_LIST where noted,
   pacing gates decide.

## g) QUESTIONS I CANNOT ANSWER MYSELF

1. Is the eviction-warn payload shape final (message + cap), or should the
   one-time warn carry `over=<map size>`, the evicted count, and/or a `surface`
   field before row 459's suite freezes it into a pin?
2. Should `tq serve` write endpoints adopt the `{error, fix}` JSON contract, or
   are they deliberately an HTML/UX surface where plain-text errors are fine?
3. Three same-ID re-dispatches in one window (§d1): is dispatch-time suppression
   (row 451) now worth building queue-side despite the earlier blocking, or is
   the per-agent step-zero `git log --grep <id>` the sanctioned mechanism?

## h) BAND DRIFT

**None recorded.** `tq facts --type task.reprioritized` (TQ_DB read-only, full
feed) returns zero facts — journal-wide, not just in this window's timespan
(00:36→01:37). All priority movement came from the standing ADR-0015 ladder
(aging, markers), not explicit reprioritization facts. Nothing to account for.

## Verification appendix

- Re-run at HEAD `d5c77115` (01:48): httpapi module gate ok 0.025s; webui module
  gate ok 10.2s; `grep http.Error internal/httpapi/*.go` → 0 hits;
  `internal/lockout/lockout.go:228` `evicted > 0` gate present;
  `internal/lockout/` has no test files; heal self-test
  `SELF-TEST FAILED (5 ok, 7 failed)`; `check-dead-sha-refs.sh` rc=1 (pre-
  existing 13-cite cluster).
- Root battery `go build ./... && go vet ./... && go test ./... -race` under
  `GOEXPERIMENT=jsonv2 GOCACHE=/tmp/go-build-cache` at HEAD `d5c77115`:
  **rc=0** (log /tmp/battery-01-48.log, completed 01:52).
- Window commits: `93c7f4ad`/`0f79f0ec` (de-sleep), `68e568c7`/`6d29314d` +
  fix `6116e23a` (warn), `d8809930` (strike pins), `6369ea75` (body shape),
  reports `61d7adb2`, `386801af`, `cc8e83c9`, `1793d6e5`, `6c095077`.
- TODO_LIST: 5 rows appended (no reworded duplicates; footer/warn-payload/heal
  candidates deduped against existing rows 283/311/457/459/89); 0 existing rows
  ticked (every `[x]` flip this window was done by its own task).
- Journal: `tq facts --type task.reprioritized` → 0 facts.
