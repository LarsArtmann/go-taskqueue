module github.com/larsartmann/go-taskqueue/internal/queue

go 1.26.7

require (
	github.com/larsartmann/go-taskqueue/internal/journal v0.2.0
	github.com/larsartmann/go-taskqueue/internal/task v0.2.0
)

replace github.com/larsartmann/go-taskqueue/internal/journal => ../journal

replace github.com/larsartmann/go-taskqueue/internal/task => ../task
