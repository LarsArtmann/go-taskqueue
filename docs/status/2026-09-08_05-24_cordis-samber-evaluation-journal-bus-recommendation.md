# Status Report — Library Evaluation Session (cordis / samber-do / samber-ro) and the Journal-Bus Recommendation

**Date:** 2026-09-08 05:24
**Session type:** Advisory / architecture exploration. **Zero code changes, zero commits authored by this session.**
**Conversation-start HEAD:** `b1fd2c0` → **HEAD at report time:** `e19767d` (master moved underneath us mid-session — see d3).

## What this session did (chronology)

1. **Q: How could go-taskqueue benefit from cordis?** — Explored `/home/lars/forks/cordis` (README, AGENTS.md, `go/README.md`), grounded the mapping in repo facts (`cmd/tq/main.go` = 1,192 lines, 5× `signal.NotifyContext`, ~15× `defer s.Close()` via grep; `internal/webui/hub.go` = 57 lines via `wc -l`). Verdict: steal the patterns now (fiber-style per-component contexts, LIFO disposal), adopt the dependency only at the daemon/plugin-API horizon; flagged that the drain-deadline invariant (`task ctx survives pool shutdown`) maps _badly_ onto fiber `StdContext` cancellation.
2. **Q: samber/ro vs samber/do?** — Loaded `samber-do-best-practices` skill (its verification block: do v2 API confirmed via pkg.go.dev). Fetched pkg.go.dev for `samber/ro` (verified: module `github.com/samber/ro`, v0.4.1 published 2026-08-23, Apache-2.0, 47 importers, ReactiveX-style Observable/Subject/operator surface). Delivered the complementary-layers comparison (do builds the machine, ro is the conveyor belts).
3. **Q: Matrix for all 3** — Delivered the 13-dimension matrix (paradigm, reactivity, lifecycle, events, time, cancellation, partial-failure safety, typing, isolation, maturity, adoption risk, taskqueue fit). Verdict: do+ro ≈ 80% of cordis's value as two small libraries; cordis's unique 20% is lifecycle reactivity.
4. **Q: What is the BEST we could do?** — First-principles answer: (1) **journal as the bus** — one typed `Subscribe(ctx, since Seq)` fact stream + persisted per-bridge watermarks (kills the bridge missed-incident gap; no framework fixes a _cross-process_ gap); (2) **run.Group-style actor composition root** for `cmd/tq` (one interrupt story, deterministic teardown, `executionScope` makes the drain invariant structural); (3) defer the framework decision to the plugin era.
5. **Q: This report.**

---

## a) FULLY DONE

- **cordis API surface mapped from primary sources** — local fork README + AGENTS.md + `go/README.md`: fibers/LIFO rollback, typed services/events, inject reactivity, isolation realms, `Batch`, `fiber.StdContext()`, Go flagship port status. Evidence: read in-session; no fabrication.
- **samber/ro existence + API shape verified against pkg.go.dev** (per the `verify-external-claims` primary-source rule): module path, v0.4.1, Apache-2.0, 47 importers, example-index confirms Observable/Subject/operator surface incl. `CombineLatest`, `Retry`, `Share`, `FromChannel`/`ToChannel`, context variants.
- **samber/do facts drawn from the loaded skill** whose own verification block confirms the API at pkg.go.dev (`Provide`/`Invoke`/scopes/lifecycle interfaces).
- **Repo-side grounding for the recommendation**: `cmd/tq/main.go` wiring counts (grep: 5 signal sites, 15+ `defer Close` across 11 commands), `hub.go` size, executor/bridge/package map from AGENTS.md.
- **All four questions answered with explicit tradeoffs and a named recommendation** (no wishy-washy "it depends" endings).

Caveat that belongs here honestly: until this report, all of the above existed **only in chat** — nothing was persisted to `docs/` before now.

## b) PARTIALLY DONE

- **The "BEST" architecture recommendation** — delivered as design prose + signature sketches (`journal.Subscribe`, `run.Group` actor list, `executionScope`). Gap: not compiled, not spiked, **and designed without reading `internal/journal`** (see d1). Effort to make real: M per component (see f).
- **cordis maturity assessment** — "~85% coverage, race-tested" is _cordis's own README claim_; I never ran `cd go && go test ./...` (one command, the fork is local). Effort to finish: S.
- **samber/ro evaluation** — stopped at API-index level; no runnable spike of an `Observable[T]` over a fact stream, no backpressure ergonomics check against Go channels. Effort: M (spike).
- **Comparison matrix** — delivered, but contains three unverified/unlabeled claims (see d2) and was written against a repo snapshot that has since moved (see d3).

## c) NOT STARTED (all implementation work; nothing was requested this session)

- `journal.Subscribe(ctx, since Seq)` typed fact-stream API — not designed against the real Journal interface, not implemented. Still wanted: yes, it is the spine of the recommendation.
- Persisted per-bridge watermarks (the actual fix for the known papdashboard missed-incident gap). TODO_LIST history shows the bridge already has a _volatile head-watermark_ (`startWatermark`, fixed once on 2026-09-07) — persistence is the missing half.
- run.Group actor composition root for `cmd/tq`; `executionScope` actor; single interrupt story.
- `tq daemon` mode (serve + agent-pool + bridges + harvest in one process) — horizon item, needs an owner go/no-go (g1).
- Plugin API for executors/bridges; any cordis adoption — horizon, gated on g2.
- SSE `Last-Event-ID` ↔ journal `Seq` client-resume mapping — natural sibling of bridge watermarks, untouched.

## d) TOTALLY FUCKED UP

- **d1 — Designed an API for a package I never opened.** The centerpiece recommendation (`journal.Subscribe(ctx, since Seq)`) was written without reading `internal/journal`'s Journal interface, the webui `tailer.go`, or `internal/bridge/papdashboard/papdashboard.go`. Everything I "knew" about the subscription landscape was secondhand from AGENTS.md. The existing subscription/tailer mechanism may already cover half the proposal; the single-serialized-writer invariant makes dispatcher design subtler than my sketch implied. Severity: medium (a blind recommendation could misdirect a week of work). Mitigation: harvest item H1 below.
- **d2 — `verify-external-claims` chat-gate violation: loaded ≠ applied.** Three claims crossed the matrix without labels: (1) "~200 operators" for ro — an _estimate from the example-index length_ presented as a count; (2) cordis "race-tested ~85% cov" — vendor self-claim, never run; (3) the ro GitHub fetch returned page chrome only and I proceeded on pkg.go.dev alone without noting the degraded source until now. Severity: low (advisory context, all three plausibly correct) — but this is the skill's _documented_ failure mode, executed while the skill sat loaded.
- **d3 — Answered against a stale repo.** AGENTS.md says "git pull your assumptions." The session opened at `b1fd2c0`; master is now `e19767d`. Of direct relevance: `2d5e729` "perf(queue): bound every journal read so tick cost stops growing with history" **touches the exact code the fact-stream proposal lives in** (does read-bounding interact with replay-from-seq?), and `4cb32f6` added token auth to `tq serve` (my ADR-0003 framing was written as if serve were still auth-less). No harm materialized because I wrote no code — but the matrix and BEST answer were not re-verified against HEAD. Severity: low here, high as a habit.
- **d4 — Secondhand characterizations.** `hub.go` ("bespoke pub/sub"), the tailer, and the bridges were never opened; sizes came from `wc -l`. Acceptable for advisory triage, dishonest if read as code review.

## e) WHAT WE SHOULD IMPROVE

1. **Interface-first rule for API proposals:** never propose an API for a package without `view`ing its interface first. This is now the second recorded class of "recommended-first, verified-later" (the 2026-08-21 report had the same shape per the skill's §0 note). If it repeats, it should become a hard hook/checklist item.
2. **Run the target's tests when maturity is load-bearing** — cordis lives locally; `go test ./...` was one command away and would have upgraded a vendor claim to a verified one.
3. **Label estimates in-line** ("~200 operators (estimated from example index)") — specificity is not evidence, and unlabeled estimates read as counts.
4. **Re-verify freshness when a session spans git activity** — `git log --oneline -3` at answer time, not at session start, whenever the answer makes claims about current code.
5. **Persist load-bearing analysis immediately** — a 4-turn architecture comparison lived only in chat; had the session died, the verdict would have evaporated. This report is the fix, but the pattern (write the ADR note _at decision time_) is better.

## f) Next tasks (brainstorm, 50 — curated five harvested to TODO_LIST, rest is ROADMAP fuel; do NOT mass-harvest: TODO_LIST.md is live dogfood-pool food)

**Verify & ground (from this session's own gaps)**

1. Inventory `internal/journal` + webui `tailer.go` subscription surface; document gaps vs proposed `Subscribe(ctx, since Seq)` — Impact High / S / Quality
2. Read `internal/bridge/papdashboard` watermark code; enumerate exactly what a persisted cursor needs — High / S / Feature
3. Check `2d5e729` (bounded journal reads) for interaction with replay-from-seq — High / S / Quality
4. Run cordis Go test suite (`GOCACHE=/tmp/gocache`); record coverage/race as the real input to the plugin-era decision — Medium / S / Quality
5. Correct the matrix: count ro operators, restate cordis coverage as verified-or-vendor — Low / S / Documentation

**Fact-stream (journal as the bus)**
6. Design `Subscribe` contract: buffering, slow-consumer policy, cancel semantics — High / M / Feature
7. Decide slow-consumer semantics: block / drop / per-subscriber ring + lag signal — Medium / M / Feature
8. Fan-out dispatcher that never blocks the single serialized writer (dispatch outside the write tx) — High / M / Feature
9. Persisted watermark storage: side table vs metadata-fact (projection purity) — High / S / Design
10. Migrate papdashboard bridge to resume-from-watermark — High / M / Feature
11. Migrate cqa bridge likewise — Medium / M / Feature
12. `hub.go` → thin adapter over the subscription (keep SSE wire format) — Medium / S / Refactor
13. Budget projections subscribe instead of re-query per tick (verify applicable first) — Medium / M / Feature
14. `tq tail --since <seq>` replay flag — Low / S / Feature
15. Test: bridge restart loses zero facts and sends remain idempotent — High / M / Test
16. Subscriber-lag metric/gauge per consumer — Low / M / Observability
17. Soak/fuzz the dispatcher under slow consumers — Medium / M / Test
18. Bench: N-subscriber fan-out vs current per-consumer polling — Medium / M / Quality

**Lifecycle actors (run.Group pattern)**
19. Actor helper (stdlib + errgroup; no framework) — Medium / S / Feature
20. Pilot on `tq serve`: interrupt + store + http actors — Medium / M / Refactor
21. Extend to `worker`/`agent-pool` — Medium / M / Refactor
22. `executionScope`: detached in-flight-task scope bounded by `--task-timeout`, enforced structurally (known bug class — the stranded-in-`running` lesson) — High / M / Feature
23. Shutdown-ordering test: SIGTERM → SSE closed → bridges flush → pool waits execution scope → store closed — High / M / Test
24. Collapse the 5 `signal.NotifyContext` sites into one story (verification item for 19–21) — Medium / S / Cleanup
25. Document the actor pattern in AGENTS.md once piloted — Low / S / Documentation

**Daemon horizon (needs g1 first)**
26. ADR: `tq daemon` = serve + agent-pool + bridges + harvest in one process — High / M / Documentation
27. Per-project isolation design if daemon lands — Medium / L / Feature
28. In-process bridges vs HTTP polling tradeoff note — Low / S / Documentation
29. Aggregate healthcheck endpoint — Medium / S / Feature
30. Graceful-reload story (SIGHUP) — only if daemon is approved — Medium / M / Feature
31. Bridge uptime/lag alerting (pap alert about the pap bridge) — Low / M / Feature

**Plugin era (needs g2 first)**
32. Executor plugin API surface: typed config, verify command, lifecycle hooks — framework-independent — High / L / Feature
33. Bridge plugin API likewise — Medium / M / Feature
34. ADR: framework-free registry vs cordis adoption _criteria_ (trigger conditions, exit plan) — Medium / S / Documentation
35. Cordis integration-cost prototype behind a build tag — Low / L / Spike
36. Track cordis v1 as the adoption gate — Low / S / Process
37. samber/ro spike: `Observable[T]` over the fact stream vs plain channels (backpressure, ops ergonomics) — Medium / M / Spike

**Session hygiene / docs**
38. Distill the do/ro/cordis verdict into `docs/adr/` (lifecycle & streaming library stance) so it survives chat — Medium / S / Documentation
39. ROADMAP: add daemon mode + plugin API horizons if absent — Low / S / Documentation
40. SSE `Last-Event-ID` ↔ journal Seq client-resume mapping (client-side twin of bridge watermarks) — Medium / M / Feature
41. `tq watermarks show/set` ops command for bridge rescue — Low / S / Feature
42. Docs/examples for the Subscribe API — Low / S / Documentation
43. Pre-answer freshness check in long sessions (process rule from e4) — Process
44. Interface-first rule for API proposals (process rule from e1/d1) — Process
45. Re-run this library evaluation when cordis or ro hits v1 — Low / S / Process
46. Note in AGENTS.md that serve now has token auth for non-loopback binds (the doc still describes loopback-default as the whole story) — Low / S / Documentation
47. Verify whether `errgroup` is already a dependency before recommending the actor helper — Low / S / Quality
48. Consider whether budget caps should also gate bridge sends (cost symmetry) — Low / S / Design
49. `docs/DOMAIN_LANGUAGE.md`: add "watermark", "subscriber", "lag" if the fact-stream lands — Low / S / Documentation
50. Meta: add "license + count claims" worked examples to the verify-external-claims references (this session's d2 misses) — Low / S / Process

## g) Questions I cannot answer myself

1. **Is `tq daemon` (serve + agent-pool + bridges + harvest in one long-lived process) an intended milestone or deliberately out of scope?** `docs/planning/` hints at autonomy/pacing work, but the product go/no-go — and whether it must wait for v0.2/v0.3 — is yours. Roughly half the f-list (26–31, 40–41) is gated on this.
2. **Framework stance for the plugin era:** when a third-party executor/bridge API arrives, is adopting cordis as the lifecycle contract ever acceptable given the "composes with both, depends on neither" philosophy, or must go-taskqueue stay framework-free forever? This decides whether f34/35 are planning or waste.
3. **Operational weight of the papdashboard bridge:** is the missed-incident gap (bridge down = incidents never replayed) causing real pain in your ops today, or is the bridge not yet load-bearing enough to prioritize persisted watermarks over other High-impact work?

---

_**ANNOTATION (2026-09-09):** the journal bus recommendation LANDED as
`internal/consumer` + `internal/runactor` (ADR-0009; papdashboard bridge and
both sweepers ride the watermark-cursor pattern — the "persisted watermarks"
this report flagged as future work SHIPPED). g3 is therefore resolved; g1
(daemon mode) and g2 (framework stance) remain owner decisions, routed as
ROADMAP open questions R1/R2._

**Harvest note:** only H1/H2-class verification items and the ADR task were pulled into TODO_LIST.md (5 items, citing this report). The remaining ~45 are ROADMAP fuel by design — `docs-health` HARVEST should route them with rigor, not mass-dump them into live pool food.
