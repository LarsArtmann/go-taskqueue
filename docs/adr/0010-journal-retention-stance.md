# ADR-0010: Journal retention stance — growth is bounded by policy, not by hope

- Status: Accepted
- Date: 2026-09-08
- Deciders: owner delegation (21:40 report §e-item: "cheap to decide early")
- Relates to: ADR-0006 (compaction mechanism), ADR-0009 (consumer policy,
  D5 loud resync)

## Context

The journal is append-only and every state change appends a fact (ADR-0001).
ADR-0006 decided HOW facts compact (hot-cold split at task granularity,
terminal-only, nothing deleted; `SQLiteStore.ArchiveFactsBefore` prototype
shipped) but deliberately left the OPERATING question open: when does a
deployment actually compact, who triggers it, and what happens before the
first compaction ever runs.

Today's numbers make the decision cheap: the dogfood journal's head is ~170
facts after two days of full autonomous dogfooding (22-task window included).
Even a 100× escalation (~17k facts) is a single-digit-megabyte SQLite table.
The risk is not size — it is that the answer "grow until it hurts" is
discovered by a long-running unattended pool where nobody is watching it hurt.

Actors with a stake in the answer:

- `tq show`/`tq facts` — read the hot table; archived tasks lose their trail
  unless the command unions the archive (ADR-0006 leaves that open).
- Bridges and sweepers — resume from persisted watermarks (ADR-0009);
  compaction below a live cursor is the loud-resync case.
- `internal/webui` — renders bounded tails; cold prefixes are invisible.

## Decision

1. **Retention is a deliberate, operator-owned act. Compaction never runs
   automatically.** No pool tick, no serve loop, no background sweeper ever
   moves facts. The single trigger is a human (or a human-written cron)
   invoking the compaction surface. Unattended growth is the accepted
   default; silent automatic history movement is not.
2. **The CLI surface lands only when it earns its complexity.** ADR-0006's
   `tq journal compact --before SEQ --min-age 720h` stays a sketch until the
   journal's hot table passes ~50k facts in a real deployment OR an operator
   asks for it — whichever comes first. Below that, `Facts()` reads stay
   cheap and the prototype's invariants are already pinned by tests.
3. **The retention floor is a first-class observable, not an afterthought.**
   Whatever surface ships must expose the compaction watermark
   (`journal_meta.archive_watermark`) next to consumer cursors — `tq stats`
   consumer-lag output and `tq doctor` are the homes. An operator cannot
   decide "compact now" without seeing "consumed through seq N, complete
   history above seq M".
4. **Compaction correctness gates are already fixed** (from ADR-0006/0009,
   restated as acceptance criteria for any future implementation):
   terminal-task granularity, `--min-age` never racing a live consumer
   cursor, and loud resync (never a silently narrowed `Facts` prefix) when a
   persisted cursor falls below the floor.
5. **Postgres parity is part of the surface, not a follow-up.** The store
   interface is shared; a compaction CLI that works only on SQLite would
   fork operational reality between stores (ADR-0007).

## Alternatives considered

- **Auto-compact on a size/age threshold inside the pool tick**: rejected —
  the pool is unattended by design; giving it a history-moving finger raises
  the blast radius (a bug archives live-consumer prefixes) for zero benefit
  at current scale.
- **TTL-based deletion**: rejected — violates the immutability trust
  contract (ADR-0006 rejects it for the same reason).
- **Deciding nothing until the journal "gets big"**: rejected — the 21:40
  report flagged it precisely because unbounded-growth decisions get
  expensive exactly when they are urgent.

## Consequences

- Nothing changes operationally today: no new commands, no new flags. This
  ADR is the recorded answer to "what happens as facts grow" — manual,
  deliberate, observable, gated by real demand.
- The trigger conditions in D2 are the go-signal for implementing ADR-0006's
  command sketch; when that lands, D3 (floor observability) lands with it in
  the same change, not after.
- SECURITY.md's unattended-pool blast-radius section needs no update: the
  pool gains no new capability.
