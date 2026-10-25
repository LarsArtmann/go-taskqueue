# NAF-Surface-Clarity Execution Report — 2026-10-23 15:39 CEST

Task: none (owner-directed session) — execute the committed plan
`docs/planning/2026-10-23_14-51_SUPERB-NAF-SURFACE-CLARITY.md` (NAF v4.1
lessons → verified doc-surface gaps). F01 owner go arrived as the session
directive "execute and verify one step at a time"; TODO_LIST harvest
strategy decided: single section batch (pools sweep once at start, so
staging buys nothing).

## a) FULLY DONE (verified at HEAD, 15:39)

All four workstreams executed as doc-only commits; every gate rc=0 with
captured output:

1. **W1 routing header** (e2adff88): AGENTS.md gained the reader-route
   map (executors → payload contracts + conventions; review agents →
   conventions + operational contracts; gate runners → commands + known
   issues; store code → store invariants). Paid by pruning the package
   table's dprint column padding per the size doctrine — net **−1,687 B**
   (21,030 → 19,343 of 21,100); `git diff -w` proved zero content change
   beyond the block. Both size-guard twins green, budget NOT moved
   (F05 unused): `check-agents-size.sh` "19343/21100 B",
   `TestAgentsDocSizeGuard` ok, root build ok.
2. **W2 viewpoint headers, 9 packages** (aa415b4d core: webui, httpapi,
   worker, readmodel, composition; 040fe93a secondary: executor, journal,
   queue, harvest): every package now states Consumer / Concern /
   Non-goals. Plan errata recorded: the packages were NOT doc-less (plan
   claimed webui/httpapi/worker had none) — webui/httpapi already had
   viewpoint-shaped prose (labels woven in, zero fact loss),
   worker/executor were one-liners (full docs written), readmodel/
   composition kept their mechanism prose with the stanza added on top.
   Gates: root build+vet; worker/readmodel/composition/executor/journal/
   queue `GOWORK=off` build+vet+test all green (captured
   /tmp/tq-battery-modules.log, **6 ok**).
3. **W3 lifecycle × concern grid** (f4abde78):
   `docs/architecture-understanding/2026-10-23_lifecycle-concern-grid.md`
   — 7 lifecycle rows × 5 concern columns, every cell citing its owning
   package + the pinning symbol (file:line where filter-scope:
   queue.go:54 ErrTaskDone, ratelimit.go:142, verifygate.go:292,
   conform/suite.go:120, papdashboard.go:769). Two cells marked GAP and
   harvested to TODO_LIST the same day (closeout×security report-body
   audit; poisoned-repo backoff observability) — single section batch.
   Gates: check-todo-list.sh rc=0, check-doc-refs.sh rc=0.
4. **W4 divergence ledger** (d0373f89):
   `docs/platform-divergence-ledger.md` — nine designed tq-vs-platform
   divergences consolidated from the three memos (single_opener LANDED,
   readmodel-open-seams DECIDED, upstream-seam DRAFTED), each with status
   - memo cite + revisit-when, current vendor pins recorded
     (system/v4 v4.15.1, metaengine/v4 v4.19.0, sqliteengine/v4 v4.5.3 —
     patch-bumped past the memos' writing pins, noted in the header).
     Bounded gap-fill found exactly ONE ruled-but-unledgered divergence
     (timers stay on runactor, compose.go:22 §4a ruling); spot-verified
     rows against code (WithEngine at model.go:180; stats/doctor self-opens
     at cmd/tq/main.go:2039 + doctor.go:956 — current lines; the memo-era
     1907/809 cites had drifted).
5. **Cross-links** (283bedaf): DOMAIN_LANGUAGE.md ↔ grid ↔ ledger ↔ all
   nine package docs (one pointer line each). check-doc-refs.sh rc=0.

**Battery receipts** (rc to files, counts via grep over captured output,
never summary lines): root build+vet rc=0
(/tmp/tq-battery-root-build-vet.log); root `test -race -count=1` rc=0,
**19 ok** (/tmp/tq-battery-root-test2.log); six touched modules
`GOWORK=off` build+vet+test rc=0, **6 ok**
(/tmp/tq-battery-modules.log); `scripts/test-cmd-tq.sh` rc=0, **1 ok**
(/tmp/tq-battery-cmdtq2.log — TestAgentsDocSizeGuard rides here);
check-todo-list.sh rc=0; check-doc-refs.sh rc=0.

## b) Delta since plan commit (b1cef4f2 → 283bedaf)

Six commits, all docs/comments: e2adff88 (W1), aa415b4d (W2 core),
040fe93a (W2 secondary), f4abde78 (W3), d0373f89 (W4), 283bedaf
(cross-links). Zero non-comment Go changes (every Go hunk is `//` lines).

## c) PARTIALLY DONE

None. Every plan task F02–F47 landed; F48–F49 done (this report is the
receipt); F50 (push) intentionally NOT executed.

## d) NOT STARTED

- **Push**: origin/master is 6 commits ahead — owner-gated per the plan's
  own guardrail ("push only if user again explicitly requests") and the
  standing no-unrequested-push rule.
- **CHANGELOG**: deliberately no entry — O14 policy: `[Unreleased]`
  carries operator-visible changes only; this window is internal docs
  (the 2026-10-13 O14 rollout audit moves internal-only rows OUT).

## e) TOTALLY FUCKED UP

1. **Daemon swept me twice** (399f715e+e6e5d9eb folding the W2 core
   edits; ce7324b4 folding the cross-links): both times I had run gates
   BEFORE committing, violating the edit→commit→battery ordering I had
   just read. Healed both via the sanctioned maneuver (soft reset of the
   unpushed contiguous exactly-mine span — verified via `git show --stat`
   — then a proper message; content unchanged). Lesson re-learned:
   stage+commit BEFORE running anything, literally.
2. **First root battery went red**: 5 e2e tests failed "race detected"
   (internal/e2e, 112s) while the module gates ran concurrently. Comment-
   only edits cannot create data races; solo re-run of `./internal/e2e
   -race` rc=0 (37s), then the full root suite solo rc=0. Load-flake at a
   clean parent — same class as the KNOWN_FLAKY entries, though these 5
   tests are not themselves listed; no attribution to this window.
3. **/mnt/buildcache ENOSPC** (100% full, 0 avail) killed the first
   cmd/tq gate with stat-cache write errors, not test failures. Remedy:
   `go clean -cache` pruned 66G of rebuildable build cache (volume now
   69%); gate re-run rc=0. The AGENTS.md "gates run
   GOCACHE=/tmp/go-build-cache" hazard note covers the /nix/store
   variant; the full-volume variant had no written remedy — recorded here.

## f) WHAT WE SHOULD IMPROVE

- The plan's evidence pass asserted "webui/httpapi/worker have NO package
  doc" — wrong for all three (webui/httpapi had good ones). Cost: minor
  (the work became weave-not-write). Step-0 verification grepped for
  doc.go FILES but not package comments in main files; a one-line grep
  for `^// Package` would have caught it.
- The memos' file:line cites had drifted (1907→2039, 809→956) — expected
  rot; the ledger now carries current lines and the convention (re-verify
  vendor/cites at consolidation time) is written into its header.

## g) NEXT (dedup-held; 2 new TODO rows this pass)

- Owner: push the 6 commits when ready (F50); decide the 2 harvested rows'
  enqueue timing at the next pool start (rows: closeout report-body secret
  audit; poisoned-repo backoff observability in pool-health).
- Grid maintenance rule is live: new lifecycle stage or concern ⇒ row/
  column with citations or an explicit GAP + same-day TODO harvest.
- No archive moves; no band drift observed (§i equivalent: zero
  task.reprioritized facts this window — no dispatcher running on this
  session).
