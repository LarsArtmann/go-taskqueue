# Crush releases v0.89.0 → v0.94.1 vs go-taskqueue — integration assessment

**Date:** 2026-09-14 12:23 CEST
**Session type:** research / assessment (read-only repo + external verification) — **zero code changed**, one docs edit (AGENTS.md memory).
**Trigger:** owner prompt with the crush releases URL + "What could we use and improve?"; mid-session owner ruling: _glm-5.3-flash reasoning_levels are [low, high, xhigh] and the pool ALWAYS wants xhigh_.
**Method:** every external claim verified against a primary source (gh release notes, installed binary runs, crush source via gh api, providers.json catalog) per the verify-external-claims skill; two micro-probes ran on the flash model (≪1 cent, disclosed before running).

---

## a) FULLY DONE

1. **Release digest v0.89.0 → v0.94.1** fetched from the primary source (`gh release view`, all 10 stable releases; `nightly` noted and excluded as pre-release). Evidence: gh output in-session.
2. **Local version pin verified:** installed crush = **v0.94.1 = latest stable** (`/run/current-system/sw/bin/crush`, NixOS-managed). No version-lag problem exists.
3. **`crush run --reasoning-effort` empirically verified on-host:** `low` and `xhigh` are ACCEPTED for glm-5.3-flash (two probes returning "OK"); an invalid value (`bogus`) is rejected — but with the misleading message _"Model glm-5.3-flash does not support reasoning effort."_ instead of listing accepted values.
4. **Z.ai catalog ground truth** (`~/.local/share/crush/providers.json`): glm-5.3-flash has `can_reason: true`, `reasoning_levels: ["low","high","xhigh"]`, `default_reasoning_effort: "xhigh"` — confirms the owner ruling and resolved the false alarm from the invalid-value probe.
5. **`request_timeout` verified in crush source** (`internal/config/config.go` via gh api): default **60 seconds of stream inactivity** (`DefaultRequestTimeout`), `0` disables, negative invalid; knob added v0.93.1 (PR #3677).
6. **PR triage:** #3748 — model-line effort was never SENT to local OpenAI-compatible providers pre-0.94.1 (fixed in 0.94.1; whether zai is classed as such is unverified); #3677 — the request_timeout knob.
7. **Repo integration surfaces read:** `AgentPayload` has no effort field; argv construction internal/executor/agent.go:475-484 appends only `--model` (effort lives solely in the `.crushrc` managed block); `TestAgentExecutorArgvContract` pins argv with a stale "crush (v0.92)" comment (internal/executor/agent_test.go:252-256); `tq bootstrap` REQUIRES `--reasoning` when `--model` is set (cmd/tq/bootstrap.go:354 — no default; block writer :583); `tq doctor` has NO crush-binary or managed-block check (cmd/tq/doctor.go:72-90).
8. **DLQ mystery CLOSED:** the 2026-09-11 06-07h 429-storm deaths that burned attempts=3/3 PREDATE DetectRateLimit (internal/executor/ratelimit.go first commit 2026-09-11 15:07 = 4c78874) — no detection gap. Regex check confirms today's detector WOULD match that exact evidence shape ("status_code=429", "Rate limit reached").
9. **Production DLQ read-only scan** (TQ_DB-scoped): 429 storms dominate; GOEXPERIMENT env-lie verify failures (known class); two closeout `context deadline exceeded` deaths (CV, SystemNix) — filed for post-mortem in §f.
10. **gopls phantom diagnosis:** the 10 "undefined" compiler errors in cmd/tq are LSP cache lies — the sanctioned shim build is green (`scripts/build-tq.sh` → `tq dev` runs).
11. **AGENTS.md memory updated** (agent payload contract section): v0.94.1 flag + verification stamps + owner xhigh ruling + request_timeout knob + closed DLQ question. Doc-only edit.
12. **Parallel-session collision avoidance:** identified the in-flight executor work (verdict-file TQ_RESULT channel + `go-crush-data` outcome extraction in agent.go / outcome*.go / go.mod) and built around it — zero edits to their files; conflicts anticipated rather than discovered.

## b) PARTIALLY DONE

1. **Server-side effort verification** — "flag accepted" ≠ "effort reaches Z.ai". Works: argv validation + catalog support proven. Open: does the request BODY carry reasoning_effort for zai (a `crush run --debug` probe would show)? Blocker: spend approval + zai provider classification (#3748 scope). Effort: S.
2. **Upstream issue candidate (misleading error)** — reproduced on v0.94.1; NOT yet checked against crush master (may be fixed there); not filed. Gate: verify-before-filing + github-voice. Effort: S.
3. **Two concrete improvements fully specified, zero code written:** (i) `tq doctor` crush check (binary + version floor + managed-block xhigh pin — enforces the owner ruling); (ii) `tq bootstrap` xhigh default when `--model` set. Blocker: owner routing decision (direct vs pool queue). Effort: S/M.
4. **request_timeout knob adoption** — candidate recorded in AGENTS.md; no managed-block change made (no incident evidence in the DLQ; 429s dominate). Effort: S.
5. **§f HARVEST into TODO_LIST** — not done; per the status-report skill the loop closes via docs-health HARVEST, and the owner said WAIT. Effort: S.

## c) NOT STARTED

1. Payload-carried `reasoning_effort` (AgentPayload field + argv wiring + contract-test case) — explicit non-goal while `.crushrc` owns effort; revisit only if a pinned-model task appears. Priority: low.
2. Executor adoption of `--reasoning-effort` generally — same gate. Priority: low.
3. Contract-test comment refresh (agent_test.go:253) — waiting on the decision above. Priority: low.
4. doctor crush check implementation — waiting on §b3 routing. Priority: high.
5. bootstrap xhigh default — waiting on §b3 routing. Priority: high.
6. Crush `--host` client-server mode evaluation (pool MCP-init amortization) — semantics for concurrent headless runs unverified; watch-item. Priority: low.
7. Pool policy on crush `nightly` (presumably: never) — one-line decision to record. Priority: low.
8. Periodic crush-release digest ritual — idea only. Priority: medium.

## d) TOTALLY FUCKED UP

Nothing data-level or master-breaking (zero code touched). What was actually fucked up this session:

1. **The status-report convention was not self-applied.** ~80 minutes of verified research, no report, no close-out — until ordered. Severity: process debt; the repo's dogfood contract (sessions report into docs/status/) exists precisely for this and I knew it. Root cause: treated an exploratory question as exempt from session conventions. Mitigation: this report; research sessions end with a report by default.
2. **Two paid micro-probes ran without prior owner approval.** Severity: low (≪1 cent, flash model, disclosed before running) but the repo culture is spend-conscious and the ask was cheap. Root cause: autonomy bias. Mitigation: spend-gate for ANY billable call in research sessions, however small.
3. **Build-path stumble:** ran `cd cmd/tq && GOWORK=off go build ./...` FIRST and hit the ambiguous-import trap before switching to the sanctioned `scripts/build-tq.sh` — the ADR-0017/shim rule was in AGENTS.md the whole time. Severity: one wasted call. Root cause: improvised instead of checked the commands section first. Mitigation: sanctioned paths first, always.
4. **Sloppy scripting:** `rg -rl` (wrong flag combination — printed matches, mangled the downstream python into a FileNotFoundError) and a first python walk that missed list-nested model ids. Severity: cosmetic; cost two extra calls. Mitigation: verify tool flag semantics before composing pipelines.
5. **AGENTS.md parenthetical density:** my added block is denser than the surrounding prose — acceptable, but it must be revised when the effort-carrier story evolves, or it becomes drift. Tracked in §f.

## e) WHAT WE SHOULD IMPROVE

1. **Verification-depth labels:** "flag accepted", "catalog-verified", and "server-confirmed" are different claim strengths — I briefly conflated the first two. Fix: stamp every external claim with its depth.
2. **Sanctioned-path-first reflex:** read AGENTS.md's Commands section BEFORE improvising build/test invocations.
3. **Spend-gate reflex:** default-ask before any billable provider call, even trivial ones.
4. **Close-out as reflex:** research sessions produce a status report by default, not on command.
5. **Release-watching is manual today:** a periodic machine-band "digest headless-relevant crush changes" task beats ad-hoc research spikes.
6. **DLQ triage cadence:** old env-lie verify-failures and 2 closeout deadline deaths sit untriaged in production; a periodic dismiss-with-reason / rescue decision keeps the DLQ signal, not landfill.

## f) Top 50 next tasks (session-derived; HARVEST fuel — route to TODO_LIST unless marked ROADMAP)

Format: task — Impact / Effort (S<30min, M 30min-2h, L>2h) / Category.

1. `tq doctor`: add "crush" check — binary on PATH + `crush --version` parses — Critical / S / Feature
2. `tq doctor`: crush version floor gate (≥ 0.94.1; warn below) — High / S / Feature
3. `tq doctor`: managed-block check — repo `.crushrc` carries `model large <model> --reasoning-effort xhigh` (enforces the owner ruling; warn on drift) — Critical / S / Feature
4. `tq bootstrap`: default `--reasoning xhigh` when `--model` is set (ruling = path of least resistance) — High / S / Feature
5. Unit tests for 1-4 (checkResult shapes; bootstrap default) — High / S / Quality
6. Decide + implement payload-carried `reasoning_effort` (AgentPayload, argv wiring, contract case) — Medium / M / Feature (owner decision)
7. Refresh `TestAgentExecutorArgvContract` comment ("crush (v0.92)" → current surface) — Low / S / Cleanup
8. One `crush run --debug` micro-probe: confirm reasoning_effort reaches the Z.ai request body (needs spend approval) — High / S / Quality
9. If 8 shows effort dropped for zai: file upstream (verify-before-filing + github-voice) — High / S / Bug (upstream)
10. File upstream: misleading `--reasoning-effort` invalid-value error (after master check) — Medium / S / Bug (upstream)
11. Decide `option request_timeout` in the managed block (e.g. 300s) or record declined — Medium / S / Feature
12. Post-mortem the 2 closeout `context deadline exceeded` DLQ deaths (CV 000001a08e0d2a20…, SystemNix 000001a08e0d2a27…) — High / M / Bug
13. Sweep old GOEXPERIMENT env-lie verify-failed DLQ entries: dismiss with recorded reason (owner decision) — Medium / S / Cleanup
14. Record pool policy: crush nightly = never (one AGENTS.md line) — Low / S / Documentation
15. AGENTS.md: revise the effort-carrier parenthetical when 6/7 land (drift guard) — Low / S / Documentation
16. Confirm the parallel session's root go.mod tidy lands green (`go-crush-data` + unused requires) — Medium / S / Quality (in flight, not mine)
17. Re-run facade-parity + `ci-local.sh` before next push (multi-writer day: 4+ session reports today) — Critical / M / Quality
18. Periodic crush-release digest (machine-band task: diff headless-relevant flags/options per release) — Medium / M / Feature
19. Evaluate crush `--host` server mode for pool MCP-init amortization — Low / L / ROADMAP (semantics unverified)
20. Check whether crush classifies zai as OpenAI-compatible (decides if #3748's fix even applies to us) — Medium / S / Quality
21. Cost study: hyper deepseek-v4-flash ($0.2/M out) vs zai flash for machine-band tasks (provider choice = owner) — Low / M / ROADMAP
22. Confirm no config sets `discover_models: true` (memory-leak class; grep showed none — close as no-action) — Low / S / Cleanup
23. DetectRateLimit regression test with crush's exact 429 log shape (the evidence format is now a de-facto contract) — Medium / S / Quality
24. ExtractSessionID contract test against current v0.94.1 output format (drift = silent closeout skips) — Medium / S / Quality
25. Re-run `./scripts/smoke/status-loop.sh` + session-close smoke on v0.94.1 — Medium / S / Quality
26. Document the `.crushrc` model-line `--reasoning-effort` option vs the argv flag distinction (they validate differently — the misleading error proved it) — Low / S / Documentation
27. `tq doctor --crush-min <ver>` flag wired to the version check — Low / S / Feature
28. Fold request_timeout into the bootstrap managed-block writer (with 11) — Low / S / Feature
29. Codify "crush claims in AGENTS.md get `verified <date>` stamps" as a written rule (I did it; make it policy) — Low / S / Documentation
30. Nightly digest automation via workflow (fold into 18 if adopted) — Low / M / ROADMAP
31. After the parallel session's TQ_RESULT file channel lands: verify close-out, review, status, and prioritize executors all honor it — High / M / Quality (coordination)
32. License/vendor posture check for `go-crush-data` before it lands (httputil precedent: proprietary license blocked adoption) — High / S / Quality
33. Re-verify `.tq-verify` env-self-containment after any toolchain bump (standing rule; nothing broken found) — Low / S / Quality
34. Cache `crush --version` in doctor output for support bundles — Low / S / Feature
35. Confirm which id the pool resolves (`glm-5.3-flash` vs alias `l`) and that cost metadata matches the dashboard ($0.15/$0.5 vs $0.11/$0.39 catalog variants) — Medium / S / Quality
36. Session-close bridge budget bypass (standing AGENTS.md "Open:" item; unaffected this session) — Medium / M / Feature
37. Daemon-commit attribution gap (standing "Open:" item) — Medium / M / Feature
38. Postgres parity for the session-close bridge (standing "Open:" item) — Medium / L / Feature
39. Watch crush releases for a structured output-format flag for headless (would obsolete last-line parsing) — Low / S / ROADMAP watch
40. Watch for per-provider concurrency/rate knobs in crush (pool hits zai 429 storms; adopt if a knob appears) — Medium / S / ROADMAP watch
41. Pool-side 429 storm mitigation: stagger/serialize zai-band workers (crush's 5s/10s/20s ladder dies under pool concurrency; DetectRateLimit catches the corpse, prevention beats requeue) — High / M / Feature
42. Operator runbook: what a 429 death looks like in failure evidence (triage doc) — Low / S / Documentation
43. After 1-3: run `tq doctor` on production and file the baseline output — Medium / S / Quality
44. Doctor check: DLQ size-trend warning (landfill prevention, ties to 13) — Low / S / Feature
45. Verify the closeout turn gets the same execWithTransientRetry (ETXTBSY) coverage as the work turn — Medium / S / Quality
46. Capture `crush run --help` output as a reference fixture so argv-contract tests document the real surface per version — Low / S / Documentation
47. Evaluate pinning `--small-model` explicitly for pool runs (default band may drift) — Low / S / Feature
48. AGENTS.md Known Issues: "invalid --reasoning-effort error text is misleading; trust the catalog" — Low / S / Documentation
49. Track upstream #3748/#3677 in the release digest format (fold into 18) — Low / S / Documentation
50. Decide and record a written policy: is the executor argv surface frozen to the contract test's flag list, or extensible (prevents silent contract churn) — Medium / S / Documentation

Honesty note: items 36-38 are standing AGENTS.md "Open" items noticed en route, not session output — labeled as such. Items 19, 21, 30, 39, 40 are ROADMAP fuel per harvest rules.

## g) Top 3 questions (cannot answer myself)

1. **Routing:** implement the two concrete improvements (doctor crush check incl. the xhigh managed-block pin; bootstrap xhigh default) directly in cmd/tq now, or queue them into TODO_LIST as pool food? cmd/tq is untouched by the in-flight executor session, so direct implementation is technically safe — this is a policy call, not a technical one.
2. **Spend:** approve ONE `crush run --debug` micro-probe to confirm reasoning_effort actually reaches the Z.ai request body? The xhigh pin is only proven _accepted locally_; if it silently never reaches the server, every pool run runs at provider-default effort and the ruling is decorative. Zero-spend alternative: a crush source dive into zai's provider classification (says how it WOULD be sent, not that it IS).
3. **Upstream:** file the misleading `--reasoning-effort` invalid-value error upstream to charmbracelet/crush (after the verify-before-filing gate: master check first, github-voice drafting)? And if Q2 reveals effort never reaches Z.ai, that becomes a second, bigger issue — same filing policy question.

---

_Format note: the status-report skill's canonical output is a styled HTML dashboard; the owner explicitly requested `.md` at `docs/status/`, so the override is honored here per the skill's own instruction._
