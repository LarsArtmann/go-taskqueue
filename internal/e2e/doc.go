//go:build !unix

// Package e2e holds end-to-end CLI tests that drive the built tq binary.
// The tests are POSIX-only (process groups, /bin/sh plumbing) and carry
// //go:build unix tags; this placeholder keeps the package non-empty on
// other platforms so `go test ./...` does not fail with "build constraints
// exclude all Go files" there (round-5 M11/F55).
package e2e
