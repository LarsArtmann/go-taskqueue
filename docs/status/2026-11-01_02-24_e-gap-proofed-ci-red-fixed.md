# Session status — E-gap core proven (routed verdict round-trip), CI-red found+fixed, pre-smoke (interrupted)

**2026-11-01 02:24 CET · Continuation burst after the 01-54 report (docs/status/2026-11-01_01-54_row852-changelog-questionspane-crushprobe.md) under the owner's repeat GO. Scope this burst: battery run 4 (observed), CI-red discovery + facade parity fix, the row-1450 E-gap mock chain fixed to a PROVEN routed verdict round-trip. INTERRUPTED by owner STOP — CI 41260643333 IN PROGRESS on the fix commit, local/origin carry a grandfather twin to reconcile, the smoke script itself not yet cut.**

---

## a) FULLY DONE (this burst)

1. **E-gap core PROVEN under routing (row 1450's heart, zero spend)** — a
   real `crush run -H <socket>` against a real `crush server`, with a
   hermetic mock OpenAI-compatible provider, lands the verdict channel:
   `verdict.json` = `{"verdict":"ok-routed"}` written by the agent turn's
   bash tool in the workspace repo, run rc=0, exactly 3 provider calls.
   The fix chain, each step verified before the next:
   - request logging added to the mock (roles + last-user per request);
   - root cause 1: title-generation requests QUOTE the task prompt — the
     mock now matches only when the LAST user message IS the task (template
     heads "Generate a concise title"/"Provide a detailed summary"
     excluded) — the tool_call had been going to the title conversation;
   - root cause 2: auto-summarize spun forever on inert answers —
     `"disable_auto_summarize": true` in the scratch config (this was the
     earlier 601-call loop);
   - root cause 3 (found by READING `repo/.crush/crush.db`): crush's bash
     tool REQUIRES a `description` param — the tool_result said
     `"missing required parameter: description"` — added to the
     tool_call arguments.
2. **Session wire shapes CAPTURED from the live DB** (row 1446's blocking
   unknown, half-solved): `messages` rows carry parts JSON —
   `{"type":"tool_call","data":{"id","name","input","provider_executed","finished"}}`,
   `{"type":"tool_result","data":{"tool_call_id","name","content","is_error"}}`,
   `{"type":"text","data":{"text"}}`,
   `{"type":"finish","data":{"reason","time"}}` — plus the `sessions`
   table and roles; sessions persist per-project at
   `<repo>/.crush/crush.db` (go-crush-data's own store).
3. **CI red on origin discovered, root-caused, FIXED** — the auto-git
   daemon had PUSHED the session's unpushed commits (origin hit 4a69bf9f
   at 19:39); CI run 41235246346 failed on **Facade parity (ADR-0016)**:
   `internal/executor.KnownEvidenceTerms` / `UnknownEvidenceTerms` (the
   T16 wave's exports) had no facade re-export. Both aliases added to
   `executor/executor.go`; facade module build green;
   `check-facade-parity.sh` GREEN (7 facades mirror). The fix reached
   origin via the daemon sweep aa043e21 + a concurrent grandfather
   9aea94ee; **CI 41260643333 on 9aea94ee was IN PROGRESS (5m21s) at
   STOP** and origin's tree verifiably carries the alias.
4. Battery run 4 launched AND observed alive (the prior report's §d3
   lesson applied: log verified growing at +20s).

## b) PARTIALLY DONE

1. **Clean ci-local receipt — still missing (third attempt)** — run 4
   executed correctly and FAILED at `check-ci`, which is the gate working:
   origin's CI was red on 4a69bf9f. Once 41260643333 greens, re-run
   ci-local for the receipt.
2. **Row 1450 smoke** — the E-gap verdict leg is proven at probe level;
   the actual `scripts/smoke/crush-server-route.sh` is NOT cut: remaining
   legs = HTTP-shape capture during a stalled run, TQ_QUESTION_FILE env
   round-trip (server env → workspace tool), 429 fast-refuse signature
   (mock 429 mode → routed run error surface), keeper/supervisor restart
   ladder + pool clean exit, scratch size-guard, COVERAGE.md row, tick.
3. **Local/origin divergence** — local 631e7e08→9aea94ee is a same-subject
   twin of the origin commit (a concurrent agent grandfathered the same sweep);
   needs a rebase reconcile (no force push; the twin should drop empty or
   conflict trivially on the baseline file).

## c) NOT STARTED (unchanged from the 01-54 report)

- Row 1446 structured closeout enricher (shapes now captured — buildable).
- T22 vendorHash drift detector + drop-day watch.
- T25 rows 462 (postgres drift-audit EXECUTED) and 534 (fullcore fatal path).
- T23 non-BLOCKED journal/facts rows; T24 executor micro-tail; T27
  webui-live-contract + drafts triage.
- Final battery + closeout + confirmed-green CI.

## d) TOTALLY FUCKED UP

1. **My facade-fix commit was a silent NO-OP** — the command
   auto-backgrounded; by the time it ran, the daemon had already
   committed the file and a concurrent flow had grandfathered it
   ("nothing to commit"). Harmless this time, but I did not immediately
   re-verify WHO carried my edit — I found out only at STOP by grepping
   origin's tree. Ownership tracing of my own edits must be immediate,
   not incidental.
2. **The grandfather twin** — I worked ~40 minutes without pulling while
   a concurrent agent pushed the same-subject grandfather; the 1/1
   divergence was a surprise at commit time. Pull/rebase at burst
   boundaries when other agents are live.
3. **Battery receipt: three runs, zero receipts** — coverage-gate red
   (fixed), mid-edit red (self-inflicted), check-ci red (external). The
   exit artifact keeps slipping; it is now strictly blocked on CI green.
4. **Two blind mock iterations before instrumenting** — the
   template-head and auto-summarize bugs were each one request-log read
   away; I only added logging on iteration three. Instrument FIRST.

## e) WHAT TO IMPROVE

1. Instrument first: request logs / DB reads before hypothesizing (the
   `description` param bug was un-guessable from outside and trivial in
   the DB).
2. Pull before every new burst (concurrent agents + a pushing daemon make
   stale local bases the default, not the exception).
3. Treat every local commit as CI-live within minutes (the daemon
   pushes) — never leave a red tree, even briefly; run
   check-facade-parity in the same change as any internal export (the
   wave missed it; CI caught it one push later — the gate worked, but a
   same-change run would have kept master green).
4. The mock recipe (template-head matcher + disable_auto_summarize +
   description param + loop guard) is now the canonical skeleton for the
   smoke — encode ALL of it, not just the happy path.

## f) NEXT (ranked, ≤50)

1. Watch CI 41260643333 to completion (`gh run watch --exit-status`) —
   the facade fix should green it.
2. Reconcile the twin: `git pull --rebase` (twin drops empty or trivial
   baseline conflict); re-run check-daemon-attribution.
3. Re-run `./scripts/ci-local.sh` for the STILL-MISSING clean receipt.
4. Capture the HTTP shapes live: stalled mock run + probe
   `/v1/workspaces/{id}/sessions` and `.../sessions/{sid}/messages`
   (the DB shapes are captured; the HTTP envelope is not) → write both
   into docs/planning/2026-10-30_04-43_crush-client-server-relations.md.
5. Cut `scripts/smoke/crush-server-route.sh`: isolated XDG boot + health
   probe + routed verdict round-trip + TQ_QUESTION_FILE env leg + 429
   signature leg + keeper/supervisor restart + pool clean exit; scratch
   size-guard (the 7.3G db-bomb class); `TQ_DB=<scratch>`; COVERAGE row;
   tick row 1450 + L1.8 cross-ref; CHANGELOG Internal row.
6. Row 1446 enricher on the verified shapes (gate on `e.CrushServer`,
   fetch inside the executor call path, stdout fallback intact).
7. T22 vendorHash drift detector + formatterWithGo drop-day watch doc.
8. T25 row 462 postgres drift-audit EXECUTED (ride postgres-conform
   smoke's cluster); row 534 fullcore fatal path live run.
9. T23 non-BLOCKED journal/facts rows; T24 executor micro-tail.
10. T27: webui-live-contract update (questions pane + answer route
    belong there now), DOMAIN_LANGUAGE drift remainder, drafts triage.
11. Final battery + closeout report + confirm CI green end-to-end.

## g) QUESTIONS (cannot figure out myself)

1. **The auto-git daemon PUSHES to origin** (that is how 4a69bf9f reached
   CI at 19:39 without me pushing) — please confirm this is known and
   intended; it retroactively answers the prior report's push-authority
   question (the system pushes regardless), but it also means any local
   red tree can hit CI within minutes.
2. **Concurrent-agent twin commits** (local 631e7e08→9aea94ee, same subject,
   different shas): reconcile locally by
   rebase-and-drop-empty, or do you want the heal tooling extended for
   this twin class?
3. **Carried from 01-54 §g2**: the 3 accidental real provider calls
   (~150 tokens) — acknowledge-and-forget, or record them somewhere?

— reported 2026-11-01 02:24 CET; probe stack cleaned (processes dead);
recipes intact at /tmp/tq-crush-recipe (mock with all three fixes +
launcher); CI 41260643333 in flight on the facade fix; production journal
untouched; no pushes by me this burst (the daemon pushed).
