module github.com/larsartmann/go-taskqueue/internal/queue

go 1.27.1

require (
	github.com/larsartmann/go-taskqueue/internal/journal v0.3.3
	github.com/larsartmann/go-taskqueue/internal/task v0.3.3
)

require github.com/larsartmann/go-cqrs-lite/queue/v4 v4.0.3 // indirect

replace github.com/larsartmann/go-taskqueue/internal/journal => ../journal

replace github.com/larsartmann/go-taskqueue/internal/task => ../task
