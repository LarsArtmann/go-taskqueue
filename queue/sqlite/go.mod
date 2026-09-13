module github.com/larsartmann/go-taskqueue/queue/sqlite

go 1.26.7

require (
	github.com/larsartmann/go-taskqueue/internal/queue v0.2.0
	github.com/larsartmann/go-taskqueue/internal/queue/sqlite v0.2.0
	github.com/larsartmann/go-taskqueue/internal/task v0.2.0
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/larsartmann/go-error-family v0.10.0 // indirect
	github.com/larsartmann/go-retry v0.5.0 // indirect
	github.com/larsartmann/go-taskqueue/internal/journal v0.2.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/sys v0.48.0 // indirect
	modernc.org/libc v1.75.7 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
	modernc.org/sqlite v1.58.0 // indirect
)

replace github.com/larsartmann/go-taskqueue/internal/journal => ../../internal/journal

replace github.com/larsartmann/go-taskqueue/internal/queue => ../../internal/queue

replace github.com/larsartmann/go-taskqueue/internal/queue/sqlite => ../../internal/queue/sqlite

replace github.com/larsartmann/go-taskqueue/internal/task => ../../internal/task
