# Done-Prompt Window: five tasks — release-gate hardening trio + projection-home single-opener arc

2026-10-11 02:30 CEST · one status reporter · branch master · window spanned 2026-10-09 03:46 → 2026-10-11 00:43 (task execution), report written 02:30.

Window tasks: `…e54e6aafb…` (per-target drift messages), `…e62a192…` (unshare -n smoke wrapper), `…e6bcc847…` (proxy-wait die constants), `…dec087009…` (projection-home double-open — DONE-on-arrival no-op re-dispatch), `…312317200…` (tq api readmodel open decision + api pump host adoption). Every claim below was verified against git/TODO_LIST/CHANGELOG at HEAD this pass.

## a) FULLY DONE

1. **Per-target drift messages in check-release-docs.sh** (closeout 2026-10-09 03-46; commit 9080294): `need_in_both` now greps release.sh then scripts/lib with per-target miss lines, union semantics preserved (verified: closeout documents the naive split broke the gate live and the nested-if shape fixed it; the TODO row is ticked, row 57).
2. **Structural offline claim for the proxy-wait smoke** (closeout 04-01; commit a3f6264): smoke re-execs under `unshare -n` when permitted (`TQ_SMOKE_NETNS` guard var), loud SKIP line otherwise — verified live on this host (EPERM path, 8/8 PASS under SKIP).
3. **Die-message constants as one source of truth** (closeout 04-14; commit dfd3e6c): `TQ_PROXY_DIE_LAG`/`TQ_PROXY_DIE_DEAD` readonly in `scripts/lib/proxy-wait.sh`; the smoke now asserts verbatim full messages (zero substring greps). Smoke 8/8 including the netns path; check-release-docs green.
4. **Projection-home double-open killed** (row 32, ticked): landed in f6492a00 (2026-10-09 22:44) — `readmodel.WithEngine` + ONE shared pragma source (`config.Deployment.ProjectionPragmas`, ADR-0022); `composition.NewProjectionRuntime` opens the one tq-owned engine. Tasks `…dec087…` windows were DONE-on-arrival no-ops (third paid dispatch total on the landed fix — see (d)).
5. **tq api readmodel open DECIDED + api pump host adoption LANDED** (rows 34 and 35, both ticked): decision 202fcea8 (2026-10-10 23:40, api keeps the self-open as a sanctioned per-surface exception; ADR-0016 resolved — composition is an S4 app-composition module, no facade; memo `docs/memos/2026-10-10_readmodel-open-seams.md`). The residual (row 35: api's raw `Model.Run` pump had no restart budget / poison-DLQ) then LANDED via daemon fold 07995bf8 (2026-10-11 00:43): `httpapi.UseReadModel` now builds `readmodel.NewProjectionHost` + best-effort DLQ sidecar, `ListenAndServe` drives Start/Stop/Close — poison facts dead-letter and the fold converges instead of a wedged cursor. Closeout report rides f4c737dc. Verified: rows 34/35 ticked at HEAD with per-module gate rcs cited; `UseReadModel`/`NewProjectionHost` present in internal/httpapi at HEAD.

Net: all five window tasks closed with ticked TODO rows; no open work item from this window's dispatches.

## b) PARTIALLY DONE

1. **The unshare structural branch has never executed** — this host cannot create a netns; only the SKIP path ran (04-01 §b). Residual risk is the 4-token re-exec line being inspected-only. Row minted (see TODO appends).
2. **Cheap-battery-only verification on two closeouts** (04-14 §b): root build+vet+test -race rc was not cited for shell/doc-only diffs — convention as written unmet, escape-hatch undefined (owner question).
3. **The window's §f follow-up pairs were unevenly minted**: the 06-38 report marked two guard-test pins as "worth a row" while minting 0; only one (pragma-union pin) was recovered — by the 00-00 closeout — and the serve call-site-count pin is deliberately deferred until row 33 lands (born-stale otherwise). Correct handling overall, but the self-contradiction cost a re-derive pass.

## c) NOT STARTED

Backlog rows the window skipped, still open at HEAD (verified):

- Row 33: route `tq stats --read-model` + `tq doctor` projection check through the single-opener seam (last two self-open call sites).
- Row 36: verify the readmodel self-open honors `TQ_SQLITE_SYNC` at zero deployment (internal/readmodel/model.go:54).
- Row 37: doctor projection-DLQ path split-brain (`DLQPathFor` derivation mismatch, cmd/tq/doctor.go:861 vs internal/readmodel/dlq.go:54) — TestDoctorProjectionDLQ is red at HEAD and papers over it.
- Row 38: pin the api pump's poison-DLQ WIRE end-to-end (TestUseReadModelArmsManagedHost only asserts non-nil).
- Row 39: flip internal/composition/single_opener.md to LANDED (still reads "Status: DESIGN" at HEAD — verified this pass).
- Row 40: pragma-union equality pin (compose.go:66 vs model.go:54).
- Row 41: fold the composition facade ruling into ADR-0016.
- Release-gate rows 61–64, 66–68: `--publish-steps-only`, reflow-tolerant pins, check-release-docs `--self-test`, `poked demand-fill` pin, smoke literal hoist, success-assertion constant, heal fork-record self-test pin.

## d) TOTALLY FUCKED UP

1. **The re-dispatch class burned THREE paid windows on landed work in this window alone**: row 32 got two phantom re-dispatches (06-32 real closeout, then 06-38 no-op) and row 34 one (00-00 no-op, ~20 min after its footer-carrying landing 202fcea8) — despite the queue-side COMPLETED short-circuit being ticked DONE (row 177). Each phantom is a full paid window. The 00-00 §g1 hypothesis (stale mint snapshot, or dedup only guards pre-mint completions) is unanswered; minted as a TODO row.
2. **The first need_in_both edit inverted the gate's semantics** (needle-in-either → needle-in-both) and would have shipped a permanently-red release-docs gate — caught within one command because the smoke ran immediately (04-14-adjacent closeout 03-46 §d). The class: "split one grep into two" hides a boolean design decision.
3. **Footer-above-attribution commit** on the unshare task (03-46→04-01 window, caught and amended pre-push) — the exact invisible-footer class AGENTS.md warns about; the queue's commit↔task cross-reference nearly lost the task.
4. **heal-daemon-sweep printed identity fork-records** for a real message rewrite (04-14 §d) — post-heal state verified correct, the record is impossible; display bug suspected, pinned in row 68, root cause open.
5. **Daemon-fold dependence continued**: the row-34 decision (8d94958b) and the pump-host landing (07995bf8) rode footer-less daemon folds; the empty marker commits (f4c737dc, 0d99b6b3) carry the footers. Cross-reference works, but the source-vs-footer commit split persists (rowed process rule, unenforced).

## e) WHAT WE SHOULD IMPROVE

1. **Fix the queue-side re-dispatch dedup for fresh completions** — the highest-leverage item; three burned windows in one window is the datapoint.
2. **Commit-first for source edits** shrinks the daemon race window (04-14 §e2) — the heal was only needed because tests ran before staging.
3. **Exercise the structural (netns) path in CI** or explicitly rule SKIP-forever acceptable — an inspected-only branch is a silent-rot liability.
4. **Encode the re-dispatch contract rules** (symbol-grep fallback after empty footer grep; tick-the-row-in-the-same-commit) somewhere the executor enforces, not just in reports — both rules were followed "this time" only.
5. **Decide the shell message-constant style** (format-string vs message function) before more prose gets constant-ized — three follow-up rows hang on it.
6. **Batch proxy-wait dispatches** — five one-liner dispatches in 24h on one script cluster (04-14 §f25 meta); dispatch overhead is dominating the work.

## f) UP TO 50 NEXT THINGS (report-only; highest-value routed into TODO_LIST under the cap)

1–7 = rows 33, 37, 36, 38, 39, 40, 41 above (the single-opener arc is the window's own tail — finish it).
8. Queue-side dedup regression fix (d1) — minted.
9. netns marker hardening (verify in-netns before honoring the env override) — minted.
10. Privileged CI leg for the smoke's structural branch — minted (owner-gated).
11. heal fork-record identity-SHA root cause — minted (owner-gated).
12. Shell message-constant style ruling — minted (owner-gated).
13. `--publish-steps-only` release resume mode (row 61).
14. check-release-docs `--self-test` pin incl. the 4-case union matrix + exact-message pinning (row 63 + 03-46 §f5).
15. Reflow-tolerant doc pins (row 62).
16. `poked demand-fill` log-line pin (row 64).
17. Smoke literal hoist + success-path constant (rows 66/67).
18. heal self-test: footer-position pin + distinct-SHA pin (row 68).
19. Flatten `need_in_both` to two captured rcs (readability; 03-46 §f1).
20. SKIP-vs-PASS distinction in the smoke summary line (04-14 §f17).
21. Post-row-33: fold the memo's exception wording + the serve call-site-count pin (00-00 §f7/§f9).
22. Repo-wide sweep: other smokes grepping substrings of lib-owned prose (04-14 §f19).
23. Repo-wide sweep: combined "or"-greps in gates with where-did-it-die opacity (03-46 §f7).
24. trim row 32/34 DONE-note accretion to the one-note convention (00-00 §e3) — next docs-health ANNOTATE pass.
25–50: not minted — remaining candidates are already queued elsewhere in TODO_LIST (proxy-wait rows 61–68, single-opener rows 33–41) or owner-gated (rows 21–27, 97); padding would mint duplicate paid dispatches.

## g) QUESTIONS FOR THE OWNER

1. **Queue-side re-dispatch**: is the dispatcher minting from a stale TODO_LIST snapshot (mint-before-commit race), or does the COMPLETED dedup only guard items completed before mint time? Three phantom paid windows in this window; decides whether rows 33/36–41 each burn one too.
2. **Root-battery scope**: do shell/docs-only closeout diffs require the cited root build+vet+test -race rc, or does the verify-window convention get an explicit docs/shell escape hatch?
3. **unshare structural branch**: production-relevant (needs a privileged CI leg / VM check) or best-effort with SKIP-in-containers forever? And is the `TQ_SMOKE_NETNS` env override allowed to stand?

## h) BAND DRIFT (ADR-0015 accountability)

The journal reachable from this shell (`./tasks.db`, `TQ_DB` unset in the session shell) holds 397 facts ending 2026-10-08 00:44 — it predates this window. `tq facts -type task.reprioritized` returns **0 facts**, and the production journal (deployed pool on evo-x2, per TODO row 19) is not reachable from this environment. **No reprioritized facts are recorded anywhere this reporter can verify: none recorded.** Priority movement in this window happened implicitly through TODO row ordering and dispatch order, not through `task.reprioritized` facts; if the deployed journal holds moves from this window, they should be folded into the next report from a shell with `TQ_DB` pointed at it.

## Cross-checks performed this pass

- All five closeout reports read (docs/status/tasks/2026-10-09_03-46, 04-01, 04-14; 2026-10-10_06-38; 2026-10-11_00-00).
- TODO rows 32–41, 57–60, 66–68 verified ticked/open at HEAD; landing commits f6492a00, 202fcea8 (8d94958b), 07995bf8, a3f6264, dfd3e6c, 9080294 verified in git.
- CHANGELOG synced this pass (see the three [Unreleased] entries added 02:30) — the single-opener fix, the api pump host adoption, and the release-gate hardening trio were absent before.
- A full docs-health ANNOTATE sweep (fifth) completed at 02:03, ~30 min before this report (docs/status/2026-10-11_01-43 + 02-03); this report deliberately does not re-run it.
