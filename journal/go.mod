module github.com/larsartmann/go-taskqueue/journal

go 1.27.1

require github.com/larsartmann/go-taskqueue/internal/journal v0.4.0

require github.com/larsartmann/go-cqrs-lite/queue/v4 v4.0.3 // indirect

replace github.com/larsartmann/go-taskqueue/internal/journal => ../internal/journal
