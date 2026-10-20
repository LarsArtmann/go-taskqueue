package readmodel

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// The fold lock is the projection home's single-writer guard: the Delta
// counter folds are NOT idempotent, so two models folding one home
// concurrently double-count every transition. Every folding Open elects
// itself through a non-blocking exclusive file lock on the
// `<projection-home>.fold-lock` sidecar; a lost election mounts the model
// read-only (store-backed reads, no fold, no checkpoint) and the lock
// holder stays the one folder. The lease lives in the kernel for the
// lifetime of the holding process — a crashed folder never wedges the
// home.

// FoldLockSuffix names the fold-lock sidecar beside a projection home.
const FoldLockSuffix = ".fold-lock"

// foldLockPoll is the retry cadence of the bounded wait in
// AcquireFoldLock.
const foldLockPoll = 100 * time.Millisecond

// ErrFoldLocked reports the projection home's fold lock is held by
// another folder: the caller lost the single-writer election and must
// mount read-only.
var ErrFoldLocked = errors.New("readmodel: projection fold lock held by another folder")

// FoldLockPath derives the fold-lock sidecar path for a projection home
// (the PathFor file).
func FoldLockPath(home string) string {
	return home + FoldLockSuffix
}

// FoldLock is one folder's held lease on a projection home. The model
// owns the lease from adoption on (WithFoldLock, or the internal acquire
// in Open) and releases it on Close.
type FoldLock struct {
	f *os.File
}

// AcquireFoldLock takes a projection home's fold lock: one non-blocking
// exclusive acquire for wait <= 0, otherwise the acquire retried every
// foldLockPoll until wait has elapsed (the composition runtime's bounded
// wait rides out a transient holder — a stats/doctor catchup — instead of
// demoting the long-lived serve folder on a startup overlap).
//
// ErrFoldLocked reports the election lost (mount read-only); any other
// error is an I/O failure, not contention. The lock file itself is never
// unlinked: unlinking under a held lock lets a third folder lock a fresh
// inode while the second still holds the old one — two folders, the exact
// corruption the lock exists to prevent. A stale zero-byte sidecar beside
// the projection home is the steady state, not a leak.
func AcquireFoldLock(home string, wait time.Duration) (*FoldLock, error) {
	var deadline time.Time

	if wait > 0 {
		deadline = time.Now().Add(wait)
	}

	for {
		lock, err := tryFoldLock(home)
		if err == nil || !errors.Is(err, ErrFoldLocked) {
			return lock, err
		}

		if wait <= 0 || !time.Now().Before(deadline) {
			return nil, ErrFoldLocked
		}

		time.Sleep(foldLockPoll)
	}
}

// tryFoldLock runs one non-blocking acquire round.
func tryFoldLock(home string) (*FoldLock, error) {
	f, err := os.OpenFile(FoldLockPath(home), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("readmodel: open %s: %w", FoldLockPath(home), err)
	}

	busy, lockErr := lockExclusive(f)
	if lockErr == nil {
		return &FoldLock{f: f}, nil
	}

	_ = f.Close()

	if busy {
		return nil, ErrFoldLocked
	}

	return nil, fmt.Errorf("readmodel: lock %s: %w", FoldLockPath(home), lockErr)
}

// Release drops the lease. Safe to call twice (the second is a no-op), so
// a caller-side cleanup may run after an Open-internal release.
func (l *FoldLock) Release() error {
	if l == nil || l.f == nil {
		return nil
	}

	err := unlockFile(l.f)

	if cerr := l.f.Close(); err == nil {
		err = cerr
	}

	l.f = nil

	if err != nil {
		return fmt.Errorf("readmodel: release fold lock: %w", err)
	}

	return nil
}
