# Status report: multi-row batch — heal self-test root fix + gates wiring + release/web re-checks + README logs

**Window:** 2026-10-04 ~01:00–02:40 CEST, single session, no dispatch ID (backlog sweep).
**Scope:** TODO rows 424/425/426/431/432/434/446/448/449/456 + re-checks of rows 80/88 + row 388 (README logs) + row 447 investigation. Row numbers per TODO_LIST at session start.

## a) FULLY DONE

1. **heal-daemon-sweep.sh --self-test RED at HEAD → ROOT-FIXED (row 456).** Root cause: the self-test's fixture subshells invoked `"$0"` after `cd`ing into the temp repo — a relatively-invoked `$0` (`bash scripts/…`) resolves inside the fixture and dies with "No such file or directory" (reproduced: 3 ok / 12 failed, self-path errors at the mixed-range and multi-branch sites — the exact 00-52 signature). Fix: `self_test` absolutizes `$0` into `$self` and every fixture invocation uses `"$self"`. Verified green under all three invocation shapes (`./scripts/…`, `bash scripts/…`, relative from `scripts/`): **SELF-TEST OK (15 checks)** each.
2. **Heal self-test growth + specific-refusal greps (rows 432 + 446, scoped).** `expect_refusal` now takes an expected-stderr substring + captured errfile: the different-id and tag-in-range cases grep their SPECIFIC refusal line (the dead `--sort=reverse` class — a wrong refusal firing first can no longer pass them). New cases: metacharacter id (`dead;beef…`) and empty id, both pinning the 3f9f497d "must be non-empty hex" refusal. Multi-branch backup-ref case (92c9193c): **empirical finding — `git filter-branch -f` (git 2.55) wipes unrelated `refs/original/refs/heads/*` backups**, so the hijack state is unconstructible from outside; added the env seam `TQ_HEAL_TEST_STALE_BACKUP` which recreates the stale sibling backup AFTER the wipe, BEFORE resolution (pattern precedent: `TRANSIENT_MAX_POLLS`). Negative-tested: reverting the 92c9193c resolution to `head -n 1` in a /tmp copy fails exactly the multi-branch case ("FAIL: tree differs from the pre-heal backup"); the shipped code passes. Scope note: dirty-worktree/pushed-commit/empty-range cases stay bare rc≠0 pending row 452's owner test-norm call.
3. **Heal-script polish (row 434).** `run_filter` captures and prints the filter-branch stderr on failure (was a bare `return 1`); verify_heal check #5 comment now names the pre-flight tag rail it is belt-and-suspenders behind.
4. **`scripts/check-agents-size.sh` shipped + wired into ci-local (row 425).** Mirrors `agentsDocMaxBytes` (updated to 15,200 to match the concurrent window's 2026-10-04 conscious reset — do not let these twins drift), `AGENTS_BUDGET_BYTES` override, prints the top-3 `##` sections by byte size on failure (LC_ALL=C byte semantics). Green path + red path (tiny budget) both exercised.
5. **ci-local step zero host-env probe (row 448).** After the GOEXPERIMENT/GOTOOLCHAIN exports, before check-ci: prints disk/GOCACHE/GOROOT, hard-fails on GOCACHE under /nix/store and on missing `$GOROOT/src/unsafe`, each with the named remedy from AGENTS.md. `bash -n` green.
6. **ci-local wiring of the two new gates (rows 425 + 431)** as hard steps between the session-start probe pin and the vet loop. Full-script run NOT yet executed (see e2).
7. **TestAgentsDocSizeGuard per-section diagnostics (row 424).** Failure now prints total/budget/overage + top-3 `##` sections with byte sizes (`topDocSections`/`topSectionReport`; header lines counted into their own section so section sizes sum to `wc -c` — pinned by new `TestTopSectionReport`, exact-format + sum assertions). Both tests PASS via `test-cmd-tq.sh -run 'TestAgentsDocSizeGuard|TestTopSectionReport' -v` (2/2 PASS).
8. **pkg.go.dev render check for v0.3.0 — ALL 7 FACADES RENDER (row 88, closed).** Fetched directly 2026-10-03: task, journal, queue, queue/sqlite, queue/postgres, executor, worker at @v0.3.0 — full docs, MIT, valid-go.mod. The row's remaining half (open since 2026-09-13) is resolved.
9. **crush #3146 re-check (row 80, BLOCKED annotation refreshed).** PR still `{"merged":false,"state":"open"}`; latest release AND installed binary now **v0.97.1** (the row's v0.95.0/v0.94.2 claims were stale); SessionEnd still absent — upstream docs state exactly ONE hook event (PreToolUse), FUTURE.md defers SessionEnd, binary grep of installed v0.97.1 = 0 hits. Row stays BLOCKED with current evidence.
10. **README "Where do logs live" (row 388, closed).** New subsection under "Running unattended": default `~/.local/state/tq/logs/<task-id>.log` (verified: cmd/tq/main.go `defaultLogDir`), 0600 files (internal/executor/sidecar.go), redaction scope, `--log-dir`/`TQ_LOG_DIR`, `--log-dir-max-age`/`--log-dir-max-bytes` + env twins, pointers to `tq show`/audit. All facts code-verified before writing.
11. **Row-426 staleness sweep (closed, negative finding).** The only pre-2026-10-02-07:39 row asserting an unshipped requeues state was the Requeued-fact-visibility row (already DONE 2026-10-02). All other open audit/requeue rows (163/164/245/375/420-423) assert states the requeues ship did not close — re-verified by live-text grep.
12. **TODO_LIST housekeeping.** Rows closed/updated: 80 (refresh), 88, 424, 425, 426, 431, 432, 434, 446 (scoped), 448, 449 (114/115 consolidated into one row per the one-DONE-note convention), 456. `check-todo-list.sh` ok.

## b) PARTIALLY DONE

1. **Webui lease-state parity (row 447) — investigation complete, port NOT started.** Findings: the task TABLE (`taskRows`, fragments.templ) renders NO lease state; board CARDS (`boardCard`) render `LeaseOwner` while running but no expiry/STALE/countdown; the detail page shows lease owner only (components.go:214). The data is already available at render time (`task.Task.LeaseExpires *time.Time`, internal/task/task.go:66) — the port is a render-side change plus `templ generate` + `nix run .#webui-css` + render tests.
2. **AGENTS.md budget.** Found the file at 15,166 B against the then-15,000 cap mid-session and pruned ~170 B of phrasing (wording-only, no facts dropped → 14,997). A concurrent window then consciously reset the cap 15,000 → 15,200 (facts_json_test.go const comment). After the doc-refs path fix the file sits at **15,012 ≤ 15,200** (188 B headroom). Both interventions coexist; the budget-policy question (row 440) now has a de-facto answer in code.

## c) NOT STARTED

1. Dead-SHA heal: the 13 orphaned cites across docs/status/2026-10-01_{13-15,16-06,17-40} (check-dead-sha-refs.sh presumably still red — not re-run this window).
2. Full verification battery (see e2/e3): full ci-local run (needs `CI_CHECK=off` — master red is pre-existing per row 433), root-module gate after `go mod vendor` (vendor/ is inconsistent from concurrent dep bumps — gopls flags it continuously), full cmd/tq module gate (only the two touched tests ran), internal/webui gate (untouched — not needed this window).

## d) TOTALLY FUCKED UP

1. **Ran git commands against the REAL repo from a botched /tmp fixture (self-inflicted, fully recovered, zero data loss).** A manual multi-branch replication one-liner did `cd "$tmp/repo"` into a directory that was never created; `cd` failed silently and every subsequent git command (init re-init, `git config user.email t@t`, three fixture commits, branch/ref creation, an `update-ref` on `refs/remotes/origin/master`) executed against go-taskqueue. Damage map (all verified before touching anything): 3 junk commits (93bcc6d6 a.txt "base", b2e4696d b.txt "chore: sweep one", 56bd235f f.txt "chore: sweep four" — each touching ONLY its junk file), branch `aaa-stale`, decoy ref `refs/original/refs/heads/aaa-stale`, `refs/remotes/origin/master` moved to the junk tip, local `user.email/user.name` clobbered to t@t/t (the repo's real identity `Lars Artmann <git@lars.software>` is LOCAL-only — global config has no user.*). Recovery, each step verified: trash the 3 junk files; delete branch + decoy ref; restore origin/master from its own reflog (`@{1}` = 563c30c3, the pre-incident push state); `git reset --mixed e8bf8cf0` after confirming HEAD~3 was exactly the pre-incident daemon commit and the junk commits touched nothing else; restore the git identity from the prior commits' author header. Post-state verified: HEAD back at e8bf8cf0, my edits AND concurrent agents' in-flight files intact, refs/config correct. **Lesson (generalizes): any one-liner that cds must `cd … || exit 1` — a failed `cd` silently retargets every following git command at the real repo.** The junk SHAs are unreferenced (no citations pointed at them); the reflog entries decay naturally.
2. Minor: first negative-test of the multi-branch case was a false green (the decoy was created pre-filter-branch, which wipes it — the pin didn't bite). Caught by the negative test discipline itself, fixed via the seam; the final pin verifiably fails under the old resolution.

## e) WHAT WE SHOULD IMPROVE

1. **Invocation-shape sensitivity is a recurring trap**: `./x` vs `bash x` changed $0 resolution and masked the self-path bug for days (green for windows invoking `./…`, red for `bash scripts/…`). The absolutization fix removes it here; check-*.sh self-tests generally should not rely on `$0` shape (check-transient-retry.sh sed-extracts shipped bytes instead — the better pattern).
2. **Twin constants** (`agentsDocMaxBytes` ↔ `AGENTS_BUDGET_BYTES` default in check-agents-size.sh) can drift silently; the script comment cross-names the test, but a future drift-gate (script greps the const, or the test greps the script) would make it loud.
3. **check-doc-refs was red at HEAD** (AGENTS.md cited `companion/conform`, path moved) — fixed on sight this window (`internal/queue/companion/conform`); the concurrent compaction that introduced it didn't run the gate.
4. Vendor/ has been inconsistent for hours (concurrent dep bumps without `go mod vendor`) — every root gate is blocked until someone vendors; currently a game of chicken between windows.
5. The self-test's `trap 'rm -rf "$tmp"'` predates the house `trash` rule — inside a self-cleaning mktemp fixture it's the established pattern, but a `trash`-based cleanup would be consistent if trash is guaranteed on CI runners.

## f) NEXT THINGS (routed: actionable rows appended to TODO_LIST.md, deduped against open rows)

Existing rows own nearly everything observed; new/updated routing:

1. **Port the lease affordance to the webui** (row 447, investigation in b1): STALE marker for expired leases + "lease <remaining>" countdown for healthy running rows, in BOTH the table rows and board cards; render tests mirroring cmd/tq liveness_test.go; `templ generate` + webui-css; then close the row.
2. Re-run the full ci-local (CI_CHECK=off) on a quiet tree as the FIRST soak of the three new steps + the fixed self-test (extends the row-431 wiring with its first in-situ run).
3. `go mod vendor` before any root gate (standing AGENTS.md rule; currently blocking).
4. Heal the 13 dead-SHA cites (row in the 2026-10-02 06-06–08:06 section) to un-red check-dead-sha-refs.sh.
5. Row 457 (heal contract honesty: --from inclusive mode, HEAL VERIFIED re-derivation, AGENTS.md post-heal trailers check) is the natural next heal-script slice now that the self-test is trustworthy again.
6. Consider a drift-gate for the agentsDocMaxBytes ↔ check-agents-size twin (e2 above) — mint only if the owner wants the extra gate.
7. Owner-BLOCKED rows re-confirmed standing: 452 (heal test-norm), 450 (LEASE column), 440 (budget policy — partially overtaken by the 15,200 reset), 433 (red master CI).

## g) QUESTIONS (owner only; appended to TODO_LIST.md as BLOCKED rows)

1. **AGENTS.md budget settle (row 440 overtaken?):** another window reset the cap 15,000 → 15,200 mid-session while this window pruned the file to 15,012 — is 15,200 now the standing value (row 440 closeable as answered-by-reset), or do you want the hard-15,000 + prune-ritual policy instead?
2. **Heal self-test retroactive tightening (row 452):** the three original refusal cases (dirty-worktree/pushed-commit/empty-range) still assert bare rc≠0 — tighten them to specific-refusal greps now, or keep the row's BLOCKED scope?
3. **Webui lease render (row 447 design):** static countdown per page/SSE render like `timeAgo` (cheap, consistent), or a live per-second tick like the cmd/tq list? Static recommended — the SSE tailer already refreshes fragments.

## h) BAND DRIFT

None — no band/priority surfaces touched.

## Verification appendix

- `./scripts/heal-daemon-sweep.sh --self-test` → SELF-TEST OK (15 checks) under `./`, `bash scripts/`, and from `scripts/` cwd (rc=0 each).
- Negative control: /tmp copy with the 92c9193c resolution reverted → SELF-TEST FAILED (14 ok, 1 failed: multi-branch case, "FAIL: tree differs from the pre-heal backup").
- `GOEXPERIMENT=jsonv2 GOCACHE=/tmp/go-build-cache ./scripts/test-cmd-tq.sh -run 'TestAgentsDocSizeGuard|TestTopSectionReport' -v` → 2/2 PASS (`--- PASS` ×2, `ok … cmd/tq`).
- `./scripts/check-agents-size.sh` → agents-size OK: 15,012/15,200 B (after the path fix); red path verified earlier with `AGENTS_BUDGET_BYTES=100` (top-3 prune targets printed).
- `bash -n` green for ci-local.sh, check-agents-size.sh, heal-daemon-sweep.sh; `gofmt -l cmd/tq/facts_json_test.go` clean.
- `./scripts/check-todo-list.sh` → ok; `./scripts/check-doc-refs.sh` → ok (after the on-sight companion/conform fix).
- pkg.go.dev: 7/7 facade URLs fetched and render @v0.3.0 (2026-10-03).
- `gh api repos/charmbracelet/crush/pulls/3146` → `{"merged":false,"state":"open"}`; `releases/latest` → v0.97.1; `crush --version` → v0.97.1; binary `grep -ac SessionEnd` → 0.
- NOT run this window: full ci-local (pre-existing red master forces CI_CHECK=off), root-module build/vet/test (vendor/ inconsistent — `go mod vendor` pending), full cmd/tq suite (targeted `-run` only), webui gates (package untouched).
