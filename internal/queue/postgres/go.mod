module github.com/larsartmann/go-taskqueue/internal/queue/postgres

go 1.27.3

require (
	github.com/jackc/pgx/v5 v5.12.0
	github.com/larsartmann/go-taskqueue/internal/queue/postgresv4 v0.4.1
)

require (
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/fxamacker/cbor/v2 v2.10.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.3 // indirect
	github.com/larsartmann/go-branded-id v0.7.0 // indirect
	github.com/larsartmann/go-codec v0.3.3 // indirect
	github.com/larsartmann/go-cqrs-lite/claiming/v4 v4.0.2 // indirect
	github.com/larsartmann/go-cqrs-lite/dedup/v4 v4.3.0 // indirect
	github.com/larsartmann/go-cqrs-lite/metaengine/v4 v4.20.0 // indirect
	github.com/larsartmann/go-cqrs-lite/queue/postgres/v4 v4.0.3 // indirect
	github.com/larsartmann/go-cqrs-lite/queue/v4 v4.0.3 // indirect
	github.com/larsartmann/go-cqrs-lite/record/v4 v4.6.2 // indirect
	github.com/larsartmann/go-error-family v0.12.0 // indirect
	github.com/larsartmann/go-sse v0.6.2 // indirect
	github.com/larsartmann/go-taskqueue/internal/journal v0.4.1 // indirect
	github.com/larsartmann/go-taskqueue/internal/queue v0.4.1 // indirect
	github.com/larsartmann/go-taskqueue/internal/queue/companion v0.4.1 // indirect
	github.com/larsartmann/go-taskqueue/internal/task v0.4.1 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/text v0.43.0 // indirect
)

replace github.com/larsartmann/go-taskqueue/internal/journal => ../../journal

replace github.com/larsartmann/go-taskqueue/internal/queue => ..

replace github.com/larsartmann/go-taskqueue/internal/task => ../../task

replace github.com/larsartmann/go-taskqueue/internal/queue/postgresv4 => ../postgresv4

replace github.com/larsartmann/go-taskqueue/internal/queue/companion => ../companion
