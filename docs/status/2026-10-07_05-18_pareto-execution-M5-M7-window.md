# Status report: Pareto execution M5–M7 — re-dispatch kill shipped, wake seam landed (2026-10-07 05:18)

**Scope of this run:** continuation of the 26-task Pareto plan
(`docs/planning/2026-10-07_03-34_pareto-trunk-green-to-production.md`).
This window closed both M4 residues, executed M5 COMPLETE (all 7 fine
tasks), landed M6 archive batch 1 with full routing discipline, and
landed the M7 core (Waker seam). All findings below are from THIS
session only.

## a) FULLY DONE

1. **M4 residues closed.** Index row for the M1–M4 report (`eb19b89c`,
   check-status-index green immediately after — commit-before-gates held
   this time); M4 CHANGELOG entry for the daemon-gate tooling
   (`8acedb8e`).
2. **M5 — re-dispatch/dedup loop kill, COMPLETE** (7/7 fine tasks):
   - **Mint-time done gate** (`ca93be46`): `harvest.RedispatchCheck` on
     every mint surface (single/batch/catchup + `tq enqueue`) refuses
     candidates the store's dedup cannot see — cured review-fix (NEW
     branch: rejected SHA exists + unreferenced + anchor gone from HEAD,
     the four-paid-lap 02-56 class), TODO row ticked/removed between
     scan and mint (row 164 anti-race), prompt citing a task ID with an
     indexed closeout (row-110 class). `ErrRedispatchRefusal` wraps the
     sentinel; refusal-only-on-positive-proof, fail-open everywhere.
   - **`--force-redispatch`** (`e68b2979`): CLI escape + Config knob,
     refusal names the escape in the error text.
   - **Gate-slow done guard** (`c62dd3e0`): the done-preflight hook now
     also runs at the verify-gate failure site — landed work completes
     mechanically instead of requeueing into a second paid window
     (row 340); negative control keeps the ladder when no proof.
   - **`tq audit --redispatch`** (`e68b2979`): live-task-on-closed-row +
     spend-after-close (attempts>1 on closed rows) as a queried fact;
     catchup tasks judged, foreign shapes invisible.
   - **`scripts/redispatch-brief.sh <id>`** (`e68b2979`): the mandated
     first batch (tq show + newest prior report + git log) as one
     command; AGENTS convention line reworded to name the mechanism
     (`fc33f528`, byte-budget 16886/16900).
   - **Five TODO rows deleted** (115/164/272/340/442), CHANGELOG Added ×4,
     table-driven tests in harvest (9 cases incl. batch-partial,
     SHA-boundary negatives), worker (guard + negative control),
     cmd/tq (CLI refusal + forced mint).
3. **M6 — archive sweep batch 1** (`c790d32b`): four fully-resolved
   September reports annotated inline (7 new verdicts: 3 TODO routes, 1
   ROADMAP owner question, 1 Won't-implement, 1 subsumed, 1 zero-residue),
   git-mv'd with a per-file manifest in `docs/status/archived/README.md`
   (new), index rows rewritten to backticked archived paths. Foreign
   root-level closeout routed to `tasks/` per O7 (`f9ab3cc6`). Tracker
   row 102 updated with real numbers (`aadd5cf0`).
4. **M7 core — claim-wake seam** (`ad51ed4c` + daemon fold `e9bc652e`,
   see d): `queue.Waker` side interface (ADR-0009 D4 precedent — NOT on
   Store, so no fake/conform breakage); sqlitev4 (= the sqlite facade
   alias) implements it: buffered-1 chan, non-blocking send, fired after
   Enqueue/Requeue/Fail/RescueDead/RecordAnswer commits; 3 tests (fire,
   coalesce + never-block, requeue-fire) green; sqlitev4 + queue modules
   + root build/vet green, vendor regenerated.
5. **Full battery this window:** harvest, worker, queue, sqlitev4 module
   gates; `test-cmd-tq.sh` ×2; root build+vet; root `-race` suite
   (RC=0); check-todo-list, check-doc-refs, check-agents-size,
   check-status-index, check-script-syntax (84 scripts) all green.

## b) PARTIALLY DONE

1. **M7 remainder** (core landed, wiring not): worker `Config.Wake` +
   idleWait select (backoff reset on wake, M7.5), agentpool type-assert
   wiring + stderr note, M7.6 latency pin (<50ms design target),
   postgresv4 Waker parity, M7.7 idle-IO bench + ADR note, M7.8
   dispatcher wake-driven drain, M7.9 multi-process semantics note,
   M7.1/M7.2 design-record notes (Waker-side-seam ruling + wake-trace
   memo reconciliation — the memo's journal-fact layer stays owner-gated
   §g-2, the channel layer is M7's and shipped).
2. **M6 corpus**: 235 live rows vs 100 threshold; batch 1 archived 4.
   The 09-15 five carry 15–23 bare items EACH (the full pre-October
   corpus ≈ 3,000 items) — a standing multi-sweep effort, tracked by
   row 102 with honest numbers. Batches 2–3, digest row, spot-verify,
   dead-sha re-run, two-write-points fix, DONE-row collapse remain.

## c) NOT STARTED

M8–M26 per the plan (M8 GitLogScanner `%(trailers)` final-paragraph gap
first, then worktree-per-agent slice, budget O2, JIT O3, ADR-0019 S2,
v0.4.0 cut, postgres CLI, readmodel verify, CI parity, secrets pins,
forensics, webui, smokes, hooks governance, upstream filings, parking
lot). Standing formalities: quiet-tree ci-local run, push decision.

## d) TOTALLY FUCKED UP

1. **The daemon ate my M7 code commit — the SAME race class, ~3 hours
   after writing the M4 lesson down.** I ran `go mod vendor` + root
   build+vet BETWEEN the sqlitev4 edits and `git add` (~40s of gates);
   the daemon swept `queue.go`+`adapter.go` into footer-less
   `e9bc652e`. My `ad51ed4c` then carried ONLY the test file. Zero data
   lost, tree green — but the code's why lives in a chore commit again,
   and `fold-marker.sh` CANNOT claim it (it requires a queue task id;
   session work has none — the standing g2 gap, second occurrence).
   Rule that would have held: commit the INSTANT a compile-clean state
   exists; run heavy gates AFTER the commit.
2. **multiedit partial-failure discipline**: a 4-edit batch reported
   "3 of 4 applied" and I proceeded WITHOUT identifying which edit
   failed — discovered a step later by grep. Earlier, a no-op edit pair
   in the same tool corrupted `audit.go` (function header glued to its
   first statement; self-caught and fixed within the minute). Lesson:
   multiedit batches must be all-or-verify — read the failure, name the
   missing edit, re-apply deliberately.
3. **Attempts-semantics assumption**: my churn test assumed claim→fail→
   claim→complete leaves attempts=2; attempts counts BURNED FAILURES
   only (=1). Cost: one debug loop with a throwaway test. The semantics
   are now encoded in the churn class (attempts>1 = multi-burn) and its
   test burns twice deliberately.

## e) WHAT WE SHOULD IMPROVE

1. **Commit at compile-clean, gate after** — the M4/M7 daemon losses
   were both "one more gate first" errors. The window between edit and
   commit must never contain a >10s command.
2. **Fold-marker task-id requirement blocks session-work folds** —
   either a synthetic session marker shape or accepting chore-carried
   code needs an owner ruling (g2, now twice-burned).
3. **Attempts semantics deserve one line in DOMAIN_LANGUAGE.md** (a
   fresh reader inherits my wrong assumption for free).
4. **vendor/ is gitignored in this repo** — noticed while committing M7:
   `git add vendor` refused, `git ls-files vendor/` = 0. If vendor is
   deliberately local-only, the M3 `check-gomod-vendor-sync.sh` gate's
   "scoped git status" mechanism cannot see vendor drift — verify the
   gate actually bites (see g3).
5. **M6 scoping**: batch-1 discipline (only fully-resolvable files)
   beat corpus grinding; keep that cadence — per-file integrity over
   row-count velocity.

## f) NEXT UP TO 50 (ordered per the plan's sequencing rules)

1. M7.5 worker `Config.Wake` + idleWait (select wake vs gap; idle reset on wake)
2. M7.6 wake latency pin (design target <50ms; CI-stable bound)
3. M7 agentpool wiring (Waker type-assert + armed stderr line)
4. postgresv4 Waker parity (+ conform Caps note if needed)
5. M7.7 idle-IO bench before/after (ClaimDue counts) + ADR note
6. M7.8 dispatcher wake-driven drain (journal consumer, poll fallback)
7. M7.9 multi-process wake semantics note (same-DB pools)
8. M7.1/M7.2 design-record notes (side-seam ruling; memo reconciliation)
9. M8.1 reproduce the %(trailers) final-paragraph gap
10. M8.2 GitLogScanner fix + unit table
11. M8.3 e2e harness-shaped commit fixture pin
12. M8.4 AMBIGUOUS verdict soften/split
13. M8.5 folded_here changed-file render
14. M8.6 historical derivation-blind census
15. M6 batch 2: the five 2026-09-15 morning reports (15–23 items each)
16. M6 batch 3: 09-16..09-20 files
17. M6 batch 4: 09-21..09-25 files
18. M6.4 digest row + under-threshold check
19. M6.5 spot-verify 10% of ARCHIVE-DONE verdicts
20. M6.6 dead-sha-refs re-run post-heal
21. M6.7 index two-write-points → one convention
22. M6.8 DONE-row note collapse pass
23. One full quiet-tree `ci-local.sh` run (standing formality)
24. Push decision for the 16 unpushed commits (greens remote master CI)
25. AGENTS.md sixth reset vs prune pass (14 B headroom now)
26. Wake-trace memo §g-2 ruling (task.wake vs harvest.wake; fact layer)
27. Row-161: stop artifact minted BY DEFAULT on forced verify-only windows
28. Header→want map refactor (routed this window, internal/webui)
29. Mechanical history-rewrite guard (routed this window)
30. §d-citation convention check (routed this window)
31. Status windows render batch members (routed this window)
32. Verify check-gomod-vendor-sync.sh still bites under gitignored vendor/ (g3)
33. M9+ per plan (worktree slice, budget O2, JIT O3, ADR-0019 S2 …)
34. DOMAIN_LANGUAGE "wake" + "attempts" entries
35. CHANGELOG entry for the M7 wake seam (when wiring lands)

## g) QUESTIONS FOR THE OWNER (cannot be resolved from here)

1. **Push the 16 unpushed commits?** M5 (re-dispatch kill), M6 batch 1,
   M7 core, plus the earlier M1–M4 heals all sit local; remote master CI
   stays red on the dep-bump lineage until a push. Policy: never without
   an explicit ask.
2. **AGENTS.md budget: sixth reset or prune pass?** 16,886/16,900 B
   (14 B headroom). M7.8/M8 windows still need convention lines; the
   Architecture section is the fat candidate (8.6 KB).
3. **Is `vendor/` being fully gitignored DELIBERATE?** `git ls-files
   vendor/` is empty and `.gitignore:64` ignores it — but root builds
   consume vendor/ and this window's M3 shipped check-gomod-vendor-sync
   asserting drift via scoped git status. If the ignore is intended
   (local-only vendor), the gate's missing-file arm looks blind; if NOT
   intended, vendor should be tracked again (or the gate re-anchored to
   a hash of `go mod vendor` output). Which is it?

## Battery receipts (this window)

- harvest module gate green ×3 runs; worker module gate green (incl.
  the load-flaky ExactlyOnce this run); queue + sqlitev4 module gates
  green; `test-cmd-tq.sh` green ×2; root build+vet green ×2; root
  `-race` suite RC=0; check-status-index/todo-list/doc-refs/
  agents-size/script-syntax (84) all green; M3 vendor-sync gate green
  (run in the M5 window).
