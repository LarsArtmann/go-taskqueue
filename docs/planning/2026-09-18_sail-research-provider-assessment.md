# Sail Research — provider + sandbox assessment

**Date:** 2026-09-18
**Status:** NOT ADOPTED — assessment only. No code, no config, no billing.
Parked as a raw idea for future consideration; owner decision pending.
ROADMAP pointer: the Fleet section (`Multi-model fleet` neighbor row).

## What Sail is

[Sail Research](https://docs.sailresearch.com/) sells two things relevant
here:

1. **Serverless LLM inference** — OpenAI-compatible (Responses, Chat
   Completions) and Anthropic-compatible (Messages) endpoints, a batch API,
   background mode (202 + completion webhook), idempotency keys, Supercache
   (stored prompt prefixes at ultra-low read cost), and per-request
   **completion windows** (ASAP / Balanced / Flex) that trade latency for
   price. Prompt caching is implicit prefix-based, with an optional
   `prompt_cache_key` routing hint.
2. **Sailboxes** — cloud sandboxes for long-horizon agents: exec API with
   reconnect cursors, checkpoint/fork (fleet of identical environments),
   credential injection ("an API key it can never read"), egress policies,
   autosleep, volumes, short-lived SSH certificates, custom domains.

Plus **Voyages** (observability/timelines for background agents) and a
**Usage API** (spend, tokens, latency).

SDKs: Python, TypeScript, Rust — **no Go SDK**. Fine for this repo: the
HTTP surface is plain REST, and the pool reaches inference through Crush
anyway (tier-1 experiment needs zero Go code).

Crucially, Sail serves **GLM-5.3 and GLM-5.3-Flash** — the exact models the
dogfood pool runs today via Z.ai. The models page documents GLM-5.3
reasoning-effort semantics that match the pool's ruling (`none` → HTTP 400,
`low` is the lowest native tier), so the `.crushrc`-managed effort
(`xhigh` for pool agents) carries over conceptually.

## Why relevant to tq

The pool is single-provider (Z.ai / GLM-5.3-Flash, pinned by each repo's
bootstrap-managed `.crushrc`). 429 storms are the dominant task-death class
— the whole `DetectRateLimit` / `RateLimitError` machinery
(`internal/executor/ratelimit.go`) and the per-repo fast-refuse gates
(`internal/executor/agent.go`, `rateLimitGates`) exist because of them.
Sail is a second source for the same model family, at lower list prices,
whose error semantics (`429`/`503` with standard `Retry-After`) land inside
patterns the queue already parses.

## Benefits, ranked by fit

| # | Sail capability                             | tq benefit                                                                                                                                                                                                                                                                                                                                                                                   |
| - | ------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1 | Second inference provider (same GLM models) | 429-storm relief via failover instead of parking. Seam = repo `.crushrc` (already the model+effort carrier). Post-acceptance 503 ("accepted, then capacity shortage") is the one NEW failure shape worth a `DetectRateLimit` pattern.                                                                                                                                                        |
| 2 | Idempotency keys                            | Maps 1:1 onto crash-reclaim: task ID as idempotency key means a re-claimed retry can never double-bill a submission — the queue's dedup semantics, mirrored at the provider boundary.                                                                                                                                                                                                        |
| 3 | Supercache                                  | Every work/review/autopsy/status prompt embeds the same large contract block (reviews literally quote the work contract). Hundreds of runs/day × shared prefix — but only pays off if the stable prefix stays byte-identical ahead of per-task substitution.                                                                                                                                 |
| 4 | Usage API                                   | Authoritative spend feed for `internal/budget` daily caps and `tq stats` — org-wide actual spend instead of session-reconstructed costs via go-crush-data.                                                                                                                                                                                                                                   |
| 5 | Sailboxes (execution substrate)             | Biggest change, biggest upside. Isolation from the host's documented failure modes (systemd env lies, ETXTBSY, PATH gaps); checkpoint/fork for warm per-repo fleets; autosleep for idle windows. **Credential injection** removes agent-readable push credentials; **egress policies** make contracts like "prioritize scorer is READ-ONLY" mechanically enforced instead of prompt-honored. |
| 6 | Batch API                                   | Natural executor variant for latency-tolerant machine-band work (prioritize batch scoring) — but it changes the verdict-channel shape (batch result → verdict file), so it is a new executor, not a config change.                                                                                                                                                                           |
| 7 | Voyages                                     | Marginal: the fact journal + status reports + go-crush-data session forensics already out-observe it at the queue level. Only matters if runs move into Sailboxes.                                                                                                                                                                                                                           |

## Price comparison (verified 2026-09-18)

All USD per 1M tokens. Sail's three completion windows trade latency for
price (ASAP = lowest latency).

### GLM-5.3-Flash (the pool's current model)

|                 | Input | Cached | Output | vs Z.ai         |
| --------------- | ----- | ------ | ------ | --------------- |
| Z.ai (api.z.ai) | $0.15 | $0.03  | $0.50  | —               |
| Sail ASAP       | $0.11 | $0.02  | $0.35  | ~27-30% cheaper |
| Sail Balanced   | $0.08 | $0.02  | $0.28  | ~33-47% cheaper |
| Sail Flex       | $0.05 | $0.01  | $0.18  | ~64-67% cheaper |

### GLM-5.3 (the pool's xhigh-effort upgrade path)

|                 | Input | Cached | Output | vs Z.ai         |
| --------------- | ----- | ------ | ------ | --------------- |
| Z.ai (api.z.ai) | $1.40 | $0.26  | $4.40  | —               |
| Sail ASAP       | $0.98 | $0.18  | $3.08  | ~30% cheaper    |
| Sail Balanced   | $0.50 | $0.12  | $2.50  | ~43-64% cheaper |
| Sail Flex       | $0.40 | $0.08  | $1.80  | ~59-71% cheaper |

Other Sail models worth knowing: DeepSeek V4 Flash ($0.05 / $0.01 / $0.09
Flex) undercuts even GLM-Flash; gpt-oss-120b is $0.06 / $0.03 / $0.40.

Reading notes:

- Sail's cheapest tier wins on every axis, including the lowest-latency
  ASAP window — this is not a "cheap but slow" tradeoff at the base tier.
- Output is where the delta is largest, and agent runs are output-heavy
  (tool-call loops), so realized savings should land near the upper end.
- **Billing-mode caveat:** if the pool's Z.ai usage rides a GLM Coding Plan
  subscription (flat rate + quota — which would explain the 429 storms)
  rather than pay-as-you-go, this list-price comparison understates Z.ai's
  effective cost but also means Sail is a _capacity_ add, not just a
  cheaper one. Unknown; owner to confirm which billing mode applies.

## Constraints and open questions

- **No Go SDK** (Python/TS/Rust only). Plain HTTPS fits the pure-Go dep
  rule; tier-1 (Crush provider) needs no Go code at all.
- **Sailbox executor is an ADR-worthy fork**: derived outcomes
  (`executor.GitLogScanner`, `git diff-tree`, go-crush-data session usage)
  assume local repos; `.tq-verify` gates assume the host/flake env; the
  `Task-Queue-ID` footer and commit-export path (push from sandbox vs
  bundle back) all need deliberate design. Do not treat tier 5 as
  incremental.
- **`rateLimitGates` keying** (`internal/executor/agent.go`) is per repo
  because provider is fixed per repo `.crushrc`. Failover routing (Z.ai
  429 → re-dispatch via Sail) needs that keying revisited — provider would
  vary per attempt, not per repo.
- **New rate-limit shape:** Sail's 503 can arrive AFTER acceptance
  (in-flight capacity shortage) — different from a pre-flight 429 refusal.
  Current detection handles 429 + Retry-After/reset-timestamp shapes; this
  one needs its own pattern if we route through a direct-API executor
  (through Crush, the client-side retry is Crush's problem).
- **Webhooks need a reachable endpoint**; `tq serve` is loopback-only by
  security model. Poll the retrieve endpoints instead (the papdashboard
  AnswerPoller is the in-repo precedent). Background results expire 10 min
  after first delivery — fine for lease+heartbeat workers that poll.
- **Third-party drift:** Sail is young; models, prices, and limits will
  move. Re-verify the pricing section before acting on this doc.

## Cheapest first experiment (when taken up)

Add Sail as a Crush provider in ONE repo's `.crushrc` (managed block
re-render via `tq bootstrap` or a reviewed manual edit), run a few agent
tasks on GLM-5.3-Flash through Sail while the Z.ai gate is closed, and
compare cost + quality using the existing session forensics
(`go-crush-data` usage + verdict channel). Zero Go changes; failover
automation, usage-API budgeting, and anything Sailbox stay behind separate
decisions.

## Verification status

| Claim                                                                                                                                | Status        | Source                                                                              |
| ------------------------------------------------------------------------------------------------------------------------------------ | ------------- | ----------------------------------------------------------------------------------- |
| Sail serves GLM-5.3 / GLM-5.3-Flash; prices as tabulated; ASAP/Balanced/Flex windows                                                 | ✅ Verified   | docs.sailresearch.com/pricing.md + /models.md (raw fetch, 2026-09-18)               |
| GLM-5.3 on Sail rejects effort `none` (HTTP 400), `low` lowest tier                                                                  | ✅ Verified   | models.md reasoning footnote (2026-09-18)                                           |
| Sail 429/503 carry standard `Retry-After`; 503 possible after acceptance                                                             | ✅ Verified   | docs.sailresearch.com/rate-limits.md (2026-09-18)                                   |
| Supercache / idempotency / batch / background+webhooks / Usage API / implicit prefix caching + `prompt_cache_key` exist as described | ✅ Verified   | docs.sailresearch.com llms.txt index + pricing notes (2026-09-18)                   |
| SDKs: Python/TypeScript/Rust only, no Go                                                                                             | ✅ Verified   | llms.txt SDK reference pages (2026-09-18)                                           |
| Z.ai list prices as tabulated                                                                                                        | ✅ Verified   | docs.z.ai/guides/overview/pricing (raw fetch, 2026-09-18)                           |
| `DetectRateLimit` / `rateLimitGates` locations and semantics                                                                         | ✅ Verified   | in-repo grep: internal/executor/ratelimit.go:128, agent.go:131 (2026-09-18)         |
| Supercache savings magnitude for tq's prompt mix                                                                                     | ❌ Unverified | No numbers in Sail docs; framed as "aimed at this pattern", not a savings claim     |
| Flex-window latency cost in practice                                                                                                 | ❌ Unverified | Docs state the tradeoff, no SLA/latency numbers; treat Flex suitability as untested |
| Pool's Z.ai billing mode (PAYG vs Coding Plan)                                                                                       | ❓ Unknown    | Not asserted anywhere in this doc's conclusions; owner to confirm                   |
