module github.com/larsartmann/go-taskqueue/queue

go 1.27.4

require github.com/larsartmann/go-taskqueue/internal/queue v0.4.2

require (
	github.com/larsartmann/go-cqrs-lite/queue/v4 v4.0.3 // indirect
	github.com/larsartmann/go-taskqueue/internal/journal v0.4.2 // indirect
	github.com/larsartmann/go-taskqueue/internal/task v0.4.2 // indirect
)

replace github.com/larsartmann/go-taskqueue/internal/journal => ../internal/journal

replace github.com/larsartmann/go-taskqueue/internal/queue => ../internal/queue

replace github.com/larsartmann/go-taskqueue/internal/task => ../internal/task
