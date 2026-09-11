# Stale re-dispatch close-out — f16 absolute-repos guard bypass (2026-09-11)

Re-dispatched TODO_LIST item: "checkProjectsDir should not require the
projects dir when every --repos entry is absolute (02:00 f16;
cmd/tq/agentpool.go + test)".

Already shipped in commit 9804683: `allReposAbsolute` (cmd/tq/seeds.go)
gates the `checkProjectsDir` call in the agent-pool options
(cmd/tq/agentpool.go:234), with table-driven coverage
(TestAllReposAbsolute, TestCheckProjectsDir in cmd/tq/seeds_test.go).

Re-verified green today (root module, GOEXPERIMENT=jsonv2):
go build ./..., go vet ./..., full cmd/tq test suite -count=1, plus the
guard tests specifically. TODO_LIST.md item already marked [x]. No code
changes needed.
