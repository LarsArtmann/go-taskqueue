//go:build windows

package readmodel

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const (
	lockfileExFailImmediately = 0x00000001
	lockfileExExclusiveLock   = 0x00000002
	// errorLockViolation is LockFileEx's contention code under
	// LOCKFILE_FAIL_IMMEDIATELY (winerror.h 33); the stdlib syscall
	// package does not re-export it.
	errorLockViolation = syscall.Errno(33)
)

var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modkernel32.NewProc("LockFileEx")
	procUnlockFileEx = modkernel32.NewProc("UnlockFileEx")
)

// lockExclusive takes the one-byte exclusive region lock at offset 0
// without blocking: busy reports contention (another folder holds the
// lease) as opposed to an I/O failure.
func lockExclusive(f *os.File) (busy bool, err error) {
	overlapped := new(syscall.Overlapped)

	r1, _, callErr := procLockFileEx.Call(
		f.Fd(),
		lockfileExFailImmediately|lockfileExExclusiveLock,
		0,
		1, // one-byte lease at offset 0
		0,
		uintptr(unsafe.Pointer(overlapped)),
	)
	if r1 != 0 {
		return false, nil
	}

	if callErr == errorLockViolation {
		return true, callErr
	}

	return false, fmt.Errorf("LockFileEx: %w", callErr)
}

// unlockFile drops the one-byte region lock at offset 0.
func unlockFile(f *os.File) error {
	overlapped := new(syscall.Overlapped)

	r1, _, callErr := procUnlockFileEx.Call(f.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(overlapped)))
	if r1 == 0 {
		return fmt.Errorf("UnlockFileEx: %w", callErr)
	}

	return nil
}
