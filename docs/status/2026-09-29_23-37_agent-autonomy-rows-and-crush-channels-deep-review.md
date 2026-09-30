# Agent-autonomy rows mint + crush Claude-Channels deep review (interactive owner session)

**Session shape**: interactive owner session (no pool task ID) — two directives
executed: (1) mint the owner's three-line "our prompts still suck" directive
into TODO_LIST.md as pool-food rows; (2) DEEP REVIEW of
charmbracelet/crush#3346 after the owner flagged it as "maybe also very
helpful for us". Everything below is session-scoped: no unrelated research.

**Timestamp discipline**: filename measured via `date` (2026-09-29 23:37:22
+0200), not projected. HEAD at report start 863b0660 (concurrent windows
actively shipping — the 22-37 security-batch and 23-28 archive-evidence
reports landed mid-session; their TODO edits were read, preserved, built
upon; nothing reverted).

## a) FULLY DONE

1. **Three directive rows minted** — new section
   `## Agent-autonomy overhaul (directive 2026-09-29: trust agents more,
   batch more per run, talk MCP)` at TODO_LIST.md:448-452:
   - prompt de-micromanage row (outcome contracts; derivation already
     verifies mechanics; pins updated same-change: `TestAgentPromptsDropSelfReport`
     family + harvest batch guardrail pins);
   - batch-default row (`DefaultBatchItems` calibration + budget-semantics
     explicitness + adjacent-section probes + batch_test pins);
   - `tq mcp` row (stdio MCP server, ≤6 read-mostly tools, ask parity,
     smoke + devmod gates, bootstrap registration recipe).
     Wording passed the `check-todo-list.sh` scan by construction (no
     unblocked-gated markers; gate rc=0 at mint, 257 unchecked rows counted).
2. **crush#3346 deep review complete** — 49 files, +2388/−91 read in full:
   delivery mechanics (`backend/channels.go` exactly-once router,
   `SubscribeChannelEvents` workspace-scoping, `channelTargetSession`
   ladder, `sessions.channel` binding lifecycle), deterministic reply-back
   (`channelreply.go`: group-over-user routing, `completedToolCalls`
   suppression, error-path reply excluding cancel, 30s detached-timeout,
   reply-before-busy-release), security posture (untrusted-payload
   escaping, per-channel tool scoping + cross-channel tool rejection,
   fail-closed gate, auto-discovery consent requirement), config surface
   (`channel_enabled` boolean + `channel_reply` schema.json routes), SSE
   wire (`proto.MCPEvent.ChannelMessage` round-trip pins), and the full
   andrinoff review thread with the fix commits (838c6fe7 tool-drop +
   direct-lookup + goroutine routing + debug logs; 3a3f2515 consent gate;
   0ddef59 channel-preferring target + ErrNoRows discrimination + error
   reply).
3. **Series + release forensics** — #3345 (wire contract: capability
   `capabilities.experimental["claude/channel"]`, `notifications/claude/channel`
   {content, meta}, `<channel>` XML render, fail-closed opt-in gate),
   #3401 (`--channels` CLI flag HIDDEN "until the rest of the feature is
   complete" — `channel_enabled` config boolean is the sanctioned opt-in),
   containment proven by compare API: merge 564d14e44 is behind v0.97.0
   (tag 0f2ed697b, behind_by=0) → the series ships in v0.97.0/v0.97.1,
   both published 2026-09-29.
4. **Host version verified, not recalled**: `crush version` → v0.96.1
   (probed 23:37 this session; the channels capability is NOT on this
   host's binary yet).
5. **Adoption assessment recorded durably** — the `tq mcp` row
   (TODO_LIST.md:458) now carries the SEE ALSO block: tq as a channel MCP
   server (push task prompts with meta={task_id, repo, attempt}, map one
   reply tool via `channel_reply` → tq verdict), with prereqs (crush
   ≥ v0.97.0; headless `crush serve` per repo = the standing
   client/server-mode-OFF experiment row, now with the strongest
   pro-experiment evidence yet; `channel_enabled: true` in the
   bootstrap-managed mcp block).
6. **Pool-fork check executed** — the row-text amendment could have forked
   an already-minted task (the reworded-duplicate class); probed the
   production journal read-only (`tq tasks --status pending --type agent
   --verify-contains` × 3, TQ_DB pointed consciously): **0 matching
   task(s)** for all three rows → amendment landed BEFORE first mint, no
   fork, next harvest mints the amended text fresh (rc=0 each probe).

## b) PARTIALLY DONE

1. **Channels adoption assessment** — the technical verdict is recorded
   (row 458) but the decision layer is open and NOT mine to close: crush
   upgrade ownership, the client/server-mode experiment go/no-go, and
   mcp-row sequencing all sit with the owner (§g). The row is pool-food;
   an executing window will hit those gates and must stop there.
2. **Part 3/3 of the series not located** — my merged-PR search surfaced
   #3345/#3346/#3401/#3348 only; whether the third part (the reference
   messaging-channel server) is an open PR, a separate repo, or
   unstarted was not established. It matters: that implementation is the
   best template for tq's channel-server side.
3. **Battery breadth** — cheap gates only (todo-list, status-index,
   doc-refs; rcs in §battery). check-ci/master-CI state not probed this
   session (the known consecutive-skip class; docs-only delta is the
   mitigation, not the excuse).

## c) NOT STARTED

1. The three rows' actual work: prompt de-micromanage, batch default
   flip, `tq mcp` server.
2. The channels chain: crush 0.97.x upgrade, channels-reference spec
   read (code.claude.com URL still unfetched — see §d3), `crush serve`
   per-repo pilot, bootstrap `channel_enabled`/`channel_reply` support.
3. AGENTS.md channels one-liner — deliberately deferred to the TODO row
   until a real 0.97 binary verifies the contract live on this host
   (avoid encoding PR-derived wire claims as host facts).

## d) TOTALLY FUCKED UP

Nothing catastrophic; three calibrated misses, all process-class:

1. **Re-hit the hot-file staleness lesson**: TODO_LIST.md edit was
   rejected twice on mod-time staleness (concurrent 22:37/22:47 windows);
   after the FIRST rejection I retried blind instead of re-reading — the
   codified rule ("re-read immediately before every write in hot files")
   was violated once before being followed. Cost: two round trips, zero
   damage (concurrent edits preserved byte-for-byte).
2. **Claim-before-probe on the crush version**: my first review answer
   cited "this host runs 0.96.1" from AGENTS.md memory; the one-second
   `crush --version` probe happened only at close-out (it confirmed the
   claim — but the citation preceded the measurement; the
   claims-carry-citations rule wants the probe first).
3. **Unfetched reference**: the channels-reference doc URL was carried
   from the PR body without fetching it (verify-external-claims class).
   Mitigating: every wire detail I actually recorded (capability key,
   notification name, render shape, config keys) was read from the
   merged diff — the shipped truth — not from the doc.

## e) WHAT WE SHOULD IMPROVE

1. **Probe-not-recall for environment facts** (`crush --version`,
   `tq version`, `git rev-parse`) at first mention in any session — one
   second each, converts memory-citations into measurements.
2. **Staleness retry discipline**: after ANY edit-tool staleness
   rejection, the next action is a read of the target region, never a
   blind retry — this session is a fresh receipt of a codified rule.
3. **External-contract verification bar**: before encoding a wire
   contract we intend to IMPLEMENT against (channels), fetch the spec
   AND diff-verify against the shipped binary's behavior; PR diffs are
   necessary, not sufficient, once we depend on the feature.

## f) NEXT (session-derived, grounded — not padded)

1. Owner: upgrade crush to v0.97.1 on this host (home-manager path is
   agent-blocked; unlocks channels for local experimentation).
2. Fetch + read the Claude Channels reference spec before implementing
   tq's channel side.
3. Locate part 3/3 of the series (open PR / separate repo) — the
   reference channel-server implementation is the template for tq.
4. Owner ruling: the client/server-mode experiment (standing OFF) —
   channels are the strongest pro-argument yet; pick the pilot repo.
5. Execute the prompt de-micromanage row (row 449) with pins.
6. Execute the batch-default row (row 450) with calibration + budget
   math.
7. Execute the `tq mcp` row (row 458) — sequencing question in §g.
8. Design the tq channel-meta schema (task_id/repo/attempt/dedup_key/
   batch) incl. XML-escaping rules for arbitrary prompt text.
9. `tq bootstrap`: managed `.crushrc` mcp block gains `channel_enabled`
   - `channel_reply` emission once crush ≥ 0.97 is the floor (schema
     known from this review).
10. Question-flow channel pivot design (row for it does not exist yet):
    AnswerPoller pushes the answer into the parked session as a
    `<channel>` message instead of NotBefore expiry → full re-claim →
    re-run.
11. Batching-via-queued-pushes evaluation: channels' serialized dispatch
    (prompts queue per session) vs `--batch-items` prompt gymnastics —
    two roads to the same directive; decide which owns the default.
12. Interplay notes to file when the channel design starts: per-repo
    exclusivity vs exactly-once push (push only while the claim is
    held); `--task-timeout` vs busy-queue wait; reply-suppression ≈
    TQ_RESULT_FILE-outranks-stdout (same dedup idea, two codebases).
13. Doctor: raise the crush floor + add a channels-capability probe
    once channels become load-bearing for the pool.
14. Watch upstream for #3401 reversal (`--channels` un-hide) —
    detection could ride the doctor crush check.
15. Harvest sanity after the next pool tick: the three rows mint exactly
    once, amended text (no dup from the pre-mint amendment — verified 0
    minted at 23:37).
16. PapDashboard-as-channel idea (noticed in-passing; alerts pushed into
    sessions) — file only if the owner wants the pattern generalized.
17. AGENTS.md channels one-liner after live 0.97 verification (per §c3).
18. Session-behavior items from §e belong to the standing
    session-start/battery rows (23-28 §f already rows the check-ci skip
    class; nothing new to mint).

## g) QUESTIONS (cannot self-answer)

1. **Crush upgrade**: v0.97.x install on this host is owner-run
   (read-only home-manager store for agents) — when do you want it, and
   is it a plain version-bump input flip on your side?
2. **Serve-per-repo experiment**: do you green-light the standing
   client/server-mode-OFF experiment now that channels give it a
   production-shaped payoff — and on which repo first (this dogfood
   repo, or a scratch repo)?
3. **`tq mcp` sequencing**: read-only tools first (works with the
   current `crush run` fleet, no serve needed, cheap) with the channel
   contract as phase 2 — or straight to the channel server (bigger,
   needs serve-per-repo, maximally aligned with the directive)? This is
   a spend/scope call, not a technical one.

## Battery (cheap gates, rc-captured)

- `./scripts/check-todo-list.sh` — rc=0 ×3 (after mint, after concurrent
  window landed, after amendment).
- Post-artifact battery (this report + index row written, rcs redirected to
  files per the PIPESTATUS rule): `check-todo-list.sh` rc=0,
  `check-status-index.sh` rc=0 ("status index ok"; its 327-live-rows BLOAT
  WARNING is the known pre-existing advisory — rows 158/179 own the next
  archive sweep, not this session), `check-doc-refs.sh` rc=0.

— interactive session; the auto-commit daemon carries this report + the
index row (both written together, so the index gate sees them atomically).
