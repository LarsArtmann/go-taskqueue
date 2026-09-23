module github.com/larsartmann/go-taskqueue/internal/queue/sqlitev4

go 1.27

require (
	github.com/larsartmann/go-taskqueue/internal/task v0.3.0
	github.com/larsartmann/go-taskqueue/internal/journal v0.3.0
	github.com/larsartmann/go-taskqueue/internal/queue v0.3.0
)

replace github.com/larsartmann/go-taskqueue/internal/task => ../../task
replace github.com/larsartmann/go-taskqueue/internal/journal => ../../journal
replace github.com/larsartmann/go-taskqueue/internal/queue => ..
