# Owner-Decision Digest — 2026-11-18

ONE list of every open decision that only the owner can make, harvested
from the todo gate's 19 UNBLOCKED-OWNER-GATED advisories (check-todo-list.sh,
2026-11-18 run) plus the sixth-window §g questions. Each entry: the decision,
the options, and the agent-recommended default. Rows marked `— BLOCKED:` in
TODO_LIST.md point here instead of carrying their full context inline.

Answers can be given per number ("D3: shared home") — the queue dispatches
the unblocked work on the ruling.

## Sixth-window §g questions (2026-11-18 14:33 report)

| # | Question | Status |
| --- | --- | --- |
| G1 | /tmp 48G cleanup ownership | RESOLVED without action: the tmpfs freed itself (47G avail at 18:00, foreign fixtures gone); nothing of another session's was deleted |
| G2 | Crush-drift cure shape (tolerant asserts vs pin input version) | DECIDED-BY-AGENT: memory-shape tolerant asserts — route M7 keeps the run-rc respawn proof and tolerates the v0.101.0 post-run idle-exit; pool verdict probe polls running-then-completed (the stall-free path completes inside one poll gap). Both smokes verified live at HEAD; receipts /var/tmp/pareto-battery/drift-{route,pool}-cure-verify.log |
| G3 | Push-policy ratification (push on >600s timing windows) | OPERATING as the standing dispatch dictates: 2 pushes this window (`2ab1f2fc`, `b5b40786`), both CI-watched; ratify or overrule |

## The 19 advisories (gate order)

| # | Decision | Options | Agent default |
| --- | --- | --- | --- |
| D1 | Crush-store growth: prune script + monthly timer (`docs/problems/crush-session-store-growth.md`) | straight DELETE vs archive-then-delete | straight DELETE if no archive consumer exists |
| D2 | cmd/tq release wave: push master + `cmd/tq/v0.4.3` (row 73; everything else landed) | owner pushes, then proxy-verify | push now; `go install github.com/larsartmann/go-taskqueue/cmd/tq@v0.4.3` is the verify |
| D3 | 2 NEW HIDDEN cross-backend mirrors (sqlitev4/postgresv4 adapter pairs, archive.go pair) | ONE shared home in `companion` vs mirror-baseline row with provenance | shared home (matches the row-426 dedup-accepts-≈-0 ruling) |
| D4 | Retro-cure task …82a1765: rows 225/226 show COMPLETED but never landed | re-enqueue vs evidence-tick + write the missing closeout | evidence-tick if the work is provably at HEAD, else re-enqueue; the closeout report is owed either way |
| D5 | AGENTS.md budget constant re-tighten (27,598/27,600 B today — 2 B headroom) | compress-only prune to ~24k then re-tighten both twins in one commit | compress + re-tighten; the T13 hook now enforces the pre-edit ritual live |
| D6 | generic_return interface-satisfaction blocker (journal/cqrs ReadAll) | permitted `//nolint` convention vs erraudit tool support vs adapter-wrapper | adapter-wrapper (no lint suppression, signature stays contractual) |
| D7 | go-health v0.8.1 foreign bump: adopt + verify or revert | run the missing verification (race, release-gates, go-mods, vendorHash) and record ownership | adopt + verify; the bump already rides master |
| D8 | 2026-10-28 local/origin divergence row | rebase ritual vs close | OBSOLETE — HEAD == origin/master == merge-base at 2026-11-18 18:04; close the row |
| D9 | internal/retryx graduation (footer-less sweep origin) | confirm intent, module gate, record adoption | adopt officially (it is load-bearing: Windows sharing-violation retries) |
| D10 | Promote UNBLOCKED-OWNER-GATED advisory to a hard gate for a batch's OWN anchors | hard-for-own-rows vs keep WARN | hard-for-own-rows (an owner-gated row must never leave its batch as a plain open row) |
| D11 | Finding-envelope dedup: anchor-text hash in dispatch envelopes | platform change in the queue | adopt (kills sibling-ticket re-pays) |
| D12 | RULING-BY-AGENT marker convention (agent-executed ruling rows are auditable) | AGENTS.md line + backfill row-1210 tick | adopt |
| D13 | Corpus-wide stale-wording sweep extension (architecture-understanding/, replay/, ADOPTION docs) | run the sweep | run it (cheap, doc-only) |
| D14 | heal-daemon-sweep `--retitle` discards footer-less sweep BODIES | document vs change | document (bodies of heuristic sweeps are daemon noise; subjects are what attribution needs) |
| D15 | Read-only production-journal probe recipe (ro-uri sqlite3) | AGENTS.md line vs docs/problems note | docs/problems note (AGENTS is 2 B under budget) |
| D16 | pma daemon commits with `--no-verify` — repo hooks never fire on sweeps | drop --no-verify for this repo vs mirror the zero-byte refusal in pma vs accept-as-is | mirror the zero-byte refusal in committer.go (protects every swept repo) |
| D17 | Rewrite-rate threshold: adaptive p95 default (code landed, TQ_REWRITE_RATE_ADAPTIVE) | flip default vs keep constant and raise MAX | flip to adaptive (22 of 28 live days hot at MAX=3 is constant noise) |
| D18 | Done-prompt amend-vs-cite: cite only POST-amend shas | prompt line + attribution-tolerance verification | adopt the prompt line; verify the two commit shapes |
| D19 | Fixtures advisory extension to plain `>` redirects (non-heredoc) | sibling rule (fixture-tmp) vs extend heredoc-fixture-tmp | extend the existing rule (one detector, two syntaxes) |

## Standing owner-action rows (not advisories, but owner-only)

- Row 24: host `MemoryMin=256M` for tq-agent-pool (ruling 2026-11-17 already
  fixed the fence + ioprio; only the unit edit remains, sudo-gated).
- Row 25: llama-vlm-cap GGUF streaming investigation (foreign service).
- Row 73/D2: the release wave push (the ONE unblock for row 80's doc note).
- Row 163: SystemNix input flip + deploy (sudo on evo-x2, agent cannot run it).
- Row 236: live CQA instance URL + credentials for the bridge verification.
