module github.com/larsartmann/go-taskqueue/examples/embed

go 1.26.7

require (
	github.com/jackc/pgx/v5 v5.11.0
	github.com/larsartmann/go-taskqueue/executor v0.3.0
	github.com/larsartmann/go-taskqueue/queue v0.3.0
	github.com/larsartmann/go-taskqueue/queue/postgres v0.3.0
	github.com/larsartmann/go-taskqueue/queue/sqlite v0.3.0
	github.com/larsartmann/go-taskqueue/task v0.3.0
	github.com/larsartmann/go-taskqueue/worker v0.3.0
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/larsartmann/go-error-family v0.10.0 // indirect
	github.com/larsartmann/go-retry v0.5.0 // indirect
	github.com/larsartmann/go-taskqueue/internal/executor v0.3.0 // indirect
	github.com/larsartmann/go-taskqueue/internal/journal v0.3.0 // indirect
	github.com/larsartmann/go-taskqueue/internal/queue v0.3.0 // indirect
	github.com/larsartmann/go-taskqueue/internal/queue/postgres v0.3.0 // indirect
	github.com/larsartmann/go-taskqueue/internal/queue/sqlite v0.3.0 // indirect
	github.com/larsartmann/go-taskqueue/internal/task v0.3.0 // indirect
	github.com/larsartmann/go-taskqueue/internal/worker v0.3.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	modernc.org/libc v1.75.7 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
	modernc.org/sqlite v1.58.0 // indirect
)

// Local-dev replaces: exactly what an external consumer needs ONLY until
// the facade tags ship on the proxy; after that, `go get` resolves them
// and these lines can go. Replace directives apply from the main module,
// so every module in the transitive graph (facades AND the internal
// modules they require) needs one here.
replace (
	github.com/larsartmann/go-taskqueue/executor => ../../executor
	github.com/larsartmann/go-taskqueue/internal/executor => ../../internal/executor
	github.com/larsartmann/go-taskqueue/internal/journal => ../../internal/journal
	github.com/larsartmann/go-taskqueue/internal/queue => ../../internal/queue
	github.com/larsartmann/go-taskqueue/internal/queue/postgres => ../../internal/queue/postgres
	github.com/larsartmann/go-taskqueue/internal/queue/sqlite => ../../internal/queue/sqlite
	github.com/larsartmann/go-taskqueue/internal/task => ../../internal/task
	github.com/larsartmann/go-taskqueue/internal/worker => ../../internal/worker
	github.com/larsartmann/go-taskqueue/queue => ../../queue
	github.com/larsartmann/go-taskqueue/queue/postgres => ../../queue/postgres
	github.com/larsartmann/go-taskqueue/queue/sqlite => ../../queue/sqlite
	github.com/larsartmann/go-taskqueue/task => ../../task
	github.com/larsartmann/go-taskqueue/worker => ../../worker
)
