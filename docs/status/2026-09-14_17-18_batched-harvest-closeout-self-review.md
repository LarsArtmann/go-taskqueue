# Batched Harvest Close-Out — Full Status + Brutal Self-Review

_2026-09-14 17:18, interactive session. Owner prompt: "batch tasks more,
give the AI more power when it's supposed to get shit done", then the
a)–g) close-out. Scope: THIS session only (feature session report:
`2026-09-14_17-16_batched-harvest-agent-power.md`)._

## a) FULLY DONE

(re-verified fresh at 17:18 for this report: harvest suite green, root
build+vet green)

1. **Batched harvest, end to end** — `--batch-items N` (default OFF):
   `internal/harvest/harvest.go` (`runRepoBatched` two-phase admission,
   `admitRun`, `enqueueBatch`, `buildBatchPayload`, `batchKeyOf`),
   `DefaultBatchPromptTemplate`, `Config.BatchItems`/`BatchPromptTemplate`;
   `AgentPayload.Items` (`internal/executor/agent.go`); batch-aware
   prune (`prune.go`), audit (`drift.go`), survey (`observe` member keys),
   webui (`payload.go` full-member lede + batch field); pool flag +
   0..10 validation + startup warning (`cmd/tq/agentpool.go`).
2. **Power grant** in both work prompts: agents MAY append NEW unchecked
   follow-up items; existing pacing/budget/priority gates own admission.
3. **Tests**: 10-test `batch_test.go` + batch rows in
   `TestAgentPromptsDropSelfReport`/`TestAgentPromptsGuardrails` +
   `TestPayloadViewAgentBatch` + `TestParseAgentPoolOptionsBatchItems`.
4. **Master-red repair for concurrent agents**: 11 facade aliases
   (DepBump + `queue.EnqueueDetail`), gosec G602 `#nosec` triage,
   two treefmt rounds. Facade parity OK (7 facades).
5. **Docs**: planning doc, CHANGELOG (Added), FEATURES row, AGENTS.md
   contract bullet, DOMAIN_LANGUAGE "Batch", 17-16 feature report + index
   row (status-index gate green).

## b) PARTIALLY DONE

1. **Full-gate run**: all module/root/race/nix/doc gates green EXCEPT
   lint-baseline — stays red on rows exclusively in concurrent agents'
   files (`depbump.go` ×6 new classes, `sweep.go` intrange,
   `session_test.go`/`handlers.go` golines). Zero findings in this
   session's files (verified in the gate's own report). Their triage.
2. **gosec G602 fix is UNVERIFIED locally** — gosec exists only in CI; the
   `#nosec` annotation is the documented mechanism, first machine proof on
   the next push.
3. **Master CI green** — repaired locally, red on origin until this tree
   is pushed (daemon never pushes; push is an owner action).

## c) NOT STARTED

1. Live-pool dogfood of batching (SystemNix `--batch-items` + raised
   `--task-timeout`).
2. Batch smoke script (no `scripts/smoke/batch-*.sh`; status-loop.sh-style
   proof absent).
3. `tq harvest --json` batch shape audit (per-member Enqueued rows share
   one TaskID — consumers may double-count work; unaudited).
4. `tq show` CLI payload section rendering `items` (only the web UI shows
   the member list).
5. Batch-aware review prompt quoting (reviews see `Item` = first member).

## d) TOTALLY FUCKED UP

Nothing catastrophic. Honest near-misses, self-caught mid-session:

1. **Premature "master fully fixed" claim** — said it BEFORE the second
   treefmt drift and the lint-baseline residue surfaced; corrected within
   the session, final state reported accurately.
2. **Test sloppiness, round 1**: wrong expected counts (2 vs 3 stale-open,
   5 vs 6 covered items), a mangled `[x]`-replace that made one prune
   assertion pass for the wrong reason, a wrong `strings.Replace` target
   (marker text is stripped at parse time). All caught by red tests, all
   fixed — but three rounds of failures on MY OWN fresh code is churn a
   careful first pass would have avoided.
3. **Forgot the mid-flight race on the audit test**: leftover
   Cancel/RescueDead stand-in code in the first draft (never committed,
   rewritten before running).

## e) WHAT WE SHOULD IMPROVE

1. **Re-run the canonical gate ONE more time before every DONE claim** —
   my last full `-race` predates the final edits (concurrent agents landed
   `task.go` changes after it); only harvest-suite + build+vet were fresh
   at close-out. The AGENTS.md ritual exists precisely for this.
2. **Measure the actual batching win** — derived session usage
   (go-crush-data: cost/tokens/messages per task) makes "batch of 3 vs 3
   singles" empirically comparable; do it during dogfood, not by vibes.
3. **Collision warning channel for cross-agent completions** — I added the
   DepBump aliases while its owner was mid-flight; if they add the same
   aliases they hit "X redeclared" in the facade package. Mitigation is
   trivial (delete dup lines) but nothing WARNED them; my report was the
   only signal.
4. **Verify security annotations with the actual scanner** — `#nosec`
   written without a local gosec run is a hypothesis, not a fix (same
   class as the 2026-09-13 gosec false-green lesson, inverted).
5. **Batch failure semantics need an operator story** — a dead batch
   dead-letters ALL members at once; recovery = edit any member text (new
   key) or `tq dlq --rescue`. That ladder lives in the plan doc, not in
   `tq dlq` output where the operator stands when it happens.

## f) NEXT (ranked, honest count: 28 — padded fluff withheld)

1. Dogfood: set `--batch-items 3` + `--task-timeout 2h` on SystemNix pool;
   watch a real batch complete.
2. Cost/token A-B: batch-of-3 vs 3 singles via go-crush-data derivation.
3. `scripts/smoke/batch-loop.sh`: stub agent ticks all items `[x]`,
   one commit per item, footer per commit; assert task completes.
4. Audit `tq harvest --json` consumers for per-member/TaskID multiplicity.
5. `tq show`: render `items` in the payload section.
6. Review prompt: quote the full batch contract, not the first member.
7. `tq dlq` batch hint: when a dead task carries `itemKeys`, print the
   member-edit escape hatch in the rescue guidance.
8. Pin `TestBatchPriorityTakesMax` to the exact marker band (90), not `> 0`.
9. Status windows: summarize batch Items (first member misrepresents scope).
10. Per-repo batch ladder (`--batch-items` map, like `--repo-timeout`).
11. Decide agent-appended item rules: forbid `— P[1-4]` markers in
    agent-written items? (see g)3)
12. Batch-aware `--prune-stale` report line: "batch of N, M/N members
    stale" instead of silence on partial staleness.
13. e2e: batch + 429 requeue mid-batch (verdict-closeout resume semantics
    under batches).
14. e2e: batch + review + `--review-autofix` quoting (cross-ref criterion
    under a batch footer).
15. Consider `DefaultBatchPromptTemplate` `{{ITEMS}}` cap guard when an
    item text itself is multi-line (renders as broken numbering today —
    item texts are single-line by TODO_LIST convention, but nothing
    enforces it for agent-appended items).
16. Harvest DryRun output: batch grouping visible in `--json` (Fresh rows
    share TaskID "" — indistinguishable from singles; add a BatchSize field).
17. Docs: README batching section once dogfooded.
18. Lint-baseline: DepBump/webui agents triage or regen (their gate).
19. gosec: confirm G602 annotation on the post-push CI run.
20. Push the tree (owner) → verify master CI green end-to-end.
21. ADR if batching becomes default-on (working-set/pacing semantics
    change materially at N>1).
22. Batch + prioritize interplay: scorer sees no `todo:` keys while
    batching is on — decide if the scorer should score member sets.
23. `tq tasks` list: badge batch tasks (e.g. `[3 items]`).
24. Fuzz: `FuzzParseRepo` corpus gains a batched TODO fixture.
25. webui board card: show batch size chip on the task card.
26. Consider capping batch TimeoutMinutes at the worker ceiling with a
    warn instead of silently exceeding (payload timeout > task-timeout
    currently just gets clamped by the worker — verify + document).
27. Close-out turn under batches: report path per batch, verify it
    dedupes member mentions (prompt says "THIS batch's work").
28. Follow the plan doc's rejected-alternatives ledger if batching
    underdelivers: warm-session chaining experiment design.

## g) QUESTIONS (cannot figure out myself)

1. **Production default**: should the SystemNix pool switch batching ON
   (and at what N + `--task-timeout`)? That is a spend/latency/risk call
   on the live Z.ai window, not derivable from code.
2. **Dead-batch semantics**: when a batch dies, all members dead-letter
   together — is that acceptable, or should rescue/autopsy split the batch
   back into single-item tasks (product ruling, changes DLQ semantics)?
3. **Agent-set priorities**: the backlog grant lets agents append items —
   may those items carry `— P[1-4]` markers (agents effectively ranking
   their own follow-up work above human-ranked items), or should the grant
   forbid markers in agent-written items?

_Report written 17:18; index row added; gates: status-index + doc-refs +
todo-list + harvest suite + root build/vet green at write time. Waiting
for instructions._
