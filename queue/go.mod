module github.com/larsartmann/go-taskqueue/queue

go 1.26.7

require github.com/larsartmann/go-taskqueue/internal/queue v0.3.0

require (
	github.com/larsartmann/go-taskqueue/internal/journal v0.3.0 // indirect
	github.com/larsartmann/go-taskqueue/internal/task v0.3.0 // indirect
)

replace github.com/larsartmann/go-taskqueue/internal/journal => ../internal/journal

replace github.com/larsartmann/go-taskqueue/internal/queue => ../internal/queue

replace github.com/larsartmann/go-taskqueue/internal/task => ../internal/task
