# Pareto master plan — execution window status (P0 wave + red-master heal)

**Written**: 2026-11-17 21:12 CET (interactive session, direct owner dispatch:
"NOW GET SHIT DONE! The WHOLE TODO LIST!" against
`docs/planning/2026-11-17_19-53_backlog-pareto-master-plan.md`).
**Scope**: T01-T08 + T36 attempted and/or landed; T02/T03/T04 partially;
everything else untouched. Honest ledger inside — the headline is NOT all
green: origin/master is still red pending one push of the local span.

## a) Fully done (verified this window)

1. **T01** push receipt verified (origin/master = 69a835e4 at window start;
   0 ahead).
2. **Red-master heal #1 — templ runtime mismatch** (commit `40f96302`,
   pushed): `93468aac` regenerated `*_templ.go` with a templ CLI newer than
   the pinned runtime v0.3.1100 (76 `templ.ResolveAttributeValue` /
   `JoinURLErrs` calls the vendored runtime does not export) — every CI leg
   red at 69a835e4. Regenerated with the go.mod tool pin; root build/vet +
   webui tests green; the test-vet and test-postgres legs went green.
3. **T06 — `scripts/verify-fork-record.sh`** (row 88's exact spec): patch-id
   → tree ladder, reachability-gated both sides (old-reachable and
   new-unreachable refusals), rc=0/1 with named reasons; 6-case
   `--self-test` green (amend / cherry-pick / unrelated / not-a-commit /
   reachable-old / dangling-new).
4. **T05+T07+T08 — dead-SHA heal wave** (commit `5e1928b6`, pushed): gate at
   HEAD reported 612 findings / **198 unique danglers** (grown from the
   plan's 108). Resolution driver (patch-id map + tree map over reachable
   commits, newest-first): **191/198 resolved, every pair verified by the
   tool**; **604 bare cites arrowed to fork records across 60 files**;
   7 no-counterpart tokens + the 3-token heal-daemon-sweep oscillation walk
   baselined with dated reasons (arrowing a succession chain would falsify
   it). Row-87 audit done (4/5 double-dangling partners re-pointed to
   verified successors; `098324f6→d3df022a` documented dead end). Rows
   88/148/1077/1291/1373 ticked. **`check-dead-sha-refs.sh` rc=0, 0 findings
   — first green since the 2026-10-12 rebase era.**
5. **Red-master heal #2 — three root causes** (commit `9d321e1d`, pushed):
   - `networkoutage_test.go` lacked `//go:build unix` (compiles on Windows
     against the unix-only `agentTaskT`) — tag added, GOOS=windows vet green,
     test-windows leg GREEN.
   - io-surface manifest: duplicate conflicting `tryFoldLock.rationale`
     field (CUE rejects) merged; `runAgent`→`runAgentTurn` renamed entry
     (the I/O moved wholesale); `writeFailureSidecar` declared — io-surface
     check **0 errors / 0 warnings**.
   - **templ-freshness gate red-by-construction**: the gnh check's oracle is
     hardcoded nixpkgs templ 0.3.1020, whose generator emits the pre-removal
     `ResolveAttributeValue`/`JoinURLErrs` API (verified in 1020's
     runtime.go/url.go), while templ-components v1.27.0 hard-requires runtime
     ≥ v0.3.1100 and upstream REMOVED that API by 1100 — no committed output
     satisfies both. Disabled with full reasoning in flake.nix; re-enable +
     gnh-oracle-override rows filed.
6. **Red-master heal #3 — two hidden legs** (folded to footered chores
   `6f9156d0→e9b01f8e`/`eaeb54a4→cbf3021e`, fork records verified): 5 module `go` directives raised to
   the 1.27.4 floor (check-go-mods 53/53 green — this red sat BEHIND the
   earlier breaks for 2+ CI cycles); `app.css` regenerated via
   `nix run .#webui-css` (check-webui-css green); 202-line golangci
   lint-leak purged from root go.sum (`go mod tidy`; cascaded through every
   module go.sum; check-gomod-vendor-sync green).

## b) Partially done

- **T02 (green battery)**: ci-local blocked TWICE by its own check-ci
  red-master guard (rc=1 receipts captured: `/tmp/pareto-battery/`). Root
  build/vet/webui-test green receipts captured individually. **Full green
  ci-local run NOT achieved** — blocked on a green origin run.
- **T36 (CI watch)**: runs 44154930476, 44156845241, 44161532529 diagnosed
  (each fully root-caused, see a); the fixes for the LAST two failing legs
  (go-mods + css) are in the unpushed local span. Next push = the green
  candidate. CI-reds chain rows 1668/1690 still unticked (correctly — no
  green origin run yet).
- **T03/T04 (status-index digest)**: classified — **0 of 233 root reports
  are archive-eligible** (every one holds open items; the ~265 floor needs
  per-item verdicts, not a mechanical move). docs-gates battery measured:
  freshness FAIL (newest report's §f unharvested — the concurrent session's
  20-10 report), routing PASS, cadence PASS. ANNOTATE+ARCHIVE wave itself
  NOT executed.
- **T27/T28** (IO in-repo rows): advanced by the CONCURRENT session during
  this window (5b613886→af94e69a counter-fold kernel-source verify;
  c531d408→b25a6f2b claims.cue judgment pass — their lineage was rebuilt,
  fork records verified) — not my work; noted so the plan's board stays
  honest.

## c) Not started

T09-T16 (heal-daemon-sweep hardening ×2, commit-task amend guidance, AGENTS
twin guard, check-agents-size self-test + delta print, deploy cadence ×2);
T17-T26 (smoke-lib ×4, SSE triage + pins, commit-msg pins, erraudit sibling
session, cqrs archive ×2); T29-T35 (deployed-unit doc, owner digest, JSON
pin, facade-parity, residue batches, pool dispatch wave). The ~600-row
residue tail remains pool food by design.

## d) Totally fucked up / honest failure ledger

1. **Five daemon-sweep races** (templ fix, heal wave ×2, CI heal,
   floors/css/go.sum): every one caught by the empty-footer re-check and
   folded (amend or soft-reset squash; no pushed history touched) — the
   floors/css/go.sum heal (24 files) was folded to footered chores
   `6f9156d0→e9b01f8e`/`eaeb54a4→cbf3021e` (fork records verified by
   scripts/verify-fork-record.sh) before the push.
2. **Pushed a fix while two more reds sat behind it** (twice): the 40f96302
   push healed vet but surfaced windows/io-surface/nix; the 5e1928b6 push
   surfaced go-mods/css. A pre-push local pass over the CHEAP gates
   (io-surface, GOOS=windows vet, check-go-mods, check-webui-css) would have
   caught all four in one cycle. Two wasted CI rounds (~10 min each).
3. First rewriter run had a file-path parse bug (`TODO_LIST.md:101` treated
   as a path) — dry-run caught it before any write.
4. `verify-fork-record.sh` shipped its first version with a bash syntax
   error (unquoted parens) + two fixture-expectation bugs — three iterations
   to green, all BEFORE first real use.
5. My row-87 edit briefly introduced a garbage fragment
   (`689d5bf9c → corrected: 689d5bf9c`-class) — caught and cleaned one
   command later.
6. One mis-verified grep (`echo rc=$?` after a pipe captured `head`'s rc,
   not grep's) sent a wrong turn on the vendor-templ symbol check —
   re-verified with a direct grep before acting on it.
7. ci-local never completed green this window; the "green battery" receipt
   the plan's T02 promised does not exist yet.

## e) Improvements — what I should have done better

1. Diagnose ALL red CI legs (log-failed, not just the first) BEFORE the
   first push; push once, green once.
2. Run the cheap CI-parity gates locally pre-push (io-surface check,
   GOOS=windows module vet, check-go-mods, check-webui-css, nix flake check
   scoped) — they are seconds each and sat behind minutes-long test legs in
   CI, hiding reds for two cycles. Candidate ci.yml re-order: cheap gates
   first.
3. Commit after EVERY file batch, not after verification batches — the
   daemon race tax was 100% self-inflicted schedule slack (the AGENTS
   edit→commit→battery ordering rule exists and I honored it late).
4. The templ-freshness gate landed (93468aac) with "green baseline" ASSERTED
   but no captured nix rc — the row-1933 ritual (new gate lands with its own
   captured green rc) is exactly the missing discipline; my disable commit
   now carries the reasoning inline.

## f) Top 50 next things (priority order)

P0 — green master at origin: 1) let the concurrent poolhealth work commit,
then PUSH the 6-commit local span (floors+css+go.sum fixes ride in it);
2) daemon chores folded + footered (6f9156d0→e9b01f8e,
eaeb54a4→cbf3021e, verified by scripts/verify-fork-record.sh);
3) watch the origin run → tick CI-reds chain rows 1668/1690 (T36);
4) full `./scripts/ci-local.sh` green rc receipt (T02); 5) root battery
rc receipts (build/vet/test -race) at final HEAD.
P0.5 — digest ritual: 6) harvest-or-fresh:skip the newest report's §f
(freshness FAIL); 7) derive routing-driven verdicts for the oldest reports
(routing PASS = living docs already cover them); 8) `annotate.go
status-items` batch (emit-keys → --verify → dry-run first); 9) check-rows
--strict + bare-rows per candidate BEFORE git mv; 10) archive wave 1
(~40 reports) + README collapse + digest row + manifest; 11)
check-status-index ≤400 (WARN silenced; stretch ~265).
P1 — tooling: 12) T09 heal-daemon-sweep oscillation breaker + --from hint +
self-test; 13) T10 --fold-below + AI-titled cover; 14) T11 commit-task.sh
--amend guidance + post-commit file-diff verify; 15) T12 AGENTS twin guard
test; 16) T13/T14 pre-edit size rule + check-agents-size --self-test +
delta print; 17) productize the twin hunt as scripts/dead-sha-twins.sh
(row 489; this window's /tmp driver is the working prototype);
18) go-nix-helpers oracle-override ask (verify at locked rev first);
19) lint-leak WRITER recurrence hunt (202-line leak re-appeared after the
11-12 fixes); 20) T15/T16 deploy cadence (SystemNix flip + system tq
rebuild + version receipt).
P1.5 — CI hygiene: 21) re-order ci.yml cheap-gates-first; 22) add
check-webui-css + check-go-mods to ci-local preflight if absent;
23) dead-sha --shrink --check snapshot refresh after the baseline append — DONE
2026-11-17: `--shrink --check --write` refreshed the snapshot (680 rows, verdicts
match, rc=0); the 8 TWINED rows the stale snapshot showed were tree+subject twins
that no longer match at HEAD (their successors are patch-id-verified — arrowed at
their bare cites in 5 living task reports instead, verify-fork-record 8/8);
24) watch nixpkgs for templ ≥1100 → re-enable templ-freshness.
P2: 25) T17 stub-agent.sh smoke-lib; 26) T18 routed_run() trio; 27) T19
negative-fixture helper; 28) T20 exec-bits check + chmod sweep; 29) T21 SSE
58-row triage; 30) T22 first 5 SSE pins; 31) T23 first 8 commit-msg pins;
32) T24 erraudit sibling session (#12/#13/#14); 33) T25/T26 cqrs archive
ritual; 34) T29 deployed-unit override doc; 35) T30 owner-decision digest
(19 advisories + §g); 36) T31 tq tasks --json [] pin; 37) T32
facade-parity pre-commit wiring; 38) IO rows 22/24 finish (concurrent
agent mid-flight — coordinate); 39) IO row 25 crush-store growth audit;
40) AGENTS size guard re-check after next edits.
P3: 41) T33 residue batch 1; 42) T34 residue batch 2; 43) T35 pool dispatch
wave (harvest eats the tail); 44) monitor365 zram-alert + IO-attribution
rows; 45) llama-vlm probe (foreign); 46) host MemoryMin=256M (owner);
47) erraudit generic_return ruling wave; 48) go-health v0.8.1 bump
verification; 49) pma daemon --no-verify bypass probe; 50) rewrite-rate
adaptive default flip (owner ruling).

## g) Up to 3 questions for the owner

1. **templ-freshness**: let the disable stand until nixpkgs ships templ
   ≥ 0.3.1100, or escalate the go-nix-helpers oracle-override ask upstream
   now (the only path to re-enabling sooner)?
2. **Lint-leak writer**: full hunt window now (it re-leaked 202 lines after
   the 2026-11-12 writer fixes), or accept tidy-on-detect until a third
   recurrence?
3. **The unpushed 6-commit span** carries the concurrent session's io
   follow-up commits + my floors/css/go.sum fixes (two of them in
   footer-less daemon chores): push as soon as their poolhealth work lands
   and retitle-heal after, or hold for a clean fold first?

---
*Verification receipts this window: /tmp/pareto-battery/ (ci-local rc=1 ×2
[check-ci red-master block], root build/vet rc=0, webui-test rc=0,
win-vet rc=0, io-surface rc=0 [0 errors/0 warnings], dead-sha rc=0 [0
findings], verify-fork-record self-test 6/6, check-go-mods 53/53,
check-webui-css rc=0, check-gomod-vendor-sync rc=0 after heal, treefmt+format
nix checks rc=0). CI runs diagnosed: 44154930476, 44156845241, 44161532529.*
