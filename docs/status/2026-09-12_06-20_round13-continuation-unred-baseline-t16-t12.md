# Round-13 continuation — un-red, baseline triage, T16+T12 execution

- **When**: 2026-09-12 05:00–06:20 CEST
- **Trigger**: owner prompt "Execute and Verify them one step at the time.
  Repeat until done" on the round-13 continuation state (04-59 self-review
  §g questions still unanswered at start).

## a) Fully done (verified this window)

1. **Master CI un-red**: HEAD was RED — the concurrent `internal/session`
   lineage landed gofumpt violations that failed CI's nix treefmt check
   (34669686887). Reproduced locally (`nix run .#fmt`: 2 files), proved
   format-only (build+vet+race on both packages), pushed 2f70deb;
   run 34670079022 SUCCESS.
2. **Lint-baseline triage + deliberate regen** (the 253-row debt):
   function-level attribution — root lll +1 was OURS (payload_test.go,
   4 long raw-string lines → wrapped content-identical, lll 0); the rest
   was concurrent-lineage code (session/httpapi/task-status/executor/
   `Store.ListWatermarks` varnamelen `e` at sqlite.go:1342, landed 05:02
   via 5afd2b0) — covered by the sanctioned deliberate regen (895de1e8);
   gate green at HEAD (820 findings, within baseline).
3. **Baseline attribution aid** (273): `--check` failure path now lints
   each affected module once and names the files per growing linter;
   pipefail-safe + colon-strip fixes found by real red-probes; restored
   green after every probe.
4. **T16 complete**: RELEASE.md multi-commits-per-task section (with
   commitsForTask degradation notes verified in cmd/tq/main.go), clean-room
   module-tag verification recipe (go get + `go list -m all` as the gate),
   fixture-execution caveat (nix leg stubbed), retro SCAFFOLD
   (FIRST-MULTI-MODULE-RELEASE-RETRO.md), and the doc↔script drift smoke
   scripts/check-release-docs.sh wired into ci-local — whose FIRST run
   caught real drift (a line-wrap had taken the sub-tag command out of
   greppability).
5. **T12 complete**: papdashboard-e2e.sh honors TQ_BIN + prints
   `tq version` first in both branches (proven green both ways;
   release-gates.sh verified tq-binary-free → guard not applicable);
   amend maneuver documented as the rule for daemon-folded unindexed
   reports (AGENTS.md + the script's UNINDEXED message); daemon
   attribution proposal landed as **O7** in the rulings package.
6. **Fuzz nightly failure diagnosed + fixed**: the 03:30Z scheduled run
   died at SETUP (GOEXPERIMENT=jsonv2 missing in fuzz.yml — ci.yml has it
   workflow-wide, fuzz.yml never did) and nightly.sh misreported the
   build failure as "FUZZ FAILURE ... wrote the crash input". fuzz.yml
   now carries the env (actionlint ok); nightly.sh differentiates
   CAMPAIGN SETUP FAILED (no crasher written) from real FUZZ FAILURE —
   both paths proven live (3s campaign green with env; honest message
   without).
7. **Proofs owed from 04-59 §d3**: `nix build` + `tq version` →
   `tq 0.2.0, go1.26.7-X:jsonv2` (ldflags interpolation real);
   `check-webui-css.sh` → in sync; full root gate
   (build+vet+`go test ./... -race`) green at window end; L280 CI-proof
   gap closed (34670475702 SUCCESS across the whole batch, incl.
   test-windows on session.go's git usage).
8. **CHANGELOG backfill** (the 04-59 §d1 sin): seven entries added for
   round-13's user-visible changes (doctor go-env probe, project-filter
   UX, baseline+version gates, triage-encoded CI scanning, tq version
   ldflags fix, filter escaping/clamp).
9. **Report correction**: the 03-21 execution report's refuted
   L147/L150 "genuinely open" verdict now carries an inline CORRECTION
   annotation (docs-health: annotate, never rewrite); TODO rows already
   carried the correction.

## b) Partially done / still gated

- T9: ruling package (O2) complete; test pins + executor gate teeth wait
  for the ruling. T10/T11: gated on O2/O3. T14 residue: gated on O4.
- T18/T19/T21/T22/T23/T24/T25/T26: not started this window (see f).

## c) Owner actions that un-block the most

- **O1–O7 ANSWER lines** (docs/planning/2026-09-12_02-48_OWNER-RULINGS-
  PACKAGE-O1-O6.md) — O7 is new this window (daemon attribution).
- **Push policy** (the BLOCKED 138 row): this window pushed twice —
  2f70deb (treefmt repair, master was red) and the e0dadb5 batch
  (already-daemon-committed round-13 residue + this window's docs/gates
  landed locally after). Everything from mid-window on is LOCAL,
  awaiting the ruling. Note: the fuzz.yml env fix should ride the next
  push before the next 03:17Z nightly or it fails again.
- §g questions from 04-59 (CHANGELOG policy, next-window priority)
  remain open.

## f) Next-window queue (highest leverage first)

1. Answer O1–O7 (un-blocks T9 teeth, T10, T11, T14 residue, T18/T26/T27).
2. Push the local batch (fuzz.yml env fix + gates + docs) — or authorize
   autonomous push so repairs and proof-carrying pushes stop being a
   judgment call.
3. T23 small-fix sweep (nine sub-12min items, incl. the varnamelen 2
   sites that keep flapping scoped-vs-full lint).
4. T13 review/status pipeline hardening (re-dispatch loop's TOP fix).
5. T19 pool-health tooling. 6. T18 fullcore/postgres proof.
7. T25 CI consistency batch. 8. T22 docs-health batch (six 2026-09-07
   reports still unannotated). 9. T24 templ-components evals.
10. T21 ADR prep once O6's release call lands.
