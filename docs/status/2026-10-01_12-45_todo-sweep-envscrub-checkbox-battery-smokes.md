# TODO-Sweep Window: pool-env scrub, double-checkbox guard, verify-battery, api/redaction smokes

Task: 000001a0f4b46711933c958a92ef00000000 (TODO sweep continuation). Window:
2026-10-01 ~11:55–12:50 CEST. Working mode: single agent session on master
with parallel windows active (a history rewrite landed mid-session and
orphaned 40 cites; healed here — §a1). Five TODO rows closed, one hard
incident healed, and the window's own battery (row 370) was the instrument
that caught the incident.

## a) FULLY DONE

1. **Incident: 2026-10-01 history rewrite orphaned 40 cite-sites.** First
   full run of the new verify-battery flagged dead-sha-default-scope rc=1:
   16 tokens / 40 lines across 7 reports + the README index, each healed
   to its unique reachable patch-id twin (fork records):
   a5a12de0→2a63b1dc, d03571f4→de92677a, 280d9912→40ad3c85,
   dad3cf45→e411e087, a6fad80e→faba1b8a, d3432c1c→0be27fd5,
   ca849564→5d54c129, 4f436285→f1e03759, ca4d1fef→2f56ee19,
   fb5bc27c→dc43c582, 7cc85f6d→1feff4ab, f45a904c→ef198bd5,
   a25227c4→9a28605b, e439d59d→7f78e21f, e759634a→4c79b362,
   ddd675a6→13dcc00b. Patch-id mapper v2
   (single pass over rev-list, 2314 pids hashed once, then join — the v1
   O(n²) shape would have taken an hour): 16/16 real tokens had a UNIQUE
   reachable twin (2 grep artifacts `c`/`e` excluded, 0 LOST, 0 AMBIG) —
   all repointed (35 replacements across 8 files). Gate now rc=0:
   "152 baselined, 0 new". Twin verification: every new sha confirmed
   `git merge-base --is-ancestor` before the sed (df0fd5f4).

2. **Row 333 + 215 — pool-env scrub + hermetic question-channel pin**
   (3121aec1). ci-local.sh now unsets TQ_QUESTION_FILE / TQ_RESULT_FILE /
   TQ_DB / TQ_REDACT / TQ_LOG_DIR / TQ_PAP_API_KEY / TQ_PAP_URL before any
   gate (child-propagation probed rc=0); TestQuestionChannelScopePinsSecondOpinions
   LookupEnv-scrubs both channel vars with t.Cleanup restore. Proof: the
   pin re-run with BOTH vars ambient-exported passes (previously rc=1 in
   every pool session per the row).

3. **Row 206 — double-checkbox class closed** (85374682, building on a
   parallel window's `--- [ ]` work). The 2026-10-01 tick batch shipped
   four rows as `- [ ] [x] …`; checkboxOf ACCEPTS those lines (exact
   `- [ ] ` prefix) so damagedCheckbox never saw them and the tick looked
   human-done while staying machine-open. New `strayCheckbox` rejects the
   file in the parser's ACCEPTED branch (TestStrayCheckboxPrefix, 11
   cases + TestParseRepoAllRejectsDoubleCheckbox; harvest module gate
   rc=0) and check-todo-list.sh grew the DOUBLE-CHECKBOX block (live
   specimen line appended → rc=1 naming file:line, restored file rc=0).

4. **Row 370 — scripts/verify-battery.sh** (24df7c45, healed from two
   daemon folds via the unpushed soft-reset maneuver). Per-leg rc captured
   from the command to files (no PIPESTATUS), `-v` legs enforce a PASS
   count > 0 and flag the bare-ok / `[no tests to run]` zero-green with
   the row-346 `TestStoreConformance/<subtest>` hint, `-m` full module
   gates (cmd/tq via the shim), `-r` date-measured filename check,
   own-file dead-sha leg through check-dead-sha-refs.sh's new
   explicit-files mode, a)-g) skeleton + leg table at the end, --self-test
   rc=0. Its FIRST real run caught the §a1 incident — the tool proved
   itself before it was ticked.

5. **Rows 133 + 134 — api lockout smoke + redaction smoke** (9d0f4b58,
   f28f3419). smoke/api.sh walks a live `tq api` socket on a scratch DB:
   200 baseline → 3×401 → 429 + numeric Retry-After=60 (lockout holds WITH
   the correct token) → expiry → 200. smoke/redaction.sh: stub agent leaks
   fake sk-ant-/AKIA shapes on BOTH outcomes — ok-turn sidecar carries
   [REDACTED] and zero raw tokens, fail-turn task surface (tq show
   lastError/evidence) redacted, `tq audit --journal` zero SECRET
   EVIDENCE; journal-drift.sh phase-4 folds the zero-SECRET-EVIDENCE
   assertion; both wired into ci-local; TQ_SMOKE_KEEP debug valve.

## b) PARTIALLY DONE

- Nothing in this window's scope. (Rows 133/134's sibling — the row-136
  query-token strike path — remains open; not touched.)

## c) NOT STARTED

- The rest of the §f list from the 07-30 close-out (redaction pins
  166–169, lockout nowFunc de-sleep, TQ_TASK_ID export, docs bundle) —
  untouched this window.

## d) TOTALLY FUCKED UP

1. **mvdan `printf --` trap, new entry for the session-hazards list:** in
   this shell, `printf -- '- [ ] text'` treats `--` as the FORMAT
   (writes literal `--`, drops the text). The first gate-specimen run
   "passed" because the specimen line never landed. Rule: never lead a
   printf format with `-`; use `printf '%s\n' 'text'`.
2. **An edit tool misuse deleted a test header** (TestParseRepoAll…'s
   comment+signature replaced instead of preceded) leaving an orphaned
   body — caught immediately by reading the file back; restored before any
   gate ran. Lesson: whole-function inserts go ABOVE the function via
   add_before semantics, not by rewriting its header in an edit pair.
3. **Unescaped `$FAKE_*` in the redaction stub heredoc** baked EMPTY
   strings into the stub (tokens expanded at stub-write… no — escaped for
   runtime, where the env does not exist), so the smoke's greps proved
   nothing for a full run. The TQ_SMOKE_KEEP forensics valve found it in
   one look: `ok turn authenticated with  / `. Rule: secrets-shaped
   fixtures are LITERALS in stub bodies.

## e) IMPROVE

1. check-dead-sha-refs.sh now accepts explicit file arguments (own-file
   leg) — the default scope is unchanged and its --self-test still passes.
2. verify-battery leg names with slashes are sanitized for log filenames
   (`module:internal/harvest` → `05-module_internal_harvest.log`); the
   first battery run died writing `…/05-module:internal/harvest.log`.
3. The mapper recipe lives in /tmp only; §a1 documents the v2 shape
   (single-pass pid table + join). If a third mass orphaning lands, the
   recipe should be promoted into scripts/ (candidate for the row-370
   follow-ups, not done here — no standing row asks for it).

## f) next

1. Redaction pins 166–169 (pattern-table identity, overlap pin, one-marker
   auth-header mask, non-overlap property test).
2. Lockout nowFunc de-sleep (row 134-sibling) + strike-path pins
   (136/404-406/413) — the api smoke gives the live-socket reference.
3. TQ_TASK_ID export (row 193) + closeout-free channel pin (195).
4. Fold-marker/footer-heal/inert-hooks cluster (251/408/314/397/316).
5. Docs bundle (425/196/420/218/385) + prompt de-micromanage (392) +
   batch default (393).
6. Promote the patch-id mapper into scripts/ with a --self-test (see §e3).

## g) questions

1. The §a1 rewrite (40 orphaned cites in one hour) is the THIRD mass
   orphaning this week — all healed post-hoc. Is the rewrite author
   (scripted lineage edit) supposed to run check-dead-sha-refs in its own
   window (row 209 says exactly this)? If it keeps happening, should the
   gate get a pre-push hard wiring in ci.yml too?
2. verify-battery.sh is now the cheapest way to produce a defensible
   battery — should the AGENTS.md verify-window battery line name it as
   the canonical instrument (one line, budget allows)?
3. Row 133's expiry leg costs ~61s in ci-local (authLockout is a const).
   Acceptable as a hard gate, or should the smoke demote to TQ_SMOKE_FULL
   like the dogfood real-mode?

— Battery: verify-battery.sh 7/7 legs green at HEAD (doc-refs, root-build,
root-vet, dead-sha-default-scope 152/0, module:internal/harvest,
target:internal/executor:TestQuestionChannelScopePinsSecondOpinions PASS
rows 1, target:internal/queue/sqlitev4:TestStoreConformance/TestCountTasksMatchesList
PASS rows 1); root `go test ./... -race` rc=0; harvest full suite -race ok;
journal-drift + api + redaction smokes PASS; check-script-syntax 70/70.
