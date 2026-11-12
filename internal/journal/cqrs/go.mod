module github.com/larsartmann/go-taskqueue/internal/journal/cqrs

go 1.27.3

require (
	github.com/larsartmann/go-codec v0.3.3
	github.com/larsartmann/go-cqrs-lite/event/v4 v4.15.2
	github.com/larsartmann/go-cqrs-lite/id/v4 v4.7.5
	github.com/larsartmann/go-taskqueue/internal/journal v0.4.2
	github.com/oklog/ulid/v2 v2.1.2
)

require (
	github.com/fxamacker/cbor/v2 v2.10.0 // indirect
	github.com/gkampitakis/go-snaps v0.5.25 // indirect
	github.com/google/pprof v0.0.0-20261016150608-1facf8f91b50 // indirect
	github.com/larsartmann/go-branded-id v0.7.1 // indirect
	github.com/larsartmann/go-cqrs-lite/dispatcher/v4 v4.5.2 // indirect
	github.com/larsartmann/go-cqrs-lite/metadata/v4 v4.7.3 // indirect
	github.com/larsartmann/go-cqrs-lite/queue/v4 v4.0.3 // indirect
	github.com/larsartmann/go-cqrs-lite/record/v4 v4.6.2 // indirect
	github.com/larsartmann/go-cqrs-lite/schema/v4 v4.7.1 // indirect
	github.com/larsartmann/go-error-family v0.12.0 // indirect
	github.com/onsi/ginkgo/v2 v2.33.1 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/tools v0.52.0 // indirect
)

replace github.com/larsartmann/go-taskqueue/internal/journal => ../
