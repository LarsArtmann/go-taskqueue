# Batched Harvest + Agent Power Grant — Feature Session

_2026-09-14, interactive session (owner prompt: "get more out of the AI
Agent if we could batch tasks more, and give the AI more power to do
things, at least when it actually is supposed to get shit done")._

## a) Fully done

- **Batched harvest (`--batch-items N`, default OFF)** — one agent task
  carries a run of up to N adjacent admissible items of the same
  TODO_LIST.md section; ONE session works them in order. Implementation:
  `internal/harvest/harvest.go` (`runRepoBatched`, `admitRun`,
  `enqueueBatch`, `buildBatchPayload`, `batchKeyOf`; `Config.BatchItems` +
  `BatchPromptTemplate`), `DefaultBatchPromptTemplate` (per-item commit +
  `Task-Queue-ID` footer + checkoff, per-item `— BLOCKED:` escape = partial
  success with green verify, retry skips already-`[x]` members).
  Semantics: deterministic `batch:` dedup over the SORTED member keys
  (reorder never forks, member edit forks); payload `items` + `itemKeys`
  (`Item` stays the first member — review quoting, status windows,
  `tq show` provenance keep working); `TimeoutMinutes` scales per member;
  priority = max over members; marker = max; a batch is ONE task against
  `--max-per-tick` and the daily budget (documented amortization).
- **Batch-aware consumers**: prune-stale (`prune.go` batch pass — cancel a
  pending batch only when EVERY member is ticked/absent; the generic absent
  pass skips `batch:` keys), audit drift (`drift.go` member-key resolution —
  stale-open mints per-item catch-ups under a completed batch, stale-done
  reports), harvest survey tracks member keys (no re-attempts next tick),
  webui payload view leads with the full member list + batch-size field.
- **Agent power grant (both work prompts)**: agents MAY append NEW
  unchecked follow-up items to TODO_LIST.md — the queue's existing
  pacing/budget/priority gates own admission, so the grant cannot bypass
  spend control (direct `tq enqueue` stays forbidden: mint bypass).
  Self-dealing guard (.crushrc/.tq-verify) untouched.
- **Pool wiring**: `--batch-items` flag (0..10 validated; help carries the
  amortization + `--task-timeout` guidance), startup warning when > 1,
  `pool.conf` key works via the generic config-file mechanism.
- **Master-red repair (concurrent agents' in-flight work)**: completed the
  11 missing facade aliases for the DepBump feature (`executor/executor.go`,
  `queue/queue.go` — `EnqueueDetail`, 10 DepBump symbols; parity gate now
  OK over all 7 facades), triaged gosec G602 at `internal/depsweep/sweep.go`
  (`#nosec` — loop guard caps i at 2, segments is `[3]int`), and formatted
  the treefmt-drifted files (`nix fmt`; a second drift round in
  `fragments.templ` from the live webui agent was also formatted).
- **Docs**: planning doc
  `docs/planning/archived/2026-09-14_batched-harvest-agent-power.md` (design +
  rejected alternatives: warm-session chaining, in-run claim loop, direct
  enqueue), CHANGELOG (Added section), FEATURES row, AGENTS.md payload
  contract bullet, DOMAIN_LANGUAGE "Batch" term.

## b) Verification (gates run, evidence)

- `internal/harvest`: build + vet + full suite green, including new
  `batch_test.go` (10 tests: grouping, gates-as-one-task, payload contract
  incl. timeout 3×30=90 + marker max, key determinism, survey no-re-attempt
  coverage map, priority max, prune all-ticked/all-absent/partial, audit
  stale-open×3 + stale-done, single-path-untouched for 0/1, executor field
  decode) and batch rows added to `TestAgentPromptsDropSelfReport` +
  `TestAgentPromptsGuardrails` (footer, BLOCKED, guardrails, follow-up
  grant pinned; no TQ_RESULT).
- All 15 sub-modules: build + vet + tests green (per-module loop).
- Root: build + vet + `go test ./... -race` green. `examples/embed`
  (facade consumer): build + vet green.
- `test-cmd-tq.sh` (devmod gate) green incl. new
  `TestParseAgentPoolOptionsBatchItems` (plumb + >10 rejection).
- Doc gates: release-docs, status-index, ghost-archives, todo-list,
  features-roadmap, features-ci (1 cited run green), doc-refs, go-mods —
  all green. `nix build` green; `nix flake check` green after the second
  treefmt round. Facade parity: OK (7 facades).

## c) Partially done / handed off

- **lint-baseline gate stays RED on rows owned by concurrent agents**
  (verified zero findings in this session's files): `depbump.go`
  (contextcheck/gocritic/golines/nlreturn/nolintlint/wsl_v5 — six NEW
  classes), `sweep.go` (intrange at the `for i := 0; i < 3` loop),
  `session_test.go`/`handlers.go` (golines — the live webui agent). The
  DepBump owner was writing `sweep.go` seconds before this report; fixing
  their style classes mid-flight invites collisions, and regenerating the
  baseline would launder feature lint debt round-13 T5 exists to catch.
  Their triage: fix the findings or own a deliberate regen.
- **Master CI red (runs 34842616172/34843189306/34843873218)**: caused by
  the pushed mid-flight states (parity/treefmt/gosec), all repaired in this
  tree — master goes green when this tree is PUSHED (daemon never pushes;
  push is owner/concurrent-agent action). `gosec` runs only in CI: the
  `#nosec G602` annotation is the documented mechanism and gets its first
  machine proof on that push.

## d) Not started

- Live-pool dogfood of batching (needs owner to set `--batch-items 3` +
  raise `--task-timeout` on the SystemNix deployment; batch deaths
  dead-letter the whole run — `tq dlq` is the human surface).
- Batch-aware review prompt (review currently quotes the first member via
  `Item`; acceptable, a richer quote is a follow-up).
- AI scorer interaction: batches are invisible to the `todo:`-keyed
  prioritize sweeper (documented in AGENTS.md; revisit if batching becomes
  the fleet default).

## e) Improve next

1. Session-chaining experiment (rejected alternative in the plan doc) if
   batching alone leaves provider windows idle.
2. `--batch-items` per-repo ladder (big repos batch, tiny don't).
3. Status windows should summarize batch Items (currently first member).
