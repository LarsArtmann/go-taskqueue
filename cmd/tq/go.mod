module github.com/larsartmann/go-taskqueue/cmd/tq

go 1.26.7

require (
	github.com/larsartmann/go-codec v0.2.0
	github.com/larsartmann/go-cqrs-lite/event/v4 v4.11.0
	github.com/larsartmann/go-cqrs-lite/id/v4 v4.6.0
	github.com/larsartmann/go-taskqueue v0.3.0
	github.com/larsartmann/go-taskqueue/internal/executor v0.3.0
	github.com/larsartmann/go-taskqueue/internal/journal v0.3.0
	github.com/larsartmann/go-taskqueue/internal/journal/cqrs v0.3.0
	github.com/larsartmann/go-taskqueue/internal/queue v0.3.0
	github.com/larsartmann/go-taskqueue/internal/queue/sqlite v0.3.0
	github.com/larsartmann/go-taskqueue/internal/task v0.3.0
	github.com/larsartmann/go-taskqueue/internal/worker v0.3.0
	modernc.org/sqlite v1.58.0
)

require (
	// Pinned for the replace-based local build graph (internal/executor needs it since v0.3.0); the FOD for the hermetic nix build downloads
	// per committed go.mod, which is replace-free (ADR-0017).
	github.com/LarsArtmann/go-crush-data v0.4.0
	github.com/Oudwins/tailwind-merge-go v0.2.3 // indirect
	github.com/a-h/templ v0.3.1020 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/fxamacker/cbor/v2 v2.9.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/larsartmann/go-branded-id v0.5.1 // indirect
	github.com/larsartmann/go-cqrs-lite/metadata/v4 v4.7.0 // indirect
	github.com/larsartmann/go-cqrs-lite/record/v4 v4.5.0 // indirect
	github.com/larsartmann/go-error-family v0.10.0 // indirect
	github.com/larsartmann/go-retry v0.5.0 // indirect
	github.com/larsartmann/go-sse v0.6.0 // indirect
	github.com/larsartmann/templ-components v1.16.0 // indirect
	github.com/larsartmann/templ-components/htmx v1.16.0 // indirect
	github.com/larsartmann/templ-components/icons v1.16.0 // indirect
	github.com/larsartmann/templ-components/utils v1.16.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/oklog/ulid/v2 v2.1.2 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	modernc.org/libc v1.75.7 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
)
