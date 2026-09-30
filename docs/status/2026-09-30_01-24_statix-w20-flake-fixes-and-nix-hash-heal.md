# 2026-09-30 01-24 — statix W20 fixes in flake.nix + two pre-existing nix-hash heals

**Session type**: INTERACTIVE owner session (no task ID, no pool dispatch). Owner pasted
`statix check` output (exit 1, two W20 "repeated keys" warnings in flake.nix) and asked for
READ → UNDERSTAND → fix → verify until done, then a full status report.
**Window**: ~00:45–01:24 CEST. **Scope discipline**: flake.nix only; zero Go/source changes;
no pushes; no commits by hand (auto-commit daemon swept everything — see §d8).
**Concurrent activity**: a sibling window shipped the Row-150 third re-dispatch close-out at
01:03 (`2026-09-30_01-03_task-…ef24.md`) mid-window; no collisions (disjoint files).

## Gates run (all rc-captured at final HEAD 06c3da66)

| Gate                               | Result                                                                                                                                                        |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `statix check`                     | rc=0, zero findings (was rc=1 with 2× W20)                                                                                                                    |
| `nix fmt` (treefmt over 251 files) | rc=0; first pass normalized my hand-indent (1 changed), final pass **0 changed**                                                                              |
| `nix flake show`                   | all three systems evaluate (x86_64-linux, aarch64-linux, aarch64-darwin)                                                                                      |
| `nix flake check`                  | rc=0, **"all checks passed"** — 6 checks: treefmt-check, package build, vendor-hash, binary-runs, nixos-module-eval, version-sync (log: /tmp/flake-check.log) |
| `nix build`                        | rc=0                                                                                                                                                          |
| App evaluation                     | all five `apps.*` eval with intact descriptions; `lib.mkForce` on `apps.test` preserved                                                                       |

## §a) FULLY DONE

1. **statix W20 #1 — `treefmt` ×3 dotted paths merged** into one nested
   `treefmt.settings = { excludes = …; formatter = { templ.command/goimports.command } }`
   literal (flake.nix:179–196). Both comment blocks preserved verbatim; `lib.mkForce` kept on
   the leaf options. Landed in daemon commit 702ee8b4.
2. **statix W20 #2 — `apps` ×5 assignments merged** into one `apps = { test/default/lint/fmt/webui-css }`
   literal (flake.nix:407–459). Uniform +2 indent is value-preserving for Nix indented strings
   (common prefix stripped), so both embedded shell scripts are byte-identical — verified by
   full line-by-line diff read of 702ee8b4 (62+/58−), not by pattern grep (see §d2).
3. **Pre-existing defect healed on sight — `goTarballHash` was a vendorHash copy-paste**
   (flake.nix:45): held `sha256-M66RZ…` (identical to `vendorHash`) instead of the real
   go 1.27.1 source-tarball hash. Corrected to `sha256-TkCKuu…` — the value proven by the FOD's
   own `got:` error AND identical to the working perSystem formatter copy at flake.nix:150.
   Predates the tree (confirmed via `git show HEAD~1:flake.nix`). Landed in 06c3da66.
4. **`vendorHash` refreshed** (flake.nix:40): pasted the FOD's `got:`
   `sha256-USH5j7+aEgqJPwGRSq0I5nmDVFoLbigGKCshpsS/iI4=` per the repo's documented drift
   runbook (no buildflow here — no `.buildflow.yml`, verified). Landed in 06c3da66.
   **PROVENANCE CORRECTED IN-WINDOW (§d9)**: initially written up as "pre-existing stale" —
   that is UNPROVEN and contradicted by the concurrent 01-03 report, whose ~00:50 battery ran
   `nix build .#checks.x86_64-linux.vendor-hash` rc=0 against the SAME M66RZ literal for tree
   702ee8b4^. Between that green check and my failing one, only flake.nix text changed (my
   restructure + goTarballHash fix), yet the modules-FOD output hash is content-derived
   (go.mod/go.sum unchanged) — so either flake.nix IS a FOD src input, or the sibling check
   exercised different inputs than its report states. Both hypotheses need one verification
   pass (§f5). What IS proven: USH5j is the correct specified hash for the final tree —
   flake check "all checks passed" at 06c3da66 is the citation.
5. **End-to-end proof the restructured treefmt config still works**: `nix fmt` and
   `nix flake check`'s `treefmt-check` both green — that check runs the exact merged
   excludes + templ/goimports wrappers over the whole tree.
6. **statix wiring census (verified, not guessed)**: `rg statix` over scripts/ci-local.sh,
   scripts/check-guard-wiring.sh, .github/workflows/ci.yml → **zero matches**. statix is
   manual-only in this repo today.
7. **Diff hygiene**: every landed hunk read in full before finishing; `pkgs.lib.getExe`
   inconsistency (webui-css) consciously left untouched (scope discipline, see §e6).

## §b) PARTIALLY DONE

1. **"The hash heals fix the red master nix job"** — hypothesis, now NARROWED: only the
   goTarballHash defect is unconditional (M66RZ ≠ tarball bytes, fails whenever realized);
   the vendorHash staleness provenance is unresolved (§a4 correction) and may even be
   tree-relative rather than pre-existing. The session-start probe
   found master RED at 8ab309b66 (cancelled run, predates tree); prior reports date the red
   nix/test/test-windows jobs to 2026-09-29 ~03:26. Both stale hashes are consistent with
   that, but `scripts/check-ci.sh` was **never re-run** after the heals and nothing was
   pushed — the claim is unverified by construction (citation discipline: no gate output, so
   no DONE-grade claim is made here).
2. **Tarball-hash single-sourcing NOT done** — the W20/hashes are fixed, but go 1.27.1's
   tarball hash now lives in TWO places (go-standard option flake.nix:45 and the perSystem
   fetchurl literal flake.nix:150) with only a manual "keep in sync" comment — the exact
   mechanism that produced defect §a3. Split brain left in place, flagged only.
3. **Report indexing** — this report + index row are written together, but the daemon may
   still split them across commits (footer-less sweep; hooks bypassed). If
   `check-status-index.sh` goes red in ci-local, the amend/follow-up-index maneuver applies.

## §c) NOT STARTED (all noticed this session, none begun)

1. `scripts/check-ci.sh` re-probe after the heals (§b1).
2. `./scripts/ci-local.sh` — the pre-push gate never run this session (nix leg run manually
   only).
3. TODO_LIST rows for this session's findings (§f items 3–6) — none filed, against the repo's
   same-session §f culture.
4. AGENTS.md update: the vendorHash known-issue bullet doesn't mention that in `nix flake
   check` the goTarball FOD fails BEFORE the go-modules FOD, so a stale vendorHash can hide
   behind a stale goTarballHash (exactly what happened: the tarball error masked the modules
   error until the tarball was fixed).
5. `nix run .#test` — the merged `apps.test` was only EVALUATED (flake check inspects it;
   `nix fmt` exercised apps.fmt), never executed.
6. `nix flake check --all-systems` — default check covered x86_64-linux only ("omitted these
   incompatible systems: aarch64-darwin, aarch64-linux"); `nix flake show` evaluated the
   others but no check ran there.
7. Fleet sweep for the §a3 defect class (same wrong `goTarballHash = <vendorHash>` copy-paste
   in sibling flakes).
8. Commit-heal decision for the footer-less daemon commits carrying this session's work.

## §d) TOTALLY FUCKED UP (own failures this window, no varnish)

1. **PIPESTATUS hazard re-hit — documented-lesson recurrence.** The first background
   `nix flake check … | tail -20; echo "rc=$?"` used exactly the pattern AGENTS.md codifies as
   broken in this shell (PIPESTATUS expands empty; `$?` = tail's status). The rc line printed
   empty/meaningless. I switched to the redirect-to-file pattern only on the second run. The
   lesson exists in AGENTS.md; it is not mechanically enforced on me — I re-derived the hazard
   live.
2. **Two wasted verification attempts before the authoritative one.** (a) `cat` of store paths
   from `nix eval` on UNBUILT derivations (nix eval computes paths, doesn't realize them —
   should have known); (b) a sed range-diff whose end-pattern anchors (`^          };$` vs
   `^            };$`) can't align across the old/new layouts — garbage output, correctly
   discarded. The right tool (read the full 120-line diff) was available from the start. In a
   paid task window that's budget burn; here it was sloppy sequencing.
3. **Stale-read edit rejection.** Edited goTarballHash against a pre-`nix fmt` read; the
   formatter had rewritten the file between my view and the edit. Protocol (re-read after any
   external rewrite) caught it, but I should have re-read proactively — `nix fmt` had JUST run
   in the same command.
4. **Serial fix-verify cycles instead of one `--keep-going` pass.** `nix build --keep-going`
   would have surfaced BOTH hash mismatches (tarball + modules) in a single cycle; I ran
   flake check three times, discovering one stale hash per run. Each cycle re-paid the
   treefmt-check wait.
5. **Unverified hypothesis shipped in the closing message.** "Very plausibly the red master
   nix job" — plausible, consistent with the evidence, and hedged, but stated before any
   re-probe. §b1 owns it; the honest grade is hypothesis, not finding.
6. **No same-session row filing.** The repo's §f standard ("grounded NOT padded, filed
   same-session") was skipped entirely — findings 1–7 below exist only in this report until
   harvested.
7. **The §a3 defect sat in the file for the whole window unnoticed until flake check tripped
   over it.** I had read flake.nix:33–131 in full at discovery time — `goTarballHash` =
   `vendorHash` (two adjacent lines, identical strings) should have been flagged by eye, per
   the "fix issues on sight" doctrine, not by build failure.
8. **All session work landed in footer-less daemon commits** (702ee8b4 restructures,
   06c3da66 hash heals) — attribution receipts #6/#7 for the known daemon-commit gap census.
   No heal attempted (interactive session, no task ID to footer); the census row grows.
9. **Encoded an unverified provenance claim into the first draft of this report.** §a4
   originally said "pre-existing stale vendorHash" — written from the FOD error alone, before
   reading the concurrent window's counter-evidence (M66RZ green at ~00:50, README.md:772).
   Citation discipline caught it during indexing, not during writing; the correction above
   is the in-window heal, but the right order was evidence-first, claim-second.

**Did I lie?** No. Every gate claim above carries its rc and every fix cites its commit.
The one soft claim (§b1) is explicitly downgraded to hypothesis.

## §e) WHAT WE SHOULD IMPROVE

1. **Mechanically enforce the rc-capture pattern on me**: the PIPESTATUS lesson keeps
   recurring because it lives in prose. A `check-agent-shell-habits`-style gate can't see
   agent commands, but session reports can carry a standing receipt line — cheap and shame-
   based (works: I caught #1 myself only because the empty rc looked wrong).
2. **`--keep-going` first in flake-hash triage**: when a fixed-output hash fails, re-run the
   build once with `--keep-going` to enumerate ALL stale hashes before editing. One cycle
   instead of N. (Candidate one-liner for the AGENTS.md vendorHash bullet.)
3. **Eye-level hash-twin check at read time**: adjacent same-format hash literals are a
   copy-paste smell — verify independence on sight, don't wait for the FOD to fail.
4. **Verification tooling order**: for "did the restructure change any string VALUE", go
   straight to the full diff read; store-path catting and pattern-range greps are both
   wrong tools here (nix eval doesn't realize; range anchors don't survive re-indent).
5. **Split-brain hygiene for hashes**: any hash literal appearing twice in one file needs
   either a `let` single-source or a generated check (§f2).
6. **Scope-call documentation**: skipping `pkgs.lib.getExe`→`lib.getExe` was a judgment call
   made silently; one clause in the final message would have made it auditable (it is now, in
   §a7).

## §f) NEXT (grounded, ranked by impact — 22 items, NOT padded to 50)

| #  | Item                                                                                                                                                                                                                                                               | Why it matters                                                                                                                                       | Size |
| -- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------- | ---- |
| 1  | Re-probe `scripts/check-ci.sh`, then `./scripts/ci-local.sh`, then push                                                                                                                                                                                            | Converts §b1 from hypothesis to verified; unblocks the red-master carry                                                                              | S    |
| 2  | Single-source the go 1.27.1 tarball hash (flake.nix:45 vs :150; perSystem reads `config.go-standard.goTarballHash` or a shared let)                                                                                                                                | Kills the split brain that produced §a3; the manual-sync comment already failed once                                                                 | S    |
| 3  | File TODO rows: statix unwired (§a6), tarball-hash twin (§b2), vendorHash-masking note (§c4), PIPESTATUS receipt (§d1), footer-less receipts #6/#7 (§d8)                                                                                                           | Repo culture: findings without rows rot; this report is entombed otherwise                                                                           | S    |
| 4  | Wire statix into the gates: `scripts/check-statix.sh` (pinned invocation, fail on findings) + ci-local step + check-guard-wiring registration — or record an explicit manual-only ruling                                                                           | Today ONLY a human running statix by hand catches W20-class nix rot; the two warnings proved the hole is real                                        | M    |
| 5  | Date the §a3 defect (`git log -S goTarballHash` → be59f26f/17e0139b/741a0849 candidates) and resolve the vendorHash provenance contradiction (§a4/§d9): is flake.nix a go-modules FOD src input, or did the sibling window's 00:50 rc=0 exercise different inputs? | Answers whether the stale hashes were THE master-red cause or a second independent break; also pins the FOD-input model the whole fleet reasons from | S    |
| 6  | AGENTS.md: vendorHash bullet gains the tarball-masks-modules FOD ordering + the `--keep-going` triage step                                                                                                                                                         | Encodes this window's two new lessons where the next window reads                                                                                    | S    |
| 7  | Sweep sibling LarsArtmann flakes for the `goTarballHash = <vendorHash>` copy-paste class                                                                                                                                                                           | Same template, same mistake likely duplicated fleet-wide                                                                                             | M    |
| 8  | Execute `nix run .#test` once at HEAD                                                                                                                                                                                                                              | The merged `apps.test` is eval-verified only; one real run closes it                                                                                 | M    |
| 9  | `nix flake check --all-systems` once locally                                                                                                                                                                                                                       | aarch64-linux/darwin currently show-evaluated, never checked                                                                                         | S    |
| 10 | Daemon-attribution decision for 702ee8b4/06c3da66: heal into one footered commit (unpushed soft-reset playbook) or accept as interactive-session receipts                                                                                                          | The two flake commits derive as zero in the queue; census grows either way — ruling wanted (§g3)                                                     | S    |
| 11 | `pkgs.lib.getExe` → `lib.getExe` at the webui-css app (flake.nix:~446)                                                                                                                                                                                             | Trivial consistency nit, flagged by the nix-review checklist, consciously deferred                                                                   | XS   |
| 12 | Track the goTarball drop-day (nixpkgs ships go ≥ 1.27.1 → delete goTarball + formatterWithGo blocks) as a row, not just comments                                                                                                                                   | The blocks carry three "drop with…" comments and no tracker                                                                                          | XS   |
| 13 | If the daemon split report from index row: follow-up indexing commit immediately (check-status-index.sh is the catcher)                                                                                                                                            | Known hole; verify after the next daemon sweep                                                                                                       | XS   |
| 14 | Re-read `docs/status/2026-09-30_01-03_task-…ef24.md` (concurrent window) for overlap with this report's findings before harvesting §f                                                                                                                              | Concurrent session may have filed overlapping rows at 01:03                                                                                          | XS   |
| 15 | Consider `.statix.toml` (pin severity/ignore policy) when item 4 lands                                                                                                                                                                                             | Keeps the future gate stable against new statix releases                                                                                             | XS   |
| 16 | Add `--keep-going` to the ci-local nix step? (enumerate all FOD failures per run)                                                                                                                                                                                  | ci-local currently stops at the first hash mismatch too                                                                                              | XS   |
| 17 | Verify `checks.format` covers `.nix` files repo-wide (statix's complement) — treefmt-check passed with nixfmt; confirm deadnix-or-similar decision is documented                                                                                                   | Nix-file quality is currently: nixfmt yes, statix no, deadnix unknown                                                                                | S    |
| 18 | Post-push: confirm the nix CI job green on the heal commit and annotate §b1 with the run URL                                                                                                                                                                       | Closes the master-red carry                                                                                                                          | S    |
| 19 | Harvest this §f into TODO_LIST.md per docs-health (most items are row-sized; items 7/16 are ROADMAP fuel)                                                                                                                                                          | Entombment prevention — this report's §f is the input, not the home                                                                                  | S    |
| 20 | lessons.md candidate (cross-project): "nix eval computes, build realizes — never verify script text by catted store paths"                                                                                                                                         | The §d2(a) failure generalizes beyond this repo                                                                                                      | XS   |
| 21 | Consider making the vendor-hash fast gate (checks.vendor-hash) part of the session-start ritual for flake-touching windows                                                                                                                                         | Drift then fails in seconds, not at flake-check depth                                                                                                | XS   |
| 22 | Re-run `statix check` after any future flake edit in this repo until item 4 lands                                                                                                                                                                                  | Manual discipline bridging the gap                                                                                                                   | XS   |

## §g) QUESTIONS I CANNOT FIGURE OUT MYSELF

1. **Tarball-hash single-sourcing style**: may perSystem read `config.go-standard.goTarballHash`
   (flake-parts config access across module scopes) for flake.nix:150, or do you prefer the
   literal+manual-sync-comment shape in this fleet? The read kills the split brain; the
   literal keeps the perSystem block copy-pasteable to flakes that don't use go-standard.
2. **statix gate ownership**: wire statix into ci-local/CI via a new `check-statix.sh` (with
   check-guard-wiring registration), or is nix-lint deliberately manual here pending a
   buildflow adoption decision (this repo has no `.buildflow.yml`, and buildflow owns statix
   fleet-wide)? My recommendation is the check script; the ruling is yours.
3. **Footer-less heal for this session**: 702ee8b4 (restructures) + 06c3da66 (hash heals) are
   unpushed daemon commits deriving as zero. Heal into a single footered commit via the
   unpushed soft-reset playbook, or leave as-is and accept two more census receipts since
   this was an interactive no-task window?

---

_Gates cited: statix rc=0 · nix fmt rc=0 (0 changed) · nix flake show all-systems eval ·
nix flake check rc=0 "all checks passed" · nix build rc=0 — all at 06c3da66. Commits:
702ee8b4 (W20 restructures), 06c3da66 (goTarballHash + vendorHash heals). Master status at
session start: RED (8ab309b66-era cancelled run, predates tree) — §b1 carries. Counter-
evidence source: docs/status/README.md:772 (sibling 01-03 report's ~00:50 vendor-hash rc=0)._
