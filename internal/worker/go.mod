module github.com/larsartmann/go-taskqueue/internal/worker

go 1.27.3

require (
	github.com/LarsArtmann/go-crush-data v0.5.0 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/fxamacker/cbor/v2 v2.10.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/larsartmann/go-branded-id v0.7.0 // indirect
	github.com/larsartmann/go-codec v0.3.3 // indirect
	github.com/larsartmann/go-cqrs-lite/claiming/v4 v4.0.2 // indirect
	github.com/larsartmann/go-cqrs-lite/dedup/v4 v4.3.0 // indirect
	github.com/larsartmann/go-cqrs-lite/metaengine/v4 v4.20.0 // indirect
	github.com/larsartmann/go-cqrs-lite/queue/sqlite/v4 v4.0.3 // indirect
	github.com/larsartmann/go-cqrs-lite/queue/v4 v4.0.3 // indirect
	github.com/larsartmann/go-cqrs-lite/record/v4 v4.6.2 // indirect
	github.com/larsartmann/go-error-family v0.12.0 // indirect
	github.com/larsartmann/go-sse v0.6.2 // indirect
	github.com/larsartmann/go-taskqueue/internal/config v0.4.1 // indirect
	github.com/larsartmann/go-taskqueue/internal/queue/companion v0.4.1 // indirect
	github.com/larsartmann/go-taskqueue/internal/queue/sqlitev4 v0.4.1 // indirect
	github.com/larsartmann/go-taskqueue/internal/retryx v0.4.1 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.1.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/sys v0.49.0 // indirect
	modernc.org/libc v1.80.0 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
	modernc.org/sqlite v1.61.1 // indirect
)

replace github.com/larsartmann/go-taskqueue/internal/executor => ../executor

replace github.com/larsartmann/go-taskqueue/internal/journal => ../journal

replace github.com/larsartmann/go-taskqueue/internal/queue => ../queue

replace github.com/larsartmann/go-taskqueue/internal/task => ../task

replace github.com/larsartmann/go-taskqueue/internal/queue/sqlite => ../queue/sqlite

require (
	github.com/larsartmann/go-retry v0.9.0
	github.com/larsartmann/go-taskqueue/internal/executor v0.4.1
	github.com/larsartmann/go-taskqueue/internal/journal v0.4.1
	github.com/larsartmann/go-taskqueue/internal/queue v0.4.1
	github.com/larsartmann/go-taskqueue/internal/queue/sqlite v0.3.3
	github.com/larsartmann/go-taskqueue/internal/task v0.4.1
)

replace github.com/larsartmann/go-taskqueue/internal/queue/sqlitev4 => ../queue/sqlitev4

replace github.com/larsartmann/go-taskqueue/internal/queue/companion => ../queue/companion

replace github.com/larsartmann/go-taskqueue/internal/retryx => ../retryx

replace github.com/larsartmann/go-taskqueue/internal/testastguard => ../testastguard
