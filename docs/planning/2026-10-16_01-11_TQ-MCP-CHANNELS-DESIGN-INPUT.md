# `tq mcp` design input — crush Claude Channels wire contract, verified 2026-10-16

**Status**: research DONE (closes the TODO row minted 2026-09-29 23-37 §f2/§f3,
"Research the crush Claude Channels wire contract … as the design input for
`tq mcp`'s channel-server shape"). The build rows re-minted in TODO_LIST.md
(`## Agent-autonomy overhaul`) own the implementation; this note owns the
verified contract and the design shape.

**Why now**: the 23-37 review was blocked on crush ≥ v0.97.0; this host runs
**v0.98.1** (probed `crush --version` 2026-10-16 01:0x) and `crush server`
exists as a first-class subcommand (`-H --host`, default
`unix:///run/user/1000/crush-1000.sock`). Both channels prereqs that were
owner-blocked in September are now technically unblocked.

## Sources (verified, not recalled)

- Host: `crush --version` → v0.98.1; `crush --help` → `server` subcommand +
  `--host` flag (2026-10-16 probe).
- Wire contract read from the shipped implementation (charmbracelet/crush,
  main, fetched 2026-10-16 via GitHub API):
  - `internal/agent/tools/mcp/channel.go` — capability key, notification
    method, payload limits, parse/render, negotiation gate, opt-in logic.
  - `schema.json` — `channel_enabled`, `channel_reply` (`MCPChannelReply`,
    `MCPChannelReplyRoute`), `message_param` default.
  - Implementation file census (git/trees): `internal/agent/channel.go`,
    `internal/agent/channelreply.go`, `internal/backend/channels.go`
    (exactly-once workspace router), `internal/db/migrations/20260921000000_add_channel_to_sessions.sql`.
- Series-merge placement (merge 564d14e44 behind v0.97.0, published
  2026-09-29) was verified by compare API in the 23-37 review §a3; v0.98.1 ⊇
  v0.97.0.
- **Not yet verified**: the v0.98.1 host binary's channels behavior live
  (source read at `main`, not the tag diff; no session has declared the
  capability on this host). The pilot row owns that probe — do not encode
  channel behavior as host fact until then (23-37 §c3 rule).

## The wire contract (as shipped)

tq plays the **channel server** role; crush is the client:

| Element          | Value                                                                                                                                                                                                                                                                            |
| ---------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Capability       | `initialize` result: `capabilities.experimental["claude/channel"] = {}` — presence alone turns on the client's notification listener                                                                                                                                             |
| Push             | Server emits JSON-RPC notification `notifications/claude/channel`, params `{content: string, meta: map[string]string}`                                                                                                                                                           |
| Content limits   | non-empty, ≤ 64 KiB; oversized → notification dropped (never truncated)                                                                                                                                                                                                          |
| Meta limits      | ≤ 32 entries; values ≤ 1024 bytes; keys MUST match `^[A-Za-z_][A-Za-z0-9_]*$`; reserved keys dropped: `source`, `xmlns`, `xml`                                                                                                                                                   |
| Validation       | fail-closed, `DisallowUnknownFields`; any structural problem drops the whole notification                                                                                                                                                                                        |
| Render           | client wraps as `<channel source="<server-name>" k="v" …>content</channel>` (keys sorted, values XML-escaped) and injects into the session's next turn                                                                                                                           |
| Negotiation gate | notifications arriving between Connect and gate resolution are buffered, then drained (open) or discarded (closed) — a server may push immediately after initialize                                                                                                              |
| Opt-in           | `channel_enabled: true` per-server in crush.json mcp block (persisted) OR hidden `--channels server:<name>` flag (per-launch); "listed is not enabled" — both sources ORed at every gate site (`ChannelOptIn`)                                                                   |
| Reply routing    | `channel_reply: {user: {tool, target_param}, group: {tool, target_param}, message_param, suppress_tools}` — replies to channel-originating turns are routed back through ONE of the server's tools; `message_param` names the arg receiving the reply text (default `"message"`) |
| Exactly-once     | server-mode crush (`backend/channels.go`) routes per workspace with must-deliver semantics; busy sessions queue pushes instead of interleaving                                                                                                                                   |

Part 3/3 of the series (the reference channel-server implementation the
23-37 review could not locate) resolved as: there is no separate reference
PR in the crush repo — the implementation files listed above ARE the
template, and the upstream doc pinned in the source comment is
`https://code.claude.com/docs/en/channels-reference` (authoritative spec).

## tq-side design

### Phase 1 — `tq mcp` stdio server, read-mostly (build row 1)

Works with today's `crush run` fleet; no serve experiment needed. Hand-rolled
JSON-RPC 2.0 over stdio (initialize / tools-list / tools-call +
notifications/initialized; no new deps — pure-Go rule holds). ≤ 6 tools:

1. `task_list` — status/type/limit filters → compact JSON
2. `task_show` — id → record + fact trail (`tq show` surface)
3. `queue_stats` — `tq stats` surface
4. `dlq_list` — dead letters
5. `park_task` — write tool: park a PENDING task (`tq park` parity: explicit
   task_id + duration + reason, running refused) — the MCP process is NOT
   inside a task run, so this is the `tq park` verb over MCP, not the
   run-internal `$TQ_QUESTION_FILE` channel

What this kills, per the original directive: the argv-quoting class (JSON
params, not shell lines) and the ambient-`TQ_DB` hazard class (the server is
launched by crush from the bootstrap-managed mcp block with an EXPLICIT
`--db` flag — configuration, not inherited shell env). Agent sessions stop
burning shell-`tq` round-trips for every read.

### Phase 2 — channels shape (build row 2)

Same server, extended: declare `claude/channel`, and instead of (or in
addition to) pull-tools, PUSH work:

- Claim-due tasks push `notifications/claude/channel`:
  `content` = the work prompt; `meta` = channel-meta schema below.
- `channel_reply` routes the session's reply through ONE tq tool
  (`verdict`, `message_param: "verdict_json"`) — replacing the
  `$TQ_RESULT_FILE` scrape for channel-originated turns.
- Prereqs: headless `crush serve` per repo (the standing client/server-mode
  experiment — owner go/no-go), `tq bootstrap` emitting `channel_enabled:
  true` + the `channel_reply` block once the serve pilot passes, doctor
  channels-capability probe, `--task-timeout` vs busy-queue interplay
  (23-37 §f12).

**Channel-meta schema draft** (all keys pass `metaKeyPattern`; all values
≤ 1 KiB): `task_id`, `repo`, `attempt`, `type`, `dedup_key` (batched items
only), `batch` (`"i/n"`, batched only). `content` carries the prompt
verbatim — the client XML-escapes it; prompts need no escaping on the tq
side.

### Sequencing recommendation

Phase 1 first (the 23-37 §g3 recommendation, now reinforced): read tools pay
off TODAY for every dispatched and interactive session; channels need the
serve-per-repo experiment and a live capability probe first. Phase 2 is a
strict extension of the same binary, not a rewrite.

## Open owner gates

1. Phase 2's `crush serve` pilot: which repo first (this dogfood repo vs
   scratch), and the standing client/server-mode experiment go/no-go.
2. Whether `park_task` (phase 1 write tool) ships in v1 or waits — it is
   the only non-read surface.
