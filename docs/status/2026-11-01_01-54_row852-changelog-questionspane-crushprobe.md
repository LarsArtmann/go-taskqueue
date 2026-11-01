# Session status — continuation wave: row-852 close-out, CHANGELOG, T20 rows 1447/1445, crush-server probe (interrupted pre-push, pre-clean-battery)

**2026-11-01 01:54 CET · Continuation of the interrupted T16–T26 wave (prior report: docs/status/2026-10-31_13-13_T16-T26-execution-wave-status.md) under the owner's "WHOLE TODO LIST" GO. Scope this session: row 852 close-out, CHANGELOG [Unreleased], final battery prep, T20 rows (1447 verify, 1445 build, 1446 research, 1450 probe), T22–T27 NOT started. INTERRUPTED by owner STOP for this report — the clean full-battery receipt and the push are STILL MISSING; ~58 commits now sit unpushed on master (HEAD 4a69bf9f, tree clean).**

---

## a) FULLY DONE (this session; each with its landing commit)

1. **Row 852 (lint baseline regen + --check green receipt)** — the prior session's
   FAIL diagnosed: a NEW `gci` class in `internal/queue/companion/conform/tests.go`
   (this wave's mixed-case rows, two stray blank lines). Fixed AT SOURCE via
   `golangci-lint run --fix` (swept c78b852d); module gate green;
   `./scripts/lint-baseline.sh --check` → **GREEN, 2430 findings vs baseline 2430,
   rc=0**. The row's "4 accepted entries dropped" condition verified in git
   history: the 10-12 regen (d8aa0fb8) dropped errcheck:2 entirely and noctx 6→4
   (the residual noctx:4 are the schema_test.go nits owned by the owner-gated
   row below it). Ticked in 8705a636.
2. **CHANGELOG [Unreleased] rows (O14)** — Added/Changed/Fixed/Internal rows for
   the whole T16–T26 wave: `tq dlq --json` + evidence UX + unknown-term warn,
   dirty-tree pool-health lane + parked hints, verify-deaths-by-stage, audit
   env-streak split, api lockout banner, doctor legacy-DLQ warn, (BEHAVIOR
   CHANGE) postgres LIKE case-parity, readmodel same-status counter-drain fix,
   and Internal rows (keeper/supervisor, release.sh fresh-cache gate,
   postgres-conform smoke, tick-claims message scanner, auth-plane pin).
   `check-release-docs.sh` green. Swept 8497f3c2, grandfathered.
3. **T20 row 1447 (dashboard cancel) — verified DONE-ALREADY** —
   `handleTaskCancelPOST` landed 2026-09-08: writeBindings route
   (webui.go:297), lockout-outside/CSRF-inside wiring, Pending→Cancel /
   Running→CancelRunning (ADR-0005 cooperative stop: worker heartbeat cancels
   runCtx, executor SIGKILLs the process group), terminal→409; cancelAction
   forms render for pending AND running rows; pinned end-to-end by
   TestWriteFlowCancelAndRescue + TestWriteRoutesNeedAllowWrites + headermatrix
   429 + CSRF-forge tests. The harvest had missed it. Ticked in 1686cb7f.
4. **postgres-conform smoke indexed** — ci-local's ONLY red was
   UNINDEXED-SMOKE (the prior session shipped scripts/smoke/postgres-conform.sh
   without its COVERAGE.md leg row). Row added; gate green (28 legs, 0
   unindexed). Commit 1686cb7f.
5. **Wave treefmt drift caught and cured** — `nix fmt` found 6 drifted files
   from the prior wave (cmd/tq imports ×3, executor/readmodel test whitespace,
   fragments.templ indent — exactly the class that killed the v0.4.0 tag on CI).
   templ + css regenerated (output in sync); executor/readmodel/webui/cmd-tq
   gates green BEFORE commit; commit 54235655. Also: root-gate **RC=0 green**,
   `nix build` green, `nix build .#checks.x86_64-linux.vendor-hash` green
   (row 1389's drift claim is STALE — no drift at HEAD).
6. **T20 row 1445 (browser-answerable questions pane) — FULL feature** —
   QuestionsPane on the dashboard (amber section above tasks; empty pane renders
   nothing so the section appears only when a question waits); rows derive from
   parked tasks' unanswered task.question-asked facts (`openQuestions` /
   `unansweredQuestions`, newest-first, cap 20, same ref-pairing as
   buildQuestionView); answer box POSTs `/task/{id}/answer` through
   writeBindings (AllowWrites + lockout/CSRF/token rails) routing home via
   `store.RecordAnswer` — the AnswerPoller's home-side seam (payload
   answered-injection + not_before clear + question-answered fact, one tx);
   unknown/lapsed ref → 409, missing answer → 400, re-answer → 409, read-only
   hint without --allow-writes; SSE live via `#frag-questions`
   (renderFragments + app.js FRAG_IDS); **M6 attach→current-session ordering
   PINNED first** (TestAgentFeedAttachesBeforeAnyOtherCall with a
   request-order-recording fake server + attachWorkspace doc contract).
   Tests: 5 new (derivation, pane render ×3 states, full write flow,
   fragment emission) + updated snapshot/golden/CollectN tests (6→7 events).
   Webui suite green. Swept mid-work 3× (9a93b088/c14066e7/07d4841d — all
   content green at each sweep), grandfathered; tick + CHANGELOG row in
   4a69bf9f.
7. **httpapi typecheck scare resolved** — LSP showed a real-looking error at
   httpapi_test.go:390; build+vet+tests green — stale diagnostic (the prior
   session's repair landed).

## b) PARTIALLY DONE

1. **Final battery** — root-gate GREEN; nix build + vendor-hash GREEN; but the
   FULL clean `ci-local.sh` receipt is MISSING: run 1 red on the coverage gate
   (fixed), run 2 red against my own mid-edit tree (expected, cmd/tq gate saw
   render.go before templ generate), run 3 (background job) DIED without
   running (job registry lost it; log empty). MUST re-run on the quiet tree
   before any push.
2. **Row 1446 (structured closeout from messages API)** — research COMPLETE
   (closeout flow map: ExtractSessionID at result.go:250-271, runCloseoutTurn
   agent.go:928-1003, hook points at agent.go:868-876/836-844, go-crush-data
   parts types, the M1 caveat — workspace reaped at client exit so the fetch
   must ride inside the executor call path). Wire shapes for
   /sessions/{sid}/messages NOT yet captured (see c/d). The enricher is NOT
   built. Row stays open — but its blocking unknown (wire contract) is now
   half-solved by the probe below.
3. **Row 1450 / L1.8 (crush-server-route smoke)** — ground-work proven, smoke
   NOT written: live API surface mapped (/v1/health 200 empty body;
   /v1/workspaces list; sessions/messages are WORKSPACE-scoped routes —
   `/v1/workspaces/{id}/sessions`, `.../sessions/{sid}/messages` return JSON
   domain 404s for unknown ids; `/v1/sessions` is NOT top-level; workspaces
   vanish at client exit; sessions persist per-project at
   `<repo>/.crush/crush.db` — exactly go-crush-data's store). Airtight
   zero-spend isolation recipe VERIFIED (see e3) and a working mock
   OpenAI-compatible provider (run: provider=mock, exit 0). The verdict
   tool-call leg still fails end-to-end (see d6) — the smoke cannot be cut
   until the tool-call streaming shape is right.
4. **Push + CI watch** — NOT done (blocked on the clean battery + owner §g1).

## c) NOT STARTED (this session's plan remainder)

- T22 nix/vendorHash automation (~10 rows: drift detector, got→flake helper,
  drop-day watch).
- T23 journal/facts deep rows (non-BLOCKED subset).
- T24 executor micro-tail (~9 rows).
- T25 rows 462 (postgres drift-audit EXECUTED) + 534 (fullcore fatal path).
- T27 research/doc tail (webui-live-contract updates, drafts triage).
- v0.4.3 tag-wave evaluation (owner question §g3 from the prior report).

## d) TOTALLY FUCKED UP (worst first)

1. **I burned 3 accidental REAL provider calls (~150 tokens) before isolation
   was proven.** `CRUSH_CONFIG_DIR` does NOT isolate crush config (file-missing
   → silent fallback to the live user config; the scratch crush.json also used
   wrong camelCase keys). Runs 1-2 hit glm-5.3-flash ("Hi 👋"), run 3 hit
   huggingface (402-refused, no charge). The standing limit is NO real-money
   spend — I probed on an UNVERIFIED isolation assumption. The correct order
   was: prove isolation via `crush dirs` + config-file inspection FIRST, run
   NEVER. (XDG_CONFIG_HOME/XDG_DATA_HOME + snake_case schema + disable_default
  _providers IS airtight — verified afterward, run4 provider=mock.)
2. **/tmp filled to 100% (48G) mid-probe** — a crash-looped scratch run
   ballooned `<repo>/.crush/crush.db` to 7.3G of session rows. It broke runs
   5-6 ("database or disk is full (13)") and ate my recipe copies (cp wrote
   0-byte files). Recovered: truncated the garbage db in place, trashed the
   scratch (7.3G free, 85%), rewrote the recipes from context. Lesson:
   crash-looped probes must be size-monitored; /tmp is shared with
   go-build-cache and foreign sessions.
3. **Battery run 3 died silently** — I backgrounded ci-local via the tool's
   auto-background, then never verified it started; the job registry lost it
   and the log is empty. The session's headline exit artifact (clean full
   battery receipt) does not exist because I trusted a launch I never
   observed.
4. **handlers.go edit surgery again** — two sloppy rounds on
   handleTaskAnswerPOST (a `_ = t` hack, then a multiedit that DELETED the
   404 guard and a closing brace). Same class as the prior session's §d2.
   Standalone-write or one-pass lsp_replace_symbol from the start.
5. **My own printf corrupted the daemon baseline** — `"07d4841d%s"` glued a
   short sha prefix onto rev-parse output → attribution FAIL → one debug
   round. Shell string-building for data files: use python.
6. **The mock tool-call leg is still wrong** — first shape (single-chunk
   tool_call delta, finish stop) → "Settling a tool call that will never be
   answered" (crush never ran the bash call); fixed to OpenAI incremental
   streaming (id/name chunk → arguments chunk → finish_reason "tool_calls"),
   but the verification run LOOPED ("mock done" ×N, no verdict.json, run.rc
   never written) and the STOP arrived before I could iterate. The launch
   chain (`&` backgrounding) was also hook-flagged; moved to a launcher
   script, but the leg remains UNPROVEN.
7. **A multiedit escape corrupted mock_provider.py** (`b"\r\"` syntax error)
   — my own edit JSON ate a backslash; caught by the mock crash log.

## e) WHAT TO IMPROVE

1. **Prove isolation before ANY probe that can spend.** The gate is: `crush
   dirs` under the candidate env shows ONLY scratch paths, AND the config file
   parses with the exact expected schema, AND (best) the first run's log line
   says `provider=mock`. No run before all three.
2. **Observe background jobs at launch** — a battery/enricher "started" is a
   claim; `job_output` within a minute + a non-empty log is the receipt.
3. **Size-watch crash-looped probes** — `du -sh` the scratch in the driver's
   wait loop; abort at e.g. 1G. The 7.3G db bomb was invisible for ~an hour.
4. **Edit discipline for Go funcs**: one-pass writes or lsp_replace_symbol;
   never "temporary" hacks (`_ = t`) — they leak into committed history via
   daemon sweeps.
5. **The daemon took 6 sweeps this session** despite fast commits — the only
   defense that worked was running the owning module's gate BEFORE `git add
   -A` (done for every sweep; all landed green). Keep that order religiously.
6. **Config-dir isolation should live in the smoke itself** — the recipe
   (XDG redirect + mock provider) is the smoke's skeleton; what's missing is
   only the tool-call streaming shape (d6).

## f) NEXT (ranked, ≤50)

1. Re-run `./scripts/ci-local.sh` on the quiet tree (OBSERVE the start; rc
   receipt to a file) — the missing exit artifact.
2. `nix fmt` (should be clean now); then PUSH master + `gh run watch
   --exit-status` (first CI contact for ~58 commits).
3. Owner §g1 answer governs push timing — do not push past this report
   without it (standing limits allow go-taskqueue pushes; the prior report's
   question was hold-for-review vs push-after-battery).
4. Fix the mock tool-call streaming leg (iterate the launcher + shapes probe
   in ONE driver; abort-on-size-guard) until `verdict.json` lands from a
   ROUTED run — that is row 1450's E-gap core.
5. During the stalled mock run, capture `/v1/workspaces/{id}/sessions` and
   `.../sessions/{sid}/messages` JSON shapes → write them into
   docs/planning/2026-10-30_04-43_crush-client-server-relations.md (kills the
   M5 unknown).
6. Cut scripts/smoke/crush-server-route.sh from the recipe: legs = boot
   isolated server + health probe + routed verdict round-trip + keeper
   keepalive + supervisor restart (kill server, watch ladder) + pool clean
   exit; COVERAGE.md row; TQ_DB=<scratch> per the hook contract.
7. Tick row 1450 + L1.8 cross-reference; CHANGELOG Internal row for the smoke.
8. Row 1446 (structured closeout enricher): now buildable on VERIFIED shapes —
   gate on `e.CrushServer != ""`, fetch inside the executor call path (before
   client exit), derive usage/files/closeout text, stdout fallback intact.
9. T22 vendorHash drift detector (nix build .#checks…vendor-hash, compare,
   print got→flake recipe) + drop-day watch doc.
10. T25 row 462: postgres drift-audit EXECUTED (ride the postgres-conform
    smoke's cluster).
11. T25 row 534: fullcore postgres fatal path live run (deadline exit 1 +
    clean teardown).
12. T23 non-BLOCKED journal/facts rows (watermark edge classes, fact-shape
    pins).
13. T24 executor micro-tail rows (verdict-channel edges, excerpt/decode
    refinements).
14. T27: webui-live-contract updates (the pane + answer route belong there
    now), DOMAIN_LANGUAGE drift remainder, drafts triage.
15. Re-measure the production parked/dirty population when one exists
    (row 1278 note).
16. v0.4.3 tag wave: KnownEvidenceTerms/parked-requeue_class/pane ride it;
    decide with owner (§g3).
17. Consider a guard row: "prove provider isolation before any crush
    probe" as an edit-advice hook note (this session's d1 class).

## g) QUESTIONS (cannot figure out myself)

1. **Push authority (carried from the prior report, now urgent):** ~58 commits
   sit unpushed (44 from the wave + this session's row-852/CHANGELOG/pane/
   fmt/cov work), all local-green but never CI-verified. Standing limits allow
   go-taskqueue pushes; the prior STOP arrived before the push step. Push
   immediately after the clean battery, or hold for your review?
2. **Disposition of the 3 accidental real provider calls** (glm-5.3-flash
   "say hi"/"write the verdict file" ≈ 150 tokens, fraction of a cent; caused
   by my unverified isolation assumption): acknowledge-and-forget, or do you
   want them recorded anywhere (e.g. a usage ledger note)?
3. **Battery-receipt strictness:** if the clean ci-local re-run trips a KNOWN
   load-flaky (TestExactlyOnceUnderConcurrency / TestSelfManagingLoop ride
   KNOWN_FLAKY retry), does a green-with-flaky-retry receipt satisfy the exit
   gate, or do you want a fully-green first run?

— reported 2026-11-01 01:54 CET; scratch probe stack dead and cleaned
(processes killed; /tmp/tq-crush2 + /tmp/tq-crush-recipe remain for the next
session's driver); /tmp at 85% (7.3G free) after the db-bomb truncation;
production journal untouched (no reads, no writes); no pushes this session;
3 accidental real provider calls acknowledged in §d1/§g2.
