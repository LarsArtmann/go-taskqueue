# Vendor/ Removal + gomod-sync Gate Rescope Window

**Scope line.** VERIFIED this window, with artifacts: vendor/ untracked+gone
(`git ls-files vendor` = 0 pre-deletion; `trash vendor` post; `test ! -d
vendor`), renamed gate self-test + REAL run green (`check-gomod-sync.sh
--self-test` rc=0; real gate rc=0 after the lint-leak heal, `/tmp/gate3.out`),
both hook self-tests green (guard 66 cases, edit-advice 15), advisory live-fire
(`CRUSH_TOOL_INPUT_COMMAND='go mod vendor'` → vendor-replant advisory fired),
root build+vet green, ALL 20 nested modules build/vet/test green
(`GOWORK=off` loop, rc=0), `nix build` rc=0 (v0.4.2 + go-modules FOD),
`check-cleanroom-install.sh` rc=0 (consumer graph with the pruned sums),
agents-size 27545/27600 (rc=0), TODO gate rc=0. NOT verified: a full
`ci-local.sh` PASS at HEAD (aborts on two FOREIGN reds — remote master CI red
at 7a979cd91 and docs-gates freshness on the concurrent channels agent's
unharvested 00-38 report; local legs were run piecemeal instead), `root-gate.sh`
PASS (rc=1 solely on the foreign webui test), the webui test fix itself
(another agent's live window), and any push (branch ahead 17, unpushed).

## a) FULLY DONE

1. **vendor/ deleted** (167 MB, gitignored local-only tree — `git ls-files`
   was 0): root builds now resolve via the module graph + proxy. Seatbelts
   against re-plant: `.gitignore vendor/` line kept, tq-bash-guard
   `vendor-replant` advisory fires on `go mod vendor` (live-verified), the
   only gate that ran `go mod vendor` (the drift gate's vendor leg) is
   retired.
2. **Gate renamed + rescoped**: `scripts/check-gomod-vendor-sync.sh` →
   `scripts/check-gomod-sync.sh` (git mv, 77% similarity). Vendor leg removed
   (`root_vendor_in_sync`, `vendor_tree_hash`, self-test vendor Cases 4–6
   dropped, fixture trimmed); per-module tidy detection, cmd/tq cqrs parity,
   and lint-leak go.sum scan kept. **TODO row 1076 implemented**: the parity
   check must report its ok-line or the run fails ("unwired or removed
   check"). Env hooks renamed: `GOMOD_SYNC_SELF_TEST`, `GOMOD_SYNC_OFF`,
   `TQ_GOMOD_SYNC_EXTRA_SUMS`; self-test renumbered 1–8, green.
3. **Wiring updated**: `ci-local.sh` (step titles + comments + gate path +
   gofmt seatbelt comment), `.github/workflows/ci.yml` (Dep-bump drift step +
   gofmt seatbelt comment).
4. **Hooks**: `tq-bash-guard.sh` stale-vendor advisory (fired on go
   build/test with dirty internals) REPLACED by vendor-replant (fires on
   `go mod vendor` itself); self-test case flipped (dirty go test now pinned
   silent) — 66/66 green. `tq-edit-advice.sh` go.mod note reworded (NEVER
   `go mod vendor`); 15/15 green.
5. **Dead fallbacks pruned**: `build-webui-css.sh` vendored-source fallback
   removed; `proxy-wait.sh` + `check-pkg-proxy.sh` `-mod=readonly` comments
   reworded to the lint-leak-writer reason (readonly kept).
6. **Docs**: AGENTS.md Known Issues bullet rewritten (deleted 2026-11-23,
   NEVER `go mod vendor`; size guard green); CHANGELOG `[Unreleased] ###
   Internal` row added (O14-compliant); TODO rows 512 (vendor-trash ruling)
   resolved OBSOLETE, 1076 ticked DONE with receipt, 845/946/1476/1850
   renamed refs.
7. **Daemon-race repair** (content lossless, tree byte-equal — verified
   `git diff` empty): my soft-reset fold orphaned 3 titled foreign commits;
   reconstructed the honest chain via `git commit-tree` (their trees+messages
   verbatim): `153d357d` (my fold) → `bfa55a4c` ← 5da0d479, `685b8232` ←
   e3d5e6a0, `24c91021` ← c5970771.
8. **Lint-leak heal** (`e9cd1465`): a 2026-11-21 08:42 lint invocation had
   written golangci-lint's graph into root `go.sum` (13 entries) and
   `cmd/tq/go.work.sum` — both sat in committed history. Root tidy pruned 202
   graph-foreign lines; go.work.sum deleted (workspace mode regenerates
   nothing for it, per the 2026-11-12 experiment note). Consumer safety
   proven by cleanroom rc=0. Gate now green end-to-end.

## b) PARTIALLY DONE

1. **ci-local.sh full PASS at HEAD — not achieved**: run 1 aborted at the
   master-CI check (foreign red 7a979cd91); runs 2–3 (CI_CHECK=off) died
   silently at the docs-gates freshness FAIL (3 unharvested §f items in the
   concurrent agent's 00-38 report). Every leg I ran manually is green; the
   remaining reds are owned by the live channels/webui agent windows.
2. **root-gate.sh rc=1**: solely `TestPayloadSectionWindowGrouping` (webui) —
   proven foreign: the test does not exist at base 398b6f5a (worktree run
   returned "no tests to run").
3. **Lint-leak WRITER hunt**: heal applied; writer unidentified. Evidence:
   both leaked files share mtime 2026-11-21 08:42 — one event, pre-existing.
4. **Fork records**: printed here (durable doc) — `aa245fe6/eae48f00/
   add5007d → 153d357d`; `5da0d479 → bfa55a4c`; `e3d5e6a0 → 685b8232`;
   `c5970771 → 24c91021`; `86f2433c/cba027ea` superseded (never cited in
   living docs — grep-verified). The channels agent's own citations to the
   old shas are its window's duty (this report gives the pairs).
5. **TODO row minted, not implemented**: heal-script `--expect-tip` refusal
   guard (pre-reset HEAD-moved check) — row appended, gate green.

## c) NOT STARTED

- Push (17 commits ahead incl. vendor removal + leak heal) — owner call,
  foreign red on master.
- `--expect-tip` guard implementation in `scripts/heal-daemon-sweep.sh`.
- Mechanical bar on `go mod vendor` reintroduction (e.g. a grep gate or
  advisory→DENY promotion — owner call, see g).
- `docs/review/libs/go-health.md` ("Vendored: … present") and
  go-cqrs-lite.md ("root `go mod vendor`" standing guidance) still say
  vendor exists — deliberately left as dated snapshots, unruled.
- Fleet propagation of the vendor-removal pattern to other LarsArtmann repos.
- Cross-project lesson to crush-config `references/lessons.md` (by commit
  there) — the pre-reset HEAD-moved lesson lives only in this repo's TODO
  row so far.

## d) TOTALLY FUCKED UP (self-caught, all repaired)

1. **Orphaned 3 titled foreign commits mid-surgery**: between my last
   `git log` check and the soft reset, the concurrent agent landed
   5da0d479/e3d5e6a0 (dangling-SHA repair + closeout) — my fold swallowed
   them; my subsequent amend then consumed their c5970771 as HEAD. For
   several minutes the branch history was wrong and their closeout sha
   citations dangled. Repaired via commit-tree reconstruction; lesson row
   minted; zero content loss (soft reset + tree-equality diffs prove it).
   Root cause: no re-check of HEAD in the mutation itself.
2. **AGENTS.md size budget blown on first edit** (41 B headroom → 97 B
   over): the hook says run check-agents-size BEFORE composing; I composed
   first, then pruned. Fixed to 27545/27600.
3. **Reported a pipeline's tail exit as a gate rc** ("gate rc=0" from
   `… | tail -8; echo rc=$?`) — the exact PIPESTATUS/tail-rc trap this repo
   bans; the gate was actually rc=1. Re-ran with rc captured to a file.
4. Minor: one edit-tool round trip lost (read-before-edit), one `sleep`
   probe mis-sized. No data loss anywhere; every repair verified by diff.

## e) WHAT WE SHOULD IMPROVE

1. **heal-daemon-sweep `--expect-tip <sha>`** refusal + self-test — turns
   today's manual recovery into a one-flag rail (row minted).
2. **ci-local silent-death UX**: rc=1 with NO FAIL line when the docs-gate
   fan-out loses a worker's message — cost three runs + bash -x to
   diagnose. Surface per-worker FAIL lines in the receipt before exit.
3. **Freshness vs live windows**: docs-freshness blocked my battery on
   ANOTHER agent's 20-minute-old unharvested report. A grace window for
   reports whose owning session is still active (or per-window scoping)
   would stop cross-window battery blocking.
4. **vendor-replant advisory → DENY?** The stale-tree class was 94% of the
   DLQ; an advisory still lets the re-plant through. Owner call (g).
5. **Row 946 ruling**: today's 202-line root prune passed cleanroom —
   evidence toward "tidy deletions are legitimate". Owner can close it with
   this receipt.
6. **Dated review docs vs fix-on-sight**: go-health.md's "Vendored: present"
   is now false-as-of-today but frozen by its dated-verdict convention —
   needs a one-line policy ruling.

## f) NEXT 50

1. Push the branch after master's foreign red clears (owner timing call).
2. Implement `--expect-tip` in heal-daemon-sweep.sh + self-test.
3. Fix `TestPayloadSectionWindowGrouping` (webui window owner).
4. Harvest the 00-38 channels report §f (its owner; clears docs-freshness).
5. Cure remote master CI red at 7a979cd91 (webui window owner).
6. Find the 2026-11-21 08:42 lint-leak WRITER (gate guidance: before it
   re-leaks).
7. Close row 946 with today's cleanroom receipt (owner ruling).
8. Status-index digest ritual — 411 live rows > 400 threshold.
9. Advisory→DENY ruling for `go mod vendor` (g).
10. Policy ruling: dated review docs vs fix-on-sight staleness.
11. Freshness grace window for in-flight reports (design).
12. ci-local: surface silent worker FAILs before exit.
13. Update the two dated docs/review/libs vendor lines if ruled living.
14. Pin go.work.sum absence/regen policy in a gate comment.
15. Add a grep gate barring `go mod vendor` in scripts/ mechanically.
16. Add a cheap tripwire test: "no vendor/ dir at HEAD".
17. Comment the kept `.gitignore vendor/` line as a deliberate seatbelt.
18. Cut v0.4.3 at next release flow (carries the Internal row).
19. Re-run full ci-local to PASS once the foreign reds clear.
20. Propagate fork records if the channels agent cites old shas
    (verify-fork-record.sh pairs in §a7).
21. Row 1514: canary consolidation — update stale gate name in its premise.
22. Rows 1850/845/946/1476: standing gate follow-ups, now vs renamed gate.
23. Advisory phrase-drift → hard-gate flip evaluation (row 110 batch).
24. Advisory tick-claims → hard-gate flip evaluation.
25. STALE-BATCH-HEADER backfill sweep (147 advisory warns, headers at
    README lines 57/184/201/240/248/252/257/261/266/273/282/290/296/332/
    367/384/399/421/441/505/579/613/624/640/652/659/680/687/703/717/726/
    785/800/804/819/830/843/871/888/905/927/932/943/948/962/968/988/1081).
26. Backfill dedup receipts onto receipt-less headers (row 1904).
27. Triage DEDUP-CLAIM-NO-RECEIPT false positives on DONE rows (row 1905).
28. Channels post-upgrade probe (owner-gated row surfaced by today's gates).
29. Dead-sha census re-run over the 11 dangling cites (gate-surfaced row).
30. Go-health v0.7.0→v0.8.1 bump verification (row 1459, owner-blocked).
31. Cure cmd/tq tidy red at the next tag wave (row 1476).
32. Gate preflight dirty-module-file "commit first" message (row 1850).
33. build-webui-css.sh: re-evaluate the -mod=mod warm-up need post-vendor.
34. check-dead-exports/check-frozen-dates/check-dlq-sidecar vendor/** globs:
    prune-or-keep cosmetic sweep.
35. smokes fixture string `[vendor-gofmt]` — rename or annotate as historic.
36. Verify nix devShell doesn't encourage vendoring (GOPRIVATE flow only).
37. Root go.sum prune: per-module twin of the cleanroom proof (row 946's
    module half).
38. AGENTS.md headroom 55 B — schedule a compression pass before the next
    append.
39. Re-run `tq doctor` gofmt/verify legs for this window's receipts.
40. Consider `tq session close` to mint this window's review/status tasks
    (owner choice; no Task-Queue-ID existed for this direct request).
41. Battery runtime: cache module tests keyed by tree hash (perf owner call).
42. Reorder ci-local: local battery BEFORE the remote CI check (today the
    check aborted runs before any local leg — evidence-starved failures).
43. Add `check-todo-list` dedup receipt to this row's section header
    (gates-with-receipts convention).
44. docs-gates freshness: dedup §f items against standing TODO rows
    (§f21 digest item duplicates the standing INDEX BLOAT row).
45. Keep `lint_leak_allowlist` empty until first legit adoption (no action).
46. Guard test pinning "renamed gate summary carries the parity ok-line"
    (row 1076's spirit, at gate level — done; keep pinned in self-test).
47. Sweep other repos in ~/projects for committed/ignored vendor/ trees
    (fleet census item).
48. CHANGELOG release-notes mention of the vendor removal at next tag
    (operator-visible? ruled Internal today — revisit at release).
49. Cross-post the pre-reset-HEAD lesson to crush-config lessons.md (by
    commit there).
50. `tq verdict` was not recorded (no task ID existed for this direct
    request) — confirm that's acceptable for non-task windows.

## g) QUESTIONS (cannot self-answer)

1. **Advisory or DENY?** Should `go mod vendor` be hard-DENIED by
   tq-bash-guard (with an escape hatch like ALLOW_VENDOR_REPLANT=1), or
   stay an advisory? Today it advises; the death class it re-opens was 94%
   of this repo's DLQ.
2. **Push timing**: push the 17 unpushed commits now (vendor removal +
   lint-leak heal; CI will run fresh on my HEAD), or hold until the webui
   window clears master's red and lands its own fixes?
3. **Row 946 ruling**: today's 202-line root go.sum prune passed
   cleanroom (consumer graph compiles, all modules local). Rule tree-wide
   tidy deletions legitimate so `check-gomod-sync.sh`'s heal-and-stage
   flow becomes commit-and-done — or keep it open pending per-module
   cleanroom proofs?
