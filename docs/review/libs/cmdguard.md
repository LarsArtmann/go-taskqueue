# cmdguard — Library Utilization Review

**Verdict:** NOT ADOPTED — KEEP OUT — a 27-subcommand CLI on stdlib `flag` with ADR-pinned installability (ADR-0017) and a framework-free stance (ADR-0004) works and is gate-covered; cmdguard's buys (typed flags, config precedence) would cost a full rewrite of the dispatch layer on a v5.0.0 released the day of this review.

**Pinned:** none (not a dependency; verified 2026-11-08 by `rg -i 'cmdguard' -g 'go.mod' -g 'go.sum' -g '!vendor/**' .` → no output, rc=1; same for `--glob '*.go'` import greps; `grep -c 'cobra\|cmdguard' cmd/tq/go.mod` → 0)

**Latest:** root series v5.0.0 (2026-11-08) — `git tag -l 'v5*' 'v4*' | sort -V | tail -3` → v4.2.0, v4.2.1, v5.0.0 (local checkout /home/lars/projects/cmdguard, checked 2026-11-08; checkout sits on branch v5-scaffold at lint/v0.3.0-22-g7ccafc0)

**Prior coverage:** none (docs/review/libs/ has no cmdguard entry; AGENTS.md records no CLI-framework decision — ADR-0017/ADR-0004 are the adjacent rulings).

**Reviewed:** 2026-11-08 by reading /home/lars/.config/crush/skills/cmdguard/SKILL.md (the v5 adoption doc, source-verified rows cited below), /home/lars/projects/cmdguard/{go.mod,README.md} plus cmd/tq/main.go (usage const :44-123, commands map :131-172, exit codes :166-171), docs/adr/0017-cmd-tq-installable-module.md, docs/adr/0004-lifecycle-streaming-library-stance.md.

## 1. Where the need exists today

cmdguard v5 wraps spf13/cobra with: typed struct-tag flags (`flag/short/default/help/values/required/count/local/hidden/validate/env/prompt/secret`), per-command flag structs (F-types — a flag exists ONLY on its command), samber/do DI, middleware, config-file precedence (flag → env → config file → default, strict keys), signal handling + graceful DI shutdown batteries ON by default, exit-code contracts (`ExitCode`/`ExitCoder`), error hints, and a machine-readable `describe` JSON contract (SKILL.md mental-model, tag table, and verification rows — each row source-verified in the checkout).

The tq code it would replace:

- **Dispatch**: a 27-entry `commands` map in `main()` — `map[string]func([]string) error` (cmd/tq/main.go:131-172), unknown command → usage + exit 2 (main.go:166-171). Sub-subcommands (`watermarks show|set`, `session …`) hand-nest their own switches (main.go:3858, :3942).
- **Flags**: stdlib `flag` per subcommand — local `flag.NewFlagSet` in each `cmdX` (18 `fs.`/`flag.` sites in main.go alone; per-command files carry their own, e.g. verdict.go, session.go). Per-FlagSet construction already gives cmdguard's headline scoping property: `tq verdict --db X` fails with "flag provided but not defined" because `--db` is not on that FlagSet.
- **Usage text**: one hand-maintained ~80-line `usage` const (main.go:44-123) — the artifact agent prompts quote verbatim (`--file` rails baked into prompt templates, internal/executor/agent.go:585-590, :594-598).
- **Config precedence**: hand-rolled — `$TQ_DB` env, `--store/--db` flags, and the pool config file ("edit freely, flags on the CLI win", bootstrap.go:795-815).
- **Exit codes**: 2 usage/unknown, 1 any command error (main.go:160-171) — asserted by smokes and cmd/tq tests.

## 2. Capability inventory (fit)

| Capability | Status | Evidence |
| --- | --- | --- |
| Typed struct-tag flags | 🟡 partial — stdlib flags are typed variables already; what's missing is the declaration boilerplate, not type safety | main.go flag sites; SKILL.md tag table |
| Per-command flag scoping (F-structs) | 🟡 partial — local FlagSets already reject foreign flags per command | SKILL.md "flag scoping" (pinned by cmdguard's own `cli_flag_scoping_test.go`); tq FlagSet-per-cmdX |
| Root-persistent vs root-local flags | ⚪ N/A — tq has no persistent root flags; every flag is command-local by construction (the `--db` family repeats per command deliberately) | usage const main.go:44-123 |
| Config-file precedence (flag→env→file→default, strict keys) | 🟡 partial — env + flags + one config file exist, hand-resolved; strict-key/did-you-mean does not | bootstrap.go:795-815, TQ_DB/TQ_SERVE_TOKEN |
| DI container (samber/do) | ⚪ N/A — composition is explicit (`internal/composition`, ADR-0011 module seams); ADR-0004 explicitly declined samber/do | ADR-0004 Context |
| Middleware (spinner, logging, telemetry) | ⚪ N/A — CLI commands are short-lived batch operations; no cross-cutting request pipeline | cmd/tq structure |
| Exit-code contract (`ExitCode`, `ExitCoder`) | 🟡 partial — the 2/1 contract is simple, documented, and test-asserted | main.go:160-171 |
| `describe` machine-readable CLI contract | 🔴 MISSING-BUT-RELEVANT (low) — nice for agent-driven tooling, but tq's agent interface is the prompt rails + `tq verdict`/`tq ask`, not generic introspection | SKILL.md "Machine-readable contract"; agent.go:585-598 |
| Signal handling + graceful DI shutdown batteries | ⚪ N/A — long-runners already own this via runactor (ADR-0004); one-shot commands need none | runactor.go:1-14 |
| Pure Go, `CGO_ENABLED=0` | compatible — cobra/fang/samber/do are pure Go | cmdguard go.mod:75-87 |

## 3. Findings (buy vs cost)

1. **Buy (real but modest):** typed flag structs would delete boilerplate and make flag/help drift structurally impossible (the `usage` const must be edited in lockstep with every FlagSet today); `values`/`validate`/`required` tags would replace hand-rolled validation (e.g. verdict.go:37-44 usage guards); config precedence would formalize TQ_DB/TQ_SERVE_TOKEN/pool.conf into one tested ladder.
2. **Cost (dominant) — churn on a working, gate-covered surface:** 27 subcommands across 21 non-test files (main.go alone is 4,385 lines) would each migrate to `NewCommand[T, F]` handlers; the usage const — quoted verbatim inside agent prompt rails and docs — becomes cobra-generated help, a behavior change for every prompt that greps it; smokes asserting exit codes and `--help` shapes all re-verify.
3. **Cost — ADR risk:** ADR-0017 makes cmd/tq a replace-free module whose `go install …/cmd/tq@vX.Y.Z` clean-room build is a release gate; pulling cmdguard/cobra/fang/do into that module grows the proxy-resolvable dep set of the installability-critical path for cosmetic gain. ADR-0004's stance ("patterns now, dependencies later") was written against exactly this trade.
4. **Cost — version risk:** v5.0.0 was tagged 2026-11-08 — the day of this review. Zero adoption runway, no downstream consumers beyond the owner's projects yet; tq would be an early adopter of a brand-new major on its most stability-sensitive module.
5. **Adoption site if it ever happened** (one command, root config carries the shared `--db` story):

```go
cli, err := cmdguard.NewCLI("tq", "projects-aware task work queue", RootConfig{})
verdict, err := cmdguard.NewCommand("verdict", VerdictFlags{},
    func(ctx context.Context, cfg *RootConfig, f VerdictFlags) error { return runVerdict(cfg, f) },
    cmdguard.WithShort("record a task's structured verdict into $TQ_RESULT_FILE"))
```

## 4. Fit assessment

cmdguard is the right tool for a greenfield multi-command CLI, and the sibling go-credentials-store shows it composing well with a config-file-heavy server product. But cmd/tq's constraints are the inverse: installability is architecture (ADR-0017), the framework-free stance is recorded (ADR-0004), the flag surface is command-local by design rather than by tagging, and the usage text is a load-bearing contract baked into agent prompts. What migration buys — less boilerplate, formal precedence, `describe` — is real but entirely internal-facing; what it costs is a rewrite of the dispatch layer of the one module whose build must never surprise the proxy. The CLI has no pain report on file (no TODO_LIST row, no docs/problems entry about flags).

## 5. Maturity + version currency

Root series: 24 tags, v1→v5.0.0; v5.0.0 tagged 2026-11-08 (brand new); v4.2.1 2026-11-04; last commit 7ccafc0 2026-11-08 (checkout on v5-scaffold, 22 commits past lint/v0.3.0). Sub-modules (glamour, prompts, spinner, telemetry, flightrecorder, lint, schema) independently versioned. Very actively maintained — same author as the go-taskqueue ecosystem; the risk is not abandonment but churn direction, which v5.0.0's same-day rename history (`CommandInfo`→`CommandMetadata` etc.) illustrates.

## 6. Conclusion

Keep out for cmd/tq. The CLI is complete, tested, and ADR-pinned to stdlib `flag`; cmdguard's genuine wins do not offset rewriting the dispatch of an installability-critical module on a major released today. Revisit only for a NEW operator-facing CLI (e.g. a future admin tool) where greenfield typed flags pay off immediately — there, `cmdguard.NewApp` + `describe` would be the right call from day one.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
rg -i 'cmdguard' -g 'go.mod' -g 'go.sum' -g '!vendor/**' .            # → no output (rc=1)
rg -i 'cmdguard' --glob '*.go' --glob '!vendor/**' .                   # → no output (rc=1)
grep -c 'cobra' cmd/tq/go.mod                                          # → 0
sed -n '131,172p' cmd/tq/main.go        # 27-entry commands map on stdlib flag
rg -c '"[a-z-]+":\s+cmd[A-Z]' cmd/tq/main.go  # → 27 command map entries
wc -l cmd/tq/main.go                                                   # → 4385
git -C /home/lars/projects/cmdguard tag -l 'v5*' 'v4*' | sort -V | tail -3  # v4.2.0 v4.2.1 v5.0.0
git -C /home/lars/projects/cmdguard log -1 --format='%ad %h' --date=short v5.0.0  # 2026-11-08 v5.0.0
head -20 /home/lars/.config/crush/skills/cmdguard/SKILL.md             # v5 capability summary
```
