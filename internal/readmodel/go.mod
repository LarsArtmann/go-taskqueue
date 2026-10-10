module github.com/larsartmann/go-taskqueue/internal/readmodel

go 1.27.1

require (
	github.com/larsartmann/go-cqrs-lite/event/v4 v4.13.1
	github.com/larsartmann/go-cqrs-lite/id/v4 v4.7.2
	github.com/larsartmann/go-cqrs-lite/metaengine/sqliteengine/v4 v4.5.2
	github.com/larsartmann/go-cqrs-lite/metaengine/v4 v4.17.0
	github.com/larsartmann/go-cqrs-lite/projection/v4 v4.4.2
	github.com/larsartmann/go-cqrs-lite/projectionhost/v4 v4.5.3
	github.com/larsartmann/go-cqrs-lite/record/v4 v4.6.2
	github.com/larsartmann/go-error-family v0.11.0
	github.com/larsartmann/go-taskqueue/internal/config v0.3.3
	github.com/larsartmann/go-taskqueue/internal/journal v0.4.0
	github.com/larsartmann/go-taskqueue/internal/journal/cqrs v0.3.3
	github.com/larsartmann/go-taskqueue/internal/queue v0.3.3
	github.com/larsartmann/go-taskqueue/internal/queue/sqlite v0.3.3
	github.com/larsartmann/go-taskqueue/internal/task v0.3.3
	github.com/oklog/ulid/v2 v2.1.2
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.6 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/larsartmann/go-branded-id v0.7.0 // indirect
	github.com/larsartmann/go-codec v0.3.1 // indirect
	github.com/larsartmann/go-cqrs-lite/claiming/v4 v4.0.2 // indirect
	github.com/larsartmann/go-cqrs-lite/dedup/v4 v4.2.4 // indirect
	github.com/larsartmann/go-cqrs-lite/dispatcher/v4 v4.5.2 // indirect
	github.com/larsartmann/go-cqrs-lite/metadata/v4 v4.7.3 // indirect
	github.com/larsartmann/go-cqrs-lite/otel/v4 v4.5.2 // indirect
	github.com/larsartmann/go-cqrs-lite/queue/sqlite/v4 v4.0.3 // indirect
	github.com/larsartmann/go-cqrs-lite/queue/v4 v4.0.3 // indirect
	github.com/larsartmann/go-flightrecorder v0.2.1 // indirect
	github.com/larsartmann/go-sse v0.6.2 // indirect
	github.com/larsartmann/go-taskqueue/internal/queue/companion v0.4.0 // indirect
	github.com/larsartmann/go-taskqueue/internal/queue/sqlitev4 v0.3.3 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.1.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.47.0 // indirect
	go.opentelemetry.io/otel/log v1.47.0 // indirect
	go.opentelemetry.io/otel/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/sdk v1.47.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/trace v1.47.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	modernc.org/libc v1.77.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
	modernc.org/sqlite v1.60.1 // indirect
)

replace github.com/larsartmann/go-taskqueue/internal/config => ../config

replace github.com/larsartmann/go-taskqueue/internal/task => ../task

replace github.com/larsartmann/go-taskqueue/internal/journal => ../journal

replace github.com/larsartmann/go-taskqueue/internal/queue => ../queue

replace github.com/larsartmann/go-taskqueue/internal/queue/sqlite => ../queue/sqlite

replace github.com/larsartmann/go-taskqueue/internal/queue/sqlitev4 => ../queue/sqlitev4

replace github.com/larsartmann/go-taskqueue/internal/queue/companion => ../queue/companion

replace github.com/larsartmann/go-taskqueue/internal/journal/cqrs => ../journal/cqrs
