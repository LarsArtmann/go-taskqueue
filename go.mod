module github.com/larsartmann/go-taskqueue

go 1.26.7

require (
	github.com/a-h/templ v0.3.1020
	github.com/larsartmann/go-sse v0.6.0
	github.com/larsartmann/go-sse/ssetest v0.3.0
	github.com/larsartmann/templ-components v1.17.0
	github.com/larsartmann/templ-components/icons v1.17.0
	github.com/larsartmann/templ-components/utils v1.17.0
	golang.org/x/sync v0.23.0
	modernc.org/sqlite v1.59.0 // indirect
)

require (
	github.com/LarsArtmann/go-crush-data v0.4.0 // indirect
	github.com/Oudwins/tailwind-merge-go v0.2.3 // indirect
	github.com/a-h/parse v0.0.0-20250122154542-74294addb73e // indirect
	github.com/andybalholm/brotli v1.1.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cli/browser v1.3.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/fatih/color v1.19.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.11.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/larsartmann/go-branded-id v0.5.1 // indirect
	github.com/larsartmann/go-datastar v0.5.0 // indirect
	github.com/larsartmann/go-error-family v0.10.1 // indirect
	github.com/larsartmann/go-retry v0.6.0 // indirect
	github.com/larsartmann/templ-components/datastar v1.17.0 // indirect
	github.com/larsartmann/templ-components/htmx v1.17.0 // indirect
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/natefinch/atomic v1.0.1 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/samber/do/v2 v2.1.0 // indirect
	github.com/samber/go-type-to-string v1.8.0 // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	golang.org/x/tools v0.49.0 // indirect
	modernc.org/libc v1.76.0 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
)

require (
	github.com/larsartmann/go-datastar/static v0.5.0
	github.com/larsartmann/go-health v0.1.3
	github.com/larsartmann/go-health-dashboard v0.8.1
	github.com/larsartmann/go-taskqueue/internal/executor v0.3.0
	github.com/larsartmann/go-taskqueue/internal/journal v0.3.0
	github.com/larsartmann/go-taskqueue/internal/queue v0.3.0
	github.com/larsartmann/go-taskqueue/internal/queue/postgres v0.3.0
	github.com/larsartmann/go-taskqueue/internal/queue/sqlite v0.3.0
	github.com/larsartmann/go-taskqueue/internal/task v0.3.0
	github.com/larsartmann/go-taskqueue/internal/worker v0.3.0
)

replace github.com/larsartmann/go-taskqueue/internal/task => ./internal/task

replace github.com/larsartmann/go-taskqueue/internal/journal => ./internal/journal

replace github.com/larsartmann/go-taskqueue/internal/executor => ./internal/executor

replace github.com/larsartmann/go-taskqueue/internal/queue => ./internal/queue

replace github.com/larsartmann/go-taskqueue/internal/worker => ./internal/worker

tool github.com/a-h/templ/cmd/templ

replace github.com/larsartmann/go-taskqueue/internal/queue/sqlite => ./internal/queue/sqlite

replace github.com/larsartmann/go-taskqueue/internal/queue/postgres => ./internal/queue/postgres

replace github.com/larsartmann/go-taskqueue/internal/journal/cqrs => ./internal/journal/cqrs
