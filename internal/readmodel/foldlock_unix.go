//go:build unix

package readmodel

import (
	"errors"
	"os"
	"syscall"
)

// lockExclusive takes the whole-file exclusive advisory lock without
// blocking: busy reports contention (another folder holds the lease) as
// opposed to an I/O failure.
func lockExclusive(f *os.File) (busy bool, err error) {
	switch e := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); {
	case e == nil:
		return false, nil
	case errors.Is(e, syscall.EWOULDBLOCK), errors.Is(e, syscall.EAGAIN), errors.Is(e, syscall.EINTR):
		// Held elsewhere (EINTR on a non-blocking request reports the
		// same not-acquired outcome on some kernels) — the caller's
		// bounded wait retries either way.
		return true, e
	default:
		return false, e
	}
}

// unlockFile drops the whole-file advisory lock.
func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
