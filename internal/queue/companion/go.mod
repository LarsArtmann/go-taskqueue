module github.com/larsartmann/go-taskqueue/internal/queue/companion

go 1.27.1

require (
	github.com/larsartmann/go-cqrs-lite/queue/v4 v4.0.0
	github.com/larsartmann/go-taskqueue/internal/journal v0.3.0
	github.com/larsartmann/go-taskqueue/internal/queue v0.3.0
	github.com/larsartmann/go-taskqueue/internal/task v0.3.0
)

require github.com/larsartmann/go-error-family v0.10.1 // indirect

replace github.com/larsartmann/go-taskqueue/internal/queue => ..

replace github.com/larsartmann/go-taskqueue/internal/task => ../../task

replace github.com/larsartmann/go-taskqueue/internal/journal => ../../journal
