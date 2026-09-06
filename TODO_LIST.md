# TODO List

Short- and mid-term actionable work. Long-term direction lives in ROADMAP.md.
This file is the agent pool's food source: `tq harvest` turns every unchecked
item below into an agent task (see README → The Agent Pool).

## Agent pool hardening

- [ ] Store-level per-project claim exclusivity (opt-in `WithProjectExclusivity` on the SQLite store): at most one running task per project across ALL worker processes, so multi-pool deployments get per-repo serialization without relying on harvester pacing alone
- [ ] Pass a model override through `tq agent-pool --model` into agent payloads (AgentPayload.Model exists; harvest/CLI wiring does not)
- [ ] Per-project concurrency limits and cost budgets (agent tasks cost real money; cap burn per repo per day)
- [ ] Harvester: poll interval per repo, and back off repos whose items repeatedly land in the DLQ (a poisoned repo should not refill its attempt budget forever)

## Observability

- [ ] `tq top`: live per-project view (pending/running/done/dead + last agent run duration) over the existing facts
- [ ] Record agent transcript location (crush session id) as task result detail on completion, so `tq show` links to the agent's session
- [ ] Surfacing "completed but item still unchecked" docs-drift: a periodic audit task that re-checks harvested repos and enqueues a docs catch-up item

## Quality

- [ ] `TestShutdownDrains` 30 ms claim window flakes under heavy parallel-agent load (10/10 green in isolation); widen the window or await claims explicitly instead of sleeping
- [ ] E2E CLI test: spawn `tq agent-pool` as a subprocess with a stub agent binary ($TQ_AGENT_BIN) and assert the full loop from outside the process
- [ ] Property test: harvest dedup keys are stable under whitespace reflow and unique across repos with identical item text
