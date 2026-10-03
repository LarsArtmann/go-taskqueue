# Paperclip vs go-taskqueue — competitive assessment

**Date:** 2026-10-03
**Status:** assessment only. No code, no config, no adoption decision.
Researched from live sources same day: paperclip.ing (home, /product/,
/product/{tasks,heartbeats,budgets,open-source}/, changelog
v2026.1001.0), docs.paperclip.ing (installation + full nav tree), and the
GitHub API (`repos/paperclipai/paperclip`). Every number below is from
those fetches; nothing is from memory or third-party summaries.

## What Paperclip is

Paperclip Labs, Inc. ships "the app people use to manage AI agents for
work" — an open-source (MIT, TypeScript) agentic-company platform: a
browser app where a CEO-style human runs an org chart of humans and AI
agents through issues, goals, approvals, and budgets. Positioning in one
line, from their own testimonials: "OpenClaw is an employee, Paperclip is
the company."

Repo reality (GitHub API, 2026-10-03):

| Signal          | Value                                              |
| --------------- | -------------------------------------------------- |
| Created         | 2026-03-02 (7 months old)                          |
| Stars / forks   | 96,586 / 16,364 (site copy still says "74k+")      |
| Open issues     | 6,318                                              |
| Language        | TypeScript                                         |
| License         | MIT                                                |
| Release cadence | weekly-ish (v2026.916.1 → v2026.1001.0, 77 commits from 8 contributors) |

Deployment shape: `git clone` + `docker compose up`, PostgreSQL behind it
(drizzle migrations, numbered `0280`+), server + browser + WebSocket
"company socket", optional hosted Cloud. Docs surface: 56-connector
catalog, 21 agent adapters (Claude Code, Codex, Gemini CLI, Cursor,
OpenCode, Grok, Kimi…), sandbox providers (Daytona, Modal, CreateOS),
MCP servers + aggregators, a plugin SDK, a 35-section CLI, and a
30-section REST API.

Product pillars (their own four-way split):

1. **Agentic Task Manager** — tasks with owners, review gates, routines,
   "verify from diffs, screenshots & tests".
2. **Org Chart for Agents** — mixed human+agent org, roles, governance,
   scoped secrets.
3. **Agent Employee Training** — skills studio, evals, active learning,
   "performance reviews for agents".
4. **Agentic OS** — cross-provider runtime, sandboxing, SSO/GRC/RBAC,
   cost controls, self-hosted.

## Side by side

Capability-level, honest. "Ahead/behind" is against *their product
claims*, not their marketing tone.

| Capability              | Paperclip                                                     | go-taskqueue                                                       | Verdict        |
| ----------------------- | ------------------------------------------------------------- | ------------------------------------------------------------------ | -------------- |
| Unit of work            | Issue with owner, thread, blocked-by, definition of done       | Task record: type, project, payload, priority, deps, attempt budget | Different level — they own a ticket app; we are file-native (TODO_LIST.md is the backlog) |
| Durability model        | Postgres app rows + activity log                              | Append-only fact journal; queue/DLQ/retry/budget are projections; replay (`tq facts`), watermark cursors | **Us** — kernel-grade, replayable, nothing deleted |
| Claim/concurrency       | Internal leases (changelog: stranded-lease sweeps exist)      | Lease TTL + heartbeat + expiry reclaim, `FOR UPDATE SKIP LOCKED` twin, exactly-once battery | **Us** — ours is documented, conformance-tested product surface |
| Planning/decomposition  | Plans fan out into blocked task trees; progress rolls up       | DAG deps gate at claim; no plan→tree minting                      | **Them** |
| "Done is a verdict"     | QA handoffs, evidence (tests/screenshots), review gates, PR review bots | Enforced `.tq-verify` gate ladder; outcomes *derived* from git (footer commits, diff-tree, session usage), never self-reported; review sweeper + autofix | **Philosophical tie** — same slogan, ours is repo-pinned and code-deep, theirs is broader (content, screenshots) |
| Autonomy/safety defaults | v2026.1001.0 defaults harnesses to FULL AUTO (approve-all, sandbox bypass); governance is top-down policy | Repo-committed `.crushrc` autonomy grant; the pool cannot over-grant what a repo never offered; dashboard read-only by default; API token-mandatory; redaction default ON | **Us, on direction** — their default is loosening; ours is bottom-up and tightening |
| Cost control            | Per-agent hard caps, per-task receipts, roll-ups by agent/goal/window, burn alerts | Daily enqueue cap, `--budget-cmd` veto, derived per-run session usage (tokens/cost), spend in `stats`/detail cards | **Them** — they enforce; we measure. (Our spend happens inside crush, so caps would be downstream lies) |
| Event-driven scheduling | Event wakes (assigned/commented/unblocked) + scheduled heartbeats + routine webhooks | Interval harvest, `--once` cron mode, `notBefore` delays; no inbound triggers | **Them** — and cheap for us to steal (see S1) |
| Watchdogs               | Stall/loop detection, auto-restart, escalate with stop point   | Lease-expiry reclaim, env-requeue circuit breaker, DLQ autopsies with structured failure evidence (stage, exit code, tail) | **Tie by different means** |
| Agent runtimes          | 21 adapters, BYOA, MCP, cloud sandboxes, execution workspaces | crush only, headless, deep integration (session resume for closeout, effort pinning, telemetry-derived contracts) | **Them on breadth, us on depth-of-one** |
| Org/multi-user          | Org chart, roles, delegation, SSO/RBAC, approvals inbox        | Single operator by design                                          | **Them** — and out of our scope (Non-goals) |
| UI                      | Full app: dashboards, chat, command palette, personas          | Read-only SSE dashboard (board/detail/journal browser) + CLI      | **Them** — ours is deliberately a projection |
| Embeddability           | None — an app; TypeScript; no library surface                 | 7 public Go facade modules over a conformance-tested `Store` contract; embeds in any Go program (`examples/embed`) | **Us, structurally** — nobody can embed Paperclip's queue semantics into their own program |
| Footprint               | docker compose + Postgres + Node                              | One static binary, one SQLite file, CGO-free, systemd/NixOS units | **Us** |
| Self-improvement loop   | Skills minted from completed tasks, evals, active learning     | Status loop writes next items back into TODO_LIST.md (pool feeds itself), review-autofix, dlqfix | Same spirit; theirs systematic, ours git-native |
| Maturity/scale          | Funded Inc., ~97k stars, weekly releases                      | v0.3.0 personal project, dogfooded daily                          | **Them** — not like-for-like |

## Claims vs observed

- "74k+ stars and counting" — the API says 96.6k. Stale copy, in their
  favor; harmless, but a marker for how the site is maintained.
- "Overruns are impossible, not just unlikely" (budgets page) — an
  absolute their own changelog contradicts in spirit: quota
  classification bugs surviving to adapter boundaries, cost fixes in
  nearly every release. Treat absolutes as tone, not spec.
- The v2026.1001.0 reliability section is the most informative read on
  the page: approval/Stop races, session continuity, sandbox
  reconnection, stranded leases. That is the tax of bridging
  *interactive* agent CLIs (Claude Code, Codex) into an app runtime. Our
  headless + verify-gate + derived-outcomes model does not carry that
  class of race by construction.
- 6,318 open issues + a no-migration retirement of the legacy Composio
  broker = velocity with churn. Connector breadth is a maintenance
  liability they can afford and we cannot.

## What to steal (scope-aligned)

- **S1 — Event-wake ingress.** Their headline "wakes, not waits" is, on
  our kernel, one endpoint: a token-gated webhook route on `tq api` that
  maps inbound events (a GitHub comment/PR/issue, or anything else) to
  dedup-keyed `enqueue`s. The queue already has every semantic the
  pattern needs (dedup, delay, priority bands); what is missing is the
  listener. This also natively yields their "agents review your pull
  requests" flow as a webhook-minted `review`-type task.
- **S2 — Bring-your-own-agent adapters.** "No agent tie-in either" is a
  top testimonial for a reason. Our `executor.Registry` is exactly that
  seam, but crush is the only shipped agent runtime. A `codex` and/or
  `claude-code` executor adapter (same AgentPayload contract, same
  verify gate, same derived-outcome rails) removes the tie-in without
  touching the kernel.
- **S3 — Cost receipts roll-up.** We already derive per-run session
  usage; Paperclip's budgets page shows the missing projection: spend
  aggregated per project and per time window (this week/last week),
  surfaced in `tq stats` and the dashboard. Measurement, not enforcement
  — consistent with our "spend lives in crush" stance. Adjacent ROADMAP
  seeds (`tq stats --fleet`, `tq top` budget spend) aggregate the
  *enqueue budget*; this is the *actual usage* twin.

## What NOT to steal (scope discipline)

- Org chart, roles, SSO, multi-user, approvals inbox — company software.
  Our Non-goals already fence this ("not a general-purpose workflow
  engine"); a queue that grows an org chart is a workflow engine with
  extra steps.
- The 56-connector catalog. We have a bridge seam (PapDashboard, CQA);
  a connector catalog is a support surface, and their 6.3k open issues
  show the bill.
- Cloud sandbox providers. Worktree-per-agent
  (`docs/planning/2026-09-12_worktree-per-agent-design.md`) is our
  isolation path — local, git-native, already designed.
- Chat-first interactive agent bridging. Their reliability changelog is
  the price list for that architecture; headless + verify avoids the
  class.

## Positioning

Paperclip is an application competing on breadth and polish for teams
that want to run "a company" in a browser. go-taskqueue is a kernel
competing on semantics and footprint for one operator (and Go
embedders) who want provable queue behavior in a single binary that
eats TODO_LIST.md and pays for itself in facts. They are not fighting
for the same buyer; where they overlap with us, the honest read is:
their planning UX, budgets, and event wakes are ahead; our durability
model, concurrency contract, autonomy direction, embeddability, and
footprint are ahead. The steals above close the gaps that are cheap on
our architecture and load-bearing on theirs; everything else is their
moat to maintain.

ROADMAP pointers: S1 and S2 live under Raw ideas (Queue core / scale,
Agent pool / loop); S3 folds into the Observability / ops budget line,
sourced here.
