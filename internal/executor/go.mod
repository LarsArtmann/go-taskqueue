module github.com/larsartmann/go-taskqueue/internal/executor

go 1.26.7

require (
	github.com/larsartmann/go-retry v0.5.0
	github.com/larsartmann/go-taskqueue/internal/task v0.2.0
)

require github.com/larsartmann/go-error-family v0.10.0 // indirect

replace github.com/larsartmann/go-taskqueue/internal/task => ../task
