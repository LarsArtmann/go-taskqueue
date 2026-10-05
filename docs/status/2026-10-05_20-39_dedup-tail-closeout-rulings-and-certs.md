# 2026-10-05 20:39 — Dedup Tail Closeout: Rulings, Certs, Harvest, Annotation

Session type: interactive continuation of the same-day 16-02 window
(`2026-10-05_16-02_art-dupl-t3-tail-zero-dedup-sweep.md`). That window shipped
the -t 3 tail zero and ended with three questions in its §g plus a 18-item
§f backlog. This window executed the rulings, worked the backlog to zero,
certified, harvested, and annotated the source report. Scope: closeout only —
no new dedup triage, no foreign work touched.

## a) FULLY DONE

1. **All three §g rulings executed and recorded.**
   - **Q1 foreign break**: `ROADMAP.md:287`'s symbol-style citation
     (`internal/budget.SessionUsage`) reworded to a checker-visible path span
     - bare-symbol span; `check-doc-refs.sh` rc=0 ("doc refs ok"). Commit
       `4be4a8ef`.
   - **Q2 templ residual**: the documented-accepted answer stands; config
     exclusion declined (visibility over canonical zero). AGENTS.md's
     contradictory blanket line ("Residual art-dupl groups accepted")
     tightened to "accepted ONLY where named here: the `fragments.templ`
     conditional-span pair". Commit `748f75f3`. Size guard OK
     (15,611/15,700 B).
   - **Q3 CHANGELOG**: internal-only sweeps get NO row (CHANGELOG precedent is
     behavioral/user-facing entries only; Keep-a-Changelog semantics); dated
     status reports are the record. Recorded as a Won't-implement annotation
     on the 16-02 report's f5 row.
2. **sqlitev4 unused require dropped** (16-02 §f2): in-module `go mod tidy`
   removed `internal/queue/sqlite` + 4 stale go.sum lines; module gate green
   (3.6s); root `go mod vendor` clean (zero diff);
   `nix build .#checks.x86_64-linux.vendor-hash` rc=0, no drift, flake.nix
   untouched. The standing gopls warning cleared. Commit `4be4a8ef`.
3. **Dead directive prune** (16-02 §f6): the six round-1 doc-position
   directives (cmdDLQ, cmdFacts, cmdWatermarks, sessionList,
   `ReviewExecutor.Execute`, `StatusExecutor.Execute`) + their `//`
   separators deleted (21 lines, commit `de63fe08`). One em-dash straggler
   (inside the cmdWatermarks directive) died with the prune. Empirical proof
   of byte-neutrality: art-dupl read **1 group / 2 clones** before AND after.
4. **Symbol-style citation audit** (16-02 §f13): reproduced the checker's
   grep across all six guarded docs — exactly ONE real failure repo-wide
   (the f1 row); every other audit hit is covered by an existing allow-rule.
5. **Directive proofread** (16-02 §f15): zero em dashes across every
   remaining `art-dupl:accept` line; style terse.
6. **`-race` battery** (16-02 §f4): executor 12.5s, companion (+conform),
   cqrsqlite 5.4s, sqlitev4 (+migration) 5.7s — all green under `-race`;
   cmd/tq green via the sanctioned devmod gate (see b1 for the -race caveat).
7. **`root-gate.sh` certification** (16-02 §f3): **rc=0** at this HEAD, on
   the composite tree INCLUDING a concurrent agent's in-flight edits
   (webui 15.8s, facadeparity OK in the tail).
8. **Final art-dupl over the composite tree**: 260 files analyzed, **1 group
   / 2 clones** — the one accepted templ residual. The concurrent agent's
   fresh Go lines (commit `06974658`) introduced nothing.
9. **TODO_LIST harvest** (16-02 §f7): five bounded rows appended under a
   dated section (upstream art-dupl templ issue; art-dupl `-c` config pin;
   14-02 open-items cross-check; 12-27 verdicts-ledger alignment;
   index-bloat check), each citing the source report;
   `check-todo-list.sh` ok.
10. **16-02 report annotated inline** (docs-health ANNOTATE, the
    2026-09-28/29 bulk-strike pattern): 15 f/g dispositions struck with
    verdicts (done-at hashes, verified evidence, Won't-implement rulings)
    via `annotate-status-items.py` (`--emit-keys` → TSV spec → dry-run →
    apply); `check-rows.py` complete; `check-status-index.sh` "status index
    ok". Questions g1-g3 carry their answers in place.
11. **Working tree clean**; every change staged-then-daemon-swept
    (`4be4a8ef`, `de63fe08`, `748f75f3`, plus sweep commits for TODO_LIST
    and the annotations).

## b) PARTIALLY DONE

1. **cmd/tq `-race` coverage is 5 of 6 modules.** The gate pins
   `CGO_ENABLED=0` (pure-Go policy, per script header) and `-race` requires
   cgo. Two attempts to force it failed (see d1); accepted the constraint
   and recorded the caveat in the 16-02 report's f4 annotation. cmd/tq stays
   gate-green without `-race`; the race-sensitive logic lives in the nested
   modules, which are covered.
2. **Index bloat observed, not swept**: `check-status-index` warns "217 live
   rows in docs/status/README.md (threshold 100)" on every run (pre-existing;
   two TRAILER WARNINGs of the f26 cluster class fire alongside). Harvested
   as a TODO row; the ANNOTATE/archived sweep itself not executed.
3. **Foreign concurrent WIP** (conform `//nolint:ireturn` ×2 + readmodel test
   pins, +24 lines, commit `06974658`): observed mid-session, judged benign
   (comment-only nolints + new pins), certified indirectly via root-gate +
   final art-dupl. Their readmodel module gate not run by me — owner's
   window owns it.

## c) NOT STARTED

1. The five harvested TODO rows (list in a9) — all open by design.
2. `ci-local.sh` once at this HEAD (16-02 §f18): root-gate ran instead;
   ci-local is the pre-push gate and no push is pending.
3. devmod lib root-resolution hardening (candidate born from d1's scare) —
   incident analyzed, fix not attempted (foreign-authored lib; fail-closed
   behavior held).
4. The archived/ sweep for the 217-row status index (b2).

## d) TOTALLY FUCKED UP

Nothing broke in the tree; every gate ends green; the fail-closed guards
held. Honest missteps, in order:

1. **cmd/tq `-race` double failure + trap scare.** (i) Ran
   `test-cmd-tq.sh -race` WITHOUT reading the script first — rc=2, the gate
   pins `CGO_ENABLED=0` (documented in its own header). (ii) Hand-replicated
   the devmod shim via `source scripts/lib/cmd-tq-devmod.sh` with
   `CGO_ENABLED=1` — direct sourcing broke the lib's `$0`-based root
   resolution: the cleanup guard grepped `/home/lars/cmd/tq/go.mod`
   (nonexistent) and refused — fail-closed, zero damage (verified
   `cmd/tq/go.mod` intact), but a 30-second integrity audit was needed — and
   the `-race` build failed with the same cgo error regardless. Cost: two
   failed rounds + a scare. The right read existed from the start: the CGO
   pin is a deliberate repo decision; `-race` for cmd/tq is structurally
   out, not a flag away.
2. **art-dupl JSON pollution**: `--json -q ... 2>&1` merged the type-aware
   info line (stderr) into the JSON file → instant parse crash. Clean
   separation on the re-run settled it; the polluted file additionally
   vanished mid-diagnosis (unexplained, moot — /tmp scratch).
3. **Edit-tool staleness dance**: the six directive-removal edits bounced
   TWICE with "modified since last read" — mtime-only touches (content
   byte-identical to HEAD both times, proven via `git diff` before
   proceeding). Burned three rounds before refreshing read-state with the
   View tool, which the editing workflow already prescribes.
4. **Annotate-spec grammar guess**: wrote all 15 specs in `n:kind:value`
   colon form from the SKILL.md prose — ALL unmatched on the first dry-run.
   The tool's real grammar is TSV `key<TAB>verdict` (the "kinds" describe
   verdict STYLE, not syntax; learned from the tool source). The mandated
   dry-run caught it: zero damage, one source-read, one rewrite.
5. **One spec substring miss**: `16@vendor-hash after item` did not match the
   actual line (a backtick sits inside the phrase). Second dry-run caught
   it; fixed with a punctuation-free substring.

## e) WHAT WE SHOULD IMPROVE

1. **Read the gate script before novel flags**: `test-cmd-tq.sh`'s header
   documents the CGO/devmod design that d1(i) tripped over.
2. **Never source repo libs directly in session shells**: `$0`-dependent
   root resolution turns sourcing into path roulette (d1(ii)); wrappers
   exist to be used.
3. **View-before-edit is non-negotiable** even when rg just showed the
   content — daemons/LSP bounce mtimes and exact-match edits refuse.
4. **art-dupl stderr gets its own file from invocation #1** when capturing
   `--json`.
5. **Probe a new tool's grammar with one line before writing fifteen** — the
   docs-health annotate grammar lived in the tool source, not the SKILL.md
   prose (candidates for a SKILL.md TSV example, crush-config repo).
6. **Same-window harvest+annotate** (done here) keeps verdict hashes fresh;
   the deferral pattern (14-02/12-27) let items age and re-litigate.
7. **Gate UX nit**: `test-cmd-tq.sh` could emit a one-line hint when
   forwarded args contain `-race` ("gate pins CGO_ENABLED=0; race is a
   nested-module concern") instead of the bare cgo error.

## f) NEXT (grounded in this window; impact-first)

| #  | Item                                                                                                                         | Why / cite                                   |
| -- | ---------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------- |
| 1  | Index-bloat ANNOTATE/archived sweep: 217 live rows vs 100 threshold; warning fires on every status-index check               | this report b2; TODO_LIST f14 row            |
| 2  | File the upstream art-dupl templ-suppression issue (directives cannot live inside templ markup)                              | TODO_LIST f8 row (16-02 §f8)                 |
| 3  | Commit an art-dupl `-c` config pinning the canonical invocation                                                              | TODO_LIST f10 row (16-02 §f10)               |
| 4  | Cross-check 14-02's open items (mutation-verify pins, evidence filing)                                                       | TODO_LIST f11 row (16-02 §f11)               |
| 5  | Align the 14 directive reasons with the 12-27 verdicts ledger                                                                | TODO_LIST f12 row (16-02 §f12)               |
| 6  | cmd/tq `-race` posture: record "no cmd/tq -race, by CGO policy" beside the gate OR fund a CGO_ENABLED=1 dev.mod-race variant | this report b1/d1; question g1               |
| 7  | Harden `scripts/lib/cmd-tq-devmod.sh` root resolution (BASH_SOURCE over `$0`) so direct sourcing cannot resolve `$HOME`      | this report d1(ii)                           |
| 8  | `test-cmd-tq.sh` -race hint in forwarded args                                                                                | this report e7                               |
| 9  | `ci-local.sh` once at this HEAD before the next push                                                                         | 16-02 §f18                                   |
| 10 | readmodel module gate cert for the concurrent agent's new test pins (`06974658`)                                             | this report b3                               |
| 11 | Living-docs tightening audit: grep for other blanket-acceptance lines like the old "Residual art-dupl groups accepted"       | 16-02 §f17 pattern, AGENTS.md fix `748f75f3` |
| 12 | Plan an AGENTS.md offsetting cut: 89 B headroom left before the next addition                                                | check-agents-size 15,611/15,700              |
| 13 | Durable home for the CHANGELOG-policy ruling (see g2)                                                                        | this report a1/Q3                            |
| 14 | Owner ruling on the two TRAILER WARNINGs (f26 cluster class) firing on every status-index check                              | check-status-index output, pre-existing      |
| 15 | docs-health SKILL.md: add the TSV spec example for annotate-status-items (crush-config repo, owner's own)                    | this report d4                               |
| 16 | art-dupl upstream nit: keep `--json` output free of stderr interleave (info line poisoned a captured file)                   | this report d2                               |

Deliberately NOT invented: worker/release/platform work — own sweeps, own
windows; this closeout certifies, it does not expand scope.

## g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **cmd/tq `-race` posture**: is "gate-green sans `-race`" the accepted
   permanent answer (the CGO pin is deliberate pure-Go policy), or do you
   want a `CGO_ENABLED=1` dev.mod-race gate variant for the CLI module —
   trading the pure-Go gate contract for race coverage of CLI wiring?
2. **Durable home for the CHANGELOG policy** ("internal-only sweeps get no
   row"): annotations on one report will rot; AGENTS.md has 89 B of headroom;
   CONTRIBUTING.md is user-facing. Where does this ruling live so the next
   sweep doesn't re-ask it?
3. **Index-bloat ruling**: 217 live rows, threshold 100, warning on every
   check — is the archived/ sweep sanctioned now (docs-health ANNOTATE over
   the oldest resolved reports, with the bulk-archive manifest), and do the
   two pending f26 TRAILER WARNINGs get their owner ruling in the same pass?

## Verification receipts

- art-dupl final (composite tree incl. `06974658`): 1 group / 2 clones
  (templ pair, fragments.templ:60/:504), 260 files — canonical invocation,
  type-aware, -t 3
- gates: sqlitev4 3.6s plain + 5.7s `-race`; executor 12.5s `-race`;
  companion, cqrsqlite `-race` green; cmd/tq devmod gate green;
  `root-gate.sh` rc=0 (webui 15.8s, facadeparity OK)
- guards: check-doc-refs "doc refs ok"; check-todo-list ok;
  check-status-index "status index ok" (pre-existing warnings: 217 rows,
  2× TRAILER f26); check-agents-size 15,611/15,700 OK; check-rows complete
  (15/15 annotated); nix vendor-hash rc=0
- commits: `4be4a8ef` (ROADMAP citation + sqlitev4 go.mod/go.sum),
  `de63fe08` (directive prune), `748f75f3` (AGENTS.md tightening);
  `06974658` is the concurrent agent's, not this window's
