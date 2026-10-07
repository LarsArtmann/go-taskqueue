# Status report: Pareto execution M1–M4 — trunk green, gates shipped (2026-10-07 04:25)

**Scope of this run:** resumed the Pareto plan
(`docs/planning/2026-10-07_03-34_pareto-trunk-green-to-production.md`,
26 medium tasks M1–M26) with orders to execute the whole list. This
window completed M1–M4 (~4 of 33.4 planned hours of medium-task work)
before this status request. All findings below are from THIS session
only.

## a) FULLY DONE

1. **M1 — journal/cqrs StreamMarker heal, tree GREEN** (`81a331b2`,
   `e28548a4`): upstream go-cqrs-lite `id/v4` now renders branded
   `StreamID.String()` with a `StreamMarker:` prefix while the
   underlying value stays bare; `ParseStreamID` strips it. Fixed the
   two adapter tests (`journal_test.go:92/:300`) AND the same fallout
   in `cmd/tq/facts_cqrs_test.go:75` to assert `Get()` (identity value)
   instead of `String()` (display form). Verified: cqrs module gate ×3
   green, root build+vet green, `TestSeqIDCodecParity` unaffected, full
   per-module gate across ALL 21 modules + root GREEN, `test-cmd-tq.sh`
   GREEN. CHANGELOG Fixed entry.
2. **M2 — SystemNix deploy-flip bundle** (`4aceec58`):
   `docs/planning/2026-10-07_04-11_deploy-flip-bundle.md` — verified
   flags matrix (deployed 28a8ae4 vs master HEAD: redaction default-ON,
   read-model default-ON, IO pass, budget gate, allow-writes,
   GOEXPERIMENT unit env), exact flip commands, 10-step post-deploy
   smoke checklist, Gatus journal-head liveness probe spec (up + advancing
   checks), and the explicit "what this flip does NOT cover" list.
3. **M3 — dep-bump drift gate** (`62b5819e`):
   `scripts/check-gomod-vendor-sync.sh` regenerates `vendor/` (root) +
   runs `go mod tidy` in every module, then requires an empty scoped
   `git status` (catches modified AND missing/untracked files — a bare
   `git diff` misses the missing-go.sum class, found by my own
   self-test mid-build). Self-test pins the b886a677 incident shape
   (bump-without-vendor, tidy drift, clean). Real-tree run GREEN. Wired
   into ci-local (self-test + gate) and `.github/workflows/ci.yml`.
   AGENTS vendor bullet extended; **AGENTS.md formatter padding
   PRUNED** (1,395 B — whitespace-normalized diff proved content
   identical before `git restore`; doctrine comment added to
   check-agents-size.sh). CHANGELOG Added entry.
4. **M4 — daemon post-sweep doc gate + footer-fold tooling** (committed
   as daemon chores `233a0bc8` + `ed6b4e47`, see §d):
   - `scripts/check-daemon-sweep-docs.sh`: detector (daemon subject
     regex × footer-less × doc-gated paths) → runs check-doc-refs +
     check-todo-list + check-status-index. Self-tested (4 selection
     cases + gate-roster pin); shellcheck-clean; REAL RUN detected one
     live doc-gated sweep and all three gates passed.
   - `scripts/fold-marker.sh` (row 206): sanctioned empty footer-claim
     commit (fold disclosure + foreign-hunk disclaimer) with rails:
     unpushed-only, footer-less-only, exactly-mine file claim (refusal
     tested).
   - `scripts/commit-task.sh` (row 508): heal-on-sweep — on
     "nothing to commit" with a daemon HEAD, prints the two sanctioned
     heals instead of failing blind.
   - `scripts/heal-daemon-sweep.sh`: new verification 3b — healed
     footers must be `git interpret-trailers --parse`-VISIBLE (the
     row-276 footer-above-attribution invisibility class); self-test 15
     checks green (fixed my own case-sensitivity bug: parse output
     preserves `Task-Queue-ID` case).
   - `docs/planning/2026-10-07_04-25_daemon-doc-gate-wiring-ruling.md`:
     cron vs hook vs ci-local analysis; host timer spec (systemd
     `*:0/5`) for the owner; row 515 narrowed to "install the timer".
   - ci-local wiring (self-test + gate step, early in the run).
   - TODO_LIST: rows 206 + 508 DELETED (done), row 515 narrowed.

## b) PARTIALLY DONE

- **M4 CHANGELOG entry**: MISSING — the M3 entry landed but the M4
  tooling (operator-visible scripts) has no CHANGELOG row because the
  daemon ate the commit before I wrote it. Two-minute follow-up.
- **M4 attribution**: the work is in the tree and gates were green
  pre-sweep, but it landed as two footer-less daemon chores with my
  explanatory commit message lost (no task ID exists to heal onto —
  heal-daemon-sweep requires one).
- **Full ci-local quiet-tree run**: never captured this window either
  (the standing gap from the previous session; individual gates all
  green at various points).

## c) NOT STARTED

M5–M26 (22 medium tasks, ~29 h of planned medium work), namely: M5
re-dispatch/dedup kill, M6 status-index archive sweep (237 live rows vs
100 threshold — saw the bloat warning fire live during this window),
M7 notify-after-commit wake, M8 GitLogScanner attribution fix (59%
invisible footers), M9 worktree-per-agent, M10 budget O2, M11 JIT
scoring O3, M12 ADR-0019 S2, M13 questions surface, M14 v0.4.0 cut,
M15 postgres CLI, M16 readmodel verify-first, M17 CI parity, M18
secrets pins, M19 verify conventions, M20 forensics/audit, M21 webui
polish, M22 smoke/guard polish, M23 hooks/lint governance, M24
docs-health verify, M25 upstream filings, M26 long-tail parking lot.

## d) TOTALLY FUCKED UP

- **I violated the edit→commit→battery ordering rule and the daemon
  ate my M4 commit** — the exact race class I was building tooling
  against. Sequence: edited 8 files → ran check-todo-list +
  check-doc-refs FIRST (gates take seconds but the daemon window is
  <60s and my whole edit sequence spanned minutes) → `git add && git
  commit` found "nothing to commit": the daemon had already swept
  2 files (`233a0bc8`) and 6 files (`ed6b4e47`). Zero data lost, all
  gates had passed, tree clean — but the M4 commit message (the WHY)
  is gone and the work is unattributed. AGENTS.md documents this rule
  verbatim ("stage+commit BEFORE running anything"). Irony noted.
- Minor: when the cmd/tq `go test` failed to build (DonePreflight
  undefined) I briefly misattributed it to a concurrent-agent mid-edit;
  it was the documented ADR-0017 state (cmd/tq builds only via the
  devmod shim; `test-cmd-tq.sh` is the gate). Cost: one extra gate run.
- Minor: two self-test bugs in check-gomod-vendor-sync.sh (missing
  mkdir; untracked-file blindness) — both caught by the self-test
  itself, which is the process working, but both were avoidable with a
  first-pass review.

## e) WHAT WE SHOULD IMPROVE

1. **Fallout-spread greps**: when fixing a format fallout (M1), grep
   repo-wide for the CALL PATTERN (`.StreamID().String()`) immediately —
   my per-dir symbol grep let the cmd/tq twin slip through for ~30 min.
2. **Commit cadence under daemon pressure**: commit after EVERY
   completed workstream, not at "end of task"; the M4 loss was a
   batching error, not a tooling gap.
3. **AGENTS.md budget is at 16,888/16,900 (12 B headroom)** — M5–M26
   cannot add a single line. A prune or a conscious sixth reset is due
   BEFORE the next AGENTS-touching workstream.
4. **Remote master CI is still red** on the b886a677 lineage — every
   heal is local-only (9 unpushed commits); remote stays red until a
   push, which is owner-gated per policy.
5. **M4's host timer is the missing half**: without the evo-x2 timer,
   footer-less doc-gated sweeps are still only caught at pre-push time
   (ci-local), not mid-window.

## f) NEXT UP TO 50 (ordered per the plan's sequencing rules)

1. M5.1 mint-time done-check (terminal-fact / indexed-closeout refusal)
2. M5.2 `--force-redispatch` escape hatch
3. M5.3 dispatch footer-join dedup
4. M5.4 gate-slow re-dispatch guard (existing-report check)
5. M5.5 harvest anti-race guard (row check-off race)
6. M5.6 `tq audit --redispatch` surface
7. M5.7 table-driven refusal tests + battery + docs
8. M6.1 docs-health ANNOTATE pre-October reports batch 1
9. M6.2–M6.3 git-mv archive batches 2–3 (09-16..09-25)
10. M6.4 digest row + check-status-index green under threshold
11. M6.5 spot-verify 10% ARCHIVE-DONE verdicts
12. M6.6 dead-sha-refs re-run post-heal
13. M6.7 index two-write-points → one convention fix
14. M6.8 DONE-row note collapse pass
15. M7.1 ADR-0009 D4 design review (outside-tx notify + Store seam)
16. M7.2 wake-trace design memo reconcile (fact vs channel)
17. M7.3 Store.Notify hook (buffered-1 chan, non-blocking)
18. M7.4 fire-after-commit at engine-backed write sites
19. M7.5 worker select wake vs tick (backoff reset on wake)
20. M7.6 wake latency pin (<50ms claim after enqueue) + fallback test
21. M7.7 idle-IO bench before/after + ADR note
22. M7.8 dispatcher wake-driven drain (poll as fallback)
23. M7.9 multi-process wake semantics check (same-DB pools)
24. M8.1 reproduce %(trailers) final-paragraph gap
25. M8.2 GitLogScanner fix + unit table
26. M8.3 e2e harness-shaped commit fixture pin
27. M8.4 AMBIGUOUS verdict soften/split
28. M8.5 folded_here changed-file render
29. M8.6 historical derivation-blind census
30. M9–M26 per the plan (worktree-per-agent slice 1, budget O2, JIT O3,
    ADR-0019 S2, questions surface, v0.4.0 cut, postgres CLI, readmodel
    verify, CI parity, secrets pins, verify conventions, forensics,
    webui, smokes, hooks governance, docs-health, upstream filings,
    long-tail parking lot — 90 fine tasks behind these)
31. M4 residue: CHANGELOG entry for the daemon-gate tooling
32. AGENTS.md prune-or-reset decision (12 B headroom)
33. One full quiet-tree ci-local.sh run (standing formality)
34. Push decision for the 9 local commits (greens remote master CI)
35. Row "heal the 02-02 done-prompt folds" (2cc73fc9 etc.) — verify
    pushed-stale vs healable before M26 parks it
36. fold-marker.sh AGENTS mention once budget allows
37. Owner: SystemNix input flip per the M2 bundle (pool alive)
38. Owner: host timer install per the M4 ruling artifact

## g) QUESTIONS FOR THE OWNER (cannot be resolved from here)

1. **Push the 9 unpushed commits?** They contain the M1 tree-green
   heals; remote master CI stays red on the dep-bump lineage until a
   push. Policy says never push without an explicit ask — say the word
   (or I keep stacking locally).
2. **The two daemon chores carrying my M4 work** (`233a0bc8`,
   `ed6b4e47`): leave as-is, or do you want the fold-disclosure prose
   recorded somewhere (no Task-Queue-ID exists to heal onto — direct
   session work)?
3. **AGENTS.md budget at 12 B headroom**: conscious sixth reset
   (16,900 → ~17,500) at the next real growth, or a restructure/prune
   pass first (Architecture section is 8.6 KB)?

## Battery receipts (this window)

- cqrs module gate ×3 green; full 21-module + root gate green;
  test-cmd-tq.sh green; check-doc-refs, check-todo-list,
  check-status-index, check-agents-size, check-script-syntax (83
  scripts) green; heal-daemon-sweep --self-test 15 checks green;
  vendor-sync self-test + real run green; daemon-doc-gate self-test +
  real run green; fold-marker happy path + refusal rails tested in
  scratch repos.
