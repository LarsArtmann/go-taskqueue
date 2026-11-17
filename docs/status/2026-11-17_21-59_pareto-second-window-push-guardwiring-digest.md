# Pareto execution — second window (green-master push, guard-wiring heal, digest wave 1)

**Written**: 2026-11-17 21:59 CET (direct owner dispatch: "GET SHIT DONE! The
WHOLE TODO LIST!", resumed after the 21-12 WAIT). **Scope**: T36 push + CI
watch, T02 battery legs, T03/T04 digest wave 1 (8 oldest reports), T09
landed, pre-push heal battery, two more daemon races. Origin is RED on one
NEW leg (guard-wiring) whose fix sits unpushed behind the push-timing guard.

## a) Fully done (verified this window)

1. **Pre-push cheap-gate battery CAUGHT three reds before the push** (the
   21-12 §e2 lesson applied): io-surface check 2 ERRORS (the concurrent
   session's poolhealth.go added `ownCgroupV2RelPath`/`readCgroupMemoryEventsLocal`
   os.ReadFile sites) → `facts.cue` regenerated via `io-surface init --force`
   (0 errors/0 warnings); `cmd/tq/go.work.sum` carried a 210-line golangci
   lint-leak (the writer hunt stays open) → truncated back to its empty
   at-origin state, check-gomod-vendor-sync green; 4 dead-SHA danglers
   (6f9156d0/eaeb54a4/5b613886/c531d408 — the folded chores + rebuilt io
   lineage) → fork-record arrows, all 4 verified by verify-fork-record.sh.
2. **The 9-commit span PUSHED clean** (`9d321e1d..0067c328`, push-timing 9
   checked / 0 refused): floors + app.css + go.sum purge + concurrent
   poolhealth + io docs + the P0 report + the pre-push heal.
3. **T09 landed** (`35393721`): heal-daemon-sweep now suggests the NEAREST
   different-id footer, killing the 2026-11-02 `--from` oscillation walk
   (the old oldest-blocker suggestion re-refused once per blocker because
   `--from` is an exclusive base). Self-test extended to 36 checks: refusal
   names the nearest, running it converges one-step (foreign footers
   byte-identical), a wider `--from` past a blocker still refuses. AGENTS
   doctrine line updated.
4. **§f item 23 resolved + docs-gates ALL GREEN**: `--shrink --check --write`
   refreshed the snapshot (680 rows, verdicts match, rc=0); the 8 TWINED
   tokens the STALE snapshot showed were arrowed at their bare cites across
   5 living task reports (verify-fork-record 8/8; they are patch-id
   successors, not tree+subject twins, so their baseline rows stay as
   escapes — correct per current mechanics). freshness PASS (was the only
   red), routing PASS, cadence PASS.
5. **Guard-wiring red root-caused + fixed** (`91298247`, unpushed): run
   44169456753 at 0067c328 failed ONLY "Guard wiring check" — 3 orphans
   (check-footer-attribution.sh, smoke/session-start.sh, smoke/_lib.sh),
   hidden behind the go-mods/css legs for two prior runs (third
   hidden-behind red this saga). Fix: footer-attribution + session-start
   pin wired as ci-local steps (standalone 4-check / 3-check self-tests
   verified); check-guard-wiring learned the sibling-sourcing rule (a smoke
   lib a sibling sources is wired; unsourced stays orphaned) with a
   hermetic negative battery via TQ_GUARD_SMOKE_DIR. Real run: 97 wired,
   0 orphaned, rc=0.
6. **T02 battery legs b1/b2/b4 green receipts**: root build rc=0, root vet
   rc=0, root-gate.sh rc=0 (all rc captured to /tmp/pareto-battery/).

## b) Partially done

- **T36**: the push LANDED but the run came back red on guard-wiring (see
  a5); the fix is committed but two push attempts were REFUSED by the
  push-timing guard (commits too fresh: "wait 513s", then "wait 197s").
  CI-reds chain rows 1687/1385/1709 correctly still unticked — no green
  origin run yet.
- **T02**: `go test ./... -race -count=1` was still running at report time
  (background job); full ci-local still blocked by the check-ci red-master
  gate until origin is green.
- **T03/T04 wave 1**: verdict specs DERIVED for the 8 oldest reports
  (2026-10-01 ×6 + 2026-10-02 ×2): 245 struck-verdict lines + 44 routed
  verdicts, evidence-cited (agent A: 4 files full-quality; agent B: 3 of 4
  files good, 06-16/06-06 specs REJECTED after cross-check — see d2 — and
  re-derived by hand). Dry-run repair reviewed (320 spec rows). NOT yet
  applied. **Archive eligibility: zero of wave 1 archivable** — every
  report carries genuinely-open routed items (TODO_LIST rows still open),
  and the every-item-resolved bar makes routed items unarchivable. The
  annotation still lands (future sweeps archive trivially once the rows
  close). The 8 remaining 10-02 reports: NOT started (subagents C/D died
  to rate limits twice).

## c) Not started

T10-T16 (fold-below, commit-task amend guidance, AGENTS twin guard, size
self-test + delta, deploy cadence ×2); T17-T32 (P2); T33-T35 (P3). The
~600-row residue tail remains pool food by design.

## d) Totally fucked up / honest failure ledger

1. **Subagent C and D died to provider rate limits twice** (4 parallel
   read-only agents + session = too aggressive); their 8 reports fall back
   to in-session derivation.
2. **Agent B fabricated spec line numbers** for 06-16 (evenly spaced ~6
   apart — clearly not read) and lazy-defaulted its §b/§c open items to
   `p <report-date>`; my dry-run cross-check against `prose-items` caught
   both (repair said "6 action lines" where 25 were specified). B's 06-06
   numbers were likewise wrong; both files re-derived by hand. The
   docs-health skill's "the map is wrong exactly when you are sure it is
   right" lesson demonstrated live. Policy adopted: subagent specs are
   verified against prose-items line-for-line before any apply.
3. **Daemon races #7-#10**: staged files taken 4 more times (heal batch,
   fold surgery ×2, T09). All folded or superseded same-command; the
   fold at 0067c328 required surgery to keep the concurrent session's
   files out of my commit (their TODO_LIST edit + crush doc rode my fold
   once — unfolded via reset --soft + restore --staged + recommit).
4. **Push refused twice by the timing guard** (513s, then 197s): I keep
   waiting rather than `--no-verify` (the guard exists to protect folding
   tasks), but it delays the green-master fix by ~10 min.

## e) Improvements

1. Subagent-derived verdict specs MUST pass a line-for-line
   prose-items cross-check before apply (fabriculation class, d2).
2. New scripts land WITH their wiring surface in the same change (the
   guard-wiring orphan class: check-footer-attribution shipped 2026-11-09
   unwired; smoke/session-start shipped 09-30 unwired; both invisible
   until an unrelated red cleared).
3. The push-timing wait is the price of fold-safety; schedule the final
   commit ≥10 min before the intended push.

## f) Next things (priority order)

1. Retry `git push` (timing window expires ~22:03) → watch run → tick
   CI-reds rows 1687/1385/1709 on green (T36).
2. Finish T02: test -race receipt + full ci-local green rc.
3. Apply `/tmp/pareto-battery/annotate-wave1.tsv` (repair dry-run already
   reviewed; apply + check-rows over the 8 touched files).
4. Derive wave-1 remainder in-session (10-02 06-44 → 08-06, 8 reports).
5. T10-T16 per the ≤12-min breakdown.
6. P2 tier (T17+): smoke-lib, SSE, pins, erraudit, cqrs archive, IO rows
   (concurrent session mid-flight — coordinate), owner digest, JSON pin,
   facade-parity, dead-sha-twins.sh.
7. P3: residue batches + pool dispatch wave.

## g) Questions for the owner

1. **Archive bar vs routed items**: every wave-1 report holds genuinely
   open §f items that live as OPEN TODO_LIST rows — the every-item-resolved
   bar keeps those reports alive until the work itself completes. Is that
   the intended long-term doctrine (reports age out as their work lands),
   or should a routed item whose TODO row exists count as settled FOR
   ARCHIVAL (a check-archive-eligibility change)? Current behavior followed
   (nothing force-archived).
2. **Push-timing guard vs green-master urgency**: twice refused on fresh
   commits. Keep waiting (default; protects folding tasks) or bypass with
   `--no-verify` for the green-master fix specifically?
3. **Subagent verdict quality policy**: keep the mandatory line-for-line
   verification pass on subagent-derived annotation specs (slower, caught
   two fabrication classes), or drop subagent derivation entirely and do
   all reports in-session?

---
*Receipts this window (/tmp/pareto-battery/): pre-push battery (io-surface
0e/0w rc=0, vendor-sync rc=0, dead-sha rc=0, go-mods 53/53, css rc=0,
cmd/tq gate ok 22.9s, root build/vet rc=0); verify-fork-record 12/12 pairs;
shrink snapshot 680 rows rc=0; docs-gates ALL PASS; guard-wiring self-test
5 pins + real 97/0 rc=0; heal-daemon-sweep 36 checks; root-gate rc=0.
CI runs: 44169685755 green (dependabot), 44169456753 RED (guard-wiring
only). Pushes: 9d321e1d..0067c328 landed; two later attempts refused by
timing guard.*
