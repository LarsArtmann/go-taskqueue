module github.com/larsartmann/go-taskqueue/internal/worker

go 1.26.7

require (
	github.com/larsartmann/go-taskqueue/internal/executor v0.0.0
	github.com/larsartmann/go-taskqueue/internal/journal v0.0.0
	github.com/larsartmann/go-taskqueue/internal/queue v0.0.0
	github.com/larsartmann/go-taskqueue/internal/task v0.0.0
)

replace github.com/larsartmann/go-taskqueue/internal/executor => ../executor

replace github.com/larsartmann/go-taskqueue/internal/journal => ../journal

replace github.com/larsartmann/go-taskqueue/internal/queue => ../queue

replace github.com/larsartmann/go-taskqueue/internal/task => ../task
