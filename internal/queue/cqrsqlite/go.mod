module github.com/larsartmann/go-taskqueue/internal/queue/cqrsqlite

go 1.27

require (
	github.com/larsartmann/go-cqrs-lite/queue/sqlite/v4 v4.0.0
	github.com/larsartmann/go-taskqueue/internal/journal v0.3.0
	github.com/larsartmann/go-taskqueue/internal/queue v0.3.0
	github.com/larsartmann/go-taskqueue/internal/task v0.3.0
)

replace github.com/larsartmann/go-taskqueue/internal/queue => ..

replace github.com/larsartmann/go-taskqueue/internal/task => ../../task

replace github.com/larsartmann/go-taskqueue/internal/journal => ../../journal
