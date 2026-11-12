# cmdguard — Library Utilization Review

**Verdict:** NOT ADOPTED — KEEP OUT — a 27-subcommand CLI on stdlib `flag` with ADR-pinned installability (ADR-0017) and a framework-free stance (ADR-0004) works and is gate-covered; cmdguard's buys (typed flags, config precedence) would cost a full rewrite of the dispatch layer on a v5 line that moved again mid-review-window (v5.0.0 2026-11-08 → v5.1.0 2026-11-11, v4 retired the same day).

> **2026-11-11 refresh:** cmdguard shipped **v5.1.0** today (2026-11-11, tag at 671c128), three days after the v5.0.0 this review originally cited. What shipped (`CHANGELOG.md [5.1.0]`): ONE capability addition — strict positional-args control (`WithStrictPositionalArgs()` / `WithArgsPassthrough()`, pinned by `cli_args_strict_test.go`) — plus a Builder concurrency fix, a release-workflow lint fix, and dep/docs chores. The same day **retired the v4 line** (final tags v4.3.0/v4.3.1, frozen on `release/v4`; v4 had been uninstallable via `go get` since v4.2.1) and moved `master` to v5. **Nothing here changes the keep-out calculus**: tq's flag surface re-verified unchanged (27-entry commands map, stdlib `flag`, exit codes 2/1 re-asserted at cmd/tq/main_test.go:338), NOT-imported greps still rc=1, ADR-0017/ADR-0004 rationale stands. One ecosystem note: sibling go-credentials-store bumped its cmdguard/v5 pin to v5.1.0 the same day (its HEAD c423a30, 2026-11-11) — a first ecosystem consumer exists on v5; it is still outside tq.

**Pinned:** none (not a dependency; verified 2026-11-11 by `rg -i 'cmdguard' -g 'go.mod' -g 'go.sum' -g '!vendor/**' .` → no output, rc=1; same for `--glob '*.go'` import greps; `grep -c 'cobra' cmd/tq/go.mod` → 0)

**Latest:** root series v5.1.0 (2026-11-11, 671c128) — `git tag -l 'v5*' 'v4*' | sort -V | tail -3` → v4.3.1, v5.0.0, v5.1.0 (local checkout /home/lars/projects/cmdguard, checked 2026-11-11; checkout now on `master` at v5.1.0-22-g093271b — master carries v5 since the 2026-11-11 v4 retirement)

**Prior coverage:** none (docs/review/libs/ has no cmdguard entry; AGENTS.md records no CLI-framework decision — ADR-0017/ADR-0004 are the adjacent rulings).

**Reviewed:** 2026-11-08 by reading /home/lars/.config/crush/skills/cmdguard/SKILL.md (the v5 adoption doc, source-verified rows cited below), /home/lars/projects/cmdguard/{go.mod,README.md} plus cmd/tq/main.go (usage const :58-123, commands map :131-174, exit codes :166-173 — line anchors re-verified 2026-11-11), docs/adr/0017-cmd-tq-installable-module.md, docs/adr/0004-lifecycle-streaming-library-stance.md.

## 1. Where the need exists today

cmdguard v5 wraps spf13/cobra with: typed struct-tag flags (`flag/short/default/help/values/required/count/local/hidden/validate/env/prompt/secret`), per-command flag structs (F-types — a flag exists ONLY on its command), samber/do DI, middleware, config-file precedence (flag → env → config file → default, strict keys), signal handling + graceful DI shutdown batteries ON by default, exit-code contracts (`ExitCode`/`ExitCoder`), error hints, and a machine-readable `describe` JSON contract (SKILL.md mental-model, tag table, and verification rows — each row source-verified in the checkout). v5.1.0 (2026-11-11) adds strict positional-args control on top (`WithStrictPositionalArgs` rejects undeclared positionals, `WithArgsPassthrough` accepts-and-ignores them — CHANGELOG.md [5.1.0]); everything else in v5.0.0..v5.1.0 is fixes, deps, and docs.

The tq code it would replace:

- **Dispatch**: a 27-entry `commands` map in `main()` — `map[string]func([]string) error` (cmd/tq/main.go:131-174; `rg -c '"[a-z-]+":\s+cmd[A-Z]'` → 27), unknown command → usage + exit 2 (main.go:166-168). Sub-subcommands (`watermarks show|set`, `session …`) hand-nest their own switches (watermarks at main.go:3858; session in cmd/tq/session.go:28).
- **Flags**: stdlib `flag` per subcommand — local `flag.NewFlagSet` in each `cmdX` (14 `flag.NewFlagSet` sites in main.go; per-command files carry their own, e.g. verdict.go, session.go, ask.go). Per-FlagSet construction already gives cmdguard's headline scoping property: `tq verdict --db X` fails with "flag provided but not defined" because `--db` is not on that FlagSet.
- **Usage text**: one hand-maintained ~66-line `usage` const (main.go:58-123) — the artifact agent prompts quote verbatim (`--file` rails baked into prompt templates, internal/executor/agent.go:586, :594).
- **Config precedence**: hand-rolled — `$TQ_DB` env, `--store/--db` flags, `$TQ_SERVE_TOKEN` (main.go:4250-4251), and the pool config file ("edit freely, flags on the CLI win", bootstrap.go:795-815).
- **Exit codes**: 2 usage/unknown (main.go:168), 1 any command error (main.go:173) — asserted by cmd/tq tests (main_test.go:338 "unknown command exit code = 2, want 2") and smokes.

## 2. Capability inventory (fit)

| Capability | Status | Evidence |
| --- | --- | --- |
| Typed struct-tag flags | 🟡 partial — stdlib flags are typed variables already; what's missing is the declaration boilerplate, not type safety | main.go flag sites; SKILL.md tag table |
| Per-command flag scoping (F-structs) | 🟡 partial — local FlagSets already reject foreign flags per command | SKILL.md "flag scoping" (pinned by cmdguard's own `cli_flag_scoping_test.go`); tq FlagSet-per-cmdX |
| Strict positional-args control (NEW v5.1.0 `WithStrictPositionalArgs`) | ⚪ N/A — tq's per-command FlagSets already refuse undeclared flags, and positional handling is a hand `switch` per command | CHANGELOG [5.1.0] 2026-11-11; session.go:28 |
| Root-persistent vs root-local flags | ⚪ N/A — tq has no persistent root flags; every flag is command-local by construction (the `--db` family repeats per command deliberately) | usage const main.go:58-123 |
| Config-file precedence (flag→env→file→default, strict keys) | 🟡 partial — env + flags + one config file exist, hand-resolved; strict-key/did-you-mean does not | bootstrap.go:795-815, TQ_DB/TQ_SERVE_TOKEN |
| DI container (samber/do) | ⚪ N/A — composition is explicit (`internal/composition`, ADR-0011 module seams); ADR-0004 explicitly declined samber/do | ADR-0004 Context |
| Middleware (spinner, logging, telemetry) | ⚪ N/A — CLI commands are short-lived batch operations; no cross-cutting request pipeline | cmd/tq structure |
| Exit-code contract (`ExitCode`, `ExitCoder`) | 🟡 partial — the 2/1 contract is simple, documented, and test-asserted | main.go:168, :173; main_test.go:338 |
| `describe` machine-readable CLI contract | 🔴 MISSING-BUT-RELEVANT (low) — nice for agent-driven tooling, but tq's agent interface is the prompt rails + `tq verdict`/`tq ask`, not generic introspection | SKILL.md "Machine-readable contract"; agent.go:579-595 |
| Signal handling + graceful DI shutdown batteries | ⚪ N/A — long-runners already own this via runactor (ADR-0004); one-shot commands need none | runactor.go:1-14 |
| Pure Go, `CGO_ENABLED=0` | compatible — cobra/fang/samber/do are pure Go | cmdguard go.mod:75-90 |

## 3. Findings (buy vs cost)

1. **Buy (real but modest):** typed flag structs would delete boilerplate and make flag/help drift structurally impossible (the `usage` const must be edited in lockstep with every FlagSet today); `values`/`validate`/`required` tags would replace hand-rolled validation (e.g. verdict.go:37-44 usage guards); config precedence would formalize TQ_DB/TQ_SERVE_TOKEN/pool.conf into one tested ladder.
2. **Cost (dominant) — churn on a working, gate-covered surface:** 27 subcommands across 21 non-test files (main.go alone is 4,397 lines) would each migrate to `NewCommand[T, F]` handlers; the usage const — quoted verbatim inside agent prompt rails and docs — becomes cobra-generated help, a behavior change for every prompt that greps it; smokes asserting exit codes and `--help` shapes all re-verify.
3. **Cost — ADR risk:** ADR-0017 makes cmd/tq a replace-free module whose `go install …/cmd/tq@vX.Y.Z` clean-room build is a release gate; pulling cmdguard/cobra/fang/do into that module grows the proxy-resolvable dep set of the installability-critical path for cosmetic gain. ADR-0004's stance ("patterns now, dependencies later") was written against exactly this trade.
4. **Cost — version risk:** the v5 major is 3 days old and has already shipped a second tag (v5.0.0 2026-11-08 → v5.1.0 2026-11-11), while the same day retired v4 entirely (final v4.3.1; v4 unresolvable via `go get` since v4.2.1). One ecosystem consumer now exists on v5 — go-credentials-store bumped its pin the day v5.1.0 landed (2026-11-11) — but tq would still be an early adopter of a brand-new major on its most stability-sensitive module, and the 3-day tag cadence + same-day major retirement is churn, not maturity.
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

Root series: 27 tags, v1→v5.1.0; v5.0.0 tagged 2026-11-08, **v5.1.0 2026-11-11** (3 days later — strict positional-args + Builder concurrency fix; tag at 671c128); the same day retired the v4 line (final v4.3.0/v4.3.1, frozen on release/v4 — v4 had been uninstallable via `go get` since v4.2.1) and moved `master` onto v5 (checkout on `master` at v5.1.0-22-g093271b, HEAD 093271b 2026-11-11). Sub-modules (glamour, prompts, spinner, telemetry, flightrecorder, lint, schema) independently versioned. Very actively maintained — same author as the go-taskqueue ecosystem; the risk is not abandonment but churn direction, which three tags in three days plus a same-day major retirement illustrates. First ecosystem consumer on v5: go-credentials-store (cmdguard/v5 v5.1.0, 2026-11-11).

## 6. Conclusion

Keep out for cmd/tq. The CLI is complete, tested, and ADR-pinned to stdlib `flag`; cmdguard's genuine wins do not offset rewriting the dispatch of an installability-critical module — re-verified 2026-11-11 against v5.1.0, whose only new capability (strict positional args) is one tq's FlagSets already deliver. Revisit only for a NEW operator-facing CLI (e.g. a future admin tool) where greenfield typed flags pay off immediately — there, `cmdguard.NewApp` + `describe` would be the right call from day one.

## How to verify

```bash
cd /home/lars/projects/go-taskqueue
rg -i 'cmdguard' -g 'go.mod' -g 'go.sum' -g '!vendor/**' .            # → no output (rc=1)
rg -i 'cmdguard' --glob '*.go' --glob '!vendor/**' .                   # → no output (rc=1)
grep -c 'cobra' cmd/tq/go.mod                                          # → 0
sed -n '131,174p' cmd/tq/main.go        # 27-entry commands map on stdlib flag
rg -c '"[a-z-]+":\s+cmd[A-Z]' cmd/tq/main.go  # → 27 command map entries
wc -l cmd/tq/main.go                                                   # → 4397
git -C /home/lars/projects/cmdguard tag -l 'v5*' 'v4*' | sort -V | tail -3  # v4.3.1 v5.0.0 v5.1.0
git -C /home/lars/projects/cmdguard log -1 --format='%ad %h' --date=short v5.1.0  # 2026-11-11 671c128
git -C /home/lars/projects/cmdguard log v5.0.0..v5.1.0 --oneline | grep -cv 'chore: auto-commit'  # non-chore volume; feat: only the strict-args addition (CHANGELOG [5.1.0])
sed -n '/^## \[5.1.0\]/,/^## \[5.0.0\]/p' /home/lars/projects/cmdguard/CHANGELOG.md | head -25  # what shipped
git -C /home/lars/projects/go-credentials-store log -1 --format='%ad %h %s' --date=short  # 2026-11-11 c423a30 bump cmdguard/v5 to v5.1.0
head -20 /home/lars/.config/crush/skills/cmdguard/SKILL.md             # v5 capability summary
```
