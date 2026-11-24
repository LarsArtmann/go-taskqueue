module github.com/larsartmann/go-taskqueue/internal/mcpserver

go 1.27.4

require (
	github.com/larsartmann/go-taskqueue/internal/journal v0.4.3
	github.com/larsartmann/go-taskqueue/internal/queue v0.4.3
	github.com/larsartmann/go-taskqueue/internal/task v0.4.3
)

require github.com/larsartmann/go-cqrs-lite/queue/v4 v4.0.3 // indirect

replace github.com/larsartmann/go-taskqueue/internal/journal => ../journal

replace github.com/larsartmann/go-taskqueue/internal/queue => ../queue

replace github.com/larsartmann/go-taskqueue/internal/task => ../task
