module github.com/larsartmann/go-taskqueue/executor

go 1.27

require (
	github.com/larsartmann/go-taskqueue/internal/executor v0.3.0
	github.com/larsartmann/go-taskqueue/internal/journal v0.3.0
	github.com/larsartmann/go-taskqueue/internal/queue v0.3.0
	github.com/larsartmann/go-taskqueue/internal/task v0.3.0
)

require (
	github.com/LarsArtmann/go-crush-data v0.4.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/larsartmann/go-error-family v0.10.1 // indirect
	github.com/larsartmann/go-retry v0.6.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/sys v0.48.0 // indirect
	modernc.org/libc v1.76.0 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
	modernc.org/sqlite v1.59.0 // indirect
)

replace github.com/larsartmann/go-taskqueue/internal/executor => ../internal/executor

replace github.com/larsartmann/go-taskqueue/internal/journal => ../internal/journal

replace github.com/larsartmann/go-taskqueue/internal/queue => ../internal/queue

replace github.com/larsartmann/go-taskqueue/internal/task => ../internal/task
