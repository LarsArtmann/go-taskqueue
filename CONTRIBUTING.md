# Contributing

Thanks for your interest in contributing!

## How to Contribute

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

## Development Setup

Every push is gated on CI (`.github/workflows/ci.yml`). Run the same gates
locally before pushing:

```sh
go vet ./...
go build ./...
go test ./... -count=1 -race -timeout 120s
test -z "$(gofmt -l .)"
golangci-lint run ./...        # config: .golangci.yml
GOOS=windows go build ./...    # cross-compile gate
./scripts/check-doc-refs.sh    # doc-cited paths must exist
nix build && nix flake check   # reproducible build + vendor-hash gate
```

Markdown, JSON and YAML are formatted with dprint (config: `dprint.json`,
available in the flake devShell): `dprint fmt` before you commit docs.

For changes to the agent-pool loop, also run the live multi-repo smoke
(stub agents, no API cost):

```sh
./scripts/smoke/multi-repo.sh
```

## Reporting Issues

Please use GitHub Issues to report bugs or request features. For security
reports, see SECURITY.md (private advisories, not public issues).
