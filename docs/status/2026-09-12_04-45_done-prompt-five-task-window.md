# Done prompt — 2026-09-12: five-task window close-out (window 09-10 09:00 → 09-12 03:42)

- **Written**: 2026-09-12 04:45 CEST
- **Window**: queue tasks `000001a08a1a8b…`, `…84b16`, `000001a08a2846…`,
  `000001a08a3f2a…`, `000001a092fc97…`, `000001a0934141…` — three 09-10
  chores (smoke/CI-measure/dogfood-hygiene) plus the 09-12 pair
  (session-close bridge, ci-local runner parity).
- **Method**: every claim below re-verified at HEAD `ad1ae42` against
  commits, gates, and live CLI/gate runs this window; stale-DONE rows were
  re-verified before closing (citations inline).

## a) FULLY DONE (verified, not just claimed)

1. **Multi-repo smoke honors `TQ_BIN`** (task `…84b16`, commit `7e32b40`):
   `scripts/smoke/multi-repo.sh` now uses the prebuilt-or-build pattern like
   the other smokes; verified green against `result/bin/tq` 0.2.0 (6
   enqueued / 6 completed / 0 dead-lettered) and on the default build path.
   TODO row closed with the verdict.
2. **CI module-loop timing measured; no tuning warranted** (task
   `…f70189`, commit `674320f`): step timings from green run `34406684629`
   show the disk-derived `-count=1` loops at 26s/271s (~10%) and 39s/173s
   (~23%) — well short of dominating. The item's own condition ("tune if it
   dominates") resolved as "don't". TODO row closed.
3. **Dogfood hygiene audit: zero stale-verify tasks** (task `…731362`,
   commit `d53362d`): the live production journal held only one pending
   review task (self-contained payload, no verify command) and the audit
   task itself; nothing to drain or cancel. TODO row closed.
4. **Session-close bridge design + prototype** (task `…9a59a26`): the
   deliverable landed via daemon commits `1e85603` (`internal/session` +
   `cmd/tq/session.go`, 618 LOC) and `277bd4f` (design doc + tests +
   DOMAIN_LANGUAGE/CHANGELOG/FEATURES); attribution commit `155ab5a`
   carries the footer and the gate record (root build+vet+race, all 7
   sub-module gates). Verified this window: `internal/session/` exists with
   tests, `tq session begin/close` is wired into the command table
   (`cmd/tq/main.go:72-74,108`), CHANGELOG/FEATURES rows present.
5. **ci-local release-gates smoke runs identity-blind** (task
   `…1414abc5`, commit `eb78d78`): `scripts/ci-local.sh:155-157` runs the
   smoke under `GIT_CONFIG_GLOBAL=/dev/null` + `user.useConfigOnly` via
   env config. The TODO's literal `/dev/null`-only mechanism was PROVEN
   inert on this host (git auto-detects identity from passwd GECOS);
   both-ways proof: a flag-stripped smoke copy dies "Author identity
   unknown", the real smoke stays green. Verified in the file this window.

## b) PARTIALLY DONE

1. **Session-close bridge is a prototype** (FEATURES correctly says
   `PARTIALLY_FUNCTIONAL`): trigger automation is not wired (crush #3146
   SessionEnd or a PreToolUse registry), daemon-folded commits stay
   unattributed, direct enqueues bypass the daily budget, and `AppendFact`
   is sqlite-only. The design doc's open-questions section
   (`docs/planning/2026-09-12_session-close-bridge-design.md`) is the
   authoritative gap list; harvested below.
2. **`task.AllStatuses` export rides the next re-tag**: the webui/httpapi
   twin lists are retired at HEAD, but the root go.mod still requires
   `internal/task` v0.2.0 — proxy consumers get the export only when the
   sub-module is re-tagged and the require bumped (04-13/04-31 close-outs;
   unharvested residue of a task outside this window, carried below).
3. **The window's runner-proof is stale**: the last CI-green run
   (`34661978016`, 00:32:31Z) predates `internal/session` (00:34Z),
   `eb78d78`, the review-prompt fix, and the AllStatuses retirement —
   26 local commits are unpushed and CI-unproven. Nothing is wrong at
   HEAD locally (gates below), but the first push after the baseline
   triage is the proof point.

## c) NOT STARTED (biggest skips; full list stays in TODO_LIST.md)

1. **SystemNix pool revival** (L45): input flip + `nix run .#deploy` +
   `Environment=GOEXPERIMENT=jsonv2` (O1) — owner-run, still the top
   systemic lever; every minted verify burns attempts until it lands.
2. **Ruling-gated round-13 items** (T9–T13, T18–T27): TQ_RESULT schema,
   report placement, per-project webui product halves, gosec gate flip,
   policies/credentials — packaged and waiting on the owner's O1–O6
   answers (`docs/planning/2026-09-12_02-48_OWNER-RULINGS-PACKAGE-O1-O6.md`).
3. **Session-bridge hardening**: no smoke script, no `session list`,
   no `--dry-run`, no Postgres story (all harvested to TODO_LIST below).
4. **T16 fixture-release follow-ups** (ci.yml sanitizer symmetry, negative
   fixture self-test) and the CI-topology one-pager — untouched.
5. **Docs-health continuation** (L134): the six 2026-09-07 reports remain
   unannotated/unarchived; this pass again did not archive anything
   (no report qualified — every candidate still carries live items).

## d) TOTALLY FUCKED UP (verified this window)

1. **`lint-baseline --check` is RED at HEAD** (run live this window):
   7 growth rows — executor mnd 20→24 and noctx 1→5, queue/sqlite
   varnamelen 32→33, worker mnd 7→8, root lll 37→38, noctx 22→29,
   varnamelen 9→10. This is a HARD ci-local gate (round-13 T5), so the
   next full gate/push fails until the rows are triaged (the noctx ones
   look like real missing-context findings; mnd/varnamelen/lll look like
   policy-regen material). First reported by the 04-13 close-out,
   re-verified here — still unresolved.
2. **Round-13's verify-then-close shipped a wrong verdict on L147/L150**:
   the 03-21 report claims both rows were "verified genuinely open", but
   round-12's T14 (01-09 report) executed `release.sh` on a real fixture —
   gates green (nix stubbed, honestly recorded), `--tag` + sub-tag cutting
   incl. a nested module, clean-room resolution via `GOPROXY=direct` +
   `go get`/`go list -m all`, negative gates, AND a real prod-script bug
   found+fixed (`scripts/release.sh:93` env-prefix substitution). The
   03-46 close-out flagged exactly this (its §f10-11). Both rows are now
   closed with the round-12 citations; the lesson: a verify-then-close
   pass must cite WHICH prior evidence it consumed, or it re-files
   refuted findings (the 04-18 refutation-repeat class).
3. **AGENTS.md was the trap vector twice over** (both fixed this pass):
   (i) the per-module gate snippet omitted the `GOEXPERIMENT=jsonv2`
   export and failed agents verbatim (04-13 + 04-31 §d1, the third
   rediscovery); (ii) its baseline numbers said 835 findings/110 rows vs
   the actual 887/114 regen.
4. **Duplicate-claim re-delivery struck again**: task
   `000001a0934efd9de…` got two close-outs 18 minutes apart (04-13, 04-31)
   — the queue re-delivered an already-closed item and the second window
   re-burned a full close-out before noticing. Policy ruling pending
   (§g3); until then every stale re-dispatch costs a window.
5. **Daemon-fold attribution gap keeps taxing every window**: the session
   bridge's own deliverable needed a manual attribution commit (`155ab5a`)
   because the daemon folded the work footer-less (`147bd17` ate an
   intermediate of `eb78d78` the same night). Fourth ask across
   03-28/03-46/04-13/04-31 — §g2.
6. **Stale-DONE rows accumulate faster than passes close them**: this
   pass verified and closed 8 (L88 same-session-priority — implemented at
   `cmd/tq/main.go:455` + `TestRunSameSessionPriority`; L147, L150;
   L186 per-module baseline — `.golangci-baseline.txt` 114 rows; L211
   setup-go pin — master green `34661441493`/`34661978016`; L241
   lint-baseline wiring — `ci-local.sh:117`; L242 env-self-contained
   minted verifies — `.tq-verify` + round-13 T2; L243 doctor ENV-LIE
   detector — `cmd/tq/doctor.go:432-520`). Each stale row is a potential
   re-dispatch burn.

## e) WHAT WE SHOULD IMPROVE

1. **Verify-then-close passes must cite consumed evidence**: "verified
   genuinely open" without naming the contradicted source (round-12 T14)
   is how a wrong verdict ships. A one-line "sources consumed" list per
   verdict would have caught it.
2. **Triage the baseline the day it goes red**: the gate is only useful
   if red means "fix now"; a red gate that sits for two windows trains
   everyone to bypass ci-local.
3. **Fix real linter classes, policy-regen style classes** — and record
   which in the regen note, so the next reader can tell.
4. **Close stale-DONE rows during every done-prompt** (this pass's 8
   closures took minutes with `tq show`/grep); cheaper than the
   three-dispatch re-fire pattern that dominated 09-11.
5. **Land the session-bridge smoke before trigger automation**: the
   bridge's enqueue-only contract is exactly what a scratch-DB smoke can
   pin cheaply.
6. **Keep AGENTS.md copy-paste-safe**: any command snippet in the doc
   must succeed verbatim outside the devShell — the jsonv2 export
   belongs in every gate line, not just prose.

## f) NEXT (harvested to TODO_LIST.md — top items)

Top of the list, in impact order (full set appended to TODO_LIST.md):
baseline triage (unblocks the push), session smoke + trigger registry,
postgres fail-fast, ci.yml sanitizer symmetry, AllStatuses re-tag at the
next release, and the duplicate-claim policy ruling. See the new
"Done-prompt window harvest (2026-09-12)" section in TODO_LIST.md —
36 items minted, 8 stale rows closed, 3 owner questions filed BLOCKED.

## g) QUESTIONS ONLY THE OWNER CAN ANSWER

1. **Budget class for session-minted tasks**: must `tq session close`'s
   direct enqueues (one review + one status) pass the daily-cap check and
   park when exhausted, or are they operator-class actions that bypass
   the budget by design? Today they mint even when the pool is paused.
2. **Daemon attribution**: is teaching the auto-commit daemon to carry
   `Crush-Session: <id>` footers (env-carried session id) the intended
   fix for the daemon-fold gap, or should `tq session close` grow a
   timestamp/author-range fallback? Is the daemon yours to change, and
   do you want tq's help? (Fourth ask across four consecutive windows.)
3. **Duplicate-claim policy**: when the queue re-delivers an
   already-closed task (the 04-13/04-31 pair), what should the second
   window do and what may its TQ_RESULT claim — verification-only
   short-circuit as the sanctioned shape, or a queue-side fix (don't
   re-dispatch tasks whose terminal fact exists)?
