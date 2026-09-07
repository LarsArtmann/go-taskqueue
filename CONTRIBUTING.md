# Contributing

Thanks for your interest in contributing!

## How to Contribute

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

## Development Setup

Every push is gated on CI (`.github/workflows/ci.yml`). The pre-push gate is
one command:

```sh
./scripts/ci-local.sh
```

It replicates the full CI sequence — vet → build → `GOOS=windows` → race
tests → gofmt → advisory lint → harvest-parse guard → web UI smoke →
doc-reference check, then `git add -A` + `nix build` + `nix flake check` on
a fully tracked tree — and must be green before every push (a stale-tree nix
check plus an unverified push once produced a red master, 2026-09-07). Run
it right before `git push`; it stages untracked files so nix measures the
same tree CI will see. The individual gates, when you need one in isolation:

```sh
go vet ./...
go build ./...
GOOS=windows go build ./...    # cross-compile gate
go test ./... -count=1 -race -timeout 120s
test -z "$(gofmt -l .)"
./scripts/smoke/webui.sh       # live web UI smoke (no browser needed)
./scripts/check-doc-refs.sh    # doc-cited paths must exist
nix build && nix flake check   # reproducible build + vendor-hash gate
```

golangci-lint (`golangci-lint run ./...`, config `.golangci.yml`) runs
advisory in CI (non-blocking): the repo carries a ~400-finding baseline
(documented in AGENTS.md). Don't add new findings in code you touch, and
fixing the findings of a function you are already editing is welcome —
never mass-"fix" the baseline.

After editing any `.templ` source, regenerate the committed output:
`go tool templ generate` (generated `*_templ.go` files are committed, and
`templ fmt` owns `.templ` formatting in the treefmt gate).

Markdown, JSON and YAML are formatted with dprint (config: `dprint.json`,
available in the flake devShell): `dprint fmt` before you commit docs.

For changes to the agent-pool loop, also run the live multi-repo smoke
(stub agents, no API cost), and keep `TODO_LIST.md` harvester-parseable
(CI runs the parse guard):

```sh
./scripts/smoke/multi-repo.sh
go test ./internal/harvest/ -run TestRepoTodoListParses
```

## Reporting Issues

Please use GitHub Issues to report bugs or request features. For security
reports, see SECURITY.md (private advisories, not public issues).
