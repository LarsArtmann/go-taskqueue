# Draft: charmbracelet/crush — server-mode bash tool drops the workspace's client-forwarded env

Filed: (pending) · Title: `Server mode: bash tool runs without the workspace's client env — only the server process env`

## Proposed title

`Server mode: the bash tool sees only the server's env, not the client-forwarded workspace env`

## Body

```markdown
> [!NOTE]
> This filing was drafted by GLM-4.6 via Crush from an AI-run investigation of crush's client/server mode; the filing itself is a step in an owner-ratified adoption plan for that mode. When the failure traces to an external report, that source is linked in the body.
>
> - [ ] MANUALLY REVIEWED by `@Lars Artmann` at `[<date-time>]`

**TL;DR:** In server mode, a bash tool call executes with the SERVER process environment only — the client's env (forwarded on the workspace) never reaches tool exec, so `$FOO` set by the client is silently empty. The interactive `$` shell already merges the workspace env; the bash tool should do the same.

## Problem

Verified on crush v0.99.0 (installed) and re-verified against `main` at `ef4993a390cce10291311de0def4029c835baabc` — same code shape.

Boot a server, run a client with `CRUSH_CLIENT_SERVER=1 crush run -H unix:///tmp/s.sock` and a prompt that runs `echo "[$TQ_RESULT_FILE]"` via the bash tool (with `TQ_RESULT_FILE` exported on the CLIENT). The tool prints `[]` — the client env never arrived. A variable exported on the SERVER process before `crush server` DOES arrive, which is how we found the split.

The tool exec path never passes an env, so `shell.Run` falls back to the server's `os.Environ()`:

```go
// internal/agent/tools/bash.go (executeBashCommand)
bgShell, err := bgManager.Start(context.Background(), execWorkingDir, blockFuncsToUse, params.Command, params.Description, maxBackgroundJobs)

// internal/shell/background.go:66
func (m *BackgroundShellManager) Start(ctx context.Context, workingDir string, blockFuncs []BlockFunc, command, description string, maxJobs int) (*BackgroundShell, error)

// internal/shell/shell.go:95-97
env := opts.Env
if env == nil {
    env = os.Environ()
}
```

Meanwhile the interactive `$` shell command DOES merge the workspace env:

```go
// internal/backend/agent.go:315 (RunShellCommand)
Env: append(os.Environ(), ws.Env...),
```

So the same variable is visible in `$` but empty in a bash tool call of the same routed session.

## Impact

Anything a headless orchestrator passes per-run through the environment is invisible to the agent's tool calls in server mode — the exact use case `CRUSH_CLIENT_SERVER` exists for. In our setup (a task queue routing headless runs through a shared `crush server`), the per-run result-file channel `$TQ_RESULT_FILE` silently breaks under routing while working in-process. There is no error and no warning — the tool just sees an empty variable.

## Fix / Proposal

Thread the workspace env into tool exec the way `RunShellCommand` already does — e.g. `BackgroundShellManager.Start` taking an env (or the bash tool building `append(os.Environ(), ws.Env...)` at its call site). If merging is a policy decision (secrets isolation?), then at minimum document that tool exec sees only the server env, and consider a warning when a workspace carries a non-empty `Env` that tool exec ignores.

Repro: two-sided env probe (client var vs server var) at https://github.com/LarsArtmann/go-taskqueue/blob/master/scripts/smoke/crush-server-route.sh — the env-gap canary leg; it flips red the moment propagation starts working.

💘 Generated with Crush
```

## Filing

```bash
gh issue create --repo charmbracelet/crush \
  --title "Server mode: the bash tool sees only the server's env, not the client-forwarded workspace env" \
  --body-file <extracted-body-file>
```

Extract the fenced body (split on the fence markers), byte-count it, post with a REAL file (never `--body-file -`), verify with `gh issue view`.
