package retryx

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRenameTransientMovesFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")

	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	if err := RenameTransient(src, dst); err != nil {
		t.Fatalf("RenameTransient: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}

	if string(got) != "payload" {
		t.Errorf("dst content = %q, want %q", got, "payload")
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("src still present after rename (stat err = %v)", err)
	}
}

func TestRenameTransientToleratesMissingSource(t *testing.T) {
	t.Parallel()

	if err := RenameTransient(filepath.Join(t.TempDir(), "absent"), filepath.Join(t.TempDir(), "dst")); err != nil {
		t.Errorf("RenameTransient on missing source = %v, want nil", err)
	}
}

func TestRenameTransientRetriesTransientFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")

	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	dstOpen, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("open dst: %v", err)
	}

	go func() {
		time.Sleep(60 * time.Millisecond)

		_ = dstOpen.Close()
	}()

	if err := RenameTransient(src, dst); err != nil {
		t.Fatalf("RenameTransient with transient dst conflict: %v", err)
	}
}
