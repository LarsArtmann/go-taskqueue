module github.com/larsartmann/go-taskqueue/examples/embed

go 1.27.4

require (
	github.com/jackc/pgx/v5 v5.12.0
	github.com/larsartmann/go-taskqueue/executor v0.4.2
	github.com/larsartmann/go-taskqueue/queue v0.4.2
	github.com/larsartmann/go-taskqueue/queue/postgres v0.4.2
	github.com/larsartmann/go-taskqueue/queue/sqlite v0.4.2
	github.com/larsartmann/go-taskqueue/task v0.4.2
	github.com/larsartmann/go-taskqueue/worker v0.4.2
)

require (
	github.com/LarsArtmann/go-crush-data v0.6.0 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/fxamacker/cbor/v2 v2.10.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.3 // indirect
	github.com/larsartmann/go-branded-id v0.7.1 // indirect
	github.com/larsartmann/go-codec v0.3.3 // indirect
	github.com/larsartmann/go-cqrs-lite/claiming/v4 v4.0.2 // indirect
	github.com/larsartmann/go-cqrs-lite/dedup/v4 v4.3.0 // indirect
	github.com/larsartmann/go-cqrs-lite/metaengine/v4 v4.23.0 // indirect
	github.com/larsartmann/go-cqrs-lite/queue/postgres/v4 v4.0.3 // indirect
	github.com/larsartmann/go-cqrs-lite/queue/sqlite/v4 v4.0.3 // indirect
	github.com/larsartmann/go-cqrs-lite/queue/v4 v4.0.3 // indirect
	github.com/larsartmann/go-cqrs-lite/record/v4 v4.6.2 // indirect
	github.com/larsartmann/go-error-family v0.12.0 // indirect
	github.com/larsartmann/go-retry v0.9.0 // indirect
	github.com/larsartmann/go-sse v0.7.0 // indirect
	github.com/larsartmann/go-taskqueue/internal/config v0.4.2 // indirect
	github.com/larsartmann/go-taskqueue/internal/executor v0.4.2 // indirect
	github.com/larsartmann/go-taskqueue/internal/journal v0.4.2 // indirect
	github.com/larsartmann/go-taskqueue/internal/queue v0.4.2 // indirect
	github.com/larsartmann/go-taskqueue/internal/queue/companion v0.4.2 // indirect
	github.com/larsartmann/go-taskqueue/internal/queue/postgres v0.4.2 // indirect
	github.com/larsartmann/go-taskqueue/internal/queue/postgresv4 v0.4.2 // indirect
	github.com/larsartmann/go-taskqueue/internal/queue/sqlite v0.4.2 // indirect
	github.com/larsartmann/go-taskqueue/internal/queue/sqlitev4 v0.4.2 // indirect
	github.com/larsartmann/go-taskqueue/internal/retryx v0.4.2 // indirect
	github.com/larsartmann/go-taskqueue/internal/task v0.4.2 // indirect
	github.com/larsartmann/go-taskqueue/internal/worker v0.4.2 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.1.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	modernc.org/libc v1.80.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
	modernc.org/sqlite v1.61.1 // indirect
)

// Local-dev replaces (OWNER RULING 2026-11-03, task
// 000001a19fbef62f653cfc535d6c00000000 §g1: KEEP-FOREVER): the facade
// tags now resolve on the proxy (CI consumer-install proves the tag
// path), but these lines stay — they make this example an adopter of the
// LOCAL tree, which is what ci-local's "adopter on-ramp rot guard" (and
// check-embed-tidy) actually guards; dropping them would turn the build
// into a stale-tag canary. Replace directives apply from the main
// module, so every module in the transitive graph (facades AND the
// internal modules they require) needs one here.
replace (
	github.com/larsartmann/go-taskqueue/executor => ../../executor
	github.com/larsartmann/go-taskqueue/internal/config => ../../internal/config
	github.com/larsartmann/go-taskqueue/internal/executor => ../../internal/executor
	github.com/larsartmann/go-taskqueue/internal/journal => ../../internal/journal
	github.com/larsartmann/go-taskqueue/internal/queue => ../../internal/queue
	github.com/larsartmann/go-taskqueue/internal/queue/companion => ../../internal/queue/companion
	github.com/larsartmann/go-taskqueue/internal/queue/postgres => ../../internal/queue/postgres
	github.com/larsartmann/go-taskqueue/internal/queue/postgresv4 => ../../internal/queue/postgresv4
	github.com/larsartmann/go-taskqueue/internal/queue/sqlite => ../../internal/queue/sqlite
	github.com/larsartmann/go-taskqueue/internal/queue/sqlitev4 => ../../internal/queue/sqlitev4
	github.com/larsartmann/go-taskqueue/internal/retryx => ../../internal/retryx
	github.com/larsartmann/go-taskqueue/internal/task => ../../internal/task
	github.com/larsartmann/go-taskqueue/internal/testastguard => ../../internal/testastguard
	github.com/larsartmann/go-taskqueue/internal/worker => ../../internal/worker
	github.com/larsartmann/go-taskqueue/queue => ../../queue
	github.com/larsartmann/go-taskqueue/queue/postgres => ../../queue/postgres
	github.com/larsartmann/go-taskqueue/queue/sqlite => ../../queue/sqlite
	github.com/larsartmann/go-taskqueue/task => ../../task
	github.com/larsartmann/go-taskqueue/worker => ../../worker
)
