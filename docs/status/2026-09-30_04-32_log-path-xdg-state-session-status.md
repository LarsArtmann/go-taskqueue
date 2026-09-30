# 2026-09-30 04-32 — Log-Path Question Session Status (Interactive, Zero Code)

- **Session type**: INTERACTIVE owner Q&A (no task ID), two turns.
- **Window**: 2026-09-30 ~04:20–04:40 CEST. HEAD at write: `9d4eb43c`, working tree clean at session start.
- **Scope**: one owner question answered ("why are logs in /var/state/tq/logs and not /var/logs/go-taskqueue") + this report. **Zero code delta — by design** (question session, no dispatch).
- **Format override**: the status-report skill's canonical format is styled HTML; the owner explicitly requested `.md`, so this report is Markdown per the user-wins rule.
- **Env noise noted and correctly ignored**: gopls reported 43 `undefined:`/`unknown field` errors on cmd/tq/main.go all session — the AGENTS.md-documented cmd/tq LSP false-positive class (module-cache resolution against the pinned tag; the devmod shim gate owns the truth). Never "fixed", per the standing warning.

## a) FULLY DONE

1. **Log-location question answered with verified citations** (owner turn 1):
   - Premise corrected twice: `/var/state/tq/logs` appears NOWHERE in this repo (grep zero hits) and `/var/logs` does not exist on FHS systems (it is `/var/log`). The real location is `$HOME/.local/state/tq/logs` — `defaultLogDir()` at cmd/tq/main.go:658-668.
   - Rationale delivered with sources: the code's own comment ("logs are state, not config — they may be deleted without breaking anything", cmd/tq/main.go:658-660); XDG_STATE_HOME semantics; no-root requirement (`/var/log` is root-owned; the pool deliberately runs as a login user, deploy/nixos/tq-agent-pool.nix:73-83); self-contained retention instead of logrotate (`--log-dir-max-age` default 168h + `--log-dir-max-bytes` default 5GiB via bootstrap, cmd/tq/bootstrap.go:264-273, swept each tick per internal/executor/sidecar.go); consistent `tq` naming scope.
   - Deployment truth verified: the repo NixOS module only puts the DB under `/var` (`StateDirectory = "tq"` → `/var/lib/tq`, deploy/nixos/tq-agent-pool.nix:231-233 and :268-270); the SystemNix house module pins `log-dir = /home/lars/.local/state/tq/logs` + `log-dir-max-age = 168h` (SystemNix `modules/nixos/services/tq-agent-pool.nix:100-101`, probed via rg this session).
   - Docs cross-check: README "Output sidecars" bullet + the `~/.local/state/tq/logs/<task-id>.log` default (README.md:202-207, :242, verified at HEAD `9d4eb43c`); CHANGELOG "default at ~/.local/state/tq/logs — daemon pools need their logs" (CHANGELOG.md:1554); FEATURES retention row (FEATURES.md:66).
2. **This report written, dated by `date` (04:32), indexed, and gated** (index row appended to docs/status/README.md; battery rc-captured below).
3. **Two real findings surfaced while answering** (documented, not fixed — see §c/§f): (i) `defaultLogDir()` ignores `$XDG_STATE_HOME`; (ii) the synthetic-`tq`-user NixOS default would scatter logs into `/var/lib/tq/.local/state/tq/logs` (user home = dirOf dbPath, deploy/nixos/tq-agent-pool.nix:172-177).

## b) PARTIALLY DONE

- Nothing from this session's ask is partial — the question was answered end-to-end.
- **Master-CI state: not probed at turn 1** (session-start.sh skipped, see §d). The report-time probe substitutes; the last indexed knowledge is the 01-24 window's local `nix build`/`flake check` ALL PASSED at `06c3da66` after the vendorHash heal, with master-CI push state unknown. Probe result recorded in §"Battery".

## c) NOT STARTED

1. `$XDG_STATE_HOME` honoring in `defaultLogDir()` (cmd/tq/main.go:661-668 reads only `os.UserHomeDir()` and hardcodes `.local/state` — the XDG-specified env var is ignored). Behavior change → owner ruling first (§g2).
2. Surfacing the ACTIVE log-dir (path + retention knobs + on-disk size) in any operator surface (`tq doctor`, `tq stats`, serve nowband) — the exact confusion that triggered this session is only answerable by reading source today.
3. README "Where do logs live" section (sidecar files vs journalctl vs daemon-folded commits vs evidence archives) — §g3.
4. `docs-health` HARVEST of this report's §f into TODO_LIST/ROADMAP — deliberately not started; the owner asked for the report THEN WAIT.

## d) TOTALLY FUCKED UP

**Nothing.** Zero code delta, zero damage, no stray files, no daemon-food created. The honest misses are process-level, not damage:

1. **Turn-1 ritual skip, both halves** (a) `scripts/session-start.sh` not run → no master-CI probe at START; (b) `CONTRIBUTING.md` turn-1 read skipped — the documented recurring miss class, one more instance. Mitigated here: read-only Q&A session, no edits, and the CI probe ran at report time (the 04-23 pattern).
2. **Premise corrected from config evidence, not from observed state**: I never `ls`'d `~/.local/state/tq/logs` to show actual sidecar files — the answer rests on source + deployed config (which is the stronger chain anyway), but the live proof was one cheap command away and would have pre-empted any "but where IS it on disk" follow-up.

## e) WHAT WE SHOULD IMPROVE

1. **Make the log answer self-serve.** The owner had to ask where logs live; no `tq` surface prints the active `--log-dir`. One `tq doctor` line (path + retention + bytes used) kills this question class.
2. **Spec-compliance nit worth a ruling**: `defaultLogDir()` should either honor `$XDG_STATE_HOME` or document why it pins `$HOME/.local/state` (predictability for forensics — every status report greps that literal path). Note the tension: honoring the env var moves existing deployments' logs silently.
3. **Synthetic-user surprise**: with the module defaults (`user = "tq"`, `dbPath = /var/lib/tq`), the user's home IS the state dir (deploy/nixos/tq-agent-pool.nix:172-177), so default sidecars land at `/var/lib/tq/.local/state/tq/logs` — legal but surprising, and nobody has verified it live. The module's poolSettings example could pin an explicit `log-dir`.
4. **Ritual discipline in question sessions**: turn-1 skips keep recurring precisely on "it's just a question" turns. The 23-57 session-start.sh row made the probe mechanical for task windows; question windows have no equivalent trigger.
5. **Answer-then-improve loop**: this session produced two findings and four §f candidates from a single question — the cheap wins (doctor surface, README section) should ride the next code window rather than waiting for a dedicated dispatch.

## f) 50 things we should get done next

**Brainstorm per the skill's larger-N rule — routing rigor (docs-health HARVEST) applies; most B-items are ROADMAP fuel or existing rows, not commitments.**

Fresh from this session (A):

1. Surface active `log-dir` + retention + on-disk bytes in `tq doctor` (and `tq stats`), owner-ruling pending (§g3).
2. README "Where do logs live" section: sidecars (`~/.local/state/tq/logs`, 0600) vs journalctl vs daemon commits vs `docs/status/assets` archives.
3. Owner ruling: honor `$XDG_STATE_HOME` in `defaultLogDir()` vs keep the hardcoded path (§g2; if honored, add a `tq doctor` echo so the move is visible).
4. Pin `log-dir` explicitly in the NixOS module's poolSettings example (kills the synthetic-`tq`-user `/var/lib/tq/.local/state/tq/logs` surprise).
5. Verify-and-pin: a unit test asserting `defaultLogDir()` shape (and the env decision from #3), so forensics greps never rot.
6. Check SECURITY.md says sidecar logs are 0600 + redaction defaults ON (`--redact=false` escape hatch) — log security posture currently lives in AGENTS/README only.
7. Verify FEATURES.md:66's "defaults off" (raw executor flags) vs bootstrap's 168h/5GiB defaults are both labeled so no reader concludes sidecars are unbounded by default.
8. Audit sibling `UserHomeDir()`-joining defaults (`defaultProjectsDir`, session registry dir) for the same XDG-blindspot class as #3.
9. Bootstrap pool.conf comment could carry the one-line rationale ("logs are state, not config") so rendered configs are self-explaining.
10. `tq show`/detail page: render the sidecar log path with the existing CopyButton if not already wired (verify-then-adopt).

Carries observed in the index while writing this report (B, each with source row):

11. Heal the 23-40 amend-contamination (foreign files derive as the 149-task's) — TIME-SENSITIVE pre-push (23-40 row).
12. Installer hooksPath delivery fix — ALL git hooks inert on this host (`core.hooksPath=.githooks` → absent dir), second landed violation (22-05 row).
13. `tq doctor`/`tq audit` derivation-blind census (footer-less daemon-commit attribution) (22-05 row).
14. Historical census row (22-05 row).
15. Row 199: third paid lap on a DONE row → re-dispatch mint policy ruling (01-03 row).
16. Row 345: master-CI red cause + push — zero CI coverage of the suppression/auto-dismiss code was still true at the 01-03 window (00-50/01-03 rows).
17. CHANGELOG/FEATURES entries for the httpauth extraction (filed 22-53 row).
18. `writeRateLimiter` keying verify-close row (filed 22-53 row).
19. Lockout literal split-brain row (filed 22-53 row).
20. Cookie raw-vs-hash ruling (22-37 §g).
21. Lint-baseline drift ownership — 5 concurrent-drift classes sitting RED on `--check` since 22-37, never regened (22-37 §g3).
22. Security-matrix parity pin row (filed 22-37 row).
23. `archive-evidence.sh --wait` default + PENDING exit-code rulings (23-40 §g).
24. Mixed-daemon-commit attribution rule (23-40 §g).
25. Utility regression-pin bar ruling (session-start.sh pin is contingent on it) (23-57 row).
26. Footered-commit atomicity receipts row (#2–#5 and counting) (23-57 row).
27. Crush upgrade to ≥0.97.0 ownership — channels prereq (23-37 §g).
28. Serve-per-repo green-light + pilot repo for client/server mode (23-37 §g).
29. `tq mcp` sequencing + the channel-MCP adoption verdict on TODO_LIST.md:458 (23-37 row).
30. Agent-autonomy overhaul rows: prompt de-micromanage, `--batch-items` default flip + budget math (minted 23-37).
31. 103-task auto-dismiss rewind runbook: live-vs-stopped + backup ruling (22-05 §g).
32. Post-close-out requeue policy ruling (verify-gate-TIMEOUT requeue AFTER close-out landed — receipt sub-class) (22-53 §g).
33. DONE-side re-dispatch dedup promotion (multiple confirming instances now) (00-50/22-53 rows).
34. Owner gate fix: scope `.tq-verify` gofmt stage to tracked files (agent never edits own gate) — still unscoped at the 01-30 sweep (01-30/01-32 rows).
35. Execute/verify the 23-DISMISS verdict sweep through the new auto-dismiss path + rewind replay of the 103-task list (01-30/22-05 rows).
36. Postgres drift-parity + seeded-drift CI coverage rows (20-47/20-50 rows).
37. Conform↔journalaudit cross-link row (20-50 row).
38. Audit blind-spot wording row (20-50 row).
39. AGENTS.md stale S1 enqueue-snapshot paragraph heal (20-47 §b).
40. Silent-cap class sweep audit/doctor/dlq (`tq tasks` truncation footer was the first of the class) (03-45 row).
41. `tq tasks` footer pin test (top follow-up of 03-45, unowned).
42. E2e truncation pin (03-45 row).
43. `--count` JSON envelope semantics ruling (bare-array consumers) (03-45 §g).
44. Footer-attribution policy under the daemon (03-45 §g).
45. Statix gate ownership ruling (manual-only today) (01-24 §g).
46. `config.go-standard` read style ruling (01-24 §g).
47. Footer-less daemon-sweep heal ruling (receipts #6/#7 by 01-24) (01-24 §g).
48. Check-ci cancelled-run semantics row (01-03 §f).
49. Run-canceller identification for the cancelled master run (01-03 §g).
50. Index-bloat sweep: the live index carries >100 unarchived rows by the README's own cadence rule — run the docs-health archive sweep (docs/status/README.md header).

## g) Questions I cannot figure out myself

1. **Where did the `/var/state/tq/logs` premise come from?** A doc, another host's unit file, a misread of `~/.local/state/tq/logs`? If any artifact in your fleet actually says `/var/state/tq/logs`, it is a lie I need to find and fix; I grepped this repo (zero hits) but cannot grep what you were looking at.
2. **Should `defaultLogDir()` honor `$XDG_STATE_HOME`?** Today it hardcodes `$HOME/.local/state` (cmd/tq/main.go:661-668). Honoring it is spec-correct but silently moves logs on hosts where the var is set — and every forensic recipe in docs/status greps the current literal. Your call: spec-correct + visible-move, or documented-pin.
3. **Do you want the log location made self-serve** — `tq doctor`/`tq stats` printing the active log-dir + retention + on-disk size (item #1), plus the README "Where do logs live" section (#2) — or is the current README line sufficient?

## Battery (rc-captured this window, stock shell, redirect-to-file per the PIPESTATUS hazard)

See the closing summary for the rc table; all gates were run AFTER the report + index row existed. No code gates needed (zero code delta): no build/vet/test beyond what the question session touched (nothing).

---
_Interactive Q&A session report. Wait state: owner instruction pending (per the session contract). Format: .md per explicit owner request (skill default is HTML)._
