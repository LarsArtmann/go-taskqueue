# Done-Prompt Status Report — Five-Task Gates/Tooling Window

**When:** 2026-10-05 04:36 CEST · **Task:** 000001a109c3c819ebc561da2fc100000000 (this done-prompt)
**Window covered:** 2026-10-04 ~01:30 → 2026-10-05 03:47, five completed agent tasks, all receipts re-verified at HEAD before writing (artifact existence, test symbols, CHANGELOG state, TODO closure bytes — not trusted from the close-outs alone). An ADJACENT sixth window (task 000001a109c3c7f31f0fef578c7100000000, close-out 04-10) landed at 04:10 while this prompt was in flight and is treated as noticed-in-passing context, not as window scope.

The five tasks:

| Task | Deliverable | Close-out |
| --- | --- | --- |
| 000001a10423a76d… | Secrets-pass polish bundle (identity pin + growth policy + windows gating) | docs/status/2026-10-04_01-44_task-000001a10423a76db4a0aa0bea7e00000000.md |
| 000001a108f0f46a… | `tq doctor --hygiene` smoke + `--reresolve-verify` claim-time proof | docs/status/2026-10-05_01-09_task-000001a108f0f46a8914c8a6d35d00000000.md |
| 000001a1093141c7… | Daemon-commit attribution audit script + 4-report self-review thread | docs/status/2026-10-05_02-11_task-000001a1093141c714732bc7478d00000000.md |
| 000001a109757cf3… | Attribution baseline gate (frozen 1076-sha baseline + ci-local wiring) | docs/status/2026-10-05_03-13_task-000001a109757cf3d260a0de4e2700000000.md |
| 000001a109ace427… | Verify-retry wrapper (lib + verify.sh + check-verify.sh) | docs/status/2026-10-05_03-47_task-000001a109ace42721f150b2f1d800000000.md |

## a) FULLY DONE (verified at HEAD this pass)

1. **Secrets-pass polish bundle** (`d5c77115` close-out; work commit healed into the footered lineage after the daemon race): `TestSecretHitsAndRedactionCompileIdenticalTable` EXISTS in `internal/executor/redact_test.go` (line ~161: both halves compile against the single `secretPatterns` var, so audit-vs-redaction cannot fork silently); the AGENTS.md "Secrets redaction" bullet carries the token-shape growth policy (ONE pattern + fake-shape sample + lint-baseline note per new provider); windows gating proven compile/vet-level (`GOOS=windows` vet OK, no `//go:build unix` tag, ci.yml per-module canary includes executor). TODO row 129 `[x]`.
2. **Doctor-hygiene smoke + re-resolve proof** (work commit `26723259`): `scripts/smoke/doctor-hygiene.sh` EXISTS (scratch `TQ_DB` — the production-journal trap respected — seeds a stale-pin agent task and asserts the `verify-pins` WARN detail via `--json`); `TestReresolveVerifyClaimsNewGateAfterFlip` EXISTS in `internal/executor/agent_test.go` and proves the flag swaps the gate at claim time (content flip → NEW command; deleted file → flag off fires the stale pin, flag on auto-detects). Executor module suite `-race` green at HEAD per the 01-09 re-verification. TODO row 130 `[x]`.
3. **Daemon-commit attribution audit** (`3eaf44e9` script + `0b72db95` marker heal + `b48fb5ef` fork-record addendum): `scripts/audit-daemon-attribution.sh` EXISTS (12.9 KB; two attribution channels — in-sweep report file, later footered marker commit; doctor exit-1-on-flags; `--json`/`--from`/`--all-chore`; 13-check scratch-repo self-test). First live full-history run: **1275 sweeps → 125 attributed / 70 report-only / 1080 unattributed shipping**; the marker channel dogfood-proven (+4 attributed after its own heal). The four-report thread (01-52, 02-06, 02-11 + addendum) also produced the window's most valuable negative result: the burn that created row 132 was a load-flake the repo ALREADY retries — the connect-the-dots that became task 5.
4. **Attribution baseline gate** (work commit `3aeb96b9`, re-dispatch-verified by `97c13831`): `scripts/daemon-sweep-baseline.txt` EXISTS (45 KB, 1076 non-comment rows = the frozen unattributed-shipping set) + `scripts/check-daemon-attribution.sh` EXISTS (fails only on NEW unattributed shipping; shrink advisory; missing-baseline fails closed; 11/11 self-test) wired as two ci-local steps after the dead-sha gate. Gate verified live green at re-dispatch (1076 baselined, 0 new); it caught three fresh footer-less sweeps of its OWN files within its first hours, all healed. TODO row 131 `[x]`.
5. **Verify-retry wrapper** (`6006fcc3` self-review + healed work commits): `scripts/lib/verify-retry.sh` (ONE known-flaky signature list + ONE `run_with_flake_retry`), `scripts/root-gate.sh` refactored onto it byte-preserved, `scripts/verify.sh` wrapping the EXACT `.tq-verify` battery line (byte-parity pinned) under the retry, `scripts/check-verify.sh` (12 assertions: parity, green/heal/exhaust/no-signature) wired as a ci-local step. Battery receipts ARCHIVED with SHA256SUMS at `docs/status/assets/2026-10-05-task132-verify-battery/` — the only window of the five that archived evidence. TODO row 132 `[x]`; CHANGELOG Added entry present and amended this window to disclose the env-export delta.

All five window tasks closed their TODO rows, carry the exact `Task-Queue-ID` footer as last trailer (post-heal), and are indexed in docs/status/README.md (rows verified present).

## b) PARTIALLY DONE

1. **The verify wrapper is a prepared cure with zero production callers.** `.tq-verify` still points at the raw battery; the one-line flip is an owner ruling (agents must never edit their own verify gate). Until it lands, the motivating failure mode — an agent verify gate dying single-shot on a known load-flake, burning an attempt + a re-dispatch cycle — is NOT cured, only cure-ready. Same for the queue-side minted payload verifies (owner-owned config).
2. **verify.sh's own retry path has never executed anything but green-first-try in situ.** The LIB's behavior cases pass; the FILE's in-situ flake-heal coverage is zero (rowed as row 135).
3. **Full ci-local was never run end-to-end in any of the five windows.** Each stated a scoped-battery ruling; the 04-10 adjacent window counts itself the FIFTH consecutive data point of skip-by-prose. The ruling is sound each time; the pattern is not.
4. **`tq show <id>` skipped at window start in at least three of the windows** (01-09 §d1, 03-13 §d1 "AGAIN", 03-47 §b4). Zero damage each time; the ritual wants to be mechanical, not remembered.
5. **Evidence archiving landed in exactly one of five windows** (task 5). The other four's /tmp receipts died with their sessions — a four-window confessed gap before task 5 finally practiced it.
6. **The attribution baseline is bare-sha** (no per-sha subject/file annotations) and has **no shrink regime** — rows whose sweeps gain footers/markers stay in the 1076 forever unless hand-pruned (02-53 §b4/§f6, carried).
7. **check-dead-sha-refs is rc=1 at HEAD with 92 dead cites across 22 files** (re-counted this pass; unchanged by the window). Foreign fallout of the 03:43 filter-branch heal — every history rewrite mints new casualties, and the chore is unmechanized (rows 418/487). The adjacent 04-10 window fixed one row on sight and deliberately did not mass-baseline.
8. **CHANGELOG has no entry for the secrets-pass identity pin + growth policy** (01-44 §f9 flagged it; the existing "Secrets-pass smoke" and 02-37-era pin entries do not cover the 2026-10-04 identity pin). Rowed this pass.
9. **The status index keeps bloating**: 207 live rows vs the 100 threshold (was 197 at 01-09, 205 at 03-47). The ANNOTATE archive sweep stays undispatched (row 138 owns one slice of it).

## c) NOT STARTED (skipped by the window; still open)

1. The `.tq-verify` → `scripts/verify.sh` flip (owner; BLOCKED row added this pass) and the queue-side payload-verify convergence decision.
2. Dead-sha mass heal (row 418: 92 cites, grew 13→92 in three days) and the `--emit-baseline` generator (row 487).
3. Batch-heal of the ~1076 historical unattributed sweeps — mechanism proven (markers), cost real, owner ruling pending.
4. The footer-visibility family (rows 95/173/188/189/282/310/370/361): daemon footers, GitLogScanner trailer visibility, message-shape rulings — the systemic root of the 1076-sweep mountain, untouched.
5. WILL-FIRE-class `--json` assertion + positive control in doctor-hygiene smoke (00-48 §f1/§f2, carried through three reports, never rowed — added this pass).
6. Multi-sweep marker fixture in the audit self-test (one marker citing N sweeps must resolve all N; 01-52 §e6, never rowed — added this pass).
7. Baseline shrink sweep for daemon-sweep-baseline.txt (02-53 §f6, never rowed — added this pass).
8. heal-daemon-sweep fork-record false old-side sha fix (row 482) and the malformed-row fail-closed fixture (row 483) — both rowed by the window, neither started.
9. Archive-evidence `--help` negation carve-out documentation + the smoke self-verify leg (04-10 §c1/§c3, adjacent window's leftovers).

## d) TOTALLY FUCKED UP

1. **The daemon race was lost in four of five windows — the repo's #1 recurring self-inflicted tax, paid AGAIN after reading the warning in context.** 01-44: swept mid-batch, healed via `heal-daemon-sweep.sh`. 02-11 thread: repeatedly (its own script iterations, then the report itself swept footer-less as `409fa071` MINUTES AFTER claiming the daemon was beaten; healed via fork-record `409fa071→e7953539`). 03-13: the window's own index row swept mid-report (`bb9ce8ef→0b1a919e`), heal #1 refused on a foreign footered commit, heal #2 succeeded scoped. 03-47: swept TWICE. The one window that beat it (04-10, adjacent) did so by commit-per-artifact sequencing — proof the cure is sequencing, not vigilance. Each loss is a history rewrite with new dead-sha casualties.
2. **A self-certified WRONG receipt shipped in the 02-11 thread**: "17/17 root packages ok" came from a grep conflating `^ok` with `^---` FAIL lines, then "re-verified" with the SAME pattern (17 matched itself, not reality — 17 of 22, 5 no-test-files). Caught only by the thread's own self-review; the correction lives as inline strikethrough annotations, but commit ba115833's message still carries the wrong number (annotating commit messages is history-rewrite, so it stands).
3. **The audit task's turn 1 died with the close-out unwritten** — the queue's `lastError` blob was the only surviving record of a full working window until turn 2 reconstructed it. The write-the-report-incrementally lesson is now four reports old and still just advice.
4. **CHANGELOG overclaimed for ~50 minutes**: "the exact `.tq-verify` battery" was true of the command line and false of the runtime env (the wrapper adds `GOTOOLCHAIN=auto` + `GOEXPERIMENT` exports) — in a repo whose history contains an env-delta CI incident. Fixed in `6006fcc3`; the trust cost of an overstated "exact" exceeds the one-edit fix.
5. **Every history rewrite keeps minting dead-sha casualties and nothing closes the loop**: 13 cites (2026-10-02) → 92 (2026-10-05), with the 03:43 heal alone producing the current batch. The gate is doing its job; the remedy is still hand-forensics against a growing pile.
6. **Known-broken tooling consumed without pause**: the heal's FORK-RECORD lines printed old==new SHA (row 482) and were consumed twice more this window before being cross-checked by hand; the archive-evidence negation tripwire forced a `.txt` rename workaround (03-47 §b5) instead of a root-cause fix — the adjacent window then fixed the root cause, retroactively vindicating the row.
7. **The 04-10 adjacent window read a 93-hit gate log via `tail -1`** and shipped a fix for "one dead cite" before seeing the other 92 — the fix itself was true; the attribution pass was blind. Recorded here because it is the same window-night's sharpest process failure and directly shapes the §f battery-hygiene items.

## e) WHAT WE SHOULD IMPROVE

1. **Commit-per-artifact, not commit-per-window.** The only window that beat the daemon committed the moment a file passed `bash -n`. Four windows paid the heal tax after reading the warning. The mechanical sequencing belongs in agent guidance (row 355's surface), not in willpower.
2. **Mechanize the re-dispatch first batch** (`tq show <id>` + newest prior report + `git log -5` as one command/wrapper). Three windows skipped `tq show` while citing the convention that mandates it — conventions that survive only careful reading don't survive.
3. **Receipt discipline: the citation pattern must mean the claim.** Pass counts from `^ok` against a `go list ./...` denominator; FAIL-absence from `^--- FAIL`; never re-verify a number with the pattern that produced it (02-11 §d3). The 04-10 `tail -1` incident is the read-side twin: captured rc files get grep-counted or catted, never tailed (rowed this window by the adjacent report).
4. **Archive evidence in the same command chain as capture** — four windows of dead /tmp receipts before task 5 got it right.
5. **Cheap skips should be re-earned by running, not ruled.** ci-local is ~10 minutes; five windows spent more than that writing prose rulings about why not to run it. Either the skip becomes a pinned convention ("shell-only delta ⇒ this gate list") or it dies.
6. **Close the dead-sha loop mechanically**: `--emit-baseline` (row 487) + fork-record-at-rewrite-time are the two candidate cures for a red that grows with every heal; the owner policy question is queued (§g).
7. **Scoped dead-sha battery legs on TODO_LIST can never go green while foreign cites live there** — every docs-touching window's own-file leg returns rc=1 regardless of its hygiene, which trains ignore-red. Needs the baseline/escape-flag ruling (03-13 §g1).
8. **KNOWN_FLAKY ↔ AGENTS.md Known Issues is a split-brain by construction** (row 136's grep gate closes it for ~15 lines).
9. **AGENTS.md byte budget is nearly exhausted**: 15,358 B against the 15,400 guard — 42 B of headroom. Any "one-liner" improvement (e.g. which-retry-cure-applies-where, 03-47 §e3) needs a prune first; several window §e items died on exactly this.
10. **The archive sweep keeps losing dispatch priority while the index grows ~10 rows/day** (207 vs 100). Row 138 owns the oldest slice; a dedicated sweep window is the only way the INDEX BLOAT WARNING ever clears.

## f) NEXT (grounded; deduped against live TODO rows this pass — existing rows 133/135/136/138/141/482/483/487 already own their items)

1. **Owner: flip `.tq-verify` to `scripts/verify.sh`** — the single act that converts the window's main deliverable from prepared to active cure; decide the env-delta intent at the same time (03-47 §g1). BLOCKED row added.
2. Owner ruling on queue-side payload-verify convergence onto the wrapper (03-47 §g2) — folded into the flip row.
3. Row 135: in-situ verify.sh flake-heal pin (verify.sh's retry path executed only green-first-try).
4. Row 136: KNOWN_FLAKY vs AGENTS.md Known Issues grep gate.
5. Row 482 + 483: heal fork-record true-pre-rewrite sha fix; malformed-row fail-closed fixture.
6. Rows 418 + 487: the dead-sha mass heal and its `--emit-baseline` generator — the highest-leverage green-as-default fix available.
7. Row 133: harvest-hygiene convention (ruling-bearing rows born BLOCKED or pre-split).
8. NEW: doctor-hygiene smoke hardening — WILL-FIRE-class `--json` assertion + a positive control (a current-pin repo asserting `verify-pins` ok) so the smoke cannot pass vacuously (00-48 §f1/§f2). Row added.
9. NEW: multi-sweep marker fixture in audit self-test (one marker citing N sweeps resolves all N; 01-52 §e6). Row added.
10. NEW: baseline shrink sweep for daemon-sweep-baseline.txt — drop rows whose sweeps gained footers/markers since the freeze, keeping the 1076 honest and shrinking (02-53 §f6). Row added.
11. NEW: mechanize the re-dispatch first batch (`tq show` + prior-report + git-log as one recipe/wrapper; three windows confessed the skip). Row added.
12. NEW: CHANGELOG entry for the secrets-pass identity pin + growth policy (01-44 §f9; repo precedent documents new executor pins). Row added.
13. Row 201: history-rewrite heals must re-run dead-sha in their own battery.
14. Row 216: ci.yml parity sweep for the ci-local-only gates (subsumes the doctor-hygiene registration question, 00-48 §f3).
15. Row 138: the 2026-09-15 triplet archive sweep — first slice of the 207-row index bloat.
16. Row 95 (BLOCKED): teach the daemon footers — the root fix behind §d1's entire tax class.
17. Rows 188/189: GitLogScanner trailer visibility + e2e pin (attribution correctness beneath the whole window).
18. `run_with_flake_retry`: trap-based temp-file cleanup on SIGINT/SIGTERM (carried pre-existing gap; fold into any next lib touch).
19. `verify_gate` line: the one-line "do not reformat; sed-pinned" comment (03-47 §e7).
20. After the flip lands: watch one real agent window through verify.sh and record whether FLAKE-RETRY ever fires in production (the value claim is lab-proven, not field-proven).
21. Row 141: `closeoutPending` lifetime audit (parked-while-cancelled leak).
22. Row 40: `tq pool-health` one-shot summarizer.
23. ADR-0019 S1→S4 flips (rows 31–34) — the big carried platform work, untouched this window, unchanged in priority.
24. One full quiet-tree ci-local pass exercising the two attribution steps + check-verify in situ — the only remaining closure for the inherited §b1 chain (five windows and counting).
25. Row 282/310 (BLOCKED/open): footer-vs-attribution message-shape ruling + footer-placement conflict fix.

(Remaining §f slots intentionally unfilled — the carried backlog already exceeds drain capacity; refuses padding.)

## g) QUESTIONS (owner only; each becomes a BLOCKED TODO row)

1. **Verify-wrapper activation cluster** (consolidates 03-47 §g1/§g2/§g3): flip `.tq-verify` to `scripts/verify.sh` — is the wrapper's env-delta (`GOTOOLCHAIN=auto`, `GOEXPERIMENT=jsonv2`) INTENDED post-flip; should the queue-side minted payload verifies also converge on the wrapper; and is signature-only retry the ratified coverage or should the transient 45s×3 class ride along at +2¼ min worst-case per real red? The cure is built and pinned; only you can switch it on.
2. **Dead-sha remedy policy** (consolidates 01-09 §g1, 03-13 §g1, 04-10 §g1): with the red grown 13→92 cites in three days and every heal minting new casualties — should fork-record minting become mechanical at rewrite time (inside heal-daemon-sweep) plus an `--emit-baseline`/scoped-leg escape for TODO-touching batteries, or does hand-curation stay deliberate pressure to prioritize the shrink? Without a ruling the gate stays red-by-default and trains ignore-red.
3. **Same-ID report + TODO closure form** (consolidates 01-09 §g3, 02-11 §g1): one task ID this window minted THREE close-out files (row 130) and another FOUR (audit thread); one row was closed REMOVED-and-replaced, the rest `[x]`+note. Which closure form and which report-per-ID policy are sanctioned? The current mix costs index bloat (207 live rows), re-derivation per re-dispatch, and queue-diff opacity.

## h) BAND DRIFT

**None recorded.** The journal (`TQ_DB` production journal, `tq facts`) holds ZERO `task.reprioritized` facts — the only fact types present are task.claimed/requeued/failed/enqueued/completed/dead-lettered/cancelled/released/question/cancel. This is consistent with ADR-0015: stored priority never mutates; claim order is stored priority + claim-time aging, and this window's de-facto sequencing (secrets polish → doctor smoke → attribution audit → its gate → the verify wrapper it motivated) was driven by the harvest ladder and task-to-task causality, not by priority moves. Nothing to explain after the fact.

## Receipts

- Artifacts verified at HEAD this pass: `scripts/lib/verify-retry.sh`, `scripts/verify.sh`, `scripts/check-verify.sh`, `scripts/audit-daemon-attribution.sh`, `scripts/check-daemon-attribution.sh`, `scripts/daemon-sweep-baseline.txt`, `scripts/smoke/doctor-hygiene.sh`; `TestSecretHitsAndRedactionCompileIdenticalTable` (internal/executor/redact_test.go), `TestReresolveVerifyClaimsNewGateAfterFlip` (internal/executor/agent_test.go); TODO rows 129–132 `[x]` + row 134 `[x]` (adjacent window); CHANGELOG Added entries (verify wrapper incl. env disclosure, attribution audit, archive-evidence negation Fixed).
- Window commits: `d5c77115`, `187b5877`, `b48fb5ef`, `97c13831`, `6006fcc3` (+ healed work lineage `26723259`, `3eaf44e9`, `0b72db95`, `3aeb96b9`, verify-lib commits).
- Live gate state at report time: check-dead-sha-refs rc=1 (92 cites, foreign, row 418); check-status-index ok with TRAILER WARNINGs (f26 cluster, owner ruling pending) + INDEX BLOAT WARNING (207 vs 100); AGENTS.md 15,358 B (guard 15,400); 48 unpushed commits (push stays owner-gated).
- Band-drift query: `tq facts --type task.reprioritized` → 0 facts (types enumerated in §h).
- Daemon-race heal (this prompt, §d1 live again): the auto-commit daemon swept this report's own file footer-less (`afe706ca`) 90 s before my close-out commit landed; healed via `heal-daemon-sweep.sh --from f8120c2c` (HEAL OK, tree byte-equal, backup ref dropped). True fork records from session git log: `afe706ca→25631367` (the swept report commit), `d66ecd10→379241b5` (the close-out commit); the heal's printed FORK-RECORD lines again showed old==new SHA — row 482's known-broken diagnostic, consumed live.
