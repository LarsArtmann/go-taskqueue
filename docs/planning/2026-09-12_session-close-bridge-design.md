# Session-close bridge — design + prototype

Status: PROTOTYPE shipped in this change (`tq session begin/close`), 2026-09-12.
Work item: TODO_LIST.md "session-close bridge design + prototype" (dogfood round,
harvested from docs/status/2026-09-10_02-00 self-review §f).

## Problem

The pool gives every agent task the full trust pipeline: claim exclusivity,
verify gate, a second-opinion review, and a periodic done-prompt status
report. INTERACTIVE crush sessions leave no task trail, so their commits skip
all of it — and since interactive sessions also run Flash, the old "interactive
= human-reviewed, pool = autonomous" trust asymmetry no longer holds. The
review pipeline exists precisely because unreviewed agent work lands on
master; interactive sessions are currently the biggest unreviewed writer.

Secondary gap: nothing in the journal explains that a session even happened.
The web UI and `tq facts` show pool work only.

## Trigger landscape (recap of the 2026-09-10 research)

crush PR charmbracelet/crush#3146 (SessionStart/SessionEnd/TurnEnd hooks) is
maintainer-approved, CI-green, users-on-branch; issue #2707 confirms it is THE
orchestrator/turn-done answer. SessionEnd has a ~2s timeout, so any hook
handler must ONLY enqueue — the pool does the rest. Ranking of interim
triggers until #3146 ships everywhere:

1. PreToolUse session registry (CRUSH_SESSION_ID + CRUSH_CWD on every tool
   call; sweeper mints the close-out when no crush process owns the ID).
2. `tq crush` wrapper on process exit.
3. Polling ~/.local/share/crush/crush.db (schema-private, migration-fragile).
4. crush.log tailing (fragile).

The prototype deliberately implements the SESSION SIDE only (`tq session
begin/close`): every trigger above reduces to calling these two commands, so
the trigger choice stays swappable without touching the bridge again.

## Contract

- `tq session begin --id <id> [--repo DIR] [--project P]` records a
  `session.opened` fact and prints the footer convention. `--id` defaults to
  $CRUSH_SESSION_ID. A double begin is refused (one open epoch per id).
- Attribution: a commit belongs to a session iff its message carries the git
  footer `Crush-Session: <id>` (same convention as Task-Queue-ID; committers
  are told to add it at begin time). Scan is ONE `git log` call with
  `%(trailers:key=Crush-Session,valueonly)` — key-scoped and value-exact, so
  quoting someone else's footer never mis-attributes (needs git ≥ 2.15).
- `tq session close --id <id> [--summary TEXT] [--allow-dirty]`:
  1. attributes the commits (oldest first),
  2. enqueues ONE `review` task over the range (dedup `review:session:<id>`,
     shared with the sweeper namespace) — Item is the operator summary (or an
     honest default), CommitSHA is the range head, Extra pins the full commit
     list and the `<oldest>^..<newest>` range instruction,
  3. enqueues ONE `status` task (dedup `status:<project>:session:<id>`,
     shared with the sweeper namespace) with a single StatusCompletion for
     the session,
  4. appends the `session.closed` fact carrying commits + minted task IDs.
- A session with zero attributed commits records only the closed fact and
  mints nothing — there is nothing to review and nothing to report.
- Both payloads run Yolo (the repo `.crushrc` is the autonomy carrier) and
  deliberately omit Model (a payload model resets reasoning effort; the
  `.crushrc` managed block is the only model+effort carrier).
- `--allow-dirty` mirrors the pool's stance into `RequireClean=false` on both
  payloads; a multi-agent repo is effectively always dirty, and a
  clean-tree-required mint there would requeue until DLQ.
- Close is enqueue-only and fast (one git call + three writes): inside the
  ~2s SessionEnd hook budget when #3146 lands.

## Fact design

`session.opened` / `session.closed` are observations, not task state. They
carry the synthetic TaskID `session:<id>`, which no real task (hex ULID) can
collide with, so:

- sweepers (react only to `task.completed`), budget projection (only
  `task.enqueued`), bridges (their own fact types) and the worker never see
  them;
- the web UI fact feed and `tq facts` render them like any fact (unknown
  types already fall back to neutral tone + raw tag);
- the journal gains the one thing it lacked: a durable answer to "did an
  interactive session touch this repo, and what did it mint?"

Consumers use the consumer-defined `session.Store` interface (Enqueue +
AppendFact + FactsForTask); `*sqlite.Store` implements it structurally.

## Store surface

One new concrete method, `(*sqlite.Store).AppendFact(ctx, journal.Fact)`, is
the only sanctioned out-of-band fact write. Task facts stay exactly where
they are: paired with their state change inside each operation's transaction
(the facts-first invariant is untouched — session facts are not task state).
The method reuses the internal `appendFact` helper via `withTx`; the 17
existing in-tx call sites are unchanged.

## Verification

- `internal/session`: table+behavior tests over a real sqlite store and a
  stub GitScanner (hermetic); the parser is tested pure (record framing,
  multi-value trailers, exact-value matching, SHA-256 repos); real-git
  integration tests are `//go:build unix` (attribution, empty repo,
  non-repo failure).
- `internal/queue/sqlite`: AppendFact persists a non-task fact, assigns
  Seq/Time, materializes no task row.
- `cmd/tq`: end-to-end begin → work (footer commits) → close → replay close
  against real git + sqlite; double-begin and missing-id refusals.

## Deliberate choices and tradeoffs

- **Facts after enqueues.** The closed fact carries the minted task IDs, so
  it is written after the two enqueues; a crash in between loses the fact
  lineage but not the work. Re-close heals: dedup keys return the existing
  tasks and the fact re-appends. Atomicity across the store's public API
  would need a combined enqueue+fact call — not worth it for a prototype.
- **Fresh-vs-known.** A replay close (closed fact exists) reports "known
  (dedup)"; a pending-but-already-enqueued task is otherwise indistinguishable
  from a fresh mint (the sweeper has the same heuristic limitation).
- **Yolo pinned, not flaggable.** Non-yolo runs would prompt headlessly and
  hang; there is no working non-yolo mode for these mints, so no flag.
- **Session reviews are type `review`.** They get the sweeper's loop-guard
  exemptions for free, and a request_changes verdict with `--review-autofix`
  mints fix tasks quoting the session summary — coherent.
- **`reviewed_task: session:<id>` is not resolvable via `tq show`.** It is
  lineage, not a foreign key; noted here so nobody "fixes" it into an ID
  format that collides with real tasks.

## Open questions (owner calls, in impact order)

1. **Attribution gap (pre-existing, stays ours):** commits folded by the
   auto-commit daemon carry no footer and are invisible to close — the
   biggest honesty hole, orthogonal to the trigger choice.
2. **Budget bypass:** direct enqueues skip the pool's daily-cap tick guard
   (two tasks per close is bounded, but a close-storm could still spend).
   Route through the budget projection if sessions become frequent.
3. **Trigger:** wire #3146's SessionEnd to `tq session close` (env carries
   the session id), or run the PreToolUse registry sweeper for pre-#3146
   environments. Neither exists yet.
4. **Postgres parity:** `AppendFact` is sqlite-only until the bridge needs
   multi-machine journals; add it to `internal/queue/postgres` + conformance
   suites when promoting.
5. **git ≥ 2.15 floor** for `%(trailers)` — every supported environment
   clears it today; a `--grep` fallback exists if a bare environment shows up.
