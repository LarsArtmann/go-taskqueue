# C16 CLI halves + owner-session park + CV evidence — 2026-10-12 11:47

Session window (owner "break it down, execute and verify" continuation of the SUPERB
pareto plan, resumed from the 10-42 report WAIT). Scope executed: the C16.1/C16.3/C16.4
CLI halves, the C15.3 two-writer park policy, and the C15.2 CV evidence run. All
go-taskqueue commits carry footer `Task-Queue-ID: 20261012-c16-park-forensics`.

## a) FULLY DONE (and verified)

1. **C16.1 CLI (row 408, CLI half)** — `tq park TASK_ID --until DUR [--reason WHY]` /
   `tq park TASK_ID --release` (`cmd/tq/park.go`, registered in the command map).
   Store-level Park with the 7d ceiling (owner Q1 default; mirrors ask's
   maxQuestionTTL, "days are not a unit" error copy), mutual exclusion of
   --until/--release, friendly pending-only refusal pointing at `tq cancel --force`,
   `parkBy()` = `$USER` fallback "operator". e2e pin
   `TestParkVerbSuppressesClaimsAndReleaseReArms` (park → ClaimDue ErrNoTaskDue → 1
   task.parked fact w/ reason+by → attempts/status untouched → release → claim+complete)
   + `TestParkVerbRefusals` (running-task, bad duration, 170h over-cap, exclusive
   flags).
2. **C16.3 CLI (row 494)** — `tq tasks --dedup <key>` rides `Filter.DedupKey`
   (store pushdown, pinned by `TestTasksDedupFilterPushdown`); `tq audit --journal
   --task-id <id>` scopes the fact replay to one task (exact id or unique prefix,
   `journalDriftScoped` via `FactsForTask`; `DriftReport.TaskID` in JSON + task-scoped
   text header; pinned by `TestJournalAuditTaskScopedReplay` incl. ambiguous-prefix
   refusal). `--task-id` without `--journal` refuses with guidance.
3. **C16.4 CLI (row 410, CLI half)** — `claimSuspects` reads ONE
   `store.ClaimCounts(ctx)` map instead of a facts walk per task; the cmd/tq
   `claimCount` twin and its art-dupl:accept note RETIRED; the stats test now pins
   grouped-map == `queue.ClaimCount` parity; `queue.ClaimCount` doc points census
   callers at `ClaimCounts` (kept for the webui detail page).
4. **C15.3 two-writer park policy** — `--owner-session-park` (default ON) +
   `--owner-session-hold` (15m) on agent-pool; each tick, BEFORE harvest,
   `parkBehindOwnerSessions` (`cmd/tq/ownersession.go`) resolves live owner crush
   sessions from the PreToolUse registry (ping within 10m, cwd at any depth under
   `<projectsDir>/<repo>`) and parks those repos' PENDING tasks for one hold horizon —
   claim/conflict/requeue churn becomes a visible, attempt-free park that lapses on
   its own. Parks fire only on claimable tasks (steady state = ≤1 fact per task per
   hold window; pinned by the double-pass assertion). Pinned:
   `TestLiveOwnerRepos` (fresh/subdir yes; stale/foreign/projects-root no) +
   `TestParkBehindOwnerSessions`. SystemNix wiring: consumption note at the
   done-preflight site in `modules/nixos/services/tq-agent-pool.nix` (policy is
   DEFAULT-ON upstream → activates at the next go-taskqueue flake.lock update; the
   explicit settings key lands at that same step — earlier would fail-fast the pool
   on the locked rev). SystemNix eval-only flake check passed (pre-commit).
5. **C15.2 CV evidence run (autonomous half)** — shared cache untouched;
   `GOMODCACHE=/tmp/gomodcache-cv GOCACHE=/tmp/go-build-cache-cv`:
   `go build ./...` GREEN; `go test ./... -count=1` **RED at base** —
   `internal/architecture` `TestArchitectureValidation` (0.65s, real assertion red)
   and `internal/features/pipeline/handlers` (FAIL at the 600s package timeout — a
   hang, not an assertion). `tests/integration` GREEN (16.5s). So the "confirm gate
   green at base" canary is NEGATIVE for reasons OUTSIDE the go-mod-cache corruption:
   the two reds are CV-local and must be fixed (C15.1-adjacent or new rows) before any
   canary dispatch.

**Batteries**: full `scripts/test-cmd-tq.sh` gate green ×4 (after each half + final);
`internal/queue` module build+vet+test green; root `go build ./... && go vet ./...`
green (vendor synced); `check-facade-parity.sh` unchanged-OK; heal verifications all
"HEAL OK".

## b) COMMIT ORDER (this window, post-heal SHAs)

`9177f28a` park verb → `278fbc12` forensics filters → `b61db8d0` C16.4 flip →
`5675f261` owner-session park. Daemon swept FIVE times (a session record);
fork records to cite: `7b42891c→4ee2b10b`, `18aad2fa→9177f28a`, `49a1ea87→0e913ef6`,
`416cd8b5→278fbc12`, `b29083e0→b61db8d0`, `1d6d963e→5675f261`. Heals were safe every
time: each range base was my own footered commit (the [footer-less span]+[footer HEAD]
shape healed cleanly because `--if-exists doNothing` preserves my footer commits).

## c) NOT DONE / OPEN

- **row-48** (api wire-pin fast-fail) and **row-29** (heal span-terminator bug fix +
  shape reproduction) — not started; both still queued.
- **Docs pass** — FEATURES.md rows for BOTH windows, major-prompting EXECUTED numbers,
  TODO ticks for rows 408/409/410/494, and CHANGELOG entries for the CLI halves are
  still pending (planned next window; the store-surface changelog entries from 10-42
  cover the contract, not the verbs/flags).
- **pg conform on postgres** — still blocked on Q3 (no sanctioned TQ_TEST_POSTGRES).
- **CV reds** — architecture validation failure + pipeline/handlers 600s hang are
  CV-local work items, not diagnosed beyond identification (out of this window's
  scope; evidence above).

## d) ISSUES NOTICED (own work, honest)

1. **Session-shell rc lie stepped on AGAIN**: the first CV test run ended `FAIL` with
   `TEST-RC=0` — `$?` after a `| tail` pipe captures tail's rc (the documented
   no-PIPESTATUS hazard). Caught on review, re-ran with grep to identify the failing
   packages. Rule reaffirmed: batteries rc TO A FILE, never through a pipe.
2. **Sloppy first drafts shipped to the test loop**: park_test.go v1 shadowed
   `testing.T` with `t.Attempts` (compile error — caught); the ambiguous-prefix test
   assumed branded IDs diverge at 10 chars (they share their millisecond prefix —
   fixed to exact-id); the agentpool.go banner multiedit targeted the wrong file with
   a mangled new_string (failed safely, redone in main.go). Two-and-three-round fixes
   that a pre-flight `go vet` per file would have collapsed into zero.
3. **SystemNix commit lost attribution**: the daemon swept `tq-agent-pool.nix` into
   footer-less `50588a22` before my `git add`; healing is NOT safe there because the
   sibling's `c68e59df` (architecture-catalog.md, 11:39) sits ON TOP — a range heal
   would stamp MY footer on the sibling's commit. Left footer-less deliberately
   (docs-only note; the attribution lives here and in go-taskqueue's commits). A
   surgical single-commit reword was possible but skipped: shared repo + active
   sibling = not worth the race.
4. **Daemon cadence rising**: five sweeps in ~90 minutes (11:16–11:44), each racing
   my `git add` within seconds. The heal path held, but commit-after-build-before-test
   discipline is now mandatory even for two-file changes.

## e) WHAT I FORGOT / COULD DO BETTER

- No pre-commit `gofmt -l` pass on the new files before the first test run (gofmt was
  clean by luck; the guard should be habitual before `git add`, not after).
- The park verb's `--until` accepts any positive duration < 7d, including units like
  `170h`; a friendlier `--until` alias for day-granularity (`168h` ceiling hint) was
  considered and dropped — error copy carries the hint instead.
- Did not re-verify C15.1's shipped state before running the C15.2 evidence (the
  reds may overlap C15.1's dead-portal/data-dir scope — flagged in c) instead of
  diagnosed).

## f) NOT STARTED (next session's queue)

row-48 api wire-pin fast-fail → row-29 heal span-terminator fix → docs pass
(FEATURES/TODO ticks/changelog) → C15.1-vs-CV-reds diagnosis → pg conform (Q3).

## g) QUESTIONS FOR THE OWNER

1. **Park defaults ratified?** This window SHIPPED the recommended Q1/Q2 defaults:
   7d `--until` ceiling (re-park to extend) and shared `not_before` visibility (fact
   join for `tq tasks --parked` NOT built). Say the word if you want the ceiling
   lifted or the operator-park fact join added — both are small CLI-side follow-ups.
2. **Permission to trash the corrupt `/mnt/buildcache/go-mod/cache/vcs/3af5bce8…`
   entry** (standing from 07-55, still unanswered): the local-cache evidence run
   PROVES the corruption is NOT what makes CV red — the two failures are CV-local —
   but the shared cache still blocks every normal `go` invocation from CV +
   templ-components consumers. Trash it?
3. **CV reds: mine to fix, or new CV-side rows?** TestArchitectureValidation
   (assertion red) + pipeline/handlers (600s hang) — want me to diagnose/fix them as
   the C15.1/C15.2 continuation next window, or mint them as CV TODO rows and keep
   this queue's window on the go-taskqueue tracks (row-48/row-29/docs)?
