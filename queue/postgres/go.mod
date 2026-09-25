module github.com/larsartmann/go-taskqueue/queue/postgres

go 1.27.1

require (
	github.com/jackc/pgx/v5 v5.11.0
	github.com/larsartmann/go-taskqueue/internal/queue/postgres v0.3.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/larsartmann/go-branded-id v0.6.0 // indirect
	github.com/larsartmann/go-cqrs-lite/claiming/v4 v4.0.0 // indirect
	github.com/larsartmann/go-cqrs-lite/dedup/v4 v4.2.2 // indirect
	github.com/larsartmann/go-cqrs-lite/metaengine/v4 v4.14.0 // indirect
	github.com/larsartmann/go-cqrs-lite/queue/postgres/v4 v4.0.0 // indirect
	github.com/larsartmann/go-cqrs-lite/queue/v4 v4.0.0 // indirect
	github.com/larsartmann/go-cqrs-lite/record/v4 v4.5.1 // indirect
	github.com/larsartmann/go-error-family v0.10.1 // indirect
	github.com/larsartmann/go-sse v0.6.0 // indirect
	github.com/larsartmann/go-taskqueue/internal/journal v0.3.0 // indirect
	github.com/larsartmann/go-taskqueue/internal/queue v0.3.0 // indirect
	github.com/larsartmann/go-taskqueue/internal/queue/postgresv4 v0.3.0 // indirect
	github.com/larsartmann/go-taskqueue/internal/task v0.3.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/text v0.42.0 // indirect
)

replace github.com/larsartmann/go-taskqueue/internal/journal => ../../internal/journal

replace github.com/larsartmann/go-taskqueue/internal/queue => ../../internal/queue

replace github.com/larsartmann/go-taskqueue/internal/queue/postgres => ../../internal/queue/postgres

replace github.com/larsartmann/go-taskqueue/internal/task => ../../internal/task

replace github.com/larsartmann/go-taskqueue/internal/queue/postgresv4 => ../../internal/queue/postgresv4
