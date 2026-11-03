# Done-prompt window report — eighth pass, verification-only

**Window under report (pass written 2026-11-02 20:18):** the SAME 17-task window,
first dispatched 2026-10-30 05:31 and closing 2026-10-31 04:18 (13 a)-g) closeouts
+ 3 free-completed tickets + the push-master task). **Honest framing first:** this
is the EIGHTH done-prompt pass over this window in one day (26th 12:26, 27th 17:16,
28th 17:43, 29th 18:15, fifth 18:52, sixth 19:13, seventh 19:49 — all in
`docs/status/`, all indexed). Per the standing dispatch-dedup convention
(fifth/sixth/seventh pass precedent; the queue-side dedup row is still BLOCKED on
the owner ruling), this pass is verification-only: no window fact is re-derived,
headline artifacts are re-grepped at HEAD, and the *delta since the seventh pass
(19:49)* is recorded. Work evidence per task: 13 ancestor-verified commits
(`df2addff`…`1b5a7825`, `git merge-base --is-ancestor` checked this pass), the two
zero-footer reviewer tickets (`…ba892ab9`, `…ba892bb3`) confirmed completed-free in
the production journal (attempts=0; preflight "fix ticket cured — SHA df3ce1c0
unreferenced and anchor text absent at HEAD"), and the push-master task
(`…f208e533`) achieved in the v0.4.1 wave per the 17:16/26th passes (cited, not
re-derived).

## a) FULLY DONE (re-verified at HEAD 8e0605c7, not re-derived)

All headline artifacts from the prior passes re-confirmed green this pass:

1. **Header-matrix chain (internal/webui)** — `internal/webui/headermatrix_test.go`
   carries 12 `TestServeHeaderMatrix*` hits (six per-surface funcs + RouteCoverage
   meta-guard), the package-level `serve429Row`/`serveFaviconRow` dedup, and the
   `with(rows …)` single merge mechanism (prior passes: 29 total pin-site hits).
2. **Failure-evidence surface** — `internal/executor/evidence.go` present
   (`LastFailureEvidence`/`LastFailureEvidenceFromTrail`); the `--evidence-stage`
   comma/`!`-negation set spec and the `evidence=` suffix remain one copy in
   executor (`MatchesEvidenceStageSet`); the dead-only requeue pin and the
   dlq-evidence-stage smoke stay green per the 19-43 batch.
3. **Secret-evidence pins** — `TestSecretPatternSamplesRawCountExactlyOne` present
   in `internal/executor/redact_test.go`; the cmd/tq audit-JSON contract tests
   unchanged.
4. **Gates landed by the window** — `scripts/check-prompt-const-folds.sh` exists
   and runs green this pass (`ok: no footer-less daemon sweeps touched prompt-const
   files since origin/master`); the status-index path gate is green (rc=0 this
   pass, mutation-proofed battery per `83b453a2`); AGENTS.md dry-run-first bullet
   present; derived hook-case count unchanged.
5. **Digest ritual** — `digest 2026-11` rows live in `docs/status/README.md`
   (8 hits this pass); 32 ANNOTATED blocks still present across the live 2026-09
   reports.
6. **Living docs** — CHANGELOG carries the evidence-stage set-spec / webui
   evidence-filter rows; FEATURES.md rows current (failure-evidence
   FULLY_FUNCTIONAL, priority ADR-0015 row intact); AGENTS.md already documents
   the set filter. No drift found this pass beyond §d below.

## b) PARTIALLY DONE (cited, not re-derived)

1. **Owner-gated §g1/§g2 SECURITY.md rows** (generation-vs-pin ceiling;
   bare-probe Cache-Control) — still BLOCKED, re-asked verbatim in ≥5 window
   reports; unchanged this pass.
2. **Push-master lineage** — the task's own objective (push master so CI re-runs
   green) was achieved in the v0.4.1 wave, but the CURRENT local master is 35
   commits ahead of the local origin/master tracking ref and origin CI ran RED ×2
   on the later pushes (per the 18:15/19:49 passes). The follow-up work keeps
   landing locally faster than it is pushed; the standing master-CI-red cure row
   owns this.

## c) NOT STARTED

Standing backlog rows the window skipped: nothing new — every §f candidate this
pass deduped against the standing rows (the 26th-dispatch append block, rows
1303/1159/1223/1429/1432-1441 and older), zero mints (§f below).

## d) TOTALLY FUCKED UP (delta since the seventh pass)

1. **check-dead-sha-refs RED at HEAD — 3 FRESH dangling cites**, all in reports
   written TODAY by concurrent dispatches: `tasks/2026-11-02_18-47…:27` cites
   `d7c25ae8 (no reachable counterpart, baselined 2026-11-03)`, `tasks/2026-11-02_19-43…:8` cites `719d980e→73fc92da`,
   `tasks/2026-11-02_20-04…:48` cites `ff0f16d6→b88e2fa9` (each "commit exists, unreachable
   from HEAD"). Same recurring class (daemon-sweep heals orphan the cited commit);
   tracked by the standing consolidated dangling-citation cure row — NOT
   re-minted, but the rate (3 in one afternoon) is the finding.
2. **AGENTS.md size budget at 24,698/24,700 B — 2 B headroom** after the
   sanctioned 24,400→24,700 reset. Every docs pass is one bullet away from a
   hard red; the prune-per-add / budget terminal policy question remains open.
3. **Eight dispatches in one day** over one window is itself the standing §d
   debt — the queue-side window dedup row is still BLOCKED; this pass adds the
   eighth data point (see §e).
4. Carried, unchanged: check-status-index 2 pre-existing f26 TRAILER WARNINGs;
   todo-list gate green with 12 advisory warns.

## e) WHAT WE SHOULD IMPROVE (process and code)

1. **Dispatch-side window dedup is the single highest-value unblock.** Eight
   paid passes over one window in one day; each prior pass proposed the same
   refusal rule ("re-dispatch of a window whose report exists at HEAD →
   verification-only or refuse"). It needs the owner ruling to become queue
   behavior, not pass-level discipline.
2. **Heal-created SHA citations need a gate-visible provenance.** Every dangling
   cite today originates in a report that CITED A COMMIT THE HEAL/REBASE was
   about to orphan. A convention line ("cite the reachable successor, or write
   the fork record BEFORE the heal") would kill the class at the source instead
   of sweeping corpses later.
3. **AGENTS.md budget is structurally exhausted again** (2 B). Third reset-class
   event in two days. The prune-first-default ruling (asked 2026-10-30 08-21
   §g3) decides whether every future docs pass burns a size surgery.
4. No new code improvement surfaced this pass; the window's standing improvement
   list (daemon-sweep `--split` heal mode, shared `daemon-scan.sh` lib,
   sabotage legs for the remaining countable gates, templ CLI drift gate)
   remains accurate and rowed.

## f) NEXT THINGS

**0 rows appended this pass.** Every candidate deduped against the standing
unchecked rows per the fifth/sixth/seventh-pass discipline: the three fresh
dead-SHA cites → standing dangling-citation cure row; master CI-red → standing
cure row; §g1/§g2 → standing BLOCKED rows; AGENTS budget → standing size-policy
row. Minting any of them again would be exactly the reworded-duplicate loop the
2026-09-11 review found.

## g) QUESTIONS FOR THE OWNER

**None new.** The four standing BLOCKED rulings cover every open question this
pass touched: (1) queue-side re-dispatch/window dedup authority; (2) AGENTS.md
size budget terminal policy; (3) §g1/§g2 SECURITY.md owner-gated rows; (4)
heal-sweep multi-item/`--split` policy. All already rowed with `— BLOCKED:`.

## h) BAND DRIFT

**None recorded.** Re-read the production journal
(`/mnt/pool/services/tq/tq.db`, read-only) this pass: zero `task.reprioritized`
facts (eighth consecutive none-census). The only grep hits for the term are
agents' own progress narration embedded in unrelated `task.requeued` detail
bodies (Oct 11/15, outside the window) — narration leaking into requeue reason
strings, not priority moves. No marker/AI/unblock/importance source moved any
priority in or around the window's timespan.
