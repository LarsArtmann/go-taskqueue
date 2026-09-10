package executor

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// BenchmarkExecWithTransientRetrySuccess measures the happy-path overhead of
// the go-retry wrapper versus a bare call. Expectation: nanoseconds against
// an exec that costs milliseconds — pinned so nobody re-litigates "is the
// wrapper too slow" without numbers.
func BenchmarkExecWithTransientRetrySuccess(b *testing.B) {
	b.Run("bare", func(b *testing.B) {
		for b.Loop() {
			out, err := func() (string, error) { return "ok", nil }()

			if err != nil || out != "ok" {
				b.Fatal("bare call failed")
			}
		}
	})

	b.Run("wrapped", func(b *testing.B) {
		for b.Loop() {
			out, err := execWithTransientRetry(func() (string, error) { return "ok", nil })

			if err != nil || out != "ok" {
				b.Fatal("wrapped call failed")
			}
		}
	})
}

// TestExecWithTransientRetryRealETXTBSY is the end-to-end version of the
// ladder test: it provokes a REAL ETXTBSY from the kernel (execve of a
// binary that is concurrently open for writing) and verifies the helper
// absorbs it. Unit tests feed syscall.ETXTBSY as a stub value; this pins
// that the actual errno from an actual failed exec flows through the
// predicate. Unix-only: O_WRONLY exec locking is a POSIX behavior.
func TestExecWithTransientRetryRealETXTBSY(t *testing.T) {
	if runtime := os.Getenv("GOOS"); runtime == "windows" {
		t.Skip("ETXTBSY exec semantics are POSIX-only")
	}

	dir := t.TempDir()
	bin := filepath.Join(dir, "busy-bin")

	script := "#!/bin/sh\necho ran\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	// Hold the binary open for writing: execve now fails with ETXTBSY until
	// we release it, which a background goroutine does after a short delay —
	// inside the retry ladder's window.
	f, err := os.OpenFile(bin, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open for write: %v", err)
	}

	go func() {
		time.Sleep(80 * time.Millisecond)

		_ = f.Close()
	}()

	// Sanity: the immediate exec really does hit the kernel error.
	probe := exec.Command(bin)
	probeErr := probe.Run()

	out, err := execWithTransientRetry(func() (string, error) {
		var buf bytes.Buffer

		cmd := exec.Command(bin)
		cmd.Stdout = &buf

		cmdErr := cmd.Run()

		return buf.String(), cmdErr
	})

	if probeErr == nil || !errors.Is(probeErr, syscall.ETXTBSY) {
		t.Skipf("kernel did not produce ETXTBSY under this environment (%v); nothing to pin", probeErr)
	}

	if err != nil {
		t.Fatalf("ladder did not absorb real ETXTBSY: %v", err)
	}

	if out == "" {
		t.Fatal("expected stub output after recovery")
	}
}
