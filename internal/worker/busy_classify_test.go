package worker

import (
	"errors"
	"fmt"
	"testing"

	"github.com/larsartmann/go-error-family"
)

// TestStoreBusyClassifierRegistered pins the T07 registration: the
// init-time errorfamily classifier must classify the sqlite lock-
// contention family (SQLITE_BUSY tokens in the message, wrapped or not)
// as Transient at the source, while the worker's own retry predicate
// stays narrow (busy-only; Classify's fail-open default never widens it).
func TestStoreBusyClassifierRegistered(t *testing.T) {
	t.Parallel()

	busyErr := errors.New("SQLITE_BUSY: database is locked (5)")
	if got := errorfamily.Classify(busyErr); got != errorfamily.Transient {
		t.Fatalf("errorfamily.Classify(SQLITE_BUSY err) = %v, want Transient (registered classifier)", got)
	}

	wrapped := fmt.Errorf("persist outcome: %w", errors.New("database is locked"))
	if got := errorfamily.Classify(wrapped); got != errorfamily.Transient {
		t.Fatalf("errorfamily.Classify(wrapped busy) = %v, want Transient", got)
	}

	if isTransientStoreBusy(errors.New("disk full")) {
		t.Fatal("non-busy error must not be classified store-busy")
	}
	if isTransientStoreBusy(nil) {
		t.Fatal("nil error must not be classified store-busy")
	}
}
