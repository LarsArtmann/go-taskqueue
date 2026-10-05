# Secret-Injection Seam — design note (M24.1)

**Status:** prototype rung SHIPPED (env denylist at the agent spawn); the full minted-env design below awaits an owner ruling before any widening.
**Predecessor:** output redaction (`internal/executor/redact.go`) — that seam redacts what agents EMIT; this seam bounds what agents RECEIVE.
**Date:** 2026-10-05.

## The problem, honestly

The agent process inherits the pool's ENTIRE environment (`os.Environ()` plus the verdict/question channels). Consequences, in rising severity:

1. **Operational bleed-through** — `TQ_DB` (the pool's journal path, in production the PRODUCTION journal) rides into every agent shell. An agent that shells out to `tq` inside its repo reads or mutates the live journal (AGENTS known-issues names the smoke-time variant of exactly this hazard).
2. **Secret blast radius** — provider keys the pool holds for its own bridges (`TQ_PAP_API_KEY`) and any future credential the pool process accumulates are one `env` away from an autonomous shell, and from there into a log, a commit, or a prompt.
3. **No revocation story** — a leaked inherited key is rotated at the operator's credential store, not at the queue.

## What shipped (the denylist rung)

`executor.agentEnv` strips an exact-key denylist from the inherited env at BOTH agent spawns (work turn + closeout turn). Shipped list: `TQ_DB` only. Design constraints:

- **Exact-key, not pattern.** No `*_API_KEY` globbing: provider keys the agent legitimately consumes (crush reads them from the environment) must never be stripped by pattern-matching enthusiasm — that would break every agent run to win a security argument.
- **Denylist, not allowlist.** An allowlist needs the inventory of everything every executor legitimately passes through; a denylist needs only the things that must never pass. Start narrow, grow by triage.
- **Malformed entries pass.** A no-`=` environment entry is not a secret-bearing assignment; dropping it could break platform quirks for zero security gain.

## The successor design (owner-gated)

1. **Minted per-run env** — the pool builds each agent's environment from an explicit ALLOWLIST template (`PATH`, `HOME`, `LANG`, the executor's channel vars) plus per-run minted credentials where a bridge needs one. Revocation = re-mint.
2. **Forbidden-key strip list under `--strict-env`** — a flag-gated widening of the denylist for high-trust-requirement pools; triaged per key with the "does crush need this to reach the provider" test.
3. **Managed HOME allowlist** — agents currently inherit the operator's HOME (crush configs, cached credentials, shell history). A managed HOME (symlinked .crushrc only) is the endgame but changes agent behavior in ways only a ruling can settle (which configs are load-bearing).
4. **Audit** — `tq doctor` reports the denylist in force and whether any stripped key was OBSERVED in the pool env (visibility without values).

## Owner questions this design needs answered

1. Which non-TQ keys may be denied by default (candidates: `TQ_PAP_API_KEY` with the bridge reading it from config instead of env)?
2. Is `--strict-env` allowlist mode wanted for the fleet, or per-repo (`.crushrc`/`.tq-verify`-style per-repo trust files)?
3. Managed HOME: is a symlink-only HOME acceptable to crush's config discovery?

## Verification trail

- Prototype: `TestAgentEnvStripsDenylist` (exact-key semantics, malformed pass-through, non-denied TQ_ keys survive).
- The shipped strip is exercised by every agent-spawn test in the executor module (env flows through `agentEnv` at both spawn sites).
