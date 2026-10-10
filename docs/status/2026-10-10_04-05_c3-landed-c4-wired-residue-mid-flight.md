# Status: C3 Landed Green; C4 Wired Through All Four Surfaces; Residue Gates Mid-Flight

**Date:** 2026-10-10
**Window:** 2026-10-10 ~03:10 → 04:05 CEST (resumed the 03-06 session mid-finalization; report finalization, C2 residue, C3 complete, C4 wiring complete)
**Plan:** `docs/planning/2026-10-09_20-23_CONFIG-SYSTEM-ALL-IN-BRIDGE.md` (Full Execution Mode, continue-executing directive)

## a) FULLY DONE

- **Todo list recreated** per handoff (A-track + C1/C2 = done; report-finalization + C2-residue in flight; C3→wrap-up pending).
- **03-06 report finalized (the owed debt):** the daemon had already swept the report file itself (`1fef57fd`) but NOT the index row — added the row to `docs/status/README.md` (top chronological cluster, above the 02-10 row), `check-status-index.sh` **RC=0** rc-to-file (pre-existing trailer + bloat warnings only), named docs commit `1788cb41`. **Push-state correction (this report's own §a initially got this wrong):** at 03:14 `origin/master == HEAD` (68ecdefe) — the daemon HAD pushed everything to that point; by 04:05 the daemon has STOPPED pushing (6 unpushed commits, `origin/master..HEAD`). So pushes are PERIODIC, not per-commit. Practical rules that fall out: never amend anything already confirmed pushed (1fef57fd was — the named follow-up was right for the 03-06 index row), and daemon-folded work on UNPUSHED commits folds via `git reset --soft` into the named commit (this report's own fold). Tags remain unpushed (owner Q1 unchanged).
- **C2 residue cleared:** `go mod vendor` RC=0, root build+vet RC=0 (vendor was stale after sqlitev4's C2 go.mod change), and the **cmd/tq shim gate RC=0** — the `8b2ef1e5` floor-held-at-v0.3.3 revert is now gate-verified for the first time.
- **C3 SHIPPED — the composition root consumes the one deployment description:**
  - `readmodel.ProjectionHomeCallerPragmas` now DELEGATES to `config.SQLite("").ProjectionPragmas()` (model.go:54) — the repo's pragma literals live ONLY in internal/config (one builder per sqlite home); the var stays for the model's self-opened engine path, byte-identical output (NORMAL tier).
  - `composition.New(ctx, config.Deployment)` (compose.go): validates the Deployment, guards the projection home as sqlite-embedded (postgres projection = future metaengine work, clear error), derives the home beside `cfg.DBPath`, and drives the declared engine from `cfg.ProjectionPragmas()` — the resolved sync tier now reaches BOTH projection-home engines.
  - `NewProjectionRuntime(ctx, src, config.Deployment)`: the `modelPath string` parameter is GONE — derived as `readmodel.PathFor(cfg.DBPath)` inside, killing the split-brain where the model path could disagree with the deployment.
  - All 6 callers updated (compose_test ×2, projection_runtime_test, webui_test.go:2380 — now derives modelPath from a temp `config.SQLite`, cmd/tq serve ×2).
  - Module wiring: readmodel + composition go.mods gain `internal/config v0.3.3` require + relative replace (sqlitev4's proven C2 recipe: require only, no go.sum entry — replaced locally).
  - **Gates rc-to-file, all RC=0:** readmodel build/vet/test, composition build/vet/test, root vendor+build+vet, webui targeted (`TestStatsReadFromReadModel`), cmd/tq shim, gofmt clean across all five touched trees. **Batteries AFTER the named commit `8ca613b9`:** webui FULL suite RC=0 (10.2s), root `go test ./... -race` **18/18 RC=0**. Daemon folded model.go footer-less (`221c90fd`) — attribution noted, content is in.
- **C4 wiring COMPLETE (gates mid-flight, see b):**
  - **Store-opening seams:** `internal/queue/sqlite` gains `OpenWithDeployment(d config.Deployment, opts ...)` — the legacy auto-upgrade (`migration.UpgradeIfNeeded`) preserved exactly as in `Open`, then `v4.OpenWithDeployment`; the facade `queue/sqlite` re-exports it; both go.mods gain the config require+replace.
  - **cmd/tq flag surface:** new shared helpers `storeFlag` (the `--store PATH|DSN` switch), `resolveDeployment` (--store wins; falls back to the exact legacy `--db`/`$TQ_DB`/`./tasks.db` chain — a bare path stays a sqlite deployment; TQ_SQLITE_SYNC merges inside FromFlags, the ONE env reader), and `mustOpenStore` (switches on the deployment; **postgres is wired but documented-gated** — refuses with a clear pointer to the owner question instead of half-serving, per the plan's no-pg-instance constraint).
  - **All four long-running surfaces consume it:** worker (resolveDeployment → mustOpenStore with exclusivity opts), serve (deployment shared with BOTH C3 composition sites — the `config.SQLite(dbPath)` double-construction is gone), api (same shape; `UseReadModel` path derives from the deployment), agent-pool (`store` field on `agentPoolOptions` + flag + struct literal; participates in the `--config` file merge with the standard flag > env > file precedence).
  - **Usage text updated:** worker/serve/api/agent-pool lines gained `[--store PATH|DSN]`; the Default-database line documents the postgres stance.
  - **Gates so far, rc-to-file:** cmd/tq shim **RC=0** (13.7s, after the fixes in §d3/§d4), internal/queue/sqlite build/vet/test **RC=0** (the last command run before this report — facade gate + parity script + root vendor + smokes are the residue, listed in f1-7).

## b) PARTIALLY DONE

- **C4 residue (all that remains before the named commit):** queue/sqlite FACADE module gate (wrapper gate is green, facade not yet run); `scripts/check-facade-parity.sh` (the facade gained a re-export — parity gates staged files BEFORE staging per AGR-0016); root `go mod vendor` + build + vet (facade go.mod changed); cmd/tq shim re-run (usage-text edits landed after the last green); `check-go-mods.sh` (two fresh go.mod edits); smokes `legacy-serve-upgrade.sh` + `webui.sh` on a scratch `TQ_DB` (the serve path changed); then the named C4 commit + a root -race re-run.
- **C5, E-track, F-track, wrap-up:** not started (see c) — unchanged from the 03-06 plan order.

## c) NOT STARTED

- **C5:** doctor/audit consume the struct; AGENTS.md store-invariants row; README config section; CHANGELOG.
- **E-track:** E1 TQ_POOL_CONFIG README + parser hardening; E2 agent-pool file-defaults diet; E3 flag-count guard in ci-local.
- **F-track:** F3 ADR-0022 three-lane config; F1 webui tailer retirement eval → ADR-0003 addendum; F2 internal/queue/sqlite legacy thinning; F4 AGENTS.md invariants (+ TestAgentsDocSizeGuard).
- **Wrap-up:** CHANGELOG for A/B/C tracks; plan-file DONE marks with gate citations; 46-file consumer inventory into the plan appendix; B4 serve smoke folded into `scripts/smoke/`; re-ask owner questions.

## d) TOTALLY FUCKED UP (all caught; honesty section)

1. **Edit-tool read-tracking failures (×2 wasted round trips):** I read model.go via bash `sed`/`cat`, then the `edit`/`multiedit` failed twice with "file modified since read" — bash reads do NOT update the tool's read-tracking; only `view` does. Lesson applied: `view` before every tool-edit (or python-heredoc for mechanical changes).
2. **Serve anchor collision (assertion caught, zero damage):** the python rewrite asserted `count == 1` and the serve anchor matched TWICE — cmdStats has the identical `dbPath := resolveDB(*db)` → `store := mustOpenDB(dbPath)` shape. Fixed by extending the anchor through the DISTINGUISHING following line (`cfg := webui.Config{`). The count-assertion discipline is exactly what made this a caught error, not a corrupted edit.
3. **Wrong arity on the auto-upgrade shim:** wrote `if err := migration.UpgradeIfNeeded(...)` — it returns `(*UpgradeResult, error)`. The pattern was 20 lines above in `Open` (`if _, err :=`); I did not check the signature before writing the call site. Shim gate caught it at compile; one-line fix (`87a9b8eb`).
4. **Wrong struct for agent-pool:** `mustOpenDBOpts(resolveDB(poolOpts.db))` sits in cmdAgentPool (main.go), so I added the `store` field+flag to the options struct in bootstrap.go — but `agentPoolOptions` lives in agentpool.go; bootstrap's struct is a different surface. Shim gate caught it (`poolOpts.store undefined`); reverted bootstrap.go cleanly, applied to the real struct.
5. **Plain `go build` in cmd/tq** (missing go.sum for the unpushed config tag) — I knew the committed go.mod is replace-free and the devmod shim is the sanctioned path; ran the plain build anyway as a "quick check" and burned a round trip learning nothing.
6. **LSP diagnostic floods:** every `edit`/`multiedit` triggered the full known false-positive dump (gopls cmd/tq phantoms, golangci stale-vendoring noise) — megabytes of noise per call. Switched to python-heredoc edits for the rest; should have switched after the FIRST flood.
7. **Inherited from the interrupted session (still open, honest):** the 03-06 report's §d items (commit-before-gate twice, garbled QueuePragmas line, wrong tag path, vendor staleness ×2) — this session's discipline (fast gates BEFORE named commits, count-asserted python edits, rc-to-file everywhere) is the direct remedy and held: every failure above was gate-caught, none reached a commit.

## e) WHAT WE SHOULD IMPROVE

- **`view` before `edit`, no exceptions** — the read-tracking contract is tool-level, not content-level; bash `sed`/`cat` reads don't count.
- **Anchor through DISTINGUISHING context:** when a shape repeats across commands (this CLI has many same-shaped flag blocks), extend the old_string through the next unique line rather than trusting count==1 alone.
- **Read the callee's signature before writing the call** — UpgradeIfNeeded's two-value return was visible in the same file I was editing.
- **Find `type X struct` before editing `opts.field`** — the usage site of a struct is not its definition site (agentpool vs bootstrap).
- **python-heredoc for mechanical multi-line edits by default** in this repo: count-asserted, no LSP flood, proven twice over now.

## f) NEXT (prioritized)

1. C4 residue: queue/sqlite facade module gate (build/vet/test rc-to-file).
2. C4 residue: `scripts/check-facade-parity.sh` BEFORE staging the facade re-export.
3. C4 residue: root `go mod vendor` + build + vet.
4. C4 residue: cmd/tq shim gate re-run (usage edits post-date the last green).
5. C4 residue: `scripts/check-go-mods.sh` (internal/queue/sqlite + queue/sqlite go.mods).
6. C4 residue: smokes `legacy-serve-upgrade.sh` + `webui.sh` on a scratch `TQ_DB` (serve open path changed).
7. C4 named commit + root `-race` re-run as the battery.
8. C5: doctor/audit consume `config.Deployment` (doctor's db check reports the resolved deployment, audit opens through it).
9. C5: AGENTS.md store-invariants row — "one deployment description; config owns the pragma literals" (watch TestAgentsDocSizeGuard).
10. C5: README config section (`--store`, TQ_SQLITE_SYNC, postgres stance).
11. C5: CHANGELOG entry for the config wave so far.
12. E1: TQ_POOL_CONFIG documented in README + parser hardening (unknown-key rejection).
13. E2: agent-pool file-defaults diet (defaults live in flags, file overrides only).
14. E3: flag-count guard in ci-local (pin the flag surface, gate growth).
15. F3: ADR-0022 — the three-lane config model (flags / env / file) written down.
16. F1: webui hand-journal-tailer retirement eval → ADR-0003 addendum (`--read-model=false` fallback still exercised by tests?).
17. F2: internal/queue/sqlite legacy thinning (what remains beyond Open/OpenWithDeployment/UpgradeIfNeeded).
18. F4: AGENTS.md invariants refresh + size-guard run.
19. Wrap-up: CHANGELOG entries for A/B/C tracks (one coherent section).
20. Wrap-up: plan-file DONE marks with gate citations (A1-A7, B, C1-C4).
21. Wrap-up: persist the consumer inventory into the plan appendix.
22. Wrap-up: fold the B4 serve smoke into `scripts/smoke/`.
23. Wrap-up: closing status report (owner hasn't asked yet — only if asked, per handoff).
24. Re-ask the three owner questions in every remaining report (see g).
25. Evaluate `scripts/heal-daemon-sweep.sh` for `221c90fd`/`0a14b689`/`bf69c52b`/`87a9b8eb` — likely SKIP: the daemon pushes, so footer-less commits are already remote; the heal script is unpushed-only by design.
26. cmd/tq committed go.sum: confirm the shim gate keeps the committed go.sum untouched (dev.sum is derived) and document that `go install cmd/tq@sha` stays broken until the config tag push (Q1) — check whether any gate exercises proxy-mode cmd/tq.

## g) QUESTIONS FOR THE OWNER (cannot self-answer)

1. **Push authorization (Q1, now broader):** the wave tags (`internal/journal/v0.4.0`, `internal/queue/companion/v0.4.0`, `internal/config/v0.3.3`) are still local-only. C4 added `internal/config v0.3.3` to cmd/tq's committed go.mod, so a proxy-resolved `go install …/cmd/tq@<sha>` fails until the tag pushes. May I push the three tags (and bump cmd/tq's floors to the pushed versions in the same motion), or does that wait for a release window?
2. **C4 Postgres stance (Q2):** shipped as documented-gated — `mustOpenStore` accepts the `postgres://` shape through FromFlags/validation, then refuses at open with a pointer to this question. Keep the refusal until a live instance exists (I can wire the driver path blind, but it would be an unverified claim in a gate-sense), or provide a scratch DSN so the postgres path gets a real test?
3. **`tq facts --json` embedded-JSON wire (Q3, unchanged):** the object-tripwire pins Detail as embedded JSON permanently — ratify, or do you want the base64 form revisited after all?
