# Release-hardening done-prompt window — seventeenth pass

**Date:** 2026-10-09 03:55 CEST
**Window:** five completed tasks, ~02:00–03:52 (commits `217fcf0f`-lineage review fix, `058a4b37`+`eb49d2e2`, `b1999bb9`/`2ea3267b`+`4f81113b`, `67dfcc7e`+`52735052`, `bf424139`+`ec297ffd`)
**Vantage:** first-hand verification at HEAD (gate runs, grep anchors); closeout reports read first, not re-derived.

## a) FULLY DONE (verified at HEAD, not just claimed)

1. **Proxy-wait loop extracted + offline smoke** (task …e271e8b, commit `058a4b37`): the release.sh verification loop now lives in `scripts/lib/proxy-wait.sh` as `wait_for_proxy_version` (call site `scripts/release.sh:171`), with overridable probe seams `tq_proxy_poke`/`tq_proxy_lists`. `scripts/smoke/proxy-wait.sh` drives both die branches plus flicker and recovery offline, zero network; wired into `scripts/ci-local.sh:212-213`. Smoke PASS at HEAD this pass.
2. **HTTP-code classification of the poke** (task …e34da26, commits `b1999bb9`→healed, `2ea3267b`): `tq_proxy_poke` captures `%{http_code}` into `PROXY_POKE_HTTP_CODE` (0 = transport failure, `scripts/lib/proxy-wait.sh:9-10`); 2xx/404 count as proxy-reachable (lag), DNS/TLS/5xx as network-dead — the 5/5 die is now HTTP-semantic.
3. **Drift-smoke lib-scope fix** (task …429a636, commit `67dfcc7e`): `need_in_both` in `scripts/check-release-docs.sh` greps `scripts/release.sh` AND `scripts/lib/*.sh` — fixes the false-fail the extraction minted (the `go list -m -versions` needle moved to lib). Gate rc=0 at HEAD this pass.
4. **LAST-attempt classification fix** (task …4bbda2e, review finding on `b1999bb9`, commit `bf424139`): the HTTP code is now captured AFTER the poke (`proxy-wait.sh:29`), `poke_ok` resets on failed pokes (`:37`), and the smoke gained varying-code cases (404→dead ⇒ "network is dead"; dead→404 ⇒ "proxy lag"). Both review findings (stale code read + sticky flag) fixed exactly, no scope creep.
5. **Prior review-fix re-dispatch closed** (task …d8b7eb, 02:00 closeout): fourth DONE-on-arrival no-op — the v0.3.3 index-row fix and closeouts verified present at HEAD; one new TODO append (mint-side footer probe, now rowed at TODO_LIST `:590`).

Also adjacent (next window, in flight at 03:46–03:49): the per-target miss-message split in `need_in_both` already landed (`612148d9`, task …e54e6aafb; TODO row ticked) — verified: per-target miss strings present at `check-release-docs.sh:29-31`.

## b) PARTIALLY DONE

- Nothing in the five window items; each landed complete with its closeout.
- Standing: the drift smoke's structural rot family is only partially addressed — lib scope fixed (row ticked), per-target messages landed, but reflow-tolerant doc pins and the `--self-test` pin remain open (TODO rows 50, 51). Second scope-drift false-fail of this smoke in two days; the smoke is still exercised by discovery, not by the move that breaks it.
- The window's harness lessons (override-after-source ordering, `unshare -n` structural offline, shared die-message constants) are rowed but unshipped (rows 46–48).

## c) NOT STARTED

- The broader release-path backlog the window intentionally did not touch: `--publish-steps-only` resume mode (row 49), `--max-time` on the poke curl, sibling-script passive-poll audit, `tq pool-health`, and — the structural one — the cmd/tq release + SystemNix pool redeploy that ends the DONE-on-arrival re-dispatch burn class.
- Archive sweep of the 09-15 reports (row 119, weekly cadence): all three live 2026-09-15 reports still carry zero resolution strikethroughs; left to the rowed sweep rather than a shallow pass here.

## d) TOTALLY FUCKED UP

- **No regressions, no broken gates at HEAD this pass**: drift gate rc=0, proxy-wait smoke PASS, `bash -n` green, tree clean.
- **Two review rejections in the band's recent history** (`b1999bb9` rejected for the stale-code/sticky-flag bug) — the reviewer caught what the original smoke missed; the fix closeout honestly records re-introducing the bug class twice before the smoke forced the right shape. Cost: extra paid iterations, zero shipped damage.
- **Daemon-sweep races hit two window tasks** (folded mid-flight, healed via `scripts/heal-daemon-sweep.sh` with footer-last rewrites; fork records in the closeouts). The daemon heal path worked, but the class enforcement guard still does not exist (rowed since row 175).
- **Footer-ordering violation** in one window commit (attribution after the footer) — caught by the heal script's own rail, rewritten. Process wart, not damage.
- **First smoke iteration of the extraction task hit the live proxy** three times from an "offline" test (override-before-source bug). The false-pass/false-offline class this task existed to kill, briefly committed by the fix itself.

## e) WHAT WE SHOULD IMPROVE

1. **Extraction commits must run the gates that grep the moved code** — the drift smoke went red for a full day after `058a4b37` before a dispatch noticed. The canonical "which gates for a scripts/↔scripts/lib/ move" list is still undefined (03-11 §g2, owner input).
2. **Test harnesses need false-pass paranoia**: assert die-message substrings, never bare rc; make offline claims structural (`unshare -n`), not aspirational — both rowed, both generalizable beyond this smoke.
3. **Seam contracts should be explicit**: `PROXY_POKE_HTTP_CODE` is an implicit contract (seams MUST set it per call); a reset helper or pin comment plus a smoke assertion would harden it (new row this pass).
4. **Review-fix iterations**: the correct patch shape (capture rc → read code → branch) should come from reading the finding's stated direction, not from smoke-failure archaeology; one extra read pass would have saved two red iterations.
5. **Edit→commit→battery ordering** keeps being applied late in bash-only tasks; the daemon is live and folds within 60 s.

## f) UP TO 50 NEXT THINGS

Dedup-checked against the 352 unchecked rows; most candidates are already rowed (46–52, 590, and the release cluster). Seven new items cleared dedup and are appended to TODO_LIST this pass (see the new section at the file tail): the `scripts/lib` glob existence guard, the seam-contract hardening, the self-test code-side mutation case, and three owner questions plus the AGENTS.md seam-rule promotion. Beyond those, the highest-value existing work is unchanged: rows 50–52 (drift-smoke structural fixes), row 49 (`--publish-steps-only`), the cmd/tq release + pool redeploy, and `tq pool-health`.

## g) QUESTIONS (only the owner can answer)

1. Is 404-as-`poke_ok` wanted in the DIE STRING prose too ("every .info poke succeeded" now covers all-404 runs), or should it reword to "every poke was answered (2xx/404)"? (03-11 §g1)
2. Should the lib pin a minimum curl version / startup feature-probe for the load-bearing `-w '%{http_code}'`-on-`-f`-failure behavior, or is host-curl trust acceptable for a release-time script? (03-11 §g3)
3. Should "network-I/O lib functions must expose overridable probe seams" be promoted into AGENTS.md Conventions as a standing rule? (03-01 §g1)

## h) BAND DRIFT

**None recorded.** The reachable journal (`./tasks.db`, via `tq facts`) holds ZERO `task.reprioritized` facts. The production pool journal was unreachable this session (`$TQ_DB` unset) — same vantage gap as passes 6–16; pool-side moves remain unverifiable from here.

---

_Point-in-time snapshot 2026-10-09 03:55. Window of five completed tasks, all verified at HEAD. 7 TODO appends (3 blocked questions). Nothing pushed._
