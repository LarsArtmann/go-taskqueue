package executor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSweepSidecarsRemovesAgedKeepsLive(t *testing.T) {
	dir := t.TempDir()

	aged := filepath.Join(dir, "0001-aged.log")
	live := filepath.Join(dir, "0002-live.log")
	foreign := filepath.Join(dir, "0003-aged.txt")

	for _, p := range []string{aged, live, foreign} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(aged, old, old); err != nil {
		t.Fatal(err)
	}

	if err := os.Chtimes(foreign, old, old); err != nil {
		t.Fatal(err)
	}

	removed, err := SweepSidecars(dir, 24*time.Hour)
	if err != nil {
		t.Fatalf("SweepSidecars: %v", err)
	}

	if removed != 1 {
		t.Fatalf("removed %d files, want 1 (only the aged .log)", removed)
	}

	if _, err := os.Stat(aged); !os.IsNotExist(err) {
		t.Error("aged sidecar survived the sweep")
	}

	if _, err := os.Stat(live); err != nil {
		t.Error("live sidecar was swept before its age")
	}

	if _, err := os.Stat(foreign); err != nil {
		t.Error("non-log file was swept — the sweep must touch only sidecar logs")
	}
}

func TestSweepSidecarsDisabled(t *testing.T) {
	// Zero age and empty dir are no-ops, never errors.
	if n, err := SweepSidecars("", time.Hour); n != 0 || err != nil {
		t.Fatalf("empty dir = %d/%v, want 0/nil", n, err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.log"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if n, err := SweepSidecars(dir, 0); n != 0 || err != nil {
		t.Fatalf("zero max-age = %d/%v, want 0/nil (retention off)", n, err)
	}

	if _, err := os.Stat(filepath.Join(dir, "a.log")); err != nil {
		t.Error("zero max-age must not delete anything")
	}
}
